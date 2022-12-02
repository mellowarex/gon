package gon

import (
	"os"
	"path"
	"path/filepath"
	"strings"
	"errors"
	"net/http"
	"strconv"
	"time"
	"bytes"
	"sync"

	"github.com/mellowarex/gon/logs"
	"github.com/mellowarex/gon/context"

	lru "github.com/hashicorp/golang-lru"
)

var errNotStaticRequest = errors.New("request not a static file request")
// find static file
// if file is not found do nothing
// controller will respond with 404 not found
func serveStaticRoutes(ctx *context.Context) {
	// dont look for static files if request is not GET|HEAD
	if ctx.Input.Method() != "GET" && ctx.Input.Method() != "HEAD" {
		return
	}

	forbidden, filepath, fileInfo, err := lookupFile(ctx)
	if err == errNotStaticRequest {
		return
	}

	// forbidden request looking for files
	// in undesignated areas
	if forbidden {
		exception("403", ctx)
		return
	}

	if filepath == "" || fileInfo == nil {
		if GConfig.EnvMode == DEV {
			logs.Warn("Cant find/open file: ", filepath, err)
		}
		http.NotFound(ctx.ResponseWriter, ctx.Request)
		return
	}

	if fileInfo.IsDir() {
		requestURL := ctx.Input.URL()
		if requestURL[len(requestURL)-1] != '/' {
			redirectURL := requestURL + "/"
			if ctx.Request.URL.RawQuery != "" {
				redirectURL = redirectURL + "?" + ctx.Request.URL.RawQuery
			}
			ctx.Redirect(302, redirectURL)
		} else {
			// serveFile will list dir
			http.ServeFile(ctx.ResponseWriter, ctx.Request, filepath)
		}
		return
	} else if fileInfo.Size() > int64(GConfig.StaticCacheFileSize) {
		// over size file serve with http module
		http.ServeFile(ctx.ResponseWriter, ctx.Request, filepath)
		return
	}

	var enableCompress = GConfig.EnableGzip && isStaticCompress(filepath)
	var acceptEncoding string
	if enableCompress {
		acceptEncoding = context.ParseEncoding(ctx.Request)
	}
	b, n, sch, reader, err := openFile(filepath, fileInfo, acceptEncoding)
	if err != nil {
		if GConfig.EnvMode == DEV {
			logs.Warn("Can't compress the file:", filepath, err)
		}
		http.NotFound(ctx.ResponseWriter, ctx.Request)
		return
	}

	if b {
		ctx.Output.Header("Content-Encoding", n)
	} else {
		ctx.Output.Header("Content-Length", strconv.FormatInt(sch.size, 10))
	}

	http.ServeContent(ctx.ResponseWriter, ctx.Request, filepath, sch.modTime, reader)
}

// lookupFile find file to serve
// if file is dir, search index.html as default file (MUST NOT BE A DIR also)
// if index.html does not exist or is a dir, return forbidden response depending on DirectoryIndex
func lookupFile(ctx *context.Context) (bool, string, os.FileInfo, error) {
	filePath, fileInfo, err := searchFile(ctx)	
	if filePath == "" || fileInfo == nil {
		return false, "", nil, err
	}
	if !fileInfo.IsDir() {
		return false, filePath, fileInfo, err
	}
	if requestURL := ctx.Input.URL(); requestURL[len(requestURL)-1] == '/' {
		ifp := filepath.Join(filePath, "index.html")
		if ifi, _ := os.Stat(ifp); ifi != nil && ifi.Mode().IsRegular() {
			return false, ifp, ifi, err
		}
	}
	return !GConfig.DirectoryIndex, filePath, fileInfo, err
}


// searchFile search the file by url path
// if no match is found return not staticRequestErr
func searchFile(ctx *context.Context) (string, os.FileInfo, error) {
	requestPath := filepath.ToSlash(filepath.Clean(ctx.Request.URL.Path))
	// special processing: favicon.ico|robots.txt 
	// look for them only in /public folder of webapp
	if requestPath == "/favicon.ico" || requestPath == "/robots.txt" {
		filepath := path.Join("public", requestPath[1:])
		if fi, _ := os.Stat(filepath); fi != nil {
			return filepath, fi, nil
		}
		return "", nil, nil
	}

	for prefix, staticDir :=  range GConfig.StaticDir {
		if !strings.Contains(requestPath, prefix) {
			continue
		}
		if prefix != "/" && len(requestPath) > len(prefix) && requestPath[len(prefix)] != '/' {
			continue
		}
		filePath := path.Join(staticDir, requestPath[len(prefix):]) // replace prefix with true server directory file
		if fi, err := os.Stat(filePath); fi != nil {
			return filePath, fi, err
		}
	}
	return "", nil, errNotStaticRequest
}

type serveContentHolder struct {
	data       []byte
	modTime    time.Time
	size       int64
	originSize int64 // original file size:to judge file changed
	encoding   string
}

type serveContentReader struct {
	*bytes.Reader
}

var (
	staticFileLruCache *lru.Cache
	lruLock            sync.RWMutex
)

func openFile(filePath string, fi os.FileInfo, acceptEncoding string) (bool, string, *serveContentHolder, *serveContentReader, error) {
	if staticFileLruCache == nil {
		// avoid lru cache error
		if GConfig.StaticCacheFileNum >= 1 {
			staticFileLruCache, _ = lru.New(GConfig.StaticCacheFileNum)
		} else {
			staticFileLruCache, _ = lru.New(1)
		}
	}
	mapKey := acceptEncoding + ":" + filePath
	lruLock.RLock()
	var mapFile *serveContentHolder
	if cacheItem, ok := staticFileLruCache.Get(mapKey); ok {
		mapFile = cacheItem.(*serveContentHolder)
	}
	lruLock.RUnlock()
	if isOk(mapFile, fi) {
		reader := &serveContentReader{Reader: bytes.NewReader(mapFile.data)}
		return mapFile.encoding != "", mapFile.encoding, mapFile, reader, nil
	}
	lruLock.Lock()
	defer lruLock.Unlock()
	if cacheItem, ok := staticFileLruCache.Get(mapKey); ok {
		mapFile = cacheItem.(*serveContentHolder)
	}
	if !isOk(mapFile, fi) {
		file, err := os.Open(filePath)
		if err != nil {
			return false, "", nil, nil, err
		}
		defer file.Close()
		var bufferWriter bytes.Buffer
		_, n, err := context.WriteFile(acceptEncoding, &bufferWriter, file)
		if err != nil {
			return false, "", nil, nil, err
		}
		mapFile = &serveContentHolder{data: bufferWriter.Bytes(), modTime: fi.ModTime(), size: int64(bufferWriter.Len()), originSize: fi.Size(), encoding: n}
		if isOk(mapFile, fi) {
			staticFileLruCache.Add(mapKey, mapFile)
		}
	}

	reader := &serveContentReader{Reader: bytes.NewReader(mapFile.data)}
	return mapFile.encoding != "", mapFile.encoding, mapFile, reader, nil
}

func isOk(s *serveContentHolder, fi os.FileInfo) bool {
	if s == nil {
		return false
	} else if s.size > int64(GConfig.StaticCacheFileSize) {
		return false
	}
	return s.modTime == fi.ModTime() && s.originSize == fi.Size()
}

// isStaticCompress detect static files
func isStaticCompress(filePath string) bool {
	for _, statExtension := range GConfig.StaticExtensionsToGzip {
		if strings.HasSuffix(strings.ToLower(filePath), strings.ToLower(statExtension)) {
			return true
		}
	}
	return false
}