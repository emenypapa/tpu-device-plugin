package main

import (
	"fmt"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
	"os"
)

func getDevices(number int) (devs []*pluginapi.Device) {
	hostname, err := os.Hostname()
	if err != nil {
		return
	}
	for i := 0; i < number; i++ {
		devs = append(devs, &pluginapi.Device{
			ID:     fmt.Sprintf("tpu-%s-%d", hostname, i),
			Health: pluginapi.Healthy,
		})
	}
	return
}

func deviceExists(devs []*pluginapi.Device, id string) bool {
	for _, d := range devs {
		if d.ID == id {
			return true
		}
	}
	return false
}

// ToDo: need TPU lib Supported
func deviceIsFree(devs []*pluginapi.Device, id string) bool {
	for _, d := range devs {
		if d.ID == id {
			if d.Health == pluginapi.Healthy {
				return true
			} else {
				return false
			}
		}
	}
	return false
}
