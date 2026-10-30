# DURUM — Faz 0.9: TR kodunun tipli yazıma göçü

Başlama: 2026-10-30 04:50 UTC. Dal: `claude/vibrant-feynman-b6x2dz` (taban `main` 862d848).

## Biten
- Sayaç (ayrıştırıcıyla, `internal/typed_code_test.go`, henüz commit'te değil): ilk sayım aşağıda.
- Tip tanığı (iki geçici commit): `TR_TYPE_WITNESS=<dosya>` uyuşmazlığı, `TR_TYPE_PROFILE=<dosya>` gözlenen tipleri yazar.
- Bulunan yanlış: sözcük çözümleyici satır sonundaki `?`'yi işleç sayıyordu (afcfc5a).
- lib (a392e62 demet, a5f14d0 tipler), tests (75b8db0 demet, b9124e0 tipler), examples (karışık liste → demet, tipler), tools/gpu + bench (demet, tipler).

## İlk sayım [ölçüldü: ayrıştırıcıyla, lib + tests + examples + tools (codegraph hariç) + belgelerin ```tr blokları + docs/codes.md]
- tipsiz parametre: 477 (lib 325)
- tipsiz alan: 170 (lib 97)
- değer döndürüp `->` yazmayan fonksiyon: 199
- öğe tipi yazılmamış `TR_List`/`TR_Dict`: 9
- genel tip (`TR_Any`, `TR_Object`, çıplak `TR_Function` ...): 7
- [1. tur]: 473 parametre, 97 alan.

## Kalan
- belge blokları, definitions.json örnekleri (docs/codes.md), şablonlar; Go testlerindeki TR parçaları (sayım); izin listesi; depo testi; status; tanığın geri alınması.

## Kararsız / sorular
- yok
