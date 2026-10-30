# DURUM — Faz 1, 4a2 ilk dilim (dal `claude/upbeat-hypatia-c890s3`)

- Başlama: 2026-10-30 13:14 UTC. Taban: `main` `5c90d096`.
- Sıra: 1 (13.10) → 2b (argüman sayısı) → 2a (metodu olmayan tip) → 4 (3.10) → 3 (dönüş tipi).

## Adım 1 — değeri kullanılmayan çağrı (13.10): bitti

- **Tanım dosyası:** `definitions.json`'a `pure` (mantık değeri). Okuyan kod `codes.Definition.Pure`; ölçüt `definition.go`'nun alan yorumunda ve `internal/lang/codes/README.md`'de. `CheckDefinitions` `pure`'u yalnız yerleşik fonksiyon ve metotta kabul ediyor. `definition.go` sınırdaydı (540/600): `CheckDefinitions` `definition_check.go`'ya taşındı (davranış aynı).
- **`pure` işaretli kodlar (39):**
  - 13.7'nin hesapları (21): `TR_floor`, `TR_ceil`, `TR_sign`, `TR_clamp`, `TR_is_finite`, `TR_exp`, `TR_log`, `TR_sin`, `TR_cos`, `TR_atan2`, `TR_acos`, `TR_asin`, `TR_atan`, `TR_tan`, `TR_sinh`, `TR_cosh`, `TR_tanh`, `TR_expm1`, `TR_log10`, `TR_log1p`, `TR_log2`.
  - 13.2–13.4'ün yeni değer veren metotları (18): `.TR_contains`, `.TR_ends_with`, `.TR_starts_with`, `.TR_filter`, `.TR_get`, `.TR_join`, `.TR_keys`, `.TR_values`, `.TR_lower`, `.TR_upper`, `.TR_map`, `.TR_reduce`, `.TR_replace`, `.TR_reverse`, `.TR_sort`, `.TR_split`, `.TR_strip`, `.TR_unique`.
  - İşaretsiz (ölçüt dışı): `TR_input`, `TR_print`, `.TR_append`, `TR_load_parameters`, `TR_sleep`, `TR_time_ms`, dosya ve tensör işlemleri. `TR_random` ve `g.TR_wait()` katalogda yok.
- **Kural:** `pure` yerleşiğin çağrısı tek başına deyimse **H1115** (`internal/lang/checker/pure_calls.go`). Yerleşik sözcüğün kodundan (`Token.Code`) bulunur; tipi bilinen kullanıcı yapısının metodu ve gölgelenmiş ad sayılmaz; bloğun değeri ve kabuk satırı sayılmaz. İleti dosyadaki çağrıyı alıp atamayı önerir (`l = l.TR_sort()`); alıcı atanabilir adsa (`TR_var`) uygulanabilir düzeltme (`WithFix`: `l` → `l = l`), değilse (`f().TR_sort()`, sabit, fonksiyon) ``<ad> = f().TR_sort()`` biçimi.
- **Testler:** `TestPureCallValueUnused` (checker; 13.2, 13.3, 13.4, 13.7, ad olmayan alıcı, sabit; işaretsiz `.TR_append`, kullanılan değer, bloğun değeri hatasız), `TestPureMarksOnlyValueBuiltins` (codes; şema), öneri tablosu satırı (H1115), TR testi `tests/yerlesikler_test.tr` "13.10 ...", olumsuz fikstür `tests/negative/degeri_atilan_cagri.tr`, 13.10'un Test satırı.
- **Sayım:** kural açılmadan önce depodaki `.tr` dosyalarında (olumsuz fikstürler hariç) bu kalıpta 0 yer [ölçüldü: `tr check` her dosyada]. Go testlerindeki TR programlarında ve belge bloklarında da yeni hata çıkmadı (paket testleri).

## Adım 2b — argüman sayısı (13.11): bitti

- **Kural:** tipi bilinen alıcıda (`TR_String`, `TR_List`, `TR_Dict`, `TR_Range`) kodla yazılmış yerleşik metodun konumlu argüman sayısı tanımın imzasına (`params`; `?` isteğe bağlı, `*`/`+` sınırsız; `codes.Signature.PositionalCounts`) uymazsa denetim hatası (`internal/lang/checker/builtin_calls.go`). Adlı argümanlar 13.5'in işi, sayılmadı. Tensör ve kitaplık tiplerinin metotları sayılmıyor: motorlarının sayısı tanımla sınanmadı.
- **Kod:** çalışma anındaki **H2011** (yeni kod yok). İleti tek kaynak: `hint.MemberArgCount` (çalışma anı `interp/methods.go` ve denetleyici aynı fonksiyonu çağırıyor). Ad ve kullanım kimlikle: ".TR_sort() takes 0 arguments, 1 given (usage: \`.TR_sort()\`)"; görünüm adı (`list.sort`) ve `<nil>` yok. Kullanımın parametre adları (`eski, yeni`, `[ayırıcı]`) `interp`'teki `signature.parameters`'tan `hint`'in `memberForms` tablosuna taşındı (iletinin dilinde; önceden `messages.Text` ile etkin dilde donuyordu, İngilizce iletide Türkçe çıkıyordu).
- **(c) sıralama anahtarı:** `.TR_sort()`'a argüman verilince H2011'in yeni `hint` parametresi **H1116**: anahtarlı sıralama henüz yok, bugünkü yol `[(f(l[i]), i) TR_for i TR_in 0..l.TR_length].TR_sort().TR_map((p) => l[p.1])` (çalışır, öneri tablosunda). `TR_for i, x TR_in` liste üretecinde sözdizimi hatası, bu yüzden indeksle yazıldı.
- **Ön şart (sayı):** `TestMethodCountsMatchDefinitions` (interp): bütün çekirdek metotların (Metin, Liste, Sözlük, Aralık) motor sayısı tanımla aynı; uyuşmazlık yok.
- **Testler:** `TestMemberArgCount` (checker; 5 alıcı/metot, anahtarlı sıralama, doğru sayı, `TR_JSON`, isteğe bağlı alıcı hatasız), `TestMemberArgCountUsesIdentities` (interp; çalışma anı iletisinde görünüm adı ve `<nil>` yok), öneri tablosunda H2011'in demet yolu, olumsuz fikstür `tests/negative/metot_arguman_sayisi.tr`. `compiler_test` ve `hooks_test`'in beklenen metni yeni biçime güncellendi; `docs/guide/03_zincirleme_metotlar.md` cümlesi güncellendi.
- **Yeni hatalar (depo):** 5, hepsi **testin konusu**: `tests/yerlesikler_test.tr` "13.2 metotlar argüman sayısını ve tipini denetler" testinde `TR_error_code { "abc".TR_contains() }` gibi çalışma anı sayı hatasını bekleyen 5 satır (110, 112, 114, 116, 117). Artık denetim hatası oldukları için olumsuz fikstüre taşındı; TR testinin adı "13.2 metotlar argümanın tipini denetler" oldu. Bulunan yanlış: 0.

## Adım 2a — metodu olmayan tipte çağrı (13.11): bitti

- **Ön şart:** `TestMemberTypesMatchDefinitions` (stdlib) bütün alıcı tiplerine genişletildi (`TR_JSON` nesne/liste/metin, `TR_Bytes`, yokluk, ileti, tip, yapı nesnesi ve yapı tipi). Uyuşmayan üç kod (`.TR_get`, `.TR_keys`, `.TR_length`; "Bulunan yanlış") `checker.MembersBeyondDefinitions`'ta, kurala girmiyor. Kural ön şart geçtikten sonra açıldı.
- **Kural:** tipi bilinen alıcı (denetleyicinin tanıdığı çekirdek tip: Metin, Liste, Sözlük, Aralık, Tensör, Tam, Ondalık, Mantık, Fonksiyon, Tip; isteğe bağlı değil) ve kodla yazılmış, kitaplığın olmayan yerleşik metot alıcının tipinde yoksa (tanımın `types`/`signatures`'ı) denetim hatası (`builtin_calls.go`, `reportMissingMember`).
- **Kod:** çalışma anındaki **H2474** ("property or method "join" not found on type String"); `hint` parametresi yeni **H1117**: metodun hangi tiplerde olduğu (`typeArg` biçimiyle) ve, genel kural olarak, tek argümanın tipinde varsa ve o tipte bir argüman alıyorsa yer değiştirme: `" ".TR_join(l)` → `` `l.TR_join(" ")` `` (alıcı ve argüman dosyadaki yazımlarıyla).
- **Testler:** `TestMissingMember` (checker; yer değiştirme, Liste'de `.TR_strip`, Aralık'ta ve Tensör'de `.TR_sort`, argümanın tipinde de olmayan metotta yer değiştirme yok; doğru tip, `TR_JSON` alıcı, isteğe bağlı alıcı hatasız), öneri tablosunda H2474'ün yer değiştirmesi (öneri kopyalanınca program denetimden ve çalışmadan geçiyor), olumsuz fikstür `tests/negative/metodu_olmayan_tip.tr`.
- **Yeni hatalar (depo):** 0 (`.tr` dosyaları `tr check` ile; Go testleri ve belge blokları paket testleriyle).
- 2b'nin commit'inde `docs/status.md`'ye yazdığım `round()` kimliksiz addı `TestReviewedSectionsWriteIdentities`'i düşürüyordu; bu commit'te düzeldi.

- 2b'nin commit'inde `MembersBeyondDefinitions`'ın İngilizce gerekçe metinlerindeki çıplak tip adı (`JSON`) `TestNoBareNames`'i (tools/gen/codeconsts) düşürüyordu; değer artık tip kodlarının listesi.

## Kalan

## Kararsız

- Fonksiyonların H2011 iletisi (`ArgCountError`, ör. `round()`) hâlâ görünüm adıyla; yalnız metot yolu kimliğe geçti (görevin sınırı metodun iletisi). Fonksiyonların sayı denetimi de bu dilimde yok.

- `.TR_map`, `.TR_filter`, `.TR_reduce` `pure` işaretli (görev listesi böyle diyor). Fonksiyon argümanı yan etkili olabilir (`l.TR_map((x) => TR_print(x))`); bugün bu da H1115. Ölçüt "argümanlarını değiştirmez" diyor, yan etkili fonksiyonu konuşmuyor.
- 13.1'in hesapları (`TR_sum`, `TR_mean`, `TR_min`, `TR_max`, `TR_abs`, `TR_sqrt`, `TR_round` ...) ölçüte uyuyor ama 13.10'un listesinde yok; işaretlenmedi.
- `.TR_split` metinde ve tensörde aynı kod; kod düzeyinde işaret tensör alıcısını da kapsıyor (tensör işlemleri bu adımda yoktu; sonuç yine de yalnız değer).

## Bulunan yanlış

- **Tanım dosyasının `types`'ı eksik (2a'nın ön şartı):** `TestMemberTypesMatchDefinitions` `TR_JSON`'u, `TR_Bytes`'ı, yokluğu, iletiyi, tipi, yapıyı ve nesnesini kapsamıyordu; genişletildi. Motor `.TR_get` ve `.TR_length`'i `TR_JSON` ve `TR_Bytes`'ta, `.TR_keys`'i `TR_JSON`'da da destekliyor; tanım yalnız Metin/Liste/Sözlük diyor. Düzeltilmedi: üç kod `checker.MembersBeyondDefinitions`'ta, metodu olmayan tip kuralına (2a) girmiyor; test listeyle tanımın uyuşmamasını bekliyor (tanım düzelince listeden çıkar). Denetleyici `TR_JSON` ve `TR_Bytes`'ı bugün tanımadığı için yanlış alarm zaten çıkmazdı.
- **13.3 ile motor çelişiyor:** dil tanımı `.TR_sort(f)`'i (anahtarlı sıralama) kural olarak yazıyor; motor ve tanım dosyası `.TR_sort()`'u argümansız veriyor. Görev "sıralama anahtarı ayrı dil kararı" dediği için ileti "henüz yok" diyor; status 13.3'e yazıldı.

## Sorular

- `tools/codegraph`'ın `TestEveryTrackedDataFileHasATestOrAReason` testi bu dalda yalnız `.devir/DURUM.md` yüzünden düşüyor (izlenen, teste bağlı olmayan veri dosyası; `uncovered_data.txt` yalnız küçülür). 4a1 dalında da aynı; birleştirmede `.devir` dışarıda kalırsa geçer. Başka bir çözüm isteniyorsa söyleyin.

## Codegraph

- **Eksik (adım 1, kendi hatam da):** adım 1'in commit'inden önce `go test ./cmd/...` koşmadım; `tests-for definition.go`/`checker.go`'nun `run:` satırları `cmd/tr/e2e`'yi (belge blokları `TestDocTRCodeBlocks`, eklenti şablonları `TestExtensionTemplatesBuild`) saymıyordu, oysa yeni denetim kuralı oradaki TR kodunu da denetliyor. Adım 1'in commit'i (`661d62b`) bu iki testte kırıktı; 2b'nin commit'inde düzeltildi (belge bloğu `TR_print` ile, şablon testi metot şablonunu atamanın sağına yazıyor).

- Adım 1: yanlış ya da yanıltıcı cevap yok. `tests-for definition.go` dosyanın sınırda olduğunu doğru söyledi (540/600, `ParseDefinitions` 99/105); taşıma sonrası `LongFuncs` istisnasının yolu (`internal/layoutrules/exceptions.go`, `internal/layout_exceptions_test.go`) güncellendi.

## İkinci dilime

- imzaların öğe ve fonksiyon tipleri, 13.10'un sekiz kısıtı, birleşim dönüşleri;
- metot ve kurucu argümanlarının tip denetimi (bu dilimde yalnız sayı);
- 6.6'nın beklenen tipten çıkarımı;
- alana atama (7.5), kapanış, çoklu tanımın öğe tipleri, kitaplık tiplerinin tanınması;
- tensör işlemlerinin `pure` işareti.
