# Durum

Dil tanımının yeniden yazılan bölümlerine göre bugünkü TR'nin durumu ve kurala varmak için yapılacaklar. Her maddenin kuralı dil tanımındadır ([`docs/spec/`](spec/README.md)); yapılan iş testiyle birlikte buradan silinir. İşlerin sırası [yol haritasındadır](roadmap.md).

## 1. Program

Kurallar: [`docs/spec/01_program.md`](spec/01_program.md). Kararlar henüz kesin değil: Gemini'nin araştırması bekleniyor, yapılacaklar ondan sonra başlar.

### 1.1

- **Mevcut durum:**
  - Modülün ve test dosyasının en dışındaki deyimler de çalışıyor (11.1, 12.5).
  - Modülde ilk değer her ifadeyi kabul ediyor (5.1, 5.2).
- **Yapılacaklar:**
  - Denetleyici modülde ve test dosyasında en dışta tanım dışındaki deyimi hata sayar.
  - Modülde ilk değer sabit ifade değilse hata verir; `TR_fn_fuse` ve `TR_fn_replay` çağrısı serbesttir.
  - `lib/`, `tests/` ve örnekler kurala göre düzeltilir.

### 1.2

- **Mevcut durum:** TR dinamik tiplidir; tip açıklaması isteğe bağlıdır (3.13).
- **Yapılacaklar:** büyük tasarım, turlarla:
  - denetleyiciye tip çıkarımı;
  - kitaplığın, örneklerin ve testlerin imzaları;
  - "olabilir" tipi;
  - tensörün tür ve şekil bilgisinin tipe ne kadar gireceği.

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
  - Argüman verilip `TR_fn_command_line` hiç çağrılmıyorsa program çalışmadan kullanım hatası verir (2).
  - Bitişteki uyarı silinir.
  - ROS 2'nin `--ros-args` blokları programın argümanlarından ayrılır.

### 1.5

- **Mevcut durum:**
  - Süreç temizlikten sonra 3 koduyla çıkıyor.
  - Çalışan her yerleşik önce bitiyor; `TR_fn_input` satır beklerken ilk istek beklemeyi kesmiyor.
  - Ağ ve alt süreç yerleşiği yok.
- **Yapılacaklar:**
  - Süreç isteğin kendi sinyaliyle biter.
  - Durum değiştirmeyen yerleşik ve her bekleme bırakılır, önce `TR_fn_input` ve dosyadan yükleme.
  - Durum değiştiren yerleşiklerin listesi yazılır ve sınanır.
  - Alt süreç yerleşiği gelince bırakılan alt sürece istek iletilir.

### 1.6

- **Mevcut durum:**
  - `TR_fn_handle_stop` ve `TR_fn_stop_requested` yok (adları onaylı).
  - Kontrol noktası kuruluyken bekleme uyanmıyor.
  - Son kayıttan sonra gelen istek kayboluyor, çıkış 0 [ölçüldü, DENETİM].
  - Ertelenen durdurma bayrağı temizlenmiyor, `.TR_mem_save()` iki kez kaydediyor [ölçüldü, DENETİM].
- **Yapılacaklar:**
  - İki yerleşik; "durdurma istendi" hatası için ileti kodu.
  - Üstlenen programda yeni iş bekleyen yerleşik bu hatayla döner, `TR_fn_sleep` hemen döner.
  - Kontrol noktası aynı yolu kullanır; istekten sonraki ilk kayıtta durur, bitişte kaydetmez.

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
- **Yapılacaklar:**
  - İstek geldiyse süreç her yoldan isteğin sinyaliyle biter (1.5, 1.7 ile).
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
- **Yapılacaklar:** ertelenen işten çağrılan `TR_fn_exit` kodu değiştirir; istek geldiyse sinyal korunur.

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
