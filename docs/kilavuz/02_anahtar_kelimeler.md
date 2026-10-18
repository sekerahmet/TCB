# 2. Anahtar Kelimeler (Tek Yazım: Yalnızca Türkçe)

- **Değişken & Sabit:** `değişken`, `:=`, `sabit`
- **Çoklu Atama & Takas:** `değişken a, b = 10, 20`, `a, b = b, a`, `ad, soyad = ["Mustafa", "Kemal"]`
- **Fonksiyon & Dönüş:** `fonksiyon`, `sonuç`, `(x) => x * 2`
- **Koşul & Eşleştirme:** `eğer`, `değilse eğer`, `değilse`, `eşle`, `varsayılan`
- **Döngüler & Liste Üreteci:** `döngü x içinde liste`, `1..10`, `1..=10`, `iken`, `çık`, `devam`, `[x * x döngü x içinde 1..=10 eğer x % 2 == 0]`
- **Mantıksal & Boş:** `doğru`, `yanlış`, `yok`, `ve` (`&&`), `veya` (`||`), `değil` (`!`), `??`, `?.`
- **Tipler:** `Tam`, `Ondalık`, `Metin`, `Mantık`, `Yok`, `Liste`, `Sözlük`, `Tensör` (matrisler 2 boyutlu tensördür). Tip açıklamasında büyük/küçük harf fark etmez (`MANTIK` ≡ `Mantık`), ama eş ya da İngilizce adlar (`tamsayı`, `int`, `harita`, `dict` ...) kabul edilmez: `'tamsayı' TR'de bir tip adı değil; 'Tam' yazmak mı istediniz?`
- **Yapı:** `yapı`, `kendi` (kalıtım yoktur)
- **Hata, Ertele, Kesme Noktası, Modül & Eşzamanlılık:** `dene`, `yakala`, `fırlat`, `ertele`, `kesme_noktası` (koşullu: `kesme_noktası eğer koşul`), `aktar "yol"`, `aktar "yol" olarak ad`, `eşzamanlı`, `bekle(saniye)`, `bekle_hepsi()`
- **Test:** `test "ad" { ... }`, `doğrula(koşul, [mesaj])`, `eşit_mi(beklenen, gerçek, [mesaj])`
- **Yorumlar:** `// satır sonuna kadar` ve `/* çok satırlı */`. `#` yorum değildir; sözdizimi hatası verir: `'#' TR'de yorum değil; yorum için '//' yazın`.
- **Sayılar:** `42`, `1_000_000`, `3.14` ve bilimsel yazım `1e-5`, `3.2E+4`, `2e10` (bilimsel yazım her zaman `Ondalık` verir; `epsilon: 1e-5` gibi küçük değerler için kullanışlıdır).

**Her işin tek yazımı vardır.** Dilin kelimeleri (anahtar kelimeler, yerleşik fonksiyonlar, metot ve özellikler, adlı argümanlar, tip adları) yalnızca Türkçe harfleriyle yazıldıkları biçimde geçerlidir. `degisken`, `sonuc`, `dogru`, `yazdir`, `dogrula`, `esit_mi` gibi Türkçe harfsiz yazımlar dilin kelimesi değil, sıradan adlardır; tanımsızken kullanıldıklarında hata mesajı asıl adı önerir:

```text
'degisken' TR'de anahtar kelime değil; 'değişken' yazmak mı istediniz?
'yazdir' TR'de yok; 'yazdır' yazmak mı istediniz?
'ondalik' TR'de bir tip adı değil; 'Ondalık' yazmak mı istediniz?
AdamW 'ogrenme_orani' adında bir parametre almaz. Bunu mu demek istediniz: 'öğrenme_oranı'?
```

Eş anlamlı ikinci adlar da kaldırıldı ve artık sıradan adlardır: `işlev`, `döndür`, `kır`, `sınıf`, `aktar ... as` yerine `fonksiyon`, `sonuç`, `çık`, `yapı`, `olarak`; `matris(...)` yerine `tensör([[...]])`, `öğrenme_hızı:` yerine `öğrenme_oranı:` yazın. Eski adla yazılan kodda hata mesajı doğrusunu önerir (`'işlev' TR'de anahtar kelime değil; 'fonksiyon' yazmak mı istediniz?`); ipucu hiçbir zaman Türkçe harfsiz bir biçim önermez (`islev` → `fonksiyon`). Kaldırılanların tam listesi `docs/dil/02_sozcuk_yapisi.md` 2.9a'dadır.

## Yerleşik ad gölgeleme uyarısı

Bir yerleşik fonksiyonun adını (`toplam`, `girdi`, `metin`, `tip` ...) değişken, fonksiyon, parametre, döngü değişkeni, `yakala` değişkeni ya da yapı adı olarak kullanmak serbesttir, ama o kapsamda yerleşik artık kullanılamaz. Denetleyici bunu bir **uyarı** olarak bildirir: program yine çalışır, çıkış kodu değişmez. Uyarılar standart hataya, tip hatalarından ayrı bir başlıkla yazılır:

```text
⚠️ Uyarılar:
  • Satır 1: 'toplam' yerleşik bir ad; bu kapsamda yerleşik 'toplam' artık kullanılamaz
     1 | değişken toplam = 3
```

Gölgelenmiş ad yine de yerleşik gibi çağrılırsa hata nedenini söyler: `Satır 2: 'toplam' bir fonksiyon değil (Tam); 1. satırda bir değişkene verildi, yerleşik 'toplam' bu kapsamda gölgelendi`. Yapı alanları ve metotları (`kendi.toplam`, `kutu.toplam()`) gölgeleme sayılmaz; metot adları (`uzunluk`, `sırala`, `ekle` ...) yerleşik fonksiyon olmadığından değişken adı olarak serbestçe kullanılır. Uyarıdan kurtulmak için başka bir ad seçin (`değişken toplam_tutar = 3`).

Dilin kuralları örnekleriyle birlikte [`docs/dil/`](../dil/README.md) klasöründe bölüm bölüm tanımlanmıştır; her kuralın `testler/` klasöründe çalıştırılabilir bir testi vardır (`tr test testler`). Her anahtar kelimenin, yerleşiğin, metodun ve adlı argümanın imzası, açıklaması ve örneği [`docs/kodlar.md`](../kodlar.md)'dedir.
