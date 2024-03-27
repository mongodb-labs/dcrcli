package fscopy

import (
	"bytes"
	"testing"
)

// Testing for
// - local copy job
// - remote copy job with Default sync port
// - remote copy job with non-Default sync port

// - local copy job
func TestStartCopyLocal(t *testing.T) {
	fcj := FSCopyJob{
		SourceDir{
			true,
			[]byte(`/Users/nishant/myprojects/testclusters/standalone/data/db/diagnostic.data/`),
			[]byte(``),
			0,
			[]byte(`ubuntu`),
		},
		DestDir{
			[]byte(`/Users/nishant/myprojects/dcrcliProject/branches/remotecopier/dcrcli/outputs`),
		},
		"N",
		&bytes.Buffer{},
	}
	err := fcj.StartCopy()
	if err != nil {
		t.Error(err.Error())
	}
}

// - remote copy job with Default sync port
func TestStartCopyRemote(t *testing.T) {
	fcj := FSCopyJob{
		SourceDir{
			false,
			[]byte(`/var/lib/mongodb/diagnostic.data/`),
			[]byte(`ec2-13-234-136-113.ap-south-1.compute.amazonaws.com`),
			0,
			[]byte(`ubuntu`),
		},
		DestDir{
			[]byte(`/Users/nishant/myprojects/dcrcliProject/branches/remotecopier/dcrcli/outputs/`),
		},
		"N",
		&bytes.Buffer{},
	}
	err := fcj.StartCopy()
	if err != nil {
		t.Error(err.Error())
	}
}
