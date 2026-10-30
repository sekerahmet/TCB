# DURUM — Faz 0.9: TR kodunun tipli yazıma göçü

Başlama: 2026-10-30 04:50 UTC. Bitiş: (aşağıda, `# BİTTİ` ile). Dal: `claude/vibrant-feynman-b6x2dz`, taban `main` 862d848 (origin/main bir commit ilerde: 1ec69a3, debugger/vscode; bu işin dosyalarına dokunmuyor).

## Birleştirme (cherry-pick) notu

Main'e girmeyecek dört commit: tanığın iki commit'i ve onları geri alan iki commit: `09bb06d`, `a4f8d65`, `ef62e15` (revert), `1a9adf3` (revert). Atlanırsa sonuç aynıdır; `.devir/` zaten alınmıyor (`667b8c1` yalnız DURUM'dur). `09bb06d` `.devir/DURUM.md`'yi de kurmuştu; geri almada DURUM korundu.

## Biten

| Alan | Commit | Not |
| :-- | :-- | :-- |
| bulunan yanlış: sözcük çözümleyici | `afcfc5a` | satır sonundaki `?` işleç sayılıyordu (aşağıda) |
| lib | `a392e62` (demet), `a5f14d0` (tipler) | 325 parametre, 97 alan, 75 dönüş |
| tests | `75b8db0` (demet), `b9124e0` (tipler) | olumsuz fikstürler dahil |
| examples | `264cd88` (demet: 35, 36), `61e6f7d` (tipler) | hata örnekleri de tipli |
| tools/gpu, tools/bench | `e8b6a27` (demet), `891138e` (tipler) | GPU'suz; yalnız `tr check` |
| tanım dosyası, belge blokları, şablonlar | `a4d64a9` (TR_List örneği), `6a56f92` | `docs/codes.md` ve `snippets.json` üreteçle |
| Go testleri | `47a63f1` (gpuprobe demet), `4a1e37c` | yalnız dil davranışı ve kitaplık testleri |
| depo testi | `0c5506d` | `TestTRCodeIsTyped` + izin listesi |
| status | `0aa4e63` | 6.1, 7.1 (ve 5, 7 girişindeki iki cümle) |

(Bütün liste: `git log --oneline 862d848..HEAD`.)

## Sayılar

**Önce** [ölçüldü: ayrıştırıcıyla, `TestTRCodeIsTyped`'in ilk koşusu; lib + tests + examples + tools (codegraph hariç) + 20 belgede 361 ` ```tr ` bloğu, `docs/codes.md` dahil]:
- tipsiz parametre: **477** (lib 325) — [1. tur]: 473
- tipsiz alan: **170** (lib 97) — [1. tur]: 97 (yalnız lib saymış olmalı: lib'in sayısı tutuyor)
- değer döndürüp `->` yazmayan fonksiyon: **199**
- öğe tipi yazılmamış `TR_List`/`TR_Dict`: **9**
- tip yerine yazılmış genel tip (`TR_Any`, `TR_Object`, çıplak `TR_Function`, `TR_BuiltinFunction`): **7**
- toplam 862

**Sonra:** 0 + izin listesindeki **46** yer (`internal/testdata/typed_code_known.txt`): 29 parametre, 5 alan, 7 dönüş, 5 genel tip; öğe tipi 0.
- 4c'yi bekleyen (genel tip): 44
- olumsuz fikstür: 1
- 4a'yı bekleyen (denetleyici): 1

## Tip tanığı (geçici, geri alındı)

- **Nasıl:** `TR_TYPE_WITNESS=<dosya>` ile açılan, yorumlayıcıda çağrı anında çalışan karşılaştırma (`internal/lang/interp/type_witness.go`, `09bb06d` + `a4f8d65`; `ef62e15` ve `1a9adf3` geri aldı). Her kullanıcı fonksiyonu çağrısında bağlanan argümanı ve dönen değeri, yapı ve durum alanlarını (kurmada ve `nesne.alan = …` atamasında) yazılan tiple karşılaştırır: `TR_List[T]`'nin öğeleri (ilk 2000), `TR_Dict[K, V]`'nin anahtar ve değerleri, demetin öğeleri, `T?`'de yokluk, fonksiyon tipinde fonksiyon olması, yapı/durumlu tip/arayüz (arayüzde metotların varlığı), motor tipleri (`TR_Path`, `TR_Optimizer`…). Tensörün cinsi bugün motorda yok (bütün tensörler float32, 15.1): tam sayılı işaretli tensör `Int`, ilk 4096 değeri tam olan `Integral`, yalnız 0/1 olan `Bool` sayılır; `TR_Tensor[TR_Int]`'e kesirli değerli, `TR_Tensor`'a tam sayılı işaretli, `TR_Tensor[TR_Bool]`'a 0/1 dışı tensör uyuşmazlıktır. `TR_Int` değer `TR_Float` yerine verilince uyuşmazlıktır; yalnız tam sayı değişmezi (`f(1)`, `f(x: 0)`, varsayılan `= 0`) değildir (3.13). Uyuşmazlık "dosya:satır: tür ad: yazılan T, gelen …" satırı olarak yazılır. Aynı commit'te `TR_TYPE_PROFILE=<dosya>` gözlem kipi: her parametrenin, dönüşün ve alanın gerçek tipini (öğe tipleriyle) yazar; tipler buradan, çağrı yerlerinin hepsinden okunup elle gözden geçirildi.
- **Koşulan:** `tr test tests`, `tr test lib`, `tr test examples`, `examples/tr/*.tr` ve `helpers/` (e2e testi gibi temiz bir kopyada, girdisiz), `tools/bench/tr/*.tr`, belgelerin ` ```tr ` blokları (dosyaya yazılıp `tr run`), tanım dosyasının örnekleri (`go test ./internal/stdlib -run 'TestExamplesRun|TestLibrary'`, süreç içi), tiplediğim Go testleri (`go test ./internal/lang/interp ./internal/stdlib/gpuprobe`). Gözlem 1209 ayrı yeri kapsadı (699 parametre, 161 alan, 349 dönüş). GPU araçları (`tools/gpu/`) burada koşulamadı.
- **Son sayı: 1 uyuşmazlık**, o da olumsuz testin bilerek verdiği argüman: `lib/training_test.tr:73` `TR_SpikeGuard(TR_history_limit: 2.5)` (H6930'u, yani pencerenin tam sayı olmadığını çalışırken sınıyor; aşağıda "olumsuz fikstür"). Onun dışında 0.

## Bulunan yanlış

1. **Sözcük çözümleyici (düzeltildi, `afcfc5a`):** yapı gövdesinde satır sonundaki isteğe bağlı tipin `?`'si "değer sürüyor" işleci sayılıyordu (`expectsOperand`): `TR_k_proj: TR_Linear?`'den sonraki satırın kimliği üye değil öbür yer okunuyor, kodlu dosyada art arda iki isteğe bağlı alan ayrışmıyordu ("bir sözlük anahtarı olamaz"). `?` TR'de yalnız tipin sonudur (`??` ve `?.` ayrı belirteç). Lexer testi eklendi (düzeltmesiz düşüyor).
2. **Karışık tipli liste döndüren fonksiyonlar (düzeltildi, demete çevrildi; 3.6, 6.3):** `lib/layers.tr` `GPT.göm` (`[x, bağlamlar]`), `lib/optimizers.tr` `_grup_momentumu` (`[sıra, güncellemeler]`), `lib/optimizers_test.tr` `eğit` (`[m, op]`), `tests/` iki GPU yardımcısı (15.41), `examples/tr/35`, `36` toplu iş (`[girdiler, hedefler]`), `tools/gpu/ss_exam.tr` `farklı_4` ve `açgözlü`, `internal/stdlib/gpuprobe` kitaplık testi (`[kayıplar, c]`). Çağıranlar demeti açıyor; çıktılar aynı (35/36 ve gpuprobe karşılaştırıldı; ss_exam GPU'suz koşulamadı). `GPT.göm`'ün dönüşü kitaplığın dışa açık metodu (`TR_mem_embed`): liste yerine demet; depoda başka kullanan yok [ölçüldü: git grep].
3. **`TR_List`'in tanım örneği ve açıklaması karışık tipliydi (düzeltildi, `a4d64a9`):** örnek `[1, "iki", 3.0]`, açıklama "karışık tipli dizi" diyordu; `TR_List[TR_Int] = [1, 2, 3]` ve "tek tipli" oldu, `docs/codes.md` üretildi.
4. **Denetleyici (düzeltilmedi, 4a):** yapının adı değer olarak kullanılınca denetleyici onu yapının kendi tipinde sayıyor (`c.scope.Define(s.Name, TypeInfo{TypeName: s.Name})`); dil tanımına göre tipi `TR_Type`'tır (3.11). `TR_yalnızca_2b(TR_Muon, ...)`'ın ad parametresine `TR_Type` yazılınca "TR_Type bekliyor ancak TR_Muon verildi" denetim hatası. Parametre tipsiz, izin listesinde ayrı başlıkla.
5. **Kimlik ve konum tensörleri ondalıkla kuruluyor (düzeltilmedi, 15.2 ile):** tipleri `TR_Tensor[TR_Int]` yazıldı (15.1: kimlik dizisi), ama kurulumları ondalık: `lib/layers.tr` `TR_Tensor([i * 1.0 …])` (KVÖnbellek.konumlar), `üret`'te `TR_Tensor([[kimlikler[-1] * 1.0]])` ve `TR_Tensor([ö.TR_position * 1.0])`, `examples/tr/45_tekrar_oynat.tr` ve `47_kendi_uretim_dongusu.tr` aynı kalıp. Bugün bütün tensörler float32 olduğu için fark yok; 15.2 cinsi veriden çıkarınca bunlar `TR_Tensor[TR_Float]` olur ve tipe uymaz: o göçte `* 1.0` kalkar ya da `TR_Tensor[TR_Int](...)` yazılır.
6. **Maske çarpımla birleşiyor (düzeltilmedi, 15.6 ile):** `KVÖnbellek.maske` dönüşü `TR_Tensor[TR_Bool]`, ama iki maskeyi `m * (...)` ile birleştiriyor; 15.1'e göre `TR_Bool` tensör yalnız `& | ^ == !=` ile işlenir. Tensörde `&` bugün yok (çalışma hatası "desteklenen işleçler: + - * / @ ** …") [ölçüldü].
7. **Çalışma anı tip sınamaları:** `TR_SpikeGuard.TR_init` ve `TR_Run.TR_init` parametrenin tam sayı olduğunu `TR_Type(x) != TR_Int` ile çalışırken sınıyor (H6930, H6932). Parametreler artık `TR_Int`; 4a1'de yanlış tipli argüman denetim hatası olur, bu sınamalar ve onları sınayan test satırı (`lib/training_test.tr:73`) o gün kalkar ya da olumsuz fikstüre taşınır.
8. **Sayacın kapsamı dışında kalan aykırılıklar (dokunulmadı):** karışık değerli sözlük ve liste değişmezleri (`examples/tr/43` siparişleri, `lib/pretrained.tr` `TR_BLOCK_NAMES`'in `["…", TR_true, -1]` listeleri, kontrol noktası testlerinin `izlenen` sözlüğü) ve ilk değeri `TR_none` olup tipini yazmayan yereller (3.13; yaklaşık 36 yer [ölçüldü: `TR_var x = TR_none` için git grep]). 4a2'nin denetimi bunları gösterir.

## 4c'yi bekleyen (genel tip)

İzin listesinin ilk başlığı (44 satır, her biri dosya:satır ve nedeniyle). Özet:
- **Kimlikler hem liste hem tensör:** `lib/layers.tr` GömmeKatmanı, PLEBloğu, TransformerBloğu `ileri`, GPT `göm`, `blokları_işle`, `gizli_durum`, `ileri`, `kayıp` (kimlikler, hedefler) — 10 satır; `examples/tr/39_veri_ile_egitim.tr:73`.
- **Model her değer:** `lib/optimizers.tr` `parametre_grupları`, `parametre_kümeleri`, `MuonVeAdamW` (TR_parameters'ın okuduğu her değer); `lib/training.tr` `Koşu.bitir`.
- **Karışık değerli sözlük:** GPT'nin ayar alanı; `Koşu`'nun ayar ve nesneler parametreleri; Hugging Face ayarı (TR_JSON ya da sözlük: `lib/pretrained.tr` 6 satır, `pretrained_test.tr` `qwen3_ayarı`); `tools/gpu/training_model.tr` ayar; `tests/kontrol_noktasi_test.tr:154` dönüş; `examples/tr/17` sepet.
- **Birden çok yapı:** `MuonVeAdamW.muon` (Muon ya da NorMuon; ortak arayüz 7.8 tanımlanmadı, yönerge gereği), `optimizers_test.tr` `ters_eğit.kur`, `tek_tek_mi` (2 satır).
- **Klasör metin ya da yol:** `lib/training.tr:115` `Koşu.klasör`.
- **Testin konusu genel değer:** `tests/deyimler_test.tr:169` (5.6; `TR_Any` silindi, parametre tipsiz), `tests/tensor_test.tr:991` `Blok.mlp`, `tests/yapilar_test.tr:166` `Kutu.f`, `tests/yardimci_test_araclari.tr:7` `hata_metni.f`.
- **Genel tipin kendi örneği (3.11 kaldırınca):** `docs/codes.md` (TR_Any, TR_BuiltinFunction, TR_Function örnekleri), `examples/tr/12_tip_adlari.tr:30` (Herhangi'yi öğretiyor), `examples/tr/14_yapilar_ve_metotlar.tr:55` (Nesne).

## Olumsuz fikstür

- `tests/negative/yapi_kurucu_deger.tr:5` dönüş `TR_init`: 7.4, kurucunun değerli `TR_return`'ü sözdizimi hatası; kurucuya dönüş tipi yazılamaz (H0283). İzin listesinde.
- `lib/training_test.tr:73` `TR_SpikeGuard(TR_history_limit: 2.5)`: tipin kendisini (pencere tam sayı mı) çalışırken sınıyor; dokunulmadı. Sayacın saydığı bir yer değil (argüman), tanığın tek uyuşmazlığı.
- Öbür 18 olumsuz fikstür (`tests/negative/`) tiplendi: sınadıkları hata tipsizlik değil; beklenen tanıları değişmedi (`TestNegativeFixtures`). Bir kitaplık testinin `TR_device_step(1)`'i (H6908, önbellek çeşidi) `TR_device_step(TR_Tensor([1]))` oldu: sınadığı hata tip değil, argüman yanlış tipliydi (`a392e62`).

## Go testlerindeki satır içi TR programları

- **Sayım** [ölçüldü: `testkit.GoStringLiterals` + aynı ayrıştırıcı sayacı, geçici test]: başta 96 dosyada 487 satırda `TR_func`/`TR_struct` [git grep]; ayrıştırıcıyla **536** tipsiz yer (222 parametre, 79 alan, 233 dönüş, 1 öğe tipi, 1 genel tip).
- **Tiplenen (79):** dil davranışını ve kitaplığı sınayanlar: `internal/lang/interp` `behavior_robustness`, `behavior_named_args_language`, `behavior_value_text`, `behavior_determinism`, `library_test`; `internal/stdlib/gpuprobe` kitaplık GPU testi. Tanıkla 0 uyuşmazlık.
- **Kalan 457:** codegraph alanı 100 (aşağıda) ve **357**: `internal/lang/interp` 79 (26'sı sahiplik testleri: Go'nun test değerlerini (`yeni()`) taşıyor, TR tipi yok; 4'ü her tipte değerin metnini sınayan genel parametre; kalanı derleyici ve VM birim testleri), `internal/stdlib/ml` 67, `internal/lang/checker` 63, `internal/lang/parser` 58, `cmd/tr/e2e` 29, `internal/stdlib/gpuprobe` 14, `internal/lang/source` 13, `internal/devtools/debugger` 11, `formatter` 8, `internal/stdlib/checkpoint` 6, `profiler` 4, `testrunner` 3, `stdlib/process` 1, `stdlib/files` 1. Ayrıştırıcı ve denetleyici testleri çoğunlukla belirli bir yazımı ya da konumu sınıyor; tipleri elle ve testin konusunu bozmadan yazmak ayrı iş.

## Codegraph alanı (dokunulmadı, sonra yapılır)

- `.tr`: **18** yer, 4 dosyada: `tools/codegraph/testdata/mod/lib/geo.tr` 7, `…/examples/coded.tr` 2, `internal/devtools/codeindex/testdata/proj/lib/geo.tr` 7, `…/examples/coded.tr` 2 (8 parametre, 2 alan, 8 dönüş). `TestTRCodeIsTyped` `tools/codegraph/`'u atlıyor (yorumunda), `internal/` testdata'yı zaten taramıyor.
- Go testleri: **100** yer: `internal/devtools/codeindex` 95 (linked 26, bind 17, infer 9, library_codes 9, snippet 9, query 6, text 6, reader 5, constructs 4, cache 3, rename 1), `tools/codegraph` 5 (planning_exam 2, tr_snippets 2, fresh 1); `tools/edithook` 0.
- Kitaplığın tipleri değişti; `codeindex`'in kitaplığı okuyan testleri (`TestIncrementalEqualsFull`, altın dosyalar) geçiyor [ölçüldü].

## Codegraph'ın cevapları

- **`go run ./tools/codegraph dups --changed main`:** beklenen: değişen dosyaların kopya adayları; gelen: `codegraph: git merge-base main HEAD: exit status 1` (yerel `main` bu sığ klonda dalla ortak atası olmayan eski bir commit, 23e7be2; origin/main 1ec69a3); doğrusu: `--changed origin/main` çalıştı ("0 duplicate candidate(s)"). `tools/codegraph/gaps.md`'de zaten yazılı (Faz 0.5'ten); yeni bir şey yok.
- **`tests-for lib/layers.tr`:** 10 Go testi verdi (çoğu `codeindex`, [INFERRED]); `.tr` için `run:` satırı yok (beklenen), adaş `lib/layers_test.tr` `tr test lib` ile koşuldu. Yanlış görmedim.

## Kararsız (DENETİM'in bakması iyi olur)

1. **Model parametreleri 4c mi `TR_GPT` mi:** `MuonVeAdamW`, `parametre_grupları`, `parametre_kümeleri`'nin `model`'i depoda yalnız `TR_GPT` alıyor [ölçüldü: gözlem], ama gövde yalnız `TR_parameters(model)` okuyor ve README "model" diyor; desen varsayılanları GPT'nin alan adları. "Her model" sayıp 4c bıraktım; `TR_GPT` yazılsaydı API daralırdı.
2. **İzin listesinde üçüncü başlık:** yönerge iki başlık diyor; denetleyicinin yanlışı yüzünden yazılamayan tek tip (`TR_only_2d.TR_name`) ikisine de girmediği için "4a'yı bekleyen (denetleyici)" başlığını ekledim.
3. **Tam sayı değişmezleriyle çağrılan ondalık anlamlı parametreler:** `examples/tr/helpers/geometri.tr` `daire_alanı(r)` `TR_Float` (yarıçap; çağıranlar `2`, `1`), `tests/tensor_test.tr` 15.33 testinin `p(x)`'i `TR_Float` (türev izleyen tensör kuruyor; 15.2'den sonra tam sayı listesi `TR_Int` tensör olur ve türev izleyemez). Öbür yerlerde gözlenen tip (`TR_Int`) kaldı; ör. `TR_step(lr)` testinin çıktısı "3" (ondalık olsaydı 4a'dan sonra "3.0" olurdu).
4. **`hata_metni(f)` ve benzerleri:** "her dönüş tipindeki blok" `() -> T` (genel T): 4c sayıldı.
5. **Örnek 12 ve 14'ün genel tipleri:** örnek o tipi öğrettiği için (Herhangi, Nesne) dokunmadım; `tests/deyimler_test.tr`'de ise `TR_Any` testin konusu değildi, kaldırdım.

## Sorular

- yok (yeni ileti kodu gerekmedi).

## Bitti ölçütü (son koşu)

- `go build ./...`, `go vet ./...`, `gofmt -l` boş: geçti.
- `go run ./cmd/tr test tests`: 352/352; `go run ./cmd/tr test lib`: 69/69.
- `go run ./tools/codegraph spec-refs`: "278 item(s), 3709 mention(s), 0 error(s), 0 note(s)".
- `TestTRCodeIsTyped`: geçti, 46 satır.
- `go test ./internal/... ./cmd/... ./tools/...`: (aşağıda)
