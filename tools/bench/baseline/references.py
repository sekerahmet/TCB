"""Baseline measurement set references in Python: the same work as the TR programs, the same BASELINE fields.

Usage (run.py builds these commands; every subcommand takes --threads, the machine's one number, O-1):
  python -I tools/bench/baseline/references.py lm --folder DIR --text T --threads N [--mode speed|agree]
  python -I tools/bench/baseline/references.py whisper_hf --model DIR --audio WAV --threads N --language tr
  python -I tools/bench/baseline/references.py faster_whisper --model DIR --audio WAV --compute_type float32 --threads N --language tr
  python -I tools/bench/baseline/references.py mms --model DIR --texts texts.json --threads N --language tr
  python -I tools/bench/baseline/references.py piper --model VOICE.onnx --ids IDS.json --texts texts.json --language en --threads N
  python -I tools/bench/baseline/references.py matmul --size 4096 --dtype bf16 --threads N
  python -I tools/bench/baseline/references.py threads N     (the settings each library gets; the test reads this)

Each program times what its TR program times (tools/bench/baseline/README.md): no in-process warm-up (the set's
warm-up is one run per process), synchronisation included, the result on the last `BASELINE {json}` line with the
library versions. The heavy libraries are imported only inside their subcommand.
"""
import argparse, json, statistics, sys, time


def thread_settings(n):
    """How the machine's one thread number reaches each library, by its own name (O-1)."""
    return {"torch.set_num_threads": n,
            "faster_whisper.WhisperModel": {"cpu_threads": n, "num_workers": 1},
            "onnxruntime.SessionOptions": {"intra_op_num_threads": n, "inter_op_num_threads": 1}}


def now_ms():
    return time.perf_counter() * 1000


def model_device(model):
    """Where the model's weights really are (Ş2, DENETİM 944): torch's parameter device, not the requested one."""
    return next(model.parameters()).device.type


def emit(result):
    # Koşturucu çıktıyı UTF-8 okur (run.py run_once); Windows'ta borunun kodlaması cp1254'tür ve `python -I`
    # PYTHONIOENCODING'i yok sayar: dizüstünde Whisper'ın Türkçe metni bozuldu, S5'in uyuşması 58 sözcük ayrı çıktı.
    sys.stdout.reconfigure(encoding="utf-8")
    print("BASELINE " + json.dumps(result, ensure_ascii=False))


def versions(*names):
    out = {"python": sys.version.split()[0]}
    for name in names:
        try:
            out[name] = __import__(name).__version__
        except Exception:
            out[name] = None
    return out


def torch_setup(a):
    import torch
    torch.set_num_threads(thread_settings(a.threads)["torch.set_num_threads"])
    device = torch.device(a.device)
    sync = (lambda: torch.cuda.synchronize()) if device.type == "cuda" else (lambda: None)
    return torch, device, sync


def sentences(a):
    """The set's sentences as mms.tr and piper.tr read them: a list as it is, a paragraph split at ". "."""
    with open(a.texts, encoding="utf-8") as f:
        got = json.load(f)[a.set][a.language]
    if isinstance(got, str):
        got = [s.strip(".") + "." for s in got.split(". ")]
    if not got:
        raise SystemExit(f"{a.texts} has no sentence under {a.set}.{a.language}")
    return got


def peak(wave):
    """The TR programs' peak normalisation (lib/vits.tr _tepe_normla): top 1, clipped to [-1, 1]."""
    import numpy as np
    top = float(np.max(np.abs(wave))) if wave.size else 0.0
    return np.zeros_like(wave) if top < 1e-8 else np.clip(wave / top, -1.0, 1.0)


def voice_result(a, rows, rate, load_ms, extra):
    total = sum(r["ms"] for r in rows)
    samples = sum(r["samples"] for r in rows)
    audio_s = samples / rate
    return {"device": a.device, "language": a.language, "noise": a.noise, "load_ms": load_ms, "sentences": rows,
            "first_ms": rows[0]["ms"], "total_ms": total, "samples": samples, "sample_rate": rate, "audio_s": audio_s,
            "audio_per_compute": audio_s / (total / 1000) if total > 0 else None, **extra}


# ---------------------------------------------------------------- S1–S3: language model (transformers)

def lm(a):
    """language_model.tr's speed and agree modes: the prompt in one call, then greedy steps over the kept cache."""
    torch, device, sync = torch_setup(a)
    from transformers import AutoModelForCausalLM, AutoTokenizer
    t0 = now_ms()
    tok = AutoTokenizer.from_pretrained(a.folder)
    model = AutoModelForCausalLM.from_pretrained(a.folder, dtype=getattr(torch, a.dtype)).to(device).eval()
    sync()
    load_ms = now_ms() - t0
    once = tok(a.text, add_special_tokens=False)["input_ids"]
    need = a.prompt + a.gen if a.mode == "speed" else len(once)
    ids = []
    while len(ids) < max(need, 1):
        ids += once
    result = {"mode": a.mode, "device": a.device, "load_ms": load_ms}
    with torch.no_grad():
        P = a.prompt if a.mode == "speed" else len(once)
        G = a.gen if a.mode == "speed" else a.agree_tokens
        b = now_ms()
        out = model(torch.tensor([ids[:P]], device=device), use_cache=True)
        x = int(out.logits[0, -1].argmax())
        sync()
        read_ms = now_ms() - b
        cache, made = out.past_key_values, []
        b = now_ms()
        for i in range(G):
            made.append(x)
            if a.mode == "agree" and i == G - 1:
                break
            out = model(torch.tensor([[x]], device=device), past_key_values=cache, use_cache=True)
            cache, x = out.past_key_values, int(out.logits[0, -1].argmax())
        sync()
        gen_ms = now_ms() - b
    if a.mode == "speed":
        result.update({"prompt_tokens": P, "prompt_ms": read_ms, "gen_tokens": G, "gen_depth": P, "gen_ms": gen_ms,
                       "tokens": made[:32]})
    else:
        result.update({"prompt_tokens": P, "tokens": made, "text": tok.decode(made)})
    result["versions"] = versions("torch", "transformers")
    result["actual_device"] = model_device(model)
    emit(result)


# ---------------------------------------------------------------- S5: speech to text

def read_wav(path):
    import wave, numpy as np
    with wave.open(path, "rb") as w:
        if w.getsampwidth() != 2 or w.getnchannels() != 1:
            raise SystemExit(f"{path}: want 16-bit mono, the set's fixed sounds (O-7)")
        rate, data = w.getframerate(), w.readframes(w.getnframes())
    return np.frombuffer(data, dtype="<i2").astype("float32") / 32768.0, rate


def whisper_hf(a):
    """whisper.tr's work with transformers' Whisper: greedy, one beam, no sampling."""
    torch, device, sync = torch_setup(a)
    from transformers import WhisperForConditionalGeneration, WhisperProcessor
    t0 = now_ms()
    processor = WhisperProcessor.from_pretrained(a.model)
    model = WhisperForConditionalGeneration.from_pretrained(a.model, dtype=torch.float32).to(device).eval()
    sync()
    load_ms = now_ms() - t0
    audio, rate = read_wav(a.audio)
    b = now_ms()
    with torch.no_grad():
        features = processor(audio, sampling_rate=rate, return_tensors="pt").input_features.to(device)
        ids = model.generate(features, language=a.language, task="transcribe", num_beams=1, do_sample=False)
        text = processor.batch_decode(ids, skip_special_tokens=True)[0]
    sync()
    ms = now_ms() - b
    emit({"device": a.device, "language": a.language, "load_ms": load_ms, "audio_s": len(audio) / rate,
          "transcribe_ms": ms, "audio_per_compute": len(audio) / rate / (ms / 1000), "text": text,
          "versions": versions("torch", "transformers"), "actual_device": model_device(model)})


def faster_whisper(a):
    """whisper.tr's work with faster-whisper: beam 1, temperature 0, no VAD filter; the segments are read inside the time."""
    from faster_whisper import WhisperModel
    s = thread_settings(a.threads)["faster_whisper.WhisperModel"]
    t0 = now_ms()
    model = WhisperModel(a.model, device=a.device, compute_type=a.compute_type, **s)
    load_ms = now_ms() - t0
    audio, rate = read_wav(a.audio)
    b = now_ms()
    segments, _ = model.transcribe(audio, language=a.language, beam_size=1, temperature=0.0, vad_filter=False)
    parts = [seg.text for seg in segments]
    ms = now_ms() - b
    emit({"device": a.device, "language": a.language, "compute_type": a.compute_type, "load_ms": load_ms,
          "audio_s": len(audio) / rate, "transcribe_ms": ms, "audio_per_compute": len(audio) / rate / (ms / 1000),
          "segments": len(parts), "text": "".join(parts), "versions": versions("faster_whisper", "ctranslate2"),
          "actual_device": model.model.device})


# ---------------------------------------------------------------- S6, S7: text to speech

def mms_ids(tok, text):
    """MMS's input as VITS builds it and TR does (lib/vits.tr _parçala): the tokenizer's characters with the blank
    (id 0) before, between and after them. VitsTokenizer's own blanks differ when the pad token is a letter
    (mms-tts-eng: pad "k", id 0): it keeps a "k" as a special token and a "k" at the text's start or end loses its
    blank ("week" gives 8 ids, not 9; A-11, devir B13). The characters come from the tokenizer without blanks."""
    blank = tok.add_blank
    tok.add_blank = False
    try:
        chars = tok.convert_tokens_to_ids(tok.tokenize(text))
    finally:
        tok.add_blank = blank
    if not blank:
        return chars
    out = [0]
    for c in chars:
        out += [c, 0]
    return out


def mms(a):
    """mms.tr's work with transformers' VitsModel; --count_random counts torch.randn and randn_like calls (A-4)."""
    torch, device, sync = torch_setup(a)
    from transformers import AutoTokenizer, VitsModel
    t0 = now_ms()
    tok = AutoTokenizer.from_pretrained(a.model)
    # float32 as TR computes (lib/vits.tr: "Hesap float32'dedir"); transformers 5 would keep a float16 file's type.
    model = VitsModel.from_pretrained(a.model, dtype=torch.float32).to(device).eval()
    if a.noise == "zero":
        model.noise_scale, model.noise_scale_duration = 0.0, 0.0
    sync()
    load_ms = now_ms() - t0
    # The count starts after loading: only the synthesis' random numbers (TR: two TR_randn per sentence).
    calls = {"randn": 0, "randn_like": 0}
    if a.count_random:
        for name in calls:
            real = getattr(torch, name)

            def counted(*args, _real=real, _name=name, **kw):
                calls[_name] += 1
                return _real(*args, **kw)
            setattr(torch, name, counted)
    torch.manual_seed(0)
    rows = []
    with torch.no_grad():
        for text in sentences(a):
            b = now_ms()
            ids = torch.tensor([mms_ids(tok, text)], device=device)
            wave = model(input_ids=ids, attention_mask=torch.ones_like(ids)).waveform[0].float().cpu().numpy()
            wave = peak(wave)
            rows.append({"ms": now_ms() - b, "samples": int(wave.size)})
    extra = {"versions": versions("torch", "transformers"), "actual_device": model_device(model)}
    if a.count_random:
        extra["random_calls"] = calls
    emit(voice_result(a, rows, model.config.sampling_rate, load_ms, extra))


def piper(a):
    """piper.tr's work with onnxruntime: the same ids, the voice's scales (noise 0 with --noise zero), peak normalised."""
    import numpy as np
    import onnxruntime as ort
    with open(a.model + ".json", encoding="utf-8") as f:
        config = json.load(f)
    inference = config.get("inference", {})
    scales = [inference.get("noise_scale", 0.667), inference.get("length_scale", 1.0), inference.get("noise_w", 0.8)]
    if a.noise == "zero":
        scales[0], scales[2] = 0.0, 0.0
    with open(a.ids, encoding="utf-8") as f:
        rows_in = json.load(f)["sentences"]
    if [r["text"] for r in rows_in] != sentences(a):
        raise SystemExit(f"{a.ids}'s sentences are not {a.texts}'s {a.set}.{a.language} sentences")
    o = ort.SessionOptions()
    for k, v in thread_settings(a.threads)["onnxruntime.SessionOptions"].items():
        setattr(o, k, v)
    providers = ["CUDAExecutionProvider", "CPUExecutionProvider"] if a.device == "cuda" else ["CPUExecutionProvider"]
    t0 = now_ms()
    session = ort.InferenceSession(a.model, sess_options=o, providers=providers)
    load_ms = now_ms() - t0
    if a.device == "cuda" and session.get_providers()[0] != "CUDAExecutionProvider":
        raise SystemExit(f"no CUDA provider: {session.get_providers()} (O-4)")
    rows = []
    for r in rows_in:
        ids = np.array([r["ids"]], dtype=np.int64)
        feed = {"input": ids, "input_lengths": np.array([ids.shape[1]], dtype=np.int64),
                "scales": np.array(scales, dtype=np.float32)}
        b = now_ms()
        wave = peak(session.run(None, feed)[0].reshape(-1))
        rows.append({"ms": now_ms() - b, "samples": int(wave.size), "ids": int(ids.shape[1])})
    emit(voice_result(a, rows, config["audio"]["sample_rate"], load_ms,
                      {"providers": session.get_providers(), "versions": versions("onnxruntime", "numpy"),
                       "actual_device": "cuda" if session.get_providers()[0] == "CUDAExecutionProvider" else "cpu"}))


# ---------------------------------------------------------------- S12: matmul

def matmul(a):
    """matmul.tr's work with torch.matmul: one element read as the synchronisation, no in-process warm-up."""
    torch, device, sync = torch_setup(a)
    dtype = {"float32": torch.float32, "bf16": torch.bfloat16}[a.dtype]
    if a.fill == "ones":
        x, y = torch.ones(a.size, a.size, dtype=dtype, device=device), torch.ones(a.size, a.size, dtype=dtype, device=device)
    else:
        g = torch.Generator(device="cpu")
        x = torch.randn(a.size, a.size, generator=g.manual_seed(1)).to(dtype).to(device)
        y = torch.randn(a.size, a.size, generator=g.manual_seed(2)).to(dtype).to(device)
    times, corner = [], 0.0
    with torch.no_grad():
        for _ in range(a.repeat):
            b = now_ms()
            corner = float(torch.matmul(x, y)[-1, -1].item())
            times.append(now_ms() - b)
    med = statistics.median(times)
    result = {"size": a.size, "dtype": a.dtype, "device": a.device, "fill": a.fill, "times_ms": times,
              "first_ms": times[0], "median_ms": med, "corner": corner, "versions": versions("torch")}
    if med > 0:
        result["gflops"] = 2.0 * a.size ** 3 / (med / 1000) / 1e9
    emit(result)


# ---------------------------------------------------------------- command line

def parser():
    p = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    sub = p.add_subparsers(dest="cmd", required=True)
    t = sub.add_parser("threads")
    t.add_argument("n", type=int)

    def side(name, func):
        s = sub.add_parser(name)
        s.add_argument("--threads", type=int, required=True)
        s.add_argument("--device", default="cpu")
        s.set_defaults(func=func)
        return s

    s = side("lm", lm)
    s.add_argument("--folder", required=True)
    s.add_argument("--text", required=True)
    s.add_argument("--mode", choices=["speed", "agree"], default="speed")
    s.add_argument("--dtype", default="bfloat16")
    s.add_argument("--prompt", type=int, default=512)
    s.add_argument("--gen", type=int, default=128)
    s.add_argument("--agree_tokens", type=int, default=32)
    for name, func in (("whisper_hf", whisper_hf), ("faster_whisper", faster_whisper)):
        s = side(name, func)
        s.add_argument("--model", required=True)
        s.add_argument("--audio", required=True)
        s.add_argument("--language", choices=["tr", "en"], default="tr")
        if name == "faster_whisper":
            s.add_argument("--compute_type", choices=["float32", "int8"], default="float32")
    for name, func in (("mms", mms), ("piper", piper)):
        s = side(name, func)
        s.add_argument("--model", required=True)
        s.add_argument("--texts", required=True)
        s.add_argument("--set", default="text_to_speech")
        s.add_argument("--language", choices=["tr", "en"], default="tr")
        s.add_argument("--noise", choices=["zero", "model"], default="zero")
        if name == "mms":
            s.add_argument("--count_random", action="store_true")
        else:
            s.add_argument("--ids", required=True)
    s = side("matmul", matmul)
    s.add_argument("--size", type=int, default=4096)
    s.add_argument("--dtype", choices=["float32", "bf16"], default="float32")
    s.add_argument("--repeat", type=int, default=5)
    s.add_argument("--fill", choices=["random", "ones"], default="random")
    return p


def main():
    a = parser().parse_args()
    if a.cmd == "threads":
        print(json.dumps(thread_settings(a.n)))
        return 0
    a.func(a)
    return 0


if __name__ == "__main__":
    sys.exit(main())
