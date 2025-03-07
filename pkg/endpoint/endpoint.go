/*
 * Copyright 2021 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package endpoint

import "context"

// Endpoint represent one method for calling from remote. Endpoint 意味着远程调用的一个方法
type Endpoint func(ctx context.Context, req, resp interface{}) (err error)

// Middleware deal with input Endpoint and output Endpoint. Middleware 用于处理输入EndPoint 及 输出 EndPoint
type Middleware func(Endpoint) Endpoint

// MiddlewareBuilder builds a middleware with information from a context. MiddlewareBuilder 生成一个携带上下文信息的middleware
type MiddlewareBuilder func(ctx context.Context) Middleware

// Chain connect middlewares into one middleware. Chain 连接一个middleware到另外一个middleware
func Chain(mws ...Middleware) Middleware {
	return func(next Endpoint) Endpoint {
		for i := len(mws) - 1; i >= 0; i-- {
			next = mws[i](next)
		}
		return next
	}
}

// Build builds the given middlewares into one middleware. 给定的middlewares生成middleware
func Build(mws []Middleware) Middleware {
	if len(mws) == 0 {
		return DummyMiddleware
	}
	return func(next Endpoint) Endpoint {
		return mws[0](Build(mws[1:])(next))
	}
}

// DummyMiddleware is a dummy middleware. 仿造的middleware
func DummyMiddleware(next Endpoint) Endpoint {
	return next
}

// DummyEndpoint is a dummy endpoint.
func DummyEndpoint(ctx context.Context, req, resp interface{}) (err error) {
	return nil
}

type mwCtxKeyType int

// Keys for components attached to the context for a middleware builder.
const (
	CtxEventBusKey mwCtxKeyType = iota
	CtxEventQueueKey
	CtxLoggerKey
)
