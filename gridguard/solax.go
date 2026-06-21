package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Solax register numbers (1-indexed into the 300-int paramInit array).
const (
	regExportControl = 48
	regWorkMode      = 28
	workModeSelfUse  = 0
	workModeFeedIn   = 1
)

// dataField builds the exact single-quoted payload the Solax API expects.
func dataField(reg, val int) string {
	return fmt.Sprintf("[{'reg': '%d', 'val': '%d'}]", reg, val)
}

func encodeExportControl(watts int) int { return watts / 10 }
func decodeExportControl(raw int) int   { return raw * 10 }

// toForm renders any string-keyed map as url-encoded form values.
func toForm[V any](m map[string]V) url.Values {
	v := url.Values{}
	for k, val := range m {
		v.Set(k, fmt.Sprintf("%v", val))
	}
	return v
}

// SolaxClient talks to the Solax private cloud control API.
type SolaxClient struct {
	secrets SolaxSecrets
	http    *http.Client
	tokenID string
}

func NewSolaxClient(s SolaxSecrets) *SolaxClient {
	return &SolaxClient{secrets: s, http: &http.Client{Timeout: 30 * time.Second}}
}

const (
	urlLogin       = "https://www.solaxcloud.com/phoebus/login/loginNew"
	urlDeviceLogin = "https://abroad.solaxcloud.com/proxy//login/remoteLogin.do"
	urlParamInit   = "https://abroad.solaxcloud.com/proxy//settingnew/paramInit"
	urlParamSet    = "https://abroad.solaxcloud.com/proxy//settingnew/paramSet"
	// `global` (not `www`) is region-agnostic: www geo-routes by requester IP, so an
	// EU-account monitoring token returns "no auth!" when called from a non-EU host
	// (e.g. a US cloud VM). global works from anywhere.
	urlRealtime = "https://global.solaxcloud.com:9443/proxy/api/getRealtimeInfo.do"
)

// checkEnvelope asserts HTTP 200 + JSON success==true and (optionally) decodes the body into out.
func checkEnvelope(endpoint string, status int, body []byte, out any) error {
	if status != http.StatusOK {
		return fmt.Errorf("%s: HTTP %d: %s", endpoint, status, string(body))
	}
	var probe struct {
		Success bool `json:"success"`
	}
	if err := json.Unmarshal(body, &probe); err != nil {
		return fmt.Errorf("%s: non-JSON response (HTTP %d): %s", endpoint, status, string(body))
	}
	if !probe.Success {
		return fmt.Errorf("%s: success=false: %s", endpoint, string(body))
	}
	if out != nil {
		if err := json.Unmarshal(body, out); err != nil {
			return fmt.Errorf("%s: decode: %w", endpoint, err)
		}
	}
	return nil
}

// postForm posts url-encoded form data with optional extra headers and decodes
// the JSON body into out, asserting HTTP 200 + success==true.
func (c *SolaxClient) postForm(endpoint string, form url.Values, headers map[string]string, out any) ([]byte, error) {
	req, err := http.NewRequest("POST", endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return body, fmt.Errorf("%s: read body: %w", endpoint, err)
	}
	return body, checkEnvelope(endpoint, resp.StatusCode, body, out)
}

// Login performs loginNew then remoteLogin.do, storing the device tokenId.
func (c *SolaxClient) Login() error {
	var login struct {
		Token string `json:"token"`
	}
	if _, err := c.postForm(urlLogin, toForm(c.secrets.LoginData.LoginPayload), nil, &login); err != nil {
		return fmt.Errorf("loginNew: %w", err)
	}
	if login.Token == "" {
		return fmt.Errorf("loginNew: no token in response")
	}
	var dev struct {
		Result struct {
			TokenID string `json:"tokenId"`
		} `json:"result"`
	}
	headers := map[string]string{"token": login.Token}
	if _, err := c.postForm(urlDeviceLogin, toForm(c.secrets.LoginData.DeviceLoginPayload), headers, &dev); err != nil {
		return fmt.Errorf("remoteLogin: %w", err)
	}
	if dev.Result.TokenID == "" {
		return fmt.Errorf("remoteLogin: no tokenId")
	}
	c.tokenID = dev.Result.TokenID
	return nil
}

func (c *SolaxClient) baseSetForm() url.Values {
	form := toForm(c.secrets.ParamsBasePayload)
	form.Set("tokenId", c.tokenID)
	return form
}

// ReadParams returns the 300-int parameter array from paramInit.
func (c *SolaxClient) ReadParams() ([]int, error) {
	if c.tokenID == "" {
		return nil, fmt.Errorf("not logged in")
	}
	var out struct {
		Result []int `json:"result"`
	}
	if _, err := c.postForm(urlParamInit, c.baseSetForm(), nil, &out); err != nil {
		return nil, err
	}
	if len(out.Result) < 300 {
		return nil, fmt.Errorf("paramInit returned %d values, expected >= 300 (API may have changed)", len(out.Result))
	}
	return out.Result, nil
}

// ExportControlW reads the current export cap in watts.
func (c *SolaxClient) ExportControlW() (int, error) {
	params, err := c.ReadParams()
	if err != nil {
		return 0, err
	}
	return decodeExportControl(params[regExportControl-1]), nil
}

func (c *SolaxClient) setParam(reg, encodedVal int) error {
	if c.tokenID == "" {
		return fmt.Errorf("not logged in")
	}
	form := c.baseSetForm()
	form.Set("Data", dataField(reg, encodedVal))
	_, err := c.postForm(urlParamSet, form, nil, nil)
	return err
}

// SetExportControlW sets the export cap in watts (encoded as W/10).
func (c *SolaxClient) SetExportControlW(watts int) error {
	return c.setParam(regExportControl, encodeExportControl(watts))
}

// SetWorkMode sets the inverter work mode (0 self-use, 1 feed-in priority).
func (c *SolaxClient) SetWorkMode(mode int) error {
	return c.setParam(regWorkMode, mode)
}

// RealtimeInfo is a subset of getRealtimeInfo we show in messages.
type RealtimeInfo struct {
	Soc         float64 `json:"soc"`
	FeedinPower float64 `json:"feedinpower"`
	AcPower     float64 `json:"acpower"`
	BatPower    float64 `json:"batPower"`
}

// Realtime reads current inverter telemetry via the monitoring token.
func (c *SolaxClient) Realtime() (RealtimeInfo, error) {
	sn := fmt.Sprintf("%v", c.secrets.ParamsBasePayload["sn"])
	u := fmt.Sprintf("%s?tokenId=%s&sn=%s", urlRealtime, url.QueryEscape(c.secrets.MonitoringToken), url.QueryEscape(sn))
	resp, err := c.http.Get(u)
	if err != nil {
		return RealtimeInfo{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return RealtimeInfo{}, fmt.Errorf("realtime read body: %w", err)
	}
	var out struct {
		Result RealtimeInfo `json:"result"`
	}
	if err := checkEnvelope("getRealtimeInfo", resp.StatusCode, body, &out); err != nil {
		return RealtimeInfo{}, err
	}
	return out.Result, nil
}
