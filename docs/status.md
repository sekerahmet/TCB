# Durum

Dil tanımının yeniden yazılan bölümlerine göre bugünkü TR'nin durumu ve kurala varmak için yapılacaklar. Her maddenin kuralı dil tanımındadır ([`docs/spec/`](spec/README.md)); yapılan iş testiyle birlikte buradan silinir. İşlerin sırası [yol haritasındadır](roadmap.md). Bu belgede kimlikler dil tanımının yazımıyla anılır (`TR_if`, 2.3a).

## Uygulamada verilen, dil tanımı turunda denetlenecek kararlar

Uygulama sırasında çıkan çelişki ya da dil tanımının sustuğu yer için ana oturumun kararı aynı gün dil tanımına yazılır; denetimi sıradaki dil tanımı turunda (Gemini ve DENETİM) yapılır. 2026-10-28, 2. katmandan: 3.7a (açmada `TR_List(j)`, karşılaştırma, dolaşma), 3.8 (`.TR_reverse()` liste verir), 3.10 (koşulun geçtiği öbür yerler), 3.12 (mantık değerinden sayı, metinden liste kurulmaz), 3.14 (yön karakterleri, kaptaki JSON, bayt dizisinin metni), 4.5 (fonksiyon içeren kap, isteğe bağlı fonksiyon ve `TR_none`), 4.9 (ifadenin ardındaki `TR_in`), 5.7 (dolaşılamayan JSON türleri), 6.6 (`_` parametre), 12.3 (koşul `TR_Bool`). Gerekçeleri ve kaynaklı emsalleri `docs/spec/rationale/`'de; kaynaklar o turda bağımsız denetlenir.

2026-10-28, 1–9'un amacın dokuz işine karşı tartısından (ana oturum ve DENETİM, `bolum1-9-dokuz-is`): 2.4 (onaltılık yazım bit deseni), 3.1 ve 13.1 (sarılan aritmetik), 3.6a (demet; 6.3'ün birden çok dönüşü demet, H2708 kalkar), 3.7 (demet anahtar), 3.7a (yinelenen JSON anahtarı hata), 4.3 (kesirli üs her makinede aynı), 4.12 ve 13.1 (`TR_unsigned_shift_right`), 5.7 ve 7.8 (`TR_next` ile dolaşma), 5.11 (sözlük üreteci), 13.8 (yapının JSON'u iki yönlü). Uygulanmadılar; bugün: onaltılık hash sabiti sözdizimi hatası ve iletisi ".0 ekle" diyor, hash çarpımının taşma iletisi ondalık öneriyor, yapı dolaşılamıyor, sözlük üreteci "expected a value" veriyor [ölçüldü]. Hepsi 4a'dan önce ya da 4a ile.

## 1. Program

Kurallar: [`docs/spec/01_program.md`](spec/01_program.md). Kararlar DENETİM'den ve Gemini'nin araştırmasından geçti (2026-10-27). 1.3–1.15 uygulandı (katman 1b, DENETİM'den geçti, 2026-10-28); aşağıda yalnız kalanlar.

### 1.1

- **Mevcut durum:** denetleyici modülde ve test dosyasında (`TR_test` içeren dosya, 12.1) en dıştaki tanım dışı deyimi (H1107) ve `TR_const`/`TR_var`'ın sabit olmayan ilk değerini (H1108) denetim hatası sayıyor; hata modülün satırında, aktaran modül hatasının başlığında; ileti deyimi fonksiyona alan yazımı veriyor ve yazım çalışıyor; fonksiyonların atadığı en dıştaki `TR_var` paylaşılan durum sayılıyor (H1114), ileti onu `TR_var x: T? = TR_none` ile başlatıp bir kez kuran fonksiyonu öneriyor (tip bilinmiyorsa `<tip>`). Depodaki modüller ve test dosyaları kurala uyuyor. Dosyanın tanımlamadığı ad (aktarılan ya da genel) sabit sayılıyor; gömülü kitaplık modülleri aktarılırken denetlenmiyor, kuralları depo testinde denetleniyor [ölçüldü: `TestRepoModulesHoldOnlyDefinitions`, `TestModuleRuleSuggestionRuns`, `TestModuleSharedStateSuggestionRuns`, dal `claude/charming-pasteur-q1vsvo`].
- **Yapılacaklar:**
  - Aktarılan adın sabitliği modüller arası tip denetimiyle (4b) bilinir; bugün aktarılan ad sabit sayılıyor.

### 1.2

- **Mevcut durum:** denetleyici yerel değişkenin tipini ilk değerinden çıkarıyor ve tipi kalıcı (3.13): değişmezler, aritmetik ve önek işleç, karşılaştırma, adıyla çağrılan tipli fonksiyonun ve tipi bilinen yapının metodunun dönüşü, yapı kurucusu, liste ve sözlüğün öğe tipi, demet, isteğe bağlı tip (`T?`, `??`'nin asıl tipi), tipi bilinen yapının alanı ve metotta `TR_self`, değer veren `TR_if`/`TR_match` (dalların ortak tipi), liste/metin/sözlük/tensör indeksi ve dilimi, döngü değişkeni (5.7); bilinmeyen `TR_Any` kalıyor. Depodaki programların 1896 ilk değerinden 1114'ünün tipi bilinmiyor (ilk dilimden önce 1212, ilk dilimden sonra 1152); tipsiz parametre, alan ve `->`'sız değerli dönüş denetim hatası (6.1, 7.1; kitaplığın 4c'yi bekleyen yerleri istisna), parametre tipleri yalnız adıyla çağrılan fonksiyonun konumlu argümanlarında denetleniyor [ölçüldü: `TestRepoFirstValueTypes`, `TestOptionalTypes`, `TestFieldTypes`, `TestIfAndMatchValueTypes`, `TestIndexTypes`, `TestLoopVariableTypes`, dal `claude/charming-pasteur-q1vsvo`].
- **Yapılacaklar:** turlarla:
  - çıkarımın kalanı (4a2): yerleşik ve kitaplık çağrılarının dönüşü (bilinmeyenlerin çoğu: 926, `TR_Tensor(...)` tek başına ~218), tipi bilinmeyen işlenenli işleç, kapanış, çoklu tanımın (`TR_var a, b = f()`) öğe tipleri, liste üretecinin sonucu, metot ve kurucu argümanlarının denetimi, kitaplık tiplerinin tanınması;
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
- **1–9 turu (2026-10-29):** kural "döngünün her turu" oldu; motor zaten her turda bakıyor (`vm.go` `loopTurn`), 256 yalnız kilit devrinin sayısıdır (10). Bunu sınayan Go testi yok: yazılacak.

### 1.9

- **Uygulamanın iç çöküşü:** Go çalışma zamanının ölümcül hatası 2 koduyla biter ve 2'nin "kullanım hatası" sözüyle karışır. Yol haritası: motor çöküşü kendi koduyla bitsin.

## 2. Sözcük yapısı

Kurallar: [`docs/spec/02_sozcuk_yapisi.md`](spec/02_sozcuk_yapisi.md). DENETİM'den (üç tur) ve Gemini'nin araştırmasından geçti (2026-10-27). 2.1–2.10 uygulandı (katman 1, iki DENETİM turundan geçti, 2026-10-28); aşağıda yalnız kalanlar.

### 2.1

- **Mevcut durum:**
  - Rehber ve kök belgelerdeki program metni kimlik yazımına çevrildi (2026-10-29). Kalan sınıflı yazım 17.2'nin "içeride" grubudur: tanım dosyalarının içeriği, eş adlar tablosunun kod sütunu, `code_identities.md`, disk biçimi örnekleri [ölçüldü: belgelerde program sınıflarının sınıflı yazımını arayan git grep].
  - Üretilen kod listesi (`docs/codes.md`) başlıklarda program yazımını, yanında sınıflı anahtarı yazıyor; örnekleri kimlik yazımıyla, altında her gömülü dilin görünümüyle (tanım dosyasının örnek alanından, 2026-10-29, Faz 0.5).
  - Satır içi kod (ters tırnak) yalnız dil tanımında ve status'ta denetleniyor; öbür belgelerde yanlış kod gösteren ad (ör. aynı kısa adın başka sınıfı) hiçbir testte düşmüyor.
  - Bölüm 11 eski başlığı, dosyanın dilini ve iç içe aktarmayı anlatıyor (11.3'ün eşzamanlı aktarma cümlesi); bölüm yeniden yazılınca düzelir. 17 yeniden yazıldı (2026-10-29).
- **Yapılacaklar:**
  - ters tırnak içindeki adların denetimi `docs/*.md`'ye genişler (üretilen ve donmuş belgeler ile eş adlar tablosunun kod sütunu hariç): program sınıflarının sınıflı yazımı hata, `.TR_x` yalnız üye, `TR_x:` yalnız adlı argüman.

### 2.3a

- **Mevcut durum:** kimlik kapısı (`TestReviewedSectionsWriteIdentities`) bir `TR_` sözcüğünün herhangi bir yerde geçerli olmasını yeterli sayıyor; yanlış yerde yazılmış `TR_append(x)` geçiyor [kanıtlı: DENETİM].
- **Yapılacaklar:** tam ifade olan parça (`(`, `.` ya da `:` içeren) sözcük çözümleyiciyle okunup yeri denetlenir.

### 2.13

- **Mevcut durum:** `TR_cmd_check --json` çalışıyor (katman 2b, DENETİM'den geçti, 2026-10-28): sözcük çözümleyicinin, ayrıştırıcının, denetleyicinin ve komut satırının hataları ve uyarıları başlangıç ve bitiş konumu, en içteki kod, ilgili konum (H1020) ve düzeltmeyle JSON satırları olarak; satır başındaki `//` → `#` ve `0X1F` → `0x1F` uygulanabilir. Metin kipinde denetleyicinin iletileri sütunu da yazıyor [ölçüldü].
- **Yapılacaklar:** başlık hatasının (H2540) bitişi; ana dosyadaki okunamayan modülün (H2131) sütunu ve bitişi; iç içe modülün `TR_import` hatasının dosyası (yorumlayıcıda `ModuleError` sarmalayıcısı); `TR_cmd_check`'te dosyadan sonraki öbür argümanların durumu (1.4); aktarılan modülün uyarıları [ölçüldü: DENETİM].

### 2.3b

- **Mevcut durum:** harf büyüklüğü olmayan adlar (`TR_struct 学生`, `TR_struct طالب`) uyarı alıyor ve önerilen düzeltme adın kendisi; `TR_const 最大` uyarısız [ölçüldü: DENETİM]. **Yapılacaklar:** büyüklüğü olmayan karakter her biçime uyar (2.3b); TR testi.

## 3. Tipler ve değerler

Kurallar: [`docs/spec/03_tipler.md`](spec/03_tipler.md). Taslak: DENETİM'den geçti, bulguları işlendi (2026-10-27); Gemini'nin araştırması bekleniyor. TR bugün dinamik tipli; tip açıklamalarını yalnız çalışmadan önceki denetleyici kullanıyor. `tests/` altındaki 3. bölüm testleri eski kuralları da sınıyor (karışık tipli liste, doğruluk sınaması, çıplak ad anahtarı, açıklamasız değişkenin tipi); testler koşulmadan önce yeni kurallara göre yeniden yazılır. 3.2, 3.3a, 3.7, 3.7a, 3.8, 3.12 ve 3.14'ün çalışma anı uygulandı (katman 2, DENETİM'den geçti, 2026-10-28; 3.2, 3.8 ve 3.12 bitti, buradan silindi); aşağıda kalanlar. Eski 3.14 (tip adının tek yazımı) 2.3a'ya düştü; eski 3.15 (katalog iletisinin tipi) hata bölümüne gider, `tests/` altındaki 3.15 testi o bölüm yazılınca taşınır.

### 3.3a

- **Mevcut durum:** `TR_Bytes` kuruluyor, okunuyor, birleşiyor; metni "Bytes([104, 105])" [ölçüldü].
- **Yapılacaklar:** dosya ve ağ okumalarının bayt da vermesi (ölçülmedi).

### 3.5

- **Mevcut durum:** denetleyici isteğe bağlı tipi `T?` olarak taşıyor: `TR_none` yalnız isteğe bağlı (ya da tipi bilinmeyen adlı) değişkene, parametreye, alana ve dönüşe veriliyor, öbürü denetim hatası (`TR_var a: TR_Int = TR_none`, `-> TR_Int` fonksiyonda değersiz `TR_return`); `x ?? d`'nin tipi asıl tip, `[1.0, TR_none]` `TR_List[TR_Float?]`. İsteğe bağlı değerin asıl tipin yerinde kullanılması denetlenmiyor (daraltma yok, aritmetikte tipi `TR_Any`) [ölçüldü: `TestOptionalTypes`, `TestTypeCompatibility`, dal `claude/charming-pasteur-q1vsvo`].
- **Yapılacaklar:** açılmamış kullanım denetim hatası; daraltma (4b) yalnız değişmeyen yerel değişkende: sınamanın dalında, `TR_else` dalında, `TR_and`/`TR_or`'un sağ yanında, her yolda biten erken çıkıştan sonra ve asıl tipten atamayla; alana atama daraltmayı bozmaz; ilk değeri `TR_none` olan açıklamasız değişkenin hatası (3.13).
### 3.6

- **Mevcut durum:** karışık tipli liste çalışıyor [ölçüldü].
- **Yapılacaklar:** `TR_List[T]`; öğelerin tipi aynı, boş liste tipini yazar; `TR_none` içeren değişmez `TR_List[T?]`.
- **Tip yazımı ayrışıyor (Faz 0.6a, 2026-10-29):** `TR_List[T]` ayrıştırılıyor, denetleyici `TR_List`'e indirgiyor: `TR_var l: TR_List[TR_Int] = ["a"]` hatasız [ölçüldü: bulut 0.6a]. Yapılacak: öğelerin tipinin denetimi; boş liste tipini yazar; `TR_none` içeren değişmez `TR_List[T?]` (4a2).

### 3.6a

- **Mevcut durum (Faz 0.6b, 2026-10-29):** demet bir değer (`TupleVal`): değişmez `(a, b)`, `(x)` parantez; `t.0`, `t.0.1` okunur, atamak sözdizimi hatası (H0298), köşeli parantezle indeksleme çalışma hatası (H2778: sabit sayıda `t.0`, değişken indekste aynı tipli öğelerde `[t.0, t.1][i]`, farklı tipli öğelerde `TR_var a, b = t` önerir); sınır dışı indeks çalışma hatası, ileti öğe sayısını söylüyor (H2774); `==` öğe öğe, tensör tutan demette çalışma hatası (H2777); `<` sözlük sırasıyla (IEEE), `.TR_sort`, `TR_min`, `TR_max` 13.10'un sırasıyla; öğeleri anahtar olabilen demet sözlük anahtarı, olamayan öğeli demet anahtarda hata (H2773); metni "(1, "a")", tipinin metni "(Int, String)"; `TR_json_string` dizi yazıyor; kontrol noktasında desteklenmeyen değer (H7457). Tipte tek öğeli parantez gruplar: `(TR_Int)` `TR_Int`, `((TR_Int) -> TR_Bool)?` isteğe bağlı fonksiyon tipi [ölçüldü: `tests/tipler_test.tr`'nin 3.6a testleri, `tests/negative/demet_ogeye_atama.tr`, `parser/types_test.go`, `parser/statements_test.go`, `interp/compiler_test.go`].
- **Yapılacaklar:** sınır dışı indeksin, tensör tutan demetin `==`'sinin ve sıralanamayan öğeli `<`'nin denetim hatası: denetleyici demet tipini bugün Herhangi'ye indirgiyor, öğe sayısını ve tiplerini taşımıyor (4a2); `TR_json_parse[T]` (13.8) motorda yok, demet tipli alanın diziden kurulması onunla gelir.

### 3.7

- **Mevcut durum:** anahtar ifade, erişim köşeli parantezle, olmayan anahtar hata (H2647), ondalık anahtar hata (H2646) [ölçüldü].
- **Yapılacaklar:** `TR_Dict[K, V]` ve anahtar tipinin denetimi (4a).
- **Tip yazımı ayrışıyor (Faz 0.6a, 2026-10-29):** `TR_Dict[K, V]` ayrıştırılıyor, `TR_Dict`'e indirgeniyor [ölçüldü: bulut 0.6a].

### 3.7a

- **Mevcut durum:** `TR_JSON` kuruluyor, açılıyor, dolaşılıyor; üst düzey `j["a"] == 1` çalışma hatası. Ama kapta ve eşlemede JSON ile JSON olmayanın karşılaştırması hâlâ sessizce yanlış: `TR_List(j) == [1, 2]`, `TR_List(j).TR_contains(1)`, `[j[0]] == [1]` → false; `TR_match j["a"] { 1 => ... }` `_` koluna düşüyor [ölçüldü: DENETİM ve ana oturum].
- **Yapılacaklar:** kapın içindeki karşılaştırma, `.TR_contains` ve eşleme deseni de çalışma hatası (3. katman, 3A); `TR_Type(j) == TR_JSON` denetim hatası ve genel tiplerin (Any, Object, StructType) kalkması (4a).
- **Mevcut durum:** JSON çözmenin iç içelik sınırı 1000 motorda var (`json_ordered.go` `maxJSONDepth`); kural 1–9 turunda yazıldı.

### 3.10

- **Mevcut durum:** koşulda yalnız `TR_Bool`; başka tip çalışma anında hata veriyor (H2600), ileti açık sınamayı öneriyor (katman 2) [ölçüldü]. Hata ayıklayıcının koşullu durağı hâlâ doğruluk sınamasıyla (`IsTruthy`) okunuyor.
- **Yapılacaklar:** başka tip denetim hatası da olur (katman 4); hata ayıklayıcının koşulu da yalnız `TR_Bool` alır.
- **DENETİM 0.6b (2026-10-29):** demet koşulda (`TR_if (1, 2)`) bugün doğru sayılıyor (eski çoklu dönüş gibi); kurala göre koşul `TR_Bool` olmalı, hata (denetimi 4a2).

### 3.13

- **Mevcut durum:** açıklamasız yerel değişkenin tipi ilk değerinden çıkıyor ve kalıcı: başka tipte atama (`TR_var birikim = 0` sonra `birikim += 0.5`) denetim hatası (H1005); dil tanımının 3.13 hata bloğu ve TR testi bunu sınıyor. Tipi çıkarılamayan ilk değerin değişkeni `TR_Any` kalıyor (1.2) [ölçüldü: `TestAssignmentTypeCheck`, `TestCompoundAssignment`, dal `claude/hopeful-brahmagupta-miw41q`].
- **Yapılacaklar:** çıkarımın kalanı (1.2; bu dilimde isteğe bağlı tip, alan, `TR_if`/`TR_match`, indeks ve döngü değişkeni eklendi); ilk değeri `TR_none` ya da boş değişmez olan değişkende tip açıklamasının zorunluluğu; tam sayı yazımı ondalık beklenen yerde ondalık: denetleyici `[1, 2.5]`'i `TR_List[TR_Float]` sayıyor, motor ilk öğeyi çalışırken `TR_Int` tutuyor [ölçüldü: DENETİM, 2026-10-30] (tam sayı değişkeni `[a, 2.5]`'te karışık sayılıyor, ondalık değil).

### 3.14

- **Mevcut durum:** değerin metni dilden bağımsız ("true", "none", "inf"); tipin metni öğe tipsiz: `TR_Type([1])` "List" [ölçüldü].
- **Yapılacaklar:** "List[Int]" ve "Int?" (öğe tipi ister; 4a).

## 4. İşleçler

Kurallar: [`docs/spec/04_operatorler.md`](spec/04_operatorler.md). Taslak: DENETİM'den geçti, bulguları işlendi (2026-10-27); Gemini'nin araştırması bekleniyor. 4.1–4.4, 4.6, 4.9, 4.11 ve 4.12'nin çalışma anı uygulandı (katman 2, DENETİM'den geçti, 2026-10-28); aşağıda yalnız kalanlar. Tip isteyen denetim hataları katman 4'tedir.

### 4.5

- **Mevcut durum:** ayrı tipler sessizce eşit değil sayılıyor; fonksiyonlar yalnız üst düzey `==` ve `!=`'de hata veriyor (H2613), fonksiyon içeren liste kimlikle karşılaştırılıyor (`[f] == [f]` doğru) [ölçüldü].
- **Yapılacaklar:** fonksiyon içeren kabın karşılaştırması çalışma hatası (3. katman, 3A); ayrı tipler ve fonksiyon içeren değer denetim hatası (4. katman).
- **Gemini Koşu 3 (2026-10-29), uygulanacak:** eşitlenemeyen tipin isteğe bağlısı (tensör, tensör içeren değer) `TR_none` değişmeziyle karşılaştırılabilir (15.5).

### 4.7

- **Mevcut durum:** sağ yanda parantezsiz ikili işleç sözdizimi hatası (H0201); isteğe bağlı tip olmadığı için sol yan her tipten olabiliyor: `1 ?? 2` 1 veriyor [ölçüldü].
- **Yapılacaklar:** sol yan isteğe bağlı değilse denetim hatası.

### 4.9

- **Mevcut durum:** satır sonunda `//`'den sonraki metin bir ifade değilse `#`'i anmayan bir hata çıkıyor: `TR_var x = 5 // bu bir yorum` iki kez "ifadenin değeri kullanılmıyor" veriyor [ölçüldü].
- **Yapılacaklar:** bu durumda da ileti `#`'i önerir.

### 4.10

- **Mevcut durum:** başka tiple `+` ve `*` çalışma anında hata veriyor (H2609, H2614); öğeleri başvuruyla paylaşılan liste sessizce yineleniyor: `[[0] * 3] * 3`'ün üç satırı aynı liste [ölçüldü].
- **Yapılacaklar:** ikisi de denetim hatası olur; paylaşılan listenin iletisi her satırı ayrı kuran yazımı önerir.
- **Mevcut durum:** yineleme 2³¹ bayt ve 2²⁷ öğede kesiliyor (`sequence_ops.go` `maxRepeatBytes`, `maxRepeatItems`); kural artık bellekle denetler (3.6, 15.42); `.TR_append`, `+=` ve sözlüğe eklemenin bellek denetimi de uygulanacak. `tests/operatorler_test.tr:267-269`'un iki satırı (H2610, H2615) kural uygulanınca tersine döner.
- `==`'nin iç içelik sınırı 10000 (`operators.go` `equalityDepth`) bir uygulama sınırıdır; 7.6'nın döngü kuralından ayrıdır.

## 5. Deyimler ve kontrol yapıları

Kurallar: [`docs/spec/05_deyimler.md`](spec/05_deyimler.md). Taslak: DENETİM'den geçti, bulguları işlendi (2026-10-27); Gemini'nin araştırması bekleniyor. Motor 2026-10-29'da dil tanımının bugünkü metnine göre yeniden ölçüldü (main `1073aea`): 5.2, 5.8, 5.9, 5.10 ve 5.13 kurala uyuyor, maddeleri yok. `tests/deyimler_test.tr` yeni kurallara göre yazılmış, 23 testi geçiyor [ölçüldü: dosyanın testleri koşuldu]; eski kuraldan kalan iki yer Faz 0.9'da kalktı: 5.6 testinin `TR_Any` parametresi tipsiz bırakıldı (sayıyı ve metni alır, 4c'yi bekler), 5.14 testindeki yapının alanı tipli.

`TR_elif` bilerek kimliği olmayan addır; kimlik kapısı düzelince bilerek yanlış örnekler listesine girer (2. bölümün girişi). Yeni denetim hataları (5.1'in uyarısı, 5.5, 5.6, 5.12, 5.14) uygulanırken depodaki kodlu dosyalar da düzeltilir; yoksa `TestRepoTRFilesPassCheck` kapısı kalır.

### 5.1

- **Mevcut durum:** `x := 5` sözdizimi hatası, ileti `TR_var`'ı öneriyor; ilk değersiz `TR_var y` sözdizimi hatası; fonksiyonun içinde hiç yeniden atanmayan yerel `TR_var` uyarı vermiyor [ölçüldü: `x := 5`, tek başına `TR_var y` ve yerelinde `TR_var a = 3` olan fonksiyon; denetim ve çalıştırma].
- **Yapılacaklar:** fonksiyonun içinde yeniden atanmayan yerel `TR_var` uyarısı, ileti `TR_const`'u önerir (programın en dışı girmez); TR testi.

### 5.3

- **Mevcut durum:** takas, listeden ve demetten açma ve `_` kurala uyuyor; öğe sayısı tutmazsa H2700, H2701 ya da (demette) H2709 çalışma hatası, sağ yan demet yazımıysa denetim hatası (H1026), H1026 ve H2709 örneğini deyimin adlarından ve kaynaktaki sağ yanından kuruyor (eksikte `_` ekler, fazlada sondaki adları siler); `_` okunamıyor (sözdizimi hatası) [ölçüldü: takas, `TR_var p, q = [1, 2, 3]`, okunan `_`, `TR_var x, y = (7, 8)`; `tests/negative/demet_acma.tr`, `checker/unpack_test.go`, `interp/compiler_test.go`; Faz 0.6b].
- **Yapılacaklar:** yok.

### 5.4

- **Mevcut durum:** on iki bileşik atama, listede yerinde `+=` ve `*=`, hedefin bir kez hesaplanması, alan hedefi ve tanımsız ada atamanın `TR_var`'ı öneren denetim hatası kurala uyuyor. Sonucun tipi denetlenmiyor: tam sayı değişkende `x /= 2` 3.5, `x += 1.5` 8.5 yazıyor, ikisi de denetimden geçiyor; metne `s += 1` ancak çalışırken hata veriyor [ölçüldü: on iki işleç, paylaşılan liste, `l[f()] += 1`, `p.x += 4`, tam sayıya `/=` ve `+= 1.5`; denetim ve çalıştırma]. Tensörde indeksle biriktirme 15.6d'de.
- **Yapılacaklar:** sonucun tipi `x`'in tipi değilse denetim hatası, `/=` iletisi `//=`'yi önerir (tip denetimiyle, 4a); denetleyicinin testi.

### 5.5

- **Mevcut durum:** değer veren `TR_if`'in `TR_else`'i zorunlu (sözdizimi hatası); Python'un koşullu ifadesinin iletisi `TR_if c { a } TR_else { b }`'yi öneriyor; `TR_elif` iletisi `TR_else TR_if`'i öneriyor ama ardından ikinci, yanıltıcı bir sözdizimi hatası ("':' bekleniyordu") geliyor; dalların tipi denetlenmiyor: `TR_if n < 0 { "a" } TR_else { 5 }` denetimden geçiyor [ölçüldü: dört küçük program; denetim ve çalıştırma]. Koşulun `TR_Bool` olması yalnız çalışırken denetleniyor (3.10).
- **Yapılacaklar:** ifade olarak kullanılınca dalların tipi aynı değilse denetim hatası; `TR_elif`'e tek ileti (ayrıştırıcının toparlanması); denetleyicinin testi.

### 5.6

- **Mevcut durum:** değer, çoklu desen, aralık, parantezli ifade, sabit adı ve `_` desenleri ile `TR_return` kolu çalışıyor; `TR_default` iletisi `_`'yi öneriyor. Kapsama denetimi yok: `_`'siz eşleme denetimden geçiyor, eşleşmeyince çalışırken H2702 veriyor. Düz adla yazılan değişken deseni hata vermiyor, değeri karşılaştırıyor. Kolların tipi denetlenmiyor (bir kol metin, öbürü sayı veriyor). Tensör içeren desen denetim hatası vermiyor, bütün tensörü karşılaştırıyor [ölçüldü: altı küçük eşleme programı; denetim ve çalıştırma].
- **Yapılacaklar:** kapsama denetimi (`TR_Bool`'da iki değer, öbür tiplerde `_`); düz adla yazılan değişken deseni denetim hatası, ileti `(x)`'i ya da `_`'yi önerir; ifade olarak kullanılınca kolların tipi aynı; tensör içeren desenin denetim hatası (15.5 ile); denetleyicinin testi.

### 5.7

- **Mevcut durum:** liste, aralık, metin, bayt dizisi (öğeler `TR_Int`), sözlük, `TR_JSON` dizisi ve nesnesi ile akışlı okuyucu dolaşılıyor, biten okuyucu yeniden dolaşılınca hemen bitiyor; iki adlı biçim ve `_` kurala uyuyor; metin, sayı ve yokluk türündeki `TR_JSON` H2664 veriyor; dolaşılan listeye ve sözlüğe ekleme H2703 ve H2704, var olan öğeye atama serbest (bugünkü 5.7 testi). Yapı dolaşılamıyor: `TR_next()` metodu olan yapıda `TR_for` "döngü kurulamaz" çalışma hatası veriyor; dönüş tipi `TR_Int?` yazılamıyor (sözdizimi hatası, 3.5) [ölçüldü: her kaynak türü, JSON'un dört türü, `TR_sequential` okuyucusu iki kez, `TR_next()` metotlu yapı; denetim ve çalıştırma]. Çıkarma kuralı ölçülemedi: listeden ve sözlükten öğe çıkaran metot yok.
- **Yapılacaklar:** `TR_next() -> T?` taşıyan yapının dolaşılması, tek seferlik dolaşıcı, `T` isteğe bağlıysa denetim hatası (ileti öğeyi sarmayı önerir); TR testi.

### 5.11

- **Mevcut durum:** liste üreteci koşullu ve koşulsuz çalışıyor, değişkeni dıştaki aynı adlı değişkeni değiştirmiyor; sözlük üreteci yok: `{k: k * 2 TR_for k TR_in 0..3}` sözdizimi hatası veriyor ("burada bir değer ya da ifade bekleniyordu") [ölçüldü: iki liste ve iki sözlük üreteci; denetim ve çalıştırma].
- **Yapılacaklar:** sözlük üreteci, yinelenen anahtarda son değer; TR testi.

### 5.12

- **Mevcut durum:** blok kapsamı, iç adın dışarıda görünmemesi (denetim hatası), turun kendi kapsamı ve fonksiyon sınırında gizleme kurala uyuyor. Yasak yeniden tanım hata vermiyor: iç bloktaki `TR_var t = 5` dıştaki `t`'yi gizliyor, dıştaki 0 kalıyor; aynı kapsamda ikinci `TR_var b` ve parametreyle aynı adlı gövde değişkeni de geçiyor [ölçüldü: iç blokta, aynı kapsamda ve parametreyle yeniden tanım; denetim ve çalıştırma].
- **Yapılacaklar:** kapsayan ya da aynı kapsamdaki yerel adla yeniden tanım denetim hatası (programın en dışı ve parametreler dahil, sondaki blok sıradan blok); denetleyicinin testi.

### 5.14

- **Mevcut durum:** tanımsız ad çalışmadan önce bulunuyor, ileti en yakın adı öneriyor. Tanımından önce kullanılan değişken, fonksiyon ve yapı denetimden geçiyor, çalışırken "böyle bir ad yok" hatası veriyor; sonra tanımlanan değişkeni okuyan fonksiyon, değişkenden önce çağrılınca da. Modülün en dışı sıraya bağlı: önündeki sabiti okuyan sabit modül çalışırken düşüyor; birbirine döngüyle bağlı iki sabit denetimden geçiyor. Aktarılan modülün fonksiyonu aktaranın en dış adını okuyor: modülde tanımsız ad aktaranda tanımlıyken denetim geçiyor, çağrı aktaranın değerini veriyor; modül tek başına denetlenince aynı ad hata [ölçüldü: tanımsız ad, öne alınmayan değişken, fonksiyon ve yapı, iki modül dosyası; denetim ve çalıştırma]. Takma adsız aktarmadaki tanımsız ad 1.3'te.
- **Yapılacaklar:** tanımından önce kullanma denetim hatası; fonksiyon ve tip tanımları bloğun her yerinden görünür; fonksiyonu çağıran ya da değer olarak alan ifade, fonksiyonun doğrudan ya da dolaylı kullandığı değişkenlerin tanımından sonra gelir (ileti tanımı yukarı almayı önerir); modülün en dışı sıradan bağımsız, sabitler arasında döngü denetim hatası; dosya aktaranın adlarını görmez, denetimde de çalışırken de; denetleyicinin testi.

## 6. Fonksiyonlar

Kurallar: [`docs/spec/06_fonksiyonlar.md`](spec/06_fonksiyonlar.md). Taslak: DENETİM'den geçti, bulguları işlendi (2026-10-27); Gemini'nin araştırması bekleniyor. Motor 2026-10-29'da dil tanımının bugünkü metnine göre yeniden ölçüldü (main `1073aea`): 6.7 ve 6.11 kurala uyuyor, maddeleri yok. Kuralla aynı olanlar: döngüde kurulan kapanış o turun değerini görüyor, fonksiyon dıştaki değişkene atayabiliyor, sırayla verilen argümanın tipi tutmazsa çalışmadan önce bulunuyor, varsayılan her çağrıda yeniden hesaplanıyor, derinlik sınırı 10000 [ölçüldü: küçük programlar; denetim ve çalıştırma].

`tests/fonksiyonlar_test.tr` yeni yazımla yazılmış, 20 testi geçiyor; depoda `TR_it`, `TR_acc`, adsız `TR_func` ve parantezsiz ok kalmadı (yalnız olumsuz fikstürde) [ölçüldü: dosyanın testleri koşuldu; `.tr` dosyalarında git grep]. Eski kurala dayanan testler: eksik, fazla, iki kez verilen argümanı ve varsayılandaki ileri başvuruyu çalışırken yakalayan 6.4 ve 6.5 testleri (denetim hatası olunca `tests/negative/` fikstürlerine taşınır).

### 6.1

- **Mevcut durum:** dönüş tipinin `:` ile yazımı, `-> TR_NoneType`, `def f(x):`, satır başındaki `@f` ve aynı blokta aynı adla ikinci tanım sözdizimi hatası, iletiler TR'deki yazımı söylüyor. Tipsiz parametre (fonksiyon, metot, kurucu) ve değer döndürüp `->` yazmayan `TR_func` denetim hatası (H1109–H1113, 4a1): ileti düzeltilmiş satırı veriyor; tip varsayılandan ya da `TR_return`'lerin ortak tipinden çıkınca düzeltme uygulanabilir, dosyadaki çağrıların ortak tipinden tahmin edilince uygulanamaz (ileti tahmin olduğunu söylüyor), bulunamazsa `x: <tip>` ve nedeni. Depodaki programlar tipli. Tek istisna gömülü kitaplığın 4c'yi bekleyen 29 yeri (genel ve birleşim tipi: birden çok tip alan parametre, alan ve dönüş): adla tutuluyor (dosya, sahip, ad), yalnız kitaplığın kendi dosyası denetlenirken tanı vermiyor, aynı yazım kullanıcının programında hata; liste yalnız küçülür, 4c'de kalkar (`internal/lang/checker/library_exceptions.go`, nedenleri `internal/testdata/typed_code_known.txt`) [ölçüldü: `TestUntypedFunctionParameter`, `TestUntypedMethodParameter`, `TestMissingReturnType`, `TestTypedSignatureSuggestionRuns`, `TestCheckJSONTypedSignatureFixesMakeTheFileClean`, `TestLibraryExceptionsAreUsed`, `TestRepoProgramsAreTyped`, dal `claude/charming-pasteur-q1vsvo`].
- **Yapılacaklar:** kitaplığın 29 istisnası 4c'de (tip parametresi ve birleşim tipi) kalkar; argümanın parametre tipine göre denetimi metotta, kurucuda ve adlı argümanda (4a2).
- **Dil tanımı düzeldi (2026-10-29):** aynı adla ikinci tanım artık tanımda da sözdizimi hatası (1.3); motor zaten öyle.

### 6.2

- **Mevcut durum:** gövdenin son ifadesi dönüş değeri değil (örtük dönüş kalktı), programın en dışında `TR_return` sözdizimi hatası; eksik dönüş denetlenmiyor: dönüş tipi `TR_Int` olan fonksiyonda `TR_return`'süz yol denetimden geçiyor, çalışınca yokluk veriyor; fonksiyon tipi beklenen fonksiyon ifadesinin blok gövdesinde de [ölçüldü: dalı biten ve bitmeyen fonksiyonlar, `TR_throw`, sonsuz döngü, `TR_match` ve `TR_exit` ile biten yollar; denetim ve çalıştırma].
- **Yapılacaklar:** bitmeyen yol denetim hatası, ileti `TR_return`'ü önerir; bitiren deyimler: `TR_return`, `TR_throw`, `TR_exit`, çıkışı olmayan sonsuz döngü, bütün dalları biten `TR_if` ve bütün kolları biten `TR_match`; denetleyicinin testi.

### 6.3

- **Mevcut durum (Faz 0.6b, 2026-10-29):** `TR_return a, b` bir demet döndürüyor: sonuç açılıyor, tutuluyor, argüman oluyor, listeye konuyor, `t.0` ile okunuyor; H2708 kalktı. Açmada sayı tutmazsa dönüşü `-> (A, B)` diye yazılmış fonksiyonun doğrudan çağrısında denetim hatası (H1026), öbür yerlerde (fonksiyon değeri, dönüş tipi yazılmamış fonksiyon) çalışma hatası (H2709); iki ileti de örneğini deyimin adlarından ve sağ yanından kuruyor [ölçüldü: `tests/tipler_test.tr`'nin 3.6a testi, `tests/fonksiyonlar_test.tr`'nin 6.3 testleri, `checker/unpack_test.go`, `interp/compiler_test.go`].
- **Yapılacaklar:** dönüş tipinin öğe tipleriyle açılan adların tipi ve dönüş tipi yazılmamış fonksiyonun sayısı (4a2).

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
- **Yapılacaklar:** fonksiyonun metni tipinin metni ("(Int) -> Bool"), `TR_Type(f)` fonksiyon tipi; parametre ve dönüş tiplerinin denetimi (4a); kimlik listesindeki genel fonksiyon tipi kalkar (3.11); TR testi.
- **Tip yazımı ayrışıyor (Faz 0.6a, 2026-10-29):** `(T) -> R`, `() -> R`, `-> (A, B) -> C` ayrıştırılıyor; denetleyici `TR_Function`'a indirgiyor, parametre ve dönüşü denetlemiyor (4a2).

### 6.10

- **Mevcut durum:** tip parametresi yok: `TR_func baştaki[T](l: TR_List[T]) -> T?` sözdizimi hatası ("'(' bekleniyordu"); çağrıda tip yazımı (`f[TR_Int]([3])`) denetimden geçiyor, çalışırken "Fonksiyon tipi indekslenemez" veriyor [ölçüldü: main 1073aeab].
- **Yapılacaklar:** tip parametreleri ve çağrıda çıkarımı, tip yazımının listenin öğesinin çağrısından ayrılması; kısıtsız `T`'de yalnız her tipte geçerli işler, öbürleri kısıt ister (13.10, 7.8); TR testi.
- **Tip yazımı ayrışıyor (Faz 0.6a, 2026-10-29):** tip yerinde `Kutu[TR_Int]` ve `T` ayrıştırılıyor, `Kutu`'ya indirgeniyor; tanımdaki tip parametresi (`TR_func f[T](...)`) hâlâ sözdizimi hatası (4c).

## 7. Yapılar

Kurallar: [`docs/spec/07_yapilar.md`](spec/07_yapilar.md). Taslak: DENETİM'den geçti, bulguları işlendi (2026-10-27); Gemini'nin araştırması bekleniyor. Motor 2026-10-29'da dil tanımının bugünkü metnine göre yeniden ölçüldü (main `1073aea`); her kuralda bir ayrılık kaldı. Kuralla aynı olanlar: nesne başvuruyla paylaşılıyor, eşitlik alanlardan, metin kurma yazımıyla ve döngüde "...", çağrısız metot nesneye bağlı bir fonksiyon; yapı tanımının sözdizimi hataları (kalıtım ve Python sınıfı iletileri, yinelenen alan ve metot, alan sırası), metotta düz alan adı, parametre olarak `TR_self`, kurucunun değerli dönüşü ve dönüş tipi, durumlu tipin ve arayüzün tanım ve desen hataları çalışmadan önce bulunuyor [ölçüldü: küçük programlar ve `tests/negative/` yapı, durumlu tip ve arayüz fikstürleri; denetim ve çalıştırma]. `tests/yapilar_test.tr` 24 testi geçiyor; alanları tipli (Faz 0.9); verilmeyen alanı, olmayan alanı çalışırken yakalayan 7.2, 7.4 ve 7.5 testleri eski kurala dayanıyor (denetim hatası olunca `tests/negative/` fikstürlerine taşınır), 7.6 testi StructType'ı sınıyor.

Yapının tipi olan genel tip (StructType) kalkar; yapının tipi `TR_Type`'tır (3.11). Örneklerdeki eski genel tipler de kalkar: StructType, Function, BuiltinFunction, Object ve `TR_Any` (`examples/tr/12_tip_adlari.tr`, `14_yapilar_ve_metotlar.tr`) [ölçüldü: örneklerde git grep].

Kütüphanenin bedeli: `.tr` dosyalarında 63 yapı tanımı (`lib/`'de 16) [ölçüldü: satır başındaki `TR_struct` için git grep]; alanlar ve kurucu parametreleri tipli yazıma çevrildi (Faz 0.9, 6.1 ve 7.1'e bakın). Alan daraltılmadığı için (3.5) `lib/`'deki 9 `TR_self.alan != TR_none` sınaması [ölçüldü: `lib/` için git grep] yerel kopyaya ya da `?.`'ya çevrilir; "yoksa varsayılan ata" kalıbının 4 yeri 3.5'in atamayla daraltmasıyla geçerli kalır; `lib/layers.tr`'nin kurucuları 7.4'e uyuyor [ölçüldü: DENETİM].

JSON'dan tipli yapıya doğrudan okuma (alan adı eşlemesiyle; Rust'ın serde'si, Python'un pydantic'i gibi) henüz hiçbir bölümde yok; sonraki bölümlerde tasarlanır.

### 7.1

- **Mevcut durum:** tipsiz alan (yapının alanı ve durumlu tipin durum alanı) denetim hatası (H1109–H1111, 4a1): ileti tipi varsayılandan (uygulanabilir düzeltme) ya da kurucusuz yapının kurma çağrılarından (tahmin, uygulanamaz) öneriyor, bulunamazsa `ad: <tip>` ve nedeni; varsayılanlı alandan sonra varsayılansız alan, yinelenen alan ve metot, kalıtım yazımları sözdizimi hatası; yinelenen metot tek ileti veriyor; yapının tip parametresi yok: `TR_struct Kutu[T]` sözdizimi hatası. Kitaplığın 4c'yi bekleyen iki alanı (GPT'nin karışık değerli ayar sözlüğü, Muon ya da NorMuon tutan alan) 6.1'in istisnası [ölçüldü: `TestUntypedStructField`, `TestUntypedEnumField`, `tests/negative/tipsiz_imza.tr`, dal `claude/charming-pasteur-q1vsvo`].
- **Yapılacaklar:** kitaplığın iki alanının istisnası 4c'de kalkar; yapının tip parametresi (6.10 ile); TR testi.

### 7.2

- **Mevcut durum:** sırayla ve adla kurma, her kurmada yeniden hesaplanan varsayılan kurala uyuyor; varsayılanı olmayan alan verilmezse, fazla ya da bilinmeyen argümanda ve iki kez verilen alanda ancak çalışırken hata (H2750, H2419, H2418, H2420); alanın tipi kurmada denetlenmiyor: `TR_Int` alana metin veriliyor [ölçüldü: iki alanlı yapıyla altı kurma; denetim ve çalıştırma].
- **Yapılacaklar:** zorunlu alanın verilmemesi ve 6.5'in argüman hataları denetim hatası; alanın tipinin denetimi; denetleyicinin testi.

### 7.3

- **Mevcut durum:** alan `TR_self.x` ile, metot `TR_self.m()` ile; metotta düz alan ya da metot adı dıştaki ada çözülse de çözülmese de denetim hatası, ileti `TR_self.x`'i öneriyor; `TR_self` parametre yazılınca sözdizimi hatası; nesnesiz metot çalışırken H2751. Python alışkanlığıyla ilk parametreye düz self sözcüğü yazılınca hata yok: o sıradan bir parametre sayılıyor, çağrı eksik argüman hatası veriyor [ölçüldü: dışta aynı adlı değişkenli metot, `TR_self` ve düz self parametreli metot, `tests/negative/yapi_metotta_duz_alan.tr`; denetim ve çalıştırma].
- **Yapılacaklar:** metodun ilk parametresi Python'un self'iyse sözdizimi hatası, ileti `TR_self`'i önerir; TR testi.

### 7.4

- **Mevcut durum:** kurucu varken kurma onun imzasıyla, varsayılanlı alanlar gövdeden önce tanım sırasıyla, değerli `TR_return` ve dönüş tipi sözdizimi hatası; kurucunun imzasına uymayan kurma ancak çalışırken bulunuyor (H2410, H2414); kurucunun atamadığı varsayılansız alan yokluk kalıyor, atanmadan okunan alan ve bütün alanlar atanmadan metot çağrısı hata vermiyor [ölçüldü: varsayılansız alanı atamayan, onu okuyan ve metot çağıran kurucu; denetim ve çalıştırma].
- **Yapılacaklar:** kurucunun sonunda her yolda varsayılansız her alanın atanması, atanmadan okuma ve önceden metot çağrısı denetim hatası; kurucunun imzasıyla kurma 6.5'in denetim hatalarıyla; denetleyicinin testi.

### 7.5

- **Mevcut durum:** alan okunuyor ve atanıyor, `TR_const` nesnenin alanı atanabiliyor; olmayan alan ve metoda atama ancak çalışırken bulunuyor (H2468, H2405, H2404); ileti en yakın adı öneriyor ama alanla eşit uzaklıktaki metodu da seçebiliyor (`p.y` için `m`), yakın ad yoksa alanı eklemeyi öneriyor; alanın tipi denetlenmiyor: `TR_Int` alana metin atanıyor [ölçüldü: bir alanlı ve bir metotlu yapıda okuma ve atama; denetim ve çalıştırma].
- **Yapılacaklar:** alan tipinin denetimi; olmayan alan ve metoda atama denetim hatası, ileti en yakın alanı önerir; denetleyicinin testi.

### 7.6

- **Mevcut durum:** paylaşım, alanlardan eşitlik, kurma yazımıyla metin ("Nokta(x: 9.0, y: 4.0)"), kendine bağlanan nesnede "...", döngüde `==`'nin çalışma hatası (H2771) ve `TR_Type(p) == Nokta` kurala uyuyor; yapının tipi StructType; fonksiyon ya da tensör alanlı yapının `==`'si denetim değil çalışma hatası (H2770), aynı nesne kendisiyle karşılaştırılınca hata yok, doğru veriyor [ölçüldü: paylaşılan nesne, kendine bağlı nesne, fonksiyon alanlı nesnenin kendisiyle karşılaştırması; denetim ve çalıştırma].
- **Yapılacaklar:** fonksiyon ya da tensör alanlı yapının `==`'si denetim hatası (aynı nesnede de); yapının tipi `TR_Type` (3.11); denetleyicinin testi.

### 7.7

- **Mevcut durum:** `TR_enum`, durumların kurulması, çağrısız alanlı ve çağrılı alansız durumun hatası, kurucunun yokluğu, değişmezlik, eşlemede konumlu desen ve alan bağlama, adlı alan ve koşullu kol hatası, eşitlik ve metin uygulandı (bugünkü 7.7 testleri ve fikstürleri). Kalanlar: tip parametresi yok (`TR_enum Cevap[T]` sözdizimi hatası); kapsama denetimi yok (`_`'siz, eksik durumlu eşleme çalışırken H2702); dıştaki adla aynı bağlanan ad hata vermiyor, dıştakini gizliyor (5.12); alanın tipi denetlenmiyor (`TR_Int` alana metin); tensör alanlı durumun `==`'si denetim değil çalışma hatası (H2770) [ölçüldü: dört durumlu tip programı; denetim ve çalıştırma].
- **Yapılacaklar:** tip parametresi (6.10 ile); kapsama denetimi (5.6); bağlanan adın 5.12 hatası; alan tipinin denetimi; tensör içeren durumlu tipin `==`'si ve `==` isteyen deseni denetim hatası (15.5); TR testi.

### 7.8

- **Mevcut durum:** `TR_interface`, boş arayüzün ve alanlı ya da gövdeli arayüzün sözdizimi hatası, yapının ve durumlu tipin yapısal karşılaması, nesnenin kendi metoduna giden çağrı, arayüz tipli parametre ve alan, `TR_Type` sınamasıyla dalda asıl tipin alanı çalışıyor. Denetim yok: metodu eksik değer ancak çağrıda "alanı ya da metodu yok" çalışma hatası veriyor, imzası tutmayan değer (`TR_String` parametreli metot) hatasız geçiyor; arayüz değerinin `==`'si hata vermiyor; arayüz kısıtlı tip parametresi (`TR_func g[T: Katman]`) sözdizimi hatası [ölçüldü: bir arayüz, üç yapı ve bir durumlu tiple karşılama, daraltma ve karşılaştırma; denetim ve çalıştırma].
- **Yapılacaklar:** karşılamayan değer denetim hatası, ileti eksik ya da imzası tutmayan metotları söyler; arayüz değerinin karşılaştırması denetim hatası (4.5); tip parametresinde kısıt (6.10 ile); daraltmanın denetleyicide 3.5'in koşullarıyla yapılması; denetleyicinin testi.
- **Karar (2026-10-29):** yapısal karşılama kalır; açık bildirimle karşılaştırması gerekçede (7.8). Eski gerekçe "kütüphanenin bugünkü yapılarını değiştirmeden" motora dayanıyordu, çıkarıldı.

## 8. Hata yönetimi

Kurallar: [`docs/spec/08_hata_yonetimi.md`](spec/08_hata_yonetimi.md). Taslak (2026-10-28): ana oturum yazdı, DENETİM'den geçti (1–7 ile tutarlılık; bulguları işlendi; Ç1: yakalamanın adı zorunlu, 2.9b'den `TR_error` kalktı, Ahmet 2026-10-28), Gemini'nin araştırması bekleniyor. `tests/` altındaki 8. bölüm testleri eski kuralı da sınıyor (yakalanan hatanın metin olması, her değerin fırlatılıp metne çevrilmesi, adsız yakalamada eski ad); kural uygulanırken yeniden yazılır.

### 8.1

- **Mevcut durum:** `TR_try`/`TR_catch` çalışıyor, `TR_catch` zorunlu; `TR_try` ifade olarak sözdizimi hatası; `TR_finally` bilinmeyen kimlik; Python'un `except`'i ipucu alıyor [ölçüldü].
- **Yapılacaklar:** ifade olarak `TR_try`; adsız `TR_catch` sözdizimi hatası (ileti `TR_catch e`; göç: depodaki 7 adsız yakalama ve 4 `TR_error`), `TR_error` kimliği kalkar; parantezli adın ve Python'un `except`/`finally`'sinin iletileri.

### 8.2

- **Mevcut durum:** her değer fırlatılabiliyor ve metne çevriliyor (`TR_throw {"kod": 5}` yakalanınca metin); `raise` sözdizimi hatası değil, denetleyicinin tanımsız ad önerisi `TR_throw`'u öneriyor; neden taşınamıyor [ölçüldü: DENETİM].
- **Yapılacaklar:** fırlatılabilen dört tür (metin, ileti, durumlu tip değeri, `TR_Error`), başka tip çalışma hatası; `TR_Error(..., TR_cause: e)`; `raise`'in iletisi.
- **Gemini Koşu 7 (2026-10-29), uygulanacak:** yeniden fırlatmada çağrı yığını korunur.

### 8.3

- **Mevcut durum:** yakalanan hata bir `TR_String`; ileti kodlu hatada kod kayboluyor, yakalayan yalnız metni görüyor [ölçüldü].
- **Yapılacaklar:** `TR_Error` tipi ve üyeleri (`TR_code`, `TR_message`, `TR_value`, `TR_cause`); `TR_value`'nun kapalı birleşim tipi (4. katman); depodaki `TR_catch` kullanımları üyelere çevrilir.
- **Gemini Koşu 7 (2026-10-29), uygulanacak:** kapalı birleşimin yerel kopyası; hatayla biten kol eşlemenin tipine katılmaz (denetimi 4a2).

### 8.4

- **Mevcut durum:** denetim karakterleri kaçırılıyor, ama `\x` yazımıyla: `TR_throw "a\u{1b}[2Jb\u{7}c"` → `a\x1b[2Jb\x07c` [ölçüldü: DENETİM].
- **Yapılacaklar:** 2.6'nın kaçışı (`\u{1b}`).

### 8.5

- **Mevcut durum:** `TR_Message` ve `TR_error_code` çalışıyor (`H6903`); düz metin hata da kodlu: `TR_error_code { TR_throw "x" }` → H2441 [ölçüldü: DENETİM].
- **Yapılacaklar:** düz metin hatanın kodu `TR_none`; nedenler zincirinde dıştan içe ilk kod.
- **Gemini Koşu 7 (2026-10-29), uygulanacak:** kodsuz hatada yeniden fırlatma (bugün sessizce yokluk veriyor [kanıtlı: DENETİM]). Programın ayırması beklenen hataların kodları kendi maddelerine yazılacak (Koşu 4, 6, 7).

### 8.6

- **Mevcut durum:** yakalanmayan hata yalnız fırlatıldığı satırı gösteriyor, çağrı yığını yok [ölçüldü].
- **Yapılacaklar:** çağrı yığını, en son çağrı en altta.
- **Gemini Koşu 7 (2026-10-29), uygulanacak:** yakalanmayan hatada nedenler zinciri.

## 9. Ertele

Kurallar: [`docs/spec/09_ertele.md`](spec/09_ertele.md). Taslak (2026-10-28): ana oturum yazdı, DENETİM bekleniyor. `tests/hatalar_ertele_test.tr` ve `examples/tr/16_ertele.tr` eski kuralı sınıyor (fonksiyona bağlı kapsam, çağrı yazımı, birleşik hata metni); kural uygulanırken yeniden yazılır.

### 9.1

- **Mevcut durum:** ertele fonksiyona bağlı: döngüde kaydedilen işler fonksiyonun sonunda toplu çalışıyor ("tur 0, tur 1, tur 2, son, kapat 2, kapat 1, kapat 0"), `TR_if` içindeki de fonksiyonu bekliyor; ertelenen bloğun içinde kaydedilen ertele hiç çalışmıyor, sessizce kayboluyor; ters sıra doğru; çağrı yazımı (`TR_defer TR_print(x)`) ve blok yazımı ikisi de çalışıyor. Depoda 31 ertele: 11 çağrı yazımı, 20 blok, 3 dosyada [ölçüldü].
- **Yapılacaklar:** kapsamın blok oluşu; çağrı yazımı sözdizimi hatası, ileti blok yazımını önerir; göç: 11 çağrı yazımı bloğa, fonksiyona bağlı kapsamı sınayan testler ve örneğin "her tur için ayrı fonksiyon" bölümü. Sayım türü (kuralı uygulayan katman): fonksiyon, test, görev ya da program gövdesi dışındaki blokta kaydedilen ertele; durumu geri koyan bir ertele (`TR_default_device` gibi) orada sessizce erken çalışır. Bugün depoda tek yer: `tests/hatalar_ertele_test.tr:73` [ölçüldü: DENETİM].

### 9.3

- **Mevcut durum:** blok çalıştığı anda okuyor (`TR_defer TR_print(x)` sonra `x = 2` → 2); ertelenen bloktaki `TR_return 5` sessizce yok sayılıyor, fonksiyon 1 döndürüyor; `TR_break` ve `TR_continue` sözdizimi hatası, iletileri `TR_return`'ü öneriyor [ölçüldü: DENETİM].
- **Yapılacaklar:** ertelenen bloktan `TR_return` ile çıkışın hatası; üçü de sözdizimi hatası (5.13 ile; dosya okunurken bulunan hata sözdizimi hatasıdır, katman 3'ün 1. kararı); iletiler `TR_return`'ü önermez.

### 9.4

- **Mevcut durum:** hatasız kapsamda ertelenen işin hatası yakalanıyor; iki hata tek metinde birleşiyor ("govde hatasi; error in defer as well: ertele hatasi") [ölçüldü].
- **Yapılacaklar:** ilk hata korunur, öbürleri `e.TR_suppressed`'e (8.3 ile).

## 10. Eşzamanlılık

Kurallar: [`docs/spec/10_eszamanlilik.md`](spec/10_eszamanlilik.md). **Kapandı** (2026-10-29, DENETİM 11-18-2). 2. tur (2026-10-29): DENETİM 10-1'in bulgularıyla (süreyle devir, uzun yerleşiklerin kilidi bırakması, iptal, 2.8'in istisnası, verilen sıradaki ilk hata). 3. tur: DENETİM 10-2'nin bulgularıyla (gecikme sözü 10 ms, izin depoda ve hepsi birden, geri çağırmada bırakılır; iptal 1.5'in noktaları ve yerleşik ayrımıyla; alt görevi bekleyen de iptal hatası alır; iptal ve zaman aşımı ileti kodları). Yeni adların gösterim adları onaylandı (2026-10-29): `TR_Task` Görev, `TR_wait` sonucu_bekle, `TR_timeout` zaman_aşımı, `TR_cancel` iptal_et, `TR_done` bitti_mi; kodları uygulandıkları gün kayda girer (17.4). `TR_sleep`'in Türkçe adı "bekle"den "uyu"ya döndü (görevi beklemekle karışmasın diye; uygulandı).

### 10.1

- **Mevcut durum:** `TR_async` bir ifade değil: `TR_var g = TR_async f(3)` sözdizimi hatası; görevin sonucu alınamıyor [ölçüldü: main `6a525af7`].
- **Yapılacaklar:** görev değeri `TR_Task[T]` ve `.TR_wait()`; TR testi.

### 10.2

- **Mevcut durum:** kilit süreyle değil, döngülerde 256 turda bir (bu bir uygulama sayısıdır; durdurma ve iptal her turda bakılır, 1.5) ve kullanıcı fonksiyonu çağrısında devrediliyor; tensör işlemleri, geri yayılım, iyileştiricinin adımı, örnekleme ve GPU beklemeleri kilidi tutuyor (`internal/stdlib/ml`'de kilidi bırakan sarmalayıcı yalnız `TR_save_tensors`'ta). 60 matris çarpımı sürerken 5 ms'de bir uyanan görev 301 ms bekledi [ölçüldü: DENETİM 10-1, main `5ec35e05`].
- **Mevcut durum (bırakanlar):** `TR_sleep`, `TR_input`, dosya okuma ve yazma, `TR_wait_all`, modül yükleme beklemesi, veri kümesini açma ve okuma, dosya işleri, `TR_save_tensors`, `TR_load_tensors`, parçalayıcıyı yükleme ve büyük girdide kodlama [kanıtlı: DENETİM 10-1].
- **Yapılacaklar:** süreyle devir (5 ms); uzun yerleşiklerin kilidi bırakması ve deponun okuma ve yazma izni (hepsi birden, geri çağırmadan önce bırakılır); gecikme sözünün Go testi. Önce ve sonra ölç: küçük tensör işlemli döngüde izinlerin bedeli (tahmin ~%5) [hipotez: DENETİM 10-2]. Geri çağıran yerleşikler: `TR_backward` (kullanıcı türeviyle), `TR_fuse`, anahtarlı sıralama, `TR_grad_check`. GPU'da iznin kuyruğun bitişine bağlanması [hipotez: GPU yolu açılmadı].

### 10.3

- **Mevcut durum:** listedeki ağ istekleri, dış süreç ve sunucu beklemeleri bugün yerleşik olarak yok [ölçüldü: DENETİM 10-1]; kural gelecek yerleşikler için (yol haritasının "Ağ" maddesi).

### 10.4

- **Mevcut durum:** `TR_no_grad` bloğunda açılan görev türev izliyor: `TR_no_grad { TR_async { ... w * 2.0 ... } }`'in sonucu türevli [ölçüldü: main `8b115692`]; hesap türünün mirası 15.4'ün status'unda (15'in yan değişikliği, uygulanmadı).
- **Yapılacaklar:** görev açıldığı yerin türevsiz kipini ve hesap türünü alır; TR testi.

### 10.5

- **Yapılacaklar:** `TR_wait_all(görevler)`'in sonuç listesi ve verilen sıradaki ilk hatası; TR testi.

### 10.6

- **Mevcut durum:** beklenmeyen görevin yakalanmamış hatası yazılıyor, program sürüyor ve 1 koduyla bitiyor [ölçüldü].

### 10.7

- **Mevcut durum:** sabit hızlı döngünün süre ölçmesi için 13.1'in `TR_monotonic_ns`'i kod kaydında yok [ölçüldü: DENETİM 10-1].
- **Yapılacaklar:** zaman aşımı, `TR_done`; TR testi.
- **Açık:** zaman aşımı ve iptal hatalarının ileti kodları tanımsız (Gemini Koşu 4); 1.6'nın "durdurma istendi" hatasında da aynı boşluk. Adlı ayırma (yeni ad) Sonraya.

### 10.8

- **Mevcut durum:** paylaşılan listeye görevlerden ekleme sırası zamana bağlı ([4, 3, 2, 1, 0]) [ölçüldü].

### 10.9

- **Mevcut durum:** iptal 1.5'in noktalarını kullandığı için döngüde 256 tur sürebilir; ağır turlu döngüde bu saniyeler olabilir [hipotez: DENETİM 10-3]. 1.5'in noktası süreyle de sınırlanabilir; 1. bölümün işi.
- **Yapılacaklar:** iptal (`TR_cancel`) 1.5'in noktalarıyla ve yerleşik ayrımıyla, alt görevlere geçmesi, ertelenen işlerin çalışması, iptalin yakalanmaması, iptal ve zaman aşımı ileti kodları (8.5); TR testi.
- **Gemini Koşu 4 (2026-10-29):** iptal edilen görevin durmasını bekleme, görevin alt görevlerini bekleyip sonra ertelenenleri çalıştırması, `TR_wait_all`'da `TR_suppressed`, 10.6'nın yazma anı, 9.1'in son deyim `TR_defer` hatası, 9.4'ün durdurmada ertelenen hatası, 5.4'ün değerlendirme sırası uygulanacak.

## 11. Modüller

Kurallar: [`docs/spec/11_moduller.md`](spec/11_moduller.md). **Kapandı** (2026-10-29): ana oturum CLAUDE.md'den türeterek yeniden yazdı, iki tur DENETİM. Kitaplık modüllerinin İngilizce yolları (11.6) onaylandı (2026-10-29): `tr/layers`, `tr/optimizers`, `tr/pretrained`, `tr/training`.

### 11.1

- **Mevcut durum:** modülün en dışındaki deyim denetim hatası (H1107, 1.1); olumsuz fikstür `tests/negative/modul_en_dis.tr` [ölçüldü: `TestModuleTopLevelHoldsOnlyDefinitions`, 4a1 ilk dilim].
- **Yapılacaklar:** yok (1.1'e bağlı kalanlar 1.1'in maddesinde).
- **Gemini Koşu 5 (2026-10-29), uygulanacak:** göreli modül yolunun programın klasörüne yedek yolu kalkar; harf büyüklüğü diskteki adla aynı olmalı.

### 11.2

- **Mevcut durum:** `TR_as` bir `TR_Dict` bağlıyor; köşeli ayraçla erişim (m["PI"]) çalışıyor; modülde olmayan bir ad (m.yok) denetimden geçiyor ve çalışırken `TR_none` veriyor [ölçüldü: main `a9a1ba14`].
- **Yapılacaklar:** modül değeri; adların tipi denetimde; köşeli ayraçla erişim ve olmayan ad denetim hatası, en yakın adı öneren ileti; olumsuz fikstür. `tests/moduller_test.tr`'den yalnız sözlüğe özgü doğrulamalar (tip, uzunluk, anahtarlar, köşeli ayraç) çıkarıldı; nokta ile erişim ve modüle özel adın hata kodu kaldı.

### 11.3

- **Gemini Koşu 5 (2026-10-29), uygulanacak:** "başka görev modülü yüklerken bekler" cümlesi kalktı (modüller program çalışmadan yüklenir); motor davranışı ölçülecek.

### 11.4

- **Yapılacaklar:** `tests/negative/hatali_modul_aktar.tr` bugün çalışma hatası bekliyor (RUN-ERROR); 11.4 uygulanınca hata çalışmadan önce verilir, fikstürün beklentisi değişir [kanıtlı: DENETİM 11-1].

### 11.5

- **Yapılacaklar:** dosya bilgisinin görevde tutulması (bir görevin modül yüklemesi öbür görevlerin yollarını değiştirmez); TR testi.
- **Gemini Koşu 5 (2026-10-29), uygulanacak:** dosya bilgisi çağrınındır (görevde çalışan modül fonksiyonu kendi modülünün klasörüyle okur).

### 11.6

- **Mevcut durum (Faz 0.2, 2026-10-29):** kitaplığın dört modülü İngilizce yollarda: `tr/layers`, `tr/optimizers`, `tr/pretrained`, `tr/training` (`lib/`); eski Türkçe yol H2478 verir (`tests/negative/kitaplik_turkce_harf.tr`). Eski yollar göçten önce 90 dosyada 976 satırda geçiyordu; şimdi yalnız `docs/archive/`'da ve o negatif testte [ölçüldü: `git grep`].

### 11.8

- **Mevcut durum:** iki aktarım aynı adı getirince sonraki sessizce kazanıyor; dosyanın kendi tanımı aktarılan adı sessizce gölgeliyor [ölçüldü: main `a9a1ba14`]. Depodaki 135 dosyanın 50 aktarmasında çakışma yok; kural göçsüz girer [ölçüldü: DENETİM 11-1].
- **Mevcut durum (atama):** takma adsız aktarılan ada atama (`PI = 7`) aktaran dosyada sessizce yerel bir kopya kuruyor, modülün değeri değişmiyor; `m.PI = 5` modülün değerini değiştiriyor ve öbür takma adlar da görüyor [ölçüldü: DENETİM 11-1]. Modülün sabiti de köşeli ayraçla değiştirilebiliyor (`m["AD"] = 1`; eski 14. bölümün sözü) [kanıtsız: bu tur ölçülmedi].
- **Yapılacaklar:** denetim hataları (çakışma, aktarılan ada atama), iki yeri gösteren ve `TR_as`'ı öneren ileti; olumsuz fikstürler.

### 11.7

- **Yapılacaklar:** yapı alanları ve metotları için `_` kuralı 11'in işi değil; 7'de karara bağlanmalı (eski 11.7 "ileride gelecektir" diyordu).

## 12. Testler

Kurallar: [`docs/spec/12_testler.md`](spec/12_testler.md). Taslak (2026-10-29): ana oturum CLAUDE.md'den türeterek yeniden yazdı; DENETİM bekleniyor. Yeni ad `TR_assert_close`'un gösterim adı onaylandı (yakın_olmalı); kodu uygulandığı gün kayda girer (17.4).

### 12.1

- **Yapılacaklar:** test dosyasının tanımı (1.1'in en dış kuralı ona dayanıyor): rehber adı `_test.tr` ile biten dosya diyor, ama `tests/yardimci_testli.tr` de test bloğu içeriyor [kanıtlı: DENETİM 12-1]. Testin hangi cihazda koştuğu ve cihazlar arası tolerans 15'in E parçasında.
- **Gemini Koşu 7 (2026-10-29), uygulanacak:** test dosyası tanımı; en dışta deyim içeren iki dosya göç ister: `examples/tr/17_testler_test.tr:35`, `tests/ozel_turev_test.tr:11` [ölçüldü: DENETİM].

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

- **Mevcut durum:** en dıştaki tanımlar testler arasında paylaşılıyor: bir testin en dıştaki değişkene atadığı değer sonraki testte görünüyor [ölçüldü: main `e16d39df`]; aktarılan modülün durumu da sızıyor [ölçüldü: DENETİM]. Test dosyasının en dışındaki deyim de testlerden önce çalışıyor (1 yer); sabit olmayan ilk değer 7 yerde (geçici klasör, komut satırı ayarı, kullanıcı tanımlı türev); rastgele üretecin durumu testten teste sürüyor [ölçüldü: DENETİM]. Rehber (`docs/guide/07_testler.md:3`) eski sınıflı yazımla.
- **4a1 ilk dilim (2026-10-30):** test dosyasının en dışındaki deyim ve sabit olmayan ilk değer denetim hatası (H1107, H1108); depodaki yerler (1 deyim, geçici klasör ve komut satırı ayarı) kurulum fonksiyonuna alındı [ölçüldü: `TestRepoModulesHoldOnlyDefinitions`].
- **Yapılacaklar:** en dıştaki tanımların her test için yeniden kurulması; her testin üreteci aynı durumda başlatması; TR testleri.
- **Gemini Koşu 7 (2026-10-29), uygulanacak:** hatalı test bitmemiş görevlerini önce iptal eder (10.9); bugün sonsuz işçiyle asılı kalıyor.

## 13. Yerleşikler

Kurallar: [`docs/spec/13_yerlesikler.md`](spec/13_yerlesikler.md). Taslak, 7. tur (2026-10-28): ana oturum yazdı; dört turu DENETİM, 1. turu ana oturum, 5. turu (dokuz iş) ikisi, 6. turu Gemini'nin araştırması (DENETİM doğruladı: 7 onay, 5 koşullu) inceledi; bulgular işlendi. Uygulama 6. ve 7. turun kararlarını henüz almadı: ayırıcısız `TR_split`, `TR_std`'nin n−1'i, telafili toplama, `TR_split("")`'in hatası, JSON'un yinelenen anahtarı, yarımı çifte yuvarlama, `TR_median([1.0, 2.0, TR_nan])` (bugün 1.0), sıfırın işareti (`TR_max(-0.0, 0.0)` sıraya bağlı), yazma yolunun programın klasörüne düşmesi, `TR_clamp(5)`'in iletisindeki eski kimlik, fonksiyon yerine verilen tipin iletisi eski davranışta [ölçüldü: DENETİM]. `tests/yerlesikler_test.tr` eski kuralları da sınıyor (kazanan değerin tipi, sayı hücreli CSV, `TR_mean`'in tek tek değerleri); kural uygulanırken yeniden yazılır.

### 13.1

- **Mevcut durum:** `TR_max` kazanan değerin tipini veriyor: `TR_max(2, 1.5)` `TR_Int` 2, `TR_max(1, 1.5)` `TR_Float` 1.5; metin argüman hata; `TR_max([])` `TR_none`; `TR_max(1)` 1 veriyor; `TR_mean(1, 2)` 1.5 (depoda 2 yer); `TR_round(5, 2)` `TR_Float` 5.0, `TR_round(3.7, 0)` `TR_Int` 4, `TR_round(TR_nan)` hata; `TR_print` yazdığı metni, `TR_sleep(0)` ve `TR_wait_all()` `TR_true` döndürüyor; boş listede `TR_mean`, `TR_std`, `TR_median` hata [ölçüldü: DENETİM].
- **Mevcut durum:** 13.1'in −0.0 kuralı, telafili toplama (Neumaier) ve tek öğede `TR_std`'nin hatası uygulanmadı [ölçüldü: DENETİM C1]; `TR_max([-0.0, 0.0])` −0.0 veriyor [ölçüldü: main `8b115692`] (15'in yan bulgusu). Tensörde `TR_round`'un `TR_Float` dönüşü 15.16a'nın status'unda (15'in yan değişikliği, uygulanmadı).
- **Yapılacaklar:** `TR_std` bugün n'ye bölüyor; ortalama ve sapma yolları (`interp/stats.go:38`) FMA'ya birleşebiliyor, `math.Exp` amd64'te işlemciye göre yol seçiyor [kanıtlı: DENETİM]; ikili veri: `TR_write_file` `TR_Bytes`'ın metnini yazıyor ("Bytes([0, 255, 128])", 20 karakter), `TR_read_bytes` yok, `TR_read_file` geçersiz UTF-8'i sessizce okuyor [ölçüldü]; düz ondalık toplam sıraya bağlı (`TR_sum([1e16, 1.0, -1e16])` 0.0, başka sırada 1.0) [ölçüldü]; `TR_time_ms` `time.Now().UnixMilli()`, `TR_monotonic_ns` yok [kanıtlı]; `TR_print` boşlukla birleştiriyor [ölçüldü]; karışık sayıların `TR_Float`'ı, metnin sıralanabilir sayılması, boş listede `TR_min`/`TR_max`'ın hatası, tek argümanın hatası, `TR_mean`/`TR_std`/`TR_median`'ın yalnız liste alması, `TR_round`'un üç imzası, değer döndürmeyen üç yerleşik.
- **Gemini Koşu 3 (2026-10-29), uygulanacak:** `TR_min` ve `TR_max`'ın anahtar fonksiyonlu biçimi sıralama sırasını kullanır; `TR_sum` tensör listesinde boşta hata (15.18).

### 13.2

- **Mevcut durum:** ayırıcısız `TR_split()` tek ASCII boşluğa bölüyor, boş öğe veriyor, sekmede ve satır sonunda bölmüyor [ölçüldü: DENETİM]; harf dönüşümü basit eşleme: `"ß".TR_upper()` → "ß", `"İSTANBUL".TR_lower().TR_length` 8 [ölçüldü: DENETİM].
- **Yapılacaklar:** ayırıcısız `TR_split()`'in Unicode boşluk dizilerine bölmesi.
- **Gemini Koşu 6 (2026-10-29), uygulanacak:** boş alt metin (motor bugün `"ab".TR_replace("", "x")`'te "xaxbx" veriyor [ölçüldü: DENETİM]; kurala göre hata) ve negatif indeks; `.TR_index_of` metinde henüz yok.

### 13.3

- **Mevcut durum:** `.TR_sort()` −0.0 ile 0.0'ı eşit sayıyor: `[0.0, -0.0].TR_sort()` `[0.0, -0.0]` veriyor [ölçüldü: main `8b115692`] (15'in yan değişikliği, uygulanmadı).
- **Yapılacaklar:** 13.10'un sıralama sırası (−0.0 0.0'dan önce, NaN'lar en sonda ve birbirine eşit; demette öğe öğe); 15.22b ile birlikte; TR testi.
- **Anahtarlı sıralama:** `.TR_sort(f)` motorda yok: `.TR_sort()` argüman almıyor (H2011; 4a2'den beri denetimde de), ileti bugünkü yolu, anahtarı demetin ilk öğesi yapan sıralamayı gösteriyor (H1116) [ölçüldü: `TestMemberArgCount`]. Eklenmesi ayrı bir dil kararı (ana oturum, 2026-10-30).

### 13.6

- **Mevcut durum:** `TR_reduce(0, (a, x) => a + x)` çalışıyor; başa yazılan ve tek parametreli fonksiyon hata, ama iletiler (H2475, H2476) eski sondaki blok yazımını öneriyor; 3A düzeltiyor [ölçüldü: DENETİM].
- **Yapılacaklar:** iletilerin testi 3. katman birleşince; başlangıçsız `TR_reduce`'un boş listede hatası; `.TR_append`'in değer döndürmemesi (bugün listenin kendisi).

### 13.7

- **Mevcut durum:** `TR_is_nan` yok (bilinmeyen kimlik); tensörde `TR_is_finite` float32 0/1 veriyor (15.36); `TR_where`'in sayı imzası var: `TR_where(TR_true, 2, 3)` 2 veriyor [ölçüldü: main `8b115692`] (15'in yan değişikliği, uygulanmadı).
- **Yapılacaklar:** `TR_is_nan`'ın sayı ve tensör imzası; tensörde `TR_Tensor[TR_Bool]` dönüşü; `TR_where`'in sayı imzasının kalkması, göçü 15.15'in status'unda (`tests/matematik_test.tr:169-171`); TR testi.

### 13.8

- **Mevcut durum:** JSON'a NaN yazmak hata (H7008) [ölçüldü]; `TR_json_parse` `TR_Dict` veriyor; `TR_json_string({1: 2})` anahtarı sessizce metne çeviriyor (`{"1":2}`); CSV hücresi sayıya benziyorsa sayı (`[["007", 7]]`) [ölçüldü].
- **Yapılacaklar:** `TR_json_parse` `TR_JSON` verir (3.7a); metin olmayan anahtarın hatası; CSV hücresi hep metin (göç: `.tr`'de 10 kullanım).
- **Gemini Koşu 6 (2026-10-29), uygulanacak:** `TR_json_string`'de yol ve tensörün yazımı; `TR_json_parse[T]`'de `null`, iç içe yapı ve tensör alanının hatası.

### 13.8a

- **Mevcut durum:** `t.TR_tolist` tensörün boyutuna göre liste ya da sayı, `k.TR_decode` toplu girişte liste, `TR_parameters(x, TR_named: TR_true)` sözlük veriyor [ölçüldü: DENETİM].
- **Yapılacaklar:** `TR_item`, `TR_batch_decode`, `TR_named_parameters` (yeni kimlikler, 4a); `TR_tolist`'in yalnız 1 boyutta, öğe tipi tensörün cinsi olan liste vermesi, 0 boyutta hatası (15'in yan değişikliği, uygulanmadı); `TR_named_parameters`'ın `TR_Dict[TR_String, TR_AnyTensor]` dönüşü (aynı) (argmax'in sonucunu indeks olarak kullanan kod `TR_Int(t.TR_item)` yazar, göçte sayılır).

### 13.9

- **Mevcut durum:** katalog 188 fonksiyon ve metottan 138 Go yerleşiğinin imzasını tutuyor; 50'si TR ile yazılmış kitaplıkta (29 fonksiyon, 21 metot), imzası kaynakta. Tipler öğesiz (`TR_List`, `TR_Dict`), fonksiyon tipi tek bir `TR_Function`, yerleşiğin tipi `TR_BuiltinFunction` (11 yerde), genel tip `TR_Any` 24 yerde [ölçüldü: DENETİM].
- **Yapılacaklar:** imzalar öğe ve fonksiyon tipleriyle; `TR_BuiltinFunction` ve `TR_Any` kalkar; Go testi (4a).
- **Gemini Koşu 6 (2026-10-29), uygulanacak:** yerleşiğin fonksiyon değeri olarak eşleşmesi (isteğe bağlı parametreler boş, adlı argümanlar varsayılanda; varsayılansız adlı argümanda hata). Denetleyicinin işi (4a2).

### 13.10

- **Mevcut durum:** 35 yerleşiğin dönüş tipi birleşim (`TR_abs`, `TR_floor`, `TR_exp` gibi), `TR_tolist`'in özelliği de; dönüşü argümanın değerine bağlı üç yer: `TR_tolist` (tensörün boyutu), `TR_decode` (toplu giriş), `TR_parameters` (`TR_named:`); 9 yerleşik değişken sayıda argüman alıyor; yalnız 7 metodun alıcıya göre ayrı imzası var [ölçüldü: DENETİM].
- **Yapılacaklar:** sekiz kısıt ("number", "ordered", "equatable", "key", "json", "struct", "function", "tensor_data"); `TR_command_line` ve `TR_Checkpoint`'in yapıyla yazımı (göç: 19 ve 50 kullanım, `.TR_state` 39); anlamsız `TR_Bool` dönüşlerinin kalkması, tek bir 4a işi: `TR_write_file`, `TR_assert` (`internal/lang/interp/test_support.go`), `.TR_load_state`, `TR_sleep`, `TR_wait_all` (argümanlı biçimi liste verir, 10.5), 19'daki `TR_Logger.TR_append` ve `TR_Progress.TR_update` gibi; `tr.json`'un açıklamalarındaki "doğru döndürür" aynı commit'te değişir [ölçüldü: DENETİM 10-2, 10-3]; `TR_topk`'nın iki değeri (6.3, 15'te), `TR_assert`'in iletisinin `TR_String` ya da `TR_Message` olması (12'de); kısıtlı tip parametreleri, ayrık imzalar, değere bağlı dönüşlerin sabit dönüşlü yerleşiklere ayrılması (15 ve 16. bölümlerle), değişken sayıda argümanın yazımı (4a).
- **Mevcut durum (15'in yan değişiklikleri, uygulanmadı):** tensör verisi kısıtının cinse bağlanması, eşitlenebilirin tensörü dışlaması, varsayılansız adlı argüman, `TR_save_tensors`'un kabul kümesi, `TR_load_tensors` ve `.TR_narrow[E]()`'nin imzaları, `TR_quantized`'ın imzası, `TR_custom_grad`'ın dönüşü ve `TR_grad_check`'in değer döndürmemesi (15.32'nin status'unda) imzalarda yok.
- **Gemini Koşu 6 (2026-10-29), uygulanacak:** karşılaştıran yerleşiklerin kesin karşılaştırması (2⁵³ üstü).
- **Değeri kullanılmayan çağrı (4a2, ilk dilim):** uygulandı. Tanım dosyasında `pure` (39 yerleşik: 13.2–13.4'ün yeni değer veren metotları, 13.7'nin hesapları); çağrısı tek başına deyimse H1115, ileti atamayı önerir, alıcı atanabilir adsa uygulanabilir düzeltme [ölçüldü: `TestPureCallValueUnused`, `TestPureMarksOnlyValueBuiltins`, öneri tablosu]. Tensör işlemlerinin işareti ve 13.1'in hesapları ikinci dilimde.

### 13.11

- **Mevcut durum:** yanlış tip ve sayı çalışma anında, iletiyle: "sqrt() expects Int or Float as argument 1, String (\"a\") given", "sqrt() takes 1 argument, 2 given (usage: sqrt(number))" [ölçüldü]. 4a2 (ilk dilim): tipi bilinen alıcıda (Metin, Liste, Sözlük, Aralık) yerleşik metodun konumlu argüman sayısı denetimde de, aynı kodla (H2011); metodun sayı iletisi çalışma anında da denetimde de kimlikle: ".TR_sort() takes 0 arguments, 1 given (usage: `.TR_sort()`)", `.TR_sort()`'a anahtar verilince demet sıralamasının yolu (H1116) [ölçüldü: `TestMemberArgCount`, `TestMemberArgCountUsesIdentities`, `TestMethodCountsMatchDefinitions`].
- **Yapılacaklar:** argümanların tip denetimi (ikinci dilim); fonksiyonların ve öbür alıcı tiplerinin (tensör, kitaplık tipleri) sayı denetimi; fonksiyonların H2011 iletisi hâlâ görünüm adıyla (`round()`).

## 15. Tensörler ve otomatik türev

Kurallar: [`docs/spec/15_tensorler.md`](spec/15_tensorler.md). Bölüm 2026-10-29'da altı parçada (A–F) CLAUDE.md'den türetilerek yeniden yazıldı; aşağıda bugünkü motorun kurallardan ayrıldığı yerler ve göç.


**En büyük göç kalemleri** [ölçüldü: DENETİM, main `8b115692`]:
1. tam sayı değişmezli tensörler: 91 yer (`tests/` 77, `examples/` 11, `lib/` 3), çalışırken 20 dosyada 158 kurulum (15.1–15.3);
2. karşılaştırmanın mantık tensörü vermesi: kaba sayım ~40 yer, çoğu testte (`TR_where` 16, karşılaştırmanın `.TR_tolist` 16, aritmetikte 1, toplamda 1, maske 4) (15.6, 15.15);
3. `TR_grad_check`: 40 çağrı, 31'i dönüşünü kullanıyor (15.32);
4. çapraz entropi ve gömmenin indeks cinsi: 33 + 24 çağrı (15.23, 15.23a);
5. `TR_custom_grad` 22 ve `TR_no_grad(x)` 20 satır (15.27a, 15.28);
6. tensörde bileşik atamanın yerinde yazması: `.tr`'de ada bileşik atama 76 satır, çoğu sayaç; önce ve sonra koşup karşılaştırmak yeter (15.6d).

### 15.1

- **Mevcut durum:** bütün tensörler float32; tam sayı ve mantık tensörü yok, argmax'in sonucu bir "tam sayılı işaret" taşıyor. 2²⁴'ün üstündeki kimlik bozuluyor: `TR_Tensor([16777217.0])` → 16777216. float64, `TR_AnyTensor` ve `.TR_narrow[E]()` yok [ölçüldü].
- **Yapılacaklar:** öğe cinsli tip (`TR_Tensor[E]`), `TR_Int` ve `TR_Bool` tensörleri, float64, `TR_AnyTensor` ve daraltma; TR testi. Bedel [kanıtlı: DENETİM'in sayımı]: motorun 45 yönteminin 12–15'i ile 7 GPU çekirdek dosyası. Motor "int32" ve bayt mantıkla başlayabilir; "int64" varsayılanı gelene kadar bu satırda yazılır.
- **Tip yazımı ayrışıyor (Faz 0.6a, 2026-10-29):** `TR_Tensor[E]` ayrıştırılıyor, `TR_Tensor`'a indirgeniyor; `TR_AnyTensor` henüz kod değil [ölçüldü: bulut 0.6a].

### 15.2

- **Mevcut durum:** `TR_Tensor([1], TR_dtype: "int64")` H6810 veriyor; cins veriden çıkmıyor, tam sayı listesi ondalık tensör kuruyor [ölçüldü].
- **Yapılacaklar:** cinsin veriden çıkması ve tip parametresi; sığma denetimi. Göç [ölçüldü: DENETİM A2]: bütün öğeleri tam sayı değişmezi olan ve `TR_dtype:`'sız `TR_Tensor([...])` 91 yerde (tests/ 77, examples/ 11, lib/ 3); çalışmada 20 dosyada 158 kurulum. 4'ü `TR_requires_grad: TR_true`'lu (tests/tensor_test.tr:817-838) ve denetim hatası olur. Hepsi denetimde görünür; ileti `[[0.0], ...]`'i ya da `TR_Tensor[TR_Float](...)`'ı önerir.
- **Gemini Koşu 3 (2026-10-29), uygulanacak:** `TR_zeros`, `TR_ones` ve `TR_eye`'ın mantık cinsi.
- **Gemini 15/2 (2026-10-29), uygulanacak:** `TR_zeros(..., TR_dtype: "q4")` gibi nicemlenmiş sıfır kurucu (T3).

### 15.3

- **Mevcut durum:** sürüm motorda var (depo başına sayaç, H3012) [kanıtlı: DENETİM, device.go:154-181, autograd.go:169-172, 226]; ama ileti `TR_sigmoid`'in ezilen çıktısına "girdi" diyor [ölçüldü: DENETİM]. Cins dönüşümü, tam sayı taşması ve öğe tipli liste yok (bütün tensörler float32).
- **Yapılacaklar:**
  - cins dönüşümü (tip çağrısı, hedef hassasiyetiyle), tam sayı taşmasının hatası ve `TR_wrapping_add`, `TR_wrapping_sub` ve `TR_wrapping_mul`'ın tensör imzası;
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

- **Mevcut durum:** `a == b` tek bir mantık değeri veriyor; karşılaştırma float32 0/1 maske veriyor. Metin dört basamaklı ve bütün öğeler yazılıyor [ölçüldü].
- **Yapılacaklar:** öğe öğe karşılaştırma ve `TR_Bool` maske, eşitlenemeyen değerlerin denetim hatası, metin; TR testi. Göç [ölçüldü: DENETİM]: değişen 0 boyutlu tensörün sayıyla karşılaştırması (tests/operatorler_test.tr'de 6 çalışma); bütün tensör karşılaştırmasının 85 yeri ve `TR_assert_equal`'ın 141 kullanımı değişmez. Dolaşmada öğenin hep tensör olması yalnız 15.5'in kendi testini değiştirir: tests/tensor_test.tr:575, :577 [ölçüldü: DENETİM A2].
- **Gemini Koşu 3 (2026-10-29), uygulanacak:** isteğe bağlı tensör içeren her tipin `TR_none`'la karşılaştırması; gösterimde cinsin varsayılanı dışındaki hassasiyet.

### 15.6

- **Mevcut durum:** argmax'in tam sayılı işareti `+ 1`'de düşüyor (sonuç `2.0`, `TR_Float`); `/ 2` ve `* 2.5` tam sayılı tensörü ondalık yapıyor. `t // 2`, `t % 2` ve bit işleçleri H6703 veriyor; `@` yalnız ondalık [ölçüldü].
- **Yapılacaklar:** cinsli işleç tablosu, bekleyen işlemler ve `TR_replay`'deki cihaz içi bayrak; TR testi. Göç: kesirli aritmetiğe giren argmax sonucu (tools/gpu/ss_exam.tr:58) [ölçüldü: DENETİM]; kitaplıkta maske aritmetiği 3 yer [ölçüldü: DENETİM B1]: lib/optimizers.tr:341 (`n + geçti` → `TR_where` ya da `TR_sum`), :344 (oranın `geçti * g` ile çarpımı → `TR_where(geçti, ..., 0.0)`), lib/layers.tr:419 (`m * (...)` → `m & (...)`).
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
  - tensörde bileşik atama adı yeni tensöre bağlıyor: `TR_var q = p; p -= ...` sonrası `q` eski değerde [ölçüldü]; dilimle yazılan `r[0..2] -= ...` yerinde [ölçüldü];
  - atamada örtüşen kaynak önce kopyalanıyor [ölçüldü: DENETİM]; yerinde bileşik atama için yeni;
  - türev izlemeyen hedefe türev izleyen değer yazmak hatasız, gradyan bağı kopuyor (`havuz[idx] += d`) [ölçüldü: DENETİM]; depodaki altı yazma yerinin hepsi türevsiz kipte [ölçüldü: DENETİM];
  - `.TR_select`, `.TR_unsqueeze` ve `.TR_fill` yok; `.TR_slice` 31 yerde kullanılıyor [ölçüldü: grep]; `m[:, None, :]` en az 4 yerde `m.TR_reshape([B, 1, T])` diye yazılıyor [ölçüldü: DENETİM].
- **Yapılacaklar:**
  - tensörle ve maskeyle indeks, `.TR_select`, `.TR_unsqueeze`, `.TR_fill`, yerinde bileşik atama, tensörle indekste biriktirme, türevin üç kuralı (türevsiz kipe bağlı); TR testi;
  - bileşik atamanın göçü sessiz bir anlam değişikliğidir (yazım aynı kalır): bütün .tr dosyalarında tensör adına bileşik atamalar ve aynı tensörü paylaşan adlar sayılır, örnekler ve lib/ önce ve sonra koşulup karşılaştırılır;
  - kitaplığın `TR_Tensor(TR_no_grad(...), TR_requires_grad: TR_true)` dolambacı (lib/layers.tr:20) yaprağın tanımıyla sadeleşir;
  - indeksli hedefte yerinde olmayan bir biriktirme (index_add türü) yok; açık soru.
- **Tersine dönecek Go testi:** `TestMatrixIndexAndAssign` (`internal/stdlib/ml/matrix_test.go:50`) tam sayı değişmezli matrisi ondalık bekliyor (15.2), 0 boyutlu tensörde `.TR_tolist` okuyor (13.8a) ve liste değerli atama yapıyor (`M[-1] = [8, 9]`; değer sayı ya da tensör olmalı). Aynı dosyanın öbür testlerine de bakılmalı.

### 15.6e

- **Mevcut durum:** `2 ** t` H6702 veriyor, ileti `TR_exp(TR_log(2) * t)`'yi öneriyor; `(-8.0) ** 0.5` tensörde NaN (kuralla aynı) [ölçüldü].
- **Yapılacaklar:** tensör üs ve tam sayı tensöründe eksi üs hatası; TR testi.

### 15.8

- **Mevcut durum:** türev izleyen hedefe yalnız `TR_no_grad` içinde yazılıyor (türevsiz kipin üç kuralı değil); başlangıç tam sayı ya da tam sayı değerli 0 boyutlu ondalık tensör.
- **Yapılacaklar:** 15.6d'nin üç kuralı; `TR_start`'ın `TR_Int` ya da 0 boyutlu `TR_Tensor[TR_Int]` olması; cins denetimi; `TR_replay`'de sınır dışı başlangıcın cihaz içi bayrakla hatası (15.45).

### 15.13

- **Gemini Koşu 3 (2026-10-29), uygulanacak:** sayı sınırda NaN'da NaN türev; tensör sınırda `TR_maximum` ve `TR_minimum` gibi (eşitlikte yarı yarıya, NaN öğe yayılır).

### 15.15

- **Mevcut durum:** koşul "sıfırdan farklı" ondalık: `TR_where(TR_Tensor([2.0, 0.0, -1.0]), 1.0, 0.0)` → `[1.0, 0.0, 1.0]`, NaN da doğru [ölçüldü]; sayı imzası var.
- **Yapılacaklar:** mantık koşulu, sayı imzasının kalkması; TR testi. Göç [ölçüldü: DENETİM B1]: çalışmada hiç 0/1 dışı koşul yok; durağan 4 satır ondalık koşul (tests/matematik_test.tr:168, :179, :182, :183) ve 3 satır sayı imzası (tests/matematik_test.tr:169-171).
- **Gemini Koşu 3 (2026-10-29), uygulanacak:** iki sayılı biçimin cinsi ve hassasiyeti.

### 15.16a

- **Mevcut durum:** `TR_maximum`, `TR_minimum`, tensörde `TR_round`, `TR_is_nan` ve tensörde `TR_is_finite` (`TR_Bool` veren) yok.
- **Yapılacaklar:** dördü; TR testi.
- **Gemini Koşu 3 (2026-10-29), uygulanacak:** `TR_maximum` ve `TR_minimum`'un NaN konumundaki türevi iki işlenene de NaN.

### 15.17

- **Mevcut durum:** `TR_approximate:`'in varsayılanı var ve tanh [ölçüldü].
- **Yapılacaklar:** zorunlu ve yalnız adla verilen `TR_approximate:`. Göç [ölçüldü: DENETİM B1]: argümansız 16 çağrı (examples/tr/ 7, tests/tensor_test.tr 5, Go testlerinin TR metni 4); göçte `TR_approximate: TR_true` yazılır, bugünkü sonuç korunur.

### 15.18

- **Mevcut durum:** eksen `TR_sum`/`TR_mean`'de konumla da veriliyor, `TR_max`/`TR_min`'de verilmiyor (H6205) [ölçüldü]. Maskenin toplamı ve ortalaması ondalık (`2.0`, `0.6666667`); `TR_argmax`'in sonucunun toplamı ondalık `1.0` [ölçüldü]. `TR_std` tensörde H2423 veriyor, `TR_logsumexp` yok [ölçüldü]. `TR_max(TR_Tensor([-0.0, 0.0]))` −0.0 veriyor [ölçüldü]. 16 777 217 birin float32 toplamı 16777216.0 (sonuç float32'ye yuvarlanıyor; doğru davranış); bf16 toplamın sonucu bf16 [ölçüldü].
- **Yapılacaklar:** eksen yalnız adla; göç 3 yer [ölçüldü: DENETİM C1]: `tests/tensor_test.tr:241`, `tools/gpu/ss_exam.tr:59`, Go dizesi `internal/stdlib/ml/builtin_reduce_test.go:86`. `tools/gpu/ss_exam.tr:58-60` 15.19'un göçüyle birlikte yapılır (`TR_argmax(...) == iş[1]` ve `TR_sum(isabet, TR_dim: 0)`); cinsli sonuç bu satırdaki gizli `TR_Int += TR_Float`'u da düzeltir. Cinsli sonuçlar, "int64" taşma hatası, `TR_std`, `TR_logsumexp`, liste eksen, −0.0 < 0.0; TR testleri.
- **Gemini Koşu 3 (2026-10-29), uygulanacak:** boş eksen ile boş tensörün ayrımı, boş tensör listesinde hata, `TR_std`'nin sıfır sapmadaki türevi 0.

### 15.18a

- **Mevcut durum:** `TR_all`, `TR_any`, `TR_equal` ve `TR_all_close` yok; `TR_all` bilinmeyen kimlik [ölçüldü].
- **Yapılacaklar:** dördü, eksenli `TR_all`/`TR_any` ve `TR_keepdim:`; hassasiyete göre toleranslar; TR testi.
- **Gemini Koşu 3 (2026-10-29), uygulanacak:** `TR_all_close`'da sonsuzun kuralı.

### 15.19

- **Mevcut durum:** `TR_argmax` eksensizde son eksende çalışıyor: `TR_argmax(TR_Tensor([[1.0, 9.0], [4.0, 2.0]]))` → `[1, 0]` [ölçüldü]. `TR_argmin` yok. −0.0 ile 0.0 eşit sayılıyor.
- **Yapılacaklar:** eksensiz biçim bütün öğelerde; göç 10 test yeri [ölçüldü: DENETİM C1]: `tests/matematik_test.tr:94`, `:182`, `:196`; `tests/tensor_test.tr:511`, `:516`, `:937`; Go dizeleri `builtin_reduce_test.go:12-13`, `:15`, `builtin_math_test.go:32`. İkisi `TR_cumsum`'u 0 boyutlu girdiye götürüp hataya döner; `tests/tensor_test.tr:937` `TR_sample` ile karşılaştırıyor, `TR_dim: -1` gerekir. Kullanıcı kodunda yok. `TR_argmin`, eksenli `TR_keepdim:`, argmax'e −0 için eşitlik dalı; TR testi.
- **Gemini Koşu 3 (2026-10-29), uygulanacak:** boş indirgemede hata.

### 15.20

- **Mevcut durum:** `TR_cumsum`'un ekseni varsayılan −1.
- **Yapılacaklar:** `TR_dim:` zorunlu; göç `.tr`'de 7 satır, Go dizelerinde 3 (ikisinin beklenen ileti kodu değişir) [ölçüldü: DENETİM C1]. `TR_Int`/`TR_Bool` girdide "int64" ve taşma hatası.

### 15.21

- **Mevcut durum:** `TR_topk` liste veriyor (`TR_Type`'ı `TR_List`) [ölçüldü]; −0 ile +0 eşit sayılıyor.
- **Yapılacaklar:** demet dönüşü (önce 3.6a'nın demeti); göç 9 çağrı [ölçüldü: DENETİM C1]: `examples/tr/25_secme_ve_siralama.tr:22-23`, `tests/matematik_test.tr` 7 satır, Go 1. Sıralama sırası (15.22b); TR testi.
- **DENETİM 0.6b (2026-10-29):** `TR_topk` hâlâ liste döndürüyor; kurala göre demet (T1). Demetin değeri motorda var (0.6b).

### 15.21a

- **Mevcut durum:** `TR_gather` yok.
- **Yapılacaklar:** uygulamak; TR testi. Gösterim adı onaylı (Ahmet, 2026-10-29).

### 15.22

- **Mevcut durum:** `TR_topk` ve `TR_argsort`'ta sıralanan eksen en çok 2²⁴ uzunlukta (indeksler float32 depoda); `TR_descending:` yok (motorda `Backend.Sort(..., desc)` hazır [kanıtlı: DENETİM C1]). −0 ile +0 eşit sayılıyor: `cpu_sort.go:47` ve `kernel_sort.go:22`'nin yorumu "−0 = +0", `orderKey`'in sıfır satırı `cpu_sort.go:55-56`.
- **Yapılacaklar:** 2²⁴ sınırı tam sayı depolamasıyla (15.1) kalkar, yeni sınır 2³² − 1; `TR_descending:`; `orderKey`'in sıfır satırı silinir [kanıtlı: DENETİM C1]; TR testi.
- **GPU'nun yolu:** her satır 64 bitlik (sıra anahtarı, indeks) çiftleriyle bitonik ağla sıralanıyor; ikinin kuvvetine tamamlanmış uzunluğu 4096'ya kadar olan satır paylaşımlı bellekte tek başlatmayla, daha uzunu genel bellekte her bitonik adım için bir başlatmayla (log₂P·(log₂P + 1)/2) sıralanıyor.
- **Uygulama sınırı:** GPU'da sıralanan anahtarın alt 32 biti indekstir; bu yüzden eksen en çok 2³² − 1 uzunlukta ve sıra anahtarı 32 bit (float32'de yeter; int64 ve float64 tensörde ve daha uzun eksende yeni bir yol gerekir). Aynı sınır `TR_topk`'ta. Kural bir sınır koymaz (bellek dışında, 15.42).

### 15.22a

- **Mevcut durum:** CPU float64'te telafisiz, GPU float32 girdiyi float32'de telafili (Neumaier) biriktiriyor (float64 girdi float64'te). CPU'da `GOMAXPROCS` 1–16'da sonuç bit bit aynı [ölçüldü: DENETİM].
- **Yapılacaklar:** `TestReductionsIndependentOfThreadCount` Go testi bölümün commit'inde yazılır (bugün yok); 15.22a başlıklı TR testi.

### 15.22b

- **Mevcut durum:** sıralama sırası uygulanmadı: tensörde ve listede −0.0 ile 0.0 eşit sayılıyor; listenin `TR_max([-0.0, 0.0])`'ı −0.0 veriyor (13.1'e aykırı) [ölçüldü]. Go testi `TestArgSortAndTopK` −0 ile +0'ın eşitliğini sabitliyor; `internal/compute/gpu/backend_wide_test.go:53-54` de [kanıtlı: DENETİM C1].
- **Yapılacaklar:** −0.0 < 0.0 her sıralamada (tensör, `.TR_sort`); iki Go testi yeni sıraya; TR testi.

### 15.23

- **Mevcut durum:** sınır dışı indeks H6305 veriyor [ölçüldü: main `8b115692`]. İndeks liste ya da ondalık tensör olabiliyor.
- **Yapılacaklar:** indeks `TR_Tensor[TR_Int]`; göç 24 satır [ölçüldü: DENETİM, `git grep` main `8b115692`]; sınır dışı denetimi bekleyen işlem (15.6).

### 15.23a

- **Mevcut durum:** her negatif hedef yok sayılıyor (`-5` de); sınıf sayısından büyük hedef H6314 [ölçüldü: main `8b115692`]. Olasılık hedefi yok.
- **Yapılacaklar:** yalnız `-100` yok sayılır, öbür negatifler hata; olasılık hedefi; iki denetim bekleyen işlem (15.6). Göç 33 satır (liste ya da ondalık hedef → `TR_Tensor[TR_Int]`); `-1`'li 3 satır: `tests/tensor_test.tr:214, 288, 289` [ölçüldü: DENETİM, `git grep` main `8b115692`]. TR testi (15.23a başlığı).

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

- **Mevcut durum:** dikkatin maskesi 0/1 sayı tensörü [ölçüldü: main `8b115692`].
- **Yapılacaklar:** maske `TR_Tensor[TR_Bool]`, 0/1 sayı maskesi denetim hatası.
- **Gemini 15/2 (2026-10-29), uygulanacak:** tamamen −∞ sapmalı satır 0 verir (bugün NaN); CPU ve GPU (T2).

### 15.27

- **Mevcut durum:** aynı çizgeden ikinci geri yayılım H3017; ara sonucun `.TR_grad`'ı H6700, iletisi nedenini söylüyor [ölçüldü: main `8b115692`]. Eski metne göre yeni `TR_async` görevi türevli kipte başlıyor [tahmin: ölçülmedi].
- **Yapılacaklar:** yeni görev açıldığı yerin türevsiz kipini ve hesap türünü alır; bloktaki `TR_defer` kipin içinde.

### 15.27a

- **Mevcut durum:** `TR_detach` yok; ayırma `TR_no_grad(x)` ile yapılıyor ve `d = TR_no_grad(h); d[0] = 9.0` türev izleyen `h`'yi sessizce değiştiriyor [ölçüldü].
- **Yapılacaklar:** `.TR_detach()`; ayrılmış görünüme yazma kuralları (15.6d); `TR_no_grad(x)` biçimi kalkar, göç 20 satır [ölçüldü: DENETİM, `git grep` main `8b115692`]; TR testi.

### 15.28

- **Mevcut durum:** `f_ileri` `[çıktı, [saklananlar]]` listesi döndürüyor [ölçüldü]; eski metne göre çağrının değeri `TR_BuiltinFunction`.
- **Yapılacaklar:** demet biçimi (önce 3.6a); dönüş tipi; göç 22 satır [ölçüldü: DENETİM, `git grep` main `8b115692`].

### 15.29

- **Mevcut durum:** eski metne göre `geri` liste döndürüyor; `g`'ye yerinde yazmak başka bir gradyanı değiştirmiyor [ölçüldü: DENETİM].
- **Yapılacaklar:** tek girdide kendisi, birden çokta demet; fonksiyon ifadesinde öğe sayısının denetim hatası.

### 15.32

- **Mevcut durum:** `TR_grad_check` `TR_true` döndürüyor [ölçüldü].
- **Yapılacaklar:** değer döndürmez; `girdiler` demet; göç 40 satır, 31'i dönüşünü kullanıyor (tek tek) [ölçüldü: DENETİM, `git grep` main `8b115692`].

### 15.33

- **Mevcut durum:** `TR_parameters` `TR_named:` argümanıyla yol adlı sözlük veriyor; `TR_named_parameters` adı yok; yapıdaki ara sonuçlar ve türev izlemeyen tensörler sessizce atlanıyor [kanıtlı: bugünkü 15.33'ün metni; `definitions.json`'da `TR_named_parameters` kodu yok].
- **Yapılacaklar:** `TR_named:` kalkar, `TR_named_parameters` (13.1) gelir; parametre tanımı (donmuşlar `TR_parameters`'ta); `TR_named_parameters` bütün yapraklar; türevli ara sonuçta hata; TR testi bunlara göre.

### 15.34

- **Mevcut durum:** dondurma yok: yaprakta `.TR_requires_grad` üyesi yok (`definitions.json`'da `TR_requires_grad` yalnız adlı argüman kodu) [kanıtlı]. Kitaplığın iyileştiricisi indeksli dolambaçla yazılı: `p[0..p.TR_shape[0]] -= ...`, `g[0..g.TR_shape[0]] = 0.0` [kanıtlı: `lib/optimizers.tr:96-97, 111`]; kitaplığın yolu `tr/optimizers`. Kitaplık parametre gruplarını birden çok iyileştiriciyle yazıyor [kanıtlı: DENETİM E1]. 15.34'ün "kendi iyileştiricisi" TR testi indeksli atamayla yazılı (`tests/tensor_test.tr:401`).
- **Yapılacaklar:** yaprağın `.TR_requires_grad` üyesi için yeni üye kodu (gösterim adı "türev_izle", onaylı); dondurma, aşamalı çözme ve iyileştiricinin donmuş yaprağı atlaması; gradyansız `.TR_step()` hatası; türevsiz kipte 16 bitlik yaprağa bileşik atamanın uyarısı; kitaplık ve 15.34'ün TR testi bileşik atamalı yazıma geçer (başlık da değişir); donma durumunun kontrol noktasına girmesi (20.4); TR testleri.
- **Gemini Koşu 1 (2026-10-29):** parametre başına "gradyan eklendi" bilgisi, gradyansız parametreyi atlayan `.TR_step()`, donmuş grubun hatasızlığı, açmanın yalnız kendi deposu olan yaprakta geçerliliği uygulanacak. Kendi iyileştiricisi (`TR_if g != TR_none`) kullanılmayan parametreyi bugün ayıramıyor; kitaplığın Muon'u için soru üyesi Sonraya.
- **Açık (Sonraya):** aynı yapıya eklenen LoRA alanları için `TR_load_parameters`'ın kesin kuralı; bugün yol alt yapıya yüklemek (`TR_load_parameters(model.gövde, ...)`).

### 15.36

- **Mevcut durum:** `TR_is_finite` float32 0/1 veriyor [ölçüldü]; 15.36'nın TR testi float listeleri bekliyor (`tests/matematik_test.tr:261-262`).
- **Yapılacaklar:** `TR_Tensor[TR_Bool]` dönüşü (15.3); test ve 15.36'nın örneği (`.TR_item()` ile) buna göre.
- **Gemini Koşu 3 (2026-10-29), uygulanacak:** kalıp kaybı da sınar; belgelerdeki ve örneklerdeki eğitim döngüleri buna göre.

### 15.37

- **Mevcut durum:** `TR_default_device(" CPU ")` kabul ediliyor [ölçüldü]; `.TR_to_device(" CPU ")` de kabul ediliyor ve TR testi bunu sabitliyor (`tests/tensor_test.tr:1023`). `"gpu:0"`'a taşınan tensörün `.TR_device`'ı `"gpu"` veriyor, test bunu bekliyor (`tests/tensor_test.tr:1031`) [kanıtlı]. Desteklenen GPU yalnız NVIDIA.
- **Yapılacaklar:** tek yazım (büyük harf ve boşluk hata); `.TR_device` uzun ad; iki test satırı tersine döner; TR testi. Başka hızlandırıcılar (Apple, gömülü NPU) yol haritasında.

### 15.38

- **Gemini 15/2 (2026-10-29), uygulanacak:** `TR_load_tensors` varsayılan cihazda kurar (T3).

### 15.41

- **Mevcut durum:** GPU'daki matematik işlevlerinin (`ex2.approx`, `lg2.approx` yolları) ulp sınırı ölçülmedi; 15.11 GPU için "birkaç ulp" diyor.
- **Yapılacaklar:** GPU sınırlarını ölçüp 15.10 ve 15.11'e yazmak; cihazlar arası sözün Go testi.

### 15.42

- **Mevcut durum:** `TR_zeros`, `TR_ones`, `TR_randn` ve boş tensörün `.TR_reshape`'i ile `.TR_tolist()`'i şekli, sıfır eksenleri 1 sayarak 50 milyon öğelik koleksiyon sınırına göre denetliyor (`TR_zeros([0, 1000000000, 1000000000])` hata) [kanıtlı: bugünkü 15.42'nin metni]. GPU çekirdeklerinin indeksi 32 bit: bir uygulama sınırıdır, dil kuralı değildir; Llama 3 405B'nin gömmesi (~2,1·10⁹ öğe) bu sınıra dayanıyor.
- **Yapılacaklar:** 50 milyon sınırı kalkar, yerine int64 taşma denetimi (yukarıdaki örnek artık hata değildir; dev sonuç ayrılırken yakalanır); 64 bitlik GPU indeksi yol haritasında.

### 15.43

- **Gemini 15/2 (2026-10-29), uygulanacak:** ifadede kurulan liste ve demet içindekilerin sahibidir (`TR_concat([a, b])` GC'yi beklemez); 0.6b'nin demet sahipliğiyle aynı kural, liste yanı D1.
- **DENETİM 0.6b (2026-10-29):** `.TR_append` ve indeksle atamada yalnız yazılan değer kaçıyor (kurala uygun); öbür değiştiren yollar (ör. `+=`) bütün listeyi kaçırıyor: güvenli taraf ama sapma. Test satırına 0.6b'nin iki sahiplik testi girecek.

### 15.44

- **Mevcut durum:** `TR_fuse`'un kapanışından okunan sayı değişince sonuç sessizce eski kalıyor (sabit 2'den 5'e çıktı, sonuç `[2, 4]` kaldı) [ölçüldü]; CPU'da `TR_fuse` eski değeri, `TR_replay` yeni değeri veriyor, GPU'da `TR_replay` de sabitliyor: aynı program cihaza göre ayrı sonuç veriyor [ölçüldü: DENETİM]. Go testi `TestFuseCapturedNumberIsBakedIn` bu sessiz eskimeyi doğru davranış diye sabitliyor (`internal/stdlib/ml/builtin_fuse_test.go:8`). `TR_clamp`, `TR_atan2` ve `TR_where` kaynaştırılmıyor.
- **Yapılacaklar:** kapanıştaki sayının kaydı ve her çağrıda, her cihazda denetimi; yolu olmayan sayıda kayıt hatası. Kural uygulanınca `TestFuseCapturedNumberIsBakedIn` tersine döner: değişen kapanış sayısı hata vermeli; TR testi.
- **Uygulama sınırı:** `TR_clamp`, `TR_atan2` ve `TR_where` bugün kaynaştırılmıyor; `f`'de kullanılınca hata veriyor. Kural onları da kapsar.
- **Gemini 15/2 (2026-10-29), uygulanacak:** kapanış yolunda liste indeksi ve sözlük anahtarı (2. kilometre taşının ön koşulu); `TR_fuse`'da float64 ve tam sayı kendi türünde (T3).

### 15.45

- **Mevcut durum:** kayıtta `TR_embedding_lookup`'ın tensör indeksi ve `TR_write_slice`'ın tensör başlangıcı sınır dışındaysa sessizce kırpılıyor (gömmede `[0, V−1]`, `TR_write_slice`'ta hedefe sığan son konum), kesirli değer budanıyor [kanıtlı: bugünkü 15.45'in metni]. Kapanıştan okunan sayılar kayıtta sabitleniyor, denetlenmiyor [ölçüldü: DENETİM].
- **Yapılacaklar:** kırpma kalkar, cihaz içi hata bayrağı (15.6); kapanıştaki sayının ve tensör deposunun yoluyla denetimi; rastgele üretecin oynatmada ilerlemesi; Go testleri.

### 15.46

- **Mevcut durum:** tensörler yalnız float tutuyor; `TR_save_tensors` float32, bf16, fp16 yazıyor (sürüm 2; tür kodları 1, 2, 3) [ölçüldü]. Dosyada veri taşıyan tensör en çok 2³² öğe, verisiz şekil (sıfır eksenler 1 sayılarak) en çok 50 milyon öğe bildirebiliyor [kanıtlı: bugünkü 15.46'nın metni]; 15.46'nın sıfır uzunluklu eksen TR testi (`tests/tensor_test.tr:551`) `[1048576, 0, 8192]` şekilli verisiz tensörün reddini bekliyor.
- **Yapılacaklar:** her cinsin ve hassasiyetin tür kodu (sürüm 2 kalır; bugünkü dosyalar okunmaya devam eder); `TR_AnyTensor` ve `.TR_narrow[E]()`; 2³² ve 50 milyon sınırları kalkar, öğe sayısı int64 taşma denetimiyle ve ayırma 15.42'nin bellek denetimiyle (sıfır eksen testi yeni kurala göre yeniden yazılır); uzantıyla biçim seçimi; TR testi.
- **Gemini 15/2 (2026-10-29), uygulanacak:** `TR_requires_grad: TR_true` yalnız nicemlenmemiş `TR_Float` tensörlerde (T3).

### 15.47

- **Mevcut durum:** safetensors yalnız okunuyor; yalnız F32, BF16, F16 destekli [ölçüldü], öbür türler (I64, U8, F8 ...) hata [kanıtlı: bugünkü 15.47'nin metni].
- **Yapılacaklar:** 15.1'in bütün cinsleri ve hassasiyetleri; `F8_E4M3`'ün ölçeği 1 "fp8" olarak okunması (15.55).

### 15.47a

- **Mevcut durum:** safetensors yazma yok [ölçüldü].
- **Yapılacaklar:** `.safetensors` uzantısıyla yazma, Python `safetensors` ile gidiş dönüş Go testi.

### 15.49

- **Mevcut durum:** karşılaştırma float32 0/1 maske veriyor; tam sayı ve mantık tensörü olmadığından cinslerin karışmaması, tam sayı taşması ve daraltma satırları uygulanmadı.
- **Yapılacaklar:** satırlar kendi maddeleriyle (15.2, 15.3, 15.5, 15.6a–15.6d) uygulanır; TR testi.

### 15.50

- **Mevcut durum:** float32'nin ucundaki büyük sonlu sayılarda CPU float64'te biriktirdiği için sonlu, GPU float32'de biriktirdiği için sonsuz sonuç verebiliyor (normlarda x², toplamın kendisi 3,4e38'i aşınca). Karşılaştırma NaN'da 0 veriyor, `TR_where` NaN koşulu doğru sayıyor [ölçüldü]; yeni kural `TR_false` ve mantık koşulu (15.5, 15.15). `TR_logsumexp`, `TR_maximum` ve `TR_minimum` yok; `TR_max`/`TR_min` ±0'ı eşit sayıp ilk öğeyi alıyor.
- **Yapılacaklar:** sayısal doğruluk işi (yol haritası Evre 4, madde 8); `TR_logsumexp`, `TR_maximum`/`TR_minimum` satırları ve `TR_max`/`TR_min`'in sıralama sırası (15.22b); TR testi.
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
- **Yapılacaklar:** TR biçiminde nicemlenmiş tür kodları; `F8_E4M3` okuma; `.safetensors`'a nicemlenmiş tensör yazma hatası; kitaplığın (`tr/pretrained`) GPTQ, AWQ ve FP8 parçalarını birleştirmesi; TR testi. GGUF okuma Sonraya (yol haritası, "Paylaşmak").

## 16. Veri ve parçalayıcı

Kurallar: [`docs/spec/16_veri_parcalayici.md`](spec/16_veri_parcalayici.md). Taslak (2026-10-29): ana oturum yeniden yazdı, önce CLAUDE.md'den türeterek (Ahmet: "dil maddeleri yalan yanlış olsa da düzeltmezsek bizi hep eskiye götürecek"). DENETİM bekleniyor. Yeni adların gösterim adları onaylandı (2026-10-29): `TR_train_tokenizer` parçalayıcı_eğit, `TR_reserved_tokens` ayrılmış_parçalar, `TR_write_dataset` veri_kümesi_yaz, `TR_shard_tokens` parça_boyu; kodları uygulandıkları gün kayda girer (17.4).

### 16.2

- **Gemini Koşu 7 (2026-10-29), uygulanacak:** `TR_special_tokens: TR_false` metin içi özel parçayı da kapatır.

### 16.3

- **Mevcut durum:** iki boyutlu tensör ya da liste listesi de metin listesine çözülüyor; dağarcıkta olmayan kimlik sessizce atlanıyor [ölçüldü: main `f1ebe718`].
- **Yapılacaklar:** `.TR_decode` yalnız tek dizi, birden çok dizi `TR_batch_decode` (13.8a); dağarcık dışı kimliğin hatası ve TR testi.

### 16.3 (sürüyor)

- **Test:** `tests/veri_parcalayici_test.tr`'nin 16.3 testi bugünkü toplu çözümü (liste listesi, 2 boyutlu tensör) sınıyor; `TR_batch_decode` gelince değişir.

### 16.4

- **Mevcut durum:** parçalayıcı eğitmek yok; bugün Python'a gidiliyor.
- **Yapılacaklar:** bayt düzeyinde BPE eğitimi, belirlenimci; `.TR_save` ile `tokenizer.json`; TR testi.
- **Gemini Koşu 7 (2026-10-29), uygulanacak:** dosya listesiyle eğitim; bayt düzeyi ön ayırıcı ve çözücü, eşitlik kuralı (Hugging Face ile bir kez sınanacak); özel parça eklemez.

### 16.5

- **Mevcut durum:** `.npy` okuyucusu işaretsiz 16 ve 32 bit ile işaretli 32 bit okuyor [kanıtlı: `internal/compute/dataload/npy.go`]; NumPy'nin varsayılanı olan işaretli 64 biti reddediyor [ölçüldü: DENETİM].
- **Yapılacaklar:** işaretli 64 bit; TR testi.
- **Gemini Koşu 7 (2026-10-29), uygulanacak:** önbellek anahtarı (ad, bayt boyu, değiştirilme zamanı); aynı adlı parçalar ayrı.

### 16.6

- **Mevcut durum:** veriyi yazan yerleşik yok; Go tarafında işaretsiz 16 bit yazıcı var (`WriteNpyU16`) [kanıtlı: `npy.go`].
- **Yapılacaklar:** `TR_write_dataset`, tür seçimi ve `meta.json`; TR testi.
- **Gemini Koşu 7 (2026-10-29), uygulanacak:** var olan kümenin yerine atomik yazma.

### 16.7

- **Mevcut durum:** `.TR_next()` iki öğeli liste veriyor (girdi ve hedef); tensörler float32 ve "tam sayılı işaretli" [ölçüldü]. Örnekleyici kontrol noktasına giriyor (20.2'nin TR testi).
- **Yapılacaklar:** demet dönüşü (3.6a) ve `TR_Tensor[TR_Int]` (15'in A'sı); göç: `.TR_next()`'i `[0]`/`[1]` ile okuyan yerler.
- **Gemini Koşu 7 (2026-10-29), uygulanacak:** pencere bağlam artı bir kimlik; kısa parçalar atlanır.

### 16.8

- **Mevcut durum:** sıralı okuyucu döngüyle geziliyor, liste veriyor [ölçüldü].
- **Yapılacaklar:** demet; döngüde açma (iki adla `TR_for`).

## 17. Kodlar

Kurallar: [`docs/spec/17_kodlar.md`](spec/17_kodlar.md). Taslak (2026-10-29): ana oturum yeniden yazdı (Ahmet: "17 şimdi düzeltsen ? sürekli kafanı karıştıracak"); 2. turda madde madde CLAUDE.md'den yeniden tartıldı (görünüm yalnız okunur); 3. turda DENETİM'in 1. turunun görünümden bağımsız bulguları alındı. DENETİM bekleniyor. 4a'dan önce gelir, çünkü 4a kod kaydına yeni kodlar ekler.

### 17.1

- **Mevcut durum:** kullanıcının yapısında `TR_` adlı alan ve metot sessizce kabul ediliyor (`TR_struct K { TR_step }`), oysa `TR_var TR_x` reddediliyor [ölçüldü: DENETİM]. Testler ve örnek düz ada geçti (`tests/coded/ornek_kodlu.tr`'nin yapısı, `examples/tr/17_testler_test.tr`'nin `Sepet`'i).
- **Yapılacaklar:** yapının alanında ve metodunda `TR_` adının denetim hatası, `TR_init` ve `TR_next` dışında; TR testi. Göç: `tools/gpu/measurement_model.tr:36-55` kullanıcı yapısında `TR_forward` (2) ve `TR_starts_with` (1) tanımlıyor [ölçüldü: DENETİM]. Kullanıcı yapısının `TR_next` ile dolaşılması bugün çalışmıyor ("Sayaç tipi üzerinde döngü kurulamaz") [ölçüldü: DENETİM]; 5.7 uygulanınca TR testi.

### 17.2

- **Mevcut durum:** program metninde kısa yazım, içeride sınıflı kimlik uygulanmış (katman 1). 60 kısa ad birden çok sınıfta, hepsi yerle ayrılıyor; aynı yerde çakışan yok, `TestNoTwoIdentitiesShareASpellingInAPlace` sınıyor [ölçüldü: DENETİM].

### 17.3

- **Gemini Koşu 6 (2026-10-29), uygulanacak:** kitaplık parametresinin ve alanının yeri; görünüm ve kod dizini kitaplık gövdesinde kapsama bakar.

### 17.4

- **Mevcut durum:** 565 kodun 245'i kitaplık kodu; kitaplık kodlarının hiçbirinin açıklaması ve örneği yok, `TestEveryCodeHasDescription` onları atlıyor ("madde 15 adım 3c") [ölçüldü].
- **Yapılacaklar:** kitaplık kodlarının açıklaması ve örneği; atlama kalkar. Dil tanımına giren ama uygulanmamış adlar (13'ün ve 15'in yeni adları) uygulandıkları commit'te tanım ve dil dosyalarına girer.
- **Mevcut durum (Faz 0.5, 2026-10-29):** 320 örnek kimlik yazımıyla tanım dosyasında (`definitions.json`, `example`); dil dosyaları örnek taşımaz, ayrıştırıcısı alanı reddeder. Taşıma bir betikle, o günkü kodlayıcıyla yapıldı (elle değil). `TestExamplesRun` program örneklerini (265) kimlik okuyucusuyla (`lexer.NewBody`) okuyup denetler ve çalıştırır; komut satırı örnekleri (55: komut, alt komut, seçenek, ortam değişkeni, kabuk ve hata ayıklayıcı komutu) kimliğin adıyla yazılı ve program olarak sınanmaz [ölçüldü]. `docs/codes.md` örneği kimlik yazımıyla ve her gömülü dilin görünümüyle gösterir.
- **Yapılacaklar:** komut satırı örneklerinin sınanması (komut satırı adları 17.9'un girdisiyle, Faz 0.4).

### 17.5

- **Mevcut durum:** ad alanında çakışma dil dosyası yüklenirken reddediliyor (`TestClashesCaught`). Türkçe harfsiz biçim kuralı kalktı (17.7). Dayanağı kalmayan ASCII denetimi ve öneri yolu (`AcceptASCII`, `language.go:33-40`, `TestASCIIFormsDoNotClash`) ile alışkanlık adının gösterim adıyla çakışma reddi (`TestClashesCaught`'un o kısmı) kalkacak.
- **Mevcut durum:** `TR_assert_equal`'ın Türkçe adı "eşit_olmalı" oldu (2026-10-29, önceki ad soru gibi okunuyordu).

### 17.7

- **Mevcut durum (Faz 0.5, 2026-10-29):** görünümden geri kodlama kalktı: kodlama komutu (tr kodla; kaydı silindi, iletileri `retired.json`'da), `internal/lang/source`'un kodlayıcısı (`Encode`, `EncodeIn`, `TextLexer`, `TextLexerWithNames`), gösterimin geri kodlama denetimi, tek seferlik çevirici `tools/migrate` [ölçüldü: `git grep`]. Yaklaşık kodlama da onunla kalktı. `internal/lang/source` 1 870 satırdan 1 441 satıra indi (testler dahil) [ölçüldü: wc].
- **Görünüm:** `TR_cmd_show` görünümü yazar (`source.Show`); görünüm başlıklı dosya (`# TR 4 view=tr`) çalıştırılınca ve denetlenince ileti (H2541) kodlu dosyayı çalıştırmayı söyler; başlıksız dosyanın iletisi (H2540) başlığı ve kimlik yazımını söyler, kodlamayı önermez. Görünüm dilin sözcüğüyle aynı yazılan kullanıcı adını `$ad` diye işaretliyor (kural değil, 17.7).
- **Kabuk ve hata ayıklayıcı:** kabuk satırı ve hata ayıklayıcının ifadesi (koşul, izleme, değerlendirme) kimlik yazımıyla okunuyor; Türkçe adıyla yazılan yazdırma çağrısı tanımsız ad hatası veriyor [ölçüldü: Go testi]. Kabuğun ve hata ayıklayıcının komut adları (`TR_repl_exit`, `TR_dbg_eval`) 17.9'un işi (Faz 0.4).
- **Düzenleyici:** VS Code eklentisi dosyayı kimlikleriyle açıp kaydediyor; görünümü yanda salt okunur açıyor (`view.js`); görünümde düzenleme ve kayıt (`coded_*.js`) kalktı.
- **Kalan:** sözcük çözümleyicinin bir yazım dilinde okuma kipi (`lexer.NewBodyIn`'in kodsuz kipi, onu izleyen `$` kaçışı, `WrittenIn`, denetleyicinin `SetFileLanguage`'ı) yalnız Go testlerinde kullanılıyor [ölçüldü: `git grep`, 31 test yeri]; kaldırılması ayrı bir iş.
- **Testler:** `TestShowTranslatesOnlyTheLanguageWords` (yalnız görünüm: satırlar, yerleştirmenin çevrilmesi, kullanıcı adının kalması), `TestViewHeader` ve `TestShowCommand` (görünümün çalışmaması), `TestTypedLineIsReadWithIdentities`, `TestCLIExpressionIsWrittenWithIdentities`, `TestHeaderlessFileDoesNotRun`.
- **Yeniden kurulacak:** Faz 0.5'te iki atlanmış sınav silindi; öncülleri bir yazım dilinde yazılmış dosyaydı (dosyanın dili 2.1'de kalktı). Görüntü sınavı (`TestDisplayExam`): kişiye görünen her yüzey okuyanın dilinde. Ad çakışması sınavı (`TestNameClashExam`): başka bir dilin sözcüğü kullanıcının adı olunca ya hata ya aynı sonuç. 17 yeniden yazılınca kodlu dosyalar ve görünümle baştan kurulur.

### 17.8

- **Mevcut durum:** değerin metni dilden bağımsız ("true", "none", "List", "<builtin print>") [ölçüldü]. Kitaplık tipinin metni sınıflı kimlik veriyor: `TR_print(TR_Type(TR_Linear(2, 3, TR_seed: 1)))` → "TR_type_Linear" [ölçüldü: main `1ef66921`].
- **Yapılacaklar:** tipin metni kimliğin adı ("Linear"); TR testi.
- **Gemini Koşu 6 (2026-10-29), uygulanacak:** kişinin dilinin ortam değişkeni 17.9 adıyla (Faz 0.4).

### 17.9

- **Mevcut durum (Faz 0.4, 2026-10-30):** komut satırı girdisi ("cmd", "sub", "opt", "repl", "dbg", "env") yalnız kimliğin adını kabul ediyor ("tr run", "tr index definition", "--lang", "exit", "step", "TR_LANG"); gösterim dilindeki ad hata veriyor ve kimliğin adını öneriyor (H8875–H8876) [test: `internal/lang/codes/language_test.go`, `cmd/tr/e2e/behavior_commands_test.go`]; yardım ve iletiler yazılacak kimlik adını ve kişinin dilindeki açıklamayı gösteriyor; depodaki yazımlar (186 dosya) kimlik adına göç ettirildi [ölçüldü: `eski_denetimler/faz0_4_betik.py`].

### 17.10

- **Mevcut durum:**
  - dil tanımında 10 bölümde 1 034 eski sınıflı yazım var: 15'te 557, 20'de 187, 18'de 106, 19'da 74, 16'da 35, 11'de 32, 12'de ve 10'da 15'er, 14'te 5 [ölçüldü: DENETİM]; öbür belgeler 2.1'in satırında;
  - başka bölümler eski 17'yi anlatıyor: 14:11 (gösterime göre sütun), 19:13 ve 19:28 ile 20:32 (kodlu sözlük anahtarı, bugün sözdizimi hatası); 17'ye göndermeler eski numarada (20:47 ve 20:67 → 17.8, 20:102 → 17.10, 19:13 → 17.9) [kanıtlı: DENETİM].
- **Yapılacaklar:** bölümler yeniden yazılırken kimlik yazımı ve 17 göndermeleri; 15 bu turda yazılıyor, 19 ve 20 sırada.

## 18. Dosya ve yol

Kurallar: [`docs/spec/18_dosya_ve_yol.md`](spec/18_dosya_ve_yol.md). **Kapandı** (2026-10-29): ana oturum CLAUDE.md'den türeterek yeniden yazdı, iki tur DENETİM. 2. tur (2026-10-29): DENETİM 18-1'in bulgularıyla. Yeni adların gösterim adları onaylandı (2026-10-29): `TR_write_lines` satırları_yaz, `TR_LineWriter` SatırYazıcı, `TR_write` yaz (18.17); `TR_ignore_case` harf_duyarsız (18.5); kodları uygulandıkları gün kayda girer (17.4).

### 18.1

- **Gemini Koşu 5 (2026-10-29), motor zaten böyle:** kök ve UNC öneki korunuyor [ölçüldü: DENETİM]. **Gemini Koşu 5 (2026-10-29), uygulanacak:** `..` ile tabandan kaçış hatası (bugün `TR_Path("veri", "../../gizli/x.json")` tabandan çıkıyor [ölçüldü: DENETİM]).

### 18.2

- **Gemini Koşu 5 (2026-10-29), motor zaten böyle:** noktayla başlayan iki noktalı adlarda son uzantı [ölçüldü: DENETİM]. `.TR_parent`'in kökte değeri sınanacak.

### 18.5

- **Mevcut durum:** Windows'ta kalıp harf büyüklüğüne bakmıyor: `"*.TXT"` `a.txt`'yi eşliyor [ölçüldü: main `dc4930ca`].
- **Mevcut durum:** başka bir klasörden çalışan programda `TR_list_dir("cikti", "*.tmp")` programın klasöründeki `cikti`'yi buluyor ve göreli yollar veriyor [ölçüldü: DENETİM 18-1].
- **Yapılacaklar:** her sistemde harf büyüklüğüne duyarlı eşleme (göç: `internal/stdlib/files/list_test.go`'nun harf büyüklüğüne bakmayan dalı), `TR_ignore_case:`, başka klasörde bulunan sonuçların mutlak olması; TR testi.

### 18.7

- **Gemini Koşu 5 (2026-10-29), uygulanacak:** `TR_Logger` ile açık dosya da silinmez.

### 18.8

- **Gemini Koşu 5 (2026-10-29), uygulanacak:** son bileşeni bağlantı olan yolda bağlantının kendisi değerlendirilir.

### 18.9

- **Gemini Koşu 5 (2026-10-29), uygulanacak:** dolu klasörün yerine konması (eski klasör geçici ada, sonra silinir); 18.10 da.

### 18.13

- **Gemini Koşu 5 (2026-10-29), uygulanacak:** bağlantıya atomik yazmada geçici dosya hedefin klasöründe.

### 18.16

- **Mevcut durum:** değiştiren işler de göreli yolu okuma sırasıyla çözüyor: başka bir klasörden çalışan program `TR_remove("yanda.txt")` ile programın klasöründeki dosyayı siliyor, `TR_write_file("prog_yolu.txt", ...)` programın klasöründeki dosyayı eziyor; `TR_make_dir` ve `TR_copy`'nin hedefi çalışma klasörüne gidiyor [ölçüldü: main `dc4930ca`].
- **Yapılacaklar:** `TR_write_file`, `TR_remove`, `TR_move` ve 18.17 yalnız çalışma klasörüne göre; TR testi.
- **Açık karar (Gemini Koşu 5):** okuma üç klasöre bakıyor, değiştirme yalnız çalışma klasörüne; `TR_list_dir`'in mutlak sonucu ve `.TR_exists` ile `TR_remove`'un ayrı dosyayı görmesi bu bakışımsızlıktan. İki yol: (a) her iş için tek kural, çalışma klasörü (Python ve Go gibi; programın yanındaki veri için açık bir taban gerekir, yeni ad); (b) bakışımı koruyup okuma sırasıyla başka klasörde bulunan yolu değiştiren işe verince hata. DENETİM (a)'yı öneriyor. Karar S1'in yönergesinden önce, programın klasöründen okuyan test ve örneklerin sayısı ölçülünce.

### 18.17

- **Yapılacaklar:** `TR_write_lines` ve `TR_LineWriter` (atomik, `.gz`, satır sonu içeren satır hatası, program sonunda kapanma, açık yazıcının yolunu okuma ve ikinci yazıcı hatası); TR testi.
- **Gemini Koşu 5 (2026-10-29), uygulanacak:** ertelenen `.TR_close()` çıkışın türüne göre işler ya da siler; yorumlayıcının ertelenen işe çıkışın türünü veren kancası (Faz 0, D1 ile S1'in ortak yeri).

## 19. Süreç ve kayıt

Kurallar: [`docs/spec/19_surec_ve_kayit.md`](spec/19_surec_ve_kayit.md). **Kapandı** (2026-10-29): ana oturum CLAUDE.md'den türeterek yeniden yazdı, iki tur DENETİM. Numaralar eski metinle aynı; eski başlıksız "Komut satırında yazım" 19.10, "Görünüm" 19.11 oldu.

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

- **Mevcut durum:** yardım seçeneğinin Türkçe ve İngilizce adı ikisi de çalışıyor; mantık değerinde Türkçe yazım da okunuyor; ondalıkta Türkçe "sonsuz" okunmuyor; yardım varsayılanları kişinin dilinde gösteriyor [ölçüldü: main `217ae940`].
- **Yapılacaklar:** tek yazım: yalnız "--help" ve "-h", "true" ve "false", "inf", "-inf" ve "nan"; yardımın varsayılanları komut satırının yazımıyla; yalnız "help" ve "h" ayrılmış ad (bugün her dildeki yardım adı ayrılmış); `internal/stdlib/process/process_test.go`'nun Türkçe mantık değeri; Go testleri.
- **Göç:** yardımın Türkçe adı 26 dosyada 47 satırda [ölçüldü: DENETİM 19-1]. İkiye ayrılır: `tr`'nin kendi seçeneği 17.9'un göçüdür (`cmd/tr/main.go`, `options.go`); programın seçeneği 19.10'unkudur (`internal/stdlib/process`, `examples/tr/37_komut_satirinda_ayar.tr`, `39_veri_ile_egitim.tr`, `tools/gpu/gpt_training.tr`, rehber).
- **Gemini Koşu 5 (2026-10-29), uygulanacak:** yardımda `TR_none` varsayılanı yazılmaz; kullanım hatası iletisi gizli adlı seçeneğin değerini yazmaz. **Açık:** standart hataya yazan yerleşik yok (yol haritası).

### 19.11

- **Mevcut durum (Faz 0.4, 2026-10-30):** iş klasörünün ortam değişkeni "TR_JOB_DIR" (`TR_env_JOB_DIR`); çubuğun dili "TR_LANG"'dan (`TR_env_LANG`) okunuyor [ölçüldü: kod].

## 20. Kontrol noktası

Kurallar: [`docs/spec/20_kontrol_noktasi.md`](spec/20_kontrol_noktasi.md). Taslak (2026-10-29): ana oturum CLAUDE.md'nin "kesilen bir eğitim kaldığı yerden aynı sonuçla sürer" cümlesinden türeterek yeniden yazdı; bölümün çoğu uygulanmış ve testli. DENETİM bekleniyor.

### 20.1

- **Mevcut durum:** koşu klasörü okuma kuralıyla çözülüyor (kontrol noktası paketinin 178. satırı): başka klasörden çalışan program programın klasöründeki koşuyu sürüp oraya yazıyor [ölçüldü: DENETİM].
- **Yapılacaklar:** yazma kuralı (13); TR testi.

### 20.2

- **Mevcut durum:** sözlük anahtarlarında eski kodlu anahtar kuralı metinden kalktı (bugün sözdizimi hatası). Tam sayılı işaretli tensör (argmax'in sonucu) model olarak kaydediliyor [ölçüldü].
- **Yapılacaklar:** gerçek tam sayı ve mantık tensörlerinin (15'in A'sı) kaydı ve yüklemesi; adlar `TR_named_parameters` ile (13.8a).

### 20.4

- **Mevcut durum:** dondurma (`.TR_requires_grad` üyesi) yok, nokta donma durumunu kaydetmiyor (15.34'ün status'u) (15'in yan değişikliği, uygulanmadı).
- **Yapılacaklar:** donma durumunun kaydı ve sürdürmede geri yüklenmesi; TR testi.

### 20.11

- **Mevcut durum:** tam sayılı işaretli tensör ve maske ondalık parametreye sessizce yazılıyor [ölçüldü: DENETİM].
- **Yapılacaklar:** cins farkının hatası (15'in A'sı gelince); `TR_AnyTensor` sözlüğü ve parametrenin cinsine daraltma, daraltan hassasiyet dönüşümünde sığmama hatası, nicemlenmiş yaprağa bütün yazma (15.54) (15'in yan değişikliği, uygulanmadı).
- **Açık (Sonraya, 2. kilometre taşında öne çekilmeli):** hazır `.safetensors` dosyaları tablo (RoPE) taşımaz; "bütün yapraklar sözlükte" kuralıyla yükleme alt yapıya yapılır. LoRA alanlarıyla aynı soru (15.34'ün açığı).
- **Gemini 15/2 (2026-10-29), uygulanacak:** nicemlenmiş hassasiyet farkı yüklemede hata, sessiz açma yok (T2, 15.54 ile).

### 20.12, 20.13

- **Mevcut durum:** `TR_load_state`'in mantık değeri dönüşü 13.10'un status'unda. Bütün tensörler float32 olduğu için `op.TR_state`'in "step"i de ondalık tensör (15.3'ün status'u); karışık cinsli `TR_Dict[TR_String, TR_AnyTensor]` yok (15'in yan değişikliği, uygulanmadı).

### 20.16

- **Mevcut durum:** nazik durdurmada süreç 3 koduyla, ikinci istekte 130 koduyla çıkıyor; 1.9'un "isteğin sinyaliyle biter" kuralı bekliyor.

### 20.17

- **Mevcut durum:** paylaşılan üreteç `math/rand` (v1), saatle tohumlanıyor ve durumu yazılamıyor; kontrol noktası kurulduktan sonra tohumsuz rastgelelik bir uyarı veriyor [ölçüldü: DENETİM].
- **Yapılacaklar:** sabit başlangıç durumu ve yazılabilen bir üreteç (Go'nun `math/rand/v2` PCG'si `MarshalBinary` veriyor); kaydı ve geri yüklenmesi; TR testi. Rehberin (`docs/guide/09_uzun_egitim.md:40`) örneği Türkçe görünüm adlarıyla yazılmış.

### 20.14

- **Mevcut durum:** geri dönüş `.TR_grad`'ları temizlemiyor, donma durumunu döndürmüyor (Gemini Koşu 1, DENETİM). **Yapılacaklar:** 20.14'ün yeni cümlesi; TR testi.
