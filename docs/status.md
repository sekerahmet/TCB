# Durum

Dil tanımının yeniden yazılan bölümlerine göre bugünkü TR'nin durumu ve kurala varmak için yapılacaklar. Her maddenin kuralı dil tanımındadır ([`docs/spec/`](spec/README.md)); yapılan iş testiyle birlikte buradan silinir. İşlerin sırası [yol haritasındadır](roadmap.md). Bu belgede kimlikler dil tanımının yazımıyla anılır (`TR_if`, 2.3a).

## Önem ve öncelik

Sınıflar ve öncelikler TCB'nin taslağıyla DENETİM'in hükmünden (istek 61, 2026-11-03). Etiket her maddenin "Yapılacaklar"ında: sınıf, öncelik, kol ya da dilim ve ölçüm tabanı, ör. **Yapılacaklar (A, Ö1, 4a4; ölçüldü 30b44b01)**. Etiketsiz madde D, E ya da F sınıfındadır ve Ö3'tür. Etiket ölçüm tabanını taşır: taban eskidikçe iddia yeniden ölçülür.

**Sınıflar** (sıra tasarım ölçütlerinin sırası: doğruluk, güvenlik, ajan için hatasız):
- **A, sessiz yanlış:** program hata vermeden yanlış sonuç, yanlış veri ya da yanlış gradyan veriyor; diskteki veriyi sessizce kaybediyor ya da bozuyor; ya da bir test bu yanlışı doğru diye sabitliyor.
- **B, güvenlik:** dış girdi (kullanıcı metni, dosya, yol) programın davranışını değiştirebiliyor.
- **C, güvenilirlik:** aynı program aynı veriyle farklı sonuç veriyor (makineye, iş parçacığı sayısına, test sırasına göre); kesilen iş aynı sürmüyor; program asılı kalıyor.
- **D, geç hata:** yanlış program denetimden geçiyor; çalışırken açık hata veriyor ya da hatasız çalışıp doğru sonuç veriyor (A için çıktı dil tanımına göre yanlış olmalı).
- **F1, yanlış öğreten ileti:** iletinin önerisi kopyalanınca yanlış çalışıyor ya da yanlış kuralı öğretiyor (ör. `d[1]`'de `.TR_get(1) ?? varsayılan`). D'nin yanında.
- **E, eksik özellik:** kural var, motorda yok, açık hatayla.
- **F, okunurluk:** ileti doğru ama eksik ya da belirsiz.

**Öncelikler:**
- **Ö0:** A ya da B, hiçbir dilime bağlı değil. Uygun kolun bir sonraki dilimi olur, planın sırasının önüne geçer. Süren dilime yalnız aynı kod yolundaki küçük madde girer.
- **Ö1:** A ya da B, bir dilime ya da altyapıya bağlı. O dilimin kesin listesine girer: dilim bunlarsız kapanmaz.
- **Ö2:** C; 2. kilometre taşının ve Jarvis'in ön koşulları.
- **Ö3:** D, F1, E, F; planın sırasıyla.

**Ö0, sırayla:**
1. 16.2 ve 16.3 (parçalayıcı; Jarvis'in ön koşulu): ilk boşalan kolun sonraki dilimi.
2. 13.1: `TR_std` n−1, `TR_write_file`'ın baytı, `TR_read_file`'ın geçersiz UTF-8 hatası: Kol A, 4a4'ten sonra ayrı küçük dilim, 4a5'ten önce. FMA'sız yol (C) bu dilime girmez. Hemen ardından aynı kolda `TR_max` ve `TR_round`'un dört sessiz yanlışı (status 13.1, DENETİM istek 71).
3. 15.34, 15.23a, 15.24, 15.44: Kol B, S-kilitten sonra, 2b'den önce.

**Ö1, dilimlerin kesin listeleri:** 4a4 (birleşti 2026-11-03, `ab95122f`; kalanları 4a5 ve D/F etiketli): 3.6, 3.7, 4.5, 4.10, 3.6a. 4a5: 11.8, 13.8, 6.9 ve 7.7 (ya da 4c), 1.2, 3.13, 5.5. 4b: 3.5, 6.2, 5.12, 5.6, 7.4. 4c: 3.14. Kol A, 13.1'in dilimi: 8.5'in işletim sistemi hatası. Kol B: 20.11, 15.27a, 15.33, 15.45.

**Gemini'nin bağımsız taraması** (DENETİM istek 63–67; taban `30b44b01`): teslim 1 (bölüm 1–5) DENETİM'den geçti, üç yeni A (1.2, 3.13, 5.5) [ölçüldü: DENETİM istek 67]; teslim 2–6 sürüyor.

**Sınır:** tarama "Yapılacaklar"ın hepsini, "Mevcut durum"dan risk sözcüğü geçen cümleleri okudu; sözcüksüz anlatılan bir sessiz yanlış kaçmış olabilir. 16.6 ve 18.9'un (kesilen yazma) sessiz olup olmadığı ölçülmedi.

## Dil tanımının bekleyen bulguları

Uygulamada verilen ve dil tanımı turunda denetlenecek kararlar, dil tanımıyla çelişen bulgular: [`docs/spec/pending.md`](spec/pending.md).

## 1. Program

Kurallar: [`docs/spec/01_program.md`](spec/01_program.md). Kararlar DENETİM'den ve Gemini'nin araştırmasından geçti (2026-10-27). 1.3–1.15 uygulandı (katman 1b, DENETİM'den geçti, 2026-10-28); aşağıda yalnız kalanlar.

### 1.1

- **Mevcut durum:** denetleyici modülde ve test dosyasında (`TR_test` içeren dosya, 12.1) en dıştaki tanım dışı deyimi (H1107) ve `TR_const`/`TR_var`'ın sabit olmayan ilk değerini (H1108) denetim hatası sayıyor; hata modülün satırında, aktaran modül hatasının başlığında; ileti deyimi fonksiyona alan yazımı veriyor ve yazım çalışıyor; fonksiyonların atadığı en dıştaki `TR_var` paylaşılan durum sayılıyor (H1114), ileti onu `TR_var x: T? = TR_none` ile başlatıp bir kez kuran fonksiyonu öneriyor (tip bilinmiyorsa `<tip>`). Depodaki modüller ve test dosyaları kurala uyuyor. Dosyanın tanımlamadığı ad (aktarılan ya da genel) sabit sayılıyor; gömülü kitaplık modülleri aktarılırken denetlenmiyor, kuralları depo testinde denetleniyor [ölçüldü: `TestRepoModulesHoldOnlyDefinitions`, `TestModuleRuleSuggestionRuns`, `TestModuleSharedStateSuggestionRuns`, dal `claude/charming-pasteur-q1vsvo`].
- **Yapılacaklar:**
  - Aktarılan adın sabitliği modüller arası tip denetimiyle (4b) bilinir; bugün aktarılan ad sabit sayılıyor.

### 1.2

- **Mevcut durum:** denetleyici yerel değişkenin tipini ilk değerinden çıkarıyor ve tipi kalıcı (3.13): değişmezler, aritmetik ve önek işleç, karşılaştırma, adıyla çağrılan tipli fonksiyonun ve tipi bilinen yapının metodunun dönüşü, yapı kurucusu, liste ve sözlüğün öğe tipi, demet, isteğe bağlı tip (`T?`, `??`'nin asıl tipi), tipi bilinen yapının alanı ve metotta `TR_self`, değer veren `TR_if`/`TR_match` (dalların ortak tipi), liste/metin/sözlük/tensör indeksi ve dilimi, döngü değişkeni (5.7), yerleşik çağrısının tek tipli dönüşü (4a2: tanım dosyasının `returns`'ü; birleşim ve motorla doğrulanmamış dönüş `TR_Any`), tipi bilinen çekirdek alıcıda (metin, liste, sözlük, aralık) özelliğin değeri ve metodun alıcıdaki dönüşü öğe tipiyle (4a2 ikinci dilim: `[1, 2].TR_length` `TR_Int`, `.TR_first` `T?`, `{"a": 1}.TR_keys()` `TR_List[TR_String]`; 28 üye-alıcı dönüşü motorla doğrulandı [test: `TestMemberValueTypes`, `TestMemberReturnsMatchEngine`]); bilinmeyen `TR_Any` kalıyor. Depodaki programların 1893 ilk değerinden 622'sinin tipi bilinmiyor (4a1'den sonra 1115, ilk dilimden önce 1212) [ölçüldü: `TestRepoFirstValueTypes`, `TestBuiltinReturnTypes`, `TestSingleReturnsMatchEngine`]; tipsiz parametre, alan ve `->`'sız değerli dönüş denetim hatası (6.1, 7.1; kitaplığın 4c'yi bekleyen yerleri istisna), parametre tipleri yalnız adıyla çağrılan fonksiyonun konumlu argümanlarında denetleniyor [ölçüldü: `TestRepoFirstValueTypes`, `TestOptionalTypes`, `TestFieldTypes`, `TestIfAndMatchValueTypes`, `TestIndexTypes`, `TestLoopVariableTypes`, dal `claude/charming-pasteur-q1vsvo`].
- **Yapılacaklar (A, Ö1, 4a5; ölçüldü 30b44b01):** turlarla:
  - çıkarımın kalanı (4a2'nin ikinci dilimi): kitaplık çağrılarının ve birleşim dönüşlü yerleşiklerin dönüşü (bilinmeyen çağrı 461), imzaların öğe tipleri, tipi bilinmeyen işlenenli işleç, kapanış, çoklu tanımın (`TR_var a, b = f()`) öğe tipleri, liste üretecinin sonucu, metot ve kurucu argümanlarının denetimi, kitaplık tiplerinin tanınması;
  - `TR_none` daraltması ve isteğe bağlı değerin açılmamış kullanımı (3.5, 4b);
  - tensörün tür ve şekil bilgisinin tipe ne kadar gireceği;
  - `TR_import ... TR_as m` modülü bugün `TR_Dict`'e bağlıyor (11.2), üyelerin tipi korunmalı; gerçek sürücü ile benzetim arasında seçim için ortak bir arayüz tipi.

### 1.3

- **Mevcut durum:** takma adsız `TR_import` olan kapsamda tanımsız adlar çalışmadan önce aranmıyor (5.14).
- **Yapılacaklar:** takma adsız aktarmada tanımsız ad çalışmadan önce bulunur.

### 1.5

- **Mevcut durum:** hesap çekirdeğinin (CPU) ortasında bırakma yok, süren hesap biter; ağ yerleşiği yok; TR alt süreç açmıyor (19.1).
- **Yapılacaklar:**
  - Bırakılan hesabın GPU'da başlamış işi beklenir, ertelenen işler sonra çalışır; CPU çekirdeğinin ortasında bırakma.
  - TR alt süreç açmaya başlarsa (19.1) bu madde ona göre genişler: istek alt sürece, kendisi zaten almadıysa iletilir.

### 1.9

- **Uygulamanın iç çöküşü:** Go çalışma zamanının ölümcül hatası 2 koduyla biter ve 2'nin "kullanım hatası" sözüyle karışır. Yol haritası: motor çöküşü kendi koduyla bitsin.

## 2. Sözcük yapısı

Kurallar: [`docs/spec/02_sozcuk_yapisi.md`](spec/02_sozcuk_yapisi.md). DENETİM'den (üç tur) ve Gemini'nin araştırmasından geçti (2026-10-27). 2.1–2.10 uygulandı (katman 1, iki DENETİM turundan geçti, 2026-10-28); aşağıda yalnız kalanlar.

### 2.1

- **Mevcut durum:**
  - Rehber ve kök belgelerdeki program metni kimlik yazımına çevrildi (2026-10-29). Kalan sınıflı yazım 17.2'nin "içeride" grubudur: tanım dosyalarının içeriği, eş adlar tablosunun kod sütunu, `code_identities.md`, disk biçimi örnekleri [ölçüldü: main `85a19c36`, belgelerde program sınıflarının sınıflı yazımını arayan git grep].
  - Üretilen kod listesi (`docs/codes.md`) başlıklarda program yazımını, yanında sınıflı anahtarı yazıyor; örnekleri kimlik yazımıyla, altında her gömülü dilin görünümüyle (tanım dosyasının örnek alanından, 2026-10-29, Faz 0.5).
  - Satır içi kod (ters tırnak) yalnız dil tanımında ve status'ta denetleniyor; öbür belgelerde yanlış kod gösteren ad (ör. aynı kısa adın başka sınıfı) hiçbir testte düşmüyor.
- **Yapılacaklar:**
  - ters tırnak içindeki adların denetimi `docs/*.md`'ye genişler (üretilen ve donmuş belgeler ile eş adlar tablosunun kod sütunu hariç): program sınıflarının sınıflı yazımı hata, `.TR_x` yalnız üye, `TR_x:` yalnız adlı argüman.

### 2.3a

- **Mevcut durum:** kimlik kapısı (`TestReviewedSectionsWriteIdentities`) bir `TR_` sözcüğünün herhangi bir yerde geçerli olmasını yeterli sayıyor; yanlış yerde yazılmış `TR_append(x)` geçiyor [kanıtlı: DENETİM].
- **Yapılacaklar:** tam ifade olan parça (`(`, `.` ya da `:` içeren) sözcük çözümleyiciyle okunup yeri denetlenir.

### 2.13

- **Mevcut durum:** `TR_cmd_check --json` çalışıyor (katman 2b, DENETİM'den geçti, 2026-10-28): sözcük çözümleyicinin, ayrıştırıcının, denetleyicinin ve komut satırının hataları ve uyarıları başlangıç ve bitiş konumu, en içteki kod, ilgili konum (H1020) ve düzeltmeyle JSON satırları olarak; satır başındaki `//` → `#` ve `0X1F` → `0x1F` uygulanabilir. Metin kipinde denetleyicinin iletileri sütunu da yazıyor [ölçüldü].
- **Yapılacaklar:** başlık hatasının (H2540) bitişi; ana dosyadaki okunamayan modülün (H2132; olmayan klasör H2131) sütunu ve bitişi [kanıtlı: `internal/lang/turkish/file_error.go:131-133`]; iç içe modülün `TR_import` hatasının dosyası (yorumlayıcıda `ModuleError` sarmalayıcısı); `TR_cmd_check`'te dosyadan sonraki öbür argümanların durumu (1.4); aktarılan modülün uyarıları [ölçüldü: DENETİM].

### 2.3b

- **Mevcut durum:** harf büyüklüğü olmayan adlar (`TR_struct 学生`, `TR_struct طالب`) uyarı alıyor ve önerilen düzeltme adın kendisi; `TR_const 最大` uyarısız [ölçüldü: DENETİM]. **Yapılacaklar:** büyüklüğü olmayan karakter her biçime uyar (2.3b); TR testi.

## 3. Tipler ve değerler

Kurallar: [`docs/spec/03_tipler.md`](spec/03_tipler.md). Taslak: metin DENETİM'den geçti, bulguları işlendi (2026-10-27); kararlar Gemini'nin 1–7 araştırmasından geçti (DENETİM doğruladı, `84ee01b8`). Sonra verilen kararlar (yukarıdaki liste) DENETİM'den geçti, Gemini'nin araştırması bekleniyor. TR bugün dinamik tipli; tip açıklamalarını yalnız çalışmadan önceki denetleyici kullanıyor. `tests/tipler_test.tr` altındaki 3. bölüm testlerinden yalnız 3.6 testi (`tests/tipler_test.tr:116-117`) eski kuralı (karışık tipli liste) sınıyor; doğruluk sınaması (3.10), çıplak ad anahtarı (3.7) ve açıklamasız değişkenin tipi (3.13) yeni kurallara geçti [kanıtlı: `tests/tipler_test.tr:116-117, 234-244, 443-461, 550-562`]; karışık tipli liste testi kural uygulanırken yeniden yazılır. 3.2, 3.3a, 3.7, 3.7a, 3.8, 3.12 ve 3.14'ün çalışma anı uygulandı (katman 2, DENETİM'den geçti, 2026-10-28; 3.2, 3.8 ve 3.12 bitti, buradan silindi); aşağıda kalanlar. Eski 3.14 (tip adının tek yazımı) 2.3a'ya düştü; eski 3.15 (katalog iletisinin tipi) hata bölümüne gider, `tests/` altındaki 3.15 testi o bölüm yazılınca taşınır.

### 3.3a

- **Mevcut durum:** `TR_Bytes` kuruluyor, okunuyor, birleşiyor; metni "Bytes([104, 105])" [ölçüldü]. 64 bayttan uzunda "Bytes(length 65, [0, 1, …, 15, …])" ve `TR_String(b)`'nin UTF-8 okuması 3.14'ün tur 1 yazımıyla aynı [test: `tests/tipler_test.tr`, "3.14 bayt dizisinin metni"].
- **Yapılacaklar:** dosya ve ağ okumalarının bayt da vermesi (ölçülmedi).

### 3.5

- **Mevcut durum:** denetleyici isteğe bağlı tipi `T?` olarak taşıyor: `TR_none` yalnız isteğe bağlı (ya da tipi bilinmeyen adlı) değişkene, parametreye, alana ve dönüşe veriliyor, öbürü denetim hatası (`TR_var a: TR_Int = TR_none`, `-> TR_Int` fonksiyonda değersiz `TR_return`); `x ?? d`'nin tipi asıl tip, `[1.0, TR_none]` `TR_List[TR_Float?]`. İsteğe bağlı değerin asıl tipin yerinde kullanılması denetlenmiyor (daraltma yok, aritmetikte tipi `TR_Any`) [ölçüldü: `TestOptionalTypes`, `TestTypeCompatibility`, dal `claude/charming-pasteur-q1vsvo`].
- **Mevcut durum (4a3, 2026-11-01):** ad olmayan bir `T?` ifadesi (`l.TR_first`, `l.TR_get(5)`, `TR_List[T?]`'nin indeksi) `T` beklenen yere verilince denetim hatası, konumun koduyla: tanım H1001, atama H1005, dönüş H1003, argüman H1004, kurucu H1125, alan H1124, yerleşik metodun argümanı H2423, `.TR_append` H1123; liste, sözlük ve demet değişmezinin öğesinde de; öğesi isteğe bağlı olmayan kabın indeksine atama H1135. H1001, H1003, H1005 ve H1135'in ipucu H1130 (`l.TR_first ?? 0`). Ad tabandaki gibi geçer (4b); yerleşik fonksiyonun argümanı (`TR_abs(l.TR_first)`) 4a4'te fonksiyonun çalışma anı koduyla (H2423; 13.11) [test: `TestUnopenedOptional`, `TestUnopenedOptionalHint`, `tests/negative/istege_bagli_acilmadan.tr`, `TestMessageSuggestionsRunCleanly`].
- **Mevcut durum (4a4, 2026-11-03):** H1004, H1124, H1125, H1123 ve H2423 (metot ve yerleşik fonksiyon) da açılmamış değerin ipucunu (H1130) veriyor: `{hint}` yuvası, yazım dosyadan (`l.TR_first ?? 0`) [test: `TestUnopenedOptionalHint`, `TestMessageSuggestionsRunCleanly`].
- **Yapılacaklar (A, Ö1, 4b; ölçüldü 30b44b01):** adın açılmamış kullanımı denetim hatası; daraltma (4b) yalnız değişmeyen yerel değişkende: sınamanın dalında, `TR_else` dalında, `TR_and`/`TR_or`'un sağ yanında, her yolda biten erken çıkıştan sonra ve asıl tipten atamayla; alana atama daraltmayı bozmaz; ilk değeri `TR_none` olan açıklamasız değişkenin hatası (3.13).
### 3.6

- **Mevcut durum:** karışık tipli liste çalışıyor ve denetlenmiyor: `[1, "a"]` hatasız, denetleyici onu öğe tipsiz `TR_List` sayıyor; `l[0] = "x"` öğe tipine karşı denetlenmiyor. 4a2 (ikinci dilim): öğe tipi bilinen listeye `.TR_append` ile başka tipte değer denetim hatası (H1123; çalışma anı öğe tipini taşımadığı için çalışma anında hata yok) [ölçüldü: `TestAppendElementType`, `tests/negative/liste_oge_tipi.tr`]. `TR_List[T]` ayrıştırılıyor ve denetleyici öğe tipini taşıyor: `TR_var l: TR_List[TR_Int] = ["a"]` denetim hatası (H1001), kabın tipi değişmez (`TR_List[TR_Int]` `TR_List[TR_Float]` yerine verilmez), `[1, TR_none]` `TR_List[TR_Int?]`, `[1, 2.5]` `TR_List[TR_Float]` [ölçüldü: main `85a19c36`, `TestCollectionElementTypes`, `TestCollectionLiteralTypes`, CLI].
- **Mevcut durum (D1 dilim 3, 2026-11-02):** motor da öğelerinin hepsi sayı yazımı olan ve birinde ondalık bulunan değişmezde tam sayı yazımlarını ondalık kuruyor: `[1, -2, 2.5]` "[1.0, -2.0, 2.5]". Öğelerden biri değişkense (`[1, x, 2.5]`) tam sayı kalıyor: öğenin tipi çıkarımdır, tip haritasıyla [test: `TestADecimalInAListLiteralMakesItsIntegersDecimal`, `tests/tipler_test.tr`'nin "3.6 değişmezde bir ondalık varsa" testi].
- **Mevcut durum (4a4, 2026-11-03):** öğe tipi bilinen listenin ve sözlüğün indeksine başka tipte değer yazmak denetim hatası (H1136: `m[0] = "s"`, `m[0] += 1.5`, `d["b"] = "s"`); öğe tipleri ayrı iki listenin `+=`'sı H1138 (4.10); `[1, TR_none]` (`TR_List[TR_Int?]`) `TR_List[TR_Int]` beklenen yerde H1001. Tipi bilinmeyen değer, öğe tipsiz kap ve dilim denetlenmiyor; çalışma anı öğe tipini taşımıyor [test: `TestElementAssignType`, `TestOptionalElementListIsNotARequiredList`, `TestListPlusElementTypes`, `tests/negative/oge_tipi_atama.tr`]. Öğe tipi bilinen listenin `.TR_contains`'ine ayrı tipte değer H1142 (motor hatasız `TR_false` veriyordu; sayılar ve `TR_none` geçer) [test: `TestListContainsElementType`, `tests/negative/liste_icerir_tipi.tr`, `TestMessageSuggestionsRunCleanly`].
- **Yapılacaklar (A, Ö1, 4a5; ölçüldü 30b44b01):** öğelerin tipi aynı olur (karışık değişmez denetim hatası; 4a5: depoda 47 yer, CSV 13.8 ve `TR_forward:` 15.28'in göçü ve tipsiz değer kalıbı önce); boş liste tipini yazar (`TR_var k = []` hata, 3.13); `TR_List[T]` bir `TR_List[T?]` yerine verilmez (değişmez olmayan kapta).

### 3.6a

- **Mevcut durum (Faz 0.6b, 2026-10-29):** demet bir değer (`TupleVal`): değişmez `(a, b)`, `(x)` parantez; `t.0`, `t.0.1` okunur, atamak sözdizimi hatası (H0298), köşeli parantezle indeksleme çalışma hatası (H2778: sabit sayıda `t.0`, değişken indekste aynı tipli öğelerde `[t.0, t.1][i]`, farklı tipli öğelerde `TR_var a, b = t` önerir); sınır dışı indeks çalışma hatası, ileti öğe sayısını söylüyor (H2774); `==` öğe öğe, tensör tutan demette çalışma hatası (H2777); `<` sözlük sırasıyla (IEEE), `.TR_sort`, `TR_min`, `TR_max` 13.10'un sırasıyla; öğeleri anahtar olabilen demet sözlük anahtarı, olamayan öğeli demet anahtarda hata (H2773); metni "(1, "a")", tipinin metni "(Int, String)"; `TR_json_string` dizi yazıyor; kontrol noktasında desteklenmeyen değer (H7457). Tipte tek öğeli parantez gruplar: `(TR_Int)` `TR_Int`, `((TR_Int) -> TR_Bool)?` isteğe bağlı fonksiyon tipi [ölçüldü: `tests/tipler_test.tr`'nin 3.6a testleri, `tests/negative/demet_ogeye_atama.tr`, `parser/types_test.go`, `parser/statements_test.go`, `interp/compiler_test.go`].
- **Mevcut durum (4a3, 2026-11-01):** tipi bilinen demette sınır dışı indeks (`t.2`) H2774, tensör tutan demetin `==`/`!=`'si H2777, sıralanamayan öğe çiftli `<`/`>`/`<=`/`>=` H2772 denetim hatası (çalışma anının kodları); tipi bilinmeyen demette çalışma hatası kalır. Çalışma anı listede tensör tutan demetin `==`'sini hatasız yapıyor (`(1, [x]) == (1, [x])` `TR_true`) [test: `TestTupleTypeRules`, `tests/negative/demet_tipi.tr`, `tests/tipler_test.tr`'nin 3.6a testleri; ölçüldü: tabanın ikilisi].
- **Mevcut durum (4a4, 2026-11-03):** tensör tutan demetin `==`/`!=`'si tensör öğenin listesinde ya da sözlüğünde (iç içe de) olsa da çalışma anında H2777 [test: `tests/tipler_test.tr`'nin "3.6a == öğe öğedir" testi, satır 196–198].
- **Yapılacaklar (D ve F1, Ö3; ölçüm 4a4 DEVİR):** tipi bilinen demette tensör bir listenin ya da sözlüğün içindeyse (`([t], 1) == ([t], 1)`) denetim yakalamıyor, çalışma anı H2777 veriyor (`checker/tuple_comparison.go`'nun `tensorElement`'i yalnız doğrudan ve iç içe demeti görüyor); H2777'nin önerisi (tensör öğeyi `.TR_tolist` ile listeye çevirip karşılaştırmak) tensör listede, sözlükte ya da iç içe demette olunca çalışmıyor.
- **Yapılacaklar:** `TR_json_parse[T]` (13.8) motorda yok, demet tipli alanın diziden kurulması onunla gelir.

### 3.7

- **Mevcut durum:** anahtar ifade, erişim köşeli parantezle, olmayan anahtar hata (H2647), ondalık anahtar hata (H2646) [ölçüldü]. `TR_Dict[K, V]` ayrıştırılıyor ve denetleyici anahtar ve değer tipini taşıyor: `TR_var d: TR_Dict[TR_String, TR_Int] = {"a": "b"}` denetim hatası (H1001), `{"a": 1}`'in tipi `TR_Dict[TR_String, TR_Int]`; karışık değerli sözlük öğe tipsiz [ölçüldü: main `85a19c36`, `TestCollectionElementTypes`, `TestCollectionLiteralTypes`, CLI].
- **Mevcut durum (4a4, 2026-11-03):** anahtar tipi bilinen sözlüğü başka tipte anahtarla okumak ve ona yazmak denetim hatası (H1140: `d[1]`, `d[2] = 3`, `.TR_get(5)`, `.TR_contains(5)`; çalışma anında okuma H2647, `.TR_get` sessizce `TR_none`, `.TR_contains` `TR_false`); değerine başka tipte değer yazmak H1136 (3.6); ondalık anahtar (`{1.5: 1}`, `e[1.5]`, `TR_Dict[TR_Float, V]` yazımı) denetimde de H2646. Tipi bilinmeyen anahtar ve öğe tipsiz sözlük çalışma anında kalıyor [test: `TestDictKeyType`, `TestElementAssignType`, `tests/negative/sozluk_anahtar_tipi.tr`, `tests/negative/oge_tipi_atama.tr`].
- **Yapılacaklar (A, Ö1, 4a5; ölçüldü 30b44b01):** değerleri karışık sözlük değişmezi (3.6'nın karışık liste kuralıyla).
- **Yapılacaklar (D, Ö3; DENETİM istek 70):** anahtarı anahtar olamayan tipte (liste, demette liste) yazılan `TR_Dict[K, V]`.

### 3.7a

- **Mevcut durum (4a3, 2026-11-01):** denetleyici `TR_JSON`'u tanıyor: `TR_json_parse(m)`, `TR_JSON(x)`, `j[k]`, `j[i]` `TR_JSON`; `TR_List(j)` `TR_List[TR_JSON]`; `.TR_get` `TR_JSON?`, `.TR_keys()`, `.TR_length` tipli; `TR_var a: TR_Int = TR_json_parse("1")` H1001. `TR_JSON(x)`'te tipi bilinen JSON olmayan değer ve öğe H2654 (çalışma anının kodu); tipi bilinmeyen yerde çalışma hatası kalır [test: `TestJSONTypes`, `TestJSONConversionKinds`, `TestJSONReturnsAreJSONValues`, `tests/negative/json_cevirme.tr`].
- **Mevcut durum (4a3, 2026-11-01):** JSON ile tipi bilinen JSON olmayan değerin karşılaştırması H1131, `TR_none`'la H1132 (üst düzeyde, kabın içinde, `.TR_contains`'te, eşleme deseninde); `TR_Type(j) == TR_JSON` H1134; `j["k"] = 2` H2403 (çalışma anının kodu), hepsi denetim hatası; tipi bilinmeyen yerde çalışma hatası kalır (H2660, H2661, H2705) [test: `TestJSONComparison`, `tests/negative/json_karsilastirma.tr`, `tests/tipler_test.tr`'nin 3.7a testleri, `TestMessageSuggestionsRunCleanly`].
- **Mevcut durum:** `TR_JSON` kuruluyor, açılıyor, dolaşılıyor; JSON ile JSON olmayanın karşılaştırması üst düzeyde, kabın içinde, `.TR_contains`'te ve eşleme deseninde çalışma hatası (kapta H2705): `j["a"] == 1`, `TR_List(j) == [1, 2]`, `TR_List(j).TR_contains(1)`, `[j[0]] == [1]`, `TR_match j["a"] { 1 => ... }` [ölçüldü: main `85a19c36`, CLI; `tests/tipler_test.tr`'nin 3.7a testleri].
- **Yapılacaklar:** genel tiplerin (Any, Object, StructType) kalkması (4a). `TR_JSON?` ile JSON olmayan değerin karşılaştırması 4a4'te denetim hatası (H1131; öneri öbür yanı çeviriyor: `j.TR_get("q") == TR_JSON(3)`) [test: `TestOptionalJSONComparison`, `tests/negative/ayri_tip_esitligi.tr`, `TestMessageSuggestionsRunCleanly`].
- **Mevcut durum (4a3, 2026-11-01):** `TR_JSON(l)` ve `TR_var j: TR_JSON = TR_JSON({"a": l})` denetimden geçiyor, `a494a16f`'te de [ölçüldü: iki ikiliyle denetim]; `TR_JSON(x)` kopyalıyor [test: `tests/tipler_test.tr`, "3.7a TR_JSON(x) listeyi ve sözlüğü kopyalayarak çevirir"].
- **Mevcut durum (4a4, 2026-11-03):** `TR_Dict(j)` JSON nesnesini `TR_Dict[TR_String, TR_JSON]` kopyası olarak açıyor (3.7a "Açmak"; denetleyici de bu tipi veriyor); başka JSON türü H2652, JSON olmayan değer H2423, `TR_Dict()` H2011 [test: `tests/tipler_test.tr`'nin "3.7a TR_Dict(j)" testi, `TestJSONTypes`].
- **Mevcut durum (4a4, 2026-11-03):** beklenen tipi `TR_JSON` olan değişmezin içinde (iç içe değişmezde de) değişmez olmayan liste ya da sözlük (değişken, çağrının sonucu) denetim hatası, H1133: ileti kopyasını çevirmeyi önerir (`{"a": TR_JSON(l)}`). Tipi bilinen tensör ve JSON'a giremeyen öbür öğe çalışma anının koduyla, H2654 (`TR_JSON(t)` de onu verdiği için önerisiz) [test: `TestJSONLiteralItems`, `tests/negative/json_degismez_ogesi.tr`, `TestMessageSuggestionsRunCleanly`].
- **Mevcut durum (D1 dilim 3, 2026-11-02):** beklenen tipi `TR_JSON` (ya da `TR_JSON?`) olan liste ve sözlük değişmezi, iç içe değişmezleriyle, JSON değeri olarak kuruluyor: bildirim, `TR_const`, parametre (argüman ve varsayılan), dönüş, yapı alanı ve bunlara sonradan atama; `g == TR_JSON(...)` `TR_true`. Denetleyici bu beş yerde değişmezi kabul ediyor (tek atanabilirlik kuralı); değişmez olmayan kap ve tek değer (`TR_var g: TR_JSON = 1`) H1001 kalıyor. JSON'a giremeyen öğe (`[1, TR_inf]`) çalışma hatası (H2653) [test: `TestAJSONPlaceBuildsItsLiteralAsJSON`, `TestJSONLiteralFitsAJSONPlace`, `tests/tipler_test.tr`'nin "3.7a beklenen tipi TR_JSON olan değişmez" testi].
- **Mevcut durum:** iç içelik sınırı 1000 kap: sınama kap açılırken yapılıyor, 1000 kap geçer, 1001 kap düşer (H2500), içteki değer ne olursa olsun; çözmede ve `TR_JSON(x)`'te aynı, kaba konan JSON değerinin kapları da sayılıyor. Eskiden sınır en içteki dizinin boş olup olmamasına bağlıydı [ölçüldü: dal `claude/d1-dilim1-bf7baf`, `tests/tipler_test.tr`'nin 3.7a iç içelik testi, `TestJSONDepthCountsContainers`, `TestMakeJSONDepthCountsContainers`].

### 3.10

- **Mevcut durum:** koşulda yalnız `TR_Bool`; başka tip çalışma anında hata veriyor (H2600), ileti açık sınamayı öneriyor (katman 2) [ölçüldü]. Hata ayıklayıcının koşullu durağı hâlâ doğruluk sınamasıyla (`IsTruthy`) okunuyor.
- **4a2 (ilk dilim):** tipi bilinen koşul `TR_Bool` değilse denetim hatası, çalışma anının koduyla: `TR_if`/`TR_else TR_if`, koşullu `TR_for`, liste üretecinin ve `TR_breakpoint TR_if`'in koşulu H2600, `TR_and`/`TR_or`/`TR_not` işleneni H2601, `TR_assert`'in koşulu H2612; demet koşul (`TR_if (1, 2)`) da [ölçüldü: `TestConditionMustBeBool`, `tests/negative/kosul_mantik_degil.tr`]. Tipi bilinmeyen ve isteğe bağlı (`TR_Bool?`) koşul susar.
- **Yapılacaklar:** `.TR_filter`'ın yükleminin dönüş tipi (fonksiyon tipleri, ikinci dilim); isteğe bağlı koşul (daraltma, 4b); hata ayıklayıcının durağının koşulu da yalnız `TR_Bool` alır.

### 3.13

- **Mevcut durum:** açıklamasız yerel değişkenin tipi ilk değerinden çıkıyor ve kalıcı: başka tipte atama (`TR_var birikim = 0` sonra `birikim += 0.5`) denetim hatası (H1005); dil tanımının 3.13 hata bloğu ve TR testi bunu sınıyor. Tipi çıkarılamayan ilk değerin değişkeni `TR_Any` kalıyor (1.2) [ölçüldü: `TestAssignmentTypeCheck`, `TestCompoundAssignment`, dal `claude/hopeful-brahmagupta-miw41q`].
- **Yapılacaklar (A, Ö1, 4a5; ölçüldü 30b44b01):** çıkarımın kalanı (1.2; bu dilimde isteğe bağlı tip, alan, `TR_if`/`TR_match`, indeks ve döngü değişkeni eklendi); ilk değeri `TR_none` ya da boş değişmez olan değişkende tip açıklamasının zorunluluğu. Tam sayının ondalık beklenen yerde ondalık olmasının kalanı tip haritasını ister: kabın öğesi (`l.TR_append(1)`, `l: TR_List[TR_Float]`, `[1]` ve `TR_Int` kalıyor; `d["a"] = 1` de; indekse değişkenle yazma da: `f: TR_List[TR_Float]`, `n: TR_Int` iken `f[0] = n` denetimden geçiyor, öğe `TR_Int` kalıyor, H1136 sayıları ayrı tip saymıyor [ölçüldü: DENETİM istek 70]) [ölçüldü: DENETİM istek 27 D-5], tam sayı değişkenli değişmez (`[a, 2.5]`) ve tipi ilk değerinden çıkan değişkene atama (`TR_var t = 0.0` sonra `t = 1`). `TR_Int` tipli değişkenin `TR_Float` yerine verilmesi (`f(i)`) denetim hatası olacak, bugün geçiyor ve motor değeri ondalığa çeviriyor; denetleyicinin işi.
- **Mevcut durum (D1 dilim 3, 2026-11-02):** tipi açıkça `TR_Float` (ya da `TR_Float?`) yazılmış yerde tam sayı ondalık kuruluyor: bildirim (`TR_var r: TR_Float = 1` → `TR_Type(r)` `TR_Float`, metni "1.0"), parametre (argüman ve varsayılan, çağrılanın girişinde; `TR_Int` değişkenin değeri de), dönüş, yapı alanı ve bunlara sonradan atama (`r = 2`). Açıklamasız `TR_var i = 1` `TR_Int` kalıyor [test: `TestAFloatPlaceMakesAnIntegerDecimal`, `tests/tipler_test.tr`'nin "3.13 tam sayı yazımı TR_Float beklenen yerde ondalıktır" testi].
- **Geri alınabilir göç (T1 dilim 2, bulgu 2):** T1, 3.13 yokken `TR_Float` parametresine tam sayı veren yerleri ondalık yazıma çevirdi (`p(n)` → `p(n.0)`): `tests/tensor_test.tr` "15.33 parametreler" testi ve `internal/stdlib/ml/model_tools_test.go`'nun `TestParameters`'ı. 3.13 main'de olduğuna göre bu yerler tam sayı yazımına dönebilir.

### 3.14

- **Mevcut durum:** değerin metni dilden bağımsız ("true", "none", "inf"); tipin metni öğe tipsiz: `TR_Type([1])` "List" [ölçüldü].
- **Yapılacaklar (A, Ö1, 4c; ölçüldü 30b44b01):** "List[Int]" ve "Int?" (öğe tipi ister; 4a). Değer öğe tipini taşımadığı için `TR_Type([1]) == TR_Type(["a"])` `TR_true`, boş kapta `TR_Type(l)` "List" [ölçüldü: DENETİM, 2026-11-02, `kol-denetim-istek25-sonuc.md` A2]; parametrenin nereden geleceği (denetimde katlama ya da tip haritası) `pending.md` 3.11'de.
- **Mevcut durum (3.14'ten kalan, D1 hız S1, 2026-11-02):** metin yerleştirmesinin (`interpolation`, `compiler_expr.go`) tek dalı var: derleyicinin `textLang`'ı her zaman `codes.ValueNames()`; ölü dal silindi, hızlı tip anahtarı canlı dalda. "Dosyanın dilinde" diyen yorumlar düzeldi (`compiler.go`, `compiler_expr.go`, `environment.go`, `printing.go`'nun `textOf`'u, `shadow.go`, `test_support.go`) [test: `TestInterpolationGivesTheValueText`].
- **Mevcut durum (ileti):** iletinin (`TR_Message`) metni onu kuran çağrı, tip adıyla: `TR_print(TR_Message("H6903", -1))` "Message("H6903", -1)" yazıyor, her dilde aynı; eskiden İngilizce katalog metniydi. Hata değerinin metni fırlatılan değerin metni [ölçüldü: dal `claude/d1-dilim2-c89bbc`, `TestMessageValueTextIsItsCall`, `TestThrownValueTextIsLanguageIndependent`].
- **Mevcut durum (iletinin değerindeki ad):** dil adı programdaki yazımıyla: "TR_requires_grad", kullanım "`.TR_map(TR_Function)`"; eskiden sınıflı iç yazımla ("TR_arg_requires_grad") ve küçük harfli iç adla ("tr_type_function") giriyordu [ölçüldü: dal `claude/d1-dilim2-c89bbc`, `TestNamesInAMessageValueAreSpellings`, `TestMessageNamesFollowTheMessageLanguage`].

## 4. İşleçler

Kurallar: [`docs/spec/04_operatorler.md`](spec/04_operatorler.md). Taslak: metin DENETİM'den geçti, bulguları işlendi (2026-10-27); kararlar Gemini'nin 1–7 araştırmasından geçti (DENETİM doğruladı, `84ee01b8`); 4.5'in isteğe bağlı karşılaştırması Gemini Koşu 3'ten (`0edb6f20`). Sonraki kararlar (4.3, 4.12, 4.10) Gemini'nin araştırmasını bekliyor. 4.1–4.4, 4.6, 4.9, 4.11 ve 4.12'nin çalışma anı uygulandı (katman 2, DENETİM'den geçti, 2026-10-28); aşağıda yalnız kalanlar. Tip isteyen denetim hataları katman 4'tedir.

### 4.5

- **Mevcut durum:** ayrı tipler sessizce eşit değil sayılıyor (`1 == "a"` denetimden geçer, `TR_false`); fonksiyonlar üst düzeyde (H2613: `f == f`) ve fonksiyon içeren liste, sözlük ve `.TR_contains`'te (H2706: `[f] == [f]`, `{"a": f} == {"a": f}`) çalışma hatası veriyor [ölçüldü: main `85a19c36`, CLI; `tests/operatorler_test.tr:175-178`].
- **Mevcut durum (4a4, 2026-11-03):** tipleri bilinen ayrı iki değerin `==`/`!=`'si denetim hatası (H1141: `"1" == 1`, `[1] == [1.0]`, ayrı iki yapı, uzunluğu ayrı demetler); sayılar (`1 == 1.0`), isteğe bağlı ile asıl tipi ve `TR_none`, JSON (H1131, H1132), fonksiyon, tip değeri, tensör, tipi bilinmeyen değer ve öğe tipsiz kap bu kurala girmiyor. Tipi bilinmeyen yerde çalışma anı ayrı tipleri hâlâ sessizce eşit değil sayıyor [test: `TestDistinctTypesAreNotCompared`, `tests/negative/ayri_tip_esitligi.tr`].
- **Yapılacaklar (D, Ö3; ölçüldü 30b44b01):** fonksiyon içeren değer denetim hatası (4. katman).
- **Gemini Koşu 3 (2026-10-29), uygulanacak:** eşitlenemeyen tipin isteğe bağlısı (tensör, tensör içeren değer) `TR_none` değişmeziyle karşılaştırılabilir (15.5).

### 4.7

- **Mevcut durum:** sağ yanda parantezsiz ikili işleç sözdizimi hatası (H0201); isteğe bağlı tip olmadığı için sol yan her tipten olabiliyor: `1 ?? 2` 1 veriyor [ölçüldü].
- **Yapılacaklar:** sol yan isteğe bağlı değilse denetim hatası.

### 4.9

- **Mevcut durum:** satır sonunda `//`'den sonraki metin bir ifade değilse `#`'i anmayan bir hata çıkıyor: `TR_var y = 6 // sayı, 2 tane` H0141 "expected a value or an expression here" veriyor; metin tanımsız adlardan oluşuyorsa (`TR_var x = 5 // bu bir yorum`) beş hata çıkıyor ve yalnız ilki (tanımsız ad `bu`) `#`'i öneriyor, ikisi "ifadenin değeri kullanılmıyor" [ölçüldü: main `85a19c36`, İngilizce iletilerle denetim].
- **Yapılacaklar:** bu durumlarda da ileti `#`'i önerir; tek hata çıkar.

### 4.10

- **Mevcut durum:** başka tiple `+` tipi bilinen işlenenlerde denetimde, çalışma anının koduyla (H2609 metinde, H2614 metinsiz listede; 4a2 ikinci dilim) [ölçüldü: `TestPlusOperands`, `tests/negative/arti_islenen.tr`]; `*` çalışma anında; öğeleri başvuruyla paylaşılan liste sessizce yineleniyor: `[[0] * 3] * 3`'ün üç satırı aynı liste [ölçüldü].
- **Mevcut durum (4a4, 2026-11-03):** öğe tipleri ayrı iki listenin `+`'sı ve `+=`'sı denetim hatası (H1138: `[1] + ["a"]`, `[1] + [2.5]`); öğeleri başvuruyla paylaşılan (liste, sözlük, tensör, yapı, onları tutan demet) listenin `*`'ı ve `*=`'i denetim hatası (H1139: `[[0] * 3] * 3`), ileti tek öğeli değişmezde her satırı ayrı kuran üreteci önerir (`[[0] * 3 TR_for _ TR_in 0..3]`). Tipi bilinmeyen öğede ikisi de çalışma anında hatasız kalıyor [test: `TestListPlusElementTypes`, `TestSharedRowsAreNotRepeated`, `tests/negative/oge_tipi_atama.tr`, `tests/negative/paylasilan_satirlar.tr`, `tests/operatorler_test.tr`'nin "4.10 paylaşılan satırların yerine" testi, `TestMessageSuggestionsRunCleanly`].
- **Yapılacaklar (D ve F, Ö3; ölçüldü 30b44b01):** başka tiple `*`'ın denetim hatası; H2609 ve H2614'ün önerisi genel bir kalıp (`"metin \\(x)"`, `l + [x]`), programın kendi yazımı değil.
- **Mevcut durum:** yineleme 2³¹ bayt ve 2²⁷ öğede kesiliyor (`sequence_ops.go` `maxRepeatBytes`, `maxRepeatItems`); kural artık bellekle denetler (3.6, 15.42); `.TR_append`, `+=` ve sözlüğe eklemenin bellek denetimi de uygulanacak. `tests/operatorler_test.tr:255-256`'nın iki satırı (H2610, H2615) kural uygulanınca tersine döner.
- `==`'nin iç içelik sınırı 10000 (`operators.go` `equalityDepth`) bir uygulama sınırıdır; 7.6'nın döngü kuralından ayrıdır.

## 5. Deyimler ve kontrol yapıları

Kurallar: [`docs/spec/05_deyimler.md`](spec/05_deyimler.md). Taslak: metin DENETİM'den geçti, bulguları işlendi (2026-10-27); kararlar Gemini'nin 1–7 araştırmasından geçti (DENETİM doğruladı, `84ee01b8`), 5.4 Gemini Koşu 4'ten (`23e7be25`). 5.7 ve 5.11'in sonraki kararları Gemini'nin araştırmasını bekliyor. Motor 2026-10-29'da dil tanımının bugünkü metnine göre yeniden ölçüldü (main `1073aea`): 5.2, 5.8, 5.9, 5.10 ve 5.13 kurala uyuyor, maddeleri yok. `tests/deyimler_test.tr` yeni kurallara göre yazılmış, 23 testi geçiyor [ölçüldü: dosyanın testleri koşuldu]; eski kuraldan kalan iki yer Faz 0.9'da kalktı: 5.6 testinin `TR_Any` parametresi tipsiz bırakıldı (sayıyı ve metni alır, 4c'yi bekler), 5.14 testindeki yapının alanı tipli.

`TR_elif` bilerek kimliği olmayan addır; kimlik kapısı düzelince bilerek yanlış örnekler listesine girer (2. bölümün girişi). Yeni denetim hataları (5.1'in uyarısı, 5.5, 5.6, 5.12, 5.14) uygulanırken depodaki kodlu dosyalar da düzeltilir; yoksa `TestRepoTRFilesPassCheck` kapısı kalır.

### 5.1

- **Mevcut durum:** `x := 5` sözdizimi hatası, ileti `TR_var`'ı öneriyor; ilk değersiz `TR_var y` sözdizimi hatası; fonksiyonun içinde hiç yeniden atanmayan yerel `TR_var` uyarı vermiyor [ölçüldü: `x := 5`, tek başına `TR_var y` ve yerelinde `TR_var a = 3` olan fonksiyon; denetim ve çalıştırma].
- **Yapılacaklar:** fonksiyonun içinde yeniden atanmayan yerel `TR_var` uyarısı, ileti `TR_const`'u önerir (programın en dışı girmez); TR testi.

### 5.3

- **Mevcut durum:** takas, listeden ve demetten açma ve `_` kurala uyuyor; öğe sayısı tutmazsa H2700, H2701 ya da (demette) H2709 çalışma hatası, sağ yan demet yazımıysa denetim hatası (H1026), H1026 ve H2709 örneğini deyimin adlarından ve kaynaktaki sağ yanından kuruyor (eksikte `_` ekler, fazlada sondaki adları siler); `_` okunamıyor (sözdizimi hatası) [ölçüldü: takas, `TR_var p, q = [1, 2, 3]`, okunan `_`, `TR_var x, y = (7, 8)`; `tests/negative/demet_acma.tr`, `checker/unpack_test.go`, `interp/compiler_test.go`; Faz 0.6b].
- **Mevcut durum (4a3, 2026-11-01):** tipi bilinen demeti (değişken, tipli parametre) yanlış sayıda ada açmak da denetim hatası (H1026, öneri programın yazımıyla); fonksiyon değerinin çağrısı, isteğe bağlı ve tipi bilinmeyen demet çalışırken (H2709). Listede sayı hatası çalışma hatası kalır [test: `TestUnpackCountOfAKnownTuple`, `tests/negative/demet_acma.tr`].

### 5.4

- **Mevcut durum:** on iki bileşik atama, listede yerinde `+=` ve `*=`, hedefin bir kez hesaplanması, alan hedefi, tanımsız ada atamanın `TR_var`'ı öneren denetim hatası ve sonucun tipi kurala uyuyor: tam sayı değişkende `x /= 2` ve `x += 1.5` denetim hatası (H1005). `/=`'nin iletisi `//=`'yi değil `//`'yi öneriyor. İşlenenlerin tipi `+` ve `+=`'da denetimde (4a2 ikinci dilim): `s += 1` ve `s + 1` (`s` metin) H2609 [ölçüldü: main `85a19c36`, on iki işleç, `l[f()] += 1`, `p.x += 4`, `x /= 2`, `x += 1.5`, `s += 1`; denetim ve çalıştırma; `TestCompoundAssignment`]. Tensörde indeksle biriktirme 15.6d'de.
- **Yapılacaklar:** `/=` iletisi `//=`'yi önerir.

### 5.5

- **Mevcut durum:** değer veren `TR_if`'in `TR_else`'i zorunlu (sözdizimi hatası); Python'un koşullu ifadesinin iletisi `TR_if c { a } TR_else { b }`'yi öneriyor; `TR_elif` iletisi `TR_else TR_if`'i öneriyor ama ardından ikinci, yanıltıcı bir sözdizimi hatası ("':' bekleniyordu") geliyor; dalların tipi denetlenmiyor: `TR_if n < 0 { "a" } TR_else { 5 }` denetimden geçiyor [ölçüldü: main `85a19c36`, dört küçük program; denetim ve çalıştırma]. Koşulun `TR_Bool` olması tipi bilinen koşulda denetim hatası (H2600, 3.10): `TR_if n { }` (n `TR_Int`) denetimden geçmiyor [ölçüldü: main `85a19c36`; `TestConditionMustBeBool`].
- **Yapılacaklar (A, Ö1, 4a5; ölçüldü 30b44b01):** ifade olarak kullanılınca dalların tipi aynı değilse denetim hatası; `TR_elif`'e tek ileti (ayrıştırıcının toparlanması); denetleyicinin testi.

### 5.6

- **Mevcut durum:** değer, çoklu desen, aralık, parantezli ifade, sabit adı ve `_` desenleri ile `TR_return` kolu çalışıyor; `TR_default` iletisi `_`'yi öneriyor. Kapsama denetimi yok: `_`'siz eşleme denetimden geçiyor, eşleşmeyince çalışırken H2702 veriyor. Düz adla yazılan değişken deseni hata vermiyor, değeri karşılaştırıyor. Kolların tipi denetlenmiyor (bir kol metin, öbürü sayı veriyor). Tensör içeren desen denetim hatası vermiyor, bütün tensörü karşılaştırıyor [ölçüldü: altı küçük eşleme programı; denetim ve çalıştırma].
- **Yapılacaklar (A, Ö1, 4b; ölçüldü 30b44b01):** kapsama denetimi (`TR_Bool`'da iki değer, öbür tiplerde `_`); düz adla yazılan değişken deseni denetim hatası, ileti `(x)`'i ya da `_`'yi önerir; ifade olarak kullanılınca kolların tipi aynı; tensör içeren desenin denetim hatası (15.5 ile); denetleyicinin testi.

### 5.7

- **Mevcut durum:** liste, aralık, metin, bayt dizisi (öğeler `TR_Int`), sözlük ve `TR_JSON` dizisi ve nesnesi dolaşılıyor; iki adlı biçim ve `_` kurala uyuyor; metin, sayı ve yokluk türündeki `TR_JSON` H2664 veriyor; dolaşılan listeye ve sözlüğe ekleme H2703 ve H2704, var olan öğeye atama serbest (bugünkü 5.7 testi). Yapı dolaşılamıyor: `TR_next() -> TR_Int?` metodu olan yapıda `TR_for` "döngü kurulamaz" çalışma hatası veriyor (dönüş tipi `T?` artık yazılıyor) [ölçüldü: main `85a19c36`, her kaynak türü (okuyucu hariç), JSON'un metin türü, `TR_next()` metotlu yapı; denetim ve çalıştırma]. Akışlı okuyucunun iki kez dolaşılması bu turda yeniden ölçülmedi. Çıkarma kuralı ölçülemedi: listeden ve sözlükten öğe çıkaran metot (`TR_pop`, `TR_remove_at`, `TR_remove`; 13.3, 13.4) henüz yok.
- **Yapılacaklar:** `TR_next() -> T?` taşıyan yapının dolaşılması, tek seferlik dolaşıcı, `T` isteğe bağlıysa denetim hatası (ileti öğeyi sarmayı önerir); TR testi.

### 5.11

- **Mevcut durum:** liste üreteci koşullu ve koşulsuz çalışıyor, değişkeni dıştaki aynı adlı değişkeni değiştirmiyor; sözlük üreteci yok: `{k: k * 2 TR_for k TR_in 0..3}` sözdizimi hatası veriyor ("burada bir değer ya da ifade bekleniyordu") [ölçüldü: iki liste ve iki sözlük üreteci; denetim ve çalıştırma].
- **Yapılacaklar:** sözlük üreteci, yinelenen anahtarda son değer; TR testi.
- **Yapılacaklar (dil tanımı turu 2, 2026-11-01):** üreteçte iki değişkenli biçim (`{x: i TR_for i, x TR_in l}`); `fd6a84d3`'de sözdizimi hatası [ölçüldü: tur 2'nin yazımı, `[i TR_for i, x TR_in l]`].
### 5.12

- **Mevcut durum:** blok kapsamı, iç adın dışarıda görünmemesi (denetim hatası), turun kendi kapsamı ve fonksiyon sınırında gizleme kurala uyuyor. Yasak yeniden tanım hata vermiyor: iç bloktaki `TR_var t = 5` dıştaki `t`'yi gizliyor, dıştaki 0 kalıyor; aynı kapsamda ikinci `TR_var b` ve parametreyle aynı adlı gövde değişkeni de geçiyor [ölçüldü: iç blokta, aynı kapsamda ve parametreyle yeniden tanım; denetim ve çalıştırma].
- **Yapılacaklar (A, Ö1, 4b; ölçüldü 30b44b01):** kapsayan ya da aynı kapsamdaki yerel adla yeniden tanım denetim hatası (programın en dışı ve parametreler dahil, sondaki blok sıradan blok); denetleyicinin testi.

### 5.14

- **Mevcut durum:** tanımsız ad çalışmadan önce bulunuyor, ileti en yakın adı öneriyor. Tanımından önce kullanılan değişken, fonksiyon ve yapı denetimden geçiyor, çalışırken "böyle bir ad yok" hatası veriyor; sonra tanımlanan değişkeni okuyan fonksiyon, değişkenden önce çağrılınca da. Modülün en dışı sıraya bağlı: önündeki sabiti okuyan sabit modül çalışırken düşüyor; birbirine döngüyle bağlı iki sabit denetimden geçiyor. Aktarılan modülün fonksiyonu aktaranın en dış adını okuyor: modülde tanımsız ad aktaranda tanımlıyken denetim geçiyor, çağrı aktaranın değerini veriyor; modül tek başına denetlenince aynı ad hata [ölçüldü: tanımsız ad, öne alınmayan değişken, fonksiyon ve yapı, iki modül dosyası; denetim ve çalıştırma]. Takma adsız aktarmadaki tanımsız ad 1.3'te.
- **Yapılacaklar:** tanımından önce kullanma denetim hatası; fonksiyon ve tip tanımları bloğun her yerinden görünür; fonksiyonu çağıran ya da değer olarak alan ifade, fonksiyonun doğrudan ya da dolaylı kullandığı değişkenlerin tanımından sonra gelir (ileti tanımı yukarı almayı önerir); modülün en dışı sıradan bağımsız, sabitler arasında döngü denetim hatası; dosya aktaranın adlarını görmez, denetimde de çalışırken de; denetleyicinin testi.

## 6. Fonksiyonlar

Kurallar: [`docs/spec/06_fonksiyonlar.md`](spec/06_fonksiyonlar.md). Taslak: metin DENETİM'den geçti, bulguları işlendi (2026-10-27); kararlar Gemini'nin 1–7 araştırmasından geçti (DENETİM doğruladı, `84ee01b8`). 6.1, 6.6, 6.7 ve 6.9'un sonraki kararları Gemini'nin araştırmasını bekliyor. Motor 2026-10-29'da dil tanımının bugünkü metnine göre yeniden ölçüldü (main `1073aea`): 6.7 ve 6.11 kurala uyuyor, maddeleri yok. Kuralla aynı olanlar: döngüde kurulan kapanış o turun değerini görüyor, fonksiyon dıştaki değişkene atayabiliyor, sırayla verilen argümanın tipi tutmazsa çalışmadan önce bulunuyor, varsayılan her çağrıda yeniden hesaplanıyor, derinlik sınırı 10000 [ölçüldü: küçük programlar; denetim ve çalıştırma].

`tests/fonksiyonlar_test.tr` yeni yazımla yazılmış, 20 testi geçiyor; depoda `TR_it`, `TR_acc`, adsız `TR_func` ve parantezsiz ok kalmadı (yalnız olumsuz fikstürde) [ölçüldü: dosyanın testleri koşuldu; `.tr` dosyalarında git grep]. Eski kurala dayanan testler: eksik, fazla, iki kez verilen argümanı ve varsayılandaki ileri başvuruyu çalışırken yakalayan 6.4 ve 6.5 testleri (denetim hatası olunca `tests/negative/` fikstürlerine taşınır).

### 6.1

- **Mevcut durum:** dönüş tipinin `:` ile yazımı, `-> TR_NoneType`, `def f(x):`, satır başındaki `@f` ve aynı blokta aynı adla ikinci tanım sözdizimi hatası, iletiler TR'deki yazımı söylüyor. Tipsiz parametre (fonksiyon, metot, kurucu) ve değer döndürüp `->` yazmayan `TR_func` denetim hatası (H1109–H1113, 4a1): ileti düzeltilmiş satırı veriyor; tip varsayılandan ya da `TR_return`'lerin ortak tipinden çıkınca düzeltme uygulanabilir, dosyadaki çağrıların ortak tipinden tahmin edilince uygulanamaz (ileti tahmin olduğunu söylüyor), bulunamazsa `x: <tip>` ve nedeni. Depodaki programlar tipli. Tek istisna gömülü kitaplığın 4c'yi bekleyen 29 yeri (genel ve birleşim tipi: birden çok tip alan parametre, alan ve dönüş): adla tutuluyor (dosya, sahip, ad), yalnız kitaplığın kendi dosyası denetlenirken tanı vermiyor, aynı yazım kullanıcının programında hata; liste yalnız küçülür, 4c'de kalkar (`internal/lang/checker/library_exceptions.go`, nedenleri `internal/testdata/typed_code_known.txt`) [ölçüldü: `TestUntypedFunctionParameter`, `TestUntypedMethodParameter`, `TestMissingReturnType`, `TestTypedSignatureSuggestionRuns`, `TestCheckJSONTypedSignatureFixesMakeTheFileClean`, `TestLibraryExceptionsAreUsed`, `TestRepoProgramsAreTyped`, dal `claude/charming-pasteur-q1vsvo`].
- **Yapılacaklar:** kitaplığın 29 istisnası 4c'de (tip parametresi ve birleşim tipi) kalkar; argümanın parametre tipine göre denetimi metotta, kurucuda ve adlı argümanda (4a2).
- **Dil tanımı düzeldi (2026-10-29):** aynı adla ikinci tanım artık tanımda da sözdizimi hatası (1.3); motor zaten öyle.

### 6.2

- **Mevcut durum:** gövdenin son ifadesi dönüş değeri değil (örtük dönüş kalktı), programın en dışında `TR_return` sözdizimi hatası; eksik dönüş denetlenmiyor: dönüş tipi `TR_Int` olan fonksiyonda `TR_return`'süz yol denetimden geçiyor, çalışınca yokluk veriyor; fonksiyon tipi beklenen fonksiyon ifadesinin blok gövdesinde de [ölçüldü: dalı biten ve bitmeyen fonksiyonlar, `TR_throw`, sonsuz döngü, `TR_match` ve `TR_exit` ile biten yollar; denetim ve çalıştırma].
- **Yapılacaklar (A, Ö1, 4b; ölçüldü 30b44b01):** bitmeyen yol denetim hatası, ileti `TR_return`'ü önerir; bitiren deyimler: `TR_return`, `TR_throw`, `TR_exit`, çıkışı olmayan sonsuz döngü, bütün dalları biten `TR_if` ve bütün kolları biten `TR_match`; denetleyicinin testi.

### 6.3

- **Mevcut durum (Faz 0.6b, 2026-10-29):** `TR_return a, b` bir demet döndürüyor: sonuç açılıyor, tutuluyor, argüman oluyor, listeye konuyor, `t.0` ile okunuyor (`t.0`'ın tipi biliniyor); H2708 kalktı. Açmada sayı tutmazsa dönüşü `-> (A, B)` diye yazılmış fonksiyonun doğrudan çağrısında denetim hatası (H1026), fonksiyon değerinde çalışma hatası (H2709); iki ileti de örneğini deyimin adlarından ve sağ yanından kuruyor. Dönüş tipi yazılmayan fonksiyon denetim hatası (6.1). Açılan adlar tipsiz kalıyor: `TR_var a, b = f()` sonrası `b`'nin tipi denetlenmiyor [ölçüldü: main `85a19c36`, `tests/tipler_test.tr`'nin 3.6a testi, `tests/fonksiyonlar_test.tr`'nin 6.3 testleri, `checker/unpack_test.go`, `interp/compiler_test.go`; küçük programlar].
- **Yapılacaklar:** dönüş tipinin öğe tipleriyle açılan adların tipi (4a2).

### 6.4

- **Mevcut durum:** varsayılansız parametrenin varsayılanlıdan sonra gelmesi sözdizimi hatası; varsayılan her çağrıda soldan sağa hesaplanıyor, önceki parametreleri görüyor, `[]` varsayılanı her çağrıda yeni liste; varsayılanda parametrenin kendisi ya da sonraki parametre ancak çalışırken bulunuyor (H2440) [ölçüldü: sıralama hatası, ileri başvuru, sayaçlı varsayılan ve liste varsayılanı; denetim ve çalıştırma].
- **Yapılacaklar:** varsayılanda parametrenin kendisi ve sonraki parametreler denetim hatası; denetleyicinin testi.

### 6.5

- **Mevcut durum:** adlı argümanlar her sırada veriliyor; `k=2` sözdizimi hatası, ileti `k: 2`'yi öneriyor; Python'un `*args`'ı genel bir sözdizimi hatası veriyor; sırayla verilen argümanın tipi tutmazsa denetim hatası. Eksik, fazla, bilinmeyen adlı ve hem sırayla hem adla verilen argüman ancak çalışırken bulunuyor (H2414, H2412, H2415, H2413); adlı argümanın tipi hiç denetlenmiyor (`b: "x"` hata ancak gövdedeki işlemde veriyor); parametre bekleyen yerleşiğe verilen sondaki blok çalışırken H2711 [ölçüldü: iki parametreli fonksiyona sekiz çağrı, liste ve sayı parametresine atama; denetim ve çalıştırma].
- **Yapılacaklar:** eksik, fazla, bilinmeyen ve iki kez verilen argüman ile tipi tutmayan adlı argüman denetim hatası, ileti parametrenin adını söyler; denetleyicinin testi.

### 6.6

- **Mevcut durum:** fonksiyon ifadesinin tek yazımı `(x) => ...`: parantezsiz ok, adsız `TR_func`, ifade yerinde `TR_func` tanımı ve `lambda` sözdizimi hatası, iletiler oku öneriyor; `(_) => 0` çalışıyor, `_` okunamıyor. Beklenen tip yokken (tipi açıklanmamış değişkenin ilk değeri, doğrudan ya da liste, sözlük, demet değişmezinde) tipsiz parametre denetim hatası (H1111); denetleyici yerleşik imzalarını bilmediği için her çağrı argümanı, tipli değişken ve dönüş beklenen tipi olan yer sayılıyor, parametrenin tipi oradan çıkarılmıyor (`TR_Any` kalıyor) [ölçüldü: `TestUntypedFunctionExpressionParameter`, dal `claude/charming-pasteur-q1vsvo`]. Blok gövdesinin eksik dönüşü 6.2'de.
- **Yapılacaklar:** parametre tipinin beklenen tipten çıkarımı ve beklenen tipin denetimi (yerleşik ve kitaplık imzalarıyla, 13.9; 4a2); denetleyicinin testi.

### 6.8

- **Mevcut durum:** kapanış dıştaki değişkeni okuyor ve atıyor, değişiklik iki yanda görünüyor, özyineleme serbest; dıştaki değişkene okumadan önce atayan fonksiyon uyarı vermiyor [ölçüldü: dıştaki sayaca okumadan atayan fonksiyon; denetim ve çalıştırma].
- **Yapılacaklar:** dıştaki değişkene bu fonksiyonda okumadan önce atama uyarısı, ileti `TR_var`'ı önerir; TR testi.

### 6.9

- **Mevcut durum:** fonksiyon tipinin yazımı ayrışıyor (`(TR_Int) -> TR_Bool`, sağdan birleşen `->`, `() -> (TR_Int, TR_Int)`), ama denetlenmiyor: `(TR_Int) -> TR_Bool` tipli değişkene sayı döndüren ok atanabiliyor; fonksiyon değeri adlı argümanla ya da eksik argümanla çağrılınca H2710 (bugünkü 6.9 testi); metin tipin metni değil: adlı fonksiyon "<function f>", ok "<lambda>"; `TR_Type(f)` genel tip "Function" veriyor (3.11) [ölçüldü: main 1073aeab]. Fonksiyonların karşılaştırması 4.5'te.
- **Yapılacaklar (A, Ö1, 4a5 ya da 4c; ölçüm status'un):** fonksiyonun metni tipinin metni ("(Int) -> Bool"), `TR_Type(f)` fonksiyon tipi; parametre ve dönüş tiplerinin denetimi (4a); kimlik listesindeki genel fonksiyon tipi kalkar (3.11); TR testi.
- **Tip yazımı ayrışıyor (Faz 0.6a, 2026-10-29):** `(T) -> R`, `() -> R`, `-> (A, B) -> C` ayrıştırılıyor; denetleyici `TR_Function`'a indirgiyor, parametre ve dönüşü denetlemiyor (4a2).

### 6.10

- **Mevcut durum:** tip parametresi yok: `TR_func baştaki[T](l: TR_List[T]) -> T?` sözdizimi hatası ("'(' bekleniyordu"); çağrıda tip yazımı (`f[TR_Int]([3])`) denetimden geçiyor, çalışırken "Fonksiyon tipi indekslenemez" veriyor [ölçüldü: main 1073aeab].
- **Yapılacaklar:** tip parametreleri ve çağrıda çıkarımı, tip yazımının listenin öğesinin çağrısından ayrılması; kısıtsız `T`'de yalnız her tipte geçerli işler, öbürleri kısıt ister (13.10, 7.8); TR testi.
- **Tip yazımı ayrışıyor (Faz 0.6a, 2026-10-29):** tip yerinde `Kutu[TR_Int]` ve `T` ayrıştırılıyor, `Kutu`'ya indirgeniyor; tanımdaki tip parametresi (`TR_func f[T](...)`) hâlâ sözdizimi hatası (4c).

## 7. Yapılar

Kurallar: [`docs/spec/07_yapilar.md`](spec/07_yapilar.md). Taslak: metin DENETİM'den geçti, bulguları işlendi (2026-10-27); kararlar Gemini'nin 1–7 araştırmasından geçti (DENETİM doğruladı, `84ee01b8`). 7.1, 7.3, 7.4 ve 7.8'in sonraki kararları Gemini'nin araştırmasını bekliyor. Motor 2026-10-29'da dil tanımının bugünkü metnine göre yeniden ölçüldü (main `1073aea`); her kuralda bir ayrılık kaldı. Kuralla aynı olanlar: nesne başvuruyla paylaşılıyor, eşitlik alanlardan, metin kurma yazımıyla ve döngüde "...", çağrısız metot nesneye bağlı bir fonksiyon; yapı tanımının sözdizimi hataları (kalıtım ve Python sınıfı iletileri, yinelenen alan ve metot, alan sırası), metotta düz alan adı, parametre olarak `TR_self`, kurucunun değerli dönüşü ve dönüş tipi, durumlu tipin ve arayüzün tanım ve desen hataları çalışmadan önce bulunuyor [ölçüldü: küçük programlar ve `tests/negative/` yapı, durumlu tip ve arayüz fikstürleri; denetim ve çalıştırma]. `tests/yapilar_test.tr` 24 testi geçiyor; alanları tipli (Faz 0.9); verilmeyen alanı, olmayan alanı çalışırken yakalayan 7.2, 7.4 ve 7.5 testleri eski kurala dayanıyor (denetim hatası olunca `tests/negative/` fikstürlerine taşınır), 7.6 testi StructType'ı sınıyor.

Yapının tipi olan genel tip (StructType) kalkar; yapının tipi `TR_Type`'tır (3.11). Örneklerdeki eski genel tipler de kalkar: StructType, Function, BuiltinFunction, Object ve `TR_Any` (`examples/tr/12_tip_adlari.tr`, `14_yapilar_ve_metotlar.tr`) [ölçüldü: örneklerde git grep].

Kütüphanenin bedeli: `.tr` dosyalarında satır başında 47 yapı tanımı (`lib/`'de 16; girintili tanımlar da sayılırsa 67) [ölçüldü: `git grep -nE '^TR_struct' -- '*.tr'` ve `git grep -nE '^\s*TR_struct\b' -- '*.tr'`, main `c39e0b56`]; alanlar ve kurucu parametreleri tipli yazıma çevrildi (Faz 0.9, 6.1 ve 7.1'e bakın). Alan daraltılmadığı için (3.5) `lib/`'deki 9 `TR_self.alan != TR_none` sınaması [ölçüldü: `lib/` için git grep] yerel kopyaya ya da `?.`'ya çevrilir; "yoksa varsayılan ata" kalıbının 4 yeri 3.5'in atamayla daraltmasıyla geçerli kalır; `lib/layers.tr`'nin kurucuları 7.4'e uyuyor [ölçüldü: DENETİM].

JSON'dan tipli yapıya doğrudan okuma (alan adı eşlemesiyle; Rust'ın serde'si, Python'un pydantic'i gibi) henüz hiçbir bölümde yok; sonraki bölümlerde tasarlanır.

### 7.1

- **Mevcut durum:** tipsiz alan (yapının alanı ve durumlu tipin durum alanı) denetim hatası (H1109–H1111, 4a1): ileti tipi varsayılandan (uygulanabilir düzeltme) ya da kurucusuz yapının kurma çağrılarından (tahmin, uygulanamaz) öneriyor, bulunamazsa `ad: <tip>` ve nedeni; varsayılanlı alandan sonra varsayılansız alan, yinelenen alan ve metot, kalıtım yazımları sözdizimi hatası; yinelenen metot tek ileti veriyor; yapının tip parametresi yok: `TR_struct Kutu[T]` sözdizimi hatası. Kitaplığın 4c'yi bekleyen iki alanı (GPT'nin karışık değerli ayar sözlüğü, Muon ya da NorMuon tutan alan) 6.1'in istisnası [ölçüldü: `TestUntypedStructField`, `TestUntypedEnumField`, `tests/negative/tipsiz_imza.tr`, dal `claude/charming-pasteur-q1vsvo`].
- **Yapılacaklar:** kitaplığın iki alanının istisnası 4c'de kalkar; yapının tip parametresi (6.10 ile); TR testi.

### 7.2

- **Mevcut durum:** sırayla ve adla kurma, her kurmada yeniden hesaplanan varsayılan kurala uyuyor; varsayılanı olmayan alan verilmezse, fazla ya da bilinmeyen argümanda ve iki kez verilen alanda ancak çalışırken hata (H2750, H2419, H2418, H2420); alanın tipi kurmada denetimde (4a2 ikinci dilim, H1125; kurucusu olmayan yapıda konumlu ve adlı argüman), çalışma anı alanın tipini denetlemiyor [ölçüldü: `TestConstructionFieldTypes`, `tests/negative/alan_tipi.tr`].
- **Yapılacaklar:** zorunlu alanın verilmemesi ve 6.5'in argüman hataları denetim hatası; kurucusu olan yapıda kurucunun parametre tipleri (7.4); denetleyicinin testi.

### 7.3

- **Mevcut durum:** alan `TR_self.x` ile, metot `TR_self.m()` ile; metotta düz alan ya da metot adı dıştaki ada çözülse de çözülmese de denetim hatası, ileti `TR_self.x`'i öneriyor; `TR_self` parametre yazılınca sözdizimi hatası; nesnesiz metot çalışırken H2751. Python alışkanlığıyla ilk parametreye düz self sözcüğü yazılınca hata yok: o sıradan bir parametre sayılıyor, çağrı eksik argüman hatası veriyor [ölçüldü: dışta aynı adlı değişkenli metot, `TR_self` ve düz self parametreli metot, `tests/negative/yapi_metotta_duz_alan.tr`; denetim ve çalıştırma].
- **Yapılacaklar:** metodun ilk parametresi Python'un self'iyse sözdizimi hatası, ileti `TR_self`'i önerir; TR testi.

### 7.4

- **Mevcut durum:** kurucu varken kurma onun imzasıyla, varsayılanlı alanlar gövdeden önce tanım sırasıyla, değerli `TR_return` ve dönüş tipi sözdizimi hatası; kurucunun imzasına uymayan kurma ancak çalışırken bulunuyor (H2410, H2414); kurucunun atamadığı varsayılansız alan yokluk kalıyor, atanmadan okunan alan ve bütün alanlar atanmadan metot çağrısı hata vermiyor [ölçüldü: varsayılansız alanı atamayan, onu okuyan ve metot çağıran kurucu; denetim ve çalıştırma].
- **Yapılacaklar (A, Ö1, 4b; ölçüm status'un):** kurucunun sonunda her yolda varsayılansız her alanın atanması, atanmadan okuma ve önceden metot çağrısı denetim hatası; kurucunun imzasıyla kurma 6.5'in denetim hatalarıyla; denetleyicinin testi.

### 7.5

- **Mevcut durum:** alan okunuyor ve atanıyor, `TR_const` nesnenin alanı atanabiliyor; olmayan alan ve metoda atama ancak çalışırken bulunuyor (H2468, H2405, H2404); ileti en yakın adı öneriyor ama alanla eşit uzaklıktaki metodu da seçebiliyor (`p.y` için `m`), yakın ad yoksa alanı eklemeyi öneriyor; alanın tipi denetimde (4a2 ikinci dilim, H1124: tipi bilinen ad ya da `TR_self` üzerinden atama, bileşik atama da) [ölçüldü: `TestFieldAssignType`, `tests/negative/alan_tipi.tr`].
- **Yapılacaklar:** olmayan alan ve metoda atama denetim hatası, ileti en yakın alanı önerir; denetleyicinin testi.

### 7.6

- **Mevcut durum:** paylaşım, alanlardan eşitlik, kurma yazımıyla metin ("Nokta(x: 9.0, y: 4.0)"), kendine bağlanan nesnede "...", döngüde `==`'nin çalışma hatası (H2771) ve `TR_Type(p) == Nokta` kurala uyuyor; yapının tipi StructType; fonksiyon ya da tensör alanlı yapının `==`'si denetim değil çalışma hatası (H2770), aynı nesne kendisiyle karşılaştırılınca hata yok, doğru veriyor [ölçüldü: paylaşılan nesne, kendine bağlı nesne, fonksiyon alanlı nesnenin kendisiyle karşılaştırması; denetim ve çalıştırma].
- **Yapılacaklar:** fonksiyon ya da tensör alanlı yapının `==`'si denetim hatası (aynı nesnede de); yapının tipi `TR_Type` (3.11); denetleyicinin testi.

### 7.7

- **Mevcut durum:** `TR_enum`, durumların kurulması, çağrısız alanlı ve çağrılı alansız durumun hatası, kurucunun yokluğu, değişmezlik, eşlemede konumlu desen ve alan bağlama, adlı alan ve koşullu kol hatası, eşitlik ve metin uygulandı (bugünkü 7.7 testleri ve fikstürleri). Kalanlar: tip parametresi yok (`TR_enum Cevap[T]` sözdizimi hatası); kapsama denetimi yok (`_`'siz, eksik durumlu eşleme çalışırken H2702); dıştaki adla aynı bağlanan ad hata vermiyor, dıştakini gizliyor (5.12); alanın tipi denetlenmiyor (`TR_Int` alana metin); tensör alanlı durumun `==`'si denetim değil çalışma hatası (H2770) [ölçüldü: dört durumlu tip programı; denetim ve çalıştırma].
- **Yapılacaklar (A, Ö1, 4a5 ya da 4c; ölçüm status'un):** tip parametresi (6.10 ile); kapsama denetimi (5.6); bağlanan adın 5.12 hatası; alan tipinin denetimi; tensör içeren durumlu tipin `==`'si ve `==` isteyen deseni denetim hatası (15.5); TR testi.

### 7.8

- **Mevcut durum:** `TR_interface`, boş arayüzün ve alanlı ya da gövdeli arayüzün sözdizimi hatası, yapının ve durumlu tipin yapısal karşılaması, nesnenin kendi metoduna giden çağrı, arayüz tipli parametre ve alan, `TR_Type` sınamasıyla dalda asıl tipin alanı çalışıyor. Denetim yok: metodu eksik değer ancak çağrıda "alanı ya da metodu yok" çalışma hatası veriyor, imzası tutmayan değer (`TR_String` parametreli metot) hatasız geçiyor; arayüz değerinin `==`'si hata vermiyor; arayüz kısıtlı tip parametresi (`TR_func g[T: Katman]`) sözdizimi hatası [ölçüldü: bir arayüz, üç yapı ve bir durumlu tiple karşılama, daraltma ve karşılaştırma; denetim ve çalıştırma].
- **Yapılacaklar:** karşılamayan değer denetim hatası, ileti eksik ya da imzası tutmayan metotları söyler; arayüz değerinin karşılaştırması denetim hatası (4.5); tip parametresinde kısıt (6.10 ile); daraltmanın denetleyicide 3.5'in koşullarıyla yapılması; denetleyicinin testi.
- **Karar (2026-10-29):** yapısal karşılama kalır; açık bildirimle karşılaştırması gerekçede (7.8). Eski gerekçe "kütüphanenin bugünkü yapılarını değiştirmeden" motora dayanıyordu, çıkarıldı.

## 8. Hata yönetimi

Kurallar: [`docs/spec/08_hata_yonetimi.md`](spec/08_hata_yonetimi.md). Taslak (2026-10-28): ana oturum yazdı, DENETİM'den geçti (1–7 ile tutarlılık; bulguları işlendi; Ç1: yakalamanın adı zorunlu, 2.9b'den yakalamanın hazır adı kalktı, Ahmet 2026-10-28); kararlar Gemini Koşu 7'den geçti (DENETİM doğruladı, `59efe468`, 2026-10-29). Yeni adların gösterim adları onaylandı (Ahmet, 2026-11-01): `TR_Error` Hata, `TR_code` kod, `TR_value` fırlatılan (üye ad alanında "değer" dolu), `TR_cause` neden (üye ve adlı argüman), `TR_suppressed` bastırılanlar; kodları uygulandıkları gün kayda girer (17.4). `TR_throw` ve `TR_catch`'in katalogdaki açıklamaları ve `tests/` altındaki 8. bölüm testleri `TR_Error`'a göre yeniden yazıldı (D1 ikinci dilim).

### 8.1

- **Mevcut durum:** `TR_try`/`TR_catch` çalışıyor, `TR_catch` zorunlu. `TR_try` ifade olarak da çalışıyor, değeri çalışan bloğun son ifadesi (`TR_var n = TR_try { TR_Int(m) } TR_catch e { 0 }`); eskiden sözdizimi hatasıydı. Python'un `except`'i H2915 (`TR_catch`'i öneriyor), `finally`'si yakalamadan sonra da yerine de H2916 (`TR_defer`'i öneriyor), parantezli ad `TR_catch (e)` H2917 (`TR_catch e`'yi öneriyor; eskiden kabul ediliyordu, depodaki tek kullanım göçtü) [ölçüldü: dal `claude/d1-dilim2-c89bbc`, `tests/hatalar_ertele_test.tr`'nin 8.1 ifade testi, `TestOtherLanguagesCatchSpellingsAreSyntaxErrors`, öneri tablosu].
- **Mevcut durum (ad):** adsız `TR_catch { ... }` sözdizimi hatası H2910, ileti `TR_catch e`'yi öneriyor; yakalamanın hazır adı (bağlamsal kimlik) kalktı. Göç: `.tr` dosyalarında 6 adsız yakalama ve hazır adın 4 kullanımı, Go testlerindeki TR programlarında 5 adsız yakalama ve hazır adın 2 kullanımı adlı yazıma geçti [ölçüldü: dal `claude/d1-dilim2-c89bbc`, `git grep "TR_catch *{"`; `TestCatchWithoutANameIsASyntaxError`, öneri tablosu].
- **Yapılacaklar:** ifade olarak `TR_try`'da dalların tipinin aynı olması ve blokların içindeki adların denetimi (denetleyici; bugün ifadenin bloklarına inmiyor: içindeki tanımsız ad ve farklı dal tipleri denetimden geçiyor [ölçüldü: tr check, dal `claude/d1-dilim2-c89bbc`]); sözdizimi hatalarının olumsuz TR fikstürleri (adsız, parantezli ad, `except`, `finally`, `raise`).

### 8.2

- **Mevcut durum:** fırlatılabilen dört tür: metin, ileti, durumlu tip değeri, `TR_Error`; başka tip çalışma hatası H2911 (`TR_throw` ve `TR_Error(...)`), ileti programın kendi adıyla `TR_throw TR_String(sayı)`'yı öneriyor (değer ad değilse yalnız `TR_String`'i anıyor; eskiden sabit `TR_String(x)`'ti) [test: `TestNotThrowableSuggestsTheProgramsSpelling`]. `TR_throw e` yakalanan hatayı olduğu gibi (kodu, değeri, nedeni, bastırılanları ve ilk fırlatıldığı yer) yeniden fırlatıyor; `TR_Error(v, TR_cause: e)` yeni hata kuruyor, nedeni hata değilse H2912. Python'un `raise x`'i sözdizimi hatası H2914, ileti `TR_throw`'u öneriyor [ölçüldü: dal `claude/d1-dilim2-c89bbc`, `tests/hatalar_ertele_test.tr`'nin 8.2 testleri, `TestPythonRaiseIsASyntaxError`, öneri tablosu].
- **Yapılacaklar:** yakalanmayan hatada nedenin ve çağrı yığınının gösterilmesi (8.6).
- **Gemini Koşu 7 (2026-10-29), uygulanacak:** yeniden fırlatmada çağrı yığını korunur.

### 8.3

- **Mevcut durum:** yakalanan hata bir `TR_Error` ("TR_Type(e)" "Error"); üyeleri `e.TR_code` (iletinin kodu ya da `TR_none`), `e.TR_value` (fırlatılan değer; çalışma hatasında asıl nedenin iletisi, `TR_error_code`'un verdiği kodla), `e.TR_cause`, `e.TR_suppressed`; metni fırlatılan değerin metni ("Message("H2469", "List", "Int", "-")"), okuyana göre değişmiyor. `e == f` çalışma hatası H2913, ileti programın kendi adlarıyla `e.TR_code == f.TR_code`'u öneriyor (iki yan da ad değilse yalnız üyeyi anıyor; eskiden sabit `e`, `f`'ti; öneri tablosu iki ad çiftiyle); yoklukla karşılaştırma serbest. `TR_assert`'in başarısızlığı kodlarıyla yakalanıyor. Sarmalayan iletinin bağlamı (dosyanın yolu, önerilen kontrol noktası) yalnız gösterilen hatada [ölçüldü: dal `claude/d1-dilim2-c89bbc`, `tests/hatalar_ertele_test.tr`'nin 8.3 testi, `TestCaughtRunTimeErrorTextIsItsMessage`, `TestCaughtErrorTypesAreErrorValues`]. Göç: yakalanan hatayı metin gibi kullanan 35 `.tr` ve 8 Go satırı önce `TR_String(h)`'ye geçti (çıktılar bayt bayt aynı); kuraldan sonra sessiz değişen 2 örnek satırı ve sesli düşen testler yeni kurala yazıldı (liste D1 devir notunda).
- **Yapılacaklar:** `TR_value`'nun kapalı birleşim tipi ve `e == f`'nin denetim hatası (denetleyici, 4a); kopyanın daraltılması (4b), `_`'siz eşlemenin kapsaması (4c).
- **Gemini Koşu 7 (2026-10-29), uygulanacak:** kapalı birleşimin yerel kopyası; hatayla biten kol eşlemenin tipine katılmaz (denetimi 4a2).

### 8.4

- **Mevcut durum:** denetim karakterleri 2.6'nın kaçışıyla yazılıyor: `TR_throw "a\u{1b}[2Jb\u{7}c"` → `a\u{1b}[2Jb\u{7}c`; eskiden `\x1b` [ölçüldü: dal `claude/d1-dilim1-bf7baf`, CLI; `TestPrintable`]. TR testi yakalanan metnin değişmediğini sınıyor; terminal çıktısı Go testinde. İletideki `{ad|q}` alıntısı da 2.6'nın kaçışıyla yazıyor (`"a\u{1b}[2Jb"`, yön karakteri `\u{202e}`); eskiden Go'nun `\x1b`'siydi [ölçüldü: dal `claude/d1-dilim2-c89bbc`, `TestRender`].

### 8.5

- **Mevcut durum:** `TR_Message` ve `TR_error_code` çalışıyor (`H6903`); kodsuz hatada (düz metin, sayı, sözlük, durumlu tip değeri) `TR_error_code` koda dönmüyor, hatayı olduğu gibi yeniden fırlatıyor; eskiden hepsi H2441 veriyordu [ölçüldü: dal `claude/d1-dilim1-bf7baf`, `TestErrorCodeRethrowsACodelessError`, `tests/hatalar_ertele_test.tr`'nin 8.5 testi]. Yakalanmayan düz metin hata içte H2441 taşımayı sürdürüyor (`examples/tr/hatalar/12_yakalanmamis_firlat.tr` onu sınıyor).
- **Mevcut durum (zincir):** `TR_error_code` hatanın kodunu, o yoksa nedenler zincirinde (`TR_cause`) dıştan içe ilk kodu veriyor; zincirde kod yoksa hata, nedeniyle birlikte yeniden fırlıyor; durdurma ve `TR_exit` geçiyor [ölçüldü: dal `claude/d1-dilim2-c89bbc`, `TestErrorCodeFollowsTheCauseChain`, `tests/hatalar_ertele_test.tr`'nin 8.5 zincir testi: üç halka, kodlu dış, kodsuz zincir].
- **Yapılacaklar:** programın ayırması beklenen hataların kodları kendi maddelerine yazılacak (Gemini Koşu 4, 6, 7).
- **Yapılacaklar (B, Ö1, Kol A, 13.1'in dilimiyle; ölçüm status'un; dil tanımı turu 2, 2026-11-01):** işletim sisteminin hatası kendi koduna ve değerlerine (yol) çevrilir, sistemin metni değere girmez (8.5). Bugün dosya hatasının değerine Go'nun hata metni giriyor (internal/stdlib/ml/save.go:135, H2101) [kanıtlı: DENETİM]; Windows'ta bu metin İngilizce dil paketi yoksa sistemin dilindedir [kaynak: Go go1.24.0 syscall_windows.go; DENETİM kol-denetim-tur2-rapor-sonuc.md].

### 8.6

- **Mevcut durum:** yakalanmayan hata yalnız fırlatıldığı satırı gösteriyor, çağrı yığını yok [ölçüldü].
- **Yapılacaklar:** çağrı yığını, en son çağrı en altta.
- **Gemini Koşu 7 (2026-10-29), uygulanacak:** yakalanmayan hatada nedenler zinciri.

## 9. Ertele

Kurallar: [`docs/spec/09_ertele.md`](spec/09_ertele.md). Taslak (2026-10-28): ana oturum yazdı; metin DENETİM'den geçti (`9c511225`); kararlar Gemini Koşu 4'ten geçti (DENETİM doğruladı, `23e7be25`, 2026-10-29). `tests/hatalar_ertele_test.tr` ve `examples/tr/16_ertele.tr` eski kuralı sınıyor (fonksiyona bağlı kapsam, çağrı yazımı, birleşik hata metni); kural uygulanırken yeniden yazılır.

### 9.1

- **Mevcut durum:** kapsam en yakın blok: dal, döngünün ve koşullu döngünün turu, `TR_try` ve `TR_catch` bloğu, eşleme kolu; iş blok nasıl biterse bitsin (son deyim, `TR_continue`, `TR_break`, `TR_return`, hata) o bloğun sonunda, ters sırayla çalışıyor; ertelenen bloğun içindeki ertele o bloğun sonunda çalışıyor. Eskiden ertele fonksiyona bağlıydı ve ertelenen bloktaki ertele kayboluyordu [ölçüldü: dal `claude/d1-dilim1-bf7baf`, `TestDeferScopeIsTheNearestBlock`, `TestDeferInsideADeferredBlockRunsAtItsEnd`, `tests/hatalar_ertele_test.tr`'nin 9.1 testleri]. Çağrı yazımı (`TR_defer f(x)`) sözdizimi hatası H0300, ileti çağrıyı bloğa alıyor (`TR_defer { f(x) }`); depodaki bütün çağrı yazımları bloğa göçtü [ölçüldü: aynı dal, `TestDeferIsWrittenOnlyWithABlock`, `tests/negative/ertele_cagri.tr`, `git grep -n 'TR_defer [^{]'`].
- **Yapılacaklar:** bir bloğun son deyimi olan `TR_defer`'in denetim hatası (denetleyici).

### 9.3

- **Mevcut durum:** blok çalıştığı anda okuyor (`TR_defer { TR_print(x) }` sonra `x = 2` → 2); ertelenen bloktan `TR_return`, `TR_break` ve `TR_continue` sözdizimi hatası (H0299), ileti `TR_return`'ü önermiyor; bloğun içindeki döngüde `TR_break`/`TR_continue`, içindeki fonksiyonda `TR_return` geçerli [ölçüldü: dal `claude/d1-dilim1-bf7baf`, `TestDeferredBlockIsAnExitBoundary`, `tests/negative/ertele_cikis.tr`].

### 9.4

- **Mevcut durum:** kapsamın hatası, kapsam hatasız bittiyse ilk ertelenen işin hatası korunuyor; ertelenen işlerin öbür hataları çalışma sırasıyla `e.TR_suppressed`'de; eskiden yakalanan metinde birleşiyordu ("govde hatasi; error in defer as well: ...") [ölçüldü: dal `claude/d1-dilim2-c89bbc`, `tests/hatalar_ertele_test.tr`'nin 9.4 testi (gövde hatası ve iki ertele hatası; hatasız kapsamda iki ertele hatası), `TestCaughtErrorTypesAreErrorValues`].
- **Yapılacaklar:** yakalanmayan hatada bastırılanların kodu ve iletisiyle gösterilmesi (8.6); gösterilen metin bugün birleşik.
- **Mevcut durum (D1 dilim 3, 2026-11-02):** yığın durdurma (1.5) ya da iptalle (10.9) çözülürken de blokların ertelenen işleri çalışıyor (önceden yalnız fonksiyonun, görevin ve programın ertelenenleri çalışıyordu, bloğunki atlanıyordu); çözülürken ertelenen işin hatası `TR_catch` ile yakalanmıyor ama kaybolmuyor: durdurmada programın hatalarına eklenip yazılıyor (çıkış kodu 1), iptalde bekleyenin aldığı iptal hatasının `TR_suppressed`'inde (önceden ikisi de sessizce atılıyordu) [test: `TestDefersKeepTheirErrorsWhileUnwinding`].

## 10. Eşzamanlılık

Kurallar: [`docs/spec/10_eszamanlilik.md`](spec/10_eszamanlilik.md). **Kapandı** (2026-10-29, DENETİM 11-18-2); kararlar sonra Gemini Koşu 4'ten de geçti (DENETİM doğruladı, `23e7be25`). 2. tur (2026-10-29): DENETİM 10-1'in bulgularıyla (süreyle devir, uzun yerleşiklerin kilidi bırakması, iptal, 2.8'in istisnası, verilen sıradaki ilk hata). 3. tur: DENETİM 10-2'nin bulgularıyla (gecikme sözü 10 ms; 2026-11-01'de geliş sırasına ve tutan görevin 5 ms'sine bağlandı, `pending.md`; izin depoda ve hepsi birden, geri çağırmada bırakılır; iptal 1.5'in noktaları ve yerleşik ayrımıyla; alt görevi bekleyen de iptal hatası alır; iptal ve zaman aşımı ileti kodları). Yeni adların gösterim adları onaylandı (2026-10-29): `TR_Task` Görev, `TR_wait` bekle, `TR_timeout` süre_sınırı (`g.TR_wait(TR_timeout: s)`'nin adlı argümanı), `TR_cancel` iptal_et, `TR_done` bitti_mi; kodları kayıtta (D1 dilim 3, 2026-11-02). `TR_sleep`'in Türkçe adı "bekle"den "uyu"ya döndü (görevi beklemekle karışmasın diye; uygulandı).

### 10.1

- **Mevcut durum (D1 dilim 3, 2026-11-02):** `TR_async f(x)` ve `TR_async { ... }` bir ifade, görev değeri (`TR_Task`) veriyor; deyim olarak yazılınca değer atılıyor (ayrıştırıcıda `AsyncExpression` deyimi sarar, `TR_try`'ın ifadesi gibi). `g.TR_wait()` sonucu veriyor (blokta `TR_none`) ya da görevin hatasını fırlatıyor; `g.TR_done()` beklemeden durup durmadığını söylüyor; görevin metni "Task" (3.14, tipinin adı). Görevin kendi değerini beklemesi H2918 (doğru yazımı kimlikle verir: sonucu görevin dışında bekleyin); görevde olmayan üye H2474, çalışırken de denetimdeki ipucuyla (üyenin hangi tiplerin metodu olduğu). Denetleyici görevin tipini biliyor: `TR_async f(x)` `TR_Task[T]` (T çağrının tipi), blok `TR_Task[TR_NoneType]` (blok görevini tutan değişken de bu tipi alır, olmayan üyesi denetimde H2474); `g.TR_wait()` T, `g.TR_done()` `TR_Bool`; `TR_Task[TR_String]` yere `TR_Task[TR_Int]` görev H1001. Dönüşü bilinmeyen çağrının görevi parametresiz `TR_Task` (denetleyicinin iç tipi): `.TR_wait()`'in tipi bilinmiyor, tipli yere verilebiliyor. Tip açıklamasında ve tip değerinde yeni kabul yok. Görev ifadesinin içi de denetleniyor (çağrıdaki ya da bloktaki tanımsız ad H1010) [test: `TestATaskValueGivesItsResult`, `TestWaitThrowsTheTasksError`, `TestATaskCannotWaitForItself`, `TestATaskHasOnlyItsMembers`, `TestAsyncIsAnExpression`, `TestATaskHasItsCallsType`, `TestATaskValuesInsideIsChecked`, `tests/eszamanli_test.tr`'nin "10.1 eşzamanlı bir ifadedir" testi].

### 10.2

- **Mevcut durum:** kilit bir bekleme kuyruğu: bekleyen görevler onu geldikleri sırayla alıyor; kilidi tutan görev, bekleyen varken kilidi aldığından 5 ms sonraki ilk deyim sınırında (blokta iki deyimin arası, döngü turu, kullanıcı fonksiyonunun girişi) devrediyor; 256 turluk sayaç ve çağrı başına devir kalktı. Bekleyen yokken sınırın bedeli tek atomik okuma. Bekleyen varken süreyi bekleyenin zamanlayıcısı (5 ms) ölçüyor ve kuşak damgalı bir bayrak kuruyor; sınır yalnız bayrağı okuyor, eski tutanın geç zamanlayıcısı yeni tutanı devredemiyor [kanıtlı: `gil.go` `arm`, `expire`, `take`; DENETİM istek 24]. Zamanlayıcı yük altında geç kalabildiği için saat de okunuyor, en çok 64 sınırda bir (sayı geçmiş deyimlerin hızına uyuyor): zamanlayıcı geç kalınca devir en çok 64 deyim sınırı gecikiyor, kaba saatte tutuşun başında bir fazla. Yarış altında 10 koşuda zamanlayıcıyla en uzun bekleme 11,5–14,0 ms, yalnız yedek saatle bir beklemede en çok 65 yavaş tur, aşan 0 [ölçüldü: D1, `233a9b56`, `TestTheLockIsHandedOffInTimeAfterStatementsSlowDown`]. Linux'ta 20 koşuda zamanlayıcıyla en uzun bekleme 11,0–12,0 ms, yalnız yedek saatle bir beklemede en çok 64 tur, aşan 0; görevlerin verimi (G görev, her biri 3 M turluk döngü, G × süre(1) / süre(G)) önce 0,72–0,75, sonra G = 2, 4, 8'de 0,98–1,04; tek görevde süre 346 → 315 ms, `fib` 60 → 61 ms, `dongu` 270 → 261 ms (ortanca, 7 örnek, ABAB) [ölçüldü: Colab CPU, `eaac5ba5` → `233a9b56`, kayıtlar `kiyas-d1/20261102100541`, `d1-gecis-testi/20261102100810`]. Önünde n bekleyen görevin kilidi alma süresi (n + 1) × 5 ms ve bir paya sığıyor: 20 koşuda en büyük aşım 15,7 ms (Windows'ta saatin adımı ~0,5 ms; yüklü makine), test payı 32 ms [ölçüldü: dal `claude/d1-dilim2-c89bbc`, `TestLockIsTakenInArrivalOrderWithinTheDelay`, iki ve üç meşgul görev]. `TR_json_parse` (8 KB'tan büyük metinde), `TR_json_string` (500 düğümden büyük `TR_JSON`'da) ve metnin metotları (64 KB'tan büyük metinde) kilidi bırakıyor; `TR_json_string` liste ve sözlük girdisinde kilidi tutuyor (girdi değişebilir; dil tanımı düzeltilecek, `pending.md`); `TR_read_file` metne dönüşümü de kilit dışında yapıyor [test: `TestJSONReleasesTheLockOnlyForLargeUnchangeableInput`, `TestLongTextMethodsReleaseTheLock`]. Gecikme sözü tensör yerleşiklerinde tutmuyor (K izi): tensör işlemleri, geri yayılım, iyileştiricinin adımı, örnekleme ve GPU beklemeleri kilidi tutuyor (`internal/stdlib/ml`'de kilidi bırakan sarmalayıcı yalnız `TR_save_tensors`'ta). 60 matris çarpımı sürerken 5 ms'de bir uyanan görev 301 ms bekledi [ölçüldü: DENETİM 10-1, main `5ec35e05`]. Bu sayı süreyle devirden önceydi: `30b44b01`'de kilit her `@` deyiminden sonra devrediliyor; 60 kez 1024×1024 `@` sürerken gözcünün en büyük beklemesi 20 koşuda 3,2–34,7 ms (ortanca 8,2; 12/20 koşu ≤ 10 ms), 100 Hz döngüde 15/20 koşuda tur kaçıyor; tek çağrısı uzun 8 kez 2048×2048 `@`'da en büyük bekleme 31,4–93,9 ms (0/20 ≤ 10 ms) [ölçüldü: T1, CPU, dal `claude/s-kilit` `7e81e7d2`, `TestOtherTasksRunDuringLongTensorWork`, 20 koşu, ölçüm penceresiyle].
- **Mevcut durum (ölçüm, 2026-11-01):** (süreyle devirden önce) `internal/compute`'ta kilidi bırakan çağrı yok. Bir görev 5 ms'de bir uyurken en büyük gecikme: TR döngüsü ve fonksiyon çağrısı 4 ms, 4×4 tensör döngüsü 5 ms; 1024×1024 `@` 60 kez 833 ms, 1 milyon öğeli `.TR_sort()` 598 ms, AdamW `.TR_step()` 212 ms, kullanıcı türevli `TR_backward` 39 ms; yüksüz taban 6 ms [ölçüldü: D1, `e9c64a45`, 230 koşu, `d1-10-2-olcum.md`]. `TR_time_ms` sıkı döngüde 10–11 ms sıçradı; gecikme sözü TR'nin içinden bu saatle sınanamaz, Go testi tekdüze saatle ölçer.
- **Mevcut durum (bırakanlar):** `TR_sleep`, `TR_input`, dosya okuma ve yazma, `TR_wait_all`, modül yükleme beklemesi, veri kümesini açma ve okuma, dosya işleri, `TR_save_tensors`, `TR_load_tensors`, parçalayıcıyı yükleme ve büyük girdide kodlama [kanıtlı: DENETİM 10-1].
- **Yapılacaklar (A, Ö1, Kol B S-kilit; ölçülmedi):** `TR_save_tensors` kilidi bırakıp tensörün deposunu kopyalamadan dosyaya yazıyor; bu sırada başka görev aynı depoyu yerinde değiştirirse dosyaya karışık veri gidebilir [kanıtlı: T1, internal/stdlib/ml/save.go:162 ve :174; sonucu ölçülmedi; DENETİM istek 64]. S-kilit'in okuma izni bunu kapatır.
- **Yapılacaklar (C, Ö2, Kol B S-kilit; ölçüm status'un):** tensör ve veri yerleşiklerinin kilidi bırakması (K izi) ve deponun okuma ve yazma izni (hepsi birden, geri çağırmadan önce bırakılır). Önce ve sonra ölç: küçük tensör işlemli döngüde izinlerin bedeli (tahmin ~%5) [hipotez: DENETİM 10-2]. Geri çağıran yerleşikler: `TR_backward` (kullanıcı türeviyle), `TR_fuse`, anahtarlı sıralama, `TR_grad_check`. GPU'da iznin kuyruğun bitişine bağlanması [hipotez: GPU yolu açılmadı].
- **Yapılacaklar (yedek saat, 2026-11-02):** kilit alınınca yedek saatin sayısını 1'e indirmek; yalnız kaba saatte tutuşun başındaki fazladan bir sınırı kaldırır, geçişin 64'ü kalır; ölçümle (D1'in önerisi, DENETİM istek 24). Döngünün ve üretecin her turu artık devir noktası (`loopTurn`) [kanıtlı: `compiler_loop.go` `listComprehension`]; üretecin turunun gecikme testi yok.

### 10.3

- **Mevcut durum:** listedeki ağ istekleri, dış süreç ve sunucu beklemeleri bugün yerleşik olarak yok [ölçüldü: DENETİM 10-1]; kural gelecek yerleşikler için (yol haritasının "Ağ" maddesi).

### 10.4

- **Mevcut durum:** `TR_no_grad` bloğunda açılan görev türev izliyor: `TR_no_grad { TR_async { ... w * 2.0 ... } }`'in sonucu türevli [ölçüldü: main `8b115692`]; hesap türünün mirası 15.4'ün status'unda (15'in yan değişikliği, uygulanmadı).
- **Yapılacaklar:** görev açıldığı yerin türevsiz kipini ve hesap türünü alır; TR testi.

### 10.5

- **Mevcut durum:** `TR_wait_all()` başlatılmış bütün görevleri bekliyor ve `TR_true` veriyor; 10.5 "sonuç vermez" diyor [ölçüldü: `9abf8ed5`].
- **Mevcut durum (D1 dilim 3, 2026-11-02):** `TR_wait_all(görevler)` görevlerin hepsini bekliyor, sonuçları verilen sırayla liste olarak veriyor (bitme sırası değiştirmiyor). Hatalarda verilen sıradaki ilk hatalı görevin hatası fırlıyor, öbürleri onun `TR_suppressed`'ine sırayla ekleniyor; iki kez verilen görevin hatası bir kez sayılıyor. Görevler ancak bütün liste beklenince beklenmiş sayılıyor (10.6). Görev olmayan öğe H2919, birden çok argüman H2012. Denetleyicide `TR_List[TR_Task[T]]` için sonuç `TR_List[T]` [test: `TestWaitAllGivesResultsAndTheFirstErrorInTheGivenOrder`, `TestWaitAllTakesNoneOrATaskList`, `TestATaskHasItsCallsType`, `tests/eszamanli_test.tr`'nin 10.5 testleri].
- **Yapılacaklar:** argümansız `TR_wait_all()` sonuç vermez (bugün `TR_true`).

### 10.6

- **Mevcut durum:** beklenmeyen görevin yakalanmamış hatası yazılıyor, program sürüyor ve 1 koduyla bitiyor [ölçüldü].
- **Mevcut durum (D1 dilim 3, 2026-11-02):** deyim olarak yazılan görevin hatası görev bitince yazılıyor. Değeri tutulan görevin hatası `.TR_wait()`'te ya da `TR_wait_all(görevler)`'de fırlıyor; kimse beklemediyse program biterken (en dıştaki ertelenenlerden sonra: ertelenen iş görevi bekleyebilir) ya da test biterken başladığı satırla yazılıyor: komut satırında standart hata ve çıkış kodu 1, testte testin hatası, hata ayıklamada konsol. Beklenip `TR_try` ile yakalanan hata çıkış kodunu bozmuyor [test: `TestExitCodes`'un "beklenmeyen görevin hatası 1" ve "yakalanan görev hatası 0" satırları, `TestUnwaitedTaskErrorShownInReport`, `TestDAPUnwaitedTaskError`, `tests/eszamanli_test.tr`'nin 10.6 testi].
- **Mevcut durum (DENETİM 35 düzeltmeleri, 2026-11-02):** hata iki yolda daha kaybolmuyor. Süre sınırı dolunca görevin kendi hatası ve ertelenen işlerinin hataları zaman aşımı hatasının (H2921) `TR_suppressed`'ine sırayla giriyor (10.7). `TR_wait_all(görevler)` görevleri ancak bütün liste beklenince beklenmiş sayıyor; bekleme yarıda kalırsa (iptal, durdurma) görevlerin hatası program ya da test biterken yazılıyor [test: `tests/eszamanli_test.tr`'nin 10.7 bastırılanlar testi, `TestAnAbandonedWaitAllKeepsTheErrors`]. İptalden önce olan kendi hatanın program sonunda yazılmasının testi yok.

### 10.7

- **Mevcut durum:** sabit hızlı döngünün süre ölçmesi için 13.1'in `TR_monotonic_ns`'i kod kaydında yok [ölçüldü: DENETİM 10-1].
- **Mevcut durum (D1 dilim 3, 2026-11-02):** `g.TR_wait(TR_timeout: saniye)` en çok o kadar bekliyor; süresinde biten görev sonucunu veriyor ya da hatasını fırlatıyor; süre dolarsa görev iptal ediliyor (10.9), durması bekleniyor ve zaman aşımı hatası (H2921) fırlıyor; aynı görevi sonra bekleyen iptal hatasını (H2920) alıyor. Görevin kendi hatası ve ertelenen işlerinin hataları H2921'in `TR_suppressed`'ine giriyor. Gövdesi bitmiş, alt görevini bekleyen görevde de süre sınır kadar sürüyor: iptal alt göreve geçiyor. Süre `TR_sleep`'inki gibi denetleniyor (H2433, H2434), saat tekdüze. Ağır turlu döngüdeki görev de bir sonraki turda duruyor. İki kod 10.7'nin cümlesine yazıldı. `TR_done` 10.1'de uygulandı [test: `TestAWaitWithATimeout`, `TestATimeoutStopsABusyLoopSoon`, `TestCancelReachesAFinishedBodysChildren`, `tests/eszamanli_test.tr`'nin 10.7 testleri].
- **Açık:** 1.6'nın "durdurma istendi" hatasının kendi kodu (zaman aşımı ve iptalinki gibi). Adlı ayırma (yeni ad) Sonraya.

### 10.8

- **Mevcut durum:** paylaşılan listeye görevlerden ekleme sırası zamana bağlı ve her koşuda değişken (ör. `0..5` görevinde beş koşuda `[4, 2, 0, 1, 3]`, `[4, 0, 1, 2, 3]`, `[2, 1, 0, 3, 4]` ...) [ölçüldü: `0..5` döngüsünde `TR_async { l.TR_append(i) }` ile ardışık çalıştırma, `b77644dd`].

### 10.9

- **Mevcut durum (D1 dilim 3, 2026-11-02):** `g.TR_cancel()` görevi 1.5'in güvenli noktalarında durduruyor (fonksiyon girişi, döngü turu, süren beklemeler; durum değiştiren yerleşik biter). İptal görevin içinde `TR_catch` ile yakalanmıyor, ertelenen işleri çalışıyor. İptal başlattığı bitmemiş görevlere geçiyor, görevin gövdesi bitmiş ve alt görevlerini bekliyorken de; bitmiş alt görevler liste dolunca budanıyor. Görev gövdesi bitince (hatasız da) önce alt görevlerini bekliyor, hatayla ya da iptalle bittiyse önce onları iptal ediyor, sonra ertelenenleri çalışıyor. Bekleyen görev durunca iptal hatasını (H2920) alıyor, `g.TR_done()` ancak o zaman `TR_true`. Bitmiş görevi iptal etmek ve ikinci iptal etkisiz; iptal görev hatası olarak yazılmıyor (10.6). Ağır turlu döngüde iptal bir sonraki turda fark ediliyor (10.7'nin ölçümü) [test: `TestACancelledTaskStopsAtItsSafePoint`, `TestCancelReachesTheTasksChildren`, `TestDefersKeepTheirErrorsWhileUnwinding`, `TestCancelReachesAFinishedBodysChildren`, `TestAFinishedChildIsNotKept`, `TestATimeoutStopsABusyLoopSoon`, `tests/eszamanli_test.tr`'nin 10.9 testi].
- **Yapılacaklar:** testi olmayan üç kısım: durum değiştiren yerleşiğin iptalde bitmesi, alt görevi bekleyen başka görev, `TR_handle_stop`.
- **Gemini Koşu 4'ün yedi maddesi (D1 dilim 3):** iptalin durmasını bekleme, alt görevler ve ertelenenler (10.9), `TR_wait_all`'da `TR_suppressed` (10.5), 10.6'nın yazma anı ve 9.4'ün durdurmada ertelenen hatası uygulandı. Kalan iki: 9.1'in son deyim olan `TR_defer` denetim hatası (denetleyicinin işi, bu dilimin yetkisi dışında) ve 5.4'ün bileşik atamadaki değerlendirme sırası (görevlerden bağımsız yorumlayıcı işi).

## 11. Modüller

Kurallar: [`docs/spec/11_moduller.md`](spec/11_moduller.md). **Kapandı** (2026-10-29): ana oturum CLAUDE.md'den türeterek yeniden yazdı, iki tur DENETİM; kararlar Gemini Koşu 5'ten geçti (DENETİM doğruladı, `d31c9226`). Kitaplık modüllerinin İngilizce yolları (11.6) onaylandı (2026-10-29): `tr/layers`, `tr/optimizers`, `tr/pretrained`, `tr/training`.

### 11.1

- **Mevcut durum:** modülün en dışındaki deyim denetim hatası (H1107, 1.1); olumsuz fikstür `tests/negative/modul_en_dis.tr` [ölçüldü: `TestModuleTopLevelHoldsOnlyDefinitions`, 4a1 ilk dilim].
- **Yapılacaklar:** yok (1.1'e bağlı kalanlar 1.1'in maddesinde).
- **Gemini Koşu 5 (2026-10-29), uygulanacak (B, Ö1; ölçüm status'un):** göreli modül yolunun programın klasörüne yedek yolu kalkar; harf büyüklüğü diskteki adla aynı olmalı.

### 11.2

- **Mevcut durum:** köşeli ayraçla okuma ve atama (`m["PI"]`, `m["PI"] = 9`) takma adda denetimde, çalışma anının koduyla (H2904), ileti `m.PI`'yi öneriyor; indeks ad gibi yazılmış bir metin değilse (`m[k]`) ve tipi bilinmeyen kopyada çalışma anında [ölçüldü: `TestModuleValueRules`, `tests/negative/modul_koseli_ayrac.tr`, `tests/moduller_test.tr`'nin 11.2 testi].
- **Mevcut durum:** `TR_as` bir `TR_Dict` bağlıyor; modülde olmayan bir ad (m.yok) denetimden geçiyor ve çalışırken `TR_none` veriyor [ölçüldü: main `a9a1ba14`].
- **Yapılacaklar:** modül değeri; adların tipi denetimde; köşeli ayraçla erişim (çalışma anının kodu H2904), olmayan ad ve modüle özel ad (`m._gizli`, 11.7; bugün çalışma hatası) denetim hatası, en yakın adı öneren ileti; olumsuz fikstür. `tests/moduller_test.tr`'den yalnız sözlüğe özgü doğrulamalar (tip, uzunluk, anahtarlar, köşeli ayraç) çıkarıldı; nokta ile erişim ve modüle özel adın hata kodu kaldı.

### 11.3

- **Mevcut durum:** modüller program çalışmadan yükleniyor; `TR_import` yalnız dosya başında yazılabilir (H0024, `tests/negative/aktarma_yeri.tr`), görevlerin modül yüklemesi yok [ölçüldü: main `85a19c36`, fonksiyon içinde `TR_import`].
- **Mevcut durum (paylaşılan değer):** modül değerinin adı (`m.sayaç`) modülün değişkeninin bugünkü değerini veriyor; modülün kendi fonksiyonu değişkeni değiştirince her takma ad yeni değeri görüyor, eskiden yüklemedeki kopyayı veriyordu [ölçüldü: dal `claude/d1-dilim1-bf7baf`, `TestModuleValueReadsTheModulesVariable`, `tests/moduller_test.tr`'nin 11.3 testi]. Takma adsız aktarılan ad (`TR_import "m"` sonra `sayaç`) da modülün değişkenine bağlı: `say()` iki kez çağrılınca `sayaç` 2 (en dışta, fonksiyonda ve adı aktaran bir modülden geçince de); eskiden yüklemedeki kopyaydı, 0 veriyordu. Bu ada aktarandan atama çalışma anında H2901 [ölçüldü: dal `claude/d1-dilim2-c89bbc`, `TestImportedNameReadsTheModulesVariable`, `tests/moduller_test.tr`'nin 11.3 testi].

### 11.4

- **Mevcut durum:** aktarılan modülün tip hatası program çalışmadan önce verilir, hiçbir satır çalışmaz; fikstür `tests/negative/hatali_modul_aktar.tr` (RUN-ERROR H1001 etiketiyle; fikstür sınaması modülü çalışmadan önce denetler ve hatadan önce çıktı yazılmadığına bakar) [ölçüldü: main `85a19c36`, denetim ve çalıştırma; `TestNegativeFixtures`].

### 11.5

- **Mevcut durum (Faz 1, 2026-11-01):** görevden çağrılan modül fonksiyonu göreli yolu kendi modülünün klasörüyle okuyor (`tests/moduller_test.tr`: "11.5 görevde çalışan modül fonksiyonu göreli yolu kendi modülünün klasörüyle okur"); bu test çağrının dosya bilgisini (`callInSource`) sınıyor, görevin kendi dosya bilgisini sınamıyor: `NewTaskVM`'in `source` alanı boşaltılınca da 12 testin 12'si geçiyor [ölçüldü: DENETİM, 2026-11-01, `kol-denetim-gemini-faz1-paralel-sonuc.md`].
- **Yapılacaklar:** görev bloğu modülün içinde göreli okuma yapınca kendi modülünün klasörünü kullanmalı; TR testi, görevin dosya bilgisi bozulunca düşen.

### 11.6

- **Mevcut durum (Faz 0.2, 2026-10-29):** kitaplığın dört modülü İngilizce yollarda: `tr/layers`, `tr/optimizers`, `tr/pretrained`, `tr/training` (`lib/`); eski Türkçe yol H2478 verir (`tests/negative/kitaplik_turkce_harf.tr`). Eski yollar göçten önce 90 dosyada 976 satırda geçiyordu; şimdi `docs/archive/` ve o negatif test dışında yalnız gerekçenin tarihçe cümlesinde (`docs/spec/rationale/11_modules.md:34`) ve iki Go testinin yapay `tr/katmanx` adlarında (`internal/devtools/codeindex/snippet_test.go:106,111`, `tools/codegraph/library_imports_test.go:25,27,29`) geçiyor [ölçüldü: `git grep -nE 'tr/(katman|iyilestirici|egitim|hazir_modeller)' -- ':!docs/archive/*'`, main `c39e0b56`].

### 11.8

- **Mevcut durum:** iki aktarım aynı adı getirince sonraki sessizce kazanıyor; dosyanın kendi tanımı aktarılan adı sessizce gölgeliyor [ölçüldü: main `a9a1ba14`]. Depodaki 135 dosyanın 50 aktarmasında çakışma yok; kural göçsüz girer [ölçüldü: DENETİM 11-1].
- **Mevcut durum (atama):** takma adsız aktarılan ada atama (`DEGER = 7`) artık çalışırken hata veriyor, yerel kopya kurulmuyor [ölçüldü: DENETİM istek 61, `30b44b01`; önceden sessiz yerel kopya, DENETİM 11-1]; denetimde hata değil (D). Modül değerinin adına atama (`m.PI = 7`, `m.sayaç += 1`, fonksiyondan gelen modül değerinde de) çalışma anında H2901, modülün değeri değişmiyor; değerin içi (`m.yer.x = 1`, `m.kayıt.TR_append(x)`) ve modülün kendi fonksiyonunun kendi değişkenini değiştirmesi geçerli [ölçüldü: dal `claude/d1-dilim1-bf7baf`, `tests/moduller_test.tr`'nin 11.8 testi, `TestModuleNameIsNotReboundFromTheImporter`, `tests/negative/modul_adina_atama.tr`]. Köşeli ayraçla atama (`m["PI"] = 9`) 11.2'nin kuralıyla H2904.
- **4a2 (ikinci dilim):** takma adın modül değerinin adına atama (`m.PI = 7`, `m.sayaç += 1`) denetimde, çalışma anının koduyla (H2901); değerin içi (`m.yer.x = 1`) geçerli; fonksiyondan gelen ya da kopyalanan modül değeri çalışma anında [ölçüldü: `TestModuleValueRules`, `tests/negative/modul_adina_atama.tr`].
- **Yapılacaklar (A, Ö1, 4a5; ölçüldü 30b44b01):** denetim hataları (çakışma, aktarılan ada atama), iki yeri gösteren ve `TR_as`'ı öneren ileti; olumsuz fikstürler.

## 12. Testler

Kurallar: [`docs/spec/12_testler.md`](spec/12_testler.md). Taslak (2026-10-29): ana oturum CLAUDE.md'den türeterek yeniden yazdı; metin DENETİM 12-1'den geçti, bulguları işlendi (`913fd2a6`); kararlar Gemini Koşu 7'den (`59efe468`) ve 12.4a Koşu 3'ten (`0edb6f20`) geçti. Kapanış turu (işlenmiş metnin yeniden okunması) yapılmadı. Yeni ad `TR_assert_close`'un gösterim adı onaylandı (yakın_olmalı); kodu uygulandığı gün kayda girer (17.4).

### 12.1

- **Yapılacaklar:** test dosyasının tanımı (1.1'in en dış kuralı ona dayanıyor): rehber adı `_test.tr` ile biten dosya diyor, ama `tests/yardimci_testli.tr` de test bloğu içeriyor [kanıtlı: DENETİM 12-1]. Testin hangi cihazda koştuğu ve cihazlar arası tolerans 15'in E parçasında.
- **Gemini Koşu 7 (2026-10-29), uygulanacak:** test dosyası tanımı (göç işi bitti: en dışta deyim içeren dosya kalmadı, 4a1 ilk dilim) [ölçüldü: main `85a19c36`, iki dosyanın denetimi].

### 12.2

- **Mevcut durum:** aynı dosyada aynı adlı iki test ikisi de çalışıyor [ölçüldü: main `e6392066`].
- **Yapılacaklar:** sözdizimi hatası ve olumsuz fikstür.

### 12.3

- **Mevcut durum:** tensör eşitliği doğrulamada tek mantık değeriyle geçiyor [ölçüldü].
- **Yapılacaklar:** 15'in öğe öğe eşitliğiyle birlikte denetim hatası ve ileti.

### 12.4a

- **Mevcut durum:** toleranslı karşılaştırma yok; ondalık kesin eşitlikle kalıyor (0.1 + 0.2 ile 0.3) [ölçüldü].
- **Yapılacaklar:** `TR_assert_close`, hassasiyete göre varsayılan toleranslar; TR testi.
- **Gemini Koşu 3 (2026-10-29), uygulanacak:** sonsuz öğe yalnız aynı işaretli sonsuza yakındır (15.18a ile aynı).
- **Gemini Koşu 7 (2026-10-29), uygulanacak:** sayı ile 0 boyutlu tensör, tek tolerans argümanı, isteğe bağlı mesaj.

### 12.3, 12.4

- **Mevcut durum:** `TR_assert`'in dönüşü 13.10'un status'unda. Tensör koşullu `TR_assert` 20 satır, 0 boyutlu tensör koşulu 4 satır (`tests/operatorler_test.tr:134-137`) [ölçüldü: DENETİM].

### 12.5

- **Mevcut durum:** en dıştaki tanımlar testler arasında paylaşılıyor: bir testin en dıştaki değişkene atadığı değer sonraki testte görünüyor [ölçüldü: main `85a19c36`, en dıştaki sayaç ve onu artıran fonksiyonla]; aktarılan modülün durumu da sızıyor [ölçüldü: DENETİM]. Tohumsuz rastgelelik T1'den (20.17) beri koşudan koşuya aynı, ama paylaşılan üreteç testler arasında paylaşılıyor: ikinci test birincinin kaldığı yerden çekiyor, tek başına koşulan bir test başka sayı alıyor; 12'nin "her test programın başlangıç durumunda başlar" kuralına aykırı [ölçüldü: DENETİM, T1'in birleşik dalı, iki testin `TR_randn([2])`'si `[-1.0469432, -1.7477671]` ve `[0.32232177, 0.13958974]`].
- **4a1 ilk dilim (2026-10-30):** test dosyasının en dışındaki deyim ve sabit olmayan ilk değer denetim hatası (H1107, H1108); depodaki yerler (1 deyim, geçici klasör ve komut satırı ayarı) kurulum fonksiyonuna alındı [ölçüldü: `TestRepoModulesHoldOnlyDefinitions`].
- **Yapılacaklar (C, Ö2; ölçüm status'un):** en dıştaki tanımların her test için yeniden kurulması; her testin üreteci aynı durumda başlatması (20.17'nin sabit başlangıç durumundan sonra); TR testleri.
- **Gemini Koşu 7 (2026-10-29), uygulanacak:** hatalı test bitmemiş görevlerini önce iptal eder (10.9); bugün sonsuz işçiyle asılı kalıyor.

## 13. Yerleşikler

Kurallar: [`docs/spec/13_yerlesikler.md`](spec/13_yerlesikler.md). Taslak, 7. tur (2026-10-28): ana oturum yazdı; dört turu DENETİM, 1. turu ana oturum, 5. turu (dokuz iş) ikisi, 6. turu Gemini'nin araştırması (DENETİM doğruladı: 7 onay, 5 koşullu) inceledi; bulgular işlendi. Sonra Gemini Koşu 6 (`c784c47a`) ve Koşu 3'ün 13.1 ile 13.10 bulguları (`0edb6f20`) işlendi (2026-10-29). Uygulama 6. ve 7. turun kararlarını henüz almadı: ayırıcısız `TR_split`, `TR_std`'nin n−1'i, telafili toplama, `TR_split("")`'in hatası, JSON'un yinelenen anahtarı, yarımı çifte yuvarlama, `TR_median([1.0, 2.0, TR_nan])` (bugün 1.0), sıfırın işareti (`TR_max(-0.0, 0.0)` sıraya bağlı), yazma yolunun programın klasörüne düşmesi, `TR_clamp(5)`'in iletisindeki eski kimlik, fonksiyon yerine verilen tipin iletisi eski davranışta [ölçüldü: DENETİM]. `tests/yerlesikler_test.tr` eski kuralları da sınıyor (kazanan değerin tipi, sayı hücreli CSV, `TR_mean`'in tek tek değerleri); kural uygulanırken yeniden yazılır.
- **Yapılacaklar (T1 birleşmesi, 2026-11-02):** test koşucusu her testten önce paylaşılan üreteci başlangıç durumuna (`"pcg:00000000000054520000000000002017"`) koysun; test koşucusunun (12) işi [kanıtlı: DENETİM, `kol-denetim-istek20-sonuc.md` C].
### 13.1

- **Mevcut durum:** `TR_max` kazanan değerin tipini veriyor: `TR_max(2, 1.5)` `TR_Int` 2, `TR_max(1, 1.5)` `TR_Float` 1.5; metin argüman hata; `TR_max([])` `TR_none`; `TR_max(1)` 1 veriyor; `TR_mean(1, 2)` 1.5 (depoda 2 yer); `TR_round(5, 2)` `TR_Float` 5.0, `TR_round(3.7, 0)` `TR_Int` 4, `TR_round(TR_nan)` hata; `TR_print` yazdığı metni, `TR_sleep(0)` ve `TR_wait_all()` `TR_true` döndürüyor; boş listede `TR_mean`, `TR_std`, `TR_median` hata [ölçüldü: DENETİM].
- **Mevcut durum:** 13.1'in −0.0 kuralı uygulandı (`TR_max([-0.0, 0.0])` → `0.0`, `TR_min([0.0, -0.0])` → `-0.0`) [ölçüldü: DENETİM istek 71]; telafili toplama (Neumaier) ve tek öğede `TR_std`'nin hatası uygulanmadı [ölçüldü: DENETİM C1]. Tensörde `TR_round`'un `TR_Float` dönüşü 15.16a'nın status'unda (15'in yan değişikliği, uygulanmadı).
- **Yapılacaklar (A, Ö0, Kol A 13.1 diliminden hemen sonra; ölçüldü: DENETİM istek 71, iki ikili):** sonucun tipi ya da değeri dil tanımından ayrı, hatasız: `TR_max(2, 1.5)` `2` (`TR_Int`; dil tanımı `2.0`), `TR_max([])` `TR_none` (dil tanımı hata), `TR_round(5, 2)` `5.0` (dil tanımı `5`, girdinin tipi), `TR_round(3.7, 0)` `4` (`TR_Int`; dil tanımı `4.0`).
- **Yapılacaklar (A, Ö0, Kol A 4a4'ten sonra; ölçüldü 30b44b01):** `TR_std` bugün n'ye bölüyor; ortalama ve sapma yolları (`interp/stats.go:38`) FMA'ya birleşebiliyor, `math.Exp` amd64'te işlemciye göre yol seçiyor [kanıtlı: DENETİM]; ikili veri: `TR_write_file` `TR_Bytes`'ın metnini yazıyor ("Bytes([0, 255, 128])", 20 karakter), `TR_read_bytes` yok, `TR_read_file` geçersiz UTF-8'i sessizce okuyor [ölçüldü]; düz ondalık toplam sıraya bağlı (`TR_sum([1e16, 1.0, -1e16])` 0.0, başka sırada 1.0) [ölçüldü]; `TR_time_ms` `time.Now().UnixMilli()`, `TR_monotonic_ns` yok [kanıtlı]; `TR_print` boşlukla birleştiriyor [ölçüldü]; karışık sayıların `TR_Float`'ı, metnin sıralanabilir sayılması, boş listede `TR_min`/`TR_max`'ın hatası, tek argümanın hatası, `TR_mean`/`TR_std`/`TR_median`'ın yalnız liste alması, `TR_round`'un üç imzası, değer döndürmeyen üç yerleşik.
- **Gemini Koşu 3 (2026-10-29), uygulanacak:** `TR_min` ve `TR_max`'ın anahtar fonksiyonlu biçimi sıralama sırasını kullanır; `TR_sum` tensör listesinde boşta hata (15.18).

### 13.2

- **Mevcut durum:** ayırıcısız `TR_split()` tek ASCII boşluğa bölüyor, boş öğe veriyor, sekmede ve satır sonunda bölmüyor [ölçüldü: DENETİM]; harf dönüşümü basit eşleme: `"ß".TR_upper()` → "ß", `"İSTANBUL".TR_lower().TR_length` 8 [ölçüldü: DENETİM].
- **Yapılacaklar:** ayırıcısız `TR_split()`'in Unicode boşluk dizilerine bölmesi.
- **Gemini Koşu 6 (2026-10-29), uygulanacak:** boş alt metin (motor bugün `"ab".TR_replace("", "x")`'te "xaxbx" veriyor [ölçüldü: DENETİM]; kurala göre hata) ve negatif indeks; `.TR_index_of` metinde henüz yok.

### 13.3

- **Mevcut durum:** 13.10'un sıralama sırası uygulandı (−0.0 0.0'dan önce, NaN'lar sonda ve birbirine eşit, demette öğe öğe); anahtarlı sıralama `.TR_sort(f)` motorda: anahtar her öğe için bir kez çağrılır, sıra kararlı, iki parametreli karşılaştırıcı H2900, sıralanamayan anahtar H2902 ve H2903, asıl listeyi değiştiren anahtar H2703 [ölçüldü: dal `claude/d1-dilim1-bf7baf`, `tests/yerlesikler_test.tr`'nin 13.3 testleri, `interp/sorting_test.go`].
- **Yapılacaklar:** denetimin anahtar fonksiyonunun parametre sayısını (H2900) ve anahtarın tipini denetlemesi (4a2, denetleyici).

### 13.6

- **Mevcut durum:** `TR_reduce(0, (a, x) => a + x)` çalışıyor; başa yazılan ve tek parametreli fonksiyon hata, iletiler (H2475, H2476) ok yazımını öneriyor [ölçüldü: main `85a19c36`, `l.TR_reduce((a, x) => a + x, 0)` ve `l.TR_reduce(0, (a) => a)` iletileri; `tests/fonksiyonlar_test.tr:143`, `tests/yerlesikler_test.tr:204` kodları sınıyor].
- **Yapılacaklar:** iletilerin metnini sınayan test (kodlar sınanıyor, metinler `internal/suggestion_messages_test.go` tablosunda yok); başlangıçsız `TR_reduce`'un boş listede hatası; `.TR_append`'in değer döndürmemesi (bugün listenin kendisi).

### 13.7

- **Mevcut durum:** `TR_is_nan` yok (bilinmeyen kimlik); tensörde `TR_is_finite` float32 0/1 veriyor (15.36); `TR_where`'in sayı imzası var: `TR_where(TR_true, 2, 3)` 2 veriyor [ölçüldü: main `8b115692`] (15'in yan değişikliği, uygulanmadı).
- **Yapılacaklar:** `TR_is_nan`'ın sayı ve tensör imzası; tensörde `TR_Tensor[TR_Bool]` dönüşü; `TR_where`'in sayı imzasının kalkması, göçü 15.15'in status'unda (`tests/matematik_test.tr:170-171`; `:169` skaler `TR_Bool` koşul + tensör dalı) [kanıtlı: `tests/matematik_test.tr:169-171`]; TR testi.

### 13.8

- **Mevcut durum:** JSON'a NaN yazmak hata (H7008) [ölçüldü]; `TR_json_parse` `TR_Dict` veriyor; `TR_json_string({1: 2})` anahtarı sessizce metne çeviriyor (`{"1":2}`); CSV hücresi sayıya benziyorsa sayı (`[["007", 7]]`) [ölçüldü].
- **Yapılacaklar (A, Ö1, 4a5; ölçüldü 30b44b01):** `TR_json_parse` `TR_JSON` verir (3.7a); metin olmayan anahtarın hatası; CSV hücresi hep metin (göç: `.tr`'de 10 kullanım).
- **Gemini Koşu 6 (2026-10-29), uygulanacak:** `TR_json_string`'de yol ve tensörün yazımı; `TR_json_parse[T]`'de `null`, iç içe yapı ve tensör alanının hatası.

### 13.8a

- **Mevcut durum:** `t.TR_tolist` tensörün boyutuna göre liste ya da sayı, `k.TR_decode` toplu girişte liste, `TR_parameters(x, TR_named: TR_true)` sözlük veriyor [ölçüldü: DENETİM].
- **Mevcut durum (T1 dilim 2):** `t.TR_item()` var: tek öğeli tensörün öğesi, öğe tipi tensörün cinsi; başka tensörde H3068 [ölçüldü: `tests/tensor_test.tr`].
- **Yapılacaklar:** `TR_batch_decode`, `TR_named_parameters` (yeni kimlikler, 4a); `TR_item`'ın iletisi (H3068'in metni "Item" diyor, `.TR_item()`'ı adıyla anmıyor); `TR_tolist`'in yalnız 1 boyutta, öğe tipi tensörün cinsi olan liste vermesi, 0 boyutta hatası (15'in yan değişikliği, uygulanmadı); `TR_named_parameters`'ın `TR_Dict[TR_String, TR_AnyTensor]` dönüşü (aynı) (argmax'in sonucunu indeks olarak kullanan kod `TR_Int(t.TR_item)` yazar, göçte sayılır). Dil tanımı her yerde çağrıyla yazıyor (`t.TR_tolist()`, on yer), motorda bir özellik: `t.TR_tolist()` "Liste tipi bir fonksiyon olarak çağrılamaz" veriyor, `lib/` ve `tests/`'te 456 çağrısız yazım var [ölçüldü: DENETİM, `kol-denetim-istek21-sonuc.md` 3]; motor dil tanımına uyar, yazımlar göçle değişir.

### 13.9

- **Mevcut durum:** katalog 188 fonksiyon ve metottan 138 Go yerleşiğinin imzasını tutuyor; 50'si TR ile yazılmış kitaplıkta (29 fonksiyon, 21 metot), imzası kaynakta. Tipler öğesiz (`TR_List`, `TR_Dict`), fonksiyon tipi tek bir `TR_Function`, yerleşiğin tipi `TR_BuiltinFunction` (11 yerde), genel tip `TR_Any` 24 yerde [ölçüldü: DENETİM].
- **4a2 (ilk dilim):** denetleyici imzaları okuyor: tek tipli dönüş çağrının tipi (`TR_BuiltinFunction` fonksiyon tipi sayılıyor), tipi bilinen alıcıda metodun varlığı (`types`) ve çekirdek metotların argüman sayısı (`params`). Tanımın motorla uyuştuğu sınanıyor: dönüş `TestSingleReturnsMatchEngine` (97 kod doğrulandı; `TR_json_parse` uyuşmuyor: tanım `TR_JSON`, motor `TR_Dict`, 13.8), üye tipleri `TestMemberTypesMatchDefinitions` (`.TR_get`, `.TR_keys`, `.TR_length` `TR_JSON`/`TR_Bytes`'ı eksik yazıyor), sayı `TestMethodCountsMatchDefinitions`.
- **4a2 (ikinci dilim):** tipler kimlik yazımıyla (`TR_Int`, `TR_List[T]`, `T?`; sınıflı yazım hata) ve dilin tip ayrıştırıcısıyla okunuyor (`parser.ParseTypeText`) [test: `TestDefinitionTypesParse`]. Öğe tipli dönüş yazan üyeler: `.TR_first`, `.TR_last`, `.TR_get` (liste `T?`, sözlük `V?`), `.TR_filter`, `.TR_sort`, `.TR_unique`, `.TR_reverse` (liste `TR_List[T]`, aralık `TR_List[TR_Int]`), `.TR_keys`, `.TR_values`, `.TR_split` (metin); depodaki ilk değerlerden tipi bilinmeyen 626 → 623 [ölçüldü: `TestRepoFirstValueTypes`, `70b8e085` ve bu dilim]. Parametreler ve fonksiyon tipleri henüz öğesiz.
- **Mevcut durum (4a4, 2026-11-03):** denetleyici fonksiyon parametresinin `A|B|C` tipini imza seçenekleri olarak okuyor (`TR_NoneType` seçeneği isteğe bağlılık); 39 fonksiyonun seçenekleri ve seçenek dışı tipe kodu motorla aynı [test: `TestFunctionParamOptionsMatchEngine`]. Seçenekleri motorla uyuşmayan ya da kodunun parametreleri denetimde doldurulamayan fonksiyonlar denetlenmiyor (`TR_clamp`, `TR_concat`, `TR_argmax` gibi; listesi 4a4'ün DEVİR'inde).
- **Yapılacaklar:** imzalar öğe ve fonksiyon tipleriyle; `TR_BuiltinFunction` ve `TR_Any` kalkar; metot parametresinin birleşim seçenekleri; dönüşün seçilen imzadan gelmesi (`TR_abs(1)` `TR_Int`, 4a5).
- **Gemini Koşu 6 (2026-10-29), uygulanacak:** yerleşiğin fonksiyon değeri olarak eşleşmesi (isteğe bağlı parametreler boş, adlı argümanlar varsayılanda; varsayılansız adlı argümanda hata). Denetleyicinin işi (4a2).

### 13.10

- **Mevcut durum:** 40 yerleşiğin dönüş tipi birleşim (`TR_abs`, `TR_floor`, `TR_exp` gibi; 36 fonksiyon ve `.TR_decode`, `.TR_grad`, `.TR_token_id`, `.TR_tolist`); dönüşü argümanın değerine bağlı üç yer: `TR_tolist` (tensörün boyutu), `TR_decode` (toplu giriş), `TR_parameters` (`TR_named:`); 8 yerleşik değişken sayıda argüman alıyor; yalnız 7 metodun alıcıya göre ayrı imzası var [ölçüldü: main `85a19c36`, `definitions.json`'dan sayıldı].
- **Yapılacaklar:** sekiz kısıt ("number", "ordered", "equatable", "key", "json", "struct", "function", "tensor_data"); `TR_command_line` ve `TR_Checkpoint`'in yapıyla yazımı (göç: 18 ve 54 satır, `.TR_state` 39); anlamsız `TR_Bool` dönüşlerinin kalkması, tek bir 4a işi: `TR_write_file`, `TR_assert` (`internal/lang/interp/test_support.go`), `.TR_load_state`, `TR_sleep`, `TR_wait_all` (argümanlı biçimi liste verir, 10.5), 19'daki `TR_Logger.TR_append` ve `TR_Progress.TR_update` gibi; `tr.json`'un açıklamalarındaki "doğru döndürür" aynı commit'te değişir [ölçüldü: DENETİM 10-2, 10-3]; `TR_topk`'nın iki değeri (6.3, 15'te), `TR_assert`'in iletisinin `TR_String` ya da `TR_Message` olması (12'de); kısıtlı tip parametreleri, ayrık imzalar, değere bağlı dönüşlerin sabit dönüşlü yerleşiklere ayrılması (15 ve 16. bölümlerle), değişken sayıda argümanın yazımı (4a).
- **Mevcut durum (15'in yan değişiklikleri, uygulanmadı):** tensör verisi kısıtının cinse bağlanması, eşitlenebilirin tensörü dışlaması, varsayılansız adlı argüman, `TR_save_tensors`'un kabul kümesi, `TR_load_tensors` ve `.TR_narrow[E]()`'nin imzaları, `TR_quantized`'ın imzası, `TR_custom_grad`'ın dönüşü ve `TR_grad_check`'in değer döndürmemesi (15.32'nin status'unda) imzalarda yok.
- **Gemini Koşu 6 (2026-10-29), uygulanacak:** karşılaştıran yerleşiklerin kesin karşılaştırması (2⁵³ üstü).
- **Değeri kullanılmayan çağrı (4a2, ilk dilim):** uygulandı. Tanım dosyasında `pure` (39 yerleşik: 13.2–13.4'ün yeni değer veren metotları, 13.7'nin hesapları); çağrısı tek başına deyimse H1115, ileti atamayı önerir, alıcı atanabilir adsa uygulanabilir düzeltme [ölçüldü: `TestPureCallValueUnused`, `TestPureMarksOnlyValueBuiltins`, öneri tablosu]. Tensör işlemlerinin işareti ve 13.1'in hesapları ikinci dilimde.

### 13.11

- **Mevcut durum:** yanlış tip ve sayı çalışma anında, iletiyle: "sqrt() expects Int or Float as argument 1, String (\"a\") given", "sqrt() takes 1 argument, 2 given (usage: sqrt(number))" [ölçüldü]. 4a2 (ilk dilim): tipi bilinen alıcıda (Metin, Liste, Sözlük, Aralık) yerleşik metodun konumlu argüman sayısı denetimde de, aynı kodla (H2011); metodun sayı iletisi çalışma anında da denetimde de kimlikle: ".TR_sort() takes 0 arguments, 1 given (usage: `.TR_sort()`)", `.TR_sort()`'a anahtar verilince demet sıralamasının yolu (H1116) [ölçüldü: `TestMemberArgCount`, `TestMemberArgCountUsesIdentities`, `TestMethodCountsMatchDefinitions`]. 4a2 (ikinci dilim): tipi bilinen çekirdek alıcıda metodun tek tipli parametresine yanlış tipte argüman denetimde de, aynı kodla (H2423), metot kimlikle (".TR_split() expects TR_String as argument 1, TR_Int given"); parametre tiplerinin motorun denetlediği tiple aynı olduğu sınanıyor (11 metot-alıcı; `.TR_reduce` motor denetlemediği için dışarıda) [ölçüldü: `TestMemberArgType`, `TestParamsMatchEngineKinds`, `tests/negative/metot_arguman_tipi.tr`]. Tipi bilinen alıcının özelliğini çağırmak (`[1].TR_length(1)`) değeri fonksiyon olmayan tek tipliyse denetimde de çalışma anının koduyla (H2421); ileti bugün yalnız tipi söylüyor ("Tam tipi bir fonksiyon olarak çağrılamaz"), özelliği ve parantezsiz yazımı söylemiyor [ölçüldü: `TestPropertyCall`, `tests/negative/ozellik_cagrisi.tr`].
- **Mevcut durum (öneri yuvası):** H2421 (özelliğin çağrılması) ve H2423 (yanlış argüman tipi) bir `hint` parametresi taşıyor; çalışma anı onu boş geçiriyor [kanıtlı: `interp/calls.go:221`, `interp/arg_errors.go:97`]. 4a4: denetleyicide H2423'ün ipucu açılmamış değerin yazımı (H1130, `l.TR_first ?? 0`), metotta `hint.MemberArgType`'ın ipucu parametresiyle, yerleşik fonksiyonda `builtin_args.go` ile [test: `TestUnopenedOptionalHint`]; öbür doğru yazım önerileri (13.9) yok.
- **Mevcut durum (4a4, 2026-11-03):** 39 yerleşik fonksiyonun konumlu argümanı seçeneklerinden hiçbirine uymazsa denetim hatası, çalışma anının fonksiyona ve verilen tipe göre kodu: `TR_abs("s")` H2423, `TR_sin("s")` H6103, `TR_zeros("a")` H6021, `TR_abs(l.TR_first)` H2423. Liste ve Tam verilen argüman çalışma anının önerili koduna (H6104, H6020) kalıyor [test: `TestBuiltinArgOptions`, `TestFunctionParamOptionsMatchEngine`, `tests/negative/yerlesik_arguman_tipi.tr`].
- **Yapılacaklar:** öbür fonksiyonların argüman tipleri (seçenekleri ya da kodları motorla uyuşmayanlar); H6104 ve H6020'nin önerisi denetimde kurulabilirse liste ve Tam argüman; çekirdek olmayan alıcıların metotları; fonksiyonların ve öbür alıcı tiplerinin (tensör, kitaplık tipleri) sayı denetimi; fonksiyonların H2011 iletisi hâlâ görünüm adıyla (`TR_round` için "round()").

## 15. Tensörler ve otomatik türev

Kurallar: [`docs/spec/15_tensorler.md`](spec/15_tensorler.md). Bölüm 2026-10-29'da altı parçada (A–F) CLAUDE.md'den türetilerek yeniden yazıldı. Metin DENETİM'den (A–F, birleşme, commit öncesi), kararlar Gemini'nin taslak okumalarından (AB, ABC, TÜM) ve üç koşusundan geçti (Koşu 1 `4afda971`, Koşu 2 `70a1e6fe`, Koşu 3 `0edb6f20`; DENETİM doğruladı); aşağıda bugünkü motorun kurallardan ayrıldığı yerler ve göç.


**En büyük göç kalemleri** [ölçüldü: DENETİM, main `8b115692`]:
1. tam sayı değişmezli tensörler: 91 yer (`tests/` 77, `examples/` 11, `lib/` 3), çalışırken 20 dosyada 158 kurulum (15.1–15.3);
2. karşılaştırmanın mantık tensörü vermesi: kaba sayım ~40 yer, çoğu testte (`TR_where` 16, karşılaştırmanın `.TR_tolist` 16, aritmetikte 1, toplamda 1, maske 4) (15.6, 15.15);
3. `TR_grad_check`: `.tr` dosyalarında 40 çağrı, 34'ü dönüşünü kullanıyor (`31` `TR_assert(TR_grad_check(...))`, `1` `TR_assert_equal(TR_true, ...)` `tests/matematik_test.tr:114`, `2` `TR_print(...)` `examples/tr/27_ozel_turev.tr:23-24`; kalan `6`'sı `TR_error_code`/`TR_try` bloğunda deyim) [ölçüldü: `git grep -n 'TR_grad_check(' -- '*.tr'`, main `c39e0b56`] (15.32);
4. çapraz entropi ve gömmenin indeks cinsi: `.tr` dosyalarında 33 satırda 34 çağrı (25 `TR_cross_entropy` + 9 `TR_linear_cross_entropy`; `tests/tensor_test.tr:207`'de ikisi aynı satırda) + 24 satırda 27 `TR_embedding_lookup` çağrısı [ölçüldü: `git grep -n 'TR_cross_entropy(' -- '*.tr'`, `git grep -n 'TR_linear_cross_entropy(' -- '*.tr'` ve `git grep -n 'TR_embedding_lookup(' -- '*.tr'`, main `c39e0b56`] (15.23, 15.23a);
5. `TR_custom_grad` 22 ve `TR_no_grad(x)` 20 satır (15.27a, 15.28);
6. tensörde bileşik atamanın yerinde yazması: `.tr`'de ada bileşik atama 76 satır, çoğu sayaç; önce ve sonra koşup karşılaştırmak yeter (15.6d).

### 15.1

- **Mevcut durum:** `TR_Int` ("int64" … "uint8"), `TR_Bool` ve float64 depolaması var; argmax, sıralama indeksleri, örnekleme ve veri kümesi kimlikleri `TR_Int` ("int64") veriyor; cinsli işleçler (15.6'nın tablosu), `TR_sum`/`TR_mean`/`TR_max`/`TR_min`/`TR_cumsum`, `TR_where`, `TR_clamp` CPU'da [test: TestTypedStorageRoundTrip, TestTypedIntArith, TestTypedConvert; TR testi "15.1 TR_Int ve TR_Bool tensörü"]. GPU'da yalnız taşıma; öbür cinsli işlemler ve ondalık çekirdeğe giren cinsli tensör 15.48'in hassasiyet hatası (H6960) [test: TestTypedStorageOnGPU, TestTypedInGPUFloatKernelIsPrecisionError]. `TR_AnyTensor` ve `.TR_narrow[E]()` yok.
- **GPU (T1 dilim 2, adım 8):** cinsli depolama ve taşıma; argmax'in `TR_Int` sonucu cihazda; karşılaştırma (hesap ondalıkları, "int64", mantıkta `==`/`!=`) `TR_Bool` verir; mantıkta `& | ^ ~`; `TR_where` mantık koşulla; "int64" ve mantıktan float32'ye tip çağrısı cihazda; tam sayı içinde `.TR_to_dtype` ve tam sayıya tip çağrısı konakta yapılıp cihaza döner (bekleyen işlem, `TR_replay` kaydında çalışmaz); öbür cinsli işlem H6969 [test: TestTypedPathsOnGPU, TestTypedMaskPathsOnGPU, TestGPUGenerateWithTypedPositions]. Gerçek kart (G4, 2026-11-02) benzetimin geçirdiği iki hatayı yakaladı, ikisi de düzeldi: cinsli çekirdeklerin PTX `.entry` adı geçersizdi (`[`, boşluk; ad `2061d924`'te düzeldi, paket her çekirdeğin adını `f2280246`'dan beri denetliyor, benzetim geçersiz adı `ee5550bc`'den beri reddediyor) ve ham bayt depolamasının konak gölgesi yoktu, kimlik okumaları kuyruğu boşaltıyordu (hedef modelde +%6; `c1bbd329`, G4 ABAB taban 145 ↔ uç 146 ms/adım [ölçüldü]). Düzeltmelerden sonra GPU testlerinin listesi kartta yeniden koşulacak.
- **Açık küçükler (DENETİM istek 54 ve 56):** cinsli çekirdek bileşimlerinin (`gpuTypedOK`'un açtığı hepsi) ad, benzersizlik ve ayrışma testi; benzetimin `.target` denetimi (`mma.sync`, `cvt.rn.*x2.f32`); Colab'da bir kez ptxas karşılaştırması; `checkKernelNames` yalnız bütün testler geçince basıyor; benzetimde ns/op güvenilmez, `BenchmarkIDBatchToHost`'un ölçütü `eşzamanlama/op`. Veri kümesi kimlikleri float32 kurulup "int64"e çevriliyor (`dataset.go` → `ml.NewInt`): 2²⁴'ün üstündeki kimlik orada bozulur ve 32K öğe başına ~0,3 ms konak işi var (T1 bulgu 12) [ölçüldü: T1, benzetimde profil; 2²⁴ sınırı kanıtlı: float32'nin anlamlı bitleri].
- **Yapılacaklar:** öğe cinsli tip (`TR_Tensor[E]`, tip denetimi 4c), `TR_AnyTensor` ve daraltma; GPU'da cinsli tensörün genel çekirdekleri (2b); `TR_replay` kaydında tam sayı tensörünün kopyası ve tam sayı tensörünün sayıyla işlemi (sayı bugün konaktan yükleniyor).
- **Tip yazımı ayrışıyor (Faz 0.6a, 2026-10-29):** `TR_Tensor[E]` ayrıştırılıyor, `TR_Tensor`'a indirgeniyor; `TR_AnyTensor` henüz kod değil [ölçüldü: bulut 0.6a].

### 15.2

- **Mevcut durum:** cins çalışırken veriden çıkıyor: tam sayı ve aralık `TR_Int` ("int64", float32'den geçmez), mantık değeri `TR_Bool`, ondalıklı sayı verisi `TR_Float`; boş liste `TR_Float`. `TR_Tensor[E](veri)`, `TR_zeros[E]`, `TR_ones[E]`, `TR_eye[E]` cinsi seçiyor; `TR_randn` yalnız `TR_Float`. `TR_dtype:` cinsin bütün hassasiyetlerini alıyor; başka cinsin hassasiyeti H6976 (ileti tip argümanını öneriyor), sığmayan değer H6964, sayıyla mantık değeri H6975, tam sayı ve mantıkta türev H6973. H6954'ün ara kuralı kalktı [test: TestTensorKindFromData; TR testi "15.2 cins veriden çıkar", "15.2 tip argümanı öğe cinsini seçer"].
- **Mevcut durum (sınır):** cins çalışırken değerden çıkıyor, tip denetiminden değil. D1 dilim 3'ün 3.13'üyle birleşince `TR_Float` parametresine verilen tam sayı yazımı ondalık geliyor ve ondan kurulan tensör `TR_Float` (`p(2)` → `[2.0000]`) [ölçüldü: main `d24b95ea`]; tipi ilk değerinden çıkan değişken (`TR_var i = 1`) `TR_Int` kalıyor. Tip denetiminin önerisi (`TR_Tensor[TR_Float](kimlikler)`) 4c'de.
- **Göç (yapıldı, T1 dilim 2):** `.tr` dosyalarında 88 değişmez ve `lib/layers.tr:85` (`858007c8`), Go testlerinin TR metinlerinde 110 değişmez (`ac9f855b`), `TR_Float` parametresine tam sayı yazımı veren iki test (`tests/tensor_test.tr`, `TestParameters`) ondalık yazıma geçti; çıktılar önce ve sonra aynı [ölçüldü]. DENETİM'in saydığı 4 `TR_requires_grad: TR_true`'lu yerden (`a1d5b545`'te `tests/tensor_test.tr:891, 892, 901, 912`) üçü ondalığa geçti, `:912` boş liste (zaten `TR_Float`).
- **Gemini Koşu 3 (2026-10-29), uygulanacak:** `TR_zeros`, `TR_ones` ve `TR_eye`'ın mantık cinsi.
- **Gemini 15/2 (2026-10-29), uygulanacak:** `TR_zeros(..., TR_dtype: "q4")` gibi nicemlenmiş sıfır kurucu (T3).

### 15.3

- **Mevcut durum:** sürüm motorda var (depo başına sayaç, H3012) [kanıtlı: internal/compute/tensor/device.go:154-156, 166-181, `internal/compute/tensor/autograd.go:169-173, 226-227`]; ama ileti `TR_sigmoid`'in ezilen çıktısına "girdi" diyor [ölçüldü: DENETİM].
- **Mevcut durum (T1 dilim 2):** `.TR_to_dtype` cinsin bütün hassasiyetlerini alıyor ("int8", "float64" ...; tam sayıda sığma denetimi H6964), başka cinsin hassasiyeti H6977 (ileti tip çağrısını öneriyor). Cins dönüşümü tip çağrısı: `TR_Tensor[TR_Int](t)` sıfıra doğru kesiyor, NaN ve sonsuz H6964; `TR_Tensor[TR_Float](i, TR_dtype: "float64")` hedef hassasiyeti alıyor; mantık ile sayı arasında dönüşüm H6978. Tam sayı taşması H6961 (CPU). `.TR_tolist()` öğe tipli liste veriyor (`TR_List[TR_Int]`) [test: TestTypedConvert, TestTypedIntArith; TR testi "15.3 hassasiyet cinsin içinde değişir"]. Hesap ondalığından float64'e `.TR_to_dtype` türev izlemiyor [ölçüldü].
- **Yapılacaklar:**
  - `TR_wrapping_add`, `TR_wrapping_sub` ve `TR_wrapping_mul`'ın tensör imzası; PyTorch'un `.float()`/`.long()`/`.bool()`/`.to(...)` alışkanlığına doğru yazımı öneren ileti (bugün genel "özellik ya da metot bulunamadı");
  - GPU'da çekirdeklerin hata bildireceği bir cihaz bayrağı yok [kanıtlı: DENETİM]: taşmanın hatası için yeni düzenek ve işlemden hemen sonra okuma (15.6'nın bekleme listesi);
  - sürüm iletisinin girdi ile çıktıyı ayırması;
  - yaprak olmayan türevli tensöre yerinde yazmanın türevsiz kipte de hatası;
  - kopya `.TR_reshape` sonucunun yazma yasağı: depoda bir işaret, yazma girişinde tek dal [kanıtlı: DENETİM];
  - tensör koşulunun denetim hatası ve iletileri; TR testi.

### 15.4

- **Mevcut durum:** yeni görevler float32 ile başlıyor; `TR_logsumexp` yok, tensörde `TR_std` H2423 veriyor [ölçüldü].
- **Yapılacaklar:** yeni görevin açıldığı yerin hesap türünü ve türevsiz kipini alması; `TR_logsumexp`, `TR_std` ve `TR_cumsum`'ın blok kuralı.
- **Gemini Koşu 3 (2026-10-29), uygulanacak:** `TR_cumsum` 16 bitlik girdiyi blokta float32'ye açar.

### 15.5

- **Mevcut durum (T1 dilim 2):** `==`, `!=`, `<`, `>`, `<=`, `>=` tensörde öğe öğedir, yayınlar ve `TR_Tensor[TR_Bool]` verir, CPU'da ve GPU'da (GPU'da float32, "int64" ve mantıkta `==`/`!=`); `TR_none` ile karşılaştırma `TR_Bool` kalır; mantık tensörü koşulda H2600/H2612 verir, sessizce sayılmaz; TR testi `tests/matematik_test.tr` [ölçüldü]. `TR_fuse` kaydında karşılaştırma bugün 0/1 ondalık maskedir, `==`/`!=` orada H3107 (kaynaşmış çekirdekte mantık cinsi yok). Denetleyici tensör karşılaştırmasının tipini `TR_Bool` sayıyor: sonucunda `.TR_item()` denetim hatası. Metin dört basamaklı ve bütün öğeler yazılıyor [ölçüldü].
- **Yapılacaklar:** kaynaştır kaydında `TR_Bool` maske; denetleyicinin karşılaştırma tipi; eşitlenemeyen değerlerin denetim hatası, metin. Göç yapıldı (T1 dilim 2, 29 satır: .tr testlerinde 18 `0781f2fa`; Go testlerinde 9, kitaplıkta 1, örnekte 1 `80c0bfe0`). Göç [ölçüldü: DENETİM]: değişen 0 boyutlu tensörün sayıyla karşılaştırması (tests/operatorler_test.tr'de 6 çalışma); bütün tensör karşılaştırmasının 85 yeri ve `TR_assert_equal`'ın 141 kullanımı değişmez. Dolaşmada öğenin hep tensör olması yalnız 15.5'in kendi testini değiştirir: tests/tensor_test.tr:575, :577 [ölçüldü: DENETİM A2].
- **Gemini Koşu 3 (2026-10-29), uygulanacak:** isteğe bağlı tensör içeren her tipin `TR_none`'la karşılaştırması; gösterimde cinsin varsayılanı dışındaki hassasiyet.

### 15.6

- **Mevcut durum:** argmax'in tam sayılı işareti `+ 1`'de düşüyor (sonuç `2.0`, `TR_Float`); `/ 2` ve `* 2.5` tam sayılı tensörü ondalık yapıyor. `t // 2`, `t % 2` ve bit işleçleri H6703 veriyor; `@` yalnız ondalık [ölçüldü].
- **T1 dilim 2:** maskeler `&`, `|`, `^`, `~` ile birleşir (CPU ve GPU); kitaplığın üç maske yeri göçtü: `lib/optimizers.tr:338` sayaç için `TR_where(..., 1.0, 0.0)` (339 ve 342 böylece değişmedi; 340'ta koşul `geçti > 0.0`), `lib/layers.tr:419` `m & (...)`; `TR_attention` `TR_Bool` maskeyi alır (içeride float32), ondalık maske de bugün geçer. Kitaplıkta kimlik ve konum `TR_Int` (KV önbelleğinin konumları, `TR_generate`'in adımı; örnek 45 ve 47); pencerenin sınırı konumların float32 kopyasıyla karşılaştırılır (GPU'da tam sayı çıkarması yok; konum < 2²⁴ kesin). GPU'da çizgeli üretim testi: `internal/stdlib/gpuprobe/behavior_gpu_generate_test.go`.
- **Yapılacaklar:** cinsli işleç tablosu, bekleyen işlemler ve `TR_replay`'deki cihaz içi bayrak; TR testi. Göç: kesirli aritmetiğe giren argmax sonucu (`tools/gpu/ss_exam.tr:58`) [kanıtlı: `tools/gpu/ss_exam.tr:58`]; kitaplıkta maske aritmetiği 3 yer [kanıtlı: `lib/optimizers.tr:339, 342`, `lib/layers.tr:419`]: `lib/optimizers.tr:339` (`n + geçti` → `TR_where` ya da `TR_sum`), `:342` (oranın `geçti * g` ile çarpımı → `TR_where(geçti, ..., 0.0)`), `lib/layers.tr:419` (`m * (...)` → `m & (...)`).
- **`@`'nin toplu eksenleri:** bugün öndeki eksenler eşit olmalı; kural onları yayınlar (gerekçe 15.6).

### 15.6a

- **Mevcut durum:** liste işlenen çalışıyor: `TR_Tensor([1.0, 2.0]) + [10, 20]` → `[11.0, 22.0]` [ölçüldü]. 16 bitlik hesap kuralla aynı (en az float32, sayı daraltılmaz) [ölçüldü: DENETİM].
- **Yapılacaklar:** liste işlenenin denetim hatası; tam sayı tensöründe sayının sığma denetimi; TR testi. Göç [ölçüldü: DENETİM B1]: liste işlenen 2 yer (tests/matematik_test.tr, tests/tensor_test.tr).

### 15.6b

- **Mevcut durum:** iki yanlı yayın uyarısı var [ölçüldü].
- **Yapılacaklar:** eksen sayıları eşitken iki yanlı genişlemenin uyarısız kalması doğrulanır; TR testi.

### 15.6c

- **Mevcut durum:** bf16 ile fp16'nın toplamı float32 (kuralla aynı) [ölçüldü].
- **Yapılacaklar:** tam sayı hassasiyetlerinin birleşmesi; TR testi.

### 15.6d

- **Mevcut durum:**
  - tensörle ve maskeyle indeks yok: `t[idx]` ve `t[m]` H6707 veriyor ("tensör indeksi sayı ya da aralık olmalıdır") [ölçüldü];
  - bileşik atama (`+=`, `-=`, `*=`, `/=`, `**=`) tensörü yerinde değiştiriyor: paylaşan adlar görüyor, hassasiyet hedefin, yayın yalnız sağda (H6958), deponun sürümü artıyor; indeksli hedef görünüm üstünde yerinde [test: TestCompoundAssignInPlace, TestCompoundAssignDTypeShapeOverlap, TestCompoundAssignGradRules; GPU'da TestGPUInPlaceCompoundAssign]. `//=`, `%=` ve bit işleçli bileşik atama tensörde hata (cinsli tensörle gelir);
  - atamada ve bileşik atamada örtüşen kaynak önce kopyalanıyor [test: TestCompoundAssignDTypeShapeOverlap];
  - türevin üç yazma kuralı atamada ve `TR_write_slice`'ta uygulanıyor, kipe bağlı (türevsiz bloktan çağrılan fonksiyon da kipte; türevsiz kipte okunan görünüm tabanını biliyor): yaprağa ve görünümüne kipin dışında H6716, ara sonuca kipte de H6955, türev izlemeyen hedefe kipin dışında türev izleyen değer H6956 [test: TestWriteRulesForGradients]; depoda kırılan yer yok [ölçüldü: `tests` ve `lib` klasörlerinin TR testleri, örnekler];
  - `.TR_select`, `.TR_unsqueeze` ve `.TR_fill` yok; `.TR_slice` 31 yerde kullanılıyor [ölçüldü: grep]; `m[:, None, :]` en az 4 yerde `m.TR_reshape([B, 1, T])` diye yazılıyor [ölçüldü: DENETİM].
- **Yapılacaklar:**
  - tensörle ve maskeyle indeks, `.TR_select`, `.TR_unsqueeze`, `.TR_fill` (üçü de türevin yazma kurallarıyla), tensörle indekste biriktirme, `//=` ve `%=` (cinsli tensörle); TR testi;
  - kitaplığın `TR_Tensor(TR_no_grad(...), TR_requires_grad: TR_true)` dolambacı (lib/layers.tr:20) yaprağın tanımıyla sadeleşir;
  - indeksli hedefte yerinde olmayan bir biriktirme (index_add türü) yok; açık soru;
  - liste değeri cinsli hedefe atanınca (`t[0..2] = [1, 2]`) H6966 veriyor; 15.6d'nin önerdiği `TR_Tensor(...)` yazımını öneren ileti değil (T1 dilim 2, DENETİM istek 49 Y3) [ölçüldü: DENETİM istek 57].
- **Tersine dönecek Go testi:** `TestMatrixIndexAndAssign` (`internal/stdlib/ml/matrix_test.go:50`) tam sayı değişmezli matrisi ondalık bekliyor (15.2), 0 boyutlu tensörde `.TR_tolist` okuyor (13.8a) ve liste değerli atama yapıyor (`M[-1] = [8, 9]`; değer sayı ya da tensör olmalı). Aynı dosyanın öbür testlerine de bakılmalı.

### 15.6e

- **Mevcut durum:** üs bir sayı olmalı: `2 ** t` ve `t ** u` (tensör üs) H6702 veriyor; ileti ikisini ayırmadan genel `TR_exp(TR_log(t) * u)` yazımını öneriyor (taban `2` ya da üs tensörü olduğu anlatılmıyor); `(-8.0) ** 0.5` tensörde NaN (kuralla aynı) [ölçüldü: main `85a19c36`, `TR_error_code { 2 ** TR_Tensor([1.0, 2.0]) }` ve `t ** t` H6702, `(TR_Tensor([-8.0]) ** 0.5).TR_tolist` NaN tutan liste].
- **Yapılacaklar:** tensör üs ve tam sayı tensöründe eksi üs hatası; TR testi.

### 15.8

- **Mevcut durum:** türev izleyen hedefe yalnız `TR_no_grad` içinde yazılıyor (türevsiz kipin üç kuralı değil); başlangıç tam sayı ya da tam sayı değerli 0 boyutlu ondalık tensör.
- **Yapılacaklar:** 15.6d'nin üç kuralı; `TR_start`'ın `TR_Int` ya da 0 boyutlu `TR_Tensor[TR_Int]` olması; cins denetimi; `TR_replay`'de sınır dışı başlangıcın cihaz içi bayrakla hatası (15.45).

### 15.13

- **Gemini Koşu 3 (2026-10-29), uygulanacak:** sayı sınırda NaN'da NaN türev; tensör sınırda `TR_maximum` ve `TR_minimum` gibi (eşitlikte yarı yarıya, NaN öğe yayılır).

### 15.15

- **Mevcut durum (T1 dilim 2):** koşul `TR_Tensor[TR_Bool]`; ondalık ya da tam sayı koşul ve sayı imzası (`TR_where(TR_true, 2, 3)`) çalışmada H6207 (ileti `t != 0`'ı ya da `TR_if`'i önerir); iki sayı dalın cinsi onlardan çıkar; TR testi `tests/matematik_test.tr` [ölçüldü]. GPU'da mantık koşul (türevli ondalık yolda koşul float32'ye çevrilir).
- **Yapılacaklar:** hataların denetimde verilmesi (tanımdaki `TR_where` imzası bugün `TR_Bool|TR_Int|TR_Float` koşul ve sayı dönüşü yazıyor; satır bu dilimin değil); `tr.json`'daki açıklamanın "0/1 maskesi" metni. Göç yapıldı (T1 dilim 2: `tests/` 6 satır, Go testleri 4, `lib/optimizers.tr:340`). Göç [ölçüldü: DENETİM B1; kanıtlı: `tests/matematik_test.tr:168-171, 179, 182-183`]: çalışmada hiç 0/1 dışı koşul yok; durağan 4 satır ondalık/tam sayı tensör koşul (`tests/matematik_test.tr:168, :179, :182, :183`), 1 satır skaler `TR_Bool` koşul + tensör dalı (`:169`) ve 2 satır sayı imzası (`tests/matematik_test.tr:170-171`).
- **Gemini Koşu 3 (2026-10-29), uygulanacak:** iki sayılı biçimin cinsi ve hassasiyeti.

### 15.16a

- **Mevcut durum:** `TR_maximum`, `TR_minimum`, tensörde `TR_round` (H2423) ve `TR_is_nan` yok; tensörde `TR_is_finite` var ama `TR_Tensor[TR_Bool]` değil float32 0/1 maskesi veriyor [ölçüldü: main `85a19c36`, `TR_maximum`, `TR_minimum`, `TR_is_nan` bilinmeyen kimlik; `TR_round(t)` H2423; `TR_is_finite(t)` 1.0'lı tensör].
- **Yapılacaklar:** üçü ve `TR_is_finite`'in `TR_Bool` tensörü vermesi; TR testi.
- **Gemini Koşu 3 (2026-10-29), uygulanacak:** `TR_maximum` ve `TR_minimum`'un NaN konumundaki türevi iki işlenene de NaN.

### 15.17

- **Mevcut durum:** `TR_approximate:`'in varsayılanı var ve tanh [ölçüldü].
- **Yapılacaklar:** zorunlu ve yalnız adla verilen `TR_approximate:`. Göç [ölçüldü: DENETİM B1]: argümansız 16 çağrı (examples/tr/ 7, tests/tensor_test.tr 5, Go testlerinin TR metni 4); göçte `TR_approximate: TR_true` yazılır, bugünkü sonuç korunur.

### 15.18

- **Mevcut durum:** eksen `TR_sum`/`TR_mean`'de konumla da veriliyor, `TR_max`/`TR_min`'de verilmiyor (H6205) [ölçüldü]. Maskenin toplamı ve ortalaması ondalık (`2.0`, `0.6666667`); `TR_argmax`'in sonucunun toplamı ondalık `1.0` [ölçüldü]. `TR_std` tensörde H2423 veriyor, `TR_logsumexp` yok [ölçüldü]. `TR_max` ve `TR_min` NaN dışındaki sayılarda −0.0 < 0.0 kullanıyor, CPU ve GPU'da [test: TestMaxNegativeZeroOrder]. 16 777 217 birin float32 toplamı 16777216.0 (sonuç float32'ye yuvarlanıyor; doğru davranış); bf16 toplamın sonucu bf16 [ölçüldü].
- **Yapılacaklar:** eksen yalnız adla; göç 3 yer [kanıtlı: `tests/tensor_test.tr:242`, `tools/gpu/ss_exam.tr:59`, `internal/stdlib/ml/builtin_reduce_test.go:86`]: `tests/tensor_test.tr:242`, `tools/gpu/ss_exam.tr:59`, Go dizesi `internal/stdlib/ml/builtin_reduce_test.go:86`. `tools/gpu/ss_exam.tr:58-60` 15.19'un göçüyle birlikte yapılır (`TR_argmax(...) == iş[1]` ve `TR_sum(isabet, TR_dim: 0)`); cinsli sonuç bu satırdaki gizli `TR_Int += TR_Float`'u da düzeltir. Cinsli sonuçlar, "int64" taşma hatası, `TR_std`, `TR_logsumexp`, liste eksen; TR testleri.
- **Gemini Koşu 3 (2026-10-29), uygulanacak:** boş eksen ile boş tensörün ayrımı, boş tensör listesinde hata, `TR_std`'nin sıfır sapmadaki türevi 0.

### 15.18a

- **Mevcut durum (T1 dilim 2):** eksensiz `TR_all` ve `TR_equal` var, TR testi `tests/tensor_test.tr`'de [ölçüldü]. `TR_equal` iki tensörü CPU'da karşılaştırır (sonuç zaten konağa okunur); cins farkı H6966, tensör olmayan argüman H6979. Tensör olmayan argümanı denetleyici yakalamıyor, çalışmada hata.
- **Yapılacaklar:** `TR_any`, `TR_all_close`, eksenli `TR_all`/`TR_any` ve `TR_keepdim:`; hassasiyete göre toleranslar; ondalık ya da tam sayı tensörün denetim hatası (bugün çalışmada H6979).
- **Gemini Koşu 3 (2026-10-29), uygulanacak:** `TR_all_close`'da sonsuzun kuralı.

### 15.19

- **Mevcut durum:** eksensiz `TR_argmax` bütün öğelerde, satır sırasıyla düzleştirilmiş indeksi 0 boyutlu tensörde veriyor; −0.0 0.0'dan küçük [test: 15.22b'nin TR testi]. `TR_argmin` yok.
- **Yapılacaklar:** `TR_argmin`, eksenli `TR_keepdim:`; TR testi.
- **Gemini Koşu 3 (2026-10-29), uygulanacak:** boş indirgemede hata.

### 15.20

- **Mevcut durum:** `TR_cumsum`'un ekseni varsayılan −1.
- **Yapılacaklar:** `TR_dim:` zorunlu; göç `.tr`'de 7 satır, Go dizelerinde 3 (ikisinin beklenen ileti kodu değişir) [ölçüldü: DENETİM C1]. `TR_Int`/`TR_Bool` girdide "int64" ve taşma hatası.

### 15.21

- **Mevcut durum:** `TR_topk` liste veriyor (`TR_Type`'ı `TR_List`) [ölçüldü]; sıralama sırası 15.22b'nin (−0.0 < 0.0) [test: TestArgSortAndTopK].
- **Yapılacaklar:** demet dönüşü (önce 3.6a'nın demeti); göç 9 çağrı [ölçüldü: DENETİM C1]: `examples/tr/25_secme_ve_siralama.tr:22-23`, `tests/matematik_test.tr` 7 satır, Go 1; TR testi.
- **DENETİM 0.6b (2026-10-29):** `TR_topk` hâlâ liste döndürüyor; kurala göre demet (T1). Demetin değeri motorda var (0.6b).

### 15.21a

- **Mevcut durum:** `TR_gather` yok.
- **Yapılacaklar:** uygulamak; TR testi. Gösterim adı onaylı (Ahmet, 2026-10-29).

### 15.22

- **Mevcut durum:** `TR_topk` ve `TR_argsort`'ta sıralanan eksen en çok 2²⁴ uzunlukta (indeksler float32 depoda); `TR_descending:` yok (motorda `Backend.Sort(..., desc)` hazır [kanıtlı: internal/compute/tensor/device.go:347]). −0.0 0.0'dan küçük, CPU ve GPU'da (`orderKey` ve `orderKeyPTX`) [test: TestArgSortAndTopK, GPU'da TestSort].
- **Yapılacaklar:** 2²⁴ sınırı tam sayı depolamasıyla (15.1) kalkar, yeni sınır 2³² − 1; `TR_descending:`; TR testi.
- **GPU'nun yolu:** her satır 64 bitlik (sıra anahtarı, indeks) çiftleriyle bitonik ağla sıralanıyor; ikinin kuvvetine tamamlanmış uzunluğu 4096'ya kadar olan satır paylaşımlı bellekte tek başlatmayla, daha uzunu genel bellekte her bitonik adım için bir başlatmayla (log₂P·(log₂P + 1)/2) sıralanıyor.
- **Uygulama sınırı:** GPU'da sıralanan anahtarın alt 32 biti indekstir; bu yüzden eksen en çok 2³² − 1 uzunlukta ve sıra anahtarı 32 bit (float32'de yeter; int64 ve float64 tensörde ve daha uzun eksende yeni bir yol gerekir). Aynı sınır `TR_topk`'ta. Kural bir sınır koymaz (bellek dışında, 15.42).

### 15.22a

- **Mevcut durum:** CPU float64'te telafisiz, GPU float32 girdiyi float32'de telafili (Neumaier) biriktiriyor (float64 girdi float64'te). CPU'da `GOMAXPROCS` 1–16'da sonuç bit bit aynı [ölçüldü: DENETİM].
- **Yapılacaklar (C, Ö2; ölçülmedi):** `TestReductionsIndependentOfThreadCount` Go testi bölümün commit'inde yazılır (bugün yok); 15.22a başlıklı TR testi.

### 15.22b

- **Mevcut durum:** tensörde uygulandı: `TR_topk`, `TR_argsort`, `TR_max`, `TR_min` ve `TR_argmax` −0.0 < 0.0 kullanıyor, CPU ve GPU'da [test: TestMaxNegativeZeroOrder]. Listede `[0.0, -0.0].TR_sort()` `[-0.0, 0.0]` veriyor, ama listenin `TR_max([-0.0, 0.0])`'ı −0.0 (13.1'e aykırı) [ölçüldü: T1, `4426eb72`'nin üstündeki çalışma ağacı].
- **Yapılacaklar:** listenin `TR_max` ve `TR_min`'i (`internal/lang/interp/builtins.go`); `TR_argmin`, `TR_maximum`, `TR_minimum` gelince aynı sıra.

### 15.23

- **Mevcut durum:** sınır dışı indeks H6305 veriyor [ölçüldü: main `8b115692`]. İndeks liste ya da ondalık tensör olabiliyor.
- **Yapılacaklar:** indeks `TR_Tensor[TR_Int]`; göç 24 satır [ölçüldü: DENETİM, `git grep` main `8b115692`]; sınır dışı denetimi bekleyen işlem (15.6).

### 15.23a

- **Mevcut durum:** her negatif hedef yok sayılıyor (`-5` de); sınıf sayısından büyük hedef H6314 [ölçüldü: main `85a19c36`]. Olasılık hedefi yok.
- **Yapılacaklar (A, Ö0, Kol B S-kilitten sonra; ölçüldü 30b44b01):** yalnız `-100` yok sayılır, öbür negatifler hata; olasılık hedefi; iki denetim bekleyen işlem (15.6). Göç 33 satır (liste ya da ondalık hedef → `TR_Tensor[TR_Int]`); `-1`'li 3 satır: `tests/tensor_test.tr:215, 289, 290` [ölçüldü: DENETİM, `git grep` main `85a19c36`]. TR testi (15.23a başlığı).

### 15.23b

- **Mevcut durum:** `TR_group_norm` yok [ölçüldü: main `8b115692`].
- **Yapılacaklar:** `TR_group_norm`; TR testi.

### 15.23c

- **Mevcut durum:** evrişim yok; kitaplık yalnız dil modellerinin katmanlarını taşıyor [ölçüldü: `lib/layers.tr`].
- **Yapılacaklar:** `TR_conv1d`, `TR_conv2d`; TR testi. Gösterim adları onaylı (Ahmet, 2026-10-29).

### 15.23d

- **Mevcut durum:** `TR_dropout` yok [ölçüldü: main `8b115692`].
- **Yapılacaklar:** paylaşılan üreteçten (20.17, oradaki yazılabilir üreteçle birlikte); TR testi.

### 15.24

- **Mevcut durum (T1 dilim 2):** `TR_attention` `TR_Bool` maskeyi alıyor (içeride float32), kitaplığın maskeleri `TR_Bool`; ondalık 0/1 maske de geçiyor: sıfırdan farklı her değer "bak" sayılıyor, PyTorch alışkanlığıyla toplanan −∞ maske (`[[0.0, -TR_inf], [-TR_inf, -TR_inf]]`) hatasız ters anlaşılıyor, −∞ yazılan yere bakılıyor [ölçüldü: DENETİM istek 61, `30b44b01`]. Tam sayı verisinden kurulan maske (`TR_Tensor([[1, 1, 0]])`, cins veriden `TR_Int`) H6960 veriyor (T1 bulgu 1) [ölçüldü: T1].
- **Yapılacaklar (A, Ö0, Kol B S-kilitten sonra; ölçüldü 30b44b01):** maske yalnız `TR_Tensor[TR_Bool]`; ondalık ve tam sayı maskenin denetim hatası, ileti `m != 0`'ı önerir. T1'in sorusu (maske hangi cinsleri alır) dil tanımında cevaplı: 15.24 yalnız `TR_Bool` diyor, uygulama ona uyar (TCB, 2026-11-02).
- **Gemini 15/2 (2026-10-29), uygulanacak:** tamamen −∞ sapmalı satır 0 verir; CPU ve GPU (T2). CPU'da tamamen maskeli `TR_Bool` satırı zaten `[0.0, 0.0, 0.0, 0.0]` [ölçüldü: DENETİM istek 61, `30b44b01`]; GPU ölçülmedi.

### 15.27

- **Mevcut durum:** aynı çizgeden ikinci geri yayılım H3017; ara sonucun `.TR_grad`'ı H6700, iletisi nedenini söylüyor [ölçüldü: main `85a19c36`]. Yeni `TR_async` görevi açıldığı yerin kipini almıyor: `TR_no_grad { TR_async { ... } }` içindeki görev türev izliyor (`TR_backward` yaprağın `.TR_grad`'ını dolduruyor), `TR_mixed_precision("bf16") { TR_async { TR_compute_dtype() } }` görevde "float32" veriyor [ölçüldü: main `85a19c36`, çalıştırma].
- **Yapılacaklar:** yeni görev açıldığı yerin türevsiz kipini ve hesap türünü alır; bloktaki `TR_defer` kipin içinde.

### 15.27a

- **Mevcut durum:** `TR_detach` yok; ayırma `TR_no_grad(x)` ile yapılıyor ve `d = TR_no_grad(h); d[0] = 9.0` türev izleyen `h`'yi sessizce değiştiriyor [ölçüldü].
- **Yapılacaklar (A, Ö1, Kol B, 15.6d ile; ölçüm status'un):** `.TR_detach()`; ayrılmış görünüme yazma kuralları (15.6d); `TR_no_grad(x)` biçimi kalkar, göç 20 satır [ölçüldü: DENETİM, `git grep` main `8b115692`]; TR testi.

### 15.28

- **Mevcut durum:** `f_ileri` `[çıktı, [saklananlar]]` listesi döndürüyor [ölçüldü]; eski metne göre çağrının değeri `TR_BuiltinFunction`.
- **Yapılacaklar:** demet biçimi (önce 3.6a); dönüş tipi; göç 22 satır [ölçüldü: DENETİM, `git grep` main `8b115692`].

### 15.29

- **Mevcut durum:** eski metne göre `geri` liste döndürüyor; `g`'ye yerinde yazmak başka bir gradyanı değiştirmiyor [ölçüldü: DENETİM].
- **Yapılacaklar:** tek girdide kendisi, birden çokta demet; fonksiyon ifadesinde öğe sayısının denetim hatası.

### 15.32

- **Mevcut durum:** `TR_grad_check` `TR_true` döndürüyor [ölçüldü].
- **Yapılacaklar:** değer döndürmez; `girdiler` demet; göç `.tr` dosyalarında 40 satır, 34'ü dönüşünü kullanıyor (`31` `TR_assert(TR_grad_check(...))`, `1` `TR_assert_equal(TR_true, ...)` `tests/matematik_test.tr:114`, `2` `TR_print(...)` `examples/tr/27_ozel_turev.tr:23-24`; kalan `6`'sı `tests/ozel_turev_test.tr:170, 172, 183, 186, 188, 189`'da `TR_error_code`/`TR_try` bloğunda deyim; ayrıca Go testinde 2 `TR_print` satırı `internal/stdlib/ml/custom_grad_test.go:30, 272`) [ölçüldü: `git grep -n 'TR_grad_check(' -- '*.tr' '*.go'`, main `c39e0b56`].

### 15.33

- **Mevcut durum:** `TR_parameters` `TR_named:` argümanıyla yol adlı sözlük veriyor; `TR_named_parameters` adı yok; yapıdaki ara sonuçlar ve türev izlemeyen tensörler sessizce atlanıyor [kanıtlı: bugünkü 15.33'ün metni; `definitions.json`'da `TR_named_parameters` kodu yok].
- **Yapılacaklar (A, Ö1, Kol B, 13.8a ile; ölçüm status'un):** `TR_named:` kalkar, `TR_named_parameters` (13.1) gelir; parametre tanımı (donmuşlar `TR_parameters`'ta); `TR_named_parameters` bütün yapraklar; türevli ara sonuçta hata; TR testi bunlara göre.

### 15.34

- **Mevcut durum:** dondurma yok: yaprakta `.TR_requires_grad` üyesi yok (`definitions.json`'da `TR_requires_grad` yalnız adlı argüman kodu) [kanıtlı]. Kitaplığın iyileştiricisi indeksli dolambaçla yazılı: `p[0..p.TR_shape[0]] -= ...`, `g[0..g.TR_shape[0]] = 0.0` [kanıtlı: `lib/optimizers.tr:96-97, 111`]; kitaplığın yolu `tr/optimizers`. Kitaplık parametre gruplarını birden çok iyileştiriciyle yazıyor [kanıtlı: DENETİM E1]. 15.34'ün "kendi iyileştiricisi" TR testi indeksli atamayla yazılı (`tests/tensor_test.tr:414, 415, 417`). Gradyansız `.TR_step()` bugün hata vermiyor (parametresi gradyansız kalan ya da hiç geri yayılım olmayan adım sessiz geçiyor); bir parametre iki iyileştiricideyse ikisi de adımlıyor (aynı gradyan iki kez uygulanıyor); türevsiz kipte 16 bitlik yaprağa bileşik atama uyarı yazmıyor [ölçüldü: main `85a19c36`, çalıştırma].
- **Yapılacaklar (A, Ö0, Kol B S-kilitten sonra; ölçüldü 30b44b01):** yaprağın `.TR_requires_grad` üyesi için yeni üye kodu (gösterim adı "türev_izle", onaylı); dondurma, aşamalı çözme ve iyileştiricinin donmuş yaprağı atlaması; gradyansız `.TR_step()` hatası; türevsiz kipte 16 bitlik yaprağa bileşik atamanın uyarısı; kitaplık ve 15.34'ün TR testi bileşik atamalı yazıma geçer (başlık da değişir); donma durumunun kontrol noktasına girmesi (20.4); TR testleri.
- **Gemini Koşu 1 (2026-10-29):** parametre başına "gradyan eklendi" bilgisi, gradyansız parametreyi atlayan `.TR_step()`, donmuş grubun hatasızlığı, açmanın yalnız kendi deposu olan yaprakta geçerliliği uygulanacak. Kendi iyileştiricisi (`TR_if g != TR_none`) kullanılmayan parametreyi bugün ayıramıyor; kitaplığın Muon'u için soru üyesi Sonraya.
- **Açık (Sonraya):** aynı yapıya eklenen LoRA alanları için `TR_load_parameters`'ın kesin kuralı; bugün yol alt yapıya yüklemek (`TR_load_parameters(model.gövde, ...)`).

### 15.36

- **Mevcut durum:** `TR_is_finite` float32 0/1 veriyor [ölçüldü]; 15.36'nın TR testi float listeleri bekliyor (`tests/matematik_test.tr:261-262`).
- **Yapılacaklar:** `TR_Tensor[TR_Bool]` dönüşü (15.3); test ve 15.36'nın örneği (`.TR_item()` ile) buna göre.
- **Gemini Koşu 3 (2026-10-29), uygulanacak:** kalıp kaybı da sınar; belgelerdeki ve örneklerdeki eğitim döngüleri buna göre.

### 15.37

- **Mevcut durum:** `TR_default_device(" CPU ")` kabul ediliyor [ölçüldü]; `.TR_to_device(" CPU ")` de kabul ediliyor ve TR testi bunu sabitliyor (`tests/tensor_test.tr:1028`). `"gpu:0"`'a taşınan tensörün `.TR_device`'ı `"gpu"` veriyor, test bunu bekliyor (`tests/tensor_test.tr:1036`) [kanıtlı]. Desteklenen GPU yalnız NVIDIA.
- **Yapılacaklar:** tek yazım (büyük harf ve boşluk hata); `.TR_device` uzun ad; iki test satırı tersine döner; TR testi. Başka hızlandırıcılar (Apple, gömülü NPU) yol haritasında.

### 15.38

- **Gemini 15/2 (2026-10-29), uygulanacak:** `TR_load_tensors` varsayılan cihazda kurar (T3).

### 15.41

- **Mevcut durum:** GPU'daki matematik işlevlerinin (`ex2.approx`, `lg2.approx` yolları) ulp sınırı ölçülmedi; 15.11 GPU için "birkaç ulp" diyor.
- **Yapılacaklar:** GPU sınırlarını ölçüp 15.10 ve 15.11'e yazmak; cihazlar arası sözün Go testi.

### 15.42

- **Mevcut durum:** sıfır eksenli (verisiz) şekil, sıfır eksenleri 1 sayarak 50 milyon öğelik koleksiyon sınırına göre denetleniyor: `TR_zeros`, `TR_ones`, `TR_randn` ve boş tensörün `.TR_reshape`'i `TR_zeros([0, 1000000000, 1000000000])`'u reddediyor; veri taşıyan tensörün sınırı ise 2³² öğe (`TR_zeros([60000000])` başarılı, `TR_zeros([100000, 100000])` "at most 4294967296 elements") [ölçüldü: main `85a19c36`, çalıştırma; `internal/lang/interp/builtins.go:287`]. GPU çekirdeklerinin indeksi 32 bit: bir uygulama sınırıdır, dil kuralı değildir; Llama 3 405B'nin gömmesi (~2,1·10⁹ öğe) bu sınıra dayanıyor.
- **Yapılacaklar:** 50 milyon sınırı (sıfır eksenli şekillerde; veri taşıyanda 2³² sınırı da kalkar) [kanıtlı: internal/stdlib/ml/error.go:188-209, `internal/lang/interp/builtins.go:287`] kalkar, yerine int64 taşma denetimi (yukarıdaki örnek artık hata değildir; dev sonuç ayrılırken yakalanır); 64 bitlik GPU indeksi yol haritasında.

### 15.43

- **DENETİM 0.6b (2026-10-29):** `.TR_append` ve indeksle atamada yalnız yazılan değer kaçıyor (kurala uygun; testli: `TestOwnershipListLiteralOwnsElementsLikeATuple`, `TestOwnershipIndexAssignmentEscapesOnlyTheWrittenValue`); öbür değiştiren yollar (ör. `+=`) bütün listeyi ve öğelerini kaçırıyor (`internal/lang/interp/assign.go:75-78`): güvenli taraf ama sapma, testi yok. Test satırına 0.6b'nin iki sahiplik testi girecek.

### 15.44

- **Mevcut durum:** `TR_fuse`'un kapanışından okunan sayı değişince sonuç sessizce eski kalıyor (sabit 2'den 5'e çıktı, sonuç `[2, 4]` kaldı) [ölçüldü]; CPU'da `TR_fuse` eski değeri, `TR_replay` yeni değeri veriyor, GPU'da `TR_replay` de sabitliyor: aynı program cihaza göre ayrı sonuç veriyor [ölçüldü: DENETİM]. Go testi `TestFuseCapturedNumberIsBakedIn` bu sessiz eskimeyi doğru davranış diye sabitliyor (`internal/stdlib/ml/builtin_fuse_test.go:8`). `TR_clamp`, `TR_atan2` ve `TR_where` kaynaştırılmıyor.
- **Yapılacaklar (A, Ö0, Kol B S-kilitten sonra; ölçüldü 30b44b01):** kapanıştaki sayının kaydı ve her çağrıda, her cihazda denetimi; yolu olmayan sayıda kayıt hatası. Kural uygulanınca `TestFuseCapturedNumberIsBakedIn` tersine döner: değişen kapanış sayısı hata vermeli; TR testi.
- **Uygulama sınırı:** `TR_clamp`, `TR_atan2` ve `TR_where` bugün kaynaştırılmıyor; `f`'de kullanılınca hata veriyor. Kural onları da kapsar.
- **Gemini 15/2 (2026-10-29), uygulanacak:** kapanış yolunda liste indeksi ve sözlük anahtarı (2. kilometre taşının ön koşulu); `TR_fuse`'da float64 ve tam sayı kendi türünde (T3).

### 15.45

- **Mevcut durum:** kayıtta `TR_embedding_lookup`'ın tensör indeksi ve `TR_write_slice`'ın tensör başlangıcı sınır dışındaysa sessizce kırpılıyor (gömmede `[0, V−1]`, `TR_write_slice`'ta hedefe sığan son konum), kesirli değer budanıyor [kanıtlı: bugünkü 15.45'in metni]. Kapanıştan okunan sayılar kayıtta sabitleniyor, denetlenmiyor [ölçüldü: DENETİM].
- **Yapılacaklar (A, Ö1, Kol B, 15.6'nın cihaz içi bayrağıyla; ölçüm status'un):** kırpma kalkar, cihaz içi hata bayrağı (15.6); kapanıştaki sayının ve tensör deposunun yoluyla denetimi; rastgele üretecin oynatmada ilerlemesi; Go testleri.

### 15.46

- **Mevcut durum:** tensörler yalnız float tutuyor; `TR_save_tensors` float32, bf16, fp16 yazıyor (sürüm 2; tür kodları 1, 2, 3) [ölçüldü]; biçim uzantıya göre seçiliyor [test: TestSaveTensorsChoosesFormatByExtension]. Dosyada veri taşıyan tensör en çok 2³² öğe, verisiz şekil (sıfır eksenler 1 sayılarak) en çok 50 milyon öğe bildirebiliyor [kanıtlı: bugünkü 15.46'nın metni]; 15.46'nın sıfır uzunluklu eksen TR testi (`tests/tensor_test.tr:560`) `[1048576, 0, 8192]` şekilli verisiz tensörün reddini bekliyor.
- **T1 dilim 2:** her cinsin ve hassasiyetin tür kodu var (sürüm 2 kaldı; 1–3 aynı, 4 float64, 5–8 int64/int32/int16/int8, 9–11 uint32/uint16/uint8, 12 mantık); `TR_requires_grad: TR_true` yalnız hesap ondalıklarında türevi açar; uzantının harf büyüklüğü fark etmiyor (`.SAFETENSORS`, `.TRT`); TR testi `tests/tensor_test.tr` "15.46 ve 15.47a dosyada her cins" [ölçüldü]. Kontrol noktası (20) cinsli tensör yazmıyor (paketine dokunulmadı; `ml.TensorEntry`'ye `Ham` alanı eklendi).
- **Yapılacaklar:** `TR_AnyTensor` ve `.TR_narrow[E]()`; nicemlenmiş saklama hassasiyetlerinin tür kodları (15.55); 2³² ve 50 milyon sınırları kalkar, öğe sayısı int64 taşma denetimiyle ve ayırma 15.42'nin bellek denetimiyle (sıfır eksen testi yeni kurala göre yeniden yazılır); TR testi.
- **Gemini 15/2 (2026-10-29), uygulanacak:** `TR_requires_grad: TR_true` yalnız nicemlenmemiş `TR_Float` tensörlerde (T3).
- **Tur 3 (2026-11-02):** uzantının harf büyüklüğü — T1 dilim 2'de yapıldı (yukarıda).

### 15.47

- **Mevcut durum (T1 dilim 2):** F32, F64, BF16, F16, I64, I32, I16, I8, U32, U16, U8 ve BOOL okunuyor (BOOL'da sıfırdan farklı bayt doğru); U64 ve F8 türleri H6643 [ölçüldü: Go `TestSafetensorsRejects`, `TestSafetensorsPythonRoundTrip`].
- **Yapılacaklar:** `F8_E4M3`'ün ölçeği 1 "fp8" olarak okunması (15.55).

### 15.47a

- **Mevcut durum (T1 dilim 2):** `.safetensors` yolu her cinsi (F32, BF16, F16, F64, I64, I32, I16, I8, U32, U16, U8, BOOL) hizaya göre yazıyor; Python `safetensors` (torch) ile iki yönde bit bit aynı [test: TestSafetensorsPythonRoundTrip, bu makinede torch 2.14 ile koştu].
- **Mevcut durum (`__metadata__`):** bu adlı tensörü yazmak çalışma hatası H6951 [test: TestSafetensorsWriteRejects].

### 15.49

- **Mevcut durum (T1 dilim 2):** cinsler ve tam sayı taşması kendi maddelerinde (15.2, 15.3, 15.5, 15.6a–15.6d). Tensörün `==`'si ve `!=`'si eşitlik kancasından gidiyor: iki yan birden genişlediğinde yayın uyarısı `<` için yazılıyor, `==` için yazılmıyor (T1 bulgu 7) [ölçüldü: DENETİM istek 57, `[2,1] < [1,3]` ve `==`].
- **Yapılacaklar:** `==`/`!=`'de yayın uyarısı; satırların kalanı kendi maddeleriyle; TR testi.

### 15.50

- **Mevcut durum:** float32'nin ucundaki büyük sonlu sayılarda CPU float64'te biriktirdiği için sonlu, GPU float32'de biriktirdiği için sonsuz sonuç verebiliyor (normlarda x², toplamın kendisi 3,4e38'i aşınca). Karşılaştırma NaN'da 0 veriyor, `TR_where` NaN koşulu doğru sayıyor [ölçüldü]; yeni kural `TR_false` ve mantık koşulu (15.5, 15.15). `TR_logsumexp`, `TR_maximum` ve `TR_minimum` yok; `TR_max`/`TR_min` ±0'ı eşit sayıp ilk öğeyi alıyor.
- **Yapılacaklar (C, Ö2; ölçüm status'un):** sayısal doğruluk kapısı (eski yol haritasının Evre 4, madde 8; bugünkü durumu ölçülmedi): her türevli işlemin CPU ve GPU sonucunun float64 referansa karşı testi ve işlem başına hata sınırı (normlar ve toplamlar için var: `TestNormsMatchFloat64Reference`, `TestSumsMatchFloat64Reference`), GPU bf16 sapma tablosu, `TR_log10` ve `TR_atan2` türevinde alt normal yerine 0, çapraz entropinin çok büyük logitte lse − x ile hassasiyet kaybı ((m − x) + log Σ sırası düzeltir); `TR_logsumexp`, `TR_maximum`/`TR_minimum` satırları ve `TR_max`/`TR_min`'in sıralama sırası (15.22b); TR testi.
- **Gemini Koşu 3 (2026-10-29), uygulanacak:** `TR_relu`, `TR_clamp`, `TR_maximum` ve `TR_minimum`'un NaN girdideki türevi NaN; iki cihazda aynı olduğunu sınayan test.

### 15.51

- **Mevcut durum:** nicemleme yok. Motorda `Int8` ("ölçek ayrı tutulur") ve `Int4` ("bir baytta iki eleman") türleri ayrılmış, "henüz hiçbir arka uç hesap desteği sunmaz" [kanıtlı: internal/compute/tensor/device.go:89-94]. Yol haritasının 2. kilometre taşı: "Ağırlık 16 bit ve nicemlenmiş (8 ve 4 bit) olarak yüklenir"; bugün "nicemleme ... yok" [kanıtlı: `docs/roadmap.md`].
- **Yapılacaklar:** dört hassasiyet ("q8", "q4", "q4z128", "fp8"); TR testi. Başka grup boyları, K-nicemleme ailesi, NF4 ve 2–3 bit Sonraya.

### 15.52

- **Mevcut durum:** uygulanmadı. Hugging Face'in `weight_scale_inv`'inin çarpılan ölçek olduğu bir hipotezdir [hipotez: DENETİM F2].
- **Yapılacaklar:** `.TR_to_dtype` ile nicemleme ve açma (llama.cpp ile bit bit, CPU ve GPU'da aynı kodlar); yeni kimlikler `TR_quantized` (gösterim adı "nicemlenmiş") ve `TR_zero_point` ("sıfır_noktası"), onaylı; NaN ve aralık denetimleri 15.6'nın bekleyen işlemlerinde; `weight_scale_inv`'in anlamını gerçek bir FP8 modeliyle doğrulamak; TR testi. Sıra değiştiren gruplar (GPTQ `desc_act`, `g_idx`) Sonraya.
- **Gemini 15/2 (2026-10-29), uygulanacak:** q4'ün sıfır bloğu −0.0 ölçek ve 8 kodla llama.cpp ile bit bit (baytlarla test); q4z128'in RTN formülü; fp8'in kısmi son bloğu (T3). **Açık karar:** GPTQ/AWQ ölçeği bf16 kalsın mı (dosya biçimi).

### 15.53

- **Mevcut durum:** uygulanmadı.
- **Yapılacaklar:** CPU'da (AVX2, ARM NEON) ve GPU'da blok blok açan çarpım ve gömme çekirdekleri; geri yönün ikinci çekirdeği; 2. kilometre taşının hız ölçütü; TR testi ve Go testi (CPU ve GPU, açılmış hesapla karşılaştırma). Nicemleme farkında eğitim (QAT) Sonraya.
- **Gemini 15/2 (2026-10-29), uygulanacak:** devrik görünümün izinli kullanımları; fp8'in ilk eksen kuralı (T3).

### 15.54

- **Mevcut durum:** uygulanmadı.
- **Yapılacaklar:** yerinde yazma hataları; yüklemenin (20.6, 20.11, 20.14) aynı belleğe bütün olarak yazması; TR testi.

### 15.55

- **Mevcut durum:** uygulanmadı; safetensors'ta `F8_E4M3` okunmuyor [kanıtlı: bugünkü 15.47'nin metni].
- **Yapılacaklar:** TR biçiminde nicemlenmiş tür kodları; `F8_E4M3` okuma; `.safetensors`'a nicemlenmiş tensör yazma hatası (H6950 bugün yalnız TR biçimini öneriyor; `.TR_to_dtype("float32")` önerisi 15.52 uygulanınca, öneri tablosunda koşulabilsin diye); kitaplığın (`tr/pretrained`) GPTQ, AWQ ve FP8 parçalarını birleştirmesi; TR testi. GGUF okuma Sonraya (yol haritası, "Paylaşmak").

## 16. Veri ve parçalayıcı

Kurallar: [`docs/spec/16_veri_parcalayici.md`](spec/16_veri_parcalayici.md). Taslak (2026-10-29): ana oturum yeniden yazdı, önce CLAUDE.md'den türeterek (Ahmet: "dil maddeleri yalan yanlış olsa da düzeltmezsek bizi hep eskiye götürecek"). Metin DENETİM 16-1'den geçti, bulguları işlendi (`0cd455a4`); kararlar Gemini Koşu 7'den (`59efe468`; 16.4'ün tabanı Koşu 4, `23e7be25`) geçti. Kapanış turu yapılmadı. Yeni adların gösterim adları onaylandı (2026-10-29): `TR_train_tokenizer` parçalayıcı_eğit, `TR_reserved_tokens` ayrılmış_parçalar, `TR_write_dataset` veri_kümesi_yaz, `TR_shard_tokens` parça_boyu; kodları uygulandıkları gün kayda girer (17.4).

### 16.2

- **Gemini Koşu 7 (2026-10-29), uygulanacak:** `TR_special_tokens: TR_false` metin içi özel parçayı da kapatır.
- **Mevcut durum:** uygulanmadı. `TR_false` yalnız başa ve sona eklenenleri kapatıyor; metindeki `<|im_end|>` yine özel parçanın kimliği oluyor. `p.TR_encode`'un Go kodu `DontAddSpecial`'ı kuruyor, `DontParseSpecial`'ı kurmuyor [ölçüldü: 2026-11-02, main `705b721b`, gpt2 `tokenizer.json`'a eklenen `<|im_end|>` ile; kanıtlı: `internal/stdlib/tokenizer/tokenizer.go`, kodlama metodu]. Güvenlik açığı: sunulan bir modelde kullanıcı metni rol parçası üretebilir.
- **Yapılacaklar (B, Ö0, ilk boşalan kol; ölçüm status'un; öncelikli; Jarvis'in sohbet biçiminin ön koşulu):**
  - 16.2'nin kuralı ve TR testi: metindeki özel parça yazımı `TR_false` ile baytlarına kodlanır.
  - 16.2'nin genişlemesi (2026-11-02, Jarvis B; dil tanımında, uygulanmadı): `TR_false` özel olmayan eklenmiş parçaları da (`<think>`, `<tool_call>`) kapatır. 2026-11-02'de TR de Hugging Face de onları her yolda eşliyor; HF'nin `split_special_tokens=True`'su yalnız özel olanları bölüyor [ölçüldü: 2026-11-02, tokenizers 0.23.2, transformers 5.17.0].

### 16.3

- **Mevcut durum:** iki boyutlu tensör ya da liste listesi de metin listesine çözülüyor; dağarcıkta olmayan kimlik sessizce atlanıyor [ölçüldü: main `f1ebe718`]. Yarım kalan UTF-8 dizisi hata vermeden U+FFFD oluyor; beş dizide Hugging Face ile bayt bayt aynı [ölçüldü: 2026-11-02, main `705b721b`]. 16.3 bunu 2026-11-02'den beri yazıyor; akışlı çözme 16.3a'da, uygulanmadı.
- **Yapılacaklar (A, Ö0, ilk boşalan kol (16.2 ile); ölçüldü 30b44b01):** `.TR_decode` yalnız tek dizi, birden çok dizi `TR_batch_decode` (13.8a); dağarcık dışı kimliğin hatası ve TR testi.

### 16.3 (sürüyor)

- **Test:** `tests/veri_parcalayici_test.tr`'nin 16.3 testi bugünkü toplu çözümü (liste listesi, 2 boyutlu tensör) sınıyor; `TR_batch_decode` gelince değişir.

### 16.3a

- **Mevcut durum:** uygulanmadı; akışlı çözme yok, `p.TR_decode` yarım karakterde U+FFFD veriyor [ölçüldü: 2026-11-02, main `705b721b`].
- **Yapılacaklar:** `TR_DecodeStream`, `.TR_step`, `.TR_finish`; birleşimin toplu çözümle aynı olduğunu sınayan TR testi (bayt düzeyi ve baştaki boşluklu çözücüde).

### 16.4

- **Mevcut durum:** parçalayıcı eğitmek yok; bugün Python'a gidiliyor.
- **Yapılacaklar:** bayt düzeyinde BPE eğitimi, belirlenimci; `.TR_save` ile `tokenizer.json`; TR testi.
- **Gemini Koşu 7 (2026-10-29), uygulanacak:** dosya listesiyle eğitim; bayt düzeyi ön ayırıcı ve çözücü, eşitlik kuralı (Hugging Face ile bir kez sınanacak); özel parça eklemez.

### 16.5

- **Yapılacaklar (dil tanımı turu 1, 2026-11-01):** okuyucu `int64`'e geçer; `int32`'ye sığmayan `<u4` ve `<i8` değerini reddetmek (H5086) dil tanımına aykırı: 16.5 yalnız tür kümesini söyler, 16.6 2³²'ye kadar işaretsiz kimliğe izin verir, 16.7 `"int64"` tensör üretir [kanıtlı: Gemini `TUR1_RAPORU.md` satır 27, DENETİM `kol-denetim-tur1-rapor-sonuc.md`]. Bugün `internal/compute/dataload/shard.go:282` (`decodeI32`, 256) reddediyor; `TestShardU32Overflow`'un `<i8` satırları ve `tests/veri_parcalayici_test.tr`'nin sınırı ters döner. S1'in işi.

### 16.6

- **Mevcut durum:** veriyi yazan yerleşik yok; Go tarafında işaretsiz 16 bit ve işaretli 32 bit yazıcı var (`WriteNpyU16`, `WriteNpyI32`) [kanıtlı: `internal/compute/dataload/npy.go:512,525`].
- **Yapılacaklar (A adayı, diskteki veri; sessiz mi ölçülmedi):** `TR_write_dataset`, tür seçimi ve `meta.json`; TR testi.
- **Gemini Koşu 7 (2026-10-29), uygulanacak:** var olan kümenin yerine atomik yazma.

### 16.7

- **Mevcut durum:** `.TR_next()` iki öğeli liste veriyor (girdi ve hedef); tensörler float32 ve "tam sayılı işaretli" [ölçüldü]. Örnekleyici kontrol noktasına giriyor (20.2'nin TR testi); bağlamdan kısa parçalar atlanıyor (`TestSamplerWindowBoundsAndShift`, `tests/veri_parcalayici_test.tr`: "16.7 örnekleyici bağlamdan kısa parçayı atlar") [kanıtlı: internal/compute/dataload/sampler.go:206-209; ölçüldü: `tests/veri_parcalayici_test.tr`, `21468919`].
- **Yapılacaklar:** demet dönüşü (3.6a) ve `TR_Tensor[TR_Int]` (15'in A'sı); göç: `.TR_next()`'i `[0]`/`[1]` ile okuyan yerler.

### 16.8

- **Mevcut durum:** sıralı okuyucu döngüyle geziliyor, liste veriyor [ölçüldü].
- **Yapılacaklar:** demet; döngüde açma (iki adla `TR_for`).

## 17. Kodlar

Kurallar: [`docs/spec/17_kodlar.md`](spec/17_kodlar.md). Taslak (2026-10-29): ana oturum yeniden yazdı (Ahmet: "17 şimdi düzeltsen ? sürekli kafanı karıştıracak"); 2. turda madde madde CLAUDE.md'den yeniden tartıldı (görünüm yalnız okunur); 3. turda DENETİM'in 1. turunun görünümden bağımsız bulguları, 4. turda 17-2'nin bulguları alındı (`0f1d6029`); kararlar Gemini Koşu 6'dan geçti (DENETİM doğruladı, `c784c47a`). Kapanış turu yapılmadı. 4a'dan önce gelir, çünkü 4a kod kaydına yeni kodlar ekler.

### 17.1

- **Mevcut durum:** kullanıcının yapısında `TR_` adlı alan ve metot sessizce kabul ediliyor (`TR_struct K { TR_step }`), oysa `TR_var TR_x` reddediliyor [ölçüldü: DENETİM]. Testler ve örnek düz ada geçti (`tests/coded/ornek_kodlu.tr`'nin yapısı, `examples/tr/17_testler_test.tr`'nin `Sepet`'i).
- **Yapılacaklar:** yapının alanında ve metodunda `TR_` adının denetim hatası, `TR_init` ve `TR_next` dışında; TR testi. Göç: `tools/gpu/measurement_model.tr:36-55` kullanıcı yapısında `TR_forward` (2) ve `TR_starts_with` (1) tanımlıyor [ölçüldü: DENETİM]. Kullanıcı yapısının `TR_next` ile dolaşılması bugün çalışmıyor ("Sayaç tipi üzerinde döngü kurulamaz") [ölçüldü: DENETİM]; 5.7 uygulanınca TR testi.

### 17.2

- **Mevcut durum:** program metninde kısa yazım, içeride sınıflı kimlik uygulanmış (katman 1). 59 kısa ad birden çok sınıfta, hepsi yerle ayrılıyor; aynı yerde çakışan yok, `TestNoTwoIdentitiesShareASpellingInAPlace` sınıyor [ölçüldü: main `85a19c36`, `definitions.json` anahtarları sayıldı ve test koşuldu].

### 17.3

- **Mevcut durum (Gemini Koşu 6, 2026-10-29):** kitaplık parametresinin (`PlaceArg`) ve yapı alanının/metodunun (`PlaceMember`) yeri sözcük çözümleyicide uygulanmış (`internal/lang/lexer/identity.go:169-176`); fonksiyon gövdesinde `paramInScope` önce çevreleyen fonksiyonun `TR_` parametresine bakar (`identity.go:285-293, 328-349`), görünüm (internal/lang/source/show.go:149) ve kod dizini (`internal/devtools/codeindex/read.go:49`, `bind.go:20-24, 135-153`) aynı sözcük çözümleyiciyle kitaplık gövdesinde kapsama göre seçer (`lib/optimizers.tr:423, 428`'deki `TR_min` parametresi "TR_arg_min" olarak çözülür: görünümde "en_az", dizinde `TR_lr_schedule`'ın "TR_arg_min" parametresi) [ölçüldü: `TR_cmd_show` ve `TR_cmd_index` `TR_sub_references`, `b77644dd`].

### 17.4

- **Mevcut durum:** 564 kodun 245'i kitaplık kodu; kitaplık kodlarının hiçbirinin açıklaması ve örneği yok, `TestEveryCodeHasDescription` onları atlıyor ("madde 15 adım 3c") [ölçüldü: main `85a19c36`, `definitions.json` ve `tr.json` sayımı].
- **Yapılacaklar:** kitaplık kodlarının açıklaması ve örneği; atlama kalkar. Dil tanımına giren ama uygulanmamış adlar (13'ün ve 15'in yeni adları) uygulandıkları commit'te tanım ve dil dosyalarına girer.
- **Mevcut durum (Faz 0.5, 2026-10-29):** 319 örnek kimlik yazımıyla tanım dosyasında (`definitions.json`, `example`); dil dosyaları örnek taşımaz, ayrıştırıcısı alanı reddeder. Taşıma bir betikle, o günkü kodlayıcıyla yapıldı (elle değil). `TestExamplesRun` program örneklerini (265) kimlik okuyucusuyla (`lexer.NewBody`) okuyup denetler ve çalıştırır; komut satırı örnekleri (54: komut, alt komut, seçenek, ortam değişkeni, kabuk ve hata ayıklayıcı komutu) kimliğin adıyla yazılı ve program olarak sınanmaz [ölçüldü: main `85a19c36`, `definitions.json` sayımı]. `docs/codes.md` örneği kimlik yazımıyla ve her gömülü dilin görünümüyle gösterir.
- **Yapılacaklar:** komut satırı örneklerinin sınanması (komut satırı adları 17.9'un girdisiyle, Faz 0.4).

### 17.5

- **Mevcut durum:** ad alanında çakışma dil dosyası yüklenirken reddediliyor (`TestClashesCaught`). Türkçe harfsiz biçim kuralı kalktı (17.7). Alışkanlık adının gösterim adıyla çakışması artık hata değil, o alışkanlık adı atlanıyor (`language.go:122-131`, `:193-195`) [ölçüldü: main `85a19c36`, kod okuma]. Dayanağı kalmayan ASCII denetimi ve öneri yolu (`AcceptASCII`, `language.go:32-39`, `TestASCIIFormsDoNotClash`) kalkacak.
- **Mevcut durum:** `TR_assert_equal`'ın Türkçe adı "eşit_olmalı" oldu (2026-10-29, önceki ad soru gibi okunuyordu).

### 17.7

- **Mevcut durum (Faz 0.5, 2026-10-29):** görünümden geri kodlama kalktı: kodlama komutu (tr kodla; kaydı silindi, iletileri `retired.json`'da), `internal/lang/source`'un kodlayıcısı (`Encode`, `EncodeIn`, `TextLexer`, `TextLexerWithNames`), gösterimin geri kodlama denetimi, tek seferlik çevirici `tools/migrate` [ölçüldü: `git grep`]. Yaklaşık kodlama da onunla kalktı. `internal/lang/source` 1 870 satırdan 1 454 satıra indi (testler dahil) [ölçüldü: wc -l, main `85a19c36`].
- **Görünüm:** `TR_cmd_show` görünümü yazar (`source.Show`); görünüm başlıklı dosya (`# TR 4 view=tr`) çalıştırılınca ve denetlenince ileti (H2541) kodlu dosyayı çalıştırmayı söyler; başlıksız dosyanın iletisi (H2540) başlığı ve kimlik yazımını söyler, kodlamayı önermez. Görünüm dilin sözcüğüyle aynı yazılan kullanıcı adını `$ad` diye işaretliyor (kural değil, 17.7).
- **Kabuk ve hata ayıklayıcı:** kabuk satırı ve hata ayıklayıcının ifadesi (koşul, izleme, değerlendirme) kimlik yazımıyla okunuyor; Türkçe adıyla yazılan yazdırma çağrısı tanımsız ad hatası veriyor [ölçüldü: Go testi]. Kabuğun ve hata ayıklayıcının komut adları (`TR_repl_exit`, `TR_dbg_eval`) 17.9'un işi (Faz 0.4).
- **Düzenleyici:** VS Code eklentisi dosyayı kimlikleriyle açıp kaydediyor; görünümü yanda salt okunur açıyor (`view.js`); görünümde düzenleme ve kayıt (`coded_*.js`) kalktı.
- **Kalan:** sözcük çözümleyicinin bir yazım dilinde okuma kipi (`lexer.NewBodyIn`'in kodsuz kipi, onu izleyen `$` kaçışı, `WrittenIn`, denetleyicinin `SetFileLanguage`'ı) üretimde çağrılmıyor, yalnız Go testlerinde kullanılıyor [ölçüldü: `git grep`, main `85a19c36`, test dosyalarında 20 satır]; kaldırılması ayrı bir iş.
- **Testler:** `TestShowTranslatesOnlyTheLanguageWords` (yalnız görünüm: satırlar, yerleştirmenin çevrilmesi, kullanıcı adının kalması), `TestViewHeader` ve `TestShowCommand` (görünümün çalışmaması), `TestTypedLineIsReadWithIdentities`, `TestCLIExpressionIsWrittenWithIdentities`, `TestHeaderlessFileDoesNotRun`.
- **Yeniden kurulacak:** Faz 0.5'te iki atlanmış sınav silindi; öncülleri bir yazım dilinde yazılmış dosyaydı (dosyanın dili 2.1'de kalktı). Görüntü sınavı (`TestDisplayExam`): kişiye görünen her yüzey okuyanın dilinde. Ad çakışması sınavı (`TestNameClashExam`): başka bir dilin sözcüğü kullanıcının adı olunca ya hata ya aynı sonuç. 17 yeniden yazılınca kodlu dosyalar ve görünümle baştan kurulur.

### 17.8

- **Mevcut durum:** değerin metni dilden bağımsız ("true", "none", "List", "<builtin print>") [ölçüldü]. Kitaplık tipinin metni sınıflı kimlik veriyor: `TR_print(TR_Type(TR_Linear(2, 3, TR_seed: 1)))` → "TR_type_Linear" (`TR_import "tr/layers"` ile) [ölçüldü: main `85a19c36`].
- **Yapılacaklar:** tipin metni kimliğin adı ("Linear"); TR testi.
- **Mevcut durum (kitaplığın sözlük anahtarları):** `TR_GPT`'nin `.TR_config` sözlüğünün anahtarları Türkçe ve İngilizce karışık ("dağarcık", "bağlam", "katman", "tohum" ... ile "embed_dim", "num_heads") [ölçüldü: `lib/layers.tr:500-503`, main `aad41d1e`]. Değer metni dilden bağımsız olmalı. Sözlük kayda giderse anahtarlar diske Türkçe yazılır.
- **Yapılacaklar:** anahtarlar kimlik adlarıyla, parametre adının kendisi ("vocab_size", "context" ...), ya da sözlük yerine bir yapı; göç: `.TR_config`'i okuyan yerler; TR testi.

### 17.9

- **Mevcut durum (Faz 0.4, 2026-10-30):** komut satırı girdisi ("cmd", "sub", "opt", "repl", "dbg", "env") yalnız kimliğin adını kabul ediyor ("tr run", "tr index definition", "--lang", "exit", "step", "TR_LANG"); gösterim dilindeki ad hata veriyor ve kimliğin adını öneriyor (komut H8602 `cmd/tr/main.go:81`, `cmd/tr/options.go:84`; alt komut H8876 cmd/tr/index.go:50; seçenek H8674 `cmd/tr/options.go:59` ve program seçeneği H7209 `internal/stdlib/process/arguments.go:206`; ortam değişkeni H8875 `internal/lang/codes/environment.go:52`) [test: `internal/lang/codes/language_test.go`, `cmd/tr/e2e/behavior_commands_test.go`]; yardım ve iletiler yazılacak kimlik adını ve kişinin dilindeki açıklamayı gösteriyor; depodaki yazımlar (commit `ca471b63`'te 192 dosya; önceki "186 dosya" sayısı depoda bulunmayan `eski_denetimler/faz0_4_betik.py`'den geldiği için doğrulanamıyor) kimlik adına göç ettirildi [ölçüldü: git show --stat `ca471b63`, main `c39e0b56`].

### 17.10

- **Mevcut durum:**
  - dil tanımının bölümleri program metnini kısa kimlik yazımıyla anlatıyor; sınıflı yazım yalnız komut adlarında (`TR_cmd_run`), disk biçiminin değerinde (17.8'in kontrol noktası örneği) ve 17'nin kendi kural anlatımında kalıyor [ölçüldü: main `85a19c36`, `docs/spec/*.md`'de sınıflı kimlik sayımı]; öbür belgeler 2.1'in satırında;
  - 14. bölüm silindi; başka bölümlerde eski 17'yi anlatan yer kalmadı; 19:56'nın göndermesi (dosya biçimi çevrilmez, "17.3") 17.8 olmalı [ölçüldü: main `85a19c36`, `docs/spec/19_surec_ve_kayit.md`].
- **Yapılacaklar:** `docs/spec/19_surec_ve_kayit.md:56`'nın 17 göndermesi.

## 18. Dosya ve yol

Kurallar: [`docs/spec/18_dosya_ve_yol.md`](spec/18_dosya_ve_yol.md). **Kapandı** (2026-10-29): ana oturum CLAUDE.md'den türeterek yeniden yazdı, iki tur DENETİM; kararlar Gemini Koşu 5'ten geçti (DENETİM doğruladı, `d31c9226`). Açık: 18.16'nın göreli yol kuralı. 2. tur (2026-10-29): DENETİM 18-1'in bulgularıyla. Yeni adların gösterim adları onaylandı (2026-10-29): `TR_write_lines` satırları_yaz, `TR_LineWriter` SatırYazıcı, `TR_write` yaz (18.17); `TR_ignore_case` harf_duyarsız (18.5); kodları uygulandıkları gün kayda girer (17.4).

### 18.1

- **Gemini Koşu 5 (2026-10-29), motor zaten böyle:** kök ve UNC öneki korunuyor [ölçüldü: DENETİM]. **Gemini Koşu 5 (2026-10-29), uygulanacak:** `..` ile tabandan kaçış hatası (bugün `TR_Path("veri", "../../gizli/x.json")` tabandan çıkıyor [ölçüldü: DENETİM]).

### 18.5

- **Mevcut durum:** Windows'ta kalıp harf büyüklüğüne bakmıyor: `"*.TXT"` `a.txt`'yi eşliyor [ölçüldü: main `dc4930ca`].
- **Mevcut durum:** başka bir klasörden çalışan programda `TR_list_dir("cikti", "*.tmp")` programın klasöründeki `cikti`'yi buluyor ve göreli yollar veriyor [ölçüldü: DENETİM 18-1].
- **Yapılacaklar:** her sistemde harf büyüklüğüne duyarlı eşleme (göç: `internal/stdlib/files/list_test.go`'nun harf büyüklüğüne bakmayan dalı), `TR_ignore_case:`, başka klasörde bulunan sonuçların mutlak olması; TR testi.

### 18.7

- **Gemini Koşu 5 (2026-10-29), uygulanacak:** `TR_Logger` ile açık dosya da silinmez.

### 18.9

- **Mevcut durum:** dolu klasörün yerine konması uygulandı (Gemini Koşu 5): eski klasör geçici ada çekiliyor, yenisi yerine konunca siliniyor, yerine koyma olmazsa eski geri konuyor; `TR_copy`'de `TestCopyDir` sınıyor, `TR_move` (18.10) aynı yolu kullanıyor [ölçüldü: main `85a19c36`, `internal/stdlib/files/copy_run.go:60-86`, dolu klasör üstüne kopyalama ve taşıma].
- **Yapılacaklar (A adayı, diskteki veri; sessiz mi ölçülmedi):** arada kesilince geçici adıyla kalan eski klasörü söyleyen ileti; `TR_move`'un dolu klasörün yerine konmasını sınayan Go testi.

### 18.16

- **Mevcut durum:** değiştiren işler de göreli yolu okuma sırasıyla çözüyor: başka bir klasörden çalışan program `TR_remove("yanda.txt")` ile programın klasöründeki dosyayı siliyor, `TR_write_file("prog_yolu.txt", ...)` programın klasöründeki dosyayı eziyor; `TR_make_dir` ve `TR_copy`'nin hedefi çalışma klasörüne gidiyor [ölçüldü: main `dc4930ca`].
- **Yapılacaklar (B, Ö1, önce dil tanımı turunun kararı; ölçüm status'un):** `TR_write_file`, `TR_remove`, `TR_move` ve 18.17 yalnız çalışma klasörüne göre; TR testi.
- **Açık karar (Gemini Koşu 5):** okuma üç klasöre bakıyor, değiştirme yalnız çalışma klasörüne; `TR_list_dir`'in mutlak sonucu ve `.TR_exists` ile `TR_remove`'un ayrı dosyayı görmesi bu bakışımsızlıktan. İki yol: (a) her iş için tek kural, çalışma klasörü (Python ve Go gibi; programın yanındaki veri için açık bir taban gerekir, yeni ad); (b) bakışımı koruyup okuma sırasıyla başka klasörde bulunan yolu değiştiren işe verince hata. DENETİM (a)'yı öneriyor. Karar S1'in yönergesinden önce, programın klasöründen okuyan test ve örneklerin sayısı ölçülünce.

### 18.17

- **Yapılacaklar:** `TR_write_lines` ve `TR_LineWriter` (atomik, `.gz`, satır sonu içeren satır hatası, program sonunda kapanma, açık yazıcının yolunu okuma ve ikinci yazıcı hatası); TR testi.
- **Gemini Koşu 5 (2026-10-29), uygulanacak:** ertelenen `.TR_close()` çıkışın türüne göre işler ya da siler; yorumlayıcının ertelenen işe çıkışın türünü veren kancası (Faz 0, D1 ile S1'in ortak yeri).

## 19. Süreç ve kayıt

Kurallar: [`docs/spec/19_surec_ve_kayit.md`](spec/19_surec_ve_kayit.md). **Kapandı** (2026-10-29): ana oturum CLAUDE.md'den türeterek yeniden yazdı, iki tur DENETİM; kararlar Gemini Koşu 5'ten geçti (DENETİM doğruladı, `d31c9226`). Numaralar eski metinle aynı; eski başlıksız "Komut satırında yazım" 19.10, "Görünüm" 19.11 oldu.

### 19.2

- **Mevcut durum:** seçenekler bir sözlükle veriliyor (`TR_command_line({"oran": 0.5, ...})`), sonuç bir sözlük [ölçüldü: main `217ae940`].
- **Yapılacaklar:** yapıyla yazım (13.10'un göçü: 19 kullanım); `tests/surec_ve_kayit_test.tr`, `internal/stdlib/process`'in `TestOptionNameIsTheTextKey`'i, rehber ve örnekler; `TR_descriptions:` değişmezinde bilinmeyen adın denetim hatası; TR testi.

### 19.4

- **Yapılacaklar:** gizli sözcüklerin Go'dan dil dosyalarına (`tr.json`, `en.json`) taşınması [kanıtlı: DENETİM 19-1].
- **Gemini Koşu 5 (2026-10-29), motor zaten böyle:** büyük harf dizisinin sonunda bölme (`HFToken`) [kanıtlı: DENETİM, `process.go:84-86`]. Çoğullar ve Türkçe çoğullar uygulanacak.

### 19.5, 19.7

- **Mevcut durum:** `TR_append` ve `TR_update` `TR_true` döndürüyor [ölçüldü]; kalkması 13.10'un tek 4a işinde.
- **Gemini Koşu 5 (2026-10-29), uygulanacak:** başlangıç kaydında `total` yok; `g.TR_close()`.

### 19.6

- **Gemini Koşu 5 (2026-10-29), uygulanacak:** günlüğün kabul ettiği değerler 13.8'in JSON kümesi; `resume` kaydının `offset`'i.

### 19.10

- **Mevcut durum:** yardım seçeneği yalnız "--help" ve "-h" (Türkçe "--yardım" bilinmeyen seçenek); "help" ve "h" dışında ad ayrılmış değil; ondalıkta Türkçe "sonsuz" okunmuyor, "inf", "-inf" ve "nan" okunuyor; mantık değerinde Türkçe yazım ("doğru", "yanlış") hâlâ okunuyor; yardım varsayılanları kişinin dilinde gösteriyor ("varsayılan: yanlış"); kullanım hatası iletisi gizli adlı seçeneğin değerini uzunluğuna bakmadan iletiye hiç koymuyor (`TestCommandLineSecretLeaks`, `internal/stdlib/process/arguments.go` `errorValue`) [ölçüldü: `TestCommandLineSecretLeaks`, `a93c9e53`; main `85a19c36`, "--help", "--yardım", "--hızlı=doğru", "--üst=sonsuz" ile çalıştırma].
- **Yapılacaklar:** mantık değerinde yalnız "true" ve "false" (`internal/stdlib/process/process_test.go`'nun Türkçe mantık değeri çıkar); yardımın varsayılanları komut satırının yazımıyla; Go testleri.
- **Göç:** yardımın Türkçe adının göçü bitti (depoda yalnız ipucu testi ve iki yorum kaldı) [ölçüldü: main `85a19c36`, `git grep`].
- **Gemini Koşu 5 (2026-10-29), uygulanacak:** yardımda `TR_none` varsayılanı yazılmaz (bugün "(default: none)"). **Açık:** standart hataya yazan yerleşik yok (yol haritası).

### 19.11

- **Mevcut durum (Faz 0.4, 2026-10-30):** iş klasörünün ortam değişkeni "TR_JOB_DIR" (`TR_env_JOB_DIR`); çubuğun dili "TR_LANG"'dan (`TR_env_LANG`) okunuyor [ölçüldü: kod].

## 20. Kontrol noktası

Kurallar: [`docs/spec/20_kontrol_noktasi.md`](spec/20_kontrol_noktasi.md). Taslak (2026-10-29): ana oturum CLAUDE.md'nin "kesilen bir eğitim kaldığı yerden aynı sonuçla sürer" cümlesinden türeterek yeniden yazdı; bölümün çoğu uygulanmış ve testli. Metin DENETİM 20-1'den geçti, bulguları işlendi (`6a525af7`); kararlar Gemini Koşu 1 (`4afda971`), Koşu 4 (`23e7be25`) ve Koşu 2 (20.11, `70a1e6fe`) ile geçti. Kapanış turu yapılmadı.

### 20.1

- **Mevcut durum:** koşu klasörü okuma kuralıyla çözülüyor (kontrol noktası paketinin 178. satırı): başka klasörden çalışan program programın klasöründeki koşuyu sürüp oraya yazıyor [ölçüldü: DENETİM].
- **Yapılacaklar:** yazma kuralı (13); TR testi.

### 20.2

- **Mevcut durum:** sözlük anahtarlarında eski kodlu anahtar kuralı metinden kalktı (bugün sözdizimi hatası). Tam sayılı işaretli tensör (argmax'in sonucu) model olarak kaydediliyor [ölçüldü].
- **Yapılacaklar:** gerçek tam sayı ve mantık tensörlerinin (15'in A'sı) kaydı ve yüklemesi; adlar `TR_named_parameters` ile (13.8a).

### 20.4

- **Mevcut durum:** dondurma (`.TR_requires_grad` üyesi) yok, nokta donma durumunu kaydetmiyor (15.34'ün status'u) (15'in yan değişikliği, uygulanmadı).
- **Yapılacaklar (C, Ö2, 15.34 ile; ölçülmedi):** donma durumunun kaydı ve sürdürmede geri yüklenmesi; TR testi.

### 20.11

- **Mevcut durum:** tam sayılı işaretli tensör ve maske ondalık parametreye sessizce yazılıyor [ölçüldü: DENETİM].
- **Yapılacaklar (A, Ö1, Kol B, 15'in A'sıyla; ölçüm status'un):** cins farkının hatası (15'in A'sı gelince); `TR_AnyTensor` sözlüğü ve parametrenin cinsine daraltma, daraltan hassasiyet dönüşümünde sığmama hatası, nicemlenmiş yaprağa bütün yazma (15.54) (15'in yan değişikliği, uygulanmadı).
- **Açık (Sonraya, 2. kilometre taşında öne çekilmeli):** hazır `.safetensors` dosyaları tablo (RoPE) taşımaz; "bütün yapraklar sözlükte" kuralıyla yükleme alt yapıya yapılır. LoRA alanlarıyla aynı soru (15.34'ün açığı).
- **Gemini 15/2 (2026-10-29), uygulanacak:** nicemlenmiş hassasiyet farkı yüklemede hata, sessiz açma yok (T2, 15.54 ile).

### 20.12, 20.13

- **Mevcut durum:** `TR_load_state`'in mantık değeri dönüşü 13.10'un status'unda. Kontrol noktası cinsli tensör yazmıyor: Go'daki kontrol noktası paketi `TensorWriter.Write`'ı ondalık veriyle çağırıyor, bu yüzden `op.TR_state`'in "step"i ondalık tensör; `TR_Int` için `WriteRaw` ve `ml.TensorEntry.Ham` T1 dilim 2'den beri hazır, paket bağlanmadı (T1 bulgu 11) [kanıtlı: DENETİM istek 57, kontrol noktasının `write.go:173`'ü `p.t.Data()` yazıyor]; karışık cinsli `TR_Dict[TR_String, TR_AnyTensor]` yok (15'in yan değişikliği, uygulanmadı).

### 20.16

- **Mevcut durum:** nazik durdurmada süreç isteğin sinyaliyle biter (Unix'te sinyalle ölür ya da 128+N, Windows'ta `STATUS_CONTROL_C_EXIT`); ikinci istek süreci hemen bitirir; 1.9'un kuralı uygulanmış [ölçüldü: main `85a19c36`, `TestInterruptGracefulExit` ve `TestSecondInterruptExitsImmediately`; `cmd/tr/signals_unix.go:27`, `signals_windows.go:27`].

### 20.17

- **Mevcut durum:** paylaşılan üreteç programa bağlı bir PCG (`math/rand/v2`), `"pcg:00000000000054520000000000002017"` ile başlıyor, durumu kontrol noktasının durum dosyasında `"rng"` alanına yazılıyor ve yüklenince geri konuyor; kesilip sürdürülen eğitim tohumsuz `TR_randn` ve `TR_sample` ile bit bit aynı [test: TestCheckpointResumeBitExact].
- **Yapılacaklar (C, Ö2, 15.45 ile; ölçüm status'un):** `TR_dropout` (15.23d) geldiğinde aynı üreteçten çekmesi.
- **Yapılacaklar (dil tanımı turu 2, 2026-11-01):** cihazın sayaçlı üreteci (15.45) gelince durumu "rng_device" alanına (cihazdan duruma eşlem) yazılır ve geri yüklenir, 15.45 ile birlikte; bugün GPU oynatmasında rastgelelik yok [kanıtlı: DENETİM kol-denetim-tur2-rapor-sonuc.md, T1'in ilk dilimi yalnız CPU üretecinde]. "rng"'nin "pcg:" öneki ve bilinmeyen önekin yükleme hatası (H6953) T1'de geldi [test: checkpoint behavior_security_test.go].
- **Yapılacaklar (T1 birleşmesi, 2026-11-01):** 20.17'nin Test satırındaki TR testi: rastgelelik çekilmeden yazılan ilk noktanın `"rng"`'si `"pcg:00000000000054520000000000002017"`'dir; bugün yalnız Go testlerinde [ölçüldü: `git grep 54520000000000002017`].

### 20.14

- **Mevcut durum:** geri dönüş `.TR_grad`'ları temizlemiyor, donma durumunu döndürmüyor (Gemini Koşu 1, DENETİM). **Yapılacaklar:** 20.14'ün yeni cümlesi; TR testi.

## 21. Dış programlar

Kurallar: [`docs/spec/21_dis_programlar.md`](spec/21_dis_programlar.md). Yazıldı (2026-11-02): ana oturum Gemini Jarvis A'dan (DENETİM istek 36 doğruladı) yazdı; metin DENETİM istek 37'den, düzeltmeleri istek 38 ve 39'dan geçti; main'e girdi (2026-11-02).

### 21.1–21.8

- **Mevcut durum:** uygulanmadı; `TR_run_process` ve `TR_spawn_process` kimlikleri yok [ölçüldü: main `ec26382c`, `definitions.json`].
- **Yapılacaklar:** uygulama Jarvis'in sırasıyla; 1.5'in alt süreç Go testi bununla.

## 22. Bayt ve sayı

Kurallar: [`docs/spec/22_bayt_ve_sayi.md`](spec/22_bayt_ve_sayi.md). Yazıldı (2026-11-02): ana oturum Gemini Jarvis A'dan (DENETİM istek 36 doğruladı) yazdı; metin DENETİM istek 37'den, düzeltmeleri istek 38 ve 39'dan geçti; main'e girdi (2026-11-02).

### 22.1–22.4

- **Mevcut durum:** uygulanmadı; `TR_from_bytes` kimliği ve `tr/audio` kitaplığı yok [ölçüldü: main `ec26382c`, `definitions.json`].
- **Yapılacaklar:** uygulama Jarvis'in sırasıyla.

## 23. Sinyal ve spektrum

Kurallar: [`docs/spec/23_sinyal_ve_spektrum.md`](spec/23_sinyal_ve_spektrum.md). Yazıldı (2026-11-02): ana oturum Gemini Jarvis B'den (DENETİM istek 42 doğruladı, istek 43 kararları okudu) yazdı; metin DENETİM istek 44'ten, düzeltmeleri istek 45'ten geçti (2026-11-02).

### 23.1–23.6

- **Mevcut durum:** uygulanmadı; `TR_rfft`, `TR_stft`, `TR_mel_filterbank` kimlikleri ve `tr/audio` kitaplığı yok [ölçüldü: main `705b721b`, `internal/lang/codes/tr.json`].
- **Yapılacaklar:** uygulama Jarvis'in sırasıyla, konuşmayı yazıya çevirmekle. GPU'da Fourier dönüşümü kendi çekirdeğiyle (23.5); o gelene kadar GPU'da hata. 22.4'ün `TR_Sound` kuruluşu da 23.6'nın iletisi için gerekir.

## 24. Sohbet biçimi

Kurallar: [`docs/spec/24_sohbet_bicimi.md`](spec/24_sohbet_bicimi.md). Yazıldı (2026-11-02): ana oturum Gemini Jarvis B'den (DENETİM istek 42 doğruladı, istek 43 kararları okudu) yazdı; metin DENETİM istek 44'ten, düzeltmeleri istek 45 ve 46'dan geçti (2026-11-02).

### 24.1–24.4

- **Mevcut durum:** uygulanmadı; `tr/chat` kitaplığı yok [ölçüldü: main `705b721b`, `lib/`].
- **Yapılacaklar:** önce 16.2'nin `TR_false` kapatması (ön koşul); sonra `tr/chat` ve Qwen3 biçimi, transformers'la altın testlerle.
- **Yapılacaklar (Jarvis B kararı, `lib/layers.tr`):** `TR_generate`'in `TR_eos`'u `TR_List[TR_Int] = []` olur, çünkü modeller birden çok durdurma parçası taşır: Qwen3-0.6B `[151645, 151643]`, Llama-3.1-8B-Instruct `[128001, 128008, 128009]` [kaynak: DENETİM istek 42, `generation_config.json`]. Çağıranların göçüyle: `lib/layers_test.tr`, `examples/tr/46_hazir_model_ve_uretim.tr`, `tools/gpu/ss_exam.tr`, `docs/guide/10_hazir_katmanlarla_gpt.md` [ölçüldü: DENETİM istek 43]. T1'in alanı.
