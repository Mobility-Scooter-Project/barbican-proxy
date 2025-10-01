package main

import (
	"encoding/json"
	"net/http"
)

type UploadSecretRequestBody struct {
	Container string
	Name      string
	Payload   string
}

func (s *AppService) UploadSecretHandler(res http.ResponseWriter, req *http.Request) {
	var body UploadSecretRequestBody
	json.NewDecoder(req.Body).Decode(&body)
	err := s.UploadSecretToContainer(req.Context(), body.Container, body.Name, body.Payload)

	if err != nil {
		res.Header().Set("Content-Type", "application/json")
		res.WriteHeader(err.Status)
		jsonErr := map[string]string{"error": err.Message}
		jsonBytes, _ := json.Marshal(jsonErr)
		res.Write(jsonBytes)
		return
	}

	res.WriteHeader(http.StatusOK)
	res.Write([]byte("OK"))
}
