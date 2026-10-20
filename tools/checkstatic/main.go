// checkstatic, verilen Linux (ELF) program dosyasının statik olduğunu denetler: yorumlayıcı
// (PT_INTERP) ve dinamik kitaplık bağımlılığı (DT_NEEDED) olmamalıdır. GPU desteği olmadan
// ("gpusuz" yapı etiketiyle) derlenen tr program dosyasının glibc gerektirmediğini tam denetim bu
// araçla doğrular (bkz. docs/architecture.md, GPU arka ucu).
//
// Kullanım (depo kökünden):
//
//	go run ./tools/checkstatic BINARY   // statikse 0, değilse bağımlılıkları yazıp 1
//
// Layer: tools (tools); kullanıcıya gitmez.
package main

import (
	"debug/elf"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: go run ./tools/checkstatic BINARY")
		os.Exit(2)
	}
	f, err := elf.Open(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot open ELF file:", err)
		os.Exit(2)
	}
	defer f.Close()
	static := true
	for _, p := range f.Progs {
		if p.Type == elf.PT_INTERP {
			fmt.Println("interpreter (PT_INTERP) present: the binary is dynamically linked")
			static = false
		}
	}
	libs, err := f.ImportedLibraries()
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot read the dynamic section:", err)
		os.Exit(2)
	}
	if len(libs) > 0 {
		fmt.Println("dynamic library dependencies (DT_NEEDED):", libs)
		static = false
	}
	if !static {
		os.Exit(1)
	}
	fmt.Println("binary is static (no interpreter, no dynamic library dependencies)")
}
