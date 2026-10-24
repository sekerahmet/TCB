package codeindex

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/parser"
	"github.com/sekerahmet/TCB/internal/lang/source"
)

// parsed bir .tr dosyasının Türkçe okunmuş hâlidir.
type parsed struct {
	program *ast.Program
	text    string // Türkçe gösterim metni; kodlu dosyada başlık satırı yerine boş satır konur, böylece satır numaraları dosyadaki gerçek satırdır (dil iletileri 17.5 gereği gövde satırını sayar)
	coded   bool   // dosya kodlu biçimdeydi
	errs    []error
	roles   map[[2]int]parser.NameRole // ad sözcüklerinin sözdizimi görevi (satır, sütun): tırnaksız sözlük anahtarı, tip yazımı, alan adı ...
	lang    string                     // dosyanın yazım dili (kodlu dosyada başlığınki, kodsuzda tr; 17.7)
}

// splitTurkish bir .tr dosyasını Türkçe gövdesine böler: kodlu dosya önce Türkçe yazıma çevrilir
// (Decode). coded dosyanın kodlu olduğunu söyler. Kodlu dosya çözülemezse ya da çözümü bölünemezse
// err doludur ve parts dosyanın kendi (çözülmemiş) bölümleridir.
func splitTurkish(text string) (parts source.Parts, coded bool, err error) {
	parts, err = source.Split(text)
	if err != nil || !parts.Coded {
		return parts, false, err
	}
	turkish, err := source.Decode(text, source.Turkish())
	if err != nil {
		return parts, true, err
	}
	decoded, err := source.Split(turkish)
	if err != nil {
		return parts, true, err
	}
	decoded.Language = parts.Language // gövde Türkçeye çevrildi, dosyanın dili başlığınkidir
	return decoded, true, nil
}

// readTurkish bir .tr dosyasını etkin yazım dilinden bağımsız ayrıştırır: kodsuz dosya her zaman
// Türkçe okunur (Parts.Lexer), kodlu dosya önce Türkçe yazıma çevrilir (Decode). Etkin dile bağlı
// source.Prepare kullanılmaz: TR_DIL=en iken o, dosyayı İngilizce görünümüne çevirirdi ve Türkçe ad
// tablosuyla çözülen yerleşik adlar kaybolurdu. module true ise dosya bir modül olarak okunur.
func readTurkish(text string, module bool) *parsed {
	res := &parsed{program: &ast.Program{}}
	b, coded, err := splitTurkish(text)
	res.coded, res.lang = coded, b.Language
	if err != nil {
		res.errs = []error{err}
		return res
	}
	if coded {
		b.Body = "\n" + b.Body // başlık satırının yerine boş satır: konumlar dosyadaki gerçek satır olur
	}
	res.text = b.Body
	p := parser.New(b.Lexer())
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

// dictKey satır ve sütundaki ad tırnaksız bir sözlük anahtarı mı (`{ad: değer}`). Karar sözdizimi ağacından verilir:
// adlı argüman (`f(ad: 1)`), tip yazımı (`değişken ad: Tam`) ve yapı alanı (`ad: Metin`) da `ad:` yazılır ama anahtar
// değildir; dizgedeki ya da yorumdaki parantez kararı etkilemez (bağımsız denetim grup 6, R3).
func (p *parsed) dictKey(line, col int) bool {
	return p != nil && p.roles[[2]int{line, col}] == parser.RoleDictKey
}

// readFile dosyayı önce program, olmazsa modül olarak okur.
func readFile(text string) *parsed {
	res := readTurkish(text, false)
	if len(res.errs) > 0 {
		res = readTurkish(text, true)
	}
	return res
}
