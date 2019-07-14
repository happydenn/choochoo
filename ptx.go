package choochoo

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"
)

var ptxClient *PTXClient

func init() {
	var err error

	appID := os.Getenv("PTX_APP_ID")
	appKey := os.Getenv("PTX_APP_KEY")

	ptxClient, err = NewPTXClient(appID, appKey)
	if err != nil {
		log.Fatalf("Error initiating PTX client: %s", err)
	}
}

type PTXClientOptions struct {
	appID   string
	appKey  string
	baseURL string
}

type PTXClientOption func(*PTXClientOptions)

type PTXClientResponse []byte

func (r *PTXClientResponse) Data() []byte {
	return []byte(*r)
}

func (r *PTXClientResponse) Unmarshal() interface{} {
	var res interface{}
	if err := json.Unmarshal(r.Data(), &res); err != nil {
		return nil
	}
	return res
}

func NewPTXClient(appID, appKey string, opts ...PTXClientOption) (*PTXClient, error) {
	if appID == "" || appKey == "" {
		return nil, fmt.Errorf("appID or appKey not specified")
	}

	options := &PTXClientOptions{
		appID:   appID,
		appKey:  appKey,
		baseURL: "https://ptx.transportdata.tw",
	}

	for _, setter := range opts {
		setter(options)
	}

	c := &http.Client{Timeout: 10 * time.Second}

	return &PTXClient{c, options}, nil
}

type PTXClient struct {
	httpc *http.Client
	opts  *PTXClientOptions
}

func (c *PTXClient) NewRequest(method, path string, query *url.Values) (*http.Request, error) {
	u, _ := url.Parse(c.opts.baseURL)
	u = u.ResolveReference(&url.URL{Path: path})
	if query != nil {
		u.RawQuery = query.Encode()
	}

	req, err := http.NewRequest(method, u.String(), nil)
	if err != nil {
		return nil, err
	}

	c.addRequestSignature(req)

	return req, nil
}

func (c *PTXClient) Do(req *http.Request) (*PTXClientResponse, error) {
	res, err := c.httpc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot do ptx query: %s", err)
	}

	d, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("cannot read response: %s", err)
	}
	defer res.Body.Close()

	cr := PTXClientResponse(d)
	return &cr, nil
}

func (c *PTXClient) Get(path string, query *url.Values) (*PTXClientResponse, error) {
	req, err := c.NewRequest("GET", path, query)
	if err != nil {
		return nil, fmt.Errorf("cannot create request: %s", err)
	}
	return c.Do(req)
}

func (c *PTXClient) addRequestSignature(req *http.Request) {
	xdate := time.Now().UTC().Format("Mon, 02 Jan 2006 15:04:05 GMT")
	mac := hmac.New(sha1.New, []byte(c.opts.appKey))
	mac.Write([]byte(fmt.Sprintf("x-date: %s", xdate)))
	sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	req.Header.Set("x-date", xdate)
	req.Header.Set("Authorization", fmt.Sprintf(`hmac username="%s", algorithm="hmac-sha1", headers="x-date", signature="%s"`, c.opts.appID, sig))
}
