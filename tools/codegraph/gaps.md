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

Bugün açık kritik madde yok (son kapanan: kimliğe yeniden adlandırılan kodların "silinmiş" sayılması, 5d5fa022). İnce ayardan son kapananlar: test dosyasının kendi testleri ve komut testlerinin uzaklığı (372ac2ed birleşmesi; ana oturum aynı komutlarla doğruladı).

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

## Kalan işler (eksik değil, yeni yetenek)

- `.tr` kodunda tekrar bulma (`dups`'ın `.tr` karşılığı; `tr dizin kopya` bugün yok, 2026-10-25'te denendi).
- **Kullanım yerinin rolü (istek, 2026-10-26, madde 15 tasarımı):** `who-uses` bir adın her kullanımını aynı listede veriyor; kullanımın ne işe yaradığını ayırmıyor. İç dil tasarımında `codes.Name`'in ~90 kullanımından hangisinin kimlik (karşılaştırma, harita anahtarı, `switch`, kayıt: `Define`/`Register`), hangisinin görüntü (ileti ya da `fmt` argümanı) olduğu kaynak okunarak ayrıldı. İstek: her kullanım yerine bir rol etiketi (görüntü / kimlik / kayıt), `who-uses --role` gibi süzme. Görüntü ve dosya biçimi rolleri d4de3803 ile kenar türü oldu (`formats`, `encodes`, `stdlib_calls`); kimlik ve kayıt rolleri ve süzme kalıyor (`compares_value` kimliğin metin tarafı).
- **Arayüz kullanımlarının kalan iki yolu (CODEGRAPH bildirdi, 2026-10-26; ana oturum henüz yeniden üretmedi, sınıfı üretilince verilir):** `...any`'yi `fmt`'ye geçen modül işlevleri (vet'in printf sarmalayıcı kuralı) bugün yalnız `messages` paketi için biçimleme sayılıyor; `error` olarak döndürülüp başka yerde biçimlenen değerin `Error()`'ı çağıransız.
- `dead-methods` ve `package-state`'in ilk koşusunun bulgularının ayıklanması (sayılar 2026-10-26, DENETİM: 56 yalnızca arayüzle ya da yalnızca testte yaşayan metot, 120 paket değişkeni; iş açılmadan önce yeniden koşulur); korumasız görünenler: `tensor.backends`, `tokenize` kayıt haritaları, `codes.defs`, `messages.catalogs`, `interp.engines`, `logging` açık çubuklar ve günlükler, `cmd/tr.exitCode`.
- **İşlev değeri argüman olarak geçince test zinciri kopuyor (CODEGRAPH bildirdi, 2026-10-26; bilinçli sınır, ana oturum yeniden üretmedi):** `f(..., buildLanguage)` gibi bir çağrıda işlev değeri INFERRED `calls` kenarıdır ve `reachEdge` onu izlemez; yüksek dereceli işlevlerde "listeyi gezen test" ve dolaylı testler kaybolur. CODEGRAPH ince ayar 3'te `startLanguages`'ı işlev yerine veri alacak biçimde değiştirerek kendi örneğini kapattı. Genel çözüm: argüman olarak geçen modül işlevi, onu çağıran parametre üzerinden gerçek bir çağrı sayılır.
- **Hız (karar 2026-10-26: şimdi yapılmaz).** DENETİM ölçtü (codegraph-karar-ve-hiz-sonuc.md, yük altında): kurulum 13,3 s, bunun 5,7 s'si iki `go list` çağrısı, tip denetimi 1,8 s, `extractGo` 1,6 s; güncel grafta soru tabanı `graph.json`'un okunması ~1,1 s. Ana oturumun ölçümü: o gün üç oturumda ~70 ilk düzenleme; kazanç günde birkaç dakika, önbellek hatasının riski (bayat ya da yanlış cevap) buna değmedi. Yapılacaksa sıra: `go list` önbelleği → grafın okunması → ağaçlar arası ortak graf → kancanın bayat ve etiketli cevabı → Go tarafını yeniden kullanmak; `go run`'ı kaldırmak ve paket başına artımlı tip denetimi kazandırmaz. **Yeniden açma eşiği:** kurulum 30 s'yi ya da güncel grafta bir soru 3 s'yi geçerse (graf 2026-10-23'te ~9 bin, 2026-10-26'da ~25 bin düğüm).
