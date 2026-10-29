# DURUM — TestBuildScalesLinearly: gürültü mü, gerileme mi?

- Başlama: 2026-10-29 17:29 UTC
- Bitiş: —
- Dal: `claude/keen-volta-zqw48d` (taban `main` 6cd1e49)
- Makine: Linux konteyner, 4 çekirdek, go1.26.10

## Adımlar
1. [ ] Ölçüm: 1073aeab, 48bd054d, main × 15 koşu
2. [ ] Karar
3. [ ] Düzeltme
4. [ ] Karesel sınama (internal/testkit)
5. [ ] Değişiklikten sonra main'de 15 koşu

## Notlar
- `1073aeab..main` arasında `internal/devtools/codeindex` ve `internal/testkit`'e commit yok: test ve ölçüm yöntemi üç commit'te aynı, yalnız ölçülen kod (sözcük çözümleyici, ayrıştırıcı) değişiyor. [Kesin: `git log 1073aeab..main -- internal/devtools/codeindex internal/testkit` boş]
- Codegraph `run:` satırları dokunmadan önce koşuldu: hepsi geçti.
