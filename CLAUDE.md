# TR — proje kuralları (her oturum ve ajan için)

TR, yapay zekâ modeli geliştirmek ve eğitmek için Türkçe bir programlama dilidir (Go, modül `github.com/sekerahmet/TCB`). Ürün dilin kendisidir; model eğitimi dilin sınavıdır.

## Nereye bakılır

| Soru | Belge |
| :-- | :-- |
| Go kodu hangi kurallarla yazılır, her kuralı hangi test korur | [`internal/README.md`](internal/README.md) |
| Yeni fonksiyon, yerleşik, metot, ileti, paket, dil kuralı nasıl eklenir | [`docs/recipes.md`](docs/recipes.md) |
| Süreç: yeni ad isteği, tam denetim, ölçüm | [`docs/development.md`](docs/development.md) |
| Sıradaki işler, evreler ve bitiş ölçütleri | [`docs/roadmap.md`](docs/roadmap.md) |
| Dilin davranışı | [`docs/spec/`](docs/spec/README.md) |
| Bir paketin kendi standartları ve davranış testleri | o paketin `README.md`'si |
| Bir şeyi bulmak: çağıranlar, kullanım yerleri, ileti/ad kodları, kuralı koruyan test, bir fonksiyonun testleri | önce `go run ./tools/codegraph build`, sonra `query <ad>`, `who-uses H2100`, `tests-for <fonksiyon>`, `path A B` ([`tools/codegraph/README.md`](tools/codegraph/README.md)); dosya okumadan önce buna sor |
| TR kodunda (`.tr`: `lib/`, `examples/`, `tests/`, `tools/`) bir adın tanımı, kullananları, testleri, etkisi | `tr dizin tanım <ad>`, `tr dizin kullananlar <ad>`, `tr dizin sınayanlar <ad>`, `tr dizin etki <ad>` (`--json` ile makine okunur; [`internal/devtools/codeindex/README.md`](internal/devtools/codeindex/README.md)) |

## İlkeler

- **Önce sağlam temel, sonra özellik.** Yama gibi ekleme yapılmaz; yeni şey yazılı kurala ve tarife uyar.
- **Her kural bir testle korunur.** Yeni bir kural yazıya geçince ona bir test bağlanır.
- **Doğrusu neyse o, önce başka dillere bakılarak:** yeni bir parça ya da bir tasarım sorunu çıktığında önce başka dillerin ve çerçevelerin (Python, Swift, Kotlin, Rust, Go, PyTorch, JAX ...) yerleşik çözümü karşılaştırılır; uygunsa TR'ye uyarlanır, hiçbiri uymuyorsa gerekçesiyle yeni bir yöntem seçilir. Karşılaştırma ve gerekçe belgeye (dil tanımı maddesi ya da commit mesajı) yazılır. Böylece başka dillerin güçlü yanları alınır, bilinen hataları tekrarlanmaz.
- **Kritik tasarım kararı araştırmayla verilir, ezberden olmaz.** Yeni bir alt sistem, dosya biçimi, komut adı ailesi, dil kuralı ya da genel API tasarlanmadan önce güncel kaynaklar okunur: resmi belge, spesifikasyon, kaynak kod. Karşılaştırma tablosunun her satırı kaynağına (bağlantı ve sürüm) dayanır. Kaynaksız iddia "kanıtsız" diye işaretlenir. Tasarım önerisi kaynak bağlantılarıyla birlikte sunulur ve öyle onaylanır.
- **Önce ölç, sonra değiştir:** hıza dokunan değişiklik önce ve sonra ölçülür; kazandırmayan geri alınır.
- **Dış inceleme bulguları doğrulanarak düzeltilir:** önce yeniden üretilir; düzeltmenin testi önce kalır, sonra geçer.

### Sadelik: kod gereksiz büyümez

1. **Önce ara, sonra yaz.** "Yok" demeden önce codegraph, `tr dizin`, dil tanımı ve `lib/` taranır; bulunamazsa nerelere bakıldığı söylenir.
2. **Var olanı aç, yenisini kurma.** Go'da hesaplanan bir şey dile lazımsa mevcut olan açılır; ikinci sayaç, ikinci yol yazılmaz.
3. **Somut ihtiyaç olmadan ekleme yok.** Bugün kullanan bir yer gösterilemiyorsa eklenmez, yol haritasına not düşülür.
4. **Kullanıcıya görünen her yeni ad bir karardır.** Yerleşik, parametre, betik seçeneği: kaynaklı karşılaştırma ve onay olmadan eklenmez.
5. **Bir sorun, bir değişiklik.** Çalışırken fark edilen başka eksik o işe eklenmez; ayrı not olur.
6. **Silmek eklemekten önce gelir.** Yeni şeyin gereksizleştirdiği kod aynı değişiklikte silinir.
7. **En küçük çözüm.** Aynı sonucu daha az kodla veren yol seçilir; genelleştirme ikinci gerçek kullanım gelince yapılır.
8. **Gerekçe bir cümle.** "Neden gerekli, mevcut olan neden yetmedi" commit mesajında tek cümleyle yazılamıyorsa eklenmez.
9. **Boyut sinyaldir.** Değişiklik beklenenden büyüyorsa durulur, tasarım yeniden düşünülür.
10. **Kopya kalmaz.** `codegraph dups` her birleştirmede çalışır; çıkan aday aynı işte giderilir.

## Kesin kurallar

- Kullanıcıyla iletişim **Türkçe** ve kısadır.
- Go adları, dosya ve klasör adları **İngilizce**; Go yorumları **Türkçe**.
- Kullanıcıya görünen her ad bir **ad kodudur** (`internal/lang/codes`, yeni ad kodunu baş geliştirici verir). Her ileti bir **ileti kodudur** (`internal/lang/messages`). Go metnine kullanıcıya görünen Türkçe ad ya da ileti yazılmaz.
- Bir iş `bash tools/full_check.sh` geçince biter. Ana dala göndermeden (push) önce tam denetim çalışır, sonra otomatik sınama (CI) izlenir. GPU'ya dokunan iş önce değişen yerin testleriyle yan dala gider, Colab'da sınanır, sonra tam denetimle ana dala gider ([`docs/development.md`](docs/development.md#gönderim-sırası-cpu-ve-gpu)). **Tam denetim sürerken depoda dosya düzenlenmez.**
- Commit mesajları İngilizcedir (`paket: ne değişti`); taşıma ile davranış değişikliği ayrı commit'lerdir.
- **Depo herkese açıktır:** kullanıcı verisi, veri kümeleri, parçalayıcı ve model dosyaları, kişisel yollar commit'lenmez. Depo kökündeki izlenmeyen dosyalar kullanıcınındır; onlara dokunulmaz, eklenmez.
- Testlerde dış ağ yoktur; sunucular yalnızca `127.0.0.1`'e bağlanır; açılan her süreç kapatılır.

## Yan oturumlar ve ajanlar

- Her yan oturum kendi çalışma ağacında (worktree) çalışır; aynı klasörü paylaşan iki oturum birbirinin denetimini bozar.
- Yan oturum push etmez ve tam denetim çalıştırmaz; birleştirme ve gönderim ana oturumdadır.
- İşin sonunda yönergedeki devir notunu yazar (durum, birleştirmede dikkat, kararlar, bulgular, öneriler, kalan) ve ana oturuma bulguları bildirir.
