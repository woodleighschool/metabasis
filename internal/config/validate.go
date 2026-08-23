package config

import (
	"fmt"
	"log/slog"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"github.com/woodleighschool/metabasis/internal/expression"
)

const supportedVersion = 2

var identifierPattern = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)

func (c *Config) validateAndCompile() error {
	if c.Version != supportedVersion {
		return fmt.Errorf("config version must be %d, found %d", supportedVersion, c.Version)
	}
	if err := c.validateLogLevel(); err != nil {
		return err
	}
	if strings.TrimSpace(c.Listen) == "" {
		return fmt.Errorf("listen is required")
	}
	if err := c.validateConnections(); err != nil {
		return err
	}
	if err := c.validateDatabase(); err != nil {
		return err
	}
	if err := c.validateWebhooks(); err != nil {
		return err
	}
	if err := c.validateIdentity(); err != nil {
		return err
	}
	if err := c.validateRules(); err != nil {
		return err
	}
	return c.validateReconcile()
}

func (c *Config) validateLogLevel() error {
	switch c.LogLevel {
	case "debug":
		c.ParsedLevel = slog.LevelDebug
	case "info":
		c.ParsedLevel = slog.LevelInfo
	case "warn":
		c.ParsedLevel = slog.LevelWarn
	case "error":
		c.ParsedLevel = slog.LevelError
	default:
		return fmt.Errorf("log_level must be debug, info, warn, or error")
	}
	return nil
}

func (c *Config) validateConnections() error {
	if len(c.Connections) == 0 {
		return fmt.Errorf("connections must define at least one connection")
	}
	for _, name := range sortedKeys(c.Connections) {
		connection := c.Connections[name]
		if !identifierPattern.MatchString(name) {
			return fmt.Errorf("connections.%s: name must match %s", name, identifierPattern)
		}
		if connection.Type != "microsoft_graph" {
			return fmt.Errorf("connections.%s.type %q is not supported", name, connection.Type)
		}
		if strings.TrimSpace(connection.TenantID) == "" ||
			strings.TrimSpace(connection.ClientID) == "" ||
			strings.TrimSpace(connection.ClientSecret) == "" {
			return fmt.Errorf("connections.%s tenant_id, client_id, and client_secret are required", name)
		}
		if connection.BaseURL != "" {
			parsed, err := url.Parse(connection.BaseURL)
			if err != nil || parsed.Scheme == "" || parsed.Host == "" {
				return fmt.Errorf("connections.%s.base_url must be absolute", name)
			}
		}
	}
	return nil
}

func (c *Config) validateDatabase() error {
	parsed, err := url.Parse(c.Database.URL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") {
		return fmt.Errorf("database.url must be an absolute PostgreSQL URL")
	}
	if c.Database.MinConnections < 0 {
		return fmt.Errorf("database.min_connections must not be negative")
	}
	if c.Database.MaxConnections <= 0 {
		return fmt.Errorf("database.max_connections must be greater than zero")
	}
	if c.Database.MinConnections > c.Database.MaxConnections {
		return fmt.Errorf("database.min_connections must not exceed max_connections")
	}
	if c.Database.MaxConnLifetime.Duration <= 0 || c.Database.MaxConnIdleTime.Duration <= 0 ||
		c.Database.HealthCheckPeriod.Duration <= 0 {
		return fmt.Errorf("database connection durations must be greater than zero")
	}
	return nil
}

func (c *Config) validateWebhooks() error {
	if len(c.Webhooks) == 0 {
		return fmt.Errorf("webhooks must define at least one source")
	}
	paths := make(map[string]string, len(c.Webhooks))
	for _, name := range sortedKeys(c.Webhooks) {
		webhook := c.Webhooks[name]
		if !identifierPattern.MatchString(name) {
			return fmt.Errorf("webhooks.%s: name must match %s", name, identifierPattern)
		}
		if !strings.HasPrefix(webhook.Path, "/webhooks/") || strings.ContainsAny(webhook.Path, "?#") {
			return fmt.Errorf("webhooks.%s.path must start with /webhooks/ and contain no query or fragment", name)
		}
		if previous := paths[webhook.Path]; previous != "" {
			return fmt.Errorf("webhooks.%s.path duplicates webhooks.%s.path", name, previous)
		}
		paths[webhook.Path] = name
		if strings.TrimSpace(webhook.BearerToken) == "" {
			return fmt.Errorf("webhooks.%s.bearer_token is required", name)
		}
	}
	return nil
}

func (c *Config) validateIdentity() error {
	connection, ok := c.Connections[c.Identity.Connection]
	if !ok {
		return fmt.Errorf("identity.connection %q does not exist", c.Identity.Connection)
	}
	if connection.Type != "microsoft_graph" {
		return fmt.Errorf("identity requires a microsoft_graph connection")
	}
	if len(c.Identity.Groups) == 0 {
		return fmt.Errorf("identity.groups must not be empty")
	}
	for _, alias := range sortedKeys(c.Identity.Groups) {
		if !identifierPattern.MatchString(alias) {
			return fmt.Errorf("identity.groups.%s: alias must match %s", alias, identifierPattern)
		}
		groupIDs := c.Identity.Groups[alias]
		if len(groupIDs) == 0 {
			return fmt.Errorf("identity.groups.%s must not be empty", alias)
		}
		seen := make(map[string]struct{}, len(groupIDs))
		for _, groupID := range groupIDs {
			if strings.TrimSpace(groupID) == "" {
				return fmt.Errorf("identity.groups.%s contains an empty group ID", alias)
			}
			if _, exists := seen[groupID]; exists {
				return fmt.Errorf("identity.groups.%s contains duplicate group ID", alias)
			}
			seen[groupID] = struct{}{}
		}
	}
	return nil
}

func (c *Config) validateRules() error {
	if len(c.Rules) == 0 {
		return fmt.Errorf("rules must not be empty")
	}
	compiler, err := expression.NewCompiler()
	if err != nil {
		return err
	}
	programs := make([]expression.Program, len(c.Rules))
	seenNames := make(map[string]struct{}, len(c.Rules))
	for index, rule := range c.Rules {
		path := fmt.Sprintf("rules[%d]", index)
		if !identifierPattern.MatchString(rule.Name) {
			return fmt.Errorf("%s.name must match %s", path, identifierPattern)
		}
		if _, exists := seenNames[rule.Name]; exists {
			return fmt.Errorf("%s.name %q is duplicated", path, rule.Name)
		}
		seenNames[rule.Name] = struct{}{}
		program, compileErr := compiler.CompileCondition(rule.When)
		if compileErr != nil {
			return fmt.Errorf("%s.when: %w", path, compileErr)
		}
		programs[index] = program
		states := []struct {
			name       string
			assertions GroupAssertions
		}{
			{name: "pending", assertions: rule.States.Pending},
			{name: "active", assertions: rule.States.Active},
			{name: "settled", assertions: rule.States.Settled},
		}
		hasAssertions := false
		for _, state := range states {
			if len(state.assertions.Present) != 0 || len(state.assertions.Absent) != 0 {
				hasAssertions = true
			}
			if err := c.validateGroupAssertions(path+".states."+state.name, state.assertions); err != nil {
				return err
			}
		}
		if !hasAssertions {
			return fmt.Errorf("%s.states must contain at least one group assertion", path)
		}
	}
	c.Programs = programs
	return nil
}

func (c *Config) validateGroupAssertions(path string, assertions GroupAssertions) error {
	type assertion struct {
		alias string
		state string
	}
	targets := make(map[string]assertion, len(assertions.Present)+len(assertions.Absent))
	for _, group := range []struct {
		state   string
		aliases []string
	}{
		{state: "present", aliases: assertions.Present},
		{state: "absent", aliases: assertions.Absent},
	} {
		seen := make(map[string]struct{}, len(group.aliases))
		for _, alias := range group.aliases {
			groupIDs, ok := c.Identity.Groups[alias]
			if !ok {
				return fmt.Errorf("%s.%s references unknown identity group alias %q", path, group.state, alias)
			}
			if len(groupIDs) != 1 {
				return fmt.Errorf("%s.%s identity group alias %q must resolve to exactly one group ID", path, group.state, alias)
			}
			if _, exists := seen[alias]; exists {
				return fmt.Errorf("%s.%s contains duplicate %q", path, group.state, alias)
			}
			seen[alias] = struct{}{}
			if previous, exists := targets[groupIDs[0]]; exists {
				if previous.state != group.state {
					return fmt.Errorf("%s both requires and forbids identity group alias %q", path, previous.alias)
				}
				return fmt.Errorf(
					"%s.%s aliases %q and %q resolve to the same group ID",
					path,
					group.state,
					previous.alias,
					alias,
				)
			}
			targets[groupIDs[0]] = assertion{alias: alias, state: group.state}
		}
	}
	return nil
}

func (c *Config) validateReconcile() error {
	if c.Reconcile.PollInterval.Duration <= 0 || c.Reconcile.RetryInitial.Duration <= 0 ||
		c.Reconcile.RetryMax.Duration <= 0 {
		return fmt.Errorf("reconcile durations must be greater than zero")
	}
	if c.Reconcile.RetryInitial.Duration > c.Reconcile.RetryMax.Duration {
		return fmt.Errorf("reconcile.retry_initial must not exceed retry_max")
	}
	return nil
}

func sortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
