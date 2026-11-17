package main

import (
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"
)

// importLine bir TR dosyasındaki aktarım deyimidir: TR_import "<yol>".
var importLine = regexp.MustCompile(`(?m)^[ \t]*TR_import[ \t]+"([^"]+)"`)

// libraryPrefix dilin kitaplığının aktarım önekidir: "tr/layers" → lib/layers.tr.
const libraryPrefix = "tr/"

// importTarget importer dosyasının yazdığı aktarım yolunun depodaki dosyasıdır: kitaplık modülü
// lib/'de, öbürleri aktaran dosyanın klasörüne göre (yorumlayıcının çözümüyle aynı,
// internal/lang/interp/library.go); .tr uzantısı isteğe bağlıdır.
func importTarget(importer, written string) string {
	p := strings.ReplaceAll(written, `\`, "/")
	if !strings.HasSuffix(p, ".tr") {
		p += ".tr"
	}
	if rest, ok := strings.CutPrefix(p, libraryPrefix); ok {
		return path.Join("lib", rest)
	}
	return path.Join(path.Dir(importer), p)
}

// programImports programın ve özyinelemeli olarak aktardığı bütün dosyaların depo yollarını
// sıralı verir (program dahil). read bir dosyanın metnini verir; okunamayan aktarım hatadır.
func programImports(program string, read func(rel string) ([]byte, error)) ([]string, error) {
	seen := map[string]bool{program: true}
	queue := []string{program}
	for len(queue) > 0 {
		file := queue[0]
		queue = queue[1:]
		data, err := read(file)
		if err != nil {
			return nil, fmt.Errorf(raw("unreadable"), file, err)
		}
		for _, m := range importLine.FindAllStringSubmatch(string(data), -1) {
			target := importTarget(file, m[1])
			if !seen[target] {
				seen[target] = true
				queue = append(queue, target)
			}
		}
	}
	files := make([]string, 0, len(seen))
	for f := range seen {
		files = append(files, f)
	}
	sort.Strings(files)
	return files, nil
}
