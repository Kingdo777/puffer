// MIT License
//
// # Copyright (c) 2020 Dmitrii Ustiugov, Plamen Petrov and EASE lab
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
	"os"
	"testing"
	"time"

	ctrdlog "github.com/containerd/containerd/log"
	"github.com/containerd/containerd/namespaces"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

// test start container
func TestStartContainer(t *testing.T) {
	log.SetFormatter(&log.TextFormatter{
		TimestampFormat: ctrdlog.RFC3339NanoFixed,
		FullTimestamp:   true,
	})
	log.SetReportCaller(true) // FIXME: make sure it's false unless debugging

	log.SetOutput(os.Stdout)

	log.SetLevel(log.DebugLevel)

	testTimeout := 120 * time.Second
	ctx, cancel := context.WithTimeout(namespaces.WithNamespace(context.Background(), namespaceName), testTimeout)
	defer cancel()

	orch := NewOrchestrator(
		"devmapper",
		"",
	)

	vmID := "1"

	_, _, err := orch.StartVM(ctx, vmID, testImageName)
	require.NoError(t, err, "Failed to start VM")

	err = orch.StopSingleVM(ctx, vmID)
	require.NoError(t, err, "Failed to stop VM")

	orch.Cleanup()
}

//func TestStartStopSerial(t *testing.T) {
//	log.SetFormatter(&log.TextFormatter{
//		TimestampFormat: ctrdlog.RFC3339NanoFixed,
//		FullTimestamp:   true,
//	})
//	//log.SetReportCaller(true) // FIXME: make sure it's false unless debugging
//
//	log.SetOutput(os.Stdout)
//
//	log.SetLevel(log.InfoLevel)
//
//	testTimeout := 120 * time.Second
//	ctx, cancel := context.WithTimeout(namespaces.WithNamespace(context.Background(), namespaceName), testTimeout)
//	defer cancel()
//
//	orch := NewOrchestrator(
//		"devmapper",
//		"",
//		WithTestModeOn(true),
//		WithUPF(*isUPFEnabled),
//		WithLazyMode(*isLazyMode),
//	)
//
//	vmID := "5"
//
//	_, _, err := orch.StartVM(ctx, vmID, testImageName)
//	require.NoError(t, err, "Failed to start VM")
//
//	err = orch.StopSingleVM(ctx, vmID)
//	require.NoError(t, err, "Failed to stop VM")
//
//	orch.Cleanup()
//}
//
//func TestPauseResumeSerial(t *testing.T) {
//	log.SetFormatter(&log.TextFormatter{
//		TimestampFormat: ctrdlog.RFC3339NanoFixed,
//		FullTimestamp:   true,
//	})
//	//log.SetReportCaller(true) // FIXME: make sure it's false unless debugging
//
//	log.SetOutput(os.Stdout)
//
//	log.SetLevel(log.InfoLevel)
//
//	testTimeout := 120 * time.Second
//	ctx, cancel := context.WithTimeout(namespaces.WithNamespace(context.Background(), namespaceName), testTimeout)
//	defer cancel()
//
//	orch := NewOrchestrator(
//		"devmapper",
//		"",
//		WithTestModeOn(true),
//		WithUPF(*isUPFEnabled),
//		WithLazyMode(*isLazyMode),
//	)
//
//	vmID := "6"
//
//	_, _, err := orch.StartVM(ctx, vmID, testImageName)
//	require.NoError(t, err, "Failed to start VM")
//
//	err = orch.PauseVM(ctx, vmID)
//	require.NoError(t, err, "Failed to pause VM")
//
//	_, err = orch.ResumeVM(ctx, vmID)
//	require.NoError(t, err, "Failed to resume VM")
//
//	err = orch.StopSingleVM(ctx, vmID)
//	require.NoError(t, err, "Failed to stop VM")
//
//	orch.Cleanup()
//}
//
//func TestStartStopParallel(t *testing.T) {
//	log.SetFormatter(&log.TextFormatter{
//		TimestampFormat: ctrdlog.RFC3339NanoFixed,
//		FullTimestamp:   true,
//	})
//	//log.SetReportCaller(true) // FIXME: make sure it's false unless debugging
//
//	log.SetOutput(os.Stdout)
//
//	log.SetLevel(log.InfoLevel)
//
//	testTimeout := 360 * time.Second
//	ctx, cancel := context.WithTimeout(namespaces.WithNamespace(context.Background(), namespaceName), testTimeout)
//	defer cancel()
//
//	vmNum := 10
//	orch := NewOrchestrator(
//		"devmapper",
//		"",
//		WithTestModeOn(true),
//		WithUPF(*isUPFEnabled),
//		WithLazyMode(*isLazyMode),
//	)
//
//	// Pull image
//	_, err := orch.getImage(ctx, testImageName)
//	require.NoError(t, err, "Failed to pull image "+testImageName)
//
//	{
//		var vmGroup sync.WaitGroup
//		for i := 0; i < vmNum; i++ {
//			vmGroup.Add(1)
//			go func(i int) {
//				defer vmGroup.Done()
//				vmID := fmt.Sprintf("%d", i)
//				_, _, err := orch.StartVM(ctx, vmID, testImageName)
//				require.NoError(t, err, "Failed to start VM "+vmID)
//			}(i)
//		}
//		vmGroup.Wait()
//	}
//
//	{
//		var vmGroup sync.WaitGroup
//		for i := 0; i < vmNum; i++ {
//			vmGroup.Add(1)
//			go func(i int) {
//				defer vmGroup.Done()
//				vmID := fmt.Sprintf("%d", i)
//				err := orch.StopSingleVM(ctx, vmID)
//				require.NoError(t, err, "Failed to stop VM "+vmID)
//			}(i)
//		}
//		vmGroup.Wait()
//	}
//
//	orch.Cleanup()
//}
//
//func TestPauseResumeParallel(t *testing.T) {
//	log.SetFormatter(&log.TextFormatter{
//		TimestampFormat: ctrdlog.RFC3339NanoFixed,
//		FullTimestamp:   true,
//	})
//	//log.SetReportCaller(true) // FIXME: make sure it's false unless debugging
//
//	log.SetOutput(os.Stdout)
//
//	log.SetLevel(log.InfoLevel)
//
//	testTimeout := 120 * time.Second
//	ctx, cancel := context.WithTimeout(namespaces.WithNamespace(context.Background(), namespaceName), testTimeout)
//	defer cancel()
//
//	vmNum := 10
//	orch := NewOrchestrator(
//		"devmapper",
//		"",
//		WithTestModeOn(true),
//		WithUPF(*isUPFEnabled),
//		WithLazyMode(*isLazyMode),
//	)
//
//	// Pull image
//	_, err := orch.getImage(ctx, testImageName)
//	require.NoError(t, err, "Failed to pull image "+testImageName)
//
//	{
//		var vmGroup sync.WaitGroup
//		for i := 0; i < vmNum; i++ {
//			vmGroup.Add(1)
//			go func(i int) {
//				defer vmGroup.Done()
//				vmID := fmt.Sprintf("%d", i)
//				_, _, err := orch.StartVM(ctx, vmID, testImageName)
//				require.NoError(t, err, "Failed to start VM")
//			}(i)
//		}
//		vmGroup.Wait()
//	}
//
//	{
//		var vmGroup sync.WaitGroup
//		for i := 0; i < vmNum; i++ {
//			vmGroup.Add(1)
//			go func(i int) {
//				defer vmGroup.Done()
//				vmID := fmt.Sprintf("%d", i)
//				err := orch.PauseVM(ctx, vmID)
//				require.NoError(t, err, "Failed to pause VM")
//			}(i)
//		}
//		vmGroup.Wait()
//	}
//
//	{
//		var vmGroup sync.WaitGroup
//		for i := 0; i < vmNum; i++ {
//			vmGroup.Add(1)
//			go func(i int) {
//				defer vmGroup.Done()
//				vmID := fmt.Sprintf("%d", i)
//				_, err := orch.ResumeVM(ctx, vmID)
//				require.NoError(t, err, "Failed to resume VM")
//			}(i)
//		}
//		vmGroup.Wait()
//	}
//
//	{
//		var vmGroup sync.WaitGroup
//		for i := 0; i < vmNum; i++ {
//			vmGroup.Add(1)
//			go func(i int) {
//				defer vmGroup.Done()
//				vmID := fmt.Sprintf("%d", i)
//				err := orch.StopSingleVM(ctx, vmID)
//				require.NoError(t, err, "Failed to stop VM")
//			}(i)
//		}
//		vmGroup.Wait()
//	}
//
//	orch.Cleanup()
//}
