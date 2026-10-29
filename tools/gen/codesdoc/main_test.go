package main

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/reporoot"
	"github.com/sekerahmet/TCB/tools/internal/genmain"
)

// docs/codes.md kod listesinden üretilenle aynı olmalı: bir kod, açıklama ya da tanım
// değiştiğinde `go run ./tools/gen/codesdoc` çalıştırılmadıysa bu test düşer.
func TestDocUpToDate(t *testing.T) {
	genmain.AssertUpToDate(t, reporoot.Root(), "codesdoc", docPath, func(string) ([]byte, error) { return Generate() })
}

// Kural (17.2): kod başlıkları ve ilişkili satırları program yazımını öne alır; aynı kısa
// adı taşıyan iki kod yan yana geçiyorsa sınıflı kimlik parantez içinde yazılır.
func TestCodeLabelAndHeading(t *testing.T) {
	for k, want := range map[codes.Code]string{
		codes.KwAnd:    "`TR_and`",
		codes.CtxAs:    "`TR_as`",
		codes.ValInf:   "`TR_inf`",
		codes.TypeInt:  "`TR_Int`",
		codes.ConstPi:  "`TR_pi`",
		codes.FnLog:    "`TR_log(...)`",
		codes.MemFirst: "`.TR_first`",
		codes.MemSort:  "`.TR_sort(...)`",
		codes.ArgK:     "`TR_k:`",
	} {
		if got := codeLabel(k); got != want {
			t.Errorf("codeLabel(%s) = %q, beklenen %q", k, got, want)
		}
	}
	if got, want := codeHeading(codes.KwAnd, "ve"), "#### `TR_and` (`TR_kw_and`, `ve`)\n\n"; got != want {
		t.Errorf("codeHeading(KwAnd) = %q, beklenen %q", got, want)
	}
	if got, want := codeList([]codes.Code{codes.KwOr, codes.KwNot}), "`TR_or`, `TR_not`"; got != want {
		t.Errorf("codeList(farklı adlar) = %q, beklenen %q", got, want)
	}
	if got, want := codeList([]codes.Code{codes.FnLog, codes.ArgLog}), "`TR_log(...)` (`TR_fn_log`), `TR_log:` (`TR_arg_log`)"; got != want {
		t.Errorf("codeList(aynı kısa ad) = %q, beklenen %q", got, want)
	}
}
