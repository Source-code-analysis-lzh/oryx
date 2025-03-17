// Copyright (c) 2022-2024 Winlin
//
// SPDX-License-Identifier: MIT
package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"os/exec"
	"path"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/ossrs/go-oryx-lib/errors"
	"github.com/ossrs/go-oryx-lib/logger"
)

var certManager *CertManager

type CertManager struct {
	// httpsCertificate is the certificate for the server.
	httpsCertificate *tls.Certificate

	// httpCertificateReload is used to reload the certificate.
	httpCertificateReload chan bool

	// certFileLock is used to lock the cert file nginx.key and nginx.crt.
	certFileLock sync.Mutex
}

// NewCertManager 创建并返回一个新的CertManager实例。
// 该函数初始化了CertManager结构体，并为其httpCertificateReload字段创建了一个容量为1的通道。
// 这个通道用于触发证书的重新加载操作，确保在需要时可以安全、高效地更新证书。
func NewCertManager() *CertManager {
	return &CertManager{
		httpCertificateReload: make(chan bool, 1),
	}
}

func (v *CertManager) Initialize(ctx context.Context) error {
	if envSelfSignedCertificate() == "on" {
		if err := v.createSelfSignCertificate(ctx); err != nil {
			return errors.Wrapf(err, "create self-signed certificate")
		}
	}

	return nil
}

func (v *CertManager) createSelfSignCertificate(ctx context.Context) error {
	var noKeyFile, noCrtFile bool
	func() {
		v.certFileLock.Lock()
		defer v.certFileLock.Unlock()

		keyFile := path.Join(conf.Pwd, "containers/data/config/nginx.key")
		crtFile := path.Join(conf.Pwd, "containers/data/config/nginx.crt")

		if _, err := os.Stat(keyFile); os.IsNotExist(err) {
			noKeyFile = true
		}
		if _, err := os.Stat(crtFile); os.IsNotExist(err) {
			noCrtFile = true
		}
	}()
	if !noKeyFile || !noCrtFile {
		logger.Tf(ctx, "cert: ignore for cert file exists")
		return nil
	}

	var key, crt string
	if true {
		privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			return errors.Wrapf(err, "generate ecdsa key")
		}

		template := x509.Certificate{
			SerialNumber: big.NewInt(1),
			Subject: pkix.Name{
				CommonName: "srs.stack.local",
			},
			NotBefore: time.Now(),
			NotAfter:  time.Now().AddDate(10, 0, 0),
			KeyUsage:  x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
			ExtKeyUsage: []x509.ExtKeyUsage{
				x509.ExtKeyUsageServerAuth,
			},
			BasicConstraintsValid: true,
		}

		derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
		if err != nil {
			return errors.Wrapf(err, "create certificate")
		}

		privateKeyBytes, err := x509.MarshalECPrivateKey(privateKey)
		if err != nil {
			return errors.Wrapf(err, "marshal ecdsa key")
		}

		privateKeyBlock := pem.Block{
			Type:  "EC PRIVATE KEY",
			Bytes: privateKeyBytes,
		}
		key = string(pem.EncodeToMemory(&privateKeyBlock))

		certBlock := pem.Block{
			Type:  "CERTIFICATE",
			Bytes: derBytes,
		}
		crt = string(pem.EncodeToMemory(&certBlock))
		logger.Tf(ctx, "cert: create self-signed certificate ok, key=%vB, crt=%vB", len(key), len(crt))
	}

	if err := v.updateSslFiles(ctx, key+"\n", crt+"\n"); err != nil {
		return errors.Wrapf(err, "updateSslFiles key=%vB, crt=%vB", len(key), len(crt))
	}

	if err := rdb.Set(ctx, SRS_HTTPS, "ssl", 0).Err(); err != nil && err != redis.Nil {
		return errors.Wrapf(err, "set %v %v", SRS_HTTPS, "ssl")
	}

	if err := nginxGenerateConfig(ctx); err != nil {
		return errors.Wrapf(err, "nginx config and reload")
	}
	logger.T(ctx, "cert: update self-signed certificate ok, key=%vB, crt=%vB", len(key), len(crt))

	return nil
}

// ReloadCertificate 重新加载证书。
// 该函数尝试将一个 true 值发送到 httpCertificateReload 通道，以触发证书的重新加载。
// 如果在发送过程中上下文被取消或超时，函数会直接返回，不会进行重新加载。
// 此方法主要用于在运行时更新证书，而无需重启应用程序。
func (v *CertManager) ReloadCertificate(ctx context.Context) {
	// 使用 select 语句尝试将 true 值发送到 httpCertificateReload 通道。
	// 如果通道满了（即没有人接收），则不会阻塞，直接进入 default 分支。
	// 如果上下文被取消或超时，也会退出函数。
	select {
	case v.httpCertificateReload <- true:
	case <-ctx.Done():
	default:
	}
}

func (v *CertManager) reloadCertificateFile(ctx context.Context) error {
	v.certFileLock.Lock()
	defer v.certFileLock.Unlock()

	keyFile := path.Join(conf.Pwd, "containers/data/config/nginx.key")
	crtFile := path.Join(conf.Pwd, "containers/data/config/nginx.crt")

	var noKeyFile, noCrtFile bool
	if _, err := os.Stat(keyFile); os.IsNotExist(err) {
		noKeyFile = true
	}
	if _, err := os.Stat(crtFile); os.IsNotExist(err) {
		noCrtFile = true
	}
	if noKeyFile || noCrtFile {
		logger.Tf(ctx, "cert: ignore for no cert file")
		return nil
	}

	cert, err := tls.LoadX509KeyPair(crtFile, keyFile)
	if err != nil {
		logger.Tf(ctx, "cert: ignore load cert %v, key %v failed", crtFile, keyFile)
		return nil
	}

	v.httpsCertificate = &cert
	logger.Tf(ctx, "cert: reload certificate file ok")

	return nil
}

func (v *CertManager) QueryCertificate() (string, string, error) {
	v.certFileLock.Lock()
	defer v.certFileLock.Unlock()

	keyFile := path.Join(conf.Pwd, "containers/data/config/nginx.key")
	crtFile := path.Join(conf.Pwd, "containers/data/config/nginx.crt")

	key, err := ioutil.ReadFile(keyFile)
	if err != nil {
		return "", "", errors.Wrapf(err, "read key %v", keyFile)
	}

	crt, err := ioutil.ReadFile(crtFile)
	if err != nil {
		return "", "", errors.Wrapf(err, "read crt %v", crtFile)
	}

	return string(key), string(crt), nil
}

// updateSslFiles update the ssl files.
func (v *CertManager) updateSslFiles(ctx context.Context, key, crt string) error {
	v.certFileLock.Lock()
	defer v.certFileLock.Unlock()

	keyFile := path.Join(conf.Pwd, "containers/data/config/nginx.key")
	crtFile := path.Join(conf.Pwd, "containers/data/config/nginx.crt")

	if err := exec.CommandContext(ctx, "rm", "-f", keyFile, crtFile).Run(); err != nil {
		return errors.Wrapf(err, "rm -f %v %v", keyFile, crtFile)
	}

	if err := ioutil.WriteFile(keyFile, []byte(key), 0644); err != nil {
		return errors.Wrapf(err, "write key %vB to %v", len(key), keyFile)
	}

	if err := ioutil.WriteFile(crtFile, []byte(crt), 0644); err != nil {
		return errors.Wrapf(err, "write crt %vB to %v", len(crt), crtFile)
	}

	return nil
}

// updateLetsEncrypt request letsencrypt and update the ssl files.
func (v *CertManager) updateLetsEncrypt(ctx context.Context, domain string) error {
	v.certFileLock.Lock()
	defer v.certFileLock.Unlock()

	defer v.ReloadCertificate(ctx)

	if true {
		args := []string{
			"--email", "srs.stack@gmail.com", "--domains", domain,
			"--http.webroot", path.Join(conf.Pwd, "containers/data"), "--http", "--accept-tos",
			"run",
		}
		cmd := exec.CommandContext(ctx, "lego", args...)
		cmd.Dir = path.Join(conf.Pwd, "containers/data/lego")

		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		if err := cmd.Run(); err != nil {
			return errors.Wrapf(err, "run lego %v in %v, stdout %v, stderr %v",
				args, cmd.Dir, stdout.String(), stderr.String())
		}
		logger.Tf(ctx, "run lego %v in %v ok, stdout %v, stderr %v",
			args, cmd.Dir, stdout.String(), stderr.String(),
		)
	}

	keyFile := path.Join(conf.Pwd, fmt.Sprintf("containers/data/lego/.lego/certificates/%v.key", domain))
	if _, err := os.Stat(keyFile); err != nil {
		return errors.Wrapf(err, "stat %v", keyFile)
	}

	crtFile := path.Join(conf.Pwd, fmt.Sprintf("containers/data/lego/.lego/certificates/%v.crt", domain))
	if _, err := os.Stat(crtFile); err != nil {
		return errors.Wrapf(err, "stat %v", crtFile)
	}

	targetKeyFile := path.Join(conf.Pwd, "containers/data/config/nginx.key")
	targetCrtFile := path.Join(conf.Pwd, "containers/data/config/nginx.crt")
	if err := exec.CommandContext(ctx, "rm", "-f", targetKeyFile, targetCrtFile).Run(); err != nil {
		return errors.Wrapf(err, "rm -f %v %v", targetKeyFile, targetCrtFile)
	}

	if true {
		source := fmt.Sprintf("../lego/.lego/certificates/%v.key", domain)
		cmd := exec.CommandContext(ctx, "ln", "-sf", source, "nginx.key")
		cmd.Dir = path.Join(conf.Pwd, "containers/data/config")
		if err := cmd.Run(); err != nil {
			return errors.Wrapf(err, "run %v in %v", cmd.Args, cmd.Dir)
		}
	}

	if true {
		source := fmt.Sprintf("../lego/.lego/certificates/%v.crt", domain)
		cmd := exec.CommandContext(ctx, "ln", "-sf", source, "nginx.crt")
		cmd.Dir = path.Join(conf.Pwd, "containers/data/config")
		if err := cmd.Run(); err != nil {
			return errors.Wrapf(err, "run %v in %v", cmd.Args, cmd.Dir)
		}
	}

	return nil
}

// renewLetsEncrypt request letsencrypt and update the ssl files.
func (v *CertManager) renewLetsEncrypt(ctx context.Context, domain string) error {
	defer v.ReloadCertificate(ctx)

	args := []string{
		"--email", "srs.stack@gmail.com", "--domains", domain,
		"--http.webroot", path.Join(conf.Pwd, "containers/data"), "--http",
		"renew", "--days", "30",
	}
	cmd := exec.CommandContext(ctx, "lego", args...)
	cmd.Dir = path.Join(conf.Pwd, "containers/data/lego")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return errors.Wrapf(err, "run lego %v in %v, stdout %v, stderr %v",
			args, cmd.Dir, stdout.String(), stderr.String())
	}
	logger.Tf(ctx, "run lego %v in %v ok, stdout %v, stderr %v",
		args, cmd.Dir, stdout.String(), stderr.String(),
	)

	return nil
}

func (v *CertManager) refreshSSLCert(ctx context.Context) error {
	provider, err := rdb.Get(ctx, SRS_HTTPS).Result()
	if err != nil && err != redis.Nil {
		return err
	}
	if provider != "lets" {
		logger.Tf(ctx, "cert: ignore ssl provider %v", provider)
		return nil
	}

	domain, err := rdb.Get(ctx, SRS_HTTPS_DOMAIN).Result()
	if err != nil && err != redis.Nil {
		return err
	}
	if domain == "" {
		logger.Tf(ctx, "cert: ignore ssl domain empty")
		return nil
	}

	if err := v.renewLetsEncrypt(ctx, domain); err != nil {
		return err
	} else {
		logger.Tf(ctx, "cert: renew ssl cert ok")
	}

	if err := nginxGenerateConfig(ctx); err != nil {
		return errors.Wrapf(err, "nginx config and reload")
	}

	logger.Tf(ctx, "cert: refresh ssl cert ok")
	return nil
}
