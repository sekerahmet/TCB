# cmd/tr

`tr` komutunun giriş noktası: komutları ayrıştırır (`TR_cmd_run`, `TR_cmd_check`, `TR_cmd_test`, `TR_cmd_format`, `TR_cmd_show`, `TR_cmd_build`, `TR_cmd_profile`, `TR_cmd_watch`, `TR_cmd_index` ...) ve her birini ilgili paketin çekirdeğine bağlar. Uçtan uca komut testleri `e2e/` klasöründedir.

## Testler

Birim testleri sınadıkları dosyanın adını taşır (`x_test.go` → `x.go`). Birden çok dosyayı kesen davranış testleri:

- `behavior_repo_tr_files_test.go`: depodaki bütün .tr dosyalarının denetimden hatasız ve uyarısız geçmesi (bilerek uyaran yerler gerekçesiyle `repoWarningAllowed`'da)

## Standartlar

**Bilinmeyen seçenek dosya adı sanılmaz.** `-` ile başlayan ve komutun seçeneği olmayan argüman (tek başına `-` standart giriştir) her komutta bilinmeyen seçenektir: ileti komutun seçeneklerini sayar ve yazımı en yakın olanı önerir (`tr format --kontrl` → `--check`); `tr --help` asıl komutu önerir. Adı `-` ile başlayan dosya `--`'dan sonra verilir (`tr -- -a.tr`). Python argparse ve Go `flag` de böyle reddeder. Denetim: `TestUnknownOptionIsNotAFile` (komutları `commands` tablosundan gezer).

**`tr check --json dosya.tr` hataları makinenin okuyacağı kayıt olarak verir (dil tanımı 2.13).** Standart çıktıya her hata ve uyarı için bir satır, satırda bir JSON nesnesi: `code` (en içteki ileti kodu, sarmalayıcı H0100/H1010 değil), `severity` (`error`/`warning`), `file` (komutun çalıştığı klasöre göre göreli, `/` ayraçlı; aktarılan modüldeki hata kendi dosyasını yazar), `start` ve `end` (`line`, `column`; 1'den başlar, sütun Unicode karakteri, `end` dışlayıcı), `message` (okuyanın dilinde, konum öneksiz), varsa `related` ve `fixes` (`replacement`, `applicable`). Konumu bilinmeyen kayıtta (kullanım hatası, okunamayan dosya) `file`, `start`, `end` yazılmaz; sütunu bilinmeyende `start` yalnız `line` taşır. Başka metin yazılmaz, kayıt yoksa çıktı boştur; çıkış kodu metin kipindeki gibidir (hata 1, kullanım hatası 2, temiz ya da yalnız uyarı 0). Kullanım hataları da kayıttır (seçenek dosya adından önce yazılır; dosya adından sonrası programın argümanıdır, orada yazılan `--json` H8301 hatasıdır). `tr run` ve `tr test` `--json` almaz: çalışmadan önceki hataları `tr check`ünkilerle aynıdır. Denetim: `behavior_check_json_test.go` (e2e).

**`TR_cmd_index` alt komutları yalnızca dizinin altındadır.** `tr index definition|references|tested_by|impact ad` ve `tr index rename ad yeni` (listeler, hiçbir dosyayı değiştirmez) çalışır; `tr definition` bir komut değildir (dosya adı sanılır). Çıkış kodu sonucun durumudur: 0 tamam, 3 belirsiz ad (adaylar listelenir), 4 bulunamayan ad (yakın adlar önerilir), 1 kullanım hatası. Çıktı biçimleri ve şema: [`internal/devtools/codeindex`](../../internal/devtools/codeindex/README.md). Denetim: `TestIndexCommandAndSubcommands`, `TestIndexSubcommandsInEveryLanguage`, `TestIndexExitCodesAndJSON`, `TestIndexUsageErrors`, `TestIndexRootDiscovery`, `TestIndexRename` (cmd/tr/e2e).
