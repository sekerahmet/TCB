# DURUM — TestBuildScalesLinearly: gürültü mü, gerileme mi?

- Başlama: 2026-10-29 17:29 UTC
- Bitiş: —
- Dal: `claude/keen-volta-zqw48d` (taban `main` 6cd1e49)
- Makine: Linux konteyner, 4 çekirdek, go1.26.10

## Adımlar
1. [x] Ölçüm: 1073aeab, 48bd054d, main × 15 koşu
2. [x] Karar
3. [ ] Düzeltme
4. [ ] Karesel sınama (internal/testkit)
5. [ ] Değişiklikten sonra main'de 15 koşu

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
