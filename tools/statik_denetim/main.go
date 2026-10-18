// statik_denetim, verilen Linux (ELF) program dosyasının statik olduğunu denetler: yorumlayıcı
// (PT_INTERP) ve dinamik kitaplık bağımlılığı (DT_NEEDED) olmamalıdır. GPU desteği olmadan
// ("gpusuz" yapı etiketiyle) derlenen tr program dosyasının glibc gerektirmediğini tam denetim bu
// araçla doğrular (bkz. docs/mimari.md, GPU arka ucu).
//
// Kullanım (depo kökünden):
//
//	go run ./tools/statik_denetim PROGRAM_DOSYASI   // statikse 0, değilse bağımlılıkları yazıp 1
//
// Katman: araçlar (tools); kullanıcıya gitmez.
package main

import (
	"debug/elf"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "kullanım: go run ./tools/statik_denetim PROGRAM_DOSYASI")
		os.Exit(2)
	}
	f, err := elf.Open(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "ELF dosyası açılamadı:", err)
		os.Exit(2)
	}
	defer f.Close()
	statik := true
	for _, p := range f.Progs {
		if p.Type == elf.PT_INTERP {
			fmt.Println("yorumlayıcı (PT_INTERP) var: program dosyası dinamik bağlı")
			statik = false
		}
	}
	kitapliklar, err := f.ImportedLibraries()
	if err != nil {
		fmt.Fprintln(os.Stderr, "dinamik bölüm okunamadı:", err)
		os.Exit(2)
	}
	if len(kitapliklar) > 0 {
		fmt.Println("dinamik kitaplık bağımlılıkları (DT_NEEDED):", kitapliklar)
		statik = false
	}
	if !statik {
		os.Exit(1)
	}
	fmt.Println("program dosyası statik (yorumlayıcı ve dinamik kitaplık bağımlılığı yok)")
}
