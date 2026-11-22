# Emsallerin en hızlı ayarı (Ş1, DENETİM 944 ve 947; TCB 2026-11-22), dizüstü: ölçü penceresinin İLK betiği. Kural koşudan
# önce burada yazılı: her emsalin iş parçacığı sayısı üç değerle denenir (kütüphanenin varsayılanı 0, 6, makinenin mantıksal
# sayısı). S1'in llama-bench okuması (-p 512 -n 0) ve üretimi (-p 0 -n 128 -d 512) ayrı gruptur, her biri kendi ayarını alır
# (N1); S3'ün llama-bench'i okumanın ayarıyla ve ayrıca flash attention 0 ve 1 ile, yalnız 700 derinliğinde. Her ayar grubun
# temsil kalemiyle bir ısınma + 3 koşu koşar; ölçülerin toplamının ortancası en kısa olan seçilir, tablo ölçü başına ortancayı
# da yazar. Gruplar ve kalemleri: llama.cpp read ve gen (S1/qwen3-4b-q4km), llama.cpp S3 (S3/qwen3-4b-q4km), faster-whisper
# float32 ve int8 (S5/tr), whisper.cpp (S5/tr), onnxruntime (S7/tr). Seçim ve tablo kayıt kökündeki en-hizli.json'a; öbür
# betikler ölçüyü o ayarla koşar (lib.sh olcu_kos --settings) ve dosyayı kendi kaydına koyar. Ölçü kipinde tarama boş dosyayla
# başlar, bütün grupları koşar, dosyayı yalnız hepsi bitince yazar; ölçü yalnız bütün grupları 3 koşuyla taranmış dosyayı alır
# (DENETİM 947 Ş1). Süre ~10 dk [tahmin]. Deneme kipinde her ayar bir kez, ısınmasız; 2 dakikaya sığması için gruplar
# argümanla tek tek verilebilir.
#   bash tools/bench/baseline/machines/laptop/en-hizli-ayar.sh [grup...]
. "$(dirname "$0")/lib.sh"
[ $# -gt 0 ] || set -- "${TARA_GRUPLARI[@]}"
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
