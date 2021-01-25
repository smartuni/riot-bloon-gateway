package main

import (
	"fmt"
	"os"
	"riot-gateway/coapgateway"
	"riot-gateway/loragateway"
)

func main() {
	if os.Getenv("GATEWAY_TYPE") == "LORA" {
		loragateway.Startup()
		loragateway.CloseClient()
	} else if os.Getenv("GATEWAY_TYPE") == "COAP" {
		coapgateway.Startup()
	} else {
		fmt.Println("No gatewaytype supplied!\nusage:\n    GATEWAY_TYPE=LORA or\n    GATEWAY_TYPE=COAP\n    in envvars")
	}
}
