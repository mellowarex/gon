package session

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"github.com/mellowarex/gon/utils"
)

type cookieConfig struct {
	SecurityKey  string `json:"securityKey"`
	BlockKey     string `json:"blockKey"`
	SecurityName string `json:"securityName"`
	CookieName   string `json:"cookieName"`
	Secure       bool   `json:"secure"`
	Maxage       int    `json:"maxage"`
}

// CookieProvider Cookie session provider
type CookieProvider struct {
	maxlifetime int64
	config      *cookieConfig
	block       cipher.Block
}

// SessionInit Init cookie session provider with max lifetime and config json.
// maxlifetime is ignored.
// json config:
// 	securityKey - hash string
// 	blockKey - gob encode hash string. it's saved as aes crypto.
// 	securityName - recognized name in encoded cookie string
// 	cookieName - cookie name
// 	maxage - cookie max life time.
func (pder *CookieProvider) SessionInit(ctx context.Context, maxlifetime int64, configFile string) error {
	pder.config = &cookieConfig{}
	// fmt.Println(configFile)
	err := utils.ParseConfigFile(configFile, pder.config)
	if err != nil {
		return err
	}
	if pder.config.BlockKey == "" {
		pder.config.BlockKey = string(generateRandomKey(16))
	}
	if pder.config.SecurityName == "" {
		pder.config.SecurityName = string(generateRandomKey(20))
	}
	pder.block, err = aes.NewCipher([]byte(pder.config.BlockKey))
	if err != nil {
		return err
	}
	pder.maxlifetime = maxlifetime
	return nil
}

// SessionRead Get SessionStore in cooke.
// decode cooke string to map and put into SessionStore with sid.
func (pder *CookieProvider) SessionRead(ctx context.Context, sid string) (*Cookie, error) {
	maps, _ := decodeCookie(pder.block,
		pder.config.SecurityKey,
		pder.config.SecurityName,
		sid, pder.maxlifetime)
	if maps == nil {
		maps = make(map[interface{}]interface{})
	}
	rs := &Cookie{sid: sid, values: maps}
	return rs, nil
}

// SessionExist Cookie session is always existed
func (pder *CookieProvider) SessionExist(ctx context.Context, sid string) (bool, error) {
	return true, nil
}

// SessionRegenerate Implement method, no used.
func (pder *CookieProvider) SessionRegenerate(ctx context.Context, oldsid, sid string) (*Cookie, error) {
	return &Cookie{}, nil
}

// SessionDestroy Implement method, no used.
func (pder *CookieProvider) SessionDestroy(ctx context.Context, sid string) error {
	return nil
}

// SessionGC Implement method, no used.
func (pder *CookieProvider) SessionGC(context.Context) {
}

// SessionAll Implement method, return 0.
func (pder *CookieProvider) SessionAll(context.Context) int {
	return 0
}

// SessionUpdate Implement method, no used.
func (pder *CookieProvider) SessionUpdate(ctx context.Context, sid string) error {
	return nil
}