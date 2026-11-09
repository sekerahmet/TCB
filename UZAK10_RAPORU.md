# Uzak İş 10 Raporu — `codegraph`: İki Yanlış "Yok" ve Dört Dar Eksik

## 1. Özet ve Kimlik Bilgileri

- **Dal:** `gemini/uzak-10`
- **Taban commit (`main`):** `6311e11acdfb1cad05521aede8b76ceced563769`
- **Son kod commit'i (`2f`):** `cdc9afe12ca50c3f128b663331d4befa5f2d5ac6`
- **Başlama saati:** `2026-11-09 11:03:34 +0300` (`2026-11-09 08:03:34 UTC`)
- **Bitiş saati:** `2026-11-09 13:27:00 +0300` (`2026-11-09 10:27:00 UTC`)
- **Bağlam sıkıştırması:** 4 kez gerçekleşti:
  1. `2026-11-09 11:29:34 +0300` (`2026-11-09 08:29:34 UTC`): Adım 1 tamamlandıktan sonra, Adım 2a'ya başlamadan önce; `CLAUDE.md`, `UZAK_YONERGE_10.md` ve `UZAK10_DURUM.md` yeniden okundu.
  2. `2026-11-09 12:04:29 +0300` (`2026-11-09 09:04:29 UTC`): Adım 2b çalışması sırasında (uzak uVM duraklatılıp yeniden başlatıldı); `CLAUDE.md`, `UZAK_YONERGE_10.md` ve `UZAK10_DURUM.md` yeniden okundu.
  3. `2026-11-09 12:35:19 +0300` (`2026-11-09 09:35:19 UTC`): Adım 2c düzenlemesi sırasında; `CLAUDE.md`, `UZAK_YONERGE_10.md` ve `UZAK10_DURUM.md` yeniden okundu.
  4. `2026-11-09 13:13:12 +0300` (`2026-11-09 10:13:12 UTC`): Adım 3 ve Bitti ölçütü komutları koşulduktan sonra, rapor yazılmadan hemen önce; `CLAUDE.md`, `UZAK_YONERGE_10.md` ve `UZAK10_DURUM.md` yeniden okundu, tüm sayılar ve `dosya:satır` kaynakları yeniden açılıp doğrulandı.

| Adım | Commit | Konu | Bütçe (üretim) | Gerçek (üretim net) | Koruyan test |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **2a** | `2d23efc24735` | İzlenmeyen yeni `.go` dosyası `dups --changed`'e girer | 15–40 (5–20) | **+20** satır (+18 kod, +2 yorum) | `TestDupsChangedAndChangedSeeUntrackedGoFile` (`tools/codegraph/changed_test.go:277`) |
| **2b** | `00f701398bc5` | Modül dışı arayüzün uygulaması teste bağlanır | 40–90 | **+67** satır (`stdlib_dispatch.go` +66, `gocode_scan.go` +1, `test_reach.go` 0 net, `graph.go` 0 net) | `TestExternalInterfaceFieldLinksOnlyImplementedMethodsToTests` (`tools/codegraph/implementations_test.go:241`) |
| **2c** | `5f17dfc9e8ab` | Belge (`.md`) dosyasının `run:` satırı ve altın dosya uyarısı | 20–50 | **+26** satır (`query_run_line.go` +23, `query_tests.go` +3) | `TestDocFileRunLineIncludesReadersAndGoldenScans` (`tools/codegraph/query_run_line_test.go:207`) |
| **2d** | `e6b22af19be9` | Eksik tanımlı kodun `trSkipped` uyarısında adı ve çaresi | 10–25 | **+2** satır (`main_query.go` +3/-1) | `TestMissingCodeDefinitionInLanguageFileNamesCodeAndRemedy` (`tools/codegraph/main_query_test.go:34`) |
| **2e** | `8455faed83de` | Atası olan commit'te `changed --go-tests` açıklaması | 5–15 | **+8** satır (`changed.go` +7 kod, +1 boş) | `TestChangedGoTestsAncestorCommitExplainsRange` (`tools/codegraph/changed_test.go:306`) |
| **2f** | `cdc9afe12ca5` | Uzak 8'in iki kuralının fikstürü | Yalnız test | **0** satır (`query_names_test.go` +63/-33 test) | `TestAdjacentNameCodeInCallIsImplementedByArgumentCallable` (`tools/codegraph/query_names_test.go:132`), `TestTableRowNameCodeIsImplementedByRowCallable` (`tools/codegraph/query_names_test.go:212`) |

---

## 2. Bulgular (Önem Sırasıyla)

### Bulgu 1: `dups --changed` izlenmeyen yeni `.go` dosyalarını hem değişen satırlarda hem de kopya derleminde görmüyordu (Adım 1.1 ve 2a)

- **İddia:** `git add` yapılmamış yeni bir `.go` dosyasında birebir kopya bir işlev varken `go run ./tools/codegraph dups --changed main` komutu yeni dosyayı ne değişen satır kümesine (`changedGoLines`) ne de kopya derlemine (`loadCorpus`) dahil ediyor ve `0 duplicate candidate(s)` döndürüyordu.
- **Kanıt:**
  - `[ölçüldü: GOTOOLCHAIN=local go run ./tools/codegraph dups --changed main]` `internal/lang/interp/operators.go:397-486` (`valuesEqual`) izlenmeyen geçici bir `.go` dosyasına (`valuesEqualCopy`) kopyalandığında tabanda çıktı:
    `0 duplicate candidate(s) touching main...HEAD (0 shown; >= 100 nodes; 1341 files, 1156119 nodes, 1.3s)`
  - `[kod: tools/codegraph/dups_changed.go:44]` `func changedGoLines(root, base, ref string) (changedLines, string, error) {` yalnız `changeHunks` (`git diff`) çıktısını okuyordu; `git diff` izlenmeyen dosyaları listelemez.
  - `[kod: tools/codegraph/dups_corpus.go:60]` `func loadCorpus(root string) (co *dupCorpus, skipped []string, err error) {` yalnız `walkFiles(root, ".go")` (`codeindex.TrackedFiles`, yani `git ls-files -z`) çağırdığı için izlenmeyen `.go` dosyası 1341 dosyalık derleme hiç girmiyordu.
  - `[test: tools/codegraph/changed_test.go:277]` `TestDupsChangedAndChangedSeeUntrackedGoFile` ile sabitlendi.
- **Öneri:** Canlı kolda (`tools/codegraph/dups_changed.go:55-61`) `untrackedGoFiles(root)` ile bulunan izlenmeyen `.go` dosyalarının satır aralığı (`[1, math.MaxInt]`) `changedGoLines`'a ve dosya yolları `loadCorpus`'a (`tools/codegraph/dups_corpus.go:60`) eklendi; aralığın etiketi `" + " + countOf(len(untracked), "untracked file")` olarak güncellendi (`changeHunks` değiştirilmedi).

### Bulgu 2: Modül dışı arayüz yerine verilen somut tiplerin yöntemleri test erişilebilirliğine bağlanmıyordu (Adım 1.2 ve 2b)

- **İddia:** Modülün somut bir tipi (`goload.importerFunc`), modülün içe aldığı bir paketin adlı ve boş olmayan arayüzünü (`go/types.Importer`) sağlayıp yapı alanı, çağrı argümanı ya da atama yoluyla o arayüz tipindeki bir yere verildiğinde `tests-for` bu tipin yöntemine (`goload.importerFunc.Import`) giden yolu görmüyor ve `no tests found` diyordu.
- **Kanıt:**
  - `[ölçüldü: GOTOOLCHAIN=local go run ./tools/codegraph tests-for goload.importerFunc.Import]` Tabanda çıktı:
    `goload.importerFunc.Import [method] internal/devtools/goload/goload.go:228` ve altında `  no tests found`.
  - `[kod: internal/devtools/goload/goload.go:117]` `u, err := m.check(p.ImportPath, p.Dir, p.GoFiles, imp, opts)` ve `[kod: internal/devtools/goload/goload.go:187]` `x, err = m.check(p.ImportPath+"_test", p.Dir, p.XTestGoFiles, withTests, opts)` çağrılarında `importerFunc` somut tipi `imp types.Importer` parametresine verilir; `[kod: internal/devtools/goload/goload.go:218]` `pkg, err := (&types.Config{Importer: imp}).Check(path, m.Fset, u.Files, u.Info)` ise `imp`'i kullanır.
  - `[kod: tools/codegraph/implementations.go:22]` `func moduleImplementations(m *goload.Module) (concrete, ifaces []*types.Named, pairs []implPair) {` yalnız modül içi arayüzleri eşleştiriyor; `[kod: tools/codegraph/test_reach.go:26]` `reachEdge` ise `edgeStdlibCalls` kenarlarını izlemiyordu.
  - `[test: tools/codegraph/implementations_test.go:241]` `TestExternalInterfaceFieldLinksOnlyImplementedMethodsToTests` ile sabitlendi.
- **Öneri:** `tools/codegraph/stdlib_dispatch.go:108-172` içinde `externalIfaceSlot` ve `externalIfaceNode` ile yapı alanı (`CompositeLit`), çağrı argümanı (`CallExpr`, açık dönüşümler hariç) ve atama (`AssignStmt`, `_` olmayan `ValueSpec`) yerlerinde modülün içe aldığı paketlerin (`named.Obj().Pkg() != nil && !x.inModule(named.Obj().Pkg().Path())`) adlı, boş olmayan arayüzlerini sağlayan somut tiplerin yalnız arayüz yöntemlerine `stdlib_calls` (`INFERRED`) kenarı eklendi ve `tools/codegraph/test_reach.go:26`'da `edgeStdlibCalls` erişilebilirliğe açıldı.

### Bulgu 3: Belge (`.md`) dosyaları için `tests-for` bir `run:` satırı ve altın dosya uyarısı vermiyordu (Adım 1.3 ve 2c)

- **İddia:** `tests-for` bir `.go` dosyası için hızlı koşulacak `run:` satırlarını verirken `.md` belge dosyaları için `run:` satırı üretmiyor ve altın dosya testi (`TestDottedMemberSpellingsGolden`) seçildiğinde `-update` uyarısı yazmıyordu.
- **Kanıt:**
  - `[ölçüldü: GOTOOLCHAIN=local go run ./tools/codegraph tests-for docs/status.md]` Tabanda `docs/status.md [doc] docs/status.md:1` altında `run:` satırı yoktu; doğrudan `<- tests (3):` başlıyordu.
  - `[kod: tools/codegraph/query_tests.go:240]` `printTestsFor` yalnız `.go` dosyalarında (`tools/codegraph/query_tests.go:243` `ix.printGoFileTests(w, n, limit)`) `runPlanFor` çağırıyor, `kindDoc` düğümlerinde çağırmıyordu.
  - `[test: tools/codegraph/query_run_line_test.go:207]` `TestDocFileRunLineIncludesReadersAndGoldenScans` ile sabitlendi.
- **Öneri:** `tools/codegraph/query_run_line.go:90-96` (`docRunPlan`) ve `tools/codegraph/query_tests.go:251-253` ile `.md` (`kindDoc`) düğümlerinde belgeyi adıyla okuyan (`splitSharedSites`), konu edinen (`t.about`) ve tarayan (`scansCovering`, `TestDottedMemberSpellingsGolden` dahil) testlerden `run:` satırı üretildi; `p.golden` doluysa `run:` satırının altına `"  golden files (%s): if they change, rewrite with -update and read the diff"` satırı eklendi.

### Bulgu 4: `tr.json`'da olup `definitions.json`'da olmayan kodda `trSkipped` uyarısı ikinci satırdaki ayrıntıyı kesiyordu (Adım 1.4 ve 2d)

- **İddia:** `internal/lang/codes/tr.json`'a geçici bir kod yazılıp `definitions.json`'a yazılmadığında `codes.LoadLanguage("tr")` iki satırlık bir hata döndürüyor (`code list tr:\n  ... invalid code "...": not an identity in the code list`), ancak `loadGraph` ilk `\n` karakterinden sonrasını kestiği için kullanıcıya yalnız `code list tr:` gösteriliyordu.
- **Kanıt:**
  - `[ölçüldü: GOTOOLCHAIN=local go run ./tools/codegraph tests-for internal/lang/codes/tr.json]` Tabanda çıktı:
    `codegraph: warning: answers about .tr code are missing (tr sources skipped: TR'de bir iç hata oluştu (bu sizin programınızın değil, TR'nin hatası); lütfen bildirin: code list tr:)`
  - `[kod: internal/lang/codes/code.go:128]` `return None, messages.Internalf("invalid code %q: not an identity in the code list (e.g. TR_fn_print)", s)` ikinci satırda eksik kodu adlandırır; `[kod: internal/lang/codes/definition_check.go:13]` `func CheckDefinitions(d *Language) []error {` ise `*Language` nesnesi ister ve `LoadLanguage` tamamlanmadan çağrılamaz.
  - `[kod: tools/codegraph/main_query.go:118]` Tabandaki `w, _, _ = strings.Cut(w, "\n")` satırı ikinci satırı siliyordu.
  - `[test: tools/codegraph/main_query_test.go:34]` `TestMissingCodeDefinitionInLanguageFileNamesCodeAndRemedy` ile sabitlendi.
- **Öneri:** `tools/codegraph/main_query.go:117-120`'de `strings.Cut(w, "\n")` kaldırıldı; uyarıda `"not an identity in the code list"` geçtiğinde sonuna `" (in internal/lang/codes/tr.json: add it to definitions.json and run go generate ./internal/lang/codes)"` eklendi.

### Bulgu 5: `changed --go-tests <c>` ana dalın atası olan bir commit verildiğinde sessiz boş dönüyordu (Adım 1.5 ve 2e)

- **İddia:** `changed --go-tests <c>` komutuna `main`'in atası olan tek bir commit (`dcbe037`) verildiğinde üç noktalı fark (`git diff main...dcbe037`) boş olduğu için araç hiçbir çıktı üretmeden `0` koduyla çıkıyordu.
- **Kanıt:**
  - `[ölçüldü: GOTOOLCHAIN=local go run ./tools/codegraph changed --go-tests dcbe037]` Tabanda 0 satır çıktı (`exit=0`).
  - `[kod: tools/codegraph/changed.go:52]` `if o.goTests {` kolu doğrudan `ix.printGoTests(stdout, main)` çağırıp boş `touchSet` için hiçbir şey yazmıyordu.
  - `[test: tools/codegraph/changed_test.go:306]` `TestChangedGoTestsAncestorCommitExplainsRange` ile sabitlendi.
- **Öneri:** `tools/codegraph/changed.go:53-55`'te `len(hs) == 0 && len(untracked) == 0 && flag != "" && ref != "HEAD" && ref != o.base && isAncestor(o.root, ref, o.base)` koşulunda `"%s is an ancestor of %s (git diff %s is empty); for a single commit's change, use \`%s~1..%s\`\n"` açıklaması yazdırıldı.

### Bulgu 6: Uzak 8'in iki kuralı (`unit(v, codes.X, (*T).M, f)` ve modül dışı `math.Floor`) fikstürde tam sabitlenmemişti (Adım 1.6 ve 2f)

- **İddia:** `tools/codegraph/query_names_test.go`'da gösterici alıcılı metot ifadesi ve alıcı argümanıyla `unit(&b, codes.FnE, (*box).ptrUnit, fnUnit)` biçimi, modül dışı `math.Floor` işlevinin paket işleviyle birlikte (`pairMath(codes.FnF, fnMath, math.Floor)`) ve tablo satırında (`noteRowNameImpl`) kullanımı ile `math.Floor` için graf düğümü oluşturulmaması fikstürle sabitlenmemişti.
- **Kanıt:**
  - `[kod: tools/codegraph/query_names_test.go:66]` `checkNameImplModule` `g.Nodes` içinde `math.Floor` düğümü bulunmadığını denetlemiyordu ve `TestTableRowNameCodeIsImplementedByRowCallable` (`:212`) `(*box).ptrUnit` ile `math.Floor` satırlarını içermiyordu.
  - `[test: tools/codegraph/query_names_test.go:132]` ve `[test: tools/codegraph/query_names_test.go:212]` ile sabitlendi.
- **Öneri:** Üretim koduna dokunmadan her iki test fikstürüne `unit(&b, codes.FnE, (*box).ptrUnit, fnUnit)`, `pairMath(codes.FnF, fnMath, math.Floor)`, `unitMath(&b, codes.FnF, (*box).exprMath, math.Floor)` ve tablo satırı karşılıkları (`unitRow`, `mathRow`) ile `g.Nodes` üzerinde `math.Floor` düğümü olmama denetimi eklendi.

### Bulgu 7 (Eskimiş Belge): `tools/codegraph/docs.go:15` yorumu `codeindex.ListFiles` için `"izlenmeyen yeni dosyalar dahil"` diyordu

- **İddia:** `tools/codegraph/docs.go:15`'teki yorum `walkFiles`'ın çağırdığı `codeindex.ListFiles(..., codeindex.TrackedFiles, ...)` için `"git'in gördüğü dosyalar, izlenmeyen yeni dosyalar dahil"` diyordu; oysa `TrackedFiles` yalnız `git ls-files -z` ile izlenen dosyaları döndürür.
- **Kanıt:**
  - `[kod: internal/devtools/codeindex/text.go:313-316]` `// TrackedFiles depo araçlarınındır (codegraph, renumber, yeniden adlandırmanın metin taraması): git'in izlediği` / `// dosyalar; git yoksa klasör gezintisi. Depo kökündeki izlenmeyen dosyalar kullanıcınındır, grafiğe ve yazılan` / `// plana girmez.`
  - `[kod: tools/codegraph/docs.go:14-15]` Yorum Adım 2a'da (`2d23efc`) `// araçlarının tek listesi (codeindex.ListFiles: git'in izlediği dosyalar, git yoksa klasör gezintisi; tek atlama listesi).` olarak düzeltildi.
- **Öneri:** `tools/codegraph/docs.go:15` düzeltildi; başka işlem gerekmiyor.

---

## 3. Adım 1 ve Adım 3: Önce ve Sonra Çıktıları

### 3.1 Altı Maddenin Önce ve Sonra Karşılaştırması

| Madde | Komut / Senaryo | Önce (`main` `6311e11`) | Sonra (`HEAD` `cdc9afe`) |
| :--- | :--- | :--- | :--- |
| **1.1** | İzlenmeyen yeni `.go` dosyasında (`valuesEqualCopy`, 3–92. satırlar) kopya işlev varken `go run ./tools/codegraph dups --changed main` | `0 duplicate candidate(s) touching main...HEAD (0 shown; >= 100 nodes; 1341 files, 1156119 nodes, 1.3s)` | `1 duplicate candidate(s) touching main...HEAD + 1 untracked file (1 shown; >= 100 nodes; 1342 files, 1158173 nodes, 1.6s)` (`valuesEqualCopy  [changed]`) |
| **1.2** | `go run ./tools/codegraph tests-for goload.importerFunc.Import` | `goload.importerFunc.Import [method] internal/devtools/goload/goload.go:228` → `  no tests found` | `<- tests, indirect (96, through calls or loaded files, up to 8 steps and 1 package boundary):` (ilk satır: `goload.TestLoadChecksPackagesAndTests [func] internal/devtools/goload/goload_test.go:38  2 steps via goload.Load  [INFERRED]`), `<- weak evidence (100; ...)`, `rules guarded by these tests (1):` |
| **1.3** | `go run ./tools/codegraph tests-for docs/status.md` | `docs/status.md [doc] docs/status.md:1` altında `run:` satırı yok; doğrudan `<- tests (3):` | `run: go test -short ./internal/ ./internal/devtools/testrunner/ ./internal/lang/source/ ./tools/codegraph/ -run '^(TestClassedIdentitiesAreLive\|TestDocCodeBlocksShow\|TestDottedMemberSpellingsGolden\|TestEveryTrackedDataFileHasATestOrAReason\|TestEveryTrackedFileIsANode\|TestIdentityExceptionsAreNotStale\|TestLayoutEnglishFileNames\|TestNoOldCodeForm\|TestNoOldCommandName\|TestNoReferenceToTheOldRoadmapNumbers\|TestRetiredTermsStayRetired\|TestReviewedSectionsHoldOnlyRules\|TestReviewedSectionsWriteIdentities)$'` ve altında `  golden files (TestDottedMemberSpellingsGolden): if they change, rewrite with -update and read the diff` |
| **1.4** | `internal/lang/codes/tr.json`'da `definitions.json`'da olmayan geçici `<gecici_kod>` varken `go run ./tools/codegraph tests-for internal/lang/codes/tr.json` | `codegraph: warning: answers about .tr code are missing (tr sources skipped: TR'de bir iç hata oluştu (bu sizin programınızın değil, TR'nin hatası); lütfen bildirin: code list tr:)` | `codegraph: warning: answers about .tr code are missing (tr sources skipped: TR'de bir iç hata oluştu (bu sizin programınızın değil, TR'nin hatası); lütfen bildirin: code list tr:` / `  TR'de bir iç hata oluştu (bu sizin programınızın değil, TR'nin hatası); lütfen bildirin: invalid code "<gecici_kod>": not an identity in the code list (e.g. TR_fn_print) (in internal/lang/codes/tr.json: add it to definitions.json and run go generate ./internal/lang/codes))` |
| **1.5** | `go run ./tools/codegraph changed --go-tests dcbe037` | 0 satır (sessiz boş çıktı, `exit=0`) | `dcbe037 is an ancestor of main (git diff main...dcbe037 is empty); for a single commit's change, use \`dcbe037~1..dcbe037\`` |
| **1.6** | `go test ./tools/codegraph -run 'TestAdjacentNameCodeInCallIsImplementedByArgumentCallable\|TestTableRowNameCodeIsImplementedByRowCallable'` | Gösterici alıcılı `unit(&b, codes.FnE, (*box).ptrUnit, fnUnit)`, `pairMath(codes.FnF, fnMath, math.Floor)` ve tablo satırı karşılıkları fikstürde yoktu | Her iki testte `unit(&b, ...)`, `pairMath`, `unitMath`, `unitRow`, `mathRow` ve `math.Floor` düğümsüzlüğü sabitlendi (`ok`) |

### 3.2 Adım 2b: Eklenen `stdlib_calls` Kenarları (Arayüze ve `dosya:satır`'a Göre)

- **Dur ve sor eşiği analizi:**
  - Evren kapsamındaki `error` (`named.Obj().Pkg() == nil`) dahil edildiğinde toplam **+269** kenar ekleniyordu (`error`: 230 [%85.5], `testing.TB`: 22, `io.Writer`: 14, `go/types.Importer`: 2, `io.Reader`: 1, `fmt.Stringer`: 0); hem `error + fmt.Stringer > 1/4` hem de `toplam > 200` eşiği aşıldığı için commit öncesi durulup kullanıcıya soruldu.
  - Kullanıcının onayıyla yönergedeki `"modülün içe aldığı bir paketin (standart kitaplık dahil) adlı arayüzünü sağlıyorsa"` kuralına uygun olarak `named.Obj().Pkg() != nil && !x.inModule(named.Obj().Pkg().Path())` koşulu uygulandı ve evren kapsamındaki `error` dışarıda bırakıldı.
  - Böylece **39 yeni kenar** eklendi (`testing.TB`: 22, `io.Writer`: 14, `go/types.Importer`: 2, `io.Reader`: 1; `error` + `fmt.Stringer`: 0). Tabandaki 10 `stdlib_calls` kenarından 1 tanesi (`internal/lang/messages/span_test.go:43` `errors.Join` üzerindeki `error` kenarı) `error` hariç tutulunca kalktığı için toplam `stdlib_calls` kenar sayısı **10 → 48** oldu (korunan 9 taban kenarı + eklenen 39 yeni kenar = 48).

#### Eklenen 39 `stdlib_calls` Kenarının Tam Listesi

**1. `go/types.Importer` (2 kenar):**
- `internal/devtools/goload/goload.go:117` — `go:internal/devtools/goload.Load` → `go:internal/devtools/goload.importerFunc.Import` (`count: 2`)
- `internal/devtools/goload/goload.go:187` — `go:internal/devtools/goload.Module.checkTests` → `go:internal/devtools/goload.importerFunc.Import`

**2. `io.Reader` (1 kenar):**
- `internal/devtools/debugger/dap_launch.go:65` — `go:internal/devtools/debugger.dapServer.launch` → `go:internal/devtools/debugger.dapNoStdin.Read`

**3. `io.Writer` (14 kenar):**
- `cmd/trlang/e2e/behavior_checkpoint_resume_test.go:122` — `go:cmd/trlang/e2e.startTraining` → `go:internal/testkit.TeeWriter.Write` (`count: 2`)
- `cmd/trlang/e2e/behavior_interrupt_test.go:32` — `go:cmd/trlang/e2e.startInterruptible` → `go:internal/testkit.TeeWriter.Write` (`count: 2`)
- `internal/devtools/debugger/dap_launch.go:63` — `go:internal/devtools/debugger.dapServer.launch` → `go:internal/devtools/debugger.dapOutput.Write` (`count: 2`)
- `internal/devtools/profiler/profile_test.go:54` — `go:internal/devtools/profiler.runThen` → `go:internal/testkit.TeeWriter.Write` (`count: 2`)
- `internal/stdlib/checkpoint/behavior_rollback_test.go:139` — `go:internal/stdlib/checkpoint_test.TestRollbackStopsBeforeWritingWhenAParamIsUnusable` → `go:internal/testkit.TeeWriter.Write` (`count: 2`)
- `internal/stdlib/checkpoint/behavior_security_test.go:134` — `go:internal/stdlib/checkpoint_test.TestDeferredInterruptSavesAtStepBoundary` → `go:internal/testkit.TeeWriter.Write` (`count: 2`)
- `internal/stdlib/checkpoint/behavior_stop_request_test.go:37` — `go:internal/stdlib/checkpoint_test.stopRunClock` → `go:internal/testkit.TeeWriter.Write` (`count: 2`)
- `internal/stdlib/checkpoint/checkpoint_test.go:30` — `go:internal/stdlib/checkpoint_test.run` → `go:internal/testkit.TeeWriter.Write` (`count: 2`)
- `internal/stdlib/checkpoint/checkpoint_test.go:231` — `go:internal/stdlib/checkpoint_test.runHooked` → `go:internal/testkit.TeeWriter.Write` (`count: 2`)
- `internal/stdlib/checkpoint/checkpoint_test.go:295` — `go:internal/stdlib/checkpoint_test.TestRunFolderOnlyInTheWorkingDirectory` → `go:internal/testkit.TeeWriter.Write` (`count: 2`)
- `internal/stdlib/logging/log_test.go:27` — `go:internal/stdlib/logging_test.newVM` → `go:internal/testkit.TeeWriter.Write` (`count: 2`)
- `internal/stdlib/process/run_process.go:67` — `go:internal/stdlib/process.State.runProcess` → `go:internal/stdlib/process.boundedOutput.Write`
- `internal/stdlib/process/run_process.go:69` — `go:internal/stdlib/process.State.runProcess` → `go:internal/stdlib/process.stderrTail.Write`
- `internal/stdlib/process/spawn_process.go:92` — `go:internal/stdlib/process.Process.start` → `go:internal/stdlib/process.stderrTail.Write`

**4. `testing.TB` (22 kenar, `internal/testkit` içindeki 9 test/yardımcı işlevde):**
- `internal/testkit/format_names_test.go:17` — `go:internal/testkit.TestFormatNames` → `go:internal/testkit.recorder.Errorf`, `go:internal/testkit.recorder.Helper` (2 kenar)
- `internal/testkit/format_names_test.go:37` — `go:internal/testkit.TestFormatJSONKeys` → `go:internal/testkit.recorder.Errorf`, `go:internal/testkit.recorder.Helper` (2 kenar)
- `internal/testkit/known_list_test.go:22` — `go:internal/testkit.TestCheckKnownListOnlyShrinks` → `go:internal/testkit.recorder.Errorf` (`count: 4`), `go:internal/testkit.recorder.Helper` (`count: 4`) (2 kenar)
- `internal/testkit/lang_file_test.go:45` — `go:internal/testkit.TestAssertSameFactSetReportsOnlyTheDifference` → `go:internal/testkit.fatalRecorder.Fatalf` (`count: 3`), `go:internal/testkit.recorder.Errorf` (`count: 3`), `go:internal/testkit.recorder.Helper` (`count: 3`) (3 kenar)
- `internal/testkit/runlog_test.go:27` — `go:internal/testkit.TestValidLossMetricsRejectsAFractionalStep` → `go:internal/testkit.fatalRecorder.Fatalf`, `go:internal/testkit.recorder.Errorf`, `go:internal/testkit.recorder.Helper` (3 kenar)
- `internal/testkit/runlog_test.go:62` — `go:internal/testkit.failsFatally` → `go:internal/testkit.panickingTB.Errorf`, `go:internal/testkit.panickingTB.Fatal`, `go:internal/testkit.panickingTB.Fatalf`, `go:internal/testkit.panickingTB.Helper` (4 kenar)
- `internal/testkit/testkit_test.go:108` — `go:internal/testkit.TestMustContainReportsEveryMissingPart` → `go:internal/testkit.recorder.Errorf` (`count: 2`), `go:internal/testkit.recorder.Helper` (`count: 2`) (2 kenar)
- `internal/testkit/testkit_test.go:159` — `go:internal/testkit.TestScalesLinearlyFailsQuadraticWork` → `go:internal/testkit.recorder.Errorf`, `go:internal/testkit.recorder.Helper` (2 kenar)
- `internal/testkit/testkit_test.go:191` — `go:internal/testkit.TestScalesLinearlyChecksTheWorkIsDone` → `go:internal/testkit.recorder.Errorf`, `go:internal/testkit.recorder.Helper` (2 kenar)

### 3.3 `untested` Grupları (Önce ve Sonra) ve `tested` Grubuna Geçen 12 İşlev

| Ölçüm | Toplam | `tested` | `subprocess` | `rule` | `measured` | `weak` | `exam` | `untested` |
| :--- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| **Önce (`main` `6311e11`, eski `codegraph`)** | 6864 | 6332 | 92 | 0 | 2 | 332 | 0 | 106 |
| **Sonra (`main` `6311e11` üzerinde yeni `codegraph`)** | 6864 | **6344** (+12) | 92 | 0 | 2 | **321** (-11) | 0 | **105** (-1) |
| **Sonra (`HEAD` `cdc9afe`, +5 yeni `codegraph` yardımcısı dahil)** | 6869 | **6348** (+16) | 92 | 0 | 2 | **321** (-11) | 0 | **106** (0) |

- Not: `HEAD`'de `tools/codegraph`'a eklenen 5 yeni yardımcı işlevden 4'ü (`untrackedGoFiles`, `externalIfaceNode`, `docRunPlan`, `isAncestor`) doğrudan `tested` grubuna girdi; `codegraph.goExtractor.externalIfaceSlot` (`tools/codegraph/stdlib_dispatch.go:108`) ise `buildGraph` girişinden 9 adım derinde olduğu için `TestMaxSteps = 8` (`tools/codegraph/gaps.md:49`) sınırına takılarak `untested` sayıldı (`106 - 1 + 1 = 106`).

#### `untested` / `weak` Grubundan `tested` Grubuna Geçen 12 Mevcut Yöntem (Tam Liste)

1. `goload.importerFunc.Import` (`internal/devtools/goload/goload.go:228`) — **`untested` → `tested`** (`go/types.Importer` alanına/parametresine verilme kenarı ile)
2. `checkpoint.counter.Write` (`internal/stdlib/checkpoint/write.go:213`) — **`weak` → `tested`** (`reachEdge` içinde `edgeStdlibCalls` izlenmesi ile)
3. `debugger.dapOutput.Write` (`internal/devtools/debugger/dap.go:277`) — **`weak` → `tested`** (`io.Writer` yapı alanına verilme kenarı ile)
4. `ml.pcgSource.Int63` (`internal/stdlib/ml/shared_random.go:43`) — **`weak` → `tested`** (`math/rand.Source` üzerinde `edgeStdlibCalls` izlenmesi ile)
5. `ml.pcgSource.Seed` (`internal/stdlib/ml/shared_random.go:46`) — **`weak` → `tested`** (`math/rand.Source` üzerinde `edgeStdlibCalls` izlenmesi ile)
6. `process.boundedOutput.Write` (`internal/stdlib/process/run_process.go:126`) — **`weak` → `tested`** (`io.Writer` atama kenarı ile)
7. `profiler.dropOnError.Write` (`internal/devtools/profiler/capture.go:81`) — **`weak` → `tested`** (`edgeStdlibCalls` izlenmesi ile)
8. `testkit.TeeWriter.Write` (`internal/testkit/testkit.go:192`) — **`weak` → `tested`** (`io.Writer` yapı alanına verilme kenarı ile)
9. `tokenize.candidateHeap.Less` (`internal/compute/tokenize/bpe.go:146`) — **`weak` → `tested`** (`container/heap.Interface` üzerinde `edgeStdlibCalls` izlenmesi ile)
10. `tokenize.candidateHeap.Pop` (`internal/compute/tokenize/bpe.go:154`) — **`weak` → `tested`** (`container/heap.Interface` üzerinde `edgeStdlibCalls` izlenmesi ile)
11. `tokenize.candidateHeap.Push` (`internal/compute/tokenize/bpe.go:153`) — **`weak` → `tested`** (`container/heap.Interface` üzerinde `edgeStdlibCalls` izlenmesi ile)
12. `tokenize.candidateHeap.Swap` (`internal/compute/tokenize/bpe.go:152`) — **`weak` → `tested`** (`container/heap.Interface` üzerinde `edgeStdlibCalls` izlenmesi ile)

### 3.4 Grafın Kurulma Süresi (Önce ve Sonra, 3 Koşunun Ortancası)

| Durum | Graf Boyutu (`build` çıktısı) | 3 Koşu | Ortanca |
| :--- | :--- | :--- | :--- |
| **Önce (`main` `6311e11`)** | `codegraph: 36467 nodes, 306661 edges (191403 EXTRACTED, 115258 INFERRED)` | `13.6s` (ilk koşu), `8.6s`, `8.3s` (sıcak önbellek: `8.6s`, `8.3s`, `8.3s`) | **`8.6s`** (sıcak: `8.3s`) |
| **Sonra (`HEAD` `cdc9afe`)** | `codegraph: 36487 nodes, 306876 edges (191545 EXTRACTED, 115331 INFERRED)` | `9.3s`, `9.3s`, `9.4s` | **`9.3s`** |

---

## 4. Adım 2a–2f Seçimleri, Gerekçeleri ve Üretim Satırları

- **Adım 2a (`2d23efc`, +20 net üretim satırı, bütçe 15–40 / 5–20):**
  - `tools/codegraph/dups_changed.go:55-76`'da `untrackedGoFiles(root)` eklendi ve `changedGoLines` canlı kolunda izlenmeyen `.go` dosyalarının tüm satırları (`[1, math.MaxInt]`) değişen satır sayıldı; etikete `" + " + countOf(len(untracked), "untracked file")` eklendi. `changeHunks`'a dokunulmadı (`changed` izlenmeyen dosyaları zaten `touchUntracked` ile sayıyor).
  - `tools/codegraph/dups_corpus.go:60`'ta `loadCorpus` içine `untrackedGoFiles(root)` eklendi; çünkü `walkFiles` (`codeindex.TrackedFiles`) yalnız `git ls-files -z` çıktısını döndürüyordu.
- **Adım 2b (`00f7013`, +67 net üretim satırı, bütçe 40–90):**
  - `tools/codegraph/stdlib_dispatch.go:108-172`'de `externalIfaceSlot` ve `externalIfaceNode` ile yapı alanı (`CompositeLit`), çağrı argümanı (`CallExpr`, açık dönüşümler hariç) ve atama (`AssignStmt`, `_` olmayan `ValueSpec`) yerlerinde modül dışı (`named.Obj().Pkg() != nil && !x.inModule(named.Obj().Pkg().Path())`) adlı, boş olmayan arayüzlere verilen modül somut tipleri için yalnız arayüzün yöntem kümesindeki yöntemlere `stdlib_calls` (`INFERRED`) kenarı üretildi; `return` ifadeleri dışarıda bırakıldı.
  - `tools/codegraph/test_reach.go:26`'da `reachEdge`'e `edgeStdlibCalls` eklendi. `extractorRevision` `94` → `95` yapıldı (`tools/codegraph/graph.go:25`).
- **Adım 2c (`5f17dfc`, +26 net üretim satırı, bütçe 20–50):**
  - `tools/codegraph/query_run_line.go:37-96`'da `runPlanFor` içinde `p.pkg` yalnız `file.Kind == kindFile` için dolduruldu; `docRunPlan` ile `.md` (`kindDoc`) düğümlerinin doğrudan okuyucuları (`splitSharedSites`), konu testleri (`t.about`) ve taramaları (`scansCovering`) birleştirildi.
  - `p.golden` listesi (`strings.HasSuffix(t.fn, "Golden")`) eklendi; `printRunPlan` (`tools/codegraph/query_run_line.go:164-166`) altın dosya testi seçildiğinde `run:` satırının altına `-update` açıklamasını yazar.
- **Adım 2d (`e6b22af`, +2 net üretim satırı, bütçe 10–25):**
  - `tools/codegraph/main_query.go:117-120`'de `strings.Cut(w, "\n")` kaldırıldı ve `"not an identity in the code list"` içeren `trSkipped` uyarılarına `" (in internal/lang/codes/tr.json: add it to definitions.json and run go generate ./internal/lang/codes)"` eklendi.
- **Adım 2e (`8455fae`, +8 net üretim satırı, bütçe 5–15):**
  - `tools/codegraph/changed.go:53-55` ve `:170-173` (`isAncestor`) ile `changed --go-tests <c>` çağrısında fark boşsa ve `<c>` `o.base`'in atasıysa `<c>~1..<c>` aralığını öneren ileti yazdırıldı.
- **Adım 2f (`cdc9afe`, 0 üretim satırı, yalnız test):**
  - `tools/codegraph/query_names_test.go:66-86`, `:132-205` ve `:212-286`'da gösterici alıcılı metot ifadesiyle `unit(&b, codes.FnE, (*box).ptrUnit, fnUnit)`, `pairMath(codes.FnF, fnMath, math.Floor)` ve tablo satırı karşılıkları (`unitRow`, `mathRow`) ile `math.Floor` düğümsüzlüğü sabitlendi.

---

## 5. Değişen Altın ve Planlama Sınavı Satırları

- **`tools/codegraph/testdata/extractor_digest.txt:1-2` (Adım 2b, `00f7013`):**
  - `revision 94` → `revision 95`
  - `digest 3925fd751678db8b` → `digest bdfac1698c0babf9`
  - **Gerekçe:** `tools/codegraph/stdlib_dispatch.go`, `tools/codegraph/gocode_scan.go` ve `tools/codegraph/graph.go` (`extractorRevision = 95`) extractor kaynak dosyaları arasında olduğu için `TestExtractorRevisionFollowsExtractorCode` (`tools/codegraph/extractor_digest_test.go:73`) özet dosyasının güncellenmesini zorunlu kılar.
- Başka hiçbir altın (`.golden`) ya da planlama sınavı dosyasına dokunulmadı.

---

## 6. Bitti Ölçütü Komutları ve Son Satırları

1. `gofmt -l tools`
   - Çıktı: *(boş, 0 satır, `exit=0`)*
2. `GOTOOLCHAIN=local go test ./tools/codegraph/...`
   - Son satır: `FAIL	github.com/sekerahmet/TCB/tools/codegraph	346.964s`
   - Not: Yönergede belirtildiği üzere yalnız `TestEveryTrackedDataFileHasATestOrAReason` kökteki `UZAK10_DURUM.md` (ve `UZAK10_RAPORU.md`) dosyasını gördüğü için düştü; diğer tüm testler geçti.
3. `GOTOOLCHAIN=local go test ./internal -run 'TestClassedIdentitiesAreLive'`
   - Son satır: `ok  	github.com/sekerahmet/TCB/internal	1.209s`
4. `GOTOOLCHAIN=local go run ./tools/codegraph dups --changed main`
   - Son satır: `0 duplicate candidate(s) touching main...HEAD (0 shown; >= 100 nodes; 1341 files, 1157731 nodes, 1.6s)`
5. `GOTOOLCHAIN=local go run ./tools/codegraph tests-for goload.importerFunc.Import`
   - Son satır: `    Araçların tek dosya listesi \`ListFiles\`, iki kipiyle [rule] internal/devtools/codeindex/README.md:107`
6. `GOTOOLCHAIN=local go run ./tools/codegraph tests-for docs/status.md`
   - Son satır: `    Gözden geçirilmiş bölüm yalnız kuralı yazar [rule] internal/devtools/testrunner/README.md:33`
7. `GOTOOLCHAIN=local go run ./tools/codegraph untested --json`
   - Son satır: `}` (ondan önceki iki satır: `"schema": "codegraph-untested/1",` ve `"total": 6869`)
8. `git diff --stat main...HEAD`
   - Son satır (`UZAK10_DURUM.md` ve `UZAK10_RAPORU.md` commit'inden önce): ` 18 files changed, 532 insertions(+), 74 deletions(-)` (yalnız `tools/codegraph/` altındaki 17 dosya ve kökteki `UZAK10_DURUM.md` / `UZAK10_RAPORU.md`).

---

## 7. Ortam (Temiz Makine Bulguları)

- Uzak Linux ortamında (`Debian 12`, `/usr/local/go/bin/go` `go1.26.2`) ilk `GOTOOLCHAIN=local go run ./tools/codegraph build` koşusu Go modül önbelleği soğuk olduğu için `13.6s` sürdü; sonraki koşular `8.3s–9.4s` aralığında tamamlandı.
- Çalışma sırasında uzak uVM iki kez (`12:04:29 +0300` ve `13:19:14 +0300`) duraklatılıp yeniden başlatıldı (`uVM is in STOPPED state`); her adım hemen commit edilip `origin/gemini/uzak-10` dalına push edildiği için veri kaybı yaşanmadı.
- Yerel makineye veya sabit yola bağımlı olup temiz makinede düşen bir test görülmedi.

---

## 8. Codegraph

- Bu işte düzeltilen 6 madde dışında yeni bir yanlış ya da yanıltıcı `codegraph` cevabı görülmedi.
- Bilinen sınır (`tools/codegraph/gaps.md:49`): `codegraph.goExtractor.externalIfaceSlot` (`tools/codegraph/stdlib_dispatch.go:108`) `buildGraph` girişinden 9 adım derinde olduğu için `TestMaxSteps = 8` sınırına takılarak `untested` listesinde yer alıyor; bu durum `tools/codegraph/gaps.md:49`'da zaten kayıtlıdır.

---

## 9. Emin Olmadıklarım ve Bakmadıklarım

- **Emin olmadıklarım:** Yok (Adım 2b'de evren kapsamındaki `error` arayüzünün hariç tutulması ve Adım 2d'de `internal/lang/codes` paketine dokunulmadan `tools/codegraph/main_query.go` üzerinden uyarı ayrıntısının ve çaresinin verilmesi kullanıcıya sorularak netleştirildi).
- **Bakmadıklarım:** Yönerge gereği `tools/codegraph/` dışındaki paketlerin üretim kodlarına ve `tools/codegraph/gaps.md` dosyasına dokunulmadı.

---

## 10. Hatalarım ve Nedenleri

1. **Adım 1.4 yeniden üretiminde `internal/lang/codes/tr.json` girdi biçimi:**
   - **Hata:** `internal/lang/codes/tr.json`'a geçici kodu ilk denemede `"<gecici_kod>": "gecici_deneme_10"` (düz dizge) olarak ekledim; JSON ayrıştırıcısı `cannot unmarshal string into Go value of type codes.rawEntry` hatası verdi.
   - **Neden:** `internal/lang/codes/tr.json` dosyasını açıp satır yapısına (`{"name": "..."}`) bakmadan bellekten anahtar-değer dizgesi olduğunu varsaydım. Dosya yapısını okuyup `{"name": "gecici_deneme_10"}` yapınca gerçek ileti üretildi.
2. **`UZAK10_DURUM.md` içinde geçici sınıflı kimlik (`TR_fn_...`) yazılması (`internal/old_codes_test.go:90`, `internal/live_identities_test.go:15`):**
   - **Hata:** Adım 1.4'te kullandığım geçici kimlik adını `UZAK10_DURUM.md` içine olduğu gibi yazdığımda `TestClassedIdentitiesAreLive` (`internal/old_codes_test.go:90`) izlenen tüm dosyalarda `classedIdentity` (`internal/live_identities_test.go:15`) düzenli ifadesini taradığı için tanımsız kimlik hatası verdi.
   - **Neden:** `TestClassedIdentitiesAreLive` testinin `.md` durum/rapor dosyalarını da taradığını hesaba katmadım. Durum ve rapor dosyalarında `<gecici_kod>` yer tutucusunu, `tools/codegraph/main_query_test.go:43`'te ise `"TR_fn_" + "temp_missing_def"` birleştirmesini kullanarak düzelttim.
3. **Adım 2a'da `loadCorpus`'un (`tools/codegraph/dups_corpus.go:60`) yalnız izlenen dosyaları okuması:**
   - **Hata:** İlk denemede yalnız `changedGoLines`'a (`tools/codegraph/dups_changed.go:55`) izlenmeyen `.go` dosyalarını ekledim; ancak `dups --changed` yine `0` döndürdü.
   - **Neden:** `tools/codegraph/docs.go:15`'teki eskimiş yorum `walkFiles` (`codeindex.ListFiles`) için `"izlenmeyen yeni dosyalar dahil"` diyordu; `tools/codegraph/dups_corpus.go:60`'ın çağırdığı `walkFiles`'ın `codeindex.TrackedFiles` (`git ls-files -z`) kullandığını kontrol etmeden yoruma güvendim. Hem `loadCorpus`'a `untrackedGoFiles(root)` ekledim hem de `tools/codegraph/docs.go:15`'teki eskimiş yorumu düzelttim.
4. **Adım 2b'de iç içe yapı sabitinde (`CompositeLit`) `n.Type == nil` paniği (`tools/codegraph/stdlib_dispatch.go:140`, `:333`):**
   - **Hata:** `externalIfaceNode` içinde `*ast.CompositeLit` için `typeExprType(fc.u.Info, n.Type)` çağırdığımda `[]struct{...}{{...}}` biçimindeki tipsiz iç yapı sabitlerinde `n.Type == nil` olduğu için `typeExprType` nil gösterici paniği verdi.
   - **Neden:** Go AST'sinde dilim/dizi elemanı olan `CompositeLit` düğümlerinde `Type` alanının `nil` olabileceğini ilk yazarken gözden kaçırdım. `typeExprType` (`tools/codegraph/stdlib_dispatch.go:333`) başına `if e == nil { return nil }` ekleyerek düzelttim.
5. **Adım 2b kenar sayımında Python 3.11 f-string ters eğik çizgi hatası:**
   - **Hata:** Tek satırlık Python ölçüm komutunda f-string süslü parantezi içinde `\"` kullandığım için Python 3.11 `SyntaxError: f-string expression part cannot include a backslash` verdi.
   - **Neden:** Python 3.12 öncesi f-string kısıtını unuttum; değişkeni f-string dışında tanımlayarak düzelttim.
6. **Aynı dosyaya paralel `replace_file_content` çağrısı (`tools/codegraph/changed.go` ve `UZAK10_DURUM.md`):**
   - **Hata:** Adım 2e'de `tools/codegraph/changed.go` üzerinde aynı anda iki paralel `replace_file_content` çağrısı yaptığımda ikinci çağrı birincinin eklediği `isAncestor` tanımını ezdi.
   - **Neden:** Aracın "aynı dosyaya paralel çağrı yapma" kuralını ihlal ettim; ardışık tek çağrıyla düzelttim.
7. **3. sıkıştırma sonrasında `UZAK10_DURUM.md:98`'e ve ilk rapor taslağına bellekten yanlış dosya/satır yazılması:**
   - **Hata:** 3. bağlam sıkıştırmasından sonra `UZAK10_DURUM.md:98`'e `internal/lang/codes/definition_check.go:13` (`CheckDefinitions`) yerine bellekten `gen_codes.go` yazdım; ilk rapor taslağında da 7 `dosya:satır` referansında (`internal/lang/messages/span_test.go:43`, `tools/codegraph/extractor_digest_test.go:73`, `tools/codegraph/dups_changed.go:44`, `tools/codegraph/graph.go:25`, `tools/codegraph/query_tests.go:243` ve `:251-253`, `tools/codegraph/main_query_test.go:43`) satır kayması veya dosya adı farkı vardı.
   - **Neden:** Sıkıştırma sonrası özetten gelen satır numaralarını dosyayı yeniden açmadan yazdım. Kendi kontrolüm aşamasındaki otomatik `dosya:satır` doğrulama betiğiyle yakalayıp hepsini kaynak dosyalarından düzelttim.

---

## 11. Kendi Kontrolüm

- **Alıntı ve `dosya:satır` doğrulaması:**
  - Rapordaki **16 birebir kaynak kod alıntısının** tamamı `grep -F "<alıntı>" <dosya>` komutuyla kaynak dosyalarıyla karakter karakter karşılaştırıldı; **16/16** alıntı birebir eşleşti (0 silinen).
  - Raporda anılan **88 tekil `dosya:satır` referansının** tamamı Python doğrulama betiği ve `sed -n` ile yeniden açıldı; ilk taslakta satır kayması veya dosya adı farkı bulunan **7 referans** (`internal/lang/messages/span_test.go:43`, `tools/codegraph/extractor_digest_test.go:73`, `tools/codegraph/dups_changed.go:44`, `tools/codegraph/graph.go:25`, `tools/codegraph/query_tests.go:243` ve `:251-253`, `tools/codegraph/main_query_test.go:43`) ile `UZAK10_DURUM.md:98`'deki `internal/lang/codes/definition_check.go:13` referansı kaynak satırlarına göre düzeltildi; yeniden koşulan kontrolde **88/88** `dosya:satır` referansı doğrulandı.
- **Sayıların kopyalandığı komut çıktıları:**
  - **Üretim satırı sayıları (`+20`, `+67`, `+26`, `+2`, `+8`, `0`):** `git show --stat 2d23efc`, `git show --stat 00f7013`, `git show --stat 5f17dfc`, `git show --stat e6b22af`, `git show --stat 8455fae`, `git show --stat cdc9afe` ve `git diff main...HEAD -- tools/codegraph/` çıktılarından kopyalandı.
  - **Graf düğüm/kenar sayıları ve kurulum süreleri (`36467 nodes, 306661 edges (191403 EXTRACTED, 115258 INFERRED)`, ortanca `8.6s` → `36487 nodes, 306876 edges (191545 EXTRACTED, 115331 INFERRED)`, ortanca `9.3s`):** `main` (`6311e11`) ve `HEAD` (`cdc9afe`) üzerinde üçer kez koşulan `GOTOOLCHAIN=local go run ./tools/codegraph build` çıktılarından kopyalandı.
  - **Adım 2b `stdlib_calls` kenar sayıları (taban `10`, `error` dahil `279` / `+269`, `error` hariç `48` / `+39` yeni kenar: `testing.TB` `22`, `io.Writer` `14`, `go/types.Importer` `2`, `io.Reader` `1`):** `.codegraph/graph.json` içindeki `"kind": "stdlib_calls"` kayıtlarını sayan ve karşılaştıran `python3` betiğinin çıktısından kopyalandı; her birinin `dosya:satır` yeri kaynak dosyası açılarak doğrulandı.
  - **`untested` grup sayıları (`6864: 6332 tested, 92 subprocess, 0 rule, 2 measured, 332 weak, 0 exam, 106 untested` → `main` üzerinde `6344 tested, 321 weak, 105 untested`, `HEAD` üzerinde `6869: 6348 tested, 92 subprocess, 0 rule, 2 measured, 321 weak, 0 exam, 106 untested`) ve grup değiştiren 12 işlev:** `GOTOOLCHAIN=local go run ./tools/codegraph untested` ve `GOTOOLCHAIN=local go run ./tools/codegraph untested --json` çıktılarının karşılaştırmasından kopyalandı.
  - **Adım 1 / Adım 3 ve Bitti ölçütü çıktıları:** Doğrudan ilgili `GOTOOLCHAIN=local go run ./tools/codegraph ...`, `GOTOOLCHAIN=local go test ...`, `gofmt -l tools` ve `git diff --stat main...HEAD` komutlarının çıktılarından kopyalandı.

Kendi incelemem bitti.

