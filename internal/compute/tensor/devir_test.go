package tensor

import "testing"

// Görünüm geri yayılımında gradyan devri: depolama kopyalanmadan ebeveyne geçer,
// ebeveynin gradyanı varsa üzerine eklenir; paylaşım ve birikim doğru kalır.
func TestGradyanDevri(t *testing.T) {
	// Yaprak, iki ayrı yeniden şekillendirme üzerinden iki kez kullanılır: biri
	// devralır, diğeri ekler.
	w := New([]float32{1, 2, 3, 4, 5, 6}, 2, 3).SetRequiresGrad(true)
	a := w.Reshape(3, 2).MulScalar(2)
	b := w.Reshape(6).Clone().MulScalar(3)
	Add(a.SumAll(), b.SumAll()).Backward()
	yakin(t, "iki yol", w.Grad().Data(), []float32{5, 5, 5, 5, 5, 5}, 0)

	// Gradyan birikimi: ikinci geri yayılım (yeni çizge) mevcut gradyana ekler.
	w.Reshape(3, 2).Contiguous().SumAll().Backward()
	yakin(t, "birikim", w.Grad().Data(), []float32{6, 6, 6, 6, 6, 6}, 0)
	w.ZeroGrad()

	// Devrik → bitişik → yeniden şekil zinciri ve ara düğümün başka bir tüketicisi.
	x := New([]float32{1, 2, 3, 4, 5, 6}, 2, 3).SetRequiresGrad(true)
	c := x.T().Contiguous() // (3, 2)
	d := c.Reshape(6)
	k := Add(d.Mul(d).SumAll(), c.Sum(0, false).SumAll())
	k.Backward()
	// d/dx Σ x² + Σ x = 2x + 1
	yakin(t, "zincir", x.Grad().Data(), []float32{3, 5, 7, 9, 11, 13}, 0)

	// Devredilen depolama yaprakta kalır; ara düğümler bırakılır.
	if c.grad != nil || d.grad != nil {
		t.Fatal("ara gradyanlar bırakılmalıydı")
	}
	// Yaprak gradyanı sonraki bir işlemle bozulmamalı.
	g := x.Grad().Data()
	y := Ones(2, 3).SetRequiresGrad(true)
	Mul(y.Reshape(6), Ones(6)).SumAll().Backward()
	yakin(t, "bağımsızlık", x.Grad().Data(), g, 0)
	yakin(t, "ikinci yaprak", y.Grad().Data(), []float32{1, 1, 1, 1, 1, 1}, 0)
}
