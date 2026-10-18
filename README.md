# TR — yapay zekâ için Türkçe programlama dili

TR, yapay zekâ modellerini **eğiten** ve **kullanan** geliştiriciler için tasarlanmış, anahtar kelimeleri Türkçe bir programlama dilidir. Tensörler, otomatik türev, iyileştiriciler, `tokenizer.json` parçalayıcıları ve büyük veri kümeleri dilin içinde yerleşik olarak gelir; testler de aynı dosyada yazılır. Hata mesajları Türkçedir ve sorunu satırıyla gösterir.

Her işin tek yazımı vardır: anahtar kelimeler, yerleşikler ve komutlar yalnızca Türkçe adlarıyla yazılır (`değişken`, `yazdır`, `tr hata_ayıkla`). Türkçe harfsiz ya da eski bir ad yazılırsa (`degisken`, `yazdir`, `matris`) hata mesajı doğru adı önerir: `'yazdir' TR'de yok; 'yazdır' yazmak mı istediniz?`

## Kurulum

Yorumlayıcıyı proje kökünde derleyin (`go` derleyicisi kurulu olmalıdır); çıktı `ikili/` klasörüne yazılır ve bu klasör depoya girmez:

```bash
GOAMD64=v3 go build -o ikili/tr.exe ./cmd/tr          # bash
$env:GOAMD64="v3"; go build -o ikili/tr.exe ./cmd/tr   # PowerShell
```

`GOAMD64=v3` 2020 sonrası x86-64 işlemcileri (AVX2/FMA) hedefler ve matris çarpımını çok hızlandırır (512×512 çarpımda v1 derlemeye göre yaklaşık 8–14 kat; ölçüm: [`bakim/kiyas/`](bakim/kiyas/README.md)). Daha eski ya da bazı düşük güçlü işlemcilerde program açılmıyorsa `GOAMD64=v1` ile derleyin; ARM (arm64) bundan etkilenmez.

## İlk program

```tr
// merhaba.tr
değişken ad = "dünya"
yazdır("Merhaba, {ad}!")

fonksiyon kare(x) { sonuç x * x }
yazdır([1, 2, 3, 4].haritala { kare(o) })   // [1, 4, 9, 16]
```

```bash
./ikili/tr.exe merhaba.tr          # çalıştırır
./ikili/tr.exe kontrol merhaba.tr  # çalıştırmadan sözdizimi ve tip denetimi
./ikili/tr.exe                     # etkileşimli kabuk
```

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

Dilin tek sayısal dizi türü tensördür: matrisler 2 boyutlu tensördür (`tensör([[1, 2], [3, 4]])`), hesaplar `float32` ile yapılır. Tensörler `döngü satır içinde t` ile ilk ekseni boyunca gezilir ve `t[i] = d` ile yerinde değiştirilir. `@` matris çarpımıdır; `+ - * /` eleman bazlı ve yayınlamalıdır. `relu`, `gelu`, `softmax`, `katman_normu`, `gömme`, `çapraz_entropi`, `SGD`, `gradyan_kırp`, `tensör_kaydet` / `tensör_yükle` ve fazlası için: [Kullanım Kılavuzu §4.C](docs/kilavuz/04_yapay_zeka_ve_veri.md#c-tensörler-ve-otomatik-türev).

## Veri ve parçalayıcı

```tr
değişken p = parçalayıcı_yükle("tokenizer.json")
yazdır(p.kodla("Merhaba dünya"), p.dağarcık_boyutu)

değişken küme = veri_kümesi("veri/train")                       // .npy veri dosyaları
değişken ö = küme.örnekleyici(toplu: 8, bağlam: 256, tohum: 1)
değişken girdi, hedef = ö.sonraki()                              // [8, 256] tensörler
```

`parçalayıcı_yükle` yaygın `tokenizer.json` biçimini okur; `veri_kümesi` büyük derlemleri belleğe almadan parça parça okur, sonraki toplu işleri arka planda hazırlar. Uçtan uca örnek: [`ornekler/08_veri_ile_egitim.tr`](ornekler/08_veri_ile_egitim.tr); ayrıntılar: [Kılavuz §4.D](docs/kilavuz/04_yapay_zeka_ve_veri.md#d-veri-ve-parçalayıcı).

## VS Code eklentisi ve hata ayıklama

[`duzenleyici/vscode/`](duzenleyici/vscode/README.md) `.tr` dosyalarına renklendirme, kod şablonları (`tensör` yazıp `Tab`: küçük bir eğitim döngüsü), çalıştırma (▶) ve hata ayıklama getirir. **F5** ile hata ayıklarken **İzleme Masası** paneli tüm değişkenleri tek tabloda gösterir; listeler, tensörler ve veri kümeleri tablo olarak açılır, değerler çalışma anında değiştirilebilir. Terminalde hata ayıklamak için: `tr hata_ayıkla dosya.tr`.

Bu depoyu VS Code'da klasör olarak açtığınızda `Ctrl+Shift+B` açık dosyayı `ikili/tr.exe` ile çalıştırır (`.vscode/tasks.json`). Çalıştırma görevleri yorumlayıcıyı **derlemez** (açık bir `tr.exe` Windows'ta dosyayı kilitlediği için otomatik derleme bilerek kapalıdır); Go kodunu değiştirdiyseniz önce `Ctrl+Shift+P` → "Tasks: Run Task" → **TR: yorumlayıcıyı derle** görevini çalıştırın.

## Örnekler

[`ornekler/`](ornekler/) klasöründeki dosyalar sırayla okunacak şekilde numaralıdır; her biri kısa, kendi başına çalışır ve baştaki yorumda ne öğrettiğini anlatır. Proje kökünden çalıştırın: `./ikili/tr.exe ornekler/01_merhaba.tr`.

| Dosya | Ne öğretir |
| :-- | :-- |
| [`01_merhaba.tr`](ornekler/01_merhaba.tr) | İlk program: değişken, `yazdır`, metne değer yerleştirme, liste, fonksiyon |
| [`02_dil_turu.tr`](ornekler/02_dil_turu.tr) | Dilin turu: `eşle`, döngüler, liste üreteci, sondaki blok ve `o`, `\|>`, yapılar, `dene/yakala`, `ertele`, `eşzamanlı`, modüller |
| [`03_testler_test.tr`](ornekler/03_testler_test.tr) | `test` blokları, `doğrula`, `eşit_mi`; `tr test ornekler/03_testler_test.tr` ile çalışır |
| [`04_hata_ayiklama.tr`](ornekler/04_hata_ayiklama.tr) | `kesme_noktası` ile hata ayıklama, değeri çalışırken değiştirme (etkileşimli) |
| [`05_tensor_ve_turev.tr`](ornekler/05_tensor_ve_turev.tr) | Tensörler, `@`, yayınlama, otomatik türev, AdamW ile doğrusal regresyon, tensörlerle iki katmanlı XOR ağı, kaydet/yükle |
| [`06_transformer.tr`](ornekler/06_transformer.tr) | Elle yazılmış küçük transformer: çok başlı nedensel dikkat, katman normu |
| [`07_karakter_modeli.tr`](ornekler/07_karakter_modeli.tr) | Projenin belgeleriyle eğitilen karakter düzeyi dil modeli ve metin üretimi |
| [`08_veri_ile_egitim.tr`](ornekler/08_veri_ile_egitim.tr) | `tokenizer.json` parçalayıcısı ve `.npy` veri kümesiyle uçtan uca eğitim ve doğrulama |
| [`09_hata_ayiklama_alistirmasi.tr`](ornekler/09_hata_ayiklama_alistirmasi.tr) | VS Code hata ayıklayıcısıyla adım adım ilerleme: kesme noktası, tablo görünümü, değeri çalışırken değiştirme (F5) |

Kendi denemeleriniz için kökte bir `calisma/` klasörü açıp örnekleri oraya kopyalayabilirsiniz (ör. `ornekler/01_merhaba.tr`); bu klasör depoya girmez.

## Belgeler ve ölçümler

- [`docs/kilavuz/`](docs/kilavuz/README.md) — komutlar, yerleşikler, tensörler ve yapay zekâ, veri ve parçalayıcı, hata ayıklayıcı.
- [`docs/dil/`](docs/dil/README.md) — dilin kuralları; her kuralın `testler/` altında bir testi vardır (`tr test testler`).
- [`docs/kodlar.md`](docs/kodlar.md) — her anahtar kelimenin, tipin, yerleşiğin, metodun ve adlı argümanın ne işe yaradığı, imzası ve örneği (kod listesinden üretilir).
- [`docs/yol_haritasi.md`](docs/yol_haritasi.md) — yapılacak işlerin sırası: GPU, kitaplık, yazım dilleri, platformlar.
- [`bakim/kiyas/`](bakim/kiyas/README.md) — hız kıyas seti ve model eğitimi süre ölçümleri.

## Depo yapısı

| Klasör | İçerik |
| :-- | :-- |
| `cmd/tr` | `tr` komutu: çalıştırma, kabuk, `çalıştır`, `kontrol`, `derle`, `test`, `biçimle`, `hata_ayıkla`, `yardım`, `sürüm`, `dap` (VS Code için) |
| `internal/dil/kod` | Kodlar: dilin bütün adları (anahtar kelimeler, yerleşikler, metotlar, adlı argümanlar) sabit kodlara bağlı; adlar ve açıklamalar `tr.json`, dilden bağımsız tanımlar `tanim.json` ([`docs/kodlar.md`](docs/kodlar.md)) |
| `internal/dil/sozcuk` | Kaynak metni sözcüklere ayırır |
| `internal/dil/ayristir` | Sözcüklerden sözdizimi ağacı kurar |
| `internal/dil/agac` | Sözdizimi ağacı düğümleri |
| `internal/dil/denetci` | Çalıştırmadan önceki tip denetimi |
| `internal/dil/yurut` | Yorumlayıcı: derleyici, değerler, yorumlayıcı kilidi, eşzamanlı görevler |
| `internal/dil/turkce` | Mesajlarda doğru Türkçe ekler ve kod çerçevesi |
| `internal/hesap/tensor` | n boyutlu tensörler, otomatik türev ve CPU çekirdekleri |
| `internal/hesap/parca` | `tokenizer.json` parçalayıcısı (metin ↔ token kimliği) |
| `internal/hesap/veriyolu` | `.npy` token veri kümesi, örnekleyici, önbellek |
| `internal/motor` | Tensör, parçalayıcı, veri ve JSON/CSV yerleşiklerini dile bağlar |
| `internal/arac/ayikla` | Terminal hata ayıklayıcısı ve VS Code hata ayıklama sunucusu |
| `internal/arac/testkosucu` | `tr test` komutunun çekirdeği |
| `internal/arac/bicim` | `tr biçimle` biçimlendiricisi |
| `testler/` | Dil tanımının kural testleri |
| `bakim/uret/` | Geliştirme araçları: kod listesinden `docs/kodlar.md`'yi üreten `kod_belgesi`, Go'daki adlı kod sabitlerini üreten `kod_sabitleri`, eklentinin renklendirme listelerini üreten `eklenti_listeleri` |
| `duzenleyici/vscode/` | VS Code eklentisi |

## Lisans

TR, [Apache Lisansı 2.0](LICENSE) ile dağıtılır: kodu kullanabilir, değiştirebilir ve dağıtabilirsiniz; lisans metni ve değişiklik bildirimleri korunmalıdır.
