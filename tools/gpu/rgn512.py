"""RGN-512 — tek dosyalık, bağımsız sürüm (yalnız torch + numpy; isteğe bağlı: tokenizers).  Kopyalayıp her yerde çalıştırılır.

Tarif (ölçümlerle seçildi, 17 Ekim 2026):
  • YYG düzeni: 8 katman YYGYGYYG — Y (yerel) katmanlar son 32 / 64 token'a bakar (sırayla), G (genel) katmanlar tam causal
  • GQA ½: KV head = head / 2 (8 head, 4 KV)
  • PLE (katman başına kelime gömmesi, Gemma 3n formülü, P = d = 512):
        e  = (RMSNorm_P(W_ctx · gömme)[katman] + √P · tablo_katman[kelime]) / √2
        H += RMSNorm_d( W_p · (GELU(W_g · H) ⊙ e) )            (blok sonunda)
  • ½ MLP: SwiGLU, genişlik 672 (= 8d/3'ün yarısı)
  • RoPE, QK-norm, ön-norm RMSNorm, gömme = LM head (paylaşımlı)
  • Eğitim: Muon (blok matrisleri) + AdamW (gömme, normlar, bias'lar) + satır seyrek RMS (PLE tabloları; yalnız o adımda
    geçen kelimelerin satırları), WSD takvimi (ısınma %1, son %20 doğrusal iniş), lr 2e-3, ağırlık sönümü 0,1,
    gradyan kırpma 1,0, bf16, 64 × 1024 token / adım
  • Varsayılan tokenizer: Türkçe BPE 40k (tr_bpe_40k_v2.json; <|endoftext|> = 0).  GPT-2 için --vocab 50304 --eos 50256.

Kullanım:
    python rgn512.py test                                          # CPU'da kendi kendini sınar (sentetik veri, ~1 dk)
    python rgn512.py train --data VERI_KLASORU --valid valid.npy --out kosu --tokens 2e9
    python rgn512.py train --out kosu --resume                     # kaldığı yerden
    python rgn512.py generate --ckpt kosu/ckpt.pt --tokenizer tr_bpe_40k_v2.json --prompt "Türkiye'nin başkenti"

Veri: uint16 .npy token dosyaları (klasör ya da tek dosya; belge sonları <|endoftext|>).  Eğitim rastgele pencereler,
doğrulama valid.npy'nin başından sabit pencereler.
"""
import argparse
import glob
import json
import math
import os
import time
from dataclasses import asdict, dataclass

import numpy as np
import torch
import torch.nn as nn
import torch.nn.functional as F
from torch.utils.checkpoint import checkpoint


# ======================================================================================================== ayarlar
@dataclass
class Config:
    vocab_size: int = 40000          # 64'ün katı olmalı (GPT-2: 50304)
    eos_id: int = 0
    d_model: int = 512
    n_layers: int = 8
    n_heads: int = 8
    n_kv_heads: int = 4              # GQA ½
    global_layers: int = 3           # G katman sayısı (eşit aralık, son katman G): 8 katmanda YYGYGYYG
    y_windows: tuple = (32, 64)      # Y katmanlarına sırayla pencere
    d_ff: int = 672                  # ½ MLP
    ple_dim: int = 512               # PLE, P = d
    max_len: int = 4096              # RoPE tablosu
    rope_base: float = 10000.0
    init_std: float = 0.02

    def layer_kinds(self):
        at = {round((i + 1) * self.n_layers / self.global_layers) - 1 for i in range(self.global_layers)}
        return "".join("G" if l in at else "Y" for l in range(self.n_layers))

    def dh(self):
        return self.d_model // self.n_heads


@dataclass
class TrainConfig:
    seq_len: int = 1024
    batch_size: int = 64             # adım başına pencere
    micro_batch: int = 32
    lr: float = 2e-3
    weight_decay: float = 0.1
    betas: tuple = (0.9, 0.95)
    momentum: float = 0.95
    warmup: float = 0.01
    decay: float = 0.2
    grad_clip: float = 1.0
    eval_every: int = 1000
    eval_tokens: int = 4_194_304
    log_every: int = 50
    ckpt_minutes: float = 30.0
    compile: bool = True
    seed: int = 0


# ======================================================================================================== model
class RMSNorm(nn.Module):
    def __init__(self, d, eps=1e-6):
        super().__init__()
        self.eps = eps
        self.weight = nn.Parameter(torch.ones(d))

    def forward(self, x):
        xf = x.float()
        return (xf * torch.rsqrt(xf.pow(2).mean(-1, keepdim=True) + self.eps)).to(x.dtype) * self.weight.to(x.dtype)


def rope_tables(n, dh, base):
    inv = 1.0 / (base ** (torch.arange(0, dh, 2, dtype=torch.float32) / dh))
    ang = torch.outer(torch.arange(n, dtype=torch.float32), inv)
    ang = torch.cat([ang, ang], dim=-1)
    return ang.cos(), ang.sin()


def apply_rope(x, cos, sin):
    x1, x2 = x.chunk(2, dim=-1)
    return x * cos.to(x.dtype) + torch.cat([-x2, x1], dim=-1) * sin.to(x.dtype)


_FLEX = {}
FLEX_BLOCK = 128


def _flex(q, k, v, window, scale, gqa):
    """CUDA'da yerel pencere: FlexAttention blok maskesi (pencere dışı bloklar hiç hesaplanmaz)."""
    from torch.nn.attention.flex_attention import create_block_mask, flex_attention
    if "fn" not in _FLEX:
        _FLEX["fn"] = torch.compile(flex_attention, dynamic=False)
    n = q.shape[2]
    key = (n, window, str(q.device))
    if key not in _FLEX:
        def mask_mod(b, h, qi, ki):
            return (qi >= ki) & (qi - ki < window)
        _FLEX[key] = create_block_mask(mask_mod, None, None, n, n, device=q.device, _compile=True)
    return _FLEX["fn"](q, k, v, block_mask=_FLEX[key], scale=scale, enable_gqa=gqa)


class Attention(nn.Module):
    def __init__(self, cfg, window):
        super().__init__()
        d, self.H, self.Hkv, self.dh, self.window = cfg.d_model, cfg.n_heads, cfg.n_kv_heads, cfg.dh(), window
        self.w_q = nn.Linear(d, self.H * self.dh, bias=False)
        self.w_kv = nn.Linear(d, 2 * self.Hkv * self.dh, bias=False)
        self.w_o = nn.Linear(self.H * self.dh, d, bias=False)
        self.q_norm, self.k_norm = RMSNorm(self.dh), RMSNorm(self.dh)

    def forward(self, x, rope):
        B, N, _ = x.shape
        q = self.w_q(x).view(B, N, self.H, self.dh).transpose(1, 2)
        k, v = self.w_kv(x).view(B, N, 2, self.Hkv, self.dh).permute(2, 0, 3, 1, 4)
        q, k = apply_rope(self.q_norm(q), *rope), apply_rope(self.k_norm(k), *rope)
        s, gqa = self.dh ** -0.5, self.Hkv != self.H
        if not self.window:
            o = F.scaled_dot_product_attention(q, k, v, is_causal=True, scale=s, enable_gqa=gqa)
        elif q.is_cuda and N >= FLEX_BLOCK and N % FLEX_BLOCK == 0:
            o = _flex(q, k, v, self.window, s, gqa)
        else:
            rel = torch.arange(N, device=x.device)[:, None] - torch.arange(N, device=x.device)[None, :]
            o = F.scaled_dot_product_attention(q, k, v, attn_mask=(rel >= 0) & (rel < self.window), scale=s,
                                               enable_gqa=gqa)
        return self.w_o(o.to(x.dtype).transpose(1, 2).reshape(B, N, self.H * self.dh))


class SwiGLU(nn.Module):
    def __init__(self, d, ff):
        super().__init__()
        self.a = nn.Linear(d, 2 * ff)                                   # [G | U]
        self.c = nn.Linear(ff, d)

    def forward(self, x):
        g, u = self.a(x).chunk(2, dim=-1)
        return self.c(F.silu(g) * u)


class Block(nn.Module):
    def __init__(self, cfg, kind, window):
        super().__init__()
        self.kind = kind
        self.norm1, self.norm2 = RMSNorm(cfg.d_model), RMSNorm(cfg.d_model)
        self.attn = Attention(cfg, window)
        self.mlp = SwiGLU(cfg.d_model, cfg.d_ff)
        P = cfg.ple_dim
        self.ple_table = nn.Parameter(torch.empty(cfg.vocab_size, P))   # kelime × P (bu katman)
        self.ple_gate = nn.Linear(cfg.d_model, P, bias=False)
        self.ple_proj = nn.Linear(P, cfg.d_model, bias=False)
        self.ple_norm = RMSNorm(cfg.d_model)

    def forward(self, x, rope, idx, ple_ctx):
        s = x + self.attn(self.norm1(x), rope)
        out = s + self.mlp(self.norm2(s))
        P = self.ple_table.shape[1]
        e = (ple_ctx + F.embedding(idx, self.ple_table).to(ple_ctx.dtype) * P ** 0.5) * 2 ** -0.5
        return out + self.ple_norm(self.ple_proj(F.gelu(self.ple_gate(out)) * e.to(out.dtype)))


class RGN(nn.Module):
    def __init__(self, cfg):
        super().__init__()
        assert cfg.vocab_size % 64 == 0 and cfg.n_heads % cfg.n_kv_heads == 0
        self.cfg = cfg
        self.embed = nn.Embedding(cfg.vocab_size, cfg.d_model)
        kinds, yi, blocks = cfg.layer_kinds(), 0, []
        for kind in kinds:
            w = 0
            if kind == "Y":
                w, yi = cfg.y_windows[yi % len(cfg.y_windows)], yi + 1
            blocks.append(Block(cfg, kind, w))
        self.blocks = nn.ModuleList(blocks)
        self.ple_ctx = nn.Linear(cfg.d_model, cfg.n_layers * cfg.ple_dim, bias=False)
        self.ple_ctx_norm = RMSNorm(cfg.ple_dim)
        self.norm_f = RMSNorm(cfg.d_model)
        cos, sin = rope_tables(cfg.max_len, cfg.dh(), cfg.rope_base)
        self.register_buffer("rope_cos", cos, persistent=False)
        self.register_buffer("rope_sin", sin, persistent=False)
        self._init()

    def _init(self):
        std, L = self.cfg.init_std, self.cfg.n_layers
        for name, p in self.named_parameters():
            if name.endswith(".ple_table"):
                nn.init.normal_(p, 0.0, std)
            elif p.dim() == 2:                                          # artığa yazanlar std / √(2L)
                nn.init.normal_(p, 0.0, std / math.sqrt(2 * L) if name.endswith(("attn.w_o.weight", "mlp.c.weight",
                                                                                "ple_proj.weight")) else std)
            elif name.endswith(".bias"):
                nn.init.zeros_(p)

    def hidden(self, idx):
        B, N = idx.shape
        x = self.embed(idx)
        rope = (self.rope_cos[:N], self.rope_sin[:N])
        D, L, P = self.cfg.d_model, self.cfg.n_layers, self.cfg.ple_dim
        ple = self.ple_ctx_norm((self.ple_ctx(x) * D ** -0.5).view(B, N, L, P))
        for i, b in enumerate(self.blocks):
            x = b(x, rope, idx, ple[:, :, i])
        return self.norm_f(x)

    def forward(self, idx, targets=None):
        h = self.hidden(idx)
        if targets is None:
            return h @ self.embed.weight.T                              # LM head = gömme (paylaşımlı)
        return lm_loss(h, self.embed.weight, targets)

    @torch.no_grad()
    def generate(self, idx, n_new, temperature=0.8, top_k=50, eos_id=None):
        """Önbelleksiz örnekleme (her adımda bağlamın tamamı; küçük modelde yeterince hızlı)."""
        for _ in range(n_new):
            logits = self(idx[:, -self.cfg.max_len:])[:, -1].float()
            if temperature <= 0:
                nxt = logits.argmax(-1, keepdim=True)
            else:
                logits = logits / temperature
                if top_k:
                    v, _ = logits.topk(top_k)
                    logits[logits < v[:, -1:]] = -float("inf")
                nxt = torch.multinomial(torch.softmax(logits, -1), 1)
            idx = torch.cat([idx, nxt], 1)
            if eos_id is not None and bool((nxt == eos_id).all()):
                break
        return idx


def _ce_sum(h, w, t):
    return F.cross_entropy((h @ w.T).float(), t, reduction="sum")


def lm_loss(h, w, targets, chunk=8192):
    """Ortalama cross-entropy; logit'ler parça parça ve geri geçişte yeniden hesaplanır (V × N matrisi bellekte durmaz)."""
    h, t = h.reshape(-1, h.shape[-1]), targets.reshape(-1)
    tot = h.new_zeros((), dtype=torch.float32)
    for i in range(0, t.numel(), chunk):
        a = (h[i:i + chunk], w, t[i:i + chunk])
        tot = tot + (checkpoint(_ce_sum, *a, use_reentrant=False) if torch.is_grad_enabled() else _ce_sum(*a))
    return tot / t.numel()


# ======================================================================================================== optimizer
NS_COEF = (3.4445, -4.7750, 2.0315)


def newton_schulz(G, steps=5, eps=1e-7):
    X = G.bfloat16()
    tall = X.size(-2) > X.size(-1)
    if tall:
        X = X.mT
    X = X / (X.norm(dim=(-2, -1), keepdim=True) + eps)
    a, b, c = NS_COEF
    for _ in range(steps):
        A = X @ X.mT
        X = a * X + (b * A + c * (A @ A)) @ X
    return X.mT if tall else X


class Muon(torch.optim.Optimizer):
    """Nesterov momentum + Newton–Schulz ortogonalleştirme; güncelleme RMS'i AdamW ölçeğinde (0,2·√max(m, n))."""

    def __init__(self, params, lr, momentum=0.95, weight_decay=0.0):
        super().__init__(params, dict(lr=lr, momentum=momentum, weight_decay=weight_decay))

    @torch.no_grad()
    def step(self):
        for g in self.param_groups:
            ps = [p for p in g["params"] if p.grad is not None]
            for p in ps:
                if "m" not in self.state[p]:
                    self.state[p]["m"] = torch.zeros_like(p, dtype=torch.float32)
            grads = [p.grad.float() for p in ps]
            bufs = [self.state[p]["m"] for p in ps]
            if not ps:
                continue
            torch._foreach_lerp_(bufs, grads, 1 - g["momentum"])
            upds = torch._foreach_lerp(grads, bufs, g["momentum"])
            shapes = {}
            for i, p in enumerate(ps):
                shapes.setdefault(tuple(p.shape), []).append(i)
            for (r, c), ix in shapes.items():                           # aynı biçimli matrisler tek yığında
                O = newton_schulz(torch.stack([upds[i] for i in ix])).float() * (0.2 * math.sqrt(max(r, c)))
                for j, i in enumerate(ix):
                    ps[i].mul_(1 - g["lr"] * g["weight_decay"])
                    ps[i].add_(O[j].to(ps[i].dtype), alpha=-g["lr"])


class RowRMS(torch.optim.Optimizer):
    """PLE tabloları: yalnız verilen satırlar (o adımda geçen kelimeler) güncellenir; satır başına tek ikinci moment,
    bias düzeltmesi satırın kendi güncelleme sayısıyla (ilk kez görülen kelime doğru ölçekle başlar)."""

    def __init__(self, params, lr, beta2=0.95, eps=1e-8):
        super().__init__(params, dict(lr=lr, beta2=beta2, eps=eps))
        self.rows = None

    @torch.no_grad()
    def step(self):
        rr = self.rows
        for g in self.param_groups:
            for p in g["params"]:
                if p.grad is None or rr is None:
                    continue
                st = self.state[p]
                if "v" not in st:
                    st["v"] = torch.zeros(p.shape[0], 1, device=p.device)
                    st["n"] = torch.zeros(p.shape[0], 1, device=p.device)
                gr = p.grad.index_select(0, rr).float()
                n = st["n"].index_select(0, rr) + 1
                v = st["v"].index_select(0, rr) * g["beta2"] + gr.pow(2).mean(1, keepdim=True) * (1 - g["beta2"])
                st["n"].index_copy_(0, rr, n)
                st["v"].index_copy_(0, rr, v)
                upd = gr / ((v / (1 - g["beta2"] ** n)).sqrt() + g["eps"])
                p.index_add_(0, rr, (-g["lr"] * upd).to(p.dtype))


def build_optimizers(model, tc):
    muon, decay, no_decay, tables = [], [], [], []
    for name, p in model.named_parameters():
        if name.endswith(".ple_table"):
            tables.append(p)
        elif name.startswith("blocks.") and p.dim() == 2:
            muon.append(p)
        elif p.dim() >= 2:
            decay.append(p)
        else:
            no_decay.append(p)
    fused = next(model.parameters()).is_cuda
    opts = [torch.optim.AdamW([dict(params=decay, weight_decay=tc.weight_decay),
                               dict(params=no_decay, weight_decay=0.0)], lr=tc.lr, betas=tc.betas, fused=fused),
            Muon(muon, lr=tc.lr, momentum=tc.momentum, weight_decay=tc.weight_decay),
            RowRMS(tables, lr=tc.lr, beta2=tc.betas[1])]
    for o in opts:
        for g in o.param_groups:
            g["base_lr"] = tc.lr
    return opts


def lr_factor(step, total, warmup=0.01, decay=0.2):
    """WSD: ısınma -> sabit -> son `decay` oranında doğrusal iniş."""
    w = max(1, int(total * warmup))
    if step < w:
        return (step + 1) / w
    start = int(total * (1 - decay))
    return 1.0 if step < start else max(0.0, 1 - (step - start) / max(1, total - start))


# ======================================================================================================== veri
class Tokens:
    """uint16 .npy dosyaları (klasör ya da tek dosya), belleğe eşlenmiş; pencere seçimi dosya boyuna orantılı."""

    def __init__(self, path):
        files = sorted(glob.glob(os.path.join(path, "**", "*.npy"), recursive=True)) if os.path.isdir(path) else [path]
        self.arrs = [np.load(f, mmap_mode="r") for f in files]
        self.lens = np.array([len(a) for a in self.arrs], dtype=np.int64)
        self.total = int(self.lens.sum())
        assert self.total, "token bulunamadı: %s" % path

    def random_batch(self, B, T, rng):
        p = (self.lens - T - 1).clip(0) / max(1, (self.lens - T - 1).clip(0).sum())
        fi = rng.choice(len(self.arrs), size=B, p=p)
        out = np.empty((B, T + 1), dtype=np.int64)
        for j, f in enumerate(fi):
            s = rng.integers(0, self.lens[f] - T - 1)
            out[j] = self.arrs[f][s:s + T + 1]
        t = torch.from_numpy(out)
        return t[:, :-1], t[:, 1:]

    def eval_windows(self, T, n_tokens):
        a = self.arrs[0]
        k = min(n_tokens // T, (len(a) - 1) // T)
        w = np.asarray(a[:k * T + 1], dtype=np.int64)
        x = torch.from_numpy(w[:-1].reshape(k, T))
        y = torch.from_numpy(w[1:].reshape(k, T))
        return x, y


@torch.no_grad()
def evaluate(model, valid, tc, device, ctx):
    model.eval()
    x, y = valid
    tot, n = 0.0, 0
    for i in range(0, len(x), tc.micro_batch):
        xb, yb = x[i:i + tc.micro_batch].to(device), y[i:i + tc.micro_batch].to(device)
        with ctx:
            tot += float(model(xb, yb)) * yb.numel()
        n += yb.numel()
    model.train()
    return tot / n


# ======================================================================================================== eğitim
def train(a, model_kw=None):
    cfg, tc = Config(vocab_size=a.vocab, eos_id=a.eos, **(model_kw or {})), TrainConfig(seq_len=a.seq_len, batch_size=a.batch,
                                                                    micro_batch=a.micro, lr=a.lr, compile=not a.no_compile)
    os.makedirs(a.out, exist_ok=True)
    ck = os.path.join(a.out, "ckpt.pt")
    state = torch.load(ck, map_location="cpu", weights_only=False) if a.resume and os.path.exists(ck) else None
    if state:
        cfg, tc = Config(**{**state["cfg"], "y_windows": tuple(state["cfg"]["y_windows"])}), TrainConfig(**{
            **state["tc"], "betas": tuple(state["tc"]["betas"])})
        a.data, a.valid, a.steps = state["data"], state["valid"], state["total"]
    torch.manual_seed(tc.seed)
    dev = torch.device("cuda" if torch.cuda.is_available() else "cpu")
    if dev.type == "cuda":
        torch.backends.cuda.matmul.allow_tf32 = torch.backends.cudnn.allow_tf32 = True
    data = Tokens(a.data)
    valid = Tokens(a.valid).eval_windows(tc.seq_len, tc.eval_tokens) if a.valid else None
    per_step = tc.batch_size * tc.seq_len
    total = a.steps or max(1, int(float(a.tokens) / per_step) if a.tokens else data.total // per_step)
    model = RGN(cfg).to(dev)
    opts = build_optimizers(model, tc)
    step, rng = 0, np.random.default_rng(tc.seed)
    if state:
        model.load_state_dict(state["model"])
        for o, s in zip(opts, state["opts"]):
            o.load_state_dict(s)
        step, rng.bit_generator.state = state["step"], state["rng"]
    fwd = torch.compile(model) if (tc.compile and dev.type == "cuda") else model
    ctx = torch.autocast("cuda", dtype=torch.bfloat16) if dev.type == "cuda" else torch.autocast("cpu", enabled=False)
    n_params = sum(p.numel() for p in model.parameters())
    n_tab = sum(b.ple_table.numel() for b in model.blocks)
    print("RGN-512 | %s | %.1fM parametre (PLE tabloları %.1fM, gövde + gömme %.1fM) | veri %.2fB token | %d adım × %d "
          "token = %.2fB | cihaz %s" % (cfg.layer_kinds(), n_params / 1e6, n_tab / 1e6, (n_params - n_tab) / 1e6,
                                        data.total / 1e9, total, per_step, total * per_step / 1e9, dev), flush=True)

    def save(final=False):
        torch.save(dict(model=model.state_dict(), opts=[o.state_dict() for o in opts], step=step, total=total,
                        rng=rng.bit_generator.state, cfg=asdict(cfg), tc=asdict(tc), data=a.data, valid=a.valid), ck + ".tmp")
        os.replace(ck + ".tmp", ck)
        if final:
            torch.save(dict(model=model.state_dict(), cfg=asdict(cfg), step=step), os.path.join(a.out, "model.pt"))

    accum = tc.batch_size // tc.micro_batch
    t0, last_ck, win = time.time(), time.time(), []
    stop_at = getattr(a, "stop_at", 0)
    while step < total:
        if stop_at and step >= stop_at:
            save()
            print("durdu: adım %d (sürdürmek için --resume)" % step)
            return
        f = lr_factor(step, total, tc.warmup, tc.decay)
        for o in opts:
            for g in o.param_groups:
                g["lr"] = g["base_lr"] * f
        ts = time.time()
        xs = [data.random_batch(tc.micro_batch, tc.seq_len, rng) for _ in range(accum)]
        loss_sum = 0.0
        for x, y in xs:
            with ctx:
                loss = fwd(x.to(dev, non_blocking=True), y.to(dev, non_blocking=True)) / accum
            loss.backward()
            loss_sum += float(loss.detach())
        gn = torch.nn.utils.clip_grad_norm_(model.parameters(), tc.grad_clip)
        opts[2].rows = torch.unique(torch.cat([x.flatten() for x, _ in xs])).to(dev)   # PLE: geçen kelimelerin satırları
        for o in opts:
            o.step()
            o.zero_grad(set_to_none=True)
        step += 1
        win.append(time.time() - ts)
        if step % tc.log_every == 0 or step == total:
            ms = 1000 * sum(win) / len(win)
            print("adım %d / %d  lr %.2e  kayıp %.4f  grad %.3f | %.0f ms/adım  %.0f token/sn | kalan ~%.0f dk" % (
                step, total, tc.lr * f, loss_sum, float(gn), ms, per_step / ms * 1000, (total - step) * ms / 60000),
                flush=True)
            win = []
        if valid is not None and (step % tc.eval_every == 0 or step == total):
            print("  DEĞERLENDİRME adım %d: kayıp %.4f" % (step, evaluate(fwd, valid, tc, dev, ctx)), flush=True)
        if (time.time() - last_ck) / 60 >= tc.ckpt_minutes:
            save()
            last_ck = time.time()
    save(final=True)
    print("BİTTİ: %d adım, %.1f dk -> %s" % (step, (time.time() - t0) / 60, os.path.join(a.out, "model.pt")))


def load(path, device="cpu"):
    st = torch.load(path, map_location="cpu", weights_only=False)
    cfg = Config(**{**st["cfg"], "y_windows": tuple(st["cfg"]["y_windows"])})
    m = RGN(cfg)
    m.load_state_dict(st["model"])
    return m.to(device).eval()


def generate(a):
    dev = torch.device("cuda" if torch.cuda.is_available() else "cpu")
    m = load(a.ckpt, dev)
    from tokenizers import Tokenizer
    tok = Tokenizer.from_file(a.tokenizer)
    ids = torch.tensor([tok.encode(a.prompt, add_special_tokens=False).ids], device=dev)
    out = m.generate(ids, a.n, a.temperature, a.top_k, m.cfg.eos_id)
    print(tok.decode(out[0].tolist()))


# ======================================================================================================== öz-sınama
def selftest():
    """CPU: küçük sentetik veri; ileri / geri, nedensellik, pencere, kayıt / sürdürme, üretim."""
    import tempfile
    torch.manual_seed(0)
    cfg = Config(vocab_size=256, d_model=64, n_heads=4, n_kv_heads=2, d_ff=96, ple_dim=64, max_len=256)
    m = RGN(cfg)
    assert cfg.layer_kinds() == "YYGYGYYG", cfg.layer_kinds()
    x = torch.randint(0, 256, (2, 40))
    l1 = m(x)
    x2 = x.clone()
    x2[:, 30] = (x2[:, 30] + 1) % 256
    assert torch.allclose(l1[:, :30], m(x2)[:, :30], atol=1e-5), "nedensellik"
    loss = m(x, x)
    loss.backward()
    assert math.isfinite(float(loss.detach())) and all(p.grad is not None for p in m.parameters())
    with tempfile.TemporaryDirectory() as d:
        rng = np.random.default_rng(0)
        np.save(os.path.join(d, "a.npy"), rng.integers(0, 64, 200_000, dtype=np.uint16))
        np.save(os.path.join(d, "v.npy"), rng.integers(0, 64, 20_000, dtype=np.uint16))
        small = dict(d_model=64, n_heads=4, n_kv_heads=2, d_ff=96, ple_dim=64, max_len=256)
        out = os.path.join(d, "kosu")
        args = ["train", "--data", os.path.join(d, "a.npy"), "--valid", os.path.join(d, "v.npy"), "--out", out,
                "--vocab", "256", "--seq_len", "64", "--batch", "8", "--micro", "4", "--no_compile"]
        train(parser().parse_args(args + ["--steps", "40"]), small)                      # kesintisiz 40 adım
        full = load(os.path.join(out, "model.pt"))
        out2 = os.path.join(d, "kosu2")
        a2 = [x if x != out else out2 for x in args]
        p = parser().parse_args(a2 + ["--steps", "40"])
        p.stop_at = 25
        train(p, small)                                                                  # 25. adımda dur
        train(parser().parse_args(a2 + ["--resume"]), small)                            # kaldığı yerden 40'a
        resumed = load(os.path.join(out2, "model.pt"))
        diff = max((x - y).abs().max().item() for x, y in zip(full.state_dict().values(), resumed.state_dict().values()))
        print("kesintisiz ↔ kesik + sürdürme, en büyük ağırlık farkı: %.2e" % diff)
        assert diff < 1e-5, diff
        assert full.generate(torch.zeros(1, 3, dtype=torch.long), 5).shape == (1, 8)
    print("öz-sınama: TAMAM")


def parser():
    ap = argparse.ArgumentParser(description="RGN-512 (tek dosya)")
    ap.add_argument("cmd", choices=["train", "generate", "test"])
    ap.add_argument("--data", default="")
    ap.add_argument("--valid", default="")
    ap.add_argument("--out", default="kosu")
    ap.add_argument("--resume", action="store_true")
    ap.add_argument("--tokens", default="", help="toplam eğitim token'ı (ör. 2e9); boş: verinin bir turu")
    ap.add_argument("--steps", type=int, default=0)
    ap.add_argument("--vocab", type=int, default=40000)
    ap.add_argument("--eos", type=int, default=0)
    ap.add_argument("--seq_len", type=int, default=1024)
    ap.add_argument("--batch", type=int, default=64)
    ap.add_argument("--micro", type=int, default=32)
    ap.add_argument("--lr", type=float, default=2e-3)
    ap.add_argument("--no_compile", action="store_true")
    ap.add_argument("--ckpt", default="")
    ap.add_argument("--tokenizer", default="")
    ap.add_argument("--prompt", default="")
    ap.add_argument("--n", type=int, default=100)
    ap.add_argument("--temperature", type=float, default=0.8)
    ap.add_argument("--top_k", type=int, default=50)
    return ap


if __name__ == "__main__":
    args = parser().parse_args()
    {"train": train, "generate": generate, "test": lambda a: selftest()}[args.cmd](args)
