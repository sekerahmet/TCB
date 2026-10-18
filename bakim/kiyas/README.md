# Hız kıyas seti

Aynı işi yapan küçük programlar TR (`tr/`), Python (`py/kiyas.py`) ve derlenmiş bir başvuru
sürümüyle (`go/kiyas.go`) yazılmıştır. Her program kendi ölçtüğü süreyi (ms) ve bir sonuç değeri
yazar; sonuç değerlerinin üç sürümde aynı olması gerekir. Yorumlayıcıda yapılan her hız değişikliğinden sonra yeniden ölçün.

```
python bakim/kiyas/kos.py                 # tüm testler, üç sürüm, üçer tekrar (en iyi süre)
python bakim/kiyas/kos.py --yalniz tr fib # yalnızca TR, yalnızca fib
python bakim/kiyas/kos.py --json sonuc.json
```

| Test | Ne ölçer |
|---|---|
| `dongu` | 3 milyon turluk sıkı döngü: değişken okuma/yazma ve tamsayı aritmetiği |
| `fib` | Özyinelemeli fib(27): fonksiyon çağrısı maliyeti |
| `sirala` | 300 bin sayı üretip sıralama: liste ekleme ve yerleşik sıralama |
| `metin` | 200 bin metin oluşturma: metin birleştirme ve dönüştürme |
| `sozluk` | 200 bin anahtar yazıp okuma: sözlük erişimi |
| `matris_dongu` | 120×120 matris çarpımı, iç içe döngülerle: liste indeksleme |
| `matris_yerlesik` | 512×512 matrisle iki çarpım, yerleşik `@` ile (Python sürümü bir dizi kütüphanesi ister; bkz. `py/kiyas.py`) |

Derlemeler `GOAMD64=v3` ile yapılır (ortamda `GOAMD64` verilmişse o kullanılır). 2020 sonrası x86-64 işlemciler (AVX2/FMA) hedeflenir; daha eski ya da bazı düşük güçlü işlemciler için `GOAMD64=v1` ile derleyin; ARM (arm64) bundan etkilenmez.
Matris çarpımı v3 ile (AVX2/FMA çekirdeği) v1'deki taşınabilir çekirdekten çok daha hızlıdır: 512×512
iki çarpım 20 turun ortalamasında 1 çekirdekte 65 → 4,7 ms, 2 çekirdekte 31,5 → 2,4 ms, 20 çekirdekte
12,4 → 1,6 ms (yaklaşık 8–14 kat; 18.10.2026, `GOMAXPROCS` ile sınırlanarak). Elle derlemek için:

```
GOAMD64=v3 go build -o ikili/tr.exe ./cmd/tr
```

Süreler makineye ve o anki yüke göre değişir; karşılaştırmaları aynı makinede, bilgisayar
boştayken yapın.

**Ölçüm çözünürlüğü:** TR programları süreyi `zaman_ms()` ile, Python sürümü tam sayıya kesilen
milisaniyeyle ölçer; ikisi de 1 ms çözünürlüklüdür. Birkaç milisaniyelik testlerde (ör. `matris_yerlesik`)
sonuç ±1 ms oynar ve `kos.py` en iyi süreyi aldığı için küçük değerler 1–2 ms'ye iner. Hassas karşılaştırma
için işi bir döngüde 20 kez yapıp toplam süreyi tur sayısına bölün.

**`matris_yerlesik` ölçümü (18.10.2026, 20 mantıksal çekirdekli dizüstü, `GOAMD64=v3`):** 512×512 iki
çarpım, 15'er çalıştırma: TR 1–2 ms (20 turun ortalaması 1,3–1,6 ms), Python sürümü (hazır dizi
kütüphanesiyle) 4–5 ms (20 turun ortalaması ~4,3 ms). TR tensörleri `float32` ile, Python sürümü
`float64` ile hesaplar; aynı kütüphane `float32` ile ~3 ms sürer.

## Model eğitimi karşılaştırması

`ornekler/07_karakter_modeli.tr` ile `py/karakter_modeli.py` aynı modeli (yaklaşık 119 bin parametreli,
2 katmanlı, yalnızca kod çözücülü karakter transformer'ı; tam sayı belgelerdeki farklı karakter sayısına
bağlıdır) aynı veriyle (projenin Türkçe belgeleri), aynı toplu iş örneklemesiyle ve aynı ayarlarla eğitir.
Kayıp eğrileri neredeyse aynı çıkmalıdır; süreler karşılaştırılır. Örnek 300 adım eğitir (`ADIM`
değişkeni); Python sürümü adım ve iş parçacığı sayısını argüman olarak alır.

```
GOMAXPROCS=2 tr ornekler/07_karakter_modeli.tr
python bakim/kiyas/py/karakter_modeli.py 300 2
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
(`go test ./internal/hesap/tensor -run XXX -bench KarakterModeli`).

Geçmiş ölçümler (aynı gün, 2 çekirdek): ilk sürüm TR 161 ms/adım (sürenin %69'u matris çarpımında);
vektör komutlu (AVX2/FMA) matris çarpımı ve float32 üstel/tanh sonrası, makine yüklüyken TR 22–31 ms/adım.
