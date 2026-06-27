package pipeline

import (
	"testing"
)

func TestMergeParallelism(t *testing.T) {
	cases := []struct {
		name     string
		parallel int
		nSources int
		want     int
	}{
		{"default when unset", 0, 10, 4},
		{"capped to source count", 8, 3, 3},
		{"explicit within bounds", 2, 10, 2},
		{"single source", 4, 1, 1},
		{"negative treated as default", -5, 10, 4},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := &Pipeline{config: &Config{Pipeline: PipelineSettings{Merge: MergeSettings{Parallel: tc.parallel}}}}
			if got := p.mergeParallelism(tc.nSources); got != tc.want {
				t.Fatalf("mergeParallelism(%d) with parallel=%d = %d, want %d", tc.nSources, tc.parallel, got, tc.want)
			}
		})
	}
}

func TestAWSRetryConfigThreadedToClient(t *testing.T) {
	p := &Pipeline{config: &Config{AWS: AWSConfig{Region: "us-east-1", MaxRetries: 7, RetryMode: "adaptive"}}}
	c := p.awsClient("", "", "test")
	if c.MaxRetries != 7 {
		t.Fatalf("MaxRetries not threaded: %d", c.MaxRetries)
	}
	if c.RetryMode != "adaptive" {
		t.Fatalf("RetryMode not threaded: %q", c.RetryMode)
	}
}
