package client_pool

import (
	"context"
	"github.com/pkg/errors"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
)

type Client struct {
	*ethclient.Client
	lastErr     error
	availableAt time.Time
	mu          sync.Mutex
	rpcClient   *rpc.Client
	endpoint    string
}

// NewClient initialize new http or universal client based on the given parameters
func NewClient(endpoint, proxyURL string) (*Client, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, errors.Wrap(err, "unable to parse endpoint to url")
	}
	switch u.Scheme {
	case "http", "https":
		return NewHTTPClient(endpoint, proxyURL)
	default:
		return NewUniversalClient(endpoint)
	}
}

func NewUniversalClient(endpoint string) (*Client, error) {
	client, err := ethclient.Dial(endpoint)
	if err != nil {
		return nil, errors.Wrap(err, "unable to dial endpoint with eth client")
	}
	return &Client{
		Client:      client,
		lastErr:     nil,
		availableAt: time.Now(),
		endpoint:    endpoint,
	}, nil
}

func NewHTTPClient(endpoint string, proxyURL string) (*Client, error) {
	httpClient := &http.Client{}
	if proxyURL != "" {
		proxyUrl, err := url.Parse(proxyURL)
		if err != nil {
			return nil, errors.Wrap(err, "unable to parse proxyURL to url")
		}
		httpClient.Transport = &http.Transport{Proxy: http.ProxyURL(proxyUrl)}
	}
	client, err := rpc.DialOptions(context.Background(), endpoint, rpc.WithHTTPClient(httpClient))
	if err != nil {
		return nil, errors.Wrap(err, "unable to dial endpoint with rpc")
	}

	return &Client{
		Client:      ethclient.NewClient(client),
		lastErr:     nil,
		availableAt: time.Now(),
		rpcClient:   client,
		endpoint:    endpoint,
	}, nil
}

// IsAvailable let you know that the client is available for use or not
func (c *Client) IsAvailable() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.lastErr == nil || time.Now().After(c.availableAt) {
		c.lastErr = nil
		return true
	}
	return false
}

// MarkError set the lassErr and availableAt to now+1minute
func (c *Client) MarkError(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastErr = err
	// client will be available after 15 seconds
	c.availableAt = time.Now().Add(15 * time.Second)
}

func (c *Client) GetRPCClient() *rpc.Client {
	return c.rpcClient
}

func (c *Client) EndpointURL() string {
	return c.endpoint
}
