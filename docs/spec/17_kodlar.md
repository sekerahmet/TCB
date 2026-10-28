# 17. Kodlar

Dilin asıl biçimi koddur. Kullanıcıya görünen her ad (anahtar kelime, bağlamsal sözcük, tip, yerleşik, metot, özellik, adlı argüman, komut) sabit bir **koda** bağlıdır. Dosyalar diskte, program içeride bu kodlarla çalışır. Türkçe ve İngilizce yalnız gösterim dilleridir, hiçbirinin ayrıcalığı yoktur.

- **Dosyalar.** Dilden bağımsız tanımlar `internal/lang/codes/definitions.json`'dadır; her dilin adları ve açıklamaları kendi dosyasındadır (`tr.json`, `en.json`; yeni diller aynı biçimde, ISO 639-1: `es.json`, `pt-BR.json`). Hepsi ikiliye gömülüdür. Her kodun işi, imzası ve örneği [`docs/codes.md`](../codes.md)'dedir (`go run ./tools/gen/codesdoc` üretir). Go kodu adları değil kodları kullanır (`codes.FnPrint`; `go generate ./internal/lang/codes`). Bir adı değiştirmek dil dosyasındaki tek satırı değiştirmektir.
- **Kimlik.** Kodun yazımı `TR_<sınıf>_<İngilizce ad>`dır (`TR_kw_if`, `TR_fn_sum`, `TR_type_Tensor`, `TR_arg_lr`, `TR_mem_step`). Bir kez verilir ve hiç değişmez; ad değişse de kimlik kalır, yalnız anlam değişince yeni kimlik açılır (17.6). Sınıf ad alanını belirler (`step`: `TR_dbg_step`, `TR_arg_step`, `TR_mem_step`). İki kimlik yalnız harf büyüklüğüyle ayrılamaz; gerekirse sahibinin adı eklenir (`TR_arg_G_muon`, `TR_arg_g_swiglu`). `TR_` ile başlayan adlar kimliklere ayrılmıştır, kullanıcı adı olamaz (2.3a).

| Sınıf | Kapsam | Ad alanı |
| :-- | :-- | :-- |
| `kw` | anahtar kelimeler (`TR_kw_if` eğer) | genel |
| `ctx` | bağlamsal sözcükler (`TR_ctx_as`, `TR_ctx_test`, `TR_ctx_init`, `TR_ctx_error`, `TR_ctx_it`, `TR_ctx_acc`) | genel |
| `val` | değer adları (`TR_val_inf` sonsuz): kaynakta sözcük değildir, kodlanmaz (3.2) | genel |
| `type` | tip adları (`TR_type_Int` Tam) | tip |
| `fn`, `const` | yerleşik fonksiyonlar ve sabitler (`TR_fn_print` yazdır, `TR_const_pi` pi) | genel |
| `arg` | adlı argümanlar (`TR_arg_dim` eksen) | adlı argüman |
| `mem` | metot ve özellikler, `x.ad` (`TR_mem_length` uzunluk) | üye |
| `cmd`, `sub`, `opt` | `tr` komutları, `TR_cmd_index`'in alt komutları, seçenekler (17.8) | komut; alt komut; seçenek |
| `repl`, `dbg` | kabuğun ve hata ayıklayıcının komutları | komut; hata ayıklayıcı |
| `env` | ortam değişkenleri (`TR_env_LANG` `TR_DIL`) | ortam |

İletiler kimlik değil `H` ve dört basamaklı numaradır (`internal/lang/messages`): `H0` sözdizimi · `H1` tip denetimi · `H2` yorumlayıcı · `H3` tensör · `H4` GPU · `H5` veri ve parçalayıcı · `H6` ml · `H7` standart kitaplık · `H8` araçlar · `H9` iç hatalar. Parametreler adlıdır (`{fn}: {path\|q} bir klasör`); ileti ve içindeki değerler kişinin dilinde (`TR_DIL`) yazılır. TR kaynağında bir ileti `TR_fn_message(H6903, ...)` ile kurulur; ileti kodu bir metin değeridir (`"H6903"`, 2.12, 8.5); kitaplığın kendi adları iletide ad yuvasıyla (`{@ad}`, `internal/lang/messages`) yazılır, kaynak adı geçirmez.

**Dil dosyası.** Her kod bir satırdır, sınıf sırasıyla ve sınıf içinde kimliğe göre dizilir. Dil dosyasının ve tanım dosyasının alan adları ile türleri dosya biçiminin anahtarlarıdır: Go adları gibi İngilizce tanımlayıcıdır, bir insan dili yalnız değerlerde görünür:

```json
"TR_fn_print": { "name": "yazdır", "family": "giriş ve çıkış", "description": "Değerleri ... yazar ...", "example": "yazdır(\"Toplam:\", 2 + 3)" },
```

- `name` kodun o dildeki tek yazımıdır (2.9). ASCII biçimi (`yazdir`) kabul edilmez, yalnız ipucunda doğru adı önermek için kullanılır (`codes.AcceptASCII` kapalı).
- `foreign` başka dillerden alışılmış adlardır (`"TR_arg_seed": { "name": "tohum", "foreign": ["seed"] }`): kabul edilmez, hata iletisi doğru adı önerir (`AdamW 'lr' adında bir parametre almaz. Bunu mu demek istediniz: 'öğrenme_oranı'?`). Her dil kendi okurunun alışkanlıklarını yazar.
- `family` kısa bir grup adı, `description` bir-iki cümle, `example` tek satırlık çalışan TR kodudur. Her kodun açıklaması, her canlı kodun örneği vardır (`go test ./internal/lang/codes ./internal/stdlib`).

**Tanımlar** (`definitions.json`, dilden bağımsız): kodun türü (`kind`: `keyword`, `contextual`, `value_name`, `type`, `function`, `constant`, `method`, `property`, `named_argument`, `command`), metot ve özelliklerin geçerli olduğu tipler, imza, ilişkili kodlar ve varsa ortak ad:

```json
"TR_fn_sum": { "kind": "function", "params": ["TR_type_List|TR_type_Tensor", "TR_type_Int?"], "named": [["TR_arg_dim", "TR_type_Int", null], ["TR_arg_keepdim", "TR_type_Bool", false]], "returns": "TR_type_Int|TR_type_Float|TR_type_Tensor", "related": ["TR_fn_mean"] },
```

İmzada parametre adı yazılmaz (adlar dile bağlıdır): konumlu parametreler tip ifadeleridir (`?` isteğe bağlı, `*` sıfır ya da çok, `+` bir ya da çok), adlı argümanlar `[kod, tip, varsayılan]` üçlüsüdür. Tanımlar davranışla sınanır: tanımdaki adlı argümanlar yerleşiğin gerçekten kabul ettikleridir (13.5).

**Ortak ad.** Uluslararası gösterimle ya da özel adla yazılan kavramın adı her dilde aynıdır (`sin`, `exp`, `log`, `max`, `argmax`, `relu`, `AdamW`, `pi`, `json`, `test`). Ad `definitions.json`'da bir kez yazılır (`"common_name": "sin"`), hiçbir dil dosyasında yazılmaz; yazılırsa yükleme hatasıdır. Yükleyici ortak adı her dile verir; ad o dilin adlarıyla aynı denetimlerden geçer. Karar (baş geliştirici, 2026-10-25/26): matematik fonksiyonlarının uluslararası adları ortak addır; Türkçe kalanlar `karekök`, `mutlak`, `ortalama`, `standart_sapma`.

**Ad alanları ve eş adlar.** Bir ad yalnız kendi ad alanında benzersizdir (genel, tip, üye, adlı argüman): `mutlak` hem yerleşik (`TR_fn_abs`) hem `Yol`'un metodudur (`TR_mem_absolute`). Yükleme yinelenen kodu, aynı ad alanında çakışan adı (ASCII biçimleri aynı olanlar dahil) ve geçerli bir adla çakışan yabancı adı reddeder. Bir ad birden çok koda yalnız aynı anlamda ya da ayrı alanda (komut satırı ve dil) verilir; her eş ad grubu gerekçesiyle `docs/terimler.md`'dedir (`internal/doc_terms_test.go`). Bu yüzden tensörleri uç uca ekleyen `bitiştir`, liste metodu `.birleştir`tir.

**Üye adları.** Metot ve özellik adları (`x.ad`) tanım ve kullanım yerinde tipe bakılmadan aynı biçimde koda eşlenir: kullanıcının yapısındaki alan ya da metot bir üye kodunun adıyla aynıysa (`yapı Sepet { fonksiyon ekle(...) }`) o da o kodla saklanır. Böylece tanım ve çağrı birlikte çevrilir. Hedef dilde kullanıcının bir adıyla çakışma olursa kullanıcı adı `$ad` kaçışıyla yazılır (2.11).

## Kodlu saklama

Her `.tr` dosyası diskte kodlu saklanır. Kişi dosyayı kendi dilinde görür ve düzenler; yazdığı metin o dilden koda çevrilir. Çevirici `internal/lang/source`'tadır:
- `tr kodla <dosya|klasör> [--dil <kod>]`: o dilde yazılmış metni yerinde kodlar; kodlu dosyaya dokunmaz.
- `tr göster <dosya> [--dil <kod>]`: dosyayı bir dilde standart çıktıya basar.
- `TR_cmd_format` kodlu dosyayı kodlu biçimde biçimler.

Kodlu dosya çevrilmeden çalışır (`tr dosya.tr`, `tr test`, `TR_cmd_check`, `TR_cmd_build`, `TR_kw_import`, hata ayıklayıcı).

```tr
//! tr kodlu 3 dil=tr
TR_kw_func kareler(liste) {
    TR_kw_return liste.TR_mem_map { TR_ctx_it * TR_ctx_it }
}
TR_fn_print(kareler([1, 2, 3]), TR_fn_sum([1, 2], TR_arg_dim: 0))
```

**17.1** Kodlu dosyanın (varsa BOM'dan sonraki) ilk satırı tam olarak `//! tr kodlu 3 dil=<dil>`dir: biçim sürümü 3, `<dil>` dosyanın yazım dili (`tr`, `en`), satır sonu LF ya da CRLF. Başka bir başlık (başka sürüm, dilsiz başlık, sondaki boşluk) dosyayı çalıştırmaz ve nedenini söyler; bilinmeyen dil de hatadır (`'fr' yazım dili yok; var olanlar: en, tr`). Başlığı olmayan dosya bir dilde yazılmış metindir ve dili belirtilerek kodlanır. Dil dosyanındır: `tr kodla - --dil en` → `dil=en`; dosya başka dilde gösterilip düzenlense de kayıtta `dil=` korunur. **Geçici (madde 15):** dili belirtilmemiş kodsuz metin bugün Türkçe sayılır; bilerek bozuk sınav dosyaları (`tests/negative`, `examples/tr/hatalar`), belgelerdeki kod blokları ve Go testlerindeki TR metni bu yolla okunur.

**17.2** Kodlu dosyada dilin her sözcüğü kimliğiyle yazılır: anahtar kelime, bağlamsal sözcük, tip ve yerleşik çıplak (`TR_kw_if`, `TR_fn_print(...)`), üye `.TR_mem_append(...)`, adlı argüman `TR_arg_lr: 0.003`. Kod olmayan her ad kullanıcının adıdır ve hiçbir zaman anahtar kelime sayılmaz (`TR_kw_var eğer = 7` geçerlidir). Kod listesinde olmayan `TR_` adı sözdizimi hatasıdır (`'TR_fn_yok' bilinen bir kimlik değil ...`). Metinler, sayılar ve yorumlar yazıldıkları gibi kalır. Kitaplığın dışa açık adları da koddur (11.6; kitaplığın dili yoktur): yapı, fonksiyon, sabit, alan, metot ve parametre (`TR_type_GPT`, `TR_type_Linear`, `TR_fn_lr_schedule`, `TR_mem_forward`, `TR_arg_embed_dim`); `definitions.json` dosyasında `modül` alanıyla işaretlidir, yerleşik değildir.

**17.3** Neyin kodlanacağını kapsam belirler (denetleyicinin kuralı, 5.12, 5.14):
- Anahtar kelimeler her zaman kodlanır.
- Değer olarak okunan ad, görünen bir kullanıcı tanımı yoksa yerleşik ya da tip adıysa kodlanır; kullanıcı gölgelediyse (`TR_kw_var toplam = 5`) o kapsamda kullanıcının adıdır. Takma adsız `TR_kw_import`ın altında yerleşik adı yerleşik sayılır.
- Bağlamsal sözcükler yalnız bağlamlarında kodlanır: `TR_ctx_test` deyim başında, `TR_ctx_as` `TR_kw_import`da, `TR_ctx_init` metot ve üye adı olarak, `TR_ctx_error` adsız `TR_kw_catch`da, `TR_ctx_it` ve `TR_ctx_acc` sondaki blokta (`(o) => o` kullanıcının adıdır).
- Üyeler "Üye adları" kuralıyla kodlanır (Y5), tanımda ve kullanımda aynı kodla.
- Sözlüğün çıplak anahtarı (`{adım: 100}`) bir üye koduysa o kodla yazılır ve `ayar.adım` ile okunur. İçeride anahtar kodu taşır, metni kodun dilden bağımsız adıdır: kimliğin İngilizce parçası (`TR_mem_step` → `"step"`), hiç değişmez (17.6). Bu yüzden Türkçe programın `{adım: 1}`'i de İngilizce programın `{step: 1}`'i de aynı anahtardır: `d["step"]`, `anahtarlar()` ve diske giden her biçim (json_metin, günlük, kontrol noktası) `"step"` görür; `yazdır` ve `metin()` anahtarı yazdıran dosyanın dilinde ve kaynaktaki gibi tırnaksız yazar, tırnak yalnız metin anahtardadır (Türkçede `yazdır({adım: 0, "renk": 1})` → `{adım: 0, "renk": 1}`, İngilizcede `{step: 0, "renk": 1}`: tırnaksız ad `d.adım` ile okunduğunu gösterir, `d["adım"]` bulmaz; Node.js `util.inspect` de tanımlayıcı anahtarları tırnaksız yazar); iletiler ve hata ayıklayıcı kişinin dilinde. Üye erişimi (`d.adım`) önce bu anahtarı, sonra erişimin dosyasının dilindeki adla duran metin anahtarı arar (o dilde yazılmış JSON için; başka hiçbir dil aranmaz, hiçbiri öncelikli değildir); yeni anahtar kodla eklenir. Kodsuz çıplak anahtar (`{renk: 1}`) ve metin anahtarlar (`{"adım": 1}`) çevrilmez. Karşılaştırma: Python ve JS'de anahtar metindir, `json.dumps` olduğu gibi yazar; yerelleştirilmiş adlar (Excel) içte tek kimliğe bağlıdır ve kayıtta kanonik adla yazılır; TR'nin yapı alanları da içeride kimlikle tutulup gösterimde çevrilir.
- **Dosya biçimleri dilden bağımsızdır.** TR'nin, motorların ve kitaplığın diske yazdığı dosyaların (kontrol noktası, günlük, `progress.json`, profil, `TR_cmd_build` paketi, veri önbelleği, `tr/egitim`'in `config.json`'u) dosya ve klasör adları, alan adları ve kayıt türleri İngilizce tanımlayıcılardır (`checkpoints/step_N/state.json`, `"kind": "resume"`); değeri bir dil öğesi olan alan kodunu taşır (`"optimizer": "TR_fn_AdamW"`). Programın verdiği adlar (ölçüler, ayarlar, nesne adları) programındır ve yukarıdaki kuralla yazılır. Eski biçimler okunmaz (dil taslaktır). Karşılaştırma: PyTorch `state_dict`/`optimizer.state_dict()` (`state`, `param_groups`, `step`, `exp_avg`) ve Hugging Face `trainer_state.json` (`global_step`, `log_history`), `config.json` (`vocab_size`) İngilizce ve dilden bağımsızdır. Sınamalar: `go test ./internal/stdlib/checkpoint -run CheckpointFormatIsLanguageIndependent`, `./internal/stdlib/logging -run LogFormatNames`, `./internal/devtools/profiler -run SnapshotKeys`, `./cmd/tr -run PackageKeys`, `./cmd/tr/e2e -run 'DictCodedKeysOnDisk|FileFormatsIndependent'`.
- Tip açıklamasındaki tip adı kodlanır (3.14).
- Kitaplığın yapı, fonksiyon ve sabit adı yerleşik gibi kodlanır, bildirildiği yerde de (`TR_kw_struct TR_type_Linear`); takma adla aktarılan modülün adı da (`k.GPT`). Kitaplık fonksiyonunun ve kitaplık yapısının metotlarının parametreleri bildirimde ve gövdedeki her kullanımda adlı argüman koduyla kodlanır; yerel değişkenler kullanıcının adıdır. Ağırlık adları kimlik yoludur (`TR_mem_blocks.0.TR_mem_attention.TR_mem_q_proj.TR_mem_weight`).
- Adlı argüman, çağrılan dilin öğesiyse kodlanır (`toplam(t, eksen: 0)` → `TR_fn_sum(t, TR_arg_dim: 0)`); kullanıcı fonksiyonunun adlı argümanı kullanıcının adıdır, kurucusuz yapıda alanın kodunu alır.

**17.4** Çeviri yeniden yazdırma değildir: yalnız dilin sözcüklerinin aralıkları değişir; boşluklar, satır düzeni, BOM, yorumlar, kullanıcı adları, metinler ve sayılar bayt bayt korunur; yerleştirmelerdeki sözcükler kodlanır, ham metin ve yorum çevrilmez. Her yazım dili için yazım → kodlu → yazım ve kodlu → yazım → kodlu bayt bayt aynıdır (`go test ./internal/lang/source`, `go test ./cmd/tr/e2e -run TestCodedFilesGiveSameOutput`). Çevirici sonucunu kendisi denetler: kodlu programın ağacı özgününkiyle aynı olmalı, çözülüp yeniden kodlanınca değişmemeli; değilse dosyaya dokunulmaz. Sözdizimi hatası olan ya da geçerli UTF-8 olmayan dosya çevrilmez.

**17.5** Satır ve sütunlar kişinin gördüğü gösterime göredir: satırlar başlıktan sonra sayılır, bir kod sonraki karakterlerin sütununu gösterimdeki adının uzunluğu kadar kaydırır; ileti, hata çerçevesi ve hata ayıklayıcı `TR_cmd_show`'in ya da düzenleyicinin gösterdiği yeri gösterir. Gösterim kişinin gösterim dilindedir (`TR_DIL`; düzenleyici kendi gösterim dilini yorumlayıcıya `TR_DIL` olarak verir): İngilizce okuyan için çerçeve İngilizce satırı gösterir, sütun o satırdaki yerdir. Satırlar her gösterimde aynıdır.

**17.6** Dil taslak aşamasındadır: kaldırılan ya da başka biçime taşınan kodun satırı silinir, eski adı için ipucu tutulmaz, depodaki kullanımlar yeni biçime çevrilir; listede olmayan kimlik hatadır. Başka dillerin alışkanlık adları (Python'un `str`, `range`; PyTorch'un `embedding`) bir TR'nin eski adı değildir: kod listesinin `familiar` alanından önerilir. Yayımdan sonra uyumluluk kuralı eklenecektir (kimlik yeniden kullanılmaz, kaldırılan ada ipucu). Kodlu dosyayı kıran değişiklik (kimliklerin yazımı, 17.3'ün kuralı, başlığın anlamı) yalnız biçim sürümünü artırarak yapılır. Yeni kod, ad değişikliği ve yeni yazım dili kırıcı değildir.

**17.7** Yazım dilleri: Türkçe (`tr`) ve İngilizce (`en`); hiçbiri asıl dil değildir, asıl biçim koddur. Her dil dosyası kod listesinin her kodunu adlandırır. Kodlar içeride kod olarak kalır: sözcük çözümleyici kimliği okur, ayrıştırıcı, denetleyici ve yorumlayıcı kimlikle çalışır (`codes.Active()` kimlik listesidir, Türkçe `codes.Source()` ile okunur; koruyan test `codes.TestProgramRunsOnIdentities`). Bu yüzden hangi yerleşiğin çağrıldığı, kullanıcının adları ve programın sonucu yazım diline bağlı değildir.
- **Programın anlamı koda, değerin metni dosyanın diline, görüntü kişinin diline bağlıdır.** Değerin metni (`doğru`/`true`, `yok`/`none`, `TR_fn_type()`, yazdırılan kodlu sözlük anahtarı) işlemin yazıldığı dosyanın dilindedir (3, 3.2). `TR_DIL` yalnız görüntüyü değiştirir: iletiler, yardım, kabuk, hata ayıklayıcı, düzenleyicideki görünüm. Dosya biçimleri dilden bağımsızdır.
- Karşılaştırma (2026-10-24/25): Python, Java, Swift ve Go değer metnini yerelleştirmez; Excel ve LibreOffice çalıştıranın diline göre değiştirir (aynı program farklı sonuç verir); Hedy ve Racket yazıldığı dile bağlar. TR yazıldığı dosyanın diline bağlar: İngilizce yazanın `str(b) == "true"` karşılaştırması tutar.
- Başka dilde yazılmış metin dili verilerek okunur: `tr kodla - --dil en`, `tr göster <dosya> --dil en` (`--dil` metnin dilidir, iletiler `TR_DIL`'dedir). Yalnız dilin sözcükleri çevrilir; hedef dilde dilin sözcüğü olarak okunacak kullanıcı adı `$ad` kaçışıyla yazılır (2.11). Kodlu → İngilizce → kodlu bayt bayt aynıdır (`go test ./cmd/tr/e2e -run 'TestEnglishWritingLanguage|TestRepoFilesEnglishLossless'`).

### Düzenleyicide

Dilin kuralı değil, düzenleyicinin davranışıdır. Kodlu dosya kişinin dilinde (`tr.writingLanguage`: `tr` ya da `en`) görünür, düzenlenir ve kaydedilince yine kodlu yazılır. Çeviriyi `tr` yapar: `tr göster - [--dil <kod>]` ve `tr kodla - [--dil <kod>]` standart girişten okur (çıkış 0 çevrildi, 2 ayrıştırılamadı, 1 başka hata).
- Başlık görünümde gizlidir; görünümün satırları iletiler ve hata ayıklayıcıyla aynıdır (17.5).
- Dosyanın dili (`dil=`) kayıtta korunur; yeni dosya yazıldığı dilde kodlanır.
- Kodsuz dosya kayıtta kişinin diliyle kodlanır. **Geçici (madde 15):** bugün kodsuz dosya Türkçe sayılır ve kodsuz Türkçe kaydedilir.
- Görünüm ayrıştırılamıyorsa (yarım kalmış kod) kayıt kayıpsızdır: metin yazıldığı gibi, başlıksız kaydedilir, dosya dilini de taşıyan bir işaretle "kodlanmayı bekliyor" olur, ilk geçerli kayıtta yeniden kodlanır. Kişinin yazdığı hiçbir şey kaybolmaz.
- Kodlu olduğu hâlde ayrıştırılamayan dosya (birleştirme çakışması) ham gösterilir ve olduğu gibi kaydedilir.
- Kısıtlı kipte çeviri yapılmaz: kodlu dosya salt okunur ve ham gösterilir.

Sınamalar: `go test ./cmd/tr/e2e -run 'TestEncodeShowStandardInput|TestExtensionJSTests'`, `go test ./internal/devtools/debugger -run TestDAPCodedFileView`.

## Komut satırı adları

**17.8** `tr` komutları, seçenekleri, kabuğun ve hata ayıklayıcının komutları da kodludur (`cmd`, `sub`, `opt`, `repl`, `dbg`) ve kendi ad alanlarındadır (`kontrol`: komut `TR_cmd_check`, seçenek `TR_opt_check`). `-` ile başlayan bilinmeyen argüman dosya adı sanılmaz, en yakın seçenek önerilir; adı `-` ile başlayan dosya `--`'dan sonra verilir. `TR_cmd_index` TR kodunun dizinine sorar: `TR_sub_definition`, `TR_sub_references`, `TR_sub_tested_by`, `TR_sub_impact`, `TR_sub_rename` (TR_sub_definition ... TR_sub_rename; İngilizcesi `definition`, `references`, `tested_by`, `impact`, `rename`); ad, konum (`dosya.tr:satır:sütun`), düğüm kimliği ya da kod kimliği (`TR_fn_sum`) olabilir; belirsiz ad çıkış 3, bulunamayan çıkış 4; `--json` kararlı şema yazar. Ayrıntı: [`internal/devtools/codeindex/README.md`](../../internal/devtools/codeindex/README.md).
- **Girdide her gömülü dilin adı kabul edilir:** `tr çalıştır` ile `tr run`, `--dil` ile `--lang` her makinede aynıdır; betik `TR_DIL`'e bağlı kalmaz. Komut satırının kodlu biçimi yoktur.
- **Çıktı kişinin dilindedir:** yardım ve iletiler adları `TR_DIL`'in dilinde yazar (`TR_DIL=en tr help` → `tr run <dosya.tr>`).
- **Ortam değişkenleri de kodludur** (`env`): `TR_DIL`/`TR_LANG`, `TR_IS_KLASORU`/`TR_JOB_DIR`, `TR_GPU_ESZAMANLI`/`TR_GPU_SYNC`, `TR_GPU_PROFIL`/`TR_GPU_PROFILE`, `TR_CPU_AVX2`, `TR_GPU_CUBLASLT`. Her dilin adıyla okunur, hiçbir dilin adı öncelikli değildir: iki ad aynı değerle verilebilir, ayrı değerle verilirse `tr` başlamadan durur (H2527). Ad kodlarını göremeyen hesap çekirdeği değişkeni kimlik adıyla okur (`TR_env_GPU_SYNC`); `tr` başlarken bu adı koyar (`codes.ExportEnvironment`). Adlar `TR_[A-Z0-9_]+` biçimindedir. Yalnız geliştiricinin ve testlerin kullandığı değişkenler kod almaz (`internal/lang/codes/environment_test.go`).
- **Seçenek yazımı:** iki tire; sözcükler tireyle ayrılır (`--go-profili`); kısa bayraklar (`-h`, `-v`) simgedir.
- **Kabul edilmeyenler ipucu verir:** Türkçe harfsiz biçim (`tr calistir`), başka araçların kısaltmaları (`n`, `c`, `bt`), `--help`, `--version` asıl adı önerir; kaldırılmış seçenek yerine geçeni söyler.
- **Programın seçenekleri** (`TR_fn_command_line`, §19) sözlüğün anahtarlarıdır; kodlu anahtarın seçeneği her gömülü dilin adıyla geçer, yardım kişinin dilindeki adı yazar (`{TR_mem_step: 100}` → `--adım` ve `--step`; 17.3); kodsuz anahtar yazıldığı gibidir. Yalnız `--yardım`/`--help` ayrılmıştır.

Karşılaştırma: Go, git ve cargo komut adlarını tek (İngilizce) yazımda tutar; Excel ve PowerShell'in yerelleştirilmiş adları içte bir kimliğe bağlıdır ama yalnız etkin dili kabul eder. TR ikisini birleştirir: kimlik koddadır, girdide bütün dillerin adı geçerlidir, çıktı kişinin dilindedir.

Sınamalar: `go test ./internal/lang/codes -run CommandLineNamesInEveryLanguage`, `go test ./cmd/tr/e2e -run 'CommandsHaveOneName|REPL$'`, `go test ./internal/devtools/debugger -run 'TerminalKeysMatchVSCode|CommandsSingleSpelling'`, `go test ./internal/stdlib/process -run CommandLine`.
