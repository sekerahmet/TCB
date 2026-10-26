# 17. Kodlar

Dilin asıl biçimi koddur. Kullanıcıya görünen her ad (anahtar kelime, bağlamsal sözcük, tip, yerleşik, metot, özellik, adlı argüman, komut) sabit bir **koda** bağlıdır. Dosyalar diskte, program içeride bu kodlarla çalışır. Türkçe ve İngilizce yalnız gösterim dilleridir, hiçbirinin ayrıcalığı yoktur.

- **Dosyalar.** Dilden bağımsız tanımlar `internal/lang/codes/definitions.json`'dadır; her dilin adları ve açıklamaları kendi dosyasındadır (`tr.json`, `en.json`; yeni diller aynı biçimde, ISO 639-1: `es.json`, `pt-BR.json`). Hepsi ikiliye gömülüdür. Her kodun işi, imzası ve örneği [`docs/codes.md`](../codes.md)'dedir (`go run ./tools/gen/codesdoc` üretir). Go kodu adları değil kodları kullanır (`codes.FnPrint`; `go generate ./internal/lang/codes`). Bir adı değiştirmek dil dosyasındaki tek satırı değiştirmektir.
- **Kimlik.** Kodun yazımı `TR_<sınıf>_<İngilizce ad>`dır (`TR_kw_if`, `TR_fn_sum`, `TR_type_Tensor`, `TR_arg_lr`, `TR_mem_step`). Bir kez verilir ve hiç değişmez; ad değişse de kimlik kalır, yalnız anlam değişince yeni kimlik açılır (17.6). Sınıf ad alanını belirler (`step`: `TR_dbg_step`, `TR_arg_step`, `TR_mem_step`). İki kimlik yalnız harf büyüklüğüyle ayrılamaz; gerekirse sahibinin adı eklenir (`TR_arg_G_muon`, `TR_arg_g_swiglu`). `TR_` ile başlayan adlar kimliklere ayrılmıştır, kullanıcı adı olamaz (2.3a).

| Sınıf | Kapsam | Ad alanı |
| :-- | :-- | :-- |
| `kw` | anahtar kelimeler (`TR_kw_if` eğer) | genel |
| `ctx` | bağlamsal sözcükler (`olarak`, `test`, `kurucu`, `hata`, `o`, `birikim`) | genel |
| `val` | değer adları (`TR_val_inf` sonsuz): kaynakta sözcük değildir, kodlanmaz (3.2) | genel |
| `type` | tip adları (`TR_type_Int` Tam) | tip |
| `fn`, `const` | yerleşik fonksiyonlar ve sabitler (`TR_fn_print` yazdır, `TR_const_pi` pi) | genel |
| `arg` | adlı argümanlar (`TR_arg_dim` eksen) | adlı argüman |
| `mem` | metot ve özellikler, `x.ad` (`TR_mem_length` uzunluk) | üye |
| `cmd`, `sub`, `opt` | `tr` komutları, `tr dizin`'in alt komutları, seçenekler (17.8) | komut; alt komut; seçenek |
| `repl`, `dbg` | kabuğun ve hata ayıklayıcının komutları | komut; hata ayıklayıcı |
| `env` | ortam değişkenleri (`TR_env_LANG` `TR_DIL`) | ortam |

İletiler kimlik değil `H` ve dört basamaklı numaradır (`internal/lang/messages`): `H0` sözdizimi · `H1` tip denetimi · `H2` yorumlayıcı · `H3` tensör · `H4` GPU · `H5` veri ve parçalayıcı · `H6` ml · `H7` standart kitaplık · `H8` araçlar · `H9` iç hatalar. Parametreler adlıdır (`{fn}: {path\|q} bir klasör`); ileti ve içindeki değerler kişinin dilinde (`TR_DIL`) yazılır.

**Dil dosyası.** Her kod bir satırdır, sınıf sırasıyla ve sınıf içinde kimliğe göre dizilir:

```json
"TR_fn_print": { "ad": "yazdır", "aile": "giriş ve çıkış", "açıklama": "Değerleri ... yazar ...", "örnek": "yazdır(\"Toplam:\", 2 + 3)" },
```

- `ad` kodun o dildeki tek yazımıdır (2.9). ASCII biçimi (`yazdir`) kabul edilmez, yalnız ipucunda doğru adı önermek için kullanılır (`codes.AcceptASCII` kapalı).
- `yabancı` başka dillerden alışılmış adlardır (`"TR_arg_seed": { "ad": "tohum", "yabancı": ["seed"] }`): kabul edilmez, hata iletisi doğru adı önerir (`AdamW 'lr' adında bir parametre almaz. Bunu mu demek istediniz: 'öğrenme_oranı'?`). Her dil kendi okurunun alışkanlıklarını yazar.
- `aile` kısa bir grup adı, `açıklama` bir-iki cümle, `örnek` tek satırlık çalışan TR kodudur. Her kodun açıklaması, her canlı kodun örneği vardır (`go test ./internal/lang/codes ./internal/stdlib`).

**Tanımlar** (`definitions.json`, dilden bağımsız): kodun türü (`anahtar_kelime`, `bağlamsal`, `değer_adı`, `tip`, `fonksiyon`, `sabit`, `metot`, `özellik`, `adlı_argüman`, `komut`), metot ve özelliklerin geçerli olduğu tipler, imza, ilişkili kodlar ve varsa ortak ad:

```json
"TR_fn_sum": { "tür": "fonksiyon", "param": ["TR_type_List|TR_type_Tensor", "TR_type_Int?"], "adlı": [["TR_arg_dim", "TR_type_Int", null], ["TR_arg_keepdim", "TR_type_Bool", false]], "dönüş": "TR_type_Int|TR_type_Float|TR_type_Tensor", "ilişkili": ["TR_fn_mean"] },
```

İmzada parametre adı yazılmaz (adlar dile bağlıdır): konumlu parametreler tip ifadeleridir (`?` isteğe bağlı, `*` sıfır ya da çok, `+` bir ya da çok), adlı argümanlar `[kod, tip, varsayılan]` üçlüsüdür. Tanımlar davranışla sınanır: tanımdaki adlı argümanlar yerleşiğin gerçekten kabul ettikleridir (13.5).

**Ortak ad.** Uluslararası gösterimle ya da özel adla yazılan kavramın adı her dilde aynıdır (`sin`, `exp`, `log`, `max`, `argmax`, `relu`, `AdamW`, `pi`, `json`, `test`). Ad `definitions.json`'da bir kez yazılır (`"ortak_ad": "sin"`), hiçbir dil dosyasında yazılmaz; yazılırsa yükleme hatasıdır. Yükleyici ortak adı her dile verir; ad o dilin adlarıyla aynı denetimlerden geçer. Karar (baş geliştirici, 2026-10-25/26): matematik fonksiyonlarının uluslararası adları ortak addır; Türkçe kalanlar `karekök`, `mutlak`, `ortalama`, `standart_sapma`.

**Ad alanları ve eş adlar.** Bir ad yalnız kendi ad alanında benzersizdir (genel, tip, üye, adlı argüman): `mutlak` hem yerleşik (`TR_fn_abs`) hem `Yol`'un metodudur (`TR_mem_absolute`). Yükleme yinelenen kodu, aynı ad alanında çakışan adı (ASCII biçimleri aynı olanlar dahil) ve geçerli bir adla çakışan yabancı adı reddeder. Bir ad birden çok koda yalnız aynı anlamda ya da ayrı alanda (komut satırı ve dil) verilir; her eş ad grubu gerekçesiyle `docs/terimler.md`'dedir (`internal/doc_terms_test.go`). Bu yüzden tensörleri uç uca ekleyen `bitiştir`, liste metodu `.birleştir`tir.

**Üye adları.** Metot ve özellik adları (`x.ad`) tanım ve kullanım yerinde tipe bakılmadan aynı biçimde koda eşlenir: kullanıcının yapısındaki alan ya da metot bir üye kodunun adıyla aynıysa (`yapı Sepet { fonksiyon ekle(...) }`) o da o kodla saklanır. Böylece tanım ve çağrı birlikte çevrilir. Hedef dilde kullanıcının bir adıyla çakışma olursa kullanıcı adı `$ad` kaçışıyla yazılır (2.11).

## Kodlu saklama

Her `.tr` dosyası diskte kodlu saklanır. Kişi dosyayı kendi dilinde görür ve düzenler; yazdığı metin o dilden koda çevrilir. Çevirici `internal/lang/source`'tadır:
- `tr kodla <dosya|klasör> [--dil <kod>]`: o dilde yazılmış metni yerinde kodlar; kodlu dosyaya dokunmaz.
- `tr göster <dosya> [--dil <kod>]`: dosyayı bir dilde standart çıktıya basar.
- `tr çöz <dosya|klasör>`: kodlu dosyayı Türkçe yazıma çevirir (geçici, kaldırılacak).
- `tr biçimle` kodlu dosyayı kodlu biçimde biçimler.

Kodlu dosya çevrilmeden çalışır (`tr dosya.tr`, `tr test`, `tr kontrol`, `tr derle`, `aktar`, hata ayıklayıcı).

```tr
//! tr kodlu 3 dil=tr
TR_kw_func kareler(liste) {
    TR_kw_return liste.TR_mem_map { TR_ctx_it * TR_ctx_it }
}
TR_fn_print(kareler([1, 2, 3]), TR_fn_sum([1, 2], TR_arg_dim: 0))
```

**17.1** Kodlu dosyanın (varsa BOM'dan sonraki) ilk satırı tam olarak `//! tr kodlu 3 dil=<dil>`dir: biçim sürümü 3, `<dil>` dosyanın yazım dili (`tr`, `en`), satır sonu LF ya da CRLF. Başka bir başlık (başka sürüm, dilsiz başlık, sondaki boşluk) dosyayı çalıştırmaz ve nedenini söyler; bilinmeyen dil de hatadır (`'fr' yazım dili yok; var olanlar: en, tr`). Başlığı olmayan dosya bir dilde yazılmış metindir ve dili belirtilerek kodlanır. Dil dosyanındır: `tr kodla - --dil en` → `dil=en`; dosya başka dilde gösterilip düzenlense de kayıtta `dil=` korunur. **Geçici (madde 15):** dili belirtilmemiş kodsuz metin bugün Türkçe sayılır; bilerek bozuk sınav dosyaları (`tests/negative`, `examples/tr/hatalar`), belgelerdeki kod blokları ve Go testlerindeki TR metni bu yolla okunur.

**17.2** Kodlu dosyada dilin her sözcüğü kimliğiyle yazılır: anahtar kelime, bağlamsal sözcük, tip ve yerleşik çıplak (`TR_kw_if`, `TR_fn_print(...)`), üye `.TR_mem_append(...)`, adlı argüman `TR_arg_lr: 0.003`. Kod olmayan her ad kullanıcının adıdır ve hiçbir zaman anahtar kelime sayılmaz (`TR_kw_var eğer = 7` geçerlidir). Kod listesinde olmayan `TR_` adı sözdizimi hatasıdır (`'TR_fn_yok' bilinen bir kimlik değil ...`). Metinler, sayılar ve yorumlar yazıldıkları gibi kalır. Kitaplığın adları (`GPT`, `Doğrusal` ..., 11.6) bugün kod değildir, kullanıcı adı gibi kalır (madde 15'te kodlanacak).

**17.3** Neyin kodlanacağını kapsam belirler (denetleyicinin kuralı, 5.12, 5.14):
- Anahtar kelimeler her zaman kodlanır.
- Değer olarak okunan ad, görünen bir kullanıcı tanımı yoksa yerleşik ya da tip adıysa kodlanır; kullanıcı gölgelediyse (`değişken toplam = 5`) o kapsamda kullanıcının adıdır. Takma adsız `aktar`ın altında yerleşik adı yerleşik sayılır.
- Bağlamsal sözcükler yalnız bağlamlarında kodlanır: `test` deyim başında, `olarak` `aktar`da, `kurucu` metot ve üye adı olarak, `hata` adsız `yakala`da, `o` ve `birikim` sondaki blokta (`(o) => o` kullanıcının adıdır).
- Üyeler "Üye adları" kuralıyla kodlanır (Y5), tanımda ve kullanımda aynı kodla.
- Sözlüğün çıplak anahtarı (`{adım: 100}`) bir üye koduysa o kodla yazılır ve `ayar.adım` ile okunur. Çalışma anında anahtarın metni dosyanın dilindedir (İngilizce dosyada `"step"`, Türkçede `"adım"`); üye erişimi anahtarı kodun herhangi bir dildeki adıyla bulur. Kodsuz çıplak anahtar (`{renk: 1}`) ve metin anahtarlar (`{"adım": 1}`) çevrilmez. Kitaplığın ve motorların yazdığı dosyaların anahtarları (config.json, kontrol noktası, günlük) bugün Türkçedir; dilden bağımsız olmaları madde 15'in işidir.
- Tip açıklamasındaki tip adı kodlanır (3.14).
- Adlı argüman, çağrılan dilin öğesiyse kodlanır (`toplam(t, eksen: 0)` → `TR_fn_sum(t, TR_arg_dim: 0)`); kullanıcı fonksiyonunun adlı argümanı kullanıcının adıdır, kurucusuz yapıda alanın kodunu alır.

**17.4** Çeviri yeniden yazdırma değildir: yalnız dilin sözcüklerinin aralıkları değişir; boşluklar, satır düzeni, BOM, yorumlar, kullanıcı adları, metinler ve sayılar bayt bayt korunur; yerleştirmelerdeki sözcükler kodlanır, ham metin ve yorum çevrilmez. Her yazım dili için yazım → kodlu → yazım ve kodlu → yazım → kodlu bayt bayt aynıdır (`go test ./internal/lang/source`, `go test ./cmd/tr/e2e -run Kodlu`). Çevirici sonucunu kendisi denetler: kodlu programın ağacı özgününkiyle aynı olmalı, çözülüp yeniden kodlanınca değişmemeli; değilse dosyaya dokunulmaz. Sözdizimi hatası olan ya da geçerli UTF-8 olmayan dosya çevrilmez.

**17.5** Satır ve sütunlar kişinin gördüğü gösterime göredir: satırlar başlıktan sonra sayılır, bir kod sonraki karakterlerin sütununu gösterimdeki adının uzunluğu kadar kaydırır; ileti, hata çerçevesi ve hata ayıklayıcı `tr göster`'in ya da düzenleyicinin gösterdiği yeri gösterir. **Geçici (madde 15, görüntü):** bugün konumlar ve çerçeve Türkçe gösterime göredir; başka dildeki gösterimde satırlar aynıdır, sütunlar kayabilir.

**17.6** Dil taslak aşamasındadır: kaldırılan ya da başka biçime taşınan kodun satırı silinir, eski adı için ipucu tutulmaz, depodaki kullanımlar yeni biçime çevrilir; listede olmayan kimlik hatadır. Yayımdan sonra uyumluluk kuralı eklenecektir (kimlik yeniden kullanılmaz, kaldırılan ada ipucu). Kodlu dosyayı kıran değişiklik (kimliklerin yazımı, 17.3'ün kuralı, başlığın anlamı) yalnız biçim sürümünü artırarak yapılır. Yeni kod, ad değişikliği ve yeni yazım dili kırıcı değildir.

**17.7** Yazım dilleri: Türkçe (`tr`) ve İngilizce (`en`); hiçbiri asıl dil değildir, asıl biçim koddur. Her dil dosyası kod listesinin her kodunu adlandırır. Kodlar içeride kod olarak kalır: sözcük çözümleyici kimliği okur, ayrıştırıcı, denetleyici ve yorumlayıcı kimlikle çalışır (`codes.Active()` kimlik listesidir, Türkçe `codes.Source()` ile okunur; koruyan test `codes.TestProgramRunsOnIdentities`). Bu yüzden hangi yerleşiğin çağrıldığı, kullanıcının adları ve programın sonucu yazım diline bağlı değildir.
- **Programın anlamı koda, değerin metni dosyanın diline, görüntü kişinin diline bağlıdır.** Değerin metni (`doğru`/`true`, `yok`/`none`, `tip()`, kod taşıyan sözlük anahtarı) işlemin yazıldığı dosyanın dilindedir (3, 3.2). `TR_DIL` yalnız görüntüyü değiştirir: iletiler, yardım, kabuk, hata ayıklayıcı, düzenleyicideki görünüm. Dosya biçimleri dilden bağımsızdır.
- Karşılaştırma (2026-10-24/25): Python, Java, Swift ve Go değer metnini yerelleştirmez; Excel ve LibreOffice çalıştıranın diline göre değiştirir (aynı program farklı sonuç verir); Hedy ve Racket yazıldığı dile bağlar. TR yazıldığı dosyanın diline bağlar: İngilizce yazanın `str(b) == "true"` karşılaştırması tutar.
- Başka dilde yazılmış metin dili verilerek okunur: `tr kodla - --dil en`, `tr göster <dosya> --dil en` (`--dil` metnin dilidir, iletiler `TR_DIL`'dedir). Yalnız dilin sözcükleri çevrilir; hedef dilde dilin sözcüğü olarak okunacak kullanıcı adı `$ad` kaçışıyla yazılır (2.11). Kodlu → İngilizce → kodlu bayt bayt aynıdır (`go test ./cmd/tr/e2e -run Ingilizce`).

### Düzenleyicide

Dilin kuralı değil, düzenleyicinin davranışıdır. Kodlu dosya kişinin dilinde (`tr.yazimDili`: `tr` ya da `en`) görünür, düzenlenir ve kaydedilince yine kodlu yazılır. Çeviriyi `tr` yapar: `tr göster - [--dil <kod>]` ve `tr kodla - [--dil <kod>]` standart girişten okur (çıkış 0 çevrildi, 2 ayrıştırılamadı, 1 başka hata).
- Başlık görünümde gizlidir; görünümün satırları iletiler ve hata ayıklayıcıyla aynıdır (17.5).
- Dosyanın dili (`dil=`) kayıtta korunur; yeni dosya yazıldığı dilde kodlanır.
- Kodsuz dosya kayıtta kişinin diliyle kodlanır. **Geçici (madde 15):** bugün kodsuz dosya Türkçe sayılır ve kodsuz Türkçe kaydedilir.
- Görünüm ayrıştırılamıyorsa (yarım kalmış kod) kayıt kayıpsızdır: metin yazıldığı gibi, başlıksız kaydedilir, dosya dilini de taşıyan bir işaretle "kodlanmayı bekliyor" olur, ilk geçerli kayıtta yeniden kodlanır. Kişinin yazdığı hiçbir şey kaybolmaz.
- Kodlu olduğu hâlde ayrıştırılamayan dosya (birleştirme çakışması) ham gösterilir ve olduğu gibi kaydedilir.
- Kısıtlı kipte çeviri yapılmaz: kodlu dosya salt okunur ve ham gösterilir.

Sınamalar: `go test ./cmd/tr/e2e -run 'KodlaGosterStandartGiris|EklentiJSTestleri'`, `go test ./internal/devtools/debugger -run Kodlu`.

## Komut satırı adları

**17.8** `tr` komutları, seçenekleri, kabuğun ve hata ayıklayıcının komutları da kodludur (`cmd`, `sub`, `opt`, `repl`, `dbg`) ve kendi ad alanlarındadır (`kontrol`: komut `TR_cmd_check`, seçenek `TR_opt_check`). `-` ile başlayan bilinmeyen argüman dosya adı sanılmaz, en yakın seçenek önerilir; adı `-` ile başlayan dosya `--`'dan sonra verilir. `tr dizin` TR kodunun dizinine sorar: `tanım`, `kullananlar`, `sınayanlar`, `etki`, `yeniden_adlandır` (TR_sub_definition ... TR_sub_rename; İngilizcesi `definition`, `references`, `tested_by`, `impact`, `rename`); ad, konum (`dosya.tr:satır:sütun`), düğüm kimliği ya da kod kimliği (`TR_fn_sum`) olabilir; belirsiz ad çıkış 3, bulunamayan çıkış 4; `--json` kararlı şema yazar. Ayrıntı: [`internal/devtools/codeindex/README.md`](../../internal/devtools/codeindex/README.md).
- **Girdide her gömülü dilin adı kabul edilir:** `tr çalıştır` ile `tr run`, `--dil` ile `--lang` her makinede aynıdır; betik `TR_DIL`'e bağlı kalmaz. Komut satırının kodlu biçimi yoktur.
- **Çıktı kişinin dilindedir:** yardım ve iletiler adları `TR_DIL`'in dilinde yazar (`TR_DIL=en tr help` → `tr run <dosya.tr>`).
- **Ortam değişkenleri de kodludur** (`env`): `TR_DIL`/`TR_LANG`, `TR_IS_KLASORU`/`TR_JOB_DIR`, `TR_GPU_ESZAMANLI`/`TR_GPU_SYNC`, `TR_GPU_PROFIL`/`TR_GPU_PROFILE`, `TR_CPU_AVX2`, `TR_GPU_CUBLASLT`. Her dilin adıyla okunur; ikisi birden verilirse Türkçe ad geçerlidir. Adlar `TR_[A-Z0-9_]+` biçimindedir. Yalnız geliştiricinin ve testlerin kullandığı değişkenler kod almaz (`internal/lang/codes/environment_test.go`).
- **Seçenek yazımı:** iki tire; sözcükler tireyle ayrılır (`--go-profili`); kısa bayraklar (`-h`, `-v`) simgedir.
- **Kabul edilmeyenler ipucu verir:** Türkçe harfsiz biçim (`tr calistir`), başka araçların kısaltmaları (`n`, `c`, `bt`), `--help`, `--version` asıl adı önerir; kaldırılmış seçenek yerine geçeni söyler.
- **Programın seçenekleri** (`komut_satırı`, §19) sözlüğün anahtarlarıdır (17.3: İngilizce betikte `{step: 100}` → `--step`); yalnız `--yardım`/`--help` ayrılmıştır.

Karşılaştırma: Go, git ve cargo komut adlarını tek (İngilizce) yazımda tutar; Excel ve PowerShell'in yerelleştirilmiş adları içte bir kimliğe bağlıdır ama yalnız etkin dili kabul eder. TR ikisini birleştirir: kimlik koddadır, girdide bütün dillerin adı geçerlidir, çıktı kişinin dilindedir.

Sınamalar: `go test ./internal/lang/codes -run CommandLineNamesInEveryLanguage`, `go test ./cmd/tr/e2e -run 'CommandsHaveOneName|REPL$'`, `go test ./internal/devtools/debugger -run 'TerminalKeysMatchVSCode|CommandsSingleSpelling'`, `go test ./internal/stdlib/process -run CommandLine`.
