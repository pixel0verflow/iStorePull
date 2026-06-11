package charles

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pixel0verflow/istorepull/pkg/credential"
)

// harFile is the subset of the HAR schema we read.
type harFile struct {
	Log struct {
		Entries []struct {
			Request struct {
				URL     string `json:"url"`
				Headers []struct {
					Name  string `json:"name"`
					Value string `json:"value"`
				} `json:"headers"`
			} `json:"request"`
		} `json:"entries"`
	} `json:"log"`
}

// charlesJSON is the subset of Charles' own "Export as JSON" schema we read.
type charlesJSON []struct {
	Host    string `json:"host"`
	Path    string `json:"path"`
	Query   string `json:"query"`
	Scheme  string `json:"scheme"`
	Request struct {
		Header struct {
			Headers []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"headers"`
		} `json:"header"`
	} `json:"request"`
}

// ParseHAR builds a session from a HAR or Charles-JSON export file.
func ParseHAR(path string) (credential.Session, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return credential.Session{}, err
	}
	flows, err := flowsFromJSON(data)
	if err != nil {
		return credential.Session{}, err
	}
	f, ok := pickFlow(flows)
	if !ok {
		return credential.Session{}, ErrNoStoreFlow
	}
	return buildSession(f, "har:"+filepath.Base(path))
}

// flowsFromJSON decodes either HAR or Charles-JSON into flows.
func flowsFromJSON(data []byte) ([]flow, error) {
	trimmed := strings.TrimSpace(string(data))
	if strings.HasPrefix(trimmed, "[") {
		return charlesJSONFlows(data)
	}
	return harFlows(data)
}

func harFlows(data []byte) ([]flow, error) {
	var h harFile
	if err := json.Unmarshal(data, &h); err != nil {
		return nil, fmt.Errorf("parse HAR: %w", err)
	}
	flows := make([]flow, 0, len(h.Log.Entries))
	for _, e := range h.Log.Entries {
		f := flow{url: e.Request.URL}
		for _, hd := range e.Request.Headers {
			f.headers = append(f.headers, header{name: hd.Name, value: hd.Value})
		}
		flows = append(flows, f)
	}
	return flows, nil
}

func charlesJSONFlows(data []byte) ([]flow, error) {
	var c charlesJSON
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse Charles JSON: %w", err)
	}
	flows := make([]flow, 0, len(c))
	for _, e := range c {
		scheme := e.Scheme
		if scheme == "" {
			scheme = "https"
		}
		u := scheme + "://" + e.Host + e.Path
		if e.Query != "" {
			u += "?" + e.Query
		}
		f := flow{url: u}
		for _, hd := range e.Request.Header.Headers {
			f.headers = append(f.headers, header{name: hd.Name, value: hd.Value})
		}
		flows = append(flows, f)
	}
	return flows, nil
}
