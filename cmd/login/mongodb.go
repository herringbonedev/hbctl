package login

import (
	"flag"
	"fmt"
	"os"

	"github.com/herringbonedev/hbctl/internal/secrets"
)

func loginMongo(args []string) {
	fs := flag.NewFlagSet("login mongodb", flag.ExitOnError)

	user := fs.String("user", "", "MongoDB username (required)")
	password := fs.String("password", "", "MongoDB password (required)")
	host := fs.String("host", "", "MongoDB host (required)")

	database := fs.String("database", "", "Database name")
	collection := fs.String("collection", "", "Collection name")
	port := fs.Int("port", 27017, "MongoDB port")
	authSource := fs.String("auth-source", "herringbone", "Auth source database")
	replicaSet := fs.String("replica-set", "", "Replica set name")

	fs.Parse(args)

	if *user == "" || *password == "" || *host == "" {
		fmt.Fprintln(os.Stderr, "Error: --user, --password, and --host are required")
		fs.Usage()
		os.Exit(1)
	}

	secret := &secrets.MongoSecret{
		User:        *user,
		Password:    *password,
		Host:        *host,
		Port:        *port,
		Database:    *database,
		Collection:  *collection,
		AuthSource:  *authSource,
		ReplicaSet: *replicaSet,
	}

	if err := secrets.SaveMongo(secret); err != nil {
		fmt.Fprintln(os.Stderr, "Failed to save secret:", err)
		os.Exit(1)
	}

	fmt.Println("✔ MongoDB credentials saved")
}
