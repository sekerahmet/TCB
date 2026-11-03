# Güncel ölçümler

Bu belge yalnız motorun en son hâliyle yapılmış ölçümleri tutar; son gözden geçirme 2026-11-03, main `51b75f0a`. Eski bir ölçüm, motor o günden beri değiştiyse bizi yanıltabilir. Bu yüzden eski ölçümler ayrı bir listede durur ve yeniden ölçülene dek karar için kullanılmaz (Ahmet, 2026-11-03).

## Kural

- **Bir ölçüm ne zaman güncel:** commit'inden bu yana ölçtüğü yolu değiştiren bir dil işi birleşmemişse. Örneğin tensör çekirdeğine, yorumlayıcıya ya da kilide dokunan bir dilim, onlara bağlı bütün hız ölçümlerini eskitir. Kuşkuda ölçüm eski sayılır.
- **Her satırda:** tarih, commit, makine, ne ölçüldü, sonuç, karşılaştırılan ve kaynak (`tools/record` kaydı ya da Colab defteri). Commit'siz ölçüm buraya girmez.
- **Karşılaştırma aynı makinede, aynı koşuda yapılır.** İki ayrı makinenin mutlak sayıları karşılaştırılmaz; oran yazılır.
- **Colab'ın aynı adlı makinesi oturumdan oturuma değişir** (CPU High-RAM 2026-11-03'te bir kez AMD EPYC 7B12, bir kez Intel Xeon geldi): her satırda işlemcinin adı yazılır.
- **Yerel makine paylaşımlıdır:** yan oturumlar çalışırken sayılar gürültülü. Hız ölçümleri Colab'da koşulur (CPU High-RAM ya da GPU) ve sürümleri kayda yazılır.
- Bir dilim birleşince TCB bu belgeyi gözden geçirir: dilimin eskittiği satırlar "Eski" listesine taşınır.

## Güncel (2026-11-03, main `51b75f0a` itibarıyla)

| Tarih | Commit | Makine | Ne | Sonuç | Kaynak |
| :-- | :-- | :-- | :-- | :-- | :-- |
| 2026-11-03 | `3a5114d2` (S-kilit, birleşen sürüm) | Colab L4, CPU | Uzun tensör işi sürerken başka görevin beklemesi; 100 Hz döngü | CPU: gözcü ≤ 3,5 ms 10/10, kaçan tur 0 10/10; GPU: uzun işte tur kaçıyor (status 10.2, C, Ö2) | Colab defteri 11:08 koşusu |
| 2026-11-03 | `6cab4f89` | Colab L4 | `gpt_training.tr` ve `ss_exam.tr` gerçek kartta, dolgulu dağarcık | 5,5 ms/adım (çok küçük model; yalnız çalıştığının sınaması) | Colab defteri 16:09:50 koşusu |

**Üretim hızı, CPU** (2026-11-03 16:43–16:55, main `51b75f0a`, Colab CPU High-RAM: Intel Xeon 2.20 GHz, 8 iş parçacığı; torch 2.11.0+cpu, transformers 5.22.0, llama.cpp `28b812d`; 100 parçalık açgözlü üretim, `tools/gpu/generation_race.sh`'in CPU kolu; tek koşu) [ölçüldü: Colab, betik `cpu_uretim_yarisi.sh`]:

| Model | TR float32 | TR bf16 ağırlık | transformers | llama.cpp f32 | llama.cpp f16 | TR/llama.cpp f32 | TR/transformers |
| :-- | --: | --: | --: | --: | --: | --: | --: |
| SmolLM2-135M | 24,2 | 10,0 | 27,4 | 62,1 | 104,0 | 0,39 | 0,88 |
| Qwen3-0.6B | 7,8 | 3,1 | 11,4 | 15,4 | 24,5 | 0,51 | 0,68 |

Birim parça/sn. Bu makinede TR, llama.cpp f32'nin ~%40–50'si ve transformers'tan yavaş. 16 bitlik ağırlık CPU'da float32'nin 0,4 katı hızda: ağırlık her çarpımda geçici float32 tampona açılıyor (yol haritası, 2. kilometre taşı "CPU" eksikleri). 2026-10-22'nin oranlarıyla (TR/llama.cpp 0,25, TR/transformers 1,05; 48 iş parçacığı) karşılaştırmayın: makine ve iş parçacığı sayısı farklı. 48 iş parçacıklı karşılığı aşağıda, G4 koşusunda.

**Dil kıyası, CPU** (2026-11-03 17:16, main `51b75f0a`, Colab CPU High-RAM: AMD EPYC 7B12, 8 iş parçacığı; Python 3.13.18, Go 1.26.10, rustc 1.99.1 `-C opt-level=3`; `tools/bench/run.py`, 5 tekrarın en iyisi; dört dilin sonuç değerleri aynı) [ölçüldü: Colab, betik `dil_kiyasi_2026_11_03.sh`]:

| Test | TR | Python | Go | Rust | TR/Python | TR/Rust |
| :-- | --: | --: | --: | --: | --: | --: |
| `dongu` | 239 ms | 187 | 1,09 | 1,94 | 1,28 | 123 |
| `fib` | 51 | 29 | 1,18 | 0,63 | 1,76 | 81 |
| `sirala` | 104 | 133 | 32,1 | 8,4 | 0,78 | 12 |
| `metin` | 64 | 27 | 8,1 | 8,1 | 2,37 | 8 |
| `sozluk` | 217 | 102 | 79,3 | 78,4 | 2,13 | 3 |
| `matris_dongu` | 207 | 148 | 2,6 | 1,7 | 1,40 | 124 |
| `matris_yerlesik` | 6 | 6 (NumPy) | 176 | 62 | 1,00 | 0,1 |

Yorumlayıcı işinde (ilk altı test) TR, Python'dan ortalama 1,52 kat yavaş [hesap: geometrik ortalama]; en büyük fark metin (2,4) ve sözlük (2,1). Saf hesapta (döngü, çağrı, indeksleme) derlenmiş dillerden iki basamak yavaş (Rust'a göre 81–124 kat); iş bir yerleşiğe düşünce fark 3–12 kata iniyor. Matris çarpımı yerleşikte NumPy'la eşit. Aynı makine türünde 2026-11-01'de oran 1,75'ti; makine örneği başka olduğu için bu farkı hızlanma diye okumayın (Python'un süreleri de kısaldı).

**Karakter modeli eğitimi, CPU** (2026-11-03 17:0x, main `51b75f0a`, Colab CPU High-RAM: Intel Xeon 2.20 GHz, 8 iş parçacığı; torch 2.11.0+cpu, Python 3.13.18; `tools/bench/cpu_baseline.sh`, 5 tekrar) [ölçüldü: Colab, betik `cpu_temel_2026_11_03.sh`]:

| İş parçacığı | TR ms/adım | PyTorch ms/adım | TR/PyTorch |
| --: | --: | --: | --: |
| 1 | 35,69 | 33,33 | 1,07 |
| 2 | 22,10 | 22,29 | 0,99 |
| 4 | 17,45 | 15,67 | 1,11 |
| 8 | 20,03 | 17,30 | 1,16 |

Kayıp ikisinde aynı yerde (TR 2,40, PyTorch 2,38). TR, PyTorch'la başa baş ya da %7–16 yavaş [hesap]. 2026-11-01'de AMD EPYC'de TR öndeydi (1 iş parçacığında 0,82): makine başka, karşılaştırmayın. Tensör kernellerinin 39 kıyası aynı koşuda (değişim katsayısı ortanca %5,3, en kötü %12,2).

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

S-kilit satırı Kol A eşitliğinden (`9f758225`) önce ölçüldü; eşitlik kilidin yolunda değil [hipotez: Kol A yalnız `==`'nin gezintisini değiştirdi]. Eğitim adımının güncel ölçümü yukarıdaki G4 koşusu (main `51b75f0a`, Kol A dahil).

## Eski (yeniden ölçülecek)

| Ölçüm | Tarih, commit | Neden eski | Yeniden ölçüm |
| :-- | :-- | :-- | :-- |
| **L4 kart sınamasında hedef model adımı:** beş birleşmede 938, 923,5, 962, 880,5, 888 ms/adım (en büyük fark %9,3 [hesap: 962 / 880,5]; ayrı koşular, PyTorch'suz) | 2026-11-02 `4d83182f` – 2026-11-03 `6cab4f89`, Colab L4 | Kol A eşitliği (`9f758225`) sonra girdi | Bir sonraki birleşmenin kart sınamasıyla; eğitim hızının güncel ölçümü G4'te (yukarıda) |

2026-10-22'nin üretim hızları, 2026-11-01/02'nin eğitim hızı ve 2026-11-01'in dil kıyası ile karakter modeli 2026-11-03'te yeniden ölçüldü (yukarıda); eski sayılar arşivde (`docs/archive/roadmap-2026-10-27.md` Evre 5) ve Colab defterinde kalır.

Motora bağlı olmayan ölçümler (ör. Jarvis tabanı: llama.cpp ve Whisper'ın kendi süreleri, 2026-11-01) eskimez; TR'yi ölçmedikleri için bu belgede yer almazlar.

## Sıradaki ölçümler

1. GPU üretimindeki %7–10'luk gerilemenin yeri: 2026-10-22'nin commit'i ile main aynı G4 koşusunda (onay gerekir).
