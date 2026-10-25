package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// stampExts içeriği imzaya giren dosya uzantılarıdır: grafiğin içini okuduğu kaynak türleri (Go, ad ve ileti sözlükleri,
// belgeler, TR kaynakları, yüklemeleri okunan JavaScript). go.mod da girer: modül yolu düğüm kimliklerinin parçasıdır.
// Öbür izlenen dosyaların yalnızca varlığı girer (dosya düğümü). Kural: TestStampCoversEverySourceTheGraphReads
// (grafikteki her düğümün ve kenarın okunduğu dosya imzaya girer).
var stampExts = map[string]bool{".go": true, ".json": true, ".md": true, ".tr": true, ".js": true, ".mjs": true, ".cjs": true}

// stamped dosyanın içeriği imzaya girer mi.
func stamped(p string) bool {
	return stampExts[strings.ToLower(filepath.Ext(p))] || filepath.Base(p) == "go.mod"
}

// gitOut root'ta git çalıştırır.
func gitOut(root string, args ...string) ([]byte, error) {
	return gitIn(root, nil, args...)
}

// gitIn gitOut'tur, stdin verilebilir.
func gitIn(root string, stdin io.Reader, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	cmd.Stdin = stdin
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(errb.String()))
	}
	return out.Bytes(), nil
}

// sourceStamp grafiğin okuduğu ağacın içeriğini tek metne indirir: izlenen her dosyanın yolu (dosya düğümü), kaynak
// dosyalarının (stamped) git nesne özeti; değişmiş kaynakta özet çalışma ağacındaki içerikten (`git hash-object`, git'in
// satır sonu süzgeciyle), silinmişte "absent". İzlenmeyen dosyalardan yalnızca Go kaynağı ve go.mod girer (Go yükleyicisi
// klasörün bütün .go dosyalarını okur; öbürlerini grafik okumaz). İmza HEAD'e değil içeriğe bağlıdır: aynı içerikle
// commit, birleştirme ya da dal değişimi grafiği eskitmez (TCB 2026-10-25, 5. madde: her ilk düzenlemede soran kanca bir
// commit'ten sonra aynı ağaç için yeniden kurduruyordu). Aynı metin aynı kaynak demektir; git yoksa ya da başarısızsa hata
// döner.
func sourceStamp(root string) (string, error) {
	top, err := gitOut(root, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	topDir := strings.TrimSpace(string(top))
	index, err := gitOut(root, "ls-files", "-s", "-z", "--full-name", "--", ".")
	if err != nil {
		return "", err
	}
	// Yollar depo kökünden verilir; kök bir alt klasörse yalnızca onun altı taranır.
	status, err := gitOut(root, "status", "--porcelain", "-z", "-uall", "--", ".")
	if err != nil {
		return "", err
	}
	content := map[string]string{} // yol -> içerik özeti (kaynak) ya da "" (yalnızca varlığı)
	for _, rec := range strings.Split(string(index), "\x00") {
		meta, path, ok := strings.Cut(rec, "\t")
		if !ok {
			continue
		}
		content[path] = ""
		if f := strings.Fields(meta); stamped(path) && len(f) >= 2 {
			content[path] = f[1]
		}
	}
	var dirty []string
	for _, e := range changedEntries(status) {
		if e.status == "old" || e.status == "??" && !untrackedSource(e.path) {
			continue // yeniden adlandırmanın eski yolu index'te yoktur; grafiğin okumadığı izlenmeyen dosya
		}
		if _, err := os.Stat(filepath.Join(topDir, filepath.FromSlash(e.path))); err != nil {
			if _, inIndex := content[e.path]; inIndex {
				content[e.path] = "absent" // izlenen ama silinmiş: düğümü hâlâ var (git ls-files)
			}
			continue
		}
		dirty = append(dirty, e.path)
	}
	if len(dirty) > 0 {
		out, err := gitIn(topDir, strings.NewReader(strings.Join(dirty, "\n")+"\n"), "hash-object", "--stdin-paths")
		if err != nil {
			return "", err
		}
		for i, sum := range strings.Fields(string(out)) {
			if i < len(dirty) {
				content[dirty[i]] = sum
			}
		}
	}
	paths := make([]string, 0, len(content))
	for p := range content {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	h := sha256.New()
	for _, p := range paths {
		fmt.Fprintf(h, "%s\x00%s\x00", p, content[p])
	}
	return "content:" + hex.EncodeToString(h.Sum(nil))[:24], nil
}

// untrackedSource izlenmeyen dosyanın grafiğe girip girmediğidir: Go yükleyicisi klasörün bütün .go dosyalarını okur.
func untrackedSource(p string) bool {
	return strings.EqualFold(filepath.Ext(p), ".go") || filepath.Base(p) == "go.mod"
}

type changedEntry struct{ status, path string }

// changedEntries `git status --porcelain -z` çıktısından, grafiği etkileyen uzantılı
// dosyaları yola göre sıralı verir. Yeniden adlandırma satırının ardından eski yol gelir.
func changedEntries(status []byte) []changedEntry {
	parts := strings.Split(string(status), "\x00")
	var out []changedEntry
	for i := 0; i < len(parts); i++ {
		p := parts[i]
		if len(p) < 4 {
			continue
		}
		st, path := p[:2], p[3:]
		if st[0] == 'R' || st[0] == 'C' {
			if i+1 < len(parts) && stamped(parts[i+1]) {
				out = append(out, changedEntry{"old", parts[i+1]})
			}
			i++
		}
		// Grafiğin kendi klasörü (git'e girmese de girse de) imzayı etkilemez.
		if stamped(path) && !strings.HasPrefix(path, ".codegraph/") && !strings.Contains(path, "/.codegraph/") {
			out = append(out, changedEntry{st, path})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].path != out[j].path {
			return out[i].path < out[j].path
		}
		return out[i].status < out[j].status
	})
	return out
}

// refresh grafik eskiyse yeniden kurar. İmzası olmayan grafik (git'siz kurulmuş ya da elle
// yazılmış) denetlenmez. Hata olursa eski grafik kalır ve stderr'e uyarı yazılır.
func refresh(g *Graph, root, graphPath string, stderr io.Writer) *Graph {
	outdated := g.Extractor != extractorRevision // eski sürümün kurduğu grafik: imza aynı olsa da eksik alanlar olabilir
	if g.Stamp == "" && !outdated {
		return g
	}
	now, err := sourceStamp(root)
	if err != nil && outdated {
		now, err = "", nil // git yok: imza karşılaştırılamaz, ama grafik eski sürümden; yine de kur
	}
	if err != nil {
		fmt.Fprintf(stderr, "codegraph: warning: cannot check whether the graph is stale (%v); using the existing graph\n", err)
		return g
	}
	if now == g.Stamp && !outdated {
		return g
	}
	start := time.Now()
	fresh, err := buildGraph(root)
	if err == nil {
		err = writeGraph(graphPath, fresh)
	}
	if err != nil {
		fmt.Fprintf(stderr, "codegraph: warning: rebuild failed (%v); using the existing, stale graph\n", firstLine(err))
		return g
	}
	fmt.Fprintf(stderr, "graph rebuilt (%.1f s)\n", time.Since(start).Seconds())
	return fresh
}

func firstLine(err error) string {
	s := err.Error()
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return s
}

// gitStdout git çıkış kodu sıfır olmasa da stdout'u verir (merge-tree çakışmada 1 döner).
func gitStdout(root string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	return out.Bytes(), err
}
