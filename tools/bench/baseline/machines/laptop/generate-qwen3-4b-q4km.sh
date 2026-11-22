# Metin üretme, Qwen3-4B Q4_K_M (Qwen/Qwen3-4B-GGUF) (S1/qwen3-4b-q4km), dizüstü: boş önbellekte 512 token okuma, 512 derinlikten 128 adım üretim;
# TR (language_model.tr, nicemli etkinleştirme Dost'un kuralıyla) ile llama.cpp b12124 (llama-bench -ngl 0), ABBA 10 koşu.
# Uyuşma (B14): llama-completion'ın 32 token'ı TR'de zorlanmış okumayla, ε TR'nin işlemcide kendi ölçtüğü nicemli/nicemsiz farkı.
# Satır: docs/measurements/text-generation.md, "Dizüstü i7-13700H". Taraflar: ikisi işlemcide (llama.cpp'nin Windows paketi
# işlemci paketi, komutta -ngl 0). llama.cpp'nin yeri OLCU_LLAMA; ikilinin sha256'sı ve sürümü sources.json'da.
#   bash tools/bench/baseline/machines/laptop/generate-qwen3-4b-q4km.sh
. "$(dirname "$0")/lib.sh"
olcu_kur "llama_bench=$LLAMA/llama-bench.exe" "llama_completion=$LLAMA/llama-completion.exe"
olcu_model qwen3-4b-q4km
olcu_makine generate-qwen3-4b-q4km
olcu_arac llama.cpp
olcu_kos generate-qwen3-4b-q4km S1 --key S1/qwen3-4b-q4km
