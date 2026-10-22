package codeindex

import "encoding/json"

// EncodeIndex dizini girintili JSON olarak yazar: düğümler, kenarlar ve başvurular sıralıdır, aynı
// proje aynı baytları verir.
func EncodeIndex(idx *Index) ([]byte, error) {
	return marshalIndentLine(idx, " ")
}

// marshalIndentLine v'yi indent girintisiyle JSON'a çevirir ve sona satır sonu ekler.
func marshalIndentLine(v any, indent string) ([]byte, error) {
	data, err := json.MarshalIndent(v, "", indent)
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// DecodeIndex EncodeIndex'in yazdığı dizini okur.
func DecodeIndex(data []byte) (*Index, error) {
	idx := new(Index)
	if err := json.Unmarshal(data, idx); err != nil {
		return nil, err
	}
	return idx, nil
}
