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

// stampExts imzaya giren değişmiş dosya uzantılarıdır: grafiği etkileyen kaynak türleri.
var stampExts = map[string]bool{".go": true, ".json": true, ".md": true}

// gitOut root'ta git çalıştırır.
func gitOut(root string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(errb.String()))
	}
	return out.Bytes(), nil
}

// sourceStamp ağacın bugünkü durumunu tek metne indirir: `git rev-parse HEAD` ve izlenen ya da
// izlenmeyen, değişmiş .go/.json/.md dosyalarının (git status --porcelain) yol, durum ve
// içerik özeti. Aynı metin aynı kaynak demektir; git yoksa ya da başarısızsa hata döner.
func sourceStamp(root string) (string, error) {
	head, err := gitOut(root, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	top, err := gitOut(root, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	// Yollar depo kökünden verilir; kök bir alt klasörse yalnızca onun altı taranır.
	status, err := gitOut(root, "status", "--porcelain", "-z", "-uall", "--", ".")
	if err != nil {
		return "", err
	}
	topDir := strings.TrimSpace(string(top))
	h := sha256.New()
	for _, e := range changedEntries(status) {
		fmt.Fprintf(h, "%s\x00%s\x00", e.status, e.path)
		if data, err := os.ReadFile(filepath.Join(topDir, filepath.FromSlash(e.path))); err == nil {
			sum := sha256.Sum256(data)
			h.Write(sum[:])
		} else {
			io.WriteString(h, "absent")
		}
	}
	return strings.TrimSpace(string(head)) + "+" + hex.EncodeToString(h.Sum(nil))[:16], nil
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
			if i+1 < len(parts) && stampExts[strings.ToLower(filepath.Ext(parts[i+1]))] {
				out = append(out, changedEntry{"old", parts[i+1]})
			}
			i++
		}
		// Grafiğin kendi klasörü (git'e girmese de girse de) imzayı etkilemez.
		if stampExts[strings.ToLower(filepath.Ext(path))] && !strings.HasPrefix(path, ".codegraph/") && !strings.Contains(path, "/.codegraph/") {
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
	if g.Stamp == "" {
		return g
	}
	now, err := sourceStamp(root)
	if err != nil {
		fmt.Fprintf(stderr, "codegraph: warning: cannot check whether the graph is stale (%v); using the existing graph\n", err)
		return g
	}
	if now == g.Stamp {
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
