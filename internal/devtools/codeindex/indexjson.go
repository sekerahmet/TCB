package codeindex

import "encoding/json"

// EncodeIndex dizini girintili JSON olarak yazar: düğümler, kenarlar ve başvurular sıralıdır, aynı
// proje aynı baytları verir.
func EncodeIndex(idx *Index) ([]byte, error) {
	data, err := json.MarshalIndent(idx, "", " ")
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
