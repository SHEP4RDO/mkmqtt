package mkmqtt

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type MQTTClient struct {
	client   mqtt.Client
	opts     *mqtt.ClientOptions
	TLS      TLS
	Handlers Handlers
	Wg       sync.WaitGroup
}

type TopicMessage struct {
	Topic   string
	Message interface{}
}

// TLS settings
type TLS struct {
	useTLS         bool
	rootCAPath     string
	clientCertPath string
	clientKeyPath  string
}

type Handlers struct {
	OnConnect        func(client mqtt.Client)
	OnConnectionLost func(client mqtt.Client, err error)
	OnReconnecting   func(client mqtt.Client, opts *mqtt.ClientOptions)
}

// NewMQTTClient creates a new instance of MQTTClient with specified options.
func NewMQTTClient(brokerURL string, clientID string, username string, password string) *MQTTClient {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(brokerURL)
	opts.SetClientID(clientID)
	opts.SetUsername(username)
	opts.SetPassword(password)

	mqttClient := &MQTTClient{
		opts: opts,
	}

	return mqttClient
}

// EnableTLS enables TLS with given certificates and keys.
func (c *MQTTClient) EnableTLS(rootCAPath, clientCertPath, clientKeyPath string) *MQTTClient {
	c.TLS.useTLS = true
	c.TLS.rootCAPath = rootCAPath
	c.TLS.clientCertPath = clientCertPath
	c.TLS.clientKeyPath = clientKeyPath

	c.configureTLS()
	return c
}

// DisableTLS disables TLS.
func (c *MQTTClient) DisableTLS() *MQTTClient {
	c.TLS.useTLS = false
	c.TLS.rootCAPath = ""
	c.TLS.clientCertPath = ""
	c.TLS.clientKeyPath = ""
	return c
}

// SetOnConnect sets a custom OnConnect handler.
func (c *MQTTClient) SetOnConnect(handler func(client mqtt.Client)) *MQTTClient {
	c.Handlers.OnConnect = handler
	return c
}

// SetOnConnectionLost sets a custom OnConnectionLost handler.
func (c *MQTTClient) SetOnConnectionLost(handler func(client mqtt.Client, err error)) *MQTTClient {
	c.Handlers.OnConnectionLost = handler
	return c
}

// SetOnReconnecting sets a custom OnReconnecting handler.
func (c *MQTTClient) SetOnReconnecting(handler func(client mqtt.Client, opts *mqtt.ClientOptions)) *MQTTClient {
	c.Handlers.OnReconnecting = handler
	return c
}

// SetDefaultHandlers sets default handlers for MQTTClient.
func (c *MQTTClient) SetDefaultHandlers() {
	// Default handler for OnConnect
	c.Handlers.OnConnect = func(client mqtt.Client) {
		fmt.Println("Connected to MQTT broker")
		if c.Handlers.OnConnect != nil {
			c.Handlers.OnConnect(client)
		}
	}

	// Default handler for OnConnectionLost
	c.Handlers.OnConnectionLost = func(client mqtt.Client, err error) {
		fmt.Printf("Connection lost: %v\n", err)
		c.TryToReconnect()
	}

	// Default handler for OnReconnecting
	c.Handlers.OnReconnecting = func(client mqtt.Client, opts *mqtt.ClientOptions) {
		fmt.Println("Reconnecting to MQTT broker...")
		if c.Handlers.OnReconnecting != nil {
			c.Handlers.OnReconnecting(client, opts)
		}
	}
}

// Connect connects to the MQTT broker.
func (c *MQTTClient) Connect() error {
	c.client = mqtt.NewClient(c.opts)
	if token := c.client.Connect(); token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to connect to MQTT broker: %v", token.Error())
	}
	return nil
}

func (c *MQTTClient) PublishMessage(topic string, msg interface{}, qos byte) error {
	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %v", err)
	}
	token := c.client.Publish(topic, qos, false, payload)
	token.Wait()
	if token.Error() != nil {
		return fmt.Errorf("failed to publish message: %v", token.Error())
	}
	return nil
}

func (c *MQTTClient) PublishRawMessage(topic string, payload []byte, qos byte) error {
	token := c.client.Publish(topic, qos, false, payload)
	token.Wait()
	if token.Error() != nil {
		return fmt.Errorf("failed to publish message: %v", token.Error())
	}
	return nil
}

func (c *MQTTClient) SubscribeToTopic(topic string, qos byte, messageHandler mqtt.MessageHandler) error {
	if token := c.client.Subscribe(topic, qos, messageHandler); token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to subscribe to topic: %v", token.Error())
	}
	return nil
}

func (c *MQTTClient) UnsubscribeFromTopic(topic string) error {
	if token := c.client.Unsubscribe(topic); token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to unsubscribe from topic: %v", token.Error())
	}
	return nil
}

func (c *MQTTClient) DisconnectMQTT() {
	c.client.Disconnect(250)
}

// tryToReconnect attempts to reconnect to the MQTT broker with exponential backoff.
func (c *MQTTClient) TryToReconnect() {
	for !c.client.IsConnected() {
		fmt.Println("Attempting to reconnect to MQTT broker...")
		if token := c.client.Connect(); token.Wait() && token.Error() != nil {
			fmt.Printf("Failed to reconnect: %v. Retrying...\n", token.Error())
			time.Sleep(5 * time.Second) // Exponential backoff or other strategy
		} else {
			fmt.Println("Reconnected to MQTT broker")
			break
		}
	}
}

// setupHandlers sets up the connection handlers for MQTT client.
func (c *MQTTClient) SetupHandlers() {
	c.opts.OnConnect = func(client mqtt.Client) {
		fmt.Println("Connected to MQTT broker")
		if c.Handlers.OnConnect != nil {
			c.Handlers.OnConnect(client)
		}
	}
	c.opts.OnConnectionLost = func(client mqtt.Client, err error) {
		fmt.Printf("Connection lost: %v\n", err)
		if c.Handlers.OnConnectionLost != nil {
			c.Handlers.OnConnectionLost(client, err)
		}
	}
	c.opts.OnReconnecting = func(client mqtt.Client, opts *mqtt.ClientOptions) {
		fmt.Println("Reconnecting to MQTT broker...")
		if c.Handlers.OnReconnecting != nil {
			c.Handlers.OnReconnecting(client, opts)
		}
	}
}

func (c *MQTTClient) BatchPublishMessagesWithConcurrency(messages map[string]TopicMessage, qos byte, numWorkers int) error {
	messageChannel := make(chan TopicMessage, len(messages))

	// Start worker goroutines
	for i := 0; i < numWorkers; i++ {
		c.Wg.Add(1)
		go c.topicWorker(messageChannel, qos)
	}

	// Send messages to the channel
	for _, msg := range messages {
		messageChannel <- msg
	}

	close(messageChannel)
	c.Wg.Wait()

	fmt.Println("Sended", len(messages))
	return nil
}

func (c *MQTTClient) topicWorker(messageChannel chan TopicMessage, qos byte) {
	defer c.Wg.Done()
	for msg := range messageChannel {
		payload, err := json.Marshal(msg.Message)
		if err != nil {
			fmt.Printf("Failed to marshal message for topic '%s': %v\n", msg.Topic, err)
			continue
		}

		token := c.client.Publish(msg.Topic, qos, false, payload)
		token.Wait()
		if token.Error() != nil {
			fmt.Printf("Failed to publish message for topic '%s': %v\n", msg.Topic, token.Error())
			continue
		}

		time.Sleep(100 * time.Millisecond)
	}
}

// configureTLS configures TLS settings if enabled.
func (c *MQTTClient) configureTLS() {
	if c.TLS.useTLS {
		tlsConfig := &tls.Config{}

		// Load root CA
		rootCA, err := ioutil.ReadFile(c.TLS.rootCAPath)
		if err == nil {
			rootCAs := x509.NewCertPool()
			rootCAs.AppendCertsFromPEM(rootCA)
			tlsConfig.RootCAs = rootCAs
		}

		// Load client certificate and key
		if c.TLS.clientCertPath != "" && c.TLS.clientKeyPath != "" {
			cert, err := tls.LoadX509KeyPair(c.TLS.clientCertPath, c.TLS.clientKeyPath)
			if err == nil {
				tlsConfig.Certificates = []tls.Certificate{cert}
			}
		}

		c.opts.SetTLSConfig(tlsConfig)
	}
}
