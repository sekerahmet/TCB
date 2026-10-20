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

## bf16 toleransları

Gradyan kapısı her kaydı bf16 girdiyle koşar ve çıktı ile gradyanları, bf16'ya yuvarlanmış aynı girdilerle float32 koşuyla karşılaştırır; hata tensörün en büyük mutlak değerine göre ölçülür. Tolerans kayıt başınadır (`behavior_grad_bf16_tolerances_test.go`, `grup/ad` anahtarlı) ve ölçülür: **en büyük ölçülen sapmanın 2 katı, 1e-3'ün katına yukarı yuvarlanır, en az 4e-3** (bir bf16 yuvarlaması, 2⁻⁸). Gerekçe: 2× tohum ve şekil değişkenliğine pay bırakır ama tek bir genel 3e-2 sabitinin sakladığı 5-10 kat boşluğu kapatır; alt sınır, tam çıkan işlemlerde (görünümler, karşılaştırmalar; ölçülen sapma 0) başka hesap sırasıyla çalışan arka ucu gereksiz düşürmesin diye vardır. Kapı her koşuda 4 girdi tohumu dener; ölçüm 32 tohumla yapılır.

Yeniden ölçmek (CPU, tablo basar): `TR_MEASURE_BF16=1 go test ./internal/compute/tensor -run TestMeasureBF16 -v`. Tablo dosyasını yenilemek için `TR_MEASURE_BF16=write` verilir. Yeni ya da değişen bir kayıt tabloda yoksa kapsam testi düşer. GPU'da sapma farklı olabilir; bu tablo yalnızca CPU ölçümüdür.

| kayıt | en büyük sapma | tolerans |
| :-- | --: | --: |
| attention/dikkat 2B | 0.0062 | 0.013 |
| attention/dikkat maske | 0.0059 | 0.012 |
| attention/dikkat nedensel GQA | 0.0045 | 0.01 |
| attention/dikkat nedensel pencere ölçek | 0.0123 | 0.025 |
| attention/dikkat pencere | 0.0055 | 0.011 |
| concat/concat eksen 0 | 0.0029 | 0.006 |
| concat/concat eksen 1 | 0.0020 | 0.004 |
| cumsum_sort/cumsum eksen 0 | 0.0039 | 0.008 |
| cumsum_sort/cumsum eksen 1 | 0.0037 | 0.008 |
| cumsum_sort/topk eksen 0 | 0.0021 | 0.005 |
| cumsum_sort/topk eksen 1 | 0.0021 | 0.005 |
| custom_op/kare (dış işlem) | 0.0047 | 0.01 |
| dtype/cast bf16 | 0.0000 | 0.004 |
| dtype/cast f16 | 0.0000 | 0.004 |
| elementwise/add [2 3 4] [4] | 0.0033 | 0.007 |
| elementwise/add [2 3] [2 3] | 0.0036 | 0.008 |
| elementwise/add_scalar | 0.0034 | 0.007 |
| elementwise/div [2 3] [2 1] | 0.0117 | 0.024 |
| elementwise/div [2 3] [2 3] | 0.0054 | 0.011 |
| elementwise/div_scalar | 0.0021 | 0.005 |
| elementwise/exp | 0.0050 | 0.011 |
| elementwise/ge | 0.0000 | 0.004 |
| elementwise/gelu | 0.0031 | 0.007 |
| elementwise/gt | 0.0000 | 0.004 |
| elementwise/le | 0.0000 | 0.004 |
| elementwise/log | 0.0032 | 0.007 |
| elementwise/lt | 0.0000 | 0.004 |
| elementwise/mul [2 3 4] [3 1] | 0.0085 | 0.017 |
| elementwise/mul [2 3] [2 3] | 0.0032 | 0.007 |
| elementwise/mul_scalar | 0.0036 | 0.008 |
| elementwise/neg | 0.0021 | 0.005 |
| elementwise/pow | 0.0029 | 0.006 |
| elementwise/pow negatif üs | 0.0035 | 0.007 |
| elementwise/rdiv_scalar | 0.0046 | 0.01 |
| elementwise/relu | 0.0029 | 0.006 |
| elementwise/rsub_scalar | 0.0027 | 0.006 |
| elementwise/sigmoid | 0.0151 | 0.031 |
| elementwise/sqrt | 0.0048 | 0.01 |
| elementwise/sub [2 1 3] [4 1] | 0.0037 | 0.008 |
| elementwise/sub [3 1] [1 4] | 0.0033 | 0.007 |
| elementwise/sub_scalar | 0.0037 | 0.008 |
| elementwise/tanh | 0.0086 | 0.018 |
| elementwise_wide/abs | 0.0020 | 0.004 |
| elementwise_wide/acos | 0.0034 | 0.007 |
| elementwise_wide/asin | 0.0037 | 0.008 |
| elementwise_wide/atan | 0.0034 | 0.007 |
| elementwise_wide/atan2 yayın | 0.0055 | 0.012 |
| elementwise_wide/ceil | 0.0000 | 0.004 |
| elementwise_wide/clamp | 0.0029 | 0.006 |
| elementwise_wide/cosh | 0.0029 | 0.006 |
| elementwise_wide/expm1 | 0.0031 | 0.007 |
| elementwise_wide/floor | 0.0000 | 0.004 |
| elementwise_wide/gelu_exact | 0.0035 | 0.008 |
| elementwise_wide/log10 | 0.0029 | 0.006 |
| elementwise_wide/log1p | 0.0036 | 0.008 |
| elementwise_wide/log2 | 0.0031 | 0.007 |
| elementwise_wide/sign | 0.0000 | 0.004 |
| elementwise_wide/silu | 0.0032 | 0.007 |
| elementwise_wide/sinh | 0.0030 | 0.006 |
| elementwise_wide/tan | 0.0086 | 0.018 |
| elementwise_wide/where yayın | 0.0046 | 0.01 |
| limits/cross_entropy yok sayma | 0.0000 | 0.004 |
| limits/embedding | 0.0016 | 0.004 |
| limits/layer_norm yalın | 0.0027 | 0.006 |
| limits/layer_norm γβ | 0.0044 | 0.009 |
| limits/rms_norm yalın | 0.0027 | 0.006 |
| limits/rms_norm γ | 0.0056 | 0.012 |
| matmul/matmul 2B | 0.0037 | 0.008 |
| matmul/matmul ND@2B | 0.0027 | 0.006 |
| matmul/matmul devrik a | 0.0029 | 0.006 |
| matmul/matmul devrik b | 0.0037 | 0.008 |
| matmul/matmul devrik ikisi | 0.0036 | 0.008 |
| matmul/matmul kenar boyutlar | 0.0031 | 0.007 |
| matmul/matmul toplu 3B | 0.0031 | 0.007 |
| matmul/matmul toplu 4B | 0.0030 | 0.007 |
| reduce/log_softmax orta eksen | 0.0039 | 0.008 |
| reduce/log_softmax son eksen | 0.0040 | 0.008 |
| reduce/max eksen 0 keepdim | 0.0029 | 0.006 |
| reduce/max orta eksen | 0.0029 | 0.006 |
| reduce/max son eksen | 0.0029 | 0.006 |
| reduce/mean eksen 0 keepdim | 0.0037 | 0.008 |
| reduce/mean eksen 1 | 0.0034 | 0.007 |
| reduce/mean_all | 0.0030 | 0.007 |
| reduce/softmax eksen 0 | 0.0065 | 0.013 |
| reduce/softmax son eksen | 0.0034 | 0.007 |
| reduce/sum eksen 0 | 0.0037 | 0.008 |
| reduce/sum orta eksen keepdim | 0.0038 | 0.008 |
| reduce/sum son eksen | 0.0039 | 0.008 |
| reduce/sum_all | 0.0034 | 0.007 |
| tensor/to | 0.0034 | 0.007 |
| trig/cos | 0.0029 | 0.006 |
| trig/sin | 0.0027 | 0.006 |
| view/clone | 0.0000 | 0.004 |
| view/contiguous | 0.0000 | 0.004 |
| view/devrik reshape | 0.0000 | 0.004 |
| view/permute | 0.0000 | 0.004 |
| view/reshape | 0.0000 | 0.004 |
| view/slice | 0.0000 | 0.004 |
| view/slice adımlı | 0.0000 | 0.004 |
| view/split | 0.0000 | 0.004 |
| view/squeeze unsqueeze | 0.0000 | 0.004 |
| view/transpose | 0.0000 | 0.004 |

## Testler

Birim testleri sınadıkları dosyanın adını taşır (`x_test.go` → `x.go`). Birden çok dosyayı kesen davranış testleri:

- `behavior_forward_test.go`: her işlemin ileri değerleri elle hesaplanmış sonuçlara karşı
- `behavior_end_to_end_test.go`: küçük modellerin CPU arka ucunda uçtan uca eğitimi
- `behavior_grad_coverage_test.go`: gradyan kapısının kapsam denetimi: `BinOp`/`UnOp` üyeleri ve `track` çağrılarından çıkan her türevli işlemin float64 sonlu fark + bf16 tolerans kaydı (`gradSpec`) ve çalıştıran testi var; eksik ya da fazla kayıt testi düşürür; her kaydın `bf16Tolerances` girdisi de zorunludur
- `behavior_grad_bf16_measure_test.go`: bf16 toleransı ölçümü (`TR_MEASURE_BF16`); tabloyu basar ya da `behavior_grad_bf16_tolerances_test.go`'yu yeniden yazar
- `behavior_grad_bf16_tolerances_test.go`: üretilmiş, kayıt başına bf16 tolerans tablosu (elle düzenlenmez)
