# checker

Programı çalıştırmadan önce okuyup hataları bulan denetçi. Bulduğu şeyler: tip uyuşmazlıkları
(tanım, atama, dönüş değeri, çağrı argümanı), sabite atama, tanımsız adlar, yerleşik adların
gölgelenmesi ve tip değerinin metinle karşılaştırılması (`tip(x) == "Tam"`).

`tr kontrol` komutunda, her program çalışmadan önce ve her modül yüklenmeden önce çalışır.
Kodlu biçime çevirici de bir sözcüğün dilin mi kullanıcının mı olduğunu buradan öğrenir.

## Standartlar

**Kod çalıştırmaz.** Yalnızca okur ve raporlar; programın hiçbir satırı burada çalışmaz.

**Hata ile uyarı ayrıdır.** Hata programın çalışmasını durdurur. Uyarı (ör. yerleşik adın
gölgelenmesi) durdurmaz ve çıkış kodunu etkilemez.

**Yanlış alarm vermez.** Emin olamadığında susar: adların tamamı bilinemiyorsa (ör. takma adsız
bir `aktar` varsa) tanımsız ad hatası verilmez. Çalışacak bir programı haksız yere durdurmak, bir
hatayı çalışma anına bırakmaktan daha kötüdür.

**Çalışma anıyla aynı sözleri kullanır.** Aynı hata denetimde ve çalışırken aynı metinle
anlatılır; ortak metinler `hint` paketinden gelir.

**Tek hata, tek mesaj.** Bir hata aynı satırda zincirleme ikinci bir hata üretmez (ör. yanlış
yazılmış tip adı ayrıca bir tip uyuşmazlığı olarak bildirilmez).
