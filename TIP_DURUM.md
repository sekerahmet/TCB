# Uzak İş 2 Durum Dosyası (`TIP_DURUM.md`)

- **Başlangıç saati:** `2026-11-04 15:39 UTC` (`2026-11-04 18:39 UTC+3`)
- **Dal:** `gemini/tip-secimi`
- **Taban (`main`):** `a259ed99e2c3b340e3c5223094158512dc4e8507`
- **Bağlam sıkıştırması:** Yok.

## Biten Adımlar

1. **Başlangıç okumaları ve dal açılışı:**
   - `CLAUDE.md` ve `origin/yonerge:UZAK_YONERGE_2.md` (sonundaki `GEMINI_KURALLARI.md` ve `Uzak ortam` bölümleri dahil) okundu.
   - `main` (`a259ed99e2c3b340e3c5223094158512dc4e8507`) ucundan `gemini/tip-secimi` dalı açıldı.
   - Sanal makinede `go` ikilisinin başlangıçta kurulu olmadığı görüldü (`go: command not found`) [ölçüldü: `GOTOOLCHAIN=local go version`, çıkış kodu `127`]; `go.mod:5` içindeki `toolchain go1.26.10` sürümü kullanıcı alanına (`/home/antigravity/.local/go`) kurulacak.

## Sıradaki Adımlar

1. Go `1.26.10` (`linux/amd64`) kurulumu ve `GOTOOLCHAIN=local` doğrulaması.
2. Ön ölçümler ("Bedel" ve `build` süresi: değişiklikten önceki `changed --go-tests` satır sayıları ve seçilen test süreleri, `go run ./tools/codegraph build` süresi).
3. İlgili dosyaların (`tools/codegraph/gaps.md`, `tools/codegraph/type_queries*.go`, `tools/codegraph/scans*.go`, `tools/codegraph/query_file_tests.go`, `tools/codegraph/helpers_test.go`, `tools/edithook/*`) aşamalı okunması ve `tests-for` koşuları.
4. Tasarım, kod sadeleştirmesi (`{test, kapsam}` gerekçeli listesi, `[INFERRED]` kenarın `direct` sayılmaması, `packages.Load` yolunun kaldırılması, yorumlar, `-short` / `slowFixture`), kapsam testi, beş satırlık fikstür testi (`--depth 1` klon dahil), gerçek depo ölçüsü doğrulaması ve `gaps.md` + `extractorRevision` / `extractor_digest.txt` güncellemeleri.
5. Bitti ölçütü komutları, `TIP_RAPORU.md` ve kendi kontrolüm.

## Emin Olmadıklarım / Açık Noktalar

- Henüz yok; kod okuma adımında netleştirilecek.
