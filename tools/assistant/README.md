# Asistan ekranı

TR'de yazılacak yerel asistanın ekranı ve TR'yle konuşacağı sözleşme. Ekran incedir: tek sayfa ve dil dosyaları, düz JavaScript, derleme yok, dışarıdan hiçbir şey yüklemez. TR'ye HTTP ile bağlanır; pencere çizmek TR'nin işi değildir.

TR'de HTTP sunucusu henüz yok, bu yüzden ekran bugün **sahte olay akışıyla** çalışır ve bunu üstte sürekli yazar. Ekrandaki her şey uydurmadır. Yer geçicidir: asistanın TR kodu gelince taşınır.

## Dosyalar

- `index.html`: ekran. Biçim, kod ve sahte akış bu tek dosyada.
- `lang/<kod>.js`: çeviriler, her dile bir dosya. Yalnız metin taşır.
- `contract.md`: ekranın TR'den beklediği her şey. Asıl değer budur; HTTP bölümü yazılırken tartılır.
- `behavior_screen_test.go`: aşağıdaki kuralları korur.
- `asistan.tr`, `zincir.tr`, `zincir_test.tr`: ekransız sesli zincir (aşağıda, "Zincir").
- `behavior_chain_test.go`: zincirin varsayılan yolu ve aygıtı olmadığını, dosyalarının burada anıldığını sınar.

## Zincir

`asistan.tr` dinler, yazıya çevirir, cevap üretir ve konuşur: VAD kesiti (`tr/vad`), Whisper (`tr/whisper`, `TR_language: "tr"`), Qwen3 biçimi (`tr/chat`, düşünmeden), GGUF modeliyle açgözlü üretim (`tr/pretrained`), cevabın ayrılması, cümle cümle ses (`tr/vits`). Ekrana henüz bağlı değildir. Yarım çift yönlüdür: mikrofon açık kalır, ama çalınırken gelen ses atılır (aşağıda, "Dinleme").

- **Kipler:** `--kip dosya` bir klasördeki WAV'ları sırayla söz olarak okur ve cevapları WAV olarak yazar; `--kip canlı` mikrofonu (`--aygıt`, dshow adı) ya da `--sahte_mikrofon`'u (bir WAV'ı `ffmpeg -re` ile gerçek zaman hızında akıtır; yeniden başlarken ilk başlatmadan bu yana geçen süreden açar) dinler ve `--çalıcı` ile çalar.
- **Varsayılan yol yoktur:** `--dil_modeli`, `--whisper`, `--vad`, `--ses_modeli` ve kipin girdileri verilmezse program hata verir. Seçeneklerin hepsi `--help`'tedir.
- **Önbellek** turdan tura korunur: yeni dizi önbellektekiyle ortak önek kadar karşılaştırılır, önbellek oraya kırpılır, yalnız kalan okunur. Önek önceki asistan başlığının sonundan (konuşmanın ilk turunda önceden okunan sistem iletisinden) kısa çıkarsa `ÖNEK_KISA` yazılır. Sistem iletisi programın açılışında ve tur hatasından sonra, ilk sözden önce önbelleğe okunur (`ısıt`; `OLCU_ISITMA` süresini ve parça sayısını yazar): ilk tur yalnız kullanıcının iletisini okur. Isıtma düşerse program sürer, ilk tur sistem iletisini de okur. Önbellek dolup yeni konuşma başlayınca ısıtılmaz: soru zaten o turdadır. Önbellek `--bağlam` parça kadar ayrılır, modelin bütün bağlamı kadar değil (Qwen3-4B'de 40960 parça 11 520 MiB olurdu). Dolacaksa program bunu söyler ve sistem iletisiyle yeni konuşma başlatır. `--önbelleksiz` her turu baştan okur: önbelleğin sınaması.
- **Cümle cümle:** cevabın cümleleri üretim sürerken, sınırları geldikçe sese çevrilir ve çalınır (`CümleAkışı`, `Konuşma`; `zincir.tr`). Sınır kuralı `cümleler`'inkidir; sınırla bitmeyen son parça cevap bitince söylenir. Dil modeli ve ses aynı görevde sırayla hesaplanır; çalma ayrı bir görevde ve dış süreçtedir, her çalma öncekini bekler. Söylenen cümleler sonunda `TR_qwen3_reply`'ın içeriğinin cümleleriyle karşılaştırılır; ayrılırsa (cevapta düşünce ya da araç çağrısı etiketi) kalan söylenmez, `SÖYLENEN FARKLI` yazılır.
- **Ölçü:** her tur bir `OLCU` satırıdır, yalnız sayı: VAD kesitinin geldiği andan yazıya, ilk parçaya, ilk cümlenin üretiminin bitmesine (sınırı geldi ya da cevap bitti), cevabın bitmesine, ilk cümlenin sesine ve çalmanın başlamasına (`toplam_ms`; dosya kipinde cümlelerin çalınmaya verildiği an) kadar ms; kullanıcının beklediği süre (`bekleme_ms`: kesitleyicinin sessizliği, 608 ms, artı `toplam_ms`); kesit tampondan geç okunduysa geç kalınan süre (`geride_ms`; tur sürerken konuşulan söz); çalmanın süresi; kesitin çalma aralığından sonraki ilk çerçevede başlayıp başlamadığı (`calma_sonrasi`: yankı ya da çalma sırasında başlamış söz); söylenenin içerikten ayrılıp ayrılmadığı (`soylenen_fark`); içerikte rakam olup olmadığı; ortak önek ve yeniden okunan parça. `OLCU_MIKROFON` mikrofonun açılıştan ilk çerçeveye gecikmesidir; `OLCU_ATILAN` çalma aralığından sonra kesitleyicinin yarım bir sözü atıp atmadığıdır. Bütün damgalar tek saatten alınır (`şimdi`, `zincir.tr`): 13.1'in tekdüze saati (`TR_monotonic_ns`), sistem saati düzeltilince kaymaz.
- **Dinleme** (`dinle`, `zincir.tr`): mikrofon bir kez açılır ve tur boyunca açık kalır; tur sürerken gelen ses mikrofonun tamponunda birikir, tur bitince okunur. Tampon 7 dakikadır (turun ölçülmüş üst sınırının iki katı, hesap `asistan.tr`'de). Her turdan hemen önce çerçeveler saate çapalanır: okumanın anı çerçevenin bitişinden önce olamaz, okumaların en küçük farkı çapadır; tampondan geç okunan çerçeveler çapayı değiştirmez. Çalma aralığına (ilk çalmanın başından son çalmanın sonuna, artı yankı kuyruğu; kuyruk dizüstünün hoparlörü ve mikrofonuyla ölçüldü: 0 ms, ölçüsü `asistan.tr`'de) düşen çerçeveler kesitleyiciye verilmez; aralıktan sonraki ilk çerçevede kesitleyici ve dedektör sıfırlanır. Böylece:
  - çalma başlarken kullanıcı konuşuyorsa sözün yarısı atılır (söz kesme yok);
  - çalmadan önce, cevap hesaplanırken konuşulan söz kaybolmaz, sonraki turun sözü olur;
  - tampon taşarsa (tur tamponu aştı) hata okumada gelir, mikrofon yeniden açılır, program sürer.
- **Hata** turu bitirir: ileti ekrana yazılır, kullanıcı iletisi geçmişten çıkar, program sonraki söze geçer. Son çalma başarıda da hatada da beklenir; çalmanın hatası (çalıcı yok) da tur hatasıdır. Geçmiş geri alınsa da kullanıcı cevabın bir kısmını duymuş olabilir: ilk sürümün kabulüdür. Yazısı boş kesit tur sayılmaz; `--en_çok_tur` cevaplanan ve hatayla biten turları sayar.
- **Ses diskte kalmaz:** canlı kipte cevabın her cümlesi geçici bir WAV'a yazılır, çalınır ve hemen silinir (çalma hatayla bitse de); geçici klasör son çalma beklendikten sonra silinir.

```bash
trlang tools/assistant/asistan.tr --kip dosya --girdi sorular --çıktı cevaplar --dil_modeli Qwen3-4B-Q4_K_M.gguf \
    --whisper whisper-small --vad silero_vad_16k.safetensors --ses_modeli mms-tts-tur
```

`zincir_test.tr` gerçek modelsiz parçaları sınar: cümlelere bölme, üretilen kimliklerden cümlelerin sınır geldikçe çıkması (cümle akışı), rakam, önbelleğin kapasitesi ve ortak öneki, sistem iletisinin önceden okunması (kimlikler önbelleksiz okumayla aynı; hatası programı durdurmaz), cümlelerin sırayla çalınması ve çalınan sesin silinmesi, dinleme (çalma aralığının atılması, çapa, taşma; zaman damgalı sahte mikrofonla).

## Açmak

`index.html`'i tarayıcıda açın. Sahte akış kendi başlar. Üstteki **Sahte sahne** seçicisi sözleşmenin bütün durumlarını oynatır: onay, süre dolması, bağlantı kopması, sunucunun yeniden başlaması, uzun çıktı, kötü niyetli metin.

Gerçek kaynağa bağlanmak için `?source=http://127.0.0.1:<kapı>` verilir. Yalnız bu biçim ve sayfayı sunan kapı kabul edilir, sayfayı da TR sunmalıdır; `file://` yalnız sahte kipte çalışır (`contract.md`, "Kaynak").

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

- `behavior_screen_test.go`: `innerHTML` yok, dış adres yok, CSP var ve özetleri tutuyor, boyut 150 KB altı, görünmez karakter yok; sözleşmedeki her olay, komut ve durum ekranda ve sahte akışta; bütün dil dosyalarında aynı anahtarlar ve boşluklar var, her hata kodunun karşılığı var, dil dosyasında kod yok, dil listesi dosyalarla aynı.
