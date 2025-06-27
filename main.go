package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"flag"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/general252/webdav-server/disk"
	"github.com/general252/webdav-server/webdav"
)

var userPassword = map[string]string{
	"root": "123456!",
}

type WindowsHandler struct {
	handlers   []*webdav.Handler
	memHandler *webdav.Handler
}

func NewWindowsHandler() (*WindowsHandler, error) {
	var (
		handlers []*webdav.Handler
		memFS    = webdav.NewMemFS()
	)

	partitions, err := disk.Partitions() // true 表示获取所有分区
	if err != nil {
		log.Println(err)
		return nil, err
	}

	for _, partition := range partitions {
		p := partition.MountPoint
		if len(p) < 1 {
			continue
		}

		v := strings.ToLower(p[0:1])

		handlers = append(handlers, &webdav.Handler{
			Prefix:     fmt.Sprintf("/%v", v),
			FileSystem: webdav.Dir(fmt.Sprintf("%v:", v)),
			LockSystem: webdav.NewMemLS(),
		})

		_ = memFS.Mkdir(context.TODO(), v, 0755)
	}

	tis := &WindowsHandler{
		handlers: handlers,
		memHandler: &webdav.Handler{
			Prefix:     "/",
			FileSystem: memFS,
			LockSystem: webdav.NewMemLS(),
		},
	}

	return tis, nil
}

func (tis *WindowsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		tis.memHandler.ServeHTTP(w, r)
		return
	}

	for _, handler := range tis.handlers {
		if strings.HasPrefix(r.URL.Path, handler.Prefix) {
			handler.ServeHTTP(w, r)
			return
		}
	}

	http.NotFound(w, r)
}

func main() {
	var (
		certFile = "webdav.crt"
		keyFile  = "webdav.key"
		host     = "0.0.0.0"
		port     = 2080
	)

	flag.StringVar(&certFile, "cert", "webdav.crt", "cert file")
	flag.StringVar(&keyFile, "key", "webdav.key", "key file")
	flag.StringVar(&host, "host", "0.0.0.0", "host")
	flag.IntVar(&port, "port", 2080, "port")
	flag.Parse()
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	var handler http.Handler

	switch runtime.GOOS {
	case `windows`:
		if h, err := NewWindowsHandler(); err != nil {
			log.Println(err)
			return
		} else {
			handler = h
		}
	default:
		handler = &webdav.Handler{
			Prefix:     "/",
			FileSystem: webdav.Dir("/"),
			LockSystem: webdav.NewMemLS(),
		}
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// 认证拦截
		u, p, ok := r.BasicAuth()
		if !ok {
			w.Header().Set("WWW-Authenticate", `Basic realm="WebDAV"`)
			w.WriteHeader(401)
			return
		}
		if v, ok := userPassword[u]; !ok {
			w.Header().Set("WWW-Authenticate", `Basic realm="WebDAV"`)
			w.WriteHeader(401)
			return
		} else if v != p {
			w.Header().Set("WWW-Authenticate", `Basic realm="WebDAV"`)
			w.WriteHeader(401)
			return
		}

		handler.ServeHTTP(w, r)
	})

	server := &http.Server{
		Addr: fmt.Sprintf("%v:%v", host, port),
		TLSConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	if _, err := os.Stat(certFile); os.IsNotExist(err) {
		if err := genSelfSignedCert(certFile, keyFile); err != nil {
			log.Println(err)
			return
		}
	}

	log.Printf("webdav server start at %v", server.Addr)
	err := server.ListenAndServeTLS(certFile, keyFile)
	log.Println(err)
}

func genSelfSignedCert(certFile, keyFile string) error {
	// 1. 生成ECDSA私钥 (P-256曲线效率更高)
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}

	// 2. 配置证书模板
	serialNumber, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"My Company"},
			CommonName:   "myserver.com",
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().AddDate(100, 0, 0), // 10年有效期
		KeyUsage:  x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth, // 服务端证书用途
		},
		DNSNames: []string{"myserver.com", "localhost"},
	}

	// 3. 自签名
	certDER, err := x509.CreateCertificate(
		rand.Reader, &template, &template, &privateKey.PublicKey, privateKey,
	)
	if err != nil {
		return err
	}

	// 4. 保存证书和私钥(PEM格式)
	certOut, _ := os.Create(certFile)
	_ = pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	_ = certOut.Close()

	keyOut, _ := os.Create(keyFile)
	keyBytes, _ := x509.MarshalECPrivateKey(privateKey)
	_ = pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})
	_ = keyOut.Close()

	return nil
}
