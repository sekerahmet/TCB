# Hız kıyas seti

Aynı işi yapan küçük programlar TR (`tr/`), Python (`py/compare.py`) ve derlenmiş bir başvuru
sürümüyle (`go/compare.go`) yazılmıştır. Her program kendi ölçtüğü süreyi (ms) ve bir sonuç değeri
yazar; sonuç değerlerinin üç sürümde aynı olması gerekir. Yorumlayıcıda yapılan her hız değişikliğinden sonra yeniden ölçün.

```
python tools/bench/run.py                 # tüm testler, üç sürüm, üçer tekrar (--repeat; en iyi süre)
python tools/bench/run.py --list          # test adları
python tools/bench/run.py --only tr fib   # yalnızca TR, yalnızca fib
python tools/bench/run.py --json sonuc.json         # sonuçları dosyaya yaz
```

| Test | Ne ölçer |
|---|---|
| `dongu` | 3 milyon turluk sıkı döngü: değişken okuma/yazma ve tamsayı aritmetiği |
| `fib` | Özyinelemeli fib(27): fonksiyon çağrısı maliyeti |
| `sirala` | 300 bin sayı üretip sıralama: liste ekleme ve yerleşik sıralama |
| `metin` | 200 bin metin oluşturma: metin birleştirme ve dönüştürme |
| `sozluk` | 200 bin anahtar yazıp okuma: sözlük erişimi |
| `matris_dongu` | 120×120 matris çarpımı, iç içe döngülerle: liste indeksleme |
| `matris_yerlesik` | 512×512 matrisle iki çarpım, yerleşik `@` ile (Python sürümü bir dizi kütüphanesi ister; bkz. `py/compare.py`) |

Derlemeler varsayılan ayarla yapılır (ortamdaki `GOAMD64` geçerlidir). AVX2/FMA çekirdekleri çalışma
anında seçilir; `TR_CPU_AVX2=0` taşınabilir Go eşlerini kullandırır. Matris çarpımı AVX2 çekirdeğiyle
taşınabilir çekirdekten çok daha hızlıdır: 512×512 iki çarpım 20 turun ortalamasında 1 çekirdekte
65 → 4,7 ms, 2 çekirdekte 31,5 → 2,4 ms, 20 çekirdekte 12,4 → 1,6 ms (yaklaşık 8–14 kat; 18.10.2026,
`GOMAXPROCS` ile sınırlanarak; o gün çekirdek derlemeyle, `GOAMD64=v3`/`v1`, seçiliyordu).

**Çalışma anında seçim (19.10.2026, karakter modeli, 2 çekirdek, 300 adım):** `GOAMD64=v1` derleme
önce 81–88, şimdi 9,4–11 ms/adım (`GOAMD64=v3` ile aynı: 9,7–11,2). AVX2'siz yol (`TR_CPU_AVX2=0`)
88–93 ms/adım: amd64'te çekirdek düzeni AVX2'ninkiyle aynı (6×16) olduğu için eski Go 4×2
çekirdeğinden (80–88) ~%7 yavaş; bu yalnızca AVX2'siz işlemcileri etkiler. Elle derlemek için:

```
go build -o bin/tr.exe ./cmd/tr
```

Süreler makineye ve o anki yüke göre değişir; karşılaştırmaları aynı makinede, bilgisayar
boştayken yapın.

**Ölçüm çözünürlüğü:** TR programları süreyi `zaman_ms()` ile, Python sürümü tam sayıya kesilen
milisaniyeyle ölçer; ikisi de 1 ms çözünürlüklüdür. Birkaç milisaniyelik testlerde (ör. `matris_yerlesik`)
sonuç ±1 ms oynar ve `run.py` en iyi süreyi aldığı için küçük değerler 1–2 ms'ye iner. Hassas karşılaştırma
için işi bir döngüde 20 kez yapıp toplam süreyi tur sayısına bölün.

**`matris_yerlesik` ölçümü (18.10.2026, 20 mantıksal çekirdekli dizüstü, `GOAMD64=v3`):** 512×512 iki
çarpım, 15'er çalıştırma: TR 1–2 ms (20 turun ortalaması 1,3–1,6 ms), Python sürümü (hazır dizi
kütüphanesiyle) 4–5 ms (20 turun ortalaması ~4,3 ms). TR tensörleri `float32` ile, Python sürümü
`float64` ile hesaplar; aynı kütüphane `float32` ile ~3 ms sürer.

## Model eğitimi karşılaştırması

`examples/tr/15_karakter_modeli.tr` ile `py/karakter_modeli.py` aynı modeli (yaklaşık 119 bin parametreli,
2 katmanlı, yalnızca kod çözücülü karakter transformer'ı; tam sayı belgelerdeki farklı karakter sayısına
bağlıdır) aynı veriyle (projenin Türkçe belgeleri), aynı toplu iş örneklemesiyle ve aynı ayarlarla eğitir.
Kayıp eğrileri neredeyse aynı çıkmalıdır; süreler karşılaştırılır. Örnek 300 adım eğitir (`ADIM`
değişkeni); Python sürümü adım ve iş parçacığı sayısını argüman olarak alır.

```
GOMAXPROCS=2 tr examples/tr/15_karakter_modeli.tr
python tools/bench/py/karakter_modeli.py 300 2
```

**Güncel ölçüm (18.10.2026, 2 çekirdek, 100 adım, `GOAMD64=v3`):** TR ~13,5 ms/adım, Python sürümü
~10–13,5 ms/adım. Aynı gün tekrar ölçüldü (dizüstü bilgisayar; makine başka işlerle paylaşılıyordu):
TR en iyi 13,5, çoğunlukla 13,5–16 ms/adım, yükün arttığı anlarda 18–22 ms/adım; Python sürümü
12–13,5 ms/adım. Yüklü makinede farklar büyür; karşılaştırmayı boş makinede yineleyin.

**Birleşik dikkat (`dikkat(...)`) sonrası (18.10.2026, 2 çekirdek, 300 adım, `GOAMD64=v3`, makine
yüklü):** örnek elle kurulan maske ve softmax yerine `dikkat(q, k, v)` kullanır; kayıp eğrisi önceki
sürümle 4 basamağa kadar aynıdır. Aynı veriyle art arda çalıştırılan çiftlerde TR önce/sonra 26,8/21,6,
24,1/19,8, 31,7/27,3, 29,0/19,8 ms/adım (en iyi: 20,6 → 18,8); Python sürümü 20,8 ms/adım. Yalnızca
dikkatin ileri+geri süresi bu şekilde (toplu 16, 4 baş, T 64, dh 16) 3,8 ms'den 1,8 ms'ye iner
(`go test ./internal/compute/tensor -run XXX -bench KarakterModeli`).

Geçmiş ölçümler (aynı gün, 2 çekirdek): ilk sürüm TR 161 ms/adım (sürenin %69'u matris çarpımında);
vektör komutlu (AVX2/FMA) matris çarpımı ve float32 üstel/tanh sonrası, makine yüklüyken TR 22–31 ms/adım.
