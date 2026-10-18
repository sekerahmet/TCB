# 17. Kodlar

Dilin kullanıcıya görünen her adı (anahtar kelimeler, bağlamsal sözcükler, tip adları, yerleşik fonksiyonlar, metot ve özellikler, adlı argümanlar) sabit bir **koda** bağlıdır. Kodların listesi (kısaca **kodlar**) iki dosyadır: dilden bağımsız tanımlar `internal/dil/kod/tanim.json`'da, adlar ve açıklamalar dil dosyasında, Türkçe için `internal/dil/kod/tr.json`'dadır (ikisi de ikiliye gömülüdür, `tr` tek dosya olarak dağıtılır). **Her kodun ne işe yaradığı, imzası ve örneği [`docs/kodlar.md`](../kodlar.md)'dedir**; bu belge iki dosyadan üretilir (`go run ./bakim/uret/kod_belgesi`) ve "hangi kod ne işe yarar" sorusunun tek başvurusudur. Yorumlayıcının Go kodu adları değil kodları kullanır (`kod.G100`; adlı sabitler dil dosyasından üretilir: `go generate ./internal/dil/kod`). Bir adı değiştirmek için yalnızca dil dosyasındaki satır değişir; kodlar hiç değişmez. Başka diller aynı kodlara kendi adlarını verecektir (`en.json`, `pt.json`; dil kodları ISO 639-1, gerekirse bölgeli: `pt-BR`).

Kod bir harf ve üç basamaklı bir sayıdır; harf aileyi, yüzler basamağı aile içindeki grubu gösterir. Gruplar arasında sonradan ekleme için boşluk bırakılmıştır.

| Aile | Kapsam | Gruplar |
| :-- | :-- | :-- |
| `A` | Dilin çekirdeği | `A0xx` anahtar kelimeler (`A001` eğer, `A010` döngü, `A020` fonksiyon ...), `A1xx` bağlamsal sözcükler (`olarak`, `test`, `kurucu`, `hata`, `o`), `A2xx` tip adları (`A200` Tam ... `A23x` motor tipleri) |
| `G` | Genel yerleşikler | `G1xx` giriş, çıkış, dosya (`G100` yazdır, `G101` girdi) · `G2xx` değer, tip, dönüşüm (`G201` tip) · `G3xx` liste · `G4xx` matematik ve istatistik · `G5xx` zaman ve eşzamanlılık · `G6xx` JSON/CSV · `G7xx` test |
| `T` | Tensör ve yapay zekâ | `T1xx` oluşturma ve görünüm (`T100` tensör) · `T2xx` etkinleştirme ve eleman bazlı matematik · `T3xx` türev · `T4xx` iyileştiriciler · `T5xx` katmanlar, kayıplar, hazır model · `T6xx` vektör yardımcıları ve dil modeli · `T7xx` kaydetme, yükleme, veri türleri · `T9xx` adlı argümanlar |
| `V` | Veri ve parçalayıcı | `V1xx` veri kümesi · `V2xx` parçalayıcı · `V9xx` adlı argümanlar |
| `W` | Web, veritabanı, arayüz | kaldırıldı; kodları bir daha kullanılmaz |
| `Y` | Yöntemler: metot ve özellikler (`x.ad`) | `Y0xx` birden çok tipte ortak (`uzunluk`, `içerir` ...) · `Y1xx` Metin · `Y2xx` Liste · `Y3xx` Sözlük · `Y4xx` Tensör · `Y5xx` iyileştirici · `Y6xx` veri kümesi, okuyucu, parçalayıcı · (`Y7xx`–`Y9xx` veritabanı, web, arayüz: kaldırıldı) |
| `H` | Hata mesajları | ayrılmış; henüz kullanılmıyor |

**Dil dosyası** (`tr.json`): her kod bir satırdır ve kodlar artan sıradadır:

```json
"G100": { "ad": "yazdır", "eski": ["yaz"], "aile": "giriş ve çıkış", "açıklama": "Değerleri ... yazar ...", "örnek": "yazdır(\"Toplam:\", 2 + 3)" },
```

- `ad` kodun bu dildeki tek adıdır; kabul edilen tek yazım budur (bkz. 2.9). Türkçe karakterleri ASCII'ye çevrilmiş biçimi (`yazdir`) dosyada saklanmaz ve **kabul edilmez**; yalnızca hata mesajı için addan türetilir. Bu kural `internal/dil/kod`'da tek bir açma-kapama noktasından, `kod.ASCIIKabul` sabitinden yönetilir (kapalı). İpucunu `kod.Oneri` ve `AsilAdAlanda` verir; ikisi de yazılan adın ad alanına bakar (aynı ASCII yazım genel adlarda bir eski ada, üyelerde başka bir adın ASCII biçimine karşılık gelebilir) ve her zaman Türkçe adı önerir.
- `eski` kaldırılmış adlardır (bkz. 2.9a): kabul edilmezler, yalnızca hata mesajında `ad` önerilir; eski bir adın ASCII biçimi de (`islev`) aynı ipucunu verir. Bir ad değiştirildiğinde ya da kaldırıldığında eski ad bu listeye eklenir ve aynı ipucu kendiliğinden çalışır. Eş yazımlar için ayrı bir alan yoktur: her işin tek adı vardır.
- `ascii: false`, `ASCIIKabul` açılsa bile o adın ASCII biçimini kapalı tutar (bugün yalnızca birkaç adlı argüman); `ASCIIKabul` kapalıyken etkisizdir. `aile` insanlar için kısa bir grup adıdır.
- `açıklama` kodun ne yaptığını ve önemli bir kısıtını söyleyen bir-iki cümledir; `örnek` tek satırlık, kendi başına çalışan bir TR kodudur. Her kodun açıklaması ve her canlı kodun örneği vardır; örnekler `tr kontrol`'den geçer ve çalışır (`go test ./internal/dil/kod ./internal/motor`).

**Tanımlar** (`tanim.json`, dilden bağımsız): her kodun türü (`anahtar_kelime`, `bağlamsal`, `tip`, `fonksiyon`, `metot`, `özellik`, `adlı_argüman`), metot ve özelliklerde geçerli olduğu tipler (tip kodlarıyla, ör. `AdamW` ve `SGD` değerleri için `A214` İyileştirici), imzası ve ilişkili kodları:

```json
"G400": { "tür": "fonksiyon", "param": ["A205|A212", "A200?"], "adlı": [["T902", "A200", null], ["T906", "A203", false]], "dönüş": "A200|A201|A212", "ilişkili": ["G410"] },
"Y103": { "tür": "metot", "imzalar": { "A202": { "param": ["A202?"], "dönüş": "A205" }, "A212": { "param": ["A200", "A200?"], "adlı": [["T902", "A200", -1]], "dönüş": "A205" } } },
```

İmzada parametre adı yazılmaz (adlar dile bağlıdır): konumlu parametreler sırayla tip ifadeleridir (`A200|A201` Tam ya da Ondalık; sonek `?` isteğe bağlı, `*` sıfır ya da çok, `+` bir ya da çok), adlı argümanlar `[kod, tip, varsayılan]` üçlüsüdür. Tanımlar davranışla sınanır: tanımdaki adlı argümanlar yerleşiğin ya da metodun gerçekten kabul ettikleridir (bilinmeyen adla çağrı, geçerli adları sayan hatayı verir, bkz. 13.5) ve metot ve özelliklerin tip listesi, üyeyi gerçekten destekleyen tiplerdir.

**Taşınan kodlar.** Bir işin biçimi değiştiğinde (fonksiyondan metoda ya da metottan fonksiyona, bkz. 13.6) yeni biçim yeni bir koddur; eski kodun satırı iki dosyada da kalır, numarası bir daha kullanılmaz. `tanim.json`'da eski kod yalnızca `"taşındı"` ile yeni kodu gösterir (`"G200": { "tür": "fonksiyon", "taşındı": "Y001" }`), dil dosyasında adını, eski adlarını ve açıklamasını tutar. Taşınan kodun adı (ve eski adları, ASCII biçimleri) kabul edilmez, sıradan bir addır; yazılınca hata yeni biçimi gösterir (`'uzunluk(...)' TR'de bir fonksiyon değil; 'x.uzunluk' yazın`). Taşınma ad alanını değiştirmelidir (genel ad ↔ üye) ve yeni kod canlı olmalıdır; dil dosyası yüklenirken taşınan bir ad canlı bir adla ya da eski bir adla aynı ad alanında çakışırsa reddedilir. Go'da taşınan kodların sabiti yoktur (`kod.G200` derlenmez).

Bir ad yalnızca kendi ad alanında benzersiz olmalıdır: genel adlar (anahtar kelimeler, bağlamsal sözcükler ve yerleşikler), tip adları, üyeler ve adlı argümanlar ayrı ad alanlarıdır; bu yüzden `sırala` hem bir liste metodu (`Y202`) hem bir adlı argümandır (`W931`). Üye adları tipten bağımsız benzersizdir. Dil dosyası yüklenirken yinelenen kod, aynı ad alanında çakışan ad (ipucu belirsiz kalmasın diye ASCII biçimleri aynı olan iki ad da çakışır) ve geçerli bir adla çakışan eski ad reddedilir; `go test ./internal/dil/kod ./internal/motor` kayıtlı her yerleşiğin ve üyenin kod listesinde kodu olduğunu, kod listesindeki her kodun da karşılandığını denetler. Adlar başlangıçta ve derleme anında koda çözülür; çalışma anında ad → kod araması yapılmaz.
