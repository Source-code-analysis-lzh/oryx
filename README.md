# ORYX

[![](https://img.shields.io/twitter/follow/srs_server?style=social)](https://twitter.com/srs_server)
[![](https://badgen.net/discord/members/bQUPDRqy79)](https://discord.gg/bQUPDRqy79)
[![](https://ossrs.net/wiki/images/wechat-badge4.svg)](https://ossrs.net/lts/zh-cn/contact#discussion)
[![](https://ossrs.net/wiki/images/do-btn-srs-125x20.svg)](https://marketplace.digitalocean.com/apps/srs)
[![](https://opencollective.com/srs-server/tiers/badge.svg)](https://opencollective.com/srs-server)

Oryx（原 SRS Stack），是一个开箱即用、一体化的开源视频解决方案，用于在云端或通过自建服务创建在线视频服务，包括直播和 WebRTC。

> 注意：我们将项目从 SRS Stack 更名为 Oryx，因为我们需要一个新名称来让 AI 助手区分 SRS 和 SRS Stack。AI 助手对 SRS 和 SRS Stack 感到困惑。

Oryx 让您轻松创建在线视频服务。它使用 Go、Reactjs、SRS、FFmpeg 和 WebRTC 构建，支持 RTMP、WebRTC、HLS、HTTP-FLV 和 SRT 等协议。它提供身份验证、多平台推流、录制、转码、虚拟直播活动、自动 HTTPS 和易用的 HTTP Open API 等功能。

Oryx 基于 SRS、FFmpeg、React.js 和 Go 构建，包含 Redis，并集成了 OpenAI 服务。它是一个为多种实用场景设计的媒体解决方案。

[![](https://ossrs.io/lts/en-us/img/Oryx-5-sd.png?v=1)](https://ossrs.io/lts/en-us/img/Oryx-5-hd.png)

> 注意：有关 Oryx 的更多详细信息，请访问以下[链接](https://www.figma.com/file/Ju5h2DZeJMzUtx5k7D0Oak/Oryx)。

## 使用方法

通过 Docker 运行 Oryx，然后在浏览器中打开 http://localhost：

```bash
docker run --restart always -d -it --name oryx -v $HOME/data:/data \
  -p 80:2022 -p 443:2443 -p 1935:1935 -p 8000:8000/udp -p 10080:10080/udp \
  ossrs/oryx:5
```

> 重要提示：请记得挂载 `/data` 卷，以避免容器重启时数据丢失。例如，如果您将 `/data` 挂载到 `$HOME/data`，所有数据将存储在 `$HOME/data` 文件夹中。请根据您所需的目录进行修改。

> 重要提示：要在浏览器中使用 WebRTC WHIP，请避免使用 localhost 或 127.0.0.1。请使用私有 IP（例如 https://192.168.3.85）、公共 IP（例如 https://136.12.117.13）或域名（例如 https://your-domain.com）。要设置 HTTPS，请参考[这篇文章](https://blog.ossrs.io/how-to-secure-srs-with-lets-encrypt-by-1-click-cb618777639f)。

> 注意：在中国，使用 `registry.cn-hangzhou.aliyuncs.com/ossrs/oryx:5` 以加速 Docker 拉取过程并确保语言设置正确。

Oryx 使用的端口：

* `80/tcp`：HTTP 端口，您也可以使用 `2022`，例如 `-p 2022:2022` 等。
* `443/tcp`：HTTPS 端口，您也可以使用 `2443`，例如 `-p 2443:2443` 等。
* `1935/tcp`：RTMP 端口，支持通过 RTMP 向 Oryx 推流。
* `8000/udp`：WebRTC UDP 端口，用于传输 WebRTC 媒体数据（如 RTP 数据包）。
* `10080/udp`：SRT UDP 端口，支持通过 SRT 协议推流。

您可以选择修改 Oryx 的挂载卷并将其指向不同的目录。

* `/data` 全局数据目录。
    * `.well-known` Let's Encrypt ACME 挑战的目录。
    * `config` 用于存储密码、srs/redis/nginx/prometheus 配置和 SSL 文件的 .env 文件。
    * `dvr` DVR 存储目录，保存 DVR 文件。
    * `lego` LEGO Let's Encrypt ACME 挑战目录。
    * `record` 录制存储目录，保存录制文件。
    * `redis` Redis 数据目录，存储推流密钥和录制配置。
    * `signals` 信号存储目录，保存信号文件。
    * `upload` 上传存储目录，保存上传文件。
    * `vlive` 虚拟直播存储目录，保存视频文件。
    * `transcript` 转录存储目录，保存转录文件。
    * `nginx-cache` Nginx 缓存存储目录，保存缓存文件。
    * `srs-s3-bucket` AWS S3 兼容存储的挂载目录。

您可以使用环境变量来修改设置。

* `MGMT_PASSWORD`：管理员密码。
* `REACT_APP_LOCALE`：UI 的国际化配置，`en` 或 `zh`，默认为 `en`。

> 注意：`MGMT_PASSWORD` 也保存在 `/data/config/.env` 中，您可以自行修改。

要访问其他环境变量，请参考[环境变量](DEVELOPER.md#environments)部分。

## 赞助

您是否需要我们提供额外帮助？通过成为 SRS 的赞助者或支持者，我们可以为您提供所需支持：

* 支持者：每月 5 美元，通过 Discord 提供在线文字聊天支持。
* 赞助者：每月 100 美元，提供在线会议支持，每月 1 次会议，时长 1 小时。

请访问 [OpenCollective](https://opencollective.com/srs-server) 成为支持者或赞助者，并通过 [Discord](https://discord.gg/bQUPDRqy79) 直接联系我们。我们目前为以下开发者提供支持：

[![](https://opencollective.com/srs-server/backers.svg?width=800&button=false)](https://opencollective.com/srs-server)

我们 SRS 旨在建立一个非盈利的开源社区，帮助全球开发者创建高质量的音视频流媒体和 RTC 平台，以支持您的业务。

## 常见问题

1. [英文 FAQ](https://ossrs.io/lts/en-us/faq-oryx)
1. [中文 FAQ](https://ossrs.net/lts/zh-cn/faq-oryx)

## 教程

- [x] 入门指南：[博客](https://blog.ossrs.io/how-to-setup-a-video-streaming-service-by-1-click-e9fe6f314ac6), [英文](https://ossrs.io/lts/en-us/docs/v6/doc/getting-started-stack), [中文](https://ossrs.net/lts/zh-cn/docs/v5/doc/getting-started-stack).
- [x] 支持 WordPress 插件：[博客](https://blog.ossrs.io/publish-your-srs-livestream-through-wordpress-ec18dfae7d6f), [英文](https://ossrs.io/lts/en-us/blog/WordPress-Plugin), [中文](https://ossrs.net/lts/zh-cn/blog/WordPress-Plugin) 或 [WordPress 插件](https://wordpress.org/plugins/srs-player).
- [x] 支持自动 HTTPS：[博客](https://blog.ossrs.io/how-to-secure-srs-with-lets-encrypt-by-1-click-cb618777639f), [英文](https://ossrs.io/lts/en-us/blog/Oryx-Tutorial), [中文](https://ossrs.net/lts/zh-cn/blog/Oryx-HTTPS).
- [x] 支持 aaPanel 在任何 Linux 上安装：[博客](https://blog.ossrs.io/how-to-setup-a-video-streaming-service-by-aapanel-9748ae754c8c), [英文](https://ossrs.io/lts/en-us/blog/BT-aaPanel), [中文](https://ossrs.net/lts/zh-cn/blog/BT-aaPanel).
- [x] 支持 DVR 到本地磁盘：[博客](https://blog.ossrs.io/how-to-record-live-streaming-to-mp4-file-2aa792c35b25), [英文](https://ossrs.io/lts/en-us/blog/Record-Live-Streaming), [中文](https://mp.weixin.qq.com/s/axN_TPo-Gk_H7CbdqUud6g).
- [x] 支持虚拟直播：[中文](https://mp.weixin.qq.com/s/I0Kmxtc24txpngO-PiR_tQ).
- [x] 支持 IP 摄像机流：[博客](https://blog.ossrs.io/easily-stream-your-rtsp-ip-camera-to-youtube-twitch-or-facebook-c078db917149), [英文](http://ossrs.io/lts/en-us/blog/Stream-IP-Camera-Events), [中文](https://ossrs.net/lts/zh-cn/blog/Stream-IP-Camera-Events).
- [x] 支持构建小型 [HLS 分发 CDN](https://github.com/ossrs/oryx/tree/main/scripts/nginx-hls-cdn) 通过 Nginx。
- [x] 支持直播：[中文](https://mp.weixin.qq.com/s/AKqVWIdk3SBD-6uiTMliyA).
- [x] 支持实时 SRT 流：[中文](https://mp.weixin.qq.com/s/HQb3gLRyJHHu56pnyHerxA).
- [x] 支持 DVR 到腾讯云存储或 VoD：[中文](https://mp.weixin.qq.com/s/UXR5EBKZ-LnthwKN_rlIjg).
- [x] 支持 Typecho 插件：[中文](https://github.com/ossrs/Typecho-Plugin-SrsPlayer).
- [x] 支持直播流转码：[博客](https://blog.ossrs.io/efficient-live-streaming-transcoding-for-reducing-bandwidth-and-saving-costs-39bd001af02d), [英文](https://ossrs.io/lts/en-us/blog/Live-Transcoding), [中文](https://ossrs.net/lts/zh-cn/blog/Live-Transcoding).
- [x] 支持语音转文字转录：[博客](https://blog.ossrs.io/revolutionizing-live-streams-with-ai-transcription-creating-accessible-multilingual-subtitles-1e902ab856bd), [英文](https://ossrs.io/lts/en-us/blog/live-streams-transcription), [中文](https://ossrs.net/lts/zh-cn/blog/live-streams-transcription).
- [x] 支持直播间的 AI 助手：[博客](https://blog.ossrs.io/transform-your-browser-into-a-personal-voice-driven-gpt-ai-assistant-with-srs-stack-13e28adf1e18), [英文](https://ossrs.io/lts/en-us/blog/browser-voice-driven-gpt), [中文](https://ossrs.net/lts/zh-cn/blog/live-streams-transcription)
- [x] 支持视频多语言配音：[博客](https://blog.ossrs.io/expand-your-global-reach-with-srs-stack-effortless-video-translation-and-dubbing-solutions-544e1db671c2), [英文](https://ossrs.io/lts/en-us/blog/browser-voice-driven-gpt), [中文](https://ossrs.net/lts/zh-cn/blog/live-streams-transcription)
- [x] 支持视频流的 OCR 识别：[博客](https://blog.ossrs.io/leveraging-openai-for-ocr-and-object-recognition-in-video-streams-using-oryx-e4d575d0ca1f), [英文](https://ossrs.io/lts/en-us/blog/ocr-video-streams), [中文](https://ossrs.net/lts/zh-cn/blog/ocr-video-streams)

更多使用场景正在开发中，请阅读[这篇文章](https://github.com/ossrs/srs/issues/2856#lighthouse)。

## 功能

我们正在开发的功能：

- [x] 支持身份验证和自动更新的管理界面。
- [x] 在 Docker 中运行 SRS，通过 Docker 和 SRS API 查询状态。
- [x] 支持通过 RTMP/WebRTC 推流，通过 RTMP/HTTP-FLV/HLS/WebRTC 播放。
- [x] SRS 容器使用 Docker 日志 `json-file` 并轮换日志。
- [x] 支持通过 SRT 进行高分辨率和实时（200~500 毫秒）直播。
- [x] 在 Docker 中运行 SRS 回调，通过 SRS 服务器进行回调。
- [x] 支持通过 SRT 推流，通过 RTMP/HTTP-FLV/HLS/WebRTC/SRT 播放。
- [x] 更改 Redis 端口并使用随机密码。
- [x] 支持与腾讯云 VoD 集成。
- [x] 支持多平台转推。
- [x] 支持 WordPress 插件：SrsPlayer。
- [x] 支持 aaPanel 在任何 Linux 上安装。
- [x] 支持 DVR 到本地磁盘。
- [x] 支持手动升级到最新版本。
- [x] 支持通过 Let's Encrypt 和 LEGO 实现 HTTPS。
- [x] 支持虚拟直播，将文件或其他资源转换为直播。
- [x] 支持自建 HLS CDN，服务 10k+ 观众。
- [x] 支持 Typecho 插件：Typecho-Plugin-SrsPlayer。
- [x] 支持 DVR 到腾讯云存储。
- [x] 支持从 IP 摄像机拉取 RTSP 并推流到 YouTube/Twitch/Facebook。
- [x] 支持通过 FFmpeg 进行直播流转码，参见 [#2869](https://github.com/ossrs/srs/issues/2869)。
- [x] 支持语音转文字转录。
- [x] 支持直播间的 AI 助手。
- [x] 支持视频多语言配音。
- [ ] 支持限制流媒体时长以控制费用。
- [ ] 支持通过 SRS 5.0 容器实现 GB28181。
- [ ] 支持 WebRTC 一对一视频聊天，参见 [#2857](https://github.com/ossrs/srs/issues/2857)。
- [ ] 支持 WebRTC 视频聊天室，参见 [#2924](https://github.com/ossrs/srs/issues/2924)。
- [ ] 支持一套开发者工具，参见 [#2891](https://github.com/ossrs/srs/issues/2891)。
- [ ] 收集管理界面和容器的日志。
- [ ] 停止、重启和升级容器。
- [ ] 支持 logrotate 管理日志。
- [ ] 增强 Prometheus API 的身份验证。
- [ ] 集成 Prometheus 和 node-exporter。

## 许可证

Oryx 是一个开源项目，遵循 [MIT](https://spdx.org/licenses/MIT.html) 许可证。

我们还使用了以下开源项目：

* [FFmpeg](https://ffmpeg.org/)：一个完整的跨平台解决方案，用于录制、转换和流式传输音视频。
* [Redis](https://redis.io/)：Redis 是一个内存数据存储，被数百万开发者用作缓存、矢量数据库、文档数据库、流处理引擎和消息代理。
* [youtube-dl](https://github.com/ytdl-org/youtube-dl)：一个命令行程序，用于从 YouTube.com 和其他视频网站下载视频。

我们使用的其他框架：

* [Reactjs](https://react.dev/)：用于 Web 和原生用户界面的库。
* [Go](https://golang.org/)：使用 Go 构建简单、安全、可扩展的系统。

## 开发者

有关开发和 API 架构的详细信息，请参考[环境变量](DEVELOPER.md)。

2022.11
