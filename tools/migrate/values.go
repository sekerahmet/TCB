package main

import (
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	trast "github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	trparser "github.com/sekerahmet/TCB/internal/lang/parser"
)

// Değer kuralları (dil tanımı 3. bölüm, katman 2): yeni biçimdeki (# TR 4) metnin çevirisi. Metin
// bugünkü ayrıştırıcıyla okunur; yalnız değişen sözcükler yazılır, öbür her karakter olduğu gibi kalır.
//
//   - sözlük değişmezinin çıplak anahtarı metin anahtarı olur: {model: m} → {"model": m}, kodlu üye
//     anahtarı kimliğin adıyla {TR_step: 1} → {"step": 1} (3.7: anahtar bir ifadedir);
//   - sözlüğe nokta ile erişim köşeli parantez olur: d.ad → d["ad"] (okuma ve atama; 3.7). Yalnız
//     kesin olan çevrilir: ad kullanıcının adıdır (TR_ ile yazılmış üye yerleşik tiplerin üyesi de
//     olabilir), dosyanın, aktardığı yerel modüllerin ve kitaplığın (lib/) hiçbir yapısının alanı ya da
//     metodu değildir ve nesne bir modülün takma adı değildir. Kalan belirsiz okumalar sayım kipiyle
//     bulunur (katman 2'nin yönergesi).
//
// Çevrilmiş metin bir daha çevrilmez: iki kez koşmak bir kez koşmakla aynıdır.

// valueEdit gövdenin karakter aralığının (start, end; sözcük çözümleyicinin kaydı gibi) yerine
// yazılacak metindir.
type valueEdit struct {
	start, end int
	text       string
}

// convertValues yeni biçimdeki bir dosya metnini (başlıklı) ya da başlıksız kodlu gövdeyi değer
// kurallarıyla çevirir; rel dosyanın depodaki yoludur (yerel aktarmaları bulmak için; boş olabilir).
func convertValues(rel, text string) (string, []problem) { return convertValuesWith(rel, text, true) }

// convertValuesWith convertValues'tur; members yanlışsa nokta erişimine dokunulmaz, yalnız sözlük
// değişmezinin anahtarları çevrilir (Go testlerindeki parçalar: nesnenin yapısı parçanın dışında
// tanımlı olabilir, nokta erişimi orada kesin değildir).
func convertValuesWith(rel, text string, members bool) (string, []problem) {
	// İç içe düzenlemelerden bir geçişte yalnız en dıştakiler yapılır (statements.go); metin
	// değişmeyene dek yeniden çevrilir. Sorunlar bir kez bildirilir.
	var probs []problem
	seen := map[problem]bool{}
	for range 16 {
		out, ps := convertValuesOnce(rel, text, members)
		for _, p := range ps {
			if !seen[p] {
				seen[p] = true
				probs = append(probs, p)
			}
		}
		if out == text {
			break
		}
		text = out
	}
	return text, probs
}

// convertValuesOnce convertValuesWith'in bir geçişidir.
func convertValuesOnce(rel, text string, members bool) (string, []problem) {
	prefix, body, lineBase := splitHeader(text)
	edits, probs := valueEdits(rel, body, lineBase, members)
	if len(edits) == 0 {
		return text, probs
	}
	edits = outermost(edits)
	runes := []rune(body)
	var b strings.Builder
	b.WriteString(prefix)
	prev := 0
	for _, e := range edits {
		b.WriteString(string(runes[prev:e.start]))
		b.WriteString(e.text)
		prev = e.end
	}
	b.WriteString(string(runes[prev:]))
	return b.String(), probs
}

// splitHeader metni başlığına (BOM, '#!' satırı ve '# TR 4' satırı) ve gövdesine ayırır; başlıksız
// metnin hepsi gövdedir. lineBase gövdenin ilk satırından önceki satır sayısıdır.
func splitHeader(text string) (prefix, body string, lineBase int) {
	rest := strings.TrimPrefix(text, bom)
	n := len(text) - len(rest)
	for _, start := range []string{"#!", newHeaderPrefix} {
		if !strings.HasPrefix(rest, start) {
			continue
		}
		i := strings.IndexByte(rest, '\n')
		if i < 0 {
			return text, "", lineBase
		}
		n += i + 1
		rest = rest[i+1:]
		lineBase++
	}
	return text[:n], text[n:], lineBase
}

// valueEdits gövdenin değer kurallarıyla değişen aralıklarıdır.
func valueEdits(rel, body string, lineBase int, members bool) ([]valueEdit, []problem) {
	prog, records, ok := parseBody(body)
	if !ok {
		return nil, []problem{{Line: lineBase + 1, Reason: "syntax error: value rules not applied, convert by hand"}}
	}
	at := map[[2]int]lexer.Record{}
	for _, r := range records {
		at[[2]int{r.Line, r.Column}] = r
	}
	withMembers := members
	structNames := structMembers(prog)
	aliases := map[string]bool{}
	trast.Walk(prog, func(n any) bool {
		if s, ok := n.(*trast.ImportStatement); ok {
			if s.Alias != "" {
				aliases[s.Alias] = true
			}
			if !strings.HasPrefix(s.Path, "tr/") && rel != "" {
				for name := range localMembers(path.Join(path.Dir(rel), s.Path)) {
					structNames[name] = true
				}
			}
		}
		return true
	})
	for name := range libraryMembers() {
		structNames[name] = true
	}
	var edits []valueEdit
	var probs []problem
	trast.Walk(prog, func(n any) bool {
		switch e := n.(type) {
		case *trast.DictLiteral:
			for _, p := range e.Pairs {
				key, ok := p.Key.(*trast.StringLiteral)
				if !ok || key.Token.Type != lexer.TokIdent {
					continue
				}
				r, found := at[[2]int{key.Token.Line, key.Token.Column}]
				if !found {
					probs = append(probs, problem{Line: lineBase + key.Token.Line, Reason: "dict key position not found"})
					continue
				}
				edits = append(edits, valueEdit{r.Start, r.End, `"` + keyName(key.Token) + `"`})
			}
		case *trast.MemberExpression:
			if !withMembers || e.NameToken.Code != codes.None || structNames[e.Property] {
				return true
			}
			if id, ok := e.Object.(*trast.Identifier); ok && aliases[id.Value] {
				return true
			}
			dot, okDot := at[[2]int{e.Token.Line, e.Token.Column}]
			name, okName := at[[2]int{e.NameToken.Line, e.NameToken.Column}]
			switch {
			case !okDot || !okName:
				probs = append(probs, problem{Line: lineBase + e.Token.Line, Reason: "member position not found"})
			case e.Safe:
				probs = append(probs, problem{Line: lineBase + e.Token.Line, Reason: "?." + e.Property + " on a dict: write the key read by hand"})
			default:
				edits = append(edits, valueEdit{dot.Start, name.End, `["` + e.Property + `"]`})
			}
		}
		return true
	})
	more, moreProbs := statementEdits(prog, body, records, at, lineBase)
	return append(edits, more...), append(probs, moreProbs...)
}

// keyName çıplak anahtarın metin anahtarındaki adıdır: kodlu üye kimliğin adı (TR_step → step),
// kullanıcının adı olduğu gibi.
func keyName(t lexer.Token) string {
	if t.Code != codes.None {
		return codes.ValueNames().Name(t.Code)
	}
	return t.Literal
}

// parseBody kodlu gövdeyi bugünkü ayrıştırıcıyla okur; ok, sözdizimi hatası olmadığıdır.
func parseBody(body string) (*trast.Program, []lexer.Record, bool) {
	l := lexer.NewBody(body, true)
	l.StartRecording()
	p := trparser.New(l)
	prog := p.ParseProgram()
	if len(p.Errs()) > 0 || len(l.Errs()) > 0 {
		return nil, nil, false
	}
	return prog, l.Records(), true
}

// structMembers programın yapılarının alan ve metot adlarıdır.
func structMembers(prog *trast.Program) map[string]bool {
	out := map[string]bool{}
	trast.Walk(prog, func(n any) bool {
		if s, ok := n.(*trast.StructStatement); ok {
			for _, f := range s.Fields {
				out[f.Name] = true
			}
			for m := range s.Methods {
				out[m] = true
			}
		}
		return true
	})
	return out
}

// repoRoot çevirinin koştuğu deponun köküdür (run kurar); kitaplığın ve yerel modüllerin yapıları
// oradan okunur.
var repoRoot string

// libraryMembers kitaplığın (lib/*.tr) yapılarının alan ve metot adlarıdır (bir kez okunur).
var libraryMembers = sync.OnceValue(func() map[string]bool {
	out := map[string]bool{}
	files, _ := filepath.Glob(filepath.Join(repoRoot, "lib", "*.tr"))
	for _, f := range files {
		for name := range fileMembers(f) {
			out[name] = true
		}
	}
	return out
})

// localMembers depodaki yerel bir modülün (rel, .tr eki yazılmamış olabilir) yapılarının üye adlarıdır.
func localMembers(rel string) map[string]bool {
	if !strings.HasSuffix(rel, ".tr") {
		rel += ".tr"
	}
	return fileMembers(filepath.Join(repoRoot, filepath.FromSlash(rel)))
}

// fileMembers bir .tr dosyasının yapılarının üye adlarıdır; okunamayan dosyada boş.
func fileMembers(file string) map[string]bool {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	_, body, _ := splitHeader(string(data))
	prog, _, ok := parseBody(body)
	if !ok {
		return nil
	}
	return structMembers(prog)
}
