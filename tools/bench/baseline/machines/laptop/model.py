"""Dizüstü ölçü betiklerinin model bağlayıcısı (machines/laptop/lib.sh olcu_model): questions.py MODELS'taki <ad>'ın
dosyalarını dizüstünde yalnız OLCU_MODELLER'de bulur (<ad>/<dosya>, <ad>/<adı>, <adı>; Ahmet'in düzeni: dizüstünün bütün
modelleri %USERPROFILE%/models altındadır, TCB 23:49), iş klasörüne
models/<ad>/<dosya> olarak sabit bağlantıyla (olmazsa kopya) koyar ve sha256'yı denetler: Dost'un kataloğu
(tools/assistant/katalog.json, depo + revision + yol), yoksa Hugging Face üstverisi (LFS dosyasının sha256'sı). LFS
olmayan küçük dosyanın sha256'sı depoda yazılı değildir; o dosya yalnız varlığıyla geçer ve çıktıda öyle yazılır.
Bulunamayan ya da sha256'sı tutmayan dosyada çıkış 2: model yok, betik ölçmez. Hiçbir şey indirilmez.
    python -I model.py <repo> <iş> <modeller> <ad>
"""
import hashlib, importlib.util, json, os, shutil, sys, urllib.parse, urllib.request
sys.stdout.reconfigure(encoding="utf-8")
repo, work, models, ad = sys.argv[1:5]
spec = importlib.util.spec_from_file_location("q", os.path.join(repo, "tools", "bench", "baseline", "questions.py"))
q = importlib.util.module_from_spec(spec)
spec.loader.exec_module(q)
hub, rev, files = q.MODELS[ad]
katalog = json.load(open(os.path.join(repo, "tools", "assistant", "katalog.json"), encoding="utf-8"))
known = {(e["source"]["repo"], e["source"]["revision"], x["path"]): x["sha256"]
         for e in katalog["entries"] for x in e["source"]["files"]}


def hub_sha(paths):
    data = urllib.parse.urlencode([("paths", p) for p in paths] + [("expand", "true")]).encode()
    req = urllib.request.Request(f"https://huggingface.co/api/models/{hub}/paths-info/{rev}", data=data,
                                 headers={"User-Agent": "tr-bench"})
    return {x["path"]: (x.get("lfs") or {}).get("oid") for x in json.load(urllib.request.urlopen(req, timeout=60))}


def sha256(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for b in iter(lambda: f.read(1 << 20), b""):
            h.update(b)
    return h.hexdigest()


need = [f for f in files if (hub, rev, f) not in known]
online = hub_sha(need) if need else {}
bad = 0
for f in files:
    cands = [os.path.join(models, ad, f), os.path.join(models, ad, os.path.basename(f)), os.path.join(models, os.path.basename(f))]
    # Klasör adı koşturucununkinden farklı olabilir (dizüstünde silero-vad, koşturucuda silero-vad-16k): alt klasörlerde adıyla.
    cands += sorted(os.path.join(models, d, os.path.basename(f)) for d in os.listdir(models)
                    if os.path.isdir(os.path.join(models, d)))
    src = next((c for c in cands if os.path.isfile(c)), None)
    if not src:
        print(f"YOK {ad}/{f}: aranan {cands}")
        bad += 1
        continue
    want = known.get((hub, rev, f)) or online.get(f)
    got = sha256(src) if want else None
    if want and got != want:
        print(f"SHA256 TUTMADI {ad}/{f}: {got} != {want} ({src})")
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
    print(f"{ad}/{f}: {how}, sha256 {got[:12] + ' (' + ('katalog' if (hub, rev, f) in known else 'hub') + ')' if want else 'yazılı değil (LFS değil)'}")
print(f"model {ad}: {hub} @ {rev[:12]}")
sys.exit(2 if bad else 0)
