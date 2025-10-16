package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
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

// extractUUIDFromRef extracts the UUID from the end of a Barbican reference URL
func extractUUIDFromRef(ref string) (string, error) {
	parts := strings.Split(ref, "/")
	if len(parts) == 0 {
		return "", fmt.Errorf("invalid reference format: %s", ref)
	}
	uuid := parts[len(parts)-1]
	if uuid == "" {
		return "", fmt.Errorf("empty UUID in reference: %s", ref)
	}
	return uuid, nil
}

// makeBarbicanRequest makes an HTTP request to the Barbican API with the given method, path, and body.
func (s *AppService) makeBarbicanRequest(method, path string, body *map[string]any) (*http.Response, *HttpError) {
	url := os.Getenv("BARBICAN_URL") + path
	req, err := http.NewRequest(method, url, MapToBuffer(body))
	if err != nil {
		s.Logger.Error("Failed to create HTTP request", "error", err, "url", url)
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

// LoadContainers loads container metadata from Barbican and caches it in memory and Redis.
func (s *AppService) LoadContainers() *HttpError {
	const path = "/v1/containers"

	resp, err := s.makeBarbicanRequest("GET", path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		s.Logger.Error("Error loading containers", "status", resp.StatusCode)
		return &HttpError{Status: resp.StatusCode, Message: "Failed to load containers"}
	}

	var body GetContainersResponse
	if readErr := json.NewDecoder(resp.Body).Decode(&body); readErr != nil {
		s.Logger.Error("Error reading Barbican response", "error", readErr)
		return &HttpError{Status: http.StatusInternalServerError, Message: "Internal server error"}
	}

	// Cache container metadata
	for _, container := range body.Containers {
		if err := s.cacheContainer(container); err != nil {
			s.Logger.Error("Failed to cache container", "container", container.Name, "error", err)
			continue
		}
	}

	return nil
}

// cacheContainer caches a single container and its secrets
func (s *AppService) cacheContainer(container ContainerMetadata) error {
	containerKey := "container:" + container.Name

	// Extract and cache container UUID
	containerUUID, err := extractUUIDFromRef(container.ContainerRef)
	if err != nil {
		s.Logger.Warn("Invalid container ref", "container", container.Name, "error", err)
		return err
	}

	s.Logger.Info("Caching container", "key", containerKey, "value", containerUUID)
	s.InMemoryCache.Set(containerKey, containerUUID)

	// Cache secret refs for this container
	for _, secret := range container.SecretRefs {
		secretUUID, err := extractUUIDFromRef(secret.SecretRef.SecretRef)
		if err != nil {
			s.Logger.Warn("Invalid secret ref", "secretName", secret.Name, "error", err)
			continue
		}

		if redisErr := s.KV.HSet(context.Background(), containerKey, secret.Name, secretUUID).Err(); redisErr != nil {
			s.Logger.Error("Error saving secret to cache", "error", redisErr)
			return redisErr
		}
	}

	return nil
}

// UploadSecretToContainer uploads a secret to a specified container in Barbican and updates the cache.
func (s *AppService) UploadSecretToContainer(ctx context.Context, container string, secretName string, secretPayload string) *HttpError {
	containerUUID, ok := s.InMemoryCache.Get("container:" + container)
	if !ok || containerUUID == nil {
		s.Logger.Warn("Container not found in cache", "container", container)
		return &HttpError{Status: http.StatusNotFound, Message: "Container not found"}
	}

	// Upload the secret first
	secretRef, err := s.uploadSecret(secretName, secretPayload)
	if err != nil {
		return err
	}

	// Add the secret to the container
	if err := s.addSecretToContainer(containerUUID.(string), secretRef, secretName); err != nil {
		return err
	}

	// Cache the secret reference
	secretUUID, extractErr := extractUUIDFromRef(secretRef)
	if extractErr != nil {
		s.Logger.Warn("Invalid secret ref returned", "secretName", secretName, "error", extractErr)
		return &HttpError{Status: http.StatusInternalServerError, Message: "Internal server error"}
	}

	if redisErr := s.KV.HSet(ctx, "container:"+container, secretName, secretUUID).Err(); redisErr != nil {
		s.Logger.Error("Error saving secret to cache", "error", redisErr)
		return &HttpError{Status: http.StatusInternalServerError, Message: "Internal server error"}
	}

	return nil
}

// uploadSecret uploads a secret to Barbican and returns the secret reference
func (s *AppService) uploadSecret(secretName, secretPayload string) (string, *HttpError) {
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
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		s.Logger.Error("Error uploading secret to Barbican", "resp", string(bodyBytes))
		return "", &HttpError{Status: resp.StatusCode, Message: "Error uploading secret to Barbican"}
	}

	var secretResponse SecretRef
	if decodeErr := json.NewDecoder(resp.Body).Decode(&secretResponse); decodeErr != nil {
		s.Logger.Error("Error decoding Barbican response", "error", decodeErr)
		return "", &HttpError{Status: http.StatusInternalServerError, Message: "Internal server error"}
	}

	return secretResponse.SecretRef, nil
}

// addSecretToContainer adds a secret to a container in Barbican
func (s *AppService) addSecretToContainer(containerUUID, secretRef, secretName string) *HttpError {
	addSecretBody := map[string]any{
		"secret_ref": secretRef,
		"name":       secretName,
	}

	path := "/v1/containers/" + containerUUID + "/secrets"
	s.Logger.Info("Adding secret to container", "path", path, "body", addSecretBody)

	resp, err := s.makeBarbicanRequest("POST", path, &addSecretBody)
	if err != nil {
		s.Logger.Error("Error adding secret to container", "error", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		s.Logger.Error("Error adding secret to container", "resp", string(bodyBytes))
		return &HttpError{Status: resp.StatusCode, Message: "Error adding secret to container"}
	}

	return nil
}

// GetSecret retrieves a secret from a specified container in Barbican, using the cache to find the secret reference.
func (s *AppService) GetSecret(ctx context.Context, container string, key string) (string, *HttpError) {
	// Get the secret ref from the cache
	secretRefUUID, err := s.KV.HGet(ctx, "container:"+container, key).Result()
	if err == redis.Nil {
		s.Logger.Warn("Secret not found in cache", "container", container, "key", key)
		return "", &HttpError{Status: http.StatusNotFound, Message: "Secret not found"}
	}
	if err != nil {
		s.Logger.Error("Error getting secret from cache", "error", err)
		return "", &HttpError{Status: http.StatusInternalServerError, Message: "Internal server error"}
	}

	// Get the secret payload from Barbican
	path := "/v1/secrets/" + secretRefUUID + "/payload"
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

// CreateContainer creates a new container in Barbican and caches its metadata.
func (s *AppService) CreateContainer(ctx context.Context, containerName string) *HttpError {
	createContainerBody := map[string]any{
		"name": containerName,
		"type": "generic",
	}

	resp, err := s.makeBarbicanRequest("POST", "/v1/containers", &createContainerBody)
	if err != nil {
		s.Logger.Error("Error creating container in Barbican", "error", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		s.Logger.Error("Error creating container in Barbican", "resp", string(bodyBytes))
		return &HttpError{Status: resp.StatusCode, Message: "Error creating container in Barbican"}
	}

	var containerResponse ContainerMetadata
	if decodeErr := json.NewDecoder(resp.Body).Decode(&containerResponse); decodeErr != nil {
		s.Logger.Error("Error decoding Barbican response", "error", decodeErr)
		return &HttpError{Status: http.StatusInternalServerError, Message: "Internal server error"}
	}

	// Cache the new container
	if err := s.cacheContainer(containerResponse); err != nil {
		s.Logger.Error("Failed to cache new container", "container", containerName, "error", err)
		return &HttpError{Status: http.StatusInternalServerError, Message: "Internal server error"}
	}

	return nil
}

// DeleteSecret deletes a secret from a specified container in Barbican and updates the cache.
func (s *AppService) DeleteSecret(container string, secretName string) *HttpError {
	containerUUID, ok := s.InMemoryCache.Get("container:" + container)
	if !ok || containerUUID == nil {
		s.Logger.Warn("Container not found in cache", "container", container)
		return &HttpError{Status: http.StatusNotFound, Message: "Container not found"}
	}

	// Get the secret ref from the cache
	secretRefUUID, err := s.KV.HGet(context.Background(), "container:"+container, secretName).Result()
	if err == redis.Nil {
		s.Logger.Warn("Secret not found in cache", "container", container, "key", secretName)
		return &HttpError{Status: http.StatusNotFound, Message: "Secret not found"}
	}
	if err != nil {
		s.Logger.Error("Error getting secret from cache", "error", err)
		return &HttpError{Status: http.StatusInternalServerError, Message: "Internal server error"}
	}

	// Delete the secret from Barbican
	path := "/v1/secrets/" + secretRefUUID
	resp, httpErr := s.makeBarbicanRequest("DELETE", path, nil)
	if httpErr != nil {
		s.Logger.Error("Error deleting secret from Barbican", "error", httpErr)
		return httpErr
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(resp.Body)
		s.Logger.Error("Error deleting secret from Barbican", "resp", string(bodyBytes))
		return &HttpError{Status: resp.StatusCode, Message: "Error deleting secret from Barbican"}
	}

	// Remove the secret from the cache
	if err := s.KV.HDel(context.Background(), "container:"+container, secretName).Err(); err != nil {
		s.Logger.Error("Error deleting secret from cache", "error", err)
		return &HttpError{Status: http.StatusInternalServerError, Message: "Internal server error"}
	}

	return nil
}
