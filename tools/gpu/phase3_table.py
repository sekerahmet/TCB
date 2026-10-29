# Evre 3 ölçümü: phase3_torch.py (eager, compile, max-autotune) ve phase3_tr.tr çıktılarını tek tabloya döker.
# Kullanım: python phase3_table.py tr.json eager.json compile.json autotune.json [--tr-memory tr_profil.json]
# Kart bilgisi (ad, bellek, sürücü, CUDA, torch) torch çıktılarından okunur. --tr-memory: `tr profile --json`
# raporu (TR'nin tepe belleği buradan: gpu.bellek[].tepe_canlı_bayt).
import argparse, json, sys


def oku(yol):
    with open(yol, encoding="utf-8") as f:
        return json.load(f)


def tr_tepe_gb(rapor):
    """tr profile --json raporundan GPU tepe canlı belleği (GB); yoksa None."""
    bellek = rapor.get("gpu", {}).get("memory") or []
    if not bellek:
        return None
    return max(b["peak_live_bytes"] for b in bellek) / 2**30


def kayip_sapma(a, b):
    """İki kayıp eğrisinin ortak uzunluktaki en büyük mutlak farkı."""
    n = min(len(a), len(b))
    return max((abs(a[i] - b[i]) for i in range(n)), default=float("nan"))


def tablo(sonuclar, tr_bellek, kart):
    satirlar = []
    if kart:
        satirlar.append(f"kart: {kart['name']} · {kart['memory_gb']} GB · sürücü {kart.get('driver', '?')} · "
                        f"CUDA {kart['cuda']} · torch {kart['torch']}")
    satirlar += ["", "| kip | ilk adım (ms) | medyan (ms/adım) | p90 (ms/adım) | tepe bellek (GB) | kayıp[0] | kayıp[son] | eager'dan en büyük kayıp farkı |",
                 "|---|---|---|---|---|---|---|---|"]
    eager = next((s for s in sonuclar if s["mode"] == "eager"), None)
    for s in sonuclar:
        o = s["measured"]
        bellek = s.get("peak_memory_gb")
        if s["tool"] == "tr":
            bellek = tr_tepe_gb(tr_bellek) if tr_bellek else None
        egri = s["loss_curve"]
        fark = "—"
        if eager is not None and s is not eager:
            fark = f"{kayip_sapma(egri, eager['loss_curve']):.4f}"
        bm = f"{bellek:.2f}" if bellek is not None else "—"
        ad = "TR" if s["tool"] == "tr" else "PyTorch " + s["mode"]
        satirlar.append(f"| {ad} | {s['first_step_ms']:.0f} | {o['median_ms']:.2f} | {o['p90_ms']:.2f} | {bm} | "
                        f"{egri[0]:.4f} | {egri[-1]:.4f} | {fark} |")
    ref = next((s for s in sonuclar if s["mode"] == "compile"), eager)
    tr = next((s for s in sonuclar if s["tool"] == "tr"), None)
    if tr and ref:
        satirlar += ["", f"TR / PyTorch {ref['mode']} (medyan süre): "
                         f"{tr['measured']['median_ms'] / ref['measured']['median_ms']:.2f}  (< 1: TR daha hızlı)"]
    return "\n".join(satirlar)


def main(argv):
    ap = argparse.ArgumentParser()
    ap.add_argument("files", nargs="+")
    ap.add_argument("--tr-memory", default="")
    arg = ap.parse_args(argv)
    sonuclar = [oku(y) for y in arg.files]
    kart = next((s["card"] for s in sonuclar if "card" in s), None)
    sys.stdout.reconfigure(encoding="utf-8")
    print(tablo(sonuclar, oku(arg.tr_memory) if arg.tr_memory else None, kart))


if __name__ == "__main__":
    main(sys.argv[1:])
