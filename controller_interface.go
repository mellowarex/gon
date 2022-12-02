package gon

import "github.com/mellocraft/gon/context"

type ControllerInterface interface {
	Init(ctrl *context.Context, listen Listen)
	BeforeAction()
	ReadFlashData()

	Get()
	Post()
	Delete()
	Put()
	Head()
	Patch()
	Options()
	Trace()

	Render() error
	XSRFToken() string
	CheckXSRFCookie() bool
	ControllerFunc(fn string) bool
	URLMapping()

	AfterAction()
}