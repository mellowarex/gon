package gon

import (
	"sync"
	"net/http"
	"time"
	"fmt"
	"strings"
	"reflect"

	"github.com/mellowarex/gon/context"
	"github.com/mellowarex/gon/logs"
)

type Multiplexer struct {
	routes []*Route

	pool sync.Pool

	routeConf
}

var (
	Mux *Multiplexer
)

func InitMux() *Multiplexer {
	mux := &Multiplexer{
		pool: sync.Pool{
			New: func() interface{} {
				return context.NewContext()
			},
		},
	}
	Mux = mux
	return mux
}

// NewMux return new Mux instance
// call in routes pkg in init func
func NewMux() *Multiplexer {
	return Mux
}

// GetContext returns context from pool
func (this *Multiplexer) GetContext() *context.Context {
	return this.pool.Get().(*context.Context)
}

// PutContext puts ctx back into pool for reuse
func (this *Multiplexer) PutContext(ctx *context.Context) {
	this.pool.Put(ctx)
}

// ServeHTTP dispatches hander registered in matched route
// when match if found, route var can be retrieved by calling
// this.Vars(request)
func (this *Multiplexer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// fmt.Println("routes: ",templar)
	// for _, r := range templar {
	// 	fmt.Println("r: ", r)
	// }
	startTime := time.Now()
	var (
		match RouteMatch
		ctrl ControllerInterface
		err error
		runMethod string
	)

	ctx := this.GetContext()
	ctx.Reset(w, r)
	defer this.PutContext(ctx)
	// defer this.conf.RecoverFunc(ctx, this.conf)

	serveStaticRoutes(ctx)

	if ctx.ResponseWriter.Started {
		goto Logging
	}

	// parseform
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		if ctx.Input.IsUpload() {
			ctx.Input.Context.Request.Body = http.MaxBytesReader(ctx.Input.Context.ResponseWriter,
				ctx.Input.Context.Request.Body, GConfig.MaxUploadSize)
		} else if GConfig.CopyRequestBody {
			// conn will close if incoming data is large
			if r.ContentLength > GConfig.MaxMemory {
				exception("413", ctx)
				goto Logging
			}
			ctx.Input.CopyBody(GConfig.MaxMemory)
		}
		err = ctx.Input.ParseForm(GConfig.MaxUploadSize)
		if err != nil {
			if strings.Contains(err.Error(), "http: request body too large") {
				exception("413", ctx)
			} else {
				exception("500", ctx)
			}
			goto Logging
		}
	}

	runMethod = r.Method

	if this.Match(r, &match) {
		ctrl = match.Controller
		_, ok := match.Route.MethodMapping[runMethod]
		if match.Route.Mapped && ok {
			runMethod = match.Route.MethodMapping[runMethod]
		}
	}

	// assign url param values to context
	ctx.Input.Params = match.Vars
	// print params in link query if present
	if len(match.Vars) > 0 {
		logs.Info("params: ", match.Vars)
	}

	if ctrl == nil {
		exception("404", ctx)
		goto Logging
	}

	// session init
	if GConfig.WebConfig.Session.SessionOn {
		ctx.Input.Cookie, err = GlobalSessions.SessionStart(w, r)
		if err != nil {
			logs.Error(err)
			exception("503", ctx)
			goto Logging
		}
	}

	// call controller init func
	ctrl.Init(ctx, GConfig.Listen) 

	// perform before action
	ctrl.BeforeAction()

	// read flash message from cookie then delete
	ctrl.ReadFlashData()

	// if XSRF is enabled check cookie in request's _csrf
	if GConfig.WebConfig.EnableXSRF {
		ctrl.XSRFToken()
		if r.Method == http.MethodPost || r.Method == http.MethodDelete || r.Method == http.MethodPut ||
			(r.Method == http.MethodPost && (ctx.Input.Query("_method") == http.MethodDelete || ctx.Input.Query("_method") == http.MethodPut)) {
			ctrl.CheckXSRFCookie()
		}
	}
	// execute action
	if !ctx.ResponseWriter.Started {
		switch runMethod {
		case http.MethodGet:
			ctrl.Get()
		case http.MethodPost:
			ctrl.Post()
		case http.MethodDelete:
			ctrl.Delete()
		case http.MethodPut:
			ctrl.Put()
		case http.MethodHead:
			ctrl.Head()
		case http.MethodPatch:
			ctrl.Patch()
		case http.MethodOptions:
			ctrl.Options()
		case http.MethodTrace:
			ctrl.Trace()
		default:
			if !ctrl.ControllerFunc(runMethod) {
				vc := reflect.ValueOf(ctrl)
				method := vc.MethodByName(runMethod)
				if method.IsValid(){
					method.Call(nil)
				}

			}
		}

		if !ctx.ResponseWriter.Started && ctx.Output.Status == 0 {
			if err := ctrl.Render(); err != nil {
				logs.Error(err)
			}
		}
	}

	ctrl.AfterAction()

	goto Logging

	Logging:
		statusCode := ctx.ResponseWriter.Status
		if statusCode == 0 {
			statusCode = 200
		}

		LogAccess(ctx, &startTime, statusCode)

		timeDur := time.Since(startTime)
		traceInfo := fmt.Sprintf("%s %3d %s|%13s|%s %-7s %s %-3s",
			logs.ColorByStatus(statusCode), statusCode, logs.ResetColor(),
			timeDur.String(),
			logs.ColorByMethod(r.Method), r.Method, logs.ResetColor(),r.URL.Path)

		logs.Debug(traceInfo)

		if ctx.Output.Status != 0 {
			ctx.ResponseWriter.WriteHeader(ctx.Output.Status)
		}
}