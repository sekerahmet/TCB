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
//! tr kodlu 3 dil=tr
// merhaba.tr
TR_kw_var ad = "dünya"
TR_fn_print("Merhaba, {ad}!")

TR_kw_func kare(x) { TR_kw_return x * x }
TR_fn_print([1, 2, 3, 4].TR_mem_map { kare(TR_ctx_it) })   // [1, 4, 9, 16]
```

```bash
./bin/tr.exe merhaba.tr          # çalıştırır
./bin/tr.exe kontrol merhaba.tr  # çalıştırmadan sözdizimi ve tip denetimi
./bin/tr.exe                     # etkileşimli kabuk
```

> **Güvenlik:** TR programları kum havuzunda (sandbox) çalışmaz; her program gibi onu çalıştıran kişinin yetkileriyle dosya okur, yazar ve siler. Tanımadığınız bir `.tr` programını ya da `TR_cmd_build` ile üretilmiş bir uygulamayı çalıştırmayın. Hangi girdilerin güvenilmeyen sayılıp korunduğu: [`SECURITY.md`](SECURITY.md).

## Testler ve biçimlendirme

```tr
//! tr kodlu 3 dil=tr
// hesap_test.tr
TR_kw_func topla(a, b) { TR_kw_return a + b }

TR_ctx_test "toplama" {
    TR_fn_assert_equal(4, topla(2, 2))
}
```

- `tr test` bulunulan klasördeki (alt klasörler dahil) tüm `*_test.tr` dosyalarını, `tr test dosya.tr` yalnızca o dosyayı çalıştırır. Bir test kalırsa çıkış kodu 1'dir.
- `tr biçimle dosya.tr` (ya da klasör) kodu standart biçime getirir; `tr biçimle --kontrol .` yalnızca denetler ve biçimlenmesi gereken dosya varsa 1 ile çıkar.
- `tr izle egit.tr` programı çalıştırırken hızını canlı gösterir (ms/adım, en pahalı tensör işlemleri, en sıcak satırlar, GPU ve bellek); `tr profil egit.tr` bitince hız raporu verir (`--json koşu_klasörü/` ile `speed.json`). Ayrıntılar: [`docs/guide/11_hizi_izlemek.md`](docs/guide/11_hizi_izlemek.md).
- `tr kodla dosya.tr` (ya da klasör) dosyayı dilden bağımsız kodlu biçime çevirir (`yazdır` → `TR_fn_print`); `tr göster dosya.tr` dosyayı değiştirmeden kişinin dilinde basar (`--dil en` ile istenen dilde). Kodlu dosya çevrilmeden de çalışır; adlarınız, metinler ve yorumlar değişmez. VS Code'da kodlu dosya kişinin yazım dilinde (ayar `tr.writingLanguage`) görünür ve düzenlenir, kaydedince yine kodlu yazılır; satırlar hata mesajları ve hata ayıklayıcıyla aynıdır ([eklenti belgesi](editors/vscode/README.md#kodlu-dosyalar)). Kurallar: [`docs/spec/17_kodlar.md`](docs/spec/17_kodlar.md).

## Tensörler ve otomatik türev

```tr
//! tr kodlu 3 dil=tr
// y = 2·x - 1 doğrusunu öğren
TR_kw_var X = TR_fn_tensor([[0], [1], [2], [3]])
TR_kw_var Y = TR_fn_tensor([[-1], [1], [3], [5]])
TR_kw_var w = TR_fn_tensor([[0.0]], TR_arg_requires_grad: TR_kw_true)
TR_kw_var b = TR_fn_tensor([0.0], TR_arg_requires_grad: TR_kw_true)
TR_kw_var iyileştirici = TR_fn_AdamW([w, b], TR_arg_lr: 0.1, TR_arg_weight_decay: 0)

TR_kw_for adım TR_kw_in 0..500 {
    iyileştirici.TR_mem_zero_grad()
    TR_kw_var fark = X @ w + b - Y
    TR_fn_backward(TR_fn_mean(fark ** 2))
    iyileştirici.TR_mem_step()
}
TR_fn_print("w = {TR_fn_round(w.TR_mem_tolist[0][0], 3)}, b = {TR_fn_round(b.TR_mem_tolist[0], 3)}")   // w = 2.0, b = -1.0
```

Dilin tek sayısal dizi türü tensördür: matrisler 2 boyutlu tensördür (`TR_fn_tensor([[1, 2], [3, 4]])`), elemanlar varsayılan olarak `float32`'dir, `bf16` ve `fp16` de seçilebilir; hesaplar en az `float32` ile yapılır. Tensörler `TR_kw_for satır TR_kw_in t` ile ilk ekseni boyunca gezilir ve `t[i] = d` ile yerinde değiştirilir. `@` matris çarpımıdır; `+ - * /` eleman bazlı ve yayınlamalıdır. `TR_fn_relu`, `TR_fn_gelu`, `TR_fn_softmax`, `TR_fn_layer_norm`, `TR_fn_embedding`, `TR_fn_cross_entropy`, `TR_fn_SGD`, `TR_fn_clip_grad`, `TR_fn_save_tensors` / `TR_fn_load_tensors` ve fazlası için: [Kullanım Kılavuzu §4.C](docs/guide/04_yapay_zeka_ve_veri.md#c-tensörler-ve-otomatik-türev).

## Veri ve parçalayıcı

```tr
//! tr kodlu 3 dil=tr
TR_kw_var p = TR_fn_load_tokenizer("tokenizer.json")
TR_fn_print(p.TR_mem_encode("Merhaba dünya"), p.TR_mem_vocab_size)

TR_kw_var küme = TR_fn_dataset("veri/train")                       // .npy veri dosyaları
TR_kw_var ö = küme.TR_mem_sampler(TR_arg_batch_size: 8, TR_arg_context: 256, TR_arg_seed: 1)
TR_kw_var girdi, hedef = ö.TR_mem_next()                              // [8, 256] tensörler
```

`TR_fn_load_tokenizer` yaygın `tokenizer.json` biçimini okur; `TR_fn_dataset` büyük derlemleri belleğe almadan parça parça okur, sonraki toplu işleri arka planda hazırlar. Uçtan uca örnek: [`examples/tr/39_veri_ile_egitim.tr`](examples/tr/39_veri_ile_egitim.tr); ayrıntılar: [Kılavuz §4.D](docs/guide/04_yapay_zeka_ve_veri.md#d-veri-ve-parçalayıcı).

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
| Liste üreteci `[x * x TR_kw_for x TR_kw_in 1..=5 TR_kw_if x % 2 == 1]` | Python | [5.11](docs/spec/05_deyimler.md) |
| `eşle konu { 1 => ..., _ => ... }` (desen değerle karşılaştırılır) | Rust'ın `match` yazımı | [5.6](docs/spec/05_deyimler.md) |
| Sondaki blok: `l.filtrele { o > 2 }`, tek parametre `TR_ctx_it` | Kotlin'in sondaki lambdası ve `it`'i | [6.7](docs/spec/06_fonksiyonlar.md) |
| `TR_kw_defer`: fonksiyon bitince, ters sırayla çalışır | Go `defer` | [9.1](docs/spec/09_ertele.md) |
| `TR_kw_async` görev ve yorumlayıcı kilidi | Go'nun `go` deyimi; kilit Python'unki gibi | [10.1, 10.2](docs/spec/10_eszamanlilik.md) |
| Başı `_` olan ad modüle özeldir ve dışarıdan erişilemez | Python'un yazımı; Python'da yalnızca gelenek, TR'de kural | [11.7](docs/spec/11_moduller.md) |
| `TR_ctx_test "ad" { ... }` programla aynı dosyada; normal çalıştırmada atlanır | Zig | [12.1](docs/spec/12_testler.md) |
| Tensör kuralları: yayınlama, tür yükseltme, indeks, dilim | PyTorch ve NumPy, satır satır karşılaştırıldı | [15.49](docs/spec/15_tensorler.md) |
| Özel değerler (±0, ±∞, NaN) CPU'da ve GPU'da aynı | PyTorch 2.14 ve NumPy 2.1 ile ölçüldü; farklar gerekçeli | [15.50](docs/spec/15_tensorler.md) |
| `TR_fn_gelu(TR_arg_approximate: TR_kw_true)`: seçenek Mantık bayraktır | JAX ve TensorFlow (PyTorch'un `"tanh"` metni yazım hatasına açık) | [15.17](docs/spec/15_tensorler.md) |
| Dikkatin tensör düzeni `(toplu, uzunluk, bakış, boyut)` | JAX ve FlashAttention (çevirme kopyası olmaz) | [15.24](docs/spec/15_tensorler.md) |
| Bellek: ara sonuçlar işlemden hemen sonra, depolama son sahibi gidince bırakılır | Mojo, PyTorch, TensorFlow.js `tidy` | [15.43](docs/spec/15_tensorler.md) |
| `TR_fn_fuse`: dışarıdan okunan sayılar ilk çağrıda sabitlenir | JAX `jit` | [15.44](docs/spec/15_tensorler.md) |
| `TR_fn_replay`: GPU adımını bir kez kaydedip oynatır, girdileri kendisi kopyalar | CUDA Graphs (PyTorch bu işi kullanıcıya bırakır) | [15.45](docs/spec/15_tensorler.md) |
| safetensors okumasının denetimleri | safetensors'un kendi okuyucusu (yinelenen anahtar ayrıca reddedilir) | [15.47](docs/spec/15_tensorler.md) |
| Günlükte NaN ve sonsuz `"NaN"`, `"Infinity"` olarak yazılır | Python `json.dumps` (`float()` geri okur) | [19.5](docs/spec/19_surec_ve_kayit.md) |

**TR'ye özgü:** kod geliştiricinin kendi dilinde yazılır ve okunur, dosya dilden bağımsız kodlarla saklanır. Karşılaştırılan dillerin hiçbiri (Python, Java, Swift, Go) değerin metnini yerelleştirmez; dosyanın dili Rust'ın `edition`'ı ve Python'un PEP 263 başlığı gibi dosyanın başında bildirilir ([17.7](docs/spec/17_kodlar.md)). `tr` komutları da her yazım dilinin adıyla çalışır; Go, git ve cargo komut adlarını tek (İngilizce) yazımda tutar ([17.8](docs/spec/17_kodlar.md)).

## VS Code eklentisi ve hata ayıklama

[`editors/vscode/`](editors/vscode/README.md) `.tr` dosyalarına renklendirme, kod şablonları (`TR_fn_tensor` yazıp `Tab`: küçük bir eğitim döngüsü), çalıştırma (▶) ve hata ayıklama getirir. **F5** ile hata ayıklarken **İzleme Masası** paneli tüm değişkenleri tek tabloda gösterir; listeler, tensörler ve veri kümeleri tablo olarak açılır, değerler çalışma anında değiştirilebilir. Terminalde hata ayıklamak için: `tr hata_ayıkla dosya.tr`.

Bu depoyu VS Code'da klasör olarak açtığınızda `Ctrl+Shift+B` açık dosyayı `bin/tr.exe` ile çalıştırır (`.vscode/tasks.json`). Çalıştırma görevleri yorumlayıcıyı **derlemez** (açık bir `tr.exe` Windows'ta dosyayı kilitlediği için otomatik derleme bilerek kapalıdır); Go kodunu değiştirdiyseniz önce `Ctrl+Shift+P` → "Tasks: Run Task" → **TR: yorumlayıcıyı derle** görevini çalıştırın.

## Örnekler

[`examples/`](examples/) basitten zora numaralı bir öğrenme yoludur: her örnek tek bir yeni kavram getirir, öncekilerde öğretilmemiş hiçbir şeyi kullanmaz; başında ne öğrettiğini ve önceki örnekten ne aldığını söyler, sonunda "Kendin dene" alıştırmaları vardır. **Örnekler kodlu saklanır** ([`examples/tr/`](examples/tr/)): aynı dosya her gösterim dilinde okunur, `tr göster examples/tr/01_merhaba.tr --dil en` İngilizce gösterir (bkz. [dil tanımı 17](docs/spec/17_kodlar.md)). Proje kökünden çalıştırın: `./bin/tr.exe examples/tr/01_merhaba.tr`.

| No | Örnek | Ne öğretir |
| :-- | :-- | :-- |
| 01 | [`01_merhaba.tr`](examples/tr/01_merhaba.tr) | Merhaba: ilk TR programı |
| 02 | [`02_degiskenler.tr`](examples/tr/02_degiskenler.tr) | Değişkenler: bir değeri adlandırıp saklamak |
| 03 | [`03_tipler.tr`](examples/tr/03_tipler.tr) | Tipler: her değerin bir tipi vardır |
| 04 | [`04_metin.tr`](examples/tr/04_metin.tr) | Metin: metne değer yerleştirmek ve metinle çalışmak |
| 05 | [`05_kosullar.tr`](examples/tr/05_kosullar.tr) | Koşullar: programın karar vermesi |
| 06 | [`06_girdi.tr`](examples/tr/06_girdi.tr) | Girdi: kullanıcıdan veri okumak |
| 07 | [`07_donguler.tr`](examples/tr/07_donguler.tr) | Döngüler: bir işi yinelemek |
| 08 | [`08_fonksiyonlar.tr`](examples/tr/08_fonksiyonlar.tr) | Fonksiyonlar: işi adlandırıp yeniden kullanmak |
| 09 | [`09_listeler.tr`](examples/tr/09_listeler.tr) | Listeler: birçok değeri sıralı tutmak |
| 10 | [`10_ok_fonksiyonlari.tr`](examples/tr/10_ok_fonksiyonlari.tr) | Ok fonksiyonları ve zincirleme: fonksiyonu değer olarak kullanmak |
| 11 | [`11_sozlukler.tr`](examples/tr/11_sozlukler.tr) | Sözlükler: değerleri anahtarla saklamak |
| 12 | [`12_tip_adlari.tr`](examples/tr/12_tip_adlari.tr) | Tip adları: değerlerin tipini sormak ve bildirmek |
| 13 | [`13_matematik.tr`](examples/tr/13_matematik.tr) | Matematik: sayılarla hesap yapan yerleşikler |
| 14 | [`14_yapilar_ve_metotlar.tr`](examples/tr/14_yapilar_ve_metotlar.tr) | Yapılar ve metotlar: ilişkili verileri ve onlara bağlı işleri bir arada tutmak |
| 15 | [`15_hata_yonetimi.tr`](examples/tr/15_hata_yonetimi.tr) | Hata yönetimi: beklenmedik durumda programı düzgün yönetmek |
| 16 | [`16_ertele.tr`](examples/tr/16_ertele.tr) | Ertele: temizliği her durumda yapmak |
| 17 | [`17_testler_test.tr`](examples/tr/17_testler_test.tr) | Testler: kodun yanına test yazmak |
| 18 | [`18_moduller.tr`](examples/tr/18_moduller.tr) | Modüller: kodu dosyalara bölüp aktarmak |
| 19 | [`19_dosya_ve_json.tr`](examples/tr/19_dosya_ve_json.tr) | Dosya ve JSON: veriyi diske yazıp geri okumak |
| 20 | [`20_yollar_ve_klasorler.tr`](examples/tr/20_yollar_ve_klasorler.tr) | Yollar ve klasörler: dosya sistemini yönetmek |
| 21 | [`21_csv_ve_ortam.tr`](examples/tr/21_csv_ve_ortam.tr) | CSV, ortam değişkenleri ve zaman: dış dünyadan veri almak |
| 22 | [`22_eszamanli_gorevler.tr`](examples/tr/22_eszamanli_gorevler.tr) | Eşzamanlı görevler: işleri aynı anda yürütmek |
| 23 | [`23_tensor.tr`](examples/tr/23_tensor.tr) | Tensörler: yapay zekânın sayı dizileri |
| 24 | [`24_tensor_matematigi.tr`](examples/tr/24_tensor_matematigi.tr) | Tensör matematiği: işlemleri bir tensörün her elemanına uygulamak |
| 25 | [`25_secme_ve_siralama.tr`](examples/tr/25_secme_ve_siralama.tr) | Seçme ve sıralama: tensörde koşula göre seçmek, en büyükleri bulmak |
| 26 | [`26_turev.tr`](examples/tr/26_turev.tr) | Otomatik türev: gradyanı bilgisayara hesaplatmak |
| 27 | [`27_ozel_turev.tr`](examples/tr/27_ozel_turev.tr) | Özel türev: kendi türevlenebilir işlemini yazmak ve doğrulamak |
| 28 | [`28_egitim_dongusu.tr`](examples/tr/28_egitim_dongusu.tr) | Eğitim döngüsü: bir modelin veriden öğrenmesi |
| 29 | [`29_iyilestiriciler.tr`](examples/tr/29_iyilestiriciler.tr) | İyileştiriciler: ağırlıkları güncellemenin yolları |
| 30 | [`30_xor_agi.tr`](examples/tr/30_xor_agi.tr) | Küçük sinir ağı: iki katmanlı bir ağ XOR'u öğrenir |
| 31 | [`31_katmanlar.tr`](examples/tr/31_katmanlar.tr) | Katmanlar: sinir ağlarının hazır yapı taşları |
| 32 | [`32_agirliklari_kaydet.tr`](examples/tr/32_agirliklari_kaydet.tr) | Ağırlıkları kaydetmek: eğitilmiş modeli diske yazıp geri yüklemek |
| 33 | [`33_cihaz_ve_hassasiyet.tr`](examples/tr/33_cihaz_ve_hassasiyet.tr) | Cihaz ve hassasiyet: tensörlerin nerede ve hangi sayı türüyle yaşadığı |
| 34 | [`34_metin_uretimi.tr`](examples/tr/34_metin_uretimi.tr) | Metin üretimi: modelin çıktısından örnek seçmek |
| 35 | [`35_transformer.tr`](examples/tr/35_transformer.tr) | Transformer: elle yazılmış küçük bir transformer |
| 36 | [`36_karakter_modeli.tr`](examples/tr/36_karakter_modeli.tr) | Karakter modeli: gerçek metinle eğitilen küçük bir dil modeli (~100 bin parametre) |
| 37 | [`37_komut_satirinda_ayar.tr`](examples/tr/37_komut_satirinda_ayar.tr) | Komut satırından ayar: programı koda dokunmadan ayarlamak |
| 38 | [`38_ilerleme_ve_gunluk.tr`](examples/tr/38_ilerleme_ve_gunluk.tr) | İlerleme çubuğu ve günlük: uzun işi izlemek ve kaydını tutmak |
| 39 | [`39_veri_ile_egitim.tr`](examples/tr/39_veri_ile_egitim.tr) | Veri ile eğitim: parçalayıcı ve veri yükleyiciyle uçtan uca küçük bir dil modeli |
| 40 | [`40_kontrol_noktasi.tr`](examples/tr/40_kontrol_noktasi.tr) | Kontrol noktası: uzun eğitimi kaldığı yerden sürdürmek |
| 41 | [`41_gpt.tr`](examples/tr/41_gpt.tr) | Hazır katmanlarla GPT: dilin kitaplığındaki katmanlar ve iyileştiricilerle küçük bir dil modeli |
| 42 | [`42_hata_ayiklama.tr`](examples/tr/42_hata_ayiklama.tr) | Hata ayıklama: koddaki kesme noktasıyla bir hesap hatasını çalışırken bulup düzeltmek |
| 43 | [`43_hata_ayiklama_alistirmasi.tr`](examples/tr/43_hata_ayiklama_alistirmasi.tr) | Hata ayıklama alıştırması: VS Code hata ayıklayıcısıyla bir programı adım adım izlemek |
| 44 | [`44_egitim_kararliligi.tr`](examples/tr/44_egitim_kararliligi.tr) | Eğitimin kararlılığı: NaN'i ve kayıp sıçramasını yakalamak, son kontrol noktasına dönüp sorunlu toplu işleri atlamak |
| 45 | [`45_tekrar_oynat.tr`](examples/tr/45_tekrar_oynat.tr) | Kaydedip oynatmak: sık tekrarlanan küçük bir GPU adımını bir kez kaydedip tek seferde oynatmak (`TR_fn_replay`, `TR_fn_write_slice`) |
| 46 | [`46_hazir_model_ve_uretim.tr`](examples/tr/46_hazir_model_ve_uretim.tr) | Hazır model ve üretim: Hugging Face biçimindeki bir modeli yüklemek (`TR_fn_pretrained_model`, `ağırlık_türü`) ve `TR_mem_generate` ile parça üretmek (küçük, rastgele ağırlıklı model `tests/data/hf_tiny`) |
| 47 | [`47_kendi_uretim_dongusu.tr`](examples/tr/47_kendi_uretim_dongusu.tr) | Kendi üretim döngün: `TR_mem_build_cache`, önbellekli `ileri`, `TR_mem_strip`; elle yazılan döngü `TR_mem_generate` ile aynı diziyi verir; `gizli_durum` + `TR_mem_single_step` + `TR_fn_replay` |
| 48 | [`48_egitim_kosusu.tr`](examples/tr/48_egitim_kosusu.tr) | Eğitim koşusu: kayıt işlerini kitaplığa bırakmak (`TR_type_Run`: klasör, günlük, çubuk, kontrol noktası, ağırlıklar önce) ve uzun koşudan önce `prova` |

**Hata örnekleri.** [`examples/tr/hatalar/`](examples/tr/hatalar/) sık yapılan hataları tek tek gösterir: her dosya bir hatayı bilerek yapar, yorumunda iletinin kodunu, nedenini ve düzeltilmiş hâlini yazar (çıkış kodu 1).

| No | Örnek | Hata |
| :-- | :-- | :-- |
| 01 | [`01_sozdizimi_kapanmayan_blok.tr`](examples/tr/hatalar/01_sozdizimi_kapanmayan_blok.tr) | Kapatılmayan blok (`H0156`) |
| 02 | [`02_sozdizimi_ad_bekleniyor.tr`](examples/tr/hatalar/02_sozdizimi_ad_bekleniyor.tr) | Ad bekleniyordu (`H0100`) |
| 03 | [`03_sozdizimi_kapanmayan_metin.tr`](examples/tr/hatalar/03_sozdizimi_kapanmayan_metin.tr) | Kapatılmayan metin (`H0010`) |
| 04 | [`04_tanimsiz_ad.tr`](examples/tr/hatalar/04_tanimsiz_ad.tr) | Tanımsız ad (`H1010`) |
| 05 | [`05_tip_uyusmazligi.tr`](examples/tr/hatalar/05_tip_uyusmazligi.tr) | Tip uyuşmazlığı (`H1001`) |
| 06 | [`06_sabit_degistirme.tr`](examples/tr/hatalar/06_sabit_degistirme.tr) | Sabiti değiştirmek (`H1002`) |
| 07 | [`07_yanlis_tip_arguman.tr`](examples/tr/hatalar/07_yanlis_tip_arguman.tr) | Yanlış tipte argüman (`H1004`) |
| 08 | [`08_eksik_arguman.tr`](examples/tr/hatalar/08_eksik_arguman.tr) | Eksik argüman (`H2414`) |
| 09 | [`09_bilinmeyen_adli_arguman.tr`](examples/tr/hatalar/09_bilinmeyen_adli_arguman.tr) | Bilinmeyen adlı argüman (`H2410`) |
| 10 | [`10_sinir_disi_indeks.tr`](examples/tr/hatalar/10_sinir_disi_indeks.tr) | Sınır dışı indeks (`H2515`) |
| 11 | [`11_sifira_bolme.tr`](examples/tr/hatalar/11_sifira_bolme.tr) | Sıfıra bölme (`H2470`) |
| 12 | [`12_yakalanmamis_firlat.tr`](examples/tr/hatalar/12_yakalanmamis_firlat.tr) | Yakalanmayan fırlat (`H2441`) |
| 13 | [`13_yok_uzerinde_ozellik.tr`](examples/tr/hatalar/13_yok_uzerinde_ozellik.tr) | yok değerin özelliği (`H2466`) |
| 14 | [`14_alan_bulunamadi.tr`](examples/tr/hatalar/14_alan_bulunamadi.tr) | Olmayan alan (`H2468`) |
| 15 | [`15_donusum_hatasi.tr`](examples/tr/hatalar/15_donusum_hatasi.tr) | Dönüşüm hatası (`H2429`) |
| 16 | [`16_fonksiyon_degil.tr`](examples/tr/hatalar/16_fonksiyon_degil.tr) | Fonksiyon olmayanı çağırmak (`H2421`) |
| 17 | [`17_dongu_tipi.tr`](examples/tr/hatalar/17_dongu_tipi.tr) | Gezilemeyen değer (`H2443`) |
| 18 | [`18_sonsuz_ozyineleme.tr`](examples/tr/hatalar/18_sonsuz_ozyineleme.tr) | Sonsuz özyineleme (`H2422`) |
| 19 | [`19_dosya_bulunamadi.tr`](examples/tr/hatalar/19_dosya_bulunamadi.tr) | Dosya bulunamadı (`H2100`) |
| 20 | [`20_modul_bulunamadi.tr`](examples/tr/hatalar/20_modul_bulunamadi.tr) | Modül bulunamadı (`H2112`) |
| 21 | [`21_tensor_sekil.tr`](examples/tr/hatalar/21_tensor_sekil.tr) | Tensör şekil uyuşmazlığı (`H6126`) |
| 22 | [`22_tensor_eksen.tr`](examples/tr/hatalar/22_tensor_eksen.tr) | Geçersiz eksen (`H6016`) |
| 23 | [`23_devam_disinda.tr`](examples/tr/hatalar/23_devam_disinda.tr) | Döngü dışında devam (`H0100`) |

Komut satırı adları (`tr test`, `TR_cmd_format`, `TR_cmd_encode`, hata ayıklayıcı komutları ...) [`examples/README.md`](examples/README.md)'dedir.

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
| `cmd/tr` | `tr` komutu: kabuk, `TR_cmd_run`, `kontrol`, `TR_cmd_build`, `test`, `TR_cmd_format`, `kodla`, `TR_cmd_show`, `izle`, `TR_cmd_profile`, `TR_cmd_debug`, `TR_cmd_index`, `yardım`, `TR_cmd_version`, `TR_cmd_dap` (VS Code için); komut adları her yazım dilinde kabul edilir |
| `internal/lang` | Dil çekirdeği: iletiler (`messages`), Türkçe ekler ve dosya hataları (`turkish`), kod listesi (`codes`, [`docs/codes.md`](docs/codes.md)), ad ipuçları (`hint`), sözcük çözümleyici (`lexer`), sözdizimi ağacı (`ast`), ayrıştırıcı (`parser`), tip denetimi (`checker`), kaynak ve kodlu biçim (`source`), yorumlayıcı (`interp`) |
| `internal/stdlib` | Standart kitaplık: tensör ve öğrenme (`ml`), veri (`dataset`), parçalayıcı (`tokenizer`), dosyalar (`files`), JSON/CSV (`formats`), komut satırı (`process`), günlük ve ilerleme (`logging`), kontrol noktası (`checkpoint`), motorların ortak yardımcıları ve JSON yazımı (`common/bind`, `common/jsonwrite`), GPU'da dil düzeyinde eğitimin sınamaları (`gpuprobe`) |
| `internal/compute` | Dili bilmeyen hesap çekirdekleri: tensör, otomatik türev ve CPU kernelleri (`tensor`), GPU arka ucu (`gpu`), parçalayıcı (`tokenize`), veri yükleyici (`dataload`) |
| `internal/devtools` | Hata ayıklayıcı (`debugger`), biçimlendirici (`formatter`), `tr test` çekirdeği (`testrunner`), canlı profil (`profiler`: `TR_cmd_watch`, `TR_cmd_profile`), TR kodunun dizini (`codeindex`: `TR_cmd_index`), Go modülünü testleriyle tip denetiminden geçirip yükleyen ortak paket (`goload`: `codegraph` ve `impact` için) |
| `lib/` | TR ile yazılan standart kitaplık, ikiliye gömülü: transformer katmanları ve GPT (`TR_kw_import "tr/katman"`), Muon/NorMuon (`TR_kw_import "tr/iyilestirici"`), eğitim koşusunun kayıt işleri (`TR_kw_import "tr/egitim"`: `TR_type_Run`, prova), Hugging Face biçimindeki hazır modeller (`TR_kw_import "tr/hazir"`); [`lib/README.md`](lib/README.md) |
| `editors/vscode/` | VS Code eklentisi |
| `docs/` | Belgeler ([dizin](docs/README.md)); kod kuralları [`internal/README.md`](internal/README.md), ekleme tarifleri [`docs/recipes.md`](docs/recipes.md) |
| `examples/`, `tests/` | Örnek programlar ve dil tanımının kural testleri |
| `tools/` | Depo bakımı: `full_check.sh`, `fuzz.sh`, üreteçler (`gen/`), hız kıyas seti (`bench/`), GPU ölçümleri (`gpu/`), ölçüm kaydı (`record/`), Go kodunun grafiği (`codegraph/`), etki sorgusu (`impact/`), dokunmadan önce koruyan testleri gösteren düzenleme kancası (`edithook/`), dil tanımı maddelerinin yeniden numaralanması (`renumber/`), Hugging Face modelleriyle karşılaştırma (`hf/`), GPU'suz ikilinin statik olduğunun denetimi (`checkstatic/`), araçların ortak paketleri (`internal/`) |

Katmanlar, aralarındaki bağımlılık kuralı ve her paketin görevi: [`docs/architecture.md`](docs/architecture.md).

## Lisans

TR, [Apache Lisansı 2.0](LICENSE) ile dağıtılır: kodu kullanabilir, değiştirebilir ve dağıtabilirsiniz; lisans metni ve değişiklik bildirimleri korunmalıdır.
