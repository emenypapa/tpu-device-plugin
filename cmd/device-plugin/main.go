package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
)

const tpuNumber = 1

func main() {
	devicePlugin := NewTPUDevicePlugin(tpuNumber)
	if err := devicePlugin.Serve(); err != nil {
		log.Println("Could not contact Kubelet, Need Support!!")
	}
	// notice OS signal
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	select {
	case s := <-sigs:
		switch s {
		case syscall.SIGHUP:
			log.Println("Received SIGHUP, Exit.")
			os.Exit(1)
		default:
			log.Printf("Received signal \"%v\", shutting down.", s)
			err := devicePlugin.Stop()
			if err == nil {
				os.Exit(0)
			}
			os.Exit(255)
		}
	}
}
