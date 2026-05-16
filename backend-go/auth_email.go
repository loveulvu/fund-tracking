package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"strings"
	"time"
)

type verificationEmailConfig struct {
	APIKey   string
	From     string
	FromName string
}

type verificationEmailResponse struct {
	ID string `json:"id"`
}

func generateVerificationCode() (string, error) {
	max := big.NewInt(1000000)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

func isValidVerificationCodeFormat(code string) bool {
	if len(code) != 6 {
		return false
	}
	for _, char := range code {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func hashVerificationCode(code string) (string, error) {
	secret := strings.TrimSpace(os.Getenv("JWT_SECRET"))
	if secret == "" {
		return "", fmt.Errorf("JWT_SECRET is not set")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(code))
	return hex.EncodeToString(mac.Sum(nil)), nil
}

func getVerificationEmailConfig() (verificationEmailConfig, error) {
	config := verificationEmailConfig{
		APIKey:   strings.TrimSpace(os.Getenv("RESEND_API_KEY")),
		From:     strings.TrimSpace(os.Getenv("ALERT_EMAIL_FROM")),
		FromName: strings.TrimSpace(os.Getenv("ALERT_EMAIL_FROM_NAME")),
	}
	missing := make([]string, 0)
	if config.APIKey == "" {
		missing = append(missing, "RESEND_API_KEY")
	}
	if config.From == "" {
		missing = append(missing, "ALERT_EMAIL_FROM")
	}
	if len(missing) > 0 {
		return verificationEmailConfig{}, fmt.Errorf("missing required email environment variables: %s", strings.Join(missing, ", "))
	}
	return config, nil
}

func sendVerificationCodeEmail(email string, code string) error {
	config, err := getVerificationEmailConfig()
	if err != nil {
		return err
	}
	payload := map[string]any{
		"from":    formatVerificationEmailFrom(config),
		"to":      []string{email},
		"subject": "Fund Tracking 邮箱验证码",
		"html":    buildVerificationCodeEmailHTML(code),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	ctxTimeout := 10 * time.Second
	client := &http.Client{Timeout: ctxTimeout}
	req, err := http.NewRequest(http.MethodPost, "https://api.resend.com/emails", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+config.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("verification email provider returned HTTP %d", resp.StatusCode)
	}

	var result verificationEmailResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode verification email response")
	}
	if strings.TrimSpace(result.ID) == "" {
		return fmt.Errorf("verification email response missing id")
	}
	return nil
}

func formatVerificationEmailFrom(config verificationEmailConfig) string {
	if config.FromName == "" {
		return config.From
	}
	return fmt.Sprintf("%s <%s>", config.FromName, config.From)
}

func buildVerificationCodeEmailHTML(code string) string {
	return fmt.Sprintf(`
<div style="font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto; padding: 20px;">
  <h2 style="color: #1f2937;">Fund Tracking 邮箱验证码</h2>
  <p style="color: #4b5563;">你的验证码是：</p>
  <div style="font-size: 32px; font-weight: 700; letter-spacing: 6px; padding: 16px; background: #f3f4f6; text-align: center; border-radius: 8px;">%s</div>
  <p style="color: #6b7280;">验证码 10 分钟内有效。</p>
  <p style="color: #6b7280;">如果不是你本人操作，可以忽略这封邮件。</p>
</div>
`, code)
}
