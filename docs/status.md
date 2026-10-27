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

Kurallar: [`docs/spec/02_sozcuk_yapisi.md`](spec/02_sozcuk_yapisi.md). Taslak: DENETİM'den geçti (2026-10-27); Gemini'nin araştırması bekleniyor. Eski 2.9a (her dilde tek yazım) ve 2.11 (diller arası ad kuralı, $ad kaçışı) gösterim dilleri bölümüne gitti; o numaralar boş kalır. `tests/` altındaki 2.9a, 2.11 ve 2.12 numaralı testler, testler koşulmadan önce taşınır ya da silinir; yoksa her test numarasını bir kurala bağlayan kapı kalır. Eski 2.12 (çıplak ileti kodu) kalktı. Anlamı değişmeyen maddelerin bugünkü TR testi var; aşağıda yalnız değişen ya da eksik olanlar.

### 2.1

- **Mevcut durum:** başlığı olmayan metin bugün Türkçe yazılmış sayılıp okunuyor. Başlık bugün eski biçimde (tr kodlu 3 dil=tr): bir Türkçe sözcük ve dosyanın dilini taşıyor. Programın sonucu bu dile bağlı: `TR_print(TR_true)` dil=tr ile "doğru", dil=en ile "true" yazdırıyor [ölçüldü].
- **Yapılacaklar:** diskteki her dosya kodludur; başlıksız dosya çalışmaz, ileti kodlama yolunu söyler. Bilerek bozuk sınav dosyaları, belgelerdeki kod blokları ve Go testlerindeki TR metni kodlu biçime geçer.
  - Başlık `//! TR 4`; bugün tanınmıyor, dosya başlıksız sayılıp hata veriyor [ölçüldü]. 1. ve 2. bölümün kod blokları yeni başlıkla yazıldı; başlık uygulanana kadar belgelerdeki kod bloklarını çalıştıran testler kalır. Dosyanın dili kalkınca programın sonucu hiçbir dile bağlı olmaz (değer metni ve hata metni ilgili bölümlerde).
  - İlk satırdaki `#!` yok sayılır; bugün sözdizimi hatası veriyor [ölçüldü].

### 2.1a

- **Mevcut durum:** Yunan ve Latin harfli `θ_eski` "farklı yazı sistemlerinin harfleri karışık" hatası veriyor [ölçüldü].
- **Yapılacaklar:** aynı dosyada UTS #39 iskeleti aynı iki ad hata; tek bir addaki karışık yazı sistemi uyarı.

### 2.2

- **Mevcut durum:** iç içe çok satırlı yorum sözdizimi hatası veriyor [ölçüldü].
- **Yapılacaklar:** çok satırlı yorum iç içe okunur.

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

Kurallar: [`docs/spec/03_tipler.md`](spec/03_tipler.md). Taslak: DENETİM ve Gemini'nin araştırması bekleniyor. TR bugün dinamik tipli; tip açıklamalarını yalnız çalışmadan önceki denetleyici kullanıyor. `tests/` altındaki 3. bölüm testleri eski kuralları da sınıyor (karışık tipli liste, doğruluk sınaması, çıplak ad anahtarı, açıklamasız değişkenin tipi); testler koşulmadan önce yeni kurallara göre yeniden yazılır. Eski 3.14 (tip adının tek yazımı) 2.3a'ya düştü; eski 3.15 (katalog iletisinin tipi) hata bölümüne gider, `tests/` altındaki 3.15 testi o bölüm yazılınca taşınır.

### 3.2

- **Mevcut durum:** 1e6 ve üstü bilimsel yazılıyor ("1e+06", 1234567.0 için "1.234567e+06") [ölçüldü].
- **Yapılacaklar:** bilimsel gösterim üs −4'ün altında ve 16'dan itibaren.

### 3.3a

- **Mevcut durum:** bayt dizisi tipi yok (kimlik listesinde yok).
- **Yapılacaklar:** `TR_Bytes`: değiştirilemez; metinden UTF-8 ile ve sayılardan kurulur; dosya ve ağ okumaları bayt da verebilir.

### 3.5

- **Mevcut durum:** isteğe bağlı tip yok; `TR_Int` açıklamalı bir değişkene `TR_none` atanabiliyor [ölçüldü].
- **Yapılacaklar:** `T?` tipi; açılmamış kullanım denetim hatası; sınamadan sonra daraltma. `.TR_get` isteğe bağlı tip döner.

### 3.6

- **Mevcut durum:** karışık tipli liste çalışıyor [ölçüldü].
- **Yapılacaklar:** `TR_List[T]`; öğelerin tipi aynı, boş liste tipini yazar.

### 3.7

- **Mevcut durum:** `{k: 1}` anahtarı "k" metni sayıyor, `d.k` anahtarı okuyor, olmayan anahtar `TR_none` veriyor, ondalık anahtar çalışıyor [ölçüldü].
- **Yapılacaklar:** `TR_Dict[K, V]`; değişmezdeki anahtar bir ifade, erişim yalnız köşeli parantezle, olmayan anahtar hata, anahtar yalnız `TR_Int`, `TR_String`, `TR_Bool`. Depodaki kullanımlar çevrilir.

### 3.7a

- **Mevcut durum:** JSON çözülünce sözlük ve liste oluyor, tip denetimi yok; genel "her şey" tipi (Any) kimlik listesinde var.
- **Yapılacaklar:** `TR_JSON`; çözme onu verir, asıl değere tip çağrısıyla geçilir. Genel "her şey" tipi kalkar; depodaki kullanımlar `TR_JSON`'a ya da asıl tipe çevrilir.

### 3.8

- **Mevcut durum:** `3..0` geriye sayıyor (3, 2, 1) [ölçüldü].
- **Yapılacaklar:** başlangıç bitişten büyük ya da eşitse aralık boş; `.TR_reverse()` aralıkta da çalışır.

### 3.10

- **Mevcut durum:** doğruluk sınaması var: boş liste koşulda yanlış sayılıyor [ölçüldü].
- **Yapılacaklar:** koşulda yalnız `TR_Bool`; başka tip denetim hatası, ileti açık sınamayı önerir.

### 3.12

- **Mevcut durum:** dönüşüm ve kurma tipten ayrı fonksiyonlarla yapılıyor; 14 tip ile fonksiyon çifti yalnız büyük-küçük harfle ayrılıyor (Int/int, Float/float, Tensor/tensor, Checkpoint/checkpoint, Dataset/dataset, Type/type gibi), yokluğun tipi ile değeri de (None/none).
- **Yapılacaklar:** tip çağrılınca kurar ve dönüştürür; ayrı fonksiyonlar kalkar (dönüşümde metin fonksiyonu da). Yokluğun tipi `TR_NoneType` olur. Depodaki kullanımlar çevrilir.

### 3.13

- **Mevcut durum:** `0` ile başlayan değişkene ondalık eklenebiliyor [ölçüldü]; açıklamasız değişkenin tipi kalıcı değil.
- **Yapılacaklar:** statik tip (1.2); kalıcı yerel tip; tam sayı yazımı ondalık beklenen yerde ondalık.

### 3.14

- **Mevcut durum:** değerin metni dosyanın dilinde: Türkçe dosyada tip "Tam", yokluk "yok", sonsuz "sonsuz" yazılıyor [ölçüldü].
- **Yapılacaklar:** değerin metni dilden bağımsız, kimliğin ad kısmıyla.

## 4. İşleçler

Kurallar: [`docs/spec/04_operatorler.md`](spec/04_operatorler.md). Taslak: DENETİM ve Gemini'nin araştırması bekleniyor. `tests/` altındaki 4. bölüm testleri eski kuralları da sınıyor (`/`'nin değere göre tipi, metne her değerin eklenmesi, boru işleci); testler koşulmadan önce yeni kurallara göre yeniden yazılır.

### 4.1

- **Mevcut durum:** `??` en gevşek düzeyde, `&&` ve `||` sözcüklerle aynı düzeyde; `==` ile `<` ayrı düzeylerde.
- **Yapılacaklar:** yeni öncelik tablosu; karşılaştırmalar tek düzeyde ve birleşmez.

### 4.2

- **Mevcut durum:** `7 / 2` 3.5, `6 / 3` 2 veriyor: sonucun tipi değere bağlı [ölçüldü].
- **Yapılacaklar:** `/` her zaman ondalık verir.

### 4.2a

- **Mevcut durum:** tam bölme yok; `-7 % 3` −1 veriyor (kesen kalan) [ölçüldü].
- **Yapılacaklar:** `TR_div` (aşağı yuvarlar); `%`'nin işareti bölenin işareti.

### 4.3

- **Mevcut durum:** `2 ** -1` 0.5 veriyor [ölçüldü].
- **Yapılacaklar:** iki tam sayıda eksi üs çalışma hatası, ileti ondalık yazımı önerir.

### 4.4

- **Mevcut durum:** metinler karşılaştırılamıyor ("Metin ve Metin tipleri arasında < işlemi desteklenmiyor") [ölçüldü].
- **Yapılacaklar:** metinler kod noktası sırasıyla karşılaştırılır; zincir ve karışık karşılaştırma iletisi.

### 4.5

- **Mevcut durum:** ayrı tipler sessizce eşit değil sayılıyor; fonksiyonlar kimlikle karşılaştırılıyor.
- **Yapılacaklar:** ayrı tipler denetim hatası; fonksiyonlar karşılaştırılamaz.

### 4.6

- **Mevcut durum:** işlenen her tipten olabiliyor: `1 && "a"` doğru veriyor [ölçüldü].
- **Yapılacaklar:** işlenen yalnız mantık değeri.

### 4.7

- **Mevcut durum:** isteğe bağlı tip olmadığı için sol yan her tipten olabiliyor.
- **Yapılacaklar:** sol yan isteğe bağlı değilse uyarı.

### 4.9

- **Mevcut durum:** `&&`, `||`, `!` ve `|>` çalışıyor; boru `[1, 2, 3] |> TR_sum` 6 veriyor [ölçüldü].
- **Yapılacaklar:** dördü sözdizimi hatası, ileti TR'deki yazımı söyler; tek sayı ya da tek ad olan satır sonu yorumunda uyarı. Depodaki kullanımlar çevrilir.

### 4.10

- **Mevcut durum:** `"a" + 1` "a1" veriyor [ölçüldü].
- **Yapılacaklar:** başka tiple `+` denetim hatası, ileti yerleştirmeyi önerir.

### 4.12

- **Mevcut durum:** bit işleçleri yok: `5 & 3` sözdizimi hatası veriyor [ölçüldü].
- **Yapılacaklar:** `&`, `|`, `^`, `~`, `<<`, `>>` tam sayıda.
