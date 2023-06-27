package main

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestMain(t *testing.T) {
	// Override environment variables for testing
	os.Setenv("MQTT_HOST", "test-mqtt-host")
	os.Setenv("PLC_HOST", "test-plc-host")
	os.Setenv("PLC_PORT", "1234")
	os.Setenv("DEVICES_16bit", "device1,device2,device3")
	os.Setenv("DEVICES_32bit", "device4,device5,device6")

	// Create a context with a timeout for the test
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start the main program in a goroutine
	go func() {
		main()
	}()

	// Wait for the program to finish or the timeout to elapse
	select {
	case <-ctx.Done():
		t.Fatal("Test timed out")
	}
}
