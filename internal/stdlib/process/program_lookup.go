package process

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Programı bulmak (21.3). exec.LookPath kullanılmaz: göreli PATH girdisini, Unix'te boş girdiyi aranır
// sayar ve Windows'ta uzantısız adı PATHEXT ile .bat ve .cmd'ye çözer; exec.Command ayırıcısız yolu da
// PATH'e götürür. Burada sonuç hep mutlak yoldur, os/exec'e arama bırakılmaz.

const onWindows = runtime.GOOS == "windows"

// Windows'ta çalışan program dosyaları Windows'un kendi arama sırasıyla, ve argümanları güvenle
// kaçırılamayan toplu iş dosyaları (21.3).
var (
	programSuffixes = []string{".com", ".exe"}
	batchSuffixes   = []string{".bat", ".cmd"}
)

// findProgram programın iletilerdeki adını ve çalışacak dosyanın mutlak yolunu verir.
func findProgram(code codes.Code, v interp.Value) (name, path string, err error) {
	switch x := v.(type) {
	case *interp.StringVal:
		path, err = findName(code, x.Val)
		return x.Val, path, err
	case interp.PathValuer:
		path, err = checkFile(code, x.FilePath())
		return x.FilePath(), path, err
	}
	return "", "", interp.ArgTypeError(code, 1, v, codes.TypeString, codes.TypePath)
}

// findName adı PATH'in mutlak girdilerinde arar; çalışma klasörü aranmaz.
func findName(code codes.Code, name string) (string, error) {
	fn := interp.CallPrefix(code)
	if strings.IndexByte(name, 0) >= 0 {
		return "", messages.New(messages.H7502, fn, name)
	}
	if strings.ContainsAny(name, `/\`) {
		return "", messages.New(messages.H7494, fn, name, codes.Spelling(code))
	}
	if err := checkNameSuffix(code, name); err != nil {
		return "", err
	}
	dirs := filepath.SplitList(os.Getenv("PATH"))
	dirs = slices.DeleteFunc(dirs, func(d string) bool { return !filepath.IsAbs(d) })
	if name != "" && !(onWindows && strings.ContainsRune(name, ':')) {
		for _, dir := range dirs {
			if p, ok := programIn(dir, name); ok {
				return p, nil
			}
		}
		if onWindows {
			if batch, ok := batchIn(dirs, name); ok {
				return "", messages.New(messages.H7496, fn, batch, codes.Spelling(code))
			}
		}
	}
	if words := strings.Fields(name); len(words) > 1 {
		return "", messages.New(messages.H7498, fn, name, words[0], listText(words[1:]), codes.Spelling(code))
	}
	return "", messages.New(messages.H7497, fn, name)
}

// checkNameSuffix adın sonunu denetler: .exe, .com, .bat ya da .cmd ile biten ad her işletim sisteminde
// hatadır, aynı kod hepsinde çalışsın (21.3). Harf ayrılmaz; Windows sondaki boşluk ve noktayı atar.
func checkNameSuffix(code codes.Code, name string) error {
	fn := interp.CallPrefix(code)
	trimmed := name
	if onWindows {
		trimmed = strings.TrimRight(name, " .")
	}
	for _, suffix := range slices.Concat(programSuffixes, batchSuffixes) {
		// Kesme yeri asıl metinden: küçük harfe çevirmek baytları değiştirir (İ, Ⱥ).
		cut := len(trimmed) - len(suffix)
		switch {
		case cut <= 0 || !strings.EqualFold(trimmed[cut:], suffix):
		case onWindows && slices.Contains(batchSuffixes, suffix):
			return messages.New(messages.H7496, fn, trimmed, codes.Spelling(code))
		default:
			return messages.New(messages.H7495, fn, name, trimmed[cut:], trimmed[:cut], codes.Spelling(code))
		}
	}
	return nil
}

// programIn klasörde adın çalışan dosyasını arar.
func programIn(dir, name string) (string, bool) {
	if onWindows {
		for _, suffix := range programSuffixes {
			if p := filepath.Join(dir, name+suffix); isFile(p) {
				return p, true
			}
		}
		return "", false
	}
	p := filepath.Join(dir, name)
	info, err := os.Stat(p)
	return p, err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0
}

// batchIn adın PATH'te yalnız toplu iş dosyası olarak bulunduğunu söyler; iletinin açık yazımı dosyanın
// adını ister.
func batchIn(dirs []string, name string) (string, bool) {
	for _, dir := range dirs {
		for _, suffix := range batchSuffixes {
			if isFile(filepath.Join(dir, name+suffix)) {
				return name + suffix, true
			}
		}
	}
	return "", false
}

func isFile(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.Mode().IsRegular()
}

// checkFile yolu denetler ve mutlak yapar: göreli yol çalışma klasörüne göredir (21.3).
func checkFile(code codes.Code, given string) (string, error) {
	fn := interp.CallPrefix(code)
	if strings.IndexByte(given, 0) >= 0 {
		return "", messages.New(messages.H7502, fn, given)
	}
	if err := interp.CheckPath(fn, given); err != nil {
		return "", err
	}
	abs, err := filepath.Abs(given)
	if err != nil {
		return "", messages.New(messages.H7501, fn, given, err)
	}
	if onWindows {
		if err := checkWindowsFile(code, given, abs); err != nil {
			return "", err
		}
	}
	if _, err := os.Stat(abs); os.IsNotExist(err) {
		return "", messages.New(messages.H7499, fn, given)
	}
	return abs, nil
}

// checkWindowsFile çözülen yolu izin listesiyle sınar (21.3). Sonekle denetim yetmez: Windows sondaki
// boşluk ve noktayı atar, CreateProcess hello.bat␠ yolunu toplu iş olarak çalıştırır. Ad bu yüzden önce
// arındırılır, harf ayrılmaz, yalnız .exe ve .com geçer ve dosya tam adıyla var olmalıdır. İki noktalı
// ad bir veri akışıdır (hello.bat:.exe), program dosyası değildir.
func checkWindowsFile(code codes.Code, given, abs string) error {
	fn := interp.CallPrefix(code)
	base := filepath.Base(given)
	trimmed := strings.TrimRight(base, " .")
	suffix := strings.ToLower(filepath.Ext(trimmed))
	switch {
	case strings.ContainsRune(trimmed, ':'):
		return messages.New(messages.H7500, fn, given)
	case slices.Contains(batchSuffixes, suffix):
		return messages.New(messages.H7496, fn, given, codes.Spelling(code))
	case slices.Contains(programSuffixes, suffix):
		if trimmed != base {
			return messages.New(messages.H7499, fn, given)
		}
		return nil
	}
	for _, s := range batchSuffixes {
		if isFile(abs + s) {
			return messages.New(messages.H7496, fn, given+s, codes.Spelling(code))
		}
	}
	return messages.New(messages.H7500, fn, given)
}
