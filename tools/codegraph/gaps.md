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

Şu an yok: 2026-10-26 partisinin dört kritik maddesi d4de3803 ile kapandı (ana oturum aynı komutlarla doğruladı).

## İnce ayar

- **Test dosyasının kendi testleri sayılmıyor.** 2026-10-25, MEKANİK bildirdi (iki kez), ana oturum main f60cdcc6'da yeniden üretti; DENETİM 2026-10-26'da yeniden üretti ve sınıfı buldu.
  - Komut: `go run ./tools/codegraph tests-for internal/lang/checker/type_names_test.go` → `tests: none found for the declarations in this file` (dosyada bir test bildirimi var). İkinci örnek: `tests-for internal/devtools/testrunner/runner_test.go` → 10 testten 1'i.
  - Sınıf (DENETİM): yalnız dosyadaki bir yardımcıyı çağıran testler listeleniyor; MEKANİK'in `prepare_test.go` ve `testkit_test.go`'da görmemesinin nedeni bu.
  - Beklenen: test dosyasının bildirdiği testler "this file's tests" olarak listelenir; "none found" yanlış "yok"tur. Risk düşük (düzenleyen kişi testi zaten görüyor), bu yüzden ince ayar.

- **`tests-for` komut fonksiyonlarının testlerini dolaylı gösteriyor.** 2026-10-26, CODEGRAPH bildirdi (gaps-parti-devir.md), ana oturum main d4de3803'te yeniden üretti.
  - Komut: `go run ./tools/codegraph tests-for tools/codegraph/query_fields.go` → ilk satır `TestPlanningQuestionsAreAnswered 5 steps [INFERRED]`; doğrudan test yok.
  - Beklenen: komutu adıyla çalıştıran testler (`run([]string{"who-uses", ...})`) doğrudan. İkinci yol: `grep -ln '"who-uses"' tools/codegraph/*_test.go` → `fields_test.go` ve 5 dosya daha.
  - Kök (CODEGRAPH): komut adı metinle `answer`'ın `switch cmd`'ine gidiyor. Test listede var, uzaklığı yanlış: ince ayar.

- **`compares_value` noktalama gürültüsü.** 2026-10-26, CODEGRAPH bildirdi, ana oturum main d4de3803'te yeniden üretti.
  - Komut: `go run ./tools/codegraph who-uses lexer.TokDot` → `compares_value (17)`, çoğu dosya yolu ve bayrak kodu (`other_os.go:42`, `docs.go:127`, `rules.go:113` ...); `"."`, `"-"`, `"+"`, `"/"` için aynı.
  - Beklenen: yalnız işleci kastedenler. Öneri (CODEGRAPH): karşılaştırılan ifadenin kaynağına (ast/lexer tipli alan) bakmak. Fazla tahmin, yanlış "yok" değil: ince ayar.

## Kalan işler (eksik değil, yeni yetenek)

- `.tr` kodunda tekrar bulma (`dups`'ın `.tr` karşılığı; `tr dizin kopya` bugün yok, 2026-10-25'te denendi).
- **Kullanım yerinin rolü (istek, 2026-10-26, madde 15 tasarımı):** `who-uses` bir adın her kullanımını aynı listede veriyor; kullanımın ne işe yaradığını ayırmıyor. İç dil tasarımında `codes.Name`'in ~90 kullanımından hangisinin kimlik (karşılaştırma, harita anahtarı, `switch`, kayıt: `Define`/`Register`), hangisinin görüntü (ileti ya da `fmt` argümanı) olduğu kaynak okunarak ayrıldı. İstek: her kullanım yerine bir rol etiketi (görüntü / kimlik / kayıt), `who-uses --role` gibi süzme. Görüntü ve dosya biçimi rolleri d4de3803 ile kenar türü oldu (`formats`, `encodes`, `stdlib_calls`); kimlik ve kayıt rolleri ve süzme kalıyor (`compares_value` kimliğin metin tarafı).
- **Arayüz kullanımlarının kalan iki yolu (CODEGRAPH bildirdi, 2026-10-26; ana oturum henüz yeniden üretmedi, sınıfı üretilince verilir):** `...any`'yi `fmt`'ye geçen modül işlevleri (vet'in printf sarmalayıcı kuralı) bugün yalnız `messages` paketi için biçimleme sayılıyor; `error` olarak döndürülüp başka yerde biçimlenen değerin `Error()`'ı çağıransız.
- **Not, CODEGRAPH'a iş değil:** madde 15 adım 0'da kodlar okunur kimliğe geçiyor (`A001` → `TR_kw_if`); codegraph'ın kodu tanıyan üç düzenli ifadesi (`gocode_scan.go:18`, `codes.go:11`, `codehist.go:10`) MEKANİK'in yönergesinde, kapsam testiyle (her kimliğin en az bir `uses_name` kenarı). Aynı iş iki kez yapılmasın.
- `dead-methods` ve `package-state`'in ilk koşusunun bulgularının ayıklanması (sayılar 2026-10-26, DENETİM: 56 yalnızca arayüzle ya da yalnızca testte yaşayan metot, 120 paket değişkeni; iş açılmadan önce yeniden koşulur); korumasız görünenler: `tensor.backends`, `tokenize` kayıt haritaları, `codes.defs`, `messages.catalogs`, `interp.engines`, `logging` açık çubuklar ve günlükler, `cmd/tr.exitCode`.
