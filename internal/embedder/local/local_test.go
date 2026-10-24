package local

import (
	"context"
	"math"
	"testing"
)

func dot(a, b []float32) float64 {
	var s float64
	for i := range a {
		s += float64(a[i]) * float64(b[i])
	}
	return s
}

func TestDimensionsDefaultAndOverride(t *testing.T) {
	if got := NewDefault().Dimensions(); got != DefaultDimensions {
		t.Fatalf("default dims = %d, want %d", got, DefaultDimensions)
	}
	e, err := New(Config{Dimensions: 256})
	if err != nil {
		t.Fatal(err)
	}
	if e.Dimensions() != 256 {
		t.Fatalf("override dims = %d, want 256", e.Dimensions())
	}
	if _, err := New(Config{Dimensions: -1}); err == nil {
		t.Fatal("negative dims must be rejected")
	}
}

func TestEmbedDeterministicAndNormalized(t *testing.T) {
	e := NewDefault()
	ctx := context.Background()
	a, err := e.Embed(ctx, "员工年假有几天")
	if err != nil {
		t.Fatal(err)
	}
	b, err := e.Embed(ctx, "员工年假有几天")
	if err != nil {
		t.Fatal(err)
	}
	if len(a) != e.Dimensions() {
		t.Fatalf("len = %d, want %d", len(a), e.Dimensions())
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("non-deterministic at %d", i)
		}
	}
	var norm float64
	for _, v := range a {
		norm += float64(v) * float64(v)
	}
	if math.Abs(norm-1) > 1e-4 {
		t.Fatalf("norm² = %f, want 1", norm)
	}
}

func TestEmbedEmptyIsZeroVector(t *testing.T) {
	v, err := NewDefault().Embed(context.Background(), "   \t")
	if err != nil {
		t.Fatal(err)
	}
	for i, x := range v {
		if x != 0 {
			t.Fatalf("empty text must give zero vector, got %f at %d", x, i)
		}
	}
}

func TestRelatedScoresHigherThanUnrelated(t *testing.T) {
	e := NewDefault()
	ctx := context.Background()
	must := func(s string) []float32 {
		v, err := e.Embed(ctx, s)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	q := must("年假可以休几天")
	related := must("员工年假天数与休假申请流程")
	unrelated := must("服务器机房温度监控告警配置")
	rel, unr := dot(q, related), dot(q, unrelated)
	if rel <= unr {
		t.Fatalf("related %f must score above unrelated %f", rel, unr)
	}
}

func TestEmbedConcurrencySafe(t *testing.T) {
	e := NewDefault()
	want, _ := e.Embed(context.Background(), "并发安全检查文本")
	done := make(chan struct{}, 8)
	for i := 0; i < 8; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			got, err := e.Embed(context.Background(), "并发安全检查文本")
			if err != nil {
				t.Error(err)
				return
			}
			for j := range got {
				if got[j] != want[j] {
					t.Errorf("concurrent result diverged at %d", j)
					return
				}
			}
		}()
	}
	for i := 0; i < 8; i++ {
		<-done
	}
}
