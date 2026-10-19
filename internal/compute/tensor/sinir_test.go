package tensor

import (
	"testing"
)

// Bulgu 13 ve 14: tanımsız sonuçlar sessizce 0 dönmez: boş tensörün/boş eksenin ortalaması
// ve bütün hedefleri yok sayılan çapraz entropi açık hata verir; 0 boyutlu girdide
// LayerNorm/RMSNorm biçimli bir hata verir (ham indeks paniği değil).
func TestTanimsizSonuclarHata(t *testing.T) {
	panikIcerir(t, "MeanAll(boş)", "ortalaması tanımsız", func() { Zeros(0).MeanAll() })
	panikIcerir(t, "Mean(boş eksen)", "ortalaması tanımsız", func() { Zeros(3, 0).Mean(1, false) })
	// Boş olmayan eksenin boş tensördeki ortalaması tanımlıdır (sonuç boş).
	if n := Zeros(3, 0).Mean(0, false).Numel(); n != 0 {
		t.Fatalf("[3,0] eksen 0 ortalaması %d elemanlı", n)
	}
	if v := Zeros(0).SumAll().Item(); v != 0 {
		t.Fatalf("boş toplam %v", v)
	}
	logit := New([]float32{1, 2, 3, 4}, 2, 2)
	panikIcerir(t, "CE yok sayılan", "geçerli hedef yok", func() { CrossEntropy(logit, []int{-1, -1}) })
	panikIcerir(t, "CE C=0", "geçerli hedef yok", func() { CrossEntropy(Zeros(2, 0), nil) })
	if v := CrossEntropy(logit, []int{-1, 1}).Item(); v != v || v <= 0 {
		t.Fatalf("tek geçerli hedefle kayıp %v", v)
	}
	panikIcerir(t, "LayerNorm 0 boyut", "en az 1 boyutlu", func() { LayerNorm(Scalar(1), nil, nil, 1e-5) })
	panikIcerir(t, "RMSNorm 0 boyut", "en az 1 boyutlu", func() { RMSNorm(Scalar(1), nil, 1e-5) })
}
