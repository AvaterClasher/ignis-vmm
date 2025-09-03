package main

import (
	"context"
	"errors"
	"net"
	"os"
	"strings"
	"time"

	"github.com/imroc/req"
	log "github.com/sirupsen/logrus"
)

func waitForVMToBoot(ctx context.Context, ip net.IP) error {
	// Query the agent until it provides a valid response
	req.SetTimeout(1000 * time.Millisecond)
	for {
		select {
		case <-ctx.Done():
			// Timeout
			return ctx.Err()
		default:
			res, err := req.Get("http://" + ip.String() + ":8080/health")
			if err != nil {
				log.WithError(err).Info("VM not ready yet")
				time.Sleep(time.Second)
				continue
			}

			if res.Response().StatusCode != 200 {
				time.Sleep(time.Second)
				log.Info("VM not ready yet")
			} else {
				log.WithField("ip", ip).Info("VM agent ready")
				return nil
			}
			time.Sleep(time.Second)
		}

	}
}

func (vm runningFirecracker) shutDown() {
	log.WithField("ip", vm.ip).Info("stopping")
	log.WithField("rootfs", "/tmp/rootfs-"+vm.vmmID+".ext4").Info("deleting rootfs")
	log.WithField("socket", vm.machine.Cfg.SocketPath).Info("deleting socket")

	// 1) Try graceful shutdown first
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer shutdownCancel()
	if err := vm.machine.Shutdown(shutdownCtx); err != nil {
		log.WithError(err).Warn("Graceful shutdown failed; will force stop if still running")
	}

	// 2) Wait for the VMM to exit and perform SDK cleanup once
	waitCtx, waitCancel := context.WithTimeout(context.Background(), 7*time.Second)
	defer waitCancel()
	if err := vm.machine.Wait(waitCtx); err != nil {
		// Ignore benign errors that frequently appear with some SDK/CNI versions
		msg := err.Error()
		ignorable := strings.Contains(msg, "signal: terminated") ||
			strings.Contains(msg, "failed to remove netns parent dir") ||
			strings.Contains(msg, "plugin type=\"tc-redirect-tap\"") ||
			strings.Contains(msg, "CNI network list \"fcnet\"") ||
			errors.Is(err, context.DeadlineExceeded)

		if ignorable {
			log.WithError(err).Debug("Ignoring non-fatal wait error")
		} else {
			log.WithError(err).Warn("Wait returned error")
		}
	}

	// 3) Ensure the VMM is gone (force stop as a fallback)
	if err := vm.machine.StopVMM(); err != nil {
		// It's okay if the process is already gone
		log.WithError(err).Debug("StopVMM returned error (likely already stopped)")
	}

	// 4) Remove socket if it still exists
	if err := os.Remove(vm.machine.Cfg.SocketPath); err != nil && !os.IsNotExist(err) {
		log.WithError(err).Error("Failed to delete firecracker socket")
	}

	// 5) Retry deleting the rootfs with backoff (the file can be busy briefly)
	rootfsPath := "/tmp/rootfs-" + vm.vmmID + ".ext4"
	var lastErr error
	for attempt := 1; attempt <= 30; attempt++ {
		if err := os.Remove(rootfsPath); err != nil {
			if os.IsNotExist(err) {
				lastErr = nil
				break
			}
			lastErr = err
			// Log at debug to avoid noise when the file is briefly busy
			log.WithError(err).Debugf("Failed to delete firecracker rootfs (attempt %d/30)", attempt)
			time.Sleep(time.Duration(attempt) * 100 * time.Millisecond)
			continue
		}
		lastErr = nil
		break
	}
	if lastErr != nil {
		log.WithError(lastErr).Error("Failed to delete firecracker rootfs after retries")
	}

	// 6) Cancel the VMM context to release any lingering resources
	if vm.vmmCancel != nil {
		vm.vmmCancel()
	}
}
