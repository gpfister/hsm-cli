package usecases_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/gpfister/nrf52840-hsm-cli/internal/domain"
	"github.com/gpfister/nrf52840-hsm-cli/internal/usecases"
)

type mockCeremonyClient struct {
	initThresholdFunc func(k, n int) (*domain.ThresholdInitResult, error)
	submitShareFunc   func(idx int, share string) (*domain.WordCommitment, error)
	statusFunc        func() (*domain.DeviceStatus, error)
	lockFunc          func() error
}

func (m *mockCeremonyClient) ThresholdInit(threshold, total int) (*domain.ThresholdInitResult, error) {
	if m.initThresholdFunc != nil {
		return m.initThresholdFunc(threshold, total)
	}
	return nil, nil
}

func (m *mockCeremonyClient) SubmitShare(index int, shareHex string) (*domain.WordCommitment, error) {
	if m.submitShareFunc != nil {
		return m.submitShareFunc(index, shareHex)
	}
	return nil, nil
}

func (m *mockCeremonyClient) Status() (*domain.DeviceStatus, error) {
	if m.statusFunc != nil {
		return m.statusFunc()
	}
	return &domain.DeviceStatus{Authenticated: true}, nil
}

func (m *mockCeremonyClient) Lock() error {
	if m.lockFunc != nil {
		return m.lockFunc()
	}
	return nil
}

func TestClearTerminalAndScrollback(t *testing.T) {
	var buf bytes.Buffer
	usecases.ClearTerminalAndScrollback(&buf)

	expected := "\033[H\033[2J\033[3J"
	if buf.String() != expected {
		t.Errorf("expected %q, got %q", expected, buf.String())
	}
}

func TestReadMaskedPassword_NonTerminal(t *testing.T) {
	input := "supersecretshare123\n"
	in := strings.NewReader(input)
	var out bytes.Buffer

	svc := usecases.NewCeremonyServiceWithIO(in, &out)
	pw, err := svc.ReadMaskedPassword("Enter share: ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if pw != "supersecretshare123" {
		t.Errorf("expected 'supersecretshare123', got %q", pw)
	}
	if !strings.Contains(out.String(), "Enter share: ") {
		t.Errorf("expected prompt in output buffer, got: %s", out.String())
	}
}

func TestVerifyCommitmentWords_Success(t *testing.T) {
	input := "abandon ability able\n"
	in := strings.NewReader(input)
	var out bytes.Buffer

	svc := usecases.NewCeremonyServiceWithIO(in, &out)
	err := svc.VerifyCommitmentWords([]string{"abandon", "ability", "able"})
	if err != nil {
		t.Fatalf("expected successful verification, got: %v", err)
	}

	if !strings.Contains(out.String(), "All commitment words verified successfully") {
		t.Errorf("expected success message in output, got: %s", out.String())
	}
}

func TestVerifyCommitmentWords_MismatchAborts(t *testing.T) {
	input := "abandon wrong able\n"
	in := strings.NewReader(input)
	var out bytes.Buffer

	svc := usecases.NewCeremonyServiceWithIO(in, &out)
	err := svc.VerifyCommitmentWords([]string{"abandon", "ability", "able"})
	if err == nil {
		t.Fatalf("expected error on word mismatch, got nil")
	}

	if !strings.Contains(err.Error(), "mismatch") {
		t.Errorf("expected mismatch in error message, got: %v", err)
	}
}

func TestRunTurnByTurnInit(t *testing.T) {
	// 5 custodians * 2 Enter prompts per turn = 10 enters
	input := strings.Repeat("\n", 12)
	in := strings.NewReader(input)
	var out bytes.Buffer

	mockClient := &mockCeremonyClient{
		initThresholdFunc: func(k, n int) (*domain.ThresholdInitResult, error) {
			return &domain.ThresholdInitResult{
				Threshold: k,
				Total:     n,
				Shares: []domain.CustodianShare{
					{Index: 1, Share: "share01hex", CommitmentWord: "abandon"},
					{Index: 2, Share: "share02hex", CommitmentWord: "ability"},
					{Index: 3, Share: "share03hex", CommitmentWord: "able"},
					{Index: 4, Share: "share04hex", CommitmentWord: "about"},
					{Index: 5, Share: "share05hex", CommitmentWord: "above"},
				},
			}, nil
		},
	}

	svc := usecases.NewCeremonyServiceWithIO(in, &out)
	shares, err := svc.RunTurnByTurnInit(mockClient)
	if err != nil {
		t.Fatalf("RunTurnByTurnInit failed: %v", err)
	}

	if len(shares) != 5 {
		t.Errorf("expected 5 shares, got %d", len(shares))
	}
	if !strings.Contains(out.String(), "THRESHOLD CEREMONY COMPLETE") {
		t.Errorf("expected completion banner in output")
	}
}

func TestRunTurnByTurnAuth_Success(t *testing.T) {
	// Turn 1: Enter -> Index 1 -> Share "share01" -> Enter
	// Turn 2: Enter -> Index 2 -> Share "share02" -> Enter
	// Turn 3: Enter -> Index 3 -> Share "share03" -> Enter
	// Joint verification: abandon ability able
	inputs := []string{
		"\n", "1\n", "share01\n", "\n",
		"\n", "2\n", "share02\n", "\n",
		"\n", "3\n", "share03\n", "\n",
		"abandon ability able\n",
	}
	in := strings.NewReader(strings.Join(inputs, ""))
	var out bytes.Buffer

	commitWords := map[int]string{
		1: "abandon",
		2: "ability",
		3: "able",
	}

	mockClient := &mockCeremonyClient{
		submitShareFunc: func(idx int, share string) (*domain.WordCommitment, error) {
			return &domain.WordCommitment{
				Index: idx,
				Word:  commitWords[idx],
			}, nil
		},
	}

	svc := usecases.NewCeremonyServiceWithIO(in, &out)
	shares, err := svc.RunTurnByTurnAuth(mockClient)
	if err != nil {
		t.Fatalf("RunTurnByTurnAuth failed: %v", err)
	}

	if len(shares) != 3 {
		t.Fatalf("expected 3 submitted shares, got %d", len(shares))
	}
	if shares[0].CommitmentWord != "abandon" || shares[1].CommitmentWord != "ability" || shares[2].CommitmentWord != "able" {
		t.Errorf("unexpected commitment words in result: %+v", shares)
	}
	if !strings.Contains(out.String(), "THRESHOLD AUTHENTICATION SUCCESSFUL") {
		t.Errorf("expected authentication success banner in output")
	}
}

func TestRunTurnByTurnAuth_VerificationMismatchAborts(t *testing.T) {
	inputs := []string{
		"\n", "1\n", "share01\n", "\n",
		"\n", "2\n", "share02\n", "\n",
		"\n", "3\n", "share03\n", "\n",
		"abandon WRONG able\n",
	}
	in := strings.NewReader(strings.Join(inputs, ""))
	var out bytes.Buffer

	mockClient := &mockCeremonyClient{
		submitShareFunc: func(idx int, share string) (*domain.WordCommitment, error) {
			return &domain.WordCommitment{Index: idx, Word: "abandon"}, nil
		},
	}

	svc := usecases.NewCeremonyServiceWithIO(in, &out)
	_, err := svc.RunTurnByTurnAuth(mockClient)
	if err == nil {
		t.Fatalf("expected error on commitment word verification mismatch, got nil")
	}

	if !strings.Contains(err.Error(), "joint commitment verification aborted") {
		t.Errorf("expected verification abort message, got: %v", err)
	}
}
