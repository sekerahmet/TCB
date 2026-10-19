# files

TR'nin dosya ve klasör yerleşikleri: `yol(...)` değeri, satır okuyucu (`dosya_satırları`),
listeleme ve kalıplar, kopyalama, taşıma, silme ve geçici klasörler.

## Standartlar

**Yollar tek kuralla çözülür.** Göreli bir yol önce çalışma klasöründe, yoksa programın
klasöründe aranır; her yerleşik aynı çözümü kullanır.

**Silme korumalıdır.** Kök, ev ve sistem klasörleri, çalışma klasörü ve programın klasörü
silinmez; dolu bir klasör yalnızca açıkça istenince silinir. Bağlantılar izlenmez: silme,
kopyalama ve listeleme bir bağlantının gösterdiği yere dokunmaz ve klasörün dışına çıkmaz.

**Yarıda kalan iş hedefi bozmaz.** Kopya önce geçici bir ada yazılır, doğrulanınca yerine
konur; başka diske taşımada kaynak ancak kopya doğrulandıktan sonra silinir.

**Açık dosya yol gösterir.** Açık bir okuyucunun dosyası silinmek ya da taşınmak istenirse
işlem başlamadan, ne yapılacağını söyleyen bir hata verilir.

**Okuyucu sızmaz.** Satır okuyucular bitince, kapatılınca, erişilemez olunca ya da program
bitince kapanır; çok uzun satırlar belleği tüketmeden hatayla durur.
