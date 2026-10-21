package ml

import (
	"bytes"
	"testing"
)

// Kural (dil tanımı 15.47): başlık birebir okunur: alan adları büyük-küçük harf duyarlı, her
// nesnede (tensör kaydı ve __metadata__ dahil) yinelenen anahtar hatadır, shape ve data_offsets'te
// null yoktur, __metadata__ değerleri metindir, eşi olmayan UTF-16 vekil kaçışı geçersizdir.
// (Gemini gerileme denetimi, Bulgu 4.)
func TestSafetensorsHeaderIsStrict(t *testing.T) {
	cases := []struct {
		name   string
		header string
	}{
		{"metadata_null_deger", `{"__metadata__":{"a":null}}`},
		{"shape_icinde_null", `{"w":{"dtype":"F32","shape":[null],"data_offsets":[0,0]}}`},
		{"data_offsets_icinde_null", `{"w":{"dtype":"F32","shape":[0],"data_offsets":[null,null]}}`},
		{"buyuk_harfli_alan_adlari", `{"w":{"DTYPE":"F32","Shape":[0],"DATA_OFFSETS":[0,0]}}`},
		{"girdi_icinde_yinelenen_alan", `{"w":{"dtype":"I64","dtype":"F32","shape":[0],"data_offsets":[0,0]}}`},
		{"metadata_icinde_yinelenen_anahtar", `{"__metadata__":{"a":"1","a":"2"}}`},
		{"eslenmemis_utf16_vekil_kacisi", `{"\ud800":{"dtype":"F32","shape":[0],"data_offsets":[0,0]}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			file := safetensorsBytes(tc.header, nil)
			if _, err := decodeTensorFile(bytes.NewReader(file), int64(len(file))); err == nil {
				t.Errorf("%s: Python safetensors 0.8.0 ve §15.47 tarafından reddedilen başlık kabul edildi: %s", tc.name, tc.header)
			}
		})
	}
}
