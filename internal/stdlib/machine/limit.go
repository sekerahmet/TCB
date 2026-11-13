package machine

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Bellek sınırı (19.12, .TR_memory_limit_bytes): Linux'ta cgroup'un sınırı, Windows'ta iş
// nesnesinin sınırı. Okuma işletim sistemine özgü dosyalardadır (limit_*.go); ayrıştırma
// burada, her işletim sisteminde sınanabilsin diye.

// unlimitedV1 cgroup v1'in "sınır yok" değeri bu eşiğin üstündedir (çekirdek LONG_MAX'ı
// sayfaya yuvarlar: 9223372036854771712).
const unlimitedV1 = 1 << 62

var errNoCgroup = errors.New("cgroup memory controller not found")

// cgroupMemoryLimit kökü root olan dosya sisteminde (gerçek okumada "/", testte testdata)
// sürecin cgroup bellek sınırını okur: /proc/self/cgroup sürecin yolunu, /proc/self/mountinfo
// cgroup dosya sisteminin bağlandığı yeri verir. v1'in bellek denetleyicisi varsa o (karma
// düzende de), yoksa v2 okunur. limited false ise sınır yoktur.
func cgroupMemoryLimit(root string) (bytes int64, limited bool, err error) {
	cg, err := os.ReadFile(filepath.Join(root, "proc", "self", "cgroup"))
	if err != nil {
		return 0, false, err
	}
	mi, err := os.ReadFile(filepath.Join(root, "proc", "self", "mountinfo"))
	if err != nil {
		return 0, false, err
	}
	v1Path, v2Path := cgroupPaths(string(cg))
	mounts := cgroupMounts(string(mi))
	v1Mount, v1Rel, v1ok := pickMount(mounts, false, v1Path)
	v2Mount, v2Rel, v2ok := pickMount(mounts, true, v2Path)
	switch {
	case v1ok:
		b, err := os.ReadFile(filepath.Join(root, v1Mount, v1Rel, "memory.limit_in_bytes"))
		if err != nil {
			return 0, false, err
		}
		n, err := strconv.ParseUint(strings.TrimSpace(string(b)), 10, 64)
		if err != nil {
			return 0, false, err
		}
		if n > unlimitedV1 {
			return 0, false, nil
		}
		return int64(n), true, nil
	case v2ok:
		b, err := os.ReadFile(filepath.Join(root, v2Mount, v2Rel, "memory.max"))
		if errors.Is(err, os.ErrNotExist) && v2Path == "/" {
			return 0, false, nil // sürecin ad alanındaki kök cgroup'un memory.max'ı yoktur: sınır yok
		}
		if err != nil {
			return 0, false, err
		}
		s := strings.TrimSpace(string(b))
		if s == "max" {
			return 0, false, nil
		}
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil || n < 0 {
			return 0, false, errors.New("memory.max: " + s)
		}
		return n, true, nil
	}
	return 0, false, errNoCgroup
}

// cgroupPaths /proc/self/cgroup'un satırlarından ("kimlik:denetleyiciler:yol") v1'in bellek
// denetleyicisinin ve v2'nin ("0::yol") yolunu verir.
func cgroupPaths(text string) (v1, v2 string) {
	for _, line := range strings.Split(text, "\n") {
		parts := strings.SplitN(strings.TrimSpace(line), ":", 3)
		if len(parts) != 3 {
			continue
		}
		if parts[0] == "0" && parts[1] == "" {
			v2 = parts[2]
			continue
		}
		for _, c := range strings.Split(parts[1], ",") {
			if c == "memory" {
				v1 = parts[2]
			}
		}
	}
	return v1, v2
}

// cgroupMount bir cgroup dosya sisteminin bağlama noktası ve bağlanan köküdür.
type cgroupMount struct {
	point, root string
	v2          bool
}

// cgroupMounts /proc/self/mountinfo'dan v2'nin ve v1'in bellek denetleyicisinin bağlamalarını
// verir. Satır: "kimlik üst ana:alt kök nokta seçenekler ... - tür kaynak üstseçenekler"; boşluk
// ve ters bölü sekizlik kaçışla yazılır (\040). Aynı hiyerarşi birden çok yere bağlanabilir
// (Colab: /sys/fs/cgroup ve sürecin cgroup'u /var/colab/cgroup/... altında).
func cgroupMounts(text string) []cgroupMount {
	var out []cgroupMount
	for _, line := range strings.Split(text, "\n") {
		pre, post, ok := strings.Cut(line, " - ")
		if !ok {
			continue
		}
		f, g := strings.Fields(pre), strings.Fields(post)
		if len(f) < 5 || len(g) < 1 {
			continue
		}
		m := cgroupMount{point: unescapeMount(f[4]), root: unescapeMount(f[3])}
		switch {
		case g[0] == "cgroup2":
			m.v2 = true
			out = append(out, m)
		case g[0] == "cgroup" && len(g) >= 3 && hasOption(g[2], "memory"):
			out = append(out, m)
		}
	}
	return out
}

// pickMount sürecin cgroup yolunu (path) gösteren bağlamayı seçer: kökü yolu içeren
// bağlamalardan kökü en uzun olanı. Yol bağlamanın kökünün dışındaysa (cgroup ad alanının
// dışında, "/../.." ile başlayan yol) o bağlama sürecin cgroup'unu göstermez.
func pickMount(mounts []cgroupMount, v2 bool, path string) (point, rel string, ok bool) {
	if path == "" {
		return "", "", false
	}
	best := -1
	for _, m := range mounts {
		if m.v2 != v2 {
			continue
		}
		r, in := relativeTo(path, m.root)
		if !in || r == "/.." || strings.HasPrefix(r, "/../") {
			continue
		}
		if best < 0 || len(m.root) > best {
			point, rel, ok, best = m.point, r, true, len(m.root)
		}
	}
	return point, rel, ok
}

func hasOption(opts, name string) bool {
	for _, o := range strings.Split(opts, ",") {
		if o == name {
			return true
		}
	}
	return false
}

// unescapeMount mountinfo'nun sekizlik kaçışlarını (\040 boşluk, \011, \012, \134) açar.
func unescapeMount(s string) string {
	if !strings.Contains(s, "\\") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+4 <= len(s) {
			if n, err := strconv.ParseUint(s[i+1:i+4], 8, 8); err == nil {
				b.WriteByte(byte(n))
				i += 3
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// relativeTo sürecin cgroup yolunu bağlanan köke göre verir; yol kökün altında değilse
// (sürecin cgroup'u bu bağlamada görünmüyorsa) ok yanlıştır.
func relativeTo(path, mountRoot string) (string, bool) {
	if mountRoot == "/" {
		return path, true
	}
	if path == mountRoot {
		return "/", true
	}
	if rest, ok := strings.CutPrefix(path, mountRoot+"/"); ok {
		return "/" + rest, true
	}
	return "", false
}

// jobMemoryLimit Windows'un iş nesnesinin bellek sınırıdır: JOB_OBJECT_LIMIT_PROCESS_MEMORY
// (0x100) ya da JOB_OBJECT_LIMIT_JOB_MEMORY (0x200) bayrağı varsa sınırların küçüğü; bayrak
// yoksa sınır yoktur.
func jobMemoryLimit(flags uint32, processLimit, jobLimit uint64) (int64, bool) {
	const processMemory, jobMemory = 0x100, 0x200
	var lim uint64
	set := false
	if flags&processMemory != 0 && processLimit > 0 {
		lim, set = processLimit, true
	}
	if flags&jobMemory != 0 && jobLimit > 0 && (!set || jobLimit < lim) {
		lim, set = jobLimit, true
	}
	if !set {
		return 0, false
	}
	if lim > 1<<63-1 {
		lim = 1<<63 - 1
	}
	return int64(lim), true
}
