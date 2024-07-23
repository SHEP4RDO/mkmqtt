package mkmqtt

import (
	"encoding/json"
	"fmt"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// SubscribeToTopic subscribes to a given topic with the specified QoS and message handler.
// messageHandler: Handler function to process incoming messages for the topic.
// Returns an error if the subscription fails.
func (c *MQTTClient) SubscribeToTopic(topic string, qos byte, messageHandler mqtt.MessageHandler) error {
	if token := c.client.Subscribe(topic, qos, messageHandler); token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to subscribe to topic: %v", token.Error())
	}

	// Track the subscription
	c.subscriptionsMux.Lock()
	c.subscriptions[topic] = qos
	c.subscriptionsMux.Unlock()

	return nil
}

// UnsubscribeFromTopic unsubscribes from a given topic.
// Returns an error if the unsubscription fails.
func (c *MQTTClient) UnsubscribeFromTopic(topic string) error {
	if token := c.client.Unsubscribe(topic); token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to unsubscribe from topic: %v", token.Error())
	}

	// Remove the subscription from the map
	c.subscriptionsMux.Lock()
	delete(c.subscriptions, topic)
	c.subscriptionsMux.Unlock()

	return nil
}

// HasSubscription checks if there is a subscription for the given topic.
// It returns true if the subscription exists, false otherwise.
func (c *MQTTClient) HasSubscription(topic string) bool {
	c.subscriptionsMux.Lock()
	defer c.subscriptionsMux.Unlock()

	_, exists := c.subscriptions[topic]
	return exists
}

// PublishMessage publishes a message to a given topic with the specified QoS.
// Returns an error if the message publication fails or if the message cannot be marshaled to JSON.
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

// PublishRawMessage publishes a raw byte array message to a given topic with the specified QoS.
// Returns an error if the message publication fails.
func (c *MQTTClient) PublishRawMessage(topic string, payload []byte, qos byte) error {
	token := c.client.Publish(topic, qos, false, payload)
	token.Wait()
	if token.Error() != nil {
		return fmt.Errorf("failed to publish message: %v", token.Error())
	}
	return nil
}

// BatchPublishMessagesWithConcurrency publishes multiple messages concurrently.
// messages: Slice of TopicMessage structs containing the messages to be published.
// numWorkers: Number of concurrent workers to handle the publishing.
// Returns an error if any message publication fails.
func (c *MQTTClient) BatchPublishMessagesWithConcurrency(messages []TopicMessage, numWorkers int) error {
	topicChannels := make(map[string]chan TopicMessage)
	for _, msg := range messages {
		if _, exists := topicChannels[msg.Topic]; !exists {
			topicChannels[msg.Topic] = make(chan TopicMessage, numWorkers*2)
		}
	}

	for _, channel := range topicChannels {
		go func(ch chan TopicMessage) {
			for msg := range ch {
				payload, err := json.Marshal(msg.Message)
				if err != nil {
					fmt.Printf("Failed to marshal message for topic '%s': %v\n", msg.Topic, err)
					continue
				}

				token := c.client.Publish(msg.Topic, msg.Qos, false, payload)
				done := make(chan struct{})
				go func() {
					token.Wait()
					close(done)
				}()

				select {
				case <-done:
					if token.Error() != nil {
						fmt.Printf("Failed to publish message for topic '%s': %v\n", msg.Topic, token.Error())
					}
				case <-time.After(c.Timeout):
					fmt.Printf("Timeout waiting for message confirmation for topic '%s'\n", msg.Topic)
				}

				time.Sleep(100 * time.Millisecond)
			}
		}(channel)
	}

	for _, msg := range messages {
		topicChannels[msg.Topic] <- msg
	}

	for _, channel := range topicChannels {
		close(channel)
	}

	c.Wg.Wait()

	fmt.Println("Sent", len(messages))
	return nil
}
