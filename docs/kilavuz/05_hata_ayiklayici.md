# 5. TR Hata Ayıklayıcı

TR'nin **terminalde** (`tr hata_ayıkla dosya.tr`, kabukta `/h`) çalışan bir hata ayıklayıcısı vardır. VS Code'da hata ayıklarken (`tr dap`) değişkenler yandaki **İzleme Masası** panelinde tek tabloda görünür; listeler tip sütununda `Tablo [3×3]` gibi etiketlenir ve adına tıklanınca tablo olarak açılır (ayrıntılar: `duzenleyici/vscode/README.md`). Denemek için: `tr ornekler/04_hata_ayiklama.tr` (koşullu kesme noktasında durur; `değiştir vergi_orani = 0.20`, ardından `devam`).

1. **Kod İçi Kesme Noktası (`kesme_noktası`):**
   - Koşulsuz: `kesme_noktası`
   - Koşullu: `kesme_noktası eğer bakiye < 0`
2. **Adımlama ve Kontrol Komutları** (tuş adları VS Code'dakiyle aynı anlamdadır; terminalde tuş adını yazıp `Enter`'a basın):
   - `f11` (`adım` veya boş `Enter`): Tek adım ilerle (fonksiyonun içine girer)
   - `f10` (`geç`): Üstten geç (fonksiyonu tek adımda çalıştırır)
   - `shift+f11` (`dön`): Mevcut fonksiyondan çıkana kadar çalıştır
   - `f5` (`devam`): Bir sonraki kesme noktasına ya da izleme noktasına kadar devam et
   - Eski terminal tuşları `f6`, `f7`, `f8` artık bir şey yapmaz; yazılırsa yeni tuşu söyler.
3. **Canlı Değişken Değiştirme (✏️):**
   - `değiştir vergi_orani = 0.20` (Programı durdurmadan çalışma anında değişkenin değerini değiştirir)
4. **Satıra Zıplama:**
   - `zıpla 25` (Çalışma okunu geçmiş veya gelecek bir satıra taşır; kodu baştan başlatmadan tekrar deneme imkanı verir)
5. **Tablo Görünümü ve İzleme Noktası:**
   - `tablo siparisler` (Liste, Sözlük veya Tensörleri tablo görünümünde açar)
   - `izle bakiye` veya `izle bakiye < 0` (Değişken değiştiğinde veya koşul sağlandığında otomatik durur)
   - `yığın` (Aktif fonksiyon çağrı yığınını gösterir)
