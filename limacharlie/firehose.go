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
	"github.com/rs/zerolog/log"
	"math/big"
	"net"
	"time"
)

// FirehoseOutputOptions holds the optional parameter for firehose output
type FirehoseOutputOptions struct {
	// Name to register as an Output
	UniqueName string

	// Type of data received from the cloud as specified in Output
	Type OutputDataType

	// Only receive events marked with this investigation ID
	// Optional
	InvestigationID *string

	// Only receive events from sensor with this tag
	// Optional
	Tag *string

	// Only receive detections of this category
	// Optional
	Category *string

	// If set to true, delete the firehose output on failure (in LC cloud)
	// Optional
	IsDeleteOnFailure *bool
}

func makeGenericOutput(opts FirehoseOutputOptions) GenericOutputConfig {
	output := GenericOutputConfig{
		Name:   opts.UniqueName,
		Module: OutputTypes.Syslog,
		Type:   opts.Type,
	}
	if opts.InvestigationID != nil {
		output.InvestigationID = *opts.InvestigationID
	}
	if opts.Tag != nil {
		output.Tag = *opts.Tag
	}
	if opts.Category != nil {
		output.Category = *opts.Category
	}
	if opts.IsDeleteOnFailure != nil {
		output.DeleteOnFailure = *opts.IsDeleteOnFailure
	}
	return output
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
	ConnectToIP net.IP

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
	Content string
}

// Firehose is a listener to receive data from a limacharlie.io organization in push mode
type Firehose struct {
	// Organization linked to this firehose
	Organization Organization
	opts         FirehoseOptions
	outputOpts   *FirehoseOutputOptions

	// Channel to receive the message from
	Messages chan FirehoseMessage

	// Channel to receive messages that could not be parsed
	// It will only be used if the supplied FirehoseOptions require message to be parsed
	ErrorMessages chan FirehoseMessage

	messageDropCount int
	listener         net.Listener
	shutdownMessage  chan bool
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

	var certificate *tls.Certificate = nil
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

func registerOutput(org Organization, outputOpts FirehoseOutputOptions) error {
	if outputOpts.UniqueName == "" {
		log.Info().Msg("output registration not required")
		return nil
	}

	outputName := "tmp_live_" + outputOpts.UniqueName
	allOutputs, err := org.Outputs()
	if err != nil {
		return fmt.Errorf("could not register output with name '%s': %s", outputName, err)
	}

	_, found := allOutputs[outputName]
	if found {
		log.Debug().Str("name", outputName).Msg("output registration already done")
		return nil
	}

	output := makeGenericOutput(outputOpts)
	_, err = org.OutputAdd(output)
	if err != nil {
		return fmt.Errorf("could not add output: %s", err)
	}
	log.Debug().Msg("output registration done")
	return nil
}

func StartFirehose(org Organization, fhOpts FirehoseOptions) (Firehose, error) {
	return StartFirehoseAndRegisterOutput(org, fhOpts, nil)
}

func StartFirehoseAndRegisterOutput(org Organization, fhOpts FirehoseOptions, fhOutputOpts *FirehoseOutputOptions) (Firehose, error) {
	if fhOutputOpts != nil {
		err := registerOutput(org, *fhOutputOpts)
		if err != nil {
			return Firehose{}, err
		}
	}

	listener, err := startListener(fhOpts.ListenOnIP, fhOpts.ListenOnPort, fhOpts.SSLCertPath, fhOpts.SSLCertKeyPath)
	if err != nil {
		return Firehose{}, err
	}

	fh := Firehose{org,
		fhOpts,
		fhOutputOpts,
		make(chan FirehoseMessage, fhOpts.MaxMessageCount),
		make(chan FirehoseMessage, fhOpts.MaxErrorMessageCount),
		0,
		*listener,
		make(chan bool, 1)}
	go fh.handleConnections()
	return fh, nil
}

func (fh Firehose) handleConnections() {
	readBufferSize := 1024 * 512
	currentData := make([]byte, readBufferSize*2)

	log.Debug().Msg("listening for connections")
	for fh.IsRunning() {
		conn, err := fh.listener.Accept()
		if err != nil {
			continue
		}
		log.Debug().Msg("new incoming connection")
		defer conn.Close()

		readBuffer := make([]byte, readBufferSize)
		sizeRead, err := conn.Read(readBuffer)
		if err != nil {
			log.Err(err).Msg("error reading from connection")
			continue
		}
		if sizeRead == 0 {
			log.Debug().Msg("empty body read")
			continue
		}
		chunks := bytes.Split(readBuffer, []byte{0x0a})
		isContinuation := len(chunks) == 1
		if isContinuation {
			currentData = append(currentData[:len(currentData)], chunks[0]...)
			continue
		}
		for _, chunk := range chunks {
			currentData = append(currentData[:len(currentData)], chunk...)
			if len(currentData) == 0 {
				continue
			}
			fh.handleMessage(currentData)
			currentData = append(currentData[:0], currentData[len(currentData):]...)
		}
	}
}

func (fh Firehose) handleMessage(raw []byte) {
	fhMessage := FirehoseMessage{string(raw)}
	if fh.opts.ParseMessage {
		isValid := json.Valid(raw)
		if isValid && len(fh.Messages) < fh.opts.MaxMessageCount {
			fh.Messages <- fhMessage
		} else {
			fh.messageDropCount++
			if len(fh.ErrorMessages) < fh.opts.MaxErrorMessageCount {
				fh.ErrorMessages <- fhMessage
			} else {
				log.Warn().Msg("maximum error message count reached")
			}
		}
	} else {
		fh.Messages <- fhMessage
	}
}

func (fh Firehose) Shutdown() {
	if !fh.IsRunning() {
		return
	}
	fh.shutdownMessage <- true
	defer fh.listener.Close()

	if fh.outputOpts != nil && fh.outputOpts.UniqueName != "" {
		log.Debug().Msg("unregistering output")
		_, err := fh.Organization.OutputDel(fh.outputOpts.UniqueName)
		if err != nil {
			log.Err(err).Msg("could not delete output")
		}
	}
	log.Debug().Msg("firehose closed")
}

func (fh Firehose) IsRunning() bool {
	return len(fh.shutdownMessage) == 0
}

func (fh Firehose) GetMessageDropCount() int {
	return fh.messageDropCount
}

func (fh Firehose) ResetMessageDropCount() {
	fh.messageDropCount = 0
}
