# 6. Fonksiyonlar

**6.1** `fonksiyon ad(parametreler) { ... }` bir fonksiyon tanımlar ve adı bulunulan kapsama bağlar. Tanım çalıştırıldığı noktada gerçekleşir; öne alma (hoisting) yoktur, fonksiyon tanımından önce çağrılamaz.

**6.2** `TR_kw_return ifade` fonksiyondan değer döndürür; yalnız `TR_kw_return` `TR_kw_none` döndürür. `TR_kw_return` hiç çalışmazsa son deyimin değeri döner. Program dosyasının en dışındaki `TR_kw_return` programı bitirir: sonraki satırlar çalışmaz, ertelenen işler çalışır (9.3), değer programın çıkış kodu olur (`TR_kw_return 3` → 3, değersiz `TR_kw_return` → 0; kod 1.11'deki gibi 0–125 arasında bir `TR_type_Int`'tir). Test gövdesinde (12.6) derleme hatasıdır; modülün en dışında deyim bulunmaz (11.1).
- *Durum:* en dıştaki `TR_kw_return`'ün değeri karar (Ahmet, 2026-10-27; C#'ın en dıştaki `return`'ü ve Rust'ın `main`'den döndürdüğü `ExitCode` gibi, 1.10). Bugün çıkış kodu her zaman 0, değer sessizce yok sayılıyor.

**6.3** Parametrelere varsayılan değer verilebilir (`ek = "!"`). Varsayılan, argümanı verilmediğinde her çağrıda yeniden, soldan sağa hesaplanır; kendisinden önceki parametreleri ve fonksiyonun tanımlandığı kapsamı görür (`fonksiyon f(a, b = a * 2)` ile `f(1)` → `b = 2`). Parametrenin kendisi ve sonraki parametreler henüz tanımlı değildir; varsayılanda adları geçerse dıştaki aynı adlı değişken kullanılmaz, çağrı `'b' parametresi henüz tanımlanmadı` hatası verir. Varsayılanı olmayan bir parametre için argüman verilmezse, fazla argüman verilirse ya da bilinmeyen bir adla argüman verilirse çağrı satırında hata oluşur (`'topla' fonksiyonu 2 argüman bekler, 1 verildi (eksik: 'b')`). Adlı argümanlar varken eksik zorunlu parametre adıyla söylenir (`fonksiyon f(a, b = 10)` ile `f(b: 20)` → `'f' fonksiyonu için zorunlu 'a' argümanı verilmedi`). Yerleşiklerin ve metotların geri çağırdığı fonksiyonlar (ör. `l.filtrele(f)` içindeki `f`, web işleyicileri) bu denetime girmez.

**6.3a** İç içe fonksiyon çağrısı derinliği 10000 ile sınırlıdır; aşılırsa `dene/yakala` ile yakalanabilen "Özyineleme derinliği sınırı aşıldı" hatası oluşur.

**6.4** Argümanlar adla da verilebilir: `böl(bölen: 2, bölünen: 10)`. Konumlu argümanlar önce yerleşir. Konumlu argümanın doldurduğu bir parametre adıyla da verilirse çağrı hata verir; adlı değer sessizce yok sayılmaz (`f(1, a: 2)` → `'f' fonksiyonu 'a' parametresi hem sırayla (1. argüman) hem adıyla verildi`).

**6.5** Fonksiyonlar tanımlandıkları ortamı yakalar (kapanış); özyineleme desteklenir.

```tr
fonksiyon sayaç_yap() {
    değişken n = 0
    sonuç fonksiyon() { n += 1; sonuç n }
}
```

**6.6** Ok fonksiyonları: `x => ifade`, `(a, b) => ifade` ya da `(x) => { blok }`; adsız `fonksiyon(x) { ... }` de bir değerdir.

**6.7** Sondaki blok: bir çağrının, üye erişiminin ya da adın hemen ardından **aynı satırda** yazılan `{ ... }`, tek parametresi `o` olan bir fonksiyondur ve çağrının son argümanı olur: `l.filtrele { o > 2 }` ≡ `l.filtrele((o) => o > 2)`. İç içe bloklarda `o` en içteki elemanı gösterir. `liste.indirge`'nin bloğu iki parametrelidir: `birikim` o ana kadarki sonuç, `o` sıradaki elemandır (`l.indirge(0) { birikim + o }` ≡ `l.indirge(0, (birikim, o) => birikim + o)`); eski `o[0]`/`o[1]` biçimi (o, `[birikim, eleman]` listesiyken) sözdizimi hatasıdır ve `birikim` ile `o` yazmayı önerir. `eğer`, `iken`, `döngü` ve `eşle` başlıklarında `{` her zaman gövdeyi başlatır; orada sondaki blok için ifade paranteze alınır.

```tr
değişken sayılar = [1, 2, 3, 4]
sayılar.filtrele { o > 2 }.haritala { o * o }
eğer (sayılar.filtrele { o > 2 }).uzunluk > 0 { yazdır("2'den büyük var") }
```

**6.8** Dönüş tipi `: Tip` ya da `-> Tip` ile bildirilir; tip denetleyicisi `sonuç` değerlerini buna göre denetler.

```tr
fonksiyon topla(a: Tam, b: Tam): Tam { sonuç a + b }
```
