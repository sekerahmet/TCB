package ayikla

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"tr_dili/internal/dil/agac"
	"tr_dili/internal/dil/ayristir"
	"tr_dili/internal/dil/sozcuk"
	"tr_dili/internal/dil/yurut"
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
	noktalar := make(map[int]dapBreakpoint, len(a.Breakpoints))
	s.mu.Lock()
	s.kesmeNoktalari[dosyaAnahtari(a.Source.Path)] = noktalar
	sameFile := s.programPath == "" || samePath(a.Source.Path, s.programPath)
	for _, bp := range a.Breakpoints {
		noktalar[bp.Line] = dapBreakpoint{Line: bp.Line, Condition: bp.Condition, LogMessage: bp.LogMessage}
		entry := map[string]interface{}{"line": bp.Line, "verified": sameFile}
		if !sameFile {
			entry["message"] = "Yalnızca hata ayıklanan dosyada kesme noktası konabilir"
		}
		result = append(result, entry)
	}
	s.mu.Unlock()
	s.respond(msg, map[string]interface{}{"breakpoints": result})
}

// dosyaAnahtari bir yolu kesme noktası haritasının anahtarına çevirir (mutlak, temiz,
// büyük/küçük harfe duyarsız: samePath ile aynı eşitlik).
func dosyaAnahtari(yol string) string {
	if yol == "" {
		return ""
	}
	yol = kaynakYolu(yol)
	m, err := filepath.Abs(yol)
	if err != nil {
		m = yol
	}
	return strings.ToLower(filepath.Clean(m))
}
func samePath(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	pa, _ := filepath.Abs(kaynakYolu(a))
	pb, _ := filepath.Abs(kaynakYolu(b))
	return strings.EqualFold(filepath.Clean(pa), filepath.Clean(pb))
}

// adresOnekiRe bir adresin şema önekidir ("tr-kodlu:"). Şema en az iki karakterdir; böylece
// Windows sürücü harfi ("C:") şema sanılmaz (düzenleyicinin kendi ayrımıyla aynı).
var adresOnekiRe = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+\-.]+:`)

// kaynakYolu düzenleyicinin bildirdiği kaynak yolunu dosya yoluna çevirir. Düzenleyici kodlu
// bir dosyayı Türkçe görünümüyle açtığında kesme noktaları dosya yolundan türetilmiş bir
// adresle gelir ("tr-kodlu:/c%3A/proje/a.tr", ağ paylaşımında "tr-kodlu://sunucu/pay/a.tr");
// adresin yolu dosyanın yoludur. Adres olmayan yol olduğu gibi döner.
func kaynakYolu(yol string) string {
	if !adresOnekiRe.MatchString(yol) {
		return yol
	}
	u, err := url.Parse(yol)
	if err != nil || u.Path == "" {
		return yol
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
func (s *dapServer) breakpointHit(v *yurut.VM, line int, env *yurut.Environment) bool {
	s.mu.Lock()
	bp, ok := s.kesmeNoktalari[dosyaAnahtari(s.programPath)][line]
	s.mu.Unlock()
	if !ok {
		return false
	}
	if bp.Condition != "" {
		val, err := s.evalQuiet(v, env, bp.Condition)
		if err != nil {
			s.output("console", fmt.Sprintf("Kesme noktası koşulu hatalı (satır %d): %v\n", line, err))
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
func (s *dapServer) interpolate(v *yurut.VM, env *yurut.Environment, text string) string {
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
func (s *dapServer) evalQuiet(v *yurut.VM, env *yurut.Environment, expr string) (yurut.Value, error) {
	p := ayristir.New(sozcuk.New(expr))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		return nil, errors.New(p.Errors()[0])
	}
	if len(prog.Statements) != 1 {
		return nil, fmt.Errorf("tek bir ifade bekleniyordu: %s", expr)
	}
	es, ok := prog.Statements[0].(*agac.ExpressionStatement)
	if !ok {
		return nil, fmt.Errorf("tek bir ifade bekleniyordu: %s", expr)
	}
	return s.quietly(func() (yurut.Value, error) { return v.EvalExpression(es.Expression, env) })
}
func (s *dapServer) quietly(fn func() (yurut.Value, error)) (yurut.Value, error) {
	if s.dbg == nil {
		return fn()
	}
	was := s.dbg.Active
	s.dbg.Active = false
	defer func() { s.dbg.Active = was }()
	return fn()
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
		s.respond(msg, map[string]interface{}{"dataId": nil, "description": "İzleme noktası yalnızca değişkenlere konabilir"})
		return
	}
	s.respond(msg, map[string]interface{}{
		"dataId": a.Name, "description": "'" + a.Name + "' değiştiğinde dur",
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
				wp.LastVal = v.Inspect()
			}
		}
		wps = append(wps, wp)
		result = append(result, map[string]interface{}{"verified": true})
	}
	s.dbg.Watchpoints = wps
	s.respond(msg, map[string]interface{}{"breakpoints": result})
}
