# Metin üretme bf16, Qwen3-4B (S2/qwen3-4b-bf16), dizüstü: language_model.tr --folder --dtype bf16, okuma, üretim ve yükleme
# süresi; TR, 1 ısınma + 5 koşu. Emsal yok (en hızlısı aranıyor: llama.cpp BF16; TCB 2026-11-20: transformers hız emsali
# değil); transformers (bfloat16) yalnız uyuşmada: açgözlü token'lar aynı. Satır: docs/measurements/text-generation.md,
# "Dizüstü i7-13700H". İşlemcide; model Qwen/Qwen3-4B (yeri ve kimliği sources.json'da; ~8 GB), %USERPROFILE%\models\
# qwen3-4b-bf16\. Deneme kipinde uyuşma ve TR bir kez başlar ama 2 dakikaya sığmaz (2026-11-22: kurulum
# 49 sn, TR'nin bf16 uyuşma koşusu 64 sn'de bitmedi): uyuşma denenmedi (2 dk), pencerede ilk koşu; uyuşmazsa durur, sıradaki
# kaleme geçilir (TCB 2026-11-22 13:27). llama.cpp (OLCU_LLAMA) yalnız S1–S3'ün kalemleri birlikte
# kurulduğu için tools.json'da; S2'de koşmaz.
#   bash tools/bench/baseline/machines/laptop/generate-bf16-qwen3-4b.sh
. "$(dirname "$0")/lib.sh"
olcu_kur "llama_bench=$LLAMA/llama-bench.exe" "llama_completion=$LLAMA/llama-completion.exe"
olcu_model qwen3-4b-bf16
olcu_makine generate-bf16-qwen3-4b
olcu_surum torch transformers
olcu_kos generate-bf16-qwen3-4b S2 --key S2/qwen3-4b-bf16
