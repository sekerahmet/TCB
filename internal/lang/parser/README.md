# parser

Sözcük birimlerinden sözdizimi ağacını kurar. Deyimler özyinelemeli inişle, ifadeler öncelik
tabanlı (Pratt) yöntemle ayrıştırılır.

## Standartlar

**Hatalar kullanıcının sözleriyle anlatılır.** Mesajlarda iç sözcük birimi adları geçmez; "dosya
sonu", "'sonuç' anahtar kelimesi", "'x' adı" gibi anlatımlar kullanılır. Her hata satır ve sütunla
başlar. Denetim: `TestErrorMessagesUseTurkishDescriptions`.

**Tek hata, tek mesaj.** Bir hatadan sonra ayrıştırıcı bir sonraki deyime toparlanır; tek bir
yanlış zincirleme hatalar üretmez. Denetim: `TestSingleErrorNoCascade`.

**Hiçbir girdide çökmez.** Bozuk kaynak panik yerine hata verir. İç içelik sınırlıdır; aşırı derin
kaynak yığını taşırmadan hatayla reddedilir. Denetim: `FuzzParser`, `TestNestingDepthLimit`, `TestInterpolationDepthLimited`, `TestMalformedStructMethodReportsErrorInsteadOfPanic`.

**Bağlam kuralları burada denetlenir.** `TR_kw_break` ve `TR_kw_continue` yalnızca döngüde, `TR_kw_return` yalnızca
fonksiyonda geçerlidir; ihlal çalıştırmadan önce sözdizimi hatasıdır. Denetim: `TestBreakContinueOutsideLoopIsError`, `TestReturnInTestBodyIsError`, `TestModuleTopLevelReturnIsError`.

**Ağaçta düğümü olmayan adlar da kaybolmaz.** Tip açıklamaları, üye ve alan adları, bağlamsal
sözcükler gibi ayrı düğümü olmayan adların görevi kaydedilebilir; kodlu biçime çevirici bu
kayda dayanır. Denetim: `TestRolesRecordedForNamesWithoutNodes`, `TestRolesOffByDefault`.
