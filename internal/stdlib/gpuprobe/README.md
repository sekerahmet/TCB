# gpuprobe

GPU'da dil düzeyinde eğitimin sınamaları: TR ile yazılmış eğitim programları, kitaplığın GPT'si
ve kontrol noktası GPU'da koşturulur.

## Standartlar

**Kartsız makinede de koşar.** Gerçek GPU yoksa sınamalar PTX benzeticisinde çalışır; CI her
gönderimde bu yolu sınar.

**CPU ile aynı sonuç.** GPU'daki eğitim, aynı programın CPU'daki sonucuyla (16 bitlik türlerde
tanımlı bir toleransla) karşılaştırılır; kontrol noktasından sürdürülen GPU eğitimi kesintisiz
eğitimle aynıdır.
