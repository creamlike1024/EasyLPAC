package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

const AID_DEFAULT = "A0000005591010FFFFFFFF8900000100"
const AID_5BER = "A0000005591010FFFFFFFF8900050500"

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
	ConfigInstance.LpacDir = exeDir

	switch platform := runtime.GOOS; platform {
	case "windows":
		ConfigInstance.EXEName = "lpac.exe"
		ConfigInstance.LogDir = filepath.Join(exeDir, "log")
	case "linux":
		ConfigInstance.EXEName = "lpac"
		ConfigInstance.LogDir = filepath.Join("/tmp", "EasyLPAC-log")
		_, err = os.Stat(filepath.Join(ConfigInstance.LpacDir, ConfigInstance.EXEName))
		if err != nil {
			ConfigInstance.LpacDir = "/usr/bin"
		}
	default:
		ConfigInstance.EXEName = "lpac"
		ConfigInstance.LogDir = filepath.Join("/tmp", "EasyLPAC-log")
	}
	ConfigInstance.AutoMode = true
	ConfigInstance.LpacAID = AID_DEFAULT

	ConfigInstance.LogFilename = fmt.Sprintf("lpac-%s.txt", time.Now().Format("20060102-150405"))
	return nil
}
