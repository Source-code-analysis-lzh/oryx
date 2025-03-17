// Copyright (c) 2022-2024 Winlin
//
// SPDX-License-Identifier: MIT
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/ossrs/go-oryx-lib/errors"
	"github.com/ossrs/go-oryx-lib/logger"
	// Use v8 because we use Go 1.16+, while v9 requires Go 1.18+
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
)

// conf 是一个全局配置对象。
var conf *Config

func init() {
	// 初始化证书管理器。
	certManager = NewCertManager()
	// 初始化配置对象。
	conf = NewConfig()

	// 初始化一个快速缓存，用于快速更新，例如 LLHLS 配置。
	fastCache = NewFastCache()
}

func main() {
	// 创建一个带有日志功能的上下文。
	ctx := logger.WithContext(context.Background())
	ctx = logger.WithContext(ctx)

	// 运行主逻辑并处理错误。
	if err := doMain(ctx); err != nil {
		logger.Tf(ctx, "run err %+v", err)
		return
	}

	logger.Tf(ctx, "run ok")
}

// doMain 包含应用程序的主要逻辑。
func doMain(ctx context.Context) error {
	// 定义一个标志来显示版本。
	var showVersion bool
	flag.BoolVar(&showVersion, "v", false, "Print version and quit")
	flag.BoolVar(&showVersion, "version", false, "Print version and quit")
	flag.Parse()

	// 如果设置了版本标志，打印版本并退出。
	if showVersion {
		fmt.Println(strings.TrimPrefix(version, "v"))
		os.Exit(0)
	}

	// 安装信号处理器以实现优雅关闭。
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		for s := range sc { // 阻塞等待信号
			logger.Tf(ctx, "Got signal %v", s)
			cancel() // 触发上下文取消
		}
	}()

	// When cancelled, the program is forced to exit due to a timeout. Normally, this doesn't occur
	// because the main thread exits after the context is cancelled. However, sometimes the main thread
	// may be blocked for some reason, so a forced exit is necessary to ensure the program terminates.
	// 启动 goroutine 监听上下文取消事件
	go func() {
		<-ctx.Done() // 阻塞等待上下文取消
		time.Sleep(30 * time.Second)
		logger.Wf(ctx, "Force to exit by timeout")
		os.Exit(1) // 强制退出程序
	}()

	// 初始化管理密码并加载环境，无需依赖 Redis。
	if true {
		// 获取当前工作目录。
		if pwd, err := os.Getwd(); err != nil {
			return errors.Wrapf(err, "getpwd")
		} else {
			conf.Pwd = pwd
		}

		// Note that we only use .env in mgmt.
		// 加载管理 .env 文件。
		envFile := path.Join(conf.Pwd, "containers/data/config/.env")
		if _, err := os.Stat(envFile); err == nil {
			if err := godotenv.Overload(envFile); err != nil {
				return errors.Wrapf(err, "load %v", envFile)
			}
		}
	}

	// For platform, default to development for Darwin.
	setEnvDefault("NODE_ENV", "development")
	// For platform, HTTP server listen port.
	setEnvDefault("PLATFORM_LISTEN", "2024")
	// Set the default language, en or zh.
	setEnvDefault("REACT_APP_LOCALE", "en")
	// Whether enable the Go pprof.
	setEnvDefault("GO_PPROF", "")

	// Migrate from mgmt.
	setEnvDefault("REDIS_DATABASE", "0")
	setEnvDefault("REDIS_HOST", "127.0.0.1")
	setEnvDefault("REDIS_PORT", "6379")
	// 面向 管理员，用于系统配置和监控，默认端口 2022，通常映射到公网 80/443
	setEnvDefault("MGMT_LISTEN", "2022")

	// For HTTPS.
	// 面向 开发者/客户端，处理流媒体业务逻辑，默认端口 2024，需根据业务需求开放。
	setEnvDefault("HTTPS_LISTEN", "2443")
	setEnvDefault("AUTO_SELF_SIGNED_CERTIFICATE", "on")

	// For feature control.
	setEnvDefault("NAME_LOOKUP", "on")
	setEnvDefault("PLATFORM_DOCKER", "off")

	// For multiple ports.
	setEnvDefault("RTMP_PORT", "1935")
	setEnvDefault("HTTP_PORT", "")
	setEnvDefault("SRT_PORT", "10080")
	setEnvDefault("RTC_PORT", "8000")

	// For system limit.
	setEnvDefault("SRS_FORWARD_LIMIT", "10")
	setEnvDefault("SRS_VLIVE_LIMIT", "10")
	setEnvDefault("SRS_CAMERA_LIMIT", "10")

	logger.Tf(ctx, "load .env as MGMT_PASSWORD=%vB, GO_PPROF=%v, "+
		"SRS_PLATFORM_SECRET=%vB, CLOUD=%v, REGION=%v, SOURCE=%v, SRT_PORT=%v, RTC_PORT=%v, "+
		"NODE_ENV=%v, LOCAL_RELEASE=%v, REDIS_DATABASE=%v, REDIS_HOST=%v, REDIS_PASSWORD=%vB, REDIS_PORT=%v, RTMP_PORT=%v, "+
		"PUBLIC_URL=%v, BUILD_PATH=%v, REACT_APP_LOCALE=%v, PLATFORM_LISTEN=%v, HTTP_PORT=%v, "+
		"REGISTRY=%v, MGMT_LISTEN=%v, HTTPS_LISTEN=%v, AUTO_SELF_SIGNED_CERTIFICATE=%v, "+
		"NAME_LOOKUP=%v, PLATFORM_DOCKER=%v, SRS_FORWARD_LIMIT=%v, SRS_VLIVE_LIMIT=%v, "+
		"SRS_CAMERA_LIMIT=%v, YTDL_PROXY=%v",
		len(envMgmtPassword()), envGoPprof(), len(envApiSecret()), envCloud(),
		envRegion(), envSource(), envSrtListen(), envRtcListen(),
		envNodeEnv(), envLocalRelease(),
		envRedisDatabase(), envRedisHost(), len(envRedisPassword()), envRedisPort(),
		envRtmpPort(), envPublicUrl(),
		envBuildPath(), envReactAppLocale(), envPlatformListen(), envHttpPort(),
		envRegistry(), envMgmtListen(), envHttpListen(),
		envSelfSignedCertificate(), envNameLookup(),
		envPlatformDocker(), envForwardLimit(), envVLiveLimit(),
		envCameraLimit(), envYtdlProxy(),
	)

	// Start the Go pprof if enabled.
	// 启用 Go 自带的性能分析工具，通过 HTTP 端口提供 profiling 数据。
	if addr := envGoPprof(); addr != "" {
		go func() {
			logger.Tf(ctx, "Start Go pprof at %v", addr)
			// GO_PPROF=localhost:6060
			http.ListenAndServe(addr, nil)
		}()
	}

	if err := initMgmtOS(ctx); err != nil {
		return errors.Wrapf(err, "init mgmt os")
	}

	// Initialize global rdb, the redis client.
	if err := InitRdb(); err != nil {
		return errors.Wrapf(err, "init rdb")
	}
	logger.Tf(ctx, "init rdb(redis client) ok")

	// For platform, we should initOS after redis.
	// Setup the OS for redis, which should never depends on redis.
	// 对于平台，我们应该在 Redis 之后初始化操作系统。
	// 为 Redis 设置操作系统，这绝不应该依赖于 Redis。
	if err := initOS(ctx); err != nil {
		return errors.Wrapf(err, "init os")
	}

	// We must initialize the platform after redis is ready.
	if err := initPlatform(ctx); err != nil {
		return errors.Wrapf(err, "init platform")
	}

	// We must initialize the mgmt after redis is ready.
	if err := initMmgt(ctx); err != nil {
		return errors.Wrapf(err, "init mgmt")
	}
	logger.Tf(ctx, "initialize platform region=%v, registry=%v, version=%v", conf.Region, conf.Registry, version)

	// 创建协程，用于将域名解析为 IP 地址。
	candidateWorker = NewCandidateWorker()
	defer candidateWorker.Close()
	if err := candidateWorker.Start(ctx); err != nil {
		return errors.Wrapf(err, "start candidate worker")
	}

	// Create callback worker.
	// 创建协程，回调你的服务
	callbackWorker = NewCallbackWorker()
	defer callbackWorker.Close()
	if err := callbackWorker.Start(ctx); err != nil {
		return errors.Wrapf(err, "start callback worker")
	}

	// Create transcript worker for transcription.
	// 创建协程，转录音频
	transcriptWorker = NewTranscriptWorker()
	defer transcriptWorker.Close()
	if err := transcriptWorker.Start(ctx); err != nil {
		return errors.Wrapf(err, "start transcript worker")
	}

	// Create OCR worker for OCR service.
	ocrWorker = NewOCRWorker()
	defer ocrWorker.Close()
	if err := ocrWorker.Start(ctx); err != nil {
		return errors.Wrapf(err, "start OCR worker")
	}

	// Create AI Talk worker for live room.
	talkServer = NewTalkServer()
	defer talkServer.Close()

	// Create AI Dubbing server for VoD translation.
	dubbingServer = NewDubbingServer()
	defer dubbingServer.Close()

	// Create transcode worker for transcoding.
	transcodeWorker = NewTranscodeWorker()
	defer transcodeWorker.Close()
	if err := transcodeWorker.Start(ctx); err != nil {
		return errors.Wrapf(err, "start transcode worker")
	}

	// Create worker for RECORD, covert live stream to local file.
	recordWorker = NewRecordWorker()
	defer recordWorker.Close()
	if err := recordWorker.Start(ctx); err != nil {
		return errors.Wrapf(err, "start record worker")
	}

	// Create worker for DVR, covert live stream to local file.
	dvrWorker = NewDvrWorker()
	defer dvrWorker.Close()
	if err := dvrWorker.Start(ctx); err != nil {
		return errors.Wrapf(err, "start dvr worker")
	}

	// Create worker for VoD, covert live stream to local file.
	vodWorker = NewVodWorker()
	defer vodWorker.Close()
	if err := vodWorker.Start(ctx); err != nil {
		return errors.Wrapf(err, "start vod worker")
	}

	// Create worker for forwarding.
	forwardWorker = NewForwardWorker()
	defer forwardWorker.Close()
	if err := forwardWorker.Start(ctx); err != nil {
		return errors.Wrapf(err, "start forward worker")
	}

	// Create worker for vLive.
	vLiveWorker = NewVLiveWorker()
	defer vLiveWorker.Close()
	if err := vLiveWorker.Start(ctx); err != nil {
		return errors.Wrapf(err, "start vLive worker")
	}

	// Create worker for IP camera.
	cameraWorker = NewCameraWorker()
	defer cameraWorker.Close()
	if err := cameraWorker.Start(ctx); err != nil {
		return errors.Wrapf(err, "start IP camera worker")
	}

	// Create worker for crontab.
	crontabWorker = NewCrontabWorker()
	defer crontabWorker.Close()
	if err := crontabWorker.Start(ctx); err != nil {
		return errors.Wrapf(err, "start crontab worker")
	}

	// Run HTTP service.
	httpService := NewHTTPService()
	defer httpService.Close()
	if err := httpService.Run(ctx); err != nil {
		return errors.Wrapf(err, "start http service")
	}

	return nil
}

// initMgmtOS 初始化管理操作系统的环境配置。
// 参数:
//
//	ctx - 上下文对象，用于控制函数的生命周期和传递信息。
//
// 返回值:
//
//	error - 如果初始化过程中发生错误，则返回具体的错误信息；否则返回 nil 表示成功。
func initMgmtOS(ctx context.Context) (err error) {
	// For Darwin, append the search PATH for docker.
	// Note that we should set the PATH env, not the exec.Cmd.Env.
	// 在 macOS（Darwin）系统中，确保 Docker 可执行文件路径 /usr/local/bin 被添加到 PATH 环境变量中。这解决了 macOS 下 Docker 命令可能无法直接调用的问题。
	if conf.IsDarwin && !strings.Contains(envPath(), "/usr/local/bin") {
		os.Setenv("PATH", fmt.Sprintf("%v:/usr/local/bin", envPath()))
	}

	// The redis is not available when os startup, so we must directly discover from env or network.
	// Redis 在操作系统启动时不可用，因此我们必须直接从环境变量或网络中发现。
	if conf.Cloud, conf.Region, err = discoverRegion(ctx); err != nil {
		return errors.Wrapf(err, "discover region")
	}

	// Always update the source, because it might change.
	if conf.Source, err = discoverSource(ctx, conf.Cloud, conf.Region); err != nil {
		return errors.Wrapf(err, "discover source")
	}

	// Always update the registry, because it might change.
	if conf.Registry, err = discoverRegistry(ctx, conf.Source); err != nil {
		return errors.Wrapf(err, "discover registry")
	}

	logger.Tf(ctx, "initOS %v", conf.String())
	return
}

// initOS 初始化操作系统相关的配置。
// 参数:
//
//	ctx - 上下文，用于控制生命周期和传递信息。
//
// 返回值:
//
//	err - 如果发生错误，则返回非 nil 的错误对象。
func initOS(ctx context.Context) (err error) {
	// Create api secret if not exists, see setupApiSecret
	// 如果 API 密钥不存在，则创建一个新的密钥。
	if token, err := rdb.HGet(ctx, SRS_PLATFORM_SECRET, "token").Result(); err != nil && err != redis.Nil {
		return errors.Wrapf(err, "hget %v token", SRS_PLATFORM_SECRET)
	} else if token == "" {
		token = envApiSecret()
		if token == "" {
			token = fmt.Sprintf("srs-v2-%v", strings.ReplaceAll(uuid.NewString(), "-", ""))
		}

		if err = rdb.HSet(ctx, SRS_PLATFORM_SECRET, "token", token).Err(); err != nil {
			return errors.Wrapf(err, "hset %v token %v", SRS_PLATFORM_SECRET, token)
		}

		update := time.Now().Format(time.RFC3339)
		if err = rdb.HSet(ctx, SRS_PLATFORM_SECRET, "update", update).Err(); err != nil {
			return errors.Wrapf(err, "hset %v update %v", SRS_PLATFORM_SECRET, update)
		}
		logger.Tf(ctx, "Platform update api secret, token=%vB, at=%v", len(token), update)
	}

	// For platform, we must use the secret to access API of mgmt.
	// Query the api secret from redis, cache it to env.
	// 从 Redis 中查询 API 密钥并缓存到环境变量中。
	if envApiSecret() == "" {
		if token, err := rdb.HGet(ctx, SRS_PLATFORM_SECRET, "token").Result(); err != nil && err != redis.Nil {
			return errors.Wrapf(err, "hget %v token", SRS_PLATFORM_SECRET)
		} else {
			os.Setenv("SRS_PLATFORM_SECRET", token)
			logger.Tf(ctx, "Update api secret to %vB", len(token))
		}
	}

	// Load the platform from redis, initialized by mgmt.
	// 从 Redis 加载云平台信息，如果不存在则初始化。
	if cloud, err := rdb.HGet(ctx, SRS_TENCENT_LH, "cloud").Result(); err != nil && err != redis.Nil {
		return errors.Wrapf(err, "hget %v cloud", SRS_TENCENT_LH)
	} else if cloud == "" || conf.Cloud != cloud {
		if err = rdb.HSet(ctx, SRS_TENCENT_LH, "cloud", conf.Cloud).Err(); err != nil && err != redis.Nil {
			return errors.Wrapf(err, "hset %v cloud %v", SRS_TENCENT_LH, conf.Cloud)
		}
		logger.Tf(ctx, "Update cloud=%v", conf.Cloud)
	}

	// Load the region first, because it never changed.
	// 加载区域信息，因为区域信息通常不会改变。
	if region, err := rdb.HGet(ctx, SRS_TENCENT_LH, "region").Result(); err != nil && err != redis.Nil {
		return errors.Wrapf(err, "hget %v region", SRS_TENCENT_LH)
	} else if region == "" || conf.Region != region {
		if err = rdb.HSet(ctx, SRS_TENCENT_LH, "region", conf.Region).Err(); err != nil && err != redis.Nil {
			return errors.Wrapf(err, "hset %v region %v", SRS_TENCENT_LH, conf.Region)
		}
		logger.Tf(ctx, "Update region=%v", conf.Region)
	}

	// Always update the source, because it might change.
	// 始终更新来源信息，因为它可能会改变。
	if source, err := rdb.HGet(ctx, SRS_TENCENT_LH, "source").Result(); err != nil && err != redis.Nil {
		return errors.Wrapf(err, "hget %v source", SRS_TENCENT_LH)
	} else if source == "" || conf.Source != source {
		if err = rdb.HSet(ctx, SRS_TENCENT_LH, "source", conf.Source).Err(); err != nil && err != redis.Nil {
			return errors.Wrapf(err, "hset %v source %v", SRS_TENCENT_LH, conf.Source)
		}
		logger.Tf(ctx, "Update source=%v", conf.Source)
	}

	// 检查注册表信息，并更新全局配置。
	if registry, err := rdb.HGet(ctx, SRS_TENCENT_LH, "registry").Result(); err != nil && err != redis.Nil {
		return errors.Wrapf(err, "hget %v registry", SRS_TENCENT_LH)
	} else if registry != "" {
		conf.Registry = registry
	}

	// Discover and update the platform for stat only, not the OS platform.
	// 发现并更新平台信息（仅用于统计目的，不是操作系统平台）。
	if platform, err := discoverPlatform(ctx, conf.Cloud); err != nil {
		return errors.Wrapf(err, "discover platform by cloud=%v", conf.Cloud)
	} else {
		if err = rdb.HSet(ctx, SRS_TENCENT_LH, "platform", platform).Err(); err != nil && err != redis.Nil {
			return errors.Wrapf(err, "hset %v platform %v", SRS_TENCENT_LH, platform)
		}
		logger.Tf(ctx, "Update platform=%v", platform)
	}

	// Always update the registry, because it might change.
	// 始终更新注册表信息，因为它可能会改变。
	if registry, err := rdb.HGet(ctx, SRS_TENCENT_LH, "registry").Result(); err != nil && err != redis.Nil {
		return errors.Wrapf(err, "hget %v registry", SRS_TENCENT_LH)
	} else if registry == "" || conf.Registry != registry {
		if err = rdb.HSet(ctx, SRS_TENCENT_LH, "registry", conf.Registry).Err(); err != nil && err != redis.Nil {
			return errors.Wrapf(err, "hset %v registry %v", SRS_TENCENT_LH, conf.Registry)
		}
		logger.Tf(ctx, "Update registry=%v", conf.Registry)
	}

	// Request the host platform OS, whether the OS is Darwin.
	// 请求主机的操作系统信息，判断是否为 Darwin 系统。
	hostPlatform := runtime.GOOS
	// Because platform might run in docker, so we overwrite it by query from mgmt.
	if hostPlatform == "darwin" {
		conf.IsDarwin = true
	}

	// Start a goroutine to update ipv4 of config.
	// 启动一个 goroutine 来更新配置中的 IPv4 地址。
	if err := refreshIPv4(ctx); err != nil {
		return errors.Wrapf(err, "refresh ipv4")
	}

	logger.Tf(ctx, "initOS %v", conf.String())
	return
}

// Initialize the platform before thread run.
// initPlatform 初始化平台环境。
// 参数:
//
//	ctx - 上下文，用于控制生命周期和传递信息。
//
// 返回值:
//
//	error - 如果发生错误，则返回非 nil 的错误对象。
func initPlatform(ctx context.Context) error {
	// For Darwin, append the search PATH for docker.
	// Note that we should set the PATH env, not the exec.Cmd.Env.
	// Note that it depends on conf.IsDarwin, so it's unavailable util initOS.
	// 对于 Darwin 系统（macOS），确保 Docker 可执行文件路径 /usr/local/bin 被添加到 PATH 环境变量中。
	// 这解决了 macOS 下 Docker 命令可能无法直接调用的问题。
	if conf.IsDarwin && !strings.Contains(envPath(), "/usr/local/bin") {
		os.Setenv("PATH", fmt.Sprintf("%v:/usr/local/bin", envPath()))
	}

	// Create directories for data, allow user to link it.
	// Keep in mind that the containers/data/srs-s3-bucket maybe mount by user, because user should generate
	// and mount it if they wish to save recordings to cloud storage.
	// 创建数据目录，允许用户链接这些目录。
	// 请注意，containers/data/srs-s3-bucket 目录可能由用户挂载，因为用户需要生成并挂载该目录以将录制保存到云存储。
	for _, dir := range []string{
		"containers/data/dvr", "containers/data/record", "containers/data/vod",
		"containers/data/upload", "containers/data/vlive", "containers/data/signals",
		"containers/data/lego", "containers/data/.well-known", "containers/data/config",
		"containers/data/transcript", "containers/data/srs-s3-bucket", "containers/data/ai-talk",
		"containers/data/dubbing", "containers/data/ocr",
	} {
		if _, err := os.Stat(dir); err != nil && os.IsNotExist(err) {
			if err = os.MkdirAll(dir, os.ModeDir|os.FileMode(0755)); err != nil {
				return errors.Wrapf(err, "create dir %v", dir)
			}
		}
	}

	// Run only once for a special version.
	// 仅针对特定版本运行一次初始化操作。
	bootRelease := "v2023-r30"
	if firstRun, err := rdb.HGet(ctx, SRS_FIRST_BOOT, bootRelease).Result(); err != nil && err != redis.Nil {
		return errors.Wrapf(err, "hget %v %v", SRS_FIRST_BOOT, bootRelease)
	} else if firstRun == "" {
		logger.Tf(ctx, "boot setup, v=%v, key=%v", bootRelease, SRS_FIRST_BOOT)

		// Generate the dynamic config for NGINX.
		// 生成 NGINX 动态配置。
		if err := nginxGenerateConfig(ctx); err != nil {
			return errors.Wrapf(err, "nginx config and reload")
		}

		// Generate the dynamic config for SRS.
		// 生成 SRS 动态配置。
		if err := srsGenerateConfig(ctx); err != nil {
			return errors.Wrapf(err, "srs config and reload")
		}

		// Run once, record in redis.
		// 记录已运行一次的操作到 Redis。
		if err := rdb.HSet(ctx, SRS_FIRST_BOOT, bootRelease, 1).Err(); err != nil {
			return errors.Wrapf(err, "hset %v %v 1", SRS_FIRST_BOOT, bootRelease)
		}

		logger.Tf(ctx, "boot done, v=%v, key=%v", bootRelease, SRS_FIRST_BOOT)
	} else {
		logger.Tf(ctx, "boot already done, v=%v, key=%v", bootRelease, SRS_FIRST_BOOT)
	}

	// For development, request the releases from itself which proxy to the releases service.
	// 在开发环境中请求来自自身的发布版本，通过代理服务进行查询。
	go refreshLatestVersion(ctx)

	// Disable srs-dev, only enable srs-server.
	// 禁用 srs-dev 容器，仅启用 srs-server 容器。
	if srsDevEnabled, err := rdb.HGet(ctx, SRS_CONTAINER_DISABLED, srsDevDockerName).Result(); err != nil && err != redis.Nil {
		return errors.Wrapf(err, "hget %v %v", SRS_CONTAINER_DISABLED, srsDevDockerName)
	} else if srsEnabled, err := rdb.HGet(ctx, SRS_CONTAINER_DISABLED, srsDockerName).Result(); err != nil && err != redis.Nil {
		return errors.Wrapf(err, "hget %v %v", SRS_CONTAINER_DISABLED, srsDockerName)
	} else if srsDevEnabled != "true" && srsEnabled == "true" {
		r0 := rdb.HSet(ctx, SRS_CONTAINER_DISABLED, srsDevDockerName, "true").Err()
		r1 := rdb.HSet(ctx, SRS_CONTAINER_DISABLED, srsDockerName, "false").Err()
		logger.Wf(ctx, "Disable srs-dev r0=%v, only enable srs-server r1=%v", r0, r1)
	}

	// For SRS, if release enabled, disable dev automatically.
	// 如果 SRS 发布版本已启用，则自动禁用开发版本。
	if srsReleaseDisabled, err := rdb.HGet(ctx, SRS_CONTAINER_DISABLED, srsDockerName).Result(); err != nil && err != redis.Nil {
		return errors.Wrapf(err, "hget %v %v", SRS_CONTAINER_DISABLED, srsDockerName)
	} else if srsDevDisabled, err := rdb.HGet(ctx, SRS_CONTAINER_DISABLED, srsDevDockerName).Result(); err != nil && err != redis.Nil {
		return errors.Wrapf(err, "hget %v %v", SRS_CONTAINER_DISABLED, srsDevDockerName)
	} else if srsReleaseDisabled != "true" && srsDevDisabled != "true" {
		r0 := rdb.HSet(ctx, SRS_CONTAINER_DISABLED, srsDevDockerName, true).Err()
		logger.Tf(ctx, "disable srs dev for release enabled, r0=%v", r0)
	}

	// Setup the publish secret for first run.
	// 设置首次运行时的发布密钥。
	if publish, err := rdb.HGet(ctx, SRS_AUTH_SECRET, "pubSecret").Result(); err != nil && err != redis.Nil {
		return errors.Wrapf(err, "hget %v pubSecret", SRS_AUTH_SECRET)
	} else if publish == "" {
		publish = strings.ReplaceAll(uuid.NewString(), "-", "")
		if err = rdb.HSet(ctx, SRS_AUTH_SECRET, "pubSecret", publish).Err(); err != nil && err != redis.Nil {
			return errors.Wrapf(err, "hset %v pubSecret %v", SRS_AUTH_SECRET, publish)
		}
		if err = rdb.Set(ctx, SRS_SECRET_PUBLISH, publish, 0).Err(); err != nil && err != redis.Nil {
			return errors.Wrapf(err, "set %v %v", SRS_SECRET_PUBLISH, publish)
		}
	}

	// Migrate from previous versions.
	// 从以前的版本迁移数据。
	for _, migrate := range []struct {
		PVK string
		CVK string
	}{
		{"SRS_RECORD_M3U8_METADATA", SRS_RECORD_M3U8_ARTIFACT},
		{"SRS_DVR_M3U8_METADATA", SRS_DVR_M3U8_ARTIFACT},
		{"SRS_VOD_M3U8_METADATA", SRS_VOD_M3U8_ARTIFACT},
	} {
		pv, _ := rdb.HLen(ctx, migrate.PVK).Result()
		cv, _ := rdb.HLen(ctx, migrate.CVK).Result()
		if pv > 0 && cv == 0 {
			if vs, err := rdb.HGetAll(ctx, migrate.PVK).Result(); err == nil {
				for k, v := range vs {
					_ = rdb.HSet(ctx, migrate.CVK, k, v)
				}
				logger.Tf(ctx, "migrate %v to %v with %v keys", migrate.PVK, migrate.CVK, len(vs))
			}
		}
	}

	// Cancel upgrading.
	// 取消升级状态。
	if upgrading, err := rdb.HGet(ctx, SRS_UPGRADING, "upgrading").Result(); err != nil && err != redis.Nil {
		return errors.Wrapf(err, "hget %v upgrading", SRS_UPGRADING)
	} else if upgrading == "1" {
		if err = rdb.HSet(ctx, SRS_UPGRADING, "upgrading", "0").Err(); err != nil && err != redis.Nil {
			return errors.Wrapf(err, "hset %v upgrading 0", SRS_UPGRADING)
		}
	}

	// Initialize the node id.
	// 初始化节点 ID。
	if nid, err := rdb.HGet(ctx, SRS_TENCENT_LH, "node").Result(); err != nil && err != redis.Nil {
		return errors.Wrapf(err, "hget %v node", SRS_TENCENT_LH)
	} else if nid == "" {
		nid = uuid.NewString()
		if err = rdb.HSet(ctx, SRS_TENCENT_LH, "node", nid).Err(); err != nil {
			return errors.Wrapf(err, "hset %v node %v", SRS_TENCENT_LH, nid)
		}
		logger.Tf(ctx, "Update node nid=%v", nid)
	}

	return nil
}

// initMmgt 初始化管理环境，包括创建必要的目录和更新环境变量。
// 参数:
//
//	ctx - 上下文对象，用于日志记录等操作。
//
// 返回值:
//
//	error - 如果初始化过程中发生错误，则返回具体的错误信息；否则返回 nil 表示成功。
func initMmgt(ctx context.Context) error {
	// Always create the data dir and sub dirs.
	// 始终创建数据目录及其子目录。
	dataDir := filepath.Join(conf.Pwd, "containers", "data")
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		// 如果数据目录不存在，则先删除可能存在的无效目录（确保干净状态）。
		if err := os.RemoveAll(dataDir); err != nil {
			return errors.Wrapf(err, "remove data dir %s", dataDir)
		}
	}

	// 创建所需的子目录列表。
	dirs := []string{"redis", "config", "dvr", "record", "vod", "upload", "vlive"}
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(dataDir, dir), 0755); err != nil {
			return errors.Wrapf(err, "create dir %s", dir)
		}
	}

	// Refresh the env file.
	// 刷新环境变量文件。
	envs := map[string]string{}
	envFile := path.Join(conf.Pwd, "containers/data/config/.env")
	if _, err := os.Stat(envFile); err == nil {
		// 如果环境变量文件已存在，则加载其中的环境变量。
		if v, err := godotenv.Read(envFile); err != nil {
			return errors.Wrapf(err, "load envs")
		} else {
			envs = v
		}
	}

	// 更新或添加新的环境变量。
	envs["CLOUD"] = conf.Cloud
	envs["REGION"] = conf.Region
	envs["SOURCE"] = conf.Source
	envs["REGISTRY"] = conf.Registry
	if envMgmtPassword() != "" { // 通过命令行设置密码环境变量
		envs["MGMT_PASSWORD"] = envMgmtPassword()
	}

	// 将更新后的环境变量写回文件。
	if err := godotenv.Write(envs, envFile); err != nil {
		return errors.Wrapf(err, "write %v", envFile)
	}
	logger.Tf(ctx, "Refresh %v ok", envFile)

	return nil
}

// Refresh the latest version when startup.
// refreshLatestVersion 在后台定期查询最新版本信息，并在获取到最新版本后取消查询。
// 参数 ctx 是上下文对象，用于控制函数的生命周期，例如取消操作或超时。
// 返回值 error 表示函数执行过程中是否发生错误。
func refreshLatestVersion(ctx context.Context) error {
	// 创建一个新的上下文和取消函数，用于控制版本查询的生命周期。
	versionsCtx, versionsCancel := context.WithCancel(context.Background())
	go func() {
		// 启动一个 goroutine 来定期查询最新版本信息。
		ctx := logger.WithContext(ctx)
		for ctx.Err() == nil {
			// 查询最新版本信息。
			versions, err := queryLatestVersion(ctx)
			if err == nil && versions != nil && versions.Latest != "" {
				// 如果查询成功且获取到最新版本信息，更新全局配置并取消查询上下文。
				logger.Tf(ctx, "query version ok, result is %v", versions.String())
				conf.Versions = *versions
				versionsCancel()

				// CrontabWorker will start a goroutine to refresh the version.
				// CrontabWorker 会启动一个 goroutine 来刷新版本信息。
				break
			}

			// Retry for error.
			// 如果查询失败，等待一段时间后重试。
			select {
			case <-ctx.Done(): // 如果主上下文被取消，则退出循环。
			case <-time.After(3 * time.Minute): // 每隔 3 分钟重试一次。
			}
		}
	}()

	// 等待主上下文或版本查询上下文完成。
	select {
	case <-ctx.Done(): // 主上下文被取消。
	case <-versionsCtx.Done(): // 版本查询上下文被取消。
	}

	return nil
}
