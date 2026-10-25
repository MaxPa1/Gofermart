// Package luhn проверяет номера алгоритмом Луна
package luhn

// Valid сообщает, является ли number непустой строкой ASCII-цифр с корректной контрольной суммой Луна.
func Valid(number string) bool {
	if number == "" {
		return false
	}

	sum := 0
	double := false
	for i := len(number) - 1; i >= 0; i-- {
		c := number[i]
		if c < '0' || c > '9' {
			return false
		}

		d := int(c - '0')
		if double {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
		double = !double
	}

	return sum%10 == 0
}
