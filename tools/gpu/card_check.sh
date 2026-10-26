#!/usr/bin/env bash
# Kart sınaması: bir GPU makinesinde (Colab, RunPod ...) tek komutla kart bilgisi, derleme, GPU testleri
# (önce her kernelden sonra eşzamanlayarak, sonra eşzamansız), dil tanımı testleri ve hedef modelde hız.
# tools/record ile sarılır: kart, sürücü ve commit kayda girer, sonuç metrics.json'dadır; kartlar ve
# zaman arasındaki karşılaştırma kayıtlardan yapılır.
#
#   go run ./tools/record --name kart --out <kayıt kökü> -- bash tools/gpu/card_check.sh
#
# Hız: tools/gpu/rgn512.tr sentetik veriyle 120 adım (ortanca ms/adım); tepe bellek nvidia-smi yoklamasıyla
# (sürecin ayırıcısı; dilden okunması yol haritası Evre 4 madde 12). GPU testleri paketin klasöründe koşar:
# bazı testler paketin kaynak dosyalarını okur. Çıkış kodu: hepsi geçtiyse 0, değilse 1.
set -u
cd "$(dirname "$0")/../.."
out=${TR_RECORD_DIR:?tools/record ile çalıştırın}
bin=$(mktemp -d)
veri=$(mktemp -d)
trap 'rm -rf "$bin" "$veri"' EXIT

echo "== Kart"
nvidia-smi --query-gpu=name,compute_cap,memory.total,driver_version --format=csv,noheader | tee "$out/kart.txt"
echo "== Derleme"
CGO_ENABLED=0 go build -trimpath -o "$bin/tr" ./cmd/tr &&
	CGO_ENABLED=0 go test -c -trimpath -o "$bin/gpu_sinama" ./internal/compute/gpu || exit 1
(cd internal/compute/gpu && "$bin/gpu_sinama" -test.run '^$' 2>&1 | head -1) # GPU ve matris çarpımı yolu

echo "== GPU testleri, eşzamanlı (TR_env_GPU_SYNC=1: hata tam o kernelde bildirilir; test ikilisi tr komutu değildir, kimlik adını okur)"
(cd internal/compute/gpu && TR_env_GPU_SYNC=1 "$bin/gpu_sinama" -test.count=1 >"$out/gpu_eszamanli.log" 2>&1)
eszamanli=$?
tail -2 "$out/gpu_eszamanli.log"
echo "== GPU testleri, eşzamansız"
(cd internal/compute/gpu && "$bin/gpu_sinama" -test.v -test.count=1 >"$out/gpu_eszamansiz.log" 2>&1)
eszamansiz=$?
grep -E '^(--- FAIL|FAIL|PASS)|panic' "$out/gpu_eszamansiz.log" | head -20
echo "== Dil tanımı testleri"
"$bin/tr" test tests >"$out/dil_tanimi.log" 2>&1
dil=$?
tail -2 "$out/dil_tanimi.log"

echo "== Hız: hedef model, sentetik veri, 120 adım"
python3 -c "import numpy as np, sys; np.save(sys.argv[1], np.random.default_rng(0).integers(1, 40000, size=40_000_000, dtype=np.uint16))" "$veri/train.npy"
nvidia-smi --query-gpu=memory.used --format=csv,noheader,nounits -lms 100 >"$out/bellek.log" &
yoklama=$!
"$bin/tr" tools/gpu/rgn512.tr --veri "$veri" --adım 120 --çıktı "$out/hiz.json" >"$out/hiz.log" 2>&1
hiz=$?
kill "$yoklama"

python3 - "$out" "$eszamanli" "$eszamansiz" "$dil" "$hiz" <<'PY'
import json, os, sys
out, codes = sys.argv[1], dict(zip(("gpu_eszamanli", "gpu_eszamansiz", "dil_tanimi", "hiz"), map(int, sys.argv[2:])))
speed = json.load(open(f"{out}/hiz.json")) if os.path.exists(f"{out}/hiz.json") else {}
mem = [int(l) for l in open(f"{out}/bellek.log") if l.strip().isdigit()]
m = {"kart": open(f"{out}/kart.txt").read().strip(), "çıkış": codes, "passed": not any(codes.values()),
     "ms_adım": (speed.get("measured") or {}).get("median_ms"), "tepe_bellek_mib": max(mem, default=None)}
json.dump(m, open(f"{out}/metrics.json", "w", encoding="utf-8"), ensure_ascii=False, indent=1)
print("sonuç:", "GEÇTİ" if m["passed"] else "KALDI", codes, "·", m["ms_adım"], "ms/adım ·", m["tepe_bellek_mib"], "MiB")
sys.exit(0 if m["passed"] else 1)
PY
