package service

import (
	"container/list"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"golang.org/x/net/proxy"
)

var (
	httpClient       *http.Client
	proxyClientLock  sync.Mutex
	proxyClients     = make(map[string]*list.Element)
	proxyClientOrder = list.New()
)

const maxProxyClientCacheSize = 128

type proxyClientCacheEntry struct {
	proxyURL string
	client   *http.Client
}

func checkRedirect(req *http.Request, via []*http.Request) error {
	fetchSetting := system_setting.GetFetchSetting()
	urlStr := req.URL.String()
	if err := common.ValidateURLWithFetchSetting(urlStr, fetchSetting.EnableSSRFProtection, fetchSetting.AllowPrivateIp, fetchSetting.DomainFilterMode, fetchSetting.IpFilterMode, fetchSetting.DomainList, fetchSetting.IpList, fetchSetting.AllowedPorts, fetchSetting.ApplyIPFilterForDomain); err != nil {
		return fmt.Errorf("redirect to %s blocked: %v", urlStr, err)
	}
	if len(via) >= 10 {
		return fmt.Errorf("stopped after 10 redirects")
	}
	return nil
}

func InitHttpClient() {
	transport := &http.Transport{
		MaxIdleConns:        common.RelayMaxIdleConns,
		MaxIdleConnsPerHost: common.RelayMaxIdleConnsPerHost,
		ForceAttemptHTTP2:   true,
		Proxy:               http.ProxyFromEnvironment, // Support HTTP_PROXY, HTTPS_PROXY, NO_PROXY env vars
	}
	if common.TLSInsecureSkipVerify {
		transport.TLSClientConfig = common.InsecureTLSConfig
	}

	if common.RelayTimeout == 0 {
		httpClient = &http.Client{
			Transport:     transport,
			CheckRedirect: checkRedirect,
		}
	} else {
		httpClient = &http.Client{
			Transport:     transport,
			Timeout:       time.Duration(common.RelayTimeout) * time.Second,
			CheckRedirect: checkRedirect,
		}
	}
}

func GetHttpClient() *http.Client {
	return httpClient
}

// GetHttpClientWithProxy returns the default client or a proxy-enabled one when proxyURL is provided.
func GetHttpClientWithProxy(proxyURL string) (*http.Client, error) {
	if proxyURL == "" {
		return GetHttpClient(), nil
	}
	return NewProxyHttpClient(proxyURL)
}

func closeHttpClientIdleConnections(client *http.Client) {
	if client == nil {
		return
	}
	if transport, ok := client.Transport.(*http.Transport); ok && transport != nil {
		transport.CloseIdleConnections()
	}
}

func getCachedProxyClient(proxyURL string) (*http.Client, bool) {
	proxyClientLock.Lock()
	defer proxyClientLock.Unlock()

	element, ok := proxyClients[proxyURL]
	if !ok {
		return nil, false
	}
	proxyClientOrder.MoveToFront(element)
	entry := element.Value.(*proxyClientCacheEntry)
	return entry.client, true
}

func cacheProxyClient(proxyURL string, client *http.Client) *http.Client {
	proxyClientLock.Lock()
	defer proxyClientLock.Unlock()

	if element, ok := proxyClients[proxyURL]; ok {
		proxyClientOrder.MoveToFront(element)
		entry := element.Value.(*proxyClientCacheEntry)
		closeHttpClientIdleConnections(client)
		return entry.client
	}

	element := proxyClientOrder.PushFront(&proxyClientCacheEntry{
		proxyURL: proxyURL,
		client:   client,
	})
	proxyClients[proxyURL] = element

	if proxyClientOrder.Len() > maxProxyClientCacheSize {
		oldest := proxyClientOrder.Back()
		if oldest != nil {
			proxyClientOrder.Remove(oldest)
			entry := oldest.Value.(*proxyClientCacheEntry)
			delete(proxyClients, entry.proxyURL)
			closeHttpClientIdleConnections(entry.client)
		}
	}

	return client
}

// ResetProxyClientCache 清空代理客户端缓存，确保下次使用时重新初始化
func ResetProxyClientCache() {
	proxyClientLock.Lock()
	defer proxyClientLock.Unlock()
	for _, element := range proxyClients {
		entry := element.Value.(*proxyClientCacheEntry)
		closeHttpClientIdleConnections(entry.client)
	}
	proxyClients = make(map[string]*list.Element)
	proxyClientOrder.Init()
}

// NewProxyHttpClient 创建支持代理的 HTTP 客户端
func NewProxyHttpClient(proxyURL string) (*http.Client, error) {
	if proxyURL == "" {
		if client := GetHttpClient(); client != nil {
			return client, nil
		}
		return http.DefaultClient, nil
	}

	if client, ok := getCachedProxyClient(proxyURL); ok {
		return client, nil
	}

	parsedURL, err := url.Parse(proxyURL)
	if err != nil {
		return nil, err
	}

	switch parsedURL.Scheme {
	case "http", "https":
		transport := &http.Transport{
			MaxIdleConns:        common.RelayMaxIdleConns,
			MaxIdleConnsPerHost: common.RelayMaxIdleConnsPerHost,
			ForceAttemptHTTP2:   true,
			Proxy:               http.ProxyURL(parsedURL),
		}
		if common.TLSInsecureSkipVerify {
			transport.TLSClientConfig = common.InsecureTLSConfig
		}
		client := &http.Client{
			Transport:     transport,
			CheckRedirect: checkRedirect,
		}
		client.Timeout = time.Duration(common.RelayTimeout) * time.Second
		return cacheProxyClient(proxyURL, client), nil

	case "socks5", "socks5h":
		// 获取认证信息
		var auth *proxy.Auth
		if parsedURL.User != nil {
			auth = &proxy.Auth{
				User:     parsedURL.User.Username(),
				Password: "",
			}
			if password, ok := parsedURL.User.Password(); ok {
				auth.Password = password
			}
		}

		// 创建 SOCKS5 代理拨号器
		// proxy.SOCKS5 使用 tcp 参数，所有 TCP 连接包括 DNS 查询都将通过代理进行。行为与 socks5h 相同
		dialer, err := proxy.SOCKS5("tcp", parsedURL.Host, auth, proxy.Direct)
		if err != nil {
			return nil, err
		}

		transport := &http.Transport{
			MaxIdleConns:        common.RelayMaxIdleConns,
			MaxIdleConnsPerHost: common.RelayMaxIdleConnsPerHost,
			ForceAttemptHTTP2:   true,
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer.Dial(network, addr)
			},
		}
		if common.TLSInsecureSkipVerify {
			transport.TLSClientConfig = common.InsecureTLSConfig
		}

		client := &http.Client{Transport: transport, CheckRedirect: checkRedirect}
		client.Timeout = time.Duration(common.RelayTimeout) * time.Second
		return cacheProxyClient(proxyURL, client), nil

	default:
		return nil, fmt.Errorf("unsupported proxy scheme: %s, must be http, https, socks5 or socks5h", parsedURL.Scheme)
	}
}
