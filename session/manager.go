package session

import (
	"errors"
	"fmt"
	"net/http"
	"net/textproto"
	"net/url"
	"time"
)

// Manager contains cookie and its configurations
type Manager struct{
	provider *CookieProvider
	config *CookieConfig
}

// CookieConfig define the session config
type CookieConfig struct {
	CookieName              string `json:"cookieName"`
	EnableSetCookie         bool   `json:"enableSetCookie,omitempty"`
	Gclifetime              int64  `json:"gclifetime"`
	Maxlifetime             int64  `json:"maxLifetime"`
	DisableHTTPOnly         bool   `json:"disableHTTPOnly"`
	Secure                  bool   `json:"secure"`
	CookieLifeTime          int    `json:"cookieLifeTime"`
	ProviderConfig          string `json:"providerConfig"`
	Domain                  string `json:"domain"`
	SessionIDLength         int64  `json:"sessionIDLength"`
	EnableSidInHTTPHeader   bool   `json:"EnableSidInHTTPHeader"`
	SessionNameInHTTPHeader string `json:"SessionNameInHTTPHeader"`
	EnableSidInURLQuery     bool   `json:"EnableSidInURLQuery"`
	SessionIDPrefix         string `json:"sessionIDPrefix"`
}

// NewManager Create new Manager with provider name and json config string.
// provider name:
// 1. cookie
// 2. file
// 3. memory
// 4. redis
// 5. mysql
// json config:
// 1. is https  default false
// 2. hashfunc  default sha1
// 3. hashkey default beegosessionkey
// 4. maxage default is none
func NewManager(cf *CookieConfig) (*Manager, error) {
	provider := cookieProvide

	if cf.Maxlifetime == 0 {
		cf.Maxlifetime = cf.Gclifetime
	}

	if cf.EnableSidInHTTPHeader {
		if cf.SessionNameInHTTPHeader == "" {
			panic(errors.New("SessionNameInHTTPHeader is empty"))
		}

		strMimeHeader := textproto.CanonicalMIMEHeaderKey(cf.SessionNameInHTTPHeader)
		if cf.SessionNameInHTTPHeader != strMimeHeader {
			strErrMsg := "SessionNameInHTTPHeader ("+ cf.SessionNameInHTTPHeader + ") has the wrong format, expected format: "+ strMimeHeader
			panic(errors.New(strErrMsg))
		}
	}

	err := provider.SessionInit(nil, cf.Maxlifetime, cf.ProviderConfig)
	if err != nil {
		return nil, err
	}

	if cf.SessionIDLength == 0 {
		cf.SessionIDLength = 16
	}

	return &Manager{provider, cf}, nil
}

// GetProvider return current manager's provider
func (manager *Manager) GetProvider() *CookieProvider {
	return manager.provider
}

// getSid retrieves session identifier from HTTP Request
// First try to retrieve id by reading from cookie, session cookie name is configurable,
// if not exist, then retrieve id from querying parameters
//
// error is not null when there is anything wrong
// sid is empty when need to generate a new session id
// otherwise return a  valid session.id
func (manager *Manager) getSid(r *http.Request) (string, error) {
	cookie, errs := r.Cookie(manager.config.CookieName)
	if errs != nil || cookie.Value == "" {
		var sid string
		if manager.config.EnableSidInURLQuery {
			errs := r.ParseForm()
			if errs != nil {
				return "", errs
			}

			sid = r.FormValue(manager.config.CookieName)
		}

		// if not found in Cookie / param, then read it from request headers
		if manager.config.EnableSidInHTTPHeader && sid == "" {
			sids, isFound := r.Header[manager.config.SessionNameInHTTPHeader]
			if isFound && len(sids) != 0 {
				return sids[0], nil
			}
		}

		return sid, nil
	}

	// HTTP Request contains cookie for sessionid info.
	return url.QueryUnescape(cookie.Value)
}

// SessionStart generate or read the session id from http request.
// if session id exists, return SessionStore with this id.
func (manager *Manager) SessionStart(w http.ResponseWriter, r *http.Request) (session *Cookie, err error) {
	sid, errs := manager.getSid(r)
	if errs != nil {
		return &Cookie{}, errs
	}

	if sid != "" {
		return manager.provider.SessionRead(nil, sid)
	}
	// Generate a new session
	sid, errs = manager.sessionID()
	if errs != nil {
		fmt.Println(errs)
		return nil, errs
	}

	session, err = manager.provider.SessionRead(nil, sid)
	if err != nil {
		return nil, err
	}
	cookie := &http.Cookie{
		Name:     manager.config.CookieName,
		Value:    url.QueryEscape(sid),
		Path:     "/",
		HttpOnly: !manager.config.DisableHTTPOnly,
		Secure:   manager.isSecure(r),
		Domain:   manager.config.Domain,
	}
	if manager.config.CookieLifeTime > 0 {
		cookie.MaxAge = manager.config.CookieLifeTime
		cookie.Expires = time.Now().Add(time.Duration(manager.config.CookieLifeTime) * time.Second)
	}
	if manager.config.EnableSetCookie {
		http.SetCookie(w, cookie)
	}
	r.AddCookie(cookie)

	if manager.config.EnableSidInHTTPHeader {
		r.Header.Set(manager.config.SessionNameInHTTPHeader, sid)
		w.Header().Set(manager.config.SessionNameInHTTPHeader, sid)
	}

	return
}

// GC Start session gc process.
// it can do gc in times after gc lifetime.
func (manager *Manager) GC() {
	manager.provider.SessionGC(nil)
	time.AfterFunc(time.Duration(manager.config.Gclifetime)*time.Second, func() { manager.GC() })
}

func (manager *Manager) sessionID() (string, error) {
	return encodeCookie(cookieProvide.block, cookieProvide.config.SecurityKey, cookieProvide.config.SecurityName, make(map[string]interface{}))
}

// Set cookie with https
func (manager *Manager) isSecure(r *http.Request) bool {
	if !manager.config.Secure {
		return false
	}
	if r.URL.Scheme != "" {
		return r.URL.Scheme == "https"
	}
	if r.TLS == nil {
		return false
	}
	return true
}

// SessionRegenerateID Regenerate a session id for this SessionStore who's id is saving in http request.
func (manager *Manager) SessionRegenerateID(w http.ResponseWriter, r *http.Request) (*Cookie, error) {
	sid, err := manager.sessionID()
	if err != nil {
		return &Cookie{}, err
	}

	var session *Cookie

	cookie, err := r.Cookie(manager.config.CookieName)
	if err != nil || cookie.Value == "" {
		// delete old cookie
		session, err = manager.provider.SessionRead(nil, sid)
		if err != nil {
			return nil, err
		}
		cookie = &http.Cookie{Name: manager.config.CookieName,
			Value:    url.QueryEscape(sid),
			Path:     "/",
			HttpOnly: !manager.config.DisableHTTPOnly,
			Secure:   manager.isSecure(r),
			Domain:   manager.config.Domain,
		}
	} else {
		oldsid, err := url.QueryUnescape(cookie.Value)
		if err != nil {
			return nil, err
		}

		session, err = manager.provider.SessionRegenerate(nil, oldsid, sid)
		if err != nil {
			return nil, err
		}

		cookie.Value = url.QueryEscape(sid)
		cookie.HttpOnly = true
		cookie.Path = "/"
	}
	if manager.config.CookieLifeTime > 0 {
		cookie.MaxAge = manager.config.CookieLifeTime
		cookie.Expires = time.Now().Add(time.Duration(manager.config.CookieLifeTime) * time.Second)
	}
	if manager.config.EnableSetCookie {
		http.SetCookie(w, cookie)
	}
	r.AddCookie(cookie)

	if manager.config.EnableSidInHTTPHeader {
		r.Header.Set(manager.config.SessionNameInHTTPHeader, sid)
		w.Header().Set(manager.config.SessionNameInHTTPHeader, sid)
	}

	return session, nil
}

// SessionDestroy Destroy session by its id in http request cookie.
func (manager *Manager) SessionDestroy(w http.ResponseWriter, r *http.Request) {
	if manager.config.EnableSidInHTTPHeader {
		r.Header.Del(manager.config.SessionNameInHTTPHeader)
		w.Header().Del(manager.config.SessionNameInHTTPHeader)
	}

	cookie, err := r.Cookie(manager.config.CookieName)
	if err != nil || cookie.Value == "" {
		return
	}

	sid, _ := url.QueryUnescape(cookie.Value)
	manager.provider.SessionDestroy(nil, sid)
	if manager.config.EnableSetCookie {
		expiration := time.Now()
		cookie = &http.Cookie{Name: manager.config.CookieName,
			Path:     "/",
			HttpOnly: !manager.config.DisableHTTPOnly,
			Expires:  expiration,
			MaxAge:   -1,
			Domain:   manager.config.Domain}

		http.SetCookie(w, cookie)
	}
}