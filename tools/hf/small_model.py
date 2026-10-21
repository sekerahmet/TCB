"""Builds a small random Hugging Face model (llama or qwen3), saves it with save_pretrained
(config.json + model.safetensors) and writes the float32 logits of fixed token ids.

Used by cmd/tr/e2e (TestReadyModelMatchesTransformers) to check that tr/hazir loads the model
and computes the same logits. No network: the model is built from a config, not downloaded.

    python -I tools/hf/small_model.py --arch qwen3 --out DIR [--tied]
"""
import argparse
import json
import os

import torch


def build(arch, tied):
    import transformers
    common = dict(vocab_size=128, hidden_size=64, intermediate_size=96, num_hidden_layers=2,
                  num_attention_heads=4, num_key_value_heads=2, max_position_embeddings=32,
                  rms_norm_eps=1e-6, tie_word_embeddings=tied)
    if arch == "llama":
        cfg = transformers.LlamaConfig(rope_theta=100000.0, **common)
        cls = transformers.LlamaForCausalLM
    else:
        # head_dim 32 differs from hidden_size / heads = 16: the independent head size is exercised.
        cfg = transformers.Qwen3Config(head_dim=32, rope_theta=1000000.0, **common)
        cls = transformers.Qwen3ForCausalLM
    torch.manual_seed(0)
    return cls(cfg).eval()


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--arch", choices=["llama", "qwen3"], required=True)
    ap.add_argument("--out", required=True)
    ap.add_argument("--tied", action="store_true")
    a = ap.parse_args()
    model = build(a.arch, a.tied)
    model.save_pretrained(a.out, safe_serialization=True)
    ids = [[3, 17, 42, 99, 5, 64, 127, 0, 8, 21]]
    with torch.no_grad():
        logits = model(torch.tensor(ids)).logits[0].float()
    with open(os.path.join(a.out, "logits.json"), "w") as f:
        json.dump({"ids": ids, "logits": logits.tolist()}, f)


if __name__ == "__main__":
    main()
