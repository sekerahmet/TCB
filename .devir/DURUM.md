# BİTTİ

# DURUM — docs/ sınıflı yazımın kimlik yazımına çevrilmesi (Faz 0, adım 3)

- Başlama: 2026-10-29 13:17 UTC (konteynerin saati)
- Bitiş: 2026-10-29 13:23 UTC (konteynerin saati; bu saat gerçek süreyi az gösteriyor olabilir, oturumun kendi zaman damgalarıyla karşılaştırın)
- Başlangıç ölçüsü: `git grep -cE 'TR_(fn|mem|arg|type|kw)_' -- 'docs/*.md'` → 26 belge, 1 318 satır (main `23e7be2`).
- Bitiş ölçüsü: 14 belge, 1 130 satır. Kalanların hepsi aşağıdaki "dokunma" listesinde; 1 089'u üç belgede (`codes.md` 745, `code_identities.md` 263, `archive/` 81).
- Üç komut geçiyor (son koşu `docs/terimler.md` commit'inden sonra; ondan sonra belge değişmedi):
  - `go test ./internal/devtools/testrunner/ ./internal/` → ok, ok
  - `go test ./cmd/tr/e2e/ -run 'DocLinks|DocTRCodeBlocks'` → ok (satır kayması olmadı, `doc_blocks_known.txt`'e dokunulmadı)
  - `go run ./tools/codegraph spec-refs` → 278 madde, 3514 anılış, 0 hata, 0 not

## Nasıl yapıldı

Commit'lenmeyen küçük bir Python betiği (oturumun geçici klasöründe): her `TR_<sınıf>_<ad>` eşleşmesini `definitions.json`'daki anahtarla doğrular, sonra `codes.Spelling` gibi sınıfı atar (`TR_fn_…`, `TR_type_…`, `TR_kw_…` → `TR_…`). Üye: önünde `.` varsa `t.TR_x`, yoksa `.TR_x`. Adlı argüman: ardından `:` geliyorsa `TR_x:`, tek başına ters tırnak içindeyse `` `TR_x:` ``; ikisi de değilse işaretler. Satır bazında atlama (`--skip dosya:satır`) ve yalnız ters tırnak içini çevirme (`--only-backtick dosya:satır`) seçenekleri var. Her belgenin farkı sözcük sözcük okundu; işaretlenen yerler elle çözüldü. `tools/migrate` yalnız ```` ```tr ```` bloklarını çevirdiği için düzyazıya uymuyordu; ama dönüşüm kuralı aynı (`tools/migrate/syntax.go` `identity`).

Kural (dil tanımı 17.2): **program metni** ve bir dil sözcüğünü düzyazıda anmak kısa yazımla; **içerideki kimlik** (tanım dosyaları, disk biçimleri, kod grafiği, Go, aynı kısa adı taşıyan kodları ayırmak) sınıflı kalır. Görevin "dokunma" tanımı bu ikinci grubu saymıyordu; 17.2'ye göre ekledim.

## Biten belgeler

- `docs/guide/` (12 belge, 4 commit): sınıflı yazım kalmadı.
- `docs/architecture.md`, `docs/development.md`, `docs/recipes.md`, `docs/terimler.md`: çevrildi; kalan satırlar aşağıda gerekçeli.

Eski bir çevirinin anlamca yanlış bıraktığı 5 ad düzeltildi (yalnız yazım değil):
- `guide/05_hata_ayiklayici.md:12` `f5` tuşu hata ayıklayıcı komutudur: `TR_kw_continue` → `TR_dbg_continue` (komşu satırlar da hata ayıklayıcı komutu).
- `guide/10_hazir_katmanlarla_gpt.md:35` KV önbelleğinin kırpma metodu: `ö.TR_mem_strip(n)` (metin metodu) → `ö.TR_truncate(n)` (`lib/katman.tr:461`).
- `guide/11_hizi_izlemek.md:25` ilerleme çubuğunun ölçü anahtarı metindir: `{TR_mem_loss: kayıp}` → `{"loss": kayıp}` (`examples/tr/39_veri_ile_egitim.tr:91`'deki gibi).
- `terimler.md:45` bloğun dikkat katmanı alandır: `TR_fn_attention` katmanı → `.TR_attention` (`TR_mem_attention`, property).
- `terimler.md:50` iyileştiricinin parametreler alanı: `TR_fn_parameters` alanı → `.TR_parameters` (`TR_mem_parameters`, property).

## Kalanlar (dokunma listesi, gerekçesiyle)

`git grep -nE 'TR_(fn|mem|arg|type|kw)_' -- 'docs/*.md'` yalnız bunları verir:

| Yer | Gerekçe |
| :-- | :-- |
| `docs/codes.md` (745 satır) | `tools/gen/codesdoc`'un ürettiği belge; elle düzenlenirse `tools/gen/codesdoc` testi (`AssertUpToDate`) düşer. Başlıklar ve "İlişkili:" satırları kodun anahtarını yazıyor. Aşağıdaki 1. soruya bakın. |
| `docs/code_identities.md` (263) | Eski kod → kimlik donmuş tablosu ("değişmez"). `tools/codegraph/renames.go` 2. sütunu kod anahtarı olarak okuyor; anahtarlar sınıflı (`definitions.json`). Çevrilirse eski kodların geçmişi koparılır. |
| `docs/archive/` (81) | Tarihli kopyalar, ikisi de "değiştirilmez" diyor; `TestClassedIdentitiesAreLive` ve `tools/migrate` de arşivi atlıyor. |
| `docs/README.md:8` | `code_identities.md`'nin içeriğini anlatıyor; tablodaki biçim `TR_kw_if`. |
| `docs/architecture.md:44` | `codes` paketi: içerideki kodlar (Go sabiti, tanım dosyası), 17.2. |
| `docs/architecture.md:75` | Kimlik biçiminin tanımı `TR_<sınıf>_<İngilizce ad>`; içerideki biçim, 17.2. (Kararsız, aşağıda.) |
| `docs/development.md:43` | Yeni kodun kimliği nasıl alınır: çakışma örneği `TR_arg_G_muon` içerideki kimliktir. |
| `docs/recipes.md:18, 54, 123` | Tarifin eklediği kodun anahtarı parantezde (`TR_fn_silu`); hemen altındaki JSON satırları bu anahtarı yazar. 18'deki program tipi (`TR_Float`) çevrildi. |
| `docs/recipes.md:22, 23, 56, 82` | `tr.json` / `definitions.json` içeriği ve anahtar sınıfının tarifi. |
| `docs/recipes.md:98` | Yalnız `"slots": {"num_heads": "TR_arg_num_heads"}`: `definitions.json` içeriği. Satırın TR kodu çevrildi. |
| `docs/recipes.md:125` | Yeni anahtar kelimenin kimliğinin İngilizce addan kurulması (`TR_kw_match`). |
| `docs/recipes.md:136, 162` | Tarifin örnek kodu. (Kararsız, aşağıda.) |
| `docs/terimler.md:31–52` (17 satır) | "Eş adlar" tablosunun kod sütunu: `TestSharedNamesListed` sınıflı kimlikle karşılaştırır. Gerekçe sütunundaki çıplak adlar aynı kısa adın ayrı kodlarını ayırır (17.2'deki `TR_fn_log` / `TR_arg_log` gibi). Bu satırlardaki ters tırnaklı program parçaları çevrildi. |
| `docs/spec/17_kodlar.md:20, 21` | İçerideki sınıflı kimliğin tanımı ve iki sınıfı ayıran örnek. |
| `docs/spec/17_kodlar.md:81`, `docs/spec/20_kontrol_noktasi.md:41` | Disk biçimi: `"optimizer": "TR_fn_AdamW"` (17.8). |
| `docs/spec/rationale/02_lexical_structure.md:27` | Ölçüm alıntısındaki silinmiş kimlik; `deadIdentityAllowed`'da kayıtlı, çevrilirse test "artık geçmiyor" der. |
| `docs/spec/rationale/17_codes.md:13` | Eski yazımı anlatıyor ("sınıflı yazım (`TR_fn_sum`) ... %19 daha çok parça"). |
| `docs/spec/rationale/17_codes.md:44`, `docs/status.md:936` | Ölçülmüş çıktı: kitaplık tipinin metni bugün `"TR_type_Linear"` (anlatılan hata). |
| `docs/status.md:59` | Sınıflı yazımın kaldığını ölçen madde; eski biçimi anlatıyor. Bu iş bitince maddenin güncellenmesi gerekebilir (2. soru). |

## Kararsız kaldığım satırlar

- `docs/recipes.md:136` "Örnek: `TR_fn_load_tensors` (`.trt` dosyası), `TR_type_Checkpoint`." ve `:162` "Örnek: `TR_fn_sin` (`sinüs` → `sin`, ...)": tarifin kodunu adlandırıyor, 18/54/123 ile tutarlı olsun diye sınıflı bıraktım. Program yazımı da savunulabilir. `:80`'i (`TR_LineReader.TR_next()`) çevirdim, çünkü çağrı biçiminde yazılmış bir ifade.
- `docs/architecture.md:75`: içerideki biçim doğru anlatılıyor ama program metnindeki yazımdan (`TR_if`, 17.2) hiç söz edilmiyor. Bir yarım cümle eklemek iyi olur; yazım çevirisinin dışında kaldığı için eklemedim.
- `docs/terimler.md:27`: `TR_arg_lr` / `TR_mem_lr` → `` `TR_lr:` ve `.TR_lr` `` yaptım (sınıfı yer söylüyor). Kod sütunuyla aynı tutmak istenirse sınıflıya dönebilir.

## Sorunlar ve sorular

1. **`docs/codes.md` (745 satır) ne olacak?** Kod listesinden üretiliyor; düzeltme üreticide (`tools/gen/codesdoc/main.go`, `codeLabel`). Önerim: başlıkta program yazımı (`TR_log(...)`, `.TR_first`, `TR_k:`), yanında parantez içinde sınıflı anahtar (aynı kısa adlı kodlar ayrılsın, `codegraph` ile eşleşsin); sonra `go run ./tools/gen/codesdoc`. Görev Go'ya dokunmamı yasakladığı için yapmadım. Ayrıca bu belgedeki örnekler Türkçe gösterimde (`yazdır(...)`, `tr.json` "example"); bu ayrı bir iş.
2. `docs/status.md:59` bu işin ölçüsünü taşıyor ("48 belgede yaklaşık 2 770 yer"); güncellemesi ana oturumun işi.
3. **Satır içi kod denetlenmiyor.** Yalnız ```` ```tr ```` blokları test ediliyor (`DocTRCodeBlocks`). Eski çeviriden kalan 5 anlam hatası buldum (yukarıda); kod listesinde var olan ama yanlış ad (ör. `TR_strip`) hiçbir testte yakalanmaz. Başka yerlerde de kalmış olabilir. Satır içi parçaları denetleyen bir test ileride değerlendirilebilir.
4. Kapsam dışı, Go'da gördüğüm: `internal/doc_terms_test.go:146` iletisinde "eş adlar tablosunda TR_none" yazıyor. Türkçe "yok" sözcüğü bir mekanik çeviride `TR_none` olmuş. Dokunmadım.
