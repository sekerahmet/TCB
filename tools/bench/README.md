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

## Güncel taban (2026-11-01, T1 öncesi)

Faz 0'ın tipli yazıma göçü, D1'in ilk dilimi ve Gemini'nin Faz 1 işinden sonra, tensör çekirdeğini değiştirecek T1'den önce. Ham kayıtlar Drive'da `TR_dili/olcumler/` (`OZET.md`, `cpu-temel/`, `kart/`); kayıt aracı (`tools/record`) her koşuya commit'i ve sürümleri yazar.

**Dil kıyası** (Colab CPU High-RAM, AMD EPYC 7B12, 8 sanal çekirdek, Python 3.13.18; 5 tekrarın en iyisi; main `6d21ed09`; üç sürümün sonuç değerleri aynı) [ölçüldü: `cpu_baseline.sh`, `cpu-temel/20261101183359.log`]:

| Test | TR | Python | Go | TR/Python |
| :-- | --: | --: | --: | --: |
| `dongu` | 276 ms | 204 ms | 2 ms | 1,35 |
| `fib` | 71 | 31 | 1 | 2,29 |
| `sirala` | 117 | 166 | 32 | 0,70 |
| `metin` | 82 | 28 | 8 | 2,93 |
| `sozluk` | 284 | 138 | 125 | 2,06 |
| `matris_dongu` | 306 | 142 | 2 | 2,15 |
| `matris_yerlesik` | 7 | 8 (NumPy 2.1.3) | 185 | 0,88 |

Yorumlayıcı işinde (ilk altı test) TR, Python'dan ortalama 1,75 kat yavaş [hesap: geometrik ortalama]; en büyük fark metin (2,9) ve fonksiyon çağrısı (2,3). Aynı makinede 30 dakika önceki koşu (`b0b4c579`) 0,7–2,9 arası [ölçüldü: `cpu-temel/20261101181506.log`].

**Rust ile** (aynı makine türü, başka bir örnek; rustc 1.99.1 `-C opt-level=3`, `rs/compare.rs`; 5 tekrarın en iyisi; `2ec818a2`; dört sürümün sonuç değerleri aynı; Go ve Rust süreyi mikrosaniye çözünürlükte yazar) [ölçüldü: `dil-kiyasi/20261101191238.log`]:

| Test | TR | Python | Go | Rust | TR/Rust |
| :-- | --: | --: | --: | --: | --: |
| `dongu` | 378 ms | 252 | 1,14 | 2,35 | 161 |
| `fib` | 100 | 39 | 1,38 | 0,90 | 111 |
| `sirala` | 172 | 223 | 39,7 | 13,0 | 13 |
| `metin` | 118 | 43 | 11,6 | 12,4 | 9,5 |
| `sozluk` | 370 | 184 | 112 | 124 | 3,0 |
| `matris_dongu` | 413 | 165 | 2,97 | 3,45 | 120 |
| `matris_yerlesik` | 5 | 8 (NumPy) | 321 | 125 | 0,04 |

Bu örnekte TR ve Python yukarıdaki koşudan yavaş; ilk altı testte TR/Python oranları 0,8–2,7. Saf hesapta (döngü, çağrı, indeksleme) TR derlenmiş dillerden iki basamak yavaştır: Rust'a göre 111–161, Go'ya göre 73–333 kat [hesap]; iş bir yerleşiğe düşünce (sözlük, sıralama, metin) fark 3–13 kata iner. `matris_yerlesik`'te Go ve Rust düz üçlü döngüdür, TR ve NumPy hazır çekirdek: bu satır dili değil yerleşiği karşılaştırır.

Rust'ın ilk iki koşusu `dongu` ve `fib`'de 0 ms verdi: LLVM sabit ya da bilinmeyen sınırlı toplamı n(n−1)/2'ye indiriyor, tam milisaniye 1 ms'nin altını gizliyordu. Rust sürümü her adımı `black_box`'tan geçirir, Go ve Rust küsuratlı milisaniye yazar (`d220e814`, `2ec818a2`; DENETİM'in okumasından). Her adımdaki `black_box` döngünün vektörlenmesini de engeller: `dongu`'da Rust tur başına 0,78 ns, Go 0,38 ns [hesap]; Rust'ın `dongu` değeri gerçeğinden yavaştır ve TR/Rust 161 bir alt sınırdır. `fib`'de `black_box` yalnız argümandadır, 0,90 ms gerçek değerdir.

**Karakter modeli, CPU** (aynı makine ve koşu, PyTorch 2.14.1; ikisinin kaybı 300 adımda 2,21):

| İş parçacığı | TR ms/adım | PyTorch ms/adım | TR/PyTorch |
| --: | --: | --: | --: |
| 1 | 22,37 | 27,32 | 0,82 |
| 2 | 19,38 | 20,62 | 0,94 |
| 4 | 16,74 | 18,01 | 0,93 |
| 8 | 17,70 | 18,75 | 0,94 |

**Hedef model, GPU** (Colab G4, RTX PRO 6000 Blackwell, sürücü 580.82.07; sentetik veri, 120 adım; main `e9c64a45`, motor dosyaları `6d21ed09`'a kadar değişmedi [ölçüldü: `git diff e9c64a45..6d21ed09 -- internal cmd lib`]):

| | ms/adım (ortanca) | Tepe bellek |
| :-- | --: | --: |
| TR | 145 | 10,57 GiB |
| PyTorch 2.11, `torch.compile` | 136 | 10,83 GiB |
| PyTorch 2.11, derlemesiz | 347 | 16,95 GiB |

İlk G4 koşusu (`kart/20261101175825`) "KALDI" ile bitti: hız sayısı geçerli, ama yalnız GPU'da koşan iki dil tanımı testi (15.40, 15.41) iletiyi Türkçe metinle arıyordu ve İngilizce makinede kaldı; `02dee59b` ileti koduna geçirdi, `kart/20261101185157` GEÇTİ. PyTorch sayıları kayıt aracıyla alınan `pytorch-temel/20261101185327`'den.

TR derlenmiş PyTorch'un 1,07 katı [hesap: 145/136]; 2026-10-23'teki oranla aynı. Bulut 4a2'nin ikinci dilimi birleştikten sonra (`a2417c16`) aynı ölçüm: TR 145 ms/adım ve 10.823 MiB, GPU testleri ve dil tanımı testlerinin hepsi geçti [ölçüldü: `kart/20261101185157`]; PyTorch derlenmiş 136, derlemesiz 347 ms/adım, bellek aynı [ölçüldü: `pytorch-temel/20261101185327`, kayıt aracıyla].

**Bu koşularda bulunan ve düzelen kırılmalar:** karakter modeli kıyası Faz 0'dan beri çalışmıyordu (`run.py` eski yazımı, `py/char_model.py` silinmiş 14. bölümü arıyordu; `7b5f9958`, `6d21ed09`); düşen kıyas sessizce boş sonuç veriyordu (`02dee59b`); iki GPU testi iletiyi Türkçe metinle arıyordu ve İngilizce makinede kaldı (`02dee59b`).

## Ölçüm notları

Derlemeler varsayılan ayarla yapılır (ortamdaki `GOAMD64` geçerlidir). AVX2/FMA kernelleri çalışma
anında seçilir; `TR_CPU_AVX2=0` taşınabilir Go eşlerini kullandırır. Matris çarpımı AVX2 kerneliyle
taşınabilir kernelden çok daha hızlıdır: 512×512 iki çarpım 20 turun ortalamasında 1 çekirdekte
65 → 4,7 ms, 2 çekirdekte 31,5 → 2,4 ms, 20 çekirdekte 12,4 → 1,6 ms (yaklaşık 8–14 kat; 18.10.2026,
`GOMAXPROCS` ile sınırlanarak; o gün kernel derlemeyle, `GOAMD64=v3`/`v1`, seçiliyordu).

**Çalışma anında seçim (19.10.2026, karakter modeli, 2 çekirdek, 300 adım):** `GOAMD64=v1` derleme
önce 81–88, şimdi 9,4–11 ms/adım (`GOAMD64=v3` ile aynı: 9,7–11,2). AVX2'siz yol (`TR_CPU_AVX2=0`)
88–93 ms/adım: amd64'te kernel düzeni AVX2'ninkiyle aynı (6×16) olduğu için eski Go 4×2
çekirdeğinden (80–88) ~%7 yavaş; bu yalnızca AVX2'siz işlemcileri etkiler. Elle derlemek için:

```
go build -o bin/tr.exe ./cmd/tr
```

Süreler makineye ve o anki yüke göre değişir; karşılaştırmaları aynı makinede, bilgisayar
boştayken yapın.

**Ölçüm çözünürlüğü:** TR programları süreyi `TR_time_ms()` ile, Python sürümü tam sayıya kesilen
milisaniyeyle ölçer; ikisi de 1 ms çözünürlüklüdür. Birkaç milisaniyelik testlerde (ör. `matris_yerlesik`)
sonuç ±1 ms oynar ve `run.py` en iyi süreyi aldığı için küçük değerler 1–2 ms'ye iner. Hassas karşılaştırma
için işi bir döngüde 20 kez yapıp toplam süreyi tur sayısına bölün.

**`matris_yerlesik` ölçümü (18.10.2026, 20 mantıksal çekirdekli dizüstü, `GOAMD64=v3`):** 512×512 iki
çarpım, 15'er çalıştırma: TR 1–2 ms (20 turun ortalaması 1,3–1,6 ms), Python sürümü (hazır dizi
kütüphanesiyle) 4–5 ms (20 turun ortalaması ~4,3 ms). TR tensörleri `float32` ile, Python sürümü
`float64` ile hesaplar; aynı kütüphane `float32` ile ~3 ms sürer.

## Model eğitimi karşılaştırması

`examples/tr/36_karakter_modeli.tr` ile `py/char_model.py` aynı modeli (yaklaşık 119 bin parametreli,
2 katmanlı, yalnızca kod çözücülü karakter transformer'ı; tam sayı belgelerdeki farklı karakter sayısına
bağlıdır) aynı veriyle (projenin Türkçe belgeleri), aynı toplu iş örneklemesiyle ve aynı ayarlarla eğitir.
Başlangıç ağırlıkları iki çerçevenin kendi üreteciyle çekildiği için kayıplar aynı değil, yakındır (300
adımda ~2,4–2,5); süreler karşılaştırılır. Örnek 300 adım eğitir (`ADIM` sabiti); Python sürümü adım ve iş
parçacığı sayısını argüman olarak alır.

```
python tools/bench/run.py karakter_modeli --threads 2 --repeat 5
```

`run.py` iki programı 300 adımla ve adımsız çalıştırır; en iyi duvar sürelerinin farkı bölü 300 adım
süresidir. Böylece açılış (yorumlayıcı, `import torch`), veri okuma ve metin üretimi iki tarafta da düşer.
TR'de iş parçacığı `GOMAXPROCS`, PyTorch'ta `torch.set_num_threads` ile verilir. Tek tek çalıştırmak:

```
GOMAXPROCS=2 tr examples/tr/36_karakter_modeli.tr
python tools/bench/py/char_model.py 300 2
```

**İlk ölçüm `run.py` ile (2026-10-23, dizüstü, 2 iş parçacığı, makine yüklü, 1 tekrar):** TR 13,8,
PyTorch 2.14 15,2 ms/adım (TR/PyTorch 0,91).

**Colab CPU (2026-10-23, AMD EPYC 7B12, 4 fiziksel / 8 sanal çekirdek, Python 3.13.16, PyTorch 2.14.1,
5 tekrarın en iyisi; `09f0cb21`):**

| İş parçacığı | TR ms/adım | PyTorch ms/adım | TR/PyTorch |
| --: | --: | --: | --: |
| 1 | 19,9 | 21,3 | 0,93 |
| 2 | 16,3 | 15,2 | 1,08 |
| 4 | 13,3 | 15,3 | 0,87 |
| 8 | 14,8 | 14,3 | 1,03 |

Colab'ın hazır PyTorch'u (2.11) ile oranlar aynıydı (0,94 / 1,07 / 0,87 / 1,01). 8 iş parçacığı 4'ten
hızlı değildir: sanal çekirdek çiftleri bir fiziksel çekirdeği paylaşır ve bu boyda model için eşgüdüm
yükü kazancı aşar.

## CPU temel ölçümü

[`cpu_baseline.sh`](cpu_baseline.sh) karakter modelini (1, 2, 4, 8 iş parçacığı), dil kıyasını ve tensör
kernellerini (`BenchmarkSpeed*`) 5'er tekrarla ölçer; `tools/record` ile sarılır, özet `metrics.json`'a
(sürümler dahil) yazılır. Colab'da hücre F ile koşar ([`tools/gpu/COLAB.md`](../gpu/COLAB.md) §0).

**Güncel ölçüm (18.10.2026, 2 çekirdek, 100 adım, `GOAMD64=v3`):** TR ~13,5 ms/adım, Python sürümü
~10–13,5 ms/adım. Aynı gün tekrar ölçüldü (dizüstü bilgisayar; makine başka işlerle paylaşılıyordu):
TR en iyi 13,5, çoğunlukla 13,5–16 ms/adım, yükün arttığı anlarda 18–22 ms/adım; Python sürümü
12–13,5 ms/adım. Yüklü makinede farklar büyür; karşılaştırmayı boş makinede yineleyin.

**Birleşik dikkat (`TR_fn_attention(...)`) sonrası (18.10.2026, 2 çekirdek, 300 adım, `GOAMD64=v3`, makine
yüklü):** örnek elle kurulan maske ve softmax yerine `TR_fn_attention(q, k, v)` kullanır; kayıp eğrisi önceki
sürümle 4 basamağa kadar aynıdır. Aynı veriyle art arda çalıştırılan çiftlerde TR önce/sonra 26,8/21,6,
24,1/19,8, 31,7/27,3, 29,0/19,8 ms/adım (en iyi: 20,6 → 18,8); Python sürümü 20,8 ms/adım. Yalnızca
dikkatin ileri+geri süresi bu şekilde (toplu 16, 4 bakış, T 64, dh 16) 3,8 ms'den 1,8 ms'ye iner
(`go test ./internal/compute/tensor -run XXX -bench KarakterModeli`).

Geçmiş ölçümler (aynı gün, 2 çekirdek): ilk sürüm TR 161 ms/adım (sürenin %69'u matris çarpımında);
vektör komutlu (AVX2/FMA) matris çarpımı ve float32 üstel/tanh sonrası, makine yüklüyken TR 22–31 ms/adım.
