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

// Package server defines the Options of server
package server

import (
	"github.com/cloudwego/kitex/internal/configutil"
	internal_stats "github.com/cloudwego/kitex/internal/stats"
	"github.com/cloudwego/kitex/pkg/acl"
	"github.com/cloudwego/kitex/pkg/diagnosis"
	"github.com/cloudwego/kitex/pkg/endpoint"
	"github.com/cloudwego/kitex/pkg/event"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/limit"
	"github.com/cloudwego/kitex/pkg/limiter"
	"github.com/cloudwego/kitex/pkg/proxy"
	"github.com/cloudwego/kitex/pkg/registry"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/codec"
	"github.com/cloudwego/kitex/pkg/remote/codec/protobuf"
	"github.com/cloudwego/kitex/pkg/remote/codec/thrift"
	"github.com/cloudwego/kitex/pkg/remote/trans/detection"
	"github.com/cloudwego/kitex/pkg/remote/trans/netpoll"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/pkg/stats"
	"github.com/cloudwego/kitex/pkg/transmeta"
	"github.com/cloudwego/kitex/pkg/utils"
)

func init() {
	remote.PutPayloadCode(serviceinfo.Thrift, thrift.NewThriftCodec())
	remote.PutPayloadCode(serviceinfo.Protobuf, protobuf.NewProtobufCodec())
}

// Option is the only way to config a server. Option 是配置服务端的唯一方式
type Option struct {
	F func(o *Options, di *utils.Slice)
}

// Options is used to initialize the server. Options 用来初始化服务端
type Options struct {
	// Svr 服务端终端基本信息
	Svr      *rpcinfo.EndpointBasicInfo
	// Configs 包含RPC的配置信息
	Configs  rpcinfo.RPCConfig
	// LockBits 位锁
	LockBits int
	// Once 确定Option 只被设置一次
	Once     *configutil.OptionOnce
	// MetaHandlers 元句柄
	MetaHandlers []remote.MetaHandler
	// RemoteOpt 服务选项
	RemoteOpt *remote.ServerOption
	// ErrHandle 错误句柄
	ErrHandle func(error) error
	// Proxy 反向代理
	Proxy     proxy.BackwardProxy

	// Registry is used for service registry. 用于服务注册
	Registry registry.Registry
	// RegistryInfo is used to in registry. 服务注册信息
	RegistryInfo *registry.Info
	// ACLRules 访问控制策略
	ACLRules      []acl.RejectFunc
	// Limits 限流配置
	Limits        *limit.Option
	// LimitReporter 限流报告
	LimitReporter limiter.LimitReporter
	// MWBs  生成一个携带上下文信息的middleware
	MWBs []endpoint.MiddlewareBuilder
	// Bus 实现了观察者模式允许事件分发及观测
	Bus    event.Bus
	// Events 事件队列
	Events event.Queue

	// DebugInfo should only contains objects that are suitable for json serialization.
	DebugInfo    utils.Slice
	// DebugService 调试服务
	DebugService diagnosis.Service

	// Observability
	// Logger 日志记录
	Logger     klog.FormatLogger
	// TracerCtl 追踪控制
	TracerCtl  *internal_stats.Controller
	// StatsLevel 统计级别
	StatsLevel *stats.Level
}

// NewOptions creates a default options.
func NewOptions(opts []Option) *Options {
	o := &Options{
		Svr:          &rpcinfo.EndpointBasicInfo{},
		Configs:      rpcinfo.NewRPCConfig(),
		Once:         configutil.NewOptionOnce(),
		RemoteOpt:    newServerOption(),
		Logger:       klog.DefaultLogger(),
		DebugService: diagnosis.NoopService,

		Bus:    event.NewEventBus(),
		Events: event.NewQueue(event.MaxEventNum),

		TracerCtl: &internal_stats.Controller{},
		Registry:  registry.NoopRegistry,
	}
	ApplyOptions(opts, o)
	o.MetaHandlers = append(o.MetaHandlers, transmeta.MetainfoServerHandler)
	rpcinfo.AsMutableRPCConfig(o.Configs).LockConfig(o.LockBits)
	if o.StatsLevel == nil {
		level := stats.LevelDisabled
		if o.TracerCtl.HasTracer() {
			level = stats.LevelDetailed
		}
		o.StatsLevel = &level
	}
	return o
}

func newServerOption() *remote.ServerOption {
	return &remote.ServerOption{
		TransServerFactory:    netpoll.NewTransServerFactory(),
		SvrHandlerFactory:     detection.NewSvrTransHandlerFactory(),
		Codec:                 codec.NewDefaultCodec(),
		Address:               defaultAddress,
		ExitWaitTime:          defaultExitWaitTime,
		MaxConnectionIdleTime: defaultConnectionIdleTime,
		AcceptFailedDelayTime: defaultAcceptFailedDelayTime,
	}
}

// ApplyOptions applies the given options.
func ApplyOptions(opts []Option, o *Options) {
	for _, op := range opts {
		op.F(o, &o.DebugInfo)
	}
}
