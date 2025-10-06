package main

import (
	"context"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

// InitKV initializes and returns a Redis client using environment variables for configuration.
func InitKV() *redis.Client {
	envErr := godotenv.Load()
	if envErr != nil {
		log.Printf("Error loading .env file: %v", envErr)
	}

	ctx := context.Background()
	kv := redis.NewClient(&redis.Options{
		Addr:     os.Getenv("KV_URL"),
		Password: "",
		DB:       0,
	})

	_, err := kv.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Error connecting to Redis: %v", err)
	}

	return kv
}

func GetSecretFromCache(kv *redis.Client, ctx context.Context, secretName string) (string, error) {
	secret, err := kv.HGet(ctx, "secrets", secretName).Result()
	return secret, err
}

func GetSecretUUIDFromCache(kv *redis.Client, ctx context.Context, secretName string) (string, error) {
	uuid, err := kv.HGet(ctx, "uuids", secretName).Result()
	return uuid, err
}
