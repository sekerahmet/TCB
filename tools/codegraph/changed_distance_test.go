package main

import (
	"slices"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/testkit"
)

// Kural (codegraph 7 Dilim B, B1): `changed --go-tests --by-distance` bayraksızın seçimini aynı kümeyle verir, her
// satırın sonunda bant: near ya da far, önce bütün yakınlar. Yakın: değişen dosyanın adaş test dosyasındaki testler,
// doğrudan testler ve en çok 2 adımlık zincirler. Uzak: 3 adım ve ötesi. Commit öncesi denetim yakınları önce koşar;
// alfabetik sırada değişen paketin 0,03 sn'lik testi kipin 120 sn'sine sığmamıştı (Dilim A, commit 1).
func TestChangedGoTestsByDistanceMarksNearAndFar(t *testing.T) {
	root := gitFixture(t)
	testkit.WriteFile(t, root, "chain/chain.go", "package chain\n\n"+
		"func Top() int { return Mid() }\n\nfunc Mid() int { return Low() }\n\nfunc Low() int { return Leaf() }\n\n"+
		"func Leaf() int { return 1 }\n")
	testkit.WriteFile(t, root, "chain/chain_test.go", "package chain\n\nimport \"testing\"\n\n"+
		"func TestChainOwn(t *testing.T) { _ = Top() }\n")
	testkit.WriteFile(t, root, "chain/uses_test.go", "package chain\n\nimport \"testing\"\n\n"+
		"func TestLeafDirect(t *testing.T) { _ = Leaf() }\n\nfunc TestLowTwoSteps(t *testing.T) { _ = Low() }\n\n"+
		"func TestMidThreeSteps(t *testing.T) { _ = Mid() }\n")
	commitAll(t, root, "chain")
	gitMust(t, root, "checkout", "-q", "-b", "claude/leaf", "main")
	replaceIn(t, root+"/chain/chain.go", "func Leaf() int { return 1 }", "func Leaf() int { return 2 }")
	commitAll(t, root, "claude/leaf")
	graph := buildInto(t, root)

	code, plain, errOut := commandIn("changed", root, graph, "--go-tests", "claude/leaf")
	if code != exitOK {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	code, banded, errOut := commandIn("changed", root, graph, "--go-tests", "--by-distance", "claude/leaf")
	if code != exitOK {
		t.Fatalf("--by-distance: exit %d: %s", code, errOut)
	}
	lines := strings.Split(strings.TrimSpace(banded), "\n")
	band := map[string]string{}
	var set []string
	farSeen := false
	for _, line := range lines {
		f := strings.Fields(line)
		if len(f) != 3 || (f[2] != "near" && f[2] != "far") {
			t.Fatalf("each line must be \"folder TestName near|far\": %q\n%s", line, banded)
		}
		if f[2] == "far" {
			farSeen = true
		} else if farSeen {
			t.Errorf("a near test comes after a far one: %q\n%s", line, banded)
		}
		band[f[1]] = f[2]
		set = append(set, f[0]+" "+f[1])
	}
	for name, want := range map[string]string{
		"TestChainOwn": "near", "TestLeafDirect": "near", "TestLowTwoSteps": "near", "TestMidThreeSteps": "far",
	} {
		if band[name] != want {
			t.Errorf("%s: band %q, want %q\n%s", name, band[name], want, banded)
		}
	}
	plainSet := strings.Split(strings.TrimSpace(plain), "\n")
	slices.Sort(set)
	slices.Sort(plainSet)
	if !slices.Equal(set, plainSet) {
		t.Errorf("--by-distance must select the same tests as the plain form:\nplain:\n%s\nbanded:\n%s", plain, banded)
	}
}
