// tokengen is a local development helper that prints a signed ES256 JWT
// suitable for authenticating against the Todo API.
//
// Usage:
//
//	go run ./cmd/tokengen
//	go run ./cmd/tokengen -sub alice -ttl 10m
package main

import (
	"crypto/x509"
	"encoding/pem"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func main() {
	keyPath := flag.String("key", "keys/private.pem", "path to ES256 private key PEM file")
	sub := flag.String("sub", "dev-user", "JWT subject claim")
	ttl := flag.Duration("ttl", 10*time.Minute, "token lifetime (max 15m enforced by server)")
	flag.Parse()

	if *ttl > 15*time.Minute {
		log.Fatal("ttl exceeds the server's 15-minute maximum")
	}

	keyBytes, err := os.ReadFile(*keyPath)
	if err != nil {
		log.Fatalf("reading private key: %v", err)
	}
	block, _ := pem.Decode(keyBytes)
	if block == nil {
		log.Fatal("no PEM block found in key file")
	}
	key, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		log.Fatalf("parsing private key: %v", err)
	}

	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
		"sub": *sub,
		"iat": now.Unix(),
		"exp": now.Add(*ttl).Unix(),
	})

	signed, err := token.SignedString(key)
	if err != nil {
		log.Fatalf("signing token: %v", err)
	}
	fmt.Println(signed)
}
