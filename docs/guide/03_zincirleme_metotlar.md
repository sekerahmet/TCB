# 3. Nokta (`.`) ile Zincirleme Metodlar

- **Metin (`Metin`):** `.uzunluk`, `.büyük_harf()` (tam Türkçe `i -> İ` uyumlu), `.küçük_harf()`, `.kırp()`, `.böl(ayırıcı)`, `.değiştir(eski, yeni)`, `.içerir(alt)`, `.başlar(önek)`, `.biter(sonek)`, `.ters_çevir()`
- **Liste (`Liste`):** `.uzunluk`, `.ilk`, `.son`, `.filtrele(fn)`, `.haritala(fn)`, `.indirge([başlangıç], fn)`, `.sırala()`, `.ters_çevir()`, `.birleştir([ayırıcı])`, `.benzersiz()`, `.içerir(el)`, `.ekle(el, ...)` (listeyi yerinde değiştirir)
- **Sözlük (`Sözlük`):** `.uzunluk`, `.anahtarlar()`, `.değerler()`, `.içerir(anahtar)`

Her metodun ve yerleşiğin imzası ve örneği [`docs/codes.md`](../codes.md)'dedir. `.uzunluk`, `.ilk`, `.son` özelliktir, parantezsiz yazılır.

**Metot mu, fonksiyon mu?** Bir değeri dönüştüren ya da ona soru soran iş metottur (`l.uzunluk`, `l.filtrele {...}`, `l.sırala()`, `l.ekle(x)`, `t.devrik()`); matematik ve tensör işi fonksiyondur (`toplam(l)`, `ortalama(l)`, `min(l)`, `max(l)`, `relu(t)`). Hedef tipin adını taşıyan dönüştürücüler (`tam(x)`, `ondalık(x)`, `metin(x)`) kurucudur, `tip(x)` de genel bir sorudur; ikisi de fonksiyondur. Her işin tek biçimi vardır; öbür biçim yazılınca hata doğrusunu gösterir:

```text
'uzunluk(...)' TR'de bir fonksiyon değil; 'x.uzunluk' yazın
'filtrele(...)' TR'de bir fonksiyon değil; 'liste.filtrele(...)' yazın
'liste.toplam()' TR'de yok; 'toplam(liste)' yazın
```

- `l.indirge([başlangıç], fn)` listeyi soldan sağa `fn(birikim, eleman)` ile tek değere indirger: `[1, 2, 3].indirge(0, (a, x) => a + x)` → `6`. Başlangıç verilmezse ilk eleman başlangıçtır; boş listede başlangıç (yoksa `yok`) döner. Fonksiyon son argümandır, böylece sondaki blokla da yazılır; blokta `birikim` önceki sonuç, `o` sıradaki elemandır: `[1, 2, 3].indirge(1) { birikim * o }` → `6`.
- `ortalama(l)` boş listede ve sayı olmayan elemanda hata verir (`ortalama(): boş listenin ortalaması yoktur`), `toplam(l)` sayı olmayan elemanda hata verir.
- `l.sırala()` yalnızca sayılardan ya da yalnızca metinlerden oluşan listeyi sıralar; karışık liste hatadır (`liste.sırala(): Tam (2) ile Metin ("a") karşılaştırılamaz; liste yalnızca sayılardan ya da yalnızca metinlerden oluşmalıdır`). `min` ve `max` sayı olmayan değerde (metin dahil) hata, boş listede `yok` verir.
- Metotlar da yerleşikler gibi argüman sayısını ve tipini denetler: `metin.içerir() 1 argüman bekler, 0 verildi (kullanım: metin.içerir(alt))`, `liste.filtrele() 1. argüman olarak Fonksiyon bekler, Tam (3) verildi`.
- Yerleşikler ve metotlar yalnızca imzalarındaki adlı argümanları kabul eder; bilinmeyen bir adlı argüman sessizce yok sayılmaz: `yazdır() 'ayraç' adında bir parametre almaz (adlı argüman almaz)`, `tensör() 'ölçek' adında bir parametre almaz (geçerli adlar: türev_izle, tür)`.

## `o` zamiri ve sondaki blok

Bir çağrının sonuna aynı satırda yazılan `{ ... }` bloğu, tek parametresi `o` olan bir fonksiyondur. `o` o anki elemanı gösterir:

```tr
değişken sayılar = [1, 2, 3, 4]
değişken personel = [{"ad": "Ali"}, {"ad": "Ayşe"}]
sayılar.filtrele { o > 2 }.haritala { o * o }   // sayılar.filtrele((x) => x > 2)... ile aynı
personel.haritala { o["ad"] }
sayılar.filtrele { o % 2 == 0 } |> toplam      // boru yalnızca fonksiyonlarla: toplam(sayılar.filtrele {...})
```

- İç içe bloklarda `o` en içteki elemanı gösterir: `[[1, 2]].haritala { o.haritala { o * 10 } }`.
- `eğer`, `iken`, `döngü` ve `eşle` başlıklarında `{` her zaman gövdeyi başlatır. Başlıkta sondaki blok kullanmak için ifadeyi paranteze alın: `eğer (liste.filtrele { o > 2 }).uzunluk > 0 { ... }`.
- Uzun yazım (`(x) => x > 2`) aynen çalışmaya devam eder.
- Boru hattı `x |> f(a)` ≡ `f(x, a)` yalnızca fonksiyonlarla çalışır (kullanıcı fonksiyonları ve `toplam`, `ortalama`, `relu`, `metin` gibi yerleşikler; tam liste `docs/codes.md` sonunda). Metotlar `.` ile zincirlenir; `l |> filtrele { ... }` yazılırsa hata `l.filtrele { ... }` yazmanızı söyler.

## Hata mesajları

Hatalar konumuyla, kaynak satırıyla ve mümkünse bir öneriyle gösterilir:

```text
❌ Tip ve Güvenlik Denetimi Hataları:
  • Satır 3, Sütun 12: Tanımsız ad — 'toplm' adında bir değişken ya da fonksiyon yok. Bunu mu demek istediniz: 'toplam'?
     3 |     yazdır(toplm + 1)
       |            ^^^^^
```

Tanımsız adlar (yazım hatası, eski ya da Türkçe harfsiz yazılmış yerleşik: `'yazdir' TR'de yok; 'yazdır' yazmak mı istediniz?`) program çalışmadan önce, tip hatalarıyla ve yerleşik ad gölgeleme uyarılarıyla birlikte bildirilir; `tr kontrol` aynı denetimi yapar (dil tanımı 5.14).

Bir modülde (`aktar`) oluşan hata, modülün adı ve satırıyla, modülün kendi kod çerçevesiyle yazılır (`• m2.tr, Satır 3: Sıfıra bölme hatası`); bulunamayan modül `❌ Modül hatası:` başlığıyla bildirilir (`"olmayan" modülü bulunamadı`). `aktar` yolu yalnızca metin olarak alır; tırnaksız yazım sözdizimi hatasıdır ve doğrusunu söyler (`'aktar' modül yolunu metin olarak bekler, ancak 'yardımcı' adı bulundu: aktar "yardımcı" yazın`). `tr kontrol` programın aktardığı modülleri de (çalıştırmadan) aynı denetimden geçirir; modüldeki hata `❌ Tip ve Güvenlik Denetimi Hataları (lib/ic.tr modülünde):` gibi modülün adıyla yazılır.

Kendi tanımladığınız adlarda öneriler Türkçe harf farkını hata saymaz (`degisken_adi` yazılınca `değişken_adı` önerilir). Dilin kelimelerinde öneri her zaman asıl Türkçe adı gösterir (bkz. §2). Mesajlardaki ekler sayının okunuşuna göre seçilir (`Satır 6'ya`, `Satır 9'a`, `Satır 3'te`).
