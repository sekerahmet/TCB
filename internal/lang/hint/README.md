# hint

Dilin adlarıyla ilgili hata ipuçlarının tek kaynağı: kaldırılmış ya da taşınmış adlar, Türkçe
karaktersiz (ASCII) yazılmış adlar, tanımsız bir ada en yakın görünen ad ve tip değerinin metinle
karşılaştırılması.

## Standartlar

**Tek kaynak.** Sözcük çözümleyici, ayrıştırıcı, denetçi ve yorumlayıcı aynı ipucunu buradan alır;
aynı hata her aşamada aynı sözlerle anlatılır. Yeni bir ad ipucu başka bir pakette yazılmaz, buraya
eklenir. Denetim: `TestHintMessagesBuiltOnlyHere`, `TestCheckerAndRuntimeShareWording` (interp).

**Ne yazılacağını söyler.** Bir ipucu yalnızca neyin yanlış olduğunu değil, yerine ne yazılacağını
da söyler (`'yazdir' TR'de yok; 'yazdır' yazmak mı istediniz?`). Denetim: `TestAsciiSpellingSuggestsRealName`, `TestDeadGlobalNamesAlwaysExplained`.

**Öneri her zaman asıl addır.** Önerilen ad dilin kabul ettiği yazımdır; ASCII biçim ya da eski ad
hiçbir zaman önerilmez. Denetim: `TestAsciiSpellingSuggestsRealName`.

**Adlar koddan gelir.** İpuçlarındaki dil adları elle yazılmaz, kod listesinden alınır; dil ya da
ad değişince ipuçları da kendiliğinden değişir. Denetim: `TestCiplakAdYok` (tools/gen/codeconsts).

**Yalnızca hata yolunda çalışır.** Buradaki işlevler program doğru çalışırken hiç çağrılmaz;
hızdan çok anlaşılırlık önemlidir.

## Testler

Birim testleri sınadıkları dosyanın adını taşır (`x_test.go` → `x.go`). Birden çok dosyayı kesen davranış testleri:

- `behavior_single_source_test.go`: ad ipucu iletilerinin (H1100-H1106) yalnızca bu pakette kurulduğu, dil çekirdeğinin öbür paketlerinde kurulmadığı
