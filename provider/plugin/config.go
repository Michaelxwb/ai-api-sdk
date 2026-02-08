package plugin

import (
	"errors"
	"strings"
	"time"
)

const (
	defaultRequestTimeout = 120 * time.Second
	defaultDialTimeout    = 10 * time.Second
	defaultReadTimeout    = 5 * time.Minute
	defaultCallbackBuffer = 16
)

// Config 配置结构
type Config struct {
	Endpoint       string            `json:"endpoint" yaml:"endpoint"`
	AuthToken      string            `json:"authToken,omitempty" yaml:"auth_token,omitempty"`
	ConfigID       string            `json:"configId,omitempty" yaml:"config_id,omitempty"`
	Locators       *ElementLocators  `json:"locators,omitempty" yaml:"locators,omitempty"`
	RequestTimeout time.Duration     `json:"requestTimeout,omitempty" yaml:"request_timeout,omitempty"`
	DialTimeout    time.Duration     `json:"dialTimeout,omitempty" yaml:"dial_timeout,omitempty"`
	ReadTimeout    time.Duration     `json:"readTimeout,omitempty" yaml:"read_timeout,omitempty"`
	CallbackBuffer int               `json:"callbackBuffer,omitempty" yaml:"callback_buffer,omitempty"`
	Headers        map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`
}

func (c Config) normalize() Config {
	if c.RequestTimeout <= 0 {
		c.RequestTimeout = defaultRequestTimeout
	}
	if c.DialTimeout <= 0 {
		c.DialTimeout = defaultDialTimeout
	}
	if c.ReadTimeout <= 0 {
		c.ReadTimeout = defaultReadTimeout
	}
	if c.CallbackBuffer <= 0 {
		c.CallbackBuffer = defaultCallbackBuffer
	}
	return c
}

// Validate 校验配置
func (c Config) Validate() error {
	if strings.TrimSpace(c.Endpoint) == "" {
		return errors.New("plugin: endpoint is required")
	}
	return nil
}
