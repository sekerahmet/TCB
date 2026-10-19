# Mimari: katmanlar ve paketler

TR tek bir ikilidir (`tr`). Bütün Go kodu `internal/` altındadır; başka bir Go modülü bu paketleri içe aktaramaz. Kullanıcı Go paketi değil `.tr` dosyası yazar.

## Katmanlar

```
araçlar (internal/arac, cmd/tr) ─▶ motorlar (internal/motor) ─▶ dil çekirdeği (internal/dil)
                                        │
                                        └──────▶ hesap çekirdekleri (internal/hesap)
TR ile yazılı kitaplık (kitaplik/*.tr) ── ikiliye gömülü; cmd/tr yorumlayıcıya bağlar (aktar "tr/...")
```

Ok "içe aktarabilir" demektir. Kural `internal/katman_test.go` ile denetlenir (`go test ./internal`); yalnızca paketlerin kendi (test dışı) içe aktarmalarına uygulanır.

| Katman | İçe aktarabileceği iç paketler |
| :-- | :-- |
| `internal/hesap/**` | yalnızca `internal/hesap/**` (dili bilmez; tek başına sınanır ve ölçülür) |
| `internal/dil/**` | yalnızca `internal/dil/**`, iç sırasına göre (aşağıda) |
| `internal/motor/**` | `internal/dil`, `internal/hesap`, `internal/motor` |
| `internal/arac/**` | `internal/dil`, `internal/hesap`, `internal/motor`, `internal/arac` |
| `cmd/tr` | hepsi (`internal/depo` dışında) |
| `internal/depo`, `kitaplik` | hiçbiri |
| `tools/**` | hepsi |

Dil çekirdeğinin içinde de sıra vardır: bir dil paketi yalnızca kendisinden önce gelenleri içe aktarabilir (`internal/katman_test.go`, `dilSirasi`):

```
turkce < kod < ipucu < sozcuk < agac < ayristir < denetci < kaynak < yurut
```

Yorumlayıcı (`internal/dil/yurut`) motorları ve hata ayıklayıcıyı içe aktarmaz; onlar yorumlayıcıya kendileri bağlanır: `internal/motor` paketi yüklenirken yerleşikleri `yurut.MotorEkle` ile, `internal/arac/ayikla` paketi yüklenirken koddaki `kesme_noktası` deyiminin açacağı hata ayıklayıcıyı `yurut.YeniHataAyiklayici` ile kaydeder. Tam dili çalıştıran program ikisini de içe aktarır (`cmd/tr`, `internal/arac/testkosucu`).

## Paketler

| Paket | Görevi |
| :-- | :-- |
| `cmd/tr` | `tr` komutu: alt komutları ayrıştırır (çalıştır, kabuk, `kontrol`, `derle`, `test`, `biçimle`, `kodla`, `çöz`, `göster`, `hata_ayıkla`, `dap`), işi `internal/`'a bırakır |
| `internal/dil/kod` | Dilin bütün adlarını sabit kodlara bağlayan kod listesi (`tanim.json`, `tr.json`) ve ondan üretilen Go sabitleri |
| `internal/dil/turkce` | Türkçe harf işlemleri, ekler, "bunu mu demek istediniz" önerileri ve hata çerçevesi |
| `internal/dil/ipucu` | Dilin adlarıyla ilgili hata ipuçları: kaldırılan, taşınan ve Türkçe karaktersiz yazılan adlar, tanımsız ada en yakın ad (sözcük çözümleyici, ayrıştırıcı, denetleyici ve yorumlayıcı aynı metni kullanır) |
| `internal/dil/sozcuk` | Sözcük çözümleyici: kaynak metni sözcük birimlerine ayırır; kodlu gövdede kodları etkin dilin adlarına çevirir, konumları Türkçe gösterime göre verir |
| `internal/dil/agac` | Sözdizimi ağacının düğümleri |
| `internal/dil/ayristir` | Ayrıştırıcı: sözcük birimlerinden sözdizimi ağacını kurar |
| `internal/dil/denetci` | Çalıştırmadan önceki tip denetimi ve uyarılar; adların kapsam bilgisi (tanımsız ad denetimi ve kodlu saklamanın ad bağları) |
| `internal/dil/kaynak` | Kaynaktan programa tek giriş noktası: dosya metnini bölümlerine ayırır (BOM, kodlu biçim başlığı), doğru sözcük çözümleyiciyi seçer, ayrıştırır, isteğe göre denetler (`Hazirla`); komut satırı, kabuk, test koşucusu, hata ayıklama sunucusu, modül yükleyici ve `tr derle` dosyaları yalnızca buradan okur. Kodlu saklamanın çeviricisi (`Kodla`, `Coz`) de buradadır |
| `internal/dil/yurut` | Derleyici ve yorumlayıcı: değerler, çekirdek yerleşikler ve metotlar, modüller, eşzamanlı görevler, motor ve hata ayıklayıcı kancaları |
| `internal/motor` | Bütün motorları doğru sırayla yorumlayıcıya kaydeder (`import _ "tr_dili/internal/motor"`) |
| `internal/motor/ogrenme` | Tensör, matris, otomatik türev, sinir ağı katmanları, iyileştiriciler ve tensör kaydını TR yerleşikleri olarak dile bağlar |
| `internal/motor/parcalayici` | `parçalayıcı_yükle` ve parçalayıcı nesnesinin üyeleri |
| `internal/motor/veri` | `veri_kümesi`, örnekleyici ve sıralı okuyucu yerleşikleri |
| `internal/motor/veribicim` | JSON ve CSV yerleşikleri |
| `internal/motor/dosya` | Standart kitaplık: `Yol` tipi, `yol`, `klasörü_listele`, `klasör_oluştur`, `sil`, `kopyala`, `taşı`, `dosya_satırları` (silme korumaları, bağlantı izlememe, `os.Root`) |
| `internal/motor/surec` | Süreç yerleşikleri: `ortam_değişkeni`, `komut_satırı` (seçenekler, `--yardım`, kullanım hatası 2), gizli adların süzgece kaydı |
| `internal/motor/gunluk` | Eğitim günlüğü (`günlük`, JSON satırları) ve ilerleme çubuğu (`ilerleme`; canlı satır, `ilerleme.json`), adım sınırı kancası |
| `internal/motor/kontrol` | Kontrol noktası (`kontrol_noktası`): eğitimin bütün durumunun koşu klasörüne atomik çok dosyalı kaydı, doğrulanarak yüklenmesi ve kaldığı yerden devam; ertelenen kesme |
| `internal/motor/ortak/baglama` | Motorların ortak yardımcıları: adlı argüman denetimi, göreli yol, panikten hataya, Türkçe sayı yazımı |
| `internal/motor/ortak/jsonyaz` | TR değerlerini JSON'a yazar |
| `internal/hesap/tensor` | n boyutlu tensör, görünümler, otomatik türev, arka uç arayüzü ve CPU arka ucu (AVX2/FMA çekirdekleri) |
| `internal/hesap/gpu` | GPU arka ucu: sürücü bağlama (cgo'suz), Go içinden üretilen PTX çekirdekleri, cihaz belleği havuzu, PTX benzetimi (donanımsız sınama) |
| `internal/hesap/parca` | `tokenizer.json` parçalayıcıları: BPE, WordPiece, Unigram, normalleştirme, ön bölme |
| `internal/hesap/veriyolu` | `.npy` veri dosyaları, veri kümesi, örnekleyici, önbellek ve önden yükleme |
| `internal/arac/ayikla` | Terminal hata ayıklayıcısı ve düzenleyicinin hata ayıklama sunucusu (`tr dap`) |
| `internal/arac/bicim` | `tr biçimle` biçimlendiricisi |
| `internal/arac/testkosucu` | `tr test` komutunun çekirdeği |
| `internal/depo` | Testlerin depo kökünü bulan yardımcı (testler `../../` yazmaz) |
| `kitaplik` | TR ile yazılan standart kitaplık (`katman.tr`, `iyilestirici.tr`, testleri yanında); `gomulu.go` onları `go:embed` ile ikiliye gömer, `cmd/tr` yorumlayıcıya `yurut.KitaplikBagla` ile bağlar. Kullanıcı `aktar "tr/katman"` yazar (dil tanımı 11.6); `tr derle` çıktısı yorumlayıcının kopyası olduğundan kitaplık ona da girer. Ayrıntı: [`kitaplik/README.md`](../kitaplik/README.md) |
| `tools/uret/*` | Üreteçler: Go kod sabitleri, `docs/kodlar.md`, eklentinin renklendirme listeleri (`go generate ./internal/dil/kod`) |
| `tools/kiyas/go` | Hız kıyas setinin derlenmiş başvuru sürümü |

### Yorumlayıcı kancaları

Motorlar ve standart kitaplık yorumlayıcıya (`internal/dil/yurut`) şu kancalarla bağlanır; yorumlayıcı onları bilmez:

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

### GPU arka ucu (`internal/hesap/gpu`)

`tensor.Backend`'in NVIDIA uygulamasıdır; otomatik türev ve üst katmanlar cihazdan bağımsızdır.

- **Kayıt:** paket init'te tembel bir arka uç kaydeder (`tensor.RegisterLazyBackend`); sürücü
  ancak bir cihaz sorulunca (`cihazlar()`, `.cihaza`, `varsayılan_cihaz`) yüklenir. Sürücü
  yoksa hiçbir şey kaydolmaz, her şey CPU'da çalışır.
- **Sürücü:** cgo ve CUDA araç takımı gerekmez. Linux'ta `libcuda.so.1` dış bağımlılık olan
  dinamik yükleyiciyle (`go.mod`; yalnızca `cuda_linux.go` içe aktarır, `internal/katman_test.go`
  denetler), Windows'ta `nvcuda.dll` yalnızca sistem klasöründen (LoadLibraryExW) yüklenir.
  **Bedel:** bu yükleyici Linux ikilisini glibc'ye dinamik bağlar (`/lib64/ld-linux-x86-64.so.2`;
  `libc.so.6`, `libdl.so.2`, `libpthread.so.0`), GPU kullanmayan Linux kullanıcıları için de;
  musl tabanlı dağıtımlarda (Alpine) ikili açılmaz. `-tags gpusuz` ile derlenen ikili statik
  kalır ve GPU desteği içermez. **Karar:** varsayılan ikili GPU desteklidir (glibc'ye bağlı);
  statik ikili isteyen `GOOS=linux CGO_ENABLED=0 go build -tags gpusuz ./cmd/tr` ile derler.
  Tam denetim `gpusuz` derlemenin statik kaldığını `tools/statik_denetim` ile doğrular.
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
  yön; `cekirdek_flash*.go`), çok tensörlü iyileştirici/ölçekleme/kareler toplamı (bütün
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
  yazılır. Canlı izleme için Go arayüzü: `gpu.CekirdekOlcumleri`, `gpu.BellekOlcumleri`.
  Dış profilleyici (nsys/ncu) bağlıysa program sonunda, bütün GPU'lar kapandıktan sonra bir
  kez C çıkış işleyicileri çalıştırılır (araçlar veriyi orada yazar; Go çıkışta bunları
  çalıştırmaz); ardından GPU kullanılamaz. Adım başına çöp toplama sayısı ve süresi de
  özettedir.
- **Bellek:** boyut sınıflı havuz (1–64 MB arası her ikinin kuvveti aralığı dörde bölünür);
  depolama otomatik türevin `Release`'iyle ya da çöp toplayıcının temizliğiyle havuza döner
  (tek akış sayesinde eşzamanlama gerekmez). Çöp toplayıcı cihaz belleğini görmediğinden her
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
  kıyaslar `gpu_sinama`'nın içindedir (`TR_GPU_MIKRO=1`, `TestMikroKiyas`); karşılaştırma
  tarafı `mikro_kiyas.py`, tablo `mikro_tablo.py`.

## Klasörler

| Klasör | İçerik |
| :-- | :-- |
| `cmd/tr/` | Tek ikili |
| `internal/` | Bütün Go kodu (yukarıdaki katmanlar) |
| `kitaplik/` | TR ile yazılı standart kitaplık, ikiliye gömülü (`aktar "tr/katman"`, `aktar "tr/iyilestirici"`; testleri `tr test kitaplik`) |
| `editors/vscode/` | Düzenleyici eklentisi (kullanıcıya gider) |
| `docs/` | Belgeler: [`kilavuz/`](kilavuz/README.md), [`dil/`](dil/README.md), [`kodlar.md`](kodlar.md) (üretilir), [`yol_haritasi.md`](yol_haritasi.md), [`katki.md`](katki.md) (kod düzeni), bu dosya |
| `ornekler/` | Öğrenme sırasıyla örnek programlar |
| `testler/` | Dil tanımının kural testleri (`tr test testler`); `olumsuz/`, `alt/`, `veri/` |
| `tools/` | Depo bakım araçları, kullanıcıya gitmez: `tam_denetim.sh` ("bitti" tanımı; otomatik sınama da bunu çalıştırır), `bulaniklastir.sh` (bulanıklaştırma testlerinin uzun koşusu), `uret/` (üreteçler), `kiyas/` (hız kıyas seti), `gpu/` (GPU sınama ve kıyas paketi: `colab_hazirla.sh`, `kiyas_gpu.tr`, `kiyas_gpu.py`, `mikro_kiyas.py`, `mikro_tablo.py`, `COLAB.md`) |
| `bin/` | Derleme çıktısı (`go build -o bin/tr.exe ./cmd/tr`); depoya girmez |
| `calisma/` | Kişisel deneme klasörü; depoya girmez |

## Sahiplik

| Rol | Alan |
| :-- | :-- |
| Dil çekirdeği | `internal/dil/**`, `internal/motor/dosya`, `internal/arac/{bicim,testkosucu,ayikla}`, `internal/depo`, `cmd/tr`, `docs/dil/`, bu dosya |
| Tensör ve GPU | `internal/hesap/tensor`, `internal/hesap/gpu`, `internal/motor/ogrenme`, ileride `kitaplik/cekirdek` |
| Veri ve parçalayıcı | `internal/hesap/{parca,veriyolu}`, `internal/motor/{parcalayici,veri,veribicim}` |
| Araçlar | `editors/`, `.vscode/`, `docs/kilavuz/`, `README.md` |
| Hız | `tools/kiyas/` |
| Test | `testler/`, `.github/`, eklenti testleri |
