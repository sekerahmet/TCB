"""PyTorch'ta eğitilen hedef modeli (rgn512.py model.pt) TR'nin ağırlık dosyasına yazar.

rgn_from_tr.py'nin tersi: PyTorch'un modelini TR'nin kendi koduyla ölçmek için. Böylece iki model ×
iki ölçücü karşılaştırılır; ölçücülerin farkı modellerin farkından ayrılır.

    python rgn_to_tr.py --pt k_pt_1/model.pt --out pt_s1.trt
    ./tr rgn512.tr --veri VERİ --doğrulama valid.npy --weight pt_s1.trt --step 0

Ağırlık adları rgn_reference.py'deki tr_name_map'tedir (Doğrusal ağırlıkları devrilir).
"""
import argparse
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import rgn512  # noqa: E402
from rgn_reference import tr_name_map  # noqa: E402
from trt_io import write_trt  # noqa: E402


def main():
    ap = argparse.ArgumentParser(description="rgn512.py model.pt -> TR ağırlık dosyası (.trt)")
    ap.add_argument("--pt", required=True, help="rgn512.py'nin yazdığı model.pt")
    ap.add_argument("--out", required=True, help="yazılacak .trt dosyası")
    a = ap.parse_args()
    model = rgn512.load(a.pt, "cpu")
    names = tr_name_map(model.cfg.n_layers)
    params = dict(model.named_parameters())
    missing = sorted(set(params) - set(names))
    if missing:
        raise SystemExit("TR karşılığı olmayan parametreler: %s" % missing)
    tensors = {}
    for k, v in params.items():
        tr_name, transpose = names[k]
        x = v.detach().float().numpy()
        tensors[tr_name] = x.T if transpose else x
    write_trt(a.out, tensors)
    print("%d tensör -> %s" % (len(tensors), a.out))


if __name__ == "__main__":
    main()
