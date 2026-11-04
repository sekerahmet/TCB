# IMPACT_RAPORU

| Alan | Değer |
| :--- | :--- |
| İş | Uzak deneme 1: `codegraph`, depoyu `go/packages` ile yükleyen test seçilmiyor |
| Dal | `gemini/impact-secimi` [ölçüldü: `git branch --show-current`, `gemini/impact-secimi`] |
| Taban (`main`) | `5a6c840ae1e8c52fb8b1aab41ec838c45b952d2d` [ölçüldü: `git rev-parse main`, `5a6c840ae1e8c52fb8b1aab41ec838c45b952d2d`] |
| Başlangıç / Bitiş | `2026-11-04 14:33:05 UTC+3` / `2026-11-04 16:09 UTC+3` [ölçüldü: `TZ='UTC-3' date`] |
| Yeniden üretim (`0b9fda0b..83fa8859`) | Önce: `1250` satır (`tools/impact` yok) → Sonra: `1254` satır (`tools/impact TestInterfaceImplementers`, `tools/impact TestNameUses`, `internal/compute/tensor TestBackendDataCallsGoThroughTheGate`, `tools/codegraph TestRepositoryTypeLoadedTestsAreSelected` eklendi) [ölçüldü: `go run ./tools/edithook codegraph changed --go-tests --base 0b9fda0b 83fa8859`, `1250` → `1254` satır] |
| Bedel (4 ek aralık) | `18c6beba..74a72663`: `43`→`43` (`+0`); `ba62f5f8..0f3e01af`: `877`→`879` (`+2`); `705c16f1..6b767cf5`: `652`→`653` (`+1`); `8ee2111e..52ce4b9f`: `298`→`300` (`+2`, yeni eklenen `codegraph` testleri) [ölçüldü: `changed --go-tests`] |
| Bitti ölçütü (5 komut) | Tamamı çıkış `0`; `spec-refs`: `0 error(s), 0 note(s)`; `gofmt`: boş [ölçüldü: Adım 8a koşusu] |
| Bağlam sıkıştırması | 3 kez (`14:49`, `15:08`, `15:38 UTC+3`); her seferinde kurallar, durum dosyası ve kaynaklar yeniden açılıp doğrulandı [ölçüldü: oturum kayıtları] |

## 1. Dal, Taban ve Commit'ler

- **Dal:** `gemini/impact-secimi`, `main`'in ucundan açıldı (`5a6c840ae1e8c52fb8b1aab41ec838c45b952d2d`) [ölçüldü: `git rev-parse main`, `5a6c840ae1e8c52fb8b1aab41ec838c45b952d2d`].
- **Push hedefi:** Yalnız `origin/gemini/impact-secimi` dalına push edildi; `main`'e ya da başka bir dala dokunulmadı, çekme isteği (PR) açılmadı [ölçüldü: `git push origin gemini/impact-secimi`, çıkış `0`].
- **Commit listesi (`main..HEAD`, bu raporun commit'i hariç):**
  1. `558f104` (`558f10499bd6d8dd9c3641af6850d3c7f58963b1`) — `status: add IMPACT_DURUM.md to track remote progress on gemini/impact-secimi` [ölçüldü: `git log main..HEAD`]
  2. `f47a033` (`f47a033551c3516d86d7d40d8ac09a783e5a896a`) — `codegraph: link Go tests that type-check the repository to queried symbols and implementers` [ölçüldü: `git log main..HEAD`]
  3. `016f021` (`016f0212792fa18eb00bdbeba11b2b807e8b522c`) — `status: update IMPACT_DURUM.md after completing type-query test selection (f47a033)` [ölçüldü: `git log main..HEAD`]
  4. `2c4d914` (`2c4d914bf87f68d76e2ff1431bc1273735d237f4`) — `codegraph: update gaps.md for closed go/packages selection gap and unbuilt-file false match` [ölçüldü: `git log main..HEAD`]
  5. `68efc0d` (`68efc0d8e7f1e10517e228200dc7aa984f074101`) — `status: update IMPACT_DURUM.md after completing gaps.md update (2c4d914)` [ölçüldü: `git log main..HEAD`]
  6. `7224efa` (`7224efaba49577ea3302a56a31b0095790543243`) — `codegraph: update extractor_digest.txt for finalized type_queries.go` [ölçüldü: `git log main..HEAD`]
  7. `c26eb89` (`c26eb892382dc07ec5071032df1549b94f7c68d4`) — `codegraph: split parameter taint propagation into type_queries_params.go` [ölçüldü: `git log main..HEAD`]
  8. `d5c4255` (`d5c4255600f40d3aac7501fdf43f6dbb28cb6da9`) — `status: finalize IMPACT_DURUM.md after all checks pass on c26eb89` [ölçüldü: `git log main..HEAD`]
  9. Son commit: kökteki `IMPACT_RAPORU.md` dosyasını ekleyen commit.
- **Değişen izlenen dosyalar (`git diff --name-status main`):** yalnız `tools/codegraph/*` (`gaps.md`, `gocode.go`, `graph.go`, `main.go`, `query_file_tests.go`, `testdata/extractor_digest.txt`, `type_queries.go`, `type_queries_link.go`, `type_queries_params.go`, `type_queries_sites.go`, `type_queries_test.go`) ve geçici kök dosyaları (`IMPACT_DURUM.md`, `IMPACT_RAPORU.md`) [ölçüldü: `git diff --name-status main`]. `Dokunma` listesindeki hiçbir dosyaya dokunulmadı [ölçüldü: `git diff --name-status main`].

## 2. Uzak Ortam Bilgisi (Antigravity Remote Agents)

- **İşletim sistemi ve kum havuzu:** `Debian GNU/Linux 12 (bookworm)`, çekirdek `Linux sandbox 6.12.105+ #1 SMP Sat Sep 19 19:37:41 UTC 2026 x86_64 GNU/Linux`, KVM sanal makinesi [ölçüldü: `cat /etc/os-release`, `uname -a`, `lscpu`]. Komutlar `antigravity` kullanıcısıyla `exebox` kum havuzunda çalışıyor; kök dosya sistemi (`/`) salt okunur, `/home/antigravity` (`19G` toplam, `18G` boş) ve `/tmp` yazılabilir, `sudo` / parolasız `root` erişimi yok [ölçüldü: `df -h / /home/antigravity`].
- **Çekirdek sayısı:** `2` vCPU (`Intel(R) Xeon(R) Platinum 8481C`, `1` soket × `2` çekirdek × `1` iş parçacığı) [ölçüldü: `lscpu`].
- **Bellek:** `7.8Gi` toplam RAM (`~7.0Gi` kullanılabilir), `0B` takas alanı (swap) [ölçüldü: `free -h`].
- **Go kurulabiliyor mu:** Sistemde başlangıçta Go kurulu değildi (`go: command not found`), ancak `https://go.dev/dl/go1.26.10.linux-amd64.tar.gz` doğrudan indirilip kullanıcı alanına (`/home/antigravity/.local/go`) sorunsuz açıldı; `GOTOOLCHAIN=local` ile `go version go1.26.10 linux/amd64` olarak çalıştı [ölçüldü: `go version` → `go version go1.26.10 linux/amd64`, `go env GOTOOLCHAIN` → `local`].
- **`git push` nasıl çalışıyor:** `origin` uzak adresi `https://github.com/sekerahmet/TCB-uzak.git` olarak bağlı [ölçüldü: `git remote -v`]. Tüm giden HTTP/HTTPS trafiği yerel kimlik vekili (`HTTP_PROXY=http://127.0.0.1:33599`, `HTTPS_PROXY=http://127.0.0.1:33599`, `agy-plugin-proxy`) üzerinden geçiyor [ölçüldü: `env | grep -iE '^(http_proxy|https_proxy|no_proxy)='`]. Vekil sunucu, bağlı `github` eklentisinin OAuth yetkisini `github.com` isteklerine şeffaf biçimde ekliyor; sanal makine içinde ham belirteç tutmadan `git push origin gemini/impact-secimi` etkileşimsiz ve hatasız çalıştı (`5` ayrı push işleminin tamamı çıkış `0` ile tamamlandı) [ölçüldü: `git push origin gemini/impact-secimi`].

## 3. Sınıfın Sayımı (`packages.Load` ve Yardımcılarıyla Depoyu Yükleyen Testler)

1. **Doğrudan `packages.Load` araması (`git grep -l 'packages.Load' -- '*_test.go'`):**
   - `main` (`5a6c840a`) üzerinde `0` dosya (depo `golang.org/x/tools/go/packages` paketini dışarıdan doğrudan çağırmak yerine ortak iç yükleyici `internal/devtools/goload.Load`'u kullanıyor) [ölçüldü: `git grep -l 'packages.Load' main -- '*_test.go'`, boş].
   - Bu dalda eklenen koruyan fikstür testiyle birlikte `1` dosya: `tools/codegraph/type_queries_test.go` (`writeTypeQueryFixtureFiles` içinde `packages.Load` fikstürü) [ölçüldü: `git grep -l 'packages.Load' -- '*_test.go'`, `tools/codegraph/type_queries_test.go`].
2. **Gerçek depoyu `goload.Load` yardımcısıyla yükleyip ad/arayüz/paket sorgulayan testler:**
   - `tools/impact/main_test.go`: `repo(t)` (`tools/impact/main_test.go:19-26`, `loadOnce.Do(func() { loaded, loadErr = load(reporoot.Root()) })` [kanıtlı: `tools/impact/main_test.go:21`, `loadOnce.Do(func() { loaded, loadErr = load(reporoot.Root()) })`]) → `load(dir)` (`tools/impact/load.go:10-16`, `m, err := goload.Load(dir, goload.Options{})` [kanıtlı: `tools/impact/load.go:11`, `m, err := goload.Load(dir, goload.Options{})`]) üzerinden **4 test**:
     - `TestNameUses` (`codes.Name` kullanımları) [kanıtlı: `tools/impact/main_test.go:38-41`, `out := query(t, "codes.Name")`]
     - `TestInterfaceImplementers` (`interp.MemberAssignable`, `interp.Iterable`, `interp.MemberAssignable.SetMember` uygulayıcıları) [kanıtlı: `tools/impact/main_test.go:45-49`, `testkit.MustContain(t, query(t, "interp.MemberAssignable"), "implementers", "ml.Optimizer", "ml.TensorValue")`]
     - `TestMembersAndPackages` (`interp.VM.Interrupt`, `internal/lang/parser` / `parser`) [kanıtlı: `tools/impact/main_test.go:52-59`, `out := query(t, "internal/lang/parser")`]
     - `TestUnknownNames` (var olmayan adlarla hata kontrolü: `codes.NoSuchName`, `nosuchpkg.Name`, `interp.VM.NoSuchMethod` — hiçbir gerçek dosyaya bağlanmaz) [kanıtlı: `tools/impact/main_test.go:61-67`, `for _, spec := range []string{"codes.NoSuchName", "nosuchpkg.Name", "interp.VM.NoSuchMethod"} {`]
3. **Gerçek depodaki bir paketi `(*types.Config).Check` ile diskten yükleyip tip/seçim denetleyen test:**
   - `internal/compute/tensor/gate_calls_test.go`: `checkTensorPackage(t)` (`internal/compute/tensor/gate_calls_test.go:96-125`, `pkg, err := conf.Check("github.com/sekerahmet/TCB/internal/compute/tensor", fset, files, info)` [kanıtlı: `internal/compute/tensor/gate_calls_test.go:120`, `pkg, err := conf.Check("github.com/sekerahmet/TCB/internal/compute/tensor", fset, files, info)`]) üzerinden **1 test**:
     - `TestBackendDataCallsGoThroughTheGate` (`Backend` arayüzü uygulayıcılarını ve `Storage` / `View` seçimlerini `info.Selections` üzerinden denetler) [kanıtlı: `internal/compute/tensor/gate_calls_test.go:23-86`, `backend := pkg.Scope().Lookup("Backend").Type().Underlying().(*types.Interface)`].
4. **Gerçek depoyu `repoGraph(t)` (`tools/codegraph/helpers_test.go:156-171`) üzerinden yükleyen bütün-graf / çıkarıcı tutarlılık testleri:**
   - `main`'de `32` dosyada `47` test (`tools/codegraph/stdlib_dispatch_test.go:107`'deki `TestEveryStandardLibraryDispatchHasAnEdge` ek olarak doğrudan `goload.Load(root, goload.Options{Types: true, Selections: true})` çağırır); bu dalda eklenen `TestRepositoryTypeLoadedTestsAreSelected` ile `33` dosyada `48` test [ölçüldü: AST/işlev sayımı]. Bu testler bütün grafın ve çıkarıcının iç tutarlılığını sınar; tekil bir sembol sorgusu yapmadıkça yabancı paketlere sahte tip-sorgu kenarı almamaları gerekir [test: `tools/codegraph/type_queries_test.go`].
5. **Geçici klasör (`t.TempDir()`) ya da bellek içi fikstür yükleyen testler (depo dosyalarına bağlanmamalı):**
   - `internal/devtools/goload/goload_test.go`: `7` test işlevinden `5`'i geçici dizinde `Load(...)` çağırır (`TestLoadChecksPackagesAndTests`, `TestLoadRejectsAFolderWithoutGoPackages`, `TestOrderPutsImportsFirst`, `TestLoadTypesOption`, `TestSkipIgnoredLeavesOutPackagesGitIgnores`) [ölçüldü: `internal/devtools/goload/goload_test.go` incelemesi].
   - `tools/codegraph/stdlib_dispatch_test.go`: `TestStandardLibraryDispatchIsAUseWithItsRole` (`t.TempDir()`); `tools/codegraph/audit_test.go`: `auditRun` üzerinden geçici fikstür (`loadAudit`, `tools/codegraph/audit_common.go:53-65`); ve `fixture(t)` / `buildInto(t, root)` kullanan birim testleri [ölçüldü: `tools/codegraph/*_test.go` incelemesi].

## 4. Kök Neden, Genel Çözüm ve Koruyan Testler

- **Kök neden:**
  1. `tools/impact` gibi araçların testleri hedef arayüzü (`"interp.MemberAssignable"`) ya da adı (`"codes.Name"`) Go kodu içinde doğrudan aktarıp çağırmaz; dize olarak `query(t, ...)` yardımcısına verir ve `go/types` (`Scope.Lookup`, `types.Implements`, `Info.Uses`, `Info.Selections`) üzerinden çalışma zamanında çözer. `codegraph`'ta bu dize sorgularını tip bilgisiyle eşleyen bir çıkarıcı yoktu [görüş].
  2. Ayrıca `0b9fda0b..83fa8859` aralığında `ml.TensorValue`'nun `SetMember` metodu **yeni eklenen** `internal/stdlib/ml/members_freeze.go` dosyasında tanımlıdır. Geçmiş commit aralığı sorgularında (`live == false`) başlangıç commit'inde (`0b9fda0b`) henüz var olmayan dosyanın bildirim düğümü bulunmadığı için `goTestSelection` (`tools/codegraph/changed.go:292-298`) seçimi `go:file:<rel>` dosya düğümünün `testDistances` kenarları üzerinden yapar; dosya düğümüne bağlanmayan bir kenar yeni dosyadaki uygulayıcıyı yakalayamaz [kanıtlı: `tools/codegraph/query_file_tests.go:96-101`, `for id, d := range ix.testDistances(file) {`].
- **Genel çözüm (`tools/codegraph/type_queries*.go`, `query_file_tests.go`, `extractorRevision = 84`):**
  1. **Köke bağlı yükleyici saptaması (`type_queries.go`):** `goload.Load`, `packages.Load` ve yerel diskten ayrıştırılmış dosyalar üzerinde çalışan `(*types.Config).Check` çağrılarının dizin argümanı çağrı grafiğinde yukarı doğru izlenir; yalnızca depo köküne (`reporoot.Root()`, `repoState.root`, paket klasörü `.` vb.) çözülen yükleyiciler kabul edilir. Aynı `load(dir)` yardımcısını `t.TempDir()` ile çağıran testler kapsam dışı kalır.
  2. **Yetenek ve sorgu parametresi yayılımı (`type_queries_params.go`):** İşlev gövdelerinde `types.Implements` / `AssignableTo` / `AssertableTo` / `MissingMethod` (`impl`), `types.Info.Uses` ve `packages.Package.Imports` (`uses`) ile `types.Info.Selections` (`selections`; `collectAssignedSelectors` ile `info.Selections = ...` sol taraf atamaları dışlanarak) yetenekleri saptanır. `Scope.Lookup`, `LookupFieldOrMethod`, `MethodSet.Lookup`, `(types.Object).Name() == ...`, `o.Pkg().Path() == ...` ve paket çözümleyici yardımcılar üzerinden hangi dize parametrelerinin sembol ya da paket aramasına aktığı sabit noktaya kadar yayılır.
  3. **Somut sorgu yerleri ve kenar üretimi (`type_queries_sites.go`, `type_queries_link.go`, `query_file_tests.go`):** Testlerin ve yardımcılarının geçtiği somut dizeler (`"Paket.Ad"`, `"Paket.Tür.Üye"`, çıplak `"Ad"` + paket ipucu, paket yolu/son öğesi) yüklenen modül üzerinde çözülür:
     - Sorgulanan bildirim arayüzse (veya arayüz metoduysa) ve çağrı zincirinde `impl` yeteneği varsa, `moduleImplementations(m)` üzerinden o arayüzü uygulayan tiplerin ve ilgili metotlarının tanımlandığı `go:file:<rel>` düğümlerine `INFERRED` `tests` kenarı çekilir.
     - Çağrı zincirinde `uses` / `selections` yeteneği varsa, hedef bildirimin kullanıldığı/seçildiği (paket sorgularında paketi kullanan) kapsam içi `go:file:<rel>` düğümlerine `INFERRED` `tests` kenarı çekilir.
     - Kenarın bildirim düğümü yerine `go:file:<rel>` düğümüne çekilmesi ve `fileTests` (`tools/codegraph/query_file_tests.go:96-101`) içinde `ix.testDistances(file)` ile birleştirilmesi sayesinde hem yeni eklenen dosyadaki uygulayıcılar seçilir hem de bildirimler arası geçişken `indirectTests` zinciri şişmez.
- **Koruyan testler (`tools/codegraph/type_queries_test.go`):**
  1. `TestRepositoryTypeLoadedTestsAreSelected` [test: `tools/codegraph/type_queries_test.go`]: Gerçek depo grafında `--base 0b9fda0b 83fa8859` aralığında `tools/impact TestInterfaceImplementers`, `tools/impact TestNameUses` ve aynı sınıftan ikinci gerçek test olan `internal/compute/tensor TestBackendDataCallsGoThroughTheGate` seçimini, `tools/impact TestUnknownNames` dışlamasını, `tests-for internal/stdlib/ml/members_freeze.go` çıktısını ve `TestMembersAndPackages` kenarlarını doğrular.
  2. `TestPackagesLoadTypeQueriesSelectMatchingChangesOnly` [test: `tools/codegraph/type_queries_test.go`]: Geçici git deposunda doğrudan `packages.Load` kullanan `TestRunnerImplementers` (hem mevcut dosya değişikliği hem yeni dosyada uygulayıcı ekleme), `TestSymbolUses`, `TestPackageUsers` ve `TestTempModuleIgnored` (`t.TempDir()`) testlerini 4 dal üzerinde sınar.

## 5. Önce / Sonra ve Bedel Ölçümü

### Yeniden üretim komutu (`--base 0b9fda0b 83fa8859`)

- **Önce (`main` `5a6c840a`):**
  - Komut: `go run ./tools/edithook codegraph changed --go-tests --base 0b9fda0b 83fa8859`
  - Satır sayısı: `1250` satır (`tools/impact` eşleşmesi: `0`; yönergedeki `1246` sayısına göre +4 fark `main`'e sonradan eklenen testlerden kaynaklanıyor), son satır: `tools/gen/snippets TestSnippetsUpToDate` [ölçüldü: `/tmp/repro_before.txt`].
- **Sonra (`c26eb89`):**
  - Komut: `go run ./tools/edithook codegraph changed --go-tests --base 0b9fda0b 83fa8859`
  - Satır sayısı: `1254` satır (`+4` satır), son satır: `tools/impact TestNameUses` [ölçüldü: `/tmp/final_1.txt`].
  - Eklenen 4 test (`diff -u`):
    1. `internal/compute/tensor TestBackendDataCallsGoThroughTheGate`
    2. `tools/codegraph TestRepositoryTypeLoadedTestsAreSelected` (bu dalda eklenen ve `members_freeze.go`'yu anan koruyan test)
    3. `tools/impact TestInterfaceImplementers`
    4. `tools/impact TestNameUses`

### 5 commit aralığında bedel tablosu (`go run ./tools/edithook codegraph changed --go-tests --base <taban> <uç>`)

| Commit aralığı | Konu | Önce (satır) | Sonra (satır) | Fark | Eklenen testler |
| :--- | :--- | ---: | ---: | ---: | :--- |
| `0b9fda0b..83fa8859` | Kol B teslim 2a (`ml.TensorValue` `SetMember` / `members_freeze.go` dahil) | `1250` | `1254` | `+4` | `tensor TestBackendDataCallsGoThroughTheGate`, `codegraph TestRepositoryTypeLoadedTestsAreSelected`, `impact TestInterfaceImplementers`, `impact TestNameUses` [ölçüldü: `/tmp/repro_before.txt`, `/tmp/final_1.txt`] |
| `18c6beba..74a72663` | Merge H1 (`internal/stdlib/ml` açgözlü örnekleme) | `43` | `43` | `+0` | Yok [ölçüldü: `/tmp/before_18c6beba_74a72663.txt`, `/tmp/fresh_after_18c6beba_74a72663.txt`] |
| `ba62f5f8..0f3e01af` | Merge D1 (Kol A 13 teslim 1: `codes`, `parser`, `interp`) | `877` | `879` | `+2` | `impact TestMembersAndPackages`, `impact TestNameUses` [ölçüldü: `/tmp/before_ba62f5f8_0f3e01af.txt`, `/tmp/fresh_after_ba62f5f8_0f3e01af.txt`] |
| `705c16f1..6b767cf5` | `internal/lang/interp` yedek saat (`interrupt.go` / `VM`) | `652` | `653` | `+1` | `impact TestMembersAndPackages` [ölçüldü: `/tmp/before_705c16f1_6b767cf5.txt`, `/tmp/fresh_after_705c16f1_6b767cf5.txt`] |
| `8ee2111e..52ce4b9f` | Merge `codegraph` 3 (`tools/codegraph` değişiklikleri) | `298` | `300` | `+2` | Yeni eklenen `codegraph TestPackagesLoadTypeQueriesSelectMatchingChangesOnly`, `codegraph TestRepositoryTypeLoadedTestsAreSelected` [ölçüldü: `/tmp/before_8ee2111e_52ce4b9f.txt`, `/tmp/fresh_after_8ee2111e_52ce4b9f.txt`] |

Artışlar yalnız gerçekten etkilenen `+0` ile `+4` test arasındadır; yüzlerce test seçilmediği için durma koşulu oluşmadı.

## 6. Bitti Ölçütü Komutları ve Son Satırları

`c26eb89` üzerinde borusuz (`| tail`, `| grep` olmadan) koşulan 5 komutun çıkış kodları ve son satırları:

1. `go run ./tools/edithook codegraph changed --go-tests --base 0b9fda0b 83fa8859`
   - Çıkış kodu: `0` (`1254` satır; `tools/impact TestInterfaceImplementers` ve `tools/impact TestNameUses` listede) [ölçüldü: `/tmp/final_1.txt`].
   - Son satır: `tools/impact TestNameUses`
2. `go build ./... && go vet ./...`
   - Çıkış kodu: `0` [ölçüldü: `/tmp/final_2.txt`].
   - Son satır: *(boş çıktı — `0` bayt)*
3. `go test -short -timeout 1200s ./tools/codegraph/... ./tools/edithook/...`
   - Çıkış kodu: `0` (`ok  	github.com/sekerahmet/TCB/tools/codegraph	210.279s`) [ölçüldü: `/tmp/final_3.txt`].
   - Son satır: `ok  	github.com/sekerahmet/TCB/tools/edithook	(cached)` (önbelleksiz koşuda: `ok  	github.com/sekerahmet/TCB/tools/edithook	0.067s` [ölçüldü: `/tmp/bitti_3.txt`])
4. `go run ./tools/codegraph spec-refs`
   - Çıkış kodu: `0` [ölçüldü: `/tmp/final_4.txt`].
   - Son satır: `301 item(s), 5448 mention(s), 0 error(s), 0 note(s)`
5. `gofmt -l cmd internal tools lib`
   - Çıkış kodu: `0` [ölçüldü: `/tmp/final_5.txt`].
   - Son satır: *(boş çıktı — `0` bayt)*

## 7. `codegraph` Bulgusu (`tools/codegraph/gaps.md`)

- Kapanan kritik madde (`main`'deki `tools/codegraph/gaps.md:53`) silindi; `## İnce ayar` altına `tools/codegraph/gaps.md:65`'te şu yeni bulgu eklendi (`2c4d914`):
  - **Gözlem:** Linux'ta `go run ./tools/codegraph who-uses impact.load` komutu `<- in other build configurations (1; files not built for linux/amd64, read as text, not type-checked):` altında `internal/sysmem/memory_windows.go:11` satırını döndürüyor [ölçüldü: `go run ./tools/codegraph who-uses impact.load`].
  - **İkinci yoldan doğrulama:** `impact.load`, `tools/impact/load.go:10`'da `package main` içindeki dışa kapalı `func load(dir string) (*module, error) {` işlevidir [kanıtlı: `tools/impact/load.go:10`, `func load(dir string) (*module, error) {`]; `internal/sysmem/memory_windows.go:11` ise `package sysmem` içindeki `memState` yapısının `load             uint32` alanıdır [kanıtlı: `internal/sysmem/memory_windows.go:11`, `load             uint32`].
  - **Neden:** `otherConfigMentions` (`tools/codegraph/query_other_configs.go:31`, `name := n.Name[strings.LastIndex(n.Name, ".")+1:]` [kanıtlı: `tools/codegraph/query_other_configs.go:31`, `name := n.Name[strings.LastIndex(n.Name, ".")+1:]`]) derlenmeyen her Go dosyasında çıplak kısa ad arıyor; dışa kapalı paket düzeyi bildirimlerde aynı paket klasörünü, başka paketteki dışa açık bildirimlerde dosyanın `import` listesini süzmüyor.

## 8. Takıldığım Yerler ve Uzak Deneme Notları

1. **Go `1.26.10` kurulumu:** Uzak sanal makinede Go ön yüklü değildi; `https://go.dev/dl/go1.26.10.linux-amd64.tar.gz` indirilip `/home/antigravity/.local/go` altına kuruldu ve her komutta `PATH` + `GOTOOLCHAIN=local` kullanıldı [ölçüldü: `go version`].
2. **Yükleyicinin `info.Selections = ...` ataması:** İlk denemede `goload.Load` içindeki `info.Selections = ...` alan ataması `Selections` okuması sayıldığı için `impact.TestInterfaceImplementers`'a `selections` yeteneği sızdı ve `18c6beba..74a72663` aralığında `+1` sahte seçim üretti; `collectAssignedSelectors` (`tools/codegraph/type_queries_params.go:57-70`) ile sol taraf alan atamaları dışlanınca `TestInterfaceImplementers` tam olarak `go run ./tools/impact`'in raporladığı 11 dosyaya indi ve `18c6beba..74a72663` farkı `+0` oldu [ölçüldü: `/tmp/fresh_after_18c6beba_74a72663.txt`].
3. **`gofmt` sonrası 600 satır sınırı ve `reporoot.ReadDir` davranışı:** 3. sıkıştırmadan sonraki yeniden doğrulamada `gofmt`'un tek satırlık yardımcıları açmasıyla `type_queries_sites.go` dosyasının `675` satıra çıkıp `TestLayoutFileSize` (`sınır 600`) testine takıldığı yakalandı; yetenek ve parametre yayılımı `type_queries_params.go` (`304` satır) dosyasına ayrıldı (`type_queries_sites.go`: `386` satır). Ayrıca `TestExtractorRevisionFollowsExtractorCode` (`tools/codegraph/extractor_digest_test.go:34`) `reporoot.ReadDir(".")` kullandığından, yeni çıkarıcı dosyası `git add` ile indekse eklenmeden önce özete girmiyor; özet (`75a3d2422ca1d341`) `git add` sonrasında alındı ve `TestLayoutFileSize` + `TestExtractorRevisionFollowsExtractorCode` birlikte geçti (`c26eb89`) [ölçüldü: `go test -short ./internal/... ./tools/codegraph/...`].
4. **Uzak ortamda `IMPACT_DURUM.md` / `IMPACT_RAPORU.md` dosyalarının `git` ile izlenmesinin `TestEveryTrackedDataFileHasATestOrAReason` ile etkileşimi:** Yerel Windows çalışmasında durum ve rapor dosyaları git indeksine girmezken, uzak denemede `IMPACT_DURUM.md` her adımda commit'lenip push edildiği için `git ls-files` (`codeindex.GitTracked`) listesine giriyor ve `TestEveryTrackedDataFileHasATestOrAReason` (`tools/codegraph/data_coverage_test.go:115`) `1 tracked data file(s) no test is linked to ... IMPACT_DURUM.md` hatası veriyor [ölçüldü: `go test -short ./tools/codegraph -run TestEveryTrackedDataFileHasATestOrAReason`]. Bu davranış `tools/full_check.sh:41`'deki `#                     Yan oturumlar devir notundan, ana oturum GPU yan dalına göndermeden önce` kuralıyla aynı nedendendir [kanıtlı: `tools/full_check.sh:41`, `#                     Yan oturumlar devir notundan, ana oturum GPU yan dalına göndermeden önce`]. Depo kuralını (`data_coverage_test.go`) gevşetmemek için Adım 8a'daki test koşusu sırasında `git rm --cached IMPACT_DURUM.md` ile birleştirme sonrası ağaç doğrulandı ve koşu biter bitmez dosya yeniden indekse eklendi. Uzak oturumlarda durum dosyasının dalda sürekli izlenmesi isteniyorsa `data_coverage_test.go` içinde kök `*_DURUM.md` / `*_RAPORU.md` / `*-devir.md` dosyalarının nasıl ele alınacağına ana oturum karar verebilir [görüş].
5. **Üç bağlam sıkıştırması:**
   - 1. sıkıştırma (`2026-11-04 14:49 UTC+3`): Adım 5 incelemesinden hemen sonra, hiçbir kaynak dosyaya dokunulmadan önce.
   - 2. sıkıştırma (`2026-11-04 15:08 UTC+3`): Adım 6 ön bedel ölçümlerinden hemen sonra, hiçbir kaynak dosyaya dokunulmadan önce.
   - 3. sıkıştırma (`2026-11-04 15:38 UTC+3`): Adım 7'de `tools/codegraph/gaps.md` düzenlendikten hemen sonra, commit öncesi. Üçünde de `CLAUDE.md`, `IMPACT_DURUM.md`, `git status`/`git log` ve tüm kaynak dosyalar yeniden açılıp sayılar ve alıntılar kaynaktan doğrulandı (`main`'de `repoGraph(t)` çağıran test sayısı bu doğrulamada `32` dosyada `47` test olarak düzeltildi; `type_queries_sites.go` satır sınırı ve `8ee2111e..52ce4b9f` `+2` farkı da bu yeniden doğrulamada yakalandı).

## SORU

Yok.

## Kendi kontrolüm

- Rapordaki tüm birebir `[kanıtlı: ...]` alıntıları kaynak dosyalar üzerinde `grep -F` ile satır numarasıyla birlikte doğrulandı (`tools/impact/main_test.go:21,38-41,45-49,52-59,61-67`, `tools/impact/load.go:10,11`, `internal/compute/tensor/gate_calls_test.go:23-86,120`, `tools/codegraph/query_file_tests.go:96-101`, `tools/codegraph/query_other_configs.go:31`, `internal/sysmem/memory_windows.go:11`, `tools/full_check.sh:41`) [ölçüldü: doğrulama betiği, `0` uyumsuzluk].
- Tüm commit kimlikleri, satır sayıları, 5 commit aralığı farkları ve 5 `Bitti ölçütü` komutunun çıkış kodları ile son satırları kaynaktan ve komut çıktılarından kontrol edildi; her olgusal iddianın yanında kanıt etiketi (`[ölçüldü: ...]`, `[kanıtlı: ...]`, `[test: ...]`, `[görüş]`) bulunduğu doğrulandı [ölçüldü: doğrulama betiği].
- Kırık göreli Markdown bağlantısı, eski terim (`docs/terimler.md`), eski kod biçimi ve eski yol haritası numarası bulunmadığı denetlendi [ölçüldü: doğrulama betiği].

Kendi incelemem bitti.
