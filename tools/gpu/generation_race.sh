#!/usr/bin/env bash
# Üretim hızı yarışı: TR, transformers (eager ve static + compile) ve llama.cpp (f32, f16);
# SmolLM2-135M ve Qwen3-0.6B; GPU ve CPU. Sonuçlar SONUC dosyasına "model | cihaz | yol | parça/sn"
# satırları olarak yazılır (COLAB.md §12; yol haritası Evre 4 "Üretim hızı" tablosu).
# Kullanım: generation_race.sh TR_IKILISI CALISMA_KLASORU
# Gerekenler: Python'da torch, transformers, huggingface_hub; CUDA araç takımı (llama.cpp için).
set -u
TR=$1
W=$2
HERE=$(cd "$(dirname "$0")" && pwd)
mkdir -p "$W" && cd "$W"
SONUC=$W/sonuc.txt
echo "işlemci: $(lscpu | grep 'Model name' | sed 's/.*: *//') · $(nproc) iş parçacığı · torch $(python -c 'import torch;print(torch.__version__)') · transformers $(python -c 'import transformers;print(transformers.__version__)')" >> "$SONUC"
python -c "
from huggingface_hub import snapshot_download
for r, d in [('HuggingFaceTB/SmolLM2-135M', 'smol'), ('Qwen/Qwen3-0.6B', 'qwen')]:
    snapshot_download(r, local_dir='$W/' + d, allow_patterns=['*.json', '*.safetensors', '*.txt', '*.model'])
" > indir.log 2>&1
[ -d llama.cpp ] || git clone -q --depth 1 https://github.com/ggml-org/llama.cpp
(cd llama.cpp && cmake -B build -DGGML_CUDA=ON -DLLAMA_CURL=OFF > ../cmake.log 2>&1 && cmake --build build -j"$(nproc)" --target llama-bench > ../derle.log 2>&1)
echo "llama.cpp: $(cd llama.cpp && git log -1 --format='%h %cd' --date=short)" >> "$SONUC"
pip install -q -e llama.cpp/gguf-py > pip.log 2>&1
for d in smol qwen; do for t in f32 f16; do
  [ -f $d-$t.gguf ] || python llama.cpp/convert_hf_to_gguf.py $d --outtype $t --outfile $d-$t.gguf > conv_${d}_$t.log 2>&1
done; done
for cihaz in gpu cpu; do
  hf=$([ $cihaz = gpu ] && echo cuda || echo cpu)
  ngl=$([ $cihaz = gpu ] && echo 99 || echo 0)
  for d in smol qwen; do
    ad=$([ $d = smol ] && echo SmolLM2-135M || echo Qwen3-0.6B)
    for t in float32 bf16; do
      "$TR" "$HERE/generation_race.tr" --klasör "$W/$d" --cihaz $cihaz --ad $ad --ağırlık_türü $t >> "$SONUC" 2>&1
    done
    python "$HERE/generation_race_hf.py" "$W/$d" $ad $hf eager >> "$SONUC" 2>> hf.log
    [ $cihaz = gpu ] && python "$HERE/generation_race_hf.py" "$W/$d" $ad $hf static >> "$SONUC" 2>> hf.log
    for t in f32 f16; do
      llama.cpp/build/bin/llama-bench -m $d-$t.gguf -p 0 -n 100 -r 3 -ngl $ngl -t "$(nproc)" -o csv 2>/dev/null | tail -1 |
        awk -F, -v a=$ad -v c=$cihaz -v t=$t '{print a " | " c " | llama.cpp " t " | " $(NF-1)}' >> "$SONUC"
    done
  done
done
echo bitti > "$W/bitti"
