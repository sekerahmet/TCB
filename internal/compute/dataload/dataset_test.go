package dataload

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestRealDataSmoke, gerçek veri köküne karşı küçük bir duman testidir.
// Yalnızca TR_VERI_YOLU ortam değişkeni veri kökünü (meta.json, train/,
// _valid/ içeren dizin) gösteriyorsa çalışır. Yalnızca meta.json'u, birkaç
// parçanın başlığını ve birkaç küçük pencereyi okur; dosyaları kopyalamaz,
// eşleme yerine ReadAt kullanır (akışlı ağ sürücülerinde en az okuma).
func TestRealDataSmoke(t *testing.T) {
	root := os.Getenv("TR_VERI_YOLU")
	if root == "" {
		t.Skip("TR_VERI_YOLU ayarlı değil; gerçek veri duman testi atlandı")
	}
	ctx := context.Background()
	pick := DatasetOptions{Shard: ShardOptions{NoMmap: true}}
	train, err := OpenDataset(filepath.Join(root, "train"), pick)
	if err != nil {
		t.Fatal(err)
	}
	m := train.Meta()
	if m == nil {
		t.Fatal("meta.json bulunamadı")
	}
	t.Logf("tokenizer: %s", m.TokenizerPath())
	t.Logf("eğitim: %d parça, %d token (meta'da %d parça kaydı)", train.ShardCount(), train.TotalTokens(), len(m.Parts))
	if train.ShardCount() != len(m.Parts) {
		t.Errorf("dizindeki parça sayısı (%d) meta.json ile (%d) uyuşmuyor", train.ShardCount(), len(m.Parts))
	}

	// İlk ve son parçanın başlığını gerçekten oku ve meta ile karşılaştır.
	ps := train.Shards()
	for _, i := range []int{0, len(ps) - 1} {
		n, err := headerLen(ps[i].Path)
		if err != nil {
			t.Fatal(err)
		}
		if n != ps[i].Token {
			t.Errorf("%s: başlık %d, meta %d", ps[i].Name, n, ps[i].Token)
		}
		t.Logf("%s: başlık uzunluğu %d token", ps[i].Name, n)
	}

	checkToken := func(ad string, v []int32) {
		for _, x := range v {
			if x < 0 || x >= 40000 {
				t.Fatalf("%s: sözlük dışı token %d", ad, x)
			}
		}
	}

	// İlk parçadan 64 token.
	p, err := train.OpenShard(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	first := make([]int32, 64)
	if err := p.Read(first, 0); err != nil {
		t.Fatal(err)
	}
	_ = p.Close()
	checkToken("ilk parça", first)
	t.Logf("%s ilk 16 token: %v (tür %s)", ps[0].Name, first[:16], p.DType())

	// Rastgele örnekleyiciden tek küçük toplu iş (2 × 33 token).
	o, err := train.Sampler(SamplerOptions{B: 2, T: 32, Seed: 1, GroupSize: 1})
	if err != nil {
		t.Fatal(err)
	}
	var ti Batch
	if err := o.Next(&ti); err != nil {
		t.Fatal(err)
	}
	_ = o.Close()
	checkToken("örnekleyici", ti.Data)
	t.Logf("rastgele toplu iş durumu %+v, satır 0 ilk 8: %v", ti.State, ti.Row(0)[:8])

	// Doğrulama kümesinin başından sıralı tek toplu iş.
	valid, err := OpenDataset(filepath.Join(root, "_valid"), pick)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("doğrulama: %d parça, %d token", valid.ShardCount(), valid.TotalTokens())
	if m.Valid != nil && m.Valid.Tokens != valid.TotalTokens() {
		t.Errorf("doğrulama token sayısı %d, meta %d", valid.TotalTokens(), m.Valid.Tokens)
	}
	so, err := valid.Sequential(SequentialOptions{B: 2, T: 32})
	if err != nil {
		t.Fatal(err)
	}
	if err := so.Fill(ctx, &ti); err != nil {
		t.Fatal(err)
	}
	_ = so.Close()
	checkToken("doğrulama", ti.Data)
	if ti.Row(1)[0] != ti.Row(0)[32] {
		t.Error("sıralı pencereler T adımıyla ardışık değil")
	}
}
