package codeindex

import (
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/parser"
	"github.com/sekerahmet/TCB/internal/lang/source"
)

// Dizin bir .tr dosyasını yorumlayıcının okuduğu biçimde okur (source.Prepare): dosya `# TR 4` başlığıyla başlar, dilin
// sözcükleri kimlik yazımıyladır (TR_print, x.TR_map; dil tanımı 2.3a), sözcük çözümleyici onları kimliğe çevirir
// (TR_fn_print, TR_type_GPT); kullanıcının adları yazıldığı gibidir. Başlıksız dosya ve görünüm (`# TR 4 view=tr`)
// çalışmaz, okunmaz: dosya Index.Skipped'a nedeniyle yazılır (2.1). Konumlar diskteki metindir: satır dosyanın satırı
// (başlık 1. satır), sütun diskteki karakter. Dosyanın dili yoktur; dizin TR_DIL'den bağımsızdır, okuyanın dilindeki
// adları çıktı yazar (Reader.display).

// parsed bir .tr dosyasının okunmuş hâlidir.
type parsed struct {
	program *ast.Program
	lines   []string // dosyanın diskteki satırları, başlık ve '#!' satırı dahil (satır numaraları dosyanınki)
	header  int      // baştaki '#!' ve başlık satırlarının sayısı (source.HeaderLines): belge yorumu değildir
	coded   bool     // dosya başlıklıydı (başlıksız gövde de kodlu okunur ama dosya değildir)
	errs    []error
	roles   map[[2]int]parser.NameRole // ad sözcüklerinin sözdizimi görevi (satır, sütun): tırnaksız sözlük anahtarı, tip yazımı, alan adı ...
}

// identities dizinin dilidir: yorumlayıcının çalıştığı kimlik dili (sözcük çözümleyicinin dili). Adı dilin öğesi olan
// her şey bu dilde, yani kimliğiyle aranır (x.lang.Find(ns, "TR_fn_print")).
func identities() *codes.Language { return codes.Active() }

// readIn bir .tr metnini okur; module true ise modül olarak ayrıştırılır. body true ise metin başlıksız bir gövdedir
// (Go testinin dizesindeki program, source.Options.Body): kodlu okunur, 1. satır metnin ilk satırıdır. Dosya başlıkla
// başlar; başlıksız dosya okunmaz (H2540, yorumlayıcının iletisi).
func readIn(text string, module, body bool) *parsed {
	res := &parsed{program: &ast.Program{}}
	b, err := source.Split(text)
	if err == nil && !b.Coded && !body {
		err = source.NoHeaderError()
	}
	if err != nil {
		res.errs = []error{err}
		return res
	}
	res.coded = b.Coded
	res.lines = strings.Split(b.Body, "\n") // başlık satırı dahil: konumlar dosyadaki gerçek satırdır
	res.header = source.HeaderLines(text)
	p := parser.New(b.Lexer()) // başlıksız gövde de kodludur (source.Options.Body); başlıksız dosya yukarıda reddedildi
	p.RecordRoles()
	if module {
		res.program = p.ParseModule()
	} else {
		res.program = p.ParseProgram()
	}
	res.errs = p.Errs()
	res.roles = p.Roles()
	return res
}

// readFile metni önce program, olmazsa modül olarak okur (body: readIn).
func readFile(text string, body bool) *parsed {
	res := readIn(text, false, body)
	if len(res.errs) > 0 {
		res = readIn(text, true, body)
	}
	return res
}

// written bir adın metinde yazıldığı biçimdir: kimliğin yazımı (TR_fn_print → TR_print, dil tanımı 2.3a), kullanıcının
// adı olduğu gibi. Adın yerini satırda arayan okumalar (fieldLine, namePos) ve konumun sonu (tokenPos) bunu arar.
func written(name string) string {
	if k, err := codes.Parse(name); err == nil {
		return codes.Spelling(k)
	}
	return name
}
