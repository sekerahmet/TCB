# Uzak İş 9 Durum Dosyası

- **Dal:** `gemini/uzak-9`
- **Taban (`main`):** `c151974ecc0ff19e597d0239f5683c5ba906a778`
- **Başlama saati:** 2026-11-08 18:55:01 UTC+3 (`2026-11-08T07:55:01-08:00`)
- **Go sürümü:** `go1.27.3 linux/amd64` (`GOTOOLCHAIN=local`)
- **Bağlam sıkıştırması:** Yok

## İlerleme

- [x] `CLAUDE.md` ve `UZAK_YONERGE_9.md` okundu, `gemini/uzak-9` dalı açıldı.
- [x] Adım 1: Yeniden üretim (4 durumun taban ölçümleri ve `testdata/mod/shape` fikstürü; commit `9c48cfd`)
- [ ] Adım 2-0a: `printTestsFor` ve `printLinkedBinaries` kol kararının taşınması
- [ ] Adım 2-0b: `untested [--in DIR] [--json]` komutu ve "önce" ölçümü
- [ ] Adım 2a: Metot ifadesi (`types.MethodExpr`) zincire girer
- [ ] Adım 2b: Yapı tablosu satırı (`uses_name` kenarı)
- [ ] Adım 2c: Yerel değişkenle işlev aktarımı
- [ ] Adım 2d: `dups --changed` etiketi ve README
- [ ] Adım 3: Büyüme, "test yok" listesi (`UZAK9_TESTSIZ.txt`), 20'lik örneklem doğrulaması
- [ ] Rapor (`UZAK9_RAPORU.md`), bitti ölçütü ve kendi kontrolüm

## Adım 1: Taban Ölçümleri ve Bulgular

1. **Metot ifadesi (`types.MethodExpr`):**
   - `internal/stdlib/ml/builtin_math.go:42` satırında `registerRounding(v, codes.FnFloor, (*tensor.Tensor).Floor)` ve `:31`–`:41` satırlarında `unit(v, ...)` çağrıları var.
   - `tensor.Tensor.Floor` taban çıktısı: yalnız `tests that use it as a function value (10)`; çağrı zinciri veya ad kodu testi yok [ölçüldü: `GOTOOLCHAIN=local go run ./tools/codegraph tests-for tensor.Tensor.Floor`, 10 değer testi].
   - `tensor.Tensor.Relu` taban çıktısı: `tests about it (1)`, `tests (4)`, `tests that use it as a function value (5)`, `tests, indirect (1)`, `measurements, not tests (2)` (toplam 11 test + 2 ölçüm; ad kodu testi yok) [ölçüldü: `GOTOOLCHAIN=local go run ./tools/codegraph tests-for tensor.Tensor.Relu`].
   - Fikstür (`tools/codegraph/testdata/mod/shape/leaf.go:15`, `shape.go:113`, `shape_test.go:32`): `shape.leafBox.exprStep` için `tests-for` tabanda `no tests found` verdi [ölçüldü: `GOTOOLCHAIN=local go run ./tools/codegraph tests-for --root tools/codegraph/testdata/mod --graph /tmp/mod_graph.json shape.leafBox.exprStep`, `no tests found`].
2. **Konumlu tablo:**
   - `internal/lang/interp/type_call.go:45` satırında `{codes.TypeBytes, bytesCall}`.
   - `interp.bytesCall` taban çıktısı: `no tests traced to it` ve `weak evidence (142): linked into the binary of cmd/trlang (142 tests run it)` [ölçüldü: `GOTOOLCHAIN=local go run ./tools/codegraph tests-for interp.bytesCall`].
   - Fikstür (`tools/codegraph/testdata/mod/shape/shape.go:66`, `:69`): `shape.runIf` için `tests-for` tabanda `no tests found` verdi [ölçüldü: `GOTOOLCHAIN=local go run ./tools/codegraph tests-for --root tools/codegraph/testdata/mod --graph /tmp/mod_graph.json shape.runIf`, `no tests found`].
3. **Değişkenle aktarım:**
   - `internal/compute/tensor/quant.go:194` (`open := dequantQ8`), `:199` (`open = dequantQ6K`), `:202` (`open(...)`).
   - `tensor.dequantQ6K` taban çıktısı: `no tests traced to it` ve `weak evidence (142): linked into the binary of cmd/trlang (142 tests run it)` [ölçüldü: `GOTOOLCHAIN=local go run ./tools/codegraph tests-for tensor.dequantQ6K`].
   - Fikstür (`tools/codegraph/testdata/mod/shape/leaf.go:17`, `shape.go:115`, `shape_test.go:34`): `shape.varLeaf` için `tests-for` tabanda `no tests found` verdi [ölçüldü: `GOTOOLCHAIN=local go run ./tools/codegraph tests-for --root tools/codegraph/testdata/mod --graph /tmp/mod_graph.json shape.varLeaf`, `no tests found`].
4. **`dups --changed`'in iki yazımı:**
   - Geçici yerel `uzak9-dups-deneme` dalında `coalesce_tensor.go.txt` ve `merge_axes_gpu.go.txt` kopyaları eklendiğinde hem `dups --changed main` hem `dups --changed uzak9-dups-deneme` birebir aynı 2 adayı (`#1 codegraph/uzak9_dup_a.go:coalesceCopyA~tensor/cpu_backend.go:MergeAxes`, `#2 codegraph/uzak9_dup_a.go:coalesceCopyA~codegraph/uzak9_dup_b.go:mergeAxesCopyB~tensor/cpu_backend.go:MergeAxes`) verdi; yalnız başlık etiketi `touching main` ve `touching uzak9-dups-deneme` olarak değişti (deneme dalı silindi, push edilmedi).
5. **Taban graf kurma süresi:** 3 koşuda `9.5s`, `9.6s`, `9.5s` -> ortanca `9.5s` (`35842 nodes, 294139 edges (188697 EXTRACTED, 105442 INFERRED)`).
