# turkish

TR'nin mesajlarında doğru Türkçe için dilbilgisi ve metin yardımcıları: ünlü uyumuna göre hal
ekleri, sayının okunuşuna göre ek (`3'ü`, `5'e`), Türkçe büyük/küçük harf, en yakın ad önerisi,
kullanıcıya giden dosya hatası metinleri ve kaynak kod çerçevesi.

## Standartlar

**En alt katman.** Hiçbir TCB paketini içe aktarmaz; dilin adlarını ve kodlarını bilmez. Yalnızca
Türkçe ve metinle ilgilenir; böylece her paket onu kullanabilir.

**Ekler sözcükten çıkarılır.** Hal eki elle yazılmaz (`'i`, `'ı`, `'e`); sözcüğün ünlü uyumundan
ve son sesinden, sayılarda okunuşundan çıkarılır. Ad değişse de ek doğru kalır.

**Türkçe harf kuralları.** Kullanıcıya giden metinde büyük/küçük harf dönüşümü Türkçe kurallarıyla
yapılır (`i` ↔ `İ`, `ı` ↔ `I`).

**Öneri yazana göredir.** Tanımsız bir ada en yakın ad önerilirken Türkçe karakterle yazana Türkçe,
ASCII yazana ASCII aday öne çıkar.

**Dosya hataları anlaşılır.** Bir dosya hatası işletim sisteminin ham metniyle gösterilmez: hangi
yolun, hangi işlemde, neden başarısız olduğu ve ne yapılacağı Türkçe söylenir. Anlatım her işletim
sisteminde aynıdır; tanınmayan bir hata da Türkçe bir mesajla gösterilir.
