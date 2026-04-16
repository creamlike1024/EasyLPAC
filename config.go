package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"time"
)

const AID_DEFAULT = "A0000005591010FFFFFFFF8900000100"
const AID_5BER = "A0000005591010FFFFFFFF8900050500"
const AID_ESIMME = "A0000005591010000000008900000300"
const AID_XESIM = "A0000005591010FFFFFFFF8900000177"

// DriverConfig holds configuration for a specific APDU backend driver
type DriverConfig struct {
	DevicePath string // Device path (for at, mbim, qmi drivers)
	UimSlot    int    // UIM slot number (for mbim, qmi drivers)
	DriverIFID string // Driver interface ID / index (for pcsc driver enumeration)
	DriverName string // Human-readable reader name (for pcsc, to work around lpac name-filter bug)
}

type Config struct {
	LpacDir     string
	LpacAID     string
	EXEName     string
	DebugHTTP   bool
	DebugAPDU   bool
	LogDir      string
	LogFilename string
	LogFile     *os.File
	AutoMode    bool

	// APDU backend configuration
	ApduBackend   string                  // Current backend: pcsc, mbim, at, qmi, etc.
	DriverConfigs map[string]DriverConfig // Config per driver type
}

var ConfigInstance Config

// AvailableDrivers holds the list of drivers returned by lpac
var AvailableDrivers []string

// DriversWithEnumeration are drivers that support device listing via combobox
var DriversWithEnumeration = map[string]bool{
	"pcsc": true,
	"at":   true,
}

// DriversWithDevicePath are drivers that need a device path
var DriversWithDevicePath = map[string]bool{
	"at":       true,
	"at_csim":  true,
	"mbim":     true,
	"qmi":      true,
	"qmi_qrtr": true,
	"uqmi":     true,
}

// DriversWithUimSlot are drivers that need a UIM slot number
var DriversWithUimSlot = map[string]bool{
	"mbim":     true,
	"qmi":      true,
	"qmi_qrtr": true,
	"uqmi":     true,
}

// DriversNoConfig are drivers that need no configuration
var DriversNoConfig = map[string]bool{
	"gbinder":      true,
	"gbinder_hidl": true,
}

// IsKnownDriver returns true if the driver is recognized
func IsKnownDriver(driver string) bool {
	return DriversWithEnumeration[driver] ||
		DriversWithDevicePath[driver] ||
		DriversNoConfig[driver]
}

// GetDriverEnvVarName returns the environment variable name for a driver's device path
func GetDriverEnvVarName(driver string) string {
	switch driver {
	case "at", "at_csim":
		return "LPAC_APDU_AT_DEVICE"
	case "mbim":
		return "LPAC_APDU_MBIM_DEVICE"
	case "qmi", "qmi_qrtr", "uqmi":
		return "LPAC_APDU_QMI_DEVICE"
	case "pcsc":
		return "LPAC_APDU_PCSC_DRV_IFID"
	default:
		return ""
	}
}

// GetDriverSlotEnvVarName returns the environment variable name for a driver's UIM slot
func GetDriverSlotEnvVarName(driver string) string {
	switch driver {
	case "mbim":
		return "LPAC_APDU_MBIM_UIM_SLOT"
	case "qmi", "qmi_qrtr", "uqmi":
		return "LPAC_APDU_QMI_UIM_SLOT"
	default:
		return ""
	}
}

// GetDefaultDevicePath returns the default device path for a driver
func GetDefaultDevicePath(driver string) string {
	switch driver {
	case "at", "at_csim":
		if runtime.GOOS == "windows" {
			return "COM3"
		}
		return "/dev/ttyUSB0"
	case "mbim", "qmi", "qmi_qrtr", "uqmi":
		return "/dev/cdc-wdm0"
	default:
		return ""
	}
}

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
			// Try to find lpac in PATH
			if lpacPath, pathErr := exec.LookPath("lpac"); pathErr == nil {
				ConfigInstance.LpacDir = filepath.Dir(lpacPath)
			} else {
				ConfigInstance.LpacDir = "/usr/bin"
			}
		}
	default:
		ConfigInstance.EXEName = "lpac"
		ConfigInstance.LogDir = filepath.Join("/tmp", "EasyLPAC-log")
	}
	ConfigInstance.AutoMode = true
	ConfigInstance.LpacAID = AID_DEFAULT
	ConfigInstance.ApduBackend = "" // Will be set after driver discovery

	// Initialize driver configs with defaults from environment variables
	ConfigInstance.DriverConfigs = make(map[string]DriverConfig)

	// Initialize configs for all known drivers
	// Only use env variables, not defaults - defaults are shown as placeholders
	initDriverConfig("pcsc", os.Getenv("LPAC_APDU_PCSC_DRV_IFID"), 0)
	initDriverConfig("at", os.Getenv("LPAC_APDU_AT_DEVICE"), 0)
	initDriverConfig("at_csim", os.Getenv("LPAC_APDU_AT_DEVICE"), 0)
	initDriverConfig("mbim", os.Getenv("LPAC_APDU_MBIM_DEVICE"), getEnvIntOrDefault("LPAC_APDU_MBIM_UIM_SLOT", 1))
	initDriverConfig("qmi", os.Getenv("LPAC_APDU_QMI_DEVICE"), getEnvIntOrDefault("LPAC_APDU_QMI_UIM_SLOT", 1))
	initDriverConfig("qmi_qrtr", os.Getenv("LPAC_APDU_QMI_DEVICE"), getEnvIntOrDefault("LPAC_APDU_QMI_UIM_SLOT", 1))
	initDriverConfig("uqmi", os.Getenv("LPAC_APDU_QMI_DEVICE"), getEnvIntOrDefault("LPAC_APDU_QMI_UIM_SLOT", 1))
	initDriverConfig("gbinder", "", 0)
	initDriverConfig("gbinder_hidl", "", 0)

	ConfigInstance.LogFilename = fmt.Sprintf("lpac-%s.txt", time.Now().Format("20060102-150405"))
	return nil
}

func initDriverConfig(driver, devicePath string, uimSlot int) {
	ConfigInstance.DriverConfigs[driver] = DriverConfig{
		DevicePath: devicePath,
		UimSlot:    uimSlot,
		DriverIFID: "",
	}
}

func getEnvOrDefault(envVar, defaultVal string) string {
	if val := os.Getenv(envVar); val != "" {
		return val
	}
	return defaultVal
}

func getEnvIntOrDefault(envVar string, defaultVal int) int {
	if val := os.Getenv(envVar); val != "" {
		if intVal, err := strconv.Atoi(val); err == nil {
			return intVal
		}
	}
	return defaultVal
}

// GetCurrentDriverConfig returns the config for the currently selected driver
func GetCurrentDriverConfig() *DriverConfig {
	if config, ok := ConfigInstance.DriverConfigs[ConfigInstance.ApduBackend]; ok {
		return &config
	}
	return nil
}

// SetCurrentDriverConfig updates the config for the currently selected driver
func SetCurrentDriverConfig(config DriverConfig) {
	ConfigInstance.DriverConfigs[ConfigInstance.ApduBackend] = config
}
