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

Son kapananlar (ana oturum 2026-10-26 aynı komutlarla doğruladı, yerel main ed69e06a; DENETİM yeniden koştu, sekizi de kapanmış): kitaplık dosyasını `aktar "tr/..."` ile koşan testlerin bağlanması (`tests-for lib/katman.tr` → `TestLibraryModuleCheckAndBuild`, `lib/hazir.tr` → `TestReadyModelMatchesTransformers`); ince ayardan `stats.go`'nun testinin uzak sırası (artık 5. ve `direct`), `main.go`'ya `.tr` testlerinden kuşkulu bağ, yapı köprüsünün `implemented_by` gürültüsü, derlenmeyen dil paketinde cevapsızlık (son derlenen ikili, 98fcbb19), README sayıları (be65d001), sınav sorularının satır ve sayıya bağlılığı (aa0b2a14).

**Denetim (DENETİM 2026-10-26, side-notes `kol-denetim-gaps-sonuc.md`):** 13 maddenin 9'u onay, 4'ü düzeltmeyle onay, ret yok; iki yeni kritik (aşağıda "yeniden adlandırma" ve "ileti kodu ↔ TR kodu" içinde), bir ince ayar. Düzeltmeler bu metne işlendi; DENETİM'in yeni bulgularını ana oturum yeniden üretti. Aşağıdaki maddelerin hepsi denetimden sonra CODEGRAPH'ta (2026-10-26).

- **`tr dizin` yok sayılan klasördeki `.tr` dosyasını depodakilerle aynı sırada ve işaretsiz veriyor.** 2026-10-26, ana oturum gerçek kullanım denemesinde yaşadı. Denetim: onay. **Yapıldı, birleşmeyi bekliyor** (`claude/codegraph-duzeltme` db6cb8db). Go tarafı (codegraph, depo aracı) kapandı: 5ce7ac39; ana oturum ve DENETİM doğruladı (yok sayılan klasöre konan geçici Go paketi grafiğe girmiyor).
  - Komut: `tr dizin tanım GPT` → `lib/katman.tr:490` ile `scratch/gpt30m/gpt30m.tr:81` aynı sırada, ayrımsız.
  - Tasarım (codeindex README, 60997f54): `tr dizin` kullanıcının aracıdır, `.gitignore`'a bakmaz. **Karar (Ahmet, 2026-10-26): böyle kalır, ama yok sayılan dosya ayrı işaretli ve depodakilerden sonra gelir.**
  - Beklenen: aday ve kullanım listelerinde yok sayılan dosyadan gelen satır sonda ve işaretli; belirsizlikte depodaki önce. Test: aynı ad `lib/`'de ve yok sayılan klasörde tanımlıyken `lib/` önce, öbürü işaretli.

- **`init()`'in okuduğu gömülü dosya, `init()`'in doldurduğu değişkeni kullanan testlere bağlanmıyor.** 2026-10-26, ana oturum yaşadı. Denetim: onay. **Yapıldı, birleşmeyi bekliyor** (3f79ba60).
  - Komut: `go run ./tools/codegraph tests-for tools/gen/snippets/data.json` → yalnız `TestSnippetDataIsLanguageNeutral`.
  - İkinci yol: `tools/gen/snippets/templates.go` `init()` `dataJSON`'dan `templates`'i, `text/*.json`'dan `texts`'i dolduruyor; `TestSnippetsUpToDate`, `TestSnippetsEveryLanguageEveryTemplate`, `TestSnippetsParseInEveryLanguage` `Generate` üzerinden bu değişkenleri okuyor (f40e9f64'te `text/*.json` geçici olarak testin doğrudan okumasıyla bağlandı).
  - Beklenen: `init()` ve paket değişkeni başlatıcısı → değişken → okuyan testler. Test: go/types ile, `init()`'te gömülü dosyadan doldurulan her değişken için okuyan testler `tests-for` cevabında.

- **`TestBuildScalesLinearly` CI'da art arda düşüyor (sahibi codeindex ve test altyapısı, codegraph'ın cevabı değil).** 2026-10-26, ana oturum CI'da yaşadı. Denetim: düzeltmeyle onay (sahip ve beklenen). **Yapıldı, birleşmeyi bekliyor** (86d49299: kök gürültü değil gerçek karesel terimdi [CODEGRAPH ölçtü], 800 modülde 1162 → 548 ms).
  - Gelen (saatler UTC): 14:29 "sıfırdan kurulum" oranı 9,2; 17:24 "yenileme" 10,6; 19:34 (08b32ffc) "önbellekten okuma" 8,7 (sınır 8, doğrusal ~4). Test `-short`'ta atlanır; yerelde `-short`'suz geçti.
  - İkinci yol: `gh run view 40387943068 --log-failed`; `internal/devtools/codeindex/cache_test.go:336`.
  - Beklenen: `ScalesLinearly` her boyu zaten üç kez ölçüp en kısasını alıyor (`testkit.go:205-270`); en iyi değer yetmiyorsa önce büyümenin gerçek olup olmadığı ölçülür. Gerçekse kök düzeltilir; değilse ölçüm gürültüye dayanıklı olur (ısınma, n=100'de daha uzun iş). Sınır gevşetilerek geçirilmez.

- **Go dışı bir testin okuduğu veri dosyası (eklentinin JS testi; Go testinin koştuğu Python betiği) bir teste bağlanmıyor.** 2026-10-26, ana oturum yaşadı (tam denetim). Denetim: onay, sınıfı genelleştir.
  - Komut: `go run ./tools/codegraph tests-for editors/vscode/l10n/bundle.l10n.tr.json` → test yok.
  - İkinci yol: `editors/vscode/test/l10n_test.js:18, 39, 54, 60, 68` (`readJSON` = `fs.readFileSync(path.join(ROOT, ...))`); onu `cmd/tr/e2e` `TestExtensionJSTests` koşuyor. `require` ile yüklenen JS dosyası bağlanıyor (`tests-for editors/vscode/coded_format.js`).
  - Geçici: dosya `testdata/uncovered_data.txt`'e yazıldı, `maxUncoveredData` 9 → 10 ("yalnızca küçülür" kuralına aykırı). `editors/vscode/*.json` satırı aynı sınıftan.
  - Beklenen: Go dışı testin okuduğu dosya (JS `readJSON`/`path.join`, Go testinin koştuğu Python'un açtığı dosya) o testi koşan Go testine bağlanır; iki istisna satırı silinir, sınır 8'e iner. (`tools/gpu/phase3_config.json` istisnası doğru kalır: onu bir test değil Colab betikleri okuyor.)

- **İletinin sahibi yanlış: hedefin üretmediği iletiyi bekleyen test "tests about it" grubunda.** 2026-10-26, ana oturum gerçek kullanım denemesinde yaşadı. Denetim: düzeltmeyle onay (kök "ortak çağrılan işlev"den geniş).
  - Komut: `go run ./tools/codegraph tests-for TR_fn_sum` → "tests about it" içinde `ml.TestGradClip`, `ml.TestDTypeChoice` `[expects H7901]`, `ml.TestConcatAndSplit` `[expects H7900]`, `ml.TestCustomGradBackwardError` `[expects H6126]`, `ml.TestErrors` `[expects H6115]`, `yerlesikler_test 13.1` `[expects H2011]`.
  - İkinci yol [ana oturum yeniden üretti]: H7900/H7901'i yalnız `bind.CheckNamed` üretiyor (`bind.go:86, 92`, adlı argüman alan her yerleşiğin ortak denetimi); H6126/H6115'i yalnız `ml.matmulDType` (`operators.go:220, 225`) ve `path interp.sumBuiltin ml.matmulDType` → "no directed path"; H2011'i `interp.ArgCountError` ve `addMethod` (her yerleşiğin ortak iletisi).
  - Neden kritik: en güçlü kanıt grubunda yanlış test. [DENETİM hipotezi] Test, TR dizesinde `toplam`'ı çağırıp iletiyi bekleyince ileti hedefe yazılıyor.
  - Beklenen: README'nin kuralı ad kodu hedefinde de uygulanır: "expects Hxxxx" ile gruplanan her testin iletisini hedef ve yalnız onun çağırdıkları üretir; birden çok hedefin çağırdığı işlevin iletisi hiçbirinin değildir.

- **İleti kodu ↔ TR kodu bağı yok: TR kodundaki ileti kodları ve iletinin metnini bekleyen TR testleri görünmüyor.** 2026-10-26, ana oturum (metinle bekleyen test) ve DENETİM (TR'deki ileti kodları) buldu; ana oturum ikisini de yeniden üretti. Denetim: iki madde birleşti.
  - Komut 1: `go run ./tools/codegraph who-uses H6932` → kullanım yok; `tests-for H6932` → yalnız üç katalog testi; `tr dizin kullananlar H6932` → "bulunamadı". İkinci yol: `lib/egitim.tr:116` `TR_fn_message(H6932, TR_arg_num_steps)`; `lib/egitim_test.tr:110` `TR_fn_assert_equal(H6932, TR_fn_error_code(...))`. Kapsam [DENETİM ölçtü]: TR kodunda geçen 27 farklı H69xx kodunun 26'sında "no uses", 10 dosya.
  - Komut 2: `go run ./tools/codegraph tests-for H2418` → yalnız kataloğu gezen üç test. İkinci yol: `tests/yapilar_test.tr:47` iletinin metnini bekliyor (`"'Nokta' yapısının 'yy' adında bir alanı yok"`).
  - Neden kritik: iletiyi değiştiren ya da silen onu üreten kitaplık satırını ve sınayan testi görmüyor (yanlış "yok").
  - Beklenen: kodlu TR kaynağındaki ileti kodu belirteci (`H6932`, `ileti(...)`, `hata_kodu(...) == H...`) `uses_message` kenarı verir; `tr dizin` ileti kodunu ad olarak bulur; TR test satırındaki metin bir iletinin sabit parçalarını taşıyorsa test o koda bağlanır. Test: kataloğun her kodu için Go'daki `messages.Hxxxx` kullanımı ya da `.tr`'deki belirteç grafikte bir kenarla. Not (ayrı iş, birikme dosyasında): TR testleri ileti metnini değil kodunu sınamalı.

- **Metinle sürülen dağıtım: testteki metin sabiti (DAP istek adı, test dizesindeki TR adı) işleyicisine bağlanmıyor.** 2026-10-26, ana oturum gerçek kullanım denemesinde yaşadı. Denetim: onay; "DAP isteği" ile "test dizesindeki TR kodu → uygulayan Go işlevi" aynı kök, tek madde.
  - Komut 1: `go run ./tools/codegraph tests-for internal/devtools/debugger/dap_table.go` → 6 test; `TestDAPSecretMasked` yok. İkinci yol: `behavior_dap_secrets_test.go:41` `c.request("trTable", ...)`; istek `dap_pause.go:125` `case "trTable": s.table(msg)`; aynı biçimde `behavior_dap_scope_test.go`, `behavior_dap_tensor_variables_test.go`, `dap_fuzz_test.go`. Köprünün öbür ucu: `extension.js:352, 431` `customRequest('trTable', ...)` ve `watch_panel_test.js:66, 70`.
  - Komut 2: `go run ./tools/codegraph tests-for --all internal/stdlib/dataset/state.go` → `dataset_test.TestReaderStateResumesSameBatches` 9 adımda (DENETİM'de 65., ana oturumda 62. sırada), `TestReaderSkipSameAsNext` yok. İkinci yol: `state_test.go` test dizesindeki TR koduyla `ö.durum`, `durumu_yükle`, `atla`'yı sınıyor; bunları `state.go`'daki `StateDict`, `loadStateTR`, `skipTR` uyguluyor.
  - Neden kritik: işleyiciyi değiştiren onun davranış testlerini görmüyor.
  - Beklenen: testteki metin sabiti bir dağıtım anahtarıysa (istek adı, TR'deki yerleşik ya da üye adı) test o anahtarın işleyicisine bağlanır; JS tarafı da aynı işleyiciye.

- **Dosyanın dış test paketindeki adaş test dosyası "this file's tests" grubunda değil.** 2026-10-26, ana oturum yaşadı. Denetim: düzeltmeyle onay (eski maddenin ilk yarısı; ikinci yarısı yukarıdaki dağıtım maddesine taşındı). Kapanan `stats.go` maddesinin (aynı paket) genellenmemiş hâli.
  - Komut: `go run ./tools/codegraph tests-for internal/stdlib/dataset/state.go` → `state_test.go`'nun testleri (`package dataset_test`) "this file's tests" grubunda yok.
  - Beklenen: `x.go`'nun adaşı `x_test.go`, aynı ya da dış (`_test`) pakette, "this file's tests" grubunda. Test: her `x.go`/`x_test.go` çiftinde dış paketteki adaş testler ilk grupta.

- **Sınav dosyasının andığı düğüm sınava bağlanmıyor; ön denetim sınavı seçmiyor.** 2026-10-26, MEKANİK bildirmişti (ince ayardaydı); DENETİM kökü codegraph'ta buldu ve kritik yaptı.
  - Komut: `go run ./tools/codegraph tests-for --all internal/lang/interp/compiler_expr.go` → `TestPlanningQuestionsAreAnswered` yok [ana oturum yeniden üretti]; sınav `planning_questions.txt:162` `interp.codedKeyOf`'u anıyor. Buna karşılık `tests-for lib/egitim.tr` sınavı veriyor (`lib/embedded.go` üzerinden).
  - Gerçek etkisi: disk anahtarları işinde `dictKey → codedKeyOf` yeniden adlandırması sınavı kırdı, ancak tam denetimde görüldü.
  - Beklenen: codegraph kendi sınav dosyasının biçimini biliyor (`contains: pkg.Name`); her soruda adı geçen düğüm sınava test bağı verir (init/veri dosyası maddesinin özel hâli). Bu bağ kurulunca ön denetimin `changed --go-tests` seçimi sınavı da seçer.

- **`tr dizin yeniden_adlandır` kodlu bir adı kullanıcı adı gibi planlıyor.** 2026-10-26, DENETİM buldu; ana oturum yeniden üretti. Kritik.
  - Komut: `tr dizin yeniden_adlandır "parameter:lib/egitim.tr#TR_type_Run.TR_ctx_init.TR_arg_num_steps" adım_toplamı` → "kesin (11)": kodlu dosyalarda `TR_arg_num_steps`'in geçtiği 11 yer (`lib/egitim.tr:115-118`, `lib/egitim_test.tr:79, 97, 110`, `examples/tr/48_egitim_kosusu.tr:38`, `tools/gpu/gpt_training.tr:71`); "olası": `internal/lang/codes/tr.json:310` (adın asıl yeri), `en.json`, `definitions.json`, ileti yuvası, `lib/iyilestirici.tr:425` (aynı kodu kullanan `öğrenme_oranı_planı`).
  - İkinci yol: `lib/egitim.tr:115` ve `lib/iyilestirici.tr:425` aynı `TR_arg_num_steps` kimliğini taşıyor; adı `tr.json:310` `"name": "adım_sayısı"`.
  - Neden kritik: "kesin" listeyi uygulamak kimliği kullanıcı adına çevirir, kodlu biçimi bozar ve aynı kodu kullanan bildirimi atlar.
  - Beklenen: hedef bir ad koduysa önizleme "dil dosyasında ad değişikliği" planı verir: her dil dosyasındaki satır, aynı kodu kullanan bildirimler, yeni adın onay gerektirdiği notu; kodlu dosya kesin listede yer almaz. Test: kodlu kitaplık parametresi, yapısı ve işlevi için önizlemede kodlu dosya kesin listede yok, dil dosyası satırı kesin listede.

- **Kabuk ve Python betiklerinin ortam değişkeni kullanımları görünmüyor.** 2026-10-26, ana oturum yaşadı. Denetim: onay.
  - Komut: `go run ./tools/codegraph query TR_env_GPU_SYNC` → Go'da okuyan (`gpu.newBackend`) ve testler; değişkeni koyan betik yok.
  - İkinci yol: `tools/gpu/card_check.sh:27` `TR_env_GPU_SYNC=1 ...`; `tools/full_check.sh:167-169, 437` `TR_CPU_AVX2`, `TR_DIL`; betiklerde `TR_` geçen 3 dosya (`tools/bench/run.py`, `full_check.sh`, `card_check.sh`); `.github/workflows/*.yml`'de yok [DENETİM ölçtü].
  - Beklenen: `.sh` ve `.py` dosyalarındaki ortam değişkeni ataması ve okuması ad koduna ya da (kodsuzsa) değişken adına `uses_env` kenarı verir. Test: betiklerde geçen her `TR_` değişkeni grafikte bir kenarla.

## İnce ayar

- **Kodun Go'daki uygulaması sorulunca harita satırındaki bütün işlevler veriliyor.** Denetim: onay. **Yapıldı, birleşmeyi bekliyor** (3d78e02d).
  - Komut: `go run ./tools/codegraph query TR_cmd_encode` → "tr.commands at cmd/tr/main.go:125 -> tr.debugCommand, tr.encodeCommand, tr.formatCommand, tr.testCommand". Beklenen: yalnız `tr.encodeCommand` (anahtar-değer çiftinin değeri).

- **`tr dizin etki`'de kısaltılmış grup bunu söylemiyor.** Denetim: onay. CODEGRAPH'ta (2026-10-26).
  - Komut: `tr dizin etki TR_type_GPT` → "bunu aktaran modüller (13):" altında 12 satır, not yok (ikinci grupta var). Beklenen: her kısaltılmış grup kaç satırın gizlendiğini yazar.

- **`tests-for` başlığı "up to 8 steps" diyor, liste 9 adımlık zincirler gösteriyor.** DENETİM buldu; ana oturum `state.go` denemesinde gördü. CODEGRAPH'ta (2026-10-26).
  - Örnekler: `17_testler_test`, `deyimler_test 5.10-5.12`, `dataset_test.TestReaderStateResumesSameBatches` (9 adım). [DENETİM hipotezi] Paket sınırı geçişi adım sayısına ekleniyor, süzgeç (`test_paths.go:500` `n.Steps > maxSteps`) onu saymadan uygulanıyor.
  - Beklenen: sınır ve liste tutarlı; ya geçiş sayılır ya başlık gerçek sınırı yazar.

- **`compares_value` noktalama gürültüsü.** 2026-10-26, CODEGRAPH bildirdi. Denetim: onay (satırlar bir kaymış: `other_os.go:43`). **Bekliyor: çekirdek değişikliği** (işleç alanlarının `lexer.TokenType` olması); CODEGRAPH'a gitmedi.
  - Komut: `go run ./tools/codegraph who-uses lexer.TokDot` → `compares_value (17)`, çoğu dosya yolu ve bayrak kodu. Beklenen: yalnız işleci kastedenler; tip süzgeci, AST'nin işleç alanları `TokenType` olduktan sonra.

## Kalan işler (eksik değil, yeni yetenek)

- **Her zaman atlanan testlerin listesi (istek, 2026-10-26, gerçek kullanım denemesi):** arşivdeki testleri (`t.Skip("ARŞİV …")`, bugün 56) toplu silmek için hepsinin listesi gerekiyor. codegraph her testin atlama koşulunu biliyor (`[skips when: always]`) ama bunları listeleyen bir sorgu yok; bugün grep ile bulunuyor. İstek: koşulu `always` olan testleri (gerekçe metniyle) veren bir sorgu ya da `query` süzgeci.

- `.tr` kodunda tekrar bulma (`dups`'ın `.tr` karşılığı; `tr dizin kopya` bugün yok, 2026-10-25'te denendi).
- **Kullanım yerinin rolü (istek, 2026-10-26, madde 15 tasarımı):** `who-uses` bir adın her kullanımını aynı listede veriyor; kullanımın ne işe yaradığını ayırmıyor. İç dil tasarımında `codes.Name`'in ~90 kullanımından hangisinin kimlik (karşılaştırma, harita anahtarı, `switch`, kayıt: `Define`/`Register`), hangisinin görüntü (ileti ya da `fmt` argümanı) olduğu kaynak okunarak ayrıldı. İstek: her kullanım yerine bir rol etiketi (görüntü / kimlik / kayıt), `who-uses --role` gibi süzme. Görüntü ve dosya biçimi rolleri d4de3803 ile kenar türü oldu (`formats`, `encodes`, `stdlib_calls`); kimlik ve kayıt rolleri ve süzme kalıyor (`compares_value` kimliğin metin tarafı).
- **Arayüz kullanımlarının kalan iki yolu (CODEGRAPH bildirdi, 2026-10-26; ana oturum henüz yeniden üretmedi). DENETİM: eksik, "yeni yetenek" değil; `Error()`'ın çağıransız görünmesi `dead-methods` için yanlış "kullanılmıyor" demek, kural gereği kritik aday; yeniden üretilince kritik listesine:** `...any`'yi `fmt`'ye geçen modül işlevleri (vet'in printf sarmalayıcı kuralı) bugün yalnız `messages` paketi için biçimleme sayılıyor; `error` olarak döndürülüp başka yerde biçimlenen değerin `Error()`'ı çağıransız.
- `dead-methods` ve `package-state`'in ilk koşusunun bulgularının ayıklanması (sayılar 2026-10-26, DENETİM: 56 yalnızca arayüzle ya da yalnızca testte yaşayan metot, 120 paket değişkeni; iş açılmadan önce yeniden koşulur); korumasız görünenler: `tensor.backends`, `tokenize` kayıt haritaları, `codes.defs`, `messages.catalogs`, `interp.engines`, `logging` açık çubuklar ve günlükler, `cmd/tr.exitCode`.
- **İşlev değeri argüman olarak geçince test zinciri kopuyor (CODEGRAPH bildirdi, 2026-10-26; ana oturum yeniden üretmedi). DENETİM: dolaylı testler kayboluyor, yanlış "az test", kural gereği kritik aday; yeniden üretilince kritik listesine:** `f(..., buildLanguage)` gibi bir çağrıda işlev değeri INFERRED `calls` kenarıdır ve `reachEdge` onu izlemez; yüksek dereceli işlevlerde "listeyi gezen test" ve dolaylı testler kaybolur. CODEGRAPH ince ayar 3'te `startLanguages`'ı işlev yerine veri alacak biçimde değiştirerek kendi örneğini kapattı. Genel çözüm: argüman olarak geçen modül işlevi, onu çağıran parametre üzerinden gerçek bir çağrı sayılır.
- **Hız (karar 2026-10-26: şimdi yapılmaz).** DENETİM ölçtü (codegraph-karar-ve-hiz-sonuc.md, yük altında): kurulum 13,3 s, bunun 5,7 s'si iki `go list` çağrısı, tip denetimi 1,8 s, `extractGo` 1,6 s; güncel grafta soru tabanı `graph.json`'un okunması ~1,1 s. Ana oturumun ölçümü: o gün üç oturumda ~70 ilk düzenleme; kazanç günde birkaç dakika, önbellek hatasının riski (bayat ya da yanlış cevap) buna değmedi. Yapılacaksa sıra: `go list` önbelleği → grafın okunması → ağaçlar arası ortak graf → kancanın bayat ve etiketli cevabı → Go tarafını yeniden kullanmak; `go run`'ı kaldırmak ve paket başına artımlı tip denetimi kazandırmaz. **Yeniden açma eşiği:** kurulum 30 s'yi ya da güncel grafta bir soru 3 s'yi geçerse (graf 2026-10-23'te ~9 bin, 2026-10-26'da ~25 bin düğüm).
