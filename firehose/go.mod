module github.com/refractionPOINT/go-limacharlie/firehose

go 1.15

require (
	github.com/akamensky/argparse v1.4.0
	github.com/google/uuid v1.4.0
	github.com/refractionPOINT/go-limacharlie/limacharlie v0.0.0
	github.com/rs/zerolog v1.31.0
	golang.org/x/crypto v0.17.0
)

replace github.com/refractionPOINT/go-limacharlie/limacharlie => ../limacharlie
