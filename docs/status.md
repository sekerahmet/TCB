# Durum

Dil tanımının yeniden yazılan bölümlerine göre bugünkü TR'nin durumu ve kurala varmak için yapılacaklar. Her maddenin kuralı dil tanımındadır ([`docs/spec/`](spec/README.md)); yapılan iş testiyle birlikte buradan silinir. İşlerin sırası [yol haritasındadır](roadmap.md). Bu belgede kimlikler dil tanımının yazımıyla anılır (`TR_if`, 2.3a).

## Uygulamada verilen, dil tanımı turunda denetlenecek kararlar

Uygulama sırasında çıkan çelişki ya da dil tanımının sustuğu yer için ana oturumun kararı aynı gün dil tanımına yazılır; denetimi sıradaki dil tanımı turunda (Gemini ve DENETİM) yapılır. 2026-10-28, 2. katmandan: 3.7a (açmada `TR_List(j)`, karşılaştırma, dolaşma), 3.8 (`.TR_reverse()` liste verir), 3.10 (koşulun geçtiği öbür yerler), 3.12 (mantık değerinden sayı, metinden liste kurulmaz), 3.14 (yön karakterleri, kaptaki JSON, bayt dizisinin metni), 4.5 (fonksiyon içeren kap, isteğe bağlı fonksiyon ve `TR_none`), 4.9 (ifadenin ardındaki `TR_in`), 5.7 (dolaşılamayan JSON türleri), 6.6 (`_` parametre), 12.3 (koşul `TR_Bool`). Gerekçeleri ve kaynaklı emsalleri `docs/spec/rationale/`'de; kaynaklar o turda bağımsız denetlenir.

## 1. Program

Kurallar: [`docs/spec/01_program.md`](spec/01_program.md). Kararlar DENETİM'den ve Gemini'nin araştırmasından geçti (2026-10-27). 1.3–1.15 uygulandı (katman 1b, DENETİM'den geçti, 2026-10-28); aşağıda yalnız kalanlar.

### 1.1

- **Mevcut durum:**
  - Modülün ve test dosyasının en dışındaki deyimler de çalışıyor (11.1, 12.5).
  - Modülde ilk değer her ifadeyi kabul ediyor (5.1, 5.2).
- **Yapılacaklar:**
  - Denetleyici modülde ve test dosyasında en dışta tanım dışındaki deyimi hata sayar.
  - Modülde ilk değer sabit ifade değilse hata verir; fonksiyon adı, dışarıdan değişken yakalamayan fonksiyon ifadesi, sonucu yalnız argümanlarına bağlı sayı yerleşikleri ve `TR_fuse`, `TR_replay`, `TR_custom_grad` serbesttir.
  - `lib/`, `tests/` ve örnekler kurala göre düzeltilir.

### 1.2

- **Mevcut durum:** TR dinamik tiplidir; tip açıklaması isteğe bağlıdır (3.13).
- **Yapılacaklar:** büyük tasarım, turlarla:
  - denetleyiciye tip çıkarımı;
  - kitaplığın, örneklerin ve testlerin imzaları;
  - "olabilir" tipi;
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

## 2. Sözcük yapısı

Kurallar: [`docs/spec/02_sozcuk_yapisi.md`](spec/02_sozcuk_yapisi.md). DENETİM'den (üç tur) ve Gemini'nin araştırmasından geçti (2026-10-27). 2.1–2.10 uygulandı (katman 1, iki DENETİM turundan geçti, 2026-10-28); aşağıda yalnız kalanlar.

### 2.1

- **Mevcut durum:**
  - Belgelerde ters tırnak içinde sınıflı kimlik yazımı kalıyor (TR_kw_…, TR_fn_…, TR_mem_… biçimi; 48 belgede yaklaşık 2 770 yer, spec ve arşiv hariç) [ölçüldü: katman 1].
  - Bölüm 11 ve 17 eski başlığı, dosyanın dilini, iç içe aktarmayı ve gösterime göre konumu anlatıyor (11.3'ün eşzamanlı aktarma cümlesi, 17.1, 17.4, 17.5, 17.7); bölümler yeniden yazılınca düzelir.
- **Yapılacaklar:** belgelerdeki sınıflı yazım `tools/migrate`'in kimlik çevirisiyle çevrilir.

### 2.3a

- **Mevcut durum:** kimlik kapısı (`TestReviewedSectionsWriteIdentities`) bir `TR_` sözcüğünün herhangi bir yerde geçerli olmasını yeterli sayıyor; yanlış yerde yazılmış `TR_append(x)` geçiyor [kanıtlı: DENETİM].
- **Yapılacaklar:** tam ifade olan parça (`(`, `.` ya da `:` içeren) sözcük çözümleyiciyle okunup yeri denetlenir.

### 2.13

- **Mevcut durum:** `TR_cmd_check --json` çalışıyor (katman 2b, DENETİM'den geçti, 2026-10-28): sözcük çözümleyicinin, ayrıştırıcının, denetleyicinin ve komut satırının hataları ve uyarıları başlangıç ve bitiş konumu, en içteki kod, ilgili konum (H1020) ve düzeltmeyle JSON satırları olarak; satır başındaki `//` → `#` ve `0X1F` → `0x1F` uygulanabilir. Metin kipinde denetleyicinin iletileri sütunu da yazıyor [ölçüldü].
- **Yapılacaklar:** başlık hatasının (H2540) bitişi; ana dosyadaki okunamayan modülün (H2131) sütunu ve bitişi; iç içe modülün `TR_import` hatasının dosyası (yorumlayıcıda `ModuleError` sarmalayıcısı); `TR_cmd_check`'te dosyadan sonraki öbür argümanların durumu (1.4); aktarılan modülün uyarıları [ölçüldü: DENETİM].

## 3. Tipler ve değerler

Kurallar: [`docs/spec/03_tipler.md`](spec/03_tipler.md). Taslak: DENETİM'den geçti, bulguları işlendi (2026-10-27); Gemini'nin araştırması bekleniyor. TR bugün dinamik tipli; tip açıklamalarını yalnız çalışmadan önceki denetleyici kullanıyor. `tests/` altındaki 3. bölüm testleri eski kuralları da sınıyor (karışık tipli liste, doğruluk sınaması, çıplak ad anahtarı, açıklamasız değişkenin tipi); testler koşulmadan önce yeni kurallara göre yeniden yazılır. 3.2, 3.3a, 3.7, 3.7a, 3.8, 3.12 ve 3.14'ün çalışma anı uygulandı (katman 2, DENETİM'den geçti, 2026-10-28; 3.2, 3.8 ve 3.12 bitti, buradan silindi); aşağıda kalanlar. Eski 3.14 (tip adının tek yazımı) 2.3a'ya düştü; eski 3.15 (katalog iletisinin tipi) hata bölümüne gider, `tests/` altındaki 3.15 testi o bölüm yazılınca taşınır.

### 3.3a

- **Mevcut durum:** `TR_Bytes` kuruluyor, okunuyor, birleşiyor; metni "Bytes([104, 105])" [ölçüldü].
- **Yapılacaklar:** dosya ve ağ okumalarının bayt da vermesi (ölçülmedi).

### 3.5

- **Mevcut durum:** isteğe bağlı tip yok; `TR_Int` açıklamalı bir değişkene `TR_none` atanabiliyor [ölçüldü].
- **Yapılacaklar:** `T?` tipi; açılmamış kullanım denetim hatası; daraltma yalnız değişmeyen yerel değişkende: sınamanın dalında, `TR_else` dalında, `TR_and`/`TR_or`'un sağ yanında, her yolda biten erken çıkıştan sonra ve asıl tipten atamayla; alana atama daraltmayı bozmaz. `.TR_get` isteğe bağlı tip döner. Değer döndürmeyen çağrı değer olarak kullanılamaz; isteğe bağlı olmayanı `TR_none`'la karşılaştırmak denetim hatası.

### 3.6

- **Mevcut durum:** karışık tipli liste çalışıyor [ölçüldü].
- **Yapılacaklar:** `TR_List[T]`; öğelerin tipi aynı, boş liste tipini yazar; `TR_none` içeren değişmez `TR_List[T?]`.

### 3.7

- **Mevcut durum:** anahtar ifade, erişim köşeli parantezle, olmayan anahtar hata (H2647), ondalık anahtar hata (H2646) [ölçüldü].
- **Yapılacaklar:** `TR_Dict[K, V]` ve anahtar tipinin denetimi (4a).

### 3.7a

- **Mevcut durum:** `TR_JSON` kuruluyor, açılıyor, dolaşılıyor; üst düzey `j["a"] == 1` çalışma hatası. Ama kapta ve eşlemede JSON ile JSON olmayanın karşılaştırması hâlâ sessizce yanlış: `TR_List(j) == [1, 2]`, `TR_List(j).TR_contains(1)`, `[j[0]] == [1]` → false; `TR_match j["a"] { 1 => ... }` `_` koluna düşüyor [ölçüldü: DENETİM ve ana oturum].
- **Yapılacaklar:** kapın içindeki karşılaştırma, `.TR_contains` ve eşleme deseni de çalışma hatası (3. katman, 3A); `TR_Type(j) == TR_JSON` denetim hatası ve genel tiplerin (Any, Object, StructType) kalkması (4a).

### 3.10

- **Mevcut durum:** koşulda yalnız `TR_Bool`; başka tip çalışma anında hata veriyor (H2600), ileti açık sınamayı öneriyor (katman 2) [ölçüldü]. Hata ayıklayıcının koşullu durağı hâlâ doğruluk sınamasıyla (`IsTruthy`) okunuyor.
- **Yapılacaklar:** başka tip denetim hatası da olur (katman 4); hata ayıklayıcının koşulu da yalnız `TR_Bool` alır.

### 3.13

- **Mevcut durum:** `0` ile başlayan değişkene ondalık eklenebiliyor [ölçüldü]; açıklamasız değişkenin tipi kalıcı değil.
- **Yapılacaklar:** statik tip (1.2); kalıcı yerel tip; tam sayı yazımı ondalık beklenen yerde ondalık.

### 3.14

- **Mevcut durum:** değerin metni dilden bağımsız ("true", "none", "inf"); tipin metni öğe tipsiz: `TR_Type([1])` "List" [ölçüldü].
- **Yapılacaklar:** "List[Int]" ve "Int?" (öğe tipi ister; 4a).

## 4. İşleçler

Kurallar: [`docs/spec/04_operatorler.md`](spec/04_operatorler.md). Taslak: DENETİM'den geçti, bulguları işlendi (2026-10-27); Gemini'nin araştırması bekleniyor. 4.1–4.4, 4.6, 4.9, 4.11 ve 4.12'nin çalışma anı uygulandı (katman 2, DENETİM'den geçti, 2026-10-28); aşağıda yalnız kalanlar. Tip isteyen denetim hataları katman 4'tedir.

### 4.5

- **Mevcut durum:** ayrı tipler sessizce eşit değil sayılıyor; fonksiyonlar yalnız üst düzey `==` ve `!=`'de hata veriyor (H2613), fonksiyon içeren liste kimlikle karşılaştırılıyor (`[f] == [f]` doğru) [ölçüldü].
- **Yapılacaklar:** fonksiyon içeren kabın karşılaştırması çalışma hatası (3. katman, 3A); ayrı tipler ve fonksiyon içeren değer denetim hatası (4. katman).

### 4.7

- **Mevcut durum:** sağ yanda parantezsiz ikili işleç sözdizimi hatası (H0201); isteğe bağlı tip olmadığı için sol yan her tipten olabiliyor: `1 ?? 2` 1 veriyor [ölçüldü].
- **Yapılacaklar:** sol yan isteğe bağlı değilse denetim hatası.

### 4.9

- **Mevcut durum:** satır sonunda `//`'den sonraki metin bir ifade değilse `#`'i anmayan bir hata çıkıyor: `TR_var x = 5 // bu bir yorum` iki kez "ifadenin değeri kullanılmıyor" veriyor [ölçüldü].
- **Yapılacaklar:** bu durumda da ileti `#`'i önerir.

### 4.10

- **Mevcut durum:** başka tiple `+` ve `*` çalışma anında hata veriyor (H2609, H2614); öğeleri başvuruyla paylaşılan liste sessizce yineleniyor: `[[0] * 3] * 3`'ün üç satırı aynı liste [ölçüldü].
- **Yapılacaklar:** ikisi de denetim hatası olur; paylaşılan listenin iletisi her satırı ayrı kuran yazımı önerir.

## 5. Deyimler ve kontrol yapıları

Kurallar: [`docs/spec/05_deyimler.md`](spec/05_deyimler.md). Taslak: DENETİM'den geçti, bulguları işlendi (2026-10-27); Gemini'nin araştırması bekleniyor. `tests/deyimler_test.tr` eski kuralları da sınıyor; testler koşulmadan önce yeni kurallara göre yeniden yazılır [DENETİM'in listesi]:
- 5.1: `:=` tanımı, ilk değersiz değişken;
- 5.2: yerel büyük harfli sabit (2.3b);
- 5.4: tam sayı değişkene `/=`;
- 5.5: `TR_else`'siz `TR_if` ifadesinin yokluk vermesi;
- 5.6: `TR_default`, kapsamayan iki eşleme, düz adla yazılmış değişken deseni (`(x)`'e çevrilir);
- 5.7: `{i}{x}` yerleştirmesi (2.7), geriye sayan `3..0` (3.8), tipsiz boş listeler (3.6);
- 5.8: `TR_while`;
- 5.12: kolda dıştaki fonksiyonla aynı adlı fonksiyon (yeni kurala göre hata); adlı fonksiyon ifadesinin adının dışarı çıkması (6. bölümün kararı);
- 5.13: kapsamayan eşleme;
- her yerde: tipsiz parametreler (1.2) ve `//` yorumları (2.2).

`TR_elif` bilerek kimliği olmayan addır; kimlik kapısı düzelince bilerek yanlış örnekler listesine girer (2. bölümün girişi). Yeni denetim hataları (5.1, 5.5, 5.6, 5.12, 5.14) uygulanırken depodaki kodlu dosyalar da düzeltilir; yoksa `TestRepoTRFilesPassCheck` kapısı kalır. Anlamı değişmeyen maddelerin bugünkü TR testi var: sabite atama denetim hatası, fonksiyonun içindeki `TR_break` sözdizimi hatası veriyor, takas çalışıyor [ölçüldü]. Koşullu döngünün `TR_while`'sız yazımı 2.9'da.

### 5.1

- **Mevcut durum:** `x := 5` çalışıyor; ilk değersiz `TR_var y` yokluk alıyor; hiç yeniden atanmayan değişken uyarı vermiyor [ölçüldü].
- **Yapılacaklar:** ilk değer zorunlu; `:=` sözdizimi hatası, ileti `TR_var`'ı önerir; fonksiyonun içinde yeniden atanmayan yerel `TR_var` uyarısı `TR_const`'u önerir (programın en dışı girmez). Depodaki `:=` kullanımları çevrilir.

### 5.3

- **Mevcut durum:** `TR_var p, q = [1, 2, 3]` hata vermiyor, ilk iki öğeyi alıyor; `TR_var a, _ = 1, 2` sonra `TR_print(_)` 2 veriyor [ölçüldü].
- **Yapılacaklar:** öğe sayısı tutmazsa çalışma hatası; `_` okunamaz (sözdizimi hatası; 3. katman, 3A).

### 5.4

- **Mevcut durum:** tam sayı değişkene `/=` 3.5 yazıyor; `%=`, `**=` ve `&=` sözdizimi hatası veriyor; `k //= 2` yorum sayılıyor; listede `+=` yeni liste kuruyor, listeyi paylaşan ad eski listede kalıyor [ölçüldü].
- **Yapılacaklar:** on iki bileşik atama; sonucun tipi değişkenin tipi, `/=` iletisi `//=`'yi önerir; listede `+=` ve `*=` yerinde, `*=` 4.10'un yineleme kuralıyla; hedef bir kez hesaplanır; tanımsız ada atamanın iletisi `TR_var`'ı önerir.

### 5.5

- **Mevcut durum:** `TR_else`'siz değer veren `TR_if` yokluk veriyor; `TR_elif` "bilinen bir kimlik değil" diyor, `TR_else TR_if`'i önermiyor [ölçüldü].
- **Yapılacaklar:** ifade olarak kullanılınca `TR_else` zorunlu, dalların tipi aynı; `TR_elif` iletisi `TR_else TR_if`'i, Python'un koşullu ifadesinin iletisi `TR_if` ifadesini önerir.

### 5.6

- **Mevcut durum:** değişkenin adı deseni değeri karşılaştırıyor; eşleşmeyen `TR_match` yokluk veriyor, kapsama denetimi yok [ölçüldü]. Varsayılan kol `TR_default` ile de yazılıyor (2.9).
- **Yapılacaklar:** kapsama denetimi (`TR_Bool`'da iki değer, öbür tiplerde `_`); ifade olarak kullanılınca kolların tipi aynı; düz ad deseni yalnız sabit, değişken düz adla yazılınca denetim hatası, ileti `(x)`'i ya da `_`'yi önerir.

### 5.7

- **Mevcut durum:** dolaşılan sözlüğe anahtar eklemek hata vermiyor (sonuç `{"a": 1, "b": 2}`), dolaşılan listeye eklemek de (sonuç `[1, 2, 1, 2]`) [ölçüldü].
- **Yapılacaklar:** dolaşırken listeye ya da sözlüğe ekleme ve çıkarma çalışma hatası; var olan öğeye ve anahtara atama serbest. Bayt dizisinde ve `TR_JSON`'da dolaşma tipleriyle birlikte (3.3a, 3.7a).

### 5.12

- **Mevcut durum:** iç bloktaki `TR_var t = 5` dıştaki `t`'yi gizliyor, dıştaki 0 kalıyor, hata yok [ölçüldü]. Üretecin değişkeni dıştakini değiştirmiyor, kuralla aynı [ölçüldü, DENETİM].
- **Yapılacaklar:** kapsayan ya da aynı kapsamdaki yerel adla yeniden tanım denetim hatası; programın en dışı ve parametreler de kurala girer; fonksiyon sınırında gizleme serbest.

### 5.14

- **Mevcut durum:** tanımsız ad çalışmadan önce bulunuyor ve en yakın adı öneriyor; tanımından önce kullanılan ad ise çalışma hatası veriyor [ölçüldü]. Kendisinden sonra tanımlanan en dış değişkeni okuyan fonksiyon, değişkenden sonra çağrılınca çalışıyor [ölçüldü, DENETİM].
- **Yapılacaklar:** tanımından önce kullanma denetim hatası; öneri 2.3a'nın uzaklığıyla. Fonksiyonu çağıran ifade, fonksiyonun doğrudan ya da dolaylı kullandığı değişkenlerden sonra gelmeli; modülün en dışında sıradan bağımsızlık ve sabitler arasında döngü hatası.

## 6. Fonksiyonlar

Kurallar: [`docs/spec/06_fonksiyonlar.md`](spec/06_fonksiyonlar.md). Taslak: DENETİM'den geçti, bulguları işlendi (2026-10-27); Gemini'nin araştırması bekleniyor. Kuralla aynı olanlar: döngüde kurulan kapanış o turun değerini görüyor, fonksiyon dıştaki değişkene atayabiliyor, tipi tutmayan argüman çalışmadan önce bulunuyor [ölçüldü]; varsayılan her çağrıda yeniden hesaplanıyor, derinlik sınırı 10000 (bugünkü testler).

`tests/fonksiyonlar_test.tr` eski kuralları da sınıyor; testler koşulmadan önce yeni kurallara göre yeniden yazılır:
- tipsiz parametreler (1.2);
- tanımından önce çağrının hata vermesi (5.14'e aykırı);
- son ifadenin örtük dönüşü (6.2);
- dönüş tipinin `:` ile yazımı (6.1);
- parantezsiz ok ve adsız `TR_func` (6.6);
- `TR_it` ve `TR_acc`'li sondaki blok, son argümanda `() => { ... }` (6.7);
- argüman hatalarını çalışırken yakalayan testler (eski 6.3 ve 6.4: eksik, fazla, iki kez verilen argüman, varsayılandaki ileri başvuru): artık denetim hatası olduklarından `tests/negative/` fikstürlerine taşınır, yoksa dosya denetimden geçmez (1.3);
- 6.7 testinin adındaki `\{` (2.6), `//` yorumları ve eski başlık (2.1, 2.2);
- numaralar: eski 6.3 → 6.4, 6.4 → 6.5, 6.5 → 6.8, 6.8 → 6.1, 6.3a → 6.11.

Kapıların bu bölümdeki kırılmaları, testler yeniden yazılırken giderilir: 6.3'ün Test satırı "gerekmez" diyor ama eski 6.3 numaralı testler var; 6.11 "TR testi" diyor ama 6.11 numaralı test yok (bugün 6.3a) [DENETİM].

Depodaki kullanımlar çevrilir: `.tr` dosyalarında `TR_it` 44, `TR_acc` 10, adsız `TR_func` 52, parantezsiz ok 21 yerde [ölçüldü]; bütün depoda (belgeler ve Go testleri dahil) `TR_it` 88, `TR_acc` 29, adsız `TR_func` 63 [ölçüldü, DENETİM]; 13.6'daki `TR_reduce` bloğu da.

### 6.1

- **Mevcut durum:** parametre tipi isteğe bağlı; dönüş tipi hem `:` hem `->` ile yazılıyor; aynı adla ikinci tanım hata vermiyor, ikincisi kazanıyor; `def f(x):` genel bir sözdizimi hatası veriyor [ölçüldü].
- **Yapılacaklar:** parametre tipi zorunlu; dönüş tipinin tek yazımı `->`, `-> TR_NoneType` hata; aynı kapsamda ikinci tanım denetim hatası; `def`, `: R` ve süsleyici için iletiler.

### 6.2

- **Mevcut durum:** dönüş tipi yazılmış fonksiyonda `TR_return`'süz yol yokluk veriyor; gövdenin son ifadesi örtük dönüyor (`TR_func s(x) { x }` 4 veriyor) [ölçüldü].
- **Yapılacaklar:** eksik dönüş denetim hatası, ileti `TR_return`'ü önerir; bitiren deyimler: `TR_return`, `TR_throw`, `TR_exit`, sonsuz döngü, bütün dalları biten `TR_if` ve `TR_match`; fonksiyon ifadesinin blok gövdesi de; örtük dönüş kalkar.

### 6.3

- **Mevcut durum:** `TR_return 1, 2` sözdizimi hatası veriyor [ölçüldü].
- **Yapılacaklar:** birden çok değer döndürme ve açma; `TR_return f()` ile iletme; tek değer olarak ve argüman olarak kullanımı denetim hatası.

### 6.4

- **Mevcut durum:** varsayılansız parametrenin varsayılanlıdan sonra gelmesi ölçülmedi.
- **Yapılacaklar:** varsayılanlı parametrelerden sonrakilerin de varsayılanı olur; varsayılanın denetimi çalışmadan önce.

### 6.5

- **Mevcut durum:** eksik argüman ancak çalışırken bulunuyor ("'m' fonksiyonu 2 argüman bekler, 1 verildi"); Python'un `b=5` yazımı genel bir sözdizimi hatası veriyor [ölçüldü].
- **Yapılacaklar:** eksik, fazla, bilinmeyen ve iki kez verilen argüman denetim hatası; `k=2` iletisi iki noktalı yazımı önerir.

### 6.6

- **Mevcut durum:** üç yazım da çalışıyor: `x => ...`, `(x) => ...`, adsız `TR_func(x) { ... }` (bugünkü 6.6 ve 6.5 testleri); `(_) => _` `_`'yi okuyor [ölçüldü: DENETİM].
- **Yapılacaklar:** yalnız parantezli ok; öbürleri ve `lambda` sözdizimi hatası, ileti oku önerir; `TR_func` tanımı ifade yerinde hata; `_` parametre okunamaz (3. katman, 3A).

### 6.7

- **Mevcut durum:** sondaki blok tek parametreli, parametresi `TR_it`; `TR_reduce` bloğunda `TR_acc` (bugünkü 6.7 testleri).
- **Yapılacaklar:** sondaki blok parametresiz; `TR_it` ve `TR_acc` kalkar (2.9b); blokta `TR_return` denetim hatası; son argümanda `() => { ... }` ve `() => x` sözdizimi hatası, ileti sondaki bloğu önerir; sondaki blokta 5.12'nin gizleme yasağı.

### 6.8

- **Mevcut durum:** fonksiyon dıştaki değişkene uyarısız atıyor [ölçüldü].
- **Yapılacaklar:** dıştaki değişkene okumadan önce atama uyarısı, ileti `TR_var`'ı önerir.

### 6.9

- **Mevcut durum:** fonksiyonun metni dosyanın dilinde ve tipsiz: ok fonksiyonu için "<ok fonksiyonu>" yazılıyor [ölçüldü].
- **Yapılacaklar:** fonksiyon tipinin yazımı (`(T) -> R`); metin tipin metni. Kimlik listesindeki genel fonksiyon tipi kalkar (3.11).

### 6.10

- **Mevcut durum:** `TR_func f[T](l)` sözdizimi hatası veriyor [ölçüldü].
- **Yapılacaklar:** tip parametreleri ve çağrıda çıkarımı; kısıtsız `T`'de izin verilen işlerin denetimi.

## 7. Yapılar

Kurallar: [`docs/spec/07_yapilar.md`](spec/07_yapilar.md). Taslak: DENETİM'den geçti, bulguları işlendi (2026-10-27); Gemini'nin araştırması bekleniyor. Kuralla aynı olanlar: nesne başvuruyla paylaşılıyor (`q.x = 9` `p.x`'i de değiştiriyor), eşitlik alanlardan, çağrısız metot nesneye bağlı bir fonksiyon [ölçüldü].

İki yeni anahtar kelime kimlik listesine eklenir: `TR_enum` (7.7) ve `TR_interface` (7.8) (Ahmet, 2026-10-27, adları ana oturuma bıraktı: "senin için hataya en az sebep olacak hangisi ise o isimleri seçebilirsin"). Yapının tipi olan genel tip (StructType) kalkar; yapının tipi `TR_Type`'tır (3.11).

`tests/yapilar_test.tr` eski kuralları da sınıyor; testler koşulmadan önce yeni kurallara göre yeniden yazılır:
- tipsiz alanlar ve varsayılanlıdan sonra varsayılansız alan (7.1); tipsiz metot ve kurucu parametreleri (1.2);
- verilmeyen alanın yokluk alması (7.2);
- argüman ve alan hatalarını çalışırken yakalayan 7.2 ve 7.5 testleri: artık denetim hatası, `tests/negative/` fikstürlerine taşınır;
- `p?.z` (7.5, 4.8);
- eski metin biçimi ve yapının tipinin StructType olması (7.6); `//` yorumları ve eski başlık.

Örneklerdeki eski genel tipler de kalkar: StructType, Function, BuiltinFunction, None (`examples/tr/12_tip_adlari.tr`, `14_yapilar_ve_metotlar.tr`) [DENETİM].

Kütüphanenin bedeli [ölçüldü, DENETİM]: depoda 38 yapı tanımı (`lib/`'de 16); alanlar ve kurucu parametreleri tipsiz, tipli yazıma çevrilir. Alan daraltılmadığı için (3.5) `lib/`'deki 15 `TR_self.alan != TR_none` sınaması yerel kopyaya ya da `?.`'ya çevrilir; "yoksa varsayılan ata" kalıbının 4 yeri 3.5'in atamayla daraltmasıyla geçerli kalır. `lib/katman.tr`'nin kurucuları 7.4'e uyuyor.

JSON'dan tipli yapıya doğrudan okuma (alan adı eşlemesiyle; Rust'ın serde'si, Python'un pydantic'i gibi) henüz hiçbir bölümde yok; sonraki bölümlerde tasarlanır.

### 7.1

- **Mevcut durum:** alan tipsiz yazılabiliyor (`y = 0`); varsayılanlı alandan sonra varsayılansız alan geçerli (bugünkü 7.1 testi).
- **Yapılacaklar:** her alanın tipi; varsayılanlıların sonda olması; yapının tip parametresi; kalıtım yazımları için iletiler.
- **Ayrıştırıcının toparlanması:** yinelenen metottan sonra kapanan `}` ikinci bir H0141 veriyor (`tests/negative/yapi_yinelenen_metot.tr` bugünkü davranışı etiketliyor); tek yanlışa tek ileti, fikstürün etiketi düzelir.

### 7.2

- **Mevcut durum:** verilmeyen ve varsayılanı olmayan alan yokluk alıyor [ölçüldü].
- **Yapılacaklar:** zorunlu alan verilmezse denetim hatası.

### 7.3

- **Mevcut durum:** metotta düz alan adı tanımsız ad hatası veriyor; atamada ileti `TR_var`'ı [ölçüldü], okumada en yakın adı öneriyor, ilgisiz bir tip adı bile olabiliyor [ölçüldü, DENETİM]. Dışta aynı adlı değişken varsa düz ad onu okuyor (hata yok).
- **Yapılacaklar:** alan ya da metot adıyla aynı düz ad denetim hatası, ileti `TR_self.x`'i önerir; metot `TR_self.m()` ile; Python'un self parametresi için ileti.

### 7.4

- **Mevcut durum:** kurucu alanı atamasa da konumlu argüman alana yazılıyor (`D(3).n` 3 veriyor) [ölçüldü].
- **Yapılacaklar:** kurucu varken yalnız onun imzasıyla kurma; varsayılanlı alanlar kurucudan önce, tanım sırasıyla; her alanın kurucuda atanma denetimi.

### 7.5

- **Mevcut durum:** `TR_Int` alana metin atanabiliyor; olmayan alan ancak çalışırken bulunuyor ("alanı veya metodu bulunamadı") [ölçüldü]; `p?.z` yokluk veriyor (bugünkü 7.5 testi).
- **Yapılacaklar:** alan tipinin denetimi; olmayan alan denetim hatası, ileti en yakın alanı önerir.

### 7.6

- **Mevcut durum:** nesnenin metni `Nokta{x: 1, y: 2}` biçiminde [ölçüldü]; yapının tipi StructType (bugünkü 7.6 testi).
- **Yapılacaklar:** metin kurma yazımıyla ("Nokta(x: 1.0, y: 2.0)"), döngüde "..."; döngüde `==` çalışma hatası; fonksiyon ya da tensör alanlı yapının `==`'si denetim hatası.

### 7.7

- **Mevcut durum:** durumlu tip yok.
- **Yapılacaklar:** `TR_enum`, tip parametresi, durumların kurulması (çağrısız alanlı ve çağrılı alansız durumun hatası, `TR_init`'in yokluğu), eşlemede konumlu desen (ad sayısının hatası, adlı alanın iletisi), alan bağlama ve kapsama, koşullu kol iletisi, metin (3. katman, 3B).

### 7.8

- **Mevcut durum:** arayüz yok.
- **Yapılacaklar:** `TR_interface` (en az bir metot; boş arayüz denetim hatası), yapısal karşılama, eksik metodu söyleyen ileti, tip parametresinde kısıt, `TR_Type` sınamasıyla asıl tipe daraltma. Kimlik kapısının onaylı yeni adlar listesine `TR_enum` ve `TR_interface` girer.

## 8. Hata yönetimi

Kurallar: [`docs/spec/08_hata_yonetimi.md`](spec/08_hata_yonetimi.md). Taslak (2026-10-28): ana oturum yazdı, DENETİM'den geçti (1–7 ile tutarlılık; bulguları işlendi; Ç1: yakalamanın adı zorunlu, 2.9b'den `TR_error` kalktı, Ahmet 2026-10-28), Gemini'nin araştırması bekleniyor. `tests/` altındaki 8. bölüm testleri eski kuralı da sınıyor (yakalanan hatanın metin olması, her değerin fırlatılıp metne çevrilmesi, adsız yakalamada eski ad); kural uygulanırken yeniden yazılır.

### 8.1

- **Mevcut durum:** `TR_try`/`TR_catch` çalışıyor, `TR_catch` zorunlu; `TR_try` ifade olarak sözdizimi hatası; `TR_finally` bilinmeyen kimlik; Python'un `except`'i ipucu alıyor [ölçüldü].
- **Yapılacaklar:** ifade olarak `TR_try`; adsız `TR_catch` sözdizimi hatası (ileti `TR_catch e`; göç: depodaki 7 adsız yakalama ve 4 `TR_error`), `TR_error` kimliği kalkar; parantezli adın ve Python'un `except`/`finally`'sinin iletileri.

### 8.2

- **Mevcut durum:** her değer fırlatılabiliyor ve metne çevriliyor (`TR_throw {"kod": 5}` yakalanınca metin); `raise` sözdizimi hatası değil, denetleyicinin tanımsız ad önerisi `TR_throw`'u öneriyor; neden taşınamıyor [ölçüldü: DENETİM].
- **Yapılacaklar:** fırlatılabilen dört tür (metin, ileti, durumlu tip değeri, `TR_Error`), başka tip çalışma hatası; `TR_Error(..., TR_cause: e)`; `raise`'in iletisi.

### 8.3

- **Mevcut durum:** yakalanan hata bir `TR_String`; ileti kodlu hatada kod kayboluyor, yakalayan yalnız metni görüyor [ölçüldü].
- **Yapılacaklar:** `TR_Error` tipi ve üyeleri (`TR_code`, `TR_message`, `TR_value`, `TR_cause`); `TR_value`'nun kapalı birleşim tipi (4. katman); depodaki `TR_catch` kullanımları üyelere çevrilir.

### 8.4

- **Mevcut durum:** denetim karakterleri kaçırılıyor, ama `\x` yazımıyla: `TR_throw "a\u{1b}[2Jb\u{7}c"` → `a\x1b[2Jb\x07c` [ölçüldü: DENETİM].
- **Yapılacaklar:** 2.6'nın kaçışı (`\u{1b}`).

### 8.5

- **Mevcut durum:** `TR_Message` ve `TR_error_code` çalışıyor (`H6903`); düz metin hata da kodlu: `TR_error_code { TR_throw "x" }` → H2441 [ölçüldü: DENETİM].
- **Yapılacaklar:** düz metin hatanın kodu `TR_none`; nedenler zincirinde dıştan içe ilk kod.

### 8.6

- **Mevcut durum:** yakalanmayan hata yalnız fırlatıldığı satırı gösteriyor, çağrı yığını yok [ölçüldü].
- **Yapılacaklar:** çağrı yığını, en son çağrı en altta.

## 9. Ertele

Kurallar: [`docs/spec/09_ertele.md`](spec/09_ertele.md). Taslak (2026-10-28): ana oturum yazdı, DENETİM bekleniyor. `tests/hatalar_ertele_test.tr` ve `examples/tr/16_ertele.tr` eski kuralı sınıyor (fonksiyona bağlı kapsam, çağrı yazımı, birleşik hata metni); kural uygulanırken yeniden yazılır.

### 9.1

- **Mevcut durum:** ertele fonksiyona bağlı: döngüde kaydedilen işler fonksiyonun sonunda toplu çalışıyor ("tur 0, tur 1, tur 2, son, kapat 2, kapat 1, kapat 0"), `TR_if` içindeki de fonksiyonu bekliyor; ertelenen bloğun içinde kaydedilen ertele hiç çalışmıyor, sessizce kayboluyor; ters sıra doğru; çağrı yazımı (`TR_defer TR_print(x)`) ve blok yazımı ikisi de çalışıyor. Depoda 31 ertele: 11 çağrı yazımı, 20 blok, 3 dosyada [ölçüldü].
- **Yapılacaklar:** kapsamın blok oluşu; çağrı yazımı sözdizimi hatası, ileti blok yazımını önerir; göç: 11 çağrı yazımı bloğa, fonksiyona bağlı kapsamı sınayan testler ve örneğin "her tur için ayrı fonksiyon" bölümü. Sayım türü (kuralı uygulayan katman): fonksiyon, test, görev ya da program gövdesi dışındaki blokta kaydedilen ertele; durumu geri koyan bir ertele (`TR_default_device` gibi) orada sessizce erken çalışır. Bugün depoda tek yer: `tests/hatalar_ertele_test.tr:73` [ölçüldü: DENETİM].

### 9.3

- **Mevcut durum:** blok çalıştığı anda okuyor (`TR_defer TR_print(x)` sonra `x = 2` → 2); ertelenen bloktaki `TR_return 5` sessizce yok sayılıyor, fonksiyon 1 döndürüyor; `TR_break` ve `TR_continue` sözdizimi hatası, iletileri `TR_return`'ü öneriyor [ölçüldü: DENETİM].
- **Yapılacaklar:** ertelenen bloktan `TR_return` ile çıkışın hatası; üçü de sözdizimi hatası (5.13'ün "denetim" sözü 3A birleşince "sözdizimi" olacak: dosya okunurken bulunan hata sözdizimi hatasıdır, katman 3'ün 1. kararı); iletiler `TR_return`'ü önermez.

### 9.4

- **Mevcut durum:** hatasız kapsamda ertelenen işin hatası yakalanıyor; iki hata tek metinde birleşiyor ("govde hatasi; error in defer as well: ertele hatasi") [ölçüldü].
- **Yapılacaklar:** ilk hata korunur, öbürleri `e.TR_suppressed`'e (8.3 ile).

## 13. Yerleşikler

Kurallar: [`docs/spec/13_yerlesikler.md`](spec/13_yerlesikler.md). Taslak, 2. tur (2026-10-28): ana oturum yazdı, 1. turu DENETİM ve ana oturum inceledi, bulgular işlendi; 2. turun DENETİM'i bekleniyor. `tests/yerlesikler_test.tr` eski kuralları da sınıyor (kazanan değerin tipi, sayı hücreli CSV, `TR_mean`'in tek tek değerleri); kural uygulanırken yeniden yazılır.

### 13.1

- **Mevcut durum:** `TR_max` kazanan değerin tipini veriyor: `TR_max(2, 1.5)` `TR_Int` 2, `TR_max(1, 1.5)` `TR_Float` 1.5; metin argüman hata; `TR_max([])` `TR_none`; `TR_max(1)` 1 veriyor; `TR_mean(1, 2)` 1.5 (depoda 2 yer); `TR_round(5, 2)` `TR_Float` 5.0, `TR_round(3.7, 0)` `TR_Int` 4, `TR_round(TR_nan)` hata; `TR_print` yazdığı metni, `TR_sleep(0)` ve `TR_wait_all()` `TR_true` döndürüyor; boş listede `TR_mean`, `TR_std`, `TR_median` hata [ölçüldü: DENETİM].
- **Yapılacaklar:** karışık sayıların `TR_Float`'ı, metnin sıralanabilir sayılması, boş listede `TR_min`/`TR_max`'ın hatası, tek argümanın hatası, `TR_mean`/`TR_std`/`TR_median`'ın yalnız liste alması, `TR_round`'un üç imzası, değer döndürmeyen üç yerleşik.

### 13.2

- **Mevcut durum:** harf dönüşümü basit eşleme: `"ß".TR_upper()` → "ß", `"İSTANBUL".TR_lower().TR_length` 8 [ölçüldü: DENETİM].
- **Yapılacaklar:** yok.

### 13.6

- **Mevcut durum:** `TR_reduce(0, (a, x) => a + x)` çalışıyor; başa yazılan ve tek parametreli fonksiyon hata, ama iletiler (H2475, H2476) eski sondaki blok yazımını öneriyor; 3A düzeltiyor [ölçüldü: DENETİM].
- **Yapılacaklar:** iletilerin testi 3. katman birleşince.

### 13.8

- **Mevcut durum:** `TR_json_parse` `TR_Dict` veriyor; `TR_json_string({1: 2})` anahtarı sessizce metne çeviriyor (`{"1":2}`); CSV hücresi sayıya benziyorsa sayı (`[["007", 7]]`) [ölçüldü].
- **Yapılacaklar:** `TR_json_parse` `TR_JSON` verir (3.7a); metin olmayan anahtarın hatası; CSV hücresi hep metin (göç: `.tr`'de 10 kullanım).

### 13.9

- **Mevcut durum:** katalog 188 fonksiyon ve metottan 138 Go yerleşiğinin imzasını tutuyor; 50'si TR ile yazılmış kitaplıkta (29 fonksiyon, 21 metot), imzası kaynakta. Tipler öğesiz (`TR_List`, `TR_Dict`), fonksiyon tipi tek bir `TR_Function`, yerleşiğin tipi `TR_BuiltinFunction` (11 yerde), genel tip `TR_Any` 24 yerde [ölçüldü: DENETİM].
- **Yapılacaklar:** imzalar öğe ve fonksiyon tipleriyle; `TR_BuiltinFunction` ve `TR_Any` kalkar; Go testi (4a).

### 13.10

- **Mevcut durum:** 35 yerleşiğin dönüş tipi birleşim (`TR_abs`, `TR_floor`, `TR_exp` gibi), `TR_tolist`'in özelliği de; dönüşü argümanın değerine bağlı üç yer: `TR_tolist` (tensörün boyutu), `TR_decode` (toplu giriş), `TR_parameters` (`TR_named:`); 9 yerleşik değişken sayıda argüman alıyor; yalnız 7 metodun alıcıya göre ayrı imzası var [ölçüldü: DENETİM].
- **Yapılacaklar:** kısıtlı tip parametreleri, ayrık imzalar, değere bağlı dönüşlerin sabit dönüşlü yerleşiklere ayrılması (15 ve 16. bölümlerle), değişken sayıda argümanın yazımı (4a).

### 13.11

- **Mevcut durum:** yanlış tip ve sayı çalışma anında, iletiyle: "sqrt() expects Int or Float as argument 1, String (\"a\") given", "sqrt() takes 1 argument, 2 given (usage: sqrt(number))" [ölçüldü].
- **Yapılacaklar:** denetleyicide aynı kodla (4a).
