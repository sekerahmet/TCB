# Durum

Dil tanımının yeniden yazılan bölümlerine göre bugünkü TR'nin durumu ve kurala varmak için yapılacaklar. Her maddenin kuralı dil tanımındadır ([`docs/spec/`](spec/README.md)); yapılan iş testiyle birlikte buradan silinir. **Mevcut durum** yalnız kuraldan farkı yazar: eksik, yanlış, sınır ve ölçülmemiş olan. Kurala uyan davranış ve kapanan parça yazılmaz; kapanan parça kapandığı commit'te silinir. Kaynak olarak açık iddianın etiketi kalır, geçmişi git tutar. Kural bir yerde "denetim hatası" diyorsa hatanın çalışma anında gelmesi de farktır. İşlerin sırası [yol haritasındadır](roadmap.md). Bu belgede kimlikler dil tanımının yazımıyla anılır (`TR_if`, 2.3a).

**Madde biçimi:** bölümlerdeki her madde kalıcı bir kimlik ve türle başlar: kimlik (S-412), tür (Yapılacaklar ya da Mevcut durum). Madde mesajda, devirde ve denetimde kimlikle anılır, satır numarasıyla anılmaz. Silinen maddenin kimliği `internal/devtools/testrunner/testdata/status_deleted_ids.txt`'ye yazılır ve yeniden kullanılmaz; yeni madde sıradaki kimliği alır ve sayı bir artar. Madde alt satırlarıyla en çok 500 harftir. Kimlikli ölçünün sayısı kopyalanmaz, ölçünün kimliği yazılır; tazelik notu, saat, tarih ve kanıt etiketinin dışında DENETİM numarası yazılmaz, karar commit mesajında durur [test: `TestStatusItemsFollowTheFormat`]. Türden sonra ayraçta sınıf, öncelik, varsa dilim ve sahip gelir; sonra kuraldan fark, dayanağın etiketi ve varsa "Sıradaki:" adımı.

**Sıradaki kimlik:** S-704.

## Önem ve öncelik

Etiket her "Yapılacaklar"ın ayracındadır: sınıf, öncelik, varsa kol ya da dilim ve sahip, ör. **S-412 Yapılacaklar (A, Ö1, 4a4; sahip T1)**. Etiketsiz madde D, F1, E ya da F sınıfındadır ve Ö3'tür.

**Sınıf karar anında yazılır:** her karar ve bulgu yazılırken (yol haritası, status ya da `pending.md`) sınıfı da yazılır. A, B ya da C ise aynı gün buraya etiketli bir "Yapılacaklar" satırı açılır; dil tanımında bölümü olmayan iş "Bölümü olmayan işler"e girer. D, E ve F yol haritasında kalabilir. Ö0 ve Ö1 listelerindeki her madde kendi bölümünde aynı öncelikle açık bir "Yapılacaklar" taşır, bölümdeki her açık Ö0 ve Ö1 satırı da listede durur; iş kapanınca madde listeden de çıkar; bölümü olmayan iş listede kendi ### başlığının adıyla, bölüm başındaki iş bölüm numarasıyla anılır [test: `TestPriorityListsNameOpenItems`]. Önem taramaları status'un yanında `pending.md`'yi ve TCB'nin takip tablosunu da okur; yol haritasını TCB ya da DENETİM okur.

**Sınıflar** (sıra tasarım ölçütlerinin sırası: doğruluk, güvenlik, ajan için hatasız):
- **A, sessiz yanlış:** program hata vermeden yanlış sonuç, yanlış veri ya da yanlış gradyan veriyor; diskteki veriyi sessizce kaybediyor ya da bozuyor; ya da bir test bu yanlışı doğru diye sabitliyor; ya da dilin kendi aracı (ikiliyle gelen komut, eklenti) ya da belgesinin veya iletisinin verdiği komut hata vermeden yanlış sonuç gösteriyor: program çalışmadan geçmiş görünüyor.
- **B, güvenlik:** dış girdi (kullanıcı metni, dosya, yol) programın davranışını değiştirebiliyor ya da gizli bir değer insanın göreceği çıktıya sızıyor (19.4).
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
1. 15.15, `TR_where`'in sayı dalının hassasiyeti (A, sahip T1).
2. 15.37, cihaz adının tek yazımı (A, sahip T1).

**Ö1, dilimlerin kesin listeleri:** 4a5: 11.8, 13.8, 6.9 ve 7.7 (ya da 4c), 1.2, 3.13, 5.5, 15.5, 13.10'un varsayılanlı parametresi (tur 7); argümanın, alanın, dönüşün, varsayılanın ve açılan adın tipi: 6.1, 6.4, 6.5, 6.6, 7.2, 7.8, 8.1, 7.6 (ya da 4c); karışık değişmez: 3.6, 3.7; fonksiyon ifadesiyle başlatılan görevin tipi: 10.1; anahtar fonksiyonunun tipi: 13.3; parantezsiz yerleşik metot: 13.6; imzaların öğe ve fonksiyon tipi: 13.9. 4b: 3.5, 6.2, 5.12, 5.6, 7.4, 3.10'un koşulu, 11.1. 4c: 3.14, 15.1'in öğe cinsi, 13.1'in boş listesi. Kol B: 20.11, 15.27a, 15.33, 15.45. Parçalayıcı dilimi: 16.2. Asistan araçları (roadmap, yerel sesli asistan): 18.1'in `TR_Path` denetiminden geçmeyen yolları. Yerli ses girişi (roadmap, yerel sesli asistan 6. adım): 21.2'nin sınırı, kaynağın kendi kaybı, "Gerçek mikrofonda sözün başı". 15.22a'nın sınıfsız (hız) Ö1'i listeye girmez, testin adlı istisnası.

**Sahipli kollar:** (sahip başına bir satır, Ö1 listesinin parçası)
- Motor (dil çekirdeği kolu): 3.3a.
- T1, tensör kolu: 13.8a (Tur 6A'dan sonraki dilim), 15.8 (1.2'den sonraki dilim), 15.24, 15.28, 15.52'nin q4z128 sıfırı, "Windows'ta Go çalışma zamanının bozulması".
- S1: 25.3'ün çok dillilik kuralı, "Yazıyı konuşmaya çevirme" (dil tanımı 27 yazılınca).
- H1: 15.52'nin fp16 ölçeği (nicemleme uygulanırken).
- D1: 17'nin yeniden kullanılan kod numarası (codegraph dilim 5'ten sonra).
- TCB: "v0.2.0 ikilisinin kaynağı".

**Önem taramasının sınırı** (Gemini'nin bağımsız taraması, taban `30b44b01`): tarama "Yapılacaklar"ın hepsini, "Mevcut durum"dan risk sözcüğü geçen cümleleri okudu; sözcüksüz anlatılan bir sessiz yanlış kaçmış olabilir. 16.6 ve 18.9'un (kesilen yazma) sessiz olup olmadığı ölçülmedi.

## Dil tanımının bekleyen bulguları

Uygulamada verilen ve dil tanımı turunda denetlenecek kararlar, dil tanımıyla çelişen bulgular: [`docs/spec/pending.md`](spec/pending.md).

## 1. Program

Kurallar: [`docs/spec/01_program.md`](spec/01_program.md).

### 1.1

- **S-1 Mevcut durum:** modülün sabit ifadesinde dosyanın tanımlamadığı ad (aktarılan ya da genel) sabit sayılıyor [kanıtlı: `internal/lang/checker/const_values.go:8-11`].
- **S-2 Yapılacaklar (D, Ö3):** aktarılan adın sabitliği modüller arası tip denetimiyle (4b) bilinir.

### 1.2

- **S-3 Mevcut durum:** depodaki ilk değerlerin bir kısmının tipi bilinmiyor (`TR_Any`, çoğu çağrı) [test: `TestRepoFirstValueTypes`]; adlı argümanın, metot argümanının ve fonksiyon değeri çağrısının argüman tipi denetlenmiyor [ölçüldü: `ae1e8dcc`].
- **S-4 Yapılacaklar (A, Ö1, 4a5):** çıkarımın kalanı: kitaplık çağrılarının ve birleşim dönüşlü yerleşiklerin dönüşü, imzaların öğe tipleri, tipi bilinmeyen işlenenli işleç, kapanış, çoklu tanımın öğe tipleri, metot ve kurucu argümanları, tensörün tür ve şekli; takma adlı modülün tipi `TR_Any` [kanıtlı: `internal/lang/checker/module_values.go:16-19`].
- **S-5 Mevcut durum:** tipi bilinmeyen değer (öğe tipi bilinmeyen kap da) tipli yere girerken denetimde değil, çalışırken yerin koduyla duruyor (H1001, H1003, H1004, H1005, H1124, H1125, H1136; kapta H1170, H1171) [test: `TestUntypedValuesAreCheckedAtTypedPlaces`, `TestBuiltAppRunsTheUntypedValueCheck`].
- **S-6 Yapılacaklar (D, Ö3; sahip tensör kolu (T1)):** döngünün ve indekslemenin tipi `typeAny` dönüyor [kanıtlı: `internal/lang/checker/local_types.go:234-278`]; `TR_Bytes(...)`'in tipi denetimde bilinmiyor, tipli yere girerken çalışırken duruyor [ölçüldü: `ae1e8dcc`]. Sıradaki: tipi denetimde vermek.
- **S-7 Yapılacaklar (A, Ö1; sahip tensör kolu (T1)):** denetlenen kap tipsiz bir adla paylaşılıyorsa o addan sonra yazılan öğe denetlenmiyor: `l: TR_List = ["a"]`, `g: TR_List[TR_String] = l`, `l.TR_append(1)` sonrası `g` `["a", 1]` [ölçüldü: `ae1e8dcc`]. Sıradaki: kaynağın tipini denetimde bilmek.
- **S-8 Yapılacaklar (D, Ö3; sahip tensör kolu (T1)):** çalışma denetiminin `TR_JSON` kolu her değeri kabul ediyor [kanıtlı: `internal/lang/interp/runtime_checks.go:160-161`]: tipsiz listeden `TR_List[TR_JSON]`'a fonksiyon giriyor, hata ancak JSON'a yazarken geliyor [ölçüldü: `ae1e8dcc`]. Sıradaki: kol öğeleri JSON kuralıyla denetlensin.
- **S-9 Yapılacaklar (D, Ö3; sahip tensör kolu (T1)):** iki yerde tip denetimde bilinmiyor: çekirdek dışı yerleşik tipli alan (`o: TR_Optimizer?` iken `s.o.TR_step()` H1180 yerine çalışırken H2744) [kanıtlı: `internal/lang/checker/local_types.go:225`] ve öğesi programın yapısı olan kabın üye dönüşü (`dn.TR_get("p").x`) [kanıtlı: `internal/lang/checker/member_types.go:61`].
- **S-10 Yapılacaklar (D, Ö3; sahip tensör kolu (T1)):** `?.`'nın sonucunun tipi denetimde bilinmiyor (4.8): `s?.n.x` H1180 vermiyor, `TR_var w: TR_Int = s?.TR_upper()` ve `= s?.TR_length` denetimden geçiyor [ölçüldü: `ae1e8dcc`].

### 1.3

- **S-11 Mevcut durum:** takma adsız `TR_import` olan kapsamda tanımsız ad denetimde aranmıyor, çalışırken hata veriyor (5.14) [ölçüldü: `ae1e8dcc`].
- **S-12 Yapılacaklar (D, Ö3):** takma adsız aktarmada tanımsız ad çalışmadan önce bulunur.

### 1.5

- **S-13 Mevcut durum:** durdurma isteği CPU'daki hesabı bırakmıyor, süren hesap biter; bırakma (`Abandonable`) yalnız okuma ve bekleme yollarında [kanıtlı: `internal/lang/interp/interrupt.go:390-397`, `internal/lang/interp/builtins.go:321`].
- **S-14 Yapılacaklar (E, Ö3):** CPU hesabının ortasında bırakma (1.5); bırakılan hesabın GPU'da başlamış işi beklenir, ertelenen işler sonra çalışır.

### 1.9

- **S-15 Yapılacaklar (D, Ö3; iç çöküş):** Go çalışma zamanının ölümcül hatası 2 koduyla bitiyor, 1.9'un kullanım hatasıyla karışıyor [kanıtlı: go1.27.4 `src/runtime/panic.go:1449`].

## 2. Sözcük yapısı

Kurallar: [`docs/spec/02_sozcuk_yapisi.md`](spec/02_sozcuk_yapisi.md).

### 2.1

- **S-16 Mevcut durum:** ters tırnak içindeki adlar yalnız dil tanımında ve status'ta denetleniyor; öbür belgelerde yanlış kod gösteren ad hiçbir testte düşmüyor [kanıtlı: `internal/devtools/testrunner/behavior_language_spec_test.go:354-362`].
- **S-17 Yapılacaklar (E, Ö3):** ters tırnak denetimi `docs/*.md`'ye genişler (üretilen ve donmuş belgeler, eş adlar tablosunun kod sütunu hariç): sınıflı yazım hata, `.TR_x` yalnız üye, `TR_x:` yalnız adlı argüman.

### 2.3a

- **S-18 Mevcut durum:** kimlik kapısı (`TestReviewedSectionsWriteIdentities`) `TR_` sözcüğünün herhangi bir yerde geçerli olmasını yeterli sayıyor; yanlış yerde yazılan `TR_append(x)` geçiyor [kanıtlı: `internal/devtools/testrunner/behavior_language_spec_test.go:338`].
- **S-19 Yapılacaklar (E, Ö3):** tam ifade olan parça (`(`, `.` ya da `:` içeren) sözcük çözümleyiciyle okunup yeri denetlenir.

### 2.13

- **S-20 Yapılacaklar (D ve F, Ö3):** H2540'ın bitişi yok [kanıtlı: `internal/lang/source/source.go:77`]; okunamayan modülün (H2132, H2131) sütunu ve bitişi yok [kanıtlı: `internal/lang/messages/system_error.go:69-73`]; iç içe modülün hatası dosyayı yazmıyor, aktarılan modülün uyarısı çıkmıyor, `TR_cmd_check` dosyadan sonraki argümanı sessizce atlıyor (1.4) [ölçüldü: `ae1e8dcc`].

### 2.3b

- **S-21 Yapılacaklar (F, Ö3):** harf büyüklüğü olmayan yapı adı (`TR_struct 学生`, `TR_struct طالب`) ad yazılışı uyarısı alıyor, önerisi adın kendisi; böyle karakter her biçime uyar (2.3b) [ölçüldü: `ae1e8dcc`]. Sıradaki: TR testi.

## 3. Tipler ve değerler

Kurallar: [`docs/spec/03_tipler.md`](spec/03_tipler.md).

### 3.3

- **S-22 Yapılacaklar (D, Ö3; sahip dil çekirdeği kolu):** metne indeksle yazma denetimden geçiyor: `s[0] = "x"` çalışırken H2403 (3.3) [ölçüldü: `ae1e8dcc`].
- **S-23 Yapılacaklar (F, Ö3; sahip dil çekirdeği kolu):** çalışma anındaki H2465, H2403, H2421 ve H2469 tip yuvasına kimliği değil görünüm adını yazıyor ("Tam tipi indekslenemez", "type NoneType cannot be called as a function") [kanıtlı: `internal/lang/interp/expressions.go:143`, `assign.go:169`, `calls.go:223`, `operators.go:189`].

### 3.3a

- **S-24 Yapılacaklar (A, Ö1; sahip Motor (dil çekirdeği kolu)):** Windows'ta eşi olmayan yarım karakter WTF-8 baytıyla gelir [kanıtlı: go1.27.4 `src/syscall/syscall_windows.go:58-59`]; ortam değeri ve program argümanı UTF-8 denetimsiz metin oluyor [kanıtlı: internal/stdlib/process/env.go:28-35, `internal/lang/interp/program_args.go:110-122`]; JSON'da sessiz U+FFFD [hipotez]. Sıradaki: `TR_env` ve `TR_command_line` ile ölçmek.
- **S-25 Yapılacaklar (E, Ö3):** ağ istemcisi yok; geldiğinde okumaları `TR_Bytes` da verir (bugün `TR_Bytes` dosyadan okumada ve sunucunun `.TR_body`'sinde) [kanıtlı: `internal/stdlib/httpserver/request.go:41`].

### 3.5

- **S-26 Mevcut durum:** isteğe bağlı ad asıl tipin yerinde denetlenmiyor: daraltma yok, aritmetikte tipi `TR_Any` (`y: TR_Int?` iken `y + 1` çalışırken H2469; 4b) [test: `TestOptionalTypes`; ölçüldü: `ae1e8dcc`].
- **S-27 Yapılacaklar (A, Ö1, 4a; sahip D1):** değer vermeyen fonksiyonun tipi bilinmeyen çağrısı denetimden geçip sessizce `TR_none` veriyor: tipsiz alıcıda `.TR_append`'in dönüşü, ada bağlanan yerleşik (`TR_var f = TR_print`), sözlükte ve listede tutulan fonksiyon, `TR_Function` parametresi, modül fonksiyonu [ölçüldü: `ae1e8dcc`]. Sıradaki: 6.9, sonra 3.6, en son 4b.
- **S-29 Yapılacaklar (D, Ö3, 4a5; sahip D1):** tipli yerde başka tip duran üç yol denetimden geçiyor, çalışırken yerin koduyla duruyor: `TR_catch e`'nin değeri, demet açma (`TR_var a, b = (1, "a")`), öğesiz kap yazımı (`l: TR_List = ["a"]`) [ölçüldü: `ae1e8dcc`].
- **S-30 Yapılacaklar (A, Ö1, 4b):** daraltma yok: isteğe bağlı alan `??` ile açılıyor [kanıtlı: `lib/layers.tr:626-627`]; `l: TR_List[TR_Int]?` iken `TR_if l != TR_none { l[0] } TR_else { TR_none }` ve `TR_var s: TR_String = l[0]` denetimden geçiyor [ölçüldü: `ae1e8dcc`].
- **S-31 Yapılacaklar (D, Ö3; sahip dil çekirdeği kolu):** isteğe bağlı olmayan değerin `TR_none`'la karşılaştırması denetimden geçiyor: `x: TR_Int` iken `x == TR_none` `TR_false` (3.5) [ölçüldü: `ae1e8dcc`].
- **S-32 Yapılacaklar (D, Ö3; sahip D1):** `T?` asıl tipin yerinde denetlenmiyor: `TR_input()`'un değeriyle `TR_Int(m)` ve `.TR_length` denetimden geçiyor, boş girdide çalışma hatası [ölçüldü: `ae1e8dcc`].
- **S-33 Yapılacaklar (A, Ö1, 4a5; sahip T1):** `TR_none` `TR_Int` bekleyen iki yere sessizce giriyor: `(TR_Bool) -> TR_Int` tipli fonksiyon ifadesinin gövdesi ve `x: TR_Int = TR_none` varsayılanı; `[TR_none]` `TR_List[TR_Int]`'e denetimden geçip çalışırken duruyor [ölçüldü: `ae1e8dcc`].

### 3.6

- **S-34 Mevcut durum:** karışık tipli liste denetlenmiyor: `[1, "a"]` öğe tipsiz `TR_List`, `l[0] = "x"` geçiyor [ölçüldü: `ae1e8dcc`]; `tests/tipler_test.tr:133`'ün 3.6 testi karışık tipli listeyi geçerli sayıyor, kural uygulanınca yeniden yazılır; dilim denetlenmiyor, çalışma anı öğe tipini taşımıyor [test: `TestElementAssignType`].
- **S-35 Yapılacaklar (A, Ö1, 4a5):** karışık değişmez denetim hatası olur (önce CSV 13.8 ve `TR_forward:` 15.28'in göçü); boş liste tipini yazar (`TR_var k = []`, 3.13); `TR_List[T]` bir `TR_List[T?]` yerine verilmez; üçü de bugün denetimden geçiyor [ölçüldü: `ae1e8dcc`].

### 3.6a

- **S-37 Yapılacaklar (E, Ö3):** `TR_json_parse[T]` (13.8) motorda yok: denetimden geçiyor, çalışırken "YerleşikFonksiyon tipi indekslenemez" [ölçüldü: `ae1e8dcc`]; demet tipli alanın diziden kurulması onunla gelir.
- **S-38 Yapılacaklar (D, Ö3; sahip D1):** tipi bilinen liste öğesi demetse `l[0][0]` denetimde değil çalışırken hata veriyor (ileti `l[0].0`'ı öneriyor) [ölçüldü: `ae1e8dcc`].

### 3.7

- **S-39 Mevcut durum:** karışık değerli sözlük öğe tipsiz; tipi bilinmeyen anahtarda başka tipte anahtar çalışma anında kalıyor: okuma H2647, `.TR_get` sessizce `TR_none`, `.TR_contains` `TR_false` [test: `TestDictKeyType`; ölçüldü: `ae1e8dcc`].
- **S-40 Yapılacaklar (A, Ö1, 4a5):** değerleri karışık sözlük değişmezi denetim hatası olur (3.6'nın kuralıyla); `{"a": 1, "b": "x"}` bugün geçiyor [ölçüldü: `ae1e8dcc`].
- **S-41 Yapılacaklar (F1, Ö3; sahip dil çekirdeği kolu (Motor)):** H2647'nin önerisi ters tırnağın içinde yer tutucu taşıyor (?? varsayılan, ?? default), kopyası tanımsız ad veriyor [kanıtlı: `internal/lang/messages/tr.json:489`, `en.json:489`]. Sıradaki: H2515'in biçimi ve öneriyi koşturan test.

### 3.7a

- **S-43 Yapılacaklar (D, Ö3):** genel tipler (`TR_Object`, `TR_StructType`, katalogdaki `TR_Any`) kalkar (4a) [kanıtlı: `internal/lang/codes/en.json:34`, `:66`, `:87`].

### 3.10

- **S-44 Mevcut durum:** hata ayıklayıcının koşullu durağı doğruluk sınamasıyla (`IsTruthy`) okunuyor [kanıtlı: `internal/devtools/debugger/dap_breakpoints.go:115`, debugger/eval.go:43]; isteğe bağlı (`TR_Bool?`) koşul denetimden geçiyor, çalışırken hata [ölçüldü: `ae1e8dcc`].
- **S-45 Yapılacaklar (A ve D, Ö1):** `.TR_filter`'ın yükleminin dönüş tipi denetimde bilinir (bugün çalışırken) [ölçüldü: `ae1e8dcc`]; isteğe bağlı koşul daraltmayla (4b); hata ayıklayıcının durağının koşulu yalnız `TR_Bool` alır.

### 3.11

- **S-46 Yapılacaklar (E ve F1, Ö2; sahip D1):** parametreli tip değeri yazılamıyor: `TR_Dict[TR_String, TR_JSON]` ifade yerinde sözdizimi hatası, `TR_List[TR_JSON]` çalışırken "Tip tipi indekslenemez"; ileti doğru yazımı vermiyor [ölçüldü: `ae1e8dcc`]. Karar `pending.md`'nin 3.11'inde.
- **S-47 Yapılacaklar (D, Ö3, 3.11'in dilimi; sahip D1):** parametresiz `TR_List`, `TR_Dict` ve `TR_Task` tip yazımı ve tip değeri olarak kabul ediliyor (`a: TR_List`, `TR_Type(a) == TR_List`) [ölçüldü: `ae1e8dcc`]; `TR_Type(x)` tam tipi verir. Sıradaki: S-46'dan sonra, 3.14'ün eşitliğinden önce.
- **S-48 Yapılacaklar (E, Ö3; sahip D1):** `TR_Dict(d)` sözlükte H2423 veriyor; dil tanımı sığ kopya diyor (3.12) [ölçüldü: `ae1e8dcc`].

### 3.13

- **S-49 Mevcut durum:** tipi çıkarılamayan ilk değerin değişkeni `TR_Any` kalıyor (1.2) [test: `TestRepoFirstValueTypes`].
- **S-50 Yapılacaklar (A, Ö1, 4a5):** ilk değeri `TR_none` ya da boş değişmez olan değişkende tip açıklaması zorunlu; tam sayı ondalık beklenen yerde tam kalıyor: `l: TR_List[TR_Float]`'te `l.TR_append(1)`, `d["a"] = 1`, `TR_var t = 0.0` sonra `t = 1`; `[a, 2.5]` çalışırken H1001 [ölçüldü: `ae1e8dcc`].
- **S-51 Yapılacaklar (D, Ö3; sahip dil çekirdeği kolu):** tipi bilinen ve indekslenemeyen değere indeksle yazma ve okuma denetimden geçiyor: `n = 5` iken `n[0] = 1` çalışırken H2403, `n[0]` H2465 (3.13) [ölçüldü: `ae1e8dcc`].

### 3.14

- **S-53 Mevcut durum:** tipin metni öğe tipsiz: `TR_Type([1])` "List" [ölçüldü: `ae1e8dcc`].
- **S-54 Yapılacaklar (A, Ö1, 4c):** tipin metni "List[Int]", "Int?" olur (4a); değer öğe tipini taşımadığı için `TR_Type([1]) == TR_Type(["a"])` `TR_true` [ölçüldü: `ae1e8dcc`]; parametrenin kaynağı `pending.md`'nin 3.11'inde.

## 4. İşleçler

Kurallar: [`docs/spec/04_operatorler.md`](spec/04_operatorler.md).

### 4.5

- **S-55 Mevcut durum:** tipi bilinmeyen yerde çalışma anı ayrı tipli iki değeri sessizce eşit değil sayıyor [test: `TestDistinctTypesAreNotCompared`].
- **S-56 Yapılacaklar (D, Ö3; sahip D1):** kaptaki JSON ile JSON olmayan değerin H2705'i birleşim-bul zincirinde (`x ~ z ~ y`) kaçabiliyor, sonuç `TR_false`; yalnız bütçeyi aşan, paylaşılan ya da kendine bağlanan genel tipli değerde [kanıtlı: `internal/lang/interp/equality.go:311-315`]. Sıradaki: 15.5 diliminden sonra.

### 4.7

### 4.9

- **S-59 Mevcut durum:** satır sonunda `//`'den sonraki metin ifade değilse ileti `#`'i anmıyor (H0141); tanımsız adlardan oluşuyorsa her ad ayrı hata veriyor, yalnız ilki `#`'i öneriyor [ölçüldü: `ae1e8dcc`].
- **S-60 Yapılacaklar (F, Ö3):** bu durumlarda da ileti `#`'i önerir; tek hata çıkar.

### 4.10

- **S-61 Mevcut durum:** başka tiple `*` (`[1] * "a"`) denetimden geçiyor; tipi bilinmeyen öğede öğe tipleri ayrı iki listenin `+`'sı ve öğeleri paylaşılan listenin `*`'ı çalışırken de hatasız [test: `TestListPlusElementTypes`, `TestSharedRowsAreNotRepeated`; ölçüldü: `ae1e8dcc`].
- **S-62 Yapılacaklar (D ve F, Ö3):** başka tiple `*`'ın denetim hatası; H2609 ve H2614'ün önerisi programın kendi yazımı değil genel kalıp [kanıtlı: `internal/lang/messages/tr.json:459`, `:464`].
- **S-63 Mevcut durum:** yineleme 2³¹ bayt ve 2²⁷ öğede sabit sınırla kesiliyor, kural bellekle denetler (3.6, 15.42) [kanıtlı: `internal/lang/interp/sequence_ops.go:93-94`]; `.TR_append`, `+=` ve sözlüğe ekleme bellekle denetlenmiyor. Kural uygulanınca `tests/operatorler_test.tr:307-308` tersine döner.

## 5. Deyimler ve kontrol yapıları

Kurallar: [`docs/spec/05_deyimler.md`](spec/05_deyimler.md).

### 5.1

- **S-64 Mevcut durum:** fonksiyonun içinde hiç yeniden atanmayan yerel `TR_var` uyarı vermiyor [ölçüldü: `ae1e8dcc`].
- **S-65 Yapılacaklar (F, Ö3):** fonksiyonun içinde yeniden atanmayan yerel `TR_var` uyarısı, ileti `TR_const`'u önerir (programın en dışı girmez); TR testi.

### 5.3

- **S-66 Mevcut durum:** fonksiyon değerinin çağrısında, isteğe bağlı ve tipi bilinmeyen demette öğe sayısının tutmaması çalışırken bulunuyor (H2709); kural demette denetim hatası diyor [test: `TestUnpackCountOfAKnownTuple`].

### 5.4

- **S-70 Yapılacaklar (hız, Ö3; sahip dil çekirdeği kolu (Motor)):** derleyici güvenli olmayan her metot çağrısında `codes.IsInPlace`'e bakıyor, denetimsiz koşuda `definitions.json` ilk metot çağrısında ayrıştırılıyor [kanıtlı: `internal/lang/interp/copy_writes.go:163`, `internal/lang/codes/definition.go:207-213`]; bedeli ölçülmedi.

### 5.5

- **S-72 Mevcut durum:** `TR_elif`'in iletisinin ardından ikinci, yanıltıcı sözdizimi hatası ("':' bekleniyordu") geliyor; dalların tipi denetlenmiyor: `TR_if n < 0 { "a" } TR_else { 5 }` geçiyor [ölçüldü: `ae1e8dcc`].
- **S-73 Yapılacaklar (A, Ö1, 4a5):** ifade olarak kullanılınca dalların tipi aynı değilse denetim hatası; `TR_elif`'e tek ileti; denetleyicinin testi.

### 5.6

- **S-74 Mevcut durum:** eşlemede kapsama denetimi yok (`_`'siz eşleme çalışırken H2702); düz adla yazılan desen hata vermeden değeri karşılaştırıyor; kolların tipi denetlenmiyor [ölçüldü: `ae1e8dcc`].
- **S-75 Yapılacaklar (A, Ö1, 4b):** kapsama denetimi (`TR_Bool`'da iki değer, öbür tiplerde `_`); düz adla yazılan desen denetim hatası, ileti `(x)`'i ya da `_`'yi önerir; ifade olarak kolların tipi aynı; denetleyicinin testi.
- **S-76 Yapılacaklar (E, Ö2; sahip D1):** demet deseninin içinde `_` ve durum deseni sözdizimi hatası ("'_' okunamaz"); asistanın ve HTTP sunucusunun yönlendirmesi buna dayanıyor [ölçüldü: `ae1e8dcc`].

### 5.7

- **S-77 Mevcut durum:** yapı dolaşılamıyor: `TR_next() -> TR_Int?` metodu olan yapıda `TR_for` denetimden geçiyor, çalışırken "döngü kurulamaz" [ölçüldü: `ae1e8dcc`].
- **S-78 Yapılacaklar (E, Ö3; sahip D1):** `TR_next() -> T?` taşıyan yapının dolaşılması, tek seferlik dolaşıcı; `T` isteğe bağlıysa denetim hatası (ileti öğeyi sarmayı önerir); TR testi.
- **S-79 Yapılacaklar (sınıfsız: hız, Ö3; sahip Motor):** iki adlı döngünün demet denetimi (`tupleGuard`) tipi denetimde bilinen döngüde de her turda koşuyor [kanıtlı: `internal/lang/interp/compiler_loop_bind.go:19-21`, `:50-59`]. Sıradaki: bedel ölçülür, gerekirse tipi bilinen döngüde kalkar.
- **S-80 Yapılacaklar (F1, Ö2; sahip Motor (dil çekirdeğinin iletisi)):** `TR_for [a, b] TR_in l` H0206 alıyor, ileti onu üyelik sınaması sayıyor [kanıtlı: `internal/lang/parser/expressions.go:324-328`]; 5.7 açmayı yalnız demette, parantezli desenle tanır. Sıradaki: 5.7'nin kuralını söyleyen ileti ve olumsuz fikstür.

### 5.11

- **S-81 Mevcut durum:** sözlük üreteci yok: `{k: k * 2 TR_for k TR_in 0..3}` sözdizimi hatası [ölçüldü: `ae1e8dcc`].
- **S-82 Yapılacaklar (E, Ö3):** sözlük üreteci, yinelenen anahtarda son değer; TR testi.
- **S-83 Yapılacaklar (E, Ö2; sahip Motor (dil çekirdeği, ayrıştırıcı)):** üreteçte iki değişkenli biçim (`[i TR_for i, x TR_in l]`, 5.11) sözdizimi hatası veriyor [ölçüldü: `ae1e8dcc`]. Sıradaki: ayrıştırıcı ve TR testi (liste ve sözlük üreteci).

### 5.12

- **S-84 Mevcut durum:** yasak yeniden tanım hata vermiyor: iç bloktaki `TR_var t` dıştakini gizliyor, aynı kapsamda ikinci `TR_var b` ve parametreyle aynı adlı gövde değişkeni geçiyor [ölçüldü: `ae1e8dcc`].
- **S-85 Yapılacaklar (A, Ö1, 4b):** kapsayan ya da aynı kapsamdaki yerel adla yeniden tanım denetim hatası (programın en dışı ve parametreler dahil); denetleyicinin testi.

### 5.14

- **S-86 Mevcut durum:** tanımından önce kullanılan ad ve sonra tanımlanan değişkeni okuyan fonksiyon denetimden geçiyor, çalışırken "ad yok"; modülde sonraki sabiti okuyan sabit çalışırken düşüyor, döngülü iki sabit denetimden geçiyor [ölçüldü: `ae1e8dcc`].
- **S-87 Yapılacaklar (D, Ö3):** 5.14'ün tanımından önce kullanma denetimi (modülün en dışı ve sabitler arası döngü dahil); ileti tanımı yukarı almayı önerir; denetleyicinin testi.

## 6. Fonksiyonlar

Kurallar: [`docs/spec/06_fonksiyonlar.md`](spec/06_fonksiyonlar.md).

- **S-697 Mevcut durum:** 6.4'ün eksik ve fazla argüman testi hatayı çalışırken yakalıyor (`tests/fonksiyonlar_test.tr:70-71`, H2414, H2412); denetim hatası olunca `tests/negative/` fikstürüne taşınır.

### 6.1

- **S-88 Yapılacaklar (D, Ö3; sahip D1, Kol A):** `TR_func` adına atama denetimden geçiyor: `f = g` `f`'yi değiştiriyor, `f += 1` çalışırken düşüyor; iç blokta `TR_var f = g` da geçiyor [ölçüldü: ae1e8dcc].
- **S-89 Yapılacaklar (A, Ö1, 4a5):** metot ve kurucu argümanının ve adlı argümanın tipi denetlenmiyor: `TR_Int` parametreli metoda metin veren çağrı hatasız çalışıyor [ölçüldü: ae1e8dcc]; kitaplığın 29 istisnası 4c'de kalkar [kanıtlı: `internal/lang/checker/library_exceptions.go:12-42`].
- **S-90 Yapılacaklar (D, Ö3; sahip Motor (denetleyici ve çalışma anı)):** olmayan tip adı hata değil: `TR_func al(x: BöyleBirTipYok)` yapı değeriyle çağrılınca çalışıyor, `TR_var y: YokTip = 5` yalnız tip uyuşmazlığı alıyor [ölçüldü: ae1e8dcc]. Sıradaki: bilinmeyen tip adı denetim hatası, ileti en yakın tipi önerir.

### 6.2

- **S-91 Mevcut durum:** eksik dönüş denetlenmiyor: `-> TR_Int` fonksiyonunda `TR_return`'süz yol denetimden geçiyor, çalışınca yokluk veriyor [ölçüldü: ae1e8dcc].
- **S-92 Yapılacaklar (A, Ö1, 4b):** bitmeyen yol (S-91) denetim hatası, ileti `TR_return`'ü önerir; bitiren deyimler `TR_return`, `TR_throw`, `TR_exit`, çıkışsız sonsuz döngü, dalları biten `TR_if` ve kolları biten `TR_match`; denetleyicinin testi; döngüden `TR_break` ile çıkan `-> TR_Int` fonksiyonu bugün `TR_none` döndürüp denetimden geçiyor [ölçüldü: ae1e8dcc], test bunu ve iç içe biten dalları da sınar.

### 6.3

- **S-94 Mevcut durum:** açılan adlar tipsiz: `TR_var a, b = f()` sonrası `b`'nin tipi denetlenmiyor, tipli yere girişi çalışırken duruyor [ölçüldü: ae1e8dcc].
- **S-95 Yapılacaklar (D, Ö3, 4a5):** açılan adların tipi dönüş tipinin öğe tiplerinden alınır (S-94).

### 6.4

- **S-97 Yapılacaklar (A, Ö1, 4a5):** varsayılanın tipi denetlenmiyor: `TR_func f(x: TR_Int = "a") -> TR_Int`'te `f()` `"a"` veriyor, denetim ve çalışma hatasız [ölçüldü: ae1e8dcc]. Sıradaki: uymayan varsayılan denetim hatası.

### 6.5

- **S-98 Mevcut durum:** eksik, fazla, bilinmeyen adlı ve hem sırayla hem adla verilen argüman ancak çalışırken bulunuyor (H2414, H2412, H2410, H2413); adlı argümanın tipi denetlenmiyor; parametre bekleyen yerleşiğe verilen sondaki blok çalışırken H2711 [ölçüldü: ae1e8dcc].
- **S-99 Yapılacaklar (A, Ö1, 4a5):** S-98'in argüman hataları denetim hatası olur, ileti parametrenin adını söyler; kitaplık fonksiyonunun bilinmeyen adlı argümanı da çalışırken H2410 [kanıtlı: `tests/negative/gemma3_araclar.tr:5`]. Sıradaki: o fikstür `# ERROR` olur, `TR_generate`'in H7172–H7174'ü yeniden tartılır.
- **S-100 Yapılacaklar (D, Ö3; sahip dil çekirdeği, denetleyici):** tipi bilinen, fonksiyon olmayan değeri çağırmak denetimden geçiyor: `p.TR_exists()` çalışırken H2421 veriyor [ölçüldü: ae1e8dcc]. Sıradaki: denetim hatası, ileti parantezsiz yazımı önerir.

### 6.6

- **S-101 Mevcut durum:** adı üzerinden gelen yokluk görülmüyor: `TR_var n = TR_none` ve `(x: TR_Bool) => { TR_return n }` denetimden geçiyor [ölçüldü: ae1e8dcc].
- **S-102 Yapılacaklar (E ve D, Ö3; sahip D1, 4a5 ile):** bir yolu `T`, öbürü `TR_none` döndüren fonksiyon ifadesi H1145 alıyor [ölçüldü: ae1e8dcc]; dönüşü `T?` olur, yalnız `TR_none` dönen gövde hata. Ortak tipin birleştirmesi 4a5'e bağlı.
- **S-103 Mevcut durum:** yerleşiğe verilen fonksiyon ifadesinin parametre tipi beklenen tipten çıkarılmıyor: `l.TR_map((x) => ...)`'te `x` tipsiz kalıyor, tipli yere girişi çalışırken duruyor [ölçüldü: ae1e8dcc].
- **S-104 Yapılacaklar (A, Ö1, 4a5):** parametre tipinin beklenen tipten çıkarımı ve denetimi, yerleşik ve kitaplık imzalarıyla (13.9; S-103); denetleyicinin testi.

### 6.8

- **S-105 Mevcut durum:** dıştaki değişkene okumadan önce atayan fonksiyon uyarı almıyor [ölçüldü: ae1e8dcc].
- **S-106 Yapılacaklar (F, Ö3):** dıştaki değişkene okumadan atamada uyarı, ileti `TR_var`'ı önerir (S-105); TR testi.

### 6.9

- **S-107 Mevcut durum:** fonksiyon tipi denetlenmiyor: `(TR_Int) -> TR_Bool` tipli ada sayı döndüren ok atanıyor; fonksiyon değeri adlı argümanla çağrılınca H2710; metni "<function f>" ve "<lambda>", `TR_Type(f)` Function [ölçüldü: ae1e8dcc].
- **S-108 Yapılacaklar (A, Ö1, 4a5 ya da 4c):** fonksiyonun metni tipinin metni ("(Int) -> Bool"), `TR_Type(f)` fonksiyon tipi; parametre ve dönüş tiplerinin denetimi (S-107); genel fonksiyon tipi kimlik listesinden kalkar (3.11); TR testi.

### 6.10

- **S-109 Mevcut durum:** tip parametresi yok: `TR_func baştaki[T](l: TR_List[T]) -> T?` sözdizimi hatası (H0140); `f[TR_Int](3)` denetimden geçiyor, çalışırken "Fonksiyon tipi indekslenemez" [ölçüldü: ae1e8dcc].
- **S-110 Yapılacaklar (E ve D, Ö3):** tip parametreleri ve çağrıda çıkarımı (S-109); tip yazımı liste öğesinin çağrısından ayrılır; kısıtsız `T`'de yalnız her tipte geçerli işler (13.10, 7.8); TR testi.
- **S-111 Mevcut durum:** tip yerindeki `Kutu[TR_Int]` `Kutu`'ya indirgeniyor: `Kutu[TR_String]` tipli ada `Kutu[TR_Int]` tipli değer denetimden geçiyor [ölçüldü: ae1e8dcc].

## 7. Yapılar

Kurallar: [`docs/spec/07_yapilar.md`](spec/07_yapilar.md).

- **S-698 Yapılacaklar:** yapının tipi `TR_Type` (3.11), genel tipler kalkar; `tests/yapilar_test.tr:165` `TR_StructType`'ı sınıyor, örnekler `TR_StructType`, `TR_Function`, `TR_BuiltinFunction` ve `TR_Object`'i kullanıyor [kanıtlı: `examples/tr/12_tip_adlari.tr:21`, `examples/tr/14_yapilar_ve_metotlar.tr:56`].

- **S-699 Yapılacaklar:** alan daraltılmadığı için (3.5) `lib/`'deki üç `TR_self.alan != TR_none` sınaması yerel kopyaya ya da `?.`'ya çevrilir [kanıtlı: `lib/layers.tr:603`, `lib/optimizers.tr:403`, `:416`].

### 7.1

- **S-112 Mevcut durum:** yapının tip parametresi yok: `TR_struct Kutu[T]` sözdizimi hatası [ölçüldü: ae1e8dcc]; kitaplığın iki alanı (GPT'nin ayar sözlüğü, Muon tutan alan) istisnalı [kanıtlı: `internal/lang/checker/library_exceptions.go:13`, `:26`].
- **S-113 Yapılacaklar (E, Ö3):** yapının tip parametresi (6.10 ile, S-112); kitaplığın iki alanının istisnası 4c'de kalkar; TR testi.

### 7.2

- **S-114 Mevcut durum:** varsayılansız alan verilmezse, fazla ya da bilinmeyen argümanda ve iki kez verilen alanda hata ancak çalışırken (H2750, H2419, H2418, H2420) [ölçüldü: ae1e8dcc]; `tests/yapilar_test.tr`'nin 7.2, 7.4 ve 7.5 testleri bu hataları çalışırken yakalıyor (ör. `tests/yapilar_test.tr:66-68`), denetim hatası olunca `tests/negative/`'e taşınır.
- **S-115 Yapılacaklar (A, Ö1, 4a5):** zorunlu alanın verilmemesi ve 6.5'in argüman hataları denetim hatası (S-114); kurucusu olan yapıda kurucunun parametre tipleri (7.4); denetleyicinin testi.

### 7.3

### 7.4

- **S-117 Mevcut durum:** kurucunun imzasına uymayan kurma ancak çalışırken bulunuyor; kurucunun atamadığı varsayılansız alan yokluk kalıyor, atanmadan okunması ve önceden metot çağrısı hata vermiyor [ölçüldü: ae1e8dcc].
- **S-118 Yapılacaklar (A, Ö1, 4b):** kurucu bitince her yolda varsayılansız her alan atanmış olur; atanmadan okuma ve önceden metot çağrısı denetim hatası; kurma kurucunun imzasıyla denetlenir (6.5; S-117); denetleyicinin testi.

### 7.5

- **S-119 Mevcut durum:** olmayan alana ve metoda atama ancak çalışırken bulunuyor (H2405, H2404); ileti alanla eşit uzaklıktaki metodu öneriyor (`p.y` için `m`) [ölçüldü: ae1e8dcc].
- **S-120 Yapılacaklar (D ve F1, Ö3):** olmayan alana ve metoda atama denetim hatası, ileti en yakın alanı önerir (S-119); denetleyicinin testi.

### 7.6

- **S-121 Mevcut durum:** yapının tipi StructType: `TR_Type(P)` StructType yazıyor [ölçüldü: ae1e8dcc].
- **S-122 Yapılacaklar (A, Ö1, 4a5 ya da 4c):** yapının tipi `TR_Type` (3.11; S-121); denetleyicinin testi.
- **S-123 Yapılacaklar (F, Ö3; test kapsamı; sahip TCB, 7.6'nın `TR_same_object`'iyle):** `lib/optimizers_test.tr`'nin 14 yeri parametrenin grupta olduğunu kimlikle değil değerle sınıyor; aynı değerli iki parametre testi düşürür [kanıtlı: `lib/optimizers_test.tr:79-88`]. Sıradaki: `TR_same_object` gelince ona geçer.
- **S-124 Yapılacaklar (E, Ö3; sahip Motor):** 7.6'nın nesne kimliği yok: `TR_same_object` kod listesinde yok [kanıtlı: `internal/lang/codes/definitions.json`]. Sıradaki: kimlik kod listesine girer; öbür tiplerin ve `TR_none`'ın denetim iletilerini uygulayana TCB verir.

### 7.7

- **S-125 Mevcut durum:** durumlu tipte tip parametresi yok (`TR_enum Cevap[T]` sözdizimi hatası); kapsama denetimi yok (`_`'siz eksik eşleme çalışırken H2702); bağlanan ad dıştakini gizliyor; alanın tipi denetlenmiyor [ölçüldü: ae1e8dcc].
- **S-126 Yapılacaklar (A, Ö1, 4a5 ya da 4c):** durumlu tipte tip parametresi (6.10 ile), kapsama denetimi (5.6), bağlanan adın 5.12 hatası, alan tipinin denetimi (S-125); TR testi.
- **S-127 Mevcut durum:** yanlış yazılan durum kurucusu (`C.Tamam`, `C.Yok()`, olmayan durum) ancak çalışırken hata veriyor [ölçüldü: ae1e8dcc].

### 7.8

- **S-128 Mevcut durum:** arayüz denetlenmiyor: metodu eksik değer ancak çağrıda çalışma hatası veriyor, imzası tutmayan metotlu değer hatasız geçiyor; kısıtlı tip parametresi (`TR_func g[T: Katman]`) sözdizimi hatası [ölçüldü: ae1e8dcc].
- **S-129 Yapılacaklar (A, Ö1, 4a5, tip parametresinin kısıtı 4c ile):** arayüzü karşılamayan değer denetim hatası, ileti eksik ya da imzası tutmayan metotları söyler (S-128); tip parametresinde kısıt (6.10 ile); daraltma denetleyicide 3.5'in koşullarıyla; denetleyicinin testi.
- **S-130 Yapılacaklar (F1, Ö3; sahip Motor):** arayüzün metodu eksik değerde çalışma hatası yapıya ileri: TR_Int alanını eklemeyi öneriyor; eksik olan bir metottur [ölçüldü: ae1e8dcc].

## 8. Hata yönetimi

Kurallar: [`docs/spec/08_hata_yonetimi.md`](spec/08_hata_yonetimi.md).

### 8.1

- **S-131 Yapılacaklar (D, Ö3; sahip D1, 2b'den sonra):** sondaki bloğun son ifadesi `TR_try` ise değer sayılmıyor: `TR_var y = TR_no_grad { TR_try { 1 } TR_catch e { 2 } }` H1023 veriyor [ölçüldü: ae1e8dcc]; 6.7 ve 8.1 ikisini de değer sayar.
- **S-132 Yapılacaklar (A, Ö1, 4a5):** denetleyici ifade olarak `TR_try`'ın bloklarına inmiyor: içindeki tanımsız ad ve farklı dal tipleri denetimden geçiyor [ölçüldü: ae1e8dcc]; adsız, parantezli ad, except, finally ve raise yazımlarının olumsuz fikstürü yok.

### 8.2

### 8.3

- **S-135 Mevcut durum:** hata değerlerinin `e == f`'si denetimden geçiyor, çalışırken H2913 [ölçüldü: ae1e8dcc].
- **S-136 Yapılacaklar (D, Ö3, 4a5):** `TR_value`'nun kapalı birleşim tipi ve `e == f`'nin denetim hatası (4a); kopyanın daraltılması (4b), `_`'siz eşlemenin kapsaması (4c); bugün metin taşıyan `e.TR_value` `TR_Int` tipli ada denetimden geçiyor [ölçüldü: ae1e8dcc].
- **S-138 Yapılacaklar (F, Ö3; sahip T1):** kayıt alanını çağıran yazımda (`.TR_text("tr")`) H2421 alanın adını ve parantezsiz yazımı söylemiyor, öneri boş [kanıtlı: `internal/lang/interp/calls.go:223`].

### 8.5

- **S-140 Yapılacaklar (F, Ö3):** programın ayırması beklenen hataların kodları dil tanımının maddelerinde yazılı değil (8.5): 1.6'nın durdurma istendi hatası H2530 [kanıtlı: `internal/lang/interp/interrupt.go:332`]; 18'in dosya hataları.
- **S-142 Yapılacaklar (F1, Ö3; sahip Motor):** `fs.ErrClosed`, `fs.ErrInvalid` gibi bilinen Go hataları H2629'a iniyor, ileti nedeni söylemiyor [kanıtlı: `internal/lang/messages/system_error.go:68-90`]. Sıradaki: bilinen Go hataları kendi kodlarına iner.
- **S-143 Yapılacaklar (C, Ö2; sahip Motor):** `TR_input`'un okuma hatası nedeni metne çevirip H2507'ye veriyor; programın gördüğü kod iç nedenin değil H2507 [kanıtlı: `internal/lang/interp/builtins.go:86`]. Sıradaki: `messages.SystemError("", readErr)`; bekçinin istisnası (`internal/lang/messages/behavior_system_errors_test.go:26`) silinir.

### 8.6

- **S-145 Mevcut durum:** yakalanmayan hata yalnız fırlatıldığı satırı gösteriyor, çağrı yığını yok [ölçüldü: ae1e8dcc].
- **S-146 Yapılacaklar (F, Ö3):** yakalanmayan hatada çağrı yığını, en son çağrı en altta (8.6; S-145); yeniden fırlatılan hata ilk fırlatıldığı yerin çağrı yığınını korur (8.2).
- **S-147 Yapılacaklar:** yakalanmayan hatada nedenler zinciri gösterilmiyor: `TR_cause`'u olan hata yalnız kendi metniyle yazılıyor [ölçüldü: ae1e8dcc].
- **S-148 Yapılacaklar (F, Ö3; sahip Motor, cmd/trlang/build.go, görünen metin değiştiği için tasarımla):** üretilen uygulamada gömülü modülün çalışma hatası modülün adıyla değil geçici kopyanın yoluyla (kullanıcının adı dahil) gösteriliyor [ölçüldü: ae1e8dcc, trlang build]; kopyalar `writeEmbeddedModules`'ta yazılıyor [kanıtlı: cmd/trlang/build.go:443].
- **S-149 Yapılacaklar (F, Ö3; sahip D1):** terminalde gösterilen kodlu hatada kod yazılmıyor: `TR_throw TR_Message("H2132")` yalnız satırı ve metni basıyor [ölçüldü: ae1e8dcc].

## 9. Ertele

Kurallar: [`docs/spec/09_ertele.md`](spec/09_ertele.md).

### 9.1

- **S-150 Yapılacaklar (D, Ö3):** bir bloğun son deyimi olan `TR_defer` denetim hatası değil, çalışıyor [ölçüldü: ae1e8dcc].

### 9.4

- **S-151 Yapılacaklar (F, Ö3):** yakalanmayan hatada bastırılanlar kodsuz: `TR_cmd_run` onları ayrı blokta, `TR_cmd_test` tek birleşik metinde (H2519) gösteriyor [ölçüldü: ae1e8dcc].

## 10. Eşzamanlılık

Kurallar: [`docs/spec/10_eszamanlilik.md`](spec/10_eszamanlilik.md).

### 10.1

- **S-152 Mevcut durum:** dönüşü bilinmeyen çağrının görevi parametresiz `TR_Task`: `.TR_wait()`'in tipi bilinmiyor, tipli yere verilebiliyor, çalışırken duruyor [ölçüldü: ae1e8dcc].
- **S-153 Yapılacaklar (A, Ö1, 4a2 ve 6.6; pending 3.11):** dönüş tipi çıkarılmamış fonksiyon ifadesiyle başlatılan görev `TR_Task[TR_Int]`'e atanıp metin taşıyor, hatasız [ölçüldü: ae1e8dcc].

### 10.2

- **S-155 Yapılacaklar (C, Ö2, Kol B, K'nin kalanı):** GPU'daki `@` ve bileşik yerleşikler (`TR_backward`) yorumlayıcı kilidini tutuyor, uzun işte başka görev bekliyor [kanıtlı: `internal/stdlib/ml/unlocked.go:53-54`, `:104`]. Kalan: yerinde yazan sık yollar (`+=`, indeksle atama, `.TR_step()`), veri yerleşikleri ve geri çağıran yerleşikler (`TR_fuse`, `TR_grad_check`) da kilidi bırakıp izin beklemeli. Sıradaki: GPU'da izin kuyruğun bitişine bağlanır; dilim önce ve sonra ölçer.
- **S-157 Yapılacaklar (F, Ö3; sahip H1):** çakışmada hazırlık yeniden yapılınca önceki çekirdeğin çıktısı ve bitişik kopyaları havuza dönmüyor, çöp toplayıcıya kalıyor; yalnız başka görev aynı depoyu tutarken [kanıtlı: `internal/stdlib/ml/unlocked.go:23-41`, `internal/compute/tensor/access.go:150-155`].
- **S-158 Yapılacaklar (test, Ö3; sahip T1):** çekirdeğin ortasında kesilmemenin (durdurma, `TR_exit`, iptal) testi yok; kesilmeye yalnız kilidin geri alınışında bakılıyor [kanıtlı: `internal/lang/interp/interrupt.go:285-290`].

### 10.3

### 10.6

- **S-160 Mevcut durum:** iptalden önce olan kendi hatanın program sonunda yazılmasının testi yok [kanıtlı: `docs/spec/10_eszamanlilik.md:45`].

### 10.7

### 10.9

- **S-162 Yapılacaklar (C, Ö2):** 10.9'un testi olmayan üç kısmı: durum değiştiren yerleşiğin iptalde bitmesi, alt görevi bekleyen başka görev, `TR_handle_stop` [kanıtlı: `docs/spec/10_eszamanlilik.md:64`].

### 10.10

- **S-164 Yapılacaklar (E, Ö3; sahip Motor (tasarım ve uygulama)):** kuyruk (`TR_Queue[T](n)`, `.TR_put`, `.TR_get`, `.TR_close()`) motorda yok, kimlik bilinmiyor (H0001) [ölçüldü: ae1e8dcc]; yazımı ifade yerinde parametreli tipe bağlı (3.11).

## 11. Modüller

Kurallar: [`docs/spec/11_moduller.md`](spec/11_moduller.md).

### 11.1

- **S-165 Mevcut durum:** takma adsız `TR_import` ile alınan fonksiyonun argüman tipi denetlenmiyor ve çalışırken de bakılmıyor: başka dosyadan `ikile("ab")` `abab` veriyor; aynı çağrı aynı dosyada denetim hatası [ölçüldü: ae1e8dcc].
- **S-166 Yapılacaklar (A, Ö1, 4b; modüller arası tip denetimi; sahip dil çekirdeği kolu (Motor)):** denetleyici aktarılan fonksiyonun imzasını bilir, yanlış tipli çağrı denetim hatası (S-165); olumsuz fikstürler: başka dosyadan yanlış tipli argüman, `TR_Tensor?` alanın `TR_Tensor` parametresine geçmesi.
- **S-167 Yapılacaklar (D, Ö3; sahip D1, modül kuralları):** aktarılan modülün aktardığı adlar aktaranda görünüyor [ölçüldü: ae1e8dcc]; karar `docs/spec/pending.md`'de, dil tanımına girmedi. Göç: `lib/whisper.tr` `tr/layers`'ı doğrudan da aktarıyor.
- **S-168 Yapılacaklar (F1, Ö3; sahip dil çekirdeği):** göreli Windows kısa (8.3) adla aktarılan var olan modül H2112 alıyor: harf büyüklüğü denetimi klasör listesinde kısa adı bulamıyor [kanıtlı: `internal/lang/interp/library.go:175`]. Sıradaki: `diskSpelling` kısa adı uzun ada çevirir.

### 11.2

- **S-169 Mevcut durum:** modül değerine değişkenle köşeli ayraç (`m[k]`) ve olmayan ad ancak çalışırken hata (H2904) [ölçüldü: ae1e8dcc].
- **S-170 Yapılacaklar (D, Ö3, 4a5):** modül değerinde köşeli ayraç (H2904), olmayan ad ve modüle özel ad (`m._gizli`, 11.7) denetim hatası, ileti en yakın adı önerir; olumsuz fikstür. Okurken sözlük gibi: `TR_for a TR_in m` adları dönüyor, `TR_print(m)` sözlük yazımı veriyor [ölçüldü: ae1e8dcc].

### 11.3

- **S-171 Yapılacaklar (D, Ö3; sahip dil çekirdeği (Motor)):** döngüsel aktarım yalnız çalışırken hata: check döngülü programı geçiriyor, run H2481 veriyor [ölçüldü: ae1e8dcc]; denetim zincirdeki dosyada hatasız dönüyor [kanıtlı: `internal/lang/interp/module_check.go:89-94`]. Sıradaki: denetim zincirdeki dosyada H2481 verir.
- **S-172 Yapılacaklar (F1, Ö2; sahip dil çekirdeği (Motor)):** trlang build kendini gösteren bağlantıda H2481 yerine H2726 veriyor: `collect` Moved'da zincire bakmadan dönüyor [kanıtlı: cmd/trlang/build.go:218-220]; `cmd/trlang/build_test.go`'da döngü durumu yok. Sıradaki: `collect` yükleme zincirini tutar.

### 11.5

### 11.7

- **S-174 Yapılacaklar (D, Ö3; sahip D1, modül kuralları):** başka modüldeki yapının `_` ile başlayan alanı ve metodu erişiliyor [ölçüldü: ae1e8dcc]; paket kuralı `docs/spec/pending.md`'de, 11.7 bugün yalnız üst düzey adlara uygulanıyor. Göç: paketin dışından erişen testler `lib/<modül>_test.tr`'ye taşınır.
- **S-175 Yapılacaklar (E, Ö3; sahip Motor (dil çekirdeği)):** H6998'in imleci terminalde kodlu sözcüğün altında tek ^, tanımsız ad iletisi bütün sözcüğü gösteriyor; JSON'daki aralığı bütün sözcük [ölçüldü: ae1e8dcc, check ve `--json`]. Sıradaki: terminal imleci aralık boyunca çizer; test imlecin boyunu sınar.

### 11.8

- **S-176 Mevcut durum:** iki aktarım aynı adı getirince sonraki sessizce kazanıyor, dosyanın kendi tanımı aktarılanı sessizce gölgeliyor; takma adsız aktarılan ada atama ancak çalışırken hata [ölçüldü: ae1e8dcc]. Kural girince `tools/assistant/screen_demo.tr:19` göçer.
- **S-177 Yapılacaklar (A, Ö1, 4a5):** çakışma ve aktarılan ada atama denetim hatası (S-176), ileti iki yeri gösterir ve `TR_as`'ı önerir; olumsuz fikstürler.

## 12. Testler

Kurallar: [`docs/spec/12_testler.md`](spec/12_testler.md).

### 12.1

### 12.2

- **S-179 Mevcut durum:** aynı dosyada aynı adlı iki test hatasız, ikisi de çalışıyor [ölçüldü: ae1e8dcc].
- **S-180 Yapılacaklar (D, Ö3):** aynı adlı iki test sözdizimi hatası, ileti iki satırı gösterir (S-179); olumsuz fikstür.

### 12.3

### 12.4a

### 12.3, 12.4

### 12.5

- **S-184 Yapılacaklar:** hatalı test bitmemiş görevlerini iptal etmiyor (12.5): sonsuz görev başlatıp düşen test asılı kalıyor [ölçüldü: ae1e8dcc, trlang test].

## 13. Yerleşikler

Kurallar: [`docs/spec/13_yerlesikler.md`](spec/13_yerlesikler.md).

### 13.1

- **S-186 Yapılacaklar (D ve F, Ö3):** `TR_write_file`'a `TR_JSON` değeri denetimden geçiyor, çalışırken H2623; önerisi yer tutuculu `TR_json_string(...)` [ölçüldü: ae1e8dcc]. Sıradaki: denetim hatası, öneri programın ifadesiyle (`TR_json_string(j)`).
- **S-187 Mevcut durum (ölçülmemiş):** `internal/detmath`'in `TestGoldenBits`'i gerçek bir ARM64 makinesinde koşmadı; CI yalnız Windows ve Linux amd64'te [kanıtlı: `.github/workflows/ci.yml:37`].
- **S-188 Yapılacaklar (A, Ö1, 4c):** öğe tipi bilinmeyen boş listenin `TR_sum`'ı hata yerine 0 veriyor; `TR_sum[TR_Float]([])` denetimden geçip çalışırken H2465 veriyor [ölçüldü: ae1e8dcc]. Sıradaki: tip parametresi denetimden çalışma anına gelir (`pending.md`'nin 3.11'i).
- **S-191 Yapılacaklar (E, Ö3; sahip dil çekirdeği kolu (Motor)):** girdi sonu durumu `inputEnded` hiç sıfırlanmıyor [kanıtlı: internal/lang/interp/input.go:37, tek atama]; etkileşimli kabuğun yeni isteminde de sürer [hipotez; terminalde koşulmadı]. Sıradaki: kabuk her istemde sıfırlar, Go testi.

### 13.2

- **S-192 Mevcut durum:** argüman sayısı iletisinin kullanım satırı `.TR_strip(karakterler)`'i bilmiyor, `.TR_strip()` yazıyor: `memberForms`'ta `MemStrip` yok [kanıtlı: `internal/lang/hint/arg_counts.go:100`].

### 13.3

- **S-193 Yapılacaklar (A, Ö1, 4a2):** denetim anahtar fonksiyonunun parametre sayısına ve anahtarın tipine bakmıyor: `TR_max([1, 2, 3], k)`'da metin parametreli `k` ve iki parametreli fonksiyon denetimden geçiyor, çalışırken düşüyor (H2558) [ölçüldü: ae1e8dcc].

### 13.4

- **S-194 Mevcut durum (hız sınırı; sınıfsız, Ö3):** sözlükten silmenin bedeli anahtar sayısıyla doğrusal, hepsini silmek karesel: silme sırada arıyor ve kaydırıyor [kanıtlı: `internal/lang/interp/removal.go:73-74`]. Sıradaki: sırada delik ve konum tablosu.

### 13.6

- **S-195 Mevcut durum:** parantezsiz yerleşik metot (`TR_item`, `TR_sort`, `TR_upper`) ve kullanıcı metodu denetimden geçip tipli yere giriyor, çalışırken H1001; `TR_print` `<builtin item>` basıyor [ölçüldü: ae1e8dcc].
- **S-196 Yapılacaklar (A, Ö1, 4a5; sahip dil çekirdeği kolu (Motor)):** yerleşikte parantezsiz metot denetim hatası olmuyor; kullanıcı metodunun çağrısız değeri fonksiyon beklenmeyen tipli yere giriyor (S-195). Sıradaki: denetim hatası, ileti parantezli çağrıyı önerir; olumsuz fikstür (dönüş, tipli değişken, argüman).
- **S-678 Yapılacaklar (A, Ö1, 4a5; sahip dil çekirdeği kolu (Motor)):** denetleyici tensör üyelerinin tipini bilmiyor (1.2'nin bilinmeyen tipi): `TR_var y: TR_Int = TR_ones([2]).TR_tolist` denetimden geçiyor, çalışırken H1001 [ölçüldü: ae1e8dcc].

### 13.7

- **S-197 Mevcut durum:** `TR_is_nan` yok (bilinmeyen kimlik); `TR_is_finite` `TR_fuse` içinde float32 0/1, dışında mantık tensörü veriyor, 15.16a ikisinde de `TR_Tensor[TR_Bool]` diyor [ölçüldü: ae1e8dcc] [kanıtlı: `docs/spec/15_tensorler.md:212`].
- **S-198 Yapılacaklar (E ve D, Ö3; sahip D1, yeni kimlik ister):** `TR_is_finite(TR_Tensor([1, 2]))` hatasız mantık tensörü veriyor, 13.7 tensörde yalnız `TR_Float` alır [ölçüldü: ae1e8dcc]. Sıradaki: `TR_is_nan`'ın sayı ve tensör imzası, `TR_fuse`'ta da; TR testi.

### 13.8

- **S-200 Mevcut durum:** sözlük üreteci (`{k: v TR_for k, v TR_in d}`, 5.11) ve iki değişkenli liste üreteci (`[i + x TR_for i, x TR_in l]`) sözdizimi hatası [ölçüldü: ae1e8dcc].
- **S-201 Yapılacaklar (A, Ö1, 18.4 ve 6.10):** `TR_json_parse[T]` motorda yok; `null` alanı, iç içe yapı ve tensör alanının hatası uygulanmadı: `TR_json_parse[A](...)` çalışırken H2465 [ölçüldü: ae1e8dcc].
- **S-202 Yapılacaklar (E, Ö3; sahip D1, Kol A 13):** yapı JSON'a yazılamıyor: `TR_json_string(K(3))` çalışırken "K değeri JSON'a yazılamaz" veriyor [ölçüldü: ae1e8dcc]. Sıradaki: `TR_json_parse[T]` ile birlikte sayı alanında 3.7a'nın açma kuralı, kurulamayan yapının denetim hatası, döngülü nesnenin hatası.
- **S-203 Yapılacaklar (D ve F, Ö3; sahip D1, Kol A 13):** `TR_json_parse[Ayar]` denetimden geçiyor, çalışırken H2465 iç adı sızdırıyor ("YerleşikFonksiyon tipi indekslenemez") [ölçüldü: ae1e8dcc]. Sıradaki: uygulanana kadar 13.8'i anan denetim hatası.

### 13.8a

- **S-204 Mevcut durum:** `t.TR_tolist` bir özellik, 0 boyutta sayı, 2+ boyutta iç içe liste veriyor; dil tanımının çağrı yazımı `t.TR_tolist()` çalışırken H2421 [ölçüldü: ae1e8dcc]. Çağrısız yazım 156 dosyada 1510 yerde [kanıtlı: `git grep -cP` ile sayım].
- **S-205 Yapılacaklar (A, Ö1, 4a):** `TR_tolist` yalnız 1 boyutta, öğe tipi tensörün cinsi olan liste vermeli; bugün 0 boyutlu tensörün `TR_tolist`'i liste tipli değişkene ondalık koyuyor, denetimden geçiyor [ölçüldü: ae1e8dcc]. Sıradaki: sayım ve çalıştırarak sınıflayan göç aracı (sahip T1); 0 ve 2+ boyutlu kullanımlar `.TR_item()`'a ya da dolaşmaya döner.
- **S-679 Yapılacaklar (A, Ö1, 4a):** `TR_named_parameters` yok (bilinmeyen kimlik); dönüşü `TR_Dict[TR_String, TR_AnyTensor]` olacak, bugün `TR_parameters`'ın dönüşü `TR_named:`'e bağlı (`TR_List|TR_Dict`) [ölçüldü: ae1e8dcc].
- **S-680 Yapılacaklar (F, Ö3, 4a):** H3068'in metni "Item" diyor, `.TR_item()`'ı kimliğiyle anmıyor [kanıtlı: `internal/lang/messages/tr.json:649`].

### 13.9

- **S-206 Mevcut durum:** katalog imzaları çoğunlukla öğesiz (`TR_List`, `TR_Dict`); fonksiyon tipi tek `TR_Function`, yerleşiğin tipi `TR_BuiltinFunction`, genel tip `TR_Any` [kanıtlı: `internal/lang/codes/definitions.json`, `params` ve `returns` alanları].
- **S-207 Mevcut durum:** `.TR_get`, `.TR_keys`, `.TR_length`'in katalogdaki üye tipleri `TR_JSON` ve `TR_Bytes`'ı eksik yazıyor, denetleyici onlara güvenmiyor [kanıtlı: `internal/lang/checker/builtin_calls.go:20-24`; test: `TestMemberTypesMatchDefinitions`].
- **S-208 Yapılacaklar (A, Ö1, 4a):** öğesiz imza ve tek fonksiyon tipi yüzünden tipli yere yanlış tip giriyor: `TR_var g, s = TR_rfft(x)` sonrası `TR_var y: TR_Int = g` denetimden geçiyor, çalışırken H1001 [ölçüldü: ae1e8dcc]. Sıradaki: imzalar öğe ve fonksiyon tipleriyle, `TR_BuiltinFunction` ve `TR_Any` kalkar, dönüş seçilen imzadan (4a5).
- **S-209 Yapılacaklar (D, Ö3, 4a2):** yerleşiğin fonksiyon değeri olarak eşleşmesi denetlenmiyor: adlı sınır isteyen `TR_clamp` `l.TR_map(TR_clamp)`'te denetimden geçiyor, çalışırken düşüyor [ölçüldü: ae1e8dcc].

### 13.10

- **S-210 Mevcut durum:** katalogda dönüşü birleşim olan yerleşikler var (`TR_abs`, `TR_floor` `TR_Int|TR_Float|TR_Tensor`; `.TR_item`); dönüşü argümanın değerine bağlı iki yer: `TR_tolist` ve `TR_parameters` (`TR_named:`) [kanıtlı: `internal/lang/codes/definitions.json`].
- **S-211 Yapılacaklar (A, Ö1, 4a):** imzalarda 13.10'un sekiz kısıtı, kısıtlı tip parametreleri, ayrık imzalar ve değişken sayıda argümanın yazımı yok: `TR_var y: TR_Int = TR_abs(-1.5)` denetimden geçiyor, çalışırken H1001 [ölçüldü: ae1e8dcc]. Sıradaki: değere bağlı dönüşlerin sabit dönüşlü yerleşiklere ayrılması (15 ve 16 ile).
- **S-212 Mevcut durum:** 15'in imzaları katalogda yok: `TR_load_tensors` `TR_Dict` veriyor (`TR_Dict[TR_String, TR_AnyTensor]` değil), `.TR_narrow[E]()` ve `TR_quantized` yok, `TR_custom_grad` `TR_BuiltinFunction` veriyor [kanıtlı: `internal/lang/codes/definitions.json`].
- **S-214 Yapılacaklar (D, Ö3):** `TR_cumsum`, `TR_rms_norm`, `TR_attention` ve `TR_linear_cross_entropy`'de `pure` işareti yok, H1115 vermiyor [kanıtlı: `internal/lang/codes/definitions.json`]. Sıradaki: işaret; `tests/tensor_test.tr:216` ve `:1271` ilk ikisini `TR_try` içinde tek başına çağırıyor, onlar da değişir.
- **S-215 Yapılacaklar (A, Ö1, 4a5; sahip D1, Kol A):** sırayla verilen varsayılanlı argüman denetimden geçiyor: aynı tipli iki bayrağın yeri değişince `f(1, TR_true, TR_false)` sessizce çalışıyor [ölçüldü: ae1e8dcc]. Sıradaki: kural 13.10'a girer (`pending.md`'nin 13.10 satırı), denetim hatası adlı yazımı önerir.

### 13.11

- **S-216 Mevcut durum:** özelliği çağırmanın iletisi H2421 yalnız tipi söylüyor ("TR_List tipi bir fonksiyon olarak çağrılamaz"), özelliği ve parantezsiz yazımı söylemiyor [ölçüldü: ae1e8dcc] [test: `TestPropertyCall`].
- **S-217 Mevcut durum:** çalışma anı H2421'in ve H2423'ün öneri yuvasını boş geçiriyor [kanıtlı: `internal/lang/interp/calls.go:223`, `internal/lang/interp/arg_errors.go:97`].
- **S-218 Yapılacaklar (D ve F, Ö3):** denetleyici yalnız `FunctionArgCodes`'taki fonksiyonların argüman tipine bakıyor [kanıtlı: `internal/lang/checker/builtin_args.go:32`]; fonksiyonların argüman sayısı denetimde yok: `TR_round()` denetimden geçiyor, H2011 çalışırken ve görünüm adıyla ("yuvarla()") [ölçüldü: ae1e8dcc].

## 15. Tensörler ve otomatik türev

Kurallar: [`docs/spec/15_tensorler.md`](spec/15_tensorler.md).

### 15.1

- **S-219 Mevcut durum:** `TR_AnyTensor` ve `.TR_narrow[E]()` yok; `TR_Tensor[E]` ayrıştırılıyor ama denetimde `TR_Tensor`'a iniyor (S-222, S-297). GPU'da kapsam dışı cinsli işlemler H6969 [test: `TestTypedPathsOnGPU`].
- **S-681 Yapılacaklar (sınıfsız: hız, Ö3; sahip veri kolu):** toplu verinin kimlikleri float32 kurulup "int64"e çevriliyor (`internal/stdlib/dataset/dataset.go:536-537`, internal/stdlib/ml/value.go:83-93): 2²⁴'ün üstündeki kimlik H5536 verir (`dataset.go:572`), öntanımlı cihaz GPU iken toplu veri konağa okunup yeniden yüklenir [kanıtlı].
- **S-220 Mevcut durum (denetleyici tensör cinsini bilmiyor):** kuralın cins ve imza denetim hataları çalışmada geliyor ya da hiç gelmiyor (S-219): 15.2'nin `TR_Tensor[TR_Int]([1.5])`'i, 15.6'nın cinsli işleç tablosu (`@` dahil), 15.6a'nın sığmayan sayısı ve liste işleneni, 15.18'in konumlu ekseni, 15.18a'nın cins farkı, 15.49'un cinsleri [ölçüldü: trlang run, `ae1e8dcc`].
- **S-221 Yapılacaklar (test, Ö3):** cinsli çekirdek bileşimlerinin (`gpuTypedOK`, `internal/compute/gpu/typed_ops.go:25`) ad, benzersizlik ve ayrışma testi [hipotez]; benzetim `.target`'ı denetlemiyor (`internal/compute/gpu/sim_parse.go:239`; `mma.sync`, `cvt.rn.*x2.f32`); `checkKernelNames` yalnız bütün testler geçince koşuyor (`internal/compute/gpu/helpers_test.go:27-29`).
- **S-222 Yapılacaklar (A, Ö1, 4c):** öğe cinsli tip (`TR_Tensor[E]`), `TR_AnyTensor` ve daraltma: `TR_Tensor[TR_Int]` tipli yere `TR_Tensor([1.5])` hatasız float32 kuruyor [ölçüldü: trlang run, `ae1e8dcc`]. GPU'da "int64" `TR_max`, `TR_min`, `TR_mean`, `TR_cumsum` ve dar tam sayılar H6969 (`internal/compute/gpu/typed_ops.go:25-52`, `typed_reduce.go:20`); `TR_replay` kaydında tam sayı tensörünün sayıyla işlemi H4066.
- **S-223 Yapılacaklar (E, Ö3; sahip T1):** CPU'da float64'te `@`, `TR_conv1d`, `TR_exp`, `TR_log`, `TR_tanh`, `TR_sigmoid`, `TR_sqrt`, `TR_gelu`, `TR_relu`, `TR_softmax`, `TR_layer_norm`, `TR_ceil`, `TR_embedding_lookup` H6960 veriyor (15.1) [ölçüldü: trlang run, `ae1e8dcc`]; `tests/conv1d_test.tr:65-66` bunu bekliyor. F1: H6960 float64'te "float32, bf16 ya da fp16 ister" diyor (`internal/lang/messages/tr.json:1476`).

### 15.2

- **S-224 Mevcut durum (sınır):** cins çalışırken değerden çıkıyor, tip denetiminden değil: `TR_Float` parametresine verilen `2` ondalık geliyor, ondan kurulan tensör float32 (`p(2)` → `[2.0]`); `TR_var i = 1` `TR_Int` kalıyor [ölçüldü: trlang run, `ae1e8dcc`]. Tip denetiminin önerisi 4c'de.

### 15.3

- **S-227 Mevcut durum:** H3012 kaydedilenin girdi mi çıktı mı olduğunu ayırmıyor, `TR_sigmoid`'in kaydettiği çıktıya da "girdi" diyor (`internal/lang/messages/tr.json:594`). Hesap ondalığından float64'e `.TR_to_dtype` türev izlemiyor [ölçüldü: trlang run, `ae1e8dcc`].
- **S-228 Yapılacaklar (E, Ö3; sahip T1):** `.TR_fill` kuralda var (15.3, 15.6d), kimliği yok (`internal/lang/codes/definitions.json`); 15.34'ün `g.TR_fill(0.0)` ve `p.TR_fill(ana)` örnekleri onunla çalışır.
- **S-229 Yapılacaklar:**
  - **(E ve F, Ö3):** `TR_wrapping_add`, `TR_wrapping_sub`, `TR_wrapping_mul` yok (13.1); x.TR_float() yazımına H0026 `TR_Float`'u öneriyor, dönüşümü değil [ölçüldü: trlang check, `ae1e8dcc`];
  - **(F, Ö3):** sürüm iletisi girdi ile çıktıyı ayırır (S-227);
  - **(F1, Ö3):** tensör koşulunda H2600 `x != 0`'ı öneriyor, tensörde yine tensör verir; `TR_all`, `TR_equal(a, b)`, `.TR_item()` önerilmeli (`internal/lang/messages/tr.json:450`).

### 15.4

- **S-230 Mevcut durum:** tensörde `TR_std` denetimde H2423 veriyor: imzası yalnız `TR_List` (`internal/lang/codes/definitions.json:266`) [ölçüldü: trlang check, `ae1e8dcc`].
- **S-231 Yapılacaklar (E, Ö3):** `TR_std`'nin tensör yolu ve blok kuralı (15.4, 15.18).

### 15.5

- **S-232 Mevcut durum:** `TR_fuse` içinde karşılaştırma 0/1 float32 maske, `==`/`!=` orada H3107 [ölçüldü: trlang run, `ae1e8dcc`]. Tensörün metni kurma yazımı değil, dört basamaklı (`[2.0000]`); 1000'den çok öğede eksenlerin yalnız başı ve sonu yazılıyor (`…`); kısaltmanın fark olup olmadığı belirsiz [hipotez].
- **S-233 Yapılacaklar (A, Ö1):** `TR_fuse` içinde karşılaştırma ve `TR_is_finite` `TR_Bool` değil float32 veriyor: `(t > 1.5) * t` fuse içinde çalışıyor, dışında H6966; 15.44'ün "bit bit aynı" sözüne aykırı [ölçüldü: trlang run, `ae1e8dcc`]. Metnin kurma yazımı S-232'de.
- **S-235 Yapılacaklar (E, Ö3; sahip D1, sonraki eşitlik dilimi):** kitaplığın ve aktarılan modülün tipleri denetleyiciye bilinmiyor (`typeDefs` yalnız programın kendi tanımlarından dolar, `internal/lang/checker/equatable.go:27-40`): iki `TR_Sound`'un `==`'si denetimden geçip çalışmada H2770 veriyor [ölçüldü: trlang run, `ae1e8dcc`]. Sıradaki: aktarılan modülün tanımları `typeDefs`'e.

### 15.6

- **S-236 Mevcut durum:** kitaplıkta pencerenin sınırı konumların float32 kopyasıyla karşılaştırılıyor (konum < 2²⁴ kesin), çünkü `TR_replay` kaydında tam sayı tensörünün sayıyla işlemi H4066 verir [kanıtlı: `lib/layers.tr:457`, `internal/compute/gpu/backend_memory.go:180`].
- **S-237 Yapılacaklar (D ve E, Ö3):** hesap ondalığında `//` ve `%` H6703 [ölçüldü: trlang run, `ae1e8dcc`]; `TR_unsigned_shift_right`'ın kimliği yok; kartta dar tam sayılar H6969 (S-222); cins farkının denetim hatası (4c; bugün çalışmada H6966); TR testi.
- **S-238 Mevcut durum:** `@`'nin toplu çarpımında öndeki eksenler yayınlanmıyor, eşit olmalı: `[2, 4] @ [1, 4, 3]` ve `[2, 2, 4] @ [1, 4, 3]` H6115 [kanıtlı: `internal/stdlib/ml/operators.go:239-243`; ölçüldü: trlang run, `ae1e8dcc`]; kural yayınlar (`docs/spec/15_tensorler.md:109`).

### 15.6a

- **S-240 Mevcut durum:** liste işlenen hata vermeden çalışıyor: `TR_Tensor([1.0, 2.0]) + [10, 20]` → `[11.0, 22.0]` [ölçüldü: trlang run, `ae1e8dcc`].
- **S-241 Yapılacaklar (D, Ö3):** liste işlenenin denetim hatası; `TR_Tensor[TR_Int]` ile `2.5`'in denetim hatası (4c; bugün çalışmada H6968) [ölçüldü: trlang run, `ae1e8dcc`]; TR testi. Göç: `tests/matematik_test.tr:38`, `tests/tensor_test.tr:189` ve Go dizeleri.

### 15.6c

- **S-242 Yapılacaklar (E, Ö3):** tam sayı hassasiyetlerinin birleşmesi `TR_concat`'ta yok ("uint8" ile "int8" H6966; işleçte ve `TR_where`'de "int16") [ölçüldü: trlang run, `ae1e8dcc`]; kartta dar tam sayılar H6969; TR testi.

### 15.6d

- **S-243 Yapılacaklar (E, Ö3; sahip T1):** atamada tensör değer hedefe yayınlanmıyor: `t[0..2] = TR_Tensor([7.0])` H6719 veriyor, `t` değişmiyor; sayı değer yayınlanıyor (15.6d) [ölçüldü: trlang run, `ae1e8dcc`].
- **S-244 Yapılacaklar (D, Ö3; sahip T1):** cinsli hedefe liste değer denetim hatası olmalı, ileti `TR_Tensor(...)`'ı önermeli: ondalık tensörde `t[0..2] = [8.0, 9.0]` hatasız yazıyor, tam sayıda `u[0..2] = [7, 8]` çalışmada H6966 [ölçüldü: trlang run, `ae1e8dcc`].
- **S-245 Yapılacaklar (E, Ö3; sahip T1):** tensörle indekse yazma motorda yok (`t[idx] = d`, `t[idx] op= d` H7042): satır yazan tensör işlemi, yinelenen indeks denetimi, 16 bitte birikim, `t[m]`, kart yolu [test: `TestTensorIndexWriteSuggestionRuns`].
- **S-246 Mevcut durum:** maskeyle indeks yok (`t[m]`, `t[m] = d` H6707); hesap ondalığında `%=` H6703, cinsli tensörde çalışıyor [ölçüldü: trlang run, `ae1e8dcc`]; `.TR_select`, `.TR_unsqueeze` ve `.TR_fill`'in kimliği yok (`internal/lang/codes/definitions.json`).
- **S-247 Yapılacaklar:**
  - **(E, Ö3):** `.TR_select` ve `.TR_unsqueeze` (türevin yazma kurallarıyla); TR testi;
  - **(F, Ö3; tutarlılık):** kitaplığın `TR_Tensor(TR_no_grad(...), TR_requires_grad: TR_true)` dolambacı (`lib/layers.tr:22` ve 8 satır daha) `x.TR_requires_grad = TR_true` ile sadeleşir.
- **S-682 Yapılacaklar (F1, Ö3; sahip Motor):** dil tanımında olup motorda olmayan kimlik (`t.TR_unsqueeze(0)`, 15.6d) uydurma adla aynı H0001'i alıyor (`internal/lang/messages/tr.json:2`) [ölçüldü: trlang check, `ae1e8dcc`]. Sıradaki: bu kimliklere yerine yazılacak yazımı veren ayrı ileti (kodu TCB verir).
- **S-248 Mevcut durum (ters dönecek test):** `TestMatrixIndexAndAssign` (`internal/stdlib/ml/matrix_test.go:50`) liste değerli atamayı (`M[-1] = [8, 9]`, `T[0][1] = [1, 2]`; 15.6d) ve 0 boyutlu tensörde `.TR_tolist`'i (13.8a) doğru bekliyor; kurallar uygulanınca ters döner.

### 15.6e

- **S-249 Mevcut durum:** hesap ondalığında `2.0 ** t` ve `t ** u` H6702 veriyor, tam sayı tensörde çalışıyor [ölçüldü: trlang run, `ae1e8dcc`]; ileti tabanın sayı ya da üssün tensör olduğunu ayırmadan `TR_exp(TR_log(t) * u)`'yu öneriyor (`internal/lang/messages/tr.json:1386`).
- **S-250 Yapılacaklar (E, Ö3):** hesap ondalığında tensör üs (H6702); kartta dar tam sayılar (H6969, S-222); TR testi.

### 15.8

- **S-251 Mevcut durum:** `TR_write_slice`'ın `TR_start:`'ı tam sayı ya da tam sayı değerli 0 boyutlu ondalık tensör alıyor (S-252) [ölçüldü: trlang run, `ae1e8dcc`].
- **S-252 Yapılacaklar (A, Ö1; sahip T1, 1.2'den sonraki dilimi):** `TR_start`'ın `TR_Int` ya da 0 boyutlu `TR_Tensor[TR_Int]` olması ve cins denetimi: ondalık başlangıç float32'ye iniyor, kesir ve 2²⁴'ün üstü sessizce kayboluyor (`TR_start: TR_Tensor(1.0 + 0.0000000001)` 1. konuma yazıyor) [ölçüldü: trlang run, `ae1e8dcc`].

### 15.13

- **S-253 Yapılacaklar (E, Ö3; sahip T1):** tensör sınır yok: `TR_clamp(t, TR_min: tensör)` H6107 veriyor (15.13) [ölçüldü: trlang run, `ae1e8dcc`].

### 15.15
- **S-693 Yapılacaklar (A, Ö0, 1. sıra; sahip T1):** `TR_where`'in sayı dalı tensörün hassasiyetini almıyor (15.15, 15.6a): bf16 `b`'de `TR_where(c, b, 0.0)` float32 veriyor, `b + 0.0` bf16 [kanıtlı: `internal/stdlib/ml/typed_where.go:80`; ölçüldü: ae1e8dcc]. Sıradaki: T1'in GPU hızı Dilim A'sından sonraki ilk iş; `TR_maximum`, `TR_minimum` ve `TR_clamp`'in sayı dalı da sınanır.
- **S-254 Yapılacaklar (D, Ö3; sahip dil çekirdeği):** 15.15'in "hiçbiri tensör değilse denetim hatası" kuralı yalnız mantık değişmezinde uygulanıyor (`checkWhereCondition`, `internal/lang/checker/builtin_args.go:210`): `TR_Bool` tipli değişken koşul çalışmada H6207 [ölçüldü: trlang run, `ae1e8dcc`]. Sıradaki: denetleyici bildirilmiş `TR_Bool`'u karşılaştırmadan ayırınca kural tamamlanır.

- **S-255 Mevcut durum:** ondalık ya da tam sayı tensör koşul (`TR_where(TR_Tensor(1.0), 2, 3)`) denetimde değil, çalışmada H6207 veriyor; mantık değişmezi koşul denetimde yakalanıyor [ölçüldü: trlang check ve run, `ae1e8dcc`].

### 15.16a

- **S-257 Mevcut durum:** `TR_maximum` ve `TR_minimum` `TR_fuse` içinde H3107; tensörde `TR_round` (imzası sayı, `internal/lang/codes/definitions.json:247`) ve `TR_is_nan` yok [ölçüldü: trlang run, `ae1e8dcc`]. Kartta mantık tensörünün `TR_sum`'ı H6969: kart yalnız "int64" topluyor (`internal/compute/gpu/typed_reduce.go:20`).
- **S-258 Yapılacaklar (E, Ö3):** tensörde `TR_round` ve `TR_is_nan`; TR testi.
- **S-259 Yapılacaklar (sınıfsız: hız, Ö3; sahip T1):** `TR_maximum`'un ondalık yolu dört karşılaştırma, iki ters alma ve üç mantık işlemiyle maske kurup `TR_where`'e veriyor (`internal/compute/tensor/extreme.go:29-34`, `:70-81`, `:116-123`). Sıradaki: −0 ve NaN kuralıyla tek geçişli öğe öğe çekirdek, `TR_fuse`'un öğe döngüleriyle aynı işte.

### 15.17

- **S-260 Mevcut durum:** `TR_gelu`'nun `TR_approximate:`'i zorunlu değil, varsayılanı tanh [ölçüldü: trlang run, `ae1e8dcc`].
- **S-261 Yapılacaklar (D ve E, Ö3):** zorunlu ve yalnız adla verilen `TR_approximate:`; konumlu `TR_gelu(1.0, TR_false)` denetimden geçip çalışmada H6101 [ölçüldü: trlang run, `ae1e8dcc`]. Göç: argümansız 14 `.tr` çağrısı, Go testlerinin TR metni ve `definitions.json`'un örneği; göçte `TR_approximate: TR_true` yazılır.

### 15.18

- **S-262 Yapılacaklar (E, Ö3; sahip T1):** `TR_dim:` listesi (`TR_sum(x, TR_dim: [1, 2])`) H6106 veriyor [ölçüldü: trlang run, `ae1e8dcc`].
- **S-263 Mevcut durum:** eksen `TR_sum`/`TR_mean`'de konumla da veriliyor (`TR_sum(x, 0)`); `TR_max`/`TR_min`'de konumlu ikinci argüman denetimde değil çalışmada H6205 [kanıtlı: `internal/stdlib/ml/builtin_reduce.go:35-38`; ölçüldü: trlang run, `ae1e8dcc`].
- **S-264 Yapılacaklar (D ve E, Ö3):** eksen yalnız adla; göç `tests/tensor_test.tr:379`, `tools/gpu/ss_exam.tr:59`, `internal/stdlib/ml/builtin_reduce_test.go:89`. Kartta dar tam sayı ve mantık tensörünün `TR_sum`'ı H6969 (`internal/compute/gpu/typed_reduce.go:20`); TR testleri.
- **S-265 Yapılacaklar (E, Ö3):** `TR_std`'nin tensör yolunda (S-231) sapması 0 olan eksende türev 0 (15.18); TR testi.

### 15.18a

- **S-266 Mevcut durum:** tensör olmayan ya da mantık olmayan tensör argümanı (`TR_equal(t, 1)`, `TR_all(t)`) denetimde değil çalışmada H6979 [ölçüldü: trlang run, `ae1e8dcc`].
- **S-267 Yapılacaklar (E ve D, Ö3):** `TR_any` yok; eksenli `TR_all`/`TR_any` ve `TR_keepdim:` (bugün `TR_all(m, TR_dim: 0)` H7902); ondalık ya da tam sayı tensörün ve cins farkının denetim hatası (bugün çalışmada H6979, H6966) [ölçüldü: trlang run, `ae1e8dcc`].

### 15.19

- **S-268 Mevcut durum:** `TR_argmin` yok (`internal/lang/codes/definitions.json`'da kimliği yok).
- **S-269 Yapılacaklar (E, Ö3):** `TR_argmin`, eksenli `TR_keepdim:`; TR testi.

### 15.20

- **S-271 Mevcut durum:** `TR_cumsum`'un ekseni zorunlu değil, varsayılanı −1 [ölçüldü: trlang run, `ae1e8dcc`].
- **S-272 Yapılacaklar (D, Ö3):** `TR_dim:` zorunlu; göç: eksensiz 13 `.tr` çağrısı (`examples/tr/25_secme_ve_siralama.tr`, `tests/matematik_test.tr`, `tests/tensor_test.tr`) ve Go dizelerinde 3.

### 15.21

- **S-273 Mevcut durum:** `TR_topk` demet değil liste veriyor (`TR_Type` `TR_List`) [ölçüldü: trlang run, `ae1e8dcc`].
- **S-274 Yapılacaklar (D, Ö3):** demet dönüşü (önce 3.6a); göç 13 `.tr` çağrısı (`examples/tr/25_secme_ve_siralama.tr:22`, `tests/matematik_test.tr`, `tests/tensor_test.tr:883`, `:886`, `tools/assistant/zincir.tr:170`) ve Go dizelerinde 5; TR testi.

### 15.21a

- **S-275 Mevcut durum:** `TR_gather` yok (`internal/lang/codes/definitions.json`'da kimliği yok).
- **S-276 Yapılacaklar (E, Ö3):** `TR_gather` (15.21a); TR testi.

### 15.22

- **S-277 Mevcut durum:** `TR_topk` ve `TR_argsort`'ta sıralanan eksen en çok 2²⁴ uzunlukta: indeksler float32 depoda (internal/compute/tensor/device.go:447-451, `MaxSortLen` `internal/compute/tensor/cumsum_sort.go:11`); `TR_descending:` yok, motorda `Sort(..., desc)` hazır.
- **S-278 Yapılacaklar (E, Ö3):** 2²⁴ sınırı tam sayı depolamasıyla (15.1) kalkar, yeni sınır 2³² − 1; `TR_descending:`; TR testi.
- **S-279 Mevcut durum (sınır):** GPU'da sıralanan anahtarın üst 32 biti değerin sıra anahtarı, alt 32 biti indekstir (`internal/compute/gpu/kernel_sort.go:21-23`): eksen en çok 2³² − 1, anahtar 32 bit; int64 ve float64 tensörde ve daha uzun eksende yeni yol gerekir. Kural sınır koymaz (15.42).

### 15.22a

- **S-281 Yapılacaklar (sınıfsız: hız; Ö1; sahip T1):** bf16 ve fp16 `@`'nin tensör çekirdekli yolu yalnız `TR_vendor_matmul` bloğunda (`internal/compute/gpu/backend_gemm.go:161`, `:175`); blok dışında float32'ye açılır, H4132 uyarır [test: `TestMixedPrecisionWithoutBlockWarns`]. Kendi çekirdeğin bf16'sı torch'tan ve kendi float32'sinden yavaş [ölçüldü: RT-2026-11-19-L4GPU-H1-2, RT-2026-11-19-G4GPU-H1-2]. Sıradaki: kendi tensör çekirdekli 16 bitlik çarpım.
- **S-283 Yapılacaklar (D, Ö3):** sayı yerleşikleri `internal/detmath`'te, tensör yolu float32'de kendi çekirdekleriyle (`internal/compute/tensor/cpu_elementwise_wide.go:101`): `TR_exp`, `TR_sigmoid`, `TR_silu` sayıda ve tensörde son bitlerde ayrılabiliyor [kanıtlı]; 15.22a sayı ile tensörü anmıyor. Sıradaki: dil tanımı turunda karar.

### 15.22b

- **S-284 Yapılacaklar (E, Ö3):** `TR_argmin` gelince 15.22b'nin sırası (S-269).

### 15.23

- **S-285 Mevcut durum:** `TR_embedding_lookup`'ta indeks liste ya da ondalık tensör olabiliyor [ölçüldü: trlang run, `ae1e8dcc`].
- **S-286 Yapılacaklar (D ve E, Ö3):** indeks `TR_Tensor[TR_Int]`; göç: liste ve ondalık indeksli çağrılar; doğrudan çağrıda sınır dışı denetimi bekleyen işlem (15.6; kayıtta cihaz içi bayrak var, 15.45).
- **S-287 Yapılacaklar (C, Ö2, Kol B (T1)):** doğrudan çağrıda ondalık indeksin ve dilim başlangıcının "tam sayı mı" denetimi Go'nun sığmayan float → int dönüşümüne dayanıyor (internal/stdlib/ml/value.go:136, `builtin_nn.go:110`, `builtin_replay.go:462`): tam 2⁶³ amd64'te ve arm64'te farklı hata verir [hesap]. Sıradaki: dönüşümden önce aralığın açık denetimi; arm64'te 2⁶³ testi.

### 15.23a

- **S-288 Yapılacaklar (E, Ö3; sahip T1):** `TR_cross_entropy`'nin hedef denetimi hedefi konağa okuyor (`IntElements`, internal/stdlib/ml/value.go:136), `TR_replay` kaydında H4065 (`internal/compute/gpu/typed.go:44`); kayıtta hedefin şekli için test yok (`internal/stdlib/ml/builtin_nn.go:247`). Sıradaki: denetim cihaz içi bayrağa; kayıtta şekli yanlış hedef H6313 testi.

### 15.23b

- **S-289 Mevcut durum:** `TR_group_norm` yok (`internal/lang/codes/definitions.json`'da kimliği yok).
- **S-290 Yapılacaklar (E, Ö3):** `TR_group_norm` (15.23b); TR testi.

### 15.23c

- **S-291 Mevcut durum:** `TR_conv2d` yok (`internal/lang/codes/definitions.json`'da kimliği yok).
- **S-292 Yapılacaklar (E, Ö3):** `TR_conv2d` (15.23c); TR testi.
- **S-293 Yapılacaklar (sınıfsız: hız, Ö2; sahip T1):** `TR_conv1d` sabit ağırlığı her çağrıda tek iş parçacığında paketliyor: `packA` döngüsü iş dağıtımından önce (`internal/compute/tensor/conv1d_cpu.go:125-131`, dağıtım `:145`); payı ölçülmedi. Sıradaki: Piper'da profil payı, sonra sabit ağırlığın bir kez paketlenmesi.
- **S-294 Yapılacaklar (sınıfsız: hız, Ö3; sahip T1):** evrişim yolu dönen işçileri tutmuyor: `holdWorkers` yalnız nicemli çarpımda (`internal/compute/tensor/cpu_quant.go:153`, `:160`, `cpu_quant_q8.go:243`, `:263`); işçi uyursa sonraki işin uyandırmayı beklediği `internal/compute/tensor/parallel.go:79-81`'de, payı ölçülmedi [hipotez]. Sıradaki: Piper'da ölçü.

### 15.23d

- **S-295 Mevcut durum:** `TR_dropout` yok (`internal/lang/codes/definitions.json`'da kimliği yok).
- **S-296 Yapılacaklar (E, Ö3):** `TR_dropout` paylaşılan üreteçten (20.17, oradaki yazılabilir üreteçle); TR testi.

### 15.24

- **S-297 Mevcut durum:** `TR_mask:`'in cins hatası (H6985) çalışmada geliyor: `TR_Tensor[TR_Float]` tipli değişken de denetimde `TR_Tensor` (S-219) [ölçüldü: trlang run, `ae1e8dcc`].
- **S-298 Yapılacaklar (D, Ö3; sahip T1):** 15.24'ün "denetim hatasıdır" sözü denetleyici tensör cinsini bilince (S-222) uygulanır.
- **S-299 Mevcut durum:** `TR_attention`'da `TR_bias:` yok (15.24): çağrı H7901 veriyor, geçerli adlar `TR_causal`, `TR_window`, `TR_mask` [ölçüldü: trlang run, `ae1e8dcc`].
- **S-300 Yapılacaklar (E, Ö3; sahip tensör/GPU kolu):** `TR_attention`'ın `TR_bias:` argümanı (S-303 buna bağlı).
- **S-302 Mevcut durum (sınır):** dikkat ölçeği bakış boyutundan farklı Gemma 3 27B (`query_pre_attn_scalar` 168) yüklenmez, H7177; GGUF'ta yalnız `block_count` 18, 26, 34, 48 ve bakış boyutu 256 yüklenir [test: `TestGemma3GGUFGates`].
- **S-303 Yapılacaklar:** tamamen −∞ sapmalı satır 0 verir, CPU'da ve GPU'da (15.24; `TR_bias:` gelince, S-300). CPU'da tamamen maskeli `TR_Bool` satırı 0 veriyor [ölçüldü: trlang run, `ae1e8dcc`]; GPU ölçülmedi.
- **S-304 Yapılacaklar (A, Ö1; sahip T1, tensör kolu):** maskelenen anahtardaki NaN satıra sızıyor (15.24): maskeli anahtarın V satırındaki NaN çıktıya, K satırındaki NaN `q.TR_grad`'a geçiyor [ölçüldü: trlang run, `ae1e8dcc`]. CPU dikkatinin karo çarpımları bakılmayan satırları sıfırlamadan çarpıyor [hipotez]; GPU çekirdeği de denetlenmeli. Sabit kapasiteli KV önbelleğinin yazılmamış satırları bu yoldan geçer.

### 15.27

- **S-305 Yapılacaklar (F, Ö3; test kapsamı):** `TR_no_grad { TR_defer { ... } }`'ta ertelenen bloğun kipin içinde çalıştığını sabitleyen TR testi yok.

### 15.27a

- **S-306 Mevcut durum:** `TR_detach` yok; `TR_no_grad(x)`'in sonucuna yazmak türev izleyen `x`'i sessizce değiştiriyor (`d = TR_no_grad(h); d[0] = 9.0`) [ölçüldü: trlang run, `ae1e8dcc`].
- **S-307 Yapılacaklar (A, Ö1, Kol B, 15.6d ile):** `.TR_detach()`; ayrılmış görünüme yazma kuralları (15.6d); `TR_no_grad(x)` biçimi kalkar, göç 22 satır; TR testi.

### 15.28

- **S-308 Mevcut durum:** `TR_custom_grad`'ın `f_ileri`'si demet değil `[çıktı, [saklananlar]]` listesi döndürüyor; çağrının değeri `TR_BuiltinFunction` [test: `tests/ozel_turev_test.tr:12`, `:36`].
- **S-309 Yapılacaklar (D ve E, Ö3):** demet biçimi (önce 3.6a); dönüş tipi; göç 22 satır.
- **S-310 Yapılacaklar (A, Ö1; sahip T1, tensör kolu):** türevsiz kipin dışında çağrılan `f_ileri` kapanıştan türev izleyen tensör okuyunca çalışma hatası yok, o tensörün gradyanı sessizce `TR_none` kalıyor [ölçüldü: trlang run, `ae1e8dcc`]. Kural çalışma hatası ve konumlu argüman önerisi istiyor (15.28).

### 15.29

- **S-311 Mevcut durum:** `f_geri` tek girdide de liste döndürüyor (`[g * 2 * x]`) [test: `tests/ozel_turev_test.tr:15`].
- **S-312 Yapılacaklar (D ve E, Ö3):** `f_geri` tek girdide kendisini, birden çokta demet döndürür; fonksiyon ifadesinde öğe sayısının denetim hatası (15.29).

### 15.32

- **S-313 Yapılacaklar (D, Ö3):** `TR_grad_check`'in `girdiler`'i demet (15.32).

### 15.33

- **S-314 Mevcut durum:** `TR_parameters` `TR_named:` ile yol adlı sözlük veriyor; `TR_named_parameters` yok [kanıtlı: `internal/lang/codes/definitions.json:218`, `:409`; ölçüldü: trlang run, `ae1e8dcc`].
- **S-315 Yapılacaklar (A, Ö1, Kol B, 13.8a ile):** `TR_named:` kalkar, `TR_named_parameters` (13.1) gelir; parametre tanımı (donmuşlar `TR_parameters`'ta); `TR_named_parameters` bütün yapraklar; türevli ara sonuçta hata; TR testi.
- **S-316 Yapılacaklar (C, Ö2; sahip D1, aileler kolu):** VITS, Whisper ve VAD'ın ağırlık taşıyan alanları `_` önekli (`lib/vits.tr:529-539`, `lib/whisper.tr:347-351`, `lib/vad.tr:43-54`); 17.8'e göre kimlik üyesi olmalı, kaydederken düz ya da `_` önekli alanda tensörün çalışma hatası da yok. Bugün yalnız çıkarımda kuruluyorlar.

### 15.34

- **S-318 Yapılacaklar (D, Ö3):** `w.TR_requires_grad = 1` denetimde değil çalışmada H6992 [ölçüldü: trlang run, `ae1e8dcc`]; denetleyici tensör üyesinin tipini bilince yakalar.
- **S-319 Yapılacaklar (E, Ö3):** kitaplığın `TR_Muon` ve `TR_NorMuon`'u 16 bitlik parametrede ana kopya tutmuyor: güncelleme doğrudan parametreye yazılıyor, H6994 uyarıyor (`lib/optimizers.tr:95-97`). Sıradaki: iki iyileştiriciye ana kopya.
- **S-320 Yapılacaklar (E, Ö3):** H6994'ün önerisi `.TR_fill` yokken `p[0..p.TR_shape[0]] = ana` (`internal/lang/messages/tr.json:1510`); `.TR_fill` gelince 15.34'teki `p.TR_fill(ana)` olur (S-228).
- **S-321 Yapılacaklar (F1, Ö3):** H6994'ün önerisi (`p[0..p.TR_shape[0]] = ana`) 0 boyutlu parametrede sınır dışı hatası verir; kitaplığın `TR_Muon`'unda uyarı kullanıcının değiştiremediği `tr/optimizers.tr` satırını gösterip ana kopya tutmayı söylüyor, "parametreyi float32 kurun" demeli [kanıtlı: `internal/lang/messages/tr.json:1510`, `lib/optimizers.tr:97`].
- **S-322 Yapılacaklar (F, Ö3):** kitaplığın iyileştiricisinde (`TR_Muon` vb.) H6995 kullanıcının satırını değil `tr/optimizers.tr:118`'i gösteriyor [kanıtlı: `lib/optimizers.tr:118`].
- **S-323 Yapılacaklar (F1, Ö3):** ara sonuç (yaprak olmayan tensör) kitaplığın `TR_Muon`'unda H6995, Go'daki `TR_SGD`'de H6514 veriyor: aynı durum iki kodla [kanıtlı: `lib/optimizers.tr:118`, `internal/stdlib/ml/optimizer.go:158-159`].

### 15.36

### 15.37

- **S-327 Yapılacaklar (A, Ö0, 2. sıra; sahip T1):** cihaz adı tek yazımlı değil (15.37): `.TR_to_device(" CPU ")`, `.TR_to_device("Cpu")` ve `TR_default_device(" CPU ")` hatasız kabul ediliyor, `tests/tensor_test.tr:1782` bunu doğru diye sabitliyor [ölçüldü: ae1e8dcc, trlang run]. Desteklenen GPU yalnız NVIDIA.
- **S-328 Yapılacaklar (D, Ö3):** cihaz adının tek yazımı (büyük harf ve boşluk hata); `tests/tensor_test.tr:1783` ters döner; TR testi.
- **S-329 Yapılacaklar (F, Ö3; sahip T1, Ö1 tensör tasarımlarından sonra):** iletiler cihazı kısa adla (`"gpu"`) yazıyor: `checkDevice` (`internal/stdlib/ml/operators.go:62-64`), `availableDevice` (internal/stdlib/ml/device.go:60) ve öbürleri Device.String'i kullanıyor (internal/compute/tensor/device.go:66); okunan ad `LongName`'dir (15.37).
- **S-330 Yapılacaklar (F, Ö3; sahip H1):** profilcinin raporu cihazı `"gpu"` yazıyor (`internal/devtools/profiler/setup.go:140`, `:197`); 15.37'nin okunan adı `"gpu:0"`.

### 15.37a

### 15.38

- **S-333 Yapılacaklar (test, Ö3; sahip T1):** denetleyicinin göremediği yoldan çağrılan `TR_default_device(ad)`'ın değer vermediğini sınayan test yok (`TR_var f = TR_default_device; f("cpu")`); `tests/tensor_test.tr`'ye bir satır.

### 15.41

- **S-334 Yapılacaklar (C, Ö2; sahip tensör kolu):** işlemcide `TR_Int` tensörünün `**`'ı üssü tek tek çarpıyor (`powInt`, `internal/compute/tensor/cpu_typed_ops.go:333-348`): taban −1 ve büyük üste program asılı kalır, kart kare alarak hesaplıyor (`internal/compute/gpu/typed_arith.go:102-105`). Sıradaki: yorumlayıcının kare-çarp yöntemi (`internal/lang/interp/operators.go:403`), TR testi; iki cihazın ilk hatalı öğesi aynı kalır.
- **S-335 Mevcut durum (sessiz konak yolları):** `TR_randn` konakta üretilip varsayılan cihaza kopyalanıyor (`internal/compute/tensor/tensor.go:245-252`); `TR_argmax`'ın indeksi konakta kurulup cihaza yükleniyor (`internal/stdlib/ml/builtin_reduce.go:225-237`).
- **S-338 Mevcut durum:** kartta tam sayıda yalnız int64'ün aritmetiği ve `TR_sum`'ı var; dar tam sayıların aritmetiği ve toplamı, int64 `TR_max`, `TR_min`, `TR_mean` ve tam sayı `TR_cumsum` kartta H6969 veriyor [kanıtlı: `internal/compute/tensor/typed.go:175-176`, `internal/compute/gpu/typed_reduce.go:24`; test: `TestTypedPathsOnGPU`].
- **S-339 Mevcut durum:** kartta float64 `TR_logsumexp` H7081, float64 `TR_rfft` ve `TR_stft` H7076 veriyor; ileti tensörü işlemciye taşımayı söylüyor [test: `TestLogSumExpRunsOnTheCard`, `TestFourierRunsOnTheCard`].
- **S-341 Yapılacaklar (sınıfsız: bellek, Ö3; sahip T1):** kartın havuzu serbest kalan blokları sürücüye geri vermiyor [kanıtlı: `internal/compute/gpu/memory.go:20-22`]. Sıradaki: havuzu boşaltan yol ya da zincirin bağlamını küçültmek.
- **S-343 Yapılacaklar (E, Ö3; sahip T1, sonraki dilim):** kartta `TR_logsumexp`'in türevi 2²⁴ öğeden uzun satırda `TR_backward`'da iç hata veriyor; işlemcide sınır yok [kanıtlı: `internal/compute/gpu/backend_row.go:66-80`]. Sıradaki: ilk +∞'ın konumunu tam sayıyla indirgemek, 2²⁴ + 1 öğelik test.
- **S-344 Yapılacaklar (D, Ö3; sahip T1):** kayıtta kartın `TR_stft`'i H4066 veriyor: çerçevelerin konum tablosu her çağrıda konaktan yükleniyor (`internal/compute/tensor/fourier_card.go:135`) [test: `TestReplayFourierOnTheCard`].
- **S-345 Yapılacaklar (C, Ö2; sahip T1):** kayıtta `TR_rfft`'in sonucu taban önbelleğinin geçmişine bağlı: önbellek dolup taban bırakılınca aynı adım tabanı konakta yeniden kurar ve H4066 verir [kanıtlı: `internal/compute/tensor/fourier_card.go:19-20`, `:42-61`].
- **S-346 Yapılacaklar (D, Ö3; sahip T1):** kayıtta H4066'nın gerekçesi (yüklenen veri her oynatmada aynı kalırdı) sabit sayı işleneninde yanlış: sabit için istenen zaten budur (`internal/lang/messages/tr.json:756`, `internal/compute/gpu/memory.go:557`).
- **S-347 Yapılacaklar (F, Ö3; sahip T1):** H4066'nın önerisi kayıtta kartın `TR_stft`'inde uygulanamaz: yüklenen konum tablosu kullanıcının verisi değil, kitaplığın sabitidir (`internal/compute/tensor/fourier_card.go:135`).
- **S-348 Yapılacaklar (F, Ö3; sahip T1):** tam sayı toplamının ve birikimli toplamının taşma iletisi H6961 konumu boş geçebiliyor (`internal/lang/messages/tr.json:1477`, konum yuvasının boş seçeneği); ileti doğru ama eksik.
- **S-349 Yapılacaklar (F, Ö3; sahip T1):** tekli eksinin taşma iletisi H6961 çift eksi yazıyor (`--9223372036854775808`) [ölçüldü: `ae1e8dcc`'nin trlang run'ı, int64'ün en küçüğünde `-x`]. Sıradaki: ileti `-(-9223372036854775808)`'i ya da işlecin adını yazar.
- **S-350 Yapılacaklar (D, Ö3; sahip T1, kapı TCB):** GPU paketinin PTX özeti (`internal/compute/gpu/testdata/ptx_ozet.txt`) yalnız bütün paketin `-short` koşusuyla yenilenebiliyor [kanıtlı: `internal/compute/gpu/behavior_ptx_digest_test.go:107-113`]. Sıradaki: dar koşuda görülen anahtarları yenilemek (T1), yerel kapıya dar GPU koşusu (TCB).
- **S-351 Yapılacaklar (sınıfsız: hız; sahip T1):** kartın int64 toplamında eksensiz toplam tek iş parçacığıyla sırayla toplanıyor (`internal/compute/gpu/typed_reduce.go:35`, `plain(m, sameBlock)`, m = 1). Sıradaki: büyük n, küçük m için ağaç yolu ya da sınır.
- **S-352 Yapılacaklar (sınıfsız: bellek, Ö3; sahip T1):** `sumInt64OnDevice`'ın bitişik kopyası bırakılmıyor (`internal/compute/tensor/typed_reduce.go:82`, `Drop` yok); 15.43'ün belirlenimli bırakması bu kopyada işlemiyor. Emsal: `internal/compute/tensor/view.go:115-119`.
- **S-353 Mevcut durum:** kartta tohumlu `TR_randn` konakta üretilip karta kopyalanıyor (`internal/stdlib/ml/builtin_create.go:200`), dil tanımı kararı `docs/spec/pending.md:70`'te.
- **S-354 Mevcut durum:** GPU'daki matematik işlevlerinin (`ex2.approx`, `lg2.approx` yolları, `internal/compute/gpu/ptx_math.go`) ulp sınırı ölçülmedi, gpu paketinde ulp testi yok; 15.11 GPU için birkaç ulp diyor.
- **S-355 Yapılacaklar (C ve F, Ö2):** GPU sınırlarını ölçüp 15.10 ve 15.11'e yazmak; cihazlar arası sözün Go testi (15.41'in Test satırı onu bekliyor).

### 15.42

- **S-356 Mevcut durum:** sıfır eksenli (verisiz) şekil 50 milyon öğelik koleksiyon sınırına, veri taşıyan tensör 2³² öğeye göre denetleniyor: `TR_zeros([0, 1000000000, 1000000000])` ve `TR_zeros([100000, 100000])` H6028 [kanıtlı: `internal/stdlib/ml/error.go:188-209`, `internal/lang/interp/builtins.go:433`].
- **S-357 Yapılacaklar (E, Ö3):** 50 milyon ve 2³² sınırları kalkar, yerine int64 taşma denetimi ve ayırmada 15.42'nin bellek denetimi (`internal/stdlib/ml/error.go:188-209`); 64 bitlik GPU indeksi yol haritasında.

### 15.43

- **S-358 Yapılacaklar (F, Ö3):** `+=` ve `*=` bütün listeyi ve öğelerini kaçırıyor (`internal/lang/interp/assign.go:79-83`), `.TR_append` ve indeksle atama yalnız yazılan değeri: güvenli taraf ama sapma, testi yok. 15.43'ün Test satırında `TestOwnershipListLiteralOwnsElementsLikeATuple` ve `TestOwnershipIndexAssignmentEscapesOnlyTheWrittenValue` yok.

### 15.44

- **S-359 Yapılacaklar (E, Ö3; sahip T1 (fuse*.go, kernel_fused.go)):** `TR_fuse` içinde `TR_maximum`, `TR_minimum`, `TR_clamp` ve `TR_atan2` H3107, `TR_where` H6207 veriyor; 15.44 öğe öğe işlemlere izin veriyor (`docs/spec/15_tensorler.md:556`) [ölçüldü: `ae1e8dcc`'nin trlang run'ı]. Sıradaki: T1'in Q4 işinden sonra, işlemci ve GPU birlikte.
- **S-360 Yapılacaklar (C, Ö2):** `TR_fuse`'un kapanış sayısı denetimi (H6986, `internal/stdlib/ml/fuse_captures.go:96-108`) GPU'da sınanmadı: H6986'nın kartlı ya da benzeticili testi yok. Sıradaki: benzeticili bir Go testi ya da L4'te koşmak.
- **S-361 Yapılacaklar (F, Ö3):** `TR_fuse` modül adını ve metot yolunu `VM.EvalExpression` ile okuyor (`internal/stdlib/ml/fuse_paths.go:120`); `internal/lang/interp/debug_support.go:5-7` bu yöntemleri yalnız hata ayıklayıcı için tanımlıyor. Sıradaki: yorum ikinci kullanıcıyı söyler ya da dar bir üye okuma fonksiyonu açılır.
- **S-362 Yapılacaklar (F1, Ö3):** H6986'nın önerisi (değişen sayıyı argüman verin) fonksiyon değerinde yanlış: fonksiyon `TR_fuse`'a argüman olamaz, doğrusu `TR_fuse(...)`'i yeniden kurmak (`internal/lang/messages/tr.json:1502`). Fabrika kapanışının görünüşü ve takma adlı bağlı metodun yolu [hipotez: yeniden sınanmadı].
- **S-363 Yapılacaklar (C, Ö3) [hipotez]:** fonksiyon kimliği `%p` ile metin olarak tutuluyor (`internal/stdlib/ml/fuse_captures.go:79-81`); eski kapanış toplanıp yenisi aynı adrese düşerse değişim görünmez. Sıradaki: gövde ve ortam işaretçilerini `fuseCapture`'da tutmak.
- **S-364 Yapılacaklar (D, Ö3; sahip T1 (tensör ve GPU), iletisi Motor):** H3107 işlemin adını söylemiyor (`internal/lang/messages/tr.json:687`); denetleyici `TR_fuse`'u yalnız `internal/lang/checker/const_values.go:46`'da tanıyor, `TR_fuse((a) => TR_sum(a))` denetimden geçip çalışırken H3107 alıyor. Sıradaki: iletiye işlemin kimliği, sonra denetimde yakalama.
- **S-365 Yapılacaklar (sınıfsız: hız, Ö3; sahip T1 (tensör ve GPU)):** `TR_fuse`'un CPU değerlendirmesi ortak alt ifadeyi her kullanımda yeniden hesaplıyor: iz ortak düğümü bir kez veriyor (internal/compute/tensor/fuse.go:121), değerlendirme özyinelemeli ve önbelleksiz (`internal/compute/tensor/fuse_cpu.go:182-216`). Sıradaki: ortak düğümü bir kez hesaplamak.
- **S-366 Yapılacaklar (E, Ö3):** `TR_fuse` float64 ve tam sayı tensörde H3107 veriyor; 15.44 onları kendi türünde hesaplar (`docs/spec/15_tensorler.md:559`) [ölçüldü: `ae1e8dcc`'nin trlang run'ı].

### 15.45

- **S-368 Yapılacaklar (A, Ö1, Kol B (T1), 15.6'nın cihaz içi bayrağıyla):** `TR_replay` kapanıştan okunan sayıyı ve tensör deposunu yoluyla denetlemiyor, cihazın sayaçlı üreteci yok; 15.45 ikisini de istiyor [kanıtlı: `internal/stdlib/ml/builtin_replay.go`'da kapanış denetimi yok]. Sıradaki: T1'in Dost GPU tasarımına dilim olarak girer.
- **S-369 Yapılacaklar (C, Ö2, Kol B (T1)):** boş girdide kayıt denetlemiyor, doğrudan çağrı hata veriyor: kaynak boşken kayıtta dilim yazma atlanıyor (`internal/compute/tensor/indexed.go:104`), gömmede D=0'da kayıt H6305 vermiyor (`:71`). Gerileme değil.

### 15.46

- **S-370 Mevcut durum:** türevi izlenen float64'ün H6040 denetimi liste ve demette en çok dört kat iniyor (`internal/stdlib/ml/builtins.go:247-249`); daha derindeki float64'ün yolu sınanmadı [hipotez].
- **S-371 Yapılacaklar (E, Ö3; sahip T1, sonraki dilim):** float64 türev yolu yok (15.32'nin float64 girdisi, `docs/spec/15_tensorler.md:54`'ün türevlenebilir `.TR_to_dtype`'ı, iyileştirici): çizgeye giren float64 H6040 (`internal/stdlib/ml/builtins.go:236-245`). Yol açılınca `gradFree` ve `trackedFloat64` girişleri kalkar.
- **S-372 Mevcut durum:** tensör dosyasında veri taşıyan tensör en çok 2³² öğe, verisiz şekil (sıfır eksenler 1 sayılarak) en çok 50 milyon öğe bildirebiliyor (`internal/stdlib/ml/error.go:188-209`); 15.46 taşma ve bellek denetimini istiyor. TR testi `tests/tensor_test.tr:1017` eski sınırın reddini bekliyor.
- **S-373 Yapılacaklar (E, Ö3):** `TR_AnyTensor` ve `.TR_narrow[E]()` yok (yalnız onaylı adlar listesinde); nicemlenmiş tür kodları (15.55); 2³² ve 50 milyon sınırlarının yerine int64 taşma ve bellek denetimi, sıfır eksen testi yeni kurala göre; TR testi.
- **S-375 Yapılacaklar (E, Ö3; sahip T1):** nicemli tensör TR biçimine ve safetensors'a yazılamıyor: yazma `t.Data()`'yı istiyor, o da H6687 (internal/stdlib/ml/save.go:96-97, `internal/compute/tensor/typed.go:172-173`). Sıradaki: 13 "q8" … 18 "q6_k" tür kodları ve 15.46'nın metni; "fp8"in düzeni OCP OFP8'in Tablo 1'i okununca.

### 15.47

- **S-376 Mevcut durum:** safetensors'ta `F8_E4M3` okunmuyor, H6643 veriyor; 15.47 onu ölçeği 1 olan "fp8" olarak okur (`internal/stdlib/ml/load_safetensors.go:39-40`'ın tür tablosunda F8 yok).

### 15.49

### 15.50

- **S-378 Mevcut durum:** float32'nin ucundaki büyük sonlu sayılarda CPU float64'te biriktirdiği için sonlu, GPU float32'de biriktirdiği için sonsuz sonuç verebiliyor (normlarda x², toplam float32'yi aşınca) [kanıtlı: `internal/compute/tensor/cpu_reduce.go:40`, `:51`]; 15.50 aynı sonucu ister.
- **S-379 Yapılacaklar (C, Ö2):** her türevli işlemin CPU ve GPU sonucunun float64 referansa karşı testi ve işlem başına hata sınırı yok; normlar ve toplamlar için var [test: `TestNormsMatchFloat64Reference`, `TestSumsMatchFloat64Reference`]. Sıradaki: GPU bf16 sapma tablosu, `TR_log10` ve `TR_atan2` türevi, büyük logitte çapraz entropi, `TR_logsumexp`, `TR_maximum`, `TR_minimum`; TR testi.

### 15.51

- **S-380 Mevcut durum:** "q4z128" ve "fp8" yok: bilinen nicemli türler "q8", "q4", "q4_k", "q6_k" (internal/compute/tensor/device.go:148-155); öbür ad bilinmeyen hassasiyet hatası verir.
- **S-382 Yapılacaklar (E, Ö3):** "q4z128" ve "fp8"; TR testi. Başka grup boyları, `Q4_1`, NF4 ve 2–3 bit Sonraya.

### 15.52

- **S-383 Mevcut durum:** nicemleme yok: `.TR_to_dtype("q8")` ve `TR_zeros` dışında değerden kurma H6691 (internal/stdlib/ml/dtype.go:116-117, `:241-243`).
- **S-384 Yapılacaklar (E, Ö3):** `.TR_to_dtype` ile nicemleme (llama.cpp ile bit bit, CPU ve GPU'da aynı kodlar; 15.52'nin sıfır bloğu, RTN ve fp8 kuralları), `TR_quantized` ve `TR_zero_point`, NaN ve aralık denetimleri (15.6'nın bekleyen işlemleri); `weight_scale_inv`'in çarpılan ölçek olduğunu gerçek bir FP8 modeliyle doğrulamak [hipotez]; TR testi. Sıra değiştiren gruplar Sonraya.
- **S-386 Yapılacaklar (A, Ö1; sahip H1, nicemleme uygulanırken):** 15.52'nin fp16 ölçeği sonsuza giden blok hatası (`docs/spec/15_tensorler.md:677`) uygulanmadı: nicemleme bugün H6691 (internal/stdlib/ml/dtype.go:116-117); kimse etkilenmiyor. `1 / d` sonsuz bloğun kodlaması aynı dilimde (D, Ö3).
- **S-387 Yapılacaklar (A, Ö1; sahip T1, tensör kolu; metin DIL UZMANI):** "q4z128"in GPTQ v1 sıfırını `(s + 1) & 15` ile açan, 16'da hata veren okuyucu yok [kanıtlı: `git grep -i "qzeros|zero_point|q4z128" lib/ internal/` yalnız onaylı adlar; ölçüldü: RT-2026-11-18-HIGHRAM-TCB-1]. Sıradaki: okuyucunun dilimi; testte aday 2'nin iki deseni ve gptqmodel 1.7.4'ün borç deseni baytlarla.

### 15.53

- **S-388 Mevcut durum:** türev izleyen girdinin nicemli ağırlıkla çarpımı H6696 (geri yön `g @ w` yok, `internal/compute/tensor/quant.go:245`); `internal/compute/tensor/linear_ce.go`'da nicemli yol yok; ARM'de NEON çekirdeği yok, Go eşi koşuyor (`internal/compute/tensor/cpu_quant_other.go`).
- **S-389 Yapılacaklar (D, Ö2; sahip H1):** G4'ün kartında okuma ve üretim llama.cpp'den yavaş, üretim 0,5 hedefinin altında [ölçüldü: TG-2026-11-19-G4GPU-H1-2, TG-2026-11-19-G4GPU-H1-3]. Sıradaki: S-562 ile birlikte (Q4_0'dan sonraki GPU işi).
- **S-390 Yapılacaklar (F, Ö3; sahip H1):** `CheckQuantScales` yalnız GGUF okumasında (`internal/stdlib/ml/load_gguf.go:97`); TR biçiminin nicemli tür kodları gelince `fileEntry.tensor`'da da çağrılmalı (internal/stdlib/ml/save.go:107-108), yoksa NaN ölçek ikinci yoldan dile açılır.
- **S-391 Yapılacaklar (E ve D, Ö3):** ARM NEON çekirdekleri; nicemli çarpımın geri yönü için ikinci çekirdek; nicemli `TR_linear_cross_entropy`; 2. kilometre taşının hız ölçütü. Nicemleme farkında eğitim (QAT) Sonraya.
- **S-392 Yapılacaklar (sınıfsız: bellek, Ö3; sahip T1 (tensör ve GPU), internal/compute/tensor ve internal/stdlib/ml):** yüklemede ağırlıktan aritmetikle türetilen kalıcı tensörler Go öbeğinde: öbek dışı blok yalnız yükleyicide ve bloktaki ağırlığın birleştirme, çevirme ve kopyasında (`internal/compute/tensor/host_block.go:207-220`). Sıradaki: bir kez kurulan tensörü bloğa koymanın yolu.

### 15.53a

- **S-396 Yapılacaklar (E, Ö3; sahip T1):** 15.53a'nın tek satırlı çarpımı (üretim) L4'te float yolundan yavaş [ölçüldü: TG-2026-11-24-L4GPU-T1-1]. Mevcut durum: GPU CPU'nun bitlerini verir [test: `TestQuantActMatchesCPUForEveryShape`]; çok satırlı çarpım sm_80+'ta `mma.sync` s8 (q4 karo), altında dp4a; sonlu olmayan satırda ara sonuç taşarsa GPU (`dot32`) ile AVX2'li CPU (`sdot32`) ayrılabilir [kanıtlı: `cpu_gemv_amd64.go:25`]. Sıradaki: Dilim C, C0; profil kart ailesini kapsar.

### 15.54

- **S-397 Yapılacaklar (E, Ö3):** nicemlenmiş yaprağa yüklemenin (20.6, 20.11, 20.14) aynı belleğe bütün olarak yazması (15.54) uygulanmadı: internal/stdlib/checkpoint'ta nicemli yol yok; TR testi.

### 15.55

- **S-402 Yapılacaklar (E, Ö3; sahip D1, aileler kolu):** transformers 5.17'nin Gemma 3 config'indeki iç içe `rope_parameters` (`full_attention`, `sliding_attention`) okunmuyor: alt anahtarlar yalnız rope_theta ve ölçek anahtarları (`lib/pretrained.tr:583`), H7160 [hipotez: çalıştırılmadı]. `layer_types` okunuyor (`lib/pretrained.tr:758-769`).
- **S-403 Yapılacaklar (C, Ö3; sahip D1):** Qwen3'te ve Llama 3'te sohbet şablonu denetlenmiyor: bilinen şablon listeleri boş (`lib/model_family.tr:25-26`), biçim yazımı `TR_gemma3_format` olarak sabit (`:119`), H7170'in şablon dalı Gemma 3'e göre (`internal/lang/messages/tr.json:1647`); klasörde şablon hiç denetlenmiyor (`:129`). Sıradaki: Qwen3 GGUF karmalarını Qwen'in şablonuyla karşılaştırmak.
- **S-405 Yapılacaklar (E ve F, Ö3):** kitaplık (`tr/pretrained`) GPTQ, AWQ ve FP8 parçalarını birleştirmiyor (`lib/`'de `qzeros` yok); nicemli tür kodları gelince `.safetensors` iletisi TR biçimini de önerir; TR testi.
- **S-407 Yapılacaklar (E, Ö3; sahip T1):** `TR_pretrained_model` Llama 3 safetensors'unda `rope_scaling` H6943 veriyor (`lib/pretrained.tr:700`); GGUF çalışıyor.
- **S-408 Yapılacaklar (F, Ö3; sahip T1):** GGUF yüklemesinin uçtan uca dış kanıtı yok: e2e testlerinde llama.cpp logitlerinden altın dosya yok (`cmd/trlang/e2e/behavior_ready_gguf_test.go`); küçük fikstürün logitleri altın dosya olabilir.
- **S-409 Yapılacaklar (F, Ö3; sahip T1, sonraki dokunuşta):** parça dizininin ad kuralı Windows'un ayrılmış adlarını (CON, PRN, AUX, NUL, COM1–9, LPT1–9) reddetmiyor (`lib/pretrained.tr:93`): `NUL`'da H6610, öbürlerinde H7911 yanlış nedeni söylüyor [ölçüldü: `27d242d5`]. Sıradaki: kural bütün ayrılmış adları reddeder.
- **S-410 Yapılacaklar (sınıfsız: bellek, Ö3; sahip T1, Dost'un model değiştirme işinden önce):** aynı süreçte ikinci yükleme tek yüklemeden çok bellek istiyor [ölçüldü: TG-2026-11-23-HIGHRAM-T1-2]. Sıradaki: ikinci yüklemenin içinde işaretli tek ölçü.
- **S-411 Yapılacaklar (sınıfsız: bellek, Ö3; sahip T1, ayrı iş):** `TR_build_pretrained_model` + `TR_load_parameters` yolu modeli tam boy sıfırla kurup ağırlığı üstüne kopyalıyor (`lib/pretrained.tr:260-261`, `_kur`); `TR_pretrained_model` parametre başına atıyor. Sıradaki: aynı atamayı bu yola da vermek.
- **S-412 Yapılacaklar (sınıfsız: bellek, Ö3; sahip T1):** bf16 yükleme katman başına dosyanın 7 bloğuna 2 birleştirme ve 5 kopya ekliyor; Linux'ta ağırlık payı 8 191 blok ve bırakılan blok eşli kalıyor (`internal/compute/tensor/host_block.go:74-77`, `host_block_linux.go:17-20`): aynı süreçte yaklaşık 16 yüklemeden sonra öbek yolu [hesap]. Sıradaki: Dost'un model değiştirmesi ve sunucu için TCB'nin kararı.
- **S-413 Yapılacaklar (sınıfsız: bellek, Ö3; sahip T1):** `TR_weight_dtype: "float32"`'de yüklemenin kopyası öbekte: `TR_Tensor` float32'yi `tensor.NewOn` ile kurar, bloğa giden `Clone` yalnız 16 bitlik türde (`internal/stdlib/ml/builtin_create.go:67`, `:71`). Sıradaki: işi yapan dilim kendi ölçüsünü alır.
- **S-414 Yapılacaklar (D, Ö3; sahip T1, sonraki dokunuşta):** birleşik katmanın giriş boyu kapısı dil modelinin önekini kendisi yazıyor (`lib/pretrained.tr:124`, `:143`), eşleme de (`lib/hugging_face.tr:64-68`, `:79`): eşleme yeni bir türün önekini öğrenip kapı öğrenmezse kapı sessizce koşmaz, ileti H7913 yerine H6311 olur [ölçüldü: `27d242d5`]. Sıradaki: önek eşlemeden alınır.

## 16. Veri ve parçalayıcı

Kurallar: [`docs/spec/16_veri_parcalayici.md`](spec/16_veri_parcalayici.md).

### 16.1

- **S-415 Yapılacaklar (E, Ö3; sahip veri ve parçalayıcı kolu):** GGUF parçalayıcısı `smaug-bpe` ön bölme etiketini tanımıyor, bu etiketli Llama 3.1 GGUF'u H6679 ile yüklenmiyor [kanıtlı: `internal/compute/tokenize/gguf.go:70-85`, `:259`]. Sıradaki: etiket ve 15.55'in aile tablosu.

### 16.2

- **S-416 Mevcut durum (ölçülmemiş):** `llama3` ailesinin boşluk kümesi boş yazılı [kanıtlı: `internal/compute/tokenize/family.go:94`]; resmî Llama 3 `tokenizer.json`'unda boşluk parçası olmadığı ölçülmedi [hipotez].

- **S-418 Yapılacaklar (B, Ö1; sahip parçalayıcı dilimi):** 16.2'nin `TR_boundary_tokens:` argümanı ve verilmeyen `TR_special_tokens:`'ın metin içi yazımdaki hatası motorda yok; `TR_special_tokens: TR_false` sınır parçalarını da kapatıyor [kanıtlı: internal/stdlib/tokenizer/tokenizer.go:323]. Sıradaki: göçte 14 `.tr` dosyasındaki 71 `TR_special_tokens: TR_false`'a `TR_boundary_tokens: TR_false` eklenir.

### 16.3

- **S-419 Yapılacaklar (D, Ö3; sahip parçalayıcı dilimi):** aynı içerikle tekrar yazılmış, bayrakları farklı eklenmiş parça yükleme hatası değil, son kaydın bayrakları alınıyor [kanıtlı: internal/compute/tokenize/tokenizer.go:279-283]; normalleşince çakışan `normalized` parçaların denetimi de yok [hipotez].
- **S-420 Yapılacaklar (C, Ö2; sahip TCB, dil tanımı turu):** aynı anda yaşayan tensörlerin toplamına sınır yok (15.42 tensör başına): büyük `p.TR_vocab_size`'la kurulan `TR_GPT` belleği doldurup TR hatası vermeden öldürülüyor [ölçüldü: RT-2026-11-04-HIGHRAM-T1-1]. Sıradaki: dil tanımı turunda 15.42'nin toplam sınırı.

### 16.3a

- **S-421 Mevcut durum:** akışlı çözme (16.3a) yok; `p.TR_decode` yarım karakterde U+FFFD veriyor [kanıtlı: internal/compute/tokenize/encode.go:160, `lossyUTF8`].
- **S-422 Yapılacaklar (E, Ö3; sahip T1):** `TR_DecodeStream`, `.TR_step`, `.TR_finish`; dağarcıkta olmayan kimlik akışta da 16.3'teki gibi hata; birleşimin toplu çözümle aynılığını sınayan TR testi. Sıradaki: DOST'un cümle cümle konuşmasından önce.

### 16.4

- **S-423 Mevcut durum:** `TR_train_tokenizer` (16.4) yok [kanıtlı: `internal/lang/codes/definitions.json`'da kaydı yok].
- **S-424 Yapılacaklar (E, Ö3):** 16.4'ün bayt düzeyinde belirlenimci BPE eğitimi (dosya listesiyle de), `.TR_save` ile `tokenizer.json`; Hugging Face ile bir kez sınanır; TR testi.

### 16.5

- **S-426 Yapılacaklar (sahip D1):** okuyucu `int32`'ye sığmayan `<u4` ve `<i8` değerini H5086 ile reddediyor, 16.5–16.7 `int64` diyor [kanıtlı: `internal/compute/dataload/shard.go:136`]. Sıradaki: okuyucu `int64`'e geçer, `TestShardU32Overflow`'un iki satırı ters döner.
- **S-427 Mevcut durum (C, Ö2):** veri kümesi önbelleğinin kapanışı işçiyi süresiz bekliyor [kanıtlı: `internal/compute/dataload/cache.go:695-696`]; işçinin okuması takılırsa aynı klasörün açılışı iletisiz bekler, ilk Ctrl+C kesmez [hipotez].

### 16.6

- **S-428 Mevcut durum:** `TR_write_dataset` (16.6) yok; Go'da `WriteNpyU16` ve `WriteNpyI32` var [kanıtlı: `internal/compute/dataload/npy.go:512`, `:525`].
- **S-429 Yapılacaklar (A adayı, diskteki veri; sessiz mi ölçülmedi):** `TR_write_dataset`: tür seçimi, `meta.json`, var olan kümenin yerine atomik yazma; TR testi.

### 16.7

- **S-431 Mevcut durum:** örnekleyicinin ve sıralı okuyucunun `.TR_next()`'i iki öğeli liste veriyor, 16.7 ve 16.8 demet diyor [kanıtlı: `internal/stdlib/dataset/dataset.go:538`].
- **S-432 Yapılacaklar (D, Ö3):** `.TR_next()`'in demet dönüşü (3.6a); göç: `.TR_next()`'i `[0]`/`[1]` ile okuyan yerler.

### 16.8

- **S-434 Yapılacaklar (D, Ö3; sahip D1; 3.6a'nın göçüyle):** `TR_topk` iki öğeli liste veriyor, 6.3 demet diyor [kanıtlı: `internal/stdlib/ml/builtin_sort.go:98`]; okuyucular S-431'de.
- **S-435 Yapılacaklar (D, Ö3; sahip veri hattı kolu (D1)):** denetleyici `.TR_next()`'e ve döngüdeki okuyucu öğesine tip vermiyor: `TR_var y: TR_Int = s.TR_next()` denetimden geçiyor, çalışırken H1001 [ölçüldü: ae1e8dcc]. Sıradaki: demet ve iki `TR_Tensor[TR_Int]` tipi denetime girer, S-432 ile birlikte.
- **S-436 Yapılacaklar (F, Ö3; sahip D1):** `docs/codes.md`'nin iki açıklaması okuyucunun öğesini "[girdi, hedef]" liste yazımıyla anlatıyor, 16.8 demet diyor: `TR_Sampler` (`docs/codes.md:1197`) ve `.TR_next()` (`:7606`).

## 17. Kodlar

Kurallar: [`docs/spec/17_kodlar.md`](spec/17_kodlar.md).

- **S-437 Yapılacaklar (A, Ö1; sahip D1, codegraph dilim 5'ten sonraki dilim):** silinen ad kodunun numarası yeniden kullanılırsa yakalayan denetim yok: emekli listesi yalnız ileti kodlarında (`internal/lang/messages/retired.json`, `tools/gen/msgconsts/main.go:102`), ad ailesi yalnız elle `codegraph deleted` ile görülüyor (`tools/codegraph/deleted.go:26`) [kanıtlı]. Sıradaki: ad kodlarına emekli listesi ve üreteçte denetim.

### 17.1

### 17.4

- **S-438 Mevcut durum:** kitaplık kodlarının açıklaması ve örneği yok; `TestEveryCodeHasDescription` onları atlıyor [kanıtlı: `internal/lang/codes/definition_test.go:63-64`].
- **S-439 Yapılacaklar (F, Ö3):** kitaplık kodlarının açıklaması ve örneği; `TestEveryCodeHasDescription`'daki atlama kalkar.
- **S-440 Mevcut durum:** komut satırı kodlarının örnekleri program olarak sınanmıyor [kanıtlı: `internal/stdlib/behavior_definitions_test.go:103`].
- **S-441 Yapılacaklar (D, Ö3):** komut satırı örneklerinin sınanması (adları 17.9'un girdisiyle).

### 17.5

- **S-442 Mevcut durum:** dayanağı kalmayan ASCII yolu duruyor: `AcceptASCII` (kapalı) ve `TestASCIIFormsDoNotClash` [kanıtlı: internal/lang/codes/language.go:39, `:171`]. Sıradaki: kaldırmak.

### 17.7

- **S-444 Mevcut durum:** sözcük çözümleyicinin yazım dilinde okuma kipi (`lexer.NewBodyIn`'in kodsuz kipi, `$` kaçışı, denetleyicinin `SetFileLanguage`'ı) üretimde çağrılmıyor, yalnız Go testlerinde [kanıtlı: `internal/lang/lexer/coded.go:34`, `internal/lang/checker/checker.go:377`]. Sıradaki: kaldırmak, ayrı iş.
- **S-445 Yapılacaklar:** iki sınav yok: `TestDisplayExam` (kişiye görünen her yüzey okuyanın dilinde) ve `TestNameClashExam` (başka dilin sözcüğü kullanıcı adı olunca hata ya da aynı sonuç) [kanıtlı: `git grep` ile depoda yok]. Sıradaki: 17 yeniden yazılınca kodlu dosyalar ve görünümle kurulur.

### 17.8

- **S-446 Mevcut durum:** `TR_GPT`'nin `.TR_config` sözlüğünün anahtarları Türkçe ve İngilizce karışık ("dağarcık", "bağlam", "embed_dim", "num_heads"); kayda giderse anahtarlar diske Türkçe yazılır [kanıtlı: `lib/layers.tr:575`].
- **S-447 Yapılacaklar (F, Ö3):** `.TR_config`'in anahtarları kimlik adlarıyla ("vocab_size", "context" ...) ya da sözlük yerine bir yapı; göç: `.TR_config`'i okuyan yerler (`lib/layers.tr:713` gibi); TR testi.

### 17.10

- **S-448 Yapılacaklar (F, Ö3):** `docs/spec/19_surec_ve_kayit.md:85`'in "(17.3)" göndermesi 17.8 olmalı (dosya biçimi çevrilmez) [kanıtlı: `docs/spec/19_surec_ve_kayit.md:85`].

## 18. Dosya ve yol

Kurallar: [`docs/spec/18_dosya_ve_yol.md`](spec/18_dosya_ve_yol.md).

### 18.1

- **S-449 Yapılacaklar (B, Ö1, asistan araçları dilimi; sahip D1 (tasarım ve uygulama)):** `TR_Path` tek parçada birleşen metni denetlemiyor: `ad` `"../x"` iken `TR_Path("veri/\(ad)")` ve `TR_Path("veri/" + ad)` `x` veriyor [ölçüldü: ae1e8dcc]; metin yol alan yerleşikler ve bağlantı üzerinden çıkış da [hipotez]. Sıradaki: asistan araçları yazılırken tartılır.
- **S-450 Yapılacaklar (D, Ö3; sahip D1):** Windows aygıt adı (`NUL`, `CON.txt`) yol parçası olarak hatasız, 18.1 yazmıyor [ölçüldü: ae1e8dcc]; Unix'te `C:/x` parçası mutlak sayılmıyor, 18.1 koşulsuz diyor [kanıtlı: internal/stdlib/files/path.go:55].

### 18.3

- **S-451 Mevcut durum (ölçülmemiş):** `.TR_modified_ns` ve `.TR_disk_available_bytes` macOS'ta sınanmadı; CI yalnız Windows ve Linux [kanıtlı: `.github/workflows/ci.yml:37`].

### 18.5

- **S-452 Mevcut durum:** Windows'ta `TR_list_dir`'in kalıbı harf büyüklüğüne bakmıyor (`"*.TXT"` `a.txt`'yi eşliyor), `TR_ignore_case:` yok [ölçüldü: ae1e8dcc].
- **S-453 Yapılacaklar (C ve E, Ö2):** her sistemde harf büyüklüğüne duyarlı eşleme ve `TR_ignore_case:`; göç: `internal/stdlib/files/list_test.go`'nun harf büyüklüğüne bakmayan dalı; TR testi.

### 18.9

- **S-454 Yapılacaklar (A adayı, diskteki veri; sessiz mi ölçülmedi):** `TR_move` dolu klasörün yerine konarken arada kesilirse eski klasör geçici `.eski` adıyla kalıyor, bunu söyleyen ileti yok [kanıtlı: `internal/stdlib/files/copy_run.go:67-84`]; dolu klasörün yerine taşımayı sınayan Go testi yok.
- **S-455 Yapılacaklar (C, Ö2; sahip Dost Interface (dosya ve yol)):** kopyalama ve taşıma değişme zamanını korurken `os.Chtimes`'ın hatasını yok sayıyor [kanıtlı: `internal/stdlib/files/copy_run.go:116`]; zaman korunmazsa boy ve zamanla karmayı atlayan program dosyayı boşuna yeniden karmalar. Sıradaki: hatayı bildirmek.

### 18.14

### 18.16

- **S-458 Mevcut durum (açık dil kararı):** okumanın da tek klasöre inip inmeyeceği açık: `.TR_exists` programın klasöründeki dosyayı görüyor, `TR_remove` görmüyor, H2630 [ölçüldü: ae1e8dcc]; `TR_absolute` modülden çağrılınca modülün dosyasını veriyor [hipotez].

### 18.17

- **S-459 Yapılacaklar (E, Ö3):** `TR_write_lines` ve `TR_LineWriter` yok (bilinmeyen kimlik) [ölçüldü: ae1e8dcc]. 18.17'nin `TR_owner_only:`'si de yok. Sıradaki: 18.17'nin hepsi; yol `ResolveWritePath`'le, ertelenen `.TR_close()` çıkışın türüne göre işler ya da siler; TR testi.

## 19. Süreç ve kayıt

Kurallar: [`docs/spec/19_surec_ve_kayit.md`](spec/19_surec_ve_kayit.md).

### 19.2

- **S-461 Mevcut durum:** `TR_command_line` seçenekleri sözlükle alıyor ve sözlük veriyor, 19.2 yapı diyor [ölçüldü: ae1e8dcc].
- **S-462 Yapılacaklar (E ve D, Ö3):** yapıyla yazım ve `TR_descriptions:`'da bilinmeyen adın denetim hatası; göç: 26 `.tr` dosyasındaki 33 `TR_command_line(`, `TestOptionNameIsTheTextKey`, rehber ve örnekler; TR testi.

### 19.4

- **S-463 Yapılacaklar (F, Ö3):** gizli sözcükler Go'da duruyor, dil dosyalarına (`tr.json`, `en.json`) taşınır [kanıtlı: `internal/stdlib/process/process.go:122-123`].

### 19.4a

### 19.5, 19.7

- **S-466 Yapılacaklar:** başlangıç kaydı bağlı çubuğun toplamını `total` olarak yazıyor, 19.5 onu çubuğun ilk kaydına koyuyor [kanıtlı: `internal/stdlib/logging/record.go:323-325`].

### 19.6

- **S-467 Yapılacaklar:** günlük yapı değerini yazamıyor: kayıt 13.8'in yazıcısından geçiyor (`internal/stdlib/logging/record.go:308`), o yazıcı yapıyı reddediyor (S-202); 19.6 alanları yazılabilen yapıyı nesne olarak ister [kanıtlı].

### 19.10

- **S-468 Mevcut durum:** komut satırında mantık değerinin Türkçe yazımı (--hizli=doğru) okunuyor, yardım varsayılanı kişinin dilinde ("(varsayılan: yanlış)") [ölçüldü: ae1e8dcc].
- **S-469 Yapılacaklar (D, Ö3):** mantık değerinde yalnız "true" ve "false"; yardımın varsayılanları komut satırının yazımıyla; Türkçe mantık değerini sınayan Go testleri (`internal/stdlib/process/arguments_test.go:38`, `process_test.go:67`) değişir.
- **S-470 Yapılacaklar:** yardım `TR_none` varsayılanını yazıyor ("(varsayılan: yok)"), 19.10 yazmaz [ölçüldü: ae1e8dcc].

### 19.12

- **S-471 Mevcut durum (ölçülmemiş):** `TR_machine()` macOS'ta sınanmadı, orada kullanılabilir bellek ve sınır H7735 verir; CI yalnız Windows ve Linux [kanıtlı: `.github/workflows/ci.yml:37`, internal/stdlib/machine/limit_other.go:5].
- **S-472 Mevcut durum (ölçülmemiş):** Dost'un toplayıcı payı ölçülmedi; H8894'ün eşiği tek makinede seçildi, tavana yakın bekleyen süreçte uyarı çıkabilir; `TR_cmd_watch` ve `TR_cmd_profile`'da uyarının satırı tablonun dışına düşebilir [hipotez].
- **S-473 Yapılacaklar (C, Ö2; sahip H1):** bellek tavanının (19.12a) eğitimdeki hız bedeli ölçülmedi: `examples/tr/36_karakter_modeli.tr` tavansız ve tavanla; Dost'un toplama payı S-498'de. Sıradaki: işi yapan dilim kendi ölçüsünü alır.
- **S-474 Yapılacaklar (C, Ö2; sahip Motor, bellek tavanı 19.12a):** tensör paketinin host blok ve bekleme testleri `-count=2 -shuffle=on`'da düşüyor (`TestHostSettleWithConcurrentCleanups`, `TestGateKeepsTheBlockDuringTheCall`, `TestHostBlockCountsTowardTheMemoryLimit` ve altısı), tohum 1795440944122239986 [ölçüldü: zorlama işi 45185143141]. Sıradaki: tohumla yerelde yeniden üretmek.
- **S-475 Yapılacaklar (C, Ö2; sahip H1, tools/bench taban testleri):** GOGC=1 adımında alt süreçteki trlang H8893'ü çıktıya yazıyor (`cmd/trlang/memory_limit.go:64`), `TestBaselineEmpty` ve `TestBaselinePureComputeTakesTheSize` boş ya da yalnız değer bekliyor (`tools/bench/behavior_baseline_programs_test.go:283-284`) [kanıtlı]. Sıradaki: test alt sürecin ortamını kendisi kursun.

## Bölümü olmayan işler

### Hız

- **S-517 Yapılacaklar (sınıfsız: hız, Ö3; sahip T1):** işlemcinin tensör havuzu bloğu tam boyuyla eşliyor (anahtar tip ve öğe sayısı) [kanıtlı: internal/compute/tensor/cpu_memory.go:26-29, :94, :130]. Sıradaki: boy sınıflarıyla havuz.
- **S-537 Mevcut durum (Colab CPU High-RAM, emsal kıyası):** okumada TR llama.cpp'den hızlı; üretimde EPYC'te TR yavaş, Xeon'da ayrışmıyor [ölçüldü: TG-2026-11-23-HIGHRAM-T1-4 (EPYC), TG-2026-11-23-HIGHRAM-T1-6 (Xeon)]. MMS Türkçede TR transformers'tan yavaş [ölçüldü: TTS-2026-11-17-HIGHRAM-S1-14].
- **S-538 Mevcut durum (dizüstü i7-13700H, emsal kıyası):** Whisper `small` Türkçede TR faster-whisper float32'den hızlı, int8'den yavaş [ölçüldü: STT-2026-11-22-LAPTOP-H1-3, -4]; Qwen3-4B `Q4_K_M`'de okumada TR llama.cpp'den hızlı, üretimde ayrışmıyor [ölçüldü: TG-2026-11-23-LAPTOP-T1-2]; MMS Türkçede TR transformers'tan hızlı [ölçüldü: TTS-2026-11-17-LAPTOP-S1-20].
- **S-558 Yapılacaklar (sınıfsız: hız, Ö3; sahip T1):** MMS Türkçede TR transformers'tan dizüstünde hızlı, Colab CPU High-RAM'de yavaş [ölçüldü: TTS-2026-11-17-LAPTOP-S1-20, TTS-2026-11-17-HIGHRAM-S1-14]; makineye göre ayrılmanın nedeni ölçülmedi [hipotez: az çekirdekte ölçekleme]. Sıradaki: aynı makinede `GOMAXPROCS` 4 ve 8 ile iş parçacığı ölçeklemesi, TR ve emsal aynı sayıda.
- **S-561 Mevcut durum (nicemli okuma):** nicemli açıkken çok satırlı okumada Q4_K çarpımı etkinleştirme satırlarını sekizli gruplarla çarpıyor (`dotQ4KQ8x8`), bitler tek satırlı yolla aynı [kanıtlı: `internal/compute/tensor/cpu_quant_q8.go:366`; test: `TestQ8EightRowsMatchOneRow`].
- **S-562 Yapılacaklar (E, Ö3; sahip T1; H1 8B/12B, bulgu 1; TCB 2026-11-17 15:02):** GPU'da (Colab L4) okuma çağrı başına sabit bir bedel taşıyor: Qwen3-8B'de ve Gemma 3 12B'de okumanın süresi token sayısıyla az, çağrıyla çok büyüyor; llama.cpp'nin aynı okuması bu payın çok altında [ölçüldü: H1, `speed_measurement.md`'nin "2026-11-17 14:47 — H1" bölümü; hesap: H1]. Sıradaki adım: Q4_0'dan sonraki GPU işi (Ahmet 13:26), önce H1'in L4'te okuma profili (kart zamanı, aktarım, eşitleme).
- **S-563 Yapılacaklar (E, Ö3; sahip T1; H1 8B/12B, bulgu 2; TCB 2026-11-17 15:02):** GPU'da Gemma 3 12B'nin üretimi bağlam derinliğiyle (200 → 700) Qwen3-8B'den ve llama.cpp'den çok daha fazla yavaşlıyor [ölçüldü: H1, aynı bölüm]; [hipotez: H1, kartta dikkat adımı; Gemma 3'ün bakış boyutu 256]. Sıradaki adım: aynı GPU işinde dikkatin derinlikle profili.
- **S-564 Yapılacaklar (E, Ö3; sahip H1):** nicemli çekirdeklerde AVX-512 VNNI yolu yok: VNNI komutu 256 bitlik VEX biçiminde, nicemli assembly'de 512 bitlik yazmaç yok [kanıtlı: `internal/compute/tensor/cpu_quant_panel_amd64.s:4`]; llama.cpp L4'te `cascadelake` kodunu seçiyor [kanıtlı: llama.cpp'nin yükleme satırları]. Sıradaki: L4'te profil, sonra Q4_K'ya 512 bitlik VNNI yolu.
- **S-565 Yapılacaklar (sınıfsız: hız, Ö3; sahip T1):** AMD EPYC 7B12'de okuma K8 eşiğinin (T1'in işlemcide metin hızı tasarımı) üstünde, Intel Xeon'da altında [ölçüldü: TG-2026-11-23-HIGHRAM-T1-4 (EPYC), TG-2026-11-23-HIGHRAM-T1-6 (Xeon)]. Sıradaki: Colab EPYC verdiğinde `b643e7ea`, `4cf97952` ve llama.cpp'yle tek koşu; tutmazsa panel yolunun profili.
- **S-566 Yapılacaklar (sınıfsız: hız, Ö3; sahip H1):** yeni kurulum büyük bellekli makinede Qwen3 8B'yi öneriyor [kanıtlı: `tools/assistant/katalog_test.tr:253`], ama 8B ile 4B işlemcide aynı koşuda ölçülmedi: TG-2026-11-23-LAPTOP-T1-6 (8B) ve TG-2026-11-23-LAPTOP-T1-2 (4B) ayrı koşular. Sıradaki: Colab'da TR ve llama.cpp, Qwen3 4B ile aynı koşuda.
- **S-571 Mevcut durum (tek sorgulu dikkat):** işlemcide dikkatin tek sorgulu adımı puanları nokta çarpımıyla, değerleri şeritlerle hesaplıyor; istem okuma ve eğitim blok çarpımında, bitler yalnız toplama sırası kadar değişiyor [test: `TestDecodeAttentionMatchesFused`].

### Bellek

- **S-498 Yapılacaklar (sınıfsız: bellek, Ö3; sahip H1):** Dost'un tepe belleğinin büyük kısmı Go'nun toplama payı [ölçüldü: AS-2026-11-12-G4CPU-H1-1]; dizüstünde `GOMEMLIMIT` tepeyi düşürdü ama tepe tavanın üstünde kaldı, iki koşu kirli [ölçüldü: AS-2026-11-15-LAPTOP-H1-3]. Sıradaki: Dost'un başlatıcısında `TR_MEMORY_LIMIT` varsayılan olmadan önce temiz ABAB ölçüsü.
- **S-506 Yapılacaklar (E, Ö3; sahip T1):** izin alamayınca bırakılan `MatMulKernel`'in bitişik kopyaları havuza dönmüyor: iade k.end'de, `Kernel.Abandon` onu çağırmıyor [kanıtlı: internal/compute/tensor/matmul.go:121, :149-150; internal/compute/tensor/access.go:150-155]. Sıradaki: `Abandon`'ın `begin`'siz kolu k.end'i de çağırır.
- **S-519 Yapılacaklar (sınıfsız: bellek, Ö3; sahip H1):** QAT modelinin dizüstündeki tepe özel belleğinin bir kısmı Go yığınının çöp toplama öncesi tepesi [ölçüldü: TG-2026-11-17-LAPTOP-T1-3]; `TR_MEMORY_LIMIT` ya da düşük `GOGC` tepeyi indirebilir [hipotez]. Sıradaki: Colab'da `GOGC` taraması, bellek ve hız birlikte.
- **S-542 Mevcut durum (model değiştirme):** yükleyici dosyadan büyük bir yüklemeden önce bırakılmış öbek dışı blokların temizliğini bekliyor, süre dolarsa H6730 uyarır ve yükleme sürer [test: `TestHostSettleWaitsForTheReleasedBlock`, `TestHostSettleIgnoresBlocksAllocatedMeanwhile`, `TestHostSettleWithConcurrentCleanups`, `TestLoadWaitsForTheReleasedBlocks`, `TestHostSettleWarning`]. Windows'ta beklemenin süresi ölçülmedi.
- **S-543 Mevcut durum (takas):** iptal edilen çağrının ortamı yorumlayıcının havuzunda kalmıyor [test: `TestACancelledCallLeavesNoEnvironmentInThePool`]; uygunluk hesabının geçicisi kv × bağlam (`geri_al`'ın yeni önbelleği) [test: `tools/assistant/katalog_test.tr`].
- **S-545 Yapılacaklar (C, Ö2; sahip zincirin sahibi, tek tık B diliminin uygunluk hesabı):** `geri_al` her tur hatasında yeni önbellek kuruyor, eskisi öbekte ölüyken yenisi ayrılıyor [kanıtlı: `tools/assistant/zincir.tr:134-141`]. Sıradaki: aynı tampon `TR_truncate` ile yeniden kullanılır.
- **S-556 Yapılacaklar (D, Ö3; sahip Dost Interface, P'den önceki dilim):** katalogun ölçü alanlarında nicemli durum yok, gereken bellek hesabı ayarın nicemli seçimini almıyor [kanıtlı: `tools/assistant/katalog.tr:276`, `:517`]; nicemli kapalıyken bellek ölçünün üstüne çıkabilir [hipotez]. Sıradaki: kapalı durumun belleğini ölçmek ya da ölçüye nicemli durumu eklemek.
- **S-557 Yapılacaklar (sınıfsız: bellek, Ö3; sahip Dost Interface, sonraki dilim):** gereken bellek en büyük bağlamlı ölçüden doğru çiziyor, en küçüğü taban alıyor, ölçülü noktalar arasında ara değer yok [kanıtlı: `tools/assistant/katalog.tr:517-530`]; ölçüler en çok 7981 token [kanıtlı: `tools/assistant/katalog.json`]. Sıradaki: ara değer ve 32768 token'da üçüncü nokta.
- **S-569 Mevcut durum (önbellek bloğu):** sabit önbelleğin işlemcideki tamponları bütün katmanlar için tek öbek dışı blokta (`TR_cache_buffer`, 11.7a, 15.43), önbelleğin kendi payında; pay dolunca önbellek öbekte, bitler aynı [kanıtlı: `lib/layers.tr:502`; test: `TestKVCacheLogitsSameInBlockAndHeap`, `TestKVCacheBuffersUseHostBlocks`, `TestCacheShareLeavesWeightShare`].
- **S-570 Yapılacaklar (C, Ö2; bellek; sahip H1):** yalnız dil modeli düzeninde (bağlam 8 192) TR'nin tepe özel belleği llama.cpp'nin ağırlığı belleğe okuyan kipinden (-lm none) büyük; önbellek öbek dışında (S-569), kalan fark Go'nun payı [ölçüldü: TG-2026-11-09-LAPTOP-H1-1; hesap: H1]. Bitti: aynı makinede ve istemle tepe, önbelleği öbek dışı düzenin hesabının altında [hesap: DENETİM 335]. Sıradaki: 16 bit önbellek (S-572), sonra Go'nun payının dökümü.
- **S-572 Yapılacaklar (C, Ö3; sahip H1):** 16 bit önbellek (fp16, bf16) commit'siz denemede karmaşıklık şartını geçmedi [ölçüldü: TG-2026-11-16-HIGHRAM-H1-7]. Sıradaki: kullanılmamış üç metinle ve önceden yazılmış şartla yeniden ölçü; geçerse öntanımlı sorusu Ahmet'e.
- **S-573 Yapılacaklar (C, Ö3; sahip R1'in uygulayıcısı):** yeni konuşmada önbellek yeniden kuruluyor, eskisinin bloğu toplayıcıyı ve temizleyiciyi beklerken iki önbellek birlikte duruyor [kanıtlı: `tools/assistant/zincir.tr:84-86`, `internal/compute/tensor/host_block.go:115-119`]. Sıradaki: R1'de önbellek `TR_truncate(0)` ile yeniden kullanılır.
- **S-574 Yapılacaklar (D, Ö3; geliştirici belgesi; sahip H1, sonraki dilim):** `bufTotalMax`'ın yorumu yığını paylaşan yolların birbirini etkilediğini söylemiyor [kanıtlı: `internal/compute/tensor/cpu_matmul.go:81-83`]; `quantPanelMaxM`'in `bufMax` hesabı yalnız 4B'ye göre, daha geniş modelde paketli A havuzlanmaz [kanıtlı: `internal/compute/tensor/cpu_quant.go:85-87`, `:94`]. Sıradaki: yoruma paylaşımın sonucu; büyük modelde sınır gözden geçirilir.

### Ses

- **S-541 Yapılacaklar (D, Ö3; sahip tek tık kurulumun kolu):** Whisper'ın tanımadığı dil kodu H7098'i ancak ilk yazıya çevirmede, modeller yüklenip kullanıcı konuştuktan sonra veriyor [kanıtlı: `lib/whisper.tr:388`, `:392`]. Sıradaki: Whisper'ın dil listesine açık bir yol, dil yüklemede bir kez sınansın.
- **S-546 Mevcut durum (Silero VAD):** Dost'un kataloğu `snakers4/silero-vad` v6.2.3'ün `silero_vad_16k.safetensors`'unu veriyor [kanıtlı: `tools/assistant/katalog.json:311-323`]; safetensors ile JIT'in ağırlıkları aynı değil ve safetensors için yayımlanmış kalite ölçüsü yok (`docs/spec/rationale/25_speech.md:64`).
- **S-576 Yapılacaklar (D, Ö3; sahip TCB):** konuşmanın bittiğini sesten anlayan sıra alma modeli (ör. Smart Turn, omurgası Whisper Tiny) bitiş sessizliğinin (`tools/assistant/zincir.tr:16`) adayı; ağırlıkların ONNX dışında bulunup bulunmadığı açık [kaynak: smart-turn README]. Sıradaki: ilk sürümün ölçüsünden sonra.

### Dost

- **S-481 Yapılacaklar (C, Ö2; sahip Dost Interface):** `startProgram` süreç hazır olmadan çıkınca iletiyi süreci beklemeden yazıyor, standart hata boş görünebilir (`TestDostReportsAnInterruptedLoad/bozuk`) [kanıtlı: tools/assistant/behavior_screen_server_test.go:95, :86-89]. Sıradaki: önce süreci bekler, sonra çıkış kodunu ve standart hatayı yazar.
- **S-482 Yapılacaklar (C, Ö3; sahip Dost Interface):** bekleyen `model_load` varken gelen dil ya da sistem iletisi ayarı önce eski takıma uygulanıyor (`dili_uygula`), yeni dilin modelleri bir kez fazla yüklenir [kanıtlı: tools/assistant/oturum.tr:195-197, :303-313]. Sıradaki: `e.yüklenecek` doluyken `dili_uygula` atlanır; sırayı sınayan test.
- **S-483 Yapılacaklar (C, Ö3; sahip Dost Interface):** test yardımcısı `tek` kuyrukta başka komut varken kendi cevabını beklemeden `komutları_işle`'ye geçiyor ve `g.TR_wait()` süresiz; sonuç takılma [kanıtlı: tools/assistant/screen_helpers.tr:112-117]. Sıradaki: `tek` bekleyen ayar olmadığını (`e.ayar_istendi == TR_none`) doğrular, bekleme süre sınırlı olur.
- **S-484 Yapılacaklar (F, Ö3; sahip Dost Interface):** Modeller penceresinde makinenin belleği ikili (bayt / 2³⁰), modelin belleği onlu (bayt / 10⁹) yazılıyor, birimin adı ikisinde aynı [kanıtlı: tools/assistant/js/models.js:178-180, :259; tools/assistant/js/screen_state.js:101-102, :112]. Sıradaki: iki boy tek birimle.
- **S-485 Yapılacaklar (E, Ö3, P'den sonra ayrı dilim; sahip Dost Interface):** `catalog` olayında `license` yok, Modeller penceresi lisansı göstermiyor; Türkçenin iki sesi ticari olmayan lisanslı [kanıtlı: tools/assistant/contract.md:222; tools/assistant/katalog.json:384, :484]. Sıradaki: `catalog`'a `license`, pencerede lisans adı ve bağlantısı.
- **S-486 Yapılacaklar (F, Ö3; sahip Dost Interface):** trlang format --check tools/assistant üç dosyada biçimlenmeli diyor (asistan.tr:220, ayarlar.tr:82, ekran.tr:122); denetimlerin biçim adımı `tools/assistant`'ı taramıyor [kanıtlı: tools/full_check.sh:1216]. Sıradaki: trlang format ile biçim commit'i.
- **S-696 Yapılacaklar (F, Ö3; sahip TCB):** commit öncesi ve G4 denetiminin biçim adımı `tools/assistant`'ı taramıyor: format listesi `tools/gpu tools/bench/tr`'yle bitiyor [kanıtlı: tools/full_check.sh:1216]. Sıradaki: sonraki kod gönderiminde listeye girer.
- **S-487 Yapılacaklar (E, Ö3; sahip dil çekirdeği, sonra Dost Interface):** Dost'un sayfasındaki bellek kutusu hep "—": sürecin belleğini okuyan yerleşik olmadığı için `metrics`'te `memory_bytes` yok [kanıtlı: tools/assistant/js/view.js:251-254; tools/assistant/asistan.tr:553]. Sıradaki: dil çekirdeği B3'ten sonra o yerleşiği tasarlar.
- **S-489 Yapılacaklar (F, Ö3, sonraki dilim; sahip Dost Interface):** Modeller satırında silme onayı ve inişin hata metni (`dl-long`) `m-meta` ile aynı ızgara hücresine çiziliyor [kanıtlı: tools/assistant/js/models.js:231, :245, :267; tools/assistant/css/screen.css:332, :371]. Sıradaki: `m-meta`'yı ya da `dl-long`'u ayrı sıraya almak; görünüm testinde iki kutunun kesişmediği sınama.
- **S-490 Yapılacaklar (F, Ö3, sonraki Dost dilimi; sahip Dost Interface):** `sesi_eşitle` ses yüklenemeyince yeniden denemiyor, `selected_for` konuşmayan sesi gösteriyor; yolun testi yok [kanıtlı: tools/assistant/oturum.tr:432-438]. Sıradaki: `selected_for` konuşan sesi gösterir ya da sayfa bunu yazar; `ses_değiştirici`'nin hata döndüğü test.
- **S-491 Yapılacaklar (E, Ö3, sonraki Dost diliminin ilk işi; sahip Dost Interface):** ayarda mikrofon ve `--aygıt` yoksa mikrofon açılmıyor (`mic_unavailable`), seçilen mikrofon yalnız yeniden açılışta geçerli [kanıtlı: tools/assistant/asistan.tr:330; tools/assistant/oturum.tr:165, :515-517]. Sıradaki: Dost ilk mikrofonu kullanıp ayara yazar ya da sayfa seçtirir.
- **S-492 Yapılacaklar (E, Ö3; sahip Dost Interface):** Dost'un ayarı ve sayfası kartta açık nicemliyi reddediyor [kanıtlı: tools/assistant/ayarlar.tr:65-66, js/view.js:239-240, js/mock_models.js:192-193]; komut satırı ve otomatik kartta çalışıyor (nicemli.tr:52, :107). Eski metin: ayarlar.tr:57, asistan.tr:67, ekran.tr:70, README.md:39, contract.md:300, js/view.js:229-230; testler: behavior_screen_look_test.go:308, :326, oturum_test.tr:274. Sıradaki: kod, metin, sözleşme ve testler birlikte.
- **S-493 Yapılacaklar (C, Ö3; sahip Dost Interface):** yarıda kalan yüklemenin işaretiyle açılışta yalnız olaylar sınanıyor, yüklemenin aynı cihazda yeniden denenmesi sınanmıyor [kanıtlı: tools/assistant/yuva.tr:482-491; tools/assistant/yuva_test.tr:221]. Sıradaki: işaretli açılışta cihaz davranışını gösteren modelsiz test.
- **S-496 Yapılacaklar (D, Ö3, tek tık; sahip Dost Interface):** `dil_değiştir` yerleştirme adımını (`uygula`) `TR_try` dışında çağırıyor, çağıran da hatayı yakalamıyor; adım atarsa yeni ses ve dil, eski takım kalır, oysa yorum atomik diyor [kanıtlı: tools/assistant/yuva.tr:389-397; tools/assistant/oturum.tr:148, :197] [hesap: yol bugün pratikte açık değil].
- **S-497 Yapılacaklar (D, Ö3, tek tığın sonraki dilimi; sahip Dost Interface):** ayarın port alanı okunuyor ama açılışta kullanılmıyor: ekranlı kip sunucuyu `--ekran`'ın kapısıyla açıyor [kanıtlı: tools/assistant/katalog.tr:447; tools/assistant/asistan.tr:437]. Sıradaki: tek tıkla açılışı yazan dilim karar verir.
- **S-518 Yapılacaklar (E, Ö3; sahip D1):** Dost'un nicemli etkinleştirme otomatiği yalnız q4_k ve q6_k ağırlığı sayıyor, motorun kabul ettiği q4'ü saymıyor [kanıtlı: tools/assistant/nicemli.tr:9, :91; internal/stdlib/ml/quant_activations.go:85]. Sıradaki: otomatiğin QAT modelinde q4'ü sayması, iletilerle (nicemli.tr:58-59, :136).
- **S-534 Mevcut durum (ekran):** `asistan.tr` ekranlı kipte ve `screen_demo.tr` sayfayı sunar, olayları yayar, komutları ana döngüde karara bağlar; olay kimliği metindir, sözleşme `tools/assistant/contract.md` [test: `TestScreenServerEndToEnd`]. Gerçek mikrofonla ve sesle doğrulama Ahmet'in.
- **S-535 Mevcut durum (deneme kaydı):** `--deneme_klasörü` her turu JSONL satırına, sözün WAV'ına ve özet tablosuna yazar, seçenek yoksa kayıt yok; klasör mutlak yol, git ağacının ve bulut klasörlerinin dışında [test: `tools/assistant/deneme_test.tr`].
- **S-539 Yapılacaklar (E, Ö3; sahip H1 (Dost'un ölçü aracı), R1'in ölçüsünden önce):** Dost'un kimlik dosyasındaki `anlar_ms` tam milisaniye, milisaniyeden küçük token farkı ayrılamıyor [kanıtlı: `tools/assistant/zincir.tr:25`, `:198`, `:329`]. Sıradaki: alt milisaniye çözünürlük ya da token sayacı ile toplam süre.
- **S-547 Mevcut durum (kurulum):** Dost'un kataloğu (`tools/assistant/katalog.json`), okuyucusu ve uygunluk hesabı (`katalog.tr`) ve `kur.tr` her dil modeline `fits`, `tight`, `too_large` ya da `too_long` verip öneri satırı yazıyor; hatalar H8160–H8174 [test: `tools/assistant/katalog_test.tr`].
- **S-548 Mevcut durum (bağlam sınırı):** her dil modeli kendi `context_max`'ını taşıyor; istenen bağlam bütün modellerinkinin üstündeyse H8174, birininkinin üstündeyse o model `too_long` olur ve öneriye girmez; katalogda dil modeli yoksa H8169 [kanıtlı: `tools/assistant/katalog.tr:304`; test: `tools/assistant/katalog_test.tr`].
- **S-549 Mevcut durum (tarayıcıyla eşleme):** `--tarayıcıyla_eşle` ile Dost ekranı kendisi açıyor, sayfa belirteci tek kullanımlık kodla `/pair`'den alıyor; yanlış kod sınırı 429, üstbilgisiz istek 400, günlükte kod yok (H8175, H8176); seçeneksiz Dost'ta belirteçsiz istek 401 [test: `tools/assistant/esleme_test.tr`]. Gerçek tarayıcıyla deneme H diliminin ön şartı.
- **S-550 Mevcut durum (ekranın görünümü):** ayarlar dişlinin açtığı panelde, Konuş düğmesi ses kartında, basılı çip onay işaretli, hata satırı panelde [test: `TestSettingsLiveBehindTheGear`, `TestTalkButtonIsInTheVoiceCard`, `TestPressedChipCarriesAMark`, `TestSettingsErrorLineIsInThePanel`].
- **S-551 Mevcut durum (model ekranı):** ayarlarda Yükle yok; Modeller penceresi her türü kendi grubunda, her modeli kendi indirme satırıyla gösteriyor [test: `TestSettingsHaveNoLoadButton`, `TestModelsDialogNamesEveryKind`, `TestDownloadRowShowsOnlyItsOwnJob`, `TestDownloadRowShowsSpeedAndSlow`, `TestModelsRowSaysInUseOnlyWithATeam`, `TestTeamBoxNamesTheTeam`].
- **S-552 Yapılacaklar (D, Ö3; sahip Dost Interface):** `TestIconButtonsHaveAName` ve `TestReducedMotionStopsAnimations` gevşek: anahtarı ya da seçiciyi sınamıyor [kanıtlı: `tools/assistant/behavior_screen_look_test.go:60`, `:84-91`]; Esc önceliği ve panelin kendiliğinden kapanışı için sayfanın betiğini koşan test yok.
- **S-553 Yapılacaklar (C, Ö2; sahip Dost Interface):** sayfanın eşleme davranışı (kodun adresten okunup silinmesi, önden işlenen sayfanın kodu tüketmemesi, belirtecin `sessionStorage`'da kalması) yalnız sayfanın metni okunarak sınanıyor [kanıtlı: `tools/assistant/behavior_screen_test.go:539-542`]. Sıradaki: sahte `location`, `history`, `sessionStorage` ve `fetch` ile sayfanın betiğini koşturan test düzeni.
- **S-555 Yapılacaklar (F, Ö3; sahip Dost Interface):** kataloğun sistem iletisi modelin güncel bilgisi olmadığını söylemiyor, Dost uydurulan güncel bilgiyi işaretsiz gösteriyor [kanıtlı: `tools/assistant/katalog.json:511`, `:518`]. Sıradaki: sistem iletisine güncel bilgi cümlesi ya da saat, tarih ve hava için araç.
- **S-559 Yapılacaklar (E, Ö3; sahip D1 (tasarım ve uygulama, örnekleme kitaplığı)):** zincirin açgözlü seçimi tekrar cezası taşımıyor, cevap aynı cümleyi yineleyebiliyor [kanıtlı: `tools/assistant/zincir.tr:169-174`]. Sıradaki: örnekleme kitaplığına tekrar cezası.
- **S-560 Yapılacaklar (F, Ö3; sahip TCB):** deneme kaydının başlığında motorun commit'i yok, denemenin hangi motorla alındığı sonradan bilinmiyor [kanıtlı: `tools/assistant/asistan.tr:481-482`, `tools/assistant/deneme.tr:55-62`]. Sıradaki: başlığa motorun commit'i.
- **S-577 Yapılacaklar (C, Ö2; sahip ekran dilimi):** her sekme kendi `/events` akışını açıyor (`tools/assistant/js/transport.js:27`); aynı köke altı akış varken `POST /command` asılı kalıyor, altıncı sekmede durdur çalışmaz [ölçüldü: DENETİM 295, Chromium, HTTP/1.1; hipotez: altıncı sekme]. Sıradaki: ekran diliminin tasarımı çareyi (akışı sekmeler arasında paylaşmak, HTTP/2) kaynaklı tartıyla seçer.
- **S-578 Yapılacaklar (C, Ö2; sahip ekran dilimi):** onay kartında argümanın gerçek kısmı boş satırlarla kaydırmalı kutunun görünmeyen yerine itilebilir, U+E0000 etiket karakterleri görünür kılınmıyor [kanıtlı: `tools/assistant/css/screen.css:186`, `tools/assistant/js/screen_state.js:43-45`].

### Araçlar ve testler

- **S-477 Yapılacaklar (C, Ö2, codegraph'ın sonraki dilimi; sahip D1):** G4 codegraph'ın kendini sınayan testlerini her koşuda koşuyor; bunlar yalnız `tools/codegraph` ya da `internal/devtools/codeindex` değişince G4'te, birleşmeden sonra K5'te koşmalı [ölçüldü: RT-2026-11-14-G4CPU-D1-3]. Sıradaki: taşınacak testleri D1 seçer.
- **S-494 Yapılacaklar (F, Ö3; sahip D1):** commit kipinin kaydı: kesilen koşuda geçici index ve yedeği git klasöründe kalıyor, `git add -A` yoksayılmayan büyük izlenmeyen dosyayı nesne deposuna yazıyor, gerçek index'i geri koyan kol testsiz [kanıtlı: tools/full_check.sh:921-933; internal/full_check_commit_test.go:447]; sonuç alanı adımın sayılı başlığını taşıyor (tools/full_check.sh:326, :751).
- **S-495 Yapılacaklar (sınıfsız: araç; sahip TCB):** `TestReviewedSectionsWriteIdentities` ters tırnaktaki her sözcüğü kod adı sayıyor; dosya yolu, komut seçeneği, ortam değişkeni ve dış biçimin anahtarı elle istisna listesine giriyor [kanıtlı: internal/devtools/testrunner/behavior_language_spec_test.go:249, :259]. Sıradaki: bunları ayıran kural ya da açık işaret.
- **S-501 Yapılacaklar (F, Ö3; geliştirici aracı; sahip dil çekirdeği):** `lib/*_test.tr` derlenmiş `trlang`'ın gömülü kopyasından farklıysa kitaplık sayılmıyor, ilgisiz bir ileti çıkıyor [kanıtlı: internal/lang/interp/library.go:225-234]. Sıradaki: dosya deponun `lib/`'indeyse ileti gömülü kopyanın farklı olduğunu ve `trlang`'ı yeniden derlemeyi söyler.
- **S-502 Yapılacaklar (F, Ö3; geliştirici aracı; sahip TCB):** commit kipi (`--kip=commit`) üreteçlerin -check'ini koşmuyor, yalnız ön ve tam denetim koşuyor [kanıtlı: tools/full_check.sh:462-467, :1390-1405]. Sıradaki: kod listesi ya da ileti dosyaları değişince kip onu koşar.
- **S-503 Yapılacaklar (F, Ö3; geliştirici aracı; sahip CODEGRAPH UZMANI):** commit kipi staticcheck'i koşmuyor, kullanılmayan işlevi yalnız G4 görüyor [kanıtlı: tools/full_check.sh:482-483, :1390-1405]; değişen pakette kipe sığıyor [ölçüldü: RT-2026-11-15-LAPTOP-CODEGRAPH-1]. Sıradaki: tasarımı sıradaki codegraph işlerinden biri.
- **S-504 Yapılacaklar (C, Ö2, ayrı dilim; geliştirici aracı; sahip H1):** trlang profile'ın örnekleyicisi `GOMAXPROCS=1`'de beklenenden çok az örnek alıyor, "Örnekleme" payı iş parçacığı sayısıyla değişiyor [ölçüldü: RT-2026-11-18-HIGHRAM-H1-3] [hipotez: örnekleyici çalışma sırası alamıyor]. Sıradaki: önce nedenin ölçüsü.
- **S-507 Yapılacaklar (E, Ö3; sahip CODEGRAPH UZMANI):** tests-for tools/assistant/index.html ve tests-for tools/assistant/js/models.js `ekran_test.tr`'yi vermiyor; test sayfanın betik listesini index.html'den okuyor [kanıtlı: tools/assistant/ekran.tr:126-135; tools/codegraph/gaps.md:116]. Sıradaki: gaps.md'nin maddesine bağlı; `tools/codegraph/tr_paths.go` 599/600, önce bölünür.
- **S-509 Yapılacaklar (D, Ö3; geliştirici aracı; sahip CODEGRAPH UZMANI):** depo taramaları denetiminin iki kolu testsiz: go run yolunun çıkış 1 ve durma satırıyla dönen kolu, bayat grafta karşılaştırmanın atlandığı söz [kanıtlı: tools/full_check.sh:885-888; tools/codegraph/fresh.go:166, :182]. Sıradaki: iki test kolu; `internal/full_check_commit_test.go` 596/600, önce bölünür.
- **S-510 Yapılacaklar (F, Ö3; geliştirici aracı; sahip CODEGRAPH UZMANI):** tazeliğe bakılamadığında da (`--no-rebuild`'de damga okunamazsa, `refresh` bakamazsa) sonraki söz "the graph is older than the tree" diyor, neden yanlış [kanıtlı: tools/codegraph/main_query.go:133-137; tools/codegraph/fresh.go:166; tools/codegraph/repo_scans.go:41]. Sıradaki: söz nedeni kapsasın.
- **S-511 Yapılacaklar (F, Ö3; geliştirici aracı; sahip CODEGRAPH UZMANI):** commit kipi süre dolunca kesilen paketin FAIL satırlarını düşüşten ayırmadan basıyor, "G4'e kaldı" satırı paketin adını vermiyor [kanıtlı: tools/full_check.sh:669-681, :755-759]; `TestDocTRCodeBlocks` da kesilen denetimi düşüş sayıyor (cmd/trlang/e2e/behavior_doc_code_blocks_test.go:67-68). Sıradaki: kesilen FAIL ayrı yazılır.
- **S-512 Yapılacaklar (F, Ö3, D1 Teslim B'den sonra; sahip D1):** `TR_Message(` argümanında tipin metni (`"\(TR_Type(x))"`) geçmesini yakalayan denetim yok; iletiye çevrilemeyen kanonik ad girer [kanıtlı: internal/lang/messages/README.md:81]. `lib/*.tr`'de bugün örneği yok.
- **S-514 Mevcut durum (Go araç zinciri):** `go.mod` dil sürümü `go 1.27.0`, araç zinciri go1.27.4; sözcük çözümleyicinin Unicode tabloları en az 17 [kanıtlı: go.mod:3, :5; x/text v0.39.0 unicode/norm/tables17.0.0.go:3].
- **S-694 Yapılacaklar (sınıfsız: araç, Ö3; sahip Motor):** HTTP istemcisi geldi (internal/stdlib/download/client.go:86), Windows'ta sertifika deposunun seçimi (`x509sslcertoverrideplatform` varsayılanı) karara bağlanmadı [hipotez].
- **S-520 Yapılacaklar (sınıfsız: test boşluğu, Ö3; sahip T1):** makine bayrağını değiştiren testlerin yol tabloları canlı bayrağı okursa çekirdeği sınamadan geçebilir; `q8Paths` düzeltildi [kanıtlı: internal/compute/tensor/cpu_quant_q8_test.go:13, :24]. Sıradaki: `internal/compute` altında başka tabloları aramak (aday: internal/compute/tensor/cpu_gemv_test.go:12) [hipotez].
- **S-521 Yapılacaklar (C, Ö2; sahip CODEGRAPH UZMANI):** `TestBuildScalesLinearly` CI'da codeindex değişmeyen birleşmelerde düşüyor, yeniden koşuda geçiyor: bir aşamanın oranı `linearLimit`'i aşıyor [ölçüldü: CI 45362496905 deneme 1; kanıtlı: `internal/testkit/testkit.go:246`]. Sıradaki: `6ba2be3e`'ten sonraki CI düşüşleri aşama aşama sayılır (temiz dosyaların kurulumu ayrı).
- **S-524 Yapılacaklar (F, Ö3; sahip D1):** `stripDoubleQuoted` ters bölüden sonraki karakteri tırnak dışında da atlıyor, ardından gelen ters tırnak sayılmıyor [kanıtlı: `internal/lang/messages/template_test.go:386-388`]; `internal/lang/messages/README.md` bunu yazmıyor. Sıradaki: test düzelir ya da kural yazılır.
- **S-525 Yapılacaklar (F, Ö3; araç; sahip D1):** `reporoot.Glob` izlenmeyen dosyayı uyarısız süzüyor, `git add` edilmemiş yeni fikstür testte görünmüyor [kanıtlı: `internal/reporoot/files.go:84-101`]. Sıradaki: süzülen eşleşmeyi bildirmek.
- **S-530 Yapılacaklar (F, Ö3; sahip D1):** trlang --help çıkış kodu 2 ile hata veriyor, ileti trlang help öneriyor [ölçüldü: `02d9fb69`]. Sıradaki: yardım gibi davranır.
- **S-582 Yapılacaklar (F, Ö3; sahip D1, sonraki dilim):** ileti kodunun binler basamağının alanı gösterdiğini (`internal/lang/messages/README.md:16`) sınayan test yok; H7030 GPU, H7042 tensör iletisi kendi alanının dışında [kanıtlı: `internal/lang/messages/en.json:1531`, `:1534`].
- **S-583 Yapılacaklar (F, Ö3; sahip D1):** `trrun.Parse` yalnız sözdizimi aşamasını denetliyor, `StageType`'ı görmüyor: H0004'lü metin Go testinde çalışır [kanıtlı: `internal/testkit/trrun/trrun.go:28`]. Yalnız test takımı.
- **S-587 Yapılacaklar (F, Ö3; sahip dil çekirdeği kolu, komut satırı):** trlang version yalnız sürüm numarasını basıyor (H8603), ikilinin derlendiği commit'i ve ağacın değişik olup olmadığını göstermiyor [kanıtlı: `cmd/trlang/main.go:130`]. Sıradaki: ileti metni tasarımla.
- **S-588 Yapılacaklar (F, Ö3; sahip TCB):** `docs/agent_reference.md` tensör işlemlerini ve `TR_fuse`'un kurallarını özetlemiyor: `TR_fuse`, `TR_Tensor`, `TR_conv1d` hiç geçmiyor [ölçüldü: `grep -c`, `02d9fb69`]. Sıradaki: Ahmet'le konuşulduktan sonra TCB yazar.
- **S-589 Yapılacaklar (test, Ö3; korunan kural A; sahip Motor (dil çekirdeği), sonraki dokunuşta, yargı ister):** denetleyicinin üç kuralı testsiz: `internal/lang/checker/runtime_checks.go:128` (M24), `local_types.go:163` (M22) ve `member_types.go:77` (M28) mutantlarıyla bütün testler geçiyor [ölçüldü: DENETİM 961, `27d242d5`]. Sıradaki: üç programın beklenen sonucu dil tanımından yazılır.
- **S-590 Yapılacaklar (test, Ö3; sahip DIL UZMANI, testrunner):** `TestEveryRuleHasATest` ve `TestRuleTestsExist` testin varlığını sınıyor, kuralı yakaladığını değil: 20.10'un tek TR testinin gövdesi (`tests/kontrol_noktasi_test.tr:302`) boşaltılınca ikisi de geçiyor [ölçüldü: `8279479c`]. Sıradaki: denetimsiz `TR_test` gövdesinin tanımı (yardımcılar ve `TR_grad_check` sayılır mı).
- **S-591 Yapılacaklar (test, Ö3; sahip TCB, yol haritası testi, sonraki dokunuşta):** `TestRoadmapShallowAndFullClones` tam klonda yalnız ata olan commit'i kuruyor (`internal/roadmap_test.go:104-160`); `checkRoadmapAncestor`'ın ata değil hatası (`:97-99`) hiçbir testte koşmuyor [kanıtlı: `internal/roadmap_test.go:97`]. Sıradaki: yardımcı sonucunu döndürsün, yan dalda kurulan ata olmayan commit'le hata beklensin.
- **S-592 Yapılacaklar (F, Ö3; sahipler dosyaların kolu, sonraki dokunuşta):** `cmd/trlang/e2e/testdata/doc_blocks_known.txt` yalnız belge ve satır tutuyor, bloktaki yeni hata sessiz kalır; `TestOptimizerStateResume`'un beklenen değeri kendi kesintisiz koşusu [kanıtlı: `cmd/trlang/e2e/testdata/doc_blocks_known.txt:3-4`, `internal/compute/tensor/optimizer_test.go:40`]. Sıradaki: doc_blocks_known.txt'ye beklenen kod (biçim kararı).
- **S-593 Yapılacaklar (E, Ö3; sahip H1, ölçü betikleri):** deneme kipinde (`OLCU_DENEME=1`) en-hizli-ayar.sh tek grupla da bu makinenin komut sınırına sığmıyor: koşturucu her koşudan önce yükün inmesini bekliyor (tools/bench/baseline/run.py:255), uyuşma ve deneme tek komutta (`tools/bench/baseline/machines/laptop/lib.sh:100`) [ölçüldü: H1, `06615910`]. Sıradaki: betik denemede koşuyu ayrı komut olarak basar.
- **S-702 Yapılacaklar (E, Ö3; sahip T1):** S1'in uyuşma ölçüsü GPU'da nicemli ile nicemsiz yolun logit farkını (ε) q4, q4_k ve q6_k ağırlıkta yazmıyor: öbür yol yalnız işlemcide koşuyor [kanıtlı: tools/bench/baseline/language_model.tr:234, `:240`]; q8 ve float ağırlıkta nicemli yol yok, GPU'da da 0 yazılıyor (`:238-239`). Sıradaki: GPU'da ε yazılsın mı kararı, tasarım satırıyla.
- **S-594 Yapılacaklar (E, Ö3; sahip H1, ölçü betikleri):** High-RAM'in uyuşma geçişi seçilen kaleme bakmadan her istekte İngilizce piper denetimini koşuyor (tools/bench/baseline/run.py:1009-1016) [kanıtlı: `9b983482`]. Denemede `run_pair` koşmuyor (`:754-756`): H1 karar verir. Sıradaki: piper denetimi yalnız istenen kalemde.

### Denetleyici ve iletiler

- **S-476 Yapılacaklar (F, Ö3; iletiler; sahip DIL UZMANI):** H8695'in düzeltme komutu yer tutucuyla (`trlang format <dosya|klasör>`), kopyalanınca çalışmıyor; tek dosyada somut yol verilebilir [kanıtlı: internal/lang/messages/tr.json:2263; cmd/trlang/format.go:72]. H2495 anahtarı çevirmeyi önermiyor, H2646 öneriyor [kanıtlı: internal/lang/messages/tr.json:395, :488].
- **S-499 Yapılacaklar (D, Ö3; sahip dil çekirdeği kolu):** `TR_func f(t: TR_Tensor) -> TR_Float { TR_return TR_Float(TR_sum(t)) }` çağrısı H2423 yerine "a released tensor was used" paniğiyle süreci düşürüyor: iletinin argüman özeti bırakılmış tensörü yazıyor [kanıtlı: internal/lang/interp/arg_errors.go:189; internal/compute/tensor/lifetime.go:110]. Beklenen: H2423, süreç düşmez.
- **S-515 Yapılacaklar (F, Ö3; sahip dil çekirdeği):** Go'nun hata metnini kullanıcıya taşıyan iletiler: `tr.json`'da `{error}` yuvalı 123 ileti var (ör. H7001) [kanıtlı: internal/lang/messages/tr.json:1516]. Sıradaki: madde madde karar sonraki ileti turunda.
- **S-526 Yapılacaklar (E, Ö3; sahip D1):** dil tanımının 36 onaylı yeni adı motorda yok, yazınca sözdizimi hatası geliyor [kanıtlı: `internal/devtools/testrunner/behavior_language_spec_test.go:388-399`]; ajan bunları var sanıyor [ölçüldü: `docs/agent_trials.md`]. Sıradaki: ajanın göreceği yerde henüz yok yazılsın, maddenin *Test:* satırı doğrulansın.
- **S-527 Yapılacaklar (F1, Ö3; sahip D1):** listede `l.find(...)` çalışırken yanlış ipucu (`TR_index_of`) alıyor, `l.index(2)` ipucu almıyor [ölçüldü: trlang run, `02d9fb69`]. Sıradaki: öneri tablosu.
- **S-528 Yapılacaklar (D, Ö3; sahip D1):** modüle özel ad (`_kur()`, `m._kur()`) trlang check'ten geçiyor, çalışırken hata veriyor (11.7) [ölçüldü: trlang check ve trlang run, `02d9fb69`]. Sıradaki: denetim hatası, 4a5'e bakılır.
- **S-529 Yapılacaklar (F1, Ö3; sahip D1):** Türkçe adlar ipucu almıyor, biri yanlış alıyor: yok için ileti `TR_Path` öneriyor, doğrusu `TR_none` [ölçüldü: trlang check, `02d9fb69`]. Sıradaki: öneri her dilin görünüm adına bakar.
- **S-531 Yapılacaklar (D, Ö3; sahip D1):** `T?` aritmetikte denetlenmiyor: `l.TR_index_of(9) + 1` denetimden geçiyor, çalışınca düşüyor [ölçüldü: trlang check ve trlang run, `02d9fb69`]. Sıradaki: 4a5'in kapsamına bakılır.
- **S-532 Yapılacaklar (D, Ö3; sahip D1):** `TR_loss`'un parametreleri tipsiz [kanıtlı: `lib/layers.tr:680`]; tipli parametre de metot argümanında denetlenmiyor: `TR_Tensor` bekleyen metoda liste geçiyor [ölçüldü: trlang check, `02d9fb69`]. Sıradaki: 4a5'in A satırı.
- **S-533 Yapılacaklar (F, Ö3; sahip D1):** modeli kaydetmek için var olmayan bir ad yazılınca ileti ters işi, `TR_load_parameters`'ı öneriyor; doğrusu `TR_save_tensors(yol, TR_parameters(m, TR_named: TR_true))` [ölçüldü: trlang check, `02d9fb69`].
- **S-575 Yapılacaklar (F1, Ö3; sahip D1, denetleyici ve iletiler):** kullanıcının adı `TR_Range`'in Türkçe görünüm adına benzeyen tipine H1006 `TR_Range` öneriyor; önerisi kopyalanırsa ajan kendi yapısına dilin tipini verir [ölçüldü: trlang check, `02d9fb69`].
- **S-584 Yapılacaklar (F, Ö3; sahip TCB, toplu karar):** adlı argümanların iç kodu iletiye sızıyor: `TR_clamp(0.5, 0.0, 1.0)` denetimden geçiyor, koşuda "sınırla(x, TR_arg_min: ..., TR_arg_max: ...) 1 konumlu parametre bekler, 3 verildi" veriyor [ölçüldü: trlang check ve run, `02d9fb69`].
- **S-695 Yapılacaklar (D, Ö3; sahip dil çekirdeği kolu, denetleyici):** `TR_clamp(0.5, 0.0, 1.0)` fazla konumlu argümanla denetimden geçiyor, hata ancak çalışırken geliyor [ölçüldü: trlang check ve trlang run, `02d9fb69`].

### Temizlik

- **S-478 Yapılacaklar (sınıfsız: temizlik, Ö3; sahip T1):** `RawBackend.FillRaw` test dışında çağrılmıyor [kanıtlı: internal/compute/tensor/typed.go:29, internal/compute/tensor/cpu_typed.go:146, internal/compute/gpu/typed.go:66]. Sıradaki: sahip okuyup karar verir.
- **S-479 Yapılacaklar (sınıfsız: temizlik, Ö3; sahip D1):** Sampler.State çağrılmıyor; durum `Fill`'de o.state() ile yazılıyor [kanıtlı: internal/compute/dataload/sampler.go:337, :329]. Sıradaki: sahip okuyup karar verir.
- **S-500 Yapılacaklar (F, Ö3, D1 Teslim B'den sonra; kod kalitesi; sahip kitaplık):** `lib/layers.tr` 788 satır, boy sınırı 600; istisna tavanı da 788 [kanıtlı: internal/layoutrules/layoutrules.go:18; internal/layoutrules/tr.go:23]. Sıradaki: dosya bölünür.
- **S-505 Yapılacaklar (F, Ö3; sahip T1):** iki Go yorumu var olmayan dosya adlarını anıyor: `sayac_saat_amd64.go` ve `sayac_saat_amd64.s` (bugün `counter_clock_amd64.go`, `counter_clock_amd64.s`) [kanıtlı: internal/compute/tensor/counter_clock.go:6, internal/compute/tensor/counter_clock_amd64.go:5]. Sıradaki: T1'in tensör çekirdeğindeki sonraki işi.
- **S-585 Yapılacaklar (F, Ö3; sahip DIL UZMANI):** kitaplık yorumlarının ve örneklerin kullanım satırları görünüm adlarıyla yazılı (ör. `lib/training.tr:5-7`): lib'de 8 dosyada 28, examples'ta 24 dosyada 29 satır [ölçüldü: `git grep -P -c`, üç kalıp, `02d9fb69`]; `docs/recipes.md:22` örneği `tr.json`'a koyuyor, dil dosyası onu reddeder [kanıtlı: internal/lang/codes/json_parse.go:139]. Sıradaki: satırlar kimliğe geçer.
- **S-586 Yapılacaklar (F1, Ö3; sahip dil çekirdeği kolu (Motor)):** `examples/tr/hatalar/`'ın 23 örneğinden 22'sinin "Düzeltilmiş hâli" görünüm adıyla, altısında (06, 08, 10, 11, 20, 21) yorum `//` ile yazılı; kopyalanan düzeltme koşmuyor [ölçüldü: `02d9fb69`; 01 ve 06 denetimi, `9956e6fa`]. Sıradaki: hepsi kimlikle ve `#` ile yazılır, düzeltilmiş kodu denetleyen test eklenir.

### Diğer

- **S-579 Yapılacaklar (C, Ö2; sahip T1):** `CopyTypeError`'ın GPU kolunu (`internal/compute/tensor/cpu_typed.go:115`) hiçbir test çağırmıyor; `TestCopyTypeErrorIsTheErrorOfTheCopy` yalnız CPU'da ve kopya da `CopyTypeError`'ı çağırdığı için kendini sınıyor [kanıtlı: `internal/compute/tensor/cpu_typed.go:86`]. Sıradaki: `TestTypedPathsOnGPU` kalıbında kalıcı test.
- **S-580 Yapılacaklar (C, Ö2; sahip T1):** erken bırakılan `TR_read_lines` okuyucusunu yalnız çöp toplayıcı kapatıyor (internal/stdlib/files/lines.go:228), `trlang test` okuyucuları yalnız sonda kapatıyor (`cmd/trlang/test_command.go:20`): bir testin okuyucusu sonraki testin `TR_remove`'unu H7125 ile düşürüyor [ölçüldü: `GOGC=off` ile iki testli program, `02d9fb69`]. Sıradaki: koşucu her testten sonra okuyucuları kapatsın.
- **S-581 Yapılacaklar (D, Ö3; sahip T1):** safetensors yolunda `max_position_embeddings` 0 denetlenmiyor, uzaktaki çarpımda H6126 ile düşüyor; H7155'in denetimi yalnız GGUF'ta [kanıtlı: `lib/pretrained.tr:706`, `lib/pretrained.tr:370`; ölçüldü: T1].
- **S-683 Yapılacaklar (E, Ö3; sahip H1, ölçü betikleri):** dizüstü betikleri koşu klasörünü yalnız betiğin adıyla kuruyor: aynı pencerede iki commit'i ölçen işte ikisi aynı klasöre yazar, `metrics.json` öncekini ezer [kanıtlı: `tools/bench/baseline/machines/laptop/lib.sh:84`]. Sıradaki: koşu klasörü commit'i de taşır.
- **S-684 Yapılacaklar (E, Ö3; sahip H1, ölçü betikleri):** dizüstünde uyuşma ısınmanın içinde koşuyor (tools/bench/baseline/run.py:1078-1079) ve sonucu yalnız log.jsonl'e gidiyor; `agreement.json`'u yalnız ayrı uyuşma geçişi dolduruyor (`:1039`) [kanıtlı: `02d9fb69`]. Sıradaki: dizüstünün uyuşması da `agreement.json`'a yazılır.

### Windows'ta Go çalışma zamanının bozulması

- **S-595 Yapılacaklar (A, Ö1; sahip T1, tensör kolu):** Windows'ta kurtarılan donanım istisnası (ör. `internal/compute/tensor/parallel.go:145`) AMX'li Intel konağında Go'nun yığınının altına yazıp öbeği sessizce bozabilir (golang/go#81238) [hipotez: rapor ve T1; ölçüldü: CI koşusu 43251746027]; düzeltme Go1.27.5'te, araç zinciri hâlâ `go1.27.4` [kanıtlı: `go.mod:5`]. Sıradaki: Go1.27.5 çıkınca araç zinciri satırı yükseltilir.

### v0.2.0 ikilisinin kaynağı

- **S-596 Yapılacaklar (B, Ö1; sahip TCB):** yayımlanan v0.2.0 ikilisinin (`AGENTS.md:18`) hangi commit'ten ve hangi Go sürümüyle derlendiği bilinmiyor; etiketin commit'i (`283e249e`) `net/http`'yi bağlamıyor, ikili ondansa GO-2026-6859 ona değmez [ölçüldü: `go list -deps ./cmd/trlang`, `283e249e`]. Sıradaki: ikilinin go version -m kaydı (`vcs.revision`).

### Gerçek mikrofonda sözün başı

- **S-597 Yapılacaklar (A, Ö1, canlı doğrulama; sahip S1):** gerçek mikrofonda sözün başı hata vermeden kayboluyordu (`tools/assistant`); kod `6588de1a` ile düzeldi, Whisper hatalarının bu kayıptan geldiği canlı denemeyle doğrulanmadı [ölçüldü: AS-2026-11-08-LAPTOP-S1-1; hipotez: sebep]. Sıradaki: Ahmet'in sesiyle canlı deneme.

### Yazıyı konuşmaya çevirme (dil tanımı 27 yazılınca kendi bölümüne taşınır)

- **S-599 Mevcut durum (S1):** Piper sesinde fonem çevirici yalnız Türkçede (`tr/phonemes`); öbür dillerin sesi H7559 veriyor [test: TestPiperVoiceWithoutPhonemizerSaysH7559].
- **S-600 Yapılacaklar (D, Ö3; sahip S1):** kaliteli ses için Chatterbox Multilingual dizüstü CPU'sunda gerçek zamandan yavaş, zaman dil modeli (T3) ile dalga çözücüsü (S3Gen) arasında yarı yarıya [ölçüldü: TTS-2026-11-08-LAPTOP-TCB-1]. Sıradaki: S3Gen'in profili; T3 için TR'nin GGUF yolu (`BricksDisplay/Chatterbox-Multilingual-TTS-GGUF`) denenir [hipotez].
- **S-601 Yapılacaklar (A, Ö1, dil tanımı 27'nin dilimi; sahip S1):** `TR_synthesize` MMS sesinde sözlükte olmayan karakteri hatasız atıyor: rakam ve noktalama da düşüyor, programa haber verilmiyor [kanıtlı: `lib/vits.tr:498`; ölçüldü: S1]. Sıradaki: çare dil tanımı 27'de seçilir (hata, uyarı ya da atılanları veren sonuç).
- **S-602 Yapılacaklar (E, Ö3; hız; sahip S1):** dizüstünde Piper Türkçe (dfki) sesinde TR, en hızlı ayarlı onnxruntime'dan yavaş [ölçüldü: TTS-2026-11-22-LAPTOP-H1-6]. Sıradaki: kalan hız işi yol haritasında (TCB); `TR_where`'in `TR_fuse`'a girmemesi 15.44'te.
- **S-603 Yapılacaklar (F, Ö3; sahip S1, Piper dilim B):** Piper sesinde eksik ağırlığı H7553 Hugging Face adıyla söylüyor, kullanıcı o adı dosyada bulamaz [kanıtlı: `lib/vits.tr:37`]. Sıradaki: Piper adının tersi de tutulsun, ileti dosyadaki adı söylesin.
- **S-604 Yapılacaklar (E, Ö3; sahip S1, Piper dilim B ya da C):** ONNX okuyucu düğümlerden yalnız Constant'ı okuyor: torch'un dışa vericisinin Identity düğümüyle verdiği eş ağırlık adı H7553 verir [kanıtlı: `internal/compute/onnx/onnx.go:1-5`; ölçüldü: S1, küçük Piper fikstürü]. Sıradaki: girdisi bilinen tensör olan Identity'nin çıkışı takma ad olarak da verilsin.
- **S-605 Yapılacaklar (F, Ö3; sahip S1, ileti metni tasarımla):** H7555 şeklin config.json'a göre olması gerektiğini söylüyor, Piper'da şekil ağırlıklardan gelir [kanıtlı: `internal/lang/messages/tr.json:1863`]. Sıradaki: "beklenen".

### Denetleyicinin üye erişimi

- **S-606 Yapılacaklar (D, Ö3; sahip dil çekirdeği):** `TR_var x = 1` ve `x.a` denetimden geçiyor, koşu H2467 veriyor; tipi bilinen değerde olmayan üye denetimde yakalanmalı [ölçüldü: trlang check ve run, `02d9fb69`].

### Ayrıştırıcının uzun zinciri

- **S-607 Yapılacaklar (C, Ö3; sahip dil çekirdeği):** ayrıştırıcı uzun üye ya da indeks zincirinde karesel: `parseInfixChain` her adımda zincirin başından o ana kadarki metni `spanToken` ile yeniden kopyalıyor [kanıtlı: `internal/lang/parser/expressions.go:307`, `:312`; `internal/lang/lexer/interpolation.go:119`]. Sıradaki: `spanToken` konumu tutsun; büyüme testi `testkit.ScalesLinearly` ile.

### Komut adı: `trlang`

- **S-608 Yapılacaklar (F1, Ö3):** yalın `trlang` yerine eski ad hâlâ duruyor (ör. `docs/architecture.md:208`, `tools/codegraph/README.md:648`, `:669`); `TestNoOldCommandName`'in altıncı biçimi sözcük listesiyle dar [kanıtlı: `internal/command_name_test.go:29`]. Sıradaki: ters tırnaklı her yalın eski ad hata olsun; istisna dil kodu kalıbı ve izin listesi.

### Denetim katmanlarının gözden geçirmesi

- **S-609 Mevcut durum:** hangi denetim katmanının (commit öncesi, ön denetim, tam denetim, CI, zorlama) neyi yakaladığı yazılı değil; `docs/development.md:50-53` yalnız kipleri sayıyor [kanıtlı: `docs/development.md:50`].
- **S-611 Yapılacaklar (C, Ö2):** `internal/testdata/typed_code_known.txt` satır numarasıyla tutuluyor: satır ekleyen düzenleme listeyi düşürüyor, -update-typed taşınan satırı eklemiyor, yalnız siliyor [kanıtlı: `internal/typed_code_test.go:38`, `:314`]. Sıradaki: anahtar dosya ve ad olsun ya da güncelleme taşınanı da yazsın.
- **S-613 Yapılacaklar (C, Ö2):** ön denetimin `selected_tests`'i `codegraph changed --go-tests` çıktısını here-string ile okuyor (`tools/full_check.sh:1171`); Windows'un Git Bash'inde 65 536 ile 65 700 bayt arası metin `<<<` ile okununca kabuk asılı kalıyor [ölçüldü: `read -r x <<<`, TCB]. Sıradaki: geçici dosyadan okumak.
- **S-614 Yapılacaklar (C, Ö2; sahip D1):** Windows'ta koşan commit kipi de planı here-string ile okuyor (`tools/full_check.sh:742`): S-613'ün asılmasına açık olabilir [hipotez: ölçülmedi].
- **S-615 Yapılacaklar (C, Ö3):** dizin önbelleği atlanan dosyanın nedenini ileti kodu ve parametreleriyle tutuyor; kodsuz hatanın metni (`internal/devtools/codeindex/skipped.go:53`) önbelleğe yazılmıyor, önbellekten boş nedenle döner [kanıtlı: `internal/devtools/codeindex/model.go:227`]. Sıradaki: kodsuz hatada metni de saklamak ya da kodsuz hatayı testle yasaklamak.

### Güvenilirlik (C): yol haritasından

- **S-618 Yapılacaklar (C, Ö2, Kol B; ölçülmedi):** küçük işaretçili yığınla dönen döngüde çöp toplama turunun tik arasına alınması ölçülmedi; 100 Hz döngü Jarvis'in ön koşulu [hipotez; `docs/roadmap.md:109`].
- **S-619 Yapılacaklar (C, Ö2):** `TestOtherTasksRunDuringLongTensorWork` (`internal/stdlib/ml/behavior_lock_release_test.go:185`) G4'te iki kez "gözcü yük sırasında hiç uyanmadı" ile düştü [ölçüldü: `99f1348f`, `9f06f747`]; GPU'da eşitleme kilidi bırakmıyor olabilir [hipotez: T1]. Sıradaki: bir daha düşerse iletideki uyanışlar okunur.
- **S-685 Yapılacaklar (C, Ö2):** `TestProfileModuleLines` (`internal/devtools/profiler/profile_test.go:378`) Windows CI'ında bir kez kaldı, kısa programda tek örnek çağrı satırına düştü [kanıtlı: CI 42507239962, `96026865`].
- **S-686 Yapılacaklar (C, Ö2; sahip H1, profilci):** `TestProfileBackwardTimeGoesToForwardLine` forward nil olunca da `t.Skip`'e gidiyor: satır atfı bozulsa da test düşmez [kanıtlı: `internal/devtools/profiler/profile_test.go:246-248`]; CI'da bir kez geri payı beklenenin altında kaldı (CI 42537298828). Sıradaki: örnek yeterliyken forward nil ise test düşsün.
- **S-687 Yapılacaklar (C, Ö2; sahip T1, tensör ve GPU kolu):** `TestUnavailableGPUStatusCarriesNoSystemText` `-count=2` ile ikinci turda düşüyor: sahte gpu:99 arka ucu süreç genelinde kaydedilip geri alınmıyor [kanıtlı: `internal/stdlib/ml/custom_grad_test.go:271`, `:343`; ölçüldü: zorlama işi 45185143141]. Sıradaki: kayıt `t.Cleanup`'la geri alınsın.
- **S-688 Yapılacaklar (C, Ö2; sahip CODEGRAPH UZMANI, kipin sahibi):** `TestCommitModeCutsAtTheLimitByPackage` yükte düşüyor, tek başına da düştü: fikstürün sabit duvar saati bütçesi (`internal/full_check_commit_test.go:307`) yüke bağlı [ölçüldü: `c41a22f7`, `aa352b5d`]. Aynı dosyadaki `TestCommitModeTrustsTheTimeoutOverTheClock` da yükte bir kez düştü, tek başına geçti [ölçüldü: T1]; bütçesi de sabit (`:346`). Sıradaki: süre yükten bağımsızlaşsın; dosya 596 satır, önce bölünür.
- **S-689 Yapılacaklar (C, Ö2; sahip CODEGRAPH UZMANI, kipin sahibi):** `TestCommitModeStopsTheLayoutWhenCut`'in `yakın koşarken` alt testi yükte "arka plandaki sahte go durdurulmalıydı" ile düşüyor [ölçüldü: `dd775178`, `9f9d0617`]; sabit bekleme (`internal/full_check_commit_layout_test.go:121`) yetmeyebilir [hipotez: T1]. Sıradaki: süre yükten bağımsızlaşsın.
- **S-690 Yapılacaklar (C, Ö2; sahip veri kolu):** `TestSparseIDMemory` zorlama işinde bir kez düştü (`internal/compute/tokenize/id_table_test.go:29`), tohum 1794830453034927885 [ölçüldü: zorlama işi 43862343352, `be1aea19`]; sonraki hafta düşmedi. Sıradaki: tohumla yerelde yeniden üretmek.
- **S-691 Yapılacaklar (C, Ö2; sahip T1, tensör ve GPU kolu):** `TestPeakLiveBySite` zorlama işinde GPU'suz Linux'ta iki turda da düştü (`internal/compute/gpu/profile_test.go:155`), tohum 1794225754371765857 [ölçüldü: zorlama işi 42613624406, `3a58a94d`]; sonraki iki hafta düşmedi. Sıradaki: tohumla yerelde yeniden üretmek.
- **S-620 Yapılacaklar (E, Ö3):** hata ayıklayıcıda (F5) `TR_input()` çalışmıyor, H8102 veriyor [kanıtlı: internal/devtools/debugger/dap.go:287].
- **S-621 Yapılacaklar (C, Ö2; ölçülmedi):** eğitimde adım süresi artıyor, sızıntı olabilir; sızıntıysa uzun eğitim kesilir [hipotez; `docs/roadmap.md:243`]. Sıradaki: önce ölçülür, eğitim hızı adaylarıyla birlikte.

## 20. Kontrol noktası

Kurallar: [`docs/spec/20_kontrol_noktasi.md`](spec/20_kontrol_noktasi.md).

### 20.1

### 20.2

- **S-624 Yapılacaklar (E, Ö2; sahip T1):** `TR_Checkpoint` `TR_Int`, `TR_Bool` ve float64 tensörü kaydedemiyor: `TR_save` H2629 veriyor, geçici klasör ve açık dosya kalıyor, Windows'ta ikinci kayıt H2142 [kanıtlı: internal/stdlib/checkpoint/write.go:179-183]. Sıradaki: `WriteRaw`'ı (internal/stdlib/ml/save.go:279) bağlamak.

### 20.11

- **S-625 Yapılacaklar (A, Ö1, Kol B, 15'in A'sıyla):** `TR_AnyTensor` tipi yok; `TR_load_parameters` hassasiyeti farklı tensörü çevirmeden H6960 ile, nicemlenmiş yaprağa yazmayı H6687 ile reddediyor (20.11, 15.54); H6687 yalnız açmayı öneriyor, hedefi aynı hassasiyette kurmayı önermiyor [kanıtlı: internal/compute/tensor/cpu_typed.go:105-122; test: `tests/kontrol_noktasi_test.tr:341-342`].
- **S-626 Yapılacaklar (A, Ö1, Kol B, S-315 ile):** `TR_load_parameters` hiç türev izlememiş yaprağı (tablo, tampon) toplamıyor (internal/stdlib/ml/params.go:90, internal/compute/tensor/grad_flag.go:53): sözlükte yoksa sessizce eski değerinde kalıyor, sözlükte verilirse H6531; 20.11 bütün yaprakları ister [ölçüldü: ae1e8dcc, trlang run]. Sıradaki: S-315'le; RoPE tablosu taşımayan hazır `.safetensors` ve LoRA alanlarının (15.34) yüklemesi aynı kararda.

### 20.12, 20.13

- **S-628 Mevcut durum:** `op.TR_state`'in "step"i tam sayı değil, iki öğeli ondalık tensör (20.12) [kanıtlı: internal/compute/tensor/optimizer.go:343-344].

### 20.17

- **S-629 Yapılacaklar (C, Ö2, 15.45 ile):** `TR_dropout` (15.23d) yok; gelince kontrol noktasının kaydettiği paylaşılan üreteçten çekmeli (20.17) [kanıtlı: `git grep -i dropout -- internal/lang/codes/definitions.json` boş].
- **S-630 Yapılacaklar (E, Ö3):** kontrol noktası 20.5'in `"rng_device"` alanını yazmıyor, yalnız CPU üretecinin "rng"'sini [kanıtlı: `git grep rng_device -- internal` boş; test: `TestStateRequiresSharedRandom`]. Sıradaki: 15.45'in sayaçlı üreteciyle birlikte.

## 21. Dış programlar

Kurallar: [`docs/spec/21_dis_programlar.md`](spec/21_dis_programlar.md).

### 21.1–21.8

- **S-631 Mevcut durum:** canlı mikrofonun `ffmpeg` dshow yolu (tools/assistant/asistan.tr:332) otomatik testte koşmuyor; `tools/assistant/ekran_test.tr:367` yalnız aygıt listesinin ayrıştırmasını sınıyor [kanıtlı: `git grep -l dshow`].
- **S-632 Yapılacaklar (F1, Ö3; sahip süreç kolu (Motor), 5.7'den sonra):** H7494 ve H7495'in önerisi argüman listesini `[]` yazıyor, H7498'inki adlı argümanları yazmıyor; kopyalanan düzeltme argümanı kaybeder [kanıtlı: internal/lang/messages/tr.json:1820, :1821, :1824]. Sıradaki: argümanlar okunup iletiye konur (21.3'ün sırası değişir).
- **S-633 Mevcut durum (sınır):** boş `PATH`'in iletisi H7564 argüman listesini yazar; maske yalnız bilinen gizli değeri kapatır, argümandaki adsız belirteç iletide görünür [kanıtlı: internal/lang/messages/tr.json:1872].
- **S-634 Yapılacaklar (C, Ö2; sahip S1, 21.2 dilimi):** durdurma isteği yalnız `runProgram`'da yakalanıyor, `trlang test`'te ve kabukta yakalanmıyor [kanıtlı: cmd/trlang/prepare.go:132] [hipotez: Unix'te süreç grubundaki dış program TR ölünce kalır].
- **S-635 Yapılacaklar (C, Ö2; sahip S1):** `TR_run_process`'in çıktı sınırı belleğin yarısı, tampon `bytes.Buffer` ikiye katlanarak büyüyor; büyürken eski ve yeni tampon birlikte durur [kanıtlı: internal/stdlib/process/run_process.go:114, :121-122, :130]. Sıradaki: sabit boylu parça listesi ve tepe ayırmayı ölçen test.
- **S-636 Yapılacaklar (D, Ö3; sahip D1):** denetleyici tipsiz ada bağlanan yolu (`TR_var y = TR_Path("a")`) argüman listesinde görmüyor, H7490 çalışma anında geliyor [kanıtlı: internal/lang/checker/process_args.go:9-11].
- **S-637 Yapılacaklar (A, Ö1, yerli ses girişi; roadmap, yerel sesli asistan 6. adım):** dış süreçten okunan seste (`ffmpeg` dshow, `arecord`) kaynağın kendi veri kaybını TR görmez; yerli ses girişi yok [kanıtlı: tools/assistant/asistan.tr:332; docs/roadmap.md:299].
- **S-638 Mevcut durum (sınır):** ortam süzgecinin `AUTH` sözcüğü `SSH_AUTH_SOCK`'u da süzer; `TR_env:` ile verilmezse dış program (git push) ajana ulaşamaz [kanıtlı: internal/stdlib/process/process.go:119].
- **S-639 Yapılacaklar (test, Ö3; sahip S1):** akışlı kipte `TestGrandchildDoesNotHangTheProcess` süreyi yalnız üst sınırla sınıyor; toplu kipteki `TestGrandchildHoldingThePipeDoesNotBlock` gibi alt sınırı da sınasın [kanıtlı: internal/stdlib/process/behavior_spawn_process_test.go:419].
- **S-640 Yapılacaklar (F, Ö3; sahip S1):** iki yerleşiğin yorumu yer tutucuyu argümanlar diye yazıyor, dil tanımı `argüman_listesi` [kanıtlı: internal/stdlib/process/run_process.go:50, spawn_process.go:55].
- **S-641 Yapılacaklar (E, Ö3; sahip süreç kolu (Motor)):** `TR_run_process`'in `TR_merge_stderr:`'ı (21.4) yok: ad kod listesinde değil, yalnız onaylı yeni adlarda [kanıtlı: internal/devtools/testrunner/behavior_language_spec_test.go:394]. Birleşik kipte çıkış hatasının iletisi birleşik çıktının sonunu taşır.
- **S-642 Yapılacaklar (sınıfsız: test inceliği, Ö3; sahip Motor, sonraki process dokunuşu):** `TestUnreadProgramStopsWithAWaitLimit`'in üst sınırı (iki tampon) tavandan gevşek, yorumu beklemeyi alt sınıra bağlıyor [kanıtlı: internal/stdlib/process/behavior_spawn_process_test.go:333-334, :351]. Sıradaki: üst sınır bir tampon ve üç okuma parçası (program_output.go:12-13), yorum iki sınıra ayrılır.

## 22. Bayt ve sayı

Kurallar: [`docs/spec/22_bayt_ve_sayi.md`](spec/22_bayt_ve_sayi.md).

### 22.1–22.4

- **S-644 Yapılacaklar (F, Ö3; sahip S1):** H7067 ve H7080 metin olmayan parça adını sayı listesi diye gösteriyor [kanıtlı: lib/audio.tr:185].

### 22.5

## 23. Sinyal ve spektrum

Kurallar: [`docs/spec/23_sinyal_ve_spektrum.md`](spec/23_sinyal_ve_spektrum.md).

### 23.1–23.6

- **S-646 Yapılacaklar (E, Ö3; sahip S1):** GPU'da Fourier `n <= 4096`'da taban matrisiyle çalışıyor, üstü H4134; 23.5'in FFT çekirdeği yok [kanıtlı: internal/compute/tensor/fourier_card.go:17, :77].

## 24. Sohbet biçimi

Kurallar: [`docs/spec/24_sohbet_bicimi.md`](spec/24_sohbet_bicimi.md).

### 24.1–24.4

- **S-647 Yapılacaklar (E, Ö3):** `TR_read_chat` (24.4) yok [kanıtlı: `git grep read_chat -- internal/lang/codes/definitions.json lib` boş].

### 24.3, 24.6–24.8

- **S-650 Yapılacaklar (D, Ö3; sahip dil çekirdeği kolu, denetleyici):** denetleyici aktarılan modülün yapılarını ve metot imzalarını okumuyor: başka modülden gelen yapının metodunun dönüşü denetlenmiyor, yanlış tip çalışırken hata verir [kanıtlı: internal/lang/checker/module_values.go:16-20]. Sıradaki: 11.1'in modüller arası tip denetimiyle (4b) aynı yol.
- **S-692 Yapılacaklar (D, Ö3; sahip dil çekirdeği kolu, denetleyici):** yerleşik üyenin dönüşü tipi bilinmeyen alıcıda hâlâ geçiyor, ikizi `internal/lang/checker/pure_calls.go:20`; düz ad dalı testsiz ve yorumu onu aktarılan modülün yapısı sayıyor [kanıtlı: internal/lang/checker/builtin_returns.go:63-64; test: `TestLibraryStructMethodIsNotTheBuiltinMember` yalnız kitaplık tipini sınıyor].
- **S-651 Yapılacaklar (D, Ö3; sahip Dost Interface):** cevap hatası (`TR_Message`) okunur metniyle değil kurucu yazımıyla yazdırılıyor; `.TR_text` dili ister, `Zincir` dili bilmiyor [kanıtlı: tools/assistant/zincir.tr:331].
- **S-652 Yapılacaklar (F, Ö3; sahip D1, aileler kolu):** kimlik kapısının düz `tokenizer` istisnası o sözcüğü her bölümde denetimsiz bırakıyor; tek kullanımı `docs/spec/15_tensorler.md:755` [kanıtlı: internal/devtools/testrunner/behavior_language_spec_test.go:278]. Sıradaki: o yer JSON anahtarının tırnaklı yazımıyla yazılınca istisna silinir.

## 25. Konuşmayı yazıya çevirme

Kurallar: [`docs/spec/25_konusma_tanima.md`](spec/25_konusma_tanima.md).

### 25.1–25.9

- **S-653 Mevcut durum (Whisper `small`, dizüstü CPU, Türkçe):** TR'nin çevirmesi faster-whisper float32'den hızlı, int8'den ve whisper.cpp'den yavaş; tepe bellek whisper.cpp'ninkinden büyük, faster-whisper float32'ninkinden küçük [ölçüldü: STT-2026-11-22-LAPTOP-H1-3, STT-2026-11-22-LAPTOP-H1-4, STT-2026-11-22-LAPTOP-H1-7].
- **S-654 Yapılacaklar (E, Ö2; sahip H1):** Whisper `small`'ın çevirmesi dizüstü CPU'da whisper.cpp'den yavaş, İngilizcede fark daha büyük [ölçüldü: STT-2026-11-22-LAPTOP-H1-7, STT-2026-11-22-LAPTOP-H1-8]. Sıradaki: kodlayıcının ve çözücü adımının profili.
- **S-655 Yapılacaklar (A [hipotez: gerçek klasörde görülmedi], Ö1; sahip S1):** 25.3'ün çok dillilik kuralı yok: `is_multilingual` okunmuyor, `lang_to_id` yoksa model tek dilli sayılıyor, açılışta hata yok [kanıtlı: lib/whisper.tr:386-390].
- **S-656 Yapılacaklar (D, Ö3; sahip S1):** 25.1'in dört özel parçası açılışta değil ilk çeviride aranıyor (H7099, öneki `TR_transcribe`) [kanıtlı: lib/whisper.tr:367, :466].
- **S-657 Yapılacaklar (E, Ö3; sahip Dost Interface (katalog) ve S1 (ses kitaplıkları)):** Whisper'ın tiny ve base boyuyla koşan test yok, Dost'un katalogunda yalnız `whisper-small` var [kanıtlı: tools/assistant/katalog.json:255]. Sıradaki: tiny ile `lib/whisper.tr` testi ve dizüstü CPU'da Türkçe doğruluk ölçüsü; katalog girdisi ölçüden sonra.

### 25.10–25.17

- **S-658 Yapılacaklar (F1, Ö3; sahip T1):** H7539'un önerisi yalnız üst sınırı anıyor (`TR_max_speech: 29.0`); sessizlik ya da baş tamponu çok büyükken öneri kopyalansa da kesitleyici kurulmaz [kanıtlı: lib/vad.tr:168; internal/lang/messages/tr.json:1855]. Sıradaki: nedeni seçen kol.
- **S-659 Yapılacaklar (E, Ö2; sahip S1; Jarvis'in söz kesmesi):** TR_in_speech (25.13) yok ve üst sınır baş tamponu artı sessizliğe eşitken H7539 gelmiyor (25.12) [kanıtlı: lib/vad.tr:168; `tests/vad_test.tr:283`, `:342` eşitliği kabul ediyor]. Sıradaki: özellik `_tetik`'i okur (lib/vad.tr:148).

## 26. Türkçe sözcük çözümleme

- **S-660 Mevcut durum:** kökü hem ad hem fiil olan sözcüğün fiil çekimi ad çözümlemesi alıyor (`TR_analyze_word("yazdı")` kökü yaz, türü ad); aday seçimi sözcük türüne bakmıyor [kanıtlı: lib/turkish.tr:291-299].

## 28. Ağ: HTTP sunucusu ve indirme

Kurallar: [`docs/spec/28_ag.md`](spec/28_ag.md).

- **S-668 Yapılacaklar (F, Ö3; araç; sahip TCB):** `TestNoFixedNamesInMessages` alıcısı bilinmeyen çağrıyı yalnız metodun adıyla eşliyor, başka tipin aynı adlı metodu yanlış uyarı veriyor [kanıtlı: tools/gen/msgconsts/fixed_names_test.go:247]. Sıradaki: alıcının tipini anahtara katmak.
- **S-669 Yapılacaklar (F, Ö3; belge; sahip TCB):** kimlik kapısının düz `yol` istisnası o sözcüğü her bölümde denetimsiz bırakıyor; kullanımları `docs/spec/13_yerlesikler.md:88` ve `docs/spec/16_veri_parcalayici.md:78` [kanıtlı: internal/devtools/testrunner/behavior_language_spec_test.go:311]. Sıradaki: iki yer tutucu kod olmayan biçimde yazılınca satır silinir.
- **S-670 Yapılacaklar (D, Ö3; sahip HTTP sunucusu kolu):** sunucu `MaxHeaderBytes`'ı ayarlamıyor, başlıkların bayt sınırı Go'nun öntanımlısında, 28.5'in sayısından gevşek; 500 başlık değeri sınırının testi yok [kanıtlı: internal/stdlib/httpserver/serve.go:87-90]. Sıradaki: tam sınır Go'nun tamponuyla sağlanamıyorsa dilim durur ve sorar.
- **S-671 Yapılacaklar (D, Ö3; sahip HTTP sunucusu kolu):** 28.3'ün `Trailer`, `Keep-Alive`, `Upgrade` başlıkları ve `426` sunucuda ele alınmıyor; değerdeki HTAB H7611 alıyor, uçlardaki boşluğun kodu yok [kanıtlı: internal/stdlib/httpserver/response.go:142-143, :172-174]. Sıradaki: dilimin ilk adımı Go'nun bu başlıklardaki davranışını ölçer.

### 28.10–28.11

- **S-674 Yapılacaklar (E, Ö3; sahip Dost Interface; indirmenin D dilimi):** "Güncelleme var" bildirimi yok: Dost'un `catalog` olayındaki girdi yalnız `installed` ve `downloaded` taşıyor, kurulu kaydın sürümünü taşımıyor [kanıtlı: tools/assistant/yuva.tr:318-319]. Sıradaki: F2'nin kurulum sayfası `catalog`'a kayıt durumunu ekler.
- **S-675 Yapılacaklar (E, Ö3; sahip Motor):** tavan üstbilgiyle aynı anda dolarsa H7716 ilk isteğin konağını yazar, yönlendirilmiş zincirde asılan sonuncununkini değil (28.10) [kanıtlı: internal/stdlib/download/fetch.go:129-133, client.go:227-233]. Sıradaki: bu dalda konak resp.Request.URL.Host'tan.
- **S-676 Yapılacaklar (E, Ö3; sahip Motor; dil tanımı 28.10, pending):** 28.10 çok adresli adda her denemenin en az süresini (`minAttempt`) ve bu yüzden denenmeyen adresleri söylemiyor [kanıtlı: internal/stdlib/download/client.go:31, :152]. Sıradaki: dil tanımı turunda `docs/spec/pending.md:63`; davranış değişmez.
- **S-677 Yapılacaklar (C, Ö2; sahip Motor; dil tanımı 28.10):** `TR_download` yavaş gövdede adres ailesini değiştirmiyor: aile yalnız bağlanma düşünce değişir (`dialEach`), gövdede yalnız hareketsizlik sınırı var [kanıtlı: internal/stdlib/download/client.go:25, :145-163; fetch.go:226, :234]. Sıradaki: yavaş gövdede öbür aileye geçip `.part`'tan sürdürmek ya da iki aileyi yarıştırmak (dil kararı, Motor'la tasarım).
