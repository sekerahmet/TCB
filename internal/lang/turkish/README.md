# turkish

TR'nin mesajlarında doğru Türkçe için dilbilgisi ve metin yardımcıları: ünlü uyumuna göre hal
ekleri, sayının okunuşuna göre ek (`3'ü`, `5'e`), Türkçe büyük/küçük harf, en yakın ad önerisi,
kullanıcıya giden dosya hatası metinleri ve kaynak kod çerçevesi.

## Standartlar

**Alt katman.** TCB paketlerinden yalnızca ileti kataloğunu (`internal/lang/messages`) içe aktarır ve Türkçe çekimciyi ona kaydeder; dilin adlarını ve kodlarını bilmez. Yalnızca
Türkçe ve metinle ilgilenir; böylece her paket onu kullanabilir.

**Ekler sözcükten çıkarılır.** Hal eki elle yazılmaz (`'i`, `'ı`, `'e`); sözcüğün ünlü uyumundan
ve son sesinden, sayılarda okunuşundan çıkarılır. Ad değişse de ek doğru kalır. Denetim: `TestSuffixWord`, `TestNumberSuffix`, `TestSuffixLongAndNonASCIINumbers`, `TestTemplateInflector`.

**Türkçe harf kuralları.** Kullanıcıya giden metinde büyük/küçük harf dönüşümü Türkçe kurallarıyla
yapılır (`i` ↔ `İ`, `ı` ↔ `I`). Denetim: `TestTurkishCaseConversion`.

**Öneri yazana göredir.** Tanımsız bir ada en yakın ad önerilirken Türkçe karakterle yazana Türkçe,
ASCII yazana ASCII aday öne çıkar. Denetim: `TestClosest`.

**Düzenleme uzaklığı tektir.** `Distance(a, b, bound)` kod noktası olarak Levenshtein uzaklığıdır; sınır aşılınca
bound+1 verir. `Closest` gerçek uzaklığı alır (sınır iki uzunluğun büyüğü); `internal/devtools/codeindex` ve
`tools/codegraph`'ın yakın ad önerileri de bunu çağırır (katman: devtools lang'ı içe aktarır). Denetim:
`TestDistanceAndClosestUnchanged`.

**Dosya hataları anlaşılır.** Bir dosya hatası işletim sisteminin ham metniyle gösterilmez: hangi
yolun, hangi işlemde, neden başarısız olduğu ve ne yapılacağı Türkçe söylenir. Anlatım her işletim
sisteminde aynıdır; tanınmayan bir hata da Türkçe bir mesajla gösterilir. Denetim: `TestFileErrorTurkishAndHelpful`, `TestFileErrorUnknownStatusCode`, `TestFileErrorReasonThroughCodedWrapper`.
