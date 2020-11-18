package cookies

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/avct/uasurfer"
	"github.com/oauth2-proxy/oauth2-proxy/pkg/apis/options"
	"github.com/oauth2-proxy/oauth2-proxy/pkg/logger"
	"github.com/oauth2-proxy/oauth2-proxy/pkg/util"
)

// MakeCookie constructs a cookie from the given parameters,
// discovering the domain from the request if not specified.
func MakeCookie(req *http.Request, name string, value string, path string, domain string, httpOnly bool, secure bool, expiration time.Duration, now time.Time, sameSite http.SameSite) *http.Cookie {
	if domain != "" {
		host := util.GetRequestHost(req)
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
		if !strings.HasSuffix(host, domain) {
			logger.Errorf("Warning: request host is %q but using configured cookie domain of %q", host, domain)
		}
	}

	// Adapt the cookie in case of "SameSite=None" Apple issue
	sameSite = AdaptSameSiteIfAppleIssue(req, sameSite)

	return &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     path,
		Domain:   domain,
		HttpOnly: httpOnly,
		Secure:   secure,
		Expires:  now.Add(expiration),
		SameSite: sameSite,
	}
}

// MakeCookieFromOptions constructs a cookie based on the given *options.CookieOptions,
// value and creation time
func MakeCookieFromOptions(req *http.Request, name string, value string, cookieOpts *options.Cookie, expiration time.Duration, now time.Time) *http.Cookie {
	domain := GetCookieDomain(req, cookieOpts.Domains)

	if domain != "" {
		return MakeCookie(req, name, value, cookieOpts.Path, domain, cookieOpts.HTTPOnly, cookieOpts.Secure, expiration, now, ParseSameSite(cookieOpts.SameSite))
	}
	// If nothing matches, create the cookie with the shortest domain
	defaultDomain := ""
	if len(cookieOpts.Domains) > 0 {
		logger.Errorf("Warning: request host %q did not match any of the specific cookie domains of %q", util.GetRequestHost(req), strings.Join(cookieOpts.Domains, ","))
		defaultDomain = cookieOpts.Domains[len(cookieOpts.Domains)-1]
	}
	return MakeCookie(req, name, value, cookieOpts.Path, defaultDomain, cookieOpts.HTTPOnly, cookieOpts.Secure, expiration, now, ParseSameSite(cookieOpts.SameSite))
}

// GetCookieDomain returns the correct cookie domain given a list of domains
// by checking the X-Fowarded-Host and host header of an an http request
func GetCookieDomain(req *http.Request, cookieDomains []string) string {
	host := util.GetRequestHost(req)
	for _, domain := range cookieDomains {
		if strings.HasSuffix(host, domain) {
			return domain
		}
	}
	return ""
}

// Parse a valid http.SameSite value from a user supplied string for use of making cookies.
func ParseSameSite(v string) http.SameSite {
	switch v {
	case "lax":
		return http.SameSiteLaxMode
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	case "":
		return http.SameSiteDefaultMode
	default:
		panic(fmt.Sprintf("Invalid value for SameSite: %s", v))
	}
}

// AdaptSameSiteIfAppleIssue adapts the SameSite property in case of the "SameSite=None" Apple issue
// by checking the User-Agent of an http request
// Ref: https://github.com/oauth2-proxy/oauth2-proxy/issues/830
func AdaptSameSiteIfAppleIssue(req *http.Request, sameSite http.SameSite) http.SameSite {
	if sameSite == http.SameSiteNoneMode {
		userAgent := uasurfer.Parse(req.UserAgent())

		// Versions from which this issue has been solved
		macOSXVersionFixingIssue := uasurfer.Version{
			Major: 10,
			Minor: 13,
			Patch: 0,
		}
		iOSVersionFixingIssue := uasurfer.Version{
			Major: 14,
			Minor: 0,
			Patch: 0,
		}

		// If the user agent is concerned by the issue, provide "SameSite=Lax" instead of "None" to allow some CORS requests within the same domain
		if (userAgent.OS.Name == uasurfer.OSMacOSX && userAgent.OS.Version.Less(macOSXVersionFixingIssue)) || (userAgent.OS.Name == uasurfer.OSiOS && userAgent.OS.Version.Less(iOSVersionFixingIssue)) {
			sameSite = http.SameSiteLaxMode
		}
	}

	return sameSite
}
