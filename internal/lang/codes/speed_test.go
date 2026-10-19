package codes

import "testing"

// Sözlük program başlarken bir kez yüklenir; başlangıç süresine katkısı küçük kalmalı.
func BenchmarkYukle(b *testing.B) {
	for i := 0; i < b.N; i++ {
		if _, err := Load("tr", trJSON); err != nil {
			b.Fatal(err)
		}
	}
}
