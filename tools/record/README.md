# record

Ölçüm ve denetim koşularını kendiliğinden kaydeder. Komutu sarar: çalıştırır, çıktısını hem ekrana hem dosyaya yazar, koşu bitince kaydını JSON olarak yazar ve bütün koşuların dizinine bir satır ekler. Elle özet yazılmaz; zaman içindeki karşılaştırma dizinden yapılır.

```sh
go run ./tools/record --name tam-denetim -- bash tools/full_check.sh
go run ./tools/record --name zorlama --out /content/drive/MyDrive/TR_dili/olcumler -- bash tools/full_check.sh --kip=zorlama
```

- `--name`: işin adı ve klasörü (harf, rakam, `-`, `_`).
- `--out`: kayıt kökü; varsayılanı ev klasöründe `tr-olcumler`. Depo dışındadır: tam denetim depoda iz bırakan bir komutu kalır sayar. Colab'da Drive'daki `TR_dili/olcumler` verilir.
- Çıkış kodu komutunkidir; aracın kendi hatasında 2.

## Düzen

```
<out>/index.jsonl                       bütün işlerin koşuları, koşu başına bir satır (kayıt, tek satır)
<out>/<name>/20261023143005.json        kayıt
<out>/<name>/20261023143005.log         komutun çıktısı (standart çıktı ve hata)
<out>/<name>/20261023143005_<dosya>     komutun bıraktığı dosyalar (alt klasör / yerine _)
```

Zaman koşunun başladığı andır, Türkiye saatiyle (UTC+3), `YYYYAAGGSSDDSS`: adlar tarih sırasıyla dizilir ve kayıtları okuyanın saatiyle uyuşur. Her makine zamanı Türkiye saatine çevirdiği için (Colab UTC'dir) farklı makinelerden gelen kayıtlar karışmaz; Türkiye 2016'dan beri yaz saati uygulamadığından fark sabittir. Kayıttaki zamanlar `+03:00` ekiyle yazılır.

Komut, koşu sürerken dosyalarını `TR_KAYIT_KLASORU` ortam değişkeninin gösterdiği geçici klasöre (`<out>/<name>/.<zaman>/`) yazar; koşu bitince araç onları zaman önekiyle iş klasörüne taşır ve geçici klasörü siler. Komut oraya `metrics.json` bırakırsa içeriği kayda `metrics` olarak da gömülür: `tools/full_check.sh` adım sürelerini, kalan adımları ve sonucu böyle yazar.

## Kayıt

| Alan | Ne |
| :-- | :-- |
| `schema` | biçimin sürümü (alan anlamı değişince artar) |
| `name`, `command` | iş ve çalıştırılan komut |
| `started`, `ended`, `seconds`, `exit_code` | zaman (Türkiye saati, `+03:00`), süre, çıkış kodu |
| `commit` | `id`, `branch`, `dirty` (izlenen dosyalarda gönderilmemiş değişiklik), `subject` |
| `machine` | `host`, `os`, `arch`, `cpu` (işlemcinin adı), `cores`, `memory_gb` (Linux), `gpus` (`nvidia-smi`: ad, bellek, sürücü), `go` |
| `metrics` | komutun `metrics.json`'ı (varsa) |
| `files` | koşunun dosyaları, kayıt köküne göre |

Karşılaştırma ve karar: pytest-benchmark kaydına commit bilgisini (`id`, `time`, `author_time`, `dirty`, `project`, `branch`; `pytest_benchmark/utils.py` `get_commit_info`) koyar; MLflow 3 bir koşuyu parametreler, ölçümler, etiketler, dosyalar ve başlangıç/bitiş zamanı olarak kaydeder, git commit'ini kendiliğinden etiketler (`mlflow.source.git.commit`). TR ikisinin ortak çekirdeğini alır, ama sunucu ya da kitaplık istemez: kayıt düz JSON'dur, Drive'da durur, dizini satır satır okunur. Ölçümü üreten komut değişmez; yalnızca isterse `metrics.json` bırakır.

## Testler

- `main_test.go`: komutun çalıştırılması, çıkış kodu, çıktının dosyaya yazılması, `metrics.json`'ın gömülmesi, dosyaların zaman önekiyle taşınması, dizine satır eklenmesi, geçersiz girdinin reddi, `nvidia-smi` çıktısının ayrıştırılması.
