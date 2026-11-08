"""Builds the small random VITS model of tests/data/vits_tiny: a Hugging Face VitsModel from a config,
weights from a fixed seed, saved as float16 with the layout of a published MMS-TTS checkpoint.

The layout is kept on purpose (DENETİM 270 Ş7, 271 Ş1): the stochastic duration predictor with its
spline flows, add_blank, and the weight norm pairs stored as `weight_g` / `weight_v` (save_pretrained
writes `parametrizations.weight.original0/1`; published checkpoints use the old names, which
transformers still loads). The vocabulary is this repository's own, with U+0307 as its own token like
a model trained on Python's str.lower(). Nothing comes from a published model.

    python -I tools/hf/small_vits.py --out tests/data/vits_tiny
    python -I tools/hf/vits_reference.py --model tests/data/vits_tiny --out DIR \
        --sentence "kisa=İşte ılık süt" --sentence "uzun=Ağır çözüm, büyük iş!"

No network. The same command gives the same files for the same torch and transformers versions.
"""
import argparse
import json
import os

import torch

# id 0 is the blank that add_blank puts between characters, as in MMS-TTS.
VOCAB = ["_", " ", "a", "b", "c", "ç", "e", "ğ", "ı", "i", "\u0307", "k", "l", "m", "ö", "r", "s", "ş", "t", "u", "ü", "z"]


def build(seed):
    import transformers
    cfg = transformers.VitsConfig(
        vocab_size=len(VOCAB), hidden_size=8, num_hidden_layers=1, num_attention_heads=2, window_size=4,
        ffn_dim=12, ffn_kernel_size=3, flow_size=4, spectrogram_bins=5,
        upsample_initial_channel=8, upsample_rates=[2, 2], upsample_kernel_sizes=[4, 4],
        resblock_kernel_sizes=[3, 5], resblock_dilation_sizes=[[1, 3], [1, 3]],
        prior_encoder_num_flows=2, prior_encoder_num_wavenet_layers=2, posterior_encoder_num_wavenet_layers=1,
        wavenet_kernel_size=3, wavenet_dilation_rate=2,
        duration_predictor_num_flows=2, duration_predictor_flow_bins=4, depth_separable_num_layers=2,
        duration_predictor_filter_channels=8, use_stochastic_duration_prediction=True,
        noise_scale=0.667, noise_scale_duration=0.8, speaking_rate=1.0, sampling_rate=16000)
    torch.manual_seed(seed)
    return transformers.VitsModel(cfg).eval()


def legacy_names(state):
    out = {}
    for k, v in state.items():
        k = k.replace("parametrizations.weight.original0", "weight_g").replace("parametrizations.weight.original1", "weight_v")
        out[k] = v.detach().to(torch.float16).contiguous()
    return out


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--out", required=True)
    ap.add_argument("--seed", type=int, default=0)
    a = ap.parse_args()
    from safetensors.torch import save_file
    model = build(a.seed)
    os.makedirs(a.out, exist_ok=True)
    model.config.save_pretrained(a.out)
    save_file(legacy_names(model.state_dict()), os.path.join(a.out, "model.safetensors"), metadata={"format": "pt"})
    with open(os.path.join(a.out, "vocab.json"), "w", encoding="utf-8", newline="\n") as f:
        json.dump({c: i for i, c in enumerate(VOCAB)}, f, ensure_ascii=False, indent=1)
        f.write("\n")
    tok = {"tokenizer_class": "VitsTokenizer", "add_blank": True, "normalize": True, "phonemize": False,
           "is_uroman": False, "language": "tur", "pad_token": "_", "unk_token": "_"}
    with open(os.path.join(a.out, "tokenizer_config.json"), "w", encoding="utf-8", newline="\n") as f:
        json.dump(tok, f, ensure_ascii=False, indent=1)
        f.write("\n")


if __name__ == "__main__":
    main()
