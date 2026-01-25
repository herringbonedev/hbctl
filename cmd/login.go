package cmd

import (
	"flag"
	"fmt"
	"os"

	"github.com/herringbonedev/hbctl/internal/secrets"
)

func init() {
	Register("login", loginCmd)
}

func loginCmd(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: hbctl login <backend>")
		fmt.Fprintln(os.Stderr, "Available backends: mongodb, jwtsecret, servicekey")
		os.Exit(1)
	}

	switch args[0] {
	case "mongodb":
		loginMongo(args[1:])
	case "jwtsecret":
		loginJWTSecret(args[1:])
	case "servicekey":
		loginServiceKey(args[1:])
	default:
		fmt.Fprintln(os.Stderr, "Unknown backend:", args[0])
		os.Exit(1)
	}
}

func loginMongo(args []string) {
	fs := flag.NewFlagSet("login mongodb", flag.ExitOnError)

	user := fs.String("user", "", "MongoDB username (required)")
	password := fs.String("password", "", "MongoDB password (required)")
	host := fs.String("host", "", "MongoDB host (required)")

	database := fs.String("database", "herringbone", "Database name")
	port := fs.Int("port", 27017, "MongoDB port")
	authSource := fs.String("auth-source", "admin", "Auth source database")
	replicaSet := fs.String("replica-set", "", "Replica set name")

	fs.Parse(args)

	if *user == "" || *password == "" || *host == "" {
		fmt.Fprintln(os.Stderr, "Error: --user, --password, and --host are required")
		fs.Usage()
		os.Exit(1)
	}

	secret := &secrets.MongoSecret{
		User:       *user,
		Password:   *password,
		Host:       *host,
		Port:       *port,
		Database:   *database,
		AuthSource: *authSource,
		ReplicaSet: *replicaSet,
	}

	if err := secrets.SaveMongo(secret); err != nil {
		fmt.Fprintln(os.Stderr, "Failed to save secret:", err)
		os.Exit(1)
	}

	fmt.Println("✔ MongoDB credentials saved")
}

func loginJWTSecret(args []string) {
	fs := flag.NewFlagSet("login jwtsecret", flag.ExitOnError)

	jwtsecret := fs.String("secret", "", "JWT Secret Phrase")

	fs.Parse(args)

	if *jwtsecret == "" {
		fmt.Fprintln(os.Stderr, "Missing required flag: -secret")
		os.Exit(1)
	}

	secret := &secrets.JWTSecret{
		JWTSecret: *jwtsecret,
	}

	if err := secrets.SaveJWTSecret(secret); err != nil {
		fmt.Fprintln(os.Stderr, "Failed to save secret:", err)
		os.Exit(1)
	}

	fmt.Println("JWT secret saved successfully.")
}

func loginServiceKey(args []string) {
	fs := flag.NewFlagSet("login servicekey", flag.ExitOnError)

	pubPath := fs.String("public", "", "Path to service public key PEM file")
	privPath := fs.String("private", "", "Path to service private key PEM file")

	pubInline := fs.String("public-key", "", "Service public key (inline)")
	privInline := fs.String("private-key", "", "Service private key (inline)")

	generate := fs.Bool("generate", false, "Generate a new RSA service keypair") // ADDED
	bits := fs.Int("bits", 4096, "RSA key size when generating")                 // ADDED

	fs.Parse(args)

	var pubKey, privKey string

	// Auto-generate keys
	if *generate { // ADDED
		pub, priv, err := secrets.GenerateServiceKeyPair(*bits) // ADDED
		if err != nil { // ADDED
			fmt.Fprintln(os.Stderr, "Failed to generate service keypair:", err) // ADDED
			os.Exit(1)                                                         // ADDED
		}                                                                       // ADDED
		pubKey = pub                                                          // ADDED
		privKey = priv                                                        // ADDED
	} else {

		// Load public key
		if *pubInline != "" {
			pubKey = *pubInline
		} else if *pubPath != "" {
			b, err := os.ReadFile(*pubPath)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Failed to read public key file:", err)
				os.Exit(1)
			}
			pubKey = string(b)
		}

		// Load private key
		if *privInline != "" {
			privKey = *privInline
		} else if *privPath != "" {
			b, err := os.ReadFile(*privPath)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Failed to read private key file:", err)
				os.Exit(1)
			}
			privKey = string(b)
		}
	}

	if pubKey == "" || privKey == "" {
		fmt.Fprintln(os.Stderr, "Error: must provide both public and private keys (or use --generate)")
		fs.Usage()
		os.Exit(1)
	}

	secret := &secrets.ServiceKey{
		PubSvcKey:  pubKey,
		PrivSvcKey: privKey,
	}

	if err := secrets.SaveServiceKey(secret); err != nil {
		fmt.Fprintln(os.Stderr, "Failed to save service keys:", err)
		os.Exit(1)
	}

	if *generate { // ADDED
		fmt.Println("✔ Service JWT keypair generated and saved securely") // ADDED
	} else {
		fmt.Println("✔ Service JWT keypair saved successfully")
	}
}
