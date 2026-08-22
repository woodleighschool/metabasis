package config

import (
	"time"

	"github.com/woodleighschool/metabasis/internal/expression"
)

// Config is the complete versioned configuration.
type Config struct {
	Version       int                   `yaml:"version"                 jsonschema:"enum=2"`
	Listen        string                `yaml:"listen,omitempty"`
	MetricsListen string                `yaml:"metrics_listen,omitempty"`
	Connections   map[string]Connection `yaml:"connections"             jsonschema:"minProperties=1"`
	Database      Database              `yaml:"database"`
	Webhooks      map[string]Webhook    `yaml:"webhooks"                jsonschema:"minProperties=1"`
	Identity      Identity              `yaml:"identity"`
	Rules         []Rule                `yaml:"rules"                   jsonschema:"minItems=1"`
	Reconcile     Reconcile             `yaml:"reconcile,omitempty"`
	Programs      []expression.Program  `yaml:"-"`
}

// Connection contains credentials for a remote API.
type Connection struct {
	Type         string `yaml:"type"                   jsonschema:"enum=microsoft_graph"`
	TenantID     string `yaml:"tenant_id"`
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"`
	BaseURL      string `yaml:"base_url,omitempty"`
}

// Database configures the PostgreSQL connection pool.
type Database struct {
	URL               string   `yaml:"url"`
	MinConnections    int32    `yaml:"min_connections,omitempty"     jsonschema:"minimum=0"`
	MaxConnections    int32    `yaml:"max_connections,omitempty"     jsonschema:"minimum=1"`
	MaxConnLifetime   Duration `yaml:"max_connection_lifetime,omitempty"`
	MaxConnIdleTime   Duration `yaml:"max_connection_idle_time,omitempty"`
	HealthCheckPeriod Duration `yaml:"health_check_period,omitempty"`
}

// Webhook defines one canonical intent source and its authentication token.
type Webhook struct {
	Path        string `yaml:"path"`
	BearerToken string `yaml:"bearer_token"`
}

// Identity selects the Entra connection and aliases available to policy.
type Identity struct {
	Connection string              `yaml:"connection"`
	Groups     map[string][]string `yaml:"groups" jsonschema:"minProperties=1"`
}

// Rule is an ordered identity policy. The first matching rule applies to the subject.
type Rule struct {
	Name   string `yaml:"name"`
	When   string `yaml:"when"`
	States States `yaml:"states"`
}

// States maps aggregate subject states to membership assertions.
type States struct {
	Pending GroupAssertions `yaml:"pending,omitempty"`
	Active  GroupAssertions `yaml:"active,omitempty"`
	Settled GroupAssertions `yaml:"settled,omitempty"`
}

// GroupAssertions lists memberships to require or forbid in one subject state.
type GroupAssertions struct {
	Present []string `yaml:"present,omitempty" jsonschema:"uniqueItems=true"`
	Absent  []string `yaml:"absent,omitempty"  jsonschema:"uniqueItems=true"`
}

// Reconcile controls polling and persisted retry behavior.
type Reconcile struct {
	PollInterval Duration `yaml:"poll_interval,omitempty"`
	RetryInitial Duration `yaml:"retry_initial,omitempty"`
	RetryMax     Duration `yaml:"retry_max,omitempty"`
}

func (c *Config) applyDefaults() {
	if c.Listen == "" {
		c.Listen = ":8080"
	}
	if c.MetricsListen == "" {
		c.MetricsListen = ":8081"
	}
	if c.Database.MaxConnections == 0 {
		c.Database.MaxConnections = 10
	}
	if !c.Database.MaxConnLifetime.set {
		c.Database.MaxConnLifetime.Duration = 30 * time.Minute
	}
	if !c.Database.MaxConnIdleTime.set {
		c.Database.MaxConnIdleTime.Duration = 5 * time.Minute
	}
	if !c.Database.HealthCheckPeriod.set {
		c.Database.HealthCheckPeriod.Duration = time.Minute
	}
	if !c.Reconcile.PollInterval.set {
		c.Reconcile.PollInterval.Duration = time.Minute
	}
	if !c.Reconcile.RetryInitial.set {
		c.Reconcile.RetryInitial.Duration = 30 * time.Second
	}
	if !c.Reconcile.RetryMax.set {
		c.Reconcile.RetryMax.Duration = 15 * time.Minute
	}
}
