package main

import (
	"fmt"
	"github.com/akamensky/argparse"
	"github.com/google/uuid"
	lc "github.com/refractionPOINT/go-limacharlie/limacharlie"
	zerolog "github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/ssh/terminal"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func addParserOptionString(p *argparse.Parser, short string, long string, required bool, help string) *string {
	return p.String(short, long, &argparse.Options{Required: required, Help: help})
}

func addParserRequiredOptionsInt(p *argparse.Parser, long string, help string) *int {
	return p.Int("", long, &argparse.Options{Required: true, Help: help})
}

func validateOutputType(s []string) error {
	parsedType := lc.ParseOutputType(s[0])
	if parsedType == nil {
		return fmt.Errorf("output type is not supported")
	}
	return nil
}

func validateUUID(s []string) error {
	_, err := uuid.Parse(s[0])
	return err
}

func parsePort(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 16)
}

func validateIPPort(s []string) error {
	split := strings.Split(s[0], ":")
	if len(split) != 2 {
		return fmt.Errorf("invalid interface: %s", s[0])
	}
	parsedIP := net.ParseIP(split[0])
	if parsedIP == nil {
		return fmt.Errorf("IP does not have a valid form")
	}
	_, err := parsePort(split[1])
	if err != nil {
		return fmt.Errorf("port is invalid")
	}
	return nil
}

func getIPPort(s string) (net.IP, uint16) {
	arrArgListenOn := strings.Split(s, ":")
	listenOnIP := net.ParseIP(arrArgListenOn[0])
	listenOnPort, _ := parsePort(arrArgListenOn[1])
	return listenOnIP, uint16(listenOnPort)
}

func parseArgs() (lc.ClientOptions, lc.FirehoseOptions, lc.FirehoseOutputOptions, error) {
	argParser := argparse.NewParser("firehose", "limacharlie.io firehose")

	argListenOn := argParser.String("", "listen_interface", &argparse.Options{Required: true, Help: "the local interface to listen on for firehose connections, like '0.0.0.0:4444'.", Validate: validateIPPort})

	var dataTypes string
	for _, o := range lc.OutputDataTypes {
		dataTypes += fmt.Sprintf("'%s', ", o)
	}
	dataTypes = strings.TrimRight(dataTypes, ", ")
	argOutputType := argParser.String("", "data_type", &argparse.Options{Required: true, Help: fmt.Sprintf("the type of data to receive in firehose, one of: %s.", dataTypes), Validate: validateOutputType})

	argOID := argParser.String("o", "oid", &argparse.Options{Required: false, Help: "the OID to authenticate as, if not specified environment credentials are used.", Validate: validateUUID})
	argDestination := argParser.String("p", "public-destination", &argparse.Options{Required: false, Help: "", Validate: validateIPPort})
	outputName := argParser.String("n", "name", &argparse.Options{Required: false, Help: "unique name to use for this firehose, will be used to register a limacharlie.io Output if specified, otherwise assumes Output is already taken care of."})
	argInvestigationID := argParser.String("i", "investigation-id", &argparse.Options{Required: false, Help: "firehose should only receive events marked with this investigation id."})
	argTag := argParser.String("t", "tag", &argparse.Options{Required: false, Help: "firehose should only receive events from sensors tagged with this tag."})
	argCategory := argParser.String("c", "category", &argparse.Options{Required: false, Help: "firehose should only receive detections from this category."})
	// argSensorID := argParser.String("s", "sensor_id", &argparse.Options{Required: false, Help: "firehose should only receive detections and events from this sensor."})

	err := argParser.Parse(os.Args)
	if err != nil {
		fmt.Print(argParser.Usage(err))
		return lc.ClientOptions{}, lc.FirehoseOptions{}, lc.FirehoseOutputOptions{}, err
	}

	oid := ""
	if argOID != nil {
		oid = *argOID
	}
	listenOnIP, listenOnPort := getIPPort(*argListenOn)
	destinationIP := listenOnIP
	destinationPort := listenOnPort
	if argDestination != nil && *argDestination != "" {
		destinationIP, destinationPort = getIPPort(*argDestination)
	}

	isDeleteOnFailure := true
	return lc.ClientOptions{
			OID: oid,
		},
		lc.FirehoseOptions{
			ListenOnIP:    listenOnIP,
			ListenOnPort:  listenOnPort,
			ConnectToIP:   destinationIP,
			ConnectToPort: destinationPort,
		},
		lc.FirehoseOutputOptions{
			UniqueName:        *outputName,
			Type:              *lc.ParseOutputType(*argOutputType),
			InvestigationID:   argInvestigationID,
			Tag:               argTag,
			Category:          argCategory,
			IsDeleteOnFailure: &isDeleteOnFailure,
		},
		nil
}

func consumeMessages(fh lc.Firehose) {
	for !fh.IsRunning() {
		if len(fh.Messages) == 0 {
			time.Sleep(1 * time.Second)
		} else {
			message := <-fh.Messages
			log.Info().Msg(message.Content)
		}
	}
}

func consumeDroppedMessages(fh lc.Firehose) {
	for !fh.IsRunning() {
		if len(fh.Messages) == 0 {
			time.Sleep(1 * time.Second)
		} else {
			errorMessage := <-fh.ErrorMessages
			log.Error().Msg(fmt.Sprintf("Error processing: '%s'", errorMessage))
		}
	}
}

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	interruptChannel := make(chan os.Signal, 1)
	signal.Notify(interruptChannel, os.Interrupt)

	clientOpts, fhOpts, fhOutputOpts, err := parseArgs()
	if err != nil {
		log.Err(err).Msg("error parsing arguments")
		return
	}

	fmt.Println("Enter secret API key: ")
	bytesAPIKey, err := terminal.ReadPassword(int(syscall.Stdin))
	if err != nil {
		log.Err(err).Msg("could not read API key")
		return
	}
	if len(bytesAPIKey) == 0 {
		log.Error().Msg("api key is empty")
		return
	}
	clientOpts.APIKey = string(bytesAPIKey)

	org, err := lc.MakeOrganization(clientOpts)
	if err != nil {
		log.Err(err).Msg("could not make organization")
		return
	}

	fh, err := lc.MakeFirehose(org, fhOpts, &fhOutputOpts)
	if err != nil {
		log.Err(err).Msg("could not make firehose")
	}

	if err := fh.Start(); err != nil {
		log.Err(err).Msg("could not start firehose")
		return
	}

	go consumeMessages(fh)
	go consumeDroppedMessages(fh)

	<-interruptChannel
	log.Info().Msg("you pressed CTRL+C, shutting down...")
	fh.Shutdown()
	log.Info().Msg("exiting.")
}
