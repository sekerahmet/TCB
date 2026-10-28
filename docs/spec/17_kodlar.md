# 17. Kodlar

Bu bölümün kurallarına göre bugünkü durum ve yapılacaklar: [`docs/status.md`](../status.md), "17. Kodlar".

Her kuralın emsali ve gerekçesi: [`rationale/17_codes.md`](rationale/17_codes.md).

TR kimlik kodlu bir dildir: dilin her sözcüğü sabit bir koda bağlıdır, program diskte ve çalışırken yalnız kodlarla yaşar, insanların dilleri yalnız gösterimdir (1, 2.3a). Bu bölüm kodların hangi adlara verildiğini, nasıl yazıldığını, nerede tanımlandığını ve bir dilde nasıl gösterildiğini söyler.

## Kod

**17.1** Dilin sözcükleri ve TR'nin kendi kitaplığının dışa açık adları koddur; kullanıcının ve başka kitaplıkların adları kod değildir.
- **Kod olanlar:** anahtar kelime, bağlamsal sözcük, tip, yerleşik fonksiyon ve sabit, metot ve özellik, adlı argüman; `tr/` ile yüklenen kitaplığın (11.6) yapı, fonksiyon, sabit, alan, metot ve parametre adları; komut satırının adları (17.9).
- **Kod olmayanlar:** kullanıcının değişken, fonksiyon, yapı, alan ve parametre adları; `tr/` dışındaki her modülün adları; dosya adları; metinler ve yorumlar. Bunlar yazıldıkları gibi kalır, hiçbir dile çevrilmez: `TR_var liste = [1, 2]`'nin `liste`'si her gösterimde `liste`'dir.
- `TR_` ile başlayan ad koda ayrılmıştır; kullanıcı adı olamaz (2.3a).
- *Test:* TR testi; Go: `TestUserNamesNeverBecomeLanguageWords`, `TestClassedIdentitiesAreLive`.

**17.2** Kodun iki yazımı vardır.
- **Program metninde** `TR_` ve kimliğin adı: `TR_if`, `TR_print(x)`, `x.TR_map(f)`, `TR_lr: 0.003`, `TR_Int` (2.3a). Sınıfı yazıldığı yer belirler; ajan sınıf yazmaz.
- **İçeride** sınıflı kimlik `TR_<sınıf>_<İngilizce ad>`: `TR_kw_if`, `TR_fn_print`, `TR_mem_map`, `TR_arg_lr`, `TR_type_Int`. Tanım dosyaları, disk biçimleri (17.8), kod grafiği ve Go kodu (`codes.FnPrint`) sınıflı kimliği kullanır. Programın dışındaki adlar (komut, seçenek, ortam değişkeni, hata ayıklayıcı komutu) belgelerde de sınıflı yazılır: `TR_cmd_check`.
- **Aynı kısa ad birden çok sınıfta olabilir**, yeri ayırdığı sürece: `TR_log(x)` logaritma (`TR_fn_log`), `TR_log: TR_true` adlı argüman (`TR_arg_log`). Aynı yerde geçerli iki sınıflı kimliğin kısa adı aynı olamaz; tanım dosyası yüklenirken reddedilir.
- Kimlik bir kez verilir ve değişmez; ad değişse de kalır. Anlam değişince yeni kimlik açılır (17.10). İki kimlik yalnız harf büyüklüğüyle ayrılamaz.
- *Test:* TR testi; Go: `TestProgramRunsOnIdentities`; aynı yerde aynı kısa adın reddi uygulanınca Go testi.

**17.3** Sınıflar ve ad alanları:

| Sınıf | Kapsam | Program metninde yeri |
| :-- | :-- | :-- |
| `kw` | anahtar kelimeler | deyim ve ifade |
| `ctx` | bağlamsal sözcükler (yalnız kendi bağlamında sözcük) | kendi bağlamı |
| `val` | değer adları (sonsuz, NaN): kaynakta sözcük değildir, değerin metninde görünür | — |
| `type` | tipler, kitaplığın yapıları dahil | tip ve değer |
| `fn`, `const` | yerleşik fonksiyonlar ve sabitler, kitaplığın fonksiyonları dahil | değer |
| `mem` | metot ve özellikler (`x.ad`) | noktadan sonra |
| `arg` | adlı argümanlar | çağrıda iki noktadan önce |
| `cmd`, `sub`, `opt` | `tr` komutları, alt komutları, seçenekleri | komut satırı |
| `repl`, `dbg` | kabuğun ve hata ayıklayıcının komutları | kabuk, hata ayıklayıcı |
| `env` | ortam değişkenleri | ortam |

- Bir ad yalnız kendi ad alanında benzersizdir: genel (`kw`, `ctx`, `fn`, `const`), tip, üye, adlı argüman ve komut satırının alanları.
- *Test:* TR testi; Go: `TestProgramRunsOnIdentities`.

## Tanım ve dil dosyaları

**17.4** Kodların tanımı dilden bağımsızdır, adları dil dosyalarındadır.
- **Tanım:** `internal/lang/codes/definitions.json`, sınıflı kimlik başına bir kayıt:
  - türü (`kind`);
  - kitaplık kodunun modülü (`module`);
  - imzası (13.9);
  - ilişkili kodlar;
  - başka dillerden alışılmış adlar (`familiar`, 17.6);
  - varsa ortak adı (`common_name`, 17.5).
- **Dil dosyaları:** `tr.json`, `en.json`, ve gelecek her dil için bir dosya (ISO 639-1: `es.json`, `pt-BR.json`); kod başına adı (`name`), grubu (`family`), bir-iki cümlelik açıklaması (`description`) ve tek satırlık çalışan örneği (`example`). Her dil dosyası her kodu adlandırır.
- Dosyaların alan adları İngilizce tanımlayıcıdır; insan dili yalnız değerlerde görünür. Hepsi ikiliye gömülüdür; kodların başvuru belgesi [`docs/codes.md`](../codes.md) bunlardan üretilir.
- **İletiler** kod değil, ileti koduyla tanımlanır (`H` ve dört basamak, `internal/lang/messages`); metni dil başına katalogdadır.
- **Kayıtlı her kod canlıdır:** açıklaması ve çalışan örneği vardır, bir yerde kullanılır. Henüz uygulanmamış bir kod dil tanımında anılabilir ama tanım dosyasına uygulandığı gün girer.
- *Test:* TR testi; Go: `TestEveryCodeHasDescription`, `TestClassedIdentitiesAreLive`.

**17.5** Ad: her kodun her dilde tek bir adı vardır (2.9: bir iş, bir yazım).
- **Ortak ad:** uluslararası gösterimle ya da özel adla yazılan kavramın adı her dilde aynıdır: `sin`, `exp`, `log`, `max`, `argmax`, `relu`, `AdamW`, `GPT`, `Muon`. Tanım dosyasında bir kez yazılır, hiçbir dil dosyasında yazılmaz.
- **Kısa ve düz:** ad işi söyler ve kısa tutulur; tek sözcük yeterse tek sözcüktür. Örnekler: Türkçede `yazdır`, `sınırla`, `dilim`; soru soran ad soru ekiyle (`sonlu_mu`).
- **Ad alanında çakışmaz:** aynı ad alanında iki kod aynı adı (ASCII biçimi aynı olanlar dahil) taşıyamaz. Ayrı alanda aynı ad ancak aynı anlamda ya da gerekçesiyle verilir; her eş ad grubu [`docs/terimler.md`](../terimler.md)'dedir.
- Harfleri o dilin harfleridir; ASCII biçimi (Türkçe harfsiz) kabul edilmez, yalnız ipucunda doğru adı önermek için kullanılır.
- *Test:* TR testi; Go: `TestEveryCodeHasDescription`; ad alanı çakışmasının reddi uygulanınca Go testi.

**17.6** Başka dillerin alışkanlık adları (`str`, `range`, `lambda`, PyTorch'un `long`'u) kod değildir; tanım dosyasının `familiar` alanında durur ve yazıldığında ileti doğru kimliği önerir (2.3a). Kaldırılmış bir kodun eski adı `familiar` değildir (17.10).
- *Test:* TR testi; Go: `TestFindForeign`.

## Görünüm

**17.7** Görünüm, kodlu dosyanın bir dildeki gösterimidir.
- `TR_cmd_show <dosya> --lang <dil>` görünümü yazar, `TR_cmd_encode <dosya>` geri kodlar. Görünümün başlığı dilini taşır: `# TR 4 view=tr`. Görünüm çalışmaz; çalıştırılırsa ileti önce kodlamayı önerir (2.1).
- **Yalnız dilin sözcükleri çevrilir:** boşluk, satır düzeni, yorumlar, kullanıcı adları, metinler ve sayılar bayt bayt korunur. Kodlu → görünüm → kodlu ve görünüm → kodlu → görünüm bayt bayt aynıdır.
- **Kaçış:** görünümde dilin bir sözcüğüyle aynı yazılan kullanıcı adı `$` ile yazılır. Kullanıcının `yazdır` adlı değişkeni Türkçe görünümde `$yazdır`'dır; `yazdır(...)` ise `TR_print`'tir. Kodlu dosyada kaçış yoktur.
- Sözdizimi hatası olan ya da UTF-8 olmayan dosya çevrilmez. Çevirici sonucunu kendisi denetler: kodlu programın ağacı özgününkiyle aynı değilse dosyaya dokunmaz.
- **Konum** dosyanın metnindedir (2.1). Görünümdeki karşılığını göstermek düzenleyicinin işidir.
- *Test:* TR testi; Go: `TestEncodeShowCommands`, `TestEncodeShowStandardInput`, `TestEnglishWritingLanguage`, `TestRepoFilesEnglishLossless`, `TestRepoTRFilesEncodeLossless`, `TestEscapeInOtherLanguage`, `TestDAPCodedFileView`.

**17.8** Program dilden bağımsızdır.
- **Anlam koda bağlıdır:** hangi yerleşiğin çağrıldığı, kullanıcının adları ve sonuç gösterim diline bağlı değildir.
- **Değerin metni dilden bağımsızdır** (3.14): "true", "none", "List", `<builtin print>`. Tipin metni kimliğin adıdır: kitaplığın `TR_Linear`'ının metni "Linear"dir, sınıflı kimlik değil.
- **Dosya biçimleri dilden bağımsızdır:** TR'nin, motorların ve kitaplığın diske yazdığı dosyaların (kontrol noktası, günlük, profil, paket, veri önbelleği) dosya ve alan adları İngilizce tanımlayıcıdır. Değeri dilin bir öğesi olan alan sınıflı kimliği taşır (`"optimizer": "TR_fn_AdamW"`); ağırlık adları kimlik yoludur.
- **Kişinin dili yalnız görüntüdedir** (`TR_DIL`): iletiler, yardım, kabuk, hata ayıklayıcı ve görünüm.
- *Test:* TR testi; Go: `TestValueTextIsLanguageIndependent`, `TestEveryTextConversionIsLanguageIndependent`, `TestCheckpointFormatIsLanguageIndependent`; kitaplık tipinin metni uygulanınca TR testi.

## Komut satırı

**17.9** `tr` komutları, alt komutları, seçenekleri, kabuğun ve hata ayıklayıcının komutları ve ortam değişkenleri de koddur (17.3).
- **Girdide her gömülü dilin adı geçer:** `tr çalıştır` ile `tr run`, `--dil` ile `--lang` her makinede aynıdır; betik `TR_DIL`'e bağlı kalmaz.
- **Çıktı kişinin dilindedir:** yardım ve iletiler adları `TR_DIL`'in dilinde yazar.
- **Ortam değişkeni** her dilin adıyla okunur (`TR_DIL`, `TR_LANG`); iki ad ayrı değerle verilirse `tr` başlamadan durur. Hesap çekirdeği değişkeni kimlik adıyla okur; `tr` başlarken bu adı koyar.
- **Seçenek yazımı:** iki tire, sözcükler tireyle ayrılır; tek harfli bayraklar (`-h`, `-v`) simgedir. `-` ile başlayan bilinmeyen argüman en yakın seçeneği önerir. Türkçe harfsiz biçim ve başka araçların kısaltmaları asıl adı önerir.
- *Test:* Go: `TestCommandLineNamesInEveryLanguage`, `TestCommandsHaveOneName`, `TestExportEnvironmentGivesTheIdentityName`.

## Değişiklik

**17.10** Dil taslak aşamasındadır: kaldırılan ya da başka biçime taşınan kodun kaydı silinir, eski adı için ipucu tutulmaz, depodaki kullanımlar yeni biçime çevrilir.
- Kodlu dosyayı kıran değişiklik (kimliklerin yazımı, başlığın anlamı) yalnız başlığın biçim sürümünü artırarak yapılır (`# TR 4`).
- Yeni kod, ad değişikliği ve yeni gösterim dili kırıcı değildir.
- Yayımdan sonra uyumluluk kuralı eklenecektir: kimlik yeniden kullanılmaz, kaldırılan ada ipucu.
- *Test:* gerekmez, henüz uygulanmadı; yayımda uyumluluk kuralı yazılınca Go testi.
