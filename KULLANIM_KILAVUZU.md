# 🇹🇷 TR (`.tr`) Programlama ve Yapay Zeka Dili — Kullanım Kılavuzu

## 1. Hızlı Başlangıç Komutları

Yorumlayıcıyı proje kökünde derleyin:

```bash
GOAMD64=v3 go build -o tr.exe ./cmd/tr          # bash
$env:GOAMD64="v3"; go build -o tr.exe ./cmd/tr   # PowerShell
```

2020 sonrası x86-64 işlemciler (AVX2/FMA) hedeflenir; daha eski ya da bazı düşük güçlü işlemciler için `GOAMD64=v1` ile derleyin; ARM (arm64) bundan etkilenmez. Eğitim ve sunucu derlemelerinde v3, matris çarpımını v1 derlemeye göre çok hızlandırır (512×512 çarpımda yaklaşık 8–14 kat; bkz. `kiyas/README.md`).

| Komut | Açıklama |
| :--- | :--- |
| `.\tr.exe` | Etkileşimli Türkçe TR kabuğunu (REPL) başlatır (`/h` ile hata ayıklayıcı açılır) |
| `.\tr.exe dosya.tr` | `.tr` kaynak dosyasını derler ve anında çalıştırır (`.\tr.exe çalıştır dosya.tr` ile aynı) |
| `.\tr.exe hata_ayıkla dosya.tr` | Terminalde etkileşimli TR Hata Ayıklayıcıyı başlatır |
| `.\tr.exe kontrol dosya.tr` | Kodu çalıştırmadan sözdizimi ve akıllı tip kontrolü yapar |
| `.\tr.exe derle dosya.tr` | `.tr` dosyasını tek başına çalışan bağımsız **`.exe`** programına dönüştürür |
| `.\tr.exe test [dosya.tr \| klasör]` | Test bloklarını çalıştırır; klasör verilirse (varsayılan: bulunulan klasör) altındaki tüm `*_test.tr` dosyalarını tarar. Bir test bile kalırsa çıkış kodu 1'dir (bkz. §9) |
| `.\tr.exe biçimle dosya.tr` | Kodu standart biçime getirip dosyaya yazar; klasör verilirse içindeki tüm `.tr` dosyalarını biçimler (bkz. §10) |
| `.\tr.exe biçimle --kontrol dosya.tr` | Dosyayı değiştirmeden biçimlenmesi gerekip gerekmediğini söyler; gerekiyorsa çıkış kodu 1'dir |
| `.\tr.exe yardım` / `-h`, `.\tr.exe sürüm` / `-v` | Yardımı ve sürümü gösterir |

Her komutun tek, Türkçe bir adı vardır; Türkçe karaktersiz ya da İngilizce yazımlar (`calistir`, `bicimle`, `debug`, `--help` ...) çalışmaz, hata mesajı asıl komutu önerir. Kısa bayraklar `-h`, `-v`, `-k` simgedir.

Öğrenmeye `ornekler/` klasöründeki numaralı örneklerle başlayın (`01_merhaba.tr` … `09_web_veritabani.tr`; liste ve açıklamalar `README.md`'nin "Örnekler" bölümünde).

---

## 2. Anahtar Kelimeler (Çift Modlu: Türkçe & ASCII)

- **Değişken & Sabit:** `değişken` (`degisken`), `:=`, `sabit`
- **Çoklu Atama & Takas:** `değişken a, b = 10, 20`, `a, b = b, a`, `ad, soyad = ["Mustafa", "Kemal"]`
- **Fonksiyon & Dönüş:** `fonksiyon`, `sonuç` (`sonuc`), `(x) => x * 2`
- **Koşul & Eşleştirme:** `eğer` (`eger`), `değilse eğer`, `değilse` (`degilse`), `eşle` (`esle`), `varsayılan` (`varsayilan`)
- **Döngüler & Liste Üreteci:** `döngü x içinde liste` (`dongu x icinde liste`), `1..10`, `1..=10`, `iken`, `çık` (`cik`), `devam`, `[x * x döngü x içinde 1..=10 eğer x % 2 == 0]`
- **Mantıksal & Boş:** `doğru` (`dogru`), `yanlış` (`yanlis`), `yok`, `ve` (`&&`), `veya` (`||`), `değil` (`!`), `??`, `?.`
- **Tipler:** `Tam`, `Ondalık`, `Metin`, `Mantık`, `Liste`, `Sözlük`, `Tensör` (matrisler 2 boyutlu tensördür)
- **Yapı:** `yapı` (`yapi`), `kendi` (kalıtım yoktur)
- **Hata, Ertele, Kesme Noktası, Modül & Eşzamanlılık:** `dene`, `yakala`, `fırlat` (`firlat`), `ertele`, `kesme_noktası` (koşullu: `kesme_noktası eğer koşul`), `aktar "yol"`, `aktar "yol" olarak ad`, `eşzamanlı` (`eszamanli`), `bekle(saniye)`, `bekle_hepsi()`
- **Test:** `test "ad" { ... }`, `doğrula(koşul, [mesaj])` (`dogrula`), `eşit_mi(beklenen, gerçek, [mesaj])` (`esit_mi`)
- **Sayılar:** `42`, `1_000_000`, `3.14` ve bilimsel yazım `1e-5`, `3.2E+4`, `2e10` (bilimsel yazım her zaman `Ondalık` verir; `epsilon: 1e-5` gibi küçük değerler için kullanışlıdır).
- **Kaldırılan eş anlamlılar:** Her işin tek yazımı vardır. `işlev`, `döndür`, `kır`, `sınıf` ve `aktar ... as` kaldırıldı; bu sözcükler artık sıradan adlardır. Yerlerine `fonksiyon`, `sonuç`, `çık`, `yapı` ve `olarak` yazın; eski yazımla yazılan kodda hata mesajı doğrusunu önerir (`'işlev' TR'de anahtar kelime değil; 'fonksiyon' yazmak mı istediniz?`). Aynı kural yerleşik fonksiyonlar için de geçerlidir: her yerleşiğin tek adı (ve varsa Türkçe karaktersiz karşılığı, ör. `sırala` / `sirala`) vardır; eski eş adlarla yazılan kodda hata mesajı asıl adı önerir. Kaldırılanların tam listesi `docs/DIL_TANIMI.md` 2.9a'dadır.

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
- `matris([[1, 2], [3, 4]])`, `sıfırlar(satır, sütun)`, `birler(satır, sütun)`, `birim_matris(n)`, `rastgele_matris(satır, sütun, tohum: 42)`: hepsi 2 boyutlu bir **tensör** döndürür (ayrı bir matris tipi yoktur; ayrıntılar §4.D)
- `A @ B` (Çok çekirdekli matris çarpımı), `devrik(A)` (veya `A.devrik()`), `A.şekil` (`[satır, sütun]`)
- `tensör(3.0, türev_izle: doğru)` (tek sayı 0 boyutlu tensör verir), `geri_yayılım(y)`, `x.gradyan`, `x.değer` (`x.gradyan` da bir tensördür; sayısını `x.gradyan.değer` verir)
- Aktivasyonlar ve Kayıp: `relu(x)`, `sigmoid(x)`, `tanh(x)`, `softmax(x)`, `kayıp_mse(tahmin, gerçek)`

### B. Yüksek Seviye Hazır Yapay Sinir Ağı + Kaydet/Yükle
```tr
değişken model = sinir_ağı([2, 8, 1], aktivasyon: "tanh", tohum: 42)
model.eğit(X_veri, Y_hedef, devir: 800, öğrenme_oranı: 0.15)
model.kaydet("modelim.json")

değişken hazır_model = model_yükle("modelim.json")
değişken tahmin = hazır_model.tahmin_et(X_veri)
```

### C. Veri Bilimi, Vektör Benzerlik, Web ve Dış Köprüler
- İstatistik: `ortalama(liste)`, `standart_sapma(liste)`, `ortanca(liste)`, `normalize_et(liste)`
- Vektör: `nokta_çarpım(v1, v2)`, `kosinüs_benzerliği(v1, v2)`, `öklid_uzaklığı(v1, v2)`
- Veri, Web & Köprüler: `csv_oku(yol_veya_metin)`, `json_çöz(metin)`, `json_metin(değer)`, `web_al(url)`, `web_gönder(url, veri)`, `yz_sor(istem)`
- `csv_oku`: argüman var olan bir dosyanın yoluysa dosya okunur. `.csv` ile biten ya da virgül içermeyip `/` veya `\` içeren tek satırlık bir argüman yol sayılır; öyle bir dosya yoksa "dosya bulunamadı" hatası verilir. Bunların dışındaki argüman CSV metninin kendisi olarak ayrıştırılır (`csv_oku("ad,yaş\nAli,30")`). Göreli yollar önce çalışma klasörüne, orada yoksa programın bulunduğu klasöre göre çözülür. Sayı görünen hücreler sayıya çevrilir.
- `web_al` / `web_gönder`: 2xx dışı bir yanıt, durum kodu ve gövdenin kısa bir özetiyle hata verir (`web_al: ... adresi 404 Not Found durum kodu döndürdü: ...`; `dene/yakala` ile yakalanır). Yanıt gövdesi en çok 50 MB olabilir, istek 30 saniyede zaman aşımına uğrar. JSON yanıt sözlük/liste olarak, diğerleri metin olarak döner.

### D. Tensörler ve Otomatik Türev
Tensör, n boyutlu bir sayı dizisidir (veri türü varsayılan olarak `float32`; `bf16` ve `fp16` de seçilebilir, bkz. **Veri türleri**); yapay zekâ modellerinin ağırlıkları, girdileri ve ara sonuçları tensördür. Her işlem hesap çizgesine kaydedilir; `geri_yayılım(kayıp)` tüm gradyanları hesaplar. Çalıştırılabilir örnekler: `ornekler/05_tensor_ve_turev.tr`, `ornekler/06_transformer.tr`, `ornekler/07_karakter_modeli.tr`.

- **Oluşturma:** `tensör(veri, türev_izle: doğru)` (veri: iç içe liste, tek sayı ya da tensör), `sıfırlar([2, 3])`, `birler([2, 3])`, `rastgele_normal([2, 3], tohum: 42)`. Şekil **liste** olarak verilir; `sıfırlar(2, 3)` gibi iki tam sayı da aynı 2 boyutlu tensörü verir. `matris(...)`, `birim_matris(n)`, `rastgele_matris(...)` da 2 boyutlu tensör döndürür. `türev_izle` varsayılanı her zaman (tek sayıda da) `yanlış`tır: `tensör(3.0)` 0 boyutlu (skaler) bir tensördür (`.şekil` `[]`, `.değer` `3.0`, `.gradyan` `yok`); türevi izlenecekse `tensör(3.0, türev_izle: doğru)` yazın.
- **Sayı hassasiyeti:** Tensör hesapları `float32` ile (yaklaşık 7 anlamlı basamak) yapılır; `Ondalık` ise 64 bitliktir. Bu yüzden `(tensör([1.0]) / 3).değer[0]` `0.33333334` verir ve `1 / 3`e eşit değildir; `tensör([16777217.0])` 16777216 olarak saklanır. Kesin karşılaştırma yerine `yuvarla(...)` ya da küçük bir tolerans kullanın.
- **Veri türleri:** Her tensörün bir türü vardır: `"float32"` (varsayılan), `"bf16"` (bfloat16: float32 ile aynı aralık, ~3 anlamlı basamak) ya da `"fp16"` (IEEE yarım hassasiyet: en büyük değer 65504, ~3 anlamlı basamak). 16 bitlik türler belleği yarıya indirir. Tür, oluşturmada `tür:` ile seçilir: `tensör(veri, tür: "bf16")`, `sıfırlar([2, 3], tür: "fp16")`, `birler([4], tür: "bf16")`, `rastgele_normal([2, 3], tohum: 1, tür: "bf16")`; değerler en yakın gösterilebilir değere yuvarlanır (`tensör([1.001], tür: "bf16").değer` `[1.0]`). Tensörden oluştururken `tür:` verilmezse kaynağın türü korunur. `.tür` türü metin olarak verir (`"bf16"`); `.türe("fp16")` dönüştürülmüş bir kopya verir ve **türevlenebilir**dir (gradyan kaynak tensöre kendi türünde akar; tür aynıysa tensörün kendisi döner). İşlemler içte en az float32 ile biriktirir ve sonucu bir kez yuvarlar; işlenenler aynı türdeyse sonuç o türde, farklıysa `float32`'dir. 16 bitlik tensörlerin gradyanları `float32` tutulur. Bilinmeyen tür adı yol gösteren bir hatadır: `tensör: bilinmeyen veri türü "bfloat16"; geçerli türler: "float32", "bf16", "fp16". Bunu mu demek istediniz: "bf16"?`
- **Karışık hassasiyet:** `karışık_hassasiyet("bf16") { ... }` bloğu (sondaki blok yazımıyla; yeni anahtar kelime değildir) içinde `@` matris çarpımları girdilerini `bf16`'ya çevirir ve float32 biriktirmeyle **float32** sonuç verir; normalizasyon (`katman_normu`, `rms_normu`), `softmax`/`log_softmax`, `çapraz_entropi` ve `toplam`/`ortalama` 16 bitlik girdiyi float32'ye açarak hesaplar. Ağırlıkların kendisi değişmez (dönüşüm türevlenebilir bir kopyadır), bu yüzden `geri_yayılım` ve iyileştirici float32 ağırlıklarla çalışır. Blok son ifadesinin değerini döndürür: `değişken kayıp = karışık_hassasiyet("bf16") { çapraz_entropi(ileri(x), y) }`. `"fp16"` de verilebilir (65504'ü aşan değerler sonsuza taşar; kayıp ölçekleme yoktur); `"float32"` iç içe bir blokta karışık hassasiyeti kapatır. Bloklar iç içe kurulabilir; blok bitince (hata ile bitse de) önceki tür geri gelir. Durum **göreve bağlıdır**: bloğun içinde kilidi bırakan bir çağrı (`bekle(...)`, ağ, dosya) sırasında çalışan başka bir `eşzamanlı` görev ya da web isteği bloktan etkilenmez; blok içinde başlatılan bir `eşzamanlı` görev de float32 ile başlar (gerekirse kendi bloğunu kurar). Hazır `sinir_ağı(...)` modeli bu bloktan etkilenmez. CPU'da bf16 hız kazandırmaz (dönüşüm maliyeti nedeniyle biraz yavaştır); amacı bellek/bant genişliği tasarrufu ve 16 bitlik donanıma hazırlıktır.
- **Eski matris özellikleri:** `.boyut` → `.şekil`, `.satır` / `.sütun` → `.şekil[0]` / `.şekil[1]`, `.T` → `.devrik()`, `.toplam` / `.ortalama` → `toplam(A)` / `ortalama(A)` (0 boyutlu tensör; sayısı `.değer`), `.liste` → `.değer`. Eski adlar kaldırıldı; kullanılırsa hata mesajı karşılığını önerir (`Bunu mu demek istediniz: 'şekil'?`).
- **Özellik ve metotlar:** `.şekil`, `.değer` (iç içe liste; skalerde tek sayı; elemanlar Ondalık, tam sayılı tensörlerde — `maks_indeks` sonucu, veri kümesinden gelen `girdi`/`hedef` — `Tam`; float32 değerler en kısa gösterimleriyle verilir, `tensör([0.1]).değer[0] == 0.1` doğrudur), `.gradyan` (yalnızca `türev_izle: doğru` ile oluşturulan yaprak tensörlerde; bir işlemin sonucu olan tensörde açıklayıcı hata verir), `.cihaz`, `.şekillendir([4, 6])` (bir eksen `-1` olabilir), `.devrik()`, `.eksenleri_sırala([0, 2, 1])`, `.dilim(eksen, baş, son)` (son hariç, negatifler sondan sayar), `.cihaza("cpu")`. Bu derlemede GPU arka ucu yoktur; `.cihaza("gpu")` açık bir hata verir.
- **İşleçler:** `@` (matris çarpımı; 1 boyutlu işlenen vektör sayılır, 3+ boyutlu toplu çarpım), `+ - * /` eleman bazlı ve **yayınlamalıdır** (eksenler sağdan hizalanır; eşit ya da 1 olan eksenler uyumludur). Sayı iki yanda da durabilir: `2 * a`, `a - 1`, `1 - a`, `a / 2`, `1 / a`. Tekli eksi (`-a`) ve sayı üssü (`a ** 2`) de vardır; hepsi türev izler. İki işlenen de genişletilirse (ör. `[4, 1]` ile `[4]` → `[4, 4]`; çoğunlukla bir şekil hatası) standart hataya her işlem ve şekil çifti için bir kez `Uyarı: [4, 1] ile [4] yayınlanarak [4, 4] üretildi (iki taraf da genişletildi); ...` yazılır; program durmaz. `%` tensörlerde tanımlı değildir.
- **Karşılaştırma ve maske:** `< > <= >=` bir sayıyla ya da **aynı şekilli** tensörle eleman bazlı karşılaştırır ve türev izlemeyen 0/1 maskesi verir: `a > 0`, `a < b`. `==` ve `!=` ise tensörleri bütün olarak karşılaştırıp tek bir `Mantık` verir: şekilleri ve tüm elemanları aynıysa eşittir (`tensör([1.0, 2.0]) == tensör([1.0, 2.0])` doğru, `[2]` ile `[1, 2]` şekilli tensörler eşit değildir). 0 boyutlu tensör bir sayıyla da karşılaştırılabilir: `tensör(7.0) == 7`, `A[1][0] == 3.0` ve `toplam(t) == 10` doğrudur; en az bir ekseni olan tensör (`tensör([7.0])`) sayıya eşit değildir.
- **İndeksleme:** `t[i]` ilk eksenin i. elemanını verir (bir eksen eksilir; negatif indeks sondan sayar), `t[baş..son]` ilk eksende dilim verir (`son` hariç); ikisi de veriyi kopyalamayan, türev izleyen görünümlerdir ve zincirlenebilir (`t[0][1]`). Sınır dışı indeks hatadır.
- **İndeksle atama:** `t[i] = d`, `t[i][j] = d`, `t[baş..son] = d` ve `t[i] += d` veriyi yerinde değiştirir. `d` bir sayıysa hedefin tüm elemanlarına yazılır (`t[0] = 0` ilk satırı sıfırlar); liste ya da tensörse şekli hedefin şekliyle aynı olmalıdır (`t[0] = [1, 2, 3]`). Türev izlenen bir tensöre (ör. bir ağırlığa) yalnızca `türevsiz { ... }` içinde atanabilir.
- **Döngü:** `döngü x içinde t`, `döngü i, x içinde t` ve `[ifade döngü x içinde t]` tensörü ilk ekseni boyunca gezer: 2 boyutlu tensörde `x` sıradaki satırdır (veriyi paylaşan 1 boyutlu tensör; `x[0] = 9` tensörü değiştirir), 1 boyutlu tensörde sıradaki sayıdır (`Ondalık`; tam sayılı tensörde `Tam`). Örnek: `[toplam(satır).değer döngü satır içinde A]` satır toplamlarını liste olarak verir. 0 boyutlu tensör üzerinde döngü kurulamaz.
- **Yazdırma:** `yazdır(t)` şekli başlıkta gösterir (`Tensör[2×3]`; float32 dışındaki türlerde tür de: `Tensör[2×3] bf16`, skalerde `Tensör(2.5000) fp16`), değerleri 4 basamakla yazar; 1000'den çok elemanda her eksenin yalnızca ilk ve son 3 elemanı gösterilir (`…`). Sıfır olmayan bir değer 0.0001'den küçükse ya da bir değer 1e8 ve üzerindeyse tüm hücreler bilimsel gösterimle (`1.0000e-05`) yazılır. Sayı olmayan değerler `Tanımsız`, `Sonsuz`, `-Sonsuz` olarak görünür.
- **Doğruluk değeri:** `eğer t { ... }` tek elemanlı tensörde eleman 0'dan farklıysa doğrudur, boş tensör yanlıştır. Çok elemanlı tensörün doğruluk değeri belirsizdir; `eğer maks(t) > 0` gibi bir indirgeme kullanın (şimdilik hata vermez, doğru sayılır).
- **Matematik:** `relu`, `sigmoid`, `tanh`, `gelu`, `karekök`, `üstel`, `log`, `sinüs`, `kosinüs` (radyan; dönel konum gömmesi gibi açılı hesaplar için, türevli), `softmax(x, eksen: -1)`, `log_softmax(x, eksen: -1)`. **Dönüş tipi kuralı:** tensör yalnızca tensör girdisinden çıkar (`matris(...)` de bir tensör olduğundan `relu(matris(...))` tensör verir). Sayı ya da liste verilirse sonuç girdinin biçimindedir: eski karşılığı olan adlarda (`relu`, `softmax`...) eski davranış geçerlidir; yeni adlar `gelu`, `üstel`, `log` sayıda Ondalık, listede aynı yapıda liste döndürür. `softmax` ve `log_softmax` listede yalnızca düz sayı listesi kabul eder; iç içe veri için `softmax(tensör(...), eksen: -1)` yazın.
- **İndirgemeler:** `toplam(x)`, `ortalama(x)`, `maks(x)`, `min(x)` tüm elemanları skaler (0 boyutlu) tensöre indirger; `eksen: 1` verilirse o eksen kaldırılır; `boyutu_koru: doğru` ile 1 uzunlukta korunur (`[B, T, D]` → `toplam(x, eksen: -1, boyutu_koru: doğru)` → `[B, T, 1]`, böylece `x / toplam(x, eksen: -1, boyutu_koru: doğru)` yayınlanır; eksen verilmezse bütün eksenler 1 olur). `toplam(x, 1)` / `ortalama(x, 1)` konumlu ekseni de kabul eder; `maks`/`min` için eksen **yalnızca adlı** verilir (`maks(x, 0)` eleman bazlı sınırla karışmasın diye hatadır). `toplam([a, b])` / `ortalama([a, 1, b])` listedeki tensörleri yayınlamalı toplamla katlar (türev korunur); iç içe sayı listesi indirgenemez, önce `tensör(...)` ile çevirin. `eksen:` yalnızca tensörlerde geçerlidir.
- **Sinir ağı:** `gömme(tablo, indeksler)` (tablo `[V, D]`; sonuç `indeks şekli + [D]`), `çapraz_entropi(çıktı, hedef)` (çıktı `[..., C]`, hedef sınıf indeksleri; negatif hedefler yok sayılır), `katman_normu(x, ağırlık, sapma, epsilon: 0.00001)`, `rms_normu(x, ağırlık, epsilon: 0.00001)` (ağırlık/sapma `yok` olabilir).
- **Birleştirme ve bölme:** `birleştir([a, b, c], eksen: -1)` tensörleri bir eksen boyunca birleştirir (diğer eksenler eşit olmalı; türevli). `x.böl(3, eksen: -1)` tensörü o eksende 3 eşit parçaya böler ve parçaların listesini verir (görünümler, türevli): `değişken q, k, v = (x @ Wqkv).böl(3)`.
- **Dikkat:** `dikkat(q, k, v, nedensel: doğru, pencere: 0, maske: yok)` birleşik ölçekli nokta çarpımı dikkatidir; `softmax(q @ k.devrik() / √dh + maske) @ v` ile aynı sonucu verir ama `T×T` puan matrisini hiç tutmaz (puanlar bloklar hâlinde, çevrimiçi softmax ile hesaplanır; bellek T ile doğrusal) ve tek bir türev düğümüdür (geri yayılım puanları yeniden hesaplar). `q` `[..., H, T, dh]`, `k`/`v` `[..., Hkv, Tk, dh]` şeklindedir; `Hkv < H` gruplu sorgu dikkatidir (`H`, `Hkv`'nin katı; k/v kopyalanmaz). Ölçek `1/√dh`'dir. `nedensel: doğru` (varsayılan) gelecekteki konumları kapatır; `Tq < Tk` iken (önbellekli üretimde yalnızca yeni token sorgulanırken) hizalama sondandır: sorgu `i`, `Tk - Tq + i`'ye kadar görür. `pencere: 128` her sorguyu son 128 konuma sınırlar (yerel dikkat katmanı; pencere dışındaki bloklar hiç hesaplanmaz). `maske:` isteğe bağlı 0/1 tensörüdür (1 = bakılır; `[Tq, Tk]` gibi yayınlanabilir şekiller kabul edilir). `karışık_hassasiyet("bf16")` bloğunda girdiler bf16'ya çevrilir, sonuç float32'dir. Hiçbir konuma bakamayan satırın çıktısı 0'dır.
- **Örnekleme:** `örnekle(logit, sıcaklık: 1.0, en_iyi_k: 0, tohum: yok)` son eksendeki `softmax(logit / sıcaklık)` dağılımından bir token kimliği örnekler; sonuç son ekseni düşürülmüş, tam sayılı bir tensördür (`.değer` `Tam`): `[B, V]` logitten `[B]` kimlik. `en_iyi_k: 40` yalnızca en olası 40 token arasından seçer; `sıcaklık: 0` en olasıyı verir (`maks_indeks` ile aynı). Aynı `tohum` aynı sonucu verir; tohumsuz çağrılar her seferinde farklıdır. Türev izlemez.
- **Model parametreleri:** `parametreler(model)` bir yapı nesnesini (iç içe yapılar, listeler ve sözlükler dahil) gezip türev izleyen yaprak tensörleri sabit sırayla (alan tanım sırası) bir liste olarak verir; aynı tensör bir kez yer alır: `AdamW(parametreler(model), öğrenme_oranı: 0.001)`. `parametreler(model, adlarıyla: doğru)` yol adı → tensör sözlüğü verir (`"bloklar.0.dikkat.Wq"`; liste indeksleri sayı, sözlük anahtarları anahtarın kendisi); bu sözlük doğrudan kaydedilir: `tensör_kaydet("model.trt", parametreler(model, adlarıyla: doğru))`.
- **Türev:** `geri_yayılım(kayıp)` (kayıp tek elemanlı olmalı), `x.gradyan` (gradyanlar **birikir**: her `geri_yayılım` yeni gradyanı öncekine ekler, bu yüzden eğitim döngüsünde her adımda önce `op.sıfırla()` çağrılır), `türevsiz { ... }` bloğu türev izlemeden çalışır ve bloğun değerini (son ifade ya da `sonuç`) döndürür, `türevsiz(x)` tensörü hesap çizgesinden ayırır.
- **İndeks:** `maks_indeks(x, eksen: -1)` en büyük elemanın indeksini verir (eşitlikte ilki). Sonuç her zaman tam sayı değerli, türev izlemeyen bir tensördür (o eksen kaldırılır; 1 boyutlu girdide skaler); doğrudan `gömme`/`çapraz_entropi` indeksi olarak kullanılabilir. Tensör tam sayılı işaretlidir: ondalıksız yazdırılır ve `.değer` `Tam` (ya da `Tam` listesi) verir, böylece `harfler[maks_indeks(x).değer[0]]` gibi liste indekslemede doğrudan kullanılır. Bu tensörle yapılan işlemlerin sonucu (ör. `i * 2`) yine Ondalık değerlidir.
- **Gradyan kırpma:** `gradyan_kırp(parametreler, en_fazla: 1.0)` gradyanların toplam L2 normunu `en_fazla` ile sınırlar ve kırpma öncesi normu döndürür; `geri_yayılım` ile `op.adım()` arasında çağrılır.
- **Kaydetme/yükleme:** `tensör_kaydet("model.trt", {"w": w, "b": b})` ve `değişken p = tensör_yükle("model.trt")` (isteğe bağlı `türev_izle: doğru`). Göreli yollar `dosya_oku`/`dosya_yaz` ile aynı kuralla çözülür (önce çalışma klasörü, orada yoksa programın klasörü; `model.kaydet`/`model_yükle` için de aynı); dosya varsa üzerine yazılır. Dosya hataları Türkçedir (`tensör_yükle: "model.trt" yüklenemedi: böyle bir dosya ya da klasör yok (yolun yazımını denetleyin); bakılan yer: ...`). Biçim (sürüm 2, küçük sonlu): `TRTENSOR` (8 bayt), sürüm `uint16`, girdi sayısı `uint32`; her girdi için ad uzunluğu `uint32` + UTF-8 ad, veri türü kodu `uint8` (1 = float32, 2 = bf16, 3 = fp16), eksen sayısı `uint8`, her eksen `uint64`, satır öncelikli veri (float32 4 bayt, bf16/fp16 2 baytlık bit deseni; 16 bitlik tensörler 16 bit olarak yazılır ve aynı türde yüklenir); sonda tüm baytların CRC-32 değeri `uint32`. Sürüm 1 dosyaları (yalnızca float32) okunmaya devam eder. Bozuk, eksik, bilinmeyen sürümlü ya da başka türden dosya açık bir Türkçe hatayla reddedilir.
- **İyileştiriciler:** `AdamW(parametreler, öğrenme_oranı: 0.001, ağırlık_çürümesi: 0.01, beta1: 0.9, beta2: 0.999, epsilon: 0.00000001)`, `SGD(parametreler, öğrenme_oranı: 0.01, momentum: 0, ağırlık_çürümesi: 0)`; `.adım()` parametreleri günceller, `.sıfırla()` gradyanları sıfırlar. Parametreler doğrudan `tensör(..., türev_izle: doğru)` ile oluşturulmuş tensörler olmalıdır. İkisi de `ana_kopya: doğru` (varsayılan) seçeneğini alır: 16 bitlik (`bf16`/`fp16`) parametrelerin float32 ana kopyası tutulur, güncelleme ana kopyada yapılıp parametreye yuvarlanarak yazılır; böylece 16 bitlik aralıktan küçük adımlar kaybolmaz (`ana_kopya: yanlış` ile güncelleme doğrudan 16 bitlik parametrede yapılır). float32 parametreleri etkilemez; moment tahminleri her zaman float32'dir.

```tr
// y = 2·x - 1 doğrusunu öğren
değişken X = tensör([[0], [1], [2], [3]])
değişken Y = tensör([[-1], [1], [3], [5]])
değişken w = tensör([[0.0]], türev_izle: doğru)
değişken b = tensör([0.0], türev_izle: doğru)
değişken op = AdamW([w, b], öğrenme_oranı: 0.1, ağırlık_çürümesi: 0)

döngü adım içinde 0..500 {
    op.sıfırla()
    değişken fark = X @ w + b - Y
    geri_yayılım(ortalama(fark * fark))
    op.adım()
}
yazdır("w = {yuvarla(w.değer[0][0], 3)}, b = {yuvarla(b.değer[0], 3)}")   // w = 2.0, b = -1.0
yazdır(türevsiz { sonuç X @ w + b })                                       // Tensör[4×1] [...]
```

Şekil hataları şekli gösterir ve `dene/yakala` ile yakalanabilir: `[32, 1024] ile [512, 10] matris çarpımı yapılamaz; 1024 ≠ 512`. Fonksiyon, metot, işleç (`@`, `+`, `<`...) ve özellik hatalarının başında `Satır N:` bulunur: `Satır 3: [2, 2] ile [1, 3] matris çarpımı yapılamaz; 2 ≠ 1 (...)`.

> **Not (türevsiz ve eşzamanlılık):** Türevsiz kip süreç geneli bir sayaçtır. `türevsiz { ... }` bloğunun içinde yorumlayıcı kilidini bırakan bir çağrı (`bekle(...)`, ağ isteği, `girdi()` gibi) varsa, o sırada çalışan başka bir `eşzamanlı` görev de türevsiz çalışır ve hesapladığı değerler türev izlemez. Eğitim ve türevsiz çıkarımı aynı anda farklı görevlerde yürütmeyin ya da `türevsiz` bloklarına engelleyici çağrı koymayın.

VS Code İzleme Masası'nda tensörler tip sütununda `Tensör [2×3]` (float32 dışındaki türlerde `Tensör [2×3] bf16`) olarak görünür; 1 ve 2 boyutlu tensörler tablo olarak açılabilir. Variables panelinde tensörler ilk ekseni boyunca açılır (satırlar, onların içinde sayılar; 1000'den çok satırda ilk 1000 satır ve bir `…` satırı gösterilir).

### E. Veri ve Parçalayıcı
Büyük metin derlemleriyle model eğitmek için iki yerleşik nesne vardır: `tokenizer.json` dosyalarını okuyan **parçalayıcı** ve `.npy` token parçalarını okuyan **veri kümesi**. Uçtan uca örnek: `ornekler/08_veri_ile_egitim.tr`.

**Parçalayıcı**
- `değişken p = parçalayıcı_yükle("tokenizer.json")` yaygın `tokenizer.json` biçimini okur (BPE, WordPiece, WordLevel, Unigram modelleri; bayt düzeyi, Metaspace, BERT tarzı bileşenler). Desteklenmeyen bir bileşen türü, hangi bileşenin hangi türü olduğunu söyleyen bir hatayla bildirilir. Tip adı `Parçalayıcı`.
- `p.sözlük_boyutu` sözlükteki token sayısıdır; `p.token_kimliği("<|endoftext|>")` tokenın kimliğini verir, yoksa `yok`.
- `p.kodla(metin)` bir `Tam` listesi, `p.kodla([metin1, metin2])` listelerin listesini verir (liste tüm işlemci çekirdeklerinde paralel kodlanır, sıra korunur). Metnin içinde yazılı özel tokenlar (`"<|endoftext|>"`) tek bir kimlik olarak tanınır.
  - `özel_tokenlar: yanlış` son işleyicinin eklediği özel tokenları (`[CLS] … [SEP]`, `<s>` gibi) kapatır; varsayılan `doğru`dur.
  - `ikinci: metin` metin çiftini (soru + bağlam gibi) tek dizi olarak kodlar; liste verilirse `ikinci:` de aynı uzunlukta bir liste olmalıdır.
- `p.çöz(kimlikler)` metin verir; listelerin listesi ya da 2 boyutlu tensör verilirse metin listesi, 1 boyutlu tensör verilirse metin döner (ör. `.sonraki()` ile alınan `girdi` doğrudan çözülebilir). Varsayılan, başvuru kütüphanesindeki gibi **özel tokenları atlamaktır**; `özel_tokenlar: doğru` onları da yazar. Sözlükte olmayan kimlikler sessizce atlanır.
- Güvenlik: parçalayıcı dosyasındaki düzenli ifadeler adım bütçesiyle çalışır. Kötü niyetli bir metin aşırı geri izlemeye yol açarsa yarım sonuç yerine `dene/yakala` ile yakalanabilen bir "adım sınırını aştı" hatası oluşur. Üstel geri izlemeye açık (iç içe sınırsız niceleyicili) desenli dosyalar yüklenmez; daha hafif riskler yüklemede standart hataya dosya başına bir kez `Uyarı:` olarak yazılır.

**Veri kümesi**
- `değişken küme = veri_kümesi("veri/train")` klasördeki `.npy` parçalarını ad sırasıyla tanır (tek bir `.npy` dosyası da verilebilir). Parçalar tek boyutlu, küçük uçlu `uint16`, `uint32` ya da `int32` token dizileridir (standart `.npy` dosya biçimi). Klasörde ya da üst klasörde `meta.json` varsa parça token sayıları oradan okunur; dosya başlıkları açılışta okunmaz. Tip adı `VeriKümesi`.
- **Önbellek:** veri yavaş bir sürücüdeyse (ağ, akışlı bulut sürücüsü) `veri_kümesi(dizin, önbellek: "onbellek/veri", kota_gb: 20)` parçaları kullanılmadan önce yerel klasöre kopyalar; kota dolunca en uzun süredir kullanılmayan kopyalar silinir. `önbellek:` verilince `kota_gb:` **zorunludur** (disk alanını sessizce doldurmamak için); verilmezse hata, bu küme için gereken en küçük kotayı (aynı anda yerelde tutulan 8 parça × en büyük parça) söyler. Önbelleksiz okumada parçalar belleğe eşlenmez, her pencere ayrı okunur: yalnızca istenen baytlar sürücüden çekilir.
- `küme.örnekleyici(toplu: 32, bağlam: 1024, tohum: 1)` eğitim için rastgele pencereler üretir (tip `Örnekleyici`; `tohum` varsayılanı 0). Aynı tohum ve küme hep aynı toplu işleri verir. Her dönemde parça sırası karışır, her token bir kez görülür, hiçbir pencere parça sınırını aşmaz; örnekleyici dönemden döneme sonsuz üretir.
- `küme.sıralı(toplu: 32, bağlam: 1024)` doğrulama için parçaları sırayla okur (tip `SıralıOkuyucu`): ardışık pencereler `bağlam` kadar kayar, böylece her token bir kez hedef olur. Son toplu iş daha az satırlı olabilir; veri bitince `.sonraki()` `yok` döner.
- `.sonraki()` `[girdi, hedef]` listesi verir: ikisi de `[toplu, bağlam]` şeklinde, tam sayı değerli tensördür ve `hedef` girdinin bir kaydırılmışıdır. Doğrudan `gömme(E, girdi)` ve `çapraz_entropi(çıktı, hedef)`'e verilir: `değişken girdi, hedef = ö.sonraki()`. İkisi de tam sayılı işaretlidir: ondalıksız yazdırılır ve `.değer` `Tam` listesi verir, böylece liste indekslemede doğrudan kullanılır. Bu tensörlerle yapılan işlemlerin sonucu (ör. `girdi + 1`) Ondalık değerlidir.
- **Önden yükleme her zaman açıktır:** sonraki 4 toplu iş arka planda hazırlanır. Bu iş TR kodu çalıştırmaz, yorumlayıcı kilidini tutmaz; `.sonraki()` beklerken kilit bırakılır, böylece `eşzamanlı` görevler ilerler. Aynı okuyucuyu birden çok görev paylaşabilir (her toplu iş bir göreve gider).
- **Kaynaklar:** açık bir kapatma adı yoktur. Sıralı okuyucu bitince dosyalarını hemen kapatır; artık kullanılmayan (hiçbir değişkenin göstermediği) okuyucu ve kümeler çöp toplayıcı tarafından kapatılır; program bitince açık kalanların hepsi kapatılır (takılan bir ağ okumasında en çok birkaç saniye beklenir).
- Göreli yollar, dosya okuyan ve yazan bütün yerleşiklerde (`dosya_oku`, `dosya_yaz`, `aktar`, `parçalayıcı_yükle`, `veri_kümesi`, `csv_oku`, `veritabanı_aç`, `html_kaydet`) aynı kuralla çözülür: göreli bir yol önce çalışma klasörüne, orada yoksa programın bulunduğu klasöre göre çözülür ve dosyanın bulunduğu ilk yer kullanılır; dosya hiçbir yerde yoksa bulunacağı klasörün var olduğu ilk yer seçilir (düz bir `rapor.txt` için bu çalışma klasörüdür). Kural okuma ve yazma için aynıdır: programın yanındaki bir dosya okunup güncellenince yine oraya yazılır, yeni dosyalar çalışma klasöründe oluşur. Dosya bulunamazsa hata bakılan yerleri listeler.
- Hatalar Türkçe ve yol göstericidir (dosya yok, `.npy` değil, kota yetersiz, bağlam parçalardan uzun, adlı argüman yazım hatası...) ve `dene/yakala` ile yakalanır.

```tr
değişken p = parçalayıcı_yükle("tokenizer.json")
yazdır(p.kodla("Merhaba dünya"), p.sözlük_boyutu)
değişken küme = veri_kümesi("veri/train")
değişken ö = küme.örnekleyici(toplu: 8, bağlam: 256, tohum: 1)
değişken E = tensör(türevsiz(rastgele_normal([p.sözlük_boyutu, 64], tohum: 1) * 0.1), türev_izle: doğru)
değişken W = tensör(türevsiz(rastgele_normal([64, p.sözlük_boyutu], tohum: 2) * 0.1), türev_izle: doğru)
değişken op = AdamW([E, W], öğrenme_oranı: 0.01)
döngü adım içinde 0..100 {
    değişken girdi, hedef = ö.sonraki()
    op.sıfırla()
    değişken kayıp = çapraz_entropi(gömme(E, girdi) @ W, hedef)
    geri_yayılım(kayıp)
    op.adım()
}
değişken doğrulama = veri_kümesi("veri/_valid").sıralı(toplu: 8, bağlam: 256)
döngü doğrulama.sonraki() != yok { }   // bitince yok döner
```

VS Code İzleme Masası'nda bu nesneler tip sütununda `Parçalayıcı (40000 sözcük)`, `Veri kümesi (246 parça, 56,9 milyar token)`, `Örnekleyici (32×1024, dönem 0, 120 toplu iş)` gibi görünür; parçalayıcı (eklenmiş/özel tokenlar) ve veri kümesi (parçalar) tablo olarak açılabilir.

---

## 5. TR Hata Ayıklayıcı

TR'nin **terminalde** (`tr hata_ayıkla dosya.tr`, kabukta `/h`) çalışan bir hata ayıklayıcısı vardır. VS Code'da hata ayıklarken (`tr dap`) değişkenler yandaki **İzleme Masası** panelinde tek tabloda görünür; listeler tip sütununda `Tablo [3×3]` gibi etiketlenir ve adına tıklanınca tablo olarak açılır (ayrıntılar: `vscode_eklentisi/README.md`). Denemek için: `tr ornekler/04_hata_ayiklama.tr` (koşullu kesme noktasında durur; `değiştir vergi_orani = 0.20`, ardından `devam`).

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
   - `tablo siparisler` (Liste, Sözlük veya Tensörleri tablo görünümünde açar)
   - `izle bakiye` veya `izle bakiye < 0` (Değişken değiştiğinde veya koşul sağlandığında otomatik durur)
   - `yığın` (Aktif fonksiyon çağrı yığınını gösterir)

---

## 6. Görsel Arayüz ve Web Sunucu Motoru

Veritabanı, REST API ve arayüzü birlikte kullanan örnek: `ornekler/09_web_veritabani.tr`.

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

**Ağır işler sürerken istekler ve görevler:** Yorumlayıcı kilidi aynı anda yalnızca bir görevin TR kodu çalıştırmasına izin verir. Kilidi tutan görev, kilidi bekleyen başka bir görev varsa onu döngülerde düzenli aralıklarla ve her fonksiyon çağrısında devreder (bir deyim hiçbir zaman ortasından bölünmez). Böylece `api.arka_planda_başlat()` ile açılan sunucu, uzun bir eğitim ya da hesap döngüsü sürerken de isteklere yanıt verir; `eşzamanlı` görevler de döngünün bitmesini beklemeden ilerler.

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
- `yapi` $\rightarrow$ `yapı` ve metot şablonu oluşturur
- `sinir` $\rightarrow$ 3 satırlık yapay sinir ağı eğitim şablonu oluşturur
- `tensor` $\rightarrow$ Tensörlerle küçük bir eğitim döngüsü oluşturur (`tensör(..., türev_izle: doğru)`, `AdamW`, `iyileştirici.sıfırla()`, `geri_yayılım`, `iyileştirici.adım()`)



---

## 9. Test Yazma ve Çalıştırma (`tr test`)

Testler kodun yanına `test "ad" { ... }` bloklarıyla yazılır. Dosya normal çalıştırıldığında (`tr dosya.tr`) test blokları atlanır; yalnızca `tr test` çalıştırır. Örnek: `tr test ornekler/03_testler_test.tr`.

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

`tr biçimle` kodu tek bir standart biçime getirir:

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
