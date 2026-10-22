"""Üretim hızı yarışının transformers tarafı (tools/gpu/generation_race.sh çağırır).

Kullanım: python generation_race_hf.py KLASÖR AD cuda|cpu eager|static
static: cache_implementation="static" (transformers kod çözme adımını kendiliğinden derler).
5 parçayla değil iki tam koşuyla ısınır (static'te derleme), üç koşunun ortancasını yazar.
"""
import statistics
import sys
import time

import torch
import transformers

d, ad, dev, mode = sys.argv[1], sys.argv[2], sys.argv[3], sys.argv[4]
n = 100
tok = transformers.AutoTokenizer.from_pretrained(d)
m = transformers.AutoModelForCausalLM.from_pretrained(d, dtype=torch.float32).eval().to(dev)
ids = tok("The capital of France is", return_tensors="pt").input_ids.to(dev)
kw = dict(max_new_tokens=n, min_new_tokens=n, do_sample=False)
if mode == "static":
    kw["cache_implementation"] = "static"


def run():
    if dev == "cuda":
        torch.cuda.synchronize()
    t = time.time()
    m.generate(ids, **kw)
    if dev == "cuda":
        torch.cuda.synchronize()
    return n / (time.time() - t)


with torch.no_grad():
    run()
    run()
    v = [run() for _ in range(3)]
print(f"{ad} | {dev} | transformers {mode} | {statistics.median(v):.1f}")
