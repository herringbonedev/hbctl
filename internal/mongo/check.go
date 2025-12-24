package mongo

import (
	"context"
	"fmt"
	"time"

	mongodrv "go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func CanConnect(
	uri string,
) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client, err := mongodrv.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return false
	}
	defer client.Disconnect(ctx)

	return client.Ping(ctx, nil) == nil
}

func WaitForConnect(uri string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for {
		if CanConnect(uri) {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for MongoDB at %s", uri)
		}
		time.Sleep(2 * time.Second)
	}
}
