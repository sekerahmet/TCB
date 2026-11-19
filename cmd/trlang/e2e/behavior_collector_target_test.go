package e2e

import (
	"context"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

// runtimeVariables Go'nun çalışma zamanının ortam değişkenleridir (go1.27.4 src/runtime/extern.go):
// dilin ayarı değildirler (19.12a). Test sürecinin ortamından gelen değer ölçüyü bozmasın diye
// alt sürecin ortamından çıkarılırlar.
var runtimeVariables = []string{"GOGC", "GOMEMLIMIT", "GODEBUG", "GOMAXPROCS", "GORACE", "GOTRACEBACK"}

// gcTraceLine gctrace'in toplama satırıdır: "A->B->C MB, D MB goal"; C toplamadan sonra canlı öbek,
// D bir sonraki toplamanın hedefi değil bu toplamanın hedefidir: önceki toplamanın canlısından kurulur.
var gcTraceLine = regexp.MustCompile(`(\d+)->(\d+)->(\d+) MB, (\d+) MB goal`)

// TestTrlangKeepsTheGoCollectorTarget trlang'ın çöp toplayıcının hedefini seçmediğini sınar: hedef
// Go'nun 100'üdür, yani her toplamanın hedefi önceki toplamanın canlısının iki katı kadardır (200'de
// üç katı). Tanım gizli ayarı yasaklar; programın tek bellek ayarı bellek tavanıdır (19.12a).
func TestTrlangKeepsTheGoCollectorTarget(t *testing.T) {
	dir := reporoot.Path("examples", "tr")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, trBinary, "36_karakter_modeli.tr")
	cmd.Dir = dir
	cmd.Env = append(withoutRuntimeVariables(os.Environ()), "GODEBUG=gctrace=1")
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("36_karakter_modeli.tr: %v\n%s", err, stderr.String())
	}
	var live, goal []int
	for _, m := range gcTraceLine.FindAllStringSubmatch(stderr.String(), -1) {
		c, _ := strconv.Atoi(m[3])
		d, _ := strconv.Atoi(m[4])
		live, goal = append(live, c), append(goal, d)
	}
	pairs := 0
	for n := 1; n < len(live); n++ {
		if live[n-1] <= 16 {
			continue
		}
		pairs++
		if r := float64(goal[n]) / float64(live[n-1]); r >= 2.5 {
			t.Errorf("toplama %d: hedef %d MB, önceki canlı %d MB, oran %.2f; Go'nun 100'ünde ~2 olmalı (200'de ~3)", n+1, goal[n], live[n-1], r)
		}
	}
	if pairs < 10 {
		t.Fatalf("önceki canlısı 16 MB'yi aşan %d toplama çifti var, en az 10 gerekir (%d toplama satırı)", pairs, len(live))
	}
}

// withoutRuntimeVariables ortamdan çalışma zamanının değişkenlerini çıkarır.
func withoutRuntimeVariables(env []string) []string {
	var out []string
	for _, e := range env {
		name, _, _ := strings.Cut(e, "=")
		keep := true
		for _, v := range runtimeVariables {
			if strings.EqualFold(name, v) {
				keep = false
			}
		}
		if keep {
			out = append(out, e)
		}
	}
	return out
}
