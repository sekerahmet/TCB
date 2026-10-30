package checker

import "strings"

// Gömülü kitaplığın (lib/) tipsiz kalabilen yerleri (6.1, 7.1): 4c'yi bekleyen genel ve birleşim
// tipleridir (birden çok tip alan parametre, alan ve dönüş; nedenleri
// internal/testdata/typed_code_known.txt'te). Bu yerler tanı vermez: kitaplığı aktaran kullanıcı
// düzeltemeyeceği bir tanı görmez. İstisna yalnız gömülü kitaplığın dosyası denetlenirken
// geçerlidir (SetLibraryFile); kullanıcının programı ve depodaki öbür dosyalar istisna almaz.
// Yerler satırla değil adla tutulur: "dosya sahip.ad", sahip fonksiyon, Yapı.metot ya da yapı;
// dönüş "dosya sahip ->". Liste yalnız küçülür; 4c'de kalkar (TestLibraryExceptionsAreUsed).
var libraryExceptions = map[string]bool{
	"layers.tr TR_type_GPT.TR_mem_config":                                true,
	"layers.tr TR_type_GPT.TR_mem_embed.TR_arg_token_ids":                true,
	"layers.tr TR_type_GPT.TR_mem_forward.TR_arg_token_ids":              true,
	"layers.tr TR_type_GPT.TR_mem_hidden_states.TR_arg_token_ids":        true,
	"layers.tr TR_type_GPT.TR_mem_loss.TR_arg_targets":                   true,
	"layers.tr TR_type_GPT.TR_mem_loss.TR_arg_token_ids":                 true,
	"layers.tr TR_type_GPT.TR_mem_run_blocks.TR_arg_token_ids":           true,
	"layers.tr TR_type_Embedding.TR_mem_forward.TR_arg_token_ids":        true,
	"layers.tr TR_type_PLEBlock.TR_mem_forward.TR_arg_token_ids":         true,
	"layers.tr TR_type_TransformerBlock.TR_mem_forward.TR_arg_token_ids": true,
	"optimizers.tr TR_fn_parameter_groups.TR_arg_model":                  true,
	"optimizers.tr parametre_kümeleri.model":                             true,
	"optimizers.tr TR_type_MuonAndAdamW.TR_ctx_init.TR_arg_model":        true,
	"optimizers.tr TR_type_MuonAndAdamW.TR_mem_muon":                     true,
	"optimizers_test.tr ters_eğit.kur":                                   true,
	"optimizers_test.tr tek_tek_mi ->":                                   true,
	"optimizers_test.tr tek_tek_mi.kur":                                  true,
	"pretrained.tr TR_fn_build_pretrained_model.TR_arg_config":           true,
	"pretrained.tr TR_fn_config_value ->":                                true,
	"pretrained.tr TR_fn_config_value.TR_arg_config":                     true,
	"pretrained.tr TR_fn_config_value.TR_arg_fallback":                   true,
	"pretrained.tr TR_fn_has_rope_scaling.TR_arg_config":                 true,
	"pretrained.tr TR_fn_map_hugging_face.TR_arg_config":                 true,
	"pretrained.tr TR_fn_rope_theta.TR_arg_config":                       true,
	"pretrained_test.tr qwen3_ayarı ->":                                  true,
	"training.tr TR_type_Run.TR_ctx_init.TR_arg_config":                  true,
	"training.tr TR_type_Run.TR_ctx_init.TR_arg_folder":                  true,
	"training.tr TR_type_Run.TR_ctx_init.TR_arg_objects":                 true,
	"training.tr TR_type_Run.TR_mem_finish.TR_arg_model":                 true,
}

// SetLibraryFile says the checked text is the embedded library's file name (lib/layers.tr →
// "layers.tr"): the library's named places that wait for 4c give no diagnostic there.
func (c *Checker) SetLibraryFile(name string) { c.libraryFile = name }

// excepted says place (sahip.ad ya da "sahip ->") is a library exception of the checked file and
// records that the exception was used.
func (c *Checker) excepted(place string) bool {
	if c.libraryFile == "" || !libraryExceptions[c.libraryFile+" "+place] {
		return false
	}
	if c.exceptionsUsed == nil {
		c.exceptionsUsed = map[string]bool{}
	}
	c.exceptionsUsed[c.libraryFile+" "+place] = true
	return true
}

// cutLast splits "a.b.c" at its last dot: "a.b", "c".
func cutLast(s string) (before, after string, ok bool) {
	i := strings.LastIndex(s, ".")
	if i < 0 {
		return "", s, false
	}
	return s[:i], s[i+1:], true
}
