# tokenizer

Genel `tokenizer.json` parçalayıcısını TR diline bağlar: `TR_fn_load_tokenizer` ve parçalayıcı
nesnesinin kodlama, çözme, dağarcık boyutu ve parça kimliği üyeleri.

## Standartlar

**Sonuç başvuru uygulamasıyla aynıdır.** TR'den kodlama ve çözme, hesap katmanındaki
parçalayıcının Go'dan verdiği sonucun aynısını verir.
Denetim: `TestEncodeDecodeSameAsGo`.

**Yarım sonuç yoktur.** Düzenli ifade adım ya da bellek sınırı aşılırsa yarım bir dizi yerine
yol gösterici bir TR hatası oluşur.
Denetim: `TestStepLimitAndWarning`, `TestLoadErrors`.

**Uzun iş yorumlayıcıyı kilitlemez.** Dosya yükleme ve liste kodlama/çözme yorumlayıcı kilidi
bırakılarak yapılır; toplu işler paralel koşar ama sonuç sırası girdi sırasıdır.
Denetim: `TestHeavyWorkReleasesLock` (ağır çağrılar `Blocking`/`maybeRelease` içindedir; kaynak taraması), `TestEncodeDecodeSameAsGo` (toplu sonuç sırası).
