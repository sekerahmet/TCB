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

## Adım 2: 1.1'in modül kuralı

- **Önce (taban 44cf853):** `tr test tests` 352/352, `tr test lib` 0. Kurala aykırı yer 6 (eşik 15 aşılmadı): `tests/kontrol_noktasi_test.tr:11,13` (`TABAN`, `GÜNLÜKLER = TR_temp_dir()`), `tests/surec_ve_kayit_test.tr:8,16` (`KOŞU = TR_temp_dir()`, `AYAR = TR_command_line(...)`), `examples/tr/17_testler_test.tr:35` (en dışta `TR_print`), `tests/yardimci_testli.tr` (modülde `TR_test`: 12.1'e göre serbest, aykırı değil). `lib/`'de aykırı yer yok; yükleme sırası değişmedi.
- **Davranış değişikliği (ayrı commit):** `TR_temp_dir()` her çağrıda yeni klasör verir (18.14): her test kendi klasörünü kuruyor (`koşu(ad)` yeni geçici klasörün altında, `temizle()` kalktı: geçici klasörler program bitince silinir; günlük koşu klasörünün yanında). `TR_command_line` programda bir kez çağrılabilir (19.2): `ayar()` tek çağrıyı `ayar_sonucu`'nda saklıyor; "bir kez" testi önce kendisi çağırıyor (eskiden önceki testin en dıştaki çağrısına dayanıyordu). Örnek 17'nin en dıştaki `TR_print`'i yorum oldu. **Sonra:** `tr test tests` 352/352, `tr test lib` 0.
- **Kural (denetleyici):** `internal/lang/checker/module_rules.go` (en dıştaki deyim, H1107) ve `const_values.go` (sabit ifade, H1108; 1.1'in listesi: sayı, sabit yerleştirmeli metin, mantık, `TR_none`, sabit ve işlemleri, aralık, indeks, sabit öğeli liste/sözlük/demet, kurucusuz yapı ve durumlu tip değeri, `TR_Bytes`, fonksiyon adı, en dıştaki `TR_var`'ı okumayan fonksiyon ifadesi, sayı yerleşikleri (`TR_sqrt` ... `TR_clamp`, `TR_min`/`TR_max`), `TR_fuse`/`TR_replay`/`TR_custom_grad`). Dosyanın türü `checker.SetFile`; `source.Prepare` modülü `Module`'den, test dosyasını `TR_test` içermesinden (`checker.HasTests`, 12.1) bilir. Hata modülün satırında; aktaran dosyayı `ModuleError`'ın başlığı (H2002/H2003) anar. İleti dosyadaki metinden deyimi `TR_func kur() { ... }`'e alan, ilk değeri `TR_func ad() -> T { TR_return ... }` yapan yazımı verir; `TestModuleRuleSuggestionRuns` önerileri modüle kopyalayıp koşuyor.
- **Testler:** `TestModuleTopLevelHoldsOnlyDefinitions`, `TestModuleFirstValuesAreConstant`, `TestModuleRuleMessages`, `TestRepoModulesHoldOnlyDefinitions` (depodaki modüller: `lib/`, aktarılan her dosya; test dosyaları), `TestModuleRuleSuggestionRuns` (interp); TR testi `tests/moduller_test.tr` ("1.1 ..."), olumsuz fikstür `tests/negative/modul_en_dis.tr` + aktaran `modul_en_dis_aktar.tr`.
- **Fikstür testi:** `TestNegativeFixtures` fikstürleri modül olarak okuyordu (yalnız en dıştaki `TR_return` için); kural gelince 11 fikstür H1107/H1108 verdi. Artık başka bir fikstürün aktardığı fikstür modül, öbürleri program ("aynı dosya aktarılınca modüldür"); `ust_duzey_sonuc.tr`'ye aktaran `ust_duzey_sonuc_aktar.tr` eklendi. Test 596 satırlık dosyadan `behavior_negative_fixtures_test.go`'ya taşındı (600 sınırı).
- **Kural yüzünden değişen Go testleri:** interp `TestModulePassesTypeCheck` (modülün `TR_print`'i fonksiyona), `TestModuleRuntimeErrorWithOwnFile` ve e2e `TestModuleRunErrorFrame` (modülün en dıştaki sıfıra bölmesi modül fonksiyonuna; aktaran onu çağırıyor): testin konusu (modül hatasının kendi dosyası ve satırı) aynı.
- 1.1'in Test satırı güncellendi (`docs/spec/01_program.md`; `TestEveryRuleHasATest` TR testini anmayı istiyor).
- **Dalda bilinen düşüş:** `tools/codegraph` `TestEveryTrackedDataFileHasATestOrAReason` `.devir/DURUM.md` için düşüyor; `.devir` ana dala alınmadığından ana dalda düşmez.

## Adım 4: 0.9'dan kalan iki yanlış

- **(a) yapının adı değer olarak:** denetleyici yapının adını kendi tipinde sayıyordu; artık `TR_Type` (3.11; `TestStructNameIsATypeValue`). `lib/optimizers.tr:117` `TR_only_2d(TR_name: TR_Type, ...)` tiplendi, izin listesinin "4a'yı bekleyen" başlığı kalktı. `tr test lib`/`tests` 0.
- **(b) yalnız not:** `TR_SpikeGuard` (`lib/training.tr:39`, H6930) ve `TR_Run.TR_init` (`lib/training.tr:116`, H6932) çalışma anında `TR_Type(x) != TR_Int` sınıyor; `lib/training_test.tr:73` `TR_history_limit: 2.5` ile H6930 bekliyor. Kapı (tipsiz/yanlış tipli argüman denetim hatası) ikinci dilimde açılınca bu sınamalar ve test kalkar (tip denetimi yakalar).
