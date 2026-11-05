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

**CPU'da üretim, 16 bit** (2026-11-04, `a84b2628` ↔ `23a2bf12`, Colab CPU High-RAM: AMD EPYC 7B12, 8 iş parçacığı, dört tur, 30 parça, kopya kollu; parça/sn):

| Model | Tür | Önce | Sonra |
| :-- | :-- | --: | --: |
| Qwen3 | bf16 | 3,8 | 12,0 |
| Qwen3 | fp16 | 1,4 | 10,6 |
| Qwen3 | float32 | 8,2 | 8,1 |
| SmolLM2 | bf16 | 10,9 | 36,8 |
| SmolLM2 | fp16 | 4,8 | 33,4 |
| SmolLM2 | float32 | 24,3 | 25,4 |

Logit, metin ve eğitim (4 tür, 3 kip, 300 adım) tabanla bit bit aynı; Qwen3 bf16 tepe bellek 12,0 → 9,6 GB. DENETİM ayrı makinede sentetik modelle 2,74 → 13,83, tepe 17,95 → 9,89 GB [ölçüldü: istek 121, Windows]. llama.cpp ve transformers kıyası aşağıda.

**GGUF kıyası** (2026-11-05, TR `fbb5a77d`, llama.cpp `8bed7cfb` `b12124`, BLAS yok, Colab CPU: Intel Xeon 2.20 GHz, 8 iş parçacığı; aynı koşu, kopya kollu, `llama-bench -r 5`; ortanca, parantezde üçüncü çeyrek) [ölçüldü: Colab, betik `h2k_kosu2.sh`, kayıt `olcumler/h2k2`]:

| Model | İş | TR | llama.cpp | Oran |
| :-- | :-- | --: | --: | --: |
| Qwen3-0.6B `Q8_0` | üretim | 15,57 (16,12) | 41,22 (42,79) | 0,378 |
| Qwen3-0.6B `Q8_0` | prompt 512 | 80,40 (80,77) | 145,57 (149,72) | 0,552 |
| SmolLM2-360M `Q8_0` | üretim | 20,75 (21,35) | 65,91 (67,45) | 0,315 |
| SmolLM2-360M `Q8_0` | prompt 512 | 111,20 (112,47) | 208,80 (210,17) | 0,533 |
| Qwen3-4B `Q4_K_M` | üretim | 2,35 (2,39) | 8,45 (8,69) | 0,278 |
| Qwen3-4B `Q4_K_M` | prompt 512 | 14,53 (14,57) | 28,38 (28,78) | 0,512 |

Birim parça/sn. llama.cpp'nin beş örneğinin biri çoğu turda iki kat yavaş; `-p 0` ile ayrı koşuda oran 0,333.

**GGUF kartta** (2026-11-05, G4, NVIDIA RTX PRO 6000 Blackwell Server Edition, sm_120, 188 SM; TR `66f6d21e`, llama.cpp `8bed7cfb`, `llama-bench -ngl 99 -r 5 -p 512 -n 100 -o json`; aynı koşu, dört tur, kopya kollu; parça/sn, ortanca, parantezde üçüncü çeyrek) [ölçüldü: H1, `h1g3/ozet.txt`]:

| Model | İş | TR | llama.cpp | Oran |
| :-- | :-- | --: | --: | --: |
| Qwen3-0.6B `Q8_0` | üretim, 2–101. parça | 403,3 (413,7) | 783,1 (783,2) | 0,515 |
| Qwen3-0.6B `Q8_0` | üretim, 102–201. parça | 307,8 (313,5) | 783,1 (783,2) | 0,393 |
| Qwen3-0.6B `Q8_0` | prompt 512 | 22 309,5 (22 358,1) | 56 545,8 (56 741,5) | 0,395 |
| Qwen3-4B `Q4_K_M` | üretim, 2–101. parça | 216,9 (222,7) | 346,6 (346,8) | 0,626 |
| Qwen3-4B `Q4_K_M` | üretim, 102–201. parça | 175,9 (179,3) | 346,6 (346,8) | 0,507 |
| Qwen3-4B `Q4_K_M` | prompt 512 | 4 758,4 (4 762,8) | 20 402,8 (20 433,4) | 0,233 |

Kartın ölçülen bandı 1457 GB/sn. Tek satır çekirdeğinde 151936'lı kafa şekilleri banda ulaşıyor (soğukta 1314–1485 GB/sn); FFN şekilleri 869–1151 GB/sn, bandın %60–79'u. `q4_k`'nın 1,8 MB'lık iki şekli soğukta dörtte birin altında (287, 339 GB/sn): 4,6 µs çağrı tabanı yüzünden, ağırlık 2,2 MB'tan küçükse eşik tutmuyor [hesap]. Tepe kart belleği dosyanın 1,08–1,19 katı, kısa prompt [ölçüldü: H1, `h1g1`].

Üretimin ayrışımı, Qwen3-0.6B `Q8_0` (TR `845dda66`, aynı işlemci türü, ayrı oturum; üç kol aynı koşuda, altı tur) [ölçüldü: kayıt `olcumler/h2k4`]: TR üretim 63,5 ms/parça, yalnız 197 matrisin çarpımı 33,9, llama.cpp bütün iş 21,2. Çarpım dışı 29,6 ms [hesap]: dikkat yaklaşık 10, kaynaştırılmış işlemler yaklaşık 9, kalan yaklaşık 9 [hesap: profil].

Tepe bellek, üretim 2 parça (TR `3fdde953`, AMD EPYC 7B12) [ölçüldü: kayıt `olcumler/h2k3`]: Qwen3-0.6B `Q8_0` dosyanın 1,67 katı, SmolLM2-360M `Q8_0` 1,71, Qwen3-4B `Q4_K_M` 1,31; yükleme 423, 239, 1 264 ms.

**CPU kıyası** (2026-11-05 07:54–08:47 TSS, main `8a3925c9`, Colab CPU High-RAM: AMD EPYC 7B12, 8 iş parçacığı; torch 2.15.0+cpu, transformers 5.22.1 float32, llama.cpp `8bed7cfb`; dört tur, kopya kollu, kol başına 16 örnek; ortanca, parantezde birinci ve üçüncü çeyrek) [ölçüldü: Colab, betik `h1k_kosu2.sh`, kayıt `olcumler/h1k2`]:

Üretim, parça/sn:

| Model | TR float32 | TR bf16 | transformers | llama.cpp f32 | llama.cpp f16 |
| :-- | --: | --: | --: | --: | --: |
| SmolLM2-135M | 26,8 (24,1–27,8) | 39,3 (29,0–40,4) | 13,9 (12,8–14,9) | 73,6 (42,2–80,8) | 130,4 (110,2–135,3) |
| Qwen3-0.6B | 8,7 (7,6–8,8) | 13,7 (13,0–13,9) | 5,3 (5,2–5,5) | 19,7 (17,2–22,0) | 33,7 (25,1–39,1) |

Prompt işleme, 512 parça, parça/sn:

| Model | TR float32 | TR bf16 | transformers | llama.cpp f32 | llama.cpp f16 |
| :-- | --: | --: | --: | --: | --: |
| SmolLM2-135M | 403 (372–413) | 378 (326–403) | 755 (535–805) | 788 (620–852) | 1014 (596–1065) |
| Qwen3-0.6B | 132 (118–133) | 127 (121–130) | 243 (212–250) | 214 (175–224) | 282 (248–304) |

Nicemli taban, yalnız llama.cpp, parça/sn:

| Model, dosya | Üretim | Prompt işleme |
| :-- | --: | --: |
| Qwen3-0.6B Q8_0, `Qwen3-0.6B-Q8_0.gguf` | 50,7 (42,7–52,4) | 215 (203–225) |
| SmolLM2-360M Q8_0, `smollm2-360m-instruct-q8_0.gguf` | 79,1 (59,0–85,3) | 322 (272–336) |
| Qwen3-4B Q4_K_M, `Qwen3-4B-Q4_K_M.gguf` | 12,5 (11,6–13,8) | 45 (44–47) |

Oranlar, ortancaların oranı, parantezde üçüncü çeyreklerin oranı [hesap: aynı tablolar]:

| Oran | SmolLM2 üretim | Qwen3 üretim | SmolLM2 prompt | Qwen3 prompt |
| :-- | --: | --: | --: | --: |
| TR bf16 / TR float32 | 1,47 (1,46) | 1,57 (1,58) | 0,94 (0,97) | 0,96 (0,97) |
| TR float32 / llama.cpp f32 | 0,36 (0,34) | 0,44 (0,40) | 0,51 (0,48) | 0,62 (0,60) |
| TR bf16 / llama.cpp f16 | 0,30 (0,30) | 0,41 (0,36) | 0,37 (0,38) | 0,45 (0,43) |
| TR float32 / transformers | 1,92 (1,86) | 1,64 (1,60) | 0,53 (0,51) | 0,54 (0,53) |
| TR bf16 / transformers | 2,82 (2,71) | 2,58 (2,52) | 0,50 (0,50) | 0,52 (0,52) |

**Hazır model yükleme** (2026-11-04 20:05–20:11, `5b554d72` ↔ `da03aa62`, Colab CPU High-RAM: AMD EPYC 7B12, 8 iş parçacığı, 50 GB; altı tur, kol sırası dönüşümlü, ilk tur atıldı, ortanca; her koşu ayrı süreç, dosyalar diskin önbelleğinde). "kur" `TR_build_pretrained_model`, "oku" `TR_load_tensors`, "eşle" `TR_map_hugging_face`, "yükle" `TR_load_parameters`; süre ms, tepe GB. "taban" model kurmadan yalnız oku ve eşle:

| Model, tür | Kol | kur | oku | eşle | yükle | toplam | tepe |
| :-- | :-- | --: | --: | --: | --: | --: | --: |
| Qwen3 0.6B bf16 | önce | 6635 | 3572 | 1105 | 1048 | 12476 | 8,99 |
| | sonra | 63 | 1309 | 756 | 326 | 2442 | 4,60 |
| | taban | | 1299 | 750 | | 2053 | 3,06 |
| Qwen3 0.6B float32 | önce | 6476 | 3535 | 1113 | 464 | 11599 | 10,03 |
| | sonra | 63 | 1288 | 757 | 601 | 2701 | 5,88 |
| | taban | | 1320 | 804 | | 2142 | 3,05 |
| SmolLM2 135M bf16 | önce | 1566 | 622 | 256 | 211 | 2644 | 1,86 |
| | sonra | 14 | 277 | 204 | 60 | 551 | 1,10 |
| | taban | | 275 | 197 | | 471 | 0,62 |
| SmolLM2 135M float32 | önce | 1450 | 646 | 250 | 77 | 2429 | 2,11 |
| | sonra | 14 | 283 | 227 | 127 | 654 | 1,43 |
| | taban | | 279 | 202 | | 479 | 0,63 |

Aynı günkü iki koşu Intel Xeon 2.20 GHz'deydi (`önce` Qwen3 bf16 17,8 ve 19,8 sn): kıyas yalnız aynı koşunun içinde geçerlidir. Modelin kendisi taban üstüne tam bir kopya ekliyor (Qwen3 bf16 1,54 GB, dosya 1,50 GB) [hesap]. Yükleme sonrası üretim (Qwen3 ve SmolLM2, float32 ve bf16, açgözlü ve örneklemeli, tohumsuz `TR_randn`) önce ve sonra bayt bayt aynı [ölçüldü: Windows, `cmp`]. Kaynak: `t1_yukleme3_20261104_2020.txt`.

**Üretim hızı, CPU** (2026-11-03 16:43–16:55, main `51b75f0a`, Colab CPU High-RAM: Intel Xeon 2.20 GHz, 8 iş parçacığı; torch 2.11.0+cpu, transformers 5.22.0, llama.cpp `28b812d`; 100 parçalık açgözlü üretim, `tools/gpu/generation_race.sh`'in CPU kolu; tek koşu) [ölçüldü: Colab, betik `cpu_uretim_yarisi.sh`]:

| Model | TR float32 | TR bf16 ağırlık | transformers | llama.cpp f32 | llama.cpp f16 | TR/llama.cpp f32 | TR/transformers |
| :-- | --: | --: | --: | --: | --: | --: | --: |
| SmolLM2-135M | 24,2 | 10,0 | 27,4 | 62,1 | 104,0 | 0,39 | 0,88 |
| Qwen3-0.6B | 7,8 | 3,1 | 11,4 | 15,4 | 24,5 | 0,51 | 0,68 |

Birim parça/sn. Bu makinede TR, llama.cpp f32'nin ~%40–50'si ve transformers'tan yavaş. 16 bitlik ağırlık o gün CPU'da float32'nin 0,4 katı hızdaydı: ağırlık her çarpımda geçici float32 tampona açılıyordu; tek satırlık çarpımda kapandı, yukarıdaki "CPU'da üretim, 16 bit" tablosu. 2026-10-22'nin oranlarıyla (TR/llama.cpp 0,25, TR/transformers 1,05; 48 iş parçacığı) karşılaştırmayın: makine ve iş parçacığı sayısı farklı. 48 iş parçacıklı karşılığı aşağıda, G4 koşusunda.

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
- **CPU, 48 iş parçacığı:** TR (float32) transformers'ın 2,2–2,3 katı; llama.cpp f32'nin 0,39 (SmolLM2) ve 0,50 (Qwen3) katı. 16 bitlik ağırlık burada da o gün float32'nin 0,4 katı hızdaydı (Eski).
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

**Tek algoritmanın bedeli** (2026-11-04 22:02–22:12, Colab CPU: Intel Xeon 2,20 GHz, 2 iş parçacığı; 15 tur, iki ikili sırayla, ortanca) [ölçüldü: `e78e38af` ve `8e58eb05`, Colab `t1_bedel_220216`]:

| Ne | Önce | Sonra | Fark |
| :-- | --: | --: | --: |
| `TR_exp`, 5 milyon çağrılık döngü | 2230 ms | 2285 ms | +%2,5 |
| `TR_tanh`, 5 milyon çağrılık döngü | 2151 ms | 2139 ms | −%0,6 |
| `x ** 1.5`, 5 milyon işlemlik döngü | 1829 ms | 1950 ms | +%6,6 |
| `TR_sample`, top-k'sız 151669 aday, 300 çağrı | 1865 ms | 2363 ms | +%26,7 |

Çağrı başına `TR_exp` +11 ns, `**` +24 ns, `TR_sample` +1,66 ms [hesap]; `TR_sample`'ın süresinde `.TR_tolist` de var. Yorumlayıcının kendi yükü farkı küçültüyor. `detmath`'in tek çağrısı (Colab AMD EPYC, yedi tur, `6a305ee5`): `Exp` 11,93 → 19,38 ns (+%62), `Sinh` +%47, `Cosh` +%57, `Tanh` +%40, `Log` +%16, `Pow` −%3 [ölçüldü: T1].

## Eski (yeniden ölçülecek)

- 2026-11-03'ün CPU kıyas tabloları (Xeon, 8 iş parçacığı; EPYC 9B45, 48 iş parçacığı): 16 bitlik çarpımdan önce; bu makinelerde yeniden ölçülmedi. Yeni kıyas başka makinede ("CPU kıyası", AMD EPYC 7B12), oranlar karşılaştırılmaz: Xeon'da TR transformers'tan yavaştı.

2026-10-22'nin üretim hızları, 2026-11-01/02'nin eğitim hızı, 2026-11-01'in dil kıyası ile karakter modeli ve L4 kart sınamasının 2026-11-02/03'teki beş adımı (938, 923,5, 962, 880,5, 888 ms; Kol A eşitliğinden önce) 2026-11-03'te yeniden ölçüldü (yukarıda); eski sayılar arşivde (`docs/archive/roadmap-2026-10-27.md` Evre 5) ve Colab defterinde kalır.

Motora bağlı olmayan ölçümler (ör. Jarvis tabanı: llama.cpp ve Whisper'ın kendi süreleri, 2026-11-01) eskimez; TR'yi ölçmedikleri için bu belgede yer almazlar.

## Sıradaki ölçümler

- Xeon'da CPU kıyası: aynı betik (`h1k_kosu2.sh`); TR ile transformers'ın yönü işlemciye bağlı, sebep ölçülmedi. transformers bf16 ölçülmedi.

1. GPU üretiminin 2026-11-02 gerilemesi (`4080e1d4..2061d924`, cinsli tensör dalı): suçlu commit ve düzeltmeden önce ve sonra aynı koşuda ABAB (hız kolu).
2. Kol B teslim 1'den sonra üretim ve eğitimin G4'te yeniden ölçümü: yukarıdaki "güncel sayılıyor" varsayımını doğrular (G4 onayla; 1. ile aynı koşuda).
