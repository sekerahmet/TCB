# Mimari: katmanlar ve paketler

TR tek bir ikilidir (`tr`). Bütün Go kodu `internal/` altındadır; başka bir Go modülü bu paketleri içe aktaramaz. Kullanıcı Go paketi değil `.tr` dosyası yazar.

## Katmanlar

```
araçlar (internal/devtools, cmd/tr) ─▶ motorlar (internal/stdlib) ─▶ dil çekirdeği (internal/lang)
                                        │
                                        └──────▶ hesap çekirdekleri (internal/compute)
TR ile yazılı kitaplık (lib/*.tr) ── ikiliye gömülü; cmd/tr yorumlayıcıya bağlar (aktar "tr/...")
```

Ok "içe aktarabilir" demektir. Kural `internal/layers_test.go` ile denetlenir (`go test ./internal`); yalnızca paketlerin kendi (test dışı) içe aktarmalarına uygulanır.

| Katman | İçe aktarabileceği iç paketler |
| :-- | :-- |
| `internal/compute/**` | yalnızca `internal/compute/**` (dili bilmez; tek başına sınanır ve ölçülür) |
| `internal/lang/**` | yalnızca `internal/lang/**`, iç sırasına göre (aşağıda) |
| `internal/stdlib/**` | `internal/lang`, `internal/compute`, `internal/stdlib` |
| `internal/devtools/**` | `internal/lang`, `internal/compute`, `internal/stdlib`, `internal/devtools` |
| `cmd/tr` | hepsi (`internal/reporoot` dışında) |
| `internal/reporoot`, `lib` | hiçbiri |
| `tools/**` | hepsi |

Dil çekirdeğinin içinde de sıra vardır: bir dil paketi yalnızca kendisinden önce gelenleri içe aktarabilir (`internal/layers_test.go`, `langOrder`):

```
turkce < kod < ipucu < sozcuk < agac < ayristir < denetci < kaynak < yurut
```

Yorumlayıcı (`internal/lang/interp`) motorları ve hata ayıklayıcıyı içe aktarmaz; onlar yorumlayıcıya kendileri bağlanır: `internal/stdlib` paketi yüklenirken yerleşikleri `interp.AddEngine` ile, `internal/devtools/debugger` paketi yüklenirken koddaki `kesme_noktası` deyiminin açacağı hata ayıklayıcıyı `interp.NewDebugger` ile kaydeder. Tam dili çalıştıran program ikisini de içe aktarır (`cmd/tr`, `internal/devtools/testrunner`).

## Paketler

| Paket | Görevi |
| :-- | :-- |
| `cmd/tr` | `tr` komutu: alt komutları ayrıştırır (çalıştır, kabuk, `kontrol`, `derle`, `test`, `biçimle`, `kodla`, `çöz`, `göster`, `izle`, `profil`, `hata_ayıkla`, `dap`), işi `internal/`'a bırakır |
| `cmd/tr/e2e` | `tr` komutunun uçtan uca testleri (yalnızca testler): ikiliyi bir kez derler, alt süreç olarak çalıştırır; komutlar, çıkış kodları, kesme, kontrol noktasından devam, belgelerdeki kod blokları ve bağlantılar, VS Code eklentisinin dosyaları ve JavaScript testleri |
| `internal/lang/codes` | Dilin bütün adlarını sabit kodlara bağlayan kod listesi (`definitions.json`, `tr.json`) ve ondan üretilen Go sabitleri |
| `internal/lang/turkish` | Türkçe harf işlemleri, ekler, "bunu mu demek istediniz" önerileri ve hata çerçevesi |
| `internal/lang/hint` | Dilin adlarıyla ilgili hata ipuçları: kaldırılan, taşınan ve Türkçe karaktersiz yazılan adlar, tanımsız ada en yakın ad (sözcük çözümleyici, ayrıştırıcı, denetleyici ve yorumlayıcı aynı metni kullanır) |
| `internal/lang/lexer` | Sözcük çözümleyici: kaynak metni sözcük birimlerine ayırır; kodlu gövdede kodları etkin dilin adlarına çevirir, konumları Türkçe gösterime göre verir |
| `internal/lang/ast` | Sözdizimi ağacının düğümleri |
| `internal/lang/parser` | Ayrıştırıcı: sözcük birimlerinden sözdizimi ağacını kurar |
| `internal/lang/checker` | Çalıştırmadan önceki tip denetimi ve uyarılar; adların kapsam bilgisi (tanımsız ad denetimi ve kodlu saklamanın ad bağları) |
| `internal/lang/source` | Kaynaktan programa tek giriş noktası: dosya metnini bölümlerine ayırır (BOM, kodlu biçim başlığı), doğru sözcük çözümleyiciyi seçer, ayrıştırır, isteğe göre denetler (`Hazirla`); komut satırı, kabuk, test koşucusu, hata ayıklama sunucusu, modül yükleyici ve `tr derle` dosyaları yalnızca buradan okur. Kodlu saklamanın çeviricisi (`Kodla`, `Coz`) de buradadır |
| `internal/lang/interp` | Derleyici ve yorumlayıcı: değerler, çekirdek yerleşikler ve metotlar, modüller, eşzamanlı görevler, motor ve hata ayıklayıcı kancaları |
| `internal/stdlib` | Bütün motorları doğru sırayla yorumlayıcıya kaydeder (`import _ "github.com/sekerahmet/TCB/internal/stdlib"`) |
| `internal/stdlib/ml` | Tensör, matris, otomatik türev, sinir ağı katmanları, iyileştiriciler ve tensör kaydını TR yerleşikleri olarak dile bağlar |
| `internal/stdlib/tokenizer` | `parçalayıcı_yükle` ve parçalayıcı nesnesinin üyeleri |
| `internal/stdlib/dataset` | `veri_kümesi`, örnekleyici ve sıralı okuyucu yerleşikleri |
| `internal/stdlib/formats` | JSON ve CSV yerleşikleri |
| `internal/stdlib/files` | Standart kitaplık: `Yol` tipi, `yol`, `klasörü_listele`, `klasör_oluştur`, `sil`, `kopyala`, `taşı`, `dosya_satırları` (silme korumaları, bağlantı izlememe, `os.Root`) |
| `internal/stdlib/process` | Süreç yerleşikleri: `ortam_değişkeni`, `komut_satırı` (seçenekler, `--yardım`, kullanım hatası 2), gizli adların süzgece kaydı |
| `internal/stdlib/logging` | Eğitim günlüğü (`günlük`, JSON satırları) ve ilerleme çubuğu (`ilerleme`; canlı satır, `ilerleme.json`), adım sınırı kancası |
| `internal/stdlib/checkpoint` | Kontrol noktası (`kontrol_noktası`): eğitimin bütün durumunun koşu klasörüne atomik çok dosyalı kaydı, doğrulanarak yüklenmesi ve kaldığı yerden devam; ertelenen kesme |
| `internal/stdlib/common/bind` | Motorların ortak yardımcıları: adlı argüman denetimi, göreli yol, panikten hataya, Türkçe sayı yazımı |
| `internal/stdlib/common/jsonwrite` | TR değerlerini JSON'a yazar |
| `internal/compute/tensor` | n boyutlu tensör, görünümler, otomatik türev, arka uç arayüzü ve CPU arka ucu (AVX2/FMA çekirdekleri) |
| `internal/compute/gpu` | GPU arka ucu: sürücü bağlama (cgo'suz), Go içinden üretilen PTX çekirdekleri, cihaz belleği havuzu, PTX benzetimi (donanımsız sınama) |
| `internal/compute/tokenize` | `tokenizer.json` parçalayıcıları: BPE, WordPiece, Unigram, normalleştirme, ön bölme |
| `internal/compute/dataload` | `.npy` veri dosyaları, veri kümesi, örnekleyici, önbellek ve önden yükleme |
| `internal/devtools/debugger` | Terminal hata ayıklayıcısı ve düzenleyicinin hata ayıklama sunucusu (`tr dap`) |
| `internal/devtools/formatter` | `tr biçimle` biçimlendiricisi |
| `internal/devtools/testrunner` | `tr test` komutunun çekirdeği |
| `internal/devtools/profiler` | Canlı profil: `tr izle` ve `tr profil`'in çekirdeği, arka plan işinin `profil.json`'u (aşağıda "Canlı profil") |
| `internal/reporoot` | Testlerin depo kökünü bulan yardımcı (testler `../../` yazmaz) |
| `lib` | TR ile yazılan standart kitaplık (`katman.tr`, `iyilestirici.tr`, testleri yanında); `gomulu.go` onları `go:embed` ile ikiliye gömer, `cmd/tr` yorumlayıcıya `interp.SetLibrary` ile bağlar. Kullanıcı `aktar "tr/katman"` yazar (dil tanımı 11.6); `tr derle` çıktısı yorumlayıcının kopyası olduğundan kitaplık ona da girer. Ayrıntı: [`lib/README.md`](../lib/README.md) |
| `tools/gen/*` | Üreteçler: Go kod sabitleri, `docs/kodlar.md`, eklentinin renklendirme listeleri (`go generate ./internal/lang/codes`) |
| `tools/bench/go` | Hız kıyas setinin derlenmiş başvuru sürümü |

### Yorumlayıcı kancaları

Motorlar ve standart kitaplık yorumlayıcıya (`internal/lang/interp`) şu kancalarla bağlanır; yorumlayıcı onları bilmez:

| Kanca | Görevi |
| :-- | :-- |
| `KitaplikBagla` | Dilin gömülü kitaplığı (`kitaplik.Dosyalar`): `aktar "tr/..."` buradan okur (11.6). Yorumlayıcı kitaplık paketini içe aktarmaz; komut satırı (ve dili tam çalıştıran testler) bağlar |
| `Yinelenebilir`, `Akis` | `döngü x içinde ...` ve liste üreteci: uzunluğu bilinen kaynak (tensör) ya da tembel akış (sıralı okuyucu) |
| `YolDegerli`, `YolArgumani` | Yol alan yerleşiklerin ortak okuyucusu (Metin ya da `Yol`; sekme/satır sonu ipucu, 13.1a) |
| `GorevliUyeKancasi` | Göreve özel üye okuma: sonucu çalışan koda bağlı özellikler (`Yol`'un diske bakan özellikleri kodun klasörüne göre çözülür) |
| `AtomikYaz`, `DosyaOkuyucuAc`, `DosyaIceriginiOku`, `DosyaIceriginiYaz` | Dosya okuyan ve yazan bütün yerleşiklerin ortak yardımcıları: atomik yazma, `.gz` saydamlığı (18.12, 18.13) |
| `CiktiYaz`, `CanliSatir` | İnsanın göreceği çıktı noktaları: etkin canlı satır (ilerleme çubuğu) silinip yeniden çizilir |
| `GizliDegerEkle`, `Maskele` | Gizli değer süzgeci: kayıtlı değerler çıktı noktalarında `***` olur (veri çıktılarında değil) |
| `ArgumanlariAyarla`, `Argumanlar` | Betiğin komut satırı argümanları (okunmazsa çıkışta uyarı) |
| `Kes`, `ErrKesildi`, `KesilmeMi` | Nazik çıkış: Ctrl+C sonrası yığın çözülür, `ertele`ler çalışır; motorların panik kurtarıcıları `KesilmeMi` ile kesilmeyi yeniden atar |
| `KesmeIste`, `KesmeyiErtele`, `DurdurmaIstendi`, `SimdiKes` | Ertelenen kesme: kesme isteğini ileten her yer (komut satırının sinyal işleyicisi, arka plan işinin durdurma isteği) `KesmeIste`'yi çağırır; bir kontrol noktası kurulduysa (`KesmeyiErtele`) istek yalnızca bayrak koyar, kontrol noktası adım sınırında kaydedip `SimdiKes` ile durdurur |
| `CikisIstegi`, `CikisIstegiMi` | Bir yerleşiğin istediği program çıkışı (`komut_satırı`: `--yardım` 0, kullanım hatası 2): `ertele`ler çalışır, `dene/yakala` yakalamaz; komut satırı ve `tr dap` mesajı yazıp kodu bildirir |
| `KomutAdi`, `ProgramKomutu` | Programı başlatan komut (`tr egit.tr`, derlenmiş uygulamada `uyg.exe`): yardım ve kullanım hatalarında gösterilir |

Motorların kendi kancaları:

| Kanca | Görevi |
| :-- | :-- |
| `gunluk.AdimKancasiAyarla` | Eğitimin adım sınırı: `İlerleme.güncelle`'nin her çağrısında (yorumlayıcı kilidi altında) adım ve zamanla çağrılır; canlı profil ms/adım'ı ve adım başına işlem dağılımını buradan ölçer |
| `gunluk.GunlukMu`, `Gunluk.Uzunluk`, `Gunluk.DevamYaz` | Kontrol noktası için: günlüğün o anki bayt uzunluğu (her zaman satır sınırı) ve sürdürmede `{"tür":"devam","adım":N}` kaydı |
| `surec.Durum.Ayarlar` | Programın `komut_satırı` sonucu (günlüğün başlangıç kaydı yazar; kontrol noktası da saklayabilir) |
| `gunluk.TumunuKapat` | Program bitince bitmemiş çubukları bitirir, günlükleri kapatır (`motor.Kapat`, `tr dap`) |
| `kontrol.Durumlu` | Kontrol noktasına katılan motor değerleri: `DurumSozlugu` ve `DurumuYukle` (TR'de `.durum`, `.durumu_yükle`); bugün okuyucular, ileride Üreteç. Arayüzü yazan bir motor `kontrol` paketini içe aktarmaz |
| `ogrenme.ParametreleriTopla`, `ParametreUyumu`, `TensoreYaz` | Modelin parametreleri (`parametreler(..., adlarıyla: doğru)` ile aynı adlar), yüklenecek kaynakla uyum denetimi ve var olan parametreye yerinde yazma; `parametreleri_yükle` ve kontrol noktası ortak kullanır |
| `ogrenme.TensorYazici`, `TensorDosyasiGez`, `Iyilestirici.DurumGorunumu` | Tensör dosyasını girdi girdi yazma ve okuma; iyileştirici durumunun kopyasız görünümü (büyük modelde bellekte ikinci kopya yok) |
| `ogrenme.TohumsuzKancasiAyarla` | Tohumsuz rastgelelik kullanılınca çağrılır (kontrol noktası bir kez uyarır) |

### GPU arka ucu (`internal/compute/gpu`)

`tensor.Backend`'in NVIDIA uygulamasıdır; otomatik türev ve üst katmanlar cihazdan bağımsızdır.

- **Kayıt:** paket init'te tembel bir arka uç kaydeder (`tensor.RegisterLazyBackend`); sürücü
  ancak bir cihaz sorulunca (`cihazlar()`, `.cihaza`, `varsayılan_cihaz`) yüklenir. Sürücü
  yoksa hiçbir şey kaydolmaz, her şey CPU'da çalışır.
- **Sürücü:** cgo ve CUDA araç takımı gerekmez. Linux'ta `libcuda.so.1` dış bağımlılık olan
  dinamik yükleyiciyle (`go.mod`; yalnızca `cuda_linux.go` içe aktarır, `internal/layers_test.go`
  denetler), Windows'ta `nvcuda.dll` yalnızca sistem klasöründen (LoadLibraryExW) yüklenir.
  **Bedel:** bu yükleyici Linux ikilisini glibc'ye dinamik bağlar (`/lib64/ld-linux-x86-64.so.2`;
  `libc.so.6`, `libdl.so.2`, `libpthread.so.0`), GPU kullanmayan Linux kullanıcıları için de;
  musl tabanlı dağıtımlarda (Alpine) ikili açılmaz. `-tags gpusuz` ile derlenen ikili statik
  kalır ve GPU desteği içermez. **Karar:** varsayılan ikili GPU desteklidir (glibc'ye bağlı);
  statik ikili isteyen `GOOS=linux CGO_ENABLED=0 go build -tags gpusuz ./cmd/tr` ile derler.
  Tam denetim `gpusuz` derlemenin statik kaldığını `tools/checkstatic` ile doğrular.
- **Bağlam ve iş parçacığı:** CUDA bağlamı işletim sistemi iş parçacığına bağlıdır. Her dışa
  açık yöntem girişte arka ucun kilidini alır, gorutini iş parçacığına kilitler ve cihazın
  birincil bağlamını bağlar (`cuCtxSetCurrent`); bütün işler tek akışta sırayla yürür.
  Ayrı bir GPU gorutinine kanalla iş göndermek yerine bu yol seçildi: kanal gidiş-dönüşü ve
  gorutin geçişi yok, panikler (Türkçe hatalar) çağıranın gorutininde kalır (`dene/yakala`).
- **Çekirdekler:** Go içinden PTX olarak üretilir (`ptx.go` yazıcı, `cekirdek_*.go` aileler;
  hedef `sm_52`, cp.async kullananlar `sm_80`), sürücü ilk kullanımda karta derler, modüller
  süreç boyunca önbellekte. Çekirdek imzası her gösterici parametresinin yazılıp yazılmadığını
  bilir; başlatma yeri okuma/yazma bildirir ve uyuşmazlık iç hatadır. Döşeme boyları arka ucun
  ayarıdır. Birleşik çekirdekler: çapraz entropi (tek geçişli logsumexp, gradyan doğrudan
  yazılır), dikkat (puan matrisi belleğe yazılmaz; döşemeli çevrimiçi softmax, atomiksiz geri
  yön; `cekirdek_flash*.go`; geri yön kartın blok başına açtığı paylaşımlı bellek yetiyorsa
  64×64 döşemeyle devingen paylaşımlı bellekte (`.extern .shared`, işlev başına
  `cuFuncSetAttribute`), yetmiyorsa 48 KB statik bellekte 32×32), çok tensörlü iyileştirici/ölçekleme/kareler toplamı (bütün
  parametreler tek başlatmada; `cekirdek_cok.go`). Az çıktılı uzun indirgemeler iki aşamalı
  ve belirlenimcidir. Çıktısını tümüyle yazan işlemler (çarpım, çapraz entropi ve dikkat
  gradyanı) taze gradyanı sıfırlamadan üzerine yazar (`tensor.GemmSetter` vb.). q, k, v aynı
  tensörse dikkatin gradyanları tek (geçici) tamponda birikir. Dikkat yolu
  `TR_GPU_DIKKAT=birlesik|carpim` ile zorlanabilir (ölçüm; varsayılan birleşik). Çok tensörlü
  işte 16 bitlik açılmaların geçici belleği 256 MB'yi aşarsa iş parçalara bölünür; aynı
  parametre listede iki kez geçerse güncellemeler sırayla yapılır.
- **Sıralama ve birikim** (`cekirdek_sirala.go`): birikimli toplam satır başına bir blokla
  (parça toplamları paylaşımlı bellekte sabit sıralı taramayla); sıralama ve en büyük k 64 bitlik
  (sıra anahtarı, indeks) çiftleriyle bitonik ağla: dolgulu satır 4096'ya kadar paylaşımlı
  bellekte tek başlatma, daha uzunu genel bellekte adım başına bir başlatma. Anahtarlar tekil
  olduğundan sonuç kararlı ve CPU ile aynıdır.
- **Matris çarpımı:** makinede NVIDIA cuBLASLt kuruluysa float32 çarpım onunla yapılır (TF32
  kapalı; adımlı toplu; sürücü gibi çalışma anında yüklenir, TR dağıtmaz). Arama sırası
  `carpim_kitaplik_yol.go`'da: `TR_GPU_CUBLASLT` (tam yol), Linux'ta standart arama yolu,
  CUDA klasörleri, pip ile kurulmuş NVIDIA paketleri, TR'nin önbellek klasörü (ileride
  kitaplığı NVIDIA'nın yeniden dağıtılabilir paketinden oraya indirecek bir komut için);
  çalışma klasörü aranmaz. Açıkça verilen yol önce, öbür adaylar sürümlerine göre (sayısal,
  en yeni önce) denenir; kartın mimarisinin istediği en düşük sürümden eskiler atlanır
  (sm_100/120 için 12.8, sm_89/90 için 11.8) ve `cublasLtCreate` başarılı olan ilk aday
  kullanılır. Her şekil planına işaretçi hizaları bildirilir (ofsetli görünümler). Kitaplık
  bir şekli yapamazsa ya da çağrı hata dönerse panik yerine kendi çekirdeğimize düşülür
  (düşüşler sayılır). Planlar en çok 256'dır; program sonunda yok edilir. Bulunamazsa kendi
  çekirdeğimiz (`cekirdek_carpim.go`; döşemeli, vektörel paylaşımlı bellek okumalı)
  kullanılır; `TR_GPU_KENDI_CARPIM=1` onu zorlar. Yol, sürüm, atlanan adaylar ve düşüşler
  `gpu.Durum()`'da ve profilde görünür. 2b'de bf16/TF32 tensör çekirdekleri de bu arayüzden
  gelecek (veri türü parametresi hazır).
- **Profil:** `TR_GPU_PROFIL=1` (ya da `gpu.ProfilAc`) her başlatmayı iki zamanlama olayıyla
  ölçer: çekirdek/aile başına çağrı ve GPU süresi, adım başına duvar, GPU, boşluk, konak ve
  eşzamanlama süreleri; program sonunda (`motor.Kapat` → `gpu.Kapat`) özet standart hataya
  yazılır. Canlı izleme için Go arayüzü: `gpu.CekirdekOlcumleri`, `gpu.BellekOlcumleri`
  (canlı profil bunları okur; aşağıda).
  Dış profilleyici (nsys/ncu) bağlıysa program sonunda, bütün GPU'lar kapandıktan sonra bir
  kez C çıkış işleyicileri çalıştırılır (araçlar veriyi orada yazar; Go çıkışta bunları
  çalıştırmaz); ardından GPU kullanılamaz. Adım başına çöp toplama sayısı ve süresi de
  özettedir.
- **Bellek:** boyut sınıflı havuz (1–64 MB arası her ikinin kuvveti aralığı dörde bölünür);
  depolama otomatik türevin `Release`'iyle ya da çöp toplayıcının temizliğiyle havuza döner
  (tek akış sayesinde eşzamanlama gerekmez). Çöp toplayıcı cihaz belleğini görmediğinden son
  toplamadan bu yana verilen depolama eşiği aşınca (kart belleğinin 1/64'ü, 256 MB–2 GB;
  `TR_GPU_COP_ESIGI` MB ile değişir; gpt30m'de tepe 38,9 → ~22 GB, hız farkı %0,6), her
  geri yayılımın sonunda (`tensor.BackwardFinisher`; son toplamadan bu yana verilen bellek
  yeterince büyükse) ve son toplamadan bu yana sürücüden ayrılan bellek toplamın 1/32'sini
  aşınca toplama açıkça başlatılır; böylece ileri yönün ara tensörleri bir sonraki adımdan
  önce havuza döner. Havuz canlı/havuzda/ayrılmış/tepe sayaçlarını tutar. Konaktan yükleme
  sayfası kilitli bir halka üzerinden eşzamansızdır; küçük konak verisinin (indeks toplu
  işleri) konak gölgesi okumayı eşzamanlamasız yapar, başlatıcı yazılan depolamaların
  gölgesini siler.
- **Benzetim:** `benzetim*.go` PTX altkümesini yorumlayan bir sürücü benzetimidir; GPU
  sınamaları donanımsız makinede onda koşar (dağınık engel, sınır dışı ve hizasız erişim,
  paylaşımlı bellek yarışı yakalanır; eşzamansız işler gerçek sürücü gibi sıralanır).
  `TR_GPU_BENZETIM=1` ile `tr` de benzetimi `"gpu"` olarak kullanır (geliştirme aracı).
  Vektörel erişimler (`.v2/.v4`) ve `cp.async` de yürütülür: eşzamansız kopya ancak iş
  parçacığı beklediğinde yapılır ve yarış dedektörüne yazma olarak bildirilir (beklemeden
  ya da engelsiz okuma yanlış sonuç ya da yarış hatası verir).
  **Bilinen sınırlar** (benzetimde geçip donanımda kalabilecekler): bölgeler havuzun boyut
  sınıfı kadar ayrılır, istenen boyun dışına taşan ama sınıf içinde kalan erişim
  yakalanmaz; ayrıştırıcı ptxas kadar katı değildir (yazmaç türü, yuvarlama kipi
  yasallığı); bloklar sırayla yürüdüğünden genel bellekte iş parçacıkları arası yarış
  görünmez (çıktı eşlemesi birebir olmalıdır; `ScatterAdd`/indirgeme hedefi örtüşen bir
  görünüm olamaz); yaklaşık komutlar (`ex2`, `lg2`, `sin`, `cos`) tam doğrulukla, `fma`
  float64 üzerinden hesaplanır (donanımla birkaç ulp fark; sınamalar toleranslıdır).
  Sinüs, kosinüs ve tanjantın aralık indirgemesi float32'dedir: |x| ≤ 10⁵ radyanda doğrudur,
  daha büyük argümanda CPU'dan (float64 indirgeme) farklı sonuç verir.
- **Colab:** `tools/gpu/` (paket hazırlama, model ve mikro kıyas betikleri, `COLAB.md`). Mikro
  kıyaslar `gpu_sinama`'nın içindedir (`TR_GPU_MIKRO=1`, `TestMicroBench`); karşılaştırma
  tarafı `mikro_kiyas.py`, tablo `mikro_tablo.py`.

### Canlı profil (`internal/devtools/profiler`)

`tr izle` (canlı tablo) ve `tr profil` (son rapor) aynı çekirdeği kullanır; arka plan işi olarak
çalışan program (`TR_İŞ` iş klasörünü verir) da canlı durumunu buradan `profil.json`'a yazar.
Ağ portu açılmaz: dışarıya yalnızca dosya yazılır. Dört kaynaktan okur:

- **Tensör sayaçları** (`internal/compute/tensor/sayac*.go`): her zaman açık. Her işlem türü
  (`tensor.Islem`: ikili ve tekli işlemler `BinOp`/`UnOp` sırasıyla, matris çarpımı, indirgeme,
  `dikkat`, iyileştirici adımı ...), yön (ileri / geri yayılım) ve cihaz (CPU / GPU) için çağrı
  sayısı (kesin), ayrılan bayt (kesin; ayırma sınırı denetimi `ayirmaBoyuOn` ve gradyan
  ayırmaları sayar) ve süre. Süre CPU'da uyarlamalı örneklenir: her kayıt ölçülecek çağrıyı
  p = 1/32 olasılıklı bağımsız (geometrik aralıklı) seçimle belirler ve ölçülen süreyi 1/p ile
  ağırlıklar (Horvitz–Thompson; p yalnızca geçmiş ölçümlere bağlı olduğundan yansız); son
  ölçümü ~10 µs'yi aşan işlemde her çağrı ölçülür. GPU işlemlerinin her çağrısı ölçülür
  (başlatma mikrosaniyeler sürer; eşzamanlama beklemesi az sayıda ve uzundur, örneklenirse
  tahmini çok oynardı). Saat amd64'te değişmez zaman damgası sayacı (RDTSC), başka yerde
  monoton saat. Sıcak yol (`sayacBasla`, `bitir`, `sayacBayt`, `etkinTur`) satır içine alınır;
  çalışan işlem küresel bir numarada tutulur (`etkinNo`; gösterici değil: çöp toplayıcının
  yazma engelinden geçmez). Geri yayılım düğümü, ileri işlem çalışırken `track`'te o işlemin
  türüyle etiketlenir (`Tensor.islem`) ve geri hesabı o türün geri sayacıyla sayılır.
  Sayaçlar kilitsizdir (yarış denetimli derlemede atomik). Okuma: `tensor.SayacGoruntusu`,
  `tensor.EtkinIslem`.
- **Yorumlayıcı örneklemesi** (`ornekleyici.go`): yalnızca `tr izle` ve `tr profil` altında.
  Yorumlayıcının hata ayıklayıcı yuvasına (`VM.Debugger`) bağlanır; yuva boşken yorumlayıcının
  her deyimdeki maliyeti bir nil denetimidir (kapalıyken sıfır maliyet). Açıkken her deyim
  konumunu (fonksiyon numarası, satır) çağrı derinliğinin yuvasına yazar; ayrı bir goroutine
  1 ms'de bir o anki konuma geçen süreyi ve o an bir tensör işlemi çalışıyorsa tensör payını
  ekler. Derinlikten okumak, fonksiyon döndükten sonra çağıran satırın kalan işini doğru
  satıra yazar. `kesme_noktası` profil altında yok sayılır.
- **Adım sınırı** (`gunluk.AdimKancasiAyarla`): `İlerleme.güncelle` her çağrıda adım sayısını
  ve zamanı bildirir; ms/adım ve adım başına işlem dağılımı buradan.
- **GPU arka ucunun profili** (`gpu.CekirdekOlcumleri`, `gpu.BellekOlcumleri`): `tr izle` ve
  `tr profil` çekirdek ölçümünü açar (`gpu.ProfilAc`; `TR_GPU_PROFIL=1` ile zaten açıksa
  dokunmaz), program bitince motorlar kapanmadan önce (`Profil.KapanmadanOnce`, komut
  satırının `kapanmadanOnce` işleri) cihazı eşzamanlayıp arka ucun raporunu (`gpu.ProfilOzetiYaz`)
  alır ve ölçümü eski durumuna döndürür; arka uç kapanırken raporu ikinci kez yazmaz. Arka
  plan işinin profili ölçümü açmaz; GPU belleği yine okunur. Okumalar arka ucu beklemez.

Profil saniyede bir anlık görüntü alır (`Goruntu`), son 5 saniyenin farklarından pencere
ölçülerini kurar (canlı tablonun hızları), dinleyiciye (canlı tablo) verir ve iş klasörü varsa
dosyaya yazar. Program bitince (hata, `çıkış` ya da Ctrl+C ile bitse de; komut satırının
çıkış temizliği) son raporu kurar. Sayaçlar süreç genelidir; profil başlangıcındaki değerler
çıkarılır.

**Canlı tablo** (`ekran.go`, `tablo.go`): standart hata bir terminalse tablo ekranın başından
denetim dizileriyle yeniden çizilir (Windows'ta konsolun sanal terminal kipi açılır). Programın
standart çıkışı ve hatası bir boruya alınır (`yakalama.go`): tamamı bir dosyaya yazılır, son
satırları tablonun altında görünür, program bitince yeniden yazılır; boru terminal
olmadığından ilerleme çubuğu düz satır kipine geçer. Terminal değilse çıktı olduğu gibi akar,
10 saniyede bir tek satırlık özet eklenir.

**Dosya biçimi** (`goruntu.go`): `profil.json` (canlı, iş klasöründe) ve `speed.json` /
`tr profil --json` (son rapor) aynı yapıdadır. Anahtarlar dosya biçiminin parçasıdır
(yazım dilinden bağımsız, çevrilmez); işlemlerin `kod` alanı değişmez anahtardır, `ad`
insana gösterilen addır (dildeki ad; çevrilebilir). Süreler saniye, baytlar bayt, paylar
0..1'dir. Yazma atomiktir (aynı klasörde geçici dosya + yeniden adlandırma); yeniden
adlandırmayı desteklemeyen sürücüde bir kez uyarılıp doğrudan yazmaya geçilir, yazma
başarısızsa sonraki saniyede yeniden denenir; program hiçbir durumda durmaz. Sürüm 1:

```jsonc
{
  "biçim": "tr-profil", "sürüm": 1, "tür": "canlı",        // "canlı" ya da "rapor"
  "program": "egit.tr", "başlangıç": "2026-10-19T08:49:12.511+03:00",
  "zaman": "2026-10-19T08:49:24.511+03:00", "süre_sn": 12.0, "bitti": false,
  "çıkış_kodu": 0,                                          // yalnızca bitince
  "adım": {"sayı": 715, "toplam": 1500, "tüm_adım": 715,   // ilerleme yoksa null; sayı ve toplam
    "ms_adım": 16.7, "adım_sn": 59.7},   // son çubuğun, tüm_adım bütün çubukların (epoch'lar) adımı
  "tensör": {"süre_sn": 11.4, "pay": 0.95, "çağrı": 772761, "bayt": 7642211100,
    "işlemler": [{"kod": "matris_carpimi", "ad": "matris çarpımı (@)", "yön": "ileri",
      "cihaz": "cpu", "çağrı": 160966, "ölçüm": 160529, "süre_sn": 4.99, "pay": 0.44,
      "bayt": 2044092928, "ms_adım": 6.99}]},              // süreye göre sıralı, en çok 30
  "gpu": {"kullanıldı": true, "ölçüm": true, "not": "", "süre_sn": 0.088,
    "aileler": [{"ad": "carpim", "çağrı": 40, "süre_sn": 0.069, "pay": 0.78}],
    "çekirdekler": [{"ad": "carpim_64_64_16_4_4_false_false_true", "çağrı": 20, "süre_sn": 0.040, "pay": 0.46}],
    "bellek": [{"cihaz": 0, "canlı_bayt": 109568, "ayrılmış_bayt": 114176, "havuzda_bayt": 4608,
      "tepe_canlı_bayt": 113664, "tepe_ayrılmış_bayt": 114176, "çöp_toplama": 0}]},
  "örnekleme": {"aralık_ms": 1, "örnek": 11800, "süre_sn": 11.8, "tensör_payı": 0.95,  // kapalıysa null
    "satırlar": [{"dosya": "examples/11_gpt.tr", "satır": 49, "fonksiyon": "",
      "metin": "geri_yayılım(kayıp)", "süre_sn": 4.6, "pay": 0.39, "tensör_payı": 0.99}],
    "fonksiyonlar": [{"ad": "dikleştir", "süre_sn": 3.9, "pay": 0.33}]},
  "bellek": {"go_yığını_bayt": 7864320, "go_sistem_bayt": 44564480, "cpu_tensör_ayrılan_bayt": 7642211100},
  "çöp_toplama": {"sayı": 423, "duraklama_sn": 0.113},
  "son_pencere": {"süre_sn": 5.0, "adım": 292, "ms_adım": 17.1, "adım_sn": 58.3,   // yalnızca canlıda
    "tensör_payı": 0.96, "işlemler": [], "satırlar": [], "çöp_toplama_sayı": 165,
    "duraklama_sn": 0.055, "cpu_tensör_ayrılan_bayt": 2980000000,
    "gpu_süre_sn": 0, "gpu_payı": 0, "gpu_aileler": []}
}
```

Satırın `dosya`sı ana programın dışındaysa (aktarılan modül) boştur: örnek yalnızca fonksiyon
adını ve satırı bilir. `ölçüm` süresi ölçülen çağrı sayısıdır (`çağrı`dan küçükse süre
örneklenmiştir). Canlı CPU tensör baytı ayrıca izlenmez (CPU tensörleri Go yığınındadır);
`cpu_tensör_ayrılan_bayt` ayırma akışıdır.

**Maliyet** (ölçüm yöntemi: önce/sonra test ikilileri dönüşümlü, P çekirdeklere sabit; makine
yüklüyken ölçüldü, boş makinede yinelenmeli): sayaçların işlem başına yükü ~4 ns
(`BenchmarkCounterStartStop`); 16 elemanlı tensörlerle `BenchmarkCounterSmall*` (işlem başına
~350 ns) %1–3; 64 boyutlu küçük MLP eğitim adımında (~75 µs) gürültü içinde (ortanca +%2,
en iyi −%1, A/A gürültüsü ±%2). Örnekleme açıkken çağrı yoğun TR kodu ~2 kat yavaşlar
(hata ayıklayıcı yuvası doluyken yorumlayıcı fonksiyon ortamlarını havuzlamaz).

## Klasörler

| Klasör | İçerik |
| :-- | :-- |
| `cmd/tr/` | Tek ikili |
| `internal/` | Bütün Go kodu (yukarıdaki katmanlar) |
| `lib/` | TR ile yazılı standart kitaplık, ikiliye gömülü (`aktar "tr/katman"`, `aktar "tr/iyilestirici"`; testleri `tr test lib`) |
| `editors/vscode/` | Düzenleyici eklentisi (kullanıcıya gider) |
| `docs/` | Belgeler: [`kilavuz/`](guide/README.md), [`dil/`](spec/README.md), [`kodlar.md`](kodlar.md) (üretilir), [`yol_haritasi.md`](yol_haritasi.md), [`katki.md`](katki.md) (kod düzeni), bu dosya |
| `examples/` | Öğrenme sırasıyla örnek programlar |
| `tests/` | Dil tanımının kural testleri (`tr test tests`); `olumsuz/`, `alt/`, `veri/` |
| `tools/` | Depo bakım araçları, kullanıcıya gitmez: `tam_denetim.sh` ("bitti" tanımı; otomatik sınama da bunu çalıştırır), `bulaniklastir.sh` (bulanıklaştırma testlerinin uzun koşusu), `uret/` (üreteçler), `kiyas/` (hız kıyas seti), `gpu/` (GPU sınama ve kıyas: `COLAB.md` Colab'da kodu GitHub'dan çekip derler; `kiyas_gpu.tr`, `gpt_egitim.tr`, `kiyas_gpu.py`, `mikro_kiyas.py`, `mikro_tablo.py`) |
| `bin/` | Derleme çıktısı (`go build -o bin/tr.exe ./cmd/tr`); depoya girmez |
| `scratch/` | Kişisel deneme klasörü; depoya girmez |

## Sahiplik

| Rol | Alan |
| :-- | :-- |
| Dil çekirdeği | `internal/lang/**`, `internal/stdlib/files`, `internal/devtools/{bicim,testkosucu,ayikla}`, `internal/reporoot`, `cmd/tr`, `docs/spec/`, bu dosya |
| Tensör ve GPU | `internal/compute/tensor`, `internal/compute/gpu`, `internal/stdlib/ml`, ileride `lib/cekirdek` |
| Veri ve parçalayıcı | `internal/compute/{tokenize,dataload}`, `internal/stdlib/{tokenizer,dataset,formats}` |
| Araçlar | `editors/`, `.vscode/`, `docs/guide/`, `README.md` |
| Hız | `tools/bench/` |
| Test | `tests/`, `.github/`, eklenti testleri |
