# DURUM — Faz 0.5: görünümden geri kodlamanın kaldırılması (17.7), örneklerin tanım dosyasına taşınması (17.4)

- **Başlama:** 2026-10-29 18:57 UTC
- **Dal:** `claude/serene-pascal-rbkqwf` (taban `main` 846379d)

## Biten adımlar

- 19:40 örneklerin taşınması (`1e8e0b3`): 320 örnek betikle (`.devir/move_examples`, bugünkü `source.Encode`) `definitions.json`'a (`example`) kimlik yazımıyla taşındı; `tr.json`'da ve `en.json`'da `"example"` yok (`en.json`'da zaten yoktu), dil dosyası ayrıştırıcısı alanı reddediyor. Komut satırı örnekleri (55: cmd, sub, opt, env, repl, dbg) kimliğin adıyla (`tr check eğit.tr`, `--go-profile`, `TR_LANG=en tr help`; kabuk `exit`, hata ayıklayıcı `quit`). `TestExamplesRun` örnekleri `lexer.NewBody` ile okuyup çalıştırıyor (265 program örneği; komut satırı örnekleri program değil, önceki gibi atlanıyor). `docs/codes.md` örneği kimlik yazımıyla ve altında her gömülü dilin görünümüyle (`en:`, `tr:`) gösteriyor.
  - **Cherry-pick notu:** `1e8e0b3` betiği (`.devir/move_examples/main.go`) de taşıyor; commit'i ayırmak için geçmişi yeniden yazmak gerekiyordu, yapmadım. Alırken `.devir/` yolunu bırakın (`git cherry-pick -n 1e8e0b3 && git rm -rq --cached .devir`).

## Kalan

2. Kodlamanın kalkması (`tr kodla`, `source.Encode*`, `TextLexer*`, kabuk, hata ayıklayıcı, görünüm iletisi).
3. VS Code eklentisi: görünüm salt okunur.
4. Belgeler, status 17.4 ve 17.7.

## Ölçüler

- `git grep -n 'TextLexer' -- '*.go'` önce: 16 satır (10 dosya).

## Kararsız kaldıklarım

## Sorular

## Codegraph'ın yanlış/eksik cevapları
