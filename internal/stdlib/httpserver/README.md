# httpserver

TR'nin HTTP sunucusu (dil tanımı 28): `TR_fn_serve_http(işleyici)`, istek ve cevap değerleri
(`TR_type_Request`, `TR_type_Response`, `TR_type_Headers`), dosya sunmak (`TR_fn_file_response`) ve
belirteç (`TR_fn_secret_token`). HTTP'yi Go'nun `net/http`'si ayrıştırır; TR programı değerlerle çalışır.

## Standartlar

**Denetimler görevden önce.** Geldiği yerin (`Host`, `Origin`, `Sec-Fetch-Site`; 28.1a), belirtecin
(28.2), yolun (28.3), eşzamanlı istek sınırının (28.4) ve gövde sınırının (28.5) denetimleri Go'nun
goroutine'inde, yorumlayıcı kilidi alınmadan ve görev açılmadan yapılır (`guard.go`). Reddedilen istekte
işleyici çağrılmaz; her denetimin testi bunu sayaçla ölçer.
Denetim: `TestOriginIsChecked`, `TestTokenIsChecked`, `TestTokenComparisonIsConstantTime`,
`TestPathIsChecked`, `FuzzValidPath`, `TestBodyLimit`, `TestMaxConcurrent`.

**TR sessiz değiştirmez.** Go'nun sunucusu geçersiz adlı başlığı siler, değerdeki satır sonunu boşluğa
çevirir, 1xx'i ara cevap yazıp sürer; TR bunları cevap kurulurken hata yapar (28.3). Go'nun davranışını
sabitleyen test Go değişince düşer.
Denetim: `TestGoChangesHeadersSilently`, `TestResponseRejectsWhatGoChanges`, `TestResponseIsWrittenAsGiven`.

**İstek görevi `interp.TaskHost`'tan açılır.** Şablon görünüm `TR_serve_http` çağrıldığında alınır,
her istek ondan çatallanır; işleyicinin hatası istemciye iç bilgisiz `500`, standart hataya kodlu ve
maskeli bir satırdır, sunucu sürer (28.4). Kapanış (`s.TR_close()`, durdurma, program sonu) süren görevleri
iptal eder, en çok 2 saniye bekler (28.8).
Denetim: `TestHandlerFailureIs500`, `TestCloseCancelsRunningHandlers`, `TestCloseFinishesAWrittenResponse`,
`TestProgramEndClosesTheServer`, `TestWaitReturnsStopRequested`.

**Adres ve belirteç.** Varsayılan adres `127.0.0.1`'dir; adres ayrıştırılmış hâliyle denetlenir, başka
adres belirteç ister; belirteç gizli değerdir (19.4). İletiler işletim sisteminin metnini taşımaz (28.9).
Denetim: `TestServeArgumentErrors`, `TestServeSuggestionsWork`, `TestPortInUse`, `TestSecretToken`,
`TestSecretTokenNeedsOwnerOnly`, `TestServerLogIsCoded`.

**Dosya sunmak her makinede aynı cevabı verir.** `TR_file_response(kök, yol)` yolu bileşen bileşen kökün
içinde arar: aygıt adı, sonu nokta ya da boşluk, `:` her işletim sisteminde reddedilir; her bileşen klasörün
gerçek adıyla harf büyüklüğü dahil aynı olmalıdır (Windows'un kısa adı da böyle düşer); kökten çıkan
bağlantı reddedilir, kökün içindeki izlenir. Tür 28.3a'nın sabit tablosundandır (28.3a).

Bağlantılar kendi çözücüyle çözülür (`resolveLinks`): `filepath.EvalSymlinks` Windows'ta junction'dan
geçen yolda hata verir, `os.Root` junction'ı ve mutlak hedefli bağlantıyı hiç açmaz (DENETİM 374).
Çözülmüş yol kökün içindeyse köke göre göreli hâli `os.Root` ile açılır: çözme ile açma arasında bir
bileşen dışarıyı gösteren bağlantıyla değiştirilirse `os.Root` onu açmaz. Boyut ve tür açılan dosyanın
`Stat`'ından okunur.
Denetim: `TestFileResponseServesInsideTheRoot`, `TestFileComponentRules`, `TestFileCaseMustMatch` (harfe
duyarsız sahte dosya sistemi), `TestFileShortNameIsRefused` (Windows), `TestFileLinksStayInsideTheRoot`,
`TestFileJunctionsStayInsideTheRoot` (Windows), `TestFileSwapAfterResolveIsRefused` (yarış),
`TestFileLinkLoopIs404`, `TestFileResponseCall`, `FuzzFileComponent`.

## Testler

Her `*_test.go` aynı adlı kaynağın testidir; `helpers_test.go` gerçek bağlantıyla sınanan sunucuyu kurar
(Go'da yazılmış, çağrı sayan bir işleyiciyle ya da TR koduyla).

- `behavior_short_name_windows_test.go`: Windows'un kısa adıyla (`INDEX~1.HTM`) istenen dosya 404 alır (28.3a); kısa ad birimde oluşmuyorsa test nedenini yazıp atlanır.
