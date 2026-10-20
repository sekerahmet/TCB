# gpu

NVIDIA GPU arka ucu: çekirdekleri PTX olarak kendisi üretir, sürücüyü cgo'suz yükler. Bellek
havuzu, matris çarpımı kitaplığı (varsa) ve profil burada. GPU olmayan makinede çekirdekler bir
PTX benzeticisinde çalıştırılarak sınanır.

## Standartlar

**CPU ile aynı sonuç.** Her GPU işlemi CPU arka ucunun sonucuyla karşılaştırılarak sınanır
(16 bitlik türlerde tanımlı bir toleransla).

**GPU yoksa zarif.** Sürücü ya da kart yoksa program çökmez; GPU görünmez olur ve nedeni
sorulabilir.

**Benzetici gerçeğe sadıktır.** Benzetici, gerçek GPU'nun reddedeceği kodu (sınır dışı erişim,
yarış, engel hatası) da reddeder; böylece GPU'suz makinede yakalanan hata gerçek kartta da
hatadır.

**Her donanım komutu benzeticide de yürür.** Çekirdeklere eklenen her PTX komutu (warp
düzeyindeki tensör çekirdeği ve değiş tokuş komutları dahil) benzeticiye de eklenir. Komutun veri
düzenini sınayan test hem benzeticide hem gerçek kartta koşar; ikisinin sonucu aynı olmalıdır.

**Belirlenimcidir.** Aynı girdi, her çalıştırmada bit bit aynı sonucu verir.

**Profil adları kararlıdır.** Çekirdek aileleri (ör. `carpim`) profil çıktısında ve araçlarda
kullanılan metinlerdir; kodun Go adlarına bağlı değildir.

## Testler

Birim testleri sınadıkları dosyanın adını taşır (`x_test.go` → `x.go`). Birden çok dosyayı kesen davranış testleri:

- `behavior_half_precision_test.go`: 16 bitlik türlerin bütün çekirdek ailelerinde açılmadan işlenmesi
- `behavior_nn_test.go`: sinir ağı işlemlerinin GPU'da CPU arka ucuna karşı sonuçları
