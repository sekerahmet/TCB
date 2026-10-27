# Durum

Dil tanımının yeniden yazılan bölümlerine göre bugünkü TR'nin durumu ve kurala varmak için yapılacaklar. Her maddenin kuralı dil tanımındadır ([`docs/spec/`](spec/README.md)); yapılan iş testiyle birlikte buradan silinir. İşlerin sırası [yol haritasındadır](roadmap.md). Bu belgede kimlikler dil tanımının yazımıyla anılır (`TR_if`, 2.3a); bugünkü kod onları sınıflı yazımla tanıyor (TR_kw_if biçimi).

## 1. Program

Kurallar: [`docs/spec/01_program.md`](spec/01_program.md). Kararlar DENETİM'den ve Gemini'nin araştırmasından geçti (2026-10-27).

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

- **Mevcut durum:**
  - Çalıştırmada modülün hatası `TR_import` satırında çıkıyor, aktaranın önceki satırları çalışmış oluyor (11.4).
  - `TR_cmd_check` modülü geçiriyor, çalıştırma reddediyor; iki ayrı yol var.
  - `TR_cmd_debug` tip denetimini hiç yapmıyor, hatalı programın satırları çalışıyor [ölçüldü, DENETİM].
  - Takma adsız `TR_import` olan kapsamda tanımsız adlar çalışmadan önce aranmıyor (5.14).
  - `TR_import` her yerde yazılabiliyor ve adları bulunduğu bloğa getiriyor [ölçüldü].
- **Yapılacaklar:**
  - Çalıştırma, `TR_cmd_check`'in yolunu kullanarak bütün modülleri önce denetler; ikinci yol silinir.
  - `TR_cmd_debug` de denetler.
  - `TR_import` yalnız dosyanın başında yazılır.
  - Takma adsız aktarmada tanımsız ad çalışmadan önce bulunur.

### 1.4

- **Mevcut durum:**
  - Argüman okunmadan program biterse bitişte uyarı yazılıyor, çıkış kodu değişmiyor.
  - Komut satırı çözümleyicisi `--ros-args`'ı bilinmeyen seçenek sayıyor.
- **Yapılacaklar:**
  - Argüman verilip programın en dışından ulaşılabilen bir `TR_command_line` çağrısı yoksa program çalışmadan kullanım hatası verir (2); ulaşılabilirlik kesin değilse çağrı var sayılır.
  - Böyle bir programa yalnız yardım seçeneği (`-h`) verilirse "seçeneği yok" yazılır, çıkış 0 (yeni ileti kodu).
  - Bitişteki uyarı silinir.
  - ROS 2'nin `--ros-args` blokları ve dağıtık eğitim başlatıcılarının `--local-rank=N`, `--local_rank=N` argümanı programın argümanlarından ayrılır.

### 1.5

- **Mevcut durum:**
  - Süreç temizlikten sonra 3 koduyla çıkıyor.
  - Çalışan her yerleşik önce bitiyor; `TR_input` satır beklerken ilk istek beklemeyi kesmiyor.
  - Ağ yerleşiği yok; TR alt süreç açmıyor (19.1).
  - SIGUSR1'in bugün ne yaptığı ölçülmedi.
- **Yapılacaklar:**
  - Süreç isteğin kendi sinyaliyle biter; SIGUSR1 de durdurma isteği olur.
  - Durum değiştirmeyen yerleşik ve bekleme bırakılır, önce `TR_input` ve dosyadan yükleme; bırakılan hesabın GPU'da başlamış işi beklenir, ertelenen işler sonra çalışır.
  - Yığın çözülürken ertelenen işlerdeki beklemeler bırakılmaz (bugün `interruptibleSleep` kesmede hemen dönüyor [kanıtlı, DENETİM]).
  - Durum değiştirenlerin listesi yazılır ve sınanır: `TR_backward`, iyileştirici adımı, indeksle atama, `TR_write_slice`, dosyaya yazma, üretecin ve yükleyicinin ilerlemesi.
  - TR alt süreç açmaya başlarsa (19.1) bu madde ona göre genişler: istek alt sürece, kendisi zaten almadıysa iletilir.

### 1.6

- **Mevcut durum:**
  - `TR_handle_stop` ve `TR_stop_requested` yok (adları onaylı).
  - Kontrol noktası kuruluyken bekleme uyanmıyor.
  - Son kayıttan sonra gelen istek kayboluyor, çıkış 0 [ölçüldü, DENETİM].
  - Ertelenen durdurma bayrağı temizlenmiyor, `.TR_save()` iki kez kaydediyor [ölçüldü, DENETİM].
- **Yapılacaklar:**
  - İki yerleşik; "durdurma istendi" hatası için ileti kodu.
  - Üstlenen programda yeni bir şey bekleyen yerleşik, `TR_sleep` dahil, bu hatayla döner.
  - `TR_defer` bloklarında ve bu hatayı yakalayan `TR_catch` gövdesinde beklemeler normal çalışır.
  - Kontrol noktası isteği yalnız kaydeder, beklemeleri hataya çevirmez; istekten sonraki ilk kayıtta durur, bitişte kaydetmez.
  - Program `TR_handle_stop` ile de üstlendiyse kontrol noktası yalnız kaydeder, durmayı programa bırakır.
  - Erteleme süreyle sınırlı: son iki `.TR_update` arasındaki sürenin iki katı içinde kayda varılmazsa ya da istek ikinci güncellemeden önce geldiyse istek 1.5'teki gibi uygulanır.

### 1.7

- **Mevcut durum:** ikinci istekte süreç sinyalle değil, 130 koduyla çıkıyor.
- **Yapılacaklar:** ikinci istekte süreç SIGINT ile biter.

### 1.8

- **Mevcut durum:** kuralın uygulanacak bir yanı yok; süre sınırı süreci başlatanındır.
- **Yapılacaklar:** yok.

### 1.9

- **Mevcut durum:**
  - Durdurmada 3 ve 130 kodlarıyla çıkılıyor.
  - `tr` aracının kendi kullanım hatası 1 veriyor.
  - Bir görevdeki yakalanmayan hata yazılıyor ama çıkış 0 [ölçüldü, DENETİM].
  - Program sona gelince önce görevleri bekliyor, ertelenen işler sonra çalışıyor [ölçüldü, DENETİM]; kuralla aynı.
- **Yapılacaklar:**
  - İstek geldiyse süreç isteğin sinyaliyle biter (1.5, 1.7 ile); durdurmayı üstlenen program sona gelirse sinyal, `TR_exit(kod)` ile o kod.
  - Görevdeki yakalanmayan hata olduysa çıkış 1.
  - `tr` aracının kullanım hatası 2 verir.

### 1.10

- **Mevcut durum:** `TR_exit` yok; programın en dışındaki `TR_return` çalışıyor (1.14'teki ölçüm).
- **Yapılacaklar:** yerleşik eklenir: nazik çıkış, `TR_defer` çalışır, `TR_catch` yakalamaz, görevden çağrılınca bütün program biter.
  - Programın en dışındaki `TR_return` sözdizimi hatası olur; depodaki kullanımlar `TR_exit`'e çevrilir.

### 1.11

- **Mevcut durum:** `TR_exit` olmadığı için yok.
- **Yapılacaklar:** 0–125 dışındaki kod çalışma hatasıdır.

### 1.12

- **Mevcut durum:** `TR_exit` olmadığı için yok.
- **Yapılacaklar:** `TR_cmd_test` altında `TR_exit` testi düşürür, öbür testler sürer.

### 1.13

- **Mevcut durum:** `TR_exit` olmadığı için yok.
- **Yapılacaklar:** ertelenen işten çağrılan `TR_exit` kodu değiştirir; durdurmayı üstlenmemiş programa istek geldiyse sinyal korunur.

### 1.14

- **Mevcut durum:**
  - Program sona gelince ve en dıştaki `TR_return` ile ertelenen işin hatası yazılıyor, çıkış 1 [ölçüldü].
  - Komut satırı çözümleyicisinin yardım ve kullanım hatası çıkışları ertelenen işin hatasını yutuyor, çıkış 0 ya da 2 [ölçüldü, DENETİM].
- **Yapılacaklar:**
  - Komut satırı çözümleyicisinin çıkışlarında da hata yazılır, çıkış 1.
  - `TR_exit` ve durdurma kısımları 1.10 ve 1.5 ile.

### 1.15

- **Mevcut durum:** ölçülmedi.
- **Yapılacaklar:** önce ölçülür; bitişte gelen ilk Ctrl+C ertelenen işleri kesmez, işler bitince süreç SIGINT ile biter.

## 2. Sözcük yapısı

Kurallar: [`docs/spec/02_sozcuk_yapisi.md`](spec/02_sozcuk_yapisi.md). DENETİM'den (üç tur) ve Gemini'nin araştırmasından geçti (2026-10-27). Sonradan değişen ve henüz denetlenmeyenler: yorumun `#`, başlığın `# TR 4` olması, 2.1a'nın `TR_` iskelet kuralı, 2.3b'de yerel sabitin küçük harfle yazılması. Eski 2.9a (her dilde tek yazım) ve 2.11 (diller arası ad kuralı, $ad kaçışı) gösterim dilleri bölümüne gitti; o numaralar boş kalır. `tests/` altındaki 2.9a, 2.11 ve 2.12 numaralı testler, testler koşulmadan önce taşınır ya da silinir; yoksa her test numarasını bir kurala bağlayan kapı kalır. Kapılar da testlerle birlikte düzelir: `TestEveryRuleHasATest` harfli kural örneği olarak 2.9a'yı sabit arıyor (yerine 2.1a); kimlik kapısı `TR_` ile başlayan sözcükleri hiç denetlemiyor, TR_ ile yazılan her sözcüğü yerine göre bir kimliğe çözmeli (onaylı yeni adlar ve bilerek yanlış örnekler bir listede); gerekçe dosyalarının düzenini ve aynı yerde çakışan kimlik olmamasını koruyan test yok. Eski 2.12 (çıplak ileti kodu) kalktı. Anlamı değişmeyen maddelerin bugünkü TR testi var; aşağıda yalnız değişen ya da eksik olanlar.

### 2.1

- **Mevcut durum:** başlığı olmayan metin bugün Türkçe yazılmış sayılıp okunuyor. Başlık bugün eski biçimde (tr kodlu 3 dil=tr): bir Türkçe sözcük ve dosyanın dilini taşıyor. Programın sonucu bu dile bağlı: `TR_print(TR_true)` dil=tr ile "doğru", dil=en ile "true" yazdırıyor [ölçüldü].
- **Yapılacaklar:** diskteki her dosya kodludur; başlıksız dosya çalışmaz, ileti kodlama yolunu söyler. Bilerek bozuk sınav dosyaları, belgelerdeki kod blokları ve Go testlerindeki TR metni kodlu biçime geçer.
  - Başlık `# TR 4`; bugün tanınmıyor, dosya başlıksız sayılıp hata veriyor [ölçüldü]. 1. ve 2. bölümün kod blokları yeni başlıkla yazıldı; başlık uygulanana kadar belgelerdeki kod bloklarını çalıştıran testler kalır. Dosyanın dili kalkınca programın sonucu hiçbir dile bağlı olmaz (değer metni ve hata metni ilgili bölümlerde).
  - İlk satırdaki `#!` yok sayılır; bugün sözdizimi hatası veriyor [ölçüldü].

### 2.1a

- **Mevcut durum:** Yunan ve Latin harfli `θ_eski` "farklı yazı sistemlerinin harfleri karışık" hatası veriyor [ölçüldü].
- **Yapılacaklar:** dosyanın ve aktardığı adların arasında UTS #39 iskeleti aynı iki ad hata; iskeleti `TR_` ile başlayan kullanıcı adı hata (bugün `ТR_print` karışık yazı sistemi hatası veriyor [ölçüldü, DENETİM]); tek bir addaki karışık yazı sistemi uyarı.

### 2.2

- **Mevcut durum:** yorum `//` ve `/* */` ile yazılıyor, iç içe yorum hata veriyor; `#` "TR'de yorum değil" hatası veriyor [ölçüldü].
- **Yapılacaklar:** yorum `#` ile; `//` tam bölme (4.2a); `/* */` kalkar; satır başındaki `//` ve `/*` iletisi `#`'i önerir. Depodaki bütün kodlu dosyalar ve belgelerdeki kod blokları çevrilir.

### 2.3

- **Mevcut durum:** adlar karşılaştırılmadan önce normalleştirilmiyor (sözcük çözümleyicide normalleştirme yok).
- **Yapılacaklar:** adlar NFC'ye göre karşılaştırılır; kodlama ve gösterim de aynı biçimi kullanır.

### 2.3 (ad kuralı)

- **Mevcut durum:** adın harf, rakam ve `_` ile yazıldığı bugünkü kural birleşen işaretleri kapsıyor mu ölçülmedi.
- **Yapılacaklar:** UAX #31 (XID_Start, XID_Continue) ile okunur.

### 2.3a

- **Mevcut durum:** kimlikler sınıflı yazılıyor (TR_kw_if, TR_fn_print). Listede olmayan kimlik hata veriyor ama en yakın kimliği önermiyor: TR_fn_prnt ve x.TR_fn_map için ileti yalnız "bilinen bir kimlik değil" diyor [ölçüldü].
- **Yapılacaklar:** program metninde sınıfsız yazım (`TR_print`, `x.TR_map`); sınıfı yer belirler. Kimlik çizelgesi yerle birlikte okunur; aynı ad iki yerde aynı kavramsa (argüman ve üye olarak lr) çeviri adları birleştirilir. Depodaki bütün kodlu dosyalar ve belgeler yeni yazıma çevrilir.
  - İleti, ad başka bir yerde geçerliyse doğru yeri, değilse o yerde yazımca en yakın kimliği önerir; uzaklık `TR_`'den sonraki adda ölçülür, komşu harflerin yer değiştirmesi tek değişikliktir.

### 2.3b

- **Mevcut durum:** her sabit büyük harf istiyor: fonksiyonun içindeki küçük harfli sabit de ad yazılışı uyarısı veriyor [ölçüldü].
- **Yapılacaklar:** büyük harf yalnız dosyanın en dışındaki sabitte; yerel sabit küçük harfle yazılır. Depodaki yerel sabitler çevrilir.

### 2.4

- **Mevcut durum:** `0x1F` sözdizimi hatası veriyor [ölçüldü]; aralığı aşan tam sayı yazımı hata veriyor, ileti ondalık yazımı öneriyor [ölçüldü], kuralla aynı.
- **Yapılacaklar:** `0x1F`, `0o17`, `0b1010` biçimindeki ön ekler ve ön ekten sonra `_`.
  - `0X1F` bugün genel bir sözdizimi hatası veriyor ("')' bekleniyordu"), küçük harfli ön eki önermiyor [ölçüldü]; ileti `0x1F`'yi önerir.

### 2.6

- **Mevcut durum:** `\u{...}` bilinmeyen kaçış dizisi hatası veriyor [ölçüldü]. Bilinen kaçışlar arasında `\'`, `\{` ve `\}` var.
- **Yapılacaklar:** `\u{...}` kaçışı; bilinmeyen kaçış iletisi bilinenlere onu da ekler.
  - Tek tırnaklı metin kalkar, iletisi çift tırnağı önerir; depodaki kullanımlar çift tırnağa çevrilir.
  - `\{` ve `\}` kaçışları kalkar (2.7).

### 2.7

- **Mevcut durum:** yerleştirme `{x}` ile yapılıyor; `"a \(1 + 2) b"` bilinmeyen kaçış dizisi hatası veriyor [ölçüldü].
- **Yapılacaklar:** yerleştirme `\(x)` ile; `{` metinde sıradan karakter. Depodaki metinler çevrilir: `{x}` → `\(x)`, `\{` ve `\}` → `{` ve `}`.

### 2.9

- **Mevcut durum:** `TR_while` ve `TR_default` çalışıyor [ölçüldü].
- **Yapılacaklar:** ikisi kalkar; koşullu döngü `TR_for` ve bir koşulla, varsayılan kol `_` ile yazılır; depodaki kullanımlar çevrilir.
  - İkisi yazılırsa ileti TR'deki yazımı söyler.

### 2.12 (kalktı)

- **Mevcut durum:** `H6903` gibi çıplak ileti kodu metin değeri olarak okunuyor.
- **Yapılacaklar:** bu sözcük sınıfı kalkar; ileti kodu metin olarak yazılır (`"H6903"`); `H6903` sıradan bir ad olur.

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
- **Yapılacaklar:** `T?` tipi; açılmamış kullanım denetim hatası; daraltma yalnız değişmeyen yerel değişkende. `.TR_get` isteğe bağlı tip döner. Değer döndürmeyen çağrı değer olarak kullanılamaz; isteğe bağlı olmayanı `TR_none`'la karşılaştırmak denetim hatası.

### 3.6

- **Mevcut durum:** karışık tipli liste çalışıyor [ölçüldü].
- **Yapılacaklar:** `TR_List[T]`; öğelerin tipi aynı, boş liste tipini yazar.

### 3.7

- **Mevcut durum:** `{k: 1}` anahtarı "k" metni sayıyor, `d.k` anahtarı okuyor, olmayan anahtar `TR_none` veriyor, ondalık anahtar çalışıyor [ölçüldü].
- **Yapılacaklar:** `TR_Dict[K, V]`; değişmezdeki anahtar bir ifade, erişim yalnız köşeli parantezle, olmayan anahtar hata, anahtar yalnız `TR_Int`, `TR_String`, `TR_Bool`. Depodaki kullanımlar çevrilir.

### 3.7a

- **Mevcut durum:** JSON çözülünce sözlük ve liste oluyor, tip denetimi yok; genel "her şey" tipi (Any) kimlik listesinde var.
- **Yapılacaklar:** `TR_JSON`; çözme ve `TR_JSON` beklenen değişmez onu kurar; tür `TR_Type(j)` ile sorulur, asıl değere tip çağrısıyla geçilir; sayının tam ya da ondalık yazımı korunur, `TR_Int`'e sığmayan tam sayının türü `TR_Float`; sonsuz ve `TR_nan` `TR_JSON`'a giremez; `.TR_keys()` yalnız nesnede. Bugün çözme Any döndürüyor; Any, Object ve StructType gibi genel tipler kalkar ya da yapılar bölümünde yeniden tanımlanır. Genel "her şey" tipi kalkar; depodaki kullanımlar `TR_JSON`'a ya da asıl tipe çevrilir.

### 3.8

- **Mevcut durum:** `3..0` geriye sayıyor (3, 2, 1) [ölçüldü].
- **Yapılacaklar:** başlangıç bitişten büyük ya da eşitse aralık boş; `.TR_reverse()` aralıkta da çalışır.

### 3.10

- **Mevcut durum:** doğruluk sınaması var: boş liste koşulda yanlış sayılıyor [ölçüldü].
- **Yapılacaklar:** koşulda yalnız `TR_Bool`; başka tip denetim hatası, ileti açık sınamayı önerir.

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

Kurallar: [`docs/spec/04_operatorler.md`](spec/04_operatorler.md). Taslak: DENETİM'den geçti, bulguları işlendi (2026-10-27); Gemini'nin araştırması bekleniyor. `tests/` altındaki 4. bölüm testleri eski kuralları da sınıyor (`/`'nin değere göre tipi, metne her değerin eklenmesi, boru işleci); testler koşulmadan önce yeni kurallara göre yeniden yazılır.

### 4.1

- **Mevcut durum:** `??` en gevşek düzeyde, `&&` ve `||` sözcüklerle aynı düzeyde; `==` ile `<` ayrı düzeylerde.
- **Yapılacaklar:** yeni öncelik tablosu; karşılaştırmalar tek düzeyde, zincir 4.4'teki gibi; `**` sağındaki tekli işleçten gevşek bağlar.

### 4.2

- **Mevcut durum:** `7 / 2` 3.5, `6 / 3` 2 veriyor: sonucun tipi değere bağlı [ölçüldü].
- **Yapılacaklar:** `/` her zaman ondalık verir.

### 4.2a

- **Mevcut durum:** tam bölme yok; `-7 % 3` −1 veriyor (kesen kalan) [ölçüldü].
- **Yapılacaklar:** `//` tam bölme (aşağı yuvarlar); `%`'nin işareti bölenin işareti.

### 4.3

- **Mevcut durum:** `2 ** -1` 0.5 veriyor; `0.0 ** -1` sonsuz, `(-8.0) ** (1.0 / 3)` NaN veriyor [ölçüldü].
- **Yapılacaklar:** iki tam sayıda eksi üs çalışma hatası, ileti ondalık yazımı önerir; sıfırın eksi üssü ve eksi sayının tam olmayan üssü çalışma hatası.

### 4.4

- **Mevcut durum:** metinler karşılaştırılamıyor ("Metin ve Metin tipleri arasında < işlemi desteklenmiyor"); `0 <= i < 5` C'deki gibi okunuyor ve çalışırken "Mantık ve Tam tipleri arasında" hatası veriyor [ölçüldü].
- **Yapılacaklar:** metinler kod noktası sırasıyla karşılaştırılır; sıralama işleçlerinin zinciri Python'daki gibi, ortadaki ifade bir kez hesaplanır; `==` ya da `!=` içeren zincir sözdizimi hatası.

### 4.5

- **Mevcut durum:** ayrı tipler sessizce eşit değil sayılıyor; fonksiyonlar kimlikle karşılaştırılıyor.
- **Yapılacaklar:** ayrı tipler denetim hatası; fonksiyonlar karşılaştırılamaz.

### 4.6

- **Mevcut durum:** işlenen her tipten olabiliyor: `1 && "a"` doğru veriyor [ölçüldü].
- **Yapılacaklar:** işlenen yalnız mantık değeri.

### 4.7

- **Mevcut durum:** isteğe bağlı tip olmadığı için sol yan her tipten olabiliyor: `1 ?? 2` 1 veriyor [ölçüldü].
- **Yapılacaklar:** sol yan isteğe bağlı değilse denetim hatası.

### 4.9

- **Mevcut durum:** `&&`, `||`, `!` ve `|>` çalışıyor; boru `[1, 2, 3] |> TR_sum` 6 veriyor [ölçüldü].
- **Yapılacaklar:** dördü sözdizimi hatası, ileti TR'deki yazımı söyler; C ailesinin yorumları hata verir, ileti `#`'i önerir; `//`'nin sağındaki tanımsız adın iletisi de `#`'i önerir. Depodaki kullanımlar çevrilir.
  - Döngü dışındaki `TR_in` bugün genel bir sözdizimi hatası veriyor ("')' bekleniyordu") [ölçüldü]; ileti `.TR_contains`'i önerir.

### 4.10

- **Mevcut durum:** `"a" + 1` "a1" veriyor; `[0] * 3` ve `"-" * 3` çalışırken "desteklenmiyor" hatası veriyor [ölçüldü].
- **Yapılacaklar:** başka tiple `+` denetim hatası, ileti yerleştirmeyi önerir; metin, bayt dizisi ve listenin `*` ile yinelenmesi; öğeleri başvuruyla paylaşılan listenin yinelenmesi denetim hatası.

### 4.12

- **Mevcut durum:** bit işleçleri yok: `5 & 3` sözdizimi hatası veriyor [ölçüldü].
- **Yapılacaklar:** `&`, `|`, `^`, `~`, `<<`, `>>` tam sayıda.

## 5. Deyimler ve kontrol yapıları

Kurallar: [`docs/spec/05_deyimler.md`](spec/05_deyimler.md). Taslak: DENETİM'den geçti, bulguları işlendi (2026-10-27); Gemini'nin araştırması bekleniyor. `tests/deyimler_test.tr` eski kuralları da sınıyor; testler koşulmadan önce yeni kurallara göre yeniden yazılır [DENETİM'in listesi]:
- 5.1: `:=` tanımı, ilk değersiz değişken;
- 5.2: yerel büyük harfli sabit (2.3b);
- 5.4: tam sayı değişkene `/=`;
- 5.5: `TR_else`'siz `TR_if` ifadesinin yokluk vermesi;
- 5.6: `TR_default`, kapsamayan iki eşleme, düz adla yazılmış değişken deseni (`(x)`'e çevrilir);
- 5.7: `{i}{x}` yerleştirmesi (2.7), `TR_fn_str` (3.12), geriye sayan `3..0` (3.8), tipsiz boş listeler (3.6);
- 5.8: `TR_while`;
- 5.12: kolda dıştaki fonksiyonla aynı adlı fonksiyon (yeni kurala göre hata); adlı fonksiyon ifadesinin adının dışarı çıkması (6. bölümün kararı);
- 5.13: kapsamayan eşleme;
- her yerde: tipsiz parametreler (1.2) ve `//` yorumları (2.2).

`TR_elif` bilerek kimliği olmayan addır; kimlik kapısı düzelince bilerek yanlış örnekler listesine girer (2. bölümün girişi). Yeni denetim hataları (5.1, 5.5, 5.6, 5.12, 5.14) uygulanırken depodaki kodlu dosyalar da düzeltilir; yoksa `TestRepoTRFilesPassCheck` kapısı kalır. Anlamı değişmeyen maddelerin bugünkü TR testi var: sabite atama denetim hatası, fonksiyonun içindeki `TR_break` sözdizimi hatası veriyor, takas çalışıyor [ölçüldü]. Koşullu döngünün `TR_while`'sız yazımı 2.9'da.

### 5.1

- **Mevcut durum:** `x := 5` çalışıyor; ilk değersiz `TR_var y` yokluk alıyor; hiç yeniden atanmayan değişken uyarı vermiyor [ölçüldü].
- **Yapılacaklar:** ilk değer zorunlu; `:=` sözdizimi hatası, ileti `TR_var`'ı önerir; yeniden atanmayan yerel `TR_var` uyarısı `TR_const`'u önerir. Depodaki `:=` kullanımları çevrilir.

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
