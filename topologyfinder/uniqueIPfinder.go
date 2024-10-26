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

package topologyfinder

import (
	"fmt"
	"net"
	"strconv"

	"dcrcli/dcrlogger"
)

type UniqueIPfinder struct {
	AllNodes ClusterNodes
	Dcrlog   *dcrlogger.DCRLogger
}

func (uf *UniqueIPfinder) netLookupIP(host string) ([]net.IP, error) {
	uf.Dcrlog.Debug(fmt.Sprintf("tfuf - looking up IPs for host: %s", host))
	addrs, err := net.LookupIP(host)
	if err != nil {
		uf.Dcrlog.Error(fmt.Sprintf("tfuf - lookup %s: %v", host, err))
		return nil, fmt.Errorf("tfuf - lookup %s: %v", host, err)
	}
	validAddrs := make([]net.IP, 0, len(addrs))
	for _, addr := range addrs {
		if addr.To4() != nil {
			uf.Dcrlog.Debug(
				fmt.Sprintf("tfuf - found ipv4 addr %s for host %s", addr.String(), host),
			)
			validAddrs = append(validAddrs, addr)
		}
	}
	return validAddrs, nil
}

func (uf *UniqueIPfinder) IpportTohostportMap(hostportList []string) (map[string][]string, error) {
	// create an empty set
	// key : unique IP address
	// values : [hostname1, hostname2]
	ipportToHostPortSet := make(map[string][]string)

	uf.Dcrlog.Debug(fmt.Sprintf("tfuf - hostPortList is : %s", hostportList))

	for _, hostPort := range hostportList {
		uf.Dcrlog.Debug(fmt.Sprintf("tfuf - hostPort is : %s", hostPort))

		hostname, listenPort, err := splitHostPort(hostPort, uf.Dcrlog)
		if err != nil {
			uf.Dcrlog.Warn(
				fmt.Sprintf(
					"tfuf - unable to properly split hostPort string: %s with err: %v",
					hostPort,
					err,
				),
			)
			// skip empty elements if any
			continue
		}

		ipAddrs, err := uf.netLookupIP(hostname)
		// if one lookup fails skip that
		if err != nil {
			uf.Dcrlog.Warn(
				fmt.Sprintf("tfuf - lookup for hostname %s failed with err: %v", hostname,
					err),
			)
			continue
		}

		for _, ipAddr := range ipAddrs {
			// join ip address and port to form a unique key
			ipPortKey := fmt.Sprintf("%s:%d", ipAddr.String(), listenPort)
			// create a set with ip+port as the key with possible multiple hostnames
			ipportToHostPortSet[ipPortKey] = append(
				ipportToHostPortSet[ipPortKey],
				hostname+":"+strconv.Itoa(listenPort),
			)
		}
	}

	return ipportToHostPortSet, nil
}
