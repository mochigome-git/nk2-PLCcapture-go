package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/future-architect/go-mcprotocol/mcp"
	"github.com/joho/godotenv"
)

func init() {
	err := godotenv.Load(".env.local.32")
	if err != nil {
		log.Fatalf("Error loading .env.local file: %v", err)
	}
}

func main() {

	// Set up a channel to listen for SIGTERM signals
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)

	// Use os.Getenv() instead of getEnv()
	mqttHost := os.Getenv("MQTT_HOST")
	plcHost := os.Getenv("PLC_HOST")
	plcPort := getEnvAsInt32bit("PLC_PORT", 5011)

	// Create a logger to use for logging messages
	logger := log.New(os.Stdout, "", log.LstdFlags)

	// Connect to the MQTT server
	mqttclient := MQTT.NewClient(MQTT.NewClientOptions().AddBroker(mqttHost))
	if token := mqttclient.Connect(); token.Wait() && token.Error() != nil {
		logger.Fatalf("Error connecting to MQTT server: %s", token.Error())
	} else {
		logger.Printf("Connected to MQTT server %s successfully", os.Getenv("MQTT_HOST"))
	}
	defer mqttclient.Disconnect(250)

	// Parse the addresses to read from the environment variable
	deviceStrings := strings.Split(os.Getenv("DEVICES"), ",")
	if len(deviceStrings)%3 != 0 {
		logger.Fatalf("Invalid DEVICES environment variable: %s", os.Getenv("DEVICES"))
	}
	var devices []struct {
		deviceType      string
		deviceNumber    uint16
		numberRegisters uint16
	}
	for i := 0; i < len(deviceStrings); i += 3 {
		deviceNumber, err := strconv.ParseUint(deviceStrings[i+1], 10, 16)
		if err != nil {
			logger.Fatalf("Invalid device number in DEVICES environment variable: %s", deviceStrings[i+1])
		}
		numberRegisters, err := strconv.ParseUint(deviceStrings[i+2], 10, 16)
		if err != nil {
			logger.Fatalf("Invalid number of registers in DEVICES environment variable: %s", deviceStrings[i+2])
		}
		devices = append(devices, struct {
			deviceType      string
			deviceNumber    uint16
			numberRegisters uint16
		}{
			deviceType:      deviceStrings[i],
			deviceNumber:    uint16(deviceNumber),
			numberRegisters: uint16(numberRegisters),
		})
	}
	if len(devices) == 0 {
		logger.Fatalf("No devices found in DEVICES environment variable")
	} else {
		logger.Printf("Loaded %d device(s)", len(devices))
	}

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

	// Store the previous data so we can check if it has changed
	prevData := make([]uint32, len(devices))

	log.Printf("Start collecting data from %s", plcHost)

	// Use a goroutine to run the main loop
	go func() {
		for {
			// Connect to the PLC with MC protocol
			client, err := mcp.New3EClient(plcHost, plcPort, mcp.NewLocalStation())
			if err != nil {
				logger.Printf("Error connecting to PLC: %s", err)
				time.Sleep(5 * time.Second)
				continue
			}

			// Check if the shutdown variable is true before connecting to the PLC
			if shutdown {
				break
			}

			// Read data from the PLC
			for j, device := range devices {
				data, err := client.Read(device.deviceType, int64(device.deviceNumber), int64(device.numberRegisters))
				if err == nil {
					registerBinary, _ := mcp.NewParser().Do(data)
					data := registerBinary.Payload
					var value uint32
					for i := 0; i < len(data); i++ {
						value |= uint32(data[i]) << uint(8*i)
					}
					// Only write to the MQTT server if the data has changed
					if prevData[j] != value {

						floatValue := math.Float32frombits(value)
						floatString := fmt.Sprintf("%.6f", floatValue)
						firstSixDigits := ""
						numDigits := 0
						for _, c := range floatString {
							if c == '-' || c == '.' {
								// Include minus sign and decimal point
								firstSixDigits += string(c)
							} else if numDigits < 6 {
								// Only include the first 6 digits
								firstSixDigits += string(c)
								numDigits++
							}
						}

						// Prepare the message to send to the MQTT server
						message := map[string]interface{}{
							"address": device.deviceType + strconv.Itoa(int(device.deviceNumber)),
							"value":   firstSixDigits,
						}

						// Convert the message to a JSON string
						messageJSON, err := json.Marshal(message)
						if err != nil {
							logger.Printf("Error marshaling message to JSON:%s", err)
							continue
						}

						// Publish the message to the MQTT server
						topic := "plc/holding_register/" + device.deviceType + strconv.Itoa(int(device.deviceNumber))
						if token := mqttclient.Publish(topic, 0, false, messageJSON); token.Wait() && token.Error() != nil {
							logger.Printf("Error publishing message to MQTT server: %s", token.Error())
						} else {
							// Update the previous data with the new value
							prevData[j] = value
						}
					}
				} else {
					logger.Printf("Error reading from PLC: %s", err)
					time.Sleep(5 * time.Second)
				}
			}
		}
		// Sleep for 1 second before reading from the PLC again
		time.Sleep(1 * time.Second)
	}()
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

// getEnv gets an environment variable with the given name or returns the default value if it is not set
func getEnv32bit(name, defaultValue string) string {
	if value, exists := os.LookupEnv(name); exists {
		return value
	}
	return defaultValue
}

// getEnvAsInt gets an environment variable with the given name as an integer or returns the default value if it is not set
func getEnvAsInt32bit(name string, defaultValue int) int {
	if value, exists := os.LookupEnv(name); exists {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}
