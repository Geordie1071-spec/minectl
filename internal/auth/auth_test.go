package auth

import (
	"strings"
	"testing"
	"time"
)

func TestPasswordHash(t *testing.T) {
	h, err := HashPassword("correct horse")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(h, "$argon2id$v=19$m=19456,t=2,p=1$") {
		t.Fatalf("unexpected encoding %q", h)
	}
	if !CheckPassword("correct horse", h) {
		t.Fatal("correct password rejected")
	}
	if CheckPassword("wrong horse", h) {
		t.Fatal("wrong password accepted")
	}
	h2, _ := HashPassword("correct horse")
	if h == h2 {
		t.Fatal("hashes should be salted")
	}
	for _, bad := range []string{"", "plain", "$argon2id$v=19$m=1,t=1,p=1$!!$!!", "$bcrypt$x$y$z$w"} {
		if CheckPassword("x", bad) {
			t.Fatalf("accepted malformed hash %q", bad)
		}
	}
}

func TestValidate(t *testing.T) {
	for _, u := range []string{"ab", "steve", "Alex_01", "a.b-c"} {
		if ValidateUsername(u) != nil {
			t.Errorf("%q should be valid", u)
		}
	}
	for _, u := range []string{"", "a", "-x", "has space", "<script>", strings.Repeat("a", 33)} {
		if ValidateUsername(u) == nil {
			t.Errorf("%q should be invalid", u)
		}
	}
	if ValidatePassword("short") == nil || ValidatePassword("longenough") != nil {
		t.Fatal("password length check wrong")
	}
}

func TestTokens(t *testing.T) {
	a, b := RandomToken(), RandomToken()
	if a == b || len(a) < 40 {
		t.Fatalf("weak tokens %q %q", a, b)
	}
	if !strings.HasPrefix(NewAPIToken(), APITokenPrefix) {
		t.Fatal("api token prefix missing")
	}
	if HashToken(a) != HashToken(a) || HashToken(a) == HashToken(b) {
		t.Fatal("hash not deterministic or collides")
	}
}

func TestLimiter(t *testing.T) {
	now := time.Unix(1000, 0)
	l := NewLimiter(3, time.Minute)
	l.now = func() time.Time { return now }
	for i := 0; i < 3; i++ {
		if b, _ := l.Blocked("ip"); b {
			t.Fatalf("blocked after %d failures", i)
		}
		l.Fail("ip")
	}
	if b, wait := l.Blocked("ip"); !b || wait != time.Minute {
		t.Fatalf("want blocked for 1m, got %v %v", b, wait)
	}
	if b, _ := l.Blocked("other"); b {
		t.Fatal("other key blocked")
	}
	now = now.Add(61 * time.Second)
	if b, _ := l.Blocked("ip"); b {
		t.Fatal("still blocked after window")
	}
	l.Fail("ip")
	l.Reset("ip")
	if len(l.fails) != 0 {
		t.Fatal("reset/prune left entries")
	}
}
