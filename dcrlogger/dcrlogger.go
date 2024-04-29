package dcrlogger

import (
	"log/slog"
	"os"
	"strconv"
	"time"
)

type DCRLogger struct {
	OutputPrefix  string
	FileName      string
	LogFileHandle *os.File
	Logger        *slog.Logger
}

func (dl *DCRLogger) CreateLogFileHandle() error {
	var err error
	dl.LogFileHandle, err = os.Create(dl.Path())
	return err
}

func (dl *DCRLogger) Create() error {
	err := dl.CreateLogFileHandle()
	if err != nil {
		return err
	}
	dl.Logger = slog.New(slog.NewJSONHandler(dl.LogFileHandle, nil))
	return nil
}

func (dl *DCRLogger) Path() string {
	currentUnixEpoch := time.Now().Local().Unix()
	return dl.OutputPrefix + dl.FileName + "_" + strconv.Itoa(int(currentUnixEpoch)) + ".log"
}

func (dl *DCRLogger) Debug(msg string) {
	dl.Logger.Debug(msg)
}

func (dl *DCRLogger) Info(msg string) {
	dl.Logger.Info(msg)
}

func (dl *DCRLogger) Warn(msg string) {
	dl.Logger.Warn(msg)
}

func (dl *DCRLogger) Error(msg string) {
	dl.Logger.Error(msg)
}
