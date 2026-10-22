# Ölçüm modeli, PyTorch tarafı: compare_gpu.py ve phase3_torch.py aynı modeli (RMS normlu, SwiGLU'lu,
# paylaşımlı çıkışlı küçük bir GPT), aynı sentetik toplu işi ve aynı eğitim adımını kullanır; TR
# karşılıkları compare_gpu.tr ve phase3_tr.tr'dir (measurement_model.tr). rgn512.py ve
# rgn_reference.py bilerek bunu kullanmaz: hedef modelin ölçümü kendi PyTorch dosyasıyla eşleşir.
import torch
import torch.nn as nn
import torch.nn.functional as F


def model_sinifi(B, T, D, H, FF, L, V):
    """Verilen boyutlarla Model sınıfını verir (Blok, Model'in içindedir)."""

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

    return Model


def sentetik_toplu(B, T, V, adim, cihaz):
    """Sabit sentetik toplu iş (TR tarafıyla aynı formül): 8 toplu işten adım % 8'inci."""
    s = adim % 8
    w = [[(s * 7919 + i * 104729 + t * 31) % V for t in range(T + 1)] for i in range(B)]
    return torch.tensor(w, dtype=torch.int64, device=cihaz)


def egitim_adimi(m, ileri, op, w, V, bf16):
    """Bir eğitim adımı: sıfırla, (isteğe bağlı bf16 autocast ile) ileri + çapraz entropi, geri, kırp, adım."""
    op.zero_grad(set_to_none=True)
    with torch.autocast("cuda", dtype=torch.bfloat16, enabled=bf16):
        kayip = F.cross_entropy(ileri(w[:, :-1]).reshape(-1, V), w[:, 1:].reshape(-1))
    kayip.backward()
    torch.nn.utils.clip_grad_norm_(m.parameters(), 1.0)
    op.step()
    return kayip
