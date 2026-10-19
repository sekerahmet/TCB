# tensor

Tensörler, işlemleri ve otomatik türev; CPU arka ucu ve arka uçların (CPU, GPU) uyduğu arayüz.
İyileştiriciler (AdamW, SGD) ve bellek sınırı da buradadır.

## Standartlar

**Her işlemin bir başvurusu vardır.** CPU'daki hızlı yollar (vektör, assembly, paralel) Go ile
yazılmış basit bir başvuruyla aynı sonucu vermelidir; testler bunu karşılaştırır.

**Türev sınanır.** Her işlemin geri yayılımı sayısal türevle karşılaştırılır.

**Belirlenimcidir.** Paralel hesap, sıralı hesapla aynı sonucu verir; sonuç işçi sayısına bağlı
değildir.

**Arka uçlar aynı arayüze uyar.** Yeni bir arka uç (GPU gibi) arayüzü gerçekler ve CPU arka ucuyla
aynı sonucu verir; bir arka uç bir işlemi desteklemiyorsa bunu açıkça söyler.

**Veri türleri tanımlı kurallarla dönüşür.** 16 bitlik türler (bf16, f16) yuvarlama ve özel
değerler (NaN, sonsuz) için belirli kurallara uyar.
