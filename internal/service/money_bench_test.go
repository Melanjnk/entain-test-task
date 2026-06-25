package service

import "testing"

func BenchmarkParseMoney(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = parseMoney("10.15")
	}
}
