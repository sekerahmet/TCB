package ayikla

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"tr_dili/internal/dil/yurut"
)

// ---------------------------------------------------------------------------
// İzleme Masası (özel istek: trDegiskenler)
// ---------------------------------------------------------------------------

// variablesTable tüm değişkenleri tek bir tabloda
// (Kapsam | Değişken | Tip | Değer) döndürür; tablo olarak açılabilen değerler işaretlenir.
func (s *dapServer) variablesTable(msg *dapMessage) {
	list := []interface{}{}
	add := func(kapsam string, h *dapHandle) {
		for _, e := range s.scopeVariables(h) {
			_, _, tblErr := TableData(e.Val)
			list = append(list, map[string]interface{}{
				"kapsam": kapsam, "ad": e.Name, "tip": typeLabel(e.Val), "tur": valueKind(e.Val),
				"deger": display(e.Val), "tablo": tblErr == nil, "ozet": izlemeOzeti(e.Val),
			})
		}
	}
	if s.curEnv != nil && s.curEnv != s.curVM.GlobalEnv {
		add("Yerel", &dapHandle{scope: s.curEnv})
	}
	add("Genel", &dapHandle{scope: s.curVM.GlobalEnv, isGlobal: true})

	frame := "ana program"
	if st := s.curVM.StackSnapshot(); len(st) > 0 {
		frame = st[len(st)-1].FuncName + "()"
	}
	s.respond(msg, map[string]interface{}{
		"degiskenler": list, "satir": s.curLine, "neden": s.curReason,
		"cerceve": frame, "dosya": filepath.Base(s.programPath),
	})
}

// IzlemeDegeri, motor paketlerinin kendi değer türlerini (ör. tensör) İzleme Masası'nda
// göstermek için uyguladığı arayüzdür. ayikla bu paketleri içe aktarmaz; değer
// arayüzü uyguluyorsa tür, tip etiketi ve tablo verisi ondan alınır.
type IzlemeDegeri interface {
	// IzlemeTuru ikon seçimi için türdür (ör. "tensor").
	IzlemeTuru() string
	// IzlemeEtiketi tip sütununda gösterilir (ör. "Tensör [2×3]").
	IzlemeEtiketi() string
	// TabloVerisi değeri tabloya çevirir; tablo olarak açılamıyorsa hata döner.
	TabloVerisi() ([]string, [][]string, error)
}

// valueKind panelde ikon seçmek için değerin türünü verir.
func valueKind(val yurut.Value) string {
	if iz, ok := val.(IzlemeDegeri); ok {
		return iz.IzlemeTuru()
	}
	switch v := val.(type) {
	case *yurut.ListVal:
		if cols, _, err := TableData(v); err == nil && !(len(cols) == 1 && cols[0] == "Değer") {
			return "tablo"
		}
		return "liste"
	case *yurut.DictVal:
		return "sozluk"
	case *yurut.StructInstanceVal:
		return "yapi"
	case *yurut.IntVal, *yurut.FloatVal:
		return "sayi"
	case *yurut.StringVal:
		return "metin"
	case *yurut.BoolVal:
		return "mantik"
	case *yurut.NullVal:
		return "yok"
	}
	return "diger"
}

// typeLabel tip sütununda değerin yapısını özetler (ör. Tablo [3×3]).
func typeLabel(val yurut.Value) string {
	if iz, ok := val.(IzlemeDegeri); ok {
		return iz.IzlemeEtiketi()
	}
	switch v := val.(type) {
	case *yurut.ListVal:
		if cols, rows, err := TableData(v); err == nil && !(len(cols) == 1 && cols[0] == "Değer") {
			return fmt.Sprintf("Tablo [%d×%d]", len(rows), len(cols))
		}
		return fmt.Sprintf("Liste [%d]", len(v.Elements))
	case *yurut.DictVal:
		return fmt.Sprintf("Sözlük [%d]", len(v.Pairs))
	case *yurut.StructInstanceVal:
		return "Yapı " + v.Def.Name
	}
	return string(val.Type())
}

// ---------------------------------------------------------------------------
// Tablo görünümü (özel istek: trTablo)
// ---------------------------------------------------------------------------

func (s *dapServer) table(msg *dapMessage) {
	var a struct {
		Ifade string `json:"ifade"`
	}
	_ = json.Unmarshal(msg.Arguments, &a)
	val, err := s.evalQuiet(s.curVM, s.curEnv, a.Ifade)
	if err != nil {
		s.respondErr(msg, "%v", err)
		return
	}
	cols, rows, err := TableData(val)
	if err != nil {
		s.respondErr(msg, "%v", err)
		return
	}
	s.respond(msg, map[string]interface{}{"baslik": a.Ifade, "sutunlar": cols, "satirlar": rows, "tip": string(val.Type())})
}

// TableData bir değeri tabloya (sütunlar + satırlar) dönüştürür:
// sözlük/yapı listeleri, düz listeler, sözlükler, yapılar ve (IzlemeDegeri ile) tensörler
// desteklenir.
func TableData(val yurut.Value) ([]string, [][]string, error) {
	const limit = 10000
	if iz, ok := val.(IzlemeDegeri); ok {
		return iz.TabloVerisi()
	}
	switch v := val.(type) {
	case *yurut.ListVal:
		records := true
		for _, el := range v.Elements {
			switch el.(type) {
			case *yurut.DictVal, *yurut.StructInstanceVal:
			default:
				records = false
			}
		}
		if records && len(v.Elements) > 0 {
			var cols []string
			seen := map[string]bool{}
			for _, el := range v.Elements {
				for _, c := range children(el, "") {
					if !seen[c.name] {
						seen[c.name] = true
						cols = append(cols, c.name)
					}
				}
			}
			var rows [][]string
			for i, el := range v.Elements {
				if i >= limit {
					break
				}
				cells := map[string]string{}
				for _, c := range children(el, "") {
					cells[c.name] = plainText(c.val)
				}
				row := make([]string, len(cols))
				for j, c := range cols {
					row[j] = cells[c]
				}
				rows = append(rows, row)
			}
			return cols, rows, nil
		}
		var rows [][]string
		for i, el := range v.Elements {
			if i >= limit {
				break
			}
			rows = append(rows, []string{plainText(el)})
		}
		return []string{"Değer"}, rows, nil
	case *yurut.DictVal:
		var rows [][]string
		for _, k := range v.Order {
			rows = append(rows, []string{v.DisplayKey(k), plainText(v.Pairs[k])})
		}
		return []string{"Anahtar", "Değer"}, rows, nil
	case *yurut.StructInstanceVal:
		var rows [][]string
		for _, c := range children(v, "") {
			rows = append(rows, []string{c.name, plainText(c.val)})
		}
		return []string{"Alan", "Değer"}, rows, nil
	}
	return nil, nil, fmt.Errorf("tablo görünümü yalnızca liste, sözlük, yapı ve tensörler için kullanılabilir (bu değer: %s)", val.Type())
}

// plainText hücrede gösterilecek metindir: metinler tırnaksız yazılır.
func plainText(val yurut.Value) string {
	if val == nil {
		return ""
	}
	if s, ok := val.(*yurut.StringVal); ok {
		return s.Val
	}
	return val.Inspect()
}

// trStringLiteral bir metni TR kaynak kodunda güvenle kullanılabilecek değişmeze çevirir.
func trStringLiteral(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "{", `\{`, "}", `\}`, "\n", `\n`, "\t", `\t`)
	return `"` + r.Replace(s) + `"`
}
