"""Builds a small random Hugging Face model (llama, qwen3, gemma3_text or gemma3), saves it as
config.json + model.safetensors and writes the float32 logits of fixed token ids.

Used by cmd/trlang/e2e (TestReadyModelMatchesTransformers) to check that tr/pretrained loads the model
and computes the same logits, and to regenerate the tiny example model in tests/data/hf_tiny
(see tools/hf/README.md). No network: the model is built from a config, not downloaded.

    python -I tools/hf/small_model.py --arch qwen3 --out DIR [--tied]
    python -I tools/hf/small_model.py --arch llama --out DIR --tied --vocab 1005 --hidden 8 \
        --intermediate 16 --layers 1 --heads 2 --kv-heads 1 --dtype float16 --no-logits \
        --tokenizer tests/data/tokenizer.json
    python -I tools/hf/small_model.py --arch gemma3_text --out DIR
    python -I tools/hf/small_model.py --arch gemma3 --out DIR

The weights are drawn from a fixed seed (--seed, default 0): the same command gives the same files.

Gemma 3 is written in the layout of the published files (transformers 4.51: rope_theta,
rope_local_base_freq, rope_scaling, sliding_window_pattern; text weights under language_model.model.
in the multimodal file), whatever the installed transformers writes: tr/pretrained reads that layout.
Its norms get random weights (transformers starts them at 0, where a missing or doubled + 1 would
not show), the sliding window is 4 and the ids are 16 long, so local and global layers differ.
gemma3_text is like 1B (no RoPE scaling); gemma3 is like 4B (linear scaling 8 on the global layers).
"""
import argparse
import json
import os
import shutil

import torch

GEMMA_IDS = [[3, 17, 42, 99, 5, 64, 127, 0, 8, 21, 33, 77, 2, 110, 56, 90]]


def gemma_text_config(a, scaled):
    """The text config in the published layout (the keys of unsloth/gemma-3-1b-it's config.json)."""
    return dict(model_type="gemma3_text", vocab_size=a.vocab, hidden_size=a.hidden,
                intermediate_size=a.intermediate, num_hidden_layers=a.layers, num_attention_heads=a.heads,
                num_key_value_heads=a.kv_heads, head_dim=a.head_dim or 32, query_pre_attn_scalar=a.head_dim or 32,
                max_position_embeddings=a.max_position, rms_norm_eps=1e-6, rope_theta=1000000.0,
                rope_local_base_freq=10000.0, rope_scaling={"rope_type": "linear", "factor": 8.0} if scaled else None,
                sliding_window=4, sliding_window_pattern=2, hidden_activation="gelu_pytorch_tanh",
                final_logit_softcapping=None, attn_logit_softcapping=None, attention_bias=False,
                attention_dropout=0.0, cache_implementation="hybrid")


def build_gemma(a):
    import transformers
    text = gemma_text_config(a, scaled=a.arch == "gemma3")
    kwargs = {k: v for k, v in text.items() if k != "model_type"}
    if a.arch == "gemma3_text":
        cfg = transformers.Gemma3TextConfig(**kwargs)
        config = dict(architectures=["Gemma3ForCausalLM"], **text)
        cls = transformers.Gemma3ForCausalLM
    else:
        vision = dict(model_type="siglip_vision_model", hidden_size=16, intermediate_size=32, num_hidden_layers=1,
                      num_attention_heads=2, image_size=28, patch_size=14)
        top = dict(mm_tokens_per_image=4, image_token_index=a.vocab, boi_token_index=1, eoi_token_index=2)
        cfg = transformers.Gemma3Config(text_config=dict(kwargs, model_type="gemma3_text"), vision_config=vision, **top)
        config = dict(architectures=["Gemma3ForConditionalGeneration"], model_type="gemma3", text_config=text,
                      vision_config=vision, **top)
        cls = transformers.Gemma3ForConditionalGeneration
    torch.manual_seed(a.seed)
    model = cls(cfg).eval()
    with torch.no_grad():
        for name, p in model.named_parameters():
            if name.endswith("norm.weight"):
                p.normal_(0.0, 0.3)
    return model, config


def published_name(name):
    """The name of a weight in the published multimodal file (transformers 4.51), from either layout."""
    for new, old in (("model.language_model.", "language_model.model."), ("model.vision_tower.", "vision_tower."),
                     ("model.multi_modal_projector.", "multi_modal_projector.")):
        if name.startswith(new):
            return old + name[len(new):]
    return name


def save_gemma(model, config, out):
    from safetensors.torch import save_file
    os.makedirs(out, exist_ok=True)
    weights = {}
    for name, t in model.state_dict().items():
        if name.endswith("lm_head.weight"):  # tied to the embedding: the published files leave it out
            continue
        weights[published_name(name) if config["model_type"] == "gemma3" else name] = t.detach().contiguous().clone()
    save_file(weights, os.path.join(out, "model.safetensors"), metadata={"format": "pt"})
    with open(os.path.join(out, "config.json"), "w") as f:
        json.dump(config, f, indent=2)


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
    ap.add_argument("--arch", choices=["llama", "qwen3", "gemma3_text", "gemma3"], required=True)
    ap.add_argument("--out", required=True)
    ap.add_argument("--tied", action="store_true")
    ap.add_argument("--vocab", type=int, default=128)
    ap.add_argument("--hidden", type=int, default=64)
    ap.add_argument("--intermediate", type=int, default=96)
    ap.add_argument("--layers", type=int, default=2)
    ap.add_argument("--heads", type=int, default=4)
    ap.add_argument("--kv-heads", type=int, default=2)
    ap.add_argument("--head-dim", type=int, default=0, help="qwen3 and gemma3 (default 32)")
    ap.add_argument("--max-position", type=int, default=32)
    ap.add_argument("--dtype", choices=["float32", "float16", "bfloat16"], default="float32")
    ap.add_argument("--seed", type=int, default=0)
    ap.add_argument("--no-logits", action="store_true", help="do not write logits.json")
    ap.add_argument("--tokenizer", default="", help="tokenizer.json copied next to the model")
    a = ap.parse_args()
    ids = [[3, 17, 42, 99, 5, 64, 127, 0, 8, 21]]
    if a.arch.startswith("gemma3"):
        if a.arch == "gemma3_text":
            a.layers = 3  # windows [4, 0, 4]: a global layer between two local ones
        model, config = build_gemma(a)
        model = model.to(getattr(torch, a.dtype))
        save_gemma(model, config, a.out)
        ids = GEMMA_IDS
    else:
        model = build(a)
        model = model.to(getattr(torch, a.dtype))
        model.save_pretrained(a.out, safe_serialization=True)
    if a.tokenizer:
        shutil.copyfile(a.tokenizer, os.path.join(a.out, "tokenizer.json"))
    if not a.no_logits:
        with torch.no_grad():
            logits = model.float()(torch.tensor(ids)).logits[0].float()
        with open(os.path.join(a.out, "logits.json"), "w") as f:
            json.dump({"ids": ids, "logits": logits.tolist()}, f)


if __name__ == "__main__":
    main()
