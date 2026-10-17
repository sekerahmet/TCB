package vm

import (
	"bufio"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"tr_dili/pkg/ast"
	"tr_dili/pkg/lexer"
	"tr_dili/pkg/parser"
	"tr_dili/pkg/turkce"
)

type StepMode int

const (
	StepInto     StepMode = iota // F5: Tek Adım (Fonksiyon içine girer)
	StepOver                     // F6: Üstten Geç (Fonksiyonu tek adımda geçer)
	StepOut                      // F7: Fonksiyondan Çık
	StepContinue                 // F8: Kesme noktasına / Watchpoint'e kadar devam et
)

type StackFrame struct {
	FuncName string
	Line     int
}

type Watchpoint struct {
	ID         int
	Expression string // Değişken adı veya koşul ifadesi (örn: "bakiye" veya "bakiye < 0")
	IsCond     bool
	LastVal    string
}

type Debugger struct {
	SourceFile   string
	SourceLines  []string
	Mode         StepMode
	TargetDepth  int
	Breakpoints  map[int]bool
	Watchpoints  []*Watchpoint
	Reader       *bufio.Reader
	Active       bool
	PauseReason  string
	StepCount    int

	// Görsel Web Arayüzü (ABAP Web Studio) kanalları
	// mu: web arayüzünün HTTP goroutine'i ile VM arasında paylaşılan alanları korur
	// (Breakpoints, Mode, PauseReason, StepCount, cur*, paused).
	mu        sync.Mutex
	paused    bool
	WebMode   bool
	WebPort   int
	webToken  string
	webServer *http.Server
	webWaitCh chan webDebugAction
	curEnv    *Environment
	curLine   int
	curVM     *VM
}

type webDebugAction struct {
	Action string `json:"action"` // "f5", "f6", "f7", "f8", "jump" ("modify" yalnızca sunucu içinde)
	Line   int    `json:"line"`
	Name   string `json:"name"`
	Value  string `json:"value"`

	reply chan error // "modify" sonucunu HTTP işleyicisine iletir
}

func NewDebugger(sourceFile string, sourceCode string, startPaused bool) *Debugger {
	clean := strings.ReplaceAll(sourceCode, "\r\n", "\n")
	lines := strings.Split(clean, "\n")
	mode := StepContinue
	if startPaused {
		mode = StepInto
	}
	return &Debugger{
		SourceFile:  sourceFile,
		SourceLines: lines,
		Mode:        mode,
		Breakpoints: make(map[int]bool),
		Watchpoints: []*Watchpoint{},
		Reader:      bufio.NewReader(os.Stdin),
		Active:      true,
		PauseReason: "Başlangıç (/h Hata Ayıklama Modu)",
	}
}

// BeforeStatement her deyim çalışmadan önce VM tarafından çağrılır.
// Eğer kullanıcı 'zıpla <satır>' (Goto Statement) yaptıysa hedef satırı (>0) döndürür.
func (d *Debugger) BeforeStatement(v *VM, stmt ast.Statement, env *Environment) int {
	if !d.Active || stmt == nil {
		return 0
	}
	// Blok deyiminin kendisinde değil içindeki satırlarda dur
	if _, isBlock := stmt.(*ast.BlockStatement); isBlock {
		return 0
	}

	line := stmt.Line()
	depth := v.StackDepth()
	d.mu.Lock()
	d.StepCount++
	d.mu.Unlock()

	shouldPause := false

	// 1. Watchpoint (İzleme Noktası) kontrolü
	wpReason := d.checkWatchpoints(v, env)

	d.mu.Lock()
	if wpReason != "" {
		shouldPause = true
		d.PauseReason = wpReason
	}

	// 2. Satır kesme noktası (Breakpoint) kontrolü
	if !shouldPause && d.Breakpoints[line] {
		shouldPause = true
		d.PauseReason = fmt.Sprintf("Kesme Noktası (Satır %d)", line)
	}

	// 3. Adım modu (F5 / F6 / F7) kontrolü
	if !shouldPause {
		switch d.Mode {
		case StepInto:
			shouldPause = true
			if d.PauseReason == "" {
				d.PauseReason = "F5 Tek Adım"
			}
		case StepOver:
			if depth <= d.TargetDepth {
				shouldPause = true
				d.PauseReason = "F6 Üstten Adım"
			}
		case StepOut:
			if depth < d.TargetDepth {
				shouldPause = true
				d.PauseReason = "F7 Fonksiyondan Çıkış"
			}
		}
	}

	d.mu.Unlock()

	if !shouldPause {
		return 0
	}

	if d.WebMode {
		return d.waitForWebAction(v, line, env)
	}
	return d.promptCLI(v, line, env)
}

// toggleBreakpoint satırdaki kesme noktasını açar/kapatır; yeni durum açık ise true döner.
func (d *Debugger) toggleBreakpoint(line int) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.Breakpoints[line] {
		delete(d.Breakpoints, line)
		return false
	}
	d.Breakpoints[line] = true
	return true
}

func (d *Debugger) hasBreakpoint(line int) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.Breakpoints[line]
}

func (d *Debugger) checkWatchpoints(v *VM, env *Environment) string {
	for _, wp := range d.Watchpoints {
		if wp.IsCond {
			l := lexer.New(wp.Expression)
			p := parser.New(l)
			prog := p.ParseProgram()
			if len(p.Errors()) == 0 && len(prog.Statements) > 0 {
				if es, ok := prog.Statements[0].(*ast.ExpressionStatement); ok {
					val, err := v.evalExpression(es.Expression, env)
					if err == nil && val.IsTruthy() {
						curStr := "doğru"
						if wp.LastVal != curStr {
							wp.LastVal = curStr
							return fmt.Sprintf("⚡ İZLEME NOKTASI (Watchpoint #%d): '%s' koşulu sağlandı!", wp.ID, wp.Expression)
						}
					} else {
						wp.LastVal = "yanlış"
					}
				}
			}
		} else {
			if val, ok := env.Get(wp.Expression); ok {
				curStr := val.Inspect()
				if wp.LastVal != "" && wp.LastVal != curStr {
					old := wp.LastVal
					wp.LastVal = curStr
					return fmt.Sprintf("⚡ İZLEME NOKTASI (Watchpoint #%d): %q değişti (%s -> %s)", wp.ID, wp.Expression, old, curStr)
				}
				wp.LastVal = curStr
			}
		}
	}
	return ""
}

func (d *Debugger) promptCLI(v *VM, line int, env *Environment) int {
	d.renderABAPScreen(v, line, env)
	d.PauseReason = ""

	for {
		fmt.Print("\n[ABAP-TR Debug] (F5:adım | F6:geç | F7:dön | F8:devam | değiştir | zıpla | tablo | izle) > ")
		input, err := d.Reader.ReadString('\n')
		if err != nil {
			d.Mode = StepContinue
			d.Active = false
			return 0
		}
		cmdLine := strings.TrimSpace(strings.TrimPrefix(input, "\xef\xbb\xbf"))

		// Boş Enter -> F5 (Tek Adım)
		if cmdLine == "" {
			d.Mode = StepInto
			return 0
		}

		parts := strings.Fields(cmdLine)
		cmd := strings.ToLower(parts[0])

		switch cmd {
		case "f5", "adım", "adim", "a", "s", "step":
			d.Mode = StepInto
			return 0

		case "f6", "geç", "gec", "g", "n", "next":
			d.Mode = StepOver
			d.TargetDepth = v.StackDepth()
			return 0

		case "f7", "dön", "don", "o", "out":
			d.Mode = StepOut
			d.TargetDepth = v.StackDepth()
			return 0

		case "f8", "devam", "d", "c", "continue":
			d.Mode = StepContinue
			return 0

		case "değiştir", "degistir", "ata", "set", "kalem":
			// değiştir bakiye = 5000
			rest := strings.TrimSpace(cmdLine[len(parts[0]):])
			if !strings.Contains(rest, "=") {
				fmt.Println("  ⚠️ Kullanım: değiştir <değişken> = <yeni_değer>  (Örn: değiştir x = 99)")
				continue
			}
			eqParts := strings.SplitN(rest, "=", 2)
			varName := strings.TrimSpace(eqParts[0])
			exprStr := strings.TrimSpace(eqParts[1])
			l := lexer.New(exprStr)
			p := parser.New(l)
			prog := p.ParseProgram()
			if len(p.Errors()) > 0 || len(prog.Statements) == 0 {
				fmt.Println("  ❌ Geçersiz değer ifadesi.")
				continue
			}
			if es, ok := prog.Statements[0].(*ast.ExpressionStatement); ok {
				newVal, err := v.evalExpression(es.Expression, env)
				if err != nil {
					fmt.Printf("  ❌ Hesaplama hatası: %v\n", err)
					continue
				}
				if _, exists := env.Get(varName); exists {
					_ = env.Assign(varName, newVal)
				} else {
					env.Define(varName, newVal, false)
				}
				fmt.Printf("  ✏️ [CANLI DEĞİŞTİRİLDİ] %s = %s (%s)\n", varName, newVal.Inspect(), newVal.Type())
			}

		case "zıpla", "zipla", "git", "goto", "j":
			if len(parts) < 2 {
				fmt.Println("  ⚠️ Kullanım: zıpla <satır_no>  (Örn: zıpla 15)")
				continue
			}
			targetLine, err := strconv.Atoi(parts[1])
			if err != nil || targetLine < 1 || targetLine > len(d.SourceLines) {
				fmt.Printf("  ❌ Geçersiz satır numarası (1-%d arası olmalı)\n", len(d.SourceLines))
				continue
			}
			fmt.Printf("  ⏭️ [SATIRA ZIPLANDI] Akış %d. satıra taşınıyor...\n", targetLine)
			d.Mode = StepInto
			d.PauseReason = fmt.Sprintf("Satır %s zıplandı (Shift+F12)", turkce.SayiEk(int64(targetLine), turkce.Yonelme))
			return targetLine

		case "kesme", "k", "b", "break":
			if len(parts) < 2 {
				fmt.Println("  ⚠️ Kullanım: kesme <satır_no>  (Örn: kesme 20)")
				continue
			}
			bpLine, err := strconv.Atoi(parts[1])
			if err != nil || bpLine < 1 || bpLine > len(d.SourceLines) {
				fmt.Println("  ❌ Geçersiz satır numarası.")
				continue
			}
			if !d.toggleBreakpoint(bpLine) {
				fmt.Printf("  ⚪ %d. satırdaki kesme noktası kaldırıldı.\n", bpLine)
			} else {
				fmt.Printf("  🔴 %d. satıra kesme noktası (Breakpoint) eklendi.\n", bpLine)
			}

		case "izle", "watch", "w":
			if len(parts) < 2 {
				fmt.Println("  ⚠️ Kullanım: izle <değişken_veya_koşul>  (Örn: izle toplam  veya  izle bakiye < 0)")
				continue
			}
			expr := strings.TrimSpace(cmdLine[len(parts[0]):])
			isCond := strings.ContainsAny(expr, "<>=!&|")
			initVal := ""
			if !isCond {
				if val, ok := env.Get(expr); ok {
					initVal = val.Inspect()
				}
			}
			wp := &Watchpoint{
				ID:         len(d.Watchpoints) + 1,
				Expression: expr,
				IsCond:     isCond,
				LastVal:    initVal,
			}
			d.Watchpoints = append(d.Watchpoints, wp)
			fmt.Printf("  👁️ İzleme Noktası (Watchpoint #%d) eklendi: %q\n", wp.ID, expr)

		case "tablo", "incele", "t":
			if len(parts) < 2 {
				fmt.Println("  ⚠️ Kullanım: tablo <değişken_adı>")
				continue
			}
			d.inspectTable(parts[1], env)

		case "yığın", "yigin", "stack", "bt":
			fmt.Println("  📚 ÇAĞRI YIĞINI (ABAP Call Stack):")
			fmt.Printf("    #0 [Aktif] Satır %d\n", line)
			stack := v.StackSnapshot()
			for i := len(stack) - 1; i >= 0; i-- {
				f := stack[i]
				fmt.Printf("    #%d %s (Çağrıldığı Satır: %d)\n", len(stack)-i, f.FuncName, f.Line)
			}

		case "hesapla", "yaz", "p", "?":
			if len(parts) < 2 {
				fmt.Println("  ⚠️ Kullanım: hesapla <ifade>  (Örn: hesapla x * 2 + 1)")
				continue
			}
			exprStr := strings.TrimSpace(cmdLine[len(parts[0]):])
			l := lexer.New(exprStr)
			p := parser.New(l)
			prog := p.ParseProgram()
			if len(p.Errors()) == 0 && len(prog.Statements) > 0 {
				if es, ok := prog.Statements[0].(*ast.ExpressionStatement); ok {
					val, err := v.evalExpression(es.Expression, env)
					if err != nil {
						fmt.Printf("  ❌ Hata: %v\n", err)
					} else {
						fmt.Printf("  💡 Sonuç (%s): %s\n", val.Type(), val.Inspect())
					}
				}
			}

		case "ekran", "göster", "goster", "l":
			d.renderABAPScreen(v, line, env)

		case "yardım", "yardim", "h", "help":
			d.printDebugHelp()

		case "çıkış", "cikis", "q", "quit":
			fmt.Println("Hata ayıklayıcıdan çıkılıyor...")
			os.Exit(0)

		default:
			// Eğer doğrudan bir değişken adı yazıldıysa değerini göster
			if val, ok := env.Get(cmdLine); ok {
				fmt.Printf("  🔍 %s (%s) = %s\n", cmdLine, val.Type(), val.Inspect())
			} else {
				fmt.Println("  ❓ Bilinmeyen komut. Yardım için 'yardım' yazın.")
			}
		}
	}
}

func (d *Debugger) renderABAPScreen(v *VM, currentLine int, env *Environment) {
	fmt.Println("\n╔══════════════════════════════════════════════════════════════════════════════╗")
	fmt.Printf("║ 🛠️  TR ABAP-TARZI HATA AYIKLAYICI (DEBUGGER) | Dosya: %-22s ║\n", d.SourceFile)
	if d.PauseReason != "" {
		fmt.Printf("║ 📌 Durum: %-66s ║\n", d.PauseReason)
	}
	fmt.Println("╠══════════════════════════════════════════════════════════════════════════════╣")

	start := currentLine - 3
	if start < 1 {
		start = 1
	}
	end := currentLine + 3
	if end > len(d.SourceLines) {
		end = len(d.SourceLines)
	}

	for l := start; l <= end; l++ {
		prefix := "     "
		if l == currentLine {
			prefix = "===> "
		} else if d.hasBreakpoint(l) {
			prefix = " [🔴] "
		}
		codeLine := ""
		if l-1 < len(d.SourceLines) {
			codeLine = d.SourceLines[l-1]
		}
		fmt.Printf("  %s%3d | %s\n", prefix, l, codeLine)
	}

	fmt.Println("╟──────────────────────────────────────────────────────────────────────────────╢")
	// Sistem ve Kullanıcı Değişkenleri Özeti (ABAP Locals / Globals)
	vars := collectVisibleVariables(env)
	fmt.Printf("  📊 SİSTEM: SY-LINNO=%d | SY-STEP=%d | YIĞIN_DERİNLİĞİ=%d\n", currentLine, d.StepCount, v.StackDepth())
	if len(vars) == 0 {
		fmt.Println("  📦 DEĞİŞKENLER: (Henüz kullanıcı değişkeni tanımlanmadı)")
	} else {
		fmt.Println("  📦 DEĞİŞKENLER (Canlı değiştirmek için: 'değiştir ad = değer'):")
		for _, kv := range vars {
			valPreview := kv.Val.Inspect()
			if len(valPreview) > 52 {
				valPreview = valPreview[:52] + "..."
			}
			fmt.Printf("     • %-16s : %-10s = %s\n", kv.Name, "("+string(kv.Val.Type())+")", valPreview)
		}
	}
	fmt.Println("╚══════════════════════════════════════════════════════════════════════════════╝")
}

func (d *Debugger) inspectTable(name string, env *Environment) {
	val, ok := env.Get(name)
	if !ok {
		fmt.Printf("  ❌ %q adında bir değişken bulunamadı.\n", name)
		return
	}
	fmt.Printf("\n  📋 ABAP TABLO / NESNE İNCELEYİCİ: %s (%s)\n", name, val.Type())
	fmt.Println("  " + strings.Repeat("─", 56))
	switch t := val.(type) {
	case *ListVal:
		fmt.Printf("  Toplam Satır Sayısı: %d\n", len(t.Elements))
		fmt.Printf("  %-8s | %-12s | %s\n", "İNDEKS", "TİP", "DEĞER")
		fmt.Println("  " + strings.Repeat("─", 56))
		for i, el := range t.Elements {
			fmt.Printf("  [%-6d] | %-12s | %s\n", i, el.Type(), el.Inspect())
		}
	case *MatrixVal:
		fmt.Printf("  Matris Boyutu: %d Satır x %d Sütun\n", t.Rows, t.Cols)
		for i := 0; i < t.Rows; i++ {
			fmt.Printf("  Satır %-3d | %v\n", i, t.Data[i])
		}
	case *DictVal:
		fmt.Printf("  Toplam Alan Sayısı: %d\n", len(t.Order))
		fmt.Printf("  %-18s | %-12s | %s\n", "ANAHTAR", "TİP", "DEĞER")
		fmt.Println("  " + strings.Repeat("─", 56))
		for _, k := range t.Order {
			v := t.Pairs[k]
			fmt.Printf("  %-18s | %-12s | %s\n", t.DisplayKey(k), v.Type(), v.Inspect())
		}
	case *StructInstanceVal:
		fmt.Printf("  Yapı Türü: %s\n", t.Def.Name)
		for k, v := range t.Fields {
			fmt.Printf("  • %-16s (%s) = %s\n", k, v.Type(), v.Inspect())
		}
	default:
		fmt.Printf("  Değer: %s\n", val.Inspect())
	}
}

func (d *Debugger) printDebugHelp() {
	fmt.Println("\n  🛠️  TR ABAP-TARZI DEBUGGER KOMUT REHBERİ:")
	fmt.Println("  • f5 / adım / Enter       : Tek adım ilerle (Fonksiyonların içine girer)")
	fmt.Println("  • f6 / geç                : Üstten geç (Fonksiyonu tek adımda çalıştırır)")
	fmt.Println("  • f7 / dön                : Mevcut fonksiyondan çıkana kadar çalıştır")
	fmt.Println("  • f8 / devam              : Sonraki kesme veya izleme noktasına kadar devam et")
	fmt.Println("  • değiştir x = 100        : ✏️ Çalışma anında 'x' değişkeninin değerini canlı değiştir!")
	fmt.Println("  • zıpla 15                : ⏭️ (Shift+F12) Çalışma sırasını 15. satıra geri/ileri taşı!")
	fmt.Println("  • izle bakiye < 0         : 👁️ (Watchpoint) Koşul sağlandığında veya değişken değiştiğinde dur!")
	fmt.Println("  • kesme 20                : 🔴 20. satıra kesme noktası (Breakpoint) ekle/kaldır")
	fmt.Println("  • tablo liste_adı         : 📋 Liste, Matris veya Sözlüğü ABAP tablo görünümünde aç")
	fmt.Println("  • yığın                   : 📚 Çağrı yığınını (Call Stack) göster")
	fmt.Println("  • hesapla x * 2           : 💡 Herhangi bir TR ifadesini anında hesapla")
	fmt.Println("  • çıkış                   : Hata ayıklayıcıyı sonlandır")
}

type varEntry struct {
	Name string
	Val  Value
}

func collectVisibleVariables(env *Environment) []varEntry {
	seen := make(map[string]bool)
	var list []varEntry
	cur := env
	for cur != nil {
		cur.mu.RLock()
		for k, v := range cur.store {
			if seen[k] {
				continue
			}
			seen[k] = true
			// Yerleşik fonksiyonları ve yapı tanımlarını kalabalık yapmaması için gizle
			if v.Type() == VAL_BUILTIN || v.Type() == VAL_STRUCT || v.Type() == VAL_FUNC {
				continue
			}
			list = append(list, varEntry{Name: k, Val: v})
		}
		cur.mu.RUnlock()
		cur = cur.outer
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
	return list
}

// ============================================================================
// GÖRSEL WEB ABAP DEBUGGER STÜDYOSU (tr debug --web dosya.tr)
// ============================================================================

// debugTokenHeader web arayüzünün her API isteğinde göndermesi gereken erişim anahtarı başlığıdır.
// Özel bir başlık olduğu için başka sitelerden gelen tarayıcı istekleri CORS ön kontrolüne takılır.
const debugTokenHeader = "X-TR-Anahtar"

func newDebugToken() string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("güvenli rastgele anahtar üretilemedi: %v", err))
	}
	return hex.EncodeToString(b)
}

// StartWebServer görsel debugger'ı yalnızca 127.0.0.1 üzerinde başlatır ve rastgele bir
// erişim anahtarı üretir. Anahtarı bilmeyen (başka bir site, ağdaki başka bir makine)
// hiçbir API'yi çağıramaz. Port kullanılamıyorsa hata döner.
func (d *Debugger) StartWebServer(v *VM, port int) error {
	d.WebMode = true
	d.WebPort = port
	d.webWaitCh = make(chan webDebugAction)
	d.curVM = v
	d.webToken = newDebugToken()

	allowedHosts := map[string]bool{
		fmt.Sprintf("127.0.0.1:%d", port): true,
		fmt.Sprintf("localhost:%d", port): true,
		fmt.Sprintf("[::1]:%d", port):     true,
	}
	tokenOK := func(given string) bool {
		return subtle.ConstantTimeCompare([]byte(given), []byte(d.webToken)) == 1
	}
	// guard: DNS rebinding'e karşı Host başlığını, ardından anahtarı ve HTTP metodunu denetler.
	guard := func(post bool, h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			if !allowedHosts[strings.ToLower(r.Host)] {
				http.Error(w, "yasak: geçersiz Host", http.StatusForbidden)
				return
			}
			if post && r.Method != http.MethodPost {
				http.Error(w, "yalnızca POST", http.StatusMethodNotAllowed)
				return
			}
			if !tokenOK(r.Header.Get(debugTokenHeader)) {
				http.Error(w, "yetkisiz: erişim anahtarı eksik veya hatalı", http.StatusUnauthorized)
				return
			}
			h(w, r)
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		if !allowedHosts[strings.ToLower(r.Host)] {
			http.Error(w, "yasak: geçersiz Host", http.StatusForbidden)
			return
		}
		if !tokenOK(r.URL.Query().Get("anahtar")) {
			http.Error(w, "yetkisiz: terminalde yazdırılan bağlantıyı (anahtarlı adres) kullanın", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(strings.Replace(abapWebDebuggerHTML, "__TR_ANAHTAR__", d.webToken, 1)))
	})

	mux.HandleFunc("/api/state", guard(false, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		d.mu.Lock()
		defer d.mu.Unlock()
		vars := []map[string]string{}
		// Değişkenler yalnızca program duraklatılmışken okunur; çalışırken VM aynı haritalara yazar
		if d.paused && d.curEnv != nil {
			for _, kv := range collectVisibleVariables(d.curEnv) {
				vars = append(vars, map[string]string{
					"name":  kv.Name,
					"type":  string(kv.Val.Type()),
					"value": kv.Val.Inspect(),
				})
			}
		}
		bps := []int{}
		for l := range d.Breakpoints {
			bps = append(bps, l)
		}
		wps := []string{}
		for _, wp := range d.Watchpoints {
			wps = append(wps, wp.Expression)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"file":        d.SourceFile,
			"lines":       d.SourceLines,
			"currentLine": d.curLine,
			"reason":      d.PauseReason,
			"stepCount":   d.StepCount,
			"variables":   vars,
			"breakpoints": bps,
			"watchpoints": wps,
			"active":      d.Active,
			"paused":      d.paused,
		})
	}))

	mux.HandleFunc("/api/step", guard(true, func(w http.ResponseWriter, r *http.Request) {
		var req webDebugAction
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "geçersiz istek", http.StatusBadRequest)
			return
		}
		switch req.Action {
		case "f5", "f6", "f7", "f8", "jump":
		default:
			http.Error(w, "bilinmeyen adım komutu", http.StatusBadRequest)
			return
		}
		// Yalnızca program duraklatılmış ve komut bekliyorken kabul et; eskiden her tıklama
		// bekleyen bir goroutine bırakıyor ve sonradan "hayalet adımlar" olarak oynatılıyordu.
		select {
		case d.webWaitCh <- req:
		default:
			http.Error(w, "program şu an çalışıyor (duraklatılmış değil)", http.StatusConflict)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))

	mux.HandleFunc("/api/modify", guard(true, func(w http.ResponseWriter, r *http.Request) {
		var req webDebugAction
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
			http.Error(w, "geçersiz istek", http.StatusBadRequest)
			return
		}
		// Değişiklik HTTP goroutine'inde değil, duraklatılmış VM'nin kendi goroutine'inde uygulanır
		req.Action = "modify"
		req.reply = make(chan error, 1)
		select {
		case d.webWaitCh <- req:
		default:
			http.Error(w, "program şu an çalışıyor (duraklatılmış değil)", http.StatusConflict)
			return
		}
		select {
		case err := <-req.reply:
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		case <-time.After(10 * time.Second):
			http.Error(w, "zaman aşımı", http.StatusGatewayTimeout)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))

	mux.HandleFunc("/api/breakpoint", guard(true, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Line int `json:"line"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "geçersiz istek", http.StatusBadRequest)
			return
		}
		d.toggleBreakpoint(req.Line)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))

	// Yalnızca yerel makineden erişilebilir (0.0.0.0 değil)
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return fmt.Errorf("görsel debugger başlatılamadı (port %d): %v", port, err)
	}
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	d.webServer = srv
	go func() {
		_ = srv.Serve(ln)
	}()

	out := v.Out
	if out == nil {
		out = os.Stdout
	}
	fmt.Fprintf(out, "🐞 Görsel ABAP Debugger Studio: http://127.0.0.1:%d/?anahtar=%s\n", port, d.webToken)
	fmt.Fprintln(out, "   (Bu bağlantı gizli bir erişim anahtarı içerir; başkasıyla paylaşmayın.)")
	return nil
}

// StopWebServer görsel debugger sunucusunu kapatır.
func (d *Debugger) StopWebServer() {
	if d.webServer != nil {
		_ = d.webServer.Close()
	}
}

func (d *Debugger) waitForWebAction(v *VM, line int, env *Environment) int {
	d.mu.Lock()
	d.curVM = v
	d.curLine = line
	d.curEnv = env
	d.paused = true
	d.mu.Unlock()

	var act webDebugAction
	for {
		act = <-d.webWaitCh
		if act.Action != "modify" {
			break
		}
		act.reply <- d.applyModify(v, env, act.Name, act.Value)
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	d.paused = false
	switch act.Action {
	case "f5":
		d.Mode = StepInto
	case "f6":
		d.Mode = StepOver
		d.TargetDepth = v.StackDepth()
	case "f7":
		d.Mode = StepOut
		d.TargetDepth = v.StackDepth()
	case "f8":
		d.Mode = StepContinue
	case "jump":
		d.Mode = StepInto
		d.PauseReason = fmt.Sprintf("Satır %s zıplandı (Shift+F12)", turkce.SayiEk(int64(act.Line), turkce.Yonelme))
		return act.Line
	}
	return 0
}

// applyModify ABAP ✏️ canlı değişken değiştirme: ifadeyi duraklatılmış VM üzerinde hesaplar ve atar.
func (d *Debugger) applyModify(v *VM, env *Environment, name, expr string) error {
	l := lexer.New(expr)
	p := parser.New(l)
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 || len(prog.Statements) == 0 {
		return fmt.Errorf("geçersiz ifade: %s", expr)
	}
	es, ok := prog.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		return fmt.Errorf("yalnızca bir ifade girilebilir: %s", expr)
	}
	// İfade içindeki fonksiyon çağrıları debugger'ı yeniden durdurmasın
	wasActive := d.Active
	d.Active = false
	val, err := v.evalExpression(es.Expression, env)
	d.Active = wasActive
	if err != nil {
		return err
	}
	if _, exists := env.Get(name); exists {
		return env.Assign(name, val)
	}
	env.Define(name, val, false)
	return nil
}

const abapWebDebuggerHTML = `<!DOCTYPE html>
<html lang="tr">
<head>
<meta charset="UTF-8">
<title>TR ABAP-Tarzı Görsel Hata Ayıklayıcı (Debugger)</title>
<style>
  body { margin: 0; font-family: 'Segoe UI', Tahoma, sans-serif; background: #1e2227; color: #e0e6ed; display: flex; flex-direction: column; height: 100vh; }
  header { background: #282c34; padding: 10px 18px; border-bottom: 2px solid #f0b429; display: flex; align-items: center; justify-content: space-between; }
  .brand { font-weight: bold; font-size: 16px; color: #f0b429; }
  .toolbar button { background: #3b4252; color: #fff; border: 1px solid #4c566a; padding: 7px 14px; margin-right: 6px; border-radius: 4px; cursor: pointer; font-weight: 600; }
  .toolbar button:hover { background: #f0b429; color: #1e2227; }
  .main { display: flex; flex: 1; overflow: hidden; }
  .code-panel { flex: 3; overflow-y: auto; border-right: 1px solid #333842; padding: 10px 0; font-family: Consolas, monospace; font-size: 14px; }
  .code-line { display: flex; align-items: center; padding: 2px 12px; }
  .code-line.active { background: rgba(240, 180, 41, 0.22); border-left: 4px solid #f0b429; }
  .line-num { width: 42px; color: #636d83; cursor: pointer; user-select: none; }
  .jump-btn { font-size: 11px; margin-right: 8px; background: #2c313a; color: #98c379; border: 1px solid #3e4451; border-radius: 3px; cursor: pointer; padding: 1px 5px; }
  .side-panel { flex: 2; padding: 14px; overflow-y: auto; background: #21252b; }
  table { width: 100%; border-collapse: collapse; margin-top: 8px; font-size: 13px; }
  th, td { border: 1px solid #333842; padding: 6px 8px; text-align: left; }
  th { background: #2c313a; color: #61afef; }
  .edit-btn { background: #e5c07b; color: #1e2227; border: none; padding: 2px 7px; border-radius: 3px; cursor: pointer; font-weight: bold; }
</style>
</head>
<body>
<header>
  <div class="brand">🇹🇷 TR ABAP-Tarzı Görsel Debugger — <span id="fileName"></span></div>
  <div class="toolbar">
    <button onclick="sendStep('f5')">F5: Tek Adım</button>
    <button onclick="sendStep('f6')">F6: Üstten Geç</button>
    <button onclick="sendStep('f7')">F7: Fonksiyondan Çık</button>
    <button onclick="sendStep('f8')">F8: Devam Et</button>
  </div>
  <div id="statusBadge" style="color:#98c379;font-size:13px;"></div>
</header>
<div class="main">
  <div class="code-panel" id="codePanel"></div>
  <div class="side-panel">
    <h3 style="margin-top:0;color:#e5c07b;">📦 Canlı Değişkenler & Tablo İnceleyici (✏️ Değiştir)</h3>
    <table>
      <thead><tr><th>Değişken</th><th>Tip</th><th>Değer</th><th>İşlem</th></tr></thead>
      <tbody id="varTable"></tbody>
    </table>
  </div>
</div>
<script>
// Sunucunun bu sayfaya gömdüğü erişim anahtarı; her API isteğinde başlık olarak gönderilir.
const ANAHTAR = '__TR_ANAHTAR__';
// Adres çubuğundaki anahtarı geçmişten/ekran görüntülerinden gizle
history.replaceState(null, '', '/');

function api(path, body) {
  const opts = { headers: { 'X-TR-Anahtar': ANAHTAR } };
  if (body !== undefined) {
    opts.method = 'POST';
    opts.headers['Content-Type'] = 'application/json';
    opts.body = JSON.stringify(body);
  }
  return fetch(path, opts);
}
// el: güvenli DOM düğümü üretir; metinler her zaman textContent ile yazılır (innerHTML yok)
function el(tag, text, cls) {
  const e = document.createElement(tag);
  if (text !== undefined) e.textContent = text;
  if (cls) e.className = cls;
  return e;
}
async function refresh() {
  const res = await api('/api/state');
  if (!res.ok) {
    document.getElementById('statusBadge').textContent = 'Bağlantı yetkisiz veya kesildi (' + res.status + ')';
    return;
  }
  const st = await res.json();
  document.getElementById('fileName').textContent = st.file + ' (Satır: ' + st.currentLine + ', Adım: ' + st.stepCount + ')';
  document.getElementById('statusBadge').textContent = st.reason || 'Hazır';
  const cp = document.getElementById('codePanel');
  cp.replaceChildren();
  st.lines.forEach((txt, idx) => {
    const ln = idx + 1;
    const isAct = ln === st.currentLine;
    const isBp = (st.breakpoints || []).includes(ln);
    const row = el('div', undefined, 'code-line' + (isAct ? ' active' : ''));
    const num = el('span', (isBp ? '🔴' : '') + ln, 'line-num');
    num.addEventListener('click', () => toggleBp(ln));
    const jb = el('button', '⏭️ Zıpla', 'jump-btn');
    jb.title = 'Shift+F12: Bu satıra zıpla';
    jb.addEventListener('click', () => jumpTo(ln));
    const code = el('span', (isAct ? '==> ' : '    ') + txt);
    code.style.whiteSpace = 'pre';
    row.append(num, jb, code);
    cp.appendChild(row);
  });
  const vt = document.getElementById('varTable');
  vt.replaceChildren();
  (st.variables || []).forEach(v => {
    const tr = el('tr');
    const nameTd = el('td');
    nameTd.appendChild(el('b', v.name));
    const valTd = el('td');
    valTd.appendChild(el('code', v.value));
    const actTd = el('td');
    const btn = el('button', '✏️ Değiştir', 'edit-btn');
    btn.addEventListener('click', () => editVar(v.name, v.value));
    actTd.appendChild(btn);
    tr.append(nameTd, el('td', v.type), valTd, actTd);
    vt.appendChild(tr);
  });
}
async function sendStep(act) {
  await api('/api/step', {action:act});
  setTimeout(refresh, 120);
}
async function jumpTo(line) {
  await api('/api/step', {action:'jump', line:line});
  setTimeout(refresh, 120);
}
async function toggleBp(line) {
  await api('/api/breakpoint', {line:line});
  refresh();
}
async function editVar(name, curVal) {
  const nv = prompt(name + ' değişkeninin yeni değerini girin:', curVal);
  if (nv !== null) {
    await api('/api/modify', {name:name, value:nv});
    refresh();
  }
}
refresh();
setInterval(refresh, 1000);
</script>
</body>
</html>`
