# Katkı

TR'ye katkı kuralları [`docs/katki.md`](docs/katki.md)'dedir: kod düzeni, Türkçe adlar, testlerin yeri, yeni kod (ad) isteği, tam denetim ve ölçüm disiplini. Katman haritası ve sahiplik: [`docs/mimari.md`](docs/mimari.md).

Kısaca:

- Bir iş, `bash tools/tam_denetim.sh` geçtiğinde biter; otomatik sınama da aynı betiği çalıştırır.
- Dilin kullanıcıya görünen her adı sabit bir koda bağlıdır ve kodu yalnızca baş geliştirici verir. Yeni bir yerleşik, metot, anahtar kelime ya da adlı argüman gerekiyorsa kodu kendiniz eklemeyin; **Sözlük isteği** konu şablonuyla isteyin (Issues → New issue). Önce [`docs/kodlar.md`](docs/kodlar.md)'ye bakın: aynı işi yapan bir kod varsa onu kullanın.
- Güvenlik açıklarını konu olarak açmayın: [`SECURITY.md`](SECURITY.md).
- Kodlu saklanan `.tr` dosyalarında (ilk satırı `//! tr kodlu 1`) `git diff`'in Türkçe fark göstermesi için `tr` fark sürücüsünü kendi git ayarınızda bir kez tanımlayın: `git config --global diff.tr.textconv "tr göster"` (depodaki `.gitattributes` `.tr` dosyalarını bu sürücüye bağlar; tarif ve sınırları: [`docs/kilavuz/01_baslangic.md`](docs/kilavuz/01_baslangic.md#kodlu-dosyalar-düzenleyici-ve-git-farkları)).
