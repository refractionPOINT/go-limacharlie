#! /bin/sh

go test -c .

sudo docker build -t limacharlie_tests -f Dockerfile .
sudo docker run -e _OID=$LC_TEST_OID -e _KEY=$LC_TEST_KEY limacharlie_tests:latest