# Güncel ölçümler

Bu belge yalnız motorun en son hâliyle yapılmış ölçümleri tutar; son gözden geçirme 2026-11-03, main `51b75f0a`. Eski bir ölçüm, motor o günden beri değiştiyse bizi yanıltabilir. Bu yüzden eski ölçümler ayrı bir listede durur ve yeniden ölçülene dek karar için kullanılmaz (Ahmet, 2026-11-03).

## Kural

- **Bir ölçüm ne zaman güncel:** commit'inden bu yana ölçtüğü yolu değiştiren bir dil işi birleşmemişse. Örneğin tensör çekirdeğine, yorumlayıcıya ya da kilide dokunan bir dilim, onlara bağlı bütün hız ölçümlerini eskitir. Kuşkuda ölçüm eski sayılır.
- **Her satırda:** tarih, commit, makine, ne ölçüldü, sonuç, karşılaştırılan ve kaynak (`tools/record` kaydı ya da Colab defteri). Commit'siz ölçüm buraya girmez.
- **Karşılaştırma aynı makinede, aynı koşuda yapılır.** İki ayrı makinenin mutlak sayıları karşılaştırılmaz; oran yazılır.
- **Yerel makine paylaşımlıdır:** yan oturumlar çalışırken sayılar gürültülü. Hız ölçümleri Colab'da koşulur (CPU High-RAM ya da GPU) ve sürümleri kayda yazılır.
- Bir dilim birleşince TCB bu belgeyi gözden geçirir: dilimin eskittiği satırlar "Eski" listesine taşınır.

## Güncel (2026-11-03, main `51b75f0a` itibarıyla)

| Tarih | Commit | Makine | Ne | Sonuç | Kaynak |
| :-- | :-- | :-- | :-- | :-- | :-- |
| 2026-11-03 | `6cab4f89` (16.3 birleşmesi) | Colab L4 | Hedef model eğitim adımı (kart sınaması, `tools/gpu/card_check.sh`) | 888 ms/adım, 10454 MiB | L4 günlüğü, Colab defteri 15:57 koşusu |
| 2026-11-03 | `af24785b` (S-kilit birleşmesi) | Colab L4 | Aynı adım | 880,5 ms/adım | Colab defteri 12:39 koşusu |
| 2026-11-03 | `fdf726fc` (S-kilit dalı) | Colab L4, CPU | Uzun tensör işi sürerken başka görevin beklemesi; 100 Hz döngü | gözcü ≤ 3,6 ms, kaçan tur 0 (2×10 koşu) | Colab defteri 10:45 koşusu |
| 2026-11-03 | `6cab4f89` | Colab L4 | `gpt_training.tr` ve `ss_exam.tr` gerçek kartta, dolgulu dağarcık | 5,5 ms/adım (çok küçük model; yalnız çalıştığının sınaması) | Colab defteri 16:09 koşusu |

**Üretim hızı, CPU** (2026-11-03 16:43–16:55, main `51b75f0a`, Colab CPU High-RAM: Intel Xeon 2.20 GHz, 8 iş parçacığı; torch 2.11.0+cpu, transformers 5.22.0, llama.cpp `28b812d`; 100 parçalık açgözlü üretim, `tools/gpu/generation_race.sh`'in CPU kolu; tek koşu) [ölçüldü: Colab, betik `cpu_uretim_yarisi.sh`]:

| Model | TR float32 | TR bf16 ağırlık | transformers | llama.cpp f32 | llama.cpp f16 | TR/llama.cpp f32 | TR/transformers |
| :-- | --: | --: | --: | --: | --: | --: | --: |
| SmolLM2-135M | 24,2 | 10,0 | 27,4 | 62,1 | 104,0 | 0,39 | 0,88 |
| Qwen3-0.6B | 7,8 | 3,1 | 11,4 | 15,4 | 24,5 | 0,51 | 0,68 |

Birim parça/sn. Bu makinede TR, llama.cpp f32'nin ~%40–50'si ve transformers'tan yavaş. 16 bitlik ağırlık CPU'da float32'nin 0,4 katı hızda: ağırlık her çarpımda geçici float32 tampona açılıyor (yol haritası, 2. kilometre taşı "CPU" eksikleri). 2026-10-22'nin oranlarıyla (TR/llama.cpp 0,25, TR/transformers 1,05; 48 iş parçacığı) karşılaştırmayın: makine ve iş parçacığı sayısı farklı.

L4 adım süresinin dört birleşmedeki değerleri: 938, 923,5, 962, 880,5, 888 ms [ölçüldü: Colab defteri]. En büyük fark %9,3 [hesap: 962 / 880,5]; aynı koşuda karşılaştırılmadığı için bunu hızlanma ya da yavaşlama diye okumayın. Bu satırlarda PyTorch ile karşılaştırma yok.

## Eski (yeniden ölçülecek)

| Ölçüm | Tarih, commit | Neden eski | Yeniden ölçüm |
| :-- | :-- | :-- | :-- |
| **Üretim hızı, CPU:** Qwen3-0.6B'de TR 12,1, llama.cpp f32 47,5, transformers 11,5 parça/sn (llama.cpp'nin ~1/5'i) | 2026-10-22, Colab, 48 iş parçacığı | Cinsli tensör, kilit, KV önbelleği ve çözme o günden sonra değişti | Yeniden ölçüldü: yukarıda, 2026-11-03 (8 iş parçacığı). 48 iş parçacıklı makinede ayrıca ölçülmedi |
| **Üretim hızı, GPU:** TR llama.cpp f32'nin %63–83'ü, transformers'ın en hızlı yolundan hızlı | 2026-10-22, Colab | Aynı | GPU makinesinde aynı betik (onay gerekir) |
| **Eğitim hızı:** hedef modelde TR 145, derlenmiş PyTorch 136 ms/adım (1,07 kat). TR'nin son ölçümü 2026-11-02, G4: T1'in cinsli tensör dilimi gerçek kartta iki hatayla (PTX adı, kimlik okumasının +%6'sı) düzeldikten sonra taban `27979f33` 145 ↔ `a385f657` 146 ms/adım (rgn512, 120 adım, ABAB) [ölçüldü: G4, Colab defteri 20:22 koşusu]; aynı gün G4 kart sınamasında hedef model 154 ms/adım (`ee5550bc`) | 2026-11-01 (PyTorch), 2026-11-02 (TR), Colab G4 | S-kilit (2026-11-03, `af24785b`) her arka uç veri çağrısına kapı ekledi ve G4'te ölçülmedi; L4 kartında S-kilit'ten sonra yavaşlama görünmedi (880,5 ↔ 923,5–962 ms, ayrı koşular). PyTorch'un 136'sı motorumuza bağlı değil ama başka bir G4 örneğinde ölçüldü | G4'te aynı koşuda TR ve PyTorch (onay gerekir) |
| **Dil kıyası ve CPU karakter modeli** (`tools/bench/README.md` "Güncel taban") | 2026-11-01, `6d21ed09`, Colab CPU High-RAM | Yorumlayıcı (Kol A eşitlik, 13.1) ve tensör çekirdeği değişti | CPU High-RAM'de `tools/bench/cpu_baseline.sh` |

Motora bağlı olmayan ölçümler (ör. Jarvis tabanı: llama.cpp ve Whisper'ın kendi süreleri, 2026-11-01) eskimez; TR'yi ölçmedikleri için bu belgede yer almazlar.

## Sıradaki ölçümler

1. Eğitim hızı ve GPU üretim yarışı: G4'te koşuyor (Ahmet onayı 2026-11-03 16:53).
2. Dil kıyası ve karakter modeli, CPU: CPU High-RAM.
