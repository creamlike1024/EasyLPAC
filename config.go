package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

type Config struct {
	LpacDir     string
	LpacAID     string
	EXEName     string
	DriverIFID  string
	DebugHTTP   bool
	DebugAPDU   bool
	LogDir      string
	LogFilename string
	LogFile     *os.File
	AutoMode    bool
}

var ConfigInstance Config

func LoadConfig() error {
	exePath, err := os.Executable()
	if err != nil {
		return err
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return err
	}
	exeDir := filepath.Dir(exePath)

	switch platform := runtime.GOOS; platform {
	case "windows":
		ConfigInstance.EXEName = "lpac.exe"
		ConfigInstance.LpacDir = exeDir
		ConfigInstance.LogDir = filepath.Join(exeDir, "log")
	default:
		ConfigInstance.EXEName = "lpac"
		ConfigInstance.LpacDir = exeDir
		ConfigInstance.LogDir = filepath.Join("/tmp", "EasyLPAC-log")
	}
	ConfigInstance.AutoMode = true
	ConfigInstance.LpacAID = "A0000005591010FFFFFFFF8900000100"

	ConfigInstance.LogFilename = fmt.Sprintf("lpac-%s.txt", time.Now().Format("20060102-150405"))
	return nil
}
