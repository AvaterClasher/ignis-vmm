package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

type LanguagePoolManager struct {
	pools map[string]chan runningFirecracker
	mutex sync.RWMutex
}

func NewLanguagePoolManager() *LanguagePoolManager {
	return &LanguagePoolManager{
		pools: make(map[string]chan runningFirecracker),
	}
}

func (lpm *LanguagePoolManager) GetPool(language string) (chan runningFirecracker, error) {
	lpm.mutex.RLock()
	pool, exists := lpm.pools[language]
	lpm.mutex.RUnlock()

	if !exists {
		return nil, fmt.Errorf("no pool found for language: %s", language)
	}
	return pool, nil
}

func (lpm *LanguagePoolManager) AddPool(language string, poolSize int) {
	lpm.mutex.Lock()
	defer lpm.mutex.Unlock()

	if _, exists := lpm.pools[language]; !exists {
		lpm.pools[language] = make(chan runningFirecracker, poolSize)
		log.WithField("language", language).WithField("size", poolSize).Info("Created pool for language")
	}
}

func discoverAvailableLanguages() ([]string, error) {
	files, err := ioutil.ReadDir("agent")
	if err != nil {
		return nil, fmt.Errorf("failed to read agent directory: %w", err)
	}

	var languages []string
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		// Look for rootfs-<language>.ext4 files
		if strings.HasPrefix(file.Name(), "rootfs-") && strings.HasSuffix(file.Name(), ".ext4") {
			// Extract language from filename: rootfs-python.ext4 -> python
			language := strings.TrimPrefix(file.Name(), "rootfs-")
			language = strings.TrimSuffix(language, ".ext4")
			languages = append(languages, language)
		}
	}

	if len(languages) == 0 {
		return nil, fmt.Errorf("no rootfs images found in agent directory")
	}

	return languages, nil
}

func getRootfsPath(language string) string {
	return filepath.Join("agent", fmt.Sprintf("rootfs-%s.ext4", language))
}

func fillLanguageVMPool(ctx context.Context, language string, pool chan<- runningFirecracker) {
	log.WithField("language", language).Info("Starting VM pool filler")

	for {
		select {
		case <-ctx.Done():
			log.WithField("language", language).Info("Stopping VM pool filler")
			return
		default:
			vm, err := createAndStartVMForLanguage(ctx, language)
			if err != nil {
				log.WithField("language", language).WithError(err).Error("Failed to create VMM")
				time.Sleep(time.Second)
				continue
			}

			log.WithField("language", language).WithField("ip", vm.ip).Info("New VM created and started")

			// Don't wait forever, if the VM is not available after 30s, move on
			vmCtx, cancel := context.WithTimeout(ctx, 30*time.Second)

			err = waitForVMToBoot(vmCtx, vm.ip)
			cancel()
			if err != nil {
				log.WithField("language", language).WithError(err).Info("VM not ready yet")
				vm.shutDown()
				continue
			}

			// Add the new microVM to the pool.
			// If the pool is full, this line will block until a slot is available.
			select {
			case pool <- *vm:
				log.WithField("language", language).WithField("ip", vm.ip).Info("VM added to pool")
			case <-ctx.Done():
				vm.shutDown()
				return
			}
		}
	}
}
