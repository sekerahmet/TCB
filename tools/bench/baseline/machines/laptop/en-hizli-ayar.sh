# Emsallerin en hızlı ayarı (Ş1, DENETİM 944; TCB 2026-11-22 13:37), dizüstü: ölçü penceresinin İLK betiği. Kural koşudan
# önce burada yazılı: her emsalin iş parçacığı sayısı üç değerle denenir (kütüphanenin varsayılanı 0, 6, makinenin mantıksal
# sayısı); S3'ün llama-bench'i ayrıca flash attention 0 ve 1 ile, seçilen iş parçacığında, yalnız 700 derinliğinde. Her ayar
# grubun temsil kalemiyle bir ısınma + 3 koşu koşar; ölçülerin toplamının ortancası en kısa olan seçilir. Gruplar ve kalemleri:
# llama.cpp (S1/qwen3-4b-q4km), faster-whisper float32 ve int8 (S5/tr), whisper.cpp (S5/tr), onnxruntime (S7/tr),
# llama.cpp S3 (S3/qwen3-4b-q4km). Seçim ve tablo kayıt kökündeki en-hizli.json'a; öbür betikler ölçüyü o ayarla koşar
# (lib.sh olcu_kos --settings) ve dosyayı kendi kaydına koyar. Süre ~10 dk [tahmin]. Deneme kipinde her ayar bir kez,
# ısınmasız; 2 dakikaya sığması için gruplar argümanla tek tek verilebilir.
#   bash tools/bench/baseline/machines/laptop/en-hizli-ayar.sh [grup...]
. "$(dirname "$0")/lib.sh"
[ $# -gt 0 ] || set -- "llama.cpp" "llama.cpp S3" "faster-whisper float32" "faster-whisper int8" "whisper.cpp" "onnxruntime"
olcu_kur "llama_bench=$LLAMA/llama-bench.exe" "llama_completion=$LLAMA/llama-completion.exe" "whisper_cli=$WHISPER/whisper-cli.exe"
gruplar=" $* "  # yalnız istenen grupların modelleri ve araçları (deneme 2 dakikaya sığsın)
if [[ $gruplar == *" llama.cpp"* ]]; then
  olcu_model qwen3-4b-q4km
  olcu_arac llama.cpp
fi
if [[ $gruplar == *"whisper"* ]]; then
  for m in whisper-small faster-whisper-small ggml-small; do olcu_model "$m"; done
  olcu_ses "stt_tr.wav=61eecd21db79818e022e32145301a92ebf4e82e3b4ce3b0c8c97035c6cdd176e"
  olcu_arac whisper.cpp
  olcu_surum faster-whisper ctranslate2
fi
if [[ $gruplar == *" onnxruntime "* ]]; then
  olcu_model piper-tr-dfki-medium
  olcu_surum onnxruntime numpy
fi
olcu_makine en-hizli-ayar
olcu_tara "$@"
