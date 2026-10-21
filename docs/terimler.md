# Terimler

TR'nin kullanıcıya görünen metinlerinde (belgeler, iletiler, ad açıklamaları, örnekler, kitaplık) kullanılan teknik terimler. Türkçe makine öğrenmesi terimlerinin çoğu İngilizcenin ilk, birebir çevirisidir ve bazıları dilde başka bir anlamla çakışır. Kural:

**İngilizce kelime birebir çevrilmez, kavrama ad verilir.** Kavramın ne yaptığını söyleyen ve TR'nin metinlerinde başka bir anlamla çakışmayan kelime seçilir. Türkçede yerleşik ve tek anlamlı bir karşılık varsa o kalır; yerleşik bir alıntı (GPU, bf16, kernel) daha açıksa alıntı kullanılır. Bir terim değişince eski ad ve gerekçe bu tabloda kalır.

Yeniden adlandırmanın etkisini `tr dizin yeniden_adlandır <ad> <yeni>` (kod adları) ve `tr dizin yeniden_adlandır <terim> <yeni> --terim` (yalnızca metin) listeler. `internal/doc_terms_test.go` aşağıdaki "kullanılmaz" sütunundaki kalıpların kullanıcıya görünen metinlerde geçmediğini denetler.

| Terim | İngilizce | Anlamı | Kullanılmaz (eski) | Neden | Tarih |
| :-- | :-- | :-- | :-- | :-- | :-- |
| bakış | head (attention head) | Dikkatin, diziye kendi izdüşüm alt uzayından bakan paralel kolu. `bakış_sayısı`, `kv_bakış_sayısı`, `bakış_boyutu`; "çok bakışlı dikkat". | baş, çok başlı, baş sayısı, baş boyu, dikkat başı, dikkat başları | "Baş" eş seslidir: TR'de "başlangıç" anlamında da geçer (`t[baş..son]`, `dilim(eksen, baş, son)`). "Bakış" mekanizmayı söyler: her biri ayrı bir bakış açısı. | 2026-10-21 |
| kernel | kernel (compute kernel) | GPU'da binlerce iş parçacığıyla ya da işlemcide SIMD ile çalışan düşük düzeyli hesap işlevi (`dikkat_geri_q_tc`, kaynaşık kernel, matris çarpımı kernelleri). Ekler okunuşa göre, kesme işaretsiz: kerneller, kernelden. | çekirdek (bu anlamda), son başlatılan çekirdek, çekirdek süreleri | Türkçede "çekirdek" hem *kernel* hem *core*'un karşılığıdır; TR metinlerinde ikisi aynı cümlede geçer ("tensör çekirdeklerini kullanan geri yön çekirdeği"). "Kernel" Türkçe teknik dilde yerleşik bir alıntıdır ve bu bağlamda tek anlamlıdır. | 2026-10-21 |
| çekirdek | core | Yalnızca donanım: işlemci çekirdeği, GPU'nun CUDA çekirdekleri, tensör çekirdekleri (matris çarpımı donanımı). | — | Yerleşik ve tek anlamlı (ör. "8 çekirdekli işlemci"). | 2026-10-21 |
| süzgeç | convolution kernel / filter | Evrişimin öğrenilen ağırlık penceresi (`kernel_size` → `süzgeç_boyu`). TR'de evrişim geldiğinde. | çekirdek (bu anlamda) | *Kernel*'ın evrişim anlamı GPU kernel'ıyla çakışmasın. | 2026-10-21 (ileride) |
| bitiştir | concatenate (`torch.cat`) | Tensör listesini bir eksen boyunca uç uca ekler. | birleştir (tensörde) | Liste metodu `.birleştir` metin yapar (Python `str.join`); iki ayrı iş aynı adı taşıyordu. Rust da `concat` ile `join`'i ayırır. | 2026-10-21 |
| fark_adımı | finite-difference step (h; PyTorch gradcheck `eps`) | `türev_denetle`'de merkezi farkın adım büyüklüğü. | adım (türev denetiminde) | `adım` eğitim adımıdır (`op.adım()`, adım sayısı). `eps` olmaz: normların kararlılık terimi `epsilon` ile çakışır (PyTorch'ta da iki anlamda `eps` var). | 2026-10-21 |

## Eş adlar

Bir ad kod listesinde (`internal/lang/codes/tr.json`) birden çok canlı koda verilebilir ama yalnızca iki durumda (Swift API Design Guidelines: *"Methods can share a base name when they share the same basic meaning or when they operate in distinct domains"*):

- **aynı anlam:** kodlar aynı işi ya da aynı kavramı adlandırır (`yardım` komutu, seçeneği ve hata ayıklayıcı komutu).
- **ayrı alan:** biri komut satırının (`tr` komutları, seçenekleri, kabuk ve hata ayıklayıcı komutları, K kodları), öbürü dilin adıdır; ikisi aynı metinde karşılaşmaz.

Dilin kendi içinde farklı işe aynı ad verilmez: tensörleri uç uca ekleyen fonksiyon bu yüzden `bitiştir`dir (liste metodu `.birleştir` metin yapar), türev denetiminin fark büyüklüğü `fark_adımı`dır (`adım` eğitim adımıdır). `internal/doc_terms_test.go` (`TestSharedNamesListed`) aynı adı taşıyan her canlı kod grubunun bu tabloda bulunduğunu, tablonun güncel olduğunu ve "ayrı alan" gerekçesinin gerçekten bir komut satırı koduyla bir dil kodunu ayırdığını denetler. Yeni bir eş ad bu tabloya gerekçesiyle girmeden kod listesine giremez.

| Ad | Kodlar | Gerekçe |
| :-- | :-- | :-- |
| adım | K300, T982, Y500 | ayrı alan: K300 hata ayıklayıcıda bir adım ilerler; T982 (tamamlanan adım sayısı) ile Y500 (`op.adım()`, PyTorch `optimizer.step()`) aynı anlamda eğitim adımıdır |
| devam | A014, K303 | aynı anlam: sürdürmek (döngünün sonraki turu; hata ayıklayıcıda programı sürdürmek) |
| değiştir | K304, Y104 | ayrı alan: K304 hata ayıklayıcıda bir değişkenin değerini değiştirir; Y104 metinde parçayı değiştirir |
| durum | K112, Y502 | ayrı alan: K112 `tr dizin`'in dizin durumu; Y502 iyileştiricinin sürdürme durumu |
| günlük | G810, G912 | aynı anlam: G912 adlı argümanı G810'un açtığı günlüğü alır |
| izle | K012, K307 | aynı anlam: izlemeye almak (`tr izle` bir koşuyu, hata ayıklayıcı bir ifadeyi) |
| kodla | K009, Y610 | ayrı alan: K009 dosyayı kodlu biçime çevirir; Y610 parçalayıcıda metni parça kimliklerine çevirir |
| çöz | K010, Y611 | ayrı alan: `kodla`'nın tersleri (K010 kodlu dosya, Y611 parça kimlikleri) |
| kontrol | K004, K101 | aynı anlam: `tr kontrol` komutu ve `--kontrol` seçeneği aynı denetimi yapar |
| mutlak | G403, Y314 | aynı anlam: göreli olmayan (mutlak değer, mutlak yol; Python `abs` ve `Path.absolute`) |
| test | A101, K007 | aynı anlam: `test` bloğu ve onu koşan `tr test` |
| tür | K110, T901, Y402 | ayrı alan: K110 `tr dizin`'de düğüm türü; T901 ile Y402 aynı anlamda tensörün veri türüdür |
| yardım | K001, K106, K312 | aynı anlam: yardım metni (komut, seçenek, hata ayıklayıcı) |
| çıkış | K200, K313 | aynı anlam: kabuktan ve hata ayıklayıcıdan çıkmak |
| öğrenme_oranı | T910, Y504 | aynı anlam: iyileştiricinin öğrenme oranı (adlı argüman ve özellik) |

## İleride karar verilecek

- **Dil çekirdeği / hesap çekirdekleri** (yazılım katmanları: `internal/lang`, `internal/compute`): "çekirdek" burada mecaz (*core*). Donanım anlamıyla aynı cümlede seyrek geçer; değişiklik gerekirse ayrıca tartışılır.
- **Dosya biçimlerindeki alan adları:** profil raporunun JSON alanları (`gpu.çekirdekler` ...) makinelerin okuduğu biçimin parçasıdır; biçim sürümü (`FormatVersion`) değişene kadar olduğu gibi kalır.
- **Go yorumları:** geliştiriciye dönük Go yorumlarında "çekirdek" (kernel anlamında) geçişleri kademeli olarak "kernel"e çevrilir; denetim testi şimdilik yalnızca kullanıcıya görünen metinlere bakar.
