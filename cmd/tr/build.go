package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/sekerahmet/TCB/internal/lang/checker"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/source"
	"os"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
)

// 'tr derle' ile üretilen uygulama, yorumlayıcı ikilisinin sonuna eklenmiş bir paketten
// oluşur. İki biçim vardır:
//
//	Sürüm 1 (eski, yalnızca okunur): [kaynak kod][kod uzunluğu u64][oldPackageSignature]
//	Sürüm 2 ve sonrası:            [JSON paket][paket uzunluğu u64][sürüm u32][packageSignature]
//
// Tüm sayılar küçük uçludur (little endian). Sürüm 2 paketinde ana programın yanında
// onun 'aktar' ile aldığı .tr modülleri de (özyinelemeli olarak) bulunur.
const (
	oldPackageSignature        = "TR_EXE_1"
	packageSignature           = "TR_PAKET"
	packageVersion      uint32 = 2

	oldFooterSize = 16 // uzunluk + imza
	footerSize    = 20 // uzunluk + sürüm + imza
)

// embeddedSourceName gömülü uygulamanın ana programına verilen addır. VM bu adı görünce
// modülleri ana programın klasörüne göre aramaz; gömülü uygulamada her 'aktar' zaten
// gömülü modülün mutlak yoluna yönlendirilir.
var embeddedSourceName = messages.Text(messages.H8638)

// packageFile pakete gömülü tek bir .tr kaynağıdır.
type packageFile struct {
	// Name derleme sırasında dosyanın çözüldüğü yoldur (yalnızca bilgi ve geçici dosya adı için).
	Name string `json:"ad"`
	Code string `json:"kod"`
	// Imports dosyadaki 'aktar' deyimlerinin, kaynaktaki sırayla, hangi gömülü modüle
	// (Modules dizisindeki sıra) gittiğini söyler.
	Imports []int `json:"aktarimlar,omitempty"`
}

// pkg gömülü uygulamanın tamamıdır.
type pkg struct {
	Main    packageFile   `json:"ana"`
	Modules []packageFile `json:"moduller,omitempty"`
}

// encodePackage paketi altbilgisiyle birlikte ikilinin sonuna eklenecek baytlara çevirir.
func encodePackage(pk *pkg) ([]byte, error) {
	content, err := json.Marshal(pk)
	if err != nil {
		return nil, err
	}
	footer := make([]byte, footerSize)
	binary.LittleEndian.PutUint64(footer[0:8], uint64(len(content)))
	binary.LittleEndian.PutUint32(footer[8:12], packageVersion)
	copy(footer[12:20], packageSignature)
	return append(content, footer...), nil
}

// extractPackage ikilinin sonundaki paketi çözer. bulundu, sonda bir paket imzası olup
// olmadığını; templateSize, paket atıldığında geriye kalan yorumlayıcının bayt sayısını
// verir. İmza var ama paket bozuksa ya da desteklenmeyen bir sürümse hata döner.
func extractPackage(exe []byte) (pk *pkg, templateSize int, found bool, err error) {
	n := len(exe)
	switch {
	case n >= footerSize && string(exe[n-8:]) == packageSignature:
		length := binary.LittleEndian.Uint64(exe[n-20 : n-12])
		version := binary.LittleEndian.Uint32(exe[n-12 : n-8])
		if length > uint64(n-footerSize) {
			return nil, 0, true, messages.New(messages.H8750)
		}
		start := n - footerSize - int(length)
		if version < 2 || version > packageVersion {
			return nil, start, true, messages.New(messages.H8751, version, packageVersion)
		}
		pk = &pkg{}
		if err := json.Unmarshal(exe[start:n-footerSize], pk); err != nil {
			return nil, start, true, messages.New(messages.H8752, err)
		}
		return pk, start, true, nil
	case n >= oldFooterSize && string(exe[n-8:]) == oldPackageSignature:
		// Sürüm 1: modülsüz, yalnızca ana programın kaynağı.
		length := binary.LittleEndian.Uint64(exe[n-16 : n-8])
		if length == 0 || length > uint64(n-oldFooterSize) {
			return nil, 0, true, messages.New(messages.H8753)
		}
		start := n - oldFooterSize - int(length)
		code := string(exe[start : n-oldFooterSize])
		return &pkg{Main: packageFile{Code: code}}, start, true, nil
	}
	return nil, n, false, nil
}

// findEmbeddedPackage çalışan ikilinin sonunda bir uygulama paketi arar.
func findEmbeddedPackage() (*pkg, bool, error) {
	exePath, err := os.Executable()
	if err != nil {
		return nil, false, nil
	}
	f, err := os.Open(exePath)
	if err != nil {
		return nil, false, nil
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, false, nil
	}
	// Önce yalnızca son 8 bayta (imza) bakılır.
	size := info.Size()
	if size < oldFooterSize {
		return nil, false, nil
	}
	signature := make([]byte, 8)
	if _, err := f.ReadAt(signature, size-8); err != nil {
		return nil, false, nil
	}
	// İmza varsa yalnızca altbilgi ve paket okunur (yorumlayıcının kendisi değil).
	var footer int64
	switch string(signature) {
	case packageSignature:
		footer = footerSize
	case oldPackageSignature:
		footer = oldFooterSize
	default:
		return nil, false, nil
	}
	if size < footer {
		return nil, true, messages.New(messages.H8754)
	}
	lengthBytes := make([]byte, 8)
	if _, err := f.ReadAt(lengthBytes, size-footer); err != nil {
		return nil, true, err
	}
	length := binary.LittleEndian.Uint64(lengthBytes)
	if length > uint64(size-footer) {
		return nil, true, messages.New(messages.H8750)
	}
	tail := make([]byte, int64(length)+footer)
	if _, err := f.ReadAt(tail, size-int64(len(tail))); err != nil {
		return nil, true, err
	}
	pk, _, found, err := extractPackage(tail)
	return pk, found, err
}

// importSite bir 'aktar' deyiminin modül yolunu yazan sözcük biriminin kaynaktaki
// karakter (rune) aralığıdır.
type importSite struct {
	start, end int
	filePath   string
	ln         int
}

// importSites koddaki tüm 'aktar' deyimlerini kaynaktaki sırayla bulur. Ayrıştırıcı da
// 'aktar' sözcüğünden sonraki ilk sözcük biriminin metnini modül yolu sayar; burada da
// aynısı yapılır, böylece iç bloklardaki aktarımlar da kaçmaz. Aralıklar kaynağın gövdesine
// göredir (source.Parts; kodlu dosyada başlıktan sonrası).
func importSites(code string) []importSite {
	b, _ := source.Split(code)
	l := b.Lexer()
	var sites []importSite
	for {
		tok := l.NextToken()
		if tok.Type == lexer.TokEOF {
			return sites
		}
		if tok.Type != lexer.TokImport {
			continue
		}
		pathToken := l.NextToken()
		if pathToken.Type == lexer.TokEOF {
			return sites
		}
		start, end := l.LastRange()
		sites = append(sites, importSite{start: start, end: end, filePath: pathToken.Literal, ln: tok.Line})
	}
}

// resolveModulePath bir 'aktar' yolunu yorumlayıcıyla aynı kuralla (yurut.ModulYolunuCoz) çözer:
// önce aktaran dosyanın klasörüne, bulunamazsa ana programın klasörüne göre (11.1). aranan,
// failure iletisinde gösterilecek denenmiş yollardır. Dilin kitaplığındaki (tr/ önekli) modüller
// diskte aranmaz; onları çağıran yurut.KitaplikModulu ile ayırır.
func resolveModulePath(filePath, importer, mainFile string) (resolved string, wanted []string, ok bool) {
	if !strings.HasSuffix(filePath, ".tr") {
		filePath += ".tr"
	}
	return interp.ResolveModulePath(filePath, interp.FileDirs(importer, mainFile))
}

// libraryImport pakette dilin gömülü kitaplığına (11.6) giden aktarımın işaretidir: bu
// modüller pakete gömülmez (uygulama zaten kitaplığı içeren yorumlayıcının kopyasıdır) ve
// yolları yönlendirilmez.
const libraryImport = -1

// moduleCollector derleme sırasında programın aktardığı modülleri özyinelemeli toplar.
type moduleCollector struct {
	index    map[string]int // mutlak yol → Moduller içindeki sıra
	modules  []packageFile
	mainFile string // ana programın yolu
	// globalNames ana programın genel kapsama tanımladığı adlardır: modüller genel kapsamın
	// altında çalışır ve bu adları görür (11.1). globalDynamic, ana programda takma adsız
	// 'aktar' olduğu için adların tam bilinmediğini söyler.
	globalNames   []string
	globalDynamic bool
}

// collect dosyanın 'aktar' deyimlerini çözer, henüz gömülmemiş modülleri okuyup pakete
// ekler ve dosyanın Imports alanını doldurur. sourcePath, dosyanın yorumlayıcıda
// alacağı SourceFile değeridir (modül yolları buna göre çözülür).
func (t *moduleCollector) collect(file *packageFile, sourcePath string) error {
	sites := importSites(file.Code)
	file.Imports = make([]int, 0, len(sites))
	for _, y := range sites {
		if name, ok := interp.LibraryModule(y.filePath); ok {
			if _, err := interp.ReadLibrary(name); err != nil {
				return messages.New(messages.H8755, sourcePath, y.ln, err)
			}
			file.Imports = append(file.Imports, libraryImport)
			continue
		}
		resolved, wanted, ok := resolveModulePath(y.filePath, sourcePath, t.mainFile)
		if !ok {
			return messages.New(messages.H8756, sourcePath, y.ln, y.filePath, strings.Join(wanted, ", "))
		}
		absolute, err := filepath.Abs(resolved)
		if err != nil {
			absolute = resolved
		}
		if i, exists := t.index[absolute]; exists {
			file.Imports = append(file.Imports, i)
			continue
		}
		content, err := os.ReadFile(resolved)
		if err != nil {
			return messages.New(messages.H8757, sourcePath, y.ln, y.filePath, err)
		}
		module := packageFile{Name: filepath.ToSlash(resolved), Code: string(content)}
		// Modül yorumlayıcıdaki 'aktar' ile aynı denetimden geçer (sözdizimi, üst düzey
		// 'sonuç', tip denetimi).
		if s := interp.CheckModule(module.Code, t.globalNames, t.globalDynamic); s.Stage != "" {
			return &moduleCheckError{filePath: filepath.ToSlash(resolved), code: s.Text, kind: s.Stage, diags: s.Errors}
		}
		// Sıra özyinelemeden önce ayrılır; döngüsel aktarımlar böylece aynı sırayı bulur
		// (döngünün kendisini yorumlayıcı çalışma anında bildirir).
		i := len(t.modules)
		t.index[absolute] = i
		t.modules = append(t.modules, packageFile{})
		if err := t.collect(&module, resolved); err != nil {
			return err
		}
		t.modules[i] = module
		file.Imports = append(file.Imports, i)
	}
	return nil
}

// moduleCheckError bir modülde sözdizimi ya da tip denetimi hatası bulunduğunu bildirir.
type moduleCheckError struct {
	filePath string
	code     string
	kind     string // source.StageSyntax ya da source.StageType
	diags    []error
}

func (e *moduleCheckError) Error() string {
	return messages.Text(messages.H8758, e.filePath, e.kind, e.diags[0])
}

// MessageText hatanın lang dilindeki metnidir (messages.Localized).
func (e *moduleCheckError) MessageText(lang string) string {
	return messages.TextIn(lang, messages.H8758, e.filePath, e.kind, e.diags[0])
}

// preparePackage ana programı ve aktardığı tüm modülleri içeren paketi kurar.
func preparePackage(srcPath, code string) (*pkg, error) {
	t := &moduleCollector{index: map[string]int{}, mainFile: srcPath}
	t.globalNames, t.globalDynamic = checker.TopLevelNames(source.Prepare(code, source.Options{}).Program)
	pk := &pkg{Main: packageFile{Name: filepath.ToSlash(srcPath), Code: code}}
	if err := t.collect(&pk.Main, srcPath); err != nil {
		return nil, err
	}
	pk.Modules = t.modules
	return pk, nil
}

// processorNote, yorumlayıcı belirli bir x86-64 düzeyi için derlenmişse (GOAMD64=v3/v4)
// üretilen uygulamanın da aynı işlemci özelliklerini isteyeceğini söyleyen satırı verir.
func processorNote() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	var arch, level string
	for _, s := range info.Settings {
		switch s.Key {
		case "GOARCH":
			arch = s.Value
		case "GOAMD64":
			level = s.Value
		}
	}
	return processorNoteText(arch, level)
}

// processorNoteText processorNote'nun derleme ayarlarından bağımsız (sınanabilir) bölümüdür.
func processorNoteText(arch, level string) string {
	if arch != "amd64" {
		return ""
	}
	switch level {
	case "v3":
		return "⚠️  " + messages.Text(messages.H8759)
	case "v4":
		return "⚠️  " + messages.Text(messages.H8760)
	}
	return ""
}

// compileToExe 'tr derle' komutudur: kaynağı ve aktardığı modülleri denetleyip çalışan
// yorumlayıcının bir kopyasının sonuna paket olarak ekler.
func compileToExe(srcPath string, outPath string) {
	code, _ := prepareFile(srcPath, buildPreparation)
	pk := checkModules(srcPath, code, buildPreparation, "❌ "+messages.Text(messages.H8761))

	if outPath == "" {
		base := strings.TrimSuffix(srcPath, filepath.Ext(srcPath))
		outPath = base + ".exe"
	}
	if err := checkOutputPath(srcPath, outPath); err != nil {
		fmt.Fprintf(os.Stderr, "❌ %s %v\n", messages.Text(messages.H8761), err)
		os.Exit(1)
	}

	exePath, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, messages.Text(messages.H8762, err))
		os.Exit(1)
	}
	baseExeBytes, err := os.ReadFile(exePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, messages.Text(messages.H8763, err))
		os.Exit(1)
	}
	// Şablonun sonunda zaten bir paket varsa (her iki biçim) atılır.
	if _, templateSize, found, _ := extractPackage(baseExeBytes); found && templateSize > 0 {
		baseExeBytes = baseExeBytes[:templateSize]
	}

	packageBytes, err := encodePackage(pk)
	if err != nil {
		fmt.Fprintln(os.Stderr, "❌ "+messages.Text(messages.H8764, err))
		os.Exit(1)
	}
	outBuf := make([]byte, 0, len(baseExeBytes)+len(packageBytes))
	outBuf = append(outBuf, baseExeBytes...)
	outBuf = append(outBuf, packageBytes...)

	// Önce geçici dosyaya yazılır, sonra yeniden adlandırılır: yazma yarıda kalırsa var
	// olan bir dosya bozulmaz.
	temporary := outPath + ".yaziliyor"
	if err := os.WriteFile(temporary, outBuf, 0755); err != nil {
		os.Remove(temporary)
		fmt.Fprintln(os.Stderr, "❌ "+messages.Text(messages.H8765, err))
		os.Exit(1)
	}
	if err := os.Rename(temporary, outPath); err != nil {
		os.Remove(temporary)
		fmt.Fprintln(os.Stderr, "❌ "+messages.Text(messages.H8765, err))
		os.Exit(1)
	}

	fmt.Println("🚀 " + messages.Text(messages.H8766, outPath))
	if len(pk.Modules) > 0 {
		names := make([]string, len(pk.Modules))
		for i, m := range pk.Modules {
			names[i] = m.Name
		}
		fmt.Println("📦 " + messages.Text(messages.H8767, len(names), strings.Join(names, ", ")))
	}
	if not := processorNote(); not != "" {
		fmt.Println(not)
	}
}

// checkOutputPath 'tr derle'nin çıktı yolunun bir kaynak dosyayı ezmeyeceğini denetler:
// çıktı kaynağın kendisi (aynı dosya) ya da .tr uzantılı bir dosya olamaz.
func checkOutputPath(srcPath, outPath string) error {
	if strings.EqualFold(filepath.Ext(outPath), ".tr") {
		return messages.New(messages.H8768, outPath, strings.TrimSuffix(outPath, filepath.Ext(outPath))+".exe")
	}
	same := false
	if a, err := filepath.Abs(srcPath); err == nil {
		if b, err := filepath.Abs(outPath); err == nil && strings.EqualFold(filepath.Clean(a), filepath.Clean(b)) {
			same = true
		}
	}
	if srcInfo, err := os.Stat(srcPath); err == nil {
		if outInfo, err := os.Stat(outPath); err == nil && os.SameFile(srcInfo, outInfo) {
			same = true
		}
	}
	if same {
		return messages.New(messages.H8769, outPath)
	}
	return nil
}

// stringLiteral bir yolu TR metin sabiti olarak yazar.
func stringLiteral(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}

// redirectImports dosyadaki her 'aktar' yolunu, gömülü modülün geçici klasöre yazılmış
// kopyasının mutlak yoluyla değiştirir. Değişiklik satır içinde kaldığı için satır
// numaraları (hata iletileri) aynı kalır. Mutlak yol, yorumlayıcının bulunulan klasördeki
// aynı adlı bir dosyayı gömülü modülün önüne koymasını da engeller.
func redirectImports(d packageFile, paths []string) (string, error) {
	sites := importSites(d.Code)
	if len(sites) != len(d.Imports) {
		return "", messages.New(messages.H8770, d.Name, len(sites), len(d.Imports))
	}
	if len(sites) == 0 {
		return d.Code, nil
	}
	b, _ := source.Split(d.Code)
	input := []rune(b.Body)
	var sb strings.Builder
	previous := 0
	for i, y := range sites {
		target := d.Imports[i]
		if target == libraryImport {
			continue // dilin kitaplığı: yol olduğu gibi kalır
		}
		if target < 0 || target >= len(paths) {
			return "", messages.New(messages.H8771, d.Name, target)
		}
		sb.WriteString(string(input[previous:y.start]))
		sb.WriteString(stringLiteral(filepath.ToSlash(paths[target])))
		previous = y.end
	}
	sb.WriteString(string(input[previous:]))
	b.Body = sb.String()
	return b.Join(), nil
}

// writeEmbeddedModules modülleri geçici klasöre (aktarımları yönlendirilmiş olarak) yazar
// ve her modülün yolunu döndürür.
func writeEmbeddedModules(pk *pkg, dir string) ([]string, error) {
	paths := make([]string, len(pk.Modules))
	for i, m := range pk.Modules {
		name := filepath.Base(filepath.FromSlash(m.Name))
		if name == "." || name == ".." || name == string(filepath.Separator) || name == "" {
			name = "modul.tr"
		}
		paths[i] = filepath.Join(dir, strconv.Itoa(i), name)
	}
	for i, m := range pk.Modules {
		code, err := redirectImports(m, paths)
		if err != nil {
			return nil, err
		}
		if err := os.MkdirAll(filepath.Dir(paths[i]), 0o700); err != nil {
			return nil, err
		}
		if err := os.WriteFile(paths[i], []byte(code), 0o600); err != nil {
			return nil, err
		}
	}
	return paths, nil
}

// runEmbeddedApp 'tr derle' ile üretilmiş uygulamanın paketini çalıştırır.
// Modüller özel bir geçici klasöre açılır ve program bitince (hata ile bitse de) silinir.
func runEmbeddedApp(pk *pkg) {
	code := pk.Main.Code
	if len(pk.Modules) > 0 {
		dir, err := os.MkdirTemp("", "tr-uygulama-*")
		if err != nil {
			fmt.Fprintln(os.Stderr, "❌ "+messages.Text(messages.H8772, err))
			os.Exit(1)
		}
		exitCleanup = append(exitCleanup, func() { os.RemoveAll(dir) })
		paths, err := writeEmbeddedModules(pk, dir)
		if err == nil {
			code, err = redirectImports(pk.Main, paths)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "❌ "+messages.Text(messages.H8772, err))
			exit(1)
		}
	} else if slices.ContainsFunc(pk.Main.Imports, func(i int) bool { return i != libraryImport }) {
		fmt.Fprintln(os.Stderr, "❌ "+messages.Text(messages.H8773))
		os.Exit(1)
	}

	s := prepareSource(code, embeddedPreparation)
	machine := interp.New()
	machine.SourceFile = embeddedSourceName
	machine.CommandName = filepath.Base(os.Args[0])
	// Hata çerçeveleri kullanıcının yazdığı (yönlendirilmemiş) kaynağı göstersin; satırlar aynıdır.
	original := source.Prepare(pk.Main.Code, source.Options{})
	machine.SourceCode, machine.SourceLanguage = original.Text, original.Language
	// Üretilen uygulamanın bütün argümanları programındır (uygulama.exe --adım 5).
	machine.SetArgs(os.Args[1:])
	runProgram(machine, s.Program, original.Text)
	cleanup()
}

// exitCleanup çıkmadan önce çalışacak işlerdir (ör. gömülü modüllerin geçici klasörü).
var exitCleanup []func()

// cleanup bekleyen temizlik işlerini bir kez çalıştırır.
func cleanup() {
	for i := len(exitCleanup) - 1; i >= 0; i-- {
		exitCleanup[i]()
	}
	exitCleanup = nil
}

// exitCode cik'e verilen koddur; temizlik işleri (ör. tr profil'in son raporu) okur.
var exitCode int

// exit temizlik işlerini yapıp verilen kodla çıkar.
func exit(code int) {
	exitCode = code
	cleanup()
	os.Exit(code)
}
