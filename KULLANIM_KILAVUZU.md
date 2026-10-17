# 🇹🇷 TR (`.tr`) Programlama ve Yapay Zeka Dili — Kullanım Kılavuzu

## 1. Hızlı Başlangıç Komutları

| Komut | Açıklama |
| :--- | :--- |
| `.\tr.exe` | Etkileşimli Türkçe TR kabuğunu (REPL) başlatır (`/h` ile hata ayıklayıcı açılır) |
| `.\tr.exe dosya.tr` | `.tr` kaynak dosyasını derler ve anında çalıştırır |
| `.\tr.exe debug dosya.tr` | Terminalde etkileşimli TR Hata Ayıklayıcıyı başlatır |
| `.\tr.exe debug --web dosya.tr` | Tarayıcı tabanlı **görsel hata ayıklayıcıyı** başlatır. Yalnızca `127.0.0.1:8090` üzerinde dinler; terminalde yazdırılan **anahtarlı bağlantıyı** (`http://127.0.0.1:8090/?anahtar=...`) açın |
| `.\tr.exe kontrol dosya.tr` | Kodu çalıştırmadan sözdizimi ve akıllı tip kontrolü yapar |
| `.\tr.exe bytecode dosya.tr` | Derlenen Türkçe Sanal Makine (VM) bytecode komutlarını gösterir |
| `.\tr.exe derle dosya.tr` | `.tr` dosyasını tek başına çalışan bağımsız **`.exe`** programına dönüştürür |
| `.\tr.exe test [dosya.tr \| klasör]` | Test bloklarını çalıştırır; klasör verilirse (varsayılan: bulunulan klasör) altındaki tüm `*_test.tr` dosyalarını tarar. Bir test bile kalırsa çıkış kodu 1'dir (bkz. §9) |
| `.\tr.exe biçimle dosya.tr` | Kodu standart biçime getirip dosyaya yazar; klasör verilirse içindeki tüm `.tr` dosyalarını biçimler (`bicimle` da olur, bkz. §10) |
| `.\tr.exe biçimle --kontrol dosya.tr` | Dosyayı değiştirmeden biçimlenmesi gerekip gerekmediğini söyler; gerekiyorsa çıkış kodu 1'dir |

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
- **Hata, Ertele, Kesme Noktası, Modül & Eşzamanlılık:** `dene`, `yakala`, `fırlat` (`firlat`), `ertele`, `kesme_noktası` (`dur_noktası`, `durdurma_noktası`; koşullu: `kesme_noktası eğer koşul`), `aktar`, `eşzamanlı` (`eszamanli`), `bekle(saniye)`, `bekle_hepsi()`
- **Test:** `test "ad" { ... }`, `doğrula(koşul, [mesaj])` (`dogrula`), `eşit_mi(beklenen, gerçek, [mesaj])` (`esit_mi`)

Dilin kuralları örnekleriyle birlikte [`docs/DIL_TANIMI.md`](docs/DIL_TANIMI.md) dosyasında tanımlanmıştır; her kuralın `testler/` klasöründe çalıştırılabilir bir testi vardır (`tr test testler`).

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

### A. Düşük Seviye Matris ve Otomatik Türev
- `matris([[1, 2], [3, 4]])`, `sıfırlar(satır, sütun)`, `birler(satır, sütun)`, `birim_matris(n)`, `rastgele_matris(satır, sütun, tohum: 42)`
- `A @ B` (Çok çekirdekli matris çarpımı), `devrik(A)` (veya `A.devrik`, `A.şekil`)
- `tensör(3.0, türev_izle: doğru)`, `geri_yayılım(y)`, `x.gradyan`, `x.değer`
- Aktivasyonlar ve Kayıp: `relu(x)`, `sigmoid(x)`, `tanh(x)`, `softmax(x)`, `kayıp_mse(tahmin, gerçek)`

### B. Yüksek Seviye Hazır Yapay Sinir Ağı + Kaydet/Yükle
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

## 5. TR Hata Ayıklayıcı

TR'nin hem **terminalde** (`tr debug dosya.tr`, kabukta `/h`) hem de **tarayıcıda** (`tr debug --web dosya.tr`) çalışan bir hata ayıklayıcısı vardır. VS Code'da hata ayıklarken (`tr dap`) değişkenler yandaki **İzleme Masası** panelinde tek tabloda görünür; listeler tip sütununda `Tablo [3×3]` gibi etiketlenir ve adına tıklanınca tablo olarak açılır (ayrıntılar: `vscode_eklentisi/README.md`).

1. **Kod İçi Kesme Noktası (`kesme_noktası`):**
   - Koşulsuz: `kesme_noktası`
   - Koşullu: `kesme_noktası eğer bakiye < 0`
2. **Adımlama ve Kontrol Komutları:**
   - `f5` (`adım` veya boş `Enter`): Tek adım ilerle (fonksiyonun içine girer)
   - `f6` (`geç`): Üstten geç (fonksiyonu tek adımda çalıştırır)
   - `f7` (`dön`): Mevcut fonksiyondan çıkana kadar çalıştır
   - `f8` (`devam`): Bir sonraki kesme noktasına ya da izleme noktasına kadar devam et
3. **Canlı Değişken Değiştirme (✏️):**
   - `değiştir vergi_orani = 0.20` (Programı durdurmadan çalışma anında değişkenin değerini değiştirir)
4. **Satıra Zıplama:**
   - `zıpla 25` (Çalışma okunu geçmiş veya gelecek bir satıra taşır; kodu baştan başlatmadan tekrar deneme imkanı verir)
5. **Tablo Görünümü ve İzleme Noktası:**
   - `tablo siparisler` (Liste, Sözlük veya Yapay Zeka Matrislerini tablo görünümünde açar)
   - `izle bakiye` veya `izle bakiye < 0` (Değişken değiştiğinde veya koşul sağlandığında otomatik durur)
   - `yığın` (Aktif fonksiyon çağrı yığınını gösterir)

---

## 6. Görsel Arayüz ve Web Sunucu Motoru

### A. Görsel Arayüz Motoru (`arayüz_oluştur` / `pencere_oluştur`)
```tr
değişken ekran = arayüz_oluştur("Kurumsal ERP Kokpiti", genişlik: 1100, yükseklik: 780)
ekran.kart_ekle("Toplam Ciro", "1.450.000 TL", "+%18 Artış", "yeşil")
ekran.grafik_ekle("Aylık Satışlar", ["Ocak", "Şubat", "Mart"], [420, 510, 520], "çubuk")
ekran.tablo_ekle("Sipariş Tablosu", siparis_listesi)
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

**Güvenlik:** Web sunucusu ve görsel arayüz yalnızca bu bilgisayardan (127.0.0.1) erişilebilir ve tarayıcıda açık başka web sitelerine karşı korunur:
- Başka sitelere okuma izni (CORS) verilmez.
- Başka bir siteden gelen veri değiştiren istekler (POST, PUT, DELETE) reddedilir.
- `localhost` dışındaki Host başlıkları reddedilir.
- İstek gövdesi en fazla 10 MB olabilir.
- `istek["veri"]` yalnızca istek `Content-Type: application/json` ile gelirse sözlük/liste olarak çözülür; aksi hâlde metindir.
- İşleyicinin döndürdüğü metin yalnızca tam bir HTML belgesiyse (`<!DOCTYPE html>` ya da `<html>` ile başlıyorsa) sayfa olarak sunulur; diğer metinler düz metin olarak gönderilir.

---

## 7. Yerleşik Türkçe SQL Veritabanı ve Listeyi Tablo Gibi Sorgulama

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

// 2. Bellekteki bir listeyi tablo gibi sorgulama
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



---

## 9. Test Yazma ve Çalıştırma (`tr test`)

Testler kodun yanına `test "ad" { ... }` bloklarıyla yazılır. Dosya normal çalıştırıldığında (`tr dosya.tr`) test blokları atlanır; yalnızca `tr test` çalıştırır.

```tr
// hesap_test.tr
fonksiyon topla(a, b) { sonuç a + b }

test "toplama doğru" {
    doğrula(topla(2, 2) == 4)
}

test "eşitlik" {
    eşit_mi(5, topla(2, 2))   // beklenen önce, gerçek sonra
}
```

- `doğrula(koşul, [mesaj])` (ASCII: `dogrula`): koşul yanlışsa test kalır.
- `eşit_mi(beklenen, gerçek, [mesaj])` (ASCII: `esit_mi`): değerler `==` anlamında eşit değilse ikisini de tipiyle gösterir.
- `test` yalnızca deyim başında ve ardından bir metin gelince test başlatır; `değişken test = 5` gibi kullanımlar çalışmaya devam eder.
- Önce dosyanın kendisi çalışıp üst düzey tanımları oluşturur; sonra her test bu tanımların üzerinde **yeni bir kapsamda** çalışır. Kalan ya da hata veren test diğerlerini durdurmaz. (Üst düzey bir değişkene atama yapan test, sonrakilerin gördüğü değeri değiştirir.)

```text
> tr test .
📄 hesap_test.tr
  ✓ toplama doğru
  ✗ eşitlik — Satır 9: eşitlik başarısız: beklenen 5 (Tam), gerçek 4 (Tam)
       9 |     eşit_mi(5, topla(2, 2))   // beklenen önce, gerçek sonra

❌ 2 test, 1 geçti, 1 kaldı
```

`tr test dosya.tr` yalnızca o dosyayı, `tr test klasör` klasördeki (alt klasörler dahil, gizli klasörler hariç) tüm `*_test.tr` dosyalarını çalıştırır. Sözdizimi ya da tip hatası olan dosya "yüklenemedi" olarak sayılır. Bir test kalırsa ya da bir dosya yüklenemezse çıkış kodu 1'dir; böylece CI'da kapı olarak kullanılabilir.

---

## 10. Kod Biçimlendirme (`tr biçimle`)

`tr biçimle` (ASCII: `tr bicimle`) kodu tek bir standart biçime getirir:

- 4 boşluk girinti; `{` her zaman aynı satırda, `değilse` / `yakala` kapanan `}` ile aynı satırda,
- ikili operatörlerin iki yanında ve virgülden sonra birer boşluk (aralıklar bitişik: `1..=10`; sözlükler `{"a": 1}`, sondaki bloklar `{ o > 2 }`),
- satır sonunda boşluk yok, art arda en fazla bir boş satır, dosya sonunda tek satır sonu.

**Yorumlar korunur**, metin sabitlerine bayt bayt dokunulmaz ve satır yapısı (hangi kodun hangi satırda olduğu) değişmez. Biçimlendirici sonucu yeniden ayrıştırıp özgün kodla karşılaştırır: anlam değişecek olursa dosyaya **dokunmaz** ve bunu bildirir. Sözdizimi hatası olan dosyalar biçimlenmez. Aynı dosyayı iki kez biçimlemek bir kez biçimlemekle aynıdır.

```text
> tr biçimle --kontrol .
biçimlenmeli: hesap.tr
3 dosyadan 1 tanesi biçimlenmeli. Düzeltmek için: tr biçimle <dosya|klasör>
> tr biçimle .
biçimlendi: hesap.tr
3 dosya incelendi, 1 dosya biçimlendi.
```
