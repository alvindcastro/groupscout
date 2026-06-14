package storage

import (
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
)

func TestSanitizeText_ValidUTF8(t *testing.T) {
	cases := []string{
		"",
		"richmond_permits",
		"25 036523 000 00 B7",
		"application/pdf",
		"Hôtel • Congrès",
	}
	for _, s := range cases {
		got := sanitizeText(s)
		assert.Equal(t, s, got, "valid UTF-8 string must pass through unchanged")
	}
}

func TestSanitizeText_InvalidUTF8(t *testing.T) {
	// Raw bytes that are not valid UTF-8 (e.g. Latin-1 from a PDF font encoding).
	invalid := string([]byte{0x41, 0x80, 0x42}) // "A\x80B"
	assert.False(t, utf8.ValidString(invalid))

	got := sanitizeText(invalid)
	assert.True(t, utf8.ValidString(got), "result must be valid UTF-8")
	assert.Contains(t, got, "A")
	assert.Contains(t, got, "B")
}

func TestSanitizeText_PreservesASCII(t *testing.T) {
	s := "bcbid|12345|https://example.com"
	assert.Equal(t, s, sanitizeText(s))
}
