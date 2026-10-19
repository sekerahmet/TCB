# process

Programın içinde çalıştığı süreçle ilgili TR yerleşikleri: ortam değişkenleri ve betiğin komut
satırı seçenekleri (`komut_satırı`).

## Standartlar

**Gizli değer gizli kalır.** Adı gizli bir şey söyleyen ortam değişkenlerinin ve seçeneklerin
(anahtar, belirteç, parola ...) değerleri, insanın göreceği her çıktıda `***` olarak görünür.

**Tek yazım.** Komut satırı seçeneği tek biçimde yazılır (`--ad değer` ya da `--ad=değer`);
konumlu argüman ya da başka yazımlar kabul edilmez, en yakın doğru ad önerilir.

**Sayılar dilin yazımıyla okunur.** Komut satırındaki sayılar TR'nin sayı yazımına uyar;
kısmen sayı olan bir değer sessizce kırpılmaz, hatadır.

**Kullanım hatası temiz çıkar.** Yanlış bir seçenek ya da `--yardım` programı açıklayıcı bir
iletiyle ve tanımlı bir çıkış koduyla bitirir; çökme ya da yığın dökümü olmaz.
