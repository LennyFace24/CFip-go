package config

import (
	"fmt"
	yaml "gopkg.in/yaml.v2"
	"os"
	"sync"
)

var once sync.Once
var cfg *Config

type Config struct {
	Latency     int `yaml:"latency"`
	Concurrency int `yaml:"concurrency"`
	Timeout     int `yaml:"timeout"`
}

func LoadConfig() (*Config, error) {
	once.Do(func() {
		path := "config.yml"
		file, err := os.ReadFile(path)
		if err != nil {
			fmt.Println("加载配置文件出错:", err)
			panic(err)
		}
		yaml.Unmarshal(file, &cfg)
	})
	return cfg, nil
}
