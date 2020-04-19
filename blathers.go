package blathers

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"

	"github.com/google/go-github/v30/github"
)

// blathersServer is the server that powers Blathers.
type blathersServer struct {
	githubAppSecret     string
	githubAppPrivateKey *rsa.PrivateKey
	githubClientID      int64
	opsgenieAPIKey      string

	tokenStoreMu struct {
		store map[installationID]*github.InstallationToken
		sync.Mutex
	}
}

// srv is started in init() to be compatible with Google Cloud Function.
var srv *blathersServer

func init() {
	// TODO(otan): use a config file instead.
	pk, err := processGithubAppPrivateKey(os.Getenv("BLATHERS_GITHUB_PRIVATE_KEY"))
	if err != nil {
		log.Fatalf("failed inferring private key: %s", err.Error())
	}

	githubClientID := int64(59700)
	if str := os.Getenv("BLATHERS_GITHUB_CLIENT_ID"); str != "" {
		var err error
		githubClientID, err = strconv.ParseInt(str, 10, 64)
		if err != nil {
			log.Fatalf("failed inferring client id: %s", err.Error())
		}
	}

	srv = &blathersServer{
		githubAppSecret:     os.Getenv("BLATHERS_GITHUB_APP_SECRET"),
		opsgenieAPIKey:      os.Getenv("BLATHERS_OPSGENIE_API_KEY"),
		githubAppPrivateKey: pk,
		githubClientID:      githubClientID,
	}
}

// CloudFunction is a compatibility piece for running on Cloud Functions.
func CloudFunction(w http.ResponseWriter, r *http.Request) {
	srv.HandleGithubWebhook(w, r)
}

// Server returns the blathers server. This is made to be compatible with
// running as main.
func Server() *blathersServer {
	return srv
}

func processGithubAppPrivateKey(base64Key string) (*rsa.PrivateKey, error) {
	key, err := base64.StdEncoding.DecodeString(base64Key)
	if err != nil {
		return nil, err
	}
	data, _ := pem.Decode([]byte(key))
	return x509.ParsePKCS1PrivateKey(data.Bytes)
}
