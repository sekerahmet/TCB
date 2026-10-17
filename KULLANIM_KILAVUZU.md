# 🇹🇷 TR (`.tr`) Programlama ve Yapay Zeka Dili — Kullanım Kılavuzu

## 1. Hızlı Başlangıç Komutları

| Komut | Açıklama |
| :--- | :--- |
| `.\tr.exe` | Etkileşimli Türkçe TR kabuğunu (REPL) başlatır (`/h` ile ABAP Debugger açılır) |
| `.\tr.exe dosya.tr` | `.tr` kaynak dosyasını derler ve anında çalıştırır |
| `.\tr.exe debug dosya.tr` | SAP ABAP tarzı etkileşimli Terminal Hata Ayıklayıcıyı (Debugger) başlatır |
| `.\tr.exe debug --web dosya.tr` | Tarayıcı tabanlı **Görsel ABAP Debugger Studio**'yu başlatır. Yalnızca `127.0.0.1:8090` üzerinde dinler; terminalde yazdırılan **anahtarlı bağlantıyı** (`http://127.0.0.1:8090/?anahtar=...`) açın |
| `.\tr.exe kontrol dosya.tr` | Kodu çalıştırmadan sözdizimi ve akıllı tip kontrolü yapar |
| `.\tr.exe bytecode dosya.tr` | Derlenen Türkçe Sanal Makine (VM) bytecode komutlarını gösterir |
| `.\tr.exe derle dosya.tr` | `.tr` dosyasını tek başına çalışan bağımsız **`.exe`** programına dönüştürür |

---

## 2. Anahtar Kelimeler (Çift Modlu: Türkçe & ASCII)

- **Değişken & Sabit:** `değişken` (`degisken`), `:=`, `sabit`
- **Çoklu Atama & Takas:** `değişken a, b = 10, 20`, `a, b = b, a`, `ad, soyad = ["Mustafa", "Kemal"]`
- **Fonksiyon & Dönüş:** `fonksiyon` (`işlev`), `sonuç` (`sonuc`), `(x) => x * 2`
- **Koşul & Eşleştirme:** `eğer` (`eger`), `değilse eğer`, `değilse` (`degilse`), `eşle` (`esle`), `varsayılan` (`varsayilan`)
- **Döngüler & Liste Üreteci:** `döngü x içinde liste` (`dongu x icinde liste`), `1..10`, `1..=10`, `iken`, `çık` (`cik`), `devam`, `[x * x döngü x içinde 1..=10 eğer x % 2 == 0]`
- **Mantıksal & Boş:** `doğru` (`dogru`), `yanlış` (`yanlis`), `yok`, `ve` (`&&`), `veya` (`||`), `değil` (`!`), `??`, `?.`
- **Tipler:** `Tam`, `Ondalık`, `Metin`, `Mantık`, `Liste`, `Sözlük`, `Matris`, `Tensör`
- **Yapı / Sınıf:** `yapı` (`yapi`), `sınıf` (`sinif`), `kendi`
- **Hata, Ertele, Kesme Noktası, Modül & Eşzamanlılık:** `dene`, `yakala`, `fırlat` (`firlat`), `ertele`, `kesme_noktası` (`kesme_noktası eğer koşul`), `aktar`, `eşzamanlı` (`eszamanli`), `bekle(saniye)`, `bekle_hepsi()`

---

## 3. Nokta (`.`) ile Zincirleme Metodlar

- **Metin (`Metin`):** `.uzunluk`, `.büyük_harf()` (tam Türkçe `i -> İ` uyumlu), `.küçük_harf()`, `.kırp()`, `.böl(ayırıcı)`, `.değiştir(eski, yeni)`, `.içerir(alt)`, `.başlar(önek)`, `.biter(sonek)`, `.ters_çevir()`
- **Liste (`Liste`):** `.uzunluk`, `.ilk`, `.son`, `.filtrele(fn)`, `.haritala(fn)`, `.sırala()`, `.ters_çevir()`, `.birleştir(ayırıcı)`, `.benzersiz()`, `.içerir(el)`, `.ekle(el)`, `.toplam()`, `.ortalama()`
- **Sözlük (`Sözlük`):** `.anahtarlar()`, `.değerler()`, `.içerir(anahtar)`

### `o` zamiri ve sondaki blok

Bir çağrının sonuna aynı satırda yazılan `{ ... }` bloğu, tek parametresi `o` olan bir fonksiyondur. `o` o anki elemanı gösterir:

```tr
sayılar.filtrele { o > 2 }.haritala { o * o }   // sayılar.filtrele((x) => x > 2)... ile aynı
personel.haritala { o["ad"] }
sayılar |> filtrele { o % 2 == 0 }
```

- İç içe bloklarda `o` en içteki elemanı gösterir: `[[1, 2]].haritala { o.haritala { o * 10 } }`.
- `eğer`, `iken`, `döngü` ve `eşle` başlıklarında `{` her zaman gövdeyi başlatır. Başlıkta sondaki blok kullanmak için ifadeyi paranteze alın: `eğer (liste.filtrele { o > 2 }).uzunluk > 0 { ... }`.
- Uzun yazım (`(x) => x > 2`) aynen çalışmaya devam eder.

### Hata mesajları

Hatalar konumuyla, kaynak satırıyla ve mümkünse bir öneriyle gösterilir:

```text
❌ Çalışma Zamanı Hatası:
  • Satır 3, Sütun 9: 'toplm' adında bir değişken ya da fonksiyon yok. Bunu mu demek istediniz: 'toplam'?
     3 |     yazdır(toplm + 1)
       |            ^^^^^
```

Öneriler Türkçe/ASCII yazım farkını hata saymaz (`degisken_adi` → `değişken_adı`) ve yazdığınız biçime uyar. Mesajlardaki ekler sayının okunuşuna göre seçilir (`Satır 6'ya`, `Satır 9'a`, `Satır 3'te`).

---

## 4. Yerleşik Yapay Zeka ("İkisi Bir Arada") ve Veri Bilimi Komutları

### A. Düşük Seviye Matris ve Otomatik Türev (PyTorch Tarzı)
- `matris([[1, 2], [3, 4]])`, `sıfırlar(satır, sütun)`, `birler(satır, sütun)`, `birim_matris(n)`, `rastgele_matris(satır, sütun, tohum: 42)`
- `A @ B` (Çok çekirdekli matris çarpımı), `devrik(A)` (veya `A.devrik`, `A.şekil`)
- `tensör(3.0, türev_izle: doğru)`, `geri_yayılım(y)`, `x.gradyan`, `x.değer`
- Aktivasyonlar ve Kayıp: `relu(x)`, `sigmoid(x)`, `tanh(x)`, `softmax(x)`, `kayıp_mse(tahmin, gerçek)`

### B. Yüksek Seviye Hazır Yapay Sinir Ağı + Kaydet/Yükle (Keras Tarzı)
```tr
değişken model = sinir_ağı([2, 8, 1], aktivasyon: "tanh", tohum: 42)
model.eğit(X_veri, Y_hedef, devir: 800, öğrenme_hızı: 0.15)
model.kaydet("modelim.json")

değişken hazır_model = model_yükle("modelim.json")
değişken tahmin = hazır_model.tahmin_et(X_veri)
```

### C. Veri Bilimi, Vektör Benzerlik, Web ve Dış Köprüler
- İstatistik: `ortalama(liste)`, `standart_sapma(liste)`, `medyan(liste)`, `normalize_et(liste)`
- Vektör: `nokta_çarpım(v1, v2)`, `kosinüs_benzerliği(v1, v2)`, `öklid_uzaklığı(v1, v2)`
- Veri, Web & Köprüler: `csv_oku(yol)`, `json_çöz(metin)`, `json_metin(değer)`, `web_al(url)`, `web_gönder(url, veri)`, `python_çalıştır(kod)`, `yz_sor(istem)`

---

## 5. SAP ABAP Tarzı Hata Ayıklayıcı (ABAP Debugger)

TR dili, SAP ABAP New Debugger (SE80 / `/h`) mimarisinden ilham alan hem **Terminal** hem de **Görsel Web Studio** hata ayıklayıcısına sahiptir:

1. **Kod İçi Kesme Noktası (`BREAK-POINT`):**
   - Koşulsuz: `kesme_noktası`
   - Koşullu: `kesme_noktası eğer bakiye < 0`
2. **Adımlama ve Kontrol Komutları:**
   - `f5` (`adım` veya boş `Enter`): Tek adım ilerle (fonksiyonun içine girer)
   - `f6` (`geç`): Üstten geç (fonksiyonu tek adımda çalıştırır)
   - `f7` (`dön`): Mevcut fonksiyondan çıkana kadar çalıştır
   - `f8` (`devam`): Bir sonraki kesme noktasına veya izleme noktasına (Watchpoint) kadar devam et
3. **Canlı Değişken Değiştirme (ABAP ✏️ Kalem İkonu):**
   - `değiştir vergi_orani = 0.20` (Programı durdurmadan çalışma anında değişkenin değerini değiştirir)
4. **Satıra Zıplama (ABAP `Shift+F12` — Jump to Statement):**
   - `zıpla 25` (Çalışma okunu geçmiş veya gelecek bir satıra taşır; kodu baştan başlatmadan tekrar deneme imkanı verir)
5.- **İç Tablo / Matris İnceleyici (ABAP ALV / ITAB Viewer) & Watchpoint:**
   - `tablo siparisler` (Liste, Sözlük veya Yapay Zeka Matrislerini tablo görünümünde açar)
   - `izle bakiye` veya `izle bakiye < 0` (Değişken değiştiğinde veya koşul sağlandığında otomatik durur)
   - `yığın` (Aktif fonksiyon çağrı yığınını gösterir)

---

## 6. Görsel Arayüz (SAP Fiori / ALV Grid & Grafik) ve Web Sunucu Motoru

### A. Görsel Arayüz Motoru (`arayüz_oluştur` / `pencere_oluştur`)
```tr
değişken ekran = arayüz_oluştur("Kurumsal ERP Kokpiti", genişlik: 1100, yükseklik: 780)
ekran.kart_ekle("Toplam Ciro", "1.450.000 TL", "+%18 Artış", "yeşil")
ekran.grafik_ekle("Aylık Satışlar", ["Ocak", "Şubat", "Mart"], [420, 510, 520], "çubuk")
ekran.tablo_ekle("SAP ALV Sipariş Tablosu", siparis_listesi)
ekran.form_ekle("Hızlı Sipariş Girişi", ["Müşteri", "Tutar"], fonksiyon(form) {
    sonuç "{form['Müşteri']} için {form['Tutar']} TL kaydedildi!"
})
ekran.html_kaydet("rapor.html") // veya ekran.başlat() ile canlı masaüstü penceresi açar
```

### B. Web Sunucu & REST API Motoru (`web_sunucu`)
```tr
değişken api = web_sunucu(port: 8080)
api.al("/api/urunler", fonksiyon(istek) {
    sonuç [{"id": 1, "ad": "Laptop", "fiyat": 45000}]
})
api.gönder("/api/ekle", fonksiyon(istek) {
    değişken gelen = istek["veri"]
    sonuç {"durum": "tamam", "eklenen": gelen}
})
api.başlat() // veya api.arka_planda_başlat()
```

---

## 7. Yerleşik Türkçe SQL Veritabanı ve ABAP OpenSQL İç Tablo Sorgulama

```tr
// 1. Kalıcı Disk Veritabanı (.trdb) veya Bellek Veritabanı (":bellek:")
değişken vt = veritabanı_aç("sirket.trdb")
vt.ekle("personel", {"id": 101, "ad": "Ayşe Yılmaz", "departman": "Yapay Zeka", "maas": 92000})

// Türkçe SQL veya Standart SQL ile Sorgulama
değişken sonuc = vt.sorgula("SEÇ ad, maas TABLO personel NEREDE maas >= 80000 SIRALA maas AZALAN")
vt.güncelle("personel", {"maas": 98000}, "id == 101")

// Dışarıdan gelen değerleri (form, URL parametresi) sorguya gömmeyin; '?' ile bağlayın:
değişken ekip = vt.sorgula("SEÇ ad TABLO personel NEREDE departman == ? VE maas > ?", [departman, 50000])
vt.sil("personel", "id == ?", [silinecek_id])   // güncelle(..., koşul, [parametreler]), say(tablo, koşul, [parametreler]) de aynı

// Not: NEREDE koşulları TR kodu olarak çalıştırılmaz. Yalnızca sütun adları, sayılar, 'metin',
// doğru/yanlış/yok, + - * / %, karşılaştırmalar, ve/veya/değil (AND/OR/NOT) ve parantez desteklenir.
// Bilinmeyen sütun veya hatalı koşul artık sessizce boş liste döndürmez, hata verir.

// 2. ABAP OpenSQL Tarzı Doğrudan Bellek-İçi Liste (ITAB) Sorgulama
değişken elit = tablo_sorgula(liste, "maas > 85000", sırala: "maas:azalan", alanlar: ["ad", "maas"])
```

---

## 8. VS Code Editör Eklentisi Kısayolları (Snippets)
`.tr` uzantılı bir dosya açtığınızda aşağıdaki kısayolları yazıp `Tab` tuşuna basabilirsiniz:
- `fonk` $\rightarrow$ Fonksiyon bloğu oluşturur
- `eger` $\rightarrow$ `eğer ... değilse` bloğu oluşturur
- `esle` $\rightarrow$ `eşle` durum eşleştirme bloğu oluşturur
- `dongu` $\rightarrow$ `döngü i içinde 1..=10` bloğu oluşturur
- `liste_uret` $\rightarrow$ Tek satırda liste üreteci oluşturur
- `yapi` $\rightarrow$ `yapı` (sınıf) ve metod şablonu oluşturur
- `sinir` $\rightarrow$ 3 satırlık yapay sinir ağı eğitim şablonu oluşturur
- `tensor` $\rightarrow$ Otomatik türev (`tensör` + `geri_yayılım`) şablonu oluşturur


