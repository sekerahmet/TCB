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
| `PATH` ve `.bat` | sisteme bağlı | 1.19'dan beri göreli `PATH` sonucu yok | 1.77.2: kaçırılamayan `.bat` argümanında `InvalidInput` | Node: `.bat` ve `.cmd` `EINVAL` | göreli `PATH` yok, `.bat` ve `.cmd` yok (21.3) |

### 21.1

- *Emsal:* Go `os/exec` "intentionally does not invoke the system shell" [DENETİM açtı: pkg.go.dev/os/exec, 2026-11-02]. Rust'ın `Command`'ı ve Deno'nun `Deno.Command`'ı da programı ve argümanları ayrı alır [rapordan: doc.rust-lang.org/std/process/struct.Command.html; docs.deno.com/api/deno/~/Deno.Command]. Python'un `shell=True`'su ve Node'un `exec`'i kabukla çalışır [rapordan]. *Kabuk seçeneğini almadık.* Boş aygıt varsayılanı Go'dan: "If Stdin is nil, the process reads from the null device" [DENETİM açtı: pkg.go.dev/os/exec].
- *Gerekçe:* ajan `"ffmpeg -i " + yol` yazdığında boşluklu bir dosya adı komutu böler, `;` içeren bir ad yeni bir komut başlatır. Kabuk yoksa argüman her zaman tek argümandır, enjeksiyon ve kırılma yazımdan kalkar. Program ile argümanların iki ayrı parametre olması, Go ve Rust'taki gibi, adın argümanla karışmasını önler. Yol argümanı `TR_String(yol)` ile `/` ayırıcılı gider (18.1); `ffmpeg` bunu her sistemde kabul eder, `cmd`'nin iç komutları gibi bazı araçlar Windows'ta etmeyebilir [hipotez]. Bu, yolun her sistemde tek yazımının bilinçli bedelidir. Varsayılan girdinin boş aygıt olması, girdiden okuyan bir programın (`-nostdin`'i unutulmuş `ffmpeg`) sonsuza dek beklemesini kökten kaldırır. Bütün çıktının belleğe alınması tek sonuçlu kısa işler içindir; akış 21.2'dedir.

### 21.2

- *Emsal:* Python `run`/`Popen`, Go `Output`/`Start`, Rust `output`/`spawn` aynı ayrımı yapar [rapordan]. Tam `n` bayt okumak Go'nun `io.ReadFull`'u ve Rust'ın `read_exact`'idir [rapordan]. Rust'ta `Child` düşünce süreç yaşamaya devam eder [rapordan: doc.rust-lang.org/std/process/struct.Child.html]. *Bunu almadık.*
- *Gerekçe:* sonucun tipi argümanın değerinden çıkamaz (13.10): toplu çağrı bayt, akışlı çağrı süreç verir; iki ayrı ad gerekir. Tam `n` bayt okumak, okunan parçanın her zaman bütün örneklerden oluşmasını sağlar: yarım örnek 22.1'de hata olur, akışın bir bayt kaymasıyla sesin gürültüye dönmesi görünür olur. Gecikmenin `n` olduğu kuralda yazılıdır, çünkü Jarvis'in ses etkinliği algılaması küçük çerçevelerle çalışır [hipotez: Silero VAD 16 kHz'de 512 örnek; belge açılmadı]. Program biterken süreçlerin durdurulması, mikrofonu ya da hoparlörü tutan yetim bir sürecin kalmamasıdır. Beklenmemiş sürecin hatasının yazılması 10.6'nın "görevin hatası kaybolmaz" kuralıyla aynı gerekçededir.

### 21.3

- *Emsal:* Go 1.19: "Command and LookPath no longer allow results from a PATH search to be found relative to the current directory" [DENETİM açtı: go.dev/doc/go1.19, "PATH lookups"]. Rust 1.77.2 (CVE-2024-24576) güvenle kaçıramadığı `.bat` argümanında `InvalidInput` verir [DENETİM açtı: blog.rust-lang.org/2024/04/09/cve-2024-24576.html]. Node (CVE-2024-27980) `shell: false` iken `.bat` ve `.cmd`'yi reddeder [rapordan: nodejs.org/en/blog/vulnerability/april-2024-security-releases-2]. *Rust'ın ince kuralı yerine Node'un tam yasağını aldık.*
- *Gerekçe:* indirilen bir veri kümesinin klasöründeki `ffmpeg.exe` adlı bir dosya, `PATH`'te `.` ya da göreli bir girdi varken asıl programın yerine çalışır. `.bat` ve `.cmd`'nin argümanları `cmd.exe`'nin kurallarıyla yeniden ayrıştırılır ve güvenle kaçırılamaz. Tam yasak, ilk sürüm için en küçük ve doğrulanabilir kuraldır. Bedeli: `npm.cmd` gibi sarmalayıcılar ancak `cmd /c` ile çalışır, ve bu kodda görünür. Adda uzantının ve yol ayırıcısının yasak olması, ajanın tek bir işletim sistemine bağlı kod yazmasını engeller; dosyanın kendisi `TR_Path` ile, ayrı bir tiple verilir.

### 21.4

- *Emsal:* Go'da sıfır olmayan çıkış `*ExitError`'dur ve toplu çağrı standart hata çıktısını hataya koyar [DENETİM açtı: pkg.go.dev/os/exec]. Python'un `check=False` varsayılanı Pylint'te `W1510` uyarısıdır, yani unutulduğu bilinen bir hatadır [rapordan: pylint.readthedocs.io, subprocess-run-check]. Rust ve Deno kodu döndürür, hata vermez [rapordan].
- *Gerekçe:* başarısız bir `ffmpeg`'in boş çıktısıyla sessizce sürmek sessiz bir yanlıştır; 10.6'daki gibi hata kaybolmaz. Başka kodu başarı sayan programlar için kabul edilen kodlar açıkça yazılır, ajan neyi kabul ettiğini görür. Kodun kendisi `TR_wait`'in değeridir, çünkü hatanın değeri çıkış kodunu taşımaz (`TR_error_code` iletinin kodudur, 8.5). İptalle durdurulan programın hatası iptal hatasıdır: çıkış kodu TR'nin kendi isteğinin sonucudur, programın hatası değildir. Standart hata çıktısının arka planda okunması, dolu bir boru yüzünden iki tarafın birbirini beklemesini önler [rapordan: Python `communicate`, Rust `wait_with_output`].

### 21.5

- *Emsal:* Go'da `CommandContext`'in varsayılan `Cancel`'ı doğrudan `Kill`dir; iki adım ancak `Cancel`'a istek gönderen bir fonksiyon verilince olur, `WaitDelay` dolunca `Kill` gelir [DENETİM açtı: pkg.go.dev/os/exec, "By default, CommandContext sets Cancel to call the Kill method"]. *İki adımı varsayılan yaptık.* Windows'ta iş nesnesinin `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE`'u süreç ağacını birlikte kapatır [rapordan: learn.microsoft.com, JOBOBJECT_EXTENDED_LIMIT_INFORMATION].
- *Gerekçe:* söz kesmede (Jarvis'te kullanıcı konuşunca cevabın kesilmesi) konuşma görevinin iptali `aplay`'i de hemen durdurmalıdır. Hemen öldürmek dosya yazan bir `ffmpeg`'e yarım dosya bıraktırır [hipotez], bu yüzden önce nazik istek gider. 2 saniye bir seçimdir; ölçülmüş bir emsali yoktur, uygulamada ölçülüp düzeltilebilir. Süreç grubu olmadan yalnız doğrudan çocuk ölür, torunları boruyu açık tutar ve bekleme bitmez [rapordan]. Ctrl+C'nin önce TR'ye gelmesi 1.5'in "ilk istek programı nazikçe durdurur" kuralını dış programla da aynı tutar [hipotez: Windows'ta `CREATE_NEW_PROCESS_GROUP` ile; Win32 belgesi açılmadı]. Engellenmiş okumanın hemen dönmesi gerekir, çünkü iptalin ulaşmadığı bir bekleme sessizce uzar (DENETİM istek 35, Ş1).

### 21.6

- *Emsal:* Python, Go, Rust ve Deno bütün ortamı varsayılan olarak devralır [rapordan; Go cümlesini DENETİM açtı]. Codex'in `shell_environment_policy`'si gizli adlı değişkenleri süzebilir, ama varsayılanda süzmez [DENETİM açtı: config-reference]. *Varsayılanda süzmek emsalsizdir.* Python'da `env=` verilince bütün ortam silinir [rapordan]; *bunu almadık, `TR_env:` ekler.*
- *Gerekçe:* ölçütlerin sırasında güvenlik hızdan ve alışkanlıktan önce gelir. Bir ajanın çalıştırdığı araç (ya da o aracın hata çıktısı) uzak modelin anahtarını (`OPENAI_API_KEY`) başka bir yere taşıyabilir; gizli adlar 19.4'te zaten tanımlıdır. Bedeli: anahtarla çalışan bir araç (`huggingface-cli`, `gh`) anahtarsız kalıp yetki hatası verir. Hata sessiz değildir, ve ileti süzülen adı ve `TR_env:` yazımını söyleyerek onu çalışan yazıma çevirir. Python'un "verilen ortam hepsinin yerine geçer" kuralı `PATH`'i ve Windows'ta `SystemRoot`'u sessizce siler [rapordan]; ekleme bu yanlışı kaldırır.

### 21.7

- *Emsal:* Go ve Rust çıktıyı bayt olarak verir; Python `text=True` ile çözer [rapordan].
- *Gerekçe:* metin ile bayt kendiliğinden dönüşmez (3.3a). Windows'ta konsol araçları başka bir kod sayfasıyla yazabilir [hipotez: Türkçe için cp857]. O zaman `TR_String(b)` hata verir, sessizce bozuk harf üretmez.

### 21.8

- *Emsal:* rapor bu kuralları önerdi (Soru 8); DENETİM ONAY verdi (istek 36). Argümanın `-` ile başlayan bir dosya adına `./` eklenmesi önerisi RET aldı: argümanları değiştirmeme kuralıyla çelişir ve ajan için sürprizdir.
- *Gerekçe:* aynı veriyle aynı sonuç (CLAUDE.md "Güvenilir") dış programın sürümünü kapsayamaz. Kural bunu açıkça yazar, ajan bunun bir güvence olmadığını bilir. `--` ve `./` POSIX'in ve programların kendi kuralıdır [rapordan: POSIX.1-2024 Utility Syntax Guidelines, Guideline 10].
