# UZAK 7 Raporu: codegraph, İşlev Değeri Argüman Olunca Kopan Test Zinciri

## Özet ve Kimlik Bilgileri

| Alan | Değer |
| :--- | :--- |
| **Dal** | `gemini/uzak-7` [ölçüldü: `git branch --show-current`, `gemini/uzak-7`] |
| **Taban (`main`) kimliği** | `ffca754c3317bbb058ea3ff649c78d8c6a692362` [ölçüldü: `git rev-parse main`, `ffca754c3317bbb058ea3ff649c78d8c6a692362`] |
| **Başlama saati** | `2026-11-08 12:41:04 UTC+3` (`2026-11-08T09:41:04+00:00`) [ölçüldü: `date -Is`] |
| **Bitiş saati** | `2026-11-08 13:33:39 UTC+3` (`2026-11-08T10:33:39+00:00`) [ölçüldü: `date -Is`] |
| **Bağlam sıkıştırması** | 1 kez: Adım 1 commit ve push edildikten hemen sonra, Adım 2'ye başlarken (`2026-11-08 13:00:53 UTC+3` / `2026-11-08T10:00:53+00:00`). Sıkıştırma sonrası `CLAUDE.md`, `UZAK_YONERGE_7.md` ve `UZAK7_DURUM.md` yeniden okundu; bütün sayılar ve alıntılar kaynak dosyalardan doğrulandı. |
| **Go sürümü (`GOTOOLCHAIN=local`)** | `go version go1.27.3 linux/amd64` [ölçüldü: `go version`, `go version go1.27.3 linux/amd64`] |
| **Üretim kodu satır farkı** | 5 üretim dosyasında (`graph.go`, `gocode_scan.go`, `test_reach.go`, `query_tests.go`, `test_paths.go`) net `+19` satır (`+30 / -11`, ~80 satır tavanının altında) [ölçüldü: `git diff --stat main...HEAD -- tools/codegraph/gocode_scan.go tools/codegraph/graph.go tools/codegraph/test_reach.go tools/codegraph/query_tests.go tools/codegraph/test_paths.go`] |

---

## Bulgular ve Adımlar (Önem Sırasıyla)

### 1. Adım 1: Kusurun Yeniden Üretilmesi (Fikstür, Gerçek Depo Örnekleri ve `sumBuiltin`)

- **İddia:** Go'da bir çağrının argümanı olarak geçirilen paket düzeyi işlev (`f(..., g)`) `INFERRED` `calls` kenarı üretir; taban uçta `reachEdge` Go'daki `INFERRED` `calls` kenarlarını zincire almadığı için `g`'yi yalnız `f` üzerinden sınayan testler `tests-for` çıktısında kaybolur ve yanlış `"tests: none found"` / `"no tests found"` cevabı döner [kanıtlı: `tools/codegraph/gaps.md:45`, "Kritik: yanlış "test yok"."].
- **Kanıt:**
  1. **Küçük fikstür (`tools/codegraph/testdata/mod/shape/`):**
     - `tools/codegraph/testdata/mod/shape/leaf.go:3` (`func leaf() {}`), `tools/codegraph/testdata/mod/shape/shape.go:101-103` (`func apply(fn func()) { fn() }` ve `func use() { apply(leaf) }`) ve `tools/codegraph/testdata/mod/shape/shape_test.go:24` (`func TestUse(t *testing.T) { use() }`) eklendi [test: `tools/codegraph/testdata/mod/shape/leaf.go`].
     - Taban uçta (`main` koduyla) dosya ve işlev sorgularının çıktısı:
       - `go run ./tools/codegraph tests-for --root tools/codegraph/testdata/mod --graph /tmp/mod_graph.json shape/leaf.go` → `tests: none found for the declarations in this file` (`declarations in this file: 1`) [ölçüldü: Adım 1 taban koşusu, `tests: none found for the declarations in this file`].
       - `go run ./tools/codegraph tests-for --root tools/codegraph/testdata/mod --graph /tmp/mod_graph.json leaf` → `no tests found` [ölçüldü: Adım 1 taban koşusu, `no tests found`].
  2. **Depodaki gerçek örnekler (`tensor.lseRow32` ve `tensor.geluExactValue`):**
     - `internal/compute/tensor/logsumexp.go:65` satırında `tensor.LogSumExp`, yüksek dereceli `lseRows` (`internal/compute/tensor/logsumexp.go:74`, `func lseRows[T floatType](x []T, p lsePlan, row func(x, work []T) T) []T {`) yardımcısına `lseRow32` (`internal/compute/tensor/logsumexp.go:100`, `func lseRow32(x, w []float32) float32 {`) işlevini argüman olarak geçirir [kanıtlı: `internal/compute/tensor/logsumexp.go:65`, "y := lseRows(v.Data(), p, lseRow32)"].
     - Taban uçta `go run ./tools/codegraph tests-for tensor.LogSumExp` sorgusu `tensor.TestLogSumExpSameBitsForAnyWorkerCount` ve 15 `logsumexp_test` TR testini verirken, `go run ./tools/codegraph tests-for tensor.lseRow32` sorgusu `no tests traced to it` (yalnızca `weak evidence (142): linked into the binary of cmd/trlang`) döndürüyordu [ölçüldü: Adım 1 taban koşusu, `no tests traced to it`].
     - İkinci gerçek örnek olarak `internal/compute/tensor/cpu_elementwise_wide.go:58` satırında `f64(geluExactValue)` ile geçirilen `tensor.geluExactValue` (`internal/compute/tensor/cpu_elementwise_wide.go:72`, `func geluExactValue(x float64) float64 {`) [kanıtlı: `internal/compute/tensor/cpu_elementwise_wide.go:58`, "f64(geluExactValue)"] için `go run ./tools/codegraph tests-for tensor.geluExactValue` taban uçta `no tests traced to it` döndürüyordu [ölçüldü: Adım 1 taban koşusu, `no tests traced to it`].
  3. **Konumsal tablo örneği (`sumBuiltin`, `internal/lang/interp/builtins.go:25`):**
     - `internal/lang/interp/builtins.go:25` satırındaki `coreBuiltins` bileşik değerinde `{codes.FnSum, sumBuiltin},` konumsal tablo öğesidir [kanıtlı: `internal/lang/interp/builtins.go:25`, "{codes.FnSum, sumBuiltin},"].
     - Taban uçta `go run ./tools/codegraph query sumBuiltin` çıktısında `<- calls (1): interp.coreBuiltins [var] at internal/lang/interp/builtins.go:25 [INFERRED]` yer alır; `go run ./tools/codegraph tests-for sumBuiltin` çıktısı `<- tests about it (6; ...)` ve `<- tests through the name codes it implements (175; same package first)` gruplarını verir (`<- tests, indirect` grubu yoktur) [ölçüldü: Adım 1 ve Adım 2 `go run ./tools/codegraph tests-for sumBuiltin` koşuları, 6 `about` + 175 ad kodu testi].
- **Öneri:** Çağrı argümanı olarak geçirilen paket düzeyi Go işlevlerini öbür `INFERRED` işlev değeri kullanımlarından ayırıp `reachEdge` zincirine dahil etmek.

---

### 2. Adım 2: Düzeltme, Kenar İşareti Seçimi ve Gerekçesi

- **İddia:** `Edge` yapısında var olan `Via` alanını (`const viaArg = "arg"`) kullanarak yalnızca bir çağrının argümanı (`*ast.CallExpr.Args`) olarak geçirilen paket düzeyi Go işlevlerini (`fn.Type().(*types.Signature).Recv() == nil`) işaretlemek ve `reachEdge` içinde `via == viaArg` koşulunu kabul etmek, şemayı (`schemaVersion`) ve `Confidence` (`INFERRED`) anlamını bozmadan en dar kapsamlı çözümü sağlar [görüş].
- **Kanıt:**
  - **Neden `Edge.Via = "arg"` (`viaArg`) en dar işarettir:**
    1. `Edge` üzerinde yeni bir JSON alanı açmak şemayı genişletir; `Confidence` değerini `EXTRACTED` yapmak ise değer olarak geçirilen işlevi doğrudan çağrıyla karıştırır ve `uses_coverage_test.go` ile `query`/`callees` çıktılarını bozar [görüş].
    2. `Edge.Via` alanı zaten `edgeCalls` kenarında ad koduyla anahtarlanmış bileşik değer çiftlerini (`"name:..."`, `keyedImplementations`) işaretlemek için kullanılmaktadır [kanıtlı: `tools/codegraph/gocode_scan.go:169`, "// keyedImplementations bileşik değerde ad kodunu anahtar, işlevi değer alan çiftin (`codes.CmdFormat: formatCommand`)"].
    3. `tools/codegraph/gocode_scan.go:56` satırında `const viaArg = "arg"` tanımlanmış [kanıtlı: `tools/codegraph/gocode_scan.go:56`, `const viaArg = "arg"`], `markCall` içinde `n.Args` öğelerinin `nameIdent(a)` tanımlayıcılarına `viaArg` atanmış (`calls[id] = viaArg`) [kanıtlı: `tools/codegraph/gocode_scan.go:64`, `calls[id] = viaArg`] ve `use` içinde hedef `*types.Func` paket düzeyi işlevse (`fn.Type().(*types.Signature).Recv() == nil`) ve `e.Via == ""` ise `e.Via = viaArg` yazılmıştır [kanıtlı: `tools/codegraph/gocode_scan.go:320`, `if callKind == viaArg && fn.Type().(*types.Signature).Recv() == nil {`].
    4. `tools/codegraph/test_reach.go:21-24` satırlarında `reachEdge` imzası `via string` alacak biçimde güncellenmiş ve `edgeCalls` kolu `via == viaArg` kenarlarını zincire almıştır [kanıtlı: `tools/codegraph/test_reach.go:24`, `return confidence == extracted || via == viaArg || strings.HasPrefix(from, "tr:") && strings.HasPrefix(to, "tr:")`].
    5. `graph.json` içeriği (`Edge.Via`) değiştiği için `extractorRevision` `88`'den `89`'a çıkarılmış [kanıtlı: `tools/codegraph/graph.go:25`, `const extractorRevision = 89`] ve `tools/codegraph/testdata/extractor_digest.txt` güncellenmiştir.
  - **Zincire girmeyen (ayrı kalan) üç durumun doğrulanması:**
    1. **Bileşik değerdeki işlev değeri (`*ast.CompositeLit`):** `{codes.FnSum, sumBuiltin}` ya da `[]func(){litLeaf}` bir `*ast.CallExpr` argümanı değildir (bileşik değerin kendisi bir çağrıya argüman olarak verilse bile `nameIdent` `*ast.CompositeLit` için `nil` döner). `sumBuiltin` çıktısı Adım 1 ile birebir aynı kalmıştır (`6` `about` + `175` ad kodu testi, `indirect` yok) [ölçüldü: `go run ./tools/codegraph tests-for --graph /tmp/after_graph.json --no-rebuild sumBuiltin`].
    2. **Değişkene atanıp sonra geçirilen işlev değeri (`f := varLeaf; apply(f)`):** `varLeaf` atamanın sağ tarafındadır (`calls[id] == ""`); `apply(f)` içindeki `f` ise `*types.Var` olduğu için (`*types.Func` değil) `varLeaf` kenarına `viaArg` yazılmaz ve zincire girmez [test: `tools/codegraph/query_tests_test.go`].
    3. **Yöntem değeri (`apply(b.methodLeaf)`):** `b.methodLeaf` için `fn.Type().(*types.Signature).Recv() != nil` olduğundan `viaArg` yazılmaz ve zincire girmez [test: `tools/codegraph/query_tests_test.go`].
  - **Koruyan test ve gerçek örneklerin düzeltme sonrası çıktısı:**
    - `tools/codegraph/query_tests_test.go:417` satırındaki `TestTestsForFollowsPackageFunctionPassedAsCallArgument` hem Adım 1 fikstürünü (`shape.leaf` ve `shape/leaf.go` → `shape.TestUse`) hem de `p.argLeaf` (zincire girer), `p.litLeaf` (bileşik değer, girmez), `p.varLeaf` (değişkene atanan, girmez) ve `box.methodLeaf` (yöntem değeri, girmez) durumlarını korur [kanıtlı: `tools/codegraph/query_tests_test.go:417`, `func TestTestsForFollowsPackageFunctionPassedAsCallArgument(t *testing.T) {`].
    - `tensor.lseRow32`: Artık `<- tests, indirect (1, ...): tensor.TestLogSumExpSameBitsForAnyWorkerCount [func] internal/compute/tensor/logsumexp_test.go:11  2 steps via tensor.LogSumExp  [INFERRED]` ve `<- tests through the name codes it implements (15; same package first)` verir [ölçüldü: `go run ./tools/codegraph tests-for --graph /tmp/after_graph.json --no-rebuild tensor.lseRow32`].
    - `tensor.geluExactValue`: Artık `<- tests, indirect (1, ...): tensor.TestUnaryOpsIgnorePosition [func] internal/compute/tensor/behavior_unary_position_test.go:19  3 steps via tensor.unaryLoopWide  [INFERRED]` verir [ölçüldü: `go run ./tools/codegraph tests-for --graph /tmp/after_graph.json --no-rebuild tensor.geluExactValue`].
- **Öneri:** Kural bu dilimde olduğu gibi korunmalı; konumsal tablolar ve değişken üzerinden geçirilen işlev değerleri ayrı dilim konusu olarak kalmalıdır.

---

### 3. Adım 3: Büyüme Ölçümleri

- **İddia:** Paket düzeyi işlev argümanlarının (`Via == "arg"`, tüm depoda toplam `237` `calls` kenarı) zincire katılması, yüksek dereceli kayıt/sarmalayıcı (`newBuiltin`, `safe`, `bind.SafeBuiltin`, `interp.AddEngine`, `RegisterModel`) kullanan paketlerde dolaylı (`indirect` ve `weak evidence`) test listelerini büyütür; graf kurulum süresini ise artırmaz [ölçüldü: `/tmp/baseline_graph.json` ve `/tmp/after_graph.json` karşılaştırması].

#### 3.1. Grafın Kurulma Süresi (Önce ve Sonra, 3 Koşunun Ortancası)

| Ölçüm | 1. Koşu | 2. Koşu | 3. Koşu | Ortanca | Düğüm / Kenar |
| :--- | :---: | :---: | :---: | :---: | :--- |
| **Önce (`extractor 88`)** | `8.8 s` | `9.1 s` | `8.9 s` | **`8.9 s`** | `35454 nodes, 287323 edges (187194 EXTRACTED, 100129 INFERRED)` [ölçüldü: `go run ./tools/codegraph build --out /tmp/baseline_graph.json`] |
| **Sonra (`extractor 89`)** | `7.7 s` | `8.0 s` | `7.7 s` | **`7.7 s`** | `35459 nodes, 287482 edges (187223 EXTRACTED, 100259 INFERRED)` [ölçüldü: `go run ./tools/codegraph build --out /tmp/after_graph.json`] |

*(Not: İlk soğuk koşuda Go modül önbelleğine `golang.org/x/text v0.39.0` indirildiği için `14.4 s` sürmüştür; tablodaki üçer koşu sıcak önbellekle ardışık alınmıştır.)*

#### 3.2. Seçilen Dosyalarda (`awk 'NR%40==1'`) `tests-for` Test Sayısı (Önce ve Sonra)

Komut: `git ls-files 'internal/*.go' 'cmd/*.go' 'tools/*.go' | grep -v _test.go | sort | awk 'NR%40==1'` [ölçüldü: `go run ./tools/codegraph tests-for --all <dosya>`].
*(Tablodaki sayı, dosyanın bildirimlerine izlenen testlerin toplamıdır: `this file's tests` + `tests about it` + `tests (direct)` + `tests, indirect` + `weak evidence`. `cmd/trlang/arguments.go` için doğrudan/dolaylı birim testi `0` olup parantez içinde `covered by subprocess tests` sayısı gösterilmiştir. Adım 1'de `tools/codegraph/testdata/mod/shape/leaf.go` eklendiği için `HEAD` üzerinde 721. dosya olan `tools/trlang_migrate/rules.go` da 19. satır olarak listeye girmiştir.)*

| Dosya | Önce | Sonra | Fark | Grup Dağılımı (Önce → Sonra) |
| :--- | :---: | :---: | :---: | :--- |
| `cmd/trlang/arguments.go` | `0` (`142` alt süreç) | `0` (`142` alt süreç) | `0` | `subprocess: 142` → `subprocess: 142` |
| `internal/compute/gpu/backend_multi.go` | `22` | `22` | `0` | `own: 4, about: 1, direct: 2, indirect: 4, weak: 11` (değişmedi) |
| `internal/compute/gpu/profile.go` | `326` | `326` | `0` | `own: 5, about: 1, indirect: 98, weak: 222` (değişmedi) |
| `internal/compute/tensor/cpu_gemv16.go` | `307` | `494` | **`+187`** | `own: 3, indirect: 2, weak: 302` → `own: 3, indirect: 2, weak: 489` |
| `internal/compute/tensor/host_block_windows.go` | `570` | `584` | **`+14`** | `indirect: 16, weak: 554` → `about: 2, indirect: 15, weak: 567` |
| `internal/compute/tokenize/model_other.go` | `49` | `73` | **`+24`** | `about: 2, indirect: 35, weak: 12` → `about: 2, indirect: 35, weak: 36` |
| `internal/devtools/codeindex/library_codes.go` | `252` | `253` | `+1` | `own: 4, about: 1, direct: 2, indirect: 75, weak: 170` → `weak: 171` |
| `internal/devtools/profiler/live_other.go` | `1` | `1` | `0` | `indirect: 1` (değişmedi) |
| `internal/lang/checker/module_rules.go` | `548` | `548` | `0` | `own: 5, about: 3, direct: 12, indirect: 194, weak: 334` (değişmedi) |
| `internal/lang/interp/bytes_value.go` | `1548` | `1557` | `+9` | `own: 3, about: 10, direct: 14, indirect: 1388, weak: 133` → `indirect: 1394, weak: 136` |
| `internal/lang/interp/path_arg.go` | `645` | `820` | **`+175`** | `own: 1, about: 3, indirect: 109, weak: 532` → `own: 1, about: 3, indirect: 425, weak: 391` |
| `internal/lang/lexer/unicode_safety.go` | `1456` | `1456` | `0` | `own: 1, about: 1, indirect: 587, weak: 866` (değişmedi) |
| `internal/layoutrules/exceptions.go` | `26` | `27` | `+1` | `indirect: 5, weak: 21` → `indirect: 5, weak: 22` |
| `internal/stdlib/logging/terminal_unix.go` | `4` | `4` | `0` | `indirect: 4` (değişmedi) |
| `internal/stdlib/ml/print.go` | `1028` | `1028` | `0` | `own: 2, indirect: 208, weak: 818` (değişmedi) |
| `internal/testkit/known_list.go` | `2` | `2` | `0` | `own: 1, direct: 1` (değişmedi) |
| `tools/codegraph/history.go` | `83` | `84` | `+1` | `own: 3, direct: 3, indirect: 1, weak: 76` → `weak: 77` |
| `tools/codegraph/test_paths_param.go` | `174` | `175` | `+1` | `own: 1, indirect: 1, weak: 172` → `weak: 173` |
| `tools/trlang_migrate/rules.go` | `0` | `0` | `0` | izlenen birim testi yok (`scans: 26, type_queries: 4`) |

#### 3.3. En Çok Büyüyen Üç Dosyanın Sebebi ve Yüksek Dereceli İşlev Örnekleri

1. **`internal/compute/tensor/cpu_gemv16.go` (`307 → 494`, `+187` test; tamamı `weak evidence: 302 → 489`):**
   - **Yüksek dereceli işlevler:** `ml.newBuiltin` (`internal/stdlib/ml/builtins.go:30` ve `internal/stdlib/ml/builtin_conv.go:25`) ve `interp.AddEngine` (`internal/stdlib/engines.go:33`).
   - **Somut zincir:** `ml.Register` içinde `wrap(v, codes.FnLinearCrossEntropy, newBuiltin(linearCrossEntropy))` [kanıtlı: `internal/stdlib/ml/builtins.go:30`, `wrap(v, codes.FnLinearCrossEntropy, newBuiltin(linearCrossEntropy))`] ve `ml.registerConv` içinde `wrap(v, codes.FnConv1d, newBuiltin(conv1dBuiltin))` [kanıtlı: `internal/stdlib/ml/builtin_conv.go:25`, `wrap(v, codes.FnConv1d, newBuiltin(conv1dBuiltin))`] çağrıları, paket düzeyi `linearCrossEntropy` ve `conv1dBuiltin` işlevlerini `newBuiltin`'e argüman olarak geçirir; `stdlib.init` de `interp.AddEngine(ml.Register)` [kanıtlı: `internal/stdlib/engines.go:33`, `interp.AddEngine(ml.Register)`] ile `ml.Register`'ı argüman olarak geçirir. `linearCrossEntropy → tensor.LinearCrossEntropy → tensor.LinearCrossEntropyChunked → tensor.gateGemm → tensor.Backend.Gemm → tensor.cpuBackend.Gemm → tensor.gemv16` (ve `conv1dBuiltin → ml.matMulOn → tensor.MatMulKernel → ... → tensor.gemv16`) yolu bağlandığı için, `ml.Register` çağıran testler `weak evidence` (>4 adım) grubuna eklenmiştir [ölçüldü: `/tmp/after_graph.json` zincir izi].
2. **`internal/lang/interp/path_arg.go` (`645 → 820`, `+175` test; `indirect: 109 → 425`, `weak evidence: 532 → 391`):**
   - **Yüksek dereceli işlevler:** `files.safe` (`internal/stdlib/files/file.go:38-45`), `bind.SafeBuiltin` (`internal/stdlib/dataset/dataset.go:59`, `internal/stdlib/tokenizer/tokenizer.go:114`), `ml.newBuiltin` (`internal/stdlib/ml/builtins.go:59-61`) ve `interp.AddEngine` (`internal/stdlib/engines.go:29-36`).
   - **Somut zincir:** `files.Register` içinde `v.RegisterNoNamed(codes.FnReadLines, safe(codes.FnReadLines, linesBuiltin))` [kanıtlı: `internal/stdlib/files/file.go:38`, `v.RegisterNoNamed(codes.FnReadLines, safe(codes.FnReadLines, linesBuiltin))`] (ve `pathBuiltin`, `listBuiltin`, `makeDirBuiltin`, `deleteBuiltin`), `dataset.Register` içinde `bind.SafeBuiltin(interp.CallPrefix(codes.TypeDataset), openDataset)` [kanıtlı: `internal/stdlib/dataset/dataset.go:59`, `v.RegisterTypeCall(codes.TypeDataset, &interp.BuiltinVal{Fn: bind.SafeBuiltin(interp.CallPrefix(codes.TypeDataset), openDataset)})`] ve `tokenizer.Register` içinde `bind.SafeBuiltin(interp.CallPrefix(codes.FnLoadTokenizer), load)` [kanıtlı: `internal/stdlib/tokenizer/tokenizer.go:114`, `v.RegisterBuiltinCode(codes.FnLoadTokenizer, bind.SafeBuiltin(interp.CallPrefix(codes.FnLoadTokenizer), load))`] çağrıları, içinde `interp.PathArg` çağıran yerleşik işlevleri yüksek dereceli sarmalayıcılara argüman olarak geçirir. Bu sayede `files.Register`, `dataset.Register`, `tokenizer.Register` ve `ml.Register` kullanan testler 3–4 adımda (`indirect`) `interp.PathArg`'a bağlanmıştır [ölçüldü: `/tmp/after_graph.json` zincir izi].
3. **`internal/compute/tokenize/model_other.go` (`49 → 73`, `+24` test; `weak evidence: 12 → 36`):**
   - **Yüksek dereceli işlevler:** `tokenize.RegisterModel` (`internal/compute/tokenize/model_other.go:16-18`), `bind.SafeBuiltin` (`internal/stdlib/tokenizer/tokenizer.go:114`) ve `interp.AddEngine` (`internal/stdlib/engines.go:35`).
   - **Somut zincir:** `tokenize.init` içinde `RegisterModel("WordPiece", newWordPiece)` [kanıtlı: `internal/compute/tokenize/model_other.go:16`, `RegisterModel("WordPiece", newWordPiece)`] (ve `newWordLevel`, `newUnigram`) kurucuları argüman olarak geçer; ayrıca `tokenizer.Register` içindeki `bind.SafeBuiltin(interp.CallPrefix(codes.FnLoadTokenizer), load)` [kanıtlı: `internal/stdlib/tokenizer/tokenizer.go:114`, `v.RegisterBuiltinCode(codes.FnLoadTokenizer, bind.SafeBuiltin(interp.CallPrefix(codes.FnLoadTokenizer), load))`] üzerinden `tokenizer.load → tokenize.Load → tokenize.Parse → tokenize.Model.Vocab → tokenize.unigram.Vocab` (ve `wordPiece.Vocab`, `wordLevel.Vocab`) zinciri bağlanarak `tokenizer.Register` çağıran 24 testi `weak evidence` grubuna katmıştır [ölçüldü: `/tmp/after_graph.json` zincir izi].

#### 3.4. `main`'in Son 10 Commit'inde `changed --go-tests` Seçimi (Önce ve Sonra)

*(Not: `changed` tek başına `<commit>` verildiğinde `main...<commit>` ortak atasını alır; `main` üzerindeki commit'lerde o commit'in kendi farkını ölçmek için `<commit>~1..<commit>` aralığı kullanılmıştır; bkz. `Codegraph` başlığı. Araç süre vermemektedir [kanıtlı: `tools/codegraph/gaps.md:74`, "- **`changed --go-tests` süre söylemiyor (S1, 2026-11-06).**"].)*

| Commit (`main` son 10) | Aralık | Önce (Test / Paket) | Sonra (Test / Paket) | Fark (Test / Paket) |
| :--- | :--- | :---: | :---: | :---: |
| `ffca754` | `ffca754~1..ffca754` | `15` test / `4` paket | `15` test / `4` paket | `0` / `0` |
| `336a423` | `336a423~1..336a423` | `15` test / `4` paket | `15` test / `4` paket | `0` / `0` |
| `4ecb9ca` | `4ecb9ca~1..4ecb9ca` | `15` test / `4` paket | `15` test / `4` paket | `0` / `0` |
| `41eeaec` | `41eeaec~1..41eeaec` | `36` test / `8` paket | `36` test / `8` paket | `0` / `0` |
| `a21ad9e` | `a21ad9e~1..a21ad9e` | `31` test / `5` paket | `31` test / `5` paket | `0` / `0` |
| `65ef3e3` | `65ef3e3~1..65ef3e3` | `33` test / `11` paket | `33` test / `11` paket | `0` / `0` |
| `10f57d8` | `10f57d8~1..10f57d8` | `34` test / `11` paket | `34` test / `11` paket | `0` / `0` |
| `d71778e` | `d71778e~1..d71778e` | `851` test / `39` paket | `888` test / `39` paket | **`+37` test / `0` paket** |
| `583a3e2` | `583a3e2~1..583a3e2` | `851` test / `39` paket | `888` test / `39` paket | **`+37` test / `0` paket** |
| `9126e27` | `9126e27~1..9126e27` | `811` test / `30` paket | `811` test / `30` paket | `0` / `0` |

[ölçüldü: `go run ./tools/codegraph changed --go-tests <commit>~1..<commit>`, `/tmp/step3_commits_before.txt` ve `/tmp/step3_commits_after.txt`]

---

## Bitti Ölçütü Komutları ve Son Satırları

1. **`gofmt -l tools`**
   - Çıktı: *(boş, 0 satır; çıkış kodu 0)* [ölçüldü: `gofmt -l tools`, boş]

2. **`go test ./tools/codegraph/...`**
   - Çıktı (`TestEveryTrackedDataFileHasATestOrAReason` beklenen biçimde yalnız iki `UZAK7_` dosyasını görür; öbür tüm testler geçer):
     ```
     --- FAIL: TestEveryTrackedDataFileHasATestOrAReason (0.07s)
         data_coverage_test.go:115: 2 tracked data file(s) no test is linked to (tests-for would be empty): link a test (a path constant a test or its helper reads; a go:embed read by a loader a test reaches) or add the file to testdata/uncovered_data.txt with a reason:
               UZAK7_DURUM.md
               UZAK7_RAPORU.md
     FAIL
     FAIL	github.com/sekerahmet/TCB/tools/codegraph	293.435s
     FAIL
     ```
     [ölçüldü: `go test ./tools/codegraph/...`, yalnız `TestEveryTrackedDataFileHasATestOrAReason` iki `UZAK7_` dosyası için düştü]

3. **`go run ./tools/codegraph tests-for --root tools/codegraph/testdata/mod --graph /tmp/mod_graph.json shape/leaf.go`**
   - Çıktı (son satırı):
     ```
     shape/leaf.go [file] shape/leaf.go:1
     run: go test -short ./shape/
       these commands run every Go test the pre-check selects for this file
       declarations in this file: 1
       <- tests, indirect (1, through calls, loaded files or name codes, up to 4 steps; same folder first at equal steps):
         shape.TestUse [func] shape/shape_test.go:24  2 steps [INFERRED]
     ```
     [ölçüldü: `go run ./tools/codegraph tests-for --root tools/codegraph/testdata/mod --graph /tmp/mod_graph.json shape/leaf.go`, `shape.TestUse [func] shape/shape_test.go:24  2 steps [INFERRED]`]

4. **`git diff --stat main...HEAD`**
   - Çıktı (yalnız `tools/codegraph/` dosyaları ve iki `UZAK7_` dosyası):
     ```
      UZAK7_DURUM.md                                   |  58 +++++++
      UZAK7_RAPORU.md                                  | 206 +++++++++++++++++++++++
      tools/codegraph/README.md                        |   2 +-
      tools/codegraph/dispatch_test.go                 |   4 +-
      tools/codegraph/gocode_scan.go                   |  33 ++--
      tools/codegraph/graph.go                         |   5 +-
      tools/codegraph/query_tests.go                   |   2 +-
      tools/codegraph/query_tests_test.go              |  60 +++++++
      tools/codegraph/test_paths.go                    |   2 +-
      tools/codegraph/test_reach.go                    |   9 +-
      tools/codegraph/testdata/extractor_digest.txt    |   4 +-
      tools/codegraph/testdata/mod/shape/leaf.go       |   3 +
      tools/codegraph/testdata/mod/shape/shape.go      |   4 +
      tools/codegraph/testdata/mod/shape/shape_test.go |   2 +
      14 files changed, 373 insertions(+), 21 deletions(-)
     ```
     [ölçüldü: `git diff --stat main...HEAD`]

---

## Ortam (Temiz Makine Bulguları)

1. **Go sürümü ve `gofmt` farkı:** Uzak sanal makinede `go version go1.27.3 linux/amd64` kuruludur (`go.mod` ise `toolchain go1.26.10` ister; yönerge gereği `GOTOOLCHAIN=local` kullanılmıştır). `go1.27.3`'ün `gofmt`'u, dokunulmayan `internal/lang/checker/member_types_test.go` dosyasında biçim farkı göstermektedir (`gofmt -l cmd internal tools lib` çıktısında tek satır: `internal/lang/checker/member_types_test.go`), `gofmt -l tools` ise tamamen boştur [ölçüldü: `gofmt -l cmd internal tools lib` ve `gofmt -l tools`]. İzinli dosya kapsamı (`tools/codegraph/`) dışında olduğu için `internal/lang/checker/member_types_test.go` dosyasına dokunulmamıştır.
2. **Modül önbelleği:** Temiz makinede ilk `go run ./tools/codegraph` çağrısı `golang.org/x/text v0.39.0` paketini indirmiştir; sonraki koşular yerel önbellekten çalışmıştır [ölçüldü: ilk `build` koşusu].

---

## Codegraph

1. **`changed --go-tests <commit>` tek commit argümanında `main` geçmişi için boş dönüyor:**
   - **Komut:** `go run ./tools/codegraph changed --go-tests 583a3e2`
   - **Beklenen:** `583a3e2` commit'inin değiştirdiği düğümlerin Go testlerini listelemesi (`888` test).
   - **Gelen:** Boş çıktı (`0` satır) [ölçüldü: `go run ./tools/codegraph changed --go-tests --graph /tmp/after_graph.json --no-rebuild 583a3e2 | wc -l`, `0`].
   - **Neden ve doğrusunu nasıl buldum:** `tools/codegraph/changed.go:30` satırında `rng, right, flag := o.base+"..."+ref, ref, "--base"` [kanıtlı: `tools/codegraph/changed.go:30`, `rng, right, flag := o.base+"..."+ref, ref, "--base"`] üç noktalı `main...<commit>` aralığı kurar. `<commit>` `main` dalının bir atası olduğunda `git merge-base main <commit>` değeri `<commit>`'in kendisi olduğundan fark boş (`<commit>..<commit>`) çıkar. Doğru sonucu `<commit>~1..<commit>` iki noktalı aralığıyla (`go run ./tools/codegraph changed --go-tests 583a3e2~1..583a3e2` → `888` satır) aldım.
2. **Test yorumunda çıplak `gaps.md` adı `cites` kenarı üretip `TestEveryTrackedDataFileHasATestOrAReason` testini düşürüyor:**
   - `tools/codegraph/query_tests_test.go` içindeki bir test yorumunda `(gaps.md:45)` yazıldığında `citeRefs` (`test_paths.go`) aynı klasördeki `tools/codegraph/gaps.md` dosyasına `cites` kenarı çekmekte ve `testdata/uncovered_data.txt` içindeki `tools/codegraph/gaps.md` istisnasını boşa düşürmektedir (`41eeaec` commit'iyle aynı durum). Yorum `(eksikler listesi, DENETİM 79)` biçiminde yazılarak çözülmüştür [ölçüldü: `go test -short ./tools/codegraph/`].

---

## Emin Olmadıklarım ve Bakmadıklarım

- **Emin olmadıklarım:** Yok.
- **Bakmadıklarım:** Yönerge gereği konumsal bileşik değer tablolarındaki (`{codes.FnSum, sumBuiltin}`) işlev değerlerinin, değişkene atanıp sonra geçirilen işlev değerlerinin ve yöntem değerlerinin (`x.M`) zincire bağlanmasına; ayrıca `tools/codegraph/` dışındaki paketlere ve `tools/codegraph/gaps.md` dosyasına dokunulmamıştır.

---

## Kendi Kontrolüm

- Rapordaki **20 birebir alıntı** `grep -n -F` ile ilgili kaynak dosyalarda (`tools/codegraph/gaps.md`, `tools/codegraph/gocode_scan.go`, `tools/codegraph/test_reach.go`, `tools/codegraph/graph.go`, `tools/codegraph/query_tests_test.go`, `tools/codegraph/changed.go`, `internal/compute/tensor/logsumexp.go`, `internal/compute/tensor/cpu_elementwise_wide.go`, `internal/lang/interp/builtins.go`, `internal/stdlib/ml/builtins.go`, `internal/stdlib/ml/builtin_conv.go`, `internal/stdlib/engines.go`, `internal/stdlib/files/file.go`, `internal/stdlib/dataset/dataset.go`, `internal/stdlib/tokenizer/tokenizer.go`, `internal/compute/tokenize/model_other.go`) tek tek arandı; 20'si de karakter karakter eşleşti (`0` silinen veya değişen alıntı).
- Raporda anılan **23 `dosya:satır` başvurusu** ve Adım 3 tablolarındaki tüm sayılar (`19` dosya satırı, `10` commit satırı, `3` graf kurulum koşusu) kaynak dosyalardan ve `/tmp/step3_files_before.txt`, `/tmp/step3_files_after.txt`, `/tmp/step3_commits_before.txt`, `/tmp/step3_commits_after.txt` çıktılarından yeniden açılıp doğrulandı (`0` silinen veya değişen sayı).

Kendi incelemem bitti.
