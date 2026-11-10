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
çevirir, 1xx'i ara cevap yazıp sürer; TR bunları cevap kurulurken hata yapar (28.3). `204`, `205` ve `304`'ün
gövdesi, sunucunun gövdeden yazdığı `Content-Length` ve `Transfer-Encoding` ve `close` dışında değerli
`Connection` da kurulurken hatadır. `Connection: close` harf büyüklüğüne bakılmadan geçer ve `close` yazılır:
Go cevaptan sonra bağlantıyı yalnız bu yazımla kapatır. Go'nun davranışını sabitleyen test Go değişince düşer.
Denetim: `TestGoChangesHeadersSilently`, `TestGoClosesOnlyOnLowercaseClose`, `TestResponseRejectsWhatGoChanges`,
`TestNoBodyStatusesAndServerHeaders`, `TestConnectionCloseClosesTheConnection`, `TestResponseIsWrittenAsGiven`.

**İstek görevi `interp.TaskHost`'tan açılır.** Şablon görünüm `TR_serve_http` çağrıldığında alınır,
her istek ondan çatallanır; işleyicinin hatası istemciye iç bilgisiz `500`, standart hataya kodlu ve
maskeli bir satırdır, sunucu sürer (28.4). Kapanış (`s.TR_close()`, durdurma, program sonu) süren görevleri
iptal eder, en çok 2 saniye bekler (28.8). Açık akışlar kapanır: yayın dinleyicisinin cevabı düzgün biter
(istemci `Last-Event-ID` ile döner), istek akışının bloğu iptal edilir ve akış düzgün bitirilmeden kesilir.
Gövde okuması 60 saniye susarsa bağlantı kapanır; Go'nun kapanışta gövdenin kalanını okuması da bu süreyle
biter (`readBody`, süre geçmiş bırakılır). Gövdesi okunmadan ya da yarım reddedilen istek (403, 401, 400, 413)
`Connection: close` ve geçmiş okuma süresiyle cevaplanır (`refuseUnread`): Go kalan gövdeyi süresiz okumaz,
susan istemci bağlantıyı tutamaz. Denetim: `TestCloseEndsOpenStreams`, `TestTimeoutsCloseTheConnection`.
Denetim: `TestHandlerFailureIs500`, `TestCloseCancelsRunningHandlers`, `TestCloseFinishesAWrittenResponse`,
`TestProgramEndClosesTheServer`, `TestWaitReturnsStopRequested`.

**İstek akışı işleyicinin görevinde yazılır.** `TR_stream_response(blok)` bir cevap değeridir; işleyici onu
döndürünce blok aynı görevde çalışır (`stream.go`). Her `.TR_send_event` yalnız `data:` satırı olan bir SSE
olayını maskeli JSON'la, kilit bırakılarak ve tamponsuz yazar; nabız (15 saniye, yorum satırı ya da
`TR_heartbeat:`'ın verisi) kilit dışında aynı yazıcıyla gider. Görev akış başladıktan sonra istemci koparsa
ya da bir yazma 10 saniyede bitmezse iptal edilir (`interp.TaskHost.RunUntil`). Bloğun hatası standart hataya
kodlu satırdır, bağlantı düzgün bitirilmeden kesilir; HEAD'de blok başlamaz.
Denetim: `TestRequestStreamWritesEvents`, `TestRequestStreamHeartbeat`, `TestHeartbeatDefaultIs15Seconds`,
`TestRequestStreamClientGoneCancelsBlock`, `TestRequestStreamBlockErrorCutsTheStream`, `TestSendAfterBlockIsAnError`,
`TestHeadStartsNoStreamBlock`.

**Yayın akışı dışlamada katlanır, dinleyici kilitsiz yazar.** `TR_EventBroadcast` (`broadcast.go`) her
`.TR_send_event`'te maskeli JSON'u kurar, olayı `TR_reduce:` ile görüntüye katlar, kimliği verir ve olayı halkaya
(son `TR_capacity:` olay) koyar; bunlar ve `.TR_subscribe`'ın görüntüyü ya da geri oynatmayı alması yayıncının
dışlamasında (`sem`) birliktedir, bekleyen kilidi bırakır, katlamanın kendine göndermesi çağrı hatasıdır. Gönderim
ağa yazmaz, dinleyicilere haber verir. Dinleyici işleyicinin görevi bittikten sonra Go'nun goroutine'inde yazar
(`listen`) ve `TR_max_concurrent:`'a sayılmaz; kaçırdığı olay halkadan düşerse ya da bir yazma 10 saniyede bitmezse
yalnız onun bağlantısı kapanır, olay atlanmaz. Görüntü abonelikte bir kez daha maskelenir. HEAD'de dinleyici
kaydedilmez. Olay kimliği `<ön ek>-<sıra>`'dır: ön ek yayıncı kurulurken rastgele üretilir (16 onaltılık
karakter), sıra 1'den sayar. Ön eki tutmayan `Last-Event-ID` (yeniden başlamadan önceki sunucunun ya da başka
yayıncının kimliği) görüntü verir, yeni olayları eski durumun üstüne oynatmaz. Denetim: `broadcast_test.go`.

**Adres ve belirteç.** Varsayılan adres `127.0.0.1`'dir; adres ayrıştırılmış hâliyle denetlenir, başka
adres belirteç ister; belirteç gizli değerdir (19.4). İletiler işletim sisteminin metnini taşımaz (28.9).
Belirteçli sunucuda `TR_public_paths:`'taki tam yollar `GET` ve `HEAD`'de belirteç istemez (`isPublic`,
`guard.go`): tarayıcının adres çubuğundan açtığı sayfa ve sayfanın betikleri `Authorization` taşıyamaz. Yerin
denetimi (28.1a) önce, yolun ve sorgunun denetimi sonra gelir; önek yoktur. Listede geçersiz yol (H7660) ve
belirteçsiz sunucuda liste (H7661) çağrı hatasıdır. Denetim: `TestServeArgumentErrors`,
`TestServeSuggestionsWork`, `TestPublicPathsNeedNoToken`, `TestPortInUse`, `TestSecretToken`,
`TestSecretTokenNeedsOwnerOnly`, `TestServerLogIsCoded`.

**Dosya sunmak her makinede aynı cevabı verir.** `TR_file_response(kök, yol)` yolu bileşen bileşen kökün
içinde arar: aygıt adı, sonu nokta ya da boşluk, `:` her işletim sisteminde reddedilir; her bileşen klasörün
gerçek adıyla harf büyüklüğü dahil aynı olmalıdır (Windows'un kısa adı da böyle düşer); kökten çıkan
bağlantı reddedilir, kökün içindeki izlenir. Tür 28.3a'nın sabit tablosundandır (28.3a).

Bağlantılar Windows'ta kendi çözücüyle çözülür (`resolveLinks`): `filepath.EvalSymlinks` Windows'ta
junction'dan geçen yolda hata verir, `os.Root` junction'ı ve mutlak hedefli bağlantıyı hiç açmaz (DENETİM 374);
`os.Readlink`'in her hatası (OneDrive, dedup gibi öbür yeniden ayrıştırma noktaları) 404'tür. Linux'ta ve
macOS'ta `filepath.EvalSymlinks` kullanılır: çözücünün hedefteki `..`'yi sözcüksel temizlemesi Linux'a uymaz.
Çözülmüş yol kökün içindeyse köke göre göreli hâli `os.Root` ile açılır: çözme ile açma arasında bir
bileşen dışarıyı gösteren bağlantıyla değiştirilirse `os.Root` onu açmaz. Boyut ve tür açılan dosyanın
`Stat`'ından okunur.
Denetim: `TestFileResponseServesInsideTheRoot`, `TestFileComponentRules`, `TestFileCaseMustMatch` (harfe
duyarsız sahte dosya sistemi), `TestFileShortNameIsRefused` (Windows), `TestFileLinksStayInsideTheRoot`,
`TestFileJunctionsStayInsideTheRoot` (Windows), `TestRealPathFollowsThePlatform`, `TestFileSwapAfterResolveIsRefused` (yarış),
`TestFileLinkLoopIs404`, `TestFileResponseCall`, `FuzzFileComponent`.

## Testler

Her `*_test.go` aynı adlı kaynağın testidir; `helpers_test.go` gerçek bağlantıyla sınanan sunucuyu kurar
(Go'da yazılmış, çağrı sayan bir işleyiciyle ya da TR koduyla).

- `behavior_short_name_windows_test.go`: Windows'un kısa adıyla (`INDEX~1.HTM`) istenen dosya 404 alır (28.3a); kısa ad birimde oluşmuyorsa test nedenini yazıp atlanır.
