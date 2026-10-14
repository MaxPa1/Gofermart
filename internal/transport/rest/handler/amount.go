package handler

import (
	"strconv"

	"github.com/MaxPa1/Gofermart/internal/model"
)

type jsonAmount model.Amount

func (a jsonAmount) MarshalJSON() ([]byte, error) {
	return []byte(formatAmount(model.Amount(a))), nil
}

func formatAmount(a model.Amount) string {
	n := uint64(a)
	sign := ""
	if a < 0 {
		n = -n
		sign = "-"
	}

	whole := strconv.FormatUint(n/100, 10)
	frac := n % 100
	switch {
	case frac == 0:
		return sign + whole
	case frac%10 == 0:
		return sign + whole + "." + strconv.FormatUint(frac/10, 10)
	case frac < 10:
		return sign + whole + ".0" + strconv.FormatUint(frac, 10)
	default:
		return sign + whole + "." + strconv.FormatUint(frac, 10)
	}
}
