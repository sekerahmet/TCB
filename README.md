# TR — yapay zekâ için programlama dili

TR, yapay zekâ modellerini **eğiten** ve **kullanan** geliştiriciler için tasarlanmış bir programlama dilidir. Tensörler, otomatik türev, iyileştiriciler, `tokenizer.json` parçalayıcıları ve büyük veri kümeleri dilin içinde yerleşik olarak gelir; testler de aynı dosyada yazılır.

**Kod, geliştiricinin kendi dilinde yazılır ve okunur:** bugün Türkçe ve İngilizce; İspanyolca, Almanca ve Fransızca sırada. Dilin ana yazımı Türkçedir. Başka bir dilde yazılan dosya dilden bağımsız **kodlu biçimde** saklanır; aynı program her yazım dilinde aynı anlama gelir ve VS Code'da her geliştirici dosyayı kendi dilinde görüp düzenler. Hata mesajları da kişinin dilindedir (`TR_DIL`) ve sorunu satırıyla gösterir.

Her işin tek yazımı vardır: bir yazım dilinde her anahtar kelimenin, yerleşiğin ve tipin tek adı vardır (Türkçede `değişken`, `yazdır`; İngilizcede `var`, `print`). Türkçe harfsiz bir yazım (`degisken`, `yazdir`) dilin kelimesi değildir, hata mesajı doğru adı önerir: `'yazdir' TR'de yok; 'yazdır' yazmak mı istediniz?` `tr` komutları ise her yazım dilinin adıyla çalışır (`tr çalıştır` ile `tr run` aynıdır).

## Kurulum

Yorumlayıcıyı proje kökünde derleyin (`go` derleyicisi kurulu olmalıdır); çıktı `bin/` klasörüne yazılır ve bu klasör depoya girmez:

```bash
go build -o bin/tr.exe ./cmd/tr
```

Program dosyası her x86-64 işlemcide çalışır: matris çarpımı ve vektör kernelleri çalışma anında seçilir. AVX2/FMA'lı işlemcide (Intel 2013, AMD 2015 sonrası) assembly kernelleri, öbürlerinde taşınabilir Go eşleri kullanılır; eğitim adımı AVX2'li yolda yaklaşık 8 kat hızlıdır (karakter modeli, 2 çekirdek: ~10 ve ~85 ms/adım; ölçüm: [`tools/bench/`](tools/bench/README.md)). `GOAMD64=v3` ile derlemek ölçümde hız kazandırmıyor, yalnızca AVX2'siz işlemcide açılmayan bir program dosyası verir. `TR_CPU_AVX2=0` bu kernelleri kapatır (sorun ayıklama için). ARM (arm64) taşınabilir kernelleri kullanır.

## İlk program

```tr
// merhaba.tr
değişken ad = "dünya"
yazdır("Merhaba, {ad}!")

fonksiyon kare(x) { sonuç x * x }
yazdır([1, 2, 3, 4].haritala { kare(o) })   // [1, 4, 9, 16]
```

```bash
./bin/tr.exe merhaba.tr          # çalıştırır
./bin/tr.exe kontrol merhaba.tr  # çalıştırmadan sözdizimi ve tip denetimi
./bin/tr.exe                     # etkileşimli kabuk
```

> **Güvenlik:** TR programları kum havuzunda (sandbox) çalışmaz; her program gibi onu çalıştıran kişinin yetkileriyle dosya okur, yazar ve siler. Tanımadığınız bir `.tr` programını ya da `tr derle` ile üretilmiş bir uygulamayı çalıştırmayın. Hangi girdilerin güvenilmeyen sayılıp korunduğu: [`SECURITY.md`](SECURITY.md).

## Testler ve biçimlendirme

```tr
// hesap_test.tr
fonksiyon topla(a, b) { sonuç a + b }

test "toplama" {
    eşit_mi(4, topla(2, 2))
}
```

- `tr test` bulunulan klasördeki (alt klasörler dahil) tüm `*_test.tr` dosyalarını, `tr test dosya.tr` yalnızca o dosyayı çalıştırır. Bir test kalırsa çıkış kodu 1'dir.
- `tr biçimle dosya.tr` (ya da klasör) kodu standart biçime getirir; `tr biçimle --kontrol .` yalnızca denetler ve biçimlenmesi gereken dosya varsa 1 ile çıkar.
- `tr izle egit.tr` programı çalıştırırken hızını canlı gösterir (ms/adım, en pahalı tensör işlemleri, en sıcak satırlar, GPU ve bellek); `tr profil egit.tr` bitince hız raporu verir (`--json koşu_klasörü/` ile `speed.json`). Ayrıntılar: [`docs/guide/11_hizi_izlemek.md`](docs/guide/11_hizi_izlemek.md).
- `tr kodla dosya.tr` (ya da klasör) dosyayı dilden bağımsız kodlu biçime çevirir (`yazdır` → `TR_fn_print`); `tr çöz` Türkçe yazıma geri çevirir, `tr göster dosya.tr` dosyayı değiştirmeden kişinin dilinde basar (`--dil en` ile istenen dilde). Kodlu dosya çevrilmeden de çalışır; adlarınız, metinler ve yorumlar değişmez. VS Code'da kodlu dosya kişinin yazım dilinde (ayar `tr.yazimDili`) görünür ve düzenlenir, kaydedince yine kodlu yazılır; satırlar hata mesajları ve hata ayıklayıcıyla aynıdır ([eklenti belgesi](editors/vscode/README.md#kodlu-dosyalar)). Kurallar: [`docs/spec/17_kodlar.md`](docs/spec/17_kodlar.md).

## Tensörler ve otomatik türev

```tr
// y = 2·x - 1 doğrusunu öğren
değişken X = tensör([[0], [1], [2], [3]])
değişken Y = tensör([[-1], [1], [3], [5]])
değişken w = tensör([[0.0]], türev_izle: doğru)
değişken b = tensör([0.0], türev_izle: doğru)
değişken iyileştirici = AdamW([w, b], öğrenme_oranı: 0.1, ağırlık_çürümesi: 0)

döngü adım içinde 0..500 {
    iyileştirici.sıfırla()
    değişken fark = X @ w + b - Y
    geri_yayılım(ortalama(fark ** 2))
    iyileştirici.adım()
}
yazdır("w = {yuvarla(w.değer[0][0], 3)}, b = {yuvarla(b.değer[0], 3)}")   // w = 2.0, b = -1.0
```

Dilin tek sayısal dizi türü tensördür: matrisler 2 boyutlu tensördür (`tensör([[1, 2], [3, 4]])`), elemanlar varsayılan olarak `float32`'dir, `bf16` ve `fp16` de seçilebilir; hesaplar en az `float32` ile yapılır. Tensörler `döngü satır içinde t` ile ilk ekseni boyunca gezilir ve `t[i] = d` ile yerinde değiştirilir. `@` matris çarpımıdır; `+ - * /` eleman bazlı ve yayınlamalıdır. `relu`, `gelu`, `softmax`, `katman_normu`, `gömme`, `çapraz_entropi`, `SGD`, `gradyan_kırp`, `tensör_kaydet` / `tensör_yükle` ve fazlası için: [Kullanım Kılavuzu §4.C](docs/guide/04_yapay_zeka_ve_veri.md#c-tensörler-ve-otomatik-türev).

## Veri ve parçalayıcı

```tr
değişken p = parçalayıcı_yükle("tokenizer.json")
yazdır(p.kodla("Merhaba dünya"), p.dağarcık_boyutu)

değişken küme = veri_kümesi("veri/train")                       // .npy veri dosyaları
değişken ö = küme.örnekleyici(toplu: 8, bağlam: 256, tohum: 1)
değişken girdi, hedef = ö.sonraki()                              // [8, 256] tensörler
```

`parçalayıcı_yükle` yaygın `tokenizer.json` biçimini okur; `veri_kümesi` büyük derlemleri belleğe almadan parça parça okur, sonraki toplu işleri arka planda hazırlar. Uçtan uca örnek: [`examples/tr/39_veri_ile_egitim.tr`](examples/tr/39_veri_ile_egitim.tr); ayrıntılar: [Kılavuz §4.D](docs/guide/04_yapay_zeka_ve_veri.md#d-veri-ve-parçalayıcı).

## Başka dillerden aldıklarımız

TR'nin hedeflerinden biri başka dillerin güçlü yanlarını bir araya getirmek, bilinen hatalarını tekrarlamamaktır: bir tasarım kararından önce başka dillerin çözümü karşılaştırılır. Aşağıdaki her satır bugünkü `tr` ile denenmiştir. "Kural" sütunu TR'deki davranışın yazılı olduğu maddedir; karşılaştırması ve gerekçesi yazılı olan maddelerde (2.3b, 2.8, 3.6, 11.7, 15. bölüm, 17.7, 17.8, 19.5) ayrıntı oradadır.

| TR'de | Nereden | Kural |
| :-- | :-- | :-- |
| Satır sonu deyimi bitirir; satır başındaki `-` önceki satıra bağlanmaz | Go, Python, Kotlin, Swift (JavaScript'in bu tuzağı alınmadı) | [2.8](docs/spec/02_sozcuk_yapisi.md) |
| Ad yazımı kuralı (sabit `BÜYÜK_HARF`, yapı `BüyükDeve`) hata değil uyarıdır | Rust'ın ad uyarıları, Python PEP 8 | [2.3b](docs/spec/02_sozcuk_yapisi.md) |
| `a..b` uç hariç, `a..=b` uç dahil | Rust | [3.8](docs/spec/03_tipler.md) |
| Sınır dışı okuma hatadır; olmayabilecek öğe `varsa_al(i) ?? varsayılan` ile istenir | Python, Go, Rust ve Kotlin'de hata; Kotlin `getOrNull`, Rust `get` | [3.6](docs/spec/03_tipler.md), [15.49](docs/spec/15_tensorler.md) |
| `a ?? b` ve `a?.b` | Swift, C# | [4.7, 4.8](docs/spec/04_operatorler.md) |
| `x \|> f(a)` ≡ `f(x, a)` (değer ilk argüman olur) | Elixir | [4.9](docs/spec/04_operatorler.md) |
| Liste üreteci `[x * x döngü x içinde 1..=5 eğer x % 2 == 1]` | Python | [5.11](docs/spec/05_deyimler.md) |
| `eşle konu { 1 => ..., _ => ... }` (desen değerle karşılaştırılır) | Rust'ın `match` yazımı | [5.6](docs/spec/05_deyimler.md) |
| Sondaki blok: `l.filtrele { o > 2 }`, tek parametre `o` | Kotlin'in sondaki lambdası ve `it`'i | [6.7](docs/spec/06_fonksiyonlar.md) |
| `ertele`: fonksiyon bitince, ters sırayla çalışır | Go `defer` | [9.1](docs/spec/09_ertele.md) |
| `eşzamanlı` görev ve yorumlayıcı kilidi | Go'nun `go` deyimi; kilit Python'unki gibi | [10.1, 10.2](docs/spec/10_eszamanlilik.md) |
| Başı `_` olan ad modüle özeldir ve dışarıdan erişilemez | Python'un yazımı; Python'da yalnızca gelenek, TR'de kural | [11.7](docs/spec/11_moduller.md) |
| `test "ad" { ... }` programla aynı dosyada; normal çalıştırmada atlanır | Zig | [12.1](docs/spec/12_testler.md) |
| Tensör kuralları: yayınlama, tür yükseltme, indeks, dilim | PyTorch ve NumPy, satır satır karşılaştırıldı | [15.49](docs/spec/15_tensorler.md) |
| Özel değerler (±0, ±∞, NaN) CPU'da ve GPU'da aynı | PyTorch 2.14 ve NumPy 2.1 ile ölçüldü; farklar gerekçeli | [15.50](docs/spec/15_tensorler.md) |
| `gelu(yaklaşık: doğru)`: seçenek Mantık bayraktır | JAX ve TensorFlow (PyTorch'un `"tanh"` metni yazım hatasına açık) | [15.17](docs/spec/15_tensorler.md) |
| Dikkatin tensör düzeni `(toplu, uzunluk, bakış, boyut)` | JAX ve FlashAttention (çevirme kopyası olmaz) | [15.24](docs/spec/15_tensorler.md) |
| Bellek: ara sonuçlar işlemden hemen sonra, depolama son sahibi gidince bırakılır | Mojo, PyTorch, TensorFlow.js `tidy` | [15.43](docs/spec/15_tensorler.md) |
| `kaynaştır`: dışarıdan okunan sayılar ilk çağrıda sabitlenir | JAX `jit` | [15.44](docs/spec/15_tensorler.md) |
| `tekrar_oynat`: GPU adımını bir kez kaydedip oynatır, girdileri kendisi kopyalar | CUDA Graphs (PyTorch bu işi kullanıcıya bırakır) | [15.45](docs/spec/15_tensorler.md) |
| safetensors okumasının denetimleri | safetensors'un kendi okuyucusu (yinelenen anahtar ayrıca reddedilir) | [15.47](docs/spec/15_tensorler.md) |
| Günlükte NaN ve sonsuz `"NaN"`, `"Infinity"` olarak yazılır | Python `json.dumps` (`float()` geri okur) | [19.5](docs/spec/19_surec_ve_kayit.md) |

**TR'ye özgü:** kod geliştiricinin kendi dilinde yazılır ve okunur, dosya dilden bağımsız kodlarla saklanır. Karşılaştırılan dillerin hiçbiri (Python, Java, Swift, Go) değerin metnini yerelleştirmez; dosyanın dili Rust'ın `edition`'ı ve Python'un PEP 263 başlığı gibi dosyanın başında bildirilir ([17.7](docs/spec/17_kodlar.md)). `tr` komutları da her yazım dilinin adıyla çalışır; Go, git ve cargo komut adlarını tek (İngilizce) yazımda tutar ([17.8](docs/spec/17_kodlar.md)).

## VS Code eklentisi ve hata ayıklama

[`editors/vscode/`](editors/vscode/README.md) `.tr` dosyalarına renklendirme, kod şablonları (`tensör` yazıp `Tab`: küçük bir eğitim döngüsü), çalıştırma (▶) ve hata ayıklama getirir. **F5** ile hata ayıklarken **İzleme Masası** paneli tüm değişkenleri tek tabloda gösterir; listeler, tensörler ve veri kümeleri tablo olarak açılır, değerler çalışma anında değiştirilebilir. Terminalde hata ayıklamak için: `tr hata_ayıkla dosya.tr`.

Bu depoyu VS Code'da klasör olarak açtığınızda `Ctrl+Shift+B` açık dosyayı `bin/tr.exe` ile çalıştırır (`.vscode/tasks.json`). Çalıştırma görevleri yorumlayıcıyı **derlemez** (açık bir `tr.exe` Windows'ta dosyayı kilitlediği için otomatik derleme bilerek kapalıdır); Go kodunu değiştirdiyseniz önce `Ctrl+Shift+P` → "Tasks: Run Task" → **TR: yorumlayıcıyı derle** görevini çalıştırın.

## Örnekler

[`examples/`](examples/) basitten zora numaralı bir öğrenme yoludur: her örnek tek bir yeni kavram getirir, öncekilerde öğretilmemiş hiçbir şeyi kullanmaz; başında ne öğrettiğini ve önceki örnekten ne aldığını söyler, sonunda "Kendin dene" alıştırmaları vardır. **Her örnek iki yazımla bulunur:** [`examples/tr/`](examples/tr/) Türkçe, [`examples/en/`](examples/en/) İngilizce; aynı numara aynı programdır (İngilizce dosyalar kodlu saklanır, `tr göster --dil en` ile okunur; bkz. [dil tanımı 17](docs/spec/17_kodlar.md)). Proje kökünden çalıştırın: `./bin/tr.exe examples/tr/01_merhaba.tr` ya da `./bin/tr.exe examples/en/01_hello.tr`.

| No | Türkçe | İngilizce | Ne öğretir |
| :-- | :-- | :-- | :-- |
| 01 | [`01_merhaba.tr`](examples/tr/01_merhaba.tr) | [`01_hello.tr`](examples/en/01_hello.tr) | Merhaba: ilk TR programı |
| 02 | [`02_degiskenler.tr`](examples/tr/02_degiskenler.tr) | [`02_variables.tr`](examples/en/02_variables.tr) | Değişkenler: bir değeri adlandırıp saklamak |
| 03 | [`03_tipler.tr`](examples/tr/03_tipler.tr) | [`03_types.tr`](examples/en/03_types.tr) | Tipler: her değerin bir tipi vardır |
| 04 | [`04_metin.tr`](examples/tr/04_metin.tr) | [`04_text.tr`](examples/en/04_text.tr) | Metin: metne değer yerleştirmek ve metinle çalışmak |
| 05 | [`05_kosullar.tr`](examples/tr/05_kosullar.tr) | [`05_conditions.tr`](examples/en/05_conditions.tr) | Koşullar: programın karar vermesi |
| 06 | [`06_girdi.tr`](examples/tr/06_girdi.tr) | [`06_input.tr`](examples/en/06_input.tr) | Girdi: kullanıcıdan veri okumak |
| 07 | [`07_donguler.tr`](examples/tr/07_donguler.tr) | [`07_loops.tr`](examples/en/07_loops.tr) | Döngüler: bir işi yinelemek |
| 08 | [`08_fonksiyonlar.tr`](examples/tr/08_fonksiyonlar.tr) | [`08_functions.tr`](examples/en/08_functions.tr) | Fonksiyonlar: işi adlandırıp yeniden kullanmak |
| 09 | [`09_listeler.tr`](examples/tr/09_listeler.tr) | [`09_lists.tr`](examples/en/09_lists.tr) | Listeler: birçok değeri sıralı tutmak |
| 10 | [`10_ok_fonksiyonlari.tr`](examples/tr/10_ok_fonksiyonlari.tr) | [`10_lambdas.tr`](examples/en/10_lambdas.tr) | Ok fonksiyonları ve zincirleme: fonksiyonu değer olarak kullanmak |
| 11 | [`11_sozlukler.tr`](examples/tr/11_sozlukler.tr) | [`11_dicts.tr`](examples/en/11_dicts.tr) | Sözlükler: değerleri anahtarla saklamak |
| 12 | [`12_tip_adlari.tr`](examples/tr/12_tip_adlari.tr) | [`12_type_names.tr`](examples/en/12_type_names.tr) | Tip adları: değerlerin tipini sormak ve bildirmek |
| 13 | [`13_matematik.tr`](examples/tr/13_matematik.tr) | [`13_math.tr`](examples/en/13_math.tr) | Matematik: sayılarla hesap yapan yerleşikler |
| 14 | [`14_yapilar_ve_metotlar.tr`](examples/tr/14_yapilar_ve_metotlar.tr) | [`14_structs_and_methods.tr`](examples/en/14_structs_and_methods.tr) | Yapılar ve metotlar: ilişkili verileri ve onlara bağlı işleri bir arada tutmak |
| 15 | [`15_hata_yonetimi.tr`](examples/tr/15_hata_yonetimi.tr) | [`15_error_handling.tr`](examples/en/15_error_handling.tr) | Hata yönetimi: beklenmedik durumda programı düzgün yönetmek |
| 16 | [`16_ertele.tr`](examples/tr/16_ertele.tr) | [`16_defer.tr`](examples/en/16_defer.tr) | Ertele: temizliği her durumda yapmak |
| 17 | [`17_testler_test.tr`](examples/tr/17_testler_test.tr) | [`17_tests_test.tr`](examples/en/17_tests_test.tr) | Testler: kodun yanına test yazmak |
| 18 | [`18_moduller.tr`](examples/tr/18_moduller.tr) | [`18_modules.tr`](examples/en/18_modules.tr) | Modüller: kodu dosyalara bölüp aktarmak |
| 19 | [`19_dosya_ve_json.tr`](examples/tr/19_dosya_ve_json.tr) | [`19_files_and_json.tr`](examples/en/19_files_and_json.tr) | Dosya ve JSON: veriyi diske yazıp geri okumak |
| 20 | [`20_yollar_ve_klasorler.tr`](examples/tr/20_yollar_ve_klasorler.tr) | [`20_paths_and_folders.tr`](examples/en/20_paths_and_folders.tr) | Yollar ve klasörler: dosya sistemini yönetmek |
| 21 | [`21_csv_ve_ortam.tr`](examples/tr/21_csv_ve_ortam.tr) | [`21_csv_and_environment.tr`](examples/en/21_csv_and_environment.tr) | CSV, ortam değişkenleri ve zaman: dış dünyadan veri almak |
| 22 | [`22_eszamanli_gorevler.tr`](examples/tr/22_eszamanli_gorevler.tr) | [`22_concurrent_tasks.tr`](examples/en/22_concurrent_tasks.tr) | Eşzamanlı görevler: işleri aynı anda yürütmek |
| 23 | [`23_tensor.tr`](examples/tr/23_tensor.tr) | [`23_tensors.tr`](examples/en/23_tensors.tr) | Tensörler: yapay zekânın sayı dizileri |
| 24 | [`24_tensor_matematigi.tr`](examples/tr/24_tensor_matematigi.tr) | [`24_tensor_math.tr`](examples/en/24_tensor_math.tr) | Tensör matematiği: işlemleri bir tensörün her elemanına uygulamak |
| 25 | [`25_secme_ve_siralama.tr`](examples/tr/25_secme_ve_siralama.tr) | [`25_selecting_and_sorting.tr`](examples/en/25_selecting_and_sorting.tr) | Seçme ve sıralama: tensörde koşula göre seçmek, en büyükleri bulmak |
| 26 | [`26_turev.tr`](examples/tr/26_turev.tr) | [`26_autodiff.tr`](examples/en/26_autodiff.tr) | Otomatik türev: gradyanı bilgisayara hesaplatmak |
| 27 | [`27_ozel_turev.tr`](examples/tr/27_ozel_turev.tr) | [`27_custom_gradients.tr`](examples/en/27_custom_gradients.tr) | Özel türev: kendi türevlenebilir işlemini yazmak ve doğrulamak |
| 28 | [`28_egitim_dongusu.tr`](examples/tr/28_egitim_dongusu.tr) | [`28_training_loop.tr`](examples/en/28_training_loop.tr) | Eğitim döngüsü: bir modelin veriden öğrenmesi |
| 29 | [`29_iyilestiriciler.tr`](examples/tr/29_iyilestiriciler.tr) | [`29_optimizers.tr`](examples/en/29_optimizers.tr) | İyileştiriciler: ağırlıkları güncellemenin yolları |
| 30 | [`30_xor_agi.tr`](examples/tr/30_xor_agi.tr) | [`30_xor_network.tr`](examples/en/30_xor_network.tr) | Küçük sinir ağı: iki katmanlı bir ağ XOR'u öğrenir |
| 31 | [`31_katmanlar.tr`](examples/tr/31_katmanlar.tr) | [`31_layers.tr`](examples/en/31_layers.tr) | Katmanlar: sinir ağlarının hazır yapı taşları |
| 32 | [`32_agirliklari_kaydet.tr`](examples/tr/32_agirliklari_kaydet.tr) | [`32_saving_weights.tr`](examples/en/32_saving_weights.tr) | Ağırlıkları kaydetmek: eğitilmiş modeli diske yazıp geri yüklemek |
| 33 | [`33_cihaz_ve_hassasiyet.tr`](examples/tr/33_cihaz_ve_hassasiyet.tr) | [`33_devices_and_precision.tr`](examples/en/33_devices_and_precision.tr) | Cihaz ve hassasiyet: tensörlerin nerede ve hangi sayı türüyle yaşadığı |
| 34 | [`34_metin_uretimi.tr`](examples/tr/34_metin_uretimi.tr) | [`34_sampling.tr`](examples/en/34_sampling.tr) | Metin üretimi: modelin çıktısından örnek seçmek |
| 35 | [`35_transformer.tr`](examples/tr/35_transformer.tr) | [`35_transformer.tr`](examples/en/35_transformer.tr) | Transformer: elle yazılmış küçük bir transformer |
| 36 | [`36_karakter_modeli.tr`](examples/tr/36_karakter_modeli.tr) | [`36_char_model.tr`](examples/en/36_char_model.tr) | Karakter modeli: gerçek metinle eğitilen küçük bir dil modeli (~100 bin parametre) |
| 37 | [`37_komut_satirinda_ayar.tr`](examples/tr/37_komut_satirinda_ayar.tr) | [`37_command_line_settings.tr`](examples/en/37_command_line_settings.tr) | Komut satırından ayar: programı koda dokunmadan ayarlamak |
| 38 | [`38_ilerleme_ve_gunluk.tr`](examples/tr/38_ilerleme_ve_gunluk.tr) | [`38_progress_and_logs.tr`](examples/en/38_progress_and_logs.tr) | İlerleme çubuğu ve günlük: uzun işi izlemek ve kaydını tutmak |
| 39 | [`39_veri_ile_egitim.tr`](examples/tr/39_veri_ile_egitim.tr) | [`39_training_with_data.tr`](examples/en/39_training_with_data.tr) | Veri ile eğitim: parçalayıcı ve veri yükleyiciyle uçtan uca küçük bir dil modeli |
| 40 | [`40_kontrol_noktasi.tr`](examples/tr/40_kontrol_noktasi.tr) | [`40_checkpoints.tr`](examples/en/40_checkpoints.tr) | Kontrol noktası: uzun eğitimi kaldığı yerden sürdürmek |
| 41 | [`41_gpt.tr`](examples/tr/41_gpt.tr) | [`41_gpt.tr`](examples/en/41_gpt.tr) | Hazır katmanlarla GPT: dilin kitaplığındaki katmanlar ve iyileştiricilerle küçük bir dil modeli |
| 42 | [`42_hata_ayiklama.tr`](examples/tr/42_hata_ayiklama.tr) | [`42_debugging.tr`](examples/en/42_debugging.tr) | Hata ayıklama: koddaki kesme noktasıyla bir hesap hatasını çalışırken bulup düzeltmek |
| 43 | [`43_hata_ayiklama_alistirmasi.tr`](examples/tr/43_hata_ayiklama_alistirmasi.tr) | [`43_debugging_exercise.tr`](examples/en/43_debugging_exercise.tr) | Hata ayıklama alıştırması: VS Code hata ayıklayıcısıyla bir programı adım adım izlemek |
| 44 | [`44_egitim_kararliligi.tr`](examples/tr/44_egitim_kararliligi.tr) | [`44_training_stability.tr`](examples/en/44_training_stability.tr) | Eğitimin kararlılığı: NaN'i ve kayıp sıçramasını yakalamak, son kontrol noktasına dönüp sorunlu toplu işleri atlamak |
| 45 | [`45_tekrar_oynat.tr`](examples/tr/45_tekrar_oynat.tr) | [`45_replay.tr`](examples/en/45_replay.tr) | Kaydedip oynatmak: sık tekrarlanan küçük bir GPU adımını bir kez kaydedip tek seferde oynatmak (`tekrar_oynat`, `yerine_yaz`) |
| 46 | [`46_hazir_model_ve_uretim.tr`](examples/tr/46_hazir_model_ve_uretim.tr) | [`46_ready_model_and_generation.tr`](examples/en/46_ready_model_and_generation.tr) | Hazır model ve üretim: Hugging Face biçimindeki bir modeli yüklemek (`hazır_model`, `ağırlık_türü`) ve `üret` ile parça üretmek (küçük, rastgele ağırlıklı model `tests/data/hf_tiny`) |
| 47 | [`47_kendi_uretim_dongusu.tr`](examples/tr/47_kendi_uretim_dongusu.tr) | [`47_own_generation_loop.tr`](examples/en/47_own_generation_loop.tr) | Kendi üretim döngün: `önbellek_kur`, önbellekli `ileri`, `kırp`; elle yazılan döngü `üret` ile aynı diziyi verir; `gizli_durum` + `tek_adım` + `tekrar_oynat` |
| 48 | [`48_egitim_kosusu.tr`](examples/tr/48_egitim_kosusu.tr) | [`48_training_run.tr`](examples/en/48_training_run.tr) | Eğitim koşusu: kayıt işlerini kitaplığa bırakmak (`Koşu`: klasör, günlük, çubuk, kontrol noktası, ağırlıklar önce) ve uzun koşudan önce `prova` |

**Hata örnekleri.** [`examples/tr/hatalar/`](examples/tr/hatalar/) ve [`examples/en/errors/`](examples/en/errors/) sık yapılan hataları tek tek gösterir: her dosya bir hatayı bilerek yapar, yorumunda iletinin kodunu, nedenini ve düzeltilmiş hâlini yazar (çıkış kodu 1).

| No | Türkçe | İngilizce | Hata |
| :-- | :-- | :-- | :-- |
| 01 | [`01_sozdizimi_kapanmayan_blok.tr`](examples/tr/hatalar/01_sozdizimi_kapanmayan_blok.tr) | [`01_syntax_unclosed_block.tr`](examples/en/errors/01_syntax_unclosed_block.tr) | Kapatılmayan blok (`H0156`) |
| 02 | [`02_sozdizimi_ad_bekleniyor.tr`](examples/tr/hatalar/02_sozdizimi_ad_bekleniyor.tr) | [`02_syntax_name_expected.tr`](examples/en/errors/02_syntax_name_expected.tr) | Ad bekleniyordu (`H0100`) |
| 03 | [`03_sozdizimi_kapanmayan_metin.tr`](examples/tr/hatalar/03_sozdizimi_kapanmayan_metin.tr) | [`03_syntax_unterminated_text.tr`](examples/en/errors/03_syntax_unterminated_text.tr) | Kapatılmayan metin (`H0010`) |
| 04 | [`04_tanimsiz_ad.tr`](examples/tr/hatalar/04_tanimsiz_ad.tr) | [`04_undefined_name.tr`](examples/en/errors/04_undefined_name.tr) | Tanımsız ad (`H1010`) |
| 05 | [`05_tip_uyusmazligi.tr`](examples/tr/hatalar/05_tip_uyusmazligi.tr) | [`05_type_mismatch.tr`](examples/en/errors/05_type_mismatch.tr) | Tip uyuşmazlığı (`H1001`) |
| 06 | [`06_sabit_degistirme.tr`](examples/tr/hatalar/06_sabit_degistirme.tr) | [`06_changing_a_constant.tr`](examples/en/errors/06_changing_a_constant.tr) | Sabiti değiştirmek (`H1002`) |
| 07 | [`07_yanlis_tip_arguman.tr`](examples/tr/hatalar/07_yanlis_tip_arguman.tr) | [`07_wrong_type_argument.tr`](examples/en/errors/07_wrong_type_argument.tr) | Yanlış tipte argüman (`H1004`) |
| 08 | [`08_eksik_arguman.tr`](examples/tr/hatalar/08_eksik_arguman.tr) | [`08_missing_argument.tr`](examples/en/errors/08_missing_argument.tr) | Eksik argüman (`H2414`) |
| 09 | [`09_bilinmeyen_adli_arguman.tr`](examples/tr/hatalar/09_bilinmeyen_adli_arguman.tr) | [`09_unknown_named_argument.tr`](examples/en/errors/09_unknown_named_argument.tr) | Bilinmeyen adlı argüman (`H2410`) |
| 10 | [`10_sinir_disi_indeks.tr`](examples/tr/hatalar/10_sinir_disi_indeks.tr) | [`10_index_out_of_range.tr`](examples/en/errors/10_index_out_of_range.tr) | Sınır dışı indeks (`H2515`) |
| 11 | [`11_sifira_bolme.tr`](examples/tr/hatalar/11_sifira_bolme.tr) | [`11_division_by_zero.tr`](examples/en/errors/11_division_by_zero.tr) | Sıfıra bölme (`H2470`) |
| 12 | [`12_yakalanmamis_firlat.tr`](examples/tr/hatalar/12_yakalanmamis_firlat.tr) | [`12_uncaught_throw.tr`](examples/en/errors/12_uncaught_throw.tr) | Yakalanmayan fırlat (`H2441`) |
| 13 | [`13_yok_uzerinde_ozellik.tr`](examples/tr/hatalar/13_yok_uzerinde_ozellik.tr) | [`13_property_of_none.tr`](examples/en/errors/13_property_of_none.tr) | yok değerin özelliği (`H2466`) |
| 14 | [`14_alan_bulunamadi.tr`](examples/tr/hatalar/14_alan_bulunamadi.tr) | [`14_field_not_found.tr`](examples/en/errors/14_field_not_found.tr) | Olmayan alan (`H2468`) |
| 15 | [`15_donusum_hatasi.tr`](examples/tr/hatalar/15_donusum_hatasi.tr) | [`15_conversion_error.tr`](examples/en/errors/15_conversion_error.tr) | Dönüşüm hatası (`H2429`) |
| 16 | [`16_fonksiyon_degil.tr`](examples/tr/hatalar/16_fonksiyon_degil.tr) | [`16_not_a_function.tr`](examples/en/errors/16_not_a_function.tr) | Fonksiyon olmayanı çağırmak (`H2421`) |
| 17 | [`17_dongu_tipi.tr`](examples/tr/hatalar/17_dongu_tipi.tr) | [`17_loop_over_wrong_type.tr`](examples/en/errors/17_loop_over_wrong_type.tr) | Gezilemeyen değer (`H2443`) |
| 18 | [`18_sonsuz_ozyineleme.tr`](examples/tr/hatalar/18_sonsuz_ozyineleme.tr) | [`18_infinite_recursion.tr`](examples/en/errors/18_infinite_recursion.tr) | Sonsuz özyineleme (`H2422`) |
| 19 | [`19_dosya_bulunamadi.tr`](examples/tr/hatalar/19_dosya_bulunamadi.tr) | [`19_file_not_found.tr`](examples/en/errors/19_file_not_found.tr) | Dosya bulunamadı (`H2100`) |
| 20 | [`20_modul_bulunamadi.tr`](examples/tr/hatalar/20_modul_bulunamadi.tr) | [`20_module_not_found.tr`](examples/en/errors/20_module_not_found.tr) | Modül bulunamadı (`H2112`) |
| 21 | [`21_tensor_sekil.tr`](examples/tr/hatalar/21_tensor_sekil.tr) | [`21_tensor_shape.tr`](examples/en/errors/21_tensor_shape.tr) | Tensör şekil uyuşmazlığı (`H6126`) |
| 22 | [`22_tensor_eksen.tr`](examples/tr/hatalar/22_tensor_eksen.tr) | [`22_tensor_axis.tr`](examples/en/errors/22_tensor_axis.tr) | Geçersiz eksen (`H6016`) |
| 23 | [`23_devam_disinda.tr`](examples/tr/hatalar/23_devam_disinda.tr) | [`23_continue_outside_loop.tr`](examples/en/errors/23_continue_outside_loop.tr) | Döngü dışında devam (`H0100`) |

Komut satırı adları (`tr test`, `tr biçimle`, `tr kodla`, hata ayıklayıcı komutları ...) [`examples/README.md`](examples/README.md)'dedir.

Kendi denemeleriniz için kökte bir `scratch/` klasörü açıp örnekleri oraya kopyalayabilirsiniz (ör. `examples/tr/01_merhaba.tr`); bu klasör depoya girmez.

## Belgeler ve ölçümler

- [`docs/guide/`](docs/guide/README.md) — komutlar, yerleşikler, tensörler ve yapay zekâ, veri ve parçalayıcı, hata ayıklayıcı.
- [`docs/spec/`](docs/spec/README.md) — dilin kuralları; her kuralın `tests/` altında bir testi vardır (`tr test tests`).
- [`docs/codes.md`](docs/codes.md) — her anahtar kelimenin, tipin, yerleşiğin, metodun ve adlı argümanın ne işe yaradığı, imzası ve örneği (kod listesinden üretilir).
- [`docs/roadmap.md`](docs/roadmap.md) — ilkeler ve evreler: sağlam temel, gerçek model, geliştirici deneyimi, yaygınlaşma.
- [`docs/architecture.md`](docs/architecture.md) — katmanlar, bağımlılık kuralı ve paketlerin görevleri.
- [`docs/development.md`](docs/development.md) — katkı rehberi: dosya ve fonksiyon boyu, adlar, paket belgesi, testler, yeni kod isteği, tam denetim, ölçüm.
- [`tools/bench/`](tools/bench/README.md) — hız kıyas seti ve model eğitimi süre ölçümleri.
- [`SECURITY.md`](SECURITY.md) — güvenlik açığı bildirme, güvenilmeyen sayılan girdiler (`.trt`, `.npy`, `tokenizer.json` ...) ve bulanıklaştırma testleri.

## Depo yapısı

| Klasör | İçerik |
| :-- | :-- |
| `cmd/tr` | `tr` komutu: kabuk, `çalıştır`, `kontrol`, `derle`, `test`, `biçimle`, `kodla`, `çöz`, `göster`, `izle`, `profil`, `hata_ayıkla`, `dizin`, `yardım`, `sürüm`, `dap` (VS Code için); komut adları her yazım dilinde kabul edilir |
| `internal/lang` | Dil çekirdeği: iletiler (`messages`), Türkçe ekler ve dosya hataları (`turkish`), kod listesi (`codes`, [`docs/codes.md`](docs/codes.md)), ad ipuçları (`hint`), sözcük çözümleyici (`lexer`), sözdizimi ağacı (`ast`), ayrıştırıcı (`parser`), tip denetimi (`checker`), kaynak ve kodlu biçim (`source`), yorumlayıcı (`interp`) |
| `internal/stdlib` | Standart kitaplık: tensör ve öğrenme (`ml`), veri (`dataset`), parçalayıcı (`tokenizer`), dosyalar (`files`), JSON/CSV (`formats`), komut satırı (`process`), günlük ve ilerleme (`logging`), kontrol noktası (`checkpoint`) |
| `internal/compute` | Dili bilmeyen hesap çekirdekleri: tensör, otomatik türev ve CPU kernelleri (`tensor`), GPU arka ucu (`gpu`), parçalayıcı (`tokenize`), veri yükleyici (`dataload`) |
| `internal/devtools` | Hata ayıklayıcı (`debugger`), biçimlendirici (`formatter`), `tr test` çekirdeği (`testrunner`), canlı profil (`profiler`: `tr izle`, `tr profil`), TR kodunun dizini (`codeindex`: `tr dizin`) |
| `lib/` | TR ile yazılan standart kitaplık, ikiliye gömülü: transformer katmanları ve GPT (`aktar "tr/katman"`), Muon/NorMuon (`aktar "tr/iyilestirici"`), eğitim koşusunun kayıt işleri (`aktar "tr/egitim"`: `Koşu`, prova), Hugging Face biçimindeki hazır modeller (`aktar "tr/hazir"`); [`lib/README.md`](lib/README.md) |
| `editors/vscode/` | VS Code eklentisi |
| `docs/` | Belgeler ([dizin](docs/README.md)); kod kuralları [`internal/README.md`](internal/README.md), ekleme tarifleri [`docs/recipes.md`](docs/recipes.md) |
| `examples/`, `tests/` | Örnek programlar ve dil tanımının kural testleri |
| `tools/` | Depo bakımı: `full_check.sh`, `fuzz.sh`, üreteçler (`gen/`), hız kıyas seti (`bench/`), GPU ölçümleri (`gpu/`), ölçüm kaydı (`record/`), Go kodunun grafiği (`codegraph/`), etki sorgusu (`impact/`) |

Katmanlar, aralarındaki bağımlılık kuralı ve her paketin görevi: [`docs/architecture.md`](docs/architecture.md).

## Lisans

TR, [Apache Lisansı 2.0](LICENSE) ile dağıtılır: kodu kullanabilir, değiştirebilir ve dağıtabilirsiniz; lisans metni ve değişiklik bildirimleri korunmalıdır.
