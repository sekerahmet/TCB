# 10. Hazır katmanlarla GPT

Bir dil modelini her seferinde ağırlık ağırlık yazmak gerekmez ([`examples/tr/35_transformer.tr`](../../examples/tr/35_transformer.tr) bunu öğretmek için yapar). Dilin kitaplığı hazır katmanlar ve iyileştiricilerle gelir; TR ile yazılmıştır, `tr` ikilisine gömülüdür ve `aktar "tr/..."` ile alınır ([dil tanımı 11.6](../spec/11_moduller.md)). Kitaplığın kodu okunur: [`lib/katman.tr`](../../lib/katman.tr), [`lib/iyilestirici.tr`](../../lib/iyilestirici.tr); bütün adlar [`lib/README.md`](../../lib/README.md)'de. Uçtan uca örnek: [`examples/tr/41_gpt.tr`](../../examples/tr/41_gpt.tr).

## Model

```tr
aktar "tr/katman"

değişken model = GPT(4096, bağlam: 256, boyut: 256, katman: 4, bakış_sayısı: 4, paylaşımlı: doğru, tohum: 1)
değişken logit = model.ileri([[1, 2, 3]])            // [1, 3, 4096]
değişken kayıp = model.kayıp([[1, 2, 3]], [[2, 3, 4]]) // ortalama çapraz entropi
yazdır(parametreler(model, adlarıyla: doğru).anahtarlar()[0..3])
```

`GPT` sıradan bir `yapı`dır: `model.bloklar[0].dikkat.sorgu_katmanı.ağırlık` bir tensördür, `parametreler(model)` bütün ağırlıkları bulur, `tensör_kaydet(yol, parametreler(model, adlarıyla: doğru))` kaydeder. İçinde döner konumlu (RoPE) nedensel çok bakışlı dikkat (İngilizce *multi-head attention*; `kv_bakış_sayısı:` ile gruplu sorgu), SwiGLU ileri besleme ve ön-norm RMS normu vardır. Ağırlıklar N(0, 0.02) ile, artık yola yazan çıkış katmanları `0.02 / √(2 · katman)` ile başlar.

Modeli değiştirmek isterseniz `lib/katman.tr`'yi kendi klasörünüze kopyalayıp `aktar "katman"` yazın: kopyanız sizindir, kitaplık değişmez. Tek bir katmanı da kullanabilirsiniz: `Doğrusal(giriş, çıkış)`, `GömmeKatmanı`, `RMSNormKatmanı`, `NedenselDikkat`, `SwiGLU`, `TransformerBloğu`; hepsinin ileri hesabı `ileri` metodudur.

## Yerel dikkat, PLE ve hedef model seçenekleri

`GPT` ve `TransformerBloğu` daha büyük tariflere seçeneklerle uyar; hepsinin varsayılanı yukarıdaki sade modeldir. Katman başına yerel pencere (`pencereler: katman_pencereleri("YYGYGYYG", [32, 64])`: G katmanı tam nedensel, Y katmanları son 32/64 konuma bakar), q/k normu (`qk_normu: doğru`), Muon'un K ve V'yi (`birleşik_kv`) ile SwiGLU'nun kapı ve yukarı matrislerini (`birleşik_kapı`) birlikte dikleştirmesi için birleşik matrisler, `epsilon`, sapmalar ve katman başına kelime gömmesi (`ple_boyutu`, Gemma 3n'deki PLE) vardır:

```tr
aktar "tr/katman"

değişken model = GPT(4096, bağlam: 64, boyut: 64, katman: 4, bakış_sayısı: 4, kv_bakış_sayısı: 2, ara_boyut: 96, paylaşımlı: doğru,
    pencereler: katman_pencereleri("YGYG", [8, 16]), qk_normu: doğru, birleşik_kv: doğru, birleşik_kapı: doğru,
    ileri_sapmalı: doğru, epsilon: 0.000001, ple_boyutu: 32)
yazdır(model.üret([1, 2, 3], 10, sıcaklık: 0, eos: 0)) // eos örneklenirse üretim durur
```

`üret` KV önbelleğiyle çalışır: başlangıç bir kez işlenir, sonraki her adım yalnızca yeni parçayı hesaplar (anahtar ve değerler katman başına önbellekte tutulur). `önbellek_çeşidi: "sabit"` (varsayılan; bir kez ayrılır, yerine yazılır), `"büyüyen"` (her adımda uzar) ya da `yok` (her adımda bütün dizi) seçilebilir; sonuç üçünde de aynıdır. Kendi üretim döngüsünü yazan `değişken ö = model.önbellek_kur()` ile `model.ileri(kimlikler, önbellek: ö)` çağırır; `ö.kırp(n)` önbelleği ilk `n` parçaya indirir (sonrakileri geri alır).

Ayrıntılı anlatım ve formüller [`lib/README.md`](../../lib/README.md)'dedir. İyileştirici tarafında aynı tarif için `MuonVeAdamW(model, muon_ölçeği: "rms", adamw_çürümesi: 0.1, ayrı: ["gömme", "ple_bağlamı.ctx_katmanı"], seyrek: ["ple.tablo"])` yazılır: parametreler ada göre (önek, sonek ya da tam yol) gruplanır, PLE tabloları satır seyrek RMS iyileştiricisine (`SatırRMS`, yalnızca o adımda gradyanı olan satırları günceller) gider.

## İyileştirici: Muon + AdamW

```tr
aktar "tr/katman"
aktar "tr/iyilestirici"

değişken model = GPT(4096, bağlam: 256, boyut: 256, katman: 4, bakış_sayısı: 4)
değişken op = MuonVeAdamW(model, muon_oranı: 0.02, adamw_oranı: 0.003)
değişken toplam_adım = 1000
döngü adım içinde 0..3 {
    op.çarpan_ayarla(oran_planı(adım, toplam_adım, ısınma: 100, azalma: 0.2))
    op.sıfırla()
    geri_yayılım(model.kayıp([[1, 2, 3]], [[2, 3, 4]]))
    op.adım()
}
```

`MuonVeAdamW` modelin parametrelerini ikiye ayırır (`parametre_grupları`): gizli katmanların 2 boyutlu ağırlıkları **Muon**'a, gömme tablosu, çıkış katmanı, norm ağırlıkları ve sapmalar **AdamW**'ye gider. Muon her ağırlığın momentumunu Newton–Schulz yinelemesiyle dikleştirir (`dikleştir`): güncelleme bütün yönlerde aynı büyüklükte olur; küçük ve orta modellerde AdamW'den belirgin hızlı öğrenir. `normuon: doğru` NorMuon'u seçer (dikleştirmeden sonra nöron başına normalleştirme; öğrenme oranı AdamW ölçeğindedir, ör. `muon_oranı: 0.003`).

`oran_planı` ısınma–sabit–azalma (WSD) planıdır: ilk `ısınma` adımda 0'dan 1'e çıkar, sonra sabit kalır, son `azalma` kesrinde 0'a iner. Çarpan iki grubun oranını birlikte ölçekler.

## Kendi iyileştiricinizi yazmak

İyileştiriciler TR kodudur; tek yapı taşı parametreyi türev izlemeden **yerinde** güncellemektir ([15.7](../spec/15_tensorler.md)):

```tr
fonksiyon işaretli_sgd(ağırlıklar, oran) {
    türevsiz {
        döngü p içinde ağırlıklar {
            eğer p.gradyan != yok {
                p[0..p.şekil[0]] -= oran * işaret(p.gradyan) // ağırlığın kendisine yazar
            }
        }
    }
}
```

Muon'u değiştirmek için `lib/iyilestirici.tr`'yi kopyalayın: `Muon.adım()` on satırdır. Durum tensörlerini (momentum gibi) `türev_izle: doğru` ile kurarsanız `kontrol_noktası` onları da saklar ([`lib/README.md`](../../lib/README.md), kontrol noktası).

## Eğitim döngüsü, kontrol noktası ve GPU

```tr
aktar "tr/katman"
aktar "tr/iyilestirici"

değişken ayar = komut_satırı({adım: 100, koşu: "runs/gpt"})
değişken p = parçalayıcı_yükle("../tests/data/tokenizer.json")
değişken ö = veri_kümesi("../tests/data/text").örnekleyici(toplu: 8, bağlam: 32, tohum: 1)
değişken model = GPT(p.dağarcık_boyutu, bağlam: 32, boyut: 32, katman: 2, bakış_sayısı: 4, paylaşımlı: doğru)
değişken op = MuonVeAdamW(model)
değişken g = günlük(yol(ayar.koşu, "metrics.jsonl"))
değişken kn = kontrol_noktası(ayar.koşu, {model: model, adamw: op.adamw, muon: op.muon.durum, ö: ö, g: g})
değişken ç = ilerleme(ayar.adım, günlük: g)
döngü adım içinde kn.kaldığı_adım..ayar.adım {
    op.çarpan_ayarla(oran_planı(adım, ayar.adım, ısınma: 10))
    değişken giriş, hedef = ö.sonraki()
    op.sıfırla()
    değişken kayıp = model.kayıp(giriş, hedef)
    geri_yayılım(kayıp)
    op.adım()
    ç.güncelle({kayıp: kayıp}, adım: adım + 1)
    kn.güncelle(adım + 1)
}
yazdır(p.çöz(model.üret(p.kodla("Bugün"), 12, sıcaklık: 0.8, tohum: 1)))
```

Kontrol noktasına modelin yanında iki iyileştiricinin durumu da verilir: `op.adamw` ve `op.muon.durum`. GPU'da eğitmek için model kurulmadan önce `varsayılan_cihaz("gpu")` çağrılır; ağırlıklar, Muon'un durumu ve toplu işler GPU'da olur ([15.27](../spec/15_tensorler.md)). Bulut not defterinde büyük bir eğitim için [`tools/gpu/gpt_training.tr`](../../tools/gpu/gpt_training.tr) hazır betiğine bakın (SimpleStories, d512, 8 katman; [`tools/gpu/COLAB.md`](../../tools/gpu/COLAB.md) 9. hücre).

## Hazır model, üretim ve kayıtlı oynatma

Hugging Face biçiminde kaydedilmiş bir Llama ya da Qwen3 modelini (`config.json` + `model.safetensors`) eğitmeden kullanmak için `tr/hazir` yeter. Ağ gerekmez: model önceden indirilmiş bir klasördür.

```tr
aktar "tr/hazir"

değişken model = hazır_model("modeller/SmolLM2-135M", ağırlık_türü: "bf16")
değişken p = parçalayıcı_yükle("modeller/SmolLM2-135M/tokenizer.json")
değişken kimlikler = model.üret(p.kodla("Bugün hava"), 20, sıcaklık: 0)
yazdır(p.çöz(kimlikler))
```

- **`ağırlık_türü`:** matris ağırlıklarının (gömme ve doğrusal katmanlar) türüdür (`"float32"`, `"bf16"`, `"fp16"`); normlar ve hesap float32 kalır. Hugging Face modelleri çoğunlukla bf16 saklanır, `ağırlık_türü: "bf16"` ağırlıkları yarı bellekle tutar. Ayrıntı: [`lib/README.md`](../../lib/README.md).
- **`üret` ve önbellek:** `model.üret(kimlikler, adet, sıcaklık:, en_iyi_k:, tohum:, eos:, önbellek_çeşidi:, çizge:)` KV önbelleğiyle çalışır: başlangıç bir kez işlenir, her sonraki adım yalnızca yeni parçayı hesaplar. `önbellek_çeşidi` `"sabit"` (varsayılan; bir kez ayrılır, yerine yazılır), `"büyüyen"` ya da `yok` (her adımda bütün dizi) olabilir; sonuç üçünde de aynıdır. `sıcaklık: 0` her adımda en olası parçayı seçer (açgözlü); aynı `tohum` aynı örneği verir. Ayrıntı: [`lib/README.md`](../../lib/README.md) "Üretim".
- **`çizge` ve `tekrar_oynat`:** GPU'da sabit önbellekle (`çizge: doğru`, varsayılan) her üretim adımı bir kez kaydedilir (CUDA Graph) ve sonraki adımlarda tek seferde oynatılır; yorumlayıcı adımı yeniden çalıştırmaz ve yüzlerce kernelin tek tek başlatılması ortadan kalkar. Aynı mekanizmayı kendi adımınız için `tekrar_oynat(f)` ile, durumu yerinde güncellemek için `yerine_yaz(hedef, kaynak, eksen:, başlangıç:)` ile kullanırsınız; değişen her değer (konum, parça kimliği) tensör olarak verilir. CPU'da `tekrar_oynat(f)` `f`'i doğrudan çağırır, sonuç aynıdır. Kurallar: [dil tanımı 15.48–15.49](../spec/15_tensorler.md). Örnekler: [`46_hazir_model_ve_uretim.tr`](../../examples/tr/46_hazir_model_ve_uretim.tr), [`47_kendi_uretim_dongusu.tr`](../../examples/tr/47_kendi_uretim_dongusu.tr), [`45_tekrar_oynat.tr`](../../examples/tr/45_tekrar_oynat.tr).
