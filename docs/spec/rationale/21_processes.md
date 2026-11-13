# 21. Dış programlar: emsal ve gerekçe

Kurallar: [`21_dis_programlar.md`](../21_dis_programlar.md). Bu dosya her kuralın emsalini (başka dillerde nasıl olduğu, neyi almadığımız, kaynaklarıyla) ve gerekçesini tutar. Dili kullanmak için kuralların kendisi yeter; bu dosya kararları tartmak ve denetlemek içindir.

Araştırma: Gemini Jarvis A (2026-11-02, taban `b2892d87`). Raporu DENETİM doğruladı (istek 36). Kaynak etiketleri:
- **[DENETİM açtı]:** sayfayı DENETİM açtı ve alıntıyı sınadı.
- **[rapordan]:** sayfayı yalnız Gemini açtı; DENETİM'in ölçtüğü örneklemde yerel alıntıların 38'i de tuttu, ama bu dış sayfa yeniden açılmadı.

Karşılaştırma:

| Konu | Python `subprocess` | Go `os/exec` | Rust `std::process` | Başka | TR |
| :-- | :-- | :-- | :-- | :-- | :-- |
| Yazım | liste ya da metin; `shell=True` var | `exec.Command(ad, arg...)`, kabuk yok | `Command::new(p).args(...)`, kabuk yok | Deno `Deno.Command` kabuksuz; Node `spawn` kabuksuz, `exec` kabuklu | ad ve liste, kabuk yok (21.1) |
| Çıkış kodu | `run`'da `check=False` varsayılan | sıfır olmayan kod `*ExitError` | `ExitStatus` döner, `.success()` ayrıca sorulur | Deno `output()` kodu döner, fırlatmaz | kabul edilmeyen kod hatadır (21.4) |
| Toplu ve akışlı | `run` ve `Popen` | `Output` ve `Start` | `output` ve `spawn` | Deno `output()` ve `spawn()` | `TR_run_process` ve `TR_spawn_process` |
| Durdurma | `timeout=`: öldürür ve bekler | `CommandContext` öldürür; `WaitDelay` | `Child` düşünce süreç yaşar | Deno `AbortSignal` | nazik istek, 2 sn, öldürme; süreç grubu (21.5) |
| Ortam | devralır; `env=` hepsini değiştirir | devralır; `Env` hepsini değiştirir | devralır; `.env` ekler | Deno devralır, `env` ekler | gizliler hariç devralır, `TR_env:` ekler (21.6) |
| `PATH` ve `.bat` | sisteme bağlı | Go 1.19'dan beri göreli `PATH` sonucu yok | 1.77.2: kaçırılamayan `.bat` argümanında `InvalidInput` | Node: `.bat` ve `.cmd` `EINVAL` | göreli `PATH` yok, `.bat` ve `.cmd` yok (21.3) |

### 21.1

- *Emsal:* Go `os/exec` "intentionally does not invoke the system shell" [DENETİM açtı: pkg.go.dev/os/exec, 2026-11-02]. Rust'ın `Command`'ı ve Deno'nun `Deno.Command`'ı da programı ve argümanları ayrı alır [rapordan: doc.rust-lang.org/std/process/struct.Command.html; docs.deno.com/api/deno/~/Deno.Command]. Python'un `shell=True`'su ve Node'un `exec`'i kabukla çalışır [rapordan]. *Kabuk seçeneğini almadık.* Boş aygıt varsayılanı Go'dan: "If Stdin is nil, the process reads from the null device" [DENETİM açtı: pkg.go.dev/os/exec].
- *Gerekçe:* ajan `"ffmpeg -i " + yol` yazdığında boşluklu bir dosya adı komutu böler, `;` içeren bir ad yeni bir komut başlatır. Kabuk yoksa argüman her zaman tek argümandır, enjeksiyon ve kırılma yazımdan kalkar. Program ile argümanların iki ayrı parametre olması, Go ve Rust'taki gibi, adın argümanla karışmasını önler. Yol argümanı `TR_String(yol)` ile `/` ayırıcılı gider (18.1); `ffmpeg` bunu her sistemde kabul eder, `cmd`'nin iç komutları gibi bazı araçlar Windows'ta etmeyebilir [hipotez]. Bu, yolun her sistemde tek yazımının bilinçli bedelidir. Varsayılan girdinin boş aygıt olması, girdiden okuyan bir programın (`-nostdin`'i unutulmuş `ffmpeg`) sonsuza dek beklemesini kökten kaldırır. Bütün çıktının belleğe alınması tek sonuçlu kısa işler içindir; akış 21.2'dedir.
- *Torunun borusu (Tur 6C, 2026-11-10):* Go'nun `WaitDelay`'i aynı iki durumu sınırlar: "a child process that exits but leaves its I/O pipes unclosed"; süre dolunca başarılı çıkışta `ErrWaitDelay` döner [kaynak: pkg.go.dev/os/exec, Gemini Uzak 12; DENETİM 402 alıntıyı açtı]. *`ErrWaitDelay`'i almadık:* arka planda süreç bırakan olağan bir araç hata sayılırdı. Beklememek de olmaz: boruyu miras alan bir torun çağrıyı sonsuza dek asar, sunucuyu ya da ajanı kilitler. Bu yüzden 2 saniye beklenir, okunan çıktı verilir, sonuç programın çıkış kodudur; torun öldürülmez, çünkü programın bilerek bıraktığı bir süreç olabilir.

### 21.2

- *Emsal:* Python `run`/`Popen`, Go `Output`/`Start`, Rust `output`/`spawn` aynı ayrımı yapar [rapordan]. Tam `n` bayt okumak Go'nun `io.ReadFull`'u ve Rust'ın `read_exact`'idir [rapordan]. Rust'ta `Child` düşünce süreç yaşamaya devam eder [rapordan: doc.rust-lang.org/std/process/struct.Child.html]. *Bunu almadık.*
- *Gerekçe:* sonucun tipi argümanın değerinden çıkamaz (13.10): toplu çağrı bayt, akışlı çağrı süreç verir; iki ayrı ad gerekir. Tam `n` bayt okumak, okunan parçanın her zaman bütün örneklerden oluşmasını sağlar: yarım örnek 22.1'de hata olur, akışın bir bayt kaymasıyla sesin gürültüye dönmesi görünür olur. Gecikmenin `n` olduğu kuralda yazılıdır, çünkü Jarvis'in ses etkinliği algılaması küçük çerçevelerle çalışır [hipotez: Silero VAD 16 kHz'de 512 örnek; belge açılmadı]. Program biterken süreçlerin durdurulması, mikrofonu ya da hoparlörü tutan yetim bir sürecin kalmamasıdır. Beklenmemiş sürecin hatasının yazılması 10.6'nın "görevin hatası kaybolmaz" kuralıyla aynı gerekçededir.
- *Tampon (DENETİM 233):* kilit dışı okuyucu her zaman vardır, argüman yalnız dolunca ne olacağını seçer. Boru Windows'ta 4096, Linux'ta 65 536 bayttır; aynı programın payı sistemden sisteme 16 kat değişmesin diye iç tampon vardır. Varsayılan geri basınçtır: dört emsalde de öyle. Taşmada süreç sürdürülmez: atılan baytların sınırı örnek sınırına denk gelmeyebilir, bir bayt kayma sesi gürültüye çevirir; eski veriyi önce vermek de bayat sesi canlı diye işletir. Node `maxBuffer` aşılınca süreci sonlandırır: doğrudan emsal.

| | Varsayılan | Sınır ve dolma | Kaynak |
| :-- | :-- | :-- | :-- |
| Go | OS borusu; okunmazsa program bekler | `WaitDelay` boruyu kapatır; okuma `Wait`'ten önce bitmeli | [ölçüldü: 4096; kanıtlı: `exec.go:1078`, `:283-302`, go1.26.10] |
| Python | `stdout=PIPE`; "will deadlock" uyarısı | `communicate` sınırsız: "do not use ... large or unlimited"; `pipesize` yalnız Linux | [kanıtlı: docs.python.org, 3.15.2] |
| Rust | `Stdio::piped`; "may end up waiting" | `wait` stdin'i kapatır (`TR_wait` gibi); `wait_with_output` sınırsız | [kanıtlı: doc.rust-lang.org, 1.99.1] |
| Node | "the subprocess blocks"; akış `highWaterMark`'ta durur | `exec`, `spawnSync` `maxBuffer` 1 MiB: "the child process is terminated" | [kanıtlı: nodejs.org, v26.13.0] |
| `ffmpeg` dshow | iç `rtbufsize` 3 041 280 bayt, dolunca çerçeve atar, sürer | `AV_LOG_ERROR`, %62'den başlayarak; çıkış kodu değişmez [hipotez: kod akışı] | [kanıtlı: `dshow.c`, `options_table.h`, n8.1.3] |
| `arecord` | taşmayı stderr'e yazar, sürer | `--fatal-errors` çıkar; hız uyarısı `!quiet_mode` altında | [kanıtlı: `aplay.c`, v1.2.16] |
| `sox` | `--buffer` 8192 | sürücüde taşma ve `waveaudio`: bulamadım | [kanıtlı: Debian man, 14.4.2+git; Sınırlar] |

### 21.3

- *Emsal:* Go 1.19: "Command and LookPath no longer allow results from a PATH search to be found relative to the current directory" [DENETİM açtı: go.dev/doc/go1.19, "PATH lookups"]. Rust 1.77.2 (CVE-2024-24576) güvenle kaçıramadığı `.bat` argümanında `InvalidInput` verir [DENETİM açtı: blog.rust-lang.org/2024/04/09/cve-2024-24576.html]. Node (CVE-2024-27980) `shell: false` iken `.bat` ve `.cmd`'yi reddeder [rapordan: nodejs.org/en/blog/vulnerability/april-2024-security-releases-2]. *Rust'ın ince kuralı yerine Node'un tam yasağını aldık.*
- *Gerekçe:* indirilen bir veri kümesinin klasöründeki `ffmpeg.exe` adlı bir dosya, `PATH`'te `.` ya da göreli bir girdi varken asıl programın yerine çalışır. `.bat` ve `.cmd`'nin argümanları `cmd.exe`'nin kurallarıyla yeniden ayrıştırılır ve güvenle kaçırılamaz. Tam yasak, ilk sürüm için en küçük ve doğrulanabilir kuraldır. Bedeli: `npm.cmd` gibi sarmalayıcılar ancak `cmd /c` ile çalışır, ve bu kodda görünür. Adda uzantının ve yol ayırıcısının yasak olması, ajanın tek bir işletim sistemine bağlı kod yazmasını engeller; dosyanın kendisi `TR_Path` ile, ayrı bir tiple verilir.
- *`TR_env:`'deki `PATH` (Tur 6C, 2026-11-10):* Rust programı açıkça verilen `PATH`'te arar: "If PATH is explicitly set via env, that new value is searched" (Unix) [kaynak: doc.rust-lang.org/std/process/struct.Command.html, 1.99.1; DENETİM 402 alıntıyı açtı]. Python POSIX'te `env`'in `PATH`'ine bakar, Windows'ta bakmaz [kaynak: docs.python.org/3/library/subprocess.html, 3.15.2]. Go'nun `exec.Command`'ı adı çağrı anında sürecin kendi `PATH`'inde arar [görüş: kaynak koddan çıkarım, belge söylemiyor; DENETİM 402 Ş4]. *Rust'ı aldık, iki sistemde de.* Gerekçe: sanal ortamın ya da bir araç klasörünün `PATH`'ini `TR_env:` ile veren ajan oradaki programın çalıştığını sanır; TR'nin kendi `PATH`'inde aramak sistemdeki başka bir programı sessizce çalıştırır. Göreli ve boş girdi yasağı `TR_env:`'den gelen `PATH`'e de uyar: veri dosyasından okunan bir ortam çalışma klasörünü aratamaz. Boş `PATH` hiçbir yerde aramaz; Rust'ın Windows'ta ana sürecin `PATH`'ine dönmesini almadık, çünkü verilen değerin yerine sessizce başka bir değer kullanılmış olurdu.

### 21.4

- *Emsal:* Go'da sıfır olmayan çıkış `*ExitError`'dur ve toplu çağrı standart hata çıktısını hataya koyar [DENETİM açtı: pkg.go.dev/os/exec]. Python'un `check=False` varsayılanı Pylint'te `W1510` uyarısıdır, yani unutulduğu bilinen bir hatadır [rapordan: pylint.readthedocs.io, subprocess-run-check]. Rust ve Deno kodu döndürür, hata vermez [rapordan].
- *Gerekçe:* başarısız bir `ffmpeg`'in boş çıktısıyla sessizce sürmek sessiz bir yanlıştır; 10.6'daki gibi hata kaybolmaz. Başka kodu başarı sayan programlar için kabul edilen kodlar açıkça yazılır, ajan neyi kabul ettiğini görür. Boş liste hiçbir bitişi kabul etmez; programı çalıştırıp her durumda hata vermek yan etkileri boşuna doğurur, bu yüzden program başlatılmadan hatadır (13.7'nin ters sınırı gibi; Tur 6C). Kodun kendisi `TR_wait`'in değeridir, çünkü hatanın değeri çıkış kodunu taşımaz (`TR_error_code` iletinin kodudur, 8.5). İptalle durdurulan programın hatası iptal hatasıdır: çıkış kodu TR'nin kendi isteğinin sonucudur, programın hatası değildir. Standart hata çıktısının arka planda okunması, dolu bir boru yüzünden iki tarafın birbirini beklemesini önler [rapordan: Python `communicate`, Rust `wait_with_output`].
- *Tur 7 (2026-11-13, başarıda standart hata):* sonucunu standart hataya yazan program (ffmpeg'in aygıt listesi) başarıda boş çıktı verir; ajan bunu "liste boş" diye okurdu (tek tık F1 devri, DENETİM 552 N3). `TR_merge_stderr:` kabuğun `2>&1`'idir; Python'un `stderr=STDOUT`'u, Go'nun `CombinedOutput`'u aynı işi görür. Birleşik kipte ayrı standart hata yoktur, çıkış hatasının iletisi birleşik çıktının sonunu taşır: tek boruda iki çıktı ayrılmaz, ileti kullanıcının `2>&1`'de göreceğini gösterir (DENETİM 646 Ş3). Dönüşü iki çıktılı bir değer yapmak reddedildi: dönüş tipini değiştirirdi ve iki boru ayrı okunurken satırların sırası kaybolurdu.

### 21.5

- *Emsal:* Go'da `CommandContext`'in varsayılan `Cancel`'ı doğrudan `Kill`dir; iki adım ancak `Cancel`'a istek gönderen bir fonksiyon verilince olur, `WaitDelay` dolunca `Kill` gelir [DENETİM açtı: pkg.go.dev/os/exec, "By default, CommandContext sets Cancel to call the Kill method"]. *İki adımı varsayılan yaptık.* Windows'ta iş nesnesinin `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE`'u süreç ağacını birlikte kapatır [rapordan: learn.microsoft.com, JOBOBJECT_EXTENDED_LIMIT_INFORMATION].
- *Gerekçe:* söz kesmede (Jarvis'te kullanıcı konuşunca cevabın kesilmesi) konuşma görevinin iptali `aplay`'i de hemen durdurmalıdır. Hemen öldürmek dosya yazan bir `ffmpeg`'e yarım dosya bıraktırır [hipotez], bu yüzden önce nazik istek gider. 2 saniye bir seçimdir; ölçülmüş bir emsali yoktur, uygulamada ölçülüp düzeltilebilir. Süreç grubu olmadan yalnız doğrudan çocuk ölür, torunları boruyu açık tutar ve bekleme bitmez [rapordan]. Program nazik istekten sonra çıkınca grup hemen öldürülür (Tur 6C): torunların hepsinin çıkışını taşınabilir biçimde beklemenin yolu yoktur, beklemek söz kesmeyi geciktirir; Go'nun `WaitDelay`'i de çocuğun çıkışını görünce kalan süreçleri ve boruları kapatır [kaynak: pkg.go.dev/os/exec, Gemini Uzak 12]. Ctrl+C'nin önce TR'ye gelmesi 1.5'in "ilk istek programı nazikçe durdurur" kuralını dış programla da aynı tutar [hipotez: Windows'ta `CREATE_NEW_PROCESS_GROUP` ile; Win32 belgesi açılmadı]. Engellenmiş okumanın hemen dönmesi gerekir, çünkü iptalin ulaşmadığı bir bekleme sessizce uzar (DENETİM istek 35, Ş1).

### 21.6

- *Emsal:* Python, Go, Rust ve Deno bütün ortamı varsayılan olarak devralır [rapordan; Go cümlesini DENETİM açtı]. Codex'in `shell_environment_policy`'si gizli adlı değişkenleri süzebilir, ama varsayılanda süzmez [DENETİM açtı: config-reference]. *Varsayılanda süzmek emsalsizdir.* Python'da `env=` verilince bütün ortam silinir [rapordan]; *bunu almadık, `TR_env:` ekler.*
- *Gerekçe:* ölçütlerin sırasında güvenlik hızdan ve alışkanlıktan önce gelir. Bir ajanın çalıştırdığı araç (ya da o aracın hata çıktısı) uzak modelin anahtarını (`OPENAI_API_KEY`) başka bir yere taşıyabilir; gizli adlar 19.4'te zaten tanımlıdır. Bedeli: anahtarla çalışan bir araç (`huggingface-cli`, `gh`) anahtarsız kalıp yetki hatası verir. Hata sessiz değildir, ve ileti süzülen adı ve `TR_env:` yazımını söyleyerek onu çalışan yazıma çevirir. Python'un "verilen ortam hepsinin yerine geçer" kuralı `PATH`'i ve Windows'ta `SystemRoot`'u sessizce siler [rapordan]; ekleme bu yanlışı kaldırır.

### 21.7

- *Emsal:* Go ve Rust çıktıyı bayt olarak verir; Python `text=True` ile çözer [rapordan].
- *Gerekçe:* metin ile bayt kendiliğinden dönüşmez (3.3a). Windows'ta konsol araçları başka bir kod sayfasıyla yazabilir [hipotez: Türkçe için cp857]. O zaman `TR_String(b)` hata verir, sessizce bozuk harf üretmez.

### 21.8

- *Emsal:* rapor bu kuralları önerdi (Soru 8); DENETİM ONAY verdi (istek 36). Argümanın `-` ile başlayan bir dosya adına `./` eklenmesi önerisi RET aldı: argümanları değiştirmeme kuralıyla çelişir ve ajan için sürprizdir.
- *Gerekçe:* aynı veriyle aynı sonuç (CLAUDE.md "Güvenilir") dış programın sürümünü kapsayamaz. Kural bunu açıkça yazar, ajan bunun bir güvence olmadığını bilir. `--` ve `./` POSIX'in ve programların kendi kuralıdır [rapordan: POSIX.1-2024 Utility Syntax Guidelines, Guideline 10].
