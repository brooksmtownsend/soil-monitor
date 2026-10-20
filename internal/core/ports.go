package core

import "context"

type MQTTEventKind int

const (
	MQTTConnected MQTTEventKind = iota
	MQTTDisconnected
	MQTTMessage
	MQTTError
)

type MQTTEvent struct {
	Kind    MQTTEventKind
	Topic   string
	Payload []byte
	Err     error
}

type MQTTSource interface {
	Start(context.Context) error
	Events() <-chan MQTTEvent
	Close() error
}

type WebServer interface {
	Run(context.Context) error
	Notify(Snapshot)
}
