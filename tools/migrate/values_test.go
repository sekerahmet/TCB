package main

import "testing"

// Kural (3.7): sözlük değişmezinin çıplak anahtarı metin anahtarı olur (kodlu üye kimliğin adıyla),
// sözlüğe nokta ile okuma ve atama köşeli parantez olur; yapının alanı, modülün üyesi ve TR_ ile
// yazılmış üye çevrilmez. İki kez çevirmek bir kez çevirmekle aynıdır.
func TestValueRulesConvertDictKeysAndReads(t *testing.T) {
	in := "# TR 4\n" +
		"TR_import \"tr/katman\" TR_as k\n" +
		"TR_struct Nokta { x }\n" +
		"TR_var p = Nokta(1)\n" +
		"TR_var ş = {ağırlık: 1, TR_step: 2, \"c\": 3, 4: 5}\n" +
		"ş.ağırlık += p.x\n" +
		"TR_print(ş.ağırlık, ş.TR_step, ş.TR_keys(), k.TR_Embedding, \"\\(ş.ağırlık)\")\n"
	want := "# TR 4\n" +
		"TR_import \"tr/katman\" TR_as k\n" +
		"TR_struct Nokta { x }\n" +
		"TR_var p = Nokta(1)\n" +
		"TR_var ş = {\"ağırlık\": 1, \"step\": 2, \"c\": 3, 4: 5}\n" +
		"ş[\"ağırlık\"] += p.x\n" +
		"TR_print(ş[\"ağırlık\"], ş.TR_step, ş.TR_keys(), k.TR_Embedding, \"\\(ş[\"ağırlık\"])\")\n"
	out, probs := convertValues("", in)
	if len(probs) > 0 {
		t.Fatalf("beklenmeyen sorun: %v", probs)
	}
	if out != want {
		t.Errorf("çeviri:\n%s\nbeklenen:\n%s", out, want)
	}
	if again, _ := convertValues("", out); again != out {
		t.Errorf("ikinci çeviri metni değiştirdi:\n%s", again)
	}
	// ?. ile okuma elle çevrilir; sözdizimi hatalı metne dokunulmaz.
	if _, probs := convertValues("", "# TR 4\nTR_var d = {}\nTR_print(d?.a)\n"); len(probs) != 1 {
		t.Errorf("?. okuması bildirilmeliydi: %v", probs)
	}
	if out, probs := convertValues("", "# TR 4\nTR_var d = {a: \n"); out != "# TR 4\nTR_var d = {a: \n" || len(probs) != 1 {
		t.Errorf("sözdizimi hatalı metin: %q %v", out, probs)
	}
}
