package mongo

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	mongodrv "go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func EnsureUser(
	host string,
	port int,
	rootPass string,
	appUser string,
	appPass string,
	dbName string,
) error {

	uri := fmt.Sprintf(
		"mongodb://root:%s@%s:%d/admin",
		rootPass, host, port,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	client, err := mongodrv.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return err
	}
	defer client.Disconnect(ctx)

	db := client.Database(dbName)

	cmd := bson.D{
		{"createUser", appUser},
		{"pwd", appPass},
		{"roles", bson.A{
			bson.D{
				{"role", "readWrite"},
				{"db", dbName},
			},
		}},
	}

	err = db.RunCommand(ctx, cmd).Err()
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			return nil
		}
		return err
	}

	return nil
}

func EnsurePlatformOrg(
	host string,
	port int,
	appUser string,
	appPass string,
	dbName string,
	authSource string,
) error {
	if authSource == "" {
		authSource = dbName
	}

	uri := fmt.Sprintf(
		"mongodb://%s:%s@%s:%d/%s?authSource=%s",
		appUser, appPass, host, port, dbName, authSource,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	client, err := mongodrv.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return err
	}
	defer client.Disconnect(ctx)

	db := client.Database(dbName)
	now := time.Now().UTC()

	_, err = db.Collection("organizations").UpdateOne(
		ctx,
		bson.D{{"slug", "platform"}},
		bson.D{
			{"$set", bson.D{
				{"name", "Platform"},
				{"slug", "platform"},
				{"status", "active"},
				{"updated_at", now},
			}},
			{"$setOnInsert", bson.D{
				{"created_at", now},
			}},
		},
		options.Update().SetUpsert(true),
	)
	return err
}
