FROM golang:1.15

ENV LC_CURRENT_ENV "test_env"
ENV LC_OID "fba6e992-ce4f-4d9e-99dc-b548f00df7f9"
ENV LC_UID "af4ddec0-c2e8-4db2-ba3f-f5e9a1aff3fd"
ENV LC_API_KEY "843e80c8-e273-4b3e-93bd-41151b4b933a"
ENV LC_CREDS_FILE_NO_ENV "tests/test_noenv.yaml"
ENV LC_CREDS_FILE "tests/test.yaml"

ADD limacharlie.test .
ADD tests ./tests

RUN ./limacharlie.test -test.v -test.run ^TestClientSuite$

