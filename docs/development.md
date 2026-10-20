# Katkı rehberi: kod düzeni

İlke: **küçük küçük yönetmek her zaman daha kolaydır.** Bir dosya tek konuyu, bir fonksiyon tek işi anlatır; okuyan kişi aradığını adından bulur. Kuralların çoğunu `internal/layout_test.go` ve `internal/layers_test.go` denetler, yani kural çiğnenirse `go test ./...` kalır.

## Klasörler ve katmanlar

Bütün Go kodu `internal/` altındadır ve dört katmana ayrılır: dil çekirdeği (`internal/lang`), hesap çekirdekleri (`internal/compute`), motorlar (`internal/stdlib`), araçlar (`internal/devtools`, `cmd/tr`). Kim kimi içe aktarabilir, her paketin görevi ve klasör haritası: [`architecture.md`](architecture.md#katmanlar). Kuralı `internal/layers_test.go` denetler; yeni bir klasör açılınca oradaki tablo da güncellenir.

## Adlar

- **Kök klasörler: küresel standart + TR standardı.** Araçların ya da her geliştiricinin beklediği yerlerde dünyada yaygın ad kullanılır: `bin/` (derleme çıktısı, depoya girmez), `docs/`, `tools/` (bakım betikleri, üreteçler, kıyas), `editors/` (düzenleyici eklentileri), `cmd/`, `internal/`, `.github/`, `README.md`, `LICENSE`, `SECURITY.md`, `CONTRIBUTING.md`. Dilin kendi içeriğini tutan klasörler de İngilizce adlıdır (`examples/`, `tests/`, `lib/`); içlerindeki `.tr` dosyaları ve kullanıcı belgeleri Türkçedir.
- **Go kodunun kuralları** (İngilizce adlar, Türkçe yorumlar, kullanıcıya görünen adların ve iletilerin kodlara bağlanması, dosya ve fonksiyon boyu, paket belgesi, hatalar, yorumlayıcı kilidi, sınırlar, test dosyası düzeni) ve her kuralı koruyan test: [`internal/README.md`](../internal/README.md). Her kodun ne işe yaradığı ve imzası: [`codes.md`](codes.md).

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
| `Fuzz*` testleri | Dış girdi okuyan kod (dosya biçimleri, gömülü paket) | `bash tools/fuzz.sh 2s '^FuzzAd$'` |

## Yeni kod (ad) isteği

Yeni bir yerleşik, metot, anahtar kelime ya da adlı argüman yeni bir **kod** ister. Kodu yalnızca baş geliştirici verir:

1. Önce [`codes.md`](codes.md)'ye bakın: aynı işi yapan bir kod (fonksiyon ya da metot biçiminde) varsa onu kullanın. Aynı işi yapan iki kod olamaz.
2. Yoksa kodu kendiniz eklemeyin. O parçayı bırakın, işin geri kalanına devam edin ve raporunuza **sözlük isteği** başlığıyla yazın: ne işe yarayacak, imzası, neden var olan kodlar yetmiyor, baktığınız yakın kodlar. Depoya dışarıdan katkıda bulunuyorsanız aynı bilgilerle **Sözlük isteği** konu şablonunu açın ([`.github/ISSUE_TEMPLATE/name_request.md`](../.github/ISSUE_TEMPLATE/name_request.md)).
3. Baş geliştirici ya var olan bir kodu gösterir ya da kodu ve adı verir. `tr.json`'a yalnızca size verilen kodlar girer; Go sabitleri `go generate ./internal/lang/codes` ile üretilir. Kullanıcıya giden iletiler de kodludur (`H` ailesi, `internal/lang/messages`); ileti kodları adlardan farklı olarak işi yapan geliştirici tarafından kendi alanının aralığında eklenir.

## Bitti tanımı ve tam denetim

İş sırasında yalnızca değiştirdiğiniz paketlerin testlerini çalıştırın. İşin sonunda bir kez tam denetim:

```sh
bash tools/full_check.sh          # dagitim + go-esleri: durağan denetimler, ARM derlemesi, go test, tr test, örnekler
bash tools/full_check.sh --hizli  # yalnızca iş sırasında: yalnızca dagitim kipi (Go eşleri atlanır)
```

"Bitti"nin tek tanımı bu betiktir: otomatik sınama (`.github/workflows/ci.yml`) da onu çağırır, Windows ve Linux'ta iki kiple: `dagitim` (kullanıcıya giden düz `go build`; AVX2 çekirdekleri çalışma anında seçilir) ve `go-esleri` (`TR_CPU_AVX2=0`: assembly'siz Go eşleri). Linux'ta ayrıca yarış dedektörü (`--kip=yaris`: yorumlayıcı, motorlar, veri hattı, tensör, parçalayıcı, GPU benzeticisi) ve `-short` olmadan uzun testler (`--kip=uzun`) koşar; bu iki kip yerelde de çalıştırılabilir (yarış dedektörü cgo ve bir C derleyicisi ister). Go testleri `-short` ile koşar; dil tanımı testleri (`tests/`) Go testinde yinelenmez, betikte `tr test tests` ile koşar. Örnekler geçici bir klasörde çalışır, depoda iz bırakmaz; denetim sonunda `git status` başlangıçtakiyle aynı olmalıdır. Kod düzeni testi (`go test ./internal`) tam denetimin içindedir. Bulanıklaştırma testleri haftada bir uzun koşar (`tools/fuzz.sh`; yerelde tek hedef: `bash tools/fuzz.sh 2s '^FuzzNpyHeader$'`); düşüren girdiler yapıt olarak saklanır. Zayıf makinede `TAM_DENETIM_P=2` go test'in aynı anda koşturduğu paket sayısını sınırlar. Belgeler (`docs/spec/`, `docs/guide/`) işle birlikte güncellenir; her dil kuralının bir testi olur.

## Gönderim sırası: CPU ve GPU

Ana dal herkese açıktır; ona yalnızca bütün kapılardan geçmiş iş gider. Kapılar ve sıraları işin dokunduğu yere göre değişir:

| İş neye dokunuyor | Sıra |
| :-- | :-- |
| Yalnızca belge | Sonraki kod gönderimine eklenir; ayrı tam denetim koşulmaz. |
| CPU tarafı (dil, standart kitaplık, araçlar, iletiler) | İş sırasında değişen paketlerin testleri → tam denetim → ana dala gönderim → otomatik sınama (CI) izlenir. |
| GPU arka ucu ya da onu kullanan hesap yolu (`internal/compute/gpu`, tensörün GPU'ya giden yolları, GPU'da koşan `lib/*.tr`) | Değişen yerin testleri (Go paketi: `go test ./paket/...`, GPU kodu benzeticide; `lib/*.tr`: `tr test lib` ve ilgili e2e testi) → **yan dala** gönderim (`git push origin main:refs/heads/gpu-dogrulama`) → Colab'da o dalla GPU sınaması ([`tools/gpu/COLAB.md`](../tools/gpu/COLAB.md): `git clone -b gpu-dogrulama`) → geçerse tam denetim → ana dala gönderim, yan dal silinir → CI izlenir. |
| Hıza dokunan iş | Yukarıdaki sıraya ek olarak önce ve sonra ölçüm (aşağıda); GPU hızı Colab'da aynı kartta ölçülür. |

- **Neden yan dal:** Colab kodu GitHub'dan çeker. Doğrudan ana dala gönderilen bir GPU değişikliği, gerçek kartta sınanmadan herkese açık olur. Yan dal, GPU sınamasını ana daldan önceye alır.
- **Tam denetim ana dala her gönderimden önce bir kez koşar** (~5 dakika sürer; go-eşleri turu yalnızca assembly paketlerinde). Yan dal herkesin kullandığı dal değildir; ona değişen yerin testleriyle gönderilir, tam denetim Colab sınaması geçtikten sonra, ana daldan önce koşar. Birden çok dal birleşecekse hepsi birleştikten sonra tek bir denetim yeterlidir. Denetim sürerken depoda dosya düzenlenmez.
- **GPU sınaması tam denetimin yerini tutmaz, ona eklenir.** Tam denetim gerçek GPU'yu sınamaz. GPU sınaması da CPU yollarını, ARM derlemesini ve Go eşlerini sınamaz.
- Colab işi bitince çalışma zamanı kapatılır (`runtime.unassign()`).

## Kod grafiği

Bir şeyi bulmak için çok dosya okumadan önce yerel kod grafiğine sorun ([`tools/codegraph`](../tools/codegraph/README.md)): işlev çağrıları, arayüz uyumları, ileti ve ad kodlarının kullanım yerleri, kuralı koruyan testler, belge başlıkları ve bağlantıları. Dış ağ ve model yoktur; çıktı belirlidir.

```sh
go run ./tools/codegraph build                   # .codegraph/graph.json (git'e girmez; ılık önbellekle ~3 sn)
go run ./tools/codegraph query messages.New      # düğüm + komşuları
go run ./tools/codegraph who-uses H2100          # ileti/ad kodunu ya da işlevi kullananlar
go run ./tools/codegraph tests-for interp.VM.Run # sınayan testler ve koruduğu kurallar
go run ./tools/codegraph query zaman_ms         # yerleşik adı → kod → Go kaydı/uygulaması → belge başlığı
go run ./tools/codegraph calls-into tensor --in internal/stdlib # tensör işlevlerinden ≥2 tanesini çağıranlar
go run ./tools/codegraph fields Tensor.grad       # alanın okunduğu/yazıldığı yerler (tipe göre, kapanışlar işaretli)
go run ./tools/codegraph captures binaryKind --type Tensor # func literal'lerin yakaladığı değişkenler
go run ./tools/codegraph callees ml.New        # bir işlevin çağırdıkları, pakete göre
go run ./tools/codegraph who-uses H3096 --by-package # kullanıcılar pakete göre
go run ./tools/codegraph history H2100          # kodun/işlevin/dosyanın commit geçmişi (git'ten, graph.json'a yazılmaz)
go run ./tools/codegraph changed claude/dal      # dalın dokunduğu düğümler, testleri, aynı yere dokunan öbür claude/ dalları
go run ./tools/codegraph deleted                 # geçmişte olup silinmiş ileti/ad kodları
go run ./tools/codegraph path interp.VM H2100    # iki düğüm arası en kısa yol
```

Belirsiz ad çıkış kodu 3 ve aday listesi, bulunamayan ad 4 verir. `[INFERRED]` etiketli kenar tahmindir, etiketsiz kenar koddan okunmuştur. Sorgular grafik eskiyse (git HEAD'i ya da değişen `.go`/`.json`/`.md` dosyaları kurulduğundan farklıysa) önce kendisi kurar ve stderr'e `graph rebuilt (N s)` yazar; `--no-rebuild` bunu kapatır, `build`'i elle çalıştırmak gerekmez. `build` çözülemeyen `Denetim:` testlerini ve kırık belge bağlantılarını uyarı olarak yazar. Grafik `lib/`, `tools/` ve `examples/` altındaki `.tr` dosyalarını da (yapı, fonksiyon, metot, alan, sabit, test; çağrılar ve yerleşik kullanımları) TR'nin kendi ayrıştırıcısıyla okur: `query NedenselDikkat`, `who-uses dikkat`. Bir adın her kullanımı için değişiklik öncesi sorgu hâlâ [`tools/impact`](../tools/impact/main.go)'tır.

## Ölçüm disiplini

Yorumlayıcı Python'la en az başa baş olmalıdır; sıcak yola dokunan her değişiklik önce ve sonra ölçülür:

```sh
python tools/bench/run.py --only tr --repeat 5           # TR programları (tools/bench/)
go test -run XXX -bench . -count 6 ./internal/stdlib/ml/ # Go kıyasları
```

- Önce/sonra ölçümü aynı makinede, aynı ayarla (düz `go build`; AVX2 çekirdekleri çalışma anında seçilir) ve makine boşken yapılır. Başka işler çalışırken alınan ölçüm hüküm değildir: "gerileme var gibi" diye işaretlenir, kesin ölçümü baş geliştirici ya da hız uzmanı boş makinede yapar.
- Sıcak yolda ad araması ve gereksiz bellek ayırma olmaz: adlar derleme anında koda ve yuvaya çözülür, çalışma anında dizi indeksi ya da `switch` ile dağıtılır.
- Ayrıntılar ve kıyas setinin testleri: [`tools/bench/README.md`](../tools/bench/README.md).
