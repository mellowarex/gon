package gon

import (
	"net/http"
	"reflect"
	"strings"
	"fmt"
	"errors"
)

var (
	// ErrMethodMismatch is returned when the method in the request does not match
	// the method defined against the route.
	ErrMethodMismatch = errors.New("method is not allowed")
	// ErrNotFound is returned when no route match is found.
	ErrNotFound = errors.New("no matching route was found")
	// HTTPMETHOD list the supported http methods.
		HTTPMETHOD = map[string]bool{
			"GET":       true,
			"POST":      true,
			"PUT":       true,
			"DELETE":    true,
			"PATCH":     true,
			"OPTIONS":   true,
			"HEAD":      true,
			"TRACE":     true,
			"CONNECT":   true,
			"MKCOL":     true,
			"COPY":      true,
			"MOVE":      true,
			"PROPFIND":  true,
			"PROPPATCH": true,
			"LOCK":      true,
			"UNLOCK":    true,
		}
	)
// Match attempts to match the given request against the router's registered routes.
//
// If the request matches a route of this router or one of its subrouters the Route,
// Handler, and Vars fields of the the match argument are filled and this function
// returns true.
//
// If the request does not match any of this router's or its subrouters' routes
// then this function returns false. If available, a reason for the match failure
// will be filled in the match argument's MatchErr field. If the match failure type
// (eg: not found) has a registered handler, the handler is assigned to the Handler
// field of the match argument.
func (mux *Multiplexer) Match(r *http.Request, match *RouteMatch) bool {
	for _, route := range mux.routes {
		if route.Match(r, match) {
			return true
		}
	}

	// check if http method is not allowed
	if match.MatchErr == ErrMethodMismatch {
		return false
	}
	return false
}

// NewRoute registers empty route
func (mux *Multiplexer) NewRoute() *Route {
	// initialize a route with a copy of parent router's config
	route := &Route{routeConf: copyRouteConf(mux.routeConf), MethodMapping: make(map[string]string)}
	mux.routes = append(mux.routes, route)
	return route
}

// Route registers a new route with matcher for URL path
func (mux *Multiplexer) Route(path string, ctrl ControllerInterface) *Route {
	return mux.NewRoute().Path(path).Controller(ctrl)
}

// Subrouter ------------------------------------------------------------------

// Subrouter creates a subrouter for the route.
//
// It will test the inner routes only if the parent route matched. For example:
//
//     r := mux.NewRouter()
//     s := r.Host("www.example.com").Subrouter()
//     s.Route("/products/", ProductsHandler)
//     s.Route("/products/{key}", ProductHandler)
//     s.Route("/articles/{category}/{id:[0-9]+}"), ArticleHandler)
//
// Here, the routes registered in the subrouter won't be tested if the host
// doesn't match.
func (r *Route) Subrouter() *Multiplexer {
	// initialize a subrouter with a copy of the parent route's configuration
	router := &Multiplexer{routeConf: copyRouteConf(r.routeConf)}
	r.addMatcher(router)
	return router
}

// Host registers a new route with a matcher for the URL host.
// See Route.Host().
func (r *Multiplexer) Host(tpl string) *Route {
	return r.NewRoute().Host(tpl)
}

// Queries registers a new route with a matcher for URL query values.
// See Route.Queries().
func (r *Multiplexer) Queries(pairs ...string) *Route {
	return r.NewRoute().Queries(pairs...)
}

// PathPrefix registers a new route with a matcher for the URL path prefix.
// See Route.PathPrefix().
func (r *Multiplexer) PathPrefix(tpl string) *Route {
	return r.NewRoute().PathPrefix(tpl)
}

// Controller controller handler and pattern rules to ControllerRegister.
// usage:
//	default methods is the same name as method
//	Add("/user",&UserController{})
//	Add("/api/list",&RestController{},"*:ListFood")
//	Add("/api/create",&RestController{},"post:CreateFood")
//	Add("/api/update",&RestController{},"put:UpdateFood")
//	Add("/api/delete",&RestController{},"delete:DeleteFood")
//	Add("/api",&RestController{},"get,post:ApiFunc"
//	Add("/simple",&SimpleController{},"get:GetFunc;post:PostFunc")
func (mux *Multiplexer) Router(path string, ctrl ControllerInterface, mappingMethods string) *Route {
	reflectVal := reflect.ValueOf(ctrl)
	t := reflect.Indirect(reflectVal).Type()
	methods := make(map[string]string)
	if len(mappingMethods) > 0 {
		semi := strings.Split(mappingMethods, ";")
		for _, v := range semi {
			colon := strings.Split(v, ":")
			if len(colon) != 2 {
				panic("method mapping format is invalid")
			}
			comma := strings.Split(colon[0], ",")
			for _, m := range comma {
				if m == "*" || HTTPMETHOD[strings.ToUpper(m)] {
					if val := reflectVal.MethodByName(colon[1]); val.IsValid() {
						methods[strings.ToUpper(m)] = colon[1]
					} else {
						panic("'" + colon[1] + "' method doesn't exist in the controller " + t.Name())
					}
				} else {
					panic(v + " is an invalid method mapping. Method doesn't exist " + m)
				}
			}
		}
	}

	return mux.MapController(path, ctrl, methods)
}

func (mux *Multiplexer) MapController(path string, ctrl ControllerInterface, methodMapping map[string]string) *Route {
	route := mux.NewRoute().Path(path).Control(ctrl)
	route.Mapped = true
	route.MethodMapping = methodMapping
	return route
}

// ----------------------------------------------------------------------------
// Context
// ----------------------------------------------------------------------------

// RouteMatch stores information about a matched route.
type RouteMatch struct {
	Route      *Route
	Handler    http.Handler
	Controller ControllerInterface
	Vars       map[string]string

	// MatchErr is set to appropriate matching error
	// It is set to ErrMethodMismatch if there is a mismatch in
	// the request method and route method
	MatchErr error
}

// matchInArray returns true if the given string value is in the array.
func matchInArray(arr []string, value string) bool {
	for _, v := range arr {
		if v == value {
			return true
		}
	}
	return false
}

// ----------------------------------------------------------------------------
// routeRegexpGroup
// ----------------------------------------------------------------------------

// routeRegexpGroup groups the route matchers that carry variables.
type routeRegexpGroup struct {
	host    *routeRegexp
	path    *routeRegexp
	queries []*routeRegexp
}

// returns an effective deep copy of `routeConf`
func copyRouteConf(r routeConf) routeConf {
	c := r

	if r.regexp.path != nil {
		c.regexp.path = copyRouteRegexp(r.regexp.path)
	}

	if r.regexp.host != nil {
		c.regexp.host = copyRouteRegexp(r.regexp.host)
	}

	c.regexp.queries = make([]*routeRegexp, 0, len(r.regexp.queries))
	for _, q := range r.regexp.queries {
		c.regexp.queries = append(c.regexp.queries, copyRouteRegexp(q))
	}

	c.matchers = make([]matcher, len(r.matchers))
	copy(c.matchers, r.matchers)

	return c
}

func copyRouteRegexp(r *routeRegexp) *routeRegexp {
	c := *r
	return &c
}

// uniqueVars returns an error if two slices contain duplicated strings.
func uniqueVars(s1, s2 []string) error {
	for _, v1 := range s1 {
		for _, v2 := range s2 {
			if v1 == v2 {
				return fmt.Errorf("mux: duplicated route variable %q", v2)
			}
		}
	}
	return nil
}