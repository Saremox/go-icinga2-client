package icinga2

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
)

// session is a minimal REST client used internally by WebClient. It
// replicates the small subset of behavior this package relied on from the
// (GPL-3.0 licensed, unmaintained) gopkg.in/jmcvetta/napping.v3 package:
// JSON-encoding a request payload, JSON-decoding a successful or error
// response body, HTTP Basic Auth, and optional request/response debug
// logging.
type session struct {
	Client   *http.Client
	Log      bool
	Userinfo *url.Userinfo
}

// request describes a single HTTP call.
type request struct {
	Method  string
	Url     string
	Params  *url.Values
	Header  *http.Header
	Payload interface{}
	Result  interface{}
	Error   interface{}
}

// Response wraps an executed HTTP request/response pair.
type Response struct {
	httpResponse *http.Response
	body         []byte
}

// HttpResponse returns the underlying net/http Response.
func (r *Response) HttpResponse() *http.Response {
	return r.httpResponse
}

func (s *session) Get(url string, p *url.Values, result, errMsg interface{}) (*Response, error) {
	return s.send(&request{Method: http.MethodGet, Url: url, Params: p, Result: result, Error: errMsg})
}

func (s *session) Post(url string, payload, result, errMsg interface{}) (*Response, error) {
	return s.send(&request{Method: http.MethodPost, Url: url, Payload: payload, Result: result, Error: errMsg})
}

func (s *session) Put(url string, payload, result, errMsg interface{}) (*Response, error) {
	return s.send(&request{Method: http.MethodPut, Url: url, Payload: payload, Result: result, Error: errMsg})
}

func (s *session) Delete(url string, p *url.Values, result, errMsg interface{}) (*Response, error) {
	return s.send(&request{Method: http.MethodDelete, Url: url, Params: p, Result: result, Error: errMsg})
}

func (s *session) send(r *request) (*Response, error) {
	u, err := url.Parse(r.Url)
	if err != nil {
		s.log("URL", r.Url, err)
		return nil, err
	}

	params := url.Values{}
	for k, v := range u.Query() {
		params[k] = v
	}
	if r.Params != nil {
		for k, v := range *r.Params {
			params[k] = v
		}
	}
	u.RawQuery = params.Encode()

	header := http.Header{}
	if r.Header != nil {
		for k := range *r.Header {
			header.Set(k, r.Header.Get(k))
		}
	}

	var body io.Reader
	if r.Payload != nil {
		b, err := json.Marshal(r.Payload)
		if err != nil {
			s.log(err)
			return nil, err
		}
		body = bytes.NewBuffer(b)
		header.Set("Content-Type", "application/json")
	}

	httpReq, err := http.NewRequest(r.Method, u.String(), body)
	if err != nil {
		s.log(err)
		return nil, err
	}
	httpReq.Header = header
	if httpReq.Header.Get("Accept") == "" {
		httpReq.Header.Set("Accept", "application/json")
	}

	if s.Userinfo != nil {
		pwd, _ := s.Userinfo.Password()
		httpReq.SetBasicAuth(s.Userinfo.Username(), pwd)
	}

	s.logRequest(httpReq, r.Payload)

	client := s.Client
	if client == nil {
		client = &http.Client{}
	}

	httpResp, err := client.Do(httpReq)
	if err != nil {
		s.log(err)
		return nil, err
	}
	defer func() { _ = httpResp.Body.Close() }()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		s.log(err)
		return nil, err
	}

	resp := &Response{httpResponse: httpResp, body: respBody}

	if len(respBody) > 0 {
		if httpResp.StatusCode < 300 && r.Result != nil {
			err = json.Unmarshal(respBody, r.Result)
		}
		if httpResp.StatusCode >= 400 && r.Error != nil {
			_ = json.Unmarshal(respBody, r.Error)
		}
	}

	s.logResponse(httpResp, respBody)

	return resp, err
}

func (s *session) log(args ...interface{}) {
	if s.Log {
		log.Println(args...)
	}
}

func (s *session) logRequest(req *http.Request, payload interface{}) {
	if !s.Log {
		return
	}
	log.Println("--------------------------------------------------------------------------------")
	log.Println("REQUEST")
	log.Println("--------------------------------------------------------------------------------")
	log.Println("Method:", req.Method)
	log.Println("URL:", req.URL)
	log.Println("Header:", req.Header)
	if payload != nil {
		if b, err := json.MarshalIndent(payload, "", "  "); err == nil {
			log.Println("Payload:", string(b))
		}
	}
}

func (s *session) logResponse(resp *http.Response, body []byte) {
	if !s.Log {
		return
	}
	log.Println("--------------------------------------------------------------------------------")
	log.Println("RESPONSE")
	log.Println("--------------------------------------------------------------------------------")
	log.Println("Status:", resp.Status)
	log.Println("Header:", resp.Header)
	log.Println("Body:", string(body))
}
