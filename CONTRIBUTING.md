# Katkı

TR'ye katkı kuralları [`docs/development.md`](docs/development.md)'dedir: kod düzeni, Türkçe adlar, testlerin yeri, yeni kod (ad) isteği, tam denetim ve ölçüm disiplini. Katman haritası ve sahiplik: [`docs/architecture.md`](docs/architecture.md).

Kısaca:

- Bir iş, `bash tools/full_check.sh` geçtiğinde biter; otomatik sınama da aynı betiği çalıştırır.
- Dilin kullanıcıya görünen her adı sabit bir koda bağlıdır: kodun numarası ailesinin kuralıyla verilir, adı baş geliştirici onaylar. Yeni bir yerleşik, metot, anahtar kelime ya da adlı argüman gerekiyorsa kodu kendiniz eklemeyin; **Sözlük isteği** konu şablonuyla isteyin (Issues → New issue). Önce [`docs/codes.md`](docs/codes.md)'ye bakın: aynı işi yapan bir kod varsa onu kullanın.
- Güvenlik açıklarını konu olarak açmayın: [`SECURITY.md`](SECURITY.md).
- Kodlu saklanan `.tr` dosyalarında (ilk satırı `//! tr kodlu 1`) `git diff`'in Türkçe fark göstermesi için `tr` fark sürücüsünü kendi git ayarınızda bir kez tanımlayın: `git config --global diff.tr.textconv "tr göster"` (depodaki `.gitattributes` `.tr` dosyalarını bu sürücüye bağlar; tarif ve sınırları: [`docs/guide/01_baslangic.md`](docs/guide/01_baslangic.md#kodlu-dosyalar-düzenleyici-ve-git-farkları)).
