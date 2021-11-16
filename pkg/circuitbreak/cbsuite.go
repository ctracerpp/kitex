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

package circuitbreak

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/bytedance/gopkg/cloud/circuitbreaker"

	"github.com/cloudwego/kitex/pkg/discovery"
	"github.com/cloudwego/kitex/pkg/endpoint"
	"github.com/cloudwego/kitex/pkg/event"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
)

const (
	serviceCBKey  = "service"
	instanceCBKey = "instance"
	cbConfig      = "cb_config"
)

var defaultCBConfig = CBConfig{Enable: true, ErrRate: 0.5, MinSample: 200}

// GetDefaultCBConfig return defaultConfig of CircuitBreaker.
func GetDefaultCBConfig() CBConfig {
	return defaultCBConfig
}

// CBConfig is policy config of CircuitBreaker. 断路器配置策略
type CBConfig struct {
	// Enable 是否可用
	Enable    bool    `json:"enable"`
	// ErrRate 错误速率
	ErrRate   float64 `json:"err_rate"`
	// MinSample 最小样例
	MinSample int64   `json:"min_sample"`
}

// GenServiceCBKeyFunc to generate circuit breaker key through rpcinfo. 通过rpcinfo 生成一个断路器key
// You can customize the config key according to your config center. 自定义配置key 依照你的配置中心
type GenServiceCBKeyFunc func(ri rpcinfo.RPCInfo) string

// instanceCBConfig 实例断路器配置
type instanceCBConfig struct {
	// CBConfig 断路器配置策略
	CBConfig
	// RWMutex 读写锁
	sync.RWMutex
}

// CBSuite is default wrapper of CircuitBreaker. If you don't have customize policy, you can specify CircuitBreaker
// CBSuite 是默认的断路器的包装
// middlewares like this:
//     cbs := NewCBSuite(GenServiceCBKeyFunc)
//     opts = append(opts, client.WithCircuitBreaker(cbs))
type CBSuite struct {
	// servicePanel 服务看板
	servicePanel    circuitbreaker.Panel
	// serviceControl 服务控制器
	serviceControl  *Control
	// instancePanel 实例看板
	instancePanel   circuitbreaker.Panel
	// instanceControl 实例控制器
	instanceControl *Control

	// genServiceCBKey 生成服务断路器key函数
	genServiceCBKey GenServiceCBKeyFunc
	// map[serviceCBKey]CBConfig 服务配置map
	serviceCBConfig sync.Map // map[serviceCBKey]CBConfig
	//instanceCBConfig 实例断路器配置
	instanceCBConfig instanceCBConfig
	// events 事件队列
	events event.Queue
}

// NewCBSuite to build a new CBSuite. 构建一个新的断路器套件
// Notice: Should NewCBSuite for every client in this version, 注意：这个版本需要为每一个客户端
// because event.Queue and event.Bus are not shared with all clients now. 因为事件和总线 不是被所有的客户端共享
func NewCBSuite(genKey GenServiceCBKeyFunc) *CBSuite {
	s := &CBSuite{genServiceCBKey: genKey}
	return s
}

// ServiceCBMW return a new service level CircuitBreakerMW.
func (s *CBSuite) ServiceCBMW() endpoint.Middleware {
	if s == nil {
		return endpoint.DummyMiddleware
	}
	s.initServiceCB()
	return NewCircuitBreakerMW(*s.serviceControl, s.servicePanel)
}

// InstanceCBMW return a new instance level CircuitBreakerMW.
func (s *CBSuite) InstanceCBMW() endpoint.Middleware {
	if s == nil {
		return endpoint.DummyMiddleware
	}
	s.initInstanceCB()
	return NewCircuitBreakerMW(*s.instanceControl, s.instancePanel)
}

// ServicePanel return return cb Panel of service
func (s *CBSuite) ServicePanel() circuitbreaker.Panel {
	if s.servicePanel == nil {
		s.initServiceCB()
	}
	return s.servicePanel
}

// ServiceControl return cb Control of service
func (s *CBSuite) ServiceControl() *Control {
	if s.serviceControl == nil {
		s.initServiceCB()
	}
	return s.serviceControl
}

// UpdateServiceCBConfig is to update service CircuitBreaker config.
// This func is suggested to be called in remote config module.
func (s *CBSuite) UpdateServiceCBConfig(key string, cfg CBConfig) {
	s.serviceCBConfig.Store(key, cfg)
}

// UpdateInstanceCBConfig is to update instance CircuitBreaker param.
// This func is suggested to be called in remote config module.
func (s *CBSuite) UpdateInstanceCBConfig(cfg CBConfig) {
	s.instanceCBConfig.Lock()
	s.instanceCBConfig.CBConfig = cfg
	s.instanceCBConfig.Unlock()
}

// SetEventBusAndQueue is to make CircuitBreaker relate to event change.
func (s *CBSuite) SetEventBusAndQueue(bus event.Bus, events event.Queue) {
	s.events = events
	if bus != nil {
		bus.Watch(discovery.ChangeEventName, s.discoveryChangeHandler)
	}
}

// Dump is to dump CircuitBreaker info for debug query.
func (s *CBSuite) Dump() interface{} {
	return map[string]interface{}{
		serviceCBKey:  cbDebugInfo(s.servicePanel),
		instanceCBKey: cbDebugInfo(s.instancePanel),
		cbConfig:      s.configInfo(),
	}
}

// Close circuitbreaker.Panel to release associated resources.
func (s *CBSuite) Close() error {
	if s.servicePanel != nil {
		s.servicePanel.Close()
		s.servicePanel = nil
		s.serviceControl = nil
	}
	if s.instancePanel != nil {
		s.instancePanel.Close()
		s.instancePanel = nil
		s.instanceControl = nil
	}
	return nil
}

func (s *CBSuite) initServiceCB() {
	if s.servicePanel != nil && s.serviceControl != nil {
		return
	}
	if s.genServiceCBKey == nil {
		s.genServiceCBKey = RPCInfo2Key
	}
	opts := circuitbreaker.Options{
		ShouldTripWithKey: s.svcTripFunc,
	}
	s.servicePanel, _ = circuitbreaker.NewPanel(s.onServiceStateChange, opts)

	svcKey := func(ctx context.Context, request interface{}) (serviceCBKey string, enabled bool) {
		ri := rpcinfo.GetRPCInfo(ctx)
		serviceCBKey = s.genServiceCBKey(ri)
		cbConfig, _ := s.serviceCBConfig.LoadOrStore(serviceCBKey, defaultCBConfig)
		enabled = cbConfig.(CBConfig).Enable
		return
	}
	s.serviceControl = &Control{
		GetKey:       svcKey,
		GetErrorType: ErrorTypeOnServiceLevel,
		DecorateError: func(ctx context.Context, request interface{}, err error) error {
			return kerrors.ErrServiceCircuitBreak
		},
	}
}

func (s *CBSuite) initInstanceCB() {
	if s.instancePanel != nil && s.instanceControl != nil {
		return
	}
	s.instanceCBConfig = instanceCBConfig{CBConfig: defaultCBConfig}
	opts := circuitbreaker.Options{
		ShouldTripWithKey: s.insTripFunc,
	}
	s.instancePanel, _ = circuitbreaker.NewPanel(s.onInstanceStateChange, opts)

	instanceKey := func(ctx context.Context, request interface{}) (instCBKey string, enabled bool) {
		ri := rpcinfo.GetRPCInfo(ctx)
		instCBKey = ri.To().Address().String()
		s.instanceCBConfig.RLock()
		enabled = s.instanceCBConfig.Enable
		s.instanceCBConfig.RUnlock()
		return
	}
	s.instanceControl = &Control{
		GetKey:       instanceKey,
		GetErrorType: ErrorTypeOnInstanceLevel,
		DecorateError: func(ctx context.Context, request interface{}, err error) error {
			return kerrors.ErrInstanceCircuitBreak
		},
	}
}

func (s *CBSuite) onStateChange(level, key string, oldState, newState circuitbreaker.State, m circuitbreaker.Metricer) {
	if s.events == nil {
		return
	}
	successes, failures, timeouts := m.Counts()
	var errRate float64
	if sum := successes + failures + timeouts; sum > 0 {
		errRate = float64(failures+timeouts) / float64(sum)
	}
	s.events.Push(&event.Event{
		Name: level + "_cb",
		Time: time.Now(),
		Detail: fmt.Sprintf("%s: %s -> %s, (succ: %d, err: %d, timeout: %d, rate: %f)",
			key, oldState, newState, successes, failures, timeouts, errRate),
	})
}

func (s *CBSuite) onServiceStateChange(key string, oldState, newState circuitbreaker.State, m circuitbreaker.Metricer) {
	s.onStateChange(serviceCBKey, key, oldState, newState, m)
}

func (s *CBSuite) onInstanceStateChange(key string, oldState, newState circuitbreaker.State, m circuitbreaker.Metricer) {
	s.onStateChange(instanceCBKey, key, oldState, newState, m)
}

func (s *CBSuite) discoveryChangeHandler(e *event.Event) {
	if s.instancePanel == nil {
		return
	}
	extra := e.Extra.(*discovery.Change)
	for i := range extra.Removed {
		instCBKey := extra.Removed[i].Address().String()
		s.instancePanel.RemoveBreaker(instCBKey)
	}
}

func (s *CBSuite) svcTripFunc(key string) circuitbreaker.TripFunc {
	pi, _ := s.serviceCBConfig.LoadOrStore(key, defaultCBConfig)
	p := pi.(CBConfig)
	return circuitbreaker.RateTripFunc(p.ErrRate, p.MinSample)
}

func (s *CBSuite) insTripFunc(key string) circuitbreaker.TripFunc {
	s.instanceCBConfig.RLock()
	errRate := s.instanceCBConfig.ErrRate
	minSample := s.instanceCBConfig.MinSample
	s.instanceCBConfig.RUnlock()
	return circuitbreaker.RateTripFunc(errRate, minSample)
}

func cbDebugInfo(panel circuitbreaker.Panel) map[string]interface{} {
	dumper, ok := panel.(interface {
		DumpBreakers() map[string]circuitbreaker.Breaker
	})
	if !ok {
		return nil
	}
	cbMap := make(map[string]interface{})
	for key, breaker := range dumper.DumpBreakers() {
		cbState := breaker.State()
		if cbState == circuitbreaker.Closed {
			continue
		}
		cbMap[key] = map[string]interface{}{
			"state":             cbState,
			"successes in 10s":  breaker.Metricer().Successes(),
			"failures in 10s":   breaker.Metricer().Failures(),
			"timeouts in 10s":   breaker.Metricer().Timeouts(),
			"error rate in 10s": breaker.Metricer().ErrorRate(),
		}
	}
	if len(cbMap) == 0 {
		cbMap["msg"] = "all circuit breakers are in closed state"
	}
	return cbMap
}

func (s *CBSuite) configInfo() map[string]interface{} {
	svcCBMap := make(map[string]interface{})
	s.serviceCBConfig.Range(func(key, value interface{}) bool {
		svcCBMap[key.(string)] = value
		return true
	})
	s.instanceCBConfig.RLock()
	instCBConfig := s.instanceCBConfig.CBConfig
	s.instanceCBConfig.RUnlock()

	cbMap := make(map[string]interface{}, 2)
	cbMap[serviceCBKey] = svcCBMap
	cbMap[instanceCBKey] = instCBConfig
	return cbMap
}

// RPCInfo2Key is to generate circuit breaker key through rpcinfo
func RPCInfo2Key(ri rpcinfo.RPCInfo) string {
	if ri == nil {
		return ""
	}
	fromService := ri.From().ServiceName()
	toService := ri.To().ServiceName()
	method := ri.To().Method()

	sum := len(fromService) + len(toService) + len(method) + 2
	var buf strings.Builder
	buf.Grow(sum)
	buf.WriteString(fromService)
	buf.WriteByte('/')
	buf.WriteString(toService)
	buf.WriteByte('/')
	buf.WriteString(method)
	return buf.String()
}
