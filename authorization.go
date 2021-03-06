package acme

import "net/http"

// FetchAuthorization fetches an authorization from an authorization url provided in an order.
// More details: https://tools.ietf.org/html/draft-ietf-acme-acme-10#section-7.5
func (c AcmeClient) FetchAuthorization(account AcmeAccount, authURL string) (AcmeAuthorization, error) {
	authResp := AcmeAuthorization{}
	_, err := c.get(authURL, &authResp, http.StatusOK)
	if err != nil {
		return authResp, err
	}

	for i := 0; i < len(authResp.Challenges); i++ {
		if authResp.Challenges[i].KeyAuthorization == "" {
			authResp.Challenges[i].KeyAuthorization = authResp.Challenges[i].Token + "." + account.Thumbprint
		}
	}

	authResp.ChallengeMap = map[string]AcmeChallenge{}
	authResp.ChallengeTypes = []string{}
	for _, c := range authResp.Challenges {
		authResp.ChallengeMap[c.Type] = c
		authResp.ChallengeTypes = append(authResp.ChallengeTypes, c.Type)
	}

	return authResp, nil
}

// DeactivateAuthorization deactivate a provided authorization url from an order.
// More details: https://tools.ietf.org/html/draft-ietf-acme-acme-10#section-7.5.2
func (c AcmeClient) DeactivateAuthorization(account AcmeAccount, authURL string) (AcmeAuthorization, error) {
	deactivateReq := struct {
		Status string `json:"status"`
	}{
		Status: "deactivated",
	}
	deactivateResp := AcmeAuthorization{}

	if _, err := c.post(authURL, account.Url, account.PrivateKey, deactivateReq, &deactivateResp, http.StatusOK); err != nil {
		return deactivateResp, err
	}

	return deactivateResp, nil
}
