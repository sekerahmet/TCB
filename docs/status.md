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
  - `TR_kw_import ... TR_ctx_as ad` modülü bugün `TR_type_Dict`'e bağlıyor (11.2), üyelerin tipi korunmalı; gerçek sürücü ile benzetim arasında seçim için ortak bir arayüz tipi.

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
  - §19 `--ros-args`'ı bilinmeyen seçenek sayıyor.
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
  - İstek geldiyse süreç isteğin sinyaliyle biter (1.5, 1.7 ile); durdurmayı üstlenen program sona gelirse sinyal, `TR_fn_exit(kod)` ya da en dıştaki `TR_kw_return kod` ile o kod.
  - Görevdeki yakalanmayan hata olduysa çıkış 1.
  - `tr` aracının kullanım hatası 2 verir.

### 1.10

- **Mevcut durum:** `TR_fn_exit` yok.
- **Yapılacaklar:** yerleşik eklenir: nazik çıkış, `TR_kw_defer` çalışır, `TR_kw_catch` yakalamaz, görevden çağrılınca bütün program biter.

### 1.11

- **Mevcut durum:** `TR_fn_exit` olmadığı için yok.
- **Yapılacaklar:** 0–125 dışındaki kod çalışma hatasıdır.

### 1.12

- **Mevcut durum:** `TR_fn_exit` olmadığı için yok.
- **Yapılacaklar:** `tr test` altında `TR_fn_exit` testi düşürür, öbür testler sürer.

### 1.13

- **Mevcut durum:** `TR_fn_exit` olmadığı için yok.
- **Yapılacaklar:** ertelenen işten çağrılan `TR_fn_exit` kodu değiştirir; durdurmayı üstlenmemiş programa istek geldiyse sinyal korunur.

### 1.14

- **Mevcut durum:**
  - Program sona gelince ve en dıştaki `TR_kw_return` ile ertelenen işin hatası yazılıyor, çıkış 1 [ölçüldü].
  - §19'un yardım ve kullanım hatası çıkışları ertelenen işin hatasını yutuyor, çıkış 0 ya da 2 [ölçüldü, DENETİM].
- **Yapılacaklar:**
  - §19'un çıkışlarında da hata yazılır, çıkış 1.
  - `TR_fn_exit` ve durdurma kısımları 1.10 ve 1.5 ile.

### 1.15

- **Mevcut durum:** ölçülmedi.
- **Yapılacaklar:** önce ölçülür; bitişte gelen ilk Ctrl+C ertelenen işleri kesmez, işler bitince süreç SIGINT ile biter.

## 2. Sözcük yapısı

Kurallar: [`docs/spec/02_sozcuk_yapisi.md`](spec/02_sozcuk_yapisi.md). Taslak: DENETİM ve Gemini'nin araştırması bekleniyor. Eski 2.9a (her dilde tek yazım) ve 2.11 (diller arası ad kuralı, $ad kaçışı) gösterim dilleri bölümüne gitti; o numaralar boş kalır, testleri o bölüm yazılınca taşınır. Anlamı değişmeyen maddelerin bugünkü TR testi var; aşağıda yalnız değişen ya da eksik olanlar.

### 2.1

- **Mevcut durum:** başlığı olmayan metin bugün Türkçe yazılmış sayılıp okunuyor.
- **Yapılacaklar:** diskteki her dosya kodludur; başlıksız dosya çalışmaz, ileti kodlama yolunu söyler. Bilerek bozuk sınav dosyaları, belgelerdeki kod blokları ve Go testlerindeki TR metni kodlu biçime geçer.

### 2.2

- **Mevcut durum:** iç içe çok satırlı yorum sözdizimi hatası veriyor [ölçüldü].
- **Yapılacaklar:** çok satırlı yorum iç içe okunur.

### 2.3

- **Mevcut durum:** adlar karşılaştırılmadan önce normalleştirilmiyor (sözcük çözümleyicide normalleştirme yok).
- **Yapılacaklar:** adlar NFC'ye göre karşılaştırılır; kodlama ve gösterim de aynı biçimi kullanır.

### 2.4

- **Mevcut durum:** `0x1F` sözdizimi hatası veriyor [ölçüldü].
- **Yapılacaklar:** `0x`, `0o`, `0b` ön ekleri ve ön ekten sonra `_`.

### 2.6

- **Mevcut durum:** `\u{...}` bilinmeyen kaçış dizisi hatası veriyor [ölçüldü].
- **Yapılacaklar:** `\u{...}` kaçışı; bilinmeyen kaçış iletisi bilinenlere onu da ekler.
