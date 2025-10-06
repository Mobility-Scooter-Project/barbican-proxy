package main

import (
	"encoding/json"
	"net/http"
)

type CreateContainerRequestBody struct {
	Name string
}

func (s *AppService) CreateContainerHandler(res http.ResponseWriter, req *http.Request) {
	var body CreateContainerRequestBody
	json.NewDecoder(req.Body).Decode(&body)
	containerName := body.Name

	httpErr := s.CreateContainer(req.Context(), containerName)
	if httpErr != nil {
		res.Header().Set("Content-Type", "application/json")
		res.WriteHeader(httpErr.Status)
		jsonErr := map[string]string{"error": httpErr.Message}
		jsonBytes, _ := json.Marshal(jsonErr)
		res.Write(jsonBytes)
		return
	}

	res.WriteHeader(http.StatusOK)
	res.Write([]byte("OK"))
}
