# Durum

Dil tanımının yeniden yazılan bölümlerine göre bugünkü TR'nin durumu ve kurala varmak için yapılacaklar. Her maddenin kuralı dil tanımındadır ([`docs/spec/`](spec/README.md)); yapılan iş testiyle birlikte buradan silinir. **Mevcut durum** yalnız kuraldan farkı yazar: eksik, yanlış, sınır ve ölçülmemiş olan. Kurala uyan davranış ve kapanan parça yazılmaz; kapanan parça kapandığı commit'te silinir. Kaynak olarak açık iddianın etiketi kalır, geçmişi git tutar. Kural bir yerde "denetim hatası" diyorsa hatanın çalışma anında gelmesi de farktır. İşlerin sırası [yol haritasındadır](roadmap.md). Bu belgede kimlikler dil tanımının yazımıyla anılır (`TR_if`, 2.3a).

## Önem ve öncelik

Sınıflar ve öncelikler TCB'nin taslağıyla DENETİM'in hükmünden (istek 61, 2026-11-03; 6–9. bölümler istek 68). Etiket her maddenin "Yapılacaklar"ında: sınıf, öncelik, kol ya da dilim ve ölçüm tabanı, ör. **Yapılacaklar (A, Ö1, 4a4; ölçüldü 30b44b01)**. Etiketsiz madde D, E ya da F sınıfındadır ve Ö3'tür. Etiket ölçüm tabanını taşır: taban eskidikçe iddia yeniden ölçülür.

**Sınıf karar anında yazılır** (DENETİM istek 76): her karar ve bulgu yazılırken (yol haritası, status ya da `pending.md`) sınıfı da yazılır. A, B ya da C ise aynı gün buraya etiketli bir "Yapılacaklar" satırı açılır; dil tanımında bölümü olmayan iş "Bölümü olmayan işler"e girer. D, E ve F yol haritasında kalabilir. Önem taramaları status'un yanında `pending.md`'yi ve TCB'nin takip tablosunu da okur; yol haritasını TCB ya da DENETİM okur.

**Sınıflar** (sıra tasarım ölçütlerinin sırası: doğruluk, güvenlik, ajan için hatasız):
- **A, sessiz yanlış:** program hata vermeden yanlış sonuç, yanlış veri ya da yanlış gradyan veriyor; diskteki veriyi sessizce kaybediyor ya da bozuyor; ya da bir test bu yanlışı doğru diye sabitliyor; ya da dilin kendi aracı (ikiliyle gelen komut, eklenti) ya da belgesinin veya iletisinin verdiği komut hata vermeden yanlış sonuç gösteriyor: program çalışmadan geçmiş görünüyor (DENETİM istek 76).
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
1. 13'ün yerleşiklerinin sessiz yanlışları (DENETİM istek 81, önem taraması teslim 3): 2a (`654c2459`) ve 2b birleşti; modülün içinden yazılan göreli yolun modül klasörüne düşmesi (13.1a, diskte veri eziyor) iki teslimin listesinde yok.
2. 15.6d, yerinde yazma kopyadan geçmez (A; T1; DENETİM 342 Ş4): hedef zincirinde kopya veren adımın hatası, denetleyici ve çalışma anı. İletinin önerisi `.TR_select` ve tensörle indekse yazmaya (15.6d'nin E satırları) bağlı; o gelene kadar H7042'nin çalışan döngüsü.
Sonra Ö2: 8.5'in 41 yolu.

**Ö1, dilimlerin kesin listeleri:** 4a5: 11.8, 13.8, 6.9 ve 7.7 (ya da 4c), 1.2, 3.13, 5.5, 15.5; argümanın, alanın, dönüşün, varsayılanın ve açılan adın tipi (DENETİM istek 68, 75): 6.1, 6.3, 6.4, 6.5, 6.6, 7.2, 7.8, 8.1, 8.3, 7.6 (ya da 4c). 4b: 3.5, 6.2, 5.12, 5.6, 7.4. 4c: 3.14, 15.1'in öğe cinsi. Kol A: 8.5'in işletim sistemi hatası (DENETİM istek 75). Kol B: 20.11, 15.27a, 15.33, 15.45. Asistan araçları (roadmap, yerel sesli asistan): 18.1'in `TR_Path` denetiminden geçmeyen yolları. Yerli ses girişi (roadmap, yerel sesli asistan 6. adım): 21.2'nin sınırı, kaynağın kendi kaybı.

**Önem taramasının sınırı** (Gemini'nin bağımsız taraması, DENETİM istek 63–67, 81; taban `30b44b01`): tarama "Yapılacaklar"ın hepsini, "Mevcut durum"dan risk sözcüğü geçen cümleleri okudu; sözcüksüz anlatılan bir sessiz yanlış kaçmış olabilir. 16.6 ve 18.9'un (kesilen yazma) sessiz olup olmadığı ölçülmedi.

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

- **Mevcut durum:** hesap çekirdeğinin (CPU) ortasında bırakma yok, süren hesap biter; ağ yerleşiği yok. Dış programa istek 21.5'teki gibi gider, `TR_run_process` ve `TR_spawn_process` durur, program sonra biter [test: `TestStopRequestStopsTheExternalProgram`, `TestSecondRequestKillsTheExternalProgram`].
- **Yapılacaklar:**
  - Bırakılan hesabın GPU'da başlamış işi beklenir, ertelenen işler sonra çalışır; CPU çekirdeğinin ortasında bırakma.

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

Kurallar: [`docs/spec/03_tipler.md`](spec/03_tipler.md). TR bugün dinamik tipli; tip açıklamalarını yalnız çalışmadan önceki denetleyici kullanıyor. `tests/tipler_test.tr`'nin 3.6 testi (`tests/tipler_test.tr:116-117`) eski kuralı (karışık tipli liste) sınıyor; kural uygulanırken yeniden yazılır. Eski 3.15'in (katalog iletisinin tipi) `tests/` altındaki testi hata bölümüne taşınacaktı.

### 3.3a

- **Yapılacaklar:** dosya ve ağ okumalarının bayt da vermesi (ölçülmedi).

### 3.5

- **Mevcut durum:** isteğe bağlı değerin asıl tipin yerinde kullanılması ad için denetlenmiyor (ad olmayan `T?` ifadesi denetim hatası; daraltma yok, aritmetikte tipi `TR_Any`, 4b) [ölçüldü: `TestOptionalTypes`, `TestUnopenedOptional`].
- **Yapılacaklar (A, Ö1, 4a; DENETİM istek 97, 103 Z3):** tipi bilinmeyen çağrı denetimden geçiyor, çalışma anı `TR_none` veriyor; yedi yol kaldı [ölçüldü: 2b, altı küçük program, hepsi `TR_none` yazıyor; yedincisi DENETİM 237]: tipi bilinmeyen alıcıda `.TR_append`'in dönüşünün kullanılması; ada bağlanan yerleşik (`TR_var f = TR_print`, `TR_var y = f("a")`, tip `TR_BuiltinFunction`), sözlükte ve listede tutulan fonksiyon değeri, `TR_Function` parametresi, modül ve kitaplık fonksiyonu (4b) ve fonksiyon tipi denetlenmediği için (6.9) değersiz gövdenin `() -> TR_Int` bekleyen parametreden geçmesi (`TR_func w(g: () -> TR_Int) -> TR_Int { TR_return g() }`, `w { f() }`). Sahip: D1 (Kol A). Sıra: 4a5'in fonksiyon tipiyle önce ada bağlanan yerleşik, `TR_Function` parametresi ve fonksiyon tipli parametre (6.9); sonra sözlükte ve listede tutulan fonksiyon değeri (3.6); en son modül ve kitaplık fonksiyonu (4b).
- **Yapılacaklar (A, Ö1, 4a5):** `TR_wait_all(görevler)`'in `TR_NoneType` listesi.
- **Mevcut durum (DENETİM 229, 237):** `TR_Any` yazılmadan da dokuz yol açık: yedisi 4a5'in satırlarında, biri 13.10'da, biri 4c'de. Sınırda çalışma denetimi yazılmaz (1.2; TCB kararı).
- **Yapılacaklar (A, Ö1, 4a5; DENETİM 229):** tipli yerde başka tip duran üç yolun satırı yoktu, denetim ve çalışma hatası yok [ölçüldü: DENETİM 229, `e8545745`]: `TR_catch e` (`TR_var y: TR_Int = e`), demet açma (`TR_var a, b = (1, "a")`, sonra `y: TR_Int = b`), öğesiz kap yazımı (`l: TR_List = ["a"]`, sonra `TR_List[TR_Int]`; `TR_Dict` de). Sahip D1.
- **Yapılacaklar (A, Ö1, 4b; ölçüldü 30b44b01):** adın açılmamış kullanımı denetim hatası; daraltma (4b) yalnız değişmeyen yerel değişkende: sınamanın dalında, `TR_else` dalında, `TR_and`/`TR_or`'un sağ yanında, her yolda biten erken çıkıştan sonra ve asıl tipten atamayla; alana atama daraltmayı bozmaz; ilk değeri `TR_none` olan açıklamasız değişkenin hatası (3.13).

### 3.6

- **Mevcut durum:** karışık tipli liste çalışıyor ve denetlenmiyor: `[1, "a"]` hatasız, denetleyici onu öğe tipsiz `TR_List` sayıyor; karışık listede `l[0] = "x"` geçiyor [ölçüldü: Gemini 4, DENETİM 166, 170]. Tipi bilinmeyen değer, öğe tipsiz kap ve dilim denetlenmiyor; çalışma anı öğe tipini taşımıyor [test: `TestElementAssignType`].
- **Yapılacaklar (A, Ö1, 4a5; ölçüldü 30b44b01):** öğelerin tipi aynı olur (karışık değişmez denetim hatası; 4a5: depoda 47 yer, CSV 13.8 ve `TR_forward:` 15.28'in göçü ve tipsiz değer kalıbı önce); boş liste tipini yazar (`TR_var k = []` hata, 3.13); `TR_List[T]` bir `TR_List[T?]` yerine verilmez (değişmez olmayan kapta).

### 3.6a

- **Yapılacaklar:** `TR_json_parse[T]` (13.8) motorda yok, demet tipli alanın diziden kurulması onunla gelir.
- **Yapılacaklar (D, Ö3; sahip D1; DENETİM 244):** tipi bilinen liste öğesi demetse `l[0][0]` denetimde değil çalışma anında hata veriyor; ileti `l[0].0`'ı öneriyor [ölçüldü: `74d38399`].

### 3.7

- **Mevcut durum:** karışık değerli sözlük öğe tipsiz. Tipi bilinmeyen anahtarda ve öğe tipsiz sözlükte başka tipte anahtar çalışma anında kalıyor: okuma H2647, `.TR_get` sessizce `TR_none`, `.TR_contains` `TR_false` [test: `TestDictKeyType`].
- **Yapılacaklar (A, Ö1, 4a5; ölçüldü 30b44b01):** değerleri karışık sözlük değişmezi (3.6'nın karışık liste kuralıyla).
- **Yapılacaklar (D, Ö3; DENETİM istek 70):** anahtarı anahtar olamayan tipte (liste, demette liste) yazılan `TR_Dict[K, V]`.

### 3.7a

- **Yapılacaklar:** genel tiplerin (Any, Object, StructType) kalkması (4a).

### 3.10

- **Mevcut durum:** hata ayıklayıcının koşullu durağı hâlâ doğruluk sınamasıyla (`IsTruthy`) okunuyor. Tipi bilinmeyen ve isteğe bağlı (`TR_Bool?`) koşul denetimde susar.
- **Yapılacaklar:** `.TR_filter`'ın yükleminin dönüş tipi (fonksiyon tipleri, ikinci dilim); isteğe bağlı koşul (daraltma, 4b); hata ayıklayıcının durağının koşulu da yalnız `TR_Bool` alır.

### 3.13

- **Mevcut durum:** tipi çıkarılamayan ilk değerin değişkeni `TR_Any` kalıyor (1.2).
- **Yapılacaklar (A, Ö1, 4a5; ölçüldü 30b44b01):** çıkarımın kalanı (1.2; bu dilimde isteğe bağlı tip, alan, `TR_if`/`TR_match`, indeks ve döngü değişkeni eklendi); ilk değeri `TR_none` ya da boş değişmez olan değişkende tip açıklamasının zorunluluğu. Tam sayının ondalık beklenen yerde ondalık olmasının kalanı tip haritasını ister: kabın öğesi (`l.TR_append(1)`, `l: TR_List[TR_Float]`, `[1]` ve `TR_Int` kalıyor; `d["a"] = 1` de; indekse değişkenle yazma da: `f: TR_List[TR_Float]`, `n: TR_Int` iken `f[0] = n` denetimden geçiyor, öğe `TR_Int` kalıyor, H1136 sayıları ayrı tip saymıyor [ölçüldü: DENETİM istek 70]) [ölçüldü: DENETİM istek 27 D-5], tam sayı değişkenli değişmez (`[a, 2.5]`) ve tipi ilk değerinden çıkan değişkene atama (`TR_var t = 0.0` sonra `t = 1`). `TR_Int` tipli değişkenin `TR_Float` yerine verilmesi (`f(i)`) denetim hatası olacak, bugün geçiyor ve motor değeri ondalığa çeviriyor; denetleyicinin işi.
- **Geri alınabilir göç (T1 dilim 2, bulgu 2):** T1, 3.13 yokken `TR_Float` parametresine tam sayı veren yerleri ondalık yazıma çevirdi (`p(n)` → `p(n.0)`): `tests/tensor_test.tr` "15.33 parametreler" testi ve `internal/stdlib/ml/model_tools_test.go`'nun `TestParameters`'ı. 3.13 main'de olduğuna göre bu yerler tam sayı yazımına dönebilir.

### 3.14

- **Mevcut durum:** tipin metni öğe tipsiz: `TR_Type([1])` "List" [ölçüldü].
- **Yapılacaklar (A, Ö1, 4c; ölçüldü 30b44b01):** "List[Int]" ve "Int?" (öğe tipi ister; 4a). Değer öğe tipini taşımadığı için `TR_Type([1]) == TR_Type(["a"])` `TR_true`, boş kapta `TR_Type(l)` "List" [ölçüldü: DENETİM, 2026-11-02, `kol-denetim-istek25-sonuc.md` A2]; parametrenin nereden geleceği (denetimde katlama ya da tip haritası) `pending.md` 3.11'de.

## 4. İşleçler

Kurallar: [`docs/spec/04_operatorler.md`](spec/04_operatorler.md). Taslak: metin DENETİM'den geçti, bulguları işlendi (2026-10-27); kararlar Gemini'nin 1–7 araştırmasından geçti (DENETİM doğruladı, `84ee01b8`); 4.5'in isteğe bağlı karşılaştırması Gemini Koşu 3'ten (`0edb6f20`). Sonraki kararlar (4.3, 4.12, 4.10) Gemini'nin araştırmasını bekliyor. 4.1–4.4, 4.6, 4.9, 4.11 ve 4.12'nin çalışma anı uygulandı (katman 2, DENETİM'den geçti, 2026-10-28); aşağıda yalnız kalanlar. Tip isteyen denetim hataları katman 4'tedir.

### 4.5

- **Mevcut durum (D1, 2026-11-09):** eşitlenemeyen değerin (fonksiyon, tensör, arayüz ve onları iç içe tutan liste, sözlük, demet, yapı, durumlu tip; `T?` `T`'nin kuralıyla) `==`/`!=`'si denetim hatası (tensör H1148, isteğe bağlı tensör H1151, fonksiyon H1152, arayüz H1153), `==` isteyen değer deseni H1150, `.TR_contains`/`.TR_index_of`/`.TR_unique` H1149 denetim hatası; `TR_none` değişmeziyle karşılaştırma ve durum deseni geçer. Tipi bilinmeyen değerde çalışma anı aynı kuralı izler: fonksiyon H2613 ve H2706, tensör H2777, yapı alanı H2770; aranan değer boş listede de [test: `TestUnequatableComparisonIsACheckError`, `tests/negative/esitlenemeyen.tr`, `TestTensorInsideAValueIsAnErrorExceptInAssertEqual`, `tests/operatorler_test.tr`'nin 4.5 testleri].
- **Mevcut durum (4a4, 2026-11-03):** tipleri bilinen ayrı iki değerin `==`/`!=`'si denetim hatası (H1141: `"1" == 1`, `[1] == [1.0]`, ayrı iki yapı, uzunluğu ayrı demetler); sayılar (`1 == 1.0`), isteğe bağlı ile asıl tipi ve `TR_none`, JSON (H1131, H1132), fonksiyon, tip değeri, tensör, tipi bilinmeyen değer ve öğe tipsiz kap bu kurala girmiyor. Tipi bilinmeyen yerde çalışma anı ayrı tipleri hâlâ sessizce eşit değil sayıyor [test: `TestDistinctTypesAreNotCompared`, `tests/negative/ayri_tip_esitligi.tr`].
- **Mevcut durum (D1, 2026-11-09):** `==`'nin iç içelik sınırı yok; kendine bağlanan liste, sözlük ve nesne açılmış ağaçlarıyla karşılaştırılıyor, aynı değer de öğe öğe (`TR_nan` tutan kendine bağlı değer kendisine eşit değil); 10 000 kap çiftinden sonra açık yığın ve birleşim-bul (`internal/lang/interp/value_equality.go`, `equality.go`); hata ve sonuç aynı biçimdeki çiftlerde tek gezintide. Karşılaştırılamayan öğe (fonksiyon, hata değeri, tensör; yapı alanında H2770) değerin neresinde olursa olsun, karşısındaki öğeye bakılmadan hata (`[f] == [TR_none]` H2706); H2771 emekli. Tipi bilinen `TR_List[F?]` denetimde H1152 (15.5 dilimi); tipi bilinmeyen kodda motor öğeye bakar, yalnız `TR_none` taşıyan liste hatasız. Kalan sapma: JSON ile JSON olmayan değerin H2705'i çift çifte aranıyor, 10 000 kap çiftini aşan paylaşılan ya da kendine bağlanan genel tipli değerde kaçabilir [test: `tests/esitlik_test.tr`, `TestEqualityHasNoDepthLimit`, `TestUncomparableMemberIsFoundBeforeTheFirstDifference`, `TestUnionFindHasNoIdentityShortcut`, `TestSelfContainingContainers`].
- **Yapılacaklar (D, Ö3; sahip D1; D1 devri Bulgu 1, DENETİM 339 Ş1):** JSON ile JSON olmayan değerin H2705'i kabın içinde çift çifte aranıyor; genel yolda birleşim-bul `x ~ z ~ y` zincirinde `(x, y)`'yi gezmez, `z`'nin aynı yerinde `TR_none` varsa o çiftin hatası kaçar ve sonuç `TR_false` olur [hesap: D1; kanıtlı: `internal/lang/interp/equality.go:288`–`:292`]. Yalnız 10 000 kap çiftini aşan, paylaşılan ya da kendine bağlanan, genel tipli değerde; tipli kodda kabın içinde JSON ile JSON olmayan değer aynı yerde durmaz. Sıradaki adım: genel yolda çift kümesiyle ayrı tarama ya da tip düzeyinde denetim, 15.5 diliminden sonra.
- **Mevcut durum (C, Ö2 kapandı; birleşik 63, 2026-11-09):** aynı alt nesneyi her katta iki dalında tutan değer ile ayrı kurulmuş eşi artık üstel değil: 20 kat 382,7 ms → 1,1 ms, 30 kat tabanda 60 sn'de bitmedi, yeni 0,57 ms [ölçüldü: D1, `03f71259`, Windows, birer koşu]; TCB'nin bağımsız programında 30 kat 2 ms, `bb59634f`'nin programı 20 sn'de çıktı vermedi [ölçüldü: TCB, dizüstü, geçici sınama programı]. Birleşim-bul bitmiş çiftleri birleştiriyor [test: `tests/esitlik_test.tr` "4.5 aynı alt nesneyi birçok yerde tutan değer"].
- **Mevcut durum (hız, birleşik 63):** Colab CPU High-RAM, AMD EPYC 7B12, 3 tur ortanca, aynı koşuda taban `6311e11a` ile: küçük liste (10 `TR_Int`) 430 → 240 ns (−%44), 5 000 düğümlük yapı zinciri 2,70 → 1,27 ms (−%53), 3 alanlı yapı 498 → 525 ns (+%5) [ölçüldü: D1, Colab günlüğü `cpu_d1-esitlik_hiz2`]. +%5'in kaynağı nesne çiftinde kapa bağlı hata için alanların ayrıca okunması [hipotez: D1, profil alınmadı]. TCB kararı: kabul; sınırsız ve döngülü eşitliğin doğruluğu ve öbür iki şekildeki kazanç bunun önünde, yapı yolu sıcak yol profilinde görünürse yeniden bakılır.

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

## 5. Deyimler ve kontrol yapıları

Kurallar: [`docs/spec/05_deyimler.md`](spec/05_deyimler.md).

`TR_elif` bilerek kimliği olmayan addır; kimlik kapısı düzelince bilerek yanlış örnekler listesine girer (2. bölümün girişi). Yeni denetim hataları (5.1'in uyarısı, 5.5, 5.6, 5.12, 5.14) uygulanırken depodaki kodlu dosyalar da düzeltilir; yoksa `TestRepoTRFilesPassCheck` kapısı kalır.

### 5.1

- **Mevcut durum:** fonksiyonun içinde hiç yeniden atanmayan yerel `TR_var` uyarı vermiyor [ölçüldü: yerelinde `TR_var a = 3` olan fonksiyon; denetim ve çalıştırma].
- **Yapılacaklar:** fonksiyonun içinde yeniden atanmayan yerel `TR_var` uyarısı, ileti `TR_const`'u önerir (programın en dışı girmez); TR testi.

### 5.3

- **Mevcut durum:** fonksiyon değerinin çağrısında, isteğe bağlı ve tipi bilinmeyen demette öğe sayısının tutmaması çalışırken bulunuyor (H2709); kural demette denetim hatası diyor [test: `TestUnpackCountOfAKnownTuple`].

### 5.4

- **Yapılacaklar:** `/=` iletisi `//=`'yi önerir.

### 5.5

- **Mevcut durum:** `TR_elif` iletisi `TR_else TR_if`'i öneriyor ama ardından ikinci, yanıltıcı bir sözdizimi hatası ("':' bekleniyordu") geliyor; dalların tipi denetlenmiyor: `TR_if n < 0 { "a" } TR_else { 5 }` denetimden geçiyor [ölçüldü: main `85a19c36`, dört küçük program; denetim ve çalıştırma].
- **Yapılacaklar (A, Ö1, 4a5; ölçüldü 30b44b01):** ifade olarak kullanılınca dalların tipi aynı değilse denetim hatası; `TR_elif`'e tek ileti (ayrıştırıcının toparlanması); denetleyicinin testi.

### 5.6

- **Mevcut durum:** kapsama denetimi yok: `_`'siz eşleme denetimden geçiyor, eşleşmeyince çalışırken H2702 veriyor. Düz adla yazılan değişken deseni hata vermiyor, değeri karşılaştırıyor. Kolların tipi denetlenmiyor (bir kol metin, öbürü sayı veriyor). Tensör içeren desen denetim hatası vermiyor, bütün tensörü karşılaştırıyor [ölçüldü: altı küçük eşleme programı; denetim ve çalıştırma].
- **Yapılacaklar (A, Ö1, 4b; ölçüldü 30b44b01):** kapsama denetimi (`TR_Bool`'da iki değer, öbür tiplerde `_`); düz adla yazılan değişken deseni denetim hatası, ileti `(x)`'i ya da `_`'yi önerir; ifade olarak kullanılınca kolların tipi aynı; tensör içeren desenin denetim hatası (15.5 ile); denetleyicinin testi.

### 5.7

- **Mevcut durum:** yapı dolaşılamıyor: `TR_next() -> TR_Int?` metodu olan yapıda `TR_for` "döngü kurulamaz" çalışma hatası veriyor [ölçüldü: main `85a19c36`]. Akışlı okuyucunun iki kez dolaşılması yeniden ölçülmedi.
- **Yapılacaklar (E, Ö3; sahip D1; DENETİM istek 163):** `TR_next() -> T?` taşıyan yapının dolaşılması: denetimden geçiyor, çalışırken açık hata [ölçüldü: DENETİM 163], tek seferlik dolaşıcı, `T` isteğe bağlıysa denetim hatası (ileti öğeyi sarmayı önerir); TR testi.

### 5.11

- **Mevcut durum:** sözlük üreteci yok: `{k: k * 2 TR_for k TR_in 0..3}` sözdizimi hatası veriyor ("burada bir değer ya da ifade bekleniyordu") [ölçüldü: iki sözlük üreteci; denetim ve çalıştırma].
- **Yapılacaklar:** sözlük üreteci, yinelenen anahtarda son değer; TR testi.
- **Yapılacaklar (dil tanımı turu 2, 2026-11-01):** üreteçte iki değişkenli biçim (`{x: i TR_for i, x TR_in l}`); `fd6a84d3`'de sözdizimi hatası [ölçüldü: tur 2'nin yazımı, `[i TR_for i, x TR_in l]`].

### 5.12

- **Mevcut durum:** yasak yeniden tanım hata vermiyor: iç bloktaki `TR_var t = 5` dıştaki `t`'yi gizliyor, dıştaki 0 kalıyor; aynı kapsamda ikinci `TR_var b` ve parametreyle aynı adlı gövde değişkeni de geçiyor [ölçüldü: iç blokta, aynı kapsamda ve parametreyle yeniden tanım; denetim ve çalıştırma].
- **Yapılacaklar (A, Ö1, 4b; ölçüldü 30b44b01):** kapsayan ya da aynı kapsamdaki yerel adla yeniden tanım denetim hatası (programın en dışı ve parametreler dahil, sondaki blok sıradan blok); denetleyicinin testi.

### 5.14

- **Mevcut durum:** tanımından önce kullanılan değişken, fonksiyon ve yapı denetimden geçiyor, çalışırken "böyle bir ad yok" hatası veriyor; sonra tanımlanan değişkeni okuyan fonksiyon, değişkenden önce çağrılınca da. Modülün en dışı sıraya bağlı: önündeki sabiti okuyan sabit modül çalışırken düşüyor; birbirine döngüyle bağlı iki sabit denetimden geçiyor [ölçüldü: öne alınmayan değişken, fonksiyon ve yapı, iki modül dosyası; denetim ve çalıştırma]. Takma adsız aktarmadaki tanımsız ad 1.3'te.
- **Yapılacaklar:** tanımından önce kullanma denetim hatası; fonksiyon ve tip tanımları bloğun her yerinden görünür; fonksiyonu çağıran ya da değer olarak alan ifade, fonksiyonun doğrudan ya da dolaylı kullandığı değişkenlerin tanımından sonra gelir (ileti tanımı yukarı almayı önerir); modülün en dışı sıradan bağımsız, sabitler arasında döngü denetim hatası; denetleyicinin testi.

## 6. Fonksiyonlar

Kurallar: [`docs/spec/06_fonksiyonlar.md`](spec/06_fonksiyonlar.md).

Eski yazımlar (`TR_it`, `TR_acc`, adsız `TR_func`, parantezsiz ok) depoda yalnız olumsuz fikstürde kalıyor. Eski kurala dayanan testler: eksik, fazla, iki kez verilen argümanı ve varsayılandaki ileri başvuruyu çalışırken yakalayan 6.4 ve 6.5 testleri (`tests/fonksiyonlar_test.tr`; denetim hatası olunca `tests/negative/` fikstürlerine taşınır).

### 6.1

- **Yapılacaklar (D, Ö3; sahip D1, Kol A; TCB 2026-11-09, tur 4):** `TR_func` adına atama denetimden geçiyor: `f = g`, `f = (x) => ...`, `f, x = g, 1` `f`'yi değiştiriyor, `f += 1` çalışırken düşüyor [ölçüldü: DENETİM 167 `s1`–`s5`, `e65e6850`]. Aynı fonksiyonda iç blokta `TR_var f = g` ve ters yönü (`TR_var f = 1`, içeride `TR_func f()`) geçiyor (`s6`, `s7`); 5.12'nin genel uygulaması ayrı satırda.
- **Yapılacaklar (A, Ö1, 4a5; ölçüldü 30b44b01 ve 16c7d0ef):** kitaplığın 29 istisnası 4c'de (tip parametresi ve birleşim tipi) kalkar; argümanın parametre tipine göre denetimi metotta, kurucuda ve adlı argümanda.

### 6.2

- **Mevcut durum:** eksik dönüş denetlenmiyor: dönüş tipi `TR_Int` olan fonksiyonda `TR_return`'süz yol denetimden geçiyor, çalışınca yokluk veriyor; fonksiyon tipi beklenen fonksiyon ifadesinin blok gövdesinde de [ölçüldü: dalı biten ve bitmeyen fonksiyonlar, `TR_throw`, sonsuz döngü, `TR_match` ve `TR_exit` ile biten yollar; denetim ve çalıştırma].
- **Yapılacaklar (A, Ö1, 4b; ölçüldü 30b44b01):** bitmeyen yol denetim hatası, ileti `TR_return`'ü önerir; bitiren deyimler: `TR_return`, `TR_throw`, `TR_exit`, çıkışı olmayan sonsuz döngü, bütün dalları biten `TR_if` ve bütün kolları biten `TR_match`; denetleyicinin testi.

### 6.3

- **Mevcut durum:** açılan adlar tipsiz kalıyor: `TR_var a, b = f()` sonrası `b`'nin tipi denetlenmiyor [ölçüldü: main `85a19c36`, küçük programlar].
- **Yapılacaklar (A, Ö1, 4a5; ölçüldü 30b44b01 ve 16c7d0ef):** dönüş tipinin öğe tipleriyle açılan adların tipi.

### 6.4

- **Mevcut durum:** varsayılanda parametrenin kendisi ya da sonraki parametre ancak çalışırken bulunuyor (H2440) [ölçüldü: ileri başvuru; denetim ve çalıştırma].
- **Yapılacaklar:** varsayılanda parametrenin kendisi ve sonraki parametreler denetim hatası; denetleyicinin testi.
- **Yapılacaklar (A, Ö1, 4a5; ölçüldü 30b44b01 ve 16c7d0ef):** varsayılanın tipi parametrenin tipine uymazsa denetim hatası: bugün `TR_func f(x: TR_Int = "a") -> TR_Int`'te `f()` `"a"` (`TR_String`) veriyor, denetim ve çalışma hatasız [ölçüldü: DENETİM istek 68].

### 6.5

- **Mevcut durum:** eksik, fazla, bilinmeyen adlı ve hem sırayla hem adla verilen argüman ancak çalışırken bulunuyor (H2414, H2412, H2415, H2413); adlı argümanın tipi hiç denetlenmiyor (`b: "x"` hata ancak gövdedeki işlemde veriyor); parametre bekleyen yerleşiğe verilen sondaki blok çalışırken H2711 [ölçüldü: iki parametreli fonksiyona sekiz çağrı; denetim ve çalıştırma].
- **Yapılacaklar (A, Ö1, 4a5; ölçüldü 30b44b01 ve 16c7d0ef):** eksik, fazla, bilinmeyen ve iki kez verilen argüman ile tipi tutmayan adlı argüman denetim hatası, ileti parametrenin adını söyler; denetleyicinin testi.

### 6.6

- **Mevcut durum:** adı üzerinden gelen yokluk görülmüyor: `TR_var n = TR_none`, `(x: TR_Bool) => { TR_return n }` geçiyor [ölçüldü: Kol A teslim 2b].
- **Yapılacaklar (E ve D, Ö3; sahip D1, 4a5 ile; TCB 2026-11-09, tur 4):** 6.6 artık tip tabanlı: bir yol `T`, öbürü `TR_none` döndürüyorsa dönüş `T?` (bugün H1145, DENETİM 167 `r1`); yalnız `TR_none` dönen gövde hata. `TR_return n` kaçağı (yukarıda) 3.13'ün işidir: tipsiz `TR_var n = TR_none` 3.5'e göre hata. Ortak tipin birleştirmesi (`T` ile `TR_NoneType` → `T?`) 4a5'e bağlı.
- **Mevcut durum:** denetleyici yerleşik imzalarını bilmediği için fonksiyon ifadesinin parametre tipi beklenen tipten çıkarılmıyor (`TR_Any` kalıyor) [ölçüldü: `TestUntypedFunctionExpressionParameter`].
- **Yapılacaklar (A, Ö1, 4a5; ölçüldü 30b44b01 ve 16c7d0ef):** parametre tipinin beklenen tipten çıkarımı ve beklenen tipin denetimi (yerleşik ve kitaplık imzalarıyla, 13.9); denetleyicinin testi.


### 6.8

- **Mevcut durum:** dıştaki değişkene okumadan önce atayan fonksiyon uyarı vermiyor [ölçüldü: dıştaki sayaca okumadan atayan fonksiyon; denetim ve çalıştırma].
- **Yapılacaklar:** dıştaki değişkene bu fonksiyonda okumadan önce atama uyarısı, ileti `TR_var`'ı önerir; TR testi.

### 6.9

- **Mevcut durum:** fonksiyon tipinin yazımı ayrışıyor (`(TR_Int) -> TR_Bool`, sağdan birleşen `->`, `() -> (TR_Int, TR_Int)`), ama denetlenmiyor: `(TR_Int) -> TR_Bool` tipli değişkene sayı döndüren ok atanabiliyor; fonksiyon değeri adlı argümanla ya da eksik argümanla çağrılınca H2710 (bugünkü 6.9 testi); metin tipin metni değil: adlı fonksiyon "<function f>", ok "<lambda>"; `TR_Type(f)` genel tip "Function" veriyor (3.11) [ölçüldü: main 1073aeab]. Fonksiyonların karşılaştırması 4.5'te.
- **Yapılacaklar (A, Ö1, 4a5 ya da 4c; ölçüldü 30b44b01):** fonksiyonun metni tipinin metni ("(Int) -> Bool"), `TR_Type(f)` fonksiyon tipi; parametre ve dönüş tiplerinin denetimi (4a); kimlik listesindeki genel fonksiyon tipi kalkar (3.11); TR testi.

### 6.10

- **Mevcut durum:** tip parametresi yok: `TR_func baştaki[T](l: TR_List[T]) -> T?` sözdizimi hatası ("'(' bekleniyordu"); çağrıda tip yazımı (`f[TR_Int]([3])`) denetimden geçiyor, çalışırken "Fonksiyon tipi indekslenemez" veriyor [ölçüldü: main 1073aeab].
- **Yapılacaklar:** tip parametreleri ve çağrıda çıkarımı, tip yazımının listenin öğesinin çağrısından ayrılması; kısıtsız `T`'de yalnız her tipte geçerli işler, öbürleri kısıt ister (13.10, 7.8); TR testi.
- **Mevcut durum (Faz 0.6a):** tip yerinde `Kutu[TR_Int]` ve `T` ayrıştırılıyor, `Kutu`'ya indirgeniyor; tanımdaki tip parametresi (`TR_func f[T](...)`) hâlâ sözdizimi hatası (4c).

## 7. Yapılar

Kurallar: [`docs/spec/07_yapilar.md`](spec/07_yapilar.md). `tests/yapilar_test.tr`'nin verilmeyen alanı, olmayan alanı çalışırken yakalayan 7.2, 7.4 ve 7.5 testleri eski kurala dayanıyor (denetim hatası olunca `tests/negative/` fikstürlerine taşınır), 7.6 testi StructType'ı sınıyor.

Yapının tipi olan genel tip (StructType) kalkar; yapının tipi `TR_Type`'tır (3.11). Örneklerdeki eski genel tipler de kalkar: StructType, Function, BuiltinFunction, Object ve `TR_Any` (`examples/tr/12_tip_adlari.tr`, `14_yapilar_ve_metotlar.tr`) [ölçüldü: örneklerde git grep].

Alan daraltılmadığı için (3.5) `lib/`'deki 9 `TR_self.alan != TR_none` sınaması [ölçüldü: `lib/` için git grep] yerel kopyaya ya da `?.`'ya çevrilir; "yoksa varsayılan ata" kalıbının 4 yeri 3.5'in atamayla daraltmasıyla geçerli kalır.

### 7.1

- **Mevcut durum:** yapının tip parametresi yok: `TR_struct Kutu[T]` sözdizimi hatası. Kitaplığın 4c'yi bekleyen iki alanı (GPT'nin karışık değerli ayar sözlüğü, Muon ya da NorMuon tutan alan) 6.1'in istisnası.
- **Yapılacaklar:** kitaplığın iki alanının istisnası 4c'de kalkar; yapının tip parametresi (6.10 ile); TR testi.

### 7.2

- **Mevcut durum:** varsayılanı olmayan alan verilmezse, fazla ya da bilinmeyen argümanda ve iki kez verilen alanda ancak çalışırken hata (H2750, H2419, H2418, H2420); çalışma anı alanın tipini denetlemiyor [ölçüldü: `TestConstructionFieldTypes`].
- **Yapılacaklar (A, Ö1, 4a5; ölçüldü 30b44b01 ve 16c7d0ef):** zorunlu alanın verilmemesi ve 6.5'in argüman hataları denetim hatası; kurucusu olan yapıda kurucunun parametre tipleri (7.4); denetleyicinin testi.

### 7.3

- **Mevcut durum:** Python alışkanlığıyla ilk parametreye düz self sözcüğü yazılınca tipsiz parametre hatası (H1111) çıkıyor; önerisi Python'un self'ine `TR_Int` tipi yazmak; bu, `TR_self` yerine self'i tiplemeyi öğretiyor ve kopyalanınca çağrı eksik argüman hatası veriyor (F1) [ölçüldü: DENETİM istek 68, `30b44b01`].
- **Yapılacaklar (D ve F1, Ö3; ölçüldü 30b44b01):** metodun ilk parametresi Python'un self'iyse sözdizimi hatası, ileti `TR_self`'i önerir; TR testi.

### 7.4

- **Mevcut durum:** kurucunun imzasına uymayan kurma ancak çalışırken bulunuyor (H2410, H2414); kurucunun atamadığı varsayılansız alan yokluk kalıyor, atanmadan okunan alan ve bütün alanlar atanmadan metot çağrısı hata vermiyor [ölçüldü: varsayılansız alanı atamayan, onu okuyan ve metot çağıran kurucu; denetim ve çalıştırma].
- **Yapılacaklar (A, Ö1, 4b; ölçüldü 30b44b01):** kurucunun sonunda her yolda varsayılansız her alanın atanması, atanmadan okuma ve önceden metot çağrısı denetim hatası; kurucunun imzasıyla kurma 6.5'in denetim hatalarıyla; denetleyicinin testi.

### 7.5

- **Mevcut durum:** olmayan alan ve metoda atama ancak çalışırken bulunuyor (H2468, H2405, H2404); ileti en yakın adı öneriyor ama alanla eşit uzaklıktaki metodu da seçebiliyor (`p.y` için `m`) [ölçüldü: `TestFieldAssignType`, küçük programlar].
- **Yapılacaklar (D ve F1, Ö3; ölçüldü 30b44b01):** olmayan alan ve metoda atama denetim hatası, ileti en yakın alanı önerir; denetleyicinin testi.

### 7.6

- **Mevcut durum:** yapının tipi StructType [ölçüldü: denetim ve çalıştırma].
- **Yapılacaklar (A, Ö1, 4a5 ya da 4c; ölçüldü 30b44b01):** yapının tipi `TR_Type` (3.11); denetleyicinin testi.
- **Yapılacaklar (sınıfsız: test kapsamı, Ö3; sahip TCB, 7.6'nın kimlik işlemiyle; D1 15.5 devri, birleşik 65):** `lib/optimizers_test.tr`'nin 14 yeri parametrenin grupta olduğunu kimlikle değil değerle sınıyor (değerle arayan bir yardımcı fonksiyon ve her parametreye ayrı sabit ekleyen bir hazırlık); aynı değerde iki parametre testi düşürür, sessiz geçirmez; hazırlık her parametreye ayrı sabit eklediği için ikiz kalmaz. Sıradaki adım: 7.6'nın kimlik işlemi gelince ona geçer.

### 7.7

- **Mevcut durum:** tip parametresi yok (`TR_enum Cevap[T]` sözdizimi hatası); kapsama denetimi yok (`_`'siz, eksik durumlu eşleme çalışırken H2702); dıştaki adla aynı bağlanan ad hata vermiyor, dıştakini gizliyor (5.12); alanın tipi denetlenmiyor (`TR_Int` alana metin) [ölçüldü: dört durumlu tip programı; denetim ve çalıştırma].
- **Yapılacaklar (A, Ö1, 4a5 ya da 4c; ölçüldü 30b44b01):** tip parametresi (6.10 ile); kapsama denetimi (5.6); bağlanan adın 5.12 hatası; alan tipinin denetimi; TR testi.

### 7.8

- **Mevcut durum:** arayüzün denetimi yok: metodu eksik değer ancak çağrıda "alanı ya da metodu yok" çalışma hatası veriyor, imzası tutmayan değer (`TR_String` parametreli metot) hatasız geçiyor; arayüz değerinin `==`'si hata vermiyor; arayüz kısıtlı tip parametresi (`TR_func g[T: Katman]`) sözdizimi hatası [ölçüldü: bir arayüz, üç yapı ve bir durumlu tiple; denetim ve çalıştırma].
- **Yapılacaklar (A, Ö1, 4a5, tip parametresinin kısıtı 4c ile; ölçüldü 30b44b01 ve 16c7d0ef):** karşılamayan değer denetim hatası, ileti eksik ya da imzası tutmayan metotları söyler; tip parametresinde kısıt (6.10 ile); daraltmanın denetleyicide 3.5'in koşullarıyla yapılması; denetleyicinin testi.

## 8. Hata yönetimi

Kurallar: [`docs/spec/08_hata_yonetimi.md`](spec/08_hata_yonetimi.md). Taslak (2026-10-28): ana oturum yazdı, DENETİM'den geçti (1–7 ile tutarlılık; bulguları işlendi; Ç1: yakalamanın adı zorunlu, 2.9b'den yakalamanın hazır adı kalktı, Ahmet 2026-10-28); kararlar Gemini Koşu 7'den geçti (DENETİM doğruladı, `59efe468`, 2026-10-29). Yeni adların gösterim adları onaylandı (Ahmet, 2026-11-01): `TR_Error` Hata, `TR_code` kod, `TR_value` fırlatılan (üye ad alanında "değer" dolu), `TR_cause` neden (üye ve adlı argüman), `TR_suppressed` bastırılanlar; kodları uygulandıkları gün kayda girer (17.4). `TR_throw` ve `TR_catch`'in katalogdaki açıklamaları ve `tests/` altındaki 8. bölüm testleri `TR_Error`'a göre yeniden yazıldı (D1 ikinci dilim).

### 8.1

- **Bulgu (DENETİM istek 103 N1, D, Ö3; sahip D1, 2b'den sonra):** sondaki bloğun son ifadesi `TR_try` ise değer sayılmıyor: `TR_var y = TR_no_grad { TR_try { 1 } TR_catch e { 2 } }` 2.8 hatası veriyor, `=>` gövdesinde aynı yazım çalışıyor [ölçüldü: DENETİM]; 6.7 ve 8.1 ikisini de değer sayar. Sesli hata.
- **Mevcut durum:** `TR_try`/`TR_catch` çalışıyor, `TR_catch` zorunlu. `TR_try` ifade olarak da çalışıyor, değeri çalışan bloğun son ifadesi (`TR_var n = TR_try { TR_Int(m) } TR_catch e { 0 }`); eskiden sözdizimi hatasıydı. Python'un `except`'i H2915 (`TR_catch`'i öneriyor), `finally`'si yakalamadan sonra da yerine de H2916 (`TR_defer`'i öneriyor), parantezli ad `TR_catch (e)` H2917 (`TR_catch e`'yi öneriyor; eskiden kabul ediliyordu, depodaki tek kullanım göçtü) [ölçüldü: dal `claude/d1-dilim2-c89bbc`, `tests/hatalar_ertele_test.tr`'nin 8.1 ifade testi, `TestOtherLanguagesCatchSpellingsAreSyntaxErrors`, öneri tablosu].
- **Mevcut durum (ad):** adsız `TR_catch { ... }` sözdizimi hatası H2910, ileti `TR_catch e`'yi öneriyor; yakalamanın hazır adı (bağlamsal kimlik) kalktı. Göç: `.tr` dosyalarında 6 adsız yakalama ve hazır adın 4 kullanımı, Go testlerindeki TR programlarında 5 adsız yakalama ve hazır adın 2 kullanımı adlı yazıma geçti [ölçüldü: dal `claude/d1-dilim2-c89bbc`, `git grep "TR_catch *{"`; `TestCatchWithoutANameIsASyntaxError`, öneri tablosu].
- **Yapılacaklar (A, Ö1, 4a5; ölçüldü 30b44b01 ve 16c7d0ef):** ifade olarak `TR_try`'da dalların tipinin aynı olması ve blokların içindeki adların denetimi (denetleyici; bugün ifadenin bloklarına inmiyor: içindeki tanımsız ad ve farklı dal tipleri denetimden geçiyor [ölçüldü: tr check, dal `claude/d1-dilim2-c89bbc`]); sözdizimi hatalarının olumsuz TR fikstürleri (adsız, parantezli ad, `except`, `finally`, `raise`).

### 8.2

- **Mevcut durum:** fırlatılabilen dört tür: metin, ileti, durumlu tip değeri, `TR_Error`; başka tip çalışma hatası H2911 (`TR_throw` ve `TR_Error(...)`), ileti programın kendi adıyla `TR_throw TR_String(sayı)`'yı öneriyor (değer ad değilse yalnız `TR_String`'i anıyor; eskiden sabit `TR_String(x)`'ti) [test: `TestNotThrowableSuggestsTheProgramsSpelling`]. `TR_throw e` yakalanan hatayı olduğu gibi (kodu, değeri, nedeni, bastırılanları ve ilk fırlatıldığı yer) yeniden fırlatıyor; `TR_Error(v, TR_cause: e)` yeni hata kuruyor, nedeni hata değilse H2912. Python'un `raise x`'i sözdizimi hatası H2914, ileti `TR_throw`'u öneriyor [ölçüldü: dal `claude/d1-dilim2-c89bbc`, `tests/hatalar_ertele_test.tr`'nin 8.2 testleri, `TestPythonRaiseIsASyntaxError`, öneri tablosu].
- **Yapılacaklar:** yakalanmayan hatada nedenin ve çağrı yığınının gösterilmesi (8.6).
- **Gemini Koşu 7 (2026-10-29), uygulanacak:** yeniden fırlatmada çağrı yığını korunur.

### 8.3

- **Mevcut durum:** yakalanan hata bir `TR_Error` ("TR_Type(e)" "Error"); üyeleri `e.TR_code` (iletinin kodu ya da `TR_none`), `e.TR_value` (fırlatılan değer; çalışma hatasında asıl nedenin iletisi, `TR_error_code`'un verdiği kodla), `e.TR_cause`, `e.TR_suppressed`; metni fırlatılan değerin metni ("Message("H2469", "List", "Int", "-")"), okuyana göre değişmiyor. `e == f` çalışma hatası H2913, ileti programın kendi adlarıyla `e.TR_code == f.TR_code`'u öneriyor (iki yan da ad değilse yalnız üyeyi anıyor; eskiden sabit `e`, `f`'ti; öneri tablosu iki ad çiftiyle); yoklukla karşılaştırma serbest. `TR_assert`'in başarısızlığı kodlarıyla yakalanıyor. Sarmalayan iletinin bağlamı (dosyanın yolu, önerilen kontrol noktası) yalnız gösterilen hatada [ölçüldü: dal `claude/d1-dilim2-c89bbc`, `tests/hatalar_ertele_test.tr`'nin 8.3 testi, `TestCaughtRunTimeErrorTextIsItsMessage`, `TestCaughtErrorTypesAreErrorValues`]. Göç: yakalanan hatayı metin gibi kullanan 35 `.tr` ve 8 Go satırı önce `TR_String(h)`'ye geçti (çıktılar bayt bayt aynı); kuraldan sonra sessiz değişen 2 örnek satırı ve sesli düşen testler yeni kurala yazıldı (liste D1 devir notunda).
- **Yapılacaklar (A, Ö1, 4a5; DENETİM 231: `x: TR_Int` içinde metin [ölçüldü: DENETİM 231]; `TR_catch e`'nin tipiyle kapanmaz):** `TR_value`'nun kapalı birleşim tipi ve `e == f`'nin denetim hatası (denetleyici, 4a); kopyanın daraltılması (4b), `_`'siz eşlemenin kapsaması (4c).
- **Gemini Koşu 7 (2026-10-29), uygulanacak:** kapalı birleşimin yerel kopyası; hatayla biten kol eşlemenin tipine katılmaz (denetimi 4a2).

### 8.4

- **Mevcut durum:** denetim karakterleri 2.6'nın kaçışıyla yazılıyor: `TR_throw "a\u{1b}[2Jb\u{7}c"` → `a\u{1b}[2Jb\u{7}c`; eskiden `\x1b` [ölçüldü: dal `claude/d1-dilim1-bf7baf`, CLI; `TestPrintable`]. TR testi yakalanan metnin değişmediğini sınıyor; terminal çıktısı Go testinde. İletideki `{ad|q}` alıntısı da 2.6'nın kaçışıyla yazıyor (`"a\u{1b}[2Jb"`, yön karakteri `\u{202e}`); eskiden Go'nun `\x1b`'siydi [ölçüldü: dal `claude/d1-dilim2-c89bbc`, `TestRender`].

### 8.5

- **Mevcut durum:** `TR_Message` ve `TR_error_code` çalışıyor (`H6903`); kodsuz hatada (düz metin, sayı, sözlük, durumlu tip değeri) `TR_error_code` koda dönmüyor, hatayı olduğu gibi yeniden fırlatıyor; eskiden hepsi H2441 veriyordu [ölçüldü: dal `claude/d1-dilim1-bf7baf`, `TestErrorCodeRethrowsACodelessError`, `tests/hatalar_ertele_test.tr`'nin 8.5 testi]. Yakalanmayan düz metin hata içte H2441 taşımayı sürdürüyor (`examples/tr/hatalar/12_yakalanmamis_firlat.tr` onu sınıyor).
- **Mevcut durum (zincir):** `TR_error_code` hatanın kodunu, o yoksa nedenler zincirinde (`TR_cause`) dıştan içe ilk kodu veriyor; zincirde kod yoksa hata, nedeniyle birlikte yeniden fırlıyor; durdurma ve `TR_exit` geçiyor [ölçüldü: dal `claude/d1-dilim2-c89bbc`, `TestErrorCodeFollowsTheCauseChain`, `tests/hatalar_ertele_test.tr`'nin 8.5 zincir testi: üç halka, kodlu dış, kodsuz zincir].
- **Yapılacaklar:** programın ayırması beklenen hataların kodları kendi maddelerine yazılacak (Gemini Koşu 4, 6, 7).
- **Mevcut durum (sistemin metni):** dosya hatalarının nedeni (`turkish.FileError`, `fileErrorReason`) sistemin ya da Go'nun metnini taşımıyor: bilinen sistem numarası kendi koduna iniyor, bilinmeyen numarada yalnız numara (H2628, ör. Windows'ta 1168), öbür bilinmeyen hatada metinsiz H2629; H2135 ve H2136 emekli; özgün hata `Unwrap` ile korunuyor [test: TestFileErrorReasonCarriesNoSystemText]. Bilinmeyen numara işletim sistemine göre değişir.
- **Yapılacaklar (A, Ö1; DENETİM istek 79 Ş3; ölçüldü: D1 devri, alt ajanın 181 yerlik okuması, D1 altısını, DENETİM dördünü açtı):** programın değerine sistemin metnini koyan 41 yol: `{error}`'a sistem hatasını doğrudan veren 34 yer (compute/dataload 22, compute/gpu 5, devtools/profiler 2, stdlib/ml 2, stdlib kontrol noktası paketi 1, compute/tokenize 1; lang/turkish'inki Kol A'da düzeldi) ve onu kodlu bir iletinin içinde taşıyan 7 yer; çoğu `turkish.FileError`'a çevrilebilir. Eksik dosya bu yollara varmadan `FileError`'dan geçiyor (H2132); ham yollar açılış ya da okumadaki seyrek hatada [hipotez: DENETİM istek 79].
- **Yapılacaklar (F, Ö3; DENETİM istek 79 Ş3):** cmd/trlang'ın 20 yolu programın değeri değil, komut satırının gösterdiği hata; gösterim makineye göre değişiyor. `trlang` göçünden sonra. H5620'nin `{detail}`'i de aynı yolda olabilir [hipotez: alt ajanın raporu]. Satır listesi D1'in Kol A devrinde.
- **Yapılacaklar (F1, Ö3; DENETİM istek 79):** H2629 hiçbir şey söylemiyor ("beklenmeyen bir hata oluştu"); Go'nun bilinen hataları (beklenmeyen dosya sonu gibi) kendi kodlarına inerse ileti yanlışı söyler.

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

Kurallar: [`docs/spec/10_eszamanlilik.md`](spec/10_eszamanlilik.md).

### 10.1

- **Mevcut durum:** dönüşü bilinmeyen çağrının görevi parametresiz `TR_Task` (denetleyicinin iç tipi): `.TR_wait()`'in tipi bilinmiyor, tipli yere verilebiliyor.
- **Yapılacaklar (A, Ö1, 4a2 ve 6.6; pending 3.11; DENETİM istek 81):** dönüş tipi çıkarılmamış fonksiyon ifadesiyle başlatılan görevin tipi parametresiz kalıyor; `TR_Task[TR_Int]`'e atanıp metin taşıyor [ölçüldü: önem taraması teslim 3, `30b44b01` ve main `51b75f0a`].

### 10.2

- **Mevcut durum:** `TR_json_string` liste ve sözlük girdisinde kilidi tutuyor (girdi değişebilir; dil tanımı düzeltilecek, `pending.md`). S-kilidin ölçütü CPU'da Linux L4'te geçti [ölçüldü: TCB, L4, `3a5114d2`; DENETİM istek 69]; Windows'taki ölçüm bilgi satırıdır, temiz pencerede yinelenecek. 1 milyon öğeli `.TR_sort()` S-kilitten önce kilidi 598 ms tutuyordu [ölçüldü: D1, `e9c64a45`]; sonra yeniden ölçülmedi.
- **Yapılacaklar (C, Ö2, Kol B, K'nin kalanı; ölçüm S-kilit DEVİR'i):** GPU yolu kilidi tutuyor: tek `@`'i 32–43 ms süren GPU işinde başka görev ~330–343 ms bekliyor, 100 Hz'de tur kaçıyor (taban ve dal aynı) [ölçüldü: L4] (adım 5, ayrı dilim). `TR_backward` kilidi tutuyor (39 ms [ölçüldü: D1, `d1-10-2-olcum.md`]): tek çekirdek ve kapıya "çekirdeğin kendisi" belirteci gerekir. Yerinde yazan sık yollara (`+=`, indeksle atama, `.TR_step()`) girişte kilidi bırakarak izin beklemek. `TR_sample`'ın eşiği ölçülmedi (2^20 [tahmin]). Veri yerleşiklerinin kilidi bırakması. Önce ve sonra ölç: küçük tensör işlemli döngüde izinlerin bedeli (tahmin ~%5) [hipotez: DENETİM 10-2]. Geri çağıran yerleşikler: `TR_backward` (kullanıcı türeviyle), `TR_fuse`, anahtarlı sıralama, `TR_grad_check`. GPU'da iznin kuyruğun bitişine bağlanması [hipotez: GPU yolu açılmadı].
- **Yapılacaklar (C, Ö2, Kol B; 2026-11-03'ten beri):** `TestTheLockIsHandedOffInTimeAfterStatementsSlowDown` ve `TestLockIsTakenInArrivalOrderWithinTheDelay` Windows'ta yük altında kalıyordu. Bilinen sebepler kapandı: yedek saat kipinin iki okuma yolu (T1, `claude/kilit-devri`, `claude/kilit-b2`; DENETİM istek 94'ün V1 ve V2'si) ve işletim sisteminin kilidi tutan iş parçacığını çalıştırmaması (iki test Windows'ta yüksek öncelikte koşuyor; 10.2'nin sözü "işletim sistemi görevleri ve saati zamanında çalıştırdığı sürece" koşuluna bağlı, DENETİM istek 96 Ş1). Açık: B2'nin düzeltmesi CI'da ölçülmedi; `bea1089c`'nin 6/6 yeşilinden sonra CI'ın birkaç koşusu daha okunacak. Sınır 10.2'nin zaman sözüdür, gevşetilmez. Bedel kabul edildi (TCB, doğruluk önce): Windows'ta sınır +%20, döngü +%1,2; Linux'ta fark yok [ölçüldü: T1; Colab CPU].
- **Yapılacaklar (D, Ö3; sahip T1):** döngünün ve üretecin her turu devir noktası (`loopTurn`) [kanıtlı: `compiler_loop.go` `listComprehension`]; üretecin turunun gecikme testi yok.
- **Yapılacaklar (F, Ö3; sahip H1; S1 bulgusu, DENETİM 188):** çakışmada hazırlık yeniden yapılınca önceki çarpımın bitişik kopyaları havuza dönmüyor, çöp toplayıcı alıyor [kanıtlı: `unlocked.go:24`–`:38`, `cpu_memory.go`]. Yalnız başka görev aynı depoyu tutarken.
- **Yapılacaklar (F, Ö3; sahip T1; S1 önerisi):** çekirdek gövdesinde kapı çağrısını yakalayan kaynak testi: kural bugün yalnız yorumda. CPU gövdeleri 16 bit tamponu kapıdan iade ediyor, izin listesinde olmadığı için kilitlenmez (`cpu_dtype.go:199`): test bunu yanlış alarm saymasın.

### 10.3

- **Mevcut durum:** listedeki ağ istekleri, dış süreç ve sunucu beklemeleri bugün yerleşik olarak yok [ölçüldü: DENETİM 10-1]; kural gelecek yerleşikler için (yol haritasının "Ağ" maddesi).

### 10.6

- **Mevcut durum:** iptalden önce olan kendi hatanın program sonunda yazılmasının testi yok.

### 10.7

- **Açık:** 1.6'nın "durdurma istendi" hatasının kendi kodu (zaman aşımı ve iptalinki gibi). Adlı ayırma (yeni ad) Sonraya.

### 10.8

- **Mevcut durum:** paylaşılan listeye görevlerden ekleme sırası zamana bağlı ve her koşuda değişken (ör. `0..5` görevinde beş koşuda `[4, 2, 0, 1, 3]`, `[4, 0, 1, 2, 3]`, `[2, 1, 0, 3, 4]` ...) [ölçüldü: `0..5` döngüsünde `TR_async { l.TR_append(i) }` ile ardışık çalıştırma, `b77644dd`].

### 10.9

- **Yapılacaklar:** testi olmayan üç kısım: durum değiştiren yerleşiğin iptalde bitmesi, alt görevi bekleyen başka görev, `TR_handle_stop`.
- **Gemini Koşu 4'ün kalan iki maddesi:** 9.1'in son deyim olan `TR_defer` denetim hatası (denetleyicinin işi) ve 5.4'ün bileşik atamadaki değerlendirme sırası (görevlerden bağımsız yorumlayıcı işi).

## 11. Modüller

Kurallar: [`docs/spec/11_moduller.md`](spec/11_moduller.md). **Kapandı** (2026-10-29): ana oturum CLAUDE.md'den türeterek yeniden yazdı, iki tur DENETİM; kararlar Gemini Koşu 5'ten geçti (DENETİM doğruladı, `d31c9226`). Kitaplık modüllerinin İngilizce yolları (11.6) onaylandı (2026-10-29): `tr/layers`, `tr/optimizers`, `tr/pretrained`, `tr/training`.

### 11.1

- **Mevcut durum (D1, `7e7706dc`; A ve D kapandı):** göreli aktarma yalnız aktaranın klasöründe çözülür, harf büyüklüğü her sistemde diskteki adla aynı olmalı: H2721 diskteki adı, H2722 programın klasöründeki dosya için açık yolu önerir [test: `TestARelativeImportUsesOnlyTheImportersFolderAndTheDisksCase`, Windows; Linux tam denetimde].
- **Yapılacaklar (F, Ö3; sahip D1; DENETİM 281):** `TR_import "ortak.TR"` `ortak.TR.tr`'yi arıyor; ileti doğru yazımı (`"ortak"`) önermiyor [ölçüldü: D1]. Mutlak yolun harf büyüklüğü kuralda yazılı değil (`pending.md`).
- **Mevcut durum:** modülün en dışındaki deyim denetim hatası (H1107, 1.1); olumsuz fikstür `tests/negative/modul_en_dis.tr` [ölçüldü: `TestModuleTopLevelHoldsOnlyDefinitions`, 4a1 ilk dilim].
- **Yapılacaklar:** yok (1.1'e bağlı kalanlar 1.1'in maddesinde).

### 11.2

- **Mevcut durum:** köşeli ayraçla okuma ve atama (`m["PI"]`, `m["PI"] = 9`) takma adda denetimde, çalışma anının koduyla (H2904), ileti `m.PI`'yi öneriyor; indeks ad gibi yazılmış bir metin değilse (`m[k]`) ve tipi bilinmeyen kopyada çalışma anında [ölçüldü: `TestModuleValueRules`, `tests/negative/modul_koseli_ayrac.tr`, `tests/moduller_test.tr`'nin 11.2 testi].
- **Mevcut durum (2026-11-05; A, Ö0 kapandı; DENETİM istek 81 Ş2, 125 N4):** modülde olmayan ad (`m.x`) denetimden geçiyor (D, aşağıda), çalışırken `TR_none` yerine H7014 veriyor, ileti modülün en yakın adını öneriyor. Sözlüğün üyeleri de aynı hata: `TR_var k = m` ile `k.TR_remove("f")` modülün adını siliyor, sonraki `m.f` hatasız `TR_none` veriyordu [ölçüldü: `25e76eae`]; `k.TR_keys()`, `k.TR_get(...)`, `k.TR_length` de sözlük gibi okunuyordu [test: `tests/moduller_test.tr`'nin 11.2 testi, `TestAMissingModuleNameSuggestsTheClosest`].
- **Yapılacaklar (D, Ö1, 4a5; DENETİM istek 81 Ş2):** modül değeri; adların tipi denetimde; köşeli ayraçla erişim (çalışma anının kodu H2904), olmayan ad ve modüle özel ad (`m._gizli`, 11.7; bugün çalışma hatası) denetim hatası, en yakın adı öneren ileti; olumsuz fikstür. `tests/moduller_test.tr`'den yalnız sözlüğe özgü doğrulamalar (tip, uzunluk, anahtarlar, köşeli ayraç) çıkarıldı; nokta ile erişim ve modüle özel adın hata kodu kaldı. Modül değeri okurken de sözlük gibi görünüyor: `TR_for a TR_in m` adları dönüyor, `TR_print(m)` ve `TR_String(m)` sözlük yazımı veriyor; salt okuma, sessiz yanlış değil, 11.2 sözlük olmadığını söylüyor [ölçüldü: DENETİM istek 138 N2, `6fa66f5a` ikilisi].

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

- **Mevcut durum (2026-11-05; A, Ö0 kapandı; DENETİM istek 125 N1):** test komutuna iki dosya verilince yalnız ilki koşuyordu, ikincinin kalan testi bile geçmiş görünüyordu ("1 test, 1 geçti", çıkış 0). Artık verilen bütün dosya ve klasörler koşuyor, biri düşerse çıkış 1; olmayan hedef hata, hiçbir test koşmuyor; ilk dosyadan sonraki seçenek de sessizce atlanmıyor [test: `TestEveryTargetRuns`, `TestAMissingTargetStopsTheRun`, `TestTestCommand`].
- **Mevcut durum (2026-11-05; A, Ö0 kapandı; DENETİM istek 81, 132 Ş1):** test dosyası `TR_test` içeren dosya: klasör taraması bütün `.tr`'lere bakıyor, `testdata` adlı klasöre inmiyor; aktarılan modülün testleri de koşuyor, her dosya bir kez, kendi makinesinde. Test bloğu olmayan verilen dosya, yanında testli dosya olsa da, ve testi olmayan klasör H8503; hiçbir test koşmuyor. Eskiden testli modül ve aktarılan modülün kalan testi koşmadan "1 test, 1 geçti", çıkış 0 veriyordu [test: `TestDirScan`, `TestImportedModuleTestsRun`, `TestATargetWithoutTestsIsAnError`, `TestFixtureDirIsOnlyRunWhenGiven`].
- **Mevcut durum (2026-11-05; A, Ö0 kapandı; DENETİM istek 138 Ş1):** adı `_test.tr` ile biten dosya taramadan elenmiyor. Sözcük birimlerinden testi görünmeyen bozuk dosya (kapanmayan metin, adsız `TR_test`, kimliksiz yazım) "yüklenemedi", çıkış 1; `c131d68e`'de klasörde elenip ✅, çıkış 0 veriyordu. Testi olmayan böyle dosya raporda H8503, çıkış 1, klasörde de; yanındaki testler koşmuş olur. Düz adlı dosya da aynı (139 Ş2): `TR_test` sözcüğü ardındaki yazıma bakılmadan sayılıyor, sözcükleri bozuk dosyada ham metin aranıyor, dosya "yüklenemedi" [test: `TestABrokenNamedTestFileIsNotDropped`, `TestANamedTestFileWithoutTestsFailsTheRun`, `TestHasTestBlock`].
- **Mevcut durum (sınır, iş yok; DENETİM istek 141 N1, 142 Ş3):** düz adlı dosyada kimliksiz yazılan test taramada görünmüyor, dosya test sayılmaz. Kimliksiz yazım kod değildir: 12.1 `TR_test`'e bakar.
- **Yapılacaklar:** testin hangi cihazda koştuğu ve cihazlar arası tolerans 15'in E parçasında.
- **Gemini Koşu 7 (2026-10-29), uygulanacak:** test dosyası tanımı (göç işi bitti: en dışta deyim içeren dosya kalmadı, 4a1 ilk dilim) [ölçüldü: main `85a19c36`, iki dosyanın denetimi].

### 12.2

- **Mevcut durum (2026-11-05; A, Ö0 kapandı; DENETİM istek 138 Ş2):** `TR_test` en dışta değilse (`TR_if`, fonksiyon, başka test) denetim hatası, H7015; eskiden test koşmadan "✅ 0 test", çıkış 0 veriyordu [test: `TestATestIsOnlyAtTheTopLevel`, `TestANestedTestFailsTheRun`, `tests/negative/testdata/test_ic_ice.tr`].
- **Mevcut durum:** aynı dosyada aynı adlı iki test ikisi de çalışıyor [ölçüldü: main `e6392066`].
- **Yapılacaklar:** sözdizimi hatası ve olumsuz fikstür.
- **Mevcut durum (2026-11-05; F1, Ö3 kapandı; DENETİM istek 141 N3):** `TR_test`, `TR_as` ve `TR_init` bağlamının dışında sözdizimi hatası, H7017: ileti kimliği ve kopyalanınca çalışan yazımı veriyor; eskiden bilinmeyen ad iletisi görünüm adını yazıyordu [test: `TestMessageSuggestionsRunCleanly`, `tests/negative/kimlik_yeri.tr`].

### 12.3

- **Mevcut durum (2026-11-03, main `51b75f0a`):** tensörün `==`'si doğrulamada denetim hatası [ölçüldü: DENETİM istek 81]; ileti tensöre hâlâ `TR_assert(x == beklenen)` öneriyor.
- **Yapılacaklar (F1, Ö3; DENETİM istek 81):** iletinin tensöre doğru yazımı önermesi (15'in öğe öğe eşitliğiyle).

### 12.4a

- **Mevcut durum:** toleranslı karşılaştırma yok; ondalık kesin eşitlikle kalıyor (0.1 + 0.2 ile 0.3) [ölçüldü].
- **Yapılacaklar:** `TR_assert_close`, hassasiyete göre varsayılan toleranslar; TR testi.
- **Gemini Koşu 3 (2026-10-29), uygulanacak:** sonsuz öğe yalnız aynı işaretli sonsuza yakındır (15.18a ile aynı).
- **Gemini Koşu 7 (2026-10-29), uygulanacak:** sayı ile 0 boyutlu tensör, tek tolerans argümanı, isteğe bağlı mesaj.

### 12.3, 12.4

- **Mevcut durum:** `TR_assert`'in dönüşü 13.10'un status'unda. Tensör koşullu `TR_assert` 20 satır, 0 boyutlu tensör koşulu 4 satır (`tests/operatorler_test.tr:134-137`) [ölçüldü: DENETİM].

### 12.5

- **Mevcut durum:** her test kendi makinesinde koşar: en dıştaki tanımlar, aktarılan modüller ve paylaşılan üreteç her testte baştan kurulur; iki testin `TR_randn([2])`'si aynı [ölçüldü: D1, DENETİM 207; TR testi `tests/testler_test.tr`].
- **4a1 ilk dilim (2026-10-30):** test dosyasının en dışındaki deyim ve sabit olmayan ilk değer denetim hatası (H1107, H1108); depodaki yerler (1 deyim, geçici klasör ve komut satırı ayarı) kurulum fonksiyonuna alındı [ölçüldü: `TestRepoModulesHoldOnlyDefinitions`].
- **Gemini Koşu 7 (2026-10-29), uygulanacak:** hatalı test bitmemiş görevlerini önce iptal eder (10.9); bugün sonsuz işçiyle asılı kalıyor.

## 13. Yerleşikler

Kurallar: [`docs/spec/13_yerlesikler.md`](spec/13_yerlesikler.md). 6. ve 7. turun kararlarından üçü son ölçümde eski davranıştaydı: yazma yolunun programın klasörüne düşmesi, `TR_clamp(5)`'in iletisindeki eski kimlik, fonksiyon yerine verilen tipin iletisi [ölçüldü: DENETİM, 2026-10-29'un tabanı].

### 13.1

- **Yapılacaklar (D ve F, Ö3; DENETİM istek 73 kapanışı):** `TR_write_file`'a `TR_JSON` değeri yalnız çalışma anında H2623; denetleyici `TR_JSON`'u bildiği için denetimde reddedilebilir, o zaman öneri programın kendi ifadesiyle `TR_json_string(j)` olur (bugün yer tutuculu `TR_json_string(...)`).
- **Bedel (TCB kararı, 2026-11-04):** `utf8.ValidString` satır okumayı yavaşlatıyor: 1 GB'da ASCII +%9,5, Türkçe metin +%147,5 [ölçüldü: D1, Colab, altı tur]; doğruluk hızdan önce gelir, kabul. SIMD doğrulayıcı hız kolunda (D, Ö3).
- **Bulgu (D1, 2026-11-04, Ö3):** kesin istatistiğin bedeli on öğeli listede `TR_mean` +0,44 µs, `TR_std` +2,4 µs (5,8 kat), `TR_median` +0,26 µs [ölçüldü: Colab]. Sığan toplamda büyük sayısız yol düşürür [hipotez].
- **Mevcut durum:** `internal/detmath`'in bit özeti Windows, Linux ve qemu arm64'te aynı; gerçek ARM makinesinde koşmadı [ölçüldü: T1, `detmath` testi].
- **Yapılacaklar (D, Ö3; sahip D1, denetleyici; D1 tekdüze saat devri, 2026-11-09):** argüman almayan yerleşiğin fazla argümanı denetimde yakalanmıyor, yalnız çalışırken H2012: `TR_print(TR_monotonic_ns(1))` denetimden (`trlang` komutunun denetim kipi) hatasız geçiyor [ölçüldü: TCB, birleşik 60]; `TR_compute_dtype(1)` de [ölçüldü: D1, taban `fbf624ee`]. Tanım dosyasında `"params": []` yazılı; denetleyici argüman sayısını oradan okumuyor [hipotez: D1, kod okunmadı]. Ajan hatayı ancak koşunca görür.
- **Yapılacaklar (A, Ö1, 4c; öncelik DENETİM istek 97 Ş2'de düzeltildi):** boş tensör listesinin `TR_sum`'ı hata yerine 0 veriyor, tensör tipli değişkene tam sayı giriyor [ölçüldü: önem taraması teslim 3, `30b44b01` ve main `51b75f0a`]. Liste değeri öğe tipini taşımıyor, çalışma anında ayırt edilemez; `TR_sum[TR_Float]([])` da "indekslenemez" hatası veriyor [ölçüldü: DENETİM]. İş `pending.md`'nin 3.11'iyle aynı: parametre denetimden çalışma anına gelmeli.
- **Gemini Koşu 3 (2026-10-29), uygulanacak:** `TR_min` ve `TR_max`'ın anahtar fonksiyonlu biçimi sıralama sırasını kullanır; `TR_sum` tensör listesinde boşta hata (15.18).

### 13.2

- **Mevcut durum:** harf dönüşümü basit eşleme: `"ß".TR_upper()` → "ß", `"İSTANBUL".TR_lower().TR_length` 8 [ölçüldü: DENETİM].
- **Gemini Koşu 6 (2026-10-29), uygulanacak:** negatif indeks.

### 13.3

- **Yapılacaklar (A, Ö1, 4a2; DENETİM istek 81: anahtarın tipi denetlenmediği için metin parametreli anahtar tam sayı alıyor):** denetimin anahtar fonksiyonunun parametre sayısını (H2900) ve anahtarın tipini denetlemesi (4a2, denetleyici).

### 13.4

- **Bilinen sınır (hız; sınıfsız, sonuç yanlışı yok, Ö3; DENETİM istek 125 N3):** silmenin bedeli anahtar sayısıyla doğrusal, hepsini silmek karesel: arkadan silmede sırayı arama [kanıtlı: `removal.go`], önden silmede arama hemen biter, bedel `slices.Delete`'in kaydırmasıdır. Yalnız konum tablosu yetmez, sırada delik gerekir. 100.000 anahtarı önden silmek 2,9 sn, arkadan silmek 10,1 sn; 10.000 anahtarda 5.000 silme 42 ms [ölçüldü: Windows, `13-4/p.tr`, tek koşu].

### 13.6

- **Yapılacaklar (F, Ö3; sahip D1):** `TR_reduce`'un iletilerinin (H2475, H2476, H2637) metnini sınayan test (kodlar sınanıyor, metinler `internal/suggestion_messages_test.go` tablosunda yok).

### 13.7

- **Mevcut durum:** `TR_is_nan` yok (bilinmeyen kimlik); `TR_where(TR_true, 2, 3)` denetimden geçip çalışırken hata veriyor [ölçüldü: DENETİM istek 81, `30b44b01` ve main `51b75f0a`] (15'in yan değişikliği, uygulanmadı). Tensörde `TR_is_finite` `TR_fuse` içinde float32 0/1 kalıyor (15.16a).
- **Yapılacaklar (E ve D, Ö3; sahip D1, yeni kimlik ister; DENETİM istek 81, 184, 229 N5: yokluğu açık hata, `TR_where` çalışırken hata veriyor, sessiz yanlış değil):** fuse'ta H3107 kalır (kural fuse'ta yalnız `TR_is_finite` istiyor); `TR_is_finite(TR_Tensor([1, 2]))` hatasız, 13.7 yalnız `TR_Float` diyor (`TR_is_nan` bunu kopyalamaz); `definitions.json`'un koşul ve dönüş yazımı ile `tr.json`'un "0/1 maskesi" sözü düzelir; `TR_is_nan`'ın sayı ve tensör imzası; `TR_where`'in sayı imzasının kalkması, göçü 15.15'in status'unda (`tests/matematik_test.tr:208-209`) [kanıtlı: `tests/matematik_test.tr:208-209`]; TR testi.

### 13.8

- **Mevcut durum:** JSON'a NaN yazmak hata (H7008) [ölçüldü]. `TR_json_parse[T]` motorda yok; geldiğinde yinelenen anahtarı reddeden aynı çözücüden geçmeli. Dil tanımının sözlük üreteci (5.11, `{k: v TR_for k, v TR_in d}`) ve köşeli parantezdeki iki değişkenli üreteç (`TR_for i, x TR_in l`) motorda yok (sözdizimi hatası); H7012'nin önerisi bu yüzden döngüyle, CSV göçü aralıkla (`s[1..s.TR_length]`) yazıldı.
- **Gemini Koşu 6 (2026-10-29), uygulanacak (tensörün yazımı A, Ö0: tam sayı değerli ondalık öğe `.0`'sız yazılıp geri okununca tam sayı oluyor; yol ve `TR_json_parse[T]` A, Ö1, 18.4 ve 6.10; DENETİM istek 81 Ş4):** `TR_json_string`'de yol ve tensörün yazımı; `TR_json_parse[T]`'de `null`, iç içe yapı ve tensör alanının hatası.
- **Yapılacaklar (E, Ö3; sahip D1, Kol A 13; TCB 2026-11-09):** yapının JSON'a yazılması yok: `TR_json_string(K(3))` "K değeri JSON'a yazılamaz" veriyor [ölçüldü: TCB, `0a8073a4`]. `TR_json_parse[T]` ile birlikte Tur 1'in 13.8 ekleri de uygulanır: sayı alanında 3.7a'nın açma kuralı, kurulamayan yapının denetim hatası, döngülü nesnenin hatası.
- **Yapılacaklar (D ve F, Ö3; sahip D1, Kol A 13; TCB 2026-11-09):** `TR_json_parse[Ayar]` denetimden geçiyor, çalışırken H2465 iç adı sızdırıyor: "YerleşikFonksiyon tipi indekslenemez" [ölçüldü: TCB, `0a8073a4`]. Uygulanana kadar denetim hatası ve 13.8'i anan bir ileti gerekir.

### 13.8a

- **Mevcut durum:** `t.TR_tolist` tensörün boyutuna göre liste ya da sayı, `k.TR_decode` toplu girişte liste, `TR_parameters(x, TR_named: TR_true)` sözlük veriyor [ölçüldü: DENETİM].
- **Yapılacaklar (A, Ö1, 4a; DENETİM istek 81: 0 boyutlu tensörün `TR_tolist`'i liste tipli değişkene ondalık koyuyor):** `TR_batch_decode`, `TR_named_parameters` (yeni kimlikler, 4a); `TR_item`'ın iletisi (H3068'in metni "Item" diyor, `.TR_item()`'ı adıyla anmıyor); `TR_tolist`'in yalnız 1 boyutta, öğe tipi tensörün cinsi olan liste vermesi, 0 boyutta hatası (15'in yan değişikliği, uygulanmadı); `TR_named_parameters`'ın `TR_Dict[TR_String, TR_AnyTensor]` dönüşü (aynı) (argmax'in sonucunu indeks olarak kullanan kod `TR_Int(t.TR_item)` yazar, göçte sayılır). Dil tanımı her yerde çağrıyla yazıyor (`t.TR_tolist()`, on yer), motorda bir özellik: `t.TR_tolist()` "Liste tipi bir fonksiyon olarak çağrılamaz" veriyor, `lib/` ve `tests/`'te 456 çağrısız yazım var [ölçüldü: DENETİM, `kol-denetim-istek21-sonuc.md` 3]; motor dil tanımına uyar, yazımlar göçle değişir.

### 13.9

- **Mevcut durum:** katalog 188 fonksiyon ve metottan 138 Go yerleşiğinin imzasını tutuyor; 50'si TR ile yazılmış kitaplıkta (29 fonksiyon, 21 metot), imzası kaynakta. Tipler öğesiz (`TR_List`, `TR_Dict`), fonksiyon tipi tek bir `TR_Function`, yerleşiğin tipi `TR_BuiltinFunction` (11 yerde), genel tip `TR_Any` 24 yerde [ölçüldü: DENETİM]. Parametreler ve fonksiyon tipleri öğesiz.
- **Mevcut durum (tanım ile motor):** `TR_json_parse`'ın dönüşü uyuşmuyor: tanım `TR_JSON`, motor `TR_Dict` (13.8); `.TR_get`, `.TR_keys`, `.TR_length` üye tiplerinde `TR_JSON`/`TR_Bytes`'ı eksik yazıyor [test: `TestSingleReturnsMatchEngine`, `TestMemberTypesMatchDefinitions`]. Seçenekleri motorla uyuşmayan ya da kodunun parametreleri denetimde doldurulamayan fonksiyonlar denetlenmiyor (`TR_clamp`, `TR_concat`, `TR_argmax` gibi; listesi 4a4'ün DEVİR'inde).
- **Yapılacaklar (A, Ö1, 4a; DENETİM istek 81, 173: öğesiz imza ve tek fonksiyon tipi yüzünden tam sayı tipli değişkene metin ve ondalık giriyor [ölçüldü: önem taraması teslim 3, `30b44b01` ve main `51b75f0a`]; `TR_rfft`, `TR_stft` sonucu `TR_Any`, `TR_var g, s = TR_rfft(x)` sonrası `TR_var y: TR_Int = g` geçiyor [ölçüldü: S1, DENETİM 173]):** imzalar öğe ve fonksiyon tipleriyle; `TR_BuiltinFunction` ve `TR_Any` kalkar; metot parametresinin birleşim seçenekleri; dönüşün seçilen imzadan gelmesi (`TR_abs(1)` `TR_Int`, 4a5). Dönüşün öğe tipinin somut sessiz yanlışı ve sahibi 6.6'da.
- **Gemini Koşu 6 (2026-10-29), uygulanacak:** yerleşiğin fonksiyon değeri olarak eşleşmesi (isteğe bağlı parametreler boş, adlı argümanlar varsayılanda; varsayılansız adlı argümanda hata). Denetleyicinin işi (4a2).

### 13.10

- **Mevcut durum:** 40 yerleşiğin dönüş tipi birleşim (`TR_abs`, `TR_floor`, `TR_exp` gibi; 36 fonksiyon ve `.TR_decode`, `.TR_grad`, `.TR_token_id`, `.TR_tolist`); dönüşü argümanın değerine bağlı üç yer: `TR_tolist` (tensörün boyutu), `TR_decode` (toplu giriş), `TR_parameters` (`TR_named:`); 8 yerleşik değişken sayıda argüman alıyor; yalnız 7 metodun alıcıya göre ayrı imzası var [ölçüldü: main `85a19c36`, `definitions.json`'dan sayıldı].
- **Yapılacaklar (A, Ö1, 4a; DENETİM istek 81: `TR_abs(-1.5)` tam sayı tipli değişkene ondalık koyuyor; `TR_custom_grad`'ın verdiği fonksiyonun dönüşü izlenmiyor [ölçüldü: önem taraması teslim 3, `30b44b01` ve main `51b75f0a`]):** sekiz kısıt ("number", "ordered", "equatable", "key", "json", "struct", "function", "tensor_data"); `TR_command_line` ve `TR_Checkpoint`'in yapıyla yazımı (göç: 18 ve 54 satır, `.TR_state` 39); `TR_topk`'nın iki değeri (6.3, 15'te), `TR_assert`'in iletisinin `TR_String` ya da `TR_Message` olması (12'de); kısıtlı tip parametreleri, ayrık imzalar, değere bağlı dönüşlerin sabit dönüşlü yerleşiklere ayrılması (15 ve 16. bölümlerle), değişken sayıda argümanın yazımı (4a).
- **Mevcut durum (15'in yan değişiklikleri, uygulanmadı):** tensör verisi kısıtının cinse bağlanması, varsayılansız adlı argüman, `TR_save_tensors`'un kabul kümesi, `TR_load_tensors` ve `.TR_narrow[E]()`'nin imzaları, `TR_quantized`'ın imzası ve `TR_custom_grad`'ın dönüşü imzalarda yok.
- **Mevcut durum (değeri kullanılmayan çağrı):** H1115 yalnız `pure` işaretli 52 yerleşikte; `TR_sqrt` ve tensör işlemleri (`TR_softmax`, `TR_log_softmax`, `TR_attention` …) işaretsiz, tek başına çağrıları hata vermiyor [kanıtlı: `definitions.json`, `checker/pure_calls.go:40`; hipotez: koşulmadı, DENETİM 360].
- **Yapılacaklar:** değer veren tensör işlemlerinin ve 13.1'in hesaplarının `pure` işareti; TR testi.

### 13.11

- **Mevcut durum:** çalışma anındaki yerleşik fonksiyon iletileri görünüm adıyla: "sqrt() expects Int or Float as argument 1, String (\"a\") given", "sqrt() takes 1 argument, 2 given (usage: sqrt(number))" [ölçüldü]. Tipi bilinen alıcının özelliğini çağırmanın iletisi (H2421) yalnız tipi söylüyor ("Tam tipi bir fonksiyon olarak çağrılamaz"), özelliği ve parantezsiz yazımı söylemiyor [ölçüldü: `TestPropertyCall`, `tests/negative/ozellik_cagrisi.tr`].
- **Mevcut durum (öneri yuvası):** H2421 (özelliğin çağrılması) ve H2423 (yanlış argüman tipi) bir `hint` parametresi taşıyor; çalışma anı onu boş geçiriyor [kanıtlı: `interp/calls.go:221`, `interp/arg_errors.go:97`]. Denetleyicide açılmamış değerin yazımı (H1130) dışındaki doğru yazım önerileri (13.9) yok. Liste ve Tam verilen argüman çalışma anının önerili koduna (H6104, H6020) kalıyor.
- **Yapılacaklar:** öbür fonksiyonların argüman tipleri (seçenekleri ya da kodları motorla uyuşmayanlar); H6104 ve H6020'nin önerisi denetimde kurulabilirse liste ve Tam argüman; çekirdek olmayan alıcıların metotları; fonksiyonların ve öbür alıcı tiplerinin (tensör, kitaplık tipleri) sayı denetimi; fonksiyonların H2011 iletisi hâlâ görünüm adıyla (`TR_round` için "round()").

## 15. Tensörler ve otomatik türev

Kurallar: [`docs/spec/15_tensorler.md`](spec/15_tensorler.md). Aşağıda bugünkü motorun kurallardan ayrıldığı yerler ve göç.

### 15.1

- **Mevcut durum:** `TR_AnyTensor` ve `.TR_narrow[E]()` yok; `TR_Tensor[E]` ayrıştırılıyor ama `TR_Tensor`'a indirgeniyor [ölçüldü: bulut 0.6a]. GPU'da kapsam dışı cinsli işlemler H6969 [ölçüldü: Gemini 4, benzetici, `TestTypedPathsOnGPU`; kartta koşmadı]; tam sayı içinde `.TR_to_dtype` ve tam sayıya tip çağrısı konakta yapılıp cihaza döner, `TR_replay` kaydında çalışmaz. Veri kümesi kimlikleri float32 kurulup "int64"e çevriliyor (`dataset.go` → `ml.NewInt`): 2²⁴'ün üstündeki kimlik açık hata veriyor, H5536 (E; sessiz bozulma yok [ölçüldü: DENETİM 231]) ve 32K öğe başına ~0,3 ms konak işi var (T1 bulgu 12) [ölçüldü: T1, benzetimde profil; 2²⁴ sınırı kanıtlı: float32'nin anlamlı bitleri].
- **Mevcut durum (sınıf, DENETİM 360 Ş1):** kuralın cins ve imza denetim hataları bugün çalışmada geliyor ya da hiç gelmiyor, çünkü denetleyici tensörün cinsini bilmiyor (yukarıdaki indirgeme, 15.24): 15.2'nin `TR_Tensor[TR_Int]([1.5])`'i, 15.6'nın cinsli işleç tablosu (`@` dahil) ve karışık cinsli işlem, 15.6a'nın sığmayan sayısı ve liste işleneni, 15.18'in konumlu ekseni, 15.18a'nın cins farkı, 15.49'un cinsleri.
- **Açık küçükler (DENETİM istek 54 ve 56):** cinsli GPU testlerinin listesi 2026-11-02'deki iki düzeltmeden sonra kartta yeniden koşulacak; cinsli çekirdek bileşimlerinin (`gpuTypedOK`'un açtığı hepsi) ad, benzersizlik ve ayrışma testi; benzetimin `.target` denetimi (`mma.sync`, `cvt.rn.*x2.f32`); Colab'da bir kez ptxas karşılaştırması; `checkKernelNames` yalnız bütün testler geçince basıyor; benzetimde ns/op güvenilmez, `BenchmarkIDBatchToHost`'un ölçütü `eşzamanlama/op`.
- **Yapılacaklar (A, Ö1, 4c; DENETİM 231: `TR_Tensor[TR_Int] = TR_Tensor([1.5])` float32 kuruyor, tipli yerin dokuzuncu yolu [ölçüldü: DENETİM 231]):** öğe cinsli tip (`TR_Tensor[E]`, tip denetimi 4c), `TR_AnyTensor` ve daraltma; GPU'da cinsli tensörün genel çekirdekleri (2b); `TR_replay` kaydında tam sayı tensörünün kopyası ve tam sayı tensörünün sayıyla işlemi (sayı bugün konaktan yükleniyor).
- **Yapılacaklar (E, Ö3; sahip T1; DENETİM 278, 280):** CPU'da float64 hesabı 15.1'e göre var olmalı, motorda çoğu işlem yok: `@`, `TR_conv1d`, `TR_exp`, `TR_log`, `TR_tanh`, `TR_sigmoid`, `TR_sqrt`, `TR_gelu`, `TR_relu`, `TR_softmax`, `TR_layer_norm`, `TR_ceil`, `TR_embedding_lookup` H6960 veriyor; çalışanlar `TR_where`, `TR_cumsum`, `TR_sum`, `TR_concat`, `**`, `TR_clamp`, tensör indeksi [ölçüldü: S1, `f64.tr`]. `tests/conv1d_test.tr:64` bu eksikliği bekliyor; uygulanınca ters döner. F1: H6960'ın metni CPU'daki float64'te "bu hesap float32, bf16 ya da fp16 ister" diyerek yanlış kuralı öğretiyor.

### 15.2

- **Mevcut durum (sınır):** cins çalışırken değerden çıkıyor, tip denetiminden değil: `TR_Float` parametresine verilen tam sayı yazımı ondalık geliyor ve ondan kurulan tensör `TR_Float` (`p(2)` → `[2.0000]`) [ölçüldü: main `d24b95ea`]; tipi ilk değerinden çıkan değişken (`TR_var i = 1`) `TR_Int` kalıyor. Tip denetiminin önerisi (`TR_Tensor[TR_Float](kimlikler)`) 4c'de.
- **Gemini Koşu 3 (2026-10-29), uygulanacak:** `TR_zeros`, `TR_ones` ve `TR_eye`'ın mantık cinsi.
- **Gemini 15/2 (2026-10-29), uygulanacak:** `TR_zeros(..., TR_dtype: "q4")` gibi nicemlenmiş sıfır kurucu (T3).

### 15.3

- **Mevcut durum:** sürüm iletisi (H3012) `TR_sigmoid`'in ezilen çıktısına "girdi" diyor [ölçüldü: DENETİM]. Hesap ondalığından float64'e `.TR_to_dtype` türev izlemiyor [ölçüldü].
- **Yapılacaklar (E, Ö3; sahip T1; DENETİM 231 N2):** `.TR_fill` kuralda var, kimliği yok [ölçüldü: DENETİM 231].
- **Yapılacaklar (E, Ö3; sahip T1; DENETİM 280):** iki argümanlı `.TR_slice(eksen, baş)` H6717 veriyor; 15.3'ün örneği (`x.TR_slice(1, 1)`) kopyalanınca çalışmıyor [ölçüldü: S1].
- **Yapılacaklar:**
  - `TR_wrapping_add`, `TR_wrapping_sub` ve `TR_wrapping_mul`'ın tensör imzası; PyTorch'un `.float()`/`.long()`/`.bool()`/`.to(...)` alışkanlığına doğru yazımı öneren ileti (bugün genel "özellik ya da metot bulunamadı");
  - GPU'da çekirdeklerin hata bildireceği bir cihaz bayrağı yok [kanıtlı: DENETİM]: taşmanın hatası için yeni düzenek ve işlemden hemen sonra okuma (15.6'nın bekleme listesi);
  - sürüm iletisinin girdi ile çıktıyı ayırması;
  - yaprak olmayan türevli tensöre yerinde yazmanın türevsiz kipte de hatası;
  - tensör koşulunun denetim hatası ve iletileri; TR testi.

### 15.4

- **Mevcut durum:** tensörde `TR_std` H2423 veriyor [ölçüldü].
- **Yapılacaklar:** `TR_std`'nin tensör yolu (H2423) ve blok kuralı.

### 15.5

- **Mevcut durum:** `TR_fuse` kaydında karşılaştırma 0/1 ondalık maskedir, `==`/`!=` orada H3107 (kaynaşmış çekirdekte mantık cinsi yok). Tensörün metni kurma yazımı değil: dört basamaklı (`[2.0000]`) ve bütün öğeler yazılıyor [ölçüldü]; kural metni kurma yazımı sayıyor (15.5'in örneği), "bütün öğeler"in fark olup olmadığı belirsiz [hipotez: DENETİM 360].
- **Yapılacaklar (A, Ö1; DENETİM istek 100 N1):** kaynaştır kaydında `TR_Bool` maske: fuse içinde karşılaştırma ve `TR_is_finite` `float32` kalıyor, `(t > 1.5) * t` fuse içinde çalışıyor, dışında hata veriyor, 15.44'ün "bit bit aynı" sözüne aykırı [ölçüldü: DENETİM, `c15dc932`]; eşitlenemeyen değerlerin denetim hatası, metin.
- **Gemini Koşu 3 (2026-10-29), uygulanacak:** gösterimde cinsin varsayılanı dışındaki hassasiyet.
- **Yapılacaklar (E, Ö3; sahip D1, sonraki eşitlik dilimi; D1 15.5 devri Bulgu 2):** kitaplığın ve aktarılan modülün tipleri denetleyiciye bilinmeyen addır (`equatable.go`'nun `typeDefs`'i yalnız denetlenen programın kendi tanımlarından dolar): tensör alanlı böyle bir tipin (`TR_Sound`) `==`'si denetimden geçer, çalışırken H2770 verir [hipotez: aktarmalı program kurulmadı]. Sıradaki adım: ölç; doğruysa aktarılan modülün tanımları `typeDefs`'e.

### 15.6

- **Mevcut durum:** kitaplıkta pencerenin sınırı konumların float32 kopyasıyla karşılaştırılır (GPU'da tam sayı çıkarması yok; konum < 2²⁴ kesin).
- **Yapılacaklar:** cinsli işleç tablosu, bekleyen işlemler ve `TR_replay`'deki cihaz içi bayrak; TR testi.
- **`@`'nin toplu eksenleri:** bugün öndeki eksenler eşit olmalı; kural onları yayınlar (gerekçe 15.6).

### 15.6a

- **Mevcut durum:** liste işlenen hata vermeden çalışıyor: `TR_Tensor([1.0, 2.0]) + [10, 20]` → `[11.0, 22.0]` [ölçüldü].
- **Yapılacaklar:** liste işlenenin denetim hatası; tam sayı tensöründe sayının sığma denetimi; TR testi. Göç [ölçüldü: DENETİM B1]: liste işlenen 2 yer (tests/matematik_test.tr, tests/tensor_test.tr).

### 15.6b

- **Yapılacaklar:** eksen sayıları eşitken iki yanlı genişlemenin uyarısız kalması doğrulanır; TR testi.

### 15.6c

- **Yapılacaklar:** tam sayı hassasiyetlerinin birleşmesi; TR testi.

### 15.6d

- **Yapılacaklar (E, Ö3; sahip T1; DENETİM istek 172):** kural var, motorda açık hata: tensörle indekse yazma. Satır yazan tensör işlemi, yinelenen indeks denetimi, 16 bitte birikim, `t[m]`, kart yolu.
- **Yapılacaklar (A, Ö0; sahip T1; DENETİM istek 172, 340 Ş4):** `t[idx][0] = 9.0` ve `t[idx][0] += 5.0` kopyaya yazıyor, `t` değişmiyor, çıkış 0 [ölçüldü: T1, DENETİM 172; TCB, `0a8073a4`, 2026-11-09]. Aynı kayıp `(t * 2.0)[0] = 9.0`, hassasiyet dönüşümünün (`.TR_to_dtype`) sonucunda ve `TR_Tensor(t)[0] = 9.0`'da [ölçüldü: DENETİM 342, `072d84ee`]. 15.6d 2026-11-09'da bunu yasakladı (kritik; Tur 5'te kesinleşti): hedef zincirinde kopya veren adım denetim hatası, tip bilinmiyorsa çalışma anında. İleti anlamı koruyan çalışan yazımı önerir; `.TR_select` ve tensörle indekse yazma (H7042, aşağıda) gelene kadar H7042'nin döngüsü. Tur 5 (DENETİM 352) aynı kurala `TR_write_slice`'ın hedefini kattı: `TR_write_slice(t[idx], …)` `f312d906`'da 15.8'in yalnız bitişiklik denetiminden (`internal/stdlib/ml/builtin_replay.go`, `IsContiguous`) geçiyor, yazılan kopyada kalıyor [kanıtlı: DENETİM 352].
- **Mevcut durum:**
  - maskeyle indeks yok: `t[m]`, `t[m] = d` H6707; `t[idx] = d` ve `t[idx] op= d` H7042, ileti döngüyü önerir [test: `TestTensorIndexWriteSuggestionRuns`];
  - `%=` ve bit işleçli bileşik atama yeniden ölçülmedi;
  - `.TR_select`, `.TR_unsqueeze` ve `.TR_fill` yok; `.TR_slice` 31 yerde kullanılıyor [ölçüldü: grep]; `m[:, None, :]` en az 4 yerde `m.TR_reshape([B, 1, T])` diye yazılıyor [ölçüldü: DENETİM].
- **Yapılacaklar:**
  - tensörle indekse atama, maskeyle indeks, `.TR_select`, `.TR_unsqueeze`, `.TR_fill` (üçü de türevin yazma kurallarıyla), tensörle indekste biriktirme, `%=` (cinsli tensörle; `//=` çalışıyor [ölçüldü: Gemini 4]); TR testi;
  - kitaplığın `TR_Tensor(TR_no_grad(...), TR_requires_grad: TR_true)` dolambacı (lib/layers.tr:20) yaprağın tanımıyla sadeleşir;
  - indeksli hedefte yerinde olmayan bir biriktirme (index_add türü) yok; açık soru;
  - liste değeri cinsli hedefe atanınca (`t[0..2] = [1, 2]`) H6966 veriyor; 15.6d'nin önerdiği `TR_Tensor(...)` yazımını öneren ileti değil (T1 dilim 2, DENETİM istek 49 Y3) [ölçüldü: DENETİM istek 57].
- **Tersine dönecek Go testi:** `TestMatrixIndexAndAssign` (`internal/stdlib/ml/matrix_test.go:50`) tam sayı değişmezli matrisi ondalık bekliyor (15.2), 0 boyutlu tensörde `.TR_tolist` okuyor (13.8a) ve liste değerli atama yapıyor (`M[-1] = [8, 9]`; değer sayı ya da tensör olmalı). Aynı dosyanın öbür testlerine de bakılmalı.

### 15.6e

- **Mevcut durum:** `2 ** t` ve `t ** u` (tensör üs) H6702 veriyor; ileti ikisini ayırmadan genel `TR_exp(TR_log(t) * u)` yazımını öneriyor (taban `2` ya da üs tensörü olduğu anlatılmıyor) [ölçüldü: main `85a19c36`].
- **Yapılacaklar:** tensör üs ve tam sayı tensöründe eksi üs hatası; TR testi.

### 15.8

- **Mevcut durum:** başlangıç tam sayı ya da tam sayı değerli 0 boyutlu ondalık tensör.
- **Yapılacaklar:** `TR_start`'ın `TR_Int` ya da 0 boyutlu `TR_Tensor[TR_Int]` olması; cins denetimi; `TR_replay`'de sınır dışı başlangıcın cihaz içi bayrakla hatası (15.45).

### 15.13

- **Yapılacaklar (E, Ö3; sahip T1; DENETİM 234):** tensör sınır motorda yok: `TR_min:` tensörle çalışma hatası veriyor [ölçüldü: DENETİM 234], 15.13 istiyor.

### 15.15

- **Mevcut durum:** ondalık ya da tam sayı koşul ve sayı imzası (`TR_where(TR_true, 2, 3)`) denetimde değil, çalışmada H6207 veriyor [ölçüldü].
- **Yapılacaklar:** hataların denetimde verilmesi (tanımdaki `TR_where` imzası bugün `TR_Bool|TR_Int|TR_Float` koşul ve sayı dönüşü yazıyor; satır bu dilimin değil); `tr.json`'daki açıklamanın "0/1 maskesi" metni.
- **Gemini Koşu 3 (2026-10-29), uygulanacak:** iki sayılı biçimin cinsi ve hassasiyeti.

### 15.16a

- **Mevcut durum:** `TR_maximum` ve `TR_minimum` `TR_fuse` içinde H3107. Tensörde `TR_round` (H2423) ve `TR_is_nan` yok [ölçüldü: main `85a19c36`, `TR_is_nan` bilinmeyen kimlik; `TR_round(t)` H2423]. `TR_is_finite` `TR_fuse` içinde float32 0/1 kalıyor (15.5). Kartta `TR_sum(TR_is_finite(x))` H6969 veriyor: mantık tensörünün toplamı GPU'da yok, ileti tensörü CPU'ya taşımayı öneriyor; kıyas maskesi `TR_sum(x > 0.5)` de aynı [ölçüldü: L4, 2b teslimi].
- **Yapılacaklar:** tensörde `TR_round`, `TR_is_nan`; TR testi.

### 15.17

- **Mevcut durum:** `TR_approximate:`'in varsayılanı var ve tanh [ölçüldü].
- **Yapılacaklar:** zorunlu ve yalnız adla verilen `TR_approximate:`. Göç [ölçüldü: DENETİM B1]: argümansız 16 çağrı (examples/tr/ 7, tests/tensor_test.tr 5, Go testlerinin TR metni 4); göçte `TR_approximate: TR_true` yazılır, bugünkü sonuç korunur.

### 15.18

- **Yapılacaklar (C, Ö3; sahip D1; D1 devri):** gömülü kitaplık denetlenmiyor, çağrıdaki öğe tipi kitaplığa ulaşmıyor; bugün kitaplıkta boş listeyi toplayan yer yok [ölçüldü: `grep -n "TR_sum(\|TR_mean(" lib/*.tr`]. Kitaplığa liste toplayan bir `TR_sum` eklenirse işaret olmaz.
- **Yapılacaklar (ölçü, sahip D1; D1 devri):** kabuğun (REPL) aktardığı modülün denetimden geçip geçmediği ölçülmedi; geçmiyorsa modülde boş ondalık listenin toplamı `0` kalır [hipotez].
- **Yapılacaklar (E, Ö3; sahip T1; DENETİM 280):** `TR_dim:` listesi (`TR_sum(x, TR_dim: [1, 2])`) H6106 veriyor [ölçüldü: S1].
- **Mevcut durum:** eksen `TR_sum`/`TR_mean`'de konumla da veriliyor [ölçüldü]; `TR_max`/`TR_min`'de konumlu ikinci argüman çalışmada H6205 (kural denetim hatası diyor) [kanıtlı: DENETİM 360, `builtin_reduce.go:35`–`:38`]. Maskenin ve `TR_argmax` sonucunun toplamı "int64" tensör [ölçüldü: Gemini 4, DENETİM 166]; maskenin ortalaması float32 [ölçüldü: DENETİM 170]. `TR_std` tensörde H2423 veriyor [ölçüldü]; `TR_logsumexp` GPU'da yok (15.4).
- **Yapılacaklar:** eksen yalnız adla; göç 3 yer [kanıtlı: `tests/tensor_test.tr:366`, `tools/gpu/ss_exam.tr:59`, `internal/stdlib/ml/builtin_reduce_test.go:86`]: `tests/tensor_test.tr:366`, `tools/gpu/ss_exam.tr:59`, Go dizesi `internal/stdlib/ml/builtin_reduce_test.go:86`. Cinsli sonuçlar, "int64" taşma hatası, `TR_std`, GPU'da `TR_logsumexp`, liste eksen; TR testleri.
- **Gemini Koşu 3 (2026-10-29), uygulanacak:** boş eksen ile boş tensörün ayrımı, boş tensör listesinde hata, `TR_std`'nin sıfır sapmadaki türevi 0.

### 15.18a

- **Mevcut durum:** tensör olmayan argümanı (`TR_equal`'da H6979) denetleyici yakalamıyor, çalışmada hata.
- **Yapılacaklar:** `TR_any`, eksenli `TR_all`/`TR_any` ve `TR_keepdim:`; ondalık ya da tam sayı tensörün denetim hatası (bugün çalışmada H6979); cins farkının denetim hatası (bugün çalışmada H6966).

### 15.19

- **Mevcut durum:** `TR_argmin` yok.
- **Yapılacaklar:** `TR_argmin`, eksenli `TR_keepdim:`; TR testi.
- **Gemini Koşu 3 (2026-10-29), uygulanacak:** boş indirgemede hata.

### 15.20

- **Mevcut durum:** `TR_cumsum`'un ekseni varsayılan −1.
- **Yapılacaklar:** `TR_dim:` zorunlu; göç `.tr`'de 7 satır, Go dizelerinde 3 (ikisinin beklenen ileti kodu değişir) [ölçüldü: DENETİM C1]. `TR_Int`/`TR_Bool` girdide "int64" ve taşma hatası.

### 15.21

- **Mevcut durum:** `TR_topk` liste veriyor (`TR_Type`'ı `TR_List`) [ölçüldü]; demetin değeri motorda var (0.6b).
- **Yapılacaklar:** demet dönüşü (önce 3.6a'nın demeti); göç 9 çağrı [ölçüldü: DENETİM C1]: `examples/tr/25_secme_ve_siralama.tr:22-23`, `tests/matematik_test.tr` 7 satır, Go 1; TR testi.

### 15.21a

- **Mevcut durum:** `TR_gather` yok. MMS-TTS'in `.tr` yedeği (tek-sıcak seçim) profilde %0 [ölçüldü: S1, 2026-11-08].
- **Yapılacaklar:** uygulamak; TR testi. Gösterim adı onaylı (Ahmet, 2026-10-29).

### 15.22

- **Mevcut durum:** `TR_topk` ve `TR_argsort`'ta sıralanan eksen en çok 2²⁴ uzunlukta (indeksler float32 depoda); `TR_descending:` yok (motorda `Backend.Sort(..., desc)` hazır [kanıtlı: internal/compute/tensor/device.go:347]).
- **Yapılacaklar:** 2²⁴ sınırı tam sayı depolamasıyla (15.1) kalkar, yeni sınır 2³² − 1; `TR_descending:`; TR testi.
- **Uygulama sınırı:** GPU'da sıralanan anahtarın alt 32 biti indekstir; bu yüzden eksen en çok 2³² − 1 uzunlukta ve sıra anahtarı 32 bit (float32'de yeter; int64 ve float64 tensörde ve daha uzun eksende yeni bir yol gerekir). Aynı sınır `TR_topk`'ta. Kural bir sınır koymaz (bellek dışında, 15.42).

### 15.22a

- **Yapılacaklar (D, Ö3; en son bakılacaklar, H1 hız kuyruğu):** CPU'da eksen 0 toplamının 28 baytlık iç döngüsü 64 baytlık sınırı geçince `BenchmarkSpeedSumAxis0` ~%26 yavaşlıyor; buyruklar aynı, yalnız döngünün yeri değişiyor, kaydırınca fark tersine dönüyor [ölçüldü: AMD EPYC 7B12, `e27cdff0` +%26,0, kaydırılmış uç −%1,8, kopya farkı %2,9]. `karakter_modeli` etkilenmedi.
- **Yapılacaklar (C, Ö2; ölçülmedi):** `TestReductionsIndependentOfThreadCount` Go testi bölümün commit'inde yazılır (bugün yok); 15.22a başlıklı TR testi.
- **Bulgu (T1, 2026-11-04, D, Ö3; TCB kararı):** sayı yerleşikleri `internal/detmath`'te, tensör yolu `math.*`'ta ve float32'de. Sayıyı float32'ye yuvarlayıp tensörle kıyasta `TR_log` ailesi, `TR_tan` ve ters trigonometri ayrılmıyor; `TR_sin`, `TR_cos`, `TR_tanh`, `TR_exp` en çok 1 float32 ulp, `TR_sigmoid` 2, `TR_silu` 3 ayrılıyor [ölçüldü: DENETİM istek 126, 200 000 girdi]. Sebep çoğunlukla `TR_exp`, `TR_sigmoid`, `TR_silu`'nun kendi float32 yolu [kanıtlı: DENETİM, `cpu_elementwise_wide.go`]. Bu tek algoritmanın sonucu değil [hipotez: DENETİM, eski ikiliyle ölçülmedi]. 15.22a sayı ile tensörü anmıyor: aynı cihaz türünü ve cihazlar arasını anlatıyor.
- **Yapılacaklar (D, Ö3; sahip H1, hız kuyruğu; DENETİM istek 126):** `TR_sample`'ın `TR_top_k: 50`'si çağrı başına 60 ms [ölçüldü: DENETİM, Windows]: bütün sözlüğü sıralıyor. Tek algoritmanın top-k'sız örneklemeye bedeli +%26,7, çağrı başına +1,66 ms (Colab Xeon, `measurements.md`), bundan küçük; üretimde CPU +%1,8 [hesap: DENETİM].

### 15.22b

- **Yapılacaklar:** `TR_argmin` gelince aynı sıra.

### 15.23

- **Mevcut durum:** sınır dışı indeks H6305 veriyor [ölçüldü: main `8b115692`]. İndeks liste ya da ondalık tensör olabiliyor.
- **Yapılacaklar:** indeks `TR_Tensor[TR_Int]`; göç 24 satır [ölçüldü: DENETİM, `git grep` main `8b115692`]; sınır dışı denetimi bekleyen işlem (15.6).

### 15.23a

- **Yapılacaklar:** iki denetim hedefi konağa okuyarak yapılıyor (GPU'da bekler); `TR_replay` kaydındaki cihaz içi bayrak 15.6'nın bayrağıyla gelir (bugün kayıtta cihazdan okuma H4065 [hipotez: `internal/compute/gpu/backend_memory.go:181`; GPU'suz ölçülmedi]).

### 15.23b

- **Mevcut durum:** `TR_group_norm` yok [ölçüldü: main `8b115692`].
- **Yapılacaklar:** `TR_group_norm`; TR testi.

### 15.23c

- **Mevcut durum:** `TR_conv2d` yok. `TR_conv1d`'in gradyan ölçütü (TCB kararı, DENETİM 186): mutlak sınır en büyük gradyan öğesinin 1e-5 katı; öğe öğe göreli sınırı float64 doğrusu bile geçmiyor.
- **Yapılacaklar:** `TR_conv2d`; TR testi. Gösterim adları onaylı (Ahmet, 2026-10-29).

### 15.23d

- **Mevcut durum:** `TR_dropout` yok [ölçüldü: main `8b115692`].
- **Yapılacaklar:** paylaşılan üreteçten (20.17, oradaki yazılabilir üreteçle birlikte); TR testi.

### 15.24

- **Mevcut durum:** `TR_mask:`'in cins hatası (H6985) çalışma anında: denetleyici tensörün cinsini bilmiyor (`TR_Tensor[TR_Float]` tipli değişken de `TR_Tensor`) [ölçüldü: T1 `claude/kolB-o0-3`, 9 yazım].
- **Yapılacaklar:** dil tanımının "denetim hatasıdır" sözü denetleyici tensör cinsini bilince (15.6'nın cinsli tipi).
- **Gemini 15/2 (2026-10-29), uygulanacak:** tamamen −∞ sapmalı satır 0 verir; CPU ve GPU (T2). CPU'da tamamen maskeli `TR_Bool` satırı zaten `[0.0, 0.0, 0.0, 0.0]` [ölçüldü: DENETİM istek 61, `30b44b01`]; GPU ölçülmedi.

### 15.27

- **Yapılacaklar:** bloktaki `TR_defer` kipin içinde.

### 15.27a

- **Mevcut durum:** `TR_detach` yok; ayırma `TR_no_grad(x)` ile yapılıyor ve `d = TR_no_grad(h); d[0] = 9.0` türev izleyen `h`'yi sessizce değiştiriyor [ölçüldü].
- **Yapılacaklar (A, Ö1, Kol B, 15.6d ile; ölçüm status'un):** `.TR_detach()`; ayrılmış görünüme yazma kuralları (15.6d); `TR_no_grad(x)` biçimi kalkar, göç 20 satır [ölçüldü: DENETİM, `git grep` main `8b115692`]; TR testi.

### 15.28

- **Mevcut durum:** `f_ileri` `[çıktı, [saklananlar]]` listesi döndürüyor [ölçüldü]; eski metne göre çağrının değeri `TR_BuiltinFunction`.
- **Yapılacaklar:** demet biçimi (önce 3.6a); dönüş tipi; göç 22 satır [ölçüldü: DENETİM, `git grep` main `8b115692`].

### 15.29

- **Mevcut durum:** eski metne göre `geri` liste döndürüyor [ölçüldü: DENETİM].
- **Yapılacaklar:** tek girdide kendisi, birden çokta demet; fonksiyon ifadesinde öğe sayısının denetim hatası.

### 15.32

- **Yapılacaklar:** `girdiler` demet.

### 15.33

- **Mevcut durum:** `TR_parameters` `TR_named:` argümanıyla yol adlı sözlük veriyor; `TR_named_parameters` adı yok; yapıdaki ara sonuçlar ve türev izlemeyen tensörler sessizce atlanıyor [kanıtlı: bugünkü 15.33'ün metni; `definitions.json`'da `TR_named_parameters` kodu yok].
- **Yapılacaklar (A, Ö1, Kol B, 13.8a ile; ölçüm status'un):** `TR_named:` kalkar, `TR_named_parameters` (13.1) gelir; parametre tanımı (donmuşlar `TR_parameters`'ta); `TR_named_parameters` bütün yapraklar; türevli ara sonuçta hata; TR testi bunlara göre.

### 15.34

- **Mevcut durum:** kendi iyileştiricisi (`TR_if g != TR_none`) kullanılmayan parametreyi ayıramıyor; kitaplığın Muon'u için soru üyesi Sonraya (Gemini Koşu 1). 16 bit uyarısının (H6994) önerisi maddedeki `p.TR_fill(ana)` değil, indeksli yazım (`p[0..p.TR_shape[0]] = ana`): `.TR_fill` yok (15.6d).
- **Yapılacaklar (D, Ö3; DENETİM istek 83 Ş4):** `w.TR_requires_grad = 1` denetimde değil çalışmada hata (H6992); denetleyici tensör üyesinin tipini bilince yakalar.
- **Yapılacaklar (E, Ö3; T1 Kol B teslim 2b):** kitaplığın `TR_Muon` ve `TR_NorMuon`'u 16 bitlik parametrede ana kopya tutmuyor; güncelleme yutuluyor ve artık uyarı veriyor (`lib/optimizers.tr:96-97`) [kanıtlı: T1 devri]. İş: iki iyileştiriciye ana kopya.
- **Yapılacaklar (E, Ö3; T1 Kol B teslim 2b):** H6994'ün önerisi `.TR_fill` yokken `p[0..p.TR_shape[0]] = ana`; `.TR_fill` gelince `p.TR_fill(ana)` olur (15.6d).
- **Yapılacaklar (F1, Ö3; DENETİM istek 84 Ş4):** H6994'ün önerisi (`p[0..p.TR_shape[0]] = ana`) 0 boyutlu parametrede "Liste indeksi sınır dışı" veriyor; öneri boyuttan bağımsız yazılmalı. Kitaplığın `TR_Muon`'unda uyarı `tr/optimizers.tr` satır 97'yi gösterip kullanıcıya ana kopya tutmasını söylüyor; kullanıcı orayı değiştiremez, ileti "parametreyi float32 kurun" demeli.
- **Yapılacaklar (F, Ö3; T1 Kol B teslim 2c):** iyileştiricinin iki iletisi parametrenin sırasını farklı sayıyor: H6920 0'dan, H6995 1'den. İkisi de 1'den saymalı.
- **Yapılacaklar (F, Ö3; DENETİM istek 85 N1):** kitaplığın iyileştiricisinde (`TR_Muon` vb.) H6995 kullanıcının satırını değil `tr/optimizers.tr`'nin 118. satırını gösteriyor.
- **Yapılacaklar (F1, Ö3; DENETİM istek 85 N2):** ara sonuç (yaprak olmayan tensör) kitaplığın `TR_Muon`'unda H6995, Go'daki `TR_SGD`'de H6514 veriyor; iki yol aynı durumu farklı kodla anlatıyor.
- **Yapılacaklar (F, Ö3; DENETİM istek 85 N3):** teslim 2c'de `typed_code_known.txt` kaymasın diye ilgisiz bir yorum tek satıra birleştirildi; liste anahtarı düzelince ("Denetim katmanlarının gözden geçirmesi") geri açılır.
- **Açık (Sonraya):** aynı yapıya eklenen LoRA alanları için `TR_load_parameters`'ın kesin kuralı; bugün yol alt yapıya yüklemek (`TR_load_parameters(model.gövde, ...)`).

### 15.36

- **Gemini Koşu 3 (2026-10-29), uygulanacak:** kalıp kaybı da sınar; belgelerdeki ve örneklerdeki eğitim döngüleri buna göre.

### 15.37

- **Mevcut durum:** `TR_default_device(" CPU ")` kabul ediliyor [ölçüldü]; `.TR_to_device(" CPU ")` de kabul ediliyor ve TR testi bunu sabitliyor. Desteklenen GPU yalnız NVIDIA.
- **Yapılacaklar:** tek yazım (büyük harf ve boşluk hata); TR testi. Başka hızlandırıcılar (Apple, gömülü NPU) yol haritasında.

### 15.38

- **Gemini 15/2 (2026-10-29), uygulanacak:** `TR_load_tensors` varsayılan cihazda kurar (T3).

### 15.41

- **Mevcut durum:** GPU'daki matematik işlevlerinin (`ex2.approx`, `lg2.approx` yolları) ulp sınırı ölçülmedi; 15.11 GPU için "birkaç ulp" diyor.
- **Yapılacaklar:** GPU sınırlarını ölçüp 15.10 ve 15.11'e yazmak; cihazlar arası sözün Go testi.
- **Yapılacaklar (D, Ö3; sahip TCB, dil tanımı turu; DENETİM istek 121 N1):** AVX2 ile genel yolun model logiti varsayılan toleransı aşıyor, metin aynı, tabanda da böyle [ölçüldü: H1]. Karar `pending.md`'de.

### 15.42

- **Mevcut durum:** sıfır eksenli (verisiz) şekil, sıfır eksenleri 1 sayarak 50 milyon öğelik koleksiyon sınırına göre denetleniyor: `TR_zeros`, `TR_ones`, `TR_randn` ve boş tensörün `.TR_reshape`'i `TR_zeros([0, 1000000000, 1000000000])`'u reddediyor; veri taşıyan tensörün sınırı ise 2³² öğe (`TR_zeros([60000000])` başarılı, `TR_zeros([100000, 100000])` "at most 4294967296 elements") [ölçüldü: main `85a19c36`, çalıştırma; `internal/lang/interp/builtins.go:287`]. GPU çekirdeklerinin indeksi 32 bit: bir uygulama sınırıdır, dil kuralı değildir; Llama 3 405B'nin gömmesi (~2,1·10⁹ öğe) bu sınıra dayanıyor.
- **Yapılacaklar:** 50 milyon sınırı (sıfır eksenli şekillerde; veri taşıyanda 2³² sınırı da kalkar) [kanıtlı: internal/stdlib/ml/error.go:188-209, `internal/lang/interp/builtins.go:287`] kalkar, yerine int64 taşma denetimi (yukarıdaki örnek artık hata değildir; dev sonuç ayrılırken yakalanır); 64 bitlik GPU indeksi yol haritasında.

### 15.43

- **DENETİM 0.6b (2026-10-29):** `.TR_append` ve indeksle atamada yalnız yazılan değer kaçıyor (kurala uygun; testli: `TestOwnershipListLiteralOwnsElementsLikeATuple`, `TestOwnershipIndexAssignmentEscapesOnlyTheWrittenValue`); öbür değiştiren yollar (ör. `+=`) bütün listeyi ve öğelerini kaçırıyor (`internal/lang/interp/assign.go:75-78`): güvenli taraf ama sapma, testi yok. Test satırına 0.6b'nin iki sahiplik testi girecek.

### 15.44

- **Yapılacaklar:** GPU'da aynı hata (denetim cihazdan önce, stdlib'de): L4'te koşulacak.
- **Yapılacaklar (F, Ö3; DENETİM istek 83 Ş3):** modül adı ve metot yolu `VM.EvalExpression` ile okunuyor; `debug_support.go`'nun yorumu bu yöntemleri yalnız hata ayıklayıcı için tanımlıyor. Yorum ikinci kullanıcıyı söyler ya da dar bir üye okuma fonksiyonu açılır.
- **Yapılacaklar (E, Ö3; T1 Kol B teslim 2b):** liste kavrayışının değişkeni `TR_fuse`'un toplayıcısında yerel sayılmıyor; dışta aynı adlı değişken varsa doğru program gereksiz H6986 ile duruyor (sessiz değil) [kanıtlı: T1 devri, okuma].
- **Yapılacaklar (F1, Ö3; DENETİM istek 84 Ş3):** H6986 fonksiyon değerinde yanlış yönlendiriyor: öneri "değişen sayıyı argüman verin" diyor, oysa fonksiyon `TR_fuse`'a argüman olamaz; doğrusu `TR_fuse(...)`'i yeniden kurmak (öneri okunan değerin türüne göre ayrılır). Fabrika kapanışında eski ve yeni fonksiyon aynı görünüyor (`<ok fonksiyonu>`). Takma adla okunan bağlı metodun yolu iletide sınıflı iç kimlikle (self anahtar sözcüğünün iç kodu) yazılıyor; nesnenin yolu olmalı.
- **Yapılacaklar (C, Ö3; DENETİM istek 84 N1) [hipotez]:** fonksiyon kimliği `%p` ile metin olarak tutuluyor; eski kapanış toplanıp yenisinin ortamı aynı adrese düşerse değişim görünmez. Yeniden üretilemedi (400 denemede 400 H6986). Koruma: kaydedilen gövde ve ortam işaretçilerini `fuseCapture`'da tutmak.
- **Uygulama sınırı:** `TR_clamp`, `TR_atan2` ve `TR_where` bugün kaynaştırılmıyor; `f`'de kullanılınca hata veriyor. Kural onları da kapsar.
- **Gemini 15/2 (2026-10-29), uygulanacak:** kapanış yolunda liste indeksi ve sözlük anahtarı (2. kilometre taşının ön koşulu); `TR_fuse`'da float64 ve tam sayı kendi türünde (T3).

### 15.45

- **Mevcut durum:** kayıtta `TR_embedding_lookup`'ın tensör indeksi ve `TR_write_slice`'ın tensör başlangıcı sınır dışındaysa sessizce kırpılıyor (gömmede `[0, V−1]`, `TR_write_slice`'ta hedefe sığan son konum), kesirli değer budanıyor [kanıtlı: bugünkü 15.45'in metni]. Kapanıştan okunan sayılar kayıtta sabitleniyor, denetlenmiyor [ölçüldü: DENETİM].
- **Yapılacaklar (A, Ö1, Kol B, 15.6'nın cihaz içi bayrağıyla; ölçüm status'un):** kırpma kalkar, cihaz içi hata bayrağı (15.6); kapanıştaki sayının ve tensör deposunun yoluyla denetimi; rastgele üretecin oynatmada ilerlemesi; Go testleri.

### 15.46

- **Mevcut durum:** türevi izlenen float64 tensörün H6040'ı liste ve demet içinde dört kata kadar sınandı; daha derindeki float64'ün yolu ölçülmedi [hipotez].
- **Yapılacaklar (E, Ö3; sahip T1, sonraki dilim; T1 devri):** float64 türev yolu (15.32'nin float64 girdisi, `15_tensorler.md:54`'ün türevlenebilir `.TR_to_dtype`'ı, iyileştirici): kural var, motorda yok, açık hatayla (H6040). Yol açılınca H6040'ın girişleri (`gradFree`, `graphMembers`) aynı yerde kalkar.
- **Mevcut durum:** dosyada veri taşıyan tensör en çok 2³² öğe, verisiz şekil (sıfır eksenler 1 sayılarak) en çok 50 milyon öğe bildirebiliyor [kanıtlı: bugünkü 15.46'nın metni]; 15.46'nın sıfır uzunluklu eksen TR testi (`tests/tensor_test.tr:560`) `[1048576, 0, 8192]` şekilli verisiz tensörün reddini bekliyor. Kontrol noktası (20) cinsli tensör yazmıyor.
- **Yapılacaklar:** `TR_AnyTensor` ve `.TR_narrow[E]()`; nicemlenmiş saklama hassasiyetlerinin tür kodları (15.55); 2³² ve 50 milyon sınırları kalkar, öğe sayısı int64 taşma denetimiyle ve ayırma 15.42'nin bellek denetimiyle (sıfır eksen testi yeni kurala göre yeniden yazılır); TR testi.
- **Gemini 15/2 (2026-10-29), uygulanacak:** `TR_requires_grad: TR_true` yalnız nicemlenmemiş `TR_Float` tensörlerde (T3).

### 15.47

- **Mevcut durum:** U64 ve F8 türleri H6643 [ölçüldü: Go `TestSafetensorsRejects`].
- **Yapılacaklar:** `F8_E4M3`'ün ölçeği 1 "fp8" olarak okunması (15.55).

### 15.49

- **Mevcut durum:** tensörün `==`'si ve `!=`'si eşitlik kancasından gidiyor: iki yan birden genişlediğinde yayın uyarısı `<` için yazılıyor, `==` için yazılmıyor (T1 bulgu 7) [ölçüldü: DENETİM istek 57, `[2,1] < [1,3]` ve `==`].
- **Yapılacaklar:** `==`/`!=`'de yayın uyarısı; satırların kalanı kendi maddeleriyle; TR testi.

### 15.50

- **Mevcut durum:** float32'nin ucundaki büyük sonlu sayılarda CPU float64'te biriktirdiği için sonlu, GPU float32'de biriktirdiği için sonsuz sonuç verebiliyor (normlarda x², toplamın kendisi 3,4e38'i aşınca).
- **Yapılacaklar (C, Ö2; ölçüm status'un):** sayısal doğruluk kapısı (eski yol haritasının Evre 4, madde 8; bugünkü durumu ölçülmedi): her türevli işlemin CPU ve GPU sonucunun float64 referansa karşı testi ve işlem başına hata sınırı (normlar ve toplamlar için var: `TestNormsMatchFloat64Reference`, `TestSumsMatchFloat64Reference`), GPU bf16 sapma tablosu, `TR_log10` ve `TR_atan2` türevinde alt normal yerine 0, çapraz entropinin çok büyük logitte lse − x ile hassasiyet kaybı ((m − x) + log Σ sırası düzeltir); `TR_logsumexp`, `TR_maximum`/`TR_minimum` satırları ve `TR_max`/`TR_min`'in sıralama sırası (15.22b); TR testi.

### 15.51

- **Mevcut durum:** "q4", "q4z128", "fp8" yok: ad bilinmeyen hassasiyet hatası verir.
- **Yapılacaklar:** "q4", "q4z128", "fp8"; TR testi. Başka grup boyları, `Q4_1`, NF4 ve 2–3 bit Sonraya.

### 15.52

- **Mevcut durum:** nicemleme yok: `.TR_to_dtype("q8")` ve değerden kurma H6691. Hugging Face'in `weight_scale_inv`'inin çarpılan ölçek olduğu bir hipotezdir [hipotez: DENETİM F2].
- **Yapılacaklar:** `.TR_to_dtype` ile nicemleme (llama.cpp ile bit bit, CPU ve GPU'da aynı kodlar); yeni kimlikler `TR_quantized` (gösterim adı "nicemlenmiş") ve `TR_zero_point` ("sıfır_noktası"), onaylı; NaN ve aralık denetimleri 15.6'nın bekleyen işlemlerinde; `weight_scale_inv`'in anlamını gerçek bir FP8 modeliyle doğrulamak; TR testi. Sıra değiştiren gruplar (GPTQ `desc_act`, `g_idx`) Sonraya.
- **Gemini 15/2 (2026-10-29), uygulanacak:** q4'ün sıfır bloğu −0.0 ölçek ve 8 kodla llama.cpp ile bit bit (baytlarla test); q4z128'in RTN formülü; fp8'in kısmi son bloğu (T3). **Açık karar:** GPTQ/AWQ ölçeği bf16 kalsın mı (dosya biçimi).
- **Yapılacaklar (A, Ö1; sahip H1, nicemleme uygulanırken; DENETİM istek 118 B1):** fp16 ölçeği sonsuza giden blok nicemleme hatası: yazılmazsa öğeler sonlu olsa da açılan değer NaN olur ("q8" `amax` 8 321 040'tan, "q4" 524 160'tan başlar) [ölçüldü: DENETİM, llama.cpp'nin kodu]. Bugün kimse etkilenmiyor, 15.52 uygulanmadı. `1 / d` sonsuzsa sıfır blok gibi kodlama da aynı dilimde (D, Ö3).

### 15.53

- **Mevcut durum:** türev izleyen girdi H6696 (geri yön `g @ w` yok); `x @ w` H6695; `TR_linear_cross_entropy` H6687. ARM'de NEON çekirdeği yok, Go eşi koşuyor.
- **Yapılacaklar (D, Ö3; sahip H1, sıradaki hız dilimi; DENETİM 236):** zayıf havuz `BF16ElementChain`'i %12,6 yavaşlatıyor, kopya %0,4, çeyrekler ayrık [ölçüldü: `h1k3b`]; yazılı %20 kapısının altında, kabul edildi. Sebep [hipotez]: 1–2 MiB depolar zayıf iade ediliyor. `poolWeakBytes` profil ve tepe birlikte ölçülmeden değişmez.
- **Yapılacaklar (D, Ö3; sahip H1, hız kuyruğu):** llama.cpp'ye kalan bellek, Qwen3-4B `Q4_K_M` 2048 konumda ileri +923 MiB, üretim +1 453 MiB [ölçüldü: `h1k3c`, tek tur, kıyas tam eş değil]: ara tensör sahipliği ve 16 bit KV; ayrımı ölçülmedi.
- **Yapılacaklar (D, Ö2; sahip H1; H1 teslimi, DENETİM istek 160 N1):** kartta prompt llama.cpp'nin 0,395 ve 0,233 katı; üretim konum ilerledikçe yavaşlıyor (403 → 308 parça/sn) [ölçüldü: H1, `h1g3`]. Önce `TR_replay` içinde çağrı tabanı: kalıyorsa 4B'de parçanın %25'i [hesap: DENETİM 160]. İlk prototipin hedefi üretimde ve prompt'ta 0,5 (Ahmet, 2026-11-05): üretim 100 parçalık pencerede geçti (0,515 ve 0,626), prompt kalıyor.
- **Yapılacaklar (F, Ö3; sahip H1; DENETİM istek 160 N2):** 15.55'in tür kodları gelince `CheckQuantScales` `fileEntry.tensor`'a da gitsin: yoksa NaN ölçek ikinci yoldan dile açılır; kartta 16 bitlik açma ve gömme ile ARM CPU'dan ayrılır [ölçüldü: DENETİM, benzetici].
- **Yapılacaklar:** ARM NEON çekirdekleri; geri yönün ikinci çekirdeği; `TR_linear_cross_entropy`; 2. kilometre taşının hız ölçütü. Nicemleme farkında eğitim (QAT) Sonraya.
- **Gemini 15/2 (2026-10-29), uygulanacak:** devrik görünümün izinli kullanımları; fp8'in ilk eksen kuralı (T3).

### 15.54

- **Yapılacaklar:** yüklemenin (20.6, 20.11, 20.14) aynı belleğe bütün olarak yazması; TR testi.

### 15.55

- **Mevcut durum:** ağırlığı kopyalamadan bağlamak yapılmadı: `TR_load_parameters` kopyalar, bağlamak ya yeni ad ya davranış değişikliği ister. Safetensors'ta `F8_E4M3` okunmuyor [kanıtlı: bugünkü 15.47'nin metni]. Llama 3.1 ölçülmedi.
- **Yapılacaklar:** TR biçiminde nicemlenmiş tür kodları: bugün iki biçime de yazmak H6687, ileti açmayı söyler; kod gelince `.safetensors` iletisi TR biçimini de önerir; GGUF `Q4_0`'ı ("q4") H6674; `F8_E4M3` okuma; kitaplığın (`tr/pretrained`) GPTQ, AWQ ve FP8 parçalarını birleştirmesi; TR testi.
- **Yapılacaklar (C, Ö3; sahip H1; DENETİM istek 161):** F32 GGUF'ta tepe dosyanın 3,5 katı (4,95 GB dosya, kısa prompt 17,3 GB) [ölçüldü: T1, `t1l4`]; F32 yolunda `readValues` ve `FromHost` iki kopya kuruyor [kanıtlı: `load_gguf.go`, `cpu_memory.go:128`], ikisi aynı anda canlı olabilir [hipotez: DENETİM 161]. Safetensors F32'de de. Çözüm önerisi sahipliği devralan kurma.
- **Yapılacaklar (E, Ö3; sahip T1; DENETİM istek 161):** `TR_pretrained_model` Llama 3 safetensors'unda `rope_scaling` H6943 veriyor [kanıtlı: `lib/pretrained.tr:67-68`]; GGUF çalışıyor.
- **Yapılacaklar (F, Ö3; sahip T1, önce ölç; DENETİM istek 161):** GGUF yüklemesinde tablo iki kez kuruluyor; süresi ölçülmedi. Repoda uçtan uca dış kanıt yok: küçük fikstürün llama.cpp logitleri altın dosya olabilir.
- **Sınır (A, sınır; T1, 2026-11-05; DENETİM istek 149 Ş1, 161):** llama.cpp de aynı; tensörü yazılmadan dönüştürülmüş eski Llama 3.1 GGUF'u tanınamaz: üstveriden "tensör olmalıydı" bilinmez, bölen 1 kurulur, uzun bağlamda sessiz yanlış olur [hipotez].
- **Yapılacaklar (D, Ö3; sahip H1, hız kuyruğu; H1 CPU kıyası):** prompt işlemede TR transformers'ın 0,50–0,54 katı, 16 bit float32'den hızlı değil (0,94 ve 0,96) [ölçüldü: H1, AMD EPYC]. Çok satırlı çarpımda ağırlık hâlâ float32 tampona açılıyor [kanıtlı: `Gemm`]; sebep bu mu [hipotez: profil çıkmadı].
- **Yapılacaklar (D, Ö2; sahip H1; H2 devri):** eşik modeli Qwen3-0.6B `Q8_0`'da 0,5 eşiği geçilmedi: `04914ce6`'da 0,477 [ölçüldü: `h1k2e`, yalnız Xeon; başka işlemcide ölçülmedi]; önce üç Xeon koşusunda 0,460, 0,484, 0,487; eşiğe parça başına 1,2–3,8 ms kaldı [hesap: `h3k7`, `h3k6`, DENETİM 180]. Qwen3-0.6B `Q8_0`'da parça 48,6 ms, llama.cpp'nin bütün parçası 23,7 ms; çarpım 36,7 ms, çarpım dışı yaklaşık 12 ms (önce 29,6) [ölçüldü: H2, `h3k7`; çarpım dışı hesap]. `sdotq8` tek iş parçacığında hesap sınırlı (6,3 GB/sn, düz okuma 9,8), 8 iş parçacığında okumanın %82'si (29,4 ve 35,8) [ölçüldü: H2, `h3k4`]. Kaynaştırılmış işlerin dağıtımı prompt'u +%20 verdi, üretimi değiştirmedi; ürünler arası önden getirme kazançsız, çıktı [ölçüldü: `h1k2e`, `h1k2a`]. İş parçacığı ayarı ve yerleşim TR'nin hızını değiştirmedi: 4 ve 8, serbest ve sabitli ortanca 21,93–23,25 parça/sn; llama.cpp 4 iş parçacığında 37,08–38,08, 8'de ortanca 43–44 ama tekrarların dördü 21–23'te kaldı, yani 0,477'nin paydası iki kümeli; büyük sayfa sınanamadı [ölçüldü: `h1k2g`]. Bu kıyas katman çarpımlarında 8 katılımcıyı sınamadı: 2^22 birimin altındaki çarpım 8 iş parçacığında da en çok 4 katılımcıyla koşuyor [kanıtlı: `parallel.go:107`, `:322`]; 0.6B'nin en büyük katman çarpımı 3 145 728 birim, çıkış başı 155 582 464 birimle eşiğin üstünde [hesap: DENETİM 215]. Kalan, yönergeye yazılmadan koşulmaz: taze sayfa, katmanlarda 8 katılımcı; etkinleştirme nicemleme varsayılan yol değildir (15.53); deneme aşağıdaki satırda.
- **Yapılacaklar (E, Ö3; sahip H1, bit değiştirmeyen işlerden sonra; Ahmet 2026-11-06):** etkinleştirme nicemleme denemesi. 15.53 bugünkü davranışıyla kalır. DENETİM 181'in gerekçesi yanlıştı: kullanıcının seçtiği yaklaşıklık dilin doğruluğuna dokunmaz, `Q4_K` gibi; kalma sebebi sadelik ve doğrulanabilirlik. Önce `Q4_K`'da tek çekirdek, hız kazancı ve kalite bedeli ölçülür; anlamlıysa 15.53'e açık seçim gelir (varsayılan kapalı, belirlenimci), değilse satır kapanır. Llama 3'te nicemli dosyalarla llama.cpp'nin F32'den ayrışması llama.cpp'nin kendi iki yolu arasında, TR'de nicemli ve F32 logitleri aynı [ölçüldü: DENETİM 161, 12 çift]; ayrışma kalite değildir.

## 16. Veri ve parçalayıcı

Kurallar: [`docs/spec/16_veri_parcalayici.md`](spec/16_veri_parcalayici.md).

### 16.2


- **Bulgu (T1, 2026-11-03, D, Ö3):** `splitFn` aynı konumda atlanan uzun bir eklenmiş parçanın içindeki kısa eklenmiş parçaya bakmıyor; yalnız `DontParseSpecial`'ın (HF anlamı) çağıranını etkiler, TR'nin `TR_false`'u (`DontParseAdded`) hepsini atladığı için etkilemez; bugünkü fikstürlerde sonuç aynı [ölçüldü: T1, altın testi].

- **Yapılacaklar (B, Ö1; sahip parçalayıcı dilimi; Tur 5, DENETİM 352):** 16.2'nin Tur 5 hâli motorda yok: `TR_boundary_tokens:` argümanı, verilmeyen `TR_special_tokens:`'ın metin içi yazımda hatası ve çift sınır parçasının hatası. Göç sessiz anlam değişikliğidir: bugünkü `TR_special_tokens: TR_false` sınır parçalarını da kapatıyor, yeni kuralda kapatmaz; her böyle çağrıya `TR_boundary_tokens: TR_false` mekanik olarak eklenir, sonra BOS istediği bilinen yerler tek tek açılır (`3220c1e9`'da 8 `.tr` dosyasında 36 geçiş [ölçüldü: DENETİM 352 kapanışı]; taban sayımı ve üç TR koşusunun karşılaştırması). İkinci göç: metin içi etiketi varsayılana dayanarak kodlayan çağrılar `TR_special_tokens: TR_true` yazar; yoksa açık hata alır (düz metinle en az 6 yer, `tests/veri_parcalayici_test.tr` [ölçüldü: DENETİM 353]). `codes/tr.json`'un `TR_special_tokens` açıklaması 16.2 ve 16.3'ün iki imzasını ayrı anlatır; `docs/guide/04_yapay_zeka_ve_veri.md`'nin 16.2 anlatımı ("Varsayılan `TR_true`") göçle değişir. `TR_Bool?` argümanın `TR_none` varsayılanı 13.9'da yeni özellik istemiyor [hipotez: DENETİM 352 §4; uygulamada ölçülür].

### 16.3

- **Yapılacaklar (D, Ö3; sahip parçalayıcı dilimi; Tur 5, DENETİM 352):** 16.1 artık aynı içerik ve kimlikle tekrar yazılmış, bayrakları farklı kayıtları ve normalleşince çakışan `normalized` parçaları yükleme hatası sayıyor; motor son yazımın bayraklarını alıyor, çakışmada dosya sırası kazanıyor. Gerçek model dosyalarında böyle kayıt yok [Gemini'nin sözü, Tur 5].
- **Bulgu (T1, 2026-11-04; sahip TCB, C, Ö2, dil tanımı turu: 15.42'nin toplam sınırı; sınıf DENETİM istek 105):** kimliği 1.000.000.000 olan iki parçalı dosyada `p.TR_vocab_size` 1000000001. `TR_GPT(p.TR_vocab_size, ...)` gömme boyu 8 ve 64'te şekil sınırı hatası veriyor (0,1 sn). Gömme boyu 4'te sınırın altında kalıyor: 50 GB'lik makinede 49,5 sn'de 49,16 GB, süreç işletim sistemince öldürüldü, TR hatası yok [ölçüldü: Colab CPU High-RAM]. Tensör başına 16 GB, 15.42'nin sınırının altında; aynı anda yaşayan tensörlerin toplamı denetlenmez, yani çelişki değil.

### 16.3a

- **Mevcut durum:** uygulanmadı; akışlı çözme yok, `p.TR_decode` yarım karakterde U+FFFD veriyor [ölçüldü: 2026-11-02, main `705b721b`].
- **Yapılacaklar (E, Ö3; sahip T1; planda DOST'un cümle cümle konuşmasından sonra; asistan mimari turu, 2026-11-09, DENETİM 324 ve 327):** DOST'un cevabı cümle cümle konuşması akışlı çözmeyi bekliyor; uygulanana kadar zincir metni her adımda baştan çözer (doğru ama ajanın yazacağı kalıp değil). `TR_DecodeStream`, `.TR_step`, `.TR_finish`; birleşimin toplu çözümle aynı olduğunu sınayan TR testi (bayt düzeyi ve baştaki boşluklu çözücüde). Dağarcıkta olmayan kimlik akışta da 16.3'teki gibi hata: `.TR_decode`'un denetimi akışa da girer [kanıtlı: T1 16.3 devri].

### 16.4

- **Mevcut durum:** parçalayıcı eğitmek yok; bugün Python'a gidiliyor.
- **Yapılacaklar:** bayt düzeyinde BPE eğitimi, belirlenimci; `.TR_save` ile `tokenizer.json`; TR testi.
- **Gemini Koşu 7 (2026-10-29), uygulanacak:** dosya listesiyle eğitim; bayt düzeyi ön ayırıcı ve çözücü, eşitlik kuralı (Hugging Face ile bir kez sınanacak); özel parça eklemez.

### 16.5

- **Yapılacaklar (dil tanımı turu 1, 2026-11-01):** okuyucu `int64`'e geçer; `int32`'ye sığmayan `<u4` ve `<i8` değerini reddetmek (H5086) dil tanımına aykırı: 16.5 yalnız tür kümesini söyler, 16.6 2³²'ye kadar işaretsiz kimliğe izin verir, 16.7 `"int64"` tensör üretir [kanıtlı: Gemini `TUR1_RAPORU.md` satır 27, DENETİM `kol-denetim-tur1-rapor-sonuc.md`]. Bugün `internal/compute/dataload/shard.go:282` (`decodeI32`, 256) reddediyor; `TestShardU32Overflow`'un `<i8` satırları ve `tests/veri_parcalayici_test.tr`'nin sınırı ters döner. S1'in işi.
- **Mevcut durum (C, Ö2; DENETİM istek 113 Z1):** veri kümesi önbelleğinde işçinin okuması takılırsa aynı klasörün açılışı süresiz bekliyor, ileti vermiyor, ilk Ctrl+C kesmiyor [ölçüldü: DENETİM istek 113 Z1]: yavaş sürücü, kapanış ve aynı klasörün yeniden açılışı birlikte gerekiyor. Süreli bekleme somut ihtiyaç çıkınca.

### 16.6

- **Mevcut durum:** veriyi yazan yerleşik yok; Go tarafında işaretsiz 16 bit ve işaretli 32 bit yazıcı var (`WriteNpyU16`, `WriteNpyI32`) [kanıtlı: `internal/compute/dataload/npy.go:512,525`].
- **Yapılacaklar (A adayı, diskteki veri; sessiz mi ölçülmedi):** `TR_write_dataset`, tür seçimi ve `meta.json`; TR testi.
- **Gemini Koşu 7 (2026-10-29), uygulanacak:** var olan kümenin yerine atomik yazma.

### 16.7

- **Mevcut durum:** `.TR_next()` iki öğeli liste veriyor (girdi ve hedef) [ölçüldü: Gemini 4].
- **Yapılacaklar:** demet dönüşü (3.6a); göç: `.TR_next()`'i `[0]`/`[1]` ile okuyan yerler.

### 16.8

- **Mevcut durum:** sıralı okuyucu döngüyle geziliyor, liste veriyor [ölçüldü].
- **Yapılacaklar (D, Ö3; sahip D1; 3.6a'nın göçüyle; DENETİM 231):** `TR_topk`, `TR_Sampler` ve `TR_SequentialReader` liste veriyor, kural demet; değerler doğru. İki adlı `TR_for` istenmez, kural reddediyor (16.8).

## 17. Kodlar

Kurallar: [`docs/spec/17_kodlar.md`](spec/17_kodlar.md). Taslak (2026-10-29): ana oturum yeniden yazdı (Ahmet: "17 şimdi düzeltsen ? sürekli kafanı karıştıracak"); 2. turda madde madde CLAUDE.md'den yeniden tartıldı (görünüm yalnız okunur); 3. turda DENETİM'in 1. turunun görünümden bağımsız bulguları, 4. turda 17-2'nin bulguları alındı (`0f1d6029`); kararlar Gemini Koşu 6'dan geçti (DENETİM doğruladı, `c784c47a`). Kapanış turu yapılmadı. 4a'dan önce gelir, çünkü 4a kod kaydına yeni kodlar ekler.

### 17.1

- **Mevcut durum (2026-11-05; B, Ö0 kapandı; DENETİM istek 155 Ş1, 158):** kullanıcının dosyasında `TR_` ile başlayan ad hiçbir tanımda yazılmıyor, geçerli kimlik de olsa: H0004. Kapı `source.Prepare`'de, ayrıştırmadan sonra; modül, kabuk ve hata ayıklayıcı da ondan geçiyor. Yapıda yalnız `TR_init` ve `TR_next` metodu serbest. Kitaplık metniyle ayrılıyor (`interp.LibraryFileName`), yolundan değil [test: `TestReservedNamesStopEveryDefinition`, `TestAModuleCannotReplaceABuiltin`, `tests/negative/kod_tanimi.tr`, `tests/negative/kod_uye.tr`].
- **Yapılacaklar:** kullanıcı yapısının `TR_next` ile dolaşılması 5.7'nin satırında (E, Ö3).

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

- **Mevcut durum (D1, birleşik 59):** değerin metni dilden bağımsız ("true", "none", "List", "<builtin print>"); kitaplık tipinin metni kimliğin adı ("Linear", "Role", "ChatMessage", "RMSNorm"), kullanıcı yapısınınki kendi adı [test: TR testi "3.14 kitaplığın tipinin metni kimliğinin adıdır, kullanıcının yapısınınki adıdır"; ölçüldü: D1].
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

- **Gemini Koşu 5 (2026-10-29), motor zaten böyle:** kök ve UNC öneki korunuyor [ölçüldü: DENETİM].
- **Mevcut durum (2026-11-06; B, Ö0 kapandı; DENETİM 219, 229, 237):** ilk parçadan sonrakilerin birleşip temizlenmiş hali ilk parçanın dışına çıkamaz, H7510; mutlak parça denetimi de bu biçime iniyor (H7139) [test: `tests/dosya_ve_yol_test.tr`, `internal/stdlib/files/path_test.go`].
- **Yapılacaklar (B, Ö1, asistan araçları dilimi; sahip TCB tasarım, D1 uygulama; DENETİM 229 N1, N2, 230):** `TR_Path`'in denetiminden geçmeyen yollar: tek parçada birleştirilen metin (`TR_Path("veri/\(ad)")`, `TR_Path("veri/" + ad)`, `../x`'te `x` veriyor [ölçüldü: DENETİM 229]), metin yol alan yerleşikler (`TR_write_file("veri/\(ad)", ...)` [kanıtlı: 18.4; koşulmadı]), bağlantı üzerinden çıkış (temizleme metin üstünde [kanıtlı: 18.1; denenmedi]). Kural bunlara izin veriyor; modelin verdiği dosya adı için asistan araçları yazılırken tartılır.
- **Yapılacaklar (D, Ö3; sahip D1; DENETİM 229 N2):** Windows aygıt adı (`NUL`, `CON.txt`) parça olarak hatasız [ölçüldü: DENETİM 229], kural yazmıyor; Unix'te `C:/x` parçası hata vermiyor, 18.1 koşulsuz diyor [kanıtlı: `path.go`; koşulmadı].

### 18.5

- **Mevcut durum:** Windows'ta kalıp harf büyüklüğüne bakmıyor: `"*.TXT"` `a.txt`'yi eşliyor [ölçüldü: main `dc4930ca`].
- **Mevcut durum (D1, birleşik 59):** çalışma klasöründe bulunmayan, programın klasöründe bulunan göreli klasörde `TR_list_dir` mutlak yollar veriyor [test: `TestListDirFoundBesideTheProgramGivesAbsolutePaths`].
- **Yapılacaklar:** her sistemde harf büyüklüğüne duyarlı eşleme (göç: `internal/stdlib/files/list_test.go`'nun harf büyüklüğüne bakmayan dalı), `TR_ignore_case:`; TR testi.

### 18.7

- **Mevcut durum (D1, `82bf29a6`; A ve F1 kapandı):** açık günlüğün dosyası her sistemde silinmez ve taşınmaz, H2723 ve H2724 `g.TR_close()`'u önerir [test: TR testi "18.7 sil ..."]. Taban Linux'ta sessiz kayıptı: `TR_remove` hatasız sildi, sonraki iki kayıt kayboldu [ölçüldü: D1, G4, `ffca754c`]; dal H2723 verdi, dosya 4 satırla durdu [ölçüldü: D1, G4, `7e7706dc`].

### 18.9

- **Mevcut durum:** dolu klasörün yerine konması uygulandı (Gemini Koşu 5): eski klasör geçici ada çekiliyor, yenisi yerine konunca siliniyor, yerine koyma olmazsa eski geri konuyor; `TR_copy`'de `TestCopyDir` sınıyor, `TR_move` (18.10) aynı yolu kullanıyor [ölçüldü: main `85a19c36`, `internal/stdlib/files/copy_run.go:60-86`, dolu klasör üstüne kopyalama ve taşıma].
- **Yapılacaklar (A adayı, diskteki veri; sessiz mi ölçülmedi):** arada kesilince geçici adıyla kalan eski klasörü söyleyen ileti; `TR_move`'un dolu klasörün yerine konmasını sınayan Go testi.

### 18.11

- **Yapılacaklar (A, Ö1; sahip T1; Tur 5, DENETİM 353 Ş2):** 64 MB'ı aşan satırın hatasından (H7105) sonra okuyucu kapanıyor ama kapatılmış sayılmıyor: sonraki `.TR_next()` `TR_none` veriyor, hatayı yakalayan döngü dosyanın bittiğini sanıp kalan satırları sessizce atlıyor (`internal/stdlib/files`, satır okuyucusu: satır sınırını aşınca kapatan yol `TR_close`'un bayrağını koymuyor) [kanıtlı: DENETİM 353]. 18.11'in Tur 5 hâli: sonraki istek 18.15'teki gibi hata. Geçersiz UTF-8'den sonra sıradaki satırla sürmek uygulanmış (13.1'in 2026-11-04 satırı).

### 18.16

- **Mevcut durum (Kol A 13 yerleşikler, 1. adım, 2026-11-04):** değiştiren işler (`TR_write_file`, `TR_save_tensors`, `TR_Checkpoint`'in koşu klasörü, `TR_Logger`'ın dosyası, `TR_Dataset`'in `TR_cache:`'i, `TR_make_dir`, `TR_remove`, `TR_move`'un kaynağı ve hedefi, `TR_copy`'nin hedefi) göreli yolu yalnız çalışma klasörüne göre çözüyor (`ResolveWritePath`); okuyanlar okuma sırasıyla. Silinecek ya da taşınacak dosya çalışma klasöründe yok ama okuma sırası onu başka klasörde buluyorsa H2630: dosyayı ve çalışma klasöründen ona giden açık yolu söylüyor, o yol çalışıyor [test: `TestWritersDoNotTouchTheProgramsFile`, `TestWritersResolveOnlyInTheWorkingDirectory`, `TestRunFolderOnlyInTheWorkingDirectory`, `TestLogFileOnlyInTheWorkingDirectory`, `TestCachePathOnlyInTheWorkingDirectory`; dosya_ve_yol_test "18.16 değiştiren işler…"]. Göç: depodaki örnekler, `tests/` ve `lib/` depo kökünden koşunca eski kuralın başka klasör seçeceği yazma çağrısı 0 (4 çağrının dördü yeni 18.16 testinin); 49 örneğin, 418 testin ve 71 kitaplık testinin çıktısı yeni 18.16 testi dışında aynı [ölçüldü: dal `claude/kolA-13-yerlesikler`, geçici sayım ve önceki/sonraki çıktı, `705c16f1` tabanıyla].
- **Yapılacaklar:** 18.17'nin yazıcıları (`TR_write_lines`, `TR_LineWriter`) `ResolveWritePath`'le; `.TR_exists` okuma sırasıyla bakıyor, `TR_remove` çalışma klasörüne: `TR_if KOŞU.TR_exists { TR_remove(KOŞU) }` başka klasördeki dosyada H2630 veriyor (bakışımsızlık, 18'in kararı; DEVİR'de).
- **Karar (Gemini Koşu 5'in açık kararı, 13.1a ve 18:113, B Ö1):** değiştiren iş yalnız çalışma klasörüne; okuma üç klasöre. Programın yanına yazan program yolu açıkça kurar. Okumanın da tek kurala inip inmeyeceği açık (TCB'nin dil kararı): o karar `TR_exists` ile `TR_remove`'un farklı dosyayı görmesini ve `TR_absolute`'un modülden çağrılınca modülün dosyasını vermesini de çözer (DENETİM istek 91 N1).

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
- **Gemini Koşu 5 (2026-10-29), motor zaten böyle:** büyük harf dizisinin sonunda bölme (`HFToken`) [kanıtlı: DENETİM, `process.go:84-86`]. Çoğullar ve Türkçe çoğullar süzülüyor; süzgeç değere de bakar, sayı değeri gizli sayılmaz [test: `TestIsSecretName`].

### 19.5, 19.7

- **Mevcut durum:** `TR_Logger.TR_append` ve `TR_Progress.TR_update` değer vermiyor [test: `TestNoValueBuiltinsGiveNone`].
- **Mevcut durum (D1, `82bf29a6`):** `g.TR_close()` var (`TR_true`, ikinci kez `TR_false`); kapatılmış günlüğe yazma H2725 [test: TR testi "19.5 g.kapat ..."].
- **Gemini Koşu 5 (2026-10-29), uygulanacak:** başlangıç kaydında `total` yok.

### 19.6

- **Mevcut durum (T1, birleşik 59):** `resume` kaydı `offset` taşıyor (noktadaki günlüğün bayt uzunluğu), hem `TR_rollback`'te hem yeni süreçte sürdürmede; biçim sürümü 2 kaldı [test: TR testleri "19.6 geri dönüşün devam kaydı…", "20.6 sürdürülen…", `TestLogAppendAndResume`].
- **Mevcut durum (T1):** günlüğün tek geçerlilik okuyucusu `testkit.ValidLossMetrics` 19.6'nın `offset` kuralını uyguluyor; `offset`'siz ya da satır ortasına düşen devam kaydında test düşer [test: `TestValidLossMetricsDropsEveryRecordAfterTheOffset`, `TestValidLossMetricsRejectsABadOffset`]. `TR_cmd_watch` ve `TR_cmd_profile` günlüğü okumuyor, adımı `logging.SetStepHook` ile alıyor [kanıtlı: devtools/profiler, profile.go:168, `logging.SetStepHook(p.stepBoundary)`].
- **Gemini Koşu 5 (2026-10-29), uygulanacak:** günlüğün kabul ettiği değerler 13.8'in JSON kümesi (`offset` yarısı birleşik 59'da yapıldı).

### 19.10

- **Mevcut durum:** yardım seçeneği yalnız "--help" ve "-h" (Türkçe "--yardım" bilinmeyen seçenek); "help" ve "h" dışında ad ayrılmış değil; ondalıkta Türkçe "sonsuz" okunmuyor, "inf", "-inf" ve "nan" okunuyor; mantık değerinde Türkçe yazım ("doğru", "yanlış") hâlâ okunuyor; yardım varsayılanları kişinin dilinde gösteriyor ("varsayılan: yanlış"); kullanım hatası iletisi gizli adlı seçeneğin değerini uzunluğuna bakmadan iletiye hiç koymuyor (`TestCommandLineSecretLeaks`, `internal/stdlib/process/arguments.go` `errorValue`) [ölçüldü: `TestCommandLineSecretLeaks`, `a93c9e53`; main `85a19c36`, "--help", "--yardım", "--hızlı=doğru", "--üst=sonsuz" ile çalıştırma].
- **Yapılacaklar:** mantık değerinde yalnız "true" ve "false" (`internal/stdlib/process/process_test.go`'nun Türkçe mantık değeri çıkar); yardımın varsayılanları komut satırının yazımıyla; Go testleri.
- **Göç:** yardımın Türkçe adının göçü bitti (depoda yalnız ipucu testi ve iki yorum kaldı) [ölçüldü: main `85a19c36`, `git grep`].
- **Gemini Koşu 5 (2026-10-29), uygulanacak:** yardımda `TR_none` varsayılanı yazılmaz (bugün "(default: none)"). **Açık:** standart hataya yazan yerleşik yok (yol haritası).

### 19.11

- **Mevcut durum (Faz 0.4, 2026-10-30):** iş klasörünün ortam değişkeni "TR_JOB_DIR" (`TR_env_JOB_DIR`); çubuğun dili "TR_LANG"'dan (`TR_env_LANG`) okunuyor [ölçüldü: kod].

## Bölümü olmayan işler

- **Yapılacaklar (test, Ö3; sahip TCB; DENETİM 251 N2):** `TestLanguageSpecTestsPass` her koşuda 284 geçici klasör bırakıyor [ölçüldü: DENETİM 251].
- **Yapılacaklar (E, Ö3; sahip D1; ajan denemesi 3):** dil tanımının adları motordan önde: testin istisna listesinde 50 onaylı yeni ad bekliyor [ölçüldü: `behavior_language_spec_test.go`; `.TR_lines()` ve `.TR_index_of` birleşik 50'de yapıldı], yazınca sözdizimi hatası geliyor; bazılarının maddesi *Test:* satırında "TR testi" diyor, 13.2 gibi. Dil tanımını okuyan ajan bunları var sanıyor: üç ajan buna çarptı [ölçüldü: `docs/agent_trials.md`]. 19.2 de yapı diyor, motor sözlük istiyor; denetim geçiriyor, çalışma anı düşüyor. Ajanın göreceği yerde "henüz yok" yazılsın; maddenin *Test:* satırı doğrulansın.
- **Yapılacaklar (F1, Ö3; sahip D1; DENETİM 256 N1, N2):** yeri değişmiş önerisi derlenmeyen yazım veriyor: sözlükte `.TR_index_of`, sayıda `.TR_contains`. Listede işlev alan JavaScript araması yanlış ipucu alıyor, Python'un dizin araması ipucu almıyor. Sıradaki adım: öneri tablosu.
- **Yapılacaklar (C, Ö2; sahip S1; CI `9126e27f`):** `TestEscapedGrandchildDoesNotBlockTheStop` yarış dedektörlü Linux işinde 15,32 saniyede düştü [ölçüldü: CI]. Sıradaki adım: süreye mi bağlı, gerçek hata mı, ayrılır.
- **Yapılacaklar (F, Ö3; sahip D1; D1 ipucu devri):** hatalı satırda kullanıcının kendi adı da ipucu alıyor: dosyada tanımlı ad ipucu almamalı. Döngü başında yalın in sözcüğü için `TR_in` ipucu aynı dilime girer.
- **Yapılacaklar (D, Ö3; sahip D1; ajan denemesi 7):** modüle özel ad denetimden geçiyor, çalışırken hata veriyor (11.7) [ölçüldü: `TR_import` ile `_kur()` ve `m._kur()`]. Sıradaki adım: denetim hatası, 4a5'e bakılır.
- **Yapılacaklar (F1, Ö3; sahip D1; DENETİM 258 N1):** Türkçe adlar genel olarak ipucu almıyor, biri yanlış alıyor: Türkçe yok sözcüğü yazılınca ileti `TR_Path` öneriyor, doğrusu `TR_none` [ölçüldü]. Sıradaki adım: öneri her dilin görünüm adına bakar.
- **Yapılacaklar (F, Ö3; sahip D1; ajan denemesi 7):** yaygın yardım bayrağı çıkış kodu 2 ile hata veriyor, ileti doğrusunu söylüyor, ama ajanın ilk denediği yazım o [ölçüldü]. Sıradaki adım: yardım gibi davranır.
- **Yapılacaklar (D, Ö3; sahip D1; DENETİM 256 N3):** `T?` aritmetikte denetlenmiyor: `.TR_index_of` sonucuna 1 eklenen program denetimden geçiyor, çalışınca H2469 veriyor. Sıradaki adım: 4a5'in kapsamına bakılır.
- **Yapılacaklar (F, Ö3; sahip D1; ajan denemesi 7):** biçim denetimi nedeni söylemiyor: son satır sonu yoksa ya da CRLF ise yalnız "biçimlenmeli" diyor [ölçüldü]; ajan sebebi tahmin ediyor. Sıradaki adım: ilk farklı satır iletiye girer.
- **Yapılacaklar (D, Ö3; sahip D1; `AGENTS.md` denemesi):** kitaplığın `.TR_loss`'u parametre tipi yazmıyor, hedef olarak liste verilince denetim değil çalışma anı yakalıyor; `lib/README.md`'nin örneği bu yüzden koşmuyordu, düzeldi [ölçüldü: `74d38399`]. Tip yazmak da tek başına yakalamıyor [ölçüldü: DENETİM 246]: argümanın metotta parametre tipine göre denetimi 4a5'in A satırına bağlı.
- **Yapılacaklar (F, Ö3; sahip D1; DENETİM 246):** modeli kaydetmek için var olmayan bir ad yazılınca ileti ters işi, `TR_load_parameters`'ı öneriyor; doğrusu `TR_save_tensors(yol, TR_parameters(m, TR_named: TR_true))`.
- **Yapılacaklar (D, Ö3; sahip TCB; DENETİM 224 N1):** metin kapısı Windows'ta koşuyor; `TestDocLinks` Linux yolunda harf büyüklüğü farkını görmez, hedef tam harfle karşılaştırılabilir. 83 gönderimde 0 kez oldu [ölçüldü: DENETİM 224], sonraki tam denetim yakalar [hipotez].
- **Mevcut durum (2026-11-06; DENETİM 228, 230):** asistanın ekranı (`tools/assistant/`) gerçek TR sunucusuyla konuşmadı (TR'de HTTP sunucusu yok, olay akışı sahte); yalnız Chromium tabanlı tarayıcıda denendi [ölçüldü: ekran oturumu].
- **Yapılacaklar (A, Ö1, canlı doğrulama; sahip S1; canlı deneme, 2026-11-08; DENETİM 309):** gerçek mikrofonda sözün başı hata vermeden kayboluyor (araçta, `tools/assistant`; dilin 21.2 sınırı yol haritasının 6. adımında): çalmadan sonra mikrofonun yeniden açılması 804–912 ms (sahte mikrofonda 23–36 ms); bir cümlede kesit 1 024 ms'ye indi, Whisper beş sözcüğün beşini yanlış yazdı [ölçüldü: S1, canlı 2. deneme; sebebin baştaki kayıp olduğu hipotez]. Kod düzeldi (S1 mimari dilimi, `6588de1a`). **Canlı doğrulama bekliyor:** Whisper hatalarının sebebinin baştaki kayıp olduğu hipotez; doğrulaması Ahmet'in sesiyle canlı deneme ister. Satır o zamana kadar açık.
- **Mevcut durum (H1 sabit bedel, 2026-11-09, birleşik 62):** korunan önbellekte 675 token'lık geçmişin üstüne 21 token okuma llama.cpp'nin 4,2 katı sürüyor [ölçüldü: H1, ABBAAB, 3 koşu]. Kalan sabit bedel panele sütun sütun açan döngüde (`quantGemmPanels.func1`) ve açmada [ölçüldü: H1, profil]. Zincirde etkisi ölçülmedi [hesap: N = 21–64 için −1,9…−3,3 sn].
- **Yapılacaklar (sınıfsız: bellek; Ö3; sahip TCB tasarımı, sonra H1; Ahmet 2026-11-09; H1 sabit bedel dökümü):** yalnız dil modeli (bağlam 8 192, 696 token) tepe özel bellek 8 191 MiB, llama.cpp b12124 aynı istemle, ağırlığı belleğe okuyan kipte (`-lm` seçeneğinin none kipi, eşlemesiz) 4 005 MiB (2,05 kat); zincirin tepesi 13 600 MiB = öbek dışı 3 426 + Go'nun tuttuğu 10 174 (yığın nesnesi 7 325, geri verilmemiş boş alan ~2,8 GiB) [ölçüldü: H1, `runtime/metrics`, `GODEBUG=gctrace=1`]. Kalemlerin hükmü: yüklemede 32 bitlik kopya yok; önbellek 2 304 MiB (32 bit, öbekte, `lib/layers.tr:457`) canlının ~%95'i; çöp toplayıcının hedefi 3 × canlı (`cmd/trlang/main.go:38`, GOGC 200) tepeyi kuruyor; iki tarafın ağırlık belleği aynı (2 434'e karşı 2 376, llama.cpp'nin REPACK'i ek kopya değil) [ölçüldü: H1]. Çare yönü: yalnız önbelleği öbek dışı bloğa almak, bitler aynı; 4B'de 8,0 → ~4,9 GiB [hesap: DENETİM 335]. Genel kural (büyük sıfır tensörünü öbek dışına) reddedildi: blok `AddCleanup` ile geri veriliyor ama adres ve blok bütçesi (Linux 10 921 blok) kısa ömürlü tensörlerle tükenir [kanıtlı: `host_block.go:22`–`:28`, `:57`–`:60`, `:97`; H1 devri]. Sıradaki adım: TCB'nin önbellek tasarımı, DENETİM, sonra H1. Bit değiştiren iki seçenek (16 bit önbellek, 8 bit etkinleştirme) Ahmet'in kararı; önce deneme kipiyle ölçü (DENETİM 335).
- **Yapılacaklar (sınıfsız: geliştirici belgesi; Ö3; sahip H1, sonraki dilim; H1 sabit bedel Bulgu 6, DENETİM 336):** `internal/compute/tensor/cpu_matmul.go:66`–`:82`'teki paketleme yığınlarının toplam sınırı (`bufTotalMax`) yığını paylaşan her yolu etkiliyor; açıklama yalnız tampon başına sınırı anlatıyor. H1 bu yüzden bir bellek gerilemesine düştü (+3,9 GiB, `dbe7a820`; `9904f6ce`'de düzeldi) [ölçüldü: H1]. Go yorumu kullanıcı iletisi olmadığı için F değil. Aynı dosyanın yanında: `cpu_quant.go`'nun `quantPanelMaxM` yorumundaki "bufMax altında kalır" hesabı yalnız 4B için doğru; 8B'de M ≥ 85, 14B'de M ≥ 61'de paketli A havuzlanmaz, her çarpımda ayrılır (bitler aynı, çöp) [hesap: DENETİM 336]. Sıradaki adım: açıklamaya paylaşımın sonucu; büyük model diliminde sınırın gözden geçirilmesi.
- **Yapılacaklar (sınıfsız: hız; Ö3; sahip S1, sonraki dilim):** ölçünün "çalma başladı" anı çalıcı sürecinin başlatıldığı an, kullanıcının duyduğu an değil: ses mikrofona 831–2 700 ms sonra geliyor; çalıcının payı ilk çalmada 2 076 ms (soğuk açılış), sonra 212–313 ms; kalan en az 0,5–0,8 sn mikrofon yolunda [ölçüldü: S1, 10 çalmanın 6'sında eşik geçildi; kalan hesap: 831–1 029 − ≤313 ms, alt sınır; sebebi hipotez: dshow'un teslimi ya da dahili mikrofonun bastırması]. Aday: açık kalan tek çalıcı ya da önceden açılan çalıcı; önkoşul çalmanın bittiği anı başka yoldan bilmek.
- **Yapılacaklar (D, E ve F1, Ö3; sahip D1, denetleyici ve iletiler; S1 mimari devri; sınıflar DENETİM 331):** (1, E) yapı kurucusunun adlı argümanına verilen fonksiyon ifadesi beklenen tipi almıyor (6.6 beklenen tipten çıkarmayı söylüyor, motor bunu çağrı argümanında yapıyor), doğru program H1145 ile reddediliyor [ölçüldü: S1]; (2) çağrılmamış `.TR_item` `TR_Int(...)`'e verilince denetimden geçiyor, çalışırken düşüyor (D) [ölçüldü: S1]; (3) kullanıcının, adı `TR_Range`'in Türkçe görünüm adıyla aynı olan yapısına H1006 `TR_Range` öneriyor (F1: önerisi kopyalanırsa ajan kendi yapısına dilin tipinin adını verir) [ölçüldü: S1; kopyalanınca ne olduğu ölçülmedi].
- **Yapılacaklar (D, Ö3; sahip TCB; asistan mimari turu, 2026-11-09, DENETİM 324):** sıra alma modeli (konuşmanın bittiğini sesten anlayan model, ör. Smart Turn) 0,6 sn eşiğinin adayı. Omurgası Whisper Tiny [kaynak: smart-turn README, DENETİM 324]; ağırlıkların ONNX dışında bulunup bulunmadığı açık soru. Sıra: ilk sürümün ölçüsünden sonra.
- **Yapılacaklar (C, Ö2; sahip TCB, tasarım turu; S1'in tanısı, 2026-11-08):** kitaplığın sabit önbelleği kapasite verilmezse modelin tam bağlamını sessizce ayırıyor: `TR_build_cache` kapasitesiz çağrılınca modelin `bağlam`'ını alıyor (`lib/layers.tr:598`), her katman ilk kullanımda parçanın şekliyle `TR_capacity` konumluk sıfır tampon ayırıyor (`:457`; Qwen3-4B'de `[1, N, 8, 128]`, N kapasite) [kanıtlı: S1]. Qwen3-4B'de (`qwen3.context_length` 40 960) bu 11 520 MiB [hesap]; kısa bir sohbet de aynı belleği tutuyor. Varsayılan yorumda yazılı (`:595`–`:596`), ama program çalışırken bunu söylemiyor. Tuzağa S1'in zinciri düştü; aynı kapasitesiz çağrı öğretici `examples/tr/47_kendi_uretim_dongusu.tr:26`'da ve `cmd/trlang/e2e/behavior_gguf_workers_test.go:40`'ta da vardı [kanıtlı: DENETİM 301]; ikisi `0d618029`'da kapasiteyi yazıyor, tuzak varsayılanda kalıyor. `TR_generate` etkilenmiyor: kapasiteyi kendisi veriyor (`:633`). Varsayılanın tasarımı (gerektikçe büyüyen önbellek ya da zorunlu kapasite) tasarım turunda kaynaklı tartıyla seçilir.
- **Yapılacaklar (C, Ö2; sahip ekran dilimi; DENETİM 295 Ş2, 2026-11-08):** aynı köke altı açık `fetch` akışı varken ekranın `POST /command`'ı asılı kalıyor, beşte 6 ms'de dönüyor [ölçüldü: DENETİM 295, Chromium, HTTP/1.1]; çok sekmeli tasarımda altıncı sekmede durdur çalışmaz [hipotez: tek sekmede altı akışla ölçüldü]. Sözleşmede yazılı değil. Çare (tek akışı sekmeler arasında paylaşmak, HTTP/2 ya da başka) ağ bölümünün tasarımında kaynaklı tartıyla seçilir; Gemini'nin "en çok 4 abone, en eskiyi kapat" önerisi keyfi ve sözleşmenin "sunucunun sekmeleri tanıması gerekmez" kuralıyla çatışıyor.
- **Yapılacaklar (C, Ö2; sahip ekran dilimi; DENETİM 230 N1):** onay kartında iki aldatma yolu: argümanın gerçek kısmı boş satırlarla görünmeyen yere itilebilir, U+E0000 etiket karakterleri görünür kılınmıyor [kanıtlı: kod; koşulmadı]. HTTP bölümü yazılırken belirteç teslimi ayrıca tartılır.
- **Yapılacaklar (C, Ö2; sahip T1; DENETİM istek 208):** `CopyTypeError`'ın kart kolunu hiçbir test çağırmıyor; CPU testi de `copyTyped` aynı fonksiyonu çağırdığı için cinsli çiftlerde kendini sınıyor. Sanal kartta 225 çiftin 225'i `CopyInto` ile aynı [ölçüldü: DENETİM 208]; kalıcı test `TestTypedPathsOnGPU` kalıbında.
- **Yapılacaklar (C, Ö2; sahip T1; DENETİM istek 207):** erken bırakılan `TR_read_lines` okuyucusu çöp toplayıcıyla kapanıyor: dosya tutamağı toplanana dek açık; aynı programda `TR_write_file` önce H2133, toplanınca hatasız; bir testin okuyucusu sonraki testin silmesini H7125 ile düşürüyor; `GOGC=off`'ta kararsız test 3/3 düştü [ölçüldü: DENETİM 207]. 18.11 çöp toplayıcıyı saymıyor. Çare: koşucu her testten sonra okuyucuları kapatsın; toplayıcı kapanışı için 18.11 tur 4'ün kararı.
- **Yapılacaklar (D, Ö3; sahip T1; DENETİM istek 206):** `max_position_embeddings` 0 olan safetensors uzaktaki çarpımda H6126 ile düşüyor [ölçüldü: T1]; H7155 yalnız GGUF'un, ayrı kod gerekir.
- **Yapılacaklar (F, Ö3; sahip D1, sonraki dilim; DENETİM istek 202):** ileti kodunun binler basamağı alanı gösterir kuralını koruyan test yok; H7030 kart, H7042 tensör alanının dışında kalmış [kanıtlı: `en.json`].

- **Yapılacaklar (F, Ö3; sahip D1; DENETİM istek 163 N1):** `trrun.go:26`'nın `Parse`'ı `StageType`'ı görmüyor: Go testinin metni H0004'lü olsa da çalışır. Yalnız test takımı.

- **Yapılacaklar (F, Ö3; sahip TCB, toplu karar):** görünüm adı iletiye sızıyor, `TR_std(1)`'in iletisi de öyle; `CallName`, `CallNameRef`, `CallPrefix` çağrısı 99 satır [ölçüldü: D1, `git grep -c`].

- **Yapılacaklar (F, Ö3; sahip D1, 2b'den sonra):** `TR_command_line` NUL ile başlayan seçenek adını iki kez kodluyor (`internal/stdlib/process`): anahtarın uzunluğu 2 değil 18 [ölçüldü: D1].

### Yazıyı konuşmaya çevirme (dil tanımı 27 yazılınca kendi bölümüne taşınır)

- **Mevcut durum (S1, `7d50a167`):** leaky_relu `TR_fuse` içinde `TR_relu(x) - TR_relu(-x) * eğim` olarak kaynaştırıldı; eski yoldan tek ayrımı `-0.0` girişte `+0` [ölçüldü: S1, G4 CPU].
- **Aday (D, Ö3; sahip S1; TCB, Ahmet onayladı 2026-11-08):** kaliteli ses için Chatterbox Multilingual'ın çözücüsünü (S3Gen) hızlandırmak; seçenekler daha az akış adımı, çözücünün nicemlenmesi ve TR'de kaynaştırma [hipotez: hiçbiri ölçülmedi]. Dizüstü CPU'sunda Chatterbox gerçek zamandan 7–12 kat yavaş, zamanın yarısı konuşma parçası üreten dil modelinde (T3), yarısı dalga çözücüsünde (S3Gen) [ölçüldü: TCB, `measurements.md`]; tek yarıyı hızlandırmak en çok 2 kat kazandırır [hesap]. Önce çözücünün içinin profili. Dil modeli yarısı için nicemli yol ayrıca denenir: TR'nin GGUF yolu, `BricksDisplay/Chatterbox-Multilingual-TTS-GGUF` (eski adı `hans00/Chatterbox-Multilingual-TTS-GGUF`), revizyon `fe6b7d97`, `q8_0` omurgası; aynı depodaki codec dosyaları çözücünün nicemli bir dönüşümü olabilir [hipotez: README'den doğrulanmadı], TR'nin GGUF yolunun bu mimariyi açıp açmadığı bilinmiyor [hipotez]. onnx-community dönüşümü orta uzunlukta bir Türkçe cümlede metin gömmesinin sınırı dışına düşen bir kimlikle hata verdi (`/text_emb/Gather`, idx 2433) [ölçüldü: TCB, dizüstü]; dönüşümün parçalayıcısı ile gömme tablosu tutmuyor olabilir [hipotez]. Yedek aday Piper'ın Türkçe sesleri: VITS tabanlıysa `lib/vits.tr` onu küçük bir uyarlamayla çalıştırabilir [hipotez: Piper'ın yapısı, ses listesi ve lisansı okunmadı].
- **Yapılacaklar (A, Ö1, dil tanımı 27'nin dilimi; sahip S1; TCB ve DENETİM istek 289, 2026-11-08):** `TR_synthesize` modelin sözlüğünde olmayan karakteri parçalayıcının içinde hatasız atıyor (`lib/vits.tr:413`): söylenen ses metnin bir kısmını kaybediyor, programa haber verilmiyor [kanıtlı: kaynak]. Rakam ve noktalama da atılıyor: sözlükte 5, 7, 8, 9 ve nokta yok [ölçüldü: S1, 2026-11-08]. Zincirde de görüldü: sistem istemi rakamı yazıyla istediği hâlde model 10 turun 4'ünde rakam yazdı, ses onu attı [ölçüldü: S1, C'nin üç koşusu]. Çare dil tanımı 27'de seçilir (hata, uyarı ya da atılanları veren bir sonuç); sayıyı sözcüğe çevirmek ayrı iş.
- **Yapılacaklar (hız; sahip H1 ya da T1):** MMS-TTS'te ses/hesap oranı TR'de 1,06–1,08, PyTorch'ta 23–26, leaky_relu kaynaştırılmadan önce ([ölçümler](measurements.md)). `TR_where` `TR_fuse`'a girmiyor (H6207); [kaynaştırmanın Go kodunda](../internal/compute/tensor/fuse.go) üç girdili düğüm yok [ölçüldü: S1; kanıtlı: S1]. Kalan pay evrişimde (`_ileri` 1,65 sn).

### Denetleyicinin üye erişimi

- **Yapılacaklar (D, Ö3; DENETİM istek 90 N1, 2026-11-04):** `TR_var x = 1` sonra `x.a`: denetim hatasız geçiyor, koşu H2467 veriyor [ölçüldü: DENETİM]. Tipi bilinen bir değerde olmayan üye denetimde yakalanmalı. Sahip: dil çekirdeği, Kol A'dan sonra.

### Ayrıştırıcının uzun zinciri

- **Yapılacaklar (C, Ö3; TCB ve DENETİM istek 90, 2026-11-04):** ayrıştırıcı uzun bir üye ya da indeks zincirinde karesel yavaşlıyor: `parseInfixChain` (`internal/lang/parser/expressions.go:303`, `:308`) her `[i]` ya da `.a` adımında zincirin başından o ana kadarki metni `spanToken` → `lexer.Slice` ile yeniden kesiyor. Geçerli kodda `x` + `.a` × 40 000 → 4,7 sn, `[0]` × 40 000 → 7,7 sn; derinlik sınırını aşan 200 000 `[` → 32 sn; `parser.TestNestingDepthLimit` tek başına 28–33 sn [ölçüldü: komut satırının denetim komutu, CPU profili]. Gerçek dosyalarda zincir kısa [hipotez]. Düzeltme: `spanToken` metni gerektiğinde kursun ya da konumu tutsun; büyüme testi üye ve indeks zinciriyle (`testkit.ScalesLinearly`); derinlik testinin girdisi sınırın hemen üstüne iner. Sahibi dil çekirdeği.

### Komut adı: `trlang`

- **Yapılacaklar (F1, Ö3; DENETİM istek 87 Ş1):** yalın `trlang` yerine eski ad yedi yerde daha kalıyor (dil tanımı 15, mimari belgesi, kılavuz 11, kodların README'si, codegraph README'sinin üç İngilizce yeri: `:154`, `:537`, `:554`; codegraph 3'ün birleşmesinden sonra da duruyor, DENETİM istek 88 N4). Testin altıncı biçimi sözcük listesiyle dar; kural ters dönmeli: ters tırnaklı her yalın eski ad hatadır, istisna dil kodu kalıbı ve izin listesidir.

### Denetim katmanlarının gözden geçirmesi

- **Mevcut durum:** hangi denetim katmanının neyi yakaladığı yazılı değil. Kalan seçim boşluğu: testin paket klasörüne göre okuduğu çıplak dosya adı ([codegraph eksikleri](../tools/codegraph/gaps.md), kritik).
- **Yapılacaklar (C, Ö2; ölçüldü 2026-11-03 18:20–18:30):** paralel oturumlar çalışırken codegraph paketinin kısa testleri main'de 562 sn sürdü; ön denetimin "değişen paketlerin testleri" adımındaki varsayılan 600 sn sınırına 38 sn kalıyor. Aynı paket L4'te, kısa kipsiz 252 sn. `trlang` dalının ön denetimi kodda hata olmadan bu sınırda kaldı.
- **Elle tutulan liste (C, Ö2; T1 Kol B teslim 2a, 2026-11-03; sınıf DENETİM istek 83 Ş7):** `internal/testdata/typed_code_known.txt` satır numarasıyla tutuluyor; satır ekleyen her düzenleme listeyi düşürüyor, güncelleme bayrağı taşınan satırı eklemiyor, yalnız siliyor. Öneri: anahtar dosya ve ad olsun, ya da güncelleme taşınan satırı da yazsın.
- **Yapılacaklar (C, Ö2; Gemini Kodlama2 `trlang` raporu, TCB yeniden üretti 2026-11-03 22:3x):** ön denetim Windows'un Git Bash'inde sonsuza dek asılı kalabiliyor. Değişen dosyaları koruyan testleri seçen adım `codegraph changed --go-tests` çıktısını `<<<` ile okuyor (`tools/full_check.sh:486`); bu kabukta 65.536–65.700 bayt arası bir metin `<<<` ile okununca kabuk asılı kalıyor, daha kısası ve daha uzunu geçiyor [ölçüldü: `read -r x <<<` 65.535 bayt çıkış 0, 65.536 ve 65.607 bayt zaman aşımı, 65.700 bayt 0]. `trlang` dalının çıktısı 65.607 bayttı; ön denetim 30 dakikada dışarıdan kesildi (TCB önce yüke bağlamıştı, yanlıştı). Öneri: `:458`'deki gibi geçici dosyadan okumak. **Sıra öne alındı (DENETİM istek 88 N3, 2026-11-04):** seçim büyüdükçe çıktı pencereye girip çıkıyor; `fa877f38` için seçim main'de 63 683 bayt, codegraph 3'ün birleşmesinde 66 672 bayt [ölçüldü: DENETİM], pencerenin iki yanında. Satır şimdi `full_check.sh:521`.
- **Yapılacaklar (C, Ö3; DENETİM istek 88 N2, 2026-11-04):** dizin önbelleği atlanan dosyanın nedenini artık ileti kodu ve parametreleriyle tutuyor (codegraph 3, 5c); kodsuz bir hata ise önbellekten boş nedenle döner (`internal/devtools/codeindex/skipped.go`, `Reason` önbelleğe yazılmıyor). `52ce4b9f`'de sözcük çözümleyicide ve ayrıştırıcıda kodsuz hata yok [ölçüldü: DENETİM istek 88, `git grep`], yani olmuyor. Koruma: kodsuz hatada metni de saklamak ya da kodsuz hatayı testle yasaklamak.
- **Envanter ve plan (2026-11-04; DENETİM istek 90 koşullu onay, şartları işlendi):** katmanlar yedi: metin testleri, düzenleme kancası, ön denetim, yerel kapı, L4, CI, DENETİM. Gerçekten yavaş iki test var (ayrıştırıcının derinlik testi, hazır modelin transformers kıyası). Ayrıntı not klasöründe (envanter ve test planı belgeleri). Sıra: plan adım 1 (here-string, TCB; kilit testi, T1), sonra adım 2–6 (mutasyon denemesi kontrol noktası paketiyle başlar, ağır testler yalnız değişen paketler adımında atlanır).

### Güvenilirlik (C): yol haritasından

Yol haritasının açık işlerinden DENETİM'in C saydıkları (istek 76, 2. bakış; taban `f6c22ae8`).

- **Yapılacaklar (C, Ö2, Kol B; ölçülmedi):** küçük işaretçili yığınla dönen döngüde çöp toplama turunun tik arasına alınması ölçülecek (roadmap "Bölümü olmayan işler"); 100 Hz döngü Jarvis'in ön koşulu [tahmin: DENETİM].
- **Yapılacaklar (C, Ö2; ölçüldü: D1 13.1c ön denetimi ve roadmap "Araç işleri"):** kararsız testler. Bilinen: `TestTheLockIsHandedOffInTimeAfterStatementsSlowDown` (10.2'de); `TestOtherTasksRunDuringLongTensorWork`, G4'te iki kez "gözcü yük sırasında hiç uyanmadı" (T1, `99f1348f`; H1, `9f06f747`): kartta eşitleme kilidi bırakmıyor, gözcü yükten hemen önce uyanırsa bütün yükü kuyrukta geçirir [hipotez: T1; 10.2'nin GPU kilidi satırı]; ölçü penceresi yükle kesişen beklemeyi artık sayıyor, ileti en yakın uyanışları yazıyor, satır bir daha düşerse sebep okunur; `TestProfileModuleLines` Windows'un dağıtım işinde bir kez kaldı (CI 42507239962, `96026865`), yeniden koşuda geçti: program 0,021 sn sürdü, tek örnek çağrı satırına düştü [kanıtlı: CI günlüğü; 1826f1ce'nin onarımı kısa programı kapsamıyor, hipotez]; `TestProfileBackwardTimeGoesToForwardLine` aynı işte bir kez kaldı (CI 42537298828, `583237d0`): geri giden zamanın payı 0,18, beklenen en az 0,3; profilleyici o birleşimde değişmedi, yeniden koşuda geçti [kanıtlı: CI günlüğü; örneklemeye bağlı olduğu hipotez, yinelerse sebep aranır]; öbürleri sayılıp her biri kendi maddesine yazılacak.
- **Yapılacaklar (E, Ö3):** hata ayıklayıcıda F5 ile `TR_input()` çalışmıyor (roadmap "Araç işleri"): asılı kalmıyor, H8102 veriyor [kanıtlı: DENETİM 170, kod okuması; F5 koşulmadı].
- **Yapılacaklar (C adayı, ölçülmedi):** eğitimde artan adım süresi, sızıntı olabilir (roadmap 2. kilometre taşı); sızıntıysa uzun eğitim kesilir [hipotez: DENETİM]. Önce ölçülecek; Ahmet'in kararını bekleyen eğitim hızı adaylarıyla birlikte.

## 20. Kontrol noktası

Kurallar: [`docs/spec/20_kontrol_noktasi.md`](spec/20_kontrol_noktasi.md). Taslak (2026-10-29): ana oturum CLAUDE.md'nin "kesilen bir eğitim kaldığı yerden aynı sonuçla sürer" cümlesinden türeterek yeniden yazdı; bölümün çoğu uygulanmış ve testli. Metin DENETİM 20-1'den geçti, bulguları işlendi (`6a525af7`); kararlar Gemini Koşu 1 (`4afda971`), Koşu 4 (`23e7be25`) ve Koşu 2 (20.11, `70a1e6fe`) ile geçti. Kapanış turu yapılmadı.

### 20.1

- **Mevcut durum:** koşu klasörü yazma kuralıyla çözülüyor [kanıtlı: kontrol noktası paketi, Gemini 4].
- **Yapılacaklar:** TR testi.

### 20.2

- **Mevcut durum:** sözlük anahtarlarında eski kodlu anahtar kuralı metinden kalktı (bugün sözdizimi hatası). Tam sayı, mantık ve float64 tensör `TR_Checkpoint`'in kaydında H2629 veriyor [ölçüldü: Gemini 4, DENETİM 166].
- **Yapılacaklar (E, Ö2; sahip T1; Gemini 4, DENETİM 166):** bu tensörlerin kaydı ve yüklemesi. Hata kurulumda değil ilk kayıtta geliyor, nokta kalmıyor; panikte geçici klasör ve açık dosya kalıyor, Windows'ta ikinci kayıt H2142 veriyor [ölçüldü: DENETİM 166]. Adlar `TR_named_parameters` ile (13.8a).

### 20.4

- **Mevcut durum (Kol B teslim 2b):** nokta her modelin donmuş parametrelerinin adlarını kaydediyor (noktanın durum dosyasında modelin kaydına eklenen donmuş adlar listesi); sürdürme ve geri dönüş aynı kümeyi kuruyor (adı listede olan donar, öbür parametreler türev izler; hiç türev izlememiş durum tensörüne dokunulmaz). Biçim sürümü 4 (TCB kararı): sürüm 3 dosyası okunuyor; donma durumu kaydetmediği için sürdürme ve geri dönüş programın kurduğu donma durumunu değiştirmiyor (Kol B teslim 2c, DENETİM istek 84 Ş1; önce programın dondurduğu parametreyi çözüyordu [ölçüldü: DENETİM]), sürüm 5 ve sonrası H7421; donmuş diye kaydedilen ad modelin parametresi değilse kayıt bozuk (H6993) [test: TR testi "15.34 kontrol noktasından donmuş sürdürme: …", `TestFrozenStateIsSavedAndResumed`, `TestVersionThreeLeavesFrozenStateAlone`, `TestFrozenNameMustBeAParameter`; beş bozma denemesi ve 2c'nin sürüm denetimini kaldıran deneme yakalandı]. Dil tanımının 20'deki "Biçim sürümü 3'tür ve yalnız o okunur" cümlesi bu kararla değişmeli (pending).

### 20.11

- **Mevcut durum:** yazılamayan tür H6960 ya da H3112–H3115 verir; `TR_load_parameters` türleri şekillerden sonra, yazmadan önce denetler, tür hatasında hiçbir parametre değişmez [ölçüldü: T1, DENETİM 206; TR testi `tests/kontrol_noktasi_test.tr`].
- **Yapılacaklar (A, Ö1, Kol B, 15'in A'sıyla; ölçüm status'un):** `TR_AnyTensor` sözlüğü ve parametrenin cinsine daraltma (20.11 hassasiyeti farklıysa çevrilir der; `tests/kontrol_noktasi_test.tr` bugünkü H6960'ı sabitliyor, kural gelince değişir), daraltan hassasiyet dönüşümünde sığmama hatası, nicemlenmiş yaprağa bütün yazma (15.54) (15'in yan değişikliği, uygulanmadı).
- **Açık (Sonraya, 2. kilometre taşında öne çekilmeli):** hazır `.safetensors` dosyaları tablo (RoPE) taşımaz; "bütün yapraklar sözlükte" kuralıyla yükleme alt yapıya yapılır. LoRA alanlarıyla aynı soru (15.34'ün açığı).
- **Gemini 15/2 (2026-10-29), uygulanacak:** nicemlenmiş hassasiyet farkı yüklemede hata, sessiz açma yok (T2, 15.54 ile).

### 20.12, 20.13

- **Mevcut durum:** `TR_load_state`'in mantık değeri dönüşü 13.10'un status'unda. Kontrol noktası cinsli tensör yazmıyor: Go'daki kontrol noktası paketi `TensorWriter.Write`'ı ondalık veriyle çağırıyor, bu yüzden `op.TR_state`'in "step"i ondalık tensör; `TR_Int` için `WriteRaw` ve `ml.TensorEntry.Ham` T1 dilim 2'den beri hazır, paket bağlanmadı (T1 bulgu 11) [kanıtlı: DENETİM istek 57, kontrol noktasının `writeTensors`'ı 16 bitlik olmayanı `p.t.Data()` ile yazıyor]; karışık cinsli `TR_Dict[TR_String, TR_AnyTensor]` yok (15'in yan değişikliği, uygulanmadı).

### 20.16

- **Mevcut durum:** nazik durdurmada süreç isteğin sinyaliyle biter (Unix'te sinyalle ölür ya da 128+N, Windows'ta `STATUS_CONTROL_C_EXIT`); ikinci istek süreci hemen bitirir; 1.9'un kuralı uygulanmış [ölçüldü: main `85a19c36`, `TestInterruptGracefulExit` ve `TestSecondInterruptExitsImmediately`; `cmd/trlang/signals_unix.go:27`, `signals_windows.go:27`].

### 20.17

- **Mevcut durum:** paylaşılan üreteç programa bağlı bir PCG (`math/rand/v2`), `"pcg:00000000000054520000000000002017"` ile başlıyor, durumu kontrol noktasının durum dosyasında `"rng"` alanına yazılıyor ve yüklenince geri konuyor; kesilip sürdürülen eğitim tohumsuz `TR_randn` ve `TR_sample` ile bit bit aynı [test: TestCheckpointResumeBitExact].
- **Yapılacaklar (C, Ö2, 15.45 ile; ölçüm status'un):** `TR_dropout` (15.23d) geldiğinde aynı üreteçten çekmesi.
- **Yapılacaklar (dil tanımı turu 2, 2026-11-01):** cihazın sayaçlı üreteci (15.45) gelince durumu "rng_device" alanına (cihazdan duruma eşlem) yazılır ve geri yüklenir, 15.45 ile birlikte; bugün GPU oynatmasında rastgelelik yok [kanıtlı: DENETİM kol-denetim-tur2-rapor-sonuc.md, T1'in ilk dilimi yalnız CPU üretecinde]. "rng"'nin "pcg:" öneki ve bilinmeyen önekin yükleme hatası (H6953) T1'de geldi [test: checkpoint behavior_security_test.go].
- **Yapılacaklar (T1 birleşmesi, 2026-11-01):** 20.17'nin Test satırındaki TR testi: rastgelelik çekilmeden yazılan ilk noktanın `"rng"`'si `"pcg:00000000000054520000000000002017"`'dir; bugün yalnız Go testlerinde [ölçüldü: `git grep 54520000000000002017`].
- **Mevcut durum (C, Ö2; T1, 2026-11-05; DENETİM istek 122 N2):** kontrol noktası 16 bitlik tensörü bitleriyle yazıyor, sürdürmede bitleriyle kuruyor: sinyal NaN'ı CPU'da susmuyor. Önce bf16 `0x7F81` sürdürünce `0x7FC1`, fp16 `0x7C01` sürdürünce `0x7E01` oluyordu [ölçüldü: T1]. Disk biçimi ve sürüm aynı: bütün desenlerde ayrılan yalnız sinyal NaN'ları, bf16'da 126, fp16'da 1022 [test: `TestSixteenBitTensorKeepsItsBits`, `TestBitsChangeOnlySignalingNaNs`].
- **Mevcut durum (kart; T1, 2026-11-05):** kontrol noktası kartın verdiği biti değiştirmiyor: kartta yükleyip `TR_save_tensors` ile yazmak, nokta dosyası ve sürdürme aynı biti veriyor [ölçüldü: G4, RTX PRO 6000, `7691d573`, tür başına dört desen].

### 20.14

- **Mevcut durum (T1, birleşik 59):** noktanın yüklemesi (geri dönüş ve sürdürme) modeldeki bütün tensörlerin `.TR_grad`'ını `TR_none` yapıyor ve "gradyan eklendi" bilgisini siliyor; sonraki `TR_backward` eski gradyanın üstüne eklemiyor. Donma durumunu geri yüklüyor [test: TR testi "20.14 geri dönüşte parametrelerin gradyanı…"; ölçüldü: Gemini 4, DENETİM 166 (donma)].

## 21. Dış programlar

Kurallar: [`docs/spec/21_dis_programlar.md`](spec/21_dis_programlar.md). Yazıldı (2026-11-02): ana oturum Gemini Jarvis A'dan (DENETİM istek 36 doğruladı) yazdı; metin DENETİM istek 37'den, düzeltmeleri istek 38 ve 39'dan geçti; main'e girdi (2026-11-02).

### 21.1–21.8

- **Mevcut durum (S1, 2026-11-06):** `TR_run_process` var: 21.1, 21.3–21.8 ve `TR_input:`, `TR_env:`, `TR_timeout:`, `TR_exit_codes:`; Windows'ta iş nesnesi ve Ctrl+Break, Unix'te süreç grubu [test: `internal/stdlib/process`, `cmd/trlang/e2e`]. `ffmpeg` 9.0.2 ile 21.1'in örneği Windows'ta koştu: 2 saniyelik mp3 32 000 örnek [ölçüldü: TCB, `aaf3078f`'den önceki `78c97d72` ikilisi, yerel]. `TR_spawn_process` ve `TR_Process` var (21.2, S1, birleşik 49): Linux'ta bütün süreç paketi, yarış dedektörü ve e2e geçti; `ffmpeg` çıktısı doğrudan çıktıyla bayt bayt aynı, geri basınç, yapışkan taşma, iptal 131 ms, SIGINT ve SIGTERM geçti [ölçüldü: DENETİM 248, G4]. Canlı mikrofon ve `dshow` koşmadı.
- **Yapılacaklar (C, Ö2; sahip S1, 21.2 dilimi; DENETİM 226 Ş2):** Unix'te `trlang test` ve kabukta durdurma isteği yakalanmıyor; süreç grubundaki dış program TR ölünce kalır [hipotez: kod okuma, `catchStop` yalnız `runProgram`'da]. Kabuktan çıkınca yazmayan program kalıyor, `TR_exit(0)` da durdurmuyor [ölçüldü: DENETİM 248, G4]; çıkış yolu `catchStop` değil, aynı dilimde yapılır.
- **Yapılacaklar (C, Ö2; sahip S1; DENETİM 250):** `TR_run_process`'in çıktı sınırı gerçek bellekte iş görmüyor: G4'te 176 GiB'de `yes` 51,5 sn sonra SIGKILL aldı, ileti yok, tepe 173,55 GiB [ölçüldü: G4]. Sınır küçültülünce ileti geliyor, ama tepe bütün bellek kadar [ölçüldü: G4, beş boy]. Tampon ikiye katlanarak büyüyor, süreç kodunun "eskisi ile yenisi birlikte durur" hesabı yetmiyor [hipotez: tepe sayılarına uyuyor]. Yön: sabit boy parça listesi, birleştirme yalnız başarılı dönüşte; test tepe ayırmayı ölçsün. 21.2'nin torun cümlesine de test bağlanacak: 4 saniye ve sessiz kesilme [ölçüldü: DENETİM 250].
- **Yapılacaklar (D, Ö3; sahip D1; S1 Bulgu 1):** denetleyici `TR_Path` öğesini yalnız yazıldığı yerde görüyor; adla gelen yol çalışma anında H7490 alıyor, program başlamaz. `TR_Path` çekirdek tip değil.
- **Yapılacaklar (A, Ö1, yerli ses girişi; roadmap, yerel sesli asistan 6. adım; DENETİM 233):** dış süreçten okunan seste kaynak kendi içinde veri atarsa TR görmez: `ffmpeg` dshow ve `arecord` taşmada sürer [kanıtlı: DENETİM 233, kaynak kod]. 21.2'nin sözü "TR aldığı baytı kaybetmez"dir, "mikrofon kaybetmez" değil.
- **Sınır (DENETİM 226 N1, N4):** `AUTH` sözcüğü `SSH_AUTH_SOCK`'u da süzer, git push ajana ulaşamaz, ileti `TR_env:` yazımını verir [ölçüldü]. `/dev/tty` okuyan program `TR_timeout:` olmadan dönmeyebilir [hipotez].

## 22. Bayt ve sayı

Kurallar: [`docs/spec/22_bayt_ve_sayi.md`](spec/22_bayt_ve_sayi.md). Yazıldı (2026-11-02): ana oturum Gemini Jarvis A'dan (DENETİM istek 36 doğruladı) yazdı; metin DENETİM istek 37'den, düzeltmeleri istek 38 ve 39'dan geçti; main'e girdi (2026-11-02).

### 22.1–22.4

- **Mevcut durum (S1, 2026-11-05):** 22.1–22.3 var: `TR_from_bytes[E]`, `.TR_to_bytes`, `.TR_read_int`, `.TR_read_float`, `TR_int_to_bytes`, `TR_float_to_bytes`; değişmez başka cins hassasiyet denetim hatası [test: `tests/bayt_ve_sayi_test.tr`, `tests/negative/bayt_hassasiyet_cinsi.tr`, `FuzzFromBytes`, `FuzzReadNumber`]. 22.4 var: `tr/audio`, `TR_read_wav`, `TR_write_wav`, `TR_Sound`; bozuk dosya sebebiyle H7061–H7071 ve H7080 [test: `lib/audio_test.tr`, `FuzzReadWav`]. Okumanın bellek tepesi dosyanın yaklaşık 10 katı: 38,4 MB'lık dosyada 395 904 KB [ölçüldü: S1].
- **Yapılacaklar (F, Ö3; sahip S1):** iletide bayt adı okunmuyor: H7067 metin olmayan parça adını sayı listesi diye gösteriyor [ölçüldü: S1]. Uzun dosyalar için parça parça okuma, önce gerçek ihtiyaç ölçülür.

## 23. Sinyal ve spektrum

Kurallar: [`docs/spec/23_sinyal_ve_spektrum.md`](spec/23_sinyal_ve_spektrum.md). Yazıldı (2026-11-02): ana oturum Gemini Jarvis B'den (DENETİM istek 42 doğruladı, istek 43 kararları okudu) yazdı; metin DENETİM istek 44'ten, düzeltmeleri istek 45'ten geçti (2026-11-02).

### 23.1–23.6

- **Mevcut durum (S1, 2026-11-05):** 23.1–23.3 var, CPU'da: `TR_rfft` (karışık taban, büyük asal çarpanda Bluestein), `TR_hann_window`, `TR_stft`; işçi sayısından bağımsız aynı bitler; türev izler (sayısal türevle kıyas); GPU'da H7076 [test: `tests/sinyal_test.tr`, `TestStftSameBitsForAnyWorkerCount`, `TestPlanMatchesTheDefinition`]. 23.4 `TR_mel_filterbank`, 23.6 `TR_whisper_log_mel` var [test: `tests/sinyal_test.tr`, `lib/audio_test.tr`, `tests/whisper_girdi_test.tr`, `TestMelFiltersMatchWhisper`]. Gerçek konuşmada, `jfk` sesinde Whisper'la en büyük fark 3,1e-5, sınırı aşan öğe 0 [ölçüldü: S1, G4].
- **Yapılacaklar (E, Ö3; sahip S1):** `TR_fuse` içinde `TR_maximum` H3107 veriyor, 15.44 izin veriyor [ölçüldü: S1]. GPU'da Fourier dönüşümü kendi çekirdeğiyle (23.5); o gelene kadar GPU'da hata.

## 24. Sohbet biçimi

Kurallar: [`docs/spec/24_sohbet_bicimi.md`](spec/24_sohbet_bicimi.md). Yazıldı (2026-11-02): ana oturum Gemini Jarvis B'den (DENETİM istek 42 doğruladı, istek 43 kararları okudu) yazdı; metin DENETİM istek 44'ten, düzeltmeleri istek 45 ve 46'dan geçti (2026-11-02).

### 24.1–24.4

- **Mevcut durum (S1, 2026-11-08):** `tr/chat` 24.1–24.3'ü uyguluyor, 24.4 dışında; `lib/chat.tr`. Küçük parçalayıcıyla 14 durum transformers'ın kimlikleriyle birebir, iki bilinen fark ayrı testte [test: `tests/chat_format_test.tr`]; gerçek Qwen3-4B'de GGUF'tan açılan parçalayıcıyla 14 durumda 1191 parça birebir [ölçüldü: S1, dizüstü].
- **Yapılacaklar:** 24.4 `TR_read_chat`.
- **Yapılacaklar (Jarvis B kararı, `lib/layers.tr`):** `TR_generate`'in `TR_eos`'u `TR_List[TR_Int] = []` olur, çünkü modeller birden çok durdurma parçası taşır: Qwen3-0.6B `[151645, 151643]`, Llama-3.1-8B-Instruct `[128001, 128008, 128009]` [kaynak: DENETİM istek 42, `generation_config.json`]. Çağıranların göçüyle: `lib/layers_test.tr`, `examples/tr/46_hazir_model_ve_uretim.tr`, `tools/gpu/ss_exam.tr`, `docs/guide/10_hazir_katmanlarla_gpt.md` [ölçüldü: DENETİM istek 43]. T1'in alanı.

## 25. Konuşmayı yazıya çevirme

Kurallar: [`docs/spec/25_konusma_tanima.md`](spec/25_konusma_tanima.md). Yazıldı (2026-11-05): ana oturum Gemini Whisper raporundan yazdı; alıntıları DENETİM istek 175, metni istek 177 okudu.

### 25.1–25.9

- **Mevcut durum (S1, 2026-11-05):** `TR_pretrained_whisper` ve 25.1'in kapıları var, kodlayıcı çalışıyor; `lib/whisper.tr`. Whisper-tiny'de özgünle en büyük fark 6,99e-4, ortalama 1,41e-6; float64 doğrusuna TR 4,19e-4, özgün 4,77e-4 uzak [ölçüldü: S1, G4 CPU, tek ses]. Ölçüt `TR_rtol: 1e-3`, `TR_atol: 1e-3`, ortalama mutlak fark 1e-5'in altında (TCB). Çözücü de var: tek adım logitinde en büyük fark 2,86e-5, dört konumda en olası parça özgünle aynı, float64 doğrusuna TR 1,97e-5, özgün 2,31e-5 uzak [ölçüldü: S1, G4 CPU, tek ses]; ölçüt `TR_rtol: 1e-4`, `TR_atol: 1e-4`, ortalama 1e-5 altı, en olası parça aynı. `TR_transcribe` var: `whisper-tiny` ve `whisper-base`, `jfk`, 81 saniyelik birleşik ses ve bir Türkçe seste parçalar, kesitler, zamanlar ve metin özgünle aynı; `GOMAXPROCS` 1, 2, 3, 8 aynı [ölçüldü: S1, G4 CPU, her biri bir kez]. Ölçüt aynılıktır, doğruluk değil: model Türkçe seste yanlış yazınca TR de aynısını yazar. Dış 16 bit blok metni değiştirmez, gövde float32 bloğunda. `whisper-small` ve `whisper-large-v3` de bire bir; sessizlik ve ortalama değerleri gerçek modelde özgünle 1e-4 içinde, gürültü sesinde pencere iki tarafta da atlanıyor [ölçüldü: S1, G4 CPU]. Hız, 8 iş parçacığı, çevirme süresi: TR özgünün 0,95–1,19 katı, whisper.cpp'nin 2,0–3,9 katı (whisper.cpp ağırlığı float16, TR float32; metin altı çiftin ikisinde aynı, orada 2,26 ve 2,93); 1 iş parçacığında özgünün 1,31–1,60, whisper.cpp'nin 1,20–1,87 katı [ölçüldü: S1, üç tur, sıra dönüşümlü]. `small`'da kodlayıcı 8 iş parçacığında whisper.cpp'nin %11–13 gerisinde, tek iş parçacığında başa baş; çözücü adımı aynı metinde yaklaşık 5 ve 3 kat yavaş [hesap: S1, DENETİM 199; iki ayrı makine]. NVIDIA kartsız bir dizüstünde, 14 iş parçacığı: `small` 16,8 saniyelik Türkçe sesi 4,2 sn, 81 saniyelik sesi 11,8 sn, tepe 4,2 GB; `base` 1,5 ve 4,8 sn [ölçüldü: TCB, üç turun ortancası, turlar arası yayılım %6,7–24,9, prizde, `78aeb8cc`]; `base` Türkçe seste bozuk yazıyor, hızlı yol sayılmaz. Mikrofondan kaydedilen 15,6 saniyelik Türkçe konuşma 2,5 sn'de yazıya döndü [ölçüldü: TCB, tek deneme]; metin doğru [Ahmet; özgünle kıyaslanmadı]. Tepe bellek `small` 4,2 GB, özgün 2,0 GB, whisper.cpp 0,66 GB; sebep çöp hızı, canlı bellek pencereyle büyümüyor, çare 15.53 [ölçüldü: S1].
- **Yapılacaklar (E, Ö2; sahip H1):** kodlayıcı ve çözücü adımı. Skaler GELU kodlayıcıda (`small`'da %12) [ölçüldü: S1; çağrı payı hesap: DENETİM 202], adımlı kopya ve matris çarpımı çözücüde; whisper.cpp aynı koşuda ölçülür.
- **Yapılacaklar (E, Ö3; sahip S1):** dilim dilim, DENETİM istek 178'den geçen yönergeyle.

### 25.10–25.17

- **Mevcut durum (T1, birleşik 49):** `tr/vad`: Silero VAD ileri geçişi ve kesitleyici saf `.tr`. Aynı ağırlıkla TR, JIT'in tabanının 0,99 ve 0,91 katı, kesit sınırları aynı; `TR_push` G4'te ortalama 0,284 ms [ölçüldü: T1]. Kart testi G4'te 40/40 [ölçüldü: DENETİM 250]. Sessizlik sayacı Silero'dan en çok 2 çerçeve ayrılıyor, kesit sayısı ayrılmıyor [ölçüldü: DENETİM 245, FLEURS tr dev]. Gerçek ağırlıkla `TR_transcribe` ve canlı dinleme koşmadı.
- **Mevcut durum (2026-11-06; E, Ö3 kapandı; TCB):** dağıtılacak model yayımlanan safetensors kalıyor: bilinen metinli tek kayıtta iki ağırlık da 9 parçayı 9 kesite ayırdı, kaçan, bölünen, birleşen ve yanlış tetikleme yok; eşitlikte önceden yazılan kural safetensors der [ölçüldü: dizüstü, tek konuşmacı, tek oda]. Yön gösterir.
- **Mevcut durum (2026-11-06; canlı dinleme, ilk koşu):** mikrofon, `TR_spawn_process`, Silero VAD ve Whisper small zinciri dizüstünde iki kez koştu. 60,0 saniye ses 60,6 saniyede okundu, aynı sesin dosyadan metniyle karşılaştırıldı, içerik kaybı görülmedi [ölçüldü: ikinci koşu]. 2,1–8,6 saniyelik sözde yazı süresi 2,2–3,9 saniye, 21 saniyelik sözde 6,3 saniye [ölçüldü; aynı makinede başka iş koşuyordu].
- **Yapılacaklar (hız; sahip H1; canlı dinleme):** kısa sözde de Whisper yaklaşık 2,2 saniyeden aşağı inmiyor [ölçüldü]; sebep tam pencere olabilir [hipotez]. Sıradaki adım: profil.
- **Yapılacaklar (F, Ö3; sahip T1):** çok büyük sessizlik süresi genel taşma iletisi veriyor (H2484) [ölçüldü: T1, DENETİM 245].

## 26. Türkçe sözcük çözümleme

- **Mevcut durum (2026-11-06; T1, birleşik 55):** `tr/turkish` Türkçe sözcüğün sözlük biçimini buluyor (26): Gemini'nin kuralları, Zemberek kökleri. Görülmemiş 60 sözcükte 57, gerileme tabanı olan 120 sözcükte 117 doğru [ölçüldü: DENETİM 264, TCB]. Ad maddesi önce geldiği için bazı fiil çekimleri ad verir [ölçüldü: T1].

## 28. Ağ: HTTP sunucusu

Kurallar: [`docs/spec/28_ag.md`](spec/28_ag.md). Yazıldı (2026-11-09): dört tasarım turu; güvenlik incelemesi ve emsal araştırması Gemini'nin, kararlar DENETİM 320, 329, 330, 338, 346'dan, metin 347'den geçti.

- **Mevcut durum (2026-11-09):** uygulanmadı; `TR_serve_http` sözdizimi hatası verir: "bilinen bir kimlik değil" [ölçüldü: main `f3fabb79`, trlang'ın denetimi]. Yan değişiklikleri de uygulanmadı: 18.13 ve 18.17'nin `TR_owner_only:`'si, 19.4'te belirteç ve olay akışı.
- **Yapılacaklar (E, Ö3; sahip TCB, kod yönergesi; sırası asistan ekranının planında):** 28.1–28.9 ve yan değişiklikleri iki aşamada (yönerge `yonerge-http-28.md`, DENETİM 356): aşama 1 sunucu, güvenlik, dosya, belirteç, `TR_owner_only:` ve akışsız kapanış; aşama 2 akış, nabız, olay maskesi; *Test:* satırları uygulanınca TR testine döner. Kod yönergesinden önce bölümün kimlik ve gösterim adları kod listesine.
- **Yapılacaklar (D, Ö3; sahip TCB, 28'in uygulama dilimiyle; DENETİM 351 Ş2):** 18.13'ün "gizli değeri `TR_owner_only:` olmadan yazmak çağrı hatasıdır" cümlesi motorda yok: `TR_write_file(yol, gizli)` hatasız yazar, dosya sıradan izinle kalır (yazma `WriteAtomic`'ten geçer, geçici dosya `0600` açılıp sonra istenen izne çevrilir) [kanıtlı: DENETİM 346 ve 351, `atomic_write.go`]. Windows erişim listesinde `SYSTEM`'in de olup olmayacağı uygulamada ölçülür [hipotez: DENETİM 351 N5].
- **Yapılacaklar (sınıfsız: sözleşme; Ö3; sahip ekran kolu, HTTP'nin uygulama dilimiyle):** `tools/assistant/contract.md` olay verisindeki `seq`'i bırakır, ekran SSE'nin `id` satırını okur ve nabızda son kimliği tutar (28.7).
