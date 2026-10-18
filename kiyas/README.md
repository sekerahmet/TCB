# Hız kıyas seti

Aynı işi yapan küçük programlar TR (`tr/`), Python (`py/kiyas.py`) ve derlenmiş bir başvuru
sürümüyle (`go/kiyas.go`) yazılmıştır. Her program kendi ölçtüğü süreyi (ms) ve bir sonuç değeri
yazar; sonuç değerlerinin üç sürümde aynı olması gerekir. Yorumlayıcıda yapılan her hız değişikliğinden sonra yeniden ölçün.

```
python kiyas/kos.py                 # tüm testler, üç sürüm, üçer tekrar (en iyi süre)
python kiyas/kos.py --yalniz tr fib # yalnızca TR, yalnızca fib
python kiyas/kos.py --json sonuc.json
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
Matris çarpımı v3 ile yaklaşık iki kat hızlıdır (FMA/VEX komutları). Elle derlemek için:

```
GOAMD64=v3 go build -o tr.exe ./cmd/tr
```

Süreler makineye ve o anki yüke göre değişir; karşılaştırmaları aynı makinede, bilgisayar
boştayken yapın.

## Model eğitimi karşılaştırması

`ornekler/karakter_modeli.tr` ile `py/karakter_modeli.py` aynı modeli (yaklaşık 119 bin parametreli,
2 katmanlı, yalnızca kod çözücülü karakter transformer'ı; tam sayı belgelerdeki farklı karakter sayısına
bağlıdır) aynı veriyle (projenin Türkçe belgeleri), aynı toplu iş örneklemesiyle ve aynı ayarlarla eğitir.
Kayıp eğrileri neredeyse aynı çıkmalıdır; süreler karşılaştırılır. Örnek 300 adım eğitir (`ADIM`
değişkeni); Python sürümü adım ve iş parçacığı sayısını argüman olarak alır.

```
GOMAXPROCS=2 tr ornekler/karakter_modeli.tr
python kiyas/py/karakter_modeli.py 300 2
```

**Güncel ölçüm (18.10.2026, 2 çekirdek, 100 adım, `GOAMD64=v3`):** TR ~13,5 ms/adım, Python sürümü
~10–13,5 ms/adım. Aynı gün tekrar ölçüldü (dizüstü bilgisayar; makine başka işlerle paylaşılıyordu):
TR en iyi 13,5, çoğunlukla 13,5–16 ms/adım, yükün arttığı anlarda 18–22 ms/adım; Python sürümü
12–13,5 ms/adım. Yüklü makinede farklar büyür; karşılaştırmayı boş makinede yineleyin.

Geçmiş ölçümler (aynı gün, 2 çekirdek): ilk sürüm TR 161 ms/adım (sürenin %69'u matris çarpımında);
vektör komutlu (AVX2/FMA) matris çarpımı ve float32 üstel/tanh sonrası, makine yüklüyken TR 22–31 ms/adım.
