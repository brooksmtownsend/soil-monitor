package core

import "context"

type Button int

const (
	ButtonPrevious Button = iota
	ButtonNext
	ButtonReserved
)

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

type Display interface {
	Render(View) error
	Close() error
}

type ButtonSource interface {
	Events() <-chan Button
	Close() error
}

type MQTTSource interface {
	Start(context.Context) error
	Events() <-chan MQTTEvent
	Close() error
}

type WebServer interface {
	Run(context.Context) error
}
