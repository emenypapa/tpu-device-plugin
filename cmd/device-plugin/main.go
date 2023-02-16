package main

import (
	"context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
	"log"
	"net"
)

const (
	pluginName   = "eicas.com/tpu"
	resourceName = "eicas.com/tpu"
)

type tpuDevicePlugin struct {
	devices []*pluginapi.Device
	stopCh  chan interface{}
}

func newTPUDevicePlugin() *tpuDevicePlugin {
	return &tpuDevicePlugin{
		devices: []*pluginapi.Device{{
			ID:     "TPU-0",
			Health: pluginapi.Healthy,
		}},
		stopCh: make(chan interface{}),
	}
}

func (p *tpuDevicePlugin) ListAndWatch(empty *pluginapi.Empty, stream pluginapi.DevicePlugin_ListAndWatchServer) error {
	// send the current device list to kubelet
	resp := new(pluginapi.ListAndWatchResponse)
	resp.Devices = p.devices
	err := stream.Send(resp)
	if err != nil {
		log.Println("error on ListAndWatch ", err)
		return err
	}
	// watch for tpu device changes
	<-p.stopCh
	return nil
}

func (p *tpuDevicePlugin) Allocate(ctx context.Context, req *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	log.Printf("Received Allocate request %+v", req)
	responses := pluginapi.AllocateResponse{}
	// allocate a device only if it is available
	for _, req := range req.ContainerRequests {
		for _, id := range req.DevicesIDs {
			found := false
			for i, dev := range p.devices {
				if id == dev.ID && dev.Health == pluginapi.Healthy {
					p.devices[i].Health = pluginapi.Unhealthy
					found = true
					break
				}
			}
			if !found {
				return nil, status.Errorf(codes.NotFound,
					"requested TPU device %s not found or is already in use", id)
			}
		}

		resp := p.getAllocateResponse(req.DevicesIDs)
		responses.ContainerResponses = append(responses.ContainerResponses, resp)
	}
	return &responses, nil
}

func (p *tpuDevicePlugin) getAllocateResponse(requestIds []string) *pluginapi.ContainerAllocateResponse {
	// need tpu spec to determined
	response := pluginapi.ContainerAllocateResponse{}
	return &response
}

// GetDevicePluginOptions returns the values of the optional settings for this plugin
func (p *tpuDevicePlugin) GetDevicePluginOptions(context.Context, *pluginapi.Empty) (*pluginapi.DevicePluginOptions, error) {
	options := &pluginapi.DevicePluginOptions{
		GetPreferredAllocationAvailable: true,
	}
	return options, nil
}

// GetPreferredAllocation returns the preferred allocation from the set of devices specified in the request
// ignore it
func (p *tpuDevicePlugin) GetPreferredAllocation(ctx context.Context, r *pluginapi.PreferredAllocationRequest) (*pluginapi.PreferredAllocationResponse, error) {
	response := &pluginapi.PreferredAllocationResponse{}
	return response, nil
}

// PreStartContainer is unimplemented for this plugin
func (p *tpuDevicePlugin) PreStartContainer(context.Context, *pluginapi.PreStartContainerRequest) (*pluginapi.PreStartContainerResponse, error) {
	return &pluginapi.PreStartContainerResponse{}, nil
}

func (p *tpuDevicePlugin) Serve() {
	// Create a gRPC server and register the device plugin service.
	server := grpc.NewServer([]grpc.ServerOption{}...)
	pluginapi.RegisterDevicePluginServer(server, p)
	// Start serving.
	socketPath := pluginapi.DevicePluginPath + pluginName
	lis, err := net.Listen("unix", socketPath)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	log.Printf("Starting to serve on %s", socketPath)
	err = server.Serve(lis)
	if err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}

func main() {
	plugin := newTPUDevicePlugin()
	plugin.Serve()
}
