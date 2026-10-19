# 1. Program

Bir TR programı, yukarıdan aşağıya çalışan deyimlerden oluşan tek bir `.tr` dosyasıdır; `main` gibi bir giriş noktası yoktur. Çalıştırmadan önce dosya ayrıştırılır ve tip denetleyiciden geçer (tip hataları ve tanımsız adlar, 5.14); ikisinden birinde hata varsa hiçbir satır çalışmaz. Aynı kural `aktar` ile alınan her modül için de geçerlidir (11.4).

```tr
yazdır("Merhaba, dünya!")
```

Dosya adından sonra yazılan argümanlar programındır: `tr egit.tr --adım 5000 veri/` (aynı biçimde `tr çalıştır`, `tr hata_ayıkla`, `tr derle` ile üretilen uygulama ve düzenleyicinin başlatma ayarındaki `"args"`). Argümanlar sessizce yok sayılmaz: program onları okumadan biterse standart hataya `⚠️ Uyarı: programa verilen 3 argüman kullanılmadı (...)` yazılır (çıkış kodu değişmez). Argümanları `komut_satırı` okur (`--ad değer` seçenekleri, `--yardım`; bkz. §19).

Çalışan program Ctrl+C ile ya da bir sonlandırma isteğiyle (Unix'te SIGTERM, SIGHUP; Windows'ta Ctrl+Break, konsol kapatma, oturum kapatma) **nazikçe durdurulur**: program bir sonraki güvenli noktada (döngü turu, fonksiyon çağrısı, bekleme ya da okuma bitince) durur, yığın çözülürken bütün `ertele` blokları (9) çalışır, `dene/yakala` (8) kesilmeyi yakalamaz, eşzamanlı görevler de durur; süreç **3** koduyla çıkar ("durduruldu"). Program bir kontrol noktası kurduysa (§20) ilk istek adım sınırına ertelenir: sıradaki `kn.güncelle(...)` noktayı kaydedip programı aynı biçimde durdurur. `bekle(...)` hemen uyanır. Ertelenen işler bitmezse ikinci Ctrl+C süreci hemen bitirir (çıkış kodu 130). Sınırlar: tek bir uzun yerleşik işlem (ör. büyük bir tensör çarpımı) bitmeden durulmaz; `girdi()` satır beklerken ilk istek beklemeyi kesmez; Windows'ta konsol penceresi kapatılınca işletim sistemi süreci birkaç saniye içinde öldürür. Çıkış kodları: 0 başarı, 1 hata, 2 komut satırı seçeneklerinde kullanım hatası (§19), 3 kesme isteğiyle durduruldu, 130 ikinci istekle hemen çıkış.
