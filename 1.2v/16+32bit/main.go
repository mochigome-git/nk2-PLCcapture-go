package main

import (
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"16+32bit/pkg/config"
	"16+32bit/pkg/mqtt"
	"16+32bit/pkg/plc"
	"16+32bit/pkg/utils"

	jsoniter "github.com/json-iterator/go"
)

func main() {
	config.LoadEnv(".env.local")

	// Use os.Getenv() instead of getEnv()
	mqttHost := os.Getenv("MQTT_HOST")
	plcHost := os.Getenv("PLC_HOST")
	plcPort := config.GetEnvAsInt("PLC_PORT", 5011)
	devices16 := os.Getenv("DEVICES_16bit")
	devices32 := os.Getenv("DEVICES_32bit")

	// Set up a channel to listen for SIGTERM signals
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)

	// Create a logger to use for logging messages
	logger := log.New(os.Stdout, "", log.LstdFlags)

	// Connect to the MQTT server
	mqttclient := mqtt.NewMQTTClient(mqttHost, logger)
	defer mqttclient.Disconnect(250)

	// Parse the device addresses for 16-bit devices
	devices16Parsed, err := utils.ParseDeviceAddresses(devices16, logger)
	if err != nil {
		logger.Fatalf("Error parsing device addresses: %v", err)
	}

	// Parse the device addresses for 32-bit devices
	devices32Parsed, err := utils.ParseDeviceAddresses(devices32, logger)
	if err != nil {
		logger.Fatalf("Error parsing device addresses: %v", err)
	}

	// Combine the 16-bit and 32-bit devices into a single slice
	devices := append(devices16Parsed, devices32Parsed...)

	// Create a channel to signal when the main loop has finished
	doneCh := make(chan struct{})

	// Create a boolean variable to indicate whether to shutdown
	shutdown := false

	// Start a goroutine to listen for signals
	go func() {
		// Iterate over the signal channel
		for sig := range signalCh {
			// If a SIGINT or SIGTERM signal is received, set the shutdown variable to true
			if sig == syscall.SIGINT || sig == syscall.SIGTERM {
				shutdown = true
				break
			}
		}
	}()

	log.Printf("Start collecting data from %s", plcHost)

	// Use a goroutine to run the main loop
	// Create a buffered channel to store the data to be processed
	dataCh := make(chan map[string]interface{}, 100)

	go func() {
		for {
			for _, device := range devices {
				// Read data from the PLC
				value, err := plc.ReadData(device.DeviceType, device.DeviceNumber, device.NumberRegisters, plcHost, plcPort)
				if err != nil {
					logger.Printf("Error reading data from PLC for device %s: %s", device.DeviceType+strconv.Itoa(int(device.DeviceNumber)), err)
					continue
				}

				// Prepare the message to send to the MQTT server
				message := map[string]interface{}{
					"address": device.DeviceType + strconv.Itoa(int(device.DeviceNumber)),
					"value":   value,
				}
				dataCh <- message
			}

			// Check if the shutdown variable is true before reconnecting to the PLC
			if shutdown {
				break
			}

			time.Sleep(5 * time.Second)
		}
	}()

	// Spawn multiple worker goroutines that read the data from the channel, process it, and send it to MQTT
	for i := 0; i < 15; i++ {
		go func() {
			for message := range dataCh {

				// Convert the message to a JSON string
				messageJSON, err := jsoniter.Marshal(message)
				if err != nil {
					logger.Printf("Error marshaling message to JSON:%s", err)
					continue
				}

				// Publish the message to the MQTT server
				topic := "ttestplc/holding_register/" + message["address"].(string)
				mqtt.PublishMessage(mqttclient, topic, string(messageJSON), logger)

			}
		}()
	}

	// Wait for either the main loop to finish or a signal to be received
	select {
	case <-signalCh:
		logger.Println("Program terminated by signal")
		shutdown = true
	}

	// Perform any necessary cleanup tasks and exit the program
	logger.Println("Exiting program...")
	// Disconnect from the MQTT server
	defer close(signalCh)
	defer close(doneCh)
	mqttclient.Disconnect(250)
}
