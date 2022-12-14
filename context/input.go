package context

import (
	"bytes"
	"sync"
	"strings"
	"errors"
	"io"
	"io/ioutil"
	"compress/gzip"
	"net"
	"net/url"
	"net/http"
	"regexp"
	"strconv"
	"reflect"

	"github.com/mellowarex/gon/session"
)

var (
	acceptsHTMLRegex = regexp.MustCompile(`(text/html|application/xhtml\+xml)(?:,|$)`)
	acceptsXMLRegex  = regexp.MustCompile(`(application/xml|text/xml)(?:,|$)`)
	acceptsJSONRegex = regexp.MustCompile(`(application/json)(?:,|$)`)
	acceptsYAMLRegex = regexp.MustCompile(`(application/x-yaml)(?:,|$)`)
	maxParam         = 50
)

// GonInput represents
// - http request header
// - data
// - cookie
// - url params & params body
type GonInput struct {
	Context *Context
	Cookie 	*session.Cookie
	Params	map[string]string
	pnames	[]string
	pvalues	[]string
	data 		map[interface{}]interface{}
	dataLock	sync.RWMutex
	RequestBody []byte
}

// NewInput returns GonInput generated by Context
func NewInput() *GonInput {
	return &GonInput{
		Params:		make(map[string]string),
		pnames:		make([]string, 0, 50),
		pvalues:	make([]string, 0, 50),
		data:  		make(map[interface{}]interface{}),
	}
}

// Reset initializes GonInput
func (this *GonInput) Reset(ctx *Context) {
	this.Context = ctx
	this.Cookie = nil
	this.Params = nil
	this.pnames = this.pnames[:0]
	this.pvalues = this.pvalues[:0]
	this.dataLock.Lock()
	this.data = nil
	this.dataLock.Unlock()
	this.RequestBody = []byte{}
}

// Method returns http request method
func (this *GonInput) Method() string {
	return this.Context.Request.Method
}

// ParamsLen return length of the params
func (this *GonInput) ParamsLen() int {
	return len(this.pnames)
}

// SetParam sets param with key & value
func (this *GonInput) SetParam(key, val string) {
	for i, v := range this.pnames {
		if v == key && i <= len(this.pvalues) {
			this.pvalues[i] = val
			return
		}
	}
	this.pnames = append(this.pnames, key)
	this.pvalues = append(this.pvalues, val)
}

// ResetParams clears all input's params
func (this *GonInput) ResetParams() {
	this.pnames = this.pnames[:0]
	this.pvalues = this.pvalues[:0]
}

// Param returns url param value for key given string
// if non-existed, returns empty string
func (this *GonInput) Param(key string) string {
	for i, v := range this.pnames {
		if v == key && i <= len(this.pvalues) {
			return this.pvalues[i]
		}
	}
	return ""
}

// Query returns data from params or form data item by given string
// first check if key is in url param values
// if nil check in form data
// if non-existed, return empty string
func (this *GonInput) Query(key string) string {
	if val := this.Param(key); val != "" {
		return val
	}
	if this.Context.Request.Form == nil {
		this.dataLock.Lock()
		this.Context.Request.ParseForm()
		this.dataLock.Unlock()
	}
	this.dataLock.RLock()
	defer this.dataLock.RUnlock()
	return this.Context.Request.Form.Get(key)
}

// Header return request header value for given key
// if non-existant, return empty string
func (this *GonInput) Header(key string) string {
	return this.Context.Request.Header.Get(key)
}

// URL returns the request url (without query, string & fragment)
func (this *GonInput) URL() string {
	return this.Context.Request.URL.Path
}

// URI returns the full request url with query, string & fragment
func (this *GonInput) URI() string {
	return this.Context.Request.RequestURI
}

// Cookie returns request cookie value for given string
// if non-existed, return empty string
func (this *GonInput) GetCookie(key string) string {
	cookie, err := this.Context.Request.Cookie(key)
	if err != nil {
		return ""
	}
	return cookie.Value
}

// ParseFormorMultiForm parseform or parseMultiForm based
// Content-Type
func (this *GonInput) ParseForm(maxMemory int64) error {
	if this.IsUpload() {
		if err := this.Context.Request.ParseMultipartForm(maxMemory); err != nil {
			return errors.New("error parsing request body: " + err.Error())
		}
	} else if err := this.Context.Request.ParseForm(); err != nil {
		return errors.New("error parsing request body: " + err.Error())
	}
	return nil
}

// CopyBody returns raw request body data as bytes
func (this *GonInput) CopyBody(maxMemory int64) []byte {
	if this.Context.Request.Body == nil {
		return []byte{}
	}

	var requestBody []byte
	safe := &io.LimitedReader{
		R: this.Context.Request.Body,
		N: maxMemory,
	}
	if this.Header("Content-Encoding") == "gzip" {
		reader, err := gzip.NewReader(safe)
		if err != nil {
			return nil
		}
		requestBody, _ = ioutil.ReadAll(reader)
	} else {
		requestBody, _ = ioutil.ReadAll(safe)
	}
	this.Context.Request.Body.Close()
	bf := bytes.NewBuffer(requestBody)
	this.Context.Request.Body = http.MaxBytesReader(this.Context.ResponseWriter, ioutil.NopCloser(bf), maxMemory)
	this.RequestBody = requestBody
	return requestBody
}

// IP returns request client ip.
// if in proxy, return first proxy id.
// if error, return RemoteAddr.
// IP returns request client ip.
// if in proxy, return first proxy id.
// if error, return RemoteAddr.
func (this *GonInput) IP() string {
	ips := this.Proxy()
	if len(ips) > 0 && ips[0] != "" {
		rip, _, err := net.SplitHostPort(ips[0])
		if err != nil {
			rip = ips[0]
		}
		return rip
	}
	if ip, _, err := net.SplitHostPort(this.Context.Request.RemoteAddr); err == nil {
		return ip
	}
	return this.Context.Request.RemoteAddr
}

// Proxy returns proxy client ips slice
func (this *GonInput) Proxy() []string {
	if ips := this.Header("X-Forwarded-For"); ips != "" {
		return strings.Split(ips, ",")
	}
	return []string{}
}

// Protocal returns HTTP protocal: HTTP/1.1
func (this *GonInput) Protocol() string {
	return this.Context.Request.Proto
}

// Site return base site url as scheme://domain type
func (this *GonInput) Site() string {
	return this.Scheme() + "://" +this.Domain()
}

// Scheme returns request scheme: http / https
func (this *GonInput) Scheme() string {
	if scheme := this.Header("X-Forwarded-Proto"); scheme != "" {
		return scheme
	}
	if this.Context.Request.URL.Scheme != "" {
		return this.Context.Request.URL.Scheme
	}
	if this.Context.Request.TLS == nil {
		return "http"
	}
	return "https"
}

// Domain returns host name
// Alias of Host method
func (this *GonInput) Domain() string {
	return this.Host()
}

// Host returns host name
// if no host info in request, return localhost
func (this *GonInput) Host() string {
	if this.Context.Request.Host != "" {
		if hostPart, _, err := net.SplitHostPort(this.Context.Request.Host); err == nil {
			return hostPart
		}
		return this.Context.Request.Host
	}
	return "localhost"
}

// IsHTTPMethod returns boolean for request on given method
// IsHTTPMethod("GET")
func (this *GonInput) IsHTTPMethod(method string) bool {
	return this.Method() == method
}

// IsGet Is this a GET method request?
func (this *GonInput) IsGet() bool {
	return this.IsHTTPMethod("GET")
}

// IsPost Is this a POST method request?
func (this *GonInput) IsPost() bool {
	return this.IsHTTPMethod("POST")
}

// IsHead Is this a Head method request?
func (this *GonInput) IsHead() bool {
	return this.IsHTTPMethod("HEAD")
}

// IsOptions Is this a OPTIONS method request?
func (this *GonInput) IsOptions() bool {
	return this.IsHTTPMethod("OPTIONS")
}

// IsPut Is this a PUT method request?
func (this *GonInput) IsPut() bool {
	return this.IsHTTPMethod("PUT")
}

// IsDelete Is this a DELETE method request?
func (this *GonInput) IsDelete() bool {
	return this.IsHTTPMethod("DELETE")
}

// IsPatch Is this a PATCH method request?
func (this *GonInput) IsPatch() bool {
	return this.IsHTTPMethod("PATCH")
}

// IsAjax returns boolean of this request is generated by ajax.
func (this *GonInput) IsAjax() bool {
	return this.Header("X-Requested-With") == "XMLHttpRequest"
}

// IsSecure returns boolean of this request is in https.
func (this *GonInput) IsSecure() bool {
	return this.Scheme() == "https"
}

// IsWebsocket returns boolean of this request is in webSocket.
func (this *GonInput) IsWebsocket() bool {
	return this.Header("Upgrade") == "websocket"
}

// IsUpload returns boolean of whether file uploads in this request or not..
func (this *GonInput) IsUpload() bool {
	return strings.Contains(this.Header("Content-Type"), "multipart/form-data")
}

// AcceptsHTML check if request accepts html response
func (this *GonInput) AcceptsHTML() bool {
	return acceptsHTMLRegex.MatchString(this.Header("Accept"))
}

// AcceptsXML checks if request accpets xml response
func (this *GonInput) AcceptsXML() bool {
	return acceptsXMLRegex.MatchString(this.Header("Accpet"))
}

// AcceptsJSON checks if request accepts json response
func (this *GonInput) AcceptsJSON() bool {
	return acceptsJSONRegex.MatchString(this.Header("Accept"))
}

// AcceptsYAML checks if request accepts yaml response
func (this *GonInput) AcceptsYAML() bool {
	return acceptsYAMLRegex.MatchString(this.Header("Accept"))
}

// Referer return http referef header
func (this *GonInput) Referer() string {
	return this.Header("Referer")
}

// Refer returns http referer header
func (this *GonInput) Refer() string {
	return this.Referer()
}

// SubDomains returns sub domain string
// if aa.bb.domain.com, returns aa.bb
func (this *GonInput) SubDomains() string {
	parts := strings.Split(this.Host(), ".")
	if len(parts) >= 3 {
		return strings.Join(parts[:len(parts)-2], ".")
	}
	return ""
}

// Port returns request client port
func (this *GonInput) Port() int {
	if _, portPart, err := net.SplitHostPort(this.Context.Request.Host); err == nil {
		port, _ := strconv.Atoi(portPart)
		return port
	}
	return 80
}

// UserAgent returns request client user agent string
func (this *GonInput) UserAgent() string {
	return this.Header("User-Agent")
}

// Session returns current session item value by key
// if non-existant, return nil
// func (this *GonInput) Session(key interface{}) interface{} {
// 	return this.Cookie.Get(key)
// }

// Data returns implicit data in input
func (this *GonInput) Data() map[interface{}]interface{} {
	this.dataLock.Lock()
	defer this.dataLock.Unlock()
	if this.data == nil {
		this.data = make(map[interface{}]interface{})
	}
	return this.data
}

// GetData returns stored data 
// if non-existant, return nil
func (this *GonInput) GetData(key interface{}) interface{} {
	this.dataLock.Lock()
	defer this.dataLock.Unlock()
	if v, ok := this.data[key]; ok {
		return v
	}
	return nil
}

// SetData stores data with given key
// data only available in current context
func (this *GonInput) SetData(key, val interface{}) {
	this.dataLock.Lock()
	defer this.dataLock.Unlock()
	if this.data == nil {
		this.data = make(map[interface{}]interface{})
	}
	this.data[key] = val
}

// Bind data from request.Form[key] to dest
// like /?id=123&isok=true&ft=1.2&ol[0]=1&ol[1]=2&ul[]=str&ul[]=array&user.Name=astaxie
// var id int  GonInput.Bind(&id, "id")  id ==123
// var isok bool  GonInput.Bind(&isok, "isok")  isok ==true
// var ft float64  GonInput.Bind(&ft, "ft")  ft ==1.2
// ol := make([]int, 0, 2)  GonInput.Bind(&ol, "ol")  ol ==[1 2]
// ul := make([]string, 0, 2)  GonInput.Bind(&ul, "ul")  ul ==[str array]
// user struct{Name}  GonInput.Bind(&user, "user")  user == {Name:"astaxie"}
func (this *GonInput) Bind(dest interface{}, key string) error {
	value := reflect.ValueOf(dest)
	if value.Kind() != reflect.Ptr {
		return errors.New("Gon: non-pointer passed to Bind: " + key)
	}
	value = value.Elem()
	if !value.CanSet() {
		return errors.New("Gon: non-settable variable passed to Bind: " + key)
	}
	typ := value.Type()
	// Get real type if dest define with interface{}.
	// e.g  var dest interface{} dest=1.0
	if value.Kind() == reflect.Interface {
		typ = value.Elem().Type()
	}
	rv := this.bind(key, typ)
	if !rv.IsValid() {
		return errors.New("Gon: reflect value is empty")
	}
	value.Set(rv)
	return nil
}

func (input *GonInput) bind(key string, typ reflect.Type) reflect.Value {
	if input.Context.Request.Form == nil {
		input.Context.Request.ParseForm()
	}
	rv := reflect.Zero(typ)
	switch typ.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		val := input.Query(key)
		if len(val) == 0 {
			return rv
		}
		rv = input.bindInt(val, typ)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		val := input.Query(key)
		if len(val) == 0 {
			return rv
		}
		rv = input.bindUint(val, typ)
	case reflect.Float32, reflect.Float64:
		val := input.Query(key)
		if len(val) == 0 {
			return rv
		}
		rv = input.bindFloat(val, typ)
	case reflect.String:
		val := input.Query(key)
		if len(val) == 0 {
			return rv
		}
		rv = input.bindString(val, typ)
	case reflect.Bool:
		val := input.Query(key)
		if len(val) == 0 {
			return rv
		}
		rv = input.bindBool(val, typ)
	case reflect.Slice:
		rv = input.bindSlice(&input.Context.Request.Form, key, typ)
	case reflect.Struct:
		rv = input.bindStruct(&input.Context.Request.Form, key, typ)
	case reflect.Ptr:
		rv = input.bindPoint(key, typ)
	case reflect.Map:
		rv = input.bindMap(&input.Context.Request.Form, key, typ)
	}
	return rv
}

func (input *GonInput) bindValue(val string, typ reflect.Type) reflect.Value {
	rv := reflect.Zero(typ)
	switch typ.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		rv = input.bindInt(val, typ)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		rv = input.bindUint(val, typ)
	case reflect.Float32, reflect.Float64:
		rv = input.bindFloat(val, typ)
	case reflect.String:
		rv = input.bindString(val, typ)
	case reflect.Bool:
		rv = input.bindBool(val, typ)
	case reflect.Slice:
		rv = input.bindSlice(&url.Values{"": {val}}, "", typ)
	case reflect.Struct:
		rv = input.bindStruct(&url.Values{"": {val}}, "", typ)
	case reflect.Ptr:
		rv = input.bindPoint(val, typ)
	case reflect.Map:
		rv = input.bindMap(&url.Values{"": {val}}, "", typ)
	}
	return rv
}

func (input *GonInput) bindInt(val string, typ reflect.Type) reflect.Value {
	intValue, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return reflect.Zero(typ)
	}
	pValue := reflect.New(typ)
	pValue.Elem().SetInt(intValue)
	return pValue.Elem()
}

func (input *GonInput) bindUint(val string, typ reflect.Type) reflect.Value {
	uintValue, err := strconv.ParseUint(val, 10, 64)
	if err != nil {
		return reflect.Zero(typ)
	}
	pValue := reflect.New(typ)
	pValue.Elem().SetUint(uintValue)
	return pValue.Elem()
}

func (input *GonInput) bindFloat(val string, typ reflect.Type) reflect.Value {
	floatValue, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return reflect.Zero(typ)
	}
	pValue := reflect.New(typ)
	pValue.Elem().SetFloat(floatValue)
	return pValue.Elem()
}

func (input *GonInput) bindString(val string, typ reflect.Type) reflect.Value {
	return reflect.ValueOf(val)
}

func (input *GonInput) bindBool(val string, typ reflect.Type) reflect.Value {
	val = strings.TrimSpace(strings.ToLower(val))
	switch val {
	case "true", "on", "1":
		return reflect.ValueOf(true)
	}
	return reflect.ValueOf(false)
}

type sliceValue struct {
	index int           // Index extracted from brackets.  If -1, no index was provided.
	value reflect.Value // the bound value for this slice element.
}

func (input *GonInput) bindSlice(params *url.Values, key string, typ reflect.Type) reflect.Value {
	maxIndex := -1
	numNoIndex := 0
	sliceValues := []sliceValue{}
	for reqKey, vals := range *params {
		if !strings.HasPrefix(reqKey, key+"[") {
			continue
		}
		// Extract the index, and the index where a sub-key starts. (e.g. field[0].subkey)
		index := -1
		leftBracket, rightBracket := len(key), strings.Index(reqKey[len(key):], "]")+len(key)
		if rightBracket > leftBracket+1 {
			index, _ = strconv.Atoi(reqKey[leftBracket+1 : rightBracket])
		}
		subKeyIndex := rightBracket + 1

		// Handle the indexed case.
		if index > -1 {
			if index > maxIndex {
				maxIndex = index
			}
			sliceValues = append(sliceValues, sliceValue{
				index: index,
				value: input.bind(reqKey[:subKeyIndex], typ.Elem()),
			})
			continue
		}

		// It's an un-indexed element.  (e.g. element[])
		numNoIndex += len(vals)
		for _, val := range vals {
			// Unindexed values can only be direct-bound.
			sliceValues = append(sliceValues, sliceValue{
				index: -1,
				value: input.bindValue(val, typ.Elem()),
			})
		}
	}
	resultArray := reflect.MakeSlice(typ, maxIndex+1, maxIndex+1+numNoIndex)
	for _, sv := range sliceValues {
		if sv.index != -1 {
			resultArray.Index(sv.index).Set(sv.value)
		} else {
			resultArray = reflect.Append(resultArray, sv.value)
		}
	}
	return resultArray
}

func (input *GonInput) bindStruct(params *url.Values, key string, typ reflect.Type) reflect.Value {
	result := reflect.New(typ).Elem()
	fieldValues := make(map[string]reflect.Value)
	for reqKey, val := range *params {
		var fieldName string
		if strings.HasPrefix(reqKey, key+".") {
			fieldName = reqKey[len(key)+1:]
		} else if strings.HasPrefix(reqKey, key+"[") && reqKey[len(reqKey)-1] == ']' {
			fieldName = reqKey[len(key)+1 : len(reqKey)-1]
		} else {
			continue
		}

		if _, ok := fieldValues[fieldName]; !ok {
			// Time to bind this field.  Get it and make sure we can set it.
			fieldValue := result.FieldByName(fieldName)
			if !fieldValue.IsValid() {
				continue
			}
			if !fieldValue.CanSet() {
				continue
			}
			boundVal := input.bindValue(val[0], fieldValue.Type())
			fieldValue.Set(boundVal)
			fieldValues[fieldName] = boundVal
		}
	}

	return result
}

func (input *GonInput) bindPoint(key string, typ reflect.Type) reflect.Value {
	return input.bind(key, typ.Elem()).Addr()
}

func (input *GonInput) bindMap(params *url.Values, key string, typ reflect.Type) reflect.Value {
	var (
		result    = reflect.MakeMap(typ)
		keyType   = typ.Key()
		valueType = typ.Elem()
	)
	for paramName, values := range *params {
		if !strings.HasPrefix(paramName, key+"[") || paramName[len(paramName)-1] != ']' {
			continue
		}

		key := paramName[len(key)+1 : len(paramName)-1]
		result.SetMapIndex(input.bindValue(key, keyType), input.bindValue(values[0], valueType))
	}
	return result
}