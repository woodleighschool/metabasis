package graph

import (
	"context"
	"fmt"

	msgraphusers "github.com/microsoftgraph/msgraph-sdk-go/users"
)

type DirectoryUser struct {
	ObjectID    string
	UPN         string
	DisplayName string
	Department  string
	Active      bool
}

func (c *Client) FetchUsers(ctx context.Context) ([]DirectoryUser, error) {
	if !c.enabled {
		return nil, ErrNotConfigured
	}
	if c.graph == nil {
		return nil, fmt.Errorf("graph client missing")
	}
	builder := c.graph.Users()
	adapter := c.graph.GetAdapter()
	top := int32(100)
	selectFields := []string{"id", "userPrincipalName", "onPremisesSamAccountName", "displayName", "department", "accountEnabled"}
	var users []DirectoryUser
	for {
		resp, err := builder.Get(ctx, &msgraphusers.UsersRequestBuilderGetRequestConfiguration{
			QueryParameters: &msgraphusers.UsersRequestBuilderGetQueryParameters{
				Top:    &top,
				Select: selectFields,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("list users: %w", err)
		}
		for _, user := range resp.GetValue() {
			if user == nil {
				continue
			}
			active := true
			if enabled := user.GetAccountEnabled(); enabled != nil {
				active = *enabled
			}
			users = append(users, DirectoryUser{
				ObjectID:    deref(user.GetId()),
				UPN:         deref(user.GetUserPrincipalName()),
				DisplayName: deref(user.GetDisplayName()),
				Department:  deref(user.GetDepartment()),
				Active:      active,
			})
		}
		next := resp.GetOdataNextLink()
		if next == nil || len(*next) == 0 {
			break
		}
		builder = msgraphusers.NewUsersRequestBuilder(*next, adapter)
	}
	return users, nil
}
