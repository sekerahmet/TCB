# codegraph eksikleri

codegraph'ın (ve `tr dizin`'in) yetmeyen ya da yanlış cevaplarının biriktiği yer. Yol haritası
yalnızca aracın gelişimini özetler ([madde 7](../../docs/roadmap.md)); tek tek eksikler buradadır.

**Kim yazar:** dosyayı yalnızca ana oturum günceller. Yan oturumlar ve ajanlar codegraph'ın
yanlış, eksik, belirsiz ya da yavaş cevabını (kancanın gösterdikleri dahil, `tools/edithook`) ana
oturuma bildirir: komut, beklenen, gelen, ikinci yoldan doğrulama (grep, kaynak). Ana oturum
yeniden üretip doğruladığını ya da baş geliştiriciyle birlikte kararlaştırdığını ekler. Ad
tahminiyle aranıp bulunamayan şey eksik değildir (CLAUDE.md, "Kod grafiğinin cevabı nasıl
okunur").

**Sınıf (Ahmet, 2026-10-25):** yanlış bir değişikliğe götüren cevap (kesin grupta yanlış yer,
yanlış "yok", yanlış kenar, ortama bağlı düşen depo testi) **kritik**tir; sıralama, etiket ve
gösterim **ince ayar**dır.

**Akış (Ahmet, 2026-10-26; baş geliştirici aksini onaylamadıkça):** maddeler bu dosyada birikir; ana oturum listeyi
baştan sona gözden geçirir; DENETİM bütününü denetler (yeniden üretim, sınıf, istenen çözümün genelliği); onaylananlar
tek mesajla CODEGRAPH'a gider (tek tek gönderilmez); gönderilen madde "CODEGRAPH'ta (tarih)" diye işaretlenir. Düzeltme ana dala birleştirilince ana oturum
düzeltmeyi aynı komutla doğrular ve maddeyi siler (geçmişi git'tedir). İstenen her zaman örneği yamalayan değil aynı sınıftaki
her durumu kapatan genel çözüm ve sınıfı koruyan bir testtir.

## Kritik

Son kapananlar (ana oturum 2026-10-26 aynı komutlarla doğruladı, yerel main ed69e06a; ana dala henüz gönderilmedi): kitaplık dosyasını `aktar "tr/..."` ile koşan testlerin bağlanması (`tests-for lib/katman.tr` → `TestLibraryModuleCheckAndBuild`, `lib/hazir.tr` → `TestReadyModelMatchesTransformers`); ince ayardan `stats.go`'nun testinin uzak sırası (artık 5. ve `direct`), `main.go`'ya `.tr` testlerinden kuşkulu bağ, yapı köprüsünün `implemented_by` gürültüsü, derlenmeyen dil paketinde cevapsızlık (son derlenen ikili, 98fcbb19), README sayıları (be65d001), sınav sorularının satır ve sayıya bağlılığı (aa0b2a14).

- **`tr dizin` yok sayılan klasördeki `.tr` dosyasını depodakilerle aynı sırada ve işaretsiz veriyor.** 2026-10-26, ana oturum gerçek kullanım denemesinde yaşadı (main ed69e06a). CODEGRAPH'ta (2026-10-26). Go tarafı (codegraph, depo aracı) kapandı: 5ce7ac39; ana oturum doğruladı (yok sayılan `scratch/`'e konan geçici bir Go paketinin işlevi `query` ile bulunmuyor).
  - Komut: `tr dizin tanım GPT` → 3 aday; `lib/katman.tr:490` ile `scratch/gpt30m/gpt30m.tr:81` aynı sırada, ayrımsız.
  - Tasarım (codeindex README, 60997f54): `tr dizin` kullanıcının aracıdır, `.gitignore`'a bakmaz; kişinin `scratch/`'te yazdığı denemeler de dizine girer. **Karar (Ahmet, 2026-10-26): böyle kalır, ama yok sayılan dosya ayrı işaretli ve depodakilerden sonra gelir.** CODEGRAPH'ın dışlayan değişikliği (6e2b2a73) alınmadı.
  - Beklenen: aday ve kullanım listelerinde git'in yok saydığı dosyadan gelen satır izlenen (ve izlenmeyip yok sayılmayan) dosyalardan sonra sıralanır, "yok sayılan klasör" diye işaretlenir; tek aday o değilse belirsizlik çözülürken depodaki önce gelir. Test: aynı ad hem `lib/`'de hem yok sayılan klasörde tanımlıyken `tanım` cevabında `lib/` önce, öbürü işaretli.

- **`init()`'in okuduğu gömülü dosya, `init()`'in doldurduğu değişkeni kullanan testlere bağlanmıyor.** 2026-10-26, ana oturum yaşadı (tam denetim ve gerçek kullanım denemesi).
  - Komut: `go run ./tools/codegraph tests-for tools/gen/snippets/data.json` → yalnız `TestSnippetDataIsLanguageNeutral` (gömülü değişkeni doğrudan okuyan test).
  - İkinci yol: `tools/gen/snippets/templates.go` `init()` `dataJSON`'dan `templates`'i, `text/*.json`'dan `texts`'i dolduruyor; `TestSnippetsUpToDate`, `TestSnippetsEveryLanguageEveryTemplate` ve `TestSnippetsParseInEveryLanguage` `Generate` üzerinden bu değişkenleri okuyor: dördü de bu dosyalara bağlı. `text/*.json` hiçbir teste bağlanmıyordu (`TestEveryTrackedDataFileHasATestOrAReason` kaldı; f40e9f64'te testin `textFiles`'ı doğrudan okumasıyla geçici olarak bağlandı).
  - Neden kritik: `tests-for` dört testin üçünü göstermiyor; kanca ve ön denetim veri dosyası değişince bu testleri seçmiyor (yanlış "az test").
  - Beklenen: `init()` (ve paket düzeyi değişken başlatıcısı) bir dosyayı okuyup bir değişkene yazıyorsa, o değişkeni okuyan testler o dosyaya bağlanır. Test: go/types ile, `init()`'te gömülü dosyadan doldurulan her değişken için okuyan testler `tests-for` cevabında.

- **`TestBuildScalesLinearly` CI'da art arda düşüyor.** 2026-10-26, ana oturum CI'da yaşadı (uzun testler işi; üç koşu).
  - Gelen (saatler UTC): 14:29 "sıfırdan kurulum" oranı 9,2; 17:24 "yenileme" 10,6; 19:34 (08b32ffc) "önbellekten okuma" 8,7 (sınır 8, doğrusal ~4); öbür aşamalar da sınırın dibinde (7,5 ve 7,8). Test `-short`'ta atlanır; yerelde (Windows) `-short`'suz `go test ./internal/devtools/codeindex` geçti (2026-10-26).
  - İkinci yol: `gh run view 40387943068 --log-failed`; `internal/devtools/codeindex/cache_test.go:340-349`.
  - Neden kritik: ortama bağlı düşen depo testi; ana dal üç gönderimdir kırmızı.
  - Beklenen: önce ölç: büyüme gerçekten doğrusalın üstündeyse (100 → 400 modülde ~8 kat) kökü bulunur ve düzeltilir; değilse ölçüm gürültüye dayanıklı olur (birkaç tekrarın en iyisi, ısınma) ve gerekçesi testte yazılır. Sınır gevşetilerek geçirilmez.

- **VS Code eklentisinin JS testlerinin okuduğu veri dosyaları bir teste bağlanmıyor.** 2026-10-26, ana oturum yaşadı (tam denetim, `TestEveryTrackedDataFileHasATestOrAReason`).
  - Komut: `go run ./tools/codegraph tests-for editors/vscode/l10n/bundle.l10n.tr.json` → test yok.
  - İkinci yol: `editors/vscode/test/l10n_test.js:39` dosyayı okuyor; onu `cmd/tr/e2e` `TestExtensionJSTests` koşuyor. (JS'nin `require` ile yüklediği dosya bağlanıyor: `tests-for editors/vscode/coded_format.js` → `TestExtensionJSTests`; `readJSON('l10n', ...)`/`path.join` ile okunan bağlanmıyor.)
  - Geçici: dosya `testdata/uncovered_data.txt`'e yazıldı, `maxUncoveredData` 9 → 10 ("yalnızca küçülür" kuralına aykırı). `editors/vscode/*.json` satırı aynı sınıftan.
  - Beklenen: eklentinin JS testinin okuduğu dosya (`readJSON(...)`, `path.join(ROOT, ...)`), o testi koşan Go testine (`TestExtensionJSTests`) bağlanır; iki istisna satırı silinir, sınır 8'e iner.

- **Ortak çağrılan işlevin iletisi hedefin kendi iletisi sayılıyor; ilgisiz testler "tests about it" grubunda.** 2026-10-26, ana oturum gerçek kullanım denemesinde yaşadı (2. tur, main e723eb54).
  - Komut: `go run ./tools/codegraph tests-for TR_fn_sum` → "tests about it": `ml.TestGradClip`, `ml.TestDTypeChoice` `[expects H7901]`, `ml.TestConcatAndSplit` `[expects H7900]`.
  - İkinci yol: `H7900` ve `H7901`'i yalnız `bind.CheckNamed` üretiyor (`internal/stdlib/common/bind/bind.go:86, 92`); onu adlı argüman alan her yerleşik çağırıyor. Bu testler `toplam`'la ilgili değil.
  - Neden kritik: en güçlü kanıt grubunda yanlış test (kesin grupta yanlış yer); okuyucu `toplam`'ın asıl testlerini bunlarla karıştırır.
  - Beklenen: README'deki "ortak sarmalayıcının iletisi kimsenin değildir" kuralı ortak çağrılan işleve de uygulanır: iletiyi üreten işlevi birden çok hedef çağırıyorsa ileti hiçbirinin "kendi" iletisi sayılmaz. Test: "expects Hxxxx" ile gruplanan her testin iletisini hedef ve yalnız onun çağırdıkları üretir.

- **TR testinin metniyle beklediği ileti, ileti koduna bağlanmıyor.** 2026-10-26, ana oturum gerçek kullanım denemesinde yaşadı (2. tur).
  - Komut: `go run ./tools/codegraph tests-for H2418` → yalnız kataloğu gezen üç test (`TestEmbeddedCatalogValid` ...).
  - İkinci yol: `tests/yapilar_test.tr:47` iletinin metnini bekliyor (`"'Nokta' yapısının 'yy' adında bir alanı yok"`); iletinin asıl davranış testi bu.
  - Neden kritik: iletiyi değiştiren, onu sınayan testi görmüyor (yanlış "yok").
  - Beklenen: TR test satırındaki metin bir iletinin sabit parçalarını taşıyorsa test o ileti koduna bağlanır (README'nin `{fn}` yuvalı kuralının genel hâli). Test: her ileti kodu için, metnini bekleyen TR testi `tests-for` cevabında. Not (ayrı iş, codegraph değil): TR testleri ileti metnini değil kodunu sınamalı (`hata_kodu`, ÇEVİRİ K4).

- **DAP isteğiyle sürülen testler isteğin işleyicisine bağlanmıyor.** 2026-10-26, ana oturum gerçek kullanım denemesinde yaşadı (2. tur).
  - Komut: `go run ./tools/codegraph tests-for internal/devtools/debugger/dap_table.go` → 6 test (`TestTableData`, `TestWatchValueInterface` ...); `TestDAPSecretMasked` yok.
  - İkinci yol: `internal/devtools/debugger/behavior_dap_secrets_test.go:41` `c.request("trTable", ...)` gönderip tablo cevabını sınıyor; istek `dap_pause.go:125` `case "trTable": s.table(msg)` ile işleyiciye gidiyor. Aynı biçimde `behavior_dap_scope_test.go`, `behavior_dap_tensor_variables_test.go`, `dap_fuzz_test.go`.
  - Kök: test isteği bağlantıya yazıyor, sunucu başka goroutine'de okuyup metin `switch`'iyle dağıtıyor; statik zincir kopuyor (test dizesindeki TR kodu ile aynı sınıf: veriyle sürülen dağıtım).
  - Neden kritik: işleyiciyi değiştiren onun davranış testlerini görmüyor.
  - Beklenen: testin gönderdiği istek adı (metin sabiti) ile o adı `case` eden işleyici arasında test bağı. Aynı köprünün öbür ucu: eklentinin `customRequest('trTable', ...)` çağıran JS kodu ve testleri (`editors/vscode/test/table_common_test.js`, `watch_panel_test.js`) Go işleyicisine bağlanmıyor; protokol anahtarı değişince (V3, 2026-10-26) iki taraf elle bulundu.

- **Kabuk ve Python betiklerinin ortam değişkeni kullanımları görünmüyor.** 2026-10-26, ana oturum gerçek kullanım denemesinde yaşadı (3. tur).
  - Komut: `go run ./tools/codegraph query TR_env_GPU_SYNC` → Go'da okuyan (`gpu.newBackend`) ve testler; değişkeni koyan betik yok.
  - İkinci yol: `tools/gpu/card_check.sh:27` `TR_env_GPU_SYNC=1 "$bin/gpu_sinama"`; `tools/full_check.sh:167-169, 437` `TR_CPU_AVX2`, `TR_DIL`; `git grep -l -E 'TR_(env_|DIL|GPU_|IS_|CPU_)' -- '*.sh' '*.py'` → 3 dosya.
  - Neden kritik: değişkenin adını değiştiren betiği görmüyor (yanlış "başka kullanan yok"); e528db5a'da `card_check.sh` elle bulundu.
  - Beklenen: `.sh` ve `.py` dosyalarındaki ortam değişkeni ataması ve okuması (`X=1 komut`, `export X`, `${X}`, `os.environ["X"]`) ad koduna ya da (kodsuzsa) değişken adına `uses_env` kenarı verir. Test: depodaki her betikte geçen `TR_` değişkeni grafikte bir kenarla.

- **Dış test paketindeki adaş test dosyası ve TR koduyla sürülen testler: dosyanın kendi testleri en sonda ya da hiç yok.** 2026-10-26, ana oturum gerçek kullanım denemesinde yaşadı (3. tur). Kapanan `stats.go` maddesinin (aynı paket) genellenmemiş hâli.
  - Komut: `go run ./tools/codegraph tests-for --all internal/stdlib/dataset/state.go` → "tests that exercise the declarations in this file (275, direct first)": listenin başı `checkpoint_test` ve `kontrol_noktasi_test` testleri (ilk 60 satır: 6 tanesi 5, 51'i 6, 2'si 7, 1'i 8 adım); `dataset_test.TestReaderStateResumesSameBatches` 62. sırada, `9 steps [INFERRED]` (README dolaylı zincir için 8 adım diyor); `dataset_test.TestReaderSkipSameAsNext` yok.
  - İkinci yol: `internal/stdlib/dataset/state_test.go` (`package dataset_test`) iki testi de test dizesindeki TR koduyla `ö.durum`, `durumu_yükle` ve `atla`'yı sınıyor; bunları `state.go`'daki `StateDict`, `loadStateTR`, `skipTR` uyguluyor.
  - Neden kritik: dosyayı değiştiren onun birim testini görmüyor ya da sonda görüyor (yanlış "yok").
  - Beklenen: dosyanın adaşı test dosyası (aynı ya da `_test` dış paket) "this file's tests" grubunda; test dizesindeki TR kodunun çağırdığı yerleşik ya da üye (ad kodu) onu uygulayan Go işlevine `direct` bağ verir. Test: her `x.go`/`x_test.go` çiftinde, dış paketteki adaş testler `tests-for x.go`'nun ilk grubunda.

## İnce ayar

- **`tr dizin etki`'de kısaltılmış grup bunu söylemiyor.** 2026-10-26, ana oturum gerçek kullanım denemesinde yaşadı (2. tur).
  - Komut: `tr dizin etki TR_type_GPT` → "bunu aktaran modüller (13):" altında 12 satır; "daha fazlası var" notu yok (ikinci grupta var). On üçüncü (`tools/gpu/training_model.tr`) yalnız `--tümü` ile görünüyor.
  - Beklenen: her kısaltılmış grup kaç satırın gizlendiğini yazar (codegraph'ın `... N more` kuralı gibi). Sayı başlıkta olduğu için yanlış karar zor: ince ayar.

- **Kodun Go'daki uygulaması sorulunca harita satırındaki bütün işlevler veriliyor.** 2026-10-26, ana oturum gerçek kullanım denemesinde yaşadı (main ed69e06a).
  - Komut: `go run ./tools/codegraph query TR_cmd_encode` → "registered or used in Go: tr.commands at cmd/tr/main.go:125 -> tr.debugCommand, tr.encodeCommand, tr.formatCommand, tr.testCommand".
  - İkinci yol: `cmd/tr/main.go:124` `codes.CmdDebug: debugCommand, codes.CmdTest: testCommand, codes.CmdFormat: formatCommand, codes.CmdEncode: encodeCommand` (bir satırda dört anahtar-değer çifti).
  - Beklenen: kod bir haritanın anahtarıysa uygulama o çiftin değeridir (`*ast.KeyValueExpr`): yalnız `tr.encodeCommand`. Doğrusu listede, ama okuyucu seçmek zorunda: ince ayar.

- **`compares_value` noktalama gürültüsü.** 2026-10-26, CODEGRAPH bildirdi, ana oturum main d4de3803'te yeniden üretti. **Bekliyor: çekirdek değişikliği** (karar ana oturum, 2026-10-26).
  - Komut: `go run ./tools/codegraph who-uses lexer.TokDot` → `compares_value (17)`, çoğu dosya yolu ve bayrak kodu (`other_os.go:42`, `docs.go:127`, `rules.go:113` ...); `"."`, `"-"`, `"+"`, `"/"` için aynı.
  - Beklenen: yalnız işleci kastedenler. Kök (DENETİM): `TokDot` tipsiz bir sabit, asıl kullanımı `TokenType` tipli. Çözüm: `compares_value` kenarı yalnız karşılaştırılan ifadenin tipi sabitin gerçek kullanımlarında aldığı tiplerden biriyse kurulur (go/types `Info.Types`). Test: `TokDot`'un `compares_value` kenarlarının hepsi `TokenType` tipli ifadelerde. Fazla tahmin, yanlış "yok" değil: ince ayar.
  - CODEGRAPH ölçtü: tip süzgeci tek başına doğru kenarları da düşürür, çünkü AST'nin işleci de metin (`ast.go:62, 295, 306` `Operator string`; `interp.opCode(op string)`, denetleyicinin `e.Operator` karşılaştırması string tipli). Karar: önce işleç alanları `lexer.TokenType` olur (Go'nun `go/ast.BinaryExpr.Op token.Token` gibi; DENETİM go/types ile 10 dosyada 25 kullanım saydı, taşıma; değişmezler sabite çevrilmez), sonra süzgeç eklenir. Çekirdek değişikliği madde 15 adım 2'den sonra.

- **Ön denetim planlama sınavını koşmuyor.** 2026-10-26, MEKANİK bildirdi, ana oturum üç kez yaşadı. Soruların satır ve sayıya bağlılığı kapandı (aa0b2a14); kalan: ön denetim (`--kip=on`) `tools/codegraph`'ın sınavını koşmuyor, kırık soru ancak tam denetimde görülüyor. Kapsamı ayrı iş (`scratch/mekanik_bekleyenler.md` C15: testleri `tests-for`/`changed` cevabından seçmek).

## Kalan işler (eksik değil, yeni yetenek)

- **Her zaman atlanan testlerin listesi (istek, 2026-10-26, gerçek kullanım denemesi):** arşivdeki testleri (`t.Skip("ARŞİV …")`, bugün 56) toplu silmek için hepsinin listesi gerekiyor. codegraph her testin atlama koşulunu biliyor (`[skips when: always]`) ama bunları listeleyen bir sorgu yok; bugün grep ile bulunuyor. İstek: koşulu `always` olan testleri (gerekçe metniyle) veren bir sorgu ya da `query` süzgeci.

- `.tr` kodunda tekrar bulma (`dups`'ın `.tr` karşılığı; `tr dizin kopya` bugün yok, 2026-10-25'te denendi).
- **Kullanım yerinin rolü (istek, 2026-10-26, madde 15 tasarımı):** `who-uses` bir adın her kullanımını aynı listede veriyor; kullanımın ne işe yaradığını ayırmıyor. İç dil tasarımında `codes.Name`'in ~90 kullanımından hangisinin kimlik (karşılaştırma, harita anahtarı, `switch`, kayıt: `Define`/`Register`), hangisinin görüntü (ileti ya da `fmt` argümanı) olduğu kaynak okunarak ayrıldı. İstek: her kullanım yerine bir rol etiketi (görüntü / kimlik / kayıt), `who-uses --role` gibi süzme. Görüntü ve dosya biçimi rolleri d4de3803 ile kenar türü oldu (`formats`, `encodes`, `stdlib_calls`); kimlik ve kayıt rolleri ve süzme kalıyor (`compares_value` kimliğin metin tarafı).
- **Arayüz kullanımlarının kalan iki yolu (CODEGRAPH bildirdi, 2026-10-26; ana oturum henüz yeniden üretmedi, sınıfı üretilince verilir):** `...any`'yi `fmt`'ye geçen modül işlevleri (vet'in printf sarmalayıcı kuralı) bugün yalnız `messages` paketi için biçimleme sayılıyor; `error` olarak döndürülüp başka yerde biçimlenen değerin `Error()`'ı çağıransız.
- `dead-methods` ve `package-state`'in ilk koşusunun bulgularının ayıklanması (sayılar 2026-10-26, DENETİM: 56 yalnızca arayüzle ya da yalnızca testte yaşayan metot, 120 paket değişkeni; iş açılmadan önce yeniden koşulur); korumasız görünenler: `tensor.backends`, `tokenize` kayıt haritaları, `codes.defs`, `messages.catalogs`, `interp.engines`, `logging` açık çubuklar ve günlükler, `cmd/tr.exitCode`.
- **İşlev değeri argüman olarak geçince test zinciri kopuyor (CODEGRAPH bildirdi, 2026-10-26; bilinçli sınır, ana oturum yeniden üretmedi):** `f(..., buildLanguage)` gibi bir çağrıda işlev değeri INFERRED `calls` kenarıdır ve `reachEdge` onu izlemez; yüksek dereceli işlevlerde "listeyi gezen test" ve dolaylı testler kaybolur. CODEGRAPH ince ayar 3'te `startLanguages`'ı işlev yerine veri alacak biçimde değiştirerek kendi örneğini kapattı. Genel çözüm: argüman olarak geçen modül işlevi, onu çağıran parametre üzerinden gerçek bir çağrı sayılır.
- **Hız (karar 2026-10-26: şimdi yapılmaz).** DENETİM ölçtü (codegraph-karar-ve-hiz-sonuc.md, yük altında): kurulum 13,3 s, bunun 5,7 s'si iki `go list` çağrısı, tip denetimi 1,8 s, `extractGo` 1,6 s; güncel grafta soru tabanı `graph.json`'un okunması ~1,1 s. Ana oturumun ölçümü: o gün üç oturumda ~70 ilk düzenleme; kazanç günde birkaç dakika, önbellek hatasının riski (bayat ya da yanlış cevap) buna değmedi. Yapılacaksa sıra: `go list` önbelleği → grafın okunması → ağaçlar arası ortak graf → kancanın bayat ve etiketli cevabı → Go tarafını yeniden kullanmak; `go run`'ı kaldırmak ve paket başına artımlı tip denetimi kazandırmaz. **Yeniden açma eşiği:** kurulum 30 s'yi ya da güncel grafta bir soru 3 s'yi geçerse (graf 2026-10-23'te ~9 bin, 2026-10-26'da ~25 bin düğüm).
