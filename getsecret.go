package main

import (
	"log"
	"net/http"

	"github.com/redis/go-redis/v9"
)

/*
*
1. Attempt to retrieve the secret from our local cache
2. If not found, try and find its uuid from our local cache
3. If not found, query barbican for the uuid
4. Once the uuid is found, query barbican for the secret if not cached
*/
func GetSecretHandler(res http.ResponseWriter, req *http.Request) {
	secretName := req.PathValue("name")
	ctx := req.Context()
	kv := InitKV()

	secret, err := GetSecretFromCache(kv, ctx, secretName)

	if err == redis.Nil {
		uuid, err := GetSecretUUIDFromCache(kv, ctx, secretName)

		if err == redis.Nil {
			log.Printf("UUID cache miss [%s]", secretName)
			data, err := GetSecretFromBarbicanByName(secretName)
			println("Data: ", data)
			if err != nil {
				log.Printf("Error retrieving secret from Barbican: %v", err)
				http.Error(res, "Internal Server Error", http.StatusInternalServerError)
				return
			}

			return
		}

		println("UUID: ", uuid)
		return
	}

	if err != nil {
		log.Printf("Error retrieving secret from cache: %v", err)
		http.Error(res, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	log.Printf("Secret cache hit [%s]", secretName)
	res.Write([]byte(secret))
}
