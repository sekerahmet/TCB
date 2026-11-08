# Uzak İş 9 Durum Dosyası

- **Dal:** `gemini/uzak-9`
- **Taban (`main`):** `c151974ecc0ff19e597d0239f5683c5ba906a778`
- **Başlama saati:** 2026-11-08 18:55:01 UTC+3 (`2026-11-08T07:55:01-08:00`)
- **Go sürümü:** `go1.27.3 linux/amd64` (`GOTOOLCHAIN=local`)
- **Bağlam sıkıştırması:** Yok

## İlerleme

- [x] `CLAUDE.md` ve `UZAK_YONERGE_9.md` okundu, `gemini/uzak-9` dalı açıldı.
- [x] Adım 1: Yeniden üretim (4 durumun taban ölçümleri ve `testdata/mod/shape` fikstürü; commit `a5244bc`)
- [x] Adım 2-0a: `printTestsFor` ve `printLinkedBinaries` kol kararının taşınması (`testsForNode` ve `linkedBinaries`)
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

## Adım 2-0a: Kararın Taşınması

- `printTestsFor` (`tools/codegraph/query_tests.go`) ve `printLinkedBinaries` (`tools/codegraph/query_subprocess.go`) içindeki 7 kollu karar `testsForNode` ve `linkedBinaries` işlevlerine ayrıldı.
- Taşınan/değişen satırlar: `query_tests.go` `+29 -46` (dosya boyu `573` -> `556`, `printTestsFor` `71` -> `54`), `query_subprocess.go` `+90 -10` (dosya boyu `157` -> `237`, `testsForNode` `48/80`). Toplam 56 satır taşındı.
- Adım 1'in 4 hedefinde ve Adım 3'ün 40'ta bir dosya örnekleminde (26 dosya) `tests-for` çıktısı 2-0a'dan önce ve sonra bayt bayt karşılaştırıldı (`diff -u /tmp/step2_0a_before.txt /tmp/step2_0a_after.txt` farkı boş).
- Altın dosyada (`tools/codegraph/testdata/planning_questions.txt`) 7 kolun karşılıkları:
  - `testli` (`branchTested`): `:27`, `:28`, `:40`, `:49`, `:52`, `:57`, `:64`, `:67`–`:69`, `:83`, `:84`, `:89`, `:90`, `:114`, `:119`, `:126`, `:139`, `:149`, `:155`–`:157`, `:160`, `:161`, `:186`, `:190`, `:207`, `:208`, `:219`, `:228`, `:244`, `:245`
  - `alt süreç` (`branchSubprocess`): `:91` (`codegraph tests-for trlang.catchStop`)
  - `kural` (`branchRule`): `:150` (`.gitignore`), `:151` (`docs/development.md`), `:152` (`internal/README.md`), `:153` (`CLAUDE.md`)
  - `ölçüm` (`branchMeasured`): `planning_questions.txt`'te yalnız ölçümü olan hedef yok (`:52` ve `:126` hem testli hem ölçümlü); birim testi `query_tests_test.go:31` (`TestTestsForKeepsMeasurementsApartFromTests`) koşar.
  - `zayıf` (`branchWeak`): `:102` (`codegraph tests-for codeindex.LoadCached`)
  - `sınav` (`branchExam`): `planning_questions.txt`'te yok; birim testi `exam_links_test.go:14` (`TestEveryNodeTheExamNamesIsLinkedFromTheExam`) koşar.
  - `test yok` (`branchUntested`): `planning_questions.txt`'te Linux'ta yok (`:103` `dataload.mmapFile` Linux'ta testlidir); birim testi `query_test.go:75` (`TestTestsFor`, `shape.Orphan`) koşar.
