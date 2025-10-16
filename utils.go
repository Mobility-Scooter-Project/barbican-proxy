package main

import (
	"bytes"
	"encoding/json"
)

// MapToBuffer converts a map to a bytes.Buffer containing its JSON representation.
func MapToBuffer(m *map[string]any) *bytes.Buffer {
	data, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	return bytes.NewBuffer(data)
}
