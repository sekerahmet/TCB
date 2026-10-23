#!/usr/bin/env bash
# CPU temel ölçümü: karakter modeli (TR ve PyTorch; 1, 2, 4, 8 iş parçacığı), dil kıyası ve tensör
# kernelleri, 5'er tekrar. tools/record ile sarılır; sonuçlar TR_KAYIT_KLASORU'na yazılır, özeti
# metrics.json'dadır (sürümler, model, tensör ortancası ve sapması, dil kıyası).
#
#   go run ./tools/record --name cpu-temel --out <kayıt kökü> -- bash tools/bench/cpu_baseline.sh
#
# Rakip en güncel hâliyle ölçülür: PyTorch'un sürümü kayda yazılır; Colab hücresi PyPI'daki son sürümü
# kurar (tools/gpu/COLAB.md §0).
set -u
cd "$(dirname "$0")/../.."
out=${TR_KAYIT_KLASORU:?tools/record ile çalıştırın}
python3 -c "import sys, json, platform, torch, numpy; json.dump({'python': platform.python_version(), 'torch': torch.__version__, 'numpy': numpy.__version__}, open(sys.argv[1], 'w'))" "$out/versions.json"
for n in 1 2 4 8; do
  echo "== Karakter modeli (TR/PyTorch, $n iş parçacığı, 5 tekrar)"
  python3 tools/bench/run.py karakter_modeli --threads "$n" --repeat 5 --json "$out/model_$n.json"
done
echo "== Dil kıyası (5 tekrar)"
python3 tools/bench/run.py --repeat 5 dongu fib sirala metin sozluk matris_dongu matris_yerlesik --json "$out/bench.json"
echo "== Tensör kernelleri (BenchmarkSpeed*, 5 tekrar)"
go test -run XXX -bench 'BenchmarkSpeed' -count 5 ./internal/compute/tensor/ | tee "$out/tensor_bench.txt"
echo "== Özet"
python3 - "$out" <<'PY'
import json, os, re, statistics as st, sys
d = sys.argv[1]
load = lambda f: json.load(open(f'{d}/{f}', encoding='utf-8')) if os.path.exists(f'{d}/{f}') else None
runs = {}
for line in open(f'{d}/tensor_bench.txt', encoding='utf-8'):
    m = re.match(r'^(Benchmark\S+?)(?:-\d+)?\s+\d+\s+([\d.]+) ns/op', line)
    if m:
        runs.setdefault(m.group(1), []).append(float(m.group(2)))
tensor = {k: {'median_ns': st.median(v), 'cv_pct': round(100 * st.pstdev(v) / st.mean(v), 2), 'n': len(v)} for k, v in runs.items()}
model = {n: (load(f'model_{n}.json') or {}).get('karakter_modeli') for n in (1, 2, 4, 8)}
json.dump({'versions': load('versions.json'), 'model': model, 'tensor': tensor, 'bench': load('bench.json')},
          open(f'{d}/metrics.json', 'w', encoding='utf-8'), ensure_ascii=False, indent=1)
print('sürümler:', load('versions.json'))
for n, r in model.items():
    if r:
        print(f"model {n} iş parçacığı: TR {r['tr'][0]} / PyTorch {r['py'][0]} ms/adım, kayıp {r['tr'][1]} / {r['py'][1]}")
cvs = sorted(v['cv_pct'] for v in tensor.values())
print(f"tensör kıyasları: {len(tensor)}, CV ortanca %{st.median(cvs):.2f}, en kötü %{cvs[-1]:.2f}")
PY
