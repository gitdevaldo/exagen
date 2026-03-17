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
	{146, "chrome146", 7876, 0, 200, `"Chromium";v="146", "Not-A.Brand";v="24", "Google Chrome";v="146"`},
	{144, "chrome144", 7825, 0, 200, `"Chromium";v="144", "Not-A.Brand";v="24", "Google Chrome";v="144"`},
	{133, "chrome133", 6943, 0, 200, `"Chromium";v="133", "Not-A.Brand";v="24", "Google Chrome";v="133"`},
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
	case "chrome146":
		return profiles.Chrome_146
	case "chrome144":
		return profiles.Chrome_144
	case "chrome133":
		return profiles.Chrome_133
	case "chrome131":
		return profiles.Chrome_131
	default:
		return profiles.Chrome_146
	}
}
