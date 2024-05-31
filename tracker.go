/*
 * Copyright 2024 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

// Package tracker provides the caddy adobe_usage_tracker plugin.
package tracker

import (
	"bytes"
	"fmt"
	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"go.uber.org/zap"
	"io"
	"net/http"
	"net/url"
)

func init() {
	caddy.RegisterModule(AdobeUsageTracker{})
	httpcaddyfile.RegisterHandlerDirective("adobe_usage_tracker", parseCaddyfile)
}

// AdobeUsageTracker implements HTTP middleware that parses
// uploaded log files from Adobe desktop applications in order to
// collect measurements about past launches. These measurements
// are then uploaded to an InfluxDB (using the v1 HTTP API).
//
// Configuration of the tracker requires four parameters:
//
// - the endpoint URL of the influx v1 upload api
// - the name of the influx v1 database
// - the retention policy of the influx v1 database
// - an API token authorized for writes of the database
//
// Note: this middleware uses the v1 HTTP write API because it's
// fully supported by both v1 and v3 databases.  When using a
// v3 database, you must specify a "dbrp" mapping from the
// database and policy names to the specific bucket you want
// uploads to go to. See the influx docs for details:
//
// https://docs.influxdata.com/influxdb/cloud-serverless/write-data/api/v1-http/
type AdobeUsageTracker struct {
	Endpoint string `json:"endpoint,omitempty"`
	Database string `json:"database,omitempty"`
	Policy   string `json:"policy,omitempty"`
	Token    string `json:"token,omitempty"`

	ep  string
	db  string
	rp  string
	tok string
}

// CaddyModule returns the Caddy module information.
func (AdobeUsageTracker) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.adobe_usage_tracker",
		New: func() caddy.Module { return new(AdobeUsageTracker) },
	}
}

// Provision implements caddy.Provisioner.
func (m *AdobeUsageTracker) Provision(caddy.Context) error {
	if m.Endpoint == "" {
		return fmt.Errorf("an endpoint URL must be specified")
	}
	u, err := url.Parse(m.Endpoint)
	if err != nil {
		return fmt.Errorf("%q is not a valid endpoint url: %v", m.Endpoint, err)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("endpoint protocol must be https, not '%s'", u.Scheme)
	}
	if u.Hostname() == "" {
		return fmt.Errorf("endpoint %q is missing a hostname", m.Endpoint)
	}
	if u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("endpoint %q cannot have a path, query, or fragment portion", m.Endpoint)
	}
	m.ep = m.Endpoint
	if m.Database == "" {
		return fmt.Errorf("database must be specified")
	}
	m.db = m.Database
	if m.Policy == "" {
		return fmt.Errorf("A retention policy must be specified")
	}
	m.rp = m.Policy
	if m.Token == "" {
		return fmt.Errorf("A token must be specified")
	}
	m.tok = m.Token
	return nil
}

// Validate implements caddy.Validator.
func (m *AdobeUsageTracker) Validate() error {
	if m.ep == "" {
		return fmt.Errorf("endpoint URL must be specified")
	}
	u, err := url.Parse(m.ep)
	if err != nil {
		return fmt.Errorf("%q is not a valid endpoint URL: %v", m.ep, err)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("endpoint protocol must be https, not '%s'", u.Scheme)
	}
	if u.Hostname() == "" {
		return fmt.Errorf("endpoint %q is missing a hostname", m.ep)
	}
	if u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("endpoint %q cannot have a path, query, or fragment portion", m.ep)
	}
	if m.db == "" {
		return fmt.Errorf("database must be specified")
	}
	if m.rp == "" {
		return fmt.Errorf("retention policy must be specified")
	}
	if m.tok == "" {
		return fmt.Errorf("token must be specified")
	}
	return nil
}

// ServeHTTP implements caddyhttp.MiddlewareHandler. It extracts
// measurements from any logs uploaded in the request, sends them
// to the influxDB endpoint, and then passes the request intact
// onto the next handler.
func (m AdobeUsageTracker) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	logger := caddy.Log()
	buf, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	sessions := parseLog(string(buf), r.RemoteAddr)
	logger.Info("AdobeUsageTracker: incoming request summary",
		zap.String("remote-address", r.RemoteAddr),
		zap.Int("content-length", len(buf)),
		zap.Int("session-count", len(sessions)),
	)
	logger.Debug("AdobeUsageTracker: uploading sessions", zap.Objects("sessions", sessions))
	if len(sessions) == 0 {
		logger.Info("AdobeUsageTracker: no sessions to upload")
	} else {
		err = sendSessions(m.ep, m.db, m.rp, m.tok, sessions, logger)
		if err != nil {
			logger.Error("AdobeUsageTracker: failed to send sessions", zap.Error(err))
		} else {
			logger.Info("AdobeUsageTracker: sent sessions successfully")
		}
	}
	r.Body = io.NopCloser(bytes.NewReader(buf))
	return next.ServeHTTP(w, r)
}

// UnmarshalCaddyfile implements caddyfile.Unmarshaler.
func (m *AdobeUsageTracker) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	d.Next() // consume directive name

	for nesting := d.Nesting(); d.NextBlock(nesting); {
		key := d.Val()
		if !d.NextArg() {
			return d.ArgErr()
		}
		switch key {
		case "endpoint":
			m.Endpoint = d.Val()
		case "database":
			m.Database = d.Val()
		case "policy":
			m.Policy = d.Val()
		case "token":
			m.Token = d.Val()
		default:
			return d.ArgErr()
		}
	}
	return nil
}

// parseCaddyfile unmarshals tokens from h into a new AdobeUsageTracker.
func parseCaddyfile(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	var m AdobeUsageTracker
	err := m.UnmarshalCaddyfile(h.Dispenser)
	return m, err
}

// Interface guards
var (
	_ caddy.Provisioner           = (*AdobeUsageTracker)(nil)
	_ caddy.Validator             = (*AdobeUsageTracker)(nil)
	_ caddyhttp.MiddlewareHandler = (*AdobeUsageTracker)(nil)
	_ caddyfile.Unmarshaler       = (*AdobeUsageTracker)(nil)
)
