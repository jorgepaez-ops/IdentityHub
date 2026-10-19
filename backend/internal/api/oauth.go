package api

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"net/url"

	"github.com/jorgepaez/identity-hub/internal/auth/oauth"
	"github.com/jorgepaez/identity-hub/internal/auth/token"
)

const (
	problemOAuthUnavailable    = "oauth-unavailable"
	detailOAuthUnavailable     = "OAuth is temporarily unavailable."
	problemOAuthInvalidRequest = "oauth-invalid-request"
	detailOAuthInvalidRequest  = "The OAuth request is invalid."
)

const hubSessionCookieName = "hub_session"

func (s *Server) authorizeClient(w http.ResponseWriter, r *http.Request, params AuthorizeClientParams) {
	if s.oauth == nil || s.hubSessions == nil {
		writeProblem(w, http.StatusServiceUnavailable, problemOAuthUnavailable, titleServiceUnavailable, detailOAuthUnavailable)
		return
	}
	input := oauth.AuthorizeInput{ClientID: params.ClientId, RedirectURI: params.RedirectUri, ResponseType: string(params.ResponseType), State: params.State, CodeChallenge: params.CodeChallenge, CodeChallengeMethod: string(params.CodeChallengeMethod)}
	if hasDuplicateOAuthParameter(r.URL.Query()) {
		writeProblem(w, http.StatusBadRequest, problemOAuthInvalidRequest, titleBadRequest, detailOAuthInvalidRequest)
		return
	}
	if err := s.oauth.ValidateAuthorizeInput(input); err != nil {
		if client, ok := s.oauth.TrustedClient(input); ok {
			s.redirectToClient(w, r, client, "invalid_request", input.State)
			return
		}
		writeProblem(w, http.StatusBadRequest, problemOAuthInvalidRequest, titleBadRequest, detailOAuthInvalidRequest)
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
	if errors.Is(err, oauth.ErrHubSessionInvalid) {
		s.redirectToHubLogin(w, r)
		return
	}
	if err != nil {
		s.writeOAuthServerError(w, "hub_session_lookup", "oauth-session-failed", "The Hub session could not be verified.", err)
		return
	}
	result, err := s.oauth.Authorize(r.Context(), userID, input)
	var rejected *oauth.RejectedError
	if errors.As(err, &rejected) {
		// RFC 6749 section 4.1.2.1: client_id and redirect_uri are valid, so the
		// refusal travels to the client; the rejection was already audited.
		client, _ := s.oauth.TrustedClient(input)
		s.redirectToClient(w, r, client, "access_denied", input.State)
		return
	}
	if err != nil {
		s.writeOAuthServerError(w, "authorize", "oauth-authorization-failed", "The authorization code could not be issued.", err)
		return
	}
	client, _ := s.oauth.TrustedClient(input)
	target, ok := clientRedirectURL(client, url.Values{"code": {result.Code}, "state": {input.State}})
	if !ok {
		writeProblem(w, http.StatusServiceUnavailable, problemOAuthUnavailable, titleServiceUnavailable, detailOAuthUnavailable)
		return
	}
	http.Redirect(w, r, target, http.StatusFound)
}

func (s *Server) redirectOAuthError(w http.ResponseWriter, r *http.Request) bool {
	if s.oauth == nil {
		return false
	}
	query := r.URL.Query()
	if hasDuplicateOAuthParameter(query) || len(query["client_id"]) != 1 || len(query["redirect_uri"]) != 1 {
		return false
	}
	client, ok := s.oauth.TrustedClient(oauth.AuthorizeInput{ClientID: query.Get("client_id"), RedirectURI: query.Get("redirect_uri")})
	if !ok {
		return false
	}
	s.redirectToClient(w, r, client, "invalid_request", query.Get("state"))
	return true
}

// redirectToClient sends an OAuth error to the registered redirect URI of a
// client already validated by the caller (RFC 6749 section 4.1.2.1).
func (s *Server) redirectToClient(w http.ResponseWriter, r *http.Request, client oauth.Client, code, state string) {
	values := url.Values{"error": {code}}
	if state != "" {
		values.Set("state", state)
	}
	target, ok := clientRedirectURL(client, values)
	if !ok {
		writeProblem(w, http.StatusServiceUnavailable, problemOAuthUnavailable, titleServiceUnavailable, detailOAuthUnavailable)
		return
	}
	http.Redirect(w, r, target, http.StatusFound)
}

// clientRedirectURL adds values to the client's registered redirect URI. It
// reports false when the registration is not an absolute URL.
func clientRedirectURL(client oauth.Client, values url.Values) (string, bool) {
	target, err := url.Parse(client.RedirectURI)
	if err != nil || !target.IsAbs() {
		return "", false
	}
	query := target.Query()
	for key, list := range values {
		query[key] = list
	}
	target.RawQuery = query.Encode()
	return target.String(), true
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
	target := "/oauth/authorize?" + r.URL.Query().Encode()
	login, err := url.Parse(s.oauthLoginURL)
	if err != nil || !login.IsAbs() {
		writeProblem(w, http.StatusServiceUnavailable, problemOAuthUnavailable, titleServiceUnavailable, detailOAuthUnavailable)
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
		writeProblem(w, http.StatusServiceUnavailable, problemOAuthUnavailable, titleServiceUnavailable, detailOAuthUnavailable)
		return
	}
	if err := r.ParseForm(); err != nil || r.PostForm.Get("grant_type") != "authorization_code" || !oauth.ValidPKCEVerifier(r.PostForm.Get("code_verifier")) {
		writeProblem(w, http.StatusBadRequest, problemOAuthInvalidRequest, titleBadRequest, detailOAuthInvalidRequest)
		return
	}
	result, err := s.oauth.Exchange(r.Context(), oauth.ExchangeInput{Code: r.PostForm.Get("code"), ClientID: r.PostForm.Get("client_id"), RedirectURI: r.PostForm.Get("redirect_uri"), CodeVerifier: r.PostForm.Get("code_verifier")})
	if errors.Is(err, oauth.ErrAuthorizationCodeInvalid) {
		writeProblem(w, http.StatusBadRequest, "oauth-invalid-grant", titleBadRequest, "The authorization code is invalid.")
		return
	}
	if err != nil {
		s.writeOAuthServerError(w, "exchange", "oauth-token-failed", "The authorization code could not be exchanged.", err)
		return
	}
	access, err := s.tokens.IssueForAudience(result.UserID.String(), result.Roles, result.Permissions, result.ClientID)
	if err != nil {
		s.writeOAuthServerError(w, "issue_access_token", "oauth-token-failed", "The access token could not be issued.", err)
		return
	}
	writeJSON(w, http.StatusOK, OAuthTokenResponse{AccessToken: access, TokenType: OAuthTokenResponseTokenType("Bearer"), ExpiresIn: token.AccessTokenExpiresIn})
}

// writeOAuthServerError logs only server-side failure context. Request values
// such as authorization codes, PKCE verifiers and tokens are never logged.
func (s *Server) writeOAuthServerError(w http.ResponseWriter, operation, problemType, detail string, err error) {
	if s.logger != nil {
		s.logger.Error("oauth request failed", "operation", operation, "error", err)
	}
	writeProblem(w, http.StatusInternalServerError, problemType, titleInternalServerError, detail)
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
		// Exact match against the allowed origin of any registered client; never a wildcard.
		if origin := r.Header.Get("Origin"); s.oauth != nil && s.oauth.AllowsOrigin(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
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
