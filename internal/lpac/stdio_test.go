package lpac

import (
	"bytes"
	"crypto/hmac"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
)

type Stdio struct {
	apduOffset int
	httpOffset int
	APDUSteps  []StdioData       `json:"apdu-steps"`
	HTTPSteps  []json.RawMessage `json:"http-steps"`
}

func (s *Stdio) APDU(request *StdioAPDURequest) (response *StdioAPDUResponse) {
	response = new(StdioAPDUResponse)
	if s.apduOffset >= len(s.APDUSteps) {
		return
	}
	switch request.Name {
	case "transmit":
		if hmac.Equal(s.APDUSteps[s.apduOffset][5:], request.Param[5:]) {
			response.Data = s.APDUSteps[s.apduOffset+1]
			s.apduOffset += 2
		} else {
			response.ErrorCode = -1
		}
	}
	return
}

func (s *Stdio) HTTP(request *StdioHTTPRequest) (response *StdioHTTPResponse) {
	response = new(StdioHTTPResponse)
	var dst bytes.Buffer
	_ = json.Compact(&dst, s.HTTPSteps[s.httpOffset])
	if !hmac.Equal(dst.Bytes(), request.Body) {
		response.StatusCode = 500
		return
	}
	if s.HTTPSteps[s.httpOffset+1] == nil {
		response.StatusCode = 204
	} else {
		dst.Reset()
		_ = json.Compact(&dst, s.HTTPSteps[s.httpOffset+1])
		response.StatusCode = 200
		response.Body = dst.Bytes()
	}
	s.httpOffset += 2
	return
}

func LoadFixture(name string) *Controller {
	executablePath, err := filepath.Abs("../../lpac/lpac")
	if err != nil {
		panic(err)
	}
	controller := &Controller{
		ExecutablePath: executablePath,
		Logger:         slog.Default(),
		APDUInterface:  "libapduinterface_stdio",
		HTTPInterface:  "libhttpinterface_stdio",
		DebugHTTP:      true,
		DebugAPDU:      true,
		Stdio:          new(Stdio),
	}
	fp, err := os.Open(filepath.Join("fixtures", name+".json"))
	if err != nil {
		panic(err)
	}
	if err = json.NewDecoder(fp).Decode(&controller.Stdio); err != nil {
		panic(err)
	}
	return controller
}
