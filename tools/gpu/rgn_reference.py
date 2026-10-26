"""Reference run of the target model (rgn512.py) for the TR correctness comparison.

Writes, into --out:
  data/train.npy   uint16 token stream; TR's sequential reader cuts it into the same batches
                   (window j starts at j*T and holds T+1 tokens: input = first T, target = shifted)
  init.trt         the initial weights under TR's parameter names (tools/gpu/trt_io.py), so that
                   rgn512.tr starts from exactly the same model
  reference.json   per step: summed micro-batch loss and gradient norm; plus the forward loss of
                   the first micro-batch before any update

The run uses rgn512.py's own model, optimizers (Muon + AdamW + RowRMS), WSD schedule, clipping
and accumulation; only the data order is fixed (sequential windows). --ns-fp32 runs Muon's
Newton-Schulz in float32 instead of bf16 so that a float32 comparison is tight.

Usage: python rgn_reference.py --out ref --steps 4 --micro 2 --accum 2 --seq 64 [--device cpu]
"""
import argparse
import json
import os
import sys

import numpy as np

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from trt_io import write_trt  # noqa: E402


def tr_name_map(n_layers):
    """PyTorch parameter name -> (TR parameter name, transpose). TR's Doğrusal weight is
    [in, out] (x @ W); PyTorch's nn.Linear weight is [out, in]."""
    m = {"embed.weight": ("TR_mem_embedding.TR_mem_table", False), "norm_f.weight": ("TR_mem_final_norm.TR_mem_weight", False),
         "ple_ctx.weight": ("TR_mem_ple_context.TR_mem_context_proj.TR_mem_weight", True),
         "ple_ctx_norm.weight": ("TR_mem_ple_context.TR_mem_norm.TR_mem_weight", False)}
    per_block = {
        "norm1.weight": ("TR_mem_input_norm.TR_mem_weight", False),
        "norm2.weight": ("TR_mem_post_attention_norm.TR_mem_weight", False),
        "attn.w_q.weight": ("TR_mem_attention.TR_mem_q_proj.TR_mem_weight", True),
        "attn.w_kv.weight": ("TR_mem_attention.TR_mem_kv_proj.TR_mem_weight", True),
        "attn.w_o.weight": ("TR_mem_attention.TR_mem_o_proj.TR_mem_weight", True),
        "attn.q_norm.weight": ("TR_mem_attention.TR_mem_q_norm.TR_mem_weight", False),
        "attn.k_norm.weight": ("TR_mem_attention.TR_mem_k_norm.TR_mem_weight", False),
        "mlp.a.weight": ("TR_mem_feed_forward.TR_mem_gate_up_proj.TR_mem_weight", True),
        "mlp.a.bias": ("TR_mem_feed_forward.TR_mem_gate_up_proj.TR_mem_bias", False),
        "mlp.c.weight": ("TR_mem_feed_forward.TR_mem_down_proj.TR_mem_weight", True),
        "mlp.c.bias": ("TR_mem_feed_forward.TR_mem_down_proj.TR_mem_bias", False),
        "ple_table": ("TR_mem_ple.TR_mem_table", False),
        "ple_gate.weight": ("TR_mem_ple.TR_mem_gate_proj.TR_mem_weight", True),
        "ple_proj.weight": ("TR_mem_ple.TR_mem_proj.TR_mem_weight", True),
        "ple_norm.weight": ("TR_mem_ple.TR_mem_norm.TR_mem_weight", False),
    }
    for i in range(n_layers):
        for k, (v, t) in per_block.items():
            m["blocks.%d.%s" % (i, k)] = ("TR_mem_blocks.%d.%s" % (i, v), t)
    return m


def main():
    ap = argparse.ArgumentParser(description="Reference run of the target model for the TR comparison")
    ap.add_argument("--out", required=True)
    ap.add_argument("--steps", type=int, default=4)
    ap.add_argument("--micro", type=int, default=2)
    ap.add_argument("--accum", type=int, default=2)
    ap.add_argument("--seq", type=int, default=64)
    ap.add_argument("--seed", type=int, default=0)
    ap.add_argument("--lr", type=float, default=2e-3)
    ap.add_argument("--device", default="cpu")
    ap.add_argument("--bf16", action="store_true", help="autocast bf16 (default float32)")
    ap.add_argument("--ns-bf16", action="store_true", help="keep Muon's Newton-Schulz in bf16 as rgn512.py does")
    ap.add_argument("--vocab", type=int, default=40000)
    ap.add_argument("--d", type=int, default=512, help="model width (and PLE width)")
    ap.add_argument("--heads", type=int, default=8)
    ap.add_argument("--kv-heads", type=int, default=4)
    ap.add_argument("--ff", type=int, default=672)
    a = ap.parse_args()

    import torch
    import rgn512 as R

    if not a.ns_bf16:
        def ns_fp32(G, steps=5, eps=1e-7):
            X = G.float()
            tall = X.size(-2) > X.size(-1)
            if tall:
                X = X.mT
            X = X / (X.norm(dim=(-2, -1), keepdim=True) + eps)
            ca, cb, cc = R.NS_COEF
            for _ in range(steps):
                A = X @ X.mT
                X = ca * X + (cb * A + cc * (A @ A)) @ X
            return X.mT if tall else X
        R.newton_schulz = ns_fp32

    torch.manual_seed(a.seed)
    dev = torch.device(a.device)
    cfg = R.Config(vocab_size=a.vocab, d_model=a.d, n_heads=a.heads, n_kv_heads=a.kv_heads, d_ff=a.ff, ple_dim=a.d)
    tc = R.TrainConfig(seq_len=a.seq, batch_size=a.micro * a.accum, micro_batch=a.micro, lr=a.lr)
    model = R.RGN(cfg).to(dev)

    os.makedirs(os.path.join(a.out, "data"), exist_ok=True)
    T, n_windows = a.seq, a.steps * a.accum * a.micro
    stream = np.random.default_rng(a.seed).integers(1, cfg.vocab_size, size=n_windows * T + 1, dtype=np.uint16)
    np.save(os.path.join(a.out, "data", "train.npy"), stream)

    names = tr_name_map(cfg.n_layers)
    sd = model.state_dict()
    missing = sorted(set(sd) - set(names))
    if missing:
        raise SystemExit("no TR name for: %s" % ", ".join(missing))
    write_trt(os.path.join(a.out, "init.trt"),
              {names[k][0]: (v.detach().float().cpu().numpy().T if names[k][1] else v.detach().float().cpu().numpy())
               for k, v in sd.items()})

    def batch(i):  # i-th micro-batch of the sequential reader
        rows = [stream[(i * a.micro + r) * T:(i * a.micro + r) * T + T + 1].astype(np.int64) for r in range(a.micro)]
        t = torch.from_numpy(np.stack(rows)).to(dev)
        return t[:, :-1], t[:, 1:]

    ctx = torch.autocast(dev.type, dtype=torch.bfloat16, enabled=a.bf16)
    with torch.no_grad(), ctx:
        x0, y0 = batch(0)
        first_loss = float(model(x0, y0))

    opts = R.build_optimizers(model, tc)
    out = {"first_forward_loss": first_loss, "steps": [], "config": {"micro": a.micro, "accum": a.accum, "seq": T,
           "lr": a.lr, "bf16": a.bf16, "ns_fp32": not a.ns_bf16}}
    for step in range(a.steps):
        f = R.lr_factor(step, a.steps, tc.warmup, tc.decay)
        for o in opts:
            for g in o.param_groups:
                g["lr"] = g["base_lr"] * f
        xs = [batch(step * a.accum + j) for j in range(a.accum)]
        loss_sum = 0.0
        for x, y in xs:
            with ctx:
                loss = model(x, y) / a.accum
            loss.backward()
            loss_sum += float(loss.detach())
        gn = float(torch.nn.utils.clip_grad_norm_(model.parameters(), tc.grad_clip))
        opts[2].rows = torch.unique(torch.cat([x.flatten() for x, _ in xs]))
        for o in opts:
            o.step()
            o.zero_grad(set_to_none=True)
        out["steps"].append({"loss": loss_sum, "grad_norm": gn, "lr_factor": f})
        print("step %d  loss %.6f  grad %.6f" % (step + 1, loss_sum, gn), flush=True)
    json.dump(out, open(os.path.join(a.out, "reference.json"), "w"), indent=1)
    print("first forward loss %.6f" % first_loss)


if __name__ == "__main__":
    main()
