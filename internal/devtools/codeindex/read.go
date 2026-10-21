package codeindex

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/parser"
	"github.com/sekerahmet/TCB/internal/lang/source"
)

// parsed bir .tr dosyasının Türkçe okunmuş hâlidir.
type parsed struct {
	program *ast.Program
	text    string // Türkçe gösterim metni; satır numaraları buna göredir (17.5)
	coded   bool   // dosya kodlu biçimdeydi
	errs    []error
}

// readTurkish bir .tr dosyasını etkin yazım dilinden bağımsız ayrıştırır: kodsuz dosya her zaman
// Türkçe okunur (Parts.Lexer), kodlu dosya önce Türkçe yazıma çevrilir (Decode). Etkin dile bağlı
// source.Prepare kullanılmaz: TR_DIL=en iken o, dosyayı İngilizce görünümüne çevirirdi ve Türkçe ad
// tablosuyla çözülen yerleşik adlar kaybolurdu. module true ise dosya bir modül olarak okunur.
func readTurkish(text string, module bool) *parsed {
	res := &parsed{program: &ast.Program{}}
	b, err := source.Split(text)
	if err != nil {
		res.errs = []error{err}
		return res
	}
	if b.Coded {
		res.coded = true
		turkish, err := source.Decode(text, source.Turkish())
		if err == nil {
			b, err = source.Split(turkish)
		}
		if err != nil {
			res.errs = []error{err}
			return res
		}
	}
	res.text = b.Body
	p := parser.New(b.Lexer())
	if module {
		res.program = p.ParseModule()
	} else {
		res.program = p.ParseProgram()
	}
	res.errs = p.Errs()
	return res
}

// readFile dosyayı önce program, olmazsa modül olarak okur.
func readFile(text string) *parsed {
	res := readTurkish(text, false)
	if len(res.errs) > 0 {
		res = readTurkish(text, true)
	}
	return res
}
