package httpserver

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/sekerahmet/TCB/internal/testkit/trrun"
)

// screenDir kökün içinde iki dosya, bir alt klasör ve kökün dışında bir dosya kurar.
func screenDir(t *testing.T) (root, outside string) {
	t.Helper()
	base := t.TempDir()
	root = filepath.Join(base, "ekran")
	for name, body := range map[string]string{"index.html": "<p>", "app.js": "x", "lang/tr.js": "t", "font.WOFF2": "w", "veri.bin": "b"} {
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	outside = filepath.Join(base, "gizli.txt")
	if err := os.WriteFile(outside, []byte("dışarıda"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, outside
}

// TestFileResponseServesInsideTheRoot: kökteki dosya 200 ve tablonun türüyle gelir; bulunmayan dosya,
// klasör ve reddedilen yol 404'tür, klasör listelenmez (28.3a).
func TestFileResponseServesInsideTheRoot(t *testing.T) {
	root, _ := screenDir(t)
	cases := map[string]struct {
		code  int
		ctype string
	}{
		"/index.html":   {200, "text/html; charset=utf-8"},
		"/app.js":       {200, "text/javascript; charset=utf-8"},
		"/lang/tr.js":   {200, "text/javascript; charset=utf-8"},
		"/font.WOFF2":   {200, "font/woff2"},
		"/veri.bin":     {200, "application/octet-stream"},
		"index.html":    {200, "text/html; charset=utf-8"},
		"/":             {404, ""},
		"/lang":         {404, ""},
		"/lang/":        {404, ""},
		"/yok.html":     {404, ""},
		"//index.html":  {404, ""},
		"/./index.html": {404, ""},
		"/../gizli.txt": {404, ""},
	}
	for p, want := range cases {
		r := serveFile(os.ReadDir, root, p)
		got, _ := r.headers.get("Content-Type")
		if r.status != want.code || want.code == 200 && got != want.ctype {
			t.Fatalf("%s: %d %q, beklenen %d %q", p, r.status, got, want.code, want.ctype)
		}
	}
}

// TestFileComponentRules: bileşen kuralları her işletim sisteminde aynıdır (28.3a): aygıt adı (uzantılı
// ve üst simge rakamlı biçim dahil, ASCII harf büyüklüğüne bakmadan), sonu nokta ya da boşluk, ':' reddedilir.
func TestFileComponentRules(t *testing.T) {
	bad := []string{"con", "CON", "Con.js", "nul.txt", "CON .js", "aux", "prn", "conin$", "CONOUT$.x", "com0", "COM9", "lpt1.txt",
		"COM¹", "lpt³", "a.js.", "a.js ", "a:b", "a\\b", "", ".", "..", "a\x00b"}
	for _, c := range bad {
		if allowedComponent(c) {
			t.Fatalf("%q reddedilmeli", c)
		}
	}
	good := []string{"index.html", "console.js", "com", "com10", "lpt", "COM4x", "a.b.c", "ş.js", "nulx", "CONIN"}
	for _, c := range good {
		if !allowedComponent(c) {
			t.Fatalf("%q geçmeli", c)
		}
	}
}

// FuzzFileComponent: bileşen denetimi hiçbir girdide paniklemez; geçen bileşen boş, '.', '..' değildir,
// \, :, denetim karakteri taşımaz, sonu nokta ya da boşluk değildir ve aygıt adı değildir (28.3a).
func FuzzFileComponent(f *testing.F) {
	for _, s := range []string{"index.html", "con", "CON .js", "a.js.", "a:b", "COM¹", "lpt3.txt", "conin$", "..", "", "ş", "nul\x00"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, c string) {
		if !allowedComponent(c) {
			return
		}
		base, _, _ := strings.Cut(c, ".")
		base = strings.ToUpper(strings.TrimRight(base, " "))
		if c == "" || c == "." || c == ".." || strings.ContainsAny(c, `\:`) || hasControl(c) ||
			strings.HasSuffix(c, ".") || strings.HasSuffix(c, " ") || base == "CON" || base == "NUL" || base == "AUX" || base == "PRN" {
			t.Fatalf("geçmemesi gereken bileşen geçti: %q", c)
		}
	})
}

// TestFileCaseMustMatch: harf büyüklüğü denetimi klasörün gerçek adıyla yapılır (28.3a). Linux'ta dosya
// sistemi zaten açmaz; denetim harfe duyarsız sahte bir dosya sistemiyle sınanır: orada /Index.html
// dosyayı açardı, TR açmaz.
func TestFileCaseMustMatch(t *testing.T) {
	fake := fstest.MapFS{"index.html": {Data: []byte("x")}, "Alt/a.js": {Data: []byte("y")}}
	readDir := func(dir string) ([]fs.DirEntry, error) {
		rel := strings.TrimPrefix(filepath.ToSlash(dir), "kök")
		rel = strings.TrimPrefix(rel, "/")
		if rel == "" {
			rel = "."
		}
		return fake.ReadDir(rel)
	}
	for p, want := range map[string]bool{"/index.html": true, "/Index.html": false, "/INDEX.HTML": false, "/Alt/a.js": true, "/alt/a.js": false} {
		if _, ok := lookup(readDir, "kök", p); ok != want {
			t.Fatalf("%s: bulundu %v, beklenen %v", p, ok, want)
		}
	}
}

// TestFileLinksStayInsideTheRoot: kökün içindeki bağlantı izlenir, kökün dışını gösteren reddedilir (28.3a).
func TestFileLinksStayInsideTheRoot(t *testing.T) {
	root, outside := screenDir(t)
	if err := os.Symlink(outside, filepath.Join(root, "dis.txt")); err != nil {
		t.Skipf("bağlantı kurulamadı (Windows'ta geliştirici kipi ya da yönetici gerekir; Linux'ta koşar): %v", err)
	}
	if err := os.Symlink(filepath.Join(root, "app.js"), filepath.Join(root, "ic.js")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Dir(outside), filepath.Join(root, "ust")); err != nil {
		t.Fatal(err)
	}
	for p, want := range map[string]int{"/ic.js": 200, "/dis.txt": 404, "/ust/gizli.txt": 404} {
		if r := serveFile(os.ReadDir, root, p); r.status != want {
			t.Fatalf("%s: %d, beklenen %d", p, r.status, want)
		}
	}
}

// TestFileResponseCall: TR_file_response(kök, yol) göreli kökü 18.16'nın kuralıyla çözer; argüman
// hataları (28.3a).
func TestFileResponseCall(t *testing.T) {
	root, _ := screenDir(t)
	r := filepath.ToSlash(root)
	o := trrun.MustExec(t, `TR_var c = TR_file_response("`+r+`", "/index.html")
TR_var d = TR_file_response("`+r+`", "/Index.html")
TR_var result = [c.TR_status, TR_String(c.TR_body), c.TR_headers["Content-Type"], d.TR_status]`)
	if got := o.Value.Inspect(); got != `[200, "<p>", "text/html; charset=utf-8", 404]` {
		t.Fatalf("dosya cevabı: %s", got)
	}
	for _, code := range []string{`TR_file_response("` + r + `")`, `TR_file_response(1, "/a")`, `TR_file_response("` + r + `", 1)`} {
		if o := trrun.Exec(t, code); o.Err == nil {
			t.Fatalf("%s hata vermeli", code)
		}
	}
}
