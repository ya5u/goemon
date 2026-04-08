package agent

import "testing"

func TestNeedsPlanning(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "simple greeting",
			input: "Hello, how are you?",
			want:  false,
		},
		{
			name:  "simple question",
			input: "What is the weather today?",
			want:  false,
		},
		{
			name:  "numbered steps with actions",
			input: "1. Search for AI news\n2. Fetch the articles\n3. Write a summary\n4. Create a PR",
			want:  true,
		},
		{
			name:  "sequential keywords with actions",
			input: "First, search for the latest AI news. Then fetch each article. Next, write a summary in Japanese. Finally, create a pull request on GitHub.",
			want:  true,
		},
		{
			name:  "japanese sequential keywords",
			input: "まず、AIニュースを検索してください。次に、各記事の詳細を取得してください。その後、日本語で記事を生成してください。最後に、GitHubにPRを作成してください。",
			want:  true,
		},
		{
			name:  "single action",
			input: "Search for AI news",
			want:  false,
		},
		{
			name:  "long but simple",
			input: "Tell me about the history of artificial intelligence, including the major breakthroughs and key researchers who contributed to the field.",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := needsPlanning(tt.input)
			if got != tt.want {
				t.Errorf("needsPlanning() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParsePlan(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		steps   int
	}{
		{
			name:  "clean JSON",
			input: `{"goal":"test","steps":[{"id":1,"description":"do something","expected_output":"done"}]}`,
			steps: 1,
		},
		{
			name:  "with markdown fences",
			input: "```json\n{\"goal\":\"test\",\"steps\":[{\"id\":1,\"description\":\"step1\",\"expected_output\":\"ok\"}]}\n```",
			steps: 1,
		},
		{
			name:  "with surrounding text",
			input: "Here is the plan:\n{\"goal\":\"test\",\"steps\":[{\"id\":1,\"description\":\"step1\",\"expected_output\":\"ok\"},{\"id\":2,\"description\":\"step2\",\"expected_output\":\"ok\"}]}\nDone.",
			steps: 2,
		},
		{
			name:    "invalid JSON",
			input:   "this is not json at all",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan, err := parsePlan(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(plan.Steps) != tt.steps {
				t.Errorf("got %d steps, want %d", len(plan.Steps), tt.steps)
			}
		})
	}
}
