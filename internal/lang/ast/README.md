# ast

TR'nin sözdizimi ağacı: ayrıştırıcının kurduğu, denetçinin ve derleyicinin okuduğu deyim ve
ifade düğümleri.

## Standartlar

**Düğümler yalnızca veri taşır.** Çözümleme, denetim ve çalıştırma başka paketlerdedir; burada iş
yapan kod yoktur. Denetim: `TestNodesCarryOnlyData`.

**Her düğüm yerini bilir.** Her düğüm kendi sözcük birimini taşır; hata mesajları ve hata
ayıklayıcı satırı oradan alır. Denetim: `TestEveryNodeKnowsItsToken`.

**Anlam konumdan bağımsız karşılaştırılabilir.** Bir ağacın dökümü konum bilgisi olmadan yazılır.
İki kaynağın dökümü aynıysa programlar aynı anlama gelir; biçimlendirici ve kodlu biçime çevirici
"anlam değişmedi" denetimini buna dayandırır. Yeni bir alan eklendiğinde dökümde de görünür. Denetim: `TestDumpIgnoresPositions`, `TestDumpWritesEveryNonTokenField`.

**Kapsam kuralının gezintisi tektir.** Hangi ifadenin hangi kapsama ad tanımladığını (5.12) denetleyici
ve yorumlayıcı aynı yardımcılardan alır: `SubExpressions`, `ExprDefinedNames`, `StatementDefinedNames`
(`scopes.go`). İkisinin ayrışması bir dil kuralı hatasıdır. Denetim: `TestExprDefinedNamesFollowsScopeRule`,
`TestStatementDefinedNames`, `TestSubExpressionsOrderAndStop`; yorumlayıcı tarafı
`TestExprWalkersMatchReferenceOnRepositoryPrograms`.
