# TR standart kitaplığı

TR ile yazılan kitaplığın yeri. Dosyalar `tr` ikilisine gömülecek (`gomulu.go`); bugün kitaplık boştur ve `tr` ikilisine bağlı değildir (hiçbir program `kitaplik` paketini içe aktarmaz).

Planlanan düzen (her öge tek dosya, testi yanında: `katman/dikkat.tr` ve `katman/dikkat_test.tr`):

| Klasör | İçerik |
| :-- | :-- |
| `katman/` | Doğrusal, Gömme, RMSNormu, Dikkat, SwiGLU, Blok |
| `model/` | Hazır modeller (ör. `gpt.tr`) |
| `egitim/` | Hazır eğitim döngüsü, ilerleme, günlük |
| `cekirdek/` | TR ile yazılmış CPU/GPU çekirdekleri (ileride) |

Testler `tr test kitaplik` ile koşar. Katmanların ve klasörlerin yeri için bkz. [`docs/mimari.md`](../docs/mimari.md).
