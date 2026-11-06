# Ajan denemeleri

Yabancı bir yapay zekâ ajanı bu depoyu yalnız bir insan istemiyle kullanınca ne oluyor? Bu dosya o denemelerin kaydıdır. Amaç ajanları puanlamak değil, dilin, aracın ve belgelerin eksiğini görmektir.

## Yöntem

- Ajan yeni bir oturumda tek istem alır: [`AGENTS.md`](../AGENTS.md)'nin raw adresi ve insan diliyle bir iş. Ne okuyacağı ve hangi komutu koşacağı söylenmez.
- Sorular cevaptan sonra aynı oturumda sorulur, baştan yazılmaz: ajan onlara göre davranmasın.
- Her bulgunun denemesi, başlığı ve durumu yazılır. Başlıklar: belge, dil, dil tasarımı, fikir, yöntem, olumlu.
- Dil ve dil tasarımı bulgularının sınıfı [`status.md`](status.md)'dekilerdir; belge bulgusu sınıf almaz, hemen düzelir.
- Ajanın ortamı sonucu değiştirir, üçe ayrılır: kod koşamayan sohbet (1, 2), sohbetin kendi kısıtlı makinesi (3, 4, 5, 9, 10), kullanıcının bilgisayarı (7, 8). Aynı ajan her işte koşmayabilir: Grok 4'te koştu, 6'da koşmadı. Kullanıcının bilgisayarındaki deneme Go'suz kurulumu ölçmez.

## Denemeler

| No | Tarih | Ajan | İş | Sonuç |
| :-- | :-- | :-- | :-- | :-- |
| 0 | 2026-11-06 | TCB, yabancı ajan gibi | küçük metin modeli: eğit, kaydet, yükle, üret | koştu, üç takılma |
| 0b | 2026-11-06 | DENETİM, başvurudan | JSON kayıtlarını ayıklayan program | ilk denemede koştu; başvuruda dört yanlış cümle |
| 1 | 2026-11-06 | ChatGPT, ücretsiz | klasördeki metinlerde paragraf arama | ajan koşamadı ve bunu söyledi; TCB olduğu gibi koştu: `check` temiz, ilk denemede çalıştı, sıralama doğru |
| 2 | 2026-11-06 | Kimi instant, ücretsiz | aynı iş | ajan koşamadı ve bunu söyledi; TCB olduğu gibi koştu: `check` temiz, ilk denemede çalıştı; TF-IDF ve kosinüs, komut satırı argümanları, hata yakalama |
| 3 | 2026-11-06 | Claude, ücretli üst model | aynı iş | ajan depoyu klonladı, `trlang` derledi, koştu; TCB de olduğu gibi koştu: temiz, BM25 ve kaba Türkçe kök |
| 4 | 2026-11-06 | Grok, ücretsiz | aynı iş | ajan Go'yu indirdi, derledi, koştu; iki düzeltmeyle çalıştı: dil tanımının yazdığı iki şey motorda yoktu |
| 5 | 2026-11-06 | GLM flash, ücretsiz | aynı iş | ajan derledi, altı senaryo koştu, kendi hatasını Python başvurusuyla buldu; çalıştı |
| 6 | 2026-11-06 | Grok, ücretsiz | Türkçe kök bulan kitaplık, kendi testleriyle | ajan koşmadı ve bunu yazdı; TCB koştu: 11 testin 9'u geçti |
| 7 | 2026-11-06 | Gemini 4 Argon High, Antigravity | aynı iş | ajan kullanıcının bilgisayarında klonladı, `trlang.exe` derledi; ses kuralları, elle 1.848 sözcüklük sözlük ve ek çözümleyicisi yazdı; TCB koştu: 10 testin 10'u geçti |
| 8 | 2026-11-06 | DeepSeek-V4.1-Flash, DeepSeek Harness | aynı iş | ajan kullanıcının bilgisayarında derledi, Zemberek sözlüğünden (Apache-2.0) 26.650 kökü TR verisine çevirdi, kitaplığı TR'de yazdı; ekrana göre 25 dakika, 121 adım; TCB koştu: 15 testin 15'i geçti |
| 9 | 2026-11-06 | Arena AI, ücretsiz flash model | aynı iş | ajan kendi Linux makinesinde derledi; TCB koştu: 18 testin 18'i geçti; bilmediği kökte sözcüğü olduğu gibi veriyor |
| 10 | 2026-11-06 | GLM, ücretsiz flash model | aynı iş | ajan kendi Linux makinesinde derledi; 610 köklük sözlük; TCB koştu: 18 testin 18'i geçti |

## Yarış

Beş programın kendisi, aynı 30 paragraf ve 18 soruyla (2026-11-06). Soruları ve metinleri TCB programları gördükten sonra yazdı, ölçü koşudan önce yazıldı; küçük bir küme, yön gösterir, kesin sıralama değil. Grok'unkine kendi bildirdiği iki düzeltme yapıldı, öbürleri olduğu gibi. Beşi de hatasız koştu [ölçüldü].

| Program | Yöntem | Aynı sözcük (6) | Başka ekli (6) | Başka sözcükle (6) | İlk sıra | İlk üç |
| :-- | :-- | :-- | :-- | :-- | :-- | :-- |
| Claude | BM25, ilk 5 harf kök | 6 | 6 | 1 ilk, 4 ilk üç | 13 | 16 |
| Kimi | TF-IDF, kosinüs | 6 | 3 | 2 | 11 | 11 |
| ChatGPT | eşleşme × nadirlik | 6 | 2 | 2 | 10 | 10 |
| GLM | BM25 benzeri | 6 | 2 | 2 | 10 | 10 |
| Grok | Jaccard | 6 | 2 | 1 ilk, 2 ilk üç | 9 | 10 |

Türkçe aramada yöntemden çok kök fark açıyor: GLM de BM25 kullandı, kök almadığı için geride kaldı. Başka sözcükle sorulan soruda hepsi zayıf; bunun yolu gömme modeli.

Aynı 18 soruda yalnız kök değiştirilince de tuttu (DENETİM 255): Claude kök kapalı 10 ilk, 10 ilk üç, yani GLM'in sayıları; ilk 5 harf kökle GLM ve Kimi 13 ve 16, Grok 12 ve 16, ChatGPT 12 ve 15 [ölçüldü]. Kökün bedeli de var: GLM, Kimi ve Claude'da başka sözcükle sorulan soruda ilk sıra 2'den 1'e düşüyor, ilk üç 2'den 4'e çıkıyor [ölçüldü]. Beş harf bu kümeye bağlı, sabit değil: Claude'da 3 ile 8 arası ilk sıra 11–13, ilk üç 13–16 [ölçüldü].

Claude ücretli üst modeldi, öbür dördü ücretsiz temel model: Claude'un önde olması dilin değil modelin katkısı olabilir; katkı programın gücü değil, kök almayı düşünmek. Hedef için asıl kanıt ücretsiz modellerin de `AGENTS.md`'den başlayıp çalışan TR yazmasıdır: ikisi koşamadan, Grok iki düzeltmeyle, GLM kendi hata ayıklamasıyla.

## Kök yarışı

Beş programın kendisi, iki liste, 120 sözcük, tam eşleşme (2026-11-06). İlk liste programlar koşulmadan sabitlendi; ikincisi programlar görüldükten sonra, nadir sözcüğü ve aşırı uyumu sınamak için yazıldı. Listeler depoda değil. Küçük bir küme, yön gösterir.

| Program | Sözlük | Liste 1 (60) | Liste 2 (60) | Toplam (120) |
| :-- | :-- | :-- | :-- | :-- |
| Gemini | elle 1.848 sözcük, kural tabloları | 59 | 53 | 112 |
| DeepSeek | Zemberek'ten 26.650 kök | 52 | 54 | 106 |
| Arena | küçük, genişletilebilir | 49 | 25 | 74 |
| GLM | 610 kök | 47 | 24 | 71 |
| Grok | yalnız ünlü düşmesi listesi, koşmadan yazıldı | 21 | 22 | 43 |

Sayılar [ölçüldü]. Büyük sözlük nadir köklerde kazandırıyor: ikinci listenin 30 nadir ad ve ünlü düşmesi sözcüğünde DeepSeek 30, Gemini 24, GLM 5, Arena 3. Ek ve ses kuralları fiilde kazandırıyor: iki listenin 30 fiilinde Gemini 29, DeepSeek 22, Arena 22. Yanlış türü de önemli: Arena bilmediği kökü uydurmuyor, sözcüğü geri veriyor; arama işinde bu daha güvenli. Birleştirme denendi: Gemini'nin kuralları ve Zemberek kökleri 59 ve 55, toplam 114; bu sayı iki liste görüldükten sonra kuruldu, gerileme tabanıdır, başarı ölçüsü değil. `tr/turkish` işi bu birleşimi alıyor.

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
| 3 | dil | `docs/spec/13_yerlesikler.md` 13.2'deki `.TR_lines()` ve `.TR_index_of()` motorda yok; onaylı adlar böyle bekliyor, ajan dil tanımından var sanıyor; Grok da çarptı | E, Ö3 | iki ad yapıldı; 50 ad status'ta |
| 3 | belge | kurulum beş deneme sürdü: `go.mod`'un `toolchain` satırı Go indirmeye kalkıyor, `AGENTS.md` bunu söylemiyordu | yok | düzeldi: hazır ikili `v0.1.0` |
| 3 | belge | başvuru "dilin kuralı" diyordu, dil tanımıyla metin metotlarında ayrışıyordu | yok | düzeldi |
| 3 | dil tasarımı | harf, rakam, boşluk soran yerleşik yok; dolaylı yol büyük harfi olmayan yazılarda yanlış | yok | pending'de |
| 3 | belge | örnek yorumlarında eski görünüm adları (`klasörü_listele`, `yol()`, `komut_satırı()`, `aktar`); kod temiz, yorum ajanı yanıltabilir | yok | açık |
| 3 | belge | örnek başlıkları `trlang dosya`, `AGENTS.md` `trlang run dosya` diyor; ikisi de çalışıyor | yok | açık |
| 4, 5 | dil | dil tanımı 19.2 `TR_command_line` yapı alır diyor, motor sözlük istiyor; `check` yakalamıyor, ileti doğru yazımı göstermiyor | E, Ö3 | status'ta; `AGENTS.md` uyarıyor |
| 4 | belge | kurulumda beş engel: eski Go, toolchain, vekil 502, `noexec` klasör, modül indirme | yok | düzeldi: hazır ikili `v0.1.0` |
| 4 | fikir | uygulanan yerleşikler listesi; metin benzerliği yerleşiği | yok | açık |
| 5 | dil tasarımı | yakalanan hatanın metni kullanıcıya `Message("H2132")` diye görünüyor; yazılı kural (8.3), program okunur ileti yazamıyor | yok | pending'de |
| 5 | belge | `.TR_split(" ")` satır sonunda bölmez; ajan bunu bir hata ayıklama turunda buldu, başvuru bağımsız `.TR_split()`'i söylemiyordu | yok | düzeldi |
| 5 | belge | ajan logaritma ve metin dilimi yok sandı; ikisi de var, tek sayfa başvuruda yoktu; `**`'ın üst klasörü kapsadığı da yazmıyordu | yok | düzeldi |
| 5 | belge | başvuru `TR_String(e)` için "hatanın metnidir" diyordu, çalışma hatasında kodu veriyor | yok | düzeldi |
| 5 | dil | denetim bazı yerleşiklerin argüman tipini sınamıyor; yanlış argüman çalışma anında düşüyor | D, Ö3 | 4a5 ile |
| 5 | yöntem | ajan "ad kuralını araç denetlemiyor" dedi; denetliyor, uyarı veriyor: ajan cevabı ölçülmeden yazılmaz | yok | |
| 5 | olumlu | ajan sonucu Python ile bağımsız doğruladı, puanlar aynı çıktı; TR'nin hatası çıkmadı | yok | |
| 3 | olumlu | "tahmin etmeden önce örneklerde ara" cümlesi işe yaradı: ajan adları örneklerden buldu | yok | |
| 1 | belge | ajan bir kimlik sayfası önerdi: imza, tipler, örnek, sık yanlış, uygulama durumu tek yerde, üretilmiş | yok | açık |
| 1 | fikir | metin işleme ve arama modülü: bölme, kök, TF-IDF, kosinüs, vektör indeksi | yok | açık |
| 1 | fikir | aynı işi Python, Go ve TR ile yazdırıp hata, düzeltme ve token ölçmek | yok | açık |
| 1 | yöntem | ajanın "ne okudum" cevabı kendi içinde tutarsız, kanıt sayılmaz; hesabın belleği de cevaba girdi | yok | sonraki denemede belleksiz oturum |
| 7, 8 | belge | `AGENTS.md` ve başvuru `trlang test` ile `trlang format`'ı söylemiyordu; istem test isterken ajan komutu kılavuzdan buldu | yok | düzeldi |
| 7 | dil | biçim denetimi yalnız "biçimlenmeli" diyor; ajanın denemesinde sebep büyük ihtimalle satır sonuydu, çok öğeli satır değil | F, Ö3 | status'ta |
| 7 | fikir | ajan Türkçe küçük harfi kendi yazdı: `.TR_lower()` dile bağlı değil, `"ISI"` `isi` oluyor [ölçüldü]; metin işleme modülüne girer | yok | açık |
| 7 | olumlu | büyük sözcük listesini çok satırlı ham metinle yazıp kümeye çevirdi; biçimleyiciyle çarpışmadı | yok | |
| 7 | olumlu | ajan 2.700 satırı tip hatası almadan yazdı; aldığı derleyici hataları sondaki virgül ve benzer addı [ajan sözü] | yok | |
| 7 | dil tasarımı | sondaki virgül çağrıda geçerli, parametre listesinde ve demette hata [ölçüldü]; ileti virgülü değil sonraki sözcüğü gösteriyor | yok | pending'de |
| 7 | dil tasarımı | 2.1a `ı` ile `i`'yi aynı sayıyor: `kır` ve `kir` birlikte yazılamıyor [ölçüldü] | yok | pending'de |
| 7 | dil tasarımı | `.TR_strip(karakterler)`, kod noktası yerleşikleri ve küme tipi yok [ölçüldü] | yok | pending'de |
| 7 | dil | yaygın yardım bayrağı çıkış kodu 2 veriyor [ölçüldü] | F, Ö3 | status'ta |
| 7 | belge | başvuruda modül yoktu: `TR_import`, `_` ile özel ad, tembel tablo kalıbı | yok | düzeldi |
| 7 | yöntem | ajan "biçimleyici her anahtarı ayrı satıra açmak istedi" dedi; aynı kalıp LF ile temiz geçti, biçimleyici satırı değiştirmedi [ölçüldü]: ajan sözü ölçülmeden yazılmaz | yok | |
| 8 | belge | `lib/README.md` kod adıyla yazıyor, ajan `TR_kw_import` yazdı; ileti doğrusunu söyledi | yok | düzeldi: giriş cümlesi, önek notu |
| 8 | belge | sık yanlışlar tablosunda `and`, `or`, `not` yoktu | yok | düzeldi |
| 8 | belge | başvuruda test yerleşikleri yoktu; `TR_assert_equal`'ın mesaj aldığı yazmıyordu | yok | düzeldi |
| 8 | dil | yalın `and`, `or`, `not` sözdizimi hatası doğru yazımı söylemiyor; ajan 14 hata aldı | F1, Ö3 | status'ta |
| 8 | fikir | modülün en dışı kod koşturmuyor (11.1) ve küme tipi yok; ajan 26.650 kök için sıralı liste ve ikili arama yazdı; ileti doğru yolu gösteriyor | yok | açık |
| 8 | yöntem | ajanın anlatımına göre algoritma önce Python'da 30'dan fazla kez düzeltilmiş, sonra TR'ye taşınmış; üretici silindi, doğrulanmadı | yok | |
| 8 | belge | ajan "modül kod koşturmaz" kuralından veri hazırlamayı TR'de yapılamaz sandı; program dosyası koşar. Kabuk ve hata ayıklayıcıyı denemedi, `AGENTS.md` söylemiyordu | yok | düzeldi |
| 8 | dil tasarımı | ajanın sözü: deneme hızı için Python'u seçti, her denemede tip yazmak yavaşlatacaktı; taşırken üç hata çıktı. Ölçülmedi: dilin deneme hızı bir soru olarak açık | yok | açık |
| 8 | belge | ajan Windows'ta hazır ikiliyi indiremedi, kaynaktan derledi; ajanın ortamından olabilir | yok | ölçülmedi |
| 8 | olumlu | çok dosyalı kitaplık, `TR_import`, yapı, durum makinesi, ikili arama: 30 bin satırlık veri dosyasıyla kendi testleri yaklaşık 1 saniye [ölçüldü] | yok | |
