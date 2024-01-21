package config

import (
	"os"

	"github.com/eli-yip/rss-zero/pkg/file"
	"github.com/joho/godotenv"
)

type Config struct {
	MinioConfig file.MinioConfig

	OpenAIApiKey  string
	OpenAIBaseURL string // e.g.: https://one-api.example.com/v1

	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string

	RedisAddr     string
	RedisPassword string
	RedisDB       int
}

var C Config

func InitConfigFromFile() {
	loadEnv()
	readEnv()
}

func InitConfigFromEnv() {
	readEnv()
}

func loadEnv() {
	err := godotenv.Load()
	if err != nil {
		panic("Error loading .env file")
	}
}

func readEnv() {
	C.MinioConfig = file.MinioConfig{
		Endpoint:        getEnv("MINIO_ENDPOINT"),
		AccessKeyID:     getEnv("MINIO_ACCESS_KEY_ID"),
		SecretAccessKey: getEnv("MINIO_SECRET_ACCESS_KEY"),
		UseSSL:          true,
		BucketName:      getEnv("MINIO_BUCKET_NAME"),
		AssetsPrefix:    getEnv("MINIO_ASSETS_PREFIX"),
	}

	C.OpenAIApiKey = getEnv("OPENAI_API_KEY")
	C.OpenAIBaseURL = getEnv("OPENAI_BASE_URL")

	C.DBHost = getEnv("DB_HOST")
	C.DBPort = getEnv("DB_PORT")
	C.DBUser = getEnv("DB_USER")
	C.DBPassword = getEnv("DB_PASSWORD")
	C.DBName = getEnv("DB_NAME")

	C.RedisAddr = getEnv("REDIS_ADDR")
	C.RedisPassword = getEnv("REDIS_PASSWORD")
	C.RedisDB = 0
}

func getEnv(key string) string {

	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	panic("Environment variable " + key + " not found")
}
