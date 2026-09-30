package api

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/jorgepaez/identity-hub/internal/auth/oauth"
	"github.com/jorgepaez/identity-hub/internal/auth/token"
)

const hubSessionCookieName = "hub_session"

func (s *Server) authorizeClient(w http.ResponseWriter, r *http.Request, params AuthorizeClientParams) {
	if s.oauth == nil || s.hubSessions == nil {
		writeProblem(w, http.StatusServiceUnavailable, "oauth-unavailable", "Service Unavailable", "OAuth is temporarily unavailable.")
		return
	}
	input := oauth.AuthorizeInput{ClientID: params.ClientId, RedirectURI: params.RedirectUri, ResponseType: string(params.ResponseType), State: params.State, CodeChallenge: params.CodeChallenge, CodeChallengeMethod: string(params.CodeChallengeMethod)}
	if hasDuplicateOAuthParameter(r.URL.Query()) {
		writeProblem(w, http.StatusBadRequest, "oauth-invalid-request", "Bad Request", "The OAuth request is invalid.")
		return
	}
	if !s.validOAuthRequest(input) {
		if input.ClientID == s.oauthClient.ID && input.RedirectURI == s.oauthClient.RedirectURI {
			s.redirectOAuthInvalid(w, r, input.State)
			return
		}
		writeProblem(w, http.StatusBadRequest, "oauth-invalid-request", "Bad Request", "The OAuth request is invalid.")
		return
	}
	cookie, err := r.Cookie(hubSessionCookieName)
	if err != nil {
		s.redirectToHubLogin(w, r)
		return
	}
	raw, err := base64.RawURLEncoding.DecodeString(cookie.Value)
	if err != nil {
		s.redirectToHubLogin(w, r)
		return
	}
	hash := sha256.Sum256(raw)
	userID, err := s.hubSessions.GetHubSessionUser(r.Context(), hash[:])
	if err != nil {
		s.redirectToHubLogin(w, r)
		return
	}
	result, err := s.oauth.Authorize(r.Context(), userID, input)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "oauth-authorization-failed", "Internal Server Error", "The authorization code could not be issued.")
		return
	}
	http.Redirect(w, r, s.clientRedirectURL(url.Values{"code": {result.Code}, "state": {input.State}}), http.StatusFound)
}

func (s *Server) validOAuthRequest(input oauth.AuthorizeInput) bool {
	return input.ClientID == s.oauthClient.ID && input.RedirectURI == s.oauthClient.RedirectURI && input.ResponseType == "code" && input.State != "" && validPKCEChallenge(input.CodeChallenge) && input.CodeChallengeMethod == "S256"
}
func validPKCEChallenge(value string) bool {
	return validPKCE(value, "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_")
}
func validPKCEVerifier(value string) bool {
	return validPKCE(value, "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-._~")
}
func validPKCE(value, alphabet string) bool {
	if len(value) < 43 || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if !strings.ContainsRune(alphabet, character) {
			return false
		}
	}
	return true
}
func (s *Server) redirectOAuthError(w http.ResponseWriter, r *http.Request) bool {
	if s.oauth == nil || s.oauthRedirect == nil {
		return false
	}
	if hasDuplicateOAuthParameter(r.URL.Query()) || len(r.URL.Query()["client_id"]) != 1 || len(r.URL.Query()["redirect_uri"]) != 1 {
		return false
	}
	if r.URL.Query().Get("client_id") != s.oauthClient.ID || r.URL.Query().Get("redirect_uri") != s.oauthClient.RedirectURI {
		return false
	}
	s.redirectOAuthInvalid(w, r, r.URL.Query().Get("state"))
	return true
}
func (s *Server) redirectOAuthInvalid(w http.ResponseWriter, r *http.Request, state string) {
	values := url.Values{"error": {"invalid_request"}}
	if state != "" {
		values.Set("state", state)
	}
	http.Redirect(w, r, s.clientRedirectURL(values), http.StatusFound)
}

// clientRedirectURL adds values to the registered redirect URI, parsed once in
// SetOAuthService so a request path can never dereference a nil URL.
func (s *Server) clientRedirectURL(values url.Values) string {
	target := *s.oauthRedirect
	query := target.Query()
	for key, list := range values {
		query[key] = list
	}
	target.RawQuery = query.Encode()
	return target.String()
}
func hasDuplicateOAuthParameter(query url.Values) bool {
	for _, key := range []string{"client_id", "redirect_uri", "response_type", "state", "code_challenge", "code_challenge_method"} {
		if len(query[key]) > 1 {
			return true
		}
	}
	return false
}
func (s *Server) redirectToHubLogin(w http.ResponseWriter, r *http.Request) {
	continuePath := r.URL.EscapedPath()
	if continuePath != "/oauth/authorize" {
		continuePath = "/oauth/authorize"
	}
	target := continuePath + "?" + r.URL.Query().Encode()
	login, err := url.Parse(s.oauthLoginURL)
	if err != nil || !login.IsAbs() {
		writeProblem(w, http.StatusServiceUnavailable, "oauth-unavailable", "Service Unavailable", "OAuth is temporarily unavailable.")
		return
	}
	values := login.Query()
	values.Set("continue", target)
	login.RawQuery = values.Encode()
	http.Redirect(w, r, login.String(), http.StatusFound)
}
func (s *Server) exchangeAuthorizationCode(w http.ResponseWriter, r *http.Request) {
	// RFC 6749 section 5.1: token endpoint responses must never be cached.
	w.Header().Set("Cache-Control", "no-store")
	if s.oauth == nil || s.tokens == nil {
		writeProblem(w, http.StatusServiceUnavailable, "oauth-unavailable", "Service Unavailable", "OAuth is temporarily unavailable.")
		return
	}
	if err := r.ParseForm(); err != nil || r.PostForm.Get("grant_type") != "authorization_code" || !validPKCEVerifier(r.PostForm.Get("code_verifier")) {
		writeProblem(w, http.StatusBadRequest, "oauth-invalid-request", "Bad Request", "The OAuth request is invalid.")
		return
	}
	result, err := s.oauth.Exchange(r.Context(), oauth.ExchangeInput{Code: r.PostForm.Get("code"), ClientID: r.PostForm.Get("client_id"), RedirectURI: r.PostForm.Get("redirect_uri"), CodeVerifier: r.PostForm.Get("code_verifier")})
	if errors.Is(err, oauth.ErrAuthorizationCodeInvalid) {
		writeProblem(w, http.StatusBadRequest, "oauth-invalid-grant", "Bad Request", "The authorization code is invalid.")
		return
	}
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "oauth-token-failed", "Internal Server Error", "The authorization code could not be exchanged.")
		return
	}
	access, err := s.tokens.IssueForAudience(result.UserID.String(), result.Roles, s.oauthClient.ID)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "oauth-token-failed", "Internal Server Error", "The access token could not be issued.")
		return
	}
	writeJSON(w, http.StatusOK, OAuthTokenResponse{AccessToken: access, TokenType: OAuthTokenResponseTokenType("Bearer"), ExpiresIn: token.AccessTokenExpiresIn})
}
func hubSessionCookie(value string, maxAge int) *http.Cookie {
	return &http.Cookie{Name: hubSessionCookieName, Value: value, Path: "/oauth", MaxAge: maxAge, HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode}
}
func (s *Server) oauthCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth/token" && r.URL.Path != "/.well-known/jwks.json" {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Add("Vary", "Origin")
		if r.Header.Get("Origin") == s.oauthClient.Origin {
			w.Header().Set("Access-Control-Allow-Origin", s.oauthClient.Origin)
		}
		if r.Method == http.MethodOptions && r.URL.Path == "/oauth/token" {
			w.Header().Set("Access-Control-Allow-Methods", http.MethodPost)
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
