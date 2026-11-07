// src/instant_server/native/win_usb_flasher.c
#define _CRT_SECURE_NO_WARNINGS
#define _POSIX_C_SOURCE 200809L
#define _FILE_OFFSET_BITS 64

#include <errno.h>
#include <inttypes.h>
#include <stdbool.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>

#ifdef _WIN32
#ifndef WIN32_LEAN_AND_MEAN
#define WIN32_LEAN_AND_MEAN
#endif
#include <windows.h>
#include <winioctl.h>
#ifndef FSCTL_LOCK_VOLUME
#define FSCTL_LOCK_VOLUME CTL_CODE(FILE_DEVICE_FILE_SYSTEM, 6, METHOD_BUFFERED, FILE_ANY_ACCESS)
#endif
#ifndef FSCTL_UNLOCK_VOLUME
#define FSCTL_UNLOCK_VOLUME CTL_CODE(FILE_DEVICE_FILE_SYSTEM, 7, METHOD_BUFFERED, FILE_ANY_ACCESS)
#endif
#ifndef FSCTL_DISMOUNT_VOLUME
#define FSCTL_DISMOUNT_VOLUME CTL_CODE(FILE_DEVICE_FILE_SYSTEM, 8, METHOD_BUFFERED, FILE_ANY_ACCESS)
#endif
#ifndef FSCTL_ALLOW_EXTENDED_DASD_IO
#define FSCTL_ALLOW_EXTENDED_DASD_IO CTL_CODE(FILE_DEVICE_FILE_SYSTEM, 32, METHOD_NEITHER, FILE_ANY_ACCESS)
#endif
#else
#include <fcntl.h>
#include <sys/stat.h>
#include <sys/types.h>
#include <unistd.h>
#endif

#define DMA_BLOCK_SIZE (4U * 1024U * 1024U)
#define SECTOR_ALIGNMENT 4096U

static uint32_t g_crc32_table[256];
static bool g_crc32_ready = false;

static void init_crc32_table(void) {
    if (g_crc32_ready) {
        return;
    }
    for (uint32_t i = 0; i < 256U; i++) {
        uint32_t c = i;
        for (int j = 0; j < 8; j++) {
            c = (c & 1U) ? (0xEDB88320U ^ (c >> 1U)) : (c >> 1U);
        }
        g_crc32_table[i] = c;
    }
    g_crc32_ready = true;
}

static uint32_t update_crc32(uint32_t crc, const uint8_t *buf, size_t len) {
    uint32_t c = crc ^ 0xFFFFFFFFU;
    for (size_t i = 0; i < len; i++) {
        c = g_crc32_table[(c ^ buf[i]) & 0xFFU] ^ (c >> 8U);
    }
    return c ^ 0xFFFFFFFFU;
}

static double monotonic_seconds(void) {
#ifdef _WIN32
    LARGE_INTEGER freq;
    LARGE_INTEGER counter;
    if (!QueryPerformanceFrequency(&freq) || freq.QuadPart == 0) {
        return (double)GetTickCount64() / 1000.0;
    }
    QueryPerformanceCounter(&counter);
    return (double)counter.QuadPart / (double)freq.QuadPart;
#else
    struct timespec ts;
    if (clock_gettime(CLOCK_MONOTONIC, &ts) != 0) {
        return 0.0;
    }
    return (double)ts.tv_sec + (double)ts.tv_nsec / 1e9;
#endif
}

static void *alloc_aligned_dma(size_t size) {
#ifdef _WIN32
    return VirtualAlloc(NULL, size, MEM_COMMIT | MEM_RESERVE, PAGE_READWRITE);
#else
    void *ptr = NULL;
    if (posix_memalign(&ptr, SECTOR_ALIGNMENT, size) != 0) {
        return NULL;
    }
    return ptr;
#endif
}

static void free_aligned_dma(void *ptr) {
    if (ptr == NULL) {
        return;
    }
#ifdef _WIN32
    VirtualFree(ptr, 0, MEM_RELEASE);
#else
    free(ptr);
#endif
}

static int64_t get_file_size_64(FILE *fp) {
#ifdef _WIN32
    if (_fseeki64(fp, 0, SEEK_END) != 0) {
        return -1;
    }
    int64_t sz = _ftelli64(fp);
    _fseeki64(fp, 0, SEEK_SET);
    return sz;
#else
    if (fseeko(fp, 0, SEEK_END) != 0) {
        return -1;
    }
    off_t sz = ftello(fp);
    fseeko(fp, 0, SEEK_SET);
    return (int64_t)sz;
#endif
}

static size_t patch_grub_autoinstall(uint8_t *buf, size_t len) {
    const char *needle = "quiet ---";
    const char *replacement = "autoinstall ds=nocloud;s=/cdrom/nocloud/ quiet ---";
    const size_t n_len = strlen(needle);
    const size_t r_len = strlen(replacement);
    size_t count = 0;

    if (len < r_len + 64U) {
        return 0;
    }
    for (size_t i = 0; i + r_len + 32U < len; i++) {
        if (memcmp(buf + i, needle, n_len) == 0) {
            size_t pad = 0;
            for (size_t k = i + n_len; k < len && k < i + r_len + 16U; k++) {
                if (buf[k] == ' ' || buf[k] == '\0') {
                    pad++;
                } else {
                    break;
                }
            }
            if (n_len + pad >= r_len) {
                memcpy(buf + i, replacement, r_len);
                count++;
                i += r_len;
            }
        }
    }
    return count;
}

#ifdef _WIN32
typedef struct {
    HANDLE handles[26];
    int count;
} LockedVolumes;

static void lock_and_dismount_volumes(DWORD disk_index, LockedVolumes *lv) {
    lv->count = 0;
    DWORD mask = GetLogicalDrives();
    for (int letter = 0; letter < 26; letter++) {
        if ((mask & (1UL << letter)) == 0) {
            continue;
        }
        wchar_t vol_path[16];
        swprintf(vol_path, 16, L"\\\\.\\%c:", (wchar_t)(L'A' + letter));
        HANDLE hVol = CreateFileW(
            vol_path,
            GENERIC_READ | GENERIC_WRITE,
            FILE_SHARE_READ | FILE_SHARE_WRITE,
            NULL,
            OPEN_EXISTING,
            0,
            NULL
        );
        if (hVol == INVALID_HANDLE_VALUE) {
            continue;
        }
        VOLUME_DISK_EXTENTS ext;
        DWORD ret_bytes = 0;
        BOOL ok = DeviceIoControl(
            hVol,
            IOCTL_VOLUME_GET_VOLUME_DISK_EXTENTS,
            NULL,
            0,
            &ext,
            (DWORD)sizeof(ext),
            &ret_bytes,
            NULL
        );
        if (!ok || ext.NumberOfDiskExtents == 0 || ext.Extents[0].DiskNumber != disk_index) {
            CloseHandle(hVol);
            continue;
        }
        BOOL locked = FALSE;
        for (int attempt = 0; attempt < 12; attempt++) {
            if (DeviceIoControl(hVol, FSCTL_LOCK_VOLUME, NULL, 0, NULL, 0, &ret_bytes, NULL)) {
                locked = TRUE;
                break;
            }
            Sleep(120);
        }
        DeviceIoControl(hVol, FSCTL_DISMOUNT_VOLUME, NULL, 0, NULL, 0, &ret_bytes, NULL);
        if (locked && lv->count < 26) {
            lv->handles[lv->count++] = hVol;
        } else {
            CloseHandle(hVol);
        }
    }
}

static void unlock_volumes(LockedVolumes *lv) {
    DWORD ret_bytes = 0;
    for (int i = 0; i < lv->count; i++) {
        DeviceIoControl(lv->handles[i], FSCTL_UNLOCK_VOLUME, NULL, 0, NULL, 0, &ret_bytes, NULL);
        CloseHandle(lv->handles[i]);
    }
    lv->count = 0;
}
#endif

static int flash_image(const char *iso_path, const char *target_path, bool patch_grub, bool verify_crc) {
    init_crc32_table();
    FILE *iso_fp = fopen(iso_path, "rb");
    if (iso_fp == NULL) {
        fprintf(stderr, "{\"error\":\"IS-ERR-102\",\"message\":\"Cannot open ISO: %s\"}\n", iso_path);
        return 2;
    }
    int64_t total_bytes = get_file_size_64(iso_fp);
    if (total_bytes <= 0) {
        fclose(iso_fp);
        fprintf(stderr, "{\"error\":\"IS-ERR-102\",\"message\":\"ISO file is empty\"}\n");
        return 2;
    }

    uint8_t *dma_buf = (uint8_t *)alloc_aligned_dma(DMA_BLOCK_SIZE);
    uint8_t *ver_buf = (uint8_t *)alloc_aligned_dma(DMA_BLOCK_SIZE);
    if (dma_buf == NULL || ver_buf == NULL) {
        fclose(iso_fp);
        free_aligned_dma(dma_buf);
        free_aligned_dma(ver_buf);
        fprintf(stderr, "{\"error\":\"IS-ERR-103\",\"message\":\"DMA allocation failed\"}\n");
        return 3;
    }

#ifdef _WIN32
    unsigned long parsed_idx = 0xFFFFFFFFUL;
    if (sscanf(target_path, "\\\\.\\PhysicalDrive%lu", &parsed_idx) != 1) {
        sscanf(target_path, "//./PhysicalDrive%lu", &parsed_idx);
    }
    LockedVolumes lv;
    memset(&lv, 0, sizeof(lv));
    if (parsed_idx != 0xFFFFFFFFUL) {
        lock_and_dismount_volumes((DWORD)parsed_idx, &lv);
    }
    HANDLE hTarget = CreateFileA(
        target_path,
        GENERIC_READ | GENERIC_WRITE,
        FILE_SHARE_READ | FILE_SHARE_WRITE,
        NULL,
        OPEN_EXISTING,
        FILE_FLAG_NO_BUFFERING | FILE_FLAG_WRITE_THROUGH,
        NULL
    );
    if (hTarget == INVALID_HANDLE_VALUE) {
        unlock_volumes(&lv);
        fclose(iso_fp);
        free_aligned_dma(dma_buf);
        free_aligned_dma(ver_buf);
        fprintf(stderr, "{\"error\":\"IS-ERR-103\",\"message\":\"Win32 CreateFileA failed on %s\"}\n", target_path);
        return 4;
    }
    DWORD dasd_bytes = 0;
    DeviceIoControl(hTarget, FSCTL_ALLOW_EXTENDED_DASD_IO, NULL, 0, NULL, 0, &dasd_bytes, NULL);
#else
    int target_fd = open(target_path, O_RDWR | O_CREAT, 0644);
    if (target_fd < 0) {
        fclose(iso_fp);
        free_aligned_dma(dma_buf);
        free_aligned_dma(ver_buf);
        fprintf(stderr, "{\"error\":\"IS-ERR-103\",\"message\":\"POSIX open failed on %s: %s\"}\n",
                target_path, strerror(errno));
        return 4;
    }
#ifdef F_NOCACHE
    (void)fcntl(target_fd, F_NOCACHE, 1);
#endif
#endif

    uint32_t src_crc = 0;
    int64_t written = 0;
    size_t patches = 0;
    double t0 = monotonic_seconds();

    for (;;) {
        size_t n_read = fread(dma_buf, 1, DMA_BLOCK_SIZE, iso_fp);
        if (n_read == 0) {
            break;
        }
        if (patch_grub && written < (64LL * 1024LL * 1024LL)) {
            patches += patch_grub_autoinstall(dma_buf, n_read);
        }
        size_t to_write = n_read;
#ifdef _WIN32
        if (parsed_idx != 0xFFFFFFFFUL && (to_write % SECTOR_ALIGNMENT) != 0) {
            size_t padded = ((to_write + SECTOR_ALIGNMENT - 1U) / SECTOR_ALIGNMENT) * SECTOR_ALIGNMENT;
            memset(dma_buf + to_write, 0, padded - to_write);
            to_write = padded;
        }
        DWORD bw = 0;
        if (!WriteFile(hTarget, dma_buf, (DWORD)to_write, &bw, NULL) || (size_t)bw != to_write) {
            CloseHandle(hTarget);
            unlock_volumes(&lv);
            fclose(iso_fp);
            free_aligned_dma(dma_buf);
            free_aligned_dma(ver_buf);
            fprintf(stderr, "{\"error\":\"IS-ERR-104\",\"message\":\"WriteFile error\"}\n");
            return 5;
        }
#else
        size_t off = 0;
        while (off < to_write) {
            ssize_t w = write(target_fd, dma_buf + off, to_write - off);
            if (w <= 0) {
                close(target_fd);
                fclose(iso_fp);
                free_aligned_dma(dma_buf);
                free_aligned_dma(ver_buf);
                fprintf(stderr, "{\"error\":\"IS-ERR-104\",\"message\":\"write error\"}\n");
                return 5;
            }
            off += (size_t)w;
        }
#endif
        src_crc = update_crc32(src_crc, dma_buf, n_read);
        written += (int64_t)n_read;

        double elapsed = monotonic_seconds() - t0;
        double mb_s = (elapsed > 0.001) ? (((double)written / (1024.0 * 1024.0)) / elapsed) : 0.0;
        int pct = (int)((written * 100LL) / total_bytes);
        double eta = (mb_s > 0.1) ? ((((double)(total_bytes - written)) / (1024.0 * 1024.0)) / mb_s) : 0.0;
        printf(
            "{\"stage\":\"FLASHING\",\"pct\":%d,\"written_bytes\":%" PRId64 ",\"total_bytes\":%" PRId64 ",\"mb_s\":%.2f,\"eta_sec\":%.1f,\"crc32\":\"%08" PRIX32 "\"}\n",
            pct,
            written,
            total_bytes,
            mb_s,
            eta,
            src_crc
        );
        fflush(stdout);
    }

#ifdef _WIN32
    FlushFileBuffers(hTarget);
#else
    fsync(target_fd);
#endif

    if (verify_crc) {
#ifdef _WIN32
        LARGE_INTEGER zero_pos;
        zero_pos.QuadPart = 0;
        SetFilePointerEx(hTarget, zero_pos, NULL, FILE_BEGIN);
#else
        lseek(target_fd, 0, SEEK_SET);
#endif
        uint32_t read_crc = 0;
        int64_t checked = 0;
        while (checked < total_bytes) {
            size_t chunk = DMA_BLOCK_SIZE;
            if (total_bytes - checked < (int64_t)chunk) {
                chunk = (size_t)(total_bytes - checked);
            }
#ifdef _WIN32
            size_t aligned_chunk = (parsed_idx != 0xFFFFFFFFUL)
                ? (((chunk + SECTOR_ALIGNMENT - 1U) / SECTOR_ALIGNMENT) * SECTOR_ALIGNMENT)
                : chunk;
            DWORD br = 0;
            if (!ReadFile(hTarget, ver_buf, (DWORD)aligned_chunk, &br, NULL) || (size_t)br < chunk) {
                CloseHandle(hTarget);
                unlock_volumes(&lv);
                fclose(iso_fp);
                free_aligned_dma(dma_buf);
                free_aligned_dma(ver_buf);
                return 6;
            }
#else
            ssize_t br = read(target_fd, ver_buf, chunk);
            if (br != (ssize_t)chunk) {
                close(target_fd);
                fclose(iso_fp);
                free_aligned_dma(dma_buf);
                free_aligned_dma(ver_buf);
                return 6;
            }
#endif
            read_crc = update_crc32(read_crc, ver_buf, chunk);
            checked += (int64_t)chunk;
        }
        if (read_crc != src_crc) {
#ifdef _WIN32
            CloseHandle(hTarget);
            unlock_volumes(&lv);
#else
            close(target_fd);
#endif
            fclose(iso_fp);
            free_aligned_dma(dma_buf);
            free_aligned_dma(ver_buf);
            fprintf(stderr, "{\"error\":\"IS-ERR-104\",\"message\":\"CRC32 mismatch\"}\n");
            return 7;
        }
        printf("{\"stage\":\"VERIFIED\",\"pct\":100,\"crc32\":\"%08" PRIX32 "\",\"grub_patches\":%zu}\n",
               read_crc, patches);
        fflush(stdout);
    }

#ifdef _WIN32
    DWORD upd_bytes = 0;
    DeviceIoControl(hTarget, IOCTL_DISK_UPDATE_PROPERTIES, NULL, 0, NULL, 0, &upd_bytes, NULL);
    CloseHandle(hTarget);
    unlock_volumes(&lv);
#else
    close(target_fd);
#endif
    fclose(iso_fp);
    free_aligned_dma(dma_buf);
    free_aligned_dma(ver_buf);
    return 0;
}

static int run_self_test(void) {
    const char *src_tmp = "win_flasher_selftest_src.bin";
    const char *dst_tmp = "win_flasher_selftest_dst.bin";
    FILE *fp = fopen(src_tmp, "wb");
    if (fp == NULL) {
        return 1;
    }
    char block[4096];
    memset(block, ' ', sizeof(block));
    const char *grub_sample = "linux /casper/vmlinuz quiet ---                                                    ";
    memcpy(block + 128, grub_sample, strlen(grub_sample));
    for (int i = 0; i < 64; i++) {
        if (fwrite(block, 1, sizeof(block), fp) != sizeof(block)) {
            fclose(fp);
            return 1;
        }
    }
    fclose(fp);
    int rc = flash_image(src_tmp, dst_tmp, true, true);
    remove(src_tmp);
    remove(dst_tmp);
    return rc;
}

int main(int argc, char **argv) {
    if (argc == 2 && strcmp(argv[1], "--self-test") == 0) {
        return run_self_test();
    }
    const char *iso_path = NULL;
    const char *target_path = NULL;
    bool patch_grub = true;
    bool verify_crc = true;

    for (int i = 1; i < argc; i++) {
        if (strcmp(argv[i], "--iso") == 0 && i + 1 < argc) {
            iso_path = argv[++i];
        } else if (strcmp(argv[i], "--target") == 0 && i + 1 < argc) {
            target_path = argv[++i];
        } else if (strcmp(argv[i], "--no-patch") == 0) {
            patch_grub = false;
        } else if (strcmp(argv[i], "--no-verify") == 0) {
            verify_crc = false;
        }
    }
    if (iso_path == NULL || target_path == NULL) {
        fprintf(stderr, "Usage: %s --iso <path> --target <device> [--no-patch] [--no-verify] | --self-test\n", argv[0]);
        return 1;
    }
    return flash_image(iso_path, target_path, patch_grub, verify_crc);
}