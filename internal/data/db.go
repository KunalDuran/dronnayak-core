package data

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	gonanoid "github.com/matoous/go-nanoid/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	dbName    = "dronnayak"
	dbTimeout = 5 * time.Second
)

var (
	db     *mongo.Client
	dbOnce sync.Once
)

// maskedURI strips credentials from a MongoDB URI for safe logging.
func maskedURI(uri string) string {
	if i := strings.Index(uri, "@"); i >= 0 {
		if j := strings.Index(uri, "://"); j >= 0 {
			return uri[:j+3] + "***@" + uri[i+1:]
		}
	}
	return uri
}

func InitDB(mongoURI string) {
	dbOnce.Do(func() {
		if mongoURI == "" {
			mongoURI = "mongodb://localhost:27017"
			slog.Warn("MONGO_URI not set, using default", "uri", mongoURI)
		}

		clientOptions := options.Client().ApplyURI(mongoURI)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		c, err := mongo.Connect(ctx, clientOptions)
		if err != nil {
			slog.Error("failed to connect to MongoDB", "uri", maskedURI(mongoURI), "error", err)
			os.Exit(1)
		}

		pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer pingCancel()

		if err = c.Ping(pingCtx, nil); err != nil {
			slog.Error("MongoDB ping failed", "uri", maskedURI(mongoURI), "error", err)
			os.Exit(1)
		}

		slog.Info("connected to MongoDB", "uri", maskedURI(mongoURI), "db", dbName)
		db = c
	})
}

func GenerateUID() string {
	id, err := gonanoid.New(10)
	if err != nil {
		slog.Error("failed to generate UID", "error", err)
		return ""
	}
	return id
}

func GetCollection(name string) *mongo.Collection {
	return db.Database(dbName).Collection(name)
}

func InsertOne(collection string, document interface{}) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()
	slog.Debug("db insert", "collection", collection)
	_, err := GetCollection(collection).InsertOne(ctx, document)
	return err
}

func Insert(collection string, document []interface{}) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()
	slog.Debug("db insert many", "collection", collection, "count", len(document))
	_, err := GetCollection(collection).InsertMany(ctx, document)
	return err
}

func FindOne(collection string, filter map[string]interface{}, result interface{}) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()
	slog.Debug("db find one", "collection", collection)
	return GetCollection(collection).FindOne(ctx, bson.M(filter)).Decode(result)
}

func FindAll(collection string, filter map[string]interface{}, results interface{}, opts ...*options.FindOptions) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()
	slog.Debug("db find all", "collection", collection)
	cursor, err := GetCollection(collection).Find(ctx, bson.M(filter), opts...)
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)
	return cursor.All(ctx, results)
}

func UpdateOne(collection string, filter map[string]interface{}, update map[string]interface{}, opts ...*options.UpdateOptions) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()
	slog.Debug("db update one", "collection", collection)
	_, err := GetCollection(collection).UpdateOne(ctx, bson.M(filter), bson.M{"$set": update}, options.Update().SetUpsert(true))
	return err
}

func DeleteOne(collection string, filter map[string]interface{}) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()
	slog.Debug("db delete one", "collection", collection)
	_, err := GetCollection(collection).DeleteOne(ctx, bson.M(filter))
	return err
}

func TryStringToObjectID(id string) primitive.ObjectID {
	o, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		slog.Warn("failed to convert string to ObjectID", "id", id, "error", err)
		return primitive.ObjectID{}
	}
	return o
}
