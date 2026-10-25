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

- **İşlecin kullanıcıları eksik: metinle karşılaştıran yerler görünmüyor.** 2026-10-25 ana oturum (tam bölme tasarımı), 2026-10-26 DENETİM asıl kökü buldu (gaps-denetim-sonuc.md), ana oturum main d9a07501'de doğruladı.
  - Komut: `go run ./tools/codegraph who-uses lexer.TokPercent` → 7 kullanım (biçimlendirici, sözcük çözümleyici, ayrıştırıcı). `query "%"` ve `query "@"` de `not found`.
  - Beklenen: işleci metniyle karşılaştıran yerler de: yorumlayıcı `internal/lang/interp/operators.go:46` (`case "%": return opMod`), denetleyici `internal/lang/checker/checker.go:357` (`case "+", "-", "*", "/", "%", "**"`). Doğrulama: kaynak.
  - Neden kritik: liste eksiksiz görünüyor (yanlış "yok"); işlecin anlamını değiştirecek bir tasarım (tam bölme, `%` işareti) yorumlayıcıyı ve denetleyiciyi göremez.
  - İstenen genel çözüm: sabitin değeriyle sorgu (`query "%"` → `lexer.TokPercent`) ve sabitin değeriyle aynı metni karşılaştıran yerlerden (`case "..."`, `== "..."`) çıkarımsal (`[INFERRED]`) kenar; işleç sabitlerinin her biri için yorumlayıcıdaki ve denetleyicideki kullanımının listede olduğunu denetleyen kapsam testi.

- **`dups --changed` commit'lenmemiş değişikliği görmüyor; kural onu commit'ten önce koşturuyor.** 2026-10-25, MEKANİK bildirdi (d7bfae3d'nin ön denetimi 0, aynı commit'te doğrudan koşu 1 aday), ana oturum main 493afbf0'da yeniden üretti.
  - Komut: geçici çalışma ağacında `internal/lang/turkish/frame.go`'ya `codeFrameAt`'in birebir kopyası (`codeFrameAtCopy`) eklendi; commit'lenmemişken `go run ./tools/codegraph dups --changed origin/main` → `0 duplicate candidate(s)`; aynı değişiklik commit'lenince → `1 duplicate candidate(s)`, `#1 turkish/frame.go:codeFrameAt~turkish/frame.go:codeFrameAtCopy`.
  - Kök [kanıtlı: `tools/codegraph/dups_changed.go:22,36`]: değişen satırlar `ad...HEAD` farkından alınıyor (yalnız commit'lenmiş); grafik ise çalışma ağacından kuruluyor. CLAUDE.md "her commit'ten önce" ve ön denetim (`full_check.sh --kip=on`) tam da commit'ten önce koşuyor: yeni kopya o anda görünmüyor, ancak bir sonraki commit'in ön denetimi ya da tam denetim yakalıyor. Yanlış "yok".
  - Beklenen: değişen satırlar ortak atadan çalışma ağacına kadar sayılır (commit'lenmiş, sahnelenmiş ve sahnelenmemiş değişiklikler; izlenen yeni dosyalar dahil). Sınıfı koruyan test: commit'lenmemiş bir kopyanın aday olarak çıktığı.
  - DENETİM ekledi (2026-10-26, yeniden üretti): aynı kök `changed` komutunda da var (değişen kodun testleri commit'lenmemiş düzenlemeyi görmüyor); satırlar çalışma ağacından, değişiklik commit'ten alındığı için satır kayması da oluyor. `dups --changed` ve `changed` değişen satırları tek fonksiyondan almalı.

- **Standart kitaplığın arayüz ve yansıma üzerinden yaptığı kullanımlar listede yok (`fmt`, `encoding/json` ...).** 2026-10-26, DENETİM bildirdi (madde 15 adım 0 incelemesi), ana oturum main 46a095cc'de yeniden üretti; DENETİM sınıfı genişletti (gaps-denetim-sonuc.md).
  - İkinci örnek (DENETİM, ölçüldü): `who-uses profiler.Snapshot.Start` yalnız 1 bileşik değişmez gösteriyor; alan yalnız JSON ile `speed.json`'a yazılıyor. Liste "okunmuyor, silinebilir" yanılgısına götürür. `dead-methods` bu tür metotları zaten ayıklıyor: bilgi iki komutta ortak kullanılmalı.
  - Komut: `go run ./tools/codegraph who-uses codes.Code.String` → 10 açık çağıran (codeindex, `source.codedBody`, `source.langName`, testler).
  - Beklenen: değeri `%s`/`%v` ile biçimlenen yerler de; ör. `internal/lang/codes/language.go:227` ve `:236` `fmt.Sprintf("code list %s: code %s is not in the language file", d.Code, k)` (`k` bir `codes.Code`; `String()` dolaylı çağrılıyor). Doğrulama: grep.
  - Neden kritik: `String()`'in çıktısı değişince (adım 0: `A001` → `TR_kw_if`) etkilenen yerlerin bir kısmı görünmüyor; liste eksiksiz sanılıyor. Aynı sınıf `Error()` için de geçerli.
  - İstenen genel çözüm: bir tipin değeri standart kitaplığın arayüz ya da yansıma üzerinden çalışan bir yerine verildiğinde (`fmt`/`log`/`errors` biçimleyicileri `fmt.Stringer`/`error` için; `encoding/json` dışa açık alanlar için; ileti argümanları) çıkarımsal (`[INFERRED]`) bir kenar ("biçimlenir", "JSON'a yazılır"); go/types ile sayılan bu tür kullanımların hepsinin kenarı olduğunu denetleyen kapsam testi. Aşağıdaki "kullanım yerinin rolü" isteğiyle aynı kullanım türü modelinde çözülür.

## İnce ayar

- **Dosya ve satır sorulamıyor: grep'ten grafiğe köprü yok (öncelikli).** 2026-10-26, ana oturum main 465f71b3'te üretti (Ahmet'in "grep gibi bir şey eklemek mantıklıysa ekleyelim" sorusu üzerine). DENETİM sınıfını ince ayara çevirdi: araç kodun yokluğunu söylemiyor, doğru dosyayı öneriyor; yanlış bir değişikliğe götürmüyor.
  - Komut: `go run ./tools/codegraph query internal/lang/codes/language.go:227` ve `tests-for internal/lang/codes/language.go:227` → `not found` (yalnız dosya düğümünü öneriyor).
  - Beklenen: satırı içeren en içteki düğüm (Go fonksiyonu, metodu, testi; `.tr` bildirimi; `.md`'de başlık bölümü) ve onun komşuları ya da testleri, sanki adıyla sorulmuş gibi. CLAUDE.md'nin yolu "metnin içeriği grep ile aranır, sonra bulunan yer grafiğe sorulur"; bugün içeren fonksiyon elle aranıyor.
  - İstenen genel çözüm: `query`, `who-uses`, `tests-for`, `history`'nin `dosya:satır` (ve `dosya:satır:sütun`) kabul etmesi; birden çok konum için (grep çıktısı) her satıra içeren düğümü ekleyen bir kip; `.tr` için `tr dizin`'in konum çözümü (17.8) yeniden kullanılır. Codegraph'a ayrı bir metin araması eklenmez (grep/ripgrep var; Sadelik 2). Test: belirlenimci sınırlar (bildirimin ilk ve son satırı, iki bildirim arası, iç içe kapanış) go/ast'nin içeren bildirimiyle karşılaştırılır.

- **Test dosyasının kendi testleri sayılmıyor.** 2026-10-25, MEKANİK bildirdi (iki kez), ana oturum main f60cdcc6'da yeniden üretti; DENETİM 2026-10-26'da yeniden üretti ve sınıfı buldu.
  - Komut: `go run ./tools/codegraph tests-for internal/lang/checker/type_names_test.go` → `tests: none found for the declarations in this file` (dosyada bir test bildirimi var). İkinci örnek: `tests-for internal/devtools/testrunner/runner_test.go` → 10 testten 1'i.
  - Sınıf (DENETİM): yalnız dosyadaki bir yardımcıyı çağıran testler listeleniyor; MEKANİK'in `prepare_test.go` ve `testkit_test.go`'da görmemesinin nedeni bu.
  - Beklenen: test dosyasının bildirdiği testler "this file's tests" olarak listelenir; "none found" yanlış "yok"tur. Risk düşük (düzenleyen kişi testi zaten görüyor), bu yüzden ince ayar.

## Kalan işler (eksik değil, yeni yetenek)

- `.tr` kodunda tekrar bulma (`dups`'ın `.tr` karşılığı; `tr dizin kopya` bugün yok, 2026-10-25'te denendi).
- **Kullanım yerinin rolü (istek, 2026-10-26, madde 15 tasarımı):** `who-uses` bir adın her kullanımını aynı listede veriyor; kullanımın ne işe yaradığını ayırmıyor. İç dil tasarımında `codes.Name`'in ~90 kullanımından hangisinin kimlik (karşılaştırma, harita anahtarı, `switch`, kayıt: `Define`/`Register`), hangisinin görüntü (ileti ya da `fmt` argümanı) olduğu kaynak okunarak ayrıldı. İstek: her kullanım yerine bir rol etiketi (görüntü / kimlik / kayıt), `who-uses --role` gibi süzme.
- **Not, CODEGRAPH'a iş değil:** madde 15 adım 0'da kodlar okunur kimliğe geçiyor (`A001` → `TR_kw_if`); codegraph'ın kodu tanıyan üç düzenli ifadesi (`gocode_scan.go:18`, `codes.go:11`, `codehist.go:10`) MEKANİK'in yönergesinde, kapsam testiyle (her kimliğin en az bir `uses_name` kenarı). Aynı iş iki kez yapılmasın.
- `dead-methods` ve `package-state`'in ilk koşusunun bulgularının ayıklanması (sayılar 2026-10-26, DENETİM: 56 yalnızca arayüzle ya da yalnızca testte yaşayan metot, 120 paket değişkeni; iş açılmadan önce yeniden koşulur); korumasız görünenler: `tensor.backends`, `tokenize` kayıt haritaları, `codes.defs`, `messages.catalogs`, `interp.engines`, `logging` açık çubuklar ve günlükler, `cmd/tr.exitCode`.
