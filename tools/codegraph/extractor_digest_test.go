package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// notExtractor graph.json'ın içeriğini belirlemeyen, paketin test dışı dosyalarıdır: komut satırı,
// sorgular, git komutları, tazelik ve kök bulma. Geri kalan her test dışı .go dosyası (yükleyici,
// çıkarıcılar, Markdown ayrıştırıcısı, grafiğin kendisi) özete girer; yeni bir çıkarıcı dosyası
// kendiliğinden kapsanır, yeni bir komut dosyası buraya yazılır.
var notExtractor = map[string]bool{
	"main.go": true, "main_query.go": true, "query.go": true, "query_path.go": true, "query_tests.go": true,
	"query_calls.go": true, "query_group.go": true, "query_names.go": true, "query_fields.go": true, "query_suggest.go": true, "history.go": true, "codehist.go": true, "changed.go": true,
	"diffmap.go": true, "branchdecl.go": true, "deleted.go": true, "fresh.go": true, "root.go": true,
	"dups_changed.go": true, "dups_corpus.go": true, "dups_exclude.go": true, "dups_find.go": true, "dups_report.go": true, "dups_similar.go": true,
}

// extractorFiles paketin test dışı .go dosyalarından notExtractor dışındakilerdir (sıralı).
func extractorFiles(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		n := e.Name()
		if strings.HasSuffix(n, ".go") && !strings.HasSuffix(n, "_test.go") && !notExtractor[n] {
			names = append(names, n)
		}
	}
	return names
}

const digestFile = "testdata/extractor_digest.txt"

// extractorDigest çıkarıcı dosyalarının yorumsuz, biçimlendirilmiş sözdizimi ağaçlarının özetidir:
// yorum ve boşluk değişikliği özeti oynatmaz.
func extractorDigest(t *testing.T) string {
	t.Helper()
	h := sha256.New()
	for _, name := range extractorFiles(t) {
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, name, nil, 0) // 0: yorumlar atılır
		if err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		if err := printer.Fprint(&buf, fset, f); err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(h, "%s\n%s\n", name, buf.Bytes())
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// TestExtractorRevisionFollowsExtractorCode çıkarıcı kodu değişince extractorRevision'ın
// (graph.go) bilinçli olarak artırıldığını ya da değişikliğin grafiğin içeriğini
// etkilemediğinin onaylandığını zorlar: özet ile revizyon çifti testdata'da kayıtlıdır.
func TestExtractorRevisionFollowsExtractorCode(t *testing.T) {
	digest := extractorDigest(t)
	want := fmt.Sprintf("revision %d\ndigest %s\n", extractorRevision, digest)
	data, err := os.ReadFile(filepath.FromSlash(digestFile))
	recorded := strings.ReplaceAll(string(data), "\r\n", "\n")
	if err == nil && recorded == want {
		return
	}
	recRev, recDigest := parseDigestFile(recorded)
	switch {
	case err != nil:
		t.Fatalf("%s is missing; create it with:\n%s", digestFile, want)
	case recDigest == digest:
		t.Errorf("extractorRevision is %d but %s records revision %d; update the file to:\n%s", extractorRevision, digestFile, recRev, want)
	case recRev == extractorRevision:
		t.Errorf("the extractor code changed (%v) but extractorRevision is still %d.\n"+
			"If graph.json's content changes (new field, node, edge or different mapping), raise extractorRevision in graph.go\n"+
			"so old graphs are rebuilt; for a pure refactor keep it. Then set %s to:\n%s", extractorFiles(t), extractorRevision, digestFile, wantFor(extractorRevision, digest))
	default:
		t.Errorf("the extractor code changed and extractorRevision was raised (%d -> %d); set %s to:\n%s", recRev, extractorRevision, digestFile, want)
	}
}

func wantFor(rev int, digest string) string {
	return fmt.Sprintf("revision %d\ndigest %s\n", rev, digest)
}

func parseDigestFile(s string) (rev int, digest string) {
	for _, l := range strings.Split(s, "\n") {
		switch {
		case strings.HasPrefix(l, "revision "):
			rev, _ = strconv.Atoi(strings.TrimPrefix(l, "revision "))
		case strings.HasPrefix(l, "digest "):
			digest = strings.TrimPrefix(l, "digest ")
		}
	}
	return rev, digest
}
