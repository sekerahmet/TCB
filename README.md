# TR — yapay zekâ için Türkçe programlama dili

TR, yapay zekâ modellerini **eğiten** ve **kullanan** geliştiriciler için tasarlanmış, anahtar kelimeleri Türkçe bir programlama dilidir. Tensörler, otomatik türev, iyileştiriciler, `tokenizer.json` parçalayıcıları ve büyük veri kümeleri dilin içinde yerleşik olarak gelir; testler de aynı dosyada yazılır. Hata mesajları Türkçedir ve sorunu satırıyla gösterir.

Her işin tek yazımı vardır: anahtar kelimeler, yerleşikler ve komutlar yalnızca Türkçe adlarıyla yazılır (`değişken`, `yazdır`, `tr hata_ayıkla`). Türkçe harfsiz ya da eski bir ad yazılırsa (`degisken`, `yazdir`, `matris`) hata mesajı doğru adı önerir: `'yazdir' TR'de yok; 'yazdır' yazmak mı istediniz?`

## Kurulum

Yorumlayıcıyı proje kökünde derleyin (`go` derleyicisi kurulu olmalıdır); çıktı `bin/` klasörüne yazılır ve bu klasör depoya girmez:

```bash
go build -o bin/tr.exe ./cmd/tr
```

Program dosyası her x86-64 işlemcide çalışır: matris çarpımı ve vektör çekirdekleri çalışma anında seçilir. AVX2/FMA'lı işlemcide (Intel 2013, AMD 2015 sonrası) assembly çekirdekleri, öbürlerinde taşınabilir Go eşleri kullanılır; eğitim adımı AVX2'li yolda yaklaşık 8 kat hızlıdır (karakter modeli, 2 çekirdek: ~10 ve ~85 ms/adım; ölçüm: [`tools/bench/`](tools/bench/README.md)). `GOAMD64=v3` ile derlemek ölçümde hız kazandırmıyor, yalnızca AVX2'siz işlemcide açılmayan bir program dosyası verir. `TR_CPU_AVX2=0` çekirdekleri kapatır (sorun ayıklama için). ARM (arm64) taşınabilir çekirdekleri kullanır.

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
- `tr kodla dosya.tr` (ya da klasör) dosyayı dilden bağımsız kodlu biçime çevirir (`yazdır` → `G100`); `tr çöz` Türkçe yazıma geri çevirir, `tr göster dosya.tr` kodlu dosyayı değiştirmeden Türkçe basar. Kodlu dosya çevrilmeden de çalışır; adlarınız, metinler ve yorumlar değişmez. VS Code'da kodlu dosya Türkçe görünür ve düzenlenir, kaydedince yine kodlu yazılır; satırlar hata mesajları ve hata ayıklayıcıyla aynıdır ([eklenti belgesi](editors/vscode/README.md#kodlu-dosyalar)). Kurallar: [`docs/spec/17_kodlar.md`](docs/spec/17_kodlar.md).

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

Dilin tek sayısal dizi türü tensördür: matrisler 2 boyutlu tensördür (`tensör([[1, 2], [3, 4]])`), hesaplar `float32` ile yapılır. Tensörler `döngü satır içinde t` ile ilk ekseni boyunca gezilir ve `t[i] = d` ile yerinde değiştirilir. `@` matris çarpımıdır; `+ - * /` eleman bazlı ve yayınlamalıdır. `relu`, `gelu`, `softmax`, `katman_normu`, `gömme`, `çapraz_entropi`, `SGD`, `gradyan_kırp`, `tensör_kaydet` / `tensör_yükle` ve fazlası için: [Kullanım Kılavuzu §4.C](docs/guide/04_yapay_zeka_ve_veri.md#c-tensörler-ve-otomatik-türev).

## Veri ve parçalayıcı

```tr
değişken p = parçalayıcı_yükle("tokenizer.json")
yazdır(p.kodla("Merhaba dünya"), p.dağarcık_boyutu)

değişken küme = veri_kümesi("veri/train")                       // .npy veri dosyaları
değişken ö = küme.örnekleyici(toplu: 8, bağlam: 256, tohum: 1)
değişken girdi, hedef = ö.sonraki()                              // [8, 256] tensörler
```

`parçalayıcı_yükle` yaygın `tokenizer.json` biçimini okur; `veri_kümesi` büyük derlemleri belleğe almadan parça parça okur, sonraki toplu işleri arka planda hazırlar. Uçtan uca örnek: [`examples/tr/16_veri_ile_egitim.tr`](examples/tr/16_veri_ile_egitim.tr); ayrıntılar: [Kılavuz §4.D](docs/guide/04_yapay_zeka_ve_veri.md#d-veri-ve-parçalayıcı).

## VS Code eklentisi ve hata ayıklama

[`editors/vscode/`](editors/vscode/README.md) `.tr` dosyalarına renklendirme, kod şablonları (`tensör` yazıp `Tab`: küçük bir eğitim döngüsü), çalıştırma (▶) ve hata ayıklama getirir. **F5** ile hata ayıklarken **İzleme Masası** paneli tüm değişkenleri tek tabloda gösterir; listeler, tensörler ve veri kümeleri tablo olarak açılır, değerler çalışma anında değiştirilebilir. Terminalde hata ayıklamak için: `tr hata_ayıkla dosya.tr`.

Bu depoyu VS Code'da klasör olarak açtığınızda `Ctrl+Shift+B` açık dosyayı `bin/tr.exe` ile çalıştırır (`.vscode/tasks.json`). Çalıştırma görevleri yorumlayıcıyı **derlemez** (açık bir `tr.exe` Windows'ta dosyayı kilitlediği için otomatik derleme bilerek kapalıdır); Go kodunu değiştirdiyseniz önce `Ctrl+Shift+P` → "Tasks: Run Task" → **TR: yorumlayıcıyı derle** görevini çalıştırın.

## Örnekler

[`examples/`](examples/) basitten zora numaralı bir öğrenme yoludur: her örnek tek bir şeyi öğretir, baştaki yorumda ne öğrettiğini söyler ve kendi başına çalışır. **Her örnek iki yazımla bulunur:** [`examples/tr/`](examples/tr/) Türkçe, [`examples/en/`](examples/en/) İngilizce; aynı numara aynı programdır (İngilizce dosyalar kodlu saklanır, `tr göster --dil en` ile okunur; bkz. [dil tanımı 17](docs/spec/17_kodlar.md)). Proje kökünden çalıştırın: `./bin/tr.exe examples/tr/01_merhaba.tr` ya da `./bin/tr.exe examples/en/01_hello.tr`.

| No | Türkçe | İngilizce | Ne öğretir |
| :-- | :-- | :-- | :-- |
| 01 | [`01_merhaba.tr`](examples/tr/01_merhaba.tr) | [`01_hello.tr`](examples/en/01_hello.tr) | İlk program: değişken, `yazdır`, metne değer yerleştirme, liste, fonksiyon |
| 02 | [`02_degiskenler_ve_tipler.tr`](examples/tr/02_degiskenler_ve_tipler.tr) | [`02_variables_and_types.tr`](examples/en/02_variables_and_types.tr) | Tipler, sabit, dönüşümler, işleçler, `yok` güvenliği (`??`) |
| 03 | [`03_kosullar_ve_donguler.tr`](examples/tr/03_kosullar_ve_donguler.tr) | [`03_conditions_and_loops.tr`](examples/en/03_conditions_and_loops.tr) | `eğer`, `eşle`, `döngü`, aralıklar, `iken`, liste üreteci |
| 04 | [`04_fonksiyonlar.tr`](examples/tr/04_fonksiyonlar.tr) | [`04_functions.tr`](examples/en/04_functions.tr) | Tip bildirimi, varsayılan ve adlı argüman, özyineleme, ok fonksiyonu, sondaki blok |
| 05 | [`05_listeler_ve_sozlukler.tr`](examples/tr/05_listeler_ve_sozlukler.tr) | [`05_lists_and_dicts.tr`](examples/en/05_lists_and_dicts.tr) | Liste ve sözlük, zincirleme metotlar, boru hattı (`\|>`) |
| 06 | [`06_yapilar_ve_metotlar.tr`](examples/tr/06_yapilar_ve_metotlar.tr) | [`06_structs_and_methods.tr`](examples/en/06_structs_and_methods.tr) | `yapı`, alanlar, metotlar, `kendi` |
| 07 | [`07_hata_yonetimi.tr`](examples/tr/07_hata_yonetimi.tr) | [`07_error_handling.tr`](examples/en/07_error_handling.tr) | `dene/yakala`, `fırlat`, `ertele` |
| 08 | [`08_testler_test.tr`](examples/tr/08_testler_test.tr) | [`08_tests_test.tr`](examples/en/08_tests_test.tr) | `test` blokları, `doğrula`, `eşit_mi`; `tr test examples/tr/08_testler_test.tr` ile çalışır |
| 09 | [`09_moduller.tr`](examples/tr/09_moduller.tr) | [`09_modules.tr`](examples/en/09_modules.tr) | `aktar`, `olarak` ile takma ad |
| 10 | [`10_eszamanli_gorevler.tr`](examples/tr/10_eszamanli_gorevler.tr) | [`10_concurrent_tasks.tr`](examples/en/10_concurrent_tasks.tr) | `eşzamanlı`, `bekle_hepsi` |
| 11 | [`11_dosya_ve_json.tr`](examples/tr/11_dosya_ve_json.tr) | [`11_files_and_json.tr`](examples/en/11_files_and_json.tr) | Dosya okuma/yazma, `json_metin`, `json_çöz`, geçici klasör |
| 12 | [`12_tensor_ve_turev.tr`](examples/tr/12_tensor_ve_turev.tr) | [`12_tensors_and_autodiff.tr`](examples/en/12_tensors_and_autodiff.tr) | Tensörler, `@`, yayınlama, otomatik türev, AdamW ile doğrusal regresyon, kaydet/yükle |
| 13 | [`13_xor_agi.tr`](examples/tr/13_xor_agi.tr) | [`13_xor_network.tr`](examples/en/13_xor_network.tr) | Tensörlerle iki katmanlı küçük sinir ağı (XOR) |
| 14 | [`14_transformer.tr`](examples/tr/14_transformer.tr) | [`14_transformer.tr`](examples/en/14_transformer.tr) | Elle yazılmış küçük transformer: çok başlı nedensel dikkat, katman normu |
| 15 | [`15_karakter_modeli.tr`](examples/tr/15_karakter_modeli.tr) | [`15_char_model.tr`](examples/en/15_char_model.tr) | Projenin belgeleriyle eğitilen karakter düzeyi dil modeli ve metin üretimi |
| 16 | [`16_veri_ile_egitim.tr`](examples/tr/16_veri_ile_egitim.tr) | [`16_training_with_data.tr`](examples/en/16_training_with_data.tr) | `tokenizer.json` parçalayıcısı ve `.npy` veri kümesiyle uçtan uca eğitim ve doğrulama |
| 17 | [`17_gpt.tr`](examples/tr/17_gpt.tr) | [`17_gpt.tr`](examples/en/17_gpt.tr) | Hazır katmanlarla GPT, Muon + AdamW, öğrenme oranı planı |
| 18 | [`18_kontrol_noktasi.tr`](examples/tr/18_kontrol_noktasi.tr) | [`18_checkpoints.tr`](examples/en/18_checkpoints.tr) | Uzun eğitim: koşu klasörü, `kontrol_noktası`, Ctrl+C ile durdurup aynı komutla kaldığı yerden (bit düzeyinde aynı) sürdürme |
| 19 | [`19_hata_ayiklama.tr`](examples/tr/19_hata_ayiklama.tr) | [`19_debugging.tr`](examples/en/19_debugging.tr) | `kesme_noktası` ile hata ayıklama, değeri çalışırken değiştirme (etkileşimli) |
| 20 | [`20_hata_ayiklama_alistirmasi.tr`](examples/tr/20_hata_ayiklama_alistirmasi.tr) | [`20_debugging_exercise.tr`](examples/en/20_debugging_exercise.tr) | VS Code hata ayıklayıcısıyla adım adım ilerleme: kesme noktası, tablo görünümü, değeri çalışırken değiştirme (F5) |

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
| `cmd/tr` | `tr` komutu: çalıştırma, kabuk, `kontrol`, `derle`, `test`, `biçimle`, `kodla`, `çöz`, `göster`, `izle`, `profil`, `hata_ayıkla`, `yardım`, `sürüm`, `dap` (VS Code için) |
| `internal/lang` | Dil çekirdeği: iletiler (`messages`), Türkçe ekler ve dosya hataları (`turkish`), kod listesi (`codes`, [`docs/codes.md`](docs/codes.md)), ad ipuçları (`hint`), sözcük çözümleyici (`lexer`), sözdizimi ağacı (`ast`), ayrıştırıcı (`parser`), tip denetimi (`checker`), kaynak ve kodlu biçim (`source`), yorumlayıcı (`interp`) |
| `internal/stdlib` | Standart kitaplık: tensör ve öğrenme (`ml`), veri (`dataset`), parçalayıcı (`tokenizer`), dosyalar (`files`), JSON/CSV (`formats`), komut satırı (`process`), günlük ve ilerleme (`logging`), kontrol noktası (`checkpoint`) |
| `internal/compute` | Dili bilmeyen hesap çekirdekleri: tensör, otomatik türev ve CPU çekirdekleri (`tensor`), GPU arka ucu (`gpu`), parçalayıcı (`tokenize`), veri yükleyici (`dataload`) |
| `internal/devtools` | Hata ayıklayıcı (`debugger`), biçimlendirici (`formatter`), `tr test` çekirdeği (`testrunner`), canlı profil (`profiler`: `tr izle`, `tr profil`) |
| `lib/` | TR ile yazılan standart kitaplık, ikiliye gömülü: transformer katmanları ve GPT (`aktar "tr/katman"`), Muon/NorMuon (`aktar "tr/iyilestirici"`) |
| `editors/vscode/` | VS Code eklentisi |
| `docs/` | Belgeler ([dizin](docs/README.md)); kod kuralları [`internal/README.md`](internal/README.md), ekleme tarifleri [`docs/recipes.md`](docs/recipes.md) |
| `examples/`, `tests/` | Örnek programlar ve dil tanımının kural testleri |
| `tools/` | Depo bakımı: `full_check.sh`, `fuzz.sh`, üreteçler (`gen/`), hız kıyas seti (`bench/`), GPU ölçümleri (`gpu/`), etki sorgusu (`impact/`) |

Katmanlar, aralarındaki bağımlılık kuralı ve her paketin görevi: [`docs/architecture.md`](docs/architecture.md).

## Lisans

TR, [Apache Lisansı 2.0](LICENSE) ile dağıtılır: kodu kullanabilir, değiştirebilir ve dağıtabilirsiniz; lisans metni ve değişiklik bildirimleri korunmalıdır.
