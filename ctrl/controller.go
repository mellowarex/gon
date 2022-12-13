package ctrl

import (
	"bytes"
	context2 "context"
	"crypto/rand"
	"errors"
	"fmt"
	"golang.org/x/crypto/bcrypt"
	"github.com/mellowarex/gon"
	"github.com/mellowarex/gon/context"
	"github.com/mellowarex/gon/session"
	"html/template"
	"io"
	r "math/rand"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

var (
	// ErrAbort custom error user error induced error
	ErrAbort = errors.New("user stop run")
	ErrorMaps = map[string]bool{
		"401": false,
		"402": false,
		"403": false,
		"413": false,
		"422": false,
		"417": false,
		"404": false,
		"405": false,
		"500": false,
		"501": false,
		"502": false,
		"503": false,
		"504": false,
	}
)

type Controller struct {
	Writer 							http.ResponseWriter
	Request  						*http.Request
	
	Ctx  								*context.Context
	
	methodMapping  			map[string]func()

	Data 								map[interface{}]interface{}
	Params 							map[string]string
	Flash  							map[string]string

	ControllerName   		string
	ActionName   				string

	// session
	Cookie   						*session.Cookie

	// template data
	TplName 						string
	ViewPath   					string
	Layout 							string
	LayoutSections   		map[string]string

	// xsrf data
	_xsrfToken  				string
	XSRFExpire  				int
	EnableXSRF  				bool
	Listen  						gon.Listen
}


// zero initializes all ctrl fields
func (this *Controller) Init(ctx *context.Context, listen gon.Listen) {
	this.Layout = ""
	this.TplName = ""
	this.Ctx = ctx
	this.Data = make(map[interface{}]interface{})
	this.Params = ctx.Input.Params
	this.Flash = make(map[string]string)
	this.LayoutSections = make(map[string]string)
	this.Writer = ctx.ResponseWriter
	this.Request = ctx.Request
	this.methodMapping = make(map[string]func())

	this.Listen = listen
}

func (this *Controller) BeforeAction() {}

// Get adds a request function to handle GET request.
func (this *Controller) Get() {
	http.Error(this.Writer, "Method Not Allowed", http.StatusMethodNotAllowed)
}

// Post adds a request function to handle POST request.
func (this *Controller) Post() {
	http.Error(this.Writer, "Method Not Allowed", http.StatusMethodNotAllowed)
}

// Delete adds a request function to handle DELETE request.
func (this *Controller) Delete() {
	http.Error(this.Writer, "Method Not Allowed", http.StatusMethodNotAllowed)
}

// Put adds a request function to handle PUT request.
func (this *Controller) Put() {
	http.Error(this.Writer, "Method Not Allowed", http.StatusMethodNotAllowed)
}

// Head adds a request function to handle HEAD request.
func (this *Controller) Head() {
	http.Error(this.Writer, "Method Not Allowed", http.StatusMethodNotAllowed)
}

// Patch adds a request function to handle PATCH request.
func (this *Controller) Patch() {
	http.Error(this.Writer, "Method Not Allowed", http.StatusMethodNotAllowed)
}

// Options adds a request function to handle OPTIONS request.
func (this *Controller) Options() {
	http.Error(this.Writer, "Method Not Allowed", http.StatusMethodNotAllowed)
}

// Trace adds a request function to handle Trace request.
// this method SHOULD NOT be overridden.
// https://tools.ietf.org/html/rfc7231#section-4.3.8
// The TRACE method requests a remote, application-level loop-back of
// the request message.  The final recipient of the request SHOULD
// reflect the message received, excluding some fields described below,
// back to the client as the message body of a 200 (OK) response with a
// Content-Type of "message/http" (Section 8.3.1 of [RFC7230]).
func (this *Controller) Trace() {
	ts := func(h http.Header) (hs string) {
		for k, v := range h {
			hs += fmt.Sprintf("\r\n%s: %s", k, v)
		}
		return
	}
	hs := fmt.Sprintf("\r\nTRACE %s %s%s\r\n", this.Request.RequestURI, this.Request.Proto, ts(this.Request.Header))
	this.Writer.Header().Set("Content-Type", "message/http")
	this.Writer.Header().Set("Content-Length", fmt.Sprint(len(hs)))
	this.Writer.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	this.Writer.Write([]byte(hs))
}

// Render sends the response with rendered template bytes as text/html type.
func (this *Controller) Render() error {
	this.Writer.Header().Set("Content-Type", "text/html; charset=utf-8")

	tpl, err := this.RenderBytes()
	if err != nil {
		return err
	}

	var encoding string
	var buf = &bytes.Buffer{}

	this.Writer.Header().Set("Content-Length", strconv.Itoa(len(tpl)))

	_, _, err = writeBody(encoding, buf, tpl)
	if err != nil {
		return err
	}

	io.Copy(this.Writer, buf)
	return nil
}

func (this *Controller)RenderBytes() ([]byte, error){
	buf, err := this.RenderTemplate()
	// if the controller has set layout, then first get the tplName's content set the content to the layout
	if err == nil && this.Layout != "" {
		this.Data["LayoutContent"] = template.HTML(buf.String())

		if this.LayoutSections != nil {
			for sectionName, sectionTpl := range this.LayoutSections {
				if sectionTpl == "" {
					this.Data[sectionName] = ""
					continue
				}
				buf.Reset()
				err = executeViewPathTemplate(&buf, sectionTpl, this.viewPath(), this.Data)
				if err != nil {
					return nil, err
				}
				this.Data[sectionName] = template.HTML(buf.String())
			}
		}

		buf.Reset()
		err = executeViewPathTemplate(&buf, this.Layout, this.viewPath(), this.Data)
	}
	return buf.Bytes(), err
}

func (this *Controller) ReadFlashData() {
	flash := this.Flash
	if cookie, err := this.Ctx.Request.Cookie(gon.GConfig.WebConfig.FlashName); err == nil {
		v, _ := url.QueryUnescape(cookie.Value)
		vals := strings.Split(v, "\x00")
		for _, v := range vals {
			if len(v) > 0 {
				kv := strings.Split(v, "\x23" + gon.GConfig.WebConfig.FlashSeparator + "\x23")
				if len(kv) == 2 {
					flash[kv[0]] = kv[1]
				}
			}
		}
		this.Ctx.SetCookie(gon.GConfig.WebConfig.FlashName, "", -1, "/")
	}
	this.Data["Flash"] = flash
}


func writeBody(encoding string, writer io.Writer, content []byte) (bool, string, error) {
	_, err := writer.Write(content)
	return false, "", err
}

// RenderTemplate returns bytes of rendered template string
func (c *Controller) RenderTemplate() (bytes.Buffer, error) {
	var buf bytes.Buffer
	if c.TplName == "" {
		if len(c.ControllerName) > 0 {
			c.TplName = strings.ToLower(c.ControllerName) + "/"
		}
		c.TplName += strings.ToLower(c.ActionName) + ".tpl"
	}
	buildFiles := []string{c.TplName}
	if c.Layout != "" {
		buildFiles = append(buildFiles, c.Layout)
		if c.LayoutSections != nil {
			for _, sectionTpl := range c.LayoutSections {
				if sectionTpl == "" {
					continue
				}
				buildFiles = append(buildFiles, sectionTpl)
			}
		}
	}
	buildTemplate(c.viewPath(), buildFiles...)
	return buf, executeViewPathTemplate(&buf, c.TplName, c.viewPath(), c.Data)
}

func (c *Controller) viewPath() string {
	if c.ViewPath == "" {
		return "views"
	}
	return c.ViewPath
}

// XSRFToken creates a CSRF token string and returns
func (c *Controller) XSRFToken() string {
	if c._xsrfToken == "" {
		expire := int64(gon.GConfig.WebConfig.XSRFExpire)
		if c.XSRFExpire > 0 {
			expire = int64(c.XSRFExpire)
		}
		c._xsrfToken = c.Ctx.XSRFToken(gon.GConfig.WebConfig.XSRFKey, expire)
	}
	return c._xsrfToken
}

// CheckXSRFCookie checks xsrf token in this request is valid or not.
// the token can provided in request header "X-Xsrftoken" and "X-CsrfToken"
// or in form field value named as "_xsrf".
func (c *Controller) CheckXSRFCookie() bool {
	if !c.EnableXSRF {
		return true
	}
	return c.Ctx.CheckXSRFCookie()
}

func (c *Controller) ControllerFunc(fn string) bool {
	if mt, ok := c.methodMapping[fn]; ok {
		mt()
		return true
	}
	return false
}

// URLMapping register the internal Controller router.
func (c *Controller) URLMapping() {}

func (c *Controller) Mapping(method string, fn func()) {
	c.methodMapping[method] = fn
}

func (c *Controller) AfterAction() {}

// Cookie sets cookie value via given key.
// others are ordered as cookie's 
// 1: max age time, 
// 2: path,
// 3: domain, 
// 4: secure 
// 5: httponly
func (c *Controller) SetCookie(name string, value string, others ...interface{}) {
	c.Ctx.Output.Cookie(name, value, others...)
}

// Cookie returns request cookie value for given string
// if non-existed, return empty string
func (c *Controller) GetCookie(key string) string{
	return c.Ctx.Input.GetCookie(key)
}

func (c *Controller) StartSession() *session.Cookie {
	if c.Cookie == nil {
		c.Cookie = c.Ctx.Input.Cookie
	}
	return c.Cookie
}

// SetSession puts value into session by name
func (c *Controller) SetSession(name string, value interface{}) error {
	if c.Cookie == nil {
		c.StartSession()
	}
	return c.Cookie.Set(context2.Background(), name, value, c.Writer)
}

// GetSession gets value from session
func (c *Controller) GetSession(name string) interface{} {
	if c.Cookie == nil {
		c.StartSession()
	}
	return c.Cookie.Get(context2.Background(),name)
}

// DelSession removes value from session.
func (c *Controller) DelSession(name string) error {
	if c.Cookie == nil {
		c.StartSession()
	}
	return c.Cookie.Delete(context2.Background(), name, c.Writer)
}

// FlushSession resets cookie values to empty map
func (c *Controller) FlushSession() error {
	if c.Cookie == nil {
		c.StartSession()
	}
	return c.Cookie.Flush(context2.Background(), c.Writer)
}

// SessionRegenerateID regenerates session id for this session.
// the session data have no changes.
func (c *Controller) SessionRegenerateID() error {
	// if c.Session == nil {
	// 	c.Session.SessionRelease(context2.Background(), c.Ctx.ResponseWriter)
	// }
	var err error
	c.Cookie, err = gon.GlobalSessions.SessionRegenerateID(c.Ctx.ResponseWriter, c.Ctx.Request)
	c.Ctx.Input.Cookie = c.Cookie
	return err
}

// DestroySession cleans session data and session cookie.
func (c *Controller) DestroySession() error {
	err := c.Ctx.Input.Cookie.Flush(nil, c.Writer)
	if err != nil {
		return err
	}
	c.Ctx.Input.Cookie = nil
	gon.GlobalSessions.SessionDestroy(c.Ctx.ResponseWriter, c.Ctx.Request)
	return nil
}

// Input returns the input data map from POST or PUT request body & query string
func (c *Controller) Input() (url.Values, error) {
	if c.Ctx.Request.Form == nil {
		err := c.Ctx.Request.ParseForm()
		if err != nil {
			return nil, err
		}
	}
	return c.Ctx.Request.Form, nil
}

// ParseForm maps input data map to obj struct
func (c *Controller) ParseForm(obj interface{}) error {
	form, err := c.Input()
	if err != nil {
		return err
	}
	return ParseForm(form, obj)
}

// GetString returns the input value by key string or the default value while it's present and input is blank
func (c *Controller) GetString(key string, def ...string) string {
	if v := c.Ctx.Input.Query(key); v != "" {
		return v
	}
	if len(def) > 0 {
		return def[0]
	}
	return ""
}

// GetStrings returns the input string slice by key string or the default value while it's present and input is blank
// it's designed for multi-value input field such as checkbox(input[type=checkbox]), multi-selection.
func (c *Controller) GetStrings(key string, def ...[]string) []string {
	var defv []string
	if len(def) > 0 {
		defv = def[0]
	}

	if f, err := c.Input(); f == nil || err != nil {
		return defv
	} else if vs := f[key]; len(vs) > 0 {
		return vs
	}

	return defv
}

// GetInt returns input as an int or the default value while it's present and input is blank
func (c *Controller) GetInt(key string, def ...int) (int, error) {
	strv := c.Ctx.Input.Query(key)
	if len(strv) == 0 && len(def) > 0 {
		return def[0], nil
	}
	return strconv.Atoi(strv)
}

// GetInt8 return input as an int8 or the default value while it's present and input is blank
func (c *Controller) GetInt8(key string, def ...int8) (int8, error) {
	strv := c.Ctx.Input.Query(key)
	if len(strv) == 0 && len(def) > 0 {
		return def[0], nil
	}
	i64, err := strconv.ParseInt(strv, 10, 8)
	return int8(i64), err
}

// GetUint8 return input as an uint8 or the default value while it's present and input is blank
func (c *Controller) GetUint8(key string, def ...uint8) (uint8, error) {
	strv := c.Ctx.Input.Query(key)
	if len(strv) == 0 && len(def) > 0 {
		return def[0], nil
	}
	u64, err := strconv.ParseUint(strv, 10, 8)
	return uint8(u64), err
}

// GetInt16 returns input as an int16 or the default value while it's present and input is blank
func (c *Controller) GetInt16(key string, def ...int16) (int16, error) {
	strv := c.Ctx.Input.Query(key)
	if len(strv) == 0 && len(def) > 0 {
		return def[0], nil
	}
	i64, err := strconv.ParseInt(strv, 10, 16)
	return int16(i64), err
}

// GetUint16 returns input as an uint16 or the default value while it's present and input is blank
func (c *Controller) GetUint16(key string, def ...uint16) (uint16, error) {
	strv := c.Ctx.Input.Query(key)
	if len(strv) == 0 && len(def) > 0 {
		return def[0], nil
	}
	u64, err := strconv.ParseUint(strv, 10, 16)
	return uint16(u64), err
}

// GetInt32 returns input as an int32 or the default value while it's present and input is blank
func (c *Controller) GetInt32(key string, def ...int32) (int32, error) {
	strv := c.Ctx.Input.Query(key)
	if len(strv) == 0 && len(def) > 0 {
		return def[0], nil
	}
	i64, err := strconv.ParseInt(strv, 10, 32)
	return int32(i64), err
}

// GetUint32 returns input as an uint32 or the default value while it's present and input is blank
func (c *Controller) GetUint32(key string, def ...uint32) (uint32, error) {
	strv := c.Ctx.Input.Query(key)
	if len(strv) == 0 && len(def) > 0 {
		return def[0], nil
	}
	u64, err := strconv.ParseUint(strv, 10, 32)
	return uint32(u64), err
}

// GetInt64 returns input value as int64 or the default value while it's present and input is blank.
func (c *Controller) GetInt64(key string, def ...int64) (int64, error) {
	strv := c.Ctx.Input.Query(key)
	if len(strv) == 0 && len(def) > 0 {
		return def[0], nil
	}
	return strconv.ParseInt(strv, 10, 64)
}

// GetUint64 returns input value as uint64 or the default value while it's present and input is blank.
func (c *Controller) GetUint64(key string, def ...uint64) (uint64, error) {
	strv := c.Ctx.Input.Query(key)
	if len(strv) == 0 && len(def) > 0 {
		return def[0], nil
	}
	return strconv.ParseUint(strv, 10, 64)
}

// GetBool returns input value as bool or the default value while it's present and input is blank.
func (c *Controller) GetBool(key string, def ...bool) (bool, error) {
	strv := c.Ctx.Input.Query(key)
	if len(strv) == 0 && len(def) > 0 {
		return def[0], nil
	}
	return strconv.ParseBool(strv)
}

// GetFloat returns input value as float64 or the default value while it's present and input is blank.
func (c *Controller) GetFloat(key string, def ...float64) (float64, error) {
	strv := c.Ctx.Input.Query(key)
	if len(strv) == 0 && len(def) > 0 {
		return def[0], nil
	}
	return strconv.ParseFloat(strv, 64)
}

// GetFile returns the file data in file upload field named as key.
// it returns the first one of multi-uploaded files.
func (c *Controller) GetFile(key string) (multipart.File, *multipart.FileHeader, error) {
	return c.Ctx.Request.FormFile(key)
}

// GetFiles return multi-upload files
// files, err:=c.GetFiles("myfiles")
//	if err != nil {
//		http.Error(w, err.Error(), http.StatusNoContent)
//		return
//	}
// for i, _ := range files {
//	//for each fileheader, get a handle to the actual file
//	file, err := files[i].Open()
//	defer file.Close()
//	if err != nil {
//		http.Error(w, err.Error(), http.StatusInternalServerError)
//		return
//	}
//	//create destination file making sure the path is writeable.
//	dst, err := os.Create("upload/" + files[i].Filename)
//	defer dst.Close()
//	if err != nil {
//		http.Error(w, err.Error(), http.StatusInternalServerError)
//		return
//	}
//	//copy the uploaded file to the destination file
//	if _, err := io.Copy(dst, file); err != nil {
//		http.Error(w, err.Error(), http.StatusInternalServerError)
//		return
//	}
// }
func (c *Controller) GetFiles(key string) ([]*multipart.FileHeader, error) {
	if files, ok := c.Ctx.Request.MultipartForm.File[key]; ok {
		return files, nil
	}
	return nil, http.ErrMissingFile
}

// SaveToFile saves uploaded file to new path.
// it only operates the first one of mutil-upload form file field.
func (c *Controller) SaveToFile(fromfile, tofile string) error {
	file, _, err := c.Ctx.Request.FormFile(fromfile)
	if err != nil {
		return err
	}
	defer file.Close()
	f, err := os.OpenFile(tofile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return err
	}
	defer f.Close()
	io.Copy(f, file)
	return nil
}

// XSRFField
func (c *Controller) XSRFField() string {
	return "<input type='hidden' name='_xsrf' value='"+c.XSRFToken()+"' />"
}

// 1: uri
// 2: subdomain
// 3: domain
// 4: port
// 5: redirectType
//
// build <scheme-name>:<hierachal-part>[?query]
func (this *Controller) Redirect(uri string, others ...string) {
	code := 303 // default redirect type

	// determine which http scheme to use
	url, domain,port := "http://", this.Listen.HTTPAddr, strconv.Itoa(this.Listen.HTTPPort)
	if this.Listen.EnableHTTPS {
		url, domain, port = "https://", this.Listen.HTTPSAddr, strconv.Itoa(this.Listen.HTTPSPort)
	}

	// if no others detected build up url from default values
	// depending on GConfig environment
	if len(others) == 0 {
		// check if domains contains string, select first choice
		if len(this.Listen.Domains) > 0 {
			url += this.Listen.Domains[0]
		}else { // if no domain provided, default to localhost
			url += domain
		}

		url += ":"
		// fill port
		if this.Listen.EnableHTTP {
			url += strconv.Itoa(this.Listen.HTTPPort)
		}else {
			url += strconv.Itoa(this.Listen.HTTPSPort)
		}
	}

	// attach subdomain 
	// append . eg admin.
	if len(others) > 0 {
		url += others[0] + "."
	}

	// attach domain
	if len(others) > 1 {
		url += others[1] 
	}else { // no domain provided, use default
		if len(this.Listen.Domains) > 0 {
			url += this.Listen.Domains[0]
		}else {
			url += domain
		}
	}

	// attach port
	if len(others) > 2 {
		url += ":" + others[2]
	}else {
		if os.Getenv("GON_ENV") != "production" {
			url += ":" + port
		}
	}

	// attach redirect type
	if len(others) > 3 {
		code, _ = strconv.Atoi(others[3])
	}

	url += "/" + uri
	fmt.Println(url)

	this.RedirectTo(code, url)
}

// Redirect sends redirection response to url with status code
func (this *Controller) RedirectTo(code int, url string) {
	this.Ctx.Redirect(code, url)
}

// Success writes success message flash
func (c *Controller) Success(msg string, args ...interface{}) {
	//write success only once
	if _,ok := c.Flash["success"]; ok {
		return
	}
	if len(args) == 0 {
		c.Flash["Success"] = msg
	} else {
		c.Flash["Success"] = fmt.Sprintf(msg, args...)
	}

	c.Store()
}

// Notice writes notice message flash
func (c *Controller) Notice(msg string, args ...interface{}) {
	//write success only once
	if _,ok := c.Flash["Notice"]; ok {
		return
	}
	if len(args) == 0 {
		c.Flash["Notice"] = msg
	} else {
		c.Flash["Notice"] = fmt.Sprintf(msg, args...)
	}

	c.Store()
}

// Warning writes warning messages to flash
func (c *Controller) Warning(msg string, args ...interface{}) {
	//write success only once
	if _,ok := c.Flash["Warning"]; ok {
		return
	}
	if len(args) == 0 {
		c.Flash["Warning"] = msg
	} else {
		c.Flash["Warning"] = fmt.Sprintf(msg, args...)
	}
	c.Store()
}

// Error writes error messages to flash
func (c *Controller) Error(msg string, args ...interface{}) {
	//write success only once
	if _,ok := c.Flash["Error"]; ok {
		return
	}
	if len(args) == 0 {
		c.Flash["Error"] = msg
	} else {
		c.Flash["Error"] = fmt.Sprintf(msg, args...)
	}

	c.Store()
}

// Set custom message to flash
func (c *Controller) FlashMsg(key, msg string, args ...interface{}) {
	//write custom flash only once
	if _,ok := c.Flash[key]; ok {
		return
	}
	if len(args) == 0 {
		c.Flash[key] = msg
	} else {
		c.Flash[key] = fmt.Sprintf(msg, args...)
	}

	c.Store()
}

// Store save flash data
// data is encoded and saved in cookie
func (c *Controller) Store() {
	c.Data["Flash"] = c.Flash
	var flashValue string
	for key, value := range c.Flash {
		flashValue += "\x00" + key + "\x23" + gon.GConfig.WebConfig.FlashSeparator + "\x23" + value + "\x00"
	}
	c.Ctx.SetCookie(gon.GConfig.WebConfig.FlashName, url.QueryEscape(flashValue), 0, "/")
}

// SetData set the data depending on the accepted
func (c *Controller) SetData(data interface{}) {
	accept := c.Ctx.Input.Header("Accept")
	switch accept {
	case context.ApplicationYAML:
		c.Data["yaml"] = data
	case context.ApplicationXML, context.TextXML:
		c.Data["xml"] = data
	default:
		c.Data["json"] = data
	}
}

// SendJSON sends a json response with encoding charset.
func (c *Controller) SendJSON(encoding ...bool) error {
	var (
		hasEncoding = len(encoding) > 0 && encoding[0]
	)

	return c.Ctx.Output.JSON(c.Data["json"], hasEncoding)
}

// SendJSONP sends a jsonp response.
func (c *Controller) SendJSONP() error {
	return c.Ctx.Output.JSONP(c.Data["jsonp"])
}

// SendXML sends xml response.
func (c *Controller) SendXML() error {
	return c.Ctx.Output.XML(c.Data["xml"])
}

// SendYAML sends yaml response.
func (c *Controller) SendYAML() error {
	return c.Ctx.Output.YAML(c.Data["yaml"])
}

// download file
func (c *Controller) DownloadFile(file , fileName string) error {
	return c.Ctx.Output.Download(file, fileName)
	// // create file
	// file, err := os.Create(filepath)
	// if err != nil {
	// 	return err
	// }
	// defer file.Close()

	// // get data
	// response, err := http.Get(url)
	// if err != nil {
	// 	return err
	// }
	// defer response.Body.Close()

	// // write the body to file
	// _, err = io.Copy(file, response.Body)
	// if err != nil {
	// 	return err
	// }

	// return c.Ctx.Output.Download()
}

// StopRun makes panic of USERSTOPRUN error and go to recover function if defined.
func (c *Controller) StopRun() {
	panic(ErrAbort)
}

// Abort stops controller handler and show the error data if code is defined in ErrorMap or code string.
func (c *Controller) Abort(code string) {
	status, err := strconv.Atoi(code)
	if err != nil {
		status = 200
	}
	c.CustomAbort(status, code)
}

// CustomAbort stops controller handler and show the error data, it's similar Aborts, but support status code and body.
func (c *Controller) CustomAbort(status int, body string) {
	// first panic from ErrorMaps, it is user defined error functions.
	if _, ok := ErrorMaps[body]; ok {
		c.Ctx.Output.Status = status
		panic(body)
	}
	// last panic user string
	c.Ctx.ResponseWriter.WriteHeader(status)
	c.Ctx.ResponseWriter.Write([]byte(body))
	panic(ErrAbort)
}

// MakePassword encrypts password using bcrypt
func (c *Controller) MakePassword(password string) (string, error) {
	pwd := []byte(password) // convert to slice byte
	hash, err := bcrypt.GenerateFromPassword(pwd, bcrypt.MinCost)
	if err != nil {
		return "", fmt.Errorf("failed to generate password: %v", err)
	}

	return string(hash), nil
}

// VerifyPassword compares string password with encrypted password
func (c *Controller) VerifyPassword(plainPassword, hashedPassword string) bool {
	bytePwd := []byte(plainPassword)
	byteHash := []byte(hashedPassword)

	err := bcrypt.CompareHashAndPassword(byteHash, bytePwd)
	if err != nil {
		return false
	}
	return true
}

var alphaNum = []byte(`0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz`)
var digits = []byte(`0123456789`)
var alphas = []byte(`ABCDEFGHIJKLMNOPQRSTUVWXYZ`)
// GenerateRandomNumbers
func (this *Controller) RandomNumbers(n int) string {
	var bytes = make([]byte, n)
	var randBy bool
	if num, err := rand.Read(bytes); num != n || err != nil {
		r.Seed(time.Now().UnixNano())
		randBy = true
	}
	for i, b := range bytes {
		if randBy {
			bytes[i] = digits[r.Intn(len(digits))]
		} else {
			bytes[i] = digits[b%byte(len(digits))]
		}
	}
	return string(bytes)
}

func (this *Controller) RandomAlphas(n int) string {
	var bytes = make([]byte, n)
	var randBy bool
	if num, err := rand.Read(bytes); num != n || err != nil {
		r.Seed(time.Now().UnixNano())
		randBy = true
	}
	for i, b := range bytes {
		if randBy {
			bytes[i] = alphas[r.Intn(len(alphas))]
		} else {
			bytes[i] = alphas[b%byte(len(alphas))]
		}
	}
	return string(bytes)
}

func (this *Controller) RandomKeys(n int) string {
	var bytes = make([]byte, n)
	var randBy bool
	if num, err := rand.Read(bytes); num != n || err != nil {
		r.Seed(time.Now().UnixNano())
		randBy = true
	}
	for i, b := range bytes {
		if randBy {
			bytes[i] = alphaNum[r.Intn(len(alphaNum))]
		} else {
			bytes[i] = alphaNum[b%byte(len(alphaNum))]
		}
	}
	return string(bytes)
}