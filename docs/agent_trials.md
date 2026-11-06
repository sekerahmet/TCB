# Ajan denemeleri

Yabancı bir yapay zekâ ajanı bu depoyu yalnız bir insan istemiyle kullanınca ne oluyor? Bu dosya o denemelerin kaydıdır. Amaç ajanları puanlamak değil, dilin, aracın ve belgelerin eksiğini görmektir.

## Yöntem

- Ajan yeni bir oturumda tek istem alır: [`AGENTS.md`](../AGENTS.md)'nin raw adresi ve insan diliyle bir iş. Ne okuyacağı ve hangi komutu koşacağı söylenmez.
- Sorular cevaptan sonra aynı oturumda sorulur, baştan yazılmaz: ajan onlara göre davranmasın.
- Her bulgunun denemesi, başlığı ve durumu yazılır. Başlıklar: belge, dil, dil tasarımı, fikir, yöntem, olumlu.
- Dil ve dil tasarımı bulgularının sınıfı [`status.md`](status.md)'dekilerdir; belge bulgusu sınıf almaz, hemen düzelir.

## Denemeler

| No | Tarih | Ajan | İş | Sonuç |
| :-- | :-- | :-- | :-- | :-- |
| 0 | 2026-11-06 | TCB, yabancı ajan gibi | küçük metin modeli: eğit, kaydet, yükle, üret | koştu, üç takılma |
| 0b | 2026-11-06 | DENETİM, başvurudan | JSON kayıtlarını ayıklayan program | ilk denemede koştu; başvuruda dört yanlış cümle |
| 1 | 2026-11-06 | ChatGPT, ücretsiz plan | klasördeki metinlerde paragraf arama | ajan koşamadı ve bunu söyledi; TCB olduğu gibi koştu: `check` temiz, ilk denemede çalıştı, sıralama doğru |
| 2 | 2026-11-06 | Kimi, instant | aynı iş | ajan koşamadı ve bunu söyledi; TCB olduğu gibi koştu: `check` temiz, ilk denemede çalıştı; TF-IDF ve kosinüs, komut satırı argümanları, hata yakalama |
| 3 | 2026-11-06 | Claude | aynı iş | ajan depoyu klonladı, `trlang` derledi, koştu; TCB de olduğu gibi koştu: temiz, BM25 ve kaba Türkçe kök |
| 4 | 2026-11-06 | Grok | aynı iş | ajan Go'yu indirdi, derledi, koştu; iki düzeltmeyle çalıştı: dil tanımının yazdığı iki şey motorda yoktu |
| 5 | 2026-11-06 | GLM flash | aynı iş | ajan derledi, altı senaryo koştu, kendi hatasını Python başvurusuyla buldu; çalıştı |

## Bulgular

| Deneme | Başlık | Bulgu | Sınıf | Durum |
| :-- | :-- | :-- | :-- | :-- |
| 0 | belge | `lib/README.md`'nin ilk örneği koşmuyordu: hedef liste veriliyordu, tensör gerekir | yok | düzeldi |
| 0 | dil | kitaplığın kayıp metodu parametre tipi yazmıyor, hatalı hedefi denetim değil çalışma anı yakalıyor | D, Ö3 | status'ta |
| 0 | dil tasarımı | ajan modeli kaydetmek için yükleme adının eşini tahmin ediyor; ad yok, ileti ters işi öneriyor | F, Ö3 | status'ta |
| 0 | olumlu | Python alışkanlığı `"".TR_join(l)` hata verdi, ileti doğrusunu söyledi | yok | |
| 0b | belge | eski başvuruda dört cümle motorda yanlıştı: JSON türlerinin yazımı, tip parametresi, tanım sırası, atama uyarısı | yok | düzeldi |
| 0b | belge | öğeye atama başvuruda yoktu | yok | düzeldi |
| 0b | dil | bir listedeki demete `l[0][0]` denetimde değil çalışma anında hata veriyor | D, Ö3 | status'ta |
| 1 | olumlu | koşamayan ajan yalnız okuyarak hatasız program yazdı; başvuruda olmayan dosya ve girdi adlarını da doğru buldu | yok | hangi dosyaları okuduğu sorulacak |
| 1, 2 | olumlu | ajan koşmadığını açıkça söyledi, çıktı uydurmadı | yok | |
| 1 | belge | tek sayfa başvuruda dosya ve klasör işleri yok; ajan 18. bölümü aramak zorunda kaldı | yok | düzeldi |
| 3 | dil | `docs/spec/13_yerlesikler.md` 13.2'deki `.TR_lines()` ve `.TR_index_of()` motorda yok; 52 onaylı ad böyle bekliyor, ajan dil tanımından var sanıyor; Grok da çarptı | E, Ö3 | status'ta |
| 3 | belge | kurulum beş deneme sürdü: `go.mod`'un `toolchain` satırı Go indirmeye kalkıyor, `AGENTS.md` bunu söylemiyordu | yok | düzeldi; hazır ikili kararı Ahmet'te |
| 3 | belge | başvuru "dilin kuralı" diyordu, dil tanımıyla metin metotlarında ayrışıyordu | yok | düzeldi |
| 3 | dil tasarımı | harf, rakam, boşluk soran yerleşik yok; dolaylı yol büyük harfi olmayan yazılarda yanlış | yok | pending'de |
| 3 | belge | örnek yorumlarında eski görünüm adları (`klasörü_listele`, `yol()`, `komut_satırı()`, `aktar`); kod temiz, yorum ajanı yanıltabilir | yok | açık |
| 3 | belge | örnek başlıkları `trlang dosya`, `AGENTS.md` `trlang run dosya` diyor; ikisi de çalışıyor | yok | açık |
| 4, 5 | dil | dil tanımı 19.2 `TR_command_line` yapı alır diyor, motor sözlük istiyor; `check` yakalamıyor, ileti doğru yazımı göstermiyor | E, Ö3 | status'ta; `AGENTS.md` uyarıyor |
| 4 | belge | kurulumda beş engel: eski Go, toolchain, vekil 502, `noexec` klasör, modül indirme | yok | `AGENTS.md`'de; hazır ikili kararı Ahmet'te |
| 4 | fikir | uygulanan yerleşikler listesi; metin benzerliği yerleşiği | yok | açık |
| 5 | dil tasarımı | yakalanan hatanın metni kullanıcıya `Message("H2132")` diye görünüyor; yazılı kural (8.3), program okunur ileti yazamıyor | yok | pending'de |
| 5 | belge | `.TR_split(" ")` satır sonunda bölmez; ajan bunu bir hata ayıklama turunda buldu, başvuru bağımsız `.TR_split()`'i söylemiyordu | yok | düzeldi |
| 5 | belge | ajan logaritma ve metin dilimi yok sandı; ikisi de var, tek sayfa başvuruda yoktu; `**`'ın üst klasörü kapsadığı da yazmıyordu | yok | düzeldi |
| 5 | belge | başvuru `TR_String(e)` için "hatanın metnidir" diyordu, çalışma hatasında kodu veriyor | yok | düzeldi |
| 5 | dil | denetim yerleşiklerin argüman tipini sınamıyor; yanlış argüman çalışma anında düşüyor | A, Ö1 | status'ta, 4a5 |
| 5 | yöntem | ajan "ad kuralını araç denetlemiyor" dedi; denetliyor, uyarı veriyor: ajan cevabı ölçülmeden yazılmaz | yok | |
| 5 | olumlu | ajan sonucu Python ile bağımsız doğruladı, puanlar aynı çıktı; TR'nin hatası çıkmadı | yok | |
| 3 | olumlu | "tahmin etmeden önce örneklerde ara" cümlesi işe yaradı: ajan adları örneklerden buldu | yok | |
| 1 | belge | ajan bir kimlik sayfası önerdi: imza, tipler, örnek, sık yanlış, uygulama durumu tek yerde, üretilmiş | yok | açık |
| 1 | fikir | metin işleme ve arama modülü: bölme, kök, TF-IDF, kosinüs, vektör indeksi | yok | açık |
| 1 | fikir | aynı işi Python, Go ve TR ile yazdırıp hata, düzeltme ve token ölçmek | yok | açık |
| 1 | yöntem | ajanın "ne okudum" cevabı kendi içinde tutarsız, kanıt sayılmaz; hesabın belleği de cevaba girdi | yok | sonraki denemede belleksiz oturum |
