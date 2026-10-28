package codeindex

import (
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/parser"
	"github.com/sekerahmet/TCB/internal/lang/source"
)

// Dizin bir .tr dosyasını yorumlayıcının okuduğu biçimde okur: dilin öğeleri kimliktir (TR_fn_print, TR_type_GPT),
// kullanıcının adları yazıldığı gibidir. Kodlu dosyanın belirteçleri kimliğin kendisidir; başlıksız dosya Türkçe
// yazılmış sayılır (17.7) ve yorumlayıcı gibi önce kodlu biçime çevrilip okunur. İki durumda da konumlar diskteki
// metne göredir: satırlar dosyanın satırları, sütunlar diskteki karakterler (kodlu dosyada kimliğin uzunluğuyla).
// Dizin bu yüzden TR_DIL'den bağımsızdır; okuyanın dilindeki adları çıktı yazar (Reader.display).

// parsed bir .tr dosyasının okunmuş hâlidir.
type parsed struct {
	program *ast.Program
	lines   []string // dosyanın diskteki satırları, başlık ve '#!' satırı dahil (satır numaraları dosyanınki)
	header  int      // baştaki '#!' ve başlık satırlarının sayısı (source.HeaderLines): belge yorumu değildir
	coded   bool     // dosya kodlu biçimdeydi
	errs    []error
	roles   map[[2]int]parser.NameRole // ad sözcüklerinin sözdizimi görevi (satır, sütun): tırnaksız sözlük anahtarı, tip yazımı, alan adı ...
	lang    *codes.Language            // adların gösterildiği dil (dosyanın dili yoktur; başlıksız metin Türkçe gösterimdir)
}

// identities dizinin dilidir: yorumlayıcının çalıştığı kimlik dili (sözcük çözümleyicinin dili). Adı dilin öğesi olan
// her şey bu dilde, yani kimliğiyle aranır (x.lang.Find(ns, "TR_fn_print")).
func identities() *codes.Language { return codes.Active() }

// readIn bir .tr dosyasını okur; module true ise dosya bir modül olarak ayrıştırılır.
func readIn(text string, module bool) *parsed {
	res := &parsed{program: &ast.Program{}, lang: source.Turkish()}
	b, err := source.Split(text)
	if err != nil {
		res.errs = []error{err}
		return res
	}
	res.coded = b.Coded
	res.lines = strings.Split(b.Body, "\n") // başlık satırı dahil: konumlar dosyadaki gerçek satırdır
	res.header = source.HeaderLines(text)
	p := parser.New(b.IdentityLexer())
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

// readFile dosyayı önce program, olmazsa modül olarak okur.
func readFile(text string) *parsed {
	res := readIn(text, false)
	if len(res.errs) > 0 {
		res = readIn(text, true)
	}
	return res
}

// written bir adın dosyada yazıldığı biçimdir: kodlu dosyada kimliğin kendisi, başlıksız dosyada kimliğin o dosyanın
// yazım dilindeki adı; kullanıcının adı olduğu gibi. Adın yerini satırda arayan okumalar (fieldLine, namePos) bunu arar.
func (p *parsed) written(name string) string {
	if p == nil || p.coded {
		return name
	}
	if k, err := codes.Parse(name); err == nil {
		return p.lang.Name(k)
	}
	return name
}

// dictKey satır ve sütundaki ad tırnaksız bir sözlük anahtarı mı (`{ad: değer}`). Karar sözdizimi ağacından verilir:
// adlı argüman (`f(ad: 1)`), tip yazımı (`değişken ad: Tam`) ve yapı alanı (`ad: Metin`) da `ad:` yazılır ama anahtar
// değildir; dizgedeki ya da yorumdaki parantez kararı etkilemez (bağımsız denetim grup 6, R3).
func (p *parsed) dictKey(line, col int) bool {
	return p != nil && p.roles[[2]int{line, col}] == parser.RoleDictKey
}
