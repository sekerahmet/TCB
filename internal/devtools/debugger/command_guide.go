package debugger

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Terminal hata ayıklayıcısının komut tabloları ve yardım metni. Komutlar ad kodudur (K3xx):
// tr komutları gibi her yazım dilinin adı kabul edilir (devam da continue da), yardım ve
// iletiler etkin dildedir. F tuşları simgedir, kod almaz. Kısaltmalar ve Türkçe harfsiz
// biçimler çalışmaz, yalnızca asıl komutu söyler.

// keyGuide komut isteminde gösterilen kısa tuş listesidir.
func keyGuide() string {
	return messages.Text(messages.H8000, codes.DisplayName(codes.K303), codes.DisplayName(codes.K301), codes.DisplayName(codes.K300),
		codes.DisplayName(codes.K302), codes.DisplayName(codes.K304), codes.DisplayName(codes.K305), codes.DisplayName(codes.K308), codes.DisplayName(codes.K307))
}

// keyCommands F tuşlarının komutlarıdır (düzenleyicinin hata ayıklama tuşlarıyla aynı anlam).
var keyCommands = map[string]codes.Code{"f11": codes.K300, "f10": codes.K301, "shift+f11": codes.K302, "f5": codes.K303}

// commandCode komut satırının ilk sözcüğünün komut kodudur: F tuşu ya da herhangi bir yazım
// dilinde komut adı. Komut değilse codes.None.
func commandCode(word string) codes.Code {
	if k, ok := keyCommands[word]; ok {
		return k
	}
	if k, ok := codes.FindDebuggerCommand(word); ok {
		return k
	}
	return codes.None
}

// oldCommand bir sözcüğün asıl komutunu verir: başka hata ayıklayıcıların kısaltmaları
// (tablodan), Türkçe harfsiz biçimler (adim → adım, kod listesinden).
func oldCommand(word string) (codes.Code, bool) {
	if k, ok := oldCommands[word]; ok {
		return k, true
	}
	return codes.Active().Suggest(codes.NamespaceDebugger, word, nil)
}

// oldCommands başka hata ayıklayıcılardan gelen kısaltmalardır (gdb'nin n, c, bt). Bunlar
// çalışmaz, asıl komutu söyler; Türkçe harfsiz biçimleri (adim) kod listesi önerir.
var oldCommands = map[string]codes.Code{
	"a": codes.K300, "s": codes.K300,
	"g": codes.K301, "n": codes.K301,
	"o": codes.K302,
	"d": codes.K303, "c": codes.K303,
	"goto": codes.K305, "j": codes.K305,
	"k": codes.K306, "b": codes.K306,
	"w":  codes.K307,
	"t":  codes.K308,
	"bt": codes.K309,
	"p":  codes.K310, "?": codes.K310,
	"l": codes.K311,
	"h": codes.K312,
	"q": codes.K313,
}

func (d *Debugger) printDebugHelp() {
	fmt.Println("\n  🛠️  " + messages.Text(messages.H8010))
	fmt.Println("  " + messages.Text(messages.H8011))
	// Her satır tek iletidir; kullanım ile açıklamayı sekme ayırır, sütunu tabwriter hizalar
	// (komut adlarının uzunluğu dile göre değişir).
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', 0)
	for _, row := range []string{
		messages.Text(messages.H8012, codes.DisplayName(codes.K300)),
		messages.Text(messages.H8013, codes.DisplayName(codes.K301)),
		messages.Text(messages.H8014, codes.DisplayName(codes.K302)),
		messages.Text(messages.H8015, codes.DisplayName(codes.K303)),
		messages.Text(messages.H8016, codes.DisplayName(codes.K304)),
		messages.Text(messages.H8017, codes.DisplayName(codes.K305)),
		messages.Text(messages.H8018, codes.DisplayName(codes.K307)),
		messages.Text(messages.H8019, codes.DisplayName(codes.K306)),
		messages.Text(messages.H8020, codes.DisplayName(codes.K308), interp.ValList, interp.ValTensor, interp.ValDict),
		messages.Text(messages.H8021, codes.DisplayName(codes.K309)),
		messages.Text(messages.H8022, codes.DisplayName(codes.K310)),
		messages.Text(messages.H8023, codes.DisplayName(codes.K313)),
	} {
		fmt.Fprintln(w, "  • "+row)
	}
	w.Flush()
}
