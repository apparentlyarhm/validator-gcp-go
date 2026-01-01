package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
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
		log.Printf("[AUTH] Exchange code network request failed: %v", err)
		return nil, apperror.ErrInternal
	}

	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		log.Printf("[AUTH] GitHub returned status %d during token exchange", resp.StatusCode)
		return nil, apperror.ErrForbidden
	}

	var result models.GithubTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("[AUTH] Failed to decode GitHub token response: %v", err)
		return nil, apperror.ErrInternal
	}

	if result.AccessToken == "" {
		log.Printf("[AUTH] GitHub response contained no access token (likely bad code): %+v", result)
		return nil, apperror.ErrInternal
	}

	return &result, nil
}

func (a *AuthService) fetchGithubUser(ctx context.Context, accessToken string) (*models.GithubUserResponse, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/user", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := a.HttpClient.Do(req)
	if err != nil {
		log.Printf("[AUTH] GitHub user profile request failed: %v", err)
		return nil, apperror.ErrInternal
	}

	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, apperror.ErrForbidden
	}

	var user models.GithubUserResponse

	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		log.Printf("[AUTH] Failed to decode GitHub user profile: %v", err)
		return nil, apperror.ErrInternal
	}

	return &user, nil
}

func (a *AuthService) ValidateToken(tokenStr string) (*UserClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &UserClaims{}, func(t *jwt.Token) (interface{}, error) {

		// This prevents "None" algorithm attacks.
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}

		return []byte(a.Cfg.SigningSecret), nil
	})

	if err != nil {
		log.Printf("[AUTH] Token validation failed: %v", err)
		return nil, apperror.ErrForbidden
	}

	if claims, ok := token.Claims.(*UserClaims); ok && token.Valid {
		return claims, nil
	}

	log.Printf("[AUTH] Token claims invalid or token not valid")
	return nil, apperror.ErrForbidden
}
