package usecases_test

import (
	"bytes"
	"errors"
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

	rawShares := []domain.CustodianShare{
		{Index: 1, Share: strings.Repeat("01", 32), CommitmentWord: "abandon"},
		{Index: 2, Share: strings.Repeat("02", 32), CommitmentWord: "ability"},
		{Index: 3, Share: strings.Repeat("03", 32), CommitmentWord: "able"},
		{Index: 4, Share: strings.Repeat("04", 32), CommitmentWord: "about"},
		{Index: 5, Share: strings.Repeat("05", 32), CommitmentWord: "above"},
	}

	mockClient := &mockCeremonyClient{
		initThresholdFunc: func(k, n int) (*domain.ThresholdInitResult, error) {
			return &domain.ThresholdInitResult{
				Threshold: k,
				Total:     n,
				Shares:    rawShares,
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

	// Verify that formatted shares with index and CRC appear in output buffer
	for _, share := range rawShares {
		formatted, err := domain.FormatCustodianShare(share.Index, share.Share)
		if err != nil {
			t.Fatalf("failed formatting test share: %v", err)
		}
		if !strings.Contains(out.String(), formatted) {
			t.Errorf("expected output to contain formatted share %s, but was not found", formatted)
		}
	}
}

func TestRunTurnByTurnAuth_Success(t *testing.T) {
	// Pre-format 3 custodian shares
	share1, err := domain.FormatCustodianShare(1, strings.Repeat("01", 32))
	if err != nil {
		t.Fatalf("FormatCustodianShare failed: %v", err)
	}
	share2, err := domain.FormatCustodianShare(2, strings.Repeat("02", 32))
	if err != nil {
		t.Fatalf("FormatCustodianShare failed: %v", err)
	}
	share3, err := domain.FormatCustodianShare(3, strings.Repeat("03", 32))
	if err != nil {
		t.Fatalf("FormatCustodianShare failed: %v", err)
	}

	// Turn 1: Enter -> Share 1 -> Enter
	// Turn 2: Enter -> Share 2 -> Enter
	// Turn 3: Enter -> Share 3 -> Enter
	// Joint verification: abandon ability able
	inputs := []string{
		"\n", share1 + "\n", "\n",
		"\n", share2 + "\n", "\n",
		"\n", share3 + "\n", "\n",
		"abandon ability able\n",
	}
	in := strings.NewReader(strings.Join(inputs, ""))
	var out bytes.Buffer

	commitWords := map[int]string{
		1: "abandon",
		2: "ability",
		3: "able",
	}

	submittedIndices := make([]int, 0, 3)
	submittedHexes := make([]string, 0, 3)

	mockClient := &mockCeremonyClient{
		submitShareFunc: func(idx int, share string) (*domain.WordCommitment, error) {
			submittedIndices = append(submittedIndices, idx)
			submittedHexes = append(submittedHexes, share)
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

	// Verify indices and shares extracted and submitted to HSM
	expectedIndices := []int{1, 2, 3}
	expectedHexes := []string{
		strings.Repeat("01", 32),
		strings.Repeat("02", 32),
		strings.Repeat("03", 32),
	}
	for i := 0; i < 3; i++ {
		if submittedIndices[i] != expectedIndices[i] {
			t.Errorf("turn %d: expected submitted index %d, got %d", i+1, expectedIndices[i], submittedIndices[i])
		}
		if submittedHexes[i] != expectedHexes[i] {
			t.Errorf("turn %d: expected submitted hex %s, got %s", i+1, expectedHexes[i], submittedHexes[i])
		}
	}
}

func TestRunTurnByTurnAuth_VerificationMismatchAborts(t *testing.T) {
	share1, _ := domain.FormatCustodianShare(1, strings.Repeat("01", 32))
	share2, _ := domain.FormatCustodianShare(2, strings.Repeat("02", 32))
	share3, _ := domain.FormatCustodianShare(3, strings.Repeat("03", 32))

	inputs := []string{
		"\n", share1 + "\n", "\n",
		"\n", share2 + "\n", "\n",
		"\n", share3 + "\n", "\n",
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

func TestRunTurnByTurnAuth_TypoDoesNotCallHardware(t *testing.T) {
	validShare, _ := domain.FormatCustodianShare(1, strings.Repeat("01", 32))
	// Corrupt the CRC suffix (last character)
	corrupted := validShare[:len(validShare)-1] + "0"
	if corrupted == validShare {
		corrupted = validShare[:len(validShare)-1] + "1"
	}

	inputs := []string{
		"\n", corrupted + "\n",
	}
	in := strings.NewReader(strings.Join(inputs, ""))
	var out bytes.Buffer

	submitCalled := false
	mockClient := &mockCeremonyClient{
		submitShareFunc: func(idx int, share string) (*domain.WordCommitment, error) {
			submitCalled = true
			return &domain.WordCommitment{Index: idx, Word: "abandon"}, nil
		},
	}

	svc := usecases.NewCeremonyServiceWithIO(in, &out)
	_, err := svc.RunTurnByTurnAuth(mockClient)
	if err == nil {
		t.Fatalf("expected error on corrupted share checksum, got nil")
	}
	if !errors.Is(err, domain.ErrInvalidShareChecksum) {
		t.Errorf("expected error wrapping ErrInvalidShareChecksum, got: %v", err)
	}
	if submitCalled {
		t.Errorf("SubmitShare was called despite invalid checksum!")
	}
	if !strings.Contains(out.String(), "Checksum mismatch") {
		t.Errorf("expected checksum mismatch diagnostic in output, got: %s", out.String())
	}
}

func TestRunTurnByTurnAuth_RawShareRejection(t *testing.T) {
	rawShare := strings.Repeat("02", 32) // 64 hex characters without hyphens

	inputs := []string{
		"\n", rawShare + "\n",
	}
	in := strings.NewReader(strings.Join(inputs, ""))
	var out bytes.Buffer

	submitCalled := false
	mockClient := &mockCeremonyClient{
		submitShareFunc: func(idx int, share string) (*domain.WordCommitment, error) {
			submitCalled = true
			return &domain.WordCommitment{Index: idx, Word: "abandon"}, nil
		},
	}

	svc := usecases.NewCeremonyServiceWithIO(in, &out)
	_, err := svc.RunTurnByTurnAuth(mockClient)
	if err == nil {
		t.Fatalf("expected error on raw share, got nil")
	}
	if !errors.Is(err, domain.ErrRawShareDetected) {
		t.Errorf("expected error wrapping ErrRawShareDetected, got: %v", err)
	}
	if submitCalled {
		t.Errorf("SubmitShare was called despite raw share input!")
	}
	if !strings.Contains(out.String(), "Legacy raw 64-hex share detected") {
		t.Errorf("expected legacy raw share diagnostic in output, got: %s", out.String())
	}
}

func TestRunTurnByTurnAuth_InvalidFormatDoesNotCallHardware(t *testing.T) {
	inputs := []string{
		"\n", "invalid-share-string\n",
	}
	in := strings.NewReader(strings.Join(inputs, ""))
	var out bytes.Buffer

	submitCalled := false
	mockClient := &mockCeremonyClient{
		submitShareFunc: func(idx int, share string) (*domain.WordCommitment, error) {
			submitCalled = true
			return &domain.WordCommitment{Index: idx, Word: "abandon"}, nil
		},
	}

	svc := usecases.NewCeremonyServiceWithIO(in, &out)
	_, err := svc.RunTurnByTurnAuth(mockClient)
	if err == nil {
		t.Fatalf("expected error on invalid format, got nil")
	}
	if submitCalled {
		t.Errorf("SubmitShare was called despite invalid format!")
	}
	if !strings.Contains(out.String(), "Format Error") {
		t.Errorf("expected format error in output, got: %s", out.String())
	}
}
