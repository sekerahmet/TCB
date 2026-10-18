// Package ayikla TR'nin hata ayıklayıcısıdır: terminalde çalışan etkileşimli hata
// ayıklayıcı ('tr hata_ayıkla', koddaki kesme_noktası) ve düzenleyicilerin bağlandığı
// hata ayıklama sunucusu ('tr dap', dap*.go). Yükleme anında kendini yorumlayıcıya
// yurut.YeniHataAyiklayici ile kaydeder.
//
// Katman: araç (internal/arac); dil, motor ve hesap katmanlarını içe aktarabilir.
package ayikla

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"tr_dili/internal/dil/agac"
	"tr_dili/internal/dil/ayristir"
	"tr_dili/internal/dil/sozcuk"
	"tr_dili/internal/dil/turkce"
	"tr_dili/internal/dil/yurut"
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

	// pauseKind duraklamanın türüdür (düzenleyiciye bildirilir): "entry", "step",
	// "breakpoint", "data breakpoint", "pause", "goto".
	pauseKind string
	// dap, düzenleyicinin hata ayıklama oturumu (tr dap) etkinse bağlı sunucudur.
	dap *dapServer

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
		Reader:      yurut.StdinOkuyucu(),
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
func (d *Debugger) BeforeStatement(v *yurut.VM, stmt agac.Statement, env *yurut.Environment) int {
	if !d.Active || stmt == nil {
		return 0
	}
	// Blok deyiminin kendisinde değil içindeki satırlarda dur
	if _, isBlock := stmt.(*agac.BlockStatement); isBlock {
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

	// 3. Adım modu (F11 / F10 / Shift+F11) kontrolü
	if !shouldPause {
		switch d.Mode {
		case StepInto:
			shouldPause = true
			if d.PauseReason == "" {
				d.PauseReason = durumAdim
			}
			if d.pauseKind == "" {
				d.pauseKind = "step"
			}
		case StepOver:
			if depth <= d.TargetDepth {
				shouldPause = true
				d.PauseReason = durumUsttenGec
				d.pauseKind = "step"
			}
		case StepOut:
			if depth < d.TargetDepth {
				shouldPause = true
				d.PauseReason = durumFonksiyondanCik
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

func (d *Debugger) checkWatchpoints(v *yurut.VM, env *yurut.Environment) string {
	for _, wp := range d.Watchpoints {
		if wp.IsCond {
			l := sozcuk.New(wp.Expression)
			p := ayristir.New(l)
			prog := p.ParseProgram()
			if len(p.Errors()) == 0 && len(prog.Statements) > 0 {
				if es, ok := prog.Statements[0].(*agac.ExpressionStatement); ok {
					val, err := v.EvalExpression(es.Expression, env)
					if err == nil && val.IsTruthy() {
						curStr := yurut.TRUE.Inspect()
						if wp.LastVal != curStr {
							wp.LastVal = curStr
							return fmt.Sprintf("⚡ İZLEME NOKTASI (Watchpoint #%d): '%s' koşulu sağlandı!", wp.ID, wp.Expression)
						}
					} else {
						wp.LastVal = yurut.FALSE.Inspect()
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

// Adımlama sonrası durum metinleri; terminalde ve düzenleyicide aynıdır.
const (
	durumAdim            = "Adım (F11)"
	durumUsttenGec       = "Üstten geçildi (F10)"
	durumFonksiyondanCik = "Fonksiyondan çıkıldı (Shift+F11)"
)

func (d *Debugger) promptCLI(v *yurut.VM, line int, env *yurut.Environment) int {
	d.renderDebugScreen(v, line, env)
	d.PauseReason = ""

	for {
		fmt.Print("\n[TR Hata Ayıklayıcı] (" + tusRehberi + ") > ")
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

		if yeni, ok := eskiTuslar[cmd]; ok {
			fmt.Printf("  ⚠️ Tuşlar düzenleyicinin hata ayıklama tuşlarıyla aynıdır: F5 devam, F10 üstten geç, F11 içine gir, Shift+F11 dışarı çık. '%s' yerine %s yazın.\n", cmd, yeni)
			continue
		}
		if asil, ok := eskiKomutlar[cmd]; ok {
			fmt.Printf("  ⚠️ '%s' hata ayıklayıcıda bir komut değil; '%s' yazın.\n", cmd, asil)
			continue
		}

		switch cmd {
		case "f11", "adım":
			d.Mode = StepInto
			return 0

		case "f10", "geç":
			d.Mode = StepOver
			d.TargetDepth = v.StackDepth()
			return 0

		case "shift+f11", "dön":
			d.Mode = StepOut
			d.TargetDepth = v.StackDepth()
			return 0

		case "f5", "devam":
			d.Mode = StepContinue
			return 0

		case "değiştir":
			// değiştir bakiye = 5000
			rest := strings.TrimSpace(cmdLine[len(parts[0]):])
			if !strings.Contains(rest, "=") {
				fmt.Println("  ⚠️ Kullanım: değiştir <değişken> = <yeni_değer>  (Örn: değiştir x = 99)")
				continue
			}
			eqParts := strings.SplitN(rest, "=", 2)
			varName := strings.TrimSpace(eqParts[0])
			exprStr := strings.TrimSpace(eqParts[1])
			l := sozcuk.New(exprStr)
			p := ayristir.New(l)
			prog := p.ParseProgram()
			if len(p.Errors()) > 0 || len(prog.Statements) == 0 {
				fmt.Println("  ❌ Geçersiz değer ifadesi.")
				continue
			}
			if es, ok := prog.Statements[0].(*agac.ExpressionStatement); ok {
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

		case "zıpla":
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

		case "kesme":
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

		case "izle":
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

		case "tablo":
			if len(parts) < 2 {
				fmt.Println("  ⚠️ Kullanım: tablo <değişken_adı>")
				continue
			}
			d.inspectTable(parts[1], env)

		case "yığın":
			fmt.Println("  📚 ÇAĞRI YIĞINI:")
			fmt.Printf("    #0 [Aktif] Satır %d\n", line)
			stack := v.StackSnapshot()
			for i := len(stack) - 1; i >= 0; i-- {
				f := stack[i]
				fmt.Printf("    #%d %s (Çağrıldığı Satır: %d)\n", len(stack)-i, f.FuncName, f.Line)
			}

		case "hesapla":
			if len(parts) < 2 {
				fmt.Println("  ⚠️ Kullanım: hesapla <ifade>  (Örn: hesapla x * 2 + 1)")
				continue
			}
			exprStr := strings.TrimSpace(cmdLine[len(parts[0]):])
			l := sozcuk.New(exprStr)
			p := ayristir.New(l)
			prog := p.ParseProgram()
			if len(p.Errors()) == 0 && len(prog.Statements) > 0 {
				if es, ok := prog.Statements[0].(*agac.ExpressionStatement); ok {
					val, err := v.EvalExpression(es.Expression, env)
					if err != nil {
						fmt.Printf("  ❌ Hata: %v\n", err)
					} else {
						fmt.Printf("  💡 Sonuç (%s): %s\n", val.Type(), val.Inspect())
					}
				}
			}

		case "ekran":
			d.renderDebugScreen(v, line, env)

		case "yardım":
			d.printDebugHelp()

		case "çıkış":
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

func (d *Debugger) renderDebugScreen(v *yurut.VM, currentLine int, env *yurut.Environment) {
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
			if _, motor := kv.Val.(IzlemeDegeri); motor && strings.Contains(valPreview, "\n") {
				valPreview = tekSatir(valPreview)
			}
			if r := []rune(valPreview); len(r) > 52 {
				valPreview = string(r[:52]) + "..."
			}
			fmt.Printf("     • %-16s : %-10s = %s\n", kv.Name, "("+string(kv.Val.Type())+")", valPreview)
		}
	}
	fmt.Println("╚══════════════════════════════════════════════════════════════════════════════╝")
}

// tekSatir çok satırlı yazdırma biçimini (ör. tensör) değişken kutusunda tek satıra indirir:
// "Tensör[2×3] [\n  [1, 2, 3],\n  ...\n]" → "Tensör[2×3] [[1, 2, 3], ...]".
func tekSatir(metin string) string {
	satirlar := strings.Split(metin, "\n")
	for i, s := range satirlar {
		satirlar[i] = strings.TrimSpace(s)
	}
	tek := strings.Join(satirlar, " ")
	return strings.NewReplacer("[ ", "[", " ]", "]").Replace(tek)
}

func (d *Debugger) inspectTable(name string, env *yurut.Environment) {
	val, ok := env.Get(name)
	if !ok {
		fmt.Printf("  ❌ %q adında bir değişken bulunamadı.\n", name)
		return
	}
	fmt.Printf("\n  📋 TABLO / NESNE İNCELEYİCİ: %s (%s)\n", name, val.Type())
	fmt.Println("  " + strings.Repeat("─", 56))
	switch t := val.(type) {
	case *yurut.ListVal:
		fmt.Printf("  Toplam Satır Sayısı: %d\n", len(t.Elements))
		fmt.Printf("  %-8s | %-12s | %s\n", "İNDEKS", "TİP", "DEĞER")
		fmt.Println("  " + strings.Repeat("─", 56))
		for i, el := range t.Elements {
			fmt.Printf("  [%-6d] | %-12s | %s\n", i, el.Type(), el.Inspect())
		}
	case IzlemeDegeri:
		// Motor değerleri (ör. tensör): tablo verisi satır satır yazılır.
		cols, rows, err := t.TabloVerisi()
		if err != nil {
			fmt.Printf("  %s\n  Değer: %s\n", t.IzlemeEtiketi(), val.Inspect())
			break
		}
		fmt.Printf("  %s\n", t.IzlemeEtiketi())
		fmt.Printf("  %-9s | %s\n", "SATIR", strings.Join(cols, " | "))
		for i, r := range rows {
			fmt.Printf("  %-9d | %s\n", i, strings.Join(r, " | "))
		}
	case *yurut.DictVal:
		fmt.Printf("  Toplam Alan Sayısı: %d\n", len(t.Order))
		fmt.Printf("  %-18s | %-12s | %s\n", "ANAHTAR", "TİP", "DEĞER")
		fmt.Println("  " + strings.Repeat("─", 56))
		for _, k := range t.Order {
			v := t.Pairs[k]
			fmt.Printf("  %-18s | %-12s | %s\n", t.DisplayKey(k), v.Type(), v.Inspect())
		}
	case *yurut.StructInstanceVal:
		fmt.Printf("  Yapı Türü: %s\n", t.Def.Name)
		for k, v := range t.Fields {
			fmt.Printf("  • %-16s (%s) = %s\n", k, v.Type(), v.Inspect())
		}
	default:
		fmt.Printf("  Değer: %s\n", val.Inspect())
	}
}

type varEntry struct {
	Name string
	Val  yurut.Value
}

func collectVisibleVariables(env *yurut.Environment) []varEntry {
	seen := make(map[string]bool)
	var list []varEntry
	cur := env
	for cur != nil {
		cur.EachLocal(func(k string, v yurut.Value) {
			if seen[k] {
				return
			}
			seen[k] = true
			// Yerleşik fonksiyonları ve yapı tanımlarını kalabalık yapmaması için gizle
			if v.Type() == yurut.VAL_BUILTIN || v.Type() == yurut.VAL_STRUCT || v.Type() == yurut.VAL_FUNC || v.Type() == yurut.VAL_TYPE {
				return
			}
			list = append(list, varEntry{Name: k, Val: v})
		})
		cur = cur.Outer()
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
	return list
}
