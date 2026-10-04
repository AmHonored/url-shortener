package shortener

import (
	"context"
	"errors"
	"testing"
)

// fakeStore is a minimal map-based Store used to test the service in
// isolation. Setting err makes every Create fail with it.
type fakeStore struct {
	byCode  map[string]Link
	byURL   map[string]string
	creates int
	err     error
}

func newFakeStore() *fakeStore {
	return &fakeStore{byCode: map[string]Link{}, byURL: map[string]string{}}
}

func (f *fakeStore) Create(_ context.Context, l Link) (Link, error) {
	f.creates++
	if f.err != nil {
		return Link{}, f.err
	}
	if code, ok := f.byURL[l.URL]; ok {
		return f.byCode[code], nil
	}
	if _, ok := f.byCode[l.Code]; ok {
		return Link{}, ErrCodeExists
	}
	f.byCode[l.Code], f.byURL[l.URL] = l, l.Code
	return l, nil
}

func (f *fakeStore) Get(_ context.Context, code string) (Link, error) {
	l, ok := f.byCode[code]
	if !ok {
		return Link{}, ErrNotFound
	}
	return l, nil
}

// codes returns a generator that yields the given codes in order.
func codes(list ...string) func() (string, error) {
	i := 0
	return func() (string, error) {
		c := list[i%len(list)]
		i++
		return c, nil
	}
}

func TestShortenIdempotent(t *testing.T) {
	svc := NewService(newFakeStore())
	ctx := context.Background()

	first, err := svc.Shorten(ctx, "https://go.dev/doc/")
	if err != nil {
		t.Fatalf("Shorten: %v", err)
	}
	// Equivalent spelling must map to the same code.
	second, err := svc.Shorten(ctx, "HTTPS://GO.DEV:443/doc/#intro")
	if err != nil {
		t.Fatalf("Shorten: %v", err)
	}
	if first.Code != second.Code {
		t.Errorf("same URL got different codes: %q vs %q", first.Code, second.Code)
	}
	if first.URL != "https://go.dev/doc/" {
		t.Errorf("stored URL = %q, want normalized form", first.URL)
	}
}

func TestShortenRetriesOnCollision(t *testing.T) {
	svc := NewService(newFakeStore())
	svc.newCode = codes("sgAAAAAA", "sgAAAAAA", "sgBBBBBB")
	ctx := context.Background()

	a, _ := svc.Shorten(ctx, "https://a.com/")
	b, err := svc.Shorten(ctx, "https://b.com/")
	if err != nil {
		t.Fatalf("Shorten: %v", err)
	}
	if a.Code != "sgAAAAAA" || b.Code != "sgBBBBBB" {
		t.Errorf("codes = %q, %q; want sgAAAAAA, sgBBBBBB", a.Code, b.Code)
	}
}

func TestShortenGivesUpAfterMaxAttempts(t *testing.T) {
	store := newFakeStore()
	svc := NewService(store)
	svc.newCode = codes("sgAAAAAA") // always the same code
	ctx := context.Background()

	_, _ = svc.Shorten(ctx, "https://a.com/")
	store.creates = 0
	_, err := svc.Shorten(ctx, "https://b.com/")
	if !errors.Is(err, ErrCodeExists) {
		t.Fatalf("err = %v, want ErrCodeExists", err)
	}
	if store.creates != maxCodeAttempts {
		t.Errorf("Create called %d times, want %d", store.creates, maxCodeAttempts)
	}
}

func TestShortenInvalidURLDoesNotTouchStore(t *testing.T) {
	store := newFakeStore()
	_, err := NewService(store).Shorten(context.Background(), "ftp://x")
	if !errors.Is(err, ErrInvalidURL) {
		t.Fatalf("err = %v, want ErrInvalidURL", err)
	}
	if store.creates != 0 {
		t.Errorf("store was called %d times for an invalid URL", store.creates)
	}
}

func TestShortenPropagatesErrors(t *testing.T) {
	boom := errors.New("boom")

	t.Run("store error", func(t *testing.T) {
		store := newFakeStore()
		store.err = boom
		_, err := NewService(store).Shorten(context.Background(), "https://a.com/")
		if !errors.Is(err, boom) {
			t.Fatalf("err = %v, want wrapped boom", err)
		}
	})

	t.Run("generator error", func(t *testing.T) {
		svc := NewService(newFakeStore())
		svc.newCode = func() (string, error) { return "", boom }
		_, err := svc.Shorten(context.Background(), "https://a.com/")
		if !errors.Is(err, boom) {
			t.Fatalf("err = %v, want boom", err)
		}
	})
}

func TestResolve(t *testing.T) {
	svc := NewService(newFakeStore())
	ctx := context.Background()

	created, _ := svc.Shorten(ctx, "https://go.dev/")
	got, err := svc.Resolve(ctx, created.Code)
	if err != nil || got.URL != "https://go.dev/" {
		t.Fatalf("Resolve(%q) = %+v, %v", created.Code, got, err)
	}
	if _, err := svc.Resolve(ctx, "sgZZZZZZ"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Resolve unknown: err = %v, want ErrNotFound", err)
	}
}
