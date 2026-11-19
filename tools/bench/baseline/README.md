# Temel ölçü seti

TR'nin bugünkü hızını ve belleğini her makinede aynı programla, aynı yöntemle ve emsalleriyle yan yana ölçer (S1–S14). Ölçünün kendisi Colab'da ve dizüstünün ölçü penceresinde koşar; satırlar `docs/measurements/`'e yazılır. Hız ve bellek ölçüsü bu koşturucuyla yapılır: yeni bir ölçü buraya soru olarak eklenir (`questions.py` ve testi), ayrı ölçüm betiği yazılmaz.

## Dosyalar

| Dosya | Ne |
| :-- | :-- |
| `run.py` | Koşturucu: yöntem (sıra, hüküm, bellek, durma), uyuşma ve ölçü geçişleri, kayıt, `--rows`, `--fetch` |
| `questions.py` | Soru kaydı: S1–S14'ün komutları, uyuşma ölçütleri, ölçüleri; makinelerin soru listeleri; modellerin deposu, revizyonu ve dosyaları; sabit seslerin üretimi |
| `references.py` | Python emsalleri (transformers, faster-whisper, onnxruntime, torch); TR programlarıyla aynı iş, aynı alanlar |
| `prepare.sh` | Colab hazırlığı: derlemeler, eksik paketler (`--no-deps`), modeller, S11'in verisi; `phonemes` kipi kimlik dosyasını üretir |
| `texts.json` | Sabit metinler: S5'in iki paragrafı, S6–S7'nin üç cümlesi, S8'in on sorusu, dil modelinin metni |
| `english_phoneme_ids.json` | İngilizce Piper'ın fonem kimlikleri (`piper-phonemize` 1.1.0, `prepare.sh phonemes`); iki taraf aynı kimliklerle konuşur |
| `language_model.tr` | S1–S3: istem okuma, üretim, derinlikli okuma; `--folder` parçalı Hugging Face klasörü |
| `catalog_memory.tr` | S4: Dost'un kataloğunun bir girdisi, kataloğun bağlamında |
| `whisper.tr` | S5: yazıya çevirme |
| `mms.tr`, `piper.tr` | S6, S7: yazıdan sese, cümle başına |
| `matmul.tr` | S12: kare matris çarpımı, TR'nin çekirdeği ya da `TR_vendor_matmul` |
| `loop_100hz.tr` | S13: tensör işi yapan bir görev varken 100 Hz'lik döngünün kaçan turu |
| `empty.tr` | S14: boş program |

## Yöntem (koşmadan önce `run.py`'de yazılı)

- **Sıra:** her taraf bir ısınma, sonra ABBA: TR, E, E, TR, TR, E, E, TR, TR, E. Her koşu kendi sürecindedir. Emsalsiz soruda (S4, S8–S10, S14) bir ısınma ve beş koşu (S8–S10'da üç).
- **Hüküm:** i'nci TR koşusunun i'nci emsal koşusuna oranı; çeyrekler `statistics.quantiles(n=4, method="exclusive")`. Q1 ve Q3 1'in aynı yanındaysa hüküm o yöndür, değilse "ayrılmıyor"; biri 1'e %1'den yakınsa "sınırda". Bir koşu birden çok ölçü verebilir (S1'de okuma ve üretim); her ölçünün kendi oranları ve hükmü vardır.
- **İş parçacığı:** makine başına tek sayı, TR sürecinin `GODEBUG=gctrace=1` satırındaki `<n> P`; `os.cpu_count()` değil. Her emsale kendi adıyla verilir: llama.cpp ve whisper.cpp `-t`, `references.py --threads` (torch `set_num_threads`, faster-whisper `cpu_threads`, onnxruntime `intra_op_num_threads`, `inter_op_num_threads` 1). TR tarafında `GOMAXPROCS` verilmez.
- **Bellek:** Linux'ta `VmHWM` (sayaç, `ru_maxrss` ile karşılaştırılır) ve `RssAnon`'un 10 ms'lik örneklemesinin en büyüğü; Windows'ta `PeakWorkingSetSize` ve `PeakPagefileUsage`; GPU'da `nvidia-smi` 1 sn'de bir. Bir taraf birden çok süreçse (S1'de llama-bench'in iki koşusu) her alanın en büyüğü.
- **Uyuşma:** Colab makinelerinde ölçüden önce ayrı bir geçiş (`--agreement`) her sorunun uyuşmasını bir kez dener; GPU makinelerinde önce onnxruntime'ın CUDA sağlayıcısı, HIGHRAM'de kimlik dosyası (`piper.tr --check_ids`). Dizüstünde ayrı geçiş yoktur: ısınma koşusu uyuşma koşusudur. Uyuşmazlık, hata veren koşu ya da dizüstünde %20'yi aşan yabancı yük o soruda durdurur, çıkış kodu 3.
- **Kayıt:** `--out` klasörüne `log.jsonl` (her koşu, öncesi ve sonrası süreç listesi), `metrics.json` (anahtar başına ölçüler, oranlar, hüküm, bellek), `agreement.json`. `--skip-done` `metrics.json`'daki anahtarları atlar. `--rows` satır taslağını yazar; satırın her sayısı `metrics.json`'dan doğrulanır.

## Sorular

| Soru | TR | Emsal | Uyuşma | Ölçü |
| :-- | :-- | :-- | :-- | :-- |
| S1 | `language_model.tr --mode speed` (6 model; nicemli etkinleştirme Dost'un kuralıyla: `--quantized auto` işlemcide yalnız nicemlenmiş ağırlıklarının hepsi q4_k ya da q6_k olan modelde açık, Q8_0'da kapalı; QAT Q4_0 `--quantized off`) | `llama-bench -p 512 -n 0` ve `-p 0 -n 128 -d 512`, `-r 1 --no-warmup -o json` | zorlanmış okuma (B14): `llama-completion -n 33 --temp 0`'ın cevabı `--mode forced`'la TR'ye verilir (33 token, karşılaştırılan ilk 32: llama-completion sona boş satır basar, son token onunla birleşebilir); 32 konumun her biri ya uyuşur (llama.cpp'nin token'ı TR'nin en olası token'ı) ya başa baştır (TR'nin en iyi logiti o token'ınkinden ε'dan az büyük). ε TR'nin nicemli ve nicemsiz yolunun aynı konumlardaki logitlerinin en büyük mutlak farkıdır (nicemli yolu kapalı modelde, Q8_0'da, ε 0: iki taraf aynı ağırlıkla, uyuşma kesin); kayda uyuşan sayısı, başa baş konumlar ve farkları, ε yazılır | `prompt_ms`, `gen_ms` |
| S2 | `language_model.tr --folder` bf16 | `references.py lm` (transformers) | açgözlü token'lar aynı | `prompt_ms`, `gen_ms`, `load_ms` |
| S3 | `language_model.tr --mode depth` (700, 4 100; 40 token) | `llama-bench -p 40 -n 0 -d 700` ve `-d 4100` | S1'inki, kendi uyuşma koşusuyla | `depth700_ms`, `depth4100_ms` |
| S4 | `catalog_memory.tr` (her girdi, her ölçülmüş bağlam) | yok | — | `load_ms`, bellek |
| S5 | `whisper.tr` (tr, en) | faster-whisper float32 ve int8, transformers, whisper.cpp f16 | emsallerin yayılımı (B15): TR'nin her float32 emsale (faster-whisper float32, transformers) sözcük uzaklığı, öbür float32 emsalin ona uzaklığını aşmaz (küçük harf, noktalamasız). int8 ve whisper.cpp f16 sınır kurmaz; kayda bütün uzaklıklar ve kaynağa göre WER'ler yazılır | `transcribe_ms` |
| S6 | `mms.tr` (tr, en; gürültü 0) | `references.py mms` (float32; kimlikler VITS'in boşluk kuralıyla, `mms_ids`: mms-tts-eng'in dolgu belirteci `k` olduğu için `VitsTokenizer` sondaki `k`'nin boşluğunu düşürür) | cümle başına örnek sayısı ve örnekleme hızı aynı; sentez başına iki rastgele tensör | `total_ms`, `first_ms` |
| S7 | `piper.tr --ids` (tr: TR'nin çeviricisinin kimlikleri, `--write_ids`; en: `english_phoneme_ids.json`) | `references.py piper` (onnxruntime) | S6'daki gibi | `total_ms`, `first_ms` |
| S8 | `tools/assistant/asistan.tr --kip canlı --sahte_mikrofon` (O-7'nin 10 soruluk sesi, sessiz çalıcı) | yok | — | `OLCU` satırlarından ilk ses (ortanca, en kötü), cevap, yazı, ilk token |
| S9 | Dost'un ekranlı kipi, `send_text` ile S8'in soruları | yok | — | `metrics` olayının `first_token_ms`, `tokens_per_second` |
| S10 | Dost'un ekranlı kipi; Colab'da soğuk ve sıcak, dizüstünde sıcak. Soğuk: ekranın açtığı dosyalar (trlang, modeller, katalog, `lib/` ve `tools/assistant/`'ın TR dosyaları) `sync` ve `posix_fadvise(DONTNEED)` ile önbellekten atılır, sistemin önbelleği değil (Colab'da `/proc/sys` salt okunur); önce ve sonra `fincore`'la önbellekteki baytlar kayda, onda birin altına düşmediyse durur | yok | — | açılıştan `state` `idle`'a |
| S11 | `tools/gpu/rgn512.tr` bf16 ve fp32 (GPU) | `tools/gpu/rgn512.py` derlemesiz ve derlemeli (yalnız bf16: float32 kipi yok) | — | ms/adım |
| S12 | `matmul.tr` float32 ve bf16, iki çekirdek (GPU) | `references.py matmul` (torch) | birlerle köşe öğesi aynı | `median_ms` |
| S13 | `tools/bench/tr/{dongu,fib,matris_dongu}.tr`; karakter modeli (300 ve 0 adımın farkı); `loop_100hz.tr` | `tools/bench/go/compare.go`; `tools/bench/py/char_model.py` | sonuç değeri aynı | ms; ms/adım; kaçan tur |
| S14 | `empty.tr` | yok | — | duvar saati |

Makinelerin listeleri `questions.py`'nin `QUESTIONS`'ındadır: HIGHRAM ve dizüstü S1–S10, S13, S14; L4 (GPU ve işlemci aynı oturumda: `l4gpu`, `l4cpu`) ve G4 GPU'da S1–S3, S5–S12, S14 ve S13'ün döngüsü; L4'ün işlemcisi S1–S3, S5–S10, S14.

## Colab'da

```bash
bash tools/bench/baseline/prepare.sh /content/w cpu      # GPU makinesinde: gpu
python3 -I tools/bench/baseline/run.py --work /content/w --machine highram --out "$TR_RECORD_DIR" --agreement
python3 -I tools/bench/baseline/run.py --work /content/w --machine highram --out "$TR_RECORD_DIR" --skip-done
python3 -I tools/bench/baseline/run.py --rows --out "$TR_RECORD_DIR"
```

Koşu `go run ./tools/record --name h1-baseline-<makine>-<bölüm> --out <Drive>/TR_dili/olcumler -- python3 -I tools/bench/baseline/run.py …` ile sarılır. Sabit sesler (O-7) bir kez, HIGHRAM'de `--make-sounds` ile üretilir: TR'nin sesi, sonra ffmpeg'le 16 kHz tek kanal 16 bit (Whisper'ın hızı; Piper 22 050 Hz konuşur); sha256'larıyla (`sounds/sounds.json`) her makineye taşınır; sesler yoksa ya da tutmazsa S5 ve S8 sona kalır.

## `cpu_baseline.sh` ile ilişkisi

[`../cpu_baseline.sh`](../cpu_baseline.sh) karakter modelini ve çekirdekleri 1, 2, 4, 8 iş parçacığıyla ölçer: iş parçacığına göre ölçeklenmeyi gösterir. Bu set makine başına tek sayıyla, emsaliyle aynı koşulda ölçer: TR'nin o makinedeki yerini gösterir.

## Testler

`tools/bench/behavior_baseline_test.go` ve `behavior_baseline_programs_test.go` yöntemin her kuralını ve her programı küçük girdilerle sınar: sıra, hüküm, bellek okuyucuları, uyuşmazlıkta durma, metinler ve kimlik dosyası, dil modeli (`tests/data/hf_tiny`), Whisper (`tests/whisper_model_test.tr`'nin küçük çeviri modeli, özgün `transcribe`'ın metni), sesler (`tests/data/vits_tiny`, `vits_piper_tiny`), katalog, matris, döngü, boş program, iş parçacığı kuralı, soru kaydı ve Dost'un okuyucuları. `behavior_baseline_colab_test.go` B–D'nin Colab ölçülerinde bulunan düzeltmeleri sınar: `--quantized auto`'nun ağırlık kuralı (küçük Q8_0 GGUF'la), Dost'un seslerinin sondaki sessizliği ve tur kaydı, S10'un soğuk kipi, faster-whisper'ın CUDA 12 kitaplıklarının yalnız onun sürecine gitmesi. Gerçek modellerin yolu (GGUF, gerçek Piper sesi, Dost'un sunucusu, GPU) Colab'da koşar.
