# GPU kıyası, karşılaştırma tarafı: compare_gpu.tr ile aynı model ve ayarlar (CUDA).
# Kullanım: python compare_gpu.py --boyut kucuk|buyuk|gpt [--veri KLASOR] [--derleme yok|var|ikisi]
#           [--hassasiyet float32|bf16]  (bf16: ileri yön ve kayıp autocast ile, ağırlıklar float32)
# --veri bir klasördür (içindeki ilk .npy dosyası okunur); verilmezse sabit sentetik toplu
# işler kullanılır. Derlemeli ölçümde derleme süresi ısınma adımlarında kalır.
# Çıktı: ısınma hariç ms/adım ve son kayıp. Veri yalnızca okunur.
import argparse, glob, os, time
import numpy as np, torch
from torch_bench_model import egitim_adimi, model_sinifi, sentetik_toplu

BOYUTLAR = {
    "kucuk": dict(B=8, T=64, D=128, H=4, FF=256, L=2, ADIM=220, ISINMA=20),
    "buyuk": dict(B=32, T=256, D=512, H=8, FF=2048, L=6, ADIM=220, ISINMA=20),
    "gpt": dict(B=16, T=1024, D=768, H=12, FF=2048, L=12, ADIM=60, ISINMA=10),
}
V = 40000

ap = argparse.ArgumentParser()
ap.add_argument("--boyut", default="kucuk", choices=BOYUTLAR)
ap.add_argument("--veri", default="")
ap.add_argument("--derleme", default="ikisi", choices=["yok", "var", "ikisi"])
ap.add_argument("--hassasiyet", default="float32", choices=["float32", "bf16"])
arg = ap.parse_args()
a = BOYUTLAR[arg.boyut]
B, T, D, H, FF, L = a["B"], a["T"], a["D"], a["H"], a["FF"], a["L"]
cihaz = "cuda"


Model = model_sinifi(B, T, D, H, FF, L, V)


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
    sabit = [sentetik_toplu(B, T, V, s, cihaz) for s in range(8)]
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
        kayip = egitim_adimi(m, ileri, op, w, V, arg.hassasiyet == "bf16")
    son = kayip.item()
    torch.cuda.synchronize()
    sure = (time.perf_counter() - bas) * 1000 / (a["ADIM"] - a["ISINMA"])
    ad = "torch.compile ile" if derle else "derlemesiz"
    print(f"PyTorch ({ad}, {arg.hassasiyet}): {sure:.2f} ms/adım · son kayıp {son:.4f} · en çok bellek {torch.cuda.max_memory_allocated() / 2**30:.2f} GB")


print(f"kart: {torch.cuda.get_device_name()} · torch {torch.__version__} · ayar: {arg.boyut} B={B} T={T} D={D} H={H} FF={FF} katman={L} V={V}")
if arg.derleme in ("yok", "ikisi"):
    olc(False)
if arg.derleme in ("var", "ikisi"):
    torch.cuda.reset_peak_memory_stats()
    olc(True)
