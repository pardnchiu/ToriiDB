package store

import (
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"math"
	"strings"
)

const internalPrefix = "__torii:"

func encodeVector(vec []float32) string {
	if len(vec) == 0 {
		return ""
	}
	buf := make([]byte, 4*len(vec))
	for i, v := range vec {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(v))
	}
	return base64.StdEncoding.EncodeToString(buf)
}

func decodeVector(s string) ([]float32, error) {
	if s == "" {
		return nil, nil
	}
	raw, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("base64.StdEncoding.DecodeString: %w", err)
	}
	if len(raw)%4 != 0 {
		return nil, fmt.Errorf("vector byte length %d not multiple of 4", len(raw))
	}

	out := make([]float32, len(raw)/4)
	for i := range out {
		out[i] = math.Float32frombits(binary.LittleEndian.Uint32(raw[i*4:]))
	}
	return out, nil
}

func cosine(a, b []float32) (float64, bool) {
	if len(a) == 0 || len(a) != len(b) {
		return 0, false
	}
	var dot, na, nb float64
	for i := range a {
		av := float64(a[i])
		bv := float64(b[i])
		dot += av * bv
		na += av * av
		nb += bv * bv
	}
	if na == 0 || nb == 0 {
		return 0, false
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb)), true
}

func isInternal(key string) bool {
	return strings.HasPrefix(key, internalPrefix)
}
