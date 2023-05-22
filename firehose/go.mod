module github.com/refractionPOINT/go-limacharlie/firehose

go 1.15

require (
	github.com/akamensky/argparse v1.4.0
	github.com/google/uuid v1.3.0
	github.com/mattn/go-isatty v0.0.19 // indirect
	github.com/refractionPOINT/go-limacharlie/limacharlie v0.0.0
	github.com/rs/zerolog v1.29.1
	github.com/stretchr/testify v1.8.3 // indirect
	golang.org/x/crypto v0.9.0
)

replace github.com/refractionPOINT/go-limacharlie/limacharlie => ../limacharlie
