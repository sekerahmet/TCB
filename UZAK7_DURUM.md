# UZAK 7 Durum Dosyası

- **Dal:** `gemini/uzak-7`
- **Taban commit (`main`):** `ffca754c3317bbb058ea3ff649c78d8c6a692362`
- **Başlama saati:** 2026-11-08 12:41:04 UTC+3 (2026-11-08 09:41:04 UTC / 01:41:04 PST)
- **Go sürümü (`GOTOOLCHAIN=local`):** `go version go1.27.3 linux/amd64`
- **Bağlam sıkıştırması:** 1 kez — 2026-11-08 13:00:53 UTC+3 (10:00:53 UTC), Adım 1 commit ve push edildikten hemen sonra (Adım 2'ye başlarken). `CLAUDE.md`, `UZAK_YONERGE_7.md` ve `UZAK7_DURUM.md` yeniden okundu.

## Bitenler
- `CLAUDE.md` ve `yonerge` dalındaki `UZAK_YONERGE_7.md` okundu.
- `gemini/uzak-7` dalı `main` (`ffca754c3317bbb058ea3ff649c78d8c6a692362`) ucundan açıldı.
- `GOTOOLCHAIN=local` ayarlandı.
- **Adım 1 (yeniden üret) tamamlandı:**
  - `tools/codegraph/gaps.md:45`, `tools/codegraph/test_reach.go:16-32`, `tools/codegraph/gocode_scan.go:75-87` ve `291-309` okundu.
  - Küçük fikstür `tools/codegraph/testdata/mod/shape/leaf.go` (`func leaf() {}`), `tools/codegraph/testdata/mod/shape/shape.go:101-103` (`func apply(fn func()) { fn() }`, `func use() { apply(leaf) }`) ve `tools/codegraph/testdata/mod/shape/shape_test.go:24` (`func TestUse(t *testing.T) { use() }`) ile kuruldu.
  - Fikstür taban çıktısı:
    - Dosya sorgusu (`go run ./tools/codegraph tests-for --root tools/codegraph/testdata/mod --graph /tmp/mod_graph.json shape/leaf.go`): `tests: none found for the declarations in this file` (`declarations in this file: 1`).
    - İşlev sorgusu (`go run ./tools/codegraph tests-for --root tools/codegraph/testdata/mod --graph /tmp/mod_graph.json leaf`): `no tests found`.
  - Depodaki gerçek örnekler (`internal/compute/tensor/logsumexp.go:63-65`, `74`, `100` ve `internal/compute/tensor/cpu_elementwise_wide.go:58`, `72`):
    - `tensor.LogSumExp` (`internal/compute/tensor/logsumexp.go:58`), yüksek dereceli `tensor.lseRows` (`internal/compute/tensor/logsumexp.go:74`, `row func(x, work []T) T`) yardımcısına `lseRow32` (`:65`) ve `lseRow64` (`:63`) işlevlerini argüman olarak geçirir (`tensor.LogSumExp -> tensor.lseRow32 [INFERRED]`).
    - `go run ./tools/codegraph tests-for tensor.LogSumExp` 1 doğrudan/konu edinen Go testi (`tensor.TestLogSumExpSameBitsForAnyWorkerCount`) ve 15 ad kodu testi (`logsumexp_test`) verirken, `go run ./tools/codegraph tests-for tensor.lseRow32` taban uçta `no tests traced to it` (yalnız `weak evidence (142): linked into the binary of cmd/trlang`) verir.
    - Benzer biçimde `tensor.unaryLoopWide` (`internal/compute/tensor/cpu_elementwise_wide.go:58`) içinde `f64(geluExactValue)` çağrısıyla geçirilen `tensor.geluExactValue` (`:72`) için `go run ./tools/codegraph tests-for tensor.geluExactValue` taban uçta `no tests traced to it` verir.
  - `sumBuiltin` (`internal/lang/interp/builtins.go:25`, konumsal tablo `{codes.FnSum, sumBuiltin}`) taban çıktısı:
    - `go run ./tools/codegraph query sumBuiltin`: `<- calls (1): interp.coreBuiltins [var] at internal/lang/interp/builtins.go:25 [INFERRED]`.
    - `go run ./tools/codegraph tests-for sumBuiltin`: `<- tests about it (6; ...)` ve `<- tests through the name codes it implements (175; same package first)` (çağrı zinciriyle gelen `<- tests, indirect` grubu yok).
  - Adım 3 için taban ("önce") ölçümleri (graf kurulum süresi ortancası 8.9s: 8.8s, 9.1s, 8.9s; 18 örnek dosya ve son 10 commit) `/tmp/baseline_graph.json` üzerinden alındı.

- **Adım 2 (düzelt) tamamlandı:**
  - `tools/codegraph/gocode_scan.go`: `const viaArg = "arg"` ve `markCall` eklendi; `*ast.CallExpr.Args` içindeki ifadelerin `nameIdent` tanımlayıcıları `viaArg` olarak işaretlendi ve `use` içinde hedef `*types.Func` paket düzeyi işlevse (`fn.Type().(*types.Signature).Recv() == nil`) `edgeCalls` kenarına `Via = viaArg` (`e.Via == ""` iken) yazıldı.
  - `tools/codegraph/test_reach.go`: `reachEdge` imzasına `via string` eklendi; `edgeCalls` kolunda `confidence == extracted || via == viaArg || strings.HasPrefix(from, "tr:") && strings.HasPrefix(to, "tr:")` ile argüman olarak geçen paket düzeyi Go işlevleri zincire alındı.
  - `tools/codegraph/graph.go` (`extractorRevision = 89`) ve `tools/codegraph/testdata/extractor_digest.txt` güncellendi.
  - `tools/codegraph/query_tests_test.go`: `TestTestsForFollowsPackageFunctionPassedAsCallArgument` koruyan testi eklendi (fikstürde `shape.TestUse` bulunur; `argLeaf` zincire girer; bileşik değerdeki `litLeaf`, değişkene atanan `varLeaf` ve yöntem değeri `box.methodLeaf` zincire girmez).
  - Gerçek örnek doğrulaması: `tensor.lseRow32` artık `tensor.TestLogSumExpSameBitsForAnyWorkerCount` (2 adım, `via tensor.LogSumExp`) ve 15 ad kodu testini veriyor; `tensor.geluExactValue` artık `tensor.TestUnaryOpsIgnorePosition` (3 adım, `via tensor.unaryLoopWide`) veriyor; `interp.sumBuiltin` çıktısı değişmedi (6 `about` + 175 ad kodu testi, `indirect` yok).

## Kalanlar
- **Adım 3 (büyümeyi ölç):**
  - 18 örnek dosyada `tests-for` ve `main`'in son 10 commit'inde `changed --go-tests` sonrası ölçümleri ve en çok büyüyen 3 dosyanın analizi.
- **Bitti ölçütü ve Rapor (`UZAK7_RAPORU.md`):**
  - Ön denetim komutları, kendi kontrolüm ve teslim.

## Emin Olmadıklarım
- Henüz yok.

