# interp

TR'nin derleyicisi ve yorumlayıcısı. Sözdizimi ağacı çalışmadan önce bir kez Go kapanışlarına
derlenir; değerler, çekirdek yerleşikler ve metotlar, modüller, eşzamanlı görevler ve yorumlayıcı
kilidi bu pakettedir.

## Standartlar

**Bir kez derlenir, adla aranmaz.** Değişkenler derleme anında yuvalara, üye adları kodlara
çözülür; çalışma anında ad karşılaştırılmaz, yalnızca kod ve yuva kullanılır.

**Motorlar kancayla bağlanır.** Tensör, veri, parçalayıcı motorları ve hata ayıklayıcı bu paketi
içe aktarıp kendilerini kancalarla bağlar; yorumlayıcı onları bilmez. Yeni bir yetenek önce bir
kanca arayüzü olarak burada tanımlanır, uygulaması motorda yazılır.

**TR kodu yalnızca kilidi tutan görevde çalışır.** Eşzamanlı her görev yorumlayıcı kilidini alarak
çalışır. Uzun bekleyen işler (uyku, disk, ağ, dış süreç) kilidi bırakarak bekler; kilidi tutmayan
kod ortamlara ve değerlere dokunmaz.

**Hatalar satırıyla ve bir kez sarılır.** Çalışma hatası oluştuğu satırı taşır; zaten satır taşıyan
bir hata yeniden sarılmaz. Kullanıcı hataları `dene`/`yakala` ile yakalanır; kesme isteği (Ctrl+C)
yakalanmaz. Bir yerleşiğin içindeki beklenmedik panik süreci çökertmez, iç hata olarak bildirilir.

**Tek kural.** Göreli dosya yolları için tek kural, modül yolları için tek kural vardır; her
yerleşik yolu aynı yerden çözer.

**Sıra belirlenimlidir.** Sözlükler ekleme sırasını korur, yapı alanları sabit bir sırayla gezilir;
çıktı harita gezinme sırasına bağlı olmaz.

**Sıcak yol ölçülerek değişir.** Yorumlayıcı döngüsündeki ve hızlı yollardaki değişiklikler
önce/sonra hız ölçümüyle yapılır.
