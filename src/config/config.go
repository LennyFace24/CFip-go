package config

import (
	"fmt"
	"os"
	"errors"
	"io/fs"
	"net"
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

	// PrimarySize 主选节点数，承载实际流量
	PrimarySize int `yaml:"primary_size"`
	// BackupSize 备用节点数，主选出现空缺时递补
	BackupSize int `yaml:"backup_size"`
	// Cooldown 节点被淘汰后的冷却时长（秒），冷却期内不参与补位
	Cooldown int `yaml:"cooldown"`

	// HealthInterval 健康检查周期（秒）
	HealthInterval int `yaml:"health_interval"`
	// PingTimes 每次健康检查对每个节点的采样次数
	PingTimes int `yaml:"ping_times"`
	// PingGap 同一次健康检查内相邻采样的间隔（毫秒）
	PingGap int `yaml:"ping_gap"`
	// LossLimit 丢包率上限，取值 0~1；为 0 表示不做该项判定
	LossLimit float64 `yaml:"tlr"`

	// ProxyListen 本地 SOCKS5 监听地址
	ProxyListen string `yaml:"proxy_listen"`
}

func DefaultConfig() *Config {
	return &Config{
		Latency:     500,
		Concurrency: 16,
		Timeout:     500,
		Number:      20,
		Colo:        "",

		PrimarySize: 10,
		BackupSize:  5,
		Cooldown:    300,

		HealthInterval: 60,
		PingTimes:      3,
		PingGap:        200,
		LossLimit:      0.1,

		ProxyListen: "127.0.0.1:1234",
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
		return errors.New("达标 IP 数量必须大于 0")
	}
	if c.PrimarySize <= 0 {
		return errors.New("主选节点数必须大于 0")
	}
	if c.BackupSize < 0 {
		return errors.New("备用节点数不能为负")
	}
	if c.Cooldown < 0 {
		return errors.New("冷却时长不能为负")
	}
	if c.HealthInterval <= 0 {
		return errors.New("健康检查周期必须大于 0")
	}
	if c.PingTimes <= 0 {
		return errors.New("采样次数必须大于 0")
	}
	if c.PingGap < 0 {
		return errors.New("采样间隔不能为负")
	}
	if c.LossLimit <= 0 || c.LossLimit > 1 {
		return errors.New("丢包率上限必须大于 0 且不超过 1")
	}
	if _, _, err := net.SplitHostPort(c.ProxyListen); err != nil {
		return errors.New("监听地址格式不正确，应形如 127.0.0.1:1234")
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
	// 数值项为 0 一律回落到默认：既兼容旧版配置文件缺字段，也避免无效取值
	if config1.PrimarySize == 0 { config1.PrimarySize = fallback.PrimarySize }
	if config1.BackupSize == 0 { config1.BackupSize = fallback.BackupSize }
	if config1.Cooldown == 0 { config1.Cooldown = fallback.Cooldown }
	if config1.HealthInterval == 0 { config1.HealthInterval = fallback.HealthInterval }
	if config1.PingTimes == 0 { config1.PingTimes = fallback.PingTimes }
	if config1.PingGap == 0 { config1.PingGap = fallback.PingGap }
	if config1.LossLimit == 0 { config1.LossLimit = fallback.LossLimit }
	if config1.ProxyListen == "" { config1.ProxyListen = fallback.ProxyListen }
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