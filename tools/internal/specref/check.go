package specref

import (
	"sort"
	"strconv"
)

// Finding bir denetim bulgusudur.
type Finding struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Check    string `json:"check"`    // "missing", "suffix", "duplicate"
	Severity string `json:"severity"` // "error" ya da "note"
	Ref      string `json:"ref"`      // anılan numara
	Message  string `json:"message"`
}

// Ref bir atıfla hedefinin eşleşmesidir (gözden geçirme listesi).
type Ref struct {
	File   string `json:"file"`
	Line   int    `json:"line"`
	Ref    string `json:"ref"`
	Target string `json:"target"` // hedef maddenin başlığı
	Where  string `json:"where"`  // hedefin dosyası:satırı
}

// Check yok olan maddelere atıfları, çift tanımlı numaraları ve ek ses uyumu hatalarını bulur.
// Açık biçimlerdeki (strong) hatalar "error", spec dosyalarındaki zayıf anmalar "note" sayılır; zayıf
// anma ondalık bir sayı da olabilir.
func Check(ms []Mention, ix *Index) []Finding {
	var out []Finding
	for _, d := range ix.Dups {
		out = append(out, Finding{File: d.File, Line: d.Line, Check: "duplicate", Severity: "error", Ref: d.ID,
			Message: "item " + d.ID + " is defined more than once (first at " + where(ix.Items[d.ID]) + ")"})
	}
	for _, m := range ms {
		if m.Kind == "definition" {
			continue
		}
		sev := "error"
		if m.Kind == "weak" {
			sev = "note"
		}
		for _, t := range m.Tokens {
			if _, ok := ix.Items[t.ID]; !ok {
				out = append(out, Finding{File: m.File, Line: m.Line, Check: "missing", Severity: sev, Ref: t.ID,
					Message: "no item " + t.ID + " in the language definition"})
			}
		}
		if f, ok := suffixFinding(m); ok {
			out = append(out, f)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		return out[i].Line < out[j].Line
	})
	return out
}

// suffixFinding anmanın ekinde ses uyumu hatası varsa bulguyu verir.
func suffixFinding(m Mention) (Finding, bool) {
	if m.Suffix == "" || len(m.Tokens) == 0 {
		return Finding{}, false
	}
	last := m.Tokens[len(m.Tokens)-1].ID
	want, ok := CheckSuffix(last, m.Suffix)
	if ok {
		return Finding{}, false
	}
	msg := "suffix '" + m.Suffix + " after " + last + " breaks vowel harmony; expected '" + want
	if want == "" {
		msg = "suffix '" + m.Suffix + " after " + last + " has a wrong buffer consonant"
	}
	return Finding{File: m.File, Line: m.Line, Check: "suffix", Severity: "error", Ref: last, Message: msg}, true
}

// Refs her açık atıfı hedef maddenin başlığıyla eşler (gözden geçirme için: atıf doğru maddeyi mi
// gösteriyor). Aralıklarda ilk ve son uç listelenir; yok olan hedefler Check'te raporlanır, burada atlanır.
func Refs(ms []Mention, ix *Index) []Ref {
	var out []Ref
	for _, m := range ms {
		if m.Kind != "strong" {
			continue
		}
		for _, t := range m.Tokens {
			if it, ok := ix.Items[t.ID]; ok {
				out = append(out, Ref{File: m.File, Line: m.Line, Ref: t.ID, Target: it.Title, Where: where(it)})
			}
		}
	}
	return out
}

func where(it Item) string { return it.File + ":" + strconv.Itoa(it.Line) }
