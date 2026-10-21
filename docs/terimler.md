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

## İleride karar verilecek

- **Dil çekirdeği / hesap çekirdekleri** (yazılım katmanları: `internal/lang`, `internal/compute`): "çekirdek" burada mecaz (*core*). Donanım anlamıyla aynı cümlede seyrek geçer; değişiklik gerekirse ayrıca tartışılır.
- **Dosya biçimlerindeki alan adları:** profil raporunun JSON alanları (`gpu.çekirdekler` ...) makinelerin okuduğu biçimin parçasıdır; biçim sürümü (`FormatVersion`) değişene kadar olduğu gibi kalır.
- **Go yorumları:** geliştiriciye dönük Go yorumlarında "çekirdek" (kernel anlamında) geçişleri kademeli olarak "kernel"e çevrilir; denetim testi şimdilik yalnızca kullanıcıya görünen metinlere bakar.
