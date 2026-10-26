# Ekleme tarifleri

TR'ye yeni bir şey eklerken hangi dosyalara, hangi sırayla dokunulduğu ve "bitti"nin nasıl anlaşıldığı. Kuralların kendisi (adlar, iletiler, katmanlar, test düzeni) ve onları koruyan testler [`internal/README.md`](../internal/README.md)'dedir; süreç (yeni ad isteği, tam denetim, ölçüm) [`development.md`](development.md)'dedir. Bu belge ikisini adım adım bir araya getirir. Her tarif, depoda o yoldan eklenmiş gerçek bir örneğe dayanır.

## Her eklemede ortak

1. **Önce başka dillere bak.** Eklenecek şeyin başka dillerde ve çerçevelerde (Python, Swift, Kotlin, Rust, Go, PyTorch, JAX ...) nasıl çözüldüğünü kısa bir tabloyla karşılaştır: hangi davranış, hangi ad, hangi hata. Yerleşik olanı TR'ye uyarla; hiçbiri uymuyorsa gerekçesiyle yeni bir yöntem seç. Tabloyu ve kararı dil tanımı maddesine ya da commit mesajına yaz.
2. **Önce var olana bak.** Aynı işi yapan bir ad ([`codes.md`](codes.md)), bir yardımcı (`bind`, `turkish.FileError`, `interp.ArgCountError`) ya da bir ileti (`internal/lang/messages/tr.json`) varsa onu kullan. Aynı işi yapan iki kod olmaz. Aramayı kod grafiğiyle yap ([`tools/codegraph`](../tools/codegraph/README.md)): benzer bir örneğin dokunduğu dosyaları `query <ad>`, çağıranlarını ve testlerini `tests-for <fonksiyon>` verir; değiştireceğin şeyin etki alanını da böyle gör.
3. **Ad koddur, ileti koddur.** Kullanıcının yazacağı her ad kod listesinden bir kod alır (numarası ailesinin kuralıyla verilir, adı baş geliştirici onaylar). Kullanıcının göreceği her ileti paketin aralığından bir ileti kodu alır (ileti kodunu işi yapan ekler). Kullanıcıya görünen her yeni ad bir örnekte geçer: dilin adları `examples/tr`'nin bir örneğinde (`TestExamplesCoverNames`), `lib/*.tr`'nin dışa açık adları da öyle (`TestExamplesCoverLibraryNames`); örnek yoksa yazılır.
4. **Testsiz ekleme yoktur.** Her davranışın bir testi olur. Bir düzeltmenin testi düzeltmeden önce kalır, sonra geçer.
5. **Belge işle birlikte değişir.** Dil davranışı değiştiyse dil tanımında (`docs/spec/`) bir madde, kullanıcı için anlatmak gerekiyorsa kılavuzda (`docs/guide/`) bir satır.
6. **Bitti = tam denetim.** `bash tools/full_check.sh` geçer. Sıcak yola dokunulduysa önce ve sonra ölçülür.

---

## 1. Eleman bazlı matematik fonksiyonu

Örnek: `silu` (TR_fn_silu). Sayıda `Ondalık`, tensörde türev izleyen tensör verir; CPU ve GPU'da, 16 bitte de çalışır.

| Adım | Dosya | Ne eklenir |
| :-- | :-- | :-- |
| Ad | `internal/lang/codes/tr.json` | `"TR_fn_silu": { "family": ..., "description": ..., "example": "yazdır(silu(tensör([-1.0, 0.0, 2.0])).değer)" }` (ad yok: `silu` ortak addır; ortak adı olmayan kodda `"name": ...` ilk alandır, `en.json`'a da `{ "ad": ... }` satırı girer) |
| İmza | `internal/lang/codes/definitions.json` | `"TR_fn_silu": { "kind": "function", "common_name": "silu", "params": ["TR_type_Tensor\|TR_type_Int\|TR_type_Float"], "returns": "TR_type_Tensor\|TR_type_Float" }` |
| Üret | — | `go generate ./internal/lang/codes`: Go sabitleri, `docs/codes.md`, düzenleyici renklendirmesi |
| İşlem | `internal/compute/tensor/device.go` | `UnOp` listesinin sonuna, `numUnOps` sayacının **önüne** `OpSilu` (sayaç her zaman son kalır; kapsam testi işlemleri onunla sayar) |
| Türev testi | `internal/compute/tensor/elementwise_wide_test.go` | `registerGrad` ile bir `gradSpec` kaydı: float64 başvuru işlevi ve türevi |
| Ad | `internal/compute/tensor/elementwise.go` | `unopName` tablosuna `OpSilu: "silu"` |
| Yöntem | `internal/compute/tensor/elementwise_wide.go` | `func (t *Tensor) Silu() *Tensor { return unary(t, OpSilu, 0) }` |
| CPU | `internal/compute/tensor/cpu_elementwise_wide.go` | ileri kolu (`unaryLoopWide`) ve türev kolu (`unaryGradLoopWide`) |
| CPU okuma | `internal/compute/tensor/cpu_dtype.go` | türev x'i mi y'yi mi okuyor: `unaryGradReads` |
| Profil | `internal/compute/tensor/counter.go`, `internal/devtools/profiler/names.go` | `OpKindSilu` ve onun kodu (`codes.FnSilu`) |
| GPU | `internal/compute/gpu/kernel_wide.go` | ileri ve türev kolu (PTX); kararlı biçim (ör. `x·σ(x)`'te büyük \|x\|) |
| Dile bağlama | `internal/stdlib/ml/builtin_math.go` | `unit(v, codes.FnSilu, (*tensor.Tensor).Silu, silu64)` ve sayı karşılığı `silu64` |

Testler:

| Dosya | Sınadığı |
| :-- | :-- |
| `internal/compute/tensor/elementwise_wide_test.go` | ileri değerler (NaN, ±∞, −0 dahil) ve türevin sayısal türevle uyuşması (`checkGrad`) |
| `internal/compute/tensor/elementwise_wide_test.go` (`gradSpec` kaydı) | float64 başvuruya karşı sonlu fark gradyanı ve bf16 gradyan toleransı (3e-2); kayıt yoksa `behavior_grad_coverage_test.go` düşer; sınır değerlerde türev `TestUnaryGradEdges` |
| `internal/compute/gpu/backend_wide_test.go` | GPU'nun CPU ile aynı sonucu vermesi, bf16/fp16 |
| `internal/stdlib/ml/builtin_math_test.go` | sayıda ve tensörde dönüş tipi, listede açık hata |
| `tests/matematik_test.tr` | dil kuralı: `test "15.11 ..."` (madde numarasıyla başlar) |
| `tr.json`'daki örnek | kendiliğinden tip denetiminden geçer ve çalışır (`internal/lang/codes`, `internal/stdlib`) |

Belge: `docs/spec/15_tensorler.md`'de ilgili madde (15.11 genişletilmiş eleman bazlı fonksiyonlar).

Bitti: NaN ve sonsuzlar tanımlı; türev sınırlarda doğru; 16 bit tensörde tür korunuyor; GPU testi benzeticide ve gerçek kartta geçiyor.

---

## 2. Standart kitaplığa yeni yerleşik fonksiyon

Örnek: `csv_oku` (TR_fn_read_csv), `internal/stdlib/formats`.

1. **Ad ve imza.** `tr.json`'a ad, açıklama, çalışan tek satırlık örnek; `definitions.json`'a tür, konumlu parametreler (`"TR_type_String|TR_type_Path"`), adlı argümanlar (`[kod, tip, varsayılan]`), dönüş. Adlı argüman da bir koddur (`arg` sınıfı: `TR_arg_dim`). Sonra `go generate ./internal/lang/codes`.
2. **Kayıt.** Paketin `Register(v *interp.VM)` fonksiyonunda:

   ```go
   v.RegisterNoNamed(codes.FnReadCsv, func(vm *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
       if len(args) != 1 {
           return nil, interp.ArgCountError(codes.FnReadCsv, "1", len(args), messages.Text(messages.H2321)) // "dosya ya da metin"
       }
       ...
   })
   ```

   Adlı argüman alan yerleşik `RegisterBuiltinCode` ile kaydedilir ve argümanları `bind.CheckNamed(fn, named, nameX, nameY)` ile denetler (yanlış yazılan ada en yakın adı önerir); değerleri `bind.GetNamed` ile okur.
3. **Argümanlar.** Tip açıkça denetlenir (`*interp.StringVal` ...), görünüşe (`Inspect()`) güvenilmez. Mantık için `bind.Bool`, sayı için `bind.Int`.
4. **İletiler.** Paketin aralığından (`formats`: `H70xx`) kodlar: `messages.New(messages.H7003, fn, path)`. Dosya hataları `turkish.FileError(fn, yol, messages.H2100, err)`.
5. **Kilit ve sınırlar.** Disk, ağ ve ağır hesap `vm.Blocking(...)` içinde; TR değerleri kilit altında kurulur. Kullanıcının boyutunu belirlediği her ayırma `interp.MaxCollectionSize` ile sınırlanır.
6. **Yeni paket ise** bkz. tarif 5.

Testler: paketin birim testi (`formats_test.go`): davranış ve hatalar, hatalarda `messages.Has(err, messages.H7003)`. Dil kuralı: `tests/<bölüm>_test.tr` (madde numarasıyla). Kod listesi örneği kendiliğinden çalışır; `internal/stdlib/definitions_test.go` tanımdaki adlı argümanların gerçekten kabul edildiğini sınar. Dış girdi okuyorsa bkz. tarif 7.

---

## 3. Bir tipe yeni metot ya da özellik

Örnek: `SatırOkuyucu.sonraki()` (`internal/stdlib/files/lines.go`).

1. **Ad.** Üye kodu (`TR_mem_...`); `definitions.json`'da `"kind": "method"` ya da `"property"` ve hangi tiplerde geçerli olduğu (`"signatures": { "TR_type_LineReader": {...} }`).
2. **Kanca.** Tip, yorumlayıcının arayüzünü uygular: üyeler için `GetProperty(kd codes.Code, prop string) (interp.Value, error)` (`interp.PropertyGetter`). Yineleme, indeksleme, işleçler için `interp.Iterable`, `interp.Indexable`, `interp.OperatorHook` ...
3. **Derleme anı denetimi.** Her kanca için: `var _ interp.PropertyGetter = (*LineReader)(nil)`. Adı değişen bir metot kancayı sessizce koparmasın.
4. **Bulunamayan üye** `bind.MemberError(ne, ad, üyeler)` ile en yakın üyeyi önerir.

Testler: tipin dosyasının birim testi; metot hem doğru hem yanlış argümanla sınanır.

---

## 4. Yeni ileti ya da hata

1. **Aralık.** Paketin aralığından sıradaki boş numara (bkz. [`internal/README.md`](../internal/README.md#kullanıcı-iletileri)). Ortak bir kalıp (`H2000` `{fn}: {error}`, dosya hataları, satır öneki) varsa yenisi açılmaz.
2. **Katalog.** `internal/lang/messages/definitions.json`: `"H7006": { "param": ["fn", "path"] }` (İngilizce tanımlayıcılar); `tr.json`: `"H7006": "{fn}: {path|q} ..."`. Kullanıcı verisi `{ad|q}` ile; düz süslü parantez `{{ }}`. Her kod bir satır, artan sırada. Dilbilgisi şablondadır: sayıya göre sözcük `{n, plural, one {..} other {..}}`, hâl eki `{ad|dat}`, seçenek listesi `messages.OrList`; Go kodu sözcük seçmez, ek eklemez, cümleyi parçalardan kurmaz (ayrıntı: [`internal/lang/messages/README.md`](../internal/lang/messages/README.md)).
3. **Üret.** `go run ./tools/gen/msgconsts`.
4. **Kullan.** Hata: `messages.New(messages.H7006, fn, path)`. Çıktı: `messages.Text(...)`. Konumlu hata: `messages.At(satır, sütun, ...)`. Yalnızca TR'nin kendi hatasıyla oluşabilen durum: `messages.Internal("English detail")`.
5. **Test.** `messages.Has(err, messages.H7006)`; metne değil koda bakılır.

Denetimler: aynı metin iki kodda olamaz (bağlamı farklı etiketler gerekçeli izinle), her çağrının değer sayısı tanımla aynı, kullanılmayan kod kalmaz, geçirilmiş pakette `fmt.Errorf`/`errors.New` yok.

İleti olmayan metin koda bağlanmaz: veri biçimi anahtarları (günlükteki `"kayıp"`, `durum.json` alanları), dosya adları, protokol alanları.

---

## 5. Yeni standart kitaplık paketi

Örnek: `internal/stdlib/formats`.

1. **Klasör ve ad.** `internal/stdlib/<ad>/`, küçük harf ASCII. Katmanı: `lang`, `compute` ve öbür `stdlib` paketlerini içe aktarabilir; araçları bilmez.
2. **Paket belgesi.** Ana dosyada `// Package <ad> ...` görevini söyler ve şu satırla biter: `// Layer: standard library (internal/stdlib); ...`.
3. **Kayıt.** `func Register(v *interp.VM)`; `internal/stdlib/engines.go`'da `interp.AddEngine(<ad>.Register)`. Kapatılması gereken kaynak varsa `Close`/`CloseAll` ve `engines.go`'daki `Close`.
4. **README.** `internal/stdlib/<ad>/README.md`: görev, `## Standartlar` (yalnızca kalıcı kurallar), davranış testi varsa `## Testler`.
5. **İletiler.** Paket için bir ileti aralığı (`H7xyy`) seçilir ve paket README'sine yazılır. Paket `tools/gen/msgconsts/main_test.go`'daki `migrated` listesine eklenir: `fmt.Errorf` baştan yasak.
6. **Belgeler.** `docs/architecture.md`'deki katman ve paket haritası.

Testler: `x_test.go` her kaynak dosyanın yanında; birden çok dosyayı kesen davranış `behavior_<konu>_test.go` ve README'de bir satır; paylaşılan yardımcılar `helpers_test.go`.

---

## 6. Yeni dil kuralı ya da anahtar kelime

Örnek: `eşle` (TR_kw_match).

1. **Tasarım ve ad.** Dil tanımında yeni madde (`docs/spec/`); ad kodu (`kw` anahtar kelime ya da `ctx` bağlamsal sözcük; kimlik İngilizce adından: `TR_kw_match`), adı baş geliştiricinin onayıyla. Kodlu saklama da adı kimlikle yazar, bu yüzden ad sonradan değişebilir.
2. **Sözdizimi.** `internal/lang/lexer` (gerekiyorsa yeni sözcük), `internal/lang/ast` (düğüm), `internal/lang/parser` (ayrıştırma; hata `messages.At(satır, sütun, messages.New(...))`).
3. **Anlam.** `internal/lang/checker` (tip denetimi ve uyarılar), `internal/lang/interp` (derleme ve çalışma; sıcak yolda ad araması yok).
4. **Araçlar.** Biçimlendirici (`internal/devtools/formatter`), düzenleyici renklendirmesi (`go generate ./internal/lang/codes`), gerekiyorsa hata ayıklayıcı.

Testler: `tests/<bölüm>_test.tr` (her test madde numarasıyla başlar; olumsuz durumlar `tests/negative/`), ayrıştırıcı ve denetleyicinin birim testleri, biçimlendiricinin "anlam korunur" testi. Dil kuralı Go testinde yinelenmez.

---

## 7. Dış girdi okuyan özellik

Örnek: `tensör_yükle` (`.trt` dosyası), `kontrol_noktası`.

1. **Güvenilmeyen girdi** sayılır: bozuk ya da kötü niyetli olabilir.
2. **Sınırlar:**
   - her boy alanı okunmadan önce sınırlanır (`interp.MaxCollectionSize`, dosyada kalan bayt),
   - derinlik sınırlıdır,
   - döngüsel değer `interp.ContainerWalker` ile yakalanır,
   - sıfır uzunluklu eksen sınırda 1 sayılır.
3. **Hata** açık ve yakalanabilirdir; çökme, sonsuz döngü, sınırsız bellek yoktur.
4. **Bulanıklaştırma testi:** `Fuzz<Ad>` (paketin `x_fuzz_test.go`'su); düşüren girdiler `testdata/fuzz/` altında kalır.
5. **`SECURITY.md`**'nin "Güvenilmeyen girdiler" tablosuna bir satır.

---

## 8. Hıza dokunan değişiklik

1. **Önce ölç:** aynı makinede, makine boşken, değişiklikten önce. CPU için `tools/bench/`, GPU için `tools/gpu/` (Colab tarifi: `tools/gpu/COLAB.md`).
2. **Değiştir ve yeniden ölç.** Kazandırmayan değişiklik geri alınır.
3. **Sonuçlar bit düzeyinde aynı olmalı:** kayıp ve çıktı. Değişiyorsa nedeni açıklanır (ör. yuvarlama sırası).
4. **Kod tek bir karta göre ayarlanmaz;** blok boyutları ve yollar parametrik kalır.
5. **Sıcak yoldaki fonksiyon** boyu ancak ölçümle bölünür.

---

## 9. Bir kavramı ortak ada çevirmek

Örnek: `TR_fn_sin` (`sinüs` → `sin`, 2026-10-26). Uluslararası gösterimle ya da özel adla yazılan bir kavramın adı her dilde aynıdır (dil tanımı 17, "Ortak ad"); adı baş geliştirici onaylar.

| Adım | Ne yapılır |
| :-- | :-- |
| Kullanımları kodla | Ad değişecekse (bugünkü adlar dillerde farklıysa) veriden **önce**: kodsuz `.tr` dosyaları `tr kodla <dosyalar>`, belgelerin ```tr blokları ve satır içi kodu `tr kodla -` ile eski adlarla kodlanır (çağrı ve başvuru kimliğe döner, yorum ve düzyazı bayt bayt kalır). Sözcükle arayıp değiştirme yapılmaz. |
| Veri | Tek betikle: `definitions.json` satırına `"common_name": "sin"`; `tr.json` satırından `"name"` düşer (`description`, `example`, `family` kalır); `en.json`'da yalnız ad taşıyan satır silinir; ortak adla aynı `foreign` girdisi silinir. |
| Geri yaz | `tr çöz <dosyalar>`, `tr göster - --dil tr`; `git diff --word-diff` yalnız ad çiftlerini göstermeli. Elle: Go test dizgilerindeki TR kodu, fonksiyonu anan yorumlar, `tr.json`'un açıklamaları, `.tr` testlerinin beklediği ileti metinleri. |
| Gölgeleme | Yeni adı yerel ad ya da parametre olarak kullanan yer yerleşiği gölgeler (2.9a): depodaki her kodsuz `.tr` dosyası `tr kontrol` ile koşulur, uyarı aranır. |
| Üret, sına | `go generate ./internal/lang/codes`; ad çakışması sınavı `-update` ile yalnız satır siler (yeni satır çıkarsa elle, nedeniyle). |

## Bitti kontrol listesi

- [ ] Ad ve ileti kodları katalogda; `go generate` / `msgconsts` çalıştırıldı.
- [ ] Birim testleri kaynağın yanında; davranış testleri README'de anılmış.
- [ ] Hatalar kodla denetleniyor (`messages.Has`).
- [ ] Dil davranışı değiştiyse dil tanımı ve `tests/*.tr` güncel.
- [ ] Dış girdi okuyorsa sınırlar, bulanıklaştırma testi, `SECURITY.md`.
- [ ] Sıcak yola dokunulduysa önce/sonra ölçüm.
- [ ] `bash tools/full_check.sh` geçti.
