package main

import (
	"context"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"nk2-PLCcapture-go/pkg/config"
	"nk2-PLCcapture-go/pkg/mqtt2"
	"nk2-PLCcapture-go/pkg/plc"
	"nk2-PLCcapture-go/pkg/utils"

	jsoniter "github.com/json-iterator/go"
)

var (
	shutdown         bool
	devicesReadCount uint32
)

func main() {
	config.LoadEnv(".env.local")

	mqttHost := os.Getenv("MQTT_HOST")
	plcHost := os.Getenv("PLC_HOST")
	plcPort := config.GetEnvAsInt("PLC_PORT", 5011)
	devices16 := os.Getenv("DEVICES_16bit")
	devices32 := os.Getenv("DEVICES_32bit")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)

	logger := log.New(os.Stdout, "", log.LstdFlags)

	mqttclient, err := mqtt2.NewMQTTClient(mqttHost, logger)
	if err != nil {
		logger.Printf("Error mqtt connecting: %s", err)
	}
	defer mqttclient.Disconnect(250)

	devices16Parsed, err := utils.ParseDeviceAddresses(devices16, logger)
	if err != nil {
		logger.Fatalf("Error parsing device addresses: %v", err)
	}

	devices32Parsed, err := utils.ParseDeviceAddresses(devices32, logger)
	if err != nil {
		logger.Fatalf("Error parsing device addresses: %v", err)
	}

	devices := append(devices16Parsed, devices32Parsed...)

	go func() {
		for sig := range signalCh {
			if sig == syscall.SIGINT || sig == syscall.SIGTERM {
				shutdown = true
				cancel()
				logger.Println("Program terminated by signal")
				break
			}
		}
	}()

	err = plc.InitMSPClient(plcHost, plcPort)
	if err != nil {
		logger.Fatalf("Failed to initialize MSP client: %v", err)
	} else {
		log.Printf("Start collecting data from %s", plcHost)
	}

	workerCount := 15
	dataCh := make(chan map[string]interface{}, workerCount)
	var wg sync.WaitGroup

	go startWorkers(ctx, workerCount, dataCh, mqttclient, logger, &wg)

	go readDataFromDevices(ctx, devices, dataCh, &wg, logger)

	select {
	case <-signalCh:
		logger.Println("Program terminated by signal")
		shutdown = true
	case <-ctx.Done():
		logger.Println("All devices read and sent to MQTT server")
	}

	cancel()
	close(dataCh)

	// Restart the program if needed
	if len(devices) != len(devices32Parsed)+len(devices16Parsed) {
		logger.Printf("Number of devices read (%d) does not match the number of devices listed in DEVICES_32bit (%d) and DEVICES_16bit (%d). Restarting the program...", len(devices), len(devices32Parsed), len(devices16Parsed))
		restartProgram()
	}

	mqttclient.Disconnect(250)
	logger.Println("Program exited")
}

func startWorkers(ctx context.Context, workerCount int, dataCh <-chan map[string]interface{}, mqttclient *mqtt2.MQTTClient, logger *log.Logger, wg *sync.WaitGroup) {
	for i := 0; i < workerCount; i++ {
		wg.Add(workerCount)
		go func() {
			defer wg.Done()

			for message := range dataCh {
				messageJSON, err := jsoniter.Marshal(message)
				if err != nil {
					logger.Printf("Error marshaling message to JSON: %s", err)
					continue
				}
				topic := "testplc/holding_register/16bit&32bit/" + message["address"].(string)
				err = mqttclient.PublishMessage(topic, string(messageJSON), logger)
				if err != nil {
					logger.Printf("Error publishing message: %s", err)
				}
			}
		}()
	}
	wg.Wait()
}

func readDataFromDevices(ctx context.Context, devices []utils.Device, dataCh chan<- map[string]interface{}, wg *sync.WaitGroup, logger *log.Logger) {
	devicesReadCount = 0

	for _, device := range devices {
		wg.Add(1)
		go func(device utils.Device) {
			defer wg.Done()

			for {
				value, err := plc.ReadData(device.DeviceType, device.DeviceNumber, device.NumberRegisters)
				if err != nil {
					logger.Printf("Error reading data from PLC for device %s: %s", device.DeviceType+strconv.Itoa(int(device.DeviceNumber)), err)
					time.Sleep(1 * time.Second)
					continue
				}

				message := make(map[string]interface{})
				message["address"] = device.DeviceType + strconv.Itoa(int(device.DeviceNumber))
				message["value"] = value

				select {
				case <-ctx.Done():
					return
				case dataCh <- message:
					count := atomic.AddUint32(&devicesReadCount, 1)
					log.Println(count)

					if count == uint32(len(devices)) {
						atomic.StoreUint32(&devicesReadCount, 0)
						return
					}

					if shutdown {
						break
					}

				}
			}
		}(device)
	}

	wg.Wait()
	close(dataCh)
}

func restartProgram() {
	log.Println("Restarting the program...")

	executable, err := os.Executable()
	if err != nil {
		log.Fatalf("Failed to get the executable path: %s", err)
	}

	// Fork the current process
	cmd := exec.Command(executable, os.Args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	// Start the new process
	err = cmd.Start()
	if err != nil {
		log.Fatalf("Failed to restart the program: %s", err)
	}

	// Terminate the current process
	os.Exit(0)
}
