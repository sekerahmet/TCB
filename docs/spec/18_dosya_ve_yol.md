# 18. Dosya ve yol

Bu bölümün kurallarına göre bugünkü durum ve yapılacaklar: [`docs/status.md`](../status.md), "18. Dosya ve yol".

Her kuralın emsali ve gerekçesi: [`rationale/18_files.md`](rationale/18_files.md).

Veriyi hazırlayan, modeli eğiten ve bir cihazda çalışan program dosyalarla çalışır: veri parçalarını bulur, büyük metni satır satır okur ve yazar, ara sonuçları geçici bir klasöre koyar, modeli kopyalar ve taşır. Bu bölüm yolları, yola sorulan soruları ve dosya ile klasör işlerini söyler. Kullanım: [`docs/guide/04_yapay_zeka_ve_veri.md`](../guide/04_yapay_zeka_ve_veri.md) §4.E.

Göreli yolların ve bağlantıların ortak kuralı 18.16'dadır.

**18.1** `TR_Path(parça, ...)` parçaları (her biri `TR_String` ya da `TR_Path`) birleştirip temizlenmiş bir `TR_Path` kurar: `a/./b/../c` → `a/c`, `a//b` → `a/b`, sondaki `/` atılır.
- `TR_Path` değiştirilemez; ayırıcıyı her sistemde `/` ile saklar ve gösterir (Windows'ta `\` de kabul edilir, işletim sistemine giderken çevrilir).
- `TR_print(y)`, `TR_String(y)` ve metin içine yerleştirme yolun metnini verir; `TR_json_string` yolu metin olarak yazar. `==` temizlenmiş yazımları karşılaştırır; bir `TR_Path` bir `TR_String`'e eşit değildir (4.5).
- İlk parçadan sonra mutlak bir parça (`/etc`, `C:/x`, `C:x`, `\\sunucu`) hatadır: dışarıdan gelen bir ad taban klasörü sessizce değiştiremez. Boş parça ve sekme ya da satır sonu içeren parça (13.1a) hatadır. `..` temizlenir ama yasak değildir: `TR_Path("a", "..")` → `.`.
- *Test:* TR testi.

**18.2** Yolun yazımına soran özellikler diske bakmaz:
- `.TR_name` son bileşendir (`.` ve kökte boş metin);
- `.TR_extension` noktasıyla birlikte son uzantıdır (`"derlem.tar.gz"` → `".gz"`); uzantısız ad, noktayla biten ad ve noktayla başlayan ad (`.gitignore`) için boş metin;
- `.TR_stem` son uzantının atıldığı addır; `.TR_stem + .TR_extension` her zaman `.TR_name`'dir;
- `.TR_parent` bulunduğu klasörün yoludur (tek bileşenli göreli yolda `.`).
- *Test:* TR testi.

**18.3** Diske soran özellikler yolu 18.16'nın okuma kuralıyla çözer:
- `.TR_absolute` çözülmüş mutlak yoldur;
- `.TR_exists`, `.TR_is_file`, `.TR_is_dir` yolda bir dosya ya da klasör, sıradan bir dosya, bir klasör olup olmadığını söyler; olmayan yolda ve kırık bağlantıda `TR_false`;
- `.TR_size` dosyanın bayt boyutudur (`TR_Int`; `.gz` dosyada sıkıştırılmış boyut); klasörde ve olmayan dosyada hatadır.
- *Test:* TR testi.

**18.4** Yol alan her yerleşik (13.1a'dakiler ve bu bölümünkiler) yolu `TR_String` ya da `TR_Path` olarak alır; başka bir tip denetim hatasıdır ve ileti iki kabul edilen tipi ve verilen tipi söyler. `TR_read_csv` bir `TR_Path` alınca onu her zaman dosya sayar: dosya yoksa hata verir, metni CSV içeriği sanmaz.
- *Test:* TR testi.

**18.5** `TR_list_dir(yer, kalıp, TR_ignore_case: TR_false)` verilen klasörde kalıba uyan dosya ve klasörleri bir `TR_Path` listesi olarak verir; `yer` `"."`, `kalıp` `"*"` varsayılandır.
- Liste yolların `/`'lı yazımının bayt sırasıyla **sıralıdır**: her çalıştırmada ve her sistemde aynı sıra.
- **Kalıp** `/` ile ayrılmış bileşenlerdir. Bir bileşenin içinde `*` sıfır ya da çok karakter, `?` tek karakter, `[a-z]`, `[!a-z]` ve `[^a-z]` karakter kümesidir; `**` yalnız tam bir bileşen olarak sıfır ya da çok klasör eşler (sonda: alttaki her şey). `{a,b}` seçeneği yoktur.
- **Eşleme her sistemde harf büyüklüğüne duyarlıdır:** `"*.jpg"` `A.JPG`'yi eşlemez. Aynı klasörde aynı program her sistemde aynı listeyi verir. Kameraların `.JPG` gibi büyük harfli uzantıları için `TR_ignore_case: TR_true` her sistemde harf büyüklüğüne bakmadan eşler.
- Noktayla başlayan adlar, kalıbın o bileşeni de noktayla başlamadıkça listelenmez. `**` gizli klasörlere ve klasör bağlantılarının içine girmez (döngü olmaz); adıyla yazılmış bir bağlantının içine girilir.
- Sonuçlar klasörün verildiği biçimle başlar: `TR_list_dir("veri", "*.npy")` → `[veri/a.npy, ...]`; mutlak klasörde mutlak yollar. Göreli klasör çalışma klasörünün dışında bulunduysa (18.16'nın okuma sırası) sonuçlar **mutlak** yollardır: sonuçlar değiştiren bir işe (`TR_remove`) verilince başka bir klasördeki aynı adlı dosyayı göstermez.
- Mutlak kalıp, `..` içeren kalıp, `a**` gibi yarım `**`, kapanmayan `[`, boş kalıp, olmayan klasör ve klasör yerine dosya hatadır. Okunamayan bir alt klasör atlanır ve tek bir uyarı yazılır.
- *Test:* TR testi; harf büyüklüğüne duyarlı eşleme uygulanınca TR testi.

**18.6** `TR_make_dir(yol)` klasörü üst klasörleriyle birlikte oluşturur ve yolunu döndürür; klasör zaten varsa sorun değildir, aynı adda bir dosya varsa hatadır.
- *Test:* TR testi.

**18.7** `TR_remove(yol, TR_recursive: TR_false)` bir dosyayı, bir bağlantıyı ya da boş bir klasörü siler ve silinen dosya (klasör olmayan girdi) sayısını `TR_Int` olarak verir (boş klasörde 0).
- Dolu bir klasör yalnız `TR_recursive: TR_true` ile silinir; yoksa hata klasörü gösterir.
- Olmayan yol hatadır: yazım yanlışı sessiz geçmez. Joker karakter genişletilmez: `TR_remove("*.tmp")` `*.tmp` adlı dosyayı arar.
- Salt okunur bir dosya zorla silinmez. Klasörün içinde salt okunur bir dosya varsa **hiçbir şey silinmez**: önce bütün ağaç denetlenir.
- Bağlantının kendisi silinir, gösterdiği yere dokunulmaz. Gezinme sırasında bir alt klasör bir bağlantıyla değiştirilse de silme klasörün dışına çıkmaz; Windows'un dizin bağlantıları da bağlantıdır.
- Bu programda bir `TR_LineReader` ya da `TR_LineWriter` ile açık olan bir dosyayı içeren yol silinmez (Windows'ta silme yarıda kalırdı); ileti okuyucuyu ya da yazıcıyı kapatmayı (18.15, 18.17) önerir.
- *Test:* TR testi.

**18.8** `TR_remove` şu klasörleri silmez, nedenini söyleyen bir hata verir:
- kök ve sürücü kökleri;
- ev klasörü ve üstleri;
- işletim sisteminin bilinen klasörleri ve üstleri (Windows: `%SystemRoot%`, `%ProgramFiles%`, `%ProgramFiles(x86)%`, `%ProgramData%`; öteki sistemler: `/bin`, `/boot`, `/dev`, `/etc`, `/lib`, `/lib64`, `/proc`, `/sbin`, `/sys`, `/usr`, `/var`, macOS'un `/System`, `/Library`, `/Applications`);
- çalışma klasörü, kodun modülünün ve programın klasörü ve üstleri (depoda çalışan programın depo kökü de).

Karşılaştırma bağlantıları çözülmüş mutlak yollarla yapılır (`.`, `..`, `a/..` aynı yeri gösterir). Aynı korumalar `TR_move`'un kaynağına ve `TR_overwrite: TR_true` ile yerine bir şey konan klasöre de uygulanır.
- *Test:* TR testi.

**18.9** `TR_copy(kaynak, hedef, TR_overwrite: TR_false)` bir dosyayı ya da klasörü içindekilerle kopyalar ve son hedefin yolunu döndürür.
- Hedef var olan bir klasörse kaynak onun içine aynı adla gider: `TR_copy("model.trt", "yedek")` → `yedek/model.trt`. `/` ile biten bir hedef metni var olan bir klasör olmalıdır.
- Son hedef zaten varsa `TR_overwrite: TR_true` gerekir. Bir dosya bir klasörün, bir klasör bir dosyanın yerine konmaz. Kaynak ile hedef aynıysa ya da bir klasör kendi içine kopyalanıyorsa hatadır.
- Kopya önce hedefin yanında geçici bir ada yazılır, sonra yerine konur: yarıda kalan bir kopya var olan hedefi bozmaz, geride geçici dosya kalmaz. İzinler ve değiştirilme zamanı korunur.
- Kaynağın kendisi bir bağlantıysa hatadır; klasörün içindeki bağlantılar ve özel dosyalar atlanır ve tek bir uyarı yazılır.
- *Test:* TR testi.

**18.10** `TR_move(kaynak, hedef, TR_overwrite: TR_false)` bir dosyayı ya da klasörü taşır ve son hedefin yolunu döndürür; hedef kuralları `TR_copy`'ninkidir, kaynağa 18.8'in korumaları uygulanır.
- Aynı diskte yeniden adlandırır; dosyanın yerine koyma tek adımdır.
- Başka bir diske önce kopyalar, kopyayı dosya sayısı ve toplam boyutla doğrular, sonra kaynağı siler. Taşınan klasörde bir bağlantı ya da özel dosya varsa taşıma yapılmaz, kaynağa dokunulmaz.
- *Test:* TR testi.

**18.11** `TR_read_lines(yol)` dosyayı açar (olmayan dosya hemen hata verir) ve bir `TR_LineReader` döndürür. Okuyucu dosyayı **akışla** okur: bellek dosyanın boyutundan bağımsızdır.
- `TR_for satır TR_in okuyucu { ... }` ile dolaşılır ya da `.TR_next()` ile sıradaki satır alınır (dosya bitince `TR_none`); ikisi aynı akışı tüketir.
- Satır sonu (`\n`, `\r\n`) atılır, ilk satırın başındaki BOM atılır; dosya satır sonuyla bitiyorsa sonda boş bir satır üretilmez. Tek bir satır en çok 64 MB'tır, daha uzunu hatadır.
- Aynı okuyucuyu dolaşan görevler her satırı bir kez alır.
- Dosya sonuna gelince, `.TR_close()` ile (18.15) ya da program bitince kapanır.
- *Test:* TR testi.

**18.12** `.gz` saydamlığı: yolu `.gz` ile biten (harf büyüklüğü fark etmez) bir dosya `TR_read_file`, `TR_read_lines` ve `TR_read_csv`'da açılarak okunur, `TR_write_file` ve `TR_write_lines`'ta sıkıştırılarak yazılır.
- Kural yalnız uzantıya bakar: içerik gzip değilse okuma bunu söyleyen bir hatayla durur; bozuk ve yarıda kesilmiş gzip içeriği de hatadır.
- `TR_read_file`'ın bir `.gz` dosyadan açacağı içerik en çok 1 GB'tır; daha büyüğü `TR_read_lines` ile okunur.
- *Test:* TR testi.

**18.13** `TR_write_file` atomiktir: içerik önce aynı klasörde geçici bir dosyaya yazılır, diske işlenir, sonra eski dosyanın yerine konur. Yarıda kalan bir yazma (hata, disk dolması, çökme) var olan dosyayı bozmaz ve geride geçici dosya kalmaz.
- Yol bir bağlantıysa gösterdiği dosya yazılır, bağlantı korunur. Var olan dosyanın izin bitleri korunur, yeni dosya `0644` açılır.
- Yazılan bağlantı taban klasörün dışını gösterebilir: bağlantıyla yazmak klasörün dışına yazmaktır. Dışarıdan gelen adlarla yazan program 18.1'in mutlak parça kuralına ve kendi denetimine güvenir.
- Bedelleri: her yazma diske işlendiği için çok sayıda küçük yazma yavaştır (büyük çıktı 18.17 ile yazılır); dosyanın başka sabit bağlantıları eski içeriği göstermeye devam eder; sahiplik, erişim listeleri ve Windows'un ek veri akışları klasörün varsayılanına döner; klasöre yazma izni gerekir; Windows'ta başka bir programın silmeye kapalı açtığı dosyanın yerine yazılamaz.
- *Test:* TR testi.

**18.14** `TR_temp_dir()` işletim sisteminin geçici klasörünün altında (Windows'ta `%TEMP%`, öteki sistemlerde `$TMPDIR` ya da `/tmp`) bu program için **yeni, boş** ve yalnız kullanıcıya açık (`0700`) bir klasör oluşturur ve mutlak yolunu döndürür; her çağrı ayrı bir klasör verir.
- Program bitince (başarıyla, hatayla, durdurmayla; `TR_cmd_test`, hata ayıklayıcı ve etkileşimli kabuk dahil) klasör içindekilerle silinir: önce programın açık dosyaları kapatılır, sonra silme `TR_remove(..., TR_recursive: TR_true)`'nun kurallarıyla yapılır (18.7, 18.8).
- Silinemeyen bir klasör standart hataya bir uyarı yazar; program klasörü kendisi silmiş ya da taşımışsa sorun değildir.
- Testler ve deneme programları dosyalarını buraya koyar; depo ve geçici klasör kirlenmez.
- *Test:* TR testi.

```tr
# TR 4
TR_var k = TR_temp_dir()
TR_write_file(TR_Path(k, "ara_sonuç.txt"), "0.42")
TR_print(TR_list_dir(k).TR_length) # program bitince k silinir
```

**18.15** `okuyucu.TR_close()` okuyucunun dosyasını sonuna kadar okumadan bırakır: dosya artık bu programda açık değildir, silinebilir ve taşınabilir.
- Dosyayı bu çağrı kapattıysa `TR_true`, zaten kapalıysa (sonuna kadar okunduysa ya da önceden kapatıldıysa) `TR_false` verir; iki kez kapatmak hata değildir.
- Kapatılmış okuyucudan satır istemek (`.TR_next()`, `TR_for`) hatadır; ileti yeni bir okuyucu açmayı söyler.
- Kapatma yorumlayıcı kilidini bırakır; başka bir görev o sırada diskten okuyorsa okuması bitince kapanır.
- *Test:* TR testi.

**18.16** Göreli yollar 13.1a'nın kuralıyla çözülür.
- **Okuyan ve soran işler** (`TR_read_file`, `TR_read_lines`, `TR_list_dir`, 18.3'ün özellikleri, `TR_copy`'nin kaynağı) önce çalışma klasörüne, sonra kodun modülünün ve programın klasörüne bakar; `TR_Path("x").TR_exists` ile `TR_read_file("x")` aynı dosyayı görür.
- **Değiştiren işler** (`TR_write_file`, `TR_write_lines`, `TR_save_tensors`, `TR_Checkpoint`'in koşu klasörü (20), `TR_Logger`'ın dosyası (19), `TR_make_dir`, `TR_remove`, `TR_move`'un kaynağı ve hedefi, `TR_copy`'nin hedefi) göreli yolu yalnız çalışma klasörüne göre çözer: başka bir klasördeki aynı adlı dosyayı silmez, ezmez, taşımaz.
- **Bağlantılar:** silen, kopyalayan, taşıyan ve klasör gezen işler bağlantıları izlemez; okuyan, soran ve yazan işler bağlantının gösterdiği dosyayı kullanır.
- Ağır işler (listeleme, kopyalama, silme, diskten okuma ve yazma) yorumlayıcı kilidini bırakır (10.3). Hatalar `TR_try`/`TR_catch` ile yakalanır; dosya bulunamazsa ileti bakılan yerleri sayar.
- *Test:* gerekmez, henüz uygulanmadı; uygulanınca TR testi (okuma sırası bugün 18.3'ün testinde sınanıyor).

**18.17** `TR_write_lines(yol)` bir `TR_LineWriter` açar ve büyük çıktıyı **akışla** yazar: bellek çıktının boyutundan bağımsızdır.
- `.TR_write(satır)` bir `TR_String` satırını ve ardından `\n` yazar. Satır sonu (`\n` ya da `\r`) içeren satır hatadır: her `.TR_write` tam bir satırdır, JSONL gibi satır biçimleri bozulmaz.
- Yazma atomiktir (18.13): satırlar aynı klasördeki geçici bir dosyaya gider, `.TR_close()`'ta diske işlenir ve yerine konur. Program yazıcı açıkken başarıyla biterse yazıcı kapanır ve dosya yerine konur; hatayla ya da durdurmayla biterse geçici dosya silinir, var olan dosya değişmez.
- `.TR_close()` 18.15'teki gibidir; kapatılmış yazıcıya yazmak hatadır. Yazıcıyı kapatmak için `TR_defer` kullanılır (9.1).
- Yazıcı açıkken aynı programda o yolu okumak (`TR_read_file`, `TR_read_lines`, `TR_read_csv`) ve o yola ikinci bir yazıcı açmak ya da yazmak hatadır: dosyanın yerinde henüz eski içerik vardır; ileti yazıcıyı önce kapatmayı söyler.
- Yazma yorumlayıcı kilidini diske işlerken bırakır; aynı yazıcıya yazan görevlerin her satırı bütün yazılır, satırların sırası 10.8'e göre tanımsızdır.
- *Test:* gerekmez, henüz uygulanmadı; uygulanınca TR testi.
