package core

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"strings"

	fhttp "github.com/bogdanfinn/fhttp"
	"github.com/fxamacker/cbor/v2"

	httputil "reg_go/internal/http"
)

// Step15KiroExchange Kiro ExchangeToken
func (r *Registrar) Step15KiroExchange(code string) (map[string]interface{}, error) {
	log.Println("[15] Kiro ExchangeToken")
	kvid := httputil.KiroVisitorID()

	cborBody, _ := cbor.Marshal(map[string]string{
		"idp":          "BuilderId",
		"code":         code,
		"codeVerifier": r.KiroCodeVerifier,
		"redirectUri":  r.Cfg.KiroRedirectURI,
		"state":        r.KiroState,
	})

	h := map[string]string{
		"Accept":                "application/cbor",
		"Content-Type":          "application/cbor",
		"User-Agent":            r.Identity.UA,
		"Origin":                r.Cfg.KiroBase,
		"Referer":               fmt.Sprintf("%s/signin/oauth?code=%s&state=%s", r.Cfg.KiroRedirectURI, code, r.KiroState),
		"smithy-protocol":       "rpc-v2-cbor",
		"x-amz-user-agent":     fmt.Sprintf("aws-sdk-js/1.0.0 ua/2.1 os/Windows#NT-10.0 lang/js md/browser#Google-Chrome_%s m/M,E", r.Identity.ChromeVer),
		"x-kiro-visitorid":     kvid,
		"amz-sdk-invocation-id": NewUUID(),
		"amz-sdk-request":      "attempt=1; max=1",
		"priority":             "u=1, i",
	}
	var cookieParts []string
	if v, ok := r.Cookies["awsccc"]; ok {
		cookieParts = append(cookieParts, "awsccc="+v)
	}
	cookieParts = append(cookieParts, "kiro-visitor-id="+kvid)
	h["Cookie"] = strings.Join(cookieParts, "; ")

	exchangeURL := r.Cfg.KiroBase + "/service/KiroWebPortalService/operation/ExchangeToken"
	client := httputil.NewTLSClient(r.Cfg.Proxy, true, r.Identity.ChromeVer)
	req, _ := fhttp.NewRequest("POST", exchangeURL, bytes.NewReader(cborBody))
	httputil.SetHeaders(req, h)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	respBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("ExchangeToken 失败 %d: %s", resp.StatusCode, string(respBody[:min(200, len(respBody))]))
	}

	for _, c := range resp.Cookies() {
		r.Cookies[c.Name] = c.Value
	}

	var rd map[string]interface{}
	cbor.Unmarshal(respBody, &rd)

	at, _ := rd["accessToken"].(string)
	csrf, _ := rd["csrfToken"].(string)
	st := r.Cookies["SessionToken"]
	rt := r.Cookies["RefreshToken"]

	if len(at) > 40 {
		log.Printf("accessToken=%s...", at[:40])
	}
	if rt != "" && len(rt) > 40 {
		log.Printf("refreshToken=%s...", rt[:40])
	} else {
		log.Println("refreshToken=N/A")
	}

	return map[string]interface{}{
		"accessToken":  at,
		"refreshToken": rt,
		"csrfToken":    csrf,
		"sessionToken": st,
		"expiresIn":    rd["expiresIn"],
	}, nil
}
