package model

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAmount_String(t *testing.T) {
	tests := []struct {
		amount Amount
		want   string
	}{
		{amount: 0, want: "0"},
		{amount: 50000, want: "500"},
		{amount: 50050, want: "500.5"},
		{amount: 50005, want: "500.05"},
		{amount: 72998, want: "729.98"},
		{amount: 1, want: "0.01"},
		{amount: 10, want: "0.1"},
		{amount: -150, want: "-1.5"},
		{amount: -5, want: "-0.05"},
		{amount: math.MaxInt64, want: "92233720368547758.07"},
		{amount: math.MinInt64, want: "-92233720368547758.08"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.amount.String())
		})
	}
}

func TestParseAmount(t *testing.T) {
	tests := []struct {
		in      string
		want    Amount
		wantErr bool
	}{
		{in: "751", want: 75100},
		{in: "500.5", want: 50050},
		{in: "500.50", want: 50050},
		{in: "500.500", want: 50050},
		{in: "0.05", want: 5},
		{in: "0.01", want: 1},
		{in: "0", want: 0},
		{in: "729.98", want: 72998},
		{in: "-1.5", want: -150},
		{in: "92233720368547758.07", want: math.MaxInt64},

		{in: "1.234", wantErr: true},
		{in: "0.001", wantErr: true},
		{in: "7.51e2", wantErr: true},
		{in: "1E3", wantErr: true},
		{in: `"751"`, wantErr: true},
		{in: "", wantErr: true},
		{in: "-", wantErr: true},
		{in: ".5", wantErr: true},
		{in: "5.", want: 500},
		{in: "abc", wantErr: true},
		{in: "true", wantErr: true},
		{in: "92233720368547758.08", wantErr: true},
		{in: "100000000000000000000", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := ParseAmount(tt.in)
			if tt.wantErr {
				require.ErrorIs(t, err, ErrInvalidAmount)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseAmount_RoundTrip(t *testing.T) {
	for _, a := range []Amount{0, 1, 10, 99, 100, 50050, 72998, 75100, -150, math.MaxInt64, math.MinInt64 + 1} {
		got, err := ParseAmount(a.String())
		require.NoError(t, err)
		assert.Equal(t, a, got)
	}
}

func TestParseAmountRounded(t *testing.T) {
	tests := []struct {
		in      string
		want    Amount
		wantErr bool
	}{
		{in: "500", want: 50000},
		{in: "729.98", want: 72998},
		{in: "9.25875", want: 926},
		{in: "729.985", want: 72999},
		{in: "729.984999", want: 72998},
		{in: "0.005", want: 1},
		{in: "0.004", want: 0},
		{in: "-0.005", want: -1},
		{in: "-1.234", want: -123},
		{in: "7.51e2", want: 75100},
		{in: "1E-7", want: 0},
		{in: "1.5e-2", want: 2},
		{in: "0", want: 0},
		{in: "92233720368547758.07", want: math.MaxInt64},

		{in: "92233720368547758.08", wantErr: true},
		{in: "1/3", wantErr: true},
		{in: "", wantErr: true},
		{in: "abc", wantErr: true},
		{in: `"5"`, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := ParseAmountRounded(tt.in)
			if tt.wantErr {
				require.ErrorIs(t, err, ErrInvalidAmount)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
