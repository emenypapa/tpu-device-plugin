package main

import (
	"context"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
	"log"
	"net"
	"os"
	"path"
	"time"
)

const (
	pluginName   = "eicas.com/tpu"
	resourceName = "eicas.com/tpu"
	serverSock   = pluginapi.DevicePluginPath + "eicas.sock"
)

type tpuDevicePlugin struct {
	devices []*pluginapi.Device
	stopCh  chan interface{}
	socket  string
	server  *grpc.Server
}

// how many tpu will to be allocated
func NewTPUDevicePlugin(number int) *tpuDevicePlugin {
	return &tpuDevicePlugin{
		devices: getDevices(number),
		stopCh:  make(chan interface{}),
		socket:  serverSock,
	}
}

// GetDevicePluginOptions returns the values of the optional settings for this plugin, NOT BE USED
func (t *tpuDevicePlugin) GetDevicePluginOptions(context.Context, *pluginapi.Empty) (*pluginapi.DevicePluginOptions, error) {
	return &pluginapi.DevicePluginOptions{}, nil
}

// PreStartContainer is unimplemented for this plugin， NOT BE USED
func (t *tpuDevicePlugin) PreStartContainer(context.Context, *pluginapi.PreStartContainerRequest) (*pluginapi.PreStartContainerResponse, error) {
	return &pluginapi.PreStartContainerResponse{}, nil
}

// GetPreferredAllocation returns the preferred allocation from the set of devices specified in the request, NOT BE USED
func (p *tpuDevicePlugin) GetPreferredAllocation(ctx context.Context, r *pluginapi.PreferredAllocationRequest) (*pluginapi.PreferredAllocationResponse, error) {
	response := &pluginapi.PreferredAllocationResponse{}
	return response, nil
}

// clean up the unix socket file
func (t *tpuDevicePlugin) cleanup() error {
	if err := os.Remove(t.socket); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (t *tpuDevicePlugin) Register(endpoint, resourceName string) error {
	conn, err := dial(endpoint, 5*time.Second)
	defer conn.Close()
	if err != nil {
		return err
	}
	client := pluginapi.NewRegistrationClient(conn)
	req := &pluginapi.RegisterRequest{
		Version:      pluginapi.Version,
		Endpoint:     path.Base(t.socket),
		ResourceName: resourceName,
	}
	_, err = client.Register(context.Background(), req)
	if err != nil {
		return err
	}
	return nil
}

func (t *tpuDevicePlugin) Stop() error {
	if t.server == nil {
		return nil
	}
	t.server.Stop()
	t.server = nil
	close(t.stopCh)
	return t.cleanup()
}

// start the grpc sever and register the device plugin to Kubelet
func (t *tpuDevicePlugin) Serve() (error, bool) {
	// Create a gRPC server and register the device plugin service.
	err := t.Start()
	if err != nil {
		log.Printf("Could not start device plugin: %s", err)
		return err, false
	}
	log.Println("eicas device plugin starting on ", t.socket)

	// register it to kubelet
	//pluginapi.RegisterDevicePluginServer(server, p)
	err = t.Register(pluginapi.KubeletSocket, resourceName)
	if err != nil {
		log.Printf("Could NOT register eicas device plugin: %s", err)
		t.Stop()
		return err, false
	}
	log.Println("Registered device plugin to Kubelet")
	return nil, true
}

// establishes the grpc with the registered device plugin
func dial(unixSocketPath string, timeout time.Duration) (c *grpc.ClientConn, err error) {
	/*
		c, err = grpc.Dial(unixSocketPath, grpc.WithInsecure(), grpc.WithBlock(), grpc.WithTimeout(timeout),
			grpc.WithDialer(func(addr string, timeout time.Duration) (net.Conn, error) {
				return net.DialTimeout("unix", addr, timeout)
			}))
	*/
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	c, err = grpc.DialContext(ctx, unixSocketPath, grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(), grpc.WithContextDialer(func(ctx2 context.Context, addr string) (net.Conn, error) {
			return net.Dial("unix", addr)
		}))
	if err != nil {
		cancel()
		return nil, err
	}
	return c, nil
}

// start the grpc server of my device plugin
func (t *tpuDevicePlugin) Start() error {
	err := t.cleanup()
	if err != nil {
		return err
	}
	sock, err := net.Listen("unix", t.socket)
	if err != nil {
		return err
	}
	t.server = grpc.NewServer([]grpc.ServerOption{}...)
	pluginapi.RegisterDevicePluginServer(t.server, t)

	go t.server.Serve(sock)

	//
	conn, err := dial(t.socket, 5*time.Second)
	defer conn.Close()
	if err != nil {
		return nil
	}
	return nil
}

// ListAndWatch lists devices and update that list according to the health status( NEED tpu lib support)
func (t *tpuDevicePlugin) ListAndWatch(empty *pluginapi.Empty, stream pluginapi.DevicePlugin_ListAndWatchServer) error {
	// send the current device list to kubelet
	resp := new(pluginapi.ListAndWatchResponse)
	resp.Devices = t.devices
	err := stream.Send(resp)
	if err != nil {
		log.Println("error on ListAndWatch ", err)
		return err
	}
	// watch for tpu device changes
	for {
		select {
		case <-t.stopCh:
			return nil
		}
	}
}

// Allocate which return list of devices.
func (t *tpuDevicePlugin) Allocate(ctx context.Context, req *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	log.Printf("Received Allocate request %+v", req)
	responses := pluginapi.AllocateResponse{}
	// allocate a device only if it is available
	for _, req := range req.ContainerRequests {
		for _, id := range req.DevicesIDs {
			if !deviceExists(t.devices, id) {
				return nil, fmt.Errorf("invalid allocation request: unknow device: %s", id)
			}
		}
		response := new(pluginapi.ContainerAllocateResponse)
		//ToDo: NEED tpu lib support
		// just for example，we mount tmp in Host path
		response.Devices = []*pluginapi.DeviceSpec{
			{
				ContainerPath: "/tmp",
				HostPath:      "/tmp",
				Permissions:   "rwm",
			},
		}
		responses.ContainerResponses = append(responses.ContainerResponses, response)
	}
	return &responses, nil
}
