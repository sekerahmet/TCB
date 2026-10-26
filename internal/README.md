# internal

TR'nin bütün Go kodu buradadır. Bu belge, buraya kod yazan herkesin (insan ya da ajan) uyduğu kuralların tek kaynağıdır. Her kuralın yanında onu koruyan test yazılıdır: kural çiğnenirse `go test ./...` kalır. Paketlerin kendi kuralları kendi `README.md`'lerindedir; depo düzeyindeki süreç (yeni ad isteği, tam denetim, ölçüm) [`docs/development.md`](../docs/development.md)'dedir. Yeni bir fonksiyon, yerleşik, metot, ileti, paket ya da dil kuralı eklerken adım adım tarifler: [`docs/recipes.md`](../docs/recipes.md).

İlke: **küçük küçük yönetmek her zaman daha kolaydır.** Bir paket tek görevi, bir dosya tek konuyu, bir fonksiyon tek işi anlatır; okuyan aradığını adından bulur. Yeni bir kural yazıya geçince ona bir test bağlanır; yalnızca yazılı kalan kural zamanla aşınır.

## Katmanlar

| Katman | Klasör | Görevi | İçe aktarabileceği |
| :-- | :-- | :-- | :-- |
| Dil çekirdeği | `lang/` | Kaynak metinden çalışan programa: sözcük çözümleme, ayrıştırma, tip denetimi, yorumlayıcı, kod listesi, iletiler | yalnızca `lang/` |
| Hesap çekirdeği | `compute/` | Tensörler, otomatik türev, CPU ve GPU arka uçları, veri hattı, parçalayıcı | `compute/` ve ileti kataloğu `lang/messages` |
| Standart kitaplık | `stdlib/` | Dili hesap çekirdeğine ve dış dünyaya bağlayan yerleşikler (ml, dosyalar, günlük, kontrol noktası ...) | `lang/`, `compute/`, `stdlib/` |
| Geliştirici araçları | `devtools/` | Hata ayıklayıcı, profilci, biçimlendirici, test çalıştırıcı, kod dizini, Go modül yükleyici (`tools/impact` ve `tools/codegraph` ortak kullanır) | `lang/`, `compute/`, `stdlib/`, `devtools/` |
| Test yardımcısı | `reporoot/` | Testlerin depo kökünü bulması | hiçbir iç paket |
| Test yardımcısı | `testkit/`, `testkit/trrun/` | Paketler arası ortak test yardımcıları (dosya yazma, ikili çalıştırma, TR programı çalıştırma) | `testkit`: yalnızca `reporoot`; `trrun`: `lang/` |

`cmd/tr` hepsini, `tools/` bütün `internal/`'ı ve araçların ortak yardımcılarını (`tools/internal/`: `repodir`) kullanabilir. Dil çekirdeği ve hesap çekirdeği birbirini bilmez: dil, tensörü kancalarla tanır (`stdlib/ml`). Tek istisna ileti kataloğudur: `lang/messages` yalnızca Go standart kitaplığını içe aktaran en alt pakettir, hesap çekirdeği de iletilerini onun kodlarıyla verir.

Dil çekirdeğinin içinde de bir sıra vardır; bir paket yalnızca listede kendinden öncekileri içe aktarır:

```
messages → turkish → codes → hint → lexer → ast → parser → checker → source → interp
```

Dış bağımlılık izne bağlıdır: bugün yalnızca `purego` (GPU sürücüsünü cgo'suz yüklemek için, `compute/gpu`).

Denetim: `layers_test.go` (`TestLayerRule`, `TestLangOrder`, `TestExternalDependencies`). Yeni bir klasör açılınca oradaki tablolar ve [`docs/architecture.md`](../docs/architecture.md) güncellenir.

## Klasörler, dosyalar, fonksiyonlar

- **Adlar küçük harf ASCII'dir:** harf, rakam ve `_` (`compiler_loop.go`, `builtin_reduce.go`). Go'nun ekleri olduğu gibi kullanılır (`_test`, `_windows`, `_unix`, `_amd64`). Denetim: `TestLayoutFileNames`.
- **Dosya başına tek konu.** Konusu birden çok olan dosya bölünür: `compiler_stmt.go`, `compiler_loop.go`, `compiler_call.go`.
- **Dosya en çok ~600 satır** (testler dahil, üretilmiş dosyalar hariç). Denetim: `TestLayoutFileSize`.
- **Fonksiyon en çok ~80 satır** (`func` satırından kapanış parantezine; test fonksiyonları hariç). Uzun bir `switch` dallarına, uzun bir kayıt listesi tabloya, uzun bir akış adımlarına bölünür. Denetim: `TestLayoutFuncSize`.
- **İstisnalar** gerekçeleriyle `layout_exceptions_test.go`'dadır ve **yalnızca küçülür**: sınırın altına inen kayıt silinir, yeni kayıt eklenmez, tavan yükseltilmez. Sıcak yoldaki bir fonksiyon (yorumlayıcı döngüsü, hesap çekirdeği) ancak önce/sonra hız ölçümüyle bölünür.
- **Taşıma ile davranış değişikliği ayrı commit'lerdir.** Bölünen dosyada fonksiyon gövdeleri değiştirilmez, yalnızca taşınır; değişiklik geçmişte izlenebilir kalır.

## Paket belgesi ve README

Her paketin tek bir paket belgesi vardır: görevini söyler ve katmanını bildiren satırla biter.

```go
// Package formats, metin veri biçimlerinin (JSON ve CSV) TR yerleşiklerini içerir: json_çöz,
// json_metin ve csv_oku.
//
// Layer: standard library (internal/stdlib); dil ve hesap çekirdeklerini içe aktarır, araçları bilmez.
package formats
```

Denetim: `TestLayoutPackageDoc`.

Her paketin bir `README.md`'si vardır. Türkçedir ve yalnızca **kalıcı** bilgiyi tutar: görev, standartlar, testler. Yapılacak işler, geçmiş ve ölçüm sonuçları README'ye girmez; onların yeri yol haritası ve commit geçmişidir.

```markdown
# formats

JSON ve CSV yerleşikleri. (bir iki cümle: görev)

## Standartlar

**Kural adı.** Kural ve gerekçesi; gerekiyorsa küçük bir örnek.

## Testler

Birim testleri sınadıkları dosyanın adını taşır. Birden çok dosyayı kesen davranış testleri:

- `behavior_konu_test.go`: ne sınadığı
```

## Adlar ve diller

- **Kod tabanı İngilizcedir:** klasör, dosya, paket, Go adları (dışa açık ve iç), test ve kıyas adları. Geliştirici belgeleri, betikler, test verisi ve eklenti dosyaları da dahildir; kullanıcı katmanı istisnadır: kullanıcı belgeleri (`docs/spec`, `docs/guide`) Türkçe yazılır, `examples/` ve `*.tr` dosyalarının adları bugün denetimin dışındadır. `.tr` dosyalarının içeriği diskte kodludur (dilden bağımsız kimlikler); dil dosyaları (`tr.json`, `en.json`) her yazım dilinin adlarını taşır, hiçbirinin ayrıcalığı yoktur. Denetim: `TestLayoutEnglishFileNames`.
- **Go yorumları Türkçedir.** Başka ürün, şirket ya da kitaplık adı yoruma yazılmaz.
- **Go adları Go biçimindedir (MixedCaps):** C tarzı BÜYÜK_HARFLİ ad yazılmaz; sabit önekleri `Tok` (belirteç türü), `Val` (değer tipi), `prec` (öncelik). Dış API adları (CUDA, Windows) ve ortam değişkeni adları metin olarak olduğu gibi kalır. Denetim: `TestGoNamesMixedCaps`; istisna listesi `internal/go_names_test.go` içindedir ve yalnızca küçülür.
- **Go metninde kullanıcıya görünen Türkçe yoktur:** ad ad kodudur, ileti ileti kodudur; geliştiriciye giden iç hata ayrıntısı ve araç çıktısı İngilizcedir. Türkçe harfli metin sabiti yalnızca izinli yerdedir: `internal/lang/turkish` (dilbilgisi tabloları), yapı alanı etiketleri ve gerekçeli `dosya:metin` kayıtları (veri biçimi anahtarları, Türkçe belge üreten aracın içeriği). Denetim: `TestNoTurkishLiterals`; izin listesi `internal/turkish_literals_test.go` içindedir ve yalnızca küçülür.
- **Kullanıcıya görünen adlar Go metnine yazılmaz.** Anahtar kelime, yerleşik, tip, üye ve adlı argüman adları sabit kodlara bağlıdır (`lang/codes`, `tr.json`). Go kodu kodu yazar, ad dil dosyasından gelir:

  ```go
  fn := codes.Name(codes.FnReadCsv)                  // "csv_oku"
  return interp.ArgCountError(codes.FnReadCsv, "1", len(args))
  ```

  Adı değiştirmek yalnızca dil dosyasının satırını değiştirir. Yeni bir ad (yeni kod) gerekiyorsa kendiniz eklemeyin: [`docs/development.md`](../docs/development.md#yeni-kod-ad-isteği). Denetim: `tools/gen/codeconsts` (`TestNoBareNames`: Go metnindeki çıplak ad).

## Kullanıcı iletileri

Kullanıcıya giden her ileti (hata, uyarı, bilgi) de adlar gibi sabit bir koda bağlıdır: `H` ve dört basamak, binler basamağı alanı gösterir. Ayrıntı: [`lang/messages/README.md`](lang/messages/README.md).

| Alan | Kodlar |
| :-- | :-- |
| sözdizimi, tip denetimi, yorumlayıcı ve ortak yardımcılar | `H0`, `H1`, `H2` |
| tensör ve CPU, GPU | `H3`, `H4` |
| veri ve parçalayıcı, ml | `H5`, `H6` |
| standart kitaplığın geri kalanı, araçlar | `H7`, `H8` |
| iç hata (yalnızca TR'nin kendi hatasıyla oluşan durum) | `H9` |

```go
// definitions.json: "H7003": { "param": ["fn", "path"] }
// tr.json:          "H7003": "{fn}: {path|q} bir klasör; CSV dosyasının yolunu verin"
return "", messages.New(messages.H7003, fn, p.arg)          // hata
m.WriteOutput(m.Err, messages.Text(kod, değerler...))     // hata olmayan çıktı (uyarı, not)
return nil, messages.Internal("vector width must be 2 or 4") // iç hata: ayrıntı İngilizce
```

- **Cümle parçalardan kurulmaz:** bir ileti koduna başka bir ileti kodunun metni (yüklem, nesne, "bir değişkene") argüman geçirilmez; her anlam kendi tam şablonudur, değişen yer şablonda `select` ile seçilir (Go yalnızca anahtar verir). Denetim: `TestMessageArgsAreNotMessageFragments`.
- Metin dil dosyasındadır; parametreler adlıdır (`{fn}`), kullanıcı verisi tırnakla yazılır (`{path|q}`: denetim karakterleri kaçırılır).
- Ortak kalıplar yeniden yazılmaz: `H2000` (`{fn}: {error}`), dosya hataları `turkish.FileError(fn, yol, messages.H2100, err)` (işlem bir koddur ve kendi tam cümlesidir: `{path|q} okunamadı: {reason}`; `fn` varsa `H2000` sarar), özel neden `turkish.NewReason(kod, ...)`, adlı argüman `bind.CheckNamed`.
- **Ortam değişkeni adları ASCII'dir:** `TR_` öneki, büyük harf, rakam ve `_` (`TR_DIL`, `TR_GPU_OWN_GEMM`, `TR_IS_KLASORU`); Türkçe harfli ad bazı kabuklarda yazılamaz. `TR_` öneki TR'nin kendi değişkenlerine ayrılmıştır, örneklerde kullanıcı değişkeni olarak kullanılmaz. Denetim: `TestLayoutEnvVarNames`.
- **İleti olmayan metin** koda bağlanmaz: veri biçimlerinin anahtarları (günlükteki `"step"`, `state.json` alanları; dilden bağımsız İngilizce tanımlayıcılardır, dil tanımı 17.3), dosya adları, protokol alanları.
- Geçirilmiş pakette `fmt.Errorf` ve `errors.New` yoktur. Denetim: `tools/gen/msgconsts` (her çağrıda değer sayısı tanımla aynı, kullanılmayan kod yok, `migrated` listesindeki paketlerde yasak çağrı yok). Geçiş sürüyor; sırası [`docs/roadmap.md`](../docs/roadmap.md)'de.

## Hatalar ve panikler

- **Hata döndürülür, yazdırılmaz.** Yerleşik bir hatayı `error` olarak döndürür; satır öneki (`Satır 12: ...`) ve kod çerçevesi yorumlayıcıda eklenir (`interp.LineError`). Hata `dene/yakala` ile yakalanabilir olmalıdır; program çökmez.
- **Hesap çekirdeği panikle bildirir, sınırda hataya çevrilir.** `compute` şekil ve sınır hatalarında panikler; `stdlib` her hesap çağrısını `bind.Recover` (ml'de `safe`) ile sarar. Kesme (Ctrl+C) paniği hata değildir, yutulmaz (`interp.IsInterrupt`).
- **İşletim sisteminin İngilizce metni kullanıcıya gitmez:** dosya hataları `turkish.FileError` ile anlatılır, özgün hata `Unwrap` ile korunur (`errors.Is(err, fs.ErrNotExist)` çalışır).
- **Sarma zinciri korunur:** hata sarılırken neden kaybolmaz (`%w` ya da ileti parametresi olarak hata); testler kodu zincirde arar (`messages.Has`).

## Yorumlayıcı kilidi ve eşzamanlılık

- TR görevleri (`eşzamanlı`) tek bir yorumlayıcı kilidi altında çalışır; TR değerlerine yalnızca kilit tutulurken dokunulur.
- **Uzun iş kilidi bırakarak yapılır:** disk ve ağ okuma, bekleme, ağır hesap `m.Blocking(func() { ... })` içinde. İçeride TR değeri okunmaz, kurulmaz; değerler kilit geri alınınca kurulur:

  ```go
  var records [][]string
  vm.Blocking(func() { records, err = parseCSV(input) }) // TR değerine dokunmaz
  return csvValues(records)                              // değerler kilit altında
  ```

  İstisna: işin kendisi TR değerlerinin tutarlı bir anlık görüntüsünü gerektiriyorsa kilit tutulur ve bu, işin kodunda gerekçesiyle yazılır. Bugünkü tek örnek kontrol noktasının yazılmasıdır (`stdlib/checkpoint/write.go`): tensörler tek tek konağa indirilip yazılır (modelin ikinci bir kopyası bellekte toplanmaz) ve kayıt sürerken başka bir görev modeli değiştiremez.

- Paylaşılan Go durumu kilitle (`sync.Mutex`) ya da atomik alanla korunur. Yarış dedektörü tam denetimin `yaris` kipinde koşar.

## Sınırlar ve güvenilmeyen girdi

- **Koleksiyon sınırı:** kullanıcının boyutunu belirlediği her ayırma `interp.MaxCollectionSize` ile denetlenir. Sıfır uzunluklu eksen sınırda 1 sayılır (`[0, 10⁹]` veri tutmaz ama `.değer` 10⁹ liste ister).
- **Döngüsel değer:** bir değeri dış biçime yazan ya da gezen kod kendini içeren listeyi `interp.ContainerWalker` ile yakalar; sonsuz özyinelemeye girmez.
- **Güvenilmeyen girdiler** (tensör dosyası, kontrol noktası, `.npy`, `tokenizer.json`, CSV/JSON metni) bozuk ya da kötü niyetli olabilir: çökertmemeli, sonsuz döngüye sokmamalı, sınırsız bellek ayırmamalı; açık bir hata vermelidir. Her birinin bir bulanıklaştırma testi (`Fuzz...`) vardır. Liste ve korumalar: [`SECURITY.md`](../SECURITY.md).
- TR programları güvenilmeyen girdi değildir; yorumlayıcı bir kum havuzu değildir.

## Kancalar

Yorumlayıcıya bir arayüzle bağlanan her tip (indeksleme, işleçler, yineleme, üyeler, izleme paneli ...) derleme anı denetimi taşır. Yorumlayıcı kancaları tip denetimiyle bulur; adı değişen bir metot bu denetim olmadan kancayı sessizce koparır:

```go
var (
	_ interp.Indexable = (*TensorValue)(nil)
	_ interp.Iterable  = (*TensorValue)(nil)
)
```

## Üretilmiş dosyalar

Kod listesinden ve ileti kataloğundan üretilen dosyalar elle düzenlenmez ve Go'nun standart başlığını taşır:

```go
// Code generated by go run ./tools/gen/msgconsts; DO NOT EDIT.
```

Üreteçler `tools/gen/` altındadır ve `-check` bayrağıyla güncelliği sınanır (tam denetim her üreteci böyle koşar). Kaynağı değiştirince üretin: `go generate ./internal/lang/codes ./internal/lang/messages`. Denetim: `TestLayoutGeneratedHeader`, tam denetimin "Üretilmiş dosyalar güncel mi" adımı.

## Testler

Üç tür test dosyası vardır; okuyan bir test dosyasının neyi sınadığını adından bilir.

| Tür | Ad | Kural |
| :-- | :-- | :-- |
| Birim testi | `x_test.go` (ve `x_fuzz_test.go`, `x_bench_test.go`) | yalnızca `x.go`'yu sınar ve yanında durur; `x` platforma göre bölünmüş olabilir (`x_linux.go`) |
| Davranış testi | `behavior_<konu>_test.go` | birden çok dosyayı kesen bir davranışı sınar; paket README'sinin `## Testler` bölümünde "- \`behavior_<konu>_test.go\`: ne sınadığı" satırıyla anılır |
| Ortak yardımcılar | `helpers_test.go` | `TestMain` ve paketin testlerinin paylaştığı işlevler |

Denetim: `TestTestFileMatchesSource` (birim testinin kaynağı var, her davranış testi README'de anılmış, README'de anılan her dosya diskte). Kural `lang/`, `compute/`, `stdlib/`, `devtools/` ve `cmd/`'de uygulanır.

- **Test konusuyla adlandırılır, onu doğuran işle değil:** `review_fixes_test.go` kalıcı bir konu değildir; düzeltmenin testi düzeltilen dosyanın testine gider.
- **Bir düzeltmenin testi önce kalır, sonra geçer:** hata yeniden üretilmeden düzeltilmez.
- **Testler iletinin metnine değil koduna bakar:** `messages.Has(err, messages.H7003)`. Metin yalnızca gerçekten biçimi sınayan yerde (ilerleme çubuğu satırı, rapor düzeni) denetlenir.
- **Depo kökü** `reporoot.Root()` ile bulunur; testler `../../` gibi göreli yol yazmaz.
- **Dış ağ yoktur;** sunucular yalnızca `127.0.0.1`'e bağlanır; açılan her süreç ve geçici dosya kapatılır, silinir.
- **Uzun testler** `testing.Short()` ile atlanır (tam denetim `-short` koşar, `uzun` kipi hepsini koşar); bir testi hızlandırmak için sınırlar test içinde küçültülebilir.
- **Testler kararlıdır:** sonucu tekrar sayısına, çalışma sırasına, süreç geneli duruma ya da çöp toplayıcının zamanlamasına bağlı değildir. Bir programa ya da göreve ait durum paket değişkeninde değil yorumlayıcıda tutulur (`VM.ProgramValue`, `VM.TaskValue`): aynı süreçte birden çok program çalışır (`tr test`'in dosyaları, düzenleyici oturumu, testler). Denetim: `tools/full_check.sh --kip=zorlama` (iki tekrar ve karışık sıra, sonra `GOGC=1`; otomatik sınamada haftalık, `.github/workflows/zorlama.yml`).
- **Süre ölçen test mutlak süreye değil büyümeye bakar:** duvar saati tavanı yüklü makinede aşılır (bütün takım paralel koşarken tek bir sürenin 90 katına uzadığı ölçüldü). Doğrusallık `testkit.ScalesLinearly` ile sınanır (n ve 4n aynı yükte, oran en çok 8); iş işlediği birimi ve hatasını verir, süreler ancak iki boy da hatasız ve 3,5–4,5 kat birimle bittiyse karşılaştırılır (yarıda kalan iş "doğrusal" görünmesin). Bir beklemenin erken bittiğini gösteren tavan, ayırt ettiği sürenin yarısı kadar geniş seçilir.
- Dilin kendi davranışı Go testinde değil, `tests/*_test.tr` programlarında sınanır (her test bir dil kuralı numarasıyla başlar).

## Değişiklikler

- Bir değişiklik `bash tools/full_check.sh` geçince biter; otomatik sınama da aynı betiği çalıştırır. İş sırasında yalnızca değişen paketlerin testleri koşulur.
- Commit mesajları İngilizcedir: ilk satır `paket: ne değişti`, gövdede neden.
- Sıcak yola dokunan değişiklik önce ve sonra ölçülür; kazandırmayan "iyileştirme" geri alınır ([`docs/development.md`](../docs/development.md#ölçüm-disiplini)).
- Bir adı ya da kancayı değiştirmeden önce nerede kullanıldığını koddan sorun: `go run ./tools/impact codes.Name`.
- Durağan çözümleme tam denetimdedir: staticcheck'in varsayılan denetimleri ve ölü kod (deadcode). Bilerek yazılmış bir kalıp (testin ürettiği panik, yalnızca assembly'nin okuduğu değişken) satırının üstünde gerekçeli `//lint:ignore DENETİM gerekçe` ile işaretlenir; gerekçesiz işaret ve yapılandırmayla kapatılan denetim yoktur. Hiçbir programdan ve testten çağrılmayan fonksiyon tutulmaz ("ileride lazım olur" diye kod bırakılmaz). Araç sürümleri `tools/full_check.sh`'ın başındadır; sürüm yükseltilince yeni bulgular aynı commit'te düzeltilir.
- Kodun çağırdığı yollarda bilinen güvenlik açığı bulunmaz: govulncheck her gönderimde ve haftada bir üç işletim sistemi için koşar (`.github/workflows/vuln.yml`). Açık standart kitaplıktaysa `go.mod`'daki `toolchain` satırı düzeltmeyi içeren, desteklenen bir sürüme yükseltilir.

## Kural → test

| Kural | Test |
| :-- | :-- |
| Katmanlar, dil içi sıra, dış bağımlılıklar | `internal/layers_test.go` |
| Dosya ve klasör adı, paket belgesi, üretilmiş dosya başlığı | `internal/layout_test.go` |
| Geliştirici dosya ve klasör adları İngilizce (kullanıcı katmanı hariç) | `internal/layout_test.go` (`TestLayoutEnglishFileNames`) |
| Mimari belgesindeki Go adları depoda var | `internal/architecture_names_test.go` (`TestArchitectureGoNamesExist`) |
| Ortam değişkeni adları | `internal/layout_test.go` (`TestLayoutEnvVarNames`) |
| Go adları MixedCaps (C tarzı BÜYÜK_HARFLİ ad yok) | `internal/go_names_test.go` (`TestGoNamesMixedCaps`) |
| Go metninde kullanıcıya görünen Türkçe metin yok | `internal/turkish_literals_test.go` (`TestNoTurkishLiterals`) |
| Dosya ve fonksiyon boyu | `internal/layout_test.go`, istisnalar `internal/layout_exceptions_test.go` |
| Test dosyası düzeni | `internal/layout_test.go` (`TestTestFileMatchesSource`) |
| README'lerdeki `Denetim:` ve `Kural`/`Test` tablosu satırlarının andığı test işlevleri ve dosyaları depoda var | `internal/rule_tests_test.go` (`TestRuleTestsExist`) |
| Kullanıcıya görünen adlar koddan | `tools/gen/codeconsts` (`TestNoBareNames`) |
| Kodlar kimlikle anılır, eski numaralı biçim kalmaz (eşleme `docs/code_identities.md`) | `internal/old_codes_test.go` (`TestNoOldCodeForm`) |
| İleti kodları | `tools/gen/msgconsts` (`TestMessageUsage`) |
| Üretilmiş dosyalar güncel | `tools/gen/*` `-check` (tam denetim) |
| staticcheck bulgusu yok, çağrılmayan fonksiyon yok | `tools/full_check.sh` (staticcheck, deadcode) |
| Çağrılan yollarda bilinen güvenlik açığı yok | `.github/workflows/vuln.yml` (govulncheck) |
| Testler tekrar sayısından, sıradan, süreç geneli durumdan ve çöp toplayıcıdan bağımsız | `.github/workflows/zorlama.yml` (`tools/full_check.sh --kip=zorlama`) |
| Kod listesindeki her örnek çalışır | `internal/lang/codes`, `internal/stdlib` (`definition_test.go`, `behavior_definitions_test.go`) |
| Dil kuralları | `tests/*_test.tr` (`tr test tests`) |
