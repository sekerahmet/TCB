# 17. Kodlar: emsal ve gerekçe

Kurallar: [`17_kodlar.md`](../17_kodlar.md). Bu dosya her kuralın emsalini (başka dillerde nasıl olduğu, neyi almadığımız, kaynaklarıyla) ve gerekçesini tutar. Dili kullanmak için kuralların kendisi yeter; bu dosya kararları tartmak ve denetlemek içindir.

### 17.1

- *Emsal:* Excel ve LibreOffice'in yerelleştirilmiş fonksiyon adları içte tek bir kimliğe bağlıdır ve kayıtta kanonik adla yazılır; kullanıcının hücre adları ise yazıldığı gibi kalır. Hedy ve Racket'in yerelleştirilmiş dilleri de yalnız dilin sözcüklerini çevirir. *Almadık:* yalnız anahtar kelimeleri kodlamak (Hedy): yerleşikler ve kitaplık İngilizce kalırdı, okuyanın en çok gördüğü adlar çevrilmezdi.
- *Gerekçe:* TR'yi en çok yapay zekâ ajanları yazar, insanlar okur. Ajan için yazımın hangi dilde olduğunun bedeli yoktur; okuyan için ise çeviri her şeydir. Bu yüzden çeviri olabildiği her yere uzanır: dilin sözcükleri ve TR ile gelen kitaplık. Kullanıcının ve başka kitaplıkların adları açık uçlu bir kümedir; her yazardan kod kaydı istenemez ve adlar yazarın katmanıdır. Bu sınır kararını baş geliştirici verdi (2026-10-29): kitaplığın İngilizce düz adla yazılması önerisi tartıldı, CLAUDE.md'nin "hiçbir dilin ayrıcalığı yoktur" ilkesiyle ve okuyanın kazancıyla kodlu kalması seçildi. `TR_` öneki ajana da okuyana da "bu dilin parçası" der; düz ad "bu kullanıcının".

### 17.2

- *Emsal:* Go'nun ve Rust'ın tek bir yazımı vardır; çeviri yoktur. Excel'in içteki kimliği ile gösterimdeki adı ayrıdır.
- *Gerekçe:* program metninde sınıf yazılmaz, çünkü ölçümde sınıflı yazım (`TR_fn_sum`) kısa yazımdan %19 daha çok parça tutuyordu ve sınıfı tahmin etmek ajanın en sık hatasıydı [ölçüldü: 2026-10-27, kısa yazım kararının ölçümü; ölçüm dosyası depoda değil]. Sınıfı yazıldığı yer zaten söyler. İçeride sınıf kalır, çünkü aynı kısa ad birden çok koddur: 2026-10-29 sayımında 60 kısa ad iki ya da üç sınıftadır (`TR_log` fonksiyonda logaritma, adlı argümanda kayıt; `TR_parameters` fonksiyon, argüman ve üye) [ölçüldü: `definitions.json`]. Kısa yazıma çevrilen bir tanım dosyası bu ayrı kodları birleştirirdi.

### 17.3

- *Emsal:* Rust'ın ad alanları: "Names are segregated into separate namespaces based on the kind of entity the name refers to. Namespaces allow the occurrence of a name in one namespace to not conflict with the same name in another namespace." ([Rust Reference, Namespaces](https://doc.rust-lang.org/reference/names/namespaces.html))
- *Gerekçe:* ad alanı, sınıfı yazıldığı yerin belirlediği kümedir; aynı yerde iki kod aynı adı taşıyamaz, ayrı yerlerde taşıyabilir. Komut satırının adları programın dışındadır ve ayrı alanlardadır.

### 17.4

- *Emsal:* Swift ve Kotlin standart kitaplığının belgeleri kaynaktan üretilir; tanım ile belge aynı yerden gelir.
- *Gerekçe:* tanım dilden bağımsızdır, ad dile bağlıdır: yeni bir gösterim dili tek bir dosyadır, tanıma dokunmaz. "Kayıtlı her kod canlıdır" kuralı ölü kodu önler: kayıtta durup hiçbir yerde çalışmayan bir ad, ajana var olmayan bir işi yazdırır. Bu yüzden dil tanımına yeni giren ad (15'in `TR_all`'ı gibi) tanım dosyasına uygulandığı gün girer; o güne kadar dil tanımının kimlik kapısı onu bilerek yazılmış ad olarak tanır.

### 17.5

- *Emsal:* matematik ve makine öğrenmesi gösterimi uluslararasıdır: `sin`, `log`, `argmax`, `relu`, optimizer adları (AdamW, Muon) her dilde aynı yazılır.
- *Gerekçe:* her işin tek adı vardır (2.9). Ortak ad iki dilde ayrı yazılsaydı aynı kavramın iki gösterimi olurdu. "Kısa ve düz" ilkesi okunurluk içindir: kod okunurken ad cümlenin parçasıdır (`TR_if hepsi(t >= 0.0)` "eğer hepsi" diye okunur); var olan Türkçe adların %60'ı tek sözcüktür, ortalaması 9 harftir [ölçüldü: `tr.json`, 2026-10-29]. Gösterim adını baş geliştirici onaylar ([`docs/development.md`](../../development.md)).

### 17.6

- *Emsal:* derleyicilerin "bunu mu demek istediniz" önerisi (Rust, Swift) [kanıtsız: belgeler bu tur açılmadı].
- *Gerekçe:* ajan PyTorch ve Python alışkanlığıyla yazar; alışkanlık adı kabul edilseydi bir işin iki yazımı olurdu. Kabul etmeyip doğru kimliği önermek hatayı tek adımda düzelttirir. Kaldırılmış ad `familiar` olmaz, çünkü dil taslaktır ve geriye uyumluluk tutulmaz (17.10).

### 17.7

- *Emsal:* Excel kaydı kanonik adla yapar, gösterimi kişinin dilinde verir. Yerelleştirilmiş programlama dillerinde (Hedy) dosya dile bağlıdır; TR'de dosyanın dili yoktur.
- *Gerekçe:* görünüm bir okuma aracıdır, programın kendisi değildir. Bu yüzden çalışmaz: bir görünüm dosyasını çalıştırmak, programın dile bağlı ikinci bir biçimi olması demekti. Bayt bayt gidiş-dönüş, görünümde yapılan düzenlemenin kodlu dosyada yalnız o düzenleme olarak görünmesini sağlar. `$` kaçışı yalnız görünümdedir, çünkü kodlu dosyada dilin sözcüğü `TR_` ile ayrılır ve çakışma olmaz.

### 17.8

- *Emsal:* Python, Java, Swift ve Go değerin metnini yerelleştirmez. Excel ve LibreOffice çalıştıranın diline göre değiştirir: aynı program farklı sonuç verir. PyTorch'un `state_dict`'i ve Hugging Face'in `config.json`'u İngilizce ve dilden bağımsızdır.
- *Gerekçe:* Güvenilir: aynı program aynı veriyle aynı sonucu verir, kimin hangi dilde okuduğundan bağımsız. Tipin metni sınıflı kimlik değil kimliğin adıdır; bugün kitaplık tipi `TR_type_Linear` yazıyor [ölçüldü: main `1ef66921`, `TR_print(TR_Type(TR_Linear(2, 3, TR_seed: 1)))`], içerideki biçim değere sızıyor.

### 17.9

- *Emsal:* Go, git ve cargo komut adlarını tek (İngilizce) yazımda tutar. PowerShell'in yerelleştirilmiş adları içte bir kimliğe bağlıdır ama yalnız etkin dili kabul eder. *Almadık:* yalnız etkin dili kabul etmek: betik `TR_DIL`'e bağlı kalırdı.
- *Gerekçe:* komut satırının kodlu biçimi yoktur; girdide her gömülü dilin adını kabul etmek betiği taşınabilir yapar, çıktıyı kişinin dilinde vermek okunur yapar.

### 17.10

- *Emsal:* *Almadık (şimdilik):* Go 1'in uyumluluk sözü: "It is intended that programs written to the Go 1 specification will continue to compile and run correctly, unchanged, over the lifetime of that specification." Bu söz yayımdan sonra TR'ye de gelir; taslakta değil. ([Go 1 and the Future of Go Programs](https://go.dev/doc/go1compat))
- *Gerekçe:* dil taslaktır ve geriye uyumluluk bir hedef değildir: eski adı tutmak bir işin iki yazımı demektir. Kıran değişiklik başlığın sürümüyle görünür, eski dosya sessizce yanlış okunmaz.
