# 2. Anahtar Kelimeler

- **Değişken & Sabit:** `TR_var x = 1`, `TR_const K = 2` (ilk değer her zaman yazılır; `:=` yoktur)
- **Çoklu Atama & Takas:** `TR_var a, b = 10, 20`, `a, b = b, a`, `ad, soyad = ["Mustafa", "Kemal"]`
- **Fonksiyon & Dönüş:** `TR_func`, `TR_return`, `(x) => x * 2`
- **Koşul & Eşleştirme:** `TR_if`, `TR_else TR_if`, `TR_else`, `TR_match` (varsayılan kol `_ =>`)
- **Döngüler & Liste Üreteci:** `TR_for x TR_in liste`, `1..10`, `1..=10`, `TR_break`, `TR_continue`, `[x * x TR_for x TR_in 1..=10 TR_if x % 2 == 0]`
- **Mantıksal & Boş:** `TR_true`, `TR_false`, `TR_none`, `TR_and`, `TR_or`, `TR_not` (`&&`, `||` ve tekli `!` sözdizimi hatasıdır, 4.9), `??`, `?.`
- **Tipler:** `TR_Int`, `TR_Float`, `TR_String`, `TR_Bool`, `TR_NoneType`, `TR_List`, `TR_Dict`, `TR_Tensor` (matrisler 2 boyutlu tensördür). Tip açıklamasında tip adı yalnızca kendi yazımıyla geçerlidir: `tam` ya da `MANTIK` yazılırsa denetleyici `'Tam' yazın` der (3.14). Eş ya da İngilizce adlar (`tamsayı`, `int`, `harita`, `dict` ...) kabul edilmez: `'tamsayı' TR'de bir tip adı değil; 'Tam' yazmak mı istediniz?`
- **Yapı:** `TR_struct`, `TR_self` (kalıtım yoktur)
- **Hata, Ertele, Kesme Noktası, Modül & Eşzamanlılık:** `TR_try`, `TR_catch`, `TR_throw`, `TR_defer`, `TR_breakpoint` (koşullu: `TR_breakpoint TR_if koşul`), `TR_import "yol"`, `TR_import "yol" TR_as ad`, `TR_async`, `TR_sleep(saniye)`, `TR_wait_all()`
- **Test:** `TR_test "ad" { ... }`, `TR_assert(koşul, [mesaj])`, `TR_assert_equal(beklenen, gerçek, [mesaj])`
- **Yorumlar:** `// satır sonuna kadar` ve `/* çok satırlı */`. `#` yorum değildir; sözdizimi hatası verir: `'#' TR'de yorum değil; yorum için '//' yazın`.
- **Metinler:** `"çift"` ya da `'tek'` tırnak; kaçışlar `\n \t \r \" \' \\ \{ \}`, yerleştirme `"Merhaba {ad}"`. Bilinmeyen kaçış (`"\d"`, `"C:\veri"`) sözdizimi hatasıdır; düzenli ifade ve Windows yolu için **ham metin** yazın: `` `C:\veri\tokenizer.json` ``, `` `\d{4}` `` (ters tırnakta kaçış ve yerleştirme yoktur, çok satırlı olabilir). `"C:\tokenizer.json"` gibi geçerli bir kaçış (`\t` sekmedir) yola karışırsa yol alan yerleşikler dosyaya bakmadan hata verir ve ham metni önerir.
- **Sayılar:** `42`, `1_000_000`, `3.14` ve bilimsel yazım `1e-5`, `3.2E+4`, `2e10` (bilimsel yazım her zaman `TR_Float` verir; `TR_eps: 1e-5` gibi küçük değerler için kullanışlıdır).

**Her işin tek yazımı vardır.** Bir yazım dilinde dilin her kelimesinin (anahtar kelime, yerleşik fonksiyon, metot ve özellik, adlı argüman, tip adı) tek adı vardır; bu kılavuz Türkçe yazımı anlatır (İngilizce yazım: `var`, `print`, `func`; [dil tanımı 17.7](../spec/17_kodlar.md)). Türkçe yazımda kelimeler yalnızca Türkçe harfleriyle yazıldıkları biçimde geçerlidir. `degisken`, `sonuc`, `dogru`, `yazdir`, `dogrula`, `esit_mi` gibi Türkçe harfsiz yazımlar dilin kelimesi değil, sıradan adlardır; tanımsızken kullanıldıklarında hata mesajı asıl adı önerir:

```text
'degisken' TR'de anahtar kelime değil; 'değişken' yazmak mı istediniz?
'yazdir' TR'de yok; 'yazdır' yazmak mı istediniz?
'ondalik' TR'de bir tip adı değil; 'Ondalık' yazmak mı istediniz?
AdamW 'ogrenme_orani' adında bir parametre almaz. Bunu mu demek istediniz: 'öğrenme_oranı'?
```

Her işin tek adı vardır; eş anlamlı ikinci bir ad sıradan bir addır. Başka dillerden alışılmış adlar (`int`, `str`, `lr`, `aktar ... as`) yazılınca hata mesajı doğrusunu önerir (`'int' TR'de bir tip adı değil; 'Tam' yazmak mı istediniz?`); ipucu hiçbir zaman Türkçe harfsiz bir biçim önermez (`yazdir` → `TR_print`). Ayrıntı `docs/spec/02_sozcuk_yapisi.md` 2.9a'dadır.

## Yerleşik ad gölgeleme uyarısı

Bir yerleşik fonksiyonun adını (`TR_sum`, `TR_input`, `TR_String`, `TR_Type` ...) değişken, fonksiyon, parametre, döngü değişkeni, `TR_catch` değişkeni ya da yapı adı olarak kullanmak serbesttir, ama o kapsamda yerleşik artık kullanılamaz. Denetleyici bunu bir **uyarı** olarak bildirir: program yine çalışır, çıkış kodu değişmez. Uyarılar standart hataya, tip hatalarından ayrı bir başlıkla yazılır:

```text
⚠️ Uyarılar:
  • Satır 1: 'toplam' yerleşik bir ad; bu kapsamda yerleşik 'toplam' artık kullanılamaz
     1 | değişken toplam = 3
```

Gölgelenmiş ad yine de yerleşik gibi çağrılırsa hata nedenini söyler: `Satır 2: 'toplam' bir fonksiyon değil (Tam); 1. satırda bir değişkene verildi, yerleşik 'toplam' bu kapsamda gölgelendi`. Yapı alanları ve metotları (`TR_self.toplam`, `kutu.toplam()`) gölgeleme sayılmaz; metot adları (`.TR_length`, `.TR_sort`, `.TR_append` ...) yerleşik fonksiyon olmadığından değişken adı olarak serbestçe kullanılır. Uyarıdan kurtulmak için başka bir ad seçin (`TR_var toplam_tutar = 3`).

Dilin kuralları örnekleriyle birlikte [`docs/spec/`](../spec/README.md) klasöründe bölüm bölüm tanımlanmıştır; her kuralın `tests/` klasöründe çalıştırılabilir bir testi vardır (`tr test tests`). Her anahtar kelimenin, yerleşiğin, metodun ve adlı argümanın imzası, açıklaması ve örneği [`docs/codes.md`](../codes.md)'dedir.
