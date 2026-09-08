package config

import (
	"fmt"
	"os"
	"errors"
	"io/fs"
	"path/filepath"

	yaml "gopkg.in/yaml.v2"

)

var cfg *Config

type Config struct {
	// Latency 是延迟上限（毫秒）：探测成功但延迟超过它的 IP 视为「超标」，
	// 不计入优选结果，也不触发提前停止。
	Latency     int `yaml:"latency"`
	Concurrency int `yaml:"concurrency"`
	Timeout     int `yaml:"timeout"`
	// Number 需要凑够的「达标」IP 数量，集满即停止测速
	Number int `yaml:"number"`
	// Colo 机房白名单，空格分隔的 IATA 代码（如 "HKG NRT SIN"）；留空表示不过滤
	Colo string `yaml:"colo"`
}

func DefaultConfig() *Config {
	return &Config{
		Latency:     500,
		Concurrency: 16,
		Timeout:     500,
		Number:      20,
		Colo:        "",
	}

}
// ConfigDir 返回应用配置目录，目录不存在时自动创建
func ConfigDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	appDir := filepath.Join(dir, "cfip-go")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		return "", err
	}
	return appDir, nil
}

// ConfigPath 返回配置文件完整路径
func ConfigPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

// Validate 校验配置取值是否合法，GUI 保存前调用
func (c *Config) Validate() error {
	if c == nil {
		return errors.New("配置为空")
	}
	if c.Latency <= 0 {
		return errors.New("允许最大延迟必须大于 0")
	}
	if c.Concurrency <= 0 {
		return errors.New("并发数必须大于 0")
	}
	if c.Timeout <= 0 {
		return errors.New("请求超时时间必须大于 0")
	}
	if c.Number <= 0 {
		return errors.New("优选 IP 最大数必须大于 0")
	}
	return nil
}

// LoadConfig 加载配置文件，启动时加载一次，后续在修改配置文件时也可以调用
func LoadConfig() (*Config,error) {
	if cfg == nil {
		c,err := LoadConfigFromPath()
		if err != nil {
			cfg = DefaultConfig()
			return cfg,err
		}
		cfg = c
	}
	return cfg,nil
}

func LoadConfigFromPath() (*Config,error) {
	path, err := ConfigPath()
	if err != nil {
		fmt.Println("获取配置文件路径出错:", err)
		return nil,err
	}
	file, err := os.ReadFile(path)

	if err != nil {
    	if !errors.Is(err, fs.ErrNotExist) {
        	return nil, err   // 真错误，交给上层提示用户
    	}
    	// 确认是"不存在" → 才走创建默认配置
    	cfg = DefaultConfig()
    	if err := UpdateConfigAndSave(cfg); err != nil {
        	return nil, err   // 连创建都失败（比如没写权限），也要报
    	}
		return cfg,nil
	}
	var config1 Config
	if err := yaml.Unmarshal(file, &config1); err != nil {
		fmt.Println("解析配置文件出错:", err)
		return nil,err
	}
	// 兼容空文件或缺字段的配置：0 值回落到默认
	fallback := DefaultConfig()
	if config1.Latency == 0 { config1.Latency = fallback.Latency }
	if config1.Concurrency == 0 { config1.Concurrency = fallback.Concurrency }
	if config1.Timeout == 0 { config1.Timeout = fallback.Timeout }
	if config1.Number == 0 { config1.Number = fallback.Number }
	cfg = &config1
	return cfg,nil
}

// 修改配置文件并保存
func UpdateConfigAndSave(config *Config) error {
	file, err := yaml.Marshal(config)
	if err != nil {
		fmt.Println("序列化配置文件出错:", err)
		return err
	}
	path, err := ConfigPath()
	if err != nil {
		fmt.Println("获取配置文件路径出错:", err)
		return err
	}
	if err := os.WriteFile(path, file, 0644); err != nil {
		fmt.Println("写入配置文件出错:", err)
		return err
	}
	cfg = config
	return nil
}