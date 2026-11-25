package gpu

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
)

// Kural: benzeticide sınamalar sırasında derlenen her çekirdeğin PTX'inin SHA-256 özeti
// testdata/ptx_ozet.txt'dedir ("özet anahtar", anahtara göre sıralı). Bilinen bir çekirdeğin
// PTX'i değişirse ya da kısa kipte (CI ve tam denetimin kipi) özette olmayan bir çekirdek
// derlenirse paketin testi kalır. Bilinçli bir değişiklikte özet yenilenir (kart gizlenir):
//
//	CUDA_VISIBLE_DEVICES= TR_GPU_PTX_UPDATE=1 go test -short ./internal/compute/gpu
//
// -run ile seçilen alt kümede yalnız derlenenler karşılaştırılır, yenilemede yalnız onlar eklenir ya da
// değişir (silinmez): yeni çekirdeği olan iş bütün paketi koşmadan özetini yeniler; artık derlenmeyen
// anahtarı bütün paketin yenilemesi siler.
//
// Neden: "değişmeyen çekirdeklerin PTX'i aynı" iddiası her GPU işinde elle (geçici dökümle)
// gösteriliyordu; şimdi her koşuda denetlenir ve bilinçli değişiklik diff'te özetin satırı olarak
// görünür. Gerçek GPU'da atlanır (karta göre başka yollar derlenir).

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

// ptxUpdateRefusal TR_GPU_PTX_UPDATE=1'in reddedilme nedenidir ("": yenilenir). Kartlı koşu özeti yenileyemez: karta
// göre başka yollar derlenir.
func ptxUpdateRefusal(sim, short bool) string {
	switch {
	case !sim:
		return "PTX özeti kartlı koşuda yenilenmez; kartı gizleyin: CUDA_VISIBLE_DEVICES="
	case !short:
		return "PTX özeti yalnız -short koşusuyla yenilenir"
	}
	return ""
}

// mergePTXDigest yazılacak özettir: bütün paketin koşusunda görülenler; alt kümede (-run) bilinenlerin üstüne görülenler.
func mergePTXDigest(known, seen map[string]string, subset bool) map[string]string {
	if !subset {
		return seen
	}
	out := maps.Clone(known)
	maps.Copy(out, seen)
	return out
}

// comparePTXDigest PTX'i değişen ve (kısa kipte) özette olmayan anahtarlardır, sıralı.
func comparePTXDigest(known, seen map[string]string, short bool) (changed, added []string) {
	for key, sum := range seen {
		switch want, ok := known[key]; {
		case ok && want != sum:
			changed = append(changed, key)
		case !ok && short:
			added = append(added, key)
		}
	}
	sort.Strings(changed)
	sort.Strings(added)
	return changed, added
}

// checkPTXDigest m.Run'dan sonra çağrılır; özetle uyuşmazlık varsa yazar ve 1 döner.
func checkPTXDigest() int {
	update := os.Getenv("TR_GPU_PTX_UPDATE") == "1"
	if !inSim() && !update {
		return 0
	}
	ptxSeen.Lock()
	seen := ptxSeen.m
	ptxSeen.Unlock()
	known, err := readPTXDigest()
	if err != nil {
		fmt.Println("PTX özeti okunamadı:", err)
		return 1
	}
	if update {
		if why := ptxUpdateRefusal(inSim(), testing.Short()); why != "" {
			fmt.Println(why)
			return 1
		}
		subset := flag.Lookup("test.run") != nil && flag.Lookup("test.run").Value.String() != ""
		return writePTXDigest(mergePTXDigest(known, seen, subset))
	}
	changed, added := comparePTXDigest(known, seen, testing.Short())
	if len(changed)+len(added) == 0 {
		return 0
	}
	fmt.Printf("PTX özeti (%s) tutmuyor: %d çekirdeğin PTX'i değişti, %d yeni çekirdek.\n", ptxDigestPath, len(changed), len(added))
	for _, k := range changed {
		fmt.Println("  değişti:", k)
	}
	for _, k := range added {
		fmt.Println("  yeni:   ", k)
	}
	fmt.Println("Değişiklik bilinçliyse: CUDA_VISIBLE_DEVICES= TR_GPU_PTX_UPDATE=1 go test -short ./internal/compute/gpu")
	return 1
}

// TestPTXDigestMerge özetin kararları: kartlı ya da uzun koşu yenilemez; alt küme yalnız gördüğünü ekler ya da yeniler,
// bütün paket görülmeyeni siler; yeni anahtar kısa kipte alt kümede de hatadır.
func TestPTXDigestMerge(t *testing.T) {
	if ptxUpdateRefusal(true, true) != "" || ptxUpdateRefusal(false, true) == "" || ptxUpdateRefusal(true, false) == "" {
		t.Error("yenileme kapısı: yalnız benzeticide ve -short'ta açıktır")
	}
	known := map[string]string{"a": "1", "b": "2"}
	seen := map[string]string{"b": "3", "c": "4"}
	if got, want := mergePTXDigest(known, seen, true), map[string]string{"a": "1", "b": "3", "c": "4"}; !maps.Equal(got, want) {
		t.Errorf("alt küme: %v, beklenen %v", got, want)
	}
	if got := mergePTXDigest(known, seen, false); !maps.Equal(got, seen) {
		t.Errorf("bütün paket: %v, beklenen %v", got, seen)
	}
	if !maps.Equal(known, map[string]string{"a": "1", "b": "2"}) {
		t.Errorf("birleştirme bilinenleri değiştirdi: %v", known)
	}
	for _, c := range []struct {
		short bool
		added []string
	}{{true, []string{"c"}}, {false, nil}} {
		changed, added := comparePTXDigest(known, seen, c.short)
		if !slices.Equal(changed, []string{"b"}) || !slices.Equal(added, c.added) {
			t.Errorf("short=%v: değişen %v, yeni %v", c.short, changed, added)
		}
	}
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
