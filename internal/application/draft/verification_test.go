package draft

import "testing"

func TestParseFinalVerificationReportPass(t *testing.T) {
	report := "PASS\nSummary: 要件に沿っています"
	verification := ParseFinalVerificationReport(report)
	if !verification.Performed || !verification.Passed {
		t.Fatalf("unexpected verification: %#v", verification)
	}
	if verification.Summary != "要件に沿っています" {
		t.Fatalf("unexpected summary: %q", verification.Summary)
	}
}

func TestParseFinalVerificationReportNeedsReview(t *testing.T) {
	report := "NEEDS_REVIEW\nSummary: 根拠が不足しています\n- 実測値の根拠が本文にありません\n- 除外条件に触れています"
	verification := ParseFinalVerificationReport(report)
	if verification.Passed {
		t.Fatalf("verification should fail: %#v", verification)
	}
	if len(verification.Failures) != 2 {
		t.Fatalf("unexpected failures: %#v", verification.Failures)
	}
}
