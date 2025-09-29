package main

import (
	"net/http"
	"os"
)

func GetAuthToken() string {
	url := os.Getenv("OS_AUTH_URL") + "/auth/tokens"

	body := map[string]any{
		"auth": map[string]any{
			"identity": map[string]any{
				"methods": []string{"application_credential"},
				"application_credential": map[string]any{
					"id":     os.Getenv("OS_APPLICATION_CREDENTIAL_CLIENT_ID"),
					"secret": os.Getenv("OS_APPLICATION_CREDENTIAL_CLIENT_SECRET"),
				},
			},
		},
	}

	resp, err := http.Post(url, "application/json", MapToBuffer(body))
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	return resp.Header.Get("X-Subject-Token")
}
