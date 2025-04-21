package ical

import (
	"bytes"
	"fmt"
	"io"

	"github.com/bounoable/ical/encode"
	"github.com/bounoable/ical/parse"
)

// Encode writes the .ics file for cal into w.
func Encode(cal Calendar, w io.Writer) error {
	return NewEncoder(w).Encode(parse.Calendar(cal))
}

// NewEncoder returns a new encode.Encoder that writes to w.
func NewEncoder(w io.Writer) *encode.Encoder {
	return encode.NewEncoder(w)
}

// Marshal returns the encoded bytes of cal.
func Marshal(cal Calendar) ([]byte, error) {
	var buf bytes.Buffer
	if err := Encode(cal, &buf); err != nil {
		return nil, fmt.Errorf("encode: %w", err)
	}
	return buf.Bytes(), nil
}
