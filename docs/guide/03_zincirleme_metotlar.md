# 3. Nokta (`.`) ile Zincirleme Metodlar

- **Metin (`TR_type_String`):** `.TR_mem_length`, `.TR_mem_upper()` (tam Türkçe `i -> İ` uyumlu), `.TR_mem_lower()`, `.TR_mem_strip()`, `.TR_mem_split(ayırıcı)`, `.TR_mem_replace(eski, yeni)`, `.TR_mem_contains(alt)`, `.TR_mem_starts_with(önek)`, `.TR_mem_ends_with(sonek)`, `.TR_mem_reverse()`
- **Liste (`TR_type_List`):** `.TR_mem_length`, `.TR_mem_first`, `.TR_mem_last`, `.TR_mem_filter(fn)`, `.TR_mem_map(fn)`, `.TR_mem_reduce([başlangıç], fn)`, `.TR_mem_sort()`, `.TR_mem_reverse()`, `.TR_mem_join([ayırıcı])`, `.TR_mem_unique()`, `.TR_mem_contains(el)`, `.TR_mem_append(el, ...)` (listeyi yerinde değiştirir)
- **Sözlük (`TR_type_Dict`):** `.TR_mem_length`, `.TR_mem_keys()`, `.TR_mem_values()`, `.TR_mem_contains(anahtar)`

Her metodun ve yerleşiğin imzası ve örneği [`docs/codes.md`](../codes.md)'dedir. `.TR_mem_length`, `.TR_mem_first`, `.TR_mem_last` özelliktir, parantezsiz yazılır.

**Metot mu, fonksiyon mu?** Bir değeri dönüştüren ya da ona soru soran iş metottur (`l.TR_mem_length`, `l.TR_mem_filter {...}`, `l.TR_mem_sort()`, `l.TR_mem_append(x)`, `t.TR_mem_transpose()`); matematik ve tensör işi fonksiyondur (`TR_fn_sum(l)`, `TR_fn_mean(l)`, `TR_fn_min(l)`, `TR_fn_max(l)`, `TR_fn_relu(t)`). Hedef tipin adını taşıyan dönüştürücüler (`TR_type_Int(x)`, `TR_type_Float(x)`, `TR_type_String(x)`) kurucudur, `TR_type_Type(x)` de genel bir sorudur; ikisi de fonksiyondur. Her işin tek biçimi vardır; öbür biçim yazılınca hata doğrusunu gösterir:

```text
'uzunluk(...)' TR'de bir fonksiyon değil; 'x.uzunluk' yazın
'filtrele(...)' TR'de bir fonksiyon değil; 'liste.filtrele(...)' yazın
'liste.toplam()' TR'de yok; 'toplam(liste)' yazın
```

- `l.TR_mem_reduce([başlangıç], fn)` listeyi soldan sağa `fn(birikim, eleman)` ile tek değere indirger: `[1, 2, 3].TR_mem_reduce(0, (a, x) => a + x)` → `6`. Başlangıç verilmezse ilk eleman başlangıçtır; boş listede başlangıç (yoksa `TR_kw_none`) döner. Fonksiyon son argümandır, böylece sondaki blokla da yazılır; blokta `TR_ctx_acc` önceki sonuç, `TR_ctx_it` sıradaki elemandır: `[1, 2, 3].TR_mem_reduce(1) { TR_ctx_acc * TR_ctx_it }` → `6`.
- `TR_fn_mean(l)` boş listede ve sayı olmayan elemanda hata verir (`ortalama(): boş listenin ortalaması yoktur`), `TR_fn_sum(l)` sayı olmayan elemanda hata verir.
- `l.TR_mem_sort()` yalnızca sayılardan ya da yalnızca metinlerden oluşan listeyi sıralar; karışık liste hatadır (`liste.sırala(): Tam (2) ile Metin ("a") karşılaştırılamaz; liste yalnızca sayılardan ya da yalnızca metinlerden oluşmalıdır`). `TR_fn_min` ve `TR_fn_max` sayı olmayan değerde (metin dahil) hata, boş listede `TR_kw_none` verir.
- Metotlar da yerleşikler gibi argüman sayısını ve tipini denetler: `metin.içerir() 1 argüman bekler, 0 verildi (kullanım: metin.içerir(alt))`, `liste.filtrele() 1. argüman olarak Fonksiyon bekler, Tam (3) verildi`.
- Yerleşikler ve metotlar yalnızca imzalarındaki adlı argümanları kabul eder; bilinmeyen bir adlı argüman sessizce yok sayılmaz: `yazdır() 'ayraç' adında bir parametre almaz (adlı argüman almaz)`, `Tensör() 'ölçek' adında bir parametre almaz (geçerli adlar: türev_izle, tür)`.

## `o` zamiri ve sondaki blok

Bir çağrının sonuna aynı satırda yazılan `{ ... }` bloğu, tek parametresi `TR_ctx_it` olan bir fonksiyondur. `TR_ctx_it` o anki elemanı gösterir:

```tr
# TR 4
TR_var sayılar = [1, 2, 3, 4]
TR_var personel = [{"ad": "Ali"}, {"ad": "Ayşe"}]
sayılar.TR_filter((x) => x > 2).TR_map((x) => x * x)   # sayılar.filtrele((x) => x > 2)... ile aynı
personel.TR_map((x) => x["ad"])
TR_sum(sayılar.TR_filter((x) => x % 2 == 0))      # fonksiyon çağrıyla: toplam(sayılar.filtrele {...})
```

- İç içe bloklarda `TR_ctx_it` en içteki elemanı gösterir: `[[1, 2]].TR_mem_map { TR_ctx_it.TR_mem_map { TR_ctx_it * 10 } }`.
- `TR_kw_if`, `TR_kw_for` ve `TR_kw_match` başlıklarında `{` her zaman gövdeyi başlatır. Başlıkta sondaki blok kullanmak için ifadeyi paranteze alın: `TR_kw_if (liste.TR_mem_filter { TR_ctx_it > 2 }).TR_mem_length > 0 { ... }`.
- Uzun yazım (`(x) => x > 2`) aynen çalışmaya devam eder.
- Metotlar `.` ile zincirlenir, fonksiyon çağrıyla verilir: `TR_fn_sum(l.TR_mem_filter { ... })`. Başka dillerin boru işleci `|>` sözdizimi hatasıdır, ileti çağrıyı önerir (4.9).

## Hata mesajları

Hatalar konumuyla, kaynak satırıyla ve mümkünse bir öneriyle gösterilir:

```text
❌ Tip ve Güvenlik Denetimi Hataları:
  • Satır 3, Sütun 12: Tanımsız ad — 'toplm' adında bir değişken ya da fonksiyon yok. Bunu mu demek istediniz: 'toplam'?
     3 |     yazdır(toplm + 1)
       |            ^^^^^
```

Tanımsız adlar (yazım hatası, eski ya da Türkçe harfsiz yazılmış yerleşik: `'yazdir' TR'de yok; 'yazdır' yazmak mı istediniz?`) program çalışmadan önce, tip hatalarıyla ve yerleşik ad gölgeleme uyarılarıyla birlikte bildirilir; `TR_cmd_check` aynı denetimi yapar (dil tanımı 5.14).

Bir modülde (`TR_kw_import`) oluşan hata, modülün adı ve satırıyla, modülün kendi kod çerçevesiyle yazılır (`• m2.tr, Satır 3: Sıfıra bölme hatası`); bulunamayan modül `❌ Modül hatası:` başlığıyla bildirilir (`"olmayan" modülü bulunamadı`). `TR_kw_import` yolu yalnızca metin olarak alır; tırnaksız yazım sözdizimi hatasıdır ve doğrusunu söyler (`'aktar' modül yolunu metin olarak bekler, ancak 'yardımcı' adı bulundu: aktar "yardımcı" yazın`). `TR_cmd_check` programın aktardığı modülleri de (çalıştırmadan) aynı denetimden geçirir; modüldeki hata `❌ Tip ve Güvenlik Denetimi Hataları (lib/ic.tr modülünde):` gibi modülün adıyla yazılır.

Kendi tanımladığınız adlarda öneriler Türkçe harf farkını hata saymaz (`degisken_adi` yazılınca `değişken_adı` önerilir). Dilin kelimelerinde öneri her zaman asıl Türkçe adı gösterir (bkz. §2). Mesajlardaki ekler sayının okunuşuna göre seçilir (`Satır 6'ya`, `Satır 9'a`, `Satır 3'te`).
