package main

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/parser"
	"github.com/sekerahmet/TCB/internal/lang/source"
)

// trResult bir .tr dosyasının Türkçe okunmuş hâlidir.
type trResult struct {
	Program *ast.Program
	Text    string // Türkçe gösterim metni; satır numaraları buna göredir (17.5)
	Coded   bool   // dosya kodlu biçimdeydi
	Errors  []error
}

// readTurkish bir .tr dosyasını etkin yazım dilinden bağımsız ayrıştırır: kodsuz dosya her zaman
// Türkçe okunur (Parts.Lexer), kodlu dosya önce Türkçe yazıma çevrilir (Decode). Etkin dile bağlı
// Prepare kullanılmaz: TR_DIL=en iken o, dosyayı İngilizce görünümüne çevirirdi ve Türkçe ad
// tablosuyla çözülen yerleşik adlar kaybolurdu. module true ise dosya bir modül olarak okunur.
func readTurkish(text string, module bool) *trResult {
	res := &trResult{Program: &ast.Program{}}
	b, err := source.Split(text)
	if err != nil {
		res.Errors = []error{err}
		return res
	}
	if b.Coded {
		res.Coded = true
		turkish, err := source.Decode(text, source.Turkish())
		if err == nil {
			b, err = source.Split(turkish)
		}
		if err != nil {
			res.Errors = []error{err}
			return res
		}
	}
	res.Text = b.Body
	p := parser.New(b.Lexer())
	if module {
		res.Program = p.ParseModule()
	} else {
		res.Program = p.ParseProgram()
	}
	res.Errors = p.Errs()
	return res
}
