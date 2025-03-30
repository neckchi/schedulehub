package httpclient

import (
	"bytes"
	"context"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

func (hc *HttpClientWrapper) methodRegister(ctx context.Context, method string, urlString *string, params *map[string]string, headers *map[string]string) (*http.Request, error) {
	var request *http.Request
	var err error
	switch method {
	case http.MethodPost:
		// Handle POST request with form data
		formData := url.Values{}
		for k, v := range *params {
			formData.Set(k, v)
		}
		request, err = http.NewRequestWithContext(ctx, method, *urlString, strings.NewReader(formData.Encode()))
		if err != nil {
			return nil, fmt.Errorf("error creating POST request: %v", err)
		}
	case http.MethodGet:
		request, err = http.NewRequestWithContext(ctx, method, *urlString, nil)
		if err != nil {
			return nil, fmt.Errorf("error creating GET request: %v", err)
		}

		if params != nil {
			q := request.URL.Query()
			for k, v := range *params {
				q.Add(k, v)
			}
			request.URL.RawQuery = q.Encode()
		}
	default:
		return nil, fmt.Errorf("unsupported HTTP method: %s", method)
	}

	for k, v := range *headers {
		request.Header.Set(k, v)
	}

	return request, nil
}

// Function to safely write data to the byte buffer
func writeToBuffer(mu *sync.Mutex, buf *bytes.Buffer, data []byte, addComma bool) {
	mu.Lock()
	defer mu.Unlock()
	if addComma && buf.Len() > 1 { // Ensure comma is added only if needed
		buf.WriteByte(',')
	}
	buf.Write(data)
	//mu.Unlock()
}

// Function to fetch partial content especially for CMA. All or nothing approach.
func (hc *HttpClientWrapper) fetchPartialContent(context context.Context, method string, urlString *string, params *map[string]string, headers *map[string]string, resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()
	// Parse Content-Range header
	contentRange := resp.Header.Get("content-range")
	parts := strings.Split(contentRange, "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid content-range format: %s", contentRange)
	}
	lastPageStr, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid content-range: %s", contentRange)
	}
	// Shared buffer and mutex
	var combinedResponses bytes.Buffer
	var mu sync.Mutex
	var wg sync.WaitGroup
	combinedResponses.WriteByte('[')

	// Read and append the first body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	body = bytes.Trim(body, "[]")
	writeToBuffer(&mu, &combinedResponses, body, false)

	// Fetch subsequent parts
	for num := 50; num < lastPageStr; num += 50 {
		wg.Add(1)
		go func(num int) {
			defer wg.Done()
			// Update Range header for the next request
			localHeaders := make(map[string]string)
			for k, v := range *headers {
				localHeaders[k] = v
			}
			localHeaders["Range"] = fmt.Sprintf("%d-%d", num, num+49)
			startExt := time.Now()
			requestExt, err := hc.methodRegister(context, method, urlString, params, &localHeaders)
			if err != nil {
				log.Errorf("failed to create request for range %d-%d: %v", num, num+49, err)
				return
			}
			respExt, err := hc.client.Do(requestExt)
			if err != nil {
				log.Errorf("failed to execute request for range %d-%d: %v", num, num+49, err)
				return
			}
			log.Infof("Request: %s %s %s %.3fs", requestExt.Method, requestExt.URL.String(), respExt.Status, time.Since(startExt).Seconds())

			defer respExt.Body.Close()
			bodyExt, err := io.ReadAll(respExt.Body)
			if err != nil {
				log.Errorf("failed to read response body for range %d-%d: %v", num, num+49, err)
				return
			}
			bodyExt = bytes.Trim(bodyExt, "[]")
			writeToBuffer(&mu, &combinedResponses, bodyExt, true)
		}(num)
	}
	wg.Wait()
	combinedResponses.WriteByte(']')
	return combinedResponses.Bytes(), nil
}

func (hc *HttpClientWrapper) Fetch(ctx context.Context, method string, urlString *string, params *map[string]string, headers *map[string]string, namespace string, expiry time.Duration) ([]byte, error) {
	var attempt int
	var result []byte
	// TimeOut and Retry mechanism
	for attempt = 0; attempt <= hc.maxRetries; attempt++ {
		if ctx.Err() == context.Canceled {
			log.Warnf("Fetch stopped: parent context canceled before attempt %d for %s", attempt, *urlString)
			return nil, fmt.Errorf("fetch aborted: parent context was canceled")
		}
		// Create a new context with timeout for each request
		childCtx, cancel := context.WithTimeout(ctx, hc.contextTimeout)
		defer cancel()
		// Record the start time
		start := time.Now()
		//Create Request
		request, err := hc.methodRegister(childCtx, method, urlString, params, headers)
		if err != nil {
			lastErr := fmt.Errorf("attempt %d: error creating request: %v", attempt, err)
			log.Error(lastErr)
			break
		}
		// Check Redis cache before making HTTP request at first time
		if attempt == 0 {
			cacheResult, exist := hc.redisDb.Get(namespace, request.URL.String())
			if exist {
				return cacheResult, nil
			}
		}
		// Perform HTTP request
		resp, err := hc.client.Do(request)
		if err != nil {
			// Detect if the parent context was canceled
			if ctx.Err() == context.Canceled {
				log.Warnf("Fetch stopped: parent context canceled after attempt %d for %s", attempt, request.URL.String())
				return nil, fmt.Errorf("fetch aborted: parent context was canceled")
			}
			//If the request was canceled due to timeout, retry
			if childCtx.Err() == context.DeadlineExceeded || childCtx.Err() == context.Canceled {
				log.Warningf("Attempt %d: %s -  %s %.3fs", attempt, childCtx.Err(), request.URL, time.Since(start).Seconds())
			} else {
				lastErr := fmt.Errorf("attempt %d: error performing HTTP request: %v", attempt, err)
				log.Error(lastErr)
			}
		} else {
			log.Infof("Request: %s %s %s %.3fs", request.Method, request.URL.String(), resp.Status, time.Since(start).Seconds())
			defer resp.Body.Close()

			switch resp.StatusCode {
			case http.StatusOK:
				result, err = io.ReadAll(resp.Body)
				if err == nil {
					hc.redisDb.AddToChannel(namespace, request.URL.String(), result, expiry)
					return result, nil
				}

			case http.StatusPartialContent:
				result, err = hc.fetchPartialContent(childCtx, method, urlString, params, headers, resp)
				if err == nil {
					hc.redisDb.AddToChannel(namespace, request.URL.String(), result, expiry)
					return result, nil
				}

			default:
				Err := fmt.Errorf("Failed to process the request for %s due to http status %d", request.URL, resp.StatusCode)
				return nil, Err

			}

		}
		// Before retrying check if parent context is canceled
		if ctx.Err() == context.Canceled {
			log.Warnf("Fetch stopped: parent context canceled before retry %d for %s", attempt, request.URL.String())
			return nil, fmt.Errorf("fetch aborted: parent context was canceled")
		}
		// Retry logic only when the parent `ctx` is still active
		if attempt < hc.maxRetries {
			backoffDelay := time.Duration(attempt+1) * hc.initialRetryDelay
			log.Infof("Retrying in %s (attempt %d/%d) for %s", backoffDelay, attempt+1, hc.maxRetries, request.URL.String())
			time.Sleep(backoffDelay)
		}
	}
	log.Errorf("Fetch failed after %d attempts", hc.maxRetries)
	return nil, fmt.Errorf("fetch failed after %d attempts", hc.maxRetries)
}
