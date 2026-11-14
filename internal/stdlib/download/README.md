# download

TR'nin indirme yerleşiği (`TR_fn_download`, 28.10) ve istemcinin güvenlik kuralları (28.11).

## Standartlar

**Son ad yarım dosya taşımaz.** Baytlar `<hedef>.part`'a yazılır; hedef ancak boy ve karma (verilmişse)
tutunca tek adımda yerine konur. Karma tutmazsa ya da boy aşılırsa `.part` sıfırlanır; boysuz ve
karmasız çağrıda kapanışla biten gövde yerine konmaz (H7739).
Denetim: `TestDownloadHashMismatchResetsPart`, `TestDownloadSizeExceededStopsEarly`, `TestDownloadCloseDelimitedBodyNeedsSizeOrHash`.

**`.part`'ın adı yalnız kilit tutulurken değişir.** `.part` açılırken kilitlenir (Unix'te `flock`,
Windows'ta dosya sonunun ötesindeki bir baytta `LockFileEx`); kilit alınınca yolun kilitlenen dosyayı
gösterdiği denetlenir; yerine koyma kilit bırakılmadan yapılır (Windows'ta tutamakla, `FileRenameInfoEx`).
Windows'ta dosya `FILE_SHARE_DELETE` ile `CreateFile`'la açılır: Go'nun açışıyla kilitli dosyanın adı
değiştirilemez (K7'nin ölçüsü). `.part` aynı tutamaktan okunur.
Denetim: `TestDownloadLockIsExclusive`, `TestDownloadLockFollowsTheFile`, `TestDownloadRenamesUnderTheLock`.

**Sürdürme yalnız karmalı çağrıdadır.** Karmalı çağrı `Range` ile sürer; `206` ve doğru
`Content-Range` dışında `.part` sıfırlanır ve aynı çağrı baştan iner. Karmasız çağrı hep baştan iner.
Denetim: `TestDownloadResumesOnlyWithHash`, `TestDownloadRestartsOnWrongRange`.

**İptal bir okuma sınırındadır.** Her okuma (en çok 1 MiB) ve istek yorumlayıcının kilidi bırakılarak
`AwaitExternal` içinde yapılır; iptal, süre sınırı ve durdurma isteğin bağlamını keser, `.part` kalır.
Bağlantı ve ilk üstbilgi 30, hareketsizlik 60 saniyedir.
Denetim: `TestDownloadCancelKeepsPart`, `TestDownloadTimeouts`, `TestDownloadProgressCalls`.

**İstemci kapatılamaz kurallarla bağlanır.** Yalnız `https` (`127.0.0.1` dışında); yerel adrese çözülen
ad `DialContext`'te, yerel IP'ye ve dışarıdan `127.0.0.1`'e yönlendirme `CheckRedirect`'te reddedilir;
vekil yalnız `TR_proxy:` ile; `Accept-Encoding: identity`; sertifika her adımda doğrulanır. İletiler
adresi kimlik kısmı ve sorgu olmadan yazar.
Denetim: `TestClientOnlyHTTPS`, `TestClientLoopbackHTTPRule`, `TestClientRefusesLocalAddresses`, `TestClientRedirectLimit`, `TestClientRedirectDropsCredentials`, `TestClientProxyIsExplicit`, `TestClientLoopbackSkipsProxy`, `TestClientVerifiesCertificates`, `TestClientIdentityEncodingOnly`, `TestClientRefusesUserInfo`, `TestClientMessagesHaveNoQuery`.

**Öneriler kopyalanınca çalışır.** Önerili her ileti, elle cevap yazan bir sunucuyla, önerisi
kopyalanmış hâliyle sınanır; tablo bütün önerili kodları taşır.
Denetim: `TestDownloadSuggestionsWork`, `TestDownloadNeedsHashAndSize`, `TestDownloadHashSpelling`, `TestDownloadErrorsAreCoded`, `TestDownloadIgnoresServerName`.

## Testler

Birim testleri sınadıkları dosyanın adını taşır (`x_test.go` → `x.go`); `helpers_test.go` ortak
yardımcılardır (TR programı koşturmak, elle cevap yazan sunucu).
