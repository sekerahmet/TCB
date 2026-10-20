# Evre 3 ölçümü, PyTorch tarafı: phase3_tr.tr ile aynı model, veri ve iyileştirici (phase3_config.json).
# Üç kip: eager, compile (varsayılan), max-autotune. Her kip ayrı süreçte koşulur:
#   python phase3_torch.py --kip eager|compile|max-autotune [--ayar phase3_config.json] [--cikti dosya.json]
#                          [--nvtx] (ölçülen adımları cudaProfilerStart/Stop ve NVTX ile işaretler)
# Çıktı (JSON): kart bilgisi, ilk derleme süresi, ısınma adımları, ölçülen adımların medyan ve p90 süresi,
# tepe bellek ve ilk kayip_adim adımın kayıp eğrisi. Her adım sonunda eşzamanlanır (TR tarafı da öyle).
import argparse, json, math, statistics, subprocess, sys, time


def ayarla(argv):
    ap = argparse.ArgumentParser()
    ap.add_argument("--kip", required=True, choices=["eager", "compile", "max-autotune"])
    ap.add_argument("--ayar", default="phase3_config.json")
    ap.add_argument("--cikti", default="")
    ap.add_argument("--nvtx", action="store_true")
    return ap.parse_args(argv)


def yuzdelik(degerler, p):
    """p. yüzdelik (en yakın sıra yöntemi)."""
    s = sorted(degerler)
    return s[max(0, math.ceil(p / 100 * len(s)) - 1)]


def ozet(sureler):
    return {"medyan_ms": statistics.median(sureler), "p90_ms": yuzdelik(sureler, 90),
            "ortalama_ms": statistics.fmean(sureler), "en_az_ms": min(sureler), "en_cok_ms": max(sureler)}


def sentetik(B, T, V, adim, torch, cihaz):
    """phase3_tr.tr ile aynı formül: 8 sabit toplu iş, adım % 8."""
    s = adim % 8
    w = [[(s * 7919 + i * 104729 + t * 31) % V for t in range(T + 1)] for i in range(B)]
    return torch.tensor(w, dtype=torch.int64, device=cihaz)


def kos(arg):
    import torch, torch.nn as nn, torch.nn.functional as F
    c = json.load(open(arg.ayar, encoding="utf-8"))
    B, T, D, H, FF, L, V = c["B"], c["T"], c["D"], c["H"], c["FF"], c["KATMAN"], c["V"]
    cihaz = "cuda"

    class Blok(nn.Module):
        def __init__(s):
            super().__init__()
            s.g1, s.g2 = nn.RMSNorm(D), nn.RMSNorm(D)
            s.q, s.k, s.v, s.o = (nn.Linear(D, D, bias=False) for _ in range(4))
            s.wg, s.wu, s.w2 = nn.Linear(D, FF, bias=False), nn.Linear(D, FF, bias=False), nn.Linear(FF, D, bias=False)

        def forward(s, x):
            h = s.g1(x)
            bas = lambda t: t.view(B, T, H, D // H).transpose(1, 2)
            y = F.scaled_dot_product_attention(bas(s.q(h)), bas(s.k(h)), bas(s.v(h)), is_causal=True)
            x = x + s.o(y.transpose(1, 2).reshape(B, T, D))
            m = s.g2(x)
            return x + s.w2(F.silu(s.wg(m)) * s.wu(m))

    class Model(nn.Module):
        def __init__(s):
            super().__init__()
            s.E = nn.Embedding(V, D)
            nn.init.normal_(s.E.weight, 0, 0.02)
            s.bloklar = nn.ModuleList(Blok() for _ in range(L))
            s.gs = nn.RMSNorm(D)

        def forward(s, i):
            x = s.E(i)
            for b in s.bloklar:
                x = b(x)
            return s.gs(x) @ s.E.weight.T

    torch.manual_seed(1)
    m = Model().to(cihaz)
    op = torch.optim.AdamW(m.parameters(), lr=c["lr"], weight_decay=0.0)
    ileri = m if arg.kip == "eager" else torch.compile(m, mode=None if arg.kip == "compile" else "max-autotune")
    bf16 = c["hassasiyet"] == "bf16"
    toplam, isinma, olculen, kayip_adim = c["isinma"] + c["olculen"], c["isinma"], c["olculen"], c["kayip_adim"]
    torch.cuda.reset_peak_memory_stats()

    def adim_at(adim):
        w = sentetik(B, T, V, adim, torch, cihaz)
        op.zero_grad(set_to_none=True)
        with torch.autocast("cuda", dtype=torch.bfloat16, enabled=bf16):
            kayip = F.cross_entropy(ileri(w[:, :-1]).reshape(-1, V), w[:, 1:].reshape(-1))
        kayip.backward()
        torch.nn.utils.clip_grad_norm_(m.parameters(), 1.0)
        op.step()
        return kayip

    kayiplar, isinma_ms = [], []
    for adim in range(isinma):
        t0 = time.perf_counter()
        kayip = adim_at(adim)
        v = kayip.item()  # eşzamanlar
        isinma_ms.append((time.perf_counter() - t0) * 1000)
        if adim < kayip_adim:
            kayiplar.append(v)
    olc = []
    if arg.nvtx:
        torch.cuda.profiler.start()
    for adim in range(isinma, toplam):
        if arg.nvtx:
            torch.cuda.nvtx.range_push(f"adim{adim}")
        t0 = time.perf_counter()
        kayip = adim_at(adim)
        v = kayip.item()
        olc.append((time.perf_counter() - t0) * 1000)
        if arg.nvtx:
            torch.cuda.nvtx.range_pop()
        if adim < kayip_adim:
            kayiplar.append(v)
    if arg.nvtx:
        torch.cuda.profiler.stop()
    # Kayıp eğrisi isinma + olculen'den kısaysa kayip_adim kadarı değil, koşulan kadarı yazılır.
    p = torch.cuda.get_device_properties(0)
    try:
        surucu = subprocess.check_output(["nvidia-smi", "--query-gpu=driver_version", "--format=csv,noheader"], text=True).split()[0]
    except Exception:
        surucu = "?"
    return {"araç": "pytorch", "kip": arg.kip, "hassasiyet": c["hassasiyet"],
            "kart": {"ad": p.name, "bellek_gb": round(p.total_memory / 2**30, 1), "torch": torch.__version__,
                     "cuda": torch.version.cuda, "surucu": surucu},
            "ayar": {k: c[k] for k in ("B", "T", "D", "H", "FF", "KATMAN", "V")},
            "ilk_adim_ms": isinma_ms[0],  # derleme dahil ilk adım (eager'de de yazılır)
            "isinma_ms": isinma_ms, "olculen": ozet(olc),
            "tepe_bellek_gb": torch.cuda.max_memory_allocated() / 2**30,
            "kayip_egrisi": kayiplar}


def main(argv):
    arg = ayarla(argv)
    sonuc = kos(arg)
    metin = json.dumps(sonuc, ensure_ascii=False)
    if arg.cikti:
        open(arg.cikti, "w", encoding="utf-8").write(metin)
    print(metin)


if __name__ == "__main__":
    main(sys.argv[1:])
