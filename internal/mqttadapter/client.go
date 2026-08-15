package mqttadapter

import (
	"context"
	"fmt"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"soilmonitor/internal/core"
)

const moistureSubscription = "plants/+/moisture"

type Client struct {
	client  mqtt.Client
	events  chan core.MQTTEvent
	cancel  context.CancelFunc
	done    chan struct{}
	once    sync.Once
	mu      sync.Mutex
	started bool
}

func New(brokerURL, clientID string) *Client {
	adapter := &Client{
		events: make(chan core.MQTTEvent, 64),
		done:   make(chan struct{}),
	}
	opts := mqtt.NewClientOptions().
		AddBroker(brokerURL).
		SetClientID(clientID).
		SetAutoReconnect(true).
		SetConnectRetry(false).
		SetOrderMatters(false).
		SetKeepAlive(30 * time.Second).
		SetPingTimeout(10 * time.Second)
	opts.SetConnectionLostHandler(func(_ mqtt.Client, err error) {
		adapter.send(core.MQTTEvent{Kind: core.MQTTDisconnected, Err: err})
	})
	opts.SetOnConnectHandler(func(client mqtt.Client) {
		token := client.Subscribe(moistureSubscription, 0, func(_ mqtt.Client, message mqtt.Message) {
			payload := append([]byte(nil), message.Payload()...)
			adapter.send(core.MQTTEvent{Kind: core.MQTTMessage, Topic: message.Topic(), Payload: payload})
		})
		if !token.WaitTimeout(5 * time.Second) {
			adapter.send(core.MQTTEvent{Kind: core.MQTTError, Err: fmt.Errorf("subscribe to %s timed out", moistureSubscription)})
			return
		}
		if err := token.Error(); err != nil {
			adapter.send(core.MQTTEvent{Kind: core.MQTTError, Err: fmt.Errorf("subscribe to %s: %w", moistureSubscription, err)})
			return
		}
		adapter.send(core.MQTTEvent{Kind: core.MQTTConnected})
	})
	adapter.client = mqtt.NewClient(opts)
	return adapter
}

func (c *Client) Start(parent context.Context) error {
	if c.client == nil {
		return fmt.Errorf("MQTT client is not initialized")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.started {
		return fmt.Errorf("MQTT client is already started")
	}
	ctx, cancel := context.WithCancel(parent)
	c.cancel = cancel
	c.started = true
	go c.connectLoop(ctx)
	return nil
}

func (c *Client) Events() <-chan core.MQTTEvent {
	return c.events
}

func (c *Client) Close() error {
	c.once.Do(func() {
		c.mu.Lock()
		started := c.started
		if c.cancel != nil {
			c.cancel()
		}
		c.mu.Unlock()
		if c.client != nil && c.client.IsConnected() {
			c.client.Disconnect(250)
		}
		if started {
			<-c.done
		}
	})
	return nil
}

func (c *Client) connectLoop(ctx context.Context) {
	defer close(c.done)
	for {
		if ctx.Err() != nil {
			return
		}
		token := c.client.Connect()
		if !token.WaitTimeout(10 * time.Second) {
			c.send(core.MQTTEvent{Kind: core.MQTTError, Err: fmt.Errorf("MQTT connection timed out")})
		} else if err := token.Error(); err != nil {
			c.send(core.MQTTEvent{Kind: core.MQTTError, Err: fmt.Errorf("connect to MQTT: %w", err)})
		} else {
			return
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
}

func (c *Client) send(event core.MQTTEvent) {
	select {
	case c.events <- event:
	default:
		// Connection callbacks must never block the MQTT network loop. A later
		// retained reading or reconnect event will restore the current view.
	}
}
