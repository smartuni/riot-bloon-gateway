package loragateway

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"riot-gateway/gatewaytypes"

	ttnsdk "github.com/TheThingsNetwork/go-app-sdk"
	ttnlog "github.com/TheThingsNetwork/go-utils/log"
	"github.com/TheThingsNetwork/go-utils/log/apex"

	cbor "github.com/fxamacker/cbor/v2"
)

const (
	sdkClientName = "riot-gateway"
	appID         = "riot_balloon_control_2020"
	appAccessKey  = "ttn-account-v2.DW_xQtirzRPgUhcKeRVF8LtCfn2886M1o9jMqfSwAOs"
)

var client ttnsdk.Client // The LORA client

// BalloonHandler handles a single balloon as a thread
// TODO: - connect to TTN
// 	     - subsribe to uplink
//       - push data to firebase structure
func BalloonHandler(balloonID string) {}

// PushToFireBase handles pushing stuff to the database
// TODO: - handle multithreaded calls
//       - update Firebase with data (refer to main.go for Firebase)
func PushToFireBase(cluster *gatewaytypes.Cluster, balloon *gatewaytypes.Balloon) {}

// UpdateCluster updates a given cluster by calculating the new averages
// should be called once new data from a balloon arrives
// TODO: - handle multithreaded calls
//       - update Firebase with data (refer to main.go for Firebase)
func UpdateCluster(cluster *gatewaytypes.Cluster) {}

// Setup sets up the cluster
func Setup() {
	setup := &gatewaytypes.ClusterList{}

	// Open our jsonFile
	jsonFile, err := os.Open("clusters.json")
	if err != nil {
		fmt.Println(err)
	}
	defer jsonFile.Close()

	byteValue, _ := ioutil.ReadAll(jsonFile)

	json.Unmarshal(byteValue, setup)

	for _, cluster := range *setup.Clusters {
		for _, balloon := range *cluster.Balloons {
			go BalloonHandler(balloon.ID) // Spawn balloon handler
			log.Println(balloon.ID)
		}
	}
}

// Run runs the LORA gateway
func Run() {
	log := apex.Stdout() // We use a cli logger at Stdout
	log.MustParseLevel("debug")
	ttnlog.Set(log) // Set the logger as default for TTN

	// Create a new SDK configuration for the public community network
	config := ttnsdk.NewCommunityConfig(sdkClientName)
	config.ClientVersion = "2.0.5" // The version of the application

	// Create a new SDK client for the application
	client = config.NewClient(appID, appAccessKey)

	// Make sure the client is closed before the function returns
	// In your application, you should call this before the application shuts down
	// defer client.Close()

	// Manage devices for the application.
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

	// Start Publish/Subscribe client (MQTT)
	pubsub, err := client.PubSub()
	if err != nil {
		log.WithError(err).Fatal("riot-gateway: could not get application pub/sub")
	}

	// Make sure the pubsub client is closed before the function returns
	// In your application, you should call this before the application shuts down
	defer pubsub.Close()

	// Get a publish/subscribe client for all devices
	// allDevicesPubSub := pubsub.AllDevices()

	// Get a publish/subscribe client scoped to my-test-device
	myNewDevicePubSub := pubsub.Device("test_device")

	// Make sure the pubsub client for this device is closed before the function returns
	// In your application, you will probably call this when you no longer need the device
	// This also stops existing subscriptions, in case you forgot to unsubscribe
	defer myNewDevicePubSub.Close()

	// Subscribe to uplink messages
	uplink, err := myNewDevicePubSub.SubscribeUplink()
	if err != nil {
		log.WithError(err).Fatal("riot-gateway: could not subscribe to uplink messages")
	}
	log.Debug("After this point, the program won't show anything until we receive an uplink message from device my-new-device.")
	for message := range uplink {
		v := &map[string]float64{}
		err := cbor.Unmarshal(message.PayloadRaw, &v)
		if err != nil {
			log.Errorf("%v\n", err)
		}
		log.WithField("data", v).Info("riot-gateway: received uplink")
		break // normally you wouldn't do this
	}
}

// CloseClient needs to be called when the TTN connection shall be closed
func CloseClient() {
	client.Close()
}
