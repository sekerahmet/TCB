package checker

import "testing"

// Kural (3.13, yanlış alarm vermez): iki kullanıcı tipi ancak ikisi de bilinen yapıysa ve ayrıysa
// uyumsuzdur; arayüz ya da bilinmeyen bir tip başka bir kullanıcı tipini karşılayabilir. Çekirdek
// tip ile kullanıcı tipi uyumsuzdur.
func TestUserTypeCompatibility(t *testing.T) {
	structs := compat{isStruct: func(n string) bool { return n == "Nokta" || n == "Kare" }}
	for _, tt := range []struct {
		k                compat
		expected, actual string
		want             bool
	}{
		{structs, "Nokta", "Nokta", true},
		{structs, "Nokta", "Kare", false},
		{structs, "Şekil", "Kare", true},
		{structs, "Kare", "Şekil", true},
		{compat{}, "Nokta", "Kare", true},
		{structs, typeInt, "Kare", false},
		{structs, "Kare", typeInt, false},
		{structs, "Kare", typeNull, true},
		{structs, typeNull, "Kare", false},
	} {
		if got := tt.k.types(tt.expected, tt.actual); got != tt.want {
			t.Errorf("types(%q, %q) = %v, beklenen %v", tt.expected, tt.actual, got, tt.want)
		}
	}
}
