# MKMQTT

**MKMQTT** is a comprehensive MQTT client library for Go, designed to provide robust and efficient communication with MQTT brokers. The library is built on top of the popular [Paho MQTT Go](https://github.com/eclipse/paho.mqtt.golang) library and offers additional functionalities and improvements.

## Features

-   **Connection Management**: Easily connect to MQTT brokers with customizable options including client ID, username, and password.
-   **TLS Support**: Enable or disable TLS with specified certificates and keys for secure communication.
-   **Event Handlers**: Set custom handlers for various MQTT events:
    -   `OnConnect`: Triggered when the client successfully connects to the broker.
    -   `OnConnectionLost`: Triggered when the connection to the broker is lost.
    -   `OnReconnecting`: Triggered when the client is attempting to reconnect to the broker.
-   **Message Publishing**:
    -   Publish individual messages to specified topics.
    -   Publish raw messages with byte payloads.
    -   Batch publish messages with concurrency support, allowing efficient parallel publishing.
-   **Message Subscription**: Subscribe and unsubscribe from topics with specified QoS levels.
-   **Reconnection Logic**: Automatic reconnection logic with exponential backoff for robust and resilient connectivity.
-   **Concurrency Control**: Utilize worker goroutines and wait groups to manage concurrent message publishing efficiently.

## Components

### MQTTClient

The main struct that manages the MQTT client, connection options, TLS settings, and event handlers.

#### Fields:

-   `client`: The underlying Paho MQTT client.
-   `opts`: MQTT client options.
-   `TLS`: TLS settings for secure communication.
-   `Handlers`: Customizable event handlers.
-   `Wg`: Wait group for managing concurrency.

### TopicMessage

A struct representing a message to be published, including the topic and the message payload.

#### Fields:

-   `Topic`: The MQTT topic to publish the message to.
-   `Message`: The message payload to be published.

### TLS

A struct to manage TLS settings, including paths to root CA, client certificate, and client key.

#### Fields:

-   `useTLS`: A boolean indicating whether TLS is enabled.
-   `rootCAPath`: Path to the root CA certificate.
-   `clientCertPath`: Path to the client certificate.
-   `clientKeyPath`: Path to the client key.

### Handlers

A struct to manage custom event handlers for MQTT events.

#### Fields:

-   `OnConnect`: Handler for the connect event.
-   `OnConnectionLost`: Handler for the connection lost event.
-   `OnReconnecting`: Handler for the reconnecting event.

## Summary

For detailed usage instructions and examples, please refer to the [Wiki](https://github.com/SHEP4RDO/mkmqtt/wiki).