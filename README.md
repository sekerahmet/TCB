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

`parçalayıcı_yükle` yaygın `tokenizer.json` biçimini okur; `veri_kümesi` büyük derlemleri belleğe almadan parça parça okur, sonraki toplu işleri arka planda hazırlar. Uçtan uca örnek: [`examples/08_veri_ile_egitim.tr`](examples/08_veri_ile_egitim.tr); ayrıntılar: [Kılavuz §4.D](docs/guide/04_yapay_zeka_ve_veri.md#d-veri-ve-parçalayıcı).

## VS Code eklentisi ve hata ayıklama

[`editors/vscode/`](editors/vscode/README.md) `.tr` dosyalarına renklendirme, kod şablonları (`tensör` yazıp `Tab`: küçük bir eğitim döngüsü), çalıştırma (▶) ve hata ayıklama getirir. **F5** ile hata ayıklarken **İzleme Masası** paneli tüm değişkenleri tek tabloda gösterir; listeler, tensörler ve veri kümeleri tablo olarak açılır, değerler çalışma anında değiştirilebilir. Terminalde hata ayıklamak için: `tr hata_ayıkla dosya.tr`.

Bu depoyu VS Code'da klasör olarak açtığınızda `Ctrl+Shift+B` açık dosyayı `bin/tr.exe` ile çalıştırır (`.vscode/tasks.json`). Çalıştırma görevleri yorumlayıcıyı **derlemez** (açık bir `tr.exe` Windows'ta dosyayı kilitlediği için otomatik derleme bilerek kapalıdır); Go kodunu değiştirdiyseniz önce `Ctrl+Shift+P` → "Tasks: Run Task" → **TR: yorumlayıcıyı derle** görevini çalıştırın.

## Örnekler

[`examples/`](examples/) klasöründeki dosyalar sırayla okunacak şekilde numaralıdır; her biri kısa, kendi başına çalışır ve baştaki yorumda ne öğrettiğini anlatır. Proje kökünden çalıştırın: `./bin/tr.exe examples/01_merhaba.tr`.

| Dosya | Ne öğretir |
| :-- | :-- |
| [`01_merhaba.tr`](examples/01_merhaba.tr) | İlk program: değişken, `yazdır`, metne değer yerleştirme, liste, fonksiyon |
| [`02_dil_turu.tr`](examples/02_dil_turu.tr) | Dilin turu: `eşle`, döngüler, liste üreteci, sondaki blok ve `o`, `\|>`, yapılar, `dene/yakala`, `ertele`, `eşzamanlı`, modüller |
| [`03_testler_test.tr`](examples/03_testler_test.tr) | `test` blokları, `doğrula`, `eşit_mi`; `tr test examples/03_testler_test.tr` ile çalışır |
| [`04_hata_ayiklama.tr`](examples/04_hata_ayiklama.tr) | `kesme_noktası` ile hata ayıklama, değeri çalışırken değiştirme (etkileşimli) |
| [`05_tensor_ve_turev.tr`](examples/05_tensor_ve_turev.tr) | Tensörler, `@`, yayınlama, otomatik türev, AdamW ile doğrusal regresyon, tensörlerle iki katmanlı XOR ağı, kaydet/yükle |
| [`06_transformer.tr`](examples/06_transformer.tr) | Elle yazılmış küçük transformer: çok başlı nedensel dikkat, katman normu |
| [`07_karakter_modeli.tr`](examples/07_karakter_modeli.tr) | Projenin belgeleriyle eğitilen karakter düzeyi dil modeli ve metin üretimi |
| [`08_veri_ile_egitim.tr`](examples/08_veri_ile_egitim.tr) | `tokenizer.json` parçalayıcısı ve `.npy` veri kümesiyle uçtan uca eğitim ve doğrulama |
| [`09_hata_ayiklama_alistirmasi.tr`](examples/09_hata_ayiklama_alistirmasi.tr) | VS Code hata ayıklayıcısıyla adım adım ilerleme: kesme noktası, tablo görünümü, değeri çalışırken değiştirme (F5) |
| [`10_kontrol_noktasi.tr`](examples/10_kontrol_noktasi.tr) | Uzun eğitim: koşu klasörü, `kontrol_noktası` ile düzenli kayıt, Ctrl+C ile durdurup aynı komutla kaldığı yerden (bit düzeyinde aynı) sürdürme |

Kendi denemeleriniz için kökte bir `scratch/` klasörü açıp örnekleri oraya kopyalayabilirsiniz (ör. `examples/01_merhaba.tr`); bu klasör depoya girmez.

## Belgeler ve ölçümler

- [`docs/guide/`](docs/guide/README.md) — komutlar, yerleşikler, tensörler ve yapay zekâ, veri ve parçalayıcı, hata ayıklayıcı.
- [`docs/spec/`](docs/spec/README.md) — dilin kuralları; her kuralın `tests/` altında bir testi vardır (`tr test tests`).
- [`docs/kodlar.md`](docs/kodlar.md) — her anahtar kelimenin, tipin, yerleşiğin, metodun ve adlı argümanın ne işe yaradığı, imzası ve örneği (kod listesinden üretilir).
- [`docs/yol_haritasi.md`](docs/yol_haritasi.md) — yapılacak işlerin sırası: GPU, kitaplık, yazım dilleri, platformlar.
- [`docs/mimari.md`](docs/mimari.md) — katmanlar, bağımlılık kuralı ve paketlerin görevleri.
- [`docs/katki.md`](docs/katki.md) — katkı rehberi: dosya ve fonksiyon boyu, adlar, paket belgesi, testler, yeni kod isteği, tam denetim, ölçüm.
- [`tools/bench/`](tools/bench/README.md) — hız kıyas seti ve model eğitimi süre ölçümleri.
- [`SECURITY.md`](SECURITY.md) — güvenlik açığı bildirme, güvenilmeyen sayılan girdiler (`.trt`, `.npy`, `tokenizer.json` ...) ve bulanıklaştırma testleri.

## Depo yapısı

| Klasör | İçerik |
| :-- | :-- |
| `cmd/tr` | `tr` komutu: çalıştırma, kabuk, `kontrol`, `derle`, `test`, `biçimle`, `kodla`, `çöz`, `göster`, `izle`, `profil`, `hata_ayıkla`, `yardım`, `sürüm`, `dap` (VS Code için) |
| `internal/lang` | Dil çekirdeği: kod listesi (`kod`, [`docs/kodlar.md`](docs/kodlar.md)), sözcük çözümleyici (`sozcuk`), sözdizimi ağacı (`agac`), ayrıştırıcı (`ayristir`), tip denetimi (`denetci`), yorumlayıcı (`yurut`), Türkçe ekler (`turkce`) |
| `internal/stdlib` | Tensör ve öğrenme, parçalayıcı, veri, JSON/CSV yerleşiklerini dile bağlayan motorlar |
| `internal/compute` | Dili bilmeyen hesap çekirdekleri: tensör ve CPU çekirdekleri (`tensor`), parçalayıcı (`parca`), veri yükleyici (`veriyolu`) |
| `internal/devtools` | Hata ayıklayıcı (`ayikla`), biçimlendirici (`bicim`), `tr test` çekirdeği (`testkosucu`), canlı profil (`profil`: `tr izle`, `tr profil`) |
| `lib/` | TR ile yazılan standart kitaplık, ikiliye gömülü: transformer katmanları ve GPT (`aktar "tr/katman"`), Muon/NorMuon (`aktar "tr/iyilestirici"`) |
| `editors/vscode/` | VS Code eklentisi |
| `docs/` | Belgeler ([dizin](docs/README.md)) |
| `examples/`, `tests/` | Örnek programlar ve dil tanımının kural testleri |
| `tools/` | Depo bakımı: `tam_denetim.sh`, `bulaniklastir.sh`, üreteçler (`uret/`), hız kıyas seti (`kiyas/`) |

Katmanlar, aralarındaki bağımlılık kuralı ve her paketin görevi: [`docs/mimari.md`](docs/mimari.md).

## Lisans

TR, [Apache Lisansı 2.0](LICENSE) ile dağıtılır: kodu kullanabilir, değiştirebilir ve dağıtabilirsiniz; lisans metni ve değişiklik bildirimleri korunmalıdır.
