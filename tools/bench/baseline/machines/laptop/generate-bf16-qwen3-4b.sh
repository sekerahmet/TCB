# Metin üretme bf16, Qwen3-4B (S2/qwen3-4b-bf16), dizüstü: language_model.tr --folder --dtype bf16, okuma, üretim ve yükleme
# süresi; TR, 1 ısınma + 5 koşu. Emsal yok (en hızlısı aranıyor: llama.cpp BF16; TCB 2026-11-20: transformers hız emsali
# değil); transformers (bfloat16) yalnız uyuşmada: açgözlü token'lar aynı. Satır: docs/measurements/text-generation.md,
# "Dizüstü i7-13700H". İşlemcide; model Qwen/Qwen3-4B (sha256 Hugging Face üstverisinden; ~8 GB), %USERPROFILE%\models\
# qwen3-4b-bf16\. Deneme kipinde uyuşma ve TR bir kez. llama.cpp'nin yeri (OLCU_LLAMA) yalnız S1–S3'ün kalemleri birlikte
# kurulduğu için tools.json'da; S2'de koşmaz.
#   bash tools/bench/baseline/machines/laptop/generate-bf16-qwen3-4b.sh
. "$(dirname "$0")/lib.sh"
LLAMA=${OLCU_LLAMA:-$USERPROFILE/tools/llama.cpp/b12124}
olcu_kur "llama_bench=$LLAMA/llama-bench.exe" "llama_completion=$LLAMA/llama-completion.exe"
olcu_model qwen3-4b-bf16
olcu_makine generate-bf16-qwen3-4b
olcu_surum torch transformers
olcu_kos generate-bf16-qwen3-4b S2 --key S2/qwen3-4b-bf16
