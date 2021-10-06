package main

import (
	"fmt"
	"context"
	"log"
	"encoding/json"
	"time"
	"os"
	"os/signal"
	"syscall"

	"github.com/segmentio/kafka-go"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	)
func cleanOnInt(r *kafka.Reader){
    c := make(chan os.Signal)
    signal.Notify(c, os.Interrupt, syscall.SIGTERM)
    go func() {
        <-c
	fmt.Println("done reading...")

	if err := r.Close(); err != nil {
	    log.Fatal("failed to close reader:", err)
	}

	fmt.Println("goodbye from kafka-consumer")
        os.Exit(1)
    }()
}


func main(){

	// message:= `{"data":{"date-time":{"system":"2021-09-24T01:40:01+00:00"},"gps-info":{"Altitude":"552.8","Date":"240921","HDOP":"0.7","Latitude":"42.70599365","Longitude":"23.31282425","SatelliteUsed":9,"Speed":52.782001495361328,"Time":"014001.00","Validity":"A"},"modem-info":{"signal-quality":"31"},"stop-info":{}},"device-id":"004101FB","device-type":"OBU","hostname":"obu","priority":1,"scheme-version":"v1_0_9","vehicle-id":"132801","id":"ddd21912-421c-4839-8669-153dfc4d6def"}`
	fmt.Println("Hello from kafka-consumer")
	fmt.Println("reading vehicles..")
	r := kafka.NewReader(kafka.ReaderConfig{
	    Brokers:   []string{"kafka:9092"},
	    Topic:     "vehicles",
	    Partition: 0,
	    MinBytes:  0, // 10e3, // 10KB
	    MaxBytes:  600e6, // 600MB
	})
	r.SetOffset(0)
	cleanOnInt(r)

	for {
	    message, err := r.ReadMessage(context.Background())
	    if err != nil {
		print(err)
		// probably kafka still booting
		time.Sleep(2 * time.Second)
	    }
	    fmt.Printf("message at offset %d: %s = %s\n", message.Offset, string(message.Key), string(message.Value))
	    // message_str := string(message.Value)
	    writeMongo(message.Value)
	}

}

func getMongoCollection(coll_name string) (*mongo.Collection, *mongo.Client){
	// Set client options
	uri := "mongodb://root:root@mongo:27017"
	clientOptions := options.Client().ApplyURI(uri)

	fmt.Println("Connecting to MongoDB..")
	client, err := mongo.Connect(context.Background(), clientOptions)

	if err != nil {
	    log.Fatal(err)
	}

	fmt.Println("Pinging MongoDB")
	err = client.Ping(context.Background(), nil)

	if err != nil {
	    log.Fatal(err)
	}

	fmt.Println("Connected to MongoDB!")

	collection := client.Database("theoremus").Collection(coll_name)
	return collection, client
}

func parseTime(rfc3339str string) time.Time{
	rfc3339time, err := time.Parse(time.RFC3339, rfc3339str)
	if err != nil {
		panic(err)
	}
	return rfc3339time
}

func addTimeKeys(message_obj *SensorFields){
	rfc3339time := message_obj.Data.DateTime.System
	// rfc3339time := parseTime(rfc3339str)

	d, h := trucateTime(rfc3339time)
	// add rounded day and hour
	message_obj.IDDay = d
	message_obj.IDHour = h
	// old: use this if working with bson directly
	// sensor_extended := append(mybson, bson.E{"Extra_Day", d}, bson.E{"Extra_Hour", h})
}
func CoerceTypes(rawMap map[string]interface{}){
	fmt.Printf("%+v\n", rawMap)
	panic("hi")

}


func writeMongo(rawmessage []byte){
	// var mybson primitive.M
	// err := bson.UnmarshalExtJSON(
	// 	[]byte(JSONstr), true, &mybson)
	// if err != nil {
	// 	panic(err)
	// }
	// rfc3339str := mybson.Map()["data"].(primitive.D).Map()["date-time"].(primitive.D).Map()["system"]

	var message_obj SensorFields
	err := json.Unmarshal(rawmessage, &message_obj)
	if err != nil {
	    panic(err)
	}

	addTimeKeys(&message_obj)
	fmt.Printf("%+v\n", message_obj)

	vehicles, client := getMongoCollection("vehicles")
	// Call the InsertOne() method and pass the context and doc objects
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	insertResult, insertErr := vehicles.InsertOne(ctx, message_obj)
	// insertResult, insertErr := vehicles.InsertOne(ctx, `{"car":100}`)

	// Check for any insertion errors
	if insertErr != nil {
		fmt.Println("InsertOne ERROR:", insertErr)
	} else {
		fmt.Println("InsertOne() API result:", insertResult)
	}


	err = client.Disconnect(context.Background())

	if err != nil {
	    log.Fatal(err)
	}
	fmt.Println("Connection to MongoDB closed.")


}

func trucateTime(t time.Time) (time.Time, time.Time) {
	h := t.Truncate(time.Hour)
	d := t.Truncate(24 * time.Hour)
	return d, h

}
