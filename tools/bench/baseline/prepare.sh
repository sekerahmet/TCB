#!/usr/bin/env bash
# Temel ölçü setinin Colab hazırlığı (tools/bench/baseline/README.md, tasarım O-6). Dizüstünde aynı adımlar elle yapılır.
#   bash tools/bench/baseline/prepare.sh WORK cpu|gpu       iş klasörü: bin/, src/, models/, tools.json, pip_list.txt
#   bash tools/bench/baseline/prepare.sh WORK phonemes      A-11, HIGHRAM, bir kez: english_phoneme_ids.json'u üretir
# İnenler tasarımın "İndirmeler"inde adı, yayımcısı, revizyonu ve boyuyla yazılıdır (Ahmet 11:44 ve 12:16, yalnız Colab).
# Python paketleri `pip install --no-deps` ile, yalnız makinede kurulu olmayanlar iner; kurulu olanlar pip_list.txt'te.
set -euo pipefail
WORK=${1:?iş klasörü}
KIND=${2:?cpu, gpu ya da phonemes}
ROOT=$(cd "$(dirname "$0")/../../.." && pwd)
mkdir -p "$WORK/bin" "$WORK/src"
say() { printf '== %s %s\n' "$(date '+%H:%M:%S %z')" "$*"; }

if [ "$KIND" = phonemes ]; then
  # A-4: piper-phonemize 1.1.0'ın tekerleği yalnız cp39–cp311 için var; uv (PyPI, astral-sh) Python 3.11.18'i kurar.
  say "uv 0.13.9 ve Python 3.11.18"
  python3 -m pip install --no-deps "uv==0.13.9"
  python3 -m uv python install 3.11.18 -v 2>&1 | tee "$WORK/uv_python.log" | grep -iE "download|https://" || true
  python3 -m uv venv --python 3.11.18 "$WORK/py311"
  python3 -m uv pip install --python "$WORK/py311/bin/python" --no-deps "piper-phonemize==1.1.0"
  VOICE="$WORK/models/piper-en-lessac-medium/en/en_US/lessac/medium/en_US-lessac-medium.onnx.json"
  "$WORK/py311/bin/python" -I - "$ROOT/tools/bench/baseline/texts.json" "$VOICE" "$ROOT/tools/bench/baseline/english_phoneme_ids.json" <<'PY'
import json, sys
import piper_phonemize
texts, voice_path, out = sys.argv[1:]
with open(texts, encoding="utf-8") as f:
    sentences = json.load(f)["text_to_speech"]["en"]
with open(voice_path, encoding="utf-8") as f:
    voice = json.load(f)
espeak, table = voice["espeak"]["voice"], voice["phoneme_id_map"]
rows = []
for text in sentences:
    pieces = piper_phonemize.phonemize_espeak(text, espeak)
    if len(pieces) != 1:
        raise SystemExit(f"{text!r}: {len(pieces)} phoneme pieces, want one sentence")
    phonemes = "".join(pieces[0])
    # Piper's ids (piper-tts phoneme_ids.py; lib/vits.tr _kimlikler): ^, _, each phoneme then _, $
    ids = table["^"] + table["_"]
    for p in pieces[0]:
        ids = ids + table[p] + table["_"]
    rows.append({"text": text, "phonemes": phonemes, "ids": ids + table["$"]})
about = {"made_by": "tools/bench/baseline/prepare.sh phonemes", "piper_phonemize": "1.1.0", "espeak_voice": espeak,
         "voice": "rhasspy/piper-voices en_US-lessac-medium", "voice_revision": "e9ab671f2a0e50a701b34361136ba3e6a3a9eb0c",
         "python": sys.version.split()[0]}
with open(out, "w", encoding="utf-8", newline="") as f:
    json.dump({**about, "sentences": rows}, f, ensure_ascii=False, indent=1)
    f.write("\n")
print(json.dumps(about))
PY
  exit 0
fi

say "trlang ve Go karşılaştırma programı (bu commit'ten)"
(cd "$ROOT" && go build -o "$WORK/bin/trlang" ./cmd/trlang && go build -o "$WORK/bin/compare" ./tools/bench/go)
git -C "$ROOT" rev-parse HEAD > "$WORK/commit.txt"

CUDA=OFF
[ "$KIND" = gpu ] && CUDA=ON
build() { # ad etiket adres hedefler...
  local name=$1 tag=$2 url=$3; shift 3
  [ -d "$WORK/src/$name" ] || git clone --depth 1 --branch "$tag" "$url" "$WORK/src/$name"
  du -sh "$WORK/src/$name" # kaynağın boyu (İndirmeler: "boy Colab'da ölçülür")
  cmake -S "$WORK/src/$name" -B "$WORK/src/$name/build" -DCMAKE_BUILD_TYPE=Release -DGGML_NATIVE=ON -DGGML_CUDA=$CUDA > "$WORK/src/$name.cmake.log"
  cmake --build "$WORK/src/$name/build" -j"$(nproc)" --target "$@" > "$WORK/src/$name.build.log"
  grep -E "^GGML_(NATIVE|CUDA)[:=]" "$WORK/src/$name/build/CMakeCache.txt" # seçeneklerin adı ve değeri (A-11)
}
say "llama.cpp b12124 ve whisper.cpp v1.9.5 (GGML_NATIVE, CUDA $CUDA)"
build llama.cpp b12124 https://github.com/ggml-org/llama.cpp llama-bench llama-completion
build whisper.cpp v1.9.5 https://github.com/ggml-org/whisper.cpp whisper-cli

say "Python paketleri: yalnız kurulu olmayanlar, --no-deps"
ORT=onnxruntime
[ "$KIND" = gpu ] && ORT=onnxruntime-gpu
for spec in faster-whisper==1.2.1 ctranslate2==4.8.5 $ORT==1.31.1 av==19.0.2 numpy==2.5.5 hf-xet==1.8.0 tokenizers==0.23.3 \
    huggingface-hub==2.4.0 pyyaml==6.0.3 protobuf==7.36.2 filelock==4.2.12 fsspec==2026.9.0 tqdm==4.70.1 packaging==26.3 \
    click==8.5.1 httpx2==2.16.2 httpcore2==2.16.2 anyio==4.16.0 idna==3.20 flatbuffers==25.12.19 typing-extensions==4.16.0 \
    truststore==0.10.5 h11==0.16.0; do
  name=${spec%%==*}
  if python3 -m pip show -q "$name" > /dev/null 2>&1; then
    echo "kurulu: $name $(python3 -m pip show "$name" | sed -n 's/^Version: //p')"
  else
    python3 -m pip install -q --no-deps "$spec"
    echo "indi: $spec"
  fi
done
python3 -m pip list > "$WORK/pip_list.txt"

say "modeller (revizyonuyla; sha256 hub'ınkiyle)"
python3 -I "$ROOT/tools/bench/baseline/run.py" --work "$WORK" --fetch

if [ "$KIND" = gpu ]; then
  say "S11'in sentetik verisi (tools/gpu/COLAB.md)"
  mkdir -p "$WORK/rgn"
  [ -f "$WORK/rgn/train.npy" ] || python3 -I -c "
import numpy as np
np.save('$WORK/rgn/train.npy', np.random.default_rng(0).integers(1, 40000, size=40_000_000, dtype=np.uint16))"
fi

B=$WORK/src
cat > "$WORK/tools.json" <<EOF
{"trlang": "$WORK/bin/trlang", "go_compare": "$WORK/bin/compare",
 "llama_bench": "$B/llama.cpp/build/bin/llama-bench", "llama_completion": "$B/llama.cpp/build/bin/llama-completion",
 "whisper_cli": "$B/whisper.cpp/build/bin/whisper-cli"}
EOF
say "hazır: $WORK"
