package httpclient

import (
	"crypto/tls"
	"github.com/neckchi/schedulehub/internal/database"
	"net/http"
	"net/url"
	"time"
)

//grpc design pattern(func opton pattern) for config mgt

type HttpFuncOption func(*HttpClientWrapper)

type HttpClientWrapper struct {
	client            *http.Client
	redisDb           database.RedisRepository
	contextTimeout    time.Duration
	maxRetries        int
	initialRetryDelay time.Duration
}

func defaultHttpConfig(rdb database.RedisRepository) HttpClientWrapper {
	//proxyUrl, _ := url.Parse("http://zscaler.proxy.int.kn:80")
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.MaxIdleConns = 100
	t.MaxConnsPerHost = 100
	t.MaxIdleConnsPerHost = 100
	t.IdleConnTimeout = 90 * time.Second
	t.DisableKeepAlives = false
	t.TLSClientConfig.InsecureSkipVerify = true
	t.TLSClientConfig.CipherSuites = []uint16{
		// TLS 1.0 - 1.2 cipher suites.
		tls.TLS_RSA_WITH_RC4_128_SHA,
		tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA,
		tls.TLS_RSA_WITH_AES_128_CBC_SHA,
		tls.TLS_RSA_WITH_AES_256_CBC_SHA,
		tls.TLS_RSA_WITH_AES_128_CBC_SHA256,
		tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_ECDSA_WITH_RC4_128_SHA,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
		tls.TLS_ECDHE_RSA_WITH_RC4_128_SHA,
		tls.TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA,
		tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
		tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256,
		tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256,
		tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
		tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
	}

	return HttpClientWrapper{
		client:            &http.Client{Transport: t},
		redisDb:           rdb,
		contextTimeout:    7 * time.Second,
		maxRetries:        2,
		initialRetryDelay: 2 * time.Second,
	}
}

func WithCtxTimeout(ctxTimeout time.Duration) HttpFuncOption {
	return func(httpConfig *HttpClientWrapper) {
		httpConfig.contextTimeout = ctxTimeout
	}
}

func WithMaxRetries(maxRetries int) HttpFuncOption {
	return func(httpConfig *HttpClientWrapper) {
		httpConfig.maxRetries = maxRetries
	}
}

func WithRetryDelay(delay time.Duration) HttpFuncOption {
	return func(httpConfig *HttpClientWrapper) {
		httpConfig.initialRetryDelay = delay
	}
}

func WithMaxIdleConns(max int) HttpFuncOption {
	return func(httpConfig *HttpClientWrapper) {
		if httpClient, ok := interface{}(httpConfig.client).(*http.Client); ok {
			if transport, ok := httpClient.Transport.(*http.Transport); ok {
				transport.MaxIdleConns = max
			}
		}
	}
}

func WithMaxConnsPerHost(max int) HttpFuncOption {
	return func(httpConfig *HttpClientWrapper) {
		if httpClient, ok := interface{}(httpConfig.client).(*http.Client); ok {
			if transport, ok := httpClient.Transport.(*http.Transport); ok {
				transport.MaxConnsPerHost = max
			}
		}
	}
}

func WithMaxIdleConnsPerHost(max int) HttpFuncOption {
	return func(httpConfig *HttpClientWrapper) {
		if httpClient, ok := interface{}(httpConfig.client).(*http.Client); ok {
			if transport, ok := httpClient.Transport.(*http.Transport); ok {
				transport.MaxIdleConnsPerHost = max
			}
		}
	}
}

func WithIdleConnTimeout(timeout time.Duration) HttpFuncOption {
	return func(httpConfig *HttpClientWrapper) {
		if httpClient, ok := interface{}(httpConfig.client).(*http.Client); ok {
			if transport, ok := httpClient.Transport.(*http.Transport); ok {
				transport.IdleConnTimeout = timeout * time.Second
			}
		}
	}
}

func WithDisableKeepAlives(disable bool) HttpFuncOption {
	return func(httpConfig *HttpClientWrapper) {
		if httpClient, ok := interface{}(httpConfig.client).(*http.Client); ok {
			if transport, ok := httpClient.Transport.(*http.Transport); ok {
				transport.DisableKeepAlives = disable
			}
		}
	}
}

func WithProxySetup(proxyAddress *url.URL) HttpFuncOption {
	return func(httpConfig *HttpClientWrapper) {
		if httpClient, ok := interface{}(httpConfig.client).(*http.Client); ok {
			if transport, ok := httpClient.Transport.(*http.Transport); ok {
				transport.Proxy = http.ProxyURL(proxyAddress)
			}
		}
	}
}

type HttpClient struct {
	HttpClientWrapper
}

// Constructor to create an instance of the HttpClientWrapper with connection pool setup
func CreateHttpClientInstance(rdb database.RedisRepository, httpConfig ...HttpFuncOption) *HttpClient {
	d := defaultHttpConfig(rdb)
	for _, fn := range httpConfig {
		fn(&d)
	}
	return &HttpClient{d}
}
