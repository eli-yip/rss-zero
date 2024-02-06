package config

import (
	"os"

	"github.com/eli-yip/rss-zero/internal/db"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/pkg/file"
	"github.com/joho/godotenv"
)

type Config struct {
	Minio file.MinioConfig

	OpenAIApiKey  string
	OpenAIBaseURL string // e.g.: https://one-api.example.com/v1

	DB db.PostgresConfig

	Redis redis.RedisConfig

	BarkURL string

	ZsxqTestURL string

	ZhihuEncryptionURL string

	ServerURL string

	InternalServerURL string
}

var C Config

func InitFromFile() {
	loadEnv()
	readEnv()
}

// InitFromEnv reads environment variables and initializes the config.
//
// it will panic if any environment variable is not found
func InitFromEnv() { readEnv() }

func loadEnv() {
	err := godotenv.Load()
	if err != nil {
		panic("Error loading .env file")
	}
}

func readEnv() {
	C.Minio = file.MinioConfig{
		Endpoint:        getEnv("MINIO_ENDPOINT"),
		AccessKeyID:     getEnv("MINIO_ACCESS_KEY_ID"),
		SecretAccessKey: getEnv("MINIO_SECRET_ACCESS_KEY"),
		UseSSL:          true,
		BucketName:      getEnv("MINIO_BUCKET_NAME"),
		AssetsPrefix:    getEnv("MINIO_ASSETS_PREFIX"),
	}

	C.OpenAIApiKey = getEnv("OPENAI_API_KEY")
	C.OpenAIBaseURL = getEnv("OPENAI_BASE_URL")

	C.DB.Host = getEnv("DB_HOST")
	C.DB.Port = getEnv("DB_PORT")
	C.DB.User = getEnv("DB_USER")
	C.DB.Password = getEnv("DB_PASSWORD")
	C.DB.Name = getEnv("DB_NAME")

	C.Redis.Addr = getEnv("REDIS_ADDR")
	C.Redis.Password = getEnv("REDIS_PASSWORD")
	C.Redis.DB = 0

	C.BarkURL = getEnv("BARK_URL")

	C.ZsxqTestURL = getEnv("ZSXQ_TEST_URL")

	C.ZhihuEncryptionURL = getEnv("ZHIHU_ENCRYPTION_URL")

	C.ServerURL = getEnv("SERVER_URL")

	C.InternalServerURL = getEnv("INTERNAL_SERVER_URL")
}

func getEnv(key string) string {

	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	panic("Environment variable " + key + " not found")
}
