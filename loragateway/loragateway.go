package loragateway

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"riot-gateway/gatewaytypes"
	"sync"
	"time"

	firebase "firebase.google.com/go"
	ttnsdk "github.com/TheThingsNetwork/go-app-sdk"
	ttnlog "github.com/TheThingsNetwork/go-utils/log"
	"github.com/TheThingsNetwork/go-utils/log/apex"
	"github.com/apex/log"
	"google.golang.org/api/option"

	cbor "github.com/fxamacker/cbor/v2"
)

const (
	sdkClientName = "riot-gateway"
	appID         = "riot_balloon_control_2020"
	p0            = 101325 // Sea-level pressure (used for calculating heights of balloons)
)

var client ttnsdk.Client               // The LORA client
var clusters *gatewaytypes.ClusterList // Global ClusterList
var clustersMutex sync.Mutex           // Mutex for multiple goroutine access to ClusterList

// balloonHandler handles a single balloon as a thread
func balloonHandler(app *firebase.App, wg *sync.WaitGroup, balloonID string) error { // Manage devices for the application.
	// Start Publish/Subscribe client (MQTT)
	pubsub, err := client.PubSub()
	if err != nil {
		log.WithError(err).Fatal("riot-gateway: could not get application pub/sub")
	}

	// Make sure the pubsub client is closed before the function returns
	// In your application, you should call this before the application shuts down
	defer pubsub.Close()

	// Get a publish/subscribe client scoped to my-test-device
	myNewDevicePubSub := pubsub.Device(balloonID)

	// Make sure the pubsub client for this device is closed before the function returns
	// In your application, you will probably call this when you no longer need the device
	// This also stops existing subscriptions, in case you forgot to unsubscribe
	defer myNewDevicePubSub.Close()

	// Subscribe to uplink messages
	uplink, err := myNewDevicePubSub.SubscribeUplink()
	if err != nil {
		return fmt.Errorf("riot-gateway: could not subscribe to uplink messages")
	}

	log.Debug("After this point, the program won't show anything until we receive an uplink message from device my-new-device.")
	for message := range uplink {
		v := &map[string]float64{}
		err := cbor.Unmarshal(message.PayloadRaw, &v)
		if err != nil {
			log.Errorf("%v\n", err)
		}
		log.WithField("data", v).Info("riot-gateway: received uplink")

		err = updateBalloon(app, balloonID, v)
		if err != nil {
			return err
		}

		// Also update cluster when value is received
		clusterIndex, _ := findBalloonInClusters(balloonID)
		err = updateCluster(app, clusterIndex)

		pushToFirebase(app)
	}

	wg.Done() // Signal done to waitgroup
	return nil
}

// calculateAltitude calculates the altitude of a balloon
func calculateAltitude(clusterID string, balloonID string) {
	balloon := clusters.Clusters[clusterID].Balloons[balloonID]

	p := balloon.Pressure[len(balloon.Pressure)-1]       // Get latest pressure value
	t := balloon.Temperature[len(balloon.Temperature)-1] // Get latest temperature value

	// Calculate the altitude
	altitude := ((math.Pow(p0/p.Value, 1/5.257) - 1) * (t.Value + 273.15)) / 0.0065

	balloon.Altitude = altitude
}

// pushToFirebase handles pushing stuff to the database
func pushToFirebase(app *firebase.App) error {
	ctx := context.Background()

	// Create a database client from App.
	client, err := app.Database(ctx)
	if err != nil {
		return err
	}
	ref := client.NewRef("clusters")

	clustersMutex.Lock()

	// Create map for DB
	clusterMap := map[string]interface{}{}
	for c, clus := range clusters.Clusters {
		clusterMap[c] = clus
	}

	err = ref.Update(ctx, clusterMap)

	if err != nil {
		return err
	}

	clustersMutex.Unlock()
	return nil
}

// updateCluster updates a given cluster by calculating the new averages
// should be called once new data from a balloon arrives
func updateCluster(app *firebase.App, clusterID string) error {
	clustersMutex.Lock()

	var avgTemp float64
	var avgPres float64
	var avgHum float64

	for _, b := range clusters.Clusters[clusterID].Balloons {
		if len(b.Temperature) < 1 {
			continue // If no temperature value is set yet, skip
		}

		avgTemp += b.Temperature[len(b.Temperature)-1].Value
		avgPres += b.Pressure[len(b.Pressure)-1].Value
		avgHum += b.Humidity[len(b.Humidity)-1].Value
	}

	// Calculate average by dividing by amount of balloons used
	// for measurement
	avgTemp /= float64(len(clusters.Clusters[clusterID].Balloons))
	avgPres /= float64(len(clusters.Clusters[clusterID].Balloons))
	avgHum /= float64(len(clusters.Clusters[clusterID].Balloons))

	// Update averages of cluster
	clusters.Clusters[clusterID].AverageTemp = avgTemp
	clusters.Clusters[clusterID].AverageHumidity = avgHum
	clusters.Clusters[clusterID].AveragePressure = avgPres

	clustersMutex.Unlock()
	return nil
}

// updateBalloon updates a single balloon in a cluster
func updateBalloon(app *firebase.App, balloonID string, values *map[string]float64) error {
	clustersMutex.Lock()

	clusterID, err := findBalloonInClusters(balloonID)
	if err != nil {
		return err
	}

	// Temperature, Humidity and Pressure
	clusters.Clusters[clusterID].Balloons[balloonID].Temperature = append(
		clusters.Clusters[clusterID].Balloons[balloonID].Temperature,
		&gatewaytypes.MeasuredValue{
			Value:     (*values)["temp"],
			Timestamp: time.Now().Unix(),
		},
	)

	clusters.Clusters[clusterID].Balloons[balloonID].Pressure = append(
		clusters.Clusters[clusterID].Balloons[balloonID].Pressure,
		&gatewaytypes.MeasuredValue{
			Value:     (*values)["pres"],
			Timestamp: time.Now().Unix(),
		},
	)

	clusters.Clusters[clusterID].Balloons[balloonID].Humidity = append(
		clusters.Clusters[clusterID].Balloons[balloonID].Humidity,
		&gatewaytypes.MeasuredValue{
			Value:     (*values)["hum"],
			Timestamp: time.Now().Unix(),
		},
	)

	// GPS coordinates & velocity
	clusters.Clusters[clusterID].Balloons[balloonID].Longitude = (*values)["long"]
	clusters.Clusters[clusterID].Balloons[balloonID].Latitude = (*values)["lat"]
	clusters.Clusters[clusterID].Balloons[balloonID].Velocity = (*values)["vel"]

	// Calculate the altitude of the balloon
	calculateAltitude(clusterID, balloonID)

	clustersMutex.Unlock()

	return nil
}

// Returns clusterindex, balloonindex
func findBalloonInClusters(balloonID string) (clusterID string, err error) {
	for i, c := range clusters.Clusters {
		for b := range c.Balloons {
			if b == balloonID {
				clusterID = i
				return clusterID, nil
			}
		}
	}

	return "", fmt.Errorf("Balloon not found")
}

// Check if a configured balloon exists within the devicelist on TTN
func checkIfBalloonOnline(balloonID string, deviceList ttnsdk.DeviceList) bool {
	for _, device := range deviceList {
		if balloonID == device.DevID {
			return true
		}
	}

	return false
}

// Startup sets up the cluster
func Startup() {
	log := apex.Stdout() // We use a cli logger at Stdout
	log.MustParseLevel("debug")
	ttnlog.Set(log) // Set the logger as default for TTN

	jsonFile, err := os.Open("config.json")
	if err != nil {
		fmt.Println(err)
	}

	byteValue, _ := ioutil.ReadAll(jsonFile)

	var configjson map[string]string
	json.Unmarshal([]byte(byteValue), &configjson)
	jsonFile.Close()

	// Create a new SDK configuration for the public community network
	config := ttnsdk.NewCommunityConfig(sdkClientName)
	config.ClientVersion = "2.0.5" // The version of the application

	// Create a new SDK client for the application
	client = config.NewClient(appID, string(configjson["app_access_key"]))

	// Initialize Firebase connection
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
	ref := dbClient.NewRef("clusters")
	err = ref.Delete(context.Background())
	if err != nil {
		log.Fatalf("%v", err)
		return
	}

	// Initialize clusters list and mutex
	clusters = &gatewaytypes.ClusterList{}
	clustersMutex = sync.Mutex{}

	// Open our jsonFile
	jsonFile, err = os.Open("clusters.json")
	if err != nil {
		fmt.Println(err)
	}
	defer jsonFile.Close()

	byteValue, _ = ioutil.ReadAll(jsonFile)

	json.Unmarshal(byteValue, clusters)

	// Initialize clusters
	pushToFirebase(app)

	devices, err := client.ManageDevices()
	if err != nil {
		log.WithError(err).Fatal("riot-gateway: could not get device manager")
	}

	// List the first 10 devices
	deviceList, err := devices.List(10, 0)
	if err != nil {
		log.WithError(err).Fatal("riot-gateway: could not get devices")
	}
	log.Info("riot-gateway: found devices")
	for _, device := range deviceList {
		fmt.Printf("- %s\n", device.DevID)
	}

	wg := sync.WaitGroup{}

	for _, cluster := range clusters.Clusters {
		for balloonID := range cluster.Balloons {
			if checkIfBalloonOnline(balloonID, deviceList) {
				go balloonHandler(app, &wg, balloonID) // Spawn balloon handler
				wg.Add(1)
			} else {
				fmt.Printf("Balloon defined is not online\n")
			}
		}
	}

	wg.Wait() // Wait for all goroutines to finish
}

// CloseClient needs to be called when the TTN connection shall be closed
func CloseClient() {
	client.Close()
}
