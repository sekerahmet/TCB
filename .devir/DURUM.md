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

## Kalan

## Kararsız

- `.TR_map`, `.TR_filter`, `.TR_reduce` `pure` işaretli (görev listesi böyle diyor). Fonksiyon argümanı yan etkili olabilir (`l.TR_map((x) => TR_print(x))`); bugün bu da H1115. Ölçüt "argümanlarını değiştirmez" diyor, yan etkili fonksiyonu konuşmuyor.
- 13.1'in hesapları (`TR_sum`, `TR_mean`, `TR_min`, `TR_max`, `TR_abs`, `TR_sqrt`, `TR_round` ...) ölçüte uyuyor ama 13.10'un listesinde yok; işaretlenmedi.
- `.TR_split` metinde ve tensörde aynı kod; kod düzeyinde işaret tensör alıcısını da kapsıyor (tensör işlemleri bu adımda yoktu; sonuç yine de yalnız değer).

## Bulunan yanlış

## Sorular

## Codegraph

- Adım 1: yanlış ya da yanıltıcı cevap yok. `tests-for definition.go` dosyanın sınırda olduğunu doğru söyledi (540/600, `ParseDefinitions` 99/105); taşıma sonrası `LongFuncs` istisnasının yolu (`internal/layoutrules/exceptions.go`, `internal/layout_exceptions_test.go`) güncellendi.

## İkinci dilime

- imzaların öğe ve fonksiyon tipleri, 13.10'un sekiz kısıtı, birleşim dönüşleri;
- metot ve kurucu argümanlarının tip denetimi (bu dilimde yalnız sayı);
- 6.6'nın beklenen tipten çıkarımı;
- alana atama (7.5), kapanış, çoklu tanımın öğe tipleri, kitaplık tiplerinin tanınması;
- tensör işlemlerinin `pure` işareti.
