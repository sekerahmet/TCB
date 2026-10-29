# DURUM — Faz 0.5: görünümden geri kodlamanın kaldırılması (17.7), örneklerin tanım dosyasına taşınması (17.4)

- **Başlama:** 2026-10-29 18:57 UTC
- **Dal:** `claude/serene-pascal-rbkqwf` (taban `main` 846379d)

## Biten adımlar

- 19:40 örneklerin taşınması (`1e8e0b3`): 320 örnek betikle (`.devir/move_examples`, bugünkü `source.Encode`) `definitions.json`'a (`example`) kimlik yazımıyla taşındı; `tr.json`'da ve `en.json`'da `"example"` yok (`en.json`'da zaten yoktu), dil dosyası ayrıştırıcısı alanı reddediyor. Komut satırı örnekleri (55: cmd, sub, opt, env, repl, dbg) kimliğin adıyla (`tr check eğit.tr`, `--go-profile`, `TR_LANG=en tr help`; kabuk `exit`, hata ayıklayıcı `quit`). `TestExamplesRun` örnekleri `lexer.NewBody` ile okuyup çalıştırıyor (265 program örneği; komut satırı örnekleri program değil, önceki gibi atlanıyor). `docs/codes.md` örneği kimlik yazımıyla ve altında her gömülü dilin görünümüyle (`en:`, `tr:`) gösteriyor.
  - **Cherry-pick notu:** `1e8e0b3` betiği (`.devir/move_examples/main.go`) de taşıyor; commit'i ayırmak için geçmişi yeniden yazmak gerekiyordu, yapmadım. Alırken `.devir/` yolunu bırakın (`git cherry-pick -n 1e8e0b3 && git rm -rq --cached .devir`).

- 20:55 kodlamanın kalkması (`e56be1b`), eklenti (`44517d4`). Ayrıntı aşağıda, bitişte.
  - **Cherry-pick uyarısı:** `e56be1b` yanlışlıkla kökte derlenmiş `tr` ikilisini (16 MB, `go build ./cmd/tr` çıktısı) de ekledi; `06d9be9` onu siler. Geçmişi yeniden yazmak izin almadığı için düzeltmeyi ayrı commit yaptım. Alırken ikilinin main'in geçmişine girmemesi için: `git cherry-pick -n e56be1b && git rm -q --cached tr && rm -f tr && git commit -C e56be1b` (06d9be9 o zaman gerekmez).

## Kalan

4. Belgeler, status 17.4 ve 17.7.

## Ölçüler

- `git grep -n 'TextLexer' -- '*.go'` önce: 16 satır (10 dosya).

## Kararsız kaldıklarım

## Sorular

## Codegraph'ın yanlış/eksik cevapları
