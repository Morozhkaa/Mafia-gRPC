package client

import (
	"bufio"
	"context"
	"fmt"
	"os"
)

const buffSize = 20

type MessageReceiver struct {
	scanner *bufio.Scanner
	input   chan string
	output  chan string
}

func NewMessenger(ctx context.Context) *MessageReceiver {
	return &MessageReceiver{
		scanner: bufio.NewScanner(os.Stdin),
		input:   make(chan string, buffSize),
		output:  make(chan string, buffSize),
	}
}

func (m *MessageReceiver) Start() {
	go func() {
		for m.scanner.Scan() {
			m.input <- m.scanner.Text()
		}
	}()
	go func() {
		for s := range m.output {
			fmt.Fprint(os.Stdout, s)
		}
	}()
}

func (m *MessageReceiver) InputChan() <-chan string {
	return m.input
}

func (m *MessageReceiver) OutputChan() chan<- string {
	return m.output
}
