package tokenize

import (
	"reflect"
	"testing"
)

// Kural: bayt yedeği ("<0xXX>") tablosu bayt değerine göre kurulur (-1: yok) ve appendByteFallback
// yalnızca bütün baytların tokenı varsa ekler; biri eksikse dst'ye dokunmaz.
func TestByteFallbackTableAndAppend(t *testing.T) {
	table := initFallbackTable(map[string]int32{"<0x41>": 7, "<0x42>": 8, "<0xFF>": 9, "x": 1})
	if table[0x41] != 7 || table[0x42] != 8 || table[0xFF] != 9 || table[0x43] != -1 || table[0] != -1 {
		t.Fatalf("tablo yanlış: %v %v %v %v", table[0x41], table[0x42], table[0xFF], table[0x43])
	}
	got, ok := appendByteFallback([]int32{1}, &table, "AB")
	if !ok || !reflect.DeepEqual(got, []int32{1, 7, 8}) {
		t.Fatalf("AB: %v %v", got, ok)
	}
	got, ok = appendByteFallback([]int32{1}, &table, "ABC")
	if ok || !reflect.DeepEqual(got, []int32{1}) {
		t.Fatalf("ABC (C yok): %v %v", got, ok)
	}
}
