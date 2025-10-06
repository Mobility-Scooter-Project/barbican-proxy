package main

import (
	"log"
	"log/slog"
	"net/http"
	"os"

	"github.com/redis/go-redis/v9"
	"istio.io/pkg/cache"
	"github.com/lmittmann/tint"
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
	logger := slog.New(tint.NewHandler(os.Stderr, nil))

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

	http.HandleFunc(("/api/v1/secrets/{container}/{name}"), func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			service.GetSecretHandler(w, r)
		case http.MethodDelete:
			service.DeleteSecretHandler(w, r)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
			w.Write([]byte("Method Not Allowed"))
		}
	})
	http.HandleFunc(("/api/v1/secrets"), service.UploadSecretHandler)
	http.HandleFunc(("/api/v1/containers"), service.CreateContainerHandler)

	logger.Info("Starting server on :3000")
	log.Fatal(http.ListenAndServe(":3000", nil))
}
