package model

import (
	"errors"
	"math/big"
	"strconv"
	"strings"
)

type Amount int64

const amountScale = 100

var ErrInvalidAmount = errors.New("invalid amount")

func (a Amount) String() string {
	n := uint64(a)
	sign := ""
	if a < 0 {
		n = -n
		sign = "-"
	}

	whole := strconv.FormatUint(n/amountScale, 10)
	frac := n % amountScale
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

func ParseAmount(s string) (Amount, error) {
	sign := ""
	if strings.HasPrefix(s, "-") {
		sign, s = "-", s[1:]
	}

	whole, frac, _ := strings.Cut(s, ".")
	frac = strings.TrimRight(frac, "0")
	if !isDigits(whole) || len(frac) > 2 || (frac != "" && !isDigits(frac)) {
		return 0, ErrInvalidAmount
	}

	n, err := strconv.ParseInt(sign+whole+frac+strings.Repeat("0", 2-len(frac)), 10, 64)
	if err != nil {
		return 0, ErrInvalidAmount
	}
	return Amount(n), nil
}

func ParseAmountRounded(s string) (Amount, error) {
	if strings.Contains(s, "/") {
		return 0, ErrInvalidAmount
	}
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		return 0, ErrInvalidAmount
	}

	r.Mul(r, big.NewRat(amountScale, 1))
	num, den := r.Num(), r.Denom()

	q, m := new(big.Int).QuoRem(num, den, new(big.Int))

	if new(big.Int).Abs(new(big.Int).Lsh(m, 1)).Cmp(den) >= 0 {
		q.Add(q, big.NewInt(int64(num.Sign())))
	}

	if !q.IsInt64() {
		return 0, ErrInvalidAmount
	}
	return Amount(q.Int64()), nil
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
