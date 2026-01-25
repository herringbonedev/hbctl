package secrets

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
)

// GenerateServiceKeyPair generates an RSA keypair for service JWT signing.
// Returns PEM-encoded public and private keys.
func GenerateServiceKeyPair(bits int) (publicPEM string, privatePEM string, err error) {
	if bits < 2048 {
		return "", "", errors.New("RSA key size must be >= 2048 bits")
	}

	key, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return "", "", err
	}

	// Private key (PKCS#1)
	privDER := x509.MarshalPKCS1PrivateKey(key)
	privBlock := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privDER,
	}
	privatePEMBytes := pem.EncodeToMemory(privBlock)

	// Public key (PKIX / X.509)
	pubDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return "", "", err
	}

	pubBlock := &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubDER,
	}
	publicPEMBytes := pem.EncodeToMemory(pubBlock)

	return string(publicPEMBytes), string(privatePEMBytes), nil
}

// ValidateServicePrivateKey checks if a PEM private key is valid RSA.
func ValidateServicePrivateKey(pemData string) error {
	block, _ := pem.Decode([]byte(pemData))
	if block == nil {
		return errors.New("invalid PEM data")
	}

	_, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	return err
}

// ValidateServicePublicKey checks if a PEM public key is valid RSA.
func ValidateServicePublicKey(pemData string) error {
	block, _ := pem.Decode([]byte(pemData))
	if block == nil {
		return errors.New("invalid PEM data")
	}

	_, err := x509.ParsePKIXPublicKey(block.Bytes)
	return err
}
