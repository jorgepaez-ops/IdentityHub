package password

import (
	"encoding/base64"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/jorgepaez/identity-hub/internal/config"
)

func TestRF001_ContrasenaSeGuardaConArgon2id(t *testing.T) {
	restore := Configure(config.PasswordConfig{MemoryKiB: 65536, Iterations: 3, Parallelism: 2, Concurrency: 4})
	defer restore()

	first, err := Hash("correct horse battery staple")
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}
	second, err := Hash("correct horse battery staple")
	if err != nil {
		t.Fatalf("Hash() second error = %v", err)
	}
	if !strings.HasPrefix(first, "$argon2id$v=19$m=65536,t=3,p=2$") {
		t.Fatalf("Hash() = %q, want PHC Argon2id parameters", first)
	}
	if first == second {
		t.Fatal("Hash() returned the same hash for equal passwords")
	}
}

func TestRF001_NeedsRehashParaParametrosAntiguos(t *testing.T) {
	restore := Configure(config.PasswordConfig{MemoryKiB: 65536, Iterations: 3, Parallelism: 2, Concurrency: 4})
	defer restore()

	current, err := Hash("password-for-rehash")
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}
	if NeedsRehash(current) {
		t.Fatal("NeedsRehash(current) = true, want false")
	}
	old := "$argon2id$v=19$m=32768,t=2,p=1$" + base64.RawStdEncoding.EncodeToString(make([]byte, 16)) + "$" + base64.RawStdEncoding.EncodeToString(make([]byte, 32))
	if !NeedsRehash(old) {
		t.Fatal("NeedsRehash(old) = false, want true")
	}
}

func TestRF001_EntradaInvalida(t *testing.T) {
	for _, password := range []string{"", strings.Repeat("a", 129)} {
		_, err := Hash(password)
		var invalid *InvalidPasswordError
		if !errors.As(err, &invalid) {
			t.Fatalf("Hash(%d chars) error = %v, want InvalidPasswordError", len(password), err)
		}
	}
}

func TestRF001_VerificacionYHashMalformado(t *testing.T) {
	restore := Configure(config.PasswordConfig{MemoryKiB: 1024, Iterations: 1, Parallelism: 1, Concurrency: 2})
	defer restore()

	hash, err := Hash("right-password")
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}
	ok, err := Verify("wrong-password", hash)
	if err != nil || ok {
		t.Fatalf("Verify(wrong) = (%v, %v), want (false, nil)", ok, err)
	}
	if _, err := Verify("right-password", "not-a-phc-hash"); err == nil {
		t.Fatal("Verify(malformed) error = nil, want non-nil")
	}
}

func TestRF001_HashSenueloYSemaforo(t *testing.T) {
	restore := Configure(config.PasswordConfig{MemoryKiB: 1024, Iterations: 1, Parallelism: 1, Concurrency: 2})
	defer restore()

	if err := VerifyDecoy("nonexistent-user-password"); err != nil {
		t.Fatalf("VerifyDecoy() error = %v", err)
	}
	hash, err := Hash("concurrent-password")
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 6)
	for range 6 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, verifyErr := Verify("concurrent-password", hash)
			if verifyErr != nil || !ok {
				errs <- verifyErr
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent Verify() failed: %v", err)
	}
}
