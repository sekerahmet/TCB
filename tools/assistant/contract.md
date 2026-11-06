# Ekran sözleşmesi (sürüm 1, öneri)

Yerel asistanın ekranı TR'den ne bekliyorsa burada yazılıdır. Ekran yalnız bu sözleşmeyle konuşur; sahte akış da aynı baytları üretir. Bu bir öneridir: TR'de HTTP sunucusu henüz yok, sözleşme HTTP bölümü yazılırken tartılır.

Tel İngilizcedir, hiçbir insan diline ayrıcalık yok. Ekrandaki yazılar telden gelmez: sunucu kod ve değer gönderir, ekran kendi sözlüğünden kullanıcının diliyle gösterir. Model metni, araç adı ve araç çıktısı veridir, çevrilmez.

Bütün örnekler uydurmadır.

## Genel

- **Uçlar:** `GET /events` (server-sent events), `POST /command`, `GET /` (ekranın kendisi).
- **Biçim:** JSON, UTF-8.
- **Sürüm:** her olayda ve her komutta `v` alanı. Bugün `1`. Ekran başka bir sürüm görürse olayı işlemez, `protocol_mismatch` gösterir ve komut göndermez.
- **Bilinmeyen alan** yok sayılır. **Bilinmeyen olay türü** yok sayılır: yeni olay eski ekranı kırmaz.
- **Süreler** milisaniye, **boyutlar** bayt, **zaman** UTC RFC 3339.
- **Hata** metin taşımaz: `{"code": "...", "values": {...}}`. Değer adı `_ms` ile bitiyorsa süre, `_bytes` ile bitiyorsa boyuttur; ekran onu kullanıcının diline göre biçimler.

## Kaynak

Ekran kaynağını `source` parametresinden alır: `index.html?source=http://127.0.0.1:8123`.

- Parametre yoksa sahte akış çalışır ve üstte "sahte kip" sürekli yazılıdır.
- Kabul edilen tek biçim `http://127.0.0.1:<kapı>`: kullanıcı adı, yol, sorgu ve parça olmaz. `localhost`, başka adres ve `https` reddedilir; `connect-src` de yalnız bunu açar.
- Reddedilen kaynakta ekran sahte akışa düşmez: bağlantı yok der ve reddedilen kaynağı gösterir.
- `file://` ile açılan sayfa yalnız sahte kipte çalışır (`Origin` `null` olur, sunucu ona izin vermez). Gerçek kipte sayfayı TR sunar.

## Olay akışı

`GET /events` şu çerçeveyle akar:

```text
id: 42
data: {"v":1,"seq":42,"type":"turn_text","data":{"turn_id":"t-7","role":"assistant","mode":"append","text":"Mer"}}

```

- `id` ve `seq` aynı sayıdır, oturum içinde birer artar. Olay adı (`event:`) kullanılmaz; tür JSON'un `type` alanındadır.
- `heartbeat` dışındaki her olayın `id`'si vardır.
- Bir olay en çok 1 MiB'dir. Ekran daha uzun çerçeveyi okumaz, bağlantıyı kapatıp `event_too_large` gösterir.
- Tarayıcının `EventSource`'u üstbilgi taşıyamaz ve belirteç URL'ye yazılmaz; bu yüzden ekran akışı `fetch` ile okur. Çerçeve aynı SSE'dir.

### Kopma ve yeniden bağlanma

- Ekran son gördüğü `id`'yi `Last-Event-ID` üstbilgisiyle gönderir.
- **Sunucu o `id`'den sonrasını tutuyorsa** yalnız eksik olayları sırayla yollar, `session` yollamaz. En az son 1024 olay tutulur.
- **Tutmuyorsa** (`id` yok, çok eski ya da sunucu yeniden başladı) ilk olay `session`'dır ve tam durumu taşır.
- Ekran `seq`'i kendi son `seq`'inden büyük olmayan olayı atar. `session` hiç atılmaz: sayacı kendi `seq`'ine kurar.
- Nabız SSE yorumu değil olaydır: tarayıcı kodu yorumu göremez, sessiz kopma ancak böyle anlaşılır. `session`'daki `heartbeat_ms` aralığın iki buçuk katı boyunca hiçbir bayt gelmezse ekran bağlantıyı kopmuş sayar.
- Ekran artan bekleyişle (en çok 10 sn) yeniden dener. `401` ve `403`'te denemez: kullanıcıya sorar ya da nedenini gösterir.

## Olaylar

### `session`

Bağlantının ilk olayı (yukarıdaki "tutmuyorsa" durumu). Ekran elindeki her şeyi siler ve `snapshot`'ı sırayla işler.

```json
{"v":1,"seq":1,"type":"session","data":{
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

- `snapshot` öbür olayların aynısıdır, `seq`'siz ve sıkıştırılmış: bir konuşma parçası tek `turn_text`, `heartbeat` ve `session` yoktur. Ekranda ikinci bir durum biçimi yok, aynı kod işler.
- `snapshot` bir `state` olayı, son ölçüler için bir `metrics` olayı ve **açık her onay için bir `approval_open`** taşır. Sığmazsa eski turlar düşer; açık onay düşmez.
- Ekranda açık kart varken gelen `snapshot` o kartı taşımıyorsa kart kalkar ve ekran onayın geçersiz kaldığını yazar: onaylanmış sayılmaz.

### `state`

```json
{"v":1,"seq":2,"type":"state","data":{"state":"loading","mic":false,"progress":0.4}}
```

- `state`: `idle`, `loading`, `listening`, `thinking`, `speaking`, `error`.
- `mic`: mikrofon gerçekten açık mı. Ekran düğmeyi yalnız bu alana göre çizer, komutu gönderdiği an değil.
- `progress`: yalnız `loading`'de, 0 ile 1 arası; bilinmiyorsa yok.

### `turn_start`

```json
{"v":1,"seq":3,"type":"turn_start","data":{"turn_id":"t-7","source":"voice"}}
```

`source`: `text` ya da `voice`.

### `turn_text`

```json
{"v":1,"seq":4,"type":"turn_text","data":{"turn_id":"t-7","role":"user","mode":"replace","text":"Bugün hava nasıl","final":false}}
{"v":1,"seq":9,"type":"turn_text","data":{"turn_id":"t-7","role":"assistant","mode":"append","text":"Örnek "}}
```

- `role`: `user` (söz) ya da `assistant` (cevap).
- `mode`: `replace` eski metnin yerine geçer, `append` sonuna eklenir. Konuşma tanıma taslağı `replace` ile gelir, çünkü tanıyıcı önceki sözcükleri düzeltir; cevap parçası `append` ile gelir.
- `final`: yalnız `user`'da. `false` taslaktır, ekran soluk gösterir; `true` sözün son halidir.

### `turn_end`

```json
{"v":1,"seq":20,"type":"turn_end","data":{"turn_id":"t-7","reason":"stopped"}}
```

`reason`: `done`, `stopped`, `error`. `error`'da `error` alanı `{"code","values"}` taşır.

### `tool_start`

```json
{"v":1,"seq":5,"type":"tool_start","data":{"call_id":"c-3","turn_id":"t-7","tool":"example_read_file"}}
```

### `tool_end`

```json
{"v":1,"seq":6,"type":"tool_end","data":{"call_id":"c-3","result":"ok","duration_ms":124,"output":"example line\n","output_bytes":13,"truncated":false}}
```

- `result`: `ok`, `error`, `denied`, `stopped`. `error`'da `error` alanı `{"code","values"}` taşır.
- `output` en çok 64 KiB. Kesildiyse `truncated` `true`'dur ve `output_bytes` kesilmeden önceki boyuttur; ekran bunu söyler.
- `output` güvenilmez girdidir: ekran metin olarak basar.

### `approval_open`

Geri alınamaz iş için onay ister. Kart yalnız bu olaydan gelir; model metni kartın düzenine giremez.

```json
{"v":1,"seq":7,"type":"approval_open","data":{
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
{"v":1,"seq":8,"type":"approval_close","data":{"approval_id":"a-2","reason":"answered","decision":"deny"}}
```

- `reason`: `answered`, `expired`, `stopped`.
- `decision`: yalnız `answered`'da, `approve` ya da `deny`.
- Ekran bu onay için komut göndermediyse cevabın öbür sekmeden geldiğini anlar ve öyle yazar. Sunucunun sekmeleri tanıması gerekmez; olaylar her sekmeye gider, ilk cevap geçerlidir.

### `metrics`

```json
{"v":1,"seq":21,"type":"metrics","data":{"memory_bytes":4617089843,"tokens_per_second":11.2,"first_token_ms":910}}
```

Bilinmeyen alan yazılmaz; ekran o ölçüyü boş gösterir. Ekranda sabit ölçü yoktur.

### `error`

Bir tura bağlı olmayan ya da sürmekte olan işi bitirmeyen hata.

```json
{"v":1,"seq":22,"type":"error","data":{"code":"out_of_memory","values":{"needed_bytes":6442450944},"fatal":false}}
```

`fatal` `true` ise sunucu çalışmaya devam edemez; ekran `error` durumunu gösterir.

### `heartbeat`

```text
data: {"v":1,"type":"heartbeat"}

```

`id` ve `seq` taşımaz. `session`'daki `heartbeat_ms` aralığıyla gelir.

## Komutlar

`POST /command`, gövde tek JSON nesnesi, en çok 64 KiB:

```json
{"v":1,"command_id":"7f3a9c1e5d20b846","type":"approval_answer","data":{"approval_id":"a-2","decision":"approve"}}
```

Cevap her zaman JSON:

```json
{"v":1,"command_id":"7f3a9c1e5d20b846","status":"accepted"}
{"v":1,"command_id":"7f3a9c1e5d20b846","status":"rejected","error":{"code":"already_answered","values":{"approval_id":"a-2"}}}
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
| `mic_unavailable` | | Mikrofon açılamadı |
| `model_load_failed` | `model` | Model yüklenemedi |
| `out_of_memory` | `needed_bytes` | Bellek yetmedi |
| `tool_failed` | `tool` | Araç hatayla bitti |
| `tool_timeout` | `tool`, `limit_ms` | Araç süresini aştı |
| `internal` | | Sunucunun iç hatası |
| `unauthorized` | | Belirteç yok ya da yanlış (`401`) |
| `forbidden_origin` | | `Origin` ya da `Host` kabul edilmedi (`403`) |
| `unsupported_media_type` | | `Content-Type` `application/json` değil (`415`) |
| `body_too_large` | `limit_bytes` | Gövde sınırı aştı (`413`) |

Ekranın kendi ürettikleri:

| Kod | Değerler | Ne zaman |
| --- | --- | --- |
| `connection_lost` | | Akış kesildi ya da komut gönderilemedi |
| `source_rejected` | `source` | `source` parametresi kurala uymuyor |
| `protocol_mismatch` | `got`, `want` | Sunucunun sürümü başka |
| `event_too_large` | `limit_bytes` | Çerçeve sınırı aştı |
| `bad_event` | | Çerçeve geçerli JSON değil |

## Güvenlik

Herhangi bir web sitesi kullanıcının tarayıcısından `127.0.0.1`'e istek atabilir: korunmayan bir sunucuda sahte onay verebilir, mikrofonu açabilir. Bu yüzden:

- **Bağlanma:** sunucu yalnız `127.0.0.1`'e bağlanır, `0.0.0.0`'a bağlanmaz.
- **`Host`:** tam olarak `127.0.0.1:<kapı>` olmalı. Başka değer `403`: DNS yeniden bağlama saldırısı böyle kesilir.
- **`Origin`:** varsa tam olarak `http://127.0.0.1:<kapı>` olmalı. `Origin: null` kabul edilmez. `POST`'ta `Origin` yoksa da ret. Aynı kökten `GET`'te tarayıcı üstbilgiyi göndermeyebilir, bu yüzden `GET` için zorunlu değildir.
- **Kimlik:** `/events` ve `/command` `Authorization: Bearer <belirteç>` ister. TR açılırken rastgele belirteç üretir ve yalnız kendisini çalıştıran kullanıcının görebileceği yere yazar. Ekran kullanıcıdan alır, yalnız bellekte tutar, diske yazmaz. Belirteç URL'de olmaz: adres geçmişe ve günlüklere düşer. Sayfada da olmaz: sayfayı kimlik sormadan sunan uç herkese açıktır.
- **`Content-Type`:** `POST /command` yalnız `application/json` kabul eder. Düz form ön uçuş isteği olmadan gönderilebildiği için öbürleri `415`.
- **CORS yok:** sunucu `Access-Control-Allow-*` göndermez, `OPTIONS` kabul etmez.
- **Gizli değer:** olaylar tele çıkmadan aynı süzgeçten geçer (dil tanımı 19.4): model metni, araç çıktısı ve onay argümanı dahil. Argüman süzüldüyse kart `***` gösterir; ekran gerçek değeri hiç almaz.
- **Boyut:** olay en çok 1 MiB, araç çıktısı ve onay argümanı 64 KiB, komut gövdesi 64 KiB. Sunucu kesince söyler; sessiz kesme yok.
- **Çerçeve:** `frame-ancestors` meta etiketinde çalışmaz; sunucu `GET /` cevabında üstbilgiyle gönderir: `Content-Security-Policy: frame-ancestors 'none'`. Sayfa başka bir sitenin içinde açılıp tıklatılamaz.
- **Önbellek:** `GET /` ve `/events` `Cache-Control: no-store`, ayrıca `X-Content-Type-Options: nosniff`.
- **Onay:** karar sunucudadır. Ekranın gönderdiği cevap yalnız bir istektir; süre, `approval_id` ve geri alınamaz işin kendisi sunucuda denetlenir. Onaylanmayan araç hiç çalışmaz.

## Emsaller

Tartıda bakılacak. MCP'nin adı geçen bölümleri ve DAP'ın temel ileti biçimi 2026-11-06'da okundu; SSE kuralları MCP'nin gösterdiği yerdedir.

- **SSE:** [HTML standardı](https://html.spec.whatwg.org/multipage/server-sent-events.html): `id`, `Last-Event-ID`.
- **MCP 2025-06-18:** [Streamable HTTP](https://modelcontextprotocol.io/specification/2025-06-18/basic/transports). "Security Warning" (`Origin` denetimi, yalnız `127.0.0.1`, kimlik doğrulama) ve "Resumability and Redelivery" (kopunca kaldığı yerden sürme) bu sözleşmenin kaynağıdır. Fark: burada JSON-RPC yok, akış tek ve her sekmeye aynıdır.
- **DAP:** [Debug Adapter Protocol](https://microsoft.github.io/debug-adapter-protocol/specification). Her iletide `seq`, olayda ad ve gövde, cevapta isteğe bağ ve başarı alanı; hata için kısa kod. `seq`, `type`, `data` ve `command_id` buradan gelir. Depoda emsali İzleme Masası'dır.
