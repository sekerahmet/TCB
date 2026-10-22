"""Builds a small random Hugging Face model (llama or qwen3), saves it with save_pretrained
(config.json + model.safetensors) and writes the float32 logits of fixed token ids.

Used by cmd/tr/e2e (TestReadyModelMatchesTransformers) to check that tr/hazir loads the model
and computes the same logits, and to regenerate the tiny example model in tests/data/hf_tiny
(see tools/hf/README.md). No network: the model is built from a config, not downloaded.

    python -I tools/hf/small_model.py --arch qwen3 --out DIR [--tied]
    python -I tools/hf/small_model.py --arch llama --out DIR --tied --vocab 1005 --hidden 8 \
        --intermediate 16 --layers 1 --heads 2 --kv-heads 1 --dtype float16 --no-logits \
        --tokenizer tests/data/tokenizer.json

The weights are drawn from a fixed seed (--seed, default 0): the same command gives the same files.
"""
import argparse
import json
import os
import shutil

import torch


def build(a):
    import transformers
    common = dict(vocab_size=a.vocab, hidden_size=a.hidden, intermediate_size=a.intermediate,
                  num_hidden_layers=a.layers, num_attention_heads=a.heads,
                  num_key_value_heads=a.kv_heads, max_position_embeddings=a.max_position,
                  rms_norm_eps=1e-6, tie_word_embeddings=a.tied)
    if a.arch == "llama":
        cfg = transformers.LlamaConfig(rope_theta=100000.0, **common)
        cls = transformers.LlamaForCausalLM
    else:
        # head_dim 32 differs from hidden_size / heads = 16: the independent head size is exercised.
        cfg = transformers.Qwen3Config(head_dim=a.head_dim or 32, rope_theta=1000000.0, **common)
        cls = transformers.Qwen3ForCausalLM
    torch.manual_seed(a.seed)
    return cls(cfg).eval()


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--arch", choices=["llama", "qwen3"], required=True)
    ap.add_argument("--out", required=True)
    ap.add_argument("--tied", action="store_true")
    ap.add_argument("--vocab", type=int, default=128)
    ap.add_argument("--hidden", type=int, default=64)
    ap.add_argument("--intermediate", type=int, default=96)
    ap.add_argument("--layers", type=int, default=2)
    ap.add_argument("--heads", type=int, default=4)
    ap.add_argument("--kv-heads", type=int, default=2)
    ap.add_argument("--head-dim", type=int, default=0, help="qwen3 only (default 32)")
    ap.add_argument("--max-position", type=int, default=32)
    ap.add_argument("--dtype", choices=["float32", "float16", "bfloat16"], default="float32")
    ap.add_argument("--seed", type=int, default=0)
    ap.add_argument("--no-logits", action="store_true", help="do not write logits.json")
    ap.add_argument("--tokenizer", default="", help="tokenizer.json copied next to the model")
    a = ap.parse_args()
    model = build(a)
    model = model.to(getattr(torch, a.dtype))
    model.save_pretrained(a.out, safe_serialization=True)
    if a.tokenizer:
        shutil.copyfile(a.tokenizer, os.path.join(a.out, "tokenizer.json"))
    if not a.no_logits:
        ids = [[3, 17, 42, 99, 5, 64, 127, 0, 8, 21]]
        with torch.no_grad():
            logits = model.float()(torch.tensor(ids)).logits[0].float()
        with open(os.path.join(a.out, "logits.json"), "w") as f:
            json.dump({"ids": ids, "logits": logits.tolist()}, f)


if __name__ == "__main__":
    main()
