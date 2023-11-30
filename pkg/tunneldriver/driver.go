package tunneldriver

import (
	"context"
	"crypto/x509"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-logr/logr"
	ingressv1alpha1 "github.com/ngrok/kubernetes-ingress-controller/api/v1alpha1"
	"github.com/ngrok/kubernetes-ingress-controller/internal/version"
	"golang.org/x/exp/maps"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"golang.ngrok.com/ngrok"
	"golang.ngrok.com/ngrok/config"
	logrok "golang.ngrok.com/ngrok/log"
)

type k8sLogger struct {
	logger logr.Logger
}

func (l k8sLogger) Log(ctx context.Context, level logrok.LogLevel, msg string, kvs map[string]interface{}) {
	keysAndValues := []any{}
	for k, v := range kvs {
		keysAndValues = append(keysAndValues, k, v)
	}
	l.logger.V(level-4).Info(msg, keysAndValues...)
}

const (
	// TODO: Make this configurable via helm and document it so users can
	// use it for things like proxies
	customCertsPath = "/etc/ssl/certs/ngrok/"
)

// TunnelDriver is a driver for creating and deleting ngrok tunnels
type TunnelDriver struct {
	session ngrok.Session
	tunnels map[string]ngrok.Forwarder
}

// TunnelDriverOpts are options for creating a new TunnelDriver
type TunnelDriverOpts struct {
	ServerAddr string
	Region     string
}

// New creates and initializes a new TunnelDriver
func New(logger logr.Logger, opts TunnelDriverOpts) (*TunnelDriver, error) {
	connOpts := []ngrok.ConnectOption{
		ngrok.WithClientInfo("ngrok-ingress-controller", version.GetVersion()),
		ngrok.WithAuthtokenFromEnv(),
		ngrok.WithLogger(k8sLogger{logger}),
	}

	if opts.Region != "" {
		connOpts = append(connOpts, ngrok.WithRegion(opts.Region))
	}

	if opts.ServerAddr != "" {
		connOpts = append(connOpts, ngrok.WithServer(opts.ServerAddr))
	}

	// Only configure custom certs if the directory exists
	if _, err := os.Stat(customCertsPath); !os.IsNotExist(err) {
		caCerts, err := caCerts()
		if err != nil {
			return nil, err
		}
		connOpts = append(connOpts, ngrok.WithCA(caCerts))
	}

	session, err := ngrok.Connect(context.Background(), connOpts...)
	if err != nil {
		return nil, err
	}
	return &TunnelDriver{
		session: session,
		tunnels: make(map[string]ngrok.Forwarder),
	}, nil
}

// caCerts combines the system ca certs with a directory of custom ca certs
func caCerts() (*x509.CertPool, error) {
	systemCertPool, err := x509.SystemCertPool()
	if err != nil {
		return nil, err
	}

	// Clone the system cert pool
	customCertPool := systemCertPool.Clone()

	// Read each .crt file in the custom cert directory
	files, err := os.ReadDir(customCertsPath)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if filepath.Ext(file.Name()) != ".crt" {
			continue
		}

		// Read the contents of the .crt file
		certBytes, err := os.ReadFile(filepath.Join(customCertsPath, file.Name()))
		if err != nil {
			return nil, err
		}

		// Append the cert to the custom cert pool
		customCertPool.AppendCertsFromPEM(certBytes)
	}

	return customCertPool, nil
}

// CreateTunnel creates and starts a new tunnel in a goroutine. If a tunnel with the same name already exists,
// it will be stopped and replaced with a new tunnel unless the labels match.
func (td *TunnelDriver) CreateTunnel(ctx context.Context, name string, labels map[string]string, backend *ingressv1alpha1.BackendConfig, destination string) error {
	log := log.FromContext(ctx)

	if tun, ok := td.tunnels[name]; ok {
		if maps.Equal(tun.Labels(), labels) {
			log.Info("Tunnel labels match existing tunnel, doing nothing")
			return nil
		}
		// There is already a tunnel with this name, start the new one and defer closing the old one
		//nolint:errcheck
		defer td.stopTunnel(context.Background(), tun)
	}
	protocol := "tcp"
	if backend != nil {
		protocol = backend.Protocol
	}
	destUrlStr := fmt.Sprintf("%s://%s", strings.ToLower(protocol), destination)
	destUrl, err := url.Parse(destUrlStr)
	if err != nil {
		return err
	}

	tun, err := td.session.ListenAndForward(ctx, destUrl, td.buildTunnelConfig(labels, destination))
	if err != nil {
		return err
	}
	td.tunnels[name] = tun
	return nil
}

// DeleteTunnel stops and deletes a tunnel
func (td *TunnelDriver) DeleteTunnel(ctx context.Context, name string) error {
	log := log.FromContext(ctx).WithValues("name", name)

	tun := td.tunnels[name]
	if tun == nil {
		log.Info("Tunnel not found while trying to delete tunnel")
		return nil
	}

	err := td.stopTunnel(ctx, tun)
	if err != nil {
		return err
	}
	delete(td.tunnels, name)
	log.Info("Tunnel deleted successfully")
	return nil
}

func (td *TunnelDriver) stopTunnel(ctx context.Context, tun ngrok.Forwarder) error {
	if tun == nil {
		return nil
	}
	return tun.CloseWithContext(ctx)
}

func (td *TunnelDriver) buildTunnelConfig(labels map[string]string, destination string) config.Tunnel {
	opts := []config.LabeledTunnelOption{}
	for key, value := range labels {
		opts = append(opts, config.WithLabel(key, value))
	}
	opts = append(opts, config.WithForwardsTo(destination))
	return config.LabeledTunnel(opts...)
}
