package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"tr_dili/internal/depo"
	"tr_dili/internal/dil/kod"
	"tr_dili/internal/dil/turkce"
)

// Eklentinin dil bilgisi dosyası sözlükten üretilenle aynı olmalı: bir ad değiştiğinde
// `go run ./tools/uret/eklenti_listeleri` çalıştırılmadıysa bu test düşer.
func TestDilbilgisiGuncel(t *testing.T) {
	yeni, err := Uret()
	if err != nil {
		t.Fatal(err)
	}
	eski, err := os.ReadFile(filepath.Join(depo.Kok(), dilbilgisiYolu))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bytes.ReplaceAll(eski, []byte("\r\n"), []byte("\n")), yeni) {
		t.Fatalf("%s güncel değil; depo kökünde `go run ./tools/uret/eklenti_listeleri` çalıştırın", dilbilgisiYolu)
	}
}

// Üretilen listeler yalnızca yazım dillerinin bugünkü adlarını içerir: Türkçe adların ASCII
// yazımları ve kaldırılmış (eski) adlar renklendirilmez; başka bir yazım dilinde geçerli bir ad
// olanlar (en: int, tensor) o dilin görünümü için renklendirilir.
func TestDilbilgisiTekYazim(t *testing.T) {
	veri, err := Uret()
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(veri) {
		t.Fatal("üretilen dosya geçerli JSON değil")
	}
	var kelimeler = map[string]bool{}
	grup := regexp.MustCompile(`\\\\b\(([^()]*)\)\\\\b`)
	for _, m := range grup.FindAllStringSubmatch(string(veri), -1) {
		for _, k := range strings.Split(m[1], "|") {
			kelimeler[k] = true
		}
	}
	baskaDilde := map[string]bool{}
	for _, d := range yazimDilleri[1:] {
		for _, k := range d.Kodlar() {
			baskaDilde[d.Ad(k)] = true
		}
	}
	for _, ad := range []string{"eğer", "değişken", "sonuç", "yazdır", "tensör", "Tensör", "veri_kümesi", "parçalayıcı_yükle", "kesme_noktası", "Ondalık", "if", "var", "print", "Tensor"} {
		if !kelimeler[ad] {
			t.Errorf("%q listelerde olmalı", ad)
		}
	}
	for _, k := range kod.Kodlar() {
		g, _ := kod.Etkin().Girdisi(k)
		if a := turkce.ASCII(g.Ad); a != g.Ad && kelimeler[a] && !baskaDilde[a] {
			t.Errorf("%s'in ASCII yazımı %q listelerde olmamalı", k, a)
		}
		for _, e := range g.Eski {
			if kelimeler[e] && !baskaDilde[e] {
				t.Errorf("%s'in eski adı %q listelerde olmamalı", k, e)
			}
		}
	}
	if strings.Contains(string(veri), "number-sign") {
		t.Error("'#' yorum değildir; renklendirilmemeli")
	}
}
