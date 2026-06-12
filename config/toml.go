package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pelletier/go-toml/v2"
)

const DefaultFetchCount = 20

var C TomlConfig

type TomlConfig struct {
	Settings struct {
		ServerURL         string `toml:"server_url"`
		InternalServerURL string `toml:"internal_server_url"`
		FreshRssURL       string `toml:"fresh_rss_url"`
		Username          string `toml:"username"`
		Password          string `toml:"password"`
		Debug             bool   `toml:"debug"`
		DisableZhihu      bool   `toml:"disable_zhihu"`
	} `toml:"settings"`
	Minio    MinioConfig    `toml:"minio"`
	Openai   OpenAIConfig   `toml:"openai"`
	Database DatabaseConfig `toml:"database"`
	Redis    RedisConfig    `toml:"redis"`
	Bark     struct {
		URL string `toml:"url"`
	} `toml:"bark"`
	TestURL struct {
		Zsxq    string `toml:"zsxq"`
		Xiaobot string `toml:"xiaobot"`
	} `toml:"test_url"`
	Zlive struct {
		ServerUrl string `toml:"server_url"`
		Username  string `toml:"username"`
		Password  string `toml:"password"`
	} `toml:"zlive"`
	LanguageDetection struct {
		Server string `toml:"server"`
	} `toml:"language_detection"`
	Utils struct {
		RsshubURL string `toml:"rsshub_url"`
	} `toml:"utils"`
	Zsxq ZsxqConfig `toml:"zsxq"`

	BJT *time.Location
}

// ZsxqConfig holds operational rules for the zsxq router that change by
// business decision rather than code. Absent section -> empty lists (no-op).
type ZsxqConfig struct {
	// BlockedAuthorIDs / BlockedAuthorNames: topics from these authors are
	// skipped during parsing. Matched by OR (either id or name).
	BlockedAuthorIDs   []int    `toml:"blocked_author_ids"`
	BlockedAuthorNames []string `toml:"blocked_author_names"`
}

type OpenAIConfig struct {
	Model   string `toml:"model"`
	APIKey  string `toml:"api_key"`
	BaseURL string `toml:"base_url"`
}

type DatabaseConfig struct {
	Host     string `toml:"host"`
	Port     string `toml:"port"`
	User     string `toml:"user"`
	Password string `toml:"password"`
	Name     string `toml:"name"`
}

type MinioConfig struct {
	Endpoint        string `toml:"endpoint"`
	AccessKeyID     string `toml:"access_key_id"`
	SecretAccessKey string `toml:"secret_access_key"`
	Bucket          string `toml:"bucket"`
	AssetsPrefix    string `toml:"assets_prefix"`
}

type RedisConfig struct {
	Address  string `toml:"address"`
	Password string `toml:"password"`
}

func InitForTestToml() (err error) {
	projectRoot, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("failed to find project root: %w", err)
	}

	configPath := filepath.Join(projectRoot, "config.toml")
	if err = InitFromToml(configPath); err != nil {
		return fmt.Errorf("failed to init config from file: %w", err)
	}

	return nil
}

func findProjectRoot() (path string, err error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current working directory: %w", err)
	}

	for {
		if _, err := os.Stat(dir + "/go.mod"); err == nil {
			return dir, nil
		}

		if dir == "/" {
			return "", fmt.Errorf("failed to find project root")
		}

		dir = filepath.Dir(dir)
	}
}

func InitFromToml(path string) (err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	if err = toml.Unmarshal(data, &C); err != nil {
		return fmt.Errorf("failed to unmarshal toml: %w", err)
	}

	BJT, err := getBJT()
	if err != nil {
		return fmt.Errorf("failed to get BJT: %w", err)
	}
	C.BJT = BJT

	return nil
}

func getBJT() (*time.Location, error) {
	return time.LoadLocation("Asia/Shanghai")
}
