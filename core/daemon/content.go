package daemon

import (
	"bytes"
	"encoding/json"
	"errors"

	"github.com/pardnchiu/toriidb/core/store"
)

func (r *Record) Value() (string, error) {
	return encodeContent(r.Content)
}

func encodeContent(raw json.RawMessage) (string, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return "", errors.New("content is required")
	}

	var text string
	if err := json.Unmarshal(trimmed, &text); err == nil {
		return text, nil
	}

	var buf bytes.Buffer
	if err := json.Compact(&buf, trimmed); err != nil {
		return "", errors.New("content is not valid json")
	}
	return buf.String(), nil
}

func decodeContent(valueType store.ValueType, value string) any {
	switch valueType {
	case store.TypeJSON, store.TypeInt, store.TypeFloat, store.TypeBool:
		if json.Valid([]byte(value)) {
			return json.RawMessage(value)
		}
	}
	return value
}
