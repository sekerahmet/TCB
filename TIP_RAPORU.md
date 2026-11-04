# Uzak İş 2 Raporu (`TIP_RAPORU.md`)

## Özet

- **Dal:** `gemini/tip-secimi` [ölçüldü: `git branch --show-current`, `gemini/tip-secimi`]
- **Taban (`main`):** `a259ed99e2c3b340e3c5223094158512dc4e8507` [ölçüldü: `git rev-parse main`, `a259ed99e2c3b340e3c5223094158512dc4e8507`]
- **Başlama saati:** `2026-11-04 15:39 UTC` (`2026-11-04 18:39 UTC+3`) [ölçüldü: `date -u` ve `TZ=Europe/Istanbul date`, `2026-11-04 15:39 UTC`]
- **Bitiş saati:** `2026-11-04 17:21 UTC` (`2026-11-04 20:21 UTC+3`) [ölçüldü: `date -u` ve `TZ=Europe/Istanbul date`, `2026-11-04 17:21 UTC`]
- **Bağlam sıkıştırması:** 3 kez:
  1. `2026-11-04 16:03 UTC` (`2026-11-04 19:03 UTC+3`) — Go `1.26.10` kurulumu ve ilk okumaların ardından, herhangi bir kod değişikliğinden önce; tüm ön ölçümler sıkıştırma sonrasında yeniden koşulup doğrulandı.
  2. `2026-11-04 16:36 UTC` (`2026-11-04 19:36 UTC+3`) — N6 düzenlemeleri çalışma ağacına yazıldıktan hemen sonra, test ve commit öncesinde; `CLAUDE.md`, `origin/yonerge:UZAK_YONERGE_2.md`, `TIP_DURUM.md` ve düzenlenen dosyalar yeniden açılıp okundu.
  3. `2026-11-04 17:13 UTC` (`2026-11-04 20:13 UTC+3`) — Tüm kod değişiklikleri, son ölçümler ve bitti ölçütü komutları tamamlanıp `TIP_RAPORU.md` taslağının kendi kontrolü çalışırken; `CLAUDE.md`, `origin/yonerge:UZAK_YONERGE_2.md`, `TIP_DURUM.md`, `TIP_RAPORU.md` ve ilgili kaynak dosyalar yeniden açılıp doğrulandı.

### İş Commit'leri (`main..gemini/tip-secimi`, durum dosyası commit'leri hariç)

| Sıra | Commit | Konu |
| :-- | :-- | :-- |
| 1 | `46aef6a` | N6: 0 adımlı `INFERRED` `tests` kenarlarının dosya (`tests-for`) ve `changed` çıktılarında `direct` diye yazılmaması ve `fileTestSet.all` yorumunun güncellenmesi |
| 2 | `e121209` | Tip bilgisiyle yükleme yapan testlerin kapsam bazlı seçimi (`1530` → `70` satır), kapsam testi ve değişiklik sonrası grafik kuran 5 satırlık fikstür testi |
| 3 | `ce94585` | `tools/codegraph/gaps.md` içinden kapanan maddenin silinmesi |
| 4 | `9b6baf4` | `extractorRevision` (`84` → `85`) ve `tools/codegraph/testdata/extractor_digest.txt` güncellemesi |

---

## Bulgular ve Uygulanan Değişiklikler

### 1. Tip bilgisiyle yükleme yapan testlerin kapsam bazlı seçimi ve kapsam testi (`e121209`)

- **İddia:** Değişiklikten sonraki grafikten sembol/arayüz hedefi çözerek bağ kurmak, kancayı koparan değişikliklerde (metodun adının değişmesi, uygulayıcı dosyanın silinmesi, ad/paket kullanımının silinmesi, başka pakette aynı çıplak adın eklenmesi) hedef kenarı kaybettiği için yanlış "test yok" üretir; bu testler `scans` yolu gibi yükleme kapsamlarındaki her Go değişikliğinde seçilmeli ve yeni bir tip yüklemeli testin sessizce liste dışı kalması bir kapsam testiyle engellenmelidir [görüş].
- **Kanıt:**
  - `tools/codegraph/type_queries_link.go`, `tools/codegraph/type_queries_params.go` ve `tools/codegraph/type_queries_sites.go` silindi; `tools/codegraph/type_queries.go` `1530` üretim satırından `70` satıra indirildi [ölçüldü: `ls tools/codegraph/type_queries*.go | grep -v _test | xargs wc -l`, `70 tools/codegraph/type_queries.go`]. Sınanamayan 8 sezginin tamamı (`lhsSels`, `usesTempDir`, `selfFile`, `incTests`, `testMaxSteps/2`, `writtenVars`, `collectSkipConds`, `hits == 1`) ve depo dışında kullanıcısı olmayan `packages.Load` yolu kaldırıldı [kanıtlı: `tools/codegraph/type_queries.go:37`, `func linkTypeQueries(b *builder, rules []typeQueryRule) {`].
  - `tools/full_check.sh` içindeki `repo_scans` dizisi `kind == "scan"` düğümlerine birebir kilitli olduğundan [kanıtlı: `tools/codegraph/scans_test.go:275`, `func TestFullCheckRepoScansMatchesScanNodes(t *testing.T) {`], `tools/full_check.sh`'e dokunmadan ayrı `kindTypeQuery` [kanıtlı: `tools/codegraph/graph.go:44`, `kindTypeQuery = "type_query" // tip bilgisiyle yükleme yapıp sorgulayan bir test grubu ve kapsamı (type_queries.go)`] ve `edgeTypeQueries` [kanıtlı: `tools/codegraph/graph.go:77`, `edgeTypeQueries   = "type_queries"   // test -> tip bilgisiyle yüklediği kapsamın type_query düğümü (type_queries.go)`] tanımlandı; kapsama ve yazdırma mantığı `tools/codegraph/query_scans.go` içinde `coveringHits` ve `printCoveringHits` ile ortaklaştırıldı (`dups --changed main`: `0 duplicate candidate(s)`) [ölçüldü: `go run ./tools/codegraph dups --changed main`, `0 duplicate candidate(s) touching main`].
  - Üretim listesi `repoTypeQueryRules` (`tools/codegraph/type_queries.go:20-33`) iki grubu kapsar: `tools/impact` (`TestInterfaceImplementers`, `TestMembersAndPackages`, `TestNameUses`, `TestUnknownNames` → `*.go` tüm depo) ve `internal/compute/tensor` (`TestBackendDataCallsGoThroughTheGate` → `internal/compute/tensor` altındaki test dışı `*.go`) [kanıtlı: `tools/codegraph/type_queries.go:20`, `var repoTypeQueryRules = []typeQueryRule{`]. Fikstür testleri mekanizmayı `buildGraphFromWithRules` üzerinden parametreyle kullanır [kanıtlı: `tools/codegraph/main.go:154`, `func buildGraphFromWithRules(root string, m *module, typeRules []typeQueryRule) *Graph {`].
  - Kapsam testi `TestRepositoryTypeLoadedTestsAreSelected` (`tools/codegraph/type_queries_test.go:211`), `tools/codegraph` dışında `goload.Load` ya da `(*types.Config).Check` çağrılarına ulaşan bütün testleri bulur ve `repoTypeQueryRules` + `repoTypeQueryExclusions` (`internal/devtools/goload` içindeki `t.TempDir()` birim testleri) ile birebir eşleştiğini, listeden ya da dışlamadan bir satır silindiğinde `checkTypeQueryCoverage`'ın düştüğünü doğrular [test: `tools/codegraph/type_queries_test.go`].
  - Fikstür testi `TestTypeQueryScopeSelectsEveryGoChange` (`tools/codegraph/type_queries_test.go:374`), `slowFixture(t)` çağırır [kanıtlı: `tools/codegraph/type_queries_test.go:375`, `slowFixture(t)`], `packages.Load` yerine `goload.Load` kullanır ve değişiklikten **sonra** kurulan grafiklerde beş tablo satırının tamamını (`claude/body_changed`, `claude/method_renamed`, `claude/impl_deleted`, `claude/use_deleted`, `claude/bare_name_collision`) ve `claude/unrelated` dalını doğrularken `TestTempModuleIgnored` testini seçmez; `--depth 1` sığ klonda da geçer [ölçüldü: `go -C /tmp/tcb-shallow-check test -v -count=1 ./tools/codegraph/ -run '^Test(RepositoryTypeLoadedTestsAreSelected|TypeQueryScopeSelectsEveryGoChange|InferredZeroStepEdgeIsNotWrittenAsDirect)$'`, `ok  	github.com/sekerahmet/TCB/tools/codegraph	12.268s`].
- **Öneri / Uygulama:** Uygulandı ve koruyan testlerle kilitlendi (`e121209`).

### 2. N6: 0 adımlı `[INFERRED]` kenarların `direct` yazılmaması (`46aef6a`)

- **İddia:** Dosya düzeyindeki `tests-for` (`query_file_tests.go`) ve `changed` (`changed.go`), `testDistances` içinden `d == 0` gelen her testi kenarın güven etiketine bakmadan `direct` diye yazıyordu; `case_tests.go` gibi çıkarımsal (`INFERRED`) 0 adımlı `tests` kenarları `direct` değil `[INFERRED]` olarak gösterilmeli ve `EXTRACTED` doğrudan kanıtlardan sonra sıralanmalıdır [görüş].
- **Kanıt:**
  - `fileTestSet.all` alanının eskimiş yorumu güncellendi [kanıtlı: `tools/codegraph/query_file_tests.go:88`, `all        map[string]int     // testsToRun ve dosyanın kendi testleri (testDistances): test -> en kısa uzaklık`] ve çıkarım olmayan doğrudan kanıtları toplayan `extractedDirectTests` eklendi [kanıtlı: `tools/codegraph/query_file_tests.go:98`, `func (ix *index) extractedDirectTests(nodes []*Node) map[string]bool {`].
  - Hem `printFileTests` (`tools/codegraph/query_file_tests.go:184-188`) hem `printTestsToRun` (`tools/codegraph/changed.go:365-372`), `d == 0` iken yalnız `extracted[id]` doğruysa `direct`, aksi hâlde `[INFERRED]` yazar; `TestInferredZeroStepEdgeIsNotWrittenAsDirect` (`tools/codegraph/query_file_tests_test.go:55`) her iki çıktıyı ve sıralamayı doğrular [test: `tools/codegraph/query_file_tests_test.go`].
- **Öneri / Uygulama:** Uygulandı (`46aef6a`).

### 3. `gaps.md` ve çıkarıcı revizyonu güncellemeleri (`ce94585`, `9b6baf4`)

- **İddia:** Kapanan kör nokta `tools/codegraph/gaps.md` listesinden çıkarılmalı ve grafik şemasına `kindTypeQuery` / `edgeTypeQueries` eklendiği için `extractorRevision` bir kez artırılmalıdır [görüş].
- **Kanıt:** `tools/codegraph/gaps.md` içindeki kapanan madde silindi (`ce94585`); `extractorRevision` `85` yapıldı [kanıtlı: `tools/codegraph/graph.go:25`, `const extractorRevision = 85`] ve `tools/codegraph/testdata/extractor_digest.txt` `digest 16d8431e084b86b6` olarak güncellendi (`9b6baf4`) [ölçüldü: `go test -short ./tools/codegraph/ -run '^TestExtractorRevisionFollowsExtractorCode$'`, `ok  	github.com/sekerahmet/TCB/tools/codegraph	0.048s`].
- **Öneri / Uygulama:** Uygulandı (`ce94585`, `9b6baf4`).

---

## Ayırıcı Seçenekleri ve Ölçüleri

| Ayırıcı Seçeneği | Davranış / Kapsam | Ölçü ve Sonuç |
| :-- | :-- | :-- |
| 1. Test AST'sinde doğrudan `Scope.Lookup` / `types.Implements` aramak | Yalnız sorguyu test gövdesinde yazan `internal/compute/tensor` gate testini görür; `tools/impact/main_test.go` sorguyu `run` üzerinden `tools/impact/query.go` içinde çalıştırdığı için asıl hedefi kaçırır | `tools/impact/query.go` sorguları üretim kodunda yapar [kanıtlı: `tools/impact/query.go:69`, `obj := pkg.Scope().Lookup(name)`] ve [kanıtlı: `tools/impact/query.go:127`, `tn, ok := pkg.Scope().Lookup(name).(*types.TypeName)`] → **elenmiş** |
| 2. Çağrı zincirinden `Scope.Lookup` / `types.Implements` erişimi | `tools/codegraph` kendi çıkarıcısında `Scope.Lookup` ve `types.Implements` çağırdığı için `repoGraph(t)` kullanan grafik testlerinin tamamını yanlış pozitif olarak çeker | Üretimde `implementations.go` çağırır [kanıtlı: `tools/codegraph/implementations.go:31`, `tn, ok := scope.Lookup(name).(*types.TypeName)`] → **elenmiş** |
| 3. **Seçilen:** Gerekçeli `{test, kapsam}` listesi (`repoTypeQueryRules`) + tip yükleyici kapsam testi (`checkTypeQueryCoverage`) | Depoda bağ alan 2 grubu (toplam 5 test: `tools/impact`'te 4, `internal/compute/tensor`'da 1) kapsamlarıyla bağlar; `internal/devtools/goload` içindeki 5 `t.TempDir()` testi gerekçeli dışlamadadır; `packages.Load` kaldırıldı | Üretim kodu `1530` → `70` satır; yeni bir `goload.Load` / `(*types.Config).Check` testi eklendiğinde kapsam testi zorlar [test: `tools/codegraph/type_queries_test.go`] |

---

## Önce ve Sonra Ölçümleri

### 1. Üretim Kodu ve `codegraph build` Süresi

| Ölçüt | Önce (`main` `a259ed9`) | Sonra (`gemini/tip-secimi` `9b6baf4`) |
| :-- | :-- | :-- |
| `type_queries*.go` üretim satırı (`_test.go` hariç) | `1530` (`383 + 457 + 304 + 386`) [ölçüldü: `ls tools/codegraph/type_queries*.go \| grep -v _test \| xargs wc -l`, `1530 total`] | `70` (`type_queries.go`) [ölçüldü: `ls tools/codegraph/type_queries*.go \| grep -v _test \| xargs wc -l`, `70 tools/codegraph/type_queries.go`] |
| `go run ./tools/codegraph build` (iç süre / duvar saati) | `8.2s` / `8.396s` (`31288 nodes, 231820 edges`) [ölçüldü: `time go run ./tools/codegraph build`, `in 8.2s`, `real_s=8.396`] | `7.6s` / `7.719s` (`7.746s` ikinci koşu; `31194 nodes, 231114 edges`) [ölçüldü: `time go run ./tools/codegraph build`, `in 7.6s`, `real_s=7.719`] |
| Seçilen `tools/impact` paket test süresi | `2.603s` (bu Linux VM'de; yönergedeki DENETİM 101 Windows ölçüsü `12,4 sn`) [ölçüldü: `go test -count=1 ./tools/impact`, `2.603s`] | `2.603s` (`4` test birlikte tek kez modül yükler) [kanıtlı: `tools/impact/main_test.go:21`, `loadOnce.Do(func() { loaded, loadErr = load(reporoot.Root()) })`] |
| Seçilen `TestBackendDataCallsGoThroughTheGate` süresi | `0.672s` [ölçüldü: `go test -count=1 ./internal/compute/tensor -run '^TestBackendDataCallsGoThroughTheGate$'`, `0.672s`] | `0.672s` (yalnız `internal/compute/tensor` test dışı `.go` dosyalarını denetler) [kanıtlı: `internal/compute/tensor/gate_calls_test.go:120`, `pkg, err := conf.Check("github.com/sekerahmet/TCB/internal/compute/tensor", fset, files, info)`] |

### 2. Commit Aralıklarında `go run ./tools/edithook codegraph changed --go-tests --base <taban> <uç>`

| Aralık (`taban..uç`) | Önce: Satır (`impact` / `gate`, süre) | Sonra: Satır (`impact` / `gate`, süre) | Fark |
| :-- | :-- | :-- | :-- |
| `0b9fda0b..83fa8859` | `1262` (`impact=2`, `gate=1`, `1254 ms`) [ölçüldü: `go run ./tools/edithook codegraph changed --go-tests --base 0b9fda0b 83fa8859`, `lines=1262 elapsed_ms=1254 impact=2 gate=1`] | `1264` (`impact=4`, `gate=1`, `1361 ms`) [ölçüldü: `go run ./tools/edithook codegraph changed --go-tests --base 0b9fda0b 83fa8859`, `lines=1264 elapsed_ms=1361 impact=4 gate=1`] | `+2` satır |
| `18c6beba..74a72663` | `43` (`impact=0`, `gate=0`, `1168 ms`) [ölçüldü: `go run ./tools/edithook codegraph changed --go-tests --base 18c6beba 74a72663`, `lines=43 elapsed_ms=1168 impact=0 gate=0`] | `47` (`impact=4`, `gate=0`, `1240 ms`) [ölçüldü: `go run ./tools/edithook codegraph changed --go-tests --base 18c6beba 74a72663`, `lines=47 elapsed_ms=1240 impact=4 gate=0`] | `+4` satır |
| `ba62f5f8..0f3e01af` | `883` (`impact=2`, `gate=0`, `1266 ms`) [ölçüldü: `go run ./tools/edithook codegraph changed --go-tests --base ba62f5f8 0f3e01af`, `lines=883 elapsed_ms=1266 impact=2 gate=0`] | `885` (`impact=4`, `gate=0`, `1304 ms`) [ölçüldü: `go run ./tools/edithook codegraph changed --go-tests --base ba62f5f8 0f3e01af`, `lines=885 elapsed_ms=1304 impact=4 gate=0`] | `+2` satır |
| `705c16f1..6b767cf5` | `660` (`impact=1`, `gate=0`, `1214 ms`) [ölçüldü: `go run ./tools/edithook codegraph changed --go-tests --base 705c16f1 6b767cf5`, `lines=660 elapsed_ms=1214 impact=1 gate=0`] | `663` (`impact=4`, `gate=0`, `1320 ms`) [ölçüldü: `go run ./tools/edithook codegraph changed --go-tests --base 705c16f1 6b767cf5`, `lines=663 elapsed_ms=1320 impact=4 gate=0`] | `+3` satır |
| `8ee2111e..52ce4b9f` | `300` (`impact=0`, `gate=0`, `1229 ms`) [ölçüldü: `go run ./tools/edithook codegraph changed --go-tests --base 8ee2111e 52ce4b9f`, `lines=300 elapsed_ms=1229 impact=0 gate=0`] | `305` (`impact=4`, `gate=0`, `1322 ms`) [ölçüldü: `go run ./tools/edithook codegraph changed --go-tests --base 8ee2111e 52ce4b9f`, `lines=305 elapsed_ms=1322 impact=4 gate=0`] | `+5` satır |

### 3. Gerçek Depo Kopyasında (`/tmp/tcb-real-check`) Üç Satırlık Doğrulama

| Senaryo (değişiklikten sonra grafik yeniden kurularak) | `changed --go-tests` Çıktısı | Sonuç |
| :-- | :-- | :-- |
| `internal/stdlib/ml/members_freeze.go` içinde `TensorValue.SetMember` → `SetMemberRenamed` | Toplam `39` satır; `tools/impact`'in dört testi (`TestInterfaceImplementers`, `TestMembersAndPackages`, `TestNameUses`, `TestUnknownNames`) seçildi; ardından koşulan `go test -count=1 ./tools/impact` `output lacks "ml.TensorValue"` ile hatayı yakaladı | [ölçüldü: `go run ./tools/edithook codegraph changed --go-tests HEAD`, `Row A total selected lines: 39`, `FAIL github.com/sekerahmet/TCB/tools/impact 2.923s`] |
| `internal/compute/tensor/tensor.go` içinde değişiklik | Toplam `36` satır; `internal/compute/tensor TestBackendDataCallsGoThroughTheGate` ve 4 `tools/impact` testi seçildi | [ölçüldü: `go run ./tools/edithook codegraph changed --go-tests HEAD`, `Row B total selected lines: 36`, `internal/compute/tensor TestBackendDataCallsGoThroughTheGate`] |
| İlgisiz paket (`internal/reporoot/root.go`) içinde değişiklik | Toplam `40` satır; 4 `tools/impact` testi seçildi, `TestBackendDataCallsGoThroughTheGate` kapsamı (`internal/compute/tensor`) gereği seçilmedi | [ölçüldü: `go run ./tools/edithook codegraph changed --go-tests HEAD`, `Row C (internal/reporoot/root.go) total selected lines: 40`] |

---

## Bitti Ölçütü Komutları ve Son Satırları

1. **Koruyan fikstür testi, kapsam testi (1 satır silinince düşme dahil), `--depth 1` klon ve 3 satırlık gerçek depo ölçüsü:**
   - Ana depoda (`-short` olmadan):
     `go test -count=1 ./tools/codegraph/ -run '^Test(RepositoryTypeLoadedTestsAreSelected|TypeQueryScopeSelectsEveryGoChange|InferredZeroStepEdgeIsNotWrittenAsDirect)$'`
     Son satır: `ok  	github.com/sekerahmet/TCB/tools/codegraph	11.483s` [ölçüldü: `go test -count=1 ./tools/codegraph/ -run '^Test(RepositoryTypeLoadedTestsAreSelected|TypeQueryScopeSelectsEveryGoChange|InferredZeroStepEdgeIsNotWrittenAsDirect)$'`, `ok  	github.com/sekerahmet/TCB/tools/codegraph	11.483s`]
   - `git clone --depth 1` sığ klonda (`rev-list --count HEAD = 1`, `-short` olmadan):
     Son satır: `ok  	github.com/sekerahmet/TCB/tools/codegraph	12.268s` [ölçüldü: `go -C /tmp/tcb-shallow-check test -v -count=1 ./tools/codegraph/ -run '^Test(RepositoryTypeLoadedTestsAreSelected|TypeQueryScopeSelectsEveryGoChange|InferredZeroStepEdgeIsNotWrittenAsDirect)$'`, `ok  	github.com/sekerahmet/TCB/tools/codegraph	12.268s`]
2. **`go build ./... && go vet ./...`:**
   - Çıkış kodu `0`, çıktı boş [ölçüldü: `go build ./... && go vet ./...`, `build_and_vet_exit=0`].
3. **`go test -short -timeout 1200s ./tools/codegraph/... ./tools/edithook/...`:**
   - Son iki satır:
     `ok  	github.com/sekerahmet/TCB/tools/codegraph	216.936s`
     `ok  	github.com/sekerahmet/TCB/tools/edithook	0.091s` [ölçüldü: `go test -short -timeout 1200s ./tools/codegraph/... ./tools/edithook/...`, `ok  	github.com/sekerahmet/TCB/tools/edithook	0.091s`]
4. **`go run ./tools/codegraph spec-refs`:**
   - Son satır: `301 item(s), 5510 mention(s), 0 error(s), 0 note(s)` [ölçüldü: `go run ./tools/codegraph spec-refs`, `301 item(s), 5510 mention(s), 0 error(s), 0 note(s)`]
5. **`gofmt -l cmd internal tools lib`:**
   - Boş (`0` satır) [ölçüldü: `gofmt -l cmd internal tools lib`, `gofmt_lines=0`].
6. **`ls tools/codegraph/type_queries*.go | grep -v _test | xargs wc -l`:**
   - Son satır: `70 tools/codegraph/type_queries.go` [ölçüldü: `ls tools/codegraph/type_queries*.go | grep -v _test | xargs wc -l`, `70 tools/codegraph/type_queries.go`]

---

## Ortam (Temiz Makine Bulguları)

1. **`go` ikilisi sanal makinede öntanımlı kurulu değildi:** `go.mod` dosyasındaki sürüm [kanıtlı: `go.mod:5`, `toolchain go1.26.10`] resmi arşivden kullanıcı alanına (`/home/antigravity/.local/go`) indirilip `GOTOOLCHAIN=local` ile kullanıldı (`go version go1.26.10 linux/amd64`).
2. **`/usr/bin/time` ikilisi yoktu:** Debian 12 sanal makinesinde dış `/usr/bin/time` paketi kurulu olmadığından ölçümlerde bash yerleşik `time` (`TIMEFORMAT`) ve `date +%s%3N` kullanıldı.
3. **Tek-dal `git` refspec:** Başlangıç klonunda `remote.origin.fetch` yalnız `main`'e ayarlıydı; `origin/yonerge` dalını okumak için `+refs/heads/*:refs/remotes/origin/*` olarak güncellendi.
4. **Dalda tutulan kök durum/rapor dosyaları ve `TestEveryTrackedDataFileHasATestOrAReason`:** Uzak ortam kuralı gereği dalın kökünde commit'lenen `TIP_DURUM.md` / `TIP_RAPORU.md` dosyaları (TCB birleştirmede siler), `git ls-files`'ta göründüğünde `TestEveryTrackedDataFileHasATestOrAReason` [kanıtlı: `tools/codegraph/data_coverage_test.go:69`, `func TestEveryTrackedDataFileHasATestOrAReason(t *testing.T) {`] tarafından teste bağlanmamış `.md` veri dosyası sayılır; tam paket koşusunda birleştirme sonrası durumu yansıtmak üzere `TIP_DURUM.md` geçici olarak `git rm --cached` ile indeksten çıkarılıp koşu bitiminde `trap` ile geri eklendi.
5. **Geçici uzak sanal makine duraklaması:** `2026-11-04 16:40 UTC`'de uzak uVM kısa süreli `STOPPED` durumuna geçip otomatik yeniden başlatıldı; `/home/antigravity` altındaki Go kurulumu ve çalışma ağacı korundu.

---

## Takıldığım Yerler, Emin Olmadıklarım ve Bakmadıklarım

- **Takıldığım yerler:**
  1. `TestInferredZeroStepEdgeIsNotWrittenAsDirect` için ilk yazdığım mini fikstürde `Run` dağıtıcısı ile `handleCheck` aynı dosyada (`p/cmd.go`) olduğundan `TestACase` dosyaya doğrudan `EXTRACTED` kenar da kuruyordu; dağıtıcıyı gerçek depodaki gibi ayrı dosyaya (`p/dispatch.go`) ayırınca `INFERRED` 0 adımlı kenar izole biçimde doğrulandı.
  2. `tools/codegraph/type_queries.go` içindeki `reason` dize sabitlerini ilk taslakta Türkçe yazdığımda `TestNoTurkishLiterals` (`./internal/`) yakaladı; yorumlar Türkçe tutulup üretim kodu dize sabitleri İngilizceye çevrildi.
  3. Gerçek depo kopyasındaki 3. satır denemesinde ilk olarak var olmayan `internal/reporoot/reporoot.go` adına ekleme yapıldığı için derleme uyarısı görüldü; gerçek dosya olan `internal/reporoot/root.go` üzerinde temiz yeniden kurulumla (`graph rebuilt (7.7 s)`) ölçüm yenilendi.
- **Emin olmadıklarım:** Yok.
- **Bakmadıklarım:** Yönergenin "Dokunma" listesindeki paketlere (`internal/lang/*`, `internal/stdlib/*`, `internal/compute/*`, `docs/spec/*`, `docs/status.md`, `tools/full_check.sh`) ve `tools/codegraph` içindeki `.tr` fikstürlerine dokunulmadı; tam ağaç testi yerine yönergenin bitti ölçütündeki paketler (`./tools/codegraph/...`, `./tools/edithook/...`) ve dokunulan dosyaların `run:` kuralları koşuldu.

## Codegraph

yok

## SORU

Yok.

## Kendi kontrolüm

- Rapordaki `17` `[kanıtlı: ...]` alıntısının tamamı, toplam `23` kaynak satır/aralık göndermesi ve `3` `[test: ...]` dosya yolu kaynak dosyaları yeniden açılarak ve `grep -F` + satır satır karakter karşılaştırması koşularak doğrulandı.
- Silinen alıntı: `0`. Değiştirilen alıntı: `5` — Markdown tablosu içinde `||` barındırdığı için sütun ayrımını etkileyen ya da kaçış gerektiren `3` alıntı borusuz tam satır karşılıklarıyla güncellendi; satır sonu yorumunu dışarıda bırakan `2` kısaltılmış sabit alıntısı (`kindTypeQuery` ve `edgeTypeQueries`) yorumlarıyla birlikte tam satır hâline getirildi. Son doğrulamada `17/17` alıntının tamamı kaçışsız `grep -F` ve tam satır karşılaştırmasından geçti.
- Bağlam sıkıştırmaları sonrasında taban commit kimliği, 4 iş commit'i, üretim satır sayıları (`383 + 457 + 304 + 386 = 1530` → `70`), `extractorRevision = 85` (`digest 16d8431e084b86b6`) ve `TIP_DURUM.md`'ye kaydedilen tüm önce/sonra ile bitti ölçütü çıktıları kaynağından yeniden doğrulandı.

Kendi incelemem bitti.
