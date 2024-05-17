package config

import (
	"fmt"
	"os"

	"github.com/eli-yip/rss-zero/internal/db"
	"github.com/eli-yip/rss-zero/internal/file"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/pelletier/go-toml/v2"
)

type TomlConfig struct {
	Settings struct {
		ServerURL         string `toml:"server_url"`
		InternalServerURL string `toml:"internal_server_url"`
		FreshRssURL       string `toml:"fresh_rss_url"`
		Debug             bool   `toml:"debug"`
	} `toml:"settings"`
	Minio struct {
		Endpoint        string `toml:"endpoint"`
		AccessKeyID     string `toml:"access_key_id"`
		SecretAccessKey string `toml:"secret_access_key"`
		Bucket          string `toml:"bucket"`
		AssetsPrefix    string `toml:"assets_prefix"`
	} `toml:"minio"`
	Openai struct {
		APIKey  string `toml:"api_key"`
		BaseURL string `toml:"base_url"`
	} `toml:"openai"`
	Database struct {
		Host     string `toml:"host"`
		Port     string `toml:"port"`
		User     string `toml:"user"`
		Password string `toml:"password"`
		Name     string `toml:"name"`
	} `toml:"database"`
	Redis struct {
		Address  string `toml:"address"`
		Password string `toml:"password"`
	} `toml:"redis"`
	Bark struct {
		URL string `toml:"url"`
	} `toml:"bark"`
	TestURL struct {
		Zsxq    string `toml:"zsxq"`
		Zhihu   string `toml:"zhihu"`
		Xiaobot string `toml:"xiaobot"`
	} `toml:"test_url"`
	Utils struct {
		ZhihuEncryptionURL string `toml:"zhihu_encryption_url"`
		RsshubURL          string `toml:"rsshub_url"`
	} `toml:"utils"`
}

func InitFromFile(path string) (err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	var c TomlConfig
	if err = toml.Unmarshal(data, &c); err != nil {
		return fmt.Errorf("failed to unmarshal toml: %w", err)
	}

	C = Config{
		Minio: file.MinioConfig{
			Endpoint:        c.Minio.Endpoint,
			AccessKeyID:     c.Minio.AccessKeyID,
			SecretAccessKey: c.Minio.SecretAccessKey,
			UseSSL:          true,
			BucketName:      c.Minio.Bucket,
			AssetsPrefix:    c.Minio.AssetsPrefix,
		},

		OpenAIApiKey:  c.Openai.APIKey,
		OpenAIBaseURL: c.Openai.BaseURL,

		DB: db.PostgresConfig{
			Host:     c.Database.Host,
			Port:     c.Database.Port,
			User:     c.Database.User,
			Password: c.Database.Password,
			Name:     c.Database.Name,
		},

		Redis: redis.RedisConfig{
			Addr:     c.Redis.Address,
			Password: c.Redis.Password,
			DB:       0,
		},

		BarkURL: c.Bark.URL,

		ZsxqTestURL:    c.TestURL.Zsxq,
		ZhihuTestURL:   c.TestURL.Zhihu,
		XiaobotTestURL: c.TestURL.Xiaobot,

		ZhihuEncryptionURL: c.Utils.ZhihuEncryptionURL,

		ServerURL:         c.Settings.ServerURL,
		InternalServerURL: c.Settings.InternalServerURL,
		RSSHubURL:         c.Utils.RsshubURL,
		FreshRSSURL:       c.Settings.FreshRssURL,
		Debug:             c.Settings.Debug,
	}

	return nil
}
