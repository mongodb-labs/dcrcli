// Copyright 2023 MongoDB Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// See the License for the specific language governing permissions and
// limitations under the License.

package webui

import "testing"

func TestLoopbackAddressesAreLocal(t *testing.T) {
	for _, host := range []string{"127.0.1.1", "127.5.5.5", "::1", "localhost"} {
		local, err := isHostnameALocalHost(host)
		if err != nil {
			t.Fatalf("%s: %v", host, err)
		}
		if !local {
			t.Errorf("%s should be treated as this machine", host)
		}
	}
}
