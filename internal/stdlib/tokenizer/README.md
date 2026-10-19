# tokenizer

Genel `tokenizer.json` parçalayıcısını TR diline bağlar: `parçalayıcı_yükle` ve parçalayıcı
nesnesinin kodlama, çözme, dağarcık boyutu ve parça kimliği üyeleri.

## Standartlar

**Sonuç başvuru uygulamasıyla aynıdır.** TR'den kodlama ve çözme, hesap katmanındaki
parçalayıcının Go'dan verdiği sonucun aynısını verir.

**Yarım sonuç yoktur.** Düzenli ifade adım ya da bellek sınırı aşılırsa yarım bir dizi yerine
yol gösterici bir TR hatası oluşur.

**Uzun iş yorumlayıcıyı kilitlemez.** Dosya yükleme ve liste kodlama/çözme yorumlayıcı kilidi
bırakılarak yapılır; toplu işler paralel koşar ama sonuç sırası girdi sırasıdır.
