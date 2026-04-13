package web

import (
	"errors"
	"io"
	"net/http"
	"strings"
)

func Request(url string, postdata string) ([]byte, int, error) {
	resp, err := http.Post(url, "application/json", strings.NewReader(postdata))
	if err != nil {
		return nil, 0, errors.New("Could not send request: " + err.Error())
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}

	return body, resp.StatusCode, nil
}

func WebRequest(method, url, postdata string, headers map[string]string) ([]byte, int, error) {
	req, err := http.NewRequest(method, url, strings.NewReader(postdata))
	if err != nil {
		return nil, 500, err
	}

	for key, value := range headers {
		req.Header.Add(key, value)
	}

	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}

	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return body, resp.StatusCode, nil
}

// CleanServerURL removes http/https/ws/wss schema from server URL
func CleanServerURL(url string) string {
	url = strings.TrimPrefix(url, "http://")
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "ws://")
	url = strings.TrimPrefix(url, "wss://")
	return url
}
