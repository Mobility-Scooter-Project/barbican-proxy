package main

import (
	"bytes"
	"encoding/json"
)

func MapToBuffer(m *map[string]any) *bytes.Buffer {
	data, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	return bytes.NewBuffer(data)
}
