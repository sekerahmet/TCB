// Package debugger TR'nin hata ayıklayıcısıdır: terminalde çalışan etkileşimli hata
// ayıklayıcı ('tr hata_ayıkla', koddaki kesme_noktası) ve düzenleyicilerin bağlandığı
// hata ayıklama sunucusu ('tr dap', dap*.go). Yükleme anında kendini yorumlayıcıya
// interp.NewDebugger ile kaydeder.
//
// Layer: developer tools (internal/devtools); dil, motor ve hesap katmanlarını içe aktarabilir.
package debugger

import (
	"bufio"
	"fmt"
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
)

type StepMode int

const (
	// Tuş adları düzenleyicinin hata ayıklama tuşlarıyla aynı anlamdadır (terminal ve düzenleyici).
	StepInto     StepMode = iota // F11: Tek Adım (Fonksiyon içine girer)
	StepOver                     // F10: Üstten Geç (Fonksiyonu tek adımda geçer)
	StepOut                      // Shift+F11: Fonksiyondan Çık
	StepContinue                 // F5: Kesme noktasına / Watchpoint'e kadar devam et
)

type Watchpoint struct {
	ID         int
	Expression string // Değişken adı veya koşul ifadesi (örn: "bakiye" veya "bakiye < 0")
	IsCond     bool
	// LastVal son görülen değerin yazılışıdır (koşulda doğru/yanlış).
	LastVal  string
	lastType interp.ValueType
	// seen değişkenin değeri görüldü mü: boş metin de bir değerdir, "henüz yok" anlamına gelmez.
	seen bool
}

// observe değişkenin şimdiki değerini kaydeder; önceki değerden farklıysa (yazılışı ya da
// tipi: Tam 123 ile Metin "123" farklıdır) önceki yazılışı ve doğru döner.
func (wp *Watchpoint) observe(val interp.Value) (old string, changed bool) {
	cur, typ := val.Inspect(), val.Type()
	old, changed = wp.LastVal, wp.seen && (cur != wp.LastVal || typ != wp.lastType)
	wp.LastVal, wp.lastType, wp.seen = cur, typ, true
	return old, changed
}

type Debugger struct {
	SourceFile  string
	SourceLines []string
	Mode        StepMode
	TargetDepth int
	Breakpoints map[int]bool
	Watchpoints []*Watchpoint
	Reader      *bufio.Reader
	Active      bool
	PauseReason string
	StepCount   int

	// pauseKind duraklamanın türüdür (düzenleyiciye bildirilir): "entry", "step",
	// "breakpoint", "data breakpoint", "pause", "goto".
	pauseKind string
	// dap, düzenleyicinin hata ayıklama oturumu (tr dap) etkinse bağlı sunucudur.
	dap *dapServer
	// otherLines başka dosyaların ('aktar' ile yüklenen modüller) satırlarıdır (fileOf).
	otherLines map[string][]string

	// mu: düzenleyici oturumunun goroutine'i ile VM arasında paylaşılan alanları korur
	// (Breakpoints, Mode, PauseReason, StepCount).
	mu sync.Mutex
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
		Reader:      interp.StdinReader(),
		Active:      true,
		PauseReason: messages.Text(messages.H8030),
	}
}

// Breakpoint koddaki kesme_noktası deyiminde çağrılır: bir sonraki deyimde durur.
func (d *Debugger) Breakpoint(why string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.Mode = StepInto
	d.pauseKind = "breakpoint"
	d.PauseReason = why
}

// BeforeStatement her deyim çalışmadan önce VM tarafından çağrılır.
// Eğer kullanıcı 'zıpla <satır>' (Goto Statement) yaptıysa hedef satırı (>0) döndürür.
func (d *Debugger) BeforeStatement(v *interp.VM, stmt ast.Statement, env *interp.Environment) int {
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

	// Düzenleyici oturumunda kesme noktaları koşul ve iz mesajı taşıyabilir; ifade
	// değerlendirmesi kilit dışında yapılır (iç içe deyimler BeforeStatement'ı çağırır).
	// Kesme noktaları hata ayıklanan dosyanın satırlarındadır; 'aktar' ile yüklenen modüldeki
	// aynı numaralı satır onları tetiklemez.
	own := d.inOwnFile(v)
	bpHit := false
	if d.dap != nil && own {
		bpHit = d.dap.breakpointHit(v, line, env)
	}

	d.mu.Lock()
	if wpReason != "" {
		shouldPause = true
		d.PauseReason = wpReason
		d.pauseKind = "data breakpoint"
	}

	// 2. Satır kesme noktası (Breakpoint) kontrolü
	if !shouldPause && (bpHit || (d.dap == nil && own && d.Breakpoints[line])) {
		shouldPause = true
		d.PauseReason = messages.Text(messages.H8031, line)
		d.pauseKind = "breakpoint"
	}

	// 3. Adım modu (F11 / F10 / Shift+F11) kontrolü
	if !shouldPause {
		shouldPause = d.stepPause(depth)
	}

	d.mu.Unlock()

	if !shouldPause {
		return 0
	}

	if d.dap != nil {
		return d.dap.waitForCommand(v, line, env)
	}
	return d.promptCLI(v, line, env)
}

// stepPause adım kipine (F11 / F10 / Shift+F11) göre bu derinlikteki deyimde durulacak mı;
// duruluyorsa nedeni yazar. d.mu tutulurken çağrılır.
func (d *Debugger) stepPause(depth int) bool {
	switch d.Mode {
	case StepInto:
		if d.PauseReason == "" {
			d.PauseReason = stateStep()
		}
		if d.pauseKind == "" {
			d.pauseKind = "step"
		}
		return true
	case StepOver:
		if depth <= d.TargetDepth {
			d.PauseReason, d.pauseKind = stateStepOver(), "step"
			return true
		}
	case StepOut:
		if depth < d.TargetDepth {
			d.PauseReason, d.pauseKind = stateStepOut(), "step"
			return true
		}
	}
	return false
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

// Adımlama sonrası durum metinleri; terminalde ve düzenleyicide aynıdır.
func stateStep() string     { return messages.Text(messages.H8034) }
func stateStepOver() string { return messages.Text(messages.H8035) }
func stateStepOut() string  { return messages.Text(messages.H8036) }

func (d *Debugger) promptCLI(v *interp.VM, line int, env *interp.Environment) int {
	d.renderDebugScreen(v, line, env)
	d.PauseReason = ""

	for {
		fmt.Print("\n" + messages.Text(messages.H8001, keyGuide()))
		input, err := d.Reader.ReadString('\n')
		if err != nil {
			d.Mode = StepContinue
			d.Active = false
			return 0
		}
		cmdLine := strings.TrimSpace(strings.TrimPrefix(input, "\xef\xbb\xbf"))

		// Boş Enter -> F11 (Tek Adım)
		if cmdLine == "" {
			d.Mode = StepInto
			return 0
		}

		parts := strings.Fields(cmdLine)
		cmd := strings.ToLower(parts[0])

		command := commandCode(cmd)
		if command == codes.None {
			if actual, ok := oldCommand(cmd); ok {
				fmt.Println("  ⚠️ " + messages.Text(messages.H8041, cmd, codes.DisplayName(actual)))
				continue
			}
		}

		switch command {
		case codes.K300:
			d.Mode = StepInto
			return 0

		case codes.K301:
			d.Mode = StepOver
			d.TargetDepth = v.StackDepth()
			return 0

		case codes.K302:
			d.Mode = StepOut
			d.TargetDepth = v.StackDepth()
			return 0

		case codes.K303:
			d.Mode = StepContinue
			return 0

		case codes.K304:
			// değiştir bakiye = 5000
			rest := strings.TrimSpace(cmdLine[len(parts[0]):])
			if !strings.Contains(rest, "=") {
				fmt.Println("  ⚠️ " + messages.Text(messages.H8042, codes.DisplayName(codes.K304)))
				continue
			}
			eqParts := strings.SplitN(rest, "=", 2)
			varName := strings.TrimSpace(eqParts[0])
			exprStr := strings.TrimSpace(eqParts[1])
			e, status := parseFirstExpression(exprStr)
			if status == exprSyntaxError {
				fmt.Println("  ❌ " + messages.Text(messages.H8043))
				continue
			}
			if status == exprParsed {
				newVal, err := d.evalQuiet(v, env, e)
				if err != nil {
					fmt.Println("  ❌ " + messages.Text(messages.H8044, err))
					continue
				}
				if _, exists := env.Get(varName); exists {
					_ = env.Assign(varName, newVal)
				} else {
					env.Define(varName, newVal, false)
				}
				fmt.Println("  ✏️ " + messages.Text(messages.H8045, varName, v.Mask(newVal.Inspect()), newVal.Type()))
			}

		case codes.K305:
			if len(parts) < 2 {
				fmt.Println("  ⚠️ " + messages.Text(messages.H8046, codes.DisplayName(codes.K305)))
				continue
			}
			targetLine, err := strconv.Atoi(parts[1])
			if err != nil || targetLine < 1 {
				fmt.Println("  ❌ " + messages.Text(messages.H8047))
				continue
			}
			// Düzenleyicideki gibi yalnızca duraklanan bloğun satırlarına zıplanabilir;
			// hedef satırda deyim yoksa sonraki ilk deyime gidilir.
			target, ok := v.JumpTarget(targetLine)
			if !ok {
				fmt.Println("  ❌ " + jumpOutsideBlock(targetLine))
				continue
			}
			targetLine = target
			fmt.Println("  ⏭️ " + messages.Text(messages.H8048, targetLine))
			d.Mode = StepInto
			d.PauseReason = messages.Text(messages.H8049, targetLine)
			return targetLine

		case codes.K306:
			if len(parts) < 2 {
				fmt.Println("  ⚠️ " + messages.Text(messages.H8050, codes.DisplayName(codes.K306)))
				continue
			}
			bpLine, err := strconv.Atoi(parts[1])
			if err != nil || bpLine < 1 || bpLine > len(d.SourceLines) {
				fmt.Println("  ❌ " + messages.Text(messages.H8047))
				continue
			}
			if !d.toggleBreakpoint(bpLine) {
				fmt.Println("  ⚪ " + messages.Text(messages.H8051, bpLine))
			} else {
				fmt.Println("  🔴 " + messages.Text(messages.H8052, bpLine))
			}

		case codes.K307:
			if len(parts) < 2 {
				fmt.Println("  ⚠️ " + messages.Text(messages.H8053, codes.DisplayName(codes.K307)))
				continue
			}
			expr := strings.TrimSpace(cmdLine[len(parts[0]):])
			isCond := strings.ContainsAny(expr, "<>=!&|")
			wp := &Watchpoint{
				ID:         len(d.Watchpoints) + 1,
				Expression: expr,
				IsCond:     isCond,
			}
			if !isCond {
				if val, ok := env.Get(expr); ok {
					wp.observe(val)
				}
			}
			d.Watchpoints = append(d.Watchpoints, wp)
			fmt.Println("  👁️ " + messages.Text(messages.H8054, wp.ID, expr))

		case codes.K308:
			if len(parts) < 2 {
				fmt.Println("  ⚠️ " + messages.Text(messages.H8055, codes.DisplayName(codes.K308)))
				continue
			}
			d.inspectTable(v, parts[1], env)

		case codes.K309:
			fmt.Println("  📚 " + messages.Text(messages.H8056))
			fmt.Println("    " + messages.Text(messages.H8057, line))
			stack := v.StackSnapshot()
			for i := len(stack) - 1; i >= 0; i-- {
				f := stack[i]
				fmt.Println("    " + messages.Text(messages.H8058, len(stack)-i, f.FuncName, f.Line))
			}

		case codes.K310:
			if len(parts) < 2 {
				fmt.Println("  ⚠️ " + messages.Text(messages.H8059, codes.DisplayName(codes.K310)))
				continue
			}
			exprStr := strings.TrimSpace(cmdLine[len(parts[0]):])
			if e, status := parseFirstExpression(exprStr); status == exprParsed {
				val, err := d.evalQuiet(v, env, e)
				if err != nil {
					fmt.Println("  ❌ " + messages.Text(messages.H8060, err))
				} else {
					fmt.Println("  💡 " + messages.Text(messages.H8061, val.Type(), v.Mask(val.Inspect())))
				}
			}

		case codes.K311:
			d.renderDebugScreen(v, line, env)

		case codes.K312:
			d.printDebugHelp()

		case codes.K313:
			fmt.Println(messages.Text(messages.H8062))
			os.Exit(0)

		default:
			// Eğer doğrudan bir değişken adı yazıldıysa değerini göster
			if val, ok := env.Get(cmdLine); ok {
				fmt.Printf("  🔍 %s (%s) = %s\n", cmdLine, val.Type(), v.Mask(val.Inspect()))
			} else {
				fmt.Println("  ❓ " + messages.Text(messages.H8063, codes.DisplayName(codes.K312)))
			}
		}
	}
}

func (d *Debugger) renderDebugScreen(v *interp.VM, currentLine int, env *interp.Environment) {
	// Satırlar çalışan deyimin dosyasından gösterilir ('aktar' ile yüklenen modül de olabilir).
	file := d.fileOf(v, v.CurrentSource())
	fmt.Println("\n╔══════════════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║ 🛠️  " + messages.Text(messages.H8070, fmt.Sprintf("%-35s", file.path)) + " ║")
	if d.PauseReason != "" {
		fmt.Println("║ 📌 " + messages.Text(messages.H8071, fmt.Sprintf("%-66s", d.PauseReason)) + " ║")
	}
	fmt.Println("╠══════════════════════════════════════════════════════════════════════════════╣")

	start := currentLine - 3
	if start < 1 {
		start = 1
	}
	end := currentLine + 3
	if end > len(file.lines) {
		end = len(file.lines)
	}

	for l := start; l <= end; l++ {
		prefix := "     "
		if l == currentLine {
			prefix = "===> "
		} else if file.own && d.hasBreakpoint(l) {
			prefix = " [🔴] "
		}
		codeLine := ""
		if l-1 < len(file.lines) {
			codeLine = file.lines[l-1]
		}
		fmt.Printf("  %s%3d | %s\n", prefix, l, codeLine)
	}

	fmt.Println("╟──────────────────────────────────────────────────────────────────────────────╢")
	// Yerel ve genel değişkenlerin özeti
	vars := collectVisibleVariables(v, env)
	fmt.Println("  📊 " + messages.Text(messages.H8072, currentLine, d.StepCount, v.StackDepth()))
	if len(vars) == 0 {
		fmt.Println("  📦 " + messages.Text(messages.H8073))
	} else {
		fmt.Println("  📦 " + messages.Text(messages.H8074, codes.DisplayName(codes.K304)))
		for _, kv := range vars {
			valPreview := v.Mask(kv.Val.Inspect())
			if _, engine := kv.Val.(WatchValue); engine && strings.Contains(valPreview, "\n") {
				valPreview = singleLine(valPreview)
			}
			if r := []rune(valPreview); len(r) > 52 {
				valPreview = string(r[:52]) + "..."
			}
			fmt.Printf("     • %-16s : %-10s = %s\n", kv.Name, "("+string(kv.Val.Type())+")", valPreview)
		}
	}
	fmt.Println("╚══════════════════════════════════════════════════════════════════════════════╝")
}

// singleLine çok satırlı yazdırma biçimini (ör. tensör) değişken kutusunda tek satıra indirir:
// "Tensör[2×3] [\n  [1, 2, 3],\n  ...\n]" → "Tensör[2×3] [[1, 2, 3], ...]".
func singleLine(text string) string {
	lines := strings.Split(text, "\n")
	for i, s := range lines {
		lines[i] = strings.TrimSpace(s)
	}
	single := strings.Join(lines, " ")
	return strings.NewReplacer("[ ", "[", " ]", "]").Replace(single)
}

// inspectTable bir değişkeni tablo olarak yazar; değerlerdeki gizli değerler maskelenir.
func (d *Debugger) inspectTable(v *interp.VM, name string, env *interp.Environment) {
	val, ok := env.Get(name)
	if !ok {
		fmt.Println("  ❌ " + messages.Text(messages.H8075, name))
		return
	}
	fmt.Println("\n  📋 " + messages.Text(messages.H8076, name, val.Type()))
	fmt.Println("  " + strings.Repeat("─", 56))
	switch t := val.(type) {
	case *interp.ListVal:
		fmt.Println("  " + messages.Text(messages.H8077, len(t.Elements)))
		fmt.Printf("  %-8s | %-12s | %s\n", messages.Text(messages.H8078), messages.Text(messages.H8079), messages.Text(messages.H8080))
		fmt.Println("  " + strings.Repeat("─", 56))
		for i, el := range t.Elements {
			fmt.Printf("  [%-6d] | %-12s | %s\n", i, el.Type(), v.Mask(el.Inspect()))
		}
	case WatchValue:
		// Motor değerleri (ör. tensör): tablo verisi satır satır yazılır.
		cols, rows, err := t.TableData()
		if err != nil {
			fmt.Printf("  %s\n  %s\n", t.WatchLabel(), messages.Text(messages.H8081, v.Mask(val.Inspect())))
			break
		}
		fmt.Printf("  %s\n", t.WatchLabel())
		fmt.Printf("  %-9s | %s\n", messages.Text(messages.H8082), strings.Join(cols, " | "))
		for i, r := range rows {
			fmt.Printf("  %-9d | %s\n", i, v.Mask(strings.Join(r, " | ")))
		}
	case *interp.DictVal:
		fmt.Println("  " + messages.Text(messages.H8083, len(t.Order)))
		fmt.Printf("  %-18s | %-12s | %s\n", messages.Text(messages.H8084), messages.Text(messages.H8079), messages.Text(messages.H8080))
		fmt.Println("  " + strings.Repeat("─", 56))
		for _, k := range t.Order {
			e := t.Pairs[k]
			fmt.Printf("  %-18s | %-12s | %s\n", t.DisplayKey(k), e.Type(), v.Mask(e.Inspect()))
		}
	case *interp.StructInstanceVal:
		fmt.Println("  " + messages.Text(messages.H8085, t.Def.Name))
		for k, e := range t.Fields {
			fmt.Printf("  • %-16s (%s) = %s\n", k, e.Type(), v.Mask(e.Inspect()))
		}
	default:
		fmt.Println("  " + messages.Text(messages.H8081, v.Mask(val.Inspect())))
	}
}

type varEntry struct {
	Name string
	Val  interp.Value
}

func collectVisibleVariables(vm *interp.VM, env *interp.Environment) []varEntry {
	return collectVariables(vm, env, nil, false)
}

// collectVariables start kapsamından dışa doğru görünür değişkenleri ad sırasıyla toplar: iç
// kapsam dış kapsamdaki aynı adı gizler; yerleşik fonksiyonlar, yapı ve tip tanımları ile
// yerleşik sabitler kalabalık yapmaması için atlanır. stopBefore doluysa o kapsama gelince durur
// (kapsamın kendisi alınmaz); singleScope true ise yalnızca start taranır.
func collectVariables(vm *interp.VM, start, stopBefore *interp.Environment, singleScope bool) []varEntry {
	seen := make(map[string]bool)
	var list []varEntry
	for cur := start; cur != nil && cur != stopBefore; cur = cur.Outer() {
		cur.EachLocal(func(k string, v interp.Value) {
			if seen[k] {
				return
			}
			seen[k] = true
			if v.Type() == interp.ValBuiltin || v.Type() == interp.ValStruct || v.Type() == interp.ValFunc || v.Type() == interp.ValType || vm.IsBuiltinConstant(k, v) {
				return
			}
			list = append(list, varEntry{Name: k, Val: v})
		})
		if singleScope {
			break
		}
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
	return list
}

// Compile-time checks: the interpreter finds these hooks with type assertions, so a
// renamed or changed method would otherwise drop the hook silently.
var (
	_ interp.Debugger = (*Debugger)(nil)
)
