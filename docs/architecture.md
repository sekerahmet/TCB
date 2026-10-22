# Mimari: katmanlar ve paketler

TR tek bir ikilidir (`tr`). Bütün Go kodu `internal/` altındadır; başka bir Go modülü bu paketleri içe aktaramaz. Kullanıcı Go paketi değil `.tr` dosyası yazar.

Bu belge **nasıl çalıştığını ve neden böyle kurulduğunu** anlatır. Kod yazarken uyulacak kurallar [`internal/README.md`](../internal/README.md)'de, ekleme adımları [`recipes.md`](recipes.md)'de, süreç (yeni ad isteği, tam denetim, ölçüm) [`development.md`](development.md)'dedir; burada tekrarlanmaz. Bir paketin kendi standartları o paketin `README.md`'sindedir.

## Katmanlar

```
araçlar (internal/devtools, cmd/tr) ─▶ motorlar (internal/stdlib) ─▶ dil çekirdeği (internal/lang)
                                        │
                                        └──────▶ hesap çekirdekleri (internal/compute)
TR ile yazılı kitaplık (lib/*.tr) ── ikiliye gömülü; cmd/tr yorumlayıcıya bağlar (aktar "tr/...")
```

Ok "içe aktarabilir" demektir. Kural `internal/layers_test.go` ile denetlenir (`go test ./internal`); yalnızca paketlerin kendi (test dışı) içe aktarmalarına uygulanır, testler sınamak için üst katmanları da kullanabilir.

| Katman | Görevi | İçe aktarabileceği iç paketler |
| :-- | :-- | :-- |
| `internal/lang/**` | Kaynak metinden çalışan programa; kod listesi, iletiler | yalnızca `internal/lang/**`, iç sırasına göre (aşağıda) |
| `internal/compute/**` | Tensör, otomatik türev, CPU ve GPU, veri hattı, parçalayıcı | `internal/compute/**` ve ileti kataloğu `internal/lang/messages` |
| `internal/stdlib/**` | Dili hesap çekirdeğine ve dış dünyaya bağlayan yerleşikler | `internal/lang`, `internal/compute`, `internal/stdlib` |
| `internal/devtools/**` | Hata ayıklayıcı, profilci, biçimlendirici, test koşucusu | `internal/lang`, `internal/compute`, `internal/stdlib`, `internal/devtools` |
| `cmd/tr` | `tr` komutu | hepsi (`internal/reporoot` dışında) |
| `internal/reporoot`, `lib` | Testlerin depo kökü yardımcısı; kitaplığın gömme dosyası | hiçbiri |
| `tools/**` | Depo bakım araçları | `internal/**`, `lib` |

**Neden böyle?** İki çekirdek birbirini bilmez: hesap çekirdeği dili bilmediği için tek başına sınanır ve ölçülür (tensör ya da GPU değişikliği dil testlerini beklemez); dil çekirdeği tensörü bilmediği için yeni bir motor dilin içine dokunmadan eklenir. İkisini `internal/stdlib` birleştirir. Tek ortak paket ileti kataloğudur (`internal/lang/messages`): yalnızca Go standart kitaplığına dayanır, hesap çekirdeğinin iletileri de onun kodlarıdır. Araçlar en üsttedir, çünkü hepsinin hepsini görmesi gerekir; ama alttakiler araçları bilmez, böylece hata ayıklayıcı ya da profilci çıkarılsa ikili derlenir.

Dış bağımlılık izne bağlıdır: bugün yalnızca `purego` (GPU sürücüsünü cgo'suz yüklemek için, `internal/compute/gpu`). Denetim: `TestExternalDependencies`.

### Dil çekirdeğinin iç sırası

Dil çekirdeğinde de bir sıra vardır; bir paket yalnızca kendisinden öncekileri içe aktarır (`internal/layers_test.go`, `langOrder`; denetim `TestLangOrder`):

```
messages → turkish → codes → hint → lexer → ast → parser → checker → source → interp
```

| Paket | Görevi |
| :-- | :-- |
| `messages` | İleti kodları (`H0000`...) ve etkin dildeki metinleri; en alttır, yalnızca standart kitaplığa dayanır |
| `turkish` | Türkçe harf kuralları, ünlü uyumuna göre ekler, en yakın ad önerisi, dosya hatası metinleri, kod çerçevesi |
| `codes` | Dilin kullanıcıya görünen bütün adlarının sabit kodları (`G100`...); dil dosyaları ve tanım dosyası |
| `hint` | ASCII yazılan ve başka dillerden alışılmış adların, tanımsız ada en yakın adın ipuçları (tek kaynak) |
| `lexer` | Sözcük çözümleyici; Türkçe ve kodlu kaynağı okur, konumları Türkçe gösterime göre verir |
| `ast` | Sözdizimi ağacı düğümleri; yalnızca veri taşır, konumdan bağımsız döküm yazar |
| `parser` | Öncelik tabanlı (Pratt) ifade ve özyinelemeli iniş deyim ayrıştırıcısı |
| `checker` | Çalıştırmadan önce tip denetimi, uyarılar, tanımsız ad denetimi |
| `source` | Kaynaktan programa tek giriş noktası, kodlu biçimin çeviricisi |
| `interp` | Derleyici ve yorumlayıcı; değerler, çekirdek yerleşikler, modüller, eşzamanlılık, kancalar |

Sıra bir bağımlılık zinciri olduğu kadar bir döngü korumasıdır: sözcük çözümleyici yorumlayıcıyı, denetleyici yorumlayıcıyı bilmez. Yeni bir dil paketi listeye yerine göre eklenir.

## Bir programın kaynaktan çalışmaya yolu

`tr egit.tr` yazıldığında olan şudur:

1. **Dosya okunur ve bölünür.** `source.Split` BOM'u ve kodlu biçimin başlığını ayırır. Komut satırı, kabuk, test koşucusu, hata ayıklama sunucusu ve modül yükleyici bir `.tr` dosyasını yalnızca `internal/lang/source` üzerinden okur; başlık kuralları tek yerdedir.
2. **Sözcüklere ayrılır.** `lexer.New` kaynağı sözcük birimlerine çevirir. Anahtar kelimeler elle yazılmaz, etkin dilin kod listesinden (`codes`) gelir. Kodlu bir dosyada (`G100` gibi) kodlar etkin dilin adlarına çevrilir; konumlar hep Türkçe gösterime göredir, böylece hata iletisi, hata ayıklayıcı ve düzenleyici aynı satırı gösterir.
3. **Ayrıştırılır.** `parser.New` sözdizimi ağacını (`ast`) kurar. Bağlam kuralları (`çık` yalnızca döngüde ...) burada denetlenir; hatalar tek mesaja toplanıp toparlanır.
4. **Denetlenir.** `source.Prepare` istenirse `checker` ile tip ve ad denetimi yapar. Denetleyici hiçbir şey çalıştırmaz; emin olamadığında susar.
5. **Derlenir.** `interp` ağacı bir kez Go kapanışlarına derler (`compiler*.go`): değişkenler yuvalara, üye adları kodlara çözülür; çalışma anında ad karşılaştırılmaz.
6. **Çalışır.** `VM.Run` programı yürütür. Eşzamanlı görevler tek yorumlayıcı kilidi altında çalışır; uzun işler (disk, ağ, uyku, ağır hesap) `VM.Blocking` içinde kilidi bırakır.
7. **Kapanır.** Program bitince (hata, `çıkış` ya da Ctrl+C ile de) motorların açık kaynakları kapatılır (`stdlib.Close`).

Hatalar aşama aşama aynı yolu izler: ileti bir kodla (`messages.New`) kurulur, satırı `interp.LineError` (`messages.PosError`) ekler, kullanıcıya etkin dilin metniyle yazılır. Kesme isteği (Ctrl+C) hata değildir; `dene/yakala` onu yakalamaz.

## Kod listesi ve ileti kataloğu

Kullanıcıya görünen her ad ve her ileti sabit bir koda bağlıdır; Go kodu kodu yazar, metin dil dosyasından gelir. Bu, bir adı ya da iletiyi değiştirmeyi (ya da başka bir dile çevirmeyi) Go koduna dokunmadan, yalnızca bir dosya satırını değiştirerek yapmayı sağlar.

| | Kod listesi (`internal/lang/codes`) | İleti kataloğu (`internal/lang/messages`) |
| :-- | :-- | :-- |
| Kod | Bir harf ve üç rakam (`A001`, `G100`, `Y400`); harf aileyi gösterir | `H` ve dört basamak (`H7003`); binler basamağı alanı gösterir |
| Dilden bağımsız | `definitions.json`: tür, imza, dönüş tipi, örnek | `definitions.json`: her kodun parametre adları |
| Dile bağlı | `tr.json` (kaynak dil), `en.json` | `tr.json` (kaynak dil), `en.json` (her kodu taşır; eksik çeviri test hatasıdır) |
| Go'dan kullanım | `codes.Name(codes.G100)`, `codes.Find`, `codes.Active` | `messages.New(messages.H7003, ...)`, `messages.Text`, `messages.Has`, `messages.Internal` |
| Dil | Okuma ve çalışma dili her zaman Türkçe (`codes.Active`); kişinin yazım dili `TR_DIL` (`codes.Display`, `codes.DisplayName`) yalnızca gösterimdir (17.7) | iletilerin dili `TR_DIL` (`messages.LanguageEnvVar`) |

Yazım kuralları (kod değişmez, ad değişebilir; her işin tek adı; parametreler adlıdır; iç hata `H9000`) paketlerin README'lerindedir: [`codes`](../internal/lang/codes/README.md), [`messages`](../internal/lang/messages/README.md). Yeni ad için [`development.md`](development.md#yeni-kod-ad-isteği), yeni ileti için [`recipes.md`](recipes.md).

**Üretilmiş dosyalar.** Kod listesinden ve ileti kataloğundan üretilen dosyalar elle düzenlenmez; üreteçler `tools/gen/` altındadır ve `-check` bayrağıyla güncelliği sınanır (tam denetim hepsini koşar). Üretmek için: `go generate ./internal/lang/codes ./internal/lang/messages`.

| Üreteç | Ürettiği |
| :-- | :-- |
| `tools/gen/codeconsts` | `internal/lang/codes/codes_generated.go` (kod sabitleri); Go metnindeki çıplak ad denetimi |
| `tools/gen/codesdoc` | [`docs/codes.md`](codes.md) (kod listesi belgesi) |
| `tools/gen/grammar` | VS Code eklentisinin renklendirme listeleri (`editors/vscode/syntaxes/tr.tmLanguage.json`) |
| `tools/gen/msgconsts` | `internal/lang/messages/codes_generated.go` (ileti sabitleri); ileti kullanım denetimi |

## Kancalar: dil çekirdeği motorları nasıl bilir

Yorumlayıcı (`internal/lang/interp`) motorları ve araçları içe aktarmaz; onlar yorumlayıcıya kancalarla bağlanır. Böylece dil çekirdeği tek başına derlenir ve sınanır, her yetenek yorumlayıcıda bir arayüz olarak tanımlanır, uygulaması motorda yazılır.

**Motor kaydı.** `internal/stdlib` paketi yüklenirken (`init`) her motoru `interp.AddEngine` ile ekler; her yeni `VM` çekirdek yerleşiklerden sonra motorları eklenme sırasıyla kaydeder. Sıra önemlidir (`internal/stdlib/engines.go`):

1. `formats`, `files`
2. `ml`: çekirdek yerleşiklerin (toplam, ortalama, min, maks, karekök) tensör sürümleriyle sarıldığı için onlardan sonra
3. `tokenizer`, `dataset`: tensör değerleri üretir
4. `process`, `logging`, `checkpoint`: tek kayıtta, çünkü günlüğün başlangıç kaydı ve kontrol noktası programın komut satırı ayarlarını (`process.State`) kullanır

Tam dili çalıştıran her program (`cmd/tr`, `internal/devtools/testrunner`) bu paketi `import _` ile içe aktarır. Program bitince `stdlib.Close` motorların kaynaklarını kapatır.

**Hata ayıklayıcı.** Koddaki `kesme_noktası` deyimi `interp.NewDebugger` değişkeniyle bir `interp.Debugger` açar; bu değişkeni `internal/devtools/debugger` paketi yüklenirken (`wire.go`) kurar. Değişken boşsa deyim yok sayılır. Profilci de aynı yuvaya (`VM.Debugger`) bağlanır.

**Kitaplık.** `lib` paketi `*.tr` dosyalarını `go:embed` ile ikiliye gömer; `cmd/tr` bunları `interp.SetLibrary` ile bağlar. `aktar "tr/katman"` yorumlayıcının kitaplık okuyucusundan (`interp.ReadLibrary`) gelir; `tr/` öneki dilin kendisine ayrılmıştır. `tr derle` çıktısı yorumlayıcının kopyası olduğundan kitaplık ona da girer. Ayrıntı: [`lib/README.md`](../lib/README.md).

### Yorumlayıcının arayüzleri

Bir motor değerinin dile uyması, `interp` içindeki arayüzlerden birini uygulamasıyla olur (tip denetimiyle bulunur; her uygulayıcıda derleme anı denetimi kuralı için bkz. [`internal/README.md`](../internal/README.md#kancalar)):

| Arayüz (`interp`) | Ne sağlar |
| :-- | :-- |
| `Equaler`, `OperatorHook`, `TaskOperatorHook`, `Negatable` | Eşitlik, ikili ve tekli işleçler (tensör `+ - * / @ **`) |
| `Indexable`, `IndexAssignable`, `MemberAssignable`, `TaskMemberHook` | `x[i]`, `x[i] = v`, `x.alan = v`, göreve özel üye okuma (sonucu çalışan koda bağlı özellikler) |
| `Iterable`, `Stream` | `döngü x içinde ...` ve liste üreteci: uzunluğu bilinen kaynak ya da tembel akış |
| `PlainValuer` | Değerin düz Go karşılığı (JSON ve kontrol noktası yazımı) |
| `PathValuer`, `PathArg` | Yol alan yerleşiklerin ortak okuyucusu (Metin ya da `Yol`) |
| `Debugger`, `LiveLine` | Hata ayıklayıcı yuvası; ilerleme çubuğu gibi canlı satır |

Yorumlayıcının motorlara sunduğu ortak hizmetler:

| Hizmet | Görevi |
| :-- | :-- |
| `AddEngine`, `VM.AddTaskHook`, `VM.TaskValue`, `VM.SetTaskValue` | Motor kaydı; görev kilidi alırken çalışan kanca ve göreve özel durum (örn. tensörün türev kipi görevden göreve sızmaz) |
| `VM.Blocking` | Kilidi bırakarak uzun iş |
| `ResolvePath`, `OpenFileReader`, `ReadFileContent`, `WriteFileContent`, `WriteAtomic` | Tek yol kuralı, `.gz` saydamlığı, atomik yazma: dosya okuyan ve yazan bütün yerleşikler bunlardan geçer |
| `WriteOutput`, `SetLiveLine`, `UpdateLiveLine`, `AddSecret`, `Mask` | İnsanın göreceği çıktı noktaları; etkin canlı satır silinip yeniden çizilir, gizli değerler `***` olur |
| `SetArgs`, `Args`, `ProgramCommand`, `UnusedArgsWarning` | Betiğin komut satırı argümanları ve programı başlatan komut |
| `Interrupt`, `RequestInterrupt`, `DeferInterrupts`, `StopRequested`, `InterruptNow`, `IsInterrupt` | Nazik çıkış ve ertelenen kesme: bir kontrol noktası kurulduysa istek yalnızca bayrak koyar, kontrol noktası adım sınırında kaydedip `InterruptNow` ile durdurur. Motorların panik kurtarıcıları `IsInterrupt` ile kesmeyi yeniden atar |
| `ExitRequest`, `IsExitRequest` | Bir yerleşiğin istediği program çıkışı (`komut_satırı`: `--yardım` 0, kullanım hatası 2): `ertele`ler çalışır, `dene/yakala` yakalamaz |

### Motorların kendi kancaları

| Kanca | Görevi |
| :-- | :-- |
| `logging.SetStepHook`, `logging.StepBoundary` | Eğitimin adım sınırı: `İlerleme.güncelle`'nin her çağrısında (yorumlayıcı kilidi altında) adım ve zamanla çağrılır. Canlı profil ms/adım'ı ve adım başına işlem dağılımını buradan ölçer; kanca yokken maliyet tek atomik okumadır |
| `logging.JobDir` | Arka plan işinin klasörü (`TR_IS_KLASORU`); canlı profil durumunu oraya yazar |
| `logging.CloseAll`, `dataset.CloseAll`, `files.CloseAll`, `ml.Close` | Program bitince kaynakların kapanışı (`stdlib.Close` hepsini çağırır) |
| `process.State.Settings` | Programın `komut_satırı` sonucu; günlüğün başlangıç kaydı yazar, kontrol noktası saklayabilir |
| `checkpoint.Stateful` | Kontrol noktasına katılan motor değerleri: `StateDict` ve `LoadState` (TR'de `.durum`, `.durumu_yükle`); bugün veri okuyucuları (`dataset`). Arayüzü uygulayan bir motor `checkpoint` paketini içe aktarmaz |
| `ml.CollectParams`, `ml.CollectTensors`, `ml.CheckParamShapes`, `ml.CopyInto` | Modelin parametreleri ve adları, yüklenecek kaynakla uyum denetimi, var olan tensöre yerinde yazma; tensör dosyası yükleme ve kontrol noktası ortak kullanır |
| `ml.TensorWriter`, `ml.WalkTensorFile` | Tensör dosyasını girdi girdi yazma ve okuma (büyük modelde bellekte ikinci kopya yok) |
| `ml.SetUnseededHook` | Tohumsuz rastgelelik kullanılınca çağrılır (kontrol noktası bir kez uyarır) |
| `debugger.WatchValue`, `debugger.WatchSummarizer` | Tensör, veri kümesi, parçalayıcı gibi motor değerlerinin İzleme Masası'nda gösterimi |

## Standart kitaplık (`internal/stdlib`)

| Paket | Görevi |
| :-- | :-- |
| `stdlib` | Motorları doğru sırayla yorumlayıcıya kaydeder (`engines.go`) ve `Close` ile kapatır |
| `ml` | Tensör, matris, otomatik türev, sinir ağı işlemleri, iyileştiriciler (AdamW, SGD, Muon), özel türev, karışık hassasiyet, cihaz seçimi ve tensör dosyalarını TR değerlerine ve yerleşiklerine bağlar. Hesabı `compute/tensor` yapar; bu paket hesap yazmaz |
| `tokenizer` | `parçalayıcı_yükle` ve parçalayıcı nesnesinin üyeleri (`compute/tokenize` üzerine) |
| `dataset` | `veri_kümesi`, örnekleyici ve sıralı okuyucu yerleşikleri (`compute/dataload` üzerine) |
| `formats` | JSON ve CSV yerleşikleri |
| `files` | `Yol` tipi, `yol`, listeleme, klasör oluşturma, silme, kopyalama, taşıma, `dosya_satırları`; silme korumaları, bağlantı izlememe |
| `process` | `ortam_değişkeni` ve `komut_satırı` (seçenekler, `--yardım`, kullanım hatası); gizli adların süzgece kaydı |
| `logging` | Eğitim günlüğü (JSON satırları) ve ilerleme çubuğu; adım sınırı kancası; sayı ve süre yazımı |
| `checkpoint` | Kontrol noktası: eğitimin bütün durumunun koşu klasörüne atomik çok dosyalı kaydı, doğrulanarak yüklenmesi ve kaldığı yerden devam |
| `gpuprobe` | GPU'da dil düzeyinde eğitimin sınamaları (yalnızca testler; kartsız makinede PTX benzeticisinde koşar) |
| `common/bind` | Motorların ortak yardımcıları: adlı argüman denetimi, göreli yol, panikten hataya (`Recover`, `Safe`) |
| `common/jsonwrite` | TR değerlerini JSON'a yazan tek yer |

**Hesap çağrısı sınırı.** `compute` şekil ve sınır hatalarında panikler; `stdlib` her hesap çağrısını panikten hataya çeviren sarmalayıcıyla çağırır. Böylece kullanıcı `dene/yakala` ile yakalayabileceği bir TR hatası görür, süreç çökmez.

**Durumlu değerler.** Kesilip sürdürülen bir eğitimin kesintisizle aynı sonucu vermesi için veri okuyucuları, iyileştirici, model ve günlük birlikte saklanır. Kontrol noktası yazımı atomiktir (geçici klasör, sonra yerine konur); okuma her dosyanın boyunu ve özetini doğrular.

## Hesap çekirdeği (`internal/compute`)

| Paket | Görevi |
| :-- | :-- |
| `tensor` | n boyutlu tensör, görünümler, otomatik türev, `Backend` arayüzü, CPU arka ucu (AVX2/FMA kernelleri, Go başvuruları), iyileştiriciler, bellek sınırı, tensör sayaçları |
| `gpu` | NVIDIA arka ucu: sürücü bağlama, Go içinden üretilen PTX kernelleri, bellek havuzu, PTX benzetimi |
| `dataload` | `.npy` parçaları, `meta.json`, yerel önbellek, örnekleyici, sıralı okuyucu, önden yükleyici |
| `tokenize` | `tokenizer.json` parçalayıcıları: BPE, WordPiece, Unigram, normalleştirme, ön bölme, kendi düzenli ifade motoru |

**Tensör ve otomatik türev.** `tensor.Tensor` bir depolama (`Storage`) üzerindeki görünümdür; işlemler bir `Backend` arayüzünden geçer. İleri işlem çalışırken türev grafiği kurulur, geri yayılım grafiği ters gezer. Her işlemin CPU'da basit bir Go başvurusu vardır; hızlı yollar (vektör, assembly, paralel) onunla aynı sonucu vermelidir ve paralel hesap işçi sayısına bağlı değildir. İşlemci yetenekleri çalışma anında seçilir (`TR_CPU_AVX2=0` yeteneği kapatır); tek ikili her işlemcide çalışır.

**Kaynaştırma** (dil tanımı 15.45, `kaynaştır`). Bir fonksiyon iz tensörleriyle (depolamasız) bir kez çalışır; desteklenen işlemler hesap yapmadan bir program kurar (`fuse.go` düğümleri, `fuse_trace.go` kancaları), diğer her işlem açık hata verir. Program her çıktı için tek geçiştir: CPU'da satır satır, kaynaşmamış yolun eleman işlevleriyle (`fuse_cpu.go`); GPU'da çıktı başına tek PTX çekirdeği (`gpu/kernel_fused.go`, `tensor.Fuser`). Her düğüm kendi türüne yuvarlandığından ileri yön kaynaşmamışla bit düzeyinde aynıdır. Geri yön programın simgesel türevidir (`fuse_grad.go`): float32'de, ara değerleri yeniden hesaplayarak; çıktılar `CustomOp` ile tek dış düğüm olarak çizgeye bağlanır.

**Arka uç seçimi.** CPU her zaman vardır. GPU tembel kaydolur (`tensor.RegisterLazyBackend`): sürücü ancak bir cihaz sorulunca yüklenir; sürücü yoksa hiçbir şey kaydolmaz ve her şey CPU'da çalışır. Varsayılan cihaz `tensor.SetDefaultDevice` ile değişir. Yeni bir arka uç `Backend`'i uygular; bir işlemi desteklemiyorsa bunu açıkça söyler. Seçmeli yetenekler ek arayüzlerle sunulur (`MemoryLimiter`, `MultiTensorOps`, `GemmSetter`, `BackwardFinisher`, `Fuser` ...).

**Veri hattı.** Veri kümesi bir klasördür (`.npy` parçaları ve `meta.json`). `dataload.OpenDataset` kümeyi açar, `Sampler` belirlenimci örnekleme yapar (aynı tohum aynı sırayı verir), `SequentialReader` sıralı okur, `Prefetcher` arkada bir sonraki toplu işi hazırlar, `Cache` uzak ya da yavaş kaynağı yerelde tutar. Okuyucu durumu kaydedilip geri yüklenebilir; sürdürülen eğitim kesintisizle aynı veriyi görür.

**Parçalayıcı.** `tokenize.Load` `tokenizer.json`'ı yükler. Sonuç başvuru uygulamasının altın çıktılarıyla birebir aynıdır (`testdata/`); desteklenmeyen bileşen sessizce yok sayılmaz. Girdi güvenilmezdir: düzenli ifade motoru adım bütçesiyle çalışır.

**Dosya biçimleri sözleşmedir.** Diske yazılan biçimler (veri parçaları, meta dosyaları, parçalayıcı tanımı, kontrol noktaları, günlük, profil) Go adlarından bağımsızdır; bir ad değişikliği kullanıcının dosyalarını okunamaz kılmaz.

### GPU arka ucu (`internal/compute/gpu`)

`tensor.Backend`'in NVIDIA uygulamasıdır; otomatik türev ve üst katmanlar cihazdan bağımsızdır.

- **Kayıt:** paket yüklenirken tembel bir arka uç kaydeder; sürücü ancak bir cihaz sorulunca (`cihazlar()`, `.cihaza`, `varsayılan_cihaz`) yüklenir (`gpu.go`). Sürücü yoksa program çökmez, GPU görünmez olur ve nedeni sorulabilir (`gpu.Status`).
- **Sürücü:** cgo ve CUDA araç takımı gerekmez. Linux'ta `libcuda.so.1` dış bağımlılık olan dinamik yükleyiciyle (`cuda_linux.go`; yalnızca burası `purego` içe aktarır, `internal/layers_test.go` denetler), Windows'ta `nvcuda.dll` yalnızca sistem klasöründen yüklenir (`cuda_windows.go`). **Bedel:** bu yükleyici Linux ikilisini glibc'ye dinamik bağlar, GPU kullanmayan Linux kullanıcıları için de; musl tabanlı dağıtımlarda (Alpine) ikili açılmaz. **Karar:** varsayılan ikili GPU desteklidir; statik ikili isteyen `GOOS=linux CGO_ENABLED=0 go build -tags gpusuz ./cmd/tr` ile derler (`cuda_other.go`). Tam denetim `gpusuz` derlemenin statik kaldığını `tools/checkstatic` ile doğrular.
- **Bağlam ve iş parçacığı:** CUDA bağlamı işletim sistemi iş parçacığına bağlıdır. Her dışa açık yöntem girişte arka ucun kilidini alır, gorutini iş parçacığına kilitler ve cihazın birincil bağlamını bağlar; bütün işler tek akışta sırayla yürür. Ayrı bir GPU gorutinine kanalla iş göndermek yerine bu yol seçildi: kanal gidiş-dönüşü ve gorutin geçişi yok, panikler (TR hataları) çağıranın gorutininde kalır (`dene/yakala`).
- **Çekirdekler:** Go içinden PTX olarak üretilir (`ptx*.go` yazıcı ve komut yardımcıları, `kernel_*.go` kernel aileleri, `backend_*.go` arka uç yöntemleri; taban hedef `sm_52`, `cp.async` kullananlar `sm_80`); sürücü ilk kullanımda karta derler, modüller süreç boyunca önbellekte. Çekirdek imzası her gösterici parametresinin yazılıp yazılmadığını bilir; başlatma yeri okuma/yazma bildirir ve uyuşmazlık iç hatadır. Birleşik kerneller: çapraz entropi (`kernel_ce.go`), dikkat (`kernel_flash*.go`: puan matrisi belleğe yazılmaz, döşemeli çevrimiçi softmax), çok tensörlü iyileştirici/ölçekleme/kareler toplamı (`kernel_multi.go`: bütün parametreler tek başlatmada), sıralama ve birikimli toplam (`kernel_sort.go`). Az çıktılı uzun indirgemeler iki aşamalı ve belirlenimcidir; çıktısını tümüyle yazan işlemler taze gradyanı sıfırlamadan üzerine yazar (`tensor.GemmSetter` vb.). Dikkat yolu `TR_GPU_DIKKAT=birlesik|carpim` ile zorlanabilir (ölçüm için).
- **Matris çarpımı:** makinede NVIDIA cuBLASLt kuruluysa float32 çarpım onunla yapılır (TF32 kapalı; sürücü gibi çalışma anında yüklenir, TR dağıtmaz). Arama sırası ve güvenlik kuralları `gemm_lib_path.go`'dadır (`TR_GPU_CUBLASLT` tam yol; çalışma klasörü hiçbir zaman aranmaz; bkz. [`SECURITY.md`](../SECURITY.md)). Kitaplık bir şekli yapamazsa ya da çağrı hata dönerse panik yerine kendi çekirdeğimize düşülür (`kernel_gemm.go`; `TR_GPU_KENDI_CARPIM=1` onu zorlar). Tek satırlık çarpımlar (M = 1: çıkarımda bir parça) kitaplığa değil `kernel_gemv.go`'ya gider: ağırlık birleşik erişimle, çok blokla, belirlenimci sırayla okunur (`TR_GPU_GEMV=0` kapatır). Hangi yolun kullanıldığı `gpu.Status()`'ta ve profilde görünür.
- **Bellek:** küçük bloklar için boyut sınıflı, 1 MB üstü için parçalı (best-fit, bölme, birleştirme) havuz (`memory.go`, `memory_blocks.go`). Depolama üç yoldan havuza döner (tek akış sayesinde eşzamanlama gerekmez): belirlenimci bırakma (aşağıda; son sahibi gidince), otomatik türevin ara gradyanları açıkça iade etmesi ve çöp toplayıcının temizliği (kaçan değerler). Çöp toplayıcı cihaz belleğini görmediğinden, son toplamadan bu yana verilip açıkça iade edilmeyen bellek eşiği aşınca toplama başlatılır (`TR_GPU_COP_ESIGI` MB ile değişir); her geri yayılımın sonunda `tensor.BackwardFinisher` çağrılır. Havuz canlı/havuzda/ayrılmış/tepe sayaçlarını tutar.
- **Belirlenimci bırakma** (dil tanımı 15.44): `tensor` depolamanın sahiplerini sayar (başlıklar ve geri yayılımın kaydettikleri; `lifetime.go`); yorumlayıcı bir değere artık ulaşılamayacağını bildiğinde onu bırakır (`interp.Owned`, `ownership.go`: çerçeve, bağ sayısı, kaçış); `ml.TensorValue` ikisini bağlar (başlığı saran değer sayısı). Emin olunamayan her değer çöp toplayıcıya kalır; bırakılmış başlığa erişim iç hatadır. `TR_BIRAKMA=0` kapatır (tanılama).
- **Kayıt ve oynatma (CUDA Graph):** işler sürücünün oluşturduğu akışta yürür (varsayılan akış kaydedilemez). `graph.go` bir iş dizisini kaydeder (RELAXED kip), çizgeye kurar ve tek çağrıyla oynatır; kayıtta ayrılan bellek kayda özel havuzdan gelir (kayıt kapanana kadar başka işe verilmez); kayıttan önce ayrılıp kayda giren depolamaları kayıt tutar (`privatePool.hold`: çöp toplayıcı bırakamaz, açık bırakma kaydın bırakılmasına ertelenir); cihazdan okuma, konaktan yükleme ve eşzamanlama kayıt sırasında kodlu hatadır (`H4065`–`H4068`). Benzetim kaydı da yürütür (işler saklanır, oynatmada sıraya konur). Anlam PyTorch `torch.cuda.graph` ile aynıdır.
- **Profil:** `TR_GPU_PROFIL=1` (ya da `gpu.EnableProfile`) her başlatmayı iki zamanlama olayıyla ölçer: kernel ve aile başına çağrı ve süre, adım başına duvar, GPU, boşluk, konak ve eşzamanlama süreleri; program sonunda özet standart hataya yazılır (`gpu.WriteProfileSummary`, `gpu.Close`). Canlı izleme için Go arayüzü `gpu.KernelSamples` ve `gpu.MemorySamples`'tır (canlı profil bunları okur; aşağıda). Çekirdek ailesi adları (`carpim` ...) profil çıktısında ve araçlarda kullanılan metinlerdir, Go adlarına bağlı değildir.
- **Benzetim:** `sim*.go` PTX altkümesini yorumlayan bir sürücü benzetimidir; GPU sınamaları donanımsız makinede onda koşar. Benzetim gerçek GPU'nun reddedeceği kodu (sınır dışı ve hizasız erişim, dağınık engel, paylaşımlı bellek yarışı) da reddeder; böylece kartsız makinede yakalanan hata gerçek kartta da hatadır. `TR_GPU_BENZETIM=1` ile `tr` de benzetimi `"gpu"` olarak kullanır (geliştirme aracı). Çekirdeklere eklenen her PTX komutu benzeticiye de eklenir. **Bilinen sınırlar** (benzetimde geçip donanımda kalabilecekler): bölgeler havuzun boyut sınıfı kadar ayrılır, ayrıştırıcı `ptxas` kadar katı değildir, bloklar sırayla yürüdüğünden genel bellekte iş parçacıkları arası yarış görünmez, yaklaşık komutlar (`ex2`, `lg2`, `sin`, `cos`) ve `fma` donanımla birkaç ulp farklı olabilir (sınamalar toleranslıdır).
- **Colab:** `tools/gpu/` (model ve mikro kıyas betikleri, `COLAB.md`). Mikro kıyaslar `TR_GPU_MIKRO=1` ile koşar.

## Geliştirici araçları

| Paket | Görevi |
| :-- | :-- |
| `cmd/tr` | `tr` komutu: alt komutları ayrıştırır (çalıştır, kabuk, `kontrol`, `derle`, `test`, `biçimle`, `kodla`, `çöz`, `göster`, `izle`, `profil`, `hata_ayıkla`, `dap`), işi `internal/`'a bırakır; kitaplığı ve Ctrl+C işleyicisini bağlar |
| `cmd/tr/e2e` | Uçtan uca testler (yalnızca testler): ikiliyi bir kez derler, alt süreç olarak çalıştırır; komutlar, çıkış kodları, kesme, kontrol noktasından devam, belgelerdeki kod blokları ve bağlantılar, VS Code eklentisi |
| `devtools/debugger` | Terminal hata ayıklayıcısı ve düzenleyicinin hata ayıklama sunucusu (`tr dap`, Debug Adapter Protocol). Yüklenirken `interp.NewDebugger`'ı kurar; gizli değerleri düzenleyiciye göndermez |
| `devtools/formatter` | `tr biçimle`: anlamı değiştirmeyen tek standart biçim (ağaç dökümü aynı kalmazsa sonuç reddedilir) |
| `devtools/testrunner` | `tr test`: `test "ad" { ... }` bloklarını yalıtılmış kapsamda koşar, Türkçe raporlar |
| `devtools/profiler` | Canlı profil: `tr izle` ve `tr profil`'in çekirdeği, arka plan işinin `profil.json`'u (aşağıda) |
| `devtools/codeindex` | TR kodunun dizini: tanım, kullananlar, sınayanlar, etki (`tr dizin ...` ve ileride dil sunucusu); `internal/lang` dışında bağımlılığı yok |
| `editors/vscode` | VS Code eklentisi; renklendirme listeleri kod listesinden üretilir, hata ayıklama ve izleme komutları yukarıdaki sunucu ve profil dosyalarına bağlanır |

### Canlı profil (`internal/devtools/profiler`)

`tr izle` (canlı tablo) ve `tr profil` (son rapor) aynı çekirdeği kullanır; arka plan işi olarak çalışan program (`TR_IS_KLASORU` iş klasörünü verir) da canlı durumunu buradan `profil.json`'a yazar. Ağ portu açılmaz: dışarıya yalnızca dosya yazılır. Dört kaynaktan okur:

- **Tensör sayaçları** (`internal/compute/tensor/counter*.go`): her zaman açık. Her işlem türü (`tensor.OpKind`), yön (ileri / geri yayılım) ve cihaz (CPU / GPU) için çağrı sayısı (kesin), ayrılan bayt (kesin) ve süre. CPU'da süre uyarlamalı örneklenir: kısa işlemlerde her çağrı p = 1/32 olasılıkla ölçülür ve ölçülen süre 1/p ile ağırlıklanır (Horvitz–Thompson; p yalnızca geçmiş ölçümlere bağlı olduğundan yansız); uzun işlemde (`longOpTicks`) her çağrı ölçülür. GPU işlemlerinin her çağrısı ölçülür. Saat amd64'te değişmez zaman damgası sayacıdır (`counter_clock_amd64.go`), başka yerde monoton saat. Sayaçlar kilitsizdir (yarış denetimli derlemede atomik: `counter_race.go`). Okuma: `tensor.CounterSnapshot`, `tensor.ActiveOp`.
- **Örtük kopya sayaçları** (`internal/compute/tensor/implicit.go`): her zaman açık. Arka ucun her kopyası, tür dönüşümü ve ekleyerek yazması (`Copy`, `CopyStrided`, `AddTo`, `ExpandAdd`, `ScatterAdd`; GPU'da her `copyStrided`) o anda çalışan işleme (üst işlem; yoksa "diğer") bağlanır: çağrı ve eleman kesin, süre yalnızca CPU'da ve en az 4096 elemanlı kopyada ölçülür (GPU'da konak süresi yalnızca başlatmadır). PyTorch profilinde `aten::copy_` / `aten::to` çağıran işlemin altında görünür; burada tablo düzdür, üst işlem anahtardır. İşlemin kendisi olan kopyalar (`.kopya()`, `.türe`) işlem tablosundadır, örtük tabloya alınmaz. Okuma: `tensor.ImplicitSnapshot`.
- **Yorumlayıcı örneklemesi** (`sampler.go`): yalnızca `tr izle` ve `tr profil` altında. Yorumlayıcının hata ayıklayıcı yuvasına (`VM.Debugger`) bağlanır; yuva boşken yorumlayıcının her deyimdeki maliyeti bir nil denetimidir. Açıkken her deyim konumunu (fonksiyon, satır) çağrı derinliğinin yuvasına yazar; ayrı bir gorutin 1 ms'de bir o anki konuma geçen süreyi ve o an bir tensör işlemi çalışıyorsa tensör payını ekler. `kesme_noktası` profil altında yok sayılır.
- **Adım sınırı** (`logging.SetStepHook`): ms/adım ve adım başına işlem dağılımı buradan.
- **GPU arka ucunun profili** (`gpu.KernelSamples`, `gpu.MemorySamples`): `tr izle` ve `tr profil` kernel ölçümünü açar (`gpu.EnableProfile`; `TR_GPU_PROFIL=1` ile zaten açıksa dokunmaz); program bitince, motorlar kapanmadan önce (`Profile.BeforeClose`, komut satırının `kapanmadanOnce` işleri) cihazı eşzamanlayıp arka ucun raporunu alır. Arka plan işinin profili ölçümü açmaz; GPU belleği yine okunur. Okumalar arka ucu beklemez.

Profil saniyede bir anlık görüntü alır (`snapshot.go`), son 5 saniyenin farklarından pencere ölçülerini kurar (canlı tablonun hızları), dinleyiciye (canlı tablo) verir ve iş klasörü varsa dosyaya yazar. Program bitince (hata, `çıkış` ya da Ctrl+C ile bitse de) son raporu kurar. Sayaçlar süreç genelidir; profil başlangıcındaki değerler çıkarılır.

**Canlı tablo** (`screen.go`): standart hata bir terminalse tablo ekranın başından denetim dizileriyle yeniden çizilir (Windows'ta konsolun sanal terminal kipi açılır; `live_windows.go`). Programın standart çıkışı ve hatası bir boruya alınır (`capture.go`): tamamı bir dosyaya yazılır, son satırları tablonun altında görünür, program bitince yeniden yazılır; boru terminal olmadığından ilerleme çubuğu düz satır kipine geçer. Terminal değilse çıktı olduğu gibi akar, 10 saniyede bir tek satırlık özet eklenir.

**Dosya biçimi** (`snapshot.go`): `profil.json` (canlı, iş klasöründe) ve `speed.json` / `tr profil --json` (son rapor) aynı yapıdadır. Anahtarlar dosya biçiminin parçasıdır (yazım dilinden bağımsız, çevrilmez); işlemlerin `kod` alanı değişmez anahtardır, `ad` insana gösterilen addır (dildeki ad; çevrilebilir). Süreler saniye, baytlar bayt, paylar 0..1'dir. Yazma atomiktir (aynı klasörde geçici dosya + yeniden adlandırma); yazma başarısızsa sonraki saniyede yeniden denenir, program hiçbir durumda durmaz. Sürüm 2 (sürüm 1'e göre `tensör.örtük` eklendi):

```jsonc
{
  "biçim": "tr-profil", "sürüm": 2, "tür": "canlı",        // "canlı" ya da "rapor"
  "program": "egit.tr", "başlangıç": "2026-10-19T08:49:12.511+03:00",
  "zaman": "2026-10-19T08:49:24.511+03:00", "süre_sn": 12.0, "bitti": false,
  "çıkış_kodu": 0,                                          // yalnızca bitince
  "adım": {"sayı": 715, "toplam": 1500, "tüm_adım": 715,   // ilerleme yoksa null; sayı ve toplam
    "ms_adım": 16.7, "adım_sn": 59.7},   // son çubuğun, tüm_adım bütün çubukların (epoch'lar) adımı
  "tensör": {"süre_sn": 11.4, "pay": 0.95, "çağrı": 772761, "bayt": 7642211100,
    "işlemler": [{"kod": "matris_carpimi", "ad": "matris çarpımı (@)", "yön": "ileri",
      "cihaz": "cpu", "çağrı": 160966, "ölçüm": 160529, "süre_sn": 4.99, "pay": 0.44,
      "bayt": 2044092928, "ms_adım": 6.99}],               // süreye göre sıralı, en çok 30
    "örtük": [{"kod": "adimli_kopya", "üst_kod": "matris_carpimi", "ad": "adımlı kopya",
      "üst_ad": "matris çarpımı (@)", "yön": "ileri", "cihaz": "cpu", "çağrı": 45,
      "eleman": 17940480, "ölçüm": 45, "süre_sn": 0.022}]}, // elemana göre sıralı, en çok 30
  "gpu": {"kullanıldı": true, "ölçüm": true, "not": "", "süre_sn": 0.088,
    "aileler": [{"ad": "carpim", "çağrı": 40, "süre_sn": 0.069, "pay": 0.78}],
    "çekirdekler": [{"ad": "carpim_64_64_16_4_4_false_false_true", "çağrı": 20, "süre_sn": 0.040, "pay": 0.46}],
    "bellek": [{"cihaz": 0, "canlı_bayt": 109568, "ayrılmış_bayt": 114176, "havuzda_bayt": 4608,
      "tepe_canlı_bayt": 113664, "tepe_ayrılmış_bayt": 114176, "çöp_toplama": 0}]},
  "örnekleme": {"aralık_ms": 1, "örnek": 11800, "süre_sn": 11.8, "tensör_payı": 0.95,  // kapalıysa null
    "satırlar": [{"dosya": "examples/tr/41_gpt.tr", "satır": 49, "fonksiyon": "",
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

Satırın `dosya`sı ana programın dışındaysa (aktarılan modül) boştur: örnek yalnızca fonksiyon adını ve satırı bilir. `ölçüm`, ölçülen çağrı sayısıdır (`çağrı`dan küçükse süre örneklenmiştir). Canlı CPU tensör baytı ayrıca izlenmez (CPU tensörleri Go yığınındadır); `cpu_tensör_ayrılan_bayt` ayırma akışıdır.

**Maliyet** (ölçüm yöntemi: önce/sonra test ikilileri dönüşümlü, P çekirdeklere sabit; makine yüklüyken ölçüldü, boş makinede yinelenmeli): sayaçların işlem başına yükü ~4 ns (`BenchmarkCounterStartStop`); 16 elemanlı tensörlerle `BenchmarkCounterSmall*` (işlem başına ~350 ns) %1–3; 64 boyutlu küçük MLP eğitim adımında (~75 µs) gürültü içinde. Örnekleme açıkken çağrı yoğun TR kodu ~2 kat yavaşlar (hata ayıklayıcı yuvası doluyken yorumlayıcı fonksiyon ortamlarını havuzlamaz).

## Testlerin yeri

Test türleri ve adlandırma kuralı [`internal/README.md`](../internal/README.md#testler)'dedir (`x_test.go`, `behavior_<konu>_test.go`, `helpers_test.go`). Hangi sınama nerede:

| Ne sınanır | Nerede |
| :-- | :-- |
| Paketin kendi davranışı | paketin birim testleri; birden çok dosyayı kesen davranışlar `behavior_*_test.go` ve paketin README'sinin `## Testler` bölümünde anılır |
| Katmanlar, dil içi sıra, dış bağımlılık; dosya düzeni | `internal/layers_test.go`, `internal/layout_test.go` |
| Çıplak ad, ileti kodu kullanımı, üretilmiş dosyaların güncelliği | `tools/gen/*` testleri ve `-check` bayrağı |
| Dilin kuralları | `tests/*_test.tr` (`tr test tests`); her test bir dil kuralı numarasıyla başlar, kapı testi `internal/devtools/testrunner` içindedir |
| TR ile yazılı kitaplık | `lib/*_test.tr` (`tr test lib`) |
| `tr` komutunun uçtan uca davranışı, belgelerdeki bağlantılar ve kod blokları, VS Code eklentisi | `cmd/tr/e2e` |
| GPU'da dil düzeyinde eğitim | `internal/stdlib/gpuprobe` (kartsızsa PTX benzetimi) |
| Güvenilmeyen girdiler (bulanıklaştırma) | `Fuzz...` testleri; uzun koşu `tools/fuzz.sh` |
| Hız | `tools/bench` kıyas seti ([`README`](../tools/bench/README.md)); sıcak yola dokunan değişiklik önce/sonra ölçülür |

Bir iş `bash tools/full_check.sh` geçince biter; otomatik sınama (`.github/workflows/ci.yml`) aynı betiği çalıştırır.

## Klasörler

| Klasör | İçerik |
| :-- | :-- |
| `cmd/tr/` | Tek ikili |
| `internal/` | Bütün Go kodu (yukarıdaki katmanlar); `reporoot` testlerin depo kökünü bulur |
| `lib/` | TR ile yazılı standart kitaplık, ikiliye gömülü (`aktar "tr/katman"`, `aktar "tr/iyilestirici"`; testleri `tr test lib`) |
| `editors/vscode/` | Düzenleyici eklentisi (kullanıcıya gider) |
| `docs/` | Belgeler: [`guide/`](guide/README.md) (kullanıcı kılavuzu), [`spec/`](spec/README.md) (dil tanımı), [`codes.md`](codes.md) (üretilir), [`roadmap.md`](roadmap.md), [`development.md`](development.md), [`recipes.md`](recipes.md), bu dosya |
| `examples/` | Öğrenme sırasıyla numaralı örnek programlar: `tr/` Türkçe, `en/` İngilizce yazım (aynı numara aynı program) |
| `tests/` | Dil tanımının kural testleri (`tr test tests`); `negative/`, `coded/`, `data/` |
| `tools/` | Depo bakım araçları, kullanıcıya gitmez: `full_check.sh` ("bitti" tanımı), `fuzz.sh`, `gen/` (üreteçler), `impact/` (bir adın etkisini koddan hesaplar), `checkstatic/` (statik ikili denetimi), `bench/` (hız kıyas seti), `gpu/` (GPU sınama ve kıyas betikleri, `COLAB.md`) |
| `bin/`, `scratch/` | Derleme çıktısı ve kişisel deneme klasörü; depoya girmez |

## Sahiplik

| Rol | Alan |
| :-- | :-- |
| Dil çekirdeği | `internal/lang/**`, `internal/stdlib/files`, `internal/devtools/{formatter,testrunner,debugger}`, `internal/reporoot`, `cmd/tr`, `docs/spec/`, bu dosya |
| Tensör ve GPU | `internal/compute/tensor`, `internal/compute/gpu`, `internal/stdlib/ml` |
| Veri ve parçalayıcı | `internal/compute/{tokenize,dataload}`, `internal/stdlib/{tokenizer,dataset,formats}` |
| Araçlar | `editors/`, `.vscode/`, `docs/guide/`, `README.md` |
| Hız | `tools/bench/` |
| Test | `tests/`, `.github/`, eklenti testleri |
