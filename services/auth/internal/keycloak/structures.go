package keycloak

// TokenResponse is the response from Keycloak's token endpoint.
type TokenResponse struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	ExpiresIn        int    `json:"expires_in"`
	RefreshExpiresIn int    `json:"refresh_expires_in"`
	TokenType        string `json:"token_type"`
	Scope            string `json:"scope"`
}

// UserRepresentation is a Keycloak user from the Admin API.
type UserRepresentation struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Enabled  bool   `json:"enabled"`
}

// IntrospectResponse is the result of a token introspection call.
type IntrospectResponse struct {
	Active   bool   `json:"active"`
	Sub      string `json:"sub"`
	Username string `json:"preferred_username"`
	Email    string `json:"email"`
}

// kcErrorResponse is used to decode error bodies from Keycloak.
type kcErrorResponse struct {
	Error       string `json:"error"`
	Description string `json:"error_description"`
}
