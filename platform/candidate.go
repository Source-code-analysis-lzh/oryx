// Copyright (c) 2022-2024 Winlin
//
// SPDX-License-Identifier: MIT
package main

import (
	"context"
	"net"
	"strings"
	"sync"
	// From ossrs.
	"github.com/ossrs/go-oryx-lib/logger"
)

var candidateWorker *CandidateWorker

type CandidateWorker struct {
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewCandidateWorker 创建并返回一个新的 CandidateWorker 实例。
// 该函数不需要任何参数。
// 返回值是一个指向 CandidateWorker 结构的指针，表示一个新的候选工作者实例。
func NewCandidateWorker() *CandidateWorker {
	return &CandidateWorker{}
}

func (v *CandidateWorker) Close() error {
	if v.cancel != nil {
		v.cancel()
	}
	v.wg.Wait()
	return nil
}

// Start 方法用于启动一个候选工作者（CandidateWorker）。
// 参数：
//
//	ctx - context.Context 类型，用于控制方法的生命周期，可以用于取消操作。
//
// 返回值：
//
//	error - 如果方法执行过程中出现错误，则返回具体的错误信息；否则返回 nil 表示成功。
func (v *CandidateWorker) Start(ctx context.Context) error {
	// 从传入的 ctx 创建一个新的可取消的上下文，并保存 cancel 函数以便后续停止工作者。
	ctx, cancel := context.WithCancel(ctx)
	v.cancel = cancel

	// 将日志功能与上下文关联，以便在日志中记录上下文信息。
	ctx = logger.WithContext(ctx)
	// 记录日志，表明候选工作者已启动。
	logger.Tf(ctx, "candidate start a worker")

	return nil
}

// Resolve host to ip. Return nil if ignore the host resolving, for example, user disable resolving by
// set env NAME_LOOKUP to false.
// 该函数用于将主机名（host）解析为 IP 地址，支持环境变量控制、本地主机特殊处理、直接 IP 解析和 DNS 查询。
func (v *CandidateWorker) Resolve(host string) (net.IP, error) {
	// Ignore the resolving.
	if envNameLookup() == "off" {
		return nil, nil
	}

	// Ignore the port.
	if strings.Contains(host, ":") {
		if hostname, _, err := net.SplitHostPort(host); err != nil {
			return nil, err
		} else {
			host = hostname
		}
	}

	// Resolve the localhost to possible IP address.
	// 处理 localhost 的特殊逻辑
	if host == "localhost" {
		// If directly run in host, like debugging, use the private ipv4.
		// 如果不在 Docker 环境（envPlatformDocker=off），返回配置的私有 IPv4
		if envPlatformDocker() == "off" {
			return conf.ipv4, nil
		}

		// If already set CANDIDATE, ignore lo.
		// 如果设置了候选地址（CANDIDATE 环境变量），忽略本地解析
		if envCandidate() != "" {
			return nil, nil
		}

		// Return lo for OBS WHIP or native client to access it.
		// 默认返回 127.0.0.1（本地回环地址）
		return net.IPv4(127, 0, 0, 1), nil
	}

	// Directly use the ip if not name.
	if ip := net.ParseIP(host); ip != nil {
		return ip, nil
	}

	// Lookup the name to parse to ip.
	if ips, err := net.LookupIP(host); err != nil {
		return nil, err
	} else if len(ips) > 0 {
		return ips[0], nil
	}

	return nil, nil
}
