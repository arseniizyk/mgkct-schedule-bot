package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	ErrEmptyInitData = errors.New("empty init data")
	ErrMissingHash   = errors.New("missing hash in init data")
	ErrInvalidHash   = errors.New("invalid init data signature")
	ErrOutdatedAuth  = errors.New("init data is outdated")
	ErrMissingUser   = errors.New("user not found in init data")
	ErrInvalidUser   = errors.New("failed to parse user JSON")
)

type TelegramUser struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
}

// ValidateInitData проверяет валидность строки Telegram WebApp initData с помощью токена бота.
// maxAge - максимальный допустимый возраст auth_date (0 - без проверки возраста).
func ValidateInitData(initData, botToken string, maxAge time.Duration) (*TelegramUser, error) {
	if strings.TrimSpace(initData) == "" {
		return nil, ErrEmptyInitData
	}

	values, err := url.ParseQuery(initData)
	if err != nil {
		return nil, fmt.Errorf("parse init data query: %w", err)
	}

	receivedHash := values.Get("hash")
	if receivedHash == "" {
		return nil, ErrMissingHash
	}

	// Формируем список пар ключ=значение без hash
	var pairs []string
	for k := range values {
		if k == "hash" {
			continue
		}
		pairs = append(pairs, fmt.Sprintf("%s=%s", k, values.Get(k)))
	}
	sort.Strings(pairs)
	dataCheckString := strings.Join(pairs, "\n")

	// 1. secret_key = HMAC_SHA256("WebAppData", botToken)
	macSecret := hmac.New(sha256.New, []byte("WebAppData"))
	macSecret.Write([]byte(botToken))
	secretKey := macSecret.Sum(nil)

	// 2. data_hash = HMAC_SHA256(secretKey, dataCheckString)
	macData := hmac.New(sha256.New, secretKey)
	macData.Write([]byte(dataCheckString))
	expectedHash := hex.EncodeToString(macData.Sum(nil))

	if subtle.ConstantTimeCompare([]byte(expectedHash), []byte(receivedHash)) != 1 {
		return nil, ErrInvalidHash
	}

	// Проверка возраста сессии (если задан maxAge)
	if maxAge > 0 {
		authDateStr := values.Get("auth_date")
		if authDateStr != "" {
			sec, err := strconv.ParseInt(authDateStr, 10, 64)
			if err == nil {
				authTime := time.Unix(sec, 0)
				if time.Since(authTime) > maxAge {
					return nil, ErrOutdatedAuth
				}
			}
		}
	}

	userStr := values.Get("user")
	if userStr == "" {
		return nil, ErrMissingUser
	}

	var user TelegramUser
	if err := json.Unmarshal([]byte(userStr), &user); err != nil {
		return nil, ErrInvalidUser
	}

	return &user, nil
}
