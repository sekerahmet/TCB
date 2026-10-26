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

**Akış:** maddeler birikir ve CODEGRAPH oturumuna toplu gider (tek tek gönderilmez); gönderilen
madde "CODEGRAPH'ta (tarih)" diye işaretlenir. Düzeltme ana dala birleştirilince ana oturum
düzeltmeyi aynı komutla doğrular ve maddeyi siler (geçmişi git'tedir). İstenen her zaman örneği yamalayan değil aynı sınıftaki
her durumu kapatan genel çözüm ve sınıfı koruyan bir testtir.

## Kritik

Son kapanan kritik madde: kimliğe yeniden adlandırılan kodların "silinmiş" sayılması (5d5fa022). İnce ayardan son kapananlar: test dosyasının kendi testleri ve komut testlerinin uzaklığı (372ac2ed birleşmesi; ana oturum aynı komutlarla doğruladı).

- **codegraph git'in yok saydığı dosyaları dizinliyor; yerel cevap temiz kopyanınkinden farklı.** 2026-10-26, ana oturum yaşadı (d290fd24'ün CI'ı kaldı). CODEGRAPH'ta (2026-10-26).
  - Komut: `go run ./tools/codegraph who-uses ast.Identifier.Value` → yerelde "111 read", temiz kopyada ve CI'da "103 read". Fark yerelde duran, izlenmeyen `scratch/libcodes/*.go`'dan.
  - İkinci yol: `git check-ignore -v scratch/libcodes/main.go` → `.gitignore:25:scratch/`.
  - Neden kritik: ortama bağlı düşen depo testi (planlama sorusu 161 yerelde geçti, CI'da kaldı); yok sayılan kodun kullanımları ve testleri cevaba karışıyor.
  - Beklenen: dizin yalnız git'in izlediği ya da yok saymadığı dosyaları okur. Sınıfı koruyan test: `.gitignore`'daki bir klasörde duran Go dosyası grafiğe girmez.
- **Kitaplık dosyasını test dizesindeki `aktar "tr/..."` ile koşan testler o dosyaya bağlanmıyor.** 2026-10-26, ana oturumun alt ajanı bildirdi (madde 15 adım 3a ölçümü), ana oturum main e2312bf3'te yeniden üretti. CODEGRAPH'ta (2026-10-26).
  - Komut: `go run ./tools/codegraph tests-for --all lib/hazir.tr` → `e2e.TestReadyModelMatchesTransformers` yok. Gelenler `TestExamplesCoverLibraryNames` ve `TestRepoFilesEnglishLossless` gibi dosyaları gezen testler.
  - İkinci yol: `cmd/tr/e2e/behavior_ready_model_test.go:16` test dizesi `aktar "tr/hazir"` ile başlıyor; test `lib/hazir.tr`'nin asıl davranış testi (transformers'la logit karşılaştırması).
  - Aynı sınıf (alt ajan): `lib/egitim.tr` ve `lib/katman.tr` için `TestGPTTrainingScriptSmoke` ve `TestLibraryModuleCheckAndBuild` görünmüyor. `tr` ikilisini ya da `tr/` önekli aktarımı koşan testler kitaplık dosyasına bağlanmıyor.
  - Neden kritik: kanca `lib/*.tr`'nin ilk düzenlemesinde bu cevabı gösteriyor; adım 3a kitaplığın her dosyasına dokunacak ve asıl testi koşmadan commit'leyebilir (yanlış "yok").
  - Kör nokta ön denetime de geçiyor (MEKANİK bildirdi, düzeltmeler-4): ön denetim değişen dosyanın testlerini `codegraph changed --go-tests` ile seçiyor (ef1c8792). `lib/` değişince bu üç test seçilmiyor.
  - Beklenen: test dizesindeki (ve testin `tr` ikilisine verdiği betikteki) `aktar "tr/X"` kitaplığın `lib/X.tr` dosyasına test bağı kurar. Sınıfı koruyan test: `lib/` altındaki her dosyanın en az bir davranış testi `tests-for` cevabında. İnce ayardaki "test dizesindeki TR kodu" maddesiyle aynı kök.

- **VS Code eklentisinin JS testlerinin okuduğu veri dosyaları bir teste bağlanmıyor.** 2026-10-26, ana oturum yaşadı (tam denetim, `TestEveryTrackedDataFileHasATestOrAReason`).
  - Komut: `go run ./tools/codegraph tests-for editors/vscode/l10n/bundle.l10n.tr.json` → test yok.
  - İkinci yol: `editors/vscode/test/l10n_test.js:39` dosyayı okuyor; onu `cmd/tr/e2e` `TestExtensionJSTests` koşuyor.
  - Geçici: dosya `testdata/uncovered_data.txt`'e yazıldı, `maxUncoveredData` 9 → 10 ("yalnızca küçülür" kuralına aykırı). `editors/vscode/*.json` satırı aynı sınıftan.
  - Beklenen: eklentinin JS testinin okuduğu dosya (`readJSON(...)`, `path.join(ROOT, ...)`), o testi koşan Go testine (`TestExtensionJSTests`) bağlanır; iki istisna satırı silinir, sınır 8'e iner.

## İnce ayar

- **Birim testi uzak sırada: test dizesindeki TR kodu.** 2026-10-26, MEKANİK bildirdi, ana oturum main a2e94151'de yeniden üretti.
  - Komut: `go run ./tools/codegraph tests-for --all internal/lang/interp/stats.go` → aynı paketin `TestStatsNameTheBadValueByItsPlace`'i (`stats_test.go:12`) 57. sırada, `2 steps [INFERRED]`; ilk 12'de yok.
  - Kök (MEKANİK): test yerleşiğe Go'dan değil test dizesindeki TR kodundan varıyor (`New().Run(... standart_sapma(...))`); CLAUDE.md'nin andığı kör nokta.
  - Beklenen: aynı paketin test dosyasındaki, dosyanın yerleşiğini TR kodu üzerinden çağıran test `direct` ya da en üstte.

- **`cmd/tr/main.go`'ya `.tr` testlerinden kuşkulu bağ.** 2026-10-26, MEKANİK bildirdi, ana oturum main a2e94151'de yeniden üretti; kenarın doğruluğu doğrulanmadı.
  - Komut: `go run ./tools/codegraph tests-for cmd/tr/main.go` → `tipler_test: 3.11 ...`, `tensor_test: 15.44 ...`, `moduller_test: 11.7 ...`, `hatalar_ertele_test: 9.4 ...`, `yerlesikler_test: 13.5 ...` `1 steps [INFERRED]`.
  - Gelen bağ: bu testler `tr test` ile süreç içinde koşuyor; MEKANİK kaynakta `main.go`'nun işlevlerini çağıran bir yol görmedi. `query tests/tipler_test.tr:219` yalnız yorumlayıcıya `implemented_by` kenarları gösteriyor.
  - Beklenen: kenarın hangi kuraldan geldiği; yanlışsa sınıfı kritik (yanlış kenar, "bu testler main.go'yu korur" yanılgısı).

- **`compares_value` noktalama gürültüsü.** 2026-10-26, CODEGRAPH bildirdi, ana oturum main d4de3803'te yeniden üretti. **Bekliyor: çekirdek değişikliği** (karar ana oturum, 2026-10-26).
  - Komut: `go run ./tools/codegraph who-uses lexer.TokDot` → `compares_value (17)`, çoğu dosya yolu ve bayrak kodu (`other_os.go:42`, `docs.go:127`, `rules.go:113` ...); `"."`, `"-"`, `"+"`, `"/"` için aynı.
  - Beklenen: yalnız işleci kastedenler. Kök (DENETİM): `TokDot` tipsiz bir sabit, asıl kullanımı `TokenType` tipli. Çözüm: `compares_value` kenarı yalnız karşılaştırılan ifadenin tipi sabitin gerçek kullanımlarında aldığı tiplerden biriyse kurulur (go/types `Info.Types`). Test: `TokDot`'un `compares_value` kenarlarının hepsi `TokenType` tipli ifadelerde. Fazla tahmin, yanlış "yok" değil: ince ayar.
  - CODEGRAPH ölçtü: tip süzgeci tek başına doğru kenarları da düşürür, çünkü AST'nin işleci de metin (`ast.go:62, 295, 306` `Operator string`; `interp.opCode(op string)`, denetleyicinin `e.Operator` karşılaştırması string tipli). Karar: önce işleç alanları `lexer.TokenType` olur (Go'nun `go/ast.BinaryExpr.Op token.Token` gibi; DENETİM go/types ile 10 dosyada 25 kullanım saydı, taşıma; değişmezler sabite çevrilmez), sonra süzgeç eklenir. Çekirdek değişikliği madde 15 adım 2'den sonra.

- **README'nin süre ve boyut sayıları eskimiş.** 2026-10-26, ana oturum ölçtü (main 798dbdae, bilgisayar meşgulken).
  - `tools/codegraph/README.md` "Time": "about 640 Go files, 9 000 nodes, 36 000 edges, `build` about 3 s". Bugün 24 852 düğüm, 172 079 kenar, `build` kendi saydığı 9,3 s (duvar saati ~13 s), en çok ~460 MB bellek. "Cost" bölümü: `tests-for` 0,95 s yazıyor; bugün güncel grafta 1,4–1,8 s (`--no-rebuild` 1,15 s, `graph.json` 61 MB).
  - Beklenen: sayılar ölçümle güncellenir ya da bir testin ürettiği sayıya bağlanır (sayının eskidiği görünür olsun). Kancanın bayat grafta beklettiği süre (~16 s [tahmin: parçaların toplamı]) de yazılır.

- **Planlama sınavının bazı soruları kodun yerine değil yazımına bağlı; ön denetim sınavı koşmuyor.** 2026-10-26, MEKANİK bildirdi (adım 2 devri), ana oturum üç kez yaşadı.
  - Sorular satır numarasına ya da sayıya bağlı:
    - `planning_questions.txt` `language.go:218`'i soruyordu; `codes` dosyasına üstten kod eklenince sorular kaldı (MEKANİK 8bf9ee84'te düzeltti).
    - Bir soru `language_test.go` için tam "this file's tests (16)" bekliyordu; adım 2 üç test ekleyince kaldı (4f6e52b9).
    - Tam yerine kesilen grupta beklenen bir test 13. sıraya kaydı (877760db, `--all` ile düzeldi).
  - Ön denetim (`--kip=on`) `tools/codegraph`'ın sınavını koşmuyor: kırık soruyla "✅" verdi; kırılma ancak tam denetimde ve birleşmeden sonra görüldü.
  - Beklenen:
    - Sorular satır yerine bir metin kalıbına bağlanır, satır test anında kalıptan hesaplanır (DENETİM, adim2-dogrulama-sonuc.md), ya da bir bildirimin içindeki yeri sorar; sayı yerine özelliği sınar (grup var, belli bir test grupta).
    - Ön denetimin kapsamı ayrı iş (`scratch/mekanik_bekleyenler.md` C15: testleri `tests-for`/`changed` cevabından seçmek); bu madde yalnız soruların biçimi.
  - Yanlış "kaldı"ya ve gecikmiş kırılmaya götürüyor, yanlış cevaba değil: ince ayar.

- **`who-uses`'ta yapı köprüsünün `implemented_by` kenarları çağıranları boğuyor ve hangi yapıdan geldiklerini söylemiyor.** 2026-10-26, DENETİM bildirdi (adim3-kararlar-sonuc.md), ana oturum main e2312bf3'te yeniden üretti.
  - Komut: `go run ./tools/codegraph who-uses interp.memberKeys` → `calls (1)` (`compiler.member`), ardından `implemented_by (1175)` `[INFERRED]`: `06_girdi.ad`, `08_fonksiyonlar.faktöriyel` ...
  - DENETİM bunu yanlış kenar sandı. Ana oturumun bulgusu: kenarlar `dispatch.go`'nun kuralıyla doğru. `memberKeys`'i `compiler.member` çağırıyor; onu `*ast.MemberExpression` dalları (`compiler_expr.go:209`, `compiler_assign.go:71`, `compiler_call.go:19`) çağırıyor. Üye erişimi kullanan her TR bildirimi bu yüzden ona bağlanıyor. `who-uses interp.compiler.member` aynı 1175'i veriyor.
  - Sorun gösterim: dikkatli bir okuyucu bile doğru kenarı yanlış sandı. Kenar hangi dil yapısından geldiğini (`MemberExpression`) yazmıyor. 1175 TR bildirimi Go çağıranlarıyla aynı ağırlıkta sıralanıyor.
  - Beklenen: `implemented_by` grubu yapının adıyla tek satır özet olur (`via MemberExpression: 1175 TR declarations`), liste `--all` ile açılır. Yanlış değişikliğe götürmüyor: ince ayar.

- **Düzenlenen dil paketi derlenmezken codegraph cevap veremiyor.** 2026-10-26, MEKANİK bildirdi (düzeltmeler-4 devri). Ana oturum yeniden üretmedi; kök kaynaktan belli.
  - Durum: `internal/lang/checker`'ın derlenmeyen bir ara hâli (`checker.go` yeni imzayı çağırıyor, `block_values.go` yarım). Kanca `go run ./tools/codegraph tests-for` çalıştırdı ve "too many arguments in call to c.inferIf" ile düştü.
  - Kök: codegraph TR tarafı için dil paketlerini içe aktarıyor (`go list -deps ./tools/codegraph`). Düzenlenen paket derlenmezken araç kendisi derlenmiyor.
  - Bu, kritik bölümünden kapanan "dil dosyası bozukken cevap" maddesinin (5d5fa022) Go tarafındaki eşi.
  - Beklenen: Go grafiği dil paketleri derlenmeden kurulabilir. Cevap, TR tarafının neden eksik olduğunu söyler. Yanlış cevap değil, cevapsızlık: ince ayar.

## Kalan işler (eksik değil, yeni yetenek)

- `.tr` kodunda tekrar bulma (`dups`'ın `.tr` karşılığı; `tr dizin kopya` bugün yok, 2026-10-25'te denendi).
- **Kullanım yerinin rolü (istek, 2026-10-26, madde 15 tasarımı):** `who-uses` bir adın her kullanımını aynı listede veriyor; kullanımın ne işe yaradığını ayırmıyor. İç dil tasarımında `codes.Name`'in ~90 kullanımından hangisinin kimlik (karşılaştırma, harita anahtarı, `switch`, kayıt: `Define`/`Register`), hangisinin görüntü (ileti ya da `fmt` argümanı) olduğu kaynak okunarak ayrıldı. İstek: her kullanım yerine bir rol etiketi (görüntü / kimlik / kayıt), `who-uses --role` gibi süzme. Görüntü ve dosya biçimi rolleri d4de3803 ile kenar türü oldu (`formats`, `encodes`, `stdlib_calls`); kimlik ve kayıt rolleri ve süzme kalıyor (`compares_value` kimliğin metin tarafı).
- **Arayüz kullanımlarının kalan iki yolu (CODEGRAPH bildirdi, 2026-10-26; ana oturum henüz yeniden üretmedi, sınıfı üretilince verilir):** `...any`'yi `fmt`'ye geçen modül işlevleri (vet'in printf sarmalayıcı kuralı) bugün yalnız `messages` paketi için biçimleme sayılıyor; `error` olarak döndürülüp başka yerde biçimlenen değerin `Error()`'ı çağıransız.
- `dead-methods` ve `package-state`'in ilk koşusunun bulgularının ayıklanması (sayılar 2026-10-26, DENETİM: 56 yalnızca arayüzle ya da yalnızca testte yaşayan metot, 120 paket değişkeni; iş açılmadan önce yeniden koşulur); korumasız görünenler: `tensor.backends`, `tokenize` kayıt haritaları, `codes.defs`, `messages.catalogs`, `interp.engines`, `logging` açık çubuklar ve günlükler, `cmd/tr.exitCode`.
- **İşlev değeri argüman olarak geçince test zinciri kopuyor (CODEGRAPH bildirdi, 2026-10-26; bilinçli sınır, ana oturum yeniden üretmedi):** `f(..., buildLanguage)` gibi bir çağrıda işlev değeri INFERRED `calls` kenarıdır ve `reachEdge` onu izlemez; yüksek dereceli işlevlerde "listeyi gezen test" ve dolaylı testler kaybolur. CODEGRAPH ince ayar 3'te `startLanguages`'ı işlev yerine veri alacak biçimde değiştirerek kendi örneğini kapattı. Genel çözüm: argüman olarak geçen modül işlevi, onu çağıran parametre üzerinden gerçek bir çağrı sayılır.
- **Hız (karar 2026-10-26: şimdi yapılmaz).** DENETİM ölçtü (codegraph-karar-ve-hiz-sonuc.md, yük altında): kurulum 13,3 s, bunun 5,7 s'si iki `go list` çağrısı, tip denetimi 1,8 s, `extractGo` 1,6 s; güncel grafta soru tabanı `graph.json`'un okunması ~1,1 s. Ana oturumun ölçümü: o gün üç oturumda ~70 ilk düzenleme; kazanç günde birkaç dakika, önbellek hatasının riski (bayat ya da yanlış cevap) buna değmedi. Yapılacaksa sıra: `go list` önbelleği → grafın okunması → ağaçlar arası ortak graf → kancanın bayat ve etiketli cevabı → Go tarafını yeniden kullanmak; `go run`'ı kaldırmak ve paket başına artımlı tip denetimi kazandırmaz. **Yeniden açma eşiği:** kurulum 30 s'yi ya da güncel grafta bir soru 3 s'yi geçerse (graf 2026-10-23'te ~9 bin, 2026-10-26'da ~25 bin düğüm).
