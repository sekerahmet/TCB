# IMPACT_DURUM

Başlangıç saati: `2026-11-04 14:33:05 UTC+3` (`2026-11-04 11:33:05 UTC`) [ölçüldü: `date`, 2026-11-04 14:33:05 UTC+3].
Son güncelleme: `2026-11-04 16:04 UTC+3` [ölçüldü: `TZ='UTC-3' date`, `2026-11-04 16:03:58 UTC+3`].

## Bitenler
- `CLAUDE.md` tamamı okundu (`1..30`).
- Yönerge ve `GEMINI_KURALLARI.md` okundu; her sıkıştırma sonrasında kurallar, `IMPACT_DURUM.md`, `git status`, `git log` ve kaynak dosyalar yeniden açılıp doğrulandı.
- **Adım 1 (Ortam ve Dal):**
  - İşletim sistemi: Debian GNU/Linux 12 (bookworm), çekirdek `6.12.105+`, KVM sanal makinesi (`agy-plugin-proxy` / `exebox` kum havuzu, `/` salt okunur, `/home/antigravity` 19G yazılabilir, `sudo`/root yok) [ölçüldü: `uname -a`, `/etc/os-release`, `df -h`].
  - Donanım: 2 vCPU (`Intel(R) Xeon(R) Platinum 8481C`), 7.8Gi RAM (`0B` swap) [ölçüldü: `lscpu`, `free -h`].
  - Go kurulumu: `https://go.dev/dl/go1.26.10.linux-amd64.tar.gz` indirilip `/home/antigravity/.local/go` altına açıldı, `GOTOOLCHAIN=local` ayarlandı (`go version go1.26.10 linux/amd64`) [ölçüldü: `go version`, `go env GOTOOLCHAIN`].
  - Dal: `main` (`5a6c840ae1e8c52fb8b1aab41ec838c45b952d2d`) ucundan `gemini/impact-secimi` açıldı [ölçüldü: `git rev-parse main`].
- **Adım 2 (`tools/codegraph/gaps.md` ve `tools/codegraph/README.md` okunması):** `gaps.md:53` (`main`) ve `README.md:1..714` okundu.
- **Adım 3 (Yeniden Üretim - Önce):**
  - `go run ./tools/edithook codegraph changed --go-tests --base 0b9fda0b 83fa8859` → çıkış `0`, `1250` satır, `tools/impact` `0`, `TestBackendDataCallsGoThroughTheGate` `0`, son satır `tools/gen/snippets TestSnippetsUpToDate` [ölçüldü: `/tmp/repro_before.txt`].
- **Adım 4 (`go/packages` ve yardımcılarıyla depoyu yükleyen testlerin sayımı — kaynaktan yeniden doğrulandı):**
  - `git grep -l 'packages.Load' -- '*_test.go'` → `main`'de `0` dosya; dalda fikstür testi `tools/codegraph/type_queries_test.go` ile `1` dosya [ölçüldü: `git grep`].
  - Gerçek depoyu `goload.Load` ile yükleyen testler:
    - `tools/impact/main_test.go`: 4 test (`TestNameUses`, `TestInterfaceImplementers`, `TestMembersAndPackages`, `TestUnknownNames`) — `repo(t)` (`tools/impact/main_test.go:19-26`) → `load(reporoot.Root())` (`tools/impact/load.go:10-16`) → `goload.Load`.
    - `tools/codegraph/*_test.go`: `repoGraph(t)` (`tools/codegraph/helpers_test.go:156-171` → `load(repoState.root)` → `goload.Load`) üzerinden `main`'de 32 dosyada 47 test, bu dalda eklenen `TestRepositoryTypeLoadedTestsAreSelected` ile 33 dosyada 48 test (`stdlib_dispatch_test.go:107`'deki `TestEveryStandardLibraryDispatchHasAnEdge` ayrıca doğrudan `goload.Load(root, ...)` çağırır).
  - Tek paketi diskten `(*types.Config).Check` ile yükleyen test:
    - `internal/compute/tensor/gate_calls_test.go`: 1 test (`TestBackendDataCallsGoThroughTheGate`, `23-86`) — `checkTensorPackage(t)` (`96-125`) → `conf.Check("github.com/sekerahmet/TCB/internal/compute/tensor", ...)`.
  - Geçici modül / bellek içi kaynak yükleyen testler (depo dosyalarına bağlanmamalı): `internal/devtools/goload/goload_test.go` (7 testten 5'i `Load(...)` çağırır), `tools/codegraph/audit_test.go` (`loadAudit` üzerinden fikstür), `stdlib_dispatch_test.go` (`TestStandardLibraryDispatchIsAUseWithItsRole`), ve `fixture(t)` / `buildInto(t, root)` testleri.
- **Adım 5 (`edithook` ve `codegraph` test seçim mimarisinin incelenmesi):**
  - `0b9fda0b..83fa8859` aralığında `SetMember` yeni eklenen `A internal/stdlib/ml/members_freeze.go` dosyasında tanımlı olduğu ve eski commit aralığında (`live == false`) yeni dosyanın bildirim düğümü başlangıç tarafında bulunmadığı için `goTestSelection` (`tools/codegraph/changed.go:292-298`) seçimi `go:file:<rel>` düğümünün `testDistances` kenarları üzerinden yapıyor.
- **Adım 6 (Genel çözüm, koruyan testler, dosya boyutu bölünmesi ve bedel ölçümü — commit `f47a033`, `7224efa`, `c26eb89`):**
  - `tools/codegraph/type_queries.go` (383 satır), `tools/codegraph/type_queries_params.go` (304 satır), `tools/codegraph/type_queries_sites.go` (386 satır), `tools/codegraph/type_queries_link.go` (457 satır) ve `tools/codegraph/type_queries_test.go` (187 satır) oluşturuldu; `gocode.go`, `graph.go` (`extractorRevision = 84`), `main.go`, `query_file_tests.go` ve `testdata/extractor_digest.txt` (`digest 75a3d2422ca1d341`) güncellendi.
  - Koruyan testler (`tools/codegraph/type_queries_test.go`): `TestRepositoryTypeLoadedTestsAreSelected` ve `TestPackagesLoadTypeQueriesSelectMatchingChangesOnly`.
  - Son ağaçta (`c26eb89`) bedel ölçümü (`changed --go-tests`, önce → sonra):
    - `0b9fda0b..83fa8859`: `1250` → `1254` (`+4`: `internal/compute/tensor TestBackendDataCallsGoThroughTheGate`, `tools/codegraph TestRepositoryTypeLoadedTestsAreSelected`, `tools/impact TestInterfaceImplementers`, `tools/impact TestNameUses`).
    - `18c6beba..74a72663`: `43` → `43` (`+0`).
    - `ba62f5f8..0f3e01af`: `877` → `879` (`+2`: `tools/impact TestMembersAndPackages`, `tools/impact TestNameUses`).
    - `705c16f1..6b767cf5`: `652` → `653` (`+1`: `tools/impact TestMembersAndPackages`).
    - `8ee2111e..52ce4b9f`: `298` → `300` (`+2`: `tools/codegraph` değiştiği için yeni eklenen `TestPackagesLoadTypeQueriesSelectMatchingChangesOnly` ve `TestRepositoryTypeLoadedTestsAreSelected`).
- **Adım 7 (`tools/codegraph/gaps.md` güncellemesi — commit `2c4d914`):**
  - Kapanan madde silindi; `otherConfigMentions` (`tools/codegraph/query_other_configs.go:31`) üzerindeki `who-uses impact.load` → `internal/sysmem/memory_windows.go:11` bulgusu `İnce ayar` başlığına eklendi.
- **Adım 8a (5 `Bitti ölçütü` komutu — `c26eb89` üzerinde tamamı çıkış `0`):**
  1. `go run ./tools/edithook codegraph changed --go-tests --base 0b9fda0b 83fa8859` → `1254` satır (`tools/impact TestInterfaceImplementers` ve `tools/impact TestNameUses` var), son satır: `tools/impact TestNameUses`.
  2. `go build ./... && go vet ./...` → çıkış `0`, boş çıktı.
  3. `go test -short -timeout 1200s ./tools/codegraph/... ./tools/edithook/...` → çıkış `0`, son satır: `ok  	github.com/sekerahmet/TCB/tools/edithook	(cached)` (`tools/codegraph` `210.279s`, önceki koşuda `tools/edithook` `0.067s`).
  4. `go run ./tools/codegraph spec-refs` → çıkış `0`, son satır: `301 item(s), 5448 mention(s), 0 error(s), 0 note(s)`.
  5. `gofmt -l cmd internal tools lib` → çıkış `0`, boş çıktı.
  - Ek düzen ve tekrar denetimleri: `go test -short ./internal/ -run '^(TestLayoutFileSize|TestLayoutFuncSize|TestTestFileMatchesSource|TestLayoutFileNames|TestLayoutEnglishFileNames|TestNoTurkishLiterals|TestGoNamesMixedCaps|TestNoOldCodeForm)$'` (`ok`, `3.463s`), `go run ./tools/codegraph dups --changed main` (`0 duplicate candidate(s)`).

## Bağlam Sıkıştırması Notu
- **1. sıkıştırma (`2026-11-04 14:49 UTC+3`):** Adım 5 tamamlandıktan hemen sonra, hiçbir kaynak dosyaya dokunulmadan önce gerçekleşti.
- **2. sıkıştırma (`2026-11-04 15:08 UTC+3`):** Adım 6 ön bedel ölçümleri tamamlandıktan hemen sonra, hiçbir kaynak dosyaya dokunulmadan önce gerçekleşti.
- **3. sıkıştırma (`2026-11-04 15:38 UTC+3`):** Adım 7'de `tools/codegraph/gaps.md` düzenlendikten hemen sonra, commit öncesi gerçekleşti. Yeniden doğrulama sırasında `gofmt` sonrası `type_queries_sites.go` boyutu (`675 > 600`) ve `extractor_digest.txt` yakalanıp `c26eb89`'da düzeltildi.

## Kalanlar
- **Adım 8b:** `IMPACT_RAPORU.md` dosyasını yazmak, kendi kontrolünü tamamlayıp commit ve push etmek.

## Emin Olmadıklarım
- Yok.
