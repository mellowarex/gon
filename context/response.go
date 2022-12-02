package context

import (
	"net"
	"net/http"
	"time"
	"bufio"
	"errors"
)

//Response is a wrapper for the http.ResponseWriter
//started set to true if response was written to then don't execute other handler
type Response struct {
	http.ResponseWriter
	Started bool 					// determine if response was already written
	Status  int  					// HTTP status code
	Elapsed time.Duration
}

func (this *Response) reset(rw http.ResponseWriter) {
	this.ResponseWriter = rw
	this.Status = 0
	this.Started = false
}

// Write writes the data to the connection as part of an HTTP reply,
// and sets `started` to true.
// started means the response was set.
func (this *Response) Write(p []byte) (int, error) {
	this.Started = true
	return this.ResponseWriter.Write(p)
}

// WriteHeader sends an HTTP response header with status code,
// and sets `started` to true.
func (this *Response) WriteHeader(code int) {
	if this.Status > 0 {
		//prevent multiple response.WriteHeader calls
		return
	}
	this.Status = code
	this.Started = true
	this.ResponseWriter.WriteHeader(code)
}

// Hijack hijacker for http
func (this *Response) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj, ok := this.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("webserver doesn't support hijacking")
	}
	return hj.Hijack()
}

// Flush http.Flusher
// flush buffered data to client
func (this *Response) Flush() {
	if f, ok := this.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// CloseNotify http.CloseNotifier
// cancel conn from client
// deprecated
func (this *Response) CloseNotify() <-chan bool {
	if cn, ok := this.ResponseWriter.(http.CloseNotifier); ok {
		return cn.CloseNotify()
	}
	return nil
}

// Pusher http.Pusher
func (this *Response) Pusher() (pusher http.Pusher) {
	if pusher, ok := this.ResponseWriter.(http.Pusher); ok {
		return pusher
	}
	return nil
}
