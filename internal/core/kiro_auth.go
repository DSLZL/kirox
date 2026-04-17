package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"

	fhttp "github.com/bogdanfinn/fhttp"
	"github.com/fxamacker/cbor/v2"

	httputil "reg_go/internal/http"
)

// Step14KiroAuthorize Kiro OIDC 授权
func (r *Registrar) Step14KiroAuthorize() (string, error) {
	log.Println("[14] Kiro OIDC 授权")

	verifier, challenge := httputil.PKCE()
	r.KiroCodeVerifier = verifier
	clientState := NewUUID()
	kvid := httputil.KiroVisitorID()

	cborBody, _ := cbor.Marshal(map[string]string{
		"idp":                 "BuilderId",
		"redirectUri":         r.Cfg.KiroRedirectURI,
		"codeChallenge":       challenge,
		"codeChallengeMethod": "S256",
		"state":               clientState,
	})

	h := map[string]string{
		"Accept":                "application/cbor",
		"Content-Type":          "application/cbor",
		"User-Agent":            r.Identity.UA,
		"Origin":                r.Cfg.KiroBase,
		"Referer":               r.Cfg.KiroBase + "/signin",
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

	initURL := r.Cfg.KiroBase + "/service/KiroWebPortalService/operation/InitiateLogin"
	client := httputil.NewTLSClient(r.Cfg.Proxy, true, r.Identity.ChromeVer)
	req, _ := fhttp.NewRequest("POST", initURL, bytes.NewReader(cborBody))
	httputil.SetHeaders(req, h)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	respBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("InitiateLogin 失败 %d: %s", resp.StatusCode, string(respBody[:min(200, len(respBody))]))
	}

	var rd map[string]interface{}
	cbor.Unmarshal(respBody, &rd)
	authURL, _ := rd["redirectUrl"].(string)
	if authURL == "" {
		return "", fmt.Errorf("InitiateLogin 无 redirectUrl")
	}
	if st := httputil.ExtractParam(authURL, "state"); st != "" {
		r.KiroState = st
	}

	// /authorize -> 302
	noRedirect := httputil.NewNoRedirectTLSClient(r.Cfg.Proxy, r.Identity.ChromeVer)
	req2, _ := fhttp.NewRequest("GET", authURL, nil)
	httputil.SetHeaders(req2, map[string]string{
		"Accept":                   "text/html,*/*;q=0.8",
		"User-Agent":               r.Identity.UA,
		"Referer":                  r.Cfg.KiroBase + "/",
		"sec-fetch-dest":           "document",
		"sec-fetch-mode":           "navigate",
		"sec-fetch-site":           "cross-site",
		"upgrade-insecure-requests": "1",
	})

	resp2, err := noRedirect.Do(req2)
	if err != nil {
		return "", err
	}
	resp2.Body.Close()
	if resp2.StatusCode != 302 {
		return "", fmt.Errorf("authorize 非 302: %d", resp2.StatusCode)
	}

	loc := resp2.Header.Get("Location")
	orchID := httputil.ExtractParam(loc, "orchestrator_id")
	if orchID == "" {
		return "", fmt.Errorf("无 orchestrator_id: %s", loc)
	}

	// /authentication_result
	h2 := map[string]string{
		"Accept":                  "application/json, text/plain, */*",
		"Content-Type":            "application/json",
		"User-Agent":              r.Identity.UA,
		"Origin":                  "https://view.awsapps.com",
		"Referer":                 "https://view.awsapps.com/",
		"x-amz-sso_bearer_token": r.SSOToken,
		"x-amz-sso-bearer-token": r.SSOToken,
		"sec-ch-ua":              r.Identity.SecUA,
		"sec-ch-ua-mobile":       "?0",
		"sec-ch-ua-platform":     `"Windows"`,
		"sec-fetch-dest":         "empty",
		"sec-fetch-mode":         "cors",
		"sec-fetch-site":         "cross-site",
	}
	authResultBody, _ := json.Marshal(map[string]string{"orchestrator_id": orchID})
	req3, _ := fhttp.NewRequest("POST", r.Cfg.OIDCBase+"/authentication_result",
		bytes.NewReader(authResultBody))
	httputil.SetHeaders(req3, h2)
	resp3, err := client.Do(req3)
	if err != nil {
		return "", err
	}
	body3, _ := io.ReadAll(resp3.Body)
	resp3.Body.Close()
	if resp3.StatusCode != 200 {
		return "", fmt.Errorf("authentication_result 失败: %d", resp3.StatusCode)
	}

	var rd3 map[string]interface{}
	json.Unmarshal(body3, &rd3)
	resumeURL, _ := rd3["location"].(string)
	if resumeURL == "" {
		return "", fmt.Errorf("无 resumption URL")
	}

	// /authorize?authorization_resumption_context -> 302
	req4, _ := fhttp.NewRequest("GET", resumeURL, nil)
	httputil.SetHeaders(req4, map[string]string{
		"Accept":                   "text/html,*/*;q=0.8",
		"User-Agent":               r.Identity.UA,
		"Referer":                  "https://view.awsapps.com/",
		"sec-fetch-dest":           "document",
		"sec-fetch-mode":           "navigate",
		"upgrade-insecure-requests": "1",
	})

	resp4, err := noRedirect.Do(req4)
	if err != nil {
		return "", err
	}
	resp4.Body.Close()
	if resp4.StatusCode != 302 {
		return "", fmt.Errorf("resumption 非 302: %d", resp4.StatusCode)
	}

	finalLoc := resp4.Header.Get("Location")
	code := httputil.ExtractParam(finalLoc, "code")
	if code == "" {
		return "", fmt.Errorf("Kiro callback 无 code: %s", finalLoc)
	}
	if st := httputil.ExtractParam(finalLoc, "state"); st != "" {
		r.KiroState = st
	}
	if len(code) > 30 {
		log.Printf("code=%s...", code[:30])
	}
	return code, nil
}
