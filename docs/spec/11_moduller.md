# 11. Modüller

Bu bölümün kurallarına göre bugünkü durum ve yapılacaklar: [`docs/status.md`](../status.md), "11. Modüller".

Her kuralın emsali ve gerekçesi: [`rationale/11_modules.md`](rationale/11_modules.md).

Bir program başka `.tr` dosyalarının (modüllerin) tanımlarını ve TR'nin kendi kitaplığını kullanır. Bu bölüm modülün nasıl bulunduğunu, adlarının nasıl geldiğini ve ne zaman denetlendiğini söyler.

**11.1** `TR_import "yol"` başka bir `.tr` dosyasının tanımlarını yükler ve en dış adlarını dosyanın kapsamına getirir.
- Aktarmalar dosyanın başında, başlıktan sonra ve öbür tanımlardan önce yazılır (2.1).
- Modülün en dışında yalnız tanımlar bulunur; ilk değerler sabittir (1.1). Bir modülün en dışında çalışan bir deyim denetim hatasıdır.
- Yol bir metindir; `.tr` uzantısı isteğe bağlıdır.
- Yolun her bileşeninin harf büyüklüğü, göreli yolda da mutlak yolda da, diskteki adla her sistemde aynı olmalıdır; farklıysa hata, ileti diskteki adı önerir (18.5 gibi). Böylece harf büyüklüğüne duyarsız sistemde aynı modül iki yazımla iki kez yüklenmez (11.3). Windows'ta sürücü harfinin büyüklüğü fark etmez.
- **Göreli yol** yalnız aktaran dosyanın klasörüne göre çözülür; çalışma klasörüne ve programın klasörüne bakılmaz (dosyası olmayan kabukta yol çalışma klasörüne göredir). Bulunamazsa hata; programın klasöründe aynı adlı dosya varsa ileti onu açık yolla önerir (`"../ad"`). Bu, veri dosyalarının yol kuralından (13) bilerek ayrılır: kod yazıldığı dosyanın yanındadır.
- Modül bulunamazsa ileti bakılan yerleri sayar. `tr/` önekli yollar diskte aranmaz (11.6).
- *Test:* TR testi, olumsuz fikstür; mutlak yolun harf büyüklüğü: Go `TestAnAbsoluteImportUsesTheDisksCase` (sürücü harfinin büyüklüğü dahil).

**11.2** `TR_import "yol" TR_as m` modülü tek bir **modül değeri** olarak `m`'ye bağlar. Adlarına `m.ad` ile erişilir ve her adın tipi çalışmadan önce bilinir: `m.kare(2)`, `m.PI`.
- Modül değeri bir sözlük değildir: `m["PI"]` denetim hatasıdır, ileti `m.PI`'yi önerir.
- Olmayan bir ad denetim hatasıdır; ileti modülün en yakın adını önerir.
- *Test:* TR testi; olumsuz fikstür `tests/negative/modul_koseli_ayrac.tr` (köşeli ayraç); olmayan adın denetim hatası uygulanınca olumsuz fikstür.

**11.3** Bir modül program boyunca bir kez yüklenir; sonraki aktarımlar aynı değerleri paylaşır. Döngüsel aktarım (yüklenmekte olan bir modülün kendi yükleme zincirinden yeniden aktarılması) hatadır.
- Aynı dosya aynı modüldür: başka bir yazımla (bağlantı, dizin kavşağı, sabit bağ, kısa ad, sürücü harfi) ulaşılan dosya da aynı modüldür. Aynı dosyaya ulaşan iki yazımın klasörleri basamak basamak (klasör ve bütün üst klasörleri, köke kadar) aynı klasörler değilse denetim hatasıdır (H2726). Klasörleri basamak basamak aynı olan başka bir yazım hata değildir: aynı klasördeki dosya bağlantısı ya da sabit bağ; hedefiyle aynı üst klasörde duran dizin bağlantısı ya da kavşağı; kısa ad; sürücü harfinin büyük ya da küçük yazılışı. Başka bir üst klasördeki bağlantıdan aynı klasöre ulaşmak H2726'dır. Yükleme zincirindeki bir dosyaya ya da ana programa ulaşan yazım, klasörleri aynı olmasa da döngüdür (H2481), H2726 değil.
- *Test:* TR testi; Go `TestAModuleThroughAnotherPathLoadsOnce` (bağlantı, sabit bağ, dizin kavşağı, sürücü harfi), `TestAModuleThroughItsShortNameLoadsOnce` (8.3, yalnız Windows), `TestAModuleThroughAnotherFolderIsAnError` (H2726), `TestAnImportCycleThroughALinkIsAnError`, `TestBuildEmbedsAModuleReachedTwoWaysOnce`.

**11.4** Program çalışmadan önce kendisi ve aktardığı bütün modüller (aktarılanların aktardıkları da) ayrıştırılır ve denetlenir (1.3). Herhangi birinde hata varsa programın hiçbir satırı çalışmaz; hata `TR_try`/`TR_catch` ile yakalanmaz, başlığı hatanın hangi modülde olduğunu söyler.
- Modülün en dışında `TR_return` sözdizimi hatasıdır: modül bir fonksiyon değildir.
- *Test:* TR testi, olumsuz fikstür.

**11.5** Kod tanımlandığı dosyanın bağlamında çalışır: bir modülde tanımlanan fonksiyon nereden çağrılırsa çağrılsın o modülün dosyasındadır.
- Göreli dosya yolları 13'ün kuralıyla, göreli aktarma yolları 11.1'e göre o modülün klasöründen çözülür.
- Başka bir dosyadan gelen çalışma hatası o dosyanın adı ve kendi satırıyla yazılır; `TR_throw` ile atılan değer değişmeden yakalanır.
- Dosya bilgisi çağrınındır: bir görevde çalışan bir modül fonksiyonu göreli yolu kendi modülünün klasörüyle okur (18.16).
- *Test:* TR testi.

**11.6** `tr/` öneki TR'nin kendi kitaplığına ayrılmıştır: `TR_import "tr/<modül>"` diskten değil `trlang` ikilisine gömülü kitaplıktan yüklenir.
- Kitaplığın adları koddur (17.1). Modül adları İngilizcedir, kimliklerin adları gibi (17.2, 17.9).
- Diskteki bir `tr` klasörü bu öneki gölgeleyemez; kullanıcının `tr` adlı klasörü `./tr/...` ile aktarılır.
- Kitaplık modülünün içindeki göreli aktarma kitaplıkta çözülür.
- Olmayan modül, `..` ile kitaplık dışına çıkan yol ve kitaplığın test dosyaları hatadır; ileti kitaplığın modüllerini ve en yakın adı söyler.
- `TR_cmd_check` ve `TR_cmd_build` kitaplık modülünün varlığını denetler; kitaplık `TR_cmd_build`'in çıktısına, yorumlayıcının parçası olarak girer.
- Kitaplığın sürümü `trlang` ikilisinin sürümüdür; aynı ikili aynı kitaplığı verir (20.17).
- *Test:* TR testi, olumsuz fikstür.

**11.7** Başı `_` olan üst düzey adlar (tek başına `_` değil) modüle özeldir: modülün içinde kullanılır, dışa aktarılmaz. `TR_import "yol"` onları getirmez; modül değerinde (`m._iç`) yoktur, yazılırsa ileti adın modüle özel olduğunu söyler. Yalnız üst düzey adlara uygulanır.
- *Test:* TR testi.

**11.7a** Kitaplığın iç yerleşiği yalnız tanım dosyasında adı yazılı kitaplık modülünün kaynağından çağrılır ve anılır; başka yerden çağrı da anma da denetim hatasıdır ve ileti yerleşiğin hizmet ettiği genel yolu söyler.
- Kullanıcı programı iç yerleşiği çağıramaz ve anamaz (`TR_var f = TR_cache_buffer` de hatadır); kod listesinde ve tamamlamada iç olduğu yazılır; adı yine kodludur (17).
- İç yerleşiğin kod listesinde örneği yoktur: örnek bir kullanıcı programıdır, iç yerleşiği çağıramaz.
- İlk iç yerleşik `TR_cache_buffer`'dır (`tr/layers`, 15.43).
- *Test:* TR testi: olumsuz fikstür `tests/negative/ic_yerlesik.tr` (kullanıcı programında çağrı ve anma) ve `lib/layers_test.tr` (`tr/layers`'tan çağrı); Go: `TestInternalBuiltinOnlyInItsLibrary`, `TestInternalBuiltinIsNeverSuggested`, kod listesinin örnek testi `TestInternalBuiltinHasNoExample`.

**11.8** Ad çakışması denetim hatasıdır: iki aktarım aynı adı getirirse ya da dosya aktarılan bir adı yeniden tanımlarsa ileti iki yeri de gösterir ve `TR_as` ile aktarmayı önerir. Sonra gelenin sessizce kazanması yoktur.
- **Aktarılan ad aktaran dosyada yeniden bağlanamaz:** takma adsız aktarılan bir ada atama (`PI = 7`) ve modül değerinin bir adına atama (`m.PI = 7`) denetim hatasıdır. Modülün en dışındaki değişken yalnız modülün kendi kodundan, onun fonksiyonlarıyla değişir; ileti modülün fonksiyonunu kullanmayı söyler.
- Değerin içi değişebilir: modülün listesine eklemek (`m.kayıt.TR_append(x)`) modülün kendi listesini değiştirir ve her aktaran onu görür (11.3). İkisi yan yana: `m.PI = 7` hatadır; `m.yer.x = 1` modülün yapısının alanını değiştirir (7), geçerlidir. Görevlerden yazmak 10.2'ye göre bozulmaz, sırası 10.8'e göre tanımsızdır.
- *Test:* TR testi ve olumsuz fikstür (modül değerinin adına atama); çakışma ve takma adsız aktarılan ada atama uygulanınca olumsuz fikstür.
