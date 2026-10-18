# TR belgeleri

| Belge | Kim okur | Ne anlatır |
| :-- | :-- | :-- |
| [`kilavuz/`](kilavuz/README.md) | Dili kullanan herkes | Komutlar, anahtar kelimeler, tensörler ve yapay zekâ, veri ve parçalayıcı, hata ayıklayıcı, testler, biçimlendirme (bölüm başına bir dosya) |
| [`dil/`](dil/README.md) | Dili geliştirenler, kesin kuralı arayanlar | Dilin numaralı kuralları; her kuralın `testler/` altında bir testi vardır (bölüm başına bir dosya: **4.2** → `dil/04_operatorler.md`) |
| [`kodlar.md`](kodlar.md) | Herkes | Her anahtar kelimenin, tipin, yerleşiğin, metodun ve adlı argümanın ne işe yaradığı, imzası ve örneği. Kod listesinden üretilir, elle düzenlenmez: `go run ./bakim/uret/kod_belgesi` |
| [`yol_haritasi.md`](yol_haritasi.md) | Herkes | Yapılacak işlerin sırası: GPU, kitaplık, yazım dilleri, platformlar |
| [`mimari.md`](mimari.md) | Depoda çalışanlar | Katmanlar, kim kimi içe aktarabilir, her paketin görevi, klasör haritası, sahiplik |
| [`katki.md`](katki.md) | Depoda çalışanlar | Kod düzeni: dosya başına tek konu, dosya ve fonksiyon sınırları, Türkçe adlar, paket belgesi, testlerin yeri, yeni kod isteği, tam denetim, ölçüm disiplini |

Depo kökündeki [`README.md`](../README.md) dili tanıtır ve ilk programı çalıştırmayı anlatır; VS Code eklentisi [`duzenleyici/vscode/README.md`](../duzenleyici/vscode/README.md)'de, hız kıyas seti [`bakim/kiyas/README.md`](../bakim/kiyas/README.md)'dedir.

Belgelerdeki her ```tr kod bloğu `tr kontrol`den geçer ve her göreli bağlantı var olan bir dosyaya ve başlığa gider; ikisini de `go test ./cmd/tr` denetler.
