// Copyright 2023 MongoDB Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package webui

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"dcrcli/dcrlogger"
	"dcrcli/diskspace"
	"dcrcli/topologyfinder"
)

func isMongoNodeAlive(hostname string, port int) (bool, error) {
	return isMongoNodeAliveContext(context.Background(), hostname, port)
}

func isMongoNodeAliveContext(ctx context.Context, hostname string, port int) (bool, error) {
	dialer := net.Dialer{Timeout: 5 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(hostname, strconv.Itoa(port)))
	if err != nil {
		return false, err
	}
	conn.Close()
	return true, nil
}

func checkAllNodesAlive(nodes []topologyfinder.ClusterNode, dcrlog *dcrlogger.DCRLogger) ([]topologyfinder.ClusterNode, error) {
	return checkAllNodesAliveContext(context.Background(), nodes, dcrlog)
}

func checkAllNodesAliveContext(ctx context.Context, nodes []topologyfinder.ClusterNode, dcrlog *dcrlogger.DCRLogger) ([]topologyfinder.ClusterNode, error) {
	unhealthy := make([]topologyfinder.ClusterNode, 0)
	var lastErr error
	for _, n := range nodes {
		alive, err := isMongoNodeAliveContext(ctx, n.Hostname, n.Port)
		if !alive {
			if err != nil {
				lastErr = err
				dcrlog.Error(fmt.Sprintf("Health check: MongoDB node %s:%d is unreachable: %v", n.Hostname, n.Port, err))
			} else {
				dcrlog.Error(fmt.Sprintf("Health check: MongoDB node %s:%d is unreachable", n.Hostname, n.Port))
			}
			unhealthy = append(unhealthy, n)
			continue
		}
		dcrlog.Debug(fmt.Sprintf("Health check: MongoDB node %s:%d is reachable", n.Hostname, n.Port))
	}
	return unhealthy, lastErr
}

func unhealthyError(nodes []topologyfinder.ClusterNode, phase string, dcrlog *dcrlogger.DCRLogger) error {
	return unhealthyErrorContext(context.Background(), nodes, phase, dcrlog)
}

func unhealthyErrorContext(ctx context.Context, nodes []topologyfinder.ClusterNode, phase string, dcrlog *dcrlogger.DCRLogger) error {
	dcrlog.Info(fmt.Sprintf("Health check (%s): probing %d cluster node(s)", phase, len(nodes)))
	unhealthy, lastErr := checkAllNodesAliveContext(ctx, nodes, dcrlog)
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(unhealthy) == 0 {
		dcrlog.Info(fmt.Sprintf("Health check (%s): all %d node(s) reachable", phase, len(nodes)))
		return nil
	}
	lines := []string{
		fmt.Sprintf("Cluster health check failed (%s).", phase),
		"The following MongoDB node(s) are unreachable:",
	}
	for _, u := range unhealthy {
		lines = append(lines, fmt.Sprintf("  - %s:%d", u.Hostname, u.Port))
	}
	lines = append(lines,
		"MongoDB Data Collector collects diagnostic data against live clusters; refusing to proceed while any cluster node is down to avoid added production risk.",
		"Verify all members are healthy (for example rs.status()) and retry.",
	)
	if lastErr != nil {
		lines = append(lines, fmt.Sprintf("Last connection error: %v", lastErr))
	}
	dcrlog.Error(fmt.Sprintf("Stopping collection: %d cluster node(s) unhealthy during %s health check", len(unhealthy), phase))
	return fmt.Errorf("%s", strings.Join(lines, "\n"))
}

func hasFreeSpace() (bool, error) {
	processwd, err := os.Getwd()
	if err != nil {
		return false, err
	}
	const minimumFreeBytes = 1181116006 // 1.1 GiB
	return diskspace.HasAtLeast(processwd, minimumFreeBytes)
}

func isDirectoryExist(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

func checkEmptyDirectory(outputPrefix string) string {
	if isDirectoryExist(outputPrefix) {
		return outputPrefix[:len(outputPrefix)-1] + "_" + strconv.FormatInt(time.Now().Unix(), 10) + "/"
	}
	return outputPrefix
}

func isHostnameALocalHost(hostname string) (bool, error) {
	resolvedIPsForHostname, err := net.LookupIP(hostname)
	if err != nil {
		return false, err
	}
	localMachineInterfaces, err := net.Interfaces()
	if err != nil {
		return false, err
	}
	localIPs := make([]net.IP, 0, 255)
	for i := 1; i <= 255; i++ {
		localIPs = append(localIPs, net.IPv4(127, 0, 0, byte(i)))
	}
	for _, iface := range localMachineInterfaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip != nil && !ip.IsLoopback() {
				localIPs = append(localIPs, ip)
			}
		}
	}
	for _, resolvedIP := range resolvedIPsForHostname {
		for _, localIP := range localIPs {
			if resolvedIP.Equal(localIP) {
				return true, nil
			}
		}
	}
	return false, nil
}

func remoteCopySkippedLabels(runFTDC, runLogs bool) string {
	var parts []string
	if runFTDC {
		parts = append(parts, "FTDC")
	}
	if runLogs {
		parts = append(parts, "mongod logs")
	}
	return strings.Join(parts, " and ")
}
