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

L4 adım süresinin dört birleşmedeki değerleri: 938, 923,5, 962, 880,5, 888 ms [ölçüldü: Colab defteri]. En büyük fark %9,3 [hesap: 962 / 880,5]; aynı koşuda karşılaştırılmadığı için bunu hızlanma ya da yavaşlama diye okumayın. Bu satırlarda PyTorch ile karşılaştırma yok.

## Eski (yeniden ölçülecek)

| Ölçüm | Tarih, commit | Neden eski | Yeniden ölçüm |
| :-- | :-- | :-- | :-- |
| **Üretim hızı, CPU:** Qwen3-0.6B'de TR 12,1, llama.cpp f32 47,5, transformers 11,5 parça/sn (llama.cpp'nin ~1/5'i) | 2026-10-22, Colab, 48 iş parçacığı | Cinsli tensör, kilit, KV önbelleği ve çözme o günden sonra değişti | Koşuyor: CPU High-RAM, 2026-11-03 16:4x, `tools/gpu/generation_race.sh`'in CPU kolu |
| **Üretim hızı, GPU:** TR llama.cpp f32'nin %63–83'ü, transformers'ın en hızlı yolundan hızlı | 2026-10-22, Colab | Aynı | GPU makinesinde aynı betik (onay gerekir) |
| **Eğitim hızı:** hedef modelde TR 145, derlenmiş PyTorch 136 ms/adım (1,07 kat) | 2026-11-01, `e9c64a45`, Colab G4 | T1'in iki dilimi ve S-kilit bu yola dokundu | G4'te `tools/bench` (onay gerekir) |
| **Dil kıyası ve CPU karakter modeli** (`tools/bench/README.md` "Güncel taban") | 2026-11-01, `6d21ed09`, Colab CPU High-RAM | Yorumlayıcı (Kol A eşitlik, 13.1) ve tensör çekirdeği değişti | CPU High-RAM'de `tools/bench/cpu_baseline.sh` |

Motora bağlı olmayan ölçümler (ör. Jarvis tabanı: llama.cpp ve Whisper'ın kendi süreleri, 2026-11-01) eskimez; TR'yi ölçmedikleri için bu belgede yer almazlar.

## Sıradaki ölçümler

1. Üretim hızı, CPU: koşuyor (CPU High-RAM, 8 iş parçacığı; aynı makinede TR, transformers ve llama.cpp).
2. Dil kıyası ve karakter modeli, CPU: CPU High-RAM.
3. Üretim hızı ve eğitim hızı, GPU: GPU makinesi gerekiyor, onay gerekir.
