# Terimler

TR'nin kullanıcıya görünen metinlerinde (belgeler, iletiler, ad açıklamaları, örnekler, kitaplık) kullanılan teknik terimler. Türkçe makine öğrenmesi terimlerinin çoğu İngilizcenin ilk, birebir çevirisidir ve bazıları dilde başka bir anlamla çakışır. Kural:

**İngilizce kelime birebir çevrilmez, kavrama ad verilir.** Kavramın ne yaptığını söyleyen ve TR'nin metinlerinde başka bir anlamla çakışmayan kelime seçilir. Türkçede yerleşik ve tek anlamlı bir karşılık varsa o kalır; yerleşik bir alıntı (GPU, bf16, kernel) daha açıksa alıntı kullanılır. Bir terim değişince eski ad ve gerekçe bu tabloda kalır.

Yeniden adlandırmanın etkisini `tr dizin yeniden_adlandır <ad> <yeni>` (kod adları) ve `tr dizin yeniden_adlandır <terim> <yeni> --terim` (yalnızca metin) listeler. `internal/doc_terms_test.go` aşağıdaki "kullanılmaz" sütunundaki kalıpların kullanıcıya görünen metinlerde geçmediğini denetler.

| Terim | İngilizce | Anlamı | Kullanılmaz (eski) | Neden | Tarih |
| :-- | :-- | :-- | :-- | :-- | :-- |
| bakış | head (attention head) | Dikkatin, diziye kendi izdüşüm alt uzayından bakan paralel kolu. `bakış_sayısı`, `kv_bakış_sayısı`, `bakış_boyutu`; "çok bakışlı dikkat". | baş, çok başlı, baş sayısı, baş boyu, dikkat başı, dikkat başları | "Baş" eş seslidir: TR'de "başlangıç" anlamında da geçer (`t[baş..son]`, `dilim(eksen, baş, son)`). "Bakış" mekanizmayı söyler: her biri ayrı bir bakış açısı. | 2026-10-21 |
| tip / tür | type / dtype | **tip** dilin değer tipidir (`TR_Int`, `TR_String`, `TR_Tensor`, `TR_Optimizer`; `TR_Type(x)`, tip açıklaması, tip denetimi). **tür** bir tensörün elemanlarının sayı türüdür (`"float32"`, `"bf16"`, `"fp16"`; `TR_dtype:`, `.TR_dtype`, `.TR_to_dtype`). Kitaplıkta da yalnızca bu anlamlarda kullanılır: önbelleğin "sabit"/"büyüyen" seçimi `çeşit`tir. | tür (değer tipi için), tip (sayı türü için), tür (önbellek çeşidi için) | Türkçede iki kelime eş anlamlıdır; iki ayrı düzeyi (değer ve tensörün içindeki sayı) ayırmak için her biri tek anlama bağlanır (Python `type(t)` ve `t.dtype` gibi). | 2026-10-22 |
| boyut / büyüklük / eksen | dimension / size / axis (`dim=`) | **boyut** bir uzayın ya da vektörün boyutudur: modelin genişliği (`boyut`), `bakış_boyutu` (head_dim), `ara_boyut`, `boyutu_koru` (keepdim). **büyüklük** bir şeyin kaç elemandan ya da bayttan oluştuğudur (size). **eksen** işlemin yürüdüğü eksendir (`TR_sum(t, TR_dim: 0)`; İngilizce adı PyTorch'taki gibi `dim`). | — | Türkçede "dimension" boyut diye yerleşik; "size" büyüklüktür. Aynı Türkçe kelime iki kavrama verilmez. Bugün `.TR_vocab_size` (vocab_size) büyüklük anlamında: yeniden adlandırma önerisi kitaplık adları ad tablosunda. | 2026-10-25 |
| kernel | kernel (compute kernel) | GPU'da binlerce iş parçacığıyla ya da işlemcide SIMD ile çalışan düşük düzeyli hesap işlevi (`dikkat_geri_q_tc`, kaynaşık kernel, matris çarpımı kernelleri). Ekler okunuşa göre, kesme işaretsiz: kerneller, kernelden. | çekirdek (bu anlamda), son başlatılan çekirdek, çekirdek süreleri | Türkçede "çekirdek" hem *kernel* hem *core*'un karşılığıdır; TR metinlerinde ikisi aynı cümlede geçer ("tensör çekirdeklerini kullanan geri yön çekirdeği"). "Kernel" Türkçe teknik dilde yerleşik bir alıntıdır ve bu bağlamda tek anlamlıdır. | 2026-10-21 |
| çekirdek | core | Yalnızca donanım: işlemci çekirdeği, GPU'nun CUDA çekirdekleri, tensör çekirdekleri (matris çarpımı donanımı). | — | Yerleşik ve tek anlamlı (ör. "8 çekirdekli işlemci"). | 2026-10-21 |
| süzgeç | convolution kernel / filter | Evrişimin öğrenilen ağırlık penceresi (`kernel_size` → `süzgeç_boyu`). TR'de evrişim geldiğinde. | çekirdek (bu anlamda) | *Kernel*'ın evrişim anlamı GPU kernel'ıyla çakışmasın. | 2026-10-21 (ileride) |
| bitiştir | concatenate (`torch.cat`) | Tensör listesini bir eksen boyunca uç uca ekler. | birleştir (tensörde) | Liste metodu `.TR_join` metin yapar (Python `str.join`); iki ayrı iş aynı adı taşıyordu. Rust da `concat` ile `join`'i ayırır. | 2026-10-21 |
| fark_adımı | finite-difference step (h; PyTorch gradcheck `eps`) | `TR_grad_check`'de merkezi farkın adım büyüklüğü. | adım (türev denetiminde) | `adım` eğitim adımıdır (`op.TR_step()`, adım sayısı). `eps` olmaz: normların kararlılık terimi `epsilon` ile çakışır (PyTorch'ta da iki anlamda `eps` var). | 2026-10-21 |

## Eş adlar

Bir ad kod listesinde (`internal/lang/codes/tr.json`) birden çok canlı koda verilebilir ama yalnızca iki durumda (Swift API Design Guidelines: *"Methods can share a base name when they share the same basic meaning or when they operate in distinct domains"*):

- **aynı anlam:** kodlar aynı işi ya da aynı kavramı adlandırır (`yardım` komutu, seçeneği ve hata ayıklayıcı komutu).
- **ayrı alan:** biri komut satırının (`tr` komutları, seçenekleri, kabuk ve hata ayıklayıcı komutları: `cmd`, `sub`, `opt`, `repl`, `dbg`, `env` sınıfları), öbürü dilin adıdır; ikisi aynı metinde karşılaşmaz.

Dilin kendi içinde farklı işe aynı ad verilmez: tensörleri uç uca ekleyen fonksiyon bu yüzden `bitiştir`dir (liste metodu `.birleştir` metin yapar), türev denetiminin fark büyüklüğü `fark_adımı`dır (`adım` eğitim adımıdır). `internal/doc_terms_test.go` (`TestSharedNamesListed`) aynı adı taşıyan her canlı kod grubunun bu tabloda bulunduğunu, tablonun güncel olduğunu ve "ayrı alan" gerekçesinin gerçekten bir komut satırı koduyla bir dil kodunu ayırdığını denetler. Yeni bir eş ad bu tabloya gerekçesiyle girmeden kod listesine giremez. Bir istisna tabloya yazılmaz: yalnız **özellik** türündeki üye kodlarıyla adlı argüman kodlarından oluşan ve İngilizce adları da aynı olan grup kendiliğinden aynı anlamdır (alan ile onu kuran adlı argüman tek kavramdır, Python dataclass'ında olduğu gibi: `öğrenme_oranı`: `TR_lr:` ve `.TR_lr`). Metot içeren grup bu kurala girmez, tabloya yazılır (`.TR_step` metodu); İngilizce adları ayrılan çift de tabloya yazılır. Kurala giren grubun satırı tabloda kalmaz.

| Ad | Kodlar | Gerekçe |
| :-- | :-- | :-- |
| adım | TR_arg_step, TR_dbg_step, TR_mem_step | ayrı alan: TR_dbg_step hata ayıklayıcıda bir adım ilerler; TR_arg_step (tamamlanan adım sayısı) ile TR_mem_step (`op.TR_step()`, PyTorch `optimizer.step()`) aynı anlamda eğitim adımıdır |
| devam | TR_dbg_continue, TR_kw_continue | aynı anlam: sürdürmek (döngünün sonraki turu; hata ayıklayıcıda programı sürdürmek) |
| değiştir | TR_dbg_set, TR_mem_replace | ayrı alan: TR_dbg_set hata ayıklayıcıda bir değişkenin değerini değiştirir; TR_mem_replace metinde parçayı değiştirir |
| durum | TR_mem_state, TR_opt_status | ayrı alan: TR_opt_status `TR_cmd_index`'in dizin durumu; TR_mem_state iyileştiricinin sürdürme durumu |
| günlük | TR_arg_log, TR_mem_logger | aynı anlam: TR_arg_log adlı argümanı `TR_Logger(...)`'ın açtığı günlüğü alır; `TR_Run`nun günlük alanı (TR_mem_logger) o günlüktür |
| izle | TR_cmd_watch, TR_dbg_watch | aynı anlam: izlemeye almak (`TR_cmd_watch` bir koşuyu, hata ayıklayıcı bir ifadeyi) |
| kodla | TR_cmd_encode, TR_mem_encode | ayrı alan: TR_cmd_encode dosyayı kodlu biçime çevirir; TR_mem_encode parçalayıcıda metni parça kimliklerine çevirir |
| kontrol | TR_cmd_check, TR_opt_check | aynı anlam: `TR_cmd_check` komutu ve `--kontrol` seçeneği aynı denetimi yapar |
| mutlak | TR_fn_abs, TR_mem_absolute | aynı anlam: göreli olmayan (mutlak değer, mutlak yol; Python `abs` ve `Path.absolute`) |
| test | TR_cmd_test, TR_ctx_test | aynı anlam: `TR_ctx_test` bloğu ve onu koşan `tr test` |
| tür | TR_arg_dtype, TR_mem_dtype, TR_opt_kind | ayrı alan: TR_opt_kind `TR_cmd_index`'de düğüm türü; TR_arg_dtype ile TR_mem_dtype aynı anlamda tensörün veri türüdür |
| yardım | TR_cmd_help, TR_dbg_help, TR_opt_help | aynı anlam: yardım metni (komut, seçenek, hata ayıklayıcı) |
| çıkış | TR_dbg_quit, TR_repl_exit | aynı anlam: kabuktan ve hata ayıklayıcıdan çıkmak |
| değer | TR_arg_value, TR_mem_tolist | aynı anlam: düz sayı değeri (`TR_SpikeGuard.TR_exceeds(değer, geçmiş)` izlenen sayıyı, tensörün `.TR_tolist`i tensörün düz değerini verir) |
| dikkat | TR_fn_attention, TR_mem_attention | aynı anlam: dikkat hesabı (`TR_attention(q, k, v)` yerleşiği ve bloğun onu çalıştıran `.TR_attention` katmanı; PyTorch `scaled_dot_product_attention` ve `self_attn`) |
| ileri | TR_arg_forward, TR_mem_forward | aynı anlam: ileri hesap (`özel_türev(ileri:, geri:)` ve katmanın `ileri` metodu; PyTorch `forward`) |
| kapı | TR_arg_gate, TR_mem_gate_proj | aynı anlam: kapı (`TR_ple_mix`ndaki kapı değeri ve onu üreten `kapı` katmanı; SwiGLU ve PLE kapısı) |
| kayıp | TR_arg_loss, TR_mem_loss | aynı anlam: eğitim kaybı (`TR_SpikeGuard.TR_observe(kayıp, norm)` ve kaybı hesaplayan `model.TR_loss(...)`) |
| maske | TR_arg_mask, TR_mem_mask | aynı anlam: dikkat maskesi (`dikkat(..., maske:)` ve `TR_KVCache.TR_mask()` ile kurulan maske) |
| parametreler | TR_arg_parameters, TR_fn_parameters, TR_mem_parameters | aynı anlam: modelin eğitilen tensörleri (`TR_parameters(model)`, `TR_Muon(parametreler)` ve iyileştiricinin `.TR_parameters` alanı; PyTorch `model.parameters()`, `Optimizer(params)`) |
| rope_tabanı | TR_arg_rope_theta, TR_fn_rope_theta | aynı anlam: RoPE tabanı (`GPT(rope_tabanı:)` ve onu Hugging Face ayarından okuyan `TR_rope_theta(ayar)`; Hugging Face `rope_theta`) |
| tablo | TR_dbg_table, TR_mem_table | ayrı alan: TR_dbg_table hata ayıklayıcıda bir değeri tablo olarak gösterir; TR_mem_table gömme katmanının ağırlık tablosudur |

## İleride karar verilecek

- **Dil çekirdeği / hesap çekirdekleri** (yazılım katmanları: `internal/lang`, `internal/compute`): "çekirdek" burada mecaz (*core*). Donanım anlamıyla aynı cümlede seyrek geçer; değişiklik gerekirse ayrıca tartışılır.
- **Dosya biçimlerindeki alan adları:** profil raporunun JSON alanları (`gpu.çekirdekler` ...) makinelerin okuduğu biçimin parçasıdır; biçim sürümü (`FormatVersion`) değişene kadar olduğu gibi kalır.
- **Go yorumları:** geliştiriciye dönük Go yorumlarında "çekirdek" (kernel anlamında) geçişleri kademeli olarak "kernel"e çevrilir; denetim testi şimdilik yalnızca kullanıcıya görünen metinlere bakar.
