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

func (s *AppService) GetSecretHandler(res http.ResponseWriter, req *http.Request) {
	container := req.PathValue("container")
	secretName := req.PathValue("name")
	ctx := req.Context()

	data, err := s.GetSecret(ctx, container, secretName)

	if err != nil {
		res.Header().Set("Content-Type", "application/json")
		res.WriteHeader(err.Status)
		jsonErr := map[string]string{"error": err.Message}
		jsonBytes, _ := json.Marshal(jsonErr)
		res.Write(jsonBytes)
		return
	}

	res.WriteHeader(http.StatusOK)
	respBody := map[string]string{"data": data}
	jsonBytes, _ := json.Marshal(respBody)
	res.Write(jsonBytes)
}

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

func (s *AppService) DeleteSecretHandler(res http.ResponseWriter, req *http.Request) {
	container := req.PathValue("container")
	secretName := req.PathValue("name")

	err := s.DeleteSecret(container, secretName)

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
