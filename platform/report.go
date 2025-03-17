// Copyright (c) 2022-2024 Winlin
//
// SPDX-License-Identifier: MIT
package main

import (
	"context"
)

// queryLatestVersion is to query the latest and stable version from Oryx API.
// queryLatestVersion 查询最新的版本信息。
// 该函数没有输入参数，但返回一个指向Versions类型的指针和一个错误值。
// Versions类型包含三个版本号：Version, Stable和Latest。
// 该函数主要用于获取当前的版本信息，包括稳定的版本号和最新的版本号。
// 注意：该函数的实现可能依赖于全局变量version，这在函数签名中并未明确。
func queryLatestVersion(ctx context.Context) (*Versions, error) {
	// 返回一个Versions类型的指针，其中包含了当前的版本号、稳定的版本号和最新的版本号。
	// 这里硬编码了Stable和Latest的版本号，这可能不是最佳实践，因为它限制了函数的灵活性和可维护性。
	// 一个改进的实现可能从外部源动态获取这些版本号，例如从一个配置文件或远程API。
	return &Versions{
		Version: version,
		Stable:  "v1.0.193",
		Latest:  "v1.0.307",
	}, nil
}
