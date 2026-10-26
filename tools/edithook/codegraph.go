package main

// codegraph.go: kancanın ve ön denetimin codegraph'ı koşma yolu. Önce Go aracı derler ve derleme
// önbelleğindeki ikilinin yolunu verir (`go run -exec`), sonra o ikili koşar; yol
// .codegraph/last-codegraph'a yazılır. Derleme düşerse yazılı yoldaki son derlenen ikili koşar ve
// cevabın başına bayat olduğu derleme hatasıyla yazılır: codegraph TR tarafı için dil paketlerini
// içe aktarır, düzenlenen bir dil paketi ara hâlde derlenmezken Go'ya dair soru da cevapsız
// kalıyordu. Son derlenen ikili yoksa (hiç derlenmedi ya da derleme önbelleği temizlendi) derleme
// hatası döner.
//
// Karşılaştırma: `go run` derleme hatasını programın kendi hatasından ayırmaz (ikisi de çıkış 1).
// `go build -o` ve `go install` ayırır ama hedefi her çağrıda yeniden yazar: Windows yeni yazılan
// 11 MB'lık ikiliyi okurken ve koşarken tarıyordu (kanca 2,3 sn'den 4,5 sn'ye çıktı) ve koşan bir
// ikilinin üzerine yazılamaz (aynı anda iki kanca). `go run -exec` ikiliyi önbellekte, içerik
// özetinden adlı bir klasörde bırakır; kopya yok, bedel `go run`'ınkiyle aynı (~0,6 sn).

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// goTool derleme komutudur; testler sahte bir go verir.
var goTool = "go"

// exePath -exec programının kipidir: Go ikiliyi derleyince kancayı `exe-path <ikili>` ile çağırır,
// kanca ikilinin yolunu yazar.
const exePath = "exe-path"

// mode kancanın öbür kipleridir (os.Args[1:]); ok, args bir kip değilse false.
func mode(args []string, out, errOut io.Writer) (code int, ok bool) {
	switch {
	case len(args) > 0 && args[0] == "codegraph":
		return runCodegraph(args[1:], out, errOut), true
	case len(args) > 1 && args[0] == exePath:
		fmt.Fprintln(out, args[1])
		return 0, true
	}
	return 0, false
}

// codegraph root'taki codegraph'ı args ile koşacak komutu kurar. stale boş değilse komut son
// derlenen ikilidir ve stale bunu söyleyen uyarıdır.
func codegraph(root string, args ...string) (cmd *exec.Cmd, stale string, err error) {
	self, err := os.Executable()
	if err != nil {
		return nil, "", err
	}
	last := filepath.Join(root, ".codegraph", "last-codegraph")
	var bin string
	var msg bytes.Buffer
	var buildErr error
	// Kaynak değişince Go ikiliyi önce geçici klasöründe koşar ve klasörü hemen siler; önbelleğe aldığı ikilinin yolunu
	// ikinci çağrı verir [ölçüldü]. Yine kalmazsa (önbellek kapalı) düz `go run`.
	for try := 0; try < 2; try++ {
		find := exec.Command(goTool, "run", "-exec", "'"+self+"' "+exePath, "./tools/codegraph")
		find.Dir = root
		var out bytes.Buffer
		msg.Reset()
		find.Stdout, find.Stderr = &out, &msg
		if buildErr = find.Run(); buildErr != nil {
			break
		}
		if bin = strings.TrimSpace(out.String()); exists(bin) {
			remember(last, bin)
			cmd = exec.Command(bin, args...)
			cmd.Dir = root
			return cmd, "", nil
		}
	}
	if buildErr == nil {
		cmd = exec.Command(goTool, append([]string{"run", "./tools/codegraph"}, args...)...)
		cmd.Dir = root
		return cmd, "", nil
	}
	b, _ := os.ReadFile(last)
	bin = strings.TrimSpace(string(b))
	info, statErr := os.Stat(last)
	if !exists(bin) || statErr != nil {
		return nil, "", fmt.Errorf("go build ./tools/codegraph: %v\n%s", buildErr, msg.String())
	}
	lines := strings.Split(strings.TrimRight(msg.String(), "\n"), "\n")
	cmd = exec.Command(bin, args...)
	cmd.Dir = root
	stale = fmt.Sprintf("warning: codegraph does not build with this tree; this is the answer of the last "+
		"version that built (%s), it may be stale:\n%s", info.ModTime().Format("2006-01-02 15:04"),
		strings.Join(lines[:min(len(lines), 5)], "\n"))
	return cmd, stale, nil
}

// exists yol boş değil ve dosya var mı.
func exists(path string) bool {
	_, err := os.Stat(path)
	return path != "" && err == nil
}

// remember son derlenen ikilinin yolunu yazar; yol değişmediyse dosyaya dokunmaz (yazılma zamanı o
// ikilinin derlendiği zamandır). Yazılamazsa yalnız bayat cevap yolu kaybolur.
func remember(last, bin string) {
	if b, err := os.ReadFile(last); err == nil && strings.TrimSpace(string(b)) == bin {
		return
	}
	if os.MkdirAll(filepath.Dir(last), 0o755) != nil {
		return
	}
	tmp := fmt.Sprintf("%s.%d", last, os.Getpid())
	if os.WriteFile(tmp, []byte(bin+"\n"), 0o644) != nil || os.Rename(tmp, last) != nil {
		os.Remove(tmp)
	}
}

// runCodegraph ön denetimin codegraph çağrısıdır (`go run ./tools/edithook codegraph <args>`):
// kancayla aynı yol. Bayat uyarısı hata çıktısına gider; çıkış kodu codegraph'ınkidir.
func runCodegraph(args []string, out, errOut io.Writer) int {
	root, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(errOut, "edithook:", err)
		return 1
	}
	cmd, stale, err := codegraph(root, args...)
	if err != nil {
		fmt.Fprintln(errOut, "edithook:", err)
		return 1
	}
	if stale != "" {
		fmt.Fprintln(errOut, stale)
	}
	cmd.Stdout, cmd.Stderr = out, errOut
	err = cmd.Run()
	if exit, ok := err.(*exec.ExitError); ok {
		return exit.ExitCode()
	}
	if err != nil {
		fmt.Fprintln(errOut, "edithook:", err)
		return 1
	}
	return 0
}
