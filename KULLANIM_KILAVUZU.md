# 🇹🇷 TR (`.tr`) Programlama ve Yapay Zeka Dili — Kullanım Kılavuzu

## 1. Hızlı Başlangıç Komutları

Yorumlayıcıyı proje kökünde derleyin:

```bash
GOAMD64=v3 go build -o tr.exe ./cmd/tr          # bash
$env:GOAMD64="v3"; go build -o tr.exe ./cmd/tr   # PowerShell
```

2020 sonrası x86-64 işlemciler (AVX2/FMA) hedeflenir; daha eski ya da bazı düşük güçlü işlemciler için `GOAMD64=v1` ile derleyin; ARM (arm64) bundan etkilenmez. Eğitim ve sunucu derlemelerinde v3, matris çarpımını yaklaşık iki kat hızlandırır.

| Komut | Açıklama |
| :--- | :--- |
| `.\tr.exe` | Etkileşimli Türkçe TR kabuğunu (REPL) başlatır (`/h` ile hata ayıklayıcı açılır) |
| `.\tr.exe dosya.tr` | `.tr` kaynak dosyasını derler ve anında çalıştırır |
| `.\tr.exe debug dosya.tr` | Terminalde etkileşimli TR Hata Ayıklayıcıyı başlatır |
| `.\tr.exe kontrol dosya.tr` | Kodu çalıştırmadan sözdizimi ve akıllı tip kontrolü yapar |
| `.\tr.exe derle dosya.tr` | `.tr` dosyasını tek başına çalışan bağımsız **`.exe`** programına dönüştürür |
| `.\tr.exe test [dosya.tr \| klasör]` | Test bloklarını çalıştırır; klasör verilirse (varsayılan: bulunulan klasör) altındaki tüm `*_test.tr` dosyalarını tarar. Bir test bile kalırsa çıkış kodu 1'dir (bkz. §9) |
| `.\tr.exe biçimle dosya.tr` | Kodu standart biçime getirip dosyaya yazar; klasör verilirse içindeki tüm `.tr` dosyalarını biçimler (`bicimle` da olur, bkz. §10) |
| `.\tr.exe biçimle --kontrol dosya.tr` | Dosyayı değiştirmeden biçimlenmesi gerekip gerekmediğini söyler; gerekiyorsa çıkış kodu 1'dir |

---

## 2. Anahtar Kelimeler (Çift Modlu: Türkçe & ASCII)

- **Değişken & Sabit:** `değişken` (`degisken`), `:=`, `sabit`
- **Çoklu Atama & Takas:** `değişken a, b = 10, 20`, `a, b = b, a`, `ad, soyad = ["Mustafa", "Kemal"]`
- **Fonksiyon & Dönüş:** `fonksiyon`, `sonuç` (`sonuc`), `(x) => x * 2`
- **Koşul & Eşleştirme:** `eğer` (`eger`), `değilse eğer`, `değilse` (`degilse`), `eşle` (`esle`), `varsayılan` (`varsayilan`)
- **Döngüler & Liste Üreteci:** `döngü x içinde liste` (`dongu x icinde liste`), `1..10`, `1..=10`, `iken`, `çık` (`cik`), `devam`, `[x * x döngü x içinde 1..=10 eğer x % 2 == 0]`
- **Mantıksal & Boş:** `doğru` (`dogru`), `yanlış` (`yanlis`), `yok`, `ve` (`&&`), `veya` (`||`), `değil` (`!`), `??`, `?.`
- **Tipler:** `Tam`, `Ondalık`, `Metin`, `Mantık`, `Liste`, `Sözlük`, `Matris`, `Tensör`
- **Yapı:** `yapı` (`yapi`), `kendi` (kalıtım yoktur)
- **Hata, Ertele, Kesme Noktası, Modül & Eşzamanlılık:** `dene`, `yakala`, `fırlat` (`firlat`), `ertele`, `kesme_noktası` (`dur_noktası`, `durdurma_noktası`; koşullu: `kesme_noktası eğer koşul`), `aktar "yol"`, `aktar "yol" olarak ad`, `eşzamanlı` (`eszamanli`), `bekle(saniye)`, `bekle_hepsi()`
- **Test:** `test "ad" { ... }`, `doğrula(koşul, [mesaj])` (`dogrula`), `eşit_mi(beklenen, gerçek, [mesaj])` (`esit_mi`)
- **Sayılar:** `42`, `1_000_000`, `3.14` ve bilimsel yazım `1e-5`, `3.2E+4`, `2e10` (bilimsel yazım her zaman `Ondalık` verir; `epsilon: 1e-5` gibi küçük değerler için kullanışlıdır).
- **Kaldırılan eş anlamlılar:** Her işin tek yazımı vardır. `işlev`, `döndür`, `kır`, `sınıf` ve `aktar ... as` kaldırıldı; bu sözcükler artık sıradan adlardır. Yerlerine `fonksiyon`, `sonuç`, `çık`, `yapı` ve `olarak` yazın; eski yazımla yazılan kodda hata mesajı doğrusunu önerir (`'işlev' TR'de anahtar kelime değil; 'fonksiyon' yazmak mı istediniz?`).

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
- Veri, Web & Köprüler: `csv_oku(yol)`, `json_çöz(metin)`, `json_metin(değer)`, `web_al(url)`, `web_gönder(url, veri)`, `yz_sor(istem)`

### D. Tensörler ve Otomatik Türev
Tensör, n boyutlu (float32) bir sayı dizisidir; yapay zekâ modellerinin ağırlıkları, girdileri ve ara sonuçları tensördür. Her işlem hesap çizgesine kaydedilir; `geri_yayılım(kayıp)` tüm gradyanları hesaplar.

- **Oluşturma:** `tensör(veri, türev_izle: doğru)` (veri: iç içe liste ya da matris), `sıfırlar([2, 3])`, `birler([2, 3])`, `rastgele_normal([2, 3], tohum: 42)`. Şekil **liste** olarak verilir; `sıfırlar(2, 3)` gibi iki tam sayı eskisi gibi `Matris` döndürür. `türev_izle` varsayılanı listede/matriste `yanlış`tır; `tensör(3.0)` gibi tek sayı eski tek sayılık otomatik türev değerini verir.
- **Özellik ve metotlar:** `.şekil`, `.değer` (iç içe liste; skalerde tek sayı; elemanlar Ondalık, tam sayılı tensörlerde — `maks_indeks` sonucu, veri kümesinden gelen `girdi`/`hedef` — `Tam`; float32 değerler en kısa gösterimleriyle verilir, `tensör([0.1]).değer[0] == 0.1` doğrudur), `.gradyan` (yalnızca `türev_izle: doğru` ile oluşturulan yaprak tensörlerde; bir işlemin sonucu olan tensörde açıklayıcı hata verir), `.cihaz`, `.şekillendir([4, 6])` (bir eksen `-1` olabilir), `.devrik()`, `.eksenleri_sırala([0, 2, 1])`, `.dilim(eksen, baş, son)` (son hariç, negatifler sondan sayar), `.cihaza("cpu")`. Bu derlemede GPU arka ucu yoktur; `.cihaza("gpu")` açık bir hata verir.
- **İşleçler:** `@` (matris çarpımı; 1 boyutlu işlenen vektör sayılır, 3+ boyutlu toplu çarpım), `+ - * /` eleman bazlı ve **yayınlamalıdır** (eksenler sağdan hizalanır; eşit ya da 1 olan eksenler uyumludur). Sayı iki yanda da durabilir: `2 * a`, `a - 1`, `1 - a`, `a / 2`, `1 / a`. Tekli eksi (`-a`) ve sayı üssü (`a ** 2`) de vardır; hepsi türev izler. İki işlenen de genişletilirse (ör. `[4, 1]` ile `[4]` → `[4, 4]`; çoğunlukla bir şekil hatası) standart hataya her işlem ve şekil çifti için bir kez `Uyarı: [4, 1] ile [4] yayınlanarak [4, 4] üretildi (iki taraf da genişletildi); ...` yazılır; program durmaz. `%` tensörlerde tanımlı değildir.
- **Karşılaştırma ve maske:** `< > <= >=` bir sayıyla ya da **aynı şekilli** tensörle eleman bazlı karşılaştırır ve türev izlemeyen 0/1 maskesi verir: `a > 0`, `a < b`. `==` ve `!=` ise tensörleri bütün olarak karşılaştırıp tek bir `Mantık` verir: şekilleri ve tüm elemanları aynıysa eşittir (`tensör([1.0, 2.0]) == tensör([1.0, 2.0])` doğru, `[2]` ile `[1, 2]` şekilli tensörler eşit değildir).
- **İndeksleme:** `t[i]` ilk eksenin i. elemanını verir (bir eksen eksilir; negatif indeks sondan sayar), `t[baş..son]` ilk eksende dilim verir (`son` hariç); ikisi de veriyi kopyalamayan, türev izleyen görünümlerdir ve zincirlenebilir (`t[0][1]`). Sınır dışı indeks hatadır; `t[i] = ...` ile atama yapılamaz.
- **Yazdırma:** `yazdır(t)` şekli başlıkta gösterir (`Tensör[2×3]`), değerleri 4 basamakla yazar; 1000'den çok elemanda her eksenin yalnızca ilk ve son 3 elemanı gösterilir (`…`). Sıfır olmayan bir değer 0.0001'den küçükse ya da bir değer 1e8 ve üzerindeyse tüm hücreler bilimsel gösterimle (`1.0000e-05`) yazılır. Sayı olmayan değerler `Tanımsız`, `Sonsuz`, `-Sonsuz` olarak görünür.
- **Doğruluk değeri:** `eğer t { ... }` tek elemanlı tensörde eleman 0'dan farklıysa doğrudur, boş tensör yanlıştır. Çok elemanlı tensörün doğruluk değeri belirsizdir; `eğer maks(t) > 0` gibi bir indirgeme kullanın (şimdilik hata vermez, doğru sayılır).
- **Matematik:** `relu`, `sigmoid`, `tanh`, `gelu`, `karekök`, `üstel`, `log`, `softmax(x, eksen: -1)`, `log_softmax(x, eksen: -1)`. **Dönüş tipi kuralı:** tensör yalnızca tensör girdisinden çıkar. Sayı, liste ya da matris verilirse sonuç girdinin biçimindedir: eski karşılığı olan adlarda (`relu`, `softmax`...) eski davranış geçerlidir; yeni adlar `gelu`, `üstel`, `log` sayıda Ondalık, listede aynı yapıda liste, matriste Matris; `log_softmax` düz listede liste, matriste satır bazlı Matris döndürür.
- **İndirgemeler:** `toplam(x)`, `ortalama(x)`, `maks(x)`, `min(x)` tüm elemanları skaler (0 boyutlu) tensöre indirger; `eksen: 1` verilirse o eksen kaldırılır (boyut korunmaz; gerekirse `.şekillendir` kullanın). `toplam(x, 1)` / `ortalama(x, 1)` konumlu ekseni de kabul eder; `maks`/`min` için eksen **yalnızca adlı** verilir (`maks(x, 0)` eleman bazlı sınırla karışmasın diye hatadır). `toplam([a, b])` / `ortalama([a, 1, b])` listedeki tensörleri yayınlamalı toplamla katlar (türev korunur); iç içe sayı listesi indirgenemez, önce `tensör(...)` ile çevirin. `eksen:` yalnızca tensörlerde geçerlidir.
- **Sinir ağı:** `gömme(tablo, indeksler)` (tablo `[V, D]`; sonuç `indeks şekli + [D]`), `çapraz_entropi(çıktı, hedef)` (çıktı `[..., C]`, hedef sınıf indeksleri; negatif hedefler yok sayılır), `katman_normu(x, ağırlık, sapma, epsilon: 0.00001)`, `rms_normu(x, ağırlık, epsilon: 0.00001)` (ağırlık/sapma `yok` olabilir).
- **Türev:** `geri_yayılım(kayıp)` (kayıp tek elemanlı olmalı), `x.gradyan` (gradyanlar birikir), `türevsiz { ... }` bloğu türev izlemeden çalışır ve bloğun değerini (son ifade ya da `sonuç`) döndürür, `türevsiz(x)` tensörü hesap çizgesinden ayırır.
- **İndeks:** `maks_indeks(x, eksen: -1)` en büyük elemanın indeksini verir (eşitlikte ilki). Sonuç her zaman tam sayı değerli, türev izlemeyen bir tensördür (o eksen kaldırılır; 1 boyutlu girdide skaler); doğrudan `gömme`/`çapraz_entropi` indeksi olarak kullanılabilir. Tensör tam sayılı işaretlidir: ondalıksız yazdırılır ve `.değer` `Tam` (ya da `Tam` listesi) verir, böylece `harfler[maks_indeks(x).değer[0]]` gibi liste indekslemede doğrudan kullanılır. Bu tensörle yapılan işlemlerin sonucu (ör. `i * 2`) yine Ondalık değerlidir.
- **Gradyan kırpma:** `gradyan_kırp(parametreler, en_fazla: 1.0)` gradyanların toplam L2 normunu `en_fazla` ile sınırlar ve kırpma öncesi normu döndürür; `geri_yayılım` ile `op.adım()` arasında çağrılır.
- **Kaydetme/yükleme:** `tensör_kaydet("model.trt", {"w": w, "b": b})` ve `değişken p = tensör_yükle("model.trt")` (isteğe bağlı `türev_izle: doğru`). Yalnızca verilen yol açılır; dosya varsa üzerine yazılır. Biçim (sürüm 1, küçük sonlu): `TRTENSOR` (8 bayt), sürüm `uint16`, girdi sayısı `uint32`; her girdi için ad uzunluğu `uint32` + UTF-8 ad, veri türü kodu `uint8` (1 = float32), eksen sayısı `uint8`, her eksen `uint64`, satır öncelikli veri; sonda tüm baytların CRC-32 değeri `uint32`. Bozuk, eksik, başka sürümlü ya da başka türden dosya açık bir Türkçe hatayla reddedilir.
- **İyileştiriciler:** `AdamW(parametreler, öğrenme_oranı: 0.001, ağırlık_çürümesi: 0.01, beta1: 0.9, beta2: 0.999, epsilon: 0.00000001)`, `SGD(parametreler, öğrenme_oranı: 0.01, momentum: 0, ağırlık_çürümesi: 0)`; `.adım()` parametreleri günceller, `.sıfırla()` gradyanları sıfırlar. Parametreler doğrudan `tensör(..., türev_izle: doğru)` ile oluşturulmuş tensörler olmalıdır.

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

VS Code İzleme Masası'nda tensörler tip sütununda `Tensör [2×3]` olarak görünür; 1 ve 2 boyutlu tensörler tablo olarak açılabilir.

### E. Veri ve Parçalayıcı
Büyük metin derlemleriyle model eğitmek için iki yerleşik nesne vardır: `tokenizer.json` dosyalarını okuyan **parçalayıcı** ve `.npy` token parçalarını okuyan **veri kümesi**. Uçtan uca örnek: `ornekler/veri_ile_egitim.tr`.

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
- Göreli yollar (`parçalayıcı_yükle` ve `veri_kümesi`) önce çalışma klasörüne, orada yoksa programın bulunduğu klasöre göre çözülür.
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

TR'nin **terminalde** (`tr debug dosya.tr`, kabukta `/h`) çalışan bir hata ayıklayıcısı vardır. VS Code'da hata ayıklarken (`tr dap`) değişkenler yandaki **İzleme Masası** panelinde tek tabloda görünür; listeler tip sütununda `Tablo [3×3]` gibi etiketlenir ve adına tıklanınca tablo olarak açılır (ayrıntılar: `vscode_eklentisi/README.md`).

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
