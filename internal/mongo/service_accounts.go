package mongo

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	mongodrv "go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// EnsureServiceAccountScopes upgrades an existing Herringbone service account's
// grant list so auth can mint a token containing newly-required scopes.
//
// The auth service refuses token requests whose scopes exceed the service
// account grants. During alpha upgrades, the service account may already exist
// with an older grant list, so registering it again is not enough. hbctl must
// extend the existing document before requesting a fresh token.
func EnsureServiceAccountScopes(
	host string,
	port int,
	appUser string,
	appPass string,
	dbName string,
	authSource string,
	serviceName string,
	scopes []string,
) error {
	serviceName = strings.TrimSpace(serviceName)
	if serviceName == "" {
		return fmt.Errorf("service name is required")
	}

	cleanScopes := make([]string, 0, len(scopes))
	seen := map[string]bool{}
	for _, scope := range scopes {
		scope = strings.TrimSpace(scope)
		if scope == "" || seen[scope] {
			continue
		}
		seen[scope] = true
		cleanScopes = append(cleanScopes, scope)
	}
	if len(cleanScopes) == 0 {
		return nil
	}

	if authSource == "" {
		authSource = dbName
	}

	uri := fmt.Sprintf(
		"mongodb://%s:%s@%s:%d/%s?authSource=%s",
		url.QueryEscape(appUser),
		url.QueryEscape(appPass),
		host,
		port,
		url.PathEscape(dbName),
		url.QueryEscape(authSource),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	client, err := mongodrv.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return err
	}
	defer client.Disconnect(ctx)

	now := time.Now().UTC()
	coll := client.Database(dbName).Collection("service_accounts")

	filter := bson.D{{"$or", bson.A{
		bson.D{{"service_name", serviceName}},
		bson.D{{"name", serviceName}},
		bson.D{{"service", serviceName}},
	}}}

	update := bson.D{
		{"$addToSet", bson.D{{"scopes", bson.D{{"$each", cleanScopes}}}}},
		{"$set", bson.D{{"updated_at", now}}},
	}

	res, err := coll.UpdateMany(ctx, filter, update)
	if err != nil {
		return err
	}
	if res.MatchedCount > 0 {
		return nil
	}

	// Some early auth builds used only service_id as the stable lookup and did
	// not store a service_name. Avoid guessing a service_id here; report clearly
	// so the operator can inspect the auth DB rather than silently creating an
	// unusable duplicate account.
	return fmt.Errorf("service account %q not found in service_accounts", serviceName)
}
