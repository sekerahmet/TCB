# IMPACT_DURUM

Başlangıç saati: `2026-11-04 14:33:05 UTC+3` (`2026-11-04 11:33:05 UTC`) [ölçüldü: `date`, 2026-11-04 14:33:05 UTC+3].

## Bitenler
- `CLAUDE.md` tamamı okundu (`1..30`).
- Yönerge ve `GEMINI_KURALLARI.md` okundu.
- **Adım 1 (Ortam ve Dal):**
  - İşletim sistemi: Debian GNU/Linux 12 (bookworm), çekirdek `6.12.105+`, KVM sanal makinesi (`agy-plugin-proxy` / `exebox` kum havuzu, `/` salt okunur, `/home/antigravity` 19G yazılabilir, `sudo`/root yok) [ölçüldü: `uname -a`, `/etc/os-release`, `df -h`].
  - Donanım: 2 vCPU (`Intel(R) Xeon(R) Platinum 8481C`), 7.8Gi RAM (`0B` swap) [ölçüldü: `lscpu`, `free -h`].
  - Go kurulumu: `https://go.dev/dl/go1.26.10.linux-amd64.tar.gz` indirilip `/home/antigravity/.local/go` altına açıldı, `GOTOOLCHAIN=local` ayarlandı (`go version go1.26.10 linux/amd64`) [ölçüldü: `go version`].
  - Dal: `main` (`5a6c840ae1e8c52fb8b1aab41ec838c45b952d2d`) ucundan `gemini/impact-secimi` açıldı [ölçüldü: `git rev-parse main`].
- **Adım 2 (`tools/codegraph/gaps.md` ve `tools/codegraph/README.md` okunması):** `gaps.md:53` ve `README.md:1..714` okundu.
- **Adım 3 (Yeniden Üretim - Önce):**
  - `go run ./tools/edithook codegraph changed --go-tests --base 0b9fda0b 83fa8859` → çıkış `0`, `1250` satır, `tools/impact` `0`, `TestBackendDataCallsGoThroughTheGate` `0`, son satır `tools/gen/snippets TestSnippetsUpToDate` [ölçüldü: `/tmp/repro_before.txt`].
- **Adım 4 (`go/packages` ve yardımcılarıyla depoyu yükleyen testlerin sayımı):**
  - `git grep -l 'packages.Load' -- '*_test.go'` → `0` dosya (depo `internal/devtools/goload.Load` ve `go/types` kullanıyor) [ölçüldü: `git grep`].
  - Gerçek depoyu `goload.Load` ile yükleyen testler:
    - `tools/impact/main_test.go`: 4 test (`TestNameUses`, `TestInterfaceImplementers`, `TestMembersAndPackages`, `TestUnknownNames`) — `repo(t)` → `load(reporoot.Root())` → `goload.Load`.
    - `tools/codegraph/*_test.go`: `repoGraph(t)` (`helpers_test.go:156` → `load(repoState.root)` → `goload.Load`) üzerinden 28 dosyada 40 test (bütün graf/çıkarıcı tutarlılık testleri).
  - Tek paketi diskten `(*types.Config).Check` ile yükleyen test:
    - `internal/compute/tensor/gate_calls_test.go`: 1 test (`TestBackendDataCallsGoThroughTheGate`) — `checkTensorPackage(t)` → `conf.Check("github.com/sekerahmet/TCB/internal/compute/tensor", ...)`.
  - Geçici modül / bellek içi kaynak yükleyen testler (depo dosyalarına bağlanmamalı): `internal/devtools/goload/goload_test.go` (5 test), `tools/codegraph/audit_*_test.go` (3 test), `dispatch_test.go`, `implementations_test.go`, `uses_coverage_test.go`, `var_access_test.go`, `fixture(t)` testleri.
- **Adım 5 (`edithook` ve `codegraph` test seçim mimarisinin incelenmesi):**
  - `0b9fda0b..83fa8859` aralığında `SetMember` yeni eklenen `A internal/stdlib/ml/members_freeze.go` dosyasında tanımlı olduğu ve eski commit aralığında (`live == false`) yeni dosyanın bildirim düğümü başlangıç tarafında bulunmadığı için `goTestSelection` (`changed.go:292-298`) seçimi `go:file:<rel>` düğümünün `testDistances` kenarları üzerinden yapıyor.
- **Adım 6 (Genel çözüm, koruyan testler ve bedel ölçümü — commit `f47a033`):**
  - `findQueryCaps` içinde `goload.Load` gibi yükleyicilerin `info.Selections = ...` atama sol tarafları süzüldü; böylece `impact.TestInterfaceImplementers` kenarları tam olarak `go run ./tools/impact`'in raporladığı 11 dosyaya indi (`18c6beba..74a72663` aralığındaki yanlış `+1` de `0` oldu).
  - Koruyan testler (`tools/codegraph/type_queries_test.go`):
    - `TestRepositoryTypeLoadedTestsAreSelected`: `0b9fda0b..83fa8859` aralığında `tools/impact TestInterfaceImplementers`, `tools/impact TestNameUses` ve `internal/compute/tensor TestBackendDataCallsGoThroughTheGate` seçimini, `TestUnknownNames` dışlanmasını ve `tests-for internal/stdlib/ml/members_freeze.go` çıktısını doğrular.
    - `TestPackagesLoadTypeQueriesSelectMatchingChangesOnly`: sentetik git deposunda `packages.Load` ile arayüz uygulayıcısı (yeni dosya ekleme dahil), ad kullanımı ve paket kullanıcısı testlerinin seçildiğini, ilgisiz dosya ve `t.TempDir()` yüklemelerinin seçilmediğini doğrular.
  - Bedel ölçümü (`changed --go-tests`, önce → sonra):
    - `0b9fda0b..83fa8859`: `1250` → `1253` (`+3`: `internal/compute/tensor TestBackendDataCallsGoThroughTheGate`, `tools/impact TestInterfaceImplementers`, `tools/impact TestNameUses`).
    - `18c6beba..74a72663`: `43` → `43` (`+0`).
    - `ba62f5f8..0f3e01af`: `877` → `879` (`+2`: `tools/impact TestMembersAndPackages`, `tools/impact TestNameUses`).
    - `705c16f1..6b767cf5`: `652` → `653` (`+1`: `tools/impact TestMembersAndPackages`).
    - `8ee2111e..52ce4b9f`: `298` → `298` (`+0`).
  - `go run ./tools/codegraph dups --changed main`: `0 duplicate candidate(s)`.
  - Düzen testleri (`./internal/` boyut, ad, eşleşme): geçti (`ok`).

## Bağlam Sıkıştırması Notu
- **1. sıkıştırma (`2026-11-04 14:49 UTC+3`):** Adım 5 tamamlandıktan hemen sonra, hiçbir kaynak dosyaya dokunulmadan önce gerçekleşti.
- **2. sıkıştırma (`2026-11-04 15:08 UTC+3`):** Adım 6 ön bedel ölçümleri tamamlandıktan hemen sonra, hiçbir kaynak dosyaya dokunulmadan önce gerçekleşti. Tüm kaynak dosyalar yeniden açıldı ve doğrulandı.

## Kalanlar
- **Adım 7:** `tools/codegraph/gaps.md` içinden kapanan maddeyi silmek ve yeni bulguyu eklemek; commit + durum güncellemesi + push.
- **Adım 8:** Bitti ölçütündeki 5 komutu koşmak, `IMPACT_RAPORU.md` yazmak, kendi kontrolünü tamamlayıp son durum güncellemesiyle yalnız `gemini/impact-secimi` dalına push etmek.

## Emin Olmadıklarım
- Yok.
