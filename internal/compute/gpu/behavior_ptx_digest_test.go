package gpu

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
)

// Kural: benzeticide sınamalar sırasında derlenen her çekirdeğin PTX'inin SHA-256 özeti
// testdata/ptx_ozet.txt'dedir ("özet anahtar", anahtara göre sıralı). Bilinen bir çekirdeğin
// PTX'i değişirse ya da kısa kipte (CI ve tam denetimin kipi) özette olmayan bir çekirdek
// derlenirse paketin testi kalır. Bilinçli bir değişiklikte özet yenilenir:
//
//	TR_GPU_PTX_UPDATE=1 go test -short ./internal/compute/gpu
//
// Neden: "değişmeyen çekirdeklerin PTX'i aynı" iddiası her GPU işinde elle (geçici dökümle)
// gösteriliyordu; şimdi her koşuda denetlenir ve bilinçli değişiklik diff'te özetin satırı olarak
// görünür. Gerçek GPU'da atlanır (karta göre başka yollar derlenir); -run ile seçilen alt kümede
// yalnızca derlenenler karşılaştırılır.

// ptxDigestPath özet dosyasıdır.
var ptxDigestPath = filepath.Join("testdata", "ptx_ozet.txt")

// ptxSeen bu koşuda derlenen çekirdeklerin özetleridir (anahtar → özet).
var ptxSeen = struct {
	sync.Mutex
	m map[string]string
}{m: map[string]string{}}

// ptxBadNames bu koşuda derlenen ve .entry adı PTX tanıtıcısı olmayan çekirdeklerdir (anahtar →
// ad). Kural (gerçek kart, 2026-11-02): `cinsli_0_0_0_[float32 float32]_bool` benzetimde geçti,
// ptxas "Parsing error near '['" ile reddetti.
var ptxBadNames = struct {
	sync.Mutex
	m map[string]string
}{m: map[string]string{}}

// recordPTX TestMain'de kernelBuilt'e kurulur.
func recordPTX(key, text string) {
	h := sha256.Sum256([]byte(text))
	ptxSeen.Lock()
	ptxSeen.m[key] = hex.EncodeToString(h[:])
	ptxSeen.Unlock()
	if name := entryName(text); !validPTXName(name) {
		ptxBadNames.Lock()
		ptxBadNames.m[key] = name
		ptxBadNames.Unlock()
	}
}

// entryName PTX metnindeki .entry adıdır.
func entryName(text string) string {
	_, rest, _ := strings.Cut(text, ".visible .entry ")
	name, _, _ := strings.Cut(rest, "(")
	return name
}

// checkKernelNames m.Run'dan sonra çağrılır: derlenen her çekirdeğin .entry adı PTX tanıtıcısıdır
// (gerçek GPU'da da). Uymayan varsa yazar ve 1 döner.
func checkKernelNames() int {
	ptxBadNames.Lock()
	defer ptxBadNames.Unlock()
	if len(ptxBadNames.m) == 0 {
		return 0
	}
	keys := make([]string, 0, len(ptxBadNames.m))
	for k := range ptxBadNames.m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	fmt.Printf("%d çekirdeğin .entry adı PTX tanıtıcısı değil ([A-Za-z][A-Za-z0-9_$]*):\n", len(keys))
	for _, k := range keys {
		fmt.Printf("  %s: %q\n", k, ptxBadNames.m[k])
	}
	return 1
}

// Kural: validPTXName PTX ISA'nın tanıtıcı kuralıdır; gerçek kartın reddettiği ad reddedilir.
func TestValidPTXName(t *testing.T) {
	for name, want := range map[string]bool{
		"cinsli_0_0_0_float32_float32_bool": true, "ham_kopya_1_8": true, "kaynasik_00ff": true,
		"_a": true, "$a1": true, "%r1": true, "a$b": true,
		"cinsli_0_0_0_[float32 float32]_bool": false, "": false, "_": false, "1a": false,
		"a b": false, "a:b": false, "a(b)": false, "a-b": false, "dtype(12)": false, "çarpım": false,
	} {
		if got := validPTXName(name); got != want {
			t.Errorf("validPTXName(%q) = %v, beklenen %v", name, got, want)
		}
	}
}

// checkPTXDigest m.Run'dan sonra çağrılır; özetle uyuşmazlık varsa yazar ve 1 döner.
func checkPTXDigest() int {
	if !inSim() {
		return 0
	}
	ptxSeen.Lock()
	seen := ptxSeen.m
	ptxSeen.Unlock()
	subset := flag.Lookup("test.run") != nil && flag.Lookup("test.run").Value.String() != ""
	if os.Getenv("TR_GPU_PTX_UPDATE") == "1" {
		if subset || !testing.Short() {
			fmt.Println("PTX özeti yalnızca bütün paketin -short koşusuyla yenilenir (-run yok)")
			return 1
		}
		return writePTXDigest(seen)
	}
	known, err := readPTXDigest()
	if err != nil {
		fmt.Println("PTX özeti okunamadı:", err)
		return 1
	}
	var changed, added []string
	for key, sum := range seen {
		switch want, ok := known[key]; {
		case ok && want != sum:
			changed = append(changed, key)
		case !ok && testing.Short() && !subset:
			added = append(added, key)
		}
	}
	if len(changed)+len(added) == 0 {
		return 0
	}
	sort.Strings(changed)
	sort.Strings(added)
	fmt.Printf("PTX özeti (%s) tutmuyor: %d çekirdeğin PTX'i değişti, %d yeni çekirdek.\n", ptxDigestPath, len(changed), len(added))
	for _, k := range changed {
		fmt.Println("  değişti:", k)
	}
	for _, k := range added {
		fmt.Println("  yeni:   ", k)
	}
	fmt.Println("Değişiklik bilinçliyse: TR_GPU_PTX_UPDATE=1 go test -short ./internal/compute/gpu")
	return 1
}

func readPTXDigest() (map[string]string, error) {
	data, err := os.ReadFile(ptxDigestPath)
	if err != nil {
		return nil, err
	}
	m := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		sum, key, ok := strings.Cut(strings.TrimRight(line, "\r"), " ") // git'in CRLF'si
		if ok {
			m[key] = sum
		}
	}
	return m, nil
}

func writePTXDigest(seen map[string]string) int {
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(seen[k] + " " + k + "\n")
	}
	if err := os.WriteFile(ptxDigestPath, []byte(b.String()), 0o644); err != nil {
		fmt.Println("PTX özeti yazılamadı:", err)
		return 1
	}
	fmt.Printf("PTX özeti yazıldı: %d çekirdek (%s)\n", len(keys), ptxDigestPath)
	return 0
}
