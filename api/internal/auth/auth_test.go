package auth

import (
	"strings"
	"testing"
	"time"
)

func TestHashAndVerify(t *testing.T) {
	hash, err := HashPassword("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$m=65536,t=3,p=2$") {
		t.Fatalf("unexpected format %q", hash)
	}
	if !VerifyPassword(hash, "correct horse battery") {
		t.Fatal("correct password rejected")
	}
	if VerifyPassword(hash, "wrong") {
		t.Fatal("wrong password accepted")
	}
	if VerifyPassword("not-a-hash", "x") || VerifyPassword("", "") {
		t.Fatal("garbage hash accepted")
	}
	if _, err := HashPassword("short"); err == nil {
		t.Fatal("short password accepted")
	}
	h2, _ := HashPassword("correct horse battery")
	if h2 == hash {
		t.Fatal("salt is not random")
	}
}

func TestLoginLimiter(t *testing.T) {
	now := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	l := NewLoginLimiter(5, 15*time.Minute)
	l.now = func() time.Time { return now }
	for i := 0; i < 4; i++ {
		l.Fail("k")
		if locked, _ := l.Locked("k"); locked {
			t.Fatalf("locked after %d failures", i+1)
		}
	}
	l.Fail("k")
	locked, remaining := l.Locked("k")
	if !locked || remaining <= 14*time.Minute {
		t.Fatalf("want lock of ~15m, got %v %v", locked, remaining)
	}
	now = now.Add(16 * time.Minute)
	if locked, _ := l.Locked("k"); locked {
		t.Fatal("lock did not expire")
	}
	l.Reset("k")
	if locked, _ := l.Locked("other"); locked {
		t.Fatal("unrelated key locked")
	}
}

func TestHashIP(t *testing.T) {
	a := HashIP("secret", "203.0.113.1")
	b := HashIP("secret", "203.0.113.2")
	if a == b || len(a) != 32 || a == "203.0.113.1" {
		t.Fatalf("bad hashes %q %q", a, b)
	}
	if HashIP("other", "203.0.113.1") == a {
		t.Fatal("hash does not depend on the secret")
	}
}
