package binance

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type Client struct {
	base, key, secret string
	http              *http.Client
	offset            int64
}

func New(baseURL, key, secret string) *Client {
	return &Client{base: baseURL, key: key, secret: secret,
		http: &http.Client{Timeout: 15 * time.Second}}
}

func (c *Client) BaseURL() string { return c.base }

type APIError struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

func (e *APIError) Error() string { return fmt.Sprintf("binance error %d: %s", e.Code, e.Msg) }

func (c *Client) do(method, path string, p url.Values, signed bool) ([]byte, error) {
	if p == nil {
		p = url.Values{}
	}
	if signed {
		p.Set("timestamp", strconv.FormatInt(time.Now().UnixMilli()+c.offset, 10))
		p.Set("recvWindow", "10000")
		mac := hmac.New(sha256.New, []byte(c.secret))
		mac.Write([]byte(p.Encode()))
		p.Set("signature", hex.EncodeToString(mac.Sum(nil)))
	}
	req, err := http.NewRequest(method, c.base+path+"?"+p.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-MBX-APIKEY", c.key)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		var ae APIError
		if json.Unmarshal(body, &ae) == nil && ae.Code != 0 {
			return nil, &ae
		}
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

func (c *Client) SyncTime() error {
	b, err := c.do("GET", "/fapi/v1/time", nil, false)
	if err != nil {
		return err
	}
	var t struct {
		ServerTime int64 `json:"serverTime"`
	}
	if err := json.Unmarshal(b, &t); err != nil {
		return err
	}
	c.offset = t.ServerTime - time.Now().UnixMilli()
	return nil
}
