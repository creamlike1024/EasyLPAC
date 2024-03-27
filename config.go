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
	EXEName     string
	DriverIFID  string
	DebugHTTP   bool
	DebugAPDU   bool
	LogDir      string
	LogFilename string
	LogFile     *os.File
}

var ConfigInstance Config

func LoadConfig() error {
	ConfigInstance.DebugAPDU = true
	ConfigInstance.DebugHTTP = true

	switch platform := runtime.GOOS; platform {
	case "windows":
		ConfigInstance.EXEName = "lpac.exe"
		pwd, err := os.Getwd()
		if err != nil {
			return err
		}
		ConfigInstance.LpacDir = pwd
		ConfigInstance.LogDir = filepath.Join(pwd, "log")
	case "darwin":
		ConfigInstance.EXEName = "lpac"
		exePath, err := os.Executable()
		if err != nil {
			return err
		}
		ConfigInstance.LpacDir = filepath.Dir(exePath)
		ConfigInstance.LogDir = filepath.Join("/tmp", "EasyLPAC-log")
	default:
		ConfigInstance.EXEName = "lpac"
		pwd, err := os.Getwd()
		if err != nil {
			return err
		}
		ConfigInstance.LpacDir = pwd
		ConfigInstance.LogDir = filepath.Join("/tmp", "EasyLPAC-log")
	}

	ConfigInstance.LogFilename = fmt.Sprintf("lpac-%s.txt", time.Now().Format("20060102-150405"))
	return nil
}
