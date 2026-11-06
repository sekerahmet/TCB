package process_test

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Gizli adlı değişken aktarılmaz, geri kalan ortam gider (21.6, 19.4).
func TestSecretNamedVariablesAreNotPassed(t *testing.T) {
	prog := helperProgram(t)
	t.Setenv("TR_DENEME_TOKEN", "hf_cokgizlideger12345")
	t.Setenv("TR_DENEME_YOL", "açık")
	got, err := run(t, `TR_String(TR_run_process(`+prog+`, ["env", "TR_DENEME_TOKEN", "TR_DENEME_YOL"]))`)
	if err != nil || got != "<yok>|açık" {
		t.Errorf("%s (%v)", got, err)
	}
}

// TR_env: ekler ya da değiştirir, geri kalanı silmez (21.6).
func TestEnvArgumentAddsAndReplaces(t *testing.T) {
	prog := helperProgram(t)
	t.Setenv("TR_DENEME_ESKI", "eski")
	t.Setenv("TR_DENEME_KALAN", "kalan")
	got, err := run(t, `TR_String(TR_run_process(`+prog+`, ["env", "TR_DENEME_ESKI", "TR_DENEME_KALAN", "TR_DENEME_YENI"], TR_env: {"TR_DENEME_ESKI": "değişti", "TR_DENEME_YENI": "çğü ="}))`)
	if err != nil || got != "değişti|kalan|çğü =" {
		t.Errorf("%s (%v)", got, err)
	}
	if got, err := run(t, `TR_String(TR_run_process(`+prog+`, ["env", "TR_DENEME_KALAN"], TR_env: {}))`); err != nil || got != "kalan" {
		t.Errorf("boş sözlük: %s (%v)", got, err)
	}
}

// Gizli değer TR_env: ile açıkça verilir ve süzgece girer: standart hatadan iletiye geçmez, ileti onu
// süzülen ad saymaz (21.6, DENETİM 193 N2).
func TestExplicitSecretReachesTheProgramAndIsMasked(t *testing.T) {
	prog := helperProgram(t)
	got, m, err := commandLine(t, `TR_String(TR_run_process(`+prog+`, ["env", "HF_TOKEN"], TR_env: {"HF_TOKEN": "hf_dosyadanokunan99"}))`)
	if err != nil || got != "hf_dosyadanokunan99" {
		t.Errorf("%s (%v)", got, err)
	}
	if s := m.Mask("hf_dosyadanokunan99"); strings.Contains(s, "hf_") {
		t.Errorf("değer süzgece girmedi: %s", s)
	}
	t.Setenv("HF_TOKEN", "hf_ortamdakidegerim")
	_, err = run(t, `TR_run_process(`+prog+`, ["exit", "1", "anahtar hf_dosyadanokunan99 reddedildi"], TR_env: {"HF_TOKEN": "hf_dosyadanokunan99"})`)
	if err == nil || strings.Contains(err.Error(), "hf_dosya") || strings.Contains(err.Error(), "HF_TOKEN") {
		t.Errorf("%v", err)
	}
}

// Başarısız bitişte ileti süzülen adları söyler, değerlerini söylemez, TR_env: yazımını verir ve yazım
// çalışır; başarılı bitişte ve süzülen yoksa söylemez (21.6).
func TestFailureNamesTheFilteredVariables(t *testing.T) {
	prog := helperProgram(t)
	t.Setenv("TR_DENEME_TOKEN", "hf_cokgizlideger12345")
	t.Setenv("TR_DENEME_API_KEY", "sk_baskagizlideger678")
	_, err := run(t, `TR_run_process(`+prog+`, ["exit", "1"])`)
	if err == nil {
		t.Fatal("hata bekleniyordu")
	}
	text := err.Error()
	for _, want := range []string{"TR_DENEME_API_KEY", "TR_DENEME_TOKEN", "`TR_env: {\"", `": TR_env("`} {
		if !strings.Contains(text, want) {
			t.Errorf("%q yok: %s", want, text)
		}
	}
	if strings.Contains(text, "hf_cok") || strings.Contains(text, "sk_baska") {
		t.Errorf("değer göründü: %s", text)
	}
	got, err := run(t, `TR_String(TR_run_process(`+prog+`, ["env", "TR_DENEME_API_KEY"], TR_env: {"TR_DENEME_API_KEY": TR_env("TR_DENEME_API_KEY")}))`)
	if err != nil || got != "sk_baskagizlideger678" {
		t.Errorf("öneri: %s (%v)", got, err)
	}
}

// Ad 19.1'in kuralına uyar; değerde boş bayt hatadır, ileti değeri söylemez; tipler denetlenir (21.6).
func TestEnvArgumentIsChecked(t *testing.T) {
	prog := helperProgram(t)
	for _, d := range []struct {
		env  string
		want messages.Code
	}{
		{`{"": "x"}`, messages.H7231},
		{`{"A=B": "x"}`, messages.H7231},
		{`{"=": "x"}`, messages.H7231},
		{`{"A\u{0}B": "x"}`, messages.H7231},
		{`{"A": "x\u{0}gizli"}`, messages.H7505},
		{`{"A": 1}`, messages.H7507},
		{`{1: "x"}`, messages.H7507},
		{`"A=B"`, messages.H7507},
		{`["A=B"]`, messages.H7507},
	} {
		_, err := run(t, `TR_run_process(`+prog+`, ["args"], TR_env: `+d.env+`)`)
		if !messages.Has(err, d.want) {
			t.Errorf("%s: %s bekleniyordu: %v", d.env, d.want, err)
		}
		if err != nil && strings.Contains(err.Error(), "gizli") {
			t.Errorf("%s: değer iletide: %v", d.env, err)
		}
	}
}

// Windows'ta ad büyük küçük harf ayırmaz: küçük harfli gizli ad da süzülür, TR_env: var olanı
// harfi farklı yazılsa da değiştirir (21.6).
func TestEnvNamesIgnoreCaseOnWindows(t *testing.T) {
	if !windows {
		t.Skip("Windows kuralı")
	}
	prog := helperProgram(t)
	t.Setenv("hf_token", "hf_kucukharflideger")
	got, err := run(t, `TR_String(TR_run_process(`+prog+`, ["env", "HF_TOKEN", "hf_token"]))`)
	if err != nil || got != "<yok>|<yok>" {
		t.Errorf("süzme: %s (%v)", got, err)
	}
	got, err = run(t, `TR_String(TR_run_process(`+prog+`, ["env", "HF_TOKEN"], TR_env: {"Hf_Token": "hf_acikverilendeger"}))`)
	if err != nil || got != "hf_acikverilendeger" {
		t.Errorf("açık verme: %s (%v)", got, err)
	}
	_, err = run(t, `TR_run_process(`+prog+`, ["exit", "1"], TR_env: {"HF_TOKEN": "hf_acikverilendeger"})`)
	if err == nil || strings.Contains(strings.ToLower(err.Error()), "hf_token") {
		t.Errorf("açık verilen süzülmüş sayılmaz: %v", err)
	}
	t.Setenv("TR_Deneme_Harf", "eski")
	got, err = run(t, `TR_String(TR_run_process(`+prog+`, ["env", "TR_DENEME_HARF"], TR_env: {"tr_deneme_harf": "yeni"}))`)
	if err != nil || got != "yeni" {
		t.Errorf("değiştirme: %s (%v)", got, err)
	}
}
