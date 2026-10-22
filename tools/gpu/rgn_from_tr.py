"""TR'de eğitilen hedef modeli PyTorch'ta açar: doğrulama kaybını hedefin kendi koduyla ölçer, metin üretir.

TR'nin bildirdiği doğrulama kaybı TR'nin kendi hesabıdır; bu araç aynı ağırlıkları rgn512.py'nin
modeline yükleyip kaybı rgn512.py'nin evaluate'iyle (aynı pencereler, bf16 autocast) yeniden hesaplar.
İki sayı yuvarlama düzeyinde aynı olmalıdır. Sonra aynı istemlerle TR modelinden ve (verilirse)
PyTorch'ta eğitilen modelden metin üretir: açgözlü ve örneklemeli (sıcaklık, en iyi k, tohum sabit).

    ./tr rgn512.tr ... --kaydet tr_son.trt
    python rgn_from_tr.py --trt tr_son.trt --valid valid.npy --tokenizer tr_bpe_40k_v2.json --pt k_pt_1/model.pt

Ağırlık adları rgn_reference.py'deki tr_name_map'tedir (TR'nin Doğrusal ağırlığı [giriş, çıkış],
PyTorch'unki [çıkış, giriş]: devrilir).
"""
import argparse
import os
import sys

import numpy as np
import torch

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import rgn512  # noqa: E402
from rgn_reference import tr_name_map  # noqa: E402
from trt_io import bf16_bits_to_float32, read_trt  # noqa: E402

PROMPTS = ["Türkiye'nin başkenti", "Bir zamanlar küçük bir köyde", "Yapay zekâ", "İstanbul'da sabah",
           "Bilim insanları yeni bir"]


def model_from_trt(path, cfg):
    """TR ağırlık dosyasından rgn512.RGN kurar; her parametre dosyadan gelmeli, dosyada fazlası olmamalı."""
    tensors = read_trt(path)
    model = rgn512.RGN(cfg)
    params = dict(model.named_parameters())
    state, used = {}, set()
    for pt_name, (tr_name, transpose) in tr_name_map(cfg.n_layers).items():
        if pt_name not in params:
            continue
        a, dt = tensors[tr_name]
        a = bf16_bits_to_float32(a) if dt == "bf16" else np.asarray(a, dtype=np.float32)
        state[pt_name] = torch.from_numpy(np.array(a.T if transpose else a, dtype=np.float32, copy=True))
        used.add(tr_name)
    missing = sorted(set(params) - set(state))
    extra = sorted(set(tensors) - used)
    if missing or extra:
        raise SystemExit("ağırlıklar eşleşmedi: PyTorch'ta karşılıksız %s, dosyada fazla %s" % (missing, extra))
    for k, v in state.items():
        if tuple(v.shape) != tuple(params[k].shape):
            raise SystemExit("%s: şekil %s, beklenen %s" % (k, tuple(v.shape), tuple(params[k].shape)))
    model.load_state_dict(state, strict=False)
    return model


def main():
    ap = argparse.ArgumentParser(description="TR'de eğitilen hedef model: PyTorch'ta doğrulama kaybı ve metin")
    ap.add_argument("--trt", required=True, help="rgn512.tr --kaydet dosyası")
    ap.add_argument("--valid", default="", help="doğrulama .npy (boşsa kayıp ölçülmez)")
    ap.add_argument("--tokenizer", default="", help="tokenizer.json (boşsa metin üretilmez)")
    ap.add_argument("--pt", default="", help="PyTorch'ta eğitilen model.pt (karşılaştırma için)")
    ap.add_argument("--n", type=int, default=60, help="üretilecek parça sayısı")
    ap.add_argument("--temperature", type=float, default=0.8)
    ap.add_argument("--top_k", type=int, default=50)
    ap.add_argument("--seed", type=int, default=0)
    ap.add_argument("--vocab", type=int, default=40000)
    ap.add_argument("--d", type=int, default=512)
    ap.add_argument("--heads", type=int, default=8)
    ap.add_argument("--kv-heads", type=int, default=4)
    ap.add_argument("--ff", type=int, default=672)
    ap.add_argument("--layers", type=int, default=8)
    ap.add_argument("--seq", type=int, default=1024)
    ap.add_argument("--eval-tokens", type=int, default=4_194_304)
    a = ap.parse_args()

    dev = torch.device("cuda" if torch.cuda.is_available() else "cpu")
    cfg = rgn512.Config(vocab_size=a.vocab, d_model=a.d, n_layers=a.layers, n_heads=a.heads, n_kv_heads=a.kv_heads,
                        d_ff=a.ff, ple_dim=a.d)
    models = {"TR": model_from_trt(a.trt, cfg).to(dev).eval()}
    if a.pt:
        models["PyTorch"] = rgn512.load(a.pt, dev)
    ctx = torch.autocast("cuda", dtype=torch.bfloat16) if dev.type == "cuda" else torch.autocast("cpu", enabled=False)

    if a.valid:
        tc = rgn512.TrainConfig(seq_len=a.seq, eval_tokens=a.eval_tokens)
        valid = rgn512.Tokens(a.valid).eval_windows(tc.seq_len, tc.eval_tokens)
        for name, m in models.items():
            print("doğrulama kaybı (rgn512.py evaluate) %-8s %.4f" % (name, rgn512.evaluate(m, valid, tc, dev, ctx)), flush=True)

    if a.tokenizer:
        from tokenizers import Tokenizer
        tok = Tokenizer.from_file(a.tokenizer)
        for p in PROMPTS:
            ids = torch.tensor([tok.encode(p, add_special_tokens=False).ids], device=dev)
            print("\n### " + p)
            for kind, temp in (("açgözlü", 0.0), ("örneklemeli", a.temperature)):
                for name, m in models.items():
                    torch.manual_seed(a.seed)
                    with ctx:
                        out = m.generate(ids, a.n, temp, a.top_k, m.cfg.eos_id)
                    text = tok.decode(out[0].tolist()).replace("\n", " ⏎ ")
                    print("[%s · %s] %s" % (name, kind, text))


if __name__ == "__main__":
    main()
