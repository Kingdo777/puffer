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

package firecracker

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Kingdo777/puffer/ctriface"
	log "github.com/sirupsen/logrus"
)

type coordinator struct {
	sync.Mutex
	orch   *ctriface.Orchestrator
	nextID uint64

	activeInstances     map[string]*funcInstance
	withoutOrchestrator bool
}

type coordinatorOption func(*coordinator)

// withoutOrchestrator is used for testing the coordinator without calling the orchestrator
func withoutOrchestrator() coordinatorOption {
	return func(c *coordinator) {
		c.withoutOrchestrator = true
	}
}

func newFirecrackerCoordinator(orch *ctriface.Orchestrator, opts ...coordinatorOption) *coordinator {
	c := &coordinator{
		activeInstances: make(map[string]*funcInstance),
		orch:            orch,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

func (c *coordinator) startVM(ctx context.Context, image string) (*funcInstance, error) {
	return c.startVMWithEnvironment(ctx, image, []string{})
}

func (c *coordinator) startVMWithEnvironment(ctx context.Context, image string, environment []string) (*funcInstance, error) {
	return c.orchStartVM(ctx, image, environment)
}

func (c *coordinator) stopVM(ctx context.Context, containerID string) error {
	c.Lock()

	fi, ok := c.activeInstances[containerID]
	delete(c.activeInstances, containerID)

	c.Unlock()

	if !ok {
		return nil
	}

	return c.orchStopVM(ctx, fi)
}

// for testing
func (c *coordinator) isActive(containerID string) bool {
	c.Lock()
	defer c.Unlock()

	_, ok := c.activeInstances[containerID]
	return ok
}

func (c *coordinator) insertActive(containerID string, fi *funcInstance) error {
	c.Lock()
	defer c.Unlock()

	logger := log.WithFields(log.Fields{"containerID": containerID, "vmID": fi.VmID})

	if fi, present := c.activeInstances[containerID]; present {
		logger.Errorf("entry for container already exists with vmID %s" + fi.VmID)
		return errors.New("entry for container already exists")
	}

	c.activeInstances[containerID] = fi
	return nil
}

func (c *coordinator) orchStartVM(ctx context.Context, image string, envVariables []string) (*funcInstance, error) {
	vmID := strconv.Itoa(int(atomic.AddUint64(&c.nextID, 1)))
	logger := log.WithFields(
		log.Fields{
			"vmID":  vmID,
			"image": image,
		},
	)

	logger.Debug("creating fresh instance")

	var (
		resp *ctriface.StartVMResponse
		err  error
	)

	ctxTimeout, cancel := context.WithTimeout(ctx, time.Minute*5)
	defer cancel()

	if !c.withoutOrchestrator {
		resp, _, err = c.orch.StartVMWithEnvironment(ctxTimeout, vmID, image, envVariables)
		if err != nil {
			logger.WithError(err).Error("coordinator failed to start VM")
		}
	}

	fi := newFuncInstance(vmID, image, resp)
	logger.Debug("successfully created fresh instance")
	return fi, err
}

func (c *coordinator) orchStopVM(ctx context.Context, fi *funcInstance) error {
	if c.withoutOrchestrator {
		return nil
	}

	if err := c.orch.StopSingleVM(ctx, fi.VmID); err != nil {
		fi.Logger.WithError(err).Error("failed to stop VM for instance")
		return err
	}

	return nil
}
