package coapgateway

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"time"

	firebase "firebase.google.com/go"
	"github.com/go-ocf/go-coap/v2/udp"
	"github.com/go-ocf/go-coap/v2/udp/client"
	"google.golang.org/api/option"
)

// Temperature describes a measured temperature value
type Temperature struct {
	Time  int64   `json:"time"`
	Value float64 `json:"value"`
}

// BalloonData contains the relevant data measured by the onboard sensors
type BalloonData struct {
	MapX        float64      `json:"mapX"`
	MapY        float64      `json:"mapY"`
	TimeStamp   uint64       `json:"Timestamp"`
	Humidity    float64      `json:"Humidity"`
	Temperature *Temperature `json:"temperature"`
	Pressure    float64      `json:"pressure"`
}

func pushToFirebase(app *firebase.App, data *BalloonData) error {
	ctx := context.Background()

	// Create a database client from App.
	client, err := app.Database(ctx)
	if err != nil {
		log.Fatalln("Error initializing database client:", err)
		return err
	}

	ref := client.NewRef("test")
	balloonsRef := ref.Child("balloons")

	balloonRef := balloonsRef.Child("balloon0")
	temperaturesRef := balloonRef.Child("temperature")
	humidityRef := balloonRef.Child("humidity")
	pressureRef := balloonRef.Child("pressure")

	_, err = temperaturesRef.Push(ctx, data.Temperature)
	if err != nil {
		return err
	}
	_, err = humidityRef.Push(ctx, data.Humidity)
	if err != nil {
		return err
	}
	_, err = pressureRef.Push(ctx, data.Pressure)
	if err != nil {
		return err
	}

	if err != nil {
		log.Fatalln("Error setting value:", err)
		return err
	}

	return nil
}

func getData(client *client.ClientConn, resource string) (float64, error) {
	resp, err := client.Get(context.Background(), resource)
	if err != nil {
		return 0.0, fmt.Errorf("Cannot get response: %v", err)
	}

	data := make([]byte, 4)

	_, err = io.ReadFull(resp.Body(), data)

	log.Printf("Response Body: %s", data)

	val, err := strconv.ParseFloat(string(data[:]), 64)
	if err != nil {
		return 0.0, err
	}

	return val, nil
}

func prepareData(balloonURI string) (*BalloonData, error) {
	client, err := udp.Dial(balloonURI)
	if err != nil {
		log.Fatalf("Error dialing: %v", err)
	}

	// Change resolution of temperature
	temp, err := getData(client, "/sens/temp")
	if err != nil {
		return nil, err
	}

	tempV := &Temperature{Value: temp / 100, Time: time.Now().Unix()}

	humidity, err := getData(client, "/sens/hum")
	if err != nil {
		return nil, err
	}

	humV := humidity / 100

	pressure, err := getData(client, "/sens/press")
	if err != nil {
		return nil, err
	}

	balloonData := &BalloonData{
		Temperature: tempV,
		Humidity:    humV,
		Pressure:    pressure,
	}

	return balloonData, client.Close()
}

// Startup starts a coap gateway instance
func Startup() {
	jsonFile, err := os.Open("config.json")
	if err != nil {
		fmt.Println(err)
	}

	byteValue, _ := ioutil.ReadAll(jsonFile)

	var configjson map[string]string
	json.Unmarshal([]byte(byteValue), &configjson)
	jsonFile.Close()

	conf := &firebase.Config{
		DatabaseURL: configjson["db_url"],
	}

	opt := option.WithCredentialsFile(configjson["cred_file"])
	app, err := firebase.NewApp(context.Background(), conf, opt)
	if err != nil {
		_ = fmt.Errorf("error initializing app: %v", err)
		return
	}

	dbClient, err := app.Database(context.Background())
	ref := dbClient.NewRef("test")
	balloonRef := ref.Child("balloons")
	err = balloonRef.Delete(context.Background())
	if err != nil {
		log.Fatalf("%v", err)
		return
	}

	// Loop of fetching and pushing data
	for {
		balloonData, err := prepareData("[fe80::32ae:a4ff:fef5:7544%eth0]:5683")
		if err != nil {
			log.Fatalf("%v", err)
			return
		}

		pushToFirebase(app, balloonData)
	}
}
