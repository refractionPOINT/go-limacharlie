#! /bin/sh

go test -c .

sudo docker build --rm -f Dockerfile .