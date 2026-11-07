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
#ifndef IOCTL_VOLUME_OFFLINE
#define IOCTL_VOLUME_OFFLINE CTL_CODE(IOCTL_VOLUME_BASE, 3, METHOD_BUFFERED, FILE_READ_ACCESS | FILE_WRITE_ACCESS)
#endif
#ifndef IOCTL_VOLUME_ONLINE
#define IOCTL_VOLUME_ONLINE CTL_CODE(IOCTL_VOLUME_BASE, 2, METHOD_BUFFERED, FILE_READ_ACCESS | FILE_WRITE_ACCESS)
#endif
#ifndef IOCTL_DISK_SET_DISK_ATTRIBUTES
#define IOCTL_DISK_SET_DISK_ATTRIBUTES CTL_CODE(IOCTL_DISK_BASE, 0x003d, METHOD_BUFFERED, FILE_READ_ACCESS | FILE_WRITE_ACCESS)
#endif
#ifndef DISK_ATTRIBUTE_OFFLINE
#define DISK_ATTRIBUTE_OFFLINE 0x0000000000000001ULL
#endif
#ifndef DISK_ATTRIBUTE_READ_ONLY
#define DISK_ATTRIBUTE_READ_ONLY 0x0000000000000002ULL
#endif

typedef struct _IS_SET_DISK_ATTRIBUTES {
    DWORD Version;
    BOOLEAN Persist;
    BOOLEAN Reserved1[3];
    DWORDLONG Attributes;
    DWORDLONG AttributesMask;
    DWORD Reserved2[4];
} IS_SET_DISK_ATTRIBUTES;

#else
#include <fcntl.h>
#include <sys/stat.h>
#include <sys/types.h>
#include <unistd.h>
#endif

#define DMA_BLOCK_SIZE (4U * 1024U * 1024U)
#define SECTOR_ALIGNMENT 4096U
#define LOGICAL_SECTOR_SIZE 512U
#define GPT_HEADER_SIGNATURE 0x5452415020494645ULL
#define GPT_REVISION_1_0 0x00010000U
#define GPT_ENTRY_COUNT 128U
#define GPT_ENTRY_SIZE 128U
#define GPT_TABLE_SECTORS 32U
#define MAX_LOCKED_VOLUMES 32

#pragma pack(push, 1)
typedef struct {
    uint8_t boot_indicator;
    uint8_t start_head;
    uint8_t start_sector;
    uint8_t start_track;
    uint8_t os_type;
    uint8_t end_head;
    uint8_t end_sector;
    uint8_t end_track;
    uint32_t starting_lba;
    uint32_t size_in_lba;
} MbrPartitionRecord;

typedef struct {
    uint8_t boot_code[440];
    uint32_t disk_signature;
    uint16_t reserved;
    MbrPartitionRecord partitions[4];
    uint16_t signature;
} MasterBootRecord;

typedef struct {
    uint64_t signature;
    uint32_t revision;
    uint32_t header_size;
    uint32_t header_crc32;
    uint32_t reserved;
    uint64_t my_lba;
    uint64_t alternate_lba;
    uint64_t first_usable_lba;
    uint64_t last_usable_lba;
    uint8_t disk_guid[16];
    uint64_t partition_entry_lba;
    uint32_t num_partition_entries;
    uint32_t size_of_partition_entry;
    uint32_t partition_entry_array_crc32;
    uint8_t reserved_padding[420];
} GptHeaderSector;

typedef struct {
    uint8_t partition_type_guid[16];
    uint8_t unique_partition_guid[16];
    uint64_t starting_lba;
    uint64_t ending_lba;
    uint64_t attributes;
    uint16_t partition_name[36];
} GptPartitionEntry;

typedef struct {
    uint8_t jmp_boot[3];
    uint8_t oem_name[8];
    uint16_t bytes_per_sector;
    uint8_t sectors_per_cluster;
    uint16_t reserved_sector_count;
    uint8_t num_fats;
    uint16_t root_entry_count;
    uint16_t total_sectors_16;
    uint8_t media;
    uint16_t fat_size_16;
    uint16_t sectors_per_track;
    uint16_t num_heads;
    uint32_t hidden_sectors;
    uint32_t total_sectors_32;
    uint32_t fat_size_32;
    uint16_t ext_flags;
    uint16_t fs_version;
    uint32_t root_cluster;
    uint16_t fs_info;
    uint16_t backup_boot_sector;
    uint8_t reserved[12];
    uint8_t drive_number;
    uint8_t reserved1;
    uint8_t boot_signature;
    uint32_t volume_id;
    uint8_t volume_label[11];
    uint8_t fs_type[8];
    uint8_t boot_code[420];
    uint16_t signature;
} Fat32BootSector;

typedef struct {
    uint32_t lead_sig;
    uint8_t reserved1[480];
    uint32_t struc_sig;
    uint32_t free_count;
    uint32_t next_free;
    uint8_t reserved2[12];
    uint32_t trail_sig;
} Fat32FsInfoSector;
#pragma pack(pop)

typedef struct {
    uint32_t state[8];
    uint64_t bitcount;
    uint8_t buffer[64];
} Sha256Context;

static uint32_t g_crc32_table[256];
static bool g_crc32_ready = false;

static const uint8_t GUID_BASIC_DATA_LE[16] = {
    0xA2, 0xA0, 0xD0, 0xEB, 0xE5, 0xB9, 0x33, 0x44,
    0x87, 0xC0, 0x68, 0xB6, 0xB7, 0x26, 0x99, 0xC7
};

static const uint8_t GUID_EFI_SYSTEM_LE[16] = {
    0x28, 0x73, 0x2A, 0xC1, 0x1F, 0xF8, 0xD2, 0x11,
    0xBA, 0x4B, 0x00, 0xA0, 0xC9, 0x3E, 0xC9, 0x3B
};

static const uint32_t SHA256_K[64] = {
    0x428a2f98UL, 0x71374491UL, 0xb5c0fbcfUL, 0xe9b5dba5UL,
    0x3956c25bUL, 0x59f111f1UL, 0x923f82a4UL, 0xab1c5ed5UL,
    0xd807aa98UL, 0x12835b01UL, 0x243185beUL, 0x550c7dc3UL,
    0x72be5d74UL, 0x80deb1feUL, 0x9bdc06a7UL, 0xc19bf174UL,
    0xe49b69c1UL, 0xefbe4786UL, 0x0fc19dc6UL, 0x240ca1ccUL,
    0x2de92c6fUL, 0x4a7484aaUL, 0x5cb0a9dcUL, 0x76f988daUL,
    0x983e5152UL, 0xa831c66dUL, 0xb00327c8UL, 0xbf597fc7UL,
    0xc6e00bf3UL, 0xd5a79147UL, 0x06ca6351UL, 0x14292967UL,
    0x27b70a85UL, 0x2e1b2138UL, 0x4d2c6dfcUL, 0x53380d13UL,
    0x650a7354UL, 0x766a0abbUL, 0x81c2c92eUL, 0x92722c85UL,
    0xa2bfe8a1UL, 0xa81a664bUL, 0xc24b8b70UL, 0xc76c51a3UL,
    0xd192e819UL, 0xd6990624UL, 0xf40e3585UL, 0x106aa070UL,
    0x19a4c116UL, 0x1e376c08UL, 0x2748774cUL, 0x34b0bcb5UL,
    0x391c0cb3UL, 0x4ed8aa4aUL, 0x5b9cca4fUL, 0x682e6ff3UL,
    0x748f82eeUL, 0x78a5636fUL, 0x84c87814UL, 0x8cc70208UL,
    0x90befffaUL, 0xa4506cebUL, 0xbef9a3f7UL, 0xc67178f2UL
};

static uint32_t rotr32(uint32_t x, uint32_t n) {
    return (x >> n) | (x << (32U - n));
}

static void sha256_init(Sha256Context *ctx) {
    ctx->state[0] = 0x6a09e667UL;
    ctx->state[1] = 0xbb67ae85UL;
    ctx->state[2] = 0x3c6ef372UL;
    ctx->state[3] = 0xa54ff53aUL;
    ctx->state[4] = 0x510e527fUL;
    ctx->state[5] = 0x9b05688cUL;
    ctx->state[6] = 0x1f83d9abUL;
    ctx->state[7] = 0x5be0cd19UL;
    ctx->bitcount = 0;
    memset(ctx->buffer, 0, sizeof(ctx->buffer));
}

static void sha256_transform(Sha256Context *ctx, const uint8_t data[64]) {
    uint32_t w[64];
    for (int i = 0; i < 16; i++) {
        w[i] = ((uint32_t)data[i * 4] << 24) |
               ((uint32_t)data[i * 4 + 1] << 16) |
               ((uint32_t)data[i * 4 + 2] << 8) |
               ((uint32_t)data[i * 4 + 3]);
    }
    for (int i = 16; i < 64; i++) {
        uint32_t s0 = rotr32(w[i - 15], 7) ^ rotr32(w[i - 15], 18) ^ (w[i - 15] >> 3);
        uint32_t s1 = rotr32(w[i - 2], 17) ^ rotr32(w[i - 2], 19) ^ (w[i - 2] >> 10);
        w[i] = w[i - 16] + s0 + w[i - 7] + s1;
    }
    uint32_t a = ctx->state[0];
    uint32_t b = ctx->state[1];
    uint32_t c = ctx->state[2];
    uint32_t d = ctx->state[3];
    uint32_t e = ctx->state[4];
    uint32_t f = ctx->state[5];
    uint32_t g = ctx->state[6];
    uint32_t h = ctx->state[7];

    for (int i = 0; i < 64; i++) {
        uint32_t S1 = rotr32(e, 6) ^ rotr32(e, 11) ^ rotr32(e, 25);
        uint32_t ch = (e & f) ^ ((~e) & g);
        uint32_t temp1 = h + S1 + ch + SHA256_K[i] + w[i];
        uint32_t S0 = rotr32(a, 2) ^ rotr32(a, 13) ^ rotr32(a, 22);
        uint32_t maj = (a & b) ^ (a & c) ^ (b & c);
        uint32_t temp2 = S0 + maj;

        h = g;
        g = f;
        f = e;
        e = d + temp1;
        d = c;
        c = b;
        b = a;
        a = temp1 + temp2;
    }
    ctx->state[0] += a;
    ctx->state[1] += b;
    ctx->state[2] += c;
    ctx->state[3] += d;
    ctx->state[4] += e;
    ctx->state[5] += f;
    ctx->state[6] += g;
    ctx->state[7] += h;
}

static void sha256_update(Sha256Context *ctx, const uint8_t *data, size_t len) {
    size_t idx = (size_t)((ctx->bitcount >> 3) & 0x3FU);
    ctx->bitcount += (uint64_t)len << 3;
    size_t part_len = 64U - idx;
    size_t i = 0;
    if (len >= part_len) {
        memcpy(&ctx->buffer[idx], data, part_len);
        sha256_transform(ctx, ctx->buffer);
        for (i = part_len; i + 63U < len; i += 64U) {
            sha256_transform(ctx, &data[i]);
        }
        idx = 0;
    }
    memcpy(&ctx->buffer[idx], &data[i], len - i);
}

static void sha256_final_hex(Sha256Context *ctx, char out_hex[65]) {
    uint8_t pad[64];
    memset(pad, 0, sizeof(pad));
    pad[0] = 0x80U;
    uint64_t bits_be = ctx->bitcount;
    uint8_t len_bytes[8];
    for (int i = 0; i < 8; i++) {
        len_bytes[7 - i] = (uint8_t)(bits_be & 0xFFU);
        bits_be >>= 8;
    }
    size_t idx = (size_t)((ctx->bitcount >> 3) & 0x3FU);
    size_t pad_len = (idx < 56U) ? (56U - idx) : (120U - idx);
    sha256_update(ctx, pad, pad_len);
    sha256_update(ctx, len_bytes, 8U);
    for (int i = 0; i < 8; i++) {
        snprintf(out_hex + (i * 8), 9, "%08" PRIx32, ctx->state[i]);
    }
    out_hex[64] = '\0';
}

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
    init_crc32_table();
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

static void sleep_millis(unsigned int ms) {
#ifdef _WIN32
    Sleep((DWORD)ms);
#else
    struct timespec req;
    req.tv_sec = (time_t)(ms / 1000U);
    req.tv_nsec = (long)((ms % 1000U) * 1000000UL);
    nanosleep(&req, NULL);
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

static void generate_deterministic_guid(uint8_t out_guid[16], uint64_t seed_a, uint64_t seed_b) {
    uint64_t lo = seed_a ^ 0x6A09E667F3BCC908ULL;
    uint64_t hi = seed_b ^ 0xBB67AE8584CAA73BULL;
    memcpy(out_guid, &lo, 8);
    memcpy(out_guid + 8, &hi, 8);
    out_guid[7] = (uint8_t)((out_guid[7] & 0x0FU) | 0x40U);
    out_guid[8] = (uint8_t)((out_guid[8] & 0x3FU) | 0x80U);
}

static void encode_utf16le_label(uint16_t out_name[36], const char *ascii) {
    memset(out_name, 0, 36U * sizeof(uint16_t));
    for (size_t i = 0; i < 35U && ascii[i] != '\0'; i++) {
        out_name[i] = (uint16_t)(uint8_t)ascii[i];
    }
}

static size_t patch_grub_autoinstall(uint8_t *buf, size_t len) {
    const char *needle = "quiet ---";
    const char *replacement = "autoinstall ds=nocloud;s=/cdrom/cidata/ quiet ---";
    const size_t n_len = strlen(needle);
    const size_t r_len = strlen(replacement);
    size_t count = 0;

    for (size_t i = 0; i + 16U < len; i++) {
        if (memcmp(buf + i, "set timeout=30", 14U) == 0) {
            memcpy(buf + i, "set timeout=03", 14U);
            count++;
        }
    }
    if (len < r_len + 64U) {
        return count;
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

static int build_protective_mbr(uint8_t sector_out[512], uint64_t total_sectors) {
    memset(sector_out, 0, 512U);
    MasterBootRecord *mbr = (MasterBootRecord *)sector_out;
    mbr->disk_signature = 0x49535256U;
    mbr->partitions[0].boot_indicator = 0x00U;
    mbr->partitions[0].start_head = 0x00U;
    mbr->partitions[0].start_sector = 0x02U;
    mbr->partitions[0].start_track = 0x00U;
    mbr->partitions[0].os_type = 0xEEU;
    mbr->partitions[0].end_head = 0xFFU;
    mbr->partitions[0].end_sector = 0xFFU;
    mbr->partitions[0].end_track = 0xFFU;
    mbr->partitions[0].starting_lba = 1U;
    mbr->partitions[0].size_in_lba = (total_sectors - 1ULL > 0xFFFFFFFFULL)
        ? 0xFFFFFFFFU
        : (uint32_t)(total_sectors - 1ULL);
    mbr->signature = 0xAA55U;
    return 0;
}

static int build_fat32_cidata_superblock(
    uint8_t *region_buf,
    size_t region_buf_len,
    uint32_t total_sectors,
    uint32_t hidden_lba
) {
    const uint32_t reserved_sectors = 32U;
    const uint32_t sectors_per_cluster = 8U;
    if (region_buf_len < 64U * 512U || total_sectors < 4096U) {
        return -1;
    }
    memset(region_buf, 0, region_buf_len);
    uint32_t data_clusters = (total_sectors - reserved_sectors) / sectors_per_cluster;
    uint32_t fat_sectors = ((data_clusters + 2U) * 4U + 511U) / 512U;
    fat_sectors = ((fat_sectors + 7U) / 8U) * 8U;

    Fat32BootSector *bs = (Fat32BootSector *)region_buf;
    bs->jmp_boot[0] = 0xEBU;
    bs->jmp_boot[1] = 0x58U;
    bs->jmp_boot[2] = 0x90U;
    memcpy(bs->oem_name, "MSWIN4.1", 8U);
    bs->bytes_per_sector = 512U;
    bs->sectors_per_cluster = (uint8_t)sectors_per_cluster;
    bs->reserved_sector_count = (uint16_t)reserved_sectors;
    bs->num_fats = 2U;
    bs->root_entry_count = 0U;
    bs->total_sectors_16 = 0U;
    bs->media = 0xF8U;
    bs->fat_size_16 = 0U;
    bs->sectors_per_track = 63U;
    bs->num_heads = 255U;
    bs->hidden_sectors = hidden_lba;
    bs->total_sectors_32 = total_sectors;
    bs->fat_size_32 = fat_sectors;
    bs->ext_flags = 0U;
    bs->fs_version = 0U;
    bs->root_cluster = 2U;
    bs->fs_info = 1U;
    bs->backup_boot_sector = 6U;
    bs->drive_number = 0x80U;
    bs->boot_signature = 0x29U;
    bs->volume_id = 0x20261107U;
    memcpy(bs->volume_label, "CIDATA     ", 11U);
    memcpy(bs->fs_type, "FAT32   ", 8U);
    bs->signature = 0xAA55U;

    Fat32FsInfoSector *fsi = (Fat32FsInfoSector *)(region_buf + 512U);
    fsi->lead_sig = 0x41615252UL;
    fsi->struc_sig = 0x61417272UL;
    fsi->free_count = (data_clusters > 2U) ? (data_clusters - 1U) : 0U;
    fsi->next_free = 3U;
    fsi->trail_sig = 0xAA550000UL;

    memcpy(region_buf + (6U * 512U), region_buf, 2U * 512U);

    size_t fat1_offset = (size_t)reserved_sectors * 512U;
    if (fat1_offset + 16U <= region_buf_len) {
        uint32_t *fat1 = (uint32_t *)(region_buf + fat1_offset);
        fat1[0] = 0x0FFFFFF8UL;
        fat1[1] = 0xFFFFFFFFUL;
        fat1[2] = 0x0FFFFFFFUL;
    }
    return 0;
}

static int relocate_gpt_and_inject_cidata_file(
    const char *image_path,
    uint32_t cidata_mb,
    uint32_t *out_part_num,
    uint64_t *out_cidata_start_lba,
    uint32_t *out_primary_crc
) {
    init_crc32_table();
    FILE *fp = fopen(image_path, "r+b");
    if (fp == NULL) {
        return -1;
    }
    int64_t total_bytes = get_file_size_64(fp);
    if (total_bytes < (int64_t)(4096LL * LOGICAL_SECTOR_SIZE)) {
        fclose(fp);
        return -2;
    }
    uint64_t total_sectors = (uint64_t)total_bytes / LOGICAL_SECTOR_SIZE;

    GptHeaderSector primary_hdr;
    if (
#ifdef _WIN32
        _fseeki64(fp, (int64_t)LOGICAL_SECTOR_SIZE, SEEK_SET) != 0
#else
        fseeko(fp, (off_t)LOGICAL_SECTOR_SIZE, SEEK_SET) != 0
#endif
        || fread(&primary_hdr, 1, sizeof(primary_hdr), fp) != sizeof(primary_hdr)
    ) {
        fclose(fp);
        return -3;
    }

    if (primary_hdr.signature != GPT_HEADER_SIGNATURE || primary_hdr.size_of_partition_entry != GPT_ENTRY_SIZE) {
        fclose(fp);
        return -4;
    }

    uint32_t num_entries = primary_hdr.num_partition_entries;
    if (num_entries == 0 || num_entries > 512U) {
        fclose(fp);
        return -5;
    }
    size_t table_bytes = (size_t)num_entries * GPT_ENTRY_SIZE;
    uint64_t table_sectors = (table_bytes + LOGICAL_SECTOR_SIZE - 1U) / LOGICAL_SECTOR_SIZE;

    GptPartitionEntry *entries = (GptPartitionEntry *)calloc(num_entries, sizeof(GptPartitionEntry));
    if (entries == NULL) {
        fclose(fp);
        return -6;
    }

    if (
#ifdef _WIN32
        _fseeki64(fp, (int64_t)(primary_hdr.partition_entry_lba * LOGICAL_SECTOR_SIZE), SEEK_SET) != 0
#else
        fseeko(fp, (off_t)(primary_hdr.partition_entry_lba * LOGICAL_SECTOR_SIZE), SEEK_SET) != 0
#endif
        || fread(entries, 1, table_bytes, fp) != table_bytes
    ) {
        free(entries);
        fclose(fp);
        return -7;
    }

    uint64_t max_used_lba = primary_hdr.first_usable_lba - 1ULL;
    int free_slot = -1;
    uint8_t zero_guid[16];
    memset(zero_guid, 0, sizeof(zero_guid));

    for (uint32_t i = 0; i < num_entries; i++) {
        if (memcmp(entries[i].partition_type_guid, zero_guid, 16U) == 0) {
            if (free_slot < 0) {
                free_slot = (int)i;
            }
        } else {
            if (entries[i].ending_lba > max_used_lba) {
                max_used_lba = entries[i].ending_lba;
            }
        }
    }
    if (free_slot < 0) {
        free(entries);
        fclose(fp);
        return -8;
    }

    uint64_t new_backup_hdr_lba = total_sectors - 1ULL;
    uint64_t new_backup_tbl_lba = new_backup_hdr_lba - table_sectors;
    uint64_t new_last_usable_lba = new_backup_tbl_lba - 1ULL;

    uint64_t align = 2048ULL;
    uint64_t cidata_start = ((max_used_lba + align) / align) * align;
    uint64_t cidata_sectors = ((uint64_t)cidata_mb * 1024ULL * 1024ULL) / LOGICAL_SECTOR_SIZE;
    uint64_t cidata_end = cidata_start + cidata_sectors - 1ULL;

    if (cidata_end > new_last_usable_lba) {
        free(entries);
        fclose(fp);
        return -9;
    }

    GptPartitionEntry *slot = &entries[free_slot];
    memcpy(slot->partition_type_guid, GUID_BASIC_DATA_LE, 16U);
    generate_deterministic_guid(slot->unique_partition_guid, cidata_start, cidata_end);
    slot->starting_lba = cidata_start;
    slot->ending_lba = cidata_end;
    slot->attributes = 0ULL;
    encode_utf16le_label(slot->partition_name, "cidata");

    uint32_t entries_crc = update_crc32(0U, (const uint8_t *)entries, table_bytes);

    primary_hdr.header_crc32 = 0U;
    primary_hdr.reserved = 0U;
    primary_hdr.my_lba = 1ULL;
    primary_hdr.alternate_lba = new_backup_hdr_lba;
    primary_hdr.last_usable_lba = new_last_usable_lba;
    primary_hdr.partition_entry_array_crc32 = entries_crc;
    memset(primary_hdr.reserved_padding, 0, sizeof(primary_hdr.reserved_padding));
    uint32_t pri_crc = update_crc32(0U, (const uint8_t *)&primary_hdr, 92U);
    primary_hdr.header_crc32 = pri_crc;

    GptHeaderSector backup_hdr = primary_hdr;
    backup_hdr.header_crc32 = 0U;
    backup_hdr.my_lba = new_backup_hdr_lba;
    backup_hdr.alternate_lba = 1ULL;
    backup_hdr.partition_entry_lba = new_backup_tbl_lba;
    uint32_t bak_crc = update_crc32(0U, (const uint8_t *)&backup_hdr, 92U);
    backup_hdr.header_crc32 = bak_crc;

    uint8_t pmbr[512];
    build_protective_mbr(pmbr, total_sectors);

#ifdef _WIN32
    _fseeki64(fp, 0, SEEK_SET);
#else
    fseeko(fp, 0, SEEK_SET);
#endif
    fwrite(pmbr, 1, sizeof(pmbr), fp);
    fwrite(&primary_hdr, 1, sizeof(primary_hdr), fp);

#ifdef _WIN32
    _fseeki64(fp, (int64_t)(primary_hdr.partition_entry_lba * LOGICAL_SECTOR_SIZE), SEEK_SET);
#else
    fseeko(fp, (off_t)(primary_hdr.partition_entry_lba * LOGICAL_SECTOR_SIZE), SEEK_SET);
#endif
    fwrite(entries, 1, table_bytes, fp);

#ifdef _WIN32
    _fseeki64(fp, (int64_t)(new_backup_tbl_lba * LOGICAL_SECTOR_SIZE), SEEK_SET);
#else
    fseeko(fp, (off_t)(new_backup_tbl_lba * LOGICAL_SECTOR_SIZE), SEEK_SET);
#endif
    fwrite(entries, 1, table_bytes, fp);

#ifdef _WIN32
    _fseeki64(fp, (int64_t)(new_backup_hdr_lba * LOGICAL_SECTOR_SIZE), SEEK_SET);
#else
    fseeko(fp, (off_t)(new_backup_hdr_lba * LOGICAL_SECTOR_SIZE), SEEK_SET);
#endif
    fwrite(&backup_hdr, 1, sizeof(backup_hdr), fp);

    uint8_t *fat_hdr = (uint8_t *)calloc(64U, 512U);
    if (fat_hdr != NULL) {
        if (build_fat32_cidata_superblock(fat_hdr, 64U * 512U, (uint32_t)cidata_sectors, (uint32_t)cidata_start) == 0) {
#ifdef _WIN32
            _fseeki64(fp, (int64_t)(cidata_start * LOGICAL_SECTOR_SIZE), SEEK_SET);
#else
            fseeko(fp, (off_t)(cidata_start * LOGICAL_SECTOR_SIZE), SEEK_SET);
#endif
            fwrite(fat_hdr, 1, 64U * 512U, fp);
        }
        free(fat_hdr);
    }

    fflush(fp);
    free(entries);
    fclose(fp);

    if (out_part_num != NULL) {
        *out_part_num = (uint32_t)(free_slot + 1);
    }
    if (out_cidata_start_lba != NULL) {
        *out_cidata_start_lba = cidata_start;
    }
    if (out_primary_crc != NULL) {
        *out_primary_crc = pri_crc;
    }
    return 0;
}

#ifdef _WIN32
typedef struct {
    HANDLE handles[MAX_LOCKED_VOLUMES];
    int count;
} LockedVolumes;

static bool parse_windows_physical_drive_index(const char *target_path, DWORD *out_disk_index) {
    unsigned long idx = 0xFFFFFFFFUL;
    if (sscanf(target_path, "\\\\.\\PhysicalDrive%lu", &idx) == 1 ||
        sscanf(target_path, "//./PhysicalDrive%lu", &idx) == 1 ||
        sscanf(target_path, "PhysicalDrive%lu", &idx) == 1) {
        *out_disk_index = (DWORD)idx;
        return true;
    }
    return false;
}

static bool verify_windows_usb_triple_lock(DWORD disk_index, uint64_t *out_size_bytes) {
    if (disk_index == 0) {
        return false;
    }
    char phys_path[64];
    snprintf(phys_path, sizeof(phys_path), "\\\\.\\PhysicalDrive%lu", (unsigned long)disk_index);
    HANDLE hDisk = CreateFileA(
        phys_path,
        GENERIC_READ,
        FILE_SHARE_READ | FILE_SHARE_WRITE,
        NULL,
        OPEN_EXISTING,
        0,
        NULL
    );
    if (hDisk == INVALID_HANDLE_VALUE) {
        return false;
    }

    STORAGE_PROPERTY_QUERY query;
    memset(&query, 0, sizeof(query));
    query.PropertyId = StorageDeviceProperty;
    query.QueryType = PropertyStandardQuery;
    uint8_t prop_buf[1024];
    DWORD ret_bytes = 0;
    bool is_usb_or_removable = false;

    if (DeviceIoControl(
            hDisk,
            IOCTL_STORAGE_QUERY_PROPERTY,
            &query,
            sizeof(query),
            prop_buf,
            sizeof(prop_buf),
            &ret_bytes,
            NULL
        )) {
        STORAGE_DEVICE_DESCRIPTOR *desc = (STORAGE_DEVICE_DESCRIPTOR *)prop_buf;
        if (desc->BusType == BusTypeUsb || desc->RemovableMedia) {
            is_usb_or_removable = true;
        }
    }

    DISK_GEOMETRY_EX geom;
    memset(&geom, 0, sizeof(geom));
    if (DeviceIoControl(
            hDisk,
            IOCTL_DISK_GET_DRIVE_GEOMETRY_EX,
            NULL,
            0,
            &geom,
            sizeof(geom),
            &ret_bytes,
            NULL
        )) {
        if (out_size_bytes != NULL) {
            *out_size_bytes = (uint64_t)geom.DiskSize.QuadPart;
        }
    }
    CloseHandle(hDisk);
    return is_usb_or_removable;
}

static int enumerate_windows_usb_disks_json(void) {
    printf("[");
    bool first = true;
    for (DWORD idx = 1; idx < 32; idx++) {
        char phys_path[64];
        snprintf(phys_path, sizeof(phys_path), "\\\\.\\PhysicalDrive%lu", (unsigned long)idx);
        HANDLE hDisk = CreateFileA(
            phys_path,
            GENERIC_READ,
            FILE_SHARE_READ | FILE_SHARE_WRITE,
            NULL,
            OPEN_EXISTING,
            0,
            NULL
        );
        if (hDisk == INVALID_HANDLE_VALUE) {
            continue;
        }
        STORAGE_PROPERTY_QUERY query;
        memset(&query, 0, sizeof(query));
        query.PropertyId = StorageDeviceProperty;
        query.QueryType = PropertyStandardQuery;
        uint8_t prop_buf[1024];
        memset(prop_buf, 0, sizeof(prop_buf));
        DWORD ret_bytes = 0;
        bool is_usb = false;
        bool removable = false;
        const char *vendor = "Generic";
        const char *product = "USB Storage";

        if (DeviceIoControl(hDisk, IOCTL_STORAGE_QUERY_PROPERTY, &query, sizeof(query), prop_buf, sizeof(prop_buf) - 1, &ret_bytes, NULL)) {
            STORAGE_DEVICE_DESCRIPTOR *desc = (STORAGE_DEVICE_DESCRIPTOR *)prop_buf;
            is_usb = (desc->BusType == BusTypeUsb);
            removable = (desc->RemovableMedia != 0);
            if (desc->VendorIdOffset && desc->VendorIdOffset < sizeof(prop_buf)) {
                vendor = (const char *)(prop_buf + desc->VendorIdOffset);
            }
            if (desc->ProductIdOffset && desc->ProductIdOffset < sizeof(prop_buf)) {
                product = (const char *)(prop_buf + desc->ProductIdOffset);
            }
        }
        DISK_GEOMETRY_EX geom;
        memset(&geom, 0, sizeof(geom));
        uint64_t disk_bytes = 0;
        if (DeviceIoControl(hDisk, IOCTL_DISK_GET_DRIVE_GEOMETRY_EX, NULL, 0, &geom, sizeof(geom), &ret_bytes, NULL)) {
            disk_bytes = (uint64_t)geom.DiskSize.QuadPart;
        }
        CloseHandle(hDisk);
        if (!is_usb && !removable) {
            continue;
        }
        if (!first) {
            printf(",");
        }
        first = false;
        printf(
            "{\"disk_index\":%lu,\"device_node\":\"\\\\\\\\.\\\\PhysicalDrive%lu\",\"vendor\":\"%s\",\"product\":\"%s\",\"size_bytes\":%" PRIu64 ",\"bus\":\"usb\",\"removable\":%s}",
            (unsigned long)idx,
            (unsigned long)idx,
            vendor,
            product,
            disk_bytes,
            removable ? "true" : "false"
        );
    }
    printf("]\n");
    return 0;
}

static void lock_and_dismount_volumes(DWORD disk_index, LockedVolumes *lv) {
    lv->count = 0;
    DWORD mask = GetLogicalDrives();
    for (int letter = 0; letter < 26; letter++) {
        if ((mask & (1UL << letter)) == 0) {
            continue;
        }
        if (letter == 2) {
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
        FlushFileBuffers(hVol);
        BOOL locked = FALSE;
        for (int attempt = 0; attempt < 16; attempt++) {
            if (DeviceIoControl(hVol, FSCTL_LOCK_VOLUME, NULL, 0, NULL, 0, &ret_bytes, NULL)) {
                locked = TRUE;
                break;
            }
            Sleep(100);
        }
        DeviceIoControl(hVol, FSCTL_DISMOUNT_VOLUME, NULL, 0, NULL, 0, &ret_bytes, NULL);
        DeviceIoControl(hVol, IOCTL_VOLUME_OFFLINE, NULL, 0, NULL, 0, &ret_bytes, NULL);
        if (locked && lv->count < MAX_LOCKED_VOLUMES) {
            lv->handles[lv->count++] = hVol;
        } else {
            CloseHandle(hVol);
        }
    }
}

static void clear_windows_disk_readonly(HANDLE hTarget) {
    IS_SET_DISK_ATTRIBUTES attrs;
    memset(&attrs, 0, sizeof(attrs));
    attrs.Version = sizeof(attrs);
    attrs.Persist = FALSE;
    attrs.Attributes = 0ULL;
    attrs.AttributesMask = DISK_ATTRIBUTE_READ_ONLY | DISK_ATTRIBUTE_OFFLINE;
    DWORD ret_bytes = 0;
    DeviceIoControl(hTarget, IOCTL_DISK_SET_DISK_ATTRIBUTES, &attrs, sizeof(attrs), NULL, 0, &ret_bytes, NULL);
}

static void unlock_volumes(LockedVolumes *lv) {
    DWORD ret_bytes = 0;
    for (int i = 0; i < lv->count; i++) {
        DeviceIoControl(lv->handles[i], IOCTL_VOLUME_ONLINE, NULL, 0, NULL, 0, &ret_bytes, NULL);
        DeviceIoControl(lv->handles[i], FSCTL_UNLOCK_VOLUME, NULL, 0, NULL, 0, &ret_bytes, NULL);
        CloseHandle(lv->handles[i]);
    }
    lv->count = 0;
}
#else
static int enumerate_posix_usb_disks_json(void) {
    printf("[]\n");
    return 0;
}
#endif

static int flash_image(
    const char *iso_path,
    const char *target_path,
    bool patch_grub,
    bool verify_crc,
    bool inject_cidata_gpt
) {
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
    DWORD disk_idx = 0xFFFFFFFFUL;
    bool is_physical_drive = parse_windows_physical_drive_index(target_path, &disk_idx);
    LockedVolumes lv;
    memset(&lv, 0, sizeof(lv));

    if (is_physical_drive) {
        uint64_t phys_bytes = 0;
        if (!verify_windows_usb_triple_lock(disk_idx, &phys_bytes)) {
            fclose(iso_fp);
            free_aligned_dma(dma_buf);
            free_aligned_dma(ver_buf);
            fprintf(stderr, "{\"error\":\"IS-ERR-101\",\"message\":\"Triple-Lock safety guard blocked PhysicalDrive%lu\"}\n", (unsigned long)disk_idx);
            return 4;
        }
        lock_and_dismount_volumes(disk_idx, &lv);
    }

    DWORD creation_disp = is_physical_drive ? OPEN_EXISTING : CREATE_ALWAYS;
    DWORD flags_and_attrs = is_physical_drive
        ? (FILE_FLAG_NO_BUFFERING | FILE_FLAG_WRITE_THROUGH)
        : (FILE_ATTRIBUTE_NORMAL | FILE_FLAG_WRITE_THROUGH);

    HANDLE hTarget = CreateFileA(
        target_path,
        GENERIC_READ | GENERIC_WRITE,
        FILE_SHARE_READ | FILE_SHARE_WRITE,
        NULL,
        creation_disp,
        flags_and_attrs,
        NULL
    );
    if (hTarget == INVALID_HANDLE_VALUE) {
        unlock_volumes(&lv);
        fclose(iso_fp);
        free_aligned_dma(dma_buf);
        free_aligned_dma(ver_buf);
        fprintf(stderr, "{\"error\":\"IS-ERR-103\",\"message\":\"Win32 CreateFileA failed on %s (err=%lu)\"}\n", target_path, (unsigned long)GetLastError());
        return 4;
    }
    if (is_physical_drive) {
        clear_windows_disk_readonly(hTarget);
        DWORD dasd_bytes = 0;
        DeviceIoControl(hTarget, FSCTL_ALLOW_EXTENDED_DASD_IO, NULL, 0, NULL, 0, &dasd_bytes, NULL);
    }
#else
    int target_fd = open(target_path, O_RDWR | O_CREAT | O_TRUNC, 0644);
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
    Sha256Context sha_ctx;
    sha256_init(&sha_ctx);
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
        if (is_physical_drive && (to_write % SECTOR_ALIGNMENT) != 0) {
            size_t padded = ((to_write + SECTOR_ALIGNMENT - 1U) / SECTOR_ALIGNMENT) * SECTOR_ALIGNMENT;
            memset(dma_buf + to_write, 0, padded - to_write);
            to_write = padded;
        }
        DWORD bw = 0;
        bool write_ok = false;
        for (int retry = 0; retry < 4; retry++) {
            if (WriteFile(hTarget, dma_buf, (DWORD)to_write, &bw, NULL) && (size_t)bw == to_write) {
                write_ok = true;
                break;
            }
            sleep_millis(150U * (unsigned int)(retry + 1));
        }
        if (!write_ok) {
            CloseHandle(hTarget);
            unlock_volumes(&lv);
            fclose(iso_fp);
            free_aligned_dma(dma_buf);
            free_aligned_dma(ver_buf);
            fprintf(stderr, "{\"error\":\"IS-ERR-104\",\"message\":\"WriteFile DMA error\"}\n");
            return 5;
        }
#else
        size_t off = 0;
        while (off < to_write) {
            ssize_t w = write(target_fd, dma_buf + off, to_write - off);
            if (w <= 0) {
                if (errno == EINTR || errno == EAGAIN) {
                    sleep_millis(50U);
                    continue;
                }
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
        sha256_update(&sha_ctx, dma_buf, n_read);
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

    char sha256_hex[65];
    sha256_final_hex(&sha_ctx, sha256_hex);

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
            size_t aligned_chunk = is_physical_drive
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
        printf("{\"stage\":\"VERIFIED\",\"pct\":100,\"crc32\":\"%08" PRIX32 "\",\"sha256\":\"%s\",\"grub_patches\":%zu}\n",
               read_crc, sha256_hex, patches);
        fflush(stdout);
    }

#ifdef _WIN32
    if (is_physical_drive) {
        DWORD upd_bytes = 0;
        DeviceIoControl(hTarget, IOCTL_DISK_UPDATE_PROPERTIES, NULL, 0, NULL, 0, &upd_bytes, NULL);
    }
    CloseHandle(hTarget);
    unlock_volumes(&lv);
#else
    close(target_fd);
#endif
    fclose(iso_fp);
    free_aligned_dma(dma_buf);
    free_aligned_dma(ver_buf);

    if (inject_cidata_gpt) {
        uint32_t part_num = 0;
        uint64_t cidata_lba = 0;
        uint32_t gpt_crc = 0;
        if (relocate_gpt_and_inject_cidata_file(target_path, 16U, &part_num, &cidata_lba, &gpt_crc) == 0) {
            printf("{\"stage\":\"GPT_RELOCATED\",\"cidata_partition\":%u,\"cidata_start_lba\":%" PRIu64 ",\"gpt_crc32\":\"%08" PRIX32 "\"}\n",
                   part_num, cidata_lba, gpt_crc);
            fflush(stdout);
        }
    }
    return 0;
}

static int create_synthetic_gpt_iso_image(const char *path, uint32_t total_mb) {
    init_crc32_table();
    uint64_t total_bytes = (uint64_t)total_mb * 1024ULL * 1024ULL;
    uint64_t total_sectors = total_bytes / LOGICAL_SECTOR_SIZE;
    uint64_t iso_sectors = (4ULL * 1024ULL * 1024ULL) / LOGICAL_SECTOR_SIZE;

    FILE *fp = fopen(path, "wb");
    if (fp == NULL) {
        return -1;
    }
    uint8_t zero_page[4096];
    memset(zero_page, 0, sizeof(zero_page));
    for (uint64_t written = 0; written < total_bytes; written += sizeof(zero_page)) {
        if (fwrite(zero_page, 1, sizeof(zero_page), fp) != sizeof(zero_page)) {
            fclose(fp);
            return -2;
        }
    }

    GptPartitionEntry *entries = (GptPartitionEntry *)calloc(GPT_ENTRY_COUNT, sizeof(GptPartitionEntry));
    if (entries == NULL) {
        fclose(fp);
        return -3;
    }
    memcpy(entries[0].partition_type_guid, GUID_EFI_SYSTEM_LE, 16U);
    generate_deterministic_guid(entries[0].unique_partition_guid, 2048ULL, iso_sectors - 64ULL);
    entries[0].starting_lba = 2048ULL;
    entries[0].ending_lba = iso_sectors - 64ULL;
    encode_utf16le_label(entries[0].partition_name, "ISO9660");

    uint32_t entries_crc = update_crc32(0U, (const uint8_t *)entries, GPT_ENTRY_COUNT * sizeof(GptPartitionEntry));

    GptHeaderSector hdr;
    memset(&hdr, 0, sizeof(hdr));
    hdr.signature = GPT_HEADER_SIGNATURE;
    hdr.revision = GPT_REVISION_1_0;
    hdr.header_size = 92U;
    hdr.my_lba = 1ULL;
    hdr.alternate_lba = iso_sectors - 1ULL;
    hdr.first_usable_lba = 34ULL;
    hdr.last_usable_lba = iso_sectors - GPT_TABLE_SECTORS - 2ULL;
    generate_deterministic_guid(hdr.disk_guid, total_sectors, iso_sectors);
    hdr.partition_entry_lba = 2ULL;
    hdr.num_partition_entries = GPT_ENTRY_COUNT;
    hdr.size_of_partition_entry = GPT_ENTRY_SIZE;
    hdr.partition_entry_array_crc32 = entries_crc;
    hdr.header_crc32 = update_crc32(0U, (const uint8_t *)&hdr, 92U);

    uint8_t pmbr[512];
    build_protective_mbr(pmbr, total_sectors);

#ifdef _WIN32
    _fseeki64(fp, 0, SEEK_SET);
#else
    fseeko(fp, 0, SEEK_SET);
#endif
    fwrite(pmbr, 1, sizeof(pmbr), fp);
    fwrite(&hdr, 1, sizeof(hdr), fp);
    fwrite(entries, 1, GPT_ENTRY_COUNT * sizeof(GptPartitionEntry), fp);

    char grub_block[512];
    memset(grub_block, ' ', sizeof(grub_block));
    const char *grub_sample = "set timeout=30\nlinux /casper/vmlinuz quiet ---                                                    ";
    memcpy(grub_block + 32, grub_sample, strlen(grub_sample));
#ifdef _WIN32
    _fseeki64(fp, (int64_t)(2048ULL * LOGICAL_SECTOR_SIZE), SEEK_SET);
#else
    fseeko(fp, (off_t)(2048ULL * LOGICAL_SECTOR_SIZE), SEEK_SET);
#endif
    fwrite(grub_block, 1, sizeof(grub_block), fp);

    fflush(fp);
    free(entries);
    fclose(fp);
    return 0;
}

static int run_self_test(void) {
    const char *src_tmp = "win_flasher_selftest_src.bin";
    const char *dst_tmp = "win_flasher_selftest_dst.bin";
    if (create_synthetic_gpt_iso_image(src_tmp, 24U) != 0) {
        return 1;
    }
    int rc = flash_image(src_tmp, dst_tmp, true, true, true);
    remove(src_tmp);
    remove(dst_tmp);
    return rc;
}

int main(int argc, char **argv) {
    if (argc == 2 && strcmp(argv[1], "--self-test") == 0) {
        return run_self_test();
    }
    if (argc == 2 && strcmp(argv[1], "--list-usb") == 0) {
#ifdef _WIN32
        return enumerate_windows_usb_disks_json();
#else
        return enumerate_posix_usb_disks_json();
#endif
    }
    const char *iso_path = NULL;
    const char *target_path = NULL;
    bool patch_grub = true;
    bool verify_crc = true;
    bool inject_gpt = false;

    for (int i = 1; i < argc; i++) {
        if (strcmp(argv[i], "--iso") == 0 && i + 1 < argc) {
            iso_path = argv[++i];
        } else if (strcmp(argv[i], "--target") == 0 && i + 1 < argc) {
            target_path = argv[++i];
        } else if (strcmp(argv[i], "--no-patch") == 0) {
            patch_grub = false;
        } else if (strcmp(argv[i], "--no-verify") == 0) {
            verify_crc = false;
        } else if (strcmp(argv[i], "--inject-cidata-gpt") == 0) {
            inject_gpt = true;
        }
    }
    if (iso_path == NULL || target_path == NULL) {
        fprintf(stderr, "Usage: %s --iso <path> --target <device> [--no-patch] [--no-verify] [--inject-cidata-gpt] | --list-usb | --self-test\n", argv[0]);
        return 1;
    }
    return flash_image(iso_path, target_path, patch_grub, verify_crc, inject_gpt);
}