# 3. Nokta (`.`) ile Zincirleme Metodlar

- **Metin (`TR_String`):** `.TR_length`, `.TR_upper()` (her dilde aynı basit eşleme, dil tanımı 13.2), `.TR_lower()`, `.TR_strip()`, `.TR_split(ayırıcı)`, `.TR_replace(eski, yeni)`, `.TR_contains(alt)`, `.TR_starts_with(önek)`, `.TR_ends_with(sonek)`, `.TR_reverse()`
- **Liste (`TR_List`):** `.TR_length`, `.TR_first`, `.TR_last`, `.TR_filter(fn)`, `.TR_map(fn)`, `.TR_reduce([başlangıç], fn)`, `.TR_sort()`, `.TR_reverse()`, `.TR_join([ayırıcı])`, `.TR_unique()`, `.TR_contains(el)`, `.TR_append(el, ...)` (listeyi yerinde değiştirir)
- **Sözlük (`TR_Dict`):** `.TR_length`, `.TR_keys()`, `.TR_values()`, `.TR_contains(anahtar)`

Her metodun ve yerleşiğin imzası ve örneği [`docs/codes.md`](../codes.md)'dedir. `.TR_length`, `.TR_first`, `.TR_last` özelliktir, parantezsiz yazılır.

**Metot mu, fonksiyon mu?** Bir değeri dönüştüren ya da ona soru soran iş metottur (`l.TR_length`, `l.TR_filter((x) => x > 2)`, `l.TR_sort()`, `l.TR_append(x)`, `t.TR_transpose()`); matematik ve tensör işi fonksiyondur (`TR_sum(l)`, `TR_mean(l)`, `TR_min(l)`, `TR_max(l)`, `TR_relu(t)`). Hedef tipin adını taşıyan dönüştürücüler (`TR_Int(x)`, `TR_Float(x)`, `TR_String(x)`) kurucudur, `TR_Type(x)` de genel bir sorudur; ikisi de fonksiyondur. Her işin tek biçimi vardır; öbür biçim yazılınca hata doğrusunu gösterir:

```text
'uzunluk(...)' TR'de bir fonksiyon değil; 'x.uzunluk' yazın
'filtrele(...)' TR'de bir fonksiyon değil; 'liste.filtrele(...)' yazın
'liste.toplam()' TR'de yok; 'toplam(liste)' yazın
```

- `l.TR_reduce([başlangıç], fn)` listeyi soldan sağa `fn(birikim, eleman)` ile tek değere indirger: `[1, 2, 3].TR_reduce(0, (a, x) => a + x)` → `6`. Başlangıç verilmezse ilk eleman başlangıçtır; boş listede başlangıç (yoksa `TR_none`) döner. Fonksiyon son argümandır ve iki parametrelidir (birikim, eleman): `[1, 2, 3].TR_reduce(1, (a, x) => a * x)` → `6`.
- `TR_mean(l)` boş listede ve sayı olmayan elemanda hata verir (`ortalama(): boş listenin ortalaması yoktur`), `TR_sum(l)` sayı olmayan elemanda hata verir.
- `l.TR_sort()` yalnızca sayılardan ya da yalnızca metinlerden oluşan listeyi sıralar; karışık liste hatadır (`liste.sırala(): Tam (2) ile Metin ("a") karşılaştırılamaz; liste yalnızca sayılardan ya da yalnızca metinlerden oluşmalıdır`). `TR_min` ve `TR_max` sayı olmayan değerde (metin dahil) hata, boş listede `TR_none` verir.
- Metotlar da yerleşikler gibi argüman sayısını ve tipini denetler: `metin.içerir() 1 argüman bekler, 0 verildi (kullanım: metin.içerir(alt))`, `liste.filtrele() 1. argüman olarak Fonksiyon bekler, Tam (3) verildi`.
- Yerleşikler ve metotlar yalnızca imzalarındaki adlı argümanları kabul eder; bilinmeyen bir adlı argüman sessizce yok sayılmaz: `yazdır() 'ayraç' adında bir parametre almaz (adlı argüman almaz)`, `Tensör() 'ölçek' adında bir parametre almaz (geçerli adlar: türev_izle, tür)`.

## Fonksiyon argümanı ve sondaki blok

Elemanla çalışan metotlara fonksiyon ok ile verilir, parantez her zaman yazılır (6.6): `(x) => x > 2`. Bir çağrının sonuna aynı satırda yazılan `{ ... }` bloğu ise parametresiz bir fonksiyondur ve çağrının son argümanı olur (6.7); değeri son ifadesidir, içinde `TR_return` yazılmaz: `TR_error_code { TR_print(1 / 0) }`. Blok argüman almaz; elemanla çalışan metoda blok verilirse (`l.TR_map { 1 }`) çalışma hatası olur, ileti oku önerir.

```tr
# TR 4
TR_var sayılar = [1, 2, 3, 4]
TR_var personel = [{"ad": "Ali"}, {"ad": "Ayşe"}]
sayılar.TR_filter((x) => x > 2).TR_map((x) => x * x)   # sayılar.filtrele((x) => x > 2)... ile aynı
personel.TR_map((x) => x["ad"])
TR_sum(sayılar.TR_filter((x) => x % 2 == 0))      # fonksiyon çağrıyla: toplam(sayılar.filtrele((x) => ...))
```

- İç içe fonksiyonlarda parametrelere ayrı ad verin: `[[1, 2]].TR_map((x) => x.TR_map((y) => y * 10))`.
- `TR_if`, `TR_for` ve `TR_match` başlıklarında `{` her zaman gövdeyi başlatır. Başlıkta sondaki bloklu bir çağrı paranteze alınır: `TR_if (TR_error_code { f() }) != TR_none { ... }`.
- Son argümandaki parametresiz ok (`f(() => x)`) sözdizimi hatasıdır, ileti sondaki bloğu önerir: `f { x }`.
- Metotlar `.` ile zincirlenir, fonksiyon çağrıyla verilir: `TR_sum(l.TR_filter((x) => x > 0))`. Başka dillerin boru işleci `|>` sözdizimi hatasıdır, ileti çağrıyı önerir (4.9).

## Hata mesajları

Hatalar konumuyla, kaynak satırıyla ve mümkünse bir öneriyle gösterilir:

```text
❌ Tip ve Güvenlik Denetimi Hataları:
  • Satır 3, Sütun 12: Tanımsız ad — 'toplm' adında bir değişken ya da fonksiyon yok. Bunu mu demek istediniz: 'toplam'?
     3 |     yazdır(toplm + 1)
       |            ^^^^^
```

Tanımsız adlar (yazım hatası, eski ya da Türkçe harfsiz yazılmış yerleşik: `'yazdir' TR'de yok; 'yazdır' yazmak mı istediniz?`) program çalışmadan önce, tip hatalarıyla ve yerleşik ad gölgeleme uyarılarıyla birlikte bildirilir; `TR_cmd_check` aynı denetimi yapar (dil tanımı 5.14).

Bir modülde (`TR_import`) oluşan hata, modülün adı ve satırıyla, modülün kendi kod çerçevesiyle yazılır (`• m2.tr, Satır 3: Sıfıra bölme hatası`); bulunamayan modül `❌ Modül hatası:` başlığıyla bildirilir (`"olmayan" modülü bulunamadı`). `TR_import` yolu yalnızca metin olarak alır; tırnaksız yazım sözdizimi hatasıdır ve doğrusunu söyler (`'aktar' modül yolunu metin olarak bekler, ancak 'yardımcı' adı bulundu: aktar "yardımcı" yazın`). `TR_cmd_check` programın aktardığı modülleri de (çalıştırmadan) aynı denetimden geçirir; modüldeki hata `❌ Tip ve Güvenlik Denetimi Hataları (lib/ic.tr modülünde):` gibi modülün adıyla yazılır.

Kendi tanımladığınız adlarda öneriler Türkçe harf farkını hata saymaz (`degisken_adi` yazılınca `değişken_adı` önerilir). Dilin kelimelerinde öneri her zaman asıl Türkçe adı gösterir (bkz. §2). Mesajlardaki ekler sayının okunuşuna göre seçilir (`Satır 6'ya`, `Satır 9'a`, `Satır 3'te`).
