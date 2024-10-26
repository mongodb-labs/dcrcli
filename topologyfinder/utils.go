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
	"strconv"
	"strings"

	"dcrcli/dcrlogger"
)

func splitHostPort(hostPort string, logger *dcrlogger.DCRLogger) (string, int, error) {
	colonPos := strings.IndexByte(hostPort, ':')

	logger.Debug(fmt.Sprintf("tf - sph - splitter colonPos: %d", colonPos))

	if colonPos == -1 {
		logger.Debug(fmt.Sprintf("tf - sph - error in splitting string on colon: %d", colonPos))
		return "", -1, fmt.Errorf("tf - sph - failed to parse hostport string")
	}

	hostname := hostPort[:colonPos]
	listenPort := hostPort[colonPos+1:]

	port, err := strconv.Atoi(listenPort)
	if err != nil {
		logger.Debug(
			fmt.Sprintf(
				"tf - sph - error in converting listenPort string to int (host, port): (%s, %s)",
				hostname,
				listenPort,
			),
		)
		return "", -1, fmt.Errorf(
			"tf - sph - invalid port string format for node %s: %s ",
			hostname,
			listenPort,
		)
	}

	return hostname, port, nil
}
