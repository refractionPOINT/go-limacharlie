module github.com/refractionPOINT/go-limacharlie/firehose

go 1.15

require (
	github.com/akamensky/argparse v1.2.2
	github.com/google/uuid v1.1.2
	github.com/refractionPOINT/go-limacharlie/limacharlie v0.0.0
	github.com/rs/zerolog v1.20.0
	golang.org/x/crypto v0.0.0-20190308221718-c2843e01d9a2
)

replace github.com/refractionPOINT/go-limacharlie/limacharlie => ../limacharlie
