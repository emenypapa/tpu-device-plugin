package main

import (
	"github.com/fsnotify/fsnotify"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const tpuNumber = 1

func main() {
	devicePlugin := NewTPUDevicePlugin(tpuNumber)
	err, _ := devicePlugin.Serve()
	if err != nil {
		log.Println("Could not contact Kubelet, Need Support!!")
	}
	// notice OS signal
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	var restartTimeout <-chan time.Time

	log.Println("Starting FS watcher.")
	watcher, err := newFSWatcher(pluginapi.DevicePluginPath)
	if err != nil {
		log.Printf("failed to create FS watcher: %v\n", err)
	}
	defer watcher.Close()

restart:
	log.Println("Starting Plugins.")
	err, restartTime := devicePlugin.Serve()
	if err != nil {
		log.Println("Could not contact Kubelet, Need Support!!")
	}
	if restartTime {
		log.Printf("Failed to start one or more plugins. Retrying in 30s...")
		restartTimeout = time.After(30 * time.Second)
	}

	for {
		select {
		// If the restart timout has expired, then restart the plugins
		case <-restartTimeout:
			goto restart

		// Detect a kubelet restart by watching for a newly created
		// 'pluginapi.KubeletSocket' file. When this occurs, restart this loop,
		// restarting all of the plugins in the process.
		case event := <-watcher.Events:
			if event.Name == pluginapi.KubeletSocket && event.Op&fsnotify.Create == fsnotify.Create {
				log.Printf("inotify: %s created, restarting.", pluginapi.KubeletSocket)
				goto restart
			}

		// Watch for any other fs errors and log them.
		case err := <-watcher.Errors:
			log.Printf("inotify: %s", err)

		// Watch for any signals from the OS. On SIGHUP, restart this loop,
		// restarting all of the plugins in the process. On all other
		// signals, exit the loop and exit the program.
		case s := <-sigs:
			switch s {
			case syscall.SIGHUP:
				log.Println("Received SIGHUP, restarting.")
				goto restart
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
}
