# BİTTİ

# DURUM — TestBuildScalesLinearly: gürültü mü, gerileme mi?

- Başlama: 2026-10-29 17:29 UTC
- Bitiş: 2026-10-29 18:15 UTC
- Dal: `claude/keen-volta-zqw48d` (taban `main` 6cd1e49)
- Makine: Linux konteyner, 4 çekirdek, go1.26.10

## Adımlar
1. [x] Ölçüm: 1073aeab, 48bd054d, main × 15 koşu
2. [x] Karar
3. [x] Düzeltme: 5 deneme + GC + dönüşümlü ölçüm (5 deneme + GC tek başına yetmedi)
4. [x] Karesel sınama (internal/testkit)
5. [x] Değişiklikten sonra main'de 15 koşu: hiçbiri düşmedi

## Notlar
- `1073aeab..main` arasında `internal/devtools/codeindex` ve `internal/testkit`'e commit yok: test ve ölçüm yöntemi üç commit'te aynı, yalnız ölçülen kod (sözcük çözümleyici, ayrıştırıcı) değişiyor. [Kesin: `git log 1073aeab..main -- internal/devtools/codeindex internal/testkit` boş]
- Codegraph `run:` satırları dokunmadan önce koşuldu: hepsi geçti.

## Ölçüm 1 — üç commit × 15 koşu (2026-10-29 17:40 UTC)
Komut her koşuda: `go test ./internal/devtools/codeindex/ -run TestBuildScalesLinearly -count=1 -v`; her commit ayrı `git worktree`'de.
Koşular commit'ler arasında dönüşümlü (1073aeab, 48bd054d, main, 1073aeab, …): makine yükündeki kayma tek bir commit'e yığılmasın.
Makine boştaydı (başka iş yok). `main` = 6cd1e49.

| commit | koşu | düşen | ölçüm | en küçük | ortanca | en büyük | bütün oranlar |
|---|---|---|---|---|---|---|---|
| 1073aeab | 15 | 0 | kurulum | 3.5 | 4.4 | 5.0 | 5.0 4.3 3.6 3.9 4.7 5.0 3.9 4.4 3.5 4.6 4.1 4.5 4.5 4.6 4.4 |
| 1073aeab | 15 | 0 | önbellek | 2.8 | 4.3 | 4.8 | 3.8 4.0 4.6 4.8 4.7 4.1 4.5 4.0 4.4 4.6 4.4 3.8 4.0 2.8 4.3 |
| 1073aeab | 15 | 0 | yenileme | 3.8 | 5.1 | 5.8 | 5.2 5.7 5.3 5.0 5.6 4.8 5.0 3.8 5.1 5.4 5.2 4.6 5.8 4.9 4.9 |
| 48bd054d | 15 | 0 | kurulum | 3.4 | 4.1 | 5.0 | 3.8 4.2 4.2 4.1 4.5 3.9 4.6 3.4 4.0 4.6 4.1 4.1 3.7 5.0 4.0 |
| 48bd054d | 15 | 0 | önbellek | 4.0 | 4.4 | 4.9 | 4.4 4.0 4.2 4.0 4.2 4.5 4.1 4.7 4.3 4.2 4.7 4.7 4.9 4.7 4.8 |
| 48bd054d | 15 | 0 | yenileme | 4.1 | 4.8 | 5.5 | 5.5 5.0 4.9 4.1 4.7 5.1 4.5 5.0 4.8 5.2 5.2 4.5 4.8 4.3 4.6 |
| main | 15 | 0 | kurulum | 3.5 | 4.2 | 4.5 | 4.1 4.3 4.5 4.5 3.7 4.2 4.3 4.0 4.5 4.0 4.3 3.5 3.6 3.9 4.5 |
| main | 15 | 0 | önbellek | 3.5 | 4.5 | 5.4 | 5.0 3.9 4.3 4.6 4.6 4.4 5.0 3.9 5.4 4.5 4.1 3.5 4.8 4.4 4.7 |
| main | 15 | 0 | yenileme | 3.9 | 4.5 | 5.2 | 4.1 4.0 4.2 4.1 4.6 4.4 5.1 5.1 5.2 3.9 4.9 5.1 4.2 4.5 4.6 |

## Karar: gürültü [Kesin, karar kuralına göre]
Yenileme oranının ortancası: 1073aeab **5,1**, 48bd054d **4,8**, main **4,5**. Kural "1073aeab'dekinden 1,5'ten fazla büyükse gerileme": 48bd054d −0,3, main −0,6. Gerileme yok; 45 koşunun hiçbiri düşmedi, en büyük oran 5,8 (1073aeab). Kurulum ve önbellek ortancaları da üç commit'te 4,1–4,5 aralığında.

Gerekçe ve sınırları:
- CI'daki 8,4 ve 9,1 burada hiç görülmedi; aynı kod CI'da sonraki koşularda da geçti. Fark kodda değil koşum ortamında. [Kesin: aynı test, aynı kod, 45 koşu]
- CI'da test tam denetimin içinde koşar (`tools/full_check.sh`: `go test` bütün paketleri aynı anda derleyip koşar). n ve 4n art arda ölçülür ama aynı anda değil; 4n'in üç denemesinin hepsi bir yük patlamasına denk gelirse oran şişer. 3 denemenin en iyisi bu patlama kısa değilse onu ayıklamaz. [Muhtemel: CI ortamını burada kurmadım, yük altında ölçüm aşağıda]
- Yenileme oranı boş makinede de 4'ün biraz üstünde (ortanca 4,5–5,1, kurulumda 4,1–4,4). Bu, üç commit'te de var, yani 0.6a'dan gelmiyor; yenilemede küçük bir doğrusal-üstü pay (ör. 4n'de büyüyen yığının çöp toplama payı) olabilir. Sınırdan uzak (8), karesel değil (16). [Tahmin: profili çıkarılmadı]

## Ölçüm 2 — yük altında main, eski yöntem, 15 koşu (17:47 UTC)
CI'daki tam denetime benzer yük: arka planda durmadan `go test -count=1 ./internal/lang/... ./internal/stdlib/...` (scratchpad/load.sh).
| commit | koşu | düşen | ölçüm | en küçük | ortanca | en büyük | bütün oranlar |
|---|---|---|---|---|---|---|---|
| main, eski yöntem (3 deneme) | 15 | 1 | kurulum | 2.8 | 4.3 | 5.4 | 3.3 3.9 4.4 4.3 4.0 5.2 4.3 5.0 4.2 3.4 4.3 3.6 5.0 2.8 5.4 |
| main, eski yöntem (3 deneme) | 15 | 1 | önbellek | 3.4 | 4.5 | 8.3 | 5.1 4.1 4.6 5.1 4.6 8.3 4.5 3.7 3.4 4.0 5.0 4.3 4.1 4.3 5.4 |
| main, eski yöntem (3 deneme) | 15 | 1 | yenileme | 2.9 | 4.2 | 6.0 | 6.0 4.2 2.9 3.7 4.0 3.2 4.1 3.5 5.2 3.9 4.9 5.6 4.9 4.9 5.1 |

Yükte 15 koşudan 1'i düştü (önbellek 8,3): CI'daki tür düşüş burada üretildi. [Kesin]

## Ölçüm 3 — aynı yükte eski yöntem ↔ 5 deneme + GC, 30'ar koşu, dönüşümlü (17:39 UTC)
| commit | koşu | düşen | ölçüm | en küçük | ortanca | en büyük | bütün oranlar |
|---|---|---|---|---|---|---|---|
| main + 5 deneme ve GC (92b9aa0) | 30 | 1 | kurulum | 2.5 | 4.3 | 5.6 | 4.3 4.5 2.7 3.5 4.7 4.8 4.1 3.9 4.3 3.3 3.7 4.4 5.3 4.6 4.3 2.5 4.9 4.4 3.0 4.2 5.6 4.3 4.4 4.0 4.6 4.3 4.5 3.8 3.6 4.4 |
| main + 5 deneme ve GC (92b9aa0) | 30 | 1 | önbellek | 3.2 | 4.5 | 8.5 | 8.5 6.3 4.0 4.2 4.5 3.4 4.8 6.2 4.2 5.4 4.1 4.2 5.4 3.2 5.1 4.1 4.7 3.9 4.5 5.4 3.3 4.6 4.3 4.6 5.7 5.0 5.9 5.2 3.5 4.0 |
| main + 5 deneme ve GC (92b9aa0) | 30 | 1 | yenileme | 2.6 | 3.8 | 5.2 | 2.7 2.6 4.2 5.2 3.7 4.7 4.6 3.1 5.0 3.9 3.2 3.7 3.9 4.9 3.6 5.2 3.5 3.7 3.5 3.6 4.0 3.6 3.8 2.7 2.9 4.0 4.4 2.7 4.7 4.2 |
| main, eski yöntem (3 deneme) | 30 | 0 | kurulum | 2.8 | 4.4 | 6.2 | 5.1 4.0 3.5 4.3 3.6 4.6 4.2 3.8 3.9 4.9 4.6 4.6 4.5 5.1 4.2 6.2 2.8 4.4 5.0 3.6 4.1 5.5 3.7 3.4 5.0 4.6 4.5 4.4 4.8 3.8 |
| main, eski yöntem (3 deneme) | 30 | 0 | önbellek | 3.2 | 4.7 | 6.4 | 4.6 4.7 5.8 3.4 4.7 3.3 5.0 5.2 3.8 5.3 5.1 4.3 5.7 3.4 4.6 5.8 5.3 4.3 5.1 6.0 6.4 4.0 4.6 4.5 4.7 3.2 6.1 4.4 5.8 5.4 |
| main, eski yöntem (3 deneme) | 30 | 0 | yenileme | 2.7 | 4.3 | 5.3 | 5.1 3.8 3.0 5.3 5.3 3.3 3.4 3.8 4.3 4.5 4.4 4.5 5.3 3.8 4.3 4.1 4.1 4.0 3.4 4.5 4.9 2.7 4.3 4.7 4.3 5.1 4.1 4.3 4.5 3.4 |

**5 deneme + GC düşüşü gidermedi** [Kesin, bu ölçümde]: yeni yöntem 30'da 1 düştü (önbellek 8,5), eski yöntem 30'da 0 (ölçüm 2 ile birlikte 45'te 1). Düşen koşu benim `dups` ve commit'imle aynı saniyeye denk geldi (ek patlama), ama tam da CI'daki durum bu. Düşen koşuda n=100 19,7 ms (normal), n=400 167 ms (normali ~95 ms): 4n'in beş denemesinin hepsi patlamanın içindeydi. n ve 4n art arda ölçüldükçe, 4n'in bütün denemelerini kaplayan (~1 sn) bir patlamayı en-iyi-N ayıklayamaz; deneme sayısını artırmak yalnız patlamanın kısa olduğu durumu kurtarır.

Bu yüzden görevin tarifinden bir adım öteye gittim: **denemeler dönüşümlü** (n, 4n, n, 4n, …). Bir patlama iki boyun denemesine de düşer, en kısa süreler aynı yükü görür. Sınır 8 kalır, yeniden deneme yok; karesel işin oranı değişmez (karesel sınama geçiyor). Bu, "yöntemi gevşetme" yasağına girmiyor diye düşünüyorum: ölçüt aynı, yalnız ölçümün yük eşitliği sağlanıyor (işlevin belgesi zaten "aynı makine yükünde ölçülür" diyordu, art arda ölçüm bunu sağlamıyordu). [Muhtemel] Ana oturum uygun görmezse dönüşümlü ölçüm tek commit'tir, geri alınabilir.

## Ölçüm 4 — ağır yükte üç yöntem, 30'ar koşu, dönüşümlü (17:47–18:02 UTC)
İki yük döngüsü aynı anda (yük ortalaması 10–14, 4 çekirdek; CI'da `go test` 4 paketi aynı anda koşar). Yöntemler: eski (3 deneme, art arda), 92b9aa0 (5 deneme + GC, art arda), 5d342f4 (5 deneme + GC, dönüşümlü). Ölçülen kod üçünde de main.
| commit | koşu | düşen | ölçüm | en küçük | ortanca | en büyük | bütün oranlar |
|---|---|---|---|---|---|---|---|
| 5 deneme + GC + dönüşümlü | 30 | 0 | kurulum | 2.8 | 4.5 | 6.0 | 4.4 4.6 5.4 5.6 4.6 5.0 4.2 3.0 4.3 4.5 4.2 3.9 4.8 4.8 4.7 2.8 6.0 5.3 3.5 5.3 5.5 5.5 4.0 5.2 5.0 3.1 4.3 4.4 3.9 4.2 |
| 5 deneme + GC + dönüşümlü | 30 | 0 | önbellek | 2.8 | 4.2 | 6.7 | 4.9 4.2 5.2 4.0 5.3 5.9 3.7 3.4 6.7 3.9 4.9 6.2 4.4 3.2 4.2 4.3 3.8 3.6 4.6 3.2 3.2 5.7 2.8 3.3 3.7 4.4 5.4 3.3 4.2 3.7 |
| 5 deneme + GC + dönüşümlü | 30 | 0 | yenileme | 3.6 | 4.5 | 6.0 | 4.9 5.0 5.6 3.8 5.4 5.1 3.9 5.7 4.1 4.7 5.1 3.8 5.0 4.1 4.2 5.1 5.2 5.2 3.6 4.1 4.6 6.0 4.3 4.5 4.1 3.9 4.4 6.0 3.7 4.3 |
| 5 deneme + GC | 30 | 1 | kurulum | 2.2 | 4.4 | 6.0 | 4.9 2.7 3.1 5.0 5.8 5.2 4.5 4.7 4.5 4.3 3.5 3.2 4.9 6.0 5.6 3.7 4.1 4.5 2.2 3.6 4.3 2.9 5.9 3.6 3.5 3.4 5.4 4.8 4.8 4.3 |
| 5 deneme + GC | 30 | 1 | önbellek | 1.7 | 4.5 | 8.4 | 5.7 6.4 3.9 5.0 7.2 3.5 4.5 3.6 4.6 5.6 8.4 2.5 3.6 4.5 3.4 2.9 4.3 4.5 5.6 4.6 6.3 5.6 3.4 5.0 6.3 3.3 6.2 4.4 5.2 1.7 |
| 5 deneme + GC | 30 | 1 | yenileme | 2.4 | 4.3 | 5.9 | 4.2 4.4 4.4 4.9 4.0 5.9 5.9 3.1 5.9 4.0 4.7 4.8 3.8 5.6 2.4 4.9 2.9 4.0 2.7 2.6 3.2 4.8 3.9 5.6 2.8 5.4 4.3 5.1 3.5 4.6 |
| eski: 3 deneme | 30 | 4 | kurulum | 2.0 | 3.8 | 6.5 | 4.2 6.5 2.8 3.6 5.1 3.6 5.2 4.9 4.7 4.0 4.2 3.8 2.7 3.9 4.3 2.7 2.8 3.8 2.7 2.0 3.2 5.3 3.1 4.5 4.5 3.6 4.7 5.5 2.7 3.2 |
| eski: 3 deneme | 30 | 4 | önbellek | 2.9 | 5.0 | 10.8 | 7.0 6.7 5.7 10.8 3.3 6.0 5.8 4.2 5.9 6.6 4.4 4.8 4.5 9.3 4.8 3.8 5.9 5.4 4.3 5.2 6.6 2.9 3.1 5.8 2.9 6.4 4.6 4.2 3.0 4.4 |
| eski: 3 deneme | 30 | 4 | yenileme | 2.1 | 4.2 | 8.8 | 2.1 8.0 4.0 4.3 6.2 4.9 4.0 3.6 3.2 5.3 5.3 2.2 8.5 4.2 4.1 3.1 6.2 4.2 3.3 6.4 3.2 4.4 3.9 2.9 6.1 7.1 2.2 3.9 8.8 2.4 |

Düşen koşular ve oranları [Kesin]:
- eski: 4/30 (önbellek 10,8 ve 9,3; yenileme 8,8 ve 8,5). **Gürültü 9–10'u da üretiyor**: tarihteki "9–10 = karesel" tek bir koşudan ayırt edilemez.
- 5 deneme + GC: 1/30 (önbellek 8,4; n=100 20,6 ms, n=400 172 ms, normali ~95 ms: 4n'in beş denemesi de patlamanın içinde).
- dönüşümlü: 0/30, en büyük oran 6,7.

90 oranın kaçı eşiği aşıyor: eski >6: 16, >7: 6, >8: 4; 5 deneme + GC >6: 6, >7: 2, >8: 1; dönüşümlü >6: 2, >7: 0, >8: 0.
İstatistik sınırı: 4/30'a karşı 0/30 tek başına zayıf bir kanıttır (Fisher p≈0,11). Asıl kanıt dağılımın kuyruğu: >7 olan oran 6 → 2 → 0. [Muhtemel: dönüşümlü ölçüm CI'daki düşüşü giderir]
Ek yük notu: `dups` ve commit 17:47:33–37 arasında koştu; o aralıkta düşen koşu yok.

## Öbür çağıran güvenli mi? (`internal/lang/parser` TestManyInterpolationsLinear, boş makine, 5'er koşu)
- eski: oran 3,8–4,4; test süresi 1,0–1,1 sn
- yeni: oran 3,9–4,7; test süresi 1,8–2,1 sn (5 deneme + GC: +0,8 sn)
Güvenli [Kesin, 5 koşu]. `git grep -n ScalesLinearly`: başka çağıran yok (codeindex'te 3, parser'da 1, testkit'in kendi testleri).

## Ölçüm 5 — değişiklikten sonra main (5d342f4), boş makine, 15 koşu (18:04 UTC)
| commit | koşu | düşen | ölçüm | en küçük | ortanca | en büyük | bütün oranlar |
|---|---|---|---|---|---|---|---|
| 5d342f4 | 15 | 0 | kurulum | 3.8 | 4.4 | 4.9 | 4.3 4.3 3.8 4.1 4.7 4.4 4.4 4.0 4.2 4.0 4.4 4.9 4.5 4.4 4.5 |
| 5d342f4 | 15 | 0 | önbellek | 3.9 | 4.1 | 5.4 | 4.3 4.4 4.0 4.3 4.5 3.9 4.5 3.9 4.0 4.0 3.9 5.4 4.1 4.2 4.0 |
| 5d342f4 | 15 | 0 | yenileme | 4.0 | 4.5 | 5.1 | 4.5 4.7 4.5 4.3 4.9 4.8 4.0 4.9 4.5 5.1 5.1 4.4 4.6 4.5 4.1 |

15 koşunun hiçbiri düşmedi [Kesin]. Test süresi 3,3 sn'den 5,0–5,8 sn'ye çıktı (5 deneme + her denemede GC). Not: boş makinede eski yöntem de 45/45 geçiyordu; bu ölçüt tek başına düzeltmeyi kanıtlamaz, kanıt ölçüm 4'tür.

## Karesel sınama (`TestScalesLinearlyFailsQuadraticWork`, internal/testkit)
`ScalesLinearly`'nin kendisi bir kaydedici ile çağrılır; iş doğrusal + karesel pay, karesel pay n'de doğrusalın 4 katı (beklenen oran ≈13,6), küçük boy ~20 ms.
- Ağır yükte 40 koşu: 40'ı da doğru düştü, oranlar 9,8–27,7. [Kesin]
- Önceki biçimler yükte kararsızdı: eşit pay (oran ~10) boş makinede 20 koşuda en küçük 8,9; iki kat pay ve 6 ms'lik küçük iş ağır yükte 40 koşuda bir kez yanlışlıkla geçti (oran 4,1); iki kat pay ve 20 ms ağır yükte en küçük 8,0.
- Bu sınama ölçümden beri geçerli kalan "aşınca yeniden ölç" yasağını da korur: işlevin belgesinde yazılı.

## Değişiklikler (dal `claude/keen-volta-zqw48d`)
- 92b9aa0 testkit: 5 deneme, her denemeden önce `runtime.GC()` (görevin tarifi).
- 5d342f4 testkit: n ve 4n'in denemeleri dönüşümlü (görevin tarifinin ötesi; gerekçe ölçüm 3 ve 4).
- 52e5285, 23cc488 testkit_test: karesel sınama.
- Sınır 8 kaldı, yeniden deneme yok, birim denetimi aynı. `cache_test.go`'ya ve başka paketlerin sınırlarına dokunulmadı.
- `go test ./internal/devtools/codeindex/ ./internal/testkit/` geçti; codegraph `run:` komutları (testkit.go ve testkit_test.go için) geçti; `dups --changed main`: 0 aday.

## Kalanlar
- **Var olan `TestLinearRatioSeparatesLinearFromQuadratic` (testkit) ağır yükte kararsız**: eski kodda 40 koşuda 7, yenisinde 40'ta 5 düştü (doğrusal işin oranı 8,3–11,6; birkaçında karesel iş ≤8). Bu değişiklikten önce de vardı [Kesin]. Nedeni [Muhtemel]: küçük iş 4 ms, büyük 16 ms; aşırı yüklü çekirdekte kısa iş zamanlayıcının bir diliminde kesilmeden bitebiliyor, uzunu hep kesiliyor. Düzeltmesi işi büyütmek (küçük boy ≥20 ms, yeni sınamadaki gibi). Kapsam dışı bıraktım; CI'da bu yüzden düştüğü görülmedi. Aynı neden kısa işli gelecekteki çağıranlar için de geçerli: `ScalesLinearly`'nin küçük boyu ≥20 ms olmalı (bugünkü iki çağıranın küçük boyu 20–90 ms).
- Tam düzeltme yerine süre değil işlemci zamanı ölçmek (getrusage) aşırı yükten bağımsız olurdu, ama Windows'ta çözünürlük ~15,6 ms ve platforma özgü kod ister. Önermiyorum. [Tahmin]
- CI'da birkaç koşu izlenmeli: yerelde CI'ın yükünü yaklaşık kurdum, kendisini değil.

## Sorular
1. Dönüşümlü ölçüm (5d342f4) görevin "gürültü çıkarsa" tarifinde yoktu. Ölçüt aynı (sınır 8, yeniden deneme yok), yalnız iki boyun aynı yükte ölçülmesi sağlanıyor; bunu "yöntemi gevşetmek" saymıyorum. Ana oturum aksini düşünürse tek commit'tir, geri alınabilir; ama 5 deneme + GC tek başına yükte düşüşü gidermedi (ölçüm 3 ve 4).
2. `TestLinearRatioSeparatesLinearFromQuadratic`'in işini büyütmek ayrı bir iş olarak açılsın mı?
