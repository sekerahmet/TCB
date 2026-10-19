# hint

Dilin adlarıyla ilgili hata ipuçlarının tek kaynağı: kaldırılmış ya da taşınmış adlar, Türkçe
karaktersiz (ASCII) yazılmış adlar, tanımsız bir ada en yakın görünen ad ve tip değerinin metinle
karşılaştırılması.

## Standartlar

**Tek kaynak.** Sözcük çözümleyici, ayrıştırıcı, denetçi ve yorumlayıcı aynı ipucunu buradan alır;
aynı hata her aşamada aynı sözlerle anlatılır. Yeni bir ad ipucu başka bir pakette yazılmaz, buraya
eklenir.

**Ne yazılacağını söyler.** Bir ipucu yalnızca neyin yanlış olduğunu değil, yerine ne yazılacağını
da söyler (`'yazdir' TR'de yok; 'yazdır' yazmak mı istediniz?`).

**Öneri her zaman asıl addır.** Önerilen ad dilin kabul ettiği yazımdır; ASCII biçim ya da eski ad
hiçbir zaman önerilmez.

**Adlar koddan gelir.** İpuçlarındaki dil adları elle yazılmaz, kod listesinden alınır; dil ya da
ad değişince ipuçları da kendiliğinden değişir.

**Yalnızca hata yolunda çalışır.** Buradaki işlevler program doğru çalışırken hiç çağrılmaz;
hızdan çok anlaşılırlık önemlidir.
