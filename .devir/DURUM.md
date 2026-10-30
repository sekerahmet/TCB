# SÜRÜYOR

# DURUM — Faz 1, 4a1, ilk dilim

- Dal: `claude/hopeful-brahmagupta-miw41q`, taban `main` a949c3a.
- Başlama: 2026-10-30 07:37 UTC.

## Adım 1: Go testlerindeki TR programları

- **Sayım (önce):** 470 tipsiz yer (0.9'un yöntemi: `testkit.GoStringLiterals` + ayrıştırıcı sayacı; Go `*_test.go` sabitleri `cmd`, `internal`, `tools` altında; tırnaklı sabitin bütün satırları sabitin Go satırında sayılır). Paket paket: `codeindex` 94, `interp` 79, `checker` 70, `ml` 67, `parser` 50, `cmd/tr/e2e` 28, `gpuprobe` 14, `source` 13, `debugger` 11, `.tr` deneme projeleri (`tools/codegraph/testdata/mod`, `internal/devtools/codeindex/testdata/proj`) 18, `checkpoint` 6, `formatter` 6, `tools/codegraph` 5, `profiler` 4, `testrunner` 3, `process` 1, `files` 1. Eşik (600) aşılmadı.
- **Sonra:** 58 yer kaldı, hepsi izin listesinde gerekçeli (`internal/testdata/typed_code_known.txt`, son başlık); 412 yer tiplendi.
- `TestTRCodeIsTyped` Go testlerinin metin sabitlerini ve araçların deneme projelerini de sayıyor (`typedGoRoots`, `testkit.GoStringLiteralsRaw`: sabitin raw olup olmadığı satır hesabı için). `skippedTypedDirs` kalktı.
- **Sayılamayanlar** (`fmt.Sprintf`/`+` ile kurulan, sayacın göremediği parametre listeleri): `internal/devtools/codeindex/cache_test.go:198`, `:200`, `:210` (`TR_func f%d_0(p)`: ad biçim dizesinde, ayrışmaz); `internal/lang/checker/checker_test.go:372` (`TR_func g() { TR_return " + invalid + " }`), `:430` (ad `c.input`'tan); `internal/devtools/codeindex/bind_test.go:89-94` (konum aramak için kesit, program değil). Öbür birleştirmeler (`command_line_reach_test.go:46-54`, `custom_grad_test.go:324,343`, `compiler_test.go:106`, `lexer/identity_test.go:16`, `parser/struct_test.go:125`, `behavior_value_text_test.go:70`, `snippet_test.go:125`) parça parça sayıldı.

### İkinci dilime giden liste (dokunulmadı)

- **Sahiplik testlerinin Go değerleri:** `internal/lang/interp/behavior_ownership_test.go` 21 yer (204, 221, 222, 239, 347, 370, 372, 407, 427, 475, 508, 554, 570, 586), `internal/lang/interp/hooks_test.go:391`, `:405` (Go değeri `w0`).
- **Tipsizliği sınayan:** `checker_test.go:282,283,308,309,329,333,340`, `unpack_test.go:75`, `parser/struct_test.go:97,98` (kurucunun değerli dönüşü H0282), `parser/types_test.go:142` (bozuk tip yazımı), `interp/shadow_test.go:19`, codeindex'in çıkarım ve ad tahmini testleri (`bind_test.go:32,173,174`, `infer_test.go:16,20,25,83`, `reader_test.go:60`, deneme projelerinin `geo.tr:20`'si).
- **Birden çok tip (genel T ya da birleşim, 4c):** `behavior_value_text_test.go:70,78`, `hooks_test.go:77` (N(1) == N(1.0)), `ml/model_tools_test.go:260` (liste ya da sözlük), `debugger/dap_test.go:182` (karışık değerli sözlük).
- **Testler (1. adımın commit'inde):** `TestTRCodeIsTyped` 0; `go test` testkit, `internal/lang/...`, `internal/devtools/...`, ml, checkpoint, files, process, gpuprobe (PTX benzetimiyle), `tools/codegraph` 0; `cmd/tr/e2e` alt ajanın kopyasında 0; `go vet` 0; `tr format --check lib tests tools` tabanla aynı (yalnız bilerek bozuk fikstürler); `dups` 0 aday.

### Bulunan yanlış (1. adım)

- **Karışık tipli listeler (3.6 liste tek tiplidir) demete çevrildi:** `gpuprobe/behavior_gpu_checkpoint_test.go` (`eğit` `[model, op, izlenen, kn]`, `sürdür`), `gpuprobe/behavior_gpu_training_test.go` (`eğit` `[kayıplar, son_cihaz]`), `ml/dtype_test.go` (`hesap`); okumalar `a[0]` yerine `a.0`.
- **Ondalık tensör indeks ve başlangıç olarak veriliyordu** (15.23, 15.8: `TR_Tensor[TR_Int]`): `ml/builtin_replay_test.go` `replayStep`, `TestReplayArgumentsKeepDirectCallMeaning` tam sayı tensöre çevrildi. Aynı yanlış dokunulmadan duruyor: `TestWriteSliceSameRulesInsideAndOutsideRecording` (`TR_start: TR_Tensor(16.0)`, `TR_Tensor(0.0)`).
- **`TR_is_finite`:** dil tanımı (13.7, 15.16a) `TR_Tensor[TR_Bool]` diyor, gerçekleme ondalık tensör veriyor (test `[0.0, 0.0]` bekliyor); `behavior_grad_clip_version_test.go:17` `sına` dil tanımına göre `TR_List[TR_Bool]` yazıldı. Gerçekleme ya da tanım düzelmeli.
- **`internal/devtools/codeindex/params.go` `defaultText`:** varsayılan değerin metnini adın bitişinden okuyor; tipli ve varsayılanlı parametrenin imzası bozuk çıkıyor (`kv_baş: TR_Int? = TR_none` → `"kv_baş = : TR_Int? = TR_none"`). Bu yüzden üç yer (`bind_test.go:18`, iki deneme projesinin `geo.tr:8`'i) tipsiz kaldı. Ayrıca codeindex fonksiyon imzasında parametre tipini ve dönüşü göstermiyor; parametre tipini alıcı çıkarımında kullanmıyor (`çevre_hesapla(k: Kare)` içinde `k.kenar` ad tahminiyle bağlanıyor).
- **`process/command_line_test.go`:** `TR_command_line`'a sözlük veriliyor; 19.2 yapı diyor.
- **`debugger/dap_test.go:182`:** alt ajan karışık değerli sözlüğü `TR_Dict[TR_String, TR_JSON]` ve `TR_Int(k["maas"])` ile tiplemişti (tip uydurma); geri alındı, izin listesinde.
