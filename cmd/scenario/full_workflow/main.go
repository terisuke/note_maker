package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func main() {
	if os.Getenv("RUN_NOTE_SCENARIO") != "1" {
		fatalf("set RUN_NOTE_SCENARIO=1 to run full workflow scenario")
	}

	run("author style", nil, "go", "run", "./cmd/scenario/author_style")
	profileID := readProfileID("tmp/author_style/profile.json")
	run("brief interview", []string{"STYLE_PROFILE_ID=" + profileID}, "go", "run", "./cmd/scenario/brief_interview")

	if os.Getenv("RUN_LOCAL_LLM_SCENARIO") == "1" {
		run("draft generation", nil, "go", "run", "./cmd/scenario/draft_generation")
	} else {
		fmt.Println("draft generation skipped; set RUN_LOCAL_LLM_SCENARIO=1 to include local LLM generation")
	}
	fmt.Println("full workflow scenario completed")
}

func run(label string, env []string, name string, args ...string) {
	fmt.Printf("running %s...\n", label)
	command := exec.Command(name, args...)
	command.Env = append(os.Environ(), env...)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		fatalf("%s failed: %v", label, err)
	}
}

func readProfileID(path string) string {
	encoded, err := os.ReadFile(path)
	if err != nil {
		fatalf("read profile: %v", err)
	}
	var payload struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(encoded, &payload); err != nil {
		fatalf("decode profile: %v", err)
	}
	if strings.TrimSpace(payload.ID) == "" {
		fatalf("profile id was empty")
	}
	return payload.ID
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
