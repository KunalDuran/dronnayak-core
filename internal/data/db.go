package data

import (
	"context"
	"log"
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

func InitDB(mongoURI string) {
	dbOnce.Do(func() {
		if mongoURI == "" {
			mongoURI = "mongodb://localhost:27017"
			log.Println("MONGO_URI not found, using default value")
		}

		clientOptions := options.Client().ApplyURI(mongoURI)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		c, err := mongo.Connect(ctx, clientOptions)
		if err != nil {
			log.Fatal(err)
		}

		pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer pingCancel()

		if err = c.Ping(pingCtx, nil); err != nil {
			log.Fatal(err)
		}

		log.Println("Connected to MongoDB")
		db = c
	})
}

func GenerateUID() string {
	id, err := gonanoid.New(10)
	if err != nil {
		log.Printf("Error generating UID: %v", err)
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
	c := GetCollection(collection)
	_, err := c.InsertOne(ctx, document)
	return err
}

func Insert(collection string, document []interface{}) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()
	c := GetCollection(collection)
	_, err := c.InsertMany(ctx, document)
	return err
}

func FindOne(collection string, filter map[string]interface{}, result interface{}) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()
	c := GetCollection(collection)
	return c.FindOne(ctx, bson.M(filter)).Decode(result)
}

func FindAll(collection string, filter map[string]interface{}, results interface{}, opts ...*options.FindOptions) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()
	c := GetCollection(collection)
	cursor, err := c.Find(ctx, bson.M(filter), opts...)
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)
	return cursor.All(ctx, results)
}

func UpdateOne(collection string, filter map[string]interface{}, update map[string]interface{}, opts ...*options.UpdateOptions) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()
	c := GetCollection(collection)
	_, err := c.UpdateOne(ctx, bson.M(filter), bson.M{"$set": update}, options.Update().SetUpsert(true))
	return err
}

func DeleteOne(collection string, filter map[string]interface{}) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()
	c := GetCollection(collection)
	_, err := c.DeleteOne(ctx, bson.M(filter))
	return err
}

func TryStringToObjectID(id string) primitive.ObjectID {
	o, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		log.Println("Error converting string to ObjectID: ", err)
		return primitive.ObjectID{}
	}
	return o
}
