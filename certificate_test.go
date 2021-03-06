package acme

import "testing"

func TestAcmeClient_FetchCertificates(t *testing.T) {
	domains := []string{randString() + ".com"}
	_, order, _ := makeOrderFinal(t, domains)
	if order.Certificate == "" {
		t.Fatalf("no certificate: %+v", order)
	}
	certs, err := testClient.FetchCertificates(order.Certificate)
	if err != nil {
		t.Fatalf("expeceted no error, got: %v", err)
	}
	if len(certs) == 0 {
		t.Fatal("no certs returned")
	}
	for _, d := range domains {
		if err := certs[0].VerifyHostname(d); err != nil {
			t.Fatalf("cert not verified for %s: %v - %+v", d, err, certs[0])
		}
	}
}

func TestAcmeClient_RevokeCertificate(t *testing.T) {
	// test revoking cert with cert key
	domains := []string{randString() + ".com"}
	account, order, privKey := makeOrderFinal(t, domains)
	if order.Certificate == "" {
		t.Fatalf("no certificate: %+v", order)
	}
	certs, err := testClient.FetchCertificates(order.Certificate)
	if err != nil {
		t.Fatalf("expeceted no error, got: %v", err)
	}
	if err := testClient.RevokeCertificate(account, certs[0], privKey, ReasonUnspecified); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestAcmeClient_RevokeCertificate2(t *testing.T) {
	// test revoking cert with account key
	domains := []string{randString() + ".com"}
	account, order, _ := makeOrderFinal(t, domains)
	if order.Certificate == "" {
		t.Fatalf("no certificate: %+v", order)
	}
	certs, err := testClient.FetchCertificates(order.Certificate)
	if err != nil {
		t.Fatalf("expeceted no error, got: %v", err)
	}
	if err := testClient.RevokeCertificate(account, certs[0], account.PrivateKey, ReasonUnspecified); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}
