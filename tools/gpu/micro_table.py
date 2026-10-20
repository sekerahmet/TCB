# Mikro kıyas çıktılarını (TR: gpu_sinama -test.run TestMicroBench, karşılaştırma:
# micro_compare.py) tek tabloya çevirir. "MIKRO|" ile başlamayan satırlar yok sayılır.
# Kullanım: python micro_table.py mikro_tr.txt mikro_py.txt
import sys

TR_YOLLAR = ("cublaslt", "kendi", "birleşik", "çok tensörlü")


def oku(yollar):
    olcum, sira = {}, []
    for yol in yollar:
        with open(yol, encoding="utf-8") as f:
            for satir in f:
                if not satir.startswith("MIKRO|"):
                    continue
                p = satir.rstrip("\n").split("|")
                if len(p) < 6 or p[1] in ("bilgi", "bellek"):
                    if len(p) >= 6:
                        print(f"# {p[1]}: {p[2]} {p[4]} {p[5]}")
                    continue
                islem, kaynak, ms, deger, birim = p[1], p[2], float(p[3]), float(p[4]), p[5]
                if islem not in olcum:
                    olcum[islem] = {}
                    sira.append(islem)
                olcum[islem][kaynak] = (ms, deger, birim)
    return olcum, sira


def hucre(o):
    return f"{o[0]:.3f} ms · {o[1]:.0f}" if o else "—"


def main():
    sys.stdout.reconfigure(encoding="utf-8")
    olcum, sira = oku(sys.argv[1:])
    print("| işlem | TR kitaplık | TR kendi/birleşik | PyTorch | TR en iyi / PyTorch (süre) | birim |")
    print("|---|---|---|---|---|---|")
    for islem in sira:
        o = olcum[islem]
        lt = o.get("cublaslt")
        kendi = next((o[y] for y in TR_YOLLAR[1:] if y in o), None)
        py = o.get("pytorch")
        tr = [x for x in (lt, kendi) if x]
        oran = "—"
        if tr and py:
            oran = f"{min(x[0] for x in tr) / py[0]:.2f}"
        birim = next((x[2] for x in (lt, kendi, py) if x), "")
        print(f"| {islem} | {hucre(lt)} | {hucre(kendi)} | {hucre(py)} | {oran} | {birim} |")


if __name__ == "__main__":
    main()
