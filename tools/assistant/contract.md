# Ekran sözleşmesi (sürüm 2)

Yerel asistanın ekranı TR'den ne bekliyorsa burada yazılıdır. Ekran yalnız bu sözleşmeyle konuşur; sahte akış da aynı baytları üretir. Sunucu TR programıdır, TR'nin HTTP sunucusuyla ve olay yayıncısıyla yazılır (dil tanımı, 28).

Tel İngilizcedir, hiçbir insan diline ayrıcalık yok. Ekrandaki yazılar telden gelmez: sunucu kod ve değer gönderir, ekran kendi sözlüğünden kullanıcının diliyle gösterir. Model metni, araç adı ve araç çıktısı veridir, çevrilmez.

Bütün örnekler uydurmadır.

## Genel

- **Uçlar:** `GET /events` (server-sent events), `POST /command`, `GET /` (ekranın kendisi), `GET /lang/<kod>.js` (ekranın dil dosyaları), `GET /pair` ve `GET /pair/renew` (yalnız Dost `--tarayıcıyla_eşle` ile açıldıysa): belirteçsiz, yalnız `GET` ve `HEAD`; ikisi de bir üstbilgi ister (`Dost-Pair-Code`, `Dost-Pair-Renew`).
- **Biçim:** JSON, UTF-8.
- **Sürüm:** her olayda ve her komutta `v` alanı. Bugün `2`. Ekran başka bir sürüm görürse olayı işlemez, `protocol_mismatch` gösterir ve komut göndermez.
- **Bilinmeyen alan** yok sayılır. **Bilinmeyen olay türü** yok sayılır: yeni olay eski ekranı kırmaz.
- **Süreler** milisaniye, **boyutlar** bayt, **zaman** UTC RFC 3339.
- **Hata** metin taşımaz: `{"code": "...", "values": {...}}`. Değer adı `_ms` ile bitiyorsa süre, `_bytes` ile bitiyorsa boyuttur; ekran onu kullanıcının diline göre biçimler.

## Kaynak

Ekranın kaynağı sayfanın kendi köküdür: TR'nin sunduğu `http://127.0.0.1:<kapı>/` sayfası olayları ve komutları aynı köke gönderir. Sayfanın adresi sorgusuzdur: sorgulu istek belirteç ister (dil tanımı, 28.2). `--tarayıcıyla_eşle` verilince Dost tarayıcıyı `http://127.0.0.1:<kapı>/#p=<kod>` ile açar. Parça sunucuya gitmez. Sayfa onu ilk iş olarak okur ve `history.replaceState` ile adresten siler; ancak ondan sonra bir istek gönderir, önden işlenirken (`document.prerendering`) göndermez. Kod tek kullanımlık ve 120 saniyeliktir; sayfa onu yalnız `GET /pair`'in `Dost-Pair-Code` üstbilgisinde kullanır. Sayfa başka parça ya da parametre okumaz. Eski `source` parametresi gerçek kipte zaten sayfanın köküne eşit olmak zorundaydı, bilgi katmıyordu; kalkınca dışarıdan verilen bir girdi de kalktı.

- `file://` ile açılan sayfa sahte akışla çalışır ve üstte "sahte kip" sürekli yazılıdır (`Origin` `null` olur, sunucu ona izin vermez). Gerçek kipte sayfayı TR sunar.
- Gerçek kipe yalnız `http://127.0.0.1:<kapı>` kökünden açılan sayfa geçer. `localhost`, başka adres ve `https` reddedilir: ekran sahte akışa düşmez, bağlantı yok der ve reddedilen kökü gösterir.
- Kaynak sayfayı sunan kapıdır, başka kapı reddedilir: belirteç yalnız ona gider. Yoksa bir bağlantı kullanıcının girdiği belirteci aynı makinedeki başka bir sürece gönderirdi. Sayfanın `connect-src`'u yerel adresle sınırlıdır, `GET /` üstbilgisi onu sayfanın kendi kapısına daraltır ("Güvenlik").

## Olay akışı

`GET /events` şu çerçeveyle akar:

```text
id: 3f9a0c2e7b1d4e65-42
data: {"v":2,"type":"turn_text","data":{"turn_id":"t-7","role":"assistant","mode":"append","text":"Mer"}}

```

- `id` olayın kimliğidir ve metindir: `<ön ek>-<sıra>` (dil tanımı, 28.7). Ön ek 16 küçük onaltılık karakterdir, sunucu her açılışta yeniden rastgele üretir; sıra 1'den birer artar. Veri kimlik taşımaz. Olay adı (`event:`) kullanılmaz; tür JSON'un `type` alanındadır.
- `heartbeat` dışındaki her olayın `id`'si vardır.
- Bir olay en çok 1 MiB'dir. Ekran daha uzun çerçeveyi okumaz, bağlantıyı kapatıp `event_too_large` gösterir.
- Tarayıcının `EventSource`'u üstbilgi taşıyamaz ve belirteç URL'ye yazılmaz; bu yüzden ekran akışı `fetch` ile okur. Çerçeve aynı SSE'dir.

### Kopma ve yeniden bağlanma

- Ekran son gördüğü `id`'yi olduğu gibi saklar ve `Last-Event-ID` üstbilgisinde aynen geri gönderir; ön eki kendisi üretmez. Karşılaştırmak için yalnız son tireden sonraki sırayı sayı olarak okur.
- **Sunucu o `id`'den sonrasını tutuyorsa** yalnız eksik olayları sırayla yollar, `session` yollamaz. En az son 1024 olay tutulur.
- **Tutmuyorsa** (`id` yok, çok eski, başka ön ekli ya da sunucu yeniden başladı) ilk olay `session`'dır ve tam durumu taşır. Sunucu yeniden başlayınca ön ek değişir ve sıra 1'den başlar: ön eki tutmayan `Last-Event-ID`'ye sunucu geri oynatmaz, önce `session`'ı verir.
- `session` hiç atılmaz, ön eki farklı ya da sırası küçük olsa da: ekran son kimliği `session`'ın `id`'sine kurar.
- `session` dışındaki olay, ön eki son kimliğinkinden farklıysa ya da sırası ondan büyük değilse uygulanmaz. Ön eki farklıysa ekran bağlantıyı kapatır ve `Last-Event-ID`'siz yeniden bağlanır (`session` gelir). Ekranın henüz son kimliği yoksa (ilk bağlantı) ilk olay `session`'dır; ondan önce gelen başka olay da uygulanmaz, ekran aynı biçimde yeniden bağlanır. Sağlam sunucudan böyle bir olay gelmez; uygulamak yeni sunucunun olaylarını eski durumun üstüne yazardı.
- Nabız SSE yorumu değil olaydır: tarayıcı kodu yorumu göremez, sessiz kopma ancak böyle anlaşılır. `session`'daki `heartbeat_ms` aralığın iki buçuk katı boyunca hiçbir bayt gelmezse ekran bağlantıyı kopmuş sayar.
- Ekran artan bekleyişle (en çok 10 sn) yeniden dener. `401` ve `403`'te denemez: kullanıcıya sorar ya da nedenini gösterir.

## Olaylar

### `session`

Bağlantının ilk olayı (yukarıdaki "tutmuyorsa" durumu). Ekran elindeki her şeyi siler ve `snapshot`'ı sırayla işler.

```json
{"v":2,"type":"session","data":{
  "session_id":"s-example-1",
  "server_version":"0.0.0-example",
  "model":"example-model-4b (made up)",
  "tools":[
    {"name":"example_read_file","irreversible":false},
    {"name":"example_delete_file","irreversible":true}
  ],
  "heartbeat_ms":15000,
  "snapshot":[
    {"type":"state","data":{"state":"idle","mic":false}},
    {"type":"turn_start","data":{"turn_id":"t-6","source":"text"}},
    {"type":"turn_text","data":{"turn_id":"t-6","role":"user","mode":"replace","text":"Example question","final":true}},
    {"type":"turn_end","data":{"turn_id":"t-6","reason":"done"}}
  ]
}}
```

- `session`'ın `id`'si görüntüye katılan son olayınkidir; daha olay yoksa `<ön ek>-0`'dır.
- `snapshot` öbür olayların aynısıdır, kimliksiz ve sıkıştırılmış: bir konuşma parçası tek `turn_text`, `heartbeat` ve `session` yoktur. Ekranda ikinci bir durum biçimi yok, aynı kod işler.
- `snapshot` bir `state` olayı, son ölçüler için bir `metrics` olayı, ayar dosyası varsa son `settings` olayı, son yüklemenin `model_load_failed` hatası (sonraki başarılı yüklemeye kadar; önceki açılışta yarıda kalan yükleme de) ve **açık her onay için bir `approval_open`** taşır. Sığmazsa eski turlar düşer; açık onay düşmez.
- Ekranda açık kart varken gelen `snapshot` o kartı taşımıyorsa kart kalkar ve ekran onayın geçersiz kaldığını yazar: onaylanmış sayılmaz.

### `state`

```json
{"v":2,"type":"state","data":{"state":"loading","mic":false,"progress":0.4}}
```

- `state`: `setup`, `idle`, `loading`, `listening`, `thinking`, `speaking`, `error`.
  - `setup`: yüklenebilir bir model takımı yok (dil modeli ya da konuşma dilinin modelleri kurulu değil); metin ve ses turu `not_ready` alır.
- `mic`: mikrofon gerçekten açık mı. Ekran düğmeyi yalnız bu alana göre çizer, komutu gönderdiği an değil.
- `progress`: yalnız `loading`'de, 0 ile 1 arası; bilinmiyorsa yok.
- `model`: yalnız `loading`'de, yüklenen dil modelinin katalog kimliği.

### `turn_start`

```json
{"v":2,"type":"turn_start","data":{"turn_id":"t-7","source":"voice"}}
```

`source`: `text` ya da `voice`.

### `turn_text`

```json
{"v":2,"type":"turn_text","data":{"turn_id":"t-7","role":"user","mode":"replace","text":"Bugün hava nasıl","final":false}}
{"v":2,"type":"turn_text","data":{"turn_id":"t-7","role":"assistant","mode":"append","text":"Örnek "}}
```

- `role`: `user` (söz) ya da `assistant` (cevap).
- `mode`: `replace` eski metnin yerine geçer, `append` sonuna eklenir. Konuşma tanıma taslağı `replace` ile gelir, çünkü tanıyıcı önceki sözcükleri düzeltir; cevap parçası `append` ile gelir.
- `final`: yalnız `user`'da. `false` taslaktır, ekran soluk gösterir; `true` sözün son halidir.

### `turn_end`

```json
{"v":2,"type":"turn_end","data":{"turn_id":"t-7","reason":"stopped"}}
```

`reason`: `done`, `stopped`, `error`. `error`'da `error` alanı `{"code","values"}` taşır.

### `tool_start`

```json
{"v":2,"type":"tool_start","data":{"call_id":"c-3","turn_id":"t-7","tool":"example_read_file"}}
```

### `tool_end`

```json
{"v":2,"type":"tool_end","data":{"call_id":"c-3","result":"ok","duration_ms":124,"output":"example line\n","output_bytes":13,"truncated":false}}
```

- `result`: `ok`, `error`, `denied`, `stopped`. `error`'da `error` alanı `{"code","values"}` taşır.
- `output` en çok 64 KiB. Kesildiyse `truncated` `true`'dur ve `output_bytes` kesilmeden önceki boyuttur; ekran bunu söyler.
- `output` güvenilmez girdidir: ekran metin olarak basar.

### `approval_open`

Geri alınamaz iş için onay ister. Kart yalnız bu olaydan gelir; model metni kartın düzenine giremez.

```json
{"v":2,"type":"approval_open","data":{
  "approval_id":"a-2",
  "turn_id":"t-7",
  "call_id":"c-4",
  "tool":"example_delete_file",
  "args":"{\"path\":\"/example/folder/made-up.txt\"}",
  "args_bytes":38,
  "args_truncated":false,
  "expires_at":"2026-01-01T09:00:30Z"
}}
```

- `args` aracın alacağı **gerçek argümanın** metnidir, modelin özeti değil; en çok 64 KiB. Kesildiyse `args_truncated` `true`'dur, `args_bytes` tam boyuttur ve ekran bunu söyler. JSON olması beklenmez: kesilmiş metin geçerli JSON değildir.
- Ekran `args` ve `tool` içindeki denetim, yön ve görünmez karakterleri `[U+202E]` biçiminde yazar: yön karakteri bir dosya adını ters gösterip onayı yanlış şeye aldırabilir.
- `expires_at`: bu andan sonra sunucu onayı kabul etmez. Ekrandaki sayaç yalnız gösterir, karar sunucudadır.
- Kart `approval_close` gelene kadar kalır. Süre dolsa, bağlantı kopsa da kalkmaz.

### `approval_close`

```json
{"v":2,"type":"approval_close","data":{"approval_id":"a-2","reason":"answered","decision":"deny"}}
```

- `reason`: `answered`, `expired`, `stopped`.
- `decision`: yalnız `answered`'da, `approve` ya da `deny`.
- Ekran bu onay için komut göndermediyse cevabın öbür sekmeden geldiğini anlar ve öyle yazar. Sunucunun sekmeleri tanıması gerekmez; olaylar her sekmeye gider, ilk cevap geçerlidir.

### `metrics`

```json
{"v":2,"type":"metrics","data":{"memory_bytes":4617089843,"tokens_per_second":11.2,"first_token_ms":910}}
```

Bilinmeyen alan yazılmaz; ekran o ölçüyü boş gösterir. Ekranda sabit ölçü yoktur.

### `error`

Bir tura bağlı olmayan ya da sürmekte olan işi bitirmeyen hata.

```json
{"v":2,"type":"error","data":{"code":"out_of_memory","values":{"needed_bytes":6442450944},"fatal":false}}
```

`fatal` `true` ise sunucu çalışmaya devam edemez; ekran `error` durumunu gösterir.

### `settings`

```json
{"v":2,"type":"settings","data":{"language_model":"qwen3-4b-q4km","speech_language":"tr","system_prompt":{"source":"catalog"},"context":8192,"device":"cpu","port":47863,"machine":{"memory_bytes":17179869184,"memory_source":"option"}}}
```

Ayarın hepsi (`ayar.json`'un alanları, `v` dışında) ve ayarın hesaplandığı makinenin özeti `machine`. `snapshot`'ta ve her değişimde gelir. Ayar dosyası yoksa gelmez; durum `setup`'tır.

### `heartbeat`

```text
data: {"v":2,"type":"heartbeat"}

```

`id` taşımaz. `session`'daki `heartbeat_ms` aralığıyla gelir: 15 saniye (dil tanımı, 28.7).

## Komutlar

`POST /command`, gövde tek JSON nesnesi, en çok 64 KiB:

```json
{"v":2,"command_id":"7f3a9c1e5d20b846","type":"approval_answer","data":{"approval_id":"a-2","decision":"approve"}}
```

Cevap her zaman JSON:

```json
{"v":2,"command_id":"7f3a9c1e5d20b846","status":"accepted"}
{"v":2,"command_id":"7f3a9c1e5d20b846","status":"rejected","error":{"code":"already_answered","values":{"approval_id":"a-2"}}}
```

- `accepted` komutun alındığı demektir, işin bittiği değil. Sonuç olaylarla gelir: mikrofon `state` ile, onay `approval_close` ile.
- Aynı `command_id` yeniden gelirse sunucu işi tekrarlamaz, ilk cevabı döner. Ekran bir komut denemesini aynı kimlikle yineler; böylece kopan bağlantıda ya da çift tıklamada iş iki kez yapılmaz.
- Bir onay yalnız bir kez cevaplanır: ikinci cevap, kimliği başka olsa da `already_answered` alır.

| `type` | `data` | Ne yapar |
| --- | --- | --- |
| `send_text` | `{"text":"..."}` | Metinle yeni tur açar; metin en çok 32 KiB |
| `mic_set` | `{"on":true}` | Mikrofonu açar ya da kapar |
| `approval_answer` | `{"approval_id":"...","decision":"approve"}` | Onay cevabı: `approve` ya da `deny` |
| `stop` | `{}` | Sürmekte olan turu durdurur; açık onaylar `stopped` ile kapanır |
| `model_load` | `{"entry_id":"..."}` | Dil modelini ve konuşma dilinin modellerini yükler; tur sürüyorsa `turn_busy` |
| `settings_set` | `{"key":"...","value":...}` | Listedeki bir ayarı değiştirir; listede olmayan `setting_invalid` |

Araçlar, arama ve odak ekranın yerel görünümleridir, komut göndermez: araç listesi `session`'dan gelir, arama ekrandaki satırlarda yapılır.

## Hata kodları

Her kodun karşılığı ekranın sözlüğündedir (`error_<kod>`); bilinmeyen kod `error_unknown` ile kodun kendisini gösterir.

Sunucudan gelenler:

| Kod | Değerler | Ne zaman |
| --- | --- | --- |
| `bad_request` | | Gövde ya da alan geçersiz |
| `unsupported_version` | `version` | `v` desteklenmiyor |
| `unknown_command` | `type` | Komut türü bilinmiyor |
| `unknown_approval` | `approval_id` | Böyle açık bir onay yok |
| `already_answered` | `approval_id` | Onay daha önce cevaplandı |
| `approval_expired` | `approval_id` | Cevap süre dolduktan sonra geldi |
| `turn_busy` | `turn_id` | Sürmekte olan tur varken yeni tur istendi |
| `text_too_long` | `limit_bytes` | Metin sınırı aştı |
| `not_ready` | | Modeller kurulu değilken ya da yüklenirken yeni tur istendi |
| `mic_unavailable` | | Mikrofon açılamadı |
| `model_load_failed` | `model` | Model yüklenemedi |
| `out_of_memory` | `needed_bytes` | Bellek yetmedi |
| `unknown_entry` | `entry_id` | Katalogda yok |
| `not_installed` | `entry_id` | Yüklenecek dosyalar yok |
| `setting_invalid` | `key`, `reason` | Ayar listede yok ya da değeri geçersiz. `reason` üç değerden biri: `unknown_key` (anahtar ayar listesinde değil), `invalid_value` (değerin türü anahtarın istediği değil: `speech_language` metin değil, `system_prompt` nesne değil), `system_prompt_language` (kullanıcının sistem iletisi başka dil için yazılmış) |
| `unknown_language` | `language` | Whisper bu dili tanımıyor ya da katalogda bu dilin modelleri yok |
| `tool_failed` | `tool` | Araç hatayla bitti |
| `tool_timeout` | `tool`, `limit_ms` | Araç süresini aştı |
| `internal` | | Sunucunun iç hatası |
| `unauthorized` | | Belirteç yok ya da yanlış (`401`) |
| `forbidden_origin` | | `Origin` ya da `Host` kabul edilmedi (`403`) |
| `pair_code_expired` | | Eşleme kodunun süresi doldu (`GET /pair`, `401`) |
| `pair_code_unknown` | | Eşleme kodu bilinmiyor (`GET /pair`, `401`) |
| `pair_code_used` | | Eşleme kodu daha önce kullanıldı; Dost olayı günlüğe yazar (`GET /pair`, `401`) |
| `too_many_requests` | | Yanlış eşleme kodu sınırı doldu ya da yeni kod için 10 saniye geçmedi (`GET /pair`, `GET /pair/renew`, `429`) |
| `unsupported_media_type` | | `Content-Type` `application/json` değil (`415`) |
| `body_too_large` | `limit_bytes` | Gövde sınırı aştı (`413`) |

Ekranın kendi ürettikleri:

| Kod | Değerler | Ne zaman |
| --- | --- | --- |
| `connection_lost` | | Akış kesildi ya da komut gönderilemedi |
| `source_rejected` | `source` | Sayfanın kökü gerçek kaynak olamaz (`localhost`, başka adres, `https`) |
| `protocol_mismatch` | `got`, `want` | Sunucunun sürümü başka |
| `event_too_large` | `limit_bytes` | Çerçeve sınırı aştı |
| `bad_event` | | Çerçeve geçerli JSON değil |

## Güvenlik

Herhangi bir web sitesi kullanıcının tarayıcısından `127.0.0.1`'e istek atabilir: korunmayan bir sunucuda sahte onay verebilir, mikrofonu açabilir. Bu yüzden:

- **Bağlanma:** sunucu yalnız `127.0.0.1`'e bağlanır, `0.0.0.0`'a bağlanmaz.
- **`Host`:** tam olarak `127.0.0.1:<kapı>` olmalı. Başka değer `403`: DNS yeniden bağlama saldırısı böyle kesilir.
- **`Origin`:** varsa tam olarak `http://127.0.0.1:<kapı>` olmalı. `Origin: null` kabul edilmez. `POST`'ta `Origin` yoksa da ret. Aynı kökten `GET`'te tarayıcı üstbilgiyi göndermeyebilir, bu yüzden `GET` için zorunlu değildir.
- **Kimlik:** `/events` ve `/command` `Authorization: Bearer <belirteç>` ister. TR açılırken rastgele belirteç üretir ve yalnız kendisini çalıştıran kullanıcının görebileceği yere yazar. Ekran belirteci eşleme kodundan alır (`GET /pair`, `--tarayıcıyla_eşle` ile); seçenek yoksa ya da eşleme olmazsa kullanıcı belirteci kutuya girer. İki yolda da belirteci yalnız sekmenin oturum belleğinde (`sessionStorage`) tutar, kalıcı depoda (`localStorage`, çerez) tutmaz; `401`'de ve `protocol_mismatch`'te siler. Belirteç URL'de olmaz. URL'de olan eşleme kodu tek kullanımlık ve 120 saniyeliktir: geçmişe ya da günlüğe düşen kod işe yaramaz. Belirteci dosyadan okuyan istemciler (ajan, komut satırı) bugünkü gibi çalışır. Sayfada da olmaz: sayfayı kimlik sormadan sunan uç herkese açıktır.
- **`Content-Type`:** `POST /command` yalnız `application/json` kabul eder. Düz form ön uçuş isteği olmadan gönderilebildiği için öbürleri `415`.
- **CORS yok:** sunucu `Access-Control-Allow-*` göndermez, `OPTIONS` kabul etmez.
- **Sunulan dosyalar:** yalnız ekran ve sayfanın listelediği dil dosyaları, `Content-Type: text/javascript`. Sayfanın politikası betiği `/lang/` altından kabul eder; başka betik sunulmaz.
- **Gizli değer:** olaylar tele çıkmadan aynı süzgeçten geçer (dil tanımı 19.4): model metni, araç çıktısı, onay argümanı, `error.values`, `snapshot` ve kullanıcının sözü dahil. Argüman süzüldüyse kart `***` gösterir; ekran gerçek değeri hiç almaz.
  - **Parçalı akış:** olay başına süzgeç iki `append` parçasına bölünen değeri görmez. Sunucu akışın parçalarını önce maske akışından geçirir (`TR_mask_stream()`, dil tanımı 19.4a), akışın sonunda `.TR_finish()`'in verdiğini de `turn_end`'den önce gönderir.
  - **Sıra:** süzme kesmeden önce yapılır (64 KiB ve 1 MiB sınırı). Önce kesilirse değerin başı kalır, süzgeç onu tanımaz ve tele çıkar.
- **Boyut:** olay en çok 1 MiB, araç çıktısı ve onay argümanı 64 KiB, komut gövdesi 64 KiB. Sunucu kesince söyler; sessiz kesme yok.
- **Çerçeve ve bağlantı:** `frame-ancestors` meta etiketinde çalışmaz; sunucu `GET /` cevabında üstbilgiyle gönderir: `Content-Security-Policy: frame-ancestors 'none'; connect-src 'self'`. Sayfa başka bir sitenin içinde açılıp tıklatılamaz. Üstbilgi ile meta etiketi birlikte geçerlidir, bu yüzden sayfa yalnız kendi kapısına bağlanabilir.
- **Önbellek:** `GET /`, `/lang/` ve `/events` `Cache-Control: no-store`, ayrıca `X-Content-Type-Options: nosniff`.
- **Onay:** karar sunucudadır. Ekranın gönderdiği cevap yalnız bir istektir; süre, `approval_id` ve geri alınamaz işin kendisi sunucuda denetlenir. Onaylanmayan araç hiç çalışmaz.

## Emsaller

Tartıda bakılacak. MCP'nin adı geçen bölümleri ve DAP'ın temel ileti biçimi 2026-11-06'da okundu; SSE kuralları MCP'nin gösterdiği yerdedir.

- **SSE:** [HTML standardı](https://html.spec.whatwg.org/multipage/server-sent-events.html): `id`, `Last-Event-ID`.
- **MCP 2025-06-18:** [Streamable HTTP](https://modelcontextprotocol.io/specification/2025-06-18/basic/transports). "Security Warning" (`Origin` denetimi, yalnız `127.0.0.1`, kimlik doğrulama) ve "Resumability and Redelivery" (kopunca kaldığı yerden sürme) bu sözleşmenin kaynağıdır. Fark: burada JSON-RPC yok, akış tek ve her sekmeye aynıdır.
- **DAP:** [Debug Adapter Protocol](https://microsoft.github.io/debug-adapter-protocol/specification). Her iletide `seq`, olayda ad ve gövde, cevapta isteğe bağ ve başarı alanı; hata için kısa kod. `type`, `data` ve `command_id` buradan gelir; sıra burada SSE'nin `id` satırındadır (28.7), veride değil. Depoda emsali İzleme Masası'dır.
