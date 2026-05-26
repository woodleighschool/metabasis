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
	graph       *msgraphsdk.GraphServiceClient
	enabled     bool
	GroupConfig GroupConfig
}

type GroupConfig struct {
	AwayGroups     []string
	HomeGroups     []string
	EnableMFAGroup string
	ForceMFAGroup  string
}

func NewClient(ctx context.Context, tenantID, clientID, clientSecret string, awayGroups []string, homeGroups []string, enableMFAGroup string, forceMFAGroup string) (*Client, error) {
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
	groupConfig := GroupConfig{
		AwayGroups:     awayGroups,
		HomeGroups:     homeGroups,
		EnableMFAGroup: enableMFAGroup,
		ForceMFAGroup:  forceMFAGroup,
	}
	return &Client{graph: graphClient, enabled: true, GroupConfig: groupConfig}, nil
}

func (c *Client) Enabled() bool {
	return c.enabled
}
