# 6. Fonksiyonlar

**6.1** `fonksiyon ad(parametreler) { ... }` bir fonksiyon tanımlar ve adı bulunulan kapsama bağlar. Tanım çalıştırıldığı noktada gerçekleşir; öne alma (hoisting) yoktur, fonksiyon tanımından önce çağrılamaz.

**6.2** `sonuç ifade` fonksiyondan değer döndürür; yalnız `sonuç` `yok` döndürür. `sonuç` hiç çalışmazsa son deyimin değeri döner. Programın üst düzeyindeki `sonuç` programı bitirir (sonraki satırlar çalışmaz, çıkış kodu 0'dır); bir modülün üst düzeyinde (11.4) ve test gövdesinde (12.6) ise derleme hatasıdır.

**6.3** Parametrelere varsayılan değer verilebilir (`ek = "!"`). Varsayılan, argümanı verilmediğinde her çağrıda yeniden, soldan sağa hesaplanır; kendisinden önceki parametreleri ve fonksiyonun tanımlandığı kapsamı görür (`fonksiyon f(a, b = a * 2)` ile `f(1)` → `b = 2`). Parametrenin kendisi ve sonraki parametreler henüz tanımlı değildir; varsayılanda adları geçerse dıştaki aynı adlı değişken kullanılmaz, çağrı `'b' parametresi henüz tanımlanmadı` hatası verir. Varsayılanı olmayan bir parametre için argüman verilmezse, fazla argüman verilirse ya da bilinmeyen bir adla argüman verilirse çağrı satırında hata oluşur (`'topla' fonksiyonu 2 argüman bekler, 1 verildi (eksik: 'b')`). Yerleşiklerin ve metotların geri çağırdığı fonksiyonlar (ör. `l.filtrele(f)` içindeki `f`, web işleyicileri) bu denetime girmez.

**6.3a** İç içe fonksiyon çağrısı derinliği 10000 ile sınırlıdır; aşılırsa `dene/yakala` ile yakalanabilen "Özyineleme derinliği sınırı aşıldı" hatası oluşur.

**6.4** Argümanlar adla da verilebilir: `böl(bölen: 2, bölünen: 10)`. Konumlu argümanlar önce yerleşir.

**6.5** Fonksiyonlar tanımlandıkları ortamı yakalar (kapanış); özyineleme desteklenir.

```tr
fonksiyon sayaç_yap() {
    değişken n = 0
    sonuç fonksiyon() { n += 1; sonuç n }
}
```

**6.6** Ok fonksiyonları: `x => ifade`, `(a, b) => ifade` ya da `(x) => { blok }`; adsız `fonksiyon(x) { ... }` de bir değerdir.

**6.7** Sondaki blok: bir çağrının, üye erişiminin ya da adın hemen ardından **aynı satırda** yazılan `{ ... }`, tek parametresi `o` olan bir fonksiyondur ve çağrının son argümanı olur: `l.filtrele { o > 2 }` ≡ `l.filtrele((o) => o > 2)`. İç içe bloklarda `o` en içteki elemanı gösterir. `eğer`, `iken`, `döngü` ve `eşle` başlıklarında `{` her zaman gövdeyi başlatır; orada sondaki blok için ifade paranteze alınır.

```tr
değişken sayılar = [1, 2, 3, 4]
sayılar.filtrele { o > 2 }.haritala { o * o }
eğer (sayılar.filtrele { o > 2 }).uzunluk > 0 { yazdır("2'den büyük var") }
```

**6.8** Dönüş tipi `: Tip` ya da `-> Tip` ile bildirilir; tip denetleyicisi `sonuç` değerlerini buna göre denetler.

```tr
fonksiyon topla(a: Tam, b: Tam): Tam { sonuç a + b }
```
