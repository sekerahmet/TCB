# 2. Anahtar Kelimeler

- **Değişken & Sabit:** `TR_var x = 1`, `TR_const K = 2` (ilk değer her zaman yazılır; `:=` yoktur)
- **Çoklu Atama & Takas:** `TR_var a, b = 10, 20`, `a, b = b, a`, `ad, soyad = ["Mustafa", "Kemal"]`
- **Fonksiyon & Dönüş:** `TR_func`, `TR_return`, `(x) => x * 2`
- **Koşul & Eşleştirme:** `TR_if`, `TR_else TR_if`, `TR_else`, `TR_match` (varsayılan kol `_ =>`)
- **Döngüler & Liste Üreteci:** `TR_for x TR_in liste`, `1..10`, `1..=10`, `TR_break`, `TR_continue`, `[x * x TR_for x TR_in 1..=10 TR_if x % 2 == 0]`
- **Mantıksal & Boş:** `TR_true`, `TR_false`, `TR_none`, `TR_and`, `TR_or`, `TR_not` (`&&`, `||` ve tekli `!` sözdizimi hatasıdır, 4.9), `??`, `?.`
- **Tipler:** `TR_Int`, `TR_Float`, `TR_String`, `TR_Bytes`, `TR_Bool`, `TR_NoneType`, `TR_JSON`, `TR_Range`, `TR_List[T]`, `TR_Dict[K, V]`, `TR_Tensor[T]`, `T?`, demet `(A, B)`, fonksiyon `(A) -> B` (matrisler 2 boyutlu tensördür). Tip açıklamasında tip adı yalnızca kimlik yazımıyla geçerlidir: `tam` ya da `MANTIK` yazılırsa denetleyici `'TR_Int' yazın` (`'TR_Bool' yazın`) der (2.3a). Eş, gösterim ya da yabancı adlar (`tamsayı`, `int`, `harita`, `dict` ...) kabul edilmez: `'tamsayı' TR'de bir tip adı değil; 'TR_Int' yazmak mı istediniz?`
- **Yapı:** `TR_struct`, `TR_self` (kalıtım yoktur)
- **Hata, Ertele, Kesme Noktası, Modül & Eşzamanlılık:** `TR_try`, `TR_catch`, `TR_throw`, `TR_defer`, `TR_breakpoint` (koşullu: `TR_breakpoint TR_if koşul`), `TR_import "yol"`, `TR_import "yol" TR_as ad`, `TR_async`, `TR_sleep(saniye)`, `TR_wait_all()`
- **Test:** `TR_test "ad" { ... }`, `TR_assert(koşul, [mesaj])`, `TR_assert_equal(beklenen, gerçek, [mesaj])`
- **Yorumlar:** `# satır sonuna kadar` (çok satırlı yorum yoktur; her satır `#` ile başlar, 2.2). `//` yorum değildir, tam bölme işlecidir (4.2a); satır başındaki `//` ve `/*` sözdizimi hatası verir: `'//' TR'de yorum değil; yorum '#' ile yazılır: # not`.
- **Metinler:** `"çift"` tırnak (tek tırnak sözdizimi hatasıdır: `metin tek tırnakla yazılmaz; çift tırnak kullanın ("...")`, 2.6); kaçışlar `\n \t \r \" \\` ve `\u{...}`, yerleştirme `"Merhaba \(ad)"` (süslü parantez sıradan karakterdir; bilinen bir ad için `"{ad}"` ya da `"${ad}"` uyarı verir ve `"\(ad)"` önerir, 2.7). Bilinmeyen kaçış (`"\d"`, `"C:\veri"`) sözdizimi hatasıdır; düzenli ifade ve Windows yolu için **ham metin** yazın: `` `C:\veri\tokenizer.json` ``, `` `\d{4}` `` (ters tırnakta kaçış ve yerleştirme yoktur, çok satırlı olabilir). `"C:\tokenizer.json"` gibi geçerli bir kaçış (`\t` sekmedir) yola karışırsa yol alan yerleşikler dosyaya bakmadan hata verir ve ham metni önerir.
- **Sayılar:** `42`, `1_000_000`, `3.14` ve bilimsel yazım `1e-5`, `3.2E+4`, `2e10` (bilimsel yazım her zaman `TR_Float` verir; `TR_eps: 1e-5` gibi küçük değerler için kullanışlıdır).

**Her işin tek yazımı vardır.** Dilin her sözcüğü (anahtar kelime, yerleşik fonksiyon, metot ve özellik, adlı argüman, tip adı) programda yalnız kimlik yazımıyla (`TR_var`, `TR_print`, `TR_func`, `TR_Float`, `TR_lr:`; [dil tanımı 17.2, 17.7](../spec/17_kodlar.md)) yazılır; Türkçe ve İngilizce görünümler (`tr show`) yalnız okumak içindir. `TR_`'siz yazımlar (`print`, `yazdır`), Türkçe harfsiz yazımlar (`degisken`, `sonuc`, `dogru`, `yazdir`, `dogrula`, `esit_mi`) ya da başka dillerden tanıdık adlar (`def`, `int`, `str`, `lr`) dilin kelimesi değil, sıradan adlardır; tanımsızken ya da yanlış yerde kullanıldıklarında hata mesajı kimliği önerir:

```text
Tanımsız değişken veya fonksiyon: 'print'. Dilin sözcükleri TR_ ile yazılır: 'TR_print' mi demek istediniz?
Tanımsız değişken veya fonksiyon: 'degisken' ('degisken' TR'de anahtar kelime değil; 'TR_var' yazmak mı istediniz?)
Tanımsız değişken veya fonksiyon: 'yazdir' ('yazdir' TR'de yok; 'TR_print' yazmak mı istediniz?)
'ondalik' TR'de bir tip adı değil; 'TR_Float' yazmak mı istediniz?
AdamW 'ogrenme_orani' adında bir parametre almaz. Bunu mu demek istediniz: 'TR_lr'?
```

Her işin tek kimliği vardır; eş anlamlı ikinci bir ad sıradan bir addır. Başka dillerden alışılmış adlar (`int`, `str`, `lr`, `TR_while`, `TR_default`) yazılınca hata mesajı doğrusunu önerir (`'int' TR'de bir tip adı değil; 'TR_Int' yazmak mı istediniz?`); ipucu hiçbir zaman Türkçe harfsiz bir biçim önermez (`yazdir` → `TR_print`). Ayrıntı `docs/spec/02_sozcuk_yapisi.md` 2.3a ve 2.9'dadır.

## Ad kuralları ve uyarılar

`TR_` ile başlayan adlar dilin kimliklerine ayrılmıştır (`TR_sum`, `TR_input`, `TR_String`, `TR_Type` ...); kullanıcı adı `TR_` ile başlayamaz (2.3a). Bu sayede `toplam`, `girdi` ya da `input` gibi kullanıcı adları hiçbir yerleşiği gölgelemez ve serbestçe kullanılır (gölgeleme uyarısı `H1009` yalnız dil kipli birim testlerinde, `SetFileLanguage` ile çalışır).

Denetleyici ad yazılışını (2.3b) **uyarı** olarak bildirir (program yine çalışır, çıkış kodu değişmez): en dıştaki sabitler `BÜYÜK_YILAN` (`TR_const EN_BÜYÜK = 10`), yapılar `BüyükDeve` (`TR_struct Kutu`), öbür adlar `küçük_yılan` (`TR_var toplam_tutar = 3`; matematik adları `W`, `dW`, `Wq` muaftır). Uyarılar standart hataya, tip hatalarından ayrı bir başlıkla (`⚠️ Uyarılar:`) yazılır ve önerilen yazımı verir.

Dilin kuralları örnekleriyle birlikte [`docs/spec/`](../spec/README.md) klasöründe bölüm bölüm tanımlanmıştır; her kuralın `tests/` klasöründe çalıştırılabilir bir testi vardır (`tr test tests`). Her anahtar kelimenin, yerleşiğin, metodun ve adlı argümanın imzası, açıklaması ve örneği [`docs/codes.md`](../codes.md)'dedir.
