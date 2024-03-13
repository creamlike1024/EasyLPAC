package lpac

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

type StdioAPDURequest struct {
	Name  string    `json:"func"`
	Param StdioData `json:"param"`
}

type StdioAPDUResponse struct {
	ErrorCode int       `json:"ecode"`
	Data      StdioData `json:"data,omitempty"`
}

type StdioHTTPRequest struct {
	URL     string    `json:"url"`
	Body    StdioData `json:"tx"`
	Headers []string  `json:"headers"`
}

func (r *StdioHTTPRequest) HTTPRequest() (request *http.Request, err error) {
	request, err = http.NewRequest(http.MethodPost, r.URL, bytes.NewReader(r.Body))
	if err != nil {
		return
	}
	for _, element := range r.Headers {
		if name, value, ok := strings.Cut(element, ":"); ok {
			request.Header.Add(strings.TrimSpace(name), strings.TrimSpace(value))
		}
	}
	return
}

type StdioHTTPResponse struct {
	StatusCode int       `json:"rcode"`
	Body       StdioData `json:"rx"`
}

func (r *StdioHTTPResponse) FromHTTPResponse(response *http.Response) (err error) {
	r.StatusCode = response.StatusCode
	r.Body, err = io.ReadAll(response.Body)
	return
}

type StdioInterface interface {
	APDU(*StdioAPDURequest) *StdioAPDUResponse
	HTTP(*StdioHTTPRequest) *StdioHTTPResponse
}

type StdioData []byte

func (h *StdioData) MarshalJSON() (encoded []byte, _ error) {
	return json.Marshal(hex.EncodeToString(*h))
}

func (h *StdioData) UnmarshalJSON(data []byte) (err error) {
	var encoded string
	if err = json.Unmarshal(data, &encoded); err == nil {
		*h, err = hex.DecodeString(encoded)
	}
	return
}

func (h *StdioData) String() string {
	return hex.EncodeToString(*h)
}
