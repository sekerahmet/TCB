# TR — proje kuralları (her oturum ve ajan için)

TR, yapay zekâ ekosistemindeki işler için bir programlama dilidir (Go, modül `github.com/sekerahmet/TCB`): model yazmak ve eğitmek, hazır modelleri çalıştırmak ve onlara ince ayar yapmak, modeli robot gibi makinelerin içinde çalıştırmak, uzaktaki modelleri bağlantı ve API ile kullanmak. Kararlar bu işlerin hepsine göre verilir, yalnız eğitime göre değil (Ahmet, 2026-10-27: "bir robotun içine yapay zeka modeli de yerleştirebilmek ... internete bağlı bir robotun içinden de çalıştırabilmeyi kastediyorum"; "mevcut yapay zeka modellerini de çalıştırmak, ince ayar yapmak vs yani yapay zeka eko sistemindeki bir sürü işlem"). Dilin asıl biçimi koddur: dosyalar diskte, program içeride dilden bağımsız kimliklerle (`TR_fn_sum`) çalışır. Türkçe ve İngilizce yalnızca gösterim dilleridir; geliştirici kodu kendi dilinde okur ve yazar, yazdığı metin o dilden koda çevrilir. Bugün Türkçe ve İngilizce var, başka diller sıradadır; hiçbir dilin ayrıcalığı yoktur, "Türkçe programlama dili" diye tanıtılmaz. Ürün dilin kendisidir; model eğitimi dilin sınavıdır.

## Nereye bakılır

| Soru | Belge |
| :-- | :-- |
| Dilin asıl biçimi: kod, kimlikler (`TR_fn_sum`), gösterim dilleri | [`docs/spec/17_kodlar.md`](docs/spec/17_kodlar.md); kod tabloları [`internal/lang/codes/`](internal/lang/codes/README.md) (`definitions.json`, dil başına `tr.json`, `en.json`) |
| Kodlu bir `.tr` dosyasını okumak ve yazmak | `tr göster <dosya> [--dil en]` dosyayı kişinin dilinde gösterir; `tr kodla <dosya> [--dil en]` o dilde yazılmış metni koda çevirir. Kodlu dosya düz metin olarak düzenlenmez: göster → düzenle → kodla |
| Go kodu hangi kurallarla yazılır, her kuralı hangi test korur | [`internal/README.md`](internal/README.md) |
| Yeni fonksiyon, yerleşik, metot, ileti, paket, dil kuralı nasıl eklenir | [`docs/recipes.md`](docs/recipes.md) |
| Süreç: yeni ad isteği, tam denetim, ölçüm | [`docs/development.md`](docs/development.md) |
| Sıradaki işler, evreler ve bitiş ölçütleri | [`docs/roadmap.md`](docs/roadmap.md) |
| Dilin davranışı | [`docs/spec/`](docs/spec/README.md) |
| Bir paketin kendi standartları ve davranış testleri | o paketin `README.md`'si |
| Bir şeyi bulmak: çağıranlar, kullanım yerleri, ileti/ad kodları, kuralı koruyan test, bir fonksiyonun testleri | önce `go run ./tools/codegraph build`, sonra `query <ad>`, `who-uses H2100`, `tests-for <fonksiyon>`, `path A B` ([`tools/codegraph/README.md`](tools/codegraph/README.md)); dosya okumadan önce buna sor |
| TR kodunda (`.tr`: `lib/`, `examples/`, `tests/`, `tools/`) bir adın tanımı, kullananları, testleri, etkisi | `tr dizin tanım <ad>`, `tr dizin kullananlar <ad>`, `tr dizin sınayanlar <ad>`, `tr dizin etki <ad>`; komutlar da koddur, İngilizcesi `tr index definition`, `references`, `tested_by`, `impact` (`TR_cmd_index`, `TR_sub_definition` ...). Ad kimliğiyle de sorulur (`tr dizin tanım TR_fn_sum`). `--json` ile makine okunur; [`internal/devtools/codeindex/README.md`](internal/devtools/codeindex/README.md) |

## İlkeler

- **Önce sağlam temel, sonra özellik.** Yama gibi ekleme yapılmaz; yeni şey yazılı kurala ve tarife uyar.
- **Her kural bir testle korunur.** Yeni bir kural yazıya geçince ona bir test bağlanır.
- **Doğrusu neyse o, önce başka dillere bakılarak:** yeni bir parça ya da bir tasarım sorunu çıktığında önce başka dillerin ve çerçevelerin (Python, Swift, Kotlin, Rust, Go, PyTorch, JAX ...) yerleşik çözümü karşılaştırılır; uygunsa TR'ye uyarlanır, hiçbiri uymuyorsa gerekçesiyle yeni bir yöntem seçilir. Karşılaştırma ve gerekçe belgeye (dil tanımı maddesi ya da commit mesajı) yazılır. Böylece başka dillerin güçlü yanları alınır, bilinen hataları tekrarlanmaz.
- **Kritik tasarım kararı araştırmayla verilir, ezberden olmaz.** Yeni bir alt sistem, dosya biçimi, komut adı ailesi, dil kuralı ya da genel API tasarlanmadan önce güncel kaynaklar okunur: resmi belge, spesifikasyon, kaynak kod. Karşılaştırma tablosunun her satırı kaynağına (bağlantı ve sürüm) dayanır. Kaynaksız iddia "kanıtsız" diye işaretlenir. Tasarım önerisi kaynak bağlantılarıyla birlikte sunulur ve öyle onaylanır.
- **Büyük tasarım turlarla yapılır.** Araştırmadan sonra tasarım yalnızca kod grafiği ve `tr dizin` sorularıyla planlanır (1. tur); aracın yanlış ya da eksik cevapları düzeltilince aynı sorularla yeniden planlanır (2. tur), gerekirse bağımsız denetim ve bir tur daha. Bulgular `scratch/<konu>_tasarim_temeli.md`'ye yazılır; her turun soruları beklentileriyle `tools/codegraph/testdata/planning_questions.txt` sınavına girer. Tasarım notu ve onay turlardan sonra gelir.
- **Önce ölç, sonra değiştir:** hıza dokunan değişiklik önce ve sonra ölçülür; kazandırmayan geri alınır.
- **Dış inceleme bulguları doğrulanarak düzeltilir:** önce yeniden üretilir; düzeltmenin testi önce kalır, sonra geçer.

### Sadelik: kod gereksiz büyümez

1. **Önce ara, sonra yaz.** "Yok" demeden önce codegraph, `tr dizin`, dil tanımı ve `lib/` taranır; bulunamazsa nerelere bakıldığı söylenir. Bir dosyaya dokunmadan önce `go run ./tools/codegraph tests-for <dosya>` onu koruyan testleri ve kuralları gösterir (`.tr` ve `.md` dosyaları için Go testleri dahil): kod o kurallara göre yazılır, commit'ten önce o testlerin saniyelik olanları yerelde koşar.
2. **Var olanı aç, yenisini kurma.** Go'da hesaplanan bir şey dile lazımsa mevcut olan açılır; ikinci sayaç, ikinci yol yazılmaz.
3. **Somut ihtiyaç olmadan ekleme yok.** Bugün kullanan bir yer gösterilemiyorsa eklenmez, yol haritasına not düşülür.
4. **Kullanıcıya görünen her yeni ad bir karardır.** Yerleşik, parametre, betik seçeneği: kaynaklı karşılaştırma ve onay olmadan eklenmez.
5. **Bir sorun, bir değişiklik.** Çalışırken fark edilen başka eksik o işe eklenmez; ayrı not olur.
6. **Silmek eklemekten önce gelir.** Yeni şeyin gereksizleştirdiği kod aynı değişiklikte silinir.
7. **En küçük çözüm.** Aynı sonucu daha az kodla veren yol seçilir; genelleştirme ikinci gerçek kullanım gelince yapılır.
8. **Gerekçe bir cümle.** "Neden gerekli, mevcut olan neden yetmedi" commit mesajında tek cümleyle yazılamıyorsa eklenmez.
9. **Boyut sinyaldir.** Değişiklik beklenenden büyüyorsa durulur, tasarım yeniden düşünülür.
10. **Kopya kalmaz.** `codegraph dups --changed main` her commit'ten ve birleştirmeden önce çalışır; çıkan aday aynı işte giderilir ya da `tools/codegraph/dups_exclude.go`'ya gerekçesiyle yazılır (ön denetim ve tam denetim yeni adayda kalır). Belgede aday numarasıyla değil kimliğiyle (`dir/file:Func~dir/file:Func`) anılır.

### Kod grafiğinin cevabı nasıl okunur

- **"Yok" cevabı kanıt değildir.** `no uses`, `no tests found`, `not found` ile silme, "test yok" ya da "kullanılmıyor" kararı verilmez; önce ikinci bir yoldan doğrulanır (grep, `tr dizin`, kaynağın kendisi). Kör noktalar arayüzlerde, veri dosyalarında, test dizesindeki TR kodunda ve tür kullanımında çıktı.
- **Metnin içeriği grep ile aranır.** Kod grafiği adları ve bağları bilir, metin sabitlerinin ve yorumların içeriğini aramaz: bir metnin (`"+Sonsuz"`, bir ileti parçası, bir yol) nerede üretildiği ya da geçtiği grep ile bulunur, sonra bulunan yer grafiğe sorulur: `query dosya:satır` satırı içeren bildirimi verir, `grep -rn ... | go run ./tools/codegraph query -` her satırın altına onu yazar.
- **Kısaltılmış grupta karar verilmez.** `... N more` görünen listede (ör. bir yapının alanları metotlarının altında) karar `--all` ile bakılarak verilir.
- **Ad tahminle aranmaz.** Kullanıcı adından ya da ad kodundan (`query yazdır`), yapıdan (okuma/yazma kenarları, arayüz uygulamaları) ya da belge yorumundan gidilir; tahmin edilen bir adın bulunamaması "yok" demek değildir. `[INFERRED]` kenarlar tahmindir; uzun zincirli dolaylı test ya da paylaşılan bir üye adı üzerinden gelen bağ kaynağı açılarak doğrulanır.
- **Yanlış ya da eksik cevap aracın hatasıdır.** Somut örnekle (komut, beklenen, gelen, ikinci yoldan doğrulama) ana oturuma bildirilir; ana oturum yeniden üretip [`tools/codegraph/gaps.md`](tools/codegraph/gaps.md)'ye yazar; maddeler birikir, ana oturum gözden geçirir, DENETİM denetler, sonra CODEGRAPH'a toplu gider (baş geliştirici aksini onaylamadıkça). Dosyayı yalnızca ana oturum günceller. O örneği yamalayan değil, aynı sınıftaki her durumu kapatan genel çözüm istenir; kör noktanın sessizce geri gelmemesi için bir tutarlılık ya da kapsam testi eklenir (go/types'a karşı kullanım testi, her veri dosyası ve yerleşik için test bağı testi gibi).

## Kesin kurallar

- **İçeride kod, dışarıda dil.** Disk ve program her zaman kodla (kimliklerle: `TR_fn_sum`) çalışır; Türkçe ve İngilizce yalnız gösterim dilidir (Ahmet, 2026-10-26: "türkçe ve ingilizce sadece gösterim dilidir. disk ve programla dili hep kodlardan çalışır. kod değiştirmek ve yazmak isteyen dili belirtince dil tercüme edilir"). İçeride bir insan diline ayrıcalık tanıyan öneri yapılmaz; `codes.Active()` kimlik listesidir, Türkçe `codes.Source()`'tur. Koruyan test: `codes.TestProgramRunsOnIdentities`.
- **Codegraph zorunludur.** Bir dosyaya dokunmadan önce `go run ./tools/codegraph tests-for <dosya>` okunur: kanca (`tools/edithook`, `.claude/settings.json`) her oturumda izlenen bir dosyanın ilk düzenlemesini bir kez durdurup cevabı gösterir. Tasarım ve "nerede, kim kullanıyor, ne etkilenir" soruları dosya okumadan önce codegraph'a ve `tr dizin`'e sorulur. Commit'ten önce `codegraph dups --changed main` çalışır; ön denetim ve tam denetim yeni adayda kalır, bilerek kalan kopya `tools/codegraph/dups_exclude.go`'ya gerekçesiyle yazılır. "Yok" ya da "belirsiz" cevap kanıt değildir, ikinci yoldan doğrulanır.
- Kullanıcıyla iletişim **Türkçe** ve kısadır.
- Go adları, dosya ve klasör adları **İngilizce**; Go yorumları **Türkçe**.
- Kullanıcıya görünen her ad bir **ad kodudur** (`internal/lang/codes`): kodun kimliği `TR_<sınıf>_<İngilizce ad>` kuralıyla bir kez verilir ve hiç değişmez ([`docs/spec/17_kodlar.md`](docs/spec/17_kodlar.md)); kullanıcıya görünen adı (Türkçe ve İngilizce karşılığı) baş geliştirici onaylar. Her ileti bir **ileti kodudur** (`internal/lang/messages`). Go metnine kullanıcıya görünen Türkçe ad ya da ileti yazılmaz.
- **Bitti ne demek:** yan oturumun işi, dokunduğu paketlerin testleri geçince ve devir notu yazılınca teslim edilir; bütün takımı koşmak onun işi değildir. İş, ana oturum birleştirmede `bash tools/full_check.sh`'i geçirince biter. Ana dala göndermeden (push) önce tam denetim ana oturumda bir kez çalışır (birçok oturum çalışırken Colab'da), sonra otomatik sınama (CI) izlenir. GPU'ya dokunan iş önce değişen yerin testleriyle yan dala gider, Colab'da sınanır, sonra tam denetimle ana dala gider ([`docs/development.md`](docs/development.md#gönderim-sırası-cpu-ve-gpu)). **Tam denetim sürerken depoda dosya düzenlenmez.** **Uzun denetimler arka planda koşar:** ön denetim ve tam denetim (`tools/full_check.sh`, her kip) ve `./...` desenli `go test` ön planda çağrılmaz; kanca (`tools/edithook`, `.claude/settings.json`) ön plandaki çağrıyı durdurur, yan oturumda arka plandakini de.
- Commit mesajları İngilizcedir (`paket: ne değişti`); taşıma ile davranış değişikliği ayrı commit'lerdir.
- **Depo herkese açıktır:** kullanıcı verisi, veri kümeleri, parçalayıcı ve model dosyaları, kişisel yollar commit'lenmez. Depo kökündeki izlenmeyen dosyalar kullanıcınındır; onlara dokunulmaz, eklenmez.
- Testlerde dış ağ yoktur; sunucular yalnızca `127.0.0.1`'e bağlanır; açılan her süreç kapatılır.

## Yan oturumlar ve ajanlar

- Her yan oturum kendi çalışma ağacında (worktree) çalışır; aynı klasörü paylaşan iki oturum birbirinin denetimini bozar.
- Yan oturum push etmez; tam denetimi ve bütün ağacın testini (`...` desenli `go test`) çalıştırmaz (kanca arka planda da durdurur). Birleştirme, tam denetim ve gönderim ana oturumdadır.
- **Testler madde madde, paket toplu** (Ahmet, 2026-10-27: "her işin testi tek tek koşular bitince toplu koşulur"): her madde ya da commit için yalnız o işin testleri adıyla koşulur (yeni ya da değişen testler, dokunulan dosyanın `tests-for`'daki doğrudan testleri, gerekiyorsa planlama sınavı); dokunulan paketlerin bütün testleri bütün maddeler bitince bir kez koşulur. Ana oturum her yönergeye bu kuralı ve işin test komutunu adıyla yazar.
- İşin sonunda yönergedeki devir notunu yazar (durum, birleştirmede dikkat, kararlar, bulgular, öneriler, kalan) ve ana oturuma bulguları bildirir. Durum bölümünde iki denetimin çıktısı yer alır: değiştirdiği her dosya için dokunmadan önce koşulan `codegraph tests-for <dosya>` sonucunun özeti (koruyan testler ve kurallar; "test yok" çıktıysa ikinci yoldan doğrulaması) ve `codegraph dups --changed main` sonucu.
