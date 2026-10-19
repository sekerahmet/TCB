# 2. Sözcük yapısı

**2.1** Kaynak UTF-8 metindir; bazı Windows editörlerinin eklediği baştaki BOM (U+FEFF) yok sayılır. Konumlar 1'den başlayan satır ve karakter (bayt değil) sütunlarıyla bildirilir. Kaynağın ortasındaki NUL karakteri (U+0000) sözdizimi hatasıdır (metin dosyasında bulunmaz; ardından gelen kod sessizce yok sayılmaz). Dosya kodlu biçimde de saklanabilir (17.1).

**2.2** Yorumlar iki biçimdedir: satır sonuna kadar `//` ve çok satırlı `/* ... */`. Kapanmayan `/*` bir hatadır. `#` yorum değildir: metin dışında yazılan `#` bir sözdizimi hatasıdır (`'#' TR'de yorum değil; yorum için '//' yazın`); metin içinde (`"#22c55e"`) sıradan bir karakterdir.

```tr
değişken x = 1  // satır sonuna kadar yorum
/* çok
   satırlı */
```

**2.3** Tanımlayıcılar bir harf ya da `_` ile başlar, harf, rakam ve `_` ile sürer; Türkçe harfler (ç ğ ı İ ö ş ü) dahil tüm Unicode harfleri geçerlidir: `öğrenci_sayısı2`.

**2.3a** Kod biçimindeki adlar ayrılmıştır: bir büyük harf ve üç rakamdan oluşan ad (`G100`, `A001`, `K001`) dilin kodlarının yazımıdır (17) ve kullanıcı adı olarak kullanılamaz; Türkçe yazılmış dosyada sözdizimi hatası verir: `'G100' kod biçiminde bir ad (büyük harf ve üç rakam); bu biçimdeki adlar dilin kodları için ayrılmıştır, başka bir ad seçin (ör. G100_ ya da g100). Kodlu dosyalar ilk satırdaki '//! tr kodlu 1' başlığıyla başlar`. Kodlu dosyada (17.1) bu adlar dilin sözcükleridir; `g100`, `G1000`, `G100_` sıradan adlardır.

**2.4** Tam sayı sabitleri ondalık rakamlardan oluşur; okunabilirlik için `_` kullanılabilir: `1_000_000`.

**2.5** Ondalık sabitler nokta içerir ve noktadan sonra rakam gelmelidir: `3.14`. (`1..5` bir aralıktır, ondalık değil.) Bilimsel yazım da Ondalık verir: üs harfi `e`/`E`, isteğe bağlı `+`/`-` ve en az bir rakam (`1e-5`, `3.2E+4`, `2e10`); üsten sonra rakam yoksa `e` sayıya ait değildir (`2e` → `2` ve `e` adı).

**2.6** Metinler çift (`"..."`) ya da tek (`'...'`) tırnakla yazılır; `\n \t \r \" \' \\ \{ \}` kaçış dizileri tanınır. Bilinmeyen bir kaçış dizisi sözdizimi hatasıdır, ters bölü sessizce atılmaz (`"\d+"`, `"C:\veri"` → `bilinmeyen kaçış dizisi '\d' ...; ters bölüyü olduğu gibi yazmak için ham metin kullanın ya da ters bölüyü ikileyin`): düzenli ifadeler ve Windows yolları için ham metin (2.6a) ya da `\\` yazılır. Metin birden çok satıra yayılabilir; metnin içindeki satır sonu kaynağın satır sonu biçiminden bağımsız olarak `\n`dir (CRLF'li bir dosyada metnin değerine `\r` girmez; `\r` yalnızca kaçışla yazılır). `tr biçimle` metinlerin içindekiler dahil bütün satır sonlarını LF yazar.

**2.6a** Ham metin ters tırnakla (`` `...` ``) yazılır: içinde kaçış dizisi ve `{ifade}` yerleştirmesi (2.7) yoktur, ters bölü, tırnaklar ve süslü parantezler olduğu gibi kalır. Düzenli ifadeler, Windows yolları ve yerleştirme istenmeyen `{` içeren metinler için önerilen yazımdır. Ham metin de `Metin` değeridir ve birden çok satıra yayılabilir (satır sonu yine `\n`dir); içine ters tırnak yazılamaz (o zaman tırnaklı metin kullanılır). Kapanmayan ham metin sözdizimi hatasıdır. `tr biçimle` ham metni olduğu gibi korur.

```tr
değişken yol = `C:\veri\tokenizer.json`
değişken tarih = `\d{4}-\d{2}-\d{2}`     // "\\d\{4\}-\\d\{2\}-\\d\{2\}" ile aynı
```

**2.7** Metin içindeki `{ifade}` yerleştirmedir: ifade hesaplanıp metne yazılır. Yerleştirme aynı satırda kapanmalıdır; düz süslü parantez için `\{` ve `\}` yazılır (ya da ham metin, 2.6a). Tırnaklı metinde kaçışsız her `{` bir yerleştirme başlatır: içi tek bir ifade olarak ayrıştırılamazsa (ya da boşsa) sözdizimi hatasıdır, düz metne dönülmez (`"{değil}"` → `metindeki her '{' bir yerleştirme başlatır, ama '{değil}' bir ifade değil ... Düz süslü parantez için '\{' yazın ya da ham metin kullanın`). Böylece bir parçanın kod mu metin mi olduğu dilin anahtar kelimelerine bağlı değildir; JSON gibi süslü parantezli metinler ham metinle yazılır: `` `{"ad": "Ali"}` ``. Yerleştirmenin içindeki tırnaklı bir metinde süslü parantez yerleştirmeyi açıp kapatmaz (`"x {"a}b"} y"` → `x a}b y`). Yerleştirmedeki ifadenin sözcükleri dosyadaki gerçek satır ve sütunlarıyla bildirilir (önündeki kaçış dizileri konumu kaydırmaz).

```tr
değişken ad = "Ali"
değişken p = {"ad": "Ayşe"}
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
- Yerleşik fonksiyonlar: `yaz` → `yazdır`, `oku` → `girdi`, `tur` → `tip`, `en_büyük` → `maks`, `en_küçük` → `min`, `uyut` → `bekle`, `std_sapma` → `standart_sapma`, `medyan` → `ortanca`, `json_yaz` → `json_metin`, `tablo_oku` → `csv_oku`, `matris` → `tensör` (matris 2 boyutlu tensördür: `tensör([[1, 2], [3, 4]])`), `rastgele_matris` → `rastgele_normal` (`rastgele_normal([satır, sütun], tohum: 7)`).
- Metot ve özellikler: `.boy` → `.uzunluk`, `.dönüştür` / `.donustur` → `.haritala`, `.türev` → `.gradyan`, `.kayıt_ekle` → `.ekle`; parçalayıcıda `.sözlük_boyutu` → `.dağarcık_boyutu`, `.token_kimliği` → `.parça_kimliği`.
- Biçim değiştirenler (13.6): `uzunluk(x)` (ve `boy(x)`) → `x.uzunluk`, `filtrele(l, f)` → `l.filtrele(f)`, `haritala(l, f)` (ve `donustur`) → `l.haritala(f)`, `indirge(l, f, b)` (ve `ozetle`) → `l.indirge(b, f)`, `sırala(l)` → `l.sırala()`, `ekle(l, x)` → `l.ekle(x)`, `devrik(t)` → `t.devrik()`; `l.toplam()` → `toplam(l)`, `l.ortalama()` → `ortalama(l)`. Bu adlar yazılınca hata yeni biçimi gösterir: `'uzunluk(...)' TR'de bir fonksiyon değil; 'x.uzunluk' yazın`, `'liste.toplam()' TR'de yok; 'toplam(liste)' yazın`.
- Adlı argümanlar: `öğrenme_hızı:` → `öğrenme_oranı:`, `özel_tokenlar:` → `özel_parçalar:`.
- Dilden kaldırılan özellikler (yerine geçen yok): Türkçe SQL veritabanı (`veritabanı_aç`, `tablo_sorgula` ve veritabanı metotları), web istemcisi ve sunucusu (`web_al`, `web_gönder`, `web_sunucu`), görsel arayüz (`arayüz_oluştur`), hazır sinir ağı (`sinir_ağı`, `model_yükle`, `.eğit`, `.tahmin_et`, `.ağırlıklar`, `kayıp_mse`), yerel dil modeli istemcisi (`yz_sor`) ve liste vektör yardımcıları (`normalize_et`, `nokta_çarpım`, `kosinüs_benzerliği`, `öklid_uzaklığı`). Bu adlar (ve eski adları) yazılınca hata `'web_sunucu' TR'den kaldırıldı` der; adlar sıradan adlardır, kodları bir daha kullanılmaz (bkz. 17). Model tensörlerle kurulur (15, `examples/05_tensor_ve_turev.tr`).
- Tip adları (bkz. 3.13): `tamsayı` / `int` → `Tam`, `sayı` / `float` → `Ondalık`, `yazı` / `str` / `string` → `Metin`, `mantıksal` / `bool` → `Mantık`, `boş` / `null` → `Yok`, `dizi` / `list` → `Liste`, `harita` / `dict` / `map` → `Sözlük`, `matris` / `matrix` → `Tensör`.
- Yorum: `#` → `//` (bkz. 2.2).
- Komut satırı komutlarının da tek Türkçe adı vardır (`tr debug` → `tr hata_ayıkla`, bkz. `docs/guide/01_baslangic.md` §1).

Kaldırılan dil adları (anahtar kelimeler, yerleşikler, üyeler, adlı argümanlar, tip adları) kod listesinde asıl adın `eski` listesinde, biçim değiştiren kodlar ise kendi satırlarında "taşındı" bilgisiyle durur (bkz. 17); ipucu oradan kendiliğinden çalışır.

**Yerleşik ad gölgeleme uyarısı.** Bir yerleşiğin adı (`toplam`, `girdi`, `metin`, `tip` ...) değişken, fonksiyon, parametre, döngü değişkeni, `yakala` değişkeni ya da yapı adı olarak tanımlanabilir; o kapsamda ad kullanıcının tanımını gösterir ve yerleşik kullanılamaz. Tip denetleyicisi bunun için bir **uyarı** verir; uyarı hata değildir, program çalışır ve çıkış kodu değişmez. Uyarılar standart hataya, tip hatalarından ayrı, `⚠️ Uyarılar:` başlığıyla yazılır: `Satır 1: 'toplam' yerleşik bir ad; bu kapsamda yerleşik 'toplam' artık kullanılamaz`. Gölgelenmiş ad yerleşik gibi çağrılırsa çalışma zamanı hatası nedenini söyler: `Satır 2: 'tip' bir fonksiyon değil (Tam); 1. satırda bir değişkene verildi, yerleşik 'tip' bu kapsamda gölgelendi`. Yapı alanları ve metotları (`kendi.toplam`, `p.toplam()`) gölgeleme sayılmaz; metot adları (`uzunluk`, `sırala`, `ekle` ...) yerleşik değildir, değişken adı olarak serbestçe kullanılır.

**2.10** İfadeler ve bloklar en fazla 1000 düzey iç içe yazılabilir (`((((1))))` dört düzeydir); sınır aşılırsa program çalışmaz, `ifade ya da blok çok derin iç içe (en fazla 1000 düzey)` sözdizimi hatası verilir. Metin içindeki yerleştirmenin ifadesi (2.7) de bulunduğu yerin düzeyinden sayılır. (Sınırsız iç içelik yorumlayıcıyı yakalanamaz biçimde çökertirdi.)

**2.11** `$ad` kaçışı: `$` ve ardından gelen ad her zaman kullanıcının adıdır, dilin bir sözcüğüyle aynı olsa bile (`değişken $eğer = 3`, `yazdır($eğer)`; ad `eğer`'dir). Kaçış, kodlu bir dosya (17) bir yazım dilinde gösterilirken o dilin sözcükleriyle çakışan kullanıcı adları içindir: gösterimde yalnızca gerektiği yerde yazılır (bir anahtar kelimeyle aynı ad, yazılsaydı dilin bir öğesine bağlanacak ad ya da kodla yazılmış bir sözcüğü kendi kapsamında yakalayacak ad), kodlama `$`'ı atar. Türkçe yazımda kullanıcı adları dilin sözcükleriyle çakışamadığı için kaçış Türkçe gösterimde ancak başka bir dilde yazılmış kodda görünür. `$` yalnızca bu kaçış içindir; bir adın ya da sayının hemen ardından yazılamaz (`a$b`, `1$x` sözdizimi hatasıdır: `$` atılınca iki sözcük birleşirdi). Kod biçimli adlar kaçışla da ayrılmıştır (2.3a).

Bağlamsal sözcükler anahtar kelime değildir ve ad olarak kullanılabilir: `test` (yalnızca deyim başında ardından metin gelince, bkz. 12.2), `olarak` (yalnızca `aktar` içinde), `kurucu` (yapının oluşturulurken çağrılan metodunun adı, 7.4), `hata` (`yakala`da ad verilmeyince hata değişkeninin adı, 8.1), `o` (sondaki bloğun parametresi, 6.7), `birikim` (yalnızca `indirge`'nin sondaki bloğunda önceki sonuç, 6.7), `_` (yalnızca `eşle` kolunda varsayılan). Bunların adları da kod listesindedir (`A1xx`, bkz. 17).
