FROM golang:1.27.1

ENV LC_CURRENT_ENV "test_env"
ENV LC_OID "00000000-0000-4000-8000-000000000001"
ENV LC_UID "00000000-0000-4000-8000-000000000002"
ENV LC_API_KEY "00000000-0000-4000-8000-000000000003"
ENV LC_CREDS_FILE_NO_ENV "tests/test_noenv.yaml"
ENV LC_CREDS_FILE "tests/test.yaml"

ADD limacharlie.test .
ADD tests ./tests

CMD ["./limacharlie.test", "-test.v", "-test.run", "^.*$"]