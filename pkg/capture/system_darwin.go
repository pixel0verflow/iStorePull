//go:build darwin

package capture

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// darwinSystem trusts the CA via `security` and toggles the secure-web proxy via
// `networksetup`.
type darwinSystem struct{}

// NewSystem returns the platform System integration.
func NewSystem() System { return darwinSystem{} }

func loginKeychain() (string, error) {
	out, err := exec.Command("security", "default-keychain", "-d", "user").Output()
	if err != nil {
		return "", fmt.Errorf("locate login keychain: %w", err)
	}
	kc := strings.TrimSpace(string(out))
	kc = strings.Trim(kc, "\"")
	return kc, nil
}

func (darwinSystem) TrustCA(pemBytes []byte) (func() error, error) {
	f, err := os.CreateTemp("", "istorepull-ca-*.pem")
	if err != nil {
		return nil, err
	}
	path := f.Name()
	if _, err := f.Write(pemBytes); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return nil, err
	}
	_ = f.Close()

	kc, err := loginKeychain()
	if err != nil {
		_ = os.Remove(path)
		return nil, err
	}

	// Imports the cert and marks it trusted for SSL. Prompts the user for
	// authorization (GUI). User-domain trust, so no sudo.
	cmd := exec.Command("security", "add-trusted-cert", "-r", "trustRoot", "-p", "ssl", "-k", kc, path)
	if out, err := cmd.CombinedOutput(); err != nil {
		_ = os.Remove(path)
		return nil, fmt.Errorf("trust CA: %w: %s", err, strings.TrimSpace(string(out)))
	}

	remove := func() error {
		_ = exec.Command("security", "remove-trusted-cert", path).Run()
		return os.Remove(path)
	}
	return remove, nil
}

// netService is the saved proxy state for one network service.
type netService struct {
	name      string
	wasOn     bool
	oldServer string
	oldPort   string
}

func (darwinSystem) SetProxy(host string, port int) (func() error, error) {
	services, err := activeNetworkServices()
	if err != nil {
		return nil, err
	}
	if len(services) == 0 {
		return nil, fmt.Errorf("no active network services found")
	}

	var changed []netService
	for _, svc := range services {
		prev := readSecureWebProxy(svc)
		if err := runNetworkSetup("-setsecurewebproxy", svc, host, fmt.Sprint(port)); err != nil {
			_ = restoreProxies(changed)
			return nil, fmt.Errorf("set proxy on %q: %w", svc, err)
		}
		changed = append(changed, prev)
	}

	restore := func() error { return restoreProxies(changed) }
	return restore, nil
}

func restoreProxies(saved []netService) error {
	var firstErr error
	for _, s := range saved {
		if s.wasOn && s.oldServer != "" {
			_ = runNetworkSetup("-setsecurewebproxy", s.name, s.oldServer, s.oldPort)
		} else {
			_ = runNetworkSetup("-setsecurewebproxystate", s.name, "off")
		}
	}
	return firstErr
}

func activeNetworkServices() ([]string, error) {
	out, err := exec.Command("networksetup", "-listallnetworkservices").Output()
	if err != nil {
		return nil, fmt.Errorf("list network services: %w", err)
	}
	var services []string
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	first := true
	for sc.Scan() {
		line := sc.Text()
		if first {
			first = false // header line
			continue
		}
		// A leading '*' marks a disabled service.
		if strings.HasPrefix(line, "*") || strings.TrimSpace(line) == "" {
			continue
		}
		services = append(services, line)
	}
	return services, nil
}

func readSecureWebProxy(svc string) netService {
	s := netService{name: svc}
	out, err := exec.Command("networksetup", "-getsecurewebproxy", svc).Output()
	if err != nil {
		return s
	}
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	for sc.Scan() {
		key, val, ok := strings.Cut(sc.Text(), ":")
		if !ok {
			continue
		}
		val = strings.TrimSpace(val)
		switch strings.TrimSpace(key) {
		case "Enabled":
			s.wasOn = strings.EqualFold(val, "Yes")
		case "Server":
			s.oldServer = val
		case "Port":
			s.oldPort = val
		}
	}
	return s
}

func runNetworkSetup(args ...string) error {
	cmd := exec.Command("networksetup", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
