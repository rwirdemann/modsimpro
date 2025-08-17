package main

import (
	"fmt"
	"log"
	"time"

	"github.com/goburrow/modbus"
)

func main() {

	handler := modbus.NewTCPClientHandler("localhost:502")
	handler.Timeout = 1 * time.Second
	handler.SlaveId = 101

	err := handler.Connect()
	if err != nil {
		log.Fatal(err)
	}
	defer handler.Close()

	client := modbus.NewClient(handler)
	bb, err := client.ReadDiscreteInputs(0x7e3, 1)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("response: %v", bb)
}
