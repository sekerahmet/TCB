# edithook

Claude Code'un düzenleme kancası (`.claude/settings.json`, `PreToolUse`: `Edit`, `Write`,
`MultiEdit`, `NotebookEdit`). CLAUDE.md'nin kesin kuralını zorlar: bir dosyaya dokunmadan önce onu
koruyan testler ve kurallar okunur (`go run ./tools/codegraph tests-for <dosya>`).

Bir oturum depoda izlenen bir dosyayı ilk kez düzenlemek isteyince kanca düzenlemeyi bir kez
durdurur (Claude Code'un `permissionDecision: "deny"` kararı; çıkış kodu kullanılmaz, çünkü
`go run` programın 2 kodunu 1'e çevirir) ve `tests-for` cevabının ilk 60 satırını gerekçe olarak
ajana verir; ajan cevabı okuyup
düzenlemeyi yineleyince geçer. Aynı oturumda aynı dosya bir daha durmaz (işaret geçici klasörde,
`tr-edithook/<oturum>/`). Geçenler: depo dışındaki ve izlenmeyen dosyalar (yeni dosya, `scratch/`),
yolu olmayan girdi. Kancanın kendisi çalışamazsa (bozuk girdi, derleme hatası) çıkış 1'dir ve
Claude Code düzenlemeyi durdurmaz: bozuk bir ağaçta iş kilitlenmez.

Neden kanca: yazılı kural acele edilince atlanıyordu (hız analizinde nedenler ölçülmeden ve
codegraph'a sorulmadan yazıldı, 2026-10-25). Kural ajanın dikkatine bırakılmaz; bedeli dosya başına
birkaç saniye ve bir tekrar.

## Testler

`main_test.go`: ilk düzenleme bir kez durur (JSON karar) ve cevap ajana gider, sonraki geçer, başka dosya ve
başka oturum yeniden durur; izlenmeyen dosya, yolsuz girdi geçer, bozuk girdi çıkış 1; `locate`
yalnızca izlenen dosyayı verir (gerçek bir git deposunda).
