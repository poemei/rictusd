package stn

import "rictusd/internal/config"

// No-op STN client for v0.3.x
type Client struct{}

func New(_ *config.Config) *Client { return &Client{} }
func (c *Client) Enabled() bool    { return false }
func (c *Client) PushAsync(_ any, _ any) {}
