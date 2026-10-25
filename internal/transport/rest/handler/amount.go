package handler

import (
	"github.com/MaxPa1/Gofermart/internal/model"
)

type jsonAmount model.Amount

// MarshalJSON кодирует сумму JSON-числом с точностью до сотых (например, 500.5) без перевода во
// float64.
func (a jsonAmount) MarshalJSON() ([]byte, error) {
	return []byte(model.Amount(a).String()), nil
}

// UnmarshalJSON разбирает JSON-число не более чем с двумя знаками после точки.
func (a *jsonAmount) UnmarshalJSON(data []byte) error {
	s := string(data)
	if s == "null" {
		return nil
	}
	v, err := model.ParseAmount(s)
	if err != nil {
		return err
	}
	*a = jsonAmount(v)
	return nil
}
