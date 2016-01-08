package server

import (
	"crypto/tls"
	"io/ioutil"
	"ngrok/server/assets"
)

func LoadTLSConfig(crtPath string, keyPath string) (tlsConfig *tls.Config, err error) {
	fileOrAsset := func(path string, default_path string) ([]byte, error) {
		loadFn := ioutil.ReadFile
		if path == "" {
			loadFn = assets.Asset
			path = default_path
		}

		return loadFn(path)
	}

	var (
		crt  []byte
		key  []byte
		cert tls.Certificate
	)

	if crt, err = fileOrAsset(crtPath, "assets/server/tls/snakeoil.crt"); err != nil {
		return
	}

	if key, err = fileOrAsset(keyPath, "assets/server/tls/snakeoil.key"); err != nil {
		return
	}

	if cert, err = tls.X509KeyPair(crt, key); err != nil {
		return
	}

	//https://github.com/golang/go/issues/9364
	//log.Info("MinVersion:", tls.VersionSSL30)
	tlsConfig = &tls.Config{
		MinVersion:   tls.VersionSSL30,
		Certificates: []tls.Certificate{cert},
	}

	return
}
