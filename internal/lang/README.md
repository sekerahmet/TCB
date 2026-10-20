# internal/lang

TR dilinin çekirdeği: kaynak metinden çalışan programa kadar her şey burada. Tensör ve GPU
işleri `internal/compute`'ta, motorlar (eğitim, veri, dosyalar) `internal/stdlib`'dedir.
Buradaki kod onları bilmez; onlar dile kancalarla bağlanır.

## Standartlar

**Adlar.** Buradaki Go kodu İngilizcedir. Kullanıcının gördüğü bir ad (anahtar kelime,
yerleşik, tip, üye, adlı argüman) hiçbir zaman Go metnine yazılmaz: kendi koduyla yazılır
(`codes.G100`), ad dil dosyasından gelir. Kullanıcıya görünen bir adı değiştirmek yalnızca o
dosyayı değiştirir.

**Kullanıcı mesajları.** Kullanıcıya giden mesajlar kullanıcının dilindedir, satırla başlar
(`Satır 3: ...`) ve ne yapılacağını söyler; metinleri ileti kodlarıyla `messages`'tadır. Kaldırılmış, taşınmış ya da yanlış yazılmış adların
ipuçları tek yerden (`hint`) gelir; böylece her aşama aynı sözlerle anlatır.

**Testler.** `X_test.go`, `X.go`'yu sınar ve onun yanında durur (`X_fuzz_test.go` da öyle).
Birden çok dosyayı kesen davranış testi `behavior_<konu>_test.go` adını taşır ve paketin
README'sinde anılır (bkz. [`internal/README.md`](../README.md#testler)). Dilin kendi davranışı
`tests/` altındaki `.tr` programlarıyla sınanır.

**Kancalar.** Yorumlayıcıya bir arayüzle bağlanan her tip (indeksleme, işleçler, yineleme,
üyeler ...) derleme anı denetimi taşır: `var _ interp.X = (*T)(nil)`. Yorumlayıcı kancaları tip
denetimiyle bulur; adı değişen bir metot, bu denetim olmadan kancayı sessizce koparır.

**Değişiklikler.** Ad değişikliği ya da taşıma, davranış değişikliğiyle aynı commit'e girmez;
taşınan kod olduğu gibi taşınır.

## Bir değişikliğin etkisi

Bir adı değiştirmeden önce nerede kullanıldığını koddan sorun; cevap her seferinde koddan
hesaplanır, eskimez:

    go run ./tools/impact codes.Name              // bir adın bütün kullanımları
    go run ./tools/impact interp.Iterable         // bir kanca arayüzü: kullanımları ve gerçekleyenler
    go run ./tools/impact interp.VM.Interrupt     // bir metot ya da alan
    go run ./tools/impact internal/lang/parser    // bir paketi kim, hangi adlarıyla kullanıyor
