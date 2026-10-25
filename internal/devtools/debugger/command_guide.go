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
	return messages.Text(messages.H8000, codes.NameRef(codes.DbgContinue), codes.NameRef(codes.DbgNext), codes.NameRef(codes.DbgStep),
		codes.NameRef(codes.DbgOut), codes.NameRef(codes.DbgSet), codes.NameRef(codes.DbgJump), codes.NameRef(codes.DbgTable), codes.NameRef(codes.DbgWatch))
}

// keyCommands F tuşlarının komutlarıdır (düzenleyicinin hata ayıklama tuşlarıyla aynı anlam).
var keyCommands = map[string]codes.Code{"f11": codes.DbgStep, "f10": codes.DbgNext, "shift+f11": codes.DbgOut, "f5": codes.DbgContinue}

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
	"a": codes.DbgStep, "s": codes.DbgStep,
	"g": codes.DbgNext, "n": codes.DbgNext,
	"o": codes.DbgOut,
	"d": codes.DbgContinue, "c": codes.DbgContinue,
	"goto": codes.DbgJump, "j": codes.DbgJump,
	"k": codes.DbgBreak, "b": codes.DbgBreak,
	"w":  codes.DbgWatch,
	"t":  codes.DbgTable,
	"bt": codes.DbgStack,
	"p":  codes.DbgEval, "?": codes.DbgEval,
	"l": codes.DbgScreen,
	"h": codes.DbgHelp,
	"q": codes.DbgQuit,
}

func (d *Debugger) printDebugHelp() {
	fmt.Println("\n  🛠️  " + messages.Text(messages.H8010))
	fmt.Println("  " + messages.Text(messages.H8011))
	// Her satır tek iletidir; kullanım ile açıklamayı sekme ayırır, sütunu tabwriter hizalar
	// (komut adlarının uzunluğu dile göre değişir).
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', 0)
	for _, row := range []string{
		messages.Text(messages.H8012, codes.NameRef(codes.DbgStep)),
		messages.Text(messages.H8013, codes.NameRef(codes.DbgNext)),
		messages.Text(messages.H8014, codes.NameRef(codes.DbgOut)),
		messages.Text(messages.H8015, codes.NameRef(codes.DbgContinue)),
		messages.Text(messages.H8016, codes.NameRef(codes.DbgSet)),
		messages.Text(messages.H8017, codes.NameRef(codes.DbgJump)),
		messages.Text(messages.H8018, codes.NameRef(codes.DbgWatch)),
		messages.Text(messages.H8019, codes.NameRef(codes.DbgBreak)),
		messages.Text(messages.H8020, codes.NameRef(codes.DbgTable), interp.ValList, interp.ValTensor, interp.ValDict),
		messages.Text(messages.H8021, codes.NameRef(codes.DbgStack)),
		messages.Text(messages.H8022, codes.NameRef(codes.DbgEval)),
		messages.Text(messages.H8023, codes.NameRef(codes.DbgQuit)),
	} {
		fmt.Fprintln(w, "  • "+row)
	}
	w.Flush()
}
