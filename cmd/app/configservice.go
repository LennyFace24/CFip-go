package main

import (
	"github.com/LennyFace24/CFip-go/src/config"
)

// ConfigDTO 是给前端用的配置视图，字段与 config.Config 一一对应。
type ConfigDTO struct {
	Latency     int
	Concurrency int
	Timeout     int
	Number      int
	Path        string
}

// ConfigService 向前端暴露配置的读取与保存能力。
type ConfigService struct{}

func (c *ConfigService) ServiceName() string { return "ConfigService" }

// Get 读取当前配置（附带配置文件所在路径，便于界面展示）。
// 读取失败时仍然返回默认配置，让界面可用。
func (c *ConfigService) Get() (ConfigDTO, error) {
	cfg, err := config.LoadConfig()
	path, _ := config.ConfigPath()
	return toDTO(cfg, path), err
}

// Save 校验并写入配置。任一项不合法时返回错误，不落盘。
func (c *ConfigService) Save(latency, concurrency, timeout, number int) error {
	cfg := &config.Config{
		Latency:     latency,
		Concurrency: concurrency,
		Timeout:     timeout,
		Number:      number,
	}
	if err := cfg.Validate(); err != nil {
		return err
	}
	return config.UpdateConfigAndSave(cfg)
}

// Defaults 返回内置默认配置，供「恢复默认」使用。
func (c *ConfigService) Defaults() ConfigDTO {
	return toDTO(config.DefaultConfig(), "")
}

func toDTO(cfg *config.Config, path string) ConfigDTO {
	if cfg == nil {
		cfg = config.DefaultConfig()
	}
	return ConfigDTO{
		Latency:     cfg.Latency,
		Concurrency: cfg.Concurrency,
		Timeout:     cfg.Timeout,
		Number:      cfg.Number,
		Path:        path,
	}
}
