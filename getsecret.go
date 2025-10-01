package main

import (
	"encoding/json"
	"net/http"
)

type SecretMetadata struct {
	//created      string
	//updated      string
	//status       string
	//name         *string
	//secretType   string `json:"secret_type"`
	//algorithm    *string
	//bitLength    *int `json:"bit_length"`
	//mode         *string
	//creator_id   *string
	//contentTypes map[string]string `json:"content_types"`
	SecretRef string `json:"secret_ref"`
}

type FetchSecretsResponse struct {
	Secrets []SecretMetadata `json:"secrets"`
	Next    *string          `json:"next"`
	Total   int              `json:"total"`
}

/*
*
1. Attempt to retrieve the secret from our local cache
2. If not found, try and find its uuid from our local cache
3. If not found, query barbican for the uuid
4. Once the uuid is found, query barbican for the secret if not cached
*/
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
	res.Write([]byte(data))
}
