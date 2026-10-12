package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

func generateValidInitData(botToken string, params map[string]string) string {
	var pairs []string
	for k, v := range params {
		pairs = append(pairs, fmt.Sprintf("%s=%s", k, v))
	}
	sort.Strings(pairs)
	dataCheckString := strings.Join(pairs, "\n")

	macSecret := hmac.New(sha256.New, []byte("WebAppData"))
	macSecret.Write([]byte(botToken))
	secretKey := macSecret.Sum(nil)

	macData := hmac.New(sha256.New, secretKey)
	macData.Write([]byte(dataCheckString))
	hash := hex.EncodeToString(macData.Sum(nil))

	query := url.Values{}
	for k, v := range params {
		query.Set(k, v)
	}
	query.Set("hash", hash)

	return query.Encode()
}

func TestValidateInitData_Success(t *testing.T) {
	token := "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11"
	params := map[string]string{
		"auth_date": strconv.FormatInt(time.Now().Unix(), 10),
		"query_id":  "AAHdF6IQAAAAAN0XohDhrOrc",
		"user":      `{"id":12345678,"first_name":"Иван","username":"ivan_test"}`,
	}

	initData := generateValidInitData(token, params)

	user, err := ValidateInitData(initData, token, time.Hour)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if user.ID != 12345678 {
		t.Errorf("expected user ID 12345678, got %d", user.ID)
	}
	if user.FirstName != "Иван" {
		t.Errorf("expected first name Иван, got %s", user.FirstName)
	}
	if user.Username != "ivan_test" {
		t.Errorf("expected username ivan_test, got %s", user.Username)
	}
}

func TestValidateInitData_InvalidHash(t *testing.T) {
	token := "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11"
	params := map[string]string{
		"auth_date": strconv.FormatInt(time.Now().Unix(), 10),
		"user":      `{"id":12345678}`,
	}

	initData := generateValidInitData(token, params)
	// Mutate token to simulate wrong signature
	_, err := ValidateInitData(initData, "wrong_token", time.Hour)
	if !errors.Is(err, ErrInvalidHash) {
		t.Fatalf("expected ErrInvalidHash, got %v", err)
	}
}

func TestValidateInitData_Outdated(t *testing.T) {
	token := "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11"
	oldTime := time.Now().Add(-2 * time.Hour).Unix()
	params := map[string]string{
		"auth_date": strconv.FormatInt(oldTime, 10),
		"user":      `{"id":12345678}`,
	}

	initData := generateValidInitData(token, params)
	_, err := ValidateInitData(initData, token, time.Hour)
	if !errors.Is(err, ErrOutdatedAuth) {
		t.Fatalf("expected ErrOutdatedAuth, got %v", err)
	}
}
