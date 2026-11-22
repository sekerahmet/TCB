"""Dizüstü ölçü betiklerinin model bağlayıcısı (machines/laptop/lib.sh olcu_model; Ş3, DENETİM 944): <ad>'ın her
dosyası sources.json'da tek yerde sabittir: depo, revision, OLCU_MODELLER'deki yeri, boyu, sha256'sı (LFS) ya da git blob
kimliği (LFS olmayan küçük dosya). Dosya yalnız o yerde aranır, başka klasöre ya da başka modelin aynı adlı dosyasına
düşülmez; yoksa, boyu ya da kimliği tutmazsa çıkış 2. Ağa çıkılmaz, hiçbir şey indirilmez. Tutan dosya iş klasörüne
models/<ad>/<dosya> olarak sabit bağlantıyla (olmazsa kopya) konur; bağlanan yol ve kimlik çıktıya (kayda) yazılır.
    python -I model.py <sources.json> <iş> <modeller> <ad>
"""
import hashlib, json, os, shutil, sys
sys.stdout.reconfigure(encoding="utf-8")
sources, work, models, ad = sys.argv[1:5]
ent = json.load(open(sources, encoding="utf-8"))["models"].get(ad)
if ent is None:
    print(f"YOK {ad}: sources.json'da kaydı yok")
    sys.exit(2)


def digest(path, git_blob):
    h = hashlib.sha1(b"blob %d\0" % os.path.getsize(path)) if git_blob else hashlib.sha256()
    with open(path, "rb") as f:
        for b in iter(lambda: f.read(1 << 20), b""):
            h.update(b)
    return h.hexdigest()


bad = 0
for f, rec in ent["files"].items():
    src = os.path.join(models, *rec["path"].split("/"))
    kind = "sha256" if "sha256" in rec else "git_blob"
    if not os.path.isfile(src):
        print(f"YOK {ad}/{f}: {rec['path']} (sources.json)")
        bad += 1
        continue
    if os.path.getsize(src) != rec["size"]:
        print(f"BOY TUTMADI {ad}/{f}: {os.path.getsize(src)} != {rec['size']} ({rec['path']})")
        bad += 1
        continue
    got = digest(src, kind == "git_blob")
    if got != rec[kind]:
        print(f"{kind.upper()} TUTMADI {ad}/{f}: {got} != {rec[kind]} ({rec['path']})")
        bad += 1
        continue
    dst = os.path.join(work, "models", ad, f)
    os.makedirs(os.path.dirname(dst), exist_ok=True)
    if os.path.exists(dst):
        os.remove(dst)
    try:
        os.link(src, dst)
        how = "bağlantı"
    except OSError:
        shutil.copyfile(src, dst)
        how = "kopya"
    print(f"{ad}/{f}: {how} <- modeller/{rec['path']}, {kind} {got[:12]}")
print(f"model {ad}: {ent['repo']} @ {ent['revision'][:12]}")
sys.exit(2 if bad else 0)
