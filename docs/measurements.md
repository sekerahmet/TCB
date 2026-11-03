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

Birim parça/sn. Bu makinede TR, llama.cpp f32'nin ~%40–50'si ve transformers'tan yavaş. 16 bitlik ağırlık CPU'da float32'nin 0,4 katı hızda: ağırlık her çarpımda geçici float32 tampona açılıyor (yol haritası, 2. kilometre taşı "CPU" eksikleri). 2026-10-22'nin oranlarıyla (TR/llama.cpp 0,25, TR/transformers 1,05; 48 iş parçacığı) karşılaştırmayın: makine ve iş parçacığı sayısı farklı. 48 iş parçacıklı karşılığı aşağıda, G4 koşusunda.

**Eğitim hızı, GPU** (2026-11-03 16:54, main `51b75f0a`, Colab G4: RTX PRO 6000 Blackwell, AMD EPYC 9B45 48 iş parçacığı; torch 2.11.0+cu130; hedef model rgn512, sentetik veri, 120 adım; aynı makinede TR ve derlenmiş PyTorch sırayla 3'er kez) [ölçüldü: Colab, betik `g4_olcum_2026_11_03.sh`]: TR 147, 147, 147; PyTorch 137, 136, 137 ms/adım. **TR 1,08 kat yavaş** [hesap: 147 / 136,7]. 2026-11-01'deki 145 / 136'yla (1,07) aynı oran: S-kilit eğitim hızını değiştirmedi.

**Üretim hızı, GPU ve 48 iş parçacıklı CPU** (aynı G4 koşusu, 16:57–17:03; transformers 5.22.0, llama.cpp `28b812d`; `tools/gpu/generation_race.sh` aynen; 100 parçalık açgözlü üretim; tek koşu) [ölçüldü: aynı betik]:

| Model, cihaz | TR float32 | TR bf16 ağırlık | transformers eager | transformers static + compile | llama.cpp f32 | llama.cpp f16 |
| :-- | --: | --: | --: | --: | --: | --: |
| SmolLM2-135M, GPU | 507,6 | 555,6 | 107,4 | 470,3 | 956,6 | 1079,4 |
| Qwen3-0.6B, GPU | 266,0 | 330,0 | 94,2 | 279,0 | 439,6 | 663,2 |
| SmolLM2-135M, CPU | 84,2 | 34,3 | 36,8 | — | 214,4 | 380,3 |
| Qwen3-0.6B, CPU | 25,1 | 10,3 | 11,5 | — | 50,7 | 92,0 |

Birim parça/sn. Aynı koşudaki oranlar [hesap]:
- **GPU:** TR (bf16) transformers'ın en hızlı yolunun 1,18 katı; llama.cpp f32'nin 0,58 (SmolLM2) ve 0,75 (Qwen3) katı, f16'nın 0,5 katı.
- **CPU, 48 iş parçacığı:** TR (float32) transformers'ın 2,2–2,3 katı; llama.cpp f32'nin 0,39 (SmolLM2) ve 0,49 (Qwen3) katı. 16 bitlik ağırlık burada da float32'nin 0,4 katı hızda.
- **GPU'da gerileme:** 2026-10-22'ye göre TR'nin GPU üretimi %7–10 yavaş (Qwen3 bf16 366 → 330, float32 290 → 266; SmolLM2 bf16 608 → 556, float32 548 → 508), llama.cpp aynı (%1 içinde: 442 → 440, 965 → 957). Kart türü aynı, örnek başka; llama.cpp'nin aynı kalması gerilemenin TR'de olduğunu gösteriyor [hipotez: ölçüm, iki commit aynı koşuda karşılaştırılarak doğrulanacak].
- **CPU'da ilerleme:** aynı tür 48 iş parçacıklı makinede 2026-10-22'ye göre TR'nin CPU üretimi 2,1–2,3 kat hızlı (Qwen3 12,1 → 25,1, SmolLM2 36 → 84,2), llama.cpp benzer (47,5 → 50,7) [ölçüldü; makine örneği başka].

L4 adım süresinin dört birleşmedeki değerleri: 938, 923,5, 962, 880,5, 888 ms [ölçüldü: Colab defteri]. En büyük fark %9,3 [hesap: 962 / 880,5]; aynı koşuda karşılaştırılmadığı için bunu hızlanma ya da yavaşlama diye okumayın. Bu satırlarda PyTorch ile karşılaştırma yok.

## Eski (yeniden ölçülecek)

| Ölçüm | Tarih, commit | Neden eski | Yeniden ölçüm |
| :-- | :-- | :-- | :-- |
| **Dil kıyası ve CPU karakter modeli** (`tools/bench/README.md` "Güncel taban") | 2026-11-01, `6d21ed09`, Colab CPU High-RAM | Yorumlayıcı (Kol A eşitlik, 13.1) ve tensör çekirdeği değişti | CPU High-RAM'de `tools/bench/cpu_baseline.sh` |

2026-10-22'nin üretim hızları ve 2026-11-01/02'nin eğitim hızı 2026-11-03'te yeniden ölçüldü (yukarıda); eski sayılar arşivde (`docs/archive/roadmap-2026-10-27.md` Evre 5) ve Colab defterinde kalır.

Motora bağlı olmayan ölçümler (ör. Jarvis tabanı: llama.cpp ve Whisper'ın kendi süreleri, 2026-11-01) eskimez; TR'yi ölçmedikleri için bu belgede yer almazlar.

## Sıradaki ölçümler

1. Dil kıyası (TR, Python, Go, Rust) ve karakter modeli, CPU: CPU High-RAM'de koşuyor (2026-11-03 17:00).
2. GPU üretimindeki %7–10'luk gerilemenin yeri: 2026-10-22'nin commit'i ile main aynı G4 koşusunda (onay gerekir).
