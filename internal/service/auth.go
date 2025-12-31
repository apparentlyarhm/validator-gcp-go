package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/validator-gcp/v2/internal/apperror"
	"github.com/validator-gcp/v2/internal/config"
	"github.com/validator-gcp/v2/internal/models"
)

type UserClaims struct {
	Username string `json:"username"`
	Role     string `json:"role"`
	ID       string `json:"id"`
	jwt.RegisteredClaims
}

type AuthService struct {
	Cfg        *config.Config
	HttpClient *http.Client
}

func (a *AuthService) GetLogin(ctx context.Context) (*models.CommonResponse, error) {
	baseURL := "https://github.com/login/oauth/authorize"
	redirectURI := fmt.Sprintf("%s/callback", a.Cfg.FeHost)

	params := url.Values{}
	params.Add("client_id", a.Cfg.GitHub.ClientId)
	params.Add("redirect_uri", redirectURI)
	params.Add("scope", "read:user user:email")

	finalURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())

	return &models.CommonResponse{
		Message: finalURL,
	}, nil
}

func (a *AuthService) Callback(ctx context.Context, code string) (*models.LoginResponse, error) {
	tokenResp, err := a.exchangeCodeForToken(ctx, code)
	if err != nil {
		return nil, err
	}

	userInfo, err := a.fetchGithubUser(ctx, tokenResp.AccessToken)
	if err != nil {
		return nil, err
	}

	email := userInfo.Email
	uid := strconv.FormatInt(userInfo.ID, 10)
	login := userInfo.Login

	role := a.Cfg.GetRoleForUser(uid)

	expiration := time.Now().Add(3 * time.Hour)

	claims := UserClaims{
		Username: login,
		Role:     role,
		ID:       uid,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "github|" + login,
			ExpiresAt: jwt.NewNumericDate(expiration),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(a.Cfg.SigningSecret))
	if err != nil {
		return nil, apperror.ErrInternal
	}

	return &models.LoginResponse{
		Id:    login,
		Token: signedToken,
		Email: email,
	}, nil
}

func (a *AuthService) exchangeCodeForToken(ctx context.Context, code string) (*models.GithubTokenResponse, error) {
	reqBody := map[string]string{
		"client_id":     a.Cfg.GitHub.ClientId,
		"client_secret": a.Cfg.GitHub.ClientSecret,
		"code":          code,
		"redirect_uri":  fmt.Sprintf("%s/callback", a.Cfg.FeHost),
	}
	jsonBody, _ := json.Marshal(reqBody)

	req, _ := http.NewRequestWithContext(ctx, "POST", "https://github.com/login/oauth/access_token", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json") // Crucial for GitHub to return JSON

	resp, err := a.HttpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github token request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	fmt.Println("RAW GITHUB RESPONSE:", string(bodyBytes))

	// Restore body for the decoder
	resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	if resp.StatusCode >= 400 {
		return nil, apperror.ErrForbidden
	}

	var result models.GithubTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode github token: %w", err)
	}

	fmt.Printf("GitHub token response: %+v\n", result)
	if result.AccessToken == "" {
		return nil, fmt.Errorf("github returned empty access token")
	}

	return &result, nil
}

func (a *AuthService) fetchGithubUser(ctx context.Context, accessToken string) (*models.GithubUserResponse, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/user", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := a.HttpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github user request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, apperror.ErrForbidden
	}

	var user models.GithubUserResponse

	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("failed to decode github user: %w", err)
	}

	return &user, nil
}
