# Güncel ölçümler

Bu belge yalnız motorun en son hâliyle yapılmış ölçümleri tutar; son gözden geçirme 2026-11-03, main `4e924cf9` (Kol B teslim 1'in birleşmesinden sonra). Eski bir ölçüm, motor o günden beri değiştiyse bizi yanıltabilir. Bu yüzden eski ölçümler ayrı bir listede durur ve yeniden ölçülene dek karar için kullanılmaz (Ahmet, 2026-11-03).

## Kural

- **Bir ölçüm ne zaman güncel:** commit'inden bu yana ölçtüğü yolu değiştiren bir dil işi birleşmemişse. Örneğin tensör çekirdeğine, yorumlayıcıya ya da kilide dokunan bir dilim, onlara bağlı bütün hız ölçümlerini eskitir. Kuşkuda ölçüm eski sayılır.
- **Her satırda:** tarih, commit, makine, ne ölçüldü, sonuç, karşılaştırılan ve kaynak (`tools/record` kaydı ya da Colab defteri). Commit'siz ölçüm buraya girmez.
- **Karşılaştırma aynı makinede, aynı koşuda yapılır.** İki ayrı makinenin mutlak sayıları karşılaştırılmaz; oran yazılır.
- **Colab'ın aynı adlı makinesi oturumdan oturuma değişir** (CPU High-RAM 2026-11-03'te bir kez AMD EPYC 7B12, bir kez Intel Xeon geldi): her satırda işlemcinin adı yazılır.
- **Yerel makine paylaşımlıdır:** yan oturumlar çalışırken sayılar gürültülü. Hız ölçümleri Colab'da koşulur (CPU High-RAM ya da GPU) ve sürümleri kayda yazılır.
- Bir dilim birleşince TCB bu belgeyi gözden geçirir: dilimin eskittiği satırlar "Eski" listesine taşınır.

## Güncel (2026-11-03; ölçümler main `51b75f0a`'da, gözden geçirme `4e924cf9`'da)

**Kol B teslim 1'den sonra (`7ab73591`, 2026-11-03):** `TR_fuse` her çağrıda kapanıştan okuduğu sayıları denetliyor; üretimin yolundaki dört kaynaşmış fonksiyon (`lib/layers.tr`: döner konum, iki SwiGLU kapısı, PLE karışımı) yalnız kendi parametrelerini ve yerleşikleri okuyor, kapanıştan okumuyor [kanıtlı: gövdeler okundu], yani denetim boş geçiyor. Dikkat maskesine tür denetimi eklendi. Bu yüzden aşağıdaki üretim ve eğitim satırları güncel sayılıyor [hipotez: bir sonraki G4 ölçümüyle doğrulanacak]. L4 kart sınamasında hedef model adımı `4e924cf9`'da 899 ms, `27a7b1c5`'te 916,5 ms (iki ayrı koşu; aşağıdaki satır).

| Tarih | Commit | Makine | Ne | Sonuç | Kaynak |
| :-- | :-- | :-- | :-- | :-- | :-- |
| 2026-11-04 | `ccd51eb8` (Kol A 2a) | Colab CPU High-RAM | `TR_read_lines` 1 GB, `67a2a7a8` ↔ `ccd51eb8`, altı tur, sıra dönüşümlü, ortanca | ASCII 3,86 → 4,23 sn (+%9,5), Türkçe 2,67 → 6,61 sn (+%147,5); `TR_read_csv` 100 MB fark yok. Bedel `utf8.ValidString`, kabul (18.11, TCB) | D1 `colab_2a.log` |
| 2026-11-04 | `9a36d0e1` (gerileme 2, `abf98c6d`'de birleşti) | Colab G4, RTX PRO 6000 Blackwell; Colab CPU, Xeon 2,20 GHz | Cinsli tensörlerden sonraki üretim düşüşü, `cb162b61`'e göre, sıra dönüşümlü; G4 dört, CPU sekiz tur | G4 bf16 Qwen3 363,0 → 376,3 (+%3,7), SmolLM2 572,2 → 613,5 (+%7,2); float32 286,3 → 294,8 (+%3,0), 525,6 → 555,6 (+%5,7); metin ve logit aynı; suçlu `1820069b` (yamalı ölçü: yamasız GPU'da koşmuyor). CPU yalnız SmolLM2: bf16 −%2,3 (kopya kol −%1,3), float32 −%0,2 (kopya −%0,9); bf16'nın −%1'lik kalanı kopyanın farkı içinde, sıfır olduğu ölçülmedi | Colab defteri 15:20 ve 15:47 koşuları |
| 2026-11-04 | `85be9773` (kilit düzeltmesi) | Colab CPU, Xeon 2,20 GHz | Bekleyen varken kilit bedeli, `705c16f1`'e göre, sıra dönüşümlü | Sınır −%0,2; iki görevli döngü +%1,7, çeyrekler +%0,8 / +%3,5 (kopya +%0,3); ikinci koşu +%1,4. `lockClock` 32 bayta inince döngü +%1,8: boyut sebep değil, sebep ölçülmedi | T1, `t1_kilit_hiz_cpu3`, `cpu4` |
| 2026-11-03 | `3a5114d2` (S-kilit, birleşen sürüm) | Colab L4, CPU | Uzun tensör işi sürerken başka görevin beklemesi; 100 Hz döngü | CPU: gözcü ≤ 3,5 ms 10/10, kaçan tur 0 10/10; GPU: uzun işte tur kaçıyor (status 10.2, C, Ö2) | Colab defteri 11:08 koşusu |
| 2026-11-03 | `6cab4f89` | Colab L4 | `gpt_training.tr` ve `ss_exam.tr` gerçek kartta, dolgulu dağarcık | 5,5 ms/adım (çok küçük model; yalnız çalıştığının sınaması) | Colab defteri 16:09:50 koşusu |
| 2026-11-03 | `27a7b1c5`, `4e924cf9` (Kol A ve Kol B teslim 1 dahil) | Colab L4 | Kart sınamasında hedef model adımı (PyTorch'suz) | 916,5 ve 899 ms/adım (iki ayrı koşu); Kol B teslim 2a'dan sonra 926, 864 ve 924,5 ms/adım. Koşular arası fark %7'ye çıkıyor [hesap: 926 / 864]: tek koşu hız sinyali değil, kart sınamasıdır (DENETİM istek 83 N1) | Colab defteri 17:44:12, 17:57:56, 19:14:50, 19:23:51 ve 19:31:22 koşuları |

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
| `matris_dongu` | 207 | 148 | 2,6 | 1,67 | 1,40 | 124 |
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

**Eğitim hızı, GPU** (2026-11-03 16:54, main `51b75f0a`, Colab G4: RTX PRO 6000 Blackwell, AMD EPYC 9B45 48 iş parçacığı; torch 2.11.0+cu130; hedef model rgn512, sentetik veri, 120 adım; aynı makinede TR ve derlenmiş PyTorch sırayla 3'er kez) [ölçüldü: Colab, betik `g4_olcum_2026_11_03.sh`]: TR 147, 147, 147; PyTorch 137, 136, 137 ms/adım. **TR 1,08 kat yavaş** [hesap: 147 / 136,7]. 2026-11-01'deki 145 / 136'yla (1,07) oran aynı kaldı.

**Üretim hızı, GPU ve 48 iş parçacıklı CPU** (aynı G4 koşusu, 16:57–17:03; transformers 5.22.0, llama.cpp `28b812d`; `tools/gpu/generation_race.sh` aynen; 100 parçalık açgözlü üretim; tek koşu) [ölçüldü: aynı betik]:

| Model, cihaz | TR float32 | TR bf16 ağırlık | transformers eager | transformers static + compile | llama.cpp f32 | llama.cpp f16 |
| :-- | --: | --: | --: | --: | --: | --: |
| SmolLM2-135M, GPU | 507,6 | 555,6 | 107,4 | 470,3 | 956,6 | 1079,4 |
| Qwen3-0.6B, GPU | 266,0 | 330,0 | 94,2 | 279,0 | 439,6 | 663,2 |
| SmolLM2-135M, CPU | 84,2 | 34,3 | 36,8 | — | 214,4 | 380,3 |
| Qwen3-0.6B, CPU | 25,1 | 10,3 | 11,5 | — | 50,7 | 92,0 |

Birim parça/sn. Aynı koşudaki oranlar [hesap]:
- **GPU:** TR (bf16) transformers'ın en hızlı yolunun 1,18 katı; llama.cpp f32'nin 0,58 (SmolLM2) ve 0,75 (Qwen3) katı, f16'nın 0,5 katı.
- **CPU, 48 iş parçacığı:** TR (float32) transformers'ın 2,2–2,3 katı; llama.cpp f32'nin 0,39 (SmolLM2) ve 0,50 (Qwen3) katı. 16 bitlik ağırlık burada da float32'nin 0,4 katı hızda.
- **GPU'da gerileme:** 2026-10-22'ye göre TR'nin GPU üretimi %7–10 yavaş (Qwen3 bf16 366 → 330, float32 290 → 266; SmolLM2 bf16 608 → 556, float32 548 → 508), llama.cpp aynı (%1 içinde: 442 → 440, 965 → 957). Kart türü aynı, örnek başka; llama.cpp'nin aynı kalması gerilemenin TR'de olduğunu gösteriyor. İki suçlu commit aynı koşularda bulundu, ikisi de kapandı (2026-11-04: yukarıdaki tablo ve 2026-10-23'ün satırı) [ölçüldü: Colab G4, defter]; koşular arası mutlak sayılar karşılaştırılmaz, yalnız aynı koşunun farkı geçerlidir.
- **CPU'da ilerleme:** her koşunun kendi oranıyla, 48 iş parçacıklı aynı tür makinede, TR/llama.cpp f32 2026-10-22'de Qwen3 0,25, SmolLM2 0,17; 2026-11-03'te 0,50 ve 0,39. TR/transformers 1,05 ve 0,97'den 2,18 ve 2,29'a [hesap: 12,1 / 47,5, 36 / 213, 12,1 / 11,5, 36 / 37; 2026-10-22'nin sayıları `docs/archive/roadmap-2026-10-27.md` Evre 5].

**GPU üretim gerilemesinin yeri** (2026-11-03 17:22–17:26, aynı G4, RTX PRO 6000 Blackwell; her commit kendi ikilisi ve kendi `generation_race.tr`'siyle; iki tur, turlar arası fark ≤ %1) [ölçüldü: Colab, betik `g4_gerileme_2026_11_03.sh`]:

| Commit | Ne | Qwen3 bf16 | Qwen3 float32 | SmolLM2 bf16 | SmolLM2 float32 |
| :-- | :-- | --: | --: | --: | --: |
| `1d6a2b4e` | 2026-10-22, eski ölçüm | 374,5 | 294,1–295,0 | 613,5 | 549,5 |
| `e9c64a45` | 2026-11-01, Faz 0'ın sonu | 343,6–344,8 | 274,0–275,5 | 595,2–598,8 | 537,6 |
| `d24b95ea` | 2026-11-02, T1 cinsli tensör | 336,7–339,0 | 271,0 | 561,8 | 512,8 |
| `af24785b` | 2026-11-03, S-kilit | 331,1 | 265,3–266,7 | 555,6 | 505,1–507,6 |
| `51b75f0a` | 2026-11-03, main | 331,1 | 266,0–266,7 | 555,6 | 505,1–507,6 |

Birim parça/sn. Qwen3 bf16'da toplam −%11,6; en büyük adım 2026-10-22 ile Faz 0'ın sonu arası (−%8), sonra T1'in cinsli tensörü (−%2) ve S-kilit (−%2). SmolLM2'de en büyük adım cinsli tensör (−%5,9). Main ile S-kilit aynı: 16.3 ve Kol A üretimi değiştirmedi [hesap]. 2026-10-22'nin 366 ve 608'i bu makinede 374,5 ve 613,5: kart türü aynı, ölçüm tekrarlanabilir.


**Gerilemenin içi** (2026-11-04, aynı kart türü, üç koşu, bf16, her koşuda iki tur; koşular arası fark %3,2'ye kadar, yalnız aynı koşu kıyaslanır) [ölçüldü: Colab, betikler `g4_gerileme_bol1..3.sh`]: iki düşüş var. Üçüncü koşunun tam aralıkları: `2fac1f49..3d5ba9b5` (2026-10-23) Qwen3 −%4,7, SmolLM2 −%2,7; `4080e1d4..2061d924` (cinsli tensör dalı, 2026-11-02) SmolLM2 −%6,7, Qwen3 −%3,0. İlk iki koşu aynı düşüşleri daha geniş aralıklarda gösterdi. Aynı koşuda `1d6a2b4e` → `705c16f1` Qwen3 −%9,6; yukarıdaki tablonun −%11,6'sı 2026-11-03'ün ayrı koşusu ve ucudur. Birincisinin suçlusu aşağıda; ikincisinin suçlu commit'i henüz ölçülmedi.

S-kilit satırı Kol A eşitliğinden (`9f758225`) önce ölçüldü; eşitlik kilidin yolunda değil [hipotez: Kol A yalnız `==`'nin gezintisini değiştirdi]. Eğitim adımının güncel ölçümü yukarıdaki G4 koşusu (main `51b75f0a`, Kol A dahil).

**10-23 gerilemesinin suçlusu ve düzeltmesi** (2026-11-04 12:49–12:59, Colab G4: RTX PRO 6000 Blackwell, AMD EPYC 9B45; dört tur, sırayla, her commit kendi ikilisi) [ölçüldü: betik `h1_bol_abab.sh`]: suçlu `5a348837`. `örnekle` her parçadan önce bütün logit satırını konakta denetlemeye başladı (bölüm 15'in B3 düzeltmesi); kayıp sözlük boyuyla orantılıdır.

| Commit | Ne | Qwen3 bf16 | Qwen3 float32 | SmolLM2 bf16 | SmolLM2 float32 |
| :-- | :-- | --: | --: | --: | --: |
| `2fac1f49` | 10-23 09:58 | 373,9 | | 629,9 | |
| `3532fda4` | suçlunun ebeveyni | 373,5 | | 628,0 | |
| `5a348837` | konak denetimi | 358,1 | | 614,5 | |
| `92aaaae2` | | 354,9 | | 610,7 | |
| `3d5ba9b5` | | 355,2 | | 610,7 | |
| `705c16f1` | main, düzeltmeden önce | 343,6 | 273,4 | 571,4 | 517,5 |
| `eab6965c` | denetim ve seçim tek geçişte | 357,8 | 283,9 | 586,5 | 528,4 |
| `a80b1f69` | açgözlü seçimde sözlük boyunda ayırma yok | 368,3 | 290,5 | 582,2 | 533,3 |

Birim parça/sn. Aynı koşuda main'e göre Qwen3 bf16 +%7,2, float32 +%6,3; SmolLM2 bf16 +%1,9, float32 +%3,1 [hesap]. Taramanın süresi aynı işlemcide Go ile ölçüldü: 151936 logitte seçim 92 µs, denetim ve seçim 218 µs, tek geçiş 92 µs; 49152 logitte 30, 71 ve 30 µs. Düzeltme metni ve logitleri değiştirmedi (iki model, bf16 ve float32, GPU ve CPU). CPU'da yavaşlama yok: Colab CPU High-RAM, Intel Xeon 2.20 GHz, dört tur, Qwen3 float32 5,8 → 5,9, SmolLM2 bf16 10,8 → 10,8 [ölçüldü: betik `h1_cpu_abab2.sh`].

## Eski (yeniden ölçülecek)

Şu an eski satır yok.

2026-10-22'nin üretim hızları, 2026-11-01/02'nin eğitim hızı, 2026-11-01'in dil kıyası ile karakter modeli ve L4 kart sınamasının 2026-11-02/03'teki beş adımı (938, 923,5, 962, 880,5, 888 ms; Kol A eşitliğinden önce) 2026-11-03'te yeniden ölçüldü (yukarıda); eski sayılar arşivde (`docs/archive/roadmap-2026-10-27.md` Evre 5) ve Colab defterinde kalır.

Motora bağlı olmayan ölçümler (ör. Jarvis tabanı: llama.cpp ve Whisper'ın kendi süreleri, 2026-11-01) eskimez; TR'yi ölçmedikleri için bu belgede yer almazlar.

## Sıradaki ölçümler

1. GPU üretiminin 2026-11-02 gerilemesi (`4080e1d4..2061d924`, cinsli tensör dalı): suçlu commit ve düzeltmeden önce ve sonra aynı koşuda ABAB (hız kolu).
2. Kol B teslim 1'den sonra üretim ve eğitimin G4'te yeniden ölçümü: yukarıdaki "güncel sayılıyor" varsayımını doğrular (G4 onayla; 1. ile aynı koşuda).
