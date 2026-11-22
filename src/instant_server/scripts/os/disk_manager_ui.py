"""
disk_manager_ui.py
------------------
Standalone dark-mode HTML/CSS/JS frontend for the Instant Server Web Disk Manager.
"""

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