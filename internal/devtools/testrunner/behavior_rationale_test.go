package testrunner

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

// rationaleTitleRe gerekçe dosyasının ilk satırıdır: "# 2. Sözcük yapısı: emsal ve gerekçe".
var rationaleTitleRe = regexp.MustCompile(`^# (\d+)\. .+: emsal ve gerekçe$`)

// Kural: gözden geçirilmiş (Test satırı biçimine geçmiş) bölümün emsali ve gerekçesi
// docs/spec/rationale/NN_*.md'dedir. Dosya "# N. Ad: emsal ve gerekçe" satırıyla başlar; bölümün her
// kuralı için, tanımdaki sırayla, bir "### N.M" başlığı vardır ve altında "- *Emsal:*" ile
// "- *Gerekçe:*" satırları bulunur; tanımda olmayan bir kuralın başlığı yoktur. Kural eklenince ya da
// silinince gerekçesi de eklenir ya da silinir.
func TestRationaleFilesFollowTheRules(t *testing.T) {
	files, _ := reporoot.Glob(reporoot.Path("docs/spec/rationale/*.md"))
	byNumber := map[string]string{}
	for _, f := range files {
		byNumber[filepath.Base(f)[:2]] = f
	}
	checked := 0
	for name, text := range languageSpec(t) {
		if !reviewedRe.MatchString(text) {
			continue
		}
		no := sectionFileRe.FindStringSubmatch(name)[1]
		path, ok := byNumber[no]
		if !ok {
			t.Errorf("docs/spec/%s gözden geçirilmiş, ama docs/spec/rationale/%s_*.md yok", name, no)
			continue
		}
		rules, _ := sectionTestProblems(name, text, nil)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range rationaleProblems(no, string(data), rules) {
			t.Errorf("docs/spec/rationale/%s: %s", filepath.Base(path), p)
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("gözden geçirilmiş bölüm bulunamadı")
	}
}

// rationaleProblems bir gerekçe dosyasının düzen sorunlarıdır; rules bölümün kurallarıdır, sırayla.
func rationaleProblems(no, text string, rules []string) (problems []string) {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	n, _ := strconv.Atoi(no)
	if m := rationaleTitleRe.FindStringSubmatch(lines[0]); m == nil || m[1] != strconv.Itoa(n) {
		problems = append(problems, fmt.Sprintf("ilk satır \"# %d. Ad: emsal ve gerekçe\" olmalı: %q", n, lines[0]))
	}
	var heads []string
	emsal, gerekce := map[string]bool{}, map[string]bool{}
	cur := ""
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "### "):
			cur = strings.TrimSpace(strings.TrimPrefix(line, "### "))
			heads = append(heads, cur)
		case cur != "" && strings.HasPrefix(line, "- *Emsal:*"):
			emsal[cur] = true
		case cur != "" && strings.HasPrefix(line, "- *Gerekçe:*"):
			gerekce[cur] = true
		}
	}
	if strings.Join(heads, " ") != strings.Join(rules, " ") {
		problems = append(problems, fmt.Sprintf("başlıklar tanımın kurallarıyla aynı sırada olmalı:\n  başlıklar: %v\n  kurallar:  %v", heads, rules))
	}
	for _, h := range heads {
		if !emsal[h] || !gerekce[h] {
			problems = append(problems, fmt.Sprintf("### %s: \"- *Emsal:*\" ve \"- *Gerekçe:*\" satırları olmalı", h))
		}
	}
	return problems
}

// Düzen sorunları yakalanır: eksik ya da sırası bozuk başlık, eksik Emsal ya da Gerekçe, yanlış ilk satır.
func TestRationaleProblems(t *testing.T) {
	good := "# 9. Örnek: emsal ve gerekçe\n\n### 9.1\n\n- *Emsal:* a\n- *Gerekçe:* b\n\n### 9.1a\n\n- *Emsal:* c\n- *Gerekçe:* d\n"
	if p := rationaleProblems("09", good, []string{"9.1", "9.1a"}); len(p) != 0 {
		t.Errorf("düzgün dosyada sorun: %v", p)
	}
	for _, c := range []struct {
		text  string
		rules []string
	}{
		{good, []string{"9.1", "9.1a", "9.2"}},
		{good, []string{"9.1a", "9.1"}},
		{strings.Replace(good, "- *Gerekçe:* d\n", "", 1), []string{"9.1", "9.1a"}},
		{strings.Replace(good, "# 9. Örnek", "# 8. Örnek", 1), []string{"9.1", "9.1a"}},
	} {
		if p := rationaleProblems("09", c.text, c.rules); len(p) != 1 {
			t.Errorf("%v için bir sorun bekleniyordu: %v", c.rules, p)
		}
	}
}
