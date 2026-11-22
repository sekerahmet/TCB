"""Dizüstü ölçü betiklerinin emsal denetimi (machines/laptop/lib.sh olcu_arac, olcu_surum; Ş3, DENETİM 944): emsal ikilisinin
sha256'sı ve sürüm satırı, Python paketlerinin sürümü sources.json'dakiyle aynı olmalı; farklıysa ya da yoksa çıkış 2.
    python -I sources.py <sources.json> tool <llama.cpp|whisper.cpp> <dizin>
    python -I sources.py <sources.json> python <paket>...
"""
import hashlib, importlib.metadata as md, json, os, subprocess, sys
sys.stdout.reconfigure(encoding="utf-8")
src, kind, *rest = sys.argv[1:]
s = json.load(open(src, encoding="utf-8"))
VERSION_CMD = {"llama.cpp": ("llama-completion.exe", "--version"), "whisper.cpp": ("whisper-cli.exe", "--version")}


def sha256(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for b in iter(lambda: f.read(1 << 20), b""):
            h.update(b)
    return h.hexdigest()


bad = []
if kind == "tool":
    name, folder = rest
    t = s["tools"][name]
    for f, want in t["files"].items():
        p = os.path.join(folder, f)
        got = sha256(p) if os.path.isfile(p) else None
        if got != want:
            bad.append(f"{name} {f}: sha256 {got} != {want} ({p})")
        else:
            print(f"{name} {f}: sha256 {got[:12]} tuttu")
    exe, flag = VERSION_CMD[name]
    r = subprocess.run([os.path.join(folder, exe), flag], capture_output=True, text=True, stdin=subprocess.DEVNULL)
    line = next((l for l in (r.stdout + r.stderr).splitlines() if t["version"] in l), None)
    if line is None:
        bad.append(f"{name}: sürüm satırında {t['version']!r} yok: {(r.stdout + r.stderr).strip()[-200:]!r}")
    else:
        print(f"{name} sürüm: {line.strip()} ({t['release']})")
elif kind == "python":
    for name in rest:
        want = s["python"].get(name)
        try:
            got = md.version(name)
        except md.PackageNotFoundError:
            got = None
        if want is None or got != want["version"]:
            bad.append(f"{name}: kurulu {got}, sources.json {want and want['version']}")
        else:
            print(f"{name} {got} ({want.get('wheel')}, sha256 {want.get('sha256', '?')[:12]})")
    if "torch" in rest and not bad:
        import torch
        print("torch aygıtı: cpu (cuda", torch.cuda.is_available(), "), iş parçacığı", torch.get_num_threads())
for b in bad:
    print("TUTMADI " + b)
sys.exit(2 if bad else 0)
