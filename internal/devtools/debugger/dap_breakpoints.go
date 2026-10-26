package debugger

import (
	"encoding/json"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/parser"
	"github.com/sekerahmet/TCB/internal/lang/source"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
)

// ---------------------------------------------------------------------------
// Kesme noktaları
// ---------------------------------------------------------------------------

func (s *dapServer) setBreakpoints(msg *dapMessage) {
	var a struct {
		Source struct {
			Path string `json:"path"`
		} `json:"source"`
		Breakpoints []struct {
			Line         int    `json:"line"`
			Condition    string `json:"condition"`
			LogMessage   string `json:"logMessage"`
			HitCondition string `json:"hitCondition"`
		} `json:"breakpoints"`
	}
	_ = json.Unmarshal(msg.Arguments, &a)

	// Kesme noktaları dosya başına tutulur: her istek yalnızca kendi dosyasınınkileri
	// değiştirir. Şimdilik yalnızca hata ayıklanan dosyanınkiler uygulanır ('aktar' ile
	// yüklenen modüllerin satırları ayrı dosyalardadır); launch'tan önce dosya bilinmediği
	// için her istek doğrulanmış sayılır.
	result := []interface{}{}
	points := make(map[int]dapBreakpoint, len(a.Breakpoints))
	s.mu.Lock()
	s.breakpointsArg[fileKey(a.Source.Path)] = points
	sameFile := s.programPath == "" || samePath(a.Source.Path, s.programPath)
	for _, bp := range a.Breakpoints {
		points[bp.Line] = dapBreakpoint{Line: bp.Line, Condition: bp.Condition, LogMessage: bp.LogMessage}
		entry := map[string]interface{}{"line": bp.Line, "verified": sameFile}
		if !sameFile {
			entry["message"] = messages.Text(messages.H8110)
		}
		result = append(result, entry)
	}
	s.mu.Unlock()
	s.respond(msg, map[string]interface{}{"breakpoints": result})
}

// fileKey bir yolu kesme noktası haritasının anahtarına çevirir (mutlak, temiz,
// büyük/küçük harfe duyarsız: samePath ile aynı eşitlik).
func fileKey(fpath string) string {
	if fpath == "" {
		return ""
	}
	fpath = sourcePath(fpath)
	m, err := filepath.Abs(fpath)
	if err != nil {
		m = fpath
	}
	return strings.ToLower(filepath.Clean(m))
}
func samePath(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	pa, _ := filepath.Abs(sourcePath(a))
	pb, _ := filepath.Abs(sourcePath(b))
	return strings.EqualFold(filepath.Clean(pa), filepath.Clean(pb))
}

// uriPrefixRe bir adresin şema önekidir ("tr-kodlu:"). Şema en az iki karakterdir; böylece
// Windows sürücü harfi ("C:") şema sanılmaz (düzenleyicinin kendi ayrımıyla aynı).
var uriPrefixRe = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+\-.]+:`)

// sourcePath düzenleyicinin bildirdiği kaynak yolunu dosya yoluna çevirir. Düzenleyici kodlu
// bir dosyayı Türkçe görünümüyle açtığında kesme noktaları dosya yolundan türetilmiş bir
// adresle gelir ("tr-kodlu:/c%3A/proje/a.tr", ağ paylaşımında "tr-kodlu://sunucu/pay/a.tr");
// adresin yolu dosyanın yoludur. Adres olmayan yol olduğu gibi döner.
func sourcePath(fpath string) string {
	if !uriPrefixRe.MatchString(fpath) {
		return fpath
	}
	u, err := url.Parse(fpath)
	if err != nil || u.Path == "" {
		return fpath
	}
	p := u.Path
	if u.Host != "" {
		p = "//" + u.Host + p
	} else if filepath.Separator == '\\' && len(p) >= 3 && p[0] == '/' && p[2] == ':' {
		p = p[1:] // "/c:/proje/a.tr" → "c:/proje/a.tr"
	}
	return filepath.FromSlash(p)
}

// breakpointHit satırda durulup durulmayacağına karar verir: koşul sağlanmazsa
// durmaz; iz noktası (logMessage) ise mesajı yazar ve durmaz.
func (s *dapServer) breakpointHit(v *interp.VM, line int, env *interp.Environment) bool {
	s.mu.Lock()
	bp, ok := s.breakpointsArg[fileKey(s.programPath)][line]
	s.mu.Unlock()
	if !ok {
		return false
	}
	if bp.Condition != "" {
		val, err := s.evalQuiet(v, env, bp.Condition)
		if err != nil {
			s.output("console", messages.Text(messages.H8111, line, err)+"\n")
			return true
		}
		if !val.IsTruthy() {
			return false
		}
	}
	if bp.LogMessage != "" {
		s.output("console", s.interpolate(v, env, bp.LogMessage)+"\n")
		return false
	}
	return true
}

// interpolate iz noktası mesajındaki {ifade} parçalarını değerleriyle doldurur.
func (s *dapServer) interpolate(v *interp.VM, env *interp.Environment, text string) string {
	var sb strings.Builder
	runes := []rune(text)
	for i := 0; i < len(runes); i++ {
		if runes[i] != '{' {
			sb.WriteRune(runes[i])
			continue
		}
		j := i + 1
		for j < len(runes) && runes[j] != '}' {
			j++
		}
		if j >= len(runes) {
			sb.WriteString(string(runes[i:]))
			break
		}
		expr := string(runes[i+1 : j])
		if val, err := s.evalQuiet(v, env, expr); err == nil {
			sb.WriteString(plainText(val))
		} else {
			sb.WriteString("<" + err.Error() + ">")
		}
		i = j
	}
	return sb.String()
}

// evalQuiet bir ifadeyi debugger'ı tetiklemeden hesaplar.
func (s *dapServer) evalQuiet(v *interp.VM, env *interp.Environment, expr string) (interp.Value, error) {
	p := parser.New(source.TextLexer(expr))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		return nil, p.Errs()[0]
	}
	es, ok := singleExpression(prog)
	if !ok {
		return nil, messages.New(messages.H8112, expr)
	}
	return s.quietly(func() (interp.Value, error) { return v.EvalExpression(es.Expression, env) })
}
func (s *dapServer) quietly(fn func() (interp.Value, error)) (interp.Value, error) {
	if s.dbg == nil {
		return fn()
	}
	return s.dbg.quietly(fn)
}

// dataBreakpointInfo: Variables panelinde "Break on Value Change" (izleme noktası).
func (s *dapServer) dataBreakpointInfo(msg *dapMessage) {
	var a struct {
		VariablesReference int    `json:"variablesReference"`
		Name               string `json:"name"`
	}
	_ = json.Unmarshal(msg.Arguments, &a)
	h := s.handles[a.VariablesReference]
	if h == nil || h.value != nil {
		s.respond(msg, map[string]interface{}{"dataId": nil, "description": messages.Text(messages.H8113)})
		return
	}
	s.respond(msg, map[string]interface{}{
		"dataId": a.Name, "description": messages.Text(messages.H8114, a.Name),
		"accessTypes": []string{"write"}, "canPersist": true,
	})
}
func (s *dapServer) setDataBreakpoints(msg *dapMessage) {
	var a struct {
		Breakpoints []struct {
			DataID string `json:"dataId"`
		} `json:"breakpoints"`
	}
	_ = json.Unmarshal(msg.Arguments, &a)
	if s.dbg == nil {
		s.respond(msg, map[string]interface{}{"breakpoints": []interface{}{}})
		return
	}
	var wps []*Watchpoint
	result := []interface{}{}
	for i, bp := range a.Breakpoints {
		wp := &Watchpoint{ID: i + 1, Expression: bp.DataID}
		if s.curEnv != nil {
			if v, ok := s.curEnv.Get(bp.DataID); ok {
				wp.observe(v)
			}
		}
		wps = append(wps, wp)
		result = append(result, map[string]interface{}{"verified": true})
	}
	s.dbg.Watchpoints = wps
	s.respond(msg, map[string]interface{}{"breakpoints": result})
}
