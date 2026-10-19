# 19. Süreç ve kayıt

Bir eğitim betiğinin dış dünyayla dört bağı vardır: ortam değişkenleri, komut satırı seçenekleri, eğitim günlüğü ve ilerleme çubuğu. Kullanım için `docs/guide/04_yapay_zeka_ve_veri.md` §4.F'ye, her adın imzası için [`docs/kodlar.md`](../kodlar.md)'ye bakın. Kural testleri `tests/surec_ve_kayit_test.tr`'dedir; komut satırı yazımı, çıkış kodları ve terminaldeki çubuk `tr test` ile sınanamadığından Go testleriyle sınanır (`internal/stdlib/process`, `internal/stdlib/logging`, `cmd/tr/e2e`).

## Ortam değişkenleri

**19.1** `ortam_değişkeni(ad)` ortam değişkeninin değerini `Metin` olarak, tanımsızsa `yok` verir; varsayılan için `??` yazılır: `ortam_değişkeni("VERİ_KLASÖRÜ") ?? "veri"`. Ad boş olmayan bir `Metin` olmalıdır ve `=` içeremez. Bütün ortamı listeleyen ya da ortamı değiştiren bir ad bilerek yoktur (yanlışlıkla döküm olmasın; TR alt süreç açmaz).

## Komut satırı seçenekleri

**19.2** `komut_satırı(seçenekler, açıklama: "", açıklamalar: {})` betiğin seçeneklerini varsayılanlarıyla tanımlar ve komut satırında verilen argümanlarla doldurulmuş **yeni** bir sözlük döndürür (verilen sözlük değişmez, anahtar sırası korunur). Argüman verilmemişse (ve `tr test`, kabuk) varsayılanlar döner. Programda bir kez çağrılır; ikinci çağrı hatadır (ilk sonucu bir değişkende tutun).

**19.3** Seçenek adı sözlüğün anahtarıdır (veri; kodlanmaz), tipi varsayılandan çıkar: `Tam`, `Ondalık`, `Metin`, `Mantık`, öğeleri aynı tipte bir `Liste` (`Tam` ve `Ondalık` karışırsa `Ondalık`; boş liste `Metin` öğeli) ya da `yok` (verilirse `Metin`). Başka tipte varsayılan, sözlüğün üye adları (`uzunluk`, `içerir`, `anahtarlar` ...: `ayar.uzunluk` seçeneği değil üyeyi verirdi), `yardım` ve `h`, boş, `-` ile başlayan, boşluk ya da `=` içeren adlar kurulumda hatadır; `açıklamalar:` içindeki her ad bir seçenek olmalı, değeri `Metin`. Kurulum hataları sıradan çalışma zamanı hatalarıdır (`dene/yakala` yakalar) ve çağrıyı tüketmez.

**Komut satırında yazım** (Go testleri: `internal/stdlib/process`, `cmd/tr/e2e`). `tr betik.tr`, `tr çalıştır`, `tr hata_ayıkla`, `tr derle` ile üretilen uygulama ve düzenleyicinin başlatma ayarındaki `"args"` aynı yoldan geçer (bkz. §1).
- `--ad değer` ya da `--ad=değer`. `Mantık` seçenek değersiz yazılınca `doğru` olur (`--hızlı`); `yanlış` yalnızca `--hızlı=yanlış` ile.
- Sayılar TR yazımıyla: `5000`, `5_000`, `1e-3`; `Ondalık` seçeneğe `3` verilirse `3.0`. Virgüllü ondalık (`0,001`), `NaN`, sonsuz ve `Tam` seçeneğe noktalı sayı hatadır. Liste değeri virgülle ayrılır (`--katmanlar 128,64`; boş değer boş liste).
- `--yardım` ya da `-h` (başka argümanlar hatalı olsa da) seçenekleri tipleri, açıklamaları ve varsayılanlarıyla standart çıktıya yazar ve programı **0** koduyla bitirir.
- Bilinmeyen seçenek, iki kez verilen seçenek, değeri eksik ya da tipine uymayan seçenek, tek tireli seçenek (`-h` dışında), konumlu argüman ve 1 MB'tan uzun argüman: standart hataya `Hata: ... Seçenekler için: tr betik.tr --yardım` yazılır ve program **2** koduyla biter ("yanlış kullanım"). Mesaj en yakın seçeneği önerir; Türkçe harfsiz (`--ogrenme_orani`) ve tireli (`--öğrenme-oranı`) yazımlar kabul edilmez, doğru ad önerilir (tek yazım). `--ad değer` biçiminde sonraki argüman `-` ile başlasa da değerdir (`--oran -0.5`); `--` ile başlayan bir argüman değer sayılmaz.
- Yardım ve kullanım hatası birer program çıkışıdır: yığın çözülürken `ertele` blokları çalışır, `dene/yakala` onları yakalamaz (kesilme gibi, bkz. §1, 8.3).

## Gizli değerler

**19.4** Adının bir **sözcüğü** gizli bir sözcük olan ortam değişkeninin ve komut satırı seçeneğinin değeri gizli sayılır. Ad sözcüklere `_`, `-`, `.`, boşluk gibi ayırıcılardan ve küçükten büyüğe harf geçişlerinden bölünür (`apiKey` → `api Key`); sözcükler Türkçe harfler ASCII karşılıklarıyla, büyük/küçük harf ayrımı olmadan **tam** karşılaştırılır. Gizli sözcükler: `TOKEN`, `KEY`, `SECRET`, `PASSWORD`, `PASSWD`, `PASS`, `PASSPHRASE`, `AUTH`, `CREDENTIAL(S)`, `PRIVATE`, `GİZLİ`, `ŞİFRE(Sİ)`, `PAROLA(SI)`, `ANAHTAR(I)` ve bitişik yazılan `APIKEY`, `ACCESSKEY`, `SECRETKEY`, `PRIVATEKEY`, `AUTHTOKEN`, `ACCESSTOKEN`, `APITOKEN`. Böylece `HF_TOKEN`, `OPENAI_API_KEY`, `apiKey`, `db_şifresi` gizlidir; `TOKENIZER_PATH`, `max_tokens`, `KEYBOARD_LAYOUT`, `PASSAGE_DIR` değildir. Yalnızca sayı ya da mantık değeri olan değerler (`--max_tokens 131072`) gizli sayılmaz: maskelemek bütün sayı çıktılarını bozardı. Gizli değerler en az 6 karakterse **insanın göreceği çıktılarda** `***` olarak görünür: `yazdır`, hata mesajları ve kod çerçevesi, uyarılar, ilerleme çubuğu, günlük dosyası (başlangıç kaydındaki ayarlar dahil), `ilerleme.json`, `--yardım` çıktısı, hata ayıklayıcının değişken görünümü ve İzleme Masası. **Veri** değişmez: değişkenin değeri, `yazdır`'ın döndürdüğü metin, `dosya_yaz`, `json_metin`. Bu bir sızıntı azaltma önlemidir, güvence değildir (metin dönüştürülünce — dilim, ters çevirme — eşleşme kaybolur); gizliyi argümanla değil ortam değişkeniyle verin.

## Eğitim günlüğü

**19.5** `günlük(yol)` `.jsonl` uzantılı bir eğitim günlüğü açar (tip `Günlük`): her satır tek bir JSON nesnesidir. Klasörü yoksa oluşturulur, dosya varsa **sonuna eklenir** (yeniden başlatılan eğitim aynı günlüğe yazar). Her açılışın ilk satırı bir başlangıç kaydıdır: `{"tür":"başlangıç","zaman":...,"sürüm":1,"program":"egit.tr","toplam":5000,"ayarlar":{...}}` (`toplam` bağlı bir çubuğun toplamı, `ayarlar` programda `komut_satırı` çağrıldıysa onun sonucu). `g.ekle(sözlük)` bir ölçü kaydı yazar ve `doğru` verir: `zaman` anahtarı verilmediyse eklenir (verilirse onun yerine geçer), tek elemanlı tensör sayı olur, daha büyüğü iç içe liste; `g.ekle(metin)` `{"zaman":...,"mesaj":metin}` yazar. Tanımsız (NaN) ve sonsuz değerler gizlenmez, `"NaN"`, `"+Sonsuz"`, `"-Sonsuz"` metni olarak yazılır. Her kayıt tek bir yazma çağrısıyla eklenir: süreç çökse de en çok son satır yarım kalır (sonraki açılış yeni kayıtları yeni satırda başlatır). Kayıt anahtarları dosya biçiminin parçasıdır, yazım dilinden bağımsızdır ve çevrilmez.

**19.6** Günlük hataları: başka uzantı (`.csv` ve `.json` dahil; bugün yalnız JSON satırları), `ekle`'de `tür` anahtarı (kayıt türlerine ayrılmış: `başlangıç`, `bitiş`, `devam`), sayı, metin, mantık, liste, sözlük ya da tensör olmayan değer, Sözlük ya da Metin olmayan argüman. Dosya hataları dosya adıyla ve nedeniyle bildirilir.

Günlük okuyucuları için kurallar: `tür` taşımayan satırlar ölçü kayıtlarıdır. Kontrol noktasından sürdürülen bir eğitim `{"tür":"devam","adım":1500}` kaydı yazar; bir `devam` kaydından **önce** gelen ve adımı ondan **büyük** olan ölçü kayıtları geçersizdir (kesilmeden önce yazılmış, yeniden hesaplanan adımlar silinmez; adımı devamın adımına eşit olan kayıt kaydedilen noktaya dahildir ve geçerlidir, bkz. §20). Günlük dosyası program bitince (hata ya da kesilmeyle bitse de) kapanır; açık bir kapatma adı yoktur.

## İlerleme çubuğu

**19.7** `ilerleme(toplam)` toplam adımlı bir ilerleme çubuğu açar (tip `İlerleme`); `toplam` pozitif bir `Tam`'dır, verilmezse ya da `yok` ise çubuk toplamsız bir sayaçtır. `ç.güncelle(ölçüler)` bir adım ilerler, verilen ölçüleri (sözlük: `{kayıp: k, öğrenme_oranı: o}`) gösterir ve `doğru` verir; ölçüsüz çağrı önceki ölçüleri korur. `adım: n` tamamlanan adım sayısını n'ye getirir: ilk güncellemede ileri ya da sonra geri atlama sürdürmedir (hız atlamadan sonra yeniden ölçülür); sonraki ileri atlamalar (her 10 adımda bir güncelleme) hızı bozmaz, aynı adımın ikinci güncellemesi (ör. doğrulama kaybı) hızı değiştirmez. Toplama ulaşınca çubuk kendiliğinden biter; biten çubuk güncellenemez. `ç.durdur()` çubuğu bitirir, ilk çağrıda `doğru`, sonra `yanlış` verir; program bitince bitmemiş çubuklar da biter. Her `güncelle` eğitimin bir **adım sınırıdır** (canlı profil adım başına süreyi buradan ölçer).

**19.8** İlerleme hataları: pozitif olmayan ya da `Tam` olmayan toplam, `Günlük` olmayan `günlük:`, toplamdan büyük ya da negatif `adım:`, ölçülerde ayrılmış adlar (`adım`, `hız`, `zaman`, `tür`: çubuk kendisi yazar).

**19.9** `ilerleme(toplam, günlük: g)` her güncellemeyi günlüğe ölçü kaydı olarak (`{"zaman":...,"adım":n, ölçüler..., "hız":adım/sn}`; hız ilk ölçümden sonra) ve bitişi `{"tür":"bitiş","zaman":...,"adım":n,"süre_sn":s}` kaydı olarak yazar.

**Görünüm** (Go testleri: `internal/stdlib/logging`). Çubuk **standart hataya** yazılır (`tr egit.tr > çıktı.txt` çubuğu dosyaya karıştırmaz):

```text
adım 1200/5000 ▕████▌░░░░░░░░░░░▏ %24 · kayıp 2,31 · öğrenme_oranı 3e-4 · 85,2 adım/sn · kalan 14 dk
adım 5000/5000 · kayıp 1,84 · 61 dk 3 sn sürdü
```

- Ölçüler 3 anlamlı basamakla, Türkçe ondalık virgülüyle (`0,001`; 0,001'den küçük ve 100 000'den büyük sayılar bilimsel: `3,12e-4`), `Tam` olduğu gibi, uzun metin kısaltılarak yazılır. Hız saniyede birin altındaysa `12,3 sn/adım`. Hız yaklaşık son 30 saniyenin üstel ortalamasıdır; kalan süre ondan hesaplanır. Toplamsız sayaçta kalan süre yerine geçen süre gösterilir.
- Standart hata bir terminalse çubuk tek satırda, satır başından en çok saniyede 10 kez yeniden çizilir ve terminal genişliğine kırpılır; `yazdır`, uyarılar ve hata mesajları çubuğu silip yazılır, çubuk alttan yeniden çizilir. Bitince satır özet satırına dönüşür. Aynı anda tek çubuk görünür: yenisi açılınca eskisi görünmez olur (saymayı ve günlüğe yazmayı sürdürür), yenisi bitince yeniden görünür.
- Terminal değilse (dosya, boru, not defteri hücresi, sürekli sınama) en çok 10 saniyede bir düz satır ve sonda özet satırı yazılır.
- Arka plan işinde (`TR_İŞ` ortam değişkeni iş klasörünü gösterir) çubuk terminal kipine geçmez ve iş klasörüne saniyede en çok bir kez `ilerleme.json` anlık görüntüsü yazar (geçici dosya + yerine koyma): `{"adım":1200,"toplam":5000,"ölçüler":{"kayıp":2.31},"hız":85.2,"kalan_sn":840,"zaman":"...","bitti":false}`.
