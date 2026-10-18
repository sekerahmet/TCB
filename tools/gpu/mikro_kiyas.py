# GPU mikro kıyasları, karşılaştırma tarafı: internal/hesap/gpu/mikro_kiyas_test.go'daki
# TestMikroKiyas ile aynı işlemler ve aynı satır biçimi:
#     MIKRO|işlem|pytorch|ms|değer|birim
# float32, TF32 kapalı. Süreler iki CUDA olayı arasında, bir ısınmadan sonra 10 tekrarın
# ortalamasıdır. Kullanım: python mikro_kiyas.py > mikro_py.txt
import math
import torch
import torch.nn.functional as F

torch.backends.cuda.matmul.allow_tf32 = False
torch.backends.cudnn.allow_tf32 = False
cihaz = "cuda"
torch.manual_seed(1)


def olc(f, n=10):
    f()
    torch.cuda.synchronize()
    a, b = torch.cuda.Event(enable_timing=True), torch.cuda.Event(enable_timing=True)
    a.record()
    for _ in range(n):
        f()
    b.record()
    torch.cuda.synchronize()
    return a.elapsed_time(b) / n


def yaz(islem, ms, deger, birim):
    print(f"MIKRO|{islem}|pytorch|{ms:.4f}|{deger:.1f}|{birim}", flush=True)


def carpimlar():
    T, D, FF, V = 8192, 512, 2048, 40000
    liste = [("çarpım qkv ileri", T, D, D, False, False), ("çarpım ff1 ileri", T, FF, D, False, False),
             ("çarpım ff2 ileri", T, D, FF, False, False), ("çarpım çıkış ileri", T, V, D, False, False),
             ("çarpım qkv dX", T, D, D, False, True), ("çarpım ff1 dX", T, D, FF, False, True),
             ("çarpım çıkış dX", T, D, V, False, True), ("çarpım qkv dW", D, D, T, True, False),
             ("çarpım ff1 dW", D, FF, T, True, False), ("çarpım çıkış dW", V, D, T, True, False)]
    for ad, M, N, K, ta, tb in liste:
        a = torch.randn(K, M, device=cihaz) if ta else torch.randn(M, K, device=cihaz)
        b = torch.randn(N, K, device=cihaz) if tb else torch.randn(K, N, device=cihaz)
        c = torch.empty(M, N, device=cihaz)
        aa, bb = (a.t() if ta else a), (b.t() if tb else b)
        ms = olc(lambda: torch.mm(aa, bb, out=c))
        yaz(ad, ms, 2 * M * N * K / (ms * 1e6), "GFLOPS")
        del a, b, c


def dikkat():
    B, H, T, Dh = 32, 8, 256, 64
    q, k, v = (torch.randn(B, H, T, Dh, device=cihaz) for _ in range(3))
    flop = 2 * 2 * B * H * T * T * Dh / 2
    ms = olc(lambda: F.scaled_dot_product_attention(q, k, v, is_causal=True))
    yaz("dikkat ileri", ms, flop / (ms * 1e6), "GFLOPS")
    q, k, v = (x.requires_grad_() for x in (q, k, v))
    out = F.scaled_dot_product_attention(q, k, v, is_causal=True)
    dO = torch.randn_like(out)
    ms = olc(lambda: torch.autograd.grad(out, (q, k, v), dO, retain_graph=True))
    yaz("dikkat geri", ms, 2.5 * flop / (ms * 1e6), "GFLOPS")


def ce():
    R, C = 8192, 40000
    x = torch.randn(R, C, device=cihaz) * 2
    t = torch.randint(0, C, (R,), device=cihaz)
    gb = 4 * R * C / 1e9
    ms = olc(lambda: F.cross_entropy(x, t))
    yaz("çapraz entropi ileri", ms, gb / (ms / 1e3), "GB/s")
    x.requires_grad_()
    kayip = F.cross_entropy(x, t)
    ms = olc(lambda: torch.autograd.grad(kayip, x, retain_graph=True))
    yaz("çapraz entropi geri", ms, 2 * gb / (ms / 1e3), "GB/s")


def adamw():
    V, D, FF = 40000, 512, 2048
    boylar = [V * D] + [D, D * D, D * D, D * D, D * D, D, D * FF, D * FF, FF * D] * 6 + [D]
    ps = [torch.nn.Parameter(torch.randn(n, device=cihaz)) for n in boylar]
    for p in ps:
        p.grad = torch.randn_like(p)
    op = torch.optim.AdamW(ps, lr=1e-3, weight_decay=0.0)
    toplam = sum(boylar)
    ms = olc(op.step)
    yaz(f"AdamW {toplam / 1e6:.1f} M parametre", ms, 7 * 4 * toplam / 1e9 / (ms / 1e3), "GB/s")


def eleman():
    kenar = 8192
    n = kenar * kenar
    a, b, c = (torch.randn(n, device=cihaz) for _ in range(3))
    ms = olc(lambda: torch.add(a, b, out=c))
    yaz(f"eleman toplama {n >> 20}M", ms, 12 * n / 1e9 / (ms / 1e3), "GB/s")
    ms = olc(lambda: torch.exp(a, out=c))
    yaz(f"eleman üstel {n >> 20}M", ms, 8 * n / 1e9 / (ms / 1e3), "GB/s")
    dev = a.view(kenar, kenar).t()
    c2 = c.view(kenar, kenar)
    ms = olc(lambda: c2.copy_(dev))
    yaz(f"devrik kopya {kenar}²", ms, 8 * n / 1e9 / (ms / 1e3), "GB/s")


if __name__ == "__main__":
    print(f"MIKRO|bilgi|{torch.cuda.get_device_name()} torch {torch.__version__}|0|0|", flush=True)
    for f in (carpimlar, dikkat, ce, adamw, eleman):
        f()
        torch.cuda.empty_cache()
    print(f"MIKRO|bellek|pytorch|0|{torch.cuda.max_memory_allocated() / 2**20:.0f}|MB tepe", flush=True)
