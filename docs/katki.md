# Katkı rehberi: kod düzeni

İlke: **küçük küçük yönetmek her zaman daha kolaydır.** Bir dosya tek konuyu, bir fonksiyon tek işi anlatır; okuyan kişi aradığını adından bulur. Kuralların çoğunu `internal/layout_test.go` ve `internal/layers_test.go` denetler, yani kural çiğnenirse `go test ./...` kalır.

## Klasörler ve katmanlar

Bütün Go kodu `internal/` altındadır ve dört katmana ayrılır: dil çekirdeği (`internal/lang`), hesap çekirdekleri (`internal/compute`), motorlar (`internal/stdlib`), araçlar (`internal/devtools`, `cmd/tr`). Kim kimi içe aktarabilir, her paketin görevi ve klasör haritası: [`mimari.md`](mimari.md#katmanlar). Kuralı `internal/layers_test.go` denetler; yeni bir klasör açılınca oradaki tablo da güncellenir.

## Adlar

- **Kök klasörler: küresel standart + TR standardı.** Araçların ya da her geliştiricinin beklediği yerlerde dünyada yaygın ad kullanılır: `bin/` (derleme çıktısı, depoya girmez), `docs/`, `tools/` (bakım betikleri, üreteçler, kıyas), `editors/` (düzenleyici eklentileri), `cmd/`, `internal/`, `.github/`, `README.md`, `LICENSE`, `SECURITY.md`, `CONTRIBUTING.md`. Dilin kendi içeriğini tutan klasörler de İngilizce adlıdır (`examples/`, `tests/`, `lib/`); içlerindeki `.tr` dosyaları ve kullanıcı belgeleri Türkçedir.
- **Go kodunun kuralları** (İngilizce adlar, Türkçe yorumlar, kullanıcıya görünen adların ve iletilerin kodlara bağlanması, dosya ve fonksiyon boyu, paket belgesi, hatalar, yorumlayıcı kilidi, sınırlar, test dosyası düzeni) ve her kuralı koruyan test: [`internal/README.md`](../internal/README.md). Her kodun ne işe yaradığı ve imzası: [`kodlar.md`](kodlar.md).

## Testler

Go test dosyalarının düzeni (birim testi `x_test.go`, davranış testi `behavior_<konu>_test.go`, ortak yardımcılar `helpers_test.go`) ve test yazma kuralları: [`internal/README.md`](../internal/README.md#testler). Dilin kural testleri `.tr` dosyaları olarak `tests/` altındadır (her kuralın [`dil/`](spec/README.md) tanımında bir numarası vardır).

### Testler nerede

| Yer | Neyi denetler | Nasıl koşulur |
| :-- | :-- | :-- |
| `tests/*_test.tr` | Dil tanımının kuralları; her testin adı kural numarasıyla başlar (`test "5.2 sabit: ..."`, [`dil/`](spec/README.md)) | `bin/tr test tests` (tam denetim koşar) |
| `lib/*_test.tr` | TR ile yazılmış standart kitaplık (`tr/katman`, `tr/iyilestirici`) | `bin/tr test lib` |
| `examples/` | Örneklerin hepsi çalışır: çıkış kodu 0, zaman aşımı yok, uyarısız; `*_test.tr` örnekleri ayrıca `tr test` ile | yalnızca tam denetim; tek örnek: `bin/tr examples/01_merhaba.tr` |
| Paketlerin yanında (`*_test.go`) | Go birim testleri: sözcük çözümleyici, ayrıştırıcı, denetleyici, yorumlayıcı, tensör ve türev, motorlar | `go test -short ./internal/...` ya da tek paket `go test ./internal/lang/interp -run Ad` |
| `internal/layout_test.go`, `internal/layers_test.go` | Kod düzeni (dosya ve fonksiyon boyu, adlar, paket belgesi, test dosyası düzeni) ve katman kuralı | `go test ./internal` |
| `cmd/tr/*_test.go` | `tr` komutunun beyaz kutu testleri: gömülü paket biçimi, aktar yönlendirmesi, depodaki `.tr` dosyalarının tanımsız ad denetimi | `go test ./cmd/tr` |
| `cmd/tr/e2e/` | Uçtan uca: derlenmiş `tr` ikilisi alt süreç olarak; komutlar, çıkış kodları, kesme ve kontrol noktasından devam, `derle`, kodlu dosyalar, belgelerdeki kod blokları ve bağlantılar | `go test ./cmd/tr/e2e -run Ad` |
| `internal/compute/gpu`, `internal/stdlib/gpuprobe` | GPU arka ucu ve GPU'da eğitim; kartsız makinede benzetimle koşar | `go test -short ./internal/compute/gpu`; gerçek kartta `gpu_sinama` ikilisiyle (`go test -c -o gpu_sinama ./internal/compute/gpu`, [`tools/gpu/COLAB.md`](../tools/gpu/COLAB.md)) |
| `editors/vscode/test/*_test.js` | VS Code eklentisi: çalıştırma, hata ayıklama yolları, kodlu görünüm, İzleme Masası komutları (sahte `vscode` modülüyle) | `node editors/vscode/test/<ad>_test.js`; hepsi gerçek yorumlayıcıyla: `go test ./cmd/tr/e2e -run EklentiJSTestleri` |
| `Fuzz*` testleri | Dış girdi okuyan kod (dosya biçimleri, gömülü paket) | `bash tools/bulaniklastir.sh 2s '^FuzzAd$'` |

## Yeni kod (ad) isteği

Yeni bir yerleşik, metot, anahtar kelime ya da adlı argüman yeni bir **kod** ister. Kodu yalnızca baş geliştirici verir:

1. Önce [`kodlar.md`](kodlar.md)'ye bakın: aynı işi yapan bir kod (fonksiyon ya da metot biçiminde) varsa onu kullanın. Aynı işi yapan iki kod olamaz.
2. Yoksa kodu kendiniz eklemeyin. O parçayı bırakın, işin geri kalanına devam edin ve raporunuza **sözlük isteği** başlığıyla yazın: ne işe yarayacak, imzası, neden var olan kodlar yetmiyor, baktığınız yakın kodlar. Depoya dışarıdan katkıda bulunuyorsanız aynı bilgilerle **Sözlük isteği** konu şablonunu açın ([`.github/ISSUE_TEMPLATE/sozluk_istegi.md`](../.github/ISSUE_TEMPLATE/sozluk_istegi.md)).
3. Baş geliştirici ya var olan bir kodu gösterir ya da kodu ve adı verir. `tr.json`'a yalnızca size verilen kodlar girer; Go sabitleri `go generate ./internal/lang/codes` ile üretilir. Kullanıcıya giden iletiler de kodludur (`H` ailesi, `internal/lang/messages`); ileti kodları adlardan farklı olarak işi yapan geliştirici tarafından kendi alanının aralığında eklenir.

## Bitti tanımı ve tam denetim

İş sırasında yalnızca değiştirdiğiniz paketlerin testlerini çalıştırın. İşin sonunda bir kez tam denetim:

```sh
bash tools/tam_denetim.sh          # dagitim + go-esleri: durağan denetimler, ARM derlemesi, go test, tr test, örnekler
bash tools/tam_denetim.sh --hizli  # yalnızca iş sırasında: yalnızca dagitim kipi (Go eşleri atlanır)
```

"Bitti"nin tek tanımı bu betiktir: otomatik sınama (`.github/workflows/ci.yml`) da onu çağırır, Windows ve Linux'ta iki kiple: `dagitim` (kullanıcıya giden düz `go build`; AVX2 çekirdekleri çalışma anında seçilir) ve `go-esleri` (`TR_CPU_AVX2=0`: assembly'siz Go eşleri). Linux'ta ayrıca yarış dedektörü (`--kip=yaris`: yorumlayıcı, motorlar, veri hattı, tensör, parçalayıcı, GPU benzeticisi) ve `-short` olmadan uzun testler (`--kip=uzun`) koşar; bu iki kip yerelde de çalıştırılabilir (yarış dedektörü cgo ve bir C derleyicisi ister). Go testleri `-short` ile koşar; dil tanımı testleri (`tests/`) Go testinde yinelenmez, betikte `tr test tests` ile koşar. Örnekler geçici bir klasörde çalışır, depoda iz bırakmaz; denetim sonunda `git status` başlangıçtakiyle aynı olmalıdır. Kod düzeni testi (`go test ./internal`) tam denetimin içindedir. Bulanıklaştırma testleri haftada bir uzun koşar (`tools/bulaniklastir.sh`; yerelde tek hedef: `bash tools/bulaniklastir.sh 2s '^FuzzNpyHeader$'`); düşüren girdiler yapıt olarak saklanır. Zayıf makinede `TAM_DENETIM_P=2` go test'in aynı anda koşturduğu paket sayısını sınırlar. Belgeler (`docs/spec/`, `docs/guide/`) işle birlikte güncellenir; her dil kuralının bir testi olur.

## Ölçüm disiplini

Yorumlayıcı Python'la en az başa baş olmalıdır; sıcak yola dokunan her değişiklik önce ve sonra ölçülür:

```sh
python tools/bench/kos.py --yalniz tr --tekrar 5           # TR programları (tools/bench/)
go test -run XXX -bench . -count 6 ./internal/stdlib/ml/ # Go kıyasları
```

- Önce/sonra ölçümü aynı makinede, aynı ayarla (düz `go build`; AVX2 çekirdekleri çalışma anında seçilir) ve makine boşken yapılır. Başka işler çalışırken alınan ölçüm hüküm değildir: "gerileme var gibi" diye işaretlenir, kesin ölçümü baş geliştirici ya da hız uzmanı boş makinede yapar.
- Sıcak yolda ad araması ve gereksiz bellek ayırma olmaz: adlar derleme anında koda ve yuvaya çözülür, çalışma anında dizi indeksi ya da `switch` ile dağıtılır.
- Ayrıntılar ve kıyas setinin testleri: [`tools/bench/README.md`](../tools/bench/README.md).
