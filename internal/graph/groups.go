package graph

import (
	"context"
	"fmt"

	abstractions "github.com/microsoft/kiota-abstractions-go"
	msgraphgroups "github.com/microsoftgraph/msgraph-sdk-go/groups"
	msgraphmodels "github.com/microsoftgraph/msgraph-sdk-go/models"
)

func (c *Client) AddGroupMember(ctx context.Context, groupID string, user DirectoryUser) error {
	requestBody := msgraphmodels.NewReferenceCreate()
	odataID := "https://graph.microsoft.com/v1.0/directoryObjects/" + user.ObjectID
	requestBody.SetOdataId(&odataID)

	isMember, err := c.isGroupMember(ctx, groupID, user)
	if err != nil {
		return fmt.Errorf("unable to check current group membership: %w", err)
	}

	if isMember {
		return nil
	}

	err = c.graph.Groups().ByGroupId(groupID).Members().Ref().Post(ctx, requestBody, nil)
	if err != nil {
		return fmt.Errorf("add group member: %w", err)
	}
	return nil
}

func (c *Client) RemoveGroupMember(ctx context.Context, groupID string, user DirectoryUser) error {
	requestBody := msgraphmodels.NewReferenceCreate()
	odataID := "https://graph.microsoft.com/v1.0/directoryObjects/" + user.ObjectID
	requestBody.SetOdataId(&odataID)

	isMember, err := c.isGroupMember(ctx, groupID, user)
	if err != nil {
		return fmt.Errorf("unable to check current group membership: %w", err)
	}

	if !isMember {
		return nil
	}

	err = c.graph.Groups().ByGroupId(groupID).Members().ByDirectoryObjectId(user.ObjectID).Ref().Delete(ctx, nil)
	if err != nil {
		return fmt.Errorf("remove group member: %w", err)
	}
	return nil
}

func (c *Client) isGroupMember(ctx context.Context, groupID string, user DirectoryUser) (bool, error) {
	headers := abstractions.NewRequestHeaders()
	headers.Add("ConsistencyLevel", "eventual")

	requestSearch := fmt.Sprintf("\"displayName:%s\"", user.DisplayName)

	parameters := &msgraphgroups.ItemTransitiveMembersRequestBuilderGetQueryParameters{
		Select: []string{"id"},
		Search: &requestSearch,
	}
	configuration := &msgraphgroups.ItemTransitiveMembersRequestBuilderGetRequestConfiguration{
		Headers:         headers,
		QueryParameters: parameters,
	}
	resp, err := c.graph.Groups().ByGroupId(groupID).TransitiveMembers().Get(ctx, configuration)
	if err != nil {
		return false, fmt.Errorf("unable to get existing members: %w", err)
	}
	data := resp.GetValue()
	switch len(data) {
	case 0:
		return false, nil
	case 1:
		return true, nil
	default:
		return false, fmt.Errorf("more than one user returned for object id")
	}
}
