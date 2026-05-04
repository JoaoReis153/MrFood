package keycloak

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Client is a thin HTTP wrapper around the Keycloak REST and Admin APIs.
type Client struct {
	baseURL      string // e.g. http://keycloak:8080
	realm        string // e.g. mrfood
	clientID     string // confidential client id
	clientSecret string // confidential client secret
	adminUser    string // Keycloak master-realm admin username
	adminPass    string // Keycloak master-realm admin password
	http         *http.Client
}

// New returns a configured Keycloak client.
func New(baseURL, realm, clientID, clientSecret, adminUser, adminPass string) *Client {
	return &Client{
		baseURL:      strings.TrimRight(baseURL, "/"),
		realm:        realm,
		clientID:     clientID,
		clientSecret: clientSecret,
		adminUser:    adminUser,
		adminPass:    adminPass,
		http:         &http.Client{Timeout: 10 * time.Second},
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Public API
// ──────────────────────────────────────────────────────────────────────────────

// Login exchanges email + password for a token pair (Resource Owner Password Credentials).
func (c *Client) Login(ctx context.Context, email, password string) (*TokenResponse, error) {
	data := url.Values{
		"client_id":     {c.clientID},
		"client_secret": {c.clientSecret},
		"username":      {email},
		"password":      {password},
		"grant_type":    {"password"},
		"scope":         {"openid"},
	}

	var resp TokenResponse
	if err := c.postForm(ctx, c.tokenURL(), data, http.StatusOK, &resp); err != nil {
		return nil, fmt.Errorf("login: %w", err)
	}
	return &resp, nil
}

// RefreshToken exchanges a refresh token for a new token pair.
func (c *Client) RefreshToken(ctx context.Context, refreshToken string) (*TokenResponse, error) {
	data := url.Values{
		"client_id":     {c.clientID},
		"client_secret": {c.clientSecret},
		"refresh_token": {refreshToken},
		"grant_type":    {"refresh_token"},
	}

	var resp TokenResponse
	if err := c.postForm(ctx, c.tokenURL(), data, http.StatusOK, &resp); err != nil {
		return nil, fmt.Errorf("refresh token: %w", err)
	}
	return &resp, nil
}

// CreateUser creates a new user in Keycloak and returns their UUID.
func (c *Client) CreateUser(ctx context.Context, username, email, password string) (string, error) {
	adminToken, err := c.getAdminToken(ctx)
	if err != nil {
		return "", fmt.Errorf("get admin token: %w", err)
	}

	type credential struct {
		Type      string `json:"type"`
		Value     string `json:"value"`
		Temporary bool   `json:"temporary"`
	}
	payload := struct {
		Username        string       `json:"username"`
		Email           string       `json:"email"`
		Enabled         bool         `json:"enabled"`
		EmailVerified   bool         `json:"emailVerified"`
		RequiredActions []string     `json:"requiredActions"`
		Credentials     []credential `json:"credentials"`
	}{
		Username:        username,
		Email:           email,
		Enabled:         true,
		EmailVerified:   true,
		RequiredActions: []string{},
		Credentials: []credential{
			{Type: "password", Value: password, Temporary: false},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal user payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.adminURL()+"/users", strings.NewReader(string(body)))
	if err != nil {
		return "", fmt.Errorf("create user request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)

	httpResp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("create user: %w", err)
	}
	defer func() { _ = httpResp.Body.Close() }()

	switch httpResp.StatusCode {
	case http.StatusCreated:
		// OK — user ID is in the Location header
	case http.StatusConflict:
		return "", ErrUserAlreadyExists
	default:
		rb, _ := io.ReadAll(httpResp.Body)
		return "", fmt.Errorf("create user failed (%d): %s", httpResp.StatusCode, string(rb))
	}

	// Location: .../admin/realms/mrfood/users/{uuid}
	location := httpResp.Header.Get("Location")
	parts := strings.Split(location, "/")
	if len(parts) == 0 {
		return "", fmt.Errorf("missing Location header in create user response")
	}
	userUUID := parts[len(parts)-1]

	// Store the int64 representation of the UUID as the "id" attribute so the
	// realm's user_id mapper can embed a numeric user_id in the JWT.
	intID := uuidToPositiveInt64String(userUUID)
	if err := c.setUserAttribute(ctx, adminToken, userUUID, username, email, "int_id", intID); err != nil {
		slog.Warn("failed to set user id attribute", "uuid", userUUID, "error", err)
	}

	return userUUID, nil
}

// uuidToPositiveInt64String hashes a UUID to a positive int64 and returns it as a decimal string.
// The high bit is always cleared so the value is guaranteed non-negative.
func uuidToPositiveInt64String(id string) string {
	h := fnv.New64a()
	h.Write([]byte(id))
	positive := int64(h.Sum64() &^ (uint64(1) << 63))
	return strconv.FormatInt(positive, 10)
}

// setUserAttribute writes a single user attribute via the Admin REST API.
// All identity fields must be included because PUT replaces the full representation.
func (c *Client) setUserAttribute(ctx context.Context, adminToken, userID, username, email, key, value string) error {
	payload := struct {
		Username        string              `json:"username"`
		Email           string              `json:"email"`
		Enabled         bool                `json:"enabled"`
		EmailVerified   bool                `json:"emailVerified"`
		RequiredActions []string            `json:"requiredActions"`
		Attributes      map[string][]string `json:"attributes"`
	}{
		Username:        username,
		Email:           email,
		Enabled:         true,
		EmailVerified:   true,
		RequiredActions: []string{},
		Attributes:      map[string][]string{key: {value}},
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPut,
		c.adminURL()+"/users/"+userID, strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNoContent {
		rb, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("set attribute failed (%d): %s", resp.StatusCode, string(rb))
	}
	return nil
}

// GetUserByEmail returns the first user whose email exactly matches.
func (c *Client) GetUserByEmail(ctx context.Context, email string) (*UserRepresentation, error) {
	adminToken, err := c.getAdminToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("get admin token: %w", err)
	}

	u := c.adminURL() + "/users?email=" + url.QueryEscape(email) + "&exact=true"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("get user request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+adminToken)

	httpResp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode != http.StatusOK {
		rb, _ := io.ReadAll(httpResp.Body)
		return nil, fmt.Errorf("get user failed (%d): %s", httpResp.StatusCode, string(rb))
	}

	var users []UserRepresentation
	if err := json.NewDecoder(httpResp.Body).Decode(&users); err != nil {
		return nil, fmt.Errorf("decode users: %w", err)
	}
	if len(users) == 0 {
		return nil, ErrUserNotFound
	}
	return &users[0], nil
}

// RevokeAllUserSessions terminates every active session for the given Keycloak user UUID.
func (c *Client) RevokeAllUserSessions(ctx context.Context, userID string) error {
	adminToken, err := c.getAdminToken(ctx)
	if err != nil {
		return fmt.Errorf("get admin token: %w", err)
	}

	u := fmt.Sprintf("%s/users/%s/logout", c.adminURL(), userID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, nil)
	if err != nil {
		return fmt.Errorf("logout request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+adminToken)

	httpResp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("logout: %w", err)
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode != http.StatusNoContent {
		rb, _ := io.ReadAll(httpResp.Body)
		return fmt.Errorf("logout failed (%d): %s", httpResp.StatusCode, string(rb))
	}
	return nil
}

// Introspect checks whether a token is currently active.
func (c *Client) Introspect(ctx context.Context, token string) (*IntrospectResponse, error) {
	data := url.Values{
		"client_id":     {c.clientID},
		"client_secret": {c.clientSecret},
		"token":         {token},
	}

	var resp IntrospectResponse
	introspectURL := c.realmURL() + "/protocol/openid-connect/token/introspect"
	if err := c.postForm(ctx, introspectURL, data, http.StatusOK, &resp); err != nil {
		return nil, fmt.Errorf("introspect: %w", err)
	}
	if !resp.Active {
		return nil, ErrTokenInactive
	}
	return &resp, nil
}

// ──────────────────────────────────────────────────────────────────────────────
// Internal helpers
// ──────────────────────────────────────────────────────────────────────────────

func (c *Client) tokenURL() string {
	return fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token", c.baseURL, c.realm)
}

func (c *Client) realmURL() string {
	return fmt.Sprintf("%s/realms/%s", c.baseURL, c.realm)
}

func (c *Client) adminURL() string {
	return fmt.Sprintf("%s/admin/realms/%s", c.baseURL, c.realm)
}

// getAdminToken obtains a short-lived admin token from the master realm.
func (c *Client) getAdminToken(ctx context.Context) (string, error) {
	data := url.Values{
		"client_id":  {"admin-cli"},
		"username":   {c.adminUser},
		"password":   {c.adminPass},
		"grant_type": {"password"},
	}

	masterTokenURL := fmt.Sprintf("%s/realms/master/protocol/openid-connect/token", c.baseURL)
	var resp TokenResponse
	if err := c.postForm(ctx, masterTokenURL, data, http.StatusOK, &resp); err != nil {
		return "", fmt.Errorf("admin token: %w", err)
	}
	slog.Debug("obtained keycloak admin token")
	return resp.AccessToken, nil
}

// postForm sends an application/x-www-form-urlencoded POST and JSON-decodes the body into target.
func (c *Client) postForm(ctx context.Context, u string, data url.Values, wantStatus int, target interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != wantStatus {
		rb, _ := io.ReadAll(resp.Body)
		// Keycloak 26 returns HTTP 400 (not 401) for invalid_grant per RFC 6749 §5.2.
		// HTTP 401 is kept for legacy compatibility.
		var kcErr kcErrorResponse
		if jsonErr := json.Unmarshal(rb, &kcErr); jsonErr == nil {
			if resp.StatusCode == http.StatusUnauthorized || kcErr.Error == "invalid_grant" {
				return ErrInvalidCredentials
			}
			if kcErr.Description != "" {
				return fmt.Errorf("%s", kcErr.Description)
			}
		}
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(rb))
	}

	if target != nil {
		if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}
