# Uzun geçmişin üstüne okuma, Qwen3-4B Q4_K_M (S3/qwen3-4b-q4km), dizüstü: korunan önbellek 700 ve 4 100 derinlikte 40 token
# okuma; TR (language_model.tr --mode depth) ile llama.cpp b12124 (llama-bench -p 40 -n 0 -d 700 ve -d 4100, -ngl 0), ABBA 10
# koşu; uyuşma S1'inki (B14). Satır: docs/measurements/text-generation.md, "Dizüstü i7-13700H". Taraflar: ikisi işlemcide.
# Not: Colab'da HIGHRAM'in satırı şüpheli işaretli (llama.cpp'nin flash attention'ı ve KV türü kayıtta yok, TCB 22:45); bu
# betik llama-bench'in sürüm satırını kayda yazar, satır llama-bench'in json çıktısındaki ayarları anar.
#   bash tools/bench/baseline/machines/laptop/read-long-prompt-qwen3-4b-q4km.sh
. "$(dirname "$0")/lib.sh"
olcu_kur "llama_bench=$LLAMA/llama-bench.exe" "llama_completion=$LLAMA/llama-completion.exe"
olcu_model qwen3-4b-q4km
olcu_makine read-long-prompt-qwen3-4b-q4km
olcu_arac llama.cpp
olcu_kos read-long-prompt-qwen3-4b-q4km S3 --key S3/qwen3-4b-q4km
