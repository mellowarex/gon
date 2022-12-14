package session

import (
	"context"
	"net/http"
	"net/url"
	"sync"
	"fmt"
)

var cookieProvide = &CookieProvider{}

// Cookie SessionStore
type Cookie struct{
	sid   string
	values map[string]interface{}
	lock sync.RWMutex
}

// Set value to cookie session
// the value are encoded as gob with hash block string
func (this *Cookie) Set(ctx context.Context, key string, value interface{},r *http.Request, w http.ResponseWriter) error {
	this.lock.Lock()
	defer this.lock.Unlock()
	this.values[key] = value
	fmt.Println("set cookie: ", this.values)
	return nil
}
// Save Write cookie session to http response cookie
func (this *Cookie) Save(ctx context.Context,rr interface{} ,r *http.Request, w http.ResponseWriter) error {
	encodedCookie, err := encodeCookie(cookieProvide.block, cookieProvide.config.SecurityKey, cookieProvide.config.SecurityName, this.values)
	fmt.Println("saving: ",this.values)
	fmt.Println("cookie: ",encodedCookie)
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
		r.AddCookie(cookie)
	}
	return nil
}

// Get value from cookie session
func (this *Cookie) Get(ctx context.Context, key string) interface{} {
	this.lock.RLock()
	defer this.lock.RUnlock()
	if v, ok := this.values[key]; ok {
		return v
	}
	return nil
}

// Delete value in cookie session
func (this *Cookie) Delete(ctx context.Context, key string,r *http.Request, w http.ResponseWriter) error {
	this.lock.Lock()
	defer this.lock.Unlock()
	delete(this.values, key)
	return nil
}

// Flush Clean all values in cookie session
func (this *Cookie) Flush(ctx context.Context,r *http.Request, w http.ResponseWriter) error {
	this.lock.Lock()
	defer this.lock.Unlock()
	this.values = make(map[string]interface{})
	return nil
}

// SessionID Return id of this cookie session
func (this *Cookie) SessionID(context.Context) string {
	return this.sid
}