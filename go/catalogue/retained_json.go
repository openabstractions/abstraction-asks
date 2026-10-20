package catalogue

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

func decodeRetainedJSON(data []byte, v any) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return errors.New("asks: trailing application state")
	}
	return nil
}

func canonicalRetainedJSON(data []byte, v any) bool {
	canonical, err := json.Marshal(v)
	return err == nil && bytes.Equal(data, canonical)
}

func DecodeRetainedJSON(data []byte, v any) error   { return decodeRetainedJSON(data, v) }
func CanonicalRetainedJSON(data []byte, v any) bool { return canonicalRetainedJSON(data, v) }
