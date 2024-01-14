package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

type Config struct {
	LpacDir       string
	EXEName       string
	APDUInterface string
	HTTPInterface string
	DriverIFID    string
	LogDir        string
	LogFilename   string
	LogFile       *os.File
}

var ConfigInstance Config

func LoadConfig() error {
	switch platform := runtime.GOOS; platform {
	case "windows":
		ConfigInstance.EXEName = "lpac.exe"
		ConfigInstance.APDUInterface = "libapduinterface_pcsc.dll"
		ConfigInstance.HTTPInterface = "libhttpinterface_curl.dll"
		pwd, err := os.Getwd()
		if err != nil {
			return err
		}
		ConfigInstance.LpacDir = filepath.Join(pwd, "lpac")
		ConfigInstance.LogDir = filepath.Join(pwd, "log")
	case "darwin":
		ConfigInstance.EXEName = "lpac"
		ConfigInstance.APDUInterface = "libapduinterface_pcsc.dylib"
		ConfigInstance.HTTPInterface = "libhttpinterface_curl.dylib"
		exePath, err := os.Executable()
		if err != nil {
			return err
		}
		ConfigInstance.LpacDir = filepath.Join(filepath.Dir(exePath), "lpac")
		ConfigInstance.LogDir = filepath.Join("/tmp", "EasyLPAC-log")
	default:
		ConfigInstance.EXEName = "lpac"
		ConfigInstance.APDUInterface = "libapduinterface_pcsc.so"
		ConfigInstance.HTTPInterface = "libhttpinterface_curl.so"
		pwd, err := os.Getwd()
		if err != nil {
			return err
		}
		ConfigInstance.LpacDir = filepath.Join(pwd, "lpac")
		ConfigInstance.LogDir = filepath.Join("/tmp", "EasyLPAC-log")
	}

	now := time.Now()
	ConfigInstance.LogFilename = fmt.Sprintf("lpac-%s.txt", now.Format("20060102-150405"))
	return nil
}
