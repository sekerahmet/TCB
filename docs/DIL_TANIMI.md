# TR Dil Tanımı

Bu belge TR dilinin **bugünkü uygulamaya** (`pkg/lexer`, `pkg/parser`, `pkg/checker`, `pkg/vm`) dayanan kısa tanımıdır. Dilek değil, davranış anlatır; bilinen eksikler §14'te açıkça listelenmiştir. Dili örneklerle tanımak için önce `ornekler/02_dil_turu.tr`'yi çalıştırın (sıralı örnek listesi: `README.md`, "Örnekler").

**Kural ve test eşlemesi.** Her kuralın bir numarası vardır (ör. **4.2**). `testler/` klasöründeki testlerin adı bu numarayla başlar (`test "4.2 aritmetik: ..."`). `tr test testler` komutu tanımdaki her kuralı çalıştırarak denetler; `go test ./pkg/testkosucu` de tanımda numarası geçip testi olmayan bir kural kalmadığını doğrular (Faz 3 kapısı: *her kural bir testte*). Numara harfli olabilir (**2.9a**); testin numarası kuralınkiyle harfi dahil aynıdır, bir kuralın birden çok testi aynı numarayı taşır. Ayrıştırıcının ve tip denetleyicisinin reddettiği kodlar (programın hiç çalışmaması beklenen durumlar) `testler/olumsuz/` altındaki bilerek hatalı modüllerle sınanır: test modülü `aktar` ile yüklemeyi dener ve beklenen hatayı doğrular (11.4). Bu fikstürler bilerek hatalıdır; `tr kontrol` ve `tr biçimle` onları hatalı bulur.

İçindekiler: [2. Sözcük yapısı](#2-sözcük-yapısı) · [3. Tipler](#3-tipler-ve-değerler) · [4. Operatörler](#4-operatörler) · [5. Deyimler](#5-deyimler-ve-kontrol-yapıları) · [6. Fonksiyonlar](#6-fonksiyonlar) · [7. Yapılar](#7-yapılar) · [8. Hatalar](#8-hata-yönetimi) · [9. ertele](#9-ertele) · [10. Eşzamanlılık](#10-eşzamanlılık-ve-yorumlayıcı-kilidi) · [11. Modüller](#11-modüller) · [12. Testler](#12-testler) · [13. Yerleşikler](#13-yerleşik-fonksiyonlar-ve-metotlar) · [14. Sınırlamalar](#14-bilinen-sınırlamalar) · [15. Tensörler](#15-tensörler-ve-otomatik-türev) · [16. Veri ve parçalayıcı](#16-veri-ve-parçalayıcı) · [17. Sözlük ve ad kodları](#17-sözlük-ve-ad-kodları)

---

## 1. Program

Bir TR programı, yukarıdan aşağıya çalışan deyimlerden oluşan tek bir `.tr` dosyasıdır; `main` gibi bir giriş noktası yoktur. Çalıştırmadan önce dosya ayrıştırılır ve tip denetleyiciden geçer; ikisinden birinde hata varsa hiçbir satır çalışmaz. Aynı kural `aktar` ile alınan her modül için de geçerlidir (11.4).

```tr
yazdır("Merhaba, dünya!")
```

---

## 2. Sözcük yapısı

**2.1** Kaynak UTF-8 metindir; bazı Windows editörlerinin eklediği baştaki BOM (U+FEFF) yok sayılır. Konumlar 1'den başlayan satır ve karakter (bayt değil) sütunlarıyla bildirilir.

**2.2** Yorumlar iki biçimdedir: satır sonuna kadar `//` ve çok satırlı `/* ... */`. Kapanmayan `/*` bir hatadır. `#` yorum değildir: metin dışında yazılan `#` bir sözdizimi hatasıdır (`'#' TR'de yorum değil; yorum için '//' yazın`); metin içinde (`"#22c55e"`) sıradan bir karakterdir.

```tr
değişken x = 1  // satır sonuna kadar yorum
/* çok
   satırlı */
```

**2.3** Tanımlayıcılar bir harf ya da `_` ile başlar, harf, rakam ve `_` ile sürer; Türkçe harfler (ç ğ ı İ ö ş ü) dahil tüm Unicode harfleri geçerlidir: `öğrenci_sayısı2`.

**2.4** Tam sayı sabitleri ondalık rakamlardan oluşur; okunabilirlik için `_` kullanılabilir: `1_000_000`.

**2.5** Ondalık sabitler nokta içerir ve noktadan sonra rakam gelmelidir: `3.14`. (`1..5` bir aralıktır, ondalık değil.) Bilimsel yazım da Ondalık verir: üs harfi `e`/`E`, isteğe bağlı `+`/`-` ve en az bir rakam (`1e-5`, `3.2E+4`, `2e10`); üsten sonra rakam yoksa `e` sayıya ait değildir (`2e` → `2` ve `e` adı).

**2.6** Metinler çift (`"..."`) ya da tek (`'...'`) tırnakla yazılır; `\n \t \r \" \' \\` kaçış dizileri tanınır, bilinmeyen kaçışta ters bölü atılır. Metin birden çok satıra yayılabilir; metnin içindeki satır sonu kaynağın satır sonu biçiminden bağımsız olarak `\n`dir (CRLF'li bir dosyada metnin değerine `\r` girmez; `\r` yalnızca kaçışla yazılır). `tr biçimle` metinlerin içindekiler dahil bütün satır sonlarını LF yazar.

**2.7** Metin içindeki `{ifade}` yerleştirmedir: ifade hesaplanıp metne yazılır. Yerleştirme aynı satırda kapanmalıdır; düz süslü parantez için `\{` ve `\}` yazılır. Ayrıştırılamayan `{...}` olduğu gibi metinde kalır.

```tr
değişken ad = "Ali"
yazdır("Merhaba {ad}, 1 + 2 = {1 + 2}, kişi: {p['ad']}, düz: \{x\}")
```

**2.8** Deyimler satır sonuyla ya da isteğe bağlı `;` ile ayrılır. Satır sonu iki yerde anlam taşır: yeni satırda başlayan `(` ya da `[` önceki ifadeyi sürdürmez, yeni bir deyim başlatır; sondaki blok `{` (bkz. 6.7) önceki ifadeyle aynı satırda olmalıdır. Satır sonunda kalan ikili operatörden sonra ifade alt satırda sürer.

```tr
değişken f = 5
(1 + 2) |> yazdır        // ayrı deyim; '5(1 + 2)' çağrısı değil
değişken t = 1 +
    2                    // t = 3
```

**2.9** Dilin kelimelerinin yalnızca Türkçe yazımı geçerlidir. Anahtar kelimeler şunlardır:

| Görev | Anahtar kelimeler |
| :-- | :-- |
| Bildirim | `değişken`, `sabit` |
| Fonksiyon | `fonksiyon`, `sonuç` |
| Koşul | `eğer`, `değilse`, `eşle`, `varsayılan` |
| Döngü | `döngü`, `içinde`, `iken`, `çık`, `devam` |
| Değer ve mantık | `doğru`, `yanlış`, `yok`, `ve`, `veya`, `değil` |
| Yapı | `yapı`, `kendi` |
| Hata yönetimi | `dene`, `yakala`, `fırlat` |
| Modül ve eşzamanlılık | `aktar`, `eşzamanlı`, `ertele` |
| Hata ayıklama | `kesme_noktası` |

Bu kural anahtar kelimelerle sınırlı değildir: yerleşik fonksiyonlar, metot ve özellikler, adlı argümanlar ve tip adları da (bkz. 3.13) yalnızca Türkçe harfleriyle yazıldıkları biçimde geçerlidir. Türkçe harfleri `c g i o s u` ile yazılmış bir biçim (`degisken`, `sonuc`, `dogru`, `yazdir`, `buyuk_harf`) dilin kelimesi değil, sıradan bir addır: `değişken degisken = 1` geçerlidir. Böyle bir ad tanımsızken kullanılırsa hata mesajı asıl Türkçe adı önerir:

- anahtar kelime: `'degisken' TR'de anahtar kelime değil; 'değişken' yazmak mı istediniz?`
- yerleşik fonksiyon ya da metot: `'yazdir' TR'de yok; 'yazdır' yazmak mı istediniz?`, `'buyuk_harf' TR'de yok; 'büyük_harf' yazmak mı istediniz?`
- adlı argüman: `AdamW 'ogrenme_orani' adında bir parametre almaz. Bunu mu demek istediniz: 'öğrenme_oranı'?`

**2.9a** Her işin tek yazımı vardır: her anahtar kelimenin, yerleşik fonksiyonun (bkz. 13), metodun, özelliğin, adlı argümanın ve tip adının yalnızca bir adı vardır; eş anlamlı ya da başka yazımlı ikinci bir ad kabul edilmez. Kaldırılan adlar artık sıradan adlardır (`değişken sınıf = "9-A"`, `değişken boy = 180`); eski alışkanlıkla yazıldıklarında (tanımsız ad, bilinmeyen metot ya da bilinmeyen adlı argüman hatasında) hata mesajı asıl adı önerir (`'uyut' TR'de yok; 'bekle' yazmak mı istediniz?`). İpuçları her zaman asıl Türkçe adı önerir, kaldırılan bir adın Türkçe harfsiz yazımı için de (`islev` → `fonksiyon`, `dondur` → `sonuç`). Kaldırılanlar (kaldırılan → asıl):

- Anahtar kelimeler: `işlev` → `fonksiyon`, `döndür` → `sonuç`, `kır` → `çık`, `sınıf` → `yapı`, `aktar ... as` → `aktar ... olarak`, `dur_noktası` / `durdurma_noktası` → `kesme_noktası`.
- Yerleşik fonksiyonlar: `yaz` → `yazdır`, `oku` → `girdi`, `tur` → `tip`, `en_büyük` → `maks`, `en_küçük` → `min`, `uyut` → `bekle`, `std_sapma` → `standart_sapma`, `medyan` → `ortanca`, `json_yaz` → `json_metin`, `tablo_oku` → `csv_oku`, `http_al` → `web_al`, `web_sunucusu` → `web_sunucu`, `veritabanı` → `veritabanı_aç`, `pencere_oluştur` → `arayüz_oluştur`, `matris` → `tensör` (matris 2 boyutlu tensördür: `tensör([[1, 2], [3, 4]])`), `rastgele_matris` → `rastgele_normal` (`rastgele_normal([satır, sütun], tohum: 7)`).
- Metot ve özellikler: `.boy` → `.uzunluk`, `.dönüştür` / `.donustur` → `.haritala`, `.türev` → `.gradyan`, `.tahmin` → `.tahmin_et`; veritabanında `.kayıt_ekle` → `.ekle`, `.sorgu` / `.seç` → `.sorgula`, `.tablo_al` → `.hepsini_al`; web sunucusu ve arayüzde `.göster` → `.başlat`, `.kapat` → `.durdur`, `.url` → `.adres`, `.get` → `.al`, `.post` → `.gönder`, `.rota` → `.rota_ekle`, `.metrik_ekle` → `.kart_ekle`, `.bilgi_ekle` → `.metin_ekle`.
- Biçim değiştirenler (13.6): `uzunluk(x)` (ve `boy(x)`) → `x.uzunluk`, `filtrele(l, f)` → `l.filtrele(f)`, `haritala(l, f)` (ve `donustur`) → `l.haritala(f)`, `indirge(l, f, b)` (ve `ozetle`) → `l.indirge(b, f)`, `sırala(l)` → `l.sırala()`, `ekle(l, x)` → `l.ekle(x)`, `devrik(t)` → `t.devrik()`; `l.toplam()` → `toplam(l)`, `l.ortalama()` → `ortalama(l)`. Bu adlar yazılınca hata yeni biçimi gösterir: `'uzunluk(...)' TR'de bir fonksiyon değil; 'x.uzunluk' yazın`, `'liste.toplam()' TR'de yok; 'toplam(liste)' yazın`.
- Adlı argümanlar: `nerede:` → `koşul:`, `params:` → `parametreler:`, `sinir_ağı(...)` modelinin `.eğit(...)` metodunda `öğrenme_hızı:` → `öğrenme_oranı:` (`AdamW` ve `SGD` ile aynı ad).
- Tip adları (bkz. 3.13): `tamsayı` / `int` → `Tam`, `sayı` / `float` → `Ondalık`, `yazı` / `str` / `string` → `Metin`, `mantıksal` / `bool` → `Mantık`, `boş` / `null` → `Yok`, `dizi` / `list` → `Liste`, `harita` / `dict` / `map` → `Sözlük`, `matris` / `matrix` → `Tensör`.
- Yorum: `#` → `//` (bkz. 2.2).
- Motorların metin içindeki sözcükleri de yalnızca Türkçedir: veritabanı sorgu ve koşul dili (`SELECT` → `SEÇ`, `WHERE` → `NEREDE`, `AND` → `VE` ..., bkz. `KULLANIM_KILAVUZU.md` §7), bellekte veritabanı yolu (`":memory:"` → `":bellek:"`), sıralama yönü (`"maas:desc"` → `"maas:azalan"`) ve arayüz renk adları (`green` → `yeşil`, `yesil` → `yeşil`; bilinmeyen renk adı da hatadır). Komut satırı komutlarının da tek Türkçe adı vardır (`tr debug` → `tr hata_ayıkla`, bkz. `KULLANIM_KILAVUZU.md` §1).

Kaldırılan dil adları (anahtar kelimeler, yerleşikler, üyeler, adlı argümanlar, tip adları) kod listesinde asıl adın `eski` listesinde, biçim değiştiren kodlar ise kendi satırlarında "taşındı" bilgisiyle durur (bkz. 17); ipucu oradan kendiliğinden çalışır.

**Yerleşik ad gölgeleme uyarısı.** Bir yerleşiğin adı (`toplam`, `girdi`, `metin`, `tip` ...) değişken, fonksiyon, parametre, döngü değişkeni, `yakala` değişkeni ya da yapı adı olarak tanımlanabilir; o kapsamda ad kullanıcının tanımını gösterir ve yerleşik kullanılamaz. Tip denetleyicisi bunun için bir **uyarı** verir; uyarı hata değildir, program çalışır ve çıkış kodu değişmez. Uyarılar standart hataya, tip hatalarından ayrı, `⚠️ Uyarılar:` başlığıyla yazılır: `Satır 1: 'toplam' yerleşik bir ad; bu kapsamda yerleşik 'toplam' artık kullanılamaz`. Gölgelenmiş ad yerleşik gibi çağrılırsa çalışma zamanı hatası nedenini söyler: `Satır 2: 'tip' bir fonksiyon değil (Tam); 1. satırda bir değişkene verildi, yerleşik 'tip' bu kapsamda gölgelendi`. Yapı alanları ve metotları (`kendi.toplam`, `p.toplam()`) gölgeleme sayılmaz; metot adları (`uzunluk`, `sırala`, `ekle` ...) yerleşik değildir, değişken adı olarak serbestçe kullanılır.

**2.10** İfadeler ve bloklar en fazla 1000 düzey iç içe yazılabilir (`((((1))))` dört düzeydir); sınır aşılırsa program çalışmaz, `ifade ya da blok çok derin iç içe (en fazla 1000 düzey)` sözdizimi hatası verilir. (Sınırsız iç içelik yorumlayıcıyı yakalanamaz biçimde çökertirdi.)

Bağlamsal sözcükler anahtar kelime değildir ve ad olarak kullanılabilir: `test` (yalnızca deyim başında ardından metin gelince, bkz. 12.2), `olarak` (yalnızca `aktar` içinde), `kurucu` (yapının oluşturulurken çağrılan metodunun adı, 7.4), `hata` (`yakala`da ad verilmeyince hata değişkeninin adı, 8.1), `o` (sondaki bloğun parametresi, 6.7), `_` (yalnızca `eşle` kolunda varsayılan). Bunların adları da kod listesindedir (`A1xx`, bkz. 17).

---

## 3. Tipler ve değerler

TR dinamik tiplidir; değerler tiplerini taşır. İsteğe bağlı tip açıklamalarını yalnızca çalıştırma öncesi tip denetleyicisi kullanır (3.13).

**3.1** `Tam`: 64 bit işaretli tam sayı. `+ - * / **` işlemleri ve tekli `-` sınırı aşarsa (ör. en küçük değerin `/ -1`'i ya da eksisi) sessizce sarmak yerine "Tam sayı taşması" hatası verir. Yerleşikler de aynı kurala uyar: `toplam`, `mutlak` (en küçük değerde), `tam` ve `yuvarla` (Ondalık'tan çevirirken `NaN`, sonsuz ya da Tam aralığı dışındaki değerde) taşma hatası verir.

**3.2** `Ondalık`: 64 bit kayan noktalı sayı; en kısa gösterimiyle yazdırılır, tam sayılı ondalıklar `1.0` biçimindedir (`123456.0`). Büyüklüğü 1e6 ve üstü ya da 1e-4'ten küçük olanlar bilimsel gösterimle yazdırılır (`1e+06`, `1.6777216e+07`, `1e-05`; `0.0001` ise olduğu gibi). Sonsuz ve tanımsız değerler `+Sonsuz`, `-Sonsuz`, `NaN` olarak yazdırılır (tensör yazdırması bunları `Sonsuz`, `-Sonsuz`, `Tanımsız` olarak gösterir).

**3.3** `Metin`: değiştirilemez Unicode metni; uzunluk ve indeks karakter (rune) bazındadır, negatif indeks sondan sayar, sınır dışı indeks `yok` verir, `m[a..b]` alt metin verir. Dilimde de negatif uç indeksteki gibi sondan sayar (`"şeker"[-3..5]` → `"ker"`, `"şeker"[1..=-1]` → `"eker"`); uçlar metnin sınırlarına kırpılır, başlangıç bitişten büyükse sonuç boştur.

```tr
"şeker".uzunluk   // 5
"şeker"[-1]        // "r"
"şeker"[1..3]      // "ek"
```

**3.4** `Mantık`: `doğru` ve `yanlış`.

**3.5** `yok`: değerin yokluğu; değer döndürmeyen fonksiyonlar ve bulunamayan indeksler `yok` verir.

**3.6** `Liste`: sıralı, karışık tipli, değiştirilebilir dizi: `[1, "iki", 3.0]`. Negatif indeks sondan sayar, okuma sırasında sınır dışı indeks `yok` verir (yazmada hatadır), `l[a..b]` dilim verir (dilim uçları Metin'deki gibidir, 3.3: `[1, 2, 3, 4][-2..4]` → `[3, 4]`). Listeler, sözlükler ve nesneler **başvuruyla** paylaşılır: `b = a` aynı listeyi gösterir.

**3.7** `Sözlük`: eklenme sırasını koruyan anahtar–değer tablosu. Çıplak ad anahtarı metindir (`{ad: "Ali"}` ≡ `{"ad": "Ali"}`); metin olmayan anahtarlar da geçerlidir (`{1: "bir"}`). Sayı anahtarları sayı eşitliğine uyar (4.5): `d[1]` ile `d[1.0]` aynı girdidir, anahtar ilk eklendiği biçimiyle saklanır; `d["1"]` ise ayrı bir anahtardır. Olmayan anahtar `yok` verir; `d.ad` biçimi `d["ad"]` ile aynıdır (okuma ve atama). Tek istisna sözlüğün kendi üyeleridir (`.uzunluk`, `.anahtarlar()`, `.değerler()`, `.içerir()`): ad bir üye adıysa üye önceliklidir; `d.anahtarlar` aynı adlı anahtarı değil metodu verir, `d.uzunluk` eleman sayısıdır. Böyle bir anahtar `d["anahtarlar"]` ile okunur ve yazılır; `d.anahtarlar = 5` ataması hatadır.

**3.8** `Aralık`: `a..b` (b hariç) ve `a..=b` (b dahil); uçlar `Tam` olmalıdır. `a > b` ise geriye doğru sayar (`3..0` → 3, 2, 1); `a..a` boş, `a..=a` tek elemanlıdır. Tam sınırındaki uçlar da taşmadan gezilir (`(üst - 1)..=üst`, üst = 9223372036854775807).

**3.9** Fonksiyonlar birinci sınıf değerlerdir: değişkene atanır, listeye konur, argüman olarak geçer, döndürülür.

**3.10** Doğruluk: `yanlış`, `yok`, `0`, `0.0`, boş metin, boş liste ve boş sözlük yanlış; diğer her değer doğru sayılır.

**3.11** `tip(x)` değerin tip adını metin olarak verir: `Tam`, `Ondalık`, `Metin`, `Mantık`, `Yok`, `Liste`, `Sözlük`, `Aralık`, `Fonksiyon`, `YerleşikFonksiyon`, `Tensör`; bir yapı nesnesi için yapının adı (`Nokta`). Ayrı bir matris tipi yoktur: matris 2 boyutlu bir `Tensör`dür (`tensör([[1, 2], [3, 4]])`, `birim_matris(n)`; 15.1).

**3.12** Dönüşümler: `tam(x)` (ondalığı keser, metni ayrıştırır), `ondalık(x)`, `metin(x)`; çevrilemeyen değer hata verir.

```tr
tam("42") + 1   // 43
tam(3.9)        // 3
metin(12) + "a" // "12a"
```

**3.13** Tip açıklamaları isteğe bağlıdır ve `ad: Tip` biçimindedir (değişken, parametre, yapı alanı; dönüş tipi için 6.8). Tip denetleyicisi bildirilen tiple çıkarılan tip uyuşmazsa ve `sabit` yeniden atanırsa programı **çalıştırmadan** reddeder. Yalnızca açıklamalı adlar denetlenir: açıklamasız bir değişkenin ilk değerinden çıkarılan tip kalıcı bir bildirim sayılmaz, değişken sonra başka tipte değer alabilir (`değişken kayıp = 0` ardından `kayıp += 0.5` geçerlidir); `sabit`in tipi ise değerinin tipidir. Denetleyicinin çıkardığı tip çalışma zamanının sonucuyla aynıdır, ondan farklı bir tip iddia etmez: `Tam / Tam` ve negatif üslü `Tam ** Tam` "Tam ya da Ondalık" sayılır (4.2, 4.3), bu yüzden `Tam` açıklamalı bir ada atanamaz (`tam(a / b)` ya da `Ondalık` açıklaması kullanılır); aralık ifadesinin tipi `Aralık`tır, `Liste` değil. Tip açıklamasında yalnızca dilin tip adları geçerlidir: `Tam`, `Ondalık`, `Metin`, `Mantık`, `Yok`, `Liste`, `Sözlük`, `Aralık`, `Fonksiyon`, `YerleşikFonksiyon`, `Tensör` ve yapı adları. Tip adları büyük/küçük harfe duyarsızdır; harfler Türkçe kuralla eşlenir (`MANTIK` ≡ `Mantık`, `ondalık` ≡ `Ondalık`). Eş ya da İngilizce adlar (`tamsayı`, `int`, `harita`, `dict` ...) ve Türkçe harfsiz yazımlar kabul edilmez; denetleyici asıl adı önerir (`'ondalik' TR'de bir tip adı değil; 'Ondalık' yazmak mı istediniz?`, `'tamsayı' TR'de bir tip adı değil; 'Tam' yazmak mı istediniz?`; kaldırılanların listesi 2.9a'dadır).

```tr hata
değişken yaş: Tam = 25
değişken ad: Tam = "Ali"   // denetim hatası: Tip uyuşmazlığı
```

---

## 4. Operatörler

**4.1** Öncelik tablosu (gevşekten sıkıya). Aynı satırdakiler soldan birleşir; `**` sağdan birleşir.

| Düzey | Operatörler | Not |
| :-: | :-- | :-- |
| 1 | `\|>` | boru hattı |
| 2 | `??` | yok-birleştirme |
| 3 | `veya` `\|\|` | |
| 4 | `ve` `&&` | |
| 5 | `değil x` | karşılaştırmadan gevşek: `değil 1 == 2` ≡ `değil (1 == 2)`; `değil a ve b` ≡ `(değil a) ve b` |
| 6 | `==` `!=` | |
| 7 | `<` `>` `<=` `>=` | |
| 8 | `..` `..=` | `1..2 + 3` ≡ `1..5` |
| 9 | `+` `-` | |
| 10 | `*` `/` `%` `@` | |
| 11 | `-x` `!x` | tekli |
| 12 | `**` | tekli eksiden sıkı: `-2 ** 2` = `-4`; `2 ** 3 ** 2` = `512` |
| 13 | `f(x)` `a[i]` `a.b` `a?.b` `f { ... }` | çağrı, indeks, üye, sondaki blok |

**4.2** Aritmetik: `Tam` ile `Tam` işlemi `Tam` verir; `/` bölümü tam çıkmazsa `Ondalık` verir (`7 / 2` = `3.5`, `6 / 3` = `2`). `Tam` ile `Ondalık` karışırsa sonuç `Ondalık`tır. `%` kalanı verir (ondalıklarda da). Sıfıra bölme ve sıfıra göre kalan hatadır. Tam bölme operatörü yoktur (`//` yorumdur).

**4.3** `**` üs alır; negatif üs `Ondalık` verir (`2 ** -1` = `0.5`).

**4.4** `< > <= >=` yalnızca sayılar arasında tanımlıdır (tensörlerde eleman bazlı 0/1 maskesi verir, bkz. 15.3); karşılaştırmalar zincirlenmez (`1 < 2 < 3` hatadır).

**4.5** Eşitlik: iki sayı tipten bağımsız değerle karşılaştırılır (`1 == 1.0`); iki `Tam` tam olarak karşılaştırılır (`9007199254740993 != 9007199254740992`); diğer değerler aynı tipte ve aynı içerikteyse eşittir (`[1, "a"] == [1, "a"]`, `"1" != 1`, `yok != yanlış`). Listeler, sözlükler ve yapı nesneleri eleman eleman bu kuralla karşılaştırılır (`[1, 2] == [1.0, 2]`); sözlüklerde anahtar sırası önemsizdir. Fonksiyonlar, yerleşikler ve yapı tanımları kimlikle karşılaştırılır: aynı değer kendisine eşittir, görünüşleri aynı olsa da ayrı tanımlar eşit değildir (`(x => x) != (x => x)`); bir nesnenin metodu aynı nesne için kendisine eşittir (`k.al == k.al`). Aralıklar uçları ve dahil olup olmamalarıyla karşılaştırılır. Tensörler şekilleri ve tüm elemanları aynıysa eşittir. 0 boyutlu tensör, değeri (`.değer`) bir sayıya eşitse o sayıya eşittir (`tensör(7.0) == 7`, `tensör([[1, 2]])[0][1] == 2.0`, `toplam(t) == 10`); en az bir ekseni olan tensör sayıya eşit değildir (`tensör([7.0]) != 7`).

**4.6** `ve`/`&&` ile `veya`/`||` kısa devre yapar ve sonucu her zaman `Mantık` olarak verir (`1 && "a"` → `doğru`). `!x` ve `değil x` doğruluğun tersidir.

**4.7** `a ?? b`: `a` `yok` ise `b`, değilse `a` (yalnızca `yok` sayılır: `0 ?? 5` → `0`).

**4.8** `a?.b`: `a` `yok` ise hata yerine `yok` verir; `yok.b` bir hatadır.

**4.9** `x |> f` ≡ `f(x)`; `x |> f(a)` ≡ `f(x, a)`. Sağ taraf bir fonksiyon çağrısı ya da fonksiyon değeri olmalıdır. Boru yalnızca fonksiyonlarla çalışır: kullanıcı fonksiyonları ve ilk parametresi borudaki değeri alan yerleşik fonksiyonlar (`toplam`, `ortalama`, `min`, `maks`, `metin`, `tip`, `relu`, `softmax`, `json_metin` ...; tam liste [`KODLAR.md`](KODLAR.md) "Boru hattıyla kullanılabilen fonksiyonlar"). Metotlar boruyla değil `.` ile zincirlenir (`l.filtrele { o > 1 }.haritala { o * 2 }`); `l |> filtrele { ... }` yazılırsa hata metot biçimini önerir (13.6).

```tr
[1, 2, 3].haritala { o * 2 } |> toplam   // 12
```

**4.10** `+` en az bir tarafı metinse öbürünü metne çevirip birleştirir (`"a" + 1` → `"a1"`); iki listeyi uç uca ekler.

**4.11** `@` matris çarpımıdır (`tensör([[1, 2], [3, 4]]) @ tensör([[1], [1]])`; matris 2 boyutlu tensördür); `+ - * /` tensörlerde eleman bazlıdır (bkz. 15.3).

---

## 5. Deyimler ve kontrol yapıları

**5.1** `değişken ad = ifade` (ya da kısa `ad := ifade`) bulunulan kapsamda yeni bir ad tanımlar; değer verilmezse `yok` olur.

**5.2** `sabit AD = ifade` yeniden atanamayan bir ad tanımlar; ihlali tip denetleyicisi çalıştırmadan önce yakalar.

**5.3** Çoklu tanım ve atama virgülle yapılır; sağdaki tüm ifadeler atamadan önce hesaplanır (takas güvenlidir), sağda tek bir liste varsa elemanlarına açılır.

```tr
değişken a, b = 1, 2
a, b = b, a
değişken ad, soyad = ["Mustafa", "Kemal"]
```

**5.4** Atama `=` ile, bileşik atama `+= -= *= /=` ile yapılır; hedef bir ad, indeks (`l[0]`) ya da alan (`p.x`) olabilir. Tanımlanmamış bir ada atama hatadır (önce `değişken` ile tanımlanmalıdır).

**5.5** `eğer koşul { ... } değilse eğer koşul { ... } değilse { ... }` bir ifadedir: değeri çalışan bloğun son deyimidir; hiçbir dal çalışmazsa `yok`.

```tr
değişken tür = eğer n < 0 { "negatif" } değilse { "pozitif" }
```

**5.6** `eşle konu { desen => sonuç ... }` konuyu kollarla sırayla karşılaştırır ve ilk eşleşen kolu çalıştırır. Desen bir ifadedir ve değeri konuyla karşılaştırılır: ad (`sınır => ...`) ve parantezli ifade (`(sınır * 2) => ...`) de desendir, ok fonksiyonu sayılmaz (kolun sonucu ise bir ok fonksiyonu olabilir: `1 => (y) => y + 1`). Bir kolda virgülle birden çok desen olabilir; `Aralık` deseni içerme sınar; `varsayılan` ya da `_` her şeyle eşleşir. Kol gövdesi bir ifade, `{ blok }` ya da `sonuç ifade` olabilir; eşleşme yoksa `yok`.

```tr
eşle puan {
    100 => "tam puan"
    90..100 => "pekiyi"
    50, 60, 70 => "yuvarlak"
    varsayılan => "diğer"
}
```

**5.7** `döngü x içinde kaynak { ... }` listede, aralıkta, metinde (karakterler), sözlükte (anahtarlar) ve tensörde (ilk eksen boyunca, bkz. 15.11) dolaşır. İki değişkenli biçim `döngü i, x içinde ...` liste/metin/tensörde indeks ile elemanı, sözlükte anahtar ile değeri verir.

**5.8** `döngü koşul { ... }` ve `iken koşul { ... }` koşul doğru oldukça tekrarlar.

**5.9** `döngü { ... }` sonsuz döngüdür; `çık` en içteki döngüden çıkar.

**5.10** `devam` en içteki döngünün sonraki turuna geçer.

**5.11** Liste üreteci: `[ifade döngü x içinde kaynak eğer koşul]` (koşul isteğe bağlı).

```tr
[x * x döngü x içinde 1..=5 eğer x % 2 == 1]   // [1, 9, 25]
```

**5.12** Her `{ ... }` bloğu (eğer/döngü gövdesi dahil) yeni bir kapsam açar; içeride tanımlanan ad dışarıda görünmez, dıştaki adlar okunup atanabilir. Döngünün her turu kendi kapsamında çalışır.

**5.13** `çık` ve `devam` yalnızca bir döngünün gövdesinde geçerlidir (araya `eğer`, `eşle`, `dene` gibi bloklar girebilir); döngü dışında yazılmaları derleme hatasıdır ve program hiç çalışmaz. Fonksiyon, ok fonksiyonu, sondaki blok, `test`, `ertele` ve `eşzamanlı` gövdeleri sınırdır: içlerinden dıştaki döngüye `çık`/`devam` uygulanamaz (`döngü { fonksiyon f() { çık } }` hatadır; fonksiyondan çıkmak için `sonuç` yazılır).

---

## 6. Fonksiyonlar

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
sayılar.filtrele { o > 2 }.haritala { o * o }
eğer (sayılar.filtrele { o > 2 }).uzunluk > 0 { yazdır("2'den büyük var") }
```

**6.8** Dönüş tipi `: Tip` ya da `-> Tip` ile bildirilir; tip denetleyicisi `sonuç` değerlerini buna göre denetler.

```tr
fonksiyon topla(a: Tam, b: Tam): Tam { sonuç a + b }
```

---

## 7. Yapılar

**7.1** `yapı Ad { alanlar ve metotlar }` bir yapı tanımlar; alanlar `ad`, `ad: Tip` ya da `ad = varsayılan` biçimindedir ve virgül/satır sonuyla ayrılır. Gövdede yalnızca alanlar ve `fonksiyon` tanımları bulunabilir; başka bir deyim ya da ifade (`yazdır(...)`, `5 + 3`) ve aynı adın iki kez tanımlanması (iki alan, iki metot ya da aynı adlı alan ve metot) sözdizimi hatasıdır.

```tr
yapı Nokta {
    x: Tam = 0
    y = 0
    fonksiyon uzaklık_kare() { sonuç kendi.x ** 2 + kendi.y ** 2 }
}
```

**7.2** Yapı adı çağrılarak nesne oluşturulur; argümanlar alan sırasıyla ya da adla verilir, verilmeyen alan varsayılanını (yoksa `yok`) alır: `Nokta(3, 4)`, `Nokta(y: 4)`. Alan sayısından fazla konumlu argüman ya da alan olmayan bir adla verilen argüman hatadır (`'Nokta' yapısının 'yy' adında bir alanı yok. Bunu mu demek istediniz: 'y'?`); yapının `kurucu`su varsa argümanlar kurucunun imzasına göre denetlenir (6.3). Varsayılanlar her nesne oluşturulurken, yapının tanımlandığı kapsamda hesaplanır (modüldeki bir yapı modülün adlarını, fonksiyon içindeki bir yapı o fonksiyonun yerel adlarını görür).

**7.3** Metotlar `kendi` ile nesnenin alanlarına erişir ve onları değiştirebilir: `p.uzaklık_kare()`.

**7.4** `kurucu` adlı metot, nesne oluşturulurken aynı argümanlarla çağrılır.

**7.5** Alanlar `p.x = 7` ile atanabilir; olmayan alana erişmek hatadır (`p?.z` ise `yok` verir). Tanımda olmayan bir alana atama da hatadır (`Nokta yapısında 'z' alanı yok; yapı tanımına 'z' alanını ekleyin`), bir metoda değer atanamaz; böylece bir nesnenin alanları her zaman yapı tanımındakilerdir ve yazdırma, eşitlik ve gezinme (15.19) aynı alanları görür.

**7.6** Nesneler içerikleriyle karşılaştırılır (`Nokta(1, 2) == Nokta(1, 2)`) ve `Nokta{x: 1, y: 2}` biçiminde yazdırılır; `tip()` yapı adını verir.

---

## 8. Hata yönetimi

**8.1** `dene { ... } yakala ad { ... }`: dene bloğunda hata olursa yakala bloğu, hata mesajı `ad` değişkenine bağlanarak çalışır. Ad verilmezse `hata` kullanılır; `yakala (ad)` biçimi de geçerlidir.

```tr
dene { fırlat "patladı" } yakala { yazdır(hata) }
```

**8.2** `fırlat ifade` bir hata başlatır; değer metne çevrilir ve yakalayan değişken her zaman `Metin`dir (`fırlat {"kod": 5}` → `"{\"kod\": 5}"`).

**8.3** Çalışma zamanı hataları (sıfıra bölme, tanımsız ad, tip uyuşmazlığı...) da yakalanabilir; mesajları `Satır N:` ile başlar. Yakalanmayan hata fonksiyon çağrılarından yukarı yayılır ve programı konum ve kod çerçevesiyle durdurur.

---

## 9. ertele

**9.1** `ertele çağrı` ya da `ertele { blok }`, içinde bulunduğu fonksiyon bittiğinde çalışacak işi kaydeder; birden çok ertele ters sırayla (son kaydedilen ilk) çalışır.

**9.2** Ertelenen iş fonksiyon `sonuç` ile erken bittiğinde ve hata ile çıkıldığında da çalışır.

**9.3** Bir blok (ör. `eğer` gövdesi) içindeki `ertele` bloğun değil, onu içeren fonksiyonun (en üst düzeyde programın, `eşzamanlı` içinde görevin, testte testin) sonunu bekler.

**9.4** Ertelenen işteki hata yutulmaz. Fonksiyon başarıyla bittiyse ertelenen işin hatası fonksiyonun hatası olur (`dene/yakala` ile yakalanır); fonksiyon zaten hata verdiyse o ilk hata korunur ve ertelenen işlerin hataları çalışma sırasıyla ona eklenir: `gövde hatası; ertele içinde de hata: ...`. Bir ertelenen iş hata verse de sonraki ertelenen işler çalışır.

**9.5** `ertele f(x)` çağrının tamamını (fonksiyonu ve argümanlarını) fonksiyon biterken hesaplar: `x`'in o anki değeri kullanılır, `ertele` satırındaki değil. Kayıt anındaki değer gerekiyorsa önce ayrı bir değişkene alınır. `eşzamanlı f(x)` de çağrıyı görevin içinde, görev çalışmaya başladığında hesaplar; değişkenler başlatıldığı kapsamdan okunur (10.4: döngüde her tur kendi kapsamındadır).

---

## 10. Eşzamanlılık ve yorumlayıcı kilidi

**10.1** `eşzamanlı { blok }` ya da `eşzamanlı çağrı(...)` işi ayrı bir görev olarak başlatır ve hemen devam eder; `bekle_hepsi()` başlatılmış tüm görevlerin bitmesini bekler. Program da bitmeden önce görevleri bekler. Görevdeki yakalanmamış hata programı durdurmaz, yorumlayıcının hata çıktısına yazılır: komut satırında standart hata, `tr test` altında testin raporu, VS Code hata ayıklama oturumunda Debug Console (`Eşzamanlı görev hatası (satır N'de başlatılan): ...`).

**10.2** Yorumlayıcı kilidi: aynı anda yalnızca bir görev TR kodu çalıştırır; bu yüzden paylaşılan listeler/sözlükler bozulmaz ve `s += 1` gibi tek bir deyim bölünmez. Kilidi bekleyen bir görev varsa kilidi tutan görev onu deyimler arasında devreder: döngülerde düzenli aralıklarla (her 256 turda bir) ve her kullanıcı fonksiyonu çağrısında. Böylece CPU-yoğun bir döngü sürerken de `eşzamanlı` görevler ve web/arayüz istekleri ilerler.

```tr
değişken toplam = 0
döngü i içinde 1..=200 { eşzamanlı { toplam += i } }
bekle_hepsi()      // toplam == 20100
```

**10.3** Engelleyici işlemler (`bekle(saniye)`, `bekle_hepsi()`, `girdi()`, ağ istekleri, dış süreç ve sunucu beklemeleri) kilidi bırakır; böylece bekleyen görevler iç içe ilerler (4 görevde `bekle(0.2)` toplam ~0.2 sn sürer).

**10.4** Görev, başlatıldığı kapsamı yakalar; döngü içinde başlatılan her görev o turun değişkenlerini görür.

---

## 11. Modüller

**11.1** `aktar "yol"` başka bir `.tr` dosyasını çalıştırır ve üst düzey adlarını bulunulan kapsama getirir. Yol bir metin sabitidir (tırnaksız `aktar yardımcı` sözdizimi hatasıdır); `.tr` uzantısı isteğe bağlıdır; yol dosya yerleşiklerindeki tek kuralla (bkz. **13.1**) önce çalışma klasörüne, bulunamazsa aktaran dosyanın klasörüne, o da olmazsa programın klasörüne göre çözülür.

**11.2** `aktar "yol" olarak ad` modülün adlarını tek bir `Sözlük` olarak `ad`a bağlar: `ym.kare(2)`, `ym["PI"]`.

**11.3** Bir modül program boyunca bir kez çalıştırılır; sonraki aktarımlar aynı değerleri paylaşır. Başka bir görev (`eşzamanlı`, web/arayüz isteği) aynı modülü o sırada yüklüyorsa aktarım yüklemenin bitmesini bekler ve aynı değerleri alır (yükleme hata verdiyse aynı hatayı). Döngüsel aktarım — yüklenmekte olan bir modülün kendi yükleme zincirinden (ya da o yükleme sırasında başlatılan bir görevden) yeniden aktarılması — hatadır.

**11.4** Modül çalışmadan önce ayrıştırılır ve tip denetiminden geçer; hata varsa modülün hiçbir satırı çalışmaz, hiçbir adı gelmez ve `aktar` deyimi (dene/yakala ile yakalanabilen) bir hata verir: `Satır 2: "m5" modülü tip denetimi hataları nedeniyle yüklenemedi: m5.tr, Satır 1: Tip uyuşmazlığı — ...`. Modülün üst düzeyinde `sonuç` derleme hatasıdır: modül bir fonksiyon değildir, aktaran programı bitiremez (fonksiyonlarının içinde geçerlidir). `tr kontrol` ve `tr derle` kaynakta yazılı bütün `aktar` deyimlerini (iç bloklardakiler ve modüllerin aktardıkları dahil) izleyip modülleri çalıştırmadan aynı denetimden geçirir; hata başlığı modülü söyler (`❌ Tip ve Güvenlik Denetimi Hataları (lib/ic.tr modülünde):`).

**11.5** Kod tanımlandığı dosyanın bağlamında çalışır: modülün üst düzeyi ve modülde tanımlanan fonksiyonlar, nereden çağrılırsa çağrılsın, o modülün dosyasındadır. Bu yüzden göreli yollar önce çalışma klasörüne, sonra o modülün klasörüne, sonra programın klasörüne göre aranır (13.1) ve başka bir dosyadan gelen çalışma hataları o dosyanın adı ve kendi satırıyla yazılır (`m2.tr, Satır 3: Sıfıra bölme hatası`; kod çerçevesi modülün kaynağından çizilir). `fırlat` ile atılan değerler ise değişmeden yakalanır. Dosya bilgisi görevindir, paylaşılmaz: bir modül yüklenirken kilidi bırakırsa (ör. `bekle`) öbür görevlerin göreli yolları kendi dosyalarına göre çözülmeye devam eder.

---

## 12. Testler

**12.1** `test "ad" { ... }` bir test tanımlar. `tr test` her testi sırayla çalıştırır; dosya normal çalıştırıldığında (`tr dosya.tr`) test blokları atlanır.

```tr
fonksiyon topla(a, b) { sonuç a + b }

test "toplama doğru" {
    eşit_mi(4, topla(2, 2))
}
```

**12.2** `test` bağlamsal bir anahtar kelimedir: yalnızca bir deyimin başında ve ardından bir metin geldiğinde test başlatır; başka her yerde sıradan bir addır (`değişken test = 5`).

**12.3** `doğrula(koşul, [mesaj])` koşul yanlışsa hata fırlatır: `doğrulama başarısız: mesaj`.

**12.4** `eşit_mi(beklenen, gerçek, [mesaj])` iki değer `==` anlamında eşit değilse ikisini de tipiyle gösteren hata fırlatır: `eşitlik başarısız: beklenen "5" (Metin), gerçek 5 (Tam)`. Her iki yerleşik test dışında da (ör. sözleşme denetimi için) kullanılabilir.

**12.5** Önce dosyanın kendisi (testler hariç) çalışıp üst düzey tanımları oluşturur; sonra her test bu tanımların üzerinde açılan **yeni bir kapsamda** çalışır. Bir testin yerel adları diğerine sızmaz; kalan ya da hata veren test diğerlerini durdurmaz. Testteki `ertele` test bitince çalışır, testin başlattığı görevler beklenir.

**12.6** Test gövdesinde, gövdedeki bir fonksiyonun dışında yazılan `sonuç` derleme hatasıdır: test her zaman sonuna kadar çalışır, bir doğrulamayı atlayarak sessizce "geçmez". (Döngü dışındaki `çık`/`devam` da her yerde olduğu gibi hatadır, 5.13.)

`tr test` kullanımı ve çıktı biçimi için `KULLANIM_KILAVUZU.md`'ye bakın; kısa bir örnek: `ornekler/03_testler_test.tr`.

---

## 13. Yerleşik fonksiyonlar ve metotlar

Tam liste, her kodun imzası ve örneğiyle [`KODLAR.md`](KODLAR.md)'dedir; burada dilin parçası sayılan çekirdek seçki tanımlanır. Metotlar `değer.metot(...)` ile çağrılır; özellikler (`.uzunluk`, `.ilk`, `.son`) parantezsiz yazılır. Metotlar da yerleşikler gibi argüman sayısını ve tipini denetler: `metin.içerir() 1 argüman bekler, 0 verildi (kullanım: metin.içerir(alt))`, `metin.içerir() 1. argüman olarak Metin bekler, Tam (1) verildi`. Her yerleşiğin ve metodun tek adı vardır ve yalnızca Türkçe yazımıyla geçerlidir (`toplam`; `toplm` sıradan bir addır); eş anlamlı ad yoktur (bkz. 2.9, 2.9a). Bir yerleşiğin adını kendi tanımınıza vermek uyarı verir (2.9a, "Yerleşik ad gölgeleme uyarısı").

**13.1** Genel: `yazdır(...)` (değerleri boşlukla birleştirip yazar), `tip`, `toplam`, `ortalama`, `min`, `maks`, `mutlak`, `karekök`, `yuvarla`, `aralık(a, b)`, `bekle`, `bekle_hepsi`, `zaman_ms`, `dosya_oku`, `dosya_yaz`, `girdi`. Sayı yerleşikleri işleçlerin kurallarına uyar (3.1, 4.2): `toplam(liste)` elemanlar Tam ise Tam, bir Ondalık varsa Ondalık verir, sayı olmayan eleman hatadır; `ortalama(liste)` Ondalık verir, boş listede ve sayı olmayan elemanda hata verir; `min` ve `maks` tek bir liste ya da birden çok sayı alır ve kazanan değerin kendisini tipiyle verir (`maks([2.0, 1.5])` → `2.0`), Tam'ları tam karşılaştırır (`maks([9007199254740993, 1])` → `9007199254740993`), sayı olmayan değerde (metin dahil) hata, boş listede `yok` verir; `yuvarla(x, basamak)` basamak pozitifse Ondalık, 0 ya da negatifse Tam verir (`yuvarla(1234, -2)` → `1200`). `girdi([istem])` standart girişten bir satır okur (satır sonu atılır; giriş bittiyse boş metin verir); süreçteki bütün okumalar (art arda `girdi()` çağrıları, etkileşimli kabuk, terminal hata ayıklayıcısı) tek bir tamponlu okuyucudan yapılır, bu yüzden boruyla verilen satırlar sırayla gelir ve hiçbiri kaybolmaz. Yerleşikler eksik ya da fazla argümanı ve yanlış tipi reddeder; hata `aralık() 1. argüman olarak Tam bekler, Metin ("a") verildi` biçimindedir. `dosya_oku(yol)` ve `dosya_yaz(yol, içerik)` yolu `Metin` olarak ister; göreli bir yol önce çalışma klasörüne, orada yoksa kodun bulunduğu modülün klasörüne (11.5), sonra programın bulunduğu klasöre göre çözülür ve dosyanın bulunduğu ilk yer kullanılır; dosya hiçbir yerde yoksa bulunacağı klasörün var olduğu ilk yer seçilir (düz bir `rapor.txt` için bu çalışma klasörüdür). Kural okuma ve yazma için aynıdır: programın yanındaki bir dosya okunup güncellenince yine oraya yazılır, yeni dosyalar çalışma klasöründe oluşur. Dosya bulunamazsa hata bakılan yerleri listeler. Dosya hataları Türkçedir (dosya ya da klasör yok, izin yok, bir klasör, disk dolu, dosya kullanımda...).

**13.2** Metin metotları: `.uzunluk`, `.büyük_harf()` / `.küçük_harf()` (Türkçe `i/İ`, `ı/I` kurallarıyla), `.kırp()`, `.böl([ayırıcı])` (varsayılan boşluk), `.değiştir(eski, yeni)`, `.içerir(alt)`, `.başlar(önek)`, `.biter(sonek)`, `.ters_çevir()`; argümanları `Metin` olmalıdır.

**13.3** Liste metotları: `.uzunluk`, `.ilk`, `.son`, `.filtrele(f)`, `.haritala(f)`, `.indirge([başlangıç], f)` (bkz. 13.6), `.sırala()` (yeni bir liste verir; liste yalnızca sayılardan ya da yalnızca metinlerden oluşmalıdır, başka bir karışım karşılaştırılamaz ve hatadır; sayılar tam karşılaştırılır, eşitlerin sırası korunur, `NaN` sona gelir; metinler karakter kodu sırasıyladır), `.ters_çevir()`, `.birleştir(ayırıcı)`, `.benzersiz()`, `.içerir(x)` (her tipte değeri `==` ile arar), `.ekle(x, ...)` (listeyi yerinde değiştirir). Listenin toplamı ve ortalaması fonksiyondur: `toplam(l)`, `ortalama(l)`.

**13.4** Sözlük metotları: `.anahtarlar()`, `.değerler()`, `.içerir(anahtar)`.

**13.5** Adlı argümanlar: bir yerleşik ya da metot yalnızca imzasındaki adlı argümanları kabul eder (bkz. [`KODLAR.md`](KODLAR.md)); bilinmeyen bir adlı argüman sessizce yok sayılmaz, çağrı hata verir. Adlı argüman almayanlarda hata `yazdır() 'ayraç' adında bir parametre almaz (adlı argüman almaz)`, alanlarda geçerli adları sayar (`web_sunucu() 'adres' adında bir parametre almaz (geçerli adlar: port)`) ya da yazıma yakın adı önerir (`toplam() 'eksn' adında bir parametre almaz. Bunu mu demek istediniz: 'eksen'?`).

**13.6** Metot mu, fonksiyon mu: bir değeri dönüştüren ya da ona soru soran iş **metottur** (`x.uzunluk`, `l.filtrele(f)`, `l.haritala(f)`, `l.indirge(b, f)`, `l.sırala()`, `l.ekle(x)`, `t.devrik()`); matematik ve tensör işi **fonksiyondur** (`toplam(l)`, `ortalama(l)`, `min`, `maks`, `relu(t)`, `softmax(t)` ...). Her işin tek biçimi vardır: aynı iş hem fonksiyon hem metot olarak yazılamaz. Öbür biçim yazılınca hata yenisini gösterir: `uzunluk(l)` → `'uzunluk(...)' TR'de bir fonksiyon değil; 'x.uzunluk' yazın`, `l.toplam()` → `'liste.toplam()' TR'de yok; 'toplam(liste)' yazın`. Bu adlar artık sıradan adlardır (`değişken uzunluk = 5` gölgeleme sayılmaz). `l.indirge([başlangıç], f)` listeyi soldan sağa `f(birikim, eleman)` ile tek değere indirger: başlangıç verilmezse ilk eleman başlangıçtır, boş listede başlangıç (verilmemişse `yok`) döner. Fonksiyon son argümandır, böylece sondaki blokla da yazılır; tek parametreli fonksiyon (sondaki bloğun `o`'su) `[birikim, eleman]` listesini alır: `[1, 2, 3].indirge(0) { o[0] + o[1] }` → `6`. Fonksiyon başa yazılırsa (`l.indirge(f, 0)`) hata doğru sırayı söyler.

Hata ayıklayıcıya bağlı `kesme_noktası` deyimi (`kesme_noktası`, `kesme_noktası eğer koşul`) etkileşimli olduğu için bu tanımın testlerine girmez; davranışı `pkg/hataayiklama` birim testleri ve `KULLANIM_KILAVUZU.md` §5 ile belgelenir (çalıştırılabilir örnek: `ornekler/04_hata_ayiklama.tr`).

---

## 14. Bilinen sınırlamalar

Bunlar bugünkü uygulamanın gerçek davranışlarıdır; ileride değişebilirler.

- **Metin karşılaştırması yok:** `"a" < "b"` hatadır; `<`, `>`, `<=`, `>=` yalnızca sayılarda çalışır. Zincirleme karşılaştırma (`1 < x < 3`) da yoktur.
- **Tam bölme operatörü yok:** `//` yorum başlatır; tam bölme için `tam(a / b)` kullanılır.
- **Testler arası yalıtım sığdır:** her test yeni bir kapsamda çalışır, ama üst düzey bir değişkene *atama* yapan ya da paylaşılan bir listeyi değiştiren test, sonraki testlerin gördüğü değeri değiştirir (üst düzey durum her testte yeniden kurulmaz).
- **Görev hataları testi düşürmez:** testin başlattığı `eşzamanlı` görevdeki hata test raporuna yazılır, test kalmaz.
- **`sabit` yalnızca ada bağlıdır:** ad yeniden atanamaz (denetleyici ve çalışma zamanı denetler), ama değer bir listeyse içeriği değişebilir; `aktar ... olarak m` ile alınan modül sözlüğünde sabitlik korunmaz (`m["AD"] = 1` çalışır).
- **Tip denetimi sığdır:** tipi çıkarılamayan her değer `Herhangi` sayılır; parametre tipleri yalnızca adıyla doğrudan çağrılan fonksiyonların konumlu argümanlarında denetlenir (adlı argümanlar, metotlar ve fonksiyon değerleri üzerinden yapılan çağrılar denetlenmez).
- **Sınır dışı okuma hata vermez:** `l[10]` ve `m[10]` `yok` döner; bu, hataların geç fark edilmesine yol açabilir.
- **Yapılarda kalıtım yok;** metotlar yapı tanımının içinde yazılmalıdır.
- **Metin içi yerleştirme konumu yaklaşıktır:** `{ifade}` içindeki hataların sütunu, ifadeden önce kaçış dizisi varsa bir-iki karakter kayabilir.
- **Tanımlar öne alınmaz:** fonksiyonlar ve yapılar, tanımlandıkları satır çalıştıktan sonra kullanılabilir.
- **Tensörde kalan işleci yoktur:** `%` tensörlerde tanımlı değildir.

---

## 15. Tensörler ve otomatik türev

Tensör, n boyutlu bir sayı dizisidir (tip adı `Tensör`); dilin tek sayısal dizi türüdür. Elemanların türü varsayılan olarak `float32`'dir; `bf16` ve `fp16` de seçilebilir (**15.12**). Hesaplar en az `float32` ile (yaklaşık 7 anlamlı basamak) yapılır: `(tensör([1.0]) / 3).değer[0]` `0.33333334`tür ve `1 / 3`e eşit değildir; `tensör([16777217.0])` 16777216 olarak saklanır. İşlemler hesap çizgesine kaydedilir; gradyanlar ters yönlü otomatik türevle hesaplanır. Ayrıntılı kullanım için `KULLANIM_KILAVUZU.md` §4.D'ye bakın.

**15.1** Oluşturma: `tensör(veri, türev_izle: doğru)` veri olarak iç içe sayı listesi, tek sayı ya da tensör alır (tensörden kendi verisine sahip yeni bir yaprak üretir); düzensiz liste hatadır. `türev_izle` varsayılanı her girdide (tek sayı dahil) `yanlış`tır. Tek sayı 0 boyutlu tensör verir: `tensör(3.0)`'ün şekli `[]`, `.değer`i `3.0`, `.gradyan`ı `yok`tur; türev için `tensör(3.0, türev_izle: doğru)` yazılır. `sıfırlar(şekil)`, `birler(şekil)`, `rastgele_normal(şekil, tohum: n)` şekli liste olarak alır (`[2, 3]`); iki tam sayı da kabul edilir (`sıfırlar(2, 3)` ≡ `sıfırlar([2, 3])`). Matris 2 boyutlu tensördür: `tensör([[1, 2], [3, 4]])` ile oluşturulur; rastgele bir matris `rastgele_normal([satır, sütun], tohum: n)`, birim matris `birim_matris(n)` ile elde edilir. Ayrı matris oluşturucuları `matris(...)` ve `rastgele_matris(...)` kaldırılmıştır; yazıldıklarında hata karşılığı önerir (`'matris' TR'de yok; 'tensör' yazmak mı istediniz?`, `'rastgele_matris' TR'de yok; 'rastgele_normal' yazmak mı istediniz?`). Eski matris özellikleri kaldırılmıştır; karşılıkları: `.boyut` → `.şekil`, `.satır` / `.sütun` → `.şekil[0]` / `.şekil[1]`, `.T` → `.devrik()`, `.toplam` / `.ortalama` → `toplam(t)` / `ortalama(t)` (0 boyutlu tensör), `.liste` → `.değer`. Eski adla erişim karşılığı öneren bir hata verir.

**15.2** Özellikler: `.şekil` (tam sayı listesi), `.değer` (iç içe Ondalık listesi; 0 boyutluda tek sayı; her float32 değer en kısa ondalık gösterimine karşılık gelen sayıdır; tam sayılı işaretli tensörlerde — 15.9, 16.5 — elemanlar `Tam`dır), `.gradyan` (yaprak tensörde tensör ya da `yok`; bir işlemin sonucu olan tensörde hata), `.cihaz` (`"cpu"`). Doğruluk değeri: tek elemanlı tensör eleman 0'dan farklıysa doğru, boş tensör yanlıştır. Görünüm metotları veriyi kopyalamaz: `.şekillendir(liste)` (bir eksen `-1` olabilir), `.devrik()` (son iki eksen), `.eksenleri_sırala(liste)`, `.dilim(eksen, baş, son)` (`son` hariç; negatif indeks sondan sayar). Görünümler bitişik olmayan tensörlerde de (ör. `.devrik()` sonucunda) görünümdür; tek istisna, `.şekillendir`in bitişik olmayan bir tensörde depoda düzenli adımla ifade edilemeyen eksenleri birleştirmesidir (ör. `t.devrik().şekillendir([-1])`): o zaman sonuç bir kopyadır ve ona yazmak `t`'yi değiştirmez. Boyutu 1 eksen ekleyen ya da silen `.şekillendir` her zaman görünümdür. `.cihaza(ad)` tensörü taşır; arka ucu olmayan cihaz (`"gpu"`) hatadır.

**15.3** İşleçler: `@` matris çarpımıdır (`[..., M, K] @ [K, N]`, toplu çarpımda öndeki eksenler eşit olmalı; 1 boyutlu işlenen vektör sayılır). `+ - * /` eleman bazlıdır ve yayınlamalıdır: şekiller sağdan hizalanır, her eksen eşit ya da 1 olmalıdır. Diğer işlenen sayı, liste ya da tensör olabilir ve iki yanda da durabilir (`1 - t`, `2 / t`). Tekli `-t` ve `t ** sayı` türev izler. `< > <= >=` sayıyla ya da aynı şekilli tensörle eleman bazlı karşılaştırıp türev izlemeyen 0/1 maskesi verir. `t[i]` ilk eksenin i. elemanını (bir eksen eksilir; negatif indeks sondan sayar), `t[baş..son]` ilk eksende dilimi verir; ikisi de veriyi kopyalamayan, türev izleyen görünümlerdir (`t[i][j]`). İndeksle atama (`t[i] = d`, `t[i][j] = d`, `t[baş..son] = d`, bileşik `t[i] += d`) veriyi yerinde değiştirir: `d` sayıysa hedefin tüm elemanlarına yazılır, liste ya da tensörse şekli hedefin şekliyle aynı olmalıdır; türev izlenen tensöre yalnızca `türevsiz { ... }` içinde atanabilir. İşleç ve özellik hataları `Satır N:` önekiyle bildirilir.

**15.4** Fonksiyonlar: `relu`, `sigmoid`, `tanh`, `gelu`, `karekök`, `üstel`, `log` (ve **15.14** `sinüs`, `kosinüs`) eleman bazlıdır; `softmax(x, eksen: -1)` ve `log_softmax(x, eksen: -1)` verilen eksende çalışır; elemanlarının hepsi `-sonsuz` olan (tamamen maskeli) bir satırın softmax'ı 0, log_softmax'ı `-sonsuz`dur ve gradyanı 0'dır (**15.17**'deki boş dikkat satırıyla tutarlı). Tensör yalnızca tensör girdisinden (`birim_matris(n)` sonucu dahil) çıkar: sayı ya da liste girdisinde sonuç girdinin biçimindedir (eski karşılığı olan adlarda eski davranış; `gelu`, `üstel`, `log` sayıda Ondalık, listede aynı yapıda liste; `softmax` ve `log_softmax` yalnızca düz sayı listesinde liste verir, iç içe liste için `tensör(...)` ile çevrilmelidir). İndirgemeler `toplam`, `ortalama`, `maks`, `min` eksen verilmezse tüm elemanları 0 boyutlu tensöre indirger, `eksen: k` ile o ekseni kaldırır (`boyutu_koru: doğru` ile korur, bkz. **15.16**). Negatif eksen sondan sayar. `toplam` ve `ortalama` ekseni konumlu da alır (`toplam(x, 1)`); `maks` ve `min` yalnızca adlı `eksen:` kabul eder, ikinci konumlu argüman hatadır. Boş tensörün (ya da boş eksenin) `ortalama`sı tanımsız olduğundan hatadır; `toplam`ı 0'dır. Sayı olmayan değer (NaN) yayılır: `relu` NaN'ı korur; `maks` ve `min` indirgedikleri elemanlar arasında NaN varsa NaN verir, `maks_indeks` (15.9) ilk NaN'ın indeksini verir; sonuç türev izlenen ve izlenmeyen tensörde aynıdır. Argüman tensör değilse bu adların eski (düz sayı listesi) davranışı geçerlidir; ancak listede tensör varsa `toplam`/`ortalama` onları katlar, iç içe liste ya da tensör olmayan argümanla `eksen:` hatadır. `sıfırlar(2, 3)` biçimi adlı argüman almaz. Adlı argümanlar yalnızca Türkçe adlarıyla ve Türkçe yazımlarıyla kabul edilir; başka dildeki alışılmış adlar (`lr`, `axis`...), Türkçe harfsiz yazımlar (`ogrenme_orani`) ve kaldırılan adlar (`öğrenme_hızı`) için hata mesajı Türkçe karşılığı önerir.

**15.5** Sinir ağı: `gömme(tablo, indeksler)` `[V, D]` tablosundan satır seçer (sonuç şekli indeks şekli + `[D]`); `çapraz_entropi(çıktı, hedef)` son eksen sınıf olmak üzere ortalama kaybı skaler verir (satır başına bir hedef; negatif hedef yok sayılır; bütün hedefler yok sayılırsa ortalama tanımsız olduğundan hatadır); `katman_normu(x, ağırlık, sapma)` ve `rms_normu(x, ağırlık)` son eksende normalleştirir (`ağırlık`/`sapma` `[D]` şeklinde ya da `yok`; `epsilon:` varsayılanı `0.00001`).

**15.6** Türev: `geri_yayılım(kayıp)` tek elemanlı bir kayıptan geri yayılım yapar; yaprak tensörlerin `.gradyan`ı birikir: her geri yayılım yeni gradyanı öncekine ekler (`geri_yayılım(toplam(v * 3))` iki kez çağrılınca `v.gradyan` `[6.0, 6.0]` olur). Eğitim adımları arasında gradyanlar iyileştiricinin `.sıfırla()`sıyla sıfırlanır (15.7). `.gradyan` gradyan deposunun **canlı takma adıdır**, kopyası değildir: sonraki geri yayılımlar, `.sıfırla()` ve `gradyan_kırp` daha önce alınmış `.gradyan` değerini de değiştirir; bu işlemler deponun sürümünü artırdığından, `.gradyan` ile kurulmuş bir hesap gradyan değiştikten sonra geri yayılırsa yerinde değişiklik hatası verir (`... yerinde değiştirildi ...`). Gradyanı saklamak için `tensör(x.gradyan)` ile kopyası alınır. `türevsiz { ... }` bloğu türev izlemeden çalıştırıp bloğun değerini döndürür; `türevsiz(x)` tensörü çizgeden ayırır. Türevsiz kip **göreve bağlıdır** (`karışık_hassasiyet` gibi, 15.12): blok yorumlayıcı kilidini devrettiğinde (fonksiyon çağrısı, döngü turu, `bekle`, 10.2–10.3) aynı anda çalışan başka görevin işlemleri türev izlemeye devam eder ve o görevde türev izlenen tensöre indeksle atama yine reddedilir; yeni bir `eşzamanlı` görev türevli kipte başlar. Türev izlemeyen bir kayıptan geri yayılım hatadır. Bir geri yayılım sürerken (ör. `özel_türev`in `geri`si kilidi bıraktığında) başka bir görev aynı çizgenin düğümlerinden geri yayılım başlatırsa açık hata alır.

**15.7** İyileştiriciler: `AdamW(parametreler, öğrenme_oranı:, ağırlık_çürümesi:, beta1:, beta2:, epsilon:)` ve `SGD(parametreler, öğrenme_oranı:, momentum:, ağırlık_çürümesi:)`. Parametreler türev izleyen yaprak tensörlerdir; `.adım()` günceller, `.sıfırla()` gradyanları sıfırlar. Sayısal ayarlar sonlu ve negatif olmayan sayılar olmalıdır; sayı olmayan (NaN) ya da sonsuz bir değer hatadır. AdamW'nin sapma düzeltmesi her parametrenin kendi güncelleme sayısını kullanır (gradyan almadığı adımlar sayılmaz). İkisi de `ana_kopya:` (`Mantık`, varsayılan `doğru`) alır: açıkken 16 bitlik parametrelerin float32 ana kopyası tutulur, güncelleme orada yapılıp parametreye yuvarlanarak yazılır; kapalıyken güncelleme doğrudan 16 bitlik parametreye yuvarlanır. float32 parametrelerde etkisizdir.

**15.8** Hatalar: şekil ve eksen hataları ilgili şekilleri gösteren Türkçe bir mesajla (`[32, 1024] ile [512, 10] matris çarpımı yapılamaz; 1024 ≠ 512`) bildirilir ve `dene/yakala` ile yakalanabilir; program çökmez.

**15.9** `maks_indeks(x, eksen: -1)` verilen eksende en büyük elemanın indeksini verir (eşitlikte ilki); sonuç o eksen kaldırılmış, tam sayı değerli ve türev izlemeyen bir tensördür (1 boyutlu girdide 0 boyutlu). Sonuç tam sayılı işaretlidir: `.değer` `Tam` ya da `Tam` listesi verir ve ondalıksız yazdırılır; bu işaret işlem sonuçlarına geçmez. Eksen yalnızca adlı verilir. `gradyan_kırp(parametreler, en_fazla: 1.0)` parametre gradyanlarının toplam L2 normu `en_fazla`'yı aşıyorsa hepsini aynı oranda küçültür ve kırpma öncesi normu `Ondalık` olarak döndürür.

**15.10** `tensör_kaydet(yol, sözlük)` adları metin, değerleri tensör olan bir sözlüğü yalnızca verilen yola yazar (dosya varsa üzerine yazılır; yazma atomiktir: veri önce aynı klasörde geçici bir dosyaya yazılıp diske işlenir, sonra eski dosyanın yerine konur, böylece yarıda kalan bir yazma önceki dosyayı bozmaz; `model.kaydet` de öyle); `tensör_yükle(yol, türev_izle: yanlış)` aynı sırayla bir sözlük döndürür. Biçim sürüm başlıklıdır (`TRTENSOR`, sürüm 2, her tensör için ad, veri türü kodu — 1 = float32, 2 = bf16, 3 = fp16 —, şekil ve tensörün kendi türündeki veri: float32 4, bf16/fp16 2 bayt; sonda CRC-32); yüklenen tensör kaydedildiği türdedir. Sürüm 1 dosyaları (yalnızca float32) okunur; bilinmeyen sürüm/tür, eksik, bozuk ya da başka türden dosya açık bir hatayla reddedilir. Göreli yollar `tensör_kaydet`/`tensör_yükle` ile `model.kaydet`/`model_yükle`de `dosya_oku`/`dosya_yaz` ile aynı tek kuralla (bkz. **13.1**) çözülür; dosya hataları Türkçedir ve okumada bakılan yerleri listeler.

**15.11** Döngü: `döngü x içinde t`, `döngü i, x içinde t` ve liste üreteci (`[ifade döngü x içinde t]`) tensörü ilk ekseni boyunca gezer. 1 boyutlu tensörde öğeler sayıdır (`.değer`deki gibi `Ondalık`; tam sayılı tensörde `Tam`); N boyutluda öğeler `t[i]` ile aynı, (N-1) boyutlu görünümlerdir (veriyi paylaşır, türev izler; öğeye atama tensörü değiştirir). 0 boyutlu tensör üzerinde döngü hatadır. Tam sayılı tensörün görünümleri (`t[i]`, `t[baş..son]`, döngü öğeleri) de tam sayılıdır.

```tr
değişken t = tensör([[1, 2], [3, 4]])
döngü i, satır içinde t {
    yazdır(i, satır.değer)                     // 0 [1.0, 2.0] / 1 [3.0, 4.0]
}
[x * 2 döngü x içinde tensör([1.5, 2.5])]      // [3.0, 5.0]
```

**15.12** Veri türleri: tür adları metindir: `"float32"` (varsayılan), `"bf16"`, `"fp16"`; başka ad (ör. `"bfloat16"`) en yakın geçerli adı öneren bir hatadır. `tensör`, `sıfırlar`, `birler` ve `rastgele_normal` `tür:` adlı argümanını alır; değerler en yakına-çifte yuvarlanır. `tensör(t)` tür verilmezse `t`'nin türünü korur. `.tür` türün adını verir; `.türe(ad)` türevlenebilir bir dönüşümdür (aynı türde tensörün kendisi). İşlemler içte en az float32 ile biriktirir; aynı türden işlenenlerin sonucu o türde, farklı türlerinki `float32`'dir; sayıyla işlemler (`t * 2`, `t + 2`, `t - 2`, `t / 2`, `2 / t`) tensörün türünü korur; 16 bitlik tensörlerin gradyanları `float32`'dir. Yazdırma ve İzleme Masası float32 dışındaki türü başlığa ekler (`Tensör[2×3] bf16`). `.değer` her zaman `Ondalık` verir.

**15.13** Karışık hassasiyet: `karışık_hassasiyet(tür) { ... }` bir yerleşiktir; blok, sondaki blok kuralıyla son argümandır. Blok içinde `@` girdileri `tür`e çevrilip float32 biriktirmeyle `float32` sonuç verir; `katman_normu`, `rms_normu`, `softmax`, `log_softmax`, `çapraz_entropi`, `toplam` ve `ortalama` 16 bitlik girdileri `float32`'ye açar. Dönüşümler türevlenebilir kopyalardır; ağırlıkların türü değişmez. Bloğun değeri son ifadesidir. `"float32"` bloğu karışık hassasiyeti kapatır; iç içe bloklarda en içteki geçerlidir ve blok bitince (hata ile de) önceki tür geri gelir. Durum görevin (eşzamanlı blok, web/arayüz isteği) durumudur: başka görevlere sızmaz, yeni görevler `float32` ile başlar.

**15.14** `sinüs(x)` ve `kosinüs(x)` (radyan) **15.4**'teki eleman bazlı fonksiyonlar gibidir: tensörde türev izleyen tensör (türevleri `kosinüs(x)` ve `-sinüs(x)`), sayıda `Ondalık`, listede aynı yapıda liste verir. Tensör sonuçları float32'ye doğru yuvarlanır (en çok 1 ulp).

**15.15** `birleştir([a, b, ...], eksen: -1)` tensörleri verilen eksen boyunca birleştirir; hepsi aynı sayıda eksenli olmalı ve birleşen eksen dışındaki eksenleri eşit olmalıdır, aksi halde şekilleri gösteren bir hata verilir. Sonuç türev izler; türler farklıysa `float32`'dir. Tensörün `.böl(parça_sayısı, eksen: -1)` metodu tensörü o eksende eşit parçalara böler ve parçaların listesini verir; parçalar veriyi paylaşan, türev izleyen görünümlerdir; eksen uzunluğu parça sayısına bölünmüyorsa hatadır. (Liste metodu `.birleştir(ayırıcı)` ve metin metodu `.böl(ayırıcı)` değişmez; üye adları tipten bağımsızdır.)

**15.16** `toplam`, `ortalama`, `maks` ve `min` `boyutu_koru: doğru` alınca indirgenen ekseni 1 uzunlukta korur: `[B, T, D]` şeklinde `x` için `toplam(x, eksen: -1, boyutu_koru: doğru)` `[B, T, 1]` verir ve `x`'e yayınlanabilir. Eksen verilmezse bütün eksenler 1 olarak korunur. `boyutu_koru` `Mantık` olmalıdır ve yalnızca tensörlerde geçerlidir.

**15.17** `dikkat(q, k, v, nedensel: doğru, pencere: 0, maske: yok)` birleşik ölçekli nokta çarpımı dikkatidir: `softmax(q @ kᵀ / √dh + maske) @ v` ile aynı sonucu (float32 yuvarlama farkıyla) ve gradyanları verir. Şekiller: `q` `[..., H, Tq, dh]`, `k` `[..., Hkv, Tk, dh]`, `v` `[..., Hkv, Tk, dv]`; öndeki eksenler eşit olmalı, sonuç `[..., H, Tq, dv]`'dir (2 boyutlu girdiler tek başlıdır). `Hkv < H` gruplu sorgu dikkatidir: `H`, `Hkv`'nin katı olmalı; ardışık `H / Hkv` baş aynı `k`/`v` başını kullanır (kopyalanmaz). Ölçek her zaman `1/√dh`'dir. `nedensel` (varsayılan `doğru`) sorgu `i`'nin yalnızca `Tk - Tq + i`'ye kadarki anahtarlara bakmasını sağlar; `Tq < Tk` iken (önbellekli üretim) nedensellik sondan hizalıdır. `pencere: w > 0` sorguyu son `w` konuma (kendisi dahil) sınırlar; `nedensel: yanlış` ile pencere iki yönlüdür (`|Tk - Tq + i - j| < w`). `maske` `[..., H, Tq, Tk]`'ye yayınlanabilen 0/1 tensörüdür: sıfırdan farklı yerlere bakılır; türev izlemez. Hiçbir anahtara bakamayan satırın çıktısı 0'dır. Hesap tek bir türev düğümüdür: puanlar satır blokları hâlinde, çevrimiçi softmax ile hesaplanır, `Tq × Tk` matris tutulmaz (bellek uzunlukla doğrusal); pencere ve nedensellik dışındaki bloklar hesaplanmaz; geri yayılım puanları yeniden hesaplar. `karışık_hassasiyet` bloğunda girdiler blok türüne çevrilir, biriktirme ve sonuç `float32`'dir; blok dışında sonuç girdilerin türündedir. Şekil ve argüman hataları Türkçe bir mesajla bildirilir; `ölçek:` gibi bilinmeyen adlı argüman hatadır.

**15.18** `örnekle(logit, sıcaklık: 1.0, en_iyi_k: 0, tohum: yok)` son eksen üzerindeki `softmax(logit / sıcaklık)` dağılımından bir indeks örnekler; sonuç son ekseni düşürülmüş (1 boyutlu girdide 0 boyutlu), tam sayılı işaretli (`.değer` `Tam`) ve türev izlemeyen bir tensördür. `en_iyi_k > 0` yalnızca en olası `k` sınıf arasından seçer (eşitlikte küçük indeks önce); `0` hepsi demektir. `sıcaklık: 0` en olasıyı verir (`maks_indeks` ile aynı); negatif sıcaklık ve negatif `en_iyi_k` hatadır. Aynı `tohum` (tam sayı) aynı sonucu verir; tohumsuz çağrılar paylaşılan bir üreteçten çeker ve her çağrıda farklı sonuç verir.

**15.19** `parametreler(model, adlarıyla: yanlış)` bir yapı nesnesini — iç içe yapılar, listeler ve sözlüklerle birlikte — gezip türev izleyen yaprak tensörleri (`tensör(..., türev_izle: doğru)`) bir liste olarak verir. Sıra sabittir: yapı alanları tanım sırasıyla, listeler sırayla, sözlükler ekleme sırasıyla gezilir. Aynı tensör birden çok yerde geçerse ilk yerinde bir kez yer alır; ara sonuçlar, türev izlemeyen tensörler ve tensör olmayan değerler atlanır; döngülü yapılar bir kez gezilir. `adlarıyla: doğru` yol adı → tensör sözlüğü verir: adlar `.` ile birleştirilir, liste indeksleri sayı, sözlük anahtarları anahtarın kendisidir (`bloklar.0.dikkat.Wq`); bu sözlük doğrudan `tensör_kaydet` ile kaydedilip `tensör_yükle` ile okunabilir (**15.10**). İki farklı tensör aynı yol adına düşerse hatadır.

**15.20** Kullanıcı tanımlı türev: `özel_türev(ileri: f, geri: g)` yeni bir işlemin ileri ve geri hesabını kullanıcının yazmasını sağlar ve sıradan bir fonksiyon gibi çağrılan bir değer (`YerleşikFonksiyon`) döndürür; yalnızca bu iki adlı argümanı alır. Çağrıldığında `ileri` verilen konumlu ve adlı argümanlarla **türevsiz** çalışır ve `[çıktı, saklananlar]` döndürmelidir: `çıktı` bir tensör ya da tensör listesi (15.22), `saklananlar` geri hesabın ihtiyaç duyacağı değerlerin (tensör ya da sıradan değer) listesidir. Konumlu tensör girdilerinden en az biri türev izliyorsa ve türevsiz kipte değilsek çıktı hesap çizgesine **tek bir düğüm** olarak kaydedilir: `ileri`nin içindeki ara sonuçlar çizgede tutulmaz, geri yayılıma kadar yalnızca `saklananlar` yaşar. Hiçbir girdi türev izlemiyorsa çıktı olduğu gibi döner. Türev yalnızca konumlu tensör girdileri için hesaplanır; `ileri`nin dışarıdan yakaladığı tensörlere gradyan akmaz.

```tr
değişken kare = özel_türev(
    ileri: fonksiyon(x) { sonuç [x * x, [x]] },          // [çıktı, saklananlar]
    geri: fonksiyon(saklananlar, g) {                     // g: çıktının gradyanı
        değişken x = saklananlar[0]
        sonuç [g * 2 * x]                                 // her girdi için bir gradyan (ya da yok)
    }
)
değişken y = kare(tensör([3.0], türev_izle: doğru))      // geri_yayılım(toplam(y)) → x.gradyan [6.0]
```

**15.21** Geri hesap: geri yayılımda `geri(saklananlar, g)` türevsiz çalışır; `g` çıktının gradyanıdır (çıktıyla aynı şekil). `geri` konumlu girdi sayısı kadar öğeli bir liste döndürür: her girdi için girdiyle **aynı şekilde** bir gradyan tensörü ya da `yok` (gradyan yok). Tensör olmayan girdilerin öğesi `yok` olmalıdır; türev izlemeyen tensör girdileri için verilen gradyan yok sayılır. Adlı argümanlar (sayılar, metinler...) `ileri`ye olduğu gibi geçer ve gradyan almaz. `geri`, `geri_yayılım`'ı çağıran görevle ve onun `karışık_hassasiyet` durumuyla çalışır; `karışık_hassasiyet` bloğu içinde çağrılan `ileri` bloğun türüyle hesaplar. Gradyanlar girdilerin gradyan türünde birikir (16 bitlik girdilerde `float32`, 15.12). `ileri` ve `geri`nin türevsiz çalışması da göreve bağlıdır (15.6): kilidi bıraktıklarında başka görevlerin hesabı etkilenmez.

**15.22** Çok çıktı: `ileri`nin çıktısı bir tensör listesiyse çağrı aynı sırada bir tensör listesi döndürür ve `geri`nin `g`'si her çıktı için bir gradyan içeren listedir. Kayba bağlı olmayan (gradyan almayan) çıktının gradyanı `yok` değil, çıktıyla aynı şekilde **sıfır tensördür**; böylece `geri` denetim yapmadan aritmetik kullanabilir. Çıktılar çizgede tek bir ortak düğüme bağlıdır: `geri` bütün çıktıların gradyanları toplandıktan sonra bir kez çalışır; aynı çizgeden ikinci geri yayılım (başka bir çıktı üzerinden de) hatadır.

**15.23** Hatalar Türkçedir ve `dene/yakala` ile yakalanır: `ileri` biçimi yanlışsa çağrı satırında `özel_türev: ileri [çıktı, saklananlar] döndürmeli ...`; `geri` liste döndürmezse, öğe sayısı girdi sayısından farklıysa (`geri 1 öğeli liste döndürdü ama işlemin 2 konumlu girdisi var ...`), tensör olmayan girdiye gradyan verilirse ya da şekil girdiyle aynı değilse (`geri 1. girdi için [] şeklinde gradyan döndürdü, beklenen [1, 2] (girdinin şekli); ... yayınlama yapılmaz`) `geri_yayılım` satırında hata oluşur. Saklanan bir tensör ileri hesaptan sonra yerinde değiştirilirse (ör. indeksle atama, iyileştirici adımı) geri yayılım, diğer işlemlerdeki gibi "yerinde değiştirildi" hatası verir. `geri`nin içindeki bir hata kendi satır bilgisiyle gelir. `ileri:`/`geri:` eksikse, fonksiyon değilse ya da `geri` iki parametre almıyorsa `özel_türev` çağrısı hata verir; bilinmeyen adlı argümanda en yakın ad önerilir.

**15.24** `türev_denetle(f, girdiler, adım: 0.001, tolerans: 0.01)` `f`'nin (özel türevli ya da sıradan TR fonksiyonu) geri yayılımla bulunan gradyanını merkezi farkla bulunan sayısal gradyanla karşılaştırır; `girdiler` `f`'nin konumlu argümanlarının listesidir. Yalnızca türev izleyen tensör girdileri denetlenir; denetim bunların float32 kopyalarıyla yapılır (kullanıcının tensörleri ve gradyanları değişmez). Çıktı tek tensör ya da tensör listesi olabilir; skaler olmayan çıktılar sabit tohumlu rastgele bir yön vektörüyle iç çarpımla skalere indirgenir, kayıp ve farklar float64'te hesaplanır. Her eleman için `|analitik - sayısal| ≤ tolerans · max(1, |analitik|, |sayısal|)` aranır; sağlanırsa `doğru` döner, sağlanmazsa en büyük farkın girdisini, elemanını ve iki değeri yazan bir hata oluşur (`türev_denetle: analitik (geri yayılım) ve sayısal gradyan uyuşmuyor; en büyük fark 1. girdinin [0, 1] elemanında: analitik ..., sayısal ...`). Hiçbir girdi türev izlemiyorsa, `adım`/`tolerans` pozitif değilse ya da `türevsiz` içinde çağrılırsa hatadır. Her eleman için `f` iki kez çalıştığından küçük girdilerle kullanılır.

---

## 16. Veri ve parçalayıcı

Model eğitimi için `tokenizer.json` parçalayıcıları ve `.npy` veri dosyaları yerleşik nesnelerle okunur. Ayrıntılı kullanım için `KULLANIM_KILAVUZU.md` §4.E'ye bakın.

**16.1** `parçalayıcı_yükle(yol)` bir `Parçalayıcı` nesnesi döndürür. `.dağarcık_boyutu` dağarcıktaki parça sayısıdır (`Tam`); `.parça_kimliği(parça)` parçanın kimliğini, dağarcıkta yoksa `yok` verir. Eski adlar (`sözlük_boyutu`, `token_kimliği`, `özel_tokenlar:`) kabul edilmez; hata yeni adı önerir.

**16.2** `.kodla(metin)` bir `Tam` listesi, `.kodla(metin_listesi)` her metin için bir liste (sıra korunur) döndürür. Metindeki özel parça yazımları tek kimlik olarak tanınır. Adlı seçenekler: `özel_parçalar:` (varsayılan `doğru`; `yanlış` son işleyicinin eklediği özel parçaları kapatır) ve `ikinci:` (çift kodlama; listeyle eşit uzunlukta liste).

**16.3** `.çöz(kimlikler)` kimlik listesinden ya da 1 boyutlu tensörden metin, listelerin listesinden ya da 2 boyutlu tensörden metin listesi üretir. Varsayılan olarak özel parçalar çıktıya yazılmaz; `özel_parçalar: doğru` onları da yazar. Dağarcıkta olmayan kimlikler atlanır.

**16.4** `veri_kümesi(dizin)` bir klasördeki `.npy` veri dosyalarını (ad sırasıyla) ya da tek bir `.npy` dosyasını `VeriKümesi` olarak açar; `meta.json` varsa parça sayıları ondan okunur. `önbellek: klasör` veri dosyalarını yerel bir önbelleğe kopyalar; bu durumda `kota_gb:` zorunludur. Aynı önbellek klasörünü kullanan kümeler tek kotayı paylaşır (en son verilen geçerlidir); klasörü aynı anda başka bir program kullanıyorsa hata verilir.

**16.5** `.örnekleyici(toplu:, bağlam:, tohum: 0)` bir `Örnekleyici` döndürür. `.sonraki()` `[girdi, hedef]` listesidir: ikisi de `[toplu, bağlam]` şeklinde tam sayılı işaretli tensördür (`.değer` `Tam` listesi verir; işlem sonuçlarına bu işaret geçmez), `hedef` girdinin bir kaydırılmışıdır ve pencereler veri dosyası sınırını aşmaz. Aynı küme ve tohum aynı toplu işleri verir; örnekleyici sonsuz üretir.

**16.6** `.sıralı(toplu:, bağlam:)` bir `SıralıOkuyucu` döndürür: pencereler veri dosyası sırasıyla ve `bağlam` adımla ardışıktır; son toplu iş daha az satırlı olabilir; veri bitince `.sonraki()` `yok` döner.

**16.7** Hatalar (dosya yok, `.npy` değil, eksik ya da hatalı adlı argüman, bağlam veri dosyalarından uzun, düzenli ifade adım sınırı...) Türkçe bir mesajla bildirilir ve `dene/yakala` ile yakalanabilir.

**16.8** Bu yerleşiklerde göreli yollar `dosya_oku`/`dosya_yaz` ile aynı tek kuralla (bkz. **13.1**) çözülür: önce çalışma klasörü, orada yoksa programın bulunduğu klasör.

Önden yükleme her zaman açıktır ve yorumlayıcı kilidini tutmaz; `.sonraki()` beklerken kilit bırakılır (10.3). Açık bir kapatma adı yoktur: sıralı okuyucu bitince, kullanılmayan nesneler çöp toplanınca ve program bitince dosyalar kapatılır.

---

## 17. Kodlar

Dilin kullanıcıya görünen her adı (anahtar kelimeler, bağlamsal sözcükler, tip adları, yerleşik fonksiyonlar, metot ve özellikler, adlı argümanlar) sabit bir **koda** bağlıdır. Kodların listesi (kısaca **kodlar**) iki dosyadır: dilden bağımsız tanımlar `pkg/kod/tanim.json`'da, adlar ve açıklamalar dil dosyasında, Türkçe için `pkg/kod/tr.json`'dadır (ikisi de ikiliye gömülüdür, `tr` tek dosya olarak dağıtılır). **Her kodun ne işe yaradığı, imzası ve örneği [`docs/KODLAR.md`](KODLAR.md)'dedir**; bu belge iki dosyadan üretilir (`go run ./araclar/kod_belgesi`) ve "hangi kod ne işe yarar" sorusunun tek başvurusudur. Yorumlayıcının Go kodu adları değil kodları kullanır (`kod.G100`; adlı sabitler dil dosyasından üretilir: `go generate ./pkg/kod`). Bir adı değiştirmek için yalnızca dil dosyasındaki satır değişir; kodlar hiç değişmez. Başka diller aynı kodlara kendi adlarını verecektir (`en.json`, `pt.json`; dil kodları ISO 639-1, gerekirse bölgeli: `pt-BR`).

Kod bir harf ve üç basamaklı bir sayıdır; harf aileyi, yüzler basamağı aile içindeki grubu gösterir. Gruplar arasında sonradan ekleme için boşluk bırakılmıştır.

| Aile | Kapsam | Gruplar |
| :-- | :-- | :-- |
| `A` | Dilin çekirdeği | `A0xx` anahtar kelimeler (`A001` eğer, `A010` döngü, `A020` fonksiyon ...), `A1xx` bağlamsal sözcükler (`olarak`, `test`, `kurucu`, `hata`, `o`), `A2xx` tip adları (`A200` Tam ... `A23x`-`A24x` motor tipleri) |
| `G` | Genel yerleşikler | `G1xx` giriş, çıkış, dosya (`G100` yazdır, `G101` girdi) · `G2xx` değer, tip, dönüşüm (`G201` tip) · `G3xx` liste · `G4xx` matematik ve istatistik · `G5xx` zaman ve eşzamanlılık · `G6xx` JSON/CSV · `G7xx` test |
| `T` | Tensör ve yapay zekâ | `T1xx` oluşturma ve görünüm (`T100` tensör) · `T2xx` etkinleştirme ve eleman bazlı matematik · `T3xx` türev · `T4xx` iyileştiriciler · `T5xx` katmanlar, kayıplar, hazır model · `T6xx` vektör yardımcıları ve dil modeli · `T7xx` kaydetme, yükleme, veri türleri · `T9xx` adlı argümanlar |
| `V` | Veri ve parçalayıcı | `V1xx` veri kümesi · `V2xx` parçalayıcı · `V9xx` adlı argümanlar |
| `W` | Web, veritabanı, arayüz | `W1xx` web istemcisi ve sunucusu · `W2xx` veritabanı · `W3xx` görsel arayüz · `W9xx` adlı argümanlar |
| `Y` | Yöntemler: metot ve özellikler (`x.ad`) | `Y0xx` birden çok tipte ortak (`uzunluk`, `içerir` ...) · `Y1xx` Metin · `Y2xx` Liste · `Y3xx` Sözlük · `Y4xx` Tensör · `Y5xx` iyileştirici ve SinirAğı · `Y6xx` veri kümesi, okuyucu, parçalayıcı · `Y7xx` veritabanı · `Y8xx` web sunucusu · `Y9xx` görsel arayüz |
| `H` | Hata mesajları | ayrılmış; henüz kullanılmıyor |

**Dil dosyası** (`tr.json`): her kod bir satırdır ve kodlar artan sıradadır:

```json
"G100": { "ad": "yazdır", "eski": ["yaz"], "aile": "giriş ve çıkış", "açıklama": "Değerleri ... yazar ...", "örnek": "yazdır(\"Toplam:\", 2 + 3)" },
```

- `ad` kodun bu dildeki tek adıdır; kabul edilen tek yazım budur (bkz. 2.9). Türkçe karakterleri ASCII'ye çevrilmiş biçimi (`yazdir`) dosyada saklanmaz ve **kabul edilmez**; yalnızca hata mesajı için addan türetilir. Bu kural `pkg/kod`'da tek bir açma-kapama noktasından, `kod.ASCIIKabul` sabitinden yönetilir (kapalı). İpucunu `kod.Oneri` ve `AsilAdAlanda` verir; ikisi de yazılan adın ad alanına bakar (aynı ASCII yazım genel adlarda bir eski ada, üyelerde başka bir adın ASCII biçimine karşılık gelebilir) ve her zaman Türkçe adı önerir.
- `eski` kaldırılmış adlardır (bkz. 2.9a): kabul edilmezler, yalnızca hata mesajında `ad` önerilir; eski bir adın ASCII biçimi de (`islev`) aynı ipucunu verir. Bir ad değiştirildiğinde ya da kaldırıldığında eski ad bu listeye eklenir ve aynı ipucu kendiliğinden çalışır. Eş yazımlar için ayrı bir alan yoktur: her işin tek adı vardır.
- `ascii: false`, `ASCIIKabul` açılsa bile o adın ASCII biçimini kapalı tutar (bugün yalnızca birkaç adlı argüman); `ASCIIKabul` kapalıyken etkisizdir. `aile` insanlar için kısa bir grup adıdır.
- `açıklama` kodun ne yaptığını ve önemli bir kısıtını söyleyen bir-iki cümledir; `örnek` tek satırlık, kendi başına çalışan bir TR kodudur. Her kodun açıklaması ve her canlı kodun örneği vardır; örnekler `tr kontrol`'den geçer ve çalışır (`go test ./pkg/kod ./pkg/motor`).

**Tanımlar** (`tanim.json`, dilden bağımsız): her kodun türü (`anahtar_kelime`, `bağlamsal`, `tip`, `fonksiyon`, `metot`, `özellik`, `adlı_argüman`), metot ve özelliklerde geçerli olduğu tipler (tip kodlarıyla; tip adı bir oluşturucunun adı olan `AdamW` ve `SGD` için oluşturucunun kodu `T400`, `T401`), imzası ve ilişkili kodları:

```json
"G400": { "tür": "fonksiyon", "param": ["A205|A212", "A200?"], "adlı": [["T902", "A200", null], ["T906", "A203", false]], "dönüş": "A200|A201|A212", "ilişkili": ["G410"] },
"Y103": { "tür": "metot", "imzalar": { "A202": { "param": ["A202?"], "dönüş": "A205" }, "A212": { "param": ["A200", "A200?"], "adlı": [["T902", "A200", -1]], "dönüş": "A205" } } },
```

İmzada parametre adı yazılmaz (adlar dile bağlıdır): konumlu parametreler sırayla tip ifadeleridir (`A200|A201` Tam ya da Ondalık; sonek `?` isteğe bağlı, `*` sıfır ya da çok, `+` bir ya da çok), adlı argümanlar `[kod, tip, varsayılan]` üçlüsüdür. Tanımlar davranışla sınanır: tanımdaki adlı argümanlar yerleşiğin ya da metodun gerçekten kabul ettikleridir (bilinmeyen adla çağrı, geçerli adları sayan hatayı verir, bkz. 13.5) ve metot ve özelliklerin tip listesi, üyeyi gerçekten destekleyen tiplerdir.

**Taşınan kodlar.** Bir işin biçimi değiştiğinde (fonksiyondan metoda ya da metottan fonksiyona, bkz. 13.6) yeni biçim yeni bir koddur; eski kodun satırı iki dosyada da kalır, numarası bir daha kullanılmaz. `tanim.json`'da eski kod yalnızca `"taşındı"` ile yeni kodu gösterir (`"G200": { "tür": "fonksiyon", "taşındı": "Y001" }`), dil dosyasında adını, eski adlarını ve açıklamasını tutar. Taşınan kodun adı (ve eski adları, ASCII biçimleri) kabul edilmez, sıradan bir addır; yazılınca hata yeni biçimi gösterir (`'uzunluk(...)' TR'de bir fonksiyon değil; 'x.uzunluk' yazın`). Taşınma ad alanını değiştirmelidir (genel ad ↔ üye) ve yeni kod canlı olmalıdır; dil dosyası yüklenirken taşınan bir ad canlı bir adla ya da eski bir adla aynı ad alanında çakışırsa reddedilir. Go'da taşınan kodların sabiti yoktur (`kod.G200` derlenmez).

Bir ad yalnızca kendi ad alanında benzersiz olmalıdır: genel adlar (anahtar kelimeler, bağlamsal sözcükler ve yerleşikler), tip adları, üyeler ve adlı argümanlar ayrı ad alanlarıdır; bu yüzden `sırala` hem bir liste metodu (`Y202`) hem bir adlı argümandır (`W931`). Üye adları tipten bağımsız benzersizdir. Dil dosyası yüklenirken yinelenen kod, aynı ad alanında çakışan ad (ipucu belirsiz kalmasın diye ASCII biçimleri aynı olan iki ad da çakışır) ve geçerli bir adla çakışan eski ad reddedilir; `go test ./pkg/kod ./pkg/motor` kayıtlı her yerleşiğin ve üyenin kod listesinde kodu olduğunu, kod listesindeki her kodun da karşılandığını denetler. Adlar başlangıçta ve derleme anında koda çözülür; çalışma anında ad → kod araması yapılmaz.
