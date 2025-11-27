package config

import "testing"

func TestTLSConfigInsecure(t *testing.T) {
	c := &Config{InsecureSkipVerify: true}
	tls := c.TLSConfig()

	if !tls.InsecureSkipVerify {
		t.Fatalf("expected InsecureSkipVerify=true")
	}
}
