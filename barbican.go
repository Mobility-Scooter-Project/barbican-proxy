package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"github.com/redis/go-redis/v9"
)

type SecretRef struct {
	SecretRef string `json:"secret_ref"`
}

type NamedSecretRef struct {
	SecretRef
	Name string `json:"name"`
}

type ContainerMetadata struct {
	Created      string           `json:"created"`
	Updated      string           `json:"updated"`
	CreatorId    string           `json:"creator_id"`
	Status       string           `json:"status"`
	Name         string           `json:"name"`
	Type         string           `json:"type"`
	ContainerRef string           `json:"container_ref"`
	SecretRefs   []NamedSecretRef `json:"secret_refs"`
}

type GetContainersResponse struct {
	Containers []ContainerMetadata `json:"containers"`
	Total      int                 `json:"total"`
}

func (s *AppService) makeBarbicanRequest(method, path string, body *map[string]any) (*http.Response, *HttpError) {
	url := os.Getenv("BARBICAN_URL") + path
	req, err := http.NewRequest(method, url, MapToBuffer(body))
	if err != nil {
		return nil, &HttpError{Status: http.StatusInternalServerError, Message: "Internal server error"}
	}

	req.Header.Set("X-Auth-Token", GetAuthToken())
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		s.Logger.Error("Error making Barbican request", "error", err, "url", url)
		return nil, &HttpError{Status: http.StatusInternalServerError, Message: "Internal server error"}
	}

	return resp, nil
}

func (s *AppService) LoadContainers() *HttpError {
	path := "/v1/containers"
	resp, err := s.makeBarbicanRequest("GET", path, nil)
	if err != nil || resp.StatusCode != http.StatusOK {
		s.Logger.Error("Error loading containers", "error", err, "status", resp.StatusCode)
		return err
	}
	defer resp.Body.Close()

	var body GetContainersResponse
	readErr := json.NewDecoder(resp.Body).Decode(&body)
	if readErr != nil {
		s.Logger.Error("Error reading Barbican response", "error", readErr)
		return &HttpError{Status: http.StatusInternalServerError, Message: "Internal server error"}
	}

	// Cache container metadata
	for _, container := range body.Containers {
		containerKey := "container:" + container.Name
		// grab the uuid at the end of the container ref
		containerValue := strings.Split(container.ContainerRef, "/")
		if len(containerValue) == 0 {
			s.Logger.Warn("Container ref is empty", "container", container.Name)
			continue
		}
		containerValueStr := containerValue[len(containerValue)-1]
		s.Logger.Info("Caching container", "key", containerKey, "value", containerValueStr)
		s.InMemoryCache.Set(containerKey, string(containerValueStr))

		// Cache secret refs for this container
		for _, secret := range container.SecretRefs {
			secretUuid := strings.Split(secret.SecretRef.SecretRef, "/")
			if len(secretUuid) == 0 {
				s.Logger.Warn("Secret ref is empty", "secretName", secret.Name)
				continue
			}
			secretUuidStr := secretUuid[len(secretUuid)-1]
			if redisErr := s.KV.HSet(context.Background(), containerKey, secret.Name, secretUuidStr).Err(); redisErr != nil {
				s.Logger.Error("Error saving secret to cache", "error", redisErr)
				return &HttpError{Status: http.StatusInternalServerError, Message: "Internal server error"}
			}
		}
	}

	return nil
}

func (s *AppService) UploadSecretToContainer(ctx context.Context, container string, secretName string, secretPayload string) *HttpError {
	containerUuid, ok := s.InMemoryCache.Get("container:" + container)

	if !ok {
		s.Logger.Warn("Container not found in cache", "container", container)
		return &HttpError{Status: http.StatusNotFound, Message: "Container not found"}
	}

	if containerUuid == nil {
		s.Logger.Warn("Container UUID is nil", "container", container)
		return &HttpError{Status: http.StatusNotFound, Message: "Container not found"}
	}

	// first upload the secret
	uploadSecretBody := map[string]any{
		"name":                     secretName,
		"payload":                  base64.StdEncoding.EncodeToString([]byte(secretPayload)),
		"payload_content_type":     "application/octet-stream",
		"payload_content_encoding": "base64",
		"secret_type":              "symmetric",
	}

	resp, err := s.makeBarbicanRequest("POST", "/v1/secrets", &uploadSecretBody)

	if err != nil {
		s.Logger.Error("Error uploading secret to Barbican", "error", err)
		return &HttpError{Status: http.StatusInternalServerError, Message: "Internal server error"}
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		s.Logger.Error("Error uploading secret to Barbican", "resp", string(bodyBytes))
		return &HttpError{Status: resp.StatusCode, Message: "Error uploading secret to Barbican"}
	}

	var secretResponse SecretRef
	decodeErr := json.NewDecoder(resp.Body).Decode(&secretResponse)
	if decodeErr != nil {
		s.Logger.Error("Error decoding Barbican response", "error", decodeErr)
		return &HttpError{Status: http.StatusInternalServerError, Message: "Internal server error"}
	}

	// now add the secret to the container
	addSecretBody := map[string]any{
		"secret_ref": secretResponse.SecretRef,
		"name":       secretName,
	}

	path := "/v1/containers/" + containerUuid.(string) + "/secrets"
	s.Logger.Info("Adding secret to container", "path", path, "body", addSecretBody)
	resp, err = s.makeBarbicanRequest("POST", path, &addSecretBody)

	if err != nil {
		s.Logger.Error("Error adding secret to container", "error", err)
		return &HttpError{Status: http.StatusInternalServerError, Message: "Internal server error"}
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		s.Logger.Error("Error adding secret to container", "resp", string(bodyBytes))
		return &HttpError{Status: resp.StatusCode, Message: "Error adding secret to container"}
	}

	// save the new container metadata to the cache
	secretUuid := strings.Split(secretResponse.SecretRef, "/")
	if len(secretUuid) == 0 {
		s.Logger.Warn("Secret ref is empty", "secretName", secretName)
		return &HttpError{Status: http.StatusInternalServerError, Message: "Internal server error"}
	}
	secretUuidStr := secretUuid[len(secretUuid)-1]

	// save the secret ref to the cache
	if redisErr := s.KV.HSet(ctx, "container:"+container, secretName, secretUuidStr).Err(); redisErr != nil {
		s.Logger.Error("Error saving secret to cache", "error", redisErr)
		return &HttpError{Status: http.StatusInternalServerError, Message: "Internal server error"}
	}

	return nil
}

func (s *AppService) GetSecret(ctx context.Context, container string, key string) (string, *HttpError) {
	// first get the secret ref from the cache
	secretRefUuid, err := s.KV.HGet(ctx, "container:"+container, key).Result()

	if err == redis.Nil {
		s.Logger.Warn("Secret not found in cache", "container", container, "key", key)
		return "", &HttpError{Status: http.StatusNotFound, Message: "Secret not found"}
	}

	if err != nil {
		s.Logger.Error("Error getting secret from cache", "error", err)
		return "", &HttpError{Status: http.StatusInternalServerError, Message: "Internal server error"}
	}

	// now get the secret from barbican
	path := "/v1/secrets/" + secretRefUuid + "/payload"
	resp, httpErr := s.makeBarbicanRequest("GET", path, nil)
	if httpErr != nil {
		s.Logger.Error("Error getting secret from Barbican", "error", httpErr)
		return "", httpErr
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		s.Logger.Error("Error getting secret from Barbican", "resp", string(bodyBytes))
		return "", &HttpError{Status: resp.StatusCode, Message: "Error getting secret from Barbican"}
	}

	bodyBytes, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		s.Logger.Error("Error reading Barbican response", "error", readErr)
		return "", &HttpError{Status: http.StatusInternalServerError, Message: "Internal server error"}
	}

	return string(bodyBytes), nil
}

func (s* AppService) CreateContainer(ctx context.Context, containerName string) *HttpError {
	createContainerBody := map[string]any{
		"name": containerName,
		"type": "generic",
	}

	resp, err := s.makeBarbicanRequest("POST", "/v1/containers", &createContainerBody)

	if err != nil {
		s.Logger.Error("Error creating container in Barbican", "error", err)
		return &HttpError{Status: http.StatusInternalServerError, Message: "Internal server error"}
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		s.Logger.Error("Error creating container in Barbican", "resp", string(bodyBytes))
		return &HttpError{Status: resp.StatusCode, Message: "Error creating container in Barbican"}
	}

	var containerResponse ContainerMetadata
	decodeErr := json.NewDecoder(resp.Body).Decode(&containerResponse)
	if decodeErr != nil {
		s.Logger.Error("Error decoding Barbican response", "error", decodeErr)
		return &HttpError{Status: http.StatusInternalServerError, Message: "Internal server error"}
	}

	// Cache container metadata
	containerKey := "container:" + containerResponse.Name
	// grab the uuid at the end of the container ref
	containerValue := strings.Split(containerResponse.ContainerRef, "/")
	if len(containerValue) == 0 {
		s.Logger.Warn("Container ref is empty", "container", containerResponse.Name)
		return &HttpError{Status: http.StatusInternalServerError, Message: "Internal server error"}
	}
	containerValueStr := containerValue[len(containerValue)-1]
	s.Logger.Info("Caching container", "key", containerKey, "value", containerValueStr)
	s.InMemoryCache.Set(containerKey, string(containerValueStr))

	return nil
}
