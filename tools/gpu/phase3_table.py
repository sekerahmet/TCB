# Evre 3 ölçümü: phase3_torch.py (eager, compile, max-autotune) ve phase3_tr.tr çıktılarını tek tabloya döker.
# Kullanım: python phase3_table.py tr.json eager.json compile.json autotune.json [--tr-bellek tr_profil.json]
# Kart bilgisi (ad, bellek, sürücü, CUDA, torch) torch çıktılarından okunur. --tr-bellek: `tr profil --json`
# raporu (TR'nin tepe belleği buradan: gpu.bellek[].tepe_canlı_bayt).
import argparse, json, sys


def oku(yol):
    with open(yol, encoding="utf-8") as f:
        return json.load(f)


def tr_tepe_gb(rapor):
    """tr profil --json raporundan GPU tepe canlı belleği (GB); yoksa None."""
    bellek = rapor.get("gpu", {}).get("bellek") or []
    if not bellek:
        return None
    return max(b["tepe_canlı_bayt"] for b in bellek) / 2**30


def kayip_sapma(a, b):
    """İki kayıp eğrisinin ortak uzunluktaki en büyük mutlak farkı."""
    n = min(len(a), len(b))
    return max((abs(a[i] - b[i]) for i in range(n)), default=float("nan"))


def tablo(sonuclar, tr_bellek, kart):
    satirlar = []
    if kart:
        satirlar.append(f"kart: {kart['ad']} · {kart['bellek_gb']} GB · sürücü {kart.get('surucu', '?')} · "
                        f"CUDA {kart['cuda']} · torch {kart['torch']}")
    satirlar += ["", "| kip | ilk adım (ms) | medyan (ms/adım) | p90 (ms/adım) | tepe bellek (GB) | kayıp[0] | kayıp[son] | eager'dan en büyük kayıp farkı |",
                 "|---|---|---|---|---|---|---|---|"]
    eager = next((s for s in sonuclar if s["kip"] == "eager"), None)
    for s in sonuclar:
        o = s["olculen"]
        bellek = s.get("tepe_bellek_gb")
        if s["araç"] == "tr":
            bellek = tr_tepe_gb(tr_bellek) if tr_bellek else None
        egri = s["kayip_egrisi"]
        fark = "—"
        if eager is not None and s is not eager:
            fark = f"{kayip_sapma(egri, eager['kayip_egrisi']):.4f}"
        bm = f"{bellek:.2f}" if bellek is not None else "—"
        ad = "TR" if s["araç"] == "tr" else "PyTorch " + s["kip"]
        satirlar.append(f"| {ad} | {s['ilk_adim_ms']:.0f} | {o['medyan_ms']:.2f} | {o['p90_ms']:.2f} | {bm} | "
                        f"{egri[0]:.4f} | {egri[-1]:.4f} | {fark} |")
    ref = next((s for s in sonuclar if s["kip"] == "compile"), eager)
    tr = next((s for s in sonuclar if s["araç"] == "tr"), None)
    if tr and ref:
        satirlar += ["", f"TR / PyTorch {ref['kip']} (medyan süre): "
                         f"{tr['olculen']['medyan_ms'] / ref['olculen']['medyan_ms']:.2f}  (< 1: TR daha hızlı)"]
    return "\n".join(satirlar)


def main(argv):
    ap = argparse.ArgumentParser()
    ap.add_argument("dosyalar", nargs="+")
    ap.add_argument("--tr-bellek", default="")
    arg = ap.parse_args(argv)
    sonuclar = [oku(y) for y in arg.dosyalar]
    kart = next((s["kart"] for s in sonuclar if "kart" in s), None)
    sys.stdout.reconfigure(encoding="utf-8")
    print(tablo(sonuclar, oku(arg.tr_bellek) if arg.tr_bellek else None, kart))


if __name__ == "__main__":
    main(sys.argv[1:])
