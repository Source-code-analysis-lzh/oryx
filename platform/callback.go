// Copyright (c) 2022-2024 Winlin
//
// SPDX-License-Identifier: MIT
package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	// From ossrs.
	"github.com/ossrs/go-oryx-lib/errors"
	ohttp "github.com/ossrs/go-oryx-lib/http"
	"github.com/ossrs/go-oryx-lib/logger"

	// Use v8 because we use Go 1.16+, while v9 requires Go 1.18+
	"github.com/go-redis/redis/v8"
)

var callbackWorker *CallbackWorker

type CallbackWorker struct {
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// The ephemeral callback config.
	ephemeralConfig CallbackConfig
	// Whether update the config immediately.
	updateConfig chan bool

	lock sync.Mutex
}

// NewCallbackWorker 创建并返回一个新的 CallbackWorker 实例。
// 该函数初始化了一个 CallbackWorker 结构体，并为其 updateConfig 字段
// 创建了一个容量为 1 的布尔类型通道。这个通道用于触发配置更新的操作，
// 确保在需要更新配置时，可以有效地通知 CallbackWorker 进行处理。
func NewCallbackWorker() *CallbackWorker {
	return &CallbackWorker{
		updateConfig: make(chan bool, 1),
	}
}

func (v *CallbackWorker) Handle(ctx context.Context, handler *http.ServeMux) error {
	ep := "/terraform/v1/mgmt/hooks/query"
	logger.Tf(ctx, "Handle %v", ep)
	handler.HandleFunc(ep, func(w http.ResponseWriter, r *http.Request) {
		if err := func() error {
			var token string
			if err := ParseBody(ctx, r.Body, &struct {
				Token *string `json:"token"`
				All   *bool   `json:"all"`
			}{
				Token: &token,
			}); err != nil {
				return errors.Wrapf(err, "parse body")
			}

			apiSecret := envApiSecret()
			if err := Authenticate(ctx, apiSecret, token, r.Header); err != nil {
				return errors.Wrapf(err, "authenticate")
			}

			var config CallbackConfig
			if err := config.Load(ctx); err != nil {
				return errors.Wrapf(err, "load")
			}

			req, err := rdb.HGet(ctx, SRS_HOOKS, "req").Result()
			if err != nil && err != redis.Nil {
				return errors.Wrapf(err, "hget %v req", SRS_HOOKS)
			}

			res, err := rdb.HGet(ctx, SRS_HOOKS, "res").Result()
			if err != nil && err != redis.Nil {
				return errors.Wrapf(err, "hget %v res", SRS_HOOKS)
			}

			type HooksQueryResult struct {
				Request  string `json:"req"`
				Response string `json:"res"`
				*CallbackConfig
			}
			ohttp.WriteData(ctx, w, r, &HooksQueryResult{
				Request:        req,
				Response:       res,
				CallbackConfig: &config,
			})
			logger.Tf(ctx, "hooks apply ok, %v, token=%vB", config.String(), len(token))
			return nil
		}(); err != nil {
			ohttp.WriteError(ctx, w, r, err)
		}
	})

	ep = "/terraform/v1/mgmt/hooks/apply"
	logger.Tf(ctx, "Handle %v", ep)
	handler.HandleFunc(ep, func(w http.ResponseWriter, r *http.Request) {
		if err := func() error {
			var token string
			var config CallbackConfig
			if err := ParseBody(ctx, r.Body, &struct {
				Token *string `json:"token"`
				*CallbackConfig
			}{
				Token:          &token,
				CallbackConfig: &config,
			}); err != nil {
				return errors.Wrapf(err, "parse body")
			}

			apiSecret := envApiSecret()
			if err := Authenticate(ctx, apiSecret, token, r.Header); err != nil {
				return errors.Wrapf(err, "authenticate")
			}

			if err := rdb.HSet(ctx, SRS_HOOKS, "target", config.Target).Err(); err != nil && err != redis.Nil {
				return errors.Wrapf(err, "hset %v target %v", SRS_HOOKS, config.Target)
			}
			if err := rdb.HSet(ctx, SRS_HOOKS, "opaque", config.Opaque).Err(); err != nil && err != redis.Nil {
				return errors.Wrapf(err, "hset %v opaque %v", SRS_HOOKS, config.Opaque)
			}
			if err := rdb.HSet(ctx, SRS_HOOKS, "all", fmt.Sprintf("%v", config.All)).Err(); err != nil && err != redis.Nil {
				return errors.Wrapf(err, "hset %v all %v", SRS_HOOKS, config.All)
			}

			// Use the request host as the default host.
			if config.Host == "" {
				config.Host = fmt.Sprintf("http://%v", r.Host)
				if r.TLS != nil {
					config.Host = fmt.Sprintf("https://%v", r.Host)
				}
			}
			if err := rdb.HSet(ctx, SRS_HOOKS, "host", config.Host).Err(); err != nil && err != redis.Nil {
				return errors.Wrapf(err, "hset %v host %v", SRS_HOOKS, config.Host)
			}

			// Notify the callback worker to update the config.
			select {
			case v.updateConfig <- true:
			case <-ctx.Done():
			default:
			}

			ohttp.WriteData(ctx, w, r, nil)
			logger.Tf(ctx, "hooks apply ok, %v, token=%vB", config.String(), len(token))
			return nil
		}(); err != nil {
			ohttp.WriteError(ctx, w, r, err)
		}
	})

	ep = "/terraform/v1/mgmt/hooks/example"
	logger.Tf(ctx, "Handle %v", ep)
	handler.HandleFunc(ep, func(w http.ResponseWriter, r *http.Request) {
		if err := func() error {
			q := r.URL.Query()

			var fail bool
			if value := q.Get("fail"); value == "true" || value == "1" {
				fail = true
			}

			var action, opaque string
			if err := ParseBody(ctx, r.Body, &struct {
				Action *string `json:"action"`
				Opaque *string `json:"opaque"`
			}{
				Action: &action,
				Opaque: &opaque,
			}); err != nil {
				return errors.Wrapf(err, "parse body")
			}

			if fail {
				return errors.Errorf("fail as required, action=%v, opaque=%v", action, opaque)
			}

			ohttp.WriteData(ctx, w, r, nil)
			logger.Tf(ctx, "hooks example ok, action=%v, opaque=%v", action, opaque)
			return nil
		}(); err != nil {
			ohttp.WriteError(ctx, w, r, err)
		}
	})

	return nil
}

// Close 关闭 CallbackWorker。
// 该方法用于优雅地关闭 CallbackWorker，确保所有后台任务都已完成。
// 参数：
//   - 无参数
//
// 返回值：
//   - error: 如果关闭过程中发生错误，则返回非 nil 的错误；否则返回 nil。
func (v *CallbackWorker) Close() error {
	// 如果 cancel 函数不为空，则调用 cancel 函数取消上下文，停止所有子任务。
	if v.cancel != nil {
		v.cancel()
	}
	// 等待 WaitGroup 中的所有 goroutine 完成，确保所有任务都已结束。
	v.wg.Wait()
	// 返回 nil 表示关闭成功。
	return nil
}

// Start 启动 CallbackWorker 并开始处理任务。
// 参数：
//
//	ctx - 控制工作生命周期的上下文。
//
// 返回值：
//
//	error - 如果启动过程中发生错误，则返回非 nil 的错误。
func (v *CallbackWorker) Start(ctx context.Context) error {
	wg := &v.wg // 使用 wg（WaitGroup）来等待后台任务完成。

	// 创建一个可取消的上下文，用于控制子任务的生命周期。
	ctx, cancel := context.WithCancel(ctx)
	v.cancel = cancel // 保存取消函数，以便后续可以停止工作。

	// 将日志上下文附加到当前上下文中，便于记录日志。
	ctx = logger.WithContext(ctx)
	logger.Tf(ctx, "callback start a worker") // 记录日志：启动一个工作线程。

	// 增加 WaitGroup 的计数器，表示有一个新的任务需要等待。
	wg.Add(1)
	go func() {
		defer wg.Done() // 任务完成后减少 WaitGroup 的计数器。

		// 循环运行，直到上下文被取消或发生错误。
		for ctx.Err() == nil {
			var config CallbackConfig                // 定义配置对象。
			if err := config.Load(ctx); err != nil { // 加载配置。
				// 如果加载失败，记录错误日志。
				logger.Wf(ctx, "load config %v err %+v", config, err)

				// 等待上下文被取消或延迟一段时间后重试。
				select {
				case <-ctx.Done():
				case <-time.After(10 * time.Second):
				}
				continue
			}

			// 更新 ephemeralConfig，确保线程安全。
			func() {
				v.lock.Lock()              // 如果上下文被取消，退出循环。
				defer v.lock.Unlock()      // 等待 3 秒后重新检查配置。
				v.ephemeralConfig = config // 如果有更新配置的通知，立即重新加载。
			}()

			// 根据条件选择退出循环或继续运行。
			select {
			case <-ctx.Done():
			case <-time.After(time.Second * 3):
			case <-v.updateConfig:
			}
		}
	}()

	return nil
}

func (v *CallbackWorker) OnStreamMessage(ctx context.Context, action SrsAction, streamObj *SrsStream) error {
	if action != SrsActionOnPublish && action != SrsActionOnUnpublish {
		return nil
	}

	var config CallbackConfig
	func() {
		v.lock.Lock()
		defer v.lock.Unlock()
		config = v.ephemeralConfig
	}()

	if !config.All || config.Target == "" {
		return nil
	}

	req := &struct {
		RequestID string `json:"request_id"`
		// The callback parameters.
		Action string `json:"action"`
		Opaque string `json:"opaque"`
		Vhost  string `json:"vhost,omitempty"`
		App    string `json:"app,omitempty"`
		Stream string `json:"stream,omitempty"`
		Param  string `json:"param,omitempty"`
	}{
		RequestID: uuid.NewString(),
		// The callback parameters.
		Action: string(action),
		Opaque: config.Opaque,
		Vhost:  streamObj.Vhost,
		App:    streamObj.App,
		Stream: streamObj.Stream,
	}
	if action == SrsActionOnPublish {
		req.Param = streamObj.Param
	}

	pfn4 := func(b, b2 []byte, code int) error {
		if code != 0 {
			return errors.Errorf("response code %v", code)
		}

		logger.Tf(ctx, "callback ok, post %v with %s, response %v", config.String(), string(b), string(b2))
		return nil
	}

	pfn3 := func(b, b2 []byte) error {
		if code, err := strconv.ParseInt(string(b2), 10, 64); err == nil {
			return pfn4(b, b2, int(code))
		}

		var code int
		if err := json.Unmarshal(b2, &struct {
			Code *int `json:"code"`
		}{
			Code: &code,
		}); err != nil {
			return errors.Wrapf(err, "unmarshal response")
		}
		return pfn4(b, b2, code)
	}

	pfn2 := func(b []byte) error {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, config.Target, bytes.NewReader(b))
		if err != nil {
			return errors.Wrapf(err, "new request")
		}

		req.Header.Set("Content-Type", "application/json")

		var res *http.Response
		if strings.HasPrefix(config.Target, "https://") {
			client := &http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{
						InsecureSkipVerify: true,
					},
				},
			}
			res, err = client.Do(req)
		} else {
			res, err = http.DefaultClient.Do(req)
		}
		if err != nil {
			return errors.Wrapf(err, "http post")
		}
		defer res.Body.Close()

		if res.StatusCode != http.StatusOK {
			return errors.Errorf("response status %v", res.StatusCode)
		}

		b2, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return errors.Wrapf(err, "read body")
		}

		if err := rdb.HSet(ctx, SRS_HOOKS, "res", string(b2)).Err(); err != nil && err != redis.Nil {
			return errors.Wrapf(err, "hset %v res %v", SRS_HOOKS, string(b2))
		}

		if err := pfn3(b, b2); err != nil {
			return errors.Wrapf(err, "res body %v", string(b2))
		}

		return nil
	}

	pfn := func() error {
		b, err := json.Marshal(req)
		if err != nil {
			return errors.Wrapf(err, "marshal req")
		}

		if err := rdb.HSet(ctx, SRS_HOOKS, "req", string(b)).Err(); err != nil && err != redis.Nil {
			return errors.Wrapf(err, "hset %v req %v", SRS_HOOKS, string(b))
		}

		if err := pfn2(b); err != nil {
			return errors.Wrapf(err, "post with %s", string(b))
		}

		return nil
	}

	if err := pfn(); err != nil {
		return errors.Wrapf(err, "callback with conf %v, req %v", config.String(), req)
	}
	return nil
}

func (v *CallbackWorker) OnRecordMessage(ctx context.Context, action SrsAction, taskUUID string, message *SrsOnHlsMessage, artifact *M3u8VoDArtifact) error {
	if action != SrsActionOnRecordBegin && action != SrsActionOnRecordEnd {
		return nil
	}
	if action == SrsActionOnRecordEnd && artifact == nil {
		return fmt.Errorf("artifact should not be nil")
	}

	var config CallbackConfig
	func() {
		v.lock.Lock()
		defer v.lock.Unlock()
		config = v.ephemeralConfig
	}()

	if !config.All || config.Target == "" {
		return nil
	}

	req := &struct {
		RequestID string `json:"request_id"`
		// The callback parameters.
		Action       string `json:"action"`
		Opaque       string `json:"opaque"`
		Vhost        string `json:"vhost,omitempty"`
		App          string `json:"app,omitempty"`
		Stream       string `json:"stream,omitempty"`
		UUID         string `json:"uuid,omitempty"`
		ArtifactCode *int   `json:"artifact_code,omitempty"`
		ArtifactPath string `json:"artifact_path,omitempty"`
		ArtifactURL  string `json:"artifact_url,omitempty"`
	}{
		RequestID: uuid.NewString(),
		// The callback parameters.
		Action: string(action),
		Opaque: config.Opaque,
		UUID:   taskUUID,
		Vhost:  message.Vhost,
		App:    message.App,
		Stream: message.Stream,
	}

	if action == SrsActionOnRecordEnd {
		code := 0
		if artifact.Processing {
			code = int(SrsStackErrorCallbackRecord)
		}
		req.ArtifactCode = &code
		req.ArtifactPath = fmt.Sprintf("%v/record/%v/index.mp4", serverDataDirectory, artifact.UUID)
		req.ArtifactURL = fmt.Sprintf("%v/terraform/v1/hooks/record/hls/%v/index.mp4", config.Host, artifact.UUID)
	}

	pfn4 := func(b, b2 []byte, code int) error {
		if code != 0 {
			return errors.Errorf("response code %v", code)
		}

		logger.Tf(ctx, "callback ok, post %v with %s, response %v", config.String(), string(b), string(b2))
		return nil
	}

	pfn3 := func(b, b2 []byte) error {
		if code, err := strconv.ParseInt(string(b2), 10, 64); err == nil {
			return pfn4(b, b2, int(code))
		}

		var code int
		if err := json.Unmarshal(b2, &struct {
			Code *int `json:"code"`
		}{
			Code: &code,
		}); err != nil {
			return errors.Wrapf(err, "unmarshal response")
		}
		return pfn4(b, b2, code)
	}

	pfn2 := func(b []byte) error {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, config.Target, bytes.NewReader(b))
		if err != nil {
			return errors.Wrapf(err, "new request")
		}

		req.Header.Set("Content-Type", "application/json")

		var res *http.Response
		if strings.HasPrefix(config.Target, "https://") {
			client := &http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{
						InsecureSkipVerify: true,
					},
				},
			}
			res, err = client.Do(req)
		} else {
			res, err = http.DefaultClient.Do(req)
		}
		if err != nil {
			return errors.Wrapf(err, "http post")
		}
		defer res.Body.Close()

		if res.StatusCode != http.StatusOK {
			return errors.Errorf("response status %v", res.StatusCode)
		}

		b2, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return errors.Wrapf(err, "read body")
		}

		if err := rdb.HSet(ctx, SRS_HOOKS, "res", string(b2)).Err(); err != nil && err != redis.Nil {
			return errors.Wrapf(err, "hset %v res %v", SRS_HOOKS, string(b2))
		}

		if err := pfn3(b, b2); err != nil {
			return errors.Wrapf(err, "res body %v", string(b2))
		}

		return nil
	}

	pfn := func() error {
		b, err := json.Marshal(req)
		if err != nil {
			return errors.Wrapf(err, "marshal req")
		}

		if err := rdb.HSet(ctx, SRS_HOOKS, "req", string(b)).Err(); err != nil && err != redis.Nil {
			return errors.Wrapf(err, "hset %v req %v", SRS_HOOKS, string(b))
		}

		if err := pfn2(b); err != nil {
			return errors.Wrapf(err, "post with %s", string(b))
		}

		return nil
	}

	if err := pfn(); err != nil {
		return errors.Wrapf(err, "callback with conf %v, req %v", config.String(), req)
	}
	return nil
}

func (v *CallbackWorker) OnOCR(ctx context.Context, action SrsAction, taskUUID string, message *SrsOnHlsMessage, prompt, result string) error {
	if action != SrsActionOnOcr {
		return nil
	}

	var config CallbackConfig
	func() {
		v.lock.Lock()
		defer v.lock.Unlock()
		config = v.ephemeralConfig
	}()

	if !config.All || config.Target == "" {
		return nil
	}

	req := &struct {
		RequestID string `json:"request_id"`
		// The callback parameters.
		Action string `json:"action"`
		Opaque string `json:"opaque"`
		Vhost  string `json:"vhost,omitempty"`
		App    string `json:"app,omitempty"`
		Stream string `json:"stream,omitempty"`
		// The OCR task UUID.
		UUID string `json:"uuid,omitempty"`
		// The OCR prompt.
		Prompt string `json:"prompt,omitempty"`
		// The OCR result.
		Result string `json:"result,omitempty"`
	}{
		RequestID: uuid.NewString(),
		// The callback parameters.
		Action: string(action),
		Opaque: config.Opaque,
		Vhost:  message.Vhost,
		App:    message.App,
		Stream: message.Stream,
		// The OCR task UUID.
		UUID: taskUUID,
		// The OCR prompt.
		Prompt: prompt,
		// The OCR result.
		Result: result,
	}

	pfn4 := func(b, b2 []byte, code int) error {
		if code != 0 {
			return errors.Errorf("response code %v", code)
		}

		logger.Tf(ctx, "callback ok, post %v with %s, response %v", config.String(), string(b), string(b2))
		return nil
	}

	pfn3 := func(b, b2 []byte) error {
		if code, err := strconv.ParseInt(string(b2), 10, 64); err == nil {
			return pfn4(b, b2, int(code))
		}

		var code int
		if err := json.Unmarshal(b2, &struct {
			Code *int `json:"code"`
		}{
			Code: &code,
		}); err != nil {
			return errors.Wrapf(err, "unmarshal response")
		}
		return pfn4(b, b2, code)
	}

	pfn2 := func(b []byte) error {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, config.Target, bytes.NewReader(b))
		if err != nil {
			return errors.Wrapf(err, "new request")
		}

		req.Header.Set("Content-Type", "application/json")

		var res *http.Response
		if strings.HasPrefix(config.Target, "https://") {
			client := &http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{
						InsecureSkipVerify: true,
					},
				},
			}
			res, err = client.Do(req)
		} else {
			res, err = http.DefaultClient.Do(req)
		}
		if err != nil {
			return errors.Wrapf(err, "http post")
		}
		defer res.Body.Close()

		if res.StatusCode != http.StatusOK {
			return errors.Errorf("response status %v", res.StatusCode)
		}

		b2, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return errors.Wrapf(err, "read body")
		}

		if err := rdb.HSet(ctx, SRS_HOOKS, "res", string(b2)).Err(); err != nil && err != redis.Nil {
			return errors.Wrapf(err, "hset %v res %v", SRS_HOOKS, string(b2))
		}

		if err := pfn3(b, b2); err != nil {
			return errors.Wrapf(err, "res body %v", string(b2))
		}

		return nil
	}

	pfn := func() error {
		b, err := json.Marshal(req)
		if err != nil {
			return errors.Wrapf(err, "marshal req")
		}

		if err := rdb.HSet(ctx, SRS_HOOKS, "req", string(b)).Err(); err != nil && err != redis.Nil {
			return errors.Wrapf(err, "hset %v req %v", SRS_HOOKS, string(b))
		}

		if err := pfn2(b); err != nil {
			return errors.Wrapf(err, "post with %s", string(b))
		}

		return nil
	}

	if err := pfn(); err != nil {
		return errors.Wrapf(err, "callback with conf %v, req %v", config.String(), req)
	}
	return nil
}

type CallbackConfig struct {
	// The callback target.
	Target string `json:"target"`
	// The opaque string, for example, the token.
	Opaque string `json:"opaque"`
	// Whether to callback all streams.
	All bool `json:"all"`
	// The full host to generate the full URl for callback.
	Host string `json:"host"`
}

func (v CallbackConfig) String() string {
	return fmt.Sprintf("target=%v, opaque=%v, all=%v, host=%v",
		v.Target, v.Opaque, v.All, v.Host)
}

// Load 是 CallbackConfig 类型的一个方法，用于从Redis中加载回调配置。
// 该方法专注于从Redis的哈希表中获取特定字段，并将它们用于配置对象。
// 参数:
//
//	ctx - 上下文，用于取消请求和传递请求级值。
//
// 返回值:
//
//	如果加载过程中发生错误，则返回错误。
func (v *CallbackConfig) Load(ctx context.Context) (err error) {
	// 从Redis中加载目标回调URL
	if v.Target, err = rdb.HGet(ctx, SRS_HOOKS, "target").Result(); err != nil && err != redis.Nil {
		return errors.Wrapf(err, "hget %v target", SRS_HOOKS)
	}

	// 从Redis中加载不透明标识符。
	if v.Opaque, err = rdb.HGet(ctx, SRS_HOOKS, "opaque").Result(); err != nil && err != redis.Nil {
		return errors.Wrapf(err, "hget %v opaque", SRS_HOOKS)
	}

	// 从Redis中加载是否启用所有回调的标志。
	if all, err := rdb.HGet(ctx, SRS_HOOKS, "all").Result(); err != nil && err != redis.Nil {
		return errors.Wrapf(err, "hget %v all", SRS_HOOKS)
	} else if all == "true" {
		v.All = true
	}

	// 从Redis中加载主机名或IP地址。
	if v.Host, err = rdb.HGet(ctx, SRS_HOOKS, "host").Result(); err != nil && err != redis.Nil {
		return errors.Wrapf(err, "hget %v host", SRS_HOOKS)
	}

	// 如果所有字段都成功加载，返回nil表示成功。
	return nil
}
