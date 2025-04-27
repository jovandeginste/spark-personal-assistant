package ai

import (
	"fmt"
)

type Client struct {
	apiKey string
	model  string
}

type AIConfig struct {
	Type   string `mapstructure:"type"`
	APIKey string `mapstructure:"api_key"`
	Model  string `mapstructure:"model"`
}

func NewClient(cc *AIConfig) (*Client, error) {
	if cc.Type != "gemini" {
		return nil, fmt.Errorf("unknown type: %s", cc.Type)
	}

	c := &Client{
		apiKey: cc.APIKey,
		model:  cc.Model,
	}

	return c, nil
}
