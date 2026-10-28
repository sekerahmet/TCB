# 9. Uzun eğitim: kontrol noktası ve devam

Saatler süren bir eğitim kesilebilir: not defterinin oturumu kapanır, Ctrl+C'ye basarsınız, makine yeniden başlar. TR'de bunun için tek bir satır yeter: `TR_fn_checkpoint` eğitimin bütün durumunu (model ağırlıkları, iyileştiricinin momentleri, veri okuyucusunun konumu, günlük, kendi sayaçlarınız) düzenli aralıklarla koşu klasörüne kaydeder; programı **aynı komutla** yeniden çalıştırdığınızda kaldığı adımdan sürer. Sürdürülen eğitim, hiç kesilmemiş bir eğitimle **bit düzeyinde aynı** sonucu verir. Kurallar: [`docs/spec/20_kontrol_noktasi.md`](../spec/20_kontrol_noktasi.md). Uçtan uca örnek: [`examples/tr/40_kontrol_noktasi.tr`](../../examples/tr/40_kontrol_noktasi.tr).

```tr
# TR 4
TR_var ayar = TR_command_line({"step": 20000, "koşu": "runs/deneme1"})
TR_var model = {"E": TR_tensor(TR_no_grad(TR_randn([50000, 256], TR_seed: 1) * 0.02), TR_requires_grad: TR_true)}
TR_var op = TR_AdamW(TR_parameters(model), TR_lr: 0.0003)
TR_var ö = TR_dataset("veri/train").TR_sampler(TR_batch_size: 16, TR_context: 512, TR_seed: 1)
TR_var g = TR_logger(TR_path(ayar["koşu"], "metrics.jsonl"))
TR_var izlenen = {"en_iyi_kayıp": 99.0}
TR_var kn = TR_checkpoint(ayar["koşu"], {"model": model, "op": op, "ö": ö, "g": g, "izlenen": izlenen})
TR_var ç = TR_progress(ayar["step"], TR_log: g)
TR_for adım TR_in kn.TR_resume_step..ayar["step"] {
    TR_var giriş, hedef = ö.TR_next()
    op.TR_zero_grad()
    TR_var kayıp = TR_cross_entropy(TR_embedding(model["E"], giriş) @ model["E"].TR_transpose(), hedef)
    TR_backward(kayıp)
    op.TR_step()
    TR_if kayıp.TR_tolist < izlenen["en_iyi_kayıp"] { izlenen["en_iyi_kayıp"] = kayıp.TR_tolist }
    ç.TR_update({"loss": kayıp}, TR_step: adım + 1)
    kn.TR_update(adım + 1) # 15 dakikada bir ve Ctrl+C gelince adım sınırında kaydeder
}
kn.TR_save()
```

**Ne kaydedilir.** Sözlükte verdiğiniz her şey: modeller (tensör içeren yapı, liste ya da sözlük; içindeki bütün tensörler — türev izleyen parametreler de, türev izlemeyen durum tensörleri de: kendi yazdığınız iyileştiricinin momentumu, sabit tablolar —; yüklemede var olan tensörlere yerinde yazılır; modelin tensör olmayan alanları — ayarlar, sayılar — kaydedilmez), iyileştiriciler (`TR_fn_AdamW`, `TR_fn_SGD`), okuyucular (`TR_mem_sampler`, `TR_mem_sequential`), günlükler ve düz değerli sözlükler (`{en_iyi_kayıp: 99.0, sayaç: 0}`). Sözlüğün anahtarları sizin adlarınızdır. Tek başına bir sayı verilemez (geri yüklenirken değişkeninize yazılamaz); bir sözlüğe koyun. Öğrenme oranı gibi ayarlar kaydedilmez: onları kodunuz ya da komut satırı verir.

**Ne zaman kaydedilir.** `kn.TR_mem_update(adım)` her adımın sonunda çağrılır; son kayıttan beri `her_dakika` (varsayılan 15) dakika geçtiyse orada kaydeder. Eğitimin sonunda `kn.TR_mem_save()` hemen kaydeder. Ctrl+C'ye bastığınızda program o anki adımı bitirir, noktayı kaydeder ve durur (`Kontrol noktası kaydedildi: adım 1532 ...`; kabuk 130 görür); hemen çıkmak için Ctrl+C'ye ikinci kez basın. Süreç öldürülürse (elektrik kesintisi, çalışma makinesinin geri alınması) en son düzenli noktadan sürülür; aradaki adımlar yeniden hesaplanır ve aynı sonucu verir.

**Kaldığı yerden devam.** Aynı komutu yeniden çalıştırın. Program başta `Kaldığı yerden devam: adım 1500 (runs/deneme1/checkpoints/step_0000001500)` yazar; `kn.TR_mem_resume_step` 1500 olur, döngü oradan başlar. Eğitimi uzatmak için `--adım` değerini artırmanız yeter (ayarlardaki fark yalnızca bir not olarak yazılır). Modeli, iyileştiriciyi ya da okuyucunun ayarlarını değiştirdiyseniz program yükleme yapmadan, neyin farklı olduğunu söyleyen bir hatayla durur: baştan başlamak için `runs/deneme1/checkpoints` klasörünü silin ya da yeni bir koşu klasörü verin.

**Günlük.** `metrics.jsonl` silinmez ve kesilmez: sürdürülen eğitim araya `{"kind":"resume","step":1500}` kaydı yazar (günlüğün ve kontrol noktasının anahtarları dilden bağımsız İngilizce adlardır, programın hangi dilde yazıldığından bağımsız aynıdır). Bu kayıttan önce gelen ve adımı 1500'den büyük olan ölçüler (kesilmeden önce yazılmış, sonra yeniden hesaplanan adımlar) geçersizdir; kayıp eğrisi çizen araçlar bu kuralı uygular.

**Koşu klasörü.** Her eğitim kendi klasörüne yazar; kontrol noktaları onun `checkpoints/` alt klasöründedir, öbür dosyalarınıza dokunulmaz. Önerilen düzen:

```text
runs/deneme1/
  config.json            ayarlar: dosya_yaz(yol(koşu, "config.json"), json_metin(ayar))
  metrics.jsonl          günlük: her adımın ölçüleri
  samples.json           üretilen örnek metinler
  results.json           son değerlendirme
  speed.json             hız ölçümleri
  model.trt              son ağırlıklar: tensör_kaydet(..., parametreler(model, adlarıyla: doğru))
  checkpoints/
    latest.json          en son nokta
    step_0000001500/     tensors.trt, state.json, digest.json
    step_0000001000/     tut: 2 → bir önceki de durur
```

**Bulut not defteri ve Drive.** Bağlanmış Drive klasörüne doğrudan yazmak yavaştır ve kaydın son adımı (yeniden adlandırma) orada başarısız olabilir. Koşu klasörünü çalışma makinesinin yerel diskinde (`/content/koşular/deneme1`) tutun ve ara ara Drive'a kopyalayın:

```tr
# TR 4
TR_copy("/content/koşular/deneme1", "/content/drive/MyDrive/koşular", TR_overwrite: TR_true)
```

Oturum kapanınca klasörü Drive'dan yerel diske geri kopyalayıp aynı komutu çalıştırın. Doğrudan Drive'a yazıyorsanız eğitim bir kayıt başarısız olunca durmaz: bir uyarı yazılır, sonraki aralıkta yeniden denenir; üç kez art arda başarısız olursa açık bir hatayla durur ve son geçerli noktayı söyler.

**Bozuk nokta.** Yarım kopyalanmış ya da değiştirilmiş bir nokta (dosyaların özeti tutmuyor) sessizce atlanmaz: program hangi dosyanın bozuk olduğunu ve bir önceki geçerli noktayı söyleyerek durur. Bozuk klasörü silerseniz bir önceki noktadan sürer.

**Elle kaydetmek.** Kontrol noktası kullanmadan da parçaları kendiniz kaydedebilirsiniz: `TR_fn_save_tensors("op.trt", op.TR_mem_state)` ile iyileştiricinin durumu, `ö.TR_mem_state` ile okuyucunun konumu (düz bir sözlük, `TR_fn_json_string` ile yazılır) alınır; `op.TR_mem_load_state(TR_fn_load_tensors("op.trt"))` ve `ö.TR_mem_load_state(...)` geri yükler. Hazır ağırlıklarla ince ayar için `TR_fn_load_parameters(model, TR_fn_load_tensors("model.trt"))` ağırlıkları modelin var olan parametrelerine yazar; iyileştirici aynı tensörleri güncellemeyi sürdürür.

**GPU.** Model GPU'daysa her şey aynı çalışır: ağırlıklar ve iyileştiricinin durumu kaydedilirken konağa indirilir, yüklenirken aynı cihaza konur. Kesintisiz ve sürdürülen eğitim aynı cihaz türünde bit düzeyinde aynıdır; CPU'da kaydedip GPU'da sürdürmek çalışır ama son bitte farklı sonuç verebilir. Rastgeleliği (`TR_fn_randn`, `TR_fn_sample`) her zaman `TR_arg_seed:` ile verin; kontrol noktası kurulduktan sonra tohumsuz rastgelelik kullanılırsa TR bir kez uyarır.

**NaN ve kayıp sıçraması.** Uzun bir eğitim bir gün bozuk bir gradyanla NaN'e düşebilir ya da kaybı birden sıçrayabilir. TR bunu yakalamanın parçalarını verir, ne yapılacağına siz karar verirsiniz:

- `TR_fn_is_finite(x)` bir sayının ya da tensörün elemanlarının NaN ve sonsuz olmadığını söyler.
- `TR_fn_clip_grad(..., TR_arg_error_if_nonfinite: TR_kw_true)` normu bozuk gradyanda hata verir.
- `TR_kw_import "tr/egitim"` ile gelen `TR_type_SpikeGuard` her adımın kaybını ve gradyan normunu son 128 sağlam adımla karşılaştırır ve `TR_const_OK`, `TR_const_NONFINITE` ya da `TR_const_SPIKE` der.
- `kn.TR_mem_rollback()` son kontrol noktasını yerinde yükleyip adımını verir; `ö.TR_mem_skip(n)` sorunlu toplu işleri atlar.

```tr
# TR 4
TR_import "tr/egitim"
TR_var w = TR_tensor([1.0, 2.0], TR_requires_grad: TR_true)
TR_var op = TR_AdamW([w])
TR_var ö = TR_dataset("veri").TR_sampler(TR_batch_size: 8, TR_context: 32, TR_seed: 1)
TR_var kn = TR_checkpoint("koşu", {"w": w, "op": op, "ö": ö})
TR_var bekçi = TR_SpikeGuard()
TR_var adım = kn.TR_resume_step
TR_for adım < 1000 {
    TR_var giriş, hedef = ö.TR_next()
    op.TR_zero_grad()
    TR_var kayıp = TR_sum(w * w)
    TR_backward(kayıp)
    TR_var norm = TR_clip_grad([w], TR_max: 1.0)
    TR_var karar = bekçi.TR_observe(kayıp.TR_tolist, norm.TR_tolist)
    TR_if karar != TR_OK {
        adım = kn.TR_rollback()
        ö.TR_skip(200)
        TR_continue
    }
    op.TR_step()
    adım += 1
    kn.TR_update(adım)
}
```

Denetimi `op.TR_mem_step()`'dan ve `kn.TR_mem_update`'den önce yapın: bozuk adım modele ve kontrol noktasına girmesin. Tam örnek: [`examples/tr/44_egitim_kararliligi.tr`](../../examples/tr/44_egitim_kararliligi.tr).
