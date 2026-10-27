# Durum

Dil tanımının yeniden yazılan bölümlerine göre bugünkü TR'nin durumu ve kurala varmak için yapılacaklar. Her maddenin kuralı dil tanımındadır ([`docs/spec/`](spec/README.md)); yapılan iş testiyle birlikte buradan silinir. İşlerin sırası [yol haritasındadır](roadmap.md).

## 1. Program

Kurallar: [`docs/spec/01_program.md`](spec/01_program.md). Kararlar DENETİM'den ve Gemini'nin araştırmasından geçti (2026-10-27).

### 1.1

- **Mevcut durum:**
  - Modülün ve test dosyasının en dışındaki deyimler de çalışıyor (11.1, 12.5).
  - Modülde ilk değer her ifadeyi kabul ediyor (5.1, 5.2).
- **Yapılacaklar:**
  - Denetleyici modülde ve test dosyasında en dışta tanım dışındaki deyimi hata sayar.
  - Modülde ilk değer sabit ifade değilse hata verir; fonksiyon adı, dışarıdan değişken yakalamayan fonksiyon ifadesi, sonucu yalnız argümanlarına bağlı sayı yerleşikleri ve `TR_fn_fuse`, `TR_fn_replay`, `TR_fn_custom_grad` serbesttir.
  - `lib/`, `tests/` ve örnekler kurala göre düzeltilir.

### 1.2

- **Mevcut durum:** TR dinamik tiplidir; tip açıklaması isteğe bağlıdır (3.13).
- **Yapılacaklar:** büyük tasarım, turlarla:
  - denetleyiciye tip çıkarımı;
  - kitaplığın, örneklerin ve testlerin imzaları;
  - "olabilir" tipi;
  - tensörün tür ve şekil bilgisinin tipe ne kadar gireceği;
  - `TR_kw_import ... TR_ctx_as m` modülü bugün `TR_type_Dict`'e bağlıyor (11.2), üyelerin tipi korunmalı; gerçek sürücü ile benzetim arasında seçim için ortak bir arayüz tipi.

### 1.3

- **Mevcut durum:**
  - Çalıştırmada modülün hatası `TR_kw_import` satırında çıkıyor, aktaranın önceki satırları çalışmış oluyor (11.4).
  - `TR_cmd_check` modülü geçiriyor, çalıştırma reddediyor; iki ayrı yol var.
  - `TR_cmd_debug` tip denetimini hiç yapmıyor, hatalı programın satırları çalışıyor [ölçüldü, DENETİM].
  - Takma adsız `TR_kw_import` olan kapsamda tanımsız adlar çalışmadan önce aranmıyor (5.14).
  - `TR_kw_import` her yerde yazılabiliyor ve adları bulunduğu bloğa getiriyor [ölçüldü].
- **Yapılacaklar:**
  - Çalıştırma, `TR_cmd_check`'in yolunu kullanarak bütün modülleri önce denetler; ikinci yol silinir.
  - `TR_cmd_debug` de denetler.
  - `TR_kw_import` yalnız dosyanın başında yazılır.
  - Takma adsız aktarmada tanımsız ad çalışmadan önce bulunur.

### 1.4

- **Mevcut durum:**
  - Argüman okunmadan program biterse bitişte uyarı yazılıyor, çıkış kodu değişmiyor.
  - Komut satırı çözümleyicisi `--ros-args`'ı bilinmeyen seçenek sayıyor.
- **Yapılacaklar:**
  - Argüman verilip programın en dışından ulaşılabilen bir `TR_fn_command_line` çağrısı yoksa program çalışmadan kullanım hatası verir (2); ulaşılabilirlik kesin değilse çağrı var sayılır.
  - Böyle bir programa yalnız yardım seçeneği (`-h`) verilirse "seçeneği yok" yazılır, çıkış 0 (yeni ileti kodu).
  - Bitişteki uyarı silinir.
  - ROS 2'nin `--ros-args` blokları ve dağıtık eğitim başlatıcılarının `--local-rank=N`, `--local_rank=N` argümanı programın argümanlarından ayrılır.

### 1.5

- **Mevcut durum:**
  - Süreç temizlikten sonra 3 koduyla çıkıyor.
  - Çalışan her yerleşik önce bitiyor; `TR_fn_input` satır beklerken ilk istek beklemeyi kesmiyor.
  - Ağ yerleşiği yok; TR alt süreç açmıyor (19.1).
  - SIGUSR1'in bugün ne yaptığı ölçülmedi.
- **Yapılacaklar:**
  - Süreç isteğin kendi sinyaliyle biter; SIGUSR1 de durdurma isteği olur.
  - Durum değiştirmeyen yerleşik ve bekleme bırakılır, önce `TR_fn_input` ve dosyadan yükleme; bırakılan hesabın GPU'da başlamış işi beklenir, ertelenen işler sonra çalışır.
  - Yığın çözülürken ertelenen işlerdeki beklemeler bırakılmaz (bugün `interruptibleSleep` kesmede hemen dönüyor [kanıtlı, DENETİM]).
  - Durum değiştirenlerin listesi yazılır ve sınanır: `TR_fn_backward`, iyileştirici adımı, indeksle atama, `TR_fn_write_slice`, dosyaya yazma, üretecin ve yükleyicinin ilerlemesi.
  - TR alt süreç açmaya başlarsa (19.1) bu madde ona göre genişler: istek alt sürece, kendisi zaten almadıysa iletilir.

### 1.6

- **Mevcut durum:**
  - `TR_fn_handle_stop` ve `TR_fn_stop_requested` yok (adları onaylı).
  - Kontrol noktası kuruluyken bekleme uyanmıyor.
  - Son kayıttan sonra gelen istek kayboluyor, çıkış 0 [ölçüldü, DENETİM].
  - Ertelenen durdurma bayrağı temizlenmiyor, `.TR_mem_save()` iki kez kaydediyor [ölçüldü, DENETİM].
- **Yapılacaklar:**
  - İki yerleşik; "durdurma istendi" hatası için ileti kodu.
  - Üstlenen programda yeni bir şey bekleyen yerleşik, `TR_fn_sleep` dahil, bu hatayla döner.
  - `TR_kw_defer` bloklarında ve bu hatayı yakalayan `TR_kw_catch` gövdesinde beklemeler normal çalışır.
  - Kontrol noktası isteği yalnız kaydeder, beklemeleri hataya çevirmez; istekten sonraki ilk kayıtta durur, bitişte kaydetmez.
  - Program `TR_fn_handle_stop` ile de üstlendiyse kontrol noktası yalnız kaydeder, durmayı programa bırakır.
  - Erteleme süreyle sınırlı: son iki `.TR_mem_update` arasındaki sürenin iki katı içinde kayda varılmazsa ya da istek ikinci güncellemeden önce geldiyse istek 1.5'teki gibi uygulanır.

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
  - İstek geldiyse süreç isteğin sinyaliyle biter (1.5, 1.7 ile); durdurmayı üstlenen program sona gelirse sinyal, `TR_fn_exit(kod)` ile o kod.
  - Görevdeki yakalanmayan hata olduysa çıkış 1.
  - `tr` aracının kullanım hatası 2 verir.

### 1.10

- **Mevcut durum:** `TR_fn_exit` yok; programın en dışındaki `TR_kw_return` çalışıyor (1.14'teki ölçüm).
- **Yapılacaklar:** yerleşik eklenir: nazik çıkış, `TR_kw_defer` çalışır, `TR_kw_catch` yakalamaz, görevden çağrılınca bütün program biter.
  - Programın en dışındaki `TR_kw_return` sözdizimi hatası olur; depodaki kullanımlar `TR_fn_exit`'e çevrilir.

### 1.11

- **Mevcut durum:** `TR_fn_exit` olmadığı için yok.
- **Yapılacaklar:** 0–125 dışındaki kod çalışma hatasıdır.

### 1.12

- **Mevcut durum:** `TR_fn_exit` olmadığı için yok.
- **Yapılacaklar:** `TR_cmd_test` altında `TR_fn_exit` testi düşürür, öbür testler sürer.

### 1.13

- **Mevcut durum:** `TR_fn_exit` olmadığı için yok.
- **Yapılacaklar:** ertelenen işten çağrılan `TR_fn_exit` kodu değiştirir; durdurmayı üstlenmemiş programa istek geldiyse sinyal korunur.

### 1.14

- **Mevcut durum:**
  - Program sona gelince ve en dıştaki `TR_kw_return` ile ertelenen işin hatası yazılıyor, çıkış 1 [ölçüldü].
  - Komut satırı çözümleyicisinin yardım ve kullanım hatası çıkışları ertelenen işin hatasını yutuyor, çıkış 0 ya da 2 [ölçüldü, DENETİM].
- **Yapılacaklar:**
  - Komut satırı çözümleyicisinin çıkışlarında da hata yazılır, çıkış 1.
  - `TR_fn_exit` ve durdurma kısımları 1.10 ve 1.5 ile.

### 1.15

- **Mevcut durum:** ölçülmedi.
- **Yapılacaklar:** önce ölçülür; bitişte gelen ilk Ctrl+C ertelenen işleri kesmez, işler bitince süreç SIGINT ile biter.

## 2. Sözcük yapısı

Kurallar: [`docs/spec/02_sozcuk_yapisi.md`](spec/02_sozcuk_yapisi.md). Taslak: DENETİM'den geçti (2026-10-27); Gemini'nin araştırması bekleniyor. Eski 2.9a (her dilde tek yazım) ve 2.11 (diller arası ad kuralı, $ad kaçışı) gösterim dilleri bölümüne gitti; o numaralar boş kalır. `tests/` altındaki 2.9a, 2.11 ve 2.12 numaralı testler, testler koşulmadan önce taşınır ya da silinir; yoksa her test numarasını bir kurala bağlayan kapı kalır. Eski 2.12 (çıplak ileti kodu) kalktı. Anlamı değişmeyen maddelerin bugünkü TR testi var; aşağıda yalnız değişen ya da eksik olanlar.

### 2.1

- **Mevcut durum:** başlığı olmayan metin bugün Türkçe yazılmış sayılıp okunuyor. Başlık bugün eski biçimde (tr kodlu 3 dil=tr): bir Türkçe sözcük ve dosyanın dilini taşıyor. Programın sonucu bu dile bağlı: `TR_fn_print(TR_kw_true)` dil=tr ile "doğru", dil=en ile "true" yazdırıyor [ölçüldü].
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

- **Mevcut durum:** listede olmayan kimlik hata veriyor ama en yakın kimliği önermiyor: `TR_fn_prnt` ve `x.TR_fn_map` için ileti yalnız "bilinen bir kimlik değil" diyor [ölçüldü].
- **Yapılacaklar:** ileti önce aynı adın başka sınıftaki kimliğini, yoksa aynı sınıfta yazımca en yakın kimliği önerir; uzaklık sınıftan sonraki adda ölçülür, komşu harflerin yer değiştirmesi tek değişikliktir.

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

- **Mevcut durum:** `TR_kw_while` ve `TR_kw_default` çalışıyor [ölçüldü].
- **Yapılacaklar:** ikisi kalkar; koşullu döngü `TR_kw_for` ve bir koşulla, varsayılan kol `_` ile yazılır; depodaki kullanımlar çevrilir.
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

### 3.5

- **Mevcut durum:** isteğe bağlı tip yok; `TR_type_Int` açıklamalı bir değişkene `TR_kw_none` atanabiliyor [ölçüldü].
- **Yapılacaklar:** `T?` tipi; açılmamış kullanım denetim hatası; sınamadan sonra daraltma. `.TR_mem_get` isteğe bağlı tip döner.

### 3.6

- **Mevcut durum:** karışık tipli liste çalışıyor [ölçüldü].
- **Yapılacaklar:** `TR_type_List[T]`; öğelerin tipi aynı, boş liste tipini yazar.

### 3.7

- **Mevcut durum:** `{k: 1}` anahtarı "k" metni sayıyor, `d.k` anahtarı okuyor, olmayan anahtar `TR_kw_none` veriyor, ondalık anahtar çalışıyor [ölçüldü].
- **Yapılacaklar:** `TR_type_Dict[K, V]`; değişmezdeki anahtar bir ifade, erişim yalnız köşeli parantezle, olmayan anahtar hata, anahtar yalnız `TR_type_Int`, `TR_type_String`, `TR_type_Bool`. Depodaki kullanımlar çevrilir.

### 3.8

- **Mevcut durum:** `3..0` geriye sayıyor (3, 2, 1) [ölçüldü].
- **Yapılacaklar:** başlangıç bitişten büyük ya da eşitse aralık boş; `.TR_mem_reverse()` aralıkta da çalışır.

### 3.10

- **Mevcut durum:** doğruluk sınaması var: boş liste koşulda yanlış sayılıyor [ölçüldü].
- **Yapılacaklar:** koşulda yalnız `TR_type_Bool`; başka tip denetim hatası, ileti açık sınamayı önerir.

### 3.13

- **Mevcut durum:** `0` ile başlayan değişkene ondalık eklenebiliyor [ölçüldü]; açıklamasız değişkenin tipi kalıcı değil.
- **Yapılacaklar:** statik tip (1.2); kalıcı yerel tip; tam sayı yazımı ondalık beklenen yerde ondalık.

### 3.14

- **Mevcut durum:** değerin metni dosyanın dilinde: Türkçe dosyada tip "Tam", yokluk "yok", sonsuz "sonsuz" yazılıyor [ölçüldü].
- **Yapılacaklar:** değerin metni dilden bağımsız, kimliğin ad kısmıyla.
