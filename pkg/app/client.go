package app

import (
	"context"

	"google.golang.org/genai"
)

type Client struct {
	APIKey string

	client *genai.Client
	ctx    context.Context
}

func (c *Client) Init(apiKey string) error {
	c.ctx = context.Background()

	client, err := genai.NewClient(c.ctx, &genai.ClientConfig{APIKey: apiKey})
	if err != nil {
		return err
	}

	c.client = client

	return nil
}

func NewClient(apiKey string) (*Client, error) {
	c := &Client{}

	if err := c.Init(apiKey); err != nil {
		return nil, err
	}

	return c, nil
}
