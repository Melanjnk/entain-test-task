package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseMoney(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr error
	}{
		{name: "valid two decimals", input: "10.15", want: "10.15"},
		{name: "valid integer", input: "5", want: "5.00"},
		{name: "valid one decimal", input: "1.5", want: "1.50"},
		{name: "zero rejected", input: "0", wantErr: ErrInvalidAmount},
		{name: "negative rejected", input: "-1.00", wantErr: ErrInvalidAmount},
		{name: "too many decimals", input: "1.234", wantErr: ErrInvalidAmount},
		{name: "garbage", input: "abc", wantErr: ErrInvalidAmount},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseMoney(tt.input)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got.StringFixed(2))
		})
	}
}
