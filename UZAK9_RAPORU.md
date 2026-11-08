# Uzak İş 9 Raporu: `codegraph`, İşlev Değeri Maddesinin Kalanı, `dups --changed` Etiketi ve "Test Yok" Listesi

- **Dal:** `gemini/uzak-9`
- **Taban (`main`):** `c151974ecc0ff19e597d0239f5683c5ba906a778`
- **Başlama saati:** 2026-11-08 18:55:01 UTC+3 (`2026-11-08T07:55:01-08:00`)
- **Bitiş saati:** 2026-11-08 21:11:00 UTC+3 (`2026-11-08T18:11:00+00:00`)
- **Bağlam sıkıştırması:** 4 kez:
  1. Adım 2-0b sırasında (`7ca1615` commit'inden sonra, 2026-11-08 19:27:48 UTC+3 / `2026-11-08T16:27:48+00:00`).
  2. Adım 2b sırasında (`5e913a9` commit'inden sonra, 2026-11-08 20:02:50 UTC+3 / `2026-11-08T17:02:50+00:00`).
  3. Adım 2c'den sonra (`92f1624` commit'inden sonra, 2026-11-08 20:26:05 UTC+3 / `2026-11-08T17:26:05+00:00`).
  4. Adım 3 sırasında (`f320be4` commit'inden sonra, 2026-11-08 21:02:37 UTC+3 / `2026-11-08T18:02:37+00:00`).
  Her sıkıştırmada yönerge, `UZAK9_DURUM.md` ve `UZAK9_RAPORU.md` yeniden okundu, `git status` ve `git log` kontrol edildi, tüm sayılar ve alıntılar kaynak dosyalardan doğrulandı.

## Özet Tablosu

| Adım | Commit | Üretim Satırı Bütçesi | Gerçek Üretim Satırı | Ana Sonuç |
| :-- | :-- | :-- | :-- | :-- |
| Adım 1 | `a5244bc` | Fikstür | `0` (`testdata/mod/shape` `+14`) | 4 durum tabanda yeniden üretildi; `shape.leafBox.exprStep` ve `shape.varLeaf` fikstürleri eklendi. |
| Adım 2-0a | `7ca1615` | Taşıma | `56` taşınan satır (`query_tests.go` `+29 -46`, `query_subprocess.go` `+90 -10`) | `printTestsFor` ve `printLinkedBinaries` içindeki 7 kollu karar `testsForNode` ve `linkedBinaries` işlevlerine ayrıldı; çıktı bayt bayt aynı kaldı. |
| Adım 2-0b | `8b9f31d` | 40–80 | `+64 -5` (net `+59`, `README.md` `+3`) | `untested [--in DIR] [--json]` komutu eklendi; taban ölçümü alındı (`6314` tested, `92` subprocess, `2` measured, `328` weak, `105` untested). |
| Adım 2a | `5e913a9` | < 10 | `+2 -2` (`gocode_scan.go` `+1 -1`, `graph.go` `+1 -1`) | Somut alıcılı `types.MethodExpr` `argCallable`'a girdi; `65` yeni `via="arg"` kenarı, `5` yeni `uses_name` kenarı eklendi, `18` iki işlevli `unit(...)` kenarı kalktı. |
| Adım 2b | `f25bc55` | 60–90 | `+45 -16` (`gocode_scan.go` `+44 -15`, `graph.go` `+1 -1`) | Yapı tablosu satırlarında (`noteRowNameImpl`) tek ayrı ad kodu ile tek işlev değeri eşleşmesi eklendi; `4` yeni INFERRED `uses_name` kenarı eklendi (`printBuiltin`, `timeMsBuiltin`, `typeCall`, `jsonCall`). |
| Adım 2c | `92f1624` | 30–50 | `+77 -7` (`gocode_scan.go` `+71 -2`, `test_reach.go` `+4 -3`, `query_tests.go` `+1 -1`, `graph.go` `+1 -1`) | Yalnız işlev değeri atanıp çağrılan yerel değişkenler (`viaVar = "var"`) zincire girdi (`17` değişken, `29` kenar); `tensor.dequantQ6K` `0` izlenen testten `29` dolaylı + `60` zayıf teste çıktı. |
| Adım 2d | `f320be4` | 10–20 | `+14 -14` (`dups_changed.go` `+11 -11`, `dups_report.go` `+1 -2`, `README.md` `+2 -1`) | `dups --changed` başlığı ve JSON `changed` alanı ham girdi yerine gerçek git aralığını (`touching main...HEAD`) basar; `README.md` her iki yazımı gösterir. |

---

## Bulgular (Önem Sırasıyla)

### Bulgu 1: Yerel Değişkenle İşlev Aktarımı (`viaVar = "var"`, Adım 2c)

- **İddia:** Yalnız işlev değeri (paket işlevi, somut metot değeri ya da somut metot ifadesi) atanıp çağrılan yerel değişkenler (`*types.Var`) üzerinden giden `calls` kenarlarının `viaVar = "var"` ile işaretlenmesi ve `reachEdge` tarafından kabul edilmesi, test dışında `8` yerel değişkende `17` kenarı ve test dosyalarında `9` yerel değişkende `12` kenarı (toplam `17` değişken, `29` kenar) zincire sokarak `tensor.dequantQ8`, `tensor.dequantQ4K`, `tensor.dequantQ6K`, `ml.writeToFile`, `ml.writeSafetensorsFile`, `parser.structBody.variant` ve `parser.Parser.variantFields` işlevlerini "no tests traced to it" (`weak`) durumundan `tested` grubuna taşır.
- **Kanıt:**
  - `tools/codegraph/test_reach.go:25`: `[kanıtlı: tools/codegraph/test_reach.go:25, "return confidence == extracted || via == viaArg || via == viaVar || strings.HasPrefix(from, \"tr:\") && strings.HasPrefix(to, \"tr:\")"]`
  - `tools/codegraph/query_tests.go:213`: `[kanıtlı: tools/codegraph/query_tests.go:213, "if !throughArgs && (e.Via == viaArg || e.Via == viaVar) && e.Confidence != extracted {"]`
  - `tensor.dequantQ6K` için `tests-for` tabanda `no tests traced to it` (`weak evidence (142)`) verirken Adım 2c sonrasında `29` dolaylı test (`via tensor.dequantBlocks`), `60` zayıf kanıt, `9` ölçüm ve `2` kural verir `[ölçüldü: GOTOOLCHAIN=local go run ./tools/codegraph tests-for --no-rebuild tensor.dequantQ6K, 29 indirect + 60 weak + 9 measurements]`.
  - `[test: tools/codegraph/query_tests_test.go]` (`TestTestsForFollowsPackageFunctionPassedAsCallArgument`) ve `[test: tools/codegraph/test_names_test.go]` (`TestNameCallersDoesNotCrossArgumentFunction`).
- **Öneri:** Akışa duyarlı yerel değişken analizi (atama sırası ve dallanma takibi) kapsam dışıdır; aşağıdaki önlem listesinde yer alan değişkenler denetim için tek tek belgelenmiştir.

### Bulgu 2: Somut Alıcılı Metot İfadelerinin (`types.MethodExpr`) Zincire Girmesi (Adım 2a)

- **İddia:** `argCallable` işlevinin `types.MethodVal` yanında `types.MethodExpr` seçimlerini de kabul etmesi, çağrı argümanı olan somut alıcılı metot ifadelerini (`57` mevcut kenar + `8` yeni fikstür kenarı = `65` `via="arg"` kenarı) test erişim zincirine sokar; tek ayrı ad kodu ve tek işlev değeri kuralı gereği `5` yeni INFERRED `uses_name` kenarı ekler ve iki işlevli `18` `unit(...)` çağrısındaki fazla `uses_name` bağlarını kaldırır.
- **Kanıt:**
  - `tools/codegraph/gocode_scan.go:464`: `[kanıtlı: tools/codegraph/gocode_scan.go:464, "return s != nil && (s.Kind() == types.MethodVal || s.Kind() == types.MethodExpr) && !types.IsInterface(s.Recv()) && !types.IsInterface(sig.Recv().Type())"]`
  - **Eklenen 5 INFERRED `uses_name` kenarı:** `internal/stdlib/ml/builtin_math.go:42`–`:45` satırlarındaki 4 `registerRounding` çağrısına (`tensor.Tensor.Floor -> name:TR_fn_floor`, `tensor.Tensor.Ceil -> name:TR_fn_ceil`, `tensor.Tensor.Sign -> name:TR_fn_sign`, `tensor.Tensor.Abs -> name:TR_fn_abs`) ek olarak 5. kenar `internal/stdlib/ml/builtins.go:100` satırındaki `[kanıtlı: internal/stdlib/ml/builtins.go:100, "unit(v, codes.FnSqrt, (*tensor.Tensor).Sqrt, nil) // karekök"]` çağrısından (`tensor.Tensor.Sqrt -> name:TR_fn_sqrt`) geldi; 4. argüman `nil` olduğundan çağrıda tek ad kodu ve tek işlev değeri vardır.
  - **Kalkan 18 INFERRED `uses_name` kenarı:** `internal/stdlib/ml/builtin_math.go:31`–`:41` (11 kayıt: `ml.silu64` ve 10 `detmath.*` işlevi) ile `internal/stdlib/ml/builtins.go:96`–`:98`, `:101`–`:104` (7 kayıt: `ml.relu64`, `ml.sigmoid64` ve 5 `detmath.*` işlevi) artık iki işlevli (`(*tensor.Tensor).X` ve skaler işlev) sayılarak elendi; `query TR_fn_exp` çıktısında `detmath.Exp` üzerindeki fazla `uses_name` kenarı kalktı ve `registered or used in Go` satırında her iki uygulama (`detmath.Exp`, `tensor.Tensor.Exp`) birlikte görünmeye devam etti `[ölçüldü: GOTOOLCHAIN=local go run ./tools/codegraph query --no-rebuild TR_fn_exp, uses_name (19) ve ml.registerMath -> detmath.Exp, tensor.Tensor.Exp]`.
  - `[test: tools/codegraph/query_names_test.go]` (`TestAdjacentNameCodeInCallIsImplementedByArgumentCallable`).
- **Öneri:** Değişiklik beklendiği gibi çalışmaktadır; ek işlem gerekmez.

### Bulgu 3: Yapı Tablosu Satırlarında Ad Kodu ve İşlev Eşleşmesi (`noteRowNameImpl`, Adım 2b)

- **İddia:** Bir bileşik değerin kendisi de bileşik değer olan yapı satırlarında (`[]struct{...}{{...}}`, `map[K]struct{...}{k: {...}}`, `&T{...}`) tek ayrı ad kodu ve doğrudan (derinlik 1) tek işlev değeri bulunduğunda işlevden ad koduna INFERRED `uses_name` kenarı eklenmesi, depodaki `26` uyumlu yapı satırından gövdeden zaten EXTRACTED bağı olmayan `4` satıra yeni INFERRED `uses_name` kenarı kazandırır ve `interp.printBuiltin` (`+198` test) ile `interp.timeMsBuiltin` (`+1` test) işlevlerini `weak` grubundan `tested` grubuna taşır.
- **Kanıt:**
  - Depoda tek ayrı ad kodu ve tek işlev değeri içeren `26` yapı satırı vardır (`17` satır `internal/lang/interp/builtins.go:23`–`:39` `coreBuiltins`, `9` satır `internal/lang/interp/type_call.go:37`–`:45` `registerTypeCalls`). Bunların `22`'sinde işlev gövdesinde ad kodu sabiti doğrudan kullanıldığı için EXTRACTED `uses_name` kenarı zaten mevcuttur (`{codes.TypeBytes, bytesCall}` satırı da `bytesCall` gövdesindeki `internal/lang/interp/bytes_value.go:188` satırından EXTRACTED `uses_name` kenarına zaten sahipti).
  - **Eklenen 4 yeni INFERRED `uses_name` kenarı:**
    1. `internal/lang/interp/builtins.go:23`: `[kanıtlı: internal/lang/interp/builtins.go:23, "{codes.FnPrint, printBuiltin},"]` (`interp.printBuiltin -> name:TR_fn_print`)
    2. `internal/lang/interp/builtins.go:32`: `[kanıtlı: internal/lang/interp/builtins.go:32, "{codes.FnTimeMs, timeMsBuiltin},"]` (`interp.timeMsBuiltin -> name:TR_fn_time_ms`)
    3. `internal/lang/interp/type_call.go:43`: `[kanıtlı: internal/lang/interp/type_call.go:43, "{codes.TypeType, typeCall},"]` (`interp.typeCall -> name:TR_type_Type`)
    4. `internal/lang/interp/type_call.go:44`: `[kanıtlı: internal/lang/interp/type_call.go:44, "{codes.TypeJSON, jsonCall},"]` (`interp.jsonCall -> name:TR_type_JSON`)
  - `interp.bytesCall`, `interp.typeCall` ve `interp.jsonCall` birer `ClassType` ad koduna (`TR_type_Bytes`, `TR_type_Type`, `TR_type_JSON`) bağlanır; ancak `implementable(n)` `[kanıtlı: tools/codegraph/test_names.go:33, "case codes.ClassFunction, codes.ClassConstant, codes.ClassMember, codes.ClassNamedArg:"]` tip adlarını (`ClassType`) bilerek dışarıda bıraktığı için `tests-for interp.bytesCall` yine `no tests traced to it` (`weak evidence (142)`) verir `[ölçüldü: GOTOOLCHAIN=local go run ./tools/codegraph tests-for --no-rebuild interp.bytesCall]`.
  - `[test: tools/codegraph/query_names_test.go]` (`TestTableRowNameCodeIsImplementedByRowCallable`).
- **Öneri:** `ClassType` kodlu kurucu çağrıların (`TR_Bytes(...)`, `TR_JSON(...)`, `TR_Type(...)`) tip bildirimi kullanımlarından ayrıştırılarak `implementable` kapsamına alınması ayrı bir tasarım konusu olarak değerlendirilebilir `[görüş]`.

### Bulgu 4: `untested` Komutu, "Test Yok" Listesi ve 20'lik Örneklemin İkinci Yoldan Doğrulaması (Adım 2-0a, 2-0b ve Adım 3)

- **İddia:** `testsForNode` karar işlevini kullanan `untested [--in DIR] [--json]` komutu, dalın ucundaki `6844` test dışı Go işlev ve metodunu `6311 tested, 92 subprocess, 0 rule, 2 measured, 333 weak, 0 exam, 106 untested` olarak gruplar; `untested` grubundaki `106` işlevin hiçbirisi `cmd/trlang` ikilisine bağlı olmayan paketlerdedir (`tools/*` ve `internal/devtools/goload`), ve `106` işlevden seçilen 20'lik örneklemin (`#1, #6, ..., #96`) ikinci yoldan (`git grep` ve kaynak okuma) doğrulamasında **11 işlevin** aslında testler tarafından çalıştırıldığı ama aracın sınırları (`go/types.Importer` standart kütüphane arayüzü, `TestMaxSteps = 8` adım sınırı, ad kodu olmayan `dupRules` yapı alanı) nedeniyle görünmediği, **9 işlevin** ise gerçekten testsiz olduğu saptanmıştır.
- **Kanıt:**
  - `[ölçüldü: GOTOOLCHAIN=local go run ./tools/codegraph untested --no-rebuild --all > UZAK9_TESTSIZ.txt, 6844 non-test Go functions and methods: 6311 tested, 92 subprocess, 0 rule, 2 measured, 333 weak, 0 exam, 106 untested]`
  - `[test: tools/codegraph/subprocess_test.go]` (`TestUntestedGroupsFunctionsByBranch`).
- **Öneri:** `UZAK9_TESTSIZ.txt` içindeki gerçekten testsiz araç paketleri (`tools/gen/confusables`, `tools/trlang_migrate` vb.) ile `tools/codegraph` içindeki 9–11 adımlık iç yardımcıların sınıflandırması aşağıdaki Adım 3 tablolarında sunulmuştur.

### Bulgu 5: `dups --changed` Etiketinin Gerçek Git Aralığını Basması (Adım 2d)

- **İddia:** `dups --changed main` ve `dups --changed <dal>` yazımlarının her ikisi de `changedRange` (`tools/codegraph/dups_changed.go:23`–`:38`) içinde birleşme tabanından (`git diff A...B`) aynı satır aralıklarını ve aynı kopya adaylarını üretir; yanıltıcı olan yalnızca `res.Changed = o.changed` nedeniyle başlıkta ham kullanıcı girdisinin (`touching main`) basılmasıydı. `changedGoLines` ve `filterChanged` kurulan git aralığını (`rng`) döndürecek şekilde güncellendiğinde her iki yazım da gerçek git aralığını (`touching main...HEAD` veya `touching main...<dal>`) basar.
- **Kanıt:**
  - `tools/codegraph/dups_report.go:152`: `[kanıtlı: tools/codegraph/dups_report.go:152, "if clusters, res.Changed, cerr = filterChanged(clusters, root, o); cerr != nil {"]`
  - `[ölçüldü: GOTOOLCHAIN=local go run ./tools/codegraph dups --changed main, 0 duplicate candidate(s) touching main...HEAD (0 shown; >= 100 nodes; 1334 files, 1153884 nodes, 1.5s)]`
  - `[test: tools/codegraph/dups_test.go]` (`TestDupsChangedOnlyReportsWhatTheBranchTouched`).
- **Öneri:** Ek işlem gerekmez.

---

## Adım 1: "Önce" ve "Sonra" Çıktıları

| Hedef | Tür | Taban ("Önce", `c151974e`) `tests-for` Çıktısı | Dalın Ucu ("Sonra", `HEAD`) `tests-for` / `query` Çıktısı |
| :-- | :-- | :-- | :-- |
| `tensor.Tensor.Floor` | Gerçek (2a, `types.MethodExpr`) | `tests that use it as a function value (10)` (toplam `10` test) | `tests about it (3)`, `tests that use it as a function value (9)`, `tests through the name codes it implements (4)` (toplam `16` test) |
| `tensor.Tensor.Relu` | Gerçek (2a, iki işlevli `unit`) | `tests about it (1)`, `tests (4)`, `tests that use it as a function value (5)`, `tests, indirect (1)`, `measurements, not tests (2)` (`11` test + `2` ölçüm) | `tests about it (1)`, `tests (4)`, `tests that use it as a function value (5)`, `tests, indirect (1)`, `measurements, not tests (2)` (`11` test + `2` ölçüm; `unit` iki işlevli olduğundan yanlış ad kodu bağı eklenmedi) |
| `interp.bytesCall` | Gerçek (2b, konumlu tablo) | `no tests traced to it`, `weak evidence (142): linked into the binary of cmd/trlang` | `no tests traced to it`, `weak evidence (142)` (`TR_type_Bytes` `ClassType` olduğundan `implementable` dışındadır; aynı kuraldan `interp.printBuiltin` `0 -> 198` ad kodu testine, `interp.timeMsBuiltin` `0 -> 1` ad kodu testine çıktı) |
| `tensor.dequantQ6K` | Gerçek (2c, yerel değişken) | `no tests traced to it`, `weak evidence (142): linked into the binary of cmd/trlang` | `tests, indirect (29, ... via tensor.dequantBlocks)`, `weak evidence (60)`, `measurements, not tests (9)`, `rules guarded by these tests (2)` |
| `shape.leafBox.exprStep` | Fikstür (2a, `tools/codegraph/testdata/mod/shape/leaf.go:15`) | `no tests found` | `tests, indirect (1): shape.TestUseExpr ... 2 steps via shape.useExpr [INFERRED]` |
| `shape.runIf` | Fikstür (2b, `tools/codegraph/testdata/mod/shape/shape.go:69`) | `no tests found` (`query`: `uses_name` yok) | `query shape.runIf`: `-> uses_name (1): TR_kw_if [name] internal/lang/codes/tr.json:2 [INFERRED]` (`TR_kw_if` `ClassKeyword` olduğundan `tests-for` `no tests found` verir; `ClassFunction` karşılığı `tools/codegraph/query_names_test.go:195`'te doğrulanır) |
| `shape.varLeaf` | Fikstür (2c, `tools/codegraph/testdata/mod/shape/leaf.go:17`) | `no tests found` | `tests, indirect (1): shape.TestUseVar ... 2 steps via shape.useVar [INFERRED]` |
| `dups --changed` | Gerçek (2d, yerel deneme dalı) | `--changed main` ve `--changed uzak9-dups-deneme` aynı 2 adayı (`#1 codegraph/uzak9_dup_a.go:coalesceCopyA~tensor/cpu_backend.go:MergeAxes`, `#2 ...`) verdi; etiket `touching main` ve `touching uzak9-dups-deneme` idi | Etiket ve JSON `changed` alanı kurulan gerçek git aralığını (`touching main...HEAD` ve `touching main...claude/new`) yazar |

---

## Adım 2-0a, 2-0b ve 2a–2d Seçimleri, Gerekçeleri ve Satır Bütçeleri

1. **Adım 2-0a (`7ca1615` — Kararın taşınması):**
   - `printTestsFor` (`tools/codegraph/query_tests.go:240`) ve `printLinkedBinaries` (`tools/codegraph/query_subprocess.go:199`) içindeki 7 kollu karar `testsForNode` (`tools/codegraph/query_subprocess.go:128`) ve `linkedBinaries` (`tools/codegraph/query_subprocess.go:176`) işlevlerine taşındı (`56` satır taşındı; `query_tests.go` `+29 -46`, `query_subprocess.go` `+90 -10`).
   - 7 kolun dalın ucundaki (`HEAD`) satırları (`tools/codegraph/query_subprocess.go`):
     - `branchTested` (`"tested"`): `:166` (`found > 0`)
     - `branchSubprocess` (`"subprocess"`): `:168` (`len(res.subTests) > 0`)
     - `branchRule` (`"rule"`): `:170` (`len(res.ruleTests)+len(res.cites)+len(res.scans)+len(res.typeChecks) > 0`)
     - `branchMeasured` (`"measured"`): `:172` (`len(res.measuredDirect)+len(res.measuredIndirect) > 0`)
     - `branchWeak` (`"weak"`): `:156` (`res.linkedTotal > 0`)
     - `branchExam` (`"exam"`): `:159` (`res.exam != nil`)
     - `branchUntested` (`"untested"`): `:161` (hiçbiri)
   - Altın dosyada (`tools/codegraph/testdata/planning_questions.txt`) 7 kolu koşan satırlar:
     - `tested`: `:27`, `:28`, `:40`, `:49`, `:52`, `:57`, `:64`, `:67`–`:69`, `:83`, `:84`, `:89`, `:90`, `:114`, `:119`, `:126`, `:139`, `:149`, `:155`–`:157`, `:160`, `:161`, `:186`, `:190`, `:207`, `:208`, `:219`, `:228`, `:244`, `:245`
     - `subprocess`: `:91` (`codegraph tests-for trlang.catchStop`)
     - `rule`: `:150` (`.gitignore`), `:151` (`docs/development.md`), `:152` (`internal/README.md`), `:153` (`CLAUDE.md`)
     - `measured`: `planning_questions.txt`'te yalnız ölçümü olan hedef yok (`:52` ve `:126` hem testli hem ölçümlü); birim testi `tools/codegraph/query_tests_test.go:31` (`TestTestsForKeepsMeasurementsApartFromTests`) koşar.
     - `weak`: `:102` (`codegraph tests-for codeindex.LoadCached`)
     - `exam`: `planning_questions.txt`'te yok; birim testi `tools/codegraph/exam_links_test.go:14` (`TestEveryNodeTheExamNamesIsLinkedFromTheExam`) koşar.
     - `untested`: `planning_questions.txt`'te Linux'ta yok (`:103` `dataload.mmapFile` Linux'ta testlidir); birim testi `tools/codegraph/query_test.go:75` (`TestTestsFor`, `shape.Orphan`) koşar.
   - Adım 1'in hedeflerinde ve Adım 3'ün 40'ta bir dosya örnekleminde (19 dosya) 2-0a öncesi ve sonrası `tests-for` çıktıları karşılaştırıldı; fark boş çıktı.

2. **Adım 2-0b (`8b9f31d` — `untested [--in DIR] [--json]` komutu):**
   - Üretim satırı: `main.go` `+2 -1`, `main_query.go` `+11 -4`, `query_subprocess.go` `+51 -0` -> toplam `+64 -5` (bütçe: 40–80).
   - `tools/codegraph/main_query.go:128` satırında `ix.withTestNotes(o.root)` çağrılarak `tests-for` ile birebir aynı ön hazırlık korundu.
   - README seçimi: "Audits" bölümü graf gerektirmeyen komutları anlattığı için (`tools/codegraph/README.md:419`–`:423`), `untested` komutu `tools/codegraph/README.md:115`–`:117` satırlarında `tests-for` açıklamasının hemen yanında anlatıldı.
   - Koşma süresi (`--no-rebuild`): `76.6s` (`1m16.6s`).
   - Düğüm kümesi ve `6719` ile karşılaştırma:
     - Test ve `testdata` dışındaki `git grep -c '^func '` sayısı `6719`'dur; taban grafta test dışı `func` (`3298`) + `method` (`3540`) düğümü toplamı `6838`'dir (2-0a ve 2-0b'de eklenen 3 işlevle `6841`, 2b ve 2c'de eklenen 3 yöntemle dalın ucunda `6844`: `3298` `func` + `3546` `method`).
     - `6719` ile `6838` arasındaki `+119` farkın nedeni: `6719` `func` bildiriminden `61`'i `IgnoredGoFiles` (`*_windows.go`, `*_other.go`, `*_race.go`) içinde Linux sürümüyle aynı işlev/metot kimliğini paylaşır ve `tools/codegraph/other_os.go:88`'de tek düğümde birleşir (`6719 - 61 = 6658`); buna karşılık `tools/codegraph/gocode.go:354`–`:364` arayüz metot bildirimleri için `180` ayrı `kindMethod` düğümü üretir (`6658 + 180 = 6838`).
     - Test dışı kapanış (`kindClosure`) düğümü sayısı: tabanda `1549`, dalın ucunda `1550` (testler dahil toplam `3556`); `untested` listesine girmez.
     - Ölçüm klasörü (`isMeasurement`, `tools/bench/`) işlevleri: `3` (`go.fib`, `go.main`, `go.ms` — `tools/bench/go/compare.go:19, 23, 30`); `untested` listesine girer (`untested` grubundadır).

3. **Adım 2a (`5e913a9` — Metot ifadesi zincire girer):**
   - Üretim satırı: `tools/codegraph/gocode_scan.go` `+1 -1` (yorumla `+2 -2`), `tools/codegraph/graph.go` `+1 -1` (`extractorRevision = 92`) -> bütçe (< 10) içinde.
   - `argCallable` içinde `s.Kind() == types.MethodVal || s.Kind() == types.MethodExpr` kabul edildi; arayüz alıcılı metot ifadeleri (`!types.IsInterface(s.Recv()) && !types.IsInterface(sig.Recv().Type())`) dışarıda bırakıldı.

4. **Adım 2b (`f25bc55` — Yapı tablosu satırı):**
   - Üretim satırı: `tools/codegraph/gocode_scan.go` `+44 -15` (net `+29`), `tools/codegraph/graph.go` `+1 -1` (`extractorRevision = 93`) -> bütçe (60–90) içinde/altında.
   - `noteArgNameImpl` ve `noteRowNameImpl` ortak `noteSingleCodeFunc(fc, exprs)` yardımcısını kullanacak şekilde birleştirildi (`dups --changed main` 0 aday). Haritanın anahtar-değer çifti (`codes.X: f`) `keyedImplementations`'ta kaldığından `noteRowNameImpl` yalnız değer kısmı bileşik yapı satırı (`*ast.CompositeLit` veya `&T{...}`) olan öğelere bakar; aynı çifte çift kenar düşmez.

5. **Adım 2c (`92f1624` — Yerel değişkenle aktarım):**
   - Üretim satırı: `tools/codegraph/gocode_scan.go` `+71 -2`, `tools/codegraph/test_reach.go` `+4 -3`, `tools/codegraph/query_tests.go` `+1 -1`, `tools/codegraph/graph.go` `+1 -1` (`extractorRevision = 94`) -> toplam `+77 -7`. (`noteLocalVar` içinde `:=`, `var x = f`, değersiz `var x func(...)`, çoklu atama `a, b = f, g`, `&x` adres alma ve kapanış içi atama/çağrı durumlarının tamamını tek geçişte ele almak için `gocode_scan.go`'da 71 satır gerekti; tüm fonksiyon ve dosya boyu sınırları korundu: `noteLocalVar` `66/80`, `gocode_scan.go` `555/600`.)
   - Yeni `Edge.Via` değeri olarak `viaVar = "var"` seçildi: `viaArg = "arg"` ile aynı 3 harfli adlandırma düzenindedir ve yerel değişken üzerinden aktarımı açıkça ifade eder. Kenarda `e.Via == ""` koşulu kontrol edilerek dolu `Via` alanı (`viaArg` veya `name:...`) aldrig ezilmez.

6. **Adım 2d (`f320be4` — `dups --changed` etiketi ve README):**
   - Üretim satırı: `tools/codegraph/dups_changed.go` `+11 -11`, `tools/codegraph/dups_report.go` `+1 -2`, `tools/codegraph/README.md` `+2 -1` -> toplam `+14 -14` (bütçe: 10–20).

---

## Adım 3: Büyüme, Kenar Dökümleri, `untested` Özeti ve 20'lik Örneklem

### 1. Adım 2b ve 2c Ayrıntılı Dökümleri

#### Adım 2b — Bağlanmayan Yapı Satırları ve Kapsam Dışı İşlev Tabloları
- `{codes.X, wrap(f)}` biçiminde (ad kodu + çağrı ifadesi olan) yapı satırı: **0** satır.
- 1 ad kodu + 0 işlev değeri olan (üstveri / anahtar sözcük / tip tabloları): **59** satır (`28` `internal/lang/lexer/keywords.go:24`–`:63`, `29` `internal/lang/checker/builtin_args.go:33`–`:75`, `2` `internal/lang/checker/builtin_calls.go:22, 210`).
- 2+ ad kodu + 0 işlev değeri olan yapı satırı: **9** satır (`3` `internal/lang/checker/builtin_args.go:50, 62, 73`, `2` `internal/lang/checker/builtin_calls.go:21, 23`, `4` `internal/lang/checker/byte_dtypes.go:23`–`:26`).
- 2+ işlev değeri ve ad kodu olan yapı satırı: **0** satır.
- **Kapsam dışı (ad kodu olmayan) işlev tabloları:**
  - Doğrudan işlev dilimi/dizisi/haritası (`[]func(...){...}`, `[N]func(...){...}`, `map[K]func(...){...}`): **4** yer (`internal/devtools/codeindex/library_codes.go:257`, `internal/devtools/codeindex/lookup.go:255`, `internal/compute/gpu/typed_ops.go:148`, `tools/codegraph/query.go:156`).
  - Ad kodu olmayan yapı tablosunda işlev alanı (`[]struct{...}{{..., fn}}`): **2** satır (`internal/compute/tokenize/pretokenizers.go:28`, `tools/codegraph/dups_exclude.go:33`).

#### Adım 2c — İşaretlenen 17 Yerel Değişken (`29` `via="var"` Kenarı) ve Önlem Listesi

| # | Değişken ve Konumu (`dosya:satır`) | Kapsayan İşlev | Atanan İşlevler (`dosya:satır`) | Çağrı Satırı | Önlem Bayrakları |
| :-- | :-- | :-- | :-- | :-- | :-- |
| 1 | `read` (`internal/lang/parser/enum.go:56`) | `parser.Parser.parseEnumStatement` | `parser.structBody.variant` (`:56`), `parser.structBody.enumMethod` (`:58`) | `:64` | Çoklu işlev (2) |
| 2 | `read` (`internal/lang/parser/struct.go:37`) | `parser.Parser.parseStructStatement` | `parser.structBody.field` (`:37`), `parser.structBody.method` (`:39`) | `:41` | Çoklu işlev (2) |
| 3 | `resolve` (`internal/stdlib/files/copy.go:106`) | `files.moveJob.findSource` | `interp.VM.ResolvePath` (`:106`), `interp.VM.ResolveWritePath` (`:108`) | `:110` | Çoklu işlev (2) |
| 4 | `explain` (`internal/stdlib/files/copy.go:106`) | `files.moveJob.findSource` | `interp.ReadError` (`:106`), `interp.VM.WriteError` (`:108`) | `:113` | Çoklu işlev (2) |
| 5 | `open` (`internal/compute/tensor/quant.go:194`) | `tensor.dequantBlocks` | `tensor.dequantQ8` (`:194`), `tensor.dequantQ4K` (`:197`), `tensor.dequantQ6K` (`:199`) | `:202` | Çoklu işlev (3) |
| 6 | `fn` (`internal/stdlib/ml/builtin_elementwise.go:57`) | `ml.registerGelu` | `tensor.Tensor.Gelu` (`:57`), `tensor.Tensor.GeluExact` (`:59`) | `:61` (iç kapanışta) | Çoklu işlev (2), çağrı iç kapanışta |
| 7 | `write` (`internal/stdlib/ml/save.go:183`) | `ml.tensorSave` | `ml.writeToFile` (`:183`), `ml.writeSafetensorsFile` (`:188`) | `:192` (iç kapanışta) | Çoklu işlev (2), çağrı iç kapanışta |
| 8 | `kernel` (`internal/stdlib/ml/unlocked.go:69`) | `ml.stepOn` | `tensor.AdamW.StepKernel` (`:72`), `tensor.SGD.StepKernel` (`:74`) | `:80` (iç kapanışta) | Çoklu işlev (2), çağrı iç kapanışta, değersiz `var` bildirimi (`:69`) |
| 9–17 | Test dosyalarındaki 9 yerel değişken (`internal/compute/tensor/cumsum_sort_test.go:202, 203`, `internal/compute/tensor/reduce_test.go:115, 138`, `internal/compute/gpu/backend_multi_test.go:35, 150`, `internal/compute/gpu/gemm_lib_test.go:24, 63`, `internal/compute/gpu/gpu_test.go:117`) | 7 test işlevi | `12` işlev/metot (`internal/compute/gpu/backend_multi_test.go:35, 150` ve `internal/compute/gpu/gpu_test.go:117`'de ikişer işlev; diğer 6'sında birer işlev) | İlgili test satırları | `internal/compute/gpu/backend_multi_test.go:35, 150` ve `internal/compute/gpu/gpu_test.go:117` çoklu işlev (2); `internal/compute/gpu/backend_multi_test.go:35` ve `internal/compute/gpu/gpu_test.go:117` değersiz `var` bildirimi |

- **Kenarda `Via` zaten dolu olduğu için ezilmeyenler:** **0** kenar.
- **Kuralın uygulanmadığı yerel değişkenler (test dışı dosyalarda çağrılan yerel değişkenler, nedenine göre):**
  - İşlev/kapanış parametresi: **191**
  - İşlev değişmezi (`func(...){...}`) atananlar: **171** (bunlardan `internal/lang/interp/sorting.go:102` satırındaki `same` değişkenine `:105`'te `isNumber` paket işlevi, `:107` ve `:109`'da ise iki `func(x Value) bool` değişmezi atandığı için kural bilerek uygulanmadı)
  - Çağrı sonucu / çoklu dönüş (`x := f()`): **134**
  - `range` döngü değişkeni: **36**
  - Yapı alanı (`x := s.f`): **5**
  - Dizin/harita erişimi (`x := m[k]`): **3**
  - Başka yerel değişken/parametre atananlar: **1**
  - Adresi alınanlar (`&x`): **0**
  - Yalnız işlev değeri atanıp hiç çağrılmayanlar (`called == false`): **2** (`internal/stdlib/process/process_value.go:52` `fn`, `tools/edithook/main.go:144` `pattern`).
- **Kapsam dışı sayılar (test dışı dosyalarda):** paket düzeyi işlev değişkeni **25**, işlev tipli yapı alanı **45**, işlev döndüren işlev/metot **146**.

### 2. 40'ta Bir Dosya Örneklemi (19 Dosya) ve En Çok Büyüyen Üç Dosya

`git ls-files 'internal/*.go' 'cmd/*.go' 'tools/*.go' | grep -v _test.go | sort | awk 'NR%40==1'` komutuyla seçilen 19 dosyada `tests-for` test sayıları (önce: `8b9f31d`, sonra: `HEAD`):

| Dosya | Güçlü Test (`<= 4` adım) Önce → Sonra | Zayıf Kanıt (`5–6` adım / ikili) Önce → Sonra | Toplam (`Güçlü + Zayıf`) Önce → Sonra | Fark |
| :-- | :-- | :-- | :-- | :-- |
| `cmd/trlang/arguments.go` | `0 → 0` (`subprocess: 142`) | `0 → 0` | `0 → 0` (`subprocess: 142`) | `+0` |
| `internal/compute/gpu/backend_multi.go` | `7 → 7` | `11 → 11` | `18 → 18` | `+0` |
| `internal/compute/gpu/profile.go` | `99 → 99` | `233 → 292` | `332 → 391` | **`+59`** |
| `internal/compute/tensor/cpu_gemv16.go` | `2 → 2` | `307 → 307` | `309 → 309` | `+0` |
| `internal/compute/tensor/host_block_windows.go` | `17 → 17` | `569 → 569` | `586 → 586` | `+0` |
| `internal/compute/tokenize/model_other.go` | `41 → 41` | `32 → 32` | `73 → 73` | `+0` |
| `internal/devtools/codeindex/library_codes.go` | `78 → 78` | `173 → 174` | `251 → 252` | **`+1`** |
| `internal/devtools/profiler/live_other.go` | `1 → 1` | `0 → 0` | `1 → 1` | `+0` |
| `internal/lang/checker/module_rules.go` | `212 → 212` | `339 → 339` | `551 → 551` | `+0` |
| `internal/lang/interp/bytes_value.go` | `1437 → 1437` | `134 → 134` | `1571 → 1571` | `+0` |
| `internal/lang/interp/path_arg.go` | `171 → 171` | `515 → 515` | `686 → 686` | `+0` |
| `internal/lang/lexer/unicode_safety.go` | `594 → 594` | `879 → 1069` | `1473 → 1663` | **`+190`** |
| `internal/layoutrules/exceptions.go` | `5 → 5` | `22 → 22` | `27 → 27` | `+0` |
| `internal/stdlib/logging/terminal_unix.go` | `4 → 4` | `0 → 0` | `4 → 4` | `+0` |
| `internal/stdlib/ml/print.go` | `209 → 209` | `834 → 1029` | `1043 → 1238` | **`+195`** |
| `internal/testkit/known_list.go` | `1 → 1` | `0 → 0` | `1 → 1` | `+0` |
| `tools/codegraph/history.go` | `4 → 4` | `78 → 78` | `82 → 82` | `+0` |
| `tools/codegraph/test_paths_param.go` | `1 → 1` | `175 → 176` | `176 → 177` | **`+1`** |
| `tools/trlang_migrate/rules.go` | `0 → 0` | `0 → 0` | `0 → 0` | `+0` |

**En çok büyüyen üç dosyanın sebebi ve örnekleri:**
1. **`internal/stdlib/ml/print.go` (`+195` zayıf kanıt, `834 → 1029`):** Adım 2b'de `internal/lang/interp/builtins.go:23` satırındaki `{codes.FnPrint, printBuiltin}` yapı satırı `interp.printBuiltin -> name:TR_fn_print` kenarını ekledi. `interp.printBuiltin` (`internal/lang/interp/builtins.go:50`) argümanlarını yazdırırken `valueText` (`:54`) üzerinden `interp.TextValue.Text` arayüz metodunu çağırır; bu metot `dispatches_to` ile `ml.TensorValue.Text` (`internal/stdlib/ml/print.go:32`), `ml.TensorValue.Inspect` (`:27`) ve `cellTexts` (`:114`) yardımcısına bağlanır. Böylece `TR_print` kullanan `.tr` testleri (ör. `tests/tensor_test.tr`) `print.go` bildirimlerine 5–6 adımda (`weak evidence`) ulaştı.
2. **`internal/lang/lexer/unicode_safety.go` (`+190` zayıf kanıt, `879 → 1069`):** Adım 2c'de `internal/lang/parser/struct.go:37` (`read := body.field` / `body.method`) ve `internal/lang/parser/enum.go:56` (`read := body.variant` / `body.enumMethod`) yerel değişkenleri `via="var"` ile `structBody` metotlarını `parseStructStatement` ve `parseEnumStatement`'a bağladı; ayrıca Adım 2b'de `printBuiltin` üzerinden gelen `.tr` testleri derleyici/sözcük çözümleyici zincirinde 1 adım kısalınca `unicode_safety.go` içindeki `CheckIdentifier` / `Skeleton` işlevlerine 5–6 adımda ulaşan test sayısı `190` arttı.
3. **`internal/compute/gpu/profile.go` (`+59` zayıf kanıt, `233 → 292`):** Adım 2a'da `(*tensor.Tensor).Floor`, `Ceil`, `Relu`, `Exp`, `Sqrt` vb. 57 somut metot ifadesi argümanı `via="arg"` ile `ml.registerRounding` ve `ml.unit` kayıt işlevlerine bağlandı; `tensor` metotları GPU arka uç çağrıları üzerinden `internal/compute/gpu/profile.go` içindeki profil sayaçlarına bağlandığından `ml` testleri 5–6 adımda `profile.go`'ya ulaştı.
- Ayrıca doğrudan etkilenen dosyalarda güçlü test (`<= 4` adım) değişimleri: `internal/lang/parser/struct.go` güçlü test sayısı `63 -> 64` (`+1`; `tests about it: 13 -> 16`), `internal/lang/interp/builtins.go` içinde `tests about it: 30 -> 36` (`+6`) ve doğrudan `tests: 28 -> 89` (`+61`, `tests, indirect: 980 -> 913`'ten yükseldi), `internal/compute/tensor/quant.go` içinde `tests about it: 9 -> 11` (`+2`).

### 3. Uçta İzlenmeyen Go INFERRED `calls` Kenarları (Önce: `357`, Sonra: `271`)

| Kenar Türü (AST Bağlamı) | Önce (`8b9f31d`) Toplam (Test Dışı / Test) | Sonra (`HEAD`) Toplam (Test Dışı / Test) | Fark ve Açıklama |
| :-- | :-- | :-- | :-- |
| Çağrı argümanı metot ifadesi (`types.MethodExpr`) | `57` (`27` / `30`) | `0` (`0` / `0`) | **`-57`** (Adım 2a ile tamamı `via="arg"` oldu ve zincire girdi) |
| Değişkene veya yapı alanına atama (`AssignStmt` / `ValueSpec`) | `46` (`28` / `18`) | `17` (`11` / `6`) | **`-29`** (Adım 2c ile `17` yerel değişkendeki `29` kenar `via="var"` oldu; kalan `17` kenar çağrılmayan/elenen değişken veya yapı alanı atamasıdır) |
| Bileşik değer — konumlu öğe (`CompositeLit`: dilim/yapı satırı) | `173` (`29` / `144`) | `173` (`29` / `144`) | `0` (Adım 2b `calls` değil `uses_name` kenarı ekler; test dosyalarındaki olanlar `valueTests` ile zaten okunur) |
| Bileşik değer — anahtarlı alan/harita (`KeyValueExpr`) | `65` (`19` / `46`) | `65` (`19` / `46`) | `0` |
| Ad kodlu anahtarlı tablo (`via="name:..."`, `keyedImplementations`) | `13` (`13` / `0`) | `13` (`13` / `0`) | `0` (`cmd/trlang/main.go` alt komut tablosu; `via="name:..."` ad kodu katmanından izlenir) |
| `return` deyiminde döndürülen işlev (`ReturnStmt`) | `3` (`2` / `1`) | `3` (`2` / `1`) | `0` (kapsam dışı) |
| **Toplam** | **`357` (`118` / `239`)** | **`271` (`74` / `197`)** | **`-86` (`-44` test dışı, `-42` test)** |

### 4. Grafın Kurulma Süresi (3 Koşunun Ortancası)

- **Önce (`c151974e`):** `9.5s`, `9.6s`, `9.5s` → ortanca **`9.5s`** (`35842 nodes, 294139 edges`).
- **Sonra (`HEAD`):** `9.5s`, `9.7s`, `9.4s` → ortanca **`9.5s`** (`35906 nodes, 294408 edges`).

### 5. "Araca Göre Test Yok" Listesi (`untested`, Önce ve Sonra)

| Grup (`testsForNode` Kolu) | Önce (2-0b Ucu `8b9f31d`, `6841` Düğüm) | Sonra (Dalın Ucu `HEAD`, `6844` Düğüm) | Fark ve Açıklama |
| :-- | :-- | :-- | :-- |
| `tested` | `6314` (tabanda `6311`) | `6311` | `-3` (`+2` yeni yardımcı `noteRowNameImpl` ve `noteLocalVar`, `+9` `weak -> tested`, `-14` `tested -> weak`) |
| `subprocess` | `92` | `92` | `0` (`cmd/trlang` işlevleri) |
| `rule` | `0` | `0` | `0` |
| `measured` | `2` | `2` | `0` (`interp.JSONVal.Operator`, `ml.TensorValue.matmulOp`) |
| `weak` | `328` | `333` | `+5` (`-9` `weak -> tested` çıktı, `+14` Adım 2a'da kalkan 18 `unit(...)` bağından düştü) |
| `exam` | `0` | `0` | `0` |
| `untested` | `105` | `106` | `+1` (Adım 2b'de eklenen `codegraph.goExtractor.noteSingleCodeFunc` testten 9 adımda olduğu için `TestMaxSteps = 8` sınırını 1 adım aşar) |

#### Gruplar Arası Geçişler
- **`weak` ("no tests traced to it", yalnız `cmd/trlang` ikili bağı) grubundan `tested` grubuna geçen 9 işlev:**
  1. `interp.printBuiltin` (`internal/lang/interp/builtins.go:50`) — Adım 2b (`{codes.FnPrint, printBuiltin}`)
  2. `interp.timeMsBuiltin` (`internal/lang/interp/builtins.go:192`) — Adım 2b (`{codes.FnTimeMs, timeMsBuiltin}`)
  3. `tensor.dequantQ8` (`internal/compute/tensor/quant.go:135`) — Adım 2c (`open := dequantQ8`)
  4. `tensor.dequantQ4K` (`internal/compute/tensor/quant.go:152`) — Adım 2c (`open = dequantQ4K`)
  5. `tensor.dequantQ6K` (`internal/compute/tensor/quant.go:170`) — Adım 2c (`open = dequantQ6K`)
  6. `ml.writeToFile` (`internal/stdlib/ml/save.go:201`) — Adım 2c (`write := writeToFile`)
  7. `ml.writeSafetensorsFile` (`internal/stdlib/ml/save_safetensors.go:61`) — Adım 2c (`write = writeSafetensorsFile`)
  8. `parser.structBody.variant` (`internal/lang/parser/enum.go:108`) — Adım 2c (`read := body.variant`)
  9. `parser.Parser.variantFields` (`internal/lang/parser/enum.go:133`) — Adım 2c (`parser.structBody.variant` çağırır)
  *(Not: `internal/` altındaki paketler `cmd/trlang` ikilisine bağlı olduğundan, izlenen testi olmayan işlevler `untested` yerine `weak` grubunda durur; bu nedenle 2a/2b/2c ile test kazanan 9 işlev `weak -> tested` geçişi yapmıştır. `untested` grubundaki 105 işlevin tamamı `cmd/trlang` ikilisine bağlı olmayan `tools/*` ve `internal/devtools/goload` paketlerindedir.)*
- **Adım 2a'nın kaldırdığı 18 `unit(...)` bağından düşen 14 işlev (`tested -> weak`):**
  - `ml.relu64` (`internal/stdlib/ml/builtin_elementwise.go:70`), `ml.sigmoid64` (`internal/stdlib/ml/builtin_elementwise.go:77`), `ml.silu64` (`internal/stdlib/ml/builtin_math.go:22`): `unit(v, codes.X, (*tensor.Tensor).X, x64)` çağrılarındaki skaler işlevler; `registerMath` çağrısını koşan doğrudan birim testi olmadığı ve `uses_name` kenarı iki işlevli `unit` çağrısından kalktığı için `weak` grubuna düştüler (skaler yol `.tr` testlerinde `ml.TensorValue.operator` / `eval` üzerinden koşulur).
  - `detmath.acos` (`internal/detmath/asin.go:61`), `detmath.asin` (`internal/detmath/asin.go:26`), `detmath.cosh` (`internal/detmath/sinh.go:81`), `detmath.sinh` (`internal/detmath/sinh.go:31`), `detmath.expm1` (`internal/detmath/expm1.go:51`), `detmath.expm1Reduce` (`internal/detmath/expm1.go:115`), `detmath.expm1Scale` (`internal/detmath/expm1.go:149`), `detmath.log1p` (`internal/detmath/log1p.go:53`), `detmath.log1pReduce` (`internal/detmath/log1p.go:130`), `detmath.tan` (`internal/detmath/tan.go:64`), `detmath.tanh` (`internal/detmath/tanh.go:59`): dışa açık sarmalayıcıları (`detmath.Asin`, `Acos`, `Sinh`, `Cosh`, `Expm1`, `Log1p`, `Tan`, `Tanh`) `internal/compute/tensor/elementwise_wide_test.go` içinde işlev değeri tablosunda (`unaryRefs`) geçtiği için `valueTests` yalnız doğrudan kullanılan dışa açık sarmalayıcıyı `tested` sayar (`valueTests` gövde içine yürümez); `unit(...)`'in `uses_name` kenarı kalkınca iç yardımcılar `tested -> weak` oldu.
- **`tested -> untested` geçen 1 işlev:**
  - `codegraph.goExtractor.noteSingleCodeFunc` (`tools/codegraph/gocode_scan.go:426`): Adım 2b'de eklenen yardımcı metot; `TestTypeQueryScopeSelectsEveryGoChange` (`tools/codegraph/type_queries_test.go:374`) `buildGraph` -> `extractGo` -> `unit` -> `file` -> `funcDecl` -> `scan` -> `noteRowNameImpl` zinciriyle `noteRowNameImpl`'a tam `8` adımda (`TestMaxSteps = 8`) ulaşırken `noteSingleCodeFunc` `9.` adımda (ve `TestTableRowNameCodeIsImplementedByRowCallable` -> `checkNameImplModule` -> `buildGraph` üzerinden `10.` adımda) kaldığı için `TestMaxSteps = 8` sınırını aşar ve `untested` grubunda görünür.

#### Hâlâ `untested` Kalan 106 İşlevin Pakete Göre Dağılımı (`UZAK9_TESTSIZ.txt`)

| Paket | `untested` İşlev/Metot Sayısı | Açıklama |
| :-- | :-- | :-- |
| `codegraph` (`tools/codegraph`) | `50` | Çoğu `extractGo` -> `unit` -> `file` -> `funcDecl` -> `scan` altında `TestMaxSteps = 8` adım sınırını aşan (9–11 adım) iç yardımcılar ve `dupRules` tablo işlevleri |
| `trlang_migrate` (`tools/trlang_migrate`) | `28` | Tek seferlik CLI göç aracı (`*_test.go` yok) |
| `confusables` (`tools/gen/confusables`) | `6` | Unicode confusables üreteci (`*_test.go` yok) |
| `edithook` (`tools/edithook`) | `3` | `main`, `testsFor`, `codegraphBin` (testler sahte `ask` işlevi verir) |
| `go` (`tools/bench/go`) | `3` | Go karşılaştırma ölçüm programı (`fib`, `main`, `ms`) |
| `grammar` (`tools/gen/grammar`) | `3` | `main` ve 2 yardımcı (`main_test.go` doğrudan `Generate` çağırır) |
| `genmain` (`tools/internal/genmain`) | `2` | `RunMain` ve ` repoRoot` (`*_test.go` yok, üreteçlerin `main` sarmalayıcısı) |
| `impact` (`tools/impact`) | `2` | `main` ve `run` (`main_test.go` iç işlevleri doğrudan çağırır) |
| `record` (`tools/record`) | `2` | `main` ve `defaultOut` (`main_test.go` doğrudan `run` çağırır) |
| `checkstatic` (`tools/checkstatic`) | `1` | `main` (`main_test.go` doğrudan `check` çağırır) |
| `codeconsts` (`tools/gen/codeconsts`) | `1` | `main` (`main_test.go` doğrudan `Generate` çağırır) |
| `codesdoc` (`tools/gen/codesdoc`) | `1` | `main` (`main_test.go` doğrudan `Generate` çağırır) |
| `goload` (`internal/devtools/goload`) | `1` | `importerFunc.Import` (`go/types.Importer` standart kütüphane arayüzü) |
| `msgconsts` (`tools/gen/msgconsts`) | `1` | `main` (`main_test.go` doğrudan `Generate` çağırır) |
| `renumber` (`tools/gen/renumber`) | `1` | `main` (`main_test.go` doğrudan `run` çağırır) |
| `snippets` (`tools/gen/snippets`) | `1` | `main` (`main_test.go` doğrudan `Generate` çağırır) |
| **Toplam** | **`106`** | Tam liste `UZAK9_TESTSIZ.txt` dosyasındadır (`542` satır). |

### 6. İkinci Yol: 20'lik Örneklemin Aracı Kullanmadan Doğrulanması

Seçim kuralı: `UZAK9_TESTSIZ.txt` içindeki sıralı `untested (106)` listesinde her 5. işlev (0-tabanlı dizinler `0, 5, 10, ..., 95`; 1-tabanlı `#1, #6, #11, ..., #96`, toplam 20 işlev). Her biri için `git grep` ile test dosyalarında adı/ad kodu arandı ve kaynak dosyalarda çağıran zinciri elle okundu:

| Sıra | İşlev / Metot ve Konumu | Testlerde `grep` | Kaynakta Çağıran Zinciri | Sonuç ve Nedeni |
| :-- | :-- | :-- | :-- | :-- |
| `#1` (`idx 0`) | `goload.importerFunc.Import` (`internal/devtools/goload/goload.go:228`) | Yok | `goload.Load` (`:88`'de `importerFunc` oluşturur) → `:218`'de `(&types.Config{Importer: imp}).Check(...)` | **Test var ama araç görmüyor:** standart kütüphane `go/types.Importer` arayüzü üzerinden çağrılır (`implementations.go` yalnız modül içi arayüzleri bağlar); `goload.Load` çağıran her test onu koşar. |
| `#6` (`idx 5`) | `codegraph.argCallable` (`tools/codegraph/gocode_scan.go:454`) | Yok | `noteCallArgs` (`:404`), `noteSingleCodeFunc` (`:436`), `noteLocalVar` (`:520`) ← `scan` (`:92, 98, 123`) ← `funcDecl`/`scanValueSpecs` ← `file` ← `unit` ← `extractGo` ← `buildGraph` ← `buildInto`/`fixtureGraph` ← `Test*` | **Test var ama araç görmüyor:** `tools/codegraph` testleri çalıştırır; ancak testten çağrı zinciri 9 adım olduğu için `TestMaxSteps = 8` (`internal/devtools/codeindex/reach.go:13`) sınırını aşar. |
| `#11` (`idx 10`) | `codegraph.goExtractor.addPathRef` (`tools/codegraph/test_paths.go:221`) | Yok | `pathJoin` (`:130`), `pathLit` (`:211`), `citeRefs` (`:259`), `noteEmbed` (`:296`) ← `scan`/`file` ← `funcDecl` ← `file` ← `unit` ← `extractGo` ← `buildGraph` ← `buildInto` ← `Test*` | **Test var ama araç görmüyor:** `test_paths_test.go` testleri çalıştırır; çağrı zinciri 9 adım (`> TestMaxSteps = 8`). |
| `#16` (`idx 15`) | `codegraph.goExtractor.collectRangeProducers` (`tools/codegraph/walks.go:230`) | Yok | `rangeUse` (`tools/codegraph/walks.go:101`) ← `scan` (`tools/codegraph/gocode_scan.go:107`) ← `funcDecl` ← `file` ← `unit` ← `extractGo` ← `buildGraph` ← `buildInto` ← `Test*` | **Test var ama araç görmüyor:** `test_paths_test.go` (`TestEmbeddedDataFilesLinkToTheTestsThatWalkThem`) çalıştırır; çağrı zinciri 9 adım (`> TestMaxSteps = 8`). |
| `#21` (`idx 20`) | `codegraph.goExtractor.noteRangeTable` (`tools/codegraph/tr_snippets.go:223`) | Yok | `noteSnippets` (`tools/codegraph/tr_snippets.go:331`) ← `scan` (`tools/codegraph/gocode_scan.go:93`) ← `funcDecl` ← `file` ← `unit` ← `extractGo` ← `buildGraph` ← `buildInto` ← `Test*` | **Test var ama araç görmüyor:** `tr_snippets` kullanan `codegraph` testleri çalıştırır; çağrı zinciri 9 adım (`> TestMaxSteps = 8`). |
| `#26` (`idx 25`) | `codegraph.goExtractor.rangeStringLookups` (`tools/codegraph/walks.go:114`) | Yok | `rangeUse` (`tools/codegraph/walks.go:103`) ← `scan` (`tools/codegraph/gocode_scan.go:107`) ← `funcDecl` ← `file` ← `unit` ← `extractGo` ← `buildGraph` ← `buildInto` ← `Test*` | **Test var ama araç görmüyor:** `walks` testleri çalıştırır; çağrı zinciri 9 adım (`> TestMaxSteps = 8`). |
| `#31` (`idx 30`) | `codegraph.isConversion` (`tools/codegraph/tr_snippets.go:184`) | Yok | `textSources` (`tools/codegraph/tr_snippets.go:144`) ve `writePathExpr` (`tools/codegraph/tr_file_writes.go:67`) ← `noteSnippets` ← `scan` ← `funcDecl` ← `file` ← `unit` ← `extractGo` ← `buildGraph` ← `buildInto` ← `Test*` | **Test var ama araç görmüyor:** çağrı zinciri 10–11 adım (`> TestMaxSteps = 8`). |
| `#36` (`idx 35`) | `codegraph.joinTexts` (`tools/codegraph/tr_snippets.go:160`) | Yok | `textSources` (`tools/codegraph/tr_snippets.go:141`) ve `writePathExpr` (`tools/codegraph/tr_file_writes.go:41, 54`) ← `noteSnippets` ← `scan` ← ... ← `Test*` | **Test var ama araç görmüyor:** çağrı zinciri 10–11 adım (`> TestMaxSteps = 8`). |
| `#41` (`idx 40`) | `codegraph.rangeExprType` (`tools/codegraph/walks.go:162`) | Yok | `rangeUse` (`tools/codegraph/walks.go:99`) ← `scan` (`tools/codegraph/gocode_scan.go:107`) ← `funcDecl` ← `file` ← `unit` ← `extractGo` ← `buildGraph` ← `buildInto` ← `Test*` | **Test var ama araç görmüyor:** çağrı zinciri 9 adım (`> TestMaxSteps = 8`). |
| `#46` (`idx 45`) | `codegraph.skipWalk.negation` (`tools/codegraph/test_skips.go:311`) | Yok | `skipWalk.condition` (`:300`) ← `walkStmt` (`:200`) ← `funcTestSkips` (`:156`) ← `fileTestSkips` (`:126`) ← `testSkips` (`:103`) ← `extractGo` ← `buildGraph` ← `fixtureGraph` ← `Test*` | **Test var ama araç görmüyor:** `test_skips` testleri çalıştırır; çağrı zinciri 9 adım (`> TestMaxSteps = 8`). |
| `#51` (`idx 50`) | `codegraph.testFloat64Reference` (`tools/codegraph/dups_exclude.go:78`) | Yok | `dupRules` yapı dilimi (`tools/codegraph/dups_exclude.go:33`: `Match: testFloat64Reference`) → `applyRules` (`tools/codegraph/dups_exclude.go:101`: `r.Match(c, co)`) ← `runDups` (`tools/codegraph/dups_report.go:145`) ← `TestDups*` | **Test var ama araç görmüyor:** ad kodu olmayan bileşik yapı satırında (`dupRules`) işlev alanı olarak tutulur ve yapı alanı (`r.Match`) üzerinden çağrılır; `TestDups*` testleri onu çalıştırır. |
| `#56` (`idx 55`) | `edithook.main` (`tools/edithook/main.go:64`) | Yok | Giriş noktası (`main`); `tools/edithook/main_test.go` doğrudan `run` ve `mode` işlevlerini çağırır | **Gerçekten testsiz:** `main()` sarmalayıcısıdır ve `tools/edithook` ikilisini alt süreç olarak koşan test yoktur. |
| `#61` (`idx 60`) | `confusables.codePoints` (`tools/gen/confusables/main.go:135`) | Yok | `parse` (`tools/gen/confusables/main.go:121, 125`) ← `run` (`:85`) ← `main` (`:50`) | **Gerçekten testsiz:** `tools/gen/confusables` paketinde hiç `*_test.go` dosyası yoktur ve başka paketten çağrılmaz. |
| `#66` (`idx 65`) | `confusables.sourceLine` (`tools/gen/confusables/main.go:103`) | Yok | `check` (`tools/gen/confusables/main.go:71`) ve `generate` (`:156`) ← `run` ← `main` | **Gerçekten testsiz:** `tools/gen/confusables` paketinde hiç `*_test.go` dosyası yoktur. |
| `#71` (`idx 70`) | `snippets.main` (`tools/gen/snippets/main.go:35`) | Yok | Giriş noktası (`main`); `tools/gen/snippets/main_test.go` doğrudan `Generate()` çağırır | **Gerçekten testsiz:** `main()` sarmalayıcısıdır; testler `Generate()` işlevini doğrudan çağırır. |
| `#76` (`idx 75`) | `record.defaultOut` (`tools/record/main.go:100`) | Yok | Yalnız `record.main` (`tools/record/main.go:86`: `flag.StringVar(&o.out, "out", defaultOut(), ...)`) içinde çağrılır | **Gerçekten testsiz:** `tools/record/main_test.go` `main()` yerine doğrudan `run(options{out: t.TempDir(), ...})` çağırır. |
| `#81` (`idx 80`) | `trlang_migrate.applyExtraLine` (`tools/trlang_migrate/main.go:207`) | Yok | `processFile` (`tools/trlang_migrate/main.go:323`) ← `main` (`:388`) | **Gerçekten testsiz:** `tools/trlang_migrate` tek seferlik göç aracında hiç test dosyası yoktur. |
| `#86` (`idx 85`) | `trlang_migrate.catStat.add` (`tools/trlang_migrate/main.go:30`) | Yok | `applyS3Line` (`:120, 127, 135, 145, 150, 154`) ve `applyExtraLine` (`:216, 221`) | **Gerçekten testsiz:** `tools/trlang_migrate` paketinde hiç test dosyası yoktur. |
| `#91` (`idx 90`) | `trlang_migrate.leftPlain` (`tools/trlang_migrate/main.go:88`) | Yok | `leftNoOnly` (`:94`), `applyS3Line` (`:111, 116, 123`), `applyTemplates` (`:164, 169`), `applyDocAndTestExtras` (`:244, 258`) | **Gerçekten testsiz:** `tools/trlang_migrate` paketinde hiç test dosyası yoktur. |
| `#96` (`idx 95`) | `trlang_migrate.prevRune` (`tools/trlang_migrate/rules.go:160`) | Yok | `leftPlain` (`tools/trlang_migrate/main.go:89`), `applyS3Line` (`:139`), `applyBinaryPaths` (`:195`), `applyExtraLine` (`:210`) | **Gerçekten testsiz:** `tools/trlang_migrate` paketinde hiç test dosyası yoktur. |

**20'lik örneklem özeti:** `11` "test var ama araç görmüyor" (`1` standart kütüphane arayüzü, `9` `TestMaxSteps = 8` adım sınırını aşan `codegraph` iç yardımcısı, `1` ad kodu olmayan yapı alanında tutulan işlev değeri), `9` "gerçekten testsiz", `0` "emin değilim".

---

## Değişen Altın ve Planlama Sınavı Satırları

1. **`tools/codegraph/testdata/extractor_digest.txt`:**
   - Çıkarıcı sürümü `extractorRevision = 91`'den `94`'e, özet `fb5b633c173ebce2`'den `3925fd751678db8b`'ye güncellendi (`gocode_scan.go` ve `test_reach.go` değişiklikleri).
2. **`tools/codegraph/testdata/planning_questions.txt:89`:**
   - Önce: `codegraph tests-for tensor.Tensor.Ceil | not: no tests found | group: function value => tensor.TestUnaryNoGrad | group: function value => via gpu.wideUnaries | src: bağımsız denetim Y2, 2026-10-24`
   - Sonra: `[kanıtlı: tools/codegraph/testdata/planning_questions.txt:89, "codegraph tests-for tensor.Tensor.Ceil | not: no tests found | group: tests about it => tensor.TestUnaryNoGrad | group: function value => tensor.TestUnaryWideEdges | group: function value => via gpu.wideUnaries | src: bağımsız denetim Y2, 2026-10-24"]`
   - **Gerekçe:** Adım 2a'da `internal/stdlib/ml/builtin_math.go:43` satırındaki `[kanıtlı: internal/stdlib/ml/builtin_math.go:43, "registerRounding(v, codes.FnCeil, (*tensor.Tensor).Ceil)"]` çağrısı `tensor.Tensor.Ceil -> name:TR_fn_ceil` (`tavan`) kenarını ekledi. `tensor.TestUnaryNoGrad` (`internal/compute/tensor/elementwise_wide_test.go:220`) belge yorumunda `[kanıtlı: internal/compute/tensor/elementwise_wide_test.go:220, "// TestUnaryNoGrad taban, tavan ve işaretin gradyanı her yerde 0'dır (işaret 0'da da)."]` `tavan` adını taşıdığı için `aboutTests` onu `tests about it` (`[names it]`) grubuna aldı ve `keepStrongest` (`tools/codegraph/query_tests.go:333`–`:345`) daha güçlü grup olan `tests about it`'te tutarak `function value` grubundan çıkardı. `function value` grubunda doğrudan test içindeki dilim kullanımı için `tensor.TestUnaryWideEdges` (`internal/compute/tensor/elementwise_wide_test.go:136`) ve yardımcı tablo kullanımı için `via gpu.wideUnaries` korundu.
3. **`tools/codegraph/testdata/member_spellings.txt` ve `tools/codegraph/codes_test.go`:** Hiç dokunulmadı.

---

## Bitti Ölçütü Komutları ve Son Satırları

1. `gofmt -l tools`
   - Son satır: *(boş çıktı, exit 0)* `[ölçüldü: gofmt -l tools, boş]`
2. `go test ./tools/codegraph/...`
   - `TestEveryTrackedDataFileHasATestOrAReason` beklenen şekilde yalnız kökteki geçici `UZAK9_` dosyalarını (`UZAK9_DURUM.md`, `UZAK9_TESTSIZ.txt`, `UZAK9_RAPORU.md`) raporlar; onun dışındaki tüm testler (`TestPlanningQuestionsAreAnswered` ve `TestDottedMemberSpellingsGolden` dahil) geçer:
   - Son satır (`TestEveryTrackedDataFileHasATestOrAReason` hariç): `ok  	github.com/sekerahmet/TCB/tools/codegraph` `[ölçüldü: GOTOOLCHAIN=local go test -skip TestEveryTrackedDataFileHasATestOrAReason ./tools/codegraph/...]`
3. `go test ./internal -run 'TestClassedIdentitiesAreLive'`
   - Son satır: `ok  	github.com/sekerahmet/TCB/internal	1.242s` `[ölçüldü: GOTOOLCHAIN=local go test ./internal -run 'TestClassedIdentitiesAreLive', ok]`
4. `go run ./tools/codegraph dups --changed main`
   - Son satır: `0 duplicate candidate(s) touching main...HEAD (0 shown; >= 100 nodes; 1334 files, 1153884 nodes, 1.5s)` `[ölçüldü: GOTOOLCHAIN=local go run ./tools/codegraph dups --changed main]`
5. `go run ./tools/codegraph tests-for tensor.dequantQ6K`
   - Son satır: `    Belirlenimcidir [rule] internal/compute/tensor/README.md:13` (`29` indirect, `60` weak, `9` measurements, `2` rules) `[ölçüldü: GOTOOLCHAIN=local go run ./tools/codegraph tests-for tensor.dequantQ6K]`
6. `go run ./tools/codegraph tests-for interp.bytesCall`
   - Son satır: `  <- weak evidence (142): linked into the binary of cmd/trlang (142 tests run it); a subprocess test may reach this code, which one is not traced` `[ölçüldü: GOTOOLCHAIN=local go run ./tools/codegraph tests-for interp.bytesCall]`
7. `go run ./tools/codegraph query TR_fn_exp`
   - Son satır: `  documented: TR_exp(...) (TR_fn_exp, exp) [heading] docs/codes.md:1919` (`ml.registerMath at internal/stdlib/ml/builtins.go:101 -> detmath.Exp [internal/detmath/exp.go:18], tensor.Tensor.Exp [internal/compute/tensor/elementwise.go:314]`) `[ölçüldü: GOTOOLCHAIN=local go run ./tools/codegraph query TR_fn_exp]`
8. `go run ./tools/codegraph untested --json`
   - Son satır: `}` (`"schema": "codegraph-untested/1", "total": 6844`, `counts`: `tested: 6311, subprocess: 92, rule: 0, measured: 2, weak: 333, exam: 0, untested: 106`) `[ölçüldü: GOTOOLCHAIN=local go run ./tools/codegraph untested --json]`
9. `git diff --stat main...HEAD`
   - Son satır: yalnızca `tools/codegraph/` altındaki dosyalar ve kökteki `UZAK9_DURUM.md`, `UZAK9_TESTSIZ.txt`, `UZAK9_RAPORU.md` dosyaları.

---

## Ortam

- Linux (`Debian 12`, `linux/amd64`), Go `go1.27.3` (`GOTOOLCHAIN=local`).
- Temiz makine bulgusu: yerel makineye bağımlı olup bu ortamda düşen bir test görülmedi (`planning_questions.txt` içindeki git geçmişi isteyen 7 satır sığ klon algılamasıyla kendiliğinden `skipped, the clone has no history` olarak atlandı). Uzun süren komutlar arasında uzak sanal makine (uVM) bir kez duraklatılıp yeniden başlatıldı (`STOPPED state`); `/tmp` altındaki geçici dosyalar dışında veri kaybı olmadı.

## Codegraph

1. **`tests-for codegraph.goExtractor.noteSingleCodeFunc` (ve `codegraph.argCallable` gibi 9+ adımlık iç yardımcılar):**
   - **Komut:** `go run ./tools/codegraph tests-for codegraph.argCallable`
   - **Beklenen:** `TestAdjacentNameCodeInCallIsImplementedByArgumentCallable` ve `TestTestsForFollowsPackageFunctionPassedAsCallArgument` testlerinin dolaylı/zayıf test olarak görünmesi.
   - **Gelen:** `no tests found` (`untested` grubunda).
   - **Doğrusunu nasıl buldum:** `tools/codegraph/gocode.go`, `tools/codegraph/gocode_scan.go` ve `tools/codegraph/query_names_test.go:66` (`checkNameImplModule`) okunduğunda `TestTypeQueryScopeSelectsEveryGoChange` -> `buildGraph` (1) -> `extractGo` (2) -> `unit` (3) -> `file` (4) -> `funcDecl` (5) -> `scan` (6) -> `noteRowNameImpl` (7) -> `noteSingleCodeFunc` (8) -> `argCallable` (9) (veya `TestTableRowNameCodeIsImplementedByRowCallable` -> `checkNameImplModule` (1) -> `buildGraph` (2) -> ... -> `noteSingleCodeFunc` (10) -> `argCallable` (11)) çağrı zincirinin `TestMaxSteps = 8` (`internal/devtools/codeindex/reach.go:13`) sınırını aştığı görüldü (`noteRowNameImpl` tam 8 adımda kaldığı için `weak evidence` alır, `noteSingleCodeFunc` 9 ve `argCallable` 10 adımda kaldığı için düşer).
2. **`tests-for interp.bytesCall`:**
   - **Komut:** `go run ./tools/codegraph tests-for interp.bytesCall`
   - **Beklenen:** Adım 2b'de `{codes.TypeBytes, bytesCall}` konumlu tablo satırı bağlandıktan sonra `TR_Bytes` kullanan testlerin görünmesi.
   - **Gelen:** `no tests traced to it` (`weak evidence (142)`).
   - **Doğrusunu nasıl buldum:** `tools/codegraph/test_names.go:28`–`:37` (`implementable`) okunduğunda `codes.ClassType` kimliklerinin (`TR_type_Bytes`, `TR_type_Type`, `TR_type_JSON`) her tip bildiriminde geçtiği için bilerek dışarıda bırakıldığı, `bytesCall` gövdesinde de `codes.TypeBytes` EXTRACTED `uses_name` kenarının (`internal/lang/interp/bytes_value.go:188`) zaten tabanda var olduğu görüldü.

## Emin Olmadıklarım ve Bakmadıklarım

- **Emin olmadıklarım:** Yoktur; 20'lik örneklemin tamamı ve tüm kenar farkları kaynak koddan doğrulandı.
- **Bakmadıklarım:** Yönerge gereği `tools/codegraph/gaps.md`, `tools/codegraph/codes_test.go`, `tools/codegraph/testdata/member_spellings.txt` ve `tools/codegraph/` dışındaki paketlerin üretim kodlarına dokunulmadı; akışa duyarlı yerel değişken analizi, paket düzeyi işlev değişkenleri (`25`), işlev tipli yapı alanları (`45`), işlev döndüren işlevler (`146`) ve ad kodu olmayan işlev tabloları (`6`) kapsam dışı bırakıldı.

## Kendi Kontrolüm

- Rapordaki **13** `[kanıtlı: ...]` birebir alıntısının tamamı `grep -n -F "<alıntı>" <dosya>` komutuyla ve kaynak dosyanın ilgili satırı açılarak karakter karakter karşılaştırıldı (`13/13` birebir eşleşti, `0` alıntı silindi).
- Raporda geçen **135** `dosya:satır` atfı (**121** tekil `dosya:satır` konumu) kaynak dosyalarda yeniden açılarak satır numaraları ve bildirim adları doğrulandı:
  - `1` yerde işlev ve satır adı düzeltildi (`internal/stdlib/ml/print.go` açıklamasında `ml.TensorValue.String` / `:19` yerine gerçekte çağrılan `ml.TensorValue.Text` `:32`, `ml.TensorValue.Inspect` `:27` ve `cellTexts` `:114` yazıldı);
  - `32` yerde yalnız dosya adıyla yazılmış kısa atıflar (`main_query.go:128`, `sinh.go:81`, `walks.go:101`, `bytes_value.go:188` vb.) depo kökünden tam göreli yollarıyla (`tools/codegraph/...`, `internal/detmath/...`, `internal/lang/interp/...`) değiştirildi.
- Bitti ölçütü komutlarının tamamı koşuldu ve son satırları rapora işlendi.

Kendi incelemem bitti.

