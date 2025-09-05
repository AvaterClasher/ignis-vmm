package main

import (
	"context"
	"fmt"
	"io"
	"os"

	firecracker "github.com/firecracker-microvm/firecracker-go-sdk"
	"github.com/rs/xid"
	log "github.com/sirupsen/logrus"
)

func copyFile(src string, dst string) error {
	// Use a streaming copy to avoid loading big rootfs images entirely in memory
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		cerr := out.Close()
		if err == nil {
			err = cerr
		}
	}()

	if _, err = io.Copy(out, in); err != nil {
		return err
	}
	// Ensure the data is flushed to disk before returning
	return out.Sync()
}

// Create a VMM with a specific language rootfs and start the VM
func createAndStartVMForLanguage(ctx context.Context, language string) (*runningFirecracker, error) {
	vmmID := xid.New().String()
	baseRootFS := getRootfsPath(language)

	// Check if the rootfs file exists
	if _, err := os.Stat(baseRootFS); os.IsNotExist(err) {
		return nil, fmt.Errorf("rootfs for language %s not found: %s", language, baseRootFS)
	}

	// Prepare a dedicated writable copy of the root filesystem for this VM instance
	if err := copyFile(baseRootFS, "/tmp/rootfs-"+vmmID+".ext4"); err != nil {
		return nil, fmt.Errorf("failed to prepare rootfs for language %s: %w", language, err)
	}

	fcCfg, err := getFirecrackerConfig(vmmID)
	if err != nil {
		log.Errorf("Error: %s", err)
		return nil, err
	}
	logger := log.New()

	if false {
		log.SetLevel(log.DebugLevel)
		logger.SetLevel(log.DebugLevel)
	}

	machineOpts := []firecracker.Opt{
		firecracker.WithLogger(log.NewEntry(logger)),
	}

	firecrackerBinary := os.Getenv("FIRECRACKER_BINARY")
	if firecrackerBinary == "" {
		firecrackerBinary = "/home/user/.local/bin/firecracker" // fallback
	}

	finfo, err := os.Stat(firecrackerBinary)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("binary %q does not exist: %v", firecrackerBinary, err)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to stat binary, %q: %v", firecrackerBinary, err)
	}

	if finfo.IsDir() {
		return nil, fmt.Errorf("binary, %q, is a directory", firecrackerBinary)
	} else if finfo.Mode()&0111 == 0 {
		return nil, fmt.Errorf("binary, %q, is not executable. Check permissions of binary", firecrackerBinary)
	}

	vmmCtx, vmmCancel := context.WithCancel(ctx)

	m, err := firecracker.NewMachine(vmmCtx, fcCfg, machineOpts...)
	if err != nil {
		vmmCancel()
		return nil, fmt.Errorf("failed creating machine: %s", err)
	}

	if err := m.Start(vmmCtx); err != nil {
		vmmCancel()
		return nil, fmt.Errorf("failed to start machine: %v", err)
	}

	log.WithFields(log.Fields{
		"ip":       m.Cfg.NetworkInterfaces[0].StaticConfiguration.IPConfiguration.IPAddr.IP,
		"language": language,
		"vmmID":    vmmID,
	}).Info("machine started")

	return &runningFirecracker{
		vmmCtx:    vmmCtx,
		vmmCancel: vmmCancel,
		vmmID:     vmmID,
		machine:   m,
		ip:        m.Cfg.NetworkInterfaces[0].StaticConfiguration.IPConfiguration.IPAddr.IP,
	}, nil
}
