package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

// OAuth2 metadata structure for Claude Teams
type OAuth2Metadata struct {
	Issuer                            string   `json:"issuer"`
	AuthorizationEndpoint             string   `json:"authorization_endpoint"`
	TokenEndpoint                     string   `json:"token_endpoint"`
	ResponseTypesSupported            []string `json:"response_types_supported"`
	GrantTypesSupported               []string `json:"grant_types_supported"`
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported"`
}

// Client registration request from Claude Teams
type ClientRegistrationRequest struct {
	ClientName    string   `json:"client_name"`
	RedirectURIs  []string `json:"redirect_uris"`
	GrantTypes    []string `json:"grant_types,omitempty"`
	ResponseTypes []string `json:"response_types,omitempty"`
}

// Client registration response
type ClientRegistrationResponse struct {
	ClientID                string   `json:"client_id"`
	ClientSecret            string   `json:"client_secret"`
	ClientName              string   `json:"client_name"`
	RedirectURIs            []string `json:"redirect_uris"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
	ClientIDIssuedAt        int64    `json:"client_id_issued_at"`
	ClientSecretExpiresAt   int      `json:"client_secret_expires_at"`
}

// Token response
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

// OAuth wrapper server
type OAuthWrapper struct {
	clients      map[string]*ClientRegistrationResponse
	authCodes    map[string]*AuthCode
	accessTokens map[string]*AccessToken
	mu           sync.RWMutex
	mcpURL       string
	slackToken   string
	publicURL    string
}

// AuthCode stores authorization code data
type AuthCode struct {
	ClientID    string
	RedirectURI string
	ExpiresAt   time.Time
}

// AccessToken stores access token data
type AccessToken struct {
	ClientID  string
	ExpiresAt time.Time
}

func main() {
	// Get configuration from environment
	port := os.Getenv("PORT")  // Runway sets PORT
	if port == "" {
		port = os.Getenv("OAUTH_WRAPPER_PORT")
		if port == "" {
			port = "8080"
		}
	}

	publicURL := os.Getenv("OAUTH_WRAPPER_PUBLIC_URL")
	if publicURL == "" {
		// Try Runway's app URL
		if runwayURL := os.Getenv("RUNWAY_APP_URL"); runwayURL != "" {
			publicURL = runwayURL
		} else {
			publicURL = "http://localhost:" + port
		}
	}

	mcpHost := os.Getenv("SLACK_MCP_HOST")
	if mcpHost == "" {
		mcpHost = "127.0.0.1"
	}

	mcpPort := os.Getenv("SLACK_MCP_PORT")
	if mcpPort == "" {
		mcpPort = "13080"
	}

	slackToken := os.Getenv("SLACK_MCP_XOXP_TOKEN")
	if slackToken == "" {
		log.Fatal("SLACK_MCP_XOXP_TOKEN environment variable is required")
	}

	wrapper := &OAuthWrapper{
		clients:      make(map[string]*ClientRegistrationResponse),
		authCodes:    make(map[string]*AuthCode),
		accessTokens: make(map[string]*AccessToken),
		mcpURL:       fmt.Sprintf("http://%s:%s", mcpHost, mcpPort),
		slackToken:   slackToken,
		publicURL:    publicURL,
	}

	// Setup routes
	http.HandleFunc("/.well-known/oauth-authorization-server", wrapper.handleMetadata)
	http.HandleFunc("/register", wrapper.handleRegistration)
	http.HandleFunc("/authorize", wrapper.handleAuthorize)
	http.HandleFunc("/oauth/callback", wrapper.handleCallback)
	http.HandleFunc("/token", wrapper.handleToken)
	http.HandleFunc("/sse", wrapper.handleSSEProxy)
	http.HandleFunc("/health", wrapper.handleHealth)

	log.Printf("OAuth wrapper server starting on port %s", port)
	log.Printf("Public URL: %s", publicURL)
	log.Printf("MCP Server URL: %s", wrapper.mcpURL)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

// Handle OAuth metadata endpoint
func (w *OAuthWrapper) handleMetadata(rw http.ResponseWriter, r *http.Request) {
	metadata := OAuth2Metadata{
		Issuer:                w.publicURL,
		AuthorizationEndpoint: w.publicURL + "/authorize",
		TokenEndpoint:         w.publicURL + "/token",
		ResponseTypesSupported: []string{"code"},
		GrantTypesSupported:    []string{"authorization_code"},
		TokenEndpointAuthMethodsSupported: []string{"client_secret_post", "client_secret_basic"},
	}

	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(metadata)
}

// Handle client registration
func (w *OAuthWrapper) handleRegistration(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(rw, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ClientRegistrationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(rw, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Generate client credentials
	clientID := generateRandomString(32)
	clientSecret := generateRandomString(64)

	response := &ClientRegistrationResponse{
		ClientID:              clientID,
		ClientSecret:          clientSecret,
		ClientName:            req.ClientName,
		RedirectURIs:          req.RedirectURIs,
		GrantTypes:            []string{"authorization_code"},
		ResponseTypes:         []string{"code"},
		ClientIDIssuedAt:      time.Now().Unix(),
		ClientSecretExpiresAt: 0, // Never expires
	}

	// Store client
	w.mu.Lock()
	w.clients[clientID] = response
	w.mu.Unlock()

	log.Printf("Registered new client: %s (%s)", clientID, req.ClientName)

	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(response)
}

// Handle authorization request
func (w *OAuthWrapper) handleAuthorize(rw http.ResponseWriter, r *http.Request) {
	clientID := r.URL.Query().Get("client_id")
	redirectURI := r.URL.Query().Get("redirect_uri")
	responseType := r.URL.Query().Get("response_type")
	state := r.URL.Query().Get("state")

	// Validate client
	w.mu.RLock()
	client, exists := w.clients[clientID]
	w.mu.RUnlock()

	if !exists {
		http.Error(rw, "Invalid client_id", http.StatusBadRequest)
		return
	}

	// Validate redirect URI
	validRedirect := false
	for _, uri := range client.RedirectURIs {
		if uri == redirectURI {
			validRedirect = true
			break
		}
	}

	if !validRedirect {
		http.Error(rw, "Invalid redirect_uri", http.StatusBadRequest)
		return
	}

	if responseType != "code" {
		http.Error(rw, "Unsupported response_type", http.StatusBadRequest)
		return
	}

	// Generate authorization code
	authCode := generateRandomString(32)

	// Store auth code
	w.mu.Lock()
	w.authCodes[authCode] = &AuthCode{
		ClientID:    clientID,
		RedirectURI: redirectURI,
		ExpiresAt:   time.Now().Add(10 * time.Minute),
	}
	w.mu.Unlock()

	// Redirect back to client with auth code
	redirectURL, _ := url.Parse(redirectURI)
	q := redirectURL.Query()
	q.Set("code", authCode)
	if state != "" {
		q.Set("state", state)
	}
	redirectURL.RawQuery = q.Encode()

	http.Redirect(rw, r, redirectURL.String(), http.StatusFound)
}

// Handle OAuth callback (not typically used, but included for completeness)
func (w *OAuthWrapper) handleCallback(rw http.ResponseWriter, r *http.Request) {
	// This endpoint is typically not used in this flow
	// Claude Teams will handle the callback on their side
	rw.WriteHeader(http.StatusOK)
	fmt.Fprintf(rw, "OAuth callback received")
}

// Handle token exchange
func (w *OAuthWrapper) handleToken(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(rw, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse form data
	if err := r.ParseForm(); err != nil {
		http.Error(rw, "Invalid request", http.StatusBadRequest)
		return
	}

	grantType := r.FormValue("grant_type")
	if grantType != "authorization_code" {
		http.Error(rw, "Unsupported grant_type", http.StatusBadRequest)
		return
	}

	code := r.FormValue("code")
	clientID := r.FormValue("client_id")
	clientSecret := r.FormValue("client_secret")
	redirectURI := r.FormValue("redirect_uri")

	// Check for basic auth if not in form
	if clientID == "" || clientSecret == "" {
		if user, pass, ok := r.BasicAuth(); ok {
			clientID = user
			clientSecret = pass
		}
	}

	// Validate client
	w.mu.RLock()
	client, exists := w.clients[clientID]
	w.mu.RUnlock()

	if !exists || client.ClientSecret != clientSecret {
		http.Error(rw, "Invalid client credentials", http.StatusUnauthorized)
		return
	}

	// Validate auth code
	w.mu.Lock()
	authCode, exists := w.authCodes[code]
	if exists {
		delete(w.authCodes, code) // Use once only
	}
	w.mu.Unlock()

	if !exists || authCode.ClientID != clientID || authCode.RedirectURI != redirectURI {
		http.Error(rw, "Invalid authorization code", http.StatusBadRequest)
		return
	}

	if time.Now().After(authCode.ExpiresAt) {
		http.Error(rw, "Authorization code expired", http.StatusBadRequest)
		return
	}

	// Generate access token
	accessToken := generateRandomString(64)

	// Store access token
	w.mu.Lock()
	w.accessTokens[accessToken] = &AccessToken{
		ClientID:  clientID,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	w.mu.Unlock()

	// Return token response
	response := TokenResponse{
		AccessToken: accessToken,
		TokenType:   "Bearer",
		ExpiresIn:   86400, // 24 hours
	}

	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(response)
}

// Proxy SSE requests to MCP server
func (w *OAuthWrapper) handleSSEProxy(rw http.ResponseWriter, r *http.Request) {
	// Validate access token
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		http.Error(rw, "Unauthorized", http.StatusUnauthorized)
		return
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")

	w.mu.RLock()
	accessToken, exists := w.accessTokens[token]
	w.mu.RUnlock()

	if !exists || time.Now().After(accessToken.ExpiresAt) {
		http.Error(rw, "Invalid or expired token", http.StatusUnauthorized)
		return
	}

	// Create reverse proxy to MCP server
	target, _ := url.Parse(w.mcpURL + "/sse")
	proxy := httputil.NewSingleHostReverseProxy(target)

	// Modify request to remove OAuth token and add MCP auth if configured
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		
		// Remove OAuth token
		req.Header.Del("Authorization")
		
		// Add MCP SSE API key if configured
		if sseAPIKey := os.Getenv("SLACK_MCP_SSE_API_KEY"); sseAPIKey != "" {
			req.Header.Set("Authorization", "Bearer "+sseAPIKey)
		}
	}

	// Start MCP server if not already running
	go w.ensureMCPServerRunning()

	// Proxy the request
	proxy.ServeHTTP(rw, r)
}

// Ensure MCP server is running
func (w *OAuthWrapper) ensureMCPServerRunning() {
	// Check if MCP server is already running
	resp, err := http.Get(w.mcpURL + "/health")
	if err == nil && resp.StatusCode == http.StatusOK {
		resp.Body.Close()
		return
	}

	// MCP server should be started by the start script
	// This is just a health check
	log.Printf("Warning: MCP server may not be running at %s", w.mcpURL)
}

// Health check endpoint
func (w *OAuthWrapper) handleHealth(rw http.ResponseWriter, r *http.Request) {
	rw.WriteHeader(http.StatusOK)
	fmt.Fprintf(rw, "OK")
}

// Generate random string for tokens and codes
func generateRandomString(length int) string {
	bytes := make([]byte, length)
	rand.Read(bytes)
	return base64.URLEncoding.EncodeToString(bytes)[:length]
}