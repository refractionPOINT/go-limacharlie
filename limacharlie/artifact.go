package limacharlie

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"cloud.google.com/go/storage"
)

type ArtifactRuleName = string
type ArtifactRule struct {
	By          string `json:"by"`
	LastUpdated uint64 `json:"updated"`

	IsIgnoreCert   bool               `json:"is_ignore_cert"`
	IsDeleteAfter  bool               `json:"is_delete_after"`
	DaysRetentions uint               `json:"days_retention"`
	Patterns       []string           `json:"patterns"`
	Filters        ArtifactRuleFilter `json:"filters"`
}

type ArtifactRuleFilter struct {
	Tags      []string `json:"tags"`
	Platforms []string `json:"platforms"`
}
type ArtifactRulesByName = map[ArtifactRuleName]ArtifactRule

type artifactExportResp struct {
	Payload string `json:"payload,omitempty"`
	Export  string `json:"export,omitempty"`
}

const MAX_UPLOAD_PART_SIZE = 1024 * 1024

func (org Organization) artifact(responseData interface{}, action string, req Dict) error {
	reqData := req
	reqData["action"] = action
	return org.client.serviceRequest(responseData, "logging", reqData, false)
}

func (org Organization) ArtifactsRules() (ArtifactRulesByName, error) {
	resp := ArtifactRulesByName{}
	if err := org.artifact(&resp, "list_rules", Dict{}); err != nil {
		return ArtifactRulesByName{}, err
	}
	return resp, nil
}

func (org Organization) ArtifactRuleAdd(ruleName ArtifactRuleName, rule ArtifactRule) error {
	resp := Dict{}
	if err := org.artifact(&resp, "add_rule", Dict{
		"name":            ruleName,
		"patterns":        rule.Patterns,
		"is_delete_after": rule.IsDeleteAfter,
		"is_ignore_cert":  rule.IsIgnoreCert,
		"days_retention":  rule.DaysRetentions,
		"tags":            rule.Filters.Tags,
		"platforms":       rule.Filters.Platforms,
	}); err != nil {
		return err
	}
	return nil
}

func (org Organization) ArtifactRuleDelete(ruleName ArtifactRuleName) error {
	resp := Dict{}
	if err := org.artifact(&resp, "remove_rule", Dict{"name": ruleName}); err != nil {
		return err
	}
	return nil
}

func getContentReader(dataOrFilePath string) (io.Reader, int64, error) {
	// Check if the input is a file path
	if fileInfo, err := os.Stat(dataOrFilePath); err == nil {
		// If it's a file, read its content
		file, err := os.Open(dataOrFilePath)
		if err != nil {
			return nil, 0, err
		}
		return file, fileInfo.Size(), nil
	}

	// If it's not a file path, assume it's a string of data
	data := bytes.NewBufferString(dataOrFilePath)
	return data, int64(data.Len()), nil
}

func (org Organization) CreateArtifact(name string, fileData string, ingestion_key string) error {
	var size int64

	file, size, err := getContentReader(fileData)
	if err != nil {
		fmt.Println("Error getting file contents:", err)
	}

	return org.UploadArtifact(file, size, "txt", name, "", "", 30, ingestion_key)
}

func (org Organization) UploadArtifact(data io.Reader, size int64, hint string, source string, artifactId string, originalPath string, nDaysRetention int, ingestion_key string) error {

	// Assemble headers
	headers := map[string]string{}
	headers["lc-source"] = source
	headers["lc-hint"] = hint
	if artifactId != "" {
		headers["lc-artifact-id"] = artifactId
	} else {
		headers["lc-artifact-id"] = uuid.New().String()
	}
	if originalPath != "" {
		absolutePath, _ := filepath.Abs(originalPath)
		headers["lc-path"] = base64.StdEncoding.EncodeToString([]byte(absolutePath))
	}
	headers["lc-retention-days"] = fmt.Sprintf("%d", nDaysRetention)

	// Get artifacts URL
	urls, err := org.GetURLs()
	if err != nil {
		return fmt.Errorf("failed resolving org URLs: %v", err)
	}
	uploadUrl, ok := urls["artifacts"]
	if !ok {
		return errors.New("artifacts URL not found in org URLs")
	}
	reqUrl := fmt.Sprintf("https://%s/ingest", uploadUrl)

	// Build request
	combined := fmt.Sprintf("%s:%s", org.GetOID(), ingestion_key)
	creds := base64.StdEncoding.EncodeToString([]byte(combined))
	c := &http.Client{}
	defer c.CloseIdleConnections()
	partId := 0
	endOffset := int64(0)

	for {
		// Read from the data in chunks of MAX_UPLOAD_PART_SIZE so we can
		// upload in parts if the file is too big.
		chunk := make([]byte, MAX_UPLOAD_PART_SIZE)
		n, err := data.Read(chunk)
		if err != nil && err != io.EOF {
			return err
		}
		if n == 0 {
			break
		}
		chunk = chunk[:n]

		// If this is the last chunk, set the part to "done".
		endOffset += int64(n)
		if endOffset > size {
			return fmt.Errorf("got more data (%d bytes) than expected (%d bytes)", endOffset, size)
		}
		if endOffset != size {
			headers["lc-part"] = fmt.Sprintf("%d", partId)
		}

		// Prepare the request.
		req, err := http.NewRequest(http.MethodPost, reqUrl, bytes.NewBuffer(chunk))
		if err != nil {
			return err
		}

		req.Header.Set("Content-Type", "application/octet-stream")
		req.Header.Set("Authorization", fmt.Sprintf("Basic %s", creds))
		// Add the dynamic headers.
		for k, v := range headers {
			req.Header.Set(k, v)
		}

		// Send the request.
		httpResp, err := c.Do(req)
		if err != nil {
			return err
		}

		// Check if the API liked it.
		if httpResp.StatusCode != 200 {
			return fmt.Errorf("failed to POST artifact, http status: %d", httpResp.StatusCode)
		}
		partId++
	}
	return nil
}

func (org Organization) ExportArtifact(artifactID string, deadline time.Time) (io.ReadCloser, error) {
	resp := artifactExportResp{}
	var request restRequest
	request = makeDefaultRequest(&resp)
	if err := org.client.reliableRequest(http.MethodGet, fmt.Sprintf("insight/%s/artifacts/originals/%s", org.GetOID(), artifactID), request); err != nil {
		return nil, err
	}
	if resp.Payload != "" {
		b64Dec, err := base64.StdEncoding.DecodeString(resp.Payload)
		if err != nil {
			return nil, err
		}
		gzR, err := gzip.NewReader(bytes.NewBuffer(b64Dec))
		if err != nil {
			return nil, err
		}

		return gzR, nil
	}
	c := http.Client{
		Timeout: 30 * time.Second,
	}
	defer c.CloseIdleConnections()

	var httpResp *http.Response
	for !time.Now().After(deadline) {
		req, err := http.NewRequest(http.MethodGet, resp.Export, &bytes.Buffer{})
		if err != nil {
			return nil, err
		}
		httpResp, err = c.Do(req)
		if err != nil {
			return nil, err
		}
		if httpResp.StatusCode == 200 {
			break
		}
		httpResp.Body.Close()
		if httpResp.StatusCode == 404 {
			time.Sleep(5 * time.Second)
			continue
		}
		// Read all the data from the body in case it includes
		// a relevant error to return.
		body, _ := io.ReadAll(httpResp.Body)
		return nil, fmt.Errorf("unexpected status getting data (%d): %s", httpResp.StatusCode, string(body))
	}

	return httpResp.Body, nil
}

func (org Organization) ExportArtifactThroughGCS(ctx context.Context, artifactID string, deadline time.Time, bucketName string, writeCreds string, readClient storage.Client) (io.ReadCloser, error) {
	resp := artifactExportResp{}
	var request restRequest
	request = makeDefaultRequest(&resp).withQueryData(Dict{
		"dest_bucket": bucketName,
		"svc_creds":   writeCreds,
	})
	if err := org.client.reliableRequest(http.MethodGet, fmt.Sprintf("insight/%s/artifacts/originals/%s", org.GetOID(), artifactID), request); err != nil {
		return nil, err
	}
	if resp.Payload != "" {
		b64Dec, err := base64.StdEncoding.DecodeString(resp.Payload)
		if err != nil {
			return nil, err
		}
		gzR, err := gzip.NewReader(bytes.NewBuffer(b64Dec))
		if err != nil {
			return nil, err
		}

		return gzR, nil
	}

	u, err := url.Parse(resp.Export)
	if err != nil {
		return nil, fmt.Errorf("failed to parse export URL: %v", err)
	}
	bucket := readClient.Bucket(bucketName)
	objPath := strings.SplitN(strings.TrimLeft(u.Path, "/"), "/", 2)[1]

	var r io.ReadCloser
	for !time.Now().After(deadline) {
		r, err = bucket.Object(objPath).NewReader(ctx)
		if err == storage.ErrObjectNotExist {
			time.Sleep(5 * time.Second)
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("failed to get object reader: %v", err)
		}
		break
	}

	return r, nil
}
