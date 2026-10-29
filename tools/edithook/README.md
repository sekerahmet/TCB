# edithook

Claude Code'un düzenleme kancası (`.claude/settings.json`, `PostToolUse`: `Edit`, `Write`,
`MultiEdit`, `NotebookEdit`). Codegraph kuralını ajanın önüne getirir: bir dosyayı koruyan testler
ve kurallar (`go run ./tools/codegraph tests-for <dosya>`).

Bir oturum depoda izlenen bir dosyayı ilk kez düzenleyince kanca `tests-for` cevabının ilk 30
satırını ajana bağlam olarak verir (Claude Code'un `additionalContext` cevabı; çıkış kodu
kullanılmaz, çünkü `go run` programın 2 kodunu 1'e çevirir). Listenin başı tek başına yetmez (doğrudan testi olmayan
bir dosyada ilk 30 satır başka paketlerin uzak testleriydi, 2026-10-29); Go dosyasında yeten, codegraph'ın en üste
koyduğu satırlardır: koşu satırları (`run:`, koşmadıkları sayıyla) ve boyut satırı (`size (...)`). Düzenleme durmaz: 2026-10-28'e kadar
kanca ilk düzenlemeyi durdurup yinelenmesini istiyordu; bu her dosyada bir tur alıyor, kararı
değiştirmiyordu. Aynı oturumda aynı dosya için bir daha söylenmez (işaret geçici klasörde,
`tr-edithook/<oturum>/`). Kabuktan (Python, sed) yapılan düzenlemeyi görmez; bu açık yol
haritasındaki kanca gözden geçirmesinde. Geçenler: depo dışındaki ve izlenmeyen dosyalar (yeni dosya, `scratch/`),
yolu olmayan girdi. Kancanın kendisi çalışamazsa (bozuk girdi, derleme hatası) çıkış 1'dir ve
Claude Code düzenlemeyi durdurmaz: bozuk bir ağaçta iş kilitlenmez. Denetim:
`TestFirstEditShowsTheGuardingTestsOnce`, `TestUntrackedFilesAndBadInputPass`, `TestLocateTrackedFilesOnly`;
ayar dosyasının kancayı her düzenleme ve kabuk aracında çalıştırdığını `TestSettingsRunTheHookOnEveryEditAndShellTool` denetler.

Kanca codegraph'ı önce derletir (`go run -exec`: Go ikiliyi derleme önbelleğinde bırakır, kanca
yolunu `.codegraph/last-codegraph`'a yazar), sonra koşar. Araç bu ağaçla derlenmiyorsa (codegraph TR tarafı için dil paketlerini içe aktarır; düzenlenen bir dil paketi ara
hâlde derlenmeyebilir) son derlenen ikilinin cevabı verilir ve başına "codegraph does not build with
this tree … it may be stale" uyarısı derleme hatasıyla yazılır; cevapsız kalmaz. Hiç derlenmiş
ikili yoksa derleme hatası görünür. Ön denetim (`tools/full_check.sh`) codegraph'ı aynı yoldan
koşar: `go run ./tools/edithook codegraph <args>`. Denetim:
`TestABrokenTreeFallsBackToTheLastBuiltCodegraph` (sahte `go` ile).

Cevap yanlış, belirsiz ya da eksikse ajan bunu ana oturuma bildirir (komut, beklenen, gelen); ana
oturum doğrulayıp [`tools/codegraph/gaps.md`](../codegraph/gaps.md)'ye yazar.

## Uzun denetimler arka planda

Aynı araç kabuk araçlarının (`Bash`, `PowerShell`; `PreToolUse`) kancasıdır ve ikinci kuralı
zorlar: dakikalar süren denetimler (ön denetim ve tam denetim `tools/full_check.sh`, her kip; bütün
ağacın ya da bir üst klasörün `go test`'i: `./...`, `./internal/...`) ön planda çağrılınca durur,
gerekçe "arka planda koş" der. Kanca bütün komutu durdurur; ileti bunu ilk sözüyle söyler ("Nothing
in this command ran, its earlier steps included"): 2026-10-28'de aynı komuttaki bir düzeltme de
çalışmamış, ajan onu uygulanmış sanmıştı. `run_in_background` ile çağrılan, saniyelik hedefli test
(`go test ./paket -run X`), dar alt ağaç (`./internal/lang/...`, yaklaşık bir dakika) ve komutu
yalnızca anan metin (tırnak, heredoc gövdesi, PowerShell here-string'i, `grep ... tools/full_check.sh`)
geçer. Ayar dosyasındaki ön süzgeç `go run`'ı yalnızca girdi bu adayları anıyorsa çağırır: sıradan
bir kabuk komutuna eklenen süre ~0,1 sn. Kanca yan oturumlarda da çalışır: 2026-10-25'te beş yan
oturumun beşinde, dalları kancayı içermediği hâlde, ön plandaki çağrıyı durdurdu (ayar ve kod
ana çalışma ağacından okunuyor). Denetim: `TestLongCommandsRunInTheBackground`.

Yan oturumda tam denetim ve `...` desenli `go test` arka planda da durur, gerekçe "ana oturumun işi"
der: yan oturum yalnız dokunduğu paketlerin testlerini koşar, bütün takımı birleştirmede ana oturum
koşar. Yan oturum, dökümü bir çalışma ağacının klasörüne (`C--Turkish-Code-Base--claude-worktrees-<ad>`)
yazılan oturumdur; o anki klasör ölçüt değildir, çünkü ana oturum tam denetimi bir çalışma ağacında
da koşar. Neden: 2026-10-26'da iki yan oturum bütün takımı arka planda koştu, makine %99'da kaldı ve
ana oturumun tam denetimi süre sınırına takıldı. Ön denetim (`--kip=on`) istisnadır: yan oturum onu bir
maddenin sonunda arka planda koşar (2026-10-28; yan dalda yalnız o dalın değiştirdiği paketleri koşar,
`--taban` ile yalnız o maddeninkileri). Denetim: `TestSideSessionsRunOnlyTheirPackagesTests`.

Neden kanca: kural hafızaya ve belgeye defalarca yazıldı, yine ön planda koşuldu (Ahmet,
2026-10-25: "61. kez hafızaya aldın").

Neden kanca: yazılı kural acele edilince atlanıyordu (hız analizinde nedenler ölçülmeden ve
codegraph'a sorulmadan yazıldı, 2026-10-25). Kural ajanın dikkatine bırakılmaz; bedeli dosya başına
birkaç saniye ve bir tekrar.

## Testler

`main_test.go`: ilk düzenleme bir kez durur (JSON karar) ve cevap ajana gider, sonraki geçer, başka dosya ve
başka oturum yeniden durur; gösterilen ilk 30 satır gerçek codegraph cevabında koşu ve boyut satırlarını içerir (kısa
kipte atlanır); izlenmeyen dosya, yolsuz girdi geçer, bozuk girdi çıkış 1; `locate`
yalnızca izlenen dosyayı verir (gerçek bir git deposunda; Windows'un kısa yol adlarında da, git
yolu kendisi verir); kabuk araçlarında ön plandaki uzun komut durur, arka plandaki, hedefli test ve
komutu yalnızca anan metin geçer; ayar dosyası kancayı bütün düzenleme ve kabuk araçlarında çalıştırır.
`codegraph_test.go`: derlenmeyen ağaçta son derlenen codegraph'ın cevabı bayat uyarısıyla gelir (kancada
ve ön denetimin yolunda), derlenmiş ikili yokken derleme hatası görünür, geçici ikili kalmaz; test
ikilisi kendisini sahte `go` ve sahte codegraph olarak kopyalar (`TestMain`).
