# Asistan ekranı

TR'de yazılacak yerel asistanın ekranı ve TR'yle konuşacağı sözleşme. Ekran incedir: tek sayfa ve dil dosyaları, düz JavaScript, derleme yok, dışarıdan hiçbir şey yüklemez. TR'ye HTTP ile bağlanır; pencere çizmek TR'nin işi değildir.

Ekranı TR programı sunar (TR'nin HTTP sunucusu ve olay yayıncısı, dil tanımı 28). Sunucusuz açılan ekran **sahte olay akışıyla** çalışır ve bunu üstte sürekli yazar; sahte akıştaki her şey uydurmadır.

## Dosyalar

- `index.html`: ekran. Biçim, kod ve sahte akış bu tek dosyada.
- `lang/<kod>.js`: çeviriler, her dile bir dosya. Yalnız metin taşır.
- `contract.md`: ekranın TR'den beklediği her şey (sürüm 2). Asıl değer budur.
- `behavior_screen_test.go`: aşağıdaki kuralları korur.
- `asistan.tr`, `zincir.tr`, `zincir_test.tr`: ekransız sesli zincir (aşağıda, "Zincir").
- `deneme.tr`, `deneme_test.tr`: canlı denemenin kaydı (aşağıda, "Deneme kaydı"); test modelsizdir.
- `screen_demo.tr`: ekranı modelsiz açar (sahte zincir: metin turu sözü yineleyen cevabı sözcük sözcük akıtır); ekranı gerçek sunucuyla denemek ve uçtan uca test içindir.
- `behavior_screen_server_test.go`: `screen_demo.tr`'yi `trlang` ile açar, gerçek bağlantıyla konuşur: belirteçsiz yollar, `401`, `403`, akıştan bir tur, `Last-Event-ID` ile yeniden bağlanma, katlama yavaşken aboneliğin bir gönderime denk gelmesi (parça ekranda tam bir kez).
- `ekran.tr`, `ekran_sunucu.tr`, `ekran_test.tr`, `ekran_tur_test.tr`: ekranın TR yanı (aşağıda, "Ekranın sunucusu"): `ekran.tr` olayları yayar ve komutları karara bağlar, `ekran_sunucu.tr` sayfayı ve dil dosyalarını sunar, akışı ve komut yolunu açar (`ekran`'ı aktarır, `ekran.tr` onu aktarmaz); testler modelsizdir (`ekran_test.tr` katlama, komutlar, sunulan dosyalar; `ekran_tur_test.tr` tur ve cevap metni). İki testin ortak yardımcıları `screen_helpers.tr`'de.
- `behavior_chain_test.go`: zincirin varsayılan yolu ve aygıtı olmadığını, dosyalarının burada anıldığını sınar.
- `yuva.tr`, `yuva_test.tr`: ekranlı kipin model yuvası (aşağıda, "Ekranlı kip"): Dost klasörü, kurulu takım, yükleme işareti, yükleme; test modelsizdir.
- `esleme.tr`, `esleme_test.tr`: ekranın eşleme kodu (dil tanımı 28.2b): `GET /pair` tek kullanımlık, 120 saniyelik kodla belirteci verir, `GET /pair/renew` yeni kod üretip tarayıcıyı açar; ikisi de üstbilgi ister, yanlış kodlar sınırlanır (60 saniyede 10), günlük satırları kodludur (H8175, H8176); kararı ana döngü verir. Test modelsizdir: sahte saat ve sahte açıcı.
- `oturum.tr`: ekranlı kipin oturumu: ana döngünün durumu ve işleri (yuvaya yükleme, dil değişimi, komutlar, mikrofon, dinleme); `asistan.tr` açılışı yapar ve oturumu kurar. Ayrı TR testi yok: `asistan.tr`'nin ekranlı kipini sınayan Go testleri (`behavior_model_load_test.go`, `behavior_token_file_test.go`) onu koşturur.
- `katalog.json`: Dost'un indirebileceği modellerin ve konuşma dillerinin kataloğu (aşağıda, "Kurulum").
- `katalog.tr`, `katalog_test.tr`: katalogu okur, katalogu ve ayarı denetler (`ayar_denetle`), makineye uygunluğu hesaplar; test modelsizdir ve `ayarlar.tr`'yi de sınar.
- `ayarlar.tr`: ayar dosyasının yolu, okunması (denetimi `katalog.tr`'nin), yazılması ve kurulumun işi (`kur`); `katalog.tr`'yi aktarır, `katalog.tr` onu aktarmaz.
- `kur.tr`: bu makineye hangi dil modelinin sığdığını ve önerileni yazan komut (aşağıda, "Kurulum"); model yüklemez.
- `behavior_setup_test.go`: kurulum dosyalarının burada anıldığını sınar.

## Zincir

`asistan.tr` dinler, yazıya çevirir, cevap üretir ve konuşur: VAD kesiti (`tr/vad`), Whisper (`tr/whisper`, `TR_language`: `--konuşma_dili`, öntanımlı `tr`; başka dilde `--ses_modeli` ve `--sistem` de o dilde verilir, örneğin `en` ile `mms-tts-eng`; `--sistem` verilmezse program açılışta hata verir, çünkü öntanımlı ileti Türkçe cevap ister), Qwen3 biçimi (`tr/chat`, düşünmeden), GGUF modeliyle açgözlü üretim (`tr/pretrained`), cevabın ayrılması, cümle cümle ses (`tr/vits`). `--ekran <kapı>` ile ekrana bağlanır (aşağıda, "Ekranlı kip"). Yarım çift yönlüdür: mikrofon açık kalır, ama çalınırken gelen ses atılır (aşağıda, "Dinleme").

- **Kipler:** `--kip dosya` bir klasördeki WAV'ları sırayla söz olarak okur ve cevapları WAV olarak yazar; `--kip canlı` mikrofonu (`--aygıt`, dshow adı) ya da `--sahte_mikrofon`'u (bir WAV'ı `ffmpeg -re` ile gerçek zaman hızında akıtır; yeniden başlarken ilk başlatmadan bu yana geçen süreden açar) dinler ve `--çalıcı` ile çalar. `--ffmpeg`'in ve `--çalıcı`'nın programı PATH'teki ad (`ffmpeg`) ya da dosyanın yoludur: `/` ya da `\` içeren değer yol sayılır. `--çalıcı` boşluklardan bölündüğü için onun yolunda boşluk olamaz.
- **Varsayılan yol yoktur:** `--dil_modeli`, `--whisper`, `--vad`, `--ses_modeli` ve kipin girdileri verilmezse program hata verir. Seçeneklerin hepsi `--help`'tedir.
- **Önbellek** turdan tura korunur: yeni dizi önbellektekiyle ortak önek kadar karşılaştırılır, önbellek oraya kırpılır, yalnız kalan okunur. Önek önceki asistan başlığının sonundan (konuşmanın ilk turunda önceden okunan sistem iletisinden) kısa çıkarsa `ÖNEK_KISA` yazılır. Sistem iletisi programın açılışında ve tur hatasından sonra, ilk sözden önce önbelleğe okunur (`ısıt`; `OLCU_ISITMA` süresini ve parça sayısını yazar): ilk tur yalnız kullanıcının iletisini okur. Isıtma düşerse program sürer, ilk tur sistem iletisini de okur. Önbellek dolup yeni konuşma başlayınca ısıtılmaz: soru zaten o turdadır. Önbellek `--bağlam` parça kadar ayrılır, modelin bütün bağlamı kadar değil (Qwen3-4B'de 40960 parça 11 520 MiB olurdu). Dolacaksa program bunu söyler ve sistem iletisiyle yeni konuşma başlatır. `--önbelleksiz` her turu baştan okur: önbelleğin sınaması.
- **Cümle cümle:** cevabın cümleleri üretim sürerken, sınırları geldikçe sese çevrilir ve çalınır (`CümleAkışı`, `Konuşma`; `zincir.tr`). Sınır kuralı `cümleler`'inkidir; sınırla bitmeyen son parça cevap bitince söylenir. Dil modeli ve ses aynı görevde sırayla hesaplanır; çalma ayrı bir görevde ve dış süreçtedir, her çalma öncekini bekler. Söylenen cümleler sonunda `TR_qwen3_reply`'ın içeriğinin cümleleriyle karşılaştırılır; ayrılırsa (cevapta düşünce ya da araç çağrısı etiketi) kalan söylenmez, `SÖYLENEN FARKLI` yazılır.
- **Ölçü:** her tur bir `OLCU` satırıdır, yalnız sayı: VAD kesitinin geldiği andan yazıya, ilk parçaya, ilk cümlenin üretiminin bitmesine (sınırı geldi ya da cevap bitti), cevabın bitmesine, ilk cümlenin sesine ve çalmanın başlamasına (`toplam_ms`; dosya kipinde cümlelerin çalınmaya verildiği an) kadar ms; kullanıcının beklediği süre (`bekleme_ms`: kesitleyicinin sessizliği, 608 ms, artı `toplam_ms`); kesit tampondan geç okunduysa geç kalınan süre (`geride_ms`; tur sürerken konuşulan söz); çalmanın süresi; kesitin çalma aralığından sonraki ilk çerçevede başlayıp başlamadığı (`calma_sonrasi`: yankı ya da çalma sırasında başlamış söz); söylenenin içerikten ayrılıp ayrılmadığı (`soylenen_fark`); içerikte rakam olup olmadığı; ortak önek ve yeniden okunan parça. `OLCU_MIKROFON` mikrofonun açılıştan ilk çerçeveye gecikmesidir; `OLCU_ATILAN` çalma aralığından sonra kesitleyicinin yarım bir sözü atıp atmadığıdır. Bütün damgalar tek saatten alınır (`şimdi`, `zincir.tr`): 13.1'in tekdüze saati (`TR_monotonic_ns`), sistem saati düzeltilince kaymaz.
- **Dinleme** (`dinle`, `zincir.tr`): mikrofon bir kez açılır ve tur boyunca açık kalır; tur sürerken gelen ses mikrofonun tamponunda birikir, tur bitince okunur. Tampon 7 dakikadır (turun ölçülmüş üst sınırının iki katı, hesap `asistan.tr`'de). Her turdan hemen önce çerçeveler saate çapalanır: okumanın anı çerçevenin bitişinden önce olamaz, okumaların en küçük farkı çapadır; tampondan geç okunan çerçeveler çapayı değiştirmez. Çalma aralığına (ilk çalmanın başından son çalmanın sonuna, artı yankı kuyruğu; kuyruk dizüstünün hoparlörü ve mikrofonuyla ölçüldü: 0 ms, ölçüsü `asistan.tr`'de) düşen çerçeveler kesitleyiciye verilmez; aralıktan sonraki ilk çerçevede kesitleyici ve dedektör sıfırlanır. Böylece:
  - çalma başlarken kullanıcı konuşuyorsa sözün yarısı atılır (söz kesme yok);
  - çalmadan önce, cevap hesaplanırken konuşulan söz kaybolmaz, sonraki turun sözü olur;
  - tampon taşarsa (tur tamponu aştı) hata okumada gelir, mikrofon yeniden açılır, program sürer.
- **Ekranlı kip** (`--kip canlı --ekran <kapı> --belirteç_dosyası <yol>`; `--kip dosya` ile hata): belirteç bellekte üretilir, sunucu onunla `127.0.0.1:<kapı>`'da açılır, kapı tutulduktan sonra belirteç dosyaya yalnız sahibine yazılır (`belirteçle_aç`; kapı doluysa açılış H7625 ile durur, dosyaya dokunulmaz), standart çıktıya ekranın adresi ve dosyanın yolu yazılır; belirtecin kendisi hiçbir yere yazılmaz. Modeller sonra yuvaya yüklenir (`yuva.tr`): `--ayar`'ın klasörü (öntanımlı `%LOCALAPPDATA%\Dost\`) Dost klasörüdür, modeller `modeller/<kimlik>/` altındadır; katalogun (`--katalog`) her dosyası boyuyla duran girdi kuruludur. Ayar dosyası ya da kurulu takım yoksa ekran `setup` görür ve `model_load`'u bekler; yükleme sürerken `loading` (`model` ile), metin turu `not_ready` alır. Yükleme başlarken Dost klasörüne `yukleme.json` yazılır, bitince ya da hatası yakalanınca silinir; açılışta kalmışsa önceki yükleme yarıda kalmıştır, görüntüde `model_load_failed` olur ve silinir. `--dil_modeli` verilirse katalog ve ayar okunmaz, komut satırının yolları yüklenir. Konuşma dili ve sistem iletisi `settings_set` ile değişir (`speech_language`, `system_prompt`): kullanıcının iletisi başka dil içinse `setting_invalid` (`system_prompt_language`), dilin yazıcısı ya da sesi kurulu değilse `not_installed`, dil katalogda yoksa `unknown_language`; model yüklüyse yeni dilin modelleri eskiler yerinde dururken yüklenir, düşerse hiçbir şey değişmez (`model_load_failed`), yüklenirse sistem iletisi yenilenir, önbellek ve geçmiş sıfırlanır. Değişiklik ayar dosyasına yazılmaz; `settings` olayı yeni değeri taşır. Mikrofon kapalı başlar; ekranın `mic_set` komutuyla açılır, `mic: true` ilk çerçeve okununca gider. Tur bir görevde koşar (`tur_yürüt`, `ekran.tr`): komutlar tur sürerken de 20 ms'de bir okunur, `stop` görevi iptal eder; `turn_end`, `metrics`, geri alma ve `OLCU` satırı bekleyen tarafta, görevin sonucuna bağlı yazılır. Ekrana yalnız `TR_qwen3_reply`'ın içeriği gider: her adımda baştan çözülen metnin öncekinin uzantısı olan kısmı maske akışından (19.4a) geçer; yarım karakter ve etiketin olası başı tutulur; turun sonunda ekrandaki metin içerikten ayrıysa `replace` içeriği yazar. Metin turunun çalma aralığı sesli turunki gibi atılır (`Dinleme`'nin `ara`'sı); durdurulan turun aralığı durdurma anına uzar, kesilen çalmanın yankısı tur açmaz. Ekransız canlı kip ve dosya kipi değişmez.
- **Hata** turu bitirir: ileti ekrana yazılır, kullanıcı iletisi geçmişten çıkar, program sonraki söze geçer. Son çalma başarıda da hatada da beklenir; çalmanın hatası (çalıcı yok) da tur hatasıdır. Geçmiş geri alınsa da kullanıcı cevabın bir kısmını duymuş olabilir: ilk sürümün kabulüdür. Yazısı boş kesit tur sayılmaz; `--en_çok_tur` cevaplanan ve hatayla biten turları sayar.
- **Ses diskte kalmaz:** canlı kipte cevabın her cümlesi geçici bir WAV'a yazılır, çalınır ve hemen silinir (çalma hatayla bitse de); geçici klasör son çalma beklendikten sonra silinir. Kullanıcının sözü yalnız `--deneme_klasörü` verilince diske yazılır (aşağıda).
- **Deneme kaydı** (`--deneme_klasörü <tam yol>`; verilmezse kayıt yok, çıktı aynı; `Deneme`, `deneme.tr`): oturumun ilk satırı ve her tur `turlar.jsonl`'e bir satırdır: kayıt ve tur numarası, saat (TSS), kaynak (ses ya da metin), son (`tamam`, `durduruldu`, `hata`, `bos`), kesitin başı, sonu ve uzunluğu (oturumun başından ms; bitiş sessizliği kesitin içindedir), kesitin WAV'ı (`tur-<kayıt>.wav`), Whisper'ın metni, dil modeline giden ileti, cevabın içeriği ve düşünce alanı (zincir düşünmeden kurulu: boş), sese çevrilen cümleler, `OLCU`'nun süreleri ve token/sn. `ozet.md` her turdan sonra yeniden yazılır (program kesilse de kalır): tur başına bir satır, değerlendirme için boş sütunlar, sonda türe göre ortancalar ve en büyük kalem. Metin turunda VAD yoktur, `bekleme_ms` 608 ms fazla çıkar. Başlığa seçeneklerden yalnız model adları, kip, ekran ve tur sınırı girer; belirteç ve dosyası girmez. Klasör açılmış tam yol olmalıdır (TR ortam değişkenini açmaz: PowerShell'de `"$env:USERPROFILE\TR_deneme\<ad>"`); göreli, `%` ya da `$` içeren, bir git deposunun ya da çalışma ağacının (yolun ya da üst klasörlerinden birinin içinde `.git`) ya da OneDrive, Google Drive (`My Drive`, `Drive'ım`), Dropbox veya iCloud Drive'ın içindeki yol reddedilir: kayıt kullanıcının sesidir. İçinde önceki bir kayıt (`turlar.jsonl`) olan klasör de reddedilir, ikinci açılış ilk kaydı silmez. Durdurulan turda ilk parçanın anı ve o ana kadar üretilen metin kayda girer (`cevap_ms` -1: cevap bitmedi); hatalı turda hatanın kodu (`hata_kodu`) da. Kaydın kendi yazma hatası turu bozmaz: `DENEME HATASI` yazılır, program sürer.
- **Kurulum** (`katalog.tr`, `ayarlar.tr`, `kur.tr`): `katalog.json` Dost'un indirebileceği modelleri ve konuşma dillerini sayar: her girdinin deposu, commit karmasıyla sabit revizyonu, dosyalarının boyu ve sha256'sı, lisansının adı, ölçülmüş belleği. Katalog ve ayar dosyası JSON'dur, ikisi de `"v": 1` taşır; bilinmeyen alan, eksik alan, yanlış tür ve bilinmeyen sürüm açık hatadır (H8160–H8174). `trlang tools/assistant/kur.tr --ayar ayar.json --toplam_bellek 17179869184` her dil modelinin bu makineye sığıp sığmadığını ve önerileni yazar; model yüklemez. Karar toplam bellekle verilir, boş bellekle değil; toplam bellek sistem bilgisi dile gelene kadar `--toplam_bellek` ile verilir. `--ayarı_yaz` ek olarak öneriyi ayar dosyasına yazar (geçici adla, sonra taşıyarak). Ayarın öntanımlı yeri `%LOCALAPPDATA%\Dost\ayar.json`'dur; `LOCALAPPDATA` yoksa `--ayar` zorunludur.

```bash
trlang tools/assistant/asistan.tr --kip dosya --girdi sorular --çıktı cevaplar --dil_modeli Qwen3-4B-Q4_K_M.gguf \
    --whisper whisper-small --vad silero_vad_16k.safetensors --ses_modeli mms-tts-tur
```

`zincir_test.tr` gerçek modelsiz parçaları sınar: cümlelere bölme, üretilen kimliklerden cümlelerin sınır geldikçe çıkması (cümle akışı), rakam, önbelleğin kapasitesi ve ortak öneki, sistem iletisinin önceden okunması (kimlikler önbelleksiz okumayla aynı; hatası programı durdurmaz), cümlelerin sırayla çalınması ve çalınan sesin silinmesi, dinleme (çalma aralığının atılması, çapa, taşma; zaman damgalı sahte mikrofonla), `ara` kancası (metin turunun aralığı, mikrofonun yeniden açılması, durma), durdurulan turun geri alınması.

## Ekranın sunucusu

`ekran.tr` ve `ekran_sunucu.tr` sözleşmenin (`contract.md`) TR yanıdır.

- **Sunucu:** `127.0.0.1`'de, belirteçli (28.2). Sayfa (`/`) ve sayfanın listelediği dil dosyaları (`/lang/<kod>.js`) `TR_tokenless_paths:`'tadır: tarayıcı adres çubuğundan açtığı sayfaya `Authorization` ekleyemez. `/events` ve `/command` belirteç ister. Başka her yol `404`. Sayfa `Content-Security-Policy: frame-ancestors 'none'; connect-src 'self'` ile, sayfa ve dil dosyaları `Cache-Control: no-store` ile gider; akışın `no-store`'unu yayıncı yazar (28.7).
- **Görüntü yalnız olaylardan katlanır** (`katla`, 28.7'nin `TR_reduce:`'u): son `state` başta, son `metrics` sonda, bir konuşma parçası tek `turn_text` (`replace`), kapanan onay düşer, `error` girmez. Yeni tur açılırken görüntü 512 KiB'ı aşıyorsa eski turlar düşer, açık onay düşmez: görüntü olayın 1 MiB sınırının altında kalır.
- **Komutun kararı ana döngüdedir** (`komutları_işle`). İşleyici biçimi denetler (`415`, bozuk JSON, sürüm, bilinmeyen komut, eksik alan), komutu tek bir `TR_append` ile bırakır ve cevabı bekler; `turn_busy`, `not_ready`, `text_too_long` ve `command_id` tekrarı ana döngüde verilir. İşleyicide "oku, sonra yaz" iki deyim olurdu ve 10.2'nin devir noktası arada iki `send_text`'i birden kabul ettirirdi. Son 1024 `command_id`'nin cevabı tutulur.

`ekran_test.tr` modelsiz sınar: katlama (rastgele olay dizisinde görüntüyü işleyen ekran ile olayların hepsini işleyen ekran aynı durumu görür), olayın ve görüntünün 1 MiB sınırı, aynı anda iki `send_text`, aynı `command_id`'nin iki kez gelmesi, komutların kodları ve biçimi, sunulan dosyalar. `ekran_tur_test.tr` turu ve cevap metnini sınar: ekrana giden metin, turun tek sonu, bitiş, belirteç dosyası, ekranın adresi, mikrofonlu çerçevede metin turu.

## Açmak

`index.html`'i tarayıcıda açın. Sahte akış kendi başlar. Üstteki **Sahte sahne** seçicisi sözleşmenin bütün durumlarını oynatır: onay, süre dolması, bağlantı kopması, sunucunun yeniden başlaması, uzun çıktı, kötü niyetli metin.

Gerçek kipte sayfayı TR sunar ve kaynak sayfanın kendi köküdür: yalnız `http://127.0.0.1:<kapı>/` kabul edilir, `localhost` reddedilir; `file://` yalnız sahte kipte çalışır (`contract.md`, "Kaynak").

Sayfayı TR'nin sunması (proje kökünden; dosya yolları örnektir):

```bash
trlang tools/assistant/asistan.tr --kip canlı --aygıt "<mikrofonun dshow adı>" --ekran 8123 --belirteç_dosyası belirtec.txt \
    --dil_modeli Qwen3-4B-Q4_K_M.gguf --whisper whisper-small --vad silero_vad_16k.safetensors --ses_modeli mms-tts-tur
```

`--tarayıcıyla_eşle` verilince (Windows başlatıcısı verir) Dost tarayıcıyı kendisi açar; ekran belirteci tek kullanımlık bir eşleme koduyla alır, kullanıcı belirteç görmez. Seçenek verilmezse çıktının `EKRAN` satırındaki adres tarayıcıda açılır; ekran belirteci sorar. Belirteç iki yolda da `belirtec.txt`'ye yazılır (dosyayı yalnız sahibi okur): ajan, komut satırı istemcileri ve ekranın kutusu için. Modelsiz denemek için `trlang tools/assistant/screen_demo.tr --ekran 8123 --belirteç_dosyası belirtec.txt`.

## Yapı

- **Tek yol:** sahte akış da gerçek SSE baytları üretir ve aynı ayrıştırıcıdan geçer; çerçeveler bölünmüş gelir ki UTF-8 sınırı da sınansın. Ekran kodu iki kaynağı ayırmaz; yalnız sahne seçicisi sahte kaynağa bakar. İkisinin farkı `fetchTransport` ve `createMockTransport`'tur, ikisi de `open` ve `post` verir.
- **Onay kartı akışın dışındadır:** kendi yuvasında durur, yalnız `approval_open` olayından gelir. Model metni o yuvaya ulaşamaz, en fazla ona benzeyen düz yazı olur. Kart `approval_close` gelmeden kalkmaz. Onayla, kart açıldıktan yarım saniye sonra etkinleşir; varsayılan odak Vazgeç'tedir ama yazı yazılırken odak çalınmaz.
- **Görünmez karakter:** kart ve araç adı yön ve denetim karakterlerini `[U+202E]` gibi yazar.
- **Güvenilmez girdi:** model metni, araç adı ve çıktısı yalnız `textContent` ile basılır.
- **Sınırlar:** dosyanın başındaki sabitler. Akış 400 satırda, bir metin 120 000 karakterde, gösterilen araç çıktısı 24 000 karakterde durur ve bunu söyler.

## Dil eklemek

Ekranda dil ayrıcalığı yok; varsayılan tarayıcının dilidir, eşleşme yoksa listedeki ilk dil, seçim hatırlanır. Dil eklemek yeni dosya ve tek satırdır, ekran kodu değişmez:

1. `lang/en.js`'i `lang/<kod>.js` adıyla kopyalayıp çevirin. Anahtarlar öbür dillerle aynı olmalı, `{ad}` boşlukları da. `language_name` dilin kendi adıdır, `direction` `ltr` ya da `rtl`'dir.
2. `index.html`'in sonundaki dil listesine satırını ekleyin: `<script src="lang/<kod>.js"></script>`.

Dosya tek bir atamadır: `assistantLanguages.<kod> = {...};`, sağı geçerli JSON ve yalnız metin. Yorum da giremez; test dosyayı JSON olarak okur, kod taşıyamaz. Biçim `.json` değil `.js`'dir, çünkü `file://`'de `fetch` çalışmaz, `<script src>` çalışır.

Dil seçicisi kendini yüklenen dillerden kurar. Sayılar, boyutlar ve süreler `Intl` ile dile göre yazılır. Biçim mantıksal özelliklerle yazıldığı için sağdan sola yazılan dil düzeni kendiliğinden çevirir; metin uzunluğu sabit genişliğe bağlı değildir. HTML'in içine yazı yazılmaz, `<title>` dahil.

Dil dosyası CSP özetini değiştirmez: politika onları `/lang/` yolundan tanır.

## Betik politikası

Tek bir meta politika bu işi yapamıyor. `file://`'de `'self'` ve `file:` bütün yerel betikleri geçirir, yolla sınırlamak ve `integrity` ise dil dosyalarını da keser. Bu yüzden iki aşama var:

1. Sayfanın politikası ana betiği özetiyle, dil dosyalarını `/lang/` yolundan ya da `file:`'dan kabul eder.
2. Ekran başlarken `script-src 'none'` taşıyan ikinci bir politika ekler (`sealScripts`). Politikalar birlikte geçerlidir: bundan sonra hiçbir betik yüklenemez.

Güvenilmez metin ancak ikinci aşamadan sonra gelir; o ana kadar yüklenenler dosyadaki yazılı satırlardır.

## Değiştirmek

Betik ya da stil değişince CSP etiketindeki özet de değişmelidir, yoksa tarayıcı sayfayı çalıştırmaz. `TestScreenPolicy` düşerken beklenen özeti yazar: onu etikete kopyalayın. Özet, `<script>` ile `</script>` arasındaki baytların `sha256`'sının base64 yazımıdır.

Ekranda görünen ad yalnız `ASSISTANT_NAME` sabitindedir.

## Sınamak

Testler tarayıcı açmaz; dosyaları okuyup kuralları denetler:

```bash
go test ./tools/assistant
```

Davranış tarayıcıda sahnelerle gezilir: konsolda hata olmamalı, 1280 ve 800 genişlikte düzen bozulmamalı.

## Testler

- `behavior_device_test.go`: zincirin cihaz geçişleri (`zincir.tr`'nin `işlemcide`, `çerçeve_örnekleri`, `kesit_cihazda`): modelsiz bir programı `trlang` ile kart benzeticisinde (`TR_GPU_SIM=1`) koşar; varsayılan cihaz kartken kesitleyicinin örnekleri ve mikrofonun 16 bitlik çerçevesi işlemcide, kesit kartta; işlemcideyken üçü de işlemcide.
- `behavior_model_load_test.go`: `asistan.tr`'nin ekranlı kipi modelsiz açılır: kurulu takım yoksa `setup`, metin turu `not_ready`, katalogda olmayan ya da dil modeli olmayan girdi `unknown_entry`, dosyaları olmayan girdi `not_installed`; yarıda kalan yüklemenin işareti görüntüde `model_load_failed` olur ve silinir, bozuk işaret girdisizdir; yuvanın dosyaları ve oturumun modülü (`oturum.tr`) burada anılır; `--tarayıcıyla_eşle` verilmeden eşleme yolları yoktur (belirteçsiz `401`, belirteçli `404`).
- `behavior_token_file_test.go`: belirteç dosyasının sırası: kapı doluyken `asistan.tr`'nin ekranlı kipi ve `screen_demo.tr` H7625 ile düşer, var olan belirteç dosyasına dokunmaz.
- `behavior_screen_files_test.go`: ekranın iki TR testinin (`ekran_test.tr`, `ekran_tur_test.tr`) ve ortak yardımcılarının (`screen_helpers.tr`) burada anıldığını, iki testin ekranı ve yardımcıları aktardığını, yardımcı modülün test bloğu taşımadığını sınar.
- `behavior_event_id_test.go`: ekranın olay kimliği kuralı (`contract.md`, "Kopma ve yeniden bağlanma"); `index.html`'deki iki saf fonksiyonu (`splitId`, `eventStep`) node ile koşar, node yoksa atlanır: ilk bağlantı, sıranın sayı olarak karşılaştırılması, yeniden başlayan sunucu, `session`'sız gelen başka ön ekli olay, bozuk kimlik.
- `behavior_screen_test.go`: `innerHTML` yok, dış adres yok, CSP var ve özetleri tutuyor, boyut 150 KB altı, görünmez karakter yok; sözleşmedeki her olay, komut ve durum ekranda ve sahte akışta; bütün dil dosyalarında aynı anahtarlar ve boşluklar var, her hata kodunun karşılığı var, dil dosyasında kod yok, dil listesi dosyalarla aynı; eşleme kodu yalnız `#p=` biçimiyle okunur ve ilk işte adresten silinir, önden işlenirken `/pair`'e gidilmez, belirteç yalnız sekmenin oturum belleğinde durur.
