"""TR_resample'ın soxr başvurusunu üretir (dil tanımı 22.4): python generate.py <çıktı klasörü>.

Girdi: sabit tohumlu gürültü (0,1 genlikli) ve aynı gürültünün küçük hızın Nyquist'inin 0,9 katının altı,
0,25 saniye, 16 bite yuvarlanmış. Beklenen: soxr.resample(quality="HQ"), float64 girdi, float32 sonuç.
"""
import sys

import numpy as np
import soxr
from safetensors.numpy import save_file

RATES = ((24000, 16000), (24000, 32000), (44100, 24000), (16000, 24000))

rng = np.random.default_rng(2204)
out = {}
for src, dst in RATES:
    n = src // 4
    noise = rng.standard_normal(n) * 0.1
    spec = np.fft.rfft(noise)
    spec[np.fft.rfftfreq(n, 1 / src) > 0.9 * min(src, dst) / 2] = 0
    for kind, x in (("noise", noise), ("band", np.fft.irfft(spec, n))):
        pcm = np.round(x * 32768).astype(np.int16)
        out[f"{kind}_{src}_{dst}"] = pcm
        y = soxr.resample(pcm.astype(np.float64) / 32768, src, dst, quality="HQ")
        out[f"soxr_{kind}_{src}_{dst}"] = y.astype(np.float32)
save_file(out, sys.argv[1] + "/resample.safetensors")
print("numpy", np.__version__, "soxr", soxr.__version__, {k: v.shape for k, v in out.items()})
