# 1. Program

Bir TR programı, yukarıdan aşağıya çalışan deyimlerden oluşan tek bir `.tr` dosyasıdır; `main` gibi bir giriş noktası yoktur. Çalıştırmadan önce dosya ayrıştırılır ve tip denetleyiciden geçer; ikisinden birinde hata varsa hiçbir satır çalışmaz. Aynı kural `aktar` ile alınan her modül için de geçerlidir (11.4).

```tr
yazdır("Merhaba, dünya!")
```
