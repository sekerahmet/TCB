package interp

import (
	"slices"
	"testing"
)

// Başlatıcıların argümanları programın sayılmaz (1.4): ROS 2'nin --ros-args ile başlayıp -- ile ya
// da sonda biten blokları ve --local-rank=N, --local_rank=N.
func TestProgramArgumentsLeaveLaunchersOut(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   []string
		want []string
	}{
		{"programın argümanları değişmez", []string{"--adım", "5", "veri"}, []string{"--adım", "5", "veri"}},
		{"ROS bloğu -- ile biter", []string{"--ros-args", "-r", "__node:=x", "-p", "a:=1", "--", "veri"}, []string{"veri"}},
		{"ROS bloğu sonda biter", []string{"--adım", "5", "--ros-args", "-r", "__node:=x"}, []string{"--adım", "5"}},
		{"iki ROS bloğu", []string{"--ros-args", "-r", "a:=b", "--", "x", "--ros-args", "-p", "c:=1"}, []string{"x"}},
		{"torch başlatıcısının sırası", []string{"--local-rank=3", "--adım", "5"}, []string{"--adım", "5"}},
		{"deepspeed başlatıcısının sırası", []string{"--local_rank=12"}, nil},
		{"ayrı yazılan sıra programın argümanıdır", []string{"--local-rank", "3"}, []string{"--local-rank", "3"}},
		{"sayı olmayan sıra programın argümanıdır", []string{"--local-rank=x"}, []string{"--local-rank=x"}},
		{"argüman yok", nil, nil},
	} {
		v := New()
		v.SetArgs(tc.in)
		if got := v.Args(); !slices.Equal(got, tc.want) {
			t.Errorf("%s: %q, %q bekleniyordu", tc.name, got, tc.want)
		}
	}
}
