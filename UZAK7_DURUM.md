# UZAK 7 Durum Dosyası

- **Dal:** `gemini/uzak-7`
- **Taban commit (`main`):** `ffca754c3317bbb058ea3ff649c78d8c6a692362`
- **Başlama saati:** 2026-11-08 12:41:04 UTC+3 (2026-11-08 09:41:04 UTC / 01:41:04 PST)
- **Go sürümü (`GOTOOLCHAIN=local`):** `go version go1.27.3 linux/amd64`
- **Bağlam sıkıştırması:** Yok.

## Bitenler
- `CLAUDE.md` ve `yonerge` dalındaki `UZAK_YONERGE_7.md` okundu.
- `gemini/uzak-7` dalı `main` (`ffca754c3317bbb058ea3ff649c78d8c6a692362`) ucundan açıldı.
- `GOTOOLCHAIN=local` ayarlandı.

## Kalanlar
- **Adım 1 (yeniden üret):**
  - `tools/codegraph/gaps.md:45` ve `tools/codegraph/test_reach.go` okunacak.
  - Küçük fikstür (`use() { apply(leaf) }`, `apply(fn func()) { fn() }`, `leaf() {}`, `TestUse`) ile `tests-for leaf` taban çıktısı ("tests: none found") üretilecek.
  - Depoda gerçek bir yüksek dereceli Go işlevi örneği (`query` ya da `who-uses` ile) ve `tests-for sumBuiltin` (`internal/lang/interp/builtins.go:25`) taban çıktıları ölçülecek.
- **Adım 2 (düzelt):**
  - Go'da bir çağrının argümanı olarak geçen paket düzeyi işleve giden kenarın test erişim zincirine girmesi sağlanacak (bileşik değerdeki işlev değeri, değişkene atanıp sonra geçirilen işlev değeri ve yöntem değeri hariç kalacak).
  - Koruyan testler eklenecek.
- **Adım 3 (büyümeyi ölç):**
  - Seçilen 40'ta 1 dosyalarda `tests-for` test sayısı (önce ve sonra).
  - `main`'in son 10 commit'inde `changed --go-tests <commit>` test ve paket sayısı (önce ve sonra).
  - Grafın kurulma süresi (önce ve sonra, üç koşunun ortancası).
  - En çok büyüyen üç dosyanın sebebi.
- **Bitti ölçütü ve Rapor (`UZAK7_RAPORU.md`):**
  - Ön denetim komutları, kendi kontrolüm ve teslim.

## Emin Olmadıklarım
- Henüz yok.
