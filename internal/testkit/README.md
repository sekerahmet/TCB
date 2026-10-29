# testkit

Birden çok paketin testlerinde aynı yazılan küçük yardımcıların tek yeri: `WriteFile`/`WritePath` (dosya yazma), `MustContain` (çıktıda parça arama), `TRSeeds` (bulanıklaştırma tohumları), `GlobFiles` (depo köküne göre dosya kalıbı), `HasJSONName` (yapı etiketi denetimi), `RunBinary`/`Wait` (derlenmiş `tr` ikilisini çalıştırıp sonucunu denetleme), `BuildWithFile` (bir depo dosyası, gömülü olanlar da, değiştirilmiş olarak bir ikili derleme: bozuk bir veri dosyasının etkisini sınama; tr ve codegraph), `ScalesLinearly` (bir işin girdi boyuyla doğrusal büyüdüğünü, makine yükünden bağımsız sınama: n ve 4n aynı anda ölçülür, oran en çok 8; iş işlediği birimi ve hatasını verir, iki boy hatasız ve 3,5–4,5 kat birimle bitmelidir), `ValidLossMetrics`/`AssertSameRunFiles` (eğitim günlüğünün geçerli kayıpları ve iki koşu klasörünün bit düzeyinde eşitliği), `FirstLineDiff` (iki metnin ilk ayrılan satırı), `WaitGoroutineBlocked` (bir gorutinin kanal beklemesine girmesini uykusuz bekleme), `PanicError` (bir işlevin paniğini hata olarak alma), `RunWithLanguageFile`/`AssertSameFactSet` (yazım dilinden bağımsızlık testleri: çıktıyı `TR_LANG=tr` ve `en` ile ayrı alt süreçlerde kurup olguları karşılaştırma; codeindex ve codegraph), `GoStringLiterals` (bir Go dosyasının metin sabitleri, etiketler ve içe aktarma yolları hariç; Türkçe metin ve çıplak ad kural testlerinin ortak gezintisi). `trrun/` altpaketi TR programı çalıştıran yardımcılardır (`Parse`, `Exec`, `MustExec`, `MustExecChecked`).

## Kurallar

- Yalnızca testler (`_test.go`) içe aktarır; üretim koduna sızmaz. Denetim: `internal/layers_test.go` (`TestLayerRule`).
- `testkit` yalnızca `reporoot`'u bilir; `trrun` yalnızca dil çekirdeğini (`internal/lang/`) bilir. Bu yüzden dil çekirdeğinin kendi testleri `trrun`'u kullanamaz (içe aktarma döngüsü); motor, araç ve komut testleri kullanır. Denetim: `internal/layers_test.go` (`TestLayerRule`).
- Paketler arası gerçek farklar (tip denetimi, VM kurulumu, günlük/denetim noktası yazıcıları, motor kaydı) paketin kendi ince sarmalayıcısında kalır; ortak çekirdek burada durur.

## Testler

Birim testleri sınadıkları dosyanın adını taşır (`x_test.go` → `x.go`): `testkit_test.go`, `trrun/trrun_test.go`.
