# Mimari: katmanlar ve paketler

TR tek bir ikilidir (`tr`). Bütün Go kodu `internal/` altındadır; başka bir Go modülü bu paketleri içe aktaramaz. Kullanıcı Go paketi değil `.tr` dosyası yazar.

## Katmanlar

```
araçlar (internal/arac, cmd/tr) ─▶ motorlar (internal/motor) ─▶ dil çekirdeği (internal/dil)
                                        │
                                        └──────▶ hesap çekirdekleri (internal/hesap)
TR ile yazılı kitaplık (kitaplik/) ── ikiliye gömülür; Go tarafında yalnızca gömme dosyası var
```

Ok "içe aktarabilir" demektir. Kural `internal/katman_test.go` ile denetlenir (`go test ./internal`); yalnızca paketlerin kendi (test dışı) içe aktarmalarına uygulanır.

| Katman | İçe aktarabileceği iç paketler |
| :-- | :-- |
| `internal/hesap/**` | yalnızca `internal/hesap/**` (dili bilmez; tek başına sınanır ve ölçülür) |
| `internal/dil/**` | yalnızca `internal/dil/**` |
| `internal/motor/**` | `internal/dil`, `internal/hesap`, `internal/motor` |
| `internal/arac/**` | `internal/dil`, `internal/hesap`, `internal/motor`, `internal/arac` |
| `cmd/tr` | hepsi (`internal/depo` dışında) |
| `internal/depo`, `kitaplik` | hiçbiri |
| `bakim/**` | hepsi |

Yorumlayıcı (`internal/dil/yurut`) motorları ve hata ayıklayıcıyı içe aktarmaz; onlar yorumlayıcıya kendileri bağlanır: `internal/motor` paketi yüklenirken yerleşikleri `yurut.MotorEkle` ile, `internal/arac/ayikla` paketi yüklenirken koddaki `kesme_noktası` deyiminin açacağı hata ayıklayıcıyı `yurut.YeniHataAyiklayici` ile kaydeder. Tam dili çalıştıran program ikisini de içe aktarır (`cmd/tr`, `internal/arac/testkosucu`).

## Paketler

| Paket | Görevi |
| :-- | :-- |
| `cmd/tr` | `tr` komutu: alt komutları ayrıştırır (çalıştır, kabuk, `kontrol`, `derle`, `test`, `biçimle`, `hata_ayıkla`, `dap`), işi `internal/`'a bırakır |
| `internal/dil/kod` | Dilin bütün adlarını sabit kodlara bağlayan kod listesi (`tanim.json`, `tr.json`) ve ondan üretilen Go sabitleri |
| `internal/dil/turkce` | Türkçe harf işlemleri, ekler, "bunu mu demek istediniz" önerileri ve hata çerçevesi |
| `internal/dil/sozcuk` | Sözcük çözümleyici: kaynak metni sözcük birimlerine ayırır |
| `internal/dil/agac` | Sözdizimi ağacının düğümleri |
| `internal/dil/ayristir` | Ayrıştırıcı: sözcük birimlerinden sözdizimi ağacını kurar |
| `internal/dil/denetci` | Çalıştırmadan önceki tip denetimi ve uyarılar |
| `internal/dil/yurut` | Derleyici ve yorumlayıcı: değerler, çekirdek yerleşikler ve metotlar, modüller, eşzamanlı görevler, motor ve hata ayıklayıcı kancaları |
| `internal/motor` | Bütün motorları doğru sırayla yorumlayıcıya kaydeder (`import _ "tr_dili/internal/motor"`) |
| `internal/motor/ogrenme` | Tensör, matris, otomatik türev, sinir ağı katmanları, iyileştiriciler ve tensör kaydını TR yerleşikleri olarak dile bağlar |
| `internal/motor/parcalayici` | `parçalayıcı_yükle` ve parçalayıcı nesnesinin üyeleri |
| `internal/motor/veri` | `veri_kümesi`, örnekleyici ve sıralı okuyucu yerleşikleri |
| `internal/motor/veribicim` | JSON ve CSV yerleşikleri |
| `internal/motor/ortak/baglama` | Motorların ortak yardımcıları: adlı argüman denetimi, göreli yol, panikten hataya, Türkçe sayı yazımı |
| `internal/motor/ortak/jsonyaz` | TR değerlerini JSON'a yazar |
| `internal/hesap/tensor` | n boyutlu tensör, görünümler, otomatik türev, arka uç arayüzü ve CPU arka ucu (AVX2/FMA çekirdekleri) |
| `internal/hesap/parca` | `tokenizer.json` parçalayıcıları: BPE, WordPiece, Unigram, normalleştirme, ön bölme |
| `internal/hesap/veriyolu` | `.npy` veri dosyaları, veri kümesi, örnekleyici, önbellek ve önden yükleme |
| `internal/arac/ayikla` | Terminal hata ayıklayıcısı ve VS Code hata ayıklama sunucusu (`tr dap`) |
| `internal/arac/bicim` | `tr biçimle` biçimlendiricisi |
| `internal/arac/testkosucu` | `tr test` komutunun çekirdeği |
| `internal/depo` | Testlerin depo kökünü bulan yardımcı (testler `../../` yazmaz) |
| `kitaplik` | TR ile yazılan standart kitaplığı ikiliye gömer (bugün boş) |
| `bakim/uret/*` | Üreteçler: Go kod sabitleri, `docs/kodlar.md`, eklentinin renklendirme listeleri (`go generate ./internal/dil/kod`) |
| `bakim/kiyas/go` | Hız kıyas setinin derlenmiş başvuru sürümü |

### Yorumlayıcı kancaları

Motorlar ve standart kitaplık yorumlayıcıya (`internal/dil/yurut`) şu kancalarla bağlanır; yorumlayıcı onları bilmez:

| Kanca | Görevi |
| :-- | :-- |
| `Yinelenebilir`, `Akis` | `döngü x içinde ...` ve liste üreteci: uzunluğu bilinen kaynak (tensör) ya da tembel akış (sıralı okuyucu) |
| `YolDegerli`, `YolArgumani` | Yol alan yerleşiklerin ortak okuyucusu (Metin ya da ileride `Yol`; sekme/satır sonu ipucu, 13.1a) |
| `CiktiYaz`, `CanliSatir` | İnsanın göreceği çıktı noktaları: etkin canlı satır (ilerleme çubuğu) silinip yeniden çizilir |
| `GizliDegerEkle`, `Maskele` | Gizli değer süzgeci: kayıtlı değerler çıktı noktalarında `***` olur (veri çıktılarında değil) |
| `ArgumanlariAyarla`, `Argumanlar` | Betiğin komut satırı argümanları (okunmazsa çıkışta uyarı) |
| `Kes`, `ErrKesildi`, `KesilmeMi` | Nazik çıkış: Ctrl+C sonrası yığın çözülür, `ertele`ler çalışır; motorların panik kurtarıcıları `KesilmeMi` ile kesilmeyi yeniden atar |

## Klasörler

| Klasör | İçerik |
| :-- | :-- |
| `cmd/tr/` | Tek ikili |
| `internal/` | Bütün Go kodu (yukarıdaki katmanlar) |
| `kitaplik/` | TR ile yazılı standart kitaplık (ikiliye gömülür) |
| `duzenleyici/vscode/` | VS Code eklentisi (kullanıcıya gider) |
| `docs/` | Belgeler: [`kilavuz/`](kilavuz/README.md), [`dil/`](dil/README.md), [`kodlar.md`](kodlar.md) (üretilir), [`yol_haritasi.md`](yol_haritasi.md), [`katki.md`](katki.md) (kod düzeni), bu dosya |
| `ornekler/` | Öğrenme sırasıyla örnek programlar |
| `testler/` | Dil tanımının kural testleri (`tr test testler`); `olumsuz/`, `alt/`, `veri/` |
| `bakim/` | Depo bakım araçları, kullanıcıya gitmez: `tam_denetim.sh` ("bitti" tanımı), `uret/` (üreteçler), `kiyas/` (hız kıyas seti) |
| `ikili/` | Derleme çıktısı (`go build -o ikili/tr.exe ./cmd/tr`); depoya girmez |
| `calisma/` | Kişisel deneme klasörü; depoya girmez |

## Sahiplik

| Rol | Alan |
| :-- | :-- |
| Dil çekirdeği | `internal/dil/**`, `internal/arac/{bicim,testkosucu,ayikla}`, `internal/depo`, `cmd/tr`, `docs/dil/`, bu dosya |
| Tensör ve GPU | `internal/hesap/tensor` (ileride `internal/hesap/gpu`), `internal/motor/ogrenme`, ileride `kitaplik/cekirdek` |
| Veri ve parçalayıcı | `internal/hesap/{parca,veriyolu}`, `internal/motor/{parcalayici,veri,veribicim}` |
| Araçlar | `duzenleyici/`, `.vscode/`, `docs/kilavuz/`, `README.md` |
| Hız | `bakim/kiyas/` |
| Test | `testler/`, `.github/`, eklenti testleri |
