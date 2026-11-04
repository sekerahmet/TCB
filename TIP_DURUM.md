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

## Sıradaki Adımlar

1. `type_queries*.go` sadeleştirmesinin (`{test, kapsam}` gerekçeli listesi, fikstür parametresi, `packages.Load` yolunun ve sınanmayan 8 sezginin silinmesi, kalan fonksiyonlara kısa Türkçe yorumlar), kapsam testinin (`goload.Load` ve `(*types.Config).Check` + 1 satır silinince düşme kontrolü) ve beş satırlık fikstür testinin (`slowFixture` + değişiklikten sonra grafik kurarak sığ klon uyumu) uygulanıp ayrı commit olarak kaydedilmesi.
2. `tools/codegraph/gaps.md` içindeki kapanan maddenin (`tools/codegraph/gaps.md:53-54`) silinmesi ve `extractorRevision` / `extractor_digest.txt` güncellemesi.
3. Geçici kopyada 3 satırlık gerçek depo ölçüsü, `--depth 1` klon koşusu, "Sonra" ölçümleri, bitti ölçütü komutları, `TIP_RAPORU.md` ve kendi kontrolüm.

## Emin Olmadıklarım / Açık Noktalar

- Açık belirsizlik yok; `kindScan` / `edgeScans` `tools/full_check.sh` ile birebir eşlendiğinden (`scans_test.go:275`), tip sorgusu kapsamları `kindTypeQuery` / `edgeTypeQueries` düğüm ve kenarıyla temsil edilip `query_scans.go` kapsama mantığını paylaşacak.

