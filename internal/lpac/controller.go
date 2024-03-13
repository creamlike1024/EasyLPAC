package lpac

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
)

type Controller struct {
	ExecutablePath  string
	Logger          *slog.Logger
	APDUInterface   string
	HTTPInterface   string
	DriverInterface string
	DebugHTTP       bool
	DebugAPDU       bool
	Stdio           StdioInterface
	mux             sync.Mutex
	invokeId        atomic.Int32
}

func (c *Controller) invoke(args ...string) (json.RawMessage, error) {
	return c.invokeWithProgress(nil, args...)
}

func (c *Controller) invokeWithProgress(steps chan<- string, args ...string) (_ json.RawMessage, err error) {
	c.mux.Lock()
	defer c.mux.Unlock()
	extension := ""
	switch runtime.GOOS {
	case "windows":
		extension = ".exe"
	}
	cmd := exec.Command(filepath.Clean(c.ExecutablePath+extension), args...)
	cmd.Dir = filepath.Dir(c.ExecutablePath)
	cmd.Env = c.environments()
	hideWindow(cmd)
	var logger *slog.Logger
	logger = c.Logger.With("id", c.invokeId.Add(1))
	logger.Info("Start", "cmd", strings.TrimSpace(strings.TrimPrefix(cmd.String(), cmd.Path)))
	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()
	if err = cmd.Start(); err != nil {
		return
	}
	go c.handleError(logger, stderr)
	errs := make(chan error, 1)
	returns := make(chan json.RawMessage, 1)
	go func() {
		if message, err := c.handle(logger, steps, stdin, stdout); err != nil {
			errs <- err
		} else {
			returns <- message
		}
	}()
	select {
	case err = <-errs:
		logger.Error(err.Error())
		return
	case message := <-returns:
		logger.Info("End")
		return message, nil
	}
}

func (c *Controller) handle(logger *slog.Logger, steps chan<- string, stdin io.Writer, stdout io.Reader) (_ json.RawMessage, err error) {
	var response _Response[json.RawMessage]
	encoder := json.NewEncoder(stdin)
	decoder := json.NewDecoder(stdout)
	for decoder.More() {
		if err = decoder.Decode(&response); err != nil {
			logger.Error(err.Error())
			return
		}
		logger.Debug(string(response.Payload), "type", response.Type, "std", "in")
		switch response.Type {
		case "progress":
			if steps == nil {
				continue
			}
			var payload _Payload
			if err = json.Unmarshal(response.Payload, &payload); err != nil {
				return
			}
			steps <- payload.Message
		case "driver":
			var payload _DriverPayload
			if err = json.Unmarshal(response.Payload, &payload); err != nil {
				return
			}
			return payload.Data, nil
		case "lpa":
			var payload _Payload
			if err = json.Unmarshal(response.Payload, &payload); err != nil {
				return
			}
			if payload.Code != 0 {
				var details string
				if err = json.Unmarshal(payload.Data, &details); err != nil {
					return
				}
				err = &Error{FunctionName: payload.Message, Details: details}
				return
			}
			return payload.Data, nil
		case "apdu":
			var request StdioAPDURequest
			if err = json.Unmarshal(response.Payload, &request); err != nil {
				return
			}
			if response.Payload, err = json.Marshal(c.Stdio.APDU(&request)); err != nil {
				return
			}
			if err = encoder.Encode(response); err != nil {
				return
			}
			logger.Debug(string(response.Payload), "type", response.Type, "std", "out")
		case "http":
			var request StdioHTTPRequest
			if err = json.Unmarshal(response.Payload, &request); err != nil {
				return
			}
			if response.Payload, err = json.Marshal(c.Stdio.HTTP(&request)); err != nil {
				return
			}
			if err = encoder.Encode(response); err != nil {
				return
			}
			logger.Debug(string(response.Payload), "type", response.Type, "std", "out")
		default:
			break
		}
	}
	return nil, errors.ErrUnsupported
}

func (c *Controller) handleError(logger *slog.Logger, stderr io.Reader) {
	scanner := bufio.NewScanner(stderr)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		text := scanner.Text()
		if strings.HasPrefix(text, "[DEBUG]") {
			logger.Debug(text, "std", "err")
		} else {
			logger.Info(scanner.Text(), "std", "err")
		}
	}
}

func (c *Controller) environments() (environs []string) {
	extension := ".so"
	switch runtime.GOOS {
	case "windows":
		extension = ".dll"
	case "darwin":
		extension = ".dylib"
	}
	environs = []string{
		"APDU_INTERFACE=" + filepath.Clean(c.APDUInterface+extension),
		"HTTP_INTERFACE=" + filepath.Clean(c.HTTPInterface+extension),
	}
	if c.DriverInterface != "" {
		environs = append(environs, "DRIVER_IFID="+c.DriverInterface)
	}
	if c.DebugHTTP {
		environs = append(environs, "LIBEUICC_DEBUG_HTTP=1")
	}
	if c.DebugAPDU {
		environs = append(environs, "LIBEUICC_DEBUG_APDU=1")
	}
	return
}
