# logging

TR'nin eğitim günlüğü ve ilerleme çubuğu yerleşikleri: `günlük(yol)` ve `ilerleme(toplam)`.
Araçlar (`tr izle`, `tr profil`) sayı ve süre yazımını buradan alır, böylece her yerde aynı
görünür.

## Standartlar

**Günlük biçimi sözleşmedir.** Günlük JSON satırlarıdır (`.jsonl`); kayıt alanlarının adları ve
biçim sürümü dosya biçiminin parçasıdır, yazım dilinden bağımsızdır ve çevrilmez. Biçim değişirse
sürüm artar.

**Kayıt kaybolmaz.** Kayıtlar dosyanın sonuna eklenir; önceki koşu yarım satır bıraktıysa yeni kayıt yeni satırda başlar,
kontrol noktasından sürdürmede "devam" kaydı düşülür. Program bitince açık günlükler kapanır,
bitmemiş çubuklar son satırlarını yazar.

**Çıktıya göre davranış.** Terminalde çubuk tek satırda canlı çizilir; dosyaya ya da boruya
giden çıktıda seyrek, düz satırlar yazılır. Çubuk programın kendi çıktısını bozmaz.

**Sayılar Türkçe yazılır.** Ölçüler 3 anlamlı basamakla ve ondalık virgülle, süreler
"3 dk 2 sn" biçiminde gösterilir.

**Adım kancası ucuzdur.** Canlı profil adım sınırlarını bir kancayla izler; kanca yokken
maliyet tek bir atomik okumadır.
