# 3. Tipler ve değerler

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
