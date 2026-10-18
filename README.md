# TR — yapay zekâ için Türkçe programlama dili

TR, yapay zekâ modellerini **eğiten** ve **kullanan** geliştiriciler için tasarlanmış, anahtar kelimeleri Türkçe bir programlama dilidir. Tensörler, otomatik türev, iyileştiriciler, `tokenizer.json` parçalayıcıları ve büyük token veri kümeleri dilin içinde yerleşik olarak gelir; aynı dosyada web sunucusu, veritabanı ve testler de yazılır. Hata mesajları Türkçedir ve sorunu satırıyla gösterir.

Her anahtar kelimenin ASCII yazımı da vardır (`değişken` / `degisken`), böylece Türkçe klavyesi olmayan da yazabilir.

## Kurulum

Yorumlayıcıyı proje kökünde derleyin (`go` derleyicisi kurulu olmalıdır):

```bash
GOAMD64=v3 go build -o tr.exe ./cmd/tr          # bash
$env:GOAMD64="v3"; go build -o tr.exe ./cmd/tr   # PowerShell
```

`GOAMD64=v3` 2020 sonrası x86-64 işlemcileri (AVX2/FMA) hedefler ve matris çarpımını yaklaşık iki kat hızlandırır. Daha eski ya da bazı düşük güçlü işlemcilerde program açılmıyorsa `GOAMD64=v1` ile derleyin; ARM (arm64) bundan etkilenmez.

## İlk program

```tr
// merhaba.tr
değişken ad = "dünya"
yazdır("Merhaba, {ad}!")

fonksiyon kare(x) { sonuç x * x }
yazdır([1, 2, 3, 4].haritala { kare(o) })   // [1, 4, 9, 16]
```

```bash
./tr.exe merhaba.tr          # çalıştırır
./tr.exe kontrol merhaba.tr  # çalıştırmadan sözdizimi ve tip denetimi
./tr.exe                     # etkileşimli kabuk
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

`@` matris çarpımıdır; `+ - * /` eleman bazlı ve yayınlamalıdır. `relu`, `gelu`, `softmax`, `katman_normu`, `gömme`, `çapraz_entropi`, `SGD`, `gradyan_kırp`, `tensör_kaydet` / `tensör_yükle` ve fazlası için: [Kullanım Kılavuzu §4.D](KULLANIM_KILAVUZU.md#d-tensörler-ve-otomatik-türev).

## Veri ve parçalayıcı

```tr
değişken p = parçalayıcı_yükle("tokenizer.json")
yazdır(p.kodla("Merhaba dünya"), p.sözlük_boyutu)

değişken küme = veri_kümesi("veri/train")                       // .npy token parçaları
değişken ö = küme.örnekleyici(toplu: 8, bağlam: 256, tohum: 1)
değişken girdi, hedef = ö.sonraki()                              // [8, 256] tensörler
```

`parçalayıcı_yükle` yaygın `tokenizer.json` biçimini okur; `veri_kümesi` büyük derlemleri belleğe almadan parça parça okur, sonraki toplu işleri arka planda hazırlar. Uçtan uca örnek: [`ornekler/veri_ile_egitim.tr`](ornekler/veri_ile_egitim.tr); ayrıntılar: [Kılavuz §4.E](KULLANIM_KILAVUZU.md#e-veri-ve-parçalayıcı).

## VS Code eklentisi ve hata ayıklama

[`vscode_eklentisi/`](vscode_eklentisi/README.md) `.tr` dosyalarına renklendirme, kod şablonları (`tensor` yazıp `Tab`: küçük bir eğitim döngüsü), çalıştırma (▶) ve hata ayıklama getirir. **F5** ile hata ayıklarken **İzleme Masası** paneli tüm değişkenleri tek tabloda gösterir; listeler, tensörler ve veri kümeleri tablo olarak açılır, değerler çalışma anında değiştirilebilir. Terminalde hata ayıklamak için: `tr debug dosya.tr`.

Bu depoyu VS Code'da klasör olarak açtığınızda `Ctrl+Shift+B` açık dosyayı çalıştırır; görev önce yorumlayıcıyı yeniden derler (`.vscode/tasks.json`).

## Belgeler, örnekler, ölçümler

- [`KULLANIM_KILAVUZU.md`](KULLANIM_KILAVUZU.md) — komutlar, yerleşikler, yapay zekâ, web, veritabanı, hata ayıklayıcı.
- [`docs/DIL_TANIMI.md`](docs/DIL_TANIMI.md) — dilin kuralları; her kuralın `testler/` altında bir testi vardır (`tr test testler`).
- [`ornekler/`](ornekler/) — küçük karakter transformer'ı (`karakter_modeli.tr`), çok başlı dikkatli transformer (`transformer.tr`), veriyle eğitim, web arayüzü ve veritabanı örnekleri.
- [`kiyas/`](kiyas/README.md) — hız kıyas seti ve model eğitimi süre ölçümleri.

## Depo yapısı

| Klasör | İçerik |
| :-- | :-- |
| `cmd/tr` | `tr` komutu: çalıştırma, kabuk, `kontrol`, `derle`, `test`, `biçimle`, `debug`, `dap` |
| `pkg/lexer` | Kaynak metni sözcüklere ayırır |
| `pkg/parser` | Sözcüklerden sözdizimi ağacı kurar |
| `pkg/ast` | Sözdizimi ağacı düğümleri |
| `pkg/checker` | Çalıştırmadan önceki tip denetimi |
| `pkg/vm` | Yorumlayıcı: derleyici, değerler, yorumlayıcı kilidi, eşzamanlı görevler |
| `pkg/turkce` | Mesajlarda doğru Türkçe ekler ve kod çerçevesi |
| `pkg/tensor` | n boyutlu tensörler, otomatik türev ve CPU çekirdekleri |
| `pkg/parcalayici` | `tokenizer.json` parçalayıcısı (metin ↔ token kimliği) |
| `pkg/veri` | `.npy` token veri kümesi, örnekleyici, önbellek |
| `pkg/motor` | Tensör, parçalayıcı, veri, web, veritabanı ve yapay zekâ yerleşiklerini dile bağlar |
| `pkg/hataayiklama` | Terminal hata ayıklayıcısı ve VS Code hata ayıklama sunucusu |
| `pkg/testkosucu` | `tr test` komutunun çekirdeği |
| `pkg/bicim` | `tr biçimle` biçimlendiricisi |
| `testler/` | Dil tanımının kural testleri |
| `vscode_eklentisi/` | VS Code eklentisi |
