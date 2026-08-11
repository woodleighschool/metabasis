package graph

import (
	"context"
	"fmt"

	msgraphmodels "github.com/microsoftgraph/msgraph-sdk-go/models"
	msgraphusers "github.com/microsoftgraph/msgraph-sdk-go/users"
)

type DirectoryUser struct {
	ObjectID    string
	UPN         string
	DisplayName string
	Department  string
	Photo       []byte
	Active      bool
}

func (c *Client) FetchUser(ctx context.Context, userID string) (DirectoryUser, error) {
	if !c.enabled {
		return DirectoryUser{}, ErrNotConfigured
	}
	if c.graph == nil {
		return DirectoryUser{}, fmt.Errorf("graph client missing")
	}
	builder := c.graph.Users()
	selectFields := []string{
		"id",
		"userPrincipalName",
		"onPremisesSamAccountName",
		"displayName",
		"department",
		"accountEnabled",
	}
	resp, err := builder.ByUserId(userID).Get(ctx, &msgraphusers.UserItemRequestBuilderGetRequestConfiguration{
		QueryParameters: &msgraphusers.UserItemRequestBuilderGetQueryParameters{
			Select: selectFields,
		},
	})
	if err != nil {
		return DirectoryUser{}, fmt.Errorf("get user object id: %w", err)
	}

	active := true
	if enabled := resp.GetAccountEnabled(); enabled != nil {
		active = *enabled
	}

	user := DirectoryUser{
		ObjectID:    deref(resp.GetId()),
		UPN:         deref(resp.GetUserPrincipalName()),
		DisplayName: deref(resp.GetDisplayName()),
		Department:  deref(resp.GetDepartment()),
		Photo:       nil,
		Active:      active,
	}
	return user, nil
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
	selectFields := []string{
		"id",
		"userPrincipalName",
		"onPremisesSamAccountName",
		"displayName",
		"department",
		"accountEnabled",
		"photo",
	}
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
			photo := c.fetchUserPhoto(ctx, user)

			users = append(users, DirectoryUser{
				ObjectID:    deref(user.GetId()),
				UPN:         deref(user.GetUserPrincipalName()),
				DisplayName: deref(user.GetDisplayName()),
				Department:  deref(user.GetDepartment()),
				Photo:       photo,
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

func (c *Client) fetchUserPhoto(ctx context.Context, user msgraphmodels.Userable) []byte {
	adapter := c.graph.GetAdapter()
	builder := msgraphusers.NewItemPhotosItemValueContentRequestBuilder(
		fmt.Sprintf("https://graph.microsoft.com/v1.0/users/%s/photos/96x96/$value", deref(user.GetId())),
		adapter,
	)

	photoData, err := builder.Get(ctx, &msgraphusers.ItemPhotosItemValueContentRequestBuilderGetRequestConfiguration{})

	if err != nil || photoData == nil {
		return nil
	}
	return photoData
}
