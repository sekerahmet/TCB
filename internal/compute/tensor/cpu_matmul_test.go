package tensor

import "testing"

// takePack havuza konamayacak (largest'tan büyük) tamponu tam boyda ayırır; küçüğünü
// ikinin kuvvetine yuvarlar.
func TestTakePackCapacity(t *testing.T) {
	y := &bufStack{largest: 1 << 10}
	if p := takePack(y, 1<<10+1); cap(*p) != 1<<10+1 {
		t.Fatalf("büyük tampon kapasitesi %d, %d bekleniyordu", cap(*p), 1<<10+1)
	}
	if p := takePack(y, 600); cap(*p) != 1024 || len(*p) != 600 {
		t.Fatalf("küçük tampon: uzunluk %d, kapasite %d", len(*p), cap(*p))
	}
}
