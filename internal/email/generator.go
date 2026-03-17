package email

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	fhttp "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
	"github.com/brianvoe/gofakeit/v7"

	"github.com/exagen-creator/exagen/internal/util"
)

var (
	blacklistedDomains sync.Map
	blacklistMutex     sync.Mutex
)

func init() {
	data, err := os.ReadFile("blacklist.json")
	if err != nil {
		return
	}

	var domains []string
	if err := json.Unmarshal(data, &domains); err != nil {
		return
	}

	for _, domain := range domains {
		blacklistedDomains.Store(domain, true)
	}
}

func saveBlacklist() {
	blacklistMutex.Lock()
	defer blacklistMutex.Unlock()

	var domains []string
	blacklistedDomains.Range(func(key, value any) bool {
		if domain, ok := key.(string); ok {
			domains = append(domains, domain)
		}
		return true
	})

	data, err := json.MarshalIndent(domains, "", "  ")
	if err != nil {
		return
	}

	_ = os.WriteFile("blacklist.json", data, 0644)
}

// AddBlacklistDomain adds a domain to the global blacklist.
func AddBlacklistDomain(domain string) {
	blacklistedDomains.Store(domain, true)
	saveBlacklist()
}

// CreateTempEmail fetches a new temp email using a random profile and gofakeit names.
func CreateTempEmail(defaultDomain string) (string, error) {
	if defaultDomain != "" {
		firstName := gofakeit.FirstName()
		lastName := gofakeit.LastName()
		email := strings.ToLower(firstName+lastName+util.RandStr(5)) + "@" + defaultDomain
		return email, nil
	}

	options := []tls_client.HttpClientOption{
		tls_client.WithClientProfile(profiles.Chrome_131),
	}

	client, err := tls_client.NewHttpClient(tls_client.NewNoopLogger(), options...)
	if err != nil {
		return "", fmt.Errorf("failed to create tls client: %w", err)
	}

	req, err := fhttp.NewRequest("GET", "https://generator.email/", nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch generator.email: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("generator.email returned status: %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to parse HTML: %w", err)
	}

	domains := []string{"smartmail.de", "enayu.com", "crazymailing.com"}
	doc.Find(".e7m.tt-suggestions div > p").Each(func(i int, s *goquery.Selection) {
		domain := strings.TrimSpace(s.Text())
		if domain != "" {
			if _, blacklisted := blacklistedDomains.Load(domain); !blacklisted {
				domains = append(domains, domain)
			}
		}
	})

	if len(domains) == 0 {
		return "", fmt.Errorf("all available domains are blacklisted")
	}

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	randomDomain := domains[r.Intn(len(domains))]

	firstName := gofakeit.FirstName()
	lastName := gofakeit.LastName()
	email := strings.ToLower(firstName+lastName+util.RandStr(5)) + "@" + randomDomain

	return email, nil
}

// GetVerificationCode polls generator.email for the OTP using a custom cookie.
func GetVerificationCode(emailAddr string, maxRetries int, delay time.Duration) (string, error) {
	parts := strings.Split(emailAddr, "@")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid email format: %s", emailAddr)
	}
	username := parts[0]
	domain := parts[1]

	otpRegex := regexp.MustCompile(`\d{6}`)

	for i := 0; i < maxRetries; i++ {
		options := []tls_client.HttpClientOption{
			tls_client.WithClientProfile(profiles.Chrome_131),
		}

		client, err := tls_client.NewHttpClient(tls_client.NewNoopLogger(), options...)
		if err != nil {
			return "", fmt.Errorf("failed to create tls client: %w", err)
		}

		url := fmt.Sprintf("https://generator.email/%s/%s", domain, username)
		req, err := fhttp.NewRequest("GET", url, nil)
		if err != nil {
			return "", fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Cookie", fmt.Sprintf("surl=%s/%s", domain, username))

		resp, err := client.Do(req)
		if err != nil {
			time.Sleep(delay)
			continue
		}

		if resp.StatusCode != 200 {
			resp.Body.Close()
			time.Sleep(delay)
			continue
		}

		doc, err := goquery.NewDocumentFromReader(resp.Body)
		resp.Body.Close()
		if err != nil {
			time.Sleep(delay)
			continue
		}

		otp := ""

		// Search the email body content for OTP
		doc.Find("div.e7m.mess_bodiyy").EachWithBreak(func(i int, s *goquery.Selection) bool {
			text := s.Text()
			matches := otpRegex.FindAllString(text, -1)
			for _, code := range matches {
				if code == "177010" {
					continue
				}
				otp = code
				return false
			}
			return true
		})

		// Fallback: search subject line divs
		if otp == "" {
			doc.Find("#email-table > div.e7m.list-group-item.list-group-item-info > div.e7m.subj_div_45g45gg").EachWithBreak(func(i int, s *goquery.Selection) bool {
				text := s.Text()
				matches := otpRegex.FindStringSubmatch(text)
				if len(matches) > 0 {
					code := matches[0]
					if code == "177010" {
						return true
					}
					otp = code
					return false
				}
				return true
			})
		}

		if otp != "" {
			return otp, nil
		}

		time.Sleep(delay)
	}

	return "", fmt.Errorf("failed to get verification code after %d retries", maxRetries)
}
