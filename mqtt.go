package mkmqtt

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// MQTTClient represents an MQTT client with connection options, TLS settings, and handlers.
type MQTTClient struct {
	client           mqtt.Client         // The underlying MQTT client
	opts             *mqtt.ClientOptions // Options for configuring the MQTT client
	TLS              TLS                 // TLS settings for secure connections
	Handlers         Handlers            // Custom handlers for connection events
	Timeout          time.Duration       // Timeout for operations
	Wg               sync.WaitGroup      // WaitGroup for managing goroutines
	subscriptions    map[string]byte     // Track active subscriptions
	subscriptionsMux sync.Mutex          // Mutex to protect subscription map
}

// TopicMessage represents a message to be published on a topic.
type TopicMessage struct {
	Topic   string      // MQTT topic
	OrderID int         // Order ID for message sequencing
	Qos     byte        // Quality of Service level
	Message interface{} // The message payload
}

// TLS settings for secure communication.
type TLS struct {
	useTLS         bool   // Flag to enable or disable TLS
	rootCAPath     string // Path to the root CA certificate
	clientCertPath string // Path to the client certificate
	clientKeyPath  string // Path to the client private key
}

// Handlers for various MQTT client events.
type Handlers struct {
	OnConnect        func(client mqtt.Client)                           // Handler for successful connection
	OnConnectionLost func(client mqtt.Client, err error)                // Handler for lost connection
	OnReconnecting   func(client mqtt.Client, opts *mqtt.ClientOptions) // Handler for reconnecting
}

// NewMQTTClient creates a new instance of MQTTClient with specified options.
func NewMQTTClient(brokerURL string, clientID string, username string, password string) *MQTTClient {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(brokerURL)
	opts.SetClientID(clientID)
	opts.SetUsername(username)
	opts.SetPassword(password)

	mqttClient := &MQTTClient{
		opts:          opts,
		subscriptions: make(map[string]byte),
	}

	return mqttClient
}

// Connect establishes a connection to the MQTT broker.
// Returns an error if the connection fails.
func (c *MQTTClient) Connect() error {
	c.client = mqtt.NewClient(c.opts)
	if token := c.client.Connect(); token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to connect to MQTT broker: %v", token.Error())
	}
	return nil
}

// DisconnectMQTT disconnects the MQTT client from the broker.
func (c *MQTTClient) DisconnectMQTT() {
	c.client.Disconnect(250) // Graceful disconnection with a 250ms timeout
}

// IsConnected checks if the client is currently connected to the broker.
// Returns true if connected, otherwise false.
func (c *MQTTClient) IsConnected() bool {
	return c.client.IsConnected()
}

// SetOnConnect sets a custom handler for the OnConnect event.
// handler: Function to be called when the client successfully connects.
func (c *MQTTClient) SetOnConnect(handler func(client mqtt.Client)) *MQTTClient {
	c.Handlers.OnConnect = handler
	return c
}

// SetOnConnectionLost sets a custom handler for the OnConnectionLost event.
// handler: Function to be called when the connection to the broker is lost.
func (c *MQTTClient) SetOnConnectionLost(handler func(client mqtt.Client, err error)) *MQTTClient {
	c.Handlers.OnConnectionLost = handler
	return c
}

// SetOnReconnecting sets a custom handler for the OnReconnecting event.
// handler: Function to be called when the client is attempting to reconnect.
func (c *MQTTClient) SetOnReconnecting(handler func(client mqtt.Client, opts *mqtt.ClientOptions)) *MQTTClient {
	c.Handlers.OnReconnecting = handler
	return c
}

// EnableTLS enables TLS with the given certificates and keys.
// rootCAPath: Path to the root CA certificate.
// clientCertPath: Path to the client certificate.
// clientKeyPath: Path to the client key.
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

// SetDefaultHandlers sets default handlers for common MQTT client events.
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

// SetTimeout sets the timeout duration for operations.
// timeout: Timeout duration as a string (e.g., "10s" for 10 seconds).
// Returns the MQTTClient instance for chaining.
func (c *MQTTClient) SetTimeout(timeout string) *MQTTClient {
	time, err := time.ParseDuration(timeout)
	if err != nil {
		fmt.Printf("Failed to parse timeout: %v\n", err)
		return c
	}
	c.Timeout = time
	return c
}

// SetLastWill sets the last will message for the client.
// topic: The topic to which the last will message will be published.
// message: The last will message (must be a string).
// qos: Quality of Service level for the last will message.
// retain: Whether the last will message should be retained by the broker.
// Returns an error if the message is not a string.
func (c *MQTTClient) SetLastWill(topic string, message interface{}, qos byte, retain bool) error {
	msgStr, ok := message.(string)
	if !ok {
		return fmt.Errorf("message must be of type string")
	}

	c.opts.SetWill(topic, msgStr, qos, retain)
	return nil
}

// SetKeepAlive sets the keep-alive interval for the client.
// duration: Keep-alive interval as a string (e.g., "30s" for 30 seconds).
// Returns the MQTTClient instance for chaining.
func (c *MQTTClient) SetKeepAlive(duration string) *MQTTClient {
	keepAlive, err := time.ParseDuration(duration)
	if err != nil {
		fmt.Printf("Failed to parse keep-alive duration: %v\n", err)
		return c
	}
	c.opts.SetKeepAlive(keepAlive)
	return c
}

// tryToReconnect attempts to reconnect to the MQTT broker with exponential backoff.
func (c *MQTTClient) TryToReconnect() {
	for !c.client.IsConnected() {
		fmt.Println("Attempting to reconnect to MQTT broker...")
		if token := c.client.Connect(); token.Wait() && token.Error() != nil {
			fmt.Printf("Failed to reconnect: %v. Retrying...\n", token.Error())
			time.Sleep(5 * time.Second)
		} else {
			fmt.Println("Reconnected to MQTT broker")
			break
		}
	}
}

// SetupHandlers sets up the connection handlers for the MQTT client.
// These handlers manage connection events such as successful connection,
// connection loss, and reconnecting attempts.
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

// configureTLS configures TLS settings if TLS is enabled.
// This method sets up the TLS configuration, loading the root CA,
// client certificate, and client key.
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
