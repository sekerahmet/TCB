# Evre 3 ölçümü: bir Nsight Systems raporundan (.sqlite) GPU çekirdek özeti çıkarır.
# Kullanım: nsys export --type sqlite --force-overwrite true -o r.sqlite r.nsys-rep
#           python phase3_nsys.py r.sqlite --window-ms 1500 --steps 10 [--ad TR] [--json r.json]
# Pencere: son çekirdeğin bittiği andan geriye pencere-ms (programın son adımları ölçülen adımlardır;
# başlangıç ve derleme boşlukları dışarıda kalır). --steps penceredeki adım sayısıdır (çekirdek/adım için).
# Boşluk oranı: pencerede hiçbir çekirdeğin çalışmadığı sürenin payı (çekirdekler birleştirilir; bellek
# kopyaları sayılmaz). Çıktı: kısa tablo; --json ile ayrıca makine okur biçimi.
import argparse, json, sqlite3, sys


def birlesik_sure(aralik):
    """(bas, bit) aralıklarının birleşiminin süresi ve aralarındaki boşluklar (ns)."""
    toplam, bosluk, bit = 0, [], None
    for b, e in sorted(aralik):
        if bit is None or b > bit:
            if bit is not None:
                bosluk.append(b - bit)
            toplam += e - b
            bit = e
        elif e > bit:
            toplam += e - bit
            bit = e
    return toplam, bosluk


def oku(yol, pencere_ms):
    db = sqlite3.connect(yol)
    sutunlar = {r[1] for r in db.execute("PRAGMA table_info(CUPTI_ACTIVITY_KIND_KERNEL)")}
    ad = "demangledName" if "demangledName" in sutunlar else "shortName"
    satir = db.execute(f"SELECT k.start, k.end, s.value FROM CUPTI_ACTIVITY_KIND_KERNEL k "
                       f"JOIN StringIds s ON s.id = k.{ad}").fetchall()
    if not satir:
        return []
    son = max(r[1] for r in satir)
    bas = son - int(pencere_ms * 1e6)
    return [r for r in satir if r[0] >= bas]


def ozet(satir, adim):
    if not satir:
        return None
    ilk, son = min(r[0] for r in satir), max(r[1] for r in satir)
    pencere = son - ilk
    mesgul, bosluk = birlesik_sure([(r[0], r[1]) for r in satir])
    sureler = {}
    for b, e, ad in satir:
        c, t = sureler.get(ad, (0, 0))
        sureler[ad] = (c + 1, t + e - b)
    toplam_cekirdek = sum(t for _, t in sureler.values())
    en_pahali = sorted(sureler.items(), key=lambda kv: -kv[1][1])[:10]
    bosluk.sort()
    return {"kernel_count": len(satir), "kernels_per_step": len(satir) / adim if adim else None,
            "window_ms": pencere / 1e6, "kernel_total_ms": toplam_cekirdek / 1e6,
            "gpu_idle_ratio": 1 - mesgul / pencere if pencere else float("nan"),
            "gap_median_us": bosluk[len(bosluk) // 2] / 1e3 if bosluk else 0.0,
            "gaps_over_10us": sum(1 for g in bosluk if g > 10_000),
            "top_kernels": [{"name": ad, "calls": c, "total_ms": t / 1e6, "share": t / toplam_cekirdek}
                          for ad, (c, t) in en_pahali]}


def yaz(o, ad):
    if o is None:
        return f"{ad}: pencerede çekirdek yok"
    s = [f"| {ad} | değer |", "|---|---|", f"| çekirdek sayısı (pencere) | {o['kernel_count']} |"]
    if o["kernels_per_step"] is not None:
        s.append(f"| çekirdek / adım | {o['kernels_per_step']:.0f} |")
    s += [f"| pencere (ms) | {o['window_ms']:.1f} |",
          f"| toplam çekirdek süresi (ms) | {o['kernel_total_ms']:.1f} |",
          f"| GPU boşta oranı | %{o['gpu_idle_ratio'] * 100:.1f} |",
          f"| çekirdekler arası boşluk, medyan (µs) | {o['gap_median_us']:.1f} |",
          f"| 10 µs'den uzun boşluk sayısı | {o['gaps_over_10us']} |", "",
          "| en pahalı 10 çekirdek | çağrı | toplam (ms) | pay |", "|---|---|---|---|"]
    for k in o["top_kernels"]:
        s.append(f"| {k['name'][:80].replace('|', '/')} | {k['calls']} | {k['total_ms']:.2f} | %{k['share'] * 100:.1f} |")
    return "\n".join(s)


def main(argv):
    ap = argparse.ArgumentParser()
    ap.add_argument("sqlite")
    ap.add_argument("--window-ms", type=float, default=1500)
    ap.add_argument("--steps", type=int, default=0)
    ap.add_argument("--name", default="")
    ap.add_argument("--json", default="")
    arg = ap.parse_args(argv)
    o = ozet(oku(arg.sqlite, arg.window_ms), arg.steps)
    sys.stdout.reconfigure(encoding="utf-8")
    print(yaz(o, arg.name or arg.sqlite))
    if arg.json:
        with open(arg.json, "w", encoding="utf-8") as f:
            json.dump(o, f, ensure_ascii=False)


if __name__ == "__main__":
    main(sys.argv[1:])
