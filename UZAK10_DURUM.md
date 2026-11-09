# Uzak İş 10 — Durum

- **Dal:** `gemini/uzak-10`
- **Taban commit:** `6311e11acdfb1cad05521aede8b76ceced563769`
- **Başlama saati:** `2026-11-09 11:03:34 +0300` (`2026-11-09 08:03:34 UTC`)
- **Bağlam sıkıştırması:** 2 kez:
  1. `2026-11-09 11:29:34 +0300` (`2026-11-09 08:29:34 UTC`), Adım 1 tamamlandıktan sonra, Adım 2a'ya başlamadan önce; `CLAUDE.md`, `UZAK_YONERGE_10.md` ve `UZAK10_DURUM.md` yeniden okundu.
  2. `2026-11-09 12:04:29 +0300` (`2026-11-09 09:04:29 UTC`), Adım 2b çalışması sırasında (uVM duraklatılıp yeniden başlatıldı); `CLAUDE.md`, `UZAK_YONERGE_10.md` ve `UZAK10_DURUM.md` yeniden okundu.

## İlerleme

- [x] `CLAUDE.md`, `GEMINI_KURALLARI.md` ve `UZAK_YONERGE_10.md` okundu.
- [x] `gemini/uzak-10` dalı `main` (`6311e11acdfb1cad05521aede8b76ceced563769`) ucundan açıldı ve `UZAK10_DURUM.md` oluşturuldu.
- [x] Adım 1: Yeniden üret (1.1–1.6) ve taban ölçümleri (`untested`, graf kurulum süresi)
- [x] Adım 2a: İzlenmeyen yeni dosya `--changed`'e girer (`2d23efc`)
- [ ] Adım 2b: Modül dışı arayüzün uygulaması
- [ ] Adım 2c: Belge dosyasının `run:` satırı
- [ ] Adım 2d: Eksik tanımlı kodun iletisi
- [ ] Adım 2e: Atası olan commit
- [ ] Adım 2f: Uzak 8'in iki kuralının fikstürü
- [ ] Adım 3: Sonra ölçümleri ve Bitti ölçütü
- [ ] Rapor (`UZAK10_RAPORU.md`) ve Kendi kontrolüm

## Adım 1 Bulguları ve Taban Ölçümleri

- **Taban graf ve kurulum süresi (`go run ./tools/codegraph build`):**
  - `codegraph: 36467 nodes, 306661 edges (191403 EXTRACTED, 115258 INFERRED)`
  - 3 koşu: `13.6s` (ilk koşu, modül indirme dahil), `8.6s`, `8.3s` (ortanca `8.6s`; sıcak önbellek 3 koşu `8.6s`, `8.3s`, `8.3s`, ortanca `8.3s`).
- **Taban `untested` (`go run ./tools/codegraph untested`):**
  - `6864 non-test Go functions and methods: 6332 tested, 92 subprocess, 0 rule, 2 measured, 332 weak, 0 exam, 106 untested`
- **1.1 Yeni dosyadaki kopya:**
  - `internal/lang/interp/operators.go:397-486` (`valuesEqual`) izlenmeyen `internal/lang/interp/zz_temp_copy.go` (`valuesEqualCopy`) dosyasına kopyalandığında:
    - `go run ./tools/codegraph dups --changed main` → `0 duplicate candidate(s) touching main...HEAD (0 shown; >= 100 nodes; 1341 files, 1156119 nodes, 1.3s)`
    - `go run ./tools/codegraph changed --go-tests main` → `untracked: internal/lang/interp/zz_temp_copy.go is not tracked by git; repository-wide checks do not see it (run \`git add internal/lang/interp/zz_temp_copy.go\`)` uyarısı ve toplam 37 satır çıktı (`touchUntracked` üzerinden).
    - `git add -N internal/lang/interp/zz_temp_copy.go` sonrası `dups --changed main` → `1 duplicate candidate(s) touching main...HEAD (1 shown; >= 100 nodes; 1342 files, 1156561 nodes, 1.4s)` (`internal/lang/interp/zz_temp_copy.go:3-92  valuesEqualCopy  [changed]`).
    - İzlenen `internal/lang/interp/operators.go` sonuna commit'lenmeden eklendiğinde `dups --changed main` → `1 duplicate candidate(s) touching main...HEAD (1 shown; >= 100 nodes; 1341 files, 1156559 nodes, 1.4s)` (`internal/lang/interp/operators.go:561-650  valuesEqualCopy  [changed]`).
- **1.2 Modül dışı arayüz:**
  - `go run ./tools/codegraph tests-for goload.importerFunc.Import` → `goload.importerFunc.Import [method] internal/devtools/goload/goload.go:228` ve `  no tests found`.
- **1.3 Belge dosyasına `run:` satırı yok:**
  - `go run ./tools/codegraph tests-for docs/status.md` → `docs/status.md [doc] docs/status.md:1`, `run:` satırı yok (`<- tests (3):` altında `testrunner.TestIdentityExceptionsAreNotStale`, `testrunner.TestReviewedSectionsHoldOnlyRules`, `testrunner.TestReviewedSectionsWriteIdentities`).
  - `go run ./tools/codegraph tests-for internal/lang/interp/equality.go` → iki `run:` satırı veriyor.
- **1.4 Eksik tanımlı kodda iç hata:**
  - `internal/lang/codes/tr.json`'a geçici `"<gecici_kod>": {"name": "gecici_deneme_10"}` eklendiğinde `tests-for internal/lang/codes/tr.json`:
    - `codegraph: warning: answers about .tr code are missing (tr sources skipped: TR'de bir iç hata oluştu (bu sizin programınızın değil, TR'nin hatası); lütfen bildirin: code list tr:)`
  - `go generate ./internal/lang/codes`:
    - `error: definitions are inconsistent: TR'de bir iç hata oluştu (bu sizin programınızın değil, TR'nin hatası); lütfen bildirin: <gecici_kod> (gecici_deneme_10) is in the language file but has no definition in definitions.json`
- **1.5 Atası olan commit:**
  - `go run ./tools/codegraph changed --go-tests dcbe037` (`dcbe037` `main~1` atası) → 0 satır, sessiz boş çıktı (`exit=0`).
- **1.6 Uzak 8'in iki kuralı fikstürsüz:**
  - `tools/codegraph/query_names_test.go:127-197`'de Uzak 9 (`5e913a9`) `unit(codes.FnE, box.exprUnit, fnTwo)`, `regMath(codes.FnE, math.Floor)` ve `unitMath(codes.FnF, box.exprMath, math.Floor)` eklemiş; ancak gösterici alıcılı metot ifadesiyle `unit(v, codes.X, (*T).M, f)` (ayrı `f` ve alıcı argümanı `v` ile), modül dışı işlevin paket işleviyle birlikte (`fnMath, math.Floor`) ve tablo satırında (`noteRowNameImpl`) davranışı fikstürde sabitlenmemiş.

## Adım 2a: İzlenmeyen Yeni Dosya `--changed`'e Girer

- **Seçim ve gerekçe:**
  - `tools/codegraph/dups_changed.go:55-76`'da `untrackedGoFiles(root)` (`untrackedWorkingFiles(root)` içindeki `.go` dosyaları) eklendi; canlı kolda (`changedGoLines`) her izlenmeyen `.go` dosyasının tam satır aralığı (`[1, math.MaxInt]`) değişen satırlara eklendi ve `rng` etiketine `" + " + countOf(len(untracked), "untracked file")` eklendi. `changeHunks` değiştirilmedi.
  - `tools/codegraph/dups_corpus.go:60`'ta `loadCorpus` yalnız `walkFiles(root, ".go")` (`codeindex.TrackedFiles`, yani `git ls-files -z`) çağırdığı için git deposundaki izlenmeyen `.go` dosyaları derleme hiç girmiyordu; `loadCorpus`'a da `untrackedGoFiles(root)` eklendi.
  - **Eskimiş belge bulgusu:** `tools/codegraph/docs.go:15` yorumu `codeindex.ListFiles` için `"izlenmeyen yeni dosyalar dahil"` diyordu; oysa `internal/devtools/codeindex/text.go:313-316` (`TrackedFiles`) yalnız `git ls-files -z` ile izlenen dosyaları döndürür. Yorum düzeltildi.
- **Üretim satırı:** +20 net satır (18 kod satırı, 2 yorum satırı; bütçe 15–40 / 5–20).
- **Koruyan test:** `tools/codegraph/changed_test.go:277` (`TestDupsChangedAndChangedSeeUntrackedGoFile`).

## Adım 2b: Modül Dışı Arayüzün Uygulaması (Dur ve Sor Eşiği)

- **Uygulama (`tools/codegraph/stdlib_dispatch.go`, `gocode_scan.go`, `test_reach.go`, ~55 üretim satırı):**
  - Yapı alanı (`CompositeLit`), çağrı argümanı (`CallExpr`, açık tip dönüşümleri hariç) ve atama (`AssignStmt`, `_` olmayan `ValueSpec`) yerlerinde modülün somut tiplerinin modül dışı adlı ve boş olmayan arayüz yerine verildiği noktalar `stdlib_calls` (`INFERRED`) kenarı olarak kaydedildi; `return` ifadeleri dışarıda bırakıldı. `test_reach.go:26` `reachEdge`'e `edgeStdlibCalls` eklendi.
  - `goload.importerFunc.Import` için `goload.Load` (`internal/devtools/goload/goload.go:117`, `m.check(..., imp, ...)`) ve `goload.Module.checkTests` (`internal/devtools/goload/goload.go:187`, `m.check(..., withTests, ...)`) üzerinden 2 kenar kuruluyor (`check` içinde `:218`'deki `types.Config{Importer: imp}`'de `imp` zaten `types.Importer` arayüz tipindedir).
- **Kenar sayıları ve "Dur ve sor" eşiği:**
  - Tabanda `stdlib_calls` kenar sayısı: **10** (`container/heap.Interface`: 5, `math/rand.Source`: 2, `io.Writer`: 2, `error`: 1).
  - **Seçenek A (`error` dahil edildiğinde; `named.Obj().Pkg() == nil` evren kapsamındaki `error` arayüzü sayılırsa):**
    - Toplam `stdlib_calls`: **279** (**+269 eklenen kenar**, > 200 eşiği).
    - Arayüz dağılımı (eklenen 269 kenar):
      - `error`: **230** kenar (%85.5 > 1/4 eşiği; `messages.Error.Error`: 210, `messages.PosError.Error`: 16, `turkish.Reason.Error`: 2, `tokenize.StepLimitError.Error`: 1, `interp.thrownError.Error`: 1 — `c.addErrorAt(..., messages.New(...))`, `recordOf(messages.New(...))`, `err = messages.New(...)` vb. çağrı argümanı ve atamalardan).
      - `testing.TB`: **22** kenar (`testkit.recorder`: 16, `testkit.panickingTB`: 4, `testkit.fatalRecorder`: 2).
      - `io.Writer`: **14** kenar (`testkit.TeeWriter.Write`: 10, `process.stderrTail.Write`: 2, `debugger.dapOutput.Write`: 1, `process.boundedOutput.Write`: 1).
      - `go/types.Importer`: **2** kenar (`goload.importerFunc.Import`, `goload.go:117` ve `goload.go:187`).
      - `io.Reader`: **1** kenar (`debugger.dapNoStdin.Read`, `dap_launch.go:65`).
      - `fmt.Stringer`: **0** kenar.
  - **Seçenek B (`error` dışarıda bırakıldığında — kuraldaki `"modülün içe aldığı bir paketin (standart kitaplık dahil) adlı arayüzünü sağlıyorsa"` lafzına göre `named.Obj().Pkg() != nil && !x.inModule(named.Obj().Pkg().Path())` koşuluyla evren kapsamındaki `error` hariç tutulduğunda):**
    - Toplam eklenen kenar: **39** (< 200 eşiği; `testing.TB`: 22, `io.Writer`: 14, `go/types.Importer`: 2, `io.Reader`: 1).
    - `error` + `fmt.Stringer` eklenen kenar: **0 / 39 (%0 < 1/4)**.
  - Yönergedeki bağlayıcı kural gereği commit'ten önce kullanıcıya soruluyor.

