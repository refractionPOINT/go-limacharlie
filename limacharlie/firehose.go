package limacharlie

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"net"
	"net/http"
	"time"
)

type FirehoseOptions struct {
	Name              string
	ListenOnPort      int
	ListenOnIP        net.IP
	ConnectToPort     int
	ConnectToIP       net.IP
	SSLCertPath       string
	SSLCertKeyPath    string
	MaxMessageCount   int
	InvestigationID   string
	EventTag          string
	DetectionCategory string
	SensorID          string
	DeleteOnFailure   bool
	// is_parse (bool): if set to True (default) the data will be parsed as JSON to native Python.
	// on_dropped (func): callback called with a data item when the item will otherwise be dropped.
}

type Firehose struct {
	Organization Organization
}

type firehoseHandler struct {
	Options FirehoseOptions
}

func (firehoseHandler) ServeHTTP(responseWriter http.ResponseWriter, request *http.Request) {

}

func createSelfSignedCertificate() (*tls.Certificate, error) {
	certPrivateKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, err
	}
	certTemplate := x509.Certificate{
		// SerialNumber: big.NewInt(1658),
		Subject: pkix.Name{
			CommonName:    "limacharlie_firehose",
			Organization:  []string{"refractionPOINT"},
			Locality:      []string{"Mountain View"},
			Province:      []string{"CA"},
			Country:       []string{"US"},
			StreetAddress: []string{""},
			PostalCode:    []string{""},
		},
		IPAddresses: []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().AddDate(10, 0, 0),
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
		KeyUsage:    x509.KeyUsageDigitalSignature,
		IsCA:        true,
	}
	certBytes, err := x509.CreateCertificate(rand.Reader, &certTemplate, &certTemplate, certPrivateKey.PublicKey, certPrivateKey)
	if err != nil {
		return nil, err
	}
	certPEM := new(bytes.Buffer)
	pem.Encode(certPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	})
	certPrivKeyPEM := new(bytes.Buffer)
	pem.Encode(certPrivKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(certPrivateKey),
	})
	cert, err := tls.X509KeyPair(certPEM.Bytes(), certPrivKeyPEM.Bytes())
	if err != nil {
		return nil, err
	}
	return &cert, nil
}

func Start(org Organization, opts FirehoseOptions) (*Firehose, error) {
	createTempCert := len(opts.SSLCertPath) == 0
	createTempKey := len(opts.SSLCertKeyPath) == 0
	if createTempCert && !createTempKey {
		return nil, fmt.Errorf("certificate key path missing")
	}
	if !createTempCert && createTempKey {
		return nil, fmt.Errorf("certificate path missing")
	}

	var certificate *tls.Certificate = nil
	if createTempCert && createTempKey {
		tempCert, err := createSelfSignedCertificate()
		if err != nil {
			return nil, fmt.Errorf("could not create self signed certificate: %s", err)
		}
		certificate = tempCert
	} else {
		tempCert, err := tls.LoadX509KeyPair(opts.SSLCertPath, opts.SSLCertKeyPath)
		if err != nil {
			return nil, fmt.Errorf("error loading certificate with cert path '%s' and key path '%s': %s", opts.SSLCertPath, opts.SSLCertKeyPath, err)
		}
		certificate = &tempCert
	}

	tlsConfig := tls.Config{
		Certificates: []tls.Certificate{*certificate},
		CipherSuites: []uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256},
	}
	srv := &http.Server{
		Addr:        fmt.Sprintf("%s:%s", opts.ListenOnIP, opts.ListenOnPort),
		TLSConfig:   &tlsConfig,
		Handler:     firehoseHandler{opts},
		IdleTimeout: time.Duration(5 * time.Second),
	}
	srv.ListenAndServeTLS("", "")
	return nil, nil
}
