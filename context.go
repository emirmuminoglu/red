package lloyd

import (
	"sync"

	"net/http"
	stdUrl "net/url"

	"github.com/valyala/fasthttp"
)

//Ctx context wrapper of fasthttp.RequestCtx to adds extra funtionality
type Ctx struct {
	noCopy noCopy //nolint:unused,structcheck
	*fasthttp.RequestCtx
	next       bool
	pathName   string
	stdUrl     *stdUrl.URL
	rw         *responseWriter
	deferFuncs []func()
	error      bool
}

var (
	ctxPool sync.Pool
	urlPool sync.Pool
	zeroUrl = &stdUrl.URL{}
)

//AcquireCtx returns an empty Ctx instance from context pool
//
//The returned Ctx instance may be passed to ReleaseCtx when it is no longer needed.
//It is forbidden accessing ctx after releasing it
func AcquireCtx(ctx *fasthttp.RequestCtx) *Ctx {
	v := ctxPool.Get()
	if v == nil {
		lCtx := new(Ctx)
		lCtx.RequestCtx = ctx
		return lCtx
	}

	lCtx := v.(*Ctx)
	lCtx.RequestCtx = ctx

	return lCtx
}

//ReleaseCtx returns ctx acquired via AcquireCtx to context pool
//
//It is forbidden accessing ctx after releasing it
func ReleaseCtx(ctx *Ctx) {
	ReleaseURL(ctx.stdUrl)
	relRespWriter(ctx.rw)

	ctx.next = false

	ctxPool.Put(ctx)
	return
}

//AcquirURL returns an url instance from pool
//
//The returned URL may be passed to ReleaseURL when it is no longer needed.
//It is forbidden accessing url after releasing it
func AcquireURL(uri *fasthttp.URI) *stdUrl.URL {
	v := urlPool.Get()
	if v == nil {
		url := new(stdUrl.URL)

		url.Scheme = B2S(uri.Scheme())
		url.Path = B2S(uri.Path())
		url.Host = B2S(uri.Host())
		url.RawQuery = B2S(uri.QueryString())

		return url
	}

	url := v.(*stdUrl.URL)

	url.Scheme = B2S(uri.Scheme())
	url.Path = B2S(uri.Path())
	url.Host = B2S(uri.Host())
	url.RawQuery = B2S(uri.QueryString())
	return url
}

//ReleaseURL returns URL acquired via AcquireURL to pool.
//
//It is forbidden accessing url after releasing it
func ReleaseURL(url *stdUrl.URL) {
	if url == nil {
		return
	}

	*url = *zeroUrl

	urlPool.Put(url)
}

//When next used, the next handler will be executed after the current handler's execution.
func (ctx *Ctx) Next() {
	ctx.next = true

	return
}

func (ctx *Ctx) PathName() string {
	return ctx.pathName
}

//URL returns the net.URL instance associated with Ctx
func (ctx *Ctx) URL() *stdUrl.URL {
	if ctx.stdUrl == nil {
		ctx.stdUrl = AcquireURL(ctx.Request.URI())
	}

	return ctx.stdUrl
}

//Defer appends given function to defer functions list
//
//defer functions will be executed after defer middlewares's executions.
func (ctx *Ctx) Defer(deferFunc func()) {
	ctx.deferFuncs = append(ctx.deferFuncs, deferFunc)
}

//RequestID returns the request id associated with Ctx
func (ctx *Ctx) RequestID() []byte {
	return ctx.Request.Header.Peek(XRequestIDHeader)
}

//ResponseWriter returns the http.ResponseWriter instance associated with Ctx
func (ctx *Ctx) ResponseWriter() http.ResponseWriter {
	if ctx.rw != nil {
		return ctx.rw
	}

	rw := acqRespWriter()

	rw.ctx = ctx
	rw.hdr = http.Header{}

	ctx.Response.Header.VisitAll(func(k, v []byte) {
		sk := B2S(k)
		sv := B2S(v)

		rw.hdr.Set(sk, sv)
	})

	ctx.rw = rw

	return rw
}
