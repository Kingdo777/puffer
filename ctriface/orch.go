// MIT License
//
// Copyright (c) 2020 Plamen Petrov and EASE lab
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package ctriface

import (
	log "github.com/sirupsen/logrus"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/containerd/containerd"
	fcclient "github.com/firecracker-microvm/firecracker-containerd/firecracker-control/client"

	"github.com/Kingdo777/puffer/misc"
)

const (
	containerdAddress      = "/run/firecracker-containerd/containerd.sock"
	containerdTTRPCAddress = containerdAddress + ".ttrpc"
	namespaceName          = "firecracker-containerd"
)

type WorkloadIoWriter struct {
	logger *log.Entry
}

func NewWorkloadIoWriter(vmID string) WorkloadIoWriter {
	return WorkloadIoWriter{log.WithFields(log.Fields{"vmID": vmID})}
}

func (wio WorkloadIoWriter) Write(p []byte) (n int, err error) {
	s := string(p)
	lines := strings.Split(s, "\n")
	for i := range lines {
		wio.logger.Info(string(lines[i]))
	}
	return len(p), nil
}

// Orchestrator Drives all VMs
type Orchestrator struct {
	vmPool       *misc.VMPool
	cachedImages map[string]containerd.Image
	workloadIo   sync.Map // vmID string -> WorkloadIoWriter
	snapshotter  string
	client       *containerd.Client
	fcClient     *fcclient.Client
	// store *skv.KVStore
	snapshotsEnabled bool
	snapshotsDir     string
	isMetricsMode    bool
	hostIface        string
}

// NewOrchestrator Initializes a new orchestrator
func NewOrchestrator(snapshotter, hostIface string, opts ...OrchestratorOption) *Orchestrator {
	var err error

	o := new(Orchestrator)
	o.vmPool = misc.NewVMPool()
	o.cachedImages = make(map[string]containerd.Image)
	o.snapshotter = snapshotter
	o.snapshotsDir = "/var/lib/puffer/snapshots"
	o.hostIface = hostIface

	for _, opt := range opts {
		opt(o)
	}

	if _, err := os.Stat(o.snapshotsDir); err != nil {
		if !os.IsNotExist(err) {
			log.Panicf("Snapshot dir %s exists", o.snapshotsDir)
		}
	}

	if err := os.MkdirAll(o.snapshotsDir, 0777); err != nil {
		log.Panicf("Failed to create snapshots dir %s", o.snapshotsDir)
	}

	log.Info("Creating containerd client")
	o.client, err = containerd.New(containerdAddress)
	if err != nil {
		log.Fatal("Failed to start containerd client", err)
	}
	log.Info("Created containerd client")

	log.Info("Creating firecracker client")
	o.fcClient, err = fcclient.New(containerdTTRPCAddress)
	if err != nil {
		log.Fatal("Failed to start firecracker client", err)
	}
	log.Info("Created firecracker client")
	return o
}

func (o *Orchestrator) setupCloseHandler() {
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		log.Info("\r- Ctrl+C pressed in Terminal")
		_ = o.StopActiveVMs()
		o.Cleanup()
		os.Exit(0)
	}()
}

// Cleanup Removes the bridges created by the VM pool's tap manager
func (o *Orchestrator) Cleanup() {
	o.vmPool.RemoveBridges()
	if err := os.RemoveAll(o.snapshotsDir); err != nil {
		log.Panic("failed to delete snapshots dir", err)
	}
}

// GetSnapshotsEnabled Returns the snapshots mode of the orchestrator
func (o *Orchestrator) GetSnapshotsEnabled() bool {
	return o.snapshotsEnabled
}

func (o *Orchestrator) getMemoryFile(funcName string) string {
	return filepath.Join(o.getVMBaseDir(funcName), "mem_file")
}

func (o *Orchestrator) getSnapshotFile(funcName string) string {
	return filepath.Join(o.getVMBaseDir(funcName), "snap_file")
}

func (o *Orchestrator) getVMBaseDir(funcName string) string {
	return filepath.Join(o.snapshotsDir, funcName)
}
