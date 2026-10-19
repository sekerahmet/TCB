package interp

import (
	"fmt"
	"strings"
)

// Betiğe komut satırında verilen argümanlar ('tr betik.tr --adım 5 veri' → ["--adım", "5",
// "veri"]). 'tr betik.tr', 'tr çalıştır', 'tr hata_ayıkla', 'tr derle' ile üretilen uygulama
// ve düzenleyicinin başlatma ayarındaki "args" aynı yoldan buraya gelir. Program argümanları
// okuyan yerleşik (komut_satırı, internal/stdlib/process) Argumanlar ile alır; okunmayan argümanlar program
// bitince uyarıyla bildirilir (sessizce yok sayılmasın).

// SetArgs records the program's arguments (once, before the program starts).
func (v *VM) SetArgs(arguments []string) {
	v.arguments = append([]string(nil), arguments...)
	v.argsRead.Store(false)
}

// Args returns a copy of the program's arguments and marks them as read.
func (v *VM) Args() []string {
	v.argsRead.Store(true)
	return append([]string(nil), v.arguments...)
}

// ProgramCommand is how the command that started the program is written (KomutAdi; when empty
// "tr <SourceFile>").
func (v *VM) ProgramCommand() string {
	if v.KomutAdi != "" {
		return v.KomutAdi
	}
	if v.SourceFile == "" {
		return "tr"
	}
	return "tr " + v.SourceFile
}

// UnusedArgsWarning is the warning to write to standard error when the program ended without
// reading its arguments; empty when no arguments were given or they were read.
func (v *VM) UnusedArgsWarning() string {
	if len(v.arguments) == 0 || v.argsRead.Load() {
		return ""
	}
	return fmt.Sprintf("⚠️ Uyarı: programa verilen %d argüman kullanılmadı (%s); program komut satırı argümanlarını okumuyor",
		len(v.arguments), v.Mask(strings.Join(v.arguments, " ")))
}
