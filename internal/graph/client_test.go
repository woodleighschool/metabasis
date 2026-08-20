package graph

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
)

type staticCredential struct{}

func (staticCredential) GetToken(context.Context, policy.TokenRequestOptions) (azcore.AccessToken, error) {
	return azcore.AccessToken{Token: "test-token", ExpiresOn: time.Now().Add(time.Hour)}, nil
}

func TestClientResolvesAliasesAndMutatesGroupMembership(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	var mutations []string
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if got, want := request.Header.Get("Authorization"), "Bearer test-token"; got != want {
			t.Errorf("Authorization = %q, want %q", got, want)
		}
		switch request.Method + " " + request.URL.Path {
		case "GET /v1.0/users/student@example.com":
			_ = json.NewEncoder(response).Encode(map[string]string{
				"id":                "user-id",
				"userPrincipalName": "student@example.com",
				"mailNickname":      "student",
				"displayName":       "Example Student",
			})
		case "POST /v1.0/users/user-id/checkMemberGroups":
			var body struct {
				GroupIDs []string `json:"groupIds"`
			}
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Errorf("decode checkMemberGroups body: %v", err)
			}
			if got, want := body.GroupIDs, []string{"managed-access", "staff-group", "student-group"}; !slices.Equal(got, want) {
				t.Errorf("groupIds = %v, want %v", got, want)
			}
			_ = json.NewEncoder(response).Encode(map[string][]string{"value": {"student-group", "managed-access"}})
		case "POST /v1.0/groups/managed-access/members/$ref":
			mu.Lock()
			mutations = append(mutations, "add")
			mu.Unlock()
			response.WriteHeader(http.StatusNoContent)
		case "DELETE /v1.0/groups/managed-access/members/user-id/$ref":
			mu.Lock()
			mutations = append(mutations, "remove")
			mu.Unlock()
			response.WriteHeader(http.StatusNoContent)
		default:
			http.Error(response, "unexpected route", http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	client := &Client{baseURL: server.URL + "/v1.0", httpClient: server.Client(), credential: staticCredential{}}

	snapshot, err := client.Resolve(
		t.Context(),
		"student@example.com",
		map[string][]string{"students": {"student-group"}, "staff": {"staff-group"}},
		map[string]string{"overseas_access": "managed-access"},
	)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got, want := snapshot.User.Groups, []string{"students"}; !slices.Equal(got, want) {
		t.Errorf("User.Groups = %v, want %v", got, want)
	}
	if got, want := snapshot.ManagedGroups, []string{"overseas_access"}; !slices.Equal(got, want) {
		t.Errorf("ManagedGroups = %v, want %v", got, want)
	}
	if err := client.AddGroupMember(t.Context(), "managed-access", "user-id"); err != nil {
		t.Fatalf("AddGroupMember() error = %v", err)
	}
	if err := client.RemoveGroupMember(t.Context(), "managed-access", "user-id"); err != nil {
		t.Fatalf("RemoveGroupMember() error = %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if got, want := mutations, []string{"add", "remove"}; !slices.Equal(got, want) {
		t.Errorf("mutations = %v, want %v", got, want)
	}
}
