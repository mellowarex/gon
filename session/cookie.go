package session

import (
	"context"
	"net/http"
	"net/url"
	"sync"
)

var cookieProvide = &CookieProvider{}

// Cookie SessionStore
type Cookie struct{
	sid   string
	values map[interface{}]interface{}
	lock sync.RWMutex
}

// Set value to cookie session
// the value are encoded as gob with hash block string
func (this *Cookie) Set(ctx context.Context, key, value interface{}, w http.ResponseWriter) error {
	this.lock.Lock()
	
	this.values[key] = value
	this.lock.Unlock()
	this.Save(w)
	return nil
}
// Save Write cookie session to http response cookie
func (this *Cookie) Save(w http.ResponseWriter) {
	this.lock.Lock()
	encodedCookie, err := encodeCookie(cookieProvide.block, cookieProvide.config.SecurityKey, cookieProvide.config.SecurityName, this.values)
	this.lock.Unlock()
	
	if err == nil {
		cookie := &http.Cookie{Name: cookieProvide.config.CookieName,
			Value:    url.QueryEscape(encodedCookie),
			Path:     "/",
			HttpOnly: true,
			Secure:   cookieProvide.config.Secure,
			MaxAge:   cookieProvide.config.Maxage,
			Domain:   "",
		}
		http.SetCookie(w, cookie)
	}
}

// Get value from cookie session
func (this *Cookie) Get(ctx context.Context, key interface{}) interface{} {
	this.lock.Lock()
	defer this.lock.Unlock()
	if v, ok := this.values[key]; ok {
		return v
	}
	return nil
}

// Delete value in cookie session
func (this *Cookie) Delete(ctx context.Context, key interface{}) error {
	this.lock.Lock()
	defer this.lock.Unlock()
	delete(this.values, key)
	return nil
}

// Flush Clean all values in cookie session
func (this *Cookie) Flush(context.Context) error {
	this.lock.Lock()
	defer this.lock.Unlock()
	this.values = make(map[interface{}]interface{})
	return nil
}

// SessionID Return id of this cookie session
func (this *Cookie) SessionID(context.Context) string {
	return this.sid
}