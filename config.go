package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

const LpacDir = "lpac"
const LogDir = "log"

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
	pwd, err := os.Getwd()
	if err != nil {
		return err
	}
	ConfigInstance.LpacDir = filepath.Join(pwd, LpacDir)
	switch platform := runtime.GOOS; platform {
	case "windows":
		ConfigInstance.EXEName = "lpac.exe"
		ConfigInstance.APDUInterface = "libapduinterface_pcsc.dll"
		ConfigInstance.HTTPInterface = "libhttpinterface_curl.dll"
	case "darwin":
		ConfigInstance.EXEName = "lpac"
		ConfigInstance.APDUInterface = "libapduinterface_pcsc.dylib"
		ConfigInstance.HTTPInterface = "libhttpinterface_curl.dylib"
	default:
		ConfigInstance.EXEName = "lpac"
		ConfigInstance.APDUInterface = "libapduinterface_pcsc.so"
		ConfigInstance.HTTPInterface = "libhttpinterface_curl.so"
	}

	now := time.Now()
	ConfigInstance.LogFilename = fmt.Sprintf("output-%s.txt", now.Format("20060102-150405"))
	ConfigInstance.LogDir = filepath.Join(pwd, LogDir)
	return nil
}
