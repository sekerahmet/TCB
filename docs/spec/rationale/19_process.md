# 19. Süreç ve kayıt: emsal ve gerekçe

Kurallar: [`19_surec_ve_kayit.md`](../19_surec_ve_kayit.md). Bu dosya her kuralın emsalini (başka dillerde nasıl olduğu, neyi almadığımız, kaynaklarıyla) ve gerekçesini tutar. Dili kullanmak için kuralların kendisi yeter; bu dosya kararları tartmak ve denetlemek içindir.

### 19.1

- *Emsal:* Python'un `os.environ.get(ad)`'ı tanımsızda `None` verir; `os.environ` bütün ortamı bir sözlük olarak açar [kanıtsız: bu tur açılmadı]. *İkincisini almadık.*
- *Gerekçe:* uzak modelin anahtarı (`HF_TOKEN`, `OPENAI_API_KEY`) ortamda durur; bütün ortamı yazdıran bir satır onları günlüğe döker. Tek bir değişkeni adıyla okumak yeter.

### 19.2

- *Emsal:* Python'un `argparse`'ı ve `typer`'ı seçenekleri bir şemadan kurar; `typer` ve Rust'ın `clap`'i (derive) bir yapının ya da fonksiyonun tiplerinden [kanıtsız: bu tur açılmadı].
- *Gerekçe:* seçenekler bir yapı olunca her seçeneğin tipi çalışmadan önce bilinir ve yanlış yazılmış bir seçenek adı (`ayar.adim`) denetimde yakalanır (1.2, 13.10). Sözlükle yazılan seçenekler tek tipli olamaz (3.7) ve yanlış anahtar ancak çalışırken görünür. Bugün seçenekler bir sözlükle veriliyor [ölçüldü: main `217ae940`].

### 19.3

- *Emsal:* `argparse` seçenek adını yazıldığı gibi alır, `-`'yi `_`'ye çevirir [kanıtsız: bu tur açılmadı]. *Çevirmeyi almadık.*
- *Gerekçe:* kullanıcının adı düzdür ve çevrilmez (17.1); seçeneğin tek yazımı vardır, ajan ve kullanıcı aynı adı yazar. Bir adın iki yazımı ajanın iki ayrı seçenek sandığı bir adı doğurur.

### 19.4

- *Emsal:* GitHub Actions gizli değerleri günlükte `***` ile maskeler; değer dönüştürülünce eşleşmez [kanıtsız: bu tur açılmadı].
- *Gerekçe:* ajanlar programın çıktısını okur ve başka yerlere taşır; çıktıya düşen bir anahtar sızmış bir anahtardır. Veriyi değiştirmemek programın doğru çalışmasını korur: maskeleme yalnız insanın göreceği yerdedir.

### 19.5

- *Emsal:* TensorBoard ve Weights & Biases ölçüleri olay kaydı olarak yazar; JSON Lines yaygın bir düz biçimdir [kanıtsız: bu tur açılmadı].
- *Gerekçe:* Güvenilir: kesilen eğitim aynı günlüğe sürer (sonuna ekleme), çöken süreç en çok bir yarım satır bırakır. Biçimin anahtarları dilden bağımsızdır; başka araçlar günlüğü okur (17.3).

### 19.6

- *Emsal:* yok; TR'nin günlük biçimi.
- *Gerekçe:* sürdürülen eğitimde kesilmeden önce yazılmış adımlar yeniden hesaplanır; okuyucu hangi kayıtların geçerli olduğunu kuraldan bilmelidir, silme yoktur (sonuna ekleme).

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
- *Gerekçe:* komut satırı girdidir ve kimlik kodlu dilde girdi tek yazımlıdır (17.9; baş geliştirici 2026-10-29: "kabul edelim ki dünyadaki tüm yazılım dilleri ingilizce"). Bugün `--yardım` ve `--help` ikisi de çalışıyor, mantık değerinde `doğru` da okunuyor; eski metnin `sonsuz`'u ise okunmuyor [ölçüldü: main `217ae940`]. İki yazım ajanın hangisinin doğru olduğunu tahmin etmesi demektir. Yardımın varsayılanları komut satırının yazımıyla gösterilir, böylece kopyalanan değer çalışır.

### 19.11

- *Emsal:* `tqdm` standart hataya yazar ve terminal değilse daha seyrek yazar [kanıtsız: bu tur açılmadı].
- *Gerekçe:* çubuk programın çıktısına karışırsa çıktıyı okuyan program bozulur. Çubuk bir görünümdür, kişinin dilinde gösterilir (17.7); iş durumu dosyası ise veridir ve dilden bağımsızdır.
