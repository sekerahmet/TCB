package main

import (
	"errors"
	"strings"
	"unicode"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/source"
)

// encodeView yazımı yeni kurala çevrilmiş gösterim metnini (langCode dilinde) bugünkü kodlama yoluyla
// kimliklere çevirir ve başlıksız kodlu gövdeyi verir. Metin bilerek bozuksa (sözdizimi hatası:
// hata örnekleri, negatif fikstürler) kodlama ayrıştırıcısız, sözcük düzeyinde yapılır (lexical);
// sözdizimsiz çeviri kapsam bilmez, sonucu gözle denetlenmeli: lexical true döner.
func encodeView(text, langCode string) (body string, lexical bool, err error) {
	lang, err := codes.LoadLanguage(langCode)
	if err != nil {
		return "", false, err
	}
	coded, err := source.EncodeIn(text, lang)
	var syntax *source.SyntaxError
	if errors.As(err, &syntax) {
		return lexicalEncode(text, lang), true, nil
	}
	if err != nil {
		return "", false, err
	}
	_, body, _ = firstLine(coded)
	return body, false, nil
}

// lexicalEncode metni sözcük sözcük kodlar: anahtar kelime ve dilin adı olan sözcük kimliğinin
// yazımıyla; noktadan sonraki ad üye, parantez içinde ':'dan önceki ad adlı argüman, öbürleri genel ad
// alanında ya da tip adı olarak aranır. Yerleştirmelerin içi kodlanmaz.
func lexicalEncode(text string, lang *codes.Language) string {
	l := lexer.NewBodyIn(text, false, lang)
	l.StartRecording()
	for l.NextToken().Type != lexer.TokEOF {
	}
	input := l.Input()
	var out strings.Builder
	prev := 0
	var prevType lexer.TokenType
	depth := 0
	for _, r := range l.Records() {
		spelling := ""
		switch {
		case r.Code != codes.None:
			spelling = codes.Spelling(r.Code)
		case lexer.KeywordCode(r.Type) != codes.None:
			spelling = codes.Spelling(lexer.KeywordCode(r.Type))
		case r.Type == lexer.TokIdent:
			spelling = lexicalName(lang, r.Literal, prevType, depth > 0 && colonAfter(input, r.End))
		}
		switch r.Type {
		case lexer.TokLParen:
			depth++
		case lexer.TokRParen:
			depth--
		}
		if spelling != "" && r.End <= len(input) {
			out.WriteString(string(input[prev:r.Start]))
			out.WriteString(spelling)
			prev = r.End
		}
		prevType = r.Type
	}
	out.WriteString(string(input[prev:]))
	return out.String()
}

// lexicalName bir adın yerine göre kimlik yazımıdır (dilin adı değilse boş).
func lexicalName(lang *codes.Language, name string, prev lexer.TokenType, named bool) string {
	ns := []codes.Namespace{codes.NamespaceGlobal, codes.NamespaceType}
	switch {
	case prev == lexer.TokDot || prev == lexer.TokSafeDot:
		ns = []codes.Namespace{codes.NamespaceMember}
	case named:
		ns = []codes.Namespace{codes.NamespaceParam}
	}
	for _, n := range ns {
		if k, ok := lang.Find(n, name); ok {
			return codes.Spelling(k)
		}
	}
	return ""
}

// colonAfter i'den sonra (boşluktan sonra) tek ':' geliyor mu?
func colonAfter(input []rune, i int) bool {
	for i < len(input) && unicode.IsSpace(input[i]) && input[i] != '\n' {
		i++
	}
	return i < len(input) && input[i] == ':' && (i+1 >= len(input) || input[i+1] != '=')
}
