package yurut

import (
	"fmt"
	"tr_dili/internal/dil/agac"
	"tr_dili/internal/dil/kod"
)

// registerTestBuiltins test yazmak için doğrulama yerleşiklerini kaydeder.
// İkisi de test blokları dışında da çalışır (bir tür "assert").
func (v *VM) registerTestBuiltins() {
	// doğrula(koşul, [mesaj]): koşul yanlışsa hata fırlatır.
	v.AdlisizKaydet(kod.G700, func(y *VM, args []Value, named map[string]Value) (Value, error) {
		if len(args) < 1 || len(args) > 2 {
			return nil, fmt.Errorf("doğrula() 1 ya da 2 parametre bekler (koşul, [mesaj]), %d verildi", len(args))
		}
		if args[0].IsTruthy() {
			return TRUE, nil
		}
		if len(args) == 2 {
			return nil, fmt.Errorf("doğrulama başarısız: %s", args[1].Inspect())
		}
		return nil, fmt.Errorf("doğrulama başarısız: koşul yanlış (%s)", testGoster(args[0]))
	})

	// eşit_mi(beklenen, gerçek, [mesaj]): iki değer '==' ile eşit değilse ikisini de gösteren hata fırlatır.
	v.AdlisizKaydet(kod.G701, func(y *VM, args []Value, named map[string]Value) (Value, error) {
		if len(args) < 2 || len(args) > 3 {
			return nil, fmt.Errorf("eşit_mi() 2 ya da 3 parametre bekler (beklenen, gerçek, [mesaj]), %d verildi", len(args))
		}
		if ValuesEqual(args[0], args[1]) {
			return TRUE, nil
		}
		ek := ""
		if len(args) == 3 {
			ek = args[2].Inspect() + " — "
		}
		return nil, fmt.Errorf("eşitlik başarısız: %sbeklenen %s, gerçek %s", ek, testGoster(args[0]), testGoster(args[1]))
	})
}

// testGoster bir değeri tipiyle birlikte gösterir; metinler tırnak içinde yazılır ki
// "5" ile 5 ayırt edilebilsin.
func testGoster(d Value) string {
	if s, ok := d.(*StringVal); ok {
		return fmt.Sprintf("%q (Metin)", s.Val)
	}
	return fmt.Sprintf("%s (%s)", d.Inspect(), d.Type())
}

// RunTest tek bir test bloğunu, dosyanın üst düzey tanımları (GlobalEnv) üzerinde açılan
// yeni bir kapsamda çalıştırır. Gövdedeki 'ertele' blokları test bitince çalışır ve testin
// başlattığı 'eşzamanlı' görevler beklenir. Hata (doğrulama dahil) dönerse test kalmıştır.
// Programın Run ile yüklenmiş olması beklenir.
func (v *VM) RunTest(t *agac.TestStatement) (err error) {
	if !v.holdsGIL {
		v.acquireGIL()
		defer v.releaseGIL()
	}
	yiginBoyu := len(v.CallStack)
	defer func() {
		if r := recover(); r != nil {
			// Çöküşte yarıda kalan çağrıların çerçevelerini at (sonraki testler için)
			v.yiginiKirp(yiginBoyu)
			if KesilmeMi(r) {
				err = ErrKesildi
				return
			}
			err = fmt.Errorf("Satır %d: İç hata (çökme önlendi): %v", t.Line(), r)
		}
	}()
	var stmts []agac.Statement
	if t.Body != nil {
		stmts = t.Body.Statements
	}
	kd, bilgi := v.genelAltindaDerle(stmts)
	env := yeniOrtam(v.GlobalEnv, bilgi)
	env.IsFuncRoot = true
	func() {
		// Testin ertelenen işleri test bitince çalışır; hataları testin hatasına eklenir (9.4)
		defer func() { err = erteleHatasiEkle(err, env.RunDefers()) }()
		if t.Body != nil {
			_, _, err = kd.calistir(v, env)
		}
	}()
	v.Blocking(v.wg.Wait)
	return err
}
