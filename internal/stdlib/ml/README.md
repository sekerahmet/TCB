# ml

TR'nin tensör ve yapay zekâ yerleşikleri: tensör değeri, işleçler (`+ - * / @ **`), sinir ağı
işlemleri (normlar, dikkat, çapraz entropi), indirgemeler, iyileştiriciler (AdamW, SGD, Muon),
özel türevler, karışık hassasiyet ve tensör dosyaları. Asıl hesabı `internal/compute/tensor`
yapar; bu paket onu TR değerlerine, adlarına ve yol gösterici hata iletilerine bağlar.

## Standartlar

**Hesap çekirdektedir.** Bu paket tensör hesabı yazmaz; çekirdeğe ne yapılacağını söyler.
Aynı işlemin CPU ve GPU sonuçları aynıdır (aynı çekirdek sözleşmesi).

**Adlar sözlükten gelir.** Yerleşiklerin, üyelerin ve adlı argümanların adları sabit kodlara
bağlıdır; ad değişikliği yalnızca çeviri dosyasını değiştirir.

**Karışık hassasiyet bloğa bağlıdır.** `karışık_hassasiyet(tür) { ... }` görevin durumudur:
başka görevlere sızmaz, blok bitince (hata ile de) önceki tür geri gelir. Ağırlıkların türü
değişmez; dönüşümler türevlenebilir kopyalardır. Kurallar dil tanımının 15.13 maddesindedir.

**Tensör dosyası doğrulanır.** Başlık, sürüm, tür, boyutlar, eleman sınırı, uzunluk ve sağlama
toplamı denetlenir; bozuk ya da eksik dosya açık bir hatayla reddedilir. Biçim değişirse sürüm
artar, eski sürümler okunmaya devam eder.

**Hata panik değildir.** Çekirdeğin panikleri (şekil, cihaz belleği) TR hatasına çevrilir;
ileti hangi işlemin neden yapılamadığını ve ne yapılacağını söyler.
