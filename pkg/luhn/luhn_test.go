package luhn

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValid(t *testing.T) {
	tests := []struct {
		name   string
		number string
		want   bool
	}{
		{name: "example from specification", number: "12345678903", want: true},
		{name: "classic example", number: "79927398713", want: true},
		{name: "card number", number: "4561261212345467", want: true},
		{name: "single zero", number: "0", want: true},
		{name: "two digits with doubling over nine", number: "59", want: true},
		{name: "leading zeros do not change checksum", number: "0079927398713", want: true},
		{name: "longer than int64", number: strings.Repeat("0", 1000) + "18", want: true},

		{name: "wrong check digit", number: "12345678900", want: false},
		{name: "single non-zero digit", number: "1", want: false},
		{name: "swapped neighbour digits", number: "79927398731", want: false},
		{name: "empty", number: "", want: false},
		{name: "letters", number: "1234567890a", want: false},
		{name: "spaces inside", number: "1234 5678 903", want: false},
		{name: "trailing newline", number: "12345678903\n", want: false},
		{name: "sign", number: "-12345678903", want: false},
		{name: "non-ASCII digits", number: "١٢٣٤٥٦٧٨٩٠٣", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Valid(tt.number))
		})
	}
}

func BenchmarkValid(b *testing.B) {
	benchmarks := []struct {
		name   string
		number string
	}{
		{name: "order number", number: "12345678903"},
		{name: "card number", number: "4561261212345467"},
		{name: "wrong check digit", number: "12345678900"},
		{name: "letter at the end", number: "1234567890a"},
		{name: "1000 digits", number: strings.Repeat("0", 998) + "18"},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				Valid(bm.number)
			}
		})
	}
}
