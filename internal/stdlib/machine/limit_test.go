package machine

import (
	"path/filepath"
	"testing"
)

// TestMachineMemoryLimitCgroup cgroup v1 ve v2'nin, kabın, kök cgroup'un, karma düzenin ve
// okunamayan sınırın okunuşunu testdata dosyalarıyla sınar (19.12).
func TestMachineMemoryLimitCgroup(t *testing.T) {
	cases := []struct {
		dir     string
		bytes   int64
		limited bool
		fails   bool
	}{
		{"v2_limited", 8 << 30, true, false},
		{"v2_max", 0, false, false},
		{"v2_container", 12 << 30, true, false},
		{"v2_root", 0, false, false},
		{"v2_missing", 0, false, true},
		{"v1_limited", 4 << 30, true, false},
		{"v1_unlimited", 0, false, false},
		{"v1_docker", 2 << 30, true, false},
		{"hybrid", 3 << 30, true, false},
		// Colab'ın G4 kabının düzeni (ölçüldü): süreç ad alanının dışında, ikinci bağlama gösterir.
		{"v2_colab", 56371445760, true, false},
		{"v2_escaped", 0, false, true},
		{"none", 0, false, true},
		{"does_not_exist", 0, false, true},
	}
	for _, c := range cases {
		b, l, err := cgroupMemoryLimit(filepath.Join("testdata", c.dir))
		if (err != nil) != c.fails || b != c.bytes || l != c.limited {
			t.Errorf("%s: %d, %v, %v; beklenen %d, %v, hata=%v", c.dir, b, l, err, c.bytes, c.limited, c.fails)
		}
	}
}

// TestUnescapeMount mountinfo'nun sekizlik kaçışları (boşluk \040, ters bölü \134) açılır;
// bozuk kaçış olduğu gibi kalır.
func TestUnescapeMount(t *testing.T) {
	for in, want := range map[string]string{
		`/sys/fs/cgroup`: "/sys/fs/cgroup",
		`/mnt/a\040b`:    "/mnt/a b",
		`/mnt/a\134b`:    `/mnt/a\b`,
		`/mnt/a\04`:      `/mnt/a\04`,
		`/mnt/a\999`:     `/mnt/a\999`,
		`\040`:           " ",
	} {
		if got := unescapeMount(in); got != want {
			t.Errorf("unescapeMount(%q) = %q, beklenen %q", in, got, want)
		}
	}
}

// TestJobMemoryLimit Windows'un iş nesnesi bayraklarının okunuşunu sahte değerlerle sınar.
func TestJobMemoryLimit(t *testing.T) {
	cases := []struct {
		flags        uint32
		process, job uint64
		want         int64
		limited      bool
	}{
		{0, 0, 0, 0, false},
		{0, 1 << 30, 2 << 30, 0, false}, // bayrak yoksa değer okunmaz
		{0x100, 1 << 30, 0, 1 << 30, true},
		{0x200, 0, 2 << 30, 2 << 30, true},
		{0x300, 3 << 30, 2 << 30, 2 << 30, true},
		{0x300, 1 << 30, 2 << 30, 1 << 30, true},
		{0x2000 | 0x100, 5 << 20, 0, 5 << 20, true}, // başka bayraklar karışmaz
		{0x100, 1<<64 - 1, 0, 1<<63 - 1, true},
	}
	for _, c := range cases {
		got, l := jobMemoryLimit(c.flags, c.process, c.job)
		if got != c.want || l != c.limited {
			t.Errorf("jobMemoryLimit(%#x, %d, %d) = %d, %v; beklenen %d, %v", c.flags, c.process, c.job, got, l, c.want, c.limited)
		}
	}
}
