# ast

TR'nin sözdizimi ağacı: ayrıştırıcının kurduğu, denetçinin ve derleyicinin okuduğu deyim ve
ifade düğümleri.

## Standartlar

**Düğümler yalnızca veri taşır.** Çözümleme, denetim ve çalıştırma başka paketlerdedir; burada iş
yapan kod yoktur.

**Her düğüm yerini bilir.** Her düğüm kendi sözcük birimini taşır; hata mesajları ve hata
ayıklayıcı satırı oradan alır.

**Anlam konumdan bağımsız karşılaştırılabilir.** Bir ağacın dökümü konum bilgisi olmadan yazılır.
İki kaynağın dökümü aynıysa programlar aynı anlama gelir; biçimlendirici ve kodlu biçime çevirici
"anlam değişmedi" denetimini buna dayandırır. Yeni bir alan eklendiğinde dökümde de görünür.
