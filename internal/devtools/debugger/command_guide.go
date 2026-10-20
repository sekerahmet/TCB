package debugger

import (
	"fmt"

	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Terminal hata ayıklayıcısının komut tabloları ve yardım metni. Her komutun tek yazımı
// vardır (F tuşları dışında); eski kısaltmalar ve eşler yalnızca ipucu verir.

// keyGuide komut isteminde gösterilen kısa tuş listesidir.
func keyGuide() string { return messages.Text(messages.H8000) }

// oldKeys önceki sürümün terminal tuşlarıdır (f5 adım, f6 geç, f7 dön, f8 devam). Bu
// tuşlar artık düzenleyicinin hata ayıklama tuşlarıyla aynı anlamda kullanılır; eski
// anlamla yazılanlar başka bir iş yapmaz, yeni tuşu söyler.
var oldKeys = map[string]string{"f6": "f10 (geç)", "f7": "shift+f11 (dön)", "f8": "f5 (devam)"}

// oldCommands komutların eski kısaltmaları, Türkçe karaktersiz ve İngilizce eşleridir. Her
// komutun tek yazımı vardır (F tuşları dışında); bunlar çalışmaz, asıl komutu söyler.
var oldCommands = map[string]string{
	"adim": "adım", "a": "adım", "s": "adım", "step": "adım",
	"gec": "geç", "g": "geç", "n": "geç", "next": "geç",
	"don": "dön", "o": "dön", "out": "dön",
	"d": "devam", "c": "devam", "continue": "devam",
	"degistir": "değiştir", "ata": "değiştir", "set": "değiştir", "kalem": "değiştir",
	"zipla": "zıpla", "git": "zıpla", "goto": "zıpla", "j": "zıpla",
	"k": "kesme", "b": "kesme", "break": "kesme",
	"watch": "izle", "w": "izle",
	"incele": "tablo", "t": "tablo",
	"yigin": "yığın", "stack": "yığın", "bt": "yığın",
	"yaz": "hesapla", "p": "hesapla", "?": "hesapla",
	"göster": "ekran", "goster": "ekran", "l": "ekran",
	"yardim": "yardım", "h": "yardım", "help": "yardım",
	"cikis": "çıkış", "q": "çıkış", "quit": "çıkış",
}

func (d *Debugger) printDebugHelp() {
	fmt.Println("\n  🛠️  " + messages.Text(messages.H8010))
	fmt.Println("  " + messages.Text(messages.H8011))
	fmt.Println("  • " + messages.Text(messages.H8012))
	fmt.Println("  • " + messages.Text(messages.H8013))
	fmt.Println("  • " + messages.Text(messages.H8014))
	fmt.Println("  • " + messages.Text(messages.H8015))
	fmt.Println("  • " + messages.Text(messages.H8016))
	fmt.Println("  • " + messages.Text(messages.H8017))
	fmt.Println("  • " + messages.Text(messages.H8018))
	fmt.Println("  • " + messages.Text(messages.H8019))
	fmt.Println("  • " + messages.Text(messages.H8020, interp.VAL_LIST, interp.VAL_TENSOR, interp.VAL_DICT))
	fmt.Println("  • " + messages.Text(messages.H8021))
	fmt.Println("  • " + messages.Text(messages.H8022))
	fmt.Println("  • " + messages.Text(messages.H8023))
}
