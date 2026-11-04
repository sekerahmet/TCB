# Uzak İş 2 Durum Dosyası (`TIP_DURUM.md`)

- **Başlangıç saati:** `2026-11-04 15:39 UTC` (`2026-11-04 18:39 UTC+3`)
- **Dal:** `gemini/tip-secimi`
- **Taban (`main`):** `a259ed99e2c3b340e3c5223094158512dc4e8507`
- **Bağlam sıkıştırması:** 2 kez:
  1. `2026-11-04 16:03 UTC` (`2026-11-04 19:03 UTC+3`): Go `1.26.10` kurulumu ve ilk kod okumalarından hemen sonra, herhangi bir üretim/test dosyası düzenlenmeden önce gerçekleşti; sonrasında bütün ön ölçümler kaynağından yeniden doğrulandı.
  2. `2026-11-04 16:36 UTC` (`2026-11-04 19:36 UTC+3`): N6 düzeltmesinin 4 dosyadaki (`tools/codegraph/query_file_tests.go`, `tools/codegraph/changed.go`, `tools/codegraph/query_file_tests_test.go`, `tools/codegraph/testdata/planning_questions.txt`) düzenlemeleri çalışma ağacına yazıldıktan hemen sonra, test ve commit öncesinde gerçekleşti; sonrasında `CLAUDE.md`, `origin/yonerge:UZAK_YONERGE_2.md`, `TIP_DURUM.md`, `query_tests.go` ve düzenlenen 4 dosya yeniden açılıp okundu.

## Biten Adımlar

1. **Başlangıç okumaları ve dal açılışı:**
   - `CLAUDE.md` ve `origin/yonerge:UZAK_YONERGE_2.md` (sonundaki `GEMINI_KURALLARI.md` ve `Uzak ortam` bölümleri dahil) okundu.
   - `main` (`a259ed99e2c3b340e3c5223094158512dc4e8507`) ucundan `gemini/tip-secimi` dalı açıldı.
   - Sanal makine tek-dal refspec ile sığ klonlandığından `remote.origin.fetch` `+refs/heads/*:refs/remotes/origin/*` olarak güncellenip `origin` getirildi.
2. **Go `1.26.10` kurulumu:**
   - Sanal makinede `go` ikilisi başlangıçta yoktu; `go.mod:5` içindeki `toolchain go1.26.10` [kanıtlı: `go.mod:5`, `toolchain go1.26.10`] kullanıcı alanına (`/home/antigravity/.local/go`) kuruldu ve doğrulandı [ölçüldü: `GOTOOLCHAIN=local go version`, `go version go1.26.10 linux/amd64`].
   - Ortamda `/usr/bin/time` ikilisi de bulunmuyor (`/bin/bash: line 5: /usr/bin/time: No such file or directory`); süre ölçümlerinde bash yerleşik `time` (`TIMEFORMAT`) ve `date +%s%3N` kullanıldı ("Ortam" başlığına yazılacak).
3. **Ön ölçümler (değişiklikten önce, sıkıştırma sonrası yeniden doğrulandı):**
   - Üretim satırları: `383` (`tools/codegraph/type_queries.go`) + `457` (`tools/codegraph/type_queries_link.go`) + `304` (`tools/codegraph/type_queries_params.go`) + `386` (`tools/codegraph/type_queries_sites.go`) = `1530` toplam [ölçüldü: `ls tools/codegraph/type_queries*.go | grep -v _test | xargs wc -l`, `1530 total`].
   - `go run ./tools/codegraph build`: sıcak koşuda iç süre `8.2s`, duvar saati `8.396s` (`31288 nodes, 231820 edges (163507 EXTRACTED, 68313 INFERRED)`) [ölçüldü: `TIMEFORMAT='real_s=%3R'; time go run ./tools/codegraph build`, `in 8.2s`, `real_s=8.396`].
   - `go run ./tools/edithook codegraph changed --go-tests --base <taban> <uç>` (önce):
     - `0b9fda0b..83fa8859`: `1262` satır (`1254 ms`, `impact=2`, `gate=1`) [ölçüldü: `go run ./tools/edithook codegraph changed --go-tests --base 0b9fda0b 83fa8859`, `lines=1262 elapsed_ms=1254 impact=2 gate=1`]
     - `18c6beba..74a72663`: `43` satır (`1168 ms`, `impact=0`, `gate=0`) [ölçüldü: `go run ./tools/edithook codegraph changed --go-tests --base 18c6beba 74a72663`, `lines=43 elapsed_ms=1168 impact=0 gate=0`]
     - `ba62f5f8..0f3e01af`: `883` satır (`1266 ms`, `impact=2`, `gate=0`) [ölçüldü: `go run ./tools/edithook codegraph changed --go-tests --base ba62f5f8 0f3e01af`, `lines=883 elapsed_ms=1266 impact=2 gate=0`]
     - `705c16f1..6b767cf5`: `660` satır (`1214 ms`, `impact=1`, `gate=0`) [ölçüldü: `go run ./tools/edithook codegraph changed --go-tests --base 705c16f1 6b767cf5`, `lines=660 elapsed_ms=1214 impact=1 gate=0`]
     - `8ee2111e..52ce4b9f`: `300` satır (`1229 ms`, `impact=0`, `gate=0`) [ölçüldü: `go run ./tools/edithook codegraph changed --go-tests --base 8ee2111e 52ce4b9f`, `lines=300 elapsed_ms=1229 impact=0 gate=0`]
   - Seçilen testlerin bu sanal makinedeki süresi:
     - `go test -count=1 ./tools/impact` → `2.603s` [ölçüldü: `go test -count=1 ./tools/impact`, `ok  	github.com/sekerahmet/TCB/tools/impact	2.603s`]
      - `go test -count=1 ./internal/compute/tensor -run '^TestBackendDataCallsGoThroughTheGate$'` → `0.672s` [ölçüldü: `go test -count=1 ./internal/compute/tensor -run '^TestBackendDataCallsGoThroughTheGate$'`, `ok  	github.com/sekerahmet/TCB/internal/compute/tensor	0.672s`]
4. **Madde 1 — N6 (`46aef6a`):**
   - `tools/codegraph/query_file_tests.go` ve `tools/codegraph/changed.go` içinde 0 adımlı `INFERRED` `tests` kenarlarının `direct` diye yazılması önlendi; `EXTRACTED` doğrudan kanıtlar (`extractedDirectTests`) önce, `INFERRED` olanlar `[INFERRED]` etiketiyle sonra sıralandı; `fileTestSet.all` yorumu güncellendi.
   - `tools/codegraph/testdata/planning_questions.txt:199` beklentisi güncellendi ve `tools/codegraph/query_file_tests_test.go` içine `TestInferredZeroStepEdgeIsNotWrittenAsDirect` koruyan testi eklendi (`ok  	github.com/sekerahmet/TCB/tools/codegraph	0.222s`, `dups --changed main` → `0 duplicate candidate(s)`).
   - Not (Ortam): `2026-11-04 16:40 UTC` sırasında uzak uVM kısa süreli `STOPPED` durumuna geçip otomatik yeniden başlatıldı; `/home/antigravity` ve çalışma ağacı korundu ("Ortam" bölümüne yazılacak).
5. **Madde 2 — `type_queries` kapsam seçimi, kapsam testi ve 5 satırlık fikstür testi (`e121209`):**
   - `tools/codegraph/type_queries_link.go`, `tools/codegraph/type_queries_params.go`, `tools/codegraph/type_queries_sites.go` silindi; `tools/codegraph/type_queries.go` `1530` satırdan `70` satıra indirildi (sınanmayan 8 sezgi ve kullanılmayan `packages.Load` yolu kaldırıldı; `repoTypeQueryRules` + `linkTypeQueries` Türkçe yorumlarla yazıldı; fikstür için `buildGraphFromWithRules` parametresi açıldı).
   - `TestRepositoryTypeLoadedTestsAreSelected` kapsam testine dönüştürüldü (`tools/codegraph` dışında `goload.Load` ve `(*types.Config).Check` erişimi + 1 kayıt silinince düşme kontrolü + `TestUnknownNames` ve `internal/compute/tensor` kapsam doğrulaması; `10.33s` geçti).
   - `TestTypeQueryScopeSelectsEveryGoChange` (`slowFixture(t)` çağrısıyla) değişiklikten **sonra** kurulan grafiklerde beş tablo satırının tamamını (`gövde değişti`, `metodun adı değişti`, `uygulayıcı dosya silindi`, `adın kullanımı silindi`, `başka pakete aynı çıplak ad eklendi`) + `claude/unrelated` + `TestTempModuleIgnored` seçilmemesini doğruladı (`1.76s` geçti).
6. **Madde 3 — `tools/codegraph/gaps.md` güncellemesi (`ce94585`):**
   - Kapanan "Depoyu tip bilgisiyle yükleyen testin bağı değişiklikten sonraki grafikten kuruluyor" maddesi silindi.
7. **Madde 4 — `extractorRevision` ve `extractor_digest.txt` güncellemesi (`9b6baf4`):**
   - `extractorRevision` `84` → `85` yapıldı ve `tools/codegraph/testdata/extractor_digest.txt` (`digest 16d8431e084b86b6`) bir kez güncellendi (`TestExtractorRevisionFollowsExtractorCode` geçti).

## Sıradaki Adımlar

1. Geçici kopyada 3 satırlık gerçek depo ölçüsü (`TensorValue.SetMember` ad değişimi → 4 `tools/impact` testi; `internal/compute/tensor` değişikliği → gate testi; ilgisiz paket değişikliği → gate testi kapsam dışı), `--depth 1` klon koşusu, "Sonra" ölçümleri ve bitti ölçütünün 6 komutunun süzmesiz koşulması.
2. `TIP_RAPORU.md` yazımı, kendi kontrolüm (`grep -F` ve dosya:satır doğrulaması), son satıra `Kendi incelemem bitti.` eklenmesi ve son push.

## Emin Olmadıklarım / Açık Noktalar

- Açık belirsizlik yok.
