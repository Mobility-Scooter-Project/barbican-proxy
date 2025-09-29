package main

import (
	"net/http"
	"os"
)

func GetSecretFromBarbicanByName(name string) (string, error) {
	token := GetAuthToken()
	println()

	url := os.Getenv("BARBICAN_URL") + "/v1/secrets"

	req, err := http.NewRequest("GET", url, nil)

	if err != nil {
		return "", err
	}

	req.Header.Set("X-Auth-Token", token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()


	return "", nil
}
