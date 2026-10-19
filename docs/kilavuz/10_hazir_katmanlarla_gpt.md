# 10. Hazır katmanlarla GPT

Bir dil modelini her seferinde ağırlık ağırlık yazmak gerekmez ([`ornekler/06_transformer.tr`](../../ornekler/06_transformer.tr) bunu öğretmek için yapar). Dilin kitaplığı hazır katmanlar ve iyileştiricilerle gelir; TR ile yazılmıştır, `tr` ikilisine gömülüdür ve `aktar "tr/..."` ile alınır ([dil tanımı 11.6](../dil/11_moduller.md)). Kitaplığın kodu okunur: [`kitaplik/katman.tr`](../../kitaplik/katman.tr), [`kitaplik/iyilestirici.tr`](../../kitaplik/iyilestirici.tr); bütün adlar [`kitaplik/README.md`](../../kitaplik/README.md)'de. Uçtan uca örnek: [`ornekler/11_gpt.tr`](../../ornekler/11_gpt.tr).

## Model

```tr
aktar "tr/katman"

değişken model = GPT(4096, bağlam: 256, boyut: 256, katman: 4, baş: 4, paylaşımlı: doğru, tohum: 1)
değişken logit = model.ileri([[1, 2, 3]])            // [1, 3, 4096]
değişken kayıp = model.kayıp([[1, 2, 3]], [[2, 3, 4]]) // ortalama çapraz entropi
yazdır(parametreler(model, adlarıyla: doğru).anahtarlar()[0..3])
```

`GPT` sıradan bir `yapı`dır: `model.bloklar[0].dikkat.sorgu_katmanı.ağırlık` bir tensördür, `parametreler(model)` bütün ağırlıkları bulur, `tensör_kaydet(yol, parametreler(model, adlarıyla: doğru))` kaydeder. İçinde döner konumlu (RoPE) nedensel çok başlı dikkat (`kv_baş:` ile gruplu sorgu), SwiGLU ileri besleme ve ön-norm RMS normu vardır. Ağırlıklar N(0, 0.02) ile, artık yola yazan çıkış katmanları `0.02 / √(2 · katman)` ile başlar.

Modeli değiştirmek isterseniz `kitaplik/katman.tr`'yi kendi klasörünüze kopyalayıp `aktar "katman"` yazın: kopyanız sizindir, kitaplık değişmez. Tek bir katmanı da kullanabilirsiniz: `Doğrusal(giriş, çıkış)`, `GömmeKatmanı`, `RMSNormKatmanı`, `NedenselDikkat`, `SwiGLU`, `TransformerBloğu`; hepsinin ileri hesabı `ileri` metodudur.

## İyileştirici: Muon + AdamW

```tr
aktar "tr/katman"
aktar "tr/iyilestirici"

değişken model = GPT(4096, bağlam: 256, boyut: 256, katman: 4, baş: 4)
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

İyileştiriciler TR kodudur; tek yapı taşı parametreyi türev izlemeden **yerinde** güncellemektir ([15.7](../dil/15_tensorler.md)):

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

Muon'u değiştirmek için `kitaplik/iyilestirici.tr`'yi kopyalayın: `Muon.adım()` on satırdır. Durum tensörlerini (momentum gibi) `türev_izle: doğru` ile kurarsanız `kontrol_noktası` onları da saklar ([`kitaplik/README.md`](../../kitaplik/README.md), kontrol noktası).

## Eğitim döngüsü, kontrol noktası ve GPU

```tr
aktar "tr/katman"
aktar "tr/iyilestirici"

değişken ayar = komut_satırı({adım: 100, koşu: "koşular/gpt"})
değişken p = parçalayıcı_yükle("../testler/veri/parcalayici.json")
değişken ö = veri_kümesi("../testler/veri/metin").örnekleyici(toplu: 8, bağlam: 32, tohum: 1)
değişken model = GPT(p.dağarcık_boyutu, bağlam: 32, boyut: 32, katman: 2, baş: 4, paylaşımlı: doğru)
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

Kontrol noktasına modelin yanında iki iyileştiricinin durumu da verilir: `op.adamw` ve `op.muon.durum`. GPU'da eğitmek için model kurulmadan önce `varsayılan_cihaz("gpu")` çağrılır; ağırlıklar, Muon'un durumu ve toplu işler GPU'da olur ([15.27](../dil/15_tensorler.md)). Bulut not defterinde büyük bir eğitim için [`tools/gpu/gpt_egitim.tr`](../../tools/gpu/gpt_egitim.tr) hazır betiğine bakın (SimpleStories, d512, 8 katman; [`tools/gpu/COLAB.md`](../../tools/gpu/COLAB.md) 9. hücre).
