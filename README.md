# go-limacharlie
API/SKI for LimaCharlie

## Running the tests
The tests runs in a docker container.

Some tests will need access to a valid *OID* and a valid *API key* that has basic permissions (org.get). The run_test.sh script will get the OID and api key from the LC_TEST_OID and LC_TEST_KEY environment variables.