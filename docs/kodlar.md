# TR kod listesi

<!-- Bu dosya internal/dil/kod/tanim.json ve internal/dil/kod/tr.json'dan üretilir; elle düzenlemeyin: go run ./bakim/uret/kod_belgesi -->

Dilin kullanıcıya görünen her adı sabit bir **koda** bağlıdır (bkz. `docs/dil/17_kodlar.md` §17). Bu belge her kodun ne işe yaradığını, imzasını ve örneğini verir; "hangi kod ne işe yarar" sorusunun tek başvurusudur. Yeni bir ad gerekmeden önce burada aynı işi yapan bir kod olup olmadığına bakın.

İmzalarda parametrelerin yalnızca tipleri yazılır: `[Tip]` isteğe bağlı, `[Tip...]` sıfır ya da çok, `Tip...` bir ya da çok argümandır; `ad: Tip = varsayılan` adlı argümandır, `→` dönüş tipidir. Metot ve özellikler `Tip.ad` biçiminde, geçerli oldukları her tip için ayrı satırda yazılır.

**Değer dönüştüren ya da değere soru soran iş metottur** (`liste.sırala()`, `metin.uzunluk`); **matematik ve tensör işi fonksiyondur** (`toplam(liste)`, `relu(x)`). Kaldırılan kodların numarası bir daha kullanılmaz; adları yazılınca hata yeni biçimi önerir.

Toplam 185 kod: 28 anahtar kelime, 5 bağlamsal sözcük, 18 tip, 62 fonksiyon, 32 metot, 9 özellik, 31 adlı argüman; ayrıca 9 taşınmış ve 64 kaldırılmış kod (numaraları bir daha kullanılmaz).

## İçindekiler

- A — Dilin çekirdeği: Anahtar kelimeler (A0xx) · Bağlamsal sözcükler (A1xx) · Tip adları (A2xx)
- G — Genel yerleşikler: Giriş, çıkış ve dosya (G1xx) · Değer, tip ve dönüşüm (G2xx) · Liste (G3xx) · Matematik ve istatistik (G4xx) · Zaman ve eşzamanlılık (G5xx) · Veri biçimleri (G6xx) · Test (G7xx)
- T — Tensör ve yapay zekâ: Tensör oluşturma ve görünüm (T1xx) · Etkinleştirme ve eleman bazlı matematik (T2xx) · Türev (T3xx) · İyileştiriciler (T4xx) · Katmanlar, kayıplar, hazır model (T5xx) · Vektör yardımcıları ve dil modeli (T6xx) · Kaydetme, yükleme, veri türleri (T7xx) · Adlı argümanlar (T9xx)
- V — Veri ve parçalayıcı: Veri kümesi (V1xx) · Parçalayıcı (V2xx) · Adlı argümanlar (V9xx)
- W — Web, veritabanı, arayüz (kaldırıldı): Web istemcisi ve sunucusu (W1xx) · Veritabanı (W2xx) · Görsel arayüz (W3xx) · Adlı argümanlar (W9xx)
- Y — Metot ve özellikler (x.ad): Birden çok tipte ortak (Y0xx) · Metin (Y1xx) · Liste (Y2xx) · Sözlük (Y3xx) · Tensör (Y4xx) · İyileştirici (Y5xx) · Veri kümesi, okuyucu, parçalayıcı (Y6xx) · Veritabanı (Y7xx) · Web sunucusu (Y8xx) · Görsel arayüz (Y9xx)
- Boru hattıyla kullanılabilen fonksiyonlar

## A — Dilin çekirdeği

### Anahtar kelimeler (A0xx)

#### A001 `eğer`

Koşul doğruysa bloğu çalıştırır; eğer bir ifadedir, değeri çalışan bloğun son deyimidir (hiçbir dal çalışmazsa yok).

```tr
değişken n = 7; eğer n % 2 == 0 { yazdır("çift") } değilse { yazdır("tek") }
```

İlişkili: A002 `değilse`

#### A002 `değilse`

eğer koşulu tutmadığında çalışacak dalı başlatır; değilse eğer ile yeni bir koşul zincirlenir.

```tr
değişken n = -3; değişken tür = eğer n < 0 { "negatif" } değilse eğer n == 0 { "sıfır" } değilse { "pozitif" }; yazdır(tür)
```

İlişkili: A001 `eğer`

#### A003 `eşle`

Konuyu kollarla sırayla karşılaştırır ve ilk eşleşen kolun sonucunu verir; Aralık deseni içerme sınar, eşleşme yoksa yok döner.

```tr
değişken puan = 95; yazdır(eşle puan { 100 => "tam puan"; 90..100 => "pekiyi"; varsayılan => "diğer" })
```

İlişkili: A004 `varsayılan`

#### A004 `varsayılan`

eşle içinde her değerle eşleşen koldur; hiçbir desen tutmadığında çalışır (_ ile aynı).

```tr
yazdır(eşle 7 { 1, 2 => "küçük"; varsayılan => "büyük" })
```

İlişkili: A003 `eşle`

#### A010 `döngü`

Bir kaynağı gezer (döngü x içinde ...), koşul doğru oldukça tekrarlar (döngü koşul { }) ya da sonsuz döner (döngü { }); her tur kendi kapsamında çalışır.

```tr
döngü i, x içinde ["a", "b"] { yazdır(i, x) }
```

İlişkili: A011 `içinde`, A012 `iken`

#### A011 `içinde`

döngüde ve liste üretecinde gezilecek kaynağı belirtir; iki değişkenli biçimde indeks ile eleman (sözlükte anahtar ile değer) verir.

```tr
yazdır([x * x döngü x içinde 1..=5 eğer x % 2 == 1])
```

İlişkili: A010 `döngü`

#### A012 `iken`

Koşul doğru oldukça bloğu tekrarlar; döngü koşul { ... } ile aynıdır.

```tr
değişken i = 0; iken i < 3 { i += 1 }; yazdır(i)
```

İlişkili: A010 `döngü`

#### A013 `çık`

En içteki döngüden çıkar; döngü dışında yazılması derleme hatasıdır (fonksiyondan çıkmak için sonuç yazılır).

```tr
döngü x içinde 1..10 { eğer x == 3 { çık }; yazdır(x) }
```

İlişkili: A014 `devam` · Eski adlar: `kır`

#### A014 `devam`

En içteki döngünün sonraki turuna geçer; yalnızca bir döngü gövdesinde geçerlidir.

```tr
döngü x içinde 1..=5 { eğer x % 2 == 0 { devam }; yazdır(x) }
```

İlişkili: A013 `çık`

#### A020 `fonksiyon`

Fonksiyon tanımlar; parametrelere varsayılan değer verilebilir, tanım öne alınmaz (tanımdan önce çağrılamaz).

```tr
fonksiyon topla(a, b = 1) { sonuç a + b }; yazdır(topla(2, 3), topla(5))
```

İlişkili: A021 `sonuç` · Eski adlar: `işlev`

#### A021 `sonuç`

Fonksiyondan değer döndürür; yalnız sonuç yok döndürür, üst düzeyde programı bitirir, modülün üst düzeyinde ve test gövdesinde derleme hatasıdır.

```tr
fonksiyon işaret(n) { eğer n < 0 { sonuç "negatif" }; sonuç "pozitif" }; yazdır(işaret(-2))
```

İlişkili: A020 `fonksiyon` · Eski adlar: `döndür`

#### A030 `değişken`

Bulunulan kapsamda yeni bir ad tanımlar; değer verilmezse yok olur, virgülle birden çok ad tanımlanabilir.

```tr
değişken a, b = 1, 2; a, b = b, a; yazdır(a, b)
```

İlişkili: A031 `sabit`

#### A031 `sabit`

Yeniden atanamayan bir ad tanımlar; ihlal çalıştırmadan önce yakalanır, ama değer bir listeyse içeriği değişebilir.

```tr
sabit PI = 3.14; yazdır(PI * 2)
```

İlişkili: A030 `değişken`

#### A040 `doğru`

Mantık tipinin doğru değeridir.

```tr
değişken hazır = doğru; eğer hazır { yazdır("hazır") }
```

İlişkili: A041 `yanlış`

#### A041 `yanlış`

Mantık tipinin yanlış değeridir; koşullarda yok, 0, boş metin, boş liste ve boş sözlük de yanlış sayılır.

```tr
yazdır(yanlış veya doğru, değil yanlış)
```

İlişkili: A040 `doğru`

#### A042 `yok`

Değerin yokluğudur; değer döndürmeyen fonksiyonlar ve sınır dışı indeksler yok verir.

```tr
değişken l = [1, 2]; yazdır(l[5] == yok, l[5] ?? "boş")
```

İlişkili: A204 `Yok`

#### A043 `ve`

Mantıksal ve (&& ile aynı); kısa devre yapar ve sonucu her zaman Mantık olarak verir.

```tr
yazdır(1 < 2 ve 2 < 3, 1 && "a")
```

İlişkili: A044 `veya`, A045 `değil`

#### A044 `veya`

Mantıksal veya (|| ile aynı); kısa devre yapar ve sonucu her zaman Mantık olarak verir.

```tr
yazdır(yanlış veya 0, 1 > 2 || 3 > 2)
```

İlişkili: A043 `ve`, A045 `değil`

#### A045 `değil`

Doğruluğun tersini verir; karşılaştırmadan gevşek bağlanır: değil 1 == 2, değil (1 == 2) demektir.

```tr
yazdır(değil 1 == 2)
```

İlişkili: A043 `ve`, A044 `veya`

#### A050 `yapı`

Alanları ve metotları olan bir yapı tanımlar; nesne yapının adıyla, alan sırasıyla ya da adlı argümanlarla oluşturulur. Kalıtım yoktur.

```tr
yapı Nokta { x = 0, y = 0 }; yazdır(Nokta(3, 4), Nokta(y: 1).x)
```

İlişkili: A051 `kendi`, A102 `kurucu` · Eski adlar: `sınıf`

#### A051 `kendi`

Yapı metodunun içinde metodun çağrıldığı nesneyi gösterir; alanlar kendi ile okunur ve değiştirilir.

```tr
yapı Sayaç { n = 0; fonksiyon artır() { kendi.n += 1 } }; değişken s = Sayaç(); s.artır(); s.artır(); yazdır(s.n)
```

İlişkili: A050 `yapı`

#### A060 `dene`

Bloktaki hatayı yakala bloğuna yönlendirir; çalışma zamanı hataları da yakalanır.

```tr
dene { fırlat "patladı" } yakala h { yazdır("yakalandı:", h) }
```

İlişkili: A061 `yakala`, A062 `fırlat`

#### A061 `yakala`

dene bloğunda hata olursa çalışır ve hata mesajını verilen ada (ad verilmezse hata adına) Metin olarak bağlar.

```tr
dene { değişken z = 1 / 0 } yakala h { yazdır(h) }
```

İlişkili: A060 `dene`, A103 `hata`

#### A062 `fırlat`

Bir hata başlatır; verilen değer metne çevrilir, yakalayan değişken her zaman Metin'dir.

```tr
dene { fırlat {"kod": 5} } yakala h { yazdır(tip(h), h) }
```

İlişkili: A060 `dene`

#### A070 `aktar`

Başka bir .tr dosyasını program boyunca bir kez çalıştırır ve üst düzey adlarını getirir; yol metin sabiti olmalıdır, .tr uzantısı isteğe bağlıdır.

```tr
aktar "testler/yardimci_modul"; yazdır(kare(PI))
```

İlişkili: A100 `olarak`

#### A080 `eşzamanlı`

Bloğu ya da çağrıyı ayrı bir görev olarak başlatıp hemen devam eder; aynı anda yalnızca bir görev TR kodu çalıştırır.

```tr
eşzamanlı { yazdır("görevde") }; bekle_hepsi(); yazdır("bitti")
```

İlişkili: G501 `bekle_hepsi`

#### A081 `ertele`

Çağrıyı ya da bloğu içinde bulunduğu fonksiyon bitince çalıştırır; birden çok ertele ters sırayla çalışır, erken sonuç ve hata durumunda da çalışır.

```tr
fonksiyon iş() { ertele yazdır("temizlik"); yazdır("iş") }; iş()
```

#### A090 `kesme_noktası`

Programı bu satırda durdurup hata ayıklayıcıyı açar; kesme_noktası eğer koşul biçiminde yalnızca koşul doğruyken durur.

```tr
değişken x = 2; kesme_noktası eğer x > 5; yazdır(x)
```

Eski adlar: `dur_noktası`, `durdurma_noktası`

### Bağlamsal sözcükler (A1xx)

#### A100 `olarak`

aktar ile alınan modülün adlarını tek bir Sözlük olarak verilen ada bağlar.

```tr
aktar "testler/yardimci_modul" olarak ym; yazdır(ym.kare(2), ym["PI"])
```

İlişkili: A070 `aktar` · Eski adlar: `as`

#### A101 `test`

test "ad" { ... } bir test tanımlar; testler yalnızca tr test ile çalışır, normal çalıştırmada atlanır. Deyim başı dışında sıradan bir addır.

```tr
test "toplama" { eşit_mi(4, 2 + 2) }
```

İlişkili: G700 `doğrula`, G701 `eşit_mi`

#### A102 `kurucu`

Bu adlı metot nesne oluşturulurken aynı argümanlarla çağrılır; argümanlar kurucunun imzasına göre denetlenir.

```tr
yapı Hesap { bakiye; fonksiyon kurucu(b) { kendi.bakiye = b * 2 } }; yazdır(Hesap(5).bakiye)
```

İlişkili: A050 `yapı`

#### A103 `hata`

yakala bloğunda ad verilmediğinde hata mesajının bağlandığı değişkendir.

```tr
dene { fırlat "patladı" } yakala { yazdır(hata) }
```

İlişkili: A061 `yakala`

#### A104 `o`

Sondaki blok { ... } tek parametreli bir fonksiyondur ve parametresinin adı o'dur; iç içe bloklarda en içteki elemanı gösterir.

```tr
yazdır([1, 2, 3, 4].filtrele { o > 2 }.haritala { o * 10 })
```

### Tip adları (A2xx)

#### A200 `Tam`

64 bit işaretli tam sayı tipidir; sınırı aşan işlem sessizce sarmak yerine taşma hatası verir.

```tr
değişken x: Tam = 5; yazdır(tip(x), x * 2)
```

İlişkili: A201 `Ondalık`, G210 `tam` · Eski adlar: `tamsayı`, `int`

#### A201 `Ondalık`

64 bit kayan noktalı sayı tipidir; Tam bir değer Ondalık açıklamalı bir ada da atanabilir.

```tr
değişken oran: Ondalık = 0.75; yazdır(tip(oran))
```

İlişkili: A200 `Tam`, G211 `ondalık` · Eski adlar: `sayı`, `float`

#### A202 `Metin`

Değiştirilemez Unicode metin tipidir; uzunluk ve indeks karakter bazındadır.

```tr
değişken ad: Metin = "şeker"; yazdır(tip(ad), ad[-1])
```

Üyeleri: Y001 `uzunluk`, Y002 `içerir`, Y003 `ters_çevir`, Y100 `büyük_harf`, Y101 `küçük_harf`, Y102 `kırp`, Y103 `böl`, Y104 `değiştir`, Y105 `başlar`, Y106 `biter` · İlişkili: G212 `metin` · Eski adlar: `yazı`, `str`, `string`

#### A203 `Mantık`

doğru ve yanlış değerlerinin tipidir.

```tr
değişken hazır: Mantık = doğru; yazdır(tip(hazır))
```

İlişkili: A040 `doğru`, A041 `yanlış` · Eski adlar: `mantıksal`, `bool`

#### A204 `Yok`

yok değerinin tipidir; yok, her tip açıklamalı ada atanabilir.

```tr
değişken d: Yok = yok; yazdır(tip(d))
```

İlişkili: A042 `yok` · Eski adlar: `boş`, `null`

#### A205 `Liste`

Sıralı, karışık tipli, değiştirilebilir dizi tipidir; listeler başvuruyla paylaşılır.

```tr
değişken l: Liste = [1, "iki", 3.0]; yazdır(tip(l), l[-1])
```

Üyeleri: Y001 `uzunluk`, Y002 `içerir`, Y003 `ters_çevir`, Y004 `ekle`, Y200 `filtrele`, Y201 `haritala`, Y202 `sırala`, Y203 `birleştir`, Y204 `benzersiz`, Y207 `indirge`, Y210 `ilk`, Y211 `son` · Eski adlar: `dizi`, `list`

#### A206 `Sözlük`

Eklenme sırasını koruyan anahtar–değer tablosu tipidir; olmayan anahtar yok verir.

```tr
değişken s: Sözlük = {"ad": "Ali"}; yazdır(tip(s), s.ad)
```

Üyeleri: Y001 `uzunluk`, Y002 `içerir`, Y300 `anahtarlar`, Y301 `değerler` · Eski adlar: `harita`, `dict`, `map`

#### A207 `Aralık`

a..b (b hariç) ve a..=b (b dahil) aralıklarının tipidir; uçlar Tam olmalıdır. aralık(...) yerleşiği ise Liste döndürür.

```tr
değişken a: Aralık = 1..5; yazdır(tip(a), tip(aralık(5)))
```

İlişkili: G305 `aralık`

#### A208 `Fonksiyon`

Kullanıcı tanımlı fonksiyonların ve ok fonksiyonlarının tipidir; fonksiyonlar birinci sınıf değerdir.

```tr
değişken f: Fonksiyon = x => x * 2; yazdır(tip(f), f(3))
```

İlişkili: A020 `fonksiyon`, A209 `YerleşikFonksiyon`

#### A209 `YerleşikFonksiyon`

Dilin yerleşik fonksiyonlarının tipidir.

```tr
değişken f: YerleşikFonksiyon = yazdır; f(tip(f))
```

İlişkili: A208 `Fonksiyon`

#### A210 `YapıTanımı`

yapı ile tanımlanan yapının kendisinin tipidir; tip açıklamasında kullanılamaz.

```tr
yapı Nokta { x, y }; yazdır(tip(Nokta) == "YapıTanımı")
```

İlişkili: A050 `yapı`, A211 `Nesne`

#### A211 `Nesne`

Yapıdan oluşturulan nesnelerin genel adıdır; tip() bir nesne için Nesne değil yapının adını verir.

```tr
yapı Nokta { x, y }; değişken p = Nokta(1, 2); yazdır(tip(p), p)
```

İlişkili: A210 `YapıTanımı`

#### A212 `Tensör`

N boyutlu sayı dizisi tipidir (matris 2 boyutlu tensördür); elemanlar varsayılan olarak float32'dir.

```tr
değişken t: Tensör = tensör([[1, 2], [3, 4]]); yazdır(tip(t), t.şekil)
```

Üyeleri: Y103 `böl`, Y400 `şekil`, Y401 `cihaz`, Y402 `tür`, Y403 `gradyan`, Y404 `değer`, Y410 `şekillendir`, Y411 `devrik`, Y412 `eksenleri_sırala`, Y413 `dilim`, Y414 `cihaza`, Y415 `türe` · İlişkili: T100 `tensör` · Eski adlar: `matris`, `matrix`

#### A213 `Herhangi`

Tip denetleyicisinin tipi bilinmeyen değerler için kullandığı tiptir; Herhangi açıklamalı bir ada her değer verilebilir.

```tr
fonksiyon kimlik(x: Herhangi) { sonuç x }; yazdır(kimlik(5), kimlik("beş"))
```

#### A230 `SinirAğı` — kaldırıldı

Kaldırıldı: 'SinirAğı' TR'den kaldırıldı; yerine geçen bir kod yoktur.

#### A231 `VeriKümesi`

veri_kümesi(...) ile açılan, .npy veri dosyalarından oluşan veri kümesinin tipidir.

```tr
değişken küme = veri_kümesi("testler/veri/sayilar"); yazdır(tip(küme) == "VeriKümesi")
```

Üyeleri: Y600 `örnekleyici`, Y601 `sıralı` · İlişkili: V100 `veri_kümesi`

#### A232 `Örnekleyici`

Veri kümesinden tohuma göre rastgele [girdi, hedef] tensör çiftleri üreten okuyucunun tipidir; dönemler boyunca sonsuz üretir.

```tr
değişken ö = veri_kümesi("testler/veri/sayilar").örnekleyici(toplu: 2, bağlam: 4, tohum: 1); yazdır(tip(ö) == "Örnekleyici")
```

Üyeleri: Y602 `sonraki` · İlişkili: Y600 `örnekleyici`, A233 `SıralıOkuyucu`

#### A233 `SıralıOkuyucu`

Veri kümesini pencereler hâlinde sırayla okuyan okuyucunun tipidir; veri bitince .sonraki() yok verir.

```tr
değişken s = veri_kümesi("testler/veri/sayilar").sıralı(toplu: 2, bağlam: 4); yazdır(tip(s) == "SıralıOkuyucu")
```

Üyeleri: Y602 `sonraki` · İlişkili: Y601 `sıralı`, A232 `Örnekleyici`

#### A234 `Parçalayıcı`

parçalayıcı_yükle(...) ile yüklenen, metni parçalara ayırıp parça kimliklerine çeviren nesnenin tipidir.

```tr
değişken p = parçalayıcı_yükle("testler/veri/parcalayici.json"); yazdır(tip(p) == "Parçalayıcı")
```

Üyeleri: Y610 `kodla`, Y611 `çöz`, Y612 `dağarcık_boyutu`, Y613 `parça_kimliği` · İlişkili: V200 `parçalayıcı_yükle`

#### A240 `Veritabanı` — kaldırıldı

Kaldırıldı: 'Veritabanı' TR'den kaldırıldı; yerine geçen bir kod yoktur.

#### A241 `WebSunucu` — kaldırıldı

Kaldırıldı: 'WebSunucu' TR'den kaldırıldı; yerine geçen bir kod yoktur.

#### A242 `GörselArayüz` — kaldırıldı

Kaldırıldı: 'GörselArayüz' TR'den kaldırıldı; yerine geçen bir kod yoktur.

## G — Genel yerleşikler

### Giriş, çıkış ve dosya (G1xx)

#### G100 `yazdır`

    yazdır([Herhangi...]) → Metin

Değerleri aralarına boşluk koyarak tek satırda yazar ve yazdığı metni döndürür; argümansız çağrı boş satır yazar.

```tr
yazdır("Toplam:", 2 + 3)
```

İlişkili: G101 `girdi` · Eski adlar: `yaz`

#### G101 `girdi`

    girdi([Herhangi]) → Metin

Standart girişten bir satır okur, varsa önce istemi yazar; satır sonu atılır, giriş bittiyse boş metin döner.

```tr
değişken ad = girdi("Adınız: "); yazdır("Merhaba " + ad)
```

İlişkili: G100 `yazdır` · Eski adlar: `oku`

#### G110 `dosya_oku`

    dosya_oku(Metin) → Metin

Dosyanın tüm içeriğini Metin olarak okur; göreli yol önce çalışma klasörüne, sonra modülün ve programın klasörüne göre çözülür, dosya yoksa hata bakılan yerleri listeler.

```tr
dosya_yaz("notlar.txt", "merhaba"); yazdır(dosya_oku("notlar.txt"))
```

İlişkili: G111 `dosya_yaz`, G602 `csv_oku`

#### G111 `dosya_yaz`

    dosya_yaz(Metin, Herhangi) → Mantık

Değeri metne çevirip dosyaya yazar ve doğru döndürür; var olan dosyanın üzerine yazılır.

```tr
dosya_yaz("not.txt", "merhaba"); yazdır(dosya_oku("not.txt"))
```

İlişkili: G110 `dosya_oku`

### Değer, tip ve dönüşüm (G2xx)

#### G200 `uzunluk` — kaldırıldı

Taşındı: Y001 `uzunluk` → `x.uzunluk`. Kaldırıldı: uzunluk bir değere sorulan soru olduğundan yalnızca özellik biçimiyle yazılır (x.uzunluk); uzunluk(x) yazılınca hata yeni biçimi önerir.

#### G201 `tip`

    tip(Herhangi) → Metin

Değerin tip adını Metin olarak verir; bir yapı nesnesi için yapının adını verir.

```tr
yazdır(tip(3.5), tip([1, 2]), tip(yok))
```

İlişkili: G210 `tam`, G211 `ondalık`, G212 `metin` · Eski adlar: `tur`

#### G210 `tam`

    tam(Tam | Ondalık | Metin | Mantık) → Tam

Değeri Tam'a çevirir: Ondalık'ın kesirli kısmını atar, metni ayrıştırır, doğru 1 ve yanlış 0 olur; çevrilemeyen metin hatadır.

```tr
yazdır(tam("42") + 1, tam(3.9), tam(doğru))
```

İlişkili: G211 `ondalık`, G212 `metin`

#### G211 `ondalık`

    ondalık(Ondalık | Tam | Metin) → Ondalık

Tam ya da Metin'i Ondalık'a çevirir; metinde ondalık ayırıcı nokta olmalıdır, Mantık kabul edilmez.

```tr
yazdır(ondalık("3.14"), ondalık(2))
```

İlişkili: G210 `tam`, G212 `metin`

#### G212 `metin`

    metin(Herhangi) → Metin

Her değeri yazdırıldığı biçimiyle Metin'e çevirir; hiçbir değerde hata vermez.

```tr
yazdır(metin(12) + "a", metin([1, 2]))
```

İlişkili: G210 `tam`, G211 `ondalık`

### Liste (G3xx)

#### G300 `filtrele` — kaldırıldı

Taşındı: Y200 `filtrele` → `liste.filtrele(...)`. Kaldırıldı: listeyi dönüştüren iş metottur; liste.filtrele { ... } yazılır. filtrele(liste, f) ve liste |> filtrele(f) hata verip yeni biçimi önerir.

#### G301 `haritala` — kaldırıldı

Taşındı: Y201 `haritala` → `liste.haritala(...)`. Kaldırıldı: listeyi dönüştüren iş metottur; liste.haritala { ... } yazılır. haritala(liste, f) hata verip yeni biçimi önerir.

#### G302 `indirge` — kaldırıldı

Taşındı: Y207 `indirge` → `liste.indirge(...)`. Kaldırıldı: listeyi indirgeyen iş metottur; liste.indirge(başlangıç, f) yazılır. indirge(liste, f) hata verip yeni biçimi önerir.

#### G303 `sırala` — kaldırıldı

Taşındı: Y202 `sırala` → `liste.sırala()`. Kaldırıldı: listeyi sıralamak metottur; liste.sırala() yazılır. sırala(liste) hata verip yeni biçimi önerir.

#### G304 `ekle` — kaldırıldı

Taşındı: Y004 `ekle` → `liste.ekle(...)`. Kaldırıldı: listeye eleman eklemek metottur; liste.ekle(x) yazılır. ekle(liste, x) hata verip yeni biçimi önerir.

#### G305 `aralık`

    aralık(Tam, [Tam]) → Liste

aralık(bitiş) 0'dan, aralık(başlangıç, bitiş) başlangıçtan bitişe kadar (bitiş hariç) Tam listesi üretir; Aralık değil Liste döner ve en fazla 50 milyon eleman üretilir.

```tr
yazdır(aralık(5), aralık(2, 5))
```

İlişkili: A207 `Aralık`

### Matematik ve istatistik (G4xx)

#### G400 `toplam`

    toplam(Liste | Tensör, [Tam], eksen: Tam = yok, boyutu_koru: Mantık = yanlış) → Tam | Ondalık | Tensör

Sayı listesini toplar (hepsi Tam ise Tam, bir Ondalık varsa Ondalık); sayı olmayan eleman ve iç içe liste hatadır. Tensörde bütün elemanları ya da eksen: ile verilen ekseni indirger.

```tr
yazdır(toplam([1, 2, 3]), toplam([1, 2.5]), toplam(tensör([[1, 2], [3, 4]]), eksen: 0).değer)
```

İlişkili: G410 `ortalama` · Önceki biçim: `x.toplam()` (Y205, kaldırıldı)

#### G401 `min`

    min(Liste | Tam | Ondalık | Tensör, [Tam | Ondalık...], eksen: Tam = yok, boyutu_koru: Mantık = yanlış) → Tam | Ondalık | Yok | Tensör

Tek bir listenin ya da birden çok sayının en küçüğünü tipini koruyarak verir; boş listede yok döner. Tensörde eksen yalnızca eksen: ile adlı verilir.

```tr
yazdır(min([3, 1, 2]), min(4, 2.5), min([]))
```

İlişkili: G402 `maks` · Eski adlar: `en_küçük`

#### G402 `maks`

    maks(Liste | Tam | Ondalık | Tensör, [Tam | Ondalık...], eksen: Tam = yok, boyutu_koru: Mantık = yanlış) → Tam | Ondalık | Yok | Tensör

Tek bir listenin ya da birden çok sayının en büyüğünü tipini koruyarak verir; boş listede yok döner. Tensörde eksen yalnızca eksen: ile adlı verilir.

```tr
yazdır(maks([2.0, 1.5]), maks(3, 7, 5), maks(tensör([[1, 5], [3, 2]]), eksen: 0).değer)
```

İlişkili: G401 `min`, T230 `maks_indeks` · Eski adlar: `en_büyük`

#### G403 `mutlak`

    mutlak(Tam | Ondalık) → Tam | Ondalık

Sayının mutlak değerini verir; Tam Tam, Ondalık Ondalık kalır, en küçük Tam değerde taşma hatası verir.

```tr
yazdır(mutlak(-5), mutlak(-2.5))
```

İlişkili: G405 `yuvarla`

#### G404 `karekök`

    karekök(Tam | Ondalık | Tensör) → Ondalık | Tensör

Sayının karekökünü Ondalık olarak verir (negatif sayıda NaN); tensörde eleman bazlı ve türev izleyerek çalışır, liste kabul etmez.

```tr
yazdır(karekök(16), karekök(2.25))
```

İlişkili: G403 `mutlak`

#### G405 `yuvarla`

    yuvarla(Tam | Ondalık, [Tam]) → Tam | Ondalık

Sayıyı en yakına yuvarlar (yarımlar sıfırdan uzağa); basamak pozitifse Ondalık, 0 ya da negatifse Tam verir.

```tr
yazdır(yuvarla(3.14159, 2), yuvarla(2.5), yuvarla(1234, -2))
```

İlişkili: G210 `tam`

#### G410 `ortalama`

    ortalama(Liste | Tam | Ondalık | Tensör, [Tam | Ondalık...], eksen: Tam = yok, boyutu_koru: Mantık = yanlış) → Ondalık | Tensör

Sayıların aritmetik ortalamasını Ondalık olarak verir; tek bir liste ya da birden çok sayı alır, boş girdi ve sayı olmayan eleman hatadır. Tensörde eksen: alır, boş tensörün ortalaması hatadır.

```tr
yazdır(ortalama([1, 2, 3, 4]), ortalama(tensör([[1, 2], [3, 4]]), eksen: 1).değer)
```

İlişkili: G400 `toplam`, G412 `ortanca` · Önceki biçim: `x.ortalama()` (Y206, kaldırıldı)

#### G411 `standart_sapma`

    standart_sapma(Liste | Tam | Ondalık, [Tam | Ondalık...]) → Ondalık

Sayıların popülasyon standart sapmasını (n'ye bölerek) Ondalık olarak verir; ortalama gibi tek bir liste ya da birden çok sayı alır, boş girdi ve sayı olmayan eleman hatadır.

```tr
yazdır(standart_sapma([2, 4, 4, 4, 5, 5, 7, 9]))
```

İlişkili: G410 `ortalama` · Eski adlar: `std_sapma`

#### G412 `ortanca`

    ortanca(Liste | Tam | Ondalık, [Tam | Ondalık...]) → Ondalık

Sayıların ortancasını Ondalık olarak verir; eleman sayısı çiftse ortadaki ikisinin ortalamasıdır; ortalama gibi boş girdi ve sayı olmayan eleman hatadır.

```tr
yazdır(ortanca([3, 1, 4, 2]))
```

İlişkili: G410 `ortalama` · Eski adlar: `medyan`

### Zaman ve eşzamanlılık (G5xx)

#### G500 `bekle`

    bekle(Tam | Ondalık) → Mantık

Verilen saniye kadar bekler ve doğru döndürür; beklerken yorumlayıcı kilidini bırakır, öbür görevler ilerler.

```tr
bekle(0.1); yazdır("0.1 saniye geçti")
```

İlişkili: G501 `bekle_hepsi`, G502 `zaman_ms` · Eski adlar: `uyut`

#### G501 `bekle_hepsi`

    bekle_hepsi() → Mantık

Başlatılmış bütün eşzamanlı görevlerin bitmesini bekler ve doğru döndürür; eşzamanlı bloğunun içinden çağrılması hatadır.

```tr
değişken s = 0; döngü i içinde 1..=3 { eşzamanlı { s += i } }; bekle_hepsi(); yazdır(s)
```

İlişkili: A080 `eşzamanlı`, G500 `bekle`

#### G502 `zaman_ms`

    zaman_ms() → Tam

Şimdiki zamanı 1970'ten bu yana geçen milisaniye olarak Tam verir; süre ölçmek için kullanılır.

```tr
değişken baş = zaman_ms(); bekle(0.05); yazdır(zaman_ms() - baş >= 50)
```

İlişkili: G500 `bekle`

### Veri biçimleri (G6xx)

#### G600 `json_çöz`

    json_çöz(Metin) → Herhangi

JSON metnini TR değerine çevirir (nesne Sözlük, dizi Liste olur); anahtar sırası korunur, tam sayı değerli ondalıklar Tam olur.

```tr
değişken v = json_çöz('{"ad": "Ali", "yaş": 30}'); yazdır(v["ad"], v["yaş"] + 1)
```

İlişkili: G601 `json_metin`

#### G601 `json_metin`

    json_metin(Herhangi) → Metin

Değeri boşluksuz JSON metnine çevirir; anahtarlar sözlükteki sırayla yazılır, NaN, sonsuz ya da kendini içeren değer hatadır.

```tr
yazdır(json_metin({"ad": "Ali", "puan": [90, 85]}))
```

İlişkili: G600 `json_çöz` · Eski adlar: `json_yaz`

#### G602 `csv_oku`

    csv_oku(Metin) → Liste

Bir dosya yolunu ya da CSV metninin kendisini satır listelerine çevirir; hücreler yalnızca katı sayı biçimindeyse sayı olur ("007" Metin kalır).

```tr
değişken tablo = csv_oku("ad,yaş\nAli,30"); yazdır(tablo[1][0], tablo[1][1] + 1)
```

İlişkili: G110 `dosya_oku` · Eski adlar: `tablo_oku`

### Test (G7xx)

#### G700 `doğrula`

    doğrula(Herhangi, [Herhangi]) → Mantık

Koşul yanlışsa "doğrulama başarısız" hatası fırlatır, doğruysa doğru döndürür; isteğe bağlı mesaj hataya eklenir. Test dışında da çalışır.

```tr
doğrula(2 > 1, "iki birden büyük olmalı"); yazdır("doğrulandı")
```

İlişkili: G701 `eşit_mi`, A101 `test`

#### G701 `eşit_mi`

    eşit_mi(Herhangi, Herhangi, [Herhangi]) → Mantık

İki değer == anlamında eşit değilse ikisini de tipiyle gösteren hata fırlatır, eşitse doğru döndürür. Test dışında da çalışır.

```tr
eşit_mi(4, 2 + 2); yazdır("eşit")
```

İlişkili: G700 `doğrula`, A101 `test`

## T — Tensör ve yapay zekâ

### Tensör oluşturma ve görünüm (T1xx)

#### T100 `tensör`

    tensör(Tam | Ondalık | Liste | Tensör, [Mantık], türev_izle: Mantık = yanlış, tür: Metin) → Tensör

İç içe sayı listesinden, tek sayıdan ya da başka bir tensörden yeni bir yaprak tensör oluşturur. türev_izle varsayılan olarak yanlıştır; tür verilmezse listeden float32, tensörden kaynağın türü kullanılır.

```tr
değişken t = tensör([[1, 2], [3, 4]], türev_izle: doğru); yazdır(t.şekil, t.tür)
```

İlişkili: T101 `sıfırlar`, T102 `birler`, T103 `rastgele_normal` · Eski adlar: `matris`

#### T101 `sıfırlar`

    sıfırlar(Liste, türev_izle: Mantık = yanlış, tür: Metin = "float32") → Tensör

Verilen şekilde, bütün elemanları 0 olan bir tensör oluşturur. Şekil yalnızca liste olarak verilir; sıfırlar(2, 3) yazımı hatadır ve liste biçimini önerir.

```tr
yazdır(sıfırlar([2, 3]).şekil)
```

İlişkili: T102 `birler`, T100 `tensör`

#### T102 `birler`

    birler(Liste, türev_izle: Mantık = yanlış, tür: Metin = "float32") → Tensör

Verilen şekilde, bütün elemanları 1 olan bir tensör oluşturur. Şekil yalnızca liste olarak verilir ([2, 3]).

```tr
yazdır(birler([2, 2], tür: "bf16").değer)
```

İlişkili: T101 `sıfırlar`, T100 `tensör`

#### T103 `rastgele_normal`

    rastgele_normal(Liste, türev_izle: Mantık = yanlış, tür: Metin = "float32", tohum: Tam) → Tensör

Ortalaması 0, standart sapması 1 olan normal dağılımdan rastgele sayılarla dolu bir tensör oluşturur. Aynı tam sayı tohum her çalıştırmada aynı değerleri verir; şekil liste olarak verilir.

```tr
yazdır(rastgele_normal([2, 3], tohum: 42).şekil)
```

İlişkili: T101 `sıfırlar` · Eski adlar: `rastgele_matris`

#### T111 `birim_matris`

    birim_matris(Tam) → Tensör

n×n birim matrisi (köşegeni 1, geri kalanı 0 olan 2 boyutlu tensör) oluşturur. n pozitif bir Tam olmalıdır.

```tr
yazdır(birim_matris(3).değer)
```

İlişkili: T100 `tensör`

#### T120 `devrik` — kaldırıldı

Taşındı: Y411 `devrik` → `tensör.devrik()`. Kaldırıldı: tensörün görünümünü değiştiren iş metottur; t.devrik() yazılır. devrik(t) hata verip yeni biçimi önerir.

#### T121 `birleştir`

    birleştir(Liste, [Tam], eksen: Tam = -1) → Tensör

Bir tensör listesini verilen eksen boyunca (varsayılan son eksen) tek tensörde birleştirir. Birleşen eksen dışındaki eksenler eşit olmalıdır; sonuç türev izler.

```tr
yazdır(birleştir([birler([2, 1]), sıfırlar([2, 2])], eksen: -1).şekil)
```

İlişkili: Y103 `böl`

### Etkinleştirme ve eleman bazlı matematik (T2xx)

#### T200 `relu`

    relu(Tensör | Liste | Tam | Ondalık) → Tensör | Liste | Ondalık

Negatif elemanları 0 yapan, pozitifleri koruyan eleman bazlı etkinleştirme fonksiyonudur. Tensörde türev izleyen tensör, sayıda Ondalık, listede aynı yapıda liste döndürür; NaN değerini korur.

```tr
yazdır(relu(tensör([-1.0, 0.5, 2.0])).değer)
```

İlişkili: T203 `gelu`, T201 `sigmoid`

#### T201 `sigmoid`

    sigmoid(Tensör | Liste | Tam | Ondalık) → Tensör | Liste | Ondalık

Her elemana 1 / (1 + e^(-x)) uygulayarak değerleri 0 ile 1 arasına sıkıştırır. Tensörde türev izleyen tensör, sayı ya da listede aynı biçimde sonuç döndürür.

```tr
yazdır(sigmoid(tensör([-2.0, 0.0, 2.0])).değer)
```

İlişkili: T202 `tanh`, T200 `relu`

#### T202 `tanh`

    tanh(Tensör | Liste | Tam | Ondalık) → Tensör | Liste | Ondalık

Eleman bazlı hiperbolik tanjanttır; değerleri -1 ile 1 arasına sıkıştırır. Tensörde türev izleyen tensör, sayıda Ondalık, listede liste döndürür.

```tr
yazdır(tanh(0.5), tanh(tensör([-1.0, 1.0])).değer)
```

İlişkili: T201 `sigmoid`

#### T203 `gelu`

    gelu(Tensör | Liste | Tam | Ondalık) → Tensör | Liste | Ondalık

Eleman bazlı GELU etkinleştirmesidir (tanh yaklaşımıyla hesaplanır). Tensörde türev izleyen tensör, sayıda Ondalık, listede aynı yapıda liste döndürür.

```tr
yazdır(gelu(tensör([-1.0, 0.0, 1.0])).değer)
```

İlişkili: T200 `relu`

#### T210 `softmax`

    softmax(Tensör | Liste, [Tam], eksen: Tam = -1) → Tensör | Liste

Verilen eksende (varsayılan son eksen) değerleri toplamı 1 olan olasılıklara çevirir. Düz sayı listesinde liste döndürür, eksen yalnızca tensörde kullanılır; tamamen -sonsuz bir satırın sonucu 0'dır.

```tr
yazdır(softmax(tensör([[1.0, 2.0, 3.0]]), eksen: -1).değer)
```

İlişkili: T211 `log_softmax`, T611 `örnekle`

#### T211 `log_softmax`

    log_softmax(Tensör | Liste, [Tam], eksen: Tam = -1) → Tensör | Liste

softmax'ın logaritmasını verilen eksende (varsayılan son eksen) sayısal olarak kararlı biçimde hesaplar. Düz sayı listesinde liste döndürür; tamamen -sonsuz bir satırın sonucu -sonsuz, gradyanı 0'dır.

```tr
yazdır(log_softmax(tensör([1.0, 2.0, 3.0])).değer)
```

İlişkili: T210 `softmax`, T510 `çapraz_entropi`

#### T220 `üstel`

    üstel(Tensör | Liste | Tam | Ondalık) → Tensör | Liste | Ondalık

Her elemanın e tabanında üstelini (e^x) hesaplar. Tensörde türev izleyen tensör, sayıda Ondalık, listede aynı yapıda liste döndürür.

```tr
yazdır(üstel(tensör([0.0, 1.0])).değer)
```

İlişkili: T221 `log`

#### T221 `log`

    log(Tensör | Liste | Tam | Ondalık) → Tensör | Liste | Ondalık

Her elemanın doğal logaritmasını hesaplar. Tensörde türev izleyen tensör, sayıda Ondalık, listede aynı yapıda liste döndürür.

```tr
yazdır(log(tensör([1.0, 10.0])).değer)
```

İlişkili: T220 `üstel`

#### T222 `sinüs`

    sinüs(Tensör | Liste | Tam | Ondalık) → Tensör | Liste | Ondalık

Her elemanın sinüsünü (radyan) hesaplar; türevi kosinüs(x)'tir. Tensörde türev izleyen tensör, sayıda Ondalık, listede liste döndürür.

```tr
yazdır(sinüs(tensör([0.0, 1.0])).değer)
```

İlişkili: T223 `kosinüs`

#### T223 `kosinüs`

    kosinüs(Tensör | Liste | Tam | Ondalık) → Tensör | Liste | Ondalık

Her elemanın kosinüsünü (radyan) hesaplar; türevi -sinüs(x)'tir. Tensörde türev izleyen tensör, sayıda Ondalık, listede liste döndürür.

```tr
yazdır(kosinüs(tensör([0.0, 1.0])).değer)
```

İlişkili: T222 `sinüs`

#### T230 `maks_indeks`

    maks_indeks(Tensör, eksen: Tam = -1) → Tensör

Verilen eksende (varsayılan son eksen) en büyük elemanın indeksini verir; eşitlikte ilki seçilir. Sonuç tam sayılı ve türev izlemeyen bir tensördür; eksen yalnızca adlı verilir.

```tr
yazdır(maks_indeks(tensör([[0.1, 2.5, 0.3], [1.7, 0.2, 0.9]]), eksen: -1).değer)
```

İlişkili: T611 `örnekle`

### Türev (T3xx)

#### T300 `geri_yayılım`

    geri_yayılım(Tensör) → Mantık

Tek elemanlı (skaler) bir kayıptan geri yayılım yaparak yaprak tensörlerin .gradyan değerlerini hesaplar. Gradyanlar birikir; adımlar arasında iyileştiricinin .sıfırla() metoduyla sıfırlanmalıdır.

```tr
değişken x = tensör(3.0, türev_izle: doğru); geri_yayılım(x * x); yazdır(x.gradyan.değer)
```

İlişkili: T301 `türevsiz`, Y403 `gradyan`

#### T301 `türevsiz`

    türevsiz(Tensör | Fonksiyon | YerleşikFonksiyon) → Herhangi

türevsiz { ... } bloğu türev izlemeden çalışır ve bloğun değerini döndürür; türevsiz(x) ise tensörü hesap çizgesinden ayırır. Türevsiz kip görev başınadır, başka eşzamanlı görevlere sızmaz.

```tr
değişken x = tensör([1.0], türev_izle: doğru); değişken y = türevsiz { x * 2 }; yazdır(y.değer)
```

İlişkili: T300 `geri_yayılım`

#### T310 `özel_türev`

    özel_türev(ileri: Fonksiyon | YerleşikFonksiyon, geri: Fonksiyon | YerleşikFonksiyon) → YerleşikFonksiyon

İleri ve geri hesabı kullanıcının yazdığı yeni bir türevlenebilir işlem kurar ve çağrılabilir bir fonksiyon döndürür. Yalnızca ileri: ve geri: adlı argümanlarını alır; ileri [çıktı, saklananlar], geri her konumlu girdi için bir gradyan (ya da yok) döndürür.

```tr
değişken kare = özel_türev(ileri: fonksiyon(x) { sonuç [x * x, [x]] }, geri: fonksiyon(s, g) { sonuç [g * 2 * s[0]] }); değişken x = tensör([3.0], türev_izle: doğru); geri_yayılım(toplam(kare(x))); yazdır(x.gradyan.değer)
```

İlişkili: T311 `türev_denetle`, T300 `geri_yayılım`

#### T311 `türev_denetle`

    türev_denetle(Fonksiyon | YerleşikFonksiyon, Liste, adım: Tam | Ondalık = 0.001, tolerans: Tam | Ondalık = 0.01) → Mantık

Bir fonksiyonun geri yayılımla bulunan gradyanını merkezi farkla bulunan sayısal gradyanla karşılaştırır; uyuşursa doğru döndürür, uyuşmazsa en büyük farkı gösteren bir hata verir. Yalnızca türev izleyen tensör girdileri, kopyaları üzerinde denetlenir.

```tr
yazdır(türev_denetle(fonksiyon(x) { sonuç x * x }, [tensör([0.5, -1.0], türev_izle: doğru)], adım: 0.001, tolerans: 0.01))
```

İlişkili: T310 `özel_türev`

### İyileştiriciler (T4xx)

#### T400 `AdamW`

    AdamW(Liste | Tensör, öğrenme_oranı: Tam | Ondalık = 0.001, ağırlık_çürümesi: Tam | Ondalık = 0.01, beta1: Tam | Ondalık = 0.9, beta2: Tam | Ondalık = 0.999, epsilon: Tam | Ondalık = 1e-08, ana_kopya: Mantık = doğru) → AdamW

Türev izleyen yaprak tensörler için ağırlık çürümesi ayrık uygulanan Adam iyileştiricisini kurar; .adım() parametreleri günceller, .sıfırla() gradyanları sıfırlar. Varsayılanlar: öğrenme_oranı 0.001, ağırlık_çürümesi 0.01, beta1 0.9, beta2 0.999, epsilon 1e-8.

```tr
değişken w = tensör([1.0], türev_izle: doğru); değişken opt = AdamW([w], öğrenme_oranı: 0.1); geri_yayılım(toplam(w * w)); opt.adım(); yazdır(w.değer)
```

İlişkili: T401 `SGD`, T402 `gradyan_kırp`, Y500 `adım`, Y501 `sıfırla`

#### T401 `SGD`

    SGD(Liste | Tensör, öğrenme_oranı: Tam | Ondalık = 0.01, momentum: Tam | Ondalık = 0, ağırlık_çürümesi: Tam | Ondalık = 0, ana_kopya: Mantık = doğru) → SGD

Türev izleyen yaprak tensörler için olasılıksal gradyan inişi iyileştiricisini kurar; .adım() ve .sıfırla() metotları vardır. Varsayılanlar: öğrenme_oranı 0.01, momentum 0, ağırlık_çürümesi 0.

```tr
değişken w = tensör([1.0], türev_izle: doğru); değişken opt = SGD([w], öğrenme_oranı: 0.1, momentum: 0.9); geri_yayılım(toplam(w * w)); opt.adım(); yazdır(w.değer)
```

İlişkili: T400 `AdamW`, Y500 `adım`, Y501 `sıfırla`

#### T402 `gradyan_kırp`

    gradyan_kırp(Liste | Tensör, en_fazla: Tam | Ondalık = 1) → Ondalık

Parametre gradyanlarının toplam L2 normu en_fazla değerini (varsayılan 1.0) aşıyorsa hepsini aynı oranda küçültür. Kırpma öncesi normu Ondalık olarak döndürür.

```tr
değişken w = tensör([3.0, 4.0], türev_izle: doğru); geri_yayılım(toplam(w * w)); yazdır(gradyan_kırp([w], en_fazla: 1.0))
```

İlişkili: T400 `AdamW`, T401 `SGD`

### Katmanlar, kayıplar, hazır model (T5xx)

#### T500 `gömme`

    gömme(Tensör, Tam | Liste | Tensör) → Tensör

[V, D] şeklindeki tablodan verilen indekslerin satırlarını seçer; sonuç şekli indeks şekli + [D]'dir. Tek tam sayı indeks [D] şeklinde tek satır verir; gradyan tabloya akar.

```tr
değişken tablo = tensör([[0.1, 0.2], [0.3, 0.4], [0.5, 0.6]]); yazdır(gömme(tablo, [2, 0]).değer)
```

İlişkili: T510 `çapraz_entropi`

#### T501 `katman_normu`

    katman_normu(Tensör | Liste, [Tensör | Liste | Yok], [Tensör | Liste | Yok], epsilon: Tam | Ondalık = 1e-05, ağırlık: Tensör | Liste | Yok = yok, sapma: Tensör | Liste | Yok = yok) → Tensör

Girdiyi son eksende ortalaması 0, varyansı 1 olacak biçimde normalleştirir; isteğe bağlı ağırlık ve sapma [D] şeklinde olmalıdır. Konumlu ya da adlı verilebilirler; epsilon varsayılanı 0.00001'dir.

```tr
yazdır(katman_normu(tensör([[1.0, 2.0, 3.0]]), ağırlık: birler([3]), sapma: sıfırlar([3])).değer)
```

İlişkili: T502 `rms_normu`

#### T502 `rms_normu`

    rms_normu(Tensör | Liste, [Tensör | Liste | Yok], epsilon: Tam | Ondalık = 1e-05, ağırlık: Tensör | Liste | Yok = yok) → Tensör

Girdiyi son eksende karelerinin ortalamasının köküne bölerek normalleştirir (ortalama çıkarılmaz). İsteğe bağlı ağırlık [D] şeklinde olmalıdır; epsilon varsayılanı 0.00001'dir.

```tr
yazdır(rms_normu(tensör([[3.0, 4.0]]), ağırlık: birler([2])).değer)
```

İlişkili: T501 `katman_normu`

#### T503 `dikkat`

    dikkat(Tensör, Tensör, Tensör, nedensel: Mantık = doğru, pencere: Tam = 0, maske: Tensör | Liste | Yok = yok) → Tensör

Birleşik ölçekli nokta çarpımı dikkatini (softmax(q @ kᵀ / √dh + maske) @ v) bellek dostu, tek türev düğümüyle hesaplar. Varsayılan olarak nedenseldir; k/v baş sayısı q'nunkinin böleni olabilir (gruplu sorgu dikkati).

```tr
değişken q = rastgele_normal([2, 4, 8], tohum: 1); yazdır(dikkat(q, q, q, nedensel: doğru).şekil)
```

İlişkili: T210 `softmax`

#### T510 `çapraz_entropi`

    çapraz_entropi(Tensör | Liste, Tam | Liste | Tensör) → Tensör

Son ekseni sınıf olan çıktıyla satır başına bir sınıf indeksi arasındaki ortalama çapraz entropi kaybını skaler tensör olarak verir. Negatif hedefler yok sayılır; hepsi yok sayılırsa hata verir.

```tr
yazdır(çapraz_entropi(tensör([[2.0, 0.5, 0.1], [0.2, 1.5, 0.3]]), [0, 1]).değer)
```

İlişkili: T211 `log_softmax`

#### T511 `kayıp_mse` — kaldırıldı

Kaldırıldı: 'kayıp_mse' TR'den kaldırıldı; yerine geçen bir kod yoktur.

#### T520 `sinir_ağı` — kaldırıldı

Kaldırıldı: 'sinir_ağı' TR'den kaldırıldı; yerine geçen bir kod yoktur.

#### T521 `model_yükle` — kaldırıldı

Kaldırıldı: 'model_yükle' TR'den kaldırıldı; yerine geçen bir kod yoktur.

#### T530 `parametreler`

    parametreler(Nesne | Liste | Sözlük | Tensör, adlarıyla: Mantık = yanlış) → Liste | Sözlük

Bir yapı nesnesini (iç içe yapılar, listeler ve sözlüklerle) gezip türev izleyen yaprak tensörleri sabit sırayla ve her birini bir kez toplar. adlarıyla: doğru ile yol adı → tensör sözlüğü döndürür; bu sözlük tensör_kaydet ile kaydedilebilir.

```tr
değişken model = {"w": tensör([1.0], türev_izle: doğru), "b": tensör([0.0], türev_izle: doğru)}; yazdır(parametreler(model, adlarıyla: doğru))
```

İlişkili: T400 `AdamW`, T700 `tensör_kaydet`

### Vektör yardımcıları ve dil modeli (T6xx)

#### T600 `normalize_et` — kaldırıldı

Kaldırıldı: 'normalize_et' TR'den kaldırıldı; yerine geçen bir kod yoktur.

#### T601 `nokta_çarpım` — kaldırıldı

Kaldırıldı: 'nokta_çarpım' TR'den kaldırıldı; yerine geçen bir kod yoktur.

#### T602 `kosinüs_benzerliği` — kaldırıldı

Kaldırıldı: 'kosinüs_benzerliği' TR'den kaldırıldı; yerine geçen bir kod yoktur.

#### T603 `öklid_uzaklığı` — kaldırıldı

Kaldırıldı: 'öklid_uzaklığı' TR'den kaldırıldı; yerine geçen bir kod yoktur.

#### T610 `yz_sor` — kaldırıldı

Kaldırıldı: 'yz_sor' TR'den kaldırıldı; yerine geçen bir kod yoktur.

#### T611 `örnekle`

    örnekle(Tensör, sıcaklık: Tam | Ondalık = 1, en_iyi_k: Tam = 0, tohum: Tam | Yok = yok) → Tensör

Son eksendeki softmax(logit / sıcaklık) dağılımından bir indeks örnekler; sonuç tam sayılı ve türev izlemeyen bir tensördür. sıcaklık 0 en olasıyı verir, en_iyi_k > 0 seçimi en olası k sınıfla sınırlar, aynı tohum aynı sonucu verir.

```tr
yazdır(örnekle(tensör([1.0, 3.0, 0.5]), sıcaklık: 0.8, en_iyi_k: 2, tohum: 7).değer)
```

İlişkili: T230 `maks_indeks`, T210 `softmax`

### Kaydetme, yükleme, veri türleri (T7xx)

#### T700 `tensör_kaydet`

    tensör_kaydet(Metin, Sözlük) → Yok

Anahtarları metin, değerleri tensör olan bir sözlüğü sürüm başlıklı ikili TR tensör dosyasına yazar. Yazma atomiktir; dosya varsa üzerine yazılır ve tensörler kendi türlerinde saklanır.

```tr
tensör_kaydet("agirliklar.trt", {"w": tensör([[1.0, 2.0]]), "b": tensör([0.5])}); yazdır(tensör_yükle("agirliklar.trt"))
```

İlişkili: T701 `tensör_yükle`, T530 `parametreler`

#### T701 `tensör_yükle`

    tensör_yükle(Metin, türev_izle: Mantık = yanlış) → Sözlük

tensör_kaydet ile yazılmış dosyayı aynı sırayla ad → tensör sözlüğü olarak okur; tensörler kaydedildikleri türdedir. Bozuk, eksik ya da başka türden dosya açık bir hatayla reddedilir.

```tr
tensör_kaydet("a.trt", {"w": tensör([1.0])}); yazdır(tensör_yükle("a.trt", türev_izle: doğru)["w"].gradyan)
```

İlişkili: T700 `tensör_kaydet`

#### T710 `karışık_hassasiyet`

    karışık_hassasiyet(Metin, Fonksiyon | YerleşikFonksiyon) → Herhangi

Bloğu verilen hesap türüyle ("bf16", "fp16" ya da kapatmak için "float32") çalıştırır ve bloğun son ifadesinin değerini döndürür. Blok içinde @ girdileri bu türe çevrilir ama sonuç float32'dir; ağırlıkların türü değişmez.

```tr
değişken y = karışık_hassasiyet("bf16") { tensör([[1.0, 2.0]]) @ tensör([[3.0], [4.0]]) }; yazdır(y.değer, y.tür)
```

İlişkili: T901 `tür`

### Adlı argümanlar (T9xx)

#### T900 `türev_izle`

Oluşturulan tensörün türevinin izlenip izlenmeyeceğini belirler; varsayılan yanlıştır. Parametreler ve gradyanı istenen girdiler türev_izle: doğru ile oluşturulmalıdır.

```tr
değişken x = tensör([2.0], türev_izle: doğru); geri_yayılım(toplam(x * x)); yazdır(x.gradyan.değer)
```

Kullananlar: T100 `tensör`, T101 `sıfırlar`, T102 `birler`, T103 `rastgele_normal`, T701 `tensör_yükle`

#### T901 `tür`

Oluşturulan tensörün veri türünü seçer: "float32" (varsayılan), "bf16" ya da "fp16". Başka bir ad en yakın geçerli adı öneren bir hatadır.

```tr
yazdır(sıfırlar([2, 2], tür: "fp16").tür)
```

Kullananlar: T100 `tensör`, T101 `sıfırlar`, T102 `birler`, T103 `rastgele_normal`

#### T902 `eksen`

İşlemin uygulanacağı ekseni seçer; negatif değer sondan sayar (-1 son eksen). İndirgemelerde verilmezse bütün elemanlar, softmax, birleştir ve maks_indeks'te son eksen kullanılır.

```tr
yazdır(toplam(tensör([[1, 2], [3, 4]]), eksen: 0).değer)
```

Kullananlar: G400 `toplam`, G401 `min`, G402 `maks`, G410 `ortalama`, T121 `birleştir`, T210 `softmax`, T211 `log_softmax`, T230 `maks_indeks`, Y103 `böl` · İlişkili: T906 `boyutu_koru`

#### T903 `tohum`

Rastgele sayı üretecinin başlangıç değeridir (tam sayı); aynı tohum her çalıştırmada aynı sonucu verir. Verilmezse paylaşılan bir üreteç kullanılır.

```tr
yazdır(rastgele_normal([3], tohum: 7).değer)
```

Kullananlar: T103 `rastgele_normal`, T611 `örnekle`, Y600 `örnekleyici`

#### T904 `en_fazla`

gradyan_kırp için izin verilen en büyük toplam gradyan normudur; pozitif bir sayı olmalıdır, varsayılan 1.0'dır.

```tr
değişken w = tensör([3.0, 4.0], türev_izle: doğru); geri_yayılım(toplam(w * w)); yazdır(gradyan_kırp([w], en_fazla: 0.5))
```

Kullananlar: T402 `gradyan_kırp`

#### T905 `epsilon`

Sıfıra bölmeyi önlemek için paydaya eklenen küçük pozitif sayıdır. Normlarda varsayılan 0.00001, AdamW'de 1e-8'dir.

```tr
yazdır(katman_normu(tensör([[1.0, 2.0, 3.0]]), epsilon: 0.001).değer)
```

Kullananlar: T400 `AdamW`, T501 `katman_normu`, T502 `rms_normu`

#### T906 `boyutu_koru`

doğru verilince indirgenen eksen 1 uzunlukta korunur, böylece sonuç girdiye yayınlanabilir; eksen verilmezse bütün eksenler 1 olur. Mantık olmalıdır ve yalnızca tensörlerde geçerlidir.

```tr
yazdır(toplam(tensör([[1, 2], [3, 4]]), eksen: -1, boyutu_koru: doğru).şekil)
```

Kullananlar: G400 `toplam`, G401 `min`, G402 `maks`, G410 `ortalama` · İlişkili: T902 `eksen`

#### T910 `öğrenme_oranı`

İyileştiricinin her adımda parametreleri ne kadar değiştireceğini belirleyen orandır. Sonlu ve negatif olmayan bir sayı olmalıdır; varsayılan AdamW'de 0.001, SGD'de 0.01'dir.

```tr
değişken w = tensör([1.0], türev_izle: doğru); değişken opt = SGD([w], öğrenme_oranı: 0.5); geri_yayılım(toplam(w * w)); opt.adım(); yazdır(w.değer)
```

Kullananlar: T400 `AdamW`, T401 `SGD` · İlişkili: T911 `ağırlık_çürümesi` · Eski adlar: `öğrenme_hızı`

#### T911 `ağırlık_çürümesi`

Her adımda parametreleri sıfıra doğru küçülten düzenlileştirme katsayısıdır. Varsayılan AdamW'de 0.01, SGD'de 0'dır.

```tr
değişken w = tensör([1.0], türev_izle: doğru); yazdır(AdamW([w], ağırlık_çürümesi: 0.1))
```

Kullananlar: T400 `AdamW`, T401 `SGD` · İlişkili: T910 `öğrenme_oranı`

#### T912 `beta1`

AdamW'nin gradyan ortalaması için kullandığı üstel azalma katsayısıdır. [0, 1) aralığında olmalıdır; varsayılan 0.9'dur.

```tr
değişken w = tensör([1.0], türev_izle: doğru); yazdır(AdamW([w], beta1: 0.8))
```

Kullananlar: T400 `AdamW` · İlişkili: T913 `beta2`

#### T913 `beta2`

AdamW'nin gradyan karelerinin ortalaması için kullandığı üstel azalma katsayısıdır. [0, 1) aralığında olmalıdır; varsayılan 0.999'dur.

```tr
değişken w = tensör([1.0], türev_izle: doğru); yazdır(AdamW([w], beta2: 0.99))
```

Kullananlar: T400 `AdamW` · İlişkili: T912 `beta1`

#### T914 `momentum`

SGD'nin önceki adımların yönünü sürdürme katsayısıdır; varsayılan 0'dır (momentum yok).

```tr
değişken w = tensör([1.0], türev_izle: doğru); yazdır(SGD([w], momentum: 0.9))
```

Kullananlar: T401 `SGD`

#### T915 `ana_kopya`

doğru iken (varsayılan) 16 bitlik parametrelerin float32 ana kopyası tutulur; güncelleme orada yapılıp parametreye yuvarlanır, böylece küçük adımlar kaybolmaz. float32 parametrelerde etkisizdir.

```tr
değişken w = tensör([1.0], türev_izle: doğru, tür: "bf16"); değişken opt = AdamW([w], ana_kopya: doğru); geri_yayılım(toplam(w * w)); opt.adım(); yazdır(w.değer)
```

Kullananlar: T400 `AdamW`, T401 `SGD`

#### T920 `ağırlık`

Normalleştirme sonucunu son eksende eleman bazlı ölçekleyen [D] şeklindeki tensördür; yok verilirse ölçekleme yapılmaz. Konumlu da verilebilir.

```tr
yazdır(rms_normu(tensör([[3.0, 4.0]]), ağırlık: tensör([2.0, 1.0])).değer)
```

Kullananlar: T501 `katman_normu`, T502 `rms_normu` · İlişkili: T921 `sapma`

#### T921 `sapma`

katman_normu sonucuna son eksende eklenen [D] şeklindeki kayma tensörüdür; yok verilirse eklenmez. Konumlu da verilebilir.

```tr
yazdır(katman_normu(tensör([[1.0, 3.0]]), sapma: tensör([0.5, 0.5])).değer)
```

Kullananlar: T501 `katman_normu` · İlişkili: T920 `ağırlık`

#### T930 `devir` — kaldırıldı

Kaldırıldı: 'devir' TR'den kaldırıldı; yerine geçen bir kod yoktur.

#### T932 `aktivasyon` — kaldırıldı

Kaldırıldı: 'aktivasyon' TR'den kaldırıldı; yerine geçen bir kod yoktur.

#### T940 `model` — kaldırıldı

Kaldırıldı: 'model' TR'den kaldırıldı; yerine geçen bir kod yoktur.

#### T941 `adres` — kaldırıldı

Kaldırıldı: 'adres' TR'den kaldırıldı; yerine geçen bir kod yoktur.

#### T950 `nedensel`

doğru iken (varsayılan) her sorgu yalnızca kendisine kadarki anahtarlara bakar; Tq < Tk iken hizalama sondandır. yanlış ile dikkat iki yönlüdür.

```tr
değişken q = rastgele_normal([4, 8], tohum: 1); yazdır(dikkat(q, q, q, nedensel: yanlış).şekil)
```

Kullananlar: T503 `dikkat` · İlişkili: T951 `pencere`, T952 `maske`

#### T951 `pencere`

Her sorguyu son w konuma (kendisi dahil) sınırlayan yerel dikkat penceresidir; 0 (varsayılan) pencere yok demektir. nedensel: yanlış ile pencere iki yönlüdür.

```tr
değişken q = rastgele_normal([6, 4], tohum: 2); yazdır(dikkat(q, q, q, pencere: 2).şekil)
```

Kullananlar: T503 `dikkat` · İlişkili: T950 `nedensel`

#### T952 `maske`

[..., H, Tq, Tk] şekline yayınlanabilen 0/1 tensörüdür; sıfırdan farklı yerlere bakılır. Türev izlemez; varsayılan yok'tur.

```tr
değişken q = rastgele_normal([3, 4], tohum: 1); yazdır(dikkat(q, q, q, nedensel: yanlış, maske: tensör([[1, 1, 0], [1, 1, 0], [1, 1, 1]])).şekil)
```

Kullananlar: T503 `dikkat` · İlişkili: T950 `nedensel`

#### T960 `sıcaklık`

örnekle'de logitlerin bölündüğü sıcaklıktır: küçük değer en olasıya yaklaştırır, 0 her zaman en olasıyı seçer. Negatif olmamalıdır; varsayılan 1.0'dır.

```tr
yazdır(örnekle(tensör([1.0, 3.0, 0.5]), sıcaklık: 0).değer)
```

Kullananlar: T611 `örnekle` · İlişkili: T961 `en_iyi_k`

#### T961 `en_iyi_k`

örnekle'de seçimi en olası k sınıfla sınırlar; 0 (varsayılan) bütün sınıflar demektir. Negatif olmayan bir tam sayı olmalıdır.

```tr
yazdır(örnekle(tensör([1.0, 3.0, 0.5]), en_iyi_k: 1).değer)
```

Kullananlar: T611 `örnekle` · İlişkili: T960 `sıcaklık`

#### T970 `adlarıyla`

doğru verilince parametreler liste yerine "bloklar.0.Wq" gibi yol adı → tensör sözlüğü döndürür. Mantık olmalıdır; varsayılan yanlış'tır.

```tr
yazdır(parametreler([tensör([1.0], türev_izle: doğru)], adlarıyla: doğru))
```

Kullananlar: T530 `parametreler`

#### T980 `ileri`

özel_türev'in ileri hesabıdır: türevsiz çalışır ve [çıktı, saklananlar] döndürmelidir. Çıktı bir tensör ya da tensör listesidir.

```tr
değişken f = özel_türev(ileri: fonksiyon(x) { sonuç [x * 3, []] }, geri: fonksiyon(s, g) { sonuç [g * 3] }); yazdır(f(tensör([2.0])).değer)
```

Kullananlar: T310 `özel_türev` · İlişkili: T981 `geri`

#### T981 `geri`

özel_türev'in geri hesabıdır: fonksiyon(saklananlar, g) biçiminde iki parametre alır ve her konumlu girdi için girdiyle aynı şekilde bir gradyan ya da yok döndürür.

```tr
değişken f = özel_türev(ileri: fonksiyon(x) { sonuç [x * 3, []] }, geri: fonksiyon(s, g) { sonuç [g * 3] }); değişken x = tensör([2.0], türev_izle: doğru); geri_yayılım(toplam(f(x))); yazdır(x.gradyan.değer)
```

Kullananlar: T310 `özel_türev` · İlişkili: T980 `ileri`

#### T982 `adım`

türev_denetle'nin merkezi farkta kullandığı adım büyüklüğüdür; pozitif olmalıdır, varsayılan 0.001'dir.

```tr
yazdır(türev_denetle(fonksiyon(x) { sonuç x * x }, [tensör([1.0, 2.0], türev_izle: doğru)], adım: 0.01))
```

Kullananlar: T311 `türev_denetle` · İlişkili: T983 `tolerans`

#### T983 `tolerans`

türev_denetle'de izin verilen göreli farktır: |analitik - sayısal| ≤ tolerans · max(1, |analitik|, |sayısal|). Pozitif olmalıdır; varsayılan 0.01'dir.

```tr
yazdır(türev_denetle(fonksiyon(x) { sonuç sigmoid(x) }, [tensör([0.3], türev_izle: doğru)], tolerans: 0.001))
```

Kullananlar: T311 `türev_denetle` · İlişkili: T982 `adım`

## V — Veri ve parçalayıcı

### Veri kümesi (V1xx)

#### V100 `veri_kümesi`

    veri_kümesi(Metin, önbellek: Metin, kota_gb: Tam | Ondalık) → VeriKümesi

Bir klasördeki .npy veri dosyalarını (ad sırasıyla) ya da tek bir .npy veri dosyasını VeriKümesi olarak açar; okuyucular .örnekleyici ve .sıralı ile alınır. önbellek: verilirse kota_gb: de verilmelidir.

```tr
değişken küme = veri_kümesi("testler/veri/sayilar"); yazdır(küme)
```

İlişkili: Y600 `örnekleyici`, Y601 `sıralı`

### Parçalayıcı (V2xx)

#### V200 `parçalayıcı_yükle`

    parçalayıcı_yükle(Metin) → Parçalayıcı

tokenizer.json biçimindeki bir parçalayıcı dosyasını yükler ve Parçalayıcı nesnesi döndürür. Adlı argüman almaz; klasör yolu verilirse hata verir.

```tr
değişken p = parçalayıcı_yükle("testler/veri/parcalayici.json"); yazdır(p)
```

İlişkili: Y610 `kodla`, Y611 `çöz`, Y612 `dağarcık_boyutu`, Y613 `parça_kimliği`

### Adlı argümanlar (V9xx)

#### V900 `önbellek`

veri_kümesi'nin veri dosyalarını kullanmadan önce kopyalayacağı yerel önbellek klasörüdür; kota dolunca en uzun süredir kullanılmayan kopyalar silinir. Tek başına verilemez: kota_gb: zorunludur.

```tr
değişken küme = veri_kümesi("testler/veri/sayilar", önbellek: "onbellek/veri", kota_gb: 1)
```

Kullananlar: V100 `veri_kümesi` · İlişkili: V901 `kota_gb`

#### V901 `kota_gb`

Önbellek klasörünün kullanabileceği en çok disk alanıdır (GB). Pozitif bir sayı olmalıdır ve yalnızca önbellek: ile birlikte geçerlidir.

```tr
değişken küme = veri_kümesi("testler/veri/sayilar", önbellek: "onbellek/veri", kota_gb: 0.5)
```

Kullananlar: V100 `veri_kümesi` · İlişkili: V900 `önbellek`

#### V902 `toplu`

Okuyucunun her .sonraki() çağrısında verdiği satır (toplu iş) sayısıdır. Pozitif bir tam sayı olmalıdır ve verilmesi zorunludur.

```tr
değişken ö = veri_kümesi("testler/veri/sayilar").örnekleyici(toplu: 8, bağlam: 16)
```

Kullananlar: Y600 `örnekleyici`, Y601 `sıralı` · İlişkili: V903 `bağlam`

#### V903 `bağlam`

Okuyucunun her satırdaki pencere uzunluğudur; girdi ve hedef [toplu, bağlam] şeklinde olur. Zorunludur ve en uzun veri dosyasından kısa olmalıdır.

```tr
değişken s = veri_kümesi("testler/veri/sayilar").sıralı(toplu: 4, bağlam: 16)
```

Kullananlar: Y600 `örnekleyici`, Y601 `sıralı` · İlişkili: V902 `toplu`

#### V910 `özel_parçalar`

Özel parçaların kodlamaya eklenip eklenmeyeceğini (kodla, varsayılan doğru) ya da çözülen metne yazılıp yazılmayacağını (çöz, varsayılan yanlış) belirler. Yalnızca doğru ya da yanlış olabilir.

```tr
yazdır(parçalayıcı_yükle("testler/veri/parcalayici.json").kodla("merhaba dünya", özel_parçalar: yanlış))
```

Kullananlar: Y610 `kodla`, Y611 `çöz` · Eski adlar: `özel_tokenlar`

#### V911 `ikinci`

kodla'ya ikinci bir metin vererek çift kodlama yapar. Liste kodlanırken aynı uzunlukta bir metin listesi olmalıdır.

```tr
yazdır(parçalayıcı_yükle("testler/veri/parcalayici.json").kodla("soru", ikinci: "cevap"))
```

Kullananlar: Y610 `kodla`

## W — Web, veritabanı, arayüz (kaldırıldı)

### Web istemcisi ve sunucusu (W1xx)

#### W100 `web_al` — kaldırıldı

Kaldırıldı: 'web_al' TR'den kaldırıldı; yerine geçen bir kod yoktur.

#### W101 `web_gönder` — kaldırıldı

Kaldırıldı: 'web_gönder' TR'den kaldırıldı; yerine geçen bir kod yoktur.

#### W110 `web_sunucu` — kaldırıldı

Kaldırıldı: 'web_sunucu' TR'den kaldırıldı; yerine geçen bir kod yoktur.

### Veritabanı (W2xx)

#### W200 `veritabanı_aç` — kaldırıldı

Kaldırıldı: 'veritabanı_aç' TR'den kaldırıldı; yerine geçen bir kod yoktur.

#### W210 `tablo_sorgula` — kaldırıldı

Kaldırıldı: 'tablo_sorgula' TR'den kaldırıldı; yerine geçen bir kod yoktur.

### Görsel arayüz (W3xx)

#### W300 `arayüz_oluştur` — kaldırıldı

Kaldırıldı: 'arayüz_oluştur' TR'den kaldırıldı; yerine geçen bir kod yoktur.

### Adlı argümanlar (W9xx)

#### W900 `port` — kaldırıldı

Kaldırıldı: 'port' TR'den kaldırıldı; yerine geçen bir kod yoktur.

#### W901 `arka_plan` — kaldırıldı

Kaldırıldı: 'arka_plan' TR'den kaldırıldı; yerine geçen bir kod yoktur.

#### W910 `genişlik` — kaldırıldı

Kaldırıldı: 'genişlik' TR'den kaldırıldı; yerine geçen bir kod yoktur.

#### W911 `yükseklik` — kaldırıldı

Kaldırıldı: 'yükseklik' TR'den kaldırıldı; yerine geçen bir kod yoktur.

#### W912 `pencere_aç` — kaldırıldı

Kaldırıldı: 'pencere_aç' TR'den kaldırıldı; yerine geçen bir kod yoktur.

#### W920 `alt_bilgi` — kaldırıldı

Kaldırıldı: 'alt_bilgi' TR'den kaldırıldı; yerine geçen bir kod yoktur.

#### W921 `renk` — kaldırıldı

Kaldırıldı: 'renk' TR'den kaldırıldı; yerine geçen bir kod yoktur.

#### W922 `tip` — kaldırıldı

Kaldırıldı: 'tip' TR'den kaldırıldı; yerine geçen bir kod yoktur.

#### W930 `koşul` — kaldırıldı

Kaldırıldı: 'koşul' TR'den kaldırıldı; yerine geçen bir kod yoktur.

#### W931 `sırala` — kaldırıldı

Kaldırıldı: 'sırala' TR'den kaldırıldı; yerine geçen bir kod yoktur.

#### W932 `limit` — kaldırıldı

Kaldırıldı: 'limit' TR'den kaldırıldı; yerine geçen bir kod yoktur.

#### W933 `alanlar` — kaldırıldı

Kaldırıldı: 'alanlar' TR'den kaldırıldı; yerine geçen bir kod yoktur.

#### W934 `parametreler` — kaldırıldı

Kaldırıldı: 'parametreler' TR'den kaldırıldı; yerine geçen bir kod yoktur.

## Y — Metot ve özellikler (x.ad)

### Birden çok tipte ortak (Y0xx)

#### Y001 `uzunluk`

    Metin.uzunluk → Tam
    Liste.uzunluk → Tam
    Sözlük.uzunluk → Tam

Metnin karakter sayısını, listenin ya da sözlüğün eleman sayısını verir. Parantezsiz yazılan bir özelliktir.

```tr
yazdır("çay".uzunluk, [1, 2, 3].uzunluk, {"a": 1}.uzunluk)
```

Eski adlar: `boy` · Önceki biçim: `uzunluk(...)` (G200, kaldırıldı)

#### Y002 `içerir`

    Metin.içerir(Metin) → Mantık
    Liste.içerir(Herhangi) → Mantık
    Sözlük.içerir(Herhangi) → Mantık

Metinde alt metni, listede değeri (== ile), sözlükte anahtarı arar ve doğru ya da yanlış verir. Metinde argüman Metin olmalıdır.

```tr
yazdır("merhaba".içerir("er"), [1, 2].içerir(2), {"a": 1}.içerir("a"))
```

İlişkili: Y105 `başlar`, Y106 `biter`

#### Y003 `ters_çevir`

    Metin.ters_çevir() → Metin
    Liste.ters_çevir() → Liste

Metnin karakterlerini ya da listenin elemanlarını ters sırayla verir. Asıl değeri değiştirmez, yeni metin ya da liste döner.

```tr
yazdır("kitap".ters_çevir(), [1, 2, 3].ters_çevir())
```

İlişkili: Y202 `sırala`

#### Y004 `ekle`

    Liste.ekle(Herhangi...) → Liste

Listenin sonuna verilen elemanları ekler; listeyi yerinde değiştirir ve listenin kendisini verir.

```tr
değişken l = [1]; l.ekle(2, 3); yazdır(l)
```

İlişkili: Y210 `ilk`, Y211 `son` · Eski adlar: `kayıt_ekle` · Önceki biçim: `ekle(...)` (G304, kaldırıldı)

#### Y005 `kaydet` — kaldırıldı

Kaldırıldı: 'kaydet' TR'den kaldırıldı; yerine geçen bir kod yoktur.

#### Y006 `başlat` — kaldırıldı

Kaldırıldı: 'başlat' TR'den kaldırıldı; yerine geçen bir kod yoktur.

#### Y007 `arka_planda_başlat` — kaldırıldı

Kaldırıldı: 'arka_planda_başlat' TR'den kaldırıldı; yerine geçen bir kod yoktur.

#### Y008 `durdur` — kaldırıldı

Kaldırıldı: 'durdur' TR'den kaldırıldı; yerine geçen bir kod yoktur.

### Metin (Y1xx)

#### Y100 `büyük_harf`

    Metin.büyük_harf() → Metin

Metni Türkçe kurallarla büyük harfe çevirir (i → İ, ı → I). Yeni bir metin verir.

```tr
yazdır("istanbul".büyük_harf())
```

İlişkili: Y101 `küçük_harf`

#### Y101 `küçük_harf`

    Metin.küçük_harf() → Metin

Metni Türkçe kurallarla küçük harfe çevirir (İ → i, I → ı). Yeni bir metin verir.

```tr
yazdır("IŞIK".küçük_harf())
```

İlişkili: Y100 `büyük_harf`

#### Y102 `kırp`

    Metin.kırp() → Metin

Metnin başındaki ve sonundaki boşlukları (satır sonları dahil) atar. Yeni bir metin verir.

```tr
yazdır("  merhaba  ".kırp())
```

#### Y103 `böl`

    Metin.böl([Metin]) → Liste
    Tensör.böl(Tam, [Tam], eksen: Tam = -1) → Liste

Metni ayırıcıdan böler ve parçaların listesini verir (ayırıcı verilmezse tek boşluk). Tensörü bir eksende (varsayılan son eksen) eşit sayıda parçaya böler; eksen uzunluğu parça sayısına tam bölünmelidir.

```tr
yazdır("a,b,c".böl(","), tensör([1, 2, 3, 4]).böl(2)[0].değer)
```

İlişkili: Y203 `birleştir`, T121 `birleştir`

#### Y104 `değiştir`

    Metin.değiştir(Metin, Metin) → Metin

Metindeki bütün eski alt metinleri yenisiyle değiştirir. İki argüman da Metin olmalıdır; yeni bir metin döner.

```tr
yazdır("elma armut elma".değiştir("elma", "erik"))
```

#### Y105 `başlar`

    Metin.başlar(Metin) → Mantık

Metin verilen önekle başlıyorsa doğru verir. Argüman Metin olmalıdır.

```tr
yazdır("merhaba".başlar("mer"))
```

İlişkili: Y106 `biter`, Y002 `içerir`

#### Y106 `biter`

    Metin.biter(Metin) → Mantık

Metin verilen sonekle bitiyorsa doğru verir. Argüman Metin olmalıdır.

```tr
yazdır("rapor.tr".biter(".tr"))
```

İlişkili: Y105 `başlar`, Y002 `içerir`

### Liste (Y2xx)

#### Y200 `filtrele`

    Liste.filtrele(Fonksiyon) → Liste

Fonksiyonun doğru döndüğü elemanlardan oluşan yeni bir liste verir. Asıl liste değişmez; fonksiyon sondaki blokla da yazılır.

```tr
yazdır([1, 2, 3, 4].filtrele { o > 2 })
```

İlişkili: Y201 `haritala`, Y207 `indirge` · Önceki biçim: `filtrele(...)` (G300, kaldırıldı)

#### Y201 `haritala`

    Liste.haritala(Fonksiyon) → Liste

Her elemana fonksiyonu uygular ve sonuçların yeni listesini verir. Asıl liste değişmez; fonksiyon sondaki blokla da yazılır.

```tr
yazdır([1, 2, 3].haritala { o * 10 })
```

İlişkili: Y200 `filtrele`, Y207 `indirge` · Eski adlar: `dönüştür` · Önceki biçim: `haritala(...)` (G301, kaldırıldı)

#### Y202 `sırala`

    Liste.sırala() → Liste

Listenin küçükten büyüğe sıralanmış yeni bir kopyasını verir. Liste yalnızca sayılardan ya da yalnızca metinlerden oluşmalıdır.

```tr
yazdır([3, 1, 2].sırala())
```

İlişkili: Y003 `ters_çevir` · Önceki biçim: `sırala(...)` (G303, kaldırıldı)

#### Y203 `birleştir`

    Liste.birleştir([Metin]) → Metin

Liste elemanlarını metne çevirip aralarına ayırıcıyı koyarak birleştirir (ayırıcı verilmezse boş metin). Sonuç bir metindir.

```tr
yazdır(["a", "b", "c"].birleştir(", "))
```

İlişkili: Y103 `böl`

#### Y204 `benzersiz`

    Liste.benzersiz() → Liste

Tekrar eden elemanları atar, her değerin ilk geçtiği sırayı korur. Yeni bir liste verir.

```tr
yazdır([1, 2, 2, 3, 1].benzersiz())
```

#### Y205 `toplam` — kaldırıldı

Taşındı: G400 `toplam` → `toplam(...)`. Kaldırıldı: toplam matematik işi olduğundan fonksiyon biçimiyle yazılır: toplam(liste). liste.toplam() hata verip yeni biçimi önerir.

#### Y206 `ortalama` — kaldırıldı

Taşındı: G410 `ortalama` → `ortalama(...)`. Kaldırıldı: ortalama matematik işi olduğundan fonksiyon biçimiyle yazılır: ortalama(liste). liste.ortalama() hata verip yeni biçimi önerir.

#### Y207 `indirge`

    Liste.indirge([Herhangi], Fonksiyon) → Herhangi

Listeyi soldan sağa (birikim, eleman) alan bir fonksiyonla tek değere indirger; başlangıç verilmezse ilk eleman başlangıçtır, boş listede başlangıç (yoksa yok) döner. Fonksiyon son argümandır; sondaki blokla yazılınca o, [birikim, eleman] listesidir.

```tr
yazdır([1, 2, 3, 4].indirge(0, (a, x) => a + x), [1, 2, 3, 4].indirge(1) { o[0] * o[1] })
```

İlişkili: Y200 `filtrele`, Y201 `haritala` · Önceki biçim: `indirge(...)` (G302, kaldırıldı)

#### Y210 `ilk`

    Liste.ilk → Herhangi

Listenin ilk elemanını verir; liste boşsa yok verir. Parantezsiz yazılan bir özelliktir.

```tr
yazdır([5, 6, 7].ilk)
```

İlişkili: Y211 `son`

#### Y211 `son`

    Liste.son → Herhangi

Listenin son elemanını verir; liste boşsa yok verir. Parantezsiz yazılan bir özelliktir.

```tr
yazdır([5, 6, 7].son)
```

İlişkili: Y210 `ilk`

### Sözlük (Y3xx)

#### Y300 `anahtarlar`

    Sözlük.anahtarlar() → Liste

Sözlüğün anahtarlarını ekleniş sırasıyla bir liste olarak verir.

```tr
yazdır({"ad": "Ali", "puan": 30}.anahtarlar())
```

İlişkili: Y301 `değerler`

#### Y301 `değerler`

    Sözlük.değerler() → Liste

Sözlüğün değerlerini anahtarların ekleniş sırasıyla bir liste olarak verir.

```tr
yazdır({"ad": "Ali", "puan": 30}.değerler())
```

İlişkili: Y300 `anahtarlar`

### Tensör (Y4xx)

#### Y400 `şekil`

    Tensör.şekil → Liste

Tensörün her eksendeki uzunluğunu Tam listesi olarak verir; 0 boyutlu tensörde boş listedir.

```tr
yazdır(tensör([[1, 2], [3, 4]]).şekil)
```

İlişkili: Y410 `şekillendir` · Eski adlar: `boyut`, `satır`, `sütun`

#### Y401 `cihaz`

    Tensör.cihaz → Metin

Tensörün bulunduğu cihazın adını verir (ör. "cpu").

```tr
yazdır(tensör([1, 2]).cihaz)
```

İlişkili: Y414 `cihaza`

#### Y402 `tür`

    Tensör.tür → Metin

Tensörün sayı türünün adını verir: "float32", "bf16" ya da "fp16".

```tr
yazdır(tensör([1.0, 2.0]).tür)
```

İlişkili: Y415 `türe`, T901 `tür`

#### Y403 `gradyan`

    Tensör.gradyan → Tensör | Yok

geri_yayılım sonrası biriken gradyanı tensör olarak verir; henüz yoksa yok döner. Yalnızca yaprak tensörlerde okunabilir, bir işlemin sonucunda hata verir.

```tr
değişken x = tensör([1.0, 2.0], türev_izle: doğru); geri_yayılım(toplam(x * 3)); yazdır(x.gradyan.değer)
```

İlişkili: T300 `geri_yayılım`, T900 `türev_izle` · Eski adlar: `türev`

#### Y404 `değer`

    Tensör.değer → Liste | Ondalık | Tam

Tensörün değerlerini iç içe liste olarak verir; 0 boyutlu tensörde tek bir sayı döner. Sonuç kopyadır, değiştirilmesi tensörü etkilemez.

```tr
yazdır(tensör([[1, 2], [3, 4]]).değer)
```

Eski adlar: `liste`

#### Y410 `şekillendir`

    Tensör.şekillendir(Liste | Tam, [Tam...]) → Tensör

Tensörü aynı eleman sayısıyla yeni bir şekle sokar; bir eksen -1 verilirse uzunluğu hesaplanır. Sonuç veriyi paylaşan bir görünümdür.

```tr
yazdır(tensör([1, 2, 3, 4, 5, 6]).şekillendir([2, 3]).şekil)
```

İlişkili: Y400 `şekil`

#### Y411 `devrik`

    Tensör.devrik() → Tensör

Tensörün son iki eksenini yer değiştirir. Argüman almaz ve en az 2 boyutlu tensör ister.

```tr
yazdır(tensör([[1, 2], [3, 4]]).devrik().değer)
```

İlişkili: Y412 `eksenleri_sırala` · Eski adlar: `T` · Önceki biçim: `devrik(...)` (T120, kaldırıldı)

#### Y412 `eksenleri_sırala`

    Tensör.eksenleri_sırala(Liste | Tam, [Tam...]) → Tensör

Tensörün eksenlerini verilen sıraya dizer. Her eksen tam olarak bir kez yer almalıdır.

```tr
yazdır(sıfırlar([2, 3, 4]).eksenleri_sırala([2, 0, 1]).şekil)
```

İlişkili: Y411 `devrik`

#### Y413 `dilim`

    Tensör.dilim(Tam, Tam, Tam) → Tensör

Verilen eksende [baş, son) aralığını keser; negatif konumlar sondan sayılır. Sonuç veriyi paylaşan bir görünümdür.

```tr
yazdır(tensör([[1, 2, 3], [4, 5, 6]]).dilim(1, 0, 2).değer)
```

#### Y414 `cihaza`

    Tensör.cihaza(Metin) → Tensör

Tensörü verilen cihaza ("cpu", "gpu", "gpu:1") taşınmış olarak verir. Bu derlemede yalnızca "cpu" kullanılabilir.

```tr
yazdır(tensör([1, 2]).cihaza("cpu").cihaz)
```

İlişkili: Y401 `cihaz`

#### Y415 `türe`

    Tensör.türe(Metin) → Tensör

Tensörün "float32", "bf16" ya da "fp16" türüne çevrilmiş kopyasını verir; gradyan kaynak tensöre akar. Tür zaten aynıysa tensörün kendisi döner.

```tr
yazdır(tensör([1.0, 2.0]).türe("bf16").tür)
```

İlişkili: Y402 `tür`, T710 `karışık_hassasiyet`

### İyileştirici (Y5xx)

#### Y500 `adım`

    AdamW.adım() → Yok
    SGD.adım() → Yok

İyileştiricinin (AdamW, SGD) bir güncelleme adımını yapar: parametreleri gradyanlarına göre yerinde günceller. Argüman almaz, yok döner.

```tr
değişken w = tensör([1.0], türev_izle: doğru); değişken op = SGD([w], öğrenme_oranı: 0.1); geri_yayılım(toplam(w * 2)); op.adım(); yazdır(w.değer)
```

İlişkili: Y501 `sıfırla`, T300 `geri_yayılım`

#### Y501 `sıfırla`

    AdamW.sıfırla() → Yok
    SGD.sıfırla() → Yok

İyileştiricinin parametrelerindeki biriken gradyanları sıfırlar. Bir sonraki geri_yayılım'dan önce çağrılır; argüman almaz.

```tr
değişken w = tensör([1.0], türev_izle: doğru); geri_yayılım(toplam(w * 2)); AdamW([w]).sıfırla(); yazdır(w.gradyan.değer)
```

İlişkili: Y500 `adım`

#### Y510 `eğit` — kaldırıldı

Kaldırıldı: 'eğit' TR'den kaldırıldı; yerine geçen bir kod yoktur.

#### Y511 `tahmin_et` — kaldırıldı

Kaldırıldı: 'tahmin_et' TR'den kaldırıldı; yerine geçen bir kod yoktur.

#### Y512 `ağırlıklar` — kaldırıldı

Kaldırıldı: 'ağırlıklar' TR'den kaldırıldı; yerine geçen bir kod yoktur.

### Veri kümesi, okuyucu, parçalayıcı (Y6xx)

#### Y600 `örnekleyici`

    VeriKümesi.örnekleyici([Tam], [Tam], [Tam], toplu: Tam, bağlam: Tam, tohum: Tam = 0) → Örnekleyici

Veri kümesinden rastgele pencereler alan bir örnekleyici kurar; toplu ve bağlam zorunludur, tohum isteğe bağlıdır. Örnekleyici sonsuzdur, dönemler bitince baştan sürer.

```tr
yazdır(veri_kümesi("testler/veri/sayilar").örnekleyici(toplu: 2, bağlam: 4, tohum: 1))
```

İlişkili: Y601 `sıralı`, Y602 `sonraki`

#### Y601 `sıralı`

    VeriKümesi.sıralı([Tam], [Tam], toplu: Tam, bağlam: Tam) → SıralıOkuyucu

Veri kümesini baştan sona sırayla okuyan bir okuyucu kurar; toplu ve bağlam zorunludur. Veri bitince sonraki() yok verir.

```tr
yazdır(veri_kümesi("testler/veri/sayilar").sıralı(toplu: 2, bağlam: 4).sonraki()[0].şekil)
```

İlişkili: Y600 `örnekleyici`, Y602 `sonraki`

#### Y602 `sonraki`

    Örnekleyici.sonraki() → Liste
    SıralıOkuyucu.sonraki() → Liste | Yok

Bir sonraki toplu işi [girdi, hedef] tensörleri olarak verir; ikisinin şekli [toplu, bağlam] olur ve hedef bir parça kaydırılmıştır. Sıralı okuyucu bitince yok verir.

```tr
değişken girdi, hedef = veri_kümesi("testler/veri/sayilar").sıralı(toplu: 2, bağlam: 4).sonraki(); yazdır(girdi.değer, hedef.değer)
```

İlişkili: Y600 `örnekleyici`, Y601 `sıralı`

#### Y610 `kodla`

    Parçalayıcı.kodla(Metin | Liste, özel_parçalar: Mantık = doğru, ikinci: Metin | Liste) → Liste

Metni parça kimliklerinin listesine çevirir; metin listesi verilirse her metni ayrı kodlar. özel_parçalar: yanlış verilmezse parçalayıcının özel parçaları eklenir; ikinci: ile metin çifti kodlanır.

```tr
yazdır(parçalayıcı_yükle("testler/veri/parcalayici.json").kodla("Merhaba dünya!"))
```

İlişkili: Y611 `çöz`

#### Y611 `çöz`

    Parçalayıcı.çöz(Liste | Tensör, özel_parçalar: Mantık = yanlış) → Metin | Liste

Parça kimliklerini (liste, listelerin listesi ya da tensör) metne geri çevirir. Varsayılan olarak özel parçaları atlar; özel_parçalar: doğru onları da yazar.

```tr
değişken p = parçalayıcı_yükle("testler/veri/parcalayici.json"); yazdır(p.çöz(p.kodla("Merhaba dünya!")))
```

İlişkili: Y610 `kodla`

#### Y612 `dağarcık_boyutu`

    Parçalayıcı.dağarcık_boyutu → Tam

Parçalayıcının dağarcığındaki parça sayısını (eklenmiş özel parçalar dahil) Tam olarak verir. Parantezsiz yazılan bir özelliktir.

```tr
yazdır(parçalayıcı_yükle("testler/veri/parcalayici.json").dağarcık_boyutu)
```

İlişkili: Y613 `parça_kimliği` · Eski adlar: `sözlük_boyutu`

#### Y613 `parça_kimliği`

    Parçalayıcı.parça_kimliği(Metin) → Tam | Yok

Verilen parçanın dağarcıktaki kimliğini verir; parça dağarcıkta yoksa yok döner. Argüman Metin olmalıdır.

```tr
yazdır(parçalayıcı_yükle("testler/veri/parcalayici.json").parça_kimliği("<|endoftext|>"))
```

İlişkili: Y612 `dağarcık_boyutu` · Eski adlar: `token_kimliği`

### Veritabanı (Y7xx)

#### Y700 `tablolar` — kaldırıldı

Kaldırıldı: 'tablolar' TR'den kaldırıldı; yerine geçen bir kod yoktur.

#### Y701 `tablo_oluştur` — kaldırıldı

Kaldırıldı: 'tablo_oluştur' TR'den kaldırıldı; yerine geçen bir kod yoktur.

#### Y702 `sorgula` — kaldırıldı

Kaldırıldı: 'sorgula' TR'den kaldırıldı; yerine geçen bir kod yoktur.

#### Y703 `güncelle` — kaldırıldı

Kaldırıldı: 'güncelle' TR'den kaldırıldı; yerine geçen bir kod yoktur.

#### Y704 `sil` — kaldırıldı

Kaldırıldı: 'sil' TR'den kaldırıldı; yerine geçen bir kod yoktur.

#### Y705 `hepsini_al` — kaldırıldı

Kaldırıldı: 'hepsini_al' TR'den kaldırıldı; yerine geçen bir kod yoktur.

#### Y706 `say` — kaldırıldı

Kaldırıldı: 'say' TR'den kaldırıldı; yerine geçen bir kod yoktur.

### Web sunucusu (Y8xx)

#### Y800 `adres` — kaldırıldı

Kaldırıldı: 'adres' TR'den kaldırıldı; yerine geçen bir kod yoktur.

#### Y801 `port` — kaldırıldı

Kaldırıldı: 'port' TR'den kaldırıldı; yerine geçen bir kod yoktur.

#### Y802 `al` — kaldırıldı

Kaldırıldı: 'al' TR'den kaldırıldı; yerine geçen bir kod yoktur.

#### Y803 `gönder` — kaldırıldı

Kaldırıldı: 'gönder' TR'den kaldırıldı; yerine geçen bir kod yoktur.

#### Y804 `rota_ekle` — kaldırıldı

Kaldırıldı: 'rota_ekle' TR'den kaldırıldı; yerine geçen bir kod yoktur.

#### Y805 `statik_klasör` — kaldırıldı

Kaldırıldı: 'statik_klasör' TR'den kaldırıldı; yerine geçen bir kod yoktur.

### Görsel arayüz (Y9xx)

#### Y900 `kart_ekle` — kaldırıldı

Kaldırıldı: 'kart_ekle' TR'den kaldırıldı; yerine geçen bir kod yoktur.

#### Y901 `başlık_ekle` — kaldırıldı

Kaldırıldı: 'başlık_ekle' TR'den kaldırıldı; yerine geçen bir kod yoktur.

#### Y902 `metin_ekle` — kaldırıldı

Kaldırıldı: 'metin_ekle' TR'den kaldırıldı; yerine geçen bir kod yoktur.

#### Y903 `tablo_ekle` — kaldırıldı

Kaldırıldı: 'tablo_ekle' TR'den kaldırıldı; yerine geçen bir kod yoktur.

#### Y904 `grafik_ekle` — kaldırıldı

Kaldırıldı: 'grafik_ekle' TR'den kaldırıldı; yerine geçen bir kod yoktur.

#### Y905 `form_ekle` — kaldırıldı

Kaldırıldı: 'form_ekle' TR'den kaldırıldı; yerine geçen bir kod yoktur.

#### Y906 `buton_ekle` — kaldırıldı

Kaldırıldı: 'buton_ekle' TR'den kaldırıldı; yerine geçen bir kod yoktur.

#### Y907 `html_kaydet` — kaldırıldı

Kaldırıldı: 'html_kaydet' TR'den kaldırıldı; yerine geçen bir kod yoktur.

#### Y908 `html_üret` — kaldırıldı

Kaldırıldı: 'html_üret' TR'den kaldırıldı; yerine geçen bir kod yoktur.

## Boru hattıyla kullanılabilen fonksiyonlar

`x |> f(a)` ≡ `f(x, a)` (bkz. `docs/dil/04_operatorler.md` 4.9). Boru yalnızca fonksiyonlarla çalışır; metotlar `.` ile zincirlenir (`l.filtrele { o > 1 }.haritala { o * 2 } |> toplam`). Borudaki değeri ilk konumlu argüman olarak alabilen yerleşik fonksiyonlar (kullanıcı fonksiyonları da kullanılabilir):

`yazdır`, `girdi`, `dosya_oku`, `dosya_yaz`, `tip`, `tam`, `ondalık`, `metin`, `aralık`, `toplam`, `min`, `maks`, `mutlak`, `karekök`, `yuvarla`, `ortalama`, `standart_sapma`, `ortanca`, `bekle`, `json_çöz`, `json_metin`, `csv_oku`, `doğrula`, `eşit_mi`, `tensör`, `sıfırlar`, `birler`, `rastgele_normal`, `birim_matris`, `birleştir`, `relu`, `sigmoid`, `tanh`, `gelu`, `softmax`, `log_softmax`, `üstel`, `log`, `sinüs`, `kosinüs`, `maks_indeks`, `geri_yayılım`, `türevsiz`, `türev_denetle`, `AdamW`, `SGD`, `gradyan_kırp`, `gömme`, `katman_normu`, `rms_normu`, `dikkat`, `çapraz_entropi`, `parametreler`, `örnekle`, `tensör_kaydet`, `tensör_yükle`, `karışık_hassasiyet`, `veri_kümesi`, `parçalayıcı_yükle`
