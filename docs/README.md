# TR belgeleri

| Belge | Kim okur | Ne anlatır |
| :-- | :-- | :-- |
| [`guide/`](guide/README.md) | Dili kullanan herkes | Komutlar, anahtar kelimeler, tensörler ve yapay zekâ, veri ve parçalayıcı, hata ayıklayıcı, testler, biçimlendirme (bölüm başına bir dosya) |
| [`spec/`](spec/README.md) | Dili geliştirenler, kesin kuralı arayanlar | Dilin numaralı kuralları; her kuralın `tests/` altında bir testi vardır (bölüm başına bir dosya: **4.2** → `spec/04_operatorler.md`) |
| [`codes.md`](codes.md) | Herkes | Her anahtar kelimenin, tipin, yerleşiğin, metodun ve adlı argümanın ne işe yaradığı, imzası ve örneği. Kod listesinden üretilir, elle düzenlenmez: `go run ./tools/gen/codesdoc` |
| [`code_identities.md`](code_identities.md) | Eski commit'leri ve notları okuyanlar | Eski kodların (`A001`) kimliklere (`TR_kw_if`) ve Go sabitlerine eşlemesi; geçişte bir kez üretildi, değişmez |
| [`recipes.md`](recipes.md) | Geliştirici | Ekleme tarifleri: yeni fonksiyon, yerleşik, metot, ileti, paket, dil kuralı; dokunulacak dosyalar ve testler |
| [`roadmap.md`](roadmap.md) | Herkes | Yol haritası: dil tanımının yeniden yazılan bölümlerinden gelen işler ve kilometre taşları; eskisi [`archive/`](archive/roadmap-2026-10-27.md) |
| [`architecture.md`](architecture.md) | Depoda çalışanlar | Katmanlar, kim kimi içe aktarabilir, her paketin görevi, klasör haritası, sahiplik |
| [`development.md`](development.md) | Depoda çalışanlar | Kod düzeni: dosya başına tek konu, dosya ve fonksiyon sınırları, Türkçe adlar, paket belgesi, testlerin yeri, yeni kod isteği, tam denetim, ölçüm disiplini |

Depo kökündeki [`README.md`](../README.md) dili tanıtır ve ilk programı çalıştırmayı anlatır; VS Code eklentisi [`editors/vscode/README.md`](../editors/vscode/README.md)'de, hız kıyas seti [`tools/bench/README.md`](../tools/bench/README.md)'dedir.

Belgelerdeki her ```tr kod bloğu `tr kontrol`den geçer ve her göreli bağlantı var olan bir dosyaya ve başlığa gider; ikisini de `go test ./cmd/tr/e2e` denetler.
