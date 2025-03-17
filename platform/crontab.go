// Copyright (c) 2022-2024 Winlin
//
// SPDX-License-Identifier: MIT
package main

import (
	"context"
	"sync"
	"time"

	"github.com/ossrs/go-oryx-lib/errors"
	"github.com/ossrs/go-oryx-lib/logger"
)

var crontabWorker *CrontabWorker

type CrontabWorker struct {
	wg sync.WaitGroup
}

// NewCrontabWorker 创建并返回一个新的CrontabWorker实例。
// 该函数不需要任何参数。
// 返回值是一个指向CrontabWorker的指针，表示新创建的CrontabWorker实例。
func NewCrontabWorker() *CrontabWorker {
	return &CrontabWorker{}
}

func (v *CrontabWorker) Close() error {
	v.wg.Wait()
	return nil
}

// Start 方法用于启动 CrontabWorker 的后台任务。这些任务包括定期刷新缓存、查询最新版本、刷新 SSL 证书以及重新加载证书文件。
// 参数：
// - ctx: context.Context 类型，用于控制任务的生命周期和取消操作。
// 返回值：
// - error: 如果证书管理器初始化失败，则返回错误；否则返回 nil。
func (v *CrontabWorker) Start(ctx context.Context) error {
	// 启动第一个 Goroutine，负责定期刷新快速缓存（fastCache）。
	v.wg.Add(1)
	go func() {
		defer v.wg.Done()

		for {
			if err := fastCache.Refresh(ctx); err != nil {
				logger.Wf(ctx, "crontab: refresh fast cache err %v", err)
			}

			select {
			case <-ctx.Done():
				return
			case <-time.After(3 * time.Second):
			}
		}
	}()

	// 启动第二个 Goroutine，负责在系统启动后的一段时间内开始查询最新版本。
	v.wg.Add(1)
	go func() {
		defer v.wg.Done()

		// Start the crontab when system startup for a while.
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Minute):
		}

		for {
			logger.Tf(ctx, "crontab: start to query latest version")
			if versions, err := queryLatestVersion(ctx); err != nil {
				logger.Wf(ctx, "crontab: ignore err %v", err)
			} else if versions != nil && versions.Latest != "" {
				conf.Versions = *versions
				logger.Tf(ctx, "crontab: query version ok, result is %v", versions.String())
			}

			select {
			case <-ctx.Done():
				return
			case <-time.After(15 * time.Minute):
			}
		}
	}()

	// 启动第三个 Goroutine，负责定期刷新 SSL 证书。
	v.wg.Add(1)
	go func() {
		defer v.wg.Done()

		for {
			logger.Tf(ctx, "crontab: start to refresh ssl cert")
			if err := certManager.refreshSSLCert(ctx); err != nil {
				logger.Wf(ctx, "crontab: ignore err %v", err)
			}

			select {
			case <-ctx.Done():
				return
			case <-time.After(24 * time.Hour):
			}
		}
	}()

	// 初始化证书管理器，如果失败则直接返回错误。
	if err := certManager.Initialize(ctx); err != nil {
		return errors.Wrapf(err, "initialize cert manager")
	}

	// 启动第四个 Goroutine，负责定期重新加载证书文件。
	v.wg.Add(1)
	go func() {
		defer v.wg.Done()

		for {
			logger.Tf(ctx, "crontab: start to refresh certificate file")
			if err := certManager.reloadCertificateFile(ctx); err != nil {
				logger.Wf(ctx, "crontab: ignore err %v", err)
			}

			select {
			case <-ctx.Done():
				return
			case <-certManager.httpCertificateReload: // 监听证书重新加载信号。
			case <-time.After(time.Duration(1*3600) * time.Second): // 每隔 1 小时重新加载一次证书文件。
			}
		}
	}()

	return nil
}
