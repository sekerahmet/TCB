# debugger

TR'nin hata ayıklayıcısı: terminalde çalışan etkileşimli hata ayıklayıcı (`tr hata_ayıkla`,
koddaki `kesme_noktası`) ve düzenleyicilerin bağlandığı hata ayıklama sunucusu (`tr dap`,
Debug Adapter Protocol). Yüklenirken kendini yorumlayıcıya kaydeder; yorumlayıcı bu paketi
içe aktarmaz.

## Standartlar

**Tuşlar ve komutlar tek yazımlıdır.** Terminaldeki tuşlar düzenleyicinin hata ayıklama
tuşlarıyla aynıdır (F5 devam, F10 üstten geç, F11 içine gir, Shift+F11 dışarı çık); eski
yazımlar doğru olanı önerir. Denetim: `TestTerminalKeysMatchVSCode`, `TestCommandsSingleSpelling`.

**Gizli değer gizli kalır.** Adı gizli bir şey söyleyen değişkenlerin ve ortam değerlerinin
değerleri düzenleyiciye giden hiçbir iletide (değişkenler, izleme ifadeleri, İzleme Masası,
konsol çıktısı) görünmez. Denetim: `TestDAPSecretMasked` (behavior_dap_secrets_test.go), `TestDAPSecretDoesNotBreakProtocol`.

**Bozuk istek çökertmez.** Düzenleyiciden gelen her istek, bozuk ya da kötü niyetli
argümanlarla bile bir yanıt alır; hata ayıklayıcı asılmaz, program sonuna kadar çalışabilir.
İleti boyu sınırlıdır. Denetim: `FuzzDAPMessage`, `FuzzDAPPauseRequest`, `TestDAPContentLengthLimit`.

**Motor değerleri arayüzle görünür.** Tensör, veri kümesi ve parçalayıcı gibi değerler
İzleme Masası'nda yapısal arayüzlerle (`WatchValue`, `WatchSummarizer`) gösterilir;
uygulayıcıları derleme anında denetlenir. Düzenleyiciye giden türler ve JSON alan adları
eklentiyle sözleşmedir, çevrilmez. Denetim: `TestWatchValueInterface` (derleme anı denetimi), `TestDAPWireFieldsHaveExplicitNames`.

## Testler

Birim testleri sınadıkları dosyanın adını taşır (`x_test.go` → `x.go`). Birden çok dosyayı kesen davranış testleri:

- `behavior_dap_coded_files_test.go`: kodlu dosyanın düzenleyicide Türkçe görünümüyle açılması: kesme noktaları, çağrı yığını ve kaynak yolları
- `behavior_dap_modules_test.go`: modül satırları, protokol alanlarıyla çakışan gizli değer ve göreli program yolu (DAP)
- `behavior_dap_scope_test.go`: aynı adlı genel/yerel değişkenin değer değiştirme ve tablo isteğinde kapsama göre seçilmesi
- `behavior_dap_secrets_test.go`: gizli değerlerin düzenleyiciye giden hiçbir iletide görünmemesi
- `behavior_dap_tensor_variables_test.go`: tensörlerin değişkenler panelinde açılması ve okunamayan program dosyası hatası
- `behavior_dap_watch_panel_test.go`: İzleme Masası düğmelerinin gönderdiği DAP isteklerinin işlenmesi
- `behavior_dap_wire_names_test.go`: protokol yapılarının bütün dışa açık alanlarının açık json adı taşıması
