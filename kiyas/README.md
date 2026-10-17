# Hız kıyas seti

Aynı işi yapan küçük programlar TR (`tr/`), Python (`py/kiyas.py`) ve Go (`go/kiyas.go`) ile
yazılmıştır. Her program kendi ölçtüğü süreyi (ms) ve bir sonuç değeri yazar; sonuç değerlerinin
üç dilde aynı olması gerekir. Yorumlayıcıda yapılan her hız değişikliğinden sonra yeniden ölçün.

```
python kiyas/kos.py                 # tüm testler, üç dil, üçer tekrar (en iyi süre)
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
| `matris_yerlesik` | 512×512 matrisle iki çarpım, yerleşik `@` ile (Python sürümü numpy ister) |

Süreler makineye ve o anki yüke göre değişir; karşılaştırmaları aynı makinede, bilgisayar
boştayken yapın.
