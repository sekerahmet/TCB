# Metin üretme, Gemma 3 12B Q4_K_M (ggml-org/gemma-3-12b-it-GGUF) (S1/gemma3-12b-q4km), dizüstü: boş önbellekte 512 token okuma, 512 derinlikten 128 adım üretim;
# TR (language_model.tr, nicemli etkinleştirme Dost'un kuralıyla) ile llama.cpp b12124 (llama-bench -ngl 0), ABBA 10 koşu.
# Uyuşma (B14): llama-completion'ın 32 token'ı TR'de zorlanmış okumayla, ε TR'nin işlemcide kendi ölçtüğü nicemli/nicemsiz farkı.
# Satır: docs/measurements/text-generation.md, "Dizüstü i7-13700H". Taraflar: ikisi işlemcide (llama.cpp'nin Windows paketi
# işlemci paketi, komutta -ngl 0). llama.cpp'nin yeri OLCU_LLAMA; ikilinin sha256'sı ve sürümü sources.json'da.
# Uyuşma denenmedi: tek kalemdir, dizüstünde 2 dakikaya sığmaz (2026-11-19); deneme kipinde yalnız model bağlama, sha256 ve
# makine kaydı. Ölçü penceresinde en sona konur; ilk koşusu uyuşmadır, uyuşmazsa durur, sıradaki kaleme geçilir (TCB 2026-11-20).
#   bash tools/bench/baseline/machines/laptop/generate-gemma3-12b-q4km.sh
. "$(dirname "$0")/lib.sh"
olcu_kur "llama_bench=$LLAMA/llama-bench.exe" "llama_completion=$LLAMA/llama-completion.exe"
olcu_model gemma3-12b-q4km
olcu_makine generate-gemma3-12b-q4km
olcu_arac llama.cpp
if [ "${OLCU_DENEME:-}" = 1 ]; then echo "deneme: uyuşma denenmedi (2 dakikaya sığmaz), pencerede ilk koşu"; exit 0; fi
olcu_kos generate-gemma3-12b-q4km S1 --key S1/gemma3-12b-q4km
