package httpserver

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/testkit/trrun"
)

// TestSecretToken: TR_secret_token() 32 rastgele baytı URL'de güvenli taban 64 ile verir (43 karakter,
// [0-9A-Za-z_-]); her çağrı başka bir belirteçtir; değeri gizlidir: TR_print
// onu *** yazar (28.2a, 19.4). Belirteç 28.2'nin kuralından geçer.
func TestSecretToken(t *testing.T) {
	o := trrun.MustExec(t, `TR_var a = TR_secret_token()
TR_var b = TR_secret_token()
TR_print(a)
TR_var result = [a.TR_length, a == b]`)
	if o.Value.Inspect() != "[43, false]" {
		t.Fatalf("belirteç 43 karakter ve her çağrıda başka olmalı: %s", o.Value.Inspect())
	}
	a, _ := o.VM.GlobalEnv.Get("a")
	tok := a.Inspect()
	if !regexp.MustCompile(`^[0-9A-Za-z_-]{43}$`).MatchString(tok) || !validToken(tok) {
		t.Fatalf("belirteç URL'de güvenli taban 64 olmalı: %q", tok)
	}
	if strings.Contains(o.Out, tok) || !strings.Contains(o.Out, "***") {
		t.Fatalf("belirteç insanın göreceği çıktıda maskelenmeli: %q", o.Out)
	}
}

// TestSecretTokenNeedsOwnerOnly: belirteç dosyaya yalnız TR_owner_only: TR_true ile yazılır (28.2a, 18.13).
func TestSecretTokenNeedsOwnerOnly(t *testing.T) {
	p := filepath.ToSlash(filepath.Join(t.TempDir(), "belirtec.txt"))
	o := trrun.MustExec(t, `TR_var b = TR_secret_token()
TR_var hata = TR_error_code { TR_write_file("`+p+`", b) }
TR_write_file("`+p+`", b, TR_owner_only: TR_true)
TR_var result = [hata, TR_read_file("`+p+`") == b]`)
	if o.Value.Inspect() != `["H7600", true]` {
		t.Fatalf("belirteç yalnız sahibine açık dosyaya yazılmalı: %s", o.Value.Inspect())
	}
}
