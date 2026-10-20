package dataset_test

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/dataload"
	"github.com/sekerahmet/TCB/internal/compute/tokenize"
	"github.com/sekerahmet/TCB/internal/reporoot"
)

// tests/data altındaki sentetik test verisi (TR testleri ve examples/08_veri_ile_egitim.tr
// kullanır) bu dosyadaki üreteçle oluşturulur. Varsayılan olarak test, depodaki dosyaların
// üreteçle birebir aynı olduğunu denetler; yeniden üretmek için:
//
//	go test ./internal/stdlib/dataset -run TestSampleData -guncelle
var update = flag.Bool("guncelle", false, "tests/data altındaki sentetik veriyi yeniden üret")

var sampleRoot = reporoot.Path("tests/data")

// sampleFiles depodaki sentetik verinin beklenen içeriğidir (göreli yol → içerik).
func sampleFiles(t *testing.T) map[string][]byte {
	t.Helper()
	read := func(path string) []byte {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	npyU16 := func(v []uint16) []byte {
		var b bytes.Buffer
		if err := dataload.WriteNpyU16(&b, v); err != nil {
			t.Fatal(err)
		}
		return b.Bytes()
	}
	npyI32 := func(v []int32) []byte {
		var b bytes.Buffer
		if err := dataload.WriteNpyI32(&b, v); err != nil {
			t.Fatal(err)
		}
		return b.Bytes()
	}
	d := map[string][]byte{}
	// Parçalayıcılar: bayt düzeyi BPE (özel tokenlı, <|endoftext|> içerir) ve BERT
	// tarzı WordPiece (son işleyicisi [CLS]/[SEP] ekler; çift kodlama için).
	bpe := read(reporoot.Path("internal/compute/tokenize/testdata/p_bayt_bpe.json"))
	d["parcalayici.json"] = bpe
	d["parcalayici_bert.json"] = read(reporoot.Path("internal/compute/tokenize/testdata/p_wordpiece.json"))

	// sayilar/: değerleri konumunu söyleyen iki parça (uint16 ve int32).
	a := make([]uint16, 100)
	for i := range a {
		a[i] = uint16(i)
	}
	b := make([]int32, 150)
	for i := range b {
		b[i] = int32(1000 + i)
	}
	d["numbers/a.npy"] = npyU16(a)
	d["numbers/b.npy"] = npyI32(b)

	// metin/: örnek Türkçe metnin satırları belge sayılıp kodlanır, aralarına
	// <|endoftext|> konur ve iki parçaya bölünür; meta.json parça token sayılarını verir.
	p, err := tokenize.Parse(bpe)
	if err != nil {
		t.Fatal(err)
	}
	sep, ok := p.TokenToID("<|endoftext|>")
	if !ok {
		t.Fatal("<|endoftext|> yok")
	}
	var docs []string
	for _, s := range strings.Split(string(read(reporoot.Path("internal/compute/tokenize/testdata/ornek_metin.txt"))), "\n") {
		if strings.TrimSpace(s) != "" {
			docs = append(docs, s)
		}
	}
	tokens := p.JoinDocuments(docs, sep, 1)
	half := len(tokens) / 2
	parts := [][]int32{tokens[:half], tokens[half:]}
	meta := map[string]any{"tokenizer": "../parcalayici.json", "parts": map[string]any{}}
	for i, pt := range parts {
		u := make([]uint16, len(pt))
		for j, x := range pt {
			u[j] = uint16(x)
		}
		name := fmt.Sprintf("metin_%03d.npy", i)
		d["text/"+name] = npyU16(u)
		meta["parts"].(map[string]any)[name] = map[string]any{"tokens": len(pt)}
	}
	mj, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	d["text/meta.json"] = append(mj, '\n')

	// Hata testleri için .npy olmayan bir dosya.
	d["bozuk.npy"] = []byte("bu bir .npy dosyası değil\n")
	return d
}

func TestSampleData(t *testing.T) {
	want := sampleFiles(t)
	if *update {
		for name, content := range want {
			path := filepath.Join(sampleRoot, filepath.FromSlash(name))
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, content, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		return
	}
	for name, content := range want {
		got, err := os.ReadFile(filepath.Join(sampleRoot, filepath.FromSlash(name)))
		if err != nil {
			t.Errorf("%s okunamadı (yeniden üretmek için -guncelle): %v", name, err)
			continue
		}
		if !bytes.Equal(got, content) {
			t.Errorf("%s üreteçle aynı değil (yeniden üretmek için -guncelle)", name)
		}
	}
}
