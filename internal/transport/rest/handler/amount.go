package handler

import (
	"errors"
	"strconv"
	"strings"

	"github.com/MaxPa1/Gofermart/internal/model"
)

var errInvalidAmount = errors.New("invalid amount")

type jsonAmount model.Amount

func (a jsonAmount) MarshalJSON() ([]byte, error) {
	return []byte(formatAmount(model.Amount(a))), nil
}

func (a *jsonAmount) UnmarshalJSON(data []byte) error {
	s := string(data)
	if s == "null" {
		return nil
	}
	v, err := parseAmount(s)
	if err != nil {
		return err
	}
	*a = jsonAmount(v)
	return nil
}

func parseAmount(s string) (model.Amount, error) {
	sign := ""
	if strings.HasPrefix(s, "-") {
		sign, s = "-", s[1:]
	}

	whole, frac, _ := strings.Cut(s, ".")
	frac = strings.TrimRight(frac, "0")
	if !isDigits(whole) || len(frac) > 2 || (frac != "" && !isDigits(frac)) {
		return 0, errInvalidAmount
	}

	n, err := strconv.ParseInt(sign+whole+frac+strings.Repeat("0", 2-len(frac)), 10, 64)
	if err != nil {
		return 0, errInvalidAmount
	}
	return model.Amount(n), nil
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
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
