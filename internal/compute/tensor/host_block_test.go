package tensor

import (
	"go/ast"
	"go/types"
	"math"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"slices"
	"strings"
	"testing"
	"time"
	"unsafe"
)

// hostSize blok eşiğinin üstünde, q8 bloğunun katı bir öğe sayısıdır (float32'de 2 MiB).
const hostSize = 1 << 19

// needBlock bu platformda blok yoksa testi atlar.
func needBlock(t *testing.T, n int) *HostBlock {
	t.Helper()
	b := NewHostBlock(n)
	if b == nil {
		t.Skip("bu platformda öbek dışı blok yok")
	}
	return b
}

// settleHostMemory çöp toplayıp temizliklerin bitmesini bekler; süreç toplamını verir.
func settleHostMemory(want func(int64) bool) int64 {
	var live int64
	for range 200 {
		runtime.GC()
		live = hostMemory.live.Load()
		if want(live) {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	return live
}

func TestHostBlockSmallStaysOnTheHeap(t *testing.T) {
	if b := NewHostBlock(hostBlockMin - 1); b != nil {
		t.Fatal("eşiğin altındaki tensöre blok verildi")
	}
}

// Blok sıfırlı gelir, sonuna kadar yazılır ve arkasındaki yedek okunabilir.
func TestHostBlockIsZeroedAndWritable(t *testing.T) {
	b := needBlock(t, hostBlockMin+1)
	m := b.Bytes()
	if len(m) != hostBlockMin+1 {
		t.Fatalf("boy %d", len(m))
	}
	for i := range m {
		if m[i] != 0 {
			t.Fatalf("%d. bayt sıfır değil", i)
		}
		m[i] = byte(i)
	}
	if tail := unsafe.Slice(unsafe.SliceData(m), len(m)+hostTail); tail[len(tail)-1] != 0 {
		t.Fatal("yedek bayt sıfır değil")
	}
	runtime.KeepAlive(b)
}

// Blok tutan tensör öbektekiyle aynı değerleri ve bitleri verir, yazılabilir, çarpıma girer.
func TestHostBlockTensorMatchesHeapTensor(t *testing.T) {
	b := needBlock(t, hostSize*4)
	v := b.Float32s()
	want := make([]float32, hostSize)
	for i := range want {
		want[i] = float32(i%97) * 0.25
		v[i] = want[i]
	}
	got := FromHostBlock(b, Float32, 1024, hostSize/1024)
	if !slices.Equal(got.Data(), want) {
		t.Fatal("float32 değerleri farklı")
	}
	heap := New(want, 1024, hostSize/1024)
	x := New(want[:1024], 1, 1024)
	if !slices.Equal(x.MatMul(got).Data(), x.MatMul(heap).Data()) {
		t.Fatal("çarpım farklı")
	}

	h := needBlock(t, hostSize*2)
	bits := h.Uint16s()
	for i := range bits {
		bits[i] = uint16(i) | 0x7F81 // sinyal NaN desenleri dahil: bitler değişmeden gelmeli
	}
	wide := FromHostBlock(h, BFloat16, hostSize)
	if !slices.Equal(wide.Bits16s(), bits) {
		t.Fatal("16 bitlik desen farklı")
	}
}

// Nicemli blok öbekteki bloklarla aynı açılır ve aynı çarpımı verir.
func TestHostBlockQuantMatchesHeap(t *testing.T) {
	elems, size := Q8.QuantBlock()
	n := hostSize * 2
	b := needBlock(t, n/elems*size)
	m := b.Bytes()
	for i := range m {
		m[i] = byte(i * 7)
	}
	for at := 0; at < len(m); at += size { // ölçek 1: 0x3C00, sonlu
		m[at], m[at+1] = 0, 0x3C
	}
	heapRaw := slices.Clone(m)
	shape := []int{n / 1024, 1024}
	got, heap := FromHostBlock(b, Q8, shape...), FromQuantized(Q8, heapRaw, shape...)
	x := New(make([]float32, 1024), 1, 1024)
	if !slices.Equal(got.Open(Float32).Data(), heap.Open(Float32).Data()) {
		t.Fatal("açılan değerler farklı")
	}
	if !slices.Equal(x.MatMul(got.Transpose(0, 1)).Data(), x.MatMul(heap.Transpose(0, 1)).Data()) {
		t.Fatal("çarpım farklı")
	}
	if got.To(got.Device()) != got {
		t.Fatal("aynı cihaza taşıma yeni depo kurdu")
	}
	rows := []int{3, 1, 2}
	if !slices.Equal(got.quantRows(rows, []int{3, 1024}).Raw(), heap.quantRows(rows, []int{3, 1024}).Raw()) {
		t.Fatal("satır seçimi farklı")
	}
}

// Çevrilmiş kopya da bloktadır: 16 bitlik ağırlık float32'ye açılınca öbeğe düşmez.
func TestHostBlockCastStaysOffTheHeap(t *testing.T) {
	b := needBlock(t, hostSize*2)
	bits := b.Uint16s()
	for i := range bits {
		bits[i] = f32ToBF16(float32(i % 251))
	}
	src := FromHostBlock(b, BFloat16, hostSize)
	got := src.Cast(Float32)
	if s := got.storage.(*cpuStorage); s.block == nil || s.block == b {
		t.Fatal("çevrilmiş kopya kendi bloğunda değil")
	}
	want := make([]float32, hostSize)
	for i := range want {
		want[i] = float32(i % 251)
	}
	if !slices.Equal(got.Data(), want) {
		t.Fatal("çevrilmiş değerler farklı")
	}
	if s := Zeros(hostSize).Cast(BFloat16).storage.(*cpuStorage); s.block != nil {
		t.Fatal("öbekteki tensörün kopyası bloğa gitti")
	}
}

// withHostLimits bütçeyi test boyunca verilen sınırlara indirir.
func withHostLimits(t *testing.T, blocks, address int64) {
	t.Helper()
	old := hostLimits
	hostLimits.blocks = func() int64 { return blocks }
	hostLimits.address = address
	t.Cleanup(func() { hostLimits = old })
}

// Bütçe dolunca yeni blok verilmez; çevirme öbekte sürer, değerler aynı kalır (DENETİM 236: blokta
// duran tensörün her Cast'i geri gelmeyen adres harcıyordu).
func TestHostBlockBudgetEndsInTheHeap(t *testing.T) {
	b := needBlock(t, hostSize*2)
	bits := b.Uint16s()
	for i := range bits {
		bits[i] = f32ToBF16(float32(i % 251))
	}
	src := FromHostBlock(b, BFloat16, hostSize)
	want := src.Cast(Float32).Data()
	blocks, address := hostMemory.blocks.Load(), hostMemory.address.Load()
	cases := []struct {
		name            string
		blocks, address int64
	}{
		{"blok sayısı", blocks, hostAddressSpace},
		{"adres", math.MaxInt64, address + hostSize*4 - 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			withHostLimits(t, c.blocks, c.address)
			if NewHostBlock(hostSize*4) != nil {
				t.Fatal("bütçe dolu, blok verildi")
			}
			got := src.Cast(Float32)
			if got.storage.(*cpuStorage).block != nil {
				t.Fatal("bütçe dolu, çevirme bloğa gitti")
			}
			if !slices.Equal(got.Data(), want) {
				t.Fatal("öbekteki çevirmenin değerleri farklı")
			}
		})
	}
	withHostLimits(t, hostMemory.blocks.Load()+1, hostAddressSpace)
	spent := hostMemory.address.Load()
	if NewHostBlock(hostSize*4) == nil {
		t.Fatal("bütçede yer varken blok verilmedi")
	}
	if got := hostMemory.address.Load() - spent; got != hostSize*4 {
		t.Fatalf("harcanan adres %d, beklenen %d", got, hostSize*4)
	}
	if NewHostBlock(hostSize*4) != nil {
		t.Fatal("son yer harcandı, blok verildi")
	}
}

// Sınır işletim sisteminden gelir, yarısı Go'ya kalır; dosya okunamazsa çekirdek varsayılanı sayılır.
func TestHostBlockLimitComesFromTheSystem(t *testing.T) {
	needBlock(t, hostSize*2)
	if hostLimits.blocks() <= 0 {
		t.Fatalf("blok sınırı %d", hostLimits.blocks())
	}
	if hostLimits.address != hostAddressSpace/2 {
		t.Fatalf("adres sınırı %d", hostLimits.address)
	}
	for text, want := range map[string]int64{"262144\n": 262144, "65530": 65530, "": hostDefaultMaps, "yok": hostDefaultMaps, "-1": hostDefaultMaps} {
		if got := hostMapCount(text); got != want {
			t.Fatalf("%q: %d, beklenen %d", text, got, want)
		}
	}
}

// Havuz bloğu tutmaz: iade edilen depo bir sonraki ayırmada yeniden verilmez.
func TestHostBlockStaysOutOfThePool(t *testing.T) {
	b := needBlock(t, hostSize*4)
	s := b.storage(Float32, hostSize)
	cpuBackend{}.Release(s)
	if s.release {
		t.Fatal("blok deposu havuza girdi")
	}
}

// Erişilmez olan blok geri döner; ölü bloklar çöp toplayıcıyı kendisi çağırır, öbek küçük olsa da
// toplam sınırlı kalır.
func TestHostBlockIsReleasedAndPressureBounds(t *testing.T) {
	needBlock(t, hostBlockMin)
	base := settleHostMemory(func(n int64) bool { return n == 0 })
	if base != 0 {
		t.Fatalf("başlangıçta %d bayt canlı", base)
	}
	const size = 64 << 20
	peak := int64(0)
	for range 64 { // 4 GiB ayrılır; çoğu eşlenir ama dokunulmaz
		b := NewHostBlock(size)
		peak = max(peak, hostMemory.live.Load())
		runtime.KeepAlive(b)
		time.Sleep(time.Millisecond)
	}
	if peak > 8*hostPressureFloor {
		t.Fatalf("ölü bloklar birikti: tepe %d bayt", peak)
	}
	if n := settleHostMemory(func(n int64) bool { return n == 0 }); n != 0 {
		t.Fatalf("toplamadan sonra %d bayt canlı", n)
	}
}

// Bırakılan bloğun adresi hiçbir kipte yeni bloğa gitmez; eski dilim yanlış değer değil, hata görür.
func TestHostBlockReleaseKeepsTheAddress(t *testing.T) {
	seen := map[*byte]bool{}
	for range 50 {
		b := needBlock(t, hostBlockMin)
		p := unsafe.SliceData(b.Bytes())
		if seen[p] {
			t.Fatal("bırakılan adres yeni bloğa verildi")
		}
		seen[p] = true
		b = nil
		settleHostMemory(func(n int64) bool { return n == 0 })
	}
}

//go:noinline
func releasedSlice(t *testing.T) []byte {
	b := needBlock(t, hostBlockMin)
	b.Bytes()[7] = 9
	return b.Bytes()
}

func TestHostBlockReadAfterReleaseFaults(t *testing.T) {
	stale := releasedSlice(t)
	if n := settleHostMemory(func(n int64) bool { return n == 0 }); n != 0 {
		t.Fatalf("blok bırakılmadı: %d bayt", n)
	}
	defer debug.SetPanicOnFault(debug.SetPanicOnFault(true))
	defer func() {
		if recover() == nil {
			t.Fatal("bırakılan bloktan okuma sessizce geçti")
		}
	}()
	t.Log(stale[7])
}

// keepProbe çekirdek çalışırken çöp toplar: kapı depoyu canlı tutmuyorsa blok çekirdeğin altından
// geri dönerdi.
type keepProbe struct {
	cpuBackend
	live int64
}

func (p *keepProbe) Copy(Storage, int, View) {
	p.live = settleHostMemory(func(n int64) bool { return false })
}

//go:noinline
func probeGate(t *testing.T, p *keepProbe, n int) {
	b := needBlock(t, n)
	s := b.storage(Float32, n/4)
	dst := &cpuStorage{d: make([]float32, 1)}
	gateCopy(p, dst, 0, View{S: s, Shape: []int{n / 4}, Strides: []int{1}})
}

func TestGateKeepsTheBlockDuringTheCall(t *testing.T) {
	settleHostMemory(func(n int64) bool { return n == 0 })
	p := &keepProbe{}
	probeGate(t, p, hostSize*4)
	if p.live != hostSize*4 {
		t.Fatalf("çekirdek çalışırken canlı %d bayt, %d olmalıydı", p.live, hostSize*4)
	}
}

// Kural (host_block.go; DENETİM 216 Ş2): depoyu dilimiyle okuyan her yer sahibini çağrı bitene dek canlı
// tutar. Kapıda her depo ve görünüm parametresinin KeepAlive'ı vardır. Okuyucu adıyla tanımlıdır: cd,
// cpuStore, quantBytes, quantRaw ya da .(*cpuStorage) geçen işlev. Arka uç yöntemleri kapının altındadır;
// yalnız onlardan çağrılan yardımcı da öyle. Geri kalanı KeepAlive çağırır, yeni yol testi düşürür.
func TestHostBlockReadersKeepTheirTensors(t *testing.T) {
	fset, files, info, pkg := checkTensorPackage(t)
	backend := pkg.Scope().Lookup("Backend").Type().Underlying().(*types.Interface)
	underGate := func(fn *ast.FuncDecl) bool {
		if fn.Recv == nil {
			return false
		}
		r := derefType(info.Types[fn.Recv.List[0].Type].Type)
		return types.Implements(r, backend) || types.Implements(types.NewPointer(r), backend)
	}
	var funcs []*ast.FuncDecl // tensör başlığının yöntemleri dışındakiler: yardımcı adayları
	callers := map[types.Object][]*ast.FuncDecl{}
	var all []*ast.FuncDecl
	gates := 0
	for _, f := range files {
		name := filepath.Base(fset.Position(f.Pos()).Filename)
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			all = append(all, fn)
			if fn.Recv == nil || exprText(fn.Recv.List[0].Type) != "*Tensor" && !underGate(fn) {
				funcs = append(funcs, fn)
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				if id, ok := n.(*ast.Ident); ok {
					if f, ok := info.Uses[id].(*types.Func); ok {
						callers[f.Origin()] = append(callers[f.Origin()], fn)
					}
				}
				return true
			})
			if name == "gate_calls.go" && strings.HasPrefix(fn.Name.Name, "gate") {
				gates++
				kept := keepAlives(fn.Body)
				for _, p := range fn.Type.Params.List {
					for _, id := range p.Names {
						want := ""
						switch exprText(p.Type) {
						case "Storage", "*View", "[]View", "[]Storage":
							want = id.Name
						case "View":
							want = id.Name + ".S"
						}
						if want != "" && !kept[want] {
							t.Errorf("%s: %s canlı tutulmuyor", fn.Name.Name, want)
						}
					}
				}
			}
		}
	}
	if gates < 50 {
		t.Fatalf("yalnız %d kapı bulundu", gates)
	}
	// Yalnız kapının altından çağrılan yardımcılar: başta hepsi aday, dışarıdan çağrılan elenir.
	helper := map[*ast.FuncDecl]bool{}
	for _, fn := range funcs {
		helper[fn] = len(callers[info.Defs[fn.Name]]) > 0
	}
	for changed := true; changed; {
		changed = false
		for _, fn := range funcs {
			if !helper[fn] {
				continue
			}
			for _, c := range callers[info.Defs[fn.Name]] {
				if c != fn && !underGate(c) && !helper[c] {
					helper[fn], changed = false, true
					break
				}
			}
		}
	}
	readers := 0
	for _, fn := range all {
		name := fn.Name.Name
		// hostTarget dilime değil bloğun kendisine bakar.
		if underGate(fn) || helper[fn] || slices.Contains([]string{"cd", "cpuStore", "quantBytes", "quantRaw", "hostTarget"}, name) {
			continue
		}
		if !readsStorage(fn.Body) {
			continue
		}
		readers++
		if len(keepAlives(fn.Body)) == 0 {
			t.Errorf("%s: %s depoyu okuyor, sahibi canlı tutulmuyor", fset.Position(fn.Pos()), name)
		}
	}
	if readers < 6 {
		t.Fatalf("yalnız %d okuyucu bulundu; tarama kapı dışı okumaları görmüyor", readers)
	}
}

// Kural (host_block.go; DENETİM 216 N1): deponun dilimi bloğu olmayan başka depoya geçmez, yoksa ilk
// depo ölünce ikincisi erişime kapalı belleğe bakar. quantTo'nun kaynağı CPU'daysa hedefi karttır: To aynı
// cihazda tensörün kendisini döndürür (TestHostBlockQuantMatchesHeap).
func TestHostBlockSlicesStayWithTheirBlock(t *testing.T) {
	fset, files, _, _ := checkTensorPackage(t)
	literals := 0
	for _, f := range files {
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				switch n := n.(type) {
				case *ast.CompositeLit:
					if exprText(n.Type) != "cpuStorage" {
						return true
					}
					literals++
					owned := false
					var taken []ast.Expr
					for _, e := range n.Elts {
						kv := e.(*ast.KeyValueExpr)
						switch exprText(kv.Key) {
						case "block":
							owned = true
						case "d", "h", "raw":
							taken = append(taken, kv.Value)
						}
					}
					for _, v := range taken {
						if !owned && borrowsStorage(v) {
							t.Errorf("%s: %s başka deponun dilimini bloksuz alıyor", fset.Position(v.Pos()), fn.Name.Name)
						}
					}
				case *ast.CallExpr:
					if exprText(n.Fun) == "quantStorage" && fn.Name.Name != "quantTo" && slices.ContainsFunc(n.Args, borrowsStorage) {
						t.Errorf("%s: %s deponun dilimini quantStorage'a veriyor", fset.Position(n.Pos()), fn.Name.Name)
					}
				}
				return true
			})
		}
	}
	if literals < 10 {
		t.Fatalf("yalnız %d depo kurulumu bulundu", literals)
	}
}

// borrowsStorage ifade var olan bir deponun dilimine bakıyor mu.
func borrowsStorage(e ast.Expr) bool {
	found := false
	ast.Inspect(e, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.SelectorExpr:
			if slices.Contains([]string{"raw", "d", "h", "quantRaw", "mem", "Bytes", "Float32s", "Uint16s"}, n.Sel.Name) {
				found = true
			}
		case *ast.Ident:
			if n.Name == "quantBytes" || n.Name == "cd" {
				found = true
			}
		}
		return true
	})
	return found
}

// keepAlives gövdedeki KeepAlive çağrılarının argümanları.
func keepAlives(body *ast.BlockStmt) map[string]bool {
	out := map[string]bool{}
	ast.Inspect(body, func(n ast.Node) bool {
		if c, ok := n.(*ast.CallExpr); ok {
			if se, ok := c.Fun.(*ast.SelectorExpr); ok && se.Sel.Name == "KeepAlive" {
				out[exprText(c.Args[0])] = true
			}
		}
		return true
	})
	return out
}

// readsStorage gövde deponun dilimine bakıyor mu.
func readsStorage(body *ast.BlockStmt) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.CallExpr:
			switch exprText(n.Fun) {
			case "cd", "cpuStore", "quantBytes", "t.quantRaw":
				found = true
			}
		case *ast.TypeAssertExpr:
			if exprText(n.Type) == "*cpuStorage" {
				found = true
			}
		}
		return true
	})
	return found
}

// exprText basit bir ifadenin kaynak yazımı.
func exprText(e ast.Expr) string {
	switch e := e.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		return exprText(e.X) + "." + e.Sel.Name
	case *ast.StarExpr:
		return "*" + exprText(e.X)
	case *ast.ArrayType:
		return "[]" + exprText(e.Elt)
	}
	return ""
}
