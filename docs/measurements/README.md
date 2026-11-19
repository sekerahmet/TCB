# Ölçüler

Her hız ve bellek ölçüsü burada, işine göre beş dosyadan birinde tek satırdır. Devir, status ve rapor sayıyı kopyalamaz, satırın kimliğini anar. Bir ölçü sayısını anmadan önce aynı şeyin en yeni satırını ara ve tazeliğini komutla ölç (ajan kurallarındaki "Ölçünün tazeliği" kuralı); eski sayı gerekçe olmaz, "eski: N commit, tarih, makine" diye bilgi olarak anılır. Satır silinmez: eskidiğini komut söyler.

| Dosya | Kısaltma | Ne |
| :-- | :-- | :-- |
| [`text-generation.md`](text-generation.md) | TG | dil modeli: okuma, üretim, bellek |
| [`speech-to-text.md`](speech-to-text.md) | STT | konuşmayı yazıya çevirme |
| [`text-to-speech.md`](text-to-speech.md) | TTS | yazıyı konuşmaya çevirme |
| [`assistant.md`](assistant.md) | AS | Dost'un zinciri; aşamalar zincirin kendi koşusundan, aşamanın tek başına ölçüsüne kimlikle gönderilir |
| [`tensor-and-runtime.md`](tensor-and-runtime.md) | RT | tensör çekirdeği, eğitim, yorumlayıcı ve çalışma zamanı |

## Tazelik komutu

`go run ./tools/measurefresh [kimlik ...] [--refs dosya]` (depo kökünden). Her satırın commit'inden bu yana `origin/main`'e girip satırın yollarına dokunan commit'leri sayar, depo dışı sürümleri aşağıdaki tabloyla karşılaştırır, "Yerini alan"ı söyler ve boşsa olası yerini alanı önerir. Çıktısı ve ayrıntısı [`tools/measurefresh/README.md`](../../tools/measurefresh/README.md)'de.

## Biçim

- Her dosyada makine başına bir `## <başlık>` bölümü vardır; başlık makine tablosundaki başlıktır. Bölümün altında tek tablo durur, en yeni satır üstte; bölümde tablodan başka metin olmaz.
- Bir ölçü bir satırdır: kimlik, aygıt ve tarih her satırda görünür. Tablonun başlığı aynen şudur: `| Kimlik | Tarih | Oturum | Aygıt | Commit | Yollar | Depo dışı | Ayar | Model | Koşu | Sonuç | Kaynak | Yerini alan |`.
- Boş hücre olmaz: değer yoksa nedeniyle `— (<neden>)` yazılır. Yalnız "Yerini alan" tek başına `—` olabilir.
- Biçimi `tools/measurefresh`'in testleri denetler (`TestMeasurementDocsAreWellFormed`); yerel yol ve kişi adı da yasaktır, makine donanımıyla yazılır.

| Alan | Biçim |
| :-- | :-- |
| Kimlik | `<kısaltma>-<YYYY-AA-GG>-<makine kodu>-<oturum kodu>-<sıra>`, ör. `TG-YYYY-AA-GG-L4GPU-H1-1`; günü Tarih'in günüdür, depoda tektir |
| Tarih | `YYYY-AA-GG SS:DD +0300` (Türkiye saati; Colab UTC yazar, çevrilir); taşınan satırda saat yoksa `YYYY-AA-GG (saat yazılmamış)` |
| Oturum | oturum tablosundaki kod; kimliğin oturum parçasıyla aynı |
| Aygıt | `işlemci, ` ya da `GPU, ` ile başlar, makinenin aygıtıyla uyuşur ve işlemcinin ya da kartın adını taşır: Colab aynı adlı makinede farklı işlemci verir (CPU High-RAM 2026-11-03'te bir kez AMD EPYC 7B12, bir kez Intel Xeon geldi) |
| Commit | ölçülen kodun tam karması (40 onaltılık); commit'siz yamada taban commit, yamanın md5'i Ayar'da; yalnız motordan bağımsız satırda `— (<neden>)` |
| Yollar | kümelerin adları ve ölçülen `.tr` programları, virgülle (aşağıda "Yollar") |
| Depo dışı | `<ad> <sürüm>`'ler `; ` ile: emsal, Go, sürücü, güç kipi, güç kısması; sürüm son sözcüktür (ör. `llama.cpp b12124; Go go1.27.4; sürücü 580.82.07; güç Dengeli`) |
| Ayar | iş parçacığı, derinlik, token sayısı, koşuların sırası (ör. TR, llama, llama, TR) |
| Model | `<dosya adı> <sha256'nın ilk 12'si>`, birden çok model `; ` ile; taşınan satırda `<dosya adı> sha256 yazılmamış (taşındı)`; modelsiz ölçüde `— (model yok)` |
| Koşu | koşu sayısı, ortanca ve aralık |
| Sonuç | sayılar birimleriyle; iki şeyin karşılaştırması oranla |
| Kaynak | `tools/record` kimliği ve devir; taşınan satırda ayrıca `taşındı: <dosya>@<commit>:<satır>`, taşınan Colab satırında günlüğün saat dilimi |
| Yerini alan | `—` ya da aynı şeyin (iş, model, aygıt, ölçü) yeni ölçüsünün kimliği; o satır aynı dosyadadır ve Tarih'i daha yenidir (aynı gün ve saat yeni sayılmaz; saati yazılmamış aynı günlü satırda sıra bilinmez, kabul edilir) |

## Yollar

TR'nin ölçüsü motorun kümesini (`engine`) ve en az bir `.tr` programını sayar; tek tek klasör yazılmaz. Komut programın aktardığı dosyaları özyinelemeli katar: `TR_import "tr/<ad>"` → `lib/<ad>.tr`, öbürleri aktaran dosyanın klasörüne göre.

- Program yoksa `— (program yok: <neden>)`, taşınan satırda bilinmiyorsa `— (program bilinmiyor, taşındı)` yazılır; komut o satırın yollarına bütün `lib/`'i katar.
- Yalnız emsali ölçen satır `— (motordan bağımsız)` yazar ve Depo dışı'yı doldurur.
- Kümelerin her yolu ve satırın her programı ağaçta vardır.
- Küme eklemek ya da daraltmak TCB'nin kararıdır; yeni makine ve oturum kodu tabloya ölçünün commit'inde eklenir.
- Codegraph'ı ölçen satır `codegraph`'ı, tam denetimi ölçen satır `full_check`'i `engine`'in yanında sayar: süre ölçtüğü aracın koduyla da eskir (TCB 2026-11-17 18:10).

## Yol kümeleri

| ad | yollar |
| :-- | :-- |
| engine | `internal/compute/`, `internal/lang/`, `internal/stdlib/`, `internal/detmath/`, `internal/sysmem/`, `cmd/trlang/`, `go.mod`, `go.sum` |
| codegraph | `tools/codegraph/` |
| full_check | `tools/full_check.sh`, `tools/codegraph/` |

## Makineler

| kod | başlık | aygıt |
| :-- | :-- | :-- |
| LAPTOP | Dizüstü i7-13700H | işlemci |
| L4CPU | Colab L4, işlemci | işlemci |
| L4GPU | Colab L4, GPU | GPU |
| G4CPU | Colab G4, işlemci | işlemci |
| G4GPU | Colab G4, GPU | GPU |
| HIGHRAM | Colab CPU High-RAM | işlemci |
| CPU | Colab CPU | işlemci |
| GHAWIN | GitHub Actions, Windows | işlemci |
| GHALINUX | GitHub Actions, Linux | işlemci |

Dizüstünün belleği iki 16 GB DDR5-4800 modülüdür; çift kanalsa teorik bant 76,8 GB/sn, kanal sayısı ölçülmedi (`Win32_PhysicalMemory`, 2026-11-01; eski `docs/measurements.md`'den).

GitHub Actions'ın iki makinesi (`GHAWIN`, `GHALINUX`) sabit değildir: runner görüntüsü (`windows-latest`, `ubuntu-latest`) aynı kalsa da donanım koşudan koşuya değişir. Bu satırların Aygıt'ı görüntünün adını taşır, Kaynak'ı CI koşusunun kimliğini ve commit'i yazar; kıyas yalnız aynı koşunun işleri arasında yapılır, iki koşunun süresi makine kıyası sayılmaz (TCB 2026-11-19 12:09).

## Oturumlar

| kod | oturum |
| :-- | :-- |
| H1 | H1 |
| T1 | T1 |
| D1 | D1 |
| S1 | S1 |
| C1 | C1 |
| H2 | H2 |
| MOTOR | Motor |
| DOST | Dost Interface |
| CODEGRAPH | CODEGRAPH UZMANI |
| TCB | TCB |
| DENETIM | DENETİM |
| DILUZMANI | DIL UZMANI |
| TASINDI | taşınan satır; ölçen oturum kaynakta yazılmamış |

## Güncel depo dışı sürümler

Emsallerin ve ortamın makine başına geçerli sürümü. Yeni sürümle ölçen oturum satırı ölçünün commit'inde günceller; eski sürümle ölçülmüş satırlar o andan eski olur. Go'nun sürümü tabloda değildir: `go.mod`'un `toolchain` satırından okunur.

| ad | makine | sürüm |
| :-- | :-- | :-- |
| llama.cpp | LAPTOP | b12124 |
| llama.cpp | L4CPU | b12124 |
| llama.cpp | L4GPU | b12124 |
| llama.cpp | G4CPU | b12124 |
| llama.cpp | HIGHRAM | b12124 |
| sürücü | L4GPU | 580.82.07 |
| güç | LAPTOP | Dengeli |
| Vantage | LAPTOP | Performance |
| onnxruntime | LAPTOP | 1.31.1 |
| torch | LAPTOP | 2.6.0+cpu |
| transformers | LAPTOP | 5.2.0 |
| onnxruntime | HIGHRAM | 1.31.1 |
| torch | HIGHRAM | 2.14.1+cpu |
| transformers | HIGHRAM | 5.22.1 |

## Kim yazar

- Ölçen oturum satırını ölçünün commit'inde kendi dalında yazar; aynı şeyin eski satırının "Yerini alan"ını aynı commit'te doldurur.
- Birleşmeyecek ölçüyü (commit'siz deneme, kontrol dalı) TCB'nin kararıyla DIL UZMANI yazar.

## Kurallar

- **Karşılaştırma aynı makinede ve aynı koşuda yapılır,** sırası dönüşümlü. İki ayrı makinenin mutlak sayıları karşılaştırılmaz; oran yazılır.
- **Dizüstü paylaşımlıdır:** yan oturumlar çalışırken sayılar gürültülüdür. Hız ölçüleri Colab'da koşulur; dizüstünde yalnız orada ölçülebilen şey ölçü penceresinde ölçülür.
- **Commit'siz ölçü girmez.** Ölçünün commit'i main'de ya da kontrol dalında bir commit'tir; deneme birleşmesi olmaz (DENETİM 842 N2: komut birleşme commit'lerini saymaz, deneme birleşmesinin kendi çözümü sayılmadan kalır).
- **Bir satır ne zaman eski:** commit'inden bu yana yollarından birine dokunan bir commit `origin/main`'e girdiyse ya da depo dışı sürümlerinden biri değiştiyse. Ayrı bir "Eski" listesi yoktur; komut söyler.

## Ölçü yöntemi notları

- **Colab L4'ün işlemcisinde llama.cpp'nin örnekleri iki kümeye düşebilir:** örneklerin yaklaşık üçte biri belirgin yavaş çıkar ve ortanca yavaş kümeye düşebilir; TR'de böyle bir küme görülmedi. Bu makinede llama.cpp için tekrar artırılır ya da iki küme ayrı yazılır (H1, 2026-11-17).
