package capture

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"
)

type fakeSystem struct {
	trustErr error
	proxyErr error

	trusted, removed   int
	setProxy, restored int
}

func (f *fakeSystem) TrustCA([]byte) (func() error, error) {
	if f.trustErr != nil {
		return nil, f.trustErr
	}
	f.trusted++
	return func() error { f.removed++; return nil }, nil
}

func (f *fakeSystem) SetProxy(string, int) (func() error, error) {
	if f.proxyErr != nil {
		return nil, f.proxyErr
	}
	f.setProxy++
	return func() error { f.restored++; return nil }, nil
}

func TestRunTimeoutTearsDown(t *testing.T) {
	sys := &fakeSystem{}
	_, err := Run(context.Background(), sys, Options{
		Addr:    "127.0.0.1:0",
		Timeout: 100 * time.Millisecond,
	}, io.Discard)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if sys.trusted != 1 || sys.setProxy != 1 {
		t.Errorf("setup not run: %+v", sys)
	}
	if sys.removed != 1 || sys.restored != 1 {
		t.Errorf("teardown not run (CA must be removed and proxy restored): %+v", sys)
	}
}

func TestRunTrustFailureAborts(t *testing.T) {
	sys := &fakeSystem{trustErr: errors.New("user denied")}
	_, err := Run(context.Background(), sys, Options{Timeout: time.Second}, io.Discard)
	if err == nil {
		t.Fatal("expected error when CA trust fails")
	}
	if sys.setProxy != 0 {
		t.Error("proxy must not be set when CA trust fails")
	}
}

func TestRunProxyFailureRemovesCA(t *testing.T) {
	sys := &fakeSystem{proxyErr: errors.New("no network service")}
	_, err := Run(context.Background(), sys, Options{Addr: "127.0.0.1:0", Timeout: time.Second}, io.Discard)
	if err == nil {
		t.Fatal("expected error when proxy set fails")
	}
	if sys.trusted != 1 || sys.removed != 1 {
		t.Errorf("CA must be trusted then removed on proxy failure: %+v", sys)
	}
	if sys.restored != 0 {
		t.Errorf("nothing to restore when proxy never set: %+v", sys)
	}
}
