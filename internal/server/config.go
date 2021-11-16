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

package server

import (
	"net"
	"time"

	"github.com/cloudwego/kitex/pkg/utils"
)

const (
	// defaultExitWaitTime 默认退出等待时间
	defaultExitWaitTime          = 5 * time.Second
	// defaultAcceptFailedDelayTime 默认接收失败延迟时间
	defaultAcceptFailedDelayTime = 10 * time.Millisecond
	//defaultConnectionIdleTime 默认连接空闲时间
	defaultConnectionIdleTime    = 10 * time.Minute
)

// defaultAddress 默认地址本机8888端口
var defaultAddress = utils.NewNetAddr("tcp", ":8888")

// Config contains some server-side configuration. Config 包含了一些服务端的配置
type Config struct {
	Address net.Addr

	// Duration that server waits for to allow any existing connection to be closed gracefully.
	ExitWaitTime time.Duration

	// Duration that server waits for after error occurs during connection accepting.
	AcceptFailedDelayTime time.Duration

	// Duration that the accepted connection waits for to read or write data, only works under NIO.
	MaxConnectionIdleTime time.Duration
}

// NewConfig creates a new default config. 创建一个默认配置
func NewConfig() *Config {
	return &Config{
		Address:               defaultAddress,
		ExitWaitTime:          defaultExitWaitTime,
		AcceptFailedDelayTime: defaultAcceptFailedDelayTime,
		MaxConnectionIdleTime: defaultConnectionIdleTime,
	}
}
