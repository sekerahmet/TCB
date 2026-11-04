# Uzak İş 2 Durum Dosyası (`TIP_DURUM.md`)

- **Başlangıç saati:** `2026-11-04 15:39 UTC` (`2026-11-04 18:39 UTC+3`)
- **Dal:** `gemini/tip-secimi`
- **Taban (`main`):** `a259ed99e2c3b340e3c5223094158512dc4e8507`
- **Bağlam sıkıştırması:** 1 kez — `2026-11-04 16:03 UTC` (`2026-11-04 19:03 UTC+3`), Go `1.26.10` kurulumu ve ilk kod okumalarından hemen sonra, herhangi bir üretim/test dosyası düzenlenmeden önce gerçekleşti. Sıkıştırma sonrasında `CLAUDE.md`, `origin/yonerge:UZAK_YONERGE_2.md`, `TIP_DURUM.md`, ilgili Go kaynakları ve bütün ön ölçümler kaynağından yeniden açılıp doğrulandı.

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

## Sıradaki Adımlar

1. N6 (`[INFERRED]` kenarın `direct` yazılmaması ve `tools/codegraph/query_file_tests.go:87` yorumunun güncellenmesi) ve `scans` benzeri kapsam temsilinin ilgili dosyaları (`query_tests.go`, `test_names_test.go`, `scans_test.go`, `graph.go`, `build.go`, `helpers_test.go`) kaynağından okuyup kesinleştirilmesi; dokunulacak dosyalar için `tests-for` ve `run:` satırlarının koşulması.
2. N6 düzeltmesinin uygulanması ve ayrı commit olarak kaydedilmesi.
3. `type_queries*.go` sadeleştirmesinin (`{test, kapsam}` gerekçeli listesi, fikstür parametresi, `packages.Load` yolunun ve sınanmayan sezgilerin silinmesi, kısa Türkçe yorumlar), kapsam testinin ve beş satırlık fikstür testinin (`slowFixture` + sığ klon uyumu) uygulanıp commit edilmesi.
4. `tools/codegraph/gaps.md` içindeki kapanan maddenin (`tools/codegraph/gaps.md:53-54`) silinmesi ve `extractorRevision` / `extractor_digest.txt` güncellemesi.
5. Geçici kopyada 3 satırlık gerçek depo ölçüsü, `--depth 1` klon koşusu, "Sonra" ölçümleri, bitti ölçütü komutları, `TIP_RAPORU.md` ve kendi kontrolüm.

## Emin Olmadıklarım / Açık Noktalar

- N6 için `query_file_tests.go`, `changed.go`, `query_tests.go`, `test_names_test.go` ve `testdata/planning_questions.txt` birlikte incelenerek tüm `[INFERRED]` `edgeTests` kenarlarının tutarlı gösterimi netleştirilecek.
