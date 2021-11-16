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

package remote

import (
	"context"
	"net"
	"time"

	internal_stats "github.com/cloudwego/kitex/internal/stats"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
)

// Option is used to pack the inbound and outbound handlers.
type Option struct {
	Outbounds []OutboundHandler

	Inbounds []InboundHandler

	StreamingMetaHandlers []StreamingMetaHandler
}

// ServerOption contains option that is used to init the remote server. 服务配置包含一个用于初始化远程服务的配置
type ServerOption struct {
	// SvcInfo 服务信息
	SvcInfo *serviceinfo.ServiceInfo
	// TransServerFactory 传输服务工厂
	TransServerFactory TransServerFactory
	// SvrHandlerFactory  ServerTransHandlerFactory to new TransHandler for server
	SvrHandlerFactory ServerTransHandlerFactory
	// Codec 编码器
	Codec Codec
	// PayloadCodec 所携带的有效数据编码器
	PayloadCodec PayloadCodec
	// Address 网络地址
	Address net.Addr

	// Duration that server waits for to allow any existing connection to be closed gracefully.
	ExitWaitTime time.Duration

	// Duration that server waits for after error occurs during connection accepting.
	AcceptFailedDelayTime time.Duration

	// Duration that the accepted connection waits for to read or write data.
	MaxConnectionIdleTime time.Duration
	// ReadWriteTimeout 读写超时时间
	ReadWriteTimeout time.Duration

	InitRPCInfoFunc func(context.Context, net.Addr) (rpcinfo.RPCInfo, context.Context)
	// TracerCtl 追踪控制
	TracerCtl *internal_stats.Controller
	// Logger 日志
	Logger klog.FormatLogger
	// Option 继承配置
	Option
}

// ClientOption is used to init the remote client. 客户端配置用于初始化远程客户端
type ClientOption struct {
	// SvcInfo 服务信息
	SvcInfo *serviceinfo.ServiceInfo
	// CliHandlerFactory 客户端传输工厂
	CliHandlerFactory ClientTransHandlerFactory
	// Codec 编码器
	Codec Codec
	// PayloadCodec 携带数据编码器
	PayloadCodec PayloadCodec
	// ConnPool 链接池
	ConnPool ConnPool
	// Dialer 拨号
	Dialer Dialer
	// Logger 日志
	Logger klog.FormatLogger
	// Option 配置
	Option
	// EnableConnPoolReporter 是否允许连接池报告
	EnableConnPoolReporter bool
}
