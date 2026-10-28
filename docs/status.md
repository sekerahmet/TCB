# Durum

Dil tanımının yeniden yazılan bölümlerine göre bugünkü TR'nin durumu ve kurala varmak için yapılacaklar. Her maddenin kuralı dil tanımındadır ([`docs/spec/`](spec/README.md)); yapılan iş testiyle birlikte buradan silinir. İşlerin sırası [yol haritasındadır](roadmap.md). Bu belgede kimlikler dil tanımının yazımıyla anılır (`TR_if`, 2.3a).

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

- **Mevcut durum:** sözdizimi hataları yalnız metin olarak veriliyor; komut satırının yardımında JSON yalnız hız raporu için var [ölçüldü].
- **Yapılacaklar:** çalışmadan önceki bütün hata ve uyarılar (sözdizimi, tip, kullanım) JSON olarak da verilir: başlangıç ve bitiş konumu, ilgili konumlar, önerilen düzeltme ve uygulanabilirlik işaretiyle. Seçenek ve alanlar sonraki bölümlerde tanımlanınca.

## 3. Tipler ve değerler

Kurallar: [`docs/spec/03_tipler.md`](spec/03_tipler.md). Taslak: DENETİM'den geçti, bulguları işlendi (2026-10-27); Gemini'nin araştırması bekleniyor. TR bugün dinamik tipli; tip açıklamalarını yalnız çalışmadan önceki denetleyici kullanıyor. `tests/` altındaki 3. bölüm testleri eski kuralları da sınıyor (karışık tipli liste, doğruluk sınaması, çıplak ad anahtarı, açıklamasız değişkenin tipi); testler koşulmadan önce yeni kurallara göre yeniden yazılır. Eski 3.14 (tip adının tek yazımı) 2.3a'ya düştü; eski 3.15 (katalog iletisinin tipi) hata bölümüne gider, `tests/` altındaki 3.15 testi o bölüm yazılınca taşınır.

### 3.2

- **Mevcut durum:** 1e6 ve üstü bilimsel yazılıyor ("1e+06", 1234567.0 için "1.234567e+06") [ölçüldü].
- **Yapılacaklar:** bilimsel gösterim üs −4'ün altında ve 16'dan itibaren; üs işaretli ve en az iki rakamlı.
  - `TR_inf` ve `TR_nan` bugün program metninde yazılamıyor: değer adı sınıflı kimlikle tanımsız ad hatası veriyor [ölçüldü, DENETİM].

### 3.3a

- **Mevcut durum:** bayt dizisi tipi yok (kimlik listesinde yok).
- **Yapılacaklar:** `TR_Bytes`: değiştirilemez; metinden UTF-8 ile ve sayılardan kurulur; dosya ve ağ okumaları bayt da verebilir.

### 3.5

- **Mevcut durum:** isteğe bağlı tip yok; `TR_Int` açıklamalı bir değişkene `TR_none` atanabiliyor [ölçüldü].
- **Yapılacaklar:** `T?` tipi; açılmamış kullanım denetim hatası; daraltma yalnız değişmeyen yerel değişkende: sınamanın dalında, `TR_else` dalında, `TR_and`/`TR_or`'un sağ yanında, her yolda biten erken çıkıştan sonra ve asıl tipten atamayla; alana atama daraltmayı bozmaz. `.TR_get` isteğe bağlı tip döner. Değer döndürmeyen çağrı değer olarak kullanılamaz; isteğe bağlı olmayanı `TR_none`'la karşılaştırmak denetim hatası.

### 3.6

- **Mevcut durum:** karışık tipli liste çalışıyor [ölçüldü].
- **Yapılacaklar:** `TR_List[T]`; öğelerin tipi aynı, boş liste tipini yazar; `TR_none` içeren değişmez `TR_List[T?]`.

### 3.7

- **Mevcut durum:** `{k: 1}` anahtarı "k" metni sayıyor, `d.k` anahtarı okuyor, olmayan anahtar `TR_none` veriyor, ondalık anahtar çalışıyor [ölçüldü].
- **Yapılacaklar:** `TR_Dict[K, V]`; değişmezdeki anahtar bir ifade, erişim yalnız köşeli parantezle, olmayan anahtar hata, anahtar yalnız `TR_Int`, `TR_String`, `TR_Bool`, `TR_Bytes` ve alanları anahtar olabilen durumlu tip. Depodaki kullanımlar çevrilir.

### 3.7a

- **Mevcut durum:** JSON çözülünce sözlük ve liste oluyor, tip denetimi yok; genel "her şey" tipi (Any) kimlik listesinde var.
- **Yapılacaklar:** `TR_JSON`; çözme ve `TR_JSON` beklenen değişmez onu kurar; tür `TR_Type(j)` ile sorulur, asıl değere tip çağrısıyla geçilir; sayının tam ya da ondalık yazımı korunur, `TR_Int`'e sığmayan tam sayının türü `TR_Float`; sonsuz ve `TR_nan` `TR_JSON`'a giremez; `.TR_keys()` yalnız nesnede; değer metni metin türünde metnin kendisi (yazdırma ve yerleştirme); `TR_Type(j) == TR_JSON` denetim hatası. Bugün çözme Any döndürüyor; Any, Object ve StructType gibi genel tipler kalkar ya da yapılar bölümünde yeniden tanımlanır. Genel "her şey" tipi kalkar; depodaki kullanımlar `TR_JSON`'a ya da asıl tipe çevrilir.

### 3.8

- **Mevcut durum:** `3..0` geriye sayıyor (3, 2, 1) [ölçüldü].
- **Yapılacaklar:** başlangıç bitişten büyük ya da eşitse aralık boş; `.TR_reverse()` aralıkta da çalışır.

### 3.10

- **Mevcut durum:** koşulda yalnız `TR_Bool`; başka tip çalışma anında hata veriyor (H2600), ileti açık sınamayı öneriyor (katman 2) [ölçüldü]. Hata ayıklayıcının koşullu durağı hâlâ doğruluk sınamasıyla (`IsTruthy`) okunuyor.
- **Yapılacaklar:** başka tip denetim hatası da olur (katman 4); hata ayıklayıcının koşulu da yalnız `TR_Bool` alır.

### 3.12

- **Mevcut durum:** dönüşüm ve kurma tipten ayrı fonksiyonlarla yapılıyor; 13 çift aynı yerde yalnız büyük-küçük harfle ayrılıyor: 12 tip ile fonksiyon çifti (Int/int, Float/float, Tensor/tensor, Checkpoint/checkpoint, Dataset/dataset, Type/type gibi), yokluğun tipi ile değeri de (None/none).
- **Yapılacaklar:** tip çağrılınca kurar ve dönüştürür; ayrı fonksiyonlar kalkar (dönüşümde metin fonksiyonu da). Yokluğun tipi `TR_NoneType` olur. Metin okuma 2.4 ve 2.5'in yazımıyla, önde isteğe bağlı işaretle, yerel ayarsız; mantık değeri yalnız iki metinden. Depodaki kullanımlar çevrilir.

### 3.13

- **Mevcut durum:** `0` ile başlayan değişkene ondalık eklenebiliyor [ölçüldü]; açıklamasız değişkenin tipi kalıcı değil.
- **Yapılacaklar:** statik tip (1.2); kalıcı yerel tip; tam sayı yazımı ondalık beklenen yerde ondalık.

### 3.14

- **Mevcut durum:** değerin metni dosyanın dilinde: Türkçe dosyada tip "Tam", yokluk "yok", sonsuz "sonsuz" yazılıyor [ölçüldü].
- **Yapılacaklar:** değerin metni dilden bağımsız, kimliğin ad kısmıyla.

## 4. İşleçler

Kurallar: [`docs/spec/04_operatorler.md`](spec/04_operatorler.md). Taslak: DENETİM'den geçti, bulguları işlendi (2026-10-27); Gemini'nin araştırması bekleniyor. 4.1–4.4, 4.6, 4.9, 4.11 ve 4.12'nin çalışma anı uygulandı (katman 2, DENETİM'den geçti, 2026-10-28); aşağıda yalnız kalanlar. Tip isteyen denetim hataları katman 4'tedir.

### 4.5

- **Mevcut durum:** ayrı tipler sessizce eşit değil sayılıyor; fonksiyonlar yalnız üst düzey `==` ve `!=`'de hata veriyor (H2613), fonksiyon içeren liste kimlikle karşılaştırılıyor (`[f] == [f]` doğru) [ölçüldü].
- **Yapılacaklar:** ayrı tipler denetim hatası; fonksiyon içeren değer de karşılaştırılamaz (denetim hatası).

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
- 5.7: `{i}{x}` yerleştirmesi (2.7), `TR_str` (3.12), geriye sayan `3..0` (3.8), tipsiz boş listeler (3.6);
- 5.8: `TR_while`;
- 5.12: kolda dıştaki fonksiyonla aynı adlı fonksiyon (yeni kurala göre hata); adlı fonksiyon ifadesinin adının dışarı çıkması (6. bölümün kararı);
- 5.13: kapsamayan eşleme;
- her yerde: tipsiz parametreler (1.2) ve `//` yorumları (2.2).

`TR_elif` bilerek kimliği olmayan addır; kimlik kapısı düzelince bilerek yanlış örnekler listesine girer (2. bölümün girişi). Yeni denetim hataları (5.1, 5.5, 5.6, 5.12, 5.14) uygulanırken depodaki kodlu dosyalar da düzeltilir; yoksa `TestRepoTRFilesPassCheck` kapısı kalır. Anlamı değişmeyen maddelerin bugünkü TR testi var: sabite atama denetim hatası, fonksiyonun içindeki `TR_break` sözdizimi hatası veriyor, takas çalışıyor [ölçüldü]. Koşullu döngünün `TR_while`'sız yazımı 2.9'da.

### 5.1

- **Mevcut durum:** `x := 5` çalışıyor; ilk değersiz `TR_var y` yokluk alıyor; hiç yeniden atanmayan değişken uyarı vermiyor [ölçüldü].
- **Yapılacaklar:** ilk değer zorunlu; `:=` sözdizimi hatası, ileti `TR_var`'ı önerir; fonksiyonun içinde yeniden atanmayan yerel `TR_var` uyarısı `TR_const`'u önerir (programın en dışı girmez). Depodaki `:=` kullanımları çevrilir.

### 5.3

- **Mevcut durum:** `TR_var p, q = [1, 2, 3]` hata vermiyor, ilk iki öğeyi alıyor [ölçüldü].
- **Yapılacaklar:** öğe sayısı tutmazsa çalışma hatası.

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
- 6.7 testinin adındaki `\{` (2.6), `TR_str` (3.12), `//` yorumları ve eski başlık (2.1, 2.2);
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

- **Mevcut durum:** üç yazım da çalışıyor: `x => ...`, `(x) => ...`, adsız `TR_func(x) { ... }` (bugünkü 6.6 ve 6.5 testleri).
- **Yapılacaklar:** yalnız parantezli ok; öbürleri ve `lambda` sözdizimi hatası, ileti oku önerir; `TR_func` tanımı ifade yerinde hata.

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
- eski metin biçimi ve yapının tipinin StructType olması (7.6); `TR_str` ve `TR_type` (3.12); `//` yorumları ve eski başlık.

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
- **Yapılacaklar:** `TR_enum`, tip parametresi, durumların kurulması, eşlemede desen, alan bağlama ve kapsama, koşullu kol iletisi, metin.

### 7.8

- **Mevcut durum:** arayüz yok.
- **Yapılacaklar:** `TR_interface` (en az bir metot; boş arayüz denetim hatası), yapısal karşılama, eksik metodu söyleyen ileti, tip parametresinde kısıt, `TR_Type` sınamasıyla asıl tipe daraltma. Kimlik kapısının onaylı yeni adlar listesine `TR_enum` ve `TR_interface` girer.
