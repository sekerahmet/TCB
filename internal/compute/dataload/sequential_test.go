package dataload

import (
	"context"
	"io"
	"slices"
	"testing"
)

// siraliHepsi okuyucunun kalan bütün satırlarını ve her toplu işten sonraki durumu verir.
func sequentialAll(t *testing.T, so *SequentialReader) ([][]int32, []State) {
	t.Helper()
	var rows [][]int32
	var cases []State
	var ti Batch
	for {
		err := so.Fill(context.Background(), &ti)
		if err == io.EOF {
			return rows, cases
		}
		if err != nil {
			t.Fatal(err)
		}
		for i := 0; i < ti.B; i++ {
			rows = append(rows, slices.Clone(ti.Row(i)))
		}
		cases = append(cases, ti.State)
	}
}

// Sıralı okuyucu, bir toplu işin Durum'u Baslangic olarak verilince tam o noktadan sürer:
// kesilip sürdürülen okuma, kesintisiz okumayla aynı satırları verir (kısa parça atlanır,
// parça sınırında ve sonunda da).
func TestSequentialResumesFromStart(t *testing.T) {
	checkShardLeak(t)
	d := encodedDataset(t, 100, 5, 37, 9)
	k, err := OpenDataset(d, DatasetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	pick := SequentialOptions{B: 3, T: 8}
	so, err := k.Sequential(pick)
	if err != nil {
		t.Fatal(err)
	}
	whole, cases := sequentialAll(t, so)
	so.Close()
	readN := 0
	for i, dm := range cases {
		readN += pick.B
		readN = min(readN, len(whole))
		s := pick
		s.Start = &dm
		so2, err := k.Sequential(s)
		if err != nil {
			t.Fatal(err)
		}
		remaining, _ := sequentialAll(t, so2)
		so2.Close()
		if !slices.EqualFunc(remaining, whole[readN:], slices.Equal[[]int32]) {
			t.Fatalf("%d. toplu işten (durum %+v) sonra sürdürülen okuma farklı: %d satır, beklenen %d", i+1, dm, len(remaining), len(whole)-readN)
		}
	}
	if _, err := k.Sequential(SequentialOptions{B: 1, T: 8, Start: &State{Pos: 9}}); err == nil {
		t.Error("parça sayısını aşan başlangıç hata vermeliydi")
	}
	if _, err := k.Sequential(SequentialOptions{B: 1, T: 8, Start: &State{Pos: 4, Window: 3}}); err == nil {
		t.Error("bitmiş okuyucunun pencereli başlangıcı hata vermeliydi")
	}
}
