// MIT License
//
// Copyright (c) 2020 Dmitrii Ustiugov, Plamen Petrov and EASE lab
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
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	"github.com/containerd/containerd/remotes/docker"

	"github.com/firecracker-microvm/firecracker-containerd/proto" // note: from the original repo
	"github.com/firecracker-microvm/firecracker-containerd/runtime/firecrackeroci"
	"github.com/pkg/errors"

	"github.com/Kingdo777/puffer/metrics"
	"github.com/Kingdo777/puffer/misc"
)

// StartVMResponse is the response returned by StartVM
// TODO: Integrate response with non-cri API
type StartVMResponse struct {
	// GuestIP is the IP of the guest MicroVM
	GuestIP string
}

const (
	testImageName = "docker.io/library/nginx:1.17-alpine"
	//testImageName = "registry.cn-hangzhou.aliyuncs.com/kingdo_puffer/function-helloworld-python:latest"
)

// StartVM Boots a VM if it does not exist
func (o *Orchestrator) StartVM(ctx context.Context, vmID, imageName string) (_ *StartVMResponse, _ *metrics.Metric, retErr error) {
	return o.StartVMWithEnvironment(ctx, vmID, imageName, []string{})
}

func (o *Orchestrator) StartVMWithEnvironment(ctx context.Context, vmID, imageName string, environmentVariables []string) (_ *StartVMResponse, _ *metrics.Metric, retErr error) {
	var (
		startVMMetric *metrics.Metric = metrics.NewMetric()
		tStart        time.Time
	)

	logger := log.WithFields(log.Fields{"vmID": vmID, "image": imageName})
	logger.Debug("StartVM: Received StartVM")

	vm, err := o.vmPool.Allocate(vmID, o.hostIface)
	if err != nil {
		logger.Error("failed to allocate VM in VM pool")
		return nil, nil, err
	}

	defer func() {
		// Free the VM from the pool if function returns error
		if retErr != nil {
			if err := o.vmPool.Free(vmID); err != nil {
				logger.WithError(err).Errorf("failed to free VM from pool after failure")
			}
		}
	}()

	ctx = namespaces.WithNamespace(ctx, namespaceName)
	tStart = time.Now()
	if vm.Image, err = o.getImage(ctx, imageName); err != nil {
		return nil, nil, errors.Wrapf(err, "Failed to get/pull image")
	}
	startVMMetric.MetricMap[metrics.GetImage] = metrics.ToUS(time.Since(tStart))

	tStart = time.Now()
	createVMRequest := o.getVMCreateRequest(vm)
	_, err = o.fcClient.CreateVM(ctx, createVMRequest)

	startVMMetric.MetricMap[metrics.FcCreateVM] = metrics.ToUS(time.Since(tStart))
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to create the microVM in firecracker-containerd")
	}

	defer func() {
		if retErr != nil {
			if _, err := o.fcClient.StopVM(ctx, &proto.StopVMRequest{VMID: vmID}); err != nil {
				logger.WithError(err).Errorf("failed to stop firecracker-containerd VM after failure")
			}
		}
	}()

	logger.Debug("StartVM: Creating a new container")
	tStart = time.Now()
	container, err := o.client.NewContainer(
		ctx,
		vmID,
		containerd.WithSnapshotter(o.snapshotter),
		containerd.WithNewSnapshot(vmID, *vm.Image),
		containerd.WithNewSpec(
			oci.WithImageConfig(*vm.Image),
			firecrackeroci.WithVMID(vmID),
			firecrackeroci.WithVMNetwork,
			oci.WithEnv(environmentVariables),
		),
		containerd.WithRuntime("aws.firecracker", nil),
	)
	startVMMetric.MetricMap[metrics.NewContainer] = metrics.ToUS(time.Since(tStart))
	vm.Container = &container
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to create a container")
	}

	defer func() {
		if retErr != nil {
			if err := container.Delete(ctx, containerd.WithSnapshotCleanup); err != nil {
				logger.WithError(err).Errorf("failed to delete container after failure")
			}
		}
	}()

	iologger := NewWorkloadIoWriter(vmID)
	o.workloadIo.Store(vmID, &iologger)
	logger.Debug("StartVM: Creating a new task")
	tStart = time.Now()
	task, err := container.NewTask(ctx, cio.NewCreator(cio.WithStreams(os.Stdin, iologger, iologger)))
	startVMMetric.MetricMap[metrics.NewTask] = metrics.ToUS(time.Since(tStart))
	vm.Task = &task
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to create a task")
	}

	defer func() {
		if retErr != nil {
			if _, err := task.Delete(ctx); err != nil {
				logger.WithError(err).Errorf("failed to delete task after failure")
			}
		}
	}()

	logger.Debug("StartVM: Waiting for the task to get ready")
	tStart = time.Now()
	ch, err := task.Wait(ctx)
	startVMMetric.MetricMap[metrics.TaskWait] = metrics.ToUS(time.Since(tStart))
	vm.TaskCh = ch
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to wait for a task")
	}

	defer func() {
		if retErr != nil {
			if err := task.Kill(ctx, syscall.SIGKILL); err != nil {
				logger.WithError(err).Errorf("failed to kill task after failure")
			}
		}
	}()

	logger.Debug("StartVM: Starting the task")
	tStart = time.Now()
	if err := task.Start(ctx); err != nil {
		return nil, nil, errors.Wrap(err, "failed to start a task")
	}
	startVMMetric.MetricMap[metrics.TaskStart] = metrics.ToUS(time.Since(tStart))

	defer func() {
		if retErr != nil {
			if err := task.Kill(ctx, syscall.SIGKILL); err != nil {
				logger.WithError(err).Errorf("failed to kill task after failure")
			}
		}
	}()

	if err := os.MkdirAll(o.getVMBaseDir(vmID), 0777); err != nil {
		logger.Error("Failed to create VM base dir")
		return nil, nil, err
	}

	logger.Debug("Successfully started a VM")

	return &StartVMResponse{GuestIP: vm.Ni.PrimaryAddress}, startVMMetric, nil
}

// StopSingleVM Shuts down a VM
// Note: VMs are not quisced before being stopped
func (o *Orchestrator) StopSingleVM(ctx context.Context, vmID string) error {
	logger := log.WithFields(log.Fields{"vmID": vmID})
	logger.Debug("Orchestrator received StopVM")

	ctx = namespaces.WithNamespace(ctx, namespaceName)
	vm, err := o.vmPool.GetVM(vmID)
	if err != nil {
		if _, ok := err.(*misc.NonExistErr); ok {
			logger.Panic("StopVM: VM does not exist")
		}
		logger.Panic("StopVM: GetVM() failed for an unknown reason")

	}

	logger = log.WithFields(log.Fields{"vmID": vmID})

	task := *vm.Task
	if err := task.Kill(ctx, syscall.SIGKILL); err != nil {
		logger.WithError(err).Error("Failed to kill the task")
		return err
	}

	<-vm.TaskCh
	//FIXME: Seems like some tasks need some extra time to die Issue#15, lr_training
	time.Sleep(500 * time.Millisecond)

	if _, err := task.Delete(ctx); err != nil {
		logger.WithError(err).Error("failed to delete task")
		return err
	}

	container := *vm.Container
	if err := container.Delete(ctx, containerd.WithSnapshotCleanup); err != nil {
		logger.WithError(err).Error("failed to delete container")
		return err
	}

	if _, err := o.fcClient.StopVM(ctx, &proto.StopVMRequest{VMID: vmID}); err != nil {
		logger.WithError(err).Error("failed to stop firecracker-containerd VM")
		return err
	}

	if err := o.vmPool.Free(vmID); err != nil {
		logger.Error("failed to free VM from VM pool")
		return err
	}

	o.workloadIo.Delete(vmID)

	logger.Debug("Stopped VM successfully")

	return nil
}

// Checks whether a URL has a .local domain
func isLocalDomain(s string) (bool, error) {
	if !strings.Contains(s, "://") {
		s = "dummy://" + s
	}

	u, err := url.Parse(s)
	if err != nil {
		return false, err
	}

	host, _, err := net.SplitHostPort(u.Host)
	if err != nil {
		host = u.Host
	}

	i := strings.LastIndex(host, ".")
	tld := host[i+1:]

	return tld == "local", nil
}

// Converts an image name to a url if it is not a URL
func getImageURL(image string) string {
	// Pull from dockerhub by default if not specified (default k8s behavior)
	if strings.Contains(image, ".") {
		return image
	}
	return "docker.io/" + image

}

func (o *Orchestrator) getImage(ctx context.Context, imageName string) (*containerd.Image, error) {
	image, found := o.cachedImages[imageName]
	if !found {
		var err error
		log.Debug(fmt.Sprintf("Pulling image %s", imageName))

		imageURL := getImageURL(imageName)
		local, _ := isLocalDomain(imageURL)
		if local {
			// Pull local image using HTTP
			resolver := docker.NewResolver(docker.ResolverOptions{
				Client: http.DefaultClient,
				Hosts: docker.ConfigureDefaultRegistries(
					docker.WithPlainHTTP(docker.MatchAllHosts),
				),
			})
			image, err = o.client.Pull(ctx, imageURL,
				containerd.WithPullUnpack,
				containerd.WithPullSnapshotter(o.snapshotter),
				containerd.WithResolver(resolver),
			)
		} else {
			// Pull remote image
			image, err = o.client.Pull(ctx, imageURL,
				containerd.WithPullUnpack,
				containerd.WithPullSnapshotter(o.snapshotter),
			)
		}

		if err != nil {
			return &image, err
		}
		o.cachedImages[imageName] = image
	}

	return &image, nil
}

func getK8sDNS() []string {
	//using googleDNS as a backup
	dnsIPs := []string{"8.8.8.8"}
	//get k8s DNS clusterIP
	cmd := exec.Command(
		"kubectl", "get", "service", "-n", "kube-system", "kube-dns", "-o=custom-columns=:.spec.clusterIP", "--no-headers",
	)
	stdoutStderr, err := cmd.CombinedOutput()
	if err != nil {
		log.Warnf("Failed to Fetch k8s dns clusterIP %v\n%s\n", err, stdoutStderr)
		log.Warnf("Using google dns %s\n", dnsIPs[0])
	} else {
		//adding k8s DNS clusterIP to the list
		dnsIPs = []string{strings.TrimSpace(string(stdoutStderr)), dnsIPs[0]}
	}
	return dnsIPs
}

func (o *Orchestrator) getVMCreateRequest(vm *misc.VM) *proto.CreateVMRequest {
	kernelArgs := "ro noapic reboot=k panic=1 pci=off nomodules systemd.log_color=false systemd.unit=firecracker.target init=/sbin/overlay-init tsc=reliable quiet 8250.nr_uarts=0 ipv6.disable=1"

	return &proto.CreateVMRequest{
		VMID:           vm.ID,
		TimeoutSeconds: 100,
		KernelArgs:     kernelArgs,
		MachineCfg: &proto.FirecrackerMachineConfiguration{
			VcpuCount:  1,
			MemSizeMib: 256,
		},
		NetworkInterfaces: []*proto.FirecrackerNetworkInterface{{
			StaticConfig: &proto.StaticNetworkConfiguration{
				MacAddress:  vm.Ni.MacAddress,
				HostDevName: vm.Ni.HostDevName,
				IPConfig: &proto.IPConfiguration{
					PrimaryAddr: vm.Ni.PrimaryAddress + vm.Ni.Subnet,
					GatewayAddr: vm.Ni.GatewayAddress,
					Nameservers: getK8sDNS(),
				},
			},
		}},
	}
}

// StopActiveVMs Shuts down all active VMs
func (o *Orchestrator) StopActiveVMs() error {
	var vmGroup sync.WaitGroup
	for vmID, vm := range o.vmPool.GetVMMap() {
		vmGroup.Add(1)
		logger := log.WithFields(log.Fields{"vmID": vmID})
		go func(vmID string, vm *misc.VM) {
			defer vmGroup.Done()
			err := o.StopSingleVM(context.Background(), vmID)
			if err != nil {
				logger.Warn(err)
			}
		}(vmID, vm)
	}

	log.Info("waiting for goroutines")
	vmGroup.Wait()
	log.Info("waiting done")

	log.Info("Closing fcClient")
	o.fcClient.Close()
	log.Info("Closing containerd client")
	o.client.Close()

	return nil
}

// PauseVM Pauses a VM
func (o *Orchestrator) PauseVM(ctx context.Context, vmID string) error {
	logger := log.WithFields(log.Fields{"vmID": vmID})
	logger.Debug("Orchestrator received PauseVM")

	ctx = namespaces.WithNamespace(ctx, namespaceName)

	if _, err := o.fcClient.PauseVM(ctx, &proto.PauseVMRequest{VMID: vmID}); err != nil {
		logger.WithError(err).Error("failed to pause the VM")
		return err
	}

	return nil
}

// ResumeVM Resumes a VM
func (o *Orchestrator) ResumeVM(ctx context.Context, vmID string) (*metrics.Metric, error) {
	var (
		resumeVMMetric *metrics.Metric = metrics.NewMetric()
		tStart         time.Time
	)

	logger := log.WithFields(log.Fields{"vmID": vmID})
	logger.Debug("Orchestrator received ResumeVM")

	ctx = namespaces.WithNamespace(ctx, namespaceName)

	tStart = time.Now()
	if _, err := o.fcClient.ResumeVM(ctx, &proto.ResumeVMRequest{VMID: vmID}); err != nil {
		logger.WithError(err).Error("failed to resume the VM")
		return nil, err
	}
	resumeVMMetric.MetricMap[metrics.FcResume] = metrics.ToUS(time.Since(tStart))

	return resumeVMMetric, nil
}

// CreateSnapshot Creates a snapshot of a VM
func (o *Orchestrator) CreateSnapshot(ctx context.Context, vmID string) error {
	logger := log.WithFields(log.Fields{"vmID": vmID})
	logger.Debug("Orchestrator received CreateSnapshot")

	ctx = namespaces.WithNamespace(ctx, namespaceName)

	req := &proto.CreateSnapshotRequest{
		VMID:             vmID,
		MemFilePath:      o.getMemoryFile(vmID),
		SnapshotFilePath: o.getSnapshotFile(vmID),
	}

	if _, err := o.fcClient.CreateSnapshot(ctx, req); err != nil {
		logger.WithError(err).Error("failed to create snapshot of the VM")
		return err
	}

	return nil
}

func (o *Orchestrator) Offload(ctx context.Context, vmID string) error {
	logger := log.WithFields(log.Fields{"vmID": vmID})
	logger.Debug("Orchestrator received Offload")

	ctx = namespaces.WithNamespace(ctx, namespaceName)

	vm, err := o.vmPool.GetVM(vmID)
	if err != nil {
		var nonExistErr *misc.NonExistErr
		if errors.As(err, &nonExistErr) {
			logger.Panic("Offload: VM does not exist")
		}
		logger.Panic("Offload: GetVM() failed for an unknown reason")

	}

	if _, err := o.fcClient.StopVM(ctx, &proto.StopVMRequest{VMID: vm.ID}); err != nil {
		logger.WithError(err).Error("failed to stop the VM")
		return err
	}

	if err := o.vmPool.RecreateTap(vmID, o.hostIface); err != nil {
		logger.Error("Failed to recreate tap upon offloading")
		return err
	}

	return nil
}

func (o *Orchestrator) StartVMFromSnapshot(ctx context.Context, vmID string) (_ *StartVMResponse, _ *metrics.Metric, retErr error) {
	var (
		startVMMetric *metrics.Metric = metrics.NewMetric()
		tStart        time.Time
	)

	logger := log.WithFields(log.Fields{"vmID": vmID})
	logger.Debug("StartVM: Received StartVM")

	memoryFile := o.getMemoryFile(vmID)
	if _, err := os.Stat(memoryFile); os.IsNotExist(err) {
		return nil, nil, errors.Wrapf(err, "Failed to get memory file for VM %s at %s", vmID, memoryFile)
	}
	snapshotFile := o.getSnapshotFile(vmID)
	if _, err := os.Stat(snapshotFile); os.IsNotExist(err) {
		return nil, nil, errors.Wrapf(err, "Failed to get snapshot file for VM %s at %s", vmID, snapshotFile)
	}

	vm, err := o.vmPool.GetVM(vmID)
	if err != nil {
		if _, ok := err.(*misc.NonExistErr); ok {
			logger.Panic("StartVM: VM does not exist")
		}
		logger.Panic("StartVM: GetVM() failed for an unknown reason")

	}

	ctx = namespaces.WithNamespace(ctx, namespaceName)

	createVMRequest := o.getVMCreateRequest(vm)
	createVMRequest.SnapshotCfg = &proto.FirecrackerSnapshotConfiguration{
		MemFilePath:         memoryFile,
		SnapshotPath:        snapshotFile,
		EnableDiffSnapshots: false,
		ResumeVM:            true,
	}

	tStart = time.Now()
	_, err = o.fcClient.CreateVM(ctx, createVMRequest)
	startVMMetric.MetricMap[metrics.FcCreateVM] = metrics.ToUS(time.Since(tStart))
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to create the microVM in firecracker-containerd")
	}

	defer func() {
		if retErr != nil {
			if _, err := o.fcClient.StopVM(ctx, &proto.StopVMRequest{VMID: vmID}); err != nil {
				logger.WithError(err).Errorf("failed to stop firecracker-containerd VM after failure")
			}
		}
	}()

	logger.Debug("Successfully started a VM from snapshot")

	return &StartVMResponse{GuestIP: vm.Ni.PrimaryAddress}, startVMMetric, nil
}
