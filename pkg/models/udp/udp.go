package udp

import (
	"os"
)

type UdpMessage struct {
	FromUser string
	Ascii    []byte
}

func NewUdpMessage(path string) (UdpMessage, error) {
	var msg UdpMessage
	data, err := os.ReadFile(path)
	if err != nil {
		return msg, err
	}

	msg.Ascii = data
	return msg, nil
}
