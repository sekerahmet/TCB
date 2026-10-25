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
Claude Code düzenlemeyi durdurmaz: bozuk bir ağaçta iş kilitlenmez. Denetim:
`TestFirstEditIsHeldOnceThenPasses`, `TestUntrackedFilesAndBadInputPass`, `TestLocateTrackedFilesOnly`;
ayar dosyasının kancayı her düzenleme ve kabuk aracında çalıştırdığını `TestSettingsRunTheHookOnEveryEditAndShellTool` denetler.

Cevap yanlış, belirsiz ya da eksikse ajan bunu ana oturuma bildirir (komut, beklenen, gelen); ana
oturum doğrulayıp [`tools/codegraph/gaps.md`](../codegraph/gaps.md)'ye yazar.

## Uzun denetimler arka planda

Aynı araç kabuk araçlarının (`Bash`, `PowerShell`) kancasıdır ve CLAUDE.md'nin ikinci kesin
kuralını zorlar: dakikalar süren denetimler (ön denetim ve tam denetim `tools/full_check.sh`, her
kip; `./...` desenli `go test`) ön planda çağrılınca durur, gerekçe "arka planda koş" der;
`run_in_background` ile çağrılan, saniyelik hedefli test (`go test ./paket -run X`) ve komutu
yalnızca anan metin (tırnak, heredoc gövdesi, PowerShell here-string'i, `grep ... tools/full_check.sh`)
geçer. Ayar dosyasındaki ön süzgeç `go run`'ı yalnızca girdi bu adayları anıyorsa çağırır: sıradan
bir kabuk komutuna eklenen süre ~0,1 sn. Ayar izlendiği için kanca yan oturumların çalışma
ağaçlarında da çalışır (dalları ana dalı içerdiğinde). Denetim: `TestLongCommandsRunInTheBackground`.

Neden kanca: kural hafızaya ve belgeye defalarca yazıldı, yine ön planda koşuldu (Ahmet,
2026-10-25: "61. kez hafızaya aldın").

Neden kanca: yazılı kural acele edilince atlanıyordu (hız analizinde nedenler ölçülmeden ve
codegraph'a sorulmadan yazıldı, 2026-10-25). Kural ajanın dikkatine bırakılmaz; bedeli dosya başına
birkaç saniye ve bir tekrar.

## Testler

`main_test.go`: ilk düzenleme bir kez durur (JSON karar) ve cevap ajana gider, sonraki geçer, başka dosya ve
başka oturum yeniden durur; izlenmeyen dosya, yolsuz girdi geçer, bozuk girdi çıkış 1; `locate`
yalnızca izlenen dosyayı verir (gerçek bir git deposunda; Windows'un kısa yol adlarında da, git
yolu kendisi verir); kabuk araçlarında ön plandaki uzun komut durur, arka plandaki, hedefli test ve
komutu yalnızca anan metin geçer; ayar dosyası kancayı bütün düzenleme ve kabuk araçlarında çalıştırır.
