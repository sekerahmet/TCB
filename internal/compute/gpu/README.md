# gpu

NVIDIA GPU arka ucu: kernelleri PTX olarak kendisi üretir, sürücüyü cgo'suz yükler. Bellek
havuzu, matris çarpımı kitaplığı (varsa) ve profil burada. GPU olmayan makinede kerneller bir
PTX benzeticisinde çalıştırılarak sınanır.

## Standartlar

**CPU ile aynı sonuç.** Her GPU işlemi CPU arka ucunun sonucuyla karşılaştırılarak sınanır
(16 bitlik türlerde tanımlı bir toleransla). Denetim: `behavior_nn_test.go`, `behavior_half_precision_test.go`, `behavior_grad_parity_test.go`.

**Toplamlar terim sayısıyla bozulmaz.** Bir iş parçacığının sıralı toplamı telafilidir
(Neumaier; `kernel_reduce.go` `compensatedSum`): düz float32 toplamın hatası terim sayısıyla
büyür, CPU ise float64'te toplar. float64 birikim seçilmedi: hedef kartlarda FP32:FP64 64:1'dir
ve ölçümde sütun toplamları 2,3–3,9 kat yavaşladı (telafili toplam %5–11). Sonuç CPU'ya değil,
float64 referansa karşı sınanır. Denetim: `TestSumsMatchFloat64Reference`,
`TestSumsKeepSpecialValues`, `TestNormsMatchFloat64Reference`.

**Özel değerlerde CPU ile bit bit aynı.** ±0, ±∞ ve NaN girdide her işlem (ileri ve türev) CPU
ile aynı sonucu verir: NaN'lar NaN, sonsuzlar ve sıfırlar aynı işaretli (türevlerde sıfırın işareti
önemsiz); davranış dil tanımı 15.50'nin tablosudur. f(±0) = ±0 olan tekli işlemler sıfırda girdiyi
verir (`kernel_elementwise.go` `keepsZero`); gerçek toplamlar sonda +0 ekler, kopyalayan
indirgemeler −0'ı korur (`reduceConfig.plusZero`). Denetim: `TestSpecialValuesSameOnBothDevices`,
`TestSpecialValuesTable`.

**GPU yoksa zarif.** Sürücü ya da kart yoksa program çökmez; GPU görünmez olur ve nedeni
sorulabilir. Denetim: `TestNoDriverGraceful`.

**Benzetici gerçeğe sadıktır.** Benzetici, gerçek GPU'nun reddedeceği kodu (sınır dışı erişim,
yarış, engel hatası) da reddeder; böylece GPU'suz makinede yakalanan hata gerçek kartta da
hatadır. Denetim: `TestSimCatchesRace`, `TestSimDivergentBarrier`, `TestSimOutOfBounds`.

**Her donanım komutu benzeticide de yürür.** Çekirdeklere eklenen her PTX komutu (warp
düzeyindeki tensör çekirdeği ve değiş tokuş komutları dahil) benzeticiye de eklenir. Komutun veri
düzenini sınayan test hem benzeticide hem gerçek kartta koşar; ikisinin sonucu aynı olmalıdır. Denetim: `TestEveryEmittedInstructionIsSimulated`, `TestMmaFragmentLayout`, `TestShflXor`.

**Belirlenimcidir.** Aynı girdi, her çalıştırmada bit bit aynı sonucu verir. Denetim: `TestDeterministic`, `TestWide16BitDeterministic`.

**PTX'in değişimi görünürdür.** Benzeticide sınamalar sırasında derlenen her çekirdeğin PTX
özeti `testdata/ptx_ozet.txt`'dedir: bilinen bir çekirdeğin PTX'i değişirse ya da kısa kipte
özette olmayan çekirdek derlenirse paketin testi kalır. Bilinçli değişiklikte özet
`TR_GPU_PTX_GUNCELLE=1 go test -short ./internal/compute/gpu` ile yenilenir ve diff'te görünür;
"değişmeyen çekirdeklerin PTX'i aynı" iddiası böylece her koşuda denetlenir. Denetim:
`behavior_ptx_digest_test.go`.

**Profil adları kararlıdır.** Çekirdek aileleri (ör. `carpim`) profil çıktısında ve araçlarda
kullanılan metinlerdir; kodun Go adlarına bağlı değildir. Denetim: `TestKernelFamily`.

**İletiler kodludur.** Kullanıcıya giden her ileti `H4000`–`H4999` aralığından bir ileti koddur
(`messages.New`, `messages.Text`); sürücünün döndürdüğü hata adı parametre olarak geçer. Benzetici,
gerçek GPU'nun da reddedeceği durumları `H41xx` koduyla bildirir (testler kodu denetler); benzeticinin
kendi ayrıştırma ve üretim hataları (bozuk PTX, desteklenmeyen komut) TR'nin hatasıdır ve
`messages.Internal` ile İngilizce ayrıntılı bildirilir. Bellek yetmezliği, desteklenmeyen işlem ve
sınır aşımı kodludur; "bu işlem GPU'da henüz yok" ailesi `H405x`. Denetim: `TestInternalfIsInternalError`, `TestUnsupported`, tools/gen/msgconsts `TestMessageUsage`.

## Testler

Birim testleri sınadıkları dosyanın adını taşır (`x_test.go` → `x.go`). Birden çok dosyayı kesen davranış testleri:

- `behavior_half_precision_test.go`: 16 bitlik türlerin bütün kernel ailelerinde açılmadan işlenmesi
- `behavior_nn_test.go`: sinir ağı işlemlerinin GPU'da CPU arka ucuna karşı sonuçları
- `behavior_norm_reference_test.go`: normların ileri sonucu ve gradyanlarının CPU'da ve GPU'da float64 referansa göre doğruluğu
- `behavior_reduce_accuracy_test.go`: indirgemenin her yolunun float64 referansa göre doğruluğu ve özel değerler (sonsuz, NaN, -0)
- `behavior_special_values_test.go`: ±0, ±∞ ve NaN girdide bütün işlemlerin (ileri ve türev) CPU ile aynılığı ve dil tanımı 15.50'nin tablosu
- `behavior_grad_parity_test.go`: gradyan kapısının GPU ayağı: başka testte GPU-CPU gradyan karşılaştırması olmayan türevli işlemler (tür dönüşümü, cihaz aktarımı, kopya, dış işlem)
- `behavior_ptx_digest_test.go`: benzeticide derlenen her çekirdeğin PTX özetinin `testdata/ptx_ozet.txt` ile aynı olması (TestMain'den sonra)
- `behavior_sim_instruction_coverage_test.go`: kernellerin ürettiği her PTX komut adının benzetici tarafından tanınması
