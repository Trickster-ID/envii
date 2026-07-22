package crypto

import (
	"errors"
	"io"
	"strings"
	"testing"

	"filippo.io/age"
)

func TestEncryptDecryptTable(t *testing.T) {
	SetWorkFactor(10)
	t.Cleanup(func() { SetWorkFactor(10) })

	tests := []struct {
		name       string
		plain      []byte
		pass       string
		decPass    string
		wantEncErr string
		wantDecErr string
		garbage    bool
	}{
		{name: "round trip empty plain", plain: []byte{}, pass: "p"},
		{name: "round trip binary", plain: []byte{0, 1, 2, 255}, pass: "binary-pass"},
		{name: "round trip large", plain: make([]byte, 4096), pass: "large"},
		{name: "encrypt empty passphrase", plain: []byte("x"), pass: "", wantEncErr: "create recipient"},
		{name: "decrypt empty passphrase", plain: []byte("x"), pass: "ok", decPass: "", wantDecErr: "create identity"},
		{name: "decrypt wrong pass", plain: []byte("secret"), pass: "right", decPass: "wrong", wantDecErr: "init decrypt"},
		{name: "decrypt garbage", garbage: true, decPass: "ok", wantDecErr: "init decrypt"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.garbage {
				_, err := Decrypt([]byte("not-an-age-file"), tt.decPass)
				if err == nil || !strings.Contains(err.Error(), tt.wantDecErr) {
					t.Fatalf("err=%v want %q", err, tt.wantDecErr)
				}
				return
			}

			ct, err := Encrypt(tt.plain, tt.pass)
			if tt.wantEncErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantEncErr) {
					t.Fatalf("enc err=%v want %q", err, tt.wantEncErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("encrypt: %v", err)
			}

			decPass := tt.decPass
			if decPass == "" && tt.wantDecErr == "" {
				decPass = tt.pass
			}
			pt, err := Decrypt(ct, decPass)
			if tt.wantDecErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantDecErr) {
					t.Fatalf("dec err=%v want %q", err, tt.wantDecErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("decrypt: %v", err)
			}
			if string(pt) != string(tt.plain) {
				t.Fatalf("plain mismatch got %q want %q", pt, tt.plain)
			}
		})
	}
}

func TestSetWorkFactor(t *testing.T) {
	old := workFactor
	t.Cleanup(func() { workFactor = old })
	SetWorkFactor(12)
	if workFactor != 12 {
		t.Fatalf("got %d", workFactor)
	}
}

func TestEncryptErrorSeams(t *testing.T) {
	SetWorkFactor(10)
	t.Cleanup(func() {
		SetWorkFactor(10)
		ageEncrypt = age.Encrypt
	})

	tests := []struct {
		name    string
		hook    func()
		wantErr string
	}{
		{
			name: "init encrypt",
			hook: func() {
				ageEncrypt = func(dst io.Writer, r ...age.Recipient) (io.WriteCloser, error) {
					return nil, errors.New("encrypt init boom")
				}
			},
			wantErr: "init encrypt",
		},
		{
			name: "write plaintext",
			hook: func() {
				ageEncrypt = func(dst io.Writer, r ...age.Recipient) (io.WriteCloser, error) {
					return errWC{werr: errors.New("write boom")}, nil
				}
			},
			wantErr: "write plaintext",
		},
		{
			name: "close writer",
			hook: func() {
				ageEncrypt = func(dst io.Writer, r ...age.Recipient) (io.WriteCloser, error) {
					return errWC{cerr: errors.New("close boom")}, nil
				}
			},
			wantErr: "close writer",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ageEncrypt = age.Encrypt
			tt.hook()
			_, err := Encrypt([]byte("x"), "pass")
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err=%v want %q", err, tt.wantErr)
			}
		})
	}
}

func TestDecryptReadError(t *testing.T) {
	t.Cleanup(func() { ageDecrypt = age.Decrypt })
	ageDecrypt = func(r io.Reader, ids ...age.Identity) (io.Reader, error) {
		return errReader{err: errors.New("read boom")}, nil
	}
	_, err := Decrypt([]byte("ignored"), "pass")
	if err == nil || !strings.Contains(err.Error(), "read plaintext") {
		t.Fatalf("err=%v", err)
	}
}

type errWC struct {
	werr, cerr error
}

func (e errWC) Write(p []byte) (int, error) {
	if e.werr != nil {
		return 0, e.werr
	}
	return len(p), nil
}
func (e errWC) Close() error { return e.cerr }

type errReader struct{ err error }

func (e errReader) Read([]byte) (int, error) { return 0, e.err }
