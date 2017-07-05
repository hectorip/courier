package utils

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"strings"
	"sync"
	"time"
)

// RequestResponseStatus represents the status of a WebhookRequeset
type RequestResponseStatus string

// RequestResponse represents both the outgoing request and response for a particular URL/method/body
type RequestResponse struct {
	URL        string
	Status     RequestResponseStatus
	StatusCode int
	Request    string
	Response   string
	Body       []byte
	Elapsed    time.Duration
}

const (
	// RRStatusSuccess represents that the webhook was successful
	RRStatusSuccess RequestResponseStatus = "S"

	// RRConnectionFailure represents that the webhook had a connection failure
	RRConnectionFailure RequestResponseStatus = "F"

	// RRStatusFailure represents that the webhook had a non 2xx status code
	RRStatusFailure RequestResponseStatus = "E"
)

// MakeInsecureHTTPRequest fires the passed in http request against a transport that does not validate
// SSL certificates.
func MakeInsecureHTTPRequest(req *http.Request) (*RequestResponse, error) {
	start := time.Now()
	requestTrace, err := httputil.DumpRequestOut(req, true)
	if err != nil {
		rr, _ := newRRFromRequestAndError(req, string(requestTrace), err)
		return rr, err
	}

	resp, err := GetInsecureHTTPClient().Do(req)
	if err != nil {
		rr, _ := newRRFromRequestAndError(req, string(requestTrace), err)
		return rr, err
	}
	defer resp.Body.Close()

	rr, err := newRRFromResponse(string(requestTrace), resp)
	rr.Elapsed = time.Now().Sub(start)
	return rr, err
}

// MakeHTTPRequest fires the passed in http request, returning any errors encountered. RequestResponse is always set
// regardless of any errors being set
func MakeHTTPRequest(req *http.Request) (*RequestResponse, error) {
	start := time.Now()
	requestTrace, err := httputil.DumpRequestOut(req, true)
	if err != nil {
		rr, _ := newRRFromRequestAndError(req, string(requestTrace), err)
		return rr, err
	}

	resp, err := GetHTTPClient().Do(req)
	if err != nil {
		rr, _ := newRRFromRequestAndError(req, string(requestTrace), err)
		return rr, err
	}
	defer resp.Body.Close()

	rr, err := newRRFromResponse(string(requestTrace), resp)
	rr.Elapsed = time.Now().Sub(start)
	return rr, err
}

// newRRFromResponse creates a new RequestResponse based on the passed in http request and error (when we received no response)
func newRRFromRequestAndError(r *http.Request, requestTrace string, requestError error) (*RequestResponse, error) {
	rr := RequestResponse{}
	rr.URL = r.URL.String()

	rr.Request = requestTrace
	rr.Status = RRConnectionFailure
	rr.Body = []byte(requestError.Error())

	return &rr, nil
}

// newRRFromResponse creates a new RequestResponse based on the passed in http Response
func newRRFromResponse(requestTrace string, r *http.Response) (*RequestResponse, error) {
	var err error
	rr := RequestResponse{}
	rr.URL = r.Request.URL.String()
	rr.StatusCode = r.StatusCode

	// set our status based on our status code
	if rr.StatusCode/100 == 2 {
		rr.Status = RRStatusSuccess
	} else {
		rr.Status = RRStatusFailure
	}

	rr.Request = requestTrace

	// figure out if our Response is something that looks like text from our headers
	isText := false
	contentType := r.Header.Get("Content-Type")
	if strings.Contains(contentType, "text") ||
		strings.Contains(contentType, "json") ||
		strings.Contains(contentType, "utf") ||
		strings.Contains(contentType, "javascript") ||
		strings.Contains(contentType, "xml") {

		isText = true
	}

	// only dump the whole body if this looks like text
	response, err := httputil.DumpResponse(r, isText)
	if err != nil {
		return &rr, err
	}
	rr.Response = string(response)

	if isText {
		bodyBytes, err := ioutil.ReadAll(r.Body)
		if err != nil {
			return &rr, err
		}
		rr.Body = bodyBytes
	}

	// return an error if we got a non-200 status
	if err == nil && rr.Status != RRStatusSuccess {
		err = fmt.Errorf("received non 200 status: %d", rr.StatusCode)
	}

	return &rr, err
}

var (
	transport *http.Transport
	client    *http.Client
	once      sync.Once
)

// GetHTTPClient returns the shared HTTP client used by all Courier threads
func GetHTTPClient() *http.Client {
	once.Do(func() {
		timeout := time.Duration(30 * time.Second)
		transport = &http.Transport{
			MaxIdleConns:    10,
			IdleConnTimeout: 30 * time.Second,
		}
		client = &http.Client{Transport: transport, Timeout: timeout}
	})

	return client
}

// GetInsecureHTTPClient returns the shared HTTP client used by all Courier threads
func GetInsecureHTTPClient() *http.Client {
	once.Do(func() {
		timeout := time.Duration(30 * time.Second)
		transport = &http.Transport{
			MaxIdleConns:    10,
			IdleConnTimeout: 30 * time.Second,
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		client = &http.Client{Transport: transport, Timeout: timeout}
	})

	return client
}