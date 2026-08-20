/*
Copyright 2026 The Dapr Authors
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package logger

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureHandler is a secondary sink that records everything it receives.
type captureHandler struct {
	mu      sync.Mutex
	records []slog.Record

	min slog.Level
	err error
}

func (c *captureHandler) Enabled(_ context.Context, l slog.Level) bool {
	return l >= c.min
}

func (c *captureHandler) Handle(_ context.Context, r slog.Record) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.records = append(c.records, r.Clone())

	return c.err
}

func (c *captureHandler) WithAttrs([]slog.Attr) slog.Handler { return c }

func (c *captureHandler) WithGroup(string) slog.Handler { return c }

func (c *captureHandler) captured() []slog.Record {
	c.mu.Lock()
	defer c.mu.Unlock()

	return append([]slog.Record{}, c.records...)
}

// resetGlobalHandlers undoes AddHandlerToAllLoggers when the test finishes:
// registration is global and permanent by design, so tests must clean up after
// themselves to not leak sinks into unrelated tests.
func resetGlobalHandlers(t *testing.T) {
	t.Helper()

	t.Cleanup(func() {
		globalStatesLock.Lock()
		defer globalStatesLock.Unlock()

		globalHandlers = nil
		for _, s := range globalStates {
			s.handlers.Store(nil)
		}
	})
}

func recordAttrs(r slog.Record) map[string]string {
	out := map[string]string{}

	r.Attrs(func(a slog.Attr) bool {
		out[a.Key] = a.Value.String()
		return true
	})

	return out
}

func TestFanoutExistingLogger(t *testing.T) {
	resetGlobalHandlers(t)

	l := New("test.fanout.existing")
	l.SetOutput(io.Discard)

	c := &captureHandler{min: slog.LevelDebug}
	AddHandlerToAllLoggers(c)

	l.Info("hello")

	recs := c.captured()
	require.Len(t, recs, 1)
	assert.Equal(t, "hello", recs[0].Message)
}

func TestFanoutLoggerCreatedAfterRegistration(t *testing.T) {
	resetGlobalHandlers(t)

	c := &captureHandler{min: slog.LevelDebug}
	AddHandlerToAllLoggers(c)

	l := New("test.fanout.after")
	l.SetOutput(io.Discard)
	l.Info("hello after")

	recs := c.captured()
	require.Len(t, recs, 1)
	assert.Equal(t, "hello after", recs[0].Message)
}

func TestFanoutLegacyLogger(t *testing.T) {
	resetGlobalHandlers(t)

	c := &captureHandler{min: slog.LevelDebug}
	AddHandlerToAllLoggers(c)

	l := NewLogger("test.fanout.legacy")
	l.SetOutput(io.Discard)
	l.Info("legacy hello")

	recs := c.captured()
	require.Len(t, recs, 1)
	assert.Equal(t, "legacy hello", recs[0].Message)
	assert.Equal(t, "test.fanout.legacy", recordAttrs(recs[0])[logFieldScope])
}

func TestFanoutDerivedLoggers(t *testing.T) {
	resetGlobalHandlers(t)

	l := New("test.fanout.derived")
	l.SetOutput(io.Discard)

	c := &captureHandler{min: slog.LevelDebug}
	AddHandlerToAllLoggers(c)

	l.With("k", "v").Info("with")
	l.WithGroup("g").Info("group", "k2", "v2")
	l.WithLogType(LogTypeRequest).Info("request")
	l.Legacy().Info("legacy view")

	recs := c.captured()
	require.Len(t, recs, 4)

	assert.Equal(t, "v", recordAttrs(recs[0])["k"])
	assert.Equal(t, "v2", recordAttrs(recs[1])["g.k2"])
	assert.Equal(t, LogTypeRequest, recordAttrs(recs[2])[logFieldType])
	assert.Equal(t, "legacy view", recs[3].Message)
}

func TestFanoutSchemaFields(t *testing.T) {
	resetGlobalHandlers(t)

	l := New("test.fanout.schema")
	l.SetOutput(io.Discard)
	l.SetAppID("myapp")

	c := &captureHandler{min: slog.LevelDebug}
	AddHandlerToAllLoggers(c)

	l.Info("schema", "k", "v")

	recs := c.captured()
	require.Len(t, recs, 1)

	attrs := recordAttrs(recs[0])
	assert.Equal(t, "test.fanout.schema", attrs[logFieldScope])
	assert.Equal(t, LogTypeLog, attrs[logFieldType])
	assert.Equal(t, hostname, attrs[logFieldInstance])
	assert.Equal(t, DaprVersion, attrs[logFieldDaprVer])
	assert.Equal(t, "myapp", attrs[logFieldAppID])
	assert.Equal(t, "v", attrs["k"])
}

func TestFanoutFollowsLoggerLevel(t *testing.T) {
	resetGlobalHandlers(t)

	l := New("test.fanout.level")
	l.SetOutput(io.Discard)
	l.SetOutputLevel(InfoLevel)

	c := &captureHandler{min: slog.LevelDebug}
	AddHandlerToAllLoggers(c)

	l.Debug("filtered out")
	assert.Empty(t, c.captured())

	l.SetOutputLevel(DebugLevel)
	l.Debug("now visible")

	recs := c.captured()
	require.Len(t, recs, 1)
	assert.Equal(t, "now visible", recs[0].Message)
}

func TestFanoutRespectsSinkEnabled(t *testing.T) {
	resetGlobalHandlers(t)

	l := New("test.fanout.sinklevel")
	l.SetOutput(io.Discard)

	c := &captureHandler{min: slog.LevelError}
	AddHandlerToAllLoggers(c)

	l.Info("below sink level")
	assert.Empty(t, c.captured())

	l.Error("at sink level")
	require.Len(t, c.captured(), 1)
}

func TestFanoutSinkErrorDoesNotAffectPrimary(t *testing.T) {
	resetGlobalHandlers(t)

	var buf bytes.Buffer

	l := New("test.fanout.sinkerror")
	l.SetOutput(&buf)
	l.EnableJSONOutput(true)

	AddHandlerToAllLoggers(&captureHandler{min: slog.LevelDebug, err: errors.New("sink broken")})

	rec := slog.NewRecord(time.Now(), slog.LevelInfo, "still fine", 0)
	require.NoError(t, l.handler.Handle(context.Background(), rec))

	assert.Contains(t, buf.String(), `"msg":"still fine"`)
}

func TestFanoutFromLoggerThirdParty(t *testing.T) {
	resetGlobalHandlers(t)

	c := &captureHandler{min: slog.LevelDebug}
	AddHandlerToAllLoggers(c)

	l := FromLogger(&nopLogger{})

	assert.NotPanics(t, func() {
		l.Info("into the void")
	})
	assert.Empty(t, c.captured())
}

func TestFanoutConcurrentRegistration(t *testing.T) {
	resetGlobalHandlers(t)

	l := New("test.fanout.concurrent")
	l.SetOutput(io.Discard)

	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for range 100 {
				l.Info("spin")
			}
		}()
	}

	for range 8 {
		AddHandlerToAllLoggers(&captureHandler{min: slog.LevelDebug})
	}

	wg.Wait()
}
