# GPU kıyası, karşılaştırma tarafı: kiyas_gpu.tr ile aynı model ve ayarlar (CUDA).
# Kullanım: python kiyas_gpu.py --boyut kucuk|buyuk [--veri KLASOR] [--derleme yok|var|ikisi]
# --veri bir klasördür (içindeki ilk .npy dosyası okunur); verilmezse sabit sentetik toplu
# işler kullanılır. Derlemeli ölçümde derleme süresi ısınma adımlarında kalır.
# Çıktı: ısınma hariç ms/adım ve son kayıp. Veri yalnızca okunur.
import argparse, glob, os, time
import numpy as np, torch, torch.nn as nn, torch.nn.functional as F

BOYUTLAR = {
    "kucuk": dict(B=8, T=64, D=128, H=4, FF=256, L=2, ADIM=220, ISINMA=20),
    "buyuk": dict(B=32, T=256, D=512, H=8, FF=2048, L=6, ADIM=220, ISINMA=20),
}
V = 40000

ap = argparse.ArgumentParser()
ap.add_argument("--boyut", default="kucuk", choices=BOYUTLAR)
ap.add_argument("--veri", default="")
ap.add_argument("--derleme", default="ikisi", choices=["yok", "var", "ikisi"])
arg = ap.parse_args()
a = BOYUTLAR[arg.boyut]
B, T, D, H, FF, L = a["B"], a["T"], a["D"], a["H"], a["FF"], a["L"]
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


def toplu_isler():
    """Toplu iş üreteci: veriden rastgele pencereler ya da sabit sentetik toplu işler."""
    if arg.veri:
        dosyalar = sorted(glob.glob(os.path.join(arg.veri, "*.npy")))
        veri = np.load(dosyalar[0], mmap_mode="r")
        rng = np.random.default_rng(1)
        while True:
            k = rng.integers(0, len(veri) - T - 1, B)
            w = np.stack([np.asarray(veri[j:j + T + 1], dtype=np.int64) for j in k])
            yield torch.from_numpy(w).to(cihaz, non_blocking=True)
    sabit = []
    for s in range(8):
        w = [[(s * 7919 + i * 104729 + t * 31) % V for t in range(T + 1)] for i in range(B)]
        sabit.append(torch.tensor(w, dtype=torch.int64, device=cihaz))
    adim = 0
    while True:
        yield sabit[adim % 8]
        adim += 1


def olc(derle):
    torch.manual_seed(1)
    m = Model().to(cihaz)
    op = torch.optim.AdamW(m.parameters(), lr=0.003, weight_decay=0.0)
    ileri = torch.compile(m) if derle else m
    kaynak = toplu_isler()
    for adim in range(a["ADIM"]):
        if adim == a["ISINMA"]:
            torch.cuda.synchronize()
            bas = time.perf_counter()
        w = next(kaynak)
        op.zero_grad(set_to_none=True)
        kayip = F.cross_entropy(ileri(w[:, :-1]).reshape(-1, V), w[:, 1:].reshape(-1))
        kayip.backward()
        torch.nn.utils.clip_grad_norm_(m.parameters(), 1.0)
        op.step()
    son = kayip.item()
    torch.cuda.synchronize()
    sure = (time.perf_counter() - bas) * 1000 / (a["ADIM"] - a["ISINMA"])
    ad = "torch.compile ile" if derle else "derlemesiz"
    print(f"PyTorch ({ad}): {sure:.2f} ms/adım · son kayıp {son:.4f} · en çok bellek {torch.cuda.max_memory_allocated() / 2**30:.2f} GB")


print(f"kart: {torch.cuda.get_device_name()} · torch {torch.__version__} · ayar: {arg.boyut} B={B} T={T} D={D} H={H} FF={FF} katman={L} V={V}")
if arg.derleme in ("yok", "ikisi"):
    olc(False)
if arg.derleme in ("var", "ikisi"):
    torch.cuda.reset_peak_memory_stats()
    olc(True)
