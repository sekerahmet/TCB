# lexer

Kaynak metni satır ve sütun bilgisiyle sözcük birimlerine (anahtar kelime, ad, sayı, metin,
işleç, yorum) ayırır. Kodlu biçimi (`TR_if`, `TR_print` ...; içeride kod `TR_kw_if`, `TR_fn_print`) ve
bir yazım dilindeki görünümü (kabuk satırı, `tr kodla`) okur.

## Standartlar

**Anahtar kelimeler sözlükten gelir.** Hangi sözcüğün anahtar kelime olduğu elle yazılmaz; kod
listesinden alınır. Burada yalnızca hangi kodun hangi sözcük birimi türünü ürettiği durur. Denetim: `TestKeywordDictionaryValid`, `TestKeywordTableComplete`, `TestKeywordMapCatchesClash`.

**Kodlu kaynakta yalnızca kodlar dilin sözcüğüdür.** Kodlu bir dosyada kod olmayan her ad
kullanıcının adıdır; başka bir dilde yazılmış bir kullanıcı adı TR'nin bir anahtar kelimesiyle aynı
olsa bile anahtar kelime sayılmaz. `TR_` ile başlayan adlar dilin kimliklerine
ayrılmıştır; kullanıcı adı olamaz (2.3a); kodlu gövdede kod listesinde olmayan kimlik sözdizimi hatasıdır. Denetim: `TestCodedTokens`, `TestEscapedName`.

**Konumlar dosyanın metnine göredir (2.1).** Satır ve sütun karakter sayar, bayt değil; başlık 1.
satırdır. Kimliklerle okunan gövdede konum diskteki metindir (`NewBody`); kişinin dilinde yazılan
metnin (kabuk satırı) konumlarını `NewBodyView` verir. Metin içi yerleştirmelerdeki sözcükler de
dosyadaki gerçek yerleriyle gelir. Hata mesajı, hata ayıklayıcı ve düzenleyici aynı yeri gösterir. Denetim: `TestLineAndColumnNumbers`, `TestInterpolationPositions`, `TestCodedTokens`.

**Her işin tek yazımı vardır (2.2, 2.6).** Satır yorumu `#`'dir; `//` taban bölmedir. Satır başındaki
`//` ve `/*` yorum değildir, açık bir hata verir ve `#`'i önerir. Bilinmeyen kaçış dizisi hatadır,
sessizce kabul edilmez. İleti kodu bir metindir (`TR_Message("H6903", x)`). Denetim: `TestComments`,
`TestCStyleCommentIsAnError`, `TestUnknownEscapeIsError`.

**Hiçbir girdide çökmez.** Kaynak güvenilmeyen girdidir. Sözcük çözümleyici panik yapmaz, sonsuz
döngüye girmez; hataları toplar ve okumaya devam eder (bulanıklaştırma testiyle sınanır). Denetim: `FuzzLexer`, `FuzzCodedLexer`.
