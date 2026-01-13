package data

import (
	"context"
	"fmt"
	"log"

	gonanoid "github.com/matoous/go-nanoid/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var db *mongo.Client

func InitDB(mongoURI string) {

	var err error
	if mongoURI == "" {
		mongoURI = "mongodb://localhost:27017"
		log.Default().Println("MONGO_URI not found, using default value")
	}

	clientOptions := options.Client().ApplyURI(mongoURI)
	c, err := mongo.Connect(context.TODO(), clientOptions)
	if err != nil {
		log.Fatal(err)
	}

	err = c.Ping(context.TODO(), nil)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Connected to MongoDB")

	db = c
}

func GenerateUID() string {
	id, _ := gonanoid.New(10)
	return id
	// return uuid.New().String()
}

func GetCollection(name string) *mongo.Collection {
	return db.Database("dronnayak").Collection(name)
}

func InsertOne(collection string, document interface{}) error {
	c := GetCollection(collection)
	_, err := c.InsertOne(context.Background(), document)
	return err
}

func Insert(collection string, document []interface{}) error {
	c := GetCollection(collection)
	_, err := c.InsertMany(context.Background(), document)
	return err
}

func FindOne(collection string, filter map[string]interface{}, result interface{}) error {
	var f = make(bson.M)
	for k, v := range filter {
		f[k] = v
	}

	c := GetCollection(collection)
	err := c.FindOne(context.Background(), f).Decode(result)
	return err
}

func FindAll(collection string, filter map[string]interface{}, results interface{}, opts ...*options.FindOptions) error {
	var f = make(bson.M)
	for k, v := range filter {
		f[k] = v
	}
	c := GetCollection(collection)
	cursor, err := c.Find(context.Background(), f)
	if err != nil {
		return err
	}
	defer cursor.Close(context.Background())

	return cursor.All(context.Background(), results)
}

func UpdateOne(collection string, filter map[string]interface{}, update map[string]interface{}, opts ...*options.UpdateOptions) error {
	c := GetCollection(collection)

	flt := make(bson.M)
	for k, v := range filter {
		flt[k] = v
	}

	upd := bson.M{
		"$set": update,
	}

	r, err := c.UpdateOne(context.Background(), flt, upd, options.Update().SetUpsert(true))
	fmt.Println(r)
	return err
}

func DeleteOne(collection string, filter bson.M) error {
	c := GetCollection(collection)
	_, err := c.DeleteOne(context.Background(), filter)
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
