package plugin

import (
	"fmt"
	"io"
	"net/http"
	"time"

	utilhttp "nekocode/util/http"
	"nekocode/util/runtime"
)

var fetchClient = &http.Client{
	Transport: utilhttp.SharedTransport,
	Timeout:   10 * time.Second,
}

func FetchURL(url string) ([]byte, error) {
	ctx, cancel := runtime.ShortContext()
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := fetchClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 64*1024))
}
