package main

import (
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"
)

func TestMain(t *testing.T) {
	// Save the original values of the environment variables
	originalMqttHost := os.Getenv("MQTT_HOST")
	originalPlcHost := os.Getenv("PLC_HOST")
	originalPlcPort := os.Getenv("PLC_PORT")
	originalDevices16 := os.Getenv("DEVICES_16bit")
	originalDevices32 := os.Getenv("DEVICES_32bit")

	// Set the necessary environment variables for testing
	os.Setenv("MQTT_HOST", "test_mqtt_host")
	os.Setenv("PLC_HOST", "test_plc_host")
	os.Setenv("PLC_PORT", "1234")
	os.Setenv("DEVICES_16bit", "device1,device2")
	os.Setenv("DEVICES_32bit", "device3,device4")

	// Create a channel to listen for SIGINT signals
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT)

	// Start your main function in a separate goroutine
	go main()

	// Wait for a short duration to allow the program to run
	time.Sleep(2 * time.Second)

	// Send a SIGINT signal to stop the program
	signalCh <- syscall.SIGINT

	// Wait for a short duration to allow the program to gracefully exit
	time.Sleep(2 * time.Second)

	// Perform your assertions or additional test steps here
	// ...

	// Cleanup any resources and restore the original environment variables
	os.Setenv("MQTT_HOST", originalMqttHost)
	os.Setenv("PLC_HOST", originalPlcHost)
	os.Setenv("PLC_PORT", originalPlcPort)
	os.Setenv("DEVICES_16bit", originalDevices16)
	os.Setenv("DEVICES_32bit", originalDevices32)
}
