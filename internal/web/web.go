package web

import (
	"errors"
	"io"
	"net/http"
	"strings"
)

func Request(url string, postdata string) ([]byte, int, error) {
	var body []byte
	resp, err := http.Post(url, "application/json", strings.NewReader(postdata))
	if err != nil {
		return body, 0, errors.New("Could not send request" + err.Error())
	}

	defer resp.Body.Close()
	statusCode := resp.StatusCode
	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return body, statusCode, nil
	}

	return body, statusCode, nil
}

func WebRequest(method, url, postdata string) ([]byte, int, error) {

	req, err := http.NewRequest(method, url, strings.NewReader(postdata))
	if err != nil {
		return nil, 500, err
	}

	headers := map[string]string{}
	for key, value := range headers {
		req.Header.Add(key, value)
	}

	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	statusCode := resp.StatusCode

	// read body
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, statusCode, err
	}
	return body, statusCode, nil
}

// cleanServerURL removes http/https schema from server URL
func CleanServerURL(url string) string {
	url = strings.TrimPrefix(url, "http://")
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "ws://")
	url = strings.TrimPrefix(url, "wss://")
	return url
}
