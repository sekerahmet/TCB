package debugger

import (
	"encoding/json"
	"fmt"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"path/filepath"
	"strings"
)

// ---------------------------------------------------------------------------
// İzleme Masası (özel istek: trDegiskenler)
// ---------------------------------------------------------------------------

// variablesTable tüm değişkenleri tek bir tabloda
// (Kapsam | Değişken | Tip | Değer) döndürür; tablo olarak açılabilen değerler işaretlenir.
func (s *dapServer) variablesTable(msg *dapMessage) {
	list := []interface{}{}
	add := func(scope string, h *dapHandle) {
		for _, e := range s.scopeVariables(h) {
			_, _, tblErr := TableData(e.Val)
			list = append(list, map[string]interface{}{
				"kapsam": scope, "ad": e.Name, "tip": typeLabel(e.Val), "tur": valueKind(e.Val),
				"deger": display(e.Val), "tablo": tblErr == nil, "ozet": watchSummary(e.Val),
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

// WatchValue, motor paketlerinin kendi değer türlerini (ör. tensör) İzleme Masası'nda
// göstermek için uyguladığı arayüzdür. ayikla bu paketleri içe aktarmaz; değer
// arayüzü uyguluyorsa tür, tip etiketi ve tablo verisi ondan alınır.
type WatchValue interface {
	// WatchKind ikon seçimi için türdür (ör. "tensor").
	WatchKind() string
	// WatchLabel tip sütununda gösterilir (ör. "Tensör [2×3]").
	WatchLabel() string
	// TableData değeri tabloya çevirir; tablo olarak açılamıyorsa hata döner.
	TableData() ([]string, [][]string, error)
}

// valueKind panelde ikon seçmek için değerin türünü verir.
func valueKind(val interp.Value) string {
	if iz, ok := val.(WatchValue); ok {
		return iz.WatchKind()
	}
	switch v := val.(type) {
	case *interp.ListVal:
		if recordList(v) {
			return "tablo"
		}
		return "liste"
	case *interp.DictVal:
		return "sozluk"
	case *interp.StructInstanceVal:
		return "yapi"
	case *interp.IntVal, *interp.FloatVal:
		return "sayi"
	case *interp.StringVal:
		return "metin"
	case *interp.BoolVal:
		return "mantik"
	case *interp.NullVal:
		return "yok"
	}
	return "diger"
}

// typeLabel tip sütununda değerin yapısını özetler (ör. Tablo [3×3]).
func typeLabel(val interp.Value) string {
	if iz, ok := val.(WatchValue); ok {
		return iz.WatchLabel()
	}
	switch v := val.(type) {
	case *interp.ListVal:
		if recordList(v) {
			cols, rows, _ := TableData(v)
			return messages.Text(messages.H8140, len(rows), len(cols))
		}
		return fmt.Sprintf("%s [%d]", interp.ValList, len(v.Elements))
	case *interp.DictVal:
		return fmt.Sprintf("%s [%d]", interp.ValDict, len(v.Pairs))
	case *interp.StructInstanceVal:
		return messages.Text(messages.H8141, v.Def.Name)
	}
	return string(val.Type())
}

// ---------------------------------------------------------------------------
// Tablo görünümü (özel istek: trTablo)
// ---------------------------------------------------------------------------

func (s *dapServer) table(msg *dapMessage) {
	var a struct {
		Expression string `json:"ifade"`
		Scope      string `json:"kapsam"`
	}
	_ = json.Unmarshal(msg.Arguments, &a)
	// İzleme Masası aynı adlı Yerel/Genel tabloları ayırır: kapsam "Genel" ise ifade genel
	// ortamda hesaplanır; boş ya da "Yerel" ise geçerli ortamda (Variables panelinden gelen
	// kapsamsız istek de buraya düşer).
	env := s.curEnv
	if a.Scope == "Genel" {
		env = s.curVM.GlobalEnv
	}
	val, err := s.evalQuiet(s.curVM, env, a.Expression)
	if err != nil {
		s.respondErr(msg, err.Error())
		return
	}
	cols, rows, err := TableData(val)
	if err != nil {
		s.respondErr(msg, err.Error())
		return
	}
	s.respond(msg, map[string]interface{}{"baslik": a.Expression, "sutunlar": cols, "satirlar": rows, "tip": string(val.Type())})
}

// TableData bir değeri tabloya (sütunlar + satırlar) dönüştürür:
// sözlük/yapı listeleri, düz listeler, sözlükler, yapılar ve (WatchValue ile) tensörler
// desteklenir.
func TableData(val interp.Value) ([]string, [][]string, error) {
	const limit = 10000
	if iz, ok := val.(WatchValue); ok {
		return iz.TableData()
	}
	switch v := val.(type) {
	case *interp.ListVal:
		if recordList(v) {
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
		return []string{messages.Text(messages.H8142)}, rows, nil
	case *interp.DictVal:
		var rows [][]string
		for _, k := range v.Order {
			rows = append(rows, []string{v.DisplayKey(k), plainText(v.Pairs[k])})
		}
		return []string{messages.Text(messages.H8143), messages.Text(messages.H8142)}, rows, nil
	case *interp.StructInstanceVal:
		var rows [][]string
		for _, c := range children(v, "") {
			rows = append(rows, []string{c.name, plainText(c.val)})
		}
		return []string{messages.Text(messages.H8144), messages.Text(messages.H8142)}, rows, nil
	}
	return nil, nil, messages.New(messages.H8145, val.Type())
}

// recordList liste, elemanlarının hepsi sözlük ya da yapı olan (her eleman bir satır, her
// alan bir sütun olan) boş olmayan bir liste mi? Değilse tablo tek sütunlu düz listedir.
func recordList(v *interp.ListVal) bool {
	for _, el := range v.Elements {
		switch el.(type) {
		case *interp.DictVal, *interp.StructInstanceVal:
		default:
			return false
		}
	}
	return len(v.Elements) > 0
}

// plainText hücrede gösterilecek metindir: metinler tırnaksız yazılır.
func plainText(val interp.Value) string {
	if val == nil {
		return ""
	}
	if s, ok := val.(*interp.StringVal); ok {
		return s.Val
	}
	return val.Inspect()
}

// trStringLiteral bir metni TR kaynak kodunda güvenle kullanılabilecek değişmeze çevirir.
func trStringLiteral(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "{", `\{`, "}", `\}`, "\n", `\n`, "\t", `\t`)
	return `"` + r.Replace(s) + `"`
}
