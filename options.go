package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	firecracker "github.com/firecracker-microvm/firecracker-go-sdk"
	models "github.com/firecracker-microvm/firecracker-go-sdk/client/models"
)

func getFirecrackerConfig(vmmID string) (firecracker.Config, error) {
	socket := getSocketPath(vmmID)
	return firecracker.Config{
		SocketPath:      socket,
		KernelImagePath: "linux/vmlinux",
		LogPath:         fmt.Sprintf("%s.log", socket),
		Drives: []models.Drive{{
			DriveID:      firecracker.String("1"),
			PathOnHost:   firecracker.String("/tmp/rootfs-" + vmmID + ".ext4"),
			IsRootDevice: firecracker.Bool(true),
			IsReadOnly:   firecracker.Bool(false),
		}},
		NetworkInterfaces: []firecracker.NetworkInterface{{
			// Use CNI to get dynamic IP
			CNIConfiguration: &firecracker.CNIConfiguration{
				NetworkName: "fcnet",
				IfName:      "fire_veth",
			},
		}},
		MachineCfg: models.MachineConfiguration{
			VcpuCount:  firecracker.Int64(1),
			MemSizeMib: firecracker.Int64(300),
		},
	}, nil
}

func getSocketPath(vmmID string) string {
	filename := strings.Join([]string{
		".firecracker.sock",
		strconv.Itoa(os.Getpid()),
		vmmID,
	},
		"-",
	)
	dir := os.TempDir()

	return filepath.Join(dir, filename)
}
