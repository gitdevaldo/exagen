package email

import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	fhttp "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
)

const mailAPIBase = "https://api.mail.tm"

// Inbox holds the state for a mail.tm temporary inbox.
type Inbox struct {
	Address  string
	Password string
	Token    string
	client   tls_client.HttpClient
}

// newTLSClient creates a TLS client for mail.tm API calls.
func newTLSClient() (tls_client.HttpClient, error) {
	options := []tls_client.HttpClientOption{
		tls_client.WithClientProfile(profiles.Chrome_131),
	}
	return tls_client.NewHttpClient(tls_client.NewNoopLogger(), options...)
}

// GetAvailableDomain fetches a usable domain from mail.tm.
func GetAvailableDomain() (string, error) {
	client, err := newTLSClient()
	if err != nil {
		return "", fmt.Errorf("failed to create tls client: %w", err)
	}

	req, _ := fhttp.NewRequest("GET", mailAPIBase+"/domains", nil)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch domains: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	// mail.tm returns a plain JSON array
	var domains []struct {
		Domain   string `json:"domain"`
		IsActive bool   `json:"isActive"`
	}
	json.Unmarshal(body, &domains)

	for _, d := range domains {
		if d.IsActive {
			return d.Domain, nil
		}
	}

	return "", fmt.Errorf("no active domains available from mail.tm")
}

// CreateTempEmail creates a new mail.tm inbox and returns the Inbox handle.
func CreateTempEmail(defaultDomain string) (*Inbox, error) {
	client, err := newTLSClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create tls client: %w", err)
	}

	domain := defaultDomain
	if domain == "" {
		d, err := GetAvailableDomain()
		if err != nil {
			return nil, err
		}
		domain = d
	}

	address := fmt.Sprintf("exa%d@%s", time.Now().UnixNano(), domain)
	password := "ExaReg2026!"

	// Create account
	payload, _ := json.Marshal(map[string]string{
		"address":  address,
		"password": password,
	})

	req, _ := fhttp.NewRequest("POST", mailAPIBase+"/accounts", strings.NewReader(string(payload)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to create mail.tm account: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 201 {
		return nil, fmt.Errorf("mail.tm account creation failed (status %d): %s", resp.StatusCode, string(body))
	}

	// Get auth token
	tokenPayload, _ := json.Marshal(map[string]string{
		"address":  address,
		"password": password,
	})

	reqToken, _ := fhttp.NewRequest("POST", mailAPIBase+"/token", strings.NewReader(string(tokenPayload)))
	reqToken.Header.Set("Content-Type", "application/json")
	reqToken.Header.Set("Accept", "application/json")

	respToken, err := client.Do(reqToken)
	if err != nil {
		return nil, fmt.Errorf("failed to get mail.tm token: %w", err)
	}
	defer respToken.Body.Close()

	tokenBody, _ := io.ReadAll(respToken.Body)
	if respToken.StatusCode != 200 {
		return nil, fmt.Errorf("mail.tm token request failed (status %d): %s", respToken.StatusCode, string(tokenBody))
	}

	var tokenResp struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(tokenBody, &tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse mail.tm token: %w", err)
	}

	return &Inbox{
		Address:  address,
		Password: password,
		Token:    tokenResp.Token,
		client:   client,
	}, nil
}

// GetVerificationCode polls the mail.tm inbox for an OTP code from Exa.
func (inbox *Inbox) GetVerificationCode(maxRetries int, delay time.Duration) (string, error) {
	otpRegex := regexp.MustCompile(`\d{6}`)

	for i := 0; i < maxRetries; i++ {
		time.Sleep(delay)

		req, _ := fhttp.NewRequest("GET", mailAPIBase+"/messages", nil)
		req.Header.Set("Authorization", "Bearer "+inbox.Token)
		req.Header.Set("Accept", "application/json")

		resp, err := inbox.client.Do(req)
		if err != nil {
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		// mail.tm returns a plain JSON array of messages
		var messages []struct {
			ID    string `json:"id"`
			Intro string `json:"intro"`
			From  struct {
				Address string `json:"address"`
			} `json:"from"`
		}
		json.Unmarshal(body, &messages)

		for _, msg := range messages {
			// Look for OTP in the intro field ("Your verification code for Exa is: 123456")
			matches := otpRegex.FindStringSubmatch(msg.Intro)
			if len(matches) > 0 {
				return matches[0], nil
			}

			// If not in intro, fetch full message and search text body
			code, err := inbox.fetchOTPFromMessage(msg.ID, otpRegex)
			if err == nil && code != "" {
				return code, nil
			}
		}
	}

	return "", fmt.Errorf("failed to get verification code after %d retries", maxRetries)
}

// fetchOTPFromMessage fetches a full message and extracts the OTP code.
func (inbox *Inbox) fetchOTPFromMessage(messageID string, otpRegex *regexp.Regexp) (string, error) {
	req, _ := fhttp.NewRequest("GET", mailAPIBase+"/messages/"+messageID, nil)
	req.Header.Set("Authorization", "Bearer "+inbox.Token)
	req.Header.Set("Accept", "application/json")

	resp, err := inbox.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var msg struct {
		Text string   `json:"text"`
		HTML []string `json:"html"`
	}
	json.Unmarshal(body, &msg)

	// Search text body
	if matches := otpRegex.FindStringSubmatch(msg.Text); len(matches) > 0 {
		return matches[0], nil
	}

	// Search HTML body
	for _, html := range msg.HTML {
		if matches := otpRegex.FindStringSubmatch(html); len(matches) > 0 {
			return matches[0], nil
		}
	}

	return "", fmt.Errorf("no OTP found in message %s", messageID)
}
