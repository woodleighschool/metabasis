package graph

import (
	"context"
	"errors"
	"fmt"

	azidentity "github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	msgraphsdk "github.com/microsoftgraph/msgraph-sdk-go"
)

var ErrNotConfigured = errors.New("graph: client not configured")

type Client struct {
	graph   *msgraphsdk.GraphServiceClient
	enabled bool
}

func NewClient(ctx context.Context, tenantID, clientID, clientSecret string) (*Client, error) {
	if tenantID == "" || clientID == "" || clientSecret == "" {
		return &Client{enabled: false}, nil
	}
	cred, err := azidentity.NewClientSecretCredential(tenantID, clientID, clientSecret, nil)
	if err != nil {
		return nil, fmt.Errorf("graph credential: %w", err)
	}
	graphClient, err := msgraphsdk.NewGraphServiceClientWithCredentials(cred, nil)
	if err != nil {
		return nil, fmt.Errorf("graph client: %w", err)
	}
	return &Client{graph: graphClient, enabled: true}, nil
}

func (c *Client) Enabled() bool {
	return c.enabled
}
