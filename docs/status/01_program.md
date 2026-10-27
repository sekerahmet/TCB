# Uygulama durumu: 1. bölüm

Dil tanımının [1. bölümü](../spec/01_program.md) kuralları yazar; bu belge o kurallara göre bugünkü TR'nin durumunu tutar: kararın kaydı ve bugünkü davranışın kuraldan farkı. Bir karar uygulanıp testi geçince satırı silinir. Yapılacak işlerin sırası [yol haritasındadır](../roadmap.md).

- **1.1** Karar: Ahmet, 2026-10-27 (modül ve test dosyası); 2026-10-27 (sabit ilk değer). Bugün modülün ve test dosyasının en dışındaki deyimler de çalışıyor (11.1, 12.5), ilk değer her ifadeyi kabul ediyor (5.1, 5.2).
- **1.2** Karar: Ahmet, 2026-10-27. Bugün TR dinamik tiplidir: tip açıklaması isteğe bağlıdır (3.13).
- **1.3** Karar: Ahmet, 2026-10-27 ("modülde hata varsa program hiç başlamasın"); aktarmanın yeri ve koşullu modülün olmaması 2026-10-27. Bugün:
  - `TR_kw_import` her yerde yazılabiliyor ve adları bulunduğu bloğa getiriyor [ölçüldü: `TR_kw_if` içinde aktarılan fonksiyon dışarıda tanımsız];
  - çalıştırmada modülün hatası `TR_kw_import` satırında çıkıyor, aktaranın önceki satırları çalışmış oluyor (11.4); `TR_cmd_check` ve `TR_cmd_build` modülleri zaten önceden denetliyor;
  - `TR_cmd_debug` tip denetimini hiç yapmıyor, hatalı programın satırları çalışıyor [ölçüldü, DENETİM];
  - takma adsız `TR_kw_import` olan kapsamda tanımsız adlar çalışmadan önce aranmıyor, tanımından önce okunan ad da ancak çalışırken hata veriyor (5.14).
- **1.4** Karar: Ahmet, 2026-10-27; ROS 2 kısmı 2026-10-27. Bugün argüman okunmadan program biterse bitişte uyarı yazılıyor, çıkış kodu değişmiyor; §19 `--ros-args`'ı bilinmeyen seçenek sayıyor.
- **1.5** Karar: Ahmet, 2026-10-27 (sinyalle bitiş); 2026-10-27 (bırakma kuralı). Bugün süreç temizlikten sonra 3 koduyla çıkıyor; çalışan her yerleşik önce bitiyor, `TR_fn_input` satır beklerken ilk istek beklemeyi kesmiyor; ağ ve alt süreç yerleşiği henüz yok.
- **1.6** Karar: 2026-10-27; adlar Ahmet'in onayıyla. `TR_fn_handle_stop` ve `TR_fn_stop_requested` henüz yok. Bugün kontrol noktası kuruluyken bekleme uyanmıyor; son kayıttan sonra gelen istek kayboluyor, çıkış 0 [ölçüldü, DENETİM].
- **1.7** Karar: Ahmet, 2026-10-27. Bugün süreç sinyalle değil, 130 koduyla çıkıyor.
- **1.8** Uygulanacak bir şey yok: davranış işletim sistemine ve süreci başlatana bağlı.
- **1.9** Karar: Ahmet, 2026-10-27. Bugün durdurmada 3 ve 130 kodlarıyla çıkılıyor; `tr` aracının kendi kullanım hatası 1 veriyor.
- **1.10** Karar: Ahmet, 2026-10-27. `TR_fn_exit` henüz yok.
- **1.11**, **1.12**, **1.13** Karar: Ahmet, 2026-10-27; 1.10 ile birlikte uygulanır.
- **1.14** Karar: Ahmet, 2026-10-27. Program sona gelince ve en dıştaki `TR_kw_return` ile bugün de böyle [ölçüldü: çıkış 1]; §19'un çıkışlarında bugün hata yazılmıyor, çıkış 0 ya da 2 [ölçüldü, DENETİM]; `TR_fn_exit` kısmı 1.10 ile, durdurma kısmı 1.5 ile uygulanır.
- **1.15** Karar: Ahmet, 2026-10-27. Bugünkü davranış ölçülmedi.
