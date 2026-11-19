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

package collectorlogger

import (
	"log/slog"
	"os"
	"strconv"
	"time"
)

type CollectorLogger struct {
	OutputPrefix  string
	FileName      string
	LogFileHandle *os.File
	Logger        *slog.Logger
	LevelVar      *slog.LevelVar
}

func (dl *CollectorLogger) CreateLogFileHandle() error {
	var err error
	dl.LogFileHandle, err = os.Create(dl.Path())
	return err
}

func (dl *CollectorLogger) Create() error {
	err := dl.CreateLogFileHandle()
	if err != nil {
		return err
	}

	dl.LevelVar = new(slog.LevelVar)

	dljsonhandler := slog.NewJSONHandler(dl.LogFileHandle, &slog.HandlerOptions{Level: dl.LevelVar})

	dl.Logger = slog.New(dljsonhandler)

	return nil
}

func (dl *CollectorLogger) Path() string {
	currentUnixEpoch := time.Now().Local().Unix()
	return dl.OutputPrefix + dl.FileName + "_" + strconv.Itoa(int(currentUnixEpoch)) + ".log"
}

func (dl *CollectorLogger) Debug(msg string) {
	dl.Logger.Debug(msg)
}

func (dl *CollectorLogger) Info(msg string) {
	dl.Logger.Info(msg)
}

func (dl *CollectorLogger) Warn(msg string) {
	dl.Logger.Warn(msg)
}

func (dl *CollectorLogger) Error(msg string) {
	dl.Logger.Error(msg)
}

func (dl *CollectorLogger) SetLogLevel(sloglvl slog.Level) {
	dl.LevelVar.Set(sloglvl)
}
