package acme

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"regexp"

	"strings"

	"bytes"
	"crypto"
)

const (
	userAgentString = "eggsampler-acme/1.0 Go-http-client/1.1"
)

// NewClient creates a new acme client given a valid directory url.
// More details: https://tools.ietf.org/html/draft-ietf-acme-acme-10#section-7.1.1
func NewClient(directoryURL string, options ...AcmeOptionFunc) (AcmeClient, error) {
	httpClient := http.DefaultClient

	ns := &nonceStack{
		client: httpClient,
	}

	acmeClient := AcmeClient{
		httpClient: httpClient,
		nonces:     ns,
	}

	for _, opt := range options {
		if err := opt(acmeClient); err != nil {
			return acmeClient, fmt.Errorf("acme: error setting option: %v", err)
		}
	}

	if _, err := acmeClient.get(directoryURL, &acmeClient.Directory, http.StatusOK); err != nil {
		return acmeClient, err
	}

	acmeClient.Directory.Url = directoryURL
	ns.newNonceURL = acmeClient.Directory.NewNonce

	return acmeClient, nil
}

// Helper function to get the poll interval and poll timeout, defaulting if 0
func (c AcmeClient) getPollingDurations() (time.Duration, time.Duration) {
	pollInterval := c.PollInterval
	if pollInterval == 0 {
		pollInterval = 500 * time.Millisecond
	}
	pollTimeout := c.PollTimeout
	if pollTimeout == 0 {
		pollTimeout = 30 * time.Second
	}
	return pollInterval, pollTimeout
}

// Helper function to have a central point for performing http requests.
// Stores any returned nonces in the stack.
func (c AcmeClient) do(req *http.Request) (*http.Response, error) {
	// More details: https://tools.ietf.org/html/draft-ietf-acme-acme-10#section-6.1
	// identifier for this client, as well as the default go user agent
	req.Header.Set("User-Agent", userAgentString)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return resp, err
	}

	c.nonces.push(resp.Header.Get("Replay-Nonce"))

	return resp, nil
}

// Helper function to perform an http get request and read the body.
func (c AcmeClient) getRaw(url string, expectedStatus ...int) (*http.Response, []byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("acme: error creating request: %v", err)
	}

	resp, err := c.do(req)
	if err != nil {
		return resp, nil, fmt.Errorf("acme: error fetching response: %v", err)
	}
	defer resp.Body.Close()

	if err := checkError(resp, expectedStatus...); err != nil {
		return resp, nil, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return resp, body, fmt.Errorf("acme: error reading response body: %v", err)
	}

	return resp, body, nil
}

// Helper function for performing a http get on an acme resource.
func (c AcmeClient) get(url string, out interface{}, expectedStatus ...int) (*http.Response, error) {
	resp, body, err := c.getRaw(url, expectedStatus...)
	if err != nil {
		return resp, err
	}

	if len(body) > 0 && out != nil {
		if err := json.Unmarshal(body, out); err != nil {
			return resp, fmt.Errorf("acme: error parsing response body: %v", err)
		}
	}

	return resp, nil
}

// Helper function to perform an http post request and read the body.
// Will attempt to retry if error is badNonce
func (c AcmeClient) postRaw(isRetry bool, requestURL, keyID string, privateKey crypto.Signer, payload interface{}, out interface{}, expectedStatus []int) (*http.Response, []byte, error) {
	nonce, err := c.nonces.Nonce()
	if err != nil {
		return nil, nil, err
	}

	data, err := jwsEncodeJSON(payload, privateKey, requestURL, keyID, nonce)
	if err != nil {
		return nil, nil, fmt.Errorf("acme: error encoding json payload: %v", err)
	}

	req, err := http.NewRequest("POST", requestURL, bytes.NewReader(data))
	if err != nil {
		return nil, nil, fmt.Errorf("acme: error creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/jose+json")

	resp, err := c.do(req)
	if err != nil {
		return resp, nil, fmt.Errorf("acme: error sending request: %v", err)
	}
	defer resp.Body.Close()

	if err := checkError(resp, expectedStatus...); err != nil {
		if isRetry {
			// don't attempt to retry if this attempt is the retry
			return resp, nil, err
		}
		acmeErr, ok := err.(AcmeError)
		if !ok {
			// don't retry for an error we don't know about
			return resp, nil, err
		}
		if !strings.HasSuffix(acmeErr.Type, ":badNonce") {
			// only retry on a badNonce error
			return resp, nil, err
		}
		// perform the retry
		return c.postRaw(true, requestURL, keyID, privateKey, payload, out, expectedStatus)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return resp, body, fmt.Errorf("acme: error reading response body: %v", err)
	}

	return resp, body, nil
}

// Helper function for performing a http post to an acme resource.
func (c AcmeClient) post(requestURL, keyID string, privateKey crypto.Signer, payload interface{}, out interface{}, expectedStatus ...int) (*http.Response, error) {
	resp, body, err := c.postRaw(false, requestURL, keyID, privateKey, payload, out, expectedStatus)
	if err != nil {
		return resp, err
	}

	if len(body) > 0 && out != nil {
		if err := json.Unmarshal(body, out); err != nil {
			return resp, fmt.Errorf("acme: error parsing response: %v - %s", err, string(body))
		}
	}

	return resp, nil
}

var regLink = regexp.MustCompile(`<(.+?)>;\s*rel="(.+?)"`)

// Fetches a http Link header from a http response
func fetchLink(resp *http.Response, wantedLink string) string {
	if resp == nil {
		return ""
	}
	linkHeader := resp.Header["Link"]
	if len(linkHeader) == 0 {
		return ""
	}
	for _, l := range linkHeader {
		matches := regLink.FindAllStringSubmatch(l, -1)
		for _, m := range matches {
			if len(m) != 3 {
				continue
			}
			if m[2] == wantedLink {
				return m[1]
			}
		}
	}
	return ""
}
