# 19. Süreç ve kayıt: emsal ve gerekçe

Kurallar: [`19_surec_ve_kayit.md`](../19_surec_ve_kayit.md). Bu dosya her kuralın emsalini (başka dillerde nasıl olduğu, neyi almadığımız, kaynaklarıyla) ve gerekçesini tutar. Dili kullanmak için kuralların kendisi yeter; bu dosya kararları tartmak ve denetlemek içindir.

### 19.1

- *Emsal:* Python'un `os.environ.get(ad)`'ı tanımsızda `None` verir; `os.environ` bütün ortamı bir sözlük olarak açar [kanıtsız: bu tur açılmadı]. *İkincisini almadık.*
- *Gerekçe:* uzak modelin anahtarı (`HF_TOKEN`, `OPENAI_API_KEY`) ortamda durur; bütün ortamı yazdıran bir satır onları günlüğe döker. Tek bir değişkeni adıyla okumak yeter.

### 19.2

- *Emsal:* Python'un `argparse`'ı ve `typer`'ı seçenekleri bir şemadan kurar; `typer` ve Rust'ın `clap`'i (derive) bir yapının ya da fonksiyonun tiplerinden [kanıtsız: bu tur açılmadı].
- *Gerekçe:* seçenekler bir yapı olunca her seçeneğin tipi çalışmadan önce bilinir ve yanlış yazılmış bir seçenek adı (`ayar.adim`) denetimde yakalanır (1.2, 13.10). Sözlükle yazılan seçenekler tek tipli olamaz (3.7) ve yanlış anahtar ancak çalışırken görünür. Bugün seçenekler bir sözlükle veriliyor [ölçüldü: main `217ae940`].
- *Gerekçe (zorunlu seçenek):* `argparse`'ın `required=True`'su gibi bir zorunlu seçenek, argümansız çalışan `TR_cmd_test`'i ve kabuğu kırar ve `TR_command_line`'ın bir tip değeri almasını gerektirirdi. Her seçeneğin varsayılanı olması tek bir kural bırakır; zorunlu seçenek isteğe bağlı tiple yazılır ve yokluğu tiple görünür (3.5): ajan `TR_none`'ı denetlemeden değeri kullanamaz.

### 19.3

- *Emsal:* `argparse` seçenek adını yazıldığı gibi alır, `-`'yi `_`'ye çevirir [kanıtsız: bu tur açılmadı]. *Çevirmeyi almadık.*
- *Gerekçe:* kullanıcının adı düzdür ve çevrilmez (17.1); seçeneğin tek yazımı vardır, ajan ve kullanıcı aynı adı yazar. Bir adın iki yazımı ajanın iki ayrı seçenek sandığı bir adı doğurur.

### 19.4

- *Emsal:* GitHub Actions gizli değerleri günlükte `***` ile maskeler; değer dönüştürülünce eşleşmez [kanıtsız: bu tur açılmadı].
- *Gerekçe:* ajanlar programın çıktısını okur ve başka yerlere taşır; çıktıya düşen bir anahtar sızmış bir anahtardır. Veriyi değiştirmemek programın doğru çalışmasını korur: maskeleme yalnız insanın göreceği yerdedir. Sözcüklerin dil dosyalarında durması hiçbir dile ayrıcalık tanımaz: yeni bir gösterim dili kendi sözcüklerini getirir (`contraseña`).
- *Gerekçe (adresin kimlik kısmı, uygulama 2026-11-13):* vekil ve veritabanı adresi parolasını adında değil değerinde taşır (`HTTPS_PROXY`, `DATABASE_URL`); ad kuralı onu görmez, sızıntı sessizdir.
  - **Taramanın yeri program başıdır:** komut satırı `SetArgs`'ta (programı çalıştıran üç yerin üçü de onu `Run`'dan önce çağırır), ortam süreç motorunun `Register`'ında (her VM'de, programdan önce). Program okumadığı değişkeni de dış programa geçirir; o programın standart hatası 21.4'ün iletisine girer, bu yüzden okuma anında taramak geç kalırdı. `TR_env` okuması yeniden taramaz: kural tek yerde (`interp`, `address_secret.go`) kalır.
  - **Toplu kayıt:** bir değerin bütün adresleri tek seferde kaydedilir, süzgeç bir kez kurulur. Tek tek kayıt, adresi çok olan bir değerde süreyi adres sayısının karesiyle büyütürdü.
  - **Önce maske, sonra kesim:** iletiye kesilerek giren değer (kullanım hatasının değeri, H7207, hata iletisindeki değer özeti) önce maskelenir. Maske çıktı yazılırken uygulanır; kesim parolanın ortasına düşerse kalan baş artık hiçbir gizli değerle eşleşmez. Standart hatanın kuyruğu zaten böyle yapıyordu.
  - **Bağlamlı maske (tur 7, K1a; uygulama 2026-11-13):** değer kaydı adresi yalnız ortamda ve komut satırında görür; dosyadan, ağdan okunan ya da programda kurulan adres ve 6 karakterden kısa parola basılan metinde açık kalıyordu [ölçüldü: `8bff80da`'nın `trlang`'ı, `motor-194-ikinci-deneme/taban.txt`]. Kural basılan metne konumla uygulanır, değer kaydedilmez: kayıt kısa parolayı her metinde örterdi.
    - *Emsal:* RFC 3986 3.2.1 parolayı ilk `:`'den sonrası sayar: "Applications should not render as clear text any data after the first colon". OpenTelemetry (semconv v1.45.0, `url.full`) kullanıcı adını da parolayı da örter, Sentry Python SDK 2.75.0 (`sanitize_url`) kimlik kısmının tamamını. Değer eşleşmesinin alt sınırı: GitLab 8 karakter ister, Google'ın Secret Manager eylemi çok kısa değerin günlüğü okunmaz yaptığını yazar [kanıtlı: tur 7 araştırması 1.1, WebFetch özeti].
    - *Bedel:* 6 ve daha uzun kullanıcı adı (`postgres://postgres@`) örtülür. Bölge son `@`'ye uzatılmadı: `https://www.npmjs.com/package/@scope/x`'in yolu örtülürdü. Boşluksuz JSON'da (`["http://a:pw@h","u@v"]`) bölge `"`'de bitmez, son `@`'ye kadar örtülür; fazla maskedir, parola açılmaz.
    - *Birleşim:* değer maskesi ve bağlam aynı özgün metinde bulunur. Sıralı iki aşamanın iki yönü de birer durumda sızdırıyordu: `https://ali` kayıtlıyken önce değer `***:ab@h`, `ali:Vekil2026` kayıtlıyken önce bağlam `http://ali:***@h` verdi. Birleşimde adressiz metnin sonucu değer maskesinin kendisidir: 200 000 rastgele metinde 0 fark [ölçüldü: prototip, `motor-194-ikinci-deneme/proto.txt`].
  - **Kaçışlı biçimin kaydı (TCB, tur 8 araştırmasının bulgusu):** kabın metni (`QuoteText`), `TR_JSON`'un metni ve iletinin `turkish.Printable`'ı metni maskeden önce kaçışlar; kayıt değeri kaçışsız aradığı için tırnaklı, ters bölülü ya da denetim harfli gizli değer kapta ve çalışma hatasında açık kalıyordu [ölçüldü: `8bff80da`, `kacis.txt`, `cr-taban.txt`]. Üç kaçış da harf harftir: geçerli UTF-8 gizli değerin metin içindeki kaçışlı hâli kendi kaçışlı biçimidir (üçünde de 0 sapma, yaklaşık 69 000 deneme) [ölçüldü: `kacis-gercek.txt`]. Bu yüzden kayıt biçimleri de tutar.
    - Seçilmeyen yol, yaprak başına maske: `valueText` veri yolunda da kullanılıyor (metin yerleştirme, birleştirme, `TR_String`); orada maskelemek "veri değişmez"i bozardı, insan çıktısına ikinci bir yazım yolu gerekirdi.
    - Kural: maskeden önce metni değiştiren yeni bir dönüşüm eklenirse kayda onun biçimi de eklenir; testi `TestEscapedSecretIsMasked`'tedir.
    - Sınır: geçersiz UTF-8 gizli değerde komşu bayt sınırdaki yarım diziyi kaçışlanan bir harfe tamamlarsa (`x\xC2` + `\x85abcdef`, U+0085) biçim eşleşmez. Ortam ve komut satırı denetlenmeden girer.
  - **`DSN` (tur 7, K1b):** MySQL'in bağlantı dizgisi `://` taşımaz; MySQL Go sürücüsü (v1.10.1) parolayı kaçışsız alır ve parolada `/`'ye izin verir, ayrıştırıcısı son `/`'yi, son `@`'yi, ilk `:`'yi arar [kanıtlı: tur 7 araştırması 1.1]. Genel bir bölge kuralı bu sınırı bilemez; `DSN` adı değeri bütünüyle gizler. `URL` gizli sözcük olmadı: her adresi bütünüyle örterdi.
  - **Maskenin anı (tur 7, K5):** maske çıktının yazıldığı anda kayıtlı değerlerle uygulanır; sonradan kaydedilen değerin önceden yazılmış çıktısı geri alınamaz. Tek ilke akışın, olay akışının görüntüsünün, nabzın ve geri oynatmanın sorusuna aynı cevabı verir [kanıtlı: tur 7 araştırması 1.5].
  - **Süreç boyu küme, VM başına kayıt ilkesine istisna:** programın durumu VM başınadır. Değer özetini kuran `TypeAndSummary`'nin ve ileti metnini yazan `MessageText`'in elinde VM yoktur (test dışındaki 54 çağrının 37'sinde imzada VM yok); VM'yi oraya taşımak 100'ü aşkın değişiklik isterdi. Bu yüzden yalnız özetin kesimi için süreç boyu bir küme tutulur; çıktının maskesi VM başına kalır. Bedel: tek süreçte birden çok VM koşan yolda bir VM'nin kaydı öbürünün yakalanan hata metninde `***` görünebilir; yön güvenlidir, gizli değer sızmaz. `trlang test` her testten önce VM'lerin kaydettiği parçayı boşaltır, testin hata metni önceki teste bağlı olmaz (12.5).

### 19.4a

- *Emsal:*
  - **TR 16.3a:** akışlı çözme yarım karakteri aynı yolla tutar: `.TR_step`, `.TR_finish` ve birleşimin toplu sonuçla aynı olduğu sözü. 19.4a aynı kalıbı gizli değerin başına uygular.
  - **GitHub Actions Runner:** maske satır başınadır; birden çok satıra yayılan gizli değer satır satır kaydedilmezse maskelenmez [kanıtsız: bu tur açılmadı].
  - **Ekranın sözleşmesi** (`tools/assistant/contract.md`, "Parçalı akış"): sunucu metnin sonundan en uzun gizli değer kadarını geri tutar. TR programına bu boyu veren bir araç yoktu (`VM.LongestSecret` yalnız Go'da).
- *Gerekçe:*
  - Dil modelinin cevabı token token gider; 20 karakterlik bir anahtar hemen her zaman iki ya da daha çok parçaya bölünür [tahmin: parçalayıcının ortalama parça boyu birkaç karakter; ölçülmedi]. Olay başına maske bu yüzden asıl kullanımda hiç çalışmazdı (DENETİM 356 Ş4, kritik).
  - Programa yalnız en uzun gizli değerin boyunu vermek geri tutmayı her programa yeniden yazdırırdı: ajan tutma ile maskenin sırasını ya da sonu göndermeyi kaçırır, kaçırması sessizdir. Akış nesnesi işi tek yerde yapar, sözü testle sabitlenir.
  - Tutulan, en uzun gizli değerin tamamı değil, bir gizli değerin başı olabilecek en uzun sondur: gizli değer yoksa ya da metnin sonu hiçbir gizli değerle başlamıyorsa gecikme olmaz.
  - Tutma kuralı toplu maskenin kararından çıkar, "sonu bir gizli değerin başı olan kısım" sezgisinden değil: önce maskeleyip sonu tutmak kısa bir gizli değeri uzun olanın içinde erken maskeler; ham sonu tutup öncesini maskelemek sonu öbür değerin başı olan bir gizli değeri hiç maskelemez. İki sezgisel okunuş 200 000 rastgele denemede 28 ve 32 879 kez toplu maskeden ayrıldı, ikincisi 32 743 kez bütün bir gizli değeri açıkta bıraktı; 19.4a'nın kuralı hiç ayrılmadı [ölçüldü: DENETİM 373, Go `strings.Replacer` benzetimi]. Örtüşme kuralı bu yüzden 19.4'e yazıldı.
  - Maske 16.3a'nın çözme akışına konmadı: `.TR_step`'in sonucu programın verisidir (araç çağrısının argümanındaki anahtar gerçek kalmalı, 19.4 "veri değişmez"), uzaktaki modelin API akışı da 16.3a'dan geçmez.
  - Terminalde de kendiliğinden tutulmaz: VM, satır sonu olmadan yazılan parçanın ne zaman boşaltılacağını bilemez; tutsaydı istem ve canlı satırla çıktının zamanı değişirdi. İki `TR_print`'e bölünen değer okuyana açıktır [ölçüldü: DENETİM 373].
  - Kendiliğinden (olay akışının içinde) geri tutmak olmazdı: iki olayın hangi metinlerinin birleşeceğini (`append`) yalnız program bilir; motorun olayın verisini bölmesi 19.4'ün "veri değişmez"iyle çatışırdı.
  - Akış sürerken eklenen gizli değerin önceden verilmiş başını hiçbir yol geri alamaz; söz bunu açıkça dışarıda bırakır.
  - **Açık adres bölgesi (2026-11-13):** 19.4'ün bağlamlı maskesi tutulanı genişletir: sınırı gelmemiş bölge `://`'dan başlayarak tutulur, sondaki `:` ve `:/` de. Bölgenin uzunluğu sınırsızdır. Seçilmeyen yol bölgeye uzunluk sınırıydı: toplu maske de aynı sınırı uygulamak zorunda kalırdı ve 19.4'ün adres kuralına bir sayı girerdi. Bedel: boşluksuz uzun bir `://`'lu metin sınırı ya da bitişi gelene kadar gösterilmez. Prototipte akış toplu maskeyle 200 000 rastgele metin ve bölmede hep aynı sonucu verdi [ölçüldü: `motor-194-ikinci-deneme/proto.txt`].

### 19.5

- *Emsal:* TensorBoard ve Weights & Biases ölçüleri olay kaydı olarak yazar; JSON Lines yaygın bir düz biçimdir [kanıtsız: bu tur açılmadı].
- *Gerekçe:* Güvenilir: kesilen eğitim aynı günlüğe sürer (sonuna ekleme), çöken süreç en çok bir yarım satır bırakır. Biçimin anahtarları dilden bağımsızdır; başka araçlar günlüğü okur (17.3).

### 19.6

- *Emsal:* yok; TR'nin günlük biçimi.
- *Gerekçe:* sürdürülen eğitimde kesilmeden önce yazılmış adımlar yeniden hesaplanır; okuyucu hangi kayıtların geçerli olduğunu kuraldan bilmelidir, silme yoktur (sonuna ekleme).
- *Tur 6C (2026-11-10, `offset`):* PyTorch'un `SummaryWriter`'ı `purge_step` ile kesilip yeniden başlayan günlükte büyük adımlı olayları gizler; TensorBoard `--purge_orphaned_data` ile yeniden başlama olayından büyük adımları atar [kaynak: github.com/pytorch/pytorch `torch/utils/tensorboard/writer.py`; github.com/tensorflow/tensorboard README; Gemini Uzak 12, DENETİM 402]. JSON Lines her değerden sonra satır sonu ister [kaynak: jsonlines.org]. *Gerekçe:* satır ortasına düşen `offset` bozuk dosyadır, sessizce yarım satır atlamak yanlış kayıpları geçerli sayardı; kesilmiş günlük ise 20.6'nın uyarıyla izin verdiği bir durumdur, hata değildir.

### 19.7

- *Emsal:* Python'un `tqdm`'i adım sayar, hızı üstel ortalamayla verir [kanıtsız: bu tur açılmadı].
- *Gerekçe:* eğitimi izleyen insan ve ajan ilerlemeyi ve kalan süreyi görür. Anlamsız `TR_true` dönüşü ajanın onu bir koşul sanmasına yol açar (13.10); bugün `TR_update` ve `TR_append` `TR_true` döndürüyor [ölçüldü: main `217ae940`].

### 19.8

- *Emsal:* yok.
- *Gerekçe:* çubuğun kendi yazdığı anahtarlar kullanıcının ölçüleriyle karışmamalıdır; karışırsa günlüğün okuyucusu yanlış adımı okur.

### 19.9

- *Emsal:* yok.
- *Gerekçe:* çubuk ve günlük aynı adımları iki kez saymamalıdır; bağlı çubuk günlüğe kendisi yazar.

### 19.10

- *Emsal:* `argparse` `--help` ve `-h` verir, kullanım hatasında 2 koduyla çıkar [kanıtsız: bu tur açılmadı].
- *Gerekçe:* komut satırı girdidir ve kimlik kodlu dilde girdi tek yazımlıdır (17.9; baş geliştirici 2026-10-29: "kabul edelim ki dünyadaki tüm yazılım dilleri ingilizce"). Bugün yardımın Türkçe ve İngilizce adı ikisi de çalışıyor, mantık değerinde `doğru` da okunuyor; eski metnin `sonsuz`'u ise okunmuyor [ölçüldü: main `217ae940`]. İki yazım ajanın hangisinin doğru olduğunu tahmin etmesi demektir. Yardımın varsayılanları komut satırının yazımıyla gösterilir, böylece kopyalanan değer çalışır.

### 19.11

- *Emsal:* `tqdm` standart hataya yazar ve terminal değilse daha seyrek yazar [kanıtsız: bu tur açılmadı].
- *Gerekçe:* çubuk programın çıktısına karışırsa çıktıyı okuyan program bozulur. Çubuk bir görünümdür, kişinin dilinde gösterilir (17.7); iş durumu dosyası ise veridir ve dilden bağımsızdır.

### 19.12

- *Emsal:* psutil'in açık hatası "/proc/meminfo shows the information from the host and not the container" (#2100); sysinfo sınırı ayrı verir. `platform.machine()` "may differ in casing and naming conventions". Go 1.25+: `GOMAXPROCS` cgroup CPU sınırını izler, `NumCPU` izlemez.
- *Gerekçe:* tek yazım (15.37'nin kalıbı); konteyner yanılgısı sessizdir.
