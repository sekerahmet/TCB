# 9. Uzun eğitim: kontrol noktası ve devam

Saatler süren bir eğitim kesilebilir: not defterinin oturumu kapanır, Ctrl+C'ye basarsınız, makine yeniden başlar. TR'de bunun için tek bir satır yeter: `kontrol_noktası` eğitimin bütün durumunu (model ağırlıkları, iyileştiricinin momentleri, veri okuyucusunun konumu, günlük, kendi sayaçlarınız) düzenli aralıklarla koşu klasörüne kaydeder; programı **aynı komutla** yeniden çalıştırdığınızda kaldığı adımdan sürer. Sürdürülen eğitim, hiç kesilmemiş bir eğitimle **bit düzeyinde aynı** sonucu verir. Kurallar: [`docs/spec/20_kontrol_noktasi.md`](../spec/20_kontrol_noktasi.md). Uçtan uca örnek: [`examples/tr/40_kontrol_noktasi.tr`](../../examples/tr/40_kontrol_noktasi.tr).

```tr
değişken ayar = komut_satırı({adım: 20000, koşu: "runs/deneme1"})
değişken model = {E: tensör(türevsiz(rastgele_normal([50000, 256], tohum: 1) * 0.02), türev_izle: doğru)}
değişken op = AdamW(parametreler(model), öğrenme_oranı: 0.0003)
değişken ö = veri_kümesi("veri/train").örnekleyici(toplu: 16, bağlam: 512, tohum: 1)
değişken g = günlük(yol(ayar.koşu, "metrics.jsonl"))
değişken izlenen = {en_iyi_kayıp: 99.0}
değişken kn = kontrol_noktası(ayar.koşu, {model: model, op: op, ö: ö, g: g, izlenen: izlenen})
değişken ç = ilerleme(ayar.adım, günlük: g)
döngü adım içinde kn.kaldığı_adım..ayar.adım {
    değişken giriş, hedef = ö.sonraki()
    op.sıfırla()
    değişken kayıp = çapraz_entropi(gömme(model.E, giriş) @ model.E.devrik(), hedef)
    geri_yayılım(kayıp)
    op.adım()
    eğer kayıp.değer < izlenen.en_iyi_kayıp { izlenen.en_iyi_kayıp = kayıp.değer }
    ç.güncelle({kayıp: kayıp}, adım: adım + 1)
    kn.güncelle(adım + 1) // 15 dakikada bir ve Ctrl+C gelince adım sınırında kaydeder
}
kn.kaydet()
```

**Ne kaydedilir.** Sözlükte verdiğiniz her şey: modeller (tensör içeren yapı, liste ya da sözlük; içindeki bütün tensörler — türev izleyen parametreler de, türev izlemeyen durum tensörleri de: kendi yazdığınız iyileştiricinin momentumu, sabit tablolar —; yüklemede var olan tensörlere yerinde yazılır; modelin tensör olmayan alanları — ayarlar, sayılar — kaydedilmez), iyileştiriciler (`AdamW`, `SGD`), okuyucular (`örnekleyici`, `sıralı`), günlükler ve düz değerli sözlükler (`{en_iyi_kayıp: 99.0, sayaç: 0}`). Sözlüğün anahtarları sizin adlarınızdır. Tek başına bir sayı verilemez (geri yüklenirken değişkeninize yazılamaz); bir sözlüğe koyun. Öğrenme oranı gibi ayarlar kaydedilmez: onları kodunuz ya da komut satırı verir.

**Ne zaman kaydedilir.** `kn.güncelle(adım)` her adımın sonunda çağrılır; son kayıttan beri `her_dakika` (varsayılan 15) dakika geçtiyse orada kaydeder. Eğitimin sonunda `kn.kaydet()` hemen kaydeder. Ctrl+C'ye bastığınızda program o anki adımı bitirir, noktayı kaydeder ve durur (`Kontrol noktası kaydedildi: adım 1532 ...`; çıkış kodu 3); hemen çıkmak için Ctrl+C'ye ikinci kez basın. Süreç öldürülürse (elektrik kesintisi, çalışma makinesinin geri alınması) en son düzenli noktadan sürülür; aradaki adımlar yeniden hesaplanır ve aynı sonucu verir.

**Kaldığı yerden devam.** Aynı komutu yeniden çalıştırın. Program başta `Kaldığı yerden devam: adım 1500 (runs/deneme1/kontrol_noktalari/adim_0000001500)` yazar; `kn.kaldığı_adım` 1500 olur, döngü oradan başlar. Eğitimi uzatmak için `--adım` değerini artırmanız yeter (ayarlardaki fark yalnızca bir not olarak yazılır). Modeli, iyileştiriciyi ya da okuyucunun ayarlarını değiştirdiyseniz program yükleme yapmadan, neyin farklı olduğunu söyleyen bir hatayla durur: baştan başlamak için `runs/deneme1/kontrol_noktalari` klasörünü silin ya da yeni bir koşu klasörü verin.

**Günlük.** `metrics.jsonl` silinmez ve kesilmez: sürdürülen eğitim araya `{"tür":"devam","adım":1500}` kaydı yazar. Bu kayıttan önce gelen ve adımı 1500'den büyük olan ölçüler (kesilmeden önce yazılmış, sonra yeniden hesaplanan adımlar) geçersizdir; kayıp eğrisi çizen araçlar bu kuralı uygular.

**Koşu klasörü.** Her eğitim kendi klasörüne yazar; kontrol noktaları onun `kontrol_noktalari/` alt klasöründedir, öbür dosyalarınıza dokunulmaz. Önerilen düzen:

```text
runs/deneme1/
  config.json            ayarlar: dosya_yaz(yol(koşu, "config.json"), json_metin(ayar))
  metrics.jsonl          günlük: her adımın ölçüleri
  samples.json           üretilen örnek metinler
  results.json           son değerlendirme
  speed.json             hız ölçümleri
  model.trt              son ağırlıklar: tensör_kaydet(..., parametreler(model, adlarıyla: doğru))
  kontrol_noktalari/
    son.json             en son nokta
    adim_0000001500/     tensorler.trt, durum.json, ozet.json
    adim_0000001000/     tut: 2 → bir önceki de durur
```

**Bulut not defteri ve Drive.** Bağlanmış Drive klasörüne doğrudan yazmak yavaştır ve kaydın son adımı (yeniden adlandırma) orada başarısız olabilir. Koşu klasörünü çalışma makinesinin yerel diskinde (`/content/koşular/deneme1`) tutun ve ara ara Drive'a kopyalayın:

```tr
kopyala("/content/koşular/deneme1", "/content/drive/MyDrive/koşular", üzerine_yaz: doğru)
```

Oturum kapanınca klasörü Drive'dan yerel diske geri kopyalayıp aynı komutu çalıştırın. Doğrudan Drive'a yazıyorsanız eğitim bir kayıt başarısız olunca durmaz: bir uyarı yazılır, sonraki aralıkta yeniden denenir; üç kez art arda başarısız olursa açık bir hatayla durur ve son geçerli noktayı söyler.

**Bozuk nokta.** Yarım kopyalanmış ya da değiştirilmiş bir nokta (dosyaların özeti tutmuyor) sessizce atlanmaz: program hangi dosyanın bozuk olduğunu ve bir önceki geçerli noktayı söyleyerek durur. Bozuk klasörü silerseniz bir önceki noktadan sürer.

**Elle kaydetmek.** Kontrol noktası kullanmadan da parçaları kendiniz kaydedebilirsiniz: `tensör_kaydet("op.trt", op.durum)` ile iyileştiricinin durumu, `ö.durum` ile okuyucunun konumu (düz bir sözlük, `json_metin` ile yazılır) alınır; `op.durumu_yükle(tensör_yükle("op.trt"))` ve `ö.durumu_yükle(...)` geri yükler. Hazır ağırlıklarla ince ayar için `parametreleri_yükle(model, tensör_yükle("model.trt"))` ağırlıkları modelin var olan parametrelerine yazar; iyileştirici aynı tensörleri güncellemeyi sürdürür.

**GPU.** Model GPU'daysa her şey aynı çalışır: ağırlıklar ve iyileştiricinin durumu kaydedilirken konağa indirilir, yüklenirken aynı cihaza konur. Kesintisiz ve sürdürülen eğitim aynı cihaz türünde bit düzeyinde aynıdır; CPU'da kaydedip GPU'da sürdürmek çalışır ama son bitte farklı sonuç verebilir. Rastgeleliği (`rastgele_normal`, `örnekle`) her zaman `tohum:` ile verin; kontrol noktası kurulduktan sonra tohumsuz rastgelelik kullanılırsa TR bir kez uyarır.

**NaN ve kayıp sıçraması.** Uzun bir eğitim bir gün bozuk bir gradyanla NaN'e düşebilir ya da kaybı birden sıçrayabilir. TR bunu yakalamanın parçalarını verir, ne yapılacağına siz karar verirsiniz:

- `sonlu_mu(x)` bir sayının ya da tensörün elemanlarının NaN ve sonsuz olmadığını söyler.
- `gradyan_kırp(..., sonlu_değilse_hata: doğru)` normu bozuk gradyanda hata verir.
- `aktar "tr/egitim"` ile gelen `SıçramaBekçisi` her adımın kaybını ve gradyan normunu son 128 sağlam adımla karşılaştırır ve `TAMAM`, `SONLU_DEĞİL` ya da `SIÇRAMA` der.
- `kn.geri_dön()` son kontrol noktasını yerinde yükleyip adımını verir; `ö.atla(n)` sorunlu toplu işleri atlar.

```tr
aktar "tr/egitim"
değişken w = tensör([1.0, 2.0], türev_izle: doğru)
değişken op = AdamW([w])
değişken ö = veri_kümesi("veri").örnekleyici(toplu: 8, bağlam: 32, tohum: 1)
değişken kn = kontrol_noktası("koşu", {w: w, op: op, ö: ö})
değişken bekçi = SıçramaBekçisi()
değişken adım = kn.kaldığı_adım
döngü adım < 1000 {
    değişken giriş, hedef = ö.sonraki()
    op.sıfırla()
    değişken kayıp = toplam(w * w)
    geri_yayılım(kayıp)
    değişken norm = gradyan_kırp([w], en_fazla: 1.0)
    değişken karar = bekçi.gözle(kayıp.değer, norm.değer)
    eğer karar != TAMAM {
        adım = kn.geri_dön()
        ö.atla(200)
        devam
    }
    op.adım()
    adım += 1
    kn.güncelle(adım)
}
```

Denetimi `op.adım()`'dan ve `kn.güncelle`'den önce yapın: bozuk adım modele ve kontrol noktasına girmesin. Tam örnek: [`examples/tr/44_egitim_kararliligi.tr`](../../examples/tr/44_egitim_kararliligi.tr).
