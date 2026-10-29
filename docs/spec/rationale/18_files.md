# 18. Dosya ve yol: emsal ve gerekçe

Kurallar: [`18_dosya_ve_yol.md`](../18_dosya_ve_yol.md). Bu dosya her kuralın emsalini (başka dillerde nasıl olduğu, neyi almadığımız, kaynaklarıyla) ve gerekçesini tutar. Dili kullanmak için kuralların kendisi yeter; bu dosya kararları tartmak ve denetlemek içindir.

### 18.1

- *Emsal:* Python'un `pathlib.Path`'i ve Rust'ın `PathBuf`'u yolu metinden ayrı bir tip yapar; ikisinde de mutlak bir parça baştaki tabanı sessizce atar (`Path("veri") / "/etc"` → `/etc`) [kanıtsız: bu tur açılmadı]. *Bunu almadık.*
- *Gerekçe:* yolu metinden ayırmak ajanın yolu metin gibi birleştirip ayırıcıyı yanlış yazmasını önler. Dışarıdan gelen bir ad (veri kümesindeki dosya adı) taban klasörü değiştirememelidir: sessizce `/etc`'ye giden bir yol "Hata riski az"ın karşıtıdır. Ayırıcının her sistemde `/` olması aynı programın her sistemde aynı metni yazmasını sağlar (Güvenilir).
- *Gerekçe (`..`):* mutlak parça kuralının amacı dışarıdan gelen adın taban klasörü değiştirmemesidir; `..` aynı kaçışın göreli yoludur [ölçüldü: DENETİM, `TR_Path("veri", "../../gizli/ayar.json")` tabandan çıkıyordu]. Kök ve UNC önekleri temizlenince anlamı değişirdi; motor onları zaten koruyordu (Gemini Koşu 5).

### 18.2

- *Emsal:* Python `pathlib`'in `name`, `suffix`, `stem`, `parent`'ı aynı tanımlardır (`".gitignore"`'ın `suffix`'i boş) [kanıtsız: bu tur açılmadı].
- *Gerekçe:* ajanın en çok bildiği tanımlar; diske bakmadıkları için sonuçları yalnız yazıma bağlıdır.

### 18.3

- *Emsal:* Python `Path.exists()`, `is_file()`, `is_dir()` bağlantıyı izler, kırık bağlantıda `False` verir [kanıtsız: bu tur açılmadı].
- *Gerekçe:* soran özellik okuyan işle aynı dosyayı görmelidir (18.16): `TR_exists` doğru dedikten sonra `TR_read_file` başka bir yerde aramaz.

### 18.4

- *Emsal:* Python'un dosya fonksiyonları `str` ve `os.PathLike` alır [kanıtsız: bu tur açılmadı].
- *Gerekçe:* iki yazım da doğaldır; yolun tipinden anlam çıkarmak (`TR_read_csv`'nin metni içerik sanması) sessiz yanlıştır, `TR_Path` her zaman dosyadır.

### 18.5

- *Emsal:* Python'un `glob.glob`'u ve `os.listdir`'i sırasız verir (dosya sisteminin sırası) [kanıtsız: bu tur açılmadı]; `glob` Windows'ta harf büyüklüğüne bakmaz, `pathlib`'in `glob`'u 3.12'den beri `case_sensitive` argümanı alır; Go'nun `filepath.Glob`'u Windows'ta da duyarlıdır [ölçüldü: DENETİM 18-1, Windows]. Go'nunki ve `pathlib`'in argümanı alındı.
- *Gerekçe:* Güvenilir: veri parçalarının sırası eğitimin sırasıdır; sırasız listeleme aynı programın iki makinede ayrı sonuç vermesidir. Harf büyüklüğü de aynı yanlışı doğurur: Mac'te yazılan `"*.jpg"` Linux'ta `A.JPG`'yi sessizce kaçırır. Her sistemde duyarlı eşleme her yerde aynı listeyi verir; bugün Windows'ta `"*.TXT"` `a.txt`'yi eşliyor [ölçüldü: main `dc4930ca`]. Duyarlı eşleme kameranın `.JPG`'lerini `"*.jpg"` ile kaçırır; açık argüman bu durumu her sistemde aynı biçimde çözer. Okuma sırasıyla başka bir klasörde bulunan sonuçların mutlak olması, listelenen dosyaları silen bir temizleme döngüsünün başka klasördeki aynı adlı dosyayı silmesini önler [ölçüldü: DENETİM 18-1, `TR_list_dir("cikti", "*.tmp")` programın klasörünü buluyor ve göreli yollar veriyor].

### 18.6

- *Emsal:* Python `os.makedirs(exist_ok=True)`, Go `os.MkdirAll` [kanıtsız: bu tur açılmadı].
- *Gerekçe:* klasör hazırlamak çoğunlukla "var olsun" demektir; ama aynı adda bir dosya bir yanlıştır ve söylenir.

### 18.7

- *Emsal:* Python'un `shutil.rmtree`'si dolu klasörü sorusuz siler ve bağlantı yarışlarına karşı ayrı bir yol izler; Go'nun `os.Root`'u klasörün dışına çıkmayan gezinme verir [kanıtsız: bu tur açılmadı].
- *Gerekçe:* silme geri alınmaz; ajanın yanlışı (yanlış yol, joker sanılan ad, dolu klasör) sessizce veri kaybettirmemelidir. Hepsi ya da hiçbiri: yarım silinmiş bir klasör hem veriyi bozar hem yeniden denemeyi zorlaştırır.

### 18.8

- *Emsal:* GNU `rm`'in `--preserve-root`'u yalnız kökü korur [kanıtsız: bu tur açılmadı].
- *Gerekçe:* ajanın en pahalı yanlışı yanlış klasörü silmektir. Programın kendi deposu, ev klasörü ve sistem klasörleri bir programın silmesi gereken yerler değildir.

### 18.9

- *Emsal:* Python `shutil.copy2` izinleri ve zamanı korur ama yerine koyma atomik değildir; `cp` hedef klasörse içine kopyalar [kanıtsız: bu tur açılmadı].
- *Gerekçe:* modeli yedeklerken yarıda kalan bir kopya eski yedeği bozmamalıdır. Üzerine yazmanın açık argüman istemesi ajanın var olan bir dosyayı farkında olmadan ezmesini önler.

### 18.10

- *Emsal:* Python `shutil.move` başka diske kopyalayıp siler [kanıtsız: bu tur açılmadı].
- *Gerekçe:* başka diske taşımada kaynak doğrulanmış bir kopyadan önce silinmemelidir.

### 18.11

- *Emsal:* Python'un dosya nesnesi satır satır dolaşılır ve satır sonunu tutar [kanıtsız: bu tur açılmadı]. *Satır sonunu atıyoruz.*
- *Gerekçe:* veriyi hazırlamak bellekten büyük metinle çalışır; akışla okumak bunun önkoşuludur. Satır sonunun atılması ajanın en sık `strip` unutmasını kaldırır.

### 18.12

- *Emsal:* pandas `.gz` uzantısından sıkıştırmayı anlar [kanıtsız: bu tur açılmadı].
- *Gerekçe:* derlemler çoğunlukla sıkıştırılmış gelir; ayrı bir açma adımı ajanın unutacağı bir adımdır. Uzantı ile içerik uyuşmazsa sessizce bayt okumak yerine hata vermek yanlışı gösterir.

### 18.13

- *Emsal:* Go'nun `renameio`'su ve Python'un `os.replace`'i geçici dosya ve yeniden adlandırmayla atomik yazar; Python'un `open(..., "w")`'si atomik değildir [kanıtsız: bu tur açılmadı].
- *Gerekçe:* Güvenilir: kesilen bir program (eğitim, robot) yazdığı yapılandırmayı ya da sonucu yarım bırakmamalıdır.

### 18.14

- *Emsal:* Python `tempfile.TemporaryDirectory` bağlam bitince siler; `mkdtemp` silmez [kanıtsız: bu tur açılmadı].
- *Gerekçe:* testlerin ve denemelerin bıraktığı dosyalar depoyu kirletir; ajan temizliği unutur. Silme 18.7 ve 18.8'in kurallarıyla yapıldığı için geçici klasör bir bağlantıyla başka bir yere açılsa da dışarıyı silmez.

### 18.15

- *Emsal:* Python dosya nesnesinin `close()`'u iki kez çağrılabilir [kanıtsız: bu tur açılmadı].
- *Gerekçe:* Windows'ta açık dosya silinemez; okumayı yarıda bırakan programın dosyayı bırakabilmesi gerekir. Kapatılmış okuyucudan okumak sessizce boş vermez, hata verir.

### 18.16

- *Emsal:* Python ve Go göreli yolu yalnız çalışma klasörüne göre çözer [kanıtsız: bu tur açılmadı].
- *Gerekçe:* okumada programın yanındaki dosyayı bulmak programın başka bir klasörden çalışmasını sağlar (13.1a). Değiştiren işte aynı arama tehlikelidir: bugün başka bir klasörden çalışan program `TR_remove("yanda.txt")` ile programın yanındaki dosyayı siliyor, `TR_write_file` onu eziyor [ölçüldü: main `dc4930ca`]. Ajan çalışma klasöründe bir dosya yarattığını ya da sildiğini sanır, değişen başka bir klasördür. Bu yüzden değiştiren işler yalnız çalışma klasörüne bakar.

### 18.17

- *Emsal:* Python'da dosyaya satır satır yazmak `open(..., "w")` ile yapılır, atomik değildir ve satırın içindeki satır sonunu denetlemez; kapatılmadan biten programda Python tamponu yazar, Go'nun `bufio.Writer`'ı hiçbir şey yazmaz (0 bayt) [ölçüldü: DENETİM 18-1].
- *Gerekçe:* amacın ilk işi veriyi temizleyip eğitime hazırlamaktır; temizlenen derlem bellekten büyüktür ve `TR_write_file` tek seferde yazar. Akışla yazan yazıcı olmadan bu iş TR'de yazılamaz. Atomiklik 18.13'ünküyle aynıdır: yarıda kalan bir temizleme eski çıktıyı bozmaz. İçinde satır sonu olan satır JSONL'ü sessizce bozar; hata bunu yazılırken gösterir. Açık yazıcının yolunu okumak eski içeriği verir; ajan yeni yazdığını okuduğunu sanar, bu yüzden hatadır.
- *Gerekçe (çıkışın türü):* ertelenen kapatma hatayla çıkışta da çalışır (9.2); tek bir `.TR_close()` yarım dosyayı yerine koyardı. Go'nun `renameio`'su iki ad kullanır (`Cleanup` her çıkışta, `CloseAtomicallyReplace` yalnız başarıda); TR tek çağrıda birleştirir, ajan iki adı karıştırmaz (Gemini Koşu 5).
