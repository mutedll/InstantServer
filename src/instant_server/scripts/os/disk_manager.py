#!/usr/bin/env python3
"""
disk_manager.py
Windows Disk Management–grade Partition & Disk Selector for Instant Server.
Runs as root (with sudo/root privilege verification) during Live USB boot
(port 8088) and from the running Server Dashboard.

Features:
- Visual disk & partition map (proportional allocated + unallocated bars)
- Initialize / Convert Disk (GPT or MBR)
- Clean / Wipe Disk (wipefs + sgdisk --zap-all)
- Create New Simple Volume from unallocated space (ext4/ntfs/fat32/exfat/btrfs/xfs + label)
- Format Partition, Shrink / Split Partition, Extend Partition, Delete Partition, Relabel
- Validation-gated "Select as Install Target" button (greyed out with exact reason
  if <15 GB, protected installer USB, read-only, or invalid)
"""

import http.server
import json
import os
import pathlib
import re
import shutil
import subprocess
import sys
import urllib.parse

PORT = int(os.environ.get("INSTANT_DISK_MANAGER_PORT", "8088"))
MIN_TARGET_BYTES = 15 * 1024 * 1024 * 1024  # 15 GB minimum for Ubuntu Server
USB_EXCLUDE = set(
    p.strip() for p in os.environ.get("INSTANT_USB_DISKS", "").split(",") if p.strip()
)
PICK_OUT = pathlib.Path(os.environ.get("INSTANT_PICK_OUT", "/tmp/instant_picked_disk"))
INSTALLER_MODE = os.environ.get("INSTANT_INSTALLER_MODE", "1") == "1"


def is_root() -> bool:
    geteuid = getattr(os, "geteuid", None)
    return callable(geteuid) and geteuid() == 0


def run_cmd(cmd: list[str], timeout: int = 120) -> tuple[bool, str]:
    try:
        res = subprocess.run(cmd, capture_output=True, text=True, timeout=timeout)
        out = (res.stdout + "\n" + res.stderr).strip()
        return res.returncode == 0, out
    except Exception as exc:
        return False, str(exc)


def valid_dev_path(path: str) -> bool:
    return bool(re.match(r"^/dev/(sd[a-z]+[0-9]*|nvme[0-9]+n[0-9]+(p[0-9]+)?|vd[a-z]+[0-9]*|mmcblk[0-9]+(p[0-9]+)?|hd[a-z]+[0-9]*)$", path))


def is_protected_usb(dev_path: str, tran: str = "", rm: bool = False) -> bool:
    for u in USB_EXCLUDE:
        if dev_path == u or dev_path.startswith(u):
            return True
    if INSTALLER_MODE and (tran.lower() == "usb" or rm):
        return True
    return False


def get_free_gaps(disk_path: str) -> list[dict]:
    """Use parted -m unit B print free to find unallocated gaps >= 100 MB."""
    ok, out = run_cmd(["parted", "-s", "-m", disk_path, "unit", "B", "print", "free"], timeout=20)
    gaps = []
    if not ok:
        return gaps
    for line in out.splitlines():
        parts = line.rstrip(";").split(":")
        if len(parts) >= 5 and parts[4] == "free":
            try:
                start_b = int(parts[1].rstrip("B"))
                end_b = int(parts[2].rstrip("B"))
                size_b = int(parts[3].rstrip("B"))
                if size_b >= 100 * 1024 * 1024:
                    gaps.append({"start_b": start_b, "end_b": end_b, "size_b": size_b})
            except ValueError:
                pass
    return gaps


def evaluate_target(size_b: int, protected: bool, ro: bool, dtype: str) -> tuple[bool, str]:
    if protected:
        return False, "Protected: Installer USB drive (cannot install onto boot USB)"
    if ro:
        return False, "Read-only media (cannot write OS)"
    if dtype not in ("disk", "part", "free"):
        return False, f"Unsupported device type ({dtype})"
    if size_b < MIN_TARGET_BYTES:
        gb = size_b / (1024 ** 3)
        return False, f"Too small ({gb:.1f} GB — minimum 15.0 GB required)"
    return True, "Valid install target"


def list_disks() -> list[dict]:
    ok, out = run_cmd([
        "lsblk", "-J", "-b", "-o",
        "NAME,PATH,SIZE,TYPE,FSTYPE,LABEL,MOUNTPOINT,MODEL,TRAN,RM,RO,PTTYPE"
    ], timeout=20)
    if not ok:
        return []
    try:
        raw = json.loads(out)
    except Exception:
        return []
    disks = []
    for d in raw.get("blockdevices", []):
        if d.get("type") != "disk":
            continue
        name = str(d.get("name", ""))
        if re.match(r"^(loop|ram|zram|sr|fd|md|dm-)", name):
            continue
        path = str(d.get("path") or f"/dev/{name}")
        size_b = int(d.get("size") or 0)
        tran = str(d.get("tran") or "")
        rm = bool(d.get("rm"))
        ro = bool(d.get("ro"))
        prot = is_protected_usb(path, tran, rm)
        can_sel, reason = evaluate_target(size_b, prot, ro, "disk")
        parts = []
        for ch in d.get("children", []) or []:
            psize = int(ch.get("size") or 0)
            ppath = str(ch.get("path") or f"/dev/{ch.get('name', '')}")
            p_can, p_reason = evaluate_target(psize, prot, bool(ch.get("ro")), "part")
            parts.append({
                "name": ch.get("name", ""),
                "path": ppath,
                "size_b": psize,
                "fstype": ch.get("fstype") or "RAW / Unformatted",
                "label": ch.get("label") or "",
                "mountpoint": ch.get("mountpoint") or "",
                "can_select": p_can,
                "select_reason": p_reason,
            })
        gaps = [] if prot else get_free_gaps(path)
        for g in gaps:
            g_can, g_reason = evaluate_target(g["size_b"], prot, ro, "free")
            g["can_select"] = g_can
            g["select_reason"] = g_reason
        disks.append({
            "name": name,
            "path": path,
            "model": (d.get("model") or "Internal Storage").strip(),
            "size_b": size_b,
            "tran": tran or "sata/nvme",
            "pttype": d.get("pttype") or "Not Initialized",
            "protected": prot,
            "ro": ro,
            "can_select": can_sel,
            "select_reason": reason,
            "partitions": parts,
            "free_gaps": gaps,
        })
    return disks


def unmount_device_tree(dev: str) -> None:
    ok, out = run_cmd(["lsblk", "-lnpo", "NAME,MOUNTPOINT", dev], timeout=15)
    if ok:
        for ln in out.splitlines():
            cols = ln.split(None, 1)
            if len(cols) == 2 and cols[1].strip():
                run_cmd(["umount", "-l", cols[0]], timeout=15)
    run_cmd(["swapoff", dev], timeout=10)


def refresh_kernel_pt(disk: str) -> None:
    run_cmd(["partprobe", disk], timeout=15)
    run_cmd(["udevadm", "settle", "--timeout=10"], timeout=15)


def format_partition_fs(part: str, fstype: str, label: str = "") -> tuple[bool, str]:
    clean_lbl = re.sub(r"[^A-Za-z0-9_-]", "", label)[:16]
    unmount_device_tree(part)
    fs = fstype.lower()
    if fs == "ext4":
        cmd = ["mkfs.ext4", "-F"] + (["-L", clean_lbl] if clean_lbl else []) + [part]
    elif fs == "fat32":
        cmd = ["mkfs.vfat", "-F", "32"] + (["-n", clean_lbl[:11].upper()] if clean_lbl else []) + [part]
    elif fs == "ntfs":
        cmd = ["mkfs.ntfs", "-Q", "-F"] + (["-L", clean_lbl] if clean_lbl else []) + [part]
    elif fs == "exfat":
        cmd = ["mkfs.exfat"] + (["-n", clean_lbl] if clean_lbl else []) + [part]
    elif fs == "btrfs":
        cmd = ["mkfs.btrfs", "-f"] + (["-L", clean_lbl] if clean_lbl else []) + [part]
    elif fs == "xfs":
        cmd = ["mkfs.xfs", "-f"] + (["-L", clean_lbl[:12]] if clean_lbl else []) + [part]
    else:
        return False, f"Unsupported filesystem: {fstype}"
    return run_cmd(cmd, timeout=120)


def handle_disk_action(payload: dict) -> dict:
    if not is_root():
        return {"ok": False, "error": "Root/sudo privileges are required to modify disks and partitions."}
    op = str(payload.get("action", "")).strip()
    target = str(payload.get("device", "")).strip()
    if not valid_dev_path(target):
        return {"ok": False, "error": f"Invalid block device path: {target}"}
    if is_protected_usb(target):
        return {"ok": False, "error": f"Refused: {target} is the protected Installer USB drive."}

    if op == "initialize":
        style = "gpt" if str(payload.get("style", "gpt")).lower() == "gpt" else "msdos"
        unmount_device_tree(target)
        ok, out = run_cmd(["parted", "-s", target, "mklabel", style])
        refresh_kernel_pt(target)
        return {"ok": ok, "output": out or f"Initialized {target} as {style.upper()}."}

    if op == "wipe":
        unmount_device_tree(target)
        run_cmd(["wipefs", "-a", target])
        ok, out = run_cmd(["sgdisk", "--zap-all", target])
        if not ok:
            ok, out = run_cmd(["parted", "-s", target, "mklabel", "gpt"])
        refresh_kernel_pt(target)
        return {"ok": ok, "output": out or f"Wiped all partitions & signatures on {target}."}

    if op == "create_volume":
        start_b = int(payload.get("start_b", 1048576))
        size_gb = float(payload.get("size_gb", 20))
        fstype = str(payload.get("fstype", "ext4"))
        label = str(payload.get("label", ""))
        end_b = start_b + int(size_gb * 1024 * 1024 * 1024)
        ok, out = run_cmd(["parted", "-s", "-a", "optimal", target, "mkpart", "primary", f"{start_b}B", f"{end_b}B"])
        refresh_kernel_pt(target)
        if not ok:
            return {"ok": False, "error": out}
        # Find newest partition on target
        _, p_out = run_cmd(["lsblk", "-lnpo", "NAME,TYPE", target])
        parts = [ln.split()[0] for ln in p_out.splitlines() if len(ln.split()) == 2 and ln.split()[1] == "part"]
        if parts and fstype.lower() != "unformatted":
            f_ok, f_out = format_partition_fs(parts[-1], fstype, label)
            return {"ok": f_ok, "output": f_out or f"Created & formatted {parts[-1]} as {fstype}."}
        return {"ok": True, "output": f"Created new volume on {target}."}

    if op == "format":
        fstype = str(payload.get("fstype", "ext4"))
        label = str(payload.get("label", ""))
        ok, out = format_partition_fs(target, fstype, label)
        return {"ok": ok, "output": out or f"Formatted {target} as {fstype}."}

    if op == "delete_partition":
        m = re.match(r"^(/dev/(?:nvme\d+n\d+|mmcblk\d+))p(\d+)$|^(/dev/[a-z]+)(\d+)$", target)
        if not m:
            return {"ok": False, "error": f"Cannot parse partition number from {target}"}
        parent = m.group(1) or m.group(3)
        pnum = m.group(2) or m.group(4)
        unmount_device_tree(target)
        ok, out = run_cmd(["parted", "-s", parent, "rm", pnum])
        refresh_kernel_pt(parent)
        return {"ok": ok, "output": out or f"Deleted partition {target}."}

    if op in ("shrink", "extend"):
        new_gb = float(payload.get("new_size_gb", 0))
        if new_gb < 1:
            return {"ok": False, "error": "New partition size must be at least 1 GB."}
        m = re.match(r"^(/dev/(?:nvme\d+n\d+|mmcblk\d+))p(\d+)$|^(/dev/[a-z]+)(\d+)$", target)
        if not m:
            return {"ok": False, "error": f"Cannot parse partition number from {target}"}
        parent = m.group(1) or m.group(3)
        pnum = m.group(2) or m.group(4)
        unmount_device_tree(target)
        _, fs_out = run_cmd(["blkid", "-o", "value", "-s", "TYPE", target])
        fs = fs_out.strip().lower()
        new_mb = int(new_gb * 1024)
        if op == "shrink":
            if fs.startswith("ext"):
                run_cmd(["e2fsck", "-fy", target], timeout=180)
                ok, out = run_cmd(["resize2fs", target, f"{new_mb}M"], timeout=180)
                if not ok:
                    return {"ok": False, "error": out}
            elif fs == "ntfs" and shutil.which("ntfsresize"):
                ok, out = run_cmd(["ntfsresize", "-f", "-s", f"{new_mb}M", target], timeout=180)
                if not ok:
                    return {"ok": False, "error": out}
            ok, out = run_cmd(["parted", "-s", "---pretend-input-tty", parent, "resizepart", pnum, f"{new_mb}MiB"])
            refresh_kernel_pt(parent)
            return {"ok": ok, "output": out or f"Shrunk {target} to {new_gb:.1f} GB (freed unallocated space)."}
        else:
            ok, out = run_cmd(["parted", "-s", parent, "resizepart", pnum, f"{new_mb}MiB"])
            refresh_kernel_pt(parent)
            if ok and fs.startswith("ext"):
                run_cmd(["e2fsck", "-fy", target], timeout=180)
                run_cmd(["resize2fs", target], timeout=180)
            elif ok and fs == "ntfs" and shutil.which("ntfsresize"):
                run_cmd(["ntfsresize", "-f", target], timeout=180)
            return {"ok": ok, "output": out or f"Extended {target} to {new_gb:.1f} GB."}

    if op == "relabel":
        label = re.sub(r"[^A-Za-z0-9_-]", "", str(payload.get("label", "")))[:16]
        _, fs_out = run_cmd(["blkid", "-o", "value", "-s", "TYPE", target])
        fs = fs_out.strip().lower()
        if fs.startswith("ext"):
            ok, out = run_cmd(["e2label", target, label])
        elif fs == "vfat":
            ok, out = run_cmd(["fatlabel", target, label[:11].upper()])
        elif fs == "ntfs":
            ok, out = run_cmd(["ntfslabel", target, label])
        else:
            return {"ok": False, "error": f"Relabel not supported for filesystem: {fs or 'unknown'}"}
        return {"ok": ok, "output": out or f"Updated label on {target} to '{label}'."}

    if op == "select_target":
        mode = str(payload.get("mode", "whole"))
        disks = list_disks()
        chosen_ok = False
        reason = "Target not found"
        for d in disks:
            if d["path"] == target:
                chosen_ok, reason = d["can_select"], d["select_reason"]
                break
            for p in d["partitions"]:
                if p["path"] == target:
                    chosen_ok, reason = p["can_select"], p["select_reason"]
                    break
        if not chosen_ok:
            return {"ok": False, "error": f"Cannot select {target}: {reason}"}
        PICK_OUT.write_text(f"{target}|{mode}\n", encoding="utf-8")
        return {"ok": True, "selected": target, "mode": mode, "output": f"Selected {target} ({mode}). Continuing Ubuntu Server installation..."}

    return {"ok": False, "error": f"Unknown disk action: {op}"}


DISK_UI_HTML = """<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8"/><meta name="viewport" content="width=device-width,initial-scale=1"/>
<title>Instant Server — Disk Management & Target Selector</title>
<style>
:root{--bg:#0d1017;--card:#151a24;--border:#262f40;--text:#f1f5f9;--sub:#94a3b8;--accent:#0a84ff;--ok:#30d158;--warn:#ff9f0a;--err:#ff453a;}
*{box-sizing:border-box;font-family:system-ui,-apple-system,Segoe UI,Roboto,sans-serif;}
body{margin:0;background:var(--bg);color:var(--text);padding:16px;}
.wrap{max-width:1080px;margin:0 auto;display:flex;flex-direction:column;gap:14px;}
.hdr{display:flex;align-items:center;justify-content:space-between;flex-wrap:wrap;gap:10px;background:var(--card);border:1px solid var(--border);border-radius:12px;padding:14px 16px;}
.disk-card{background:var(--card);border:1px solid var(--border);border-radius:12px;padding:14px;display:flex;flex-direction:column;gap:10px;}
.disk-top{display:flex;justify-content:space-between;align-items:center;flex-wrap:wrap;gap:8px;}
.bar{display:flex;height:48px;border-radius:8px;overflow:hidden;border:1px solid var(--border);background:#0b0e14;}
.seg{display:flex;flex-direction:column;justify-content:center;padding:4px 8px;font-size:11px;overflow:hidden;white-space:nowrap;border-right:1px solid #0d1017;min-width:68px;transition:filter .15s;}
.seg:hover{filter:brightness(1.15);}
.seg-part{background:linear-gradient(180deg,#1d4ed8,#1e3a8a);}
.seg-free{background:repeating-linear-gradient(45deg,#1e2430,#1e2430 8px,#141821 8px,#141821 16px);color:var(--sub);}
.seg-prot{background:#3b1d24;color:#fca5a5;}
.btn{background:#222a38;color:var(--text);border:1px solid var(--border);border-radius:7px;padding:6px 10px;font-size:12px;font-weight:600;cursor:pointer;transition:all .15s;}
.btn:hover:not(:disabled){background:#2d3748;border-color:var(--accent);}
.btn-accent{background:var(--accent);border-color:var(--accent);color:#fff;}
.btn-ok{background:var(--ok);border-color:var(--ok);color:#09110d;}
.btn-danger{background:rgba(255,69,58,.15);border-color:rgba(255,69,58,.45);color:#ff6961;}
.btn:disabled{opacity:.42;cursor:not-allowed;}
.tbl{width:100%;border-collapse:collapse;font-size:12px;}
.tbl th,.tbl td{text-align:left;padding:7px 8px;border-bottom:1px solid var(--border);}
.pill{font-size:11px;padding:2px 7px;border-radius:99px;background:#222a38;color:var(--sub);}
.pill-err{background:rgba(255,69,58,.15);color:#ff6961;}
.pill-ok{background:rgba(48,209,88,.15);color:#30d158;}
.toast{position:fixed;bottom:16px;right:16px;background:#1e293b;border:1px solid var(--accent);padding:10px 14px;border-radius:9px;display:none;max-width:420px;font-size:12.5px;}
</style></head><body>
<div class="wrap">
  <div class="hdr">
    <div><div style="font-size:17px;font-weight:700;">Instant Server — Disk Management &amp; Target Selector</div>
    <div style="font-size:12px;color:var(--sub);" id="privBanner">Checking root disk permissions...</div></div>
    <button class="btn btn-accent" onclick="loadDisks()">Refresh Disks</button>
  </div>
  <div id="disksRoot">Loading block devices...</div>
</div>
<div class="toast" id="toast"></div>
<script>
const fmtGB = b => (b / 1073741824).toFixed(1) + " GB";
function showMsg(t){const el=document.getElementById("toast");el.textContent=t;el.style.display="block";setTimeout(()=>el.style.display="none",4500);}
async function api(url,body){const r=await fetch(url,body?{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(body)}:{});return r.json();}
async function doAct(body,confirmText){
  if(confirmText && !confirm(confirmText)) return;
  const res=await api("/api/disk/action",body);
  showMsg(res.ok ? (res.output||"Done") : ("Error: "+(res.error||res.output)));
  await loadDisks();
}
async function promptFormat(dev){
  const fs=prompt("Format "+dev+" — enter filesystem (ext4, ntfs, fat32, exfat, btrfs, xfs):","ext4");
  if(!fs) return;
  const lbl=prompt("Optional Volume Label (leave blank for none):","");
  await doAct({action:"format",device:dev,fstype:fs,label:lbl||""},"Erase all data on "+dev+" and format as "+fs+"?");
}
async function promptCreate(disk,startB,maxB){
  const maxGB=(maxB/1073741824).toFixed(1);
  const sz=prompt("New Simple Volume on "+disk+" — size in GB (max "+maxGB+" GB):",maxGB);
  if(!sz) return;
  const fs=prompt("Filesystem (ext4, ntfs, fat32, exfat, btrfs, xfs):","ext4");
  if(!fs) return;
  const lbl=prompt("Volume Label (optional):","DATA");
  await doAct({action:"create_volume",device:disk,start_b:startB,size_gb:parseFloat(sz),fstype:fs,label:lbl||""});
}
async function promptResize(dev,op,curB){
  const curGB=(curB/1073741824).toFixed(1);
  const sz=prompt((op==="shrink"?"Shrink/Split ":"Extend ")+dev+" (current "+curGB+" GB) — enter NEW total size in GB:",curGB);
  if(!sz) return;
  await doAct({action:op,device:dev,new_size_gb:parseFloat(sz)},"Resize "+dev+" to "+sz+" GB?");
}
async function loadDisks(){
  const data=await api("/api/disk/list");
  document.getElementById("privBanner").innerHTML = data.is_root
    ? '<span class="pill pill-ok">Root Disk Privileges Active</span> Manage volumes, format, split, initialize GPT/MBR, and pick your target.'
    : '<span class="pill pill-err">Non-Root Mode</span> Run with sudo/root to execute partition operations.';
  const root=document.getElementById("disksRoot");
  root.innerHTML=(data.disks||[]).map(d=>{
    const total=Math.max(1,d.size_b);
    const segs=[
      ...d.partitions.map(p=>`<div class="seg ${d.protected?'seg-prot':'seg-part'}" style="flex:${Math.max(8,Math.round(p.size_b/total*100))}" title="${p.path} ${fmtGB(p.size_b)} (${p.fstype})"><b>${p.name}</b><span>${fmtGB(p.size_b)} • ${p.fstype}</span></div>`),
      ...d.free_gaps.map(g=>`<div class="seg seg-free" style="flex:${Math.max(8,Math.round(g.size_b/total*100))}" title="Unallocated ${fmtGB(g.size_b)}"><b>Unallocated</b><span>${fmtGB(g.size_b)}</span></div>`)
    ].join("") || `<div class="seg seg-free" style="flex:100"><b>Unallocated / Empty Disk</b><span>${fmtGB(d.size_b)}</span></div>`;
    const partRows=d.partitions.map(p=>`<tr>
      <td><b>${p.path}</b></td><td>${p.label||"—"}</td><td>${p.fstype}</td><td>${fmtGB(p.size_b)}</td><td>${p.mountpoint||"—"}</td>
      <td style="display:flex;gap:5px;flex-wrap:wrap;">
        <button class="btn" ${d.protected?"disabled":""} onclick="promptFormat('${p.path}')">Format</button>
        <button class="btn" ${d.protected?"disabled":""} onclick="promptResize('${p.path}','shrink',${p.size_b})">Shrink/Split</button>
        <button class="btn" ${d.protected?"disabled":""} onclick="promptResize('${p.path}','extend',${p.size_b})">Extend</button>
        <button class="btn btn-danger" ${d.protected?"disabled":""} onclick="doAct({action:'delete_partition',device:'${p.path}'},'Delete partition ${p.path}?')">Delete</button>
      </td></tr>`).join("");
    const gapRows=d.free_gaps.map(g=>`<tr>
      <td><i>Unallocated Space</i></td><td>—</td><td>Free</td><td>${fmtGB(g.size_b)}</td><td>—</td>
      <td style="display:flex;gap:5px;flex-wrap:wrap;">
        <button class="btn btn-accent" ${d.protected?"disabled":""} onclick="promptCreate('${d.path}',${g.start_b},${g.size_b})">+ New Simple Volume</button>
        <button class="btn btn-ok" ${g.can_select?"":"disabled"} title="${g.select_reason}" onclick="doAct({action:'select_target',device:'${d.path}',mode:'gap'})">Select Free Space${g.can_select?"":" ("+g.select_reason+")"}</button>
      </td></tr>`).join("");
    return `<div class="disk-card">
      <div class="disk-top">
        <div><b>${d.path}</b> — ${d.model} <span class="pill">${fmtGB(d.size_b)}</span> <span class="pill">${d.pttype}</span> ${d.protected?'<span class="pill pill-err">Installer USB Protected</span>':''}</div>
        <div style="display:flex;gap:6px;flex-wrap:wrap;">
          <button class="btn" ${d.protected?"disabled":""} onclick="doAct({action:'initialize',device:'${d.path}',style:'gpt'},'Initialize ${d.path} as GPT (erases partition table)?')">Init GPT</button>
          <button class="btn" ${d.protected?"disabled":""} onclick="doAct({action:'initialize',device:'${d.path}',style:'msdos'},'Initialize ${d.path} as MBR (erases partition table)?')">Init MBR</button>
          <button class="btn btn-danger" ${d.protected?"disabled":""} onclick="doAct({action:'wipe',device:'${d.path}'},'WIPE ALL DATA on ${d.path}?')">Clean / Wipe</button>
          <button class="btn btn-ok" ${d.can_select?"":"disabled"} title="${d.select_reason}" onclick="doAct({action:'select_target',device:'${d.path}',mode:'whole'})">Select Whole Disk${d.can_select?"":" — "+d.select_reason}</button>
        </div>
      </div>
      <div class="bar">${segs}</div>
      <table class="tbl"><thead><tr><th>Volume</th><th>Label</th><th>File System</th><th>Size</th><th>Mount</th><th>Actions</th></tr></thead>
      <tbody>${partRows}${gapRows || '<tr><td colspan="6" style="color:var(--sub);">No partitions yet — click Init GPT or Select Whole Disk.</td></tr>'}</tbody></table>
    </div>`;
  }).join("") || "<p>No disks found.</p>";
}
loadDisks();
</script></body></html>"""


class DiskManagerHandler(http.server.BaseHTTPRequestHandler):
    def log_message(self, format: str, *args: object) -> None:
        return

    def _json(self, obj: dict, status: int = 200) -> None:
        raw = json.dumps(obj).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Cache-Control", "no-store")
        self.send_header("Content-Length", str(len(raw)))
        self.end_headers()
        self.wfile.write(raw)

    def do_GET(self) -> None:
        path = urllib.parse.urlsplit(self.path).path
        if path == "/api/disk/list":
            self._json({"ok": True, "is_root": is_root(), "disks": list_disks()})
            return
        raw = DISK_UI_HTML.encode("utf-8")
        self.send_response(200)
        self.send_header("Content-Type", "text/html; charset=utf-8")
        self.send_header("Content-Length", str(len(raw)))
        self.end_headers()
        self.wfile.write(raw)

    def do_POST(self) -> None:
        path = urllib.parse.urlsplit(self.path).path
        ln = int(self.headers.get("Content-Length", "0") or "0")
        body = json.loads(self.rfile.read(ln).decode("utf-8") or "{}") if ln > 0 else {}
        if path == "/api/disk/action":
            self._json(handle_disk_action(body))
            return
        self._json({"ok": False, "error": "Not found"}, status=404)


if __name__ == "__main__":
    srv = http.server.ThreadingHTTPServer(("0.0.0.0", PORT), DiskManagerHandler)
    srv.serve_forever()