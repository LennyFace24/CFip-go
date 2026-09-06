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
	Latency     int `yaml:"latency"`
	Concurrency int `yaml:"concurrency"`
	Timeout     int `yaml:"timeout"`
	Number      int `yaml:"number"`
}

func DefaultConfig() *Config {
	return &Config{
		Latency:     500,
		Concurrency: 16,
		Timeout:     500,
		Number:      20,
	}

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
	dir, err := os.UserConfigDir()          // /root/.config（可能不存在）
	if err != nil {
		fmt.Println("获取用户配置目录出错:", err)
		return nil,err
	}
	appDir := filepath.Join(dir, "cfip-go")
	err = os.MkdirAll(appDir, 0755)
	if err != nil{
		fmt.Println("创建目录出错:", err)
	}
	file, err := os.ReadFile(appDir + "/config.yaml")

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
	yaml.Unmarshal(file, &config1)
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
	dir, err := os.UserConfigDir()
	if err != nil{
		fmt.Println("获取用户配置目录出错:", err)
		return err
	}
	appDir := filepath.Join(dir, "cfip-go")
	err = os.WriteFile(appDir+"/config.yaml", file, 0644)
	if err != nil{
		fmt.Println("写入配置文件出错:", err)
		return err
	}
	cfg = config
	return nil
}