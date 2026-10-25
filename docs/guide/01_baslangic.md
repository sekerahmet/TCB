# 1. Hızlı Başlangıç Komutları

> **Güvenlik:** TR programları kum havuzunda (sandbox) çalışmaz: bir `.tr` programı onu çalıştıran kişinin yetkileriyle dosya okur, yazar ve siler. Tanımadığınız bir programı çalıştırmayın; güvenilmeyen sayılıp korunan girdiler (tensör dosyaları, kontrol noktaları, veri ve parçalayıcı dosyaları) [`SECURITY.md`](../../SECURITY.md)'de.

Yorumlayıcıyı proje kökünde derleyin:

```bash
go build -o bin/tr.exe ./cmd/tr
```

Program dosyası her x86-64 işlemcide çalışır: matris çarpımı ve vektör kernelleri çalışma anında seçilir. AVX2/FMA'lı işlemcide (Intel 2013, AMD 2015 sonrası) assembly kernelleri, öbürlerinde taşınabilir Go eşleri kullanılır; eğitim adımı AVX2'li yolda yaklaşık 8 kat hızlıdır (karakter modeli, 2 çekirdek: ~10 ve ~85 ms/adım; ölçüm: [`tools/bench/`](../../tools/bench/README.md)). `GOAMD64=v3` ile derlemek ölçümde hız kazandırmıyor, yalnızca AVX2'siz işlemcide açılmayan bir program dosyası verir. `TR_CPU_AVX2=0` bu kernelleri kapatır (sorun ayıklama için). ARM (arm64) taşınabilir kernelleri kullanır. Aynı tohumla aynı sonuç sözü aynı program dosyası ve aynı işlemci türü için geçerlidir: AVX2'li ve AVX2'siz yol (kaynaştırılmış çarpma-toplama yüzünden) son basamaklarda farklı sonuç verebilir.

| Komut | Açıklama |
| :--- | :--- |
| `./bin/tr.exe` | Etkileşimli TR kabuğunu (REPL) kişinin yazım dilinde başlatır (`/h` ile hata ayıklayıcı açılır) |
| `./bin/tr.exe dosya.tr [argümanlar]` | `.tr` kaynak dosyasını derler ve anında çalıştırır (`./bin/tr.exe çalıştır dosya.tr` ile aynı). Dosya adından sonrakiler programa iletilir; program okumazsa çıkışta uyarı yazılır |
| `./bin/tr.exe izle dosya.tr [argümanlar]` | Programı çalıştırır ve hızını canlı gösterir: ms/adım, en pahalı tensör işlemleri, en sıcak satırlar, GPU, bellek; bitince rapor (bkz. §11) |
| `./bin/tr.exe profil [--json yol] dosya.tr [argümanlar]` | Programı çalıştırır, bitince hız raporu verir; `--json` ile JSON raporu (klasör verilirse içine `speed.json`) (bkz. §11) |
| `./bin/tr.exe hata_ayıkla dosya.tr` | Terminalde etkileşimli TR Hata Ayıklayıcıyı başlatır |
| `./bin/tr.exe kontrol dosya.tr` | Kodu çalıştırmadan sözdizimi ve akıllı tip kontrolü yapar |
| `./bin/tr.exe derle dosya.tr` | `.tr` dosyasını tek başına çalışan bağımsız **`.exe`** programına dönüştürür |
| `./bin/tr.exe test [dosya.tr \| klasör]` | Test bloklarını çalıştırır; klasör verilirse (varsayılan: bulunulan klasör) altındaki tüm `*_test.tr` dosyalarını tarar. Bir test bile kalırsa çıkış kodu 1'dir (bkz. §7) |
| `./bin/tr.exe biçimle dosya.tr` | Kodu standart biçime getirip dosyaya yazar; klasör verilirse içindeki tüm `.tr` dosyalarını biçimler (bkz. §8) |
| `./bin/tr.exe biçimle --kontrol dosya.tr` | Dosyayı değiştirmeden biçimlenmesi gerekip gerekmediğini söyler; gerekiyorsa çıkış kodu 1'dir |
| `./bin/tr.exe kodla dosya.tr` | Dosyayı yerinde **kodlu biçime** çevirir: dilin sözcükleri dilden bağımsız kodlarla (`TR_kw_if`, `TR_fn_print`) saklanır, gerisi bayt bayt aynı kalır; klasör verilirse içindeki tüm `.tr` dosyalarını çevirir, zaten kodlu dosyaya dokunmaz. Kodlu dosya çevrilmeden çalışır, test edilir ve aktarılır (bkz. `docs/spec/17_kodlar.md`, Kodlu saklama) |
| `./bin/tr.exe çöz dosya.tr` | Kodlu dosyayı yerinde Türkçe yazıma geri çevirir (klasör de verilebilir) |
| `./bin/tr.exe göster dosya.tr [--dil tr]` | Dosyayı bir yazım dilinde (`tr`, `en`; verilmezse kişinin dili, `TR_DIL`) standart çıktıya basar; kodlu dosyanın hata mesajlarındaki satır ve sütunlar bu gösterime göredir |
| `./bin/tr.exe göster -`, `./bin/tr.exe kodla -` | Metni standart girişten okur, gösterimini ya da kodlu biçimini standart çıktıya yazar (düzenleyiciler için; çıkış kodu 2: metin ayrıştırılamadı, çevrilmedi) |
| `./bin/tr.exe dap` | VS Code hata ayıklama sunucusu; eklenti kendisi başlatır, elle çalıştırılmaz |
| `./bin/tr.exe yardım` / `-h`, `./bin/tr.exe sürüm` / `-v` | Yardımı ve sürümü gösterir |

Çalışan bir programı **Ctrl+C** nazikçe durdurur: program sıradaki döngü turunda ya da çağrıda durur, `ertele` blokları çalışır (dosyalar kapanır, kayıtlar yazılır), çıkış kodu 3'tür; ikinci Ctrl+C hemen çıkar (130). Ayrıntılar: `docs/spec/01_program.md`.

Her komutun tek, Türkçe bir adı vardır; Türkçe karaktersiz ya da İngilizce yazımlar (`calistir`, `bicimle`, `debug`, `--help` ...) çalışmaz, hata mesajı asıl komutu önerir (`'debug' TR'de bir komut değil; 'tr hata_ayıkla' yazmak mı istediniz?`). Kısa bayraklar `-h`, `-v`, `-k` simgedir. Kabuktan (REPL) yalnızca `çıkış` ile çıkılır; `exit` ya da `cikis` yazılınca kabuk `çıkış` yazmanızı söyler.

Öğrenmeye `examples/` klasöründeki numaralı örneklerle başlayın (`examples/tr/01_merhaba.tr` … `45_tekrar_oynat.tr`; her örneğin İngilizce yazımı `examples/en/` altında aynı numarayla durur; liste ve açıklamalar `README.md`'nin "Örnekler" bölümünde).

## Kodlu dosyalar: düzenleyici ve git farkları

Kodlu bir dosyayı (ilk satırı `//! tr kodlu 2 dil=<dil>`) VS Code'da açtığınızda Türkçe yazımıyla görürsünüz; kaydedince diske yine kodlu yazılır, satır numaraları hata mesajlarıyla ve hata ayıklayıcıyla aynıdır (ayrıntılar: [eklenti belgesi](../../editors/vscode/README.md#kodlu-dosyalar); kurallar: [17_kodlar.md, Düzenleyicide](../spec/17_kodlar.md#düzenleyicide)).

`git diff` kodlu dosyada kodları gösterir (`TR_kw_var x = 1`). Farkı Türkçe görmek için git'e `tr göster`'i bir "textconv" (farktan önce metni çeviren komut) olarak tanıtın. Depodaki `.gitattributes` `.tr` dosyalarını `tr` adlı fark sürücüsüne bağlar (`*.tr diff=tr`); sürücüyü siz, kendi git ayarınızda bir kez tanımlarsınız:

```bash
git config --global diff.tr.textconv "tr göster"                      # tr PATH üzerindeyse
git config --global diff.tr.textconv "'C:/Projeler/TR/bin/tr.exe' göster"   # ya da tam yolla (boşluklu yol tek tırnakta)
```

Sonra `git diff`, `git log -p` ve `git show` kodlu dosyalarda Türkçe fark gösterir (Türkçe yazılmış dosyalar olduğu gibi gösterilir); ham farkı görmek için `git diff --no-textconv`. Sürücü tanımlı değilse git olağan farkı gösterir, hiçbir şey bozulmaz. Textconv yalnızca gösterimi değiştirir: depoya kodlu metin girer, birleştirme de kodlu metin üzerinde yapılır. Bir birleştirme çakışması dosyayı ayrıştırılamaz bıraktıysa `tr göster` hata verir (git "textconv failed" der); çakışmayı düzeltin ya da `--no-textconv` kullanın. VS Code'un kendi kaynak denetimi görünümündeki farklar ham (kodlu) metni gösterir.
