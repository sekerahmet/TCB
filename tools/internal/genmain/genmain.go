// Package genmain, üretici araçların (tools/gen/...) ortak komut satırı iskeletidir: -check ve
// -root bayrakları, üretilen baytları hedef dosyayla karşılaştırma ya da diske yazma ve testlerin
// "üretilen dosya güncel mi" sınaması. Dört üretici de yalnızca kendi Generate işlevini verir.
//
// Layer: tools (tools); kullanıcıya gitmez.
package genmain

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/sekerahmet/TCB/tools/internal/repodir"
)

// Generator üretilecek dosyanın baytlarını depo kökünden (root) üretir.
type Generator func(root string) ([]byte, error)

// RunMain bir üreticinin main işlevidir: name üreticinin klasör adıdır (go run ./tools/gen/<name>),
// relOut üretilen dosyanın depo köküne göre yoludur. -check dosyayı yazmadan karşılaştırır ve
// güncel değilse 1 koduyla çıkar (satır sonu farkı CRLF/LF sayılmaz); -root verilmezse go.mod
// çalışma klasöründen yukarı aranır.
func RunMain(name, relOut string, gen Generator) {
	check := flag.Bool("check", false, "do not write the file; exit 1 if it is out of date")
	root := flag.String("root", "", "repository root (if empty, go.mod is searched upwards from the working directory)")
	flag.Parse()

	if *root == "" {
		*root = repodir.Find()
	}
	fresh, err := gen(*root)
	if err != nil {
		fail(err)
	}
	path := filepath.Join(*root, relOut)
	if *check {
		old, err := os.ReadFile(path)
		if err != nil {
			fail(err)
		}
		if !sameBytes(old, fresh) {
			fmt.Fprintf(os.Stderr, "%s is out of date; regenerate it with go run ./tools/gen/%s\n", path, name)
			os.Exit(1)
		}
		fmt.Printf("%s is up to date\n", path)
		return
	}
	if err := os.WriteFile(path, fresh, 0o644); err != nil {
		fail(err)
	}
	fmt.Printf("%s written\n", path)
}

// AssertUpToDate üretilen dosyanın (root'a göre relOut) üreticinin bugünkü çıktısıyla aynı
// olduğunu sınar; satır sonu farkı (Windows CRLF çalışma ağacı) sayılmaz.
func AssertUpToDate(t *testing.T, root, name, relOut string, gen Generator) {
	t.Helper()
	fresh, err := gen(root)
	if err != nil {
		t.Fatal(err)
	}
	old, err := os.ReadFile(filepath.Join(root, relOut))
	if err != nil {
		t.Fatal(err)
	}
	if !sameBytes(old, fresh) {
		t.Fatalf("%s is out of date; run `go run ./tools/gen/%s` at the repository root", relOut, name)
	}
}

func sameBytes(old, fresh []byte) bool {
	return bytes.Equal(bytes.ReplaceAll(old, []byte("\r\n"), []byte("\n")), fresh)
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}
