# 开发者指南

本指南面向开发者，涵盖了 OpenAPI、环境变量、资源、端口以及在 macOS 或 Docker 上进行开发的相关内容。

## 在 macOS 上进行开发

通过 Docker 启动 Redis 和 SRS，并显式设置候选地址：

```bash
docker stop redis 2>/dev/null || echo ok && docker rm -f redis srs 2>/dev/null &&
docker run --name redis --rm -it -v $HOME/data/redis:/data -p 6379:6379 -d redis:5.0 &&
touch platform/containers/data/config/srs.server.conf platform/containers/data/config/srs.vhost.conf &&
docker run --name srs --rm -it \
    -v $(pwd)/platform/containers/data/config:/usr/local/srs/containers/data/config \
    -v $(pwd)/platform/containers/conf/srs.release-mac.conf:/usr/local/srs/conf/docker.conf \
    -v $(pwd)/platform/containers/objs/nginx:/usr/local/srs/objs/nginx \
    -p 1935:1935 -p 1985:1985 -p 8080:8080 -p 8000:8000/udp -p 10080:10080/udp \
    -d ossrs/srs:5
```

> 注意：使用内网IP作为WebRTC的候选地址（candidate）进行设置。

> 注意：通过 docker stop redis srs 停止服务，注意我们停止redis是为了让它将数据保存到磁盘。

> 注意：你也可以通过 `(cd platform && ~/git/srs/trunk/objs/srs -c containers/conf/srs.release-local.conf)` 运行SRS。

> 注意：你可以通过 `--env CANDIDATE=$(ifconfig en0 |grep 'inet ' |awk '{print $2}')` 设置WebRTC的候选地址。

### 各个端口详解

- 1935(TCP)：RTMP 协议：默认用于接收 RTMP 推流（如 OBS、FFmpeg）和分发 RTMP 拉流，支持直播场景的实时传输
- 1985(TCP)：HTTP API：提供 RESTful 接口管理流媒体服务，如查询推流状态、触发 WebRTC 信令、鉴权等；WebRTC 信令：处理 WebRTC 的 SDP 协商（如 WHIP 推流和 WHEP 播放）
- 8080(TCP)：HTTP 服务：用于 HLS、HTTP-FLV 播放，以及 Web 管理界面访问；静态资源：托管播放器页面（如 rtc_player.html）和测试工具
- 8000(UDP)：WebRTC 协议：处理 WebRTC 的 RTP/RTCP 数据包（音视频流）
- 10080(UDP)：SRT UDP 端口，支持通过 SRT 协议推流。

运行平台后端，或在 GoLand 中运行：

```bash
(cd platform && go run .)
```

> 注意：如果不需要生成自签名证书，请将 `AUTO_SELF_SIGNED_CERTIFICATE` 设置为 `off`。

运行所有测试:

```bash
bash scripts/tools/secret.sh --output test/.env &&
(cd test && go test -timeout=1h -failfast -v --endpoint=http://localhost:2022 -init-self-signed-cert=true) &&
(cd test && go test -timeout=1h -failfast -v --endpoint=https://localhost:2443 -init-self-signed-cert=false)
```

运行平台的 React UI，或在 WebStorm 中运行：

```bash
(cd ui && npm install && npm start)
```

访问浏览器：http://localhost:3000

## 开发 Docker 镜像

构建 Docker 镜像：

```bash
docker rmi platform:latest 2>/dev/null || echo OK &&
docker build -t platform:latest -f Dockerfile . &&
docker save -o platform.tar platform:latest
```

启动容器：

```bash
docker stop redis 2>/dev/null || echo ok && docker rm -f redis srs 2>/dev/null &&
docker run --rm -it --name oryx -v $HOME/data:/data \
  -p 2022:2022 -p 2443:2443 -p 1935:1935 -p 8000:8000/udp -p 10080:10080/udp \
  -p 80:2022 -p 443:2443 -e CANDIDATE=$(ifconfig en0 |grep 'inet ' |awk '{print $2}') \
  platform
```

访问 [http://localhost/mgmt](http://localhost/mgmt) 来管理 Oryx。

或者访问 [http://srs.stack.local/mgmt](http://srs.stack.local/mgmt) 来使用域名测试 Oryx。

更新 Docker 中的平台：

```bash
docker run --rm -it -v $(pwd)/platform:/g -w /g ossrs/srs:ubuntu20 make
```

使用新平台启动容器：

```bash
docker stop redis 2>/dev/null || echo ok && docker rm -f redis srs 2>/dev/null &&
docker run --rm -it --name oryx -v $HOME/data:/data \
  -p 2022:2022 -p 2443:2443 -p 1935:1935 -p 8000:8000/udp -p 10080:10080/udp \
  -p 80:2022 -p 443:2443 -e CANDIDATE=$(ifconfig en0 |grep 'inet ' |awk '{print $2}') \
  -v $(pwd)/platform/platform:/usr/local/oryx/platform/platform \
  platform
```

平台现已更新。它也兼容调试用的 WebUI，该 WebUI 监听端口 3000 并代理到平台。

## 开发脚本安装程序

> 注意：请注意 BT 插件将使用当前分支版本，包括开发版本。

构建 Docker 镜像：

```bash
docker rm -f script 2>/dev/null &&
docker rmi srs-script-dev 2>/dev/null || echo OK &&
docker build -t srs-script-dev -f scripts/setup-ubuntu/Dockerfile.script .
```

以守护进程模式创建 Docker 容器：

```bash
docker rm -f script 2>/dev/null &&
docker run -p 2022:2022 -p 2443:2443 -p 1935:1935 -p 8000:8000/udp -p 10080:10080/udp \
    --env CANDIDATE=$(ifconfig en0 |grep 'inet ' |awk '{print $2}') \
    --privileged -v /sys/fs/cgroup:/sys/fs/cgroup:rw --cgroupns=host \
    -d --rm -it -v $(pwd):/g -w /g --name=script srs-script-dev
```

> 注意：对于 Linux 服务器，请使用 `--privileged -v /sys/fs/cgroup:/sys/fs/cgroup:ro` 来启动 Docker。

构建并将脚本镜像保存到文件：

```bash
docker rmi platform:latest 2>/dev/null || echo OK &&
docker build -t platform:latest -f Dockerfile . &&
docker save -o platform.tar platform:latest
```

进入 Docker 容器：

```bash
version=$(bash scripts/version.sh) &&
docker exec -it script docker load -i platform.tar && 
docker exec -it script docker tag platform:latest ossrs/oryx:$version &&
docker exec -it script docker tag platform:latest registry.cn-hangzhou.aliyuncs.com/ossrs/oryx:$version &&
docker exec -it script docker images
```

在 Docker 容器中测试构建脚本：

```bash
docker exec -it script rm -f /data/config/.env &&
docker exec -it script bash build/oryx/scripts/setup-ubuntu/uninstall.sh 2>/dev/null || echo OK &&
bash scripts/setup-ubuntu/build.sh --output $(pwd)/build --extract &&
docker exec -it script bash build/oryx/scripts/setup-ubuntu/install.sh --verbose
```

运行脚本测试：

```bash
rm -f test/oryx.test &&
docker exec -it script make -j -C test &&
bash scripts/tools/secret.sh --output test/.env && sleep 5 &&
docker exec -it script ./test/oryx.test -test.timeout=1h -test.failfast -test.v -endpoint http://localhost:2022 \
    -srs-log=true -wait-ready=true -init-password=true -check-api-secret=true -init-self-signed-cert=true \
    -test.run TestSystem_Empty &&
bash scripts/tools/secret.sh --output test/.env && sleep 5 &&
docker exec -it script ./test/oryx.test -test.timeout=1h -test.failfast -test.v -wait-ready -endpoint http://localhost:2022 \
    -srs-log=true -wait-ready=true -init-password=false -check-api-secret=true \
    -test.parallel 3 &&
docker exec -it script ./test/oryx.test -test.timeout=1h -test.failfast -test.v -wait-ready -endpoint https://localhost:2443 \
    -srs-log=true -wait-ready=true -init-password=false -check-api-secret=true \
    -test.parallel 3
```

访问浏览器: [http://localhost:2022](http://localhost:2022)

## Develop the aaPanel Plugin

> Note: Please note that BT plugin will use the current branch version, including develop version.

Start a container and mount as plugin:

```bash
docker rm -f bt aapanel 2>/dev/null &&
AAPANEL_KEY=$(cat $HOME/.bt/api.json |awk -F token_crypt '{print $2}' |cut -d'"' -f3) &&
docker run -p 80:80 -p 443:443 -p 7800:7800 -p 1935:1935 -p 8000:8000/udp -p 10080:10080/udp \
    --env CANDIDATE=$(ifconfig en0 |grep 'inet ' |awk '{print $2}') \
    -v $(pwd)/build/oryx:/www/server/panel/plugin/oryx \
    -v $HOME/.bt/api.json:/www/server/panel/config/api.json -e BT_KEY=$AAPANEL_KEY \
    --privileged -v /sys/fs/cgroup:/sys/fs/cgroup:rw --cgroupns=host \
    -d --rm -it -v $(pwd):/g -w /g --name=aapanel ossrs/aapanel-plugin-dev:1
```

> Note: For Linux server, please use `--privileged -v /sys/fs/cgroup:/sys/fs/cgroup:ro` to start docker.

> Note: Enable the [HTTP API](https://www.bt.cn/bbs/thread-20376-1-1.html) and get the `api.json`,
> and save it to `$HOME/.bt/api.json`.

Build and save the platform image to file:

```bash
docker rmi platform:latest 2>/dev/null || echo OK &&
docker build -t platform:latest -f Dockerfile . &&
docker save -o platform.tar platform:latest
```

Enter the docker container:

```bash
version=$(bash scripts/version.sh) &&
major=$(echo $version |awk -F '.' '{print $1}' |sed 's/v//g') &&
docker exec -it aapanel docker load -i platform.tar && 
docker exec -it aapanel docker tag platform:latest ossrs/oryx:$version &&
docker exec -it aapanel docker tag platform:latest ossrs/oryx:$major &&
docker exec -it aapanel docker tag platform:latest registry.cn-hangzhou.aliyuncs.com/ossrs/oryx:$version &&
docker exec -it aapanel docker tag platform:latest registry.cn-hangzhou.aliyuncs.com/ossrs/oryx:$major &&
docker exec -it aapanel docker images
```

Next, build the aaPanel plugin and install it:

```bash
docker exec -it aapanel rm -f /data/config/.env &&
docker exec -it aapanel bash /www/server/panel/plugin/oryx/install.sh uninstall 2>/dev/null || echo OK &&
bash scripts/setup-aapanel/auto/zip.sh --output $(pwd)/build --extract &&
docker exec -it aapanel bash /www/server/panel/plugin/oryx/install.sh install
```

You can use aaPanel panel to install the plugin, or by command:

```bash
docker exec -it aapanel python3 /www/server/panel/plugin/oryx/bt_api_remove_site.py &&
docker exec -it aapanel python3 /www/server/panel/plugin/oryx/bt_api_create_site.py &&
docker exec -it aapanel python3 /www/server/panel/plugin/oryx/bt_api_setup_site.py &&
docker exec -it aapanel bash /www/server/panel/plugin/oryx/setup.sh \
    --r0 /tmp/oryx_install.r0 --nginx /www/server/nginx/logs/nginx.pid \
    --www /www/wwwroot --site srs.stack.local
```

Setup the dns lookup for domain `srs.stack.local`:

```bash
PIP=$(docker exec -it aapanel ifconfig eth0 |grep 'inet ' |awk '{print $2}') &&
docker exec -it aapanel bash -c "echo '$PIP srs.stack.local' >> /etc/hosts" &&
docker exec -it aapanel cat /etc/hosts && echo OK &&
docker exec -it aapanel docker exec -it oryx bash -c "echo '$PIP srs.stack.local' >> /etc/hosts" &&
docker exec -it aapanel docker exec -it oryx cat /etc/hosts
```
> Note: We add host `srs.stack.local` to the ip of eth0, because we need to access it in the
> oryx docker in docker.

Run test for aaPanel:

```bash
rm -f test/oryx.test &&
docker exec -it aapanel make -j -C test &&
bash scripts/tools/secret.sh --output test/.env && sleep 5 &&
docker exec -it aapanel ./test/oryx.test -test.timeout=1h -test.failfast -test.v -endpoint http://srs.stack.local:80 \
    -srs-log=true -wait-ready=true -init-password=true -check-api-secret=true -init-self-signed-cert=true \
    -test.run TestSystem_Empty &&
bash scripts/tools/secret.sh --output test/.env && sleep 5 &&
docker exec -it aapanel ./test/oryx.test -test.timeout=1h -test.failfast -test.v -wait-ready -endpoint http://srs.stack.local:80 \
    -srs-log=true -wait-ready=true -init-password=false -check-api-secret=true \
    -test.parallel 3 &&
docker exec -it aapanel ./test/oryx.test -test.timeout=1h -test.failfast -test.v -wait-ready -endpoint https://srs.stack.local:443 \
    -srs-log=true -wait-ready=true -init-password=false -check-api-secret=true \
    -test.parallel 3
```

Open [http://localhost:7800/srsstack](http://localhost:7800/srsstack) to install plugin.

> Note: Or you can use `docker exec -it aapanel bt default` to show the login info.

In the [application store](http://localhost:7800/soft), there is a `oryx` plugin. After test, you can install the plugin
`build/aapanel-oryx.zip` to production aaPanel panel.

## Develop the BT Plugin

> Note: Please note that BT plugin will use the current branch version, including develop version.

Start a container and mount as plugin:

```bash
docker rm -f bt aapanel 2>/dev/null &&
BT_KEY=$(cat $HOME/.bt/api.json |awk -F token_crypt '{print $2}' |cut -d'"' -f3) &&
docker run -p 80:80 -p 443:443 -p 7800:7800 -p 1935:1935 -p 8000:8000/udp -p 10080:10080/udp \
    --env CANDIDATE=$(ifconfig en0 |grep 'inet ' |awk '{print $2}') \
    -v $(pwd)/build/oryx:/www/server/panel/plugin/oryx \
    -v $HOME/.bt/userInfo.json:/www/server/panel/data/userInfo.json \
    -v $HOME/.bt/api.json:/www/server/panel/config/api.json -e BT_KEY=$BT_KEY \
    --privileged -v /sys/fs/cgroup:/sys/fs/cgroup:rw --cgroupns=host \
    -d --rm -it -v $(pwd):/g -w /g --name=bt ossrs/bt-plugin-dev:1
```

> Note: For Linux server, please use `--privileged -v /sys/fs/cgroup:/sys/fs/cgroup:ro` to start docker.

> Note: Should bind the docker to your BT account, then you will get the `userInfo.json`, 
> and save it to `$HOME/.bt/userInfo.json`.

> Note: Enable the [HTTP API](https://www.bt.cn/bbs/thread-20376-1-1.html) and get the `api.json`, 
> and save it to `$HOME/.bt/api.json`.

Build and save the platform image to file:

```bash
docker rmi platform:latest 2>/dev/null || echo OK &&
docker build -t platform:latest -f Dockerfile . &&
docker save -o platform.tar platform:latest
```

Enter the docker container:

```bash
version=$(bash scripts/version.sh) &&
major=$(echo $version |awk -F '.' '{print $1}' |sed 's/v//g') &&
docker exec -it bt docker load -i platform.tar && 
docker exec -it bt docker tag platform:latest ossrs/oryx:$version &&
docker exec -it bt docker tag platform:latest ossrs/oryx:$major &&
docker exec -it bt docker tag platform:latest registry.cn-hangzhou.aliyuncs.com/ossrs/oryx:$version &&
docker exec -it bt docker tag platform:latest registry.cn-hangzhou.aliyuncs.com/ossrs/oryx:$major &&
docker exec -it bt docker images
```

Next, build the BT plugin and install it:

```bash
docker exec -it bt bash /www/server/panel/plugin/oryx/install.sh uninstall 2>/dev/null || echo OK &&
docker exec -it bt rm -f /data/config/.env &&
bash scripts/setup-bt/auto/zip.sh --output $(pwd)/build --extract &&
docker exec -it bt bash /www/server/panel/plugin/oryx/install.sh install
```

You can use BT panel to install the plugin, or by command:

```bash
docker exec -it bt python3 /www/server/panel/plugin/oryx/bt_api_remove_site.py &&
docker exec -it bt python3 /www/server/panel/plugin/oryx/bt_api_create_site.py &&
docker exec -it bt python3 /www/server/panel/plugin/oryx/bt_api_setup_site.py &&
docker exec -it bt bash /www/server/panel/plugin/oryx/setup.sh \
    --r0 /tmp/oryx_install.r0 --nginx /www/server/nginx/logs/nginx.pid \
    --www /www/wwwroot --site srs.stack.local
```

Setup the dns lookup for domain `srs.stack.local`:

```bash
PIP=$(docker exec -it bt ifconfig eth0 |grep 'inet ' |awk '{print $2}') &&
docker exec -it bt bash -c "echo '$PIP srs.stack.local' >> /etc/hosts" &&
docker exec -it bt cat /etc/hosts && echo OK &&
docker exec -it bt docker exec -it oryx bash -c "echo '$PIP srs.stack.local' >> /etc/hosts" &&
docker exec -it bt docker exec -it oryx cat /etc/hosts
```
> Note: We add host `srs.stack.local` to the ip of eth0, because we need to access it in the
> oryx docker in docker.

Run test for BT:

```bash
rm -f test/oryx.test &&
docker exec -it bt make -j -C test &&
bash scripts/tools/secret.sh --output test/.env && sleep 5 &&
docker exec -it bt ./test/oryx.test -test.timeout=1h -test.failfast -test.v -endpoint http://srs.stack.local:80 \
    -srs-log=true -wait-ready=true -init-password=true -check-api-secret=true -init-self-signed-cert=true \
    -test.run TestSystem_Empty &&
bash scripts/tools/secret.sh --output test/.env && sleep 5 &&
docker exec -it bt ./test/oryx.test -test.timeout=1h -test.failfast -test.v -wait-ready -endpoint http://srs.stack.local:80 \
    -srs-log=true -wait-ready=true -init-password=false -check-api-secret=true \
    -test.parallel 3 &&
docker exec -it bt ./test/oryx.test -test.timeout=1h -test.failfast -test.v -wait-ready -endpoint https://srs.stack.local:443 \
    -srs-log=true -wait-ready=true -init-password=false -check-api-secret=true \
    -test.parallel 3
```

Open [http://localhost:7800/srsstack](http://localhost:7800/srsstack) to install plugin.

> Note: Or you can use `docker exec -it bt bt default` to show the login info.

In the [application store](http://localhost:7800/soft), there is a `oryx` plugin. After test, you can install the plugin 
`build/bt-oryx.zip` to production BT panel.

## Develop the Droplet Image

> Note: Please note that BT plugin will use the current branch version, including develop version.

To build SRS droplet image for [DigitalOcean Marketplace](https://marketplace.digitalocean.com/).

For the first run, please [install Packer](https://www.packer.io/intro/getting-started/install.html) and 
[plugin](https://developer.hashicorp.com/packer/integrations/digitalocean/digitalocean):

```bash
brew tap hashicorp/tap &&
brew install hashicorp/tap/packer &&
PACKER_LOG=1 packer plugins install github.com/digitalocean/digitalocean
```

Start to build SRS image by:

```bash
(export DIGITALOCEAN_TOKEN=$(grep market "${HOME}/Library/Application Support/doctl/config.yaml" |grep -v context |awk '{print $2}') &&
cd scripts/setup-droplet && packer build srs.json)
```

> Note: You can also create a [token](https://cloud.digitalocean.com/account/api/tokens) and setup the env `DIGITALOCEAN_TOKEN`.

Please check the [snapshot](https://cloud.digitalocean.com/images/snapshots/droplets), and create a test droplet.

```bash
IMAGE=$(doctl compute snapshot list --context market --format ID --no-header) &&
sshkey=$(doctl compute ssh-key list --context market --no-header |grep srs |awk '{print $1}') &&
doctl compute droplet create oryx-test --context market --image $IMAGE \
    --region sgp1 --size s-2vcpu-2gb --ssh-keys $sshkey --wait &&
SRS_DROPLET_EIP=$(doctl compute droplet get oryx-test --context market --format PublicIPv4 --no-header)
```

Prepare test environment:

```bash
ssh root@$SRS_DROPLET_EIP sudo mkdir -p /data/upload test scripts/tools &&
ssh root@$SRS_DROPLET_EIP sudo chmod 777 /data/upload &&
cp ~/git/srs/trunk/doc/source.200kbps.768x320.flv test/ &&
scp ./test/source.200kbps.768x320.flv root@$SRS_DROPLET_EIP:/data/upload/ &&
docker run --rm -it -v $(pwd):/g -w /g -e CGO_ENABLED=0 -e GOOS=linux -e GOARCH=amd64 \
    ossrs/srs:ubuntu20 make -C test clean default &&
scp ./test/oryx.test ./test/source.200kbps.768x320.flv root@$SRS_DROPLET_EIP:~/test/ &&
scp ./scripts/tools/secret.sh root@$SRS_DROPLET_EIP:~/scripts/tools &&
ssh root@$SRS_DROPLET_EIP docker run --rm -v /usr/bin:/g ossrs/srs:tools \
    cp /usr/local/bin/ffmpeg /usr/local/bin/ffprobe /g/
```

> Note: By setting `CGO_ENABLED=0 GOOS=linux GOARCH=amd64`, we always build for x86_64 linux platform, 
> because we only use the DigitalOcean VPS with x86_64.

Test the droplet instance:

```bash
ssh root@$SRS_DROPLET_EIP bash scripts/tools/secret.sh --output test/.env && sleep 5 &&
ssh root@$SRS_DROPLET_EIP ./test/oryx.test -test.timeout=1h -test.failfast -test.v -endpoint http://$SRS_DROPLET_EIP:2022 \
    -srs-log=true -wait-ready=true -init-password=true -check-api-secret=true -init-self-signed-cert=true \
    -test.run TestSystem_Empty &&
ssh root@$SRS_DROPLET_EIP bash scripts/tools/secret.sh --output test/.env && sleep 5 &&
ssh root@$SRS_DROPLET_EIP ./test/oryx.test -test.timeout=1h -test.failfast -test.v -wait-ready -endpoint http://$SRS_DROPLET_EIP:2022 \
    -endpoint-rtmp rtmp://$SRS_DROPLET_EIP -endpoint-http http://$SRS_DROPLET_EIP -endpoint-srt srt://$SRS_DROPLET_EIP:10080 \
    -srs-log=true -wait-ready=true -init-password=false -check-api-secret=true \
    -test.parallel 1
```

Remove the droplet instance:

```bash
doctl compute droplet delete oryx-test --context market --force
```

After submit to [marketplace](https://cloud.digitalocean.com/vendorportal/624145d53da4ad68de259945/10/edit), cleanup the snapshot:

```bash
IMAGE=$(doctl compute snapshot list --context market --format ID --no-header) &&
doctl compute snapshot delete $IMAGE --context market --force
```

> Note: The snapshot should be removed if submit to marketplace, so you don't need to delete it.

## Develop the Lighthouse Image

> Note: Please note that BT plugin will use the current branch version, including develop version.

To build SRS image for [TencentCloud Lighthouse](https://cloud.tencent.com/product/lighthouse).

For the first run, please create a [TencentCloud Secret](https://console.cloud.tencent.com/cam/capi) and save
to `~/.lighthouse/.env` file:

```bash
LH_ACCOUNT=xxxxxx
LH_PROD=xxxxxx
SECRET_ID=xxxxxx
SECRET_KEY=xxxxxx
```

> Note: Share the image to `LH_ACCOUNT` to publish it.

Create a CVM instance:

```bash
rm -f .tmp/lh-*.txt &&
echo "$(openssl rand -base64 32 | tr -dc 'a-zA-Z0-9' | head -c 16)A0" >.tmp/lh-token.txt &&
VM_TOKEN=$(cat .tmp/lh-token.txt) bash scripts/tools/tencent-cloud/helper.sh create-cvm.py --id $(pwd)/.tmp/lh-instance.txt &&
bash scripts/tools/tencent-cloud/helper.sh query-cvm-ip.py --instance $(cat .tmp/lh-instance.txt) --id $(pwd)/.tmp/lh-ip.txt &&
echo "Instance: $(cat .tmp/lh-instance.txt), IP: ubuntu@$(cat .tmp/lh-ip.txt), Password: $(cat .tmp/lh-token.txt)" && sleep 5 &&
bash scripts/setup-lighthouse/build.sh --ip $(cat .tmp/lh-ip.txt) --os ubuntu --user ubuntu --password $(cat .tmp/lh-token.txt) &&
bash scripts/tools/tencent-cloud/helper.sh create-image.py --instance $(cat .tmp/lh-instance.txt) --id $(pwd)/.tmp/lh-image.txt &&
bash scripts/tools/tencent-cloud/helper.sh share-image.py --image $(cat .tmp/lh-image.txt) &&
echo "Image: $(cat .tmp/lh-image.txt) created and shared." &&
bash scripts/tools/tencent-cloud/helper.sh remove-cvm.py --instance $(cat .tmp/lh-instance.txt)
```

Next, create a test CVM instance with the image:

```bash
echo "$(openssl rand -base64 32 | tr -dc 'a-zA-Z0-9' | head -c 16)A0" >.tmp/lh-token2.txt &&
VM_TOKEN=$(cat .tmp/lh-token2.txt) bash scripts/tools/tencent-cloud/helper.sh create-verify.py --image $(cat .tmp/lh-image.txt) --id $(pwd)/.tmp/lh-test.txt &&
bash scripts/tools/tencent-cloud/helper.sh query-cvm-ip.py --instance $(cat .tmp/lh-test.txt) --id $(pwd)/.tmp/lh-ip2.txt && 
echo "IP: ubuntu@$(cat .tmp/lh-ip2.txt), Password: $(cat .tmp/lh-token2.txt)" &&
echo "http://$(cat .tmp/lh-ip2.txt)"
```

Prepare test environment:

```bash
sshCmd="sshpass -p $(cat .tmp/lh-token2.txt) ssh -o StrictHostKeyChecking=no -t" &&
scpCmd="sshpass -p $(cat .tmp/lh-token2.txt) scp -o StrictHostKeyChecking=no" &&
$sshCmd ubuntu@$(cat .tmp/lh-ip2.txt) sudo mkdir -p /data/upload &&
$sshCmd ubuntu@$(cat .tmp/lh-ip2.txt) mkdir -p test scripts/tools &&
$sshCmd ubuntu@$(cat .tmp/lh-ip2.txt) sudo chmod 777 /data/upload &&
cp ~/git/srs/trunk/doc/source.200kbps.768x320.flv test/ &&
$scpCmd test/source.200kbps.768x320.flv ubuntu@$(cat .tmp/lh-ip2.txt):/data/upload/ &&
docker run --rm -it -v $(pwd):/g -w /g ossrs/srs:ubuntu20 make -C test clean default &&
$scpCmd ./test/oryx.test ./test/source.200kbps.768x320.flv ubuntu@$(cat .tmp/lh-ip2.txt):~/test/ &&
$scpCmd ./scripts/tools/secret.sh ubuntu@$(cat .tmp/lh-ip2.txt):~/scripts/tools &&
$sshCmd ubuntu@$(cat .tmp/lh-ip2.txt) sudo docker run --rm -v /usr/bin:/g \
    registry.cn-hangzhou.aliyuncs.com/ossrs/srs:tools \
    cp /usr/local/bin/ffmpeg /usr/local/bin/ffprobe /g/
```

Test the CVM instance:

```bash
$sshCmd ubuntu@$(cat .tmp/lh-ip2.txt) sudo bash scripts/tools/secret.sh --output test/.env && sleep 5 &&
$sshCmd ubuntu@$(cat .tmp/lh-ip2.txt) ./test/oryx.test -test.timeout=1h -test.failfast -test.v -endpoint http://$(cat .tmp/lh-ip2.txt):2022 \
    -srs-log=true -wait-ready=true -init-password=true -check-api-secret=true -init-self-signed-cert=true \
    -test.run TestSystem_Empty &&
$sshCmd ubuntu@$(cat .tmp/lh-ip2.txt) sudo bash scripts/tools/secret.sh --output test/.env && sleep 5 &&
$sshCmd ubuntu@$(cat .tmp/lh-ip2.txt) ./test/oryx.test -test.timeout=1h -test.failfast -test.v -wait-ready -endpoint http://$(cat .tmp/lh-ip2.txt):2022 \
    -endpoint-rtmp rtmp://$(cat .tmp/lh-ip2.txt) -endpoint-http http://$(cat .tmp/lh-ip2.txt) -endpoint-srt srt://$(cat .tmp/lh-ip2.txt):10080 \
    -srs-log=true -wait-ready=true -init-password=false -check-api-secret=true \
    -test.parallel 3 &&
ssh ubuntu@$(cat .tmp/lh-ip2.txt) ./test/oryx.test -test.timeout=1h -test.failfast -test.v -wait-ready -endpoint https://$(cat .tmp/lh-ip2.txt):2443 \
    -endpoint-rtmp rtmp://$(cat .tmp/lh-ip2.txt) -endpoint-http https://$(cat .tmp/lh-ip2.txt) -endpoint-srt srt://$(cat .tmp/lh-ip2.txt):10080 \
    -srs-log=true -wait-ready=true -init-password=false -check-api-secret=true \
    -test.parallel 3
```

Verify then cleanup the test CVM instance:

```bash
bash scripts/tools/tencent-cloud/helper.sh remove-cvm.py --instance $(cat .tmp/lh-test.txt)
```

After publish to lighthouse, cleanup the CVM, disk images, and snapshots:

```bash
bash scripts/tools/tencent-cloud/helper.sh remove-image.py --image $(cat .tmp/lh-image.txt)
```

If need to test the domain of lighthouse, create a domain `lighthouse.ossrs.net`:

```bash
# Create the test domain for lighthouse
doctl compute domain records create ossrs.net \
    --record-type A --record-name lighthouse --record-data $(cat .tmp/lh-ip2.txt) \
    --record-ttl 300 &&
echo "https://lighthouse.ossrs.net"

# Remove the test domain for lighthouse
doctl compute domain records delete ossrs.net -f \
    $(doctl compute domain records list ossrs.net --no-header |grep lighthouse |awk '{print $1}') &&
echo "Record lighthouse.ossrs.net removed"
```

## 开发 HTTPS 的 SSL 证书

为 HTTPS 创建域名：

```bash
LNAME=lego && LDOMAIN=ossrs.net &&
doctl compute domain records create $LDOMAIN \
    --record-type A --record-name $LNAME --record-data $(dig +short $LDOMAIN) \
    --record-ttl 3600
```

> 注意：1. 定义变量 LNAME=lego 和 LDOMAIN=ossrs.net。 2. 使用 doctl 在 DigitalOcean 中为域名 ossrs.net 创建一条 A 记录。 3. 子域名为 lego，解析到与 ossrs.net 相同的 IP 地址。 4. 设置 TTL 为 1 小时。

构建并将脚本镜像保存到文件：

```bash
docker rmi platform:latest 2>/dev/null || echo OK &&
docker build -t platform:latest -f Dockerfile . &&
docker save -o platform.tar platform:latest
```

复制并加载镜像到服务器：

```bash
ssh root@$LNAME.$LDOMAIN rm -f platform.tar* 2>/dev/null &&
rm -f platform.tar.gz && tar zcf platform.tar.gz platform.tar &&
scp platform.tar.gz root@$LNAME.$LDOMAIN:~ &&
ssh root@$LNAME.$LDOMAIN tar xf platform.tar.gz &&
version=$(bash scripts/version.sh) &&
ssh root@$LNAME.$LDOMAIN docker load -i platform.tar &&
ssh root@$LNAME.$LDOMAIN docker tag platform:latest ossrs/oryx:$version &&
ssh root@$LNAME.$LDOMAIN docker tag platform:latest registry.cn-hangzhou.aliyuncs.com/ossrs/oryx:$version &&
ssh root@$LNAME.$LDOMAIN docker image prune -f &&
ssh root@$LNAME.$LDOMAIN docker images
```

接下来，构建 BT 插件并安装它：

```bash
ssh root@$LNAME.$LDOMAIN bash /www/server/panel/plugin/oryx/install.sh uninstall 2>/dev/null || echo OK &&
bash scripts/setup-bt/auto/zip.sh --output $(pwd)/build --extract &&
scp build/bt-oryx.zip root@$LNAME.$LDOMAIN:~ &&
ssh root@$LNAME.$LDOMAIN unzip -q bt-oryx.zip -d /www/server/panel/plugin &&
ssh root@$LNAME.$LDOMAIN bash /www/server/panel/plugin/oryx/install.sh install
```

在服务器上，设置 `.bashrc`：

```bash
export BT_KEY=xxxxxx
export PYTHONIOENCODING=UTF-8
```

你可以使用 BT 面板安装插件，或通过命令安装：

```bash
ssh root@$LNAME.$LDOMAIN python3 /www/server/panel/plugin/oryx/bt_api_remove_site.py &&
ssh root@$LNAME.$LDOMAIN DOMAIN=$LNAME.$LDOMAIN python3 /www/server/panel/plugin/oryx/bt_api_create_site.py &&
ssh root@$LNAME.$LDOMAIN python3 /www/server/panel/plugin/oryx/bt_api_setup_site.py &&
ssh root@$LNAME.$LDOMAIN bash /www/server/panel/plugin/oryx/setup.sh \
    --r0 /tmp/oryx_install.r0 --nginx /www/server/nginx/logs/nginx.pid \
    --www /www/wwwroot --site srs.stack.local
```

清理，删除文件和域名：

```bash
ssh root@$LNAME.$LDOMAIN rm -f platform.tar* bt-oryx.zip 2>/dev/null &&
ssh root@$LNAME.$LDOMAIN python3 /www/server/panel/plugin/oryx/bt_api_remove_site.py &&
ssh root@$LNAME.$LDOMAIN bash /www/server/panel/plugin/oryx/install.sh uninstall 2>/dev/null || echo OK &&
domains=$(doctl compute domain records ls $LDOMAIN --no-header |grep $LNAME) && echo "Cleanup domains: $domains" &&
doctl compute domain records delete $LDOMAIN $(echo $domains |awk '{print $1}') -f
```

查询域名和 droplet：

```bash
doctl compute domain records ls ossrs.io |grep lego &&
doctl compute droplet ls |grep lego
```

## 开发 NGINX HLS CDN

按照之前的步骤运行 Oryx，例如 [在 macOS 中开发所有功能](https://www.wenxiaobai.com/chat/200006#)，发布流后应该会有一个 HLS 流：

* [http://localhost:2022/live/livestream.m3u8](http://localhost:2022/tools/player.html?url=http://localhost:2022/live/livestream.m3u8)

构建 NGINX 的镜像：

```bash
docker rm -f nginx 2>/dev/null &&
docker rmi scripts/nginx-hls-cdn 2>/dev/null || echo OK &&
docker build -t ossrs/oryx:nginx-hls-cdn scripts/nginx-hls-cdn
```

> 注意：官方镜像由 [workflow](https://github.com/ossrs/oryx/actions/runs/5970907929) 构建，该流程是手动触发的。

如果你想使用 NGINX 作为代理，通过 docker 运行：

```bash
ORYX_SERVER=$(ifconfig en0 |grep 'inet ' |awk '{print $2}') &&
docker run --rm -it -p 80:80 --name nginx -e ORYX_SERVER=${ORYX_SERVER}:2022 \
    ossrs/oryx:nginx-hls-cdn
```

此时应该会有一个新的 HLS 流，由 NGINX 缓存：

* [http://localhost/live/livestream.m3u8](http://localhost:2022/tools/player.html?url=http://localhost/live/livestream.m3u8)

要测试带有 `OPTIONS` 的 CROS，可以使用 [HTTP-REST](http://ossrs.net/http-rest/) 工具，或通过 curl：

```bash
curl 'http://localhost/live/livestream.m3u8' -X 'OPTIONS' -H 'Origin: http://ossrs.net' -v
curl 'http://localhost/live/livestream.m3u8' -X 'GET' -H 'Origin: http://ossrs.net' -v
```

要启动 [srs-bench](https://github.com/ossrs/srs-bench) 来测试性能：

```bash
docker run --rm -d ossrs/srs:sb ./objs/sb_hls_load \
    -c 100 -r http://host.docker.internal/live/livestream.m3u8
```

负载应该由 NGINX 承担，而不是 Oryx。

## 生产环境中的 NGINX HLS CDN

通过 BT 或 aaPanel 或 docker 安装 Oryx，假设域名为 `bt.ossrs.net`，发布一个 RTMP 流到 Oryx：

```bash
ffmpeg -re -i ~/git/srs/trunk/doc/source.flv -c copy \
    -f flv rtmp://bt.ossrs.net/live/livestream?secret=xxx
```

打开 [http://bt.ossrs.net/live/livestream.m3u8](http://bt.ossrs.net/tools/player.html?url=http://bt.ossrs.net/live/livestream.m3u8) 进行检查。

为同一服务器创建一个新域名 `bt2.ossrs.net`：

```bash
doctl compute domain records create ossrs.net \
    --record-type A --record-name bt2 --record-data 39.100.79.15 \
    --record-ttl 3600
```

通过 BT 或 aaPanel 创建一个新网站 `bt2.ossrs.net`，代理到 NGINX HLS 边缘服务器：

```nginx
location /tools/ {
  proxy_pass http://localhost:2022;
}
location / {
  proxy_no_cache 1;
  proxy_cache_bypass 1;
  add_header X-Cache-Status-Proxy $upstream_cache_status;
  proxy_pass http://localhost:23080;
}
```

启动 NGINX HLS 边缘服务器：

```bash
docker rm -f oryx-nginx01 || echo OK &&
PIP=$(ifconfig eth0 |grep 'inet ' |awk '{print $2}') &&
docker run --rm -it -e ORYX_SERVER=$PIP:2022 \
    -p 23080:80 --name oryx-nginx01 -d \
    ossrs/oryx:nginx-hls-cdn
```

打开 [http://bt2.ossrs.net/live/livestream.m3u8](http://bt.ossrs.net/tools/player.html?url=http://bt2.ossrs.net/live/livestream.m3u8) 进行检查。

使用 curl 测试 HLS 缓存：

```bash
curl -v http://bt.ossrs.net/live/livestream.m3u8
curl -v http://bt.ossrs.net/live/livestream.m3u8 -H 'Origin: http://test.com'
curl -v http://bt2.ossrs.net/live/livestream.m3u8
curl -v http://bt2.ossrs.net/live/livestream.m3u8 -H 'Origin: http://test.com'
```

需要注意的是，缓存会同时存储 CORS 头信息。这意味着如果你查询并获取了没有 CORS 的 HLS，即使后续请求包含需要 CORS 的 Origin 头，它仍然会保持没有 CORS 的状态。

## Product the Lightsail Installer

Build the image with docker:

```bash
docker rm -f script 2>/dev/null &&
docker rmi srs-script-dev 2>/dev/null || echo OK &&
docker build -t srs-script-dev -f scripts/setup-ubuntu/Dockerfile.script .
```

Create a docker container in daemon:

```bash
docker rm -f script 2>/dev/null &&
docker run -p 2022:2022 -p 2443:2443 -p 1935:1935 -p 8000:8000/udp -p 10080:10080/udp \
    --env CANDIDATE=$(ifconfig en0 |grep 'inet ' |awk '{print $2}') \
    --privileged -v /sys/fs/cgroup:/sys/fs/cgroup:rw --cgroupns=host \
    -d --rm -it -v $(pwd):/g -w /g --name=script srs-script-dev
```

> Note: For Linux server, please use `--privileged -v /sys/fs/cgroup:/sys/fs/cgroup:ro` to start docker.

Install Oryx with script:

```bash
docker exec -it -w /tmp script bash /g/scripts/lightsail.sh 
```

## Use HELM to Install Oryx

Install [HELM](https://helm.sh/docs/intro/install/) and [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/),
then add repo of Oryx:

```bash
helm repo add srs http://helm.ossrs.io/stable
```

Install the latest Oryx:

```bash
helm install srs srs/oryx
```

Or, install from file:

```bash
helm install srs ~/git/srs-helm/stable/oryx-1.0.6.tgz
```

Or, setup the persistence directory:

```bash
helm install srs ~/git/srs-helm/stable/oryx-1.0.6.tgz \
  --set persistence.path=$HOME/data
```

Finally, open [http://localhost](http://localhost) to check it.

## 设置 OpenAI 功能测试用例

如果需要测试 OpenAI 功能（如语音转录），请设置环境变量 `OPENAI_API_KEY` 和 `OPENAI_PROXY`。

> 注意：已在 secret.sh 脚本中设置，无需额外步骤。

## 在 Goland 中运行测试

准备 .env 文件：

```bash
bash scripts/tools/secret.sh --output test/.env &&
cp ~/git/srs/trunk/doc/source.200kbps.768x320.flv test/
```

在 Goland 中运行测试用例。

## 更新 SRS 演示环境

更新 Oryx 的演示环境，针对 bt.ossrs.net：

```bash
IMAGE=$(ssh root@ossrs.net docker images |grep oryx |grep v5 |awk '{print $1":"$2}' |head -n 1) &&
docker build -t $IMAGE -f Dockerfile . &&
docker save $IMAGE |gzip > t.tar.gz &&
scp t.tar.gz root@ossrs.net:~/ &&
ssh root@ossrs.net docker load -i t.tar.gz && 
ssh root@ossrs.net docker stop oryx && 
sleep 3 && ssh root@ossrs.net docker rm -f oryx && 
ssh root@ossrs.net docker image prune -f
```

针对 bt.ossrs.io：

```bash
IMAGE=$(ssh root@ossrs.io docker images |grep oryx |grep v5 |awk '{print $1":"$2}' |head -n 1) &&
docker build -t $IMAGE -f Dockerfile . &&
docker save $IMAGE |gzip > t.tar.gz &&
scp t.tar.gz root@ossrs.io:~/ &&
ssh root@ossrs.io docker load -i t.tar.gz && 
ssh root@ossrs.io docker stop oryx && 
sleep 3 && ssh root@ossrs.io docker rm -f oryx && 
ssh root@ossrs.io docker image prune -f
```

## 配置 SRS 容器

SRS 容器通过环境变量配置，加载 `/data/config/.srs.env` 文件。构建测试镜像：

```bash
docker rmi oryx-env 2>/dev/null || echo OK &&
docker build -t oryx-env -f Dockerfile .
```

设置日志写入文件：

```bash
echo 'SRS_LOG_TANK=file' > $HOME/data/config/.srs.env
```

通过 docker 运行 Oryx：

```bash
docker run --rm -it -p 2022:2022 -p 2443:2443 -p 1935:1935 \
  -p 8000:8000/udp -p 10080:10080/udp --name oryx \
  --env CANDIDATE=$(ifconfig en0 |grep 'inet ' |awk '{print $2}') \
  -v $HOME/data:/data oryx-env
```

注意日志应写入文件，控制台不会显示日志 `write log to console`，而是会显示 `you can check log by`。

## Go PPROF

要分析 Oryx 的性能，可以启用 Go pprof 工具：

```bash
GO_PPROF=localhost:6060 go run .
```

运行 CPU 性能分析：

```bash
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30
```

然后使用 `top` 查看热点函数。

## 设置 youtube-dl

安装 pyinstaller：

```bash
brew install pyinstaller
```

克隆并构建 [youtube-dl](https://github.com/ytdl-org/youtube-dl)：

```bash
cd ~/git && git clone https://github.com/ytdl-org/youtube-dl.git &&
cd ~/git/youtube-dl && pyinstaller --onefile --clean --noconfirm --name youtube-dl youtube_dl/__main__.py &&
ln -sf ~/git/youtube-dl/dist/youtube-dl /opt/homebrew/bin/
```

对于 Linux 服务器，下载最新的 youtube-dl：

```bash
curl -L -o /usr/local/bin/youtube-dl 'https://github.com/ytdl-org/ytdl-nightly/releases/latest/download/youtube-dl' &&
rm -f /usr/bin/python && ln -sf /usr/bin/python3 /usr/bin/python &&
chmod +x /usr/local/bin/youtube-dl
```

替代项目 [yt-dlp](https://github.com/yt-dlp/yt-dlp) 是 youtube-dl 的一个分支：

```bash
cd ~/git && git clone https://github.com/yt-dlp/yt-dlp.git &&
cd ~/git/yt-dlp && python3 -m venv venv && source venv/bin/activate &&
python3 devscripts/install_deps.py --include pyinstaller &&
python3 devscripts/make_lazy_extractors.py
python3 -m bundle.pyinstaller --onefile --clean --noconfirm --name youtube-dl yt_dlp/__main__.py &&
ln -sf ~/git/yt-dlp/dist/youtube-dl /opt/homebrew/bin/
```

在 macOS 上使用 socks5 代理下载：

```bash
youtube-dl --proxy socks5://127.0.0.1:10000 --output srs 'https://youtu.be/SqrazCPWcV0?si=axNvjynVb7Tf4Bfe'
```

> 注意：如果需要使用代理，请设置 `--proxy socks5://127.0.0.1:10000` 或 `YTDL_PROXY=socks5://127.0.0.1:10000`，使用 `ssh -D 127.0.0.1:10000 root@x.y.z.m dstat 30` 启动代理服务器。

> 注意：如果需要定义文件名，请设置 `--output TEMPLATE`。

## Regenrate ASR for Dubbing

在项目文件下创建一个 `regenerate.txt` 文件，然后重启 Oryx 并刷新页面：

```bash
touch ./platform/containers/data/dubbing/4830675a-7945-48fe-bed9-72e6fa904a19/regenerate.txt
```

Oryx 会重新生成 ASR 和翻译，然后删除 `regenerate.txt` 以确保只执行一次。

### 为什么要重新生成 ASR？

- 改进识别结果：如果之前的 ASR 识别结果不准确（例如语音识别错误或漏识别），可以通过重新生成 ASR 来改进。
- 更新内容：如果音频内容发生了变化（例如重新录制或编辑），需要重新生成 ASR 以匹配最新的音频。
- 支持多语言：如果需要为不同语言生成配音，重新生成 ASR 可以提供更准确的文本基础。

## WebRTC 候选地址（Candidate）

Oryx 遵循 WebRTC 候选地址的规则，参见 [CANDIDATE](https://ossrs.io/lts/en-us/docs/v5/doc/webrtc#config-candidate)，但在代理 API 后还有一些额外的改进。

1. 在 SRS 配置中禁用 `use_auto_detect_network_ip` 和 `api_as_candidates`。
1. Always use `?eip=xxx` and ignore any other config, if user force to use the specified IP.
1. If `NAME_LOOKUP` (default is `on`) isn't `off`, try to resolve the candidate from `Host` of HTTP API by Oryx.
  1. If access Oryx by `localhost` for debugging or run in localhost.
    1. If `PLATFORM_DOCKER` is `off`, such as directly run in host, not in docker, use the private ip of Oryx.
    1. If not set `CANDIDATE`, use `127.0.0.1` for OBS WHIP or native client to access Oryx by localhost.
  1. Use `Host` if it's a valid IP address, for example, to access Oryx by public ip address.
  1. Use DNS lookup if `Host` is a domain, for example, to access Oryx by domain name.
1. If no candidate, use docker IP address discovered by SRS. 

> 注意：客户端也可以通过设置 `X-Real-Host` 头来指定候选地址。

> 注意：切勿使用 `host.docker.internal`，因为它仅在 docker 中可用，而不在主机服务器中可用。

## Docker 分配的端口

分配的端口如下：

| 模块 | TCP 端口                                         | UDP 端口 | 备注                                                                                                              |
| ------ |---------------------------------------------------| --------- |-----------------------------------------------------------------------------------------------------------------|
| SRS | 1935, 1985, 8080,<br/> 8088, 1990, 554,<br/> 8936 | 8000, 8935, 10080,<br/> 1989 | See [SRS ports](https://github.com/ossrs/srs/blob/develop/trunk/doc/Resources.md#ports)                         |
| platform | 2022                                              |  - | 挂载在 `/mgmt/`, `/terraform/v1/mgmt/`, `/terraform/v1/hooks/`, `/terraform/v1/ffmpeg/` 和 `/terraform/v1/tencent/` |
| debugging | 22022 | - | 挂载在 `/terraform/v1/debug/goroutines`                                                                       |

> Note: FFmpeg(2019), TencentCloud(2020), Hooks(2021), Mgmt(2022), Platform(2024) has been migrated to platform(2024).

## HTTP OpenAPI

无需身份验证的 API：

* `/terraform/v1/mgmt/versions` 公开版本 API。
* `/terraform/v1/mgmt/check`  检查系统是否正常。
* `/terraform/v1/mgmt/envs` 查询 mgmt 的环境变量。
* `/terraform/v1/releases` 所有组件的版本管理。
* `/terraform/v1/host/versions` 公开版本 API。
* `/terraform/v1/hooks/record/hls/:uuid.m3u8` Hooks：生成 HLS/m3u8 URL 以预览或下载。
* `/terraform/v1/hooks/record/hls/:uuid/index.m3u8` Hooks：提供 HLS m3u8 文件。
* `/terraform/v1/hooks/record/hls/:dir/:m3u8/:uuid.ts` Hooks：提供 HLS ts 文件。
* `/terraform/v1/ai/transcript/hls/overlay/:uuid.m3u8` 生成带有覆盖文本的转录流的预览 HLS。
* `/terraform/v1/ai/transcript/hls/webvtt/:uuid/index.m3u8` 生成带有 WebVTT 文本的转录流的预览 HLS。
  * `/terraform/v1/ai/transcript/hls/webvtt/:uuid/subtitles.m3u8`  HLS 字幕的 HLS。
  * `/terraform/v1/ai/transcript/hls/webvtt/:uuid.m3u8`  WebVTT 的 HLS 流。
* `/terraform/v1/ai/transcript/hls/original/:uuid.m3u8` 生成不带覆盖文本的原始流的预览 HLS。
* `/terraform/v1/ai/ocr/image/:uuid.jpg` 获取 OCR 任务的图像。
* `/terraform/v1/mgmt/beian/query` 查询备案信息。
* `/terraform/v1/ai-talk/stage/hello-voices/:file.aac` AI-Talk：播放示例音频。
* `/.well-known/acme-challenge/` HTTPS 验证挂载（用于 letsencrypt）。
* 对于 SRS 代理:
  * `/rtc/` SRS 的代理：SRS 媒体服务器的 WebRTC HTTP API。
  * `/*/*.(flv|m3u8|ts|aac|mp3)` SRS 的代理：HTTP-FLV、HLS、HTTP-TS、HTTP-AAC、HTTP-MP3 的媒体流。
* 对于静态文件:
  * `/tools/` 一组 H5 工具，如简单播放器、xgplayer 等，由 mgmt 提供。
  * `/console/` SRS 控制台，由 mgmt 提供。
  * `/players/` SRS 播放器，由 mgmt 提供。
  * `/mgmt/` mgmt 的 UI，由 mgmt 提供。

无需令牌验证但需要密码验证的 API：

* `/terraform/v1/mgmt/init` 检查 mgmt 是否已初始化。通过密码登录。
* `/terraform/v1/mgmt/login` 使用密码进行系统身份验证。

平台(Platform)，需要令牌验证的 API：

* `/terraform/v1/mgmt/token` 使用令牌进行系统身份验证。
* `/terraform/v1/mgmt/status` 查询 mgmt 的版本。
* `/terraform/v1/mgmt/bilibili` 查询视频信息。
* `/terraform/v1/mgmt/beian/update` 更新备案信息。
* `/terraform/v1/mgmt/limits/query` 查询限制信息。
* `/terraform/v1/mgmt/limits/update` 更新限制信息。
* `/terraform/v1/mgmt/openai/query` 查询 OpenAI 设置。
* `/terraform/v1/mgmt/openai/update` 更新 OpenAI 设置。
* `/terraform/v1/mgmt/secret/query` 查询 OpenAPI 的 API 密钥。
* `/terraform/v1/mgmt/hphls/update` 高质量模式下 HLS 传递。
* `/terraform/v1/mgmt/hphls/query` 查询搞质量模式下 HLS 传递。
* `/terraform/v1/mgmt/hlsll/update` 设置 HLS 低延迟模式。
* `/terraform/v1/mgmt/hlsll/query` 查询 HLS 低延迟模式的状态。
* `/terraform/v1/mgmt/ssl` 配置系统的 SSL 配置。
* `/terraform/v1/mgmt/auto-self-signed-certificate` 如果没有证书，创建自签名证书。
* `/terraform/v1/mgmt/letsencrypt` 配置 Let's Encrypt SSL。
* `/terraform/v1/mgmt/cert/query` 查询 HTTPS 的密钥和证书。
* `/terraform/v1/mgmt/hooks/apply` 更新 HTTP 回调。
* `/terraform/v1/mgmt/hooks/query` 查询 HTTP 回调。
* `/terraform/v1/mgmt/hooks/example` HTTP 回调的示例目标。
* `/terraform/v1/mgmt/streams/query` 查询活跃的流。
* `/terraform/v1/mgmt/streams/kickoff` 按名称踢出流。
* `/terraform/v1/hooks/srs/verify` Hooks：验证 SRS 的流请求 URL。
* `/terraform/v1/hooks/srs/secret/query` Hooks：查询生成流 URL 的密钥。
* `/terraform/v1/hooks/srs/secret/update` Hooks：更新生成流 URL 的密钥。
* `/terraform/v1/hooks/srs/secret/disable` Hooks：禁用用于身份验证的密钥。
* `/terraform/v1/hooks/srs/hls` Hooks：处理 `on_hls` 事件。
* `/terraform/v1/hooks/record/query` Hooks：查询录制模式。
* `/terraform/v1/hooks/record/apply` Hooks：应用录制模式。
* `/terraform/v1/hooks/record/globs` 更新录制的全局过滤器。
* `/terraform/v1/hooks/record/post-processing` 更新录制的后处理。
* `/terraform/v1/hooks/record/remove` Hooks: 删除录制文件。
* `/terraform/v1/hooks/record/end` 录制：当流未发布时，快速完成录制任务。
* `/terraform/v1/hooks/record/files` Hooks：列出录制文件。
* `/terraform/v1/live/room/create` 直播：创建一个新的直播间。
* `/terraform/v1/live/room/query` 直播：查询一个直播间。
* `/terraform/v1/live/room/update` 直播：更新一个直播间。
* `/terraform/v1/live/room/remove`: 直播：删除一个直播间。
* `/terraform/v1/live/room/list` 直播：列出所有可用的直播间。
* `/terraform/v1/ai-talk/stage/start` AI-Talk: 开始一个新的舞台。
* `/terraform/v1/ai-talk/stage/conversation` AI-Talk: 开始舞台的新对话请求。
* `/terraform/v1/ai-talk/stage/upload` AI-Talk: 上传用户输入的音频文件。
* `/terraform/v1/ai-talk/stage/query` AI-Talk: 查询输入的响应。
* `/terraform/v1/ai-talk/stage/verify` AI-Talk: 验证舞台级别的弹出令牌。
* `/terraform/v1/ai-talk/subscribe/start` AI-Talk: 开始一个带舞台的弹出窗口。
* `/terraform/v1/ai-talk/subscribe/query` AI-Talk: 查询弹出音频响应。
* `/terraform/v1/ai-talk/subscribe/tts` AI-Talk: 播放 TTS 音频。
* `/terraform/v1/ai-talk/subscribe/remove` AI-Talk: 删除弹出音频响应。
* `/terraform/v1/ai-talk/user/query` AI-Talk: 查询用户信息。
* `/terraform/v1/ai-talk/user/update` AI-Talk: 更新用户信息。
* `/terraform/v1/dubbing/create` Dubbing: 创建一个配音项目。
* `/terraform/v1/dubbing/list` Dubbing: 列出所有配音项目。
* `/terraform/v1/dubbing/remove` Dubbing: 删除一个配音项目。
* `/terraform/v1/dubbing/query` Dubbing: 查询一个配音项目。
* `/terraform/v1/dubbing/update` Dubbing: 更新一个配音项目。
* `/terraform/v1/dubbing/play` Dubbing: 播放配音音频或视频。
* `/terraform/v1/dubbing/export` Dubbing: 生成并下载配音音频文件。
* `/terraform/v1/dubbing/task-start` Dubbing: 启动配音的工作任务。
* `/terraform/v1/dubbing/task-query` Dubbing: 查询配音的工作任务。
* `/terraform/v1/dubbing/task-tts` Dubbing: 播放配音的 TTS 音频。
* `/terraform/v1/dubbing/task-rephrase` Dubbing: 重新生成配音组的 TTS。
* `/terraform/v1/dubbing/task-merge`: Dubbing: 将配音组合并到前一个或下一个组。
* `/terraform/v1/ffmpeg/forward/secret` FFmpeg: 设置直播平台的转发密钥。
* `/terraform/v1/ffmpeg/forward/streams` FFmpeg: 查询转发流。
* `/terraform/v1/ffmpeg/vlive/secret` 设置虚拟直播流的密钥。
* `/terraform/v1/ffmpeg/vlive/streams` 查询虚拟直播流。
* `/terraform/v1/ffmpeg/vlive/source` 设置虚拟直播源文件。
* `/terraform/v1/ffmpeg/vlive/upload/` Source: 上传虚拟直播或配音源文件。
* `/terraform/v1/ffmpeg/vlive/server` Source: 使用服务器文件作为虚拟直播或配音源。
* `/terraform/v1/ffmpeg/vlive/ytdl` Source: 使用 [youtube-dl](https://github.com/ytdl-org/youtube-dl) 下载 URL 作为虚拟直播或配音源。
* `/terraform/v1/ffmpeg/vlive/stream-url` Source: 使用流 URL 作为虚拟直播源。
* `/terraform/v1/ffmpeg/camera/secret` 设置 IP 摄像头流的密钥。
* `/terraform/v1/ffmpeg/camera/streams` 查询 IP 摄像头流。
* `/terraform/v1/ffmpeg/camera/source` 设置 IP 摄像头源文件。
* `/terraform/v1/ffmpeg/camera/stream-url` Source: 使用流 URL 作为 IP 摄像头源。
* `/terraform/v1/ffmpeg/transcode/query`  查询转码配置。
* `/terraform/v1/ffmpeg/transcode/apply`  应用转码配置。
* `/terraform/v1/ffmpeg/transcode/task` 查询转码任务。
* `/terraform/v1/ai/transcript/apply` 更新转录设置。
* `/terraform/v1/ai/transcript/query`  查询转录设置。
* `/terraform/v1/ai/transcript/check` 检查转录的 OpenAI 服务。
* `/terraform/v1/ai/transcript/clear-subtitle`:  清除修复队列中的段字幕。
* `/terraform/v1/ai/transcript/live-queue` 查询转录的实时队列。
* `/terraform/v1/ai/transcript/asr-queue` 查询转录的 ASR 队列。
* `/terraform/v1/ai/transcript/fix-queue`  查询转录的修复队列。
* `/terraform/v1/ai/transcript/overlay-queue` 查询转录的覆盖队列。
* `/terraform/v1/ai/ocr/apply`  更新 OCR 设置。
* `/terraform/v1/ai/ocr/query` 查询 OCR 设置。
* `/terraform/v1/ai/ocr/check` 检查 OCR 的 OpenAI 服务。
* `/terraform/v1/ai/ocr/live-queue`  查询 OCR 的实时队列。
* `/terraform/v1/ai/ocr/ocr-queue` 查询 OCR 的识别队列。
* `/terraform/v1/ai/ocr/callback-queue` 查询 OCR 的回调队列。
* `/terraform/v1/ai/ocr/cleanup-queue` 查询 OCR 的清理队列。

平台为 SRS 代理提供的 API：

* `/api/` SRS: SRS 媒体服务器的 HTTP API。需要令牌验证。

**Deprecated(弃用)** API:

* `/terraform/v1/tencent/cam/secret` 腾讯：设置 CAM SecretId 和 SecretKey。
* `/terraform/v1/hooks/dvr/apply` Hooks: 应用 DVR 模式。
* `/terraform/v1/hooks/dvr/query` Hooks: 查询 DVR 模式。
* `/terraform/v1/hooks/dvr/files` Hooks: 列出 DVR 文件。
* `/terraform/v1/hooks/dvr/hls/:uuid.m3u8` Hooks: 生成 HLS/m3u8 URL 以预览或下载。
* `/terraform/v1/hooks/vod/query` Hooks: 查询 VoD 模式。
* `/terraform/v1/hooks/vod/apply` Hooks: 应用 VoD 模式。
* `/terraform/v1/hooks/vod/files` Hooks: 列出 VoD 文件。
* `/terraform/v1/hooks/vod/hls/:uuid.m3u8` Hooks: 生成 HLS/m3u8 URL 以预览或下载。

**Removed(移除)** API:

* `/terraform/v1/mgmt/strategy` 切换升级策略。
* `/prometheus` Prometheus: 时间序列数据库和监控。
* `/terraform/v1/mgmt/nginx/proxy` 设置反向代理位置。
* `/terraform/v1/mgmt/dns/lb` HTTP-DNS 用于 HLS 负载均衡。
* `/terraform/v1/mgmt/dns/backend/update` HTTP-DNS：更新 HLS 负载均衡的后端服务器。
* `/terraform/v1/mgmt/nginx/homepage` 设置首页重定向。
* `/terraform/v1/mgmt/window/query` 查询升级时间窗口。
* `/terraform/v1/mgmt/window/update` 更新升级时间窗口。
* `/terraform/v1/mgmt/pubkey` 更新平台管理员公钥的访问权限。
* `/terraform/v1/mgmt/upgrade` 将 mgmt 升级到最新版本。
* `/terraform/v1/mgmt/containers` 查询 SRS 容器。
* `/terraform/v1/host/exec` 同步执行命令，返回 stdout 和 stderr。
* `/terraform/v1/mgmt/secret/token` 为 OpenAPI 创建令牌。

## 依赖的软件

我们依赖的软件包括：

* Docker, `apt-get install -y docker.io`
* Nginx, `apt-get install -y nginx`
    * Conf: `platform/containers/conf/nginx.conf`
    * Include: `platform/containers/data/config/nginx.http.conf`
    * Include: `platform/containers/data/config/nginx.server.conf`
    * SSL Key: `platform/containers/data/config/nginx.key`
    * Certificate: `platform/containers/data/config/nginx.crt`
* [LEGO](https://github.com/go-acme/lego)
    * Verify webroot: `platform/containers/data/.well-known/acme-challenge/`
    * Cert files: `platform/containers/data/lego/.lego/certificates/`
* [SRS](https://github.com/ossrs/srs)
    * Config: `platform/containers/conf/srs.release.conf` mount as `/usr/local/srs/conf/srs.conf`
    * Include: `platform/containers/data/config/srs.server.conf`
    * Include: `platform/containers/data/config/srs.vhost.conf`
    * Volume: `platform/containers/objs/nginx/` mount as `/usr/local/srs/objs/nginx/`
* FFmpeg:
    * [FFmpeg and ffprobe](https://ffmpeg.org) tools in `ossrs/srs:ubuntu20`

## 环境变量

由 `platform/containers/data/config/.env` 定义的可选环境变量：

* `MGMT_PASSWORD`: mgmt 管理员密码。
* `REACT_APP_LOCALE`: UI 的国际化配置，`en` 或 `zh`，默认为 `en`。

由 `platform/containers/data/config/.env` 定义的其他环境变量：

* `CLOUD`: `dev|bt|aapanel|droplet|docker`, 云平台名称，`DEV` 表示开发环境。
* `REGION`: `ap-guangzhou|ap-singapore|sgp1`, 升级源的区域。
* `REGISTRY`: `docker.io|registry.cn-hangzhou.aliyuncs.com`, Docker 镜像仓库。
* `MGMT_LISTEN`: mgmt HTTP 服务器的监听端口。默认：`2022`
* `PLATFORM_LISTEN`: 平台 HTTP 服务器的监听端口。默认：`2024`
* `HTTPS_LISTEN`: HTTPS 服务器的监听端口。默认：`2443`

对于在同一主机服务器上运行多个容器的多端口配置：

* `HTTP_PORT`: HTTP 服务器的监听端口。默认用于访问仪表板的端口。
* `RTMP_PORT`: RTMP 服务器的监听端口。默认：`1935`
* `SRT_PORT`: SRT 服务器的 UDP 监听端口。默认：`10080`
* `RTC_PORT`: RTC 服务器的 UDP 监听端口。默认：`8000`

用于限制的可配置项：

* `SRS_FORWARD_LIMIT`: SRS 转发的限制。默认：`10`。
* `SRS_VLIVE_LIMIT`: SRS 虚拟直播的限制。默认：`10`。

用于功能控制的配置：

* `NAME_LOOKUP`: `on|off`, 是否启用主机名解析。默认：`on`

用于测试指定服务的配置：

* `NODE_ENV`: `development|production`，如果为 development，则使用本地 redis；否则，使用 docker 中的 `mgmt.srs.local`。默认：`development`
* `LOCAL_RELEASE`: `on|off`，是否使用本地发布服务。默认：`off`
* `PLATFORM_DOCKER`: `on|off`，是否在 docker 中运行平台。默认：`off`

用于 mgmt 和容器连接 redis 的配置：

* `REDIS_PASSWORD`: redis 密码。默认：空。
* `REDIS_PORT`: redis 端口。默认：`6379`。

用于 React UI 的环境变量：

* `PUBLIC_URL`: 挂载前缀。
* `BUILD_PATH`: 输出构建路径，默认：`build`。

> 注意：React 的环境变量必须以 `REACT_APP_` 开头，请阅读 [这篇文章](https://create-react-app.dev/docs/adding-custom-environment-variables/#referencing-environment-variables-in-the-html)。

已从 .env 中移除的变量：

* `SRS_PLATFORM_SECRET`: 用于生成和验证令牌的 mgmt API 密钥。

用于 HTTPS，自动生成自签名证书的配置：

* `AUTO_SELF_SIGNED_CERTIFICATE`: `on|off`，是否生成自签名证书。默认：`on`。

已弃用且未使用的变量：

* `SRS_DOCKERIZED`: `on|off`, 表示操作系统是否在 docker 中。
* `SRS_DOCKER`: `srs`  强制使用 `ossrs/srs` docker 镜像。
* `MGMT_DOCKER`: `on|off`, 是否在 docker 中运行 mgmt。默认：false
* `USE_DOCKER`: `on|off`, 如果为 false，则禁用所有 docker 容器。
* `SRS_UTEST`: `on|off`, 如果为 on，则在单元测试模式下运行。
* `SOURCE`: `github|gitee`, 升级的源代码来源。

其他变量：

* `YTDL_PROXY`: 为 youtube-dl 设置代理，例如：`socks5://127.0.0.1:10000`
* `GO_PPROF`: 为 Go PPROF 工具设置监听地址，例如：`localhost:6060`

当 `.env` 文件更改时，请重启服务。

## 编码指南

对于包含两个以上单词的 JSON 字段：

```go
type CameraConfigure struct {
	ExtraAudio string `json:"extraAudio"`
}
```

或者使用以下格式：

```go
type CameraConfigure struct {
    ExtraAudio string `json:"extra_audio"`
}
```

通常情况下，我们遵循此指南，但一些遗留代码可能例外。

## Changelog

The following are the update records for the Oryx server.

* v5.15:
    * Forward: Support multiple forwarding servers. v5.15.1
    * ENV: Refine the environment variables. v5.15.2
    * OCR: Support OCR for image recognition. v5.15.3
    * VLive: Support multiple virtual live streaming. v5.15.4
    * Camera: Support multiple camera streaming. [v5.15.5](https://github.com/ossrs/oryx/releases/tag/v5.15.5)
    * Transcript: Upgrade the hls.js to 1.4 for WebVTT. v5.15.6
    * Disable version query and check. v5.15.7
    * Change LICENSE from AGPL-3.0-or-later to MIT. v5.15.8
    * Dubbing: Support scrolling card in fullscreen. v5.15.9
    * Support external redis host and using 127.0.0.1 as default. v5.15.10
    * Support setup global OpenAI settings. [v5.15.11](https://github.com/ossrs/oryx/releases/tag/v5.15.11)
    * Add youtube-dl binary for dubbing etc. v5.15.12
    * VLive: Fix bug when source codec is not supported. v5.15.13
    * Forward: Fix high CPU bug. v5.15.14
    * Support Go PPROF for CPU profiling. [v5.15.15](https://github.com/ossrs/oryx/releases/tag/v5.15.15)
    * VLive: Support download by youtube-dl. v5.15.16
    * Dubbing: Use gpt-4o and smaller ASR segment. v5.15.17
    * Dubbing: Merge more words if in small duration. v5.15.17
    * Dubbing: Allow fullscreen when ASR. v5.15.18
    * Dubbing: Support disable asr or translation. v5.15.19
    * Dubbing: Fix bug when changing ASR segment size. v5.15.20
    * Dubbing: Refine the window of text. [v5.15.20](https://github.com/ossrs/oryx/releases/tag/v5.15.20)
    * Dubbing: Support space key to play/pause. v5.15.21
    * AI: Support OpenAI o1-preview model. v5.15.22
* v5.14:
    * Merge features and bugfix from releases. v5.14.1
    * Dubbing: Support VoD dubbing for multiple languages. [v5.14.2](https://github.com/ossrs/oryx/releases/tag/v5.14.2)
    * Dubbing: Support disable translation, rephrase, or tts. v5.14.3
    * Dubbing: Highlight the currently playing group. v5.14.3
    * NGINX: Support set the m3u8 and ts expire. v5.14.3
    * HLS: Set m3u8 expire time to 1s for LLHLS. v5.14.4
    * HLS: Use fast cache for HLS config. v5.14.4
    * Transcript: Support set the force_style for overlay subtitle. v5.14.5
    * Transcript: Use Whisper response without LF. (#163). v5.14.5
    * Token: Fix bug for Bearer token while initializing. [v5.14.6](https://github.com/ossrs/oryx/releases/tag/v5.14.6)
    * Room: Enable dictation mode for AI-Talk. v5.14.7
    * Dubbing: Refine download button with comments. v5.14.8
    * Room: AI-Talk support post processing. v5.14.9
    * Website: Support setting title for popout. v5.14.10
    * Transcript: Support set the video codec parameters. [v5.14.11](https://github.com/ossrs/oryx/releases/tag/v5.14.11)
    * Transcript: Support subtitle with WebVTT format. v5.14.12
    * Transcript: Fix overlay transcoding parameters parsing bug. v5.14.13
    * Use port 80/443 by default in README. v5.14.14 
    * Use fastfail for test and utest. v5.14.15
    * Rename project to Oryx. [v5.14.15](https://github.com/ossrs/oryx/releases/tag/v5.14.15)
    * API: Support kickoff stream by name. v5.14.16
    * AI-Talk: Refine the delay of ASR to 3s. [v5.14.17](https://github.com/ossrs/oryx/releases/tag/v5.14.17)
    * AI-Talk: Ignore silent ASR text. v5.14.18
    * Refine installer for lightsail. [v5.14.19](https://github.com/ossrs/oryx/releases/tag/v5.14.19)
    * Update model to gpt-3.5-turbo, gpt-4-turbo, gpt-4o. v5.14.20
    * Transcript: Upgrade the hls.js to 1.4 for WebVTT. v5.14.21
    * Disable version query and check. [v5.14.22](https://github.com/ossrs/oryx/releases/tag/v5.14.22)
    * Support Go PPROF for CPU profiling. v5.14.23
    * Forward: Fix high CPU bug. v5.14.24
    * VLive: Refine wait timeout. [v5.14.25](https://github.com/ossrs/oryx/releases/tag/v5.14.25)
* v5.13:
    * Fix bug for vlive and transcript. v5.13.1
    * Support AWS Lightsail install script. v5.13.2
    * Limits: Support limit bitrate for VLive stream. v5.13.3
    * Fix bug: Remove HTTP port for SRS. v5.13.4
    * Refine API with Bearer token. v5.13.5
    * Switch to fluid max width. v5.13.6
    * HLS: Support low latency mode about 5s. v5.13.7
    * RTSP: Rebuild the URL with escaped user info. v5.13.8
    * VLive: Support SRT URL filter. [v5.13.9](https://github.com/ossrs/oryx/releases/tag/v5.13.9)
    * FFmpeg: Monitor and restart FFmpeg if stuck. v5.13.10
    * Room: Support live room secret with stream URL. v5.13.11
    * Camera: Support IP Camera streaming scenario. v5.13.12
    * Camera: Support silent extra audio stream for video only device. v5.13.13
    * Camera: Support replace audio if enabled extra audio. v5.13.13
    * VLive/Camera/Forward: Extract multilingual text. v5.13.14
    * Room: AI-Talk support assistant for live room. v5.13.15
    * Room: AI-Talk support popout live chat. v5.13.16
    * Room: AI-Talk support popout AI assistant. v5.13.17
    * Room: AI-Talk support multiple assistant in a room. v5.13.18
    * Room: AI-Talk support user different languages. v5.13.18
    * Room: AI-Talk allow disable ASR/TTS, enable text. v5.13.19
    * Room: AI-Talk support dictation mode. v5.13.20
    * FFmpeg: Restart if time and speed abnormal. v5.13.21
    * Transcript: Fix panic bug for sync goroutines. [v5.13.21](https://github.com/ossrs/oryx/releases/tag/v5.13.21)
    * Support OpenAI organization for billing. v5.13.22
    * Room: Fix the empty room UI sort and secret bug. [v5.13.23](https://github.com/ossrs/oryx/releases/tag/v5.13.23)
    * FFmpeg: Fix restart bug for abnormal speed. v5.13.24
    * FFmpeg: Fix bug for output SRT protocol. v5.13.25
    * FFmpeg: Support ingest SRT protocol. v5.13.26
    * VLive: Fix the re bug for file. [v5.13.27](https://github.com/ossrs/oryx/releases/tag/v5.13.27)
    * Release stable version and support debugging. [v5.13.28](https://github.com/ossrs/oryx/releases/tag/v5.13.28)
    * HLS: Set m3u8 expire time to 1s for LLHLS. v5.13.29
    * Transcript: Support set the force_style for overlay subtitle. v5.13.30
    * Transcript: Use Whisper response without LF. (#163). v5.13.31
    * Token: Fix bug for Bearer token while initializing. [v5.13.32](https://github.com/ossrs/oryx/releases/tag/v5.13.32)
    * Room: Refine stat for AI-Talk. v5.13.33
    * Rename Oryx to Oryx. [v5.13.34](https://github.com/ossrs/oryx/releases/tag/v5.13.34)
* v5.12
    * Refine local variable name conf to config. v5.12.1
    * Add forced exit on timeout for program termination. v5.12.1
    * Transcript: Support convert live speech to text by whisper. [v5.12.2](https://github.com/ossrs/oryx/releases/tag/v5.12.2)
    * Transcript: Update base image for FFmpeg subtitles. v5.12.3
    * Transcript: Limit all queue base on overlay. v5.12.4
    * Transcript: Allow work without active stream. [v5.12.5](https://github.com/ossrs/oryx/releases/tag/v5.12.5)
    * Transcript: Support testing connection to OpenAI service. v5.12.6
    * Filter locale value. v5.12.6
    * Transcript: Add test case for OpenAI. [v5.12.7](https://github.com/ossrs/oryx/releases/tag/v5.12.7)
    * Transcript: Use m4a and 30kbps bitrate to make ASR faster. v5.12.8
    * Hooks: Support callback on_record_begin and on_record_end. v5.12.9
    * VLive: Fix ffprobe RTSP bug, always use TCP transport. v5.12.10
    * Confirm when user logout. v5.12.11
    * Record: Support glob filters to match stream. v5.12.11
    * Transcript: Show detail error if exceeded quota. [v5.12.12](https://github.com/ossrs/oryx/releases/tag/v5.12.12)
    * Record: Support finish record task quickly after stream unpublished. v5.12.13
    * Record: Support post-processing to cp file for S3. v5.12.14
    * Transcript: Support clear the subtitle of segment in fixing queue. [v5.12.15](https://github.com/ossrs/oryx/releases/tag/v5.12.15)
    * VLive: Fix bug for url with query string. v5.12.16
    * Transcript: Check the base url for OpenAI. [v5.12.17](https://github.com/ossrs/oryx/releases/tag/v5.12.17)
    * HLS: Support low latency mode about 5s. v5.12.18
    * RTSP: Rebuild the URL with escaped user info. v5.12.19
    * VLive: Fix rebuild URL bug. v5.12.20
    * HLS: Fix LL HLS setting bug. [v5.12.21](https://github.com/ossrs/oryx/releases/tag/v5.12.21)
    * VLive: Support SRT URL filter. v5.12.22
    * HLS: Set m3u8 expire time to 1s for LLHLS. [v5.12.22](https://github.com/ossrs/oryx/releases/tag/v5.12.22)
* v5.11
    * VLive: Decrease the latency for virtual live. v5.11.1
    * Live: Refine multiple language. v5.11.2
    * Hooks: Support HTTP Callback and test. [v5.11.3](https://github.com/ossrs/oryx/releases/tag/v5.11.3)
    * HELM: Support resolve name to ip for rtc. v5.11.4
    * HELM: Disable NAME_LOOKUP by default. [v5.11.5](https://github.com/ossrs/oryx/releases/tag/v5.11.5)
    * Refine env variable for bool. v5.11.7
    * RTC: Refine WHIP player and enable NAME_LOOKUP by default. v5.11.8
    * RTC: Update WHIP and WHEP player. v5.11.9
    * RTC: Resolve candidate for lo and docker. v5.11.10
    * RTC: Refine test and tutorial for WHIP/WHEP. [v5.11.10](https://github.com/ossrs/oryx/releases/tag/v5.11.10)
    * Refine player open speed. v5.11.11
    * HTTPS: Check dashboard and ssl domain. v5.11.12
    * API: Add curl and jQuery example. v5.11.12
    * API: Allow CORS by default. v5.11.13
    * API: Remove duplicated CORS headers. [v5.11.14](https://github.com/ossrs/oryx/releases/tag/v5.11.14)
    * Support expose ports for multiple containers. v5.11.15
    * HTTPS: Check dashboard hostname and port. v5.11.15
    * Error when eslint fail. v5.11.16
    * Use upx to make binary smaller. v5.11.16
    * Refine transcode test case. [v5.11.17](https://github.com/ossrs/oryx/releases/tag/v5.11.17)
    * HTTPS: Enable self-signed certificate by default. v5.11.18
    * HLS: Nginx HLS CDN support HTTPS. v5.11.19
    * Refine scenarios with discouraged and deprecated. v5.11.20
    * Transcode: Refine stream compare algorithm. v5.11.21
    * Hooks: Support callback self-sign HTTPS URL. v5.11.22
    * Fix utest fail. [v5.11.23](https://github.com/ossrs/oryx/releases/tag/v5.11.23)
    * VLive: Fix ffprobe RTSP bug, always use TCP transport. [v5.11.24](https://github.com/ossrs/oryx/releases/tag/v5.11.24)
* v5.10
    * Refine README. v5.10.1
    * Refine DO and droplet release script. v5.10.2
    * VLive: Fix bug of link. v5.10.2
    * Record: Fix bug of change record directory. v5.10.2 (#133)
    * Streaming: Add SRT streaming. [v5.10.2](https://github.com/ossrs/oryx/releases/tag/v5.10.2)
    * Streaming: Add OBS SRT streaming. v5.10.3
    * Fix lighthouse script bug. v5.10.4
    * VLive: Support forward stream. v5.10.5
    * VLive: Cleanup temporary file when uploading. v5.10.6
    * VLive: Use TCP transport when pull RTSP stream. [v5.10.7](https://github.com/ossrs/oryx/releases/tag/v5.10.7)
    * Refine statistic and report data. v5.10.8
    * Support file picker with language. [v5.10.9](https://github.com/ossrs/oryx/releases/tag/v5.10.9)
    * Report language. v5.10.10
    * Transcode: Support live stream transcoding. [v5.10.11](https://github.com/ossrs/oryx/releases/tag/v5.10.11)
    * Transcode: Fix param bug. v5.10.12
    * Fix default stream name bug. v5.10.13
    * Update doc. v5.10.14
    * New stable release. [v5.10.15](https://github.com/ossrs/oryx/releases/tag/v5.10.15)
    * Fix js missing bug. v5.10.16
    * Support docker images for helm. [v5.10.17](https://github.com/ossrs/oryx/releases/tag/v5.10.17)
    * Use WHIP and WHEP for RTC. v5.10.18
    * Transcode: Refine stream compare algorithm. v5.10.19
* v5.9
    * Update NGINX HLS CDN guide. v5.9.2
    * Move DVR and VoD to others. v5.9.3
    * Remove the Tencent CAM setting. v5.9.4
    * Refine Virtual Live start and stop button. v5.9.5
    * Refine Record start and stop button. v5.9.6
    * Refine Forward start and stop button. v5.9.7
    * Move SRT streaming to others. v5.9.8
    * Support vlive to use server file. v5.9.9
    * Add test for virtual live. v5.9.10
    * Add test for record. v5.9.11
    * Add test for forward. v5.9.12
    * Refine test to transmux to mp4. [v5.9.13](https://github.com/ossrs/oryx/releases/tag/v5.9.13)
    * Upgrade jquery and mpegtsjs. v5.9.14
    * Support authentication for SRS HTTP API. [v5.9.15](https://github.com/ossrs/oryx/releases/tag/v5.9.15)
    * Don't expose 1985 API port. v5.9.16
    * Load environment variables from /data/config/.srs.env. v5.9.17
    * Change guide to use $HOME/data as home. v5.9.18
    * Translate forward to English. [v5.9.19](https://github.com/ossrs/oryx/releases/tag/v5.9.19)
    * Refine record, dvr, and vod files. v5.9.20
    * Translate record to English. [v5.9.21](https://github.com/ossrs/oryx/releases/tag/v5.9.21)
    * Refine virtual live files. v5.9.22
    * Translate virtual live to English. v5.9.23
    * Support always open tabs. v5.9.24
    * Remove record and vlive group. [v5.9.25](https://github.com/ossrs/oryx/releases/tag/v5.9.25)
    * Refine project description. v5.9.26
    * Refine DO and droplet release script. [v5.9.27](https://github.com/ossrs/oryx/releases/tag/v5.9.27)
    * Fix bug, release stable version. v5.9.28
    * VLive: Fix bug of link. v5.9.28
    * Record: Fix bug of change record directory. v5.9.28 (#133)
    * Streaming: Add SRT streaming. [v5.9.28](https://github.com/ossrs/oryx/releases/tag/v5.9.28)
    * Fix lighthouse HTTPS bug. v5.9.29
* v5.8
    * Always dispose DO VM and domain for test. v1.0.306
    * Fix docker start failed, cover by test. v1.0.306
    * Switch default language to en. v1.0.306
    * Support include for SRS config. v1.0.306
    * Support High Performance HLS mode. v1.0.307
    * Show current config for settings. v1.0.307
    * Switch MIT to AGPL License. v1.0.307
    * Use one version strategy. [v5.8.20](https://github.com/ossrs/oryx/releases/tag/v5.8.20)
    * Always check test result. v5.8.21
    * SRT: Enable srt in default vhost. v5.8.22
    * Add utest for HP HLS. v5.8.23
    * Migrate docs to new website. v5.8.23
    * BT and aaPanel plugin ID should match filename. v5.8.24
    * Add Nginx HLS Edge tutorial. v5.8.25
    * Download test file from SRS. v5.8.26
    * Do not require version. v5.8.26
    * Fix Failed to execute 'insertBefore' on 'Node'. v5.8.26
    * Eliminate unused callback events. v5.8.26
    * Add docker for nginx HLS CDN. v5.8.27
    * Update Oryx architecture. v5.8.27
    * Use DO droplet s-1vcpu-1gb for auto test. v5.8.28
    * Use default context when restore hphls. [v5.8.28](https://github.com/ossrs/oryx/releases/tag/v5.8.28)
    * Support remote test. v5.8.29
    * Enable CORS and timestamp in HLS. [v5.8.30](https://github.com/ossrs/oryx/releases/tag/v5.8.30)
    * Release stable version. [v5.8.31](https://github.com/ossrs/oryx/releases/tag/v5.8.31)
* v5.7
    * Refine DigitalOcean droplet image. v1.0.302
    * Support local test all script. v1.0.302
    * Rewrite script for lighthouse. v1.0.303
    * Set nginx max body to 100GB. v1.0.303
    * Use LEGO instead of certbot. v1.0.304
    * Rename SRS Cloud to Oryx. v1.0.304
    * Support HTTPS by SSL file. v1.0.305
    * Support reload nginx for SSL. v1.0.305
    * Support request SSL from letsencrypt. v1.0.305
    * Support work with bt/aaPanel ssl. v1.0.305
    * Support self-sign certificate by default. v1.0.305
    * Query configured SSL cert. v1.0.305
    * 2023.08.13: Support test online environment. [v5.7.19](https://github.com/ossrs/oryx/releases/tag/publication-v5.7.19)
    * 2023.08.20: Fix the BT and aaPanel filename issue. [v5.7.20](https://github.com/ossrs/oryx/releases/tag/publication-v5.7.20)
* 2023.08.06, v1.0.301, v5.7.18
    * Simplify startup script, fix bug, adjust directory to `/data` top-level directory. v1.0.296
    * Improve message prompts, script comments, and log output. v1.0.297
    * Avoid modifying the global directory every time it starts, initialize it in the container and platform script. v1.0.298
    * Improve release script, check version matching, manually update version. v1.0.299
    * Remove upgrade function, maintain consistency of docker and other platforms. v1.0.300
    * Improved BT and aaPanel scripts, added test pipeline. v1.0.300
    * Always use the latest SRS 5.0 release. v1.0.301
    * Use status to check SRS, not by the exit value. v1.0.301
* 2023.04.05, v1.0.295, structural improvements
    * Remove HTTPS certificate application, administrator authorization, NGINX reverse proxy, and other functions. v1.0.283
    * Implement Release using Go, reducing memory requirements and image size. v1.0.284
    * Remove dashboard and Prometheus, making it easier to support a single Docker image. v1.0.283
    * Implement mgmt and platform using Go, reducing memory requirements and image size. v1.0.283
    * Use Ubuntu focal(20) as the base image, reducing image size. v1.0.283
    * Support fast upgrade, installation in about 40 seconds, upgrade in about 10 seconds. v1.0.283
    * Solve the problem of forwarding without stream. v1.0.284
    * Solve the problem of uploading large files and getting stuck. v1.0.286
    * Remove AI face-changing video, B station review did not pass. v1.0.289 (stable)
    * Remove Redis container and start Redis directly in the platform. v1.0.290
    * Remove SRS container and start SRS directly in the platform. v1.0.291
    * Support single container startup, including mgmt in one container. v1.0.292
    * Support mounting to `/data` directory for persistence. v1.0.295
* 2023.02.01, v1.0.281, experience improvement, Stable version.
    * Allow users to turn off automatic updates and use manual updates.
    * Adapt to the new version of Bao Ta, solve the nodejs detection problem.
    * Bao Ta checks the plug-in status, and cannot operate before the installation is complete.
    * Improve the display of forwarding status, add `waiting` status. v1.0.260
    * Improve image update, not strongly dependent on certbot. #47
    * Merge hooks/tencent/ffmpeg image into the platform. v1.0.269
    * Support custom platform for forwarding. v1.0.270
    * Support virtual live broadcast, file-to-live broadcast. v1.0.272
    * Upload file limit 100GB. v1.0.274
    * Fix bug in virtual live broadcast. v1.0.276
    * Release service, replace Nodejs with Go, reduce image size. v1.0.280
    * Do not use buildx to build single-architecture docker images, CentOS will fail. v1.0.281
* 2022.11.20, v1.0.256, major version update, experience improvement, Release 4.6
    * Proxy root site resources, such as favicon.ico
    * Support [SrsPlayer](https://wordpress.org/plugins/srs-player) WebRTC push stream shortcode.
    * Support [local recording](https://github.com/ossrs/oryx/issues/42), recording to Oryx local disk.
    * Support deleting local recording files and tasks.
    * Support local recording as MP4 files and downloads.
    * Support local recording directory as a soft link, storing recorded content on other disks.
    * Improve recording navigation bar, merge into recording.
    * Resolve conflicts between home page and proxy root directory.
    * Solve the problem of not updating NGINX configuration during upgrade.
    * Fix the bug of setting record soft link.
    * Replace all images with standard images `ossrs/srs`.
    * Support setting website title and footer (filing requirements).
    * Prompt administrator password path, can retrieve password when forgotten.
    * Allow recovery of the page when an error occurs, no need to refresh the page.
* 2022.06.06, v1.0.240, major version update, Bao Ta, Release 4.5
    * Reduce disk usage, clean up docker images
    * Improve dependencies, no longer strongly dependent on Redis and Nginx
    * Support [Bao Ta](https://mp.weixin.qq.com/s/nutc5eJ73aUa4Hc23DbCwQ) or [aaPanel](https://blog.ossrs.io/how-to-setup-a-video-streaming-service-by-aapanel-9748ae754c8c) plugin, support CentOS or Ubuntu command line installation
    * Migrate ossrs.net to lightweight server, no longer dependent on K8s.
    * Login password default changed to display password.
    * Stop pushing stream for a certain time, clean up HLS cache files.
    * Create a 2GB swap area if memory is less than 2GB.
    * Support collecting SRS coredump.
    * Live scene display SRT push stream address and command.
    * Support setting NGINX root proxy path.
* 2022.04.18, v1.0.222, minor version update, containerized Redis
    * Improve instructions, support disabling push stream authentication.
    * Support English guidance, [medium](https://blog.ossrs.io) articles.
    * Improve simple player, support mute autoplay.
    * Add CORS support when NGINX distributes HLS.
    * Add English guidance, [Create SRS](https://blog.ossrs.io/how-to-setup-a-video-streaming-service-by-1-click-e9fe6f314ac6) and [Set up HTTPS](https://blog.ossrs.io/how-to-secure-srs-with-lets-encrypt-by-1-click-cb618777639f), [WordPress](https://blog.ossrs.io/publish-your-srs-livestream-through-wordpress-ec18dfae7d6f).
    * Enhance key length, strengthen security, and avoid brute force cracking.
    * Support WordPress Shortcode guidance.
    * Support setting home page redirection path, support mixed running with other websites.
    * Support setting reverse proxy, support hanging other services under NGINX.
    * Support applying for multiple domain names for HTTPS, solving the `www` prefix domain name problem.
    * Change `filing` to `website`, can set home page redirection and footer filing number.
    * Improve NGINX configuration file structure, centralize configuration in `containers` directory.
    * Support setting simple load balancing, randomly selecting a backend NGINX for HLS distribution.
    * Containers work in an independent `oryx` network.
    * Add `System > Tools` option.
    * Use Redis container, not dependent on host Redis service.
* 2022.04.06, v1.0.200, major version update, multi-language, Release 4.4
    * Support Chinese and English bilingual.
    * Support DigitalOcean image, see [SRS Droplet](https://marketplace.digitalocean.com/apps/srs).
    * Support OpenAPI to get push stream key, see [#19](https://github.com/ossrs/oryx/pull/19).
    * Improve container image update script.
    * Support using NGINX to distribute HLS, see [#2989](https://github.com/ossrs/srs/issues/2989#nginx-direclty-serve-hls).
    * Improve VoD storage and service detection.
    * Improve installation script.
* 2022.03.18, v1.0.191, minor version update, experience improvement
    * Scenes default to display tutorial.
    * Support SRT address separation, play without secret.
    * Separate Platform module, simplify mgmt logic.
    * Improve UTest upgrade test script.
    * Support changing stream name, randomly generating stream name.
    * Support copying stream name, configuration, address, etc.
    * Separate upgrade and UI, simplify mgmt logic.
    * Separate container management and upgrade.
    * Fast and efficient upgrade, completed within 30 seconds.
    * Support CVM image, see [SRS CVM](https://mp.weixin.qq.com/s/x-PjoKjJj6HRF-eCKX0KzQ).
* 2022.03.16, v1.0.162, Major version update, error handling, Release 4.3
    * Support for React Error Boundary, friendly error display.
    * Support for RTMP push QR code, core image guidance.
    * Support for simple player, playing HTTP-FLV and HLS.
    * Improved callbacks, created with React.useCallback.
    * Improved page cache time, increased loading speed.
    * Added REACT UI components and Nodejs project testing.
    * Added script for installing dependency packages.
    * Improved simple player, not muted by default, requires user click to play.
    * Added Watermelon Player [xgplayer](https://github.com/bytedance/xgplayer), playing FLV and HLS
* 2022.03.09, v1.0.144, Minor version update, multi-platform forwarding
    * Support for multi-platform forwarding, video number, Bilibili, Kuaishou.
    * Restart forwarding task when modifying forwarding configuration.
    * Support for setting upgrade window, default upgrade from 23:00 to 5:00.
    * Support for jest unit testing, covering mgmt.
    * Support for switching SRS, stable version and development version.
    * Optimized display of disabled container status.
* 2022.03.04, v1.0.132, Minor version update, cloud on-demand
    * Support for cloud on-demand, HLS and MP4 downloads.
    * Cloud on-demand supports live playback, updating SessionKey.
    * Disable password setting during upgrade to avoid environment variable conflicts.
    * Restart all containers dependent on .env when initializing the system.
    * Update the differences between cloud recording and cloud on-demand.
    * SRT supports vMix tutorial.
* 2022.02.25, v1.0.120, Minor version update, cloud recording
    * Improved upgrade script, restarting necessary containers.
    * Modified Redis listening port, enhanced security.
    * Resolved cloud recording, asynchronous long time (8h+) conflict issue.
    * Improved key creation link, using cloud API key.
    * Improved scene and settings TAB, loaded on demand, URL address identification.
* 2022.02.23, v1.0.113, Minor version update, cloud recording
    * Support for resetting push key. [#2](https://github.com/ossrs/srs-terraform/pull/2)
    * SRT push disconnects when RTMP conversion fails.
    * Disabled containers no longer start.
    * SRT supports QR code scanning for push and playback. [#6](https://github.com/ossrs/srs-terraform/pull/6)
    * Support for [cloud recording](https://mp.weixin.qq.com/s/UXR5EBKZ-LnthwKN_rlIjg), recording to Tencent Cloud COS.
* 2022.02.14, v1.0.98, Major version update, upgrade, Release 4.2
    * Improved React static resource caching, increasing subsequent loading speed.
    * Added Contact exclusive group QR code, scan code to join group.
    * Support for setting Redis values, disabling automatic updates.
    * Automatically detect overseas regions, use overseas sources for updates and upgrades.
    * Improved upgrade prompts, countdown and status detection.
    * Display video tutorials created by everyone on the page, sorted by play count.
    * Support for authorizing platform administrators to access Lighthouse instances.
    * Small memory systems, automatically create swap to avoid OOM during upgrades.
* 2022.02.05, v1.0.74, minor update, dashboard
    * Support for Prometheus monitoring, WebUI mounted on `/prometheus`, no authentication for now.
    * Support for Prometheus NodeExporter, node monitoring, Lighthouse's CPU, network, disk, etc.
    * Added dashboard, added CPU chart, can jump to [Prometheus](https://github.com/ossrs/srs/issues/2899#prometheus).
    * Improved certbot, started with docker, not an installation package.
    * Improved upgrade process to prevent duplicate upgrades.
    * Support for upgrading machines with 1GB memory, disabling node's GENERATE_SOURCEMAP to prevent OOM.
* 2022.02.01, v1.0.64, minor update, HTTPS
    * Support for Windows version of ffplay to play SRT addresses
    * Support for container startup hooks, stream authentication and authorization
    * Change Redis listening on lo and eth0, otherwise container cannot access
    * Support for setting HTTPS certificates, Nginx format, refer to [here](https://github.com/ossrs/srs/issues/2864#ssl-file)
    * Support for Let's Encrypt automatic application of HTTPS certificates, refer to [here](https://github.com/ossrs/srs/issues/2864#lets-encrypt)
* 2022.01.31, v1.0.58, minor update, SRT
    * Support for ultra-clear real-time live streaming scenarios, SRT push and pull streaming, 200~500ms latency, refer to [here](https://github.com/ossrs/srs/issues/1147#lagging)
    * Chip/OBS+SRS+ffplay push and pull SRT stream address, support authentication.
    * Support for manual upgrade to the latest version, support for forced upgrade.
    * Improved upgrade script, execute after updating the script
    * Support for restarting SRS server container
* 2022.01.27, v1.0.42, major update, stream authentication, Release 4.1
    * Support for push stream authentication and management backend
    * Support for updating backend, manual update
    * Live room scenario, push stream and play guide
    * SRS source code download, with GIT
    * Support for Lighthouse image, refer to [SRS Lighthouse](https://mp.weixin.qq.com/s/fWmdkw-2AoFD_pEmE_EIkA).
* 2022.01.21, Initialized.
