package TransportMessage

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"
)

// HttpTransport 是一个简单的 HTTP 传输实现，endpoint 不应包含路径。
// SendMessage 会把 endpoint + "/RecieveMessage" 拼成目标 URL；SendAck 会拼 "/RecieveAck"。
type HttpTransport struct{}

func NewHttpTransport() *HttpTransport {
	return &HttpTransport{}
}

func normalize(endpoint, path string) string {
	ep := strings.TrimRight(endpoint, "/")
	p := path
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return ep + p
}

func (h *HttpTransport) SendMessage(ctx context.Context, endpoint string, data []byte) error {
	url := normalize(endpoint, "/RecieveMessage")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	// ignore body; return error on non-2xx
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}

func (h *HttpTransport) SendAck(ctx context.Context, endpoint string, data []byte) error {
	url := normalize(endpoint, "/RecieveAck")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}
