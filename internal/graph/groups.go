package graph

import (
	"context"
	"fmt"
	"slices"

	msgraphmodels "github.com/microsoftgraph/msgraph-sdk-go/models"
)

func (c *Client) AddGroupMember(ctx context.Context, groupID string, userID string) error {
	requestBody := msgraphmodels.NewReferenceCreate()
	odataID := "https://graph.microsoft.com/v1.0/directoryObjects/" + userID
	requestBody.SetOdataId(&odataID)

	existingMembers, err := c.getGroupMembers(ctx, groupID)
	if err != nil {
		return fmt.Errorf("unable to get existing members: %w", err)
	}

	if slices.Contains(existingMembers, userID) {
		return nil
	}

	err = c.graph.Groups().ByGroupId(groupID).Members().Ref().Post(ctx, requestBody, nil)
	if err != nil {
		return fmt.Errorf("add group member: %w", err)
	}
	return nil
}

func (c *Client) RemoveGroupMember(ctx context.Context, groupID string, userID string) error {
	requestBody := msgraphmodels.NewReferenceCreate()
	odataID := "https://graph.microsoft.com/v1.0/directoryObjects/" + userID
	requestBody.SetOdataId(&odataID)

	existingMembers, err := c.getGroupMembers(ctx, groupID)
	if err != nil {
		return fmt.Errorf("unable to get existing members: %w", err)
	}

	if !slices.Contains(existingMembers, userID) {
		return nil
	}

	err = c.graph.Groups().ByGroupId(groupID).Members().ByDirectoryObjectId(userID).Ref().Delete(ctx, nil)
	if err != nil {
		return fmt.Errorf("remove group member: %w", err)
	}
	return nil
}

func (c *Client) getGroupMembers(ctx context.Context, groupID string) ([]string, error) {
	var members []string
	resp, err := c.graph.Groups().ByGroupId(groupID).TransitiveMembers().Get(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to get existing members: %w", err)
	}
	for _, member := range resp.GetValue() {
		members = append(members, deref(member.GetId()))
	}
	return members, nil
}
