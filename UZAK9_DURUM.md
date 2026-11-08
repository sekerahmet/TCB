# Uzak İş 9 Durum Dosyası

- **Dal:** `gemini/uzak-9`
- **Taban (`main`):** `c151974ecc0ff19e597d0239f5683c5ba906a778`
- **Başlama saati:** 2026-11-08 18:55:01 UTC+3 (`2026-11-08T07:55:01-08:00`)
- **Go sürümü:** `go1.27.3 linux/amd64` (`GOTOOLCHAIN=local`)
- **Bağlam sıkıştırması:** 1 kez — Adım 2-0b sırasında (`7ca1615` commit'inden sonra, 2026-11-08 19:27:48 UTC+3 / `2026-11-08T16:27:48+00:00`). Yönerge ve durum dosyası yeniden okundu, `git status` ve `git log` kontrol edildi.

## İlerleme

- [x] `CLAUDE.md` ve `UZAK_YONERGE_9.md` okundu, `gemini/uzak-9` dalı açıldı.
- [x] Adım 1: Yeniden üretim (4 durumun taban ölçümleri ve `testdata/mod/shape` fikstürü; commit `a5244bc`)
- [x] Adım 2-0a: `printTestsFor` ve `printLinkedBinaries` kol kararının taşınması (`testsForNode` ve `linkedBinaries`; commit `7ca1615`)
- [x] Adım 2-0b: `untested [--in DIR] [--json]` komutu ve "önce" ölçümü (commit `8b9f31d`)
- [x] Adım 2a: Metot ifadesi (`types.MethodExpr`) zincire girer
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

## Adım 2-0b: `untested [--in DIR] [--json]` ve "Önce" Ölçümü

- Üretim satırı: `main.go` `+2 -1`, `main_query.go` `+11 -4`, `query_subprocess.go` `+51 -0` -> toplam `+64 -5` (net `+59`, 40–80 bütçesi içinde).
- README seçimi: `tools/codegraph/README.md:115`–`:117` satırlarında `tests-for` açıklamasının hemen yanında anlatıldı ("Audits" bölümü graf gerektirmeyen komutlar için olduğundan oraya konulmadı).
- Koruyan test: `tools/codegraph/subprocess_test.go:77` (`TestUntestedGroupsFunctionsByBranch`) — `tested`, `subprocess`, `measured`, `weak` ve `untested` kollarını hem metin hem `--json` çıktısında doğrular, kapanışları dışarıda bırakır.
- Koşma süresi (`--no-rebuild`): `1m16.6s` (`76.6s`).
- Düğüm sayıları ve `6719` ile fark:
  - Taban (`c151974e`): `git grep '^func '` (test ve `testdata` hariç) `6719`; grafın test dışı `func` (`3298`) + `method` (`3540`) düğümü `6838` (2-0a ve 2-0b'de eklenen 3 işlevle `6841`: `3298` `func` + `3543` `method`).
  - Farkın nedeni: `6719` `func` bildiriminden `61`'i `IgnoredGoFiles` (`*_windows.go`, `*_other.go`, `*_race.go`) içinde Linux sürümüyle aynı işlev/metot kimliğini paylaşır ve `other_os.go:88`'de tek düğümde birleşir (`6719 - 61 = 6658`); buna karşılık `gocode.go:354`–`:364` arayüz metot bildirimleri için `180` ayrı `kindMethod` düğümü üretir (`6658 + 180 = 6838`).
  - Test dışı kapanış (`kindClosure`) düğümü: `1549` (testler dahil toplam `3555`); `untested` listesine girmez.
  - Ölçüm klasörü (`isMeasurement`, `tools/bench/`) işlevleri: `3` (`go.fib`, `go.main`, `go.ms`); listeye girer (`untested` grubunda).
- `untested` "Önce" ölçümü (2-0b'nin ucu, 6841 düğüm; taban 6838 düğümde `tested` 6311, diğer 6 grup aynı):
  - `tested`: `6314` (tabanda `6311`)
  - `subprocess`: `92`
  - `rule`: `0`
  - `measured`: `2` (`interp.JSONVal.Operator`, `ml.TensorValue.matmulOp`)
  - `weak`: `328`
  - `exam`: `0`
  - `untested`: `105`

## Adım 2a: Metot İfadesi (`types.MethodExpr`) Zincire Girer

- Üretim satırı: `tools/codegraph/gocode_scan.go:413` (`+1 -1`, yorumla `+2 -2`), `tools/codegraph/graph.go:25` (`+1 -1`, `extractorRevision = 92`), `tools/codegraph/testdata/extractor_digest.txt` (`+2 -2`) -> üretim kodu değişimi 10 satırın altında.
- `uses_name` kenar farkı:
  - **5 yeni INFERRED `uses_name` kenarı:** beklenen 4 `registerRounding` kaydına (`internal/stdlib/ml/builtin_math.go:42`–`:45`: `tensor.Tensor.Floor -> name:TR_fn_floor`, `Ceil -> name:TR_fn_ceil`, `Sign -> name:TR_fn_sign`, `Abs -> name:TR_fn_abs`) ek olarak 5. kenar `internal/stdlib/ml/builtins.go:100` satırındaki `unit(v, codes.FnSqrt, (*tensor.Tensor).Sqrt, nil)` çağrısından geldi (`tensor.Tensor.Sqrt -> name:TR_fn_sqrt`), çünkü 4. argüman işlev değil `nil` olduğundan çağrıda tek ad kodu ve tek işlev değeri vardır.
  - **18 kalkan INFERRED `uses_name` kenarı:** `internal/stdlib/ml/builtin_math.go:31`–`:41` (11 kayıt: `silu64` ve 10 `detmath.*`) ile `internal/stdlib/ml/builtins.go:96`–`:98`, `:101`–`:104` (7 kayıt: `relu64`, `sigmoid64` ve 5 `detmath.*`) toplam 18 `unit(...)` çağrısı artık iki işlevli oldu ve kenarları kalktı (`detmath.Exp`'in ad kodu testleri `69 + 4 weak`'ten `60 + 4 weak`'e indi).
  - **65 yeni `calls` (`via=arg`) kenarı** eklendi.
  - `tensor.Tensor.Floor`: `tests about it (3)`, `tests that use it as a function value (9)`, `tests through the name codes it implements (4)` (toplam 16 test; tabanda yalnız 10 değer testiydi).
  - `tensor.Tensor.Relu`: tabanla aynı (11 test + 2 ölçüm; `unit` çağrısı iki işlevli olduğundan `uses_name` eklenmedi, `registerMath` zaten test edilmiyor).
- Koruyan testler: `tools/codegraph/query_tests_test.go:419` (`TestTestsForFollowsPackageFunctionPassedAsCallArgument`) ve `tools/codegraph/query_names_test.go:69` (`TestAdjacentNameCodeInCallIsImplementedByArgumentCallable`).


