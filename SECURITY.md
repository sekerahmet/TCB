# Güvenlik

## Açık nasıl bildirilir

Güvenlik açıklarını herkese açık bir konu (issue) olarak **açmayın**. Deponun **Security** sekmesindeki **Report a vulnerability** düğmesiyle (GitHub'ın özel güvenlik açığı bildirme özelliği) bildirin; bildirim yalnızca proje sahibine görünür.

Bildirimde şunlar olsun: etkilenen girdi ya da komut, açığı gösteren en küçük dosya veya program, beklenen ve gerçekleşen davranış, kullandığınız işletim sistemi ve `tr sürüm` çıktısı. Bildirim doğrulanır; düzeltme ana dala alındıktan sonra açık, bildirenle birlikte açıklanır.

## Desteklenen sürüm

Yalnızca ana dal (`main`) desteklenir. Ayrı sürüm dalı ya da eski sürümlere düzeltme yoktur; düzeltmeler ana dala girer.

## Güvenilmeyen girdiler

TR, bir yapay zekâ modeli geliştiricisinin başkasından aldığı dosyaları okur. Aşağıdaki girdiler **güvenilmeyen** sayılır: bozuk ya da kötü niyetli bir dosya programı çökertmemeli, sonsuz döngüye sokmamalı, sınırsız bellek ayırmamalı, okunması istenen yerin dışına yazmamalı; açık bir Türkçe hata vermeli (`dene/yakala` ile yakalanabilir). Bunlardan biri olmuyorsa bu bir güvenlik hatasıdır.

| Girdi | Okuyan | Korumalar |
| :-- | :-- | :-- |
| `.trt` tensör dosyaları | `tensör_yükle` | biçim ve sürüm denetimi, sağlama toplamı, tensör başına eleman sınırı, fiziksel bellek sınırı ([kılavuz §4.C](docs/guide/04_yapay_zeka_ve_veri.md#c-tensörler-ve-otomatik-türev), [dil tanımı 15.10 ve 15.25](docs/spec/15_tensorler.md)) |
| Kontrol noktaları (`<koşu>/kontrol_noktalari/`) | `kontrol_noktası` | JSON dosyalarında boy sınırı (64 KB / 64 MB) ve katı çözüm, biçim sürümü, adımın klasör adı ve dosyalar arasında tutması, her dosyanın SHA-256 özeti, tensör dosyasının bütün `tensör_yükle` sınırları; biçimde yalnızca veri var (kod, nesne kurma, yol çözme yok); bozuk nokta açık hata, önceki nokta önerilir ([dil tanımı §20](docs/spec/20_kontrol_noktasi.md)); bulanıklaştırma: `FuzzNoktaDosyalari` |
| `.npy` veri dosyaları | `veri_kümesi` | başlık boyu sınırı, tür ve şekil denetimi |
| `tokenizer.json` parçalayıcı dosyaları | `parçalayıcı_yükle` | düzenli ifadeler adım bütçesiyle çalışır, üstel geri izlemeye açık ve çok derin iç içe desenler yüklenmez ([kılavuz §4.D](docs/guide/04_yapay_zeka_ve_veri.md)) |
| CSV ve JSON metinleri | `csv_oku`, `csv_çöz`, `json_çöz` | biçim hataları açık hata verir |
| `tr derle` ile üretilmiş uygulamanın gömülü paketi | uygulamanın kendisi | bozuk (yarım indirilmiş, kırpılmış) paket açık hata verir, modüller geçici klasörün dışına yazılmaz |
| Hata ayıklama iletileri | `tr dap` | iletiler standart giriş/çıkıştan gelir; ağ portu açılmaz |

**TR programları güvenilmeyen girdi değildir.** Bir `.tr` programı ya da `tr derle` ile üretilmiş bir uygulama, her program gibi, onu çalıştıran kişinin yetkileriyle dosya okur ve yazar. Yorumlayıcı bir kum havuzu (sandbox) değildir; tanımadığınız bir programı çalıştırmayın.

**Yüklenen dış kitaplıklar (GPU).** GPU arka ucu çalışma anında yalnızca NVIDIA'nın sürücüsünü (Linux'ta `libcuda.so.1`, Windows'ta sistem klasöründen `nvcuda.dll`) ve makinede kuruluysa matris çarpımı kitaplığını (cuBLASLt: `libcublasLt.so.13/.12`, `cublasLt64_13/12.dll`) yükler; TR bu kitaplıkları dağıtmaz. Linux'ta `LD_LIBRARY_PATH` boş öğe ya da göreli yol içeriyorsa sürücü de standart arama yoluyla değil, yalnızca bilinen sürücü klasörlerinden tam yolla yüklenir. cuBLASLt yalnızca şu yerlerde aranır: `TR_GPU_CUBLASLT` (tam yol olmalı), Linux'ta standart kitaplık arama yolu (yalnızca `LD_LIBRARY_PATH` mutlak klasörlerden oluşuyorsa), bilinen CUDA kurulum klasörleri, Python'un pip ile kurduğu NVIDIA paketlerinin bilinen klasörleri ve TR'nin kendi önbellek klasörü. **Çalışma klasörü ve programın yanındaki klasör hiçbir zaman aranmaz** (kitaplık kaçırma). Windows'ta DLL'in bağımlılıkları yalnızca DLL'in kendi klasöründen ve sistem klasöründen yüklenir. Bulunamazsa TR aynı GPU'da kendi çekirdeğini kullanır; hangi yolun kullanıldığı GPU durum mesajında görünür ([mimari](docs/architecture.md#gpu-arka-ucu-internalcomputegpu)).

**Ağ.** Yorumlayıcı bugün hiçbir ağ bağlantısı açmaz. İleride bir sunucu eklenirse yalnızca `127.0.0.1`'e bağlanır.

**VS Code eklentisi.** Eklenti yalnızca güvenilen yorumlayıcıyı çalıştırır: yorumlayıcının yolu (`tr.yorumlayiciYolu`) yalnızca kullanıcı ayarından okunur, çalışma alanının kendi ayar dosyası onu değiştiremez; güvenilmeyen (kısıtlı kipteki) çalışma alanında hiçbir şey çalıştırılmaz; yorumlayıcı üst klasörlerde aranmaz ([eklenti belgesi](editors/vscode/README.md)). Kodlu saklanan dosyaların Türkçe görünümü de yorumlayıcıyı (`tr göster -`, `tr kodla -`; metin standart girişten verilir, kabuk kullanılmaz) aynı kuralla çalıştırır; kısıtlı kipte kodlu dosya çevrilmez, salt okunur ham hâliyle gösterilir.

## Bulanıklaştırma testleri

Yukarıdaki her girdinin bir bulanıklaştırma testi vardır (`go test` içinde `Fuzz` ile başlayan testler: tensör dosyası, `.npy` başlığı, parçalayıcı ve düzenli ifadeler, CSV, JSON, gömülü paket, hata ayıklama iletileri, sözcük çözümleyici, derleyici, biçimlendirici). Sıradan `go test` yalnızca kayıtlı girdileri koşar; otomatik sınama her hafta her hedefi rastgele girdiyle ayrıca koşar (`.github/workflows/fuzz.yml`). Yerelde tek bir hedef:

```sh
bash tools/fuzz.sh 30s '^FuzzTensorKaydiCoz$'
```

## Bağımlılık ve araç zinciri açıkları

govulncheck her gönderimde ve haftada bir, Linux, Windows ve macOS hedefleri için kodun gerçekten çağırdığı yollarda Go'nun açık veri tabanındaki (vuln.go.dev) bilinen açıkları arar (`.github/workflows/vuln.yml`). Açık standart kitaplıktaysa `go.mod`'daki `toolchain` satırı düzeltmeyi içeren, hâlâ desteklenen bir Go sürümüne yükseltilir (Go her ana sürümü, iki yeni ana sürüm çıkana kadar destekler). Yerelde:

```sh
go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...
```
