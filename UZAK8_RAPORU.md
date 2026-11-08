# Uzak İş 8 Raporu: `tools/codegraph` — Argüman Olarak Geçen Metot Değeri ve Yanındaki Ad Kodu

- **Dal:** `gemini/uzak-8`
- **Taban (`main`):** `47e0984cddfc726ca8e4dc7fdf423af4a1ab67a9`
- **Başlama saati:** 2026-11-08 16:11:08 UTC+3 (13:11:08 UTC)
- **Bitiş saati:** 2026-11-08 17:26:00 UTC+3 (14:26:00 UTC)
- **Bağlam sıkıştırmaları:**
  1. `2026-11-08 16:27:42 UTC+3 (13:27:42 UTC)` — Adım 1 tamamlanıp commit edildikten (`b34e285`) hemen sonra, Adım 2a düzenlemelerine başlamadan önce gerçekleşti; `CLAUDE.md`, `UZAK_YONERGE_8.md`, `UZAK8_DURUM.md` ve hedef kaynak dosyalar yeniden açıldı.
  2. `2026-11-08 17:12:26 UTC+3 (14:12:26 UTC)` — Adım 2a (`fd9e8c4`), Adım 2b (`126cc02`), Adım 3 ölçümleri ve `go test ./tools/codegraph/...` tamamlandıktan sonra, uzak sanal makinenin duraklatılıp yeniden başlatılmasının (`17:10:37 UTC+3`) hemen ardından gerçekleşti; `CLAUDE.md`, `UZAK_YONERGE_8.md` ve `UZAK8_DURUM.md` yeniden açıldı.

---

## 1. Kısa Özet Tablosu

| Ölçüt | Önce (`main` `47e0984c`, rev 89) | Sonra (`126cc02`, rev 91) | Fark ve Açıklama |
| :--- | :--- | :--- | :--- |
| `tests-for ml.TensorValue.toDevice` | `no tests traced to it` (zayıf: `142`, ikili bağlantı) | Doğrudan ad/ileti: `3`, ad kodu (`TR_mem_to_device`): `18`, dolaylı: `0`, zayıf: `3` (`>4` adım) | `0 → 21` gerçek test (`+3` zayıf çağrı zinciri) |
| `tests-for ml.tensorLoad` | `no tests traced to it` (zayıf: `142`, ikili bağlantı) | Doğrudan ad/ileti: `4`, ad kodu (`TR_fn_load_tensors`): `29` | `0 → 33` gerçek test |
| `tests-for tokenizer.load` | `no tests traced to it` (zayıf: `142`, ikili bağlantı) | Doğrudan ad/ileti: `5`, ad kodu (`TR_fn_load_tokenizer`): `21` | `0 → 26` gerçek test |
| `tests-for files.tempDirBuiltin` | Doğrudan: `6`, ad kodu (`TR_fn_temp_dir`): `44` | Doğrudan: `6`, ad kodu (`TR_fn_temp_dir`): `44` | Değişmedi; gövdedeki (`internal/stdlib/files/temp.go:34`) `EXTRACTED` `uses_name` kenarı ezilmedi |
| `query TR_mem_to_device` | `uses_name (17)`, `registered or used in Go (3)` | `uses_name (18)`, `registered or used in Go (4)` | `ml.TensorValue.toDevice at internal/stdlib/ml/members.go:81` (`INFERRED`) eklendi |
| `tests-for --all interp.PathArg` (ad kodu testleri) | `111` test (`10` ad kodu, `<=4` adım) | `170` test (`14` ad kodu, `<=4` adım) | `+59` test; dönen `4` kodun tamamı gerçekten dosya yolu alır |
| Uçta izlenmeyen Go `INFERRED` `calls` (`Via != "arg"`) | Toplam `443` (metot hedefi: `311`); test dışı `193` (metot: `132`) | Toplam `357` (metot hedefi: `225`); test dışı `118` (metot: `57`) | `-86` `INFERRED` kenar (`+1` `EXTRACTED` kenar) artık `Via = "arg"` taşıyor |
| Graf `uses_name` kenar sayısı | `9038` | `9107` | `+69` yeni `INFERRED` `uses_name` kenarı (`109` tekil çiftten `40`'ı gövdede zaten `EXTRACTED` idi) |
| Graf kurulum süresi (3 sıcak koşu ortancası) | `9.6 s` (`9.6 s`, `9.6 s`, `10.1 s`) | `9.2 s` (`9.1 s`, `9.6 s`, `9.2 s`) | Gerileme yok (`-0.4 s`) |
| Üretim kodu satır farkı (`tools/codegraph/*.go`, test hariç) | — | `+94 / -18` satır (net `+76` satır) | ~80 satır tavanının altında (`gocode.go`, `gocode_scan.go`, `graph.go`, `test_reach.go`) |

---

## 2. Bulgular (En Önemliden Başlayarak)

### Bulgu 1: Çağrı argümanı olarak geçen somut alıcılı metot değerinin (`x.M`) test erişim zincirine girmesi (Adım 2a — Çözüldü)

- **İddia:** `tools/codegraph` çıkarıcısı bir çağrının argümanı olarak geçen işlevlere `Via = viaArg` (`"arg"`) işaretini yalnız alıcısız işlevlerde (`sig.Recv() == nil`) koyuyordu; bu nedenle `d.method(memberToDevice, d.toDevice)` gibi somut alıcılı metot değerleri (`types.MethodVal`) test erişim zincirine girmiyor ve test dışı dosyalarda `132` metot hedefi olan Go `INFERRED` `calls` kenarı izlenmeden kalıyordu.
- **Kanıt:**
  - `[kanıtlı: tools/codegraph/gaps.md:45, "İşlev değeri test zincirini koparıyor: konumlu tablo, değişkenle aktarım, metot değeri (CODEGRAPH bildirdi, 2026-10-26; argümanlı yarısı kapandı). Kritik: yanlış \"test yok\". Sahip: CODEGRAPH."]`
  - `[kanıtlı: internal/stdlib/ml/members.go:81, "return d.method(memberToDevice, d.toDevice), nil"]`
  - `[kanıtlı: tools/codegraph/gocode_scan.go:321, "if callKind == viaArg && argCallable(fc, fn, stack[len(stack)-2]) {"]`
  - `[kanıtlı: tools/codegraph/gocode_scan.go:414, "return s != nil && s.Kind() == types.MethodVal && !types.IsInterface(s.Recv()) && !types.IsInterface(sig.Recv().Type())"]`
  - `[ölçüldü: go run ./tools/codegraph build --out /tmp/after_graph.json, depoda somut alıcılı MethodVal kullanımı 144 yerde; 101'i doğrudan çağrı argümanı olup 86 INFERRED ve 1 EXTRACTED calls kenarına Via = "arg" eklendi; uçta izlenmeyen Go INFERRED calls kenarı toplamda 443 -> 357, metot hedefliler 311 -> 225, test dışı dosyalarda metot hedefliler 132 -> 57 oldu]`
  - `[test: tools/codegraph/query_tests_test.go]` (`TestTestsForFollowsPackageFunctionPassedAsCallArgument`) ve `[test: tools/codegraph/test_names_test.go]` (`TestNameCodeTestsDoNotFollowAFunctionPassedAsAnArgument`).
- **Öneri:** Commit `fd9e8c4` ile uygulandı. `use` içinde `stack[len(stack)-2]` düğümünden `*ast.SelectorExpr` alınarak `fc.u.Info.Selections[sel]` üzerinden `s.Kind() == types.MethodVal && !types.IsInterface(s.Recv()) && !types.IsInterface(sig.Recv().Type())` denetimi eklendi; ad kodu katmanı (`nameCallersOf`, `tools/codegraph/query_tests.go:208`) `Via == viaArg` kenarlarını izlememeye devam eder.

### Bulgu 2: Kayıt çağrısında yan yana duran tek ayrı ad kodu ile tek işlev/metot değerinin `INFERRED` `uses_name` kenarıyla bağlanması (Adım 2b — Çözüldü)

- **İddia:** Test dışı bir çağrının argümanlarında ve bir düzey iç çağrı argümanlarında (`wrap(v, codes.FnLoadTensors, newBuiltin(tensorLoad))`, `d.method(memberToDevice, d.toDevice)`, `safe(codes.FnTempDir, tempDirBuiltin)`) tek ayrı bir ad kodu (`codes` sabiti ya da `codeAliases` takma sabiti; `codes` paketinin işlevine argüman olarak verilen `nameReads` hariç) ve argüman olarak geçen tek bir işlev/somut metot değeri bulunduğunda, işlevden koda henüz bir `uses_name` kenarı yoksa `INFERRED` `uses_name` kenarı eklenmesi, hem kayıt işlevine geçirilen gerçek uygulayıcıların testlerini geri kazandırır hem de kayıt işlevinin diğer kodlarının sızmasını önler.
- **Kanıt:**
  - `[kanıtlı: tools/codegraph/gaps.md:52, "Kayıt işlevinden gelen gerçek ad kodları kayboluyor (2026-11-08, DENETİM 279 Ş1, 282 N2)."]`
  - `[kanıtlı: tools/codegraph/gocode_scan.go:399, "x.argImpls = append(x.argImpls, argNameImpl{fn: fnRef, code: \"name:\" + code, file: fc.rel, line: x.line(fnID.Pos())})"]`
  - `[kanıtlı: tools/codegraph/gocode.go:152, "if x.b.edges[edgeKey{p.fn, p.code, edgeUsesName}] == nil {"]`
  - `[ölçüldü: go run ./tools/codegraph tests-for ml.TensorValue.toDevice, 0 testten 3 doğrudan + 18 ad kodu testine çıktı; ml.tensorLoad 0 testten 4 doğrudan + 29 ad kodu testine çıktı; tokenizer.load 0 testten 5 doğrudan + 21 ad kodu testine çıktı; interp.PathArg ad kodu testleri 111'den 170'e çıktı]`
  - `[test: tools/codegraph/query_names_test.go]` (`TestAdjacentNameCodeInCallIsImplementedByArgumentCallable`).
- **Öneri:** Commit `126cc02` ile uygulandı.

### Bulgu 3: Metot ifadesinin (`T.M`, `types.MethodExpr`) depodaki ölçümü ve değerlendirmesi (Adım 2a Kapsam Dışı Ölçüm)

- **İddia:** Depoda `s.Kind() == types.MethodExpr` olan toplam `217` seçici ifadesi vardır; bunların `57`'si doğrudan bir çağrının argümanı olarak geçmekte (`22`'si test dışı dosyalarda, `35`'i test dosyalarında), `160`'ı ise bileşik değer tablolarında (`[]struct{...}` / `map[...]...`) yer almaktadır. Çağrı argümanı olan `22` test dışı kullanımın `18`'i `unit(v, codes.FnRelu, (*tensor.Tensor).Relu, relu64)` biçiminde hem metot ifadesi hem skaler türev işlevi alan çağrılar, `4`'ü ise `registerRounding(v, codes.FnFloor, (*tensor.Tensor).Floor)` biçiminde tek metot ifadesi alan kayıt çağrılarıdır.
- **Kanıt:**
  - `[ölçüldü: go test ./tools/codegraph üzerinden go/types Info.Selections taraması, MethodExpr toplam 217: 57 çağrı argümanı (internal/stdlib/ml/builtins.go:96-104 içinde 7, internal/stdlib/ml/builtin_math.go:31-45 içinde 15, internal/compute/tensor/*_test.go içinde 35) ve 160 çağrı argümanı dışı bileşik değer]`
  - `[kanıtlı: tools/codegraph/gocode_scan.go:414, "return s != nil && s.Kind() == types.MethodVal && !types.IsInterface(s.Recv()) && !types.IsInterface(sig.Recv().Type())"]`
- **Öneri (`[görüş]`):** Yönerge gereği bu işte `MethodExpr` zincir dışında bırakılmıştır (`s.Kind() == types.MethodVal`). Ancak ayrı bir adımda somut alıcılı metot ifadesinin de (`(s.Kind() == types.MethodVal || s.Kind() == types.MethodExpr) && !types.IsInterface(s.Recv()) && !types.IsInterface(sig.Recv().Type())`) `argCallable`'a alınması önerilir:
  1. Metot ifadesinde alıcı tipi sözdiziminde açıkça yazılıdır (`(*tensor.Tensor).Floor`), dolayısıyla çağrılacak somut metot metot değerindeki kadar bellidir.
  2. `registerRounding(v, codes.FnFloor, (*tensor.Tensor).Floor)` (`internal/stdlib/ml/builtin_math.go:42-45`) gibi 4 yuvarlama kayıt çağrısında `(*tensor.Tensor).Floor`, `Ceil`, `Round`, `Trunc` metotları hem `Via = viaArg` çağrı zincirine hem de Adım 2b üzerinden `TR_fn_floor`, `TR_fn_ceil`, `TR_fn_round`, `TR_fn_trunc` ad kodlarına bağlanır.
  3. `unit(v, codes.FnRelu, (*tensor.Tensor).Relu, relu64)` gibi 18 çağrıda bugün `(*tensor.Tensor).Relu` `MethodExpr` olduğu için `argCallable` tarafından sayılmamakta ve Adım 2b `nFuncs == 1` görerek ad kodunu yalnız skaler türev işlevi `relu64`'e bağlamaktadır; `MethodExpr` de `argCallable` sayıldığında bu 18 çağrıda `nFuncs == 2` (`(*tensor.Tensor).Relu` ve `relu64`) olacağı için tek yanlı bağlama kendiliğinden engellenir.

### Bulgu 4: `query <kod>` çıktısında "registered or used in Go" satırı ve `implementationsAt` davranışı (Adım 2b Risk Ölçümü)

- **İddia:** Adım 2b ile işlevden koda `INFERRED` `uses_name` kenarı eklendiğinde kenarın `From` düğümü uygulayan işlevin kendisidir (`ml.TensorValue.toDevice`) ve konum bilgisi kayıt çağrısının satırıdır (`internal/stdlib/ml/members.go:81`). `query TR_mem_to_device` çıktısında "registered or used in Go" listesine `ml.TensorValue.toDevice at internal/stdlib/ml/members.go:81` satırı eklenir; `implementationsAt` (`tools/codegraph/query_names.go:61`) o satırda `use.From`'dan çıkan bir `calls` kenarı aradığı ve `use.From` zaten hedef işlev olduğu için satırın sonunda `-> ...` soneki oluşmaz.
- **Kanıt:**
  - `[kanıtlı: tools/codegraph/query_names.go:68, "if e.Via == use.To {"]`
  - `[ölçüldü: go run ./tools/codegraph query TR_mem_to_device, "registered or used in Go (4; implementation named on the same line):" altında "ml.TensorValue.toDevice at internal/stdlib/ml/members.go:81" satırı yer alır]`
- **Öneri (`[görüş]`):** Mevcut çıktı uygulayan işlevin adını ve kayıt satırını doğrudan gösterdiği için doğrudur ve yanıltıcı değildir; ek bir kod değişikliğine gerek yoktur.

---

## 3. Adım 1: Taban (Önce) Çıktıları

Taban (`main` `47e0984cddfc726ca8e4dc7fdf423af4a1ab67a9`, `extractorRevision = 89`) üzerinde alınan çıktılar:

1. **`go run ./tools/codegraph tests-for ml.TensorValue.toDevice`** `[ölçüldü: go run ./tools/codegraph tests-for ml.TensorValue.toDevice, "no tests traced to it" ve "weak evidence (142): linked into the binary of cmd/trlang"]`:
   - Çıktı: `ml.TensorValue.toDevice [method] internal/stdlib/ml/device.go:77` (`[kanıtlı: internal/stdlib/ml/device.go:77, "func (d *TensorValue) toDevice(_ *interp.VM, args []interp.Value, _ map[string]interp.Value) (interp.Value, error) {"]`) altında `no tests traced to it` ve `<- weak evidence (142): linked into the binary of cmd/trlang`.
2. **Küçük fikstür (`tools/codegraph/testdata/mod/shape/`):**
   - `tools/codegraph/testdata/mod/shape/leaf.go` içine `type leafBox struct{}` ve `func (leafBox) step() {}`, `shape.go` içine `func useMethod(b leafBox) { apply(b.step) }`, `shape_test.go` içine `func TestUseMethod(t *testing.T) { useMethod(leafBox{}) }` eklendi.
   - `[ölçüldü: go run ./tools/codegraph tests-for --root tools/codegraph/testdata/mod --graph /tmp/mod_graph.json shape.leafBox.step, "no tests found"]`.
3. **`go run ./tools/codegraph tests-for ml.tensorLoad`** (`[kanıtlı: internal/stdlib/ml/save.go:327, "func tensorLoad(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {"]`):
   - `[ölçüldü: go run ./tools/codegraph tests-for ml.tensorLoad, "no tests traced to it" ve "weak evidence (142): linked into the binary of cmd/trlang"]`.
4. **`tokenizer` paketindeki `load` işlevinin gerçek adı ve taban `tests-for` çıktısı:**
   - `[ölçüldü: go run ./tools/codegraph query tokenizer.load, "tokenizer.load [func] internal/stdlib/tokenizer/tokenizer.go:120" ve "id: go:func:github.com/sekerahmet/TCB/internal/stdlib/tokenizer.load"]` (`[kanıtlı: internal/stdlib/tokenizer/tokenizer.go:120, "func load(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {"]`).
   - `[ölçüldü: go run ./tools/codegraph tests-for tokenizer.load, "no tests traced to it", "weak evidence (142): linked into the binary of cmd/trlang", "in other build configurations (1): internal/sysmem/memory_windows.go:11"]`.
5. **`go run ./tools/codegraph tests-for files.tempDirBuiltin`** (`[kanıtlı: internal/stdlib/files/temp.go:33, "func tempDirBuiltin(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {"]`):
   - `[ölçüldü: go run ./tools/codegraph tests-for files.tempDirBuiltin, "tests about it (6)" ve "tests through the name codes it implements (44; same package first)"]`. Gövdesinde `internal/stdlib/files/temp.go:34` satırında `interp.CallPrefix(codes.FnTempDir)` (`[kanıtlı: internal/stdlib/files/temp.go:34, "fn := interp.CallPrefix(codes.FnTempDir)"]`) bulunduğu için `EXTRACTED` `uses_name` kenarı tabanda da mevcuttur.
6. **`go run ./tools/codegraph query TR_mem_to_device` taban çıktısı:**
   - `[ölçüldü: go run ./tools/codegraph query TR_mem_to_device, "reads_name (5)", "tests (21)", "uses_name (17)", "registered or used in Go (3; implementation named on the same line): ml.TensorValue.GetProperty at internal/stdlib/ml/members.go:80, ml.memberToDevice at internal/stdlib/ml/members.go:29, ml.tensorMemberCodes at internal/stdlib/ml/members.go:37"]`.
7. **`go run ./tools/codegraph tests-for --all interp.PathArg` taban çıktısı** (`[kanıtlı: internal/lang/interp/path_arg.go:22, "func PathArg(fn any, d Value) (filePath string, ok bool, err error) {"]`):
   - `[ölçüldü: go run ./tools/codegraph tests-for --all interp.PathArg, "tests about it (1)", "tests through the name codes it implements (111; same package first)", "tests, indirect (1)", "weak evidence (5)"]`.

---

## 4. Adım 2a ve Adım 2b: Seçimler ve Gerekçeler

### 4.1 Adım 2a (`fd9e8c4` — `tools/codegraph: follow concrete method values passed as call arguments`)

- **Seçici ifadenin taşınması:** `ast.Inspect` yığınında (`stack`) tanımlayıcı (`*ast.Ident`) ziyaret edildiğinde `stack[len(stack)-1]` tanımlayıcının kendisi, bir metot değerinde (`b.step`) ise `stack[len(stack)-2]` onu saran `*ast.SelectorExpr` düğümüdür. `markCall` imzasını büyütmek yerine `use` içinde zaten geçirilen `stack`'in sondan ikinci elemanı (`stack[len(stack)-2]`) ve `noteArgNameImpl` içinde `ast.Unparen(a)` doğrudan `argCallable(fc, fn, n)` yardımcısına verildi `[kanıtlı: tools/codegraph/gocode_scan.go:321, "if callKind == viaArg && argCallable(fc, fn, stack[len(stack)-2]) {"]`.
- **Somut alıcı ve arayüz ayrımı:** `argCallable` içinde `s := fc.u.Info.Selections[sel]` alınarak `s != nil && s.Kind() == types.MethodVal && !types.IsInterface(s.Recv()) && !types.IsInterface(sig.Recv().Type())` kontrolü yapıldı `[kanıtlı: tools/codegraph/gocode_scan.go:414, "return s != nil && s.Kind() == types.MethodVal && !types.IsInterface(s.Recv()) && !types.IsInterface(sig.Recv().Type())"]`. Hem `s.Recv()` hem `sig.Recv().Type()` kontrol edildiği için:
  - Somut bir türün kendi metodu (`b.step`),
  - Somut bir türe gömülü somut alanın metodu (`eb.step` → `leafBox.step`),
  - Somut genel (generic) bir türün metodu (`gb.genStep` → `objectID` içindeki `o.Origin()` ile `genBox.genStep`'e bağlanır) zincire girer.
  - Arayüz alıcılı metot değeri (`r.run`) ve bir yapıya gömülü arayüz alanının metodu (`sig.Recv().Type()` arayüz olduğu için) zincire girmez.
- **Ad kodu katmanı sınırı:** `Via = viaArg` işareti korundu (`[kanıtlı: tools/codegraph/test_reach.go:24, "return confidence == extracted || via == viaArg || strings.HasPrefix(from, \"tr:\") && strings.HasPrefix(to, \"tr:\")"]`). `nameCallersOf` (`tools/codegraph/query_tests.go:208`) yalnız `EXTRACTED` `calls` kenarlarını izlediği için `ml.TensorValue.GetProperty`'nin kullandığı 15 `tensorMemberCodes` (`[kanıtlı: internal/stdlib/ml/members.go:37, "var tensorMemberCodes = []codes.Code{"]`) `d.toDevice`'a sızmaz.
- **Girmeyenlerin depodaki sayımı** `[ölçüldü: go/types Info.Selections taraması]`:
  - Arayüz metodu değeri (`s.Kind() == types.MethodVal && (types.IsInterface(s.Recv()) || types.IsInterface(sig.Recv().Type()))`): depoda `0` adet (test fikstürü `testdata/mod/shape/shape.go:42` içindeki `r.run` koruma testi olarak durmaktadır).
  - Metot ifadesi (`s.Kind() == types.MethodExpr`): depoda `217` adet (`57` çağrı argümanı, `160` bileşik değer tablosu; ayrıntısı Bulgu 3'te).
  - Çağrı argümanı olmayan somut alıcılı metot değeri (`s.Kind() == types.MethodVal`): depoda `43` adet (`27`'si değişkene atama `*ast.AssignStmt`, örn. `internal/devtools/debugger/dap.go:211-263`; `14`'ü anahtarlı yapı alanı `*ast.KeyValueExpr`; `2`'si `return` ifadesi `*ast.ReturnStmt`).

### 4.2 Adım 2b (`126cc02` — `tools/codegraph: link single name code to callable argument in call`)

- **Kod çözümü (`codeKey`):** Hem `codes` paketinin sabitlerini hem de `x.codeAliases` içindeki takma sabitleri (`memberToDevice = codes.MemToDevice`, `[kanıtlı: internal/stdlib/ml/members.go:29, "memberToDevice  = codes.MemToDevice"]`) çözen `codeKey(fc, id)` ayrıştırıldı (`tools/codegraph/gocode_scan.go:342-354`).
- **Çağrı ve bir düzey iç çağrı taraması (`noteArgNameImpl`):**
  - Test dosyalarındaki çağrılar (`fc.isTest`) atlanır `[kanıtlı: tools/codegraph/README.md:557, "a helper in a test file names a code but does not implement it"]`.
  - Dış çağrının argümanları (`c.Args`) ve o argümanlardan çağrı olanların (`ic, ok := ast.Unparen(a).(*ast.CallExpr)`) argümanları (`ic.Args`, yalnız bir düzey derine) taranır.
  - İç çağrı taranmadan önce `x.noteNameReads(fc, ic)` çağrılır; böylece iç çağrı `codes.NameRef(codes.X)` veya `codes.Spellings(codes.X)` ise `x.nameReads[id]` işaretlenir ve o kod uygulama sayılmaz `[kanıtlı: tools/codegraph/codes_test.go:104, "bir ad kodu codes paketinin bir işlevine argüman olarak verilmişse"]`. Buna karşılık `interp.CallPrefix(codes.FnLoadTokenizer)` (`[kanıtlı: internal/stdlib/tokenizer/tokenizer.go:114, "v.RegisterBuiltinCode(codes.FnLoadTokenizer, bind.SafeBuiltin(interp.CallPrefix(codes.FnLoadTokenizer), load))"]`) `codes` paketinde olmadığı için `uses_name` sayılır ve dış çağrıdaki `codes.FnLoadTokenizer` ile aynı kod anahtarını (`k == code`) verdiği için tek ayrı kod (`nCodes == 1`) olarak birleşir.
  - Yalnız `nCodes == 1 && nFuncs == 1` olduğunda `(fnRef, "name:"+code, fc.rel, line)` çifti `x.argImpls` listesine eklenir `[kanıtlı: tools/codegraph/gocode_scan.go:399, "x.argImpls = append(x.argImpls, argNameImpl{fn: fnRef, code: \"name:\" + code, file: fc.rel, line: x.line(fnID.Pos())})"]`.
- **2b'nin 4 riskinin ölçümü ve çözümü:**
  1. **`addEdge`'in var olan kenarın konumunu tutması ve gövde kenarının ezilmemesi:** `addEdge` (`[kanıtlı: tools/codegraph/graph.go:176, "func (b *builder) addEdge(from, to, kind, conf, file string, line int) {"]`) ilk eklenen kenarın `File` ve `Line` alanlarını tutar. Kayıt dosyası (`internal/stdlib/files/file.go:45`, `[kanıtlı: internal/stdlib/files/file.go:45, "safe(codes.FnTempDir, tempDirBuiltin)"]`) işlevin gövdesinin bulunduğu dosyadan (`internal/stdlib/files/temp.go:34`) önce tarandığı için, kenar tarama sırasında hemen eklenseydi `tempDirBuiltin -> name:TR_fn_temp_dir` kenarının konumu `temp.go:34` yerine `file.go:45` olurdu. Bunu önlemek için `argImpls` bütün birimler (`for _, u := range m.Units`) tarandıktan sonra `extractGo` içinde boşaltıldı ve yalnız `x.b.edges[edgeKey{p.fn, p.code, edgeUsesName}] == nil` ise `INFERRED` kenar eklendi `[kanıtlı: tools/codegraph/gocode.go:152, "if x.b.edges[edgeKey{p.fn, p.code, edgeUsesName}] == nil {"]`, emsal: `[kanıtlı: tools/codegraph/other_os.go:131, "b.addEdge(owner, \"name:\"+key, edgeUsesName, inferred, rel, line)"]`. Aynı çiftin dış ve iç çağrıdan iki kez gelmesi de (`tokenizer.go:114`) ilk eklemeden sonra `edges[...] != nil` olduğu için tek kenar üretir.
  2. **Kenarın dosya ve satırının kayıt yerinde olması:** Gövdesinde `uses_name` kenarı olmayan `69` işlevde kenarın konumu kayıt çağrısında işlev tanımlayıcısının satırıdır (örn. `ml.TensorValue.toDevice` için `internal/stdlib/ml/members.go:81`, `ml.tensorLoad` için `internal/stdlib/ml/builtins.go:60` `[kanıtlı: internal/stdlib/ml/builtins.go:60, "wrap(v, codes.FnLoadTensors, newBuiltin(tensorLoad))"]`).
  3. **`query <kod>` çıktısında "registered or used in Go" (`implementationsAt`):** Bulgu 4'te ölçüldüğü üzere `query TR_mem_to_device` çıktısında `ml.TensorValue.toDevice at internal/stdlib/ml/members.go:81` satırı listelenir.
  4. **Kodun "kendi işlevleri" kümesinin büyümesi ve planlama sınavı:** `go test -short ./tools/codegraph/ -run 'TestPlanningQuestionsAreAnswered|TestEveryBuiltinNameHasADirectTestOrAReason|TestDottedMemberSpellingsGolden'` koşuldu; hiçbir planlama sınavı sorusunun veya altın dosyanın cevabı değişmedi.

---

## 5. Adım 3: Büyüme Ölçümleri ve Tablolar

### 5.1 Hedef İşlevlerin Önce ve Sonra `tests-for` Grupları

| Hedef | Grup | Önce (`main` `47e0984c`) | Sonra (`126cc02`) |
| :--- | :--- | :--- | :--- |
| `ml.TensorValue.toDevice` | `tests about it` (doğrudan ad/ileti) | `0` | `3` (`ml.TestDeviceMismatchMessageNamesTheTR`, `ml.TestIndexAssignAcrossDevicesIsAnError`, `tensor_test: 15.39`) |
| `ml.TensorValue.toDevice` | `tests through the name codes it implements` | `0` | `18` (`via cihaza (Tensör)`) |
| `ml.TensorValue.toDevice` | `tests, indirect` (`<=4` adım) | `0` | `0` |
| `ml.TensorValue.toDevice` | `weak evidence` | `142` (`linked into the binary of cmd/trlang`) | `3` (`>4` adım, `via ml.TensorValue.GetProperty`: `interp.TestCompilerDynamicEval` 7 adım, `interp.TestValueHooks` 7 adım, `interp.FuzzCompileSource` 8 adım) |
| `ml.tensorLoad` | `tests about it` (doğrudan ad/ileti) | `0` | `4` (`e2e.TestTrtFilesBetweenPythonAndTR`, `ml.TestSafetensorsLoad`, `tensor_test: 15.46`, `tensor_test: 15.47`) |
| `ml.tensorLoad` | `tests through the name codes it implements` | `0` | `29` (`via tensör_yükle (kaydetme ve yükleme)`) |
| `ml.tensorLoad` | `weak evidence` | `142` (`linked into the binary of cmd/trlang`) | `0` |
| `tokenizer.load` | `tests about it` (doğrudan ad/ileti) | `0` | `5` (`tokenizer_test.TestLoadErrors`, `TestLoadRejectsAnAddedTokenOnATakenID`, `TestLoadTokenizerFromGGUF`, `veri_parcalayici_test: 16.1`, `16.9`) |
| `tokenizer.load` | `tests through the name codes it implements` | `0` | `21` (`via parçalayıcı_yükle (parçalayıcı)`) |
| `tokenizer.load` | `weak evidence` / `other build configurations` | `142` (ikili) / `1` (`memory_windows.go:11`) | `0` (zayıf yok) / `1` (`memory_windows.go:11`) |
| `files.tempDirBuiltin` | `tests about it` / `tests through the name codes` | `6` / `44` (`via geçici_klasör (dosya)`) | `6` / `44` (`via geçici_klasör (dosya)`, `temp.go:34` `EXTRACTED` kenarı korundu) |

### 5.2 `query TR_mem_to_device` Önce ve Sonra

| Bölüm | Önce (`main` `47e0984c`) | Sonra (`126cc02`) |
| :--- | :--- | :--- |
| `reads_name` | `5` (`ml.TensorValue.toDevice` at `device.go:78` dahil) | `5` (değişmedi) |
| `tests` | `21` | `21` (değişmedi) |
| `uses_name` | `17` | `18` (`+1`: `ml.TensorValue.toDevice [method] at internal/stdlib/ml/members.go:81 [INFERRED]`) |
| `registered or used in Go` | `3` (`ml.TensorValue.GetProperty at members.go:80`, `ml.memberToDevice at members.go:29`, `ml.tensorMemberCodes at members.go:37`) | `4` (`+1`: `ml.TensorValue.toDevice at internal/stdlib/ml/members.go:81`) |

### 5.3 `interp.PathArg` ve `gaps.md:52`'deki 8 Ad Kodunun Durumu

- **`tests-for --all interp.PathArg` özeti** `[ölçüldü: go run ./tools/codegraph tests-for --all interp.PathArg]`:
  - Önce (`main` `47e0984c`): `tests about it (1)`, `tests through the name codes it implements (111; same package first)` (`10` ad kodu, `<=4` adım), `tests, indirect (1)`, `weak evidence (5)` (`TR_fn_copy` ve `TR_fn_move`, `5` adım).
  - Sonra (`126cc02`): `tests about it (1)`, `tests through the name codes it implements (170; same package first)` (`14` ad kodu, `<=4` adım; `+59` test), `tests, indirect (1)`, `weak evidence (5)`.
- **`gaps.md:52`'de anılan 8 kodun tek tek durumu:**
  1. **`TR_fn_load_tokenizer` (`parçalayıcı_yükle`) — Döndü (`<=4` adım):** `tokenizer.load` (`internal/stdlib/tokenizer/tokenizer.go:128`) doğrudan `interp.PathArg` çağırır (`PathArg <- tokenizer.load -> TR_fn_load_tokenizer`, `3` adım). Gerçekten dosya yolu alır.
  2. **`TR_fn_load_tensors` (`tensör_yükle`) — Döndü (`<=4` adım):** `ml.tensorLoad` (`internal/stdlib/ml/save.go:335`) `ml.yolArg` (`save.go:132`) üzerinden `interp.PathArg` çağırır (`3` adım). Gerçekten dosya yolu alır.
  3. **`TR_fn_save_tensors` (`tensör_kaydet`) — Döndü (`<=4` adım):** `ml.tensorSave` (`internal/stdlib/ml/save.go:155`) `ml.yolArg` (`save.go:132`) üzerinden `interp.PathArg` çağırır (`3` adım). Gerçekten dosya yolu alır.
  4. **`TR_fn_load_metadata` (`üstveri_yükle`) — Döndü (`<=4` adım):** `ml.metadataLoad` (`internal/stdlib/ml/load_metadata.go:35`) `ml.yolArg` (`save.go:132`) üzerinden `interp.PathArg` çağırır (`3` adım). Gerçekten dosya yolu alır.
  5. **`TR_fn_copy` (`kopyala`) — Tabanda zaten gövdeden bağlıydı (`5` adım, `weak evidence`):** `files.copyBuiltin` (`internal/stdlib/files/copy.go:20`) `codes.FnCopy` sabitini gövdesinde `EXTRACTED` olarak kullanır ve `PathArg <- readPath <- doMove <- copyBuiltin` (`5` adım) zinciriyle hem tabanda hem sonrasında `weak evidence (5)` altındadır (uzak 7 `75bef26f`'de `PathArg <- pathBuiltin <- files.Register` yanlış kısayolu yüzünden `3` adımda görünmüştü).
  6. **`TR_fn_move` (`taşı`) — Tabanda zaten gövdeden bağlıydı (`5` adım, `weak evidence`):** `files.moveBuiltin` (`internal/stdlib/files/copy.go:25`) `codes.FnMove` sabitini gövdesinde `EXTRACTED` olarak kullanır ve `PathArg <- readPath <- doMove <- moveBuiltin` (`5` adım) zinciriyle `weak evidence (5)` altındadır.
  7. **`TR_fn_make_dir` (`klasör_oluştur`) — Tabanda zaten gövdeden bağlıydı (`3` adım):** `files.makeDirBuiltin` (`internal/stdlib/files/dir.go:15`) `codes.FnMakeDir` sabitini gövdesinde `EXTRACTED` olarak kullanır (`PathArg <- readPath <- makeDirBuiltin`, `3` adım); testlerinin tamamı aynı zamanda `dosya_yaz`, `klasörü_listele` veya `sil` kullandığı için tekilleştirmede o kodların altında listelenir.
  8. **`TR_fn_temp_dir` (`geçici_klasör`) — Dönmedi ve dönmemesi doğrudur:** `files.tempDirBuiltin` (`internal/stdlib/files/temp.go:33-49`) sıfır argüman alır ve `os.MkdirTemp("", "tr-*")` çağırır; `interp.PathArg` veya `readPath` çağırmaz. `75bef26f`'de yalnızca `PathArg <- pathBuiltin <- files.Register -> TR_fn_temp_dir` yanlış kısayolu üzerinden gelmişti.

### 5.4 Uzak 7'nin 19 Örneklem Dosyasında (`awk 'NR%40==1'`) `tests-for --all` Sayıları

Komut: `git ls-files 'internal/*.go' 'cmd/*.go' 'tools/*.go' | grep -v _test.go | sort | awk 'NR%40==1'` `[ölçüldü: /tmp/before_graph.json ve /tmp/after_graph.json ile tests-for --all]`:

| # | Dosya | Zayıf Olmayan (Önce → Sonra) | Zayıf Kanıt (Önce → Sonra) | Toplam (Önce → Sonra) |
| :--- | :--- | :--- | :--- | :--- |
| 1 | `cmd/trlang/arguments.go` | `0 → 0` (`+0`) | `0 → 0` (`+0`) | `0 → 0` (`+0`) |
| 2 | `internal/compute/gpu/backend_multi.go` | `11 → 11` (`+0`) | `11 → 11` (`+0`) | `22 → 22` (`+0`) |
| 3 | `internal/compute/gpu/profile.go` | `104 → 104` (`+0`) | `223 → 233` (`+10`) | `327 → 337` (`+10`) |
| 4 | `internal/compute/tensor/cpu_gemv16.go` | `5 → 5` (`+0`) | `307 → 307` (`+0`) | `312 → 312` (`+0`) |
| 5 | `internal/compute/tensor/host_block_windows.go` | `17 → 17` (`+0`) | `560 → 569` (`+9`) | `577 → 586` (`+9`) |
| 6 | `internal/compute/tokenize/model_other.go` | `37 → 41` (`+4`) | `12 → 32` (`+20`) | `49 → 73` (`+24`) |
| 7 | `internal/devtools/codeindex/library_codes.go` | `82 → 82` (`+0`) | `171 → 172` (`+1`) | `253 → 254` (`+1`) |
| 8 | `internal/devtools/profiler/live_other.go` | `1 → 1` (`+0`) | `0 → 0` (`+0`) | `1 → 1` (`+0`) |
| 9 | `internal/lang/checker/module_rules.go` | `213 → 217` (`+4`) | `340 → 339` (`-1`) | `553 → 556` (`+3`) |
| 10 | `internal/lang/interp/bytes_value.go` | `1431 → 1440` (`+9`) | `135 → 134` (`-1`) | `1566 → 1574` (`+8`) |
| 11 | `internal/lang/interp/path_arg.go` | `113 → 172` (`+59`) | `534 → 515` (`-19`) | `647 → 687` (`+40`) |
| 12 | `internal/lang/lexer/unicode_safety.go` | `592 → 596` (`+4`) | `880 → 879` (`-1`) | `1472 → 1475` (`+3`) |
| 13 | `internal/layoutrules/exceptions.go` | `5 → 5` (`+0`) | `22 → 22` (`+0`) | `27 → 27` (`+0`) |
| 14 | `internal/stdlib/logging/terminal_unix.go` | `4 → 4` (`+0`) | `0 → 0` (`+0`) | `4 → 4` (`+0`) |
| 15 | `internal/stdlib/ml/print.go` | `211 → 211` (`+0`) | `834 → 834` (`+0`) | `1045 → 1045` (`+0`) |
| 16 | `internal/testkit/known_list.go` | `2 → 2` (`+0`) | `0 → 0` (`+0`) | `2 → 2` (`+0`) |
| 17 | `tools/codegraph/history.go` | `7 → 7` (`+0`) | `77 → 77` (`+0`) | `84 → 84` (`+0`) |
| 18 | `tools/codegraph/test_paths_param.go` | `2 → 2` (`+0`) | `173 → 174` (`+1`) | `175 → 176` (`+1`) |
| 19 | `tools/trlang_migrate/rules.go` | `0 → 0` (`+0`) | `0 → 0` (`+0`) | `0 → 0` (`+0`) |

- **En çok büyüyen 3 dosyanın sebebi ve örnek zincirleri:**
  1. **`internal/lang/interp/path_arg.go` (`+59` zayıf olmayan, `+40` toplam):** Adım 2b ile `tokenizer.load -> TR_fn_load_tokenizer`, `ml.tensorLoad -> TR_fn_load_tensors`, `ml.tensorSave -> TR_fn_save_tensors` ve `ml.metadataLoad -> TR_fn_load_metadata` `INFERRED` `uses_name` kenarları eklendi. Bu 4 işlev `interp.PathArg`'ı `1–2` adımda çağırdığı için bu 4 ad kodunun testleri (`59` test, örn. `ml.TestTensorSaveLoad` `via tensör_yükle, 2 steps`, `tokenizer_test.TestLoadTokenizerFromGGUF` `via parçalayıcı_yükle, 1 steps`) doğrudan `tests through the name codes it implements` grubuna girdi (`19` tanesi önceden `>4` adımda zayıf kanıtta olduğu için zayıf kanıt `534 → 515` düştü).
  2. **`internal/compute/tokenize/model_other.go` (`+4` zayıf olmayan, `+20` zayıf, `+24` toplam):** Bu dosyadaki `newWordPiece`, `newWordLevel` ve `newUnigram` kurucuları `init()` (`model_other.go:15-19`) içinde `RegisterModel` ile kaydedilir ve `tokenize.Load` üzerinden `tokenizer.load` (`internal/stdlib/tokenizer/tokenizer.go:120`) tarafından çağrılır. Adım 2a ile `tokenizer.Register` (`tokenizer.go:114`) içindeki `load` argümanı ve Adım 2b ile `tokenizer.load -> TR_fn_load_tokenizer` kenarı eklendiğinde:
     - `TR_fn_load_tokenizer` kullanan `tokenizer_test.TestEncodeDecodeSameAsGo` (`3 steps`), `tokenizer_test.TestSpecialTokensAndPair` (`4 steps`), `veri_parcalayici_test: 16.1` (`4 steps`) gibi `4` test `tests, indirect` (`<=4` adım) grubuna,
     - `tokenizer_test.TestLoadErrors`, `TestLoadTokenizerFromGGUF` (`5 steps`) gibi `20` test ise `weak evidence` (`>4` adım) grubuna eklendi.
  3. **`internal/compute/gpu/profile.go` (`+10` zayıf, `+10` toplam) ve `internal/lang/interp/bytes_value.go` (`+9` zayıf olmayan, `+8` toplam):**
     - `internal/compute/gpu/profile.go` içindeki `(g *backend) measureStart()`, `hostStart()`, `hostEnd()`, `equalBegin()` (`profile.go:91,236,242,257`) metotları GPU arka uç metotları (`internal/compute/gpu/gpu.go:212,216,250`, `memory.go:320,338,351,541`) tarafından çağrılır; Adım 2a ve 2b ile `ml` paketindeki tensör metot ve yerleşikleri (`ml.TensorValue.toDevice`, `ml.TensorValue.toBytes`, `ml.tensorLoad` vb.) testlerine bağlandığında bu zincirler üzerinden `5–8` adımda `10` ek test `weak evidence` grubuna girdi.
     - `internal/lang/interp/bytes_value.go` içindeki `interp.NewBytes` (`bytes_value.go:25`) işlevini `ml.TensorValue.toBytes` (`internal/stdlib/ml/builtin_bytes.go:150`, `members.go:93` içinde `d.method(memberToBytes, d.toBytes)`) ve `ml.intBytesBuiltin` / `ml.floatBytesBuiltin` (`builtin_bytes.go:38-39,265,268,297`) doğrudan çağırır; Adım 2a ve 2b ile `TR_mem_to_bytes`, `TR_fn_int_bytes` ve `TR_fn_float_bytes` ad kodları bu işlevlere bağlandığı için `bayt_ve_sayi_test` ve ilgili testlerden `9` yeni test `<=4` adımda zayıf olmayan testlere katıldı.

### 5.5 Uçta İzlenmeyen Go `INFERRED` `calls` Kenarları ve 2b Sayıları

- **Uçta izlenmeyen Go `INFERRED` `calls` kenarları (`Via != "arg"`):**
  - Önce (`main` `47e0984c`): Toplam `443` kenar (`223` ayrı hedef), bunların `311`'i metot hedefidir (`kind == "method"`). Test dışı dosyalarda `193` kenar (`161` ayrı hedef), bunların `132`'si metot hedefidir.
  - Sonra (`126cc02`): Toplam `357` kenar (`169` ayrı hedef), bunların `225`'i metot hedefidir (`-86` `INFERRED` kenar ve `+1` `EXTRACTED` kenar artık `Via = "arg"` taşıyor). Test dışı dosyalarda `118` kenar (`57`'si metot hedefidir; kalan `57` metot hedefinin `34`'ü `MethodExpr`, `23`'ü değişkene atama veya anahtarlı/konumlu yapı alanıdır).
- **Adım 2b `uses_name` kenarları ve çoklu kod/işlev sayıları:**
  - Eşleşen çağrı ifadesi (`nCodes == 1 && nFuncs == 1`, test dışı): `123` çağrı ifadesi, `109` tekil `(işlev, kod)` çifti (`14` çift aynı satırdaki dış ve iç çağrıdan tekrar etmiştir: `files/file.go:38,40-45`, `tokenizer/tokenizer.go:114`, `ml/builtins.go:76`, `ml/builtin_extreme.go:23-24`, `process/process.go:84-86`).
  - `109` tekil çiftin `40`'ında işlevin gövdesinde zaten `EXTRACTED` `uses_name` kenarı bulunduğu için korundu; **`69`** yeni `INFERRED` `uses_name` kenarı eklendi (`uses_name`: `9038 → 9107`).
  - İki ayrı kod (`nCodes >= 2`) veya iki ayrı işlev (`nFuncs >= 2`) nedeniyle bağlanmayan çağrılar `[ölçüldü: noteArgNameImpl sayımı]`:
    - Test dışı dosyalarda `nCodes >= 2 && nFuncs >= 1` olan çağrı: **`0`** (`nCodes >= 2 && nFuncs == 0` olan `59` çağrı vardır; tamamı `interp.ArgTypeError(v, codes.FnX, ..., codes.TypeY, ...)` gibi işlev argümanı taşımayan hata çağrılarıdır).
    - Test dışı dosyalarda `nCodes >= 1 && nFuncs >= 2` olan çağrı (`argCallable` tanımıyla): **`0`** (`nCodes == 0 && nFuncs >= 2` olan `3` çağrı vardır: `internal/stdlib/ml/builtin_transpose.go:63` ve `internal/stdlib/ml/quant_matmul.go:34,70`). Not: `internal/stdlib/ml/builtins.go:96-98,101-104` ve `internal/stdlib/ml/builtin_math.go:31-41` içindeki `18` `unit(v, codes.FnX, (*tensor.Tensor).X, x64)` çağrısı bir paket işlevi (`x64`) ve bir metot ifadesi (`(*tensor.Tensor).X`, `MethodExpr`) içerir.

### 5.6 Graf Kurulum Süresi (3 Sıcak Koşunun Ortancası)

- **Önce (`main` `47e0984c`):** `go run ./tools/codegraph build --out /tmp/before_graph.json` → `9.6 s`, `9.6 s`, `10.1 s` → ortanca **`9.6 s`**.
- **Sonra (`126cc02`):** `go run ./tools/codegraph build --out /tmp/after_graph.json` → `9.1 s`, `9.6 s`, `9.2 s` → ortanca **`9.2 s`**.

---

## 6. Değişen Altın ve Planlama Sınavı Satırları

- **`tools/codegraph/testdata/extractor_digest.txt`:** Çıkarıcı revizyonu Adım 2a'da `89 → 90`, Adım 2b'de `90 → 91` olarak artırıldı ve `tools/codegraph/testdata/mod` fikstür özetini tutan `extractor_digest.txt` güncellendi (`revision 91`, `digest 60f59806cc564684`).
- **`tools/codegraph/testdata/member_spellings.txt` ve `TestPlanningQuestionsAreAnswered`:** Hiçbir satır değişmedi (`-update` kullanılmadı); `TestPlanningQuestionsAreAnswered`, `TestDottedMemberSpellingsGolden`, `TestReadmeGraphSizeIsCurrent` ve `TestEveryBuiltinNameHasADirectTestOrAReason` olduğu gibi geçti.

---

## 7. Bitti Ölçütü Komutları ve Son Satırları

1. **`gofmt -l tools`**
   - Son satır: *(boş çıktı, çıkış kodu 0)* `[ölçüldü: gofmt -l tools, boş]`
2. **`go test ./tools/codegraph/...`**
   - Son satırlar `[ölçüldü: go test ./tools/codegraph/..., yalnız TestEveryTrackedDataFileHasATestOrAReason UZAK8_DURUM.md yüzünden düştü]`:
     ```
     --- FAIL: TestEveryTrackedDataFileHasATestOrAReason (0.04s)
         data_files_test.go:108: 1 tracked data file(s) have no test reading them in the code graph and no reason in dataFileCoverageExceptions:
               UZAK8_DURUM.md
     FAIL
     FAIL	github.com/sekerahmet/TCB/tools/codegraph	336.526s
     ```
   - Açıklama: Yönergede belirtildiği üzere `TestEveryTrackedDataFileHasATestOrAReason` dalın kökündeki `UZAK8_DURUM.md` ve `UZAK8_RAPORU.md` dosyalarını görür; diğer bütün testler geçmiştir.
3. **`go run ./tools/codegraph tests-for ml.TensorValue.toDevice`**
   - Son satır: `    interp.FuzzCompileSource [func] internal/lang/interp/compiler_fuzz_test.go:22  8 steps via ml.TensorValue.GetProperty  [INFERRED]` (`tests about it (3)`, `tests through the name codes it implements (18; same package first)`, `weak evidence (3)`).
4. **`go run ./tools/codegraph tests-for ml.tensorLoad`**
   - Son satır: `    ... 17 more (--all lists them)` (`tests about it (4)`, `tests through the name codes it implements (29; same package first)`).
5. **`go run ./tools/codegraph tests-for --all interp.PathArg`**
   - Son satır: `    files.TestOverwriteDirWithReadOnlyFile [func] internal/stdlib/files/copy_test.go:159  via TR_fn_copy, 5 steps  [INFERRED]` (`tests about it (1)`, `tests through the name codes it implements (170; same package first)`, `tests, indirect (1)`, `weak evidence (5)`).
6. **`go run ./tools/codegraph query TR_mem_to_device`**
   - Son satır: `  documented: ... (TR_mem_to_device, cihaza) [heading] docs/codes.md:7001` (`reads_name (5)`, `tests (21)`, `uses_name (18)`, `registered or used in Go (4; implementation named on the same line)` içinde `ml.TensorValue.toDevice at internal/stdlib/ml/members.go:81`).
7. **`git diff --stat main...HEAD`**
   - Son satır: ` 13 files changed, 608 insertions(+), 45 deletions(-)` — yalnız `tools/codegraph/` altındaki dosyalar ile kökteki `UZAK8_DURUM.md` ve `UZAK8_RAPORU.md` değişmiştir (`gaps.md`'ye dokunulmamıştır).

---

## 8. Ortam

- **Go Araç Zinciri (`GOTOOLCHAIN=local`):** `go.mod` `toolchain go1.26.10` istediği ve yerel ortamda `go1.26.2` kurulu olduğu için `go env -w GOTOOLCHAIN=local` ayarlandı.
- **Sığ Klon (`shallow clone`):** Uzak ortamda depo `--depth` ile sığ klonlandığı için (`git rev-parse --is-shallow-repository` → `true`), `TestPlanningQuestionsAreAnswered` içindeki `git log -S` geçmişine dayanan 7 soru (`plan_test.go`) testin kendi sığ klon kontrolüyle atlandı; kalan 43 planlama sorusu tam koştu ve geçti.
- **Sanal Makine Duraklatma/Yeniden Başlatma:** `2026-11-08 17:10:37 UTC+3` saatinde uzak sanal makine duraklatılıp yeniden başlatıldı; tüm commit'ler `origin/gemini/uzak-8` dalına önceden push edilmiş olduğu için veri kaybı yaşanmadı.

---

## 9. Codegraph

- İşin konusu olan ve Adım 2a / 2b ile düzeltilen iki kusur (`x.M` metot değerinin argüman kenarına girmemesi ve yanındaki ad kodunun argüman işleve bağlanmaması) dışında `tools/codegraph` sorgularında yanlış ya da yanıltıcı bir cevap görülmedi.
- Küçük CLI notu: `build` alt komutu graf yolu için `--graph` değil `--out` bayrağını kullanmakta, `similar` alt komutu ise `--graph` bayrağı almamaktadır (`tests-for` ve `query` `--graph` bayrağını kabul etmektedir).

---

## 10. Emin Olmadıklarım ve Bakmadıklarım

- **Emin olmadıklarım:** `unit(v, codes.FnRelu, (*tensor.Tensor).Relu, relu64)` (`internal/stdlib/ml/builtins.go:96-104`) biçimindeki 18 çağrıda `(*tensor.Tensor).Relu` bir metot ifadesi (`types.MethodExpr`) olduğu için Adım 2a'da `argCallable` dışında kalmaktadır; bu nedenle Adım 2b bu çağrılarda tek `argCallable` olarak skaler türev işlevi `relu64`'ü görür ve ad kodunu ona bağlar. `MethodExpr` de `argCallable`'a alındığında (Bulgu 3'teki öneri) bu 18 çağrıda `nFuncs == 2` olacağı için ad kodunun yalnızca iki işlevden birine bağlanması önlenir ya da her ikisine bağlama kuralı ayrıca tasarlanabilir.
- **Bakmadıklarım:** Yönergede kapsam dışı bırakıldığı üzere `keyedImplementations`'ın `uses_name` kenarına çevrilmesine, konumlu tablo (`{codes.TypeBytes, bytesCall}`) ve değişkenle aktarım (`open = dequantQ6K`) durumlarına ve `tools/codegraph/gaps.md` dosyasına dokunulmamıştır.

---

## 11. Kendi Kontrolüm

- Rapordaki `29` `[kanıtlı: dosya:satır, "..."]` birebir alıntısının tamamı ve metin içinde anılan tüm `dosya:satır` başvuruları kaynak dosyalardan `sed -n` ve `grep -F` ile yeniden açılarak karakter karakter karşılaştırıldı.
- İlk taslakta bellekten yazılmış olan `9` işlev/değişken imzası alıntısı (`members.go:29,37,81`, `device.go:77`, `save.go:327`, `tokenizer.go:120`, `temp.go:33,34`, `path_arg.go:22`), `1` README satır numarası (`tools/codegraph/README.md:56` → `:557`) ve Bölüm 5.3 / 5.4 içindeki satır numaraları (`save.go:155,335`, `load_metadata.go:35`, `copy.go:20,25`, `dir.go:15`, `query_tests.go:208`, `query_names.go:61`, `bytes_value.go:25`) kaynak dosyalardaki gerçek satırlarla birebir değiştirildi.
- Düzeltme sonrasında koşulan otomatik `grep -F` ve satır numarası denetiminde `29/29` alıntının tamamı `0` hatayla doğrulandı; ayrıca `UZAK8_RAPORU.md` ve `UZAK8_DURUM.md` içinde `.TR_` noktalı üye yazımının bulunmadığı (`0` eşleşme) teyit edildi.

Kendi incelemem bitti.
