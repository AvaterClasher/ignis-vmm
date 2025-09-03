package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	firecracker "github.com/firecracker-microvm/firecracker-go-sdk"
	"github.com/joho/godotenv"
	log "github.com/sirupsen/logrus"
)

type benchJob struct {
	ID       string `json:"id"`
	Language string `json:"language"`
	Code     string `json:"code"`
}

type agentExecReq struct {
	ID      string `json:"id"`
	Command string `json:"command"`
}

type agentRunReq struct {
	ID       string `json:"id"`
	Language string `json:"language"`
	Code     string `json:"code"`
	Variant  string `json:"variant"`
}

type agentExecRes struct {
	Message      string `json:"message"`
	Error        string `json:"error"`
	StdErr       string `json:"stderr"`
	StdOut       string `json:"stdout"`
	ExecDuration int    `json:"exec_duration"`
	MemUsage     int64  `json:"mem_usage"`
}

type runningFirecracker struct {
	vmmCtx    context.Context
	vmmCancel context.CancelFunc
	vmmID     string
	machine   *firecracker.Machine
	ip        net.IP
}

var (
	q jobQueue
)

func main() {
	// Load environment variables from .env file if it exists
	if err := godotenv.Load(); err != nil {
		log.WithError(err).Debug("No .env file found or failed to load, using system environment variables")
	} else {
		log.Info("Loaded environment variables from .env file")
	}

	defer deleteVMMSockets()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Discover available languages from rootfs files
	languages, err := discoverAvailableLanguages()
	if err != nil {
		log.WithError(err).Fatal("Failed to discover available languages")
		return
	}

	log.WithField("languages", languages).Info("Discovered available languages")

	// Create the language pool manager
	poolManager := NewLanguagePoolManager()

	// Create pools for each language and start pool fillers
	const poolSize = 1 // Number of VMs per language pool
	for _, language := range languages {
		poolManager.AddPool(language, poolSize)

		// Start the pool filler for this language
		pool, _ := poolManager.GetPool(language)
		go fillLanguageVMPool(ctx, language, pool)
	}
	installSignalHandlers()
	log.SetReportCaller(true)

	// Initialize NATS connection URL. Prefer the NATS_URL env var, otherwise fall back to default.
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://localhost:422"
	}

	q = newJobQueue(natsURL)
	defer q.nc.Close()

	err = q.getQueueForJob(ctx)
	if err != nil {
		log.WithError(err).Fatal("Failed to get status queue")
		return
	}

	log.Info("Waiting for NATS jobs...")
	for d := range q.jobs {
		log.Printf("Received a message: %s", d.Body)

		var job benchJob
		err := json.Unmarshal([]byte(d.Body), &job)
		if err != nil {
			log.WithError(err).Error("Received invalid job")
			continue
		}

		go job.run(ctx, poolManager)
	}
}

func deleteVMMSockets() {
	tempDir := os.TempDir()
	entries, err := ioutil.ReadDir(tempDir)
	if err != nil {
		log.WithError(err).Error("Failed to read temp directory")
		return
	}

	prefix := fmt.Sprintf(".firecracker.sock-%d-", os.Getpid())
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, prefix) {
			full := filepath.Join(tempDir, name)
			if err := os.Remove(full); err != nil && !os.IsNotExist(err) {
				log.WithError(err).Warnf("Failed to remove leftover file %s", full)
			}
			if !strings.HasSuffix(name, ".log") {
				logFile := full + ".log"
				if err := os.Remove(logFile); err != nil && !os.IsNotExist(err) {
					log.WithError(err).Debugf("Failed to remove leftover log %s", logFile)
				}
			}
		}
	}
}

func installSignalHandlers() {
	go func() {
		// Clear some default handlers installed by the firecracker SDK:
		signal.Reset(os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)

		for {
			switch s := <-c; {
			case s == syscall.SIGTERM || s == os.Interrupt:
				log.Printf("Caught signal: %s, requesting clean shutdown", s.String())
				deleteVMMSockets()
				os.Exit(0)
			case s == syscall.SIGQUIT:
				log.Printf("Caught signal: %s, forcing shutdown", s.String())
				deleteVMMSockets()
				os.Exit(0)
			}
		}
	}()
}
