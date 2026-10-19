# debugger

TR'nin hata ayıklayıcısı: terminalde çalışan etkileşimli hata ayıklayıcı (`tr hata_ayıkla`,
koddaki `kesme_noktası`) ve düzenleyicilerin bağlandığı hata ayıklama sunucusu (`tr dap`,
Debug Adapter Protocol). Yüklenirken kendini yorumlayıcıya kaydeder; yorumlayıcı bu paketi
içe aktarmaz.

## Standartlar

**Tuşlar ve komutlar tek yazımlıdır.** Terminaldeki tuşlar düzenleyicinin hata ayıklama
tuşlarıyla aynıdır (F5 devam, F10 üstten geç, F11 içine gir, Shift+F11 dışarı çık); eski
yazımlar doğru olanı önerir.

**Gizli değer gizli kalır.** Adı gizli bir şey söyleyen değişkenlerin ve ortam değerlerinin
değerleri düzenleyiciye giden hiçbir iletide (değişkenler, izleme ifadeleri, İzleme Masası,
konsol çıktısı) görünmez.

**Bozuk istek çökertmez.** Düzenleyiciden gelen her istek, bozuk ya da kötü niyetli
argümanlarla bile bir yanıt alır; hata ayıklayıcı asılmaz, program sonuna kadar çalışabilir.
İleti boyu sınırlıdır.

**Motor değerleri arayüzle görünür.** Tensör, veri kümesi ve parçalayıcı gibi değerler
İzleme Masası'nda yapısal arayüzlerle (`WatchValue`, `WatchSummarizer`) gösterilir;
uygulayıcıları derleme anında denetlenir. Düzenleyiciye giden türler ve JSON alan adları
eklentiyle sözleşmedir, çevrilmez.
