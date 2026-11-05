# UZAK3_RAPORU: codegraph, ön denetimin kaçırdığı iki test sınıfı ve bir panik

| Alan | Değer |
|---|---|
| **Dal** | `gemini/uzak-3` |
| **Taban (`main`)** | `25e76eae782f857178f9c685af47c86d08dad5ec` `[ölçüldü: git rev-parse main, 25e76eae782f857178f9c685af47c86d08dad5ec]` |
| **Başlama / Bitiş** | `2026-11-05 08:13 +03` / `2026-11-05 11:20 +03` `[ölçüldü: TZ=Europe/Istanbul date, +03]` |
| **Kod commit'leri (2 → 3 → 1 sırasıyla)** | 1. `37e463f9db6a9aa4f0ca17bd049c979992ff2a67` (`codegraph: recover panics in buildGraph to keep stale graph on broken messages`)<br>2. `594bbe7ad5f144d68b1c84a5f96b98be521100b7` (`codegraph: link bare file reads in test files and drop slash-tail comment cites`)<br>3. `788cdb2c6bc1bcfe7f95e96ca6542aeeccfeb19b` (`codegraph: link planning exam targets via names_in_exam and deleted-line diffs`) |
| **Bağlam sıkıştırması** | 6 kez: `08:31 +03` (taban ölçümlerinden sonra), `08:53 +03` (Madde 2 push'undan sonra), `09:18 +03` (Madde 3 yerel commit'inden sonra), `09:36 +03` (Madde 3 durum push'undan sonra), `09:59 +03` (Madde 1 taslak kodlaması sırasında), `10:45 +03` (üç maddenin commit/push işlemleri bittikten sonra, rapor yazımından önce) |
| **Geçici dal dosyaları** | Kök dizindeki `UZAK3_DURUM.md` ve `UZAK3_RAPORU.md` dosyalarını `TestEveryTrackedDataFileHasATestOrAReason` görür (`tools/codegraph/data_coverage_test.go:115`); yönerge gereği teste ve `testdata/uncovered_data.txt` listesine dokunulmadı, TCB birleştirmede siler `[ölçüldü: go test -short ./tools/codegraph -run '^TestEveryTrackedDataFileHasATestOrAReason$', UZAK3_DURUM.md]` |

---

## Bulgular ve Uygulamalar (Önem Sırasıyla: Madde 1, Madde 3, Madde 2)

### 1. Madde 1 (Kritik): Sınav dosyasının andığı düğümlerin `names_in_exam` ile bağlanması ve yeniden adlandırma/silme farkları (`788cdb2c6bc1bcfe7f95e96ca6542aeeccfeb19b`)

- **İddia ve yeniden üretim (önce):** `tools/codegraph/testdata/planning_questions.txt` soruları düz metin olarak taşır; `TestPlanningQuestionsAreAnswered` andığı işlevleri çağırmadığı için `tests-for` ve `changed --go-tests` bu testi seçmiyordu `[ölçüldü: go run ./tools/codegraph tests-for internal/lang/interp/compiler_expr.go (25e76eae üzerinde), çıktıda TestPlanningQuestionsAreAnswered yok; son satır: ... 76 more (--all lists them)]`.
- **Sınıfların sayımı:**
  - `testdata/planning_questions.txt` içindeki komut argümanları, `@{dosya#metin}` satır başvuruları ve beklenti sözcükleri (`examNodeKeys` süzgeciyle başlık, paket ve bölüm numaraları elendikten ve en az iki parçalı nokta son ekleri eklendikten sonra) yalnız `TestPlanningQuestionsAreAnswered` düğümünden `names_in_exam` (`INFERRED`) kenarıyla **251** benzersiz düğüme bağlandı `[ölçüldü: go run ./tools/codegraph build, names_in_exam 251]`.
  - Bağlanan `251` düğümün tür dağılımı: `func` `96`, `method` `46`, `file` `41`, `name` `21`, `doc` `9`, `field` `8`, `var` `8`, `type` `6`, `tr_method` `5`, `message` `3`, `tr_struct` `3`, `const` `2`, `tr_field` `1`, `tr_func` `1`, `tr_param` `1` `[ölçüldü: .codegraph/graph.json üzerinde names_in_exam hedef düğüm türleri sayımı, toplam 251]`.
  - En çok bağlanan düğümler (soru satırı geçiş sayısına göre `e.Count`): `TR_mem_map` (`x8`), `layers.TR_type_Linear.TR_mem_forward` (`x5`), `codes.Language.Name` (`x4`), `TR_fn_print` (`x4`), `TR_mem_shape` (`x4`), `lib/layers.tr` (`x4`), `lib/layers_test.tr` (`x4`), `lib/pretrained_test.tr` (`x4`), `internal/lang/codes/tr.json` (`x3`), `internal/lang/interp/compiler_assign.go` (`x3`), `e2e.TestReadyModelMatchesTransformers` (`x3`), `gpu.backend.Where` (`x3`) `[ölçüldü: .codegraph/graph.json üzerinde names_in_exam kenarlarının count sıralaması]`.
- **Çözüm ve kanıt:**
  - Yeni kenar türü `edgeNamesInExam` eklendi `[kanıtlı: tools/codegraph/graph.go:79, `edgeNamesInExam   = "names_in_exam"  // sınav testi -> sorunun metin olarak andığı düğüm (exam_links.go)`]`.
  - `tools/codegraph/exam_links.go` (`157` toplam satır, `135` boşluksuz/yorumsuz üretim satırı; Madde 1'in tüm `.go` dosyalarındaki eklenen ham kod satırı toplamı `149 <= 150` tavan) içinde `examNodeKeys` (`tools/codegraph/exam_links.go:29`), `parseExamWords` (`tools/codegraph/exam_links.go:44`), `linkExam` (`tools/codegraph/exam_links.go:61`, çağrı yeri `tools/codegraph/main.go:180`) yazıldı.
  - `tests-for` çıktısında doğrudan test listelerine karışmadan ayrı başlık basılır `[kanıtlı: tools/codegraph/exam_links.go:100, `fmt.Fprintf(w, "  <- exam questions that name it (1; questions are plain text in %s):\n    %s\n", examFile, ix.edgeLine(e, e.From, true))`]`; `query_tests.go:260-263` içinde `printLinkedBinaries` davranışı korundu, `who-uses` (`tools/codegraph/query.go:385`) ve yönsüz `path` (`tools/codegraph/query_path.go:68`) bu kenarı atlar, böylece planlama sınavının mevcut sorularının hiçbir cevabı bozulmaz `[ölçüldü: go test ./tools/codegraph -run TestPlanningQuestionsAreAnswered, ok]`.
  - Sınav dosyası değiştiğinde grafiğin bayat kalmaması için `stamped` işlevine `examFile` eklendi `[kanıtlı: tools/codegraph/fresh.go:25, `return stampExts[strings.ToLower(filepath.Ext(p))] || filepath.Base(p) == "go.mod" || filepath.ToSlash(p) == examFile`]`.
  - Yeniden adlandırma ve silme farklarında eski ad çalışma ağacının grafiğinde kalmadığından `touchSet.hunks` (`tools/codegraph/diffmap.go:79`, `tools/codegraph/diffmap.go:104`) üzerinden `deletedHunkNamesExam` (`tools/codegraph/exam_links.go:122`) farkın silinen satırlarında ve eski dosya yollarında grafikte artık düğümü bulunmayan sınav sözcüklerini yakalar; grafikte zaten hiç düğümü olmayan 18 nitelikli sözcüğün her değişiklikte yanlış seçim üretmemesi için yalnız silinen satırlarda geçen simgeler eşlenir `[test: tools/codegraph/exam_links_test.go]`.
  - `extractorRevision` `86 → 87` (`tools/codegraph/graph.go:25`) ve `tools/codegraph/testdata/extractor_digest.txt` güncellendi; kapanan madde `tools/codegraph/gaps.md` içinden silindi ve `tools/codegraph/README.md:41,169`'a belgelendi.
- **Koruyan testler:** `TestEveryNodeTheExamNamesIsLinkedFromTheExam` ve `TestExamSelectionOnModifyRenameDeleteAndStamp` `[test: tools/codegraph/exam_links_test.go]`.

---

### 2. Madde 3 (Kritik): Test dosyalarındaki çıplak dosya okuma çağrıları ve yorumlardaki yol kuyruğu `cites` kenarları (`594bbe7ad5f144d68b1c84a5f96b98be521100b7`)

- **İddia ve yeniden üretim (önce):** `TestReadmeGraphSizeIsCurrent` paket klasöründeki `README.md` dosyasını çıplak adla okuyor `[kanıtlı: tools/codegraph/graph_test.go:265, `data, err := os.ReadFile("README.md")`]`; ancak `codegraph` bunu bağlamadığı için `tests-for tools/codegraph/README.md` doğrudan okuyucu göstermiyordu `[ölçüldü: go run ./tools/codegraph tests-for tools/codegraph/README.md (25e76eae üzerinde), no tests read it by name (tests that guard its rules, cite it or scan its pattern, below)]`.
- **Sınıfların sayımı:**
  - Depodaki bütün `*_test.go` dosyalarında dosya okuma çağrılarına (`os.ReadFile`, `Open`, `OpenFile`, `Stat`, `Lstat`, `ReadDir`, `ioutil.ReadFile`/`ReadDir`, `reporoot.ReadDir`, `parser.ParseDir`, `src == nil` iken `parser.ParseFile`) verilen `/` içermeyen çıplak argüman sayısı toplam **35** (`6` dizgi değişmezi, `0` sabit, `0` tek atamalı yerel değişken, `29` yardımcı işlev parametresi; üretim `.go` dosyalarında sabit/dizgi çıplak argüman `0`) `[ölçüldü: go/types tabanlı AST taraması, toplam 35 çıplak argüman]`:
    1. Çağıran testin paket klasöründe var olan izlenen dosya: **1** (`tools/codegraph/graph_test.go:265` → `tools/codegraph/README.md`).
    2. Klasör okuyan `reporoot.ReadDir(".")`: **3** (`internal/compute/tensor/gate_calls_test.go:99`, `internal/devtools/debugger/behavior_dap_wire_names_test.go:18`, `tools/codegraph/extractor_digest_test.go:34`; `.` dosya olmadığı için bağlanmaz).
    3. İzlenmeyen geçici/sözdizimi dizgisi: **2** (`internal/testkit/testkit_test.go:34` içinde `"p.go"` ve `tools/edithook/codegraph_test.go:37` içinde `"broken"`).
    4. Sabit, tek atamalı yerel değişken ya da yardımcı parametresi üzerinden çağıran testin paket klasöründe var olan izlenen bir dosyaya çözülen çıplak ad: **0**.
  - Yorumlarda `'/'` karakterinden hemen sonra gelen yol kuyruklarının (`fileWord`) sahte `cites` kurduğu yerler: **4 yorum satırında toplam 6 sahte `cites` kenarı** kaldırıldı `[ölçüldü: .codegraph/graph.json önce/sonra cites farkı, -6 sahte kenar]`:
    1. `cmd/trlang/e2e/behavior_examples_coverage_test.go:22` (`TestExamplesCoverCommandLine` ve `TestExamplesCoverNames` → sahte `doc:cmd/trlang/e2e/README.md`, `2` kenar) `[kanıtlı: cmd/trlang/e2e/behavior_examples_coverage_test.go:22, `// examples/README.md'de aranır.`]`
    2. `cmd/trlang/e2e/behavior_examples_test.go:163` (`TestExamplesListedInReadme` → sahte `doc:cmd/trlang/e2e/README.md`, `1` kenar) `[kanıtlı: cmd/trlang/e2e/behavior_examples_test.go:163, `// examples/README.md'deki sıralı liste her örneği ve her hata örneğini listeler.`]`
    3. `internal/stdlib/ml/behavior_grad_composite_test.go:20` (`TestCompositeGrad` ve `TestMeasureCompositeBF16` → sahte `go:file:internal/stdlib/ml/helpers_test.go`, `2` kenar) `[kanıtlı: internal/stdlib/ml/behavior_grad_composite_test.go:20, `// (internal/compute/tensor/helpers_test.go): float64 başvurunun merkezi sonlu farkı, bf16`]`
    4. `tools/codegraph/codes_test.go:106` (`TestANameCodeGivenToTheCodesPackageIsReadNotImplemented` → sahte `go:file:tools/codegraph/main.go`, `1` kenar) `[kanıtlı: tools/codegraph/codes_test.go:106, `// kabuğun startsWithPrint'i yazdır'ın yazımlarını okuduğu için yazdır kullanan altı TR testi cmd/trlang/main.go'nun testi`]`
- **Çözüm ve kanıt:**
  - `tools/codegraph/test_paths.go:33` içine `local bool` alanı ve `linkTestPaths` (`tools/codegraph/test_paths.go:360-368`) içine yalnız çağıran testin paket klasöründen (`path.Join(reach.pkgOf(t), r.text)`) çözen, depo kökünden asla denemeyen ve üretim koduna uygulanmayan dal eklendi.
  - `tools/codegraph/test_paths_param.go:80` içinde `resolveTestBareReads`, `takesTesting`, `bareReadArg`, `concreteExprStrings` yazıldı (`96` boşluksuz/yorumsuz üretim satırı `< 100` tavan; çağrı yeri `tools/codegraph/gocode.go:155`).
  - `tools/codegraph/test_paths.go:236` içinde `commentFileWords` yazıldı `[kanıtlı: tools/codegraph/test_paths.go:239, `if idx[0] == 0 || text[idx[0]-1] != '/' {`]`; iki `//` satırına bölünen yolun kuyruğu (`// main.go`) sınırı `tools/codegraph/gaps.md:57`'ye ince ayar notu olarak eklendi.
  - `tools/codegraph/README.md` artık `TestReadmeGraphSizeIsCurrent` tarafından kapsandığı için `tools/codegraph/testdata/uncovered_data.txt` listesinden çıkarıldı ve istisna tavanı düşürüldü `[kanıtlı: tools/codegraph/data_coverage_test.go:22, `const maxUncoveredData = 8`]`.
- **Koruyan test:** `TestBareReadCallsAndSlashTailComments` `[test: tools/codegraph/test_paths_param_test.go]`.

---

### 3. Madde 2 (Küçük): İletide tanımsız yuva varken `buildGraph` paniğinin yakalanıp eski grafiğin korunması (`37e463f9db6a9aa4f0ca17bd049c979992ff2a67`)

- **İddia ve yeniden üretim (önce):** `internal/lang/messages/tr.json` içinde tanımsız `{#tanimsiz}` yuvası varken `messages.load` `sync.Once` içinde panikler `[kanıtlı: internal/lang/messages/catalog.go:82, `loadOnce.Do(func() {`]`; panik `err` dönüşünden geçmediği için `tests-for` eski grafiği kullanmak yerine `86` satır panik iziyle (alt süreç çıkış `2`) çöküyordu `[ölçüldü: geçici {#tanimsiz} ile go run ./tools/codegraph tests-for internal/lang/messages/tr.json, stderr 86 satır: panic: messages: tr.json: H7012: {#tanimsiz}: no such name slot in the definition (slots: [String for in var]), stdout 0 satır; geçici değişiklik hemen geri alındı]`.
- **Çözüm ve kanıt:**
  - `tools/codegraph/main.go:137-142` içinde `buildGraph` çevresine `recover` eklendi `[kanıtlı: tools/codegraph/main.go:139, `if r := recover(); r != nil {`]`; yakalanan panik `firstLine` ile tek satırlık `error` değerine çevrilir.
  - Böylece `refresh` mevcut kuralına düşer `[kanıtlı: tools/codegraph/fresh.go:181, `fmt.Fprintf(stderr, "codegraph: warning: rebuild failed (%v); using the existing, stale graph\n", firstLine(err))`]`: eski grafik varken tek satır uyarı yazar, eski grafikten cevabı basar ve çıkış `0` döner; `build` komutunda ya da grafik yokken tek satır hata basıp sıfır dışı kodla çıkar.
- **Koruyan test:** `TestUndefinedMessageSlotKeepsStaleGraphAndExitsOnBuild` (`testkit.BuildWithFile` ile ayrı süreçte hem eski grafikle `tests-for` çıkış `0` + tek satır uyarı + cevap, hem `build` sıfır dışı çıkış + tek satır hata, hem de grafik yokken `tests-for` sıfır dışı çıkış + tek satır hata sınanır) `[test: tools/codegraph/main_query_test.go]`.

---

## Bedel: Önce ve Sonra Ölçümleri

### 1. Grafik boyutu, kenar sayıları ve `build` süresi

| Ölçüt | Önce (`main` `25e76eae`) | Sonra (`788cdb2c`, temiz birleştirme ağacı) | Açıklama |
|---|---|---|---|
| **Düğüm / Kenar** | `31604` düğüm, `235111` kenar (`165266 EXTRACTED`, `69845 INFERRED`) | `31647` düğüm, `235894` kenar (`165578 EXTRACTED`, `70316 INFERRED`) | `[ölçüldü: go run ./tools/codegraph build]` (`+43` düğüm yeni üretim ve test bildirimlerinden) |
| **Sıcak `build` süresi** | `8.1s` / `7.3s` (sakin CPU'da `6.2s`) | `6.1s` (duvar `6239 ms`) | `[ölçüldü: go run ./tools/codegraph build]` (ek gecikme yok; soğuk ilk koşu modül indirmesiyle `11.5s` / duvar `28944 ms`) |
| **`cites` kenar sayısı** | `604` | `601` | `-6` sahte yol kuyruğu kenarı silindi (`598`), bu dalda eklenen 3 yeni koruyan test dosyasından `+3` gerçek anma (`docs/README.md`, `internal/lang/interp/compiler_expr.go`, `internal/lang/messages/tr.json`) eklendi `[ölçüldü: .codegraph/graph.json]` |
| **`names_in_exam` kenar sayısı** | `0` | `251` | Yalnız `TestPlanningQuestionsAreAnswered` düğümünden sınavda anılan `251` hedefe `[ölçüldü: .codegraph/graph.json]` |
| **`tests-for` çıktısı değişen dosya** | `1805` izlenen dosya (taban) | `261` değişti (`1544` aynı) | `126` dosya `<- exam questions that name it` kazandı; `tools/codegraph/README.md` `TestReadmeGraphSizeIsCurrent` kazandı; `14` dosya bu dalda değişen `tools/codegraph/*` dosyalarıdır; kalanlar `cmd/trlang/e2e/README.md` (`-3` sahte `cites`), yeni koruyan testlerin andığı/okuduğu dosyalar ve `tools/codegraph`'ın aktardığı Go paketlerindeki `not run by these commands: N more Go tests` sayacının yeni 4 testle artmasıdır `[ölçüldü: runQueryOn ile 1805 dosya SHA-256 karşılaştırması]` |

### 2. Beş aralık (`go run ./tools/edithook codegraph changed --go-tests --base <taban> <uç>`) ve son 60 commit

| Aralık | Önce (satır) | Sonra (satır) | Fark | Farkın kaynağı (`diff -u`) |
|---|---:|---:|---:|---|
| `0b9fda0b..83fa8859` | `1290` | `1291` | `+1` | Mevcut testlerde `+0`; eklenen tek satır `tools/codegraph TestUndefinedMessageSlotKeepsStaleGraphAndExitsOnBuild` (aralık `internal/lang/messages/tr.json` dosyasına dokunduğu için yeni koruyan testi seçer) `[ölçüldü: diff -u before after]` |
| `18c6beba..74a72663` | `47` | `47` | `0` | Değişiklik yok `[ölçüldü: diff -u before after]` |
| `ba62f5f8..0f3e01af` | `905` | `906` | `+1` | Mevcut testlerde `+0`; eklenen tek satır `tools/codegraph TestUndefinedMessageSlotKeepsStaleGraphAndExitsOnBuild` (aralık `internal/lang/messages/tr.json` dosyasına dokunur) `[ölçüldü: diff -u before after]` |
| `705c16f1..6b767cf5` | `670` | `670` | `0` | Değişiklik yok `[ölçüldü: diff -u before after]` |
| `8ee2111e..52ce4b9f` | `306` | `310` | `+4` | Aralık `tools/codegraph/README.md` yanında `tools/codegraph/*.go` dosyalarına da dokunduğu için `TestReadmeGraphSizeIsCurrent` zaten tabandaki `306` satırın içindeydi (`+0` mevcut test); `+4` satırın tamamı bu dalda `tools/codegraph` paketine eklenen 4 yeni koruyan testtir `[ölçüldü: diff -u before after]` |

- **Son 60 commit (`main` geriye doğru):** Madde 1, `tools/codegraph TestPlanningQuestionsAreAnswered` testini tam **4** commit aralığına ekledi: `01d168e6..6535ac87`, `6c1291d2..5d880530`, `52d58950..d1af3110`, `62b5e07f..8e58eb05` (yönergedeki DENETİM 130 ölçümüyle bire bir aynı) `[ölçüldü: runQueryOn changed --go-tests son 60 commit karşılaştırması, gainedExam=4]`.
- **Çalışma sırasında düşen testler:** Yalnız beklenen `TestExtractorRevisionFollowsExtractorCode` (`extractorRevision` ve `testdata/extractor_digest.txt` güncellendi), Madde 3 sonrasında `tools/codegraph/README.md` bağlanınca bayat istisnayı yakalayan `TestEveryTrackedDataFileHasATestOrAReason` (`testdata/uncovered_data.txt` ve `maxUncoveredData` güncellendi) ve dal kökündeki geçici `UZAK3_DURUM.md` için aynı veri kapsamı testi düştü; beklenmedik hiçbir test düşmedi `[görüş]`.

---

## Bitti Ölçütü Komutları ve Son Satırları

1. `go run ./tools/codegraph tests-for tools/codegraph/README.md` (`<- tests (1):` altında `codegraph.TestReadmeGraphSizeIsCurrent` ve `<- exam questions that name it` altında `codegraph.TestPlanningQuestionsAreAnswered` görünür; çıkış `0`):
   - Son satır: `    *.json, *.md, *.tr under the repository (tools/codegraph/data_coverage_test.go:61): codegraph.TestEveryTrackedDataFileHasATestOrAReason  [INFERRED]` `[ölçüldü: go run ./tools/codegraph tests-for tools/codegraph/README.md]`
2. `go run ./tools/codegraph tests-for internal/lang/interp/compiler_expr.go` (`<- exam questions that name it (1; questions are plain text in tools/codegraph/testdata/planning_questions.txt):` başlığı altında `codegraph.TestPlanningQuestionsAreAnswered` görünür; çıkış `0`):
   - Son satır: `    ... 76 more (--all lists them)` `[ölçüldü: go run ./tools/codegraph tests-for internal/lang/interp/compiler_expr.go]`
3. `go build ./... && go vet ./...` (çıkış `0`):
   - Son satır: `(boş çıktı, çıkış kodu 0)` `[ölçüldü: go build ./... && go vet ./...]`
4. `go test -short -timeout 1200s ./tools/codegraph/... ./tools/edithook/...`:
   - Geçici dal dosyası (`UZAK3_DURUM.md`) çıkarılmış temiz birleştirme ağacında (çıkış `0`):
     - `tools/codegraph`: `ok  	github.com/sekerahmet/TCB/tools/codegraph	211.294s`
     - Son satır: `ok  	github.com/sekerahmet/TCB/tools/edithook	0.060s` `[ölçüldü: temiz klonda go test -short -timeout 1200s ./tools/codegraph/... ./tools/edithook/...]`
   - Ham dal ağacında yalnız `TestEveryTrackedDataFileHasATestOrAReason` kökteki `UZAK3_DURUM.md` (ve bu rapor eklendiğinde `UZAK3_RAPORU.md`) yüzünden düşer (`UZAK_YONERGE_3.md:13` uyarınca teste dokunulmadı) `[ölçüldü: go test -short ./tools/codegraph -run '^TestEveryTrackedDataFileHasATestOrAReason$', FAIL UZAK3_DURUM.md]`.
5. Koruyan testler `git clone --depth 1` ile alınmış temiz kopyada (`git rev-list --count HEAD` = `1`, `go test -timeout 300s ./tools/codegraph -run '^(TestUndefinedMessageSlotKeepsStaleGraphAndExitsOnBuild|TestBareReadCallsAndSlashTailComments|TestEveryNodeTheExamNamesIsLinkedFromTheExam|TestExamSelectionOnModifyRenameDeleteAndStamp)$'`, çıkış `0`):
   - Son satır: `ok  	github.com/sekerahmet/TCB/tools/codegraph	8.546s` `[ölçüldü: git clone --depth 1 kopyasında koruyan testler]`
6. `go run ./tools/codegraph spec-refs` (çıkış `0`, `0 note(s)`):
   - Son satır: `301 item(s), 5647 mention(s), 0 error(s), 0 note(s)` `[ölçüldü: go run ./tools/codegraph spec-refs]`
7. `gofmt -l cmd internal tools lib` (çıkış `0`):
   - Son satır: `(boş çıktı, çıkış kodu 0)` `[ölçüldü: gofmt -l cmd internal tools lib]`

---

## Ortam Farkları (Temiz Linux Sanal Makine Bulguları)

1. **Go kurulu değildi:** Sanal makinede (`Debian 12`, `linux/amd64`) öntanımlı `go` ikilisi yoktu `[ölçüldü: which go, çıkış 1]`; `go.mod` sürümü olan `go1.26.10` kullanıcı alanına (`/home/antigravity/.local/go`) indirildi ve `GOTOOLCHAIN=local` ile kullanıldı `[ölçüldü: go version, go version go1.26.10 linux/amd64]`.
2. **Tek dal refspec ve sığ klon (`200` commit):** Başlangıç klonunda `remote.origin.fetch` yalnız `main` dalını çekiyordu ve depo sığ klondu (`git rev-parse --is-shallow-repository` → `true`); `origin/yonerge` dalını okumak için refspec genişletildi. Sığ klonda `TestPlanningQuestionsAreAnswered`, tam tarihçe isteyen 7 soruyu (`79`, `85`, `180`, `193`, `194`, `241`, `290`) tasarlandığı üzere `skipped, the clone has no history` ile atlayıp geçiyor `[ölçüldü: go test -v ./tools/codegraph -run TestPlanningQuestionsAreAnswered]`.
3. **`/usr/bin/time` yok:** Harici `time` ikilisi kurulu değildi `[ölçüldü: which time, çıkış 1]`; süreler `codegraph`'ın iç sayacı ve `date +%s%3N` ile ölçüldü.
4. **Sanal makine gRPC kesintileri ve ~`300s` komut akışı sınırı:** Oturum boyunca sanal makine dört kez (`09:14 +03`, `10:16 +03`, `10:49 +03`, `11:18 +03`) kısa süreli duraklama/`RPC::UNREACHABLE` ile yeniden bağlandı (`/home/antigravity` korundu) ve tek bir `run_command` çağrısı ~`300s` sınırını aşınca `generic::deadline_exceeded` döndü; uzun doğrulamalar tek süreçte `runQueryOn` kullanılarak ya da ayrı adımlara bölünerek koşuldu.

---

## Takıldığım Yerler

1. **Çıplak joker dizgiler (`*.go`):** Madde 3'te okuma çağrılarındaki çıplak dizgileri bağlarken `reporoot.Glob("*.go")` benzeri joker kalıpların `pathRef.local` yoluna girmemesi için `resolveTestBareReads` içinde `*?[` karakterleri dışlandı (`tools/codegraph/test_paths_param.go:93`).
2. **İzlenmeyen yeni `.go` dosyası ve `extractor_digest.txt`:** `TestExtractorRevisionFollowsExtractorCode` testi dosyaları `reporoot.ReadDir(".")` (`git ls-files`) ile listelediği için yeni oluşturulan `tools/codegraph/exam_links.go` `git add` yapılana kadar özet hesabına girmedi; dosya `git add` ile sahneye alındıktan sonra `testdata/extractor_digest.txt` yeniden üretildi.
3. **`parseExamWords` içinde noktalama kırpma tekilleştirmesi:** İlk taslakta `row = append(row, w, t)` satırı noktalama taşımayan sözcüklerde (`t == w`) aynı sözcüğü iki kez ekleyerek kenarın `Count` değerini ikiye katlıyordu (`x8` yerine `x16`); `slices.Compact([]string{w, t})` (`tools/codegraph/exam_links.go:51`) ile düzeltildi.
4. **`deletedHunkNamesExam` içinde `.md` düzyazısı ve Go paket adı:** İlk benzetimde `.md` belgelerinin silinen satırlarındaki Türkçe düzyazı sözcükleri (`toplam`) ve Go `package` düğümünün `n.Name` alanının tam göreli yol (`internal/lang/interp`) olması yanlış eşleşme üretti; `.md` silinen satırları dışlanarak ve Go dosyalarında paket kısa adı `path.Base(path.Dir(h.file))` ile alınarak (`tools/codegraph/exam_links.go:139-141`) giderildi.

---

## Emin Olmadıklarım ve Bakmadıklarım

- **Emin olmadıklarım:** Yok.
- **Bakmadıklarım:** Yönergenin "Dokunma" listesindeki paketlere (`internal/lang/*`, `internal/stdlib/*`, `internal/compute/*`, `docs/spec/*`, `docs/status.md`, `tools/full_check.sh`, `lib/*`) dokunulmadı; tam ağaç testi yerine yönergenin bitti ölçütündeki `./tools/codegraph/... ./tools/edithook/...` paketleri ve `go build ./... && go vet ./...` koşuldu.

---

## SORU

Yok.

---

## Codegraph

Yönergede kapatılan üç madde dışında yeni bir yanlış ya da eksik `codegraph` cevabı görülmedi (`yok`); Madde 3'ün iki `//` satırına bölünen yol kuyruğu sınırı (`// main.go`) yönerge gereği `tools/codegraph/gaps.md:57` satırına eklendi `[kanıtlı: tools/codegraph/gaps.md:57, "Yorumda satır sonunda bölünen yolun kuyruğu (`// .../ \n // main.go`) çıplak ad gibi `cites` kurabilir (2026-11-05, DENETİM istek 102)."]`.

---

## Kendi kontrolüm

- `UZAK3_RAPORU.md` içindeki **14** birebir `[kanıtlı: ...]` alıntısının tamamı kaynak dosyalarında hem `grep -F` komutuyla hem de belirtilen satır numarasıyla karakter karakter karşılaştırılarak doğrulandı; **26** benzersiz dosyadaki toplam **51** `dosya:satır` başvurusu (ve `UZAK3_DURUM.md` içindeki **39** `dosya:satır` başvurusu) yeniden açılıp okundu.
- Kontrol sırasında **1** alıntıda (`tools/codegraph/gaps.md:57`) baştaki `-` karakteri düz `grep -F` çağrısında bayrak sayılmaması için kırpıldı ve **1** satır başvurusu (`tools/codegraph/README.md:274` → `tools/codegraph/README.md:41,169`) düzeltildi; **0** alıntı ya da bulgu silindi.

Kendi incelemem bitti.
