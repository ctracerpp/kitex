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

// Package discovery defines interfaces for service discovery.
// Developers that are willing to customize service discovery
// should implement their own Resolver and supply it with the
// option WithResolver at client's creation.
package discovery

import (
	"context"
	"net"

	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/utils"
)

// DefaultWeight is the default weight for an instance.
const DefaultWeight = 10

// Result contains the result of service discovery process.
// Cacheable tells whether the instance list can/should be cached.
// When Cacheable is true, CacheKey can be used to map the instance list in cache.
type Result struct {
	// Cacheable 是否可以缓存
	Cacheable bool
	// CacheKey 缓存key instance
	CacheKey  string
	// Instances 多实例
	Instances []Instance
}

// Change contains the difference between the current discovery result and the previous one.
// Change 包含当前发现结果和上一个结果的不同
// It is designed for providing detail information when dispatching an event for service
// 设计师为了提供服务分发的细节信息
// discovery result change.
// 发现结果的改变
// Since the loadbalancer may rely on caching the result of resolver to improve performance,
// 因负载均衡器可能依赖缓存解决结果用来提升性能
// the resolver implementation should dispatch an event when result changes.
// 解决器实现应该派发结果改变事件
type Change struct {
	// Result 结果
	Result  Result
	// Added 被增加的实例
	Added   []Instance
	// Updated 被更新的实例
	Updated []Instance
	// Removed 被删除的实例
	Removed []Instance
}

// Resolver resolves the target endpoint into a list of Instance. 解析器 解析目标终端放入实例列表
type Resolver interface {
	// Target should return a description for the given target that is suitable for being a key for cache.
	Target(ctx context.Context, target rpcinfo.EndpointInfo) (description string)

	// Resolve returns a list of instances for the given description of a target.
	Resolve(ctx context.Context, desc string) (Result, error)

	// Diff computes the difference between two results.
	// When `next` is cacheable, the Change should be cacheable, too. And the `Result` field's CacheKey in
	// the return value should be set with the given cacheKey.
	Diff(cacheKey string, prev, next Result) (Change, bool)

	// Name returns the name of the resolver.
	Name() string
}

// DefaultDiff provides a natural implementation for the Diff method of the Resolver interface.
// 默认差异提供一个通用实现：解决其接口的差异方法
func DefaultDiff(cacheKey string, prev, next Result) (Change, bool) {
	ch := Change{
		Result: Result{
			Cacheable: next.Cacheable,
			CacheKey:  cacheKey,
			Instances: next.Instances,
		},
	}

	prevMap := make(map[string]struct{}, len(prev.Instances))
	for _, ins := range prev.Instances {
		prevMap[ins.Address().String()] = struct{}{}
	}

	nextMap := make(map[string]struct{}, len(next.Instances))
	//查找改变：增加的实例
	for _, ins := range next.Instances {
		addr := ins.Address().String()
		nextMap[addr] = struct{}{}
		if _, found := prevMap[addr]; !found {
			ch.Added = append(ch.Added, ins)
		}
	}
	//查找改变：删除的实例
	for _, ins := range prev.Instances {
		if _, found := nextMap[ins.Address().String()]; !found {
			ch.Removed = append(ch.Removed, ins)
		}
	}
	return ch, len(ch.Added)+len(ch.Removed) != 0
}

// instance 实例（发现中心-> 服务发现）
type instance struct {
	// addr 地址
	addr   net.Addr
	// weight 权重
	weight int
	// tags 标志
	tags   map[string]string
}

func (i *instance) Address() net.Addr {
	return i.addr
}

func (i *instance) Weight() int {
	return i.weight
}

func (i *instance) Tag(key string) (value string, exist bool) {
	value, exist = i.tags[key]
	return
}

// NewInstance creates a Instance using the given network, address and tags
func NewInstance(network, address string, weight int, tags map[string]string) Instance {
	return &instance{
		addr:   utils.NewNetAddr(network, address),
		weight: weight,
		tags:   tags,
	}
}

// SynthesizedResolver synthesizes a Resolver using a resolve function. 合成解析器
type SynthesizedResolver struct {
	// 目标函数
	// ctx 上下文
	// target 目标端点信息
	TargetFunc  func(ctx context.Context, target rpcinfo.EndpointInfo) string
	// 解决函数
	ResolveFunc func(ctx context.Context, key string) (Result, error)
	// 差异函数
	DiffFunc    func(key string, prev, next Result) (Change, bool)
	// 命名函数
	NameFunc    func() string
}

// Target implements the Resolver interface.
func (sr SynthesizedResolver) Target(ctx context.Context, target rpcinfo.EndpointInfo) string {
	if sr.TargetFunc == nil {
		return ""
	}
	return sr.TargetFunc(ctx, target)
}

// Resolve implements the Resolver interface.
func (sr SynthesizedResolver) Resolve(ctx context.Context, key string) (Result, error) {
	return sr.ResolveFunc(ctx, key)
}

// Diff implements the Resolver interface.
func (sr SynthesizedResolver) Diff(key string, prev, next Result) (Change, bool) {
	if sr.DiffFunc == nil {
		return DefaultDiff(key, prev, next)
	}
	return sr.DiffFunc(key, prev, next)
}

// Name implements the Resolver interface
func (sr SynthesizedResolver) Name() string {
	if sr.NameFunc == nil {
		return ""
	}
	return sr.NameFunc()
}

// Instance contains information of an instance from the target service.
type Instance interface {
	Address() net.Addr
	Weight() int
	Tag(key string) (value string, exist bool)
}
