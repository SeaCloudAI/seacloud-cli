package models

import "testing"

func TestEmbeddedAliasConfigLoaded(t *testing.T) {
	if got := modelAliasConfig.Exact["kling_image_o1"]; got != "kirin_image_omni" {
		t.Fatalf("expected embedded exact alias for kling_image_o1, got %q", got)
	}
	if len(modelAliasConfig.Prefixes) != 3 {
		t.Fatalf("expected 3 embedded prefix rules, got %d", len(modelAliasConfig.Prefixes))
	}
}

func TestResolveModelID(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		display string
	}{
		{name: "kling_duration_extension", input: "kling_duration_extension", want: "kirin_duration_extension", display: "kling_duration_extension"},
		{name: "kling_image_o1", input: "kling_image_o1", want: "kirin_image_omni", display: "kling_image_o1"},
		{name: "kling_v1_5_i2v", input: "kling_v1_5_i2v", want: "kirin_v1_5_i2v", display: "kling_v1_5_i2v"},
		{name: "kling_v1_6_i2v", input: "kling_v1_6_i2v", want: "kirin_v1_6_i2v", display: "kling_v1_6_i2v"},
		{name: "kling_v1_6_multi_i2v", input: "kling_v1_6_multi_i2v", want: "kirin_v1_6_multi_i2v", display: "kling_v1_6_multi_i2v"},
		{name: "kling_v1_6_t2v", input: "kling_v1_6_t2v", want: "kirin_v1_6_t2v", display: "kling_v1_6_t2v"},
		{name: "kling_v1_i2v", input: "kling_v1_i2v", want: "kirin_v1_i2v", display: "kling_v1_i2v"},
		{name: "kling_v1_t2v", input: "kling_v1_t2v", want: "kirin_v1_t2v", display: "kling_v1_t2v"},
		{name: "kling_v2_1_i2v", input: "kling_v2_1_i2v", want: "kirin_v2_1_i2v", display: "kling_v2_1_i2v"},
		{name: "kling_v2_1_master_i2v", input: "kling_v2_1_master_i2v", want: "kirin_v2_1_master_i2v", display: "kling_v2_1_master_i2v"},
		{name: "kling_v2_1_master_t2v", input: "kling_v2_1_master_t2v", want: "kirin_v2_1_master_t2v", display: "kling_v2_1_master_t2v"},
		{name: "kling_v2_5_turbo_i2v", input: "kling_v2_5_turbo_i2v", want: "kirin_v2_5_turbo_i2v", display: "kling_v2_5_turbo_i2v"},
		{name: "kling_v2_5_turbo_t2v", input: "kling_v2_5_turbo_t2v", want: "kirin_v2_5_turbo_t2v", display: "kling_v2_5_turbo_t2v"},
		{name: "kling_v2_6_i2v", input: "kling_v2_6_i2v", want: "kirin_v2_6_i2v", display: "kling_v2_6_i2v"},
		{name: "kling_v2_6_t2v", input: "kling_v2_6_t2v", want: "kirin_v2_6_t2v", display: "kling_v2_6_t2v"},
		{name: "kling_v2_master_i2v", input: "kling_v2_master_i2v", want: "kirin_v2_master_i2v", display: "kling_v2_master_i2v"},
		{name: "kling_v2_master_t2v", input: "kling_v2_master_t2v", want: "kirin_v2_master_t2v", display: "kling_v2_master_t2v"},
		{name: "kling_v3_i2v", input: "kling_v3_i2v", want: "kirin_v3_i2v", display: "kling_v3_i2v"},
		{name: "kling_v3_image", input: "kling_v3_image", want: "kirin_v3_image", display: "kling_v3_image"},
		{name: "kling_v3_omni_image", input: "kling_v3_omni_image", want: "kirin_v3_omni_image", display: "kling_v3_omni_image"},
		{name: "kling_v3_omni_video", input: "kling_v3_omni_video", want: "kirin_v3_omni_video", display: "kling_v3_omni_video"},
		{name: "kling_v3_t2v", input: "kling_v3_t2v", want: "kirin_v3_t2v", display: "kling_v3_t2v"},
		{name: "kling_video_o1", input: "kling_video_o1", want: "kirin_video_o1", display: "kling_video_o1"},
		{name: "seedance_2_0", input: "seedance_2_0", want: "spark_dance_v2_0", display: "seedance_2_0"},
		{name: "seedance_2_0_fast", input: "seedance_2_0_fast", want: "spark_dance_v2_0_fast", display: "seedance_2_0_fast"},
		{name: "seedream_3_0", input: "seedream_3_0", want: "spark_dream_3_0", display: "seedream_3_0"},
		{name: "seedream_4_0", input: "seedream_4_0", want: "spark_dream_4_0", display: "seedream_4_0"},
		{name: "seedream_4_5", input: "seedream_4_5", want: "spark_dream_4_5", display: "seedream_4_5"},
		{name: "seedream_5_0", input: "seedream_5_0", want: "spark_dream_5_0", display: "seedream_5_0"},
		{name: "seacloud source prefix", input: "seacloud__happyhorse_1.0_t2v", want: "happyhorse_1.0_t2v", display: "happyhorse_1.0_t2v"},
		{
			name:    "legacy id stays unchanged",
			input:   "kirin_v2_6_i2v",
			want:    "kirin_v2_6_i2v",
			display: "kling_v2_6_i2v",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ResolveModelID(tt.input); got != tt.want {
				t.Fatalf("ResolveModelID(%q) = %q, want %q", tt.input, got, tt.want)
			}
			if got := DisplayModelID(tt.want); got != tt.display {
				t.Fatalf("DisplayModelID(%q) = %q, want %q", tt.want, got, tt.display)
			}
		})
	}
}

func TestPreferredModelID(t *testing.T) {
	if got := PreferredModelID("seedance_2_0", "spark_dance_v2_0"); got != "seedance_2_0" {
		t.Fatalf("expected preferred requested alias, got %q", got)
	}
	if got := PreferredModelID("seedream_4_5", "spark_dream_4_5"); got != "seedream_4_5" {
		t.Fatalf("expected preferred requested seedream alias, got %q", got)
	}
	if got := PreferredModelID("spark_dance_v2_0", "spark_dance_v2_0"); got != "spark_dance_v2_0" {
		t.Fatalf("expected preferred legacy id, got %q", got)
	}
}

func TestRewriteModelIDText(t *testing.T) {
	text := "model=kirin_v3_t2v endpoint=kirin_v3_t2v"
	got := RewriteModelIDText(text, "kirin_v3_t2v", "kling_v3_t2v")
	want := "model=kling_v3_t2v endpoint=kling_v3_t2v"
	if got != want {
		t.Fatalf("RewriteModelIDText() = %q, want %q", got, want)
	}
}
