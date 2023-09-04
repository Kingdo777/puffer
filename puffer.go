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

package main

import (
	"flag"
	"github.com/Kingdo777/puffer/cri"
	fccri "github.com/Kingdo777/puffer/cri/firecracker"
	"github.com/Kingdo777/puffer/ctriface"
	ctrdlog "github.com/containerd/containerd/log"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"net"
	"os"
)

var (
	orch *ctriface.Orchestrator

	criSock   *string
	hostIface *string
)

func main() {
	snapshotter := flag.String("ss", "devmapper", "snapshotter name")
	debug := flag.Bool("dbg", false, "Enable debug logging")

	criSock = flag.String("criSock", "/run/puffer/puffer.sock", "Socket address for CRI service")
	hostIface = flag.String("hostIface", "", "Host net-interface for the VMs to bind to for internet access")
	sandbox := flag.String("sandbox", "firecracker", "Sandbox tech to use, valid options: firecracker, gvisor")
	flag.Parse()

	if *sandbox != "firecracker" {
		log.Fatalln("Only \"firecracker\" are supported as sandboxing-techniques")
		return
	}

	log.SetFormatter(&log.TextFormatter{
		TimestampFormat: ctrdlog.RFC3339NanoFixed,
		FullTimestamp:   true,
	})
	log.SetReportCaller(true) // FIXME: make sure it's false unless debugging

	log.SetOutput(os.Stdout)

	if *debug {
		log.SetLevel(log.DebugLevel)
		log.Debug("Debug logging is enabled")
	} else {
		log.SetLevel(log.InfoLevel)
	}

	switch *sandbox {
	case "firecracker":
		orch = ctriface.NewOrchestrator(
			*snapshotter,
			*hostIface,
			ctriface.WithSnapshots(true),
		)
		setupFirecrackerCRI()
	}
}

func setupFirecrackerCRI() {
	lis, err := net.Listen("unix", *criSock)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()

	fcService, err := fccri.NewFirecrackerService(orch)
	if err != nil {
		log.Fatalf("failed to create firecracker service %v", err)
	}

	criService, err := cri.NewService(fcService)
	if err != nil {
		log.Fatalf("failed to create CRI service %v", err)
	}

	criService.Register(s)

	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
