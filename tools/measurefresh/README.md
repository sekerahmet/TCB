# measurefresh

Ölçüler klasörünün ([`docs/measurements/`](../../docs/measurements/README.md)) satırlarının tazeliğini söyler. Bir ölçü sayısını anmadan önce o satırın kimliğiyle koşulur; eski sayı gerekçe olmaz, "eski: N commit, tarih, makine" diye bilgi olarak anılır.

```sh
go run ./tools/measurefresh                      # bütün satırlar
go run ./tools/measurefresh TG-YYYY-AA-GG-L4GPU-H1-1 …
go run ./tools/measurefresh --refs notlar.md     # dosyadaki kimlik sözleri
```

## Ne yapar

- **Yollar:** satırın "Yollar" hücresindeki kümeleri README'nin "Yol kümeleri" tablosundan açar, ölçülen `.tr` programlarını ve onların özyinelemeli aktardıklarını katar (`TR_import "tr/<ad>"` → `lib/<ad>.tr`, öbürleri aktaran dosyanın klasörüne göre; yorumlayıcının çözümüyle aynı). Aktarımlar `origin/main`'deki metinden okunur, orada olmayan program çalışılan ağaçtan; aktarım değişirse aktaran dosya değişmiş ve sayılmıştır. Programsız ya da programı bilinmeyen satırın yollarına bütün `lib/` girer; motordan bağımsız satırın yolu yoktur. Denetim: `TestProgramImportsJoinThePaths`.
- **Commit'ler:** `git rev-list --count --no-merges <commit>..origin/main -- <yollar>`; sıfır değilse satır eskidir. Commit depoda yoksa (getirilmemiş kontrol dalı, yanlış karma) bunu söyler. Denetim: `TestFreshnessCountsCommitsOnThePaths`.
- **Depo dışı sürümler:** satırın `<ad> <sürüm>`'lerini README'nin "Güncel depo dışı sürümler" tablosundaki o makinenin sürümüyle karşılaştırır; Go'yu `origin/main`'deki `go.mod`'un `toolchain` satırıyla (yoksa `go` satırıyla). Tabloda o makine için adı olmayan sürüm karşılaştırılmaz. Denetim: `TestOutOfRepoVersionChangeMakesRowOld`.
- **Yerini alan:** doluysa satırın sonuna yazar. Boşsa aynı dosyada, aynı makine bölümünde, aynı modelle ve aynı programla ölçülmüş daha yeni bir satırı olası yerini alan olarak önerir: model dosya adıyla karşılaştırılır, sha256 yalnız iki satırda da yazılıysa; programı bilinmeyen taşınan satır her programla eşleşir (fazladan aday zararsızdır, eksik aday sessiz bayatlıktır). Denetim: `TestSuggestsAPossibleReplacement`.
- Satırlar çalışılan ağaçtan okunur; `origin/main` ne kadar tazeyse sonuç o kadar tazedir (önce `git fetch`).

## Çıktı

```
origin/main <kısa karma> (<YYYY-AA-GG>) ile kıyaslandı
<kimlik>: güncel
<kimlik>: eski: <N> commit, <YYYY-AA-GG>, <makine başlığı>
<kimlik>: eski: depo dışı <ad> <satırdaki sürüm> → <güncel sürüm>, <YYYY-AA-GG>, <makine başlığı>
<kimlik>: eski: commit bulunamadı (<commit>), <YYYY-AA-GG>, <makine başlığı>
<kimlik>: olası yerini alan: <kimlik>
<kimlik>: klasörde yok
<dosya>:<satır>: <kimlik> klasörde yok
```

Çıktının, biçim iletilerinin ve ölçü dosyalarının biçim sözcüklerinin (tablo başlıkları, `— (motordan bağımsız)` ...) metni [`texts.json`](texts.json)'dadır ve ikiliye gömülür; Go kodunda Türkçe metin sabiti yoktur.

"Yerini alan" doluysa durum satırının sonuna `; yerini alan: <kimlik>` eklenir. Çıkış kodu bulunamayan kimlikte ve `--refs`'in kırık göndermesinde 1; kullanımda, okunamayan dosyada ve git hatasında 2 (ileti standart hataya); eski satır ve olası yerini alan uyarıdır, çıkış 0. Denetim: `TestRunArguments`.

## Testler

Biçim ve gönderme testleri bu pakettedir ve CI'da koşar; yalnız çalışılan ağaca bakar, geçmişe bakmaz:

- **Biçim:** depodaki klasör biçime uyar (README'nin dört tablosu, beş dosya, her satır) ve her biçim yanlışı kendi iletisiyle bulunur. "Yerini alan" daha yeni bir satıra gider; aynı gün ve saatli satır yeni sayılmaz, saati yazılmamış aynı günlü satırda sıra bilinmez ve kabul edilir. Denetim: `TestMeasurementDocsAreWellFormed`, `TestCheckFindsEachProblem`.
- **Yerel yol ve kişi yoktur.** Yasak yerel yol ve kişi biçimleri: sürücü harfli yol (ters bölüyle, bölüyle; harfin önünde harf ya da rakam olmaz, yoksa `https://` da yakalanırdı), Git Bash'in `/c/`'si, `%USERPROFILE%`, `%LOCALAPPDATA%`, `Users\`, `Users/`, `~/.claude`, `/content/`, `/home/`, `scratchpad`, "<Ad>'ın makinesi / dizüstüsü / bilgisayarı". Git Bash deseninde `/<harf>/`'in önünde harf, rakam, `_`, `.`, `/` ya da `-` olmaz, yoksa depo yolu (`testdata/s/x`, `./s/x`) da yakalanırdı. Denetim: `TestCheckFindsEachProblem` (her biçim için bir olumlu örnek; `https://` ve iki depo yolu olumsuz örnek).
- **Göndermeler:** depodaki her `.md`'de kimlik desenine uyan söz klasördeki bir satıra gider. Not klasörü depoda değildir; onun için `--refs`. Denetim: `TestMeasurementReferencesExist`.

Tazelik komutunun kendisi geçici bir git deposunda sınanır (yukarıdaki "Denetim:" satırları).
