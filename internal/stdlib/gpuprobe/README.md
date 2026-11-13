# gpuprobe

GPU'da dil düzeyinde eğitimin sınamaları: TR ile yazılmış eğitim programları, kitaplığın GPT'si
ve kontrol noktası GPU'da koşturulur.

## Standartlar

**Kartsız makinede de koşar.** Gerçek GPU yoksa sınamalar PTX benzeticisinde çalışır; CI her
gönderimde bu yolu sınar.
Denetim: `TestGPUTrainingLanguageLevel`, `TestGPUCheckpoint`, `TestGPULibraryGPT`, `TestGPUInPlaceCompoundAssign` (her biri `gpu.RegisterSimulator()` ile başlar; CI'nin her gönderimde koşması bir iş akışı ayarıdır, Go testiyle sınanamaz).

**CPU ile aynı sonuç.** GPU'daki eğitim, aynı programın CPU'daki sonucuyla (16 bitlik türlerde
tanımlı bir toleransla) karşılaştırılır; kontrol noktasından sürdürülen GPU eğitimi kesintisiz
eğitimle aynıdır.
Denetim: `TestGPUTrainingLanguageLevel`, `TestGPULibraryGPT`, `TestGPUCheckpoint`, `TestGPUInPlaceCompoundAssign`.

## Testler

Birim testleri sınadıkları dosyanın adını taşır (`x_test.go` → `x.go`). Birden çok dosyayı kesen davranış testleri:

- `behavior_gpu_checkpoint_test.go`: GPU'da kontrol noktası kaydı ve devamı
- `behavior_gpu_device_info_test.go`: `TR_fn_device_info`'nun kartın adını, belleğini ve sürücü sürümünü vermesi (gerçek kartta G4'te; kartsız makinede benzetimin sabit değerleri)
- `behavior_gpu_generate_test.go`: kitaplığın üretiminin (kimlik ve konum `TR_Int`, KV önbelleğinin `TR_Bool` maskesi, `TR_replay` çizgesi) GPU'da CPU ile aynı parçaları vermesi
- `behavior_gpu_inplace_test.go`: yerinde bileşik atamanın, türevin yazma kurallarının, −0.0 sırasının (`TR_min`, eksensiz `TR_argmax` dahil) ve bf16 tensörün `.safetensors`'a yazılmasının GPU'da CPU ile aynı olması
- `behavior_gpu_library_test.go`: kitaplıktaki GPT ve iyileştiricilerin GPU'da CPU ile aynı kaybı vermesi
- `behavior_gpu_sixteen_bit_test.go`: kartta yüklenen 16 bitlik tensörün bütün desenlerinin dosyaya ve kontrol noktasına bitleriyle yazılması, sürdürmede karta geri gelmesi
- `behavior_gpu_training_test.go`: dil düzeyinde GPU eğitiminin CPU ile aynı sonucu vermesi
