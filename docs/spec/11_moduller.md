# 11. Modüller

Bu bölümün kurallarına göre bugünkü durum ve yapılacaklar: [`docs/status.md`](../status.md), "11. Modüller".

Her kuralın emsali ve gerekçesi: [`rationale/11_modules.md`](rationale/11_modules.md).

Bir program başka `.tr` dosyalarının (modüllerin) tanımlarını ve TR'nin kendi kitaplığını kullanır. Bu bölüm modülün nasıl bulunduğunu, adlarının nasıl geldiğini ve ne zaman denetlendiğini söyler.

**11.1** `TR_import "yol"` başka bir `.tr` dosyasının tanımlarını yükler ve en dış adlarını dosyanın kapsamına getirir.
- Aktarmalar dosyanın başında, başlıktan sonra ve öbür tanımlardan önce yazılır (2.1).
- Modülün en dışında yalnız tanımlar bulunur; ilk değerler sabittir (1.1). Bir modülün en dışında çalışan bir deyim sözdizimi hatasıdır.
- Yol bir metindir; `.tr` uzantısı isteğe bağlıdır.
- **Göreli yol** önce aktaran dosyanın klasörüne, bulunamazsa programın klasörüne göre çözülür; çalışma klasörüne bakılmaz (dosyası olmayan kabukta yol çalışma klasörüne göredir). Bu, veri dosyalarının yol kuralından (13) bilerek ayrılır: kod yazıldığı dosyanın yanındadır.
- Modül bulunamazsa ileti bakılan yerleri sayar. `tr/` önekli yollar diskte aranmaz (11.6).
- *Test:* TR testi, olumsuz fikstür.

**11.2** `TR_import "yol" TR_as m` modülü tek bir **modül değeri** olarak `m`'ye bağlar. Adlarına `m.ad` ile erişilir ve her adın tipi çalışmadan önce bilinir: `m.kare(2)`, `m.PI`.
- Modül değeri bir sözlük değildir: `m["PI"]` denetim hatasıdır, ileti `m.PI`'yi önerir.
- Olmayan bir ad denetim hatasıdır; ileti modülün en yakın adını önerir.
- *Test:* TR testi; köşeli ayraçla erişimin ve olmayan adın denetim hatası uygulanınca olumsuz fikstür.

**11.3** Bir modül program boyunca bir kez yüklenir; sonraki aktarımlar aynı değerleri paylaşır. Başka bir görev aynı modülü o sırada yüklüyorsa aktarım yüklemenin bitmesini bekler. Döngüsel aktarım (yüklenmekte olan bir modülün kendi yükleme zincirinden yeniden aktarılması) hatadır.
- *Test:* TR testi.

**11.4** Program çalışmadan önce kendisi ve aktardığı bütün modüller (aktarılanların aktardıkları da) ayrıştırılır ve denetlenir (1.3). Herhangi birinde hata varsa programın hiçbir satırı çalışmaz; hata `TR_try`/`TR_catch` ile yakalanmaz, başlığı hatanın hangi modülde olduğunu söyler.
- Modülün en dışında `TR_return` sözdizimi hatasıdır: modül bir fonksiyon değildir.
- *Test:* TR testi, olumsuz fikstür.

**11.5** Kod tanımlandığı dosyanın bağlamında çalışır: bir modülde tanımlanan fonksiyon nereden çağrılırsa çağrılsın o modülün dosyasındadır.
- Göreli dosya yolları 13'ün kuralıyla, göreli aktarma yolları 11.1'e göre o modülün klasöründen çözülür.
- Başka bir dosyadan gelen çalışma hatası o dosyanın adı ve kendi satırıyla yazılır; `TR_throw` ile atılan değer değişmeden yakalanır.
- Dosya bilgisi görevindir: bir görevin modül yüklerken beklemesi öbür görevlerin yollarını değiştirmez.
- *Test:* TR testi.

**11.6** `tr/` öneki TR'nin kendi kitaplığına ayrılmıştır: `TR_import "tr/<modül>"` diskten değil `tr` ikilisine gömülü kitaplıktan yüklenir.
- Kitaplığın adları koddur (17.1). Modül adları İngilizcedir, kimliklerin adları gibi (17.2, 17.9).
- Diskteki bir `tr` klasörü bu öneki gölgeleyemez; kullanıcının `tr` adlı klasörü `./tr/...` ile aktarılır.
- Kitaplık modülünün içindeki göreli aktarma kitaplıkta çözülür.
- Olmayan modül, `..` ile kitaplık dışına çıkan yol ve kitaplığın test dosyaları hatadır; ileti kitaplığın modüllerini ve en yakın adı söyler.
- `TR_cmd_check` ve `TR_cmd_build` kitaplık modülünün varlığını denetler; kitaplık `TR_cmd_build`'in çıktısına, yorumlayıcının parçası olarak girer.
- Kitaplığın sürümü `tr` ikilisinin sürümüdür; aynı ikili aynı kitaplığı verir (20.17).
- *Test:* TR testi, olumsuz fikstür.

**11.7** Başı `_` olan üst düzey adlar (tek başına `_` değil) modüle özeldir: modülün içinde kullanılır, dışa aktarılmaz. `TR_import "yol"` onları getirmez; modül değerinde (`m._iç`) yoktur, yazılırsa ileti adın modüle özel olduğunu söyler. Yalnız üst düzey adlara uygulanır.
- *Test:* TR testi.

**11.8** Ad çakışması denetim hatasıdır: iki aktarım aynı adı getirirse ya da dosya aktarılan bir adı yeniden tanımlarsa ileti iki yeri de gösterir ve `TR_as` ile aktarmayı önerir. Sonra gelenin sessizce kazanması yoktur.
- **Aktarılan ad aktaran dosyada yeniden bağlanamaz:** takma adsız aktarılan bir ada atama (`PI = 7`) ve modül değerinin bir adına atama (`m.PI = 7`) denetim hatasıdır. Modülün en dışındaki değişken yalnız modülün kendi kodundan, onun fonksiyonlarıyla değişir; ileti modülün fonksiyonunu kullanmayı söyler.
- Değerin içi değişebilir: modülün listesine eklemek (`m.kayıt.TR_append(x)`) modülün kendi listesini değiştirir ve her aktaran onu görür (11.3). İkisi yan yana: `m.PI = 7` hatadır; `m.nokta.x = 1` modülün yapısının alanını değiştirir (7), geçerlidir. Görevlerden yazmak 10.2'ye göre bozulmaz, sırası 10.8'e göre tanımsızdır.
- *Test:* gerekmez, henüz uygulanmadı; uygulanınca olumsuz fikstür.
