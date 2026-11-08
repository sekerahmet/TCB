# UZAK 8 — Durum

- **Dal:** `gemini/uzak-8`
- **Taban (`main`):** `47e0984cddfc726ca8e4dc7fdf423af4a1ab67a9`
- **Başlama saati:** 2026-11-08 16:11:08 UTC+3 (13:11:08 UTC)
- **Bağlam sıkıştırması:** Yok

## İlerleme

- [x] `CLAUDE.md`, `GEMINI_KURALLARI.md` ve `UZAK_YONERGE_8.md` okundu.
- [x] **Adım 1: Yeniden üret ve taban ölçümleri al (tamamlandı):**
  - `tools/codegraph/gocode_scan.go:58-67`, `:172-192`, `:248-345`, `tools/codegraph/query_names.go:13-79`, `tools/codegraph/test_names.go:41-89`, `tools/codegraph/other_os.go:119-134`, `internal/stdlib/ml/members.go:19-108`, `internal/stdlib/ml/device.go:77-102`, `internal/stdlib/ml/builtins.go:58-63`, `internal/stdlib/ml/save.go:327-367`, `internal/stdlib/files/file.go:37-51`, `internal/stdlib/files/temp.go:33-50`, `internal/stdlib/tokenizer/tokenizer.go:113-130` okundu.
  - **Taban graf kurulum süresi (3 sıcak koşunun ortancası):** `9.6 s` (`9.6 s`, `9.6 s`, `10.1 s`; `35831 nodes, 293982 edges (188624 EXTRACTED, 105358 INFERRED)`, `calls 34521`, `uses_name 9038`, `reads_name 483`).
  - **Taban `tests-for` ve `query` çıktıları (`/tmp/step1_before.txt`):**
    - `go run ./tools/codegraph tests-for ml.TensorValue.toDevice` → `no tests traced to it` (ve `weak evidence (142): linked into the binary of cmd/trlang`).
    - Küçük fikstür (`tools/codegraph/testdata/mod/shape/leaf.go:5-7` içinde `type leafBox struct{}` ve `func (leafBox) step() {}`, `shape.go:105` içinde `func useMethod(b leafBox) { apply(b.step) }`, `shape_test.go:26` içinde `func TestUseMethod(t *testing.T) { useMethod(leafBox{}) }`):
      - `go run ./tools/codegraph tests-for --root tools/codegraph/testdata/mod --graph /tmp/mod_graph.json shape.leafBox.step` → `no tests found`.
    - `go run ./tools/codegraph tests-for ml.tensorLoad` → `no tests traced to it` (ve `weak evidence (142): linked into the binary of cmd/trlang`).
    - `tokenizer` paketindeki `load`'un gerçek adı: `go run ./tools/codegraph query tokenizer.load` → `tokenizer.load [func] internal/stdlib/tokenizer/tokenizer.go:120` (`id: go:func:github.com/sekerahmet/TCB/internal/stdlib/tokenizer.load`).
    - `go run ./tools/codegraph tests-for tokenizer.load` → `no tests traced to it` (`weak evidence (142): linked into the binary of cmd/trlang`, `in other build configurations (1): internal/sysmem/memory_windows.go:11`).
    - `go run ./tools/codegraph tests-for files.tempDirBuiltin` → `tests about it (6)` ve `tests through the name codes it implements (44; same package first)` (gövdesinde `internal/stdlib/files/temp.go:34` satırında `interp.CallPrefix(codes.FnTempDir)` bulunduğu için `uses_name` kenarı gövdeden vardır; `tests, indirect` ve `weak evidence` çağrı zinciri grubu yoktur).
    - `go run ./tools/codegraph query TR_mem_to_device` → `reads_name (5)` (içinde `ml.TensorValue.toDevice [method] at internal/stdlib/ml/device.go:78`), `tests (21)`, `uses_name (17)`, `registered or used in Go (3; implementation named on the same line): ml.TensorValue.GetProperty at internal/stdlib/ml/members.go:80, ml.memberToDevice at internal/stdlib/ml/members.go:29, ml.tensorMemberCodes at internal/stdlib/ml/members.go:37` (`-> ml.TensorValue.toDevice` bağlantısı yok).
    - `go run ./tools/codegraph tests-for --all interp.PathArg` → `tests about it (1)`, `tests through the name codes it implements (111; same package first)`, `tests, indirect (1): ml.TestSaveCopiesBeforeReleasingTheLock`, `weak evidence (5)`.
  - **Adım 3 taban ölçümleri (`/tmp/step3_files_before.txt`):**
    - Uçta izlenmeyen Go `INFERRED` `calls` kenarı (`Via != "arg"`): toplam `443` kenar (`223` ayrı hedef), bunların `311`'i metot hedefidir (`kind == "method"`); test dışı dosyalarda `193` kenar (`161` hedef), `132`'si metot hedefidir.
    - 19 örneklem dosyasının (`awk 'NR%40==1'`) taban `tests-for --all` sayıları `/tmp/step3_files_before.txt` dosyasına kaydedildi.
- [ ] Adım 2a: Metot değeri test erişimine girer (ayrı commit).
- [ ] Adım 2b: Yanındaki ad kodu, işlevin uygulaması sayılır (ayrı commit).
- [ ] Adım 3: Büyümeyi ölç ve `UZAK8_RAPORU.md` yaz.
