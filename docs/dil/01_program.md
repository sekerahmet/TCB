# 1. Program

Bir TR programı, yukarıdan aşağıya çalışan deyimlerden oluşan tek bir `.tr` dosyasıdır; `main` gibi bir giriş noktası yoktur. Çalıştırmadan önce dosya ayrıştırılır ve tip denetleyiciden geçer (tip hataları ve tanımsız adlar, 5.14); ikisinden birinde hata varsa hiçbir satır çalışmaz. Aynı kural `aktar` ile alınan her modül için de geçerlidir (11.4).

```tr
yazdır("Merhaba, dünya!")
```

Dosya adından sonra yazılan argümanlar programındır: `tr egit.tr --adım 5000 veri/` (aynı biçimde `tr çalıştır`, `tr hata_ayıkla`, `tr derle` ile üretilen uygulama ve VS Code başlatma ayarındaki `"args"`). Argümanlar sessizce yok sayılmaz: program onları okumadan biterse standart hataya `⚠️ Uyarı: programa verilen 3 argüman kullanılmadı (...)` yazılır (çıkış kodu değişmez). Argümanları okuyan yerleşik standart kitaplıkla gelecek; bugün program argüman okumaz.
