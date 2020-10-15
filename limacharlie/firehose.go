package limacharlie

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net"
	"net/http"
	"strings"
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

type FirehoseMessage struct {
	Message string
	IsValid bool
}

type Firehose struct {
	Organization Organization
	Messages     chan FirehoseMessage
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
	tlsListener, err := tls.Listen("tcp", fmt.Sprintf("%s:%s", opts.ListenOnIP, opts.ListenOnPort), &tlsConfig)
	if err != nil {
		return nil, fmt.Errorf("could not start TLS listener: %s", err)
	}
	defer tlsListener.Close()

	messages := make(chan FirehoseMessage, 64)
	go handleConnections(tlsListener, opts, messages)

	// FIX return value
	return nil, nil
}

func handleConnections(listener net.Listener, opts FirehoseOptions, messages chan FirehoseMessage) {
	readBufferSize := 1024 * 512
	readBuffer := bytes.NewBuffer(make([]byte, readBufferSize))
	var currentData string
	for {
		conn, err := listener.Accept()
		if err != nil {
			// TODO log error
			continue
		}
		defer conn.Close()

		sizeRead, err := conn.Read(readBuffer.Bytes())
		if err != nil {
			// TODO log error
			continue
		}
		if sizeRead == 0 {
			// TODO log
			continue
		}
		chunks := strings.Split(readBuffer.String(), "\n")
		isContinuation := len(chunks) == 1
		if isContinuation {
			currentData += chunks[0]
			continue
		}
		for _, chunk := range chunks {
			currentData += chunk
			if len(currentData) == 0 {
				continue
			}
			isValid := json.Valid([]byte(currentData))
			// TODO add dropped count
			messages <- FirehoseMessage{currentData, isValid}
		}
	}
}
