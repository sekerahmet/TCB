# BİTTİ

# DURUM — Faz 0.5: görünümden geri kodlamanın kaldırılması (17.7), örneklerin tanım dosyasına taşınması (17.4)

- **Başlama:** 2026-10-29 18:57 UTC · **Bitiş:** 2026-10-29 19:50 UTC
- **Dal:** `claude/serene-pascal-rbkqwf` (taban `main` 846379d)

## Biten adımlar (commit sırasıyla)

1. **Örneklerin taşınması** (`1e8e0b3`, 19:07): 320 örnek betikle (`.devir/move_examples`, o günkü `source.Encode`; elle çevrilmedi) `definitions.json`'a (`example`, satırın son alanı) kimlik yazımıyla taşındı; `tr.json`'da ve `en.json`'da `"example"` yok (`en.json`'da zaten yoktu), dil dosyası ayrıştırıcısı alanı reddediyor. Komut satırı örnekleri kimliğin adıyla: `tr check eğit.tr`, `--go-profile`, `TR_LANG=en tr help`; kabuk `exit`, hata ayıklayıcı `quit` (ikisinin de Türkçesi `çıkış`; betik adı kendi sınıfında arıyor). `TestExamplesRun` örnekleri `lexer.NewBody` ile okuyup denetliyor ve çalıştırıyor. `docs/codes.md` örneği kimlik yazımıyla, altında her gömülü dilin görünümüyle (`en:`, `tr:`) gösteriyor.
   - Bugün tanım dosyasında **319** örnek var: kodlama komutunun kaydıyla birlikte örneği de silindi (2. adım). 265'i program örneği (testte çalışıyor), 54'ü komut satırı örneği (program değil; önceki gibi atlanıyor).
2. **Kodlamanın kalkması** (`e56be1b`, 19:31):
   - `tr kodla` (kodu ve kaydı silindi), `cmd/tr/coded.go` → `cmd/tr/show.go` (yalnız `tr göster`); `source.Encode`, `EncodeIn`, `TextLexer`, `TextLexerWithNames`, kabuk satırının görünümü (`Options.Typed`), gösterimin geri kodlama denetimi (H2543); `convert.go` → `show.go` (`Show`, `ShowBody`).
   - Kabuk satırı `Options.Body` ile, hata ayıklayıcının ifadesi `source.TypedLexer` ile kimlik yazımıyla okunuyor (`yazdır(x)` tanımsız ad hatası, `TR_print(x)` çalışıyor: `TestCLIExpressionIsWrittenWithIdentities`, `TestTypedLineIsReadWithIdentities`, `TestHeaderlessFileDoesNotRun`).
   - `Parts.IdentityLexer` ile `Parts.Lexer` birleşti: başlıksız gövde kodlu okunuyor (`TestLexerReadsIdentitiesAtDiskPositions`).
   - Görünüm başlıklı dosya çalıştırılınca ve `tr kontrol`'de H2541 kodlu dosyayı çalıştırmayı söylüyor (`TestViewHeader`, `TestShowCommand`); başlıksız dosyanın iletisi H2540 başlığı ve kimlik yazımını söylüyor, kodlamayı önermiyor.
   - `tools/migrate` (eski biçimden yeniye tek seferlik çevirici) silindi: kodlayıcıyla çalışıyordu; main'de bile plan kipi 79 dosyayı "çevirmek" istiyordu (Go test dizelerini Türkçe görünüm sayıyor).
   - Eklenti şablonlarının gövdesi her dilde kimlik yazımı (`tools/gen/snippets`): şablon kodlu dosyaya yazılır.
   - Go testlerindeki görünümde yazılmış programlar kimlik yazımına geçti; dönüşümü yine betikle yaptım: main'in kopyasında kodlayıcıya kayıt koydum, ilgili testleri koşturup girdi→çıktı çiftlerini topladım, çiftleri testlere uyguladım (elle çevirme yok).
3. **Eklenti** (`44517d4`, 19:35): `coded_view.js`, `coded_filesystem.js`, `coded_format.js` ve `coded_view_test.js` kalktı; `view.js` görünümü yanda salt okunur açıyor (`tr.showView`, metin belgesi sağlayıcısı, `tr göster - --dil <dil>`), dosya düzenlenince ve yazım dili değişince yenileniyor; görünümden çalıştırma dosyada çalışıyor; kısıtlı kipte açılmıyor. Testler `view_test.js` (9/9), `snippets_test.js` güncel; paket 1.9.0. Komut: `node editors/vscode/test/<ad>_test.js` (yorumlayıcı `TR_YORUMLAYICI` ile); `go test ./cmd/tr/e2e -run TestExtensionJSTests` hepsini koşuyor, geçiyor.
4. **Sözcük çözümleyici ve codegraph** (`349bcdd`, `6816f48`): `lexer.NewBodyView` ve sütun kaymaları (yazılan metnin görünümdeki konumları) kalktı; codegraph'ın TR metni okuyan işlev listesi (`trSinks`) bugünkü adlara (`Show`, `ShowBody`, `TypedLexer`) çevrildi, `extractorRevision` 74.
5. **Belgeler** (`3526b35`): kılavuz (01, 08), `SECURITY.md`, mimari, geliştirme, tarifler (§9), terimler (eş ad satırı), örneklerin komut listesi, eklenti README'si; status 2.1, 17.4, 17.7; dil tanımının 17.4 ve 17.7 *Test:* satırları yeni testleri anıyor.

## Bitiş ölçütü

- `go build ./...`, `go vet ./...`: temiz.
- `go test ./internal/... ./cmd/... ./tools/...`: yalnız `TestEveryTrackedDataFileHasATestOrAReason` düşüyor (`.devir/DURUM.md`, beklenen; `.devir/` alınmıyor). Öbür her paket geçiyor (eklentinin JS testleri dahil, `TestExtensionJSTests`).
- `go run ./tools/codegraph spec-refs`: "278 item(s), 3672 mention(s), 0 error(s), 0 note(s)".
- `go run ./tools/codegraph dups --changed main`: 0 aday.
- `gofmt -l`: boş.
- Bitti ölçütündeki git grep: boş; yalnız arşiv ve donmuş eşleme tablosu kalıyor (Ölçüler).
- `tr.json`'da ve `en.json`'da `"example"` yok; `definitions.json`'da 319 örnek kimlik yazımıyla (320 taşındı, kalkan kodun örneği onunla silindi); 265 program örneği `TestExamplesRun`'da `lexer.NewBody` ile okunup çalışıyor.
- Hata ayıklayıcıda `yazdır(x)` hata, `TR_print(x)` çalışıyor: `TestCLIExpressionIsWrittenWithIdentities` (debugger), DAP'ta `TestDAPExpressionsAreWrittenWithIdentities`.
- Görünüm başlıklı dosya çalışmıyor, ileti çıkıyor: `TestViewHeader` (source), `TestShowCommand` (e2e, çalıştırma ve `tr kontrol`).

## Ölçüler

- `git grep -n 'TextLexer' -- '*.go'`: önce **16** satır (10 dosya), sonra **0**. Kalan yer yok.
- Bitti ölçütündeki üç adlık git grep (kodlama komutunun kimliği, iki eklenti dosyası): yalnız `docs/archive/` (2 satır) ve `docs/code_identities.md:126` (eski kodu kimliğe eşleyen donmuş tablo; `TestClassedIdentitiesAreLive`'ın izin listesinde "silinen kimlikleri bilerek taşır"). İleti kodlarının `retired.json`'unda kalkan iletiler var.
- `internal/lang/source`: 1 870 satırdan 1 441 satıra (testler dahil).

## Kararsız kaldıklarım

1. **İleti numaraları:** yeni numara almadım. H2540 (başlıksız dosya) ve H2541 (görünüm dosyası) kalıyor, metinleri ve parametreleri değişti (README: "Kod değişmez, metin değişebilir"): H2541 artık "görünüm yalnız okunur, çalışmaz ve koda geri çevrilmez; görünümün yazıldığı kodlu dosyayı çalıştırın (görünüm ondan 'tr göster' ile yazılır)" diyor. H8875–H8879 ve H0299 boş. Ayrı numara isteniyorsa H2541 → H8875 taşınabilir.
2. **Emekliye ayrılan iletiler** (`retired.json`, commit alanı dal adı): H2115, H2511, H2542, H2543, H8620, H8624, H8681, H8703, H8704, H8706, H8707. H2510 ve H2514 kaldı (görünüm de kullanıyor), metinleri "çeviri" yerine görünümü söylüyor.
3. **"`tr help` yazılacak adları gösterir":** yardım artık kalkan komutu (iki satır) göstermiyor. Komut adlarının kimliğin adıyla gösterilmesini 17.9'un (Faz 0.4) işi saydım, dokunmadım.
4. **Komut satırı örnekleri** (`tr check eğit.tr`): 17.9'un yazımıyla kimliğin adıyla yazıldı (bugün de çalışıyor: İngilizce adlar kabul ediliyor). 0.4'ten sonra da doğru kalır.
5. **`docs/codes.md` biçimi:** örnek kimlik yazımıyla bir ` ```tr ` bloğunda, altında "Görünümü:" ve her gömülü dilin satırı bir ` ```text ` bloğunda. Komut satırı örneğinin görünümü yok (girdi kimliğin adıyla).
6. **Şablonlar:** gövde her dilde aynı kimlik yazımı; kısayol (fonksiyon, func), ad ve durakların varsayılan sözcükleri dilde. Kimlik yazımını (`TR_func`) ayrıca kısayol yapmadım.
7. **Silinen iki sınav testi:** `TestDisplayExam` (`cmd/tr/e2e/behavior_display_language_test.go`, `testdata/display_known.txt`) ve `TestNameClashExam` (`internal/stdlib/behavior_name_clash_test.go`, `testdata/name_clash_known.txt`). İkisi de `t.Skip`'liydi ("bölüm 17 yeniden yazılınca baştan kurulur") ve öncülleri bir yazım dilinde yazılmış dosyaydı (dosyalar Türkçe görünümde yazılıp kodlanıyordu); kodlayıcı kalkınca derlenmiyorlardı. Yeniden kurulurken kodlu dosyalarla yazılmalı.
8. **Görünümün `$ad` işareti** kaldı (17.7: "işaretleyebilir ama bu bir kural değildir"); okuyan taraf (`$`'ı okuyan sözcük çözümleyici kipi) aşağıdaki 9'da.
9. **Kalan: sözcük çözümleyicinin yazım dilinde okuma kipi.** `lexer.NewBodyIn(…, false, dil)` (dilin anahtar kelimeleri, `$ad` kaçışı, `Token.Escaped`), `Lexer.WrittenIn`, ayrıştırıcının dildeki bağlamsal sözcükleri, denetleyicinin `SetFileLanguage`'ı (gölgeleme uyarısı H1009) üretim kodunda artık kullanılmıyor; yalnız Go testleri (31 yer: `internal/lang/{lexer,parser,checker}`) Türkçe görünümde yazılmış parçaları onunla okuyor. Görünüm (`Show`) kodlu okuyor, bu kipe ihtiyacı yok. Kaldırmak 31 test yerinin kimlik yazımına geçmesini ve H1009'un emekliye ayrılmasını ister; bu işin madde listesinde yoktu, ayrı iş olarak önerdim (aşağıda "Sorular" 2).
10. **Kitaplık kapsam testi** (`TestExamplesCoverLibraryNames`) kitaplığın ve örneklerin Türkçe görünümünü metin olarak tarıyor (anahtarlar Türkçe adlar). Bu görünümü koda geri çevirmek değil, adı metinde aramak; bıraktım.
11. **Hata ayıklayıcı:** `parseFirstExpression`'ın kullanılmayan `env` parametresini kaldırdım. DAP'ın `viewURI` desteği (Go tarafı, `TestDAPCodedFileView`) duruyor; eklenti artık vermiyor.
12. **İpucu eksiği (karar sizin):** kabukta İngilizce `var` yazınca ileti `TR_var`'ı öneriyor (kimliğin `TR_`'siz adı), Türkçe `değişken` yazınca öneri yok. Kişinin yazım dilindeki adı tanıyıp kimliği önermek (2.3a'ya ek) yararlı olur.
13. **Kökteki `tr` ikilisi:** iki kez `go build ./cmd/tr` ile kökte `tr` oluştu; ilki `e56be1b`'ye girdi, `06d9be9` siliyor (ayrıntı: cherry-pick notları). `.gitignore` kökteki `tr`'yi yok saymıyor; `/tr` eklenmesi önerilir (dokunmadım).

## Durma noktaları

- **Görünümü okumaya gerçekten ihtiyaç duyan yer:** üretim kodunda yok. Kalan okuma kipi yalnız testlerde (Kararsız 9).
- **Göç aracı:** depoda görünüm başlıklı (`# TR 4 view=`) dosya **0**; başlıksız `.tr` dosyası 2, ikisi de bilerek bozuk olumsuz fikstür (`tests/negative/basliksiz.tr`, `tests/negative/kodlu_surum.tr`) [ölçüldü: `find … -name '*.tr'` ve başlık araması]. Depo için araç gerekmiyor. Elinde görünümde yazılmış dosya olan kişi için karar sizin; gerekirse main'deki son `tr kodla` (846379d) tek seferlik kullanılabilir.

## Cherry-pick notları

- `1e8e0b3` betiği (`.devir/move_examples/main.go`) de taşıyor: `git cherry-pick -n 1e8e0b3 && git rm -rq --cached .devir && git commit -C 1e8e0b3`.
- `e56be1b` yanlışlıkla kökteki derlenmiş `tr` ikilisini (16 MB) ekledi, `06d9be9` siliyor. İkilinin main'in geçmişine girmemesi için: `git cherry-pick -n e56be1b && git rm -q --cached tr && rm -f tr && git commit -C e56be1b`, sonra `06d9be9`'u atlayın. Geçmişi yeniden yazmak (force push) izin almadı.
- `.devir/` commit'leri: `d096811`, `e831e58`, `429a5f4`, `6e6922a`, son DURUM commit'i.
- Kod commit'leri sırasıyla: `1e8e0b3`, `e56be1b`, `44517d4`, `349bcdd`, `3526b35`, `6816f48`.

## Sorular

1. H2541'i yerinde mi bırakalım, H8875'e mi taşıyalım (Kararsız 1)?
2. Sözcük çözümleyicinin yazım dilinde okuma kipinin (Kararsız 9) kaldırılması ayrı bir iş olarak açılsın mı? (Önerdim.)
3. Kabukta kişinin dilindeki adı tanıyıp kimliği öneren ipucu (Kararsız 12) istenir mi?

## Codegraph'ın yanlış, eksik ya da yanıltıcı cevapları

1. **`dups --changed main` çalışmadı.** Komut: `go run ./tools/codegraph dups --changed main`. Beklenen: dalın main'e göre değişen düğümlerinde aday listesi. Gelen: `codegraph: git merge-base main HEAD: exit status 1` (açıklamasız). Doğrusu: yerel `main` dalı eski bir commit'teydi (23e7be2) ve sığ klonda ortak ata yoktu; `git branch -f main origin/main` sonrası "0 duplicate candidate(s)". Codegraph `origin/main`'e düşebilir ya da yerel `main`'in eski olduğunu söyleyebilir.
2. **`trSinks` eski adları taşıyordu** (`tools/codegraph/tr_snippets.go`): `source.Decode`, `Encode`, `EncodeIn` vardı, `source.Show` yoktu; yani `source.Show`'a verilen TR metni ad kodlarına bağlanmıyordu (eksik `tests` kenarları). Komut: kodu okurken gördüm (`tests-for` cevabını tek tek sınamadım). Düzelttim (`349bcdd`).
3. **`tests-for` grafik yoksa çalışmıyor:** ilk çağrıda `graph.json not found; run go run ./tools/codegraph build first`. Yanlış değil, beklenen davranış; not düştüm.
