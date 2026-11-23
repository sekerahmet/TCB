# Emsallerin en hızlı ayarı (Ş1, DENETİM 944 ve 947; TCB 2026-11-22), dizüstü. Kural koşudan önce burada yazılı: her emsalin
# iş parçacığı sayısı üç değerle denenir (kütüphanenin varsayılanı 0, 6, makinenin mantıksal sayısı). S1'in llama-bench
# okuması (-p 512 -n 0) ve üretimi (-p 0 -n 128 -d 512) ayrı gruptur, her biri kendi ayarını alır (N1); S3'ün llama-bench'i
# okumanın ayarıyla ve ayrıca flash attention 0 ve 1 ile, yalnız 700 derinliğinde. Her ayar grubun temsil kalemiyle bir kez
# ısınır, sonra 3 koşu ayarlar arasında döner (0, 6, N, 0, 6, N, …); öncelik 0, 6, N'dir ve sonraki ayar ancak bütün
# koşuları (ölçülerin toplamı) o ana kadar seçilenin bütün koşularından kısaysa seçilir (H1e K6, DENETİM 960 Ş1); tablo her
# koşuyu, sırayı ve ölçü başına ortancayı da yazar. Gruplar ve kalemleri: llama.cpp read ve gen (S1/qwen3-4b-q4km),
# llama.cpp S3 (S3/qwen3-4b-q4km), faster-whisper float32 ve int8 (S5/tr), whisper.cpp (S5/tr), onnxruntime (S7/tr).
# Seçim ve tablo kayıt kökündeki en-hizli.json'a, her grup bitince kimliğiyle (makine ve emsal; H1e K2); taranmayan
# grupların girdisi korunur, dosya silinmez (K5). Bir grup makine ya da emsal değişene kadar geçerlidir; ölçü betikleri
# eksik grubu bu betiği o grupla çağırarak kendileri tarar (K4), bu yüzden betik elle yalnız yenilemek için koşar:
# argümansız bütün grupları, argümanla verilen grupları. Süre ~10 dk [tahmin]. Deneme kipinde her ayar bir kez, ısınmasız,
# kendi klasörüne (run-en-hizli-ayar-deneme); 2 dakikaya sığması için gruplar argümanla tek tek verilebilir.
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
  # Tarama yalnız emsalleri koşar: TR'nin ve transformers'ın whisper-small'u bağlanmaz (BC28).
  for m in faster-whisper-small ggml-small; do olcu_model "$m"; done
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
