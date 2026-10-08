package integration_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// RawWireMockStub captures the raw JSON structure of a WireMock stub file.
type RawWireMockStub struct {
	Request  json.RawMessage `json:"request"`
	Response struct {
		Status   int             `json:"status"`
		Headers  json.RawMessage `json:"headers"`
		JSONBody json.RawMessage `json:"jsonBody"`
	} `json:"response"`
}

func loadRawStubBody(t *testing.T, filename string) (int, any) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "docker", "wiremock", "mappings", filename))
	require.NoError(t, err, "failed to read stub file: %s", filename)

	var stub RawWireMockStub
	err = json.Unmarshal(data, &stub)
	require.NoError(t, err, "failed to parse stub JSON: %s", filename)

	var body any
	err = json.Unmarshal(stub.Response.JSONBody, &body)
	require.NoError(t, err, "failed to parse jsonBody: %s", filename)

	return stub.Response.Status, body
}

func compileSchema(t *testing.T, schemaRelPath string) *jsonschema.Schema {
	t.Helper()
	absPath, err := filepath.Abs(filepath.Join("..", "schemas", schemaRelPath))
	require.NoError(t, err, "failed to resolve schema absolute path: %s", schemaRelPath)

	compiler := jsonschema.NewCompiler()
	compiler.Draft = jsonschema.Draft7
	schema, err := compiler.Compile(absPath)
	require.NoError(t, err, "failed to compile JSON schema %s", schemaRelPath)
	return schema
}

// TestJiraIssueCompatibilityMatrix validates JIRA issue stubs against all supported JIRA Server version schemas.
func TestJiraIssueCompatibilityMatrix(t *testing.T) {
	jiraVersions := []string{
		"v2-server-7.6",
		"v2-server-8.x",
	}

	stubs := []string{
		"jira_get_issue_vsb_101.json",
		"jira_get_issue_vsb_102.json",
	}

	for _, ver := range jiraVersions {
		schemaPath := filepath.Join("jira", ver, "issue.json")
		schema := compileSchema(t, schemaPath)

		for _, stubFile := range stubs {
			testName := ver + "/" + stubFile
			t.Run(testName, func(t *testing.T) {
				status, body := loadRawStubBody(t, stubFile)
				assert.Equal(t, 200, status)

				err := schema.Validate(body)
				if err != nil {
					t.Fatalf("Validation failed for %s against schema %s:\n%#v", stubFile, schemaPath, err)
				}
			})
		}
	}
}

// TestJiraErrorCollectionCompatibilityMatrix validates JIRA error stubs against supported error schemas.
func TestJiraErrorCollectionCompatibilityMatrix(t *testing.T) {
	jiraVersions := []string{
		"v2-server-7.6",
		"v2-server-8.x",
	}

	errorStubs := []struct {
		stubFile       string
		expectedStatus int
	}{
		{"jira_get_issue_404.json", 404},
		{"jira_get_issue_401.json", 401},
	}

	for _, ver := range jiraVersions {
		schemaPath := filepath.Join("jira", ver, "error_collection.json")
		schema := compileSchema(t, schemaPath)

		for _, tc := range errorStubs {
			testName := ver + "/" + tc.stubFile
			t.Run(testName, func(t *testing.T) {
				status, body := loadRawStubBody(t, tc.stubFile)
				assert.Equal(t, tc.expectedStatus, status)

				err := schema.Validate(body)
				if err != nil {
					t.Fatalf("Validation failed for %s against schema %s:\n%#v", tc.stubFile, schemaPath, err)
				}
			})
		}
	}
}

// TestBitbucketPullRequestCompatibilityMatrix validates Bitbucket PR stubs against supported Bitbucket Server versions.
func TestBitbucketPullRequestCompatibilityMatrix(t *testing.T) {
	bbVersions := []string{
		"v1-server-6.1",
		"v1-server-8.19",
	}

	for _, ver := range bbVersions {
		schemaPath := filepath.Join("bitbucket", ver, "pull_request.json")
		schema := compileSchema(t, schemaPath)

		testName := ver + "/bitbucket_post_pull_request_201.json"
		t.Run(testName, func(t *testing.T) {
			status, body := loadRawStubBody(t, "bitbucket_post_pull_request_201.json")
			assert.Equal(t, 201, status)

			err := schema.Validate(body)
			if err != nil {
				t.Fatalf("Validation failed for 201 PR stub against schema %s:\n%#v", schemaPath, err)
			}
		})
	}
}

// TestBitbucketErrorsCompatibilityMatrix validates Bitbucket error responses against error schemas.
func TestBitbucketErrorsCompatibilityMatrix(t *testing.T) {
	bbVersions := []string{
		"v1-server-6.1",
		"v1-server-8.19",
	}

	for _, ver := range bbVersions {
		schemaPath := filepath.Join("bitbucket", ver, "errors.json")
		schema := compileSchema(t, schemaPath)

		testName := ver + "/bitbucket_post_pull_request_409.json"
		t.Run(testName, func(t *testing.T) {
			status, body := loadRawStubBody(t, "bitbucket_post_pull_request_409.json")
			assert.Equal(t, 409, status)

			err := schema.Validate(body)
			if err != nil {
				t.Fatalf("Validation failed for 409 conflict stub against schema %s:\n%#v", schemaPath, err)
			}
		})
	}
}

// TestSchemaValidationRejection verifies that the validator legitimately catches invalid bodies.
func TestSchemaValidationRejection(t *testing.T) {
	schema := compileSchema(t, filepath.Join("jira", "v2-server-7.6", "issue.json"))

	// Malformed payload missing required 'key' and invalid type for 'id'
	badPayloadJSON := `{"id": 12345, "self": "not-a-uri", "fields": {}}`
	var badBody any
	err := json.Unmarshal([]byte(badPayloadJSON), &badBody)
	require.NoError(t, err)

	err = schema.Validate(badBody)
	assert.Error(t, err, "schema validator MUST reject malformed payload")
	assert.Contains(t, strings.ToLower(err.Error()), "missing", "error message should mention missing properties")
}
