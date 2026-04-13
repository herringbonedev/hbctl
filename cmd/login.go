package cmd

import (
	"fmt"
	"os"

	"github.com/herringbonedev/hbctl/internal/secrets"
	"github.com/spf13/cobra"
)

func loginCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Store encrypted credentials and keys",
	}

	cmd.AddCommand(loginMongoCommand())
	cmd.AddCommand(loginJWTSecretCommand())
	cmd.AddCommand(loginServiceKeyCommand())
	return cmd
}

func loginMongoCommand() *cobra.Command {
	var user string
	var password string
	var host string
	var database string
	var port int
	var authSource string
	var replicaSet string

	cmd := &cobra.Command{
		Use:   "mongodb",
		Short: "Store MongoDB credentials",
		RunE: func(cmd *cobra.Command, args []string) error {
			if user == "" || password == "" || host == "" {
				return fmt.Errorf("--user, --password, and --host are required")
			}
			err := secrets.SaveMongo(&secrets.MongoSecret{
				User:       user,
				Password:   password,
				Host:       host,
				Port:       port,
				Database:   database,
				AuthSource: authSource,
				ReplicaSet: replicaSet,
			})
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "MongoDB credentials saved")
			return nil
		},
	}

	cmd.Flags().StringVar(&user, "user", "", "MongoDB username")
	cmd.Flags().StringVar(&password, "password", "", "MongoDB password")
	cmd.Flags().StringVar(&host, "host", "", "MongoDB host")
	cmd.Flags().StringVar(&database, "database", "herringbone", "MongoDB database")
	cmd.Flags().IntVar(&port, "port", 27017, "MongoDB port")
	cmd.Flags().StringVar(&authSource, "auth-source", "admin", "MongoDB auth source")
	cmd.Flags().StringVar(&replicaSet, "replica-set", "", "MongoDB replica set")
	return cmd
}

func loginJWTSecretCommand() *cobra.Command {
	var jwtSecret string

	cmd := &cobra.Command{
		Use:   "jwtsecret",
		Short: "Store the JWT secret",
		RunE: func(cmd *cobra.Command, args []string) error {
			if jwtSecret == "" {
				return fmt.Errorf("--secret is required")
			}
			if err := secrets.SaveJWTSecret(&secrets.JWTSecret{JWTSecret: jwtSecret}); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "JWT secret saved")
			return nil
		},
	}

	cmd.Flags().StringVar(&jwtSecret, "secret", "", "JWT secret value")
	return cmd
}

func loginServiceKeyCommand() *cobra.Command {
	var publicPath string
	var privatePath string
	var publicInline string
	var privateInline string
	var generate bool
	var bits int

	cmd := &cobra.Command{
		Use:   "servicekey",
		Short: "Store or generate service signing keys",
		RunE: func(cmd *cobra.Command, args []string) error {
			var publicKey string
			var privateKey string

			if generate {
				generatedPublic, generatedPrivate, err := secrets.GenerateServiceKeyPair(bits)
				if err != nil {
					return err
				}
				publicKey = generatedPublic
				privateKey = generatedPrivate
			} else {
				if publicInline != "" {
					publicKey = publicInline
				} else if publicPath != "" {
					data, err := os.ReadFile(publicPath)
					if err != nil {
						return err
					}
					publicKey = string(data)
				}

				if privateInline != "" {
					privateKey = privateInline
				} else if privatePath != "" {
					data, err := os.ReadFile(privatePath)
					if err != nil {
						return err
					}
					privateKey = string(data)
				}
			}

			if publicKey == "" || privateKey == "" {
				return fmt.Errorf("provide both public and private keys or use --generate")
			}

			if err := secrets.SaveServiceKey(&secrets.ServiceKey{
				PubSvcKey:  publicKey,
				PrivSvcKey: privateKey,
			}); err != nil {
				return err
			}
			if generate {
				fmt.Fprintln(cmd.OutOrStdout(), "Service keypair generated and saved")
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "Service keypair saved")
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&publicPath, "public", "", "Path to the public key PEM file")
	cmd.Flags().StringVar(&privatePath, "private", "", "Path to the private key PEM file")
	cmd.Flags().StringVar(&publicInline, "public-key", "", "Inline public key PEM")
	cmd.Flags().StringVar(&privateInline, "private-key", "", "Inline private key PEM")
	cmd.Flags().BoolVar(&generate, "generate", false, "Generate a new RSA keypair")
	cmd.Flags().IntVar(&bits, "bits", 4096, "RSA key size when generating")
	return cmd
}
