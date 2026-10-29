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

Dış bağımlılık izne bağlıdır: bugün iki tane, `purego` (GPU sürücüsünü cgo'suz yüklemek için, `internal/compute/gpu`) ve `golang.org/x/text`'in yalnız `unicode/norm` paketi (adların NFC karşılaştırması ve benzerlik iskeleti, dil tanımı 2.3 ve 2.1a; `internal/lang/lexer`). Benzerlik çizelgesi Unicode'un `confusables.txt`'inden (17.0.0) `tools/gen/confusables` ile üretilip gömülür; ham dosya depoya girmez. Denetim: `TestExternalDependencies`.

### Dil çekirdeğinin iç sırası

Dil çekirdeğinde de bir sıra vardır; bir paket yalnızca kendisinden öncekileri içe aktarır (`internal/layers_test.go`, `langOrder`; denetim `TestLangOrder`):

```
messages → turkish → codes → hint → lexer → ast → parser → checker → source → interp
```

| Paket | Görevi |
| :-- | :-- |
| `messages` | İleti kodları (`H0000`...) ve etkin dildeki metinleri; en alttır, yalnızca standart kitaplığa dayanır |
| `turkish` | Türkçe harf kuralları, ünlü uyumuna göre ekler, en yakın ad önerisi, dosya hatası metinleri, kod çerçevesi |
| `codes` | Dilin kullanıcıya görünen bütün adlarının sabit kodları (`TR_fn_print`...); dil dosyaları ve tanım dosyası |
| `hint` | ASCII yazılan ve başka dillerden alışılmış adların, tanımsız ada en yakın adın ipuçları (tek kaynak) |
| `lexer` | Sözcük çözümleyici; kodlu kaynağı kimlikleriyle (`TR_print`) okur, konumları istenen görünüme göre verir (kişinin dilindeki görünüm ya da diskteki metin) |
| `ast` | Sözdizimi ağacı düğümleri; yalnızca veri taşır, konumdan bağımsız döküm yazar |
| `parser` | Öncelik tabanlı (Pratt) ifade ve özyinelemeli iniş deyim ayrıştırıcısı |
| `checker` | Çalıştırmadan önce tip denetimi, uyarılar, tanımsız ad denetimi |
| `source` | Kaynaktan programa tek giriş noktası, kodlu biçimin çeviricisi |
| `interp` | Derleyici ve yorumlayıcı; değerler, çekirdek yerleşikler, modüller, eşzamanlılık, kancalar |

Sıra bir bağımlılık zinciri olduğu kadar bir döngü korumasıdır: sözcük çözümleyici yorumlayıcıyı, denetleyici yorumlayıcıyı bilmez. Yeni bir dil paketi listeye yerine göre eklenir.

## Bir programın kaynaktan çalışmaya yolu

`tr egit.tr` yazıldığında olan şudur:

1. **Dosya okunur ve bölünür.** `source.Split` BOM'u ve kodlu biçimin başlığını ayırır. Komut satırı, kabuk, test koşucusu, hata ayıklama sunucusu ve modül yükleyici bir `.tr` dosyasını yalnızca `internal/lang/source` üzerinden okur; başlık kuralları tek yerdedir.
2. **Sözcüklere ayrılır.** Sözcük çözümleyici kaynağı sözcük birimlerine çevirir. Program içeride kimliklerle çalışır: kodlu dosyanın her dil sözcüğü (`TR_if`, `TR_print`) kimliğiyle okunur; ayrıştırıcı, denetleyici ve yorumlayıcı hiçbir yazım dilinin adını görmez (`codes.Active` kimlik listesidir; koruyan test `TestProgramRunsOnIdentities`). Türkçe, İngilizce ve sonra eklenecek diller yalnızca gösterimdir, hiçbirinin ayrıcalığı yoktur: hata çerçevesi ve hata ayıklayıcı dosyanın kişinin dilindeki (`TR_DIL`) görünümünü gösterir, satır ve sütun o görünüme göredir (17.5). Başlıksız metin bugün Türkçe yazılmış sayılır ve önce kodlu biçime çevrilir (17.7; kalkacak: [yol haritası](roadmap.md) madde 15).
3. **Ayrıştırılır.** `parser.New` sözdizimi ağacını (`ast`) kurar. Bağlam kuralları (`TR_break` yalnızca döngüde ...) burada denetlenir; hatalar tek mesaja toplanıp toparlanır.
4. **Denetlenir.** `source.Prepare` istenirse `checker` ile tip ve ad denetimi yapar. Denetleyici hiçbir şey çalıştırmaz; emin olamadığında susar.
5. **Derlenir.** `interp` ağacı bir kez Go kapanışlarına derler (`compiler*.go`): değişkenler yuvalara, üye adları kodlara çözülür; çalışma anında ad karşılaştırılmaz.
6. **Çalışır.** `VM.Run` programı yürütür. Eşzamanlı görevler tek yorumlayıcı kilidi altında çalışır; uzun işler (disk, ağ, uyku, ağır hesap) `VM.Blocking` içinde kilidi bırakır.
7. **Kapanır.** Program bitince (hata, `çıkış` ya da Ctrl+C ile de) motorların açık kaynakları kapatılır (`stdlib.Close`).

Hatalar aşama aşama aynı yolu izler: ileti bir kodla (`messages.New`) kurulur, satırı `interp.LineError` (`messages.PosError`) ekler, kullanıcıya etkin dilin metniyle yazılır. Kesme isteği (Ctrl+C) hata değildir; `dene/yakala` onu yakalamaz.

## Kod listesi ve ileti kataloğu

Kullanıcıya görünen her ad ve her ileti sabit bir koda bağlıdır; Go kodu kodu yazar, metin dil dosyasından gelir. Bu, bir adı ya da iletiyi değiştirmeyi (ya da başka bir dile çevirmeyi) Go koduna dokunmadan, yalnızca bir dosya satırını değiştirerek yapmayı sağlar.

| | Kod listesi (`internal/lang/codes`) | İleti kataloğu (`internal/lang/messages`) |
| :-- | :-- | :-- |
| Kod | Kimlik `TR_<sınıf>_<İngilizce ad>` (`TR_kw_if`, `TR_fn_print`, `TR_mem_shape`); sınıf kodun ne olduğunu gösterir; program metninde sınıfsız yazılır, yeri sınıfı belirler (`TR_if`, `TR_print(x)`, `x.TR_shape`, 17.2) | `H` ve dört basamak (`H7003`); binler basamağı alanı gösterir |
| Dilden bağımsız | `definitions.json`: tür, imza, dönüş tipi, örnek | `definitions.json`: her kodun parametre adları |
| Dile bağlı | her yazım dilinin bir dosyası: `tr.json`, `en.json` | her yazım dilinin bir dosyası: `tr.json`, `en.json` (her kodu taşır; eksik çeviri test hatasıdır) |
| Go'dan kullanım | `codes.Name(codes.FnPrint)`, `codes.Find`, `codes.Active` | `messages.New(messages.H7003, ...)`, `messages.Text`, `messages.Has`, `messages.Internal` |
| Dil | Program kimliklerle okunur ve çalışır (`codes.Active` kimlik listesidir); hiçbir yazım dilinin ayrıcalığı yoktur. Kişinin yazım dili `TR_DIL` (`codes.Display`, `codes.DisplayName`) yalnızca gösterimdir (17.7). Kalan geçici istisna: başlıksız metin ve değerlerin `Inspect` metni Türkçedir (`codes.Source`) | iletilerin dili `TR_DIL` (`messages.LanguageEnvVar`); metni eksik ileti bugün Türkçeye düşer (geçici istisna) |

Yazım kuralları (kod değişmez, ad değişebilir; her işin tek adı; parametreler adlıdır; iç hata `H9000`) paketlerin README'lerindedir: [`codes`](../internal/lang/codes/README.md), [`messages`](../internal/lang/messages/README.md). Yeni ad için [`development.md`](development.md#yeni-kod-ad-isteği), yeni ileti için [`recipes.md`](recipes.md).

**Üretilmiş dosyalar.** Kod listesinden ve ileti kataloğundan üretilen dosyalar elle düzenlenmez; üreteçler `tools/gen/` altındadır ve `-check` bayrağıyla güncelliği sınanır (tam denetim hepsini koşar). Üretmek için: `go generate ./internal/lang/codes ./internal/lang/messages`.

| Üreteç | Ürettiği |
| :-- | :-- |
| `tools/gen/codeconsts` | `internal/lang/codes/codes_generated.go` (kod sabitleri); Go metnindeki çıplak ad denetimi |
| `tools/gen/codesdoc` | [`docs/codes.md`](codes.md) (kod listesi belgesi) |
| `tools/gen/grammar` | VS Code eklentisinin renklendirme listeleri (`editors/vscode/syntaxes/tr.tmLanguage.json`) |
| `tools/gen/snippets` | VS Code eklentisinin her yazım dilindeki kod şablonları (`editors/vscode/snippets/snippets.json`) |
| `tools/gen/msgconsts` | `internal/lang/messages/codes_generated.go` (ileti sabitleri); ileti kullanım denetimi |

## Kancalar: dil çekirdeği motorları nasıl bilir

Yorumlayıcı (`internal/lang/interp`) motorları ve araçları içe aktarmaz; onlar yorumlayıcıya kancalarla bağlanır. Böylece dil çekirdeği tek başına derlenir ve sınanır, her yetenek yorumlayıcıda bir arayüz olarak tanımlanır, uygulaması motorda yazılır.

**Motor kaydı.** `internal/stdlib` paketi yüklenirken (`init`) her motoru `interp.AddEngine` ile ekler; her yeni `VM` çekirdek yerleşiklerden sonra motorları eklenme sırasıyla kaydeder. Sıra önemlidir (`internal/stdlib/engines.go`):

1. `formats`, `files`
2. `ml`: çekirdek yerleşiklerin (toplam, ortalama, min, max, karekök) tensör sürümleriyle sarıldığı için onlardan sonra
3. `tokenizer`, `dataset`: tensör değerleri üretir
4. `process`, `logging`, `checkpoint`: tek kayıtta, çünkü günlüğün başlangıç kaydı ve kontrol noktası programın komut satırı ayarlarını (`process.State`) kullanır

Tam dili çalıştıran her program (`cmd/tr`, `internal/devtools/testrunner`) bu paketi `import _` ile içe aktarır. Program bitince `stdlib.Close` motorların kaynaklarını kapatır.

**Hata ayıklayıcı.** Koddaki `TR_breakpoint` deyimi `interp.NewDebugger` değişkeniyle bir `interp.Debugger` açar; bu değişkeni `internal/devtools/debugger` paketi yüklenirken (`wire.go`) kurar. Değişken boşsa deyim yok sayılır. Profilci de aynı yuvaya (`VM.Debugger`) bağlanır.

**Kitaplık.** `lib` paketi `*.tr` dosyalarını `go:embed` ile ikiliye gömer; `cmd/tr` bunları `interp.SetLibrary` ile bağlar. `TR_import "tr/katman"` yorumlayıcının kitaplık okuyucusundan (`interp.ReadLibrary`) gelir; `tr/` öneki dilin kendisine ayrılmıştır. `TR_cmd_build` çıktısı yorumlayıcının kopyası olduğundan kitaplık ona da girer. Ayrıntı: [`lib/README.md`](../lib/README.md).

### Yorumlayıcının arayüzleri

Bir motor değerinin dile uyması, `interp` içindeki arayüzlerden birini uygulamasıyla olur (tip denetimiyle bulunur; her uygulayıcıda derleme anı denetimi kuralı için bkz. [`internal/README.md`](../internal/README.md#kancalar)):

| Arayüz (`interp`) | Ne sağlar |
| :-- | :-- |
| `Equaler`, `OperatorHook`, `TaskOperatorHook`, `Negatable` | Eşitlik, ikili ve tekli işleçler (tensör `+ - * / @ **`) |
| `Indexable`, `IndexAssignable`, `MemberAssignable`, `TaskMemberHook` | `x[i]`, `x[i] = v`, `x.alan = v`, göreve özel üye okuma (sonucu çalışan koda bağlı özellikler) |
| `Iterable`, `Stream` | `TR_for x TR_in ...` ve liste üreteci: uzunluğu bilinen kaynak ya da tembel akış |
| `PlainValuer` | Değerin düz Go karşılığı (JSON ve kontrol noktası yazımı) |
| `PathValuer`, `PathArg` | Yol alan yerleşiklerin ortak okuyucusu (Metin ya da `TR_Path`) |
| `Debugger`, `LiveLine` | Hata ayıklayıcı yuvası; ilerleme çubuğu gibi canlı satır |

Yorumlayıcının motorlara sunduğu ortak hizmetler:

| Hizmet | Görevi |
| :-- | :-- |
| `AddEngine`, `VM.AddTaskHook`, `VM.TaskValue`, `VM.SetTaskValue` | Motor kaydı; görev kilidi alırken çalışan kanca ve göreve özel durum (örn. tensörün türev kipi görevden göreve sızmaz) |
| `VM.Blocking` | Kilidi bırakarak uzun iş |
| `ResolvePath`, `OpenFileReader`, `ReadFileContent`, `WriteFileContent`, `WriteAtomic` | Tek yol kuralı, `.gz` saydamlığı, atomik yazma: dosya okuyan ve yazan bütün yerleşikler bunlardan geçer |
| `WriteOutput`, `SetLiveLine`, `UpdateLiveLine`, `AddSecret`, `Mask` | İnsanın göreceği çıktı noktaları; etkin canlı satır silinip yeniden çizilir, gizli değerler `***` olur |
| `SetArgs`, `Args`, `ProgramCommand`, `UnusedArgsWarning` | Betiğin komut satırı argümanları ve programı başlatan komut |
| `RequestStop`, `DeferStops`, `HandleStop`, `StopRequested`, `Interrupt`, `InterruptNow`, `IsInterrupt`, `AwaitNew`, `Abandonable` | Durdurma isteği (1.5–1.7): istek bayrak koyar; bir kontrol noktası kurulduysa erteleme penceresinde (`DeferStops`) adım sınırında kaydedip `InterruptNow` ile durdurur; `TR_handle_stop` çağrılınca program durdurmayı üstlenir (`HandleStop`). Yeni bir şey bekleyen yerleşik `AwaitNew`, bırakılabilen bekleme `Abandonable` ile sarılır. Motorların panik kurtarıcıları `IsInterrupt` ile kesmeyi yeniden atar |
| `RequestExit`, `IsExitRequest`, `End` | Programın çıkışı (1.9–1.14): `TR_exit` ve bir yerleşiğin istediği çıkış (`TR_command_line`: `--help` 0, kullanım hatası 2) `TR_defer`leri çalıştırır, `TR_try` yakalamaz; `End` sinyal, yakalanmamış hata ve `TR_exit` kodunun öncelik sırasıyla çıkış kodunu verir |

### Motorların kendi kancaları

| Kanca | Görevi |
| :-- | :-- |
| `logging.SetStepHook`, `logging.StepBoundary` | Eğitimin adım sınırı: `İlerleme.güncelle`'nin her çağrısında (yorumlayıcı kilidi altında) adım ve zamanla çağrılır. Canlı profil ms/adım'ı ve adım başına işlem dağılımını buradan ölçer; kanca yokken maliyet tek atomik okumadır |
| `logging.JobDir` | Arka plan işinin klasörü (`TR_IS_KLASORU`); canlı profil durumunu oraya yazar |
| `logging.CloseAll`, `dataset.CloseAll`, `files.CloseAll`, `ml.Close` | Program bitince kaynakların kapanışı (`stdlib.Close` hepsini çağırır) |
| `process.State.Settings` | Programın `TR_command_line` sonucu; günlüğün başlangıç kaydı yazar, kontrol noktası saklayabilir |
| `checkpoint.Stateful` | Kontrol noktasına katılan motor değerleri: `StateDict` ve `LoadState` (TR'de `.TR_state`, `.TR_load_state`); bugün veri okuyucuları (`dataset`). Arayüzü uygulayan bir motor `checkpoint` paketini içe aktarmaz |
| `ml.CollectParams`, `ml.CollectTensors`, `ml.CheckParamShapes`, `ml.CopyInto` | Modelin parametreleri ve adları, yüklenecek kaynakla uyum denetimi, var olan tensöre yerinde yazma; tensör dosyası yükleme ve kontrol noktası ortak kullanır |
| `ml.TensorWriter`, `ml.WalkTensorFile` | Tensör dosyasını girdi girdi yazma ve okuma (büyük modelde bellekte ikinci kopya yok) |
| `ml.SetUnseededHook` | Tohumsuz rastgelelik kullanılınca çağrılır (kontrol noktası bir kez uyarır) |
| `debugger.WatchValue`, `debugger.WatchSummarizer` | Tensör, veri kümesi, parçalayıcı gibi motor değerlerinin İzleme Masası'nda gösterimi |

## Standart kitaplık (`internal/stdlib`)

| Paket | Görevi |
| :-- | :-- |
| `stdlib` | Motorları doğru sırayla yorumlayıcıya kaydeder (`engines.go`) ve `Close` ile kapatır |
| `ml` | Tensör, matris, otomatik türev, sinir ağı işlemleri, iyileştiriciler (AdamW, SGD, Muon), özel türev, karışık hassasiyet, cihaz seçimi ve tensör dosyalarını TR değerlerine ve yerleşiklerine bağlar. Hesabı `compute/tensor` yapar; bu paket hesap yazmaz |
| `tokenizer` | `TR_load_tokenizer` ve parçalayıcı nesnesinin üyeleri (`compute/tokenize` üzerine) |
| `dataset` | `TR_Dataset`, örnekleyici ve sıralı okuyucu yerleşikleri (`compute/dataload` üzerine) |
| `formats` | JSON ve CSV yerleşikleri |
| `files` | `TR_Path` tipi ve çağrısı, listeleme, klasör oluşturma, silme, kopyalama, taşıma, `TR_read_lines`; silme korumaları, bağlantı izlememe |
| `process` | `TR_env` ve `TR_command_line` (seçenekler, `--yardım`, kullanım hatası); gizli adların süzgece kaydı |
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

**Kaynaştırma** (dil tanımı 15.44, `TR_fuse`). Bir fonksiyon iz tensörleriyle (depolamasız) bir kez çalışır; desteklenen işlemler hesap yapmadan bir program kurar (`fuse.go` düğümleri, `fuse_trace.go` kancaları), diğer her işlem açık hata verir. Program her çıktı için tek geçiştir: CPU'da satır satır, kaynaşmamış yolun eleman işlevleriyle (`fuse_cpu.go`); GPU'da çıktı başına tek PTX çekirdeği (`gpu/kernel_fused.go`, `tensor.Fuser`). Her düğüm kendi türüne yuvarlandığından ileri yön kaynaşmamışla bit düzeyinde aynıdır. Geri yön programın simgesel türevidir (`fuse_grad.go`): float32'de, ara değerleri yeniden hesaplayarak; çıktılar `CustomOp` ile tek dış düğüm olarak çizgeye bağlanır.

**Arka uç seçimi.** CPU her zaman vardır. GPU tembel kaydolur (`tensor.RegisterLazyBackend`): sürücü ancak bir cihaz sorulunca yüklenir; sürücü yoksa hiçbir şey kaydolmaz ve her şey CPU'da çalışır. Varsayılan cihaz `tensor.SetDefaultDevice` ile değişir. Yeni bir arka uç `Backend`'i uygular; bir işlemi desteklemiyorsa bunu açıkça söyler. Seçmeli yetenekler ek arayüzlerle sunulur (`MemoryLimiter`, `MultiTensorOps`, `GemmSetter`, `BackwardFinisher`, `Fuser` ...).

**Veri hattı.** Veri kümesi bir klasördür (`.npy` parçaları ve `meta.json`). `dataload.OpenDataset` kümeyi açar, `Sampler` belirlenimci örnekleme yapar (aynı tohum aynı sırayı verir), `SequentialReader` sıralı okur, `Prefetcher` arkada bir sonraki toplu işi hazırlar, `Cache` uzak ya da yavaş kaynağı yerelde tutar. Okuyucu durumu kaydedilip geri yüklenebilir; sürdürülen eğitim kesintisizle aynı veriyi görür.

**Parçalayıcı.** `tokenize.Load` `tokenizer.json`'ı yükler. Sonuç başvuru uygulamasının altın çıktılarıyla birebir aynıdır (`testdata/`); desteklenmeyen bileşen sessizce yok sayılmaz. Girdi güvenilmezdir: düzenli ifade motoru adım bütçesiyle çalışır.

**Dosya biçimleri sözleşmedir.** Diske yazılan biçimler (veri parçaları, meta dosyaları, parçalayıcı tanımı, kontrol noktaları, günlük, profil) Go adlarından bağımsızdır; bir ad değişikliği kullanıcının dosyalarını okunamaz kılmaz.

### GPU arka ucu (`internal/compute/gpu`)

`tensor.Backend`'in NVIDIA uygulamasıdır; otomatik türev ve üst katmanlar cihazdan bağımsızdır.

- **Kayıt:** paket yüklenirken tembel bir arka uç kaydeder; sürücü ancak bir cihaz sorulunca (`TR_devices()`, `.TR_to_device`, `TR_default_device`) yüklenir (`gpu.go`). Sürücü yoksa program çökmez, GPU görünmez olur ve nedeni sorulabilir (`gpu.Status`).
- **Sürücü:** cgo ve CUDA araç takımı gerekmez. Linux'ta `libcuda.so.1` dış bağımlılık olan dinamik yükleyiciyle (`cuda_linux.go`; yalnızca burası `purego` içe aktarır, `internal/layers_test.go` denetler), Windows'ta `nvcuda.dll` yalnızca sistem klasöründen yüklenir (`cuda_windows.go`). **Bedel:** bu yükleyici Linux ikilisini glibc'ye dinamik bağlar, GPU kullanmayan Linux kullanıcıları için de; musl tabanlı dağıtımlarda (Alpine) ikili açılmaz. **Karar:** varsayılan ikili GPU desteklidir; statik ikili isteyen `GOOS=linux CGO_ENABLED=0 go build -tags gpusuz ./cmd/tr` ile derler (`cuda_other.go`). Tam denetim `gpusuz` derlemenin statik kaldığını `tools/checkstatic` ile doğrular.
- **Bağlam ve iş parçacığı:** CUDA bağlamı işletim sistemi iş parçacığına bağlıdır. Her dışa açık yöntem girişte arka ucun kilidini alır, gorutini iş parçacığına kilitler ve cihazın birincil bağlamını bağlar; bütün işler tek akışta sırayla yürür. Ayrı bir GPU gorutinine kanalla iş göndermek yerine bu yol seçildi: kanal gidiş-dönüşü ve gorutin geçişi yok, panikler (TR hataları) çağıranın gorutininde kalır (`dene/yakala`).
- **Çekirdekler:** Go içinden PTX olarak üretilir (`ptx*.go` yazıcı ve komut yardımcıları, `kernel_*.go` kernel aileleri, `backend_*.go` arka uç yöntemleri; taban hedef `sm_52`, `cp.async` kullananlar `sm_80`); sürücü ilk kullanımda karta derler, modüller süreç boyunca önbellekte. Çekirdek imzası her gösterici parametresinin yazılıp yazılmadığını bilir; başlatma yeri okuma/yazma bildirir ve uyuşmazlık iç hatadır. Birleşik kerneller: çapraz entropi (`kernel_ce.go`), dikkat (`kernel_flash*.go`: puan matrisi belleğe yazılmaz, döşemeli çevrimiçi softmax), çok tensörlü iyileştirici/ölçekleme/kareler toplamı (`kernel_multi.go`: bütün parametreler tek başlatmada), sıralama ve birikimli toplam (`kernel_sort.go`). Az çıktılı uzun indirgemeler iki aşamalı ve belirlenimcidir; çıktısını tümüyle yazan işlemler taze gradyanı sıfırlamadan üzerine yazar (`tensor.GemmSetter` vb.). Dikkat yolu `TR_GPU_ATTENTION=birlesik|carpim` ile zorlanabilir (ölçüm için).
- **Matris çarpımı:** makinede NVIDIA cuBLASLt kuruluysa float32 çarpım onunla yapılır (TF32 kapalı; sürücü gibi çalışma anında yüklenir, TR dağıtmaz). Arama sırası ve güvenlik kuralları `gemm_lib_path.go`'dadır (`TR_GPU_CUBLASLT` tam yol; çalışma klasörü hiçbir zaman aranmaz; bkz. [`SECURITY.md`](../SECURITY.md)). Kitaplık bir şekli yapamazsa ya da çağrı hata dönerse panik yerine kendi çekirdeğimize düşülür (`kernel_gemm.go`; `TR_GPU_OWN_GEMM=1` onu zorlar). Tek satırlık çarpımlar (M = 1: çıkarımda bir parça) kitaplığa değil `kernel_gemv.go`'ya gider: ağırlık birleşik erişimle, çok blokla, belirlenimci sırayla okunur (`TR_GPU_GEMV=0` kapatır). Hangi yolun kullanıldığı `gpu.Status()`'ta ve profilde görünür.
- **Bellek:** küçük bloklar için boyut sınıflı, 1 MB üstü için parçalı (best-fit, bölme, birleştirme) havuz (`memory.go`, `memory_blocks.go`). Depolama üç yoldan havuza döner (tek akış sayesinde eşzamanlama gerekmez): belirlenimci bırakma (aşağıda; son sahibi gidince), otomatik türevin ara gradyanları açıkça iade etmesi ve çöp toplayıcının temizliği (kaçan değerler). Çöp toplayıcı cihaz belleğini görmediğinden, son toplamadan bu yana verilip açıkça iade edilmeyen bellek eşiği aşınca toplama başlatılır (`TR_GPU_GC_THRESHOLD` MB ile değişir); her geri yayılımın sonunda `tensor.BackwardFinisher` çağrılır. Havuz canlı/havuzda/ayrılmış/tepe sayaçlarını tutar.
- **Belirlenimci bırakma** (dil tanımı 15.43): `tensor` depolamanın sahiplerini sayar (başlıklar ve geri yayılımın kaydettikleri; `lifetime.go`); yorumlayıcı bir değere artık ulaşılamayacağını bildiğinde onu bırakır (`interp.Owned`, `ownership.go`: çerçeve, bağ sayısı, kaçış); `ml.TensorValue` ikisini bağlar (başlığı saran değer sayısı). Emin olunamayan her değer çöp toplayıcıya kalır; bırakılmış başlığa erişim iç hatadır. `TR_RELEASE=0` kapatır (tanılama).
- **Kayıt ve oynatma (CUDA Graph):** işler sürücünün oluşturduğu akışta yürür (varsayılan akış kaydedilemez). `graph.go` bir iş dizisini kaydeder (RELAXED kip), çizgeye kurar ve tek çağrıyla oynatır; kayıtta ayrılan bellek kayda özel havuzdan gelir (kayıt kapanana kadar başka işe verilmez); kayıttan önce ayrılıp kayda giren depolamaları kayıt tutar (`privatePool.hold`: çöp toplayıcı bırakamaz, açık bırakma kaydın bırakılmasına ertelenir); cihazdan okuma, konaktan yükleme ve eşzamanlama kayıt sırasında kodlu hatadır (`H4065`–`H4068`). Benzetim kaydı da yürütür (işler saklanır, oynatmada sıraya konur). Anlam PyTorch `torch.cuda.graph` ile aynıdır.
- **Profil:** `TR_GPU_PROFIL=1` (ya da `gpu.EnableProfile`) her başlatmayı iki zamanlama olayıyla ölçer: kernel ve aile başına çağrı ve süre, adım başına duvar, GPU, boşluk, konak ve eşzamanlama süreleri; program sonunda özet standart hataya yazılır (`gpu.WriteProfileSummary`, `gpu.Close`). Canlı izleme için Go arayüzü `gpu.KernelSamples` ve `gpu.MemorySamples`'tır (canlı profil bunları okur; aşağıda). Çekirdek ailesi adları (`carpim` ...) profil çıktısında ve araçlarda kullanılan metinlerdir, Go adlarına bağlı değildir.
- **Benzetim:** `sim*.go` PTX altkümesini yorumlayan bir sürücü benzetimidir; GPU sınamaları donanımsız makinede onda koşar. Benzetim gerçek GPU'nun reddedeceği kodu (sınır dışı ve hizasız erişim, dağınık engel, paylaşımlı bellek yarışı) da reddeder; böylece kartsız makinede yakalanan hata gerçek kartta da hatadır. `TR_GPU_SIM=1` ile `tr` de benzetimi `"gpu"` olarak kullanır (geliştirme aracı). Çekirdeklere eklenen her PTX komutu benzeticiye de eklenir. **Bilinen sınırlar** (benzetimde geçip donanımda kalabilecekler): bölgeler havuzun boyut sınıfı kadar ayrılır, ayrıştırıcı `ptxas` kadar katı değildir, bloklar sırayla yürüdüğünden genel bellekte iş parçacıkları arası yarış görünmez, yaklaşık komutlar (`ex2`, `lg2`, `sin`, `cos`) ve `fma` donanımla birkaç ulp farklı olabilir (sınamalar toleranslıdır).
- **Colab:** `tools/gpu/` (model ve mikro kıyas betikleri, `COLAB.md`). Mikro kıyaslar `TR_GPU_MICRO=1` ile koşar.

## Geliştirici araçları

| Paket | Görevi |
| :-- | :-- |
| `cmd/tr` | `tr` komutu: alt komutları ayrıştırır (çalıştır, kabuk, `kontrol`, `TR_cmd_build`, `test`, `TR_cmd_format`, `kodla`, `TR_cmd_show`, `izle`, `TR_cmd_profile`, `TR_cmd_debug`, `TR_cmd_dap`), işi `internal/`'a bırakır; kitaplığı ve Ctrl+C işleyicisini bağlar |
| `cmd/tr/e2e` | Uçtan uca testler (yalnızca testler): ikiliyi bir kez derler, alt süreç olarak çalıştırır; komutlar, çıkış kodları, kesme, kontrol noktasından devam, belgelerdeki kod blokları ve bağlantılar, VS Code eklentisi |
| `devtools/debugger` | Terminal hata ayıklayıcısı ve düzenleyicinin hata ayıklama sunucusu (`TR_cmd_dap`, Debug Adapter Protocol). Yüklenirken `interp.NewDebugger`'ı kurar; gizli değerleri düzenleyiciye göndermez |
| `devtools/formatter` | `TR_cmd_format`: anlamı değiştirmeyen tek standart biçim (ağaç dökümü aynı kalmazsa sonuç reddedilir) |
| `devtools/testrunner` | `tr test`: `TR_test "ad" { ... }` bloklarını yalıtılmış kapsamda koşar, Türkçe raporlar |
| `devtools/profiler` | Canlı profil: `TR_cmd_watch` ve `TR_cmd_profile`'in çekirdeği, arka plan işinin `profile.json`'u (aşağıda) |
| `devtools/codeindex` | TR kodunun dizini: tanım, kullananlar, sınayanlar, etki (`tr dizin ...` ve ileride dil sunucusu); `internal/lang` dışında bağımlılığı yok |
| `editors/vscode` | VS Code eklentisi; renklendirme listeleri kod listesinden üretilir, hata ayıklama ve izleme komutları yukarıdaki sunucu ve profil dosyalarına bağlanır |

### Canlı profil (`internal/devtools/profiler`)

`TR_cmd_watch` (canlı tablo) ve `TR_cmd_profile` (son rapor) aynı çekirdeği kullanır; arka plan işi olarak çalışan program (`TR_IS_KLASORU` iş klasörünü verir) da canlı durumunu buradan `profile.json`'a yazar. Ağ portu açılmaz: dışarıya yalnızca dosya yazılır. Dört kaynaktan okur:

- **Tensör sayaçları** (`internal/compute/tensor/counter*.go`): her zaman açık. Her işlem türü (`tensor.OpKind`), yön (ileri / geri yayılım) ve cihaz (CPU / GPU) için çağrı sayısı (kesin), ayrılan bayt (kesin) ve süre. CPU'da süre uyarlamalı örneklenir: kısa işlemlerde her çağrı p = 1/32 olasılıkla ölçülür ve ölçülen süre 1/p ile ağırlıklanır (Horvitz–Thompson; p yalnızca geçmiş ölçümlere bağlı olduğundan yansız); uzun işlemde (`longOpTicks`) her çağrı ölçülür. GPU işlemlerinin her çağrısı ölçülür. Saat amd64'te değişmez zaman damgası sayacıdır (`counter_clock_amd64.go`), başka yerde monoton saat. Sayaçlar kilitsizdir (yarış denetimli derlemede atomik: `counter_race.go`). Okuma: `tensor.CounterSnapshot`, `tensor.ActiveOp`.
- **Örtük kopya sayaçları** (`internal/compute/tensor/implicit.go`): her zaman açık. Arka ucun her kopyası, tür dönüşümü ve ekleyerek yazması (`Copy`, `CopyStrided`, `AddTo`, `ExpandAdd`, `ScatterAdd`; GPU'da her `copyStrided`) o anda çalışan işleme (üst işlem; yoksa "diğer") bağlanır: çağrı ve eleman kesin, süre yalnızca CPU'da ve en az 4096 elemanlı kopyada ölçülür (GPU'da konak süresi yalnızca başlatmadır). PyTorch profilinde `aten::copy_` / `aten::to` çağıran işlemin altında görünür; burada tablo düzdür, üst işlem anahtardır. İşlemin kendisi olan kopyalar (`.kopya()`, `.TR_to_dtype`) işlem tablosundadır, örtük tabloya alınmaz. Okuma: `tensor.ImplicitSnapshot`.
- **Yorumlayıcı örneklemesi** (`sampler.go`): yalnızca `TR_cmd_watch` ve `TR_cmd_profile` altında. Yorumlayıcının hata ayıklayıcı yuvasına (`VM.Debugger`) bağlanır; yuva boşken yorumlayıcının her deyimdeki maliyeti bir nil denetimidir. Açıkken her deyim konumunu (fonksiyon, satır) çağrı derinliğinin yuvasına yazar; ayrı bir gorutin 1 ms'de bir o anki konuma geçen süreyi ve o an bir tensör işlemi çalışıyorsa tensör payını ekler. `TR_breakpoint` profil altında yok sayılır.
- **Geri hesabın satırı** (`internal/compute/tensor/position.go`): örnekleme açıkken örnekleyici tensör paketine konum kaynağı olur (`tensor.SetPositionSource`); her düğüm oluştuğu satırı taşır, geri hesabı çalışırken o satır etkin geri konumdur (`tensor.BackwardPosition`) ve örnekleyici o ana düşen süreyi `TR_backward(...)` satırına değil, düğümü üreten ileri satıra yazar (satırın `geri_payı`). PyTorch'ta aynı eşleme `emit_nvtx`'in sıra numarasıyla yapılır (ileri `seq=N`, geri `stashed seq=N`); burada eşleme düğümün kendisindedir. Dış (özel) geri hesap TR kodu çalıştırdığından kendi satırlarında sayılır. Kapalıyken maliyet düğüm başına bir atomik gösterici okumasıdır.
- **Adım sınırı** (`logging.SetStepHook`): ms/adım ve adım başına işlem dağılımı buradan. Program `TR_Progress(...)` kullanmıyorsa sınır iyileştiricinin adımıdır (`tensor.SetOptimizerStepHook`; AdamW/SGD `.TR_step()` bitince, birden çok iyileştiricide ilk adım atan): PyTorch profilinde adım `prof.step()`, JAX'ta `StepTraceAnnotation` ile işaretlenir, TR'de açık işaret `TR_Progress(...)`, iyileştirici yedektir.
- **Adım evreleri** (`internal/compute/tensor/phase.go`): her adım sınırında önceki sınırdan bu yana geçen süre üçe bölünür: geri (geri yayılımların toplam süresi, sayaç saatiyle; iç içe geri yayılım bir kez), sonrası (son geri yayılımın bitişinden sınıra: iyileştirici, kırpma, günlük) ve ileri (kalan). Evreler kırpılmaz; toplamları adım süresidir. Konak süresidir: GPU'da işler eşzamansızdır, eşzamanlamada beklenen süre beklediği evreye düşer.
- **GPU arka ucunun profili** (`gpu.KernelSamples`, `gpu.MemorySamples`): `TR_cmd_watch` ve `TR_cmd_profile` kernel ölçümünü açar (`gpu.EnableProfile`; `TR_GPU_PROFIL=1` ile zaten açıksa dokunmaz); program bitince, motorlar kapanmadan önce (`Profile.BeforeClose`, komut satırının `kapanmadanOnce` işleri) cihazı eşzamanlayıp arka ucun raporunu alır. Arka plan işinin profili ölçümü açmaz; GPU belleği yine okunur. Okumalar arka ucu beklemez. Ölçüm açıkken her çekirdek kaydı onu başlatan tensör işlemini ve TR konumunu taşır (`tensor.CurrentOwner`, `gpu.OwnerSamples`; geri yöndeki çekirdekler düğümü üreten ileri satırda): rapor GPU süresini işleme ve satıra göre ayrı tablolarda verir. Konakta ölçülen süre GPU'da yalnızca başlatma ve beklemedir; GPU'yu meşgul eden satır buradan okunur. PyTorch'ta aynı bağı CUPTI'nin `correlationId`'si kurar. Profil adım sınırını GPU arka ucuna da verir (`gpu.SetExternalSteps`, `gpu.MarkStep`): iyileştiricinin kendi işareti adımda birden çok iyileştiriciyle birden çok kez düşerdi (MuonVeAdamW'de 2).

Profil saniyede bir anlık görüntü alır (`snapshot.go`), son 5 saniyenin farklarından pencere ölçülerini kurar (canlı tablonun hızları), dinleyiciye (canlı tablo) verir ve iş klasörü varsa dosyaya yazar. Program bitince (hata, `çıkış` ya da Ctrl+C ile bitse de) son raporu kurar. Sayaçlar süreç genelidir; profil başlangıcındaki değerler çıkarılır.

**Canlı tablo** (`screen.go`): standart hata bir terminalse tablo ekranın başından denetim dizileriyle yeniden çizilir (Windows'ta konsolun sanal terminal kipi açılır; `live_windows.go`). Programın standart çıkışı ve hatası bir boruya alınır (`capture.go`): tamamı bir dosyaya yazılır, son satırları tablonun altında görünür, program bitince yeniden yazılır; boru terminal olmadığından ilerleme çubuğu düz satır kipine geçer. Terminal değilse çıktı olduğu gibi akar, 10 saniyede bir tek satırlık özet eklenir.

**Dosya biçimi** (`snapshot.go`): `profile.json` (canlı, iş klasöründe) ve `speed.json` / `tr profil --json` (son rapor) aynı yapıdadır. Anahtarlar ve sabit değerler (`live`, `forward` ...) dosya biçiminin parçasıdır: dilden bağımsız İngilizce tanımlayıcılardır, çevrilmez (dil tanımı 17.3); işlemlerin `code` alanı değişmez anahtardır, `name` insana gösterilen addır. Süreler saniye, baytlar bayt, paylar 0..1'dir. Yazma atomiktir (aynı klasörde geçici dosya + yeniden adlandırma); yazma başarısızsa sonraki saniyede yeniden denenir, program hiçbir durumda durmaz. Sürüm 6 (sürüm 1'e göre `tensor` altında `implicit` (2), satırların `backward_share` (3), adımın `source` ve evreleri (4), `gpu` altında `ops` ve `lines` (5) eklendi; 6'da anahtarlar Türkçeden İngilizce tanımlayıcılara geçti):

```jsonc
{
  "format": "tr-profile", "version": 6, "kind": "live",         // "live" ya da "report"
  "program": "egit.tr", "start": "2026-10-19T08:49:12.511+03:00",
  "time": "2026-10-19T08:49:24.511+03:00", "seconds": 12.0, "done": false,
  "exit_code": 0,                                          // yalnızca bitince
  "step": {"count": 715, "total": 1500, "all_steps": 715,   // ilerleme yoksa null; count ve total
    "ms_per_step": 16.7, "steps_per_second": 59.7,    // son çubuğun, all_steps bütün çubukların (epoch'lar) adımı
    "source": "progress_bar", "forward_ms": 5.1, "backward_ms": 9.8, "after_ms": 1.8},  // source: progress_bar | optimizer
  "tensor": {"seconds": 11.4, "share": 0.95, "calls": 772761, "bytes": 7642211100,
    "ops": [{"code": "matmul", "name": "matris çarpımı (@)", "direction": "forward",
      "device": "cpu", "calls": 160966, "measured": 160529, "seconds": 4.99, "share": 0.44,
      "bytes": 2044092928, "ms_per_step": 6.99}],               // süreye göre sıralı, en çok 30
    "implicit": [{"code": "strided_copy", "parent_code": "matmul", "name": "adımlı kopya",
      "parent_name": "matris çarpımı (@)", "direction": "forward", "device": "cpu", "calls": 45,
      "elements": 17940480, "measured": 45, "seconds": 0.022}]}, // elemana göre sıralı, en çok 30
  "gpu": {"used": true, "measured": true, "note": "", "seconds": 0.088,
    "families": [{"name": "carpim", "calls": 40, "seconds": 0.069, "share": 0.78}],
    "kernels": [{"name": "carpim_64_64_16_4_4_false_false_true", "calls": 20, "seconds": 0.040, "share": 0.46}],
    "memory": [{"device": 0, "live_bytes": 109568, "allocated_bytes": 114176, "pooled_bytes": 4608,
      "peak_live_bytes": 113664, "peak_allocated_bytes": 114176, "gc_collections": 0}]},
  "sampling": {"interval_ms": 1, "samples": 11800, "seconds": 11.8, "tensor_share": 0.95,  // kapalıysa null
    "lines": [{"file": "examples/tr/41_gpt.tr", "line": 49, "function": "",
      "text": "değişken y = x @ kendi.ağırlık", "seconds": 4.6, "share": 0.39, "tensor_share": 0.99,
      "backward_share": 0.64}],
    "functions": [{"name": "dikleştir", "seconds": 3.9, "share": 0.33}]},
  "memory": {"go_heap_bytes": 7864320, "go_sys_bytes": 44564480, "cpu_tensor_alloc_bytes": 7642211100},
  "gc": {"count": 423, "pause_seconds": 0.113},
  "window": {"seconds": 5.0, "step": 292, "ms_per_step": 17.1, "steps_per_second": 58.3,   // yalnızca canlıda
    "tensor_share": 0.96, "ops": [], "lines": [], "gc_count": 165,
    "pause_seconds": 0.055, "cpu_tensor_alloc_bytes": 2980000000,
    "gpu_seconds": 0, "gpu_share": 0, "gpu_families": []}
}
```

Satırın `file`ı ana programın dışındaysa (aktarılan modül) boştur: örnek yalnızca fonksiyon adını ve satırı bilir. `measured`, ölçülen çağrı sayısıdır (`calls`tan küçükse süre örneklenmiştir). Canlı CPU tensör baytı ayrıca izlenmez (CPU tensörleri Go yığınındadır); `cpu_tensor_alloc_bytes` ayırma akışıdır.

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
| `lib/` | TR ile yazılı standart kitaplık, ikiliye gömülü (`TR_import "tr/katman"`, `TR_import "tr/iyilestirici"`; testleri `tr test lib`) |
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
