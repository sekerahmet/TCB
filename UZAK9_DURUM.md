# Uzak İş 9 Durum Dosyası

- **Dal:** `gemini/uzak-9`
- **Taban (`main`):** `c151974ecc0ff19e597d0239f5683c5ba906a778`
- **Başlama saati:** 2026-11-08 18:55:01 UTC+3 (`2026-11-08T07:55:01-08:00`)
- **Go sürümü:** `go1.27.3 linux/amd64` (`GOTOOLCHAIN=local`)
- **Bağlam sıkıştırması:** 4 kez:
  1. Adım 2-0b sırasında (`7ca1615` commit'inden sonra, 2026-11-08 19:27:48 UTC+3 / `2026-11-08T16:27:48+00:00`).
  2. Adım 2b sırasında (`5e913a9` commit'inden sonra, 2026-11-08 20:02:50 UTC+3 / `2026-11-08T17:02:50+00:00`).
  3. Adım 2c'den sonra (`92f1624` commit'inden sonra, 2026-11-08 20:26:05 UTC+3 / `2026-11-08T17:26:05+00:00`).
  4. Adım 3 sırasında (`f320be4` commit'inden sonra, 2026-11-08 21:02:37 UTC+3 / `2026-11-08T18:02:37+00:00`). Her dördünde de yönerge ve durum dosyası yeniden okundu, `git status` ve `git log` kontrol edildi.

## İlerleme

- [x] `CLAUDE.md` ve `UZAK_YONERGE_9.md` okundu, `gemini/uzak-9` dalı açıldı.
- [x] Adım 1: Yeniden üretim (4 durumun taban ölçümleri ve `testdata/mod/shape` fikstürü; commit `a5244bc`)
- [x] Adım 2-0a: `printTestsFor` ve `printLinkedBinaries` kol kararının taşınması (`testsForNode` ve `linkedBinaries`; commit `7ca1615`)
- [x] Adım 2-0b: `untested [--in DIR] [--json]` komutu ve "önce" ölçümü (commit `8b9f31d`)
- [x] Adım 2a: Metot ifadesi (`types.MethodExpr`) zincire girer (commit `5e913a9`)
- [x] Adım 2b: Yapı tablosu satırı (`uses_name` kenarı; commit `f25bc55`)
- [x] Adım 2c: Yerel değişkenle işlev aktarımı (commit `92f1624`)
- [x] Adım 2d: `dups --changed` etiketi ve README (commit `f320be4`)
- [x] Adım 3: Büyüme, "test yok" listesi (`UZAK9_TESTSIZ.txt`), 20'lik örneklem doğrulaması
- [x] Rapor (`UZAK9_RAPORU.md`), bitti ölçütü ve kendi kontrolüm

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
- Adım 1'in 4 hedefinde ve Adım 3'ün 40'ta bir dosya örnekleminde (19 dosya) `tests-for` çıktısı 2-0a'dan önce ve sonra bayt bayt karşılaştırıldı (`diff -u /tmp/step2_0a_before.txt /tmp/step2_0a_after.txt` farkı boş).
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

## Adım 2b: Yapı Tablosu Satırı (`noteRowNameImpl`)

- Üretim satırı: `tools/codegraph/gocode_scan.go` `+44 -15` (net `+29`), `tools/codegraph/graph.go` `+1 -1` (`extractorRevision = 93`) -> 60–90 bütçesinin altında/içinde.
- Depoda kurala uyan (tek ayrı ad kodu + tek işlev değeri) toplam **26 yapı satırı** var (`17` satır `internal/lang/interp/builtins.go:23`–`:39` `coreBuiltins`, `9` satır `internal/lang/interp/type_call.go:37`–`:45` `registerTypeCalls`).
- Bunların **22'sinde** işlev gövdesinden ad koduna EXTRACTED `uses_name` kenarı zaten var (`15`'i `builtins.go`, `7`'si `type_call.go`; `{codes.TypeBytes, bytesCall}` satırı da `bytesCall` gövdesinde `bytes_value.go:188` satırında `codes.TypeBytes` geçtiği için EXTRACTED `uses_name` kenarına zaten sahipti).
- **Eklenen 4 yeni INFERRED `uses_name` kenarı (`dosya:satır`):**
  1. `internal/lang/interp/builtins.go:23`: `interp.printBuiltin -> name:TR_fn_print` (`printBuiltin` için 198 ad kodu testi eklendi)
  2. `internal/lang/interp/builtins.go:32`: `interp.timeMsBuiltin -> name:TR_fn_time_ms` (`timeMsBuiltin` için 1 ad kodu testi eklendi)
  3. `internal/lang/interp/type_call.go:43`: `interp.typeCall -> name:TR_type_Type`
  4. `internal/lang/interp/type_call.go:44`: `interp.jsonCall -> name:TR_type_JSON`
  - Not: `interp.bytesCall`, `interp.typeCall` ve `interp.jsonCall` birer `ClassType` ad koduna (`TR_type_Bytes`, `TR_type_Type`, `TR_type_JSON`) bağlanır; ancak `implementable(n)` (`tools/codegraph/test_names.go:28`–`:37`) tip adlarını (`ClassType`) bilerek dışarıda bıraktığından `tests-for interp.bytesCall` yine `no tests traced to it` (`weak evidence (142)`) verir; buna karşılık `interp.printBuiltin` ve `interp.timeMsBuiltin` (`ClassFunction`) ad kodu testlerini alır.
- **Bağlanmayan yapı satırları ve nedenleri:**
  - `{codes.X, wrap(f)}` biçiminde (ad kodu + çağrı ifadesi olan) yapı satırı: **0** satır.
  - 1 ad kodu + 0 işlev değeri olan (salt üstveri/anahtar sözcük/tip tabloları): **59** satır (`28` `internal/lang/lexer/keywords.go:24`–`:63`, `29` `internal/lang/checker/builtin_args.go:33`–`:75`, `2` `internal/lang/checker/builtin_calls.go:22,210`).
  - 2+ ad kodu + 0 işlev değeri olan: **9** satır (`3` `internal/lang/checker/builtin_args.go:50,62,73`, `2` `internal/lang/checker/builtin_calls.go:21,23`, `4` `internal/lang/checker/byte_dtypes.go:23`–`:26`).
  - 2+ işlev değeri ve ad kodu olan yapı satırı: **0** satır.
- **Ad kodu olmayan işlev tabloları (kapsam dışı):**
  - Doğrudan işlev dilimi/dizisi/haritası (`[]func(...){...}`, `[N]func(...){...}`, `map[K]func(...){...}`): **4** yer (`internal/devtools/codeindex/library_codes.go:257`, `internal/devtools/codeindex/lookup.go:255`, `internal/compute/gpu/typed_ops.go:148`, `tools/codegraph/query.go:156`).
  - Ad kodu olmayan yapı tablosunda işlev alanı (`[]struct{...}{{..., fn}}`): **2** satır (`internal/compute/tokenize/pretokenizers.go:28`, `tools/codegraph/dups_exclude.go:28`).
- Koruyan test: `tools/codegraph/query_names_test.go:195` (`TestTableRowNameCodeIsImplementedByRowCallable`).

## Adım 2c: Yerel Değişkenle İşlev Aktarımı (`viaVar = "var"`)

- Yeni `Edge.Via` sabiti: `viaVar = "var"` (`viaArg = "arg"` ile aynı 3 harfli adlandırma düzeninde; yerel değişken üzerinden aktarımı belirtir). `reachEdge` (`test_reach.go:25`) `via == viaVar` kabul eder; `nameCallersOf` (`query_tests.go:213`, `!throughArgs`) `viaVar`'ı izlemez.
- Üretim satırı: `gocode_scan.go` `+71 -2`, `test_reach.go` `+4 -3`, `query_tests.go` `+1 -1`, `graph.go` `+1 -1` (`extractorRevision = 94`).
- **İşaretlenen kenarlar (`via="var"`):**
  - Test dışı dosyalarda **8 yerel değişken** (7 işlevde) üzerinden **17 `calls` kenarı**:
    1. `internal/lang/parser/enum.go:56` (`read` in `parser.Parser.parseEnumStatement`, çağrı `:64`): `:56` `parser.structBody.variant`, `:58` `parser.structBody.enumMethod`
    2. `internal/lang/parser/struct.go:37` (`read` in `parser.Parser.parseStructStatement`, çağrı `:41`): `:37` `parser.structBody.field`, `:39` `parser.structBody.method`
    3. `internal/stdlib/files/copy.go:106` (`resolve` in `files.moveJob.findSource`, çağrı `:110`): `:106` `interp.VM.ResolvePath`, `:108` `interp.VM.ResolveWritePath`
    4. `internal/stdlib/files/copy.go:106` (`explain` in `files.moveJob.findSource`, çağrı `:113`): `:106` `interp.ReadError`, `:108` `interp.VM.WriteError`
    5. `internal/compute/tensor/quant.go:194` (`open` in `tensor.dequantBlocks`, çağrı `:202`): `:194` `tensor.dequantQ8`, `:197` `tensor.dequantQ4K`, `:199` `tensor.dequantQ6K`
    6. `internal/stdlib/ml/builtin_elementwise.go:57` (`fn` in `ml.registerGelu`, çağrı `:61`): `:57` `tensor.Tensor.Gelu`, `:59` `tensor.Tensor.GeluExact`
    7. `internal/stdlib/ml/save.go:183` (`write` in `ml.tensorSave`, çağrı `:192`): `:183` `ml.writeToFile`, `:188` `ml.writeSafetensorsFile`
    8. `internal/stdlib/ml/unlocked.go:69` (`kernel` in `ml.stepOn`, çağrı `:80`): `:72` `tensor.AdamW.StepKernel`, `:74` `tensor.SGD.StepKernel`
  - Test dosyalarında **9 yerel değişken** (7 işlevde) üzerinden **12 `calls` kenarı** (`cumsum_sort_test.go:202,203`, `reduce_test.go:115,138`, `backend_multi_test.go:35,150`, `gemm_lib_test.go:24,63`, `gpu_test.go:117`).
  - Toplam: **17 yerel değişken**, **29 `calls` (`via="var"`) kenarı**.
- **Önlem listeleri (`UZAK_YONERGE_9.md:128`–`:134`):**
  - **Birden çok işlev atanan değişkenler:** test dışındaki 8 değişkenin tamamı (`enum.go:56`, `struct.go:37`, `copy.go:106` `resolve`, `copy.go:106` `explain`, `quant.go:194` `open`, `builtin_elementwise.go:57` `fn`, `save.go:183` `write`, `unlocked.go:69` `kernel`) ve testlerdeki 3 değişken (`backend_multi_test.go:35,150`, `gpu_test.go:117`).
  - **Ataması ile çağrısı farklı düğümde (biri kapanışta) olanlar:** 3 değişken (`internal/stdlib/ml/builtin_elementwise.go:57` `fn`, `internal/stdlib/ml/save.go:183` `write`, `internal/stdlib/ml/unlocked.go:69` `kernel`). Not: `714bbaf5`'te `use` (`gocode_scan.go:99,321`) kapanış içindeki `edgeCalls` kenarlarını da doğrudan kapsayan `owner` işlevine bağlar (`edgeContains -> kindClosure` `reachEdge`'de yoktur).
  - **Değersiz bildirimle (`var x func(...)`) başlayanlar:** kabul edilenlerden test dışında 1 (`internal/stdlib/ml/unlocked.go:69` `var kernel func() *tensor.Kernel`), testlerde 2 (`backend_multi_test.go:35`, `gpu_test.go:117`).
  - **Kenarda `Via` zaten dolu olduğu için ezilmeyenler:** **0** kenar.
- **Kuralın uygulanmadığı değişkenler (test dışı, çağrılan yerel değişkenler):** parametre (`191`), işlev değişmezi `func(...){...}` (`171`; bunlardan `internal/lang/interp/sorting.go:102` `same` hem `isNumber` hem 2 `func_lit` alır), çağrı sonucu / çoklu dönüş (`134`), `range` döngü değişkeni (`36`), yapı alanı (`5`), dizin/harita (`3`), başka yerel değişken (`1`), adres alma `&x` (`0`); yalnız işlev atanıp hiç çağrılmayan (`called=false`): `2` (`internal/stdlib/process/process_value.go:52` `fn`, `tools/edithook/main.go:144` `pattern`).
- **Kapsam dışı sayılar (test dışı):** paket düzeyi işlev değişkeni `25`, işlev tipli yapı alanı `45`, işlev döndüren işlev/metot `146`.
- `tensor.dequantQ6K`: `tests, indirect (29)`, `weak evidence (60)`, `measurements, not tests (9)` (tabanda `no tests traced to it`, yalnız 142 ikili bağı vardı).

## Adım 2d: `dups --changed` Etiketi ve README

- Adım 1'in 4. ölçüsünde iki yazım (`--changed main` ve `--changed <dal>`) aynı adayları verdiği için her iki yazım korundu; `changedGoLines` ve `filterChanged` (`tools/codegraph/dups_changed.go:42`, `:69`) kurulan git aralığını (`rng`, ör. `main...HEAD` veya `main...claude/new`) döndürecek şekilde güncellendi ve `runDups` (`tools/codegraph/dups_report.go:152`) `res.Changed`'e ham girdi yerine gerçek git aralığını yazdı.
- `tools/codegraph/README.md:332`–`:333` satırlarına hem `--changed main` hem `--changed claude/x` örneği eklendi.
- Üretim satırı: `dups_changed.go` `+11 -11`, `dups_report.go` `+1 -2`, `README.md` `+2 -1` -> toplam `+14 -14` (10–20 bütçesi içinde).
- Koruyan test: `tools/codegraph/dups_test.go:290` (`TestDupsChangedOnlyReportsWhatTheBranchTouched`) hem `--changed claude/new` (`res.Changed == "main...claude/new"`) hem `--changed main` (`1 duplicate candidate(s) touching main...HEAD` ve `new.go:`) çıktılarını doğrular.

## Adım 3: Büyüme, `untested` ve 20'lik Örneklem Doğrulaması

- **Graf kurma süresi (sonra):** 3 koşuda `9.5s`, `9.7s`, `9.4s` -> ortanca `9.5s` (önce ortancası `9.5s`; `35905 nodes, 294406 edges (188964 EXTRACTED, 105442 INFERRED)`).
- **Uçta izlenmeyen Go INFERRED `calls` kenarları:** `357` (önce: `118` test dışı, `239` test) -> `271` (sonra: `74` test dışı, `197` test).
  - Çağrı argümanı metot ifadeleri: `57` (`56` `SelectorExpr` + `1` `*ast.StarExpr`; `27` test dışı, `30` test) -> `0` (2a ile hepsi `via=arg` oldu).
  - Değişkene/alana atama: `46` (`28` test dışı, `18` test) -> `17` (`11` test dışı, `6` test; 2c ile `29` kenar `via=var` oldu).
  - Bileşik değer (konumlu yapı/dilim): `173` (`29` test dışı, `144` test) -> `173`.
  - Bileşik değer (anahtarlı alan/harita): `65` (`19` test dışı, `46` test) -> `65`.
  - Ad kodlu anahtarlı tablo (`via=name:...`): `13` (`13` test dışı, `0` test) -> `13`.
  - `return` deyimi: `3` (`2` test dışı, `1` test) -> `3`.
- **`untested` karşılaştırması (önce: 2-0b ucu `8b9f31d`, 6841 düğüm; sonra: `HEAD`, 6844 düğüm):**
  - `tested`: `6314` -> `6311` (2b ve 2c'de eklenen 3 yardımcıdan `noteRowNameImpl` ve `noteLocalVar` `tested`, `noteSingleCodeFunc` 9 adımda kaldığı için `untested`; `weak -> tested` `+9`, `tested -> weak` `-14`)
  - `subprocess`: `92` -> `92`
  - `rule`: `0` -> `0`
  - `measured`: `2` -> `2`
  - `weak`: `328` -> `333` (`-9` `tested`'a geçti, `+14` 2a'nın kaldırdığı 18 `unit(...)` bağından düştü)
  - `exam`: `0` -> `0`
  - `untested`: `105` -> `106` (`+1` `codegraph.goExtractor.noteSingleCodeFunc`)
- **Tam liste:** `UZAK9_TESTSIZ.txt` (`--all` ile 542 satır) depo köküne yazıldı.
- **20'lik örneklem (her 5. `untested` işlevi, `#1, #6, ..., #96`):** 11'i "test var ama araç görmüyor" (1'i standart kütüphane `go/types.Importer` arayüzü, 9'u `tools/codegraph` içinde `TestMaxSteps = 8` sınırını aşan 9–11 adımlık iç yardımcılar, 1'i `dupRules` yapı alanındaki işlev değeri), 9'u "gerçekten testsiz" (`edithook.main`, `snippets.main`, `record.defaultOut`, `tools/gen/confusables`'tan 2 işlev ve `tools/trlang_migrate`'ten 4 işlev), 0'ı "emin değilim".


