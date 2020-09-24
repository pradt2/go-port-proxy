package connectivity

import (
	"net"
	"crypto/x509"
	"crypto/tls"
)

type ConnFactory interface {
	Connect() (net.Conn, error)
	Listen() (net.Listener, error)
	GetNetworkType() string
	GetAddress() string
}

type tcpFactory struct {
	networkType string
	address     string
}

func NewTCPConnectionFactory(networkType string, address string) ConnFactory {
	return &tcpFactory{
		networkType: networkType,
		address:     address,
	}
}

func (f *tcpFactory) Connect() (net.Conn, error) {
	return net.Dial(f.networkType, f.address)
}

func (f *tcpFactory) Listen() (net.Listener, error) {
	return net.Listen(f.networkType, f.address)
}

func (f *tcpFactory) GetNetworkType() string {
	return f.networkType
}

func (f *tcpFactory) GetAddress() string {
	return f.address
}

type tlsFactory struct {
	tcpFactory
	config *tls.Config
}

func NewTLSConnectionFactory(rootCert string, key string, cert string, onlyAllowRootCertSignedClients bool, networkType string, address string) ConnFactory {
	roots := x509.NewCertPool()
	ok := roots.AppendCertsFromPEM([]byte(rootCert))
	if !ok {
		log.Fatal("Could not parse the root certificate. Cause is unknown")
	}
	cer, err := tls.X509KeyPair([]byte(cert), []byte(key))
	if err != nil {
		log.Fatalf("Could not parse the server TLS key pair. Cause: %s", err)
	}
	config := &tls.Config{
		RootCAs:      roots,
		Certificates: []tls.Certificate{cer},
	}
	if onlyAllowRootCertSignedClients {
		config.ClientCAs = roots
		config.ClientAuth = tls.RequireAndVerifyClientCert
	}
	return &tlsFactory{
		tcpFactory: tcpFactory{
			networkType: networkType,
			address:     address,
		},
		config: config,
	}
}

func (f *tlsFactory) Connect() (net.Conn, error) {
	return tls.Dial(f.networkType, f.address, f.config)
}

func (f *tlsFactory) Listen() (net.Listener, error) {
	return tls.Listen(f.networkType, f.address, f.config)
}

func (f *tlsFactory) GetNetworkType() string {
	return f.networkType
}

func (f *tlsFactory) GetAddress() string {
	return f.address
}
