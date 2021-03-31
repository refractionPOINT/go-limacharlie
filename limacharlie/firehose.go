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
	"io"
	"math/big"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
)

const (
	readBufferSize = 512 * 1024
)

// FirehoseOutputOptions holds the optional parameter for firehose output
type FirehoseOutputOptions struct {
	// Name to register as an Output
	UniqueName string

	// Type of data received from the cloud as specified in Output
	Type OutputDataType

	// Only receive events from this SensorID.
	SensorID string

	// Only receive events marked with this investigation ID
	// Optional
	InvestigationID string

	// Only receive events from sensor with this tag
	// Optional
	Tag string

	// Only receive detections of this category
	// Optional
	Category string

	// If set to true, delete the firehose output on failure (in LC cloud)
	// Optional
	IsDeleteOnFailure bool

	// If set to true, do not validate certs, useful for self-signed certs.
	IsNotStrictSSL bool
}

// FirehoseOptions holds the parameters for the firehose
type FirehoseOptions struct {
	// IP to listen on
	ListenOnPort uint16

	// Port to listen on
	ListenOnIP net.IP

	// IP that LC should use to connect to this object
	ConnectToPort uint16

	// Port that LC should use to connect to this object
	ConnectTo string

	// Path to the SSL cert file (PEM) to use to receive from the cloud
	// Optional
	// If not set, generates self-signed certificate
	SSLCertPath string

	// Path to the SSL key file (PEM) to use to receive from the cloud
	// Optional
	// If not set, generates self-signed certificate
	SSLCertKeyPath string

	// Maximum number of message to buffer in the queue
	// Once the queue is full, messages will be considered as dropped
	MaxMessageCount int

	// Maximum number of dropped message to buffer
	// Once the queue is full, dropped count will continue to raise but will not be sent to the queue
	MaxErrorMessageCount int

	// If set to true, the data received will be parsed to json
	ParseMessage bool
}

// FirehoseMessage holds the content of a message received from a firehose
type FirehoseMessage struct {
	// Message content
	RawContent string
	Content    map[string]interface{}
}

// Firehose is a listener to receive data from a limacharlie.io organization in push mode
type Firehose struct {
	// Organization linked to this firehose
	Organization *Organization
	opts         FirehoseOptions
	outputOpts   *FirehoseOutputOptions

	// Channel to receive the message from
	Messages chan FirehoseMessage

	// Channel to receive messages that could not be parsed
	// It will only be used if the supplied FirehoseOptions require message to be parsed
	ErrorMessages chan FirehoseMessage

	messageDropCount int32
	listenerConfig   *tls.Config
	listener         net.Listener

	mutex         sync.Mutex
	wgFeeders     sync.WaitGroup
	activeFeeders map[net.Conn]struct{}
	isRunning     bool
}

type firehoseHandler struct {
	Options FirehoseOptions
}

func createSelfSignedCertificate() (*tls.Certificate, error) {
	certPrivateKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, err
	}
	certTemplate := x509.Certificate{
		SerialNumber: big.NewInt(1),
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
	certBytes, err := x509.CreateCertificate(rand.Reader, &certTemplate, &certTemplate, &certPrivateKey.PublicKey, certPrivateKey)
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

func startListener(listenOnIP net.IP, listenOnPort uint16, sslCertPath string, sslCertKeyPath string) (*net.Listener, error) {
	createTempCert := len(sslCertPath) == 0
	createTempKey := len(sslCertKeyPath) == 0
	if createTempCert && !createTempKey {
		return nil, fmt.Errorf("certificate key path missing")
	}
	if !createTempCert && createTempKey {
		return nil, fmt.Errorf("certificate path missing")
	}

	var certificate *tls.Certificate
	if createTempCert && createTempKey {
		tempCert, err := createSelfSignedCertificate()
		if err != nil {
			return nil, fmt.Errorf("could not create self signed certificate: %s", err)
		}
		certificate = tempCert
	} else {
		tempCert, err := tls.LoadX509KeyPair(sslCertPath, sslCertKeyPath)
		if err != nil {
			return nil, fmt.Errorf("error loading certificate with cert path '%s' and key path '%s': %s", sslCertPath, sslCertKeyPath, err)
		}
		certificate = &tempCert
	}

	tlsConfig := tls.Config{
		Certificates: []tls.Certificate{*certificate},
		CipherSuites: []uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256},
	}

	tlsListener, err := tls.Listen("tcp", fmt.Sprintf("%s:%d", listenOnIP, listenOnPort), &tlsConfig)
	if err != nil {
		return nil, fmt.Errorf("could not start TLS listener: %s", err)
	}
	return &tlsListener, nil
}

func (org *Organization) registerOutput(fhOpts FirehoseOutputOptions, dest string) error {
	if fhOpts.UniqueName == "" {
		log.Info().Msg("output registration not required")
		return nil
	}

	outputName := "tmp_live_" + fhOpts.UniqueName

	output := OutputConfig{
		Name:            outputName,
		Module:          OutputTypes.Syslog,
		Type:            fhOpts.Type,
		DestinationHost: dest,
		SensorID:        fhOpts.SensorID,
		InvestigationID: fhOpts.InvestigationID,
		Tag:             fhOpts.Tag,
		Category:        fhOpts.Category,
		DeleteOnFailure: fhOpts.IsDeleteOnFailure,
		StrictTLS:       !fhOpts.IsNotStrictSSL,
		TLS:             true,
		NoHeader:        true,
	}
	_, err := org.OutputAdd(output)
	if err != nil {
		return fmt.Errorf("could not add output: %s", err)
	}
	log.Debug().Msg("output registration done")
	return nil
}

func (org *Organization) unregisterOutput(fhOutputOpts FirehoseOutputOptions) {
	if fhOutputOpts.UniqueName == "" {
		return
	}

	log.Debug().Msg("unregistering output")
	_, err := org.OutputDel("tmp_live_" + fhOutputOpts.UniqueName)
	if err != nil {
		log.Err(err).Msg("could not delete output")
	}
}

func (fhOpts FirehoseOptions) makeTLSConfig() (*tls.Config, error) {
	createTempCert := len(fhOpts.SSLCertPath) == 0
	createTempKey := len(fhOpts.SSLCertKeyPath) == 0
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
		tempCert, err := tls.LoadX509KeyPair(fhOpts.SSLCertPath, fhOpts.SSLCertKeyPath)
		if err != nil {
			return nil, fmt.Errorf("error loading certificate with cert path '%s' and key path '%s': %s", fhOpts.SSLCertPath, fhOpts.SSLCertKeyPath, err)
		}
		certificate = &tempCert
	}

	tlsConfig := tls.Config{
		Certificates: []tls.Certificate{*certificate},
		CipherSuites: []uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256},
	}
	return &tlsConfig, nil
}

// NewFirehose initialize the firehose
func NewFirehose(org *Organization, fhOpts FirehoseOptions, fhOutputOpts *FirehoseOutputOptions) (*Firehose, error) {
	tlsConfig, err := fhOpts.makeTLSConfig()
	if err != nil {
		return nil, fmt.Errorf("could not make tls config: %s", err)
	}
	fh := &Firehose{
		Organization:     org,
		opts:             fhOpts,
		outputOpts:       fhOutputOpts,
		Messages:         make(chan FirehoseMessage, fhOpts.MaxMessageCount),
		ErrorMessages:    make(chan FirehoseMessage, fhOpts.MaxErrorMessageCount),
		messageDropCount: 0,
		listenerConfig:   tlsConfig,
		listener:         nil,
		activeFeeders:    map[net.Conn]struct{}{},
	}
	return fh, nil
}

// Start register the optional output to limacharlie.io and start listening for data
func (fh *Firehose) Start() error {
	fh.mutex.Lock()
	defer fh.mutex.Unlock()
	if fh.isRunning {
		return fmt.Errorf("firehose already started")
	}
	fh.isRunning = true

	// start the listener
	listener, err := tls.Listen("tcp", fmt.Sprintf("%s:%d", fh.opts.ListenOnIP, fh.opts.ListenOnPort), fh.listenerConfig)
	if err != nil {
		return fmt.Errorf("could not start TLS listener: %s", err)
	}

	if fh.outputOpts != nil {
		dest := fmt.Sprintf("%s:%d", fh.opts.ConnectTo, fh.opts.ConnectToPort)
		err := fh.Organization.registerOutput(*fh.outputOpts, dest)
		if err != nil {
			log.Info().Msg("shutting down listener")
			listener.Close()
			return err
		}
	}

	fh.listener = listener
	go fh.handleConnections()
	log.Info().Msg("firehose started")
	return nil
}

func (fh *Firehose) handleConnections() {
	log.Debug().Msg(fmt.Sprintf("listening for connections on %s:%d", fh.opts.ListenOnIP, fh.opts.ListenOnPort))

	var err error

	defer log.Debug().Msg(fmt.Sprintf("stopped listening for connections on %s:%d (%v)", fh.opts.ListenOnIP, fh.opts.ListenOnPort, err))

	for fh.IsRunning() {
		var conn net.Conn
		conn, err = fh.listener.Accept()
		if err != nil {
			break
		}
		fh.mutex.Lock()
		if !fh.isRunning {
			fh.mutex.Unlock()
			conn.Close()
			break
		}
		fh.activeFeeders[conn] = struct{}{}
		fh.wgFeeders.Add(1)
		fh.mutex.Unlock()
		go fh.handleConnection(conn)
	}
}

func (fh *Firehose) handleConnection(conn net.Conn) {
	log.Debug().Msg("new incoming connection")
	defer log.Debug().Msg("incoming connection disconnected")
	defer func() {
		fh.mutex.Lock()
		delete(fh.activeFeeders, conn)
		fh.mutex.Unlock()
		conn.Close()
	}()
	defer fh.wgFeeders.Done()

	readBuffer := make([]byte, readBufferSize)
	currentData := make([]byte, 0, readBufferSize*2)
	for fh.IsRunning() {
		sizeRead, err := conn.Read(readBuffer[:])
		if err != nil {
			if err != io.EOF {
				log.Err(err).Msg("error reading from connection")
			}
			return
		}

		data := readBuffer[:sizeRead]
		dataStart := 0

		for i, b := range data {
			if b == 0x0a {
				// Found a newline, so we can use what we
				// have accumulated before plus this as
				// a message.
				if i-1 > dataStart {
					currentData = append(currentData, data[dataStart:i]...)
				}
				dataStart = i + 1
				fh.handleMessage(currentData)
				currentData = make([]byte, 0, readBufferSize*2)
				continue
			}
			if len(data)-1 == i {
				// This is the end of the buffer and
				// we got no newline, keep it for later.
				currentData = append(currentData, data[dataStart:i+1]...)
			}
		}
	}
}

func (fh *Firehose) handleMessage(raw []byte) {
	fhMessage := FirehoseMessage{RawContent: string(raw)}

	if fh.opts.ParseMessage {
		if err := json.Unmarshal([]byte(fhMessage.RawContent), &fhMessage.Content); err != nil {
			log.Warn().Msg(fmt.Sprintf("error parsing: %v", fhMessage.RawContent))
			select {
			case fh.ErrorMessages <- fhMessage:
			default:
				// Error channel is full.
				log.Warn().Msg("maximum error message count reached")
			}
			return
		}
	}

	// Are we over-queue?
	select {
	case fh.Messages <- fhMessage:
		// Success
	default:
		// Channel is full.
		atomic.AddInt32(&fh.messageDropCount, 1)
		// Try to generate an error message.
		select {
		case fh.ErrorMessages <- fhMessage:
		default:
			// Error channel is full.
			log.Warn().Msg("maximum error message count reached")
		}
	}
}

// Shutdown stops the listener and delete the output previsouly registered if any
func (fh *Firehose) Shutdown() {
	log.Debug().Msg("closing firehose")
	fh.mutex.Lock()

	if !fh.isRunning {
		fh.mutex.Unlock()
		return
	}
	fh.isRunning = false
	fh.mutex.Unlock()

	listener := fh.listener
	fh.listener = nil
	listener.Close()

	if fh.outputOpts != nil {
		fh.Organization.unregisterOutput(*fh.outputOpts)
	}

	fh.mutex.Lock()
	for conn := range fh.activeFeeders {
		go conn.Close()
		delete(fh.activeFeeders, conn)
	}
	fh.mutex.Unlock()

	fh.wgFeeders.Wait()

	close(fh.Messages)
	close(fh.ErrorMessages)
	log.Debug().Msg("firehose closed")
}

// IsRunning will return true if firehose has been started
func (fh *Firehose) IsRunning() bool {
	fh.mutex.Lock()
	defer fh.mutex.Unlock()
	return fh.isRunning
}

// GetMessageDropCount returns the current count of dropped messages
func (fh *Firehose) GetMessageDropCount() int {
	return int(fh.messageDropCount)
}

// ResetMessageDropCount reset the count of dropped messages
func (fh *Firehose) ResetMessageDropCount() {
	fh.messageDropCount = 0
}
