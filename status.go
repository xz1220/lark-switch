package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"
)

// authStatus mirrors the relevant fields of `lark-cli auth status` JSON output.
// On an unconfigured home lark-cli returns {"ok":false,"error":{...}} instead
// (printed to stderr with a non-zero exit code).
type authStatus struct {
	OK         *bool  `json:"ok"`
	AppID      string `json:"appId"`
	Brand      string `json:"brand"`
	Identity   string `json:"identity"`
	Identities struct {
		User struct {
			OpenID           string `json:"openId"`
			UserName         string `json:"userName"`
			TokenStatus      string `json:"tokenStatus"`
			ExpiresAt        string `json:"expiresAt"`
			RefreshExpiresAt string `json:"refreshExpiresAt"`
		} `json:"user"`
	} `json:"identities"`
	Error struct {
		Subtype string `json:"subtype"`
		Message string `json:"message"`
	} `json:"error"`
}

func (st *authStatus) configured() bool {
	return st.OK == nil || *st.OK
}

// runLark runs lark-cli for an account, capturing stdout and stderr separately.
// A non-zero exit (e.g. not_configured, expired token) is expected — callers
// parse the JSON body rather than relying on the exit code.
func runLark(larkCli string, a *Account, args ...string) (stdout, stderr []byte) {
	cmd := exec.Command(larkCli, args...)
	cmd.Env = envForAccount(a)
	var so, se bytes.Buffer
	cmd.Stdout = &so
	cmd.Stderr = &se
	_ = cmd.Run()
	return so.Bytes(), se.Bytes()
}

// decodeJSON finds and decodes the first DECODABLE JSON object across the given
// buffers (stdout is tried before stderr). lark-cli prints proxy warnings and
// other prose around the JSON, so we try every '{' — not just the first, which
// may fall inside non-JSON prose — and accept the first that decodes. A Decoder
// is used so trailing bytes after the object are ignored.
func decodeJSON(v any, buffers ...[]byte) bool {
	for _, b := range buffers {
		for i := 0; i < len(b); i++ {
			if b[i] != '{' {
				continue
			}
			if json.NewDecoder(bytes.NewReader(b[i:])).Decode(v) == nil {
				return true
			}
		}
	}
	return false
}

func queryStatus(larkCli string, a *Account) (*authStatus, error) {
	so, se := runLark(larkCli, a, "auth", "status")
	var st authStatus
	if !decodeJSON(&st, so, se) {
		return nil, fmt.Errorf("could not parse `auth status` output")
	}
	return &st, nil
}

// keepAlive makes one cheap authenticated call so lark-cli's auto-refresh keeps
// the rolling ~7-day refresh token alive. Success requires an explicit code:0.
func keepAlive(larkCli string, a *Account) error {
	so, se := runLark(larkCli, a, "api", "GET", "/open-apis/authen/v1/user_info")
	var r struct {
		Code *int   `json:"code"`
		Msg  string `json:"msg"`
		OK   *bool  `json:"ok"`
	}
	if !decodeJSON(&r, so, se) {
		return fmt.Errorf("no response (token may be expired — try `lark-switch login %s`)", a.Name)
	}
	if r.OK != nil && !*r.OK {
		return fmt.Errorf("not authorized (try `lark-switch login %s`)", a.Name)
	}
	if r.Code == nil {
		return fmt.Errorf("unexpected response (try `lark-switch login %s`)", a.Name)
	}
	if *r.Code != 0 {
		return fmt.Errorf("api code %d: %s", *r.Code, r.Msg)
	}
	return nil
}

// refreshRemaining returns how long until the refresh token expires.
func refreshRemaining(st *authStatus) (time.Duration, bool) {
	r := st.Identities.User.RefreshExpiresAt
	if r == "" {
		return 0, false
	}
	t, err := time.Parse(time.RFC3339, r)
	if err != nil {
		return 0, false
	}
	return time.Until(t), true
}

func humanDur(d time.Duration) string {
	if d <= 0 {
		return "expired"
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	if days > 0 {
		return fmt.Sprintf("%dd%dh", days, hours)
	}
	mins := int(d.Minutes()) % 60
	return fmt.Sprintf("%dh%dm", hours, mins)
}
