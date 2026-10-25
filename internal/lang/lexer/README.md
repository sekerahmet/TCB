# lexer

Kaynak metni satır ve sütun bilgisiyle sözcük birimlerine (anahtar kelime, ad, sayı, metin,
işleç, yorum) ayırır. Hem Türkçe yazılmış kaynağı hem kodlu biçimi (`TR_kw_if`, `TR_fn_print` ...) okur.

## Standartlar

**Anahtar kelimeler sözlükten gelir.** Hangi sözcüğün anahtar kelime olduğu elle yazılmaz; kod
listesinden alınır. Burada yalnızca hangi kodun hangi sözcük birimi türünü ürettiği durur. Denetim: `TestKeywordDictionaryValid`, `TestKeywordTableComplete`, `TestKeywordMapCatchesClash`.

**Kodlu kaynakta yalnızca kodlar dilin sözcüğüdür.** Kodlu bir dosyada kod olmayan her ad
kullanıcının adıdır; başka bir dilde yazılmış bir kullanıcı adı TR'nin bir anahtar kelimesiyle aynı
olsa bile anahtar kelime sayılmaz. `TR_` ile başlayan adlar dilin kimliklerine
ayrılmıştır; kullanıcı adı olamaz (2.3a); kodlu gövdede kod listesinde olmayan kimlik sözdizimi hatasıdır. Denetim: `TestCodedTokens`, `TestEscapedName`.

**Konumlar kişinin gördüğü metne göredir.** Satır ve sütun karakter sayar, bayt değil. Kodlu
dosyada konumlar Türkçe gösterime göre hesaplanır; metin içi yerleştirmelerdeki sözcükler de
dosyadaki gerçek yerleriyle gelir. Hata mesajı, hata ayıklayıcı ve düzenleyici aynı yeri gösterir. Denetim: `TestLineAndColumnNumbers`, `TestInterpolationPositions`, `TestCodedTokens`.

**Her işin tek yazımı vardır.** Satır yorumu yalnızca `//`'dir; `#` yorum değildir ve açık bir hata
verir. Bilinmeyen kaçış dizisi hatadır, sessizce kabul edilmez. Denetim: `TestHashIsNotComment`, `TestUnknownEscapeIsError`.

**Hiçbir girdide çökmez.** Kaynak güvenilmeyen girdidir. Sözcük çözümleyici panik yapmaz, sonsuz
döngüye girmez; hataları toplar ve okumaya devam eder (bulanıklaştırma testiyle sınanır). Denetim: `FuzzLexer`, `FuzzCodedLexer`, `TestCodedPreScanDepthLimited`.
