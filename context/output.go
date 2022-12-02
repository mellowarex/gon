package context

import (
	"bytes"
	"strconv"
	"io"
	"fmt"
	"strings"
	"time"
	"encoding/json"
	"encoding/xml"
	"errors"
	"html/template"
	"os"
	"path/filepath"
	"net/url"
	"net/http"
	"mime"

	yaml "gopkg.in/yaml.v2"
)

//commonly used mime-types
const (
	ApplicationJSON = "application/json"
	ApplicationXML  = "application/xml"
	ApplicationYAML = "application/x-yaml"
	TextXML         = "text/xml"
)

// GonOutput sends response header
type GonOutput struct {
	Context 	 *Context
	Status  	 int
	EnableGzip bool
}

// NewOutput returns new GonOutput
// contain nothing
func NewOutput() *GonOutput {
	return &GonOutput{}
}

// Reset initialize GonOutput
func (this *GonOutput) Reset(ctx *Context) {
	this.Context = ctx
	this.Status = 0
}

// Header sets response header item
// via key
func (this *GonOutput) Header(key, val string) {
	this.Context.ResponseWriter.Header().Set(key, val)
}

// Body sets response body content
// if EnableGzip, compress content string
// then sent out response body directly
func (this *GonOutput) Body(content []byte) error {
	var encoding string
	var buf = &bytes.Buffer{}
	if this.EnableGzip {
		encoding = ParseEncoding(this.Context.Request)
	}
	if b, n, _ := WriteBody(encoding, buf, content); b {
		this.Header("Content-Encoding", n)
		this.Header("Content-Length", strconv.Itoa(buf.Len()))
	} else {
		this.Header("Content-Length", strconv.Itoa(len(content)))
	}
	// Write status code if set 
	if this.Status != 0 {
		this.Context.ResponseWriter.WriteHeader(this.Status)
		this.Status = 0
	} else {
		this.Context.ResponseWriter.Started = true
	}
	io.Copy(this.Context.ResponseWriter, buf)
	return nil
}

// Cookie sets cookie value via given key.
// others are ordered as cookie's 
// 1: max age time, 
// 2: path,
// 3: domain, 
// 4: secure 
// 5: httponly
func (this *GonOutput) Cookie(name string, value string, others ...interface{}) {
	var b bytes.Buffer
	fmt.Fprintf(&b, "%s=%s", sanitizeName(name), sanitizeValue(value))

	//fix cookie not work in IE
	if len(others) > 0 {
		var maxAge int64

		switch v := others[0].(type) {
		case int:
			maxAge = int64(v)
		case int32:
			maxAge = int64(v)
		case int64:
			maxAge = v
		}

		switch {
		case maxAge > 0:
			fmt.Fprintf(&b, "; Expires=%s; Max-Age=%d", time.Now().Add(time.Duration(maxAge)*time.Second).UTC().Format(time.RFC1123), maxAge)
		case maxAge < 0:
			fmt.Fprintf(&b, "; Max-Age=0")
		}
	}

	// the settings below
	// Path, Domain, Secure, HttpOnly
	// can use nil skip set

	// default "/"
	if len(others) > 1 {
		if v, ok := others[1].(string); ok && len(v) > 0 {
			fmt.Fprintf(&b, "; Path=%s", sanitizeValue(v))
		}
	} else {
		fmt.Fprintf(&b, "; Path=%s", "/")
	}

	// default empty
	if len(others) > 2 {
		if v, ok := others[2].(string); ok && len(v) > 0 {
			fmt.Fprintf(&b, "; Domain=%s", sanitizeValue(v))
		}
	}

	// default empty
	if len(others) > 3 {
		var secure bool
		switch v := others[3].(type) {
		case bool:
			secure = v
		default:
			if others[3] != nil {
				secure = true
			}
		}
		if secure {
			fmt.Fprintf(&b, "; Secure")
		}
	}

	// default false for session cookie default true
	if len(others) > 4 {
		if v, ok := others[4].(bool); ok && v {
			fmt.Fprintf(&b, "; HttpOnly")
		}
	}

	this.Context.ResponseWriter.Header().Add("Set-Cookie", b.String())
}

var cookieNameSanitizer = strings.NewReplacer("\n", "-", "\r", "-")

// sanitizeName replace \n|\r with "-"
func sanitizeName(n string) string {
	return cookieNameSanitizer.Replace(n)
}

var cookieValueSanitizer = strings.NewReplacer("\n", " ", "\r", " ", ";", " ")

// sanitizeValue replace
// \n|\r with " "
// ; with " "
func sanitizeValue(v string) string {
	return cookieValueSanitizer.Replace(v)
}

// func jsonRenderer(value interface{}) Renderer {
// 	return rendererFunc(func(ctx *Context) {
// 		ctx.Output.JSON(value, false, false)
// 	})
// }

// func errorRenderer(err error) Renderer {
// 	return rendererFunc(func(ctx *Context) {
// 		ctx.Output.SetStatus(500)
// 		ctx.Output.Body([]byte(err.Error()))
// 	})
// }

// JSON writes json to the response body.
// if encoding is true, it converts utf-8 to \u0000 type.
func (output *GonOutput) JSON(data interface{}, encoding bool) error {
	output.Header("Content-Type", "application/json; charset=utf-8")
	var content []byte
	var err error
	content, err = json.Marshal(data)
	if err != nil {
		http.Error(output.Context.ResponseWriter, err.Error(), http.StatusInternalServerError)
		return err
	}
	if encoding {
		content = []byte(stringsToJSON(string(content)))
	}
	return output.Body(content)
}

// JSONP writes jsonp to the response body.
func (output *GonOutput) JSONP(data interface{}) error {
	output.Header("Content-Type", "application/javascript; charset=utf-8")
	var content []byte
	var err error
	content, err = json.Marshal(data)
	if err != nil {
		http.Error(output.Context.ResponseWriter, err.Error(), http.StatusInternalServerError)
		return err
	}
	callback := output.Context.Input.Query("callback")
	if callback == "" {
		return errors.New(`"callback" parameter required`)
	}
	callback = template.JSEscapeString(callback)
	callbackContent := bytes.NewBufferString(" if(window." + callback + ")" + callback)
	callbackContent.WriteString("(")
	callbackContent.Write(content)
	callbackContent.WriteString(");\r\n")
	return output.Body(callbackContent.Bytes())
}

// XML writes xml string to the response body.
func (output *GonOutput) XML(data interface{}) error {
	output.Header("Content-Type", "application/xml; charset=utf-8")
	var content []byte
	var err error
	content, err = xml.Marshal(data)
	if err != nil {
		http.Error(output.Context.ResponseWriter, err.Error(), http.StatusInternalServerError)
		return err
	}
	return output.Body(content)
}

// YAML writes yaml to the response body.
func (output *GonOutput) YAML(data interface{}) error {
	output.Header("Content-Type", "application/x-yaml; charset=utf-8")
	var content []byte
	var err error
	content, err = yaml.Marshal(data)
	if err != nil {
		http.Error(output.Context.ResponseWriter, err.Error(), http.StatusInternalServerError)
		return err
	}
	return output.Body(content)
}

// ServeFormatted serves YAML, XML or JSON, depending on the value of the Accept header
func (output *GonOutput) ServeFormatted(data interface{}, hasEncode ...bool) error {
	accept := output.Context.Input.Header("Accept")
	switch accept {
	case ApplicationYAML:
		return output.YAML(data)
	case ApplicationXML, TextXML:
		return output.XML(data)
	default:
		return output.JSON(data, len(hasEncode) > 0 && hasEncode[0])
	}
}

// Download forces response for download file.
// Prepares the download response header automatically.
func (output *GonOutput) Download(file string, filename ...string) error{
	// check get file error, file not found or other error.
	if _, err := os.Stat(file); err != nil {
		http.ServeFile(output.Context.ResponseWriter, output.Context.Request, file)
		return err
	}

	var fName string
	if len(filename) > 0 && filename[0] != "" {
		fName = filename[0]
	} else {
		fName = filepath.Base(file)
	}
	// https://tools.ietf.org/html/rfc6266#section-4.3
	fn := url.PathEscape(fName)
	if fName == fn {
		fn = "filename=" + fn
	} else {
		/**
		  The parameters "filename" and "filename*" differ only in that
		  "filename*" uses the encoding defined in [RFC5987], allowing the use
		  of characters not present in the ISO-8859-1 character set
		  ([ISO-8859-1]).
		*/
		fn = "filename=" + fName + "; filename*=utf-8''" + fn
	}
	output.Header("Content-Disposition", "attachment; "+fn)
	output.Header("Content-Description", "File Transfer")
	output.Header("Content-Type", "application/octet-stream")
	output.Header("Content-Transfer-Encoding", "binary")
	output.Header("Expires", "0")
	output.Header("Cache-Control", "must-revalidate")
	output.Header("Pragma", "public")
	http.ServeFile(output.Context.ResponseWriter, output.Context.Request, file)

	return nil
}

// ContentType sets the content type from ext string
// MIME type is given in mime package
func (this *GonOutput) ContentType(ext string) {
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	ctype := mime.TypeByExtension(ext)
	if ctype != "" {
		this.Header("Content-Type", ctype)
	}
}

// SetStatus sets response status code
// It writes response header directly
func (this *GonOutput) SetStatus(status int) {
	this.Status = status
}

// IsCachable returns boolean if this request is cached
// HTTP 304 means cached
func (this *GonOutput) IsCachable() bool {
	return this.Status >= 200 && this.Status < 300 || this.Status == 304
}

// IsEmpty returns boolean if this request is empty
// HTTP 201，204 and 304 means empty
func (this *GonOutput) IsEmpty() bool {
	return this.Status == 201 || this.Status == 204 || this.Status == 304
}

// IsOk returns boolean if this request runs well
// HTTP 200 means ok
func (this *GonOutput) IsOk() bool {
	return this.Status == 200
}

// IsSuccessful returns boolean if this request runs successfully
// HTTP 2xx means ok
func (this *GonOutput) IsSuccessful() bool {
	return this.Status >= 200 && this.Status < 300
}

// IsRedirect returns boolean if this request is redirection header
// HTTP 301,302,307 means redirection
func (this *GonOutput) IsRedirect() bool {
	return this.Status == 301 || this.Status == 302 || this.Status == 303 || this.Status == 307
}

// IsForbidden returns boolean if this request is forbidden
// HTTP 403 means forbidden
func (this *GonOutput) IsForbidden() bool {
	return this.Status == 403
}

// IsNotFound returns boolean if this request is not found
// HTTP 404 means not found
func (this *GonOutput) IsNotFound() bool {
	return this.Status == 404
}

// IsClientError returns boolean if this request client sends error data
// HTTP 4xx means client error
func (this *GonOutput) IsClientError() bool {
	return this.Status >= 400 && this.Status < 500
}

// IsServerError returns boolean of this server handler errors
// HTTP 5xx means server internal error
func (this *GonOutput) IsServerError() bool {
	return this.Status >= 500 && this.Status < 600
}

func stringsToJSON(str string) string {
	var jsons bytes.Buffer
	for _, r := range str {
		rint := int(r)
		if rint < 128 {
			jsons.WriteRune(r)
		} else {
			jsons.WriteString("\\u")
			if rint < 0x100 {
				jsons.WriteString("00")
			} else if rint < 0x1000 {
				jsons.WriteString("0")
			}
			jsons.WriteString(strconv.FormatInt(int64(rint), 16))
		}
	}
	return jsons.String()
}

// Session sets session item value with given key
// func (this *GonOutput) Session(name interface{}, value interface{}) {
// 	this.Context.Input.Cookie.Set(name, value)
// }