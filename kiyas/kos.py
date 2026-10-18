"""TR hız kıyas seti: aynı programları TR, Python ve Go ile çalıştırıp süreleri karşılaştırır.

Kullanım (proje kökünden):  python kiyas/kos.py [--tekrar 3] [--yalniz tr] [test ...]

Her program kendi ölçtüğü süreyi (ms) ve bir sonuç değeri yazar; sonuç değerlerinin üç dilde
aynı olması beklenir. Yorumlayıcı ve Go programı her çalıştırmada geçici bir klasöre yeniden
derlenir (GOAMD64 ortamda verilmemişse v3 ile: 2020 sonrası x86-64 işlemciler, AVX2/FMA; daha eski
işlemcilerde GOAMD64=v1 verin). matris_yerlesik testinin Python sürümü numpy ister; yoksa atlanır.
"""
import argparse, json, os, subprocess, sys, tempfile

KOK = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
KIYAS = os.path.join(KOK, "kiyas")
TESTLER = ["dongu", "fib", "sirala", "metin", "sozluk", "matris_dongu", "matris_yerlesik"]
UZANTI = ".exe" if os.name == "nt" else ""


def derle(gecici):
    tr = os.path.join(gecici, "tr" + UZANTI)
    go = os.path.join(gecici, "kiyas" + UZANTI)
    ortam = dict(os.environ)
    ortam.setdefault("GOAMD64", "v3")
    subprocess.run(["go", "build", "-o", tr, "./cmd/tr"], cwd=KOK, check=True, env=ortam)
    subprocess.run(["go", "build", "-o", go, "./kiyas/go"], cwd=KOK, check=True, env=ortam)
    return tr, go


def kos(komut, tekrar):
    en_iyi, deger = None, None
    for _ in range(tekrar):
        r = subprocess.run(komut, capture_output=True, text=True, encoding="utf-8", timeout=600, cwd=KOK)
        satirlar = r.stdout.strip().splitlines()
        if r.returncode != 0 or not satirlar:
            return None, (r.stderr.strip().splitlines() or ["hata"])[-1]
        parca = satirlar[-1].split()
        ms = int(parca[-1])
        en_iyi = ms if en_iyi is None else min(en_iyi, ms)
        deger = parca[1]
    return en_iyi, deger


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("testler", nargs="*", default=TESTLER)
    ap.add_argument("--tekrar", type=int, default=3)
    ap.add_argument("--yalniz", choices=["tr", "py", "go"], help="yalnızca bu dili çalıştır")
    ap.add_argument("--json", help="sonuçları bu dosyaya yaz")
    a = ap.parse_args()
    diller = [a.yalniz] if a.yalniz else ["tr", "py", "go"]
    sonuc = {}
    with tempfile.TemporaryDirectory() as gecici:
        tr, go = derle(gecici)
        for t in a.testler:
            komutlar = {
                "tr": [tr, os.path.join(KIYAS, "tr", t + ".tr")],
                "py": [sys.executable, os.path.join(KIYAS, "py", "kiyas.py"), t],
                "go": [go, t],
            }
            sonuc[t] = {d: kos(komutlar[d], a.tekrar) for d in diller}
            parcalar = []
            for d in diller:
                ms, deger = sonuc[t][d]
                parcalar.append(f"{d.upper():>2} {'-' if ms is None else ms:>7} ms")
            degerler = {v for _, v in sonuc[t].values()}
            satir = f"{t:16} " + " | ".join(parcalar)
            if "tr" in diller and "py" in diller and sonuc[t]["tr"][0] and sonuc[t]["py"][0]:
                satir += f" | TR/Python {sonuc[t]['tr'][0] / max(sonuc[t]['py'][0], 1):5.1f}x"
            if len(diller) > 1:
                satir += " | sonuç aynı" if len(degerler) == 1 else f" | SONUÇ FARKLI {degerler}"
            print(satir, flush=True)
    if a.json:
        with open(a.json, "w", encoding="utf-8") as f:
            json.dump(sonuc, f, ensure_ascii=False, indent=1)


if __name__ == "__main__":
    main()
