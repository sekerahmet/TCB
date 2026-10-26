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

Son kapananlar (ana oturum 2026-10-26 aynı komutlarla doğruladı, main ed69e06a): kitaplık dosyasını `aktar "tr/..."` ile koşan testlerin bağlanması (`tests-for lib/katman.tr` → `TestLibraryModuleCheckAndBuild`, `lib/hazir.tr` → `TestReadyModelMatchesTransformers`); ince ayardan `stats.go`'nun testinin uzak sırası (artık 5. ve `direct`), `main.go`'ya `.tr` testlerinden kuşkulu bağ, yapı köprüsünün `implemented_by` gürültüsü, derlenmeyen dil paketinde cevapsızlık (son derlenen ikili, 98fcbb19), README sayıları (be65d001), sınav sorularının satır ve sayıya bağlılığı (aa0b2a14).

- **`tr dizin` git'in yok saydığı `.tr` dosyalarını dizinliyor.** 2026-10-26, ana oturum gerçek kullanım denemesinde yaşadı (main ed69e06a). CODEGRAPH'ta (2026-10-26). Go tarafı kapandı (5ce7ac39: yok sayılan klasördeki paket grafiğe girmiyor); `.tr` dizini açık.
  - Komut: `tr dizin tanım GPT` → 3 aday; biri `scratch/gpt30m/gpt30m.tr:81` (kimlik `struct:scratch/gpt30m/gpt30m.tr#TR_type_GPT`). İngilizcede de (`TR_DIL=en tr index definition GPT`).
  - İkinci yol: `git check-ignore -v scratch/gpt30m/gpt30m.tr` → `.gitignore:25:scratch/`.
  - Neden kritik: yerel cevap temiz kopyanınkinden farklı; ad sorusu yok sayılan dosyayla belirsizleşiyor, kişinin kendi denemesi kitaplığın bildirimiyle aynı sırada çıkıyor.
  - Beklenen: codeindex yalnız git'in izlediği ya da yok saymadığı `.tr` dosyalarını okur (Go tarafıyla aynı kural, tek yerde). Test: yok sayılan klasördeki `.tr` dizine girmez.

- **`init()`'in okuduğu gömülü dosya, `init()`'in doldurduğu değişkeni kullanan testlere bağlanmıyor.** 2026-10-26, ana oturum yaşadı (tam denetim ve gerçek kullanım denemesi).
  - Komut: `go run ./tools/codegraph tests-for tools/gen/snippets/data.json` → yalnız `TestSnippetDataIsLanguageNeutral` (gömülü değişkeni doğrudan okuyan test).
  - İkinci yol: `tools/gen/snippets/templates.go` `init()` `dataJSON`'dan `templates`'i, `text/*.json`'dan `texts`'i dolduruyor; `TestSnippetsUpToDate`, `TestSnippetsEveryLanguageEveryTemplate` ve `TestSnippetsParseInEveryLanguage` `Generate` üzerinden bu değişkenleri okuyor: dördü de bu dosyalara bağlı. `text/*.json` hiçbir teste bağlanmıyordu (`TestEveryTrackedDataFileHasATestOrAReason` kaldı; f40e9f64'de testin `textFiles`'ı doğrudan okumasıyla geçici olarak bağlandı).
  - Neden kritik: `tests-for` dört testin üçünü göstermiyor; kanca ve ön denetim veri dosyası değişince bu testleri seçmiyor (yanlış "az test").
  - Beklenen: `init()` (ve paket düzeyi değişken başlatıcısı) bir dosyayı okuyup bir değişkene yazıyorsa, o değişkeni okuyan testler o dosyaya bağlanır. Test: go/types ile, `init()`'te gömülü dosyadan doldurulan her değişken için okuyan testler `tests-for` cevabında.

- **`TestBuildScalesLinearly` CI'da art arda düşüyor.** 2026-10-26, ana oturum CI'da yaşadı (uzun testler işi; üç koşu).
  - Gelen: 14:29 "sıfırdan kurulum" oranı 9,2; 17:24 "yenileme" 10,6; 19:34 (08b32ffc) "önbellekten okuma" 8,7 (sınır 8, doğrusal ~4); öbür aşamalar da sınırın dibinde (7,5 ve 7,8). Yerelde (Windows) geçiyor.
  - İkinci yol: `gh run view 40387943068 --log-failed`; `internal/devtools/codeindex/cache_test.go:340-349`.
  - Neden kritik: ortama bağlı düşen depo testi; ana dal üç gönderimdir kırmızı.
  - Beklenen: önce ölç: büyüme gerçekten doğrusalın üstündeyse (100 → 400 modülde ~8 kat) kökü bulunur ve düzeltilir; değilse ölçüm gürültüye dayanıklı olur (birkaç tekrarın en iyisi, ısınma) ve gerekçesi testte yazılır. Sınır gevşetilerek geçirilmez.

- **VS Code eklentisinin JS testlerinin okuduğu veri dosyaları bir teste bağlanmıyor.** 2026-10-26, ana oturum yaşadı (tam denetim, `TestEveryTrackedDataFileHasATestOrAReason`).
  - Komut: `go run ./tools/codegraph tests-for editors/vscode/l10n/bundle.l10n.tr.json` → test yok.
  - İkinci yol: `editors/vscode/test/l10n_test.js:39` dosyayı okuyor; onu `cmd/tr/e2e` `TestExtensionJSTests` koşuyor. (JS'nin `require` ile yüklediği dosya bağlanıyor: `tests-for editors/vscode/coded_format.js` → `TestExtensionJSTests`; `readJSON('l10n', ...)`/`path.join` ile okunan bağlanmıyor.)
  - Geçici: dosya `testdata/uncovered_data.txt`'e yazıldı, `maxUncoveredData` 9 → 10 ("yalnızca küçülür" kuralına aykırı). `editors/vscode/*.json` satırı aynı sınıftan.
  - Beklenen: eklentinin JS testinin okuduğu dosya (`readJSON(...)`, `path.join(ROOT, ...)`), o testi koşan Go testine (`TestExtensionJSTests`) bağlanır; iki istisna satırı silinir, sınır 8'e iner.

## İnce ayar

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
