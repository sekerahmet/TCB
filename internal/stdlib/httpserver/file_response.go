package httpserver

import (
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
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

// afterResolve yalnız testler içindir: boş değilse yol çözüldükten sonra, dosya açılmadan önce
// çalışır (28.3a'nın yarış testi arada bir bileşeni bağlantıyla değiştirir).
var afterResolve func()

// serveFile yolu bileşen bileşen kökün içinde arar; readDir bir klasörün gerçek adlarını verir.
// Bağlantılar çözülür (realPath), çözülmüş yolun kökün içinde olduğuna bakılır ve dosya köke
// göre göreli hâliyle os.Root'ta açılır: çözme ile açma arasında bir bileşen dışarıyı gösteren bir
// bağlantıyla değiştirilirse os.Root onu açmaz.
func serveFile(readDir func(string) ([]fs.DirEntry, error), root, reqPath string) *Response {
	full, ok := lookup(readDir, root, reqPath)
	if !ok {
		return notFound()
	}
	realRoot, ok1 := realPath(root)
	realFile, ok2 := realPath(full)
	rel, ok3 := within(realRoot, realFile)
	if !ok1 || !ok2 || !ok3 {
		return notFound()
	}
	if afterResolve != nil {
		afterResolve()
	}
	data, ok := readInRoot(realRoot, rel)
	if !ok {
		return notFound()
	}
	h := newHeaders()
	h.add("Content-Type", contentTypeOf(full))
	return &Response{status: http.StatusOK, body: data, headers: h}
}

// readInRoot dosyayı kökün içinde os.Root ile açar; boyut ve tür açılan dosyanın Stat'ından okunur.
func readInRoot(root, rel string) ([]byte, bool) {
	dir, err := os.OpenRoot(root)
	if err != nil {
		return nil, false
	}
	defer dir.Close()
	f, err := dir.Open(rel)
	if err != nil {
		return nil, false
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, false
	}
	data := make([]byte, info.Size())
	if _, err := io.ReadFull(f, data); err != nil {
		return nil, false
	}
	return data, true
}

// realPath yolun bağlantılarını çözer. Windows'ta kendi çözücüdür (resolveLinks); Linux'ta ve macOS'ta
// filepath.EvalSymlinks'tir, çünkü çözücünün hedefteki ..'yi sözcüksel temizlemesi Windows'a uyar,
// Linux'a uymaz (28.3a; TCB'nin kararı, 2026-11-10).
func realPath(p string) (string, bool) {
	if runtime.GOOS == "windows" {
		return resolveLinks(p)
	}
	r, err := filepath.EvalSymlinks(p)
	return r, err == nil
}

// maxLinks bir yolu çözerken izlenen en çok bağlantıdır (Linux'un MAXSYMLINKS'i); aşılırsa 404.
const maxLinks = 40

// resolveLinks mutlak bir yolun bağlantılarını bileşen bileşen çözer: kipi ModeSymlink ya da
// ModeIrregular olan bileşen (Windows'ta junction, Go 1.23'ten beri ModeIrregular) os.Readlink'in
// hedefiyle değiştirilir. filepath.EvalSymlinks junction'dan geçen yolda hata verdiği, os.Root da
// junction'ı ve mutlak hedefli bağlantıyı hiç açmadığı için kendi çözücüdür (DENETİM 374). Readlink'in
// her hatası (cevap vermediği öbür yeniden ayrıştırma noktaları: OneDrive, dedup) 404'tür.
func resolveLinks(p string) (string, bool) {
	cur := filepath.VolumeName(p) + string(filepath.Separator)
	todo := splitPath(p[len(filepath.VolumeName(p)):])
	links := 0
	for len(todo) > 0 {
		c := todo[0]
		todo = todo[1:]
		switch c {
		case ".":
			continue
		case "..":
			cur = filepath.Dir(cur)
			continue
		}
		next := filepath.Join(cur, c)
		info, err := os.Lstat(next)
		if err != nil {
			return "", false
		}
		if info.Mode()&(fs.ModeSymlink|fs.ModeIrregular) == 0 {
			cur = next
			continue
		}
		if links++; links > maxLinks {
			return "", false
		}
		target, err := os.Readlink(next)
		if err != nil {
			return "", false
		}
		if v := filepath.VolumeName(target); v != "" {
			cur, target = v+string(filepath.Separator), target[len(v):]
		} else if target != "" && isSeparator(rune(target[0])) {
			cur = filepath.VolumeName(cur) + string(filepath.Separator)
		}
		todo = append(splitPath(target), todo...)
	}
	return cur, true
}

// splitPath yolu ayırıcılardan böler, boş bileşenleri atar.
func splitPath(p string) []string { return strings.FieldsFunc(p, isSeparator) }

func isSeparator(r rune) bool { return r == '/' || r == filepath.Separator }

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

// within dosyanın çözülmüş yeri kökün çözülmüş yerinin içinde mi; içindeyse köke göre göreli yolu
// verir. Kökten çıkan bağlantı reddedilir, kökün içindeki izlenir (28.3a; 18.16'nın tek istisnası).
func within(root, file string) (string, bool) {
	rel, err := filepath.Rel(root, file)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", false
	}
	return rel, true
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
