package formatter

import "testing"

// Devam satırında açılan gövde deyimin girintisine göredir.
func TestContinuationBody(t *testing.T) {
	cases := []struct{ ad, input, expected string }{
		{"devam satırında gövde", "eğer a ve\nb {\nyazdır(1)\n}\nyazdır(2)\n",
			"eğer a ve\n    b {\n    yazdır(1)\n}\nyazdır(2)\n"},
		{"parantezli koşul devamı", "eğer (a ve\nb) {\nyazdır(1)\n}\n",
			"eğer (a ve\n        b) {\n    yazdır(1)\n}\n"},
		{"iç blokta devam", "döngü i içinde 0..3 {\neğer i > 0 veya\ni < 2 {\nyazdır(i)\n}\n}\n",
			"döngü i içinde 0..3 {\n    eğer i > 0 veya\n        i < 2 {\n        yazdır(i)\n    }\n}\n"},
		// Bir argümanın içindeki gövde (parantez açıkken) eskisi gibi satırın girintisine göredir.
		{"devam satırında fonksiyon argümanı", "x = 1 +\nf(fonksiyon() {\nyazdır(1)\n})\n",
			"x = 1 +\n    f(fonksiyon() {\n        yazdır(1)\n    })\n"},
		// Ayrıştırıcı yeni satırdaki '-x'i önceki ifadeyle (eğer de bir ifadedir) ikili eksi okur;
		// biçim bu okumayı gösterir.
		{"bloktan sonra eksi", "eğer a {\nb()\n}\n-x\n", "eğer a {\n    b()\n}\n- x\n"},
	}
	for _, d := range cases {
		t.Run(d.ad, func(t *testing.T) {
			output := formatT(t, d.input)
			if output != d.expected {
				t.Fatalf("beklenen:\n%s\nalınan:\n%s", d.expected, output)
			}
			if second := formatT(t, output); second != output {
				t.Fatalf("idempotent değil:\n%q\n->\n%q", output, second)
			}
		})
	}
}
