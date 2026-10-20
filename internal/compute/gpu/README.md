# gpu

NVIDIA GPU arka ucu: çekirdekleri PTX olarak kendisi üretir, sürücüyü cgo'suz yükler. Bellek
havuzu, matris çarpımı kitaplığı (varsa) ve profil burada. GPU olmayan makinede çekirdekler bir
PTX benzeticisinde çalıştırılarak sınanır.

## Standartlar

**CPU ile aynı sonuç.** Her GPU işlemi CPU arka ucunun sonucuyla karşılaştırılarak sınanır
(16 bitlik türlerde tanımlı bir toleransla). Denetim: `behavior_nn_test.go`, `behavior_half_precision_test.go`, `behavior_grad_parity_test.go`.

**GPU yoksa zarif.** Sürücü ya da kart yoksa program çökmez; GPU görünmez olur ve nedeni
sorulabilir. Denetim: `TestNoDriverGraceful`.

**Benzetici gerçeğe sadıktır.** Benzetici, gerçek GPU'nun reddedeceği kodu (sınır dışı erişim,
yarış, engel hatası) da reddeder; böylece GPU'suz makinede yakalanan hata gerçek kartta da
hatadır. Denetim: `TestSimCatchesRace`, `TestSimDivergentBarrier`, `TestSimOutOfBounds`.

**Her donanım komutu benzeticide de yürür.** Çekirdeklere eklenen her PTX komutu (warp
düzeyindeki tensör çekirdeği ve değiş tokuş komutları dahil) benzeticiye de eklenir. Komutun veri
düzenini sınayan test hem benzeticide hem gerçek kartta koşar; ikisinin sonucu aynı olmalıdır. Denetim: `TestEveryEmittedInstructionIsSimulated`, `TestMmaFragmentLayout`, `TestShflXor`.

**Belirlenimcidir.** Aynı girdi, her çalıştırmada bit bit aynı sonucu verir. Denetim: `TestDeterministic`, `TestWide16BitDeterministic`.

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

- `behavior_half_precision_test.go`: 16 bitlik türlerin bütün çekirdek ailelerinde açılmadan işlenmesi
- `behavior_nn_test.go`: sinir ağı işlemlerinin GPU'da CPU arka ucuna karşı sonuçları
- `behavior_grad_parity_test.go`: gradyan kapısının GPU ayağı: başka testte GPU-CPU gradyan karşılaştırması olmayan türevli işlemler (tür dönüşümü, cihaz aktarımı, kopya, dış işlem)
- `behavior_sim_instruction_coverage_test.go`: çekirdeklerin ürettiği her PTX komut adının benzetici tarafından tanınması
