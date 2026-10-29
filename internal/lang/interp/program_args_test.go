package interp

import (
	"slices"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
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

// Argüman verilip komut satırını okumayan programda karar (1.4): yalnız -h ya da yardım seçeneği
// (herhangi bir yazım dilinin adı) "seçeneği yok"tur; başka argüman kullanım hatasıdır; seçenek
// biçiminde argüman yoksa (konumlu) ileti seçenek olarak yazılmasını söyler.
func TestCheckArguments(t *testing.T) {
	prog := mustParseProgram(t, "# TR 4\nTR_print(\"a\")\n")
	for _, tc := range []struct {
		name string
		args []string
		want ArgumentVerdict
	}{
		{"argüman yok", nil, ArgumentsUsable},
		{"-h", []string{"-h"}, ArgumentsNoOptions},
		{"--help", []string{"--help"}, ArgumentsNoOptions},
		{"yardım seçeneğinin başka bir dildeki adı", []string{"--help"}, ArgumentsNoOptions},
		{"-h ve başka argüman", []string{"-h", "--adim", "5"}, ArgumentsUnread},
		{"seçenek", []string{"--adim", "5"}, ArgumentsUnread},
		{"konumlu", []string{"veri.txt"}, ArgumentsUnread},
		{"yalnız başlatıcının argümanı", []string{"--local-rank=2"}, ArgumentsUsable},
	} {
		v := New()
		v.SetArgs(tc.args)
		if got := v.CheckArguments(prog); got != tc.want {
			t.Errorf("%s: karar %v, %v bekleniyordu", tc.name, got, tc.want)
		}
	}
	// İleti: konumlu argümana kendi iletisi, seçeneğe seçenek adları (değerler yazılmaz)
	v := New()
	v.SetArgs([]string{"veri.txt"})
	positional := messages.Text(messages.H8667, 1, codes.NameRef(codes.KwVar), codes.NameRef(codes.FnCommandLine))
	if got := v.ArgumentsMessage(ArgumentsUnread); got != positional {
		t.Errorf("konumlu argümanın iletisi H8667 olmalı: %q", got)
	}
	v = New()
	v.SetArgs([]string{"--adim=gizli_deger", "veri.txt"})
	got := v.ArgumentsMessage(ArgumentsUnread)
	if !strings.Contains(got, "--adim") || strings.Contains(got, "gizli_deger") {
		t.Errorf("ileti seçenek adını yazmalı, değerini yazmamalı: %q", got)
	}
}
