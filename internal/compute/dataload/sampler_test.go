package dataload

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

func takeBatches(t *testing.T, o *Sampler, n int) [][]int32 {
	t.Helper()
	var ti Batch
	var result [][]int32
	for i := 0; i < n; i++ {
		if err := o.Next(&ti); err != nil {
			t.Fatal(err)
		}
		result = append(result, append([]int32(nil), ti.Data...))
	}
	return result
}

func TestDatasetDetection(t *testing.T) {
	d := encodedDataset(t, 100, 250, 7)
	_ = os.WriteFile(filepath.Join(d, "notlar.txt"), []byte("x"), 0o644)
	k, err := OpenDataset(d, DatasetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if k.ShardCount() != 3 || k.TotalTokens() != 357 || k.Meta() != nil {
		t.Fatalf("küme yanlış: %d parça, %d token", k.ShardCount(), k.TotalTokens())
	}
	ps := k.Shards()
	if ps[0].Name != shardName(0) || ps[1].Token != 250 {
		t.Fatalf("parça bilgisi yanlış: %+v", ps)
	}
	// Tek dosya da küme olabilir.
	k1, err := OpenDataset(ps[1].Path, DatasetOptions{})
	if err != nil || k1.ShardCount() != 1 || k1.TotalTokens() != 250 {
		t.Fatalf("tek dosyalı küme: %v", err)
	}
	if _, err := OpenDataset(t.TempDir(), DatasetOptions{}); err == nil {
		t.Fatal("boş dizin kabul edildi")
	}
}

func TestDatasetMeta(t *testing.T) {
	root := t.TempDir()
	tr := filepath.Join(root, "train")
	_ = os.Mkdir(tr, 0o755)
	writeU16(t, tr, "a.npy", fakeU16(300, 1))
	writeU16(t, tr, "b.npy", fakeU16(500, 2))
	meta := map[string]any{
		"tokenizer": "/bir/yer/tok.json",
		"files":     map[string]any{"a": map[string]any{"tokens": 800}},
		"parts":     map[string]any{"a.npy": map[string]any{"tokens": 300}, "b.npy": map[string]any{"tokens": 500}},
		"valid":     map[string]any{"tokens": 10, "docs": 2},
		"progress":  map[string]any{},
	}
	b, _ := json.Marshal(meta)
	_ = os.WriteFile(filepath.Join(root, "meta.json"), b, 0o644)
	k, err := OpenDataset(tr, DatasetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if k.Meta() == nil || k.Meta().TokenizerPath() != "/bir/yer/tok.json" || k.Meta().Valid.Tokens != 10 {
		t.Fatal("meta okunmadı")
	}
	if k.TotalTokens() != 800 {
		t.Fatal("toplam yanlış")
	}
	// meta.json ile başlık uyuşmazsa parça açılırken hata.
	meta["parts"] = map[string]any{"a.npy": map[string]any{"tokens": 301}}
	b, _ = json.Marshal(meta)
	_ = os.WriteFile(filepath.Join(root, "meta.json"), b, 0o644)
	k, err = OpenDataset(tr, DatasetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if k.Shards()[1].Token != 500 { // b.npy meta'da yok → başlıktan
		t.Fatal("eksik meta kaydı başlıktan okunmadı")
	}
	if _, err := k.OpenShard(context.Background(), 0); !messages.Has(err, messages.H5026) {
		t.Fatalf("uyuşmazlık hatası bekleniyordu: %v", err)
	}
	k2, err := OpenDataset(tr, DatasetOptions{NoMeta: true})
	if err != nil || k2.Meta() != nil || k2.TotalTokens() != 800 {
		t.Fatal("MetaYok çalışmadı")
	}
}

func TestSamplerReproducible(t *testing.T) {
	checkShardLeak(t)
	d := encodedDataset(t, 1000, 2000, 1500, 333, 10)
	k, _ := OpenDataset(d, DatasetOptions{})
	pick := SamplerOptions{B: 4, T: 16, Seed: 42}
	o1, err := k.Sampler(pick)
	if err != nil {
		t.Fatal(err)
	}
	o2, _ := k.Sampler(pick)
	perEpoch := int(o1.EpochWindows()) / 4
	a := takeBatches(t, o1, 3*perEpoch)
	b := takeBatches(t, o2, 3*perEpoch)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("aynı tohum farklı toplu işler verdi")
	}
	if o1.Epoch() < 2 {
		t.Fatalf("üç dönem geçilmedi: %d", o1.Epoch())
	}
	pick.Seed = 43
	o3, _ := k.Sampler(pick)
	c := takeBatches(t, o3, 3*perEpoch)
	if reflect.DeepEqual(a, c) {
		t.Fatal("farklı tohum aynı toplu işleri verdi")
	}
	// Eşleme kapalı küme aynı veriyi vermeli.
	kr, _ := OpenDataset(d, DatasetOptions{Shard: ShardOptions{NoMmap: true}})
	pick.Seed = 42
	o4, _ := kr.Sampler(pick)
	if !reflect.DeepEqual(a, takeBatches(t, o4, 3*perEpoch)) {
		t.Fatal("ReadAt yolu eşleme yolundan farklı")
	}
	for _, o := range []*Sampler{o1, o2, o3, o4} {
		_ = o.Close()
	}
}

func TestSamplerWindowBoundsAndShift(t *testing.T) {
	checkShardLeak(t)
	lengths := []int{1000, 2000, 1500, 333, 10, 17}
	d := encodedDataset(t, lengths...)
	k, _ := OpenDataset(d, DatasetOptions{})
	const B, T = 3, 16
	o, err := k.Sampler(SamplerOptions{B: B, T: T, Seed: 9})
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()
	expected := 0
	for _, n := range lengths {
		expected += n / (T + 1)
	}
	if int(o.EpochWindows()) != expected {
		t.Fatalf("dönem pencere sayısı %d != %d", o.EpochWindows(), expected)
	}
	var ti Batch
	var input, target []int32
	for step := 0; step < 3*expected/B; step++ {
		if err := o.Next(&ti); err != nil {
			t.Fatal(err)
		}
		if ti.B != B || ti.T != T || len(ti.Data) != B*(T+1) {
			t.Fatalf("toplu iş boyutu yanlış: %d×%d, %d", ti.B, ti.T, len(ti.Data))
		}
		input, target = ti.SplitInputTarget(input, target)
		for r := 0; r < B; r++ {
			s := ti.Row(r)
			p, begin := int(s[0])/1_000_000, int(s[0])%1_000_000
			for j, v := range s {
				if int(v) != p*1_000_000+begin+j {
					t.Fatalf("pencere bitişik değil ya da parça sınırını aşıyor: %v", s)
				}
			}
			if begin+T+1 > lengths[p] {
				t.Fatalf("pencere parça sonunu aşıyor: parça %d, baş %d", p, begin)
			}
			for j := 0; j < T; j++ {
				if ti.Target(r)[j] != ti.Input(r)[j]+1 || target[r*T+j] != input[r*T+j]+1 || input[r*T+j] != s[j] {
					t.Fatal("hedef girdiden bir kaydırılmış değil")
				}
			}
		}
	}
	// Tek bir dönemi ayrıca denetle: tam bir dönemde her parçadaki
	// pencereler üst üste binmemeli ve sayıları floor(n/(T+1)) olmalı.
	// Her grup boyu için (1 = parça parça, 10 = tüm parçalar tek grup).
	for _, group := range []int{1, 2, 4, 10} {
		checkScope(t, k, lengths, T, group)
	}
	// Dönemler arasında parça sırası değişmeli (6 parçada aynı kalma
	// olasılığı tohuma bağlı ama bu tohumda değişir).
	var orders [][]int
	for epoch := 0; epoch < 3; epoch++ {
		orders = append(orders, o.epochOrder(epoch, nil))
	}
	if reflect.DeepEqual(orders[0], orders[1]) && reflect.DeepEqual(orders[1], orders[2]) {
		t.Fatal("parça sırası dönemler arasında karışmıyor")
	}
}

func checkScope(t *testing.T, k *Dataset, lengths []int, T, group int) {
	t.Helper()
	o2, err := k.Sampler(SamplerOptions{B: 1, T: T, Seed: 9, GroupSize: group})
	if err != nil {
		t.Fatal(err)
	}
	defer o2.Close()
	expected := int(o2.EpochWindows())
	num := map[int]int{}
	scope := map[int]map[int]bool{}
	var one Batch
	for i := 0; i < expected; i++ {
		if err := o2.Next(&one); err != nil {
			t.Fatal(err)
		}
		p, begin := int(one.Data[0])/1_000_000, int(one.Data[0])%1_000_000
		num[p]++
		if scope[p] == nil {
			scope[p] = map[int]bool{}
		}
		for j := 0; j <= T; j++ {
			if scope[p][begin+j] {
				t.Fatalf("bir dönemde pencereler üst üste biniyor: parça %d konum %d", p, begin+j)
			}
			scope[p][begin+j] = true
		}
	}
	if err := o2.Next(&one); err != nil || o2.Epoch() != 1 {
		t.Fatalf("dönem sonunda bir sonraki pencere 1. dönemden olmalı: %d %v", o2.Epoch(), err)
	}
	for p, n := range lengths {
		if num[p] != n/(T+1) {
			t.Fatalf("grup %d, parça %d: %d pencere, beklenen %d", group, p, num[p], n/(T+1))
		}
	}
}

func TestSamplerGroupMixing(t *testing.T) {
	checkShardLeak(t)
	d := encodedDataset(t, 4000, 4000, 4000, 4000, 4000, 4000, 4000, 4000)
	k, _ := OpenDataset(d, DatasetOptions{})
	// Toplu iş başına farklı parça sayısının ortalaması.
	mean := func(group int) float64 {
		o, err := k.Sampler(SamplerOptions{B: 8, T: 15, Seed: 21, GroupSize: group})
		if err != nil {
			t.Fatal(err)
		}
		defer o.Close()
		if want := (8 + group - 1) / group; o.GroupCount() != want {
			t.Fatalf("grup sayısı %d, beklenen %d", o.GroupCount(), want)
		}
		var ti Batch
		total, step := 0, int(o.EpochWindows())/8
		for i := 0; i < step; i++ {
			if err := o.Next(&ti); err != nil {
				t.Fatal(err)
			}
			different := map[int32]bool{}
			for r := 0; r < ti.B; r++ {
				different[ti.Row(r)[0]/1_000_000] = true
			}
			total += len(different)
		}
		return float64(total) / float64(step)
	}
	single, four := mean(1), mean(4)
	if single > 1.1 {
		t.Fatalf("grup 1'de toplu işler çoğunlukla tek parçadan gelmeli: %.2f", single)
	}
	if four < 3 {
		t.Fatalf("grup 4'te satırlar birden çok parçadan gelmeli: ortalama %.2f farklı parça", four)
	}
	// Tekrar üretilebilirlik ve durumdan sürdürme gruplarla da korunur.
	pick := SamplerOptions{B: 8, T: 15, Seed: 21, GroupSize: 3}
	o1, _ := k.Sampler(pick)
	defer o1.Close()
	first := takeBatches(t, o1, 300)
	o2, _ := k.Sampler(pick)
	defer o2.Close()
	if !reflect.DeepEqual(first, takeBatches(t, o2, 300)) {
		t.Fatal("gruplu örnekleme tekrar üretilemiyor")
	}
	var ti Batch
	_ = o1.Next(&ti)
	state := ti.State
	after := takeBatches(t, o1, 100)
	pick.Start = &state
	o3, err := k.Sampler(pick)
	if err != nil {
		t.Fatal(err)
	}
	defer o3.Close()
	if !reflect.DeepEqual(after, takeBatches(t, o3, 100)) {
		t.Fatal("gruplu örneklemede durumdan sürdürme bozuk")
	}
	if _, err := k.Sampler(SamplerOptions{B: 1, T: 5, GroupSize: -1}); err == nil {
		t.Fatal("negatif grup boyu kabul edildi")
	}
}

func TestSamplerResumeFromState(t *testing.T) {
	checkShardLeak(t)
	d := encodedDataset(t, 500, 800, 300)
	k, _ := OpenDataset(d, DatasetOptions{})
	pick := SamplerOptions{B: 5, T: 8, Seed: 3}
	o, _ := k.Sampler(pick)
	defer o.Close()
	var ti Batch
	var state State
	for i := 0; i < 37; i++ {
		if err := o.Next(&ti); err != nil {
			t.Fatal(err)
		}
		state = ti.State
	}
	after := takeBatches(t, o, 50)
	pick.Start = &state
	o2, err := k.Sampler(pick)
	if err != nil {
		t.Fatal(err)
	}
	defer o2.Close()
	if !reflect.DeepEqual(after, takeBatches(t, o2, 50)) {
		t.Fatal("durumdan sürdürme aynı toplu işleri vermedi")
	}
	pick.Start = &State{Pos: 99}
	if _, err := k.Sampler(pick); err == nil {
		t.Fatal("geçersiz durum kabul edildi")
	}
}

func TestSamplerErrors(t *testing.T) {
	d := encodedDataset(t, 10, 12)
	k, _ := OpenDataset(d, DatasetOptions{})
	if _, err := k.Sampler(SamplerOptions{B: 1, T: 50}); err == nil {
		t.Fatal("hiçbir parçaya sığmayan T kabul edildi")
	}
	if _, err := k.Sampler(SamplerOptions{B: 0, T: 5}); err == nil {
		t.Fatal("B=0 kabul edildi")
	}
	o, _ := k.Sampler(SamplerOptions{B: 1, T: 5})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var ti Batch
	if err := o.Fill(ctx, &ti); !errors.Is(err, context.Canceled) {
		t.Fatalf("iptal hatası bekleniyordu: %v", err)
	}
	_ = o.Close()
	if err := o.Next(&ti); !errors.Is(err, ErrClosed) {
		t.Fatalf("ErrKapali bekleniyordu: %v", err)
	}
}

func TestSequentialReader(t *testing.T) {
	checkShardLeak(t)
	lengths := []int{100, 5, 37}
	d := encodedDataset(t, lengths...)
	k, _ := OpenDataset(d, DatasetOptions{})
	const T = 8
	so, err := k.Sequential(SequentialOptions{B: 5, T: T})
	if err != nil {
		t.Fatal(err)
	}
	defer so.Close()
	var expected [][]int32
	for p, n := range lengths {
		for begin := 0; begin+T+1 <= n; begin += T {
			s := make([]int32, T+1)
			for j := range s {
				s[j] = int32(p*1_000_000 + begin + j)
			}
			expected = append(expected, s)
		}
	}
	if so.WindowCount() != int64(len(expected)) {
		t.Fatalf("pencere sayısı %d != %d", so.WindowCount(), len(expected))
	}
	for dtype := 0; dtype < 2; dtype++ {
		var taken [][]int32
		var ti Batch
		for {
			err := so.Fill(context.Background(), &ti)
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
			for r := 0; r < ti.B; r++ {
				taken = append(taken, append([]int32(nil), ti.Row(r)...))
			}
		}
		if !reflect.DeepEqual(taken, expected) {
			t.Fatalf("sıralı okuma yanlış:\n%v\n%v", taken, expected)
		}
		if len(expected)%5 == 0 {
			t.Fatal("test son toplu işin eksik olmasını denetlemeli")
		}
		if err := so.Fill(context.Background(), &ti); err != io.EOF {
			t.Fatal("bitişten sonra io.EOF bekleniyordu")
		}
		_ = so.Rewind()
	}
	// Adım T+1: üst üste binmeyen pencereler.
	so2, _ := k.Sequential(SequentialOptions{B: 100, T: T, Stride: T + 1})
	defer so2.Close()
	var ti Batch
	if err := so2.Fill(context.Background(), &ti); err != nil {
		t.Fatal(err)
	}
	if ti.B != 100/9+37/9 || ti.Row(1)[0] != 9 {
		t.Fatalf("adım yanlış: B=%d, %v", ti.B, ti.Row(1))
	}
}

// benchDataset, 4 parça × 4M tokenlık sentetik '<u2' küme kurar.
func benchDataset(b *testing.B, mapping bool) *Dataset {
	b.Helper()
	d := b.TempDir()
	for i := 0; i < 4; i++ {
		writeU16(b, d, shardName(i), fakeU16(4<<20, uint64(i)))
	}
	k, err := OpenDataset(d, DatasetOptions{Shard: ShardOptions{NoMmap: !mapping}})
	if err != nil {
		b.Fatal(err)
	}
	return k
}

func benchSampler(b *testing.B, mapping bool) {
	const B, T = 32, 1024
	k := benchDataset(b, mapping)
	o, err := k.Sampler(SamplerOptions{B: B, T: T, Seed: 1})
	if err != nil {
		b.Fatal(err)
	}
	defer o.Close()
	var ti Batch
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := o.Next(&ti); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportMetric(float64(b.N)*B*T/b.Elapsed().Seconds(), "token/s")
}

func BenchmarkSamplerMmap(b *testing.B)   { benchSampler(b, true) }
func BenchmarkSamplerReadAt(b *testing.B) { benchSampler(b, false) }
