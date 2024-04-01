package dcroutdir

import (
	"os"
)

type DCROutputDir struct {
	OutputPrefix string
	Hostname     string
	Port         string
}

func (od *DCROutputDir) CreateDCROutputDir() error {
	err := os.MkdirAll(od.Path(), 0744)
	if err != nil {
		return err
	}
	return nil
}

func (od *DCROutputDir) Path() string {
	return od.OutputPrefix + od.Hostname + "_" + od.Port
}
