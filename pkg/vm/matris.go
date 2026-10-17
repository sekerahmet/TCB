package vm

import (
	"fmt"
	"strings"
	"sync"
)

// ============================================================================
// 1. MATRİS TİPİ (Çok Çekirdekli Matris ve @ Operatörü)
// ============================================================================

type MatrixVal struct {
	Rows int
	Cols int
	Data [][]float64
}

func NewMatrix(rows, cols int) *MatrixVal {
	data := make([][]float64, rows)
	for i := range data {
		data[i] = make([]float64, cols)
	}
	return &MatrixVal{Rows: rows, Cols: cols, Data: data}
}

func (m *MatrixVal) Type() ValueType { return VAL_MATRIX }
func (m *MatrixVal) IsTruthy() bool  { return m.Rows > 0 && m.Cols > 0 }
func (m *MatrixVal) Inspect() string {
	var rows []string
	for i := 0; i < m.Rows; i++ {
		var cols []string
		for j := 0; j < m.Cols; j++ {
			cols = append(cols, fmt.Sprintf("%8.4f", m.Data[i][j]))
		}
		rows = append(rows, "  ["+strings.Join(cols, ", ")+"]")
	}
	return fmt.Sprintf("Matris(%dx%d) [\n%s\n]", m.Rows, m.Cols, strings.Join(rows, ",\n"))
}

func (m *MatrixVal) Add(other Value) (Value, error) {
	if om, ok := other.(*MatrixVal); ok {
		// Broadcasting (1xCols satır vektörü ekleme desteği)
		if m.Rows == om.Rows && m.Cols == om.Cols {
			res := NewMatrix(m.Rows, m.Cols)
			for i := 0; i < m.Rows; i++ {
				for j := 0; j < m.Cols; j++ {
					res.Data[i][j] = m.Data[i][j] + om.Data[i][j]
				}
			}
			return res, nil
		}
		if om.Rows == 1 && m.Cols == om.Cols {
			res := NewMatrix(m.Rows, m.Cols)
			for i := 0; i < m.Rows; i++ {
				for j := 0; j < m.Cols; j++ {
					res.Data[i][j] = m.Data[i][j] + om.Data[0][j]
				}
			}
			return res, nil
		}
		return nil, fmt.Errorf("Matris toplama boyut uyuşmazlığı: (%dx%d) ile (%dx%d)", m.Rows, m.Cols, om.Rows, om.Cols)
	}
	if scalar, ok := ToFloat(other); ok {
		res := NewMatrix(m.Rows, m.Cols)
		for i := 0; i < m.Rows; i++ {
			for j := 0; j < m.Cols; j++ {
				res.Data[i][j] = m.Data[i][j] + scalar
			}
		}
		return res, nil
	}
	return nil, fmt.Errorf("Matris ile %s toplanamaz", other.Type())
}

func (m *MatrixVal) Sub(other Value) (Value, error) {
	if om, ok := other.(*MatrixVal); ok {
		if m.Rows != om.Rows || m.Cols != om.Cols {
			return nil, fmt.Errorf("Matris çıkarma boyut uyuşmazlığı: (%dx%d) ile (%dx%d)", m.Rows, m.Cols, om.Rows, om.Cols)
		}
		res := NewMatrix(m.Rows, m.Cols)
		for i := 0; i < m.Rows; i++ {
			for j := 0; j < m.Cols; j++ {
				res.Data[i][j] = m.Data[i][j] - om.Data[i][j]
			}
		}
		return res, nil
	}
	if scalar, ok := ToFloat(other); ok {
		res := NewMatrix(m.Rows, m.Cols)
		for i := 0; i < m.Rows; i++ {
			for j := 0; j < m.Cols; j++ {
				res.Data[i][j] = m.Data[i][j] - scalar
			}
		}
		return res, nil
	}
	return nil, fmt.Errorf("Matris'ten %s çıkarılamaz", other.Type())
}

func (m *MatrixVal) Mul(other Value) (Value, error) {
	if om, ok := other.(*MatrixVal); ok {
		if m.Rows != om.Rows || m.Cols != om.Cols {
			return nil, fmt.Errorf("Eleman bazlı çarpım boyut uyuşmazlığı: (%dx%d) ile (%dx%d)", m.Rows, m.Cols, om.Rows, om.Cols)
		}
		res := NewMatrix(m.Rows, m.Cols)
		for i := 0; i < m.Rows; i++ {
			for j := 0; j < m.Cols; j++ {
				res.Data[i][j] = m.Data[i][j] * om.Data[i][j]
			}
		}
		return res, nil
	}
	if scalar, ok := ToFloat(other); ok {
		res := NewMatrix(m.Rows, m.Cols)
		for i := 0; i < m.Rows; i++ {
			for j := 0; j < m.Cols; j++ {
				res.Data[i][j] = m.Data[i][j] * scalar
			}
		}
		return res, nil
	}
	return nil, fmt.Errorf("Matris ile %s çarpılamaz", other.Type())
}

func (m *MatrixVal) Div(other Value) (Value, error) {
	if scalar, ok := ToFloat(other); ok {
		if scalar == 0 {
			return nil, fmt.Errorf("Matris sıfıra bölünemez")
		}
		res := NewMatrix(m.Rows, m.Cols)
		for i := 0; i < m.Rows; i++ {
			for j := 0; j < m.Cols; j++ {
				res.Data[i][j] = m.Data[i][j] / scalar
			}
		}
		return res, nil
	}
	return nil, fmt.Errorf("Matris %s tipine bölünemez", other.Type())
}

// MatMul ('@' operatörü): Büyük matrislerde tüm CPU çekirdeklerini paralel kullanır
func (m *MatrixVal) MatMul(other Value) (Value, error) {
	om, ok := other.(*MatrixVal)
	if !ok {
		return nil, fmt.Errorf("'@' (Matris çarpımı) sağ tarafta Matris bekler, %s verildi", other.Type())
	}
	if m.Cols != om.Rows {
		return nil, fmt.Errorf("Matris çarpımı ('@') boyut hatası: (%dx%d) @ (%dx%d) — sol sütun (%d) ile sağ satır (%d) eşit olmalıdır",
			m.Rows, m.Cols, om.Rows, om.Cols, m.Cols, om.Rows)
	}

	res := NewMatrix(m.Rows, om.Cols)
	if m.Rows >= 16 {
		var wg sync.WaitGroup
		for i := 0; i < m.Rows; i++ {
			wg.Add(1)
			go func(row int) {
				defer wg.Done()
				for k := 0; k < m.Cols; k++ {
					aik := m.Data[row][k]
					for j := 0; j < om.Cols; j++ {
						res.Data[row][j] += aik * om.Data[k][j]
					}
				}
			}(i)
		}
		wg.Wait()
	} else {
		for i := 0; i < m.Rows; i++ {
			for k := 0; k < m.Cols; k++ {
				aik := m.Data[i][k]
				for j := 0; j < om.Cols; j++ {
					res.Data[i][j] += aik * om.Data[k][j]
				}
			}
		}
	}
	return res, nil
}

func (m *MatrixVal) Transpose() *MatrixVal {
	res := NewMatrix(m.Cols, m.Rows)
	for i := 0; i < m.Rows; i++ {
		for j := 0; j < m.Cols; j++ {
			res.Data[j][i] = m.Data[i][j]
		}
	}
	return res
}

func (m *MatrixVal) GetProperty(prop string) (Value, error) {
	switch prop {
	case "şekil", "sekil", "boyut":
		return &ListVal{Elements: []Value{&IntVal{Val: int64(m.Rows)}, &IntVal{Val: int64(m.Cols)}}}, nil
	case "satır", "satir":
		return &IntVal{Val: int64(m.Rows)}, nil
	case "sütun", "sutun":
		return &IntVal{Val: int64(m.Cols)}, nil
	case "devrik", "T":
		return m.Transpose(), nil
	case "toplam":
		sum := 0.0
		for i := 0; i < m.Rows; i++ {
			for j := 0; j < m.Cols; j++ {
				sum += m.Data[i][j]
			}
		}
		return &FloatVal{Val: sum}, nil
	case "ortalama":
		sum := 0.0
		count := float64(m.Rows * m.Cols)
		if count == 0 {
			return &FloatVal{Val: 0}, nil
		}
		for i := 0; i < m.Rows; i++ {
			for j := 0; j < m.Cols; j++ {
				sum += m.Data[i][j]
			}
		}
		return &FloatVal{Val: sum / count}, nil
	case "liste":
		rows := make([]Value, m.Rows)
		for i := 0; i < m.Rows; i++ {
			cols := make([]Value, m.Cols)
			for j := 0; j < m.Cols; j++ {
				cols[j] = &FloatVal{Val: m.Data[i][j]}
			}
			rows[i] = &ListVal{Elements: cols}
		}
		return &ListVal{Elements: rows}, nil
	}
	return nil, fmt.Errorf("Matris üzerinde %q özelliği bulunamadı", prop)
}
