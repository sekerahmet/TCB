# 19. Süreç ve kayıt

Bu bölümün kurallarına göre bugünkü durum ve yapılacaklar: [`docs/status.md`](../status.md), "19. Süreç ve kayıt".

Her kuralın emsali ve gerekçesi: [`rationale/19_process.md`](rationale/19_process.md).

Bir eğitim, ölçme ya da hizmet programının dış dünyayla dört bağı vardır: ortam değişkenleri, komut satırı seçenekleri, eğitim günlüğü ve ilerleme çubuğu. Bu bölüm onları ve gizli değerlerin çıktıda nasıl göründüğünü söyler. Kullanım: [`docs/guide/04_yapay_zeka_ve_veri.md`](../guide/04_yapay_zeka_ve_veri.md) §4.F.

**19.1** `TR_env(ad)` ortam değişkeninin değerini `TR_String` olarak, tanımsızsa `TR_none` verir; varsayılan `??` ile yazılır: `TR_env("VERI_KLASORU") ?? "veri"`.
- Ad boş olmayan bir `TR_String` olmalıdır ve `=` içeremez.
- Bütün ortamı listeleyen ya da ortamı değiştiren bir yerleşik yoktur: ortamın tamamı gizli değerleri de taşır, yanlışlıkla dökülmez.
- *Test:* TR testi.

**19.2** `TR_command_line(varsayılanlar, TR_description: "", TR_descriptions: {})` programın seçeneklerini bir **yapıyla** tanımlar (13.10): `varsayılanlar` bir yapının değeridir, alanları seçeneklerdir. Komut satırında verilen argümanlarla doldurulmuş aynı tipte **yeni** bir değer döndürür; verilen değer değişmez.
- Argüman verilmemişse (`TR_cmd_test` ve kabuk dahil) varsayılanlar döner.
- Programda bir kez çağrılır; ikinci çağrı hatadır. Sonuç bir değişkende tutulur.
- Seçeneklerin tipi alanların tipidir; `ayar.adım` tipiyle okunur, yanlış alan adı denetim hatasıdır.

```tr
# TR 4
TR_struct Ayar {
    adım: TR_Int = 5000
    oran: TR_Float = 0.001
    veri: TR_String = "veri/eğitim"
    hızlı: TR_Bool = TR_false
}
TR_var ayar = TR_command_line(Ayar(), TR_descriptions: {"oran": "öğrenme oranı"})
TR_print(ayar.adım)
```

- *Test:* TR testi; yapıyla yazım uygulanınca TR testi.

**19.3** Seçeneğin adı alanın adıdır ve yazıldığı gibidir: `ağ_genişliği` alanı `--ağ_genişliği` seçeneğidir. Kullanıcının adı bir koda çevrilmez, başka bir yazımı yoktur (17.1).
- Alanın tipi `TR_Int`, `TR_Float`, `TR_String`, `TR_Bool`, bunların `TR_List`'i ya da bunlardan biri isteğe bağlı (`T?`, varsayılanı `TR_none`) olmalıdır; başka tip kurulumda hatadır.
- `help` ve `h` adlı alan, boş, `-` ile başlayan, boşluk ya da `=` içeren ad kurulumda hatadır. `TR_descriptions:`'daki her ad bir alan olmalı, değeri `TR_String` olmalıdır.
- Kurulum hataları sıradan çalışma hatalarıdır (`TR_try`/`TR_catch` yakalar) ve çağrıyı tüketmez.
- *Test:* TR testi; Go: `TestOptionNameIsTheTextKey`, `TestCommandLineSetupErrors`.

**19.4** Adının bir **sözcüğü** gizli bir sözcük olan ortam değişkeninin ve komut satırı seçeneğinin değeri gizlidir.
- Ad sözcüklere `_`, `-`, `.`, boşluk gibi ayırıcılardan ve küçükten büyüğe harf geçişlerinden bölünür (`apiKey` → `api Key`). Sözcükler Türkçe harfler ASCII karşılıklarıyla, harf büyüklüğüne bakmadan, **tam** karşılaştırılır.
- Gizli sözcükler: `TOKEN`, `KEY`, `SECRET`, `PASSWORD`, `PASSWD`, `PASS`, `PASSPHRASE`, `AUTH`, `CREDENTIAL(S)`, `PRIVATE`, `GİZLİ`, `ŞİFRE(Sİ)`, `PAROLA(SI)`, `ANAHTAR(I)` ve bitişik `APIKEY`, `ACCESSKEY`, `SECRETKEY`, `PRIVATEKEY`, `AUTHTOKEN`, `ACCESSTOKEN`, `APITOKEN`. Kullanıcının adları her dilde olabildiği için liste gösterim dillerinin (17.7) sözcüklerini taşır. `HF_TOKEN`, `OPENAI_API_KEY`, `apiKey`, `db_şifresi` gizlidir; `TOKENIZER_PATH`, `max_tokens`, `KEYBOARD_LAYOUT`, `PASSAGE_DIR` değildir.
- Yalnız sayı ya da mantık değeri olan değer (`--max_tokens 131072`) gizli sayılmaz: maskelemek bütün sayı çıktılarını bozardı.
- En az 6 karakterli gizli değer **insanın göreceği çıktılarda** `***` olarak görünür: `TR_print`'in çıktısı, hata iletileri ve kod çerçevesi, uyarılar, ilerleme çubuğu, günlük dosyası (başlangıç kaydındaki ayarlar dahil), iş durumu dosyası (19.11), yardım çıktısı, hata ayıklayıcının değişken görünümü.
- **Veri değişmez:** değişkenin değeri, `TR_write_file`, `TR_json_string` gerçek değeri taşır.
- Bu, sızıntıyı azaltır ama güvence değildir: değer dönüştürülünce (dilim, ters çevirme) eşleşme kaybolur. Gizli değer argümanla değil ortam değişkeniyle verilir.
- *Test:* TR testi; Go: `TestIsSecretName`, `TestSplitWords`, `TestCommandLineSecretValue`, `TestCommandLineSecretLeaks`.

**19.5** `TR_Logger(yol)` `.jsonl` uzantılı bir eğitim günlüğü açar: her satır tek bir JSON nesnesidir.
- Yol 18.16'nın değiştiren işler kuralıyla çözülür. Klasörü yoksa oluşturulur; dosya varsa **sonuna eklenir**: yeniden başlatılan eğitim aynı günlüğe yazar.
- Her açılışın ilk satırı bir başlangıç kaydıdır: `{"kind":"start","time":...,"version":2,"program":"egit.tr","total":5000,"settings":{...}}`. `total` bağlı bir çubuğun toplamı, `settings` programda `TR_command_line` çağrıldıysa onun sonucudur.
- `g.TR_append(kayıt)` bir ölçü kaydı yazar, değer döndürmez (13.10). `time` anahtarı verilmediyse eklenir; tek öğeli tensör sayı, daha büyüğü iç içe liste olur. `g.TR_append(metin)` `{"time":...,"message":...}` yazar.
- NaN ve sonsuz değerler gizlenmez; `"NaN"`, `"Infinity"`, `"-Infinity"` metni olarak yazılır.
- Her kayıt tek bir yazma çağrısıyla eklenir: süreç çökse de en çok son satır yarım kalır, sonraki açılış yeni kaydı yeni satırda başlatır.
- Sistemin yazdığı anahtarlar ve kayıt türleri (`kind`: `start`, `end`, `resume`; `time`, `version`, `program`, `total`, `settings`, `step`, `rate`, `seconds`, `message`) dosya biçiminin parçasıdır ve çevrilmez (17.3). Kaydın kendi anahtarları programındır ve yazıldığı gibi yazılır.
- Günlük program bitince (hatayla ya da durdurmayla da) kapanır.
- *Test:* TR testi; Go: `TestLogRecords`, `TestLogAppendAndResume`, `TestLogFormatNamesAreLanguageIndependent`.

**19.6** Günlük hataları: `.jsonl` olmayan uzantı; `TR_append`'de "kind" anahtarı (kayıt türlerine ayrılmıştır); sayı, metin, mantık, liste, sözlük ya da tensör olmayan değer; sözlük ya da metin olmayan argüman. Dosya hataları dosyanın adıyla ve nedeniyle bildirilir.
- **Okuyucular için:** `kind` taşımayan satırlar ölçü kayıtlarıdır. Kontrol noktasından sürdürülen eğitim `{"kind":"resume","step":1500}` yazar. Bir `resume` kaydından **önce** gelen ve adımı ondan **büyük** olan ölçü kayıtları geçersizdir: kesilmeden önce yazılmışlardır ve adımları yeniden hesaplanmıştır. Adımı devamın adımına eşit olan kayıt geçerlidir (20).
- *Test:* TR testi; Go: `TestLogErrors`.

**19.7** `TR_Progress(n)` n adımlı bir ilerleme çubuğu açar; `n` pozitif bir `TR_Int`'tir, verilmezse ya da `TR_none` ise çubuk toplamsız bir sayaçtır.
- `ç.TR_update(ölçüler)` bir adım ilerler, verilen ölçüleri (sözlük) gösterir ve değer döndürmez (13.10); ölçüsüz çağrı önceki ölçüleri korur.
- `TR_step: n` tamamlanan adım sayısını n'ye getirir. İlk güncellemede ileri ya da sonra geri atlama sürdürmedir; hız atlamadan sonra yeniden ölçülür. Sonraki ileri atlamalar (her 10 adımda bir güncelleme) hızı bozmaz; aynı adımın ikinci güncellemesi (doğrulama kaybı gibi) hızı değiştirmez.
- Toplama ulaşınca çubuk biter; biten çubuk güncellenemez. `ç.TR_stop()` çubuğu bitirir: ilk çağrıda `TR_true`, sonra `TR_false` verir (18.15 gibi). Program bitince bitmemiş çubuklar da biter.
- Her `TR_update` eğitimin bir **adım sınırıdır**: canlı profil adım başına süreyi buradan ölçer.
- *Test:* TR testi; Go: `TestProgressStepAndErrors`, `TestProgressRateWithStepGaps`, `TestStepHook`.

**19.8** İlerleme hataları: pozitif olmayan ya da `TR_Int` olmayan adım sayısı, `TR_Logger` olmayan `TR_log:`, adım sayısından büyük ya da negatif `TR_step:`, ölçülerde ayrılmış adlar ("step", "rate", "time", "kind": bunları çubuk kendisi yazar).
- *Test:* TR testi.

**19.9** `TR_Progress(n, TR_log: g)` her güncellemeyi günlüğe bir ölçü kaydı olarak, bitişi de bir bitiş kaydı olarak yazar.
- Ölçü kaydı: `{"time":...,"step":n, ölçüler..., "rate":adım/sn}`; `rate` ilk ölçümden sonra.
- Bitiş kaydı: `{"kind":"end","time":...,"step":n,"seconds":s}`.
- *Test:* TR testi; Go: `TestProgressPlainLinesAndLog`.

**19.10** Komut satırında yazım. `TR_cmd_run`, `TR_cmd_debug`, `TR_cmd_build`'in ürettiği uygulama ve düzenleyicinin `"args"` ayarı aynı yoldan geçer (1.4).
- `--ad değer` ya da `--ad=değer`. `TR_Bool` seçenek değersiz yazılınca `TR_true` olur (`--hızlı`); `TR_false` yalnız `--hızlı=false` ile.
- **Değerlerin tek yazımı vardır** (17.9): mantık `true`, `false`; ondalık `inf`, `-inf`, `nan`; sayılar TR'nin yazımıyla (`5000`, `5_000`, `1e-3`; `TR_Float` seçeneğe `3` verilirse `3.0`). Virgüllü ondalık (`0,001`) ve `TR_Int` seçeneğe noktalı sayı hatadır. Liste değeri virgülle ayrılır (`--katmanlar 128,64`; boş değer boş liste).
- `--help` ya da `-h` (başka argümanlar hatalı olsa da) seçenekleri tipleri, açıklamaları ve varsayılanlarıyla standart çıktıya yazar ve programı **0** koduyla bitirir. Yardımın sözcükleri kişinin dilindedir (17.7), varsayılanları ise komut satırının yazımıyla: gösterilen her varsayılan geri verilince aynı değeri verir.
- Bilinmeyen seçenek, iki kez verilen seçenek, değeri eksik ya da tipine uymayan seçenek, tek tireli seçenek (`-h` dışında), konumlu argüman ve 1 MB'tan uzun argüman kullanım hatasıdır: standart hataya hata ve yardımın nasıl alınacağı yazılır, program **2** koduyla biter. İleti en yakın seçeneği önerir; başka yazımlar (`--ogrenme_orani`, `--öğrenme-oranı`) kabul edilmez, doğru ad önerilir.
- `--ad değer` biçiminde sonraki argüman `-` ile başlasa da değerdir (`--oran -0.5`); `--` ile başlayan argüman değer sayılmaz.
- Yardım ve kullanım hatası birer program çıkışıdır: `TR_defer` blokları çalışır, `TR_try`/`TR_catch` onları yakalamaz (1.5, 8.3).
- *Test:* Go: `TestCommandLineValues`, `TestCommandLineUsageErrors`, `TestCommandLineHelp`, `TestCommandLineHelpShowsHowToTurnOff`, `TestHelpDefaultsReadBack`, `TestExitRequestNotCaught`, `TestNumberParsing`, `TestParseErrorCodes`; tek yazım uygulanınca Go testi.

**19.11** İlerleme çubuğunun görünümü. Çubuk **standart hataya** yazılır: programın çıktısı bir dosyaya yönlendirilince çubuk ona karışmaz.

```text
adım 1200/5000 ▕████▌░░░░░░░░░░░▏ %24 · kayıp 2,31 · öğrenme_oranı 3e-4 · 85,2 adım/sn · kalan 14 dk
adım 5000/5000 · kayıp 1,84 · 61 dk 3 sn sürdü
```

- Çubuk bir görünümdür ve kişinin dilindedir (`TR_LANG`, 17.7): sözcükler o dilde, ölçüler 3 anlamlı basamakla ve o dilin ondalık ayırıcısıyla. 0,001'den küçük ve 100 000'den büyük sayılar bilimsel yazılır; `TR_Int` olduğu gibi, uzun metin kısaltılarak yazılır. Hız saniyede birin altındaysa saniye başına adım yerine adım başına saniye gösterilir.
- Hız yaklaşık son 30 saniyenin üstel ortalamasıdır; kalan süre ondan hesaplanır. Toplamsız sayaçta geçen süre gösterilir.
- Standart hata bir terminalse çubuk tek satırda, en çok saniyede 10 kez yeniden çizilir ve terminal genişliğine kırpılır. `TR_print`, uyarılar ve hata iletileri çubuğu silip yazılır, çubuk alttan yeniden çizilir. Bitince satır özet satırına döner. Aynı anda tek çubuk görünür: yenisi açılınca eskisi görünmez olur ama saymayı ve günlüğe yazmayı sürdürür, yenisi bitince yeniden görünür.
- Terminal değilse (dosya, boru, not defteri hücresi, sürekli sınama) en çok 10 saniyede bir düz satır ve sonda özet satırı yazılır.
- Arka plan işinde (`TR_JOB_DIR` ortam değişkeni iş klasörünü gösterir) çubuk terminal kipine geçmez ve iş klasörüne saniyede en çok bir kez `progress.json` yazar (18.13 gibi atomik): `{"step":1200,"total":5000,"metrics":{"loss":2.31},"rate":85.2,"remaining_seconds":840,"time":"...","done":false}`.
- *Test:* Go: `TestProgressTerminal`, `TestFormatNumber`, `TestFormatDurationAndBar`, `TestJobStatus`, `TestNestedProgressRedrawsOuter`, `TestCloseAllFinishesBar`.
