package turso

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/tursodatabase/turso-cli/internal"
)

type Database struct {
	ID            string `json:"dbId" mapstructure:"dbId"`
	Name          string
	Regions       []string
	PrimaryRegion string
	Hostname      string
	Version       string
	Group         string
	Sleeping      bool
}

type DatabasesClient client

func (d *DatabasesClient) List() ([]Database, error) {
	r, err := d.client.Get(d.URL(""), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get database listing: %s", err)
	}
	defer r.Body.Close()

	org := d.client.Org
	if isNotMemberErr(r.StatusCode, org) {
		return nil, notMemberErr(org)
	}

	if r.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get database listing: %w", parseResponseError(r))
	}

	type ListResponse struct {
		Databases []Database `json:"databases"`
	}
	resp, err := unmarshal[ListResponse](r)
	return resp.Databases, err
}

func (d *DatabasesClient) Delete(database string) error {
	url := d.URL("/" + database)
	r, err := d.client.Delete(url, nil)
	if err != nil {
		return fmt.Errorf("failed to delete database: %s", err)
	}
	defer r.Body.Close()

	org := d.client.Org
	if isNotMemberErr(r.StatusCode, org) {
		return notMemberErr(org)
	}

	if r.StatusCode == http.StatusNotFound {
		return fmt.Errorf("database %s not found. List known databases using %s", internal.Emph(database), internal.Emph("turso db list"))
	}

	if r.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to delete database: %w", parseResponseError(r))
	}

	return nil
}

type CreateDatabaseResponse struct {
	Database Database
	Username string
}

type DBSeed struct {
	Type      string     `json:"type"`
	Name      string     `json:"value,omitempty"`
	URL       string     `json:"url,omitempty"`
	Timestamp *time.Time `json:"timestamp,omitempty"`
}

type CreateDatabaseBody struct {
	Name       string  `json:"name"`
	Location   string  `json:"location"`
	Image      string  `json:"image,omitempty"`
	Extensions string  `json:"extensions,omitempty"`
	Group      string  `json:"group,omitempty"`
	Seed       *DBSeed `json:"seed,omitempty"`
	Schema     string  `json:"schema,omitempty"`
	IsSchema   bool    `json:"is_schema,omitempty"`
}

func (d *DatabasesClient) Create(name, location, image, extensions, group string, schema string, isSchema bool, seed *DBSeed) (*CreateDatabaseResponse, error) {
	params := CreateDatabaseBody{name, location, image, extensions, group, seed, schema, isSchema}

	body, err := marshal(params)
	if err != nil {
		return nil, fmt.Errorf("could not serialize request body: %w", err)
	}

	res, err := d.client.Post(d.URL(""), body)
	if err != nil {
		return nil, fmt.Errorf("failed to create database: %s", err)
	}
	defer res.Body.Close()

	org := d.client.Org
	if isNotMemberErr(res.StatusCode, org) {
		return nil, notMemberErr(org)
	}

	if res.StatusCode == http.StatusUnprocessableEntity {
		return nil, fmt.Errorf("database name '%s' is not available", name)
	}

	if res.StatusCode != http.StatusOK {
		return nil, parseResponseError(res)
	}

	data, err := unmarshal[*CreateDatabaseResponse](res)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize response: %w", err)
	}

	return data, nil
}

func (d *DatabasesClient) Seed(name string, dbFile *os.File) error {
	url := d.URL(fmt.Sprintf("/%s/seed", name))
	res, err := d.client.Upload(url, dbFile)
	if err != nil {
		return fmt.Errorf("failed to create database: %w", err)
	}
	defer res.Body.Close()

	org := d.client.Org
	if isNotMemberErr(res.StatusCode, org) {
		return notMemberErr(org)
	}

	if res.StatusCode == http.StatusUnprocessableEntity {
		return fmt.Errorf("database name '%s' is not available", name)
	}

	if res.StatusCode != http.StatusOK {
		return parseResponseError(res)
	}
	return nil
}

func (d *DatabasesClient) UploadDump(dbFile *os.File) (string, error) {
	url := d.URL("/dumps")
	res, err := d.client.Upload(url, dbFile)
	if err != nil {
		return "", fmt.Errorf("failed to upload the dump file: %w", err)
	}
	defer res.Body.Close()

	org := d.client.Org
	if isNotMemberErr(res.StatusCode, org) {
		return "", notMemberErr(org)
	}
	if res.StatusCode != http.StatusOK {
		return "", parseResponseError(res)
	}
	type response struct {
		DumpURL string `json:"dump_url"`
	}
	data, err := unmarshal[response](res)
	if err != nil {
		return "", err
	}
	return data.DumpURL, nil
}

type DatabaseTokenRequest struct {
	Permissions *PermissionsClaim `json:"permissions,omitempty"`
}

func (d *DatabasesClient) Token(database string, expiration string, readOnly bool, permissions *PermissionsClaim) (string, error) {
	authorization := ""
	if readOnly {
		authorization = "&authorization=read-only"
	}
	url := d.URL(fmt.Sprintf("/%s/auth/tokens?expiration=%s%s", database, expiration, authorization))

	req := DatabaseTokenRequest{permissions}
	body, err := marshal(req)
	if err != nil {
		return "", fmt.Errorf("could not serialize request body: %w", err)
	}

	r, err := d.client.Post(url, body)
	if err != nil {
		return "", fmt.Errorf("failed to get database token: %w", err)
	}
	defer r.Body.Close()

	org := d.client.Org
	if isNotMemberErr(r.StatusCode, org) {
		return "", notMemberErr(org)
	}

	if r.StatusCode != http.StatusOK {
		err, _ := unmarshal[string](r)
		return "", fmt.Errorf("failed to get database token: %d %s", r.StatusCode, err)
	}

	type JwtResponse struct{ Jwt string }
	data, err := unmarshal[JwtResponse](r)
	if err != nil {
		return "", err
	}
	return data.Jwt, nil
}

func (d *DatabasesClient) Rotate(database string) error {
	url := d.URL(fmt.Sprintf("/%s/auth/rotate", database))
	r, err := d.client.Post(url, nil)
	if err != nil {
		return fmt.Errorf("failed to rotate database keys: %w", err)
	}
	defer r.Body.Close()

	org := d.client.Org
	if isNotMemberErr(r.StatusCode, org) {
		return notMemberErr(org)
	}

	if r.StatusCode != http.StatusOK {
		err, _ := unmarshal[string](r)
		return fmt.Errorf("failed to rotate database keys: %d %s", r.StatusCode, err)
	}

	return nil
}

func (d *DatabasesClient) Update(database string, group bool) error {
	url := d.URL(fmt.Sprintf("/%s/update", database))
	if group {
		url += "?group=true"
	}
	r, err := d.client.Post(url, nil)
	if err != nil {
		return fmt.Errorf("failed to update database: %w", err)
	}
	defer r.Body.Close()

	org := d.client.Org
	if isNotMemberErr(r.StatusCode, org) {
		return notMemberErr(org)
	}

	if r.StatusCode != http.StatusOK {
		err, _ := unmarshal[string](r)
		return fmt.Errorf("failed to update database: %d %s", r.StatusCode, err)
	}

	return nil
}

type Stats struct {
	TopQueries []struct {
		Query       string `json:"query"`
		RowsRead    int    `json:"rows_read"`
		RowsWritten int    `json:"rows_written"`
	} `json:"top_queries,omitempty"`
}

func (d *DatabasesClient) Stats(database string) (Stats, error) {
	url := d.URL(fmt.Sprintf("/%s/stats", database))
	r, err := d.client.Get(url, nil)
	if err != nil {
		return Stats{}, fmt.Errorf("failed to update database: %w", err)
	}
	defer r.Body.Close()

	org := d.client.Org
	if isNotMemberErr(r.StatusCode, org) {
		return Stats{}, notMemberErr(org)
	}

	if r.StatusCode != http.StatusOK {
		err = parseResponseError(r)
		return Stats{}, fmt.Errorf("failed to get stats for database: %d %s", r.StatusCode, err)
	}

	return unmarshal[Stats](r)
}

type Body struct {
	Org string `json:"org"`
}

func (d *DatabasesClient) Transfer(database, org string) error {
	url := d.URL(fmt.Sprintf("/%s/transfer", database))
	body, err := json.Marshal(Body{Org: org})
	bodyReader := bytes.NewReader(body)
	if err != nil {
		return fmt.Errorf("could not serialize request body: %w", err)
	}
	r, err := d.client.Post(url, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to transfer database")
	}
	defer r.Body.Close()

	if r.StatusCode != http.StatusOK {
		err := parseResponseError(r)
		return fmt.Errorf("failed to transfer %s database to org %s: %w", database, org, err)
	}

	return nil
}

func (d *DatabasesClient) Wakeup(database string) error {
	url := d.URL(fmt.Sprintf("/%s/wakeup", database))
	r, err := d.client.Post(url, nil)
	if err != nil {
		return fmt.Errorf("failed to wakeup database: %w", err)
	}
	defer r.Body.Close()

	org := d.client.Org
	if isNotMemberErr(r.StatusCode, org) {
		return notMemberErr(org)
	}

	if r.StatusCode != http.StatusOK {

		err, _ := unmarshal[string](r)
		return fmt.Errorf("failed to wakeup database: %d %s", r.StatusCode, err)
	}

	return nil
}

type Usage struct {
	RowsRead         uint64 `json:"rows_read,omitempty"`
	RowsWritten      uint64 `json:"rows_written,omitempty"`
	StorageBytesUsed uint64 `json:"storage_bytes,omitempty"`
}

type InstanceUsage struct {
	UUID  string `json:"uuid,omitempty"`
	Usage Usage  `json:"usage"`
}

type DbUsage struct {
	UUID      string          `json:"uuid,omitempty"`
	Instances []InstanceUsage `json:"instances"`
	Usage     Usage           `json:"usage"`
}

type DbUsageResponse struct {
	DbUsage DbUsage `json:"database"`
}

func (d *DatabasesClient) Usage(database string) (DbUsage, error) {
	url := d.URL(fmt.Sprintf("/%s/usage", database))

	r, err := d.client.Get(url, nil)
	if err != nil {
		return DbUsage{}, fmt.Errorf("failed to get database usage: %w", err)
	}
	defer r.Body.Close()

	if r.StatusCode != http.StatusOK {
		err, _ := unmarshal[string](r)
		return DbUsage{}, fmt.Errorf("failed to get database usage: %d %s", r.StatusCode, err)
	}

	body, err := unmarshal[DbUsageResponse](r)
	if err != nil {
		return DbUsage{}, err
	}
	return body.DbUsage, nil
}

func (d *DatabasesClient) URL(suffix string) string {
	prefix := "/v1"
	if d.client.Org != "" {
		prefix = "/v1/organizations/" + d.client.Org
	}
	return prefix + "/databases" + suffix
}

type DatabaseConfig struct {
	AllowAttach bool `json:"allow_attach"`
}

func (d *DatabasesClient) GetConfig(database string) (DatabaseConfig, error) {
	url := d.URL(fmt.Sprintf("/%s/configuration", database))
	r, err := d.client.Get(url, nil)
	if err != nil {
		return DatabaseConfig{}, fmt.Errorf("failed to get database: %w", err)
	}
	defer r.Body.Close()

	org := d.client.Org
	if isNotMemberErr(r.StatusCode, org) {
		return DatabaseConfig{}, notMemberErr(org)
	}

	if r.StatusCode != http.StatusOK {
		err = parseResponseError(r)
		return DatabaseConfig{}, fmt.Errorf("failed to get config for database: %d %s", r.StatusCode, err)
	}

	return unmarshal[DatabaseConfig](r)
}

func (d *DatabasesClient) UpdateConfig(database string, config DatabaseConfig) error {
	url := d.URL(fmt.Sprintf("/%s/configuration", database))
	body, err := marshal(config)
	if err != nil {
		return fmt.Errorf("could not serialize request body: %w", err)
	}
	r, err := d.client.Patch(url, body)
	if err != nil {
		return fmt.Errorf("failed to update database: %w", err)
	}
	defer r.Body.Close()

	org := d.client.Org
	if isNotMemberErr(r.StatusCode, org) {
		return notMemberErr(org)
	}

	if r.StatusCode != http.StatusOK {
		err = parseResponseError(r)
		return fmt.Errorf("failed to update config for database: %d %s", r.StatusCode, err)
	}

	return nil
}
