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
	v1Mount, v1Root, v2Mount, v2Root := cgroupMounts(string(mi))
	switch {
	case v1Path != "" && v1Mount != "":
		rel, ok := relativeTo(v1Path, v1Root)
		if !ok {
			return 0, false, errNoCgroup
		}
		b, err := os.ReadFile(filepath.Join(root, v1Mount, rel, "memory.limit_in_bytes"))
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
	case v2Path != "" && v2Mount != "":
		rel, ok := relativeTo(v2Path, v2Root)
		if !ok {
			return 0, false, errNoCgroup
		}
		b, err := os.ReadFile(filepath.Join(root, v2Mount, rel, "memory.max"))
		if errors.Is(err, os.ErrNotExist) && rel == "/" {
			return 0, false, nil // kök cgroup'un memory.max'ı yoktur: sınır yok
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

// cgroupMounts /proc/self/mountinfo'dan v1'in bellek denetleyicisinin ve v2'nin bağlama
// noktasını ve bağlanan kökünü verir. Satır: "kimlik üst ana:alt kök nokta seçenekler ... -
// tür kaynak üstseçenekler"; boşluk ve ters bölü sekizlik kaçışla yazılır (\040).
func cgroupMounts(text string) (v1Mount, v1Root, v2Mount, v2Root string) {
	for _, line := range strings.Split(text, "\n") {
		pre, post, ok := strings.Cut(line, " - ")
		if !ok {
			continue
		}
		f, g := strings.Fields(pre), strings.Fields(post)
		if len(f) < 5 || len(g) < 1 {
			continue
		}
		switch g[0] {
		case "cgroup2":
			if v2Mount == "" {
				v2Mount, v2Root = unescapeMount(f[4]), unescapeMount(f[3])
			}
		case "cgroup":
			if len(g) >= 3 && hasOption(g[2], "memory") && v1Mount == "" {
				v1Mount, v1Root = unescapeMount(f[4]), unescapeMount(f[3])
			}
		}
	}
	return v1Mount, v1Root, v2Mount, v2Root
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
