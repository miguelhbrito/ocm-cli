package utils

import (
	"crypto/rand"
	"crypto/x509"
	"fmt"
	"log"
	"math/big"
	"net/url"
	"os"
	"regexp"
	"time"
)

// the following regex defines four different patterns:
// first pattern is to validate IPv4 address
// second,is for IPv4 CIDR range validation
// third pattern is to validate domains
// and the fourth petterrn is to be able to remove the existing no-proxy value by typing empty string ("").
// nolint
var UserNoProxyRE = regexp.MustCompile(`^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$|^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])(\/(3[0-2]|[1-2][0-9]|[0-9]))$|^(.?[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z0-9][a-z0-9-]{0,61}[a-z0-9]$|^""$`)

var TimeoutError = fmt.Errorf("Timeout occurred")

func ValidateHTTPProxy(val interface{}) error {
	if httpProxy, ok := val.(string); ok {
		if httpProxy == "" {
			return nil
		}
		url, err := url.ParseRequestURI(httpProxy)
		if err != nil {
			return fmt.Errorf("Invalid 'proxy.http_proxy' attribute '%s'", httpProxy)
		}
		if url.Scheme != "http" {
			return fmt.Errorf("%s", "Expected http-proxy to have an http:// scheme")
		}
		return nil
	}
	return fmt.Errorf("can only validate strings, got %v", val)
}

// IsURL validates whether the given value is a valid URL
func IsURL(val interface{}) error {
	if val == nil {
		return nil
	}
	if s, ok := val.(string); ok {
		if s == "" {
			return nil
		}
		_, err := url.ParseRequestURI(fmt.Sprintf("%v", val))
		return err
	}
	return fmt.Errorf("can only validate strings, got %v", val)
}

func ValidateAdditionalTrustBundle(val interface{}) error {
	if additionalTrustBundleFile, ok := val.(string); ok {
		if additionalTrustBundleFile == "" {
			return nil
		}
		cert, err := os.ReadFile(additionalTrustBundleFile)
		if err != nil {
			return err
		}
		additionalTrustBundle := string(cert)
		if additionalTrustBundle == "" {
			return fmt.Errorf("%s", "Additional trust bundle file is empty")
		}
		additionalTrustBundleBytes := []byte(additionalTrustBundle)
		if !x509.NewCertPool().AppendCertsFromPEM(additionalTrustBundleBytes) {
			return fmt.Errorf("%s", "Failed to parse additional trust bundle")
		}
		return nil
	}
	return fmt.Errorf("can only validate strings, got %v", val)
}

func MatchNoPorxyRE(noProxyValues []string) error {
	for _, v := range noProxyValues {
		if !UserNoProxyRE.MatchString(v) {
			return fmt.Errorf("expected a valid user no-proxy value: '%s' matching %s", v,
				UserNoProxyRE.String())
		}
	}
	return nil
}

func HasDuplicates(valSlice []string) (string, bool) {
	visited := make(map[string]bool)
	for _, v := range valSlice {
		if visited[v] {
			return v, true
		}
		visited[v] = true
	}
	return "", false
}

// Tries the passed in function multiple times within a timeout window,
// sleeping with backoff in between calls.
// When non-nil logger is passed as a parameter, log messages about the retry
// logic will be sent.
func RetryWithBackoffandTimeout(
	f func() (bool, error),
	timeoutSeconds int,
	log *log.Logger,
) error {
	var (
		backoffSeconds = int(1)
		timeoutTimer   = time.NewTimer(time.Duration(timeoutSeconds) * time.Second)
	)
	for {
		retry, callErr := f()
		if !retry {
			return callErr
		}
		select {
		case <-timeoutTimer.C:
			return TimeoutError
		default:
			if log != nil {
				log.Printf("Trying again in %d seconds...", backoffSeconds)
			}
			// A jitter is introduced to avoid the "thundering herd" problem.
			jitter := time.Duration(randIntN(250)) * time.Millisecond
			backoffTimer := time.NewTimer(jitter + time.Duration(backoffSeconds)*time.Second)
			backoffSeconds = backoffSeconds * 2
			select {
			case <-timeoutTimer.C:
				return TimeoutError
			case <-backoffTimer.C:
				continue
			}
		}
	}
}

// Returns a random int between (0, N]. Panics on failure.
func randIntN(maxN int) int {
	outN, err := rand.Int(rand.Reader, big.NewInt(int64(maxN)))
	if err != nil {
		// This is highly unlikely to happen
		panic(fmt.Sprintf("Failed to generate random number: %s", err.Error()))
	}
	return int(outN.Int64())
}
