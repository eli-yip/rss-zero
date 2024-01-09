package config

import (
	"os"

	"github.com/eli-yip/zsxq-parser/pkg/file"
)

type Config struct {
	MinioConfig file.MinioConfig
	ApiKey      string
	BaseURL     string // e.g.: https://one-api.example.com/v1
}

var C Config

func InitConfig() {
	C.MinioConfig = file.MinioConfig{
		Endpoint:        os.Getenv("MINIO_ENDPOINT"),
		AccessKeyID:     os.Getenv("MINIO_ACCESS_KEY_ID"),
		SecretAccessKey: os.Getenv("MINIO_SECRET_ACCESS_KEY"),
		UseSSL:          false,
		BucketName:      os.Getenv("MINIO_BUCKET_NAME"),
		AssetsDomain:    os.Getenv("MINIO_ASSETS_DOMAIN"),
	}

	C.ApiKey = os.Getenv("OPENAI_API_KEY")
}
