package ml

import (
	"strings"
	"testing"
)

// Norm epsilon'u float32'de pozitif ve sonlu olmalıdır: NaN ve sonsuz (f <= 0 denetimini
// geçerdi) ya da float32'de 0'a düşen değer çıktıyı NaN yapardı.
func TestNormEpsilonMustBeFinite(t *testing.T) {
	for _, eps := range []string{"nan", "1e300", "1e-50", "0", "-1"} {
		code := "değişken nan = log(tensör([-1.0])).değer[0]\nkatman_normu(tensör([[1.0, 2.0]]), epsilon: " + eps + ")"
		if eps == "nan" {
			code = "değişken nan = log(tensör([-1.0])).değer[0]\nrms_normu(tensör([[1.0, 2.0]]), epsilon: nan)"
		}
		_, err := run(t, code)
		if err == nil || !strings.Contains(err.Error(), "pozitif ve sonlu") {
			t.Fatalf("epsilon %s: hata beklenirdi: %v", eps, err)
		}
	}
	mustRun(t, "katman_normu(tensör([[1.0, 2.0]]), epsilon: 0.001)")
}

// türev_izle Mantık olmalıdır: dolu bir metin ("yanlış") doğru sayılıp türevi açmaz.
func TestRequiresGradMustBeBool(t *testing.T) {
	for _, code := range []string{
		`tensör([1.0, 2.0], türev_izle: "yanlış")`,
		`tensör([1.0, 2.0], 1)`,
		`sıfırlar([2], türev_izle: "doğru")`,
	} {
		_, err := run(t, code)
		if err == nil || !strings.Contains(err.Error(), "doğru ya da yanlış olmalıdır") {
			t.Fatalf("%s: hata beklenirdi: %v", code, err)
		}
	}
	mustRun(t, "değişken q = tensör([1.0], türev_izle: yanlış)\nyazdır(q.gradyan)")
}
