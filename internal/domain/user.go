package domain

// User is the provider-neutral identity record available to policy expressions.
type User struct {
	Present           bool     `cel:"present"             json:"present"`
	ID                string   `cel:"id"                  json:"id"`
	MailNickname      string   `cel:"mail_nickname"       json:"mail_nickname"`
	UserPrincipalName string   `cel:"user_principal_name" json:"user_principal_name"`
	DisplayName       string   `cel:"display_name"        json:"display_name"`
	Groups            []string `cel:"groups"              json:"groups,omitempty"`
}
