package icinga2

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

type QueryFilter struct {
	Filter string `json:"filter"`
}

type Client interface {
	GetHost(string) (Host, error)
	CreateHost(Host) error
	ListHosts(string) ([]Host, error)
	DeleteHost(string) error
	UpdateHost(Host) error

	GetHostGroup(string) (HostGroup, error)
	CreateHostGroup(HostGroup) error
	ListHostGroups(string) ([]HostGroup, error)
	DeleteHostGroup(string) error
	UpdateHostGroup(HostGroup) error

	ListDowntimes(QueryFilter) ([]Downtime, error)

	GetService(string) (Service, error)
	CreateService(Service) error
	ListServices(QueryFilter) ([]Service, error)
	DeleteService(string) error
	UpdateService(Service) error

	ProcessCheckResult(Service, Action) error
	GetClientConfig() ClientConfig
	TestIcingaApi() error
	SetIcingaUrl(string)
}

type WebClient struct {
	httpSession session
	mu          sync.RWMutex
	// URL is the Icinga API base URL. It can be changed concurrently via
	// SetIcingaUrl (e.g. by a caller failing over between Icinga
	// instances) while other goroutines are issuing requests through this
	// client, so it must not be read or written directly - use
	// GetClientConfig/SetIcingaUrl, or the unexported url() helper from
	// within this package, instead of touching the field itself.
	URL               string
	Username          string
	Password          string
	Debug             bool
	DisableKeepAlives bool
	Zone              string
	TLSConfig         *tls.Config
}

// url returns the client's current Icinga API base URL, safe for
// concurrent use alongside SetIcingaUrl.
func (s *WebClient) url() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.URL
}

type ClientConfig struct {
	URL               string
	Username          string
	Password          string
	Debug             bool
	DisableKeepAlives bool
	Zone              string
	TLSConfig         *tls.Config
}

func (s *WebClient) GetClientConfig() ClientConfig {
	return ClientConfig{
		URL:               s.url(),
		Username:          s.Username,
		Password:          s.Password,
		Debug:             s.Debug,
		DisableKeepAlives: s.DisableKeepAlives,
		Zone:              s.Zone,
		TLSConfig:         s.TLSConfig,
	}
}

func (s *MockClient) GetClientConfig() ClientConfig {
	return ClientConfig{}
}

type MockClient struct {
	Hostgroups map[string]HostGroup
	Hosts      map[string]Host
	Services   map[string]Service
	Actions    map[string][]Action
	mutex      sync.Mutex
	URL        string
	Templates  []string
}

type Vars map[string]interface{}

type Checkable interface {
	GetCheckCommand() string
	GetVars() Vars
	GetNotes() string
	GetNotesURL() string
}

type Object interface {
	GetVars() Vars
}

// New builds a WebClient from s, which carries the desired configuration
// (URL, credentials, TLS settings, ...). s is taken by pointer, both
// because WebClient contains a mutex guarding URL that must not be copied,
// and because the returned *WebClient is the same instance as s: it is
// configured in place and returned, rather than copied into a new value.
func New(s *WebClient) (*WebClient, error) {
	transport := &http.Transport{
		TLSClientConfig:   s.TLSConfig,
		DisableKeepAlives: s.DisableKeepAlives,
		ForceAttemptHTTP2: true,
		Proxy:             http.ProxyFromEnvironment,
	}
	client := &http.Client{Transport: transport}

	s.httpSession = session{
		Log:      s.Debug,
		Client:   client,
		Userinfo: url.UserPassword(s.Username, s.Password),
	}

	s.URL = strings.TrimRight(s.URL, "/")

	return s, nil
}

func NewMockClient() (c *MockClient) {
	c = new(MockClient)
	c.Hostgroups = make(map[string]HostGroup)
	c.Hosts = make(map[string]Host)
	c.Services = make(map[string]Service)
	c.Actions = make(map[string][]Action)
	c.mutex = sync.Mutex{}
	c.Templates = []string{}
	return
}

type Results struct {
	Results []struct {
		Code   float64  `json:"code"`
		Errors []string `json:"errors,omitempty"`
		Status string   `json:"status,omitempty"`
		Name   string   `json:"name,omitempty"`
		Type   string   `json:"type,omitempty"`
	} `json:"results"`
}

func (s *WebClient) CreateObject(path string, create interface{}) error {
	var results, errmsg Results

	resp, err := s.httpSession.Put(s.url()+"/v1/objects"+path, create, &results, &errmsg)

	return s.handleResults("create", path, resp, &results, &errmsg, err)
}

func (s *WebClient) UpdateObject(path string, create interface{}) error {
	var results, errmsg Results

	resp, err := s.httpSession.Post(s.url()+"/v1/objects"+path, create, &results, &errmsg)
	return s.handleResults("update", path, resp, &results, &errmsg, err)
}

func (s *WebClient) FilteredQuery(url string, filter QueryFilter, result, errmsg interface{}) (*Response, error) {
	header := http.Header{
		"Accept": []string{"application/json"},
	}
	req := request{
		Method:  http.MethodGet,
		Url:     url,
		Header:  &header,
		Payload: filter,
		Result:  result,
		Error:   errmsg,
	}
	return s.httpSession.send(&req)
}

func (s *WebClient) SetIcingaUrl(url string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.URL = url
}

func (s *MockClient) SetIcingaUrl(url string) {
	s.URL = url
}

func (s *WebClient) TestIcingaApi() error {
	var results, errmsg Results

	resp, err := s.httpSession.Get(s.url()+"/v1", nil, &results, &errmsg)
	if err != nil {
		return err
	}

	if resp.HttpResponse().StatusCode != http.StatusOK {
		return fmt.Errorf("did not get 200 OK, got %s", resp.HttpResponse().Status)
	}

	return nil
}

func (s *MockClient) TestIcingaApi() error {
	parsedUrl, err := url.Parse(s.URL)
	if err != nil {
		return err
	}

	if parsedUrl.Host == "" {
		return fmt.Errorf("URL without hostname not supported: %v", parsedUrl)
	}
	return nil
}

func (s *WebClient) handleResults(typ, path string, resp *Response, results, errmsg *Results, oerr error) error {
	var resultReport string

	if oerr != nil {
		return oerr
	}

	for _, r := range results.Results {
		if r.Code >= 400.0 {
			resultReport += r.Status + " " + strings.Join(r.Errors, " ") + " "
		}
	}

	for _, r := range errmsg.Results {
		if r.Code >= 400.0 {
			resultReport += r.Status + " " + strings.Join(r.Errors, " ") + " "
		}
	}

	if resp.HttpResponse().StatusCode >= 400 {
		return fmt.Errorf("%s %s : %s - %s", typ, path, resp.HttpResponse().Status, resultReport)
	}

	if resultReport != "" {
		return fmt.Errorf("%s %s : %s", typ, path, resultReport)
	}

	return oerr

}
