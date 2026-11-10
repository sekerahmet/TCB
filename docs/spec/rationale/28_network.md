# 28. Ağ: emsal ve gerekçe

Kurallar: [`28_ag.md`](../28_ag.md). Bu dosya her kuralın emsalini ve seçimin gerekçesini tutar.

Kaynaklar dört turdan gelir:
- tur 1 ve 2: ekran oturumunun kaynak devri ve TCB'nin kaynaktan okuması (2026-11-08);
- tur 3: Gemini'nin güvenlik incelemesi, DENETİM 329 ve 330 yeniden üretti;
- tur 4: Gemini'nin emsal araştırması (S1–S8), DENETİM 346 alıntıları örnekledi (2026-11-09).

Go satırları `go1.26.10`'dandır (TR'nin derlendiği sürüm).

### 28.1

- *Emsal:*
  - **llama.cpp sunucusu:** varsayılan `127.0.0.1` (`b12211`, `tools/server/README.md:194`: "(default: 127.0.0.1)").
  - **Ollama:** varsayılan `127.0.0.1:11434` (`v0.40.15`, `docs/faq.mdx:185`: "Ollama binds 127.0.0.1 port 11434 by default.").
  - **MCP:** yerel sunucu yalnız localhost'a bağlanmalı (`2025-06-18`, `transports.mdx:77`: "servers **SHOULD** bind only to localhost (127.0.0.1)").
  - **Go `net/http`:** `Server` açıkça başlatılır ve kapatılır. TR bunu kullanır.
  - **Rust axum:** varsayılan adres yok; program `TcpListener`'ı kendisi bağlar ve belge örnekleri bütün ağa açıyor (`tokio-rs/axum` `4cc51a96`, `axum/src/serve/mod.rs:58`: `TcpListener::bind("0.0.0.0:3000")`). TR bunu almadı: örnekten kopyalayan ajan sunucuyu farkında olmadan dışa açar.
- *Gerekçe:*
  - Ajan için en az hatalı varsayılan, dışarı sızmayan varsayılandır. Yanlışlıkla `0.0.0.0`'a açılan bir model sunucusu sessiz bir güvenlik açığıdır.
  - Beklemeden dönmek, sunucunun bir görev olarak sürmesine ve programın başka işler yapmasına izin verir (10.1).
  - `s.TR_port` testte kapı 0 ile çakışmasız açmayı sağlar.

### 28.1a

- *Emsal:*
  - **Ekranın sözleşmesi** (`tools/assistant/contract.md`, "Güvenlik"): `Host` tam olarak `127.0.0.1:<kapı>` olmalı, başka değer `403`; `localhost` reddedilir.
  - **MCP:** `Origin` doğrulaması MUST, "to prevent DNS rebinding attacks" (`transports.mdx:76`).
  - **llama.cpp:** varsayılan CORS isteğin `Origin`'ini geri yazar ve kimlik bilgisine izin verir (`server-http.cpp:333`–`:335`). Karşı emsal: herhangi bir sayfa cevabı okuyabilir.
  - **Rust axum:** çerçevede varsayılan `Host`/`Origin` denetimi yok.
  - **`Sec-Fetch-Site`:** W3C Fetch Metadata `388c204d`, `index.bs:205` (dört değer), `:221` (elle yazılan adres `none`), `:291` (yalnız güvenilir adrese gider), `:314`–`:315` (yönlendirme zincirinin her adresi), `:341` (sayfanın betiği değiştiremez). MDN BCD: Chrome 76, Firefox 90, Safari 16.4.
  - **Go `CrossOriginProtection`:** `same-origin` ve `none`'ı kabul eder, ama `GET`, `HEAD`, `OPTIONS`'u hiç denetlemez (`csrf.go:136`–`:139`: "Safe methods are always allowed").
  - **Başlığın değerleri** (Chromium `134.0.6998.35`, VS Code `1.98.0`, Electron `v34.3.0`):
    - aynı makinedeki başka kapı `same-site` (`registry_controlled_domain.cc:488`–`:489`);
    - `localhost` ile `127.0.0.1` arası `cross-site` (`:338`–`:340`);
    - yenileme ilk açılışın kaynağını korur (`navigation_entry_impl.cc:899`–`:900`);
    - VS Code'un Simple Browser'ı hedefi `vscode-webview://` kökündeki bir `<iframe>`'de açar (`simpleBrowserView.ts:168`, `preview-src/index.ts:100`): `cross-site`;
    - Electron'un `loadURL`'ü adres çubuğundan yazılmış gibi yükler (`electron_api_web_contents.cc:2461`–`:2462`): `none`.
  - **Danışmalar:** MCP Inspector `GHSA-vcc9-3xgm-p9v4` (`Origin`'siz isteğe güvenmek, `Host` denetimsiz); MCP Python ve TypeScript SDK `GHSA-9h52-p55h-vw2f`, `GHSA-w48q-cv73-mx4w` (DNS yeniden bağlama koruması varsayılan kapalı); Ollama `GHSA-5jx5-hqx5-2vrj`.
- *Gerekçe:*
  - Yalnız bu bilgisayara açık sunucu, tarayıcıdan gelen saldırıya karşı yine açıktır: kullanıcının açtığı herhangi bir sayfa `127.0.0.1`'e istek atabilir.
  - Savunmayı her işleyiciye bırakmak, ajanın bir gün unutması demektir. Asistanın araçları (dosya, süreç) bu yüzden doğrudan açık olurdu. Varsayılan ve kapatılamaz olması "ajan için hatasız" ölçütünden gelir.
  - `GET`'i de denetlemek Go'dan ayrılıştır: yerel sunucuya başka bir sitenin `GET`'i meşru değildir, `<img src>` ile `Origin`'siz bir `GET` veri sızdırır.
  - Bedeli kabul edildi: başka bir sayfadaki bağlantıdan açılan ekran `403` alır, yenilenince de; adres elle yazılır. VS Code'un içindeki görünümler de `403` alır; ekran tarayıcıda ya da kendi penceresinde açılır (ekranın `frame-ancestors 'none'`'ı gömmeyi zaten engelliyor).
  - `Origin`'siz ve `Sec-Fetch-Site`'sız isteğin geçmesi, tarayıcı olmayan istemcileri (API kullanıcıları) bozmamak içindir.
  - Tek yerel biçim `127.0.0.1`: tarayıcıda `localhost` ayrı bir köktür, ekranın sekmeleri iki kökte tek akışı paylaşamaz.

### 28.2

- *Emsal:*
  - **llama.cpp:** belirteç varsayılan kapalı (`README.md:216`: "(default: none)"). Dışa açılan sunucu kimliksiz kalabiliyor; TR bunu almadı.
  - **Jupyter Server:** belirteç varsayılan açık (`v2.21.1`, `security.rst:8`: "token-based authentication that is **on by default**").
  - **Rust tower-http:** `Bearer` doğrulaması ara katman olarak var, yanlış belirteçte `401` (`tower-rs/tower-http` `8569ec8b`, `tower-http/src/auth/require_authorization.rs:124`, `:182`).
  - **RFC 9110:** `401` `WWW-Authenticate` ister (`rfc9110.txt:5026`–`:5027`).
  - **Belirtecin uzunluğu:** OWASP Session Management Cheat Sheet en az 64 bit, onaltılık yazımla en az 16 karakter ister (`Session_Management_Cheat_Sheet.md:37`, `:51`; okuma 2026-11-09). NIST SP 800-63B-4 rastgele oturum sırrında en az 64 bit (Bölüm 5.1). VS Code belirtecin karakter kümesini `/^[0-9A-Za-z_-]+$/` ile denetler, uzunluğa bakmaz (`1.98.0`, `serverConnectionToken.ts:16`); Jupyter, code-server, llama.cpp ve tower-http boş olmayan her dizgeyi kabul eder.
  - **Go `subtle.ConstantTimeCompare`:** uzunluk farklıysa hemen döner (`constant_time.go:18`–`:19`).
- *Gerekçe:*
  - Dışa açmak bilinçli bir karar olmalı. Belirteçsiz dış adres hatası, unutkanlığı sessiz açık yerine açık hataya çevirir.
  - Alt sınır 16 karakter: OWASP'ın 64 bitlik eşiği ve 19.4'ün maskeleme eşiğinin (6 karakter) üstü; kısa belirteç hem kırılır hem maskelenmeden sızar. Küme ASCII olduğu için karakter ile bayt aynıdır; çok baytlı bir harf eşiği aşıp maskelemenin altında kalamaz.
  - Belirteç her zaman gizlidir: 19.4'ün "yalnız sayı olan değer gizli değil" istisnası belirtece uysaydı, rakamlardan oluşan bir belirteç terminale açık yazılırdı.
  - Sabit sürede karşılaştırma zamanlama ile tahmini engeller. Uzunluk farkı sızar; `TR_secret_token()`'ın uzunluğu sabittir.
  - Adresin ayrıştırılmış hâliyle denetlenmesi `::1`, `[::1]` ve `127.0.0.2` gibi yazımların belirteçsiz dışarı ya da yanlış yerel köke açılmasını önler.
  - TLS'nin bedeli (sertifika, yerel kök) ilk sürümün kazancından büyük [görüş]. `127.0.0.1` tarayıcıda zaten güvenli bağlamdır (W3C Secure Contexts).

### 28.2a

- *Emsal:*
  - **Jupyter:** belirteci `os.urandom(24)` ile üretir (`jupyter_server/auth/identity.py:224`).
  - **Rust:** `rand`, işletim sisteminin kaynağını `SysRng` adıyla veriyor (`rust-random/rand` `d1cbc134`, `src/rngs/mod.rs:18`).
  - **Python:** `secrets.token_urlsafe` (CPython `v3.13.1`, `Lib/secrets.py:60`); varsayılan 32 bayt (`:31`: "DEFAULT_ENTROPY = 32").
  - **Teslim:**
    - code-server parolayı terminale hiç yazmaz, günlükte `<redacted>` yazar, kullanıcının yapılandırma dosyasına yazar ve terminale dosyanın yolunu yazar (`v4.98.0`, `src/node/cli.ts:462`, `:672`–`:673`; `src/node/main.ts:148`);
    - Jupyter bilgi dosyasını `secure_write` ile yazar: Unix'te `0o0600`, Windows'ta yalnız kullanıcıya ve Administrators'a açık erişim listesi (`jupyter_core` `v5.8.1`, `paths.py:1006`, `:1067`);
    - VS Code belirteç dosyasını `0o600` ile yazar, Windows'ta erişim listesi kurmaz (`serverConnectionToken.ts:115`);
    - Jupyter ve VS Code adresi `?token=` ile terminale yazar; adresle teslim `Referer` sızıntısı taşır (Jupyter `GHSA-c3mw-737p-c7g2`, günlükte belirteç).
- *Gerekçe:*
  - Dilin rastgele üreteci tohumludur ("aynı veri aynı sonuç"); güvenlik için kullanılırsa belirteç tahmin edilebilir. Ayrı bir ad, ajanın yanlış üreteci seçmesini zorlaştırır [görüş].
  - Terminal yolu 19.4 yüzünden çalışmaz (belirteç `***` görünür). code-server'ın yolu TR'nin kurallarına uyan tek yoldur: değer dosyada, terminalde dosyanın yolu.
  - Dosyanın yalnız kullanıcıya açık olması 18.13'ün `TR_owner_only:`'siyle yazılır; içeriğe göre izin seçmek kırılgan ve Windows'ta anlamsız olurdu (tur 3'te geri çevrildi).

### 28.3

- *Emsal:*
  - **Rust axum:** işleyici bir değer döndürür (`axum/src/docs/handlers_intro.md:1`–`:3`).
  - **Go `net/http`:** işleyici `ResponseWriter`'a yazar; ikinci `WriteHeader` yalnız günlüğe uyarı yazar (`src/net/http/server.go:1147`: "http: superfluous response.WriteHeader call from %s"). Yazıcı modelinin sessiz sıra hatası; TR almadı.
  - **Başlık adının harf büyüklüğü:** Go'nun `Header.Get`'i harf büyüklüğüne bakmaz (`header.go:45`: "It is case insensitive"). Rust `http` başlık adını küçük harfe çevirir (`hyperium/http` `4a0885cf`, `src/header/name.rs:26`).
  - **Başlıkların birleşmesi:** RFC 9110 5.3 (`rfc9110.txt:1517`).
  - **Yol ve başlık:** Go `ReadRequest` `Host`'u başlık haritasından siler (`request.go:1064`); `Header.writeSubset` geçersiz başlık adını atar (`header.go:202`), değerdeki satır sonunu boşluğa çevirir (`:139`, `:206`). Yol aşımı danışmaları: Ollama `GHSA-8hqg-whrw-pv92`, Jupyter `GHSA-5789-5fc7-67v3`.
  - **`HEAD`:** RFC 9110 9.3.2: "identical to GET except that the server MUST NOT send content" (`rfc9110.txt:3987`–`:3988`). Beş çerçevenin beşi `GET` işleyicisini `HEAD`'e de çağırır, `Content-Length`'i `GET` gövdesinden yazar, gövdeyi göndermez: Go `ServeMux` (`server.go:2504`–`:2505`), Werkzeug (`3.1.3`, `routing/rules.py:497`–`:498`), Starlette (`0.46.2`, `routing.py:245`–`:246`), axum (`method_routing.rs:1238`–`:1239`), Express (`4.21.2`, `lib/router/route.js:113`–`:115`).
  - **JSON gövdesi:** RFC 8259 11 `application/json`'a parametre tanımlamaz (`rfc8259.txt:623`–`:624`). Flask `jsonify` (`3.1.0`, `json/provider.py:105`), FastAPI'nin varsayılan `JSONResponse`'u (`0.115.11`, `routing.py:221`) ve axum'un `Json`'u (`json.rs:210`–`:211`) türü kendiliğinden yazar; Go yazmaz, JSON metni `DetectContentType` ile `text/plain` gider (`server.go:1454`–`:1455`).
- *Gerekçe:*
  - Değer döndüren işleyici, yazıcı modelinin sıra hatalarını yapılamaz kılar (ajan için hatasız).
  - Gövdenin `TR_Bytes` olması dış programların çıktısıyla (21.1) ve dilin geri kalanıyla tutarlı.
  - Başlıkların harf büyüklüğüne bakmadan okunması ajanın alışkanlığıyla (`["Content-Type"]`, `["Authorization"]`) çalışır. Küçük harfli sözlük o yazımda başlığı bulamaz ve yetki denetimi sessizce yanlış dala gider.
  - Sorgunun liste değerli olması tekrarlanan anahtarı sessizce yutmamak içindir.
  - TR sessiz değiştirmez: Go'nun sildiği ya da boşluğa çevirdiği başlık TR'de çalışma hatasıdır. Aynı ilke `204`, `205` ve `304`'ün gövdesine ve sunucunun yazdığı `Content-Length` ve `Transfer-Encoding` başlıklarına uygulanır: aşama 1'de gövde ve verilen `Content-Length` hatasız düşüyor ya da eziliyordu [kanıtlı: DENETİM 371, `response.go`'nun yazma işlevi]. Go `204` ve `304`'te gövdeyi düşürür, `205`'te gönderir [kanıtlı: DENETİM 373, Go 1.26 `net/http/transfer.go`, `bodyAllowedForStatus`]; RFC 9110 üçüne de içerik yazdırmaz (204 için 15.3.5, 304 için 15.4.5, Go'nun `net/http/status.go` yorumlarıyla aynı; 205 için 15.3.6 [kanıtsız: açılmadı]). `Connection: close` Go'da ezilmez, uygulanır (`server.go`, `closeAfterReply`); öbür değerleri (`keep-alive`) sunucu ezer [kanıtlı: DENETİM 373]. Bu yüzden yalnız `close` geçerlidir: işleyici bir cevaptan sonra bağlantıyı bitirmek isteyebilir. Go `close`'u yalnız tam küçük harfle uygular (`server.go:1347`), başlığı korurken harfe bakmaz; `Connection: Close` istemciye gider ama bağlantı açık kalırdı [kanıtlı: DENETİM 374]. TR değeri harfe bakmadan tanır ve kapatmayı kendisi sağlar. Çözülmüş yolda `\` ve `..` bileşeninin `400` olması Windows'ta `\`'nin de ayırıcı olmasındandır.
  - `HEAD`'de işleyicinin `"GET"` görmesi emsalden bir adım öte: çerçevelerde açık bir `HEAD` kolu `GET`'ten önce gelir; TR'de yönlendirme demet desenidir (28.6), ajan `("GET", "/saglik")` yazar ve `HEAD`'i düşünmez. `HEAD`'i işleyiciye olduğu gibi vermek her `GET` ucunu sağlık yoklamasında (`curl -I`) `404`'e çevirirdi. `TR_Request(TR_method: "HEAD")`'in hata olması testin gerçek sunucunun göstermediği bir yöntemi kurmasını önler; kurucu her geçersiz yöntem adını reddeder (RFC 9110'un `token`'ı).
  - JSON'un tek yazımı `TR_json:`: ajan türü elle yazmayı unutunca geçerli JSON `text/plain` gider ve sıkı istemci reddeder. `TR_body: TR_json_string(x)` metin olarak kalır; iki yol aynı sonucu verseydi ajan hangisini yazacağını bilemezdi.
  - İşleyicinin sıradan fonksiyon olması onu ağsız sınanabilir yapar ("aynı veri aynı sonuç").

### 28.3a

- *Emsal:*
  - **Go `os.Root`:** `os.OpenInRoot(dir, name)` (`src/os/root.go:25`); kökün içinde bağlantı izlenir, dışına çıkan ya da mutlak bağlantı reddedilir (`:41`–`:43`); Windows'ta bileşenler `O_NOFOLLOW_ANY` ile tek tek açılır (`root_windows.go:147`).
  - **Go'nun ayrılmış adları:** `CON`, `PRN`, `AUX`, `NUL`, `COM1`–`COM9`, `LPT1`–`LPT9`, üst simge rakamlılar, `CONIN$`, `CONOUT$`; ad ilk `.` ya da `:`'ya kadar, sondaki boşluklar atılarak karşılaştırılır; uzantılı biçim Windows 11'den beri aygıt değildir ve Go bunu `RtlIsDosDeviceName_U`'ya sorar (`src/internal/filepathlite/path_windows.go:98`–`:160`).
  - **Rust tower-http `ServeDir`:** baştaki `/`'ı soyar, yalnız sıradan bileşenleri kabul eder, ayrılmış DOS adlarını reddeder (`8569ec8b`, `serve_dir/mod.rs:619`, `:627`–`:636`); bağlantıları izler (`backend.rs:81`).
  - **Go `http.Dir`:** baştaki `/`'ı soyar ama dışarıyı gösteren bağlantıyı izler (`src/net/http/fs.go:36`–`:37`: "Dir will follow symlinks pointing out of the directory tree").
  - **Werkzeug `safe_join`:** `/` ile başlayan parçada `None` döner (`3.1.3`, `security.py:158`–`:162`); istek yoluyla doğrudan çağrılamaz.
  - **Starlette `StaticFiles`:** kökten çıkan bağlantıyı varsayılan olarak reddeder (`0.46.2`, `staticfiles.py:47`, `:158`–`:160`); Windows adlarını süzmez.
  - **Go'nun `mime` tablosu** Windows'ta kayıt defterinden genişler (`src/mime/type_windows.go:16`, `:24`).
- *Gerekçe:*
  - Ajan yolu hiç kurmaz: `TR_Path("ekran", istek.TR_path)` 18.1'in mutlak parça kuralıyla düşer (H7139); kaçış olarak yazılan `"ekran" + istek.TR_path` ya da `istek.TR_path[1..]` ajanın unutacağı adımlardır. İstek yolunu olduğu gibi alan tek bir yerleşik bu tuzağı yapısal olarak kapatır.
  - `os.Root` bağlantı kaçışını çekirdek düzeyinde kapatır; 18.16'nın "bağlantı izlenir" kuralı burada kökün içiyle sınırlanır.
  - Aygıt adı, sondaki nokta ve boşluk, `:` ve harf büyüklüğü kuralları her işletim sisteminde aynı uygulanır: aynı istek Linux'ta `404`, Windows'ta bir aygıt ya da başka bir dosya olmamalı ("aynı program aynı veriyle aynı sonucu verir"). Uzantılı aygıt adı Windows 11'de serbest; TR yine reddeder, sonuç Windows sürümüne de bağlı kalmasın. `COM0` ve `LPT0` Go'nun listesinde yok; TR onları da reddeder (Microsoft'un adlandırma sayfası sayıyor olabilir [hipotez]); fazla reddetmek sonucu makineden bağımsız tutar. Karşılaştırma ASCII'dir: Türkçe yerel ayarda büyük harfe çevirmek `i`'yi `İ` yapar ve `conin$` eşleşmeden kaçardı (DENETİM 347).
  - Tam harf eşleşmesinin bedeli: her bileşende üst klasörü okuyup bayt bayt karşılaştırmak; 20 dosyalık klasörde ortanca 162 µs (düz açma 55 µs), 2000 dosyada 825 µs [ölçüldü: DENETİM 347, Windows, D1 çalışırken].
  - Tam harf eşleşmesi Windows ve macOS'un büyük-küçük harfe bakmayan dosya sisteminde `/Index.html`'in ayrı sonuç vermesini önler; kısa adları (`INDEX~1.HTM`) da kapatır.
  - Türün sabit tablodan bulunması aynı dosyanın iki makinede ayrı türle gitmesini önler; `nosniff` tarayıcının türü tahmin etmesini kapatır.

### 28.4

- *Emsal:*
  - **Go `net/http`:** her bağlantı bir goroutine'de (`src/net/http/server.go:3484`: `go c.serve(connCtx)`).
  - **Rust axum:** bağlantılar `tokio::spawn` ile ayrı görevde (`axum/src/serve/mod.rs:357`).
  - **Eşzamanlı istek sınırı:** sekiz emsalin çoğunda varsayılan yok. Verilince aşan istek uvicorn'da beklemeden `503`, `Retry-After`'sız (`0.34.2`, `config.py:210`; `protocols/http/h11_impl.py:220`–`:224`), nginx'in `limit_conn`'unda da `503` (`release-1.27.4`, `ngx_http_limit_conn_module.c:552`–`:553`). Model sunucuları kuyrukta bekletir: llama.cpp yuva doluysa görevi erteler (`server-context.cpp:3051`–`:3053`), Ollama 512 isteğe kadar bekletir, sonra `503` (`d63b7026`, `envconfig/config.go:281`; `server/routes.go:3640`–`:3641`).
  - **TR'nin kendisi:** 10.1 ve 10.3.
- *Gerekçe:*
  - Bir isteğin uzun model hesabı öbür istekleri (ekranın durdur düğmesi gibi) bekletmemeli.
  - `500` gövdesinin iç bilgi taşımaması hata metninin (yol, yığın) dışarı sızmaması içindir.
  - Çıkış kodunun değişmemesi: sunucu hatayı yakalayıp `500` yazar, 10.6 "yakalanan hata çıkış kodunu bozmaz" der. Eğitimin durum ucundaki tek bir bozuk istek, başarıyla biten eğitimi "düştü" göstermemeli.
  - Denetimlerin görev açılmadan yapılması: 10.2'nin sözüyle önünde 1000 görev bekleyen görev kilidi ~5 sn'de alır [hesap]; kimliksiz bir sel görev açamazsa robotun döngüsünü de bekletemez.
  - Bekletmek yerine `503`: bekleyen istek bağlantı ve bellek biriktirir, `503` istemcinin geri çekilmesini sağlar. `Retry-After`'sız, çünkü sunucu ne zaman boşalacağını bilmez.
  - Sınıra TR kodu koşan işler sayılır: işleyici, istek akışının bloğu ve dinleyici açılırken çağrılan `TR_snapshot:`. Yayın dinleyicisine olayı ve nabzı sunucu kilit dışında yazar, TR kodu koşmaz; sayılsaydı açık sekmeler sunucuyu kilitlerdi.
  - Dinleyici sayısı bilinçli olarak sınırsızdır: bedeli bir bağlantı ve tamponudur; yerelde 28.1a, dış adreste belirteç korur; yavaş okuyanın bağlantısı kapanır (28.7). Ekranın ölçüsünde 7 sekme ve 4'lük bir dinleyici sınırı 60 saniyede 133 açılış ve atılma döngüsü kurmuştu [ölçüldü: ekran kolu, 2026-11-08]. Yerelde başka bir süreç de sınırsız bağlantı açabilir; bu, yerel bir sürecin zaten yapabildiklerinin içindedir.
  - Sınır robotun sabit hızlı döngüsünü tek başına korumaz: kilidi bırakmayan tek bir uzun işleyici de döngüyü bekletir (10.2). Bu yüzden maddede yazılı.

### 28.5

- *Emsal:*
  - **Go:** `http.MaxBytesReader` (`src/net/http/request.go:1176`); başlık sınırı `DefaultMaxHeaderBytes` 1 MiB (`server.go:895`); süre sınırlarının varsayılanı yok (`server.go:3007`–`:3008`: "A zero or negative value means there will be no timeout").
  - **Gövde sınırı:** nginx 1 MiB (`ngx_http_core_module.c:3794`–`:3795`), axum 2 MB (`axum-core/src/extract/default_body_limit.rs:7`); model ve defter sunucuları yüksek tutar (llama.cpp 100 MiB, Jupyter 512 MiB).
  - **Süreler:** nginx başlık ve gövde okumasında 60 sn (`:3496`–`:3497`, `:3799`–`:3800`); boşta bekleme uvicorn'da 5 sn (`config.py:213`), llama.cpp'nin HTTP kitaplığında 5 sn (`httplib.h:34`), nginx'te 75 sn. İşleyici süresi hiçbirinde yok; yazma süresi nginx'te var (60 sn).
  - **Yerel ve dış:** sekiz emsalin hiçbirinde değerler adrese göre değişmez.
  - **Danışma:** MCP Python SDK `GHSA-fmmv-w9g8-j3gc` (gövdeyi boyutunu denetlemeden belleğe okumak).
- *Gerekçe:*
  - Sınırsız gövde bellek tüketme saldırısıdır; Go'nun sıfır süreleri yavaş istemci saldırısına açıktır.
  - 1 MiB ekranın 64 KiB'lık komutlarını rahatça karşılar; çok kipli girdi `TR_max_body_bytes:` ile büyütülür.
  - Yazma ve işleyici süresi yok: akış (28.7) ve uzun üretim kesilmemeli. Süreyi program kendi koyar.
  - Gövde okumasında iki okuma arası süre, toplam süre değil: büyük ama akan bir yükleme kesilmez, duran kesilir.

### 28.6

- *Emsal:*
  - **Rust axum:** yönlendirici kitaplığın nesnesidir, yollar `route` ile eklenir (`axum/src/docs/routing/route.md:1`).
  - **Go:** `ServeMux` deseni yöntemi ve yolu birlikte eşler (`src/net/http/server.go:2496`: "[METHOD ][HOST]/[PATH]").
- *Gerekçe:*
  - Dilin `TR_match`'i ve demet deseni (5.6) yönlendirmeyi yeni bir kavram eklemeden yazdırır. Ajan bildiği deyimi kullanır.
  - Yönlendirici gerekirse sonra kitaplık olarak gelir.

### 28.7

- *Emsal:*
  - **WHATWG HTML 9.2:** `Last-Event-ID` ve yeniden bağlanma (9.2.4); `id` alanı olmayan olay son kimliği değiştirmez (9.2.6).
  - **MCP:** SSE `id` ve sürdürme (`transports.mdx:154`–`:155`).
  - **Olay kimliği verinin dışında:** MCP, Go `r3labs/sse` (`v2.10.0`, `event_log.go:21`; `http.go:88`, `:99`), `sse-starlette` (`v2.2.1`, `event.py:39`) ve axum (`sse.rs:328`, `:450`) kimliği yalnız `id` satırına yazar; `fetch` tabanlı ayrıştırıcı her olayın `id`'sini ayrı okur (`Azure/fetch-event-source` `v2.0.1`, `parse.ts:151`–`:152`, `:180`). Veriye kimlik koyan emsal bulunmadı.
  - **Go:** `Shutdown` uzun bağlantıyı bekler (`src/net/http/server.go:3150`–`:3153`); devralınmış bağlantıyı beklemez (`:3162`).
  - **Rust axum:** SSE nabzı (`axum/src/response/sse.rs:23`: `KeepAlive::default()`); `KeepAlive::new()`'in `max_interval`'ı 15 saniye, varsayılan nabız yorum satırı.
  - **sse-starlette:** `DEFAULT_PING_INTERVAL = 15`, nabız yorum satırı.
  - **openai-python:** `_streaming.py` adsız her olayın verisini bir parça sayar [ölçüldü: DENETİM 356].
  - **Süzme:** GitHub Actions Runner gizli değerleri kaçışlı biçimleriyle maskeler (`HostContext.cs:108`–`:109`); Jupyter yalnız günlükte (`log.py:20`); tower-http yalnız başlık işaretler.
- *Gerekçe:*
  - İki kip iki ayrı kullanıcıdan gelir: "Sunmak"ın istek başına akışında istemci koparsa üretim durur; ekranın yayın akışında bir sekme koparsa üretim sürer, sekme kaldığı yerden yeniden bağlanır. Tek kip ikisinden birini bozar.
  - Kimliğin yalnız `id` satırında gitmesi: veriye kimlik koymak 19.4'ün "veri değişmez"iyle çatışır; programın kendi sayacı sessiz bir eşitlik varsayımı olurdu.
  - Olay akışı 19.4'ün insanın göreceği çıktılarındandır: maskeyi `.TR_send_event` yapar, çünkü gizli değerleri yalnız çalışma anı bilir (19.1 ortamı listeletmez). Maske JSON'a çevrilmeden önce uygulanır; sonra `\"` ve `\uXXXX` kaçışı eşleşmeyi bozardı. Düz `TR_Response` gövdesi veridir, maskelenmez.
  - İstek akışında bloğun hatası akışı düzgün bitirmeden keser: başlıklar gittiği için `500` yazılamaz; akış yalnız kapansaydı dil modelinin cevabını bekleyen istemci yarım cevabı tam sanırdı (sessiz yanlış, DENETİM 374 Ş3). Kesilen `chunked` akış son parçasız biter, istemci hatayı görür [hipotez: Go'nun bu yolu koşulmadı].
  - Anlık görüntü sürerken gönderenin beklemesi "kaybolmaz, iki kez gelmez" sözünün tek yoludur: görüntü kilidi bırakırsa (`TR_sleep`, dosya okuma) araya giren olay kimlik başta alınırsa iki kez gelir, sonda alınırsa kaybolur. Gönderen kilidi tutarak bekleseydi görüntünün görevi kilide dönemez, kilitlenirdi (DENETİM 374 Ş2).
  - Parça parça giden metnin maskesi 19.4a'dadır (gerekçesi orada): olay başına maske iki olaya bölünen değeri görmez, hangi metinlerin birleşeceğini yalnız program bilir.
  - `TR_snapshot:` dinleyici açılırken işleyicinin içinde çağrılır: anlık görüntü ile kimlik arasında olay kaybolmaz ve süresi 28.4'ün sınırına girer.
  - Nabız kimliksizdir: ekranın ayrıştırıcısı son kimliği tutar, yeniden bağlanmada `Last-Event-ID` doğru kalır.
  - Nabzın biçimi kipe göre ayrıdır. "Sunmak"ın yaygın biçimi istek akışıdır ve OpenAI uyumlu istemci adsız her olayı bir parça sayar; orada veri olayı olan nabız her 15 saniyede sahte bir parça olurdu. Yorum satırını her SSE istemcisi atlar (axum ve sse-starlette'in varsayılanı). Ekranın tarayıcı kodu yorumu göremez; ekran yayın akışını kullanır, orada nabız veri olayıdır ve sözleşmenin biçimi `TR_heartbeat:` ile verilir. Aralık emsallerle aynı, 15 saniye.

### 28.8

- *Emsal:*
  - **Go:** `Server.Shutdown`, bağlam süresiyle (`src/net/http/server.go:3150`–`:3155`, `:3170`).
  - **Rust axum:** `with_graceful_shutdown` (`axum/src/serve/mod.rs:403`).
  - **TR:** 21.5'in 2 saniyelik nazik durdurması, 1.5 durdurma, 10.9 iptal; `TR_Bool` dönüşü 18.15 gibi.
- *Gerekçe:* kapanış dilin geri kalanıyla aynı yoldan geçer; yarım yazılmış olay istemcide bozuk çerçeve bırakmaz.

### 28.9

- *Emsal:*
  - TR'nin 8.5'i (iletiler kodlu, işletim sisteminin metni taşınmaz).
  - **Rust:** kapı dolu hatası işletim sisteminden bağımsız bir türdür (`rust-lang/rust` `1.91.0`, `library/std/src/io/error.rs:256`: `AddrInUse,`).
- *Gerekçe:* kapı dolu hatasının metni işletim sistemine göre değişir. Kodlu ileti onu her dilde aynı yazar ve çaresini önerir: başka kapı ya da `TR_port: 0`.
