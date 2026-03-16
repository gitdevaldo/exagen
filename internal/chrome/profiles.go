package chrome

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/bogdanfinn/tls-client/profiles"
)

// Profile describes a Chrome browser version used for TLS fingerprinting.
type Profile struct {
	Major       int
	Impersonate string
	Build       int
	PatchMin    int
	PatchMax    int
	SecChUA     string
}

var chromeProfiles = []Profile{
	{131, "chrome131", 6778, 0, 300, "\"Chromium\";v=\"131\", \"Google Chrome\";v=\"131\", \"Not_A Brand\";v=\"24\""},
}

// RandomChromeVersion selects a random Chrome version profile and returns
// the profile, full version string, and user-agent.
func RandomChromeVersion() (Profile, string, string) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	profile := chromeProfiles[r.Intn(len(chromeProfiles))]
	patch := r.Intn(profile.PatchMax-profile.PatchMin+1) + profile.PatchMin
	fullVersion := fmt.Sprintf("%d.0.%d.%d", profile.Major, profile.Build, patch)
	userAgent := fmt.Sprintf("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%s Safari/537.36", fullVersion)
	return profile, fullVersion, userAgent
}

// MapToTLSProfile maps an impersonate string to a tls-client ClientProfile.
func MapToTLSProfile(impersonate string) profiles.ClientProfile {
	switch impersonate {
	case "chrome131":
		return profiles.Chrome_131
	case "chrome133":
		return profiles.Chrome_133
	default:
		return profiles.Chrome_133
	}
}
