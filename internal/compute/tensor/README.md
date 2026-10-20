# tensor

Tensörler, işlemleri ve otomatik türev; CPU arka ucu ve arka uçların (CPU, GPU) uyduğu arayüz.
İyileştiriciler (AdamW, SGD) ve bellek sınırı da buradadır.

## Standartlar

**Her işlemin bir başvurusu vardır.** CPU'daki hızlı yollar (vektör, assembly, paralel) Go ile
yazılmış basit bir başvuruyla aynı sonucu vermelidir; testler bunu karşılaştırır. Denetim: `TestAssemblyKernelsMatchGo`, `TestMicro6x16GoMatchesAVX2`, `TestNormVectorMatchesGo`.

**Türev sınanır.** Her işlemin geri yayılımı sayısal türevle karşılaştırılır. Denetim: `TestGradCoverage`.

**Belirlenimcidir.** Paralel hesap, sıralı hesapla aynı sonucu verir; sonuç işçi sayısına bağlı
değildir. Denetim: `TestParallelKernelsMatchSerial`, `TestAttentionIndependentOfWorkers`.

**Arka uçlar aynı arayüze uyar.** Yeni bir arka uç (GPU gibi) arayüzü gerçekler ve CPU arka ucuyla
aynı sonucu verir; bir arka uç bir işlemi desteklemiyorsa bunu açıkça söyler. Denetim: `TestLazyBackendRegistration`; GPU ayağı gpu paketinin `TestUnsupported` ve `behavior_*` testleri.

**Veri türleri tanımlı kurallarla dönüşür.** 16 bitlik türler (bf16, f16) yuvarlama ve özel
değerler (NaN, sonsuz) için belirli kurallara uyar. Denetim: `TestBF16ConversionEdges`, `TestF16ConversionEdges`, `TestSixteenBitRounding`.

## Testler

Birim testleri sınadıkları dosyanın adını taşır (`x_test.go` → `x.go`). Birden çok dosyayı kesen davranış testleri:

- `behavior_forward_test.go`: her işlemin ileri değerleri elle hesaplanmış sonuçlara karşı
- `behavior_end_to_end_test.go`: küçük modellerin CPU arka ucunda uçtan uca eğitimi
- `behavior_grad_coverage_test.go`: gradyan kapısının kapsam denetimi: `BinOp`/`UnOp` üyeleri ve `track` çağrılarından çıkan her türevli işlemin float64 sonlu fark + bf16 tolerans kaydı (`gradSpec`) ve çalıştıran testi var; eksik ya da fazla kayıt testi düşürür
