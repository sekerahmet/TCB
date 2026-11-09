package httpserver

import (
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
)

// fileResponse TR_file_response(kök, yol)'dur (28.3a): kök klasördeki dosyayı cevap olarak verir. Yolu
// program kurmaz: baştaki / atılır, dosya kökün içinde açılır. Bulunmayan dosya, reddedilen yol ve
// klasör 404'tür; klasör listelenmez.
func fileResponse(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := interp.CallPrefix(codes.FnFileResponse)
	if len(named) > 0 {
		return nil, interp.NoNamedArgsError(fn, named)
	}
	if len(args) != 2 {
		return nil, interp.ArgCountError(codes.FnFileResponse, 2, len(args))
	}
	root, ok, err := interp.PathArg(fn, args[0])
	if !ok {
		return nil, interp.ArgTypeError(codes.FnFileResponse, 1, args[0], codes.TypeString, codes.TypePath)
	}
	if err != nil {
		return nil, err
	}
	p, ok := args[1].(*interp.StringVal)
	if !ok {
		return nil, interp.ArgTypeError(codes.FnFileResponse, 2, args[1], codes.TypeString)
	}
	dir, _ := bind.ResolvePathTried(m, root) // 18.16: göreli kök programın kuralıyla çözülür
	var res *Response
	m.Blocking(func() { res = serveFile(os.ReadDir, dir, p.Val) })
	return res, nil
}

// serveFile yolu bileşen bileşen kökün içinde arar; readDir bir klasörün gerçek adlarını verir.
func serveFile(readDir func(string) ([]fs.DirEntry, error), root, reqPath string) *Response {
	full, ok := lookup(readDir, root, reqPath)
	if !ok || !inside(root, full) {
		return notFound()
	}
	info, err := os.Stat(full)
	if err != nil || !info.Mode().IsRegular() {
		return notFound()
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return notFound()
	}
	h := newHeaders()
	h.add("Content-Type", contentTypeOf(full))
	return &Response{status: http.StatusOK, body: data, headers: h}
}

// lookup yolun her bileşenini denetler ve klasördeki gerçek adla, harf büyüklüğü dahil aynı olmasını
// ister (28.3a): Windows ve macOS'un büyük-küçük harf duyarsız açması geçmez, kısa ad (INDEX~1.HTM)
// klasörün listesinde olmadığı için bulunmaz.
func lookup(readDir func(string) ([]fs.DirEntry, error), root, reqPath string) (string, bool) {
	rest := strings.TrimPrefix(reqPath, "/")
	if rest == "" {
		return "", false
	}
	cur := root
	for _, part := range strings.Split(rest, "/") {
		if !allowedComponent(part) {
			return "", false
		}
		entries, err := readDir(cur)
		if err != nil {
			return "", false
		}
		found := false
		for _, e := range entries {
			if e.Name() == part {
				found = true
				break
			}
		}
		if !found {
			return "", false
		}
		cur = filepath.Join(cur, part)
	}
	return cur, true
}

// allowedComponent bir yol bileşeni 28.3a'nın kurallarından geçiyor mu? Kurallar her işletim
// sisteminde aynıdır: aygıt adı, sonu nokta ya da boşluk, ':' reddedilir; boş, '.', '..', '\' ve
// denetim karakteri de (kökten çıkan ya da Windows'ta başka anlama gelen yol).
func allowedComponent(c string) bool {
	switch {
	case c == "" || c == "." || c == "..", strings.ContainsAny(c, `\:`), hasControl(c):
		return false
	case strings.HasSuffix(c, ".") || strings.HasSuffix(c, " "):
		return false
	}
	return !isDeviceName(c)
}

// isDeviceName bileşenin ilk '.'ya kadarki kısmı, sondaki boşluklar atılınca, ASCII harf büyüklüğüne
// bakmadan bir Windows aygıt adı mı (28.3a)? Uzantılı biçim de reddedilir.
func isDeviceName(c string) bool {
	base, _, _ := strings.Cut(c, ".")
	base = asciiUpper(strings.TrimRight(base, " "))
	switch base {
	case "CON", "PRN", "AUX", "NUL", "CONIN$", "CONOUT$":
		return true
	}
	if len(base) < 4 || base[:3] != "COM" && base[:3] != "LPT" {
		return false
	}
	switch base[3:] {
	case "0", "1", "2", "3", "4", "5", "6", "7", "8", "9", "¹", "²", "³":
		return true
	}
	return false
}

// asciiUpper yalnız ASCII harfleri büyütür (yerel ayara göre değil: conin$ her makinede CONIN$).
func asciiUpper(s string) string {
	b := []byte(s)
	for i, c := range b {
		if 'a' <= c && c <= 'z' {
			b[i] = c - 'a' + 'A'
		}
	}
	return string(b)
}

// inside dosyanın gerçek yeri (bağlantılar çözülünce) kökün gerçek yerinin içinde mi? Kökten çıkan
// bağlantı reddedilir, kökün içindeki izlenir (28.3a; 18.16'nın tek istisnası).
func inside(root, full string) bool {
	r, err1 := filepath.EvalSymlinks(root)
	f, err2 := filepath.EvalSymlinks(full)
	if err1 != nil || err2 != nil {
		return false
	}
	rel, err := filepath.Rel(r, f)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

// notFound 404'tür; gövde insan içindir.
func notFound() *Response {
	h := newHeaders()
	h.add("Content-Type", "text/plain; charset=utf-8")
	return &Response{status: http.StatusNotFound, body: []byte(messages.Text(messages.H7630) + "\n"), headers: h}
}

// contentTypes 28.3a'nın sabit tablosudur: tür işletim sistemine göre değişmez (mime.TypeByExtension
// kullanılmaz).
var contentTypes = map[string]string{
	".html": "text/html; charset=utf-8", ".htm": "text/html; charset=utf-8",
	".js": "text/javascript; charset=utf-8", ".mjs": "text/javascript; charset=utf-8",
	".css": "text/css; charset=utf-8", ".json": "application/json", ".map": "application/json",
	".txt": "text/plain; charset=utf-8", ".xml": "application/xml", ".svg": "image/svg+xml",
	".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".gif": "image/gif",
	".webp": "image/webp", ".avif": "image/avif", ".ico": "image/x-icon", ".wasm": "application/wasm",
	".woff": "font/woff", ".woff2": "font/woff2", ".pdf": "application/pdf", ".wav": "audio/wav",
	".mp3": "audio/mpeg",
}

// contentTypeOf uzantıdan, harf büyüklüğüne bakmadan türdür; tabloda olmayan application/octet-stream.
func contentTypeOf(name string) string {
	if t, ok := contentTypes[strings.ToLower(filepath.Ext(name))]; ok {
		return t
	}
	return "application/octet-stream"
}
