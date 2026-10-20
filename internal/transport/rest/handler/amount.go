package handler

import (
	"github.com/MaxPa1/Gofermart/internal/model"
)

type jsonAmount model.Amount

func (a jsonAmount) MarshalJSON() ([]byte, error) {
	return []byte(model.Amount(a).String()), nil
}

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
