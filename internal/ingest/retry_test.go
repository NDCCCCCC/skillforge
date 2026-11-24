package ingest

import (
"context"
"errors"
"testing"

"github.com/skillforge/skillforge/internal/document"
"github.com/stretchr/testify/assert"
"github.com/stretchr/testify/require"
)

// ---- P3-g: retry loop attempt counts (Runner injected with 0 backoff) ----

func TestRetryLoopAttempts(t *testing.T) {
r := &Runner{retryAttempts: 3, retryBackoffBase: 0} // 0 backoff = instant

// Transient: exactly 1 + 3 attempts, then the tagged error surfaces.
calls := 0
err := r.retryLoop(context.Background(), func() error {
calls++
return errors.New("429 too many requests")
})
require.Error(t, err)
assert.Equal(t, 4, calls, "transient must retry exactly 3 times after the first attempt")
assert.Contains(t, err.Error(), "after 3 transient retries")

// Deterministic: exactly 1 attempt.
calls = 0
err = r.retryLoop(context.Background(), func() error {
calls++
return document.ErrUnknownFormat
})
require.Error(t, err)
assert.Equal(t, 1, calls, "deterministic must fail after exactly one attempt")

// Success on the second attempt stops the loop.
calls = 0
err = r.retryLoop(context.Background(), func() error {
calls++
if calls < 2 {
return errors.New("503")
}
return nil
})
require.NoError(t, err)
assert.Equal(t, 2, calls)
}
