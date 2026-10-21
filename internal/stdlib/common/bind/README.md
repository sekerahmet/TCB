# bind

Motor paketlerinin (tensör, parçalayıcı, veri, dosya ...) TR diline bağlanırken paylaştığı
yardımcılar: adlı argüman denetimi, göreli yol çözme, paniği hataya çevirme ve sayıların kısa
yazımı.

## Standartlar

**Kullanıcıya görünen ad tanımlamaz.** Adlı argümanların adları dilin kod listesindedir; motorlar
yalnızca kodları kullanır, ad değişikliği yalnızca dil dosyasını değiştirir.
Denetim: `TestNoBareCodes` (`tools/gen/codeconsts`), `TestNamedArg`.

**Yanlış yazılmış ad sessizce geçmez.** Bilinmeyen bir adlı argüman hatadır ve en yakın geçerli ad
önerilir; başka dillerden alışılmış adlar da tanınıp doğrusu gösterilir.
Denetim: `TestCheckNamedSuggestionOrder`.

**Panik programı çökertmez.** Motordaki bir Go paniği, TR'nin `dene/yakala` ile yakalayabildiği
bir hataya çevrilir; kullanıcının kesmesi (Ctrl+C) yutulmaz.
Denetim: `TestPanicBecomesError`, `TestRecoverKeepsInterrupt`.

**Göreli yollar dilin tek yol kuralıyla çözülür.** Motorlar dosya yollarını kendileri çözmez;
dilin kuralı kullanılır, bulunamayan yolun hatası bakılan yerleri söyler.
Denetim: `TestResolvePath`; kuralın motorlarda uygulanması için `TestUserPathsResolvedByLanguageRule` (`internal/stdlib`).
