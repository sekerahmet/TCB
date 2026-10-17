package hataayiklama

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"tr_dili/pkg/ast"
	"tr_dili/pkg/lexer"
	"tr_dili/pkg/parser"
	"tr_dili/pkg/turkce"
	"tr_dili/pkg/vm"
)

type StepMode int

const (
	StepInto     StepMode = iota // F5: Tek Adım (Fonksiyon içine girer)
	StepOver                     // F6: Üstten Geç (Fonksiyonu tek adımda geçer)
	StepOut                      // F7: Fonksiyondan Çık
	StepContinue                 // F8: Kesme noktasına / Watchpoint'e kadar devam et
)

type Watchpoint struct {
	ID         int
	Expression string // Değişken adı veya koşul ifadesi (örn: "bakiye" veya "bakiye < 0")
	IsCond     bool
	LastVal    string
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

	// pauseKind duraklamanın türüdür (VS Code'a bildirilir): "entry", "step",
	// "breakpoint", "data breakpoint", "pause", "goto".
	pauseKind string
	// dap, VS Code hata ayıklama oturumu (tr dap) etkinse bağlı sunucudur.
	dap *dapServer

	// mu: VS Code oturumunun goroutine'i ile VM arasında paylaşılan alanları korur
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
		Reader:      bufio.NewReader(os.Stdin),
		Active:      true,
		PauseReason: "Başlangıç (/h Hata Ayıklama Modu)",
	}
}

// KesmeNoktasi koddaki kesme_noktası deyiminde çağrılır: bir sonraki deyimde durur.
func (d *Debugger) KesmeNoktasi(gerekce string) {
	d.Mode = StepInto
	d.pauseKind = "breakpoint"
	d.PauseReason = gerekce
}

// BeforeStatement her deyim çalışmadan önce VM tarafından çağrılır.
// Eğer kullanıcı 'zıpla <satır>' (Goto Statement) yaptıysa hedef satırı (>0) döndürür.
func (d *Debugger) BeforeStatement(v *vm.VM, stmt ast.Statement, env *vm.Environment) int {
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

	// VS Code oturumunda kesme noktaları koşul ve iz mesajı taşıyabilir; ifade
	// değerlendirmesi kilit dışında yapılır (iç içe deyimler BeforeStatement'ı çağırır).
	bpHit := false
	if d.dap != nil {
		bpHit = d.dap.breakpointHit(v, line, env)
	}

	d.mu.Lock()
	if wpReason != "" {
		shouldPause = true
		d.PauseReason = wpReason
		d.pauseKind = "data breakpoint"
	}

	// 2. Satır kesme noktası (Breakpoint) kontrolü
	if !shouldPause && (bpHit || (d.dap == nil && d.Breakpoints[line])) {
		shouldPause = true
		d.PauseReason = fmt.Sprintf("Kesme Noktası (Satır %d)", line)
		d.pauseKind = "breakpoint"
	}

	// 3. Adım modu (F5 / F6 / F7) kontrolü
	if !shouldPause {
		switch d.Mode {
		case StepInto:
			shouldPause = true
			if d.PauseReason == "" {
				d.PauseReason = "F5 Tek Adım"
			}
			if d.pauseKind == "" {
				d.pauseKind = "step"
			}
		case StepOver:
			if depth <= d.TargetDepth {
				shouldPause = true
				d.PauseReason = "F6 Üstten Adım"
				d.pauseKind = "step"
			}
		case StepOut:
			if depth < d.TargetDepth {
				shouldPause = true
				d.PauseReason = "F7 Fonksiyondan Çıkış"
				d.pauseKind = "step"
			}
		}
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

func (d *Debugger) checkWatchpoints(v *vm.VM, env *vm.Environment) string {
	for _, wp := range d.Watchpoints {
		if wp.IsCond {
			l := lexer.New(wp.Expression)
			p := parser.New(l)
			prog := p.ParseProgram()
			if len(p.Errors()) == 0 && len(prog.Statements) > 0 {
				if es, ok := prog.Statements[0].(*ast.ExpressionStatement); ok {
					val, err := v.EvalExpression(es.Expression, env)
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

func (d *Debugger) promptCLI(v *vm.VM, line int, env *vm.Environment) int {
	d.renderDebugScreen(v, line, env)
	d.PauseReason = ""

	for {
		fmt.Print("\n[TR Hata Ayıklayıcı] (F5:adım | F6:geç | F7:dön | F8:devam | değiştir | zıpla | tablo | izle) > ")
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
				newVal, err := v.EvalExpression(es.Expression, env)
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
			d.PauseReason = fmt.Sprintf("Satır %s zıplandı", turkce.SayiEk(int64(targetLine), turkce.Yonelme))
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
			fmt.Println("  📚 ÇAĞRI YIĞINI:")
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
					val, err := v.EvalExpression(es.Expression, env)
					if err != nil {
						fmt.Printf("  ❌ Hata: %v\n", err)
					} else {
						fmt.Printf("  💡 Sonuç (%s): %s\n", val.Type(), val.Inspect())
					}
				}
			}

		case "ekran", "göster", "goster", "l":
			d.renderDebugScreen(v, line, env)

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

func (d *Debugger) renderDebugScreen(v *vm.VM, currentLine int, env *vm.Environment) {
	fmt.Println("\n╔══════════════════════════════════════════════════════════════════════════════╗")
	fmt.Printf("║ 🛠️  TR HATA AYIKLAYICI  | Dosya: %-35s ║\n", d.SourceFile)
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
	// Yerel ve genel değişkenlerin özeti
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

func (d *Debugger) inspectTable(name string, env *vm.Environment) {
	val, ok := env.Get(name)
	if !ok {
		fmt.Printf("  ❌ %q adında bir değişken bulunamadı.\n", name)
		return
	}
	fmt.Printf("\n  📋 TABLO / NESNE İNCELEYİCİ: %s (%s)\n", name, val.Type())
	fmt.Println("  " + strings.Repeat("─", 56))
	switch t := val.(type) {
	case *vm.ListVal:
		fmt.Printf("  Toplam Satır Sayısı: %d\n", len(t.Elements))
		fmt.Printf("  %-8s | %-12s | %s\n", "İNDEKS", "TİP", "DEĞER")
		fmt.Println("  " + strings.Repeat("─", 56))
		for i, el := range t.Elements {
			fmt.Printf("  [%-6d] | %-12s | %s\n", i, el.Type(), el.Inspect())
		}
	case *vm.MatrixVal:
		fmt.Printf("  Matris Boyutu: %d Satır x %d Sütun\n", t.Rows, t.Cols)
		for i := 0; i < t.Rows; i++ {
			fmt.Printf("  Satır %-3d | %v\n", i, t.Data[i])
		}
	case *vm.DictVal:
		fmt.Printf("  Toplam Alan Sayısı: %d\n", len(t.Order))
		fmt.Printf("  %-18s | %-12s | %s\n", "ANAHTAR", "TİP", "DEĞER")
		fmt.Println("  " + strings.Repeat("─", 56))
		for _, k := range t.Order {
			v := t.Pairs[k]
			fmt.Printf("  %-18s | %-12s | %s\n", t.DisplayKey(k), v.Type(), v.Inspect())
		}
	case *vm.StructInstanceVal:
		fmt.Printf("  Yapı Türü: %s\n", t.Def.Name)
		for k, v := range t.Fields {
			fmt.Printf("  • %-16s (%s) = %s\n", k, v.Type(), v.Inspect())
		}
	default:
		fmt.Printf("  Değer: %s\n", val.Inspect())
	}
}

func (d *Debugger) printDebugHelp() {
	fmt.Println("\n  🛠️  TR HATA AYIKLAYICI KOMUT REHBERİ:")
	fmt.Println("  • f5 / adım / Enter       : Tek adım ilerle (Fonksiyonların içine girer)")
	fmt.Println("  • f6 / geç                : Üstten geç (Fonksiyonu tek adımda çalıştırır)")
	fmt.Println("  • f7 / dön                : Mevcut fonksiyondan çıkana kadar çalıştır")
	fmt.Println("  • f8 / devam              : Sonraki kesme veya izleme noktasına kadar devam et")
	fmt.Println("  • değiştir x = 100        : ✏️ Çalışma anında 'x' değişkeninin değerini canlı değiştir!")
	fmt.Println("  • zıpla 15                : ⏭️ Çalışma sırasını 15. satıra geri/ileri taşı!")
	fmt.Println("  • izle bakiye < 0         : 👁️ (Watchpoint) Koşul sağlandığında veya değişken değiştiğinde dur!")
	fmt.Println("  • kesme 20                : 🔴 20. satıra kesme noktası (Breakpoint) ekle/kaldır")
	fmt.Println("  • tablo liste_adı         : 📋 Liste, Matris veya Sözlüğü tablo görünümünde aç")
	fmt.Println("  • yığın                   : 📚 Çağrı yığınını (Call Stack) göster")
	fmt.Println("  • hesapla x * 2           : 💡 Herhangi bir TR ifadesini anında hesapla")
	fmt.Println("  • çıkış                   : Hata ayıklayıcıyı sonlandır")
}

type varEntry struct {
	Name string
	Val  vm.Value
}

func collectVisibleVariables(env *vm.Environment) []varEntry {
	seen := make(map[string]bool)
	var list []varEntry
	cur := env
	for cur != nil {
		cur.EachLocal(func(k string, v vm.Value) {
			if seen[k] {
				return
			}
			seen[k] = true
			// Yerleşik fonksiyonları ve yapı tanımlarını kalabalık yapmaması için gizle
			if v.Type() == vm.VAL_BUILTIN || v.Type() == vm.VAL_STRUCT || v.Type() == vm.VAL_FUNC {
				return
			}
			list = append(list, varEntry{Name: k, Val: v})
		})
		cur = cur.Outer()
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
	return list
}
