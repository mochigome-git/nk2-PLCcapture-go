package mqtt2

import (
	"log"

	MQTT "github.com/eclipse/paho.mqtt.golang"
)

type MQTTClient struct {
	client MQTT.Client
}

func NewMQTTClient(mqttHost string, logger *log.Logger) (*MQTTClient, error) {
	opts := MQTT.NewClientOptions().AddBroker(mqttHost)
	client := MQTT.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return nil, token.Error()
	}

	logger.Printf("Connected to MQTT server %s successfully", mqttHost)

	mqttClient := &MQTTClient{
		client: client,
	}
	return mqttClient, nil
}

func (m *MQTTClient) PublishMessage(topic string, message string, logger *log.Logger) error {
	token := m.client.Publish(topic, 0, false, message)
	token.Wait()
	if token.Error() != nil {
		logger.Printf("Error publishing message to topic %s: %s", topic, token.Error())
		return token.Error()
	}

	logger.Printf("Published message to topic %s: %s", topic, message)
	return nil
}

func (m *MQTTClient) Disconnect(timeout uint) {
	m.client.Disconnect(timeout)
}
