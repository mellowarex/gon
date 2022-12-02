package context

import (
	"net/http"
	"encoding/base64"
	"strconv"
	"crypto/hmac"
	"crypto/sha256"
	"time"
	"fmt"
	"strings"

	"github.com/mellocraft/gon/utils"
)

type Context struct {
	Input 					*GonInput
	Output  			*GonOutput
	Request 				*http.Request
	ResponseWriter	*Response
	_xsrfToken			string
}

// NewContext returns empty Input & Output
func NewContext() *Context {
	return &Context{
		Input: NewInput(),
		Output: NewOutput(),
	}
}

// Reset initializes from given http:
// Context, GonInput & GonOutput
// according to arguments given
func (this *Context) Reset(w http.ResponseWriter, r *http.Request) {
	this.Request = r
	if this.ResponseWriter == nil {
		this.ResponseWriter = &Response{}
	}
	this.ResponseWriter.reset(w)
	this.Input.Reset(this)
	this.Output.Reset(this)
	this._xsrfToken = ""
}

// XSRFToken creates and returns xsrf token string
func (this *Context) XSRFToken(key string, expire int64) string {
	if this._xsrfToken == "" {
		token, ok := this.GetSecureCookie(key, "_xsrf")
		if !ok {
			token = string(utils.RandomCreateBytes(32))
			this.SetSecureCookie(key, "_xsrf", token, expire, "", "")
		}
		this._xsrfToken = token
	}
	return this._xsrfToken
}

// SetSecureCookie for response
func (this *Context) SetSecureCookie(Secret, name, value string, others ...interface{}) {
	vs := base64.URLEncoding.EncodeToString([]byte(value))
	timestamp := strconv.FormatInt(time.Now().UnixNano(), 10)
	h := hmac.New(sha256.New, []byte(Secret))
	fmt.Fprintf(h, "%s%s", vs, timestamp)
	sig := fmt.Sprintf("%02x", h.Sum(nil))
	cookie := strings.Join([]string{vs, timestamp, sig}, "|")
	this.Output.Cookie(name, cookie, others...)
}

// GetSecureCookie from request for given key
// if non-existant, return empty string, false
func (this *Context) GetSecureCookie(Secret, key string) (string, bool) {
	val := this.Input.GetCookie(key)
	if val == "" {
		return "", false
	}

	parts := strings.SplitN(val, "|", 3)
	if len(parts) != 3 {
		return "", false
	}

	vs, timestamp, sig := parts[0], parts[1], parts[2]

	h := hmac.New(sha256.New, []byte(Secret))
	fmt.Fprintf(h, "%s%s", vs, timestamp)

	if fmt.Sprintf("%02x", h.Sum(nil)) != sig {
		return "", false
	}
	res, _ := base64.URLEncoding.DecodeString(vs)
	return string(res), true
}

// CheckXSRFCookie checks if the XSRF token in this request is valid or not
// The token can be provided in the request header in the form "X-Xsrftoken" or "X-CsrfToken"
// or in form field value named as "_xsrf".
func (this *Context) CheckXSRFCookie() bool {
	token := this.Input.Query("_xsrf")
	if token == "" {
		token = this.Request.Header.Get("X-Xsrftoken")
	}
	if token == "" {
		token = this.Request.Header.Get("X-Csrftoken")
	}
	if token == "" {
		this.Abort(422, "422")
		return false
	}
	if this._xsrfToken != token {
		this.Abort(417, "417")
		return false
	}
	return true
}

// WriteString writes string to response body
func (this *Context) WriteString(content string) {
	this.ResponseWriter.Write([]byte(content))
}

// Redirect redirects to localurl with http header status code
func (this *Context) Redirect(status int, localurl string) {
	http.Redirect(this.ResponseWriter, this.Request, localurl, status)
}

// Abort stops the request
// if gon.ErrorMaps exists, panic body
func (this *Context) Abort(status int, body string) {
	this.Output.SetStatus(status)
	panic(body)
}

// SetCookie sets a cookie for a response
func (this *Context) SetCookie(name, value string, others ...interface{}) {
	this.Output.Cookie(name, value, others...)
}