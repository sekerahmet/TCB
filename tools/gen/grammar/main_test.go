package main

import (
	"encoding/json"
	"github.com/sekerahmet/TCB/tools/internal/genmain"
	"regexp"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
	"github.com/sekerahmet/TCB/internal/reporoot"
)

// Eklentinin dil bilgisi dosyası sözlükten üretilenle aynı olmalı: bir ad değiştiğinde
// `go run ./tools/gen/grammar` çalıştırılmadıysa bu test düşer.
func TestGrammarUpToDate(t *testing.T) {
	genmain.AssertUpToDate(t, reporoot.Root(), "grammar", grammarPath, func(string) ([]byte, error) { return Generate() })
}

// Üretilen listeler yalnızca yazım dillerinin bugünkü adlarını içerir: Türkçe adların ASCII
// yazımları ve kaldırılmış (eski) adlar renklendirilmez; başka bir yazım dilinde geçerli bir ad
// olanlar (en: int, tensor) o dilin görünümü için renklendirilir.
func TestGrammarSingleSpelling(t *testing.T) {
	data, err := Generate()
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(data) {
		t.Fatal("üretilen dosya geçerli JSON değil")
	}
	var words = map[string]bool{}
	group := regexp.MustCompile(`\\\\b\(([^()]*)\)\\\\b`)
	for _, m := range group.FindAllStringSubmatch(string(data), -1) {
		for _, k := range strings.Split(m[1], "|") {
			words[k] = true
		}
	}
	inOtherLanguage := map[string]bool{}
	for _, d := range spellingLanguages[1:] {
		for _, k := range d.Codes() {
			inOtherLanguage[d.Name(k)] = true
		}
	}
	for _, name := range []string{"eğer", "değişken", "sonuç", "yazdır", "tensör", "Tensör", "veri_kümesi", "parçalayıcı_yükle", "kesme_noktası", "Ondalık", "if", "var", "print", "Tensor"} {
		if !words[name] {
			t.Errorf("%q listelerde olmalı", name)
		}
	}
	for _, k := range codes.Codes() {
		g, _ := codes.Active().Entry(k)
		if a := turkish.ASCII(g.Name); a != g.Name && words[a] && !inOtherLanguage[a] {
			t.Errorf("%s'in ASCII yazımı %q listelerde olmamalı", k, a)
		}
	}
	if strings.Contains(string(data), "number-sign") {
		t.Error("'#' yorum değildir; renklendirilmemeli")
	}
}
