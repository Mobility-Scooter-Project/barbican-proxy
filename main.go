package main

import (
	"log"
	"log/slog"
	"net/http"
	"os"

	"github.com/redis/go-redis/v9"
	"istio.io/pkg/cache"
)

type AppService struct {
	InMemoryCache cache.ExpiringCache
	KV            *redis.Client
	Logger        *slog.Logger
}

type HttpError struct {
	Status  int
	Message string
}

func main() {
	jsonHandler := slog.NewJSONHandler(os.Stderr, nil)
	logger := slog.New(jsonHandler)

	inMemoryCache := cache.NewLRU(0, 0, 1000) // do not evict items based on time, only size
	kv := InitKV()
	service := &AppService{
		InMemoryCache: inMemoryCache,
		KV:            kv,
		Logger:        logger,
	}

	err := service.LoadContainers()
	if err != nil {
		logger.Error("Error loading containers", "error", err)
		return
	}

	http.HandleFunc(("/health"), func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	http.HandleFunc(("/api/v1/secrets/{container}/{name}"), service.GetSecretHandler)
	http.HandleFunc(("/api/v1/secrets"), service.UploadSecretHandler)

	println("Barbican ES proxy listening on :3000")
	log.Fatal(http.ListenAndServe(":3000", nil))
}
