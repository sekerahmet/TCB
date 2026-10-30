# 5. TR Hata Ayıklayıcı

TR'nin **terminalde** (`tr debug dosya.tr`, kabukta `/h`) çalışan bir hata ayıklayıcısı vardır. VS Code'da hata ayıklarken (`TR_cmd_dap`) değişkenler yandaki **İzleme Masası** panelinde tek tabloda görünür; listeler tip sütununda `Tablo [3×3]` gibi etiketlenir ve adına tıklanınca tablo olarak açılır (ayrıntılar: `editors/vscode/README.md`). Denemek için: `tr examples/tr/42_hata_ayiklama.tr` (koşullu kesme noktasında durur; `set vergi_orani = 0.20`, ardından `continue`).

1. **Kod İçi Kesme Noktası (`TR_breakpoint`):**
   - Koşulsuz: `TR_breakpoint`
   - Koşullu: `TR_breakpoint TR_if bakiye < 0`
2. **Adımlama ve Kontrol Komutları** (tuş adları VS Code'dakiyle aynı anlamdadır; terminalde tuş adını yazıp `Enter`'a basın):
   - `f11` (`step` veya boş `Enter`): Tek adım ilerle (fonksiyonun içine girer)
   - `f10` (`next`): Üstten geç (fonksiyonu tek adımda çalıştırır)
   - `shift+f11` (`out`): Mevcut fonksiyondan çıkana kadar çalıştır
   - `f5` (`continue`): Bir sonraki kesme noktasına ya da izleme noktasına kadar devam et
   - Eski terminal tuşları `f6`, `f7`, `f8` ve Türkçe komut adları (`devam`, `adim`, `degistir` ...) çalışmaz; yazılırsa geçerli tuşu ya da komutu önerir (17.9). Hata ayıklayıcıya yazılan ifadeler (`set`, `watch`, koşullu kesme noktası) kod gibi kimlik yazımıyla okunur (17.7).
3. **Canlı Değişken Değiştirme (✏️):**
   - `set vergi_orani = 0.20` (Programı durdurmadan çalışma anında değişkenin değerini değiştirir)
4. **Satıra Zıplama:**
   - `jump 25` (Çalışma okunu geçmiş veya gelecek bir satıra taşır; kodu baştan başlatmadan tekrar deneme imkanı verir)
5. **Tablo Görünümü ve İzleme Noktası:**
   - `table siparisler` (Liste, Sözlük veya Tensörleri tablo görünümünde açar)
   - `watch bakiye` veya `watch bakiye < 0` (Değişken değiştiğinde veya koşul sağlandığında otomatik durur)
   - `stack` (Aktif fonksiyon çağrı yığınını gösterir)
