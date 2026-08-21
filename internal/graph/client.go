package graph

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"

	"github.com/woodleighschool/metabasis/internal/config"
	"github.com/woodleighschool/metabasis/internal/domain"
)

const (
	defaultBaseURL         = "https://graph.microsoft.com/v1.0"
	checkMemberGroupsLimit = 20
)

var graphScopes = []string{"https://graph.microsoft.com/.default"}

type tokenCredential interface {
	GetToken(context.Context, policy.TokenRequestOptions) (azcore.AccessToken, error)
}

// Client is the narrow Microsoft Graph client used by reconciliation.
type Client struct {
	baseURL    string
	httpClient *http.Client
	credential tokenCredential
}

// NewClient creates an application-authenticated Microsoft Graph client.
func NewClient(connection config.Connection) (*Client, error) {
	credential, err := azidentity.NewClientSecretCredential(
		connection.TenantID,
		connection.ClientID,
		connection.ClientSecret,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("create Microsoft credential: %w", err)
	}
	baseURL := strings.TrimRight(connection.BaseURL, "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &Client{baseURL: baseURL, httpClient: http.DefaultClient, credential: credential}, nil
}

// Resolve fetches one Entra user and its configured group aliases.
func (c *Client) Resolve(
	ctx context.Context,
	subject string,
	groupAliases map[string][]string,
) (domain.User, error) {
	var response struct {
		ID                string `json:"id"`
		UserPrincipalName string `json:"userPrincipalName"`
		MailNickname      string `json:"mailNickname"`
		DisplayName       string `json:"displayName"`
	}
	endpoint := c.baseURL + "/users/" + url.PathEscape(subject)
	query := url.Values{"$select": {"id,userPrincipalName,mailNickname,displayName"}}
	if err := c.request(ctx, http.MethodGet, endpoint+"?"+query.Encode(), nil, &response); err != nil {
		return domain.User{}, fmt.Errorf("resolve Entra user %q: %w", subject, err)
	}
	if response.ID == "" {
		return domain.User{}, fmt.Errorf("resolve Entra user %q: Graph returned no ID", subject)
	}

	groupIDs, aliasesByID := prepareGroupAliases(groupAliases)
	memberships, err := c.checkMemberGroups(ctx, response.ID, groupIDs)
	if err != nil {
		return domain.User{}, err
	}
	groups := make(map[string]struct{})
	for _, groupID := range memberships {
		for _, alias := range aliasesByID[groupID] {
			groups[alias] = struct{}{}
		}
	}
	return domain.User{
		Present:           true,
		ID:                response.ID,
		MailNickname:      response.MailNickname,
		UserPrincipalName: response.UserPrincipalName,
		DisplayName:       response.DisplayName,
		Groups:            sortedSet(groups),
	}, nil
}

// AddGroupMember adds a user to a group.
func (c *Client) AddGroupMember(ctx context.Context, groupID, userID string) error {
	body := map[string]string{"@odata.id": c.baseURL + "/directoryObjects/" + url.PathEscape(userID)}
	endpoint := c.baseURL + "/groups/" + url.PathEscape(groupID) + "/members/$ref"
	if err := c.request(ctx, http.MethodPost, endpoint, body, nil); err != nil {
		return fmt.Errorf("add Entra group member: %w", err)
	}
	return nil
}

// RemoveGroupMember removes a user from a group.
func (c *Client) RemoveGroupMember(ctx context.Context, groupID, userID string) error {
	endpoint := c.baseURL + "/groups/" + url.PathEscape(groupID) + "/members/" + url.PathEscape(userID) + "/$ref"
	if err := c.request(ctx, http.MethodDelete, endpoint, nil, nil); err != nil {
		return fmt.Errorf("remove Entra group member: %w", err)
	}
	return nil
}

func (c *Client) checkMemberGroups(ctx context.Context, userID string, groupIDs []string) ([]string, error) {
	var memberships []string
	for start := 0; start < len(groupIDs); start += checkMemberGroupsLimit {
		end := min(start+checkMemberGroupsLimit, len(groupIDs))
		var response struct {
			Value []string `json:"value"`
		}
		endpoint := c.baseURL + "/users/" + url.PathEscape(userID) + "/checkMemberGroups"
		if err := c.request(ctx, http.MethodPost, endpoint, map[string][]string{"groupIds": groupIDs[start:end]}, &response); err != nil {
			return nil, fmt.Errorf("check Entra group memberships: %w", err)
		}
		memberships = append(memberships, response.Value...)
	}
	sort.Strings(memberships)
	return memberships, nil
}

func (c *Client) request(ctx context.Context, method, endpoint string, body any, response any) error {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode Graph request: %w", err)
		}
		reader = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return fmt.Errorf("create Graph request: %w", err)
	}
	token, err := c.credential.GetToken(ctx, policy.TokenRequestOptions{Scopes: graphScopes})
	if err != nil {
		return fmt.Errorf("authenticate Graph request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+token.Token)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	httpResponse, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("send Graph request: %w", err)
	}
	defer func() { _ = httpResponse.Body.Close() }()
	if httpResponse.StatusCode < 200 || httpResponse.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(httpResponse.Body, 4096))
		return fmt.Errorf("graph returned %s: %s", httpResponse.Status, strings.TrimSpace(string(message)))
	}
	if response == nil || httpResponse.StatusCode == http.StatusNoContent {
		_, _ = io.Copy(io.Discard, httpResponse.Body)
		return nil
	}
	if err := json.NewDecoder(httpResponse.Body).Decode(response); err != nil {
		return fmt.Errorf("decode Graph response: %w", err)
	}
	return nil
}

func prepareGroupAliases(groupAliases map[string][]string) ([]string, map[string][]string) {
	aliasesByID := make(map[string][]string)
	for alias, groupIDs := range groupAliases {
		for _, groupID := range groupIDs {
			aliasesByID[groupID] = append(aliasesByID[groupID], alias)
		}
	}
	groupIDs := make([]string, 0, len(aliasesByID))
	for groupID, aliases := range aliasesByID {
		sort.Strings(aliases)
		aliasesByID[groupID] = slices.Compact(aliases)
		groupIDs = append(groupIDs, groupID)
	}
	sort.Strings(groupIDs)
	return groupIDs, aliasesByID
}

func sortedSet(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
