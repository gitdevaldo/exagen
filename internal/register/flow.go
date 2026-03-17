package register

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/url"
	"strings"
	"time"

	http "github.com/bogdanfinn/fhttp"

	"github.com/exagen-creator/exagen/internal/email"
)

// callbackURL is the URL that auth.exa.ai redirects to after successful authentication.
const callbackURL = dashboardURL + "/"

// visitHomepage visits exa.ai to initialize the session and cookies.
func (c *Client) visitHomepage() error {
	var resp *http.Response
	var err error
	for retry := 0; retry < 3; retry++ {
		req, _ := http.NewRequest("GET", baseURL+"/", nil)
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
		req.Header.Set("Upgrade-Insecure-Requests", "1")

		resp, err = c.do(req)
		if err != nil {
			return err
		}

		c.log("Visit Homepage", resp.StatusCode)

		if resp.StatusCode == 200 || resp.StatusCode == 302 || resp.StatusCode == 307 {
			resp.Body.Close()
			return nil
		}
		resp.Body.Close()
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("failed to visit homepage after 3 retries (status: %d)", resp.StatusCode)
}

// visitAuthPage visits auth.exa.ai sign-in page (like clicking "Sign In" on exa.ai).
func (c *Client) visitAuthPage() error {
	params := url.Values{}
	params.Set("callbackUrl", callbackURL)

	fullURL := authURL + "/?" + params.Encode()

	req, _ := http.NewRequest("GET", fullURL, nil)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Set("Referer", baseURL+"/")
	req.Header.Set("Upgrade-Insecure-Requests", "1")

	resp, err := c.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	c.log("Visit Auth Page", resp.StatusCode)

	if resp.StatusCode != 200 {
		return fmt.Errorf("failed to visit auth page (status: %d)", resp.StatusCode)
	}
	return nil
}

// getCSRF retrieves the CSRF token from auth.exa.ai.
func (c *Client) getCSRF() (string, error) {
	req, _ := http.NewRequest("GET", authURL+"/api/auth/csrf", nil)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Referer", authURL+"/")

	resp, err := c.do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var data struct {
		CSRFToken string `json:"csrfToken"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return "", err
	}

	c.log("Get CSRF", resp.StatusCode)
	if data.CSRFToken == "" {
		return "", fmt.Errorf("csrf token not found")
	}
	return data.CSRFToken, nil
}

// signinEmail submits the email to auth.exa.ai to trigger an OTP email.
func (c *Client) signinEmail(emailAddr, csrf string) error {
	formData := url.Values{}
	formData.Set("email", emailAddr)
	formData.Set("redirect", "false")
	formData.Set("callbackUrl", callbackURL)
	formData.Set("csrfToken", csrf)
	formData.Set("json", "true")

	req, _ := http.NewRequest("POST", authURL+"/api/auth/signin/email", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Referer", authURL+"/?callbackUrl="+url.QueryEscape(callbackURL))
	req.Header.Set("Origin", authURL)

	resp, err := c.do(req)
	if err != nil {
		return fmt.Errorf("signin email request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	c.log("Signin Email", resp.StatusCode)

	if resp.StatusCode != 200 {
		return fmt.Errorf("signin email failed (status %d): %s", resp.StatusCode, truncateBody(string(body), 200))
	}

	// Check response body — Exa returns 200 even when email is rejected
	var data struct {
		URL string `json:"url"`
	}
	json.Unmarshal(body, &data)

	if strings.Contains(data.URL, "error=EmailSignin") {
		return fmt.Errorf("email rejected by Exa (unsupported domain): %s", emailAddr)
	}

	return nil
}

// verifyOTP submits the OTP code to auth.exa.ai for verification.
func (c *Client) verifyOTP(emailAddr, code string) error {
	payload := map[string]string{
		"email": emailAddr,
		"otp":   code,
	}
	jsonPayload, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", authURL+"/api/verify-otp", strings.NewReader(string(jsonPayload)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Referer", authURL+"/?callbackUrl="+url.QueryEscape(callbackURL))
	req.Header.Set("Origin", authURL)

	resp, err := c.do(req)
	if err != nil {
		return fmt.Errorf("verify otp request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	c.log(fmt.Sprintf("Verify OTP [%s]", code), resp.StatusCode)

	if resp.StatusCode != 200 {
		return fmt.Errorf("verify otp failed (status %d): %s", resp.StatusCode, truncateBody(string(body), 200))
	}

	return nil
}

// RunRegister performs the exa.ai registration flow: signup + OTP verification.
func (c *Client) RunRegister(emailAddr string) error {
	c.print("Starting registration flow...")

	// Step 1: Visit exa.ai homepage to initialize session
	if err := c.visitHomepage(); err != nil {
		return err
	}
	c.randomDelay(0.3, 0.8)

	// Step 2: Visit auth.exa.ai sign-in page (like clicking "Sign In")
	if err := c.visitAuthPage(); err != nil {
		return err
	}
	c.randomDelay(0.3, 0.8)

	// Step 3: Get CSRF token
	csrf, err := c.getCSRF()
	if err != nil {
		return err
	}
	c.randomDelay(0.2, 0.5)

	// Step 4: Submit email to trigger OTP
	if err := c.signinEmail(emailAddr, csrf); err != nil {
		return err
	}
	c.randomDelay(0.5, 1.0)

	// Step 5: Poll for OTP from email
	otpCode, err := email.GetVerificationCode(emailAddr, 20, 3*time.Second)
	if err != nil {
		return err
	}
	c.print(fmt.Sprintf("Got OTP: %s", otpCode))

	// Step 6: Verify OTP
	c.randomDelay(0.3, 0.8)
	if err := c.verifyOTP(emailAddr, otpCode); err != nil {
		// Retry once on failure
		c.print("OTP verification failed, retrying...")
		c.randomDelay(2.0, 4.0)

		otpCode, err = email.GetVerificationCode(emailAddr, 10, 3*time.Second)
		if err != nil {
			return err
		}

		c.randomDelay(0.3, 0.8)
		if err := c.verifyOTP(emailAddr, otpCode); err != nil {
			return fmt.Errorf("otp verification failed after retry: %w", err)
		}
	}

	return nil
}

func (c *Client) randomDelay(low, high float64) {
	delay := low + rand.Float64()*(high-low)
	time.Sleep(time.Duration(delay * float64(time.Second)))
}

func truncateBody(body string, maxLen int) string {
	if len(body) > maxLen {
		return body[:maxLen] + "..."
	}
	return body
}
