"""Runs a Hugging Face VITS model (VitsModel) on fixed sentences and writes every value that lib/vits.tr
must reproduce: token ids, durations before and after ceil, the text encoder's prior, the two noise
tensors, the latents after the flow and the waveform.

Used for the speech synthesis slice (facebook/mms-tts-tur) and for the small test model in
tests/data/vits_tiny. Each sentence runs twice: without noise (noise_scale=0, noise_scale_duration=0)
and with the model's noise. Each run is done in float32 and float64 with the same noise: the float32
to float64 gap is the base of every comparison.

    python -I tools/hf/vits_reference.py --model DIR_OR_HUB_ID [--revision SHA] --out DIR [--wav]

The two randn calls of VitsModel.forward are recorded (float32, from --seed) and replayed in float64.
Writes DIR/<sentence>_<run>_<dtype>.safetensors, DIR/meta.json and, with --wav, 16-bit WAV files of
the float32 runs. No file of the model is written to DIR.
"""
import argparse
import json
import math
import os
import platform
import struct
import sys

import torch

SENTENCES = {
    "kisa": "Bugün hava güzel",
    "orta": "Yarın sabah erkenden kalkıp markete gideceğim ve biraz ekmek ile süt alacağım",
    "harfli": "Işıl ile İsmail çok güzel şarkı söyleyip dönüşte ağladı",
}


class Noise:
    """Records the randn calls of one forward pass, or replays recorded ones in another dtype."""

    def __init__(self, seed, replay=None, dtype=torch.float32, tail=None):
        self.gen = torch.Generator().manual_seed(seed)
        self.tail = tail
        self.replay = replay
        self.dtype = dtype
        self.taken = []
        self.fresh = False

    def _next(self, shape):
        i = len(self.taken)
        if self.replay is not None and i < len(self.replay) and tuple(self.replay[i].shape) == tuple(shape):
            t = self.replay[i]
        else:
            self.fresh = self.fresh or self.replay is not None
            t = self.randn(*shape, generator=self.gen, dtype=torch.float32)
            if i == 0 and self.tail is not None:
                # One duration noise value past the spline's tail bound: the identity branch is used.
                t[0, 0, min(1, t.shape[2] - 1)] = self.tail
        self.taken.append(t)
        return t.to(self.dtype)

    def __enter__(self):
        self.randn, self.randn_like = torch.randn, torch.randn_like
        torch.randn = lambda *shape, **kw: self._next(tuple(shape[0]) if len(shape) == 1 and isinstance(shape[0], (tuple, list, torch.Size)) else shape)
        torch.randn_like = lambda x, **kw: self._next(tuple(x.shape))
        return self

    def __exit__(self, *a):
        torch.randn, torch.randn_like = self.randn, self.randn_like


def capture(model):
    """Forward hooks keep the intermediate values of one pass."""
    got = {}
    hooks = [
        model.text_encoder.register_forward_hook(lambda m, i, o: got.update(
            hidden=o.last_hidden_state[0], prior_means=o.prior_means[0], prior_log_variances=o.prior_log_variances[0])),
        model.duration_predictor.register_forward_hook(lambda m, i, o: got.update(log_duration=o[0, 0])),
        model.flow.register_forward_pre_hook(lambda m, a: got.update(prior_latents=a[0][0])),
        model.flow.register_forward_hook(lambda m, i, o: got.update(latents=o[0])),
    ]
    return got, hooks


def run(model, ids, noise_scale, noise_duration, noise):
    model.noise_scale, model.noise_scale_duration = noise_scale, noise_duration
    got, hooks = capture(model)
    try:
        with torch.no_grad(), noise:
            out = model(torch.tensor([ids]))
    finally:
        for h in hooks:
            h.remove()
    got["waveform"] = out.waveform[0]
    got["noise_duration"], got["noise_prior"] = noise.taken[0][0], noise.taken[1][0]
    got["before_ceil"] = torch.exp(got["log_duration"]) / model.speaking_rate
    got["duration"] = torch.ceil(got["before_ceil"]).to(torch.int64)
    got["ids"] = torch.tensor(ids, dtype=torch.int64)
    return got


def snr(ref, x):
    n = min(ref.numel(), x.numel())
    ref, x = ref[:n].double(), x[:n].double()
    e = float(torch.sum((ref - x) ** 2))
    return math.inf if e == 0 else 10 * math.log10(float(torch.sum(ref ** 2)) / e)


def write_wav(path, wave, rate):
    pcm = (wave.clamp(-1, 1) * 32767).round().to(torch.int16).tolist()
    data = struct.pack("<%dh" % len(pcm), *pcm)
    with open(path, "wb") as f:
        f.write(b"RIFF" + struct.pack("<I", 36 + len(data)) + b"WAVEfmt " + struct.pack("<IHHIIHH", 16, 1, 1, rate, rate * 2, 2, 16))
        f.write(b"data" + struct.pack("<I", len(data)) + data)


def tokenize(tok, text):
    ids = tok(text)["input_ids"]
    kept = tok.prepare_for_tokenization(text)[0]
    lowered = tok.normalize_text(text) if tok.normalize else text
    dropped = sorted({c for c in lowered if c not in tok.encoder})
    return ids, kept, dropped


def compare(out, tr_dir):
    """Reads TR's <case>_tr.safetensors next to the reference and writes compare.json: where the largest
    hidden difference is (token, channel) and the largest before-ceil duration difference against float32."""
    from safetensors.torch import load_file
    with open(os.path.join(out, "meta.json"), encoding="utf-8") as f:
        meta = json.load(f)
    rate = meta["config"].get("speaking_rate", 1.0)
    res = {}
    for case, c in meta["cases"].items():
        p32 = load_file(os.path.join(out, f"{case}_f32.safetensors"))
        tr = load_file(os.path.join(tr_dir, f"{case}_tr.safetensors"))
        d = (tr["hidden"].double() - p32["hidden"].double()).abs()
        token, channel = divmod(int(torch.argmax(d)), d.shape[1])
        b_tr = torch.exp(tr["log_duration"].double()) / rate
        b_pt = torch.tensor(c["before_ceil_f32"], dtype=torch.float64)
        db = (b_tr - b_pt).abs()
        i = int(torch.argmax(db))
        res[case] = {
            "hidden_max_abs": float(d.max()), "hidden_token": token, "hidden_channel": channel, "hidden_id": c["ids"][token],
            "hidden_value_f32": float(p32["hidden"][token, channel]),
            "before_ceil_max_abs": float(db[i]), "before_ceil_token": i,
            "before_ceil_f32": float(b_pt[i]), "before_ceil_tr": float(b_tr[i]),
            "before_ceil_margin": float(torch.min(torch.abs(b_pt - torch.round(b_pt)))),
        }
    with open(os.path.join(tr_dir, "compare.json"), "w", encoding="utf-8") as f:
        json.dump(res, f, ensure_ascii=False, indent=1)
    for case, r in res.items():
        sys.stdout.buffer.write((f"{case}: hidden {r['hidden_max_abs']:.2e} at token {r['hidden_token']} (id {r['hidden_id']}) "
                                 f"channel {r['hidden_channel']} value {r['hidden_value_f32']:.4g}; before_ceil {r['before_ceil_max_abs']:.2e} "
                                 f"at token {r['before_ceil_token']}, margin {r['before_ceil_margin']:.2e}\n").encode("utf-8"))


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--model", required=True)
    ap.add_argument("--revision", default=None)
    ap.add_argument("--out", required=True)
    ap.add_argument("--seed", type=int, default=0)
    ap.add_argument("--wav", action="store_true")
    ap.add_argument("--sentence", action="append", default=[], help="name=text; replaces the fixed sentences")
    ap.add_argument("--tail-noise", type=float, default=None, help="puts this value into the duration noise of the noisy run")
    ap.add_argument("--keep", default="", help="comma separated tensor names to write (default: all)")
    ap.add_argument("--f32-only", action="store_true", help="write only the float32 files (the float64 run still happens)")
    ap.add_argument("--compare", default=None, help="TR output folder: compare it with the reference already in --out, no model run")
    a = ap.parse_args()
    if a.compare:
        return compare(a.out, a.compare)
    import transformers
    from safetensors.torch import save_file
    os.makedirs(a.out, exist_ok=True)
    tok = transformers.VitsTokenizer.from_pretrained(a.model, revision=a.revision)
    m32 = transformers.VitsModel.from_pretrained(a.model, revision=a.revision).eval().float()
    m64 = transformers.VitsModel.from_pretrained(a.model, revision=a.revision).eval().double()
    sentences = dict(s.split("=", 1) for s in a.sentence) if a.sentence else SENTENCES
    meta = {
        "versions": {"python": platform.python_version(), "torch": torch.__version__,
                     "transformers": transformers.__version__, "threads": torch.get_num_threads()},
        "model": a.model, "revision": a.revision, "seed": a.seed, "tail_noise": a.tail_noise,
        "tokenizer": {k: getattr(tok, k) for k in ("normalize", "add_blank", "phonemize", "is_uroman", "language")},
        "vocab_size": len(tok.encoder), "config": m32.config.to_dict(), "cases": {},
    }
    meta["config"].pop("id2label", None), meta["config"].pop("label2id", None)
    rate = m32.config.sampling_rate
    for name, text in sentences.items():
        ids, kept, dropped = tokenize(tok, text)
        for run_name, ns, nd in (("sessiz", 0.0, 0.0), ("gurultulu", m32.config.noise_scale, m32.config.noise_scale_duration)):
            r32 = run(m32, ids, ns, nd, Noise(a.seed, tail=a.tail_noise if ns else None))
            replay = Noise(a.seed, replay=[r32["noise_duration"][None], r32["noise_prior"][None]], dtype=torch.float64)
            r64 = run(m64, ids, ns, nd, replay)
            case = f"{name}_{run_name}"
            keep = set(a.keep.split(",")) if a.keep else None
            for dt, r in (("f32", r32), ("f64", r64)):
                if dt == "f64" and a.f32_only:
                    continue
                save_file({k: v.contiguous() for k, v in r.items() if keep is None or k in keep}, os.path.join(a.out, f"{case}_{dt}.safetensors"))
            b = r32["before_ceil"].double()
            meta["cases"][case] = {
                "text": text, "kept": kept, "dropped": dropped, "ids": ids,
                "noise_scale": ns, "noise_scale_duration": nd,
                "duration_f32": r32["duration"].tolist(), "duration_f64": r64["duration"].tolist(),
                "durations_equal": r32["duration"].tolist() == r64["duration"].tolist(),
                "f64_noise_fresh": replay.fresh,
                "before_ceil_f32": b.tolist(),
                "closest_to_integer": float(torch.min(torch.abs(b - torch.round(b)))),
                "frames": int(r32["duration"].sum()), "samples": int(r32["waveform"].numel()),
                "wave_max_abs_f32_f64": float(torch.max(torch.abs(r32["waveform"].double()[: r64["waveform"].numel()] - r64["waveform"][: r32["waveform"].numel()]))) if r32["waveform"].numel() == r64["waveform"].numel() else None,
                "wave_snr_f32_f64_db": snr(r64["waveform"], r32["waveform"]) if r32["waveform"].numel() == r64["waveform"].numel() else None,
            }
            if a.wav:
                write_wav(os.path.join(a.out, f"{case}.wav"), r32["waveform"], rate)
    with open(os.path.join(a.out, "meta.json"), "w", encoding="utf-8") as f:
        json.dump(meta, f, ensure_ascii=False, indent=1, default=str)
    for case, c in meta["cases"].items():
        sys.stdout.buffer.write((f"{case}: ids {len(c['ids'])} frames {c['frames']} samples {c['samples']} "
                                 f"durations_equal {c['durations_equal']} closest {c['closest_to_integer']:.2e} "
                                 f"max_abs {c['wave_max_abs_f32_f64']} snr {c['wave_snr_f32_f64_db']} dropped {c['dropped']}\n").encode("utf-8"))


if __name__ == "__main__":
    main()
